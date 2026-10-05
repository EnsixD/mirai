package tgbot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"mirai/internal/panel/domain"
	"mirai/internal/panel/settings"
	"mirai/internal/panel/store/db"
)

// Callback data: m main, s subscription, d devices, dc:<id> confirm unbind, du:<id>
// unbind, c connect, r renew, p:<button> the admin's page, w switch list, u:<user> show
// that subscription.

// screen renders what the chat sees for a press: text (HTML) and buttons.
func (b *Bot) screen(ctx context.Context, cfg Config, chat int64, data, notice string) (string, *Keyboard) {
	if strings.HasPrefix(data, "a:") {
		return b.adminScreen(ctx, chat, data, notice)
	}
	if gate := b.botGate(ctx, chat); gate != "" {
		return gate, nil
	}
	w := wordsFor(cfg.Lang)
	list, u, ok := b.subs(ctx, chat)
	cmd, arg, _ := strings.Cut(data, ":")
	if cmd == "help" {
		return b.instruction(ctx, cfg)
	}
	if cmd == "orders" {
		return b.ordersScreen(ctx, chat, arg, false)
	}
	if cmd == "order" {
		id, _ := strconv.ParseInt(arg, 10, 64)
		return b.orderScreen(ctx, chat, id, false)
	}
	if cmd == "r" {
		rows := [][]Button{}
		for _, sub := range list {
			rows = append(rows, []Button{{Text: "🔑 " + shortAdmin(sub.Name, 48), CallbackData: "renewselect:" + strconv.FormatInt(sub.ID, 10)}})
		}
		rows = append(rows, []Button{{Text: "← Меню", CallbackData: "m"}})
		text := "🔄 <b>Продление</b>\n\nВыберите подписку, которую нужно продлить. Оплаченные дни добавятся к её оставшемуся сроку."
		if len(list) == 0 {
			text = "🔄 <b>Продление</b>\n\nУ вас пока нет подписок — продлевать нечего."
			rows = [][]Button{{{Text: "🛒 Купить", CallbackData: "b"}, {Text: "← Меню", CallbackData: "m"}}}
		}
		return text, &Keyboard{KeepRows: true, InlineKeyboard: rows}
	}
	if cmd == "b" {
		text, kb := b.shopList(ctx, w, w.buyTitle, "tn", notice, []Button{{Text: w.back, CallbackData: "m"}})
		if b.d.Billing != nil && b.d.Billing.TrialOpen(ctx, chat) {
			tariff, _ := b.d.Billing.TrialTariff(ctx)
			text = strings.ReplaceAll(text, html.EscapeString(w.payUnavailable), "")
			kb.InlineKeyboard = append([][]Button{{{Text: "🎁 Пробный · " + tariff.Name + " · бесплатно", CallbackData: "tr"}}}, kb.InlineKeyboard...)
			text += fmt.Sprintf("\n\n🎁 Пробный: %s · %d дн. · один раз на аккаунт", html.EscapeString(tariff.Name), tariff.DurationDays)
		}
		return text, kb
	}
	// Buying a new subscription works with or without one.
	if b.canBuyNew(ctx) {
		switch cmd {
		case "tn":
			return b.shopTariff(ctx, w, arg, "tn", "pn", shopBack(w, "tn", arg, "b"))
		case "pn":
			id, _, _ := strings.Cut(arg, ":")
			return b.shopInvoice(ctx, w, chat, 0, arg, []Button{{Text: w.back, CallbackData: "tn:" + id}})
		}
	}
	if cmd == "tr" {
		return b.takeTrial(ctx, w, chat)
	}
	if cmd == "pf" {
		return b.customerProfile(ctx, cfg, chat, list)
	}
	if cmd == "m" || cmd == "" {
		name := strconv.FormatInt(chat, 10)
		if account, err := b.d.Store.Q.GetTgChat(ctx, chat); err == nil && account.FirstName != "" {
			name = account.FirstName
		}
		text := render(pick(cfg.Texts.Welcome, w.welcome), map[string]string{"brand": b.brand(ctx), "name": name})
		if notice != "" {
			text = html.EscapeString(notice) + "\n\n" + text
		}
		kb := b.menu(ctx, cfg, w, len(list))
		b.addAdminButton(ctx, chat, kb)
		return text, kb
	}
	if !ok && cmd == "w" {
		return "📋 Мои подписки\n\nУ вас пока нет подписок.", &Keyboard{KeepRows: true, InlineKeyboard: [][]Button{{{Text: w.back, CallbackData: "pf"}}}}
	}
	if !ok {
		return b.welcome(ctx, cfg, w, chat, notice)
	}
	now := b.d.Now()
	vars := b.vars(ctx, w, u, now)
	back := []Button{{Text: w.back, CallbackData: "m"}}
	withNotice := func(s string) string {
		if notice != "" {
			return html.EscapeString(notice) + "\n\n" + s
		}
		return s
	}
	switch cmd {
	case "t":
		return b.shopTariff(ctx, w, arg, "t", "py", shopBack(w, "t", arg, "r"))
	case "py":
		id, _, _ := strings.Cut(arg, ":")
		return b.shopInvoice(ctx, w, chat, u.ID, arg, []Button{{Text: w.back, CallbackData: "t:" + id}})
	case "s":
		lines := []string{"<b>" + html.EscapeString(fmt.Sprintf(w.subTitle, u.Name)) + "</b>", html.EscapeString(vars["state"]), "",
			"📅 " + html.EscapeString(vars["term"]), "📦 " + html.EscapeString(vars["traffic"])}
		for _, p := range b.poolLines(ctx, w, u.ID) {
			lines = append(lines, "📦 "+html.EscapeString(p))
		}
		if r := vars["reset"]; r != "" {
			lines = append(lines, html.EscapeString(fmt.Sprintf(w.resets, r)))
		}
		lines = append(lines, "📱 "+html.EscapeString(vars["devices"]))
		if url := b.subURL(ctx, u); url != "" {
			lines = append(lines, "", "🔗 <b>Ссылка для подключения</b>", "<code>"+html.EscapeString(url)+"</code>", "", "Скопируйте ссылку и добавьте её в Happ, INCY или другое VPN-приложение. Храните её как пароль.")
		}
		rows := [][]Button{{{Text: fmt.Sprintf("📱 %s (%d)", w.devicesTitle, func() int64 { n, _ := b.d.Store.Q.CountBoundDevices(ctx, u.ID); return n }()), CallbackData: "d"}}}
		back = []Button{{Text: "← Мои подписки", CallbackData: "w"}}
		return withNotice(strings.Join(lines, "\n")), &Keyboard{KeepRows: true, InlineKeyboard: append(rows, back)}
	case "x", "xk", "xp":
		return b.trafficShop(ctx, w, chat, u, cmd, arg, notice)
	case "d", "dc", "da":
		id, _ := strconv.ParseInt(arg, 10, 64)
		return b.devices(ctx, w, u, cmd, id, notice, now, chat)
	case "c":
		text := "<b>" + w.connectTitle + "</b>\n\n" + fmt.Sprintf(w.connectText, html.EscapeString(b.subURL(ctx, u)))
		rows := [][]Button{}
		if btn, ok := b.pageButton(ctx, cfg, w, w.openPage); ok {
			rows = append(rows, []Button{btn})
		}
		return withNotice(text), &Keyboard{KeepRows: true, InlineKeyboard: append(rows, back)}
	case "rr":
		if offers, _ := b.offers(ctx); len(offers) > 0 {
			text, kb := b.shopList(ctx, w, fmt.Sprintf(w.renewTitle, u.Name), "t", notice, nil)
			text = render(pick(cfg.Texts.Renew, w.renew), vars) + "\n\n" + text
			if sup := b.supportURL(ctx); sup != "" {
				kb.InlineKeyboard = append(kb.InlineKeyboard, []Button{{Text: labelOf(cfg, "support", w.support), URL: sup}})
			}
			kb.InlineKeyboard = append(kb.InlineKeyboard, back)
			return text, kb
		}
		rows := [][]Button{}
		if sup := b.supportURL(ctx); sup != "" {
			rows = append(rows, []Button{{Text: labelOf(cfg, "support", w.support), URL: sup}})
		}
		return withNotice(render(pick(cfg.Texts.Renew, w.renew), vars)), &Keyboard{KeepRows: true, InlineKeyboard: append(rows, back)}
	case "p":
		for _, btn := range cfg.Buttons {
			if btn.Action == "page" && btn.ID == arg {
				return withNotice(render(btn.Text, vars)), &Keyboard{KeepRows: true, InlineKeyboard: [][]Button{back}}
			}
		}
	case "w":
		page, _ := strconv.Atoi(arg)
		pages := (len(list) + 7) / 8
		if page < 0 || page >= pages {
			page = 0
		}
		rows := [][]Button{}
		lines := []string{"<b>📋 Мои подписки</b>", "", "Выберите подписку, чтобы посмотреть срок, трафик, ссылку для подключения и устройства."}
		end := min((page+1)*8, len(list))
		for _, subscription := range list[page*8 : end] {
			rows = append(rows, []Button{{Text: "🔑 " + subscription.SubToken, CallbackData: "u:" + strconv.FormatInt(subscription.ID, 10)}})
		}
		nav := []Button{}
		if page > 0 {
			nav = append(nav, Button{Text: "←", CallbackData: "w:" + strconv.Itoa(page-1)})
		}
		if page+1 < pages {
			nav = append(nav, Button{Text: "→", CallbackData: "w:" + strconv.Itoa(page+1)})
		}
		if len(nav) > 0 {
			rows = append(rows, nav)
		}
		return strings.Join(lines, "\n"), &Keyboard{KeepRows: true, InlineKeyboard: append(rows, []Button{{Text: w.back, CallbackData: "pf"}})}
	}
	kb := b.menu(ctx, cfg, w, len(list))
	b.addAdminButton(ctx, chat, kb)
	return withNotice(render(pick(cfg.Texts.Main, w.main), vars)), kb
}

