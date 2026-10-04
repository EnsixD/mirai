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

}

type visitorChanges struct{}

func (visitorChanges) PoliciesChanged() {}
func (visitorChanges) SlotsChanged()    {}
