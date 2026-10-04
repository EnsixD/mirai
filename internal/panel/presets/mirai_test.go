package presets

import (
	"net/url"
	"testing"

	"mirai/internal/proto"
)

func TestMiraiCatalogAndTransports(t *testing.T) {
	want := []string{"vless_reality_tcp", "vless_reality_grpc", "vless_ws_tls"}
	for i, id := range want {
		if All[i].ID != id {
			t.Fatalf("card %d: %s", i, All[i].ID)
		}
	}
	removed := map[string]bool{"tuic_v5": true, "vless_reality_vision": true, PresetGecko: true, "anytls": true, "trusttunnel": true, "shadowquic": true, "mieru": true, "shadowsocks_2022": true, "sudoku": true, "snell": true}
	for _, p := range All {
		if p.Default || removed[p.ID] {
			t.Fatalf("unwanted default/catalog entry: %s", p.ID)
		}
	}
	for _, id := range want {
		src, err := NewConfig(id, "")
		if err != nil {
			t.Fatal(err)
		}
		tpl, err := proto.Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		listener, err := proto.Listener(tpl, "test", "0.0.0.0", "443", []proto.Slot{{Name: "user", UUID: "11111111-1111-4111-8111-111111111111"}}, proto.Cert{CertPath: "/cert.pem", KeyPath: "/key.pem"}, proto.Options{})
		if err != nil {
			t.Fatalf("%s server: %v", id, err)
		}
		client, err := proto.ClientConfig(tpl, proto.ClientInput{Name: "Test", Host: "vpn.example.com", Port: 443, SNI: "vpn.example.com", Slot: proto.Slot{UUID: "11111111-1111-4111-8111-111111111111"}})
		if err != nil {
			t.Fatalf("%s client: %v", id, err)
		}
		uri, err := url.Parse(client.URI)
		if err != nil {
			t.Fatal(err)
		}
		switch id {
		case "vless_reality_tcp":
			if uri.Query().Get("type") != "tcp" || uri.Query().Get("security") != "reality" {
				t.Fatal(client.URI)
			}
		case "vless_reality_grpc":
			if uri.Query().Get("type") != "grpc" || uri.Query().Get("security") != "reality" {
				t.Fatal(client.URI)
			}
		case "vless_ws_tls":
			if tpl["reality-config"] != nil || listener["ws-path"] == nil || listener["certificate"] == nil || uri.Query().Get("type") != "ws" || uri.Query().Get("security") != "tls" || uri.Query().Get("path") != listener["ws-path"] {
				t.Fatalf("WS server/client disagree: %v %s", listener, client.URI)
			}
		}
	}
}
