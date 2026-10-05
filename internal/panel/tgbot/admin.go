package tgbot

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"mirai/internal/panel/domain"
	"mirai/internal/panel/secure"
	"mirai/internal/panel/settings"
	"mirai/internal/panel/store/db"
)

// Alert recipients and bot administrators are separate permissions. Old alert links
// never grant access to subscriptions.
const KeyAdminID = "tg_admin_id"

type adminFlow struct {
	Kind                         string
	UserID, TariffID, TgID, Days int64
	Name, Nonce                  string
	Expires                      time.Time
}

func (b *Bot) isAdmin(ctx context.Context, sender, chat int64) bool {
	id, ok, err := settings.Get[int64](ctx, b.d.Settings, KeyAdminID)
	return err == nil && ok && id > 0 && sender == id && chat == id && b.Config(ctx).Admin.Enabled
}

func (b *Bot) addAdminButton(ctx context.Context, chat int64, kb *Keyboard) {
	if kb != nil && b.isAdmin(ctx, chat, chat) {
		kb.InlineKeyboard = append(kb.InlineKeyboard, []Button{{Text: "⚙️ Админ-панель", CallbackData: "a:home"}})
	}
}

func adminKB(rows ...[]Button) *Keyboard   { return &Keyboard{InlineKeyboard: rows, KeepRows: true} }
func adminButton(text, data string) Button { return Button{Text: text, CallbackData: "a:" + data} }
func adminBack() []Button                  { return []Button{adminButton("← Админ", "home")} }
func (b *Bot) setAdminFlow(chat int64, f adminFlow) {
	f.Expires = b.d.Now().Add(10 * time.Minute)
	f.Nonce = secure.Token(12)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.adminFlows == nil {
		b.adminFlows = map[int64]adminFlow{}
	}
	b.adminFlows[chat] = f
}
func (b *Bot) adminFlow(chat int64, consume bool) (adminFlow, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	f, ok := b.adminFlows[chat]
	if consume || !f.Expires.After(b.d.Now()) {
		delete(b.adminFlows, chat)
	}
	return f, ok && f.Expires.After(b.d.Now())
}

func (b *Bot) takeAdminConfirmation(chat int64, nonce string) (adminFlow, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	f, ok := b.adminFlows[chat]
	if !ok || f.Nonce != nonce || !f.Expires.After(b.d.Now()) || !(f.Kind == "delete" || f.Kind == "extend" && f.Days != 0 || f.Kind == "create" || f.Kind == "account-delete" || f.Kind == "trial-reset" || f.Kind == "maintenance" || f.Kind == "broadcast-send") {
		return adminFlow{}, false
	}
	delete(b.adminFlows, chat)
	return f, true
}

func (b *Bot) adminScreen(ctx context.Context, chat int64, data, notice string) (string, *Keyboard) {
	if !b.isAdmin(ctx, chat, chat) {
		return "Нет доступа.", nil
	}
	if !b.adminSectionAllowed(ctx, chat, strings.TrimPrefix(data, "a:")) {
		return "Этот раздел отключён в панели.", adminKB(adminBack())
	}
	text, kb := b.renderAdmin(ctx, chat, strings.TrimPrefix(data, "a:"))
	if notice != "" {
		text = html.EscapeString(notice) + "\n\n" + text
	}
	return text, kb
}

