package tgbot

import (
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"
)

// Config is what the admin sets up in the panel: the menu, the texts and which
// notifications go out. It is stored as one JSON setting.
type AdminMenuConfig struct {
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
	Action string `json:"action" enum:"profile,buy,sub,devices,connect,renew,support,app,url,page"`
	Label  string `json:"label"`
	On     bool   `json:"on"`
	Row    bool   `json:"row" doc:"В одном ряду с предыдущей"`
	URL    string `json:"url,omitempty" doc:"Для action=url: https:// или tg://"`
	Text   string `json:"text,omitempty" doc:"Для action=page: текст страницы"`
}

// Texts the admin writes. Variables: {name} {brand} {until} {days} {used} {left} {limit}
// {devices} {reset}; an empty text is the built-in one.
type Texts struct {
	Welcome    string `json:"welcome" doc:"Для тех, у кого ещё нет подписки в боте"`
	Main       string `json:"main" doc:"Шапка главного меню"`
	Renew      string `json:"renew" doc:"Экран «Продлить»"`
	Expiring   string `json:"expiring" doc:"Уведомление: подписка скоро закончится"`
	Expired    string `json:"expired" doc:"Уведомление: подписка закончилась"`
	Traffic90  string `json:"traffic_90" doc:"Уведомление: израсходовано 90% трафика"`
	TrafficEnd string `json:"traffic_end" doc:"Уведомление: трафик закончился"`
}

type Notify struct {
	Expire3d   bool `json:"expire_3d"`
	Expire1d   bool `json:"expire_1d"`
	Expired    bool `json:"expired"`
	Traffic90  bool `json:"traffic_90"`
	Traffic100 bool `json:"traffic_100"`
}

// Built-in actions, each at most once in the menu.
var builtins = []string{"profile", "buy", "devices", "connect", "renew", "support", "app"}

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
		MenuVersion: 2,
		Texts:       DefaultTexts(lang),
		Admin:       AdminMenuConfig{Enabled: true, Users: true, Subscriptions: true, Search: true, Grant: true, Statistics: true},
		Buttons: []MenuButton{
			{ID: "profile", Action: "profile", Label: l("◉ Профиль", "◉ Profile"), On: true},
			{ID: "buy", Action: "buy", Label: l("◇ Купить", "◇ Buy"), On: true},
			{ID: "renew", Action: "renew", Label: l("↻ Продлить", "↻ Renew"), On: true},
		},
		Notify:     Notify{Expire3d: true, Expire1d: true, Expired: true, Traffic90: true, Traffic100: true},
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
				button.Action, button.ID, button.Label = "profile", "profile", "◉ Профиль"
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
			buttons = append(buttons, Default(c.Lang).Buttons[1])
		}
		c.Buttons = buttons
		c.MenuVersion = 2
		if strings.Contains(c.Texts.Main, "{state}") && strings.Contains(c.Texts.Main, "{term}") {
			c.Texts.Main = DefaultTexts(c.Lang).Main
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
		stock := map[string]string{"👤 Профиль": "◉ Профиль", "👤 Profile": "◉ Profile", "📋 Мои подписки": "▤ Мои подписки", "📋 My subscriptions": "▤ My subscriptions", "📱 Устройства": "▣ Устройства", "📱 Devices": "▣ Devices", "🔌 Подключить устройство": "↗ Подключить устройство", "🔌 Connect a device": "↗ Connect a device", "💳 Продлить": "◇ Продлить", "💳 Renew": "◇ Renew", "💬 Поддержка": "◌ Поддержка", "💬 Support": "◌ Support", "🌐 Открыть страницу подписки": "◎ Открыть страницу подписки", "🌐 Open the subscription page": "◎ Open the subscription page"}
		if next, ok := stock[b.Label]; ok {
			b.Label = next
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
