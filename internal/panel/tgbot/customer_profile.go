package tgbot

import (
	"context"
	"fmt"
	"html"
	"mirai/internal/panel/domain"
	"mirai/internal/panel/store/db"
	"strconv"
	"strings"
)

func (b *Bot) customerProfile(ctx context.Context, cfg Config, chat int64, subscriptions []db.User) (string, *Keyboard) {
	ru := cfg.Lang != "en"
	tr := func(r, e string) string {
		if ru {
			return r
		}
		return e
	}
	totals, err := b.d.Store.Q.CustomerPurchases(ctx, chat)
	back := []Button{{Text: wordsFor(cfg.Lang).back, CallbackData: "m"}}
	if err != nil {
		return tr("Не удалось загрузить профиль. Попробуйте ещё раз.", "Could not load your profile. Please try again."), &Keyboard{[][]Button{back}}
	}
	name := strconv.FormatInt(chat, 10)
	if account, err := b.d.Store.Q.GetTgChat(ctx, chat); err == nil {
		if account.FirstName != "" {
			name = account.FirstName
		}
		if account.Username != "" {
			name += " · @" + account.Username
		}
	}
	active := 0
	for _, subscription := range subscriptions {
		grants, err := domain.UserGrantsLeft(ctx, b.d.Store.Q, subscription.ID, b.d.Now())
		if err != nil {
			continue
		}
		if state := domain.State(subscription, grants.Main(subscription.ID), b.d.Now()); state == domain.StateActive || state == domain.StateExpiring {
			active++
		}
	}
	duration := fmt.Sprintf(tr("%d дн. (%d мес. и %d дн.)", "%d days (%d months and %d days)"), totals.Days, totals.Days/30, totals.Days%30)
	lines := []string{"<b>" + tr("◉ Профиль", "◉ Profile") + "</b>", html.EscapeString(name), "ID: <code>" + strconv.FormatInt(chat, 10) + "</code>", "",
		fmt.Sprintf(tr("◇ Потрачено: %d,%02d ₽", "◇ Spent: %d.%02d RUB"), totals.RublesKopecks/100, totals.RublesKopecks%100),
		"📅 " + tr("Куплено подписки: ", "Subscription purchased: ") + duration,
		fmt.Sprintf(tr("🧾 Покупок и продлений: %d", "🧾 Purchases and renewals: %d"), totals.Purchases),
		fmt.Sprintf(tr("▤ Подписок: %d · активных: %d", "▤ Subscriptions: %d · active: %d"), len(subscriptions), active)}
	if totals.Unlimited > 0 {
		lines = append(lines, fmt.Sprintf(tr("♾ Бессрочных покупок: %d", "♾ Unlimited purchases: %d"), totals.Unlimited))
	}
	lines = append(lines, "", tr("Учтены завершённые покупки без возвратов. Бесплатные и выданные администратором подписки не входят в оплаченный срок. Месяц в статистике — 30 дней.", "Completed purchases excluding refunds. Free and administrator-issued subscriptions do not count as purchased time. A month here is 30 days."))
	rows := [][]Button{}
	if len(subscriptions) > 0 {
		rows = append(rows, []Button{{Text: tr("▤ Мои подписки", "▤ My subscriptions"), CallbackData: "w"}})
	}
	if b.canBuyNew(ctx) {
		rows = append(rows, []Button{{Text: wordsFor(cfg.Lang).buy, CallbackData: "b"}})
	}
	return strings.Join(lines, "\n"), &Keyboard{append(rows, back)}
}
