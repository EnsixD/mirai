package tgbot

import (
	"fmt"
	"strings"
	"testing"

	"mirai/internal/panel/domain"
	"mirai/internal/panel/store/db"
)

func TestAdminCustomerProfileAndGrant(t *testing.T) {
	e := adminEnv(t)
	e.say(555, "/start")
	e.tg.wait(t, 0, "sendMessage")
	if err := e.st.Q.LinkTg(e.ctx, db.LinkTgParams{UserID: e.user.ID, TgID: 555, CreatedAt: e.clock().Unix()}); err != nil {
		t.Fatal(err)
	}
	tariffs, _ := e.st.Q.ListTariffs(e.ctx)
	if _, err := e.bot.d.Users.Create(e.ctx, domain.CreateInput{Name: "Second", TariffID: tariffs[0].ID, TelegramID: 555}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.st.DB.ExecContext(e.ctx, `INSERT INTO payments(provider,payload,tg_id,kind,tariff_id,tariff_name,amount,currency,status,created_at,term_days) VALUES('addon:yookassa','profile-test',555,'new',$1,'Paid',35000,'RUB','applied',1,90)`, tariffs[0].ID); err != nil {
		t.Fatal(err)
	}
	text, kb := e.bot.adminScreen(e.ctx, 900, "a:contact:555", "")
	if !strings.Contains(text, "350,00 ₽") || !strings.Contains(text, "90 дн.") || !strings.Contains(text, "Подписок: 2") {
		t.Fatal("administrator cannot see customer totals:", text)
	}
	grant := false
	owned := false
	for _, row := range kb.InlineKeyboard {
		for _, button := range row {
			grant = grant || button.CallbackData == "a:grant:555"
			owned = owned || button.CallbackData == "a:owned:555"
			if strings.HasPrefix(button.CallbackData, "a:user:") {
				t.Fatal("subscription buttons mixed into the customer profile")
			}
		}
	}
	if !grant {
		t.Fatal("grant is not tied to the selected account")
	}
	if !owned {
		t.Fatal("separate customer subscription list missing")
	}
	text, kb = e.bot.adminScreen(e.ctx, 900, "a:owned:555", "")
	if !strings.Contains(text, "Подписки пользователя: 2") || !strings.HasPrefix(kb.InlineKeyboard[0][0].Text, "📋 #") {
		t.Fatal("subscription list does not clearly identify subscriptions", text)
	}
	text, kb = e.bot.adminScreen(e.ctx, 900, fmt.Sprintf("a:user:%d", e.user.ID), "")
	for _, removed := range []string{"Контакт:", "За всё время:", "Последняя активность:", "Остаток пакетов:", "↑"} {
		if strings.Contains(text, removed) {
			t.Fatalf("unwanted subscription detail: %s", removed)
		}
	}
	back := false
	for _, row := range kb.InlineKeyboard {
		for _, button := range row {
			back = back || button.CallbackData == "a:subs:0"
			if strings.HasPrefix(button.CallbackData, "a:owner:") {
				t.Fatal("duplicate customer-profile action remains on subscription")
			}
		}
	}
	if !back {
		t.Fatal("subscription must return to all subscriptions")
	}
	_, kb = e.bot.adminScreen(e.ctx, 900, "a:home", "")
	for _, row := range kb.InlineKeyboard {
		for _, button := range row {
			if button.CallbackData == "a:grant" {
				t.Fatal("grant still shown on home")
			}
		}
	}
	_, kb = e.bot.adminScreen(e.ctx, 900, "a:users:0", "")
	contacts := 0
	for _, row := range kb.InlineKeyboard {
		for _, button := range row {
			if button.CallbackData == "a:contact:555" {
				contacts++
			}
		}
	}
	if contacts != 1 {
		t.Fatal("account duplicated for multiple subscriptions", contacts)
	}
	text, kb = e.bot.screen(e.ctx, e.bot.Config(e.ctx), 555, "s", "")
	if !strings.Contains(text, "<code>https://vpn.example.com:21355/sub/"+e.user.SubToken+"</code>") {
		t.Fatal("connection link missing from subscription")
	}
	if kb.InlineKeyboard[len(kb.InlineKeyboard)-1][0].CallbackData != "w" {
		t.Fatal("subscription does not return to subscription list")
	}
	adminTap(e, "a:grant:555")
	flow, ok := e.bot.adminFlow(900, false)
	if !ok || flow.TgID != 555 || flow.Kind != "tariff" {
		t.Fatal(fmt.Sprintf("wrong selected recipient: %+v", flow))
	}
}

func TestRichTelegramDefaults(t *testing.T) {
	cfg := Default("ru")
	cfg.Texts.Welcome = "{brand}\n\nБезопасное и удобное подключение к интернету. Выберите действие ниже."
	cfg.Texts.Renew = "My custom renewal message"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cfg.Texts.Welcome, "<b>") || cfg.Texts.Renew != "My custom renewal message" {
		t.Fatal("built-in migration replaced custom text or retained old default")
	}
	if got := render("<b>{name}</b>\n<i>Help</i>", map[string]string{"name": "<b>unsafe</b>"}); got != "<b>&lt;b&gt;unsafe&lt;/b&gt;</b>\n<i>Help</i>" {
		t.Fatal("user data rendered as markup", got)
	}
	if got := render("<b>unclosed", nil); got != "&lt;b&gt;unclosed" {
		t.Fatal("unbalanced Telegram markup was enabled", got)
	}
}
