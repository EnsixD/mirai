package api

import (
	"context"
	"mirai/internal/panel/domain"
	"mirai/internal/panel/settings"
	"mirai/internal/panel/store/db"
	"mirai/internal/panel/store/storetest"
	"testing"
	"time"
)

func TestTelegramVisitorsAppearWithoutSubscriptions(t *testing.T) {
	ctx := context.Background()
	st, err := storetest.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Q.UpsertTgChat(ctx, db.UpsertTgChatParams{TgID: 987654, Username: "visitor_name", FirstName: "Visitor Nick", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	h := handlers{d: Deps{Store: st, Settings: settings.New(st.Q), Now: time.Now}}
	overview, err := h.overview(ctx, nil)
	if err != nil || overview.Body.UsersTotal != 1 {
		t.Fatal("navigation counter excludes Telegram visitor", err)
	}
	result, err := h.listUsers(ctx, &listUsersInput{State: "all", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Body.Items) != 1 || result.Body.Items[0].State != "visitor" || result.Body.Items[0].Contact != "@visitor_name" || result.Body.Items[0].Name != "Visitor Nick" || result.Body.Items[0].SubURL != "" {
		t.Fatalf("visitor is missing or granted access: %+v", result.Body)
	}
	found, err := h.listUsers(ctx, &listUsersInput{State: "visitor", Query: "visitor_name", Limit: 100})
	if err != nil || found.Body.Total != 1 {
		t.Fatal("username search failed", err)
	}
	// Repeated /start updates identity and never duplicates the contact.
	if err := st.Q.UpsertTgChat(ctx, db.UpsertTgChatParams{TgID: 987654, Username: "renamed", FirstName: "New Nick", CreatedAt: 2, UpdatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	result, err = h.listUsers(ctx, &listUsersInput{State: "all", Limit: 100})
	if err != nil || len(result.Body.Items) != 1 || result.Body.Items[0].Name != "New Nick" {
		t.Fatal("contact update failed", err)
	}
	if err := domain.Seed(ctx, st, time.Now()); err != nil {
		t.Fatal(err)
	}
	tariff, err := st.Q.CreateTariff(ctx, db.CreateTariffParams{Name: "Linked tariff", DurationDays: 30, ResetStrategy: "none", CreatedAt: 1})
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Now
	service := domain.NewUsers(st, domain.NewPool(st, clock), visitorChanges{}, clock)
	user, err := service.Create(ctx, domain.CreateInput{Name: "Subscription", TariffID: tariff.ID, TelegramID: 987654})
	if err != nil {
		t.Fatal(err)
	}
	result, err = h.listUsers(ctx, &listUsersInput{State: "all", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Body.Items) != 1 || result.Body.Items[0].ID != user.ID || result.Body.Items[0].TariffID == nil || result.Body.Items[0].Telegram == nil || result.Body.Items[0].Contact != "@renamed" {
		t.Fatalf("visitor duplicated after issuing subscription: %+v", result.Body)
	}
	devices := domain.NewDevices(st, domain.NewPool(st, clock), visitorChanges{}, clock)
	if _, err := devices.Bind(ctx, user, domain.DeviceInfo{HWID: "HappDevice123456789", OS: "Windows", App: "Happ/4.4.1"}, false); err != nil {
		t.Fatal(err)
	}
	result, err = h.listUsers(ctx, &listUsersInput{State: "all", Limit: 100})
	if err != nil || result.Body.Items[0].BoundDevices != 1 || len(result.Body.Items[0].OnlineIPs) != 0 {
		t.Fatal("offline registered device was not counted", err)
	}
	if count, err := st.Q.CountTelegramVisitors(ctx); err != nil || count != 0 {
		t.Fatal("subscribed account counted twice", count, err)
	}
	if affected, err := service.Bulk(ctx, []int64{-987654}, domain.BulkDelete, 0); err != nil || affected != 0 {
		t.Fatal("stale visitor selection deleted a subscribed account", affected, err)
	}
	if _, err := st.DB.ExecContext(ctx, `INSERT INTO payments(provider,payload,tg_id,user_id,kind,tariff_id,tariff_name,amount,currency,status,created_at,term_days) VALUES('addon:yookassa','delete-test',987654,$1,'new',$2,'Paid',20000,'RUB','applied',1,30)`, user.ID, tariff.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	result, err = h.listUsers(ctx, &listUsersInput{State: "all", Limit: 100})
	if err != nil || len(result.Body.Items) != 1 || result.Body.Items[0].State != "visitor" || result.Body.Items[0].Contact != "@renamed" {
		t.Fatal("deleting the subscription removed the Telegram profile", err)
	}
	if count, err := st.Q.CountTelegramVisitors(ctx); err != nil || count != 1 {
		t.Fatal("unsubscribed account absent from counter", count, err)
	}
	if totals, err := st.Q.CustomerPurchases(ctx, 987654); err != nil || totals.RublesKopecks != 20000 || totals.Days != 30 {
		t.Fatal("subscription deletion lost customer purchase history", err)
	}
	if affected, err := service.Bulk(ctx, []int64{-987654, -987654}, domain.BulkDelete, 0); err != nil || affected != 1 {
		t.Fatal("visitor cannot be deleted through panel bulk action", affected, err)
	}
	result, err = h.listUsers(ctx, &listUsersInput{State: "all", Limit: 100})
	if err != nil || result.Body.Total != 0 {
		t.Fatal("deleted visitor remains in panel", err)
	}
	if totals, err := st.Q.CustomerPurchases(ctx, 987654); err != nil || totals.RublesKopecks != 20000 {
		t.Fatal("visitor deletion removed receipts", err)
	}

}

type visitorChanges struct{}

func (visitorChanges) PoliciesChanged() {}
func (visitorChanges) SlotsChanged()    {}
