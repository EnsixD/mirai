package domain

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"
)

func TestImportedHashedDeviceKeepsSlot(t *testing.T) {
	now := time.Unix(1800000000, 0)
	st, users, ch := setup(t, &now)
	ctx := context.Background()
	clock := func() time.Time { return now }
	d := NewDevices(st, NewPool(st, clock), ch, clock)
	if _, err := st.DB.ExecContext(ctx, "INSERT INTO tariffs(id,name,duration_days,created_at) VALUES(1,'Legacy',30,$1)", now.Unix()); err != nil {
		t.Fatal(err)
	}
	u, err := users.Create(ctx, CreateInput{Name: "legacy", TariffID: 1})
	if err != nil {
		t.Fatal(err)
	}
	info := DeviceInfo{HWID: "old-device-1234567890", OS: "Android"}
	slot, err := d.Bind(ctx, u, info, false)
	if err != nil {
		t.Fatal(err)
	}
	hash := fmt.Sprintf("legacy-sha256:%x", sha256.Sum256([]byte(info.HWID)))
	if _, err = st.DB.ExecContext(ctx, "UPDATE bound_devices SET hwid=$1 WHERE user_id=$2", hash, u.ID); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	again, err := d.Bind(ctx, u, info, false)
	if err != nil || again.ID != slot.ID {
		t.Fatal("imported device changed slot", err)
	}
	n, err := st.Q.CountBoundDevices(ctx, u.ID)
	if err != nil || n != 1 {
		t.Fatal("imported device counted twice", n, err)
	}
}
