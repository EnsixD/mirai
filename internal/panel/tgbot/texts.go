package tgbot

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
)

// The bot's own words in both languages. The admin's texts replace the ones marked
// "admin" when set.
type words struct {
	welcome, main, renew, expiring, expired, traffic90, trafficEnd string // admin

	back, yesUnbind, cancel, subscriptions, openPage, support, promo   string
	subTitle, devicesTitle, connectTitle, switchTitle                  string
	stateActive, stateExpiring, stateLimited, stateExpired, stateOff   string
	forever, termUntil, noLimit, trafficOf, trafficNoLimit, resets     string
	devicesNone, devicesOff, devicesNote, confirmUnbind, unbound, wait string
	connectText, linked, alreadyLinked, linkExpired, linkInvalid       string
	linkLimit, noSub, sharedPlace, device, justNow                     string
	// A subscription already linked to another account is moved only if its owner agrees.
	transferAsk, transferAllow, transferDeny, transferAsked, transferBusy, transferDone string
	transferDenied, transferDeniedNew, transferStale, transferNotice                    string
	minAgo, hoursAgo, yesterday, daysAgo                                                string
	months                                                                              [12]string
	commands                                                                            string
	// menuApp is the Mini App button next to the input field. It shares the row with the
	// field: a long one leaves no room to type on a phone.
	menuApp string

	// The shop.
	buy, buyTitle, renewTitle, payHow, payStars, payCard, payCrypto, payAddon, payButton, invoice, payNew, payRenew string
	notForSale, payUnavailable, tooManyInvoices, payStale, paidNew, paidRenew                                       string
	trial, trialDone, trialUsed, trialOff, trialOpen, trialFail                                                     string
	pickTerm, priceFrom                                                                                             string
	poolOut                                                                                                         string // a traffic pool used up

	// Traffic packages.
	buyTraffic, trafficTitle, payPackage, packageGone, paidPackage, plusPackages string
}

