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

func adminKB(rows ...[]Button) *Keyboard   { return &Keyboard{InlineKeyboard: rows} }
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
	if !ok || f.Nonce != nonce || !f.Expires.After(b.d.Now()) || !(f.Kind == "delete" || f.Kind == "extend" && f.Days > 0 || f.Kind == "create") {
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
		rows = append(rows, []Button{adminButton("🔎 Найти пользователя", "search")}, adminBack())
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
		freezeLabel, freezeAction := "⏸ Заморозить", "freeze"
		if frozen {
			freezeLabel, freezeAction = "▶ Разморозить", "unfreeze"
		}
		rows := [][]Button{{adminButton(freezeLabel, fmt.Sprintf("%s:%d", freezeAction, id)), adminButton("📅 Продлить", fmt.Sprintf("extend:%d", id))}, {adminButton("🔗 Ссылка", fmt.Sprintf("link:%d", id)), adminButton("🗑 Удалить", fmt.Sprintf("delete:%d", id))}}
		if link, err := q.TgLinkOfUser(ctx, id); err == nil {
			rows = append(rows, []Button{adminButton("← Подписки пользователя", fmt.Sprintf("owned:%d", link.TgID))})
		} else {
			rows = append(rows, []Button{adminButton("← Все подписки", "subs:0")})
		}
		rows = append(rows, adminBack())
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
		rows = append(rows, []Button{adminButton(fmt.Sprintf("📋 Подписки пользователя (%d)", len(subscriptions)), fmt.Sprintf("owned:%d", id))})
		if cfg.Admin.Grant {
			rows = append(rows, []Button{adminButton("🎁 Выдать подписку", fmt.Sprintf("grant:%d", id))})
		}
		return text, adminKB(append(rows, []Button{adminButton("← Пользователи", "users:0")}, adminBack())...)
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
			text = fmt.Sprintf("Продлить подписку #%d на %d дней? Подписка будет включена.", f.UserID, f.Days)
		case "create":
			text = fmt.Sprintf("Выдать новую подписку %s Telegram ID %d на %d дней?", html.EscapeString(f.Name), f.TgID, f.Days)
		}
		return text, adminKB([]Button{adminButton("✅ Подтвердить", "commit:"+f.Nonce)}, []Button{adminButton("Отмена", "home")})
	case "prompt":
		f, ok := b.adminFlow(chat, false)
		if !ok {
			return "Действие истекло.", adminKB(adminBack())
		}
		text := map[string]string{"search": "Введите ID подписки или часть имени пользователя.", "extend": "Введите количество дней продления (1–36500).", "recipient": "Введите числовой Telegram ID пользователя. Он должен сначала написать /start боту.", "name": "Введите название новой подписки (1–60 символов).", "days": "Введите срок новой подписки в днях (1–36500)."}[f.Kind]
		return text, adminKB([]Button{adminButton("Отмена", "home")})
	default:
		counts, err := domain.CountStates(ctx, q, b.d.Now())
		if err != nil {
			return "Не удалось загрузить статистику.", adminKB(adminBack())
		}
		config := b.Config(ctx).Admin
		rows := [][]Button{}
		for _, button := range config.Buttons {
			if !button.On {
				continue
			}
			target := map[string]string{"users": "users:0", "subscriptions": "subs:0", "search": "search"}[button.Action]
			if target == "" {
				continue
			}
			entry := adminButton(button.Label, target)
			if button.Row && len(rows) > 0 && len(rows[len(rows)-1]) < 3 {
				rows[len(rows)-1] = append(rows[len(rows)-1], entry)
			} else {
				rows = append(rows, []Button{entry})
			}
		}
		rows = append(rows, []Button{{Text: "← Меню бота", CallbackData: "m"}})
		text := "<b>⚙ Администрирование mirai</b>"
		if config.Statistics {
			text += fmt.Sprintf("\nАктивные: %d · Истекают: %d\nИстекли: %d · Лимит: %d · Заморожены: %d", counts.Active, counts.Expiring, counts.Expired, counts.Limited, counts.Disabled)
		}
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
	switch cmd {
	case "home", "users", "subs", "user", "search", "grant", "extend", "delete", "freeze", "unfreeze", "link":
		b.adminFlow(chat, true)
	}
	switch cmd {
	case "search":
		b.setAdminFlow(chat, adminFlow{Kind: "search"})
		data = "a:prompt"
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
		_, err := b.d.Users.Update(ctx, id, domain.Patch{Disabled: &disabled})
		if err == nil {
			b.adminAudit(ctx, chat, cmd, id, 0)
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
		f.Kind = "name"
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
		err := c.Edit(ctx, chat, q.Message.MessageID, text, kb)
		if err == nil {
			return nil
		}
		var ae *APIError
		if errors.As(err, &ae) && ae.Code == 429 {
			return err
		}
		_, err = c.Send(ctx, chat, text, kb, false)
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
		case "search":
			users, err := b.d.Store.Q.ListUsers(ctx)
			if err != nil {
				notice = "Ошибка поиска."
				break
			}
			rows := [][]Button{}
			matches := 0
			accounts, _ := b.d.Store.Q.TelegramAccounts(ctx)
			identity := map[int64]string{}
			for _, account := range accounts {
				identity[account.UserID] = account.Username + " " + account.Name + " " + strconv.FormatInt(account.ID, 10)
				if account.UserID == 0 && (strconv.FormatInt(account.ID, 10) == text || strings.Contains(strings.ToLower(account.Name+" "+account.Username), strings.ToLower(text))) {
					matches++
					if len(rows) < 10 {
						rows = append(rows, []Button{adminButton(shortAdmin(account.Name+" @"+account.Username, 60), fmt.Sprintf("contact:%d", account.ID))})
					}
				}
			}
			for _, u := range users {
				if strconv.FormatInt(u.ID, 10) == text || strings.Contains(strings.ToLower(u.Name+" "+identity[u.ID]), strings.ToLower(text)) {
					matches++
					if len(rows) < 10 {
						rows = append(rows, []Button{adminButton(shortAdmin(fmt.Sprintf("#%d · %s", u.ID, u.Name), 60), fmt.Sprintf("user:%d", u.ID))})
					}
				}
			}
			rows = append(rows, adminBack())
			result := fmt.Sprintf("Найдено: %d. Показаны первые 10; уточните поиск при необходимости.", matches)
			out.Reply(chat, "admin-reply", 1, func(ctx context.Context, c *Client) error {
				if !b.isAdmin(ctx, chat, chat) {
					return nil
				}
				_, err := c.Send(ctx, chat, result, adminKB(rows...), false)
				return err
			})
			return true
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
			if err != nil || days < 1 || days > 36500 {
				notice = "Введите целое число дней от 1 до 36500."
				break
			}
			f.Days = days
			if f.Kind == "days" {
				f.Kind = "create"
			}
			b.setAdminFlow(chat, f)
			data = "a:confirm"
		default:
			return false
		}
	}
	out.Reply(chat, "admin-reply", 1, func(ctx context.Context, c *Client) error {
		text, kb := b.adminScreen(ctx, chat, data, notice)
		_, err := c.Send(ctx, chat, text, kb, false)
		return err
	})
	return true
}