func (b *Bot) renderAdmin(ctx context.Context, chat int64, data string) (string, *Keyboard) {
	if text, kb, handled := b.adminNavigationScreen(ctx, chat, data); handled {
		return text, kb
	}
	cmd, arg, _ := strings.Cut(data, ":")
	id, _ := strconv.ParseInt(arg, 10, 64)
	q := b.d.Store.Q
	switch cmd {
	case "users", "subs":
		users, err := q.ListUsers(ctx)
		if err != nil {
			return "Не удалось загрузить пользователей.", adminKB(adminBack())
		}
		if cmd == "users" {
			accounts, err := q.TelegramAccounts(ctx)
			if err != nil {
				return "Не удалось загрузить посетителей.", adminKB(adminBack())
			}
			accountIDs := map[int64]bool{}
			for _, account := range accounts {
				accountIDs[account.ID] = true
			}
			individual := []db.User{}
			for _, user := range users {
				link, err := q.TgLinkOfUser(ctx, user.ID)
				if err != nil || !accountIDs[link.TgID] {
					individual = append(individual, user)
				}
			}
			users = individual
			seenAccounts := map[int64]bool{}
			for _, account := range accounts {
				if seenAccounts[account.ID] {
					continue
				}
				seenAccounts[account.ID] = true
				name := account.Name
				if name == "" {
					name = strconv.FormatInt(account.ID, 10)
				}
				if account.Username != "" {
					name += " · @" + account.Username
				}
				users = append(users, db.User{ID: -account.ID, Name: name, Status: "account"})
			}
		}
		page, _ := strconv.Atoi(arg)
		page = max(0, page)
		if len(users) > 0 {
			page = min(page, (len(users)-1)/10)
		} else {
			page = 0
		}
		rows := [][]Button{}
		grants, err := domain.LoadGrantsLeft(ctx, q, b.d.Now())
		if err != nil {
			return "Не удалось загрузить статистику.", adminKB(adminBack())
		}
		for _, u := range users[min(page*10, len(users)):min(page*10+10, len(users))] {
			label := fmt.Sprintf("#%d · %s · %s", u.ID, u.Name, adminState(domain.State(u, grants.Main(u.ID), b.d.Now())))
			if u.ID < 0 {
				label = u.Name
				rows = append(rows, []Button{adminButton(shortAdmin(label, 60), fmt.Sprintf("contact:%d", -u.ID))})
			} else {
				rows = append(rows, []Button{adminButton(shortAdmin(label, 60), fmt.Sprintf("user:%d", u.ID))})
			}
		}
		nav := []Button{}
		if page > 0 {
			nav = append(nav, adminButton("←", fmt.Sprintf("%s:%d", cmd, page-1)))
		}
		if (page+1)*10 < len(users) {
			nav = append(nav, adminButton("→", fmt.Sprintf("%s:%d", cmd, page+1)))
		}
		if len(nav) > 0 {
			rows = append(rows, nav)
		}
		rows = append(rows, adminBack())
		title := "Пользователи"
		if cmd == "subs" {
			title = "Все подписки"
		}
		return fmt.Sprintf("<b>%s: %d</b>\nСтраница %d/%d", title, len(users), page+1, max(1, (len(users)+9)/10)), adminKB(rows...)
	case "user":
		u, err := q.GetUser(ctx, id)
		if err != nil {
			return "Подписка не найдена.", adminKB(adminBack())
		}
		g, err := domain.UserGrantsLeft(ctx, q, id, b.d.Now())
		if err != nil {
			return "Не удалось загрузить статистику.", adminKB(adminBack())
		}
		expiry := "Без срока"
		if u.ExpiresAt.Valid {
			expiry = time.Unix(u.ExpiresAt.Int64, 0).UTC().Format("02.01.2006 15:04 UTC")
		}
		limit := "∞"
		if u.TrafficLimit.Valid {
			limit = adminGB(u.TrafficLimit.Int64)
		}
		tariff := "Индивидуальная"
		if u.TariffID.Valid {
			if t, e := q.GetTariff(ctx, u.TariffID.Int64); e == nil {
				tariff = t.Name
			}
		}
		text := fmt.Sprintf("<b>#%d · %s</b>\nСтатус: %s\nТариф: %s\nДо: %s\n\nТрафик периода: %s / %s\n↑ %s · ↓ %s\nЗа всё время: %s\nОстаток пакетов: %s", u.ID, html.EscapeString(shortAdmin(u.Name, 100)), adminState(domain.State(u, g.Main(id), b.d.Now())), html.EscapeString(shortAdmin(tariff, 100)), expiry, adminGB(u.UsedUp+u.UsedDown), limit, adminGB(u.UsedUp), adminGB(u.UsedDown), adminGB(u.TotalUp+u.TotalDown), adminGB(g.Main(id)))
		if link, e := q.TgLinkOfUser(ctx, id); e == nil {
			text += fmt.Sprintf("\nTelegram: %d · @%s", link.TgID, html.EscapeString(link.Username))
		}
		if devs, e := q.ListBoundDevices(ctx, id); e == nil {
			limit := "∞"
			if u.DeviceLimit.Valid {
				limit = strconv.FormatInt(u.DeviceLimit.Int64, 10)
			}
			text += fmt.Sprintf("\nУстройства: %d / %s", len(devs), limit)
		}
		if u.OnlineAt.Valid {
			text += "\nПоследняя активность: " + time.Unix(u.OnlineAt.Int64, 0).UTC().Format("02.01.2006 15:04 UTC")
		}
		if u.Contact != "" {
			text += "\nКонтакт: " + html.EscapeString(shortAdmin(u.Contact, 120))
		}
		for i, line := range b.poolLines(ctx, wordsFor("ru"), id) {
			if i >= 8 {
				text += "\nОстальные пулы доступны в панели."
				break
			}
			text += "\n" + html.EscapeString(shortAdmin(line, 140))
		}
		frozen := u.Status == "disabled"
		freezeLabel, freezeAction := "🔴 Деактивировать", "freeze"
		if frozen {
			freezeLabel, freezeAction = "🟢 Активировать", "unfreeze"
		}
		devices, _ := q.CountBoundDevices(ctx, id)
		deviceLimit := "∞"
		if u.DeviceLimit.Valid {
			deviceLimit = strconv.FormatInt(u.DeviceLimit.Int64, 10)
		}
		if url := b.subURL(ctx, u); url != "" {
			text += "\n\n🔗 <b>Ссылка</b>\n<code>" + html.EscapeString(url) + "</code>"
		}
		rows := [][]Button{
			{adminButton("⏳ Продлить", fmt.Sprintf("extend:%d", id)), adminButton("📱 Лимит: "+deviceLimit, fmt.Sprintf("limit:%d", id))},
			{adminButton(freezeLabel, fmt.Sprintf("%s:%d", freezeAction, id))},
			{adminButton(fmt.Sprintf("📱 Устройства (%d)", devices), fmt.Sprintf("devices:%d", id)), adminButton("🗑 Удалить", fmt.Sprintf("delete:%d", id))},
		}
		rows = append(rows, []Button{adminButton("← Все подписки", "subs:0")})
		return text, adminKB(rows...)
	case "link":
		u, err := q.GetUser(ctx, id)
		if err != nil {
			return "Подписка не найдена.", adminKB(adminBack())
		}
		url := b.subURL(ctx, u)
		if url == "" {
			return "Укажите публичный адрес панели, чтобы получить ссылку.", adminKB(adminBack())
		}
		return "<b>Ссылка подписки</b>\n<code>" + html.EscapeString(url) + "</code>\nХраните её как пароль.", adminKB([]Button{adminButton("← Подписка", fmt.Sprintf("user:%d", id))})
	case "owner":
		link, err := q.TgLinkOfUser(ctx, id)
		if err != nil {
			return "У этой подписки нет привязанного Telegram-аккаунта.", adminKB(adminBack())
		}
		return b.renderAdmin(ctx, chat, fmt.Sprintf("contact:%d", link.TgID))
	case "contact":
		if _, err := q.GetTgChat(ctx, id); err != nil {
			return "Пользователь не найден.", adminKB(adminBack())
		}
		subscriptions, err := q.ListTgLinksOf(ctx, id)
		if err != nil {
			return "Не удалось загрузить подписки.", adminKB(adminBack())
		}
		cfg := b.Config(ctx)
		cfg.Lang = "ru"
		text, _ := b.customerProfile(ctx, cfg, id, subscriptions)
		rows := [][]Button{}
		if cfg.Admin.Grant {
			rows = append(rows, []Button{adminButton("🎁 Выдать", fmt.Sprintf("grant:%d", id))})
		}
		subscriptionButton := adminButton(fmt.Sprintf("🔑 Все подписки (%d)", len(subscriptions)), fmt.Sprintf("owned:%d", id))
		if len(rows) > 0 {
			rows[0] = append(rows[0], subscriptionButton)
		} else {
			rows = append(rows, []Button{subscriptionButton})
		}
		account, _ := q.GetTgChat(ctx, id)
		text += "\n\nПришёл: " + time.Unix(account.CreatedAt, 0).UTC().Format("02.01.2006")
		used, _ := q.AccountTrialUsed(ctx, id)
		trial := "доступна"
		if used {
			trial = "использована"
			rows = append(rows, []Button{adminButton("♻️ Сбросить пробную", fmt.Sprintf("trial-reset:%d", id))})
		}
		text += "\n🎁 Пробная: " + trial
		banned, _ := q.AccountBanned(ctx, id)
		banLabel := "🚫 Заблокировать"
		if banned {
			banLabel = "✅ Разблокировать"
			text += "\n\n🚫 <b>Заблокирован</b>"
		}
		rows = append(rows, []Button{adminButton(banLabel, fmt.Sprintf("ban:%d", id)), adminButton("🗑 Удалить", fmt.Sprintf("account-delete:%d", id))})
		return text, adminKB(append(rows, []Button{adminButton("Назад", "users:0"), adminButton("⚙️ Админка", "home")})...)
	case "owned":
		subscriptions, err := q.ListTgLinksOf(ctx, id)
		if err != nil {
			return "Не удалось загрузить подписки.", adminKB(adminBack())
		}
		rows := [][]Button{}
		for _, subscription := range subscriptions {
			label := subscription.Name
			if user, err := q.GetUser(ctx, subscription.ID); err == nil && user.TariffID.Valid {
				if tariff, err := q.GetTariff(ctx, user.TariffID.Int64); err == nil {
					label = tariff.Name + " · " + subscription.Name
				}
			}
			rows = append(rows, []Button{adminButton(shortAdmin(fmt.Sprintf("📋 #%d · %s", subscription.ID, label), 60), fmt.Sprintf("user:%d", subscription.ID))})
		}
		text := fmt.Sprintf("<b>📋 Подписки пользователя: %d</b>\n\nВыберите подписку, чтобы посмотреть срок, трафик и ссылку подключения или изменить её.", len(subscriptions))
		if len(subscriptions) == 0 {
			text = "<b>📋 Подписки пользователя</b>\n\nУ пользователя пока нет подписок. Выдать подписку можно в его профиле."
		}
		return text, adminKB(append(rows, []Button{adminButton("← Профиль пользователя", fmt.Sprintf("contact:%d", id))}, adminBack())...)
	case "tariffs":
		tariffs, err := q.ListTariffs(ctx)
		if err != nil {
			return "Не удалось загрузить тарифы.", adminKB(adminBack())
		}
		active := []db.Tariff{}
		for _, t := range tariffs {
			if t.Archived == 0 {
				active = append(active, t)
			}
		}
		page := max(0, int(id))
		if len(active) > 0 {
			page = min(page, (len(active)-1)/10)
		} else {
			page = 0
		}
		rows := [][]Button{}
		for _, t := range active[min(page*10, len(active)):min(page*10+10, len(active))] {
			rows = append(rows, []Button{adminButton(shortAdmin(t.Name, 60), fmt.Sprintf("tariff:%d", t.ID))})
		}
		nav := []Button{}
		if page > 0 {
			nav = append(nav, adminButton("←", fmt.Sprintf("tariffs:%d", page-1)))
		}
		if (page+1)*10 < len(active) {
			nav = append(nav, adminButton("→", fmt.Sprintf("tariffs:%d", page+1)))
		}
		if len(nav) > 0 {
			rows = append(rows, nav)
		}
		rows = append(rows, adminBack())
		return "<b>Выберите тариф для новой подписки</b>\nСрок можно указать своим количеством дней.", adminKB(rows...)
	case "confirm":
		f, ok := b.adminFlow(chat, false)
		if !ok {
			return "Действие истекло. Начните заново.", adminKB(adminBack())
		}
		text := ""
		switch f.Kind {
		case "delete":
			text = fmt.Sprintf("Удалить подписку #%d? Ссылка и доступ перестанут работать.", f.UserID)
		case "extend":
			text = fmt.Sprintf("Изменить срок подписки #%d на %+d дней? Подписка будет включена, если аккаунт не заблокирован.", f.UserID, f.Days)
		case "create":
			text = fmt.Sprintf("Выдать новую подписку %s Telegram ID %d на %d дней?", html.EscapeString(f.Name), f.TgID, f.Days)
		}
		cancel := "home"
		if f.UserID > 0 {
			cancel = fmt.Sprintf("user:%d", f.UserID)
		} else if f.TgID > 0 {
			cancel = fmt.Sprintf("contact:%d", f.TgID)
		}
		return text, adminKB([]Button{adminButton("✅ Подтвердить", "commit:"+f.Nonce), adminButton("Отмена", cancel)})
	case "prompt":
		f, ok := b.adminFlow(chat, false)
		if !ok {
			return "Действие истекло.", adminKB(adminBack())
		}
		text := map[string]string{"extend": "⏳ Введите количество дней для изменения срока (от -36500 до 36500, кроме 0). Отрицательное число сокращает срок.", "recipient": "Введите числовой Telegram ID пользователя. Он должен сначала написать /start боту.", "name": "Введите название новой подписки (1–60 символов).", "days": "🎁 Введите срок новой подписки в днях (1–36500)."}[f.Kind]
		back := "home"
		if f.UserID > 0 {
			back = fmt.Sprintf("user:%d", f.UserID)
		} else if f.TgID > 0 {
			back = fmt.Sprintf("contact:%d", f.TgID)
		}
		return text, adminKB([]Button{adminButton("Назад", back), adminButton("⚙️ Админка", "home")})
	default:
		counts, err := domain.CountStates(ctx, q, b.d.Now())
		if err != nil {
			return "Не удалось загрузить статистику.", adminKB(adminBack())
		}
		config := b.Config(ctx).Admin
		stats, err := q.AdminTelegramStats(ctx)
		if err != nil {
			return "Не удалось загрузить статистику.", adminKB(adminBack())
		}
		rows := [][]Button{}
		for _, button := range config.Buttons {
			if !button.On {
				continue
			}
			target := map[string]string{"users": "users:0", "subscriptions": "subs:0", "orders": "orders:0", "broadcast": "broadcast", "maintenance": "maintenance", "refresh": "home"}[button.Action]
			if target == "" {
				continue
			}
			label := button.Label
			switch button.Action {
			case "users":
				label = fmt.Sprintf("%s (%d)", label, stats.Accounts)
			case "subscriptions":
				label = fmt.Sprintf("%s (%d)", label, stats.Keys)
			case "orders":
				label = fmt.Sprintf("%s (%d)", label, stats.Pending)
			}
			entry := adminButton(label, target)
			if len(rows) > 0 && len(rows[len(rows)-1]) < 2 {
				rows[len(rows)-1] = append(rows[len(rows)-1], entry)
			} else {
				rows = append(rows, []Button{entry})
			}
		}
		rows = append(rows, []Button{{Text: "Меню", CallbackData: "m"}})
		text := "⚙️ <b>Админ-панель</b>"
		if config.Statistics {
			text += fmt.Sprintf("\n\n👥 Пользователи: <b>%d</b> · заблокированы: <b>%d</b>\n🔑 Ключи: <b>%d</b> · активные: <b>%d</b>\n🧾 Заказы: <b>%d</b> · ожидают оплаты: <b>%d</b>\n\nИстекают: %d · истекли: %d · лимит: %d", stats.Accounts, stats.Banned, stats.Keys, counts.Active+counts.Expiring, stats.Orders, stats.Pending, counts.Expiring, counts.Expired, counts.Limited)
		}
		text += "\n\n<i>Обновлено " + b.d.Now().UTC().Format("15:04:05 UTC") + "</i>"
		return text, adminKB(rows...)
	}
}

