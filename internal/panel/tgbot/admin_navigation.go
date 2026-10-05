package tgbot

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"

	"mirai/internal/panel/settings"
	"mirai/internal/panel/store/db"
)

func (b *Bot) adminNavigationScreen(ctx context.Context, chat int64, data string) (string, *Keyboard, bool) {
	cmd, arg, _ := strings.Cut(data, ":")
	id, _ := strconv.ParseInt(arg, 10, 64)
	switch cmd {
	case "orders":
		text, kb := b.ordersScreen(ctx, chat, arg, true)
		return text, kb, true
	case "order":
		text, kb := b.orderScreen(ctx, chat, id, true)
		return text, kb, true
	case "maintenance":
		on, _, _ := settings.Get[bool](ctx, b.d.Settings, KeyMaintenance)
		state, label := "выключены", "🔴 Включить"
		if on {
			state, label = "включены", "🟢 Выключить"
		}
		return "🛠 <b>Технические работы</b>\n\nСейчас: " + state + ".\n\nВо время техработ пользовательское меню бота временно недоступно. Подписки, платежи и доступ администратора сохраняются.", adminKB([]Button{adminButton(label, "mainttoggle"), adminButton("← Админка", "home")}), true
	case "devices":
		u, err := b.d.Store.Q.GetUser(ctx, id)
		if err != nil {
			return "Подписка не найдена.", adminKB(adminBack()), true
		}
		text, kb := b.devices(ctx, wordsFor("ru"), u, "d", 0, "", b.d.Now(), chat)
		for i, row := range kb.InlineKeyboard {
			for j, button := range row {
				target := ""
				switch {
				case strings.HasPrefix(button.CallbackData, "du:"):
					target = fmt.Sprintf("unbind:%d.%s", id, strings.TrimPrefix(button.CallbackData, "du:"))
				case button.CallbackData == "da":
					target = fmt.Sprintf("clear:%d", id)
				case button.CallbackData == "s":
					target = fmt.Sprintf("user:%d", id)
				}
				if target != "" {
					kb.InlineKeyboard[i][j] = adminButton(button.Text, target)
				}
			}
		}
		kb.InlineKeyboard = append(kb.InlineKeyboard, adminBack())
		return text, kb, true
	case "prompt":
		f, ok := b.adminFlow(chat, false)
		if !ok {
			return "", nil, false
		}
		if f.Kind == "devices" {
			return "📱 <b>Лимит устройств</b>\n\nВведите новое количество устройств для выбранной подписки. Например: 7.\n0 — без ограничения количества; занятые устройства продолжают учитываться.", adminKB([]Button{adminButton("← Подписка", fmt.Sprintf("user:%d", f.UserID)), adminButton("⚙️ Админка", "home")}), true
		}
		if f.Kind == "broadcast" {
			return "📢 <b>Рассылка</b>\n\nОтправьте текст сообщения. Перед отправкой всем пользователям будет показано подтверждение. /cancel — отмена.", adminKB(adminBack()), true
		}
	case "confirm":
		f, ok := b.adminFlow(chat, false)
		if !ok {
			return "", nil, false
		}
		text := ""
		switch f.Kind {
		case "account-delete":
			text = fmt.Sprintf("⚠️ <b>Удалить пользователя %d?</b>\n\nВсе его подписки и устройства будут удалены, ссылки перестанут работать. История платежей сохранится. Это действие необратимо.", f.TgID)
		case "trial-reset":
			text = fmt.Sprintf("♻️ <b>Разрешить пробную подписку ещё раз пользователю %d?</b>\n\nЭто исключение из ограничения одного пробного периода на аккаунт.", f.TgID)
		case "maintenance":
			text = "🛠 Изменить режим технических работ?"
		case "broadcast-send":
			text = "📢 <b>Подтвердите рассылку</b>\n\n" + html.EscapeString(f.Name) + "\n\nСообщение будет отправлено пользователям этого бота."
		}
		if text != "" {
			return text, adminKB([]Button{adminButton("✅ Подтвердить", "commit:"+f.Nonce), adminButton("❌ Отмена", "home")}), true
		}
	}
	return "", nil, false
}