func (b *Bot) commitAdmin(ctx context.Context, out *Outbox, chat int64, f adminFlow) (string, string) {
	if !b.isAdmin(ctx, chat, chat) || b.d.Users == nil {
		return "a:home", "Управление недоступно."
	}
	switch f.Kind {
	case "delete":
		err := b.d.Users.Delete(ctx, f.UserID)
		if err == nil {
			b.adminAudit(ctx, chat, "delete", f.UserID, 0)
		}
		return "a:subs:0", adminResult(err, "Подписка удалена.")
	case "extend":
		_, err := b.d.Users.Extend(ctx, f.UserID, f.Days)
		if err == nil {
			b.adminAudit(ctx, chat, "extend", f.UserID, f.Days)
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
		b.freshMenu(out, f.TgID, "🎁 Администратор выдал вам новую подписку: "+f.Name)
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
	case "users":
		return cfg.Users
	case "contact", "owner", "owned":
		return cfg.Users || cfg.Subscriptions || cfg.Search
	case "subs":
		return cfg.Subscriptions
	case "search":
		return cfg.Search
	case "grant", "tariffs", "tariff":
		return cfg.Grant
	case "user", "extend", "delete", "freeze", "unfreeze", "link":
		return cfg.Users || cfg.Subscriptions || cfg.Search
	case "confirm", "ok", "prompt":
		if flow, ok := b.adminFlow(chat, false); ok {
			switch flow.Kind {
			case "recipient", "tariff", "name", "days", "create":
				return cfg.Grant
			case "search":
				return cfg.Search
			case "extend", "delete":
				return cfg.Users || cfg.Subscriptions
			}
		}
		return false
	default:
		return true
	}
}
