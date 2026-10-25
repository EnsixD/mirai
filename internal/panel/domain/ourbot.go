package domain

// Helpers for the 2trillion bot (internal/panel/ourbot), carried over from Mirai 0.5 and
// mapped onto the unbind rules of this panel.

import (
	"context"
	"database/sql"
	"time"

	"mirai/internal/panel/settings"
	"mirai/internal/panel/store/db"
)

// ErrResetDisabled: the subscriber may not unbind devices themselves.
var ErrResetDisabled = ErrUnbindOff

// DeviceResetPolicy is the old bot's view of UnbindRules.
type DeviceResetPolicy struct {
	Single, All bool
	Limit       int
	PeriodDays  int
}

func ResetPolicy(ctx context.Context, q *db.Queries) (DeviceResetPolicy, error) {
	r, err := LoadUnbindRules(ctx, settings.New(q))
	return DeviceResetPolicy{Single: !r.Off, All: !r.Off, Limit: r.Limit, PeriodDays: r.Days}, err
}

// NextReset is when the subscriber may unbind again; zero: now.
func (d *Devices) NextReset(ctx context.Context, user int64) (time.Time, error) {
	st, err := d.UnbindState(ctx, user)
	return st.Next, err
}

// UnbindAll frees every device of the user. The subscriber's clear-all counts as one
// unbind under the rules.
func (d *Devices) UnbindAll(ctx context.Context, user int64, subscriber bool) error {
	if subscriber {
		st, err := d.UnbindState(ctx, user)
		if err != nil {
			return err
		}
		if st.Rules.Off {
			return ErrUnbindOff
		}
		if !st.Next.IsZero() {
			return ErrUnbindCooldown
		}
	}
	devs, err := d.st.Q.ListBoundDevices(ctx, user)
	if err != nil {
		return err
	}
	for _, dev := range devs {
		if err := d.Unbind(ctx, user, dev.ID, false); err != nil && err != ErrNotFound {
			return err
		}
	}
	if subscriber && len(devs) > 0 {
		return d.st.Q.AddUnbind(ctx, db.AddUnbindParams{UserID: user, At: d.now().Unix()})
	}
	return nil
}

// TrafficLeft is what the user may still use of a quota: the base limit's rest plus the
// active grants. -1: unlimited (no limit), which never touches grants.
func TrafficLeft(limit sql.NullInt64, used, grants int64) int64 {
	if !limit.Valid {
		return -1
	}
	return max(0, limit.Int64-used) + max(0, grants)
}

// DeleteTelegramAccount revokes all of the account's keys atomically, retaining receipts.
func (s *Users) DeleteTelegramAccount(ctx context.Context, tgID int64) error {
	err := s.st.Tx(ctx, func(q *db.Queries) error {
		keys, err := q.ListTgLinksOf(ctx, tgID)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(keys))
		for _, key := range keys {
			ids = append(ids, key.ID)
		}
		if len(ids) > 0 {
			if err := deleteUsers(ctx, q, ids, s.now().Unix()); err != nil {
				return err
			}
		}
		return q.DeleteTelegramAccount(ctx, tgID)
	})
	if err == nil {
		s.Changed()
	}
	return err
}

var _ = sql.ErrNoRows