func (b *Bot) adminPress(ctx context.Context, c *Client, out *Outbox, q *CallbackQuery) {
	chat := q.Message.Chat.ID
	b.answer(ctx, c, q.ID)
	if !b.isAdmin(ctx, q.From.ID, chat) {
		return
	}
	if b.flooding(chat) {
		return
	}
	cmd, arg, _ := strings.Cut(strings.TrimPrefix(q.Data, "a:"), ":")
	id, _ := strconv.ParseInt(arg, 10, 64)
	if !b.adminSectionAllowed(ctx, chat, strings.TrimPrefix(q.Data, "a:")) {
		return
	}
	data, notice := q.Data, ""
	if target, message, handled := b.adminNavigationAction(ctx, chat, cmd, arg); handled {
		data, notice = target, message
		cmd = ""
	}
	switch cmd {
	case "home", "users", "subs", "user", "contact", "owned", "devices", "orders", "order", "search", "grant", "extend", "delete", "freeze", "unfreeze", "link":
		b.adminFlow(chat, true)
	}
	switch cmd {
	case "grant":
		if id > 0 {
			if _, err := b.d.Store.Q.GetTgChat(ctx, id); err != nil {
				return
			}
			b.setAdminFlow(chat, adminFlow{Kind: "tariff", TgID: id})
			data = "a:tariffs:0"
		} else {
			data = "a:users:0"
			notice = "Выберите пользователя и нажмите «Выдать подписку» в его профиле."
		}
	case "extend", "delete":
		if _, err := b.d.Store.Q.GetUser(ctx, id); err != nil {
			notice = "Подписка не найдена."
			data = "a:home"
			break
		}
		b.setAdminFlow(chat, adminFlow{Kind: cmd, UserID: id})
		data = "a:prompt"
		if cmd == "delete" {
			data = "a:confirm"
		}
	case "freeze", "unfreeze":
		if b.d.Users == nil {
			notice = "Управление недоступно."
			break
		}
		disabled := cmd == "freeze"
		updated, err := b.d.Users.Update(ctx, id, domain.Patch{Disabled: &disabled})
		if err == nil {
			b.adminAudit(ctx, chat, cmd, id, 0)
			b.NotifySubscriptionChange(ctx, updated, 0, cmd, 0)
		}
		notice = adminResult(err, "Готово. Заморозка запрещает подключение, срок продолжает идти.")
		data = fmt.Sprintf("a:user:%d", id)
	case "tariff":
		f, ok := b.adminFlow(chat, false)
		if !ok || f.Kind != "tariff" {
			data = "a:home"
			notice = "Начните выдачу заново."
			break
		}
		t, err := b.d.Store.Q.GetTariff(ctx, id)
		if err != nil || t.Archived != 0 {
			notice = "Тариф недоступен."
			break
		}
		f.TariffID = id
		f.Kind = "days"
		f.Name = t.Name
		if account, err := b.d.Store.Q.GetTgChat(ctx, f.TgID); err == nil && account.FirstName != "" {
			f.Name = shortAdmin(account.FirstName, 60)
		}
		b.setAdminFlow(chat, f)
		data = "a:prompt"
	case "commit":
		f, ok := b.takeAdminConfirmation(chat, arg)
		if !ok {
			data = "a:home"
			notice = "Действие истекло или уже выполнено."
			break
		}
		// Consume before mutation: a repeated callback cannot issue or extend twice.
		data, notice = b.commitAdmin(ctx, out, chat, f)
	}
	out.Reply(chat, "admin-edit", 1, func(ctx context.Context, c *Client) error {
		text, kb := b.adminScreen(ctx, chat, data, notice)
		edited, err := b.editScreen(ctx, c, q.Message, text, kb)
		if err == nil {
			_ = b.d.Store.Q.SetTgMenu(ctx, db.SetTgMenuParams{MenuMsgID: edited, TgID: chat})
			return nil
		}
		var ae *APIError
		if errors.As(err, &ae) && ae.Code == 429 {
			return err
		}
		_, err = b.sendScreen(ctx, c, chat, text, kb)
		return err
	})
}