var ru = words{
	welcome:    "🌐 <b>{brand}</b>\n<i>Удобный доступ к интернету — всё нужное в одном месте.</i>\n\n📱 <b>Подключайтесь на своих устройствах</b>\nДобавьте ссылку подписки в Happ, INCY или другое совместимое приложение. Доступный трафик и количество устройств зависят от выбранного тарифа.\n\n👤 <b>Ваш личный кабинет</b>\nВ профиле — история покупок, в «Моих подписках» — сроки, ссылки и управление устройствами.\n\n🛒 <b>Начните с подходящего тарифа</b>\nНовая подписка — в разделе «Купить», продление действующей — в разделе «Продлить».\n\n👇 Выберите действие ниже.",
	main:       "🌐 <b>{brand}</b>\n<i>Удобный доступ к интернету — всё нужное в одном месте.</i>\n\n📱 <b>Подключайтесь на своих устройствах</b>\nДобавьте ссылку подписки в Happ, INCY или другое совместимое приложение. Доступный трафик и количество устройств зависят от выбранного тарифа.\n\n👤 <b>Ваш личный кабинет</b>\nВ профиле — история покупок, в «Моих подписках» — сроки, ссылки и управление устройствами.\n\n🛒 <b>Начните с подходящего тарифа</b>\nНовая подписка — в разделе «Купить», продление действующей — в разделе «Продлить».\n\n👇 Выберите действие ниже.",
	renew:      "💳 <b>Продление подписки</b>\n\n🔗 {subscription_url}\n📅 Действует до: {until}\n\nВыберите срок и завершите оплату. Новые дни добавятся к оставшемуся сроку — действующие дни не потеряются, а ссылка останется прежней.\n\nЕсли доступ уже закончился, продление вернёт его после подтверждения оплаты. Если тарифы недоступны, обратитесь в поддержку.",
	expiring:   "⏳ <b>Подписка скоро закончится</b>\n\n🔗 {subscription_url}\n📅 Окончание: {until}\n🕒 Осталось: {days}\n\nЧтобы продолжить пользоваться VPN без перерыва, откройте «Продлить» в главном меню и выберите срок. Оплаченные дни добавятся к оставшимся, а ссылка для подключения сохранится.",
	expired:    "⛔️ <b>Срок подписки закончился</b>\n\n🔗 {subscription_url}\n📅 Доступ был оплачен до: {until}\n\nПодключение приостановлено. Откройте «Продлить» в главном меню, выберите срок и завершите оплату. После подтверждения доступ восстановится — заново добавлять ссылку в приложение не нужно.\n\nЕсли нужна помощь с продлением, напишите в поддержку.",
	traffic90:  "📊 <b>Осталось немного трафика</b>\n\n🔗 {subscription_url}\nИзрасходовано 90% доступного объёма.\n📦 Использовано: {used} из {limit}\n🟢 Остаток: {left}\n\nПроверьте расход в «Моих подписках». Если для вашего тарифа доступны пакеты трафика, можно добавить объём до следующего сброса.",
	trafficEnd: "📦 <b>Трафик на этот период закончился</b>\n\n🔗 {subscription_url}\n📊 Использовано: {used} из {limit}\n🔄 Следующий сброс: {reset}\n\nОткройте подписку в профиле, чтобы проверить условия тарифа и доступные пакеты трафика. Если сброс не предусмотрен или нужна помощь, обратитесь в поддержку.",

	back: "⬅️ Назад", promo: "🎟 Промокоды", yesUnbind: "✅ Да, отвязать", cancel: "↩️ Отмена", subscriptions: "🔁 Подписки", openPage: "🌐 Открыть страницу подписки", support: "💬 Поддержка",
	subTitle: "Подписка «%s»", devicesTitle: "Устройства", connectTitle: "Подключить устройство", switchTitle: "Какую подписку показать?",
	stateActive: "✅ Работает", stateExpiring: "⏳ Скоро закончится", stateLimited: "📦 Трафик на этот период закончился", stateExpired: "⛔️ Подписка закончилась", stateOff: "⏸ Доступ приостановлен",
	forever: "бессрочно", termUntil: "до %s — осталось %s", noLimit: "без лимита", trafficOf: "%s из %s", trafficNoLimit: "%s, без лимита", resets: "🔄 Обновится: %s",
	devicesNone:       "Устройства появятся здесь, когда приложение загрузит подписку.",
	devicesOff:        "Привязка устройств выключена: одновременно можно подключаться не больше чем с %s адресов.",
	devicesNote:       "Отвязанное устройство сразу отключается, а место освобождается. Отвязывать можно одно устройство в сутки.",
	confirmUnbind:     "Отвязать «%s»? Оно сразу отключится.",
	unbound:           "✅ «%s» отвязано.",
	wait:              "⏳ Следующее устройство можно отвязать %s.",
	connectText:       "1. Установите приложение: Happ (iPhone, Android), ClashFest (Android), Koala Clash (Windows), SlothClash (Windows, Mac, Linux) или другое со страницы подписки.\n2. Добавьте в него ссылку:\n<code>%s</code>\n\nНа странице подписки — кнопки «Добавить» для всех приложений.",
	linked:            "✅ Подписка «%s» подключена.",
	alreadyLinked:     "Подписка «%s» уже здесь.",
	linkExpired:       "Ссылка устарела. Обновите страницу подписки и нажмите «Открыть в Telegram» ещё раз.",
	linkInvalid:       "Ссылка не подошла. Откройте страницу подписки и нажмите «Открыть в Telegram».",
	linkLimit:         "К одному аккаунту можно подключить не больше %d подписок.",
	transferAsk:       "Подписку «%s» хотят подключить к другому аккаунту Telegram (%s). Если это вы, разрешите. Если нет, откажите: подписка останется у вас.",
	transferAllow:     "✅ Разрешить",
	transferDeny:      "❌ Отказать",
	transferAsked:     "Эта подписка уже подключена к другому аккаунту. Мы спросили владельца: когда он разрешит, подписка появится здесь. Запрос действует час.",
	transferBusy:      "Для этой подписки уже есть запрос от другого аккаунта. Попробуйте позже.",
	transferDone:      "Готово: подписка «%s» теперь подключена к другому аккаунту.",
	transferDenied:    "Отказ записан, подписка «%s» осталась у вас.",
	transferDeniedNew: "Владелец подписки не разрешил подключение.",
	transferStale:     "Запрос уже не действует.",
	transferNotice:    "Подписка «%s» теперь подключена к этому аккаунту.",
	noSub:             "Подписка не найдена.",
	sharedPlace:       "Приложения без ID устройства", device: "Устройство",
	justNow: "только что", minAgo: "%d мин назад", hoursAgo: "%d ч назад", yesterday: "вчера", daysAgo: "%s назад",
	months:   [12]string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"},
	commands: "Главное меню",
	menuApp:  "Подписка",

	buy: "🛒 Купить подписку", buyTitle: "Выберите тариф", renewTitle: "Продление подписки «%s»: выберите тариф",
	payHow: "Как оплатить?", payStars: "⭐ Telegram Stars — %s", payCard: "💳 Картой или СБП — %s", payCrypto: "🪙 Криптовалютой — %s", payAddon: "💳 %s — %s", payButton: "Оплатить %s",
	invoice:         "Счёт: «%s» — %s.\n\nОплатите по кнопке ниже: %s сразу после оплаты, бот пришлёт сообщение.",
	payNew:          "подписка будет готова",
	payRenew:        "подписка продлится",
	notForSale:      "Этот тариф больше не продаётся.",
	pickTerm:        "На какой срок?",
	priceFrom:       "от %s",
	payUnavailable:  "Оплата сейчас недоступна. Попробуйте позже или напишите в поддержку.",
	tooManyInvoices: "Слишком много счетов подряд. Попробуйте через час.",
	payStale:        "Счёт устарел. Откройте меню бота и оплатите заново.",
	trial:           "🎁 Попробовать бесплатно",
	trialDone:       "🎁 Пробная подписка готова: %s.\n\nСсылка и инструкции в меню.",
	trialUsed:       "Пробный период даётся один раз, и только тем, у кого ещё не было подписки.",
	trialOff:        "Пробный период сейчас недоступен.",
	trialOpen:       "📱 Открыть подписку",
	trialFail:       "Не получилось выдать пробный период. Попробуйте позже или напишите в поддержку.",
	paidNew:         "✅ Оплата получена — подписка «%s» готова (тариф «%s»).\n\nДобавьте ссылку в приложение:\n<code>%s</code>",
	paidRenew:       "✅ Оплата получена — подписка «%s» продлена до %s.",
	poolOut:         "закончился до сброса",

	buyTraffic:   "📦 Докупить трафик",
	trafficTitle: "Трафик для подписки «%s»: выберите пакет",
	payPackage:   "трафик начислится",
	packageGone:  "Этот пакет больше не продаётся.",
	paidPackage:  "✅ Оплата получена — пакет «%s» начислен на подписку «%s».",
	plusPackages: "%s + пакеты %s",
}

