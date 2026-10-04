package domain

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"mirai/internal/panel/store"
	"mirai/internal/panel/store/db"
	"mirai/internal/proto"
)

// Seed initializes the credential pool and upgrades existing inbound templates.
// Tariffs and inbounds are created explicitly by the administrator.
func Seed(ctx context.Context, st *store.Store, now time.Time) error {
	inbounds, err := st.Q.ListInbounds(ctx)
	if err != nil {
		return err
	}
	// Inbounds created by mirai ≤ 0.1.2 carry per-preset settings; give them a template.
	for _, in := range inbounds {
		if in.Config != "" {
			continue
		}
		t, err := proto.FromPreset(in.Preset, []byte(in.Settings))
		if err != nil {
			return fmt.Errorf("inbound %s: %w", in.Name, err)
		}
		if err := st.Q.SetInboundConfig(ctx, db.SetInboundConfigParams{Config: proto.Marshal(t), ID: in.ID}); err != nil {
			return err
		}
	}
	stats, err := NewPool(st, func() time.Time { return now }).Stats(ctx)
	if err != nil {
		return err
	}
	if stats.Free+stats.Assigned+stats.Burned == 0 {
		return NewPool(st, func() time.Time { return now }).Refill(ctx, RefillBatch)
	}
	return nil
}

func nullInt(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: true} }