func (b *Bot) welcome(ctx context.Context, cfg Config, w *words, chat int64, notice string) (string, *Keyboard) {
	brand := b.brand(ctx)
	text := render(pick(cfg.Texts.Welcome, w.welcome), map[string]string{"brand": brand})
	if notice != "" {
		text = html.EscapeString(notice) + "\n\n" + text
	}
	var rows [][]Button
	profileLabel := "👤 Профиль"
	if cfg.Lang == "en" {
		profileLabel = "👤 Profile"
	}
	rows = append(rows, []Button{{Text: profileLabel, CallbackData: "pf"}})
	if b.d.Billing != nil && b.d.Billing.TrialOpen(ctx, chat) {
		rows = append(rows, []Button{{Text: w.trial, CallbackData: "tr"}})
	}
	if b.canBuyNew(ctx) {
		rows = append(rows, []Button{{Text: w.buy, CallbackData: "b"}})
	}
	if sup := b.supportURL(ctx); sup != "" {
		rows = append(rows, []Button{{Text: labelOf(cfg, "support", w.support), URL: sup}})
	}
	kb := &Keyboard{KeepRows: true, InlineKeyboard: rows}
	b.addAdminButton(ctx, chat, kb)
	rows = kb.InlineKeyboard
	if rows == nil {
		return text, nil
	}
	return text, &Keyboard{KeepRows: true, InlineKeyboard: rows}
}

