package tgbot

import (
	"mirai/internal/panel/store/db"
	"testing"
)

func TestDeviceLabelIncludesModelOSAndApp(t *testing.T) {
	got := deviceLabel(&ru, db.BoundDevice{Hwid: "test", Model: "Pixel 9", Os: "Android", OsVersion: "16", App: "Happ/3.10"})
	if got != "Pixel 9 · Android 16 · Happ" {
		t.Fatalf("device label: %q", got)
	}
}
