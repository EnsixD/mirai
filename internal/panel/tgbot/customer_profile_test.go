package tgbot

import (
	"context"
	"mirai/internal/panel/settings"
	"mirai/internal/panel/store/db"
	"mirai/internal/panel/store/storetest"
	"strings"
	"testing"
	"time"
)

func TestCustomerProfilePurchaseTotals(t *testing.T) {
	ctx := context.Background()
	st, err := storetest.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tariff, err := st.Q.CreateTariff(ctx, db.CreateTariffParams{Name: "Paid", DurationDays: 30, ResetStrategy: "none", CreatedAt: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct {
		payload, status, kind string
		chat, amount, days    int64
	}{
		{"mine", "applied", "new", 42, 15000, 30}, {"renew", "applied", "renew", 42, 20000, 60},
		{"refunded", "refunded", "renew", 42, 99900, 30}, {"pending", "pending", "new", 42, 500, 7},
		{"other", "applied", "new", 99, 99900, 365},
	} {
		_, err = st.DB.ExecContext(ctx, `INSERT INTO payments(provider,payload,tg_id,kind,tariff_id,tariff_name,amount,currency,status,created_at,term_days) VALUES('addon:yookassa',$1,$2,$3,$4,'Paid',$5,'RUB',$6,1,$7)`, p.payload, p.chat, p.kind, tariff.ID, p.amount, p.status, p.days)
		if err != nil {
			t.Fatal(err)
		}
	}
	bot := New(Deps{Store: st, Settings: settings.New(st.Q), Now: time.Now, MiniApp: func() bool { return false }, SubBase: func(context.Context) string { return "" }})
	text, kb := bot.customerProfile(ctx, Default("ru"), 42, nil)
	if !strings.Contains(text, "350,00 ₽") || !strings.Contains(text, "90 дн.") || !strings.Contains(text, "3 мес.") || strings.Contains(text, "999") || kb == nil {
		t.Fatalf("incorrect private totals: %s", text)
	}
	legacy := Config{Lang: "ru", Buttons: []MenuButton{{ID: "sub", Action: "sub", Label: "📊 Подписка", On: true}}}
	if err := legacy.Validate(); err != nil {
		t.Fatal(err)
	}
	if legacy.Buttons[0].Action != "profile" || len(legacy.Buttons) != 3 || legacy.Buttons[1].Action != "buy" {
		t.Fatalf("legacy menu not migrated: %+v", legacy.Buttons)
	}
	menu := bot.menu(ctx, Config{Buttons: legacy.Buttons}, wordsFor("ru"), 1)
	profile, buy := false, false
	for _, row := range menu.InlineKeyboard {
		if len(row) > 2 {
			t.Fatal("menu must keep compact rows")
		}
		for _, button := range row {
			profile = profile || button.CallbackData == "pf"
			buy = buy || button.CallbackData == "b"
		}
	}
	if !profile || !buy {
		t.Fatal("profile and purchase entry are missing")
	}
}

func TestAdminMenuSectionsAndDefaultTexts(t *testing.T) {
	ctx := context.Background()
	st, err := storetest.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := Default("ru")
	cfg.Admin.Grant = false
	cfg.Admin.Search = false
	if err := settings.Set(ctx, settings.New(st.Q), KeyConfig, cfg); err != nil {
		t.Fatal(err)
	}
	bot := New(Deps{Store: st, Settings: settings.New(st.Q), Now: time.Now})
	if bot.adminSectionAllowed(ctx, 42, "grant") || bot.adminSectionAllowed(ctx, 42, "search") {
		t.Fatal("disabled section can be opened")
	}
	bot.setAdminFlow(42, adminFlow{Kind: "create"})
	if bot.adminSectionAllowed(ctx, 42, "ok") {
		t.Fatal("disabled grant confirmation still works")
	}
	if !bot.adminSectionAllowed(ctx, 42, "users") {
		t.Fatal("enabled section unavailable")
	}
	cfg.Texts = Texts{}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Texts.Welcome == "" || cfg.Texts.Main == "" || cfg.Texts.Renew == "" || cfg.Texts.Expiring == "" || cfg.Texts.Expired == "" || cfg.Texts.Traffic90 == "" || cfg.Texts.TrafficEnd == "" {
		t.Fatal("standard texts are not filled")
	}
}