// menu is the admin's main menu as buttons.
func (b *Bot) menu(ctx context.Context, cfg Config, w *words, subs int) *Keyboard {
	var rows [][]Button
	for _, mb := range cfg.Buttons {
		if !mb.On {
			continue
		}
		var btn Button
		switch mb.Action {
		case "profile":
			btn = Button{Text: mb.Label, CallbackData: "pf"}
		case "buy":
			btn = Button{Text: mb.Label, CallbackData: "b"}
		case "help":
			btn = Button{Text: mb.Label, CallbackData: "help"}
		case "devices":
			btn = Button{Text: mb.Label, CallbackData: "d"}
		case "connect":
			btn = Button{Text: mb.Label, CallbackData: "c"}
		case "renew":
			btn = Button{Text: mb.Label, CallbackData: "r"}
		case "support":
			sup := b.supportURL(ctx)
			if sup == "" {
				continue
			}
			btn = Button{Text: mb.Label, URL: sup}
		case "app":
			var ok bool
			if btn, ok = b.pageButton(ctx, cfg, w, mb.Label); !ok {
				continue
			}
		case "url":
			btn = Button{Text: mb.Label, URL: mb.URL}
		case "page":
			btn = Button{Text: mb.Label, CallbackData: "p:" + mb.ID}
		default:
			continue
		}
		if mb.Row && len(rows) > 0 && len(rows[len(rows)-1]) < 3 {
			rows[len(rows)-1] = append(rows[len(rows)-1], btn)
		} else {
			rows = append(rows, []Button{btn})
		}
	}
	return &Keyboard{KeepRows: true, InlineKeyboard: rows}
}