func (b *Bot) adminNavigationAction(ctx context.Context, chat int64, cmd, arg string) (string, string, bool) {
	id, _ := strconv.ParseInt(arg, 10, 64)
	switch cmd {
	case "orderrestore":
		if b.d.Billing == nil {
			return "a:orders:0", "Оплаты недоступны.", true
		}
		err := b.d.Billing.ReopenPayment(ctx, id)
		return fmt.Sprintf("a:order:%d", id), adminResult(err, "Статус заказа обновлён."), true
	case "ordercheck", "orderclose":
		data, notice := b.orderAction(ctx, chat, id, true, cmd == "orderclose")
		return data, notice, true
	case "ban":
		if id == chat {
			return fmt.Sprintf("a:contact:%d", id), "Нельзя заблокировать свой аккаунт администратора.", true
		}
		if _, err := b.d.Store.Q.GetTgChat(ctx, id); err != nil {
			return "a:users:0", "Пользователь не найден.", true
		}
		err := b.d.Store.Tx(ctx, func(q *db.Queries) error { _, err := q.ToggleAccountBan(ctx, id); return err })
		if err == nil {
			b.d.Users.Changed()
			b.adminAudit(ctx, chat, "account-ban", id, 0)
			banned, _ := b.d.Store.Q.AccountBanned(ctx, id)
			notice := "✅ <b>Доступ восстановлен</b>\n\nАдминистратор разблокировал ваш аккаунт. Откройте профиль, чтобы посмотреть свои подписки."
			if banned {
				notice = "🚫 <b>Аккаунт заблокирован</b>\n\nАдминистратор ограничил доступ к боту и вашим подпискам. Если это ошибка, обратитесь в поддержку."
			}
			b.NotifyAdminNotice(ctx, id, notice)
		}
		return fmt.Sprintf("a:contact:%d", id), adminResult(err, "Доступ пользователя изменён."), true
	case "account-delete", "trial-reset":
		if id == chat && cmd == "account-delete" {
			return "a:home", "Нельзя удалить свой аккаунт администратора.", true
		}
		if _, err := b.d.Store.Q.GetTgChat(ctx, id); err != nil {
			return "a:users:0", "Пользователь не найден.", true
		}
		b.setAdminFlow(chat, adminFlow{Kind: cmd, TgID: id})
		return "a:confirm", "", true
	case "limit":
		if _, err := b.d.Store.Q.GetUser(ctx, id); err != nil {
			return "a:subs:0", "Подписка не найдена.", true
		}
		b.setAdminFlow(chat, adminFlow{Kind: "devices", UserID: id})
		return "a:prompt", "", true
	case "unbind":
		user, device, _ := strings.Cut(arg, ".")
		uid, _ := strconv.ParseInt(user, 10, 64)
		did, _ := strconv.ParseInt(device, 10, 64)
		err := b.d.Devices.Unbind(ctx, uid, did, false)
		if err == nil {
			b.adminAudit(ctx, chat, "device-unbind", uid, 0)
		}
		return fmt.Sprintf("a:devices:%d", uid), adminResult(err, "Устройство очищено."), true
	case "clear":
		err := b.d.Devices.UnbindAll(ctx, id, false)
		if err == nil {
			b.adminAudit(ctx, chat, "devices-clear", id, 0)
		}
		return fmt.Sprintf("a:devices:%d", id), adminResult(err, "Все устройства очищены."), true
	case "broadcast":
		b.setAdminFlow(chat, adminFlow{Kind: "broadcast"})
		return "a:prompt", "", true
	case "mainttoggle":
		on, _, _ := settings.Get[bool](ctx, b.d.Settings, KeyMaintenance)
		desired := int64(1)
		if on {
			desired = 0
		}
		b.setAdminFlow(chat, adminFlow{Kind: "maintenance", Days: desired})
		return "a:confirm", "", true
	}
	return "", "", false
}

func (b *Bot) commitAdminNavigation(ctx context.Context, chat int64, f adminFlow) (string, string, bool) {
	switch f.Kind {
	case "account-delete":
		err := b.d.Users.DeleteTelegramAccount(ctx, f.TgID)
		if err == nil {
			b.adminAudit(ctx, chat, f.Kind, f.TgID, 0)
		}
		return "a:users:0", adminResult(err, "Пользователь и его подписки удалены."), true
	case "trial-reset":
		err := b.d.Store.Q.ResetAccountTrial(ctx, f.TgID)
		if err == nil {
			b.adminAudit(ctx, chat, f.Kind, f.TgID, 0)
			b.NotifyAdminNotice(ctx, f.TgID, "🎁 <b>Пробный период снова доступен</b>\n\nАдминистратор разрешил повторно попробовать сервис. Откройте «Купить», чтобы выбрать пробную подписку.")
		}
		return fmt.Sprintf("a:contact:%d", f.TgID), adminResult(err, "Пробная подписка снова доступна."), true
	case "maintenance":
		err := settings.Set(ctx, b.d.Settings, KeyMaintenance, f.Days == 1)
		if err == nil {
			b.adminAudit(ctx, chat, f.Kind, 0, f.Days)
		}
		return "a:maintenance", adminResult(err, "Режим сохранён."), true
	case "broadcast-send":
		count, err := b.Broadcast(ctx, f.Name)
		if err == nil {
			b.adminAudit(ctx, chat, "broadcast", 0, int64(count))
		}
		return "a:home", adminResult(err, fmt.Sprintf("Рассылка поставлена в очередь: %d получателей.", count)), true
	}
	return "", "", false
}

func (b *Bot) adminMenuActionEnabled(ctx context.Context, action string) bool {
	for _, button := range b.Config(ctx).Admin.Buttons {
		if button.Action == action {
			return button.On
		}
	}
	return false
}
