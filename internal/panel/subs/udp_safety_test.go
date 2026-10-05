package subs

import (
	"encoding/json"
	"mirai/internal/panel/presets"
	"mirai/internal/panel/store/db"
	"testing"
)

func TestXHTTPUDPFailClosed(t *testing.T) {
	config, err := presets.NewConfig("vless_reality_xhttp", "www.microsoft.com:443")
	if err != nil {
		t.Fatal(err)
	}
	p := Profile{Slot: db.Slot{Uuid: "0b4ddc4c-7c4f-4a36-9d62-6f1a44b8c4e1"}, Nodes: []Node{{ID: 1, Endpoint: Endpoint{Host: "203.0.113.7"}}}, Inbounds: []db.Inbound{{ID: 1, NodeID: 1, Name: "XHTTP", Preset: "vless_reality_xhttp", Config: config, Port: "443", Enabled: 1}}}
	for _, routing := range []Routing{RoutingAll, RoutingRUDirect} {
		data, err := Mihomo(p, Groups{Main: "Tunnel"}, routing)
		if err != nil {
			t.Fatal(err)
		}
		var cfg struct {
			Rules   []string         `json:"rules"`
			Proxies []map[string]any `json:"proxies"`
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatal(err)
		}
		if len(cfg.Proxies) != 1 || cfg.Proxies[0]["udp"] != false {
			t.Fatal("XHTTP UDP capability changed")
		}
		if cfg.Rules[len(cfg.Rules)-2] != "MATCH,Tunnel" || cfg.Rules[len(cfg.Rules)-1] != "MATCH,REJECT" {
			t.Fatalf("unsafe rule fallback: %v", cfg.Rules)
		}
	}
}