func (b *Bot) adminMessage(ctx context.Context, out *Outbox, m *Message) bool {
	if !b.isAdmin(ctx, m.From.ID, m.Chat.ID) {
		return false
	}
	chat := m.Chat.ID
	text := strings.TrimSpace(m.Text)
	data, notice := "", ""
	if text == "/admin" {
		b.adminFlow(chat, true)
		data = "a:home"
	} else {
		f, ok := b.adminFlow(chat, false)
		if !ok {
			return false
		}
		if strings.HasPrefix(text, "/") {
			b.adminFlow(chat, true)
			return false
		}
		data = "a:prompt"
		if !b.adminSectionAllowed(ctx, chat, "prompt") {
			b.adminFlow(chat, true)
			return true
		}
		switch f.Kind {
		case "recipient":
			id, err := strconv.ParseInt(text, 10, 64)
			if err != nil || id <= 0 || id > 9007199254740991 {
				notice = "Введите положительный числовой Telegram ID."
				break
			}
			if _, err := b.d.Store.Q.GetTgChat(ctx, id); err != nil {
				notice = "Пользователь ещё не написал /start этому боту."
				break
			}
			f.TgID = id
			f.Kind = "tariff"
			b.setAdminFlow(chat, f)
			data = "a:tariffs:0"
		case "name":
			if len([]rune(text)) < 1 || len([]rune(text)) > 60 {
				notice = "Название должно содержать 1–60 символов."
				break
			}
			f.Name = text
			f.Kind = "days"
			b.setAdminFlow(chat, f)
		case "days", "extend":
			days, err := strconv.ParseInt(text, 10, 64)
			if err != nil || days == 0 || days > 36500 || days < -36500 || f.Kind == "days" && days < 1 {
				notice = "Введите допустимое целое число дней: положительное для выдачи, отрицательное — только для сокращения срока."
				break
			}
			f.Days = days
			if f.Kind == "days" {
				f.Kind = "create"
			}
			b.setAdminFlow(chat, f)
			data = "a:confirm"
		case "devices":
			limit, err := strconv.ParseInt(text, 10, 64)
			if err != nil || limit < 0 || limit > 10000 {
				notice = "Введите целое число от 0 до 10000."
				break
			}
			patch := domain.Patch{DeviceLimit: &limit}
			if limit == 0 {
				patch = domain.Patch{ClearDeviceLimit: true}
			}
			updated, err := b.d.Users.Update(ctx, f.UserID, patch)
			if err == nil {
				b.adminFlow(chat, true)
				b.adminAudit(ctx, chat, "device-limit", f.UserID, limit)
				b.NotifySubscriptionChange(ctx, updated, 0, "devices", 0)
			}
			data = fmt.Sprintf("a:user:%d", f.UserID)
			notice = adminResult(err, "Лимит устройств сохранён.")
		case "broadcast":
			if len([]rune(text)) == 0 || len([]rune(text)) > 3000 {
				notice = "Текст должен содержать 1–3000 символов."
				break
			}
			f.Kind, f.Name = "broadcast-send", text
			b.setAdminFlow(chat, f)
			data = "a:confirm"
		default:
			return false
		}
	}
	out.Reply(chat, "admin-reply", 1, func(ctx context.Context, c *Client) error {
		text, kb := b.adminScreen(ctx, chat, data, notice)
		_, err := b.sendScreen(ctx, c, chat, text, kb)
		return err
	})
	return true
}

