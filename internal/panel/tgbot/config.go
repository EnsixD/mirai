package tgbot

import (
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"
)

// Config is what the admin sets up in the panel: the menu, the texts and which
// notifications go out. It is stored as one JSON setting.
type AdminMenuButton struct {
	ID     string `json:"id"`
	Action string `json:"action" enum:"users,subscriptions,grant,orders,broadcast,maintenance,refresh"`
	Label  string `json:"label" maxLength:"40"`
	On     bool   `json:"on"`
	Row    bool   `json:"row"`
}
type AdminMenuConfig struct {
	Buttons []AdminMenuButton `json:"buttons"`
	Version int               `json:"version"`

	Enabled       bool `json:"enabled"`
	Users         bool `json:"users"`
	Subscriptions bool `json:"subscriptions"`
	Search        bool `json:"search"`
	Grant         bool `json:"grant"`
	Statistics    bool `json:"statistics"`
}
type Config struct {
	MenuVersion int             `json:"menu_version"`
	Admin       AdminMenuConfig `json:"admin"`
	Lang        string          `json:"lang" enum:"ru,en" doc:"Язык встроенных надписей бота"`
	Buttons     []MenuButton    `json:"buttons" doc:"Кнопки главного меню по порядку"`
	Texts       Texts           `json:"texts"`
	Notify      Notify          `json:"notify"`
	MiniApp     bool            `json:"mini_app" doc:"Кнопка Mini App со страницей подписки"`
	CleanChat   bool            `json:"clean_chat" doc:"Удалять сообщения пользователя, чтобы в чате было одно меню"`
	// QuietNight: the automatic notices from 22:00 to 9:00 Moscow time come without a sound.
	QuietNight bool `json:"quiet_night" doc:"Уведомления с 22:00 до 9:00 МСК приходят без звука"`
}

// MenuButton of the main menu. Built-in actions open screens; "url" opens a link, "page"
// a text of the admin's.
type MenuButton struct {
	ID     string `json:"id" doc:"Постоянный id кнопки"`
	Action string `json:"action" enum:"profile,buy,sub,devices,connect,renew,support,app,help,url,page"`
	Label  string `json:"label"`
	On     bool   `json:"on"`
	Row    bool   `json:"row" doc:"В одном ряду с предыдущей"`
	URL    string `json:"url,omitempty" doc:"Для action=url: https:// или tg://"`
	Text   string `json:"text,omitempty" doc:"Для action=page: текст страницы"`
}

// Texts the admin writes. Variables: {name} {brand} {until} {days} {used} {left} {limit}
// {devices} {reset}; an empty text is the built-in one.
type Texts struct {
	Welcome    string `json:"welcome" doc:"Сообщение /start с информацией о VPN и кнопками главного меню"`
	Main       string `json:"main" doc:"Шапка главного меню"`
	Renew      string `json:"renew" doc:"Экран «Продлить»"`
	Expiring   string `json:"expiring" doc:"Уведомление: подписка скоро закончится"`
	Expired    string `json:"expired" doc:"Уведомление: подписка закончилась"`
	Traffic90  string `json:"traffic_90" doc:"Уведомление: израсходовано 90% трафика"`
	TrafficEnd string `json:"traffic_end" doc:"Уведомление: трафик закончился"`
}

type Notify struct {
	AdminChanges bool `json:"admin_changes" doc:"Уведомлять о выдаче и изменениях подписки администратором"`
	Expire3d     bool `json:"expire_3d"`
	Expire1d     bool `json:"expire_1d"`
	Expired      bool `json:"expired"`
	Traffic90    bool `json:"traffic_90"`
	Traffic100   bool `json:"traffic_100"`
}

// Built-in actions, each at most once in the menu.
var builtins = []string{"help", "profile", "buy", "devices", "connect", "renew", "support", "app"}

