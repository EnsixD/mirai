package domain

import (
	"context"
	"errors"
	"mirai/internal/panel/settings"
	"mirai/internal/panel/store/db"
	"testing"
	"time"
)

func TestDeviceResetQuota(t *testing.T) {
	now := time.Unix(1800000000, 0)
	st, users, changes := setup(t, &now)
	ctx := context.Background()
	tariff, err := st.Q.CreateTariff(ctx, db.CreateTariffParams{Name: "Reset", DurationDays: 365, ResetStrategy: "none", CreatedAt: now.Unix()})
	if err != nil {
		t.Fatal(err)
	}
	u, err := users.Create(ctx, CreateInput{Name: "Reset user", TariffID: tariff.ID})
	if err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return now }
	devs := NewDevices(st, NewPool(st, clock), changes, clock)
	bind := func() {
		t.Helper()
		if _, err := devs.Bind(ctx, u, DeviceInfo{HWID: "phone-0123456789"}, false); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 4; i++ {
		bind()
		if _, err := devs.Bind(ctx, u, DeviceInfo{HWID: "laptop-0123456789"}, false); err != nil {
			t.Fatal(err)
		}
		if err := devs.UnbindAll(ctx, u.ID, true); err != nil {
			t.Fatal(err)
		}
	}
	bind()
	if err := devs.UnbindAll(ctx, u.ID, true); !errors.Is(err, ErrUnbindCooldown) {
		t.Fatalf("fifth reset: %v", err)
	}
	if err := devs.UnbindAll(ctx, u.ID, false); err != nil {
		t.Fatalf("admin reset: %v", err)
	}
	times, err := st.Q.ResetTimes(ctx, u.ID, 0)
	if err != nil || len(times) != 4 {
		t.Fatalf("journal: %v %v", times, err)
	}
	now = now.Add(30 * 24 * time.Hour)
	bind()
	if err := devs.UnbindAll(ctx, u.ID, true); err != nil {
		t.Fatalf("window expired: %v", err)
	}
	policy := DefaultDeviceResetPolicy()
	policy.Single = false
	policy.All = false
	if err := settings.Set(ctx, settings.New(st.Q), DeviceResetKey, policy); err != nil {
		t.Fatal(err)
	}
	bind()
	list, _ := st.Q.ListBoundDevices(ctx, u.ID)
	if err := devs.Unbind(ctx, u.ID, list[0].ID, true); !errors.Is(err, ErrResetDisabled) {
		t.Fatalf("single disabled: %v", err)
	}
	if err := devs.UnbindAll(ctx, u.ID, true); !errors.Is(err, ErrResetDisabled) {
		t.Fatalf("all disabled: %v", err)
	}
	if err := devs.UnbindAll(ctx, u.ID, false); err != nil {
		t.Fatalf("admin unrestricted: %v", err)
	}
}
