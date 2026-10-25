package app

import (
	"context"
	"errors"

	"mirai/internal/panel/ourbot"
	"mirai/internal/panel/settings"
	"mirai/internal/panel/tgate"
)

// botSwitch hands billing the bot that holds the token now: ours while settings.OurBot is
// on, else the gateway (the bot addon).
type botSwitch struct {
	set  *settings.Settings
	ours *ourbot.Bot
	gate *tgate.Gateway
}

func (s botSwitch) on(ctx context.Context) bool {
	on, _ := s.set.On(ctx, settings.OurBot)
	return on
}

func (s botSwitch) InvoiceLink(ctx context.Context, title, description, payload string, stars int64) (string, error) {
	if s.on(ctx) {
		return s.ours.InvoiceLink(ctx, title, description, payload, stars)
	}
	return s.gate.InvoiceLink(ctx, title, description, payload, stars)
}

func (s botSwitch) RefundStars(ctx context.Context, tgID int64, chargeID string) error {
	if s.on(ctx) {
		return s.ours.RefundStars(ctx, tgID, chargeID)
	}
	return s.gate.RefundStars(ctx, tgID, chargeID)
}

func (s botSwitch) BotURL(ctx context.Context) string {
	if s.on(ctx) {
		return s.ours.BotURL(ctx)
	}
	return s.gate.BotURL(ctx)
}

func (s botSwitch) Channel(ctx context.Context) (string, bool) {
	if s.on(ctx) {
		return "", false
	}
	return s.gate.Channel(ctx)
}

func (s botSwitch) Subscribed(ctx context.Context, tgID int64) (bool, error) {
	if s.on(ctx) {
		return false, errors.New("no channel")
	}
	return s.gate.Subscribed(ctx, tgID)
}
