package tgbot

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"mirai/internal/panel/domain"
	"mirai/internal/panel/settings"
)

func adminEnv(t *testing.T) *env {
	return setup(t, func(e *env, d *Deps) {
		d.Users = domain.NewUsers(e.st, domain.NewPool(e.st, e.clock), noChanges{}, e.clock)
		if err := settings.Set(e.ctx, e.set, KeyAdminID, int64(900)); err != nil {
			t.Fatal(err)
		}
	})
}
func adminTap(e *env, data string) call {
	e.t.Helper()
	e.later()
	n := e.tg.count()
	e.press(900, 1234, data)
	calls := e.tg.wait(e.t, n, "editMessageText")
	c, _ := find(calls, "editMessageText")
	return c
}
func adminSay(e *env, s string) call {
	e.t.Helper()
	e.later()
	n := e.tg.count()
	e.say(900, s)
	calls := e.tg.wait(e.t, n, "sendMessage")
	c, _ := find(calls, "sendMessage")
	return c
}
func adminConfirm(e *env) string {
	e.t.Helper()
	f, ok := e.bot.adminFlow(900, false)
	if !ok {
		e.t.Fatal("missing confirmation")
	}
	return "a:commit:" + f.Nonce
}

func TestAdminAccess(t *testing.T) {
	e := adminEnv(t)
	_, kb := e.bot.screen(e.ctx, e.bot.Config(e.ctx), 900, "m", "")
	found := false
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			found = found || btn.CallbackData == "a:home"
		}
	}
	if !found {
		t.Fatal("owner has no Admin button without a subscription")
	}
	_, kb = e.bot.screen(e.ctx, e.bot.Config(e.ctx), 555, "m", "")
	if kb != nil {
		for _, row := range kb.InlineKeyboard {
			for _, btn := range row {
				if strings.HasPrefix(btn.CallbackData, "a:") {
					t.Fatal("stranger sees Admin")
				}
			}
		}
	}
	text, _ := e.bot.adminScreen(e.ctx, 555, "a:users:0", "")
	if text != "Нет доступа." {
		t.Fatal(text)
	}
	// The message's destination alone is not authority: validate Telegram's sender.
	q := &CallbackQuery{ID: "spoof", From: User{ID: 555}, Message: &Message{MessageID: 1234, Chat: Chat{ID: 900, Type: "private"}}, Data: fmt.Sprintf("a:freeze:%d", e.user.ID)}
	e.bot.adminPress(e.ctx, e.bot.client.Load(), e.bot.out.Load(), q)
	u, _ := e.st.Q.GetUser(e.ctx, e.user.ID)
	if u.Status == "disabled" {
		t.Fatal("spoofed sender changed subscription")
	}
	settings.Set(e.ctx, e.set, KeyInfraAdminChat, int64(555))
	if e.bot.isAdmin(e.ctx, 555, 555) {
		t.Fatal("alert recipient became administrator")
	}
	settings.Set(e.ctx, e.set, KeyAdminID, int64(0))
	if e.bot.isAdmin(e.ctx, 900, 900) {
		t.Fatal("revoked administrator retained access")
	}
}

func TestAdminSubscriptionLifecycle(t *testing.T) {
	e := adminEnv(t)
	id := e.user.ID
	if c := adminTap(e, "a:subs:0"); !strings.Contains(text(c), "Все подписки") {
		t.Fatal(text(c))
	}
	if c := adminTap(e, fmt.Sprintf("a:user:%d", id)); !strings.Contains(text(c), "Трафик периода") {
		t.Fatal(text(c))
	}
	adminTap(e, fmt.Sprintf("a:freeze:%d", id))
	u, _ := e.st.Q.GetUser(e.ctx, id)
	if u.Status != "disabled" {
		t.Fatal("freeze failed")
	}
	adminTap(e, fmt.Sprintf("a:unfreeze:%d", id))
	u, _ = e.st.Q.GetUser(e.ctx, id)
	if u.Status != "active" {
		t.Fatal("unfreeze failed")
	}
	before := u.ExpiresAt.Int64
	adminTap(e, fmt.Sprintf("a:extend:%d", id))
	adminSay(e, "-5")
	f, _ := e.bot.adminFlow(900, false)
	if f.Days != 0 {
		t.Fatal("invalid extension accepted")
	}
	adminSay(e, "17")
	confirm := adminConfirm(e)
	adminTap(e, confirm)
	u, _ = e.st.Q.GetUser(e.ctx, id)
	if u.ExpiresAt.Int64 != before+17*86400 {
		t.Fatalf("expiry %d want %d", u.ExpiresAt.Int64, before+17*86400)
	}
	adminTap(e, confirm)
	u, _ = e.st.Q.GetUser(e.ctx, id)
	if u.ExpiresAt.Int64 != before+17*86400 {
		t.Fatal("duplicate callback extended twice")
	}
	adminTap(e, "a:search")
	if c := adminSay(e, "Анна"); !strings.Contains(text(c), "Найдено: 1") {
		t.Fatal(text(c))
	}
	adminTap(e, fmt.Sprintf("a:delete:%d", id))
	if _, err := e.st.Q.GetUser(e.ctx, id); err != nil {
		t.Fatal("deleted before confirmation")
	}
	adminTap(e, adminConfirm(e))
	if _, err := e.st.Q.GetUser(e.ctx, id); err == nil {
		t.Fatal("delete failed")
	}
}

func TestAdminGrantAndExpiry(t *testing.T) {
	e := adminEnv(t)
	e.say(555, "/start")
	e.tg.wait(t, 0, "sendMessage")
	adminTap(e, "a:grant:555")
	tariffs, _ := e.st.Q.ListTariffs(e.ctx)
	adminTap(e, fmt.Sprintf("a:tariff:%d", tariffs[1].ID))
	adminSay(e, "Подарок <test>")
	adminSay(e, "9")
	at := e.clock()
	confirm := adminConfirm(e)
	adminTap(e, confirm)
	links, err := e.st.Q.ListTgLinksOf(e.ctx, 555)
	if err != nil || len(links) != 1 {
		t.Fatalf("grant links: %v %v", links, err)
	}
	u := links[0]
	if u.Name != "Подарок <test>" || u.ExpiresAt.Int64 != at.Add(2*time.Second+9*24*time.Hour).Unix() {
		t.Fatalf("grant: %+v", u)
	}
	adminTap(e, confirm)
	links, _ = e.st.Q.ListTgLinksOf(e.ctx, 555)
	if len(links) != 1 {
		t.Fatal("duplicate grant")
	}
	adminTap(e, fmt.Sprintf("a:extend:%d", u.ID))
	adminSay(e, "3")
	confirm = adminConfirm(e)
	e.mu.Lock()
	e.now = e.now.Add(11 * time.Minute)
	e.mu.Unlock()
	if _, ok := e.bot.takeAdminConfirmation(900, strings.TrimPrefix(confirm, "a:commit:")); ok {
		t.Fatal("expired action was accepted")
	}
}
