package tgbot

import (
	"fmt"
	"strings"
	"testing"

	"mirai/internal/panel/domain"
	"mirai/internal/panel/settings"
	"mirai/internal/panel/store/db"
)

func TestLocalBotStructureAndOwnership(t *testing.T) {
	e := adminEnv(t)
	e.say(555, "/start")
	e.tg.wait(t, 0, "sendMessage")
	stats, err := e.st.Q.AdminTelegramStats(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, home := e.bot.renderAdmin(e.ctx, 900, "home")
	counts := map[string]int64{"a:users:0": stats.Accounts, "a:subs:0": stats.Keys, "a:orders:0": stats.Pending}
	for _, row := range home.InlineKeyboard {
		for _, button := range row {
			if count, ok := counts[button.CallbackData]; ok {
				if !strings.HasSuffix(button.Text, fmt.Sprintf("(%d)", count)) {
					t.Fatal("missing button count", button.Text)
				}
				delete(counts, button.CallbackData)
			}
			if button.CallbackData == "a:search" {
				t.Fatal("removed search button returned")
			}
		}
	}
	if len(counts) != 0 {
		t.Fatal("counted admin buttons missing")
	}
	for _, screen := range []string{"users:0", "subs:0"} {
		_, list := e.bot.renderAdmin(e.ctx, 900, screen)
		for _, row := range list.InlineKeyboard {
			for _, button := range row {
				if button.CallbackData == "a:search" {
					t.Fatal("search remains in list", screen)
				}
			}
		}
	}
	if err := e.st.Q.LinkTg(e.ctx, db.LinkTgParams{UserID: e.user.ID, TgID: 555, CreatedAt: e.clock().Unix()}); err != nil {
		t.Fatal(err)
	}
	_, kb := e.bot.screen(e.ctx, e.bot.Config(e.ctx), 555, "r", "")
	if len(kb.InlineKeyboard) < 2 || kb.InlineKeyboard[0][0].CallbackData != fmt.Sprintf("renewselect:%d", e.user.ID) {
		t.Fatal("renew skips the subscription picker")
	}
	_, kb = e.bot.customerProfile(e.ctx, Default("ru"), 555, nil)
	orders := false
	for _, row := range kb.InlineKeyboard {
		for _, button := range row {
			orders = orders || button.CallbackData == "orders"
		}
	}
	if !orders {
		t.Fatal("profile order history missing")
	}
	var id int64
	if err := e.st.DB.QueryRowContext(e.ctx, `INSERT INTO payments(provider,payload,tg_id,kind,tariff_name,amount,currency,status,created_at) VALUES('addon:yookassa','order-owner',555,'new','Example',19900,'RUB','pending',1) RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	text, _ := e.bot.orderScreen(e.ctx, 666, id, false)
	if strings.Contains(text, "199") || text != "Заказ не найден." {
		t.Fatal("another account sees an order", text)
	}
	text, _ = e.bot.orderScreen(e.ctx, 555, id, false)
	if !strings.Contains(text, "199 ₽") {
		t.Fatal("owner cannot resume order", text)
	}
	text, _ = e.bot.screen(e.ctx, e.bot.Config(e.ctx), 555, "help", "")
	if !strings.Contains(text, "Мои подписки") || !strings.Contains(text, "INCY") {
		t.Fatal("instruction navigation missing")
	}
	if err := settings.Set(e.ctx, e.set, KeyMaintenance, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(e.bot.botGate(e.ctx, 555), "Технические работы") || e.bot.botGate(e.ctx, 900) != "" {
		t.Fatal("maintenance gate or admin exception is wrong")
	}
	data, _ := e.bot.act(e.ctx, 555, fmt.Sprintf("renewselect:%d", e.user.ID))
	if data != "m" {
		t.Fatal("mutations bypass maintenance")
	}
}

func TestAdminAccountBanAndDeletion(t *testing.T) {
	e := adminEnv(t)
	e.say(555, "/start")
	e.tg.wait(t, 0, "sendMessage")
	if err := e.st.Q.LinkTg(e.ctx, db.LinkTgParams{UserID: e.user.ID, TgID: 555, CreatedAt: e.clock().Unix()}); err != nil {
		t.Fatal(err)
	}
	tariffs, _ := e.st.Q.ListTariffs(e.ctx)
	frozen, err := e.bot.d.Users.Create(e.ctx, domain.CreateInput{Name: "Frozen", TariffID: tariffs[0].ID, TelegramID: 555})
	if err != nil {
		t.Fatal(err)
	}
	disabled := true
	if _, err = e.bot.d.Users.Update(e.ctx, frozen.ID, domain.Patch{Disabled: &disabled}); err != nil {
		t.Fatal(err)
	}
	_, notice, _ := e.bot.adminNavigationAction(e.ctx, 900, "ban", "555")
	if !strings.Contains(notice, "изменён") {
		t.Fatal(notice)
	}
	if !strings.Contains(e.bot.botGate(e.ctx, 555), "заблокирован") {
		t.Fatal("account can still use the bot")
	}
	if _, err = e.bot.d.Users.Extend(e.ctx, e.user.ID, 30); err != nil {
		t.Fatal(err)
	}
	u, _ := e.st.Q.GetUser(e.ctx, e.user.ID)
	if u.Status != "disabled" {
		t.Fatal("extension reactivated a banned account")
	}
	issued, err := e.bot.d.Users.Create(e.ctx, domain.CreateInput{Name: "Issued while banned", TariffID: tariffs[0].ID, TelegramID: 555})
	if err != nil {
		t.Fatal(err)
	}
	u, _ = e.st.Q.GetUser(e.ctx, issued.ID)
	if u.Status != "disabled" {
		t.Fatal("a newly issued key bypasses the ban")
	}
	_, _, _ = e.bot.adminNavigationAction(e.ctx, 900, "ban", "555")
	for _, id := range []int64{e.user.ID, issued.ID} {
		u, _ = e.st.Q.GetUser(e.ctx, id)
		if u.Status == "disabled" {
			t.Fatal("unblock did not restore active keys")
		}
	}
	u, _ = e.st.Q.GetUser(e.ctx, frozen.ID)
	if u.Status != "disabled" {
		t.Fatal("unblock changed a previously frozen subscription")
	}
	if _, err = e.st.DB.ExecContext(e.ctx, `INSERT INTO payments(provider,payload,tg_id,kind,user_id,tariff_name,amount,currency,status,created_at) VALUES('addon:yookassa','retained-receipt',555,'new',$1,'Example',19900,'RUB','applied',1)`, e.user.ID); err != nil {
		t.Fatal(err)
	}
	if err = e.bot.d.Users.DeleteTelegramAccount(e.ctx, 555); err != nil {
		t.Fatal(err)
	}
	if _, err = e.st.Q.GetTgChat(e.ctx, 555); err == nil {
		t.Fatal("deleted account remains")
	}
	keys, _ := e.st.Q.ListTgLinksOf(e.ctx, 555)
	if len(keys) != 0 {
		t.Fatal("deleted account retains subscriptions")
	}
	var receipts int
	if err = e.st.DB.QueryRowContext(e.ctx, `SELECT count(*) FROM payments WHERE payload='retained-receipt'`).Scan(&receipts); err != nil || receipts != 1 {
		t.Fatal("deletion lost receipt", err)
	}
}

func TestAdminDeviceNavigationAndRevokedPermission(t *testing.T) {
	e := adminEnv(t)
	if _, err := e.devs.Bind(e.ctx, e.user, domain.DeviceInfo{HWID: "test-hwid-device-01", Model: "Phone", App: "Happ"}, true); err != nil {
		t.Fatal(err)
	}
	_, kb := e.bot.adminScreen(e.ctx, 900, fmt.Sprintf("a:devices:%d", e.user.ID), "")
	callback := ""
	for _, row := range kb.InlineKeyboard {
		for _, button := range row {
			if strings.HasPrefix(button.CallbackData, "a:unbind:") {
				callback = button.CallbackData
			}
		}
	}
	if callback == "" {
		t.Fatal("admin device callback lost its subscription context")
	}
	adminTap(e, callback)
	if n, _ := e.st.Q.CountBoundDevices(e.ctx, e.user.ID); n != 0 {
		t.Fatal("admin could not remove the selected device")
	}
	adminTap(e, fmt.Sprintf("a:limit:%d", e.user.ID))
	adminSay(e, "4")
	u, _ := e.st.Q.GetUser(e.ctx, e.user.ID)
	if !u.DeviceLimit.Valid || u.DeviceLimit.Int64 != 4 {
		t.Fatal("device limit did not reach the panel")
	}
	previous := u.ExpiresAt.Int64
	adminTap(e, fmt.Sprintf("a:extend:%d", e.user.ID))
	adminSay(e, "-1")
	adminTap(e, adminConfirm(e))
	u, _ = e.st.Q.GetUser(e.ctx, e.user.ID)
	if u.ExpiresAt.Int64 != previous-86400 {
		t.Fatal("signed expiry adjustment is wrong")
	}
	e.bot.setAdminFlow(900, adminFlow{Kind: "account-delete", TgID: 555})
	cfg := e.bot.Config(e.ctx)
	for i := range cfg.Admin.Buttons {
		if cfg.Admin.Buttons[i].Action == "users" {
			cfg.Admin.Buttons[i].On = false
		}
	}
	if err := settings.Set(e.ctx, e.set, KeyConfig, cfg); err != nil {
		t.Fatal(err)
	}
	if e.bot.adminSectionAllowed(e.ctx, 900, "commit:nonce") {
		t.Fatal("revoked user permission still allows a destructive confirmation")
	}
}