// pageButton opens the subscription page: in the Mini App when Telegram can load it,
// else in the browser.
func (b *Bot) pageButton(ctx context.Context, cfg Config, w *words, label string) (Button, bool) {
	if url := b.miniAppURL(ctx, cfg); url != "" {
		return Button{Text: label, WebApp: &WebApp{URL: url}}, true
	}
	return Button{}, false
}

func (b *Bot) devices(ctx context.Context, w *words, u db.User, cmd string, id int64, notice string, now time.Time, chat int64) (string, *Keyboard) {
	back := []Button{{Text: w.back, CallbackData: "s"}}
	binding, _ := b.d.Settings.On(ctx, settings.DeviceBinding)
	head := "<b>" + w.devicesTitle + "</b> · " + html.EscapeString(b.vars(ctx, w, u, now)["devices"])
	if !binding {
		limit := "∞"
		if u.DeviceLimit.Valid {
			limit = strconv.FormatInt(u.DeviceLimit.Int64, 10)
		}
		return head + "\n\n" + html.EscapeString(fmt.Sprintf(w.devicesOff, limit)), &Keyboard{KeepRows: true, InlineKeyboard: [][]Button{back}}
	}
	devs, err := b.d.Store.Q.ListBoundDevices(ctx, u.ID)
	if err != nil {
		devs = nil
	}
	name := func(d db.BoundDevice) string { return deviceName(w, d) }
	policy, _ := domain.ResetPolicy(ctx, b.d.Store.Q)
	admin := b.isAdmin(ctx, chat, chat)
	if cmd == "da" {
		return "Очистить все устройства этой подписки?", &Keyboard{KeepRows: true, InlineKeyboard: [][]Button{{{Text: "Очистить все", CallbackData: "dua"}, {Text: w.cancel, CallbackData: "d"}}}}
	}
	if cmd == "dc" {
		for _, d := range devs {
			if d.ID == id {
				return html.EscapeString(fmt.Sprintf(w.confirmUnbind, name(d))), &Keyboard{KeepRows: true, InlineKeyboard: [][]Button{
					{{Text: w.yesUnbind, CallbackData: "du:" + strconv.FormatInt(id, 10)}, {Text: w.cancel, CallbackData: "d"}}}}
			}
		}
	}
	lines := []string{head, ""}
	if notice != "" {
		lines = append([]string{html.EscapeString(notice), ""}, lines...)
	}
	rows := [][]Button{}
	if len(devs) == 0 {
		lines = append(lines, html.EscapeString(w.devicesNone))
	}
	traffic, _ := b.d.Store.Q.DeviceTrafficOf(ctx, u.ID)
	for i, d := range devs {
		meta := []string{}
		if d.Hwid != "" && d.Model != "" && d.Os != "" {
			meta = append(meta, strings.TrimSpace(d.Os+" "+d.OsVersion))
		}
		if app, _, _ := strings.Cut(strings.TrimSpace(d.App), " "); app != "" {
			meta = append(meta, strings.Replace(app, "/", " ", 1))
		}
		meta = append(meta, w.ago(time.Unix(d.LastSeen, 0), now))
		if count, ok := traffic[d.ID]; ok {
			meta = append(meta, fmt.Sprintf("↑ %.2f GB · ↓ %.2f GB", float64(count.Up)/(1<<30), float64(count.Down)/(1<<30)))
		}
		lines = append(lines, fmt.Sprintf("%d. %s — %s", i+1, html.EscapeString(name(d)), html.EscapeString(strings.Join(meta, " · "))))
		if policy.Single || admin {
			rows = append(rows, []Button{{Text: "📱 " + name(d), CallbackData: "du:" + strconv.FormatInt(d.ID, 10)}})
		}
	}
	if len(devs) > 0 {
		if policy.All || admin {
			rows = append(rows, []Button{{Text: "🧹 Очистить все устройства", CallbackData: "da"}})
		}
		if !admin {
			lines = append(lines, "", fmt.Sprintf("Лимит: %d очистки за %d дней. Очистка всех устройств считается одной операцией.", policy.Limit, policy.PeriodDays))
		}
	}
	return strings.Join(lines, "\n"), &Keyboard{KeepRows: true, InlineKeyboard: append(rows, back)}
}