// Default is the menu of a fresh bot in lang, "en" or else Russian.
func Default(lang string) Config {
	if lang != "en" {
		lang = "ru"
	}
	l := func(ru, en string) string {
		if lang == "en" {
			return en
		}
		return ru
	}
	return Config{
		Lang:        lang,
		MenuVersion: 4,
		Texts:       DefaultTexts(lang),
		Admin:       AdminMenuConfig{Version: 3, Buttons: defaultAdminButtons(), Enabled: true, Users: true, Subscriptions: true, Search: false, Grant: true, Statistics: true},
		Buttons: []MenuButton{
			{ID: "buy", Action: "buy", Label: l("🛒 Купить", "🛒 Buy"), On: true},
			{ID: "renew", Action: "renew", Label: l("🔄 Продлить", "🔄 Renew"), On: true},
			{ID: "profile", Action: "profile", Label: l("👤 Профиль", "👤 Profile"), On: true},
			{ID: "help", Action: "help", Label: l("📖 Инструкция", "📖 Instructions"), On: true},
			{ID: "support", Action: "support", Label: l("💬 Поддержка", "💬 Support"), On: true},
		},
		Notify:     Notify{AdminChanges: true, Expire3d: true, Expire1d: true, Expired: true, Traffic90: true, Traffic100: true},
		MiniApp:    true,
		CleanChat:  true,
		QuietNight: true,
	}
}

// Limits keep the menu within Telegram's: 100 buttons, 64-byte callback data, 4096-char
// messages.
const (
	maxButtons = 20
	maxLabel   = 40
	maxText    = 3000
)

// Validation codes (the UI translates tg_<code>).
var (
	ErrButtons     = errors.New("tg_buttons")
	ErrButtonLabel = errors.New("tg_button_label")
	ErrButtonURL   = errors.New("tg_button_url")
	ErrButtonText  = errors.New("tg_button_text")
	ErrText        = errors.New("tg_text")
)