var en = words{
	welcome:    "🌐 <b>{brand}</b>\n<i>Convenient internet access, managed in one place.</i>\n\n📱 <b>Connect your devices</b>\nAdd your subscription link to Happ, INCY or another compatible app. Traffic and device limits depend on your plan.\n\n👤 <b>Your account</b>\nYour profile shows purchase totals. My subscriptions contains expiration dates, connection links and device management.\n\n🛒 <b>Choose your next step</b>\nBuy opens new plans; Renew extends an existing subscription.\n\n👇 Choose an action below.",
	main:       "🌐 <b>{brand}</b>\n<i>Convenient internet access, managed in one place.</i>\n\n📱 <b>Connect your devices</b>\nAdd your subscription link to Happ, INCY or another compatible app. Traffic and device limits depend on your plan.\n\n👤 <b>Your account</b>\nYour profile shows purchase totals. My subscriptions contains expiration dates, connection links and device management.\n\n🛒 <b>Choose your next step</b>\nBuy opens new plans; Renew extends an existing subscription.\n\n👇 Choose an action below.",
	renew:      "💳 <b>Renew your subscription</b>\n\n🔗 {subscription_url}\n📅 Valid until: {until}\n\nChoose a duration and complete payment. Purchased days are added to your remaining time, and your connection link stays the same.\n\nExpired access resumes after payment confirmation. If plans are unavailable, contact support.",
	expiring:   "⏳ <b>Your subscription expires soon</b>\n\n🔗 {subscription_url}\n📅 Expires: {until}\n🕒 Remaining: {days}\n\nOpen Renew in the main menu to keep your access uninterrupted. Purchased days are added to the remaining time; your connection link stays the same.",
	expired:    "⛔️ <b>Your subscription has expired</b>\n\n🔗 {subscription_url}\n📅 Paid until: {until}\n\nAccess is paused. Open Renew, choose a duration and complete payment. Access resumes after confirmation, without adding the link to your app again.\n\nContact support if you need help.",
	traffic90:  "📊 <b>Your traffic allowance is nearly used up</b>\n\n🔗 {subscription_url}\n90% of your allowance has been used.\n📦 Used: {used} of {limit}\n🟢 Remaining: {left}\n\nCheck your usage in My subscriptions. If traffic packages are available for your plan, you can add more data before the next reset.",
	trafficEnd: "📦 <b>Your traffic allowance has been used up</b>\n\n🔗 {subscription_url}\n📊 Used: {used} of {limit}\n🔄 Next reset: {reset}\n\nOpen your subscription to review plan terms and available traffic packages. Contact support if your plan has no reset or you need help.",

	back: "⬅️ Back", promo: "🎟 Promo codes", yesUnbind: "✅ Yes, unbind", cancel: "↩️ Cancel", subscriptions: "🔁 Subscriptions", openPage: "🌐 Open the subscription page", support: "💬 Support",
	subTitle: "Subscription “%s”", devicesTitle: "Devices", connectTitle: "Connect a device", switchTitle: "Which subscription to show?",
	stateActive: "✅ Working", stateExpiring: "⏳ Ends soon", stateLimited: "📦 Traffic for this period is used up", stateExpired: "⛔️ The subscription has ended", stateOff: "⏸ Access is paused",
	forever: "no end date", termUntil: "until %s — %s left", noLimit: "unlimited", trafficOf: "%s of %s", trafficNoLimit: "%s, unlimited", resets: "🔄 Renews: %s",
	devicesNone:       "Devices appear here once an app loads the subscription.",
	devicesOff:        "Device binding is off: at most %s addresses can be connected at once.",
	devicesNote:       "An unbound device is disconnected at once and its place frees up. You can unbind one device a day.",
	confirmUnbind:     "Unbind “%s”? It disconnects at once.",
	unbound:           "✅ “%s” is unbound.",
	wait:              "⏳ You can unbind the next device %s.",
	connectText:       "1. Install an app: Happ (iPhone, Android), ClashFest (Android), Koala Clash (Windows), SlothClash (Windows, Mac, Linux) or another from the subscription page.\n2. Add this link to it:\n<code>%s</code>\n\nThe subscription page has “Add” buttons for every app.",
	linked:            "✅ Subscription “%s” is connected.",
	alreadyLinked:     "Subscription “%s” is already here.",
	linkExpired:       "The link has expired. Reload the subscription page and tap “Open in Telegram” again.",
	linkInvalid:       "The link did not work. Open the subscription page and tap “Open in Telegram”.",
	linkLimit:         "One account can hold at most %d subscriptions.",
	transferAsk:       "Someone wants to connect subscription “%s” to another Telegram account (%s). If that is you, allow it. If not, refuse: the subscription stays with you.",
	transferAllow:     "✅ Allow",
	transferDeny:      "❌ Refuse",
	transferAsked:     "This subscription is already connected to another account. We asked its owner: once they allow it, the subscription appears here. The request is good for an hour.",
	transferBusy:      "There is already a request for this subscription from another account. Try again later.",
	transferDone:      "Done: subscription “%s” is now connected to the other account.",
	transferDenied:    "Refused: subscription “%s” stays with you.",
	transferDeniedNew: "The owner of the subscription did not allow it.",
	transferStale:     "This request is no longer valid.",
	transferNotice:    "Subscription “%s” is now connected to this account.",
	noSub:             "Subscription not found.",
	sharedPlace:       "Apps without a device ID", device: "Device",
	justNow: "just now", minAgo: "%d min ago", hoursAgo: "%d h ago", yesterday: "yesterday", daysAgo: "%s ago",
	months:   [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
	commands: "Main menu",
	menuApp:  "Subscription",

	buy: "🛒 Buy a subscription", buyTitle: "Pick a plan", renewTitle: "Renew subscription “%s”: pick a plan",
	payHow: "How would you like to pay?", payStars: "⭐ Telegram Stars — %s", payCard: "💳 Card or SBP — %s", payCrypto: "🪙 Crypto — %s", payAddon: "💳 %s — %s", payButton: "Pay %s",
	invoice:         "Invoice: “%s” — %s.\n\nPay with the button below: the %s as soon as it is paid, and the bot will message you.",
	payNew:          "subscription is ready",
	payRenew:        "subscription is renewed",
	notForSale:      "This plan is no longer sold.",
	pickTerm:        "For how long?",
	priceFrom:       "from %s",
	payUnavailable:  "Payment is not available right now. Try later or message support.",
	tooManyInvoices: "Too many invoices in a row. Try again in an hour.",
	payStale:        "The invoice is out of date. Open the bot's menu and pay again.",
	trial:           "🎁 Try it for free",
	trialDone:       "🎁 Your trial subscription is ready: %s.\n\nThe link and instructions are in the menu.",
	trialUsed:       "The trial is given once, and only to people who have not had a subscription.",
	trialOff:        "The trial is not available now.",
	trialOpen:       "📱 Open the subscription",
	trialFail:       "Could not give the trial. Try later or message support.",
	paidNew:         "✅ Payment received — subscription “%s” is ready (plan “%s”).\n\nAdd the link to your app:\n<code>%s</code>",
	paidRenew:       "✅ Payment received — subscription “%s” is renewed until %s.",
	poolOut:         "used up until the reset",

	buyTraffic:   "📦 Buy more traffic",
	trafficTitle: "Traffic for subscription “%s”: pick a package",
	payPackage:   "traffic is added",
	packageGone:  "This package is no longer sold.",
	paidPackage:  "✅ Payment received — package “%s” is added to subscription “%s”.",
	plusPackages: "%s + packages %s",
}

func wordsFor(lang string) *words {
	if lang == "en" {
		return &en
	}
	return &ru
}

// render fills {variables} into an admin's (or built-in) text. The text itself is plain:
// it is escaped for Telegram's HTML, and so are the values.
//
// One pass over the text: a value that itself holds {something} is not filled again, and
// the result does not depend on the order the variables are listed in.
func render(text string, vars map[string]string) string {
	rest := telegramTemplate(text)
	var out strings.Builder
	for {
		i := strings.IndexByte(rest, '{')
		if i < 0 {
			break
		}
		j := strings.IndexByte(rest[i:], '}')
		if j < 0 {
			break
		}
		if v, ok := vars[rest[i+1:i+j]]; ok {
			out.WriteString(rest[:i])
			out.WriteString(html.EscapeString(v))
			rest = rest[i+j+1:]
			continue
		}
		out.WriteString(rest[:i+1])
		rest = rest[i+1:]
	}
	out.WriteString(rest)
	return out.String()
}

var telegramTags = regexp.MustCompile(`&lt;(/?)(b|i|u|s|code)&gt;`)

// Only balanced formatting from the template is enabled; substituted user data
// remains escaped. Links with attributes and arbitrary HTML are never enabled.
func telegramTemplate(text string) string {
	escaped := html.EscapeString(text)
	stack := []string{}
	for _, tag := range telegramTags.FindAllStringSubmatch(escaped, -1) {
		if tag[1] == "" {
			stack = append(stack, tag[2])
		} else {
			if len(stack) == 0 || stack[len(stack)-1] != tag[2] {
				return escaped
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) != 0 {
		return escaped
	}
	return telegramTags.ReplaceAllString(escaped, "<$1$2>")
}

func pick(admin, builtin string) string {
	if strings.TrimSpace(admin) != "" {
		return admin
	}
	return builtin
}

func (w *words) date(t time.Time) string {
	if w == &en {
		return fmt.Sprintf("%s %d, %d", w.months[t.Month()-1], t.Day(), t.Year())
	}
	return fmt.Sprintf("%d %s %d", t.Day(), w.months[t.Month()-1], t.Year())
}

func (w *words) shortDate(t time.Time) string {
	if w == &en {
		return fmt.Sprintf("%s %d", w.months[t.Month()-1][:3], t.Day())
	}
	return fmt.Sprintf("%d %s", t.Day(), w.months[t.Month()-1])
}

func (w *words) days(n int) string {
	if w == &en {
		if n == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", n)
	}
	switch {
	case n%10 == 1 && n%100 != 11:
		return fmt.Sprintf("%d день", n)
	case n%10 >= 2 && n%10 <= 4 && (n%100 < 10 || n%100 >= 20):
		return fmt.Sprintf("%d дня", n)
	}
	return fmt.Sprintf("%d дней", n)
}

func (w *words) bytes(n int64) string {
	units := []string{"Б", "КБ", "МБ", "ГБ", "ТБ"}
	sep := ","
	if w == &en {
		units, sep = []string{"B", "KB", "MB", "GB", "TB"}, "."
	}
	v, i := float64(n), 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	s := fmt.Sprintf("%.0f", v)
	if i > 0 && v < 100 {
		s = strings.Replace(strings.TrimSuffix(fmt.Sprintf("%.1f", v), ".0"), ".", sep, 1)
	}
	return s + " " + units[i]
}

func (w *words) ago(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return w.justNow
	case d < time.Hour:
		return fmt.Sprintf(w.minAgo, int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf(w.hoursAgo, int(d/time.Hour))
	case d < 48*time.Hour:
		return w.yesterday
	}
	return fmt.Sprintf(w.daysAgo, w.days(int(d/(24*time.Hour))))
}

// DefaultTexts are the built-in texts the admin's empty fields fall back to.
func DefaultTexts(lang string) Texts {
	w := wordsFor(lang)
	return Texts{Welcome: w.welcome, Main: w.main, Renew: w.renew, Expiring: w.expiring, Expired: w.expired, Traffic90: w.traffic90, TrafficEnd: w.trafficEnd}
}

// Migrate only the former built-in wording; preserve administrator-written texts.
func migrateDefaultTexts(texts *Texts, lang string) {
	defaults := DefaultTexts(lang)
	legacy := Texts{
		Welcome:    "{brand}\n\nБезопасное и удобное подключение к интернету. Выберите действие ниже.",
		Main:       "{brand}\n\nБезопасное и удобное подключение к интернету. Выберите действие ниже.",
		Renew:      "Продление подписки «{name}».\n\nВыберите срок и оплатите — оплаченные дни добавятся к оставшемуся сроку. Если тарифы сейчас недоступны, обратитесь в поддержку.",
		Expiring:   "⏳ Подписка {subscription_url} заканчивается {until}: осталось {days}.",
		Expired:    "⛔️ Подписка {subscription_url} закончилась. Чтобы продлить, напишите в поддержку.",
		Traffic90:  "📦 Израсходовано 90% трафика подписки «{name}»: осталось {left}.",
		TrafficEnd: "📦 Трафик подписки «{name}» на этот период закончился. Обновится {reset}.",
	}
	if lang == "en" {
		legacy = Texts{
			Welcome:    "This is the {brand} bot.\n\nTo manage your subscription, open its page and tap “Open in Telegram”, or send the subscription link here.",
			Main:       "{brand}\n\nSecure, convenient internet access. Choose an action below.",
			Renew:      "To renew your subscription, message support: they will tell you how to pay.",
			Expiring:   "⏳ Subscription “{name}” ends {until}: {days} left.",
			Expired:    "⛔️ Subscription “{name}” has ended. Message support to renew it.",
			Traffic90:  "📦 90% of the traffic of “{name}” is used: {left} left.",
			TrafficEnd: "📦 The traffic of “{name}” for this period is used up. It renews {reset}.",
		}
	}
	for _, field := range []struct {
		value        *string
		old, current string
	}{
		{&texts.Welcome, legacy.Welcome, defaults.Welcome},
		{&texts.Main, legacy.Main, defaults.Main},
		{&texts.Renew, legacy.Renew, defaults.Renew},
		{&texts.Expiring, legacy.Expiring, defaults.Expiring},
		{&texts.Expired, legacy.Expired, defaults.Expired},
		{&texts.Traffic90, legacy.Traffic90, defaults.Traffic90},
		{&texts.TrafficEnd, legacy.TrafficEnd, defaults.TrafficEnd},
	} {
		if *field.value == field.old {
			*field.value = field.current
		}
	}
}
