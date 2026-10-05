package tgbot

import (
	"encoding/json"
	"testing"
)

func TestKeyboardWireGridPreservesActions(t *testing.T) {
	k := Keyboard{InlineKeyboard: [][]Button{
		{{Text: "Profile", CallbackData: "pf"}},
		{{Text: "Buy", CallbackData: "b"}, {Text: "Link", URL: "https://example.com"}, {Text: "Back", CallbackData: "w"}},
		{{Text: "Admin", CallbackData: "a:home"}},
	}}
	b, err := json.Marshal(k)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Rows [][]Button `json:"inline_keyboard"`
	}
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Rows) != 3 || len(wire.Rows[0]) != 2 || len(wire.Rows[1]) != 2 || len(wire.Rows[2]) != 1 {
		t.Fatalf("unexpected grid: %s", b)
	}
	if wire.Rows[0][1].CallbackData != "b" || wire.Rows[1][0].URL != "https://example.com" || wire.Rows[1][1].CallbackData != "w" || wire.Rows[2][0].CallbackData != "a:home" {
		t.Fatalf("actions changed: %s", b)
	}
}
