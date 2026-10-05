package domain

import (
	"context"
	"mirai/internal/panel/store/db"
	"testing"
	"time"
)

func TestBoundDeviceTraffic(t *testing.T) {
	now := time.Unix(1800000000, 0)
	st, users, changes := setup(t, &now)
	ctx := context.Background()
	tariff, err := st.Q.CreateTariff(ctx, db.CreateTariffParams{Name: "Traffic", DurationDays: 365, ResetStrategy: "none", CreatedAt: now.Unix()})
	if err != nil {
		t.Fatal(err)
	}
	u, err := users.Create(ctx, CreateInput{Name: "Traffic user", TariffID: tariff.ID})
	if err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return now }
	devices := NewDevices(st, NewPool(st, clock), changes, clock)
	phone, err := devices.Bind(ctx, u, DeviceInfo{HWID: "phone-0123456789"}, false)
	if err != nil {
		t.Fatal(err)
	}
	laptop, err := devices.Bind(ctx, u, DeviceInfo{HWID: "laptop-0123456789"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Q.CountDeviceTraffic(ctx, phone.Name, 100, 200); err != nil {
		t.Fatal(err)
	}
	if err := st.Q.CountDeviceTraffic(ctx, laptop.Name, 300, 400); err != nil {
		t.Fatal(err)
	}
	if err := st.Q.CountDeviceTraffic(ctx, phone.Name, 10, 20); err != nil {
		t.Fatal(err)
	}
	list, _ := st.Q.ListBoundDevices(ctx, u.ID)
	traffic, err := st.Q.DeviceTrafficOf(ctx, u.ID)
	if err != nil || len(traffic) != 2 {
		t.Fatalf("traffic: %v %v", traffic, err)
	}
	for _, d := range list {
		want := db.DeviceTraffic{Up: 110, Down: 220}
		if d.Hwid == "laptop-0123456789" {
			want = db.DeviceTraffic{Up: 300, Down: 400}
		}
		if traffic[d.ID] != want {
			t.Fatalf("device traffic mixed: %v", traffic)
		}
		if err := devices.Unbind(ctx, u.ID, d.ID, false); err != nil {
			t.Fatal(err)
		}
	}
	var remaining int
	if err := st.DB.QueryRowContext(ctx, "SELECT count(*) FROM bound_device_traffic").Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("deleted device counters survive: %d %v", remaining, err)
	}
}