// Validate checks a config from the admin panel and fills in defaults.
func (c *Config) Validate() error {
	if c.MenuVersion < 2 {
		buttons := []MenuButton{}
		for _, button := range c.Buttons {
			if button.Action == "sub" {
				button.Action, button.ID, button.Label = "profile", "profile", "👤 Профиль"
			}
			if button.Action == "profile" || button.Action == "renew" || button.Action == "buy" || button.Action == "url" || button.Action == "page" {
				buttons = append(buttons, button)
			}
		}
		found := false
		for _, button := range buttons {
			if button.Action == "buy" {
				found = true
			}
		}
		if !found {
			for _, button := range Default(c.Lang).Buttons {
				if button.Action == "buy" {
					buttons = append(buttons, button)
					break
				}
			}
		}
		c.Buttons = buttons
		c.MenuVersion = 2
		if strings.Contains(c.Texts.Main, "{state}") && strings.Contains(c.Texts.Main, "{term}") {
			c.Texts.Main = DefaultTexts(c.Lang).Main
		}
	}
	if c.MenuVersion < 3 {
		for i := range c.Buttons {
			if c.Buttons[i].Action == "buy" && i > 0 && c.Buttons[i-1].Action == "renew" {
				break
			}
			if c.Buttons[i].Action == "buy" {
				for j := i + 1; j < len(c.Buttons); j++ {
					if c.Buttons[j].Action == "renew" {
						c.Buttons[i], c.Buttons[j] = c.Buttons[j], c.Buttons[i]
						break
					}
				}
				break
			}
		}
		c.MenuVersion = 3
		c.Texts.Main = strings.ReplaceAll(c.Texts.Main, " · {name}", "")
		if strings.Contains(c.Texts.Welcome, "в «Моих подписках»") {
			c.Texts.Welcome = DefaultTexts(c.Lang).Welcome
		}
	}
	if c.Admin.Version < 1 {
		c.Admin.Buttons = defaultAdminButtons()
		for i := range c.Admin.Buttons {
			button := &c.Admin.Buttons[i]
			button.On = map[string]bool{"users": c.Admin.Users, "subscriptions": c.Admin.Subscriptions, "search": c.Admin.Search, "grant": c.Admin.Grant}[button.Action]
		}
		c.Admin.Version = 1
	}
	// Issuing is a permission on the customer profile, not a main-menu button.
	buttons := []AdminMenuButton{}
	for _, button := range c.Admin.Buttons {
		if button.Action != "grant" && button.Action != "search" {
			buttons = append(buttons, button)
		}
	}
	c.Admin.Buttons = buttons
	if c.Admin.Version < 3 {
		for _, entry := range defaultAdminButtons() {
			if entry.Action == "users" || entry.Action == "subscriptions" || entry.Action == "search" {
				continue
			}
			found := false
			for _, current := range c.Admin.Buttons {
				if current.Action == entry.Action {
					found = true
				}
			}
			if !found {
				c.Admin.Buttons = append(c.Admin.Buttons, entry)
			}
		}
	}
	c.Admin.Version = 3
	if c.MenuVersion < 4 {
		if len(c.Buttons) == 3 && c.Buttons[0].Action == "profile" && c.Buttons[1].Action == "renew" && c.Buttons[2].Action == "buy" {
			c.Buttons[0], c.Buttons[2] = c.Buttons[2], c.Buttons[0]
		}
		found := false
		for _, button := range c.Buttons {
			if button.Action == "help" {
				found = true
			}
		}
		if !found {
			c.Buttons = append(c.Buttons, MenuButton{ID: "help", Action: "help", Label: "📖 Инструкция", On: true})
		}
		c.MenuVersion = 4
	}
	seenAdmin := map[string]bool{}
	if len(c.Admin.Buttons) > 10 {
		return ErrButtons
	}
	c.Admin.Users = false
	c.Admin.Subscriptions = false
	c.Admin.Search = false
	for i := range c.Admin.Buttons {
		button := &c.Admin.Buttons[i]
		if !contains([]string{"users", "subscriptions", "orders", "broadcast", "maintenance", "refresh"}, button.Action) || seenAdmin[button.Action] {
			return ErrButtons
		}
		seenAdmin[button.Action] = true
		button.ID = button.Action
		button.Label = strings.TrimSpace(button.Label)
		if button.Label == "" || utf8.RuneCountInString(button.Label) > 40 {
			return ErrButtonLabel
		}
		switch button.Action {
		case "users":
			c.Admin.Users = button.On
		case "subscriptions":
			c.Admin.Subscriptions = button.On
		case "search":
			c.Admin.Search = button.On
		}
	}
	filtered := []MenuButton{}
	for _, button := range c.Buttons {
		if button.Action != "subscriptions" {
			filtered = append(filtered, button)
		}
	}
	c.Buttons = filtered

	if c.Lang != "en" {
		c.Lang = "ru"
	}
	if len(c.Buttons) > maxButtons {
		return ErrButtons
	}
	seen := map[string]bool{}
	for i := range c.Buttons {
		b := &c.Buttons[i]
		if b.Label == "👤 Профиль" {
			b.Label = "👤 Профиль"
		}
		if b.Label == "👤 Profile" {
			b.Label = "👤 Profile"
		}
		if b.Label == "🛒 Купить" {
			b.Label = "🛒 Купить"
		}
		if b.Label == "🛒 Buy" {
			b.Label = "🛒 Buy"
		}
		if b.Label == "💳 Продлить" {
			b.Label = "💳 Продлить"
		}
		if b.Label == "💳 Продлить" {
			b.Label = "💳 Продлить"
		}
		if b.Label == "💳 Renew" {
			b.Label = "💳 Renew"
		}
		if b.Label == "💳 Renew" {
			b.Label = "💳 Renew"
		}
		if b.Label == "📱 Устройства" {
			b.Label = "📱 Устройства"
		}
		if b.Label == "📱 Devices" {
			b.Label = "📱 Devices"
		}
		if b.Label == "🔌 Подключить устройство" {
			b.Label = "🔌 Подключить устройство"
		}
		if b.Label == "🔌 Connect a device" {
			b.Label = "🔌 Connect a device"
		}
		if b.Label == "💬 Поддержка" {
			b.Label = "💬 Поддержка"
		}
		if b.Label == "💬 Support" {
			b.Label = "💬 Support"
		}
		if b.Label == "🌐 Открыть страницу подписки" {
			b.Label = "🌐 Открыть страницу подписки"
		}
		if b.Label == "🌐 Open the subscription page" {
			b.Label = "🌐 Open the subscription page"
		}
		if b.Label == "⚙️ Админ-панель" {
			b.Label = "⚙️ Админ-панель"
		}
		if b.Label == "👥 Пользователи" {
			b.Label = "👥 Пользователи"
		}
		if b.Label == "📋 Все подписки" {
			b.Label = "📋 Все подписки"
		}
		if b.Label == "🔎 Поиск" {
			b.Label = "🔎 Поиск"
		}
		if b.Label == "🎁 Выдать подписку" {
			b.Label = "🎁 Выдать подписку"
		}
		if b.Label == "📋 Подписок" {
			b.Label = "📋 Подписок"
		}
		if b.Label == "💳 Потрачено" {
			b.Label = "💳 Потрачено"
		}
		if b.Label == "💳 Spent" {
			b.Label = "💳 Spent"
		}
		if b.Label == "◉ Профиль" {
			b.Label = "👤 Профиль"
		}
		if b.Label == "◉ Profile" {
			b.Label = "👤 Profile"
		}
		if b.Label == "◇ Купить" {
			b.Label = "🛒 Купить"
		}
		if b.Label == "◇ Buy" {
			b.Label = "🛒 Buy"
		}
		if b.Label == "↻ Продлить" {
			b.Label = "💳 Продлить"
		}
		if b.Label == "◇ Продлить" {
			b.Label = "💳 Продлить"
		}
		if b.Label == "↻ Renew" {
			b.Label = "💳 Renew"
		}
		if b.Label == "◇ Renew" {
			b.Label = "💳 Renew"
		}
		if b.Label == "▣ Устройства" {
			b.Label = "📱 Устройства"
		}
		if b.Label == "↗ Подключить устройство" {
			b.Label = "🔌 Подключить устройство"
		}
		if b.Label == "◌ Поддержка" {
			b.Label = "💬 Поддержка"
		}
		if b.Label == "◎ Открыть страницу подписки" {
			b.Label = "🌐 Открыть страницу подписки"
		}
		b.Label = strings.TrimSpace(b.Label)
		if b.Label == "" || utf8.RuneCountInString(b.Label) > maxLabel {
			return ErrButtonLabel
		}
		switch b.Action {
		case "url":
			if !safeURL(b.URL) {
				return ErrButtonURL
			}
		case "page":
			if strings.TrimSpace(b.Text) == "" || utf8.RuneCountInString(b.Text) > maxText {
				return ErrButtonText
			}
		default:
			if b.Action == "help" && utf8.RuneCountInString(b.Text) > maxText {
				return ErrButtonText
			}
			if !contains(builtins, b.Action) || seen[b.Action] {
				return ErrButtons
			}
			seen[b.Action] = true
			b.ID = b.Action
		}
		if b.Action == "url" || b.Action == "page" {
			// Custom buttons keep their id: the callback data points at it.
			if b.ID == "" || contains(builtins, b.ID) || len(b.ID) > 16 || strings.ContainsAny(b.ID, ": ") || seen["id:"+b.ID] {
				b.ID = "c" + itoa(i+1)
			}
			seen["id:"+b.ID] = true
			if b.Action == "url" {
				b.Text = ""
			} else {
				b.URL = ""
			}
		}
	}
	for _, t := range []string{c.Texts.Welcome, c.Texts.Main, c.Texts.Renew, c.Texts.Expiring, c.Texts.Expired, c.Texts.Traffic90, c.Texts.TrafficEnd} {
		if utf8.RuneCountInString(t) > maxText {
			return ErrText
		}
	}
	migrateDefaultTexts(&c.Texts, c.Lang)
	c.Texts.Expiring = strings.ReplaceAll(strings.ReplaceAll(c.Texts.Expiring, "«{name}»", "{subscription_url}"), "{name}", "{subscription_url}")
	c.Texts.Expired = strings.ReplaceAll(strings.ReplaceAll(c.Texts.Expired, "«{name}»", "{subscription_url}"), "{name}", "{subscription_url}")
	defaults := DefaultTexts(c.Lang)
	fields := []struct {
		value    *string
		fallback string
	}{{&c.Texts.Welcome, defaults.Welcome}, {&c.Texts.Main, defaults.Main}, {&c.Texts.Renew, defaults.Renew}, {&c.Texts.Expiring, defaults.Expiring}, {&c.Texts.Expired, defaults.Expired}, {&c.Texts.Traffic90, defaults.Traffic90}, {&c.Texts.TrafficEnd, defaults.TrafficEnd}}
	for _, field := range fields {
		if strings.TrimSpace(*field.value) == "" {
			*field.value = field.fallback
		}
	}
	return nil
}

// safeURL: links the bot may show — web pages and Telegram links.
func safeURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "https" && u.Host != "" || u.Scheme == "tg" && u.Host != "") && u.User == nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

func defaultAdminButtons() []AdminMenuButton {
	return []AdminMenuButton{{ID: "users", Action: "users", Label: "👥 Пользователи", On: true}, {ID: "subscriptions", Action: "subscriptions", Label: "🔑 Ключи", On: true}, {ID: "orders", Action: "orders", Label: "🧾 Заказы", On: true}, {ID: "broadcast", Action: "broadcast", Label: "📢 Рассылка", On: true}, {ID: "maintenance", Action: "maintenance", Label: "🛠 Тех. работы", On: true}, {ID: "refresh", Action: "refresh", Label: "🔄 Обновить", On: true}}
}
