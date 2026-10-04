package tgbot

import (
	"strings"
	"testing"
)

func TestMenuStructureAndDeletion(t *testing.T) {
	cfg := Default("ru")
	if len(cfg.Buttons) != 3 || cfg.Buttons[0].Action != "profile" || cfg.Buttons[1].Action != "renew" || cfg.Buttons[2].Action != "buy" {
		t.Fatalf("default menu: %+v", cfg.Buttons)
	}
	if strings.Contains(cfg.Texts.Main, "{term}") || strings.Contains(cfg.Texts.Main, "{state}") {
		t.Fatal("main shows subscription stats")
	}
	cfg.Buttons = cfg.Buttons[:1]
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Buttons) != 1 {
		t.Fatal("deleted buttons returned")
	}
	cfg.MenuVersion = 0
	cfg.Buttons = append(cfg.Buttons, MenuButton{ID: "subscriptions", Action: "subscriptions", Label: "old", On: true})
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, b := range cfg.Buttons {
		if b.Action == "subscriptions" {
			t.Fatal("legacy subscriptions button remains")
		}
	}
}

func TestAdminButtonsPreserveRowsAndDeletion(t *testing.T) {
	cfg := Default("ru")
	cfg.Admin.Buttons = []AdminMenuButton{{ID: "grant", Action: "grant", Label: "🎁 Выдать", On: true}, {ID: "users", Action: "users", Label: "👥 Люди", On: true, Row: true}}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Admin.Buttons) != 2 || cfg.Admin.Buttons[0].Action != "grant" || !cfg.Admin.Buttons[1].Row || cfg.Admin.Subscriptions || cfg.Admin.Search {
		t.Fatal("order, row or deletion lost")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Admin.Buttons) != 2 {
		t.Fatal("deleted buttons returned")
	}
	cfg.Admin.Buttons = append(cfg.Admin.Buttons, cfg.Admin.Buttons[0])
	if err := cfg.Validate(); err == nil {
		t.Fatal("duplicate admin action accepted")
	}
}
