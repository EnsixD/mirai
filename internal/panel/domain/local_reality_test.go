package domain

import (
	"context"
	"mirai/internal/panel/presets"
	"mirai/internal/panel/settings"
	"mirai/internal/proto"
	"testing"
	"time"
)

func TestLocalRealityOwnDomain(t *testing.T) {
	now := time.Unix(1800000000, 0)
	st, s := inbounds(t, &now, nil)
	ctx := context.Background()
	if err := settings.Set(ctx, settings.New(st.Q), settings.KeyDomain, "vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	row, err := s.Create(ctx, NewInbound{Preset: "vless_reality_tcp", Port: "2443"})
	if err != nil {
		t.Fatal(err)
	}
	config, err := proto.Parse(row.Config)
	if err != nil {
		t.Fatal(err)
	}
	dest, _ := presets.Dest(config)
	if dest != "vpn.example.com:443" {
		t.Fatalf("external camouflage selected: %s", dest)
	}
	row, err = s.Create(ctx, NewInbound{Preset: "vless_reality_tcp", Port: "2444", Dest: "www.microsoft.com:443"})
	if err != nil {
		t.Fatal(err)
	}
	config, _ = proto.Parse(row.Config)
	dest, _ = presets.Dest(config)
	if dest != "www.microsoft.com:443" {
		t.Fatal("explicit target replaced")
	}
}
