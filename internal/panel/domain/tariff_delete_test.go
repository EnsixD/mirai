package domain

import (
	"context"
	"database/sql"
	"errors"
	"mirai/internal/panel/store/db"
	"mirai/internal/panel/store/storetest"
	"testing"
	"time"
)

func TestTariffDeletePreservesSubscriptionsAndReceipts(t *testing.T) {
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
	tariff, err := st.Q.CreateTariff(ctx, db.CreateTariffParams{Name: "Delete me", DurationDays: 30, ResetStrategy: "none", CreatedAt: now.Unix()})
	if err != nil {
		t.Fatal(err)
	}
	users := NewUsers(st, NewPool(st, func() time.Time { return now }), &changes{}, func() time.Time { return now })
	user, err := users.Create(ctx, CreateInput{Name: "Keep subscription", TariffID: tariff.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.DB.ExecContext(ctx, `INSERT INTO payments(provider,payload,tg_id,kind,user_id,tariff_id,tariff_name,amount,currency,status,created_at) VALUES('addon:yookassa','receipt',42,'new',$1,$2,'Delete me',10000,'RUB','applied',$3)`, user.ID, tariff.ID, now.Unix())
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteTariff(ctx, st, tariff.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Q.GetTariff(ctx, tariff.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("tariff not deleted: %v", err)
	}
	kept, err := st.Q.GetUser(ctx, user.ID)
	if err != nil || kept.TariffID.Valid || kept.ExpiresAt != user.ExpiresAt || kept.TrafficLimit != user.TrafficLimit {
		t.Fatalf("subscription changed: %v", err)
	}
	receipt, err := st.Q.GetPaymentByPayload(ctx, "receipt")
	if err != nil || receipt.TariffID.Valid || receipt.Status != "applied" || receipt.TermDays.Int64 != 30 {
		t.Fatalf("receipt lost: %v", err)
	}
	totals, err := st.Q.CustomerPurchases(ctx, 42)
	if err != nil || totals.Days != 30 || totals.RublesKopecks != 10000 {
		t.Fatalf("purchase history changed: %+v %v", totals, err)
	}
	tariff, err = st.Q.CreateTariff(ctx, db.CreateTariffParams{Name: "Pending", DurationDays: 7, ResetStrategy: "none", CreatedAt: now.Unix()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.DB.ExecContext(ctx, `INSERT INTO payments(provider,payload,tg_id,kind,tariff_id,tariff_name,amount,currency,status,created_at) VALUES('addon:yookassa','pending',42,'new',$1,'Pending',100,'RUB','pending',$2)`, tariff.ID, now.Unix())
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteTariff(ctx, st, tariff.ID); !errors.Is(err, ErrTariffPayments) {
		t.Fatalf("pending payment not protected: %v", err)
	}
}
