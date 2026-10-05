package tgbot

import (
	"context"
	"fmt"
	"html"
	"time"

	"mirai/internal/panel/store/db"
)

func (b *Bot) NotifyAdminNotice(ctx context.Context, chat int64, text string) {
	out := b.out.Load()
	cfg := b.Config(ctx)
	if out == nil || chat == 0 || !cfg.Notify.AdminChanges {
		return
	}
	silent := cfg.QuietNight && night(b.d.Now())
	out.Notice(chat, func(ctx context.Context, c *Client) error {
		_, err := c.Send(ctx, chat, text, nil, silent)
		return err
	}, nil)
}

// NotifySubscriptionChange runs only after the administrator's mutation succeeds.
func (b *Bot) NotifySubscriptionChange(ctx context.Context, u db.User, chat int64, action string, days int64) {
	if chat == 0 {
		link, err := b.d.Store.Q.GetTgLink(ctx, u.ID)
		if err != nil {
			return
		}
		chat = link.TgID
	}
	until := "Без срока"
	if u.ExpiresAt.Valid {
		until = time.Unix(u.ExpiresAt.Int64, 0).UTC().Format("02.01.2006")
	}
	limit := "∞"
	if u.DeviceLimit.Valid {
		limit = fmt.Sprint(u.DeviceLimit.Int64)
	}
	text := ""
	switch action {
	case "create":
		text = fmt.Sprintf("🎁 <b>Вам выдана подписка!</b>\n\nАдминистратор предоставил вам доступ.\n📅 Действует до: <b>%s</b>\n📱 Устройств: <b>%s</b>\n\n🔗 <b>Ваша ссылка:</b>\n<code>%s</code>\n\nСкопируйте её и добавьте в Happ или INCY.", until, limit, html.EscapeString(b.subURL(ctx, u)))
	case "extend":
		if days > 0 {
			text = fmt.Sprintf("⏳ <b>Подписка продлена</b>\n\nАдминистратор добавил вам <b>%d дней</b>.\n📅 Действует до: <b>%s</b>\n\n<i>Ссылка не изменилась.</i>", days, until)
		} else {
			text = "⚠️ <b>Срок подписки изменён</b>\n\n📅 Теперь действует до: <b>" + until + "</b>\n\n<i>Ссылка не изменилась.</i>"
		}
	case "devices":
		text = "📱 <b>Лимит устройств изменён</b>\n\nТеперь на вашей подписке доступно: <b>" + limit + "</b>.\n\n<i>Ссылка не изменилась.</i>"
	case "freeze":
		text = "⏸ <b>Подписка отключена администратором</b>\n\nПодключение временно недоступно. Срок подписки продолжает идти. Если это ошибка, обратитесь в поддержку."
	case "unfreeze":
		text = "▶️ <b>Подписка включена</b>\n\nАдминистратор восстановил доступ. Проверьте срок и лимиты в «Моих подписках»; ссылка осталась прежней."
	case "delete":
		text = "🗑 <b>Подписка удалена администратором</b>\n\nСтарая ссылка больше не работает. Ваш профиль и история покупок сохранены. Если это ошибка, обратитесь в поддержку."
	}
	if text != "" {
		b.NotifyAdminNotice(ctx, chat, text)
	}
}