func deviceName(w *words, d db.BoundDevice) string {
	switch {
	case d.Hwid == "":
		return w.sharedPlace
	case d.Model != "":
		return d.Model
	case d.Os != "":
		return strings.TrimSpace(d.Os + " " + d.OsVersion)
	}
	return w.device
}

// act does what a tap changes — unbinding a device, showing another subscription — and
// says which screen to draw after it, with a line on top. A tap's effect never waits in
// the outbox: only its screen does, and a later tap may replace that.
func (b *Bot) act(ctx context.Context, chat int64, data string) (screen, notice string) {
	if gate := b.botGate(ctx, chat); gate != "" {
		return "m", ""
	}
	cmd, arg, _ := strings.Cut(data, ":")
	id, _ := strconv.ParseInt(arg, 10, 64)
	switch cmd {
	case "ordercheck", "orderclose":
		return b.orderAction(ctx, chat, id, false, cmd == "orderclose")
	case "renewselect":
		list, _, _ := b.subs(ctx, chat)
		for _, sub := range list {
			if sub.ID == id {
				_ = b.d.Store.Q.SetTgCurrent(ctx, db.SetTgCurrentParams{Current: id, TgID: chat})
				return "rr", ""
			}
		}
		return "r", "Подписка не найдена."
	case "dua":
		_, u, ok := b.subs(ctx, chat)
		if !ok {
			return "m", ""
		}
		err := b.d.Devices.UnbindAll(ctx, u.ID, !b.isAdmin(ctx, chat, chat))
		if err != nil {
			return "d", "Очистка недоступна: проверьте лимит и настройки."
		}
		return "d", "Все устройства очищены."
	case "du":
		w := wordsFor(b.Config(ctx).Lang)
		_, u, ok := b.subs(ctx, chat)
		if !ok {
			return "m", ""
		}
		devs, _ := b.d.Store.Q.ListBoundDevices(ctx, u.ID)
		for _, d := range devs {
			if d.ID != id {
				continue
			}
			err := b.d.Devices.Unbind(ctx, u.ID, id, !b.isAdmin(ctx, chat, chat))
			switch {
			case err == nil:
				return "d", fmt.Sprintf(w.unbound, deviceName(w, d))
			case errors.Is(err, domain.ErrUnbindCooldown):
				next, _ := b.d.Devices.NextReset(ctx, u.ID)
				return "d", fmt.Sprintf(w.wait, b.when(w, next))
			case errors.Is(err, domain.ErrResetDisabled):
				return "d", "Очистка устройств отключена администратором."
			}
		}
		return "d", ""
	case "u":
		list, _, _ := b.subs(ctx, chat)
		for _, s := range list {
			if s.ID == id {
				_ = b.d.Store.Q.SetTgCurrent(ctx, db.SetTgCurrentParams{Current: id, TgID: chat})
			}
		}
		return "s", ""
	}
	return data, ""
}

func (b *Bot) when(w *words, t time.Time) string {
	if t.IsZero() {
		return w.justNow
	}
	return w.shortDate(t) + " " + t.Format("15:04") + " UTC"
}

