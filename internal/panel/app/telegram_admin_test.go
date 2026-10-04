package app

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"mirai/internal/panel/settings"
	"mirai/internal/panel/tgbot"
)

func TestTelegramAdminIDOverHTTP(t *testing.T) {
	h := newHarness(t)
	if r, _ := h.login(password, ""); r.StatusCode != http.StatusOK {
		t.Fatal("login")
	}
	api := "/" + adminPath + "/api/v1/telegram"
	csrf := map[string]string{"X-CSRF-Token": h.csrf}
	for _, id := range []any{-1, "@username", 9007199254740992} {
		r, _ := h.do(http.MethodPatch, api, map[string]any{"admin_id": id}, csrf)
		if r.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("invalid ID %v: %d", id, r.StatusCode)
		}
	}
	for _, id := range []int64{123456789, 0} {
		r, body := h.do(http.MethodPatch, api, map[string]any{"admin_id": id}, csrf)
		var v struct {
			AdminID      int64 `json:"admin_id"`
			AdminChatSet bool  `json:"admin_chat_set"`
		}
		if r.StatusCode != http.StatusOK || json.Unmarshal(body, &v) != nil || v.AdminID != id || v.AdminChatSet != (id > 0) {
			t.Fatalf("saved ID %d: %d %s", id, r.StatusCode, body)
		}
		set := settings.New(h.st.Q)
		actual, _, err := settings.Get[int64](context.Background(), set, tgbot.KeyAdminID)
		if err != nil || actual != id {
			t.Fatalf("stored admin ID: %d %v", actual, err)
		}
		chat, _, err := settings.Get[int64](context.Background(), set, tgbot.KeyInfraAdminChat)
		if err != nil || chat != id {
			t.Fatalf("stored alert chat: %d %v", chat, err)
		}
	}
}
