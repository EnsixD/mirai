package domain

import (
	"context"
	"mirai/internal/panel/settings"
	"mirai/internal/panel/store/db"
	"testing"
	"time"
)

func TestShortSubscriptionTokens(t *testing.T) {
	now := time.Now()
	st, users, _ := setup(t, &now)
	ctx := context.Background()
	tariff, err := st.Q.CreateTariff(ctx, db.CreateTariffParams{Name: "Short", DurationDays: 30, ResetStrategy: "none", CreatedAt: now.Unix()})
	if err != nil {
		t.Fatal(err)
	}
	old, err := users.Create(ctx, CreateInput{Name: "Old", TariffID: tariff.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(old.SubToken) != 24 {
		t.Fatal("default length")
	}
	if err := settings.Set(ctx, settings.New(st.Q), settings.KeySubIDLength, 9); err != nil {
		t.Fatal(err)
	}
	short, err := users.Create(ctx, CreateInput{Name: "Short", TariffID: tariff.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(short.SubToken) != 9 {
		t.Fatal("short length")
	}
	found, err := st.Q.GetUserBySubToken(ctx, old.SubToken)
	if err != nil || found.ID != old.ID {
		t.Fatal("old link changed")
	}
	if _, err := users.Reissue(ctx, short.ID); err != nil {
		t.Fatal(err)
	}
	found, err = st.Q.GetUser(ctx, short.ID)
	if err != nil || len(found.SubToken) != 9 || found.SubToken == short.SubToken {
		t.Fatal("reissue length")
	}
}
