package domain

import (
	"context"
	"database/sql"
	"mirai/internal/panel/store/db"
	"mirai/internal/panel/store/storetest"
	"testing"
	"time"
)

func TestTariffLimitsPropagateWithoutResettingPaidTime(t *testing.T) {
	ctx := context.Background()
	st, err := storetest.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now()
	if err := Seed(ctx, st, now); err != nil {
		t.Fatal(err)
	}
	tariff, err := st.Q.CreateTariff(ctx, db.CreateTariffParams{Name: "Update", DurationDays: 30, TrafficLimit: sql.NullInt64{Int64: 100, Valid: true}, DeviceLimit: sql.NullInt64{Int64: 1, Valid: true}, ResetStrategy: "none", CreatedAt: now.Unix()})
	if err != nil {
		t.Fatal(err)
	}
	users := NewUsers(st, NewPool(st, func() time.Time { return now }), &changes{}, func() time.Time { return now })
	user, err := users.Create(ctx, CreateInput{Name: "Subscriber", TariffID: tariff.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.DB.ExecContext(ctx, "UPDATE users SET used_down=42 WHERE id=$1", user.ID)
	if err != nil {
		t.Fatal(err)
	}
	tariff.TrafficLimit.Int64 = 300
	tariff.DeviceLimit.Int64 = 4
	tariff.ResetStrategy = "month_start"
	err = st.Tx(ctx, func(q *db.Queries) error {
		ids, err := q.PropagateTariff(ctx, tariff, now.Unix())
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := ApplyTariffPools(ctx, q, id, tariff.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := st.Q.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.TrafficLimit.Int64 != 300 || updated.DeviceLimit.Int64 != 4 || updated.UsedDown != 42 || updated.ExpiresAt != user.ExpiresAt || updated.SubToken != user.SubToken {
		t.Fatal("limits not propagated or paid subscription reset")
	}
}
