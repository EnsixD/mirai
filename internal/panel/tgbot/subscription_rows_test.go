package tgbot

import (
	"encoding/json"
	"fmt"
	"mirai/internal/panel/store/db"
	"testing"
)

func TestSubscriptionNavigationRows(t *testing.T) {
	e := adminEnv(t)
	if err := e.st.Q.UpsertTgChat(e.ctx, db.UpsertTgChatParams{TgID: 555, FirstName: "Example", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := e.st.Q.LinkTg(e.ctx, db.LinkTgParams{UserID: e.user.ID, TgID: 555, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	rows := func(k *Keyboard) [][]Button {
		t.Helper()
		b, err := json.Marshal(k)
		if err != nil {
			t.Fatal(err)
		}
		var v struct {
			Rows [][]Button `json:"inline_keyboard"`
		}
		if err = json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		return v.Rows
	}
	_, home := e.bot.renderAdmin(e.ctx, 900, "home")
	homeRows := rows(home)
	if len(homeRows) != 4 || len(homeRows[0]) != 2 || len(homeRows[1]) != 2 || len(homeRows[2]) != 2 || len(homeRows[3]) != 1 || homeRows[3][0].CallbackData != "m" {
		t.Fatal("admin home must be 2-2-2-1", homeRows)
	}
	for _, screen := range []string{"users:0", "subs:0"} {
		_, k := e.bot.renderAdmin(e.ctx, 900, screen)
		for _, r := range rows(k) {
			if len(r) != 1 {
				t.Fatal("list must be vertical", screen)
			}
		}
	}
	_, k := e.bot.renderAdmin(e.ctx, 900, fmt.Sprintf("user:%d", e.user.ID))
	r := rows(k)
	if len(r) != 4 || len(r[0]) != 2 || len(r[1]) != 1 || len(r[2]) != 2 || len(r[3]) != 1 || r[3][0].CallbackData != "a:subs:0" {
		t.Fatal("wrong admin subscription layout", r)
	}
	for _, screen := range []string{"w", "s", "d"} {
		_, k := e.bot.screen(e.ctx, e.bot.Config(e.ctx), 555, screen, "")
		for _, r := range rows(k) {
			if len(r) != 1 {
				t.Fatal("user subscription navigation must be vertical", screen)
			}
		}
	}
}
