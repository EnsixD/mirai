package billing

// Helpers for the 2trillion bot (internal/panel/ourbot), carried over from Mirai 0.5.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"mirai/internal/panel/addons"
	"mirai/internal/panel/domain"
	"mirai/internal/panel/model"
	"mirai/internal/panel/store/db"
)

// AddonID is the adapter of a provider name, "" for a built-in provider.
func AddonID(provider string) string {
	id, ok := strings.CutPrefix(provider, AddonPrefix)
	if !ok || !addons.ValidID(id) {
		return ""
	}
	return id
}

// RefreshPayment verifies the invoice with its provider before applying anything.
func (s *Service) RefreshPayment(ctx context.Context, id int64) error {
	p, err := s.d.Store.Q.GetPayment(ctx, id)
	if err != nil {
		return err
	}
	if p.ExternalID.Valid && AddonID(p.Provider) != "" {
		if err := s.checkAddon(ctx, p.Provider, p.ExternalID.String); err != nil {
			return err
		}
	}
	p, err = s.d.Store.Q.GetPayment(ctx, id)
	if err != nil {
		return err
	}
	if p.Status == "paid" {
		return s.Apply(ctx, id)
	}
	return nil
}

// ClosePayment expires the local order; a late real payment is still reconciled safely.
func (s *Service) ClosePayment(ctx context.Context, id int64) error {
	if err := s.RefreshPayment(ctx, id); err != nil {
		return err
	}
	changed, err := s.d.Store.Q.SetPaymentStatus(ctx, db.SetPaymentStatusParams{ID: id, OldStatus: "pending", NewStatus: "expired"})
	if err != nil {
		return err
	}
	if changed > 0 && s.d.Promo != nil {
		return s.d.Promo.ReleasePayment(ctx, id)
	}
	return nil
}

func (s *Service) ReopenPayment(ctx context.Context, id int64) error {
	if err := s.RefreshPayment(ctx, id); err != nil {
		return err
	}
	p, err := s.d.Store.Q.GetPayment(ctx, id)
	if err != nil {
		return err
	}
	if p.Status != "expired" {
		return nil
	}
	if s.d.Promo != nil {
		_, err := s.d.Promo.GetPaymentRedemption(ctx, id)
		if err == nil {
			return errors.New("create a new order to refresh its promotional price")
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	_, err = s.d.Store.Q.SetPaymentStatus(ctx, db.SetPaymentStatusParams{ID: id, OldStatus: "expired", NewStatus: "pending"})
	return err
}

// Describe is tariff t bought for a term of days in a line, "30 days · 100 GB · 3
// devices", in lang ("en", else Russian): invoices, the bot and the Mini App show it.
func Describe(t db.Tariff, days int64, lang string) string {
	return TermLabel(t, days, lang) + " · " + DescribeLimits(t, lang)
}

// TermLabel is a term of tariff t in a few words: "30 days", "3 months" for a tariff that
// ends on a billing day, "no end date".
func TermLabel(t db.Tariff, days int64, lang string) string {
	pick := func(ru, en string) string {
		if lang == "en" {
			return en
		}
		return ru
	}
	switch {
	case days <= 0:
		return pick("бессрочно", "no end date")
	case t.BillingDay.Valid:
		n := model.TermMonths(days)
		if n == 1 {
			return pick("1 мес.", "1 month")
		}
		return fmt.Sprintf(pick("%d мес.", "%d months"), n)
	}
	return fmt.Sprintf(pick("%d дн.", "%d days"), days)
}

// DescribeOffer is a tariff on sale in a line: Describe with its one term, or the range
// of its terms, "7 days – 90 days · 100 GB · 3 devices".
func DescribeOffer(o Offer, lang string) string {
	switch len(o.Terms) {
	case 0:
		return Describe(o.Tariff, o.Tariff.DurationDays, lang)
	case 1:
		return Describe(o.Tariff, o.Terms[0].Days, lang)
	}
	var lo, hi int64 = -1, -1
	forever := false
	for _, t := range o.Terms {
		if t.Days <= 0 {
			forever = true
			continue
		}
		if lo < 0 || t.Days < lo {
			lo = t.Days
		}
		hi = max(hi, t.Days)
	}
	if forever {
		hi = 0
	}
	return TermLabel(o.Tariff, lo, lang) + " – " + TermLabel(o.Tariff, hi, lang) + " · " + DescribeLimits(o.Tariff, lang)
}

// DescribeLimits is what tariff t gives whatever the term: "100 GB · 3 devices".
func DescribeLimits(t db.Tariff, lang string) string {
	en := lang == "en"
	pick := func(ru, en_ string) string {
		if en {
			return en_
		}
		return ru
	}
	parts := []string{}
	if t.TrafficLimit.Valid {
		parts = append(parts, fmt.Sprintf(pick("%d ГБ", "%d GB"), t.TrafficLimit.Int64>>30))
	} else {
		parts = append(parts, pick("трафик без лимита", "unlimited traffic"))
	}
	if t.DeviceLimit.Valid {
		parts = append(parts, fmt.Sprintf(pick("устройств: %d", "devices: %d"), t.DeviceLimit.Int64))
	}
	return strings.Join(parts, " · ")
}

// DescribePackage is a package in a line, "50 GB · WL · until the period ends", in lang
// ("en", else Russian); pool is the pool's name, "" for the main traffic.
func DescribePackage(p db.TrafficPackage, pool, lang string) string {
	en := lang == "en"
	pick := func(ru, en_ string) string {
		if en {
			return en_
		}
		return ru
	}
	size := fmt.Sprintf(pick("%s ГБ", "%s GB"), gigabytes(p.Bytes, en))
	target := pool
	if target == "" {
		target = pick("основной трафик", "main traffic")
	}
	var term string
	switch p.Lifetime {
	case domain.LifetimePeriod:
		term = pick("до конца периода", "until the period ends")
	case domain.LifetimeDays:
		term = fmt.Sprintf(pick("на %d дн.", "for %d days"), p.Days)
	default:
		term = pick("пока не израсходован", "until used up")
	}
	return strings.Join([]string{size, target, term}, " · ")
}

// gigabytes: "50", or "1.5" ("1,5" in Russian) for a part of a GB.
func gigabytes(n int64, en bool) string {
	if n%domain.GiB == 0 {
		return fmt.Sprint(n / domain.GiB)
	}
	s := strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/float64(domain.GiB)), ".0")
	if !en {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s
}