// vars are the {variables} of the admin's texts for one subscription.
func (b *Bot) vars(ctx context.Context, w *words, u db.User, now time.Time) map[string]string {
	grants, err := domain.UserGrantsLeft(ctx, b.d.Store.Q, u.ID, now)
	if err != nil {
		b.d.Log.Warn("telegram: traffic packages", "err", err)
	}
	extra := grants.Main(u.ID)
	state := domain.State(u, extra, now)
	v := map[string]string{"subscription_url": b.subURL(ctx, u), "name": u.Name, "brand": b.brand(ctx), "until": w.forever, "days": "—", "term": w.forever, "reset": ""}
	switch state {
	case domain.StateActive:
		v["state"] = w.stateActive
	case domain.StateExpiring:
		v["state"] = w.stateExpiring
	case domain.StateLimited:
		v["state"] = w.stateLimited
	case domain.StateExpired:
		v["state"] = w.stateExpired
	default:
		v["state"] = w.stateOff
	}
	if u.ExpiresAt.Valid {
		exp := time.Unix(u.ExpiresAt.Int64, 0).UTC()
		left := int((exp.Sub(now) + 24*time.Hour - time.Second) / (24 * time.Hour))
		v["until"] = w.date(exp)
		v["days"] = w.days(max(0, left))
		v["term"] = fmt.Sprintf(w.termUntil, v["until"], v["days"])
	}
	used := u.UsedUp + u.UsedDown
	v["used"] = w.bytes(used)
	if u.TrafficLimit.Valid {
		v["limit"] = w.bytes(u.TrafficLimit.Int64)
		v["left"] = w.bytes(domain.TrafficLeft(u.TrafficLimit, used, extra))
		v["traffic"] = fmt.Sprintf(w.trafficOf, v["used"], w.withPackages(u.TrafficLimit.Int64, extra))
	} else {
		v["limit"], v["left"] = w.noLimit, w.noLimit
		v["traffic"] = fmt.Sprintf(w.trafficNoLimit, v["used"])
	}
	if t, ok := domain.NextReset(u, now); ok {
		v["reset"] = w.shortDate(t)
	}
	n, _ := b.d.Store.Q.CountBoundDevices(ctx, u.ID)
	v["devices"] = strconv.FormatInt(n, 10)
	if u.DeviceLimit.Valid {
		v["devices"] = fmt.Sprintf(w.trafficOf, v["devices"], strconv.FormatInt(u.DeviceLimit.Int64, 10))
	}
	return v
}

func (b *Bot) brand(ctx context.Context) string {
	if s, _ := b.d.Settings.String(ctx, settings.KeyBrand); s != "" {
		return s
	}
	return "VPN"
}

// supportURL is the panel's support link when Telegram can open it.
func (b *Bot) supportURL(ctx context.Context) string {
	s, _ := b.d.Settings.String(ctx, settings.KeySupportURL)
	if safeURL(s) {
		return s
	}
	return ""
}

func (b *Bot) subURL(ctx context.Context, u db.User) string {
	if base := b.d.SubBase(ctx); base != "" {
		return base + "/" + u.SubToken
	}
	return ""
}

// poolLines: one line per traffic pool with a limit — "WL: 30 GB of 100 GB + packages 50 GB".
func (b *Bot) poolLines(ctx context.Context, w *words, userID int64) []string {
	rows, err := b.d.Store.Q.ListUserPools(ctx, userID)
	if err != nil || len(rows) == 0 {
		return nil
	}
	pools, err := b.d.Store.Q.ListTrafficPools(ctx)
	if err != nil {
		return nil
	}
	grants, err := domain.UserGrantsLeft(ctx, b.d.Store.Q, userID, b.d.Now())
	if err != nil {
		return nil
	}
	names := map[int64]string{}
	for _, p := range pools {
		names[p.ID] = p.Name
	}
	var out []string
	for _, r := range rows {
		if !r.TrafficLimit.Valid {
			continue
		}
		extra := grants.Pool(userID, r.PoolID)
		line := names[r.PoolID] + ": " + fmt.Sprintf(w.trafficOf, w.bytes(r.UsedUp+r.UsedDown), w.withPackages(r.TrafficLimit.Int64, extra))
		if domain.PoolExhausted(r, extra) {
			line += " — " + w.poolOut
		}
		out = append(out, line)
	}
	return out
}
