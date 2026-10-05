package tgbot

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"mirai/internal/panel/settings"
	"mirai/internal/panel/store/db"
)

const KeyMaintenance = "tg_maintenance"

func (b *Bot) botGate(ctx context.Context, chat int64) string {
	if b.isAdmin(ctx, chat, chat) {
		return ""
	}
	banned, err := b.d.Store.Q.AccountBanned(ctx, chat)
	if err != nil {
		return "Не удалось проверить доступ. Попробуйте ещё раз."
	}
	if banned {
		return "🚫 <b>Доступ ограничен</b>\n\nВаш аккаунт заблокирован администратором. Обратитесь в поддержку."
	}
	maintenance, _, _ := settings.Get[bool](ctx, b.d.Settings, KeyMaintenance)
	if maintenance {
		return "🛠 <b>Технические работы</b>\n\nМы обновляем сервис. Пожалуйста, зайдите немного позже. Ваши подписки и покупки сохранены."
	}
	return ""
}

func (b *Bot) instruction(ctx context.Context, cfg Config) (string, *Keyboard) {
	text := "📖 <b>Как подключиться</b>\n\n<b>1. Установите приложение</b>\nHapp — для телефона и компьютера. INCY — для Android и Android TV.\n\n<b>2. Скопируйте ссылку</b>\n«👤 Профиль» → «📋 Мои подписки» → нужная подписка. Нажмите на ссылку в сообщении, чтобы скопировать её.\n\n<b>3. Добавьте подписку</b>\nВ Happ нажмите «+» → «Добавить из буфера обмена». В INCY — «+» → импорт из буфера.\n\n<b>4. Подключайтесь</b>\nВыберите подключение и включите VPN. При первом запуске разрешите создание VPN-соединения.\n\n<b>Полезно знать</b>\n• Одна ссылка используется на ваших устройствах в пределах лимита тарифа.\n• После продления менять ссылку не нужно.\n• Занятые устройства и их очистка доступны в карточке подписки."
	for _, button := range cfg.Buttons {
		if button.Action == "help" && strings.TrimSpace(button.Text) != "" {
			text = render(button.Text, map[string]string{"brand": b.brand(ctx)})
			break
		}
	}
	rows := [][]Button{{{Text: "📲 Happ · iOS", URL: "https://apps.apple.com/app/happ-proxy-utility/id6504287215"}, {Text: "🤖 Happ · Android", URL: "https://play.google.com/store/apps/details?id=com.happproxy"}}, {{Text: "🤖 INCY · Android", URL: "https://play.google.com/store/apps/details?id=com.incy.app"}}}
	if support := b.supportURL(ctx); support != "" {
		rows = append(rows, []Button{{Text: "💬 Нужна помощь", URL: support}})
	}
	rows = append(rows, []Button{{Text: "← Меню", CallbackData: "m"}})
	return text, &Keyboard{rows}
}

func (b *Bot) ordersScreen(ctx context.Context, chat int64, arg string, admin bool) (string, *Keyboard) {
	page, _ := strconv.Atoi(arg)
	page = max(0, page)
	all, err := b.d.Store.Q.ListPendingPayments(ctx, 0)
	if err != nil {
		return "Не удалось загрузить заказы.", &Keyboard{}
	}
	orders := []db.Payment{}
	for i := len(all) - 1; i >= 0; i-- {
		if admin || all[i].TgID == chat {
			orders = append(orders, all[i])
		}
	}
	if len(orders) > 0 {
		page = min(page, (len(orders)-1)/8)
	} else {
		page = 0
	}
	prefix, back, title := "order:", Button{Text: "← Профиль", CallbackData: "pf"}, "🧾 Мои заказы"
	if admin {
		prefix = "a:order:"
		back = adminButton("⚙️ Админка", "home")
		title = "🧾 Заказы"
	}
	rows := [][]Button{}
	for _, p := range orders[min(page*8, len(orders)):min(page*8+8, len(orders))] {
		label := fmt.Sprintf("🧾 #%d · %s · %s", p.ID, p.TariffName, wordsFor("ru").price(p.Amount, p.Currency))
		if admin {
			label = fmt.Sprintf("🧾 #%d · %d · %s", p.ID, p.TgID, wordsFor("ru").price(p.Amount, p.Currency))
		}
		rows = append(rows, []Button{{Text: shortAdmin(label, 60), CallbackData: prefix + strconv.FormatInt(p.ID, 10)}})
	}
	nav := []Button{}
	listPrefix := "orders:"
	if admin {
		listPrefix = "a:orders:"
	}
	if page > 0 {
		nav = append(nav, Button{Text: "‹", CallbackData: listPrefix + strconv.Itoa(page-1)})
	}
	if (page+1)*8 < len(orders) {
		nav = append(nav, Button{Text: "›", CallbackData: listPrefix + strconv.Itoa(page+1)})
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}
	rows = append(rows, []Button{{Text: "🔄 Обновить", CallbackData: listPrefix + strconv.Itoa(page)}, back})
	text := fmt.Sprintf("<b>%s (%d)</b>\n\nВыберите заказ, чтобы вернуться к оплате или проверить её статус. Подтверждённая оплата выдаёт или продлевает подписку автоматически.", title, len(orders))
	if len(orders) == 0 {
		text = "<b>" + title + "</b>\n\nНеоплаченных заказов нет."
		if !admin {
			rows = [][]Button{{{Text: "🛒 Купить", CallbackData: "b"}, back}}
		}
	}
	return text, &Keyboard{rows}
}

