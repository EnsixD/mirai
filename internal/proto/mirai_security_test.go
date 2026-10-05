package proto

import (
	"encoding/json"
	"net/url"
	"testing"
)

func TestMiraiTransportSafety(t *testing.T) {
	priv, _ := realityKey(t)
	tpl := Template{"type": "vless", "reality-config": map[string]any{"private-key": priv, "short-id": []any{"a1b2c3d4"}, "dest": "www.microsoft.com:443", "server-names": []any{"www.microsoft.com"}}, "ws-path": "/ws"}
	if code(Validate(tpl, Options{})) != "config_reality_ws" {
		t.Fatal("REALITY with WebSocket accepted")
	}
	delete(tpl, "ws-path")
	tpl["xhttp-config"] = map[string]any{"path": "/x", "mode": "stream-one", "x-padding-bytes": "2000-3000", "x-padding-obfs-mode": true, "session-key": "session", "seq-key": "seq", "uplink-http-method": "PUT"}
	if err := Validate(tpl, Options{}); err != nil {
		t.Fatal(err)
	}
	c, err := ClientConfig(tpl, ClientInput{Slot: slots[0], Host: "example.com", Port: 443, Name: "Test"})
	if err != nil {
		t.Fatal(err)
	}
	opts := c.Mihomo["xhttp-opts"].(map[string]any)
	u, _ := url.Parse(c.URI)
	var extra map[string]any
	if err := json.Unmarshal([]byte(u.Query().Get("extra")), &extra); err != nil {
		t.Fatal(err)
	}
	if opts["x-padding-bytes"] != "2000-3000" || extra["xPaddingBytes"] != "2000-3000" || extra["xPaddingObfsMode"] != true || extra["sessionKey"] != "session" {
		t.Fatalf("settings lost: %v / %v", opts, extra)
	}
	tpl.section("xhttp-config")["unsupported-setting"] = "yes"
	if Validate(tpl, Options{}) == nil {
		t.Fatal("unknown XHTTP setting accepted")
	}
	for _, typ := range []string{"tuic", "anytls", "trusttunnel", "shadowquic", "mieru", "shadowsocks", "sudoku", "snell"} {
		if code(Validate(Template{"type": typ}, Options{})) != "config_type" {
			t.Fatalf("removed protocol accepted: %s", typ)
		}
	}
}