func (b *Bot) commitAdmin(ctx context.Context, out *Outbox, chat int64, f adminFlow) (string, string) {
	if !b.isAdmin(ctx, chat, chat) || b.d.Users == nil {
		return "a:home", "Управление недоступно."
	}
	if data, notice, handled := b.commitAdminNavigation(ctx, chat, f); handled {
		return data, notice
	}
	switch f.Kind {
	case "delete":
		previous, _ := b.d.Store.Q.GetUser(ctx, f.UserID)
		owner, _ := b.d.Store.Q.GetTgLink(ctx, f.UserID)
		err := b.d.Users.Delete(ctx, f.UserID)
		if err == nil {
			b.adminAudit(ctx, chat, "delete", f.UserID, 0)
			b.NotifySubscriptionChange(ctx, previous, owner.TgID, "delete", 0)
		}
		return "a:subs:0", adminResult(err, "Подписка удалена.")
	case "extend":
		updated, err := b.d.Users.Extend(ctx, f.UserID, f.Days)
		if err == nil {
			b.adminAudit(ctx, chat, "extend", f.UserID, f.Days)
			b.NotifySubscriptionChange(ctx, updated, 0, "extend", f.Days)
		}
		return fmt.Sprintf("a:user:%d", f.UserID), adminResult(err, "Подписка продлена.")
	case "create":
		var u db.User
		if err := b.d.Users.RefillFor(ctx, 1); err != nil {
			return "a:home", adminResult(err, "")
		}
		err := b.d.Store.Tx(ctx, func(q *db.Queries) error {
			count, err := q.CountTgLinksOf(ctx, f.TgID)
			if err != nil {
				return err
			}
			if count >= MaxLinks {
				return fmt.Errorf("У пользователя уже %d подписок", MaxLinks)
			}
			until := b.d.Now().Add(time.Duration(f.Days) * 24 * time.Hour)
			u, err = b.d.Users.CreateOn(ctx, q, domain.CreateInput{Name: f.Name, Contact: fmt.Sprintf("tg:%d", f.TgID), Note: "Выдано администратором Telegram", TariffID: f.TariffID, TermDays: sql.NullInt64{Int64: f.Days, Valid: true}}, domain.Patch{ExpiresAt: &until})
			if err != nil {
				return err
			}
			return q.LinkTg(ctx, db.LinkTgParams{UserID: u.ID, TgID: f.TgID, CreatedAt: b.d.Now().Unix()})
		})
		if err != nil {
			return "a:home", adminResult(err, "")
		}
		b.d.Users.Changed()
		b.adminAudit(ctx, chat, "create", u.ID, f.Days)
		b.NotifySubscriptionChange(ctx, u, f.TgID, "create", f.Days)
		return fmt.Sprintf("a:user:%d", u.ID), "Подписка создана и привязана к пользователю."
	}
	return "a:home", "Действие недоступно."
}

