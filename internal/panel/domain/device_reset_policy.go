package domain

import (
	"context"
	"mirai/internal/panel/settings"
	"mirai/internal/panel/store/db"
	"time"
)

const DeviceResetKey = "device_reset_policy"

type DeviceResetPolicy struct {
	Single     bool `json:"single"`
	All        bool `json:"all"`
	Limit      int  `json:"limit" minimum:"1" maximum:"1000"`
	PeriodDays int  `json:"period_days" minimum:"1" maximum:"365"`
}

func DefaultDeviceResetPolicy() DeviceResetPolicy {
	return DeviceResetPolicy{Single: true, All: true, Limit: 4, PeriodDays: 30}
}
func ResetPolicy(ctx context.Context, q *db.Queries) (DeviceResetPolicy, error) {
	p, _, err := settings.GetOver(ctx, settings.New(q), DeviceResetKey, DefaultDeviceResetPolicy())
	if p.Limit < 1 || p.PeriodDays < 1 {
		p = DefaultDeviceResetPolicy()
	}
	return p, err
}
func resetAllowance(ctx context.Context, q *db.Queries, user int64, now time.Time, all bool) (time.Time, error) {
	p, err := ResetPolicy(ctx, q)
	if err != nil {
		return time.Time{}, err
	}
	if all && !p.All || !all && !p.Single {
		return time.Time{}, ErrResetDisabled
	}
	times, err := q.ResetTimes(ctx, user, now.Add(-time.Duration(p.PeriodDays)*24*time.Hour).Unix())
	if err != nil {
		return time.Time{}, err
	}
	if len(times) >= p.Limit {
		return time.Unix(times[len(times)-p.Limit], 0).Add(time.Duration(p.PeriodDays) * 24 * time.Hour).UTC(), ErrUnbindCooldown
	}
	return time.Time{}, nil
}
func (d *Devices) NextReset(ctx context.Context, user int64) (time.Time, error) {
	t, err := resetAllowance(ctx, d.st.Q, user, d.now(), false)
	if err == ErrUnbindCooldown {
		return t, nil
	}
	return t, err
}
func (d *Devices) UnbindAll(ctx context.Context, user int64, subscriber bool) error {
	err := d.resetAll(ctx, user, subscriber)
	if err == ErrNoSlots {
		if err := d.pool.Refill(ctx, RefillBatch); err != nil {
			return err
		}
		err = d.resetAll(ctx, user, subscriber)
	}
	if err == nil {
		d.changes.PoliciesChanged()
	}
	return err
}
func (d *Devices) resetAll(ctx context.Context, user int64, subscriber bool) error {
	return d.st.Tx(ctx, func(q *db.Queries) error {
		if _, err := q.GetUser(ctx, user); err != nil {
			return err
		}
		devs, err := q.ListBoundDevices(ctx, user)
		if err != nil {
			return err
		}
		if len(devs) == 0 {
			return nil
		}
		if subscriber {
			if _, err := resetAllowance(ctx, q, user, d.now(), true); err != nil {
				return err
			}
		}
		for _, dev := range devs {
			if err := d.removeDeviceTx(ctx, q, user, dev); err != nil {
				return err
			}
		}
		if subscriber {
			return q.RecordReset(ctx, user, d.now().Unix())
		}
		return nil
	})
}