func (b *Bot) orderScreen(ctx context.Context, chat, id int64, admin bool) (string, *Keyboard) {
	p, err := b.d.Store.Q.GetPayment(ctx, id)
	if err != nil || !admin && p.TgID != chat {
		return "Заказ не найден.", &Keyboard{}
	}
	back := Button{Text: "← Мои заказы", CallbackData: "orders"}
	prefix := ""
	if admin {
		back = adminButton("← Заказы", "orders:0")
		prefix = "a:"
	}
	text := fmt.Sprintf("🧾 <b>Заказ #%d</b>\n\nТариф: %s\nСумма: <b>%s</b>\nСтатус: %s", p.ID, html.EscapeString(p.TariffName), wordsFor("ru").price(p.Amount, p.Currency), html.EscapeString(p.Status))
	switch p.Status {
	case "pending":
		text += "\n\nОплатите по ссылке и нажмите «Проверить оплату». Подписка появится после подтверждения платежа платёжной системой."
	case "paid":
		text += "\n\nОплата подтверждена. Подписка выдаётся автоматически."
	case "applied":
		text += "\n\nЗаказ выполнен. Подписка доступна в разделе «Мои подписки»."
	default:
		text += "\n\nЭтот заказ больше не ожидает оплаты. Для новой покупки выберите тариф в разделе «Купить»."
	}
	status := map[string]string{"pending": "ожидает оплаты", "paid": "оплачен, выдача в процессе", "applied": "выполнен", "expired": "закрыт", "failed": "отклонён", "refunded": "возвращён"}[p.Status]
	if status != "" {
		text = strings.Replace(text, "Статус: "+p.Status, "Статус: <b>"+status+"</b>", 1)
	}
	text += "\n\n🕒 Создан: " + time.Unix(p.CreatedAt, 0).UTC().Format("02.01.2006 15:04 UTC")
	if strings.HasPrefix(p.ExternalID.String, "test:") {
		text += "\n\n🧪 <b>Тестовый магазин</b> — этот заказ не входит в статистику реальных покупок."
	}
	if admin {
		text += fmt.Sprintf("\n\nПользователь: <code>%d</code>", p.TgID)
		if account, err := b.d.Store.Q.GetTgChat(ctx, p.TgID); err == nil {
			text += " · " + html.EscapeString(account.FirstName)
			if account.Username != "" {
				text += " · @" + html.EscapeString(account.Username)
			}
		}
	}
	rows := [][]Button{}
	if p.Status == "pending" {
		if !admin && strings.HasPrefix(p.PayUrl, "https://") {
			rows = append(rows, []Button{{Text: "🌐 Перейти к оплате", URL: p.PayUrl}})
		}
		rows = append(rows, []Button{{Text: "✅ Проверить оплату", CallbackData: prefix + "ordercheck:" + strconv.FormatInt(id, 10)}, {Text: "🗑 Закрыть заказ", CallbackData: prefix + "orderclose:" + strconv.FormatInt(id, 10)}})
	}
	if admin && p.Status == "expired" {
		rows = append(rows, []Button{adminButton("♻️ Вернуть в работу", fmt.Sprintf("orderrestore:%d", id))})
	}
	if !admin && p.Status == "applied" {
		rows = append(rows, []Button{{Text: "🔑 Мои подписки", CallbackData: "w"}})
	}
	rows = append(rows, []Button{back})
	if admin {
		rows = append(rows, adminBack())
	}
	return text, &Keyboard{rows}
}

func (b *Bot) orderAction(ctx context.Context, chat, id int64, admin, close bool) (string, string) {
	p, err := b.d.Store.Q.GetPayment(ctx, id)
	if err != nil || (!admin && p.TgID != chat) || b.d.Billing == nil {
		return "m", "Заказ не найден."
	}
	if close {
		err = b.d.Billing.ClosePayment(ctx, id)
	} else {
		err = b.d.Billing.RefreshPayment(ctx, id)
	}
	prefix := "order:"
	if admin {
		prefix = "a:order:"
	}
	if err != nil {
		return prefix + strconv.FormatInt(id, 10), "Не удалось проверить платёж. Попробуйте позже."
	}
	if close {
		latest, _ := b.d.Store.Q.GetPayment(ctx, id)
		if latest.Status == "applied" || latest.Status == "paid" {
			return prefix + strconv.FormatInt(id, 10), "Платёж подтверждён. Заказ уже обрабатывается или выполнен."
		}
		prefix = "orders"
		if admin {
			prefix = "a:orders:0"
		}
		return prefix, "Заказ закрыт в списке. Если платёж уже отправлен, он будет обработан после подтверждения платёжной системой."
	}
	return prefix + strconv.FormatInt(id, 10), "Статус платежа проверен."
}