func (b *Bot) adminAudit(ctx context.Context, chat int64, action string, id, days int64) {
	raw, _ := json.Marshal(map[string]int64{"telegram_id": chat, "days": days})
	err := b.d.Store.Q.InsertAudit(ctx, db.InsertAuditParams{Ts: b.d.Now().Unix(), Action: "telegram.admin." + action, TargetType: sql.NullString{String: "user", Valid: true}, TargetID: sql.NullString{String: strconv.FormatInt(id, 10), Valid: true}, Details: sql.NullString{String: string(raw), Valid: true}})
	if err != nil && b.d.Log != nil {
		b.d.Log.Warn("telegram admin audit", "err", err)
	}
}
func adminResult(err error, ok string) string {
	if err != nil {
		return "Не удалось выполнить действие: " + err.Error()
	}
	return ok
}
func adminGB(n int64) string { return fmt.Sprintf("%.2f ГБ", float64(n)/1e9) }
func shortAdmin(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
func adminState(s string) string {
	if v, ok := map[string]string{"active": "активна", "expiring": "истекает", "expired": "истекла", "limited": "лимит", "disabled": "заморожена"}[s]; ok {
		return v
	}
	return s
}

func (b *Bot) adminSectionAllowed(ctx context.Context, chat int64, data string) bool {
	cfg := b.Config(ctx).Admin
	if !cfg.Enabled {
		return false
	}
	cmd, _, _ := strings.Cut(data, ":")
	switch cmd {
	case "orders", "order", "ordercheck", "orderclose", "orderrestore":
		return b.adminMenuActionEnabled(ctx, "orders")
	case "broadcast":
		return b.adminMenuActionEnabled(ctx, "broadcast")
	case "maintenance", "mainttoggle":
		return b.adminMenuActionEnabled(ctx, "maintenance")
	case "users":
		return cfg.Users
	case "contact", "owner", "owned":
		return cfg.Users || cfg.Subscriptions
	case "subs":
		return cfg.Subscriptions
	case "search":
		return false
	case "grant", "tariffs", "tariff":
		return cfg.Grant
	case "user", "extend", "delete", "freeze", "unfreeze", "link", "devices", "unbind", "clear", "limit":
		return cfg.Users || cfg.Subscriptions
	case "ban", "account-delete", "trial-reset":
		return cfg.Users
	case "confirm", "ok", "prompt", "commit":
		if flow, ok := b.adminFlow(chat, false); ok {
			switch flow.Kind {
			case "devices":
				return cfg.Users || cfg.Subscriptions
			case "account-delete", "trial-reset":
				return cfg.Users
			case "maintenance":
				return b.adminMenuActionEnabled(ctx, "maintenance")
			case "broadcast", "broadcast-send":
				return b.adminMenuActionEnabled(ctx, "broadcast")
			case "recipient", "tariff", "name", "days", "create":
				return cfg.Grant
			case "search":
				return false
			case "extend", "delete":
				return cfg.Users || cfg.Subscriptions
			}
		}
		// An already consumed confirmation still receives an expiry response.
		if cmd == "commit" {
			return true
		}
		return false
	default:
		return true
	}
}
