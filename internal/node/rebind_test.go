package node

import (
	"net"
	"strconv"
	"testing"
	"time"

	C "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/listener"
	"github.com/metacubex/mihomo/tunnel"

	"mirai/internal/nodeapi"
)

// An inbound renamed on the same port: the old name has to let go of the port before the
// new one binds it, which mihomo alone does the other way round.
func TestRenamedInboundRebindsItsPort(t *testing.T) {
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := free.Addr().(*net.TCPAddr).Port
	free.Close()

	mixed := func(name string) C.InboundListener {
		l, err := listener.ParseListener(map[string]any{"name": name, "type": "mixed", "listen": "127.0.0.1", "port": port})
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	defer listener.PatchInboundListeners(map[string]C.InboundListener{}, tunnel.Tunnel, true)

	old := map[string]C.InboundListener{"import-24": mixed("import-24")}
	listener.PatchInboundListeners(old, tunnel.Tunnel, true)
	applied := map[string]nodeapi.ListenerStatus{"import-24": {Name: "import-24", OK: true}}

	next := map[string]C.InboundListener{"vless-reality": mixed("vless-reality")}
	kept := keptListeners(next, applied)
	if len(kept) != 0 {
		t.Fatalf("a renamed inbound is not kept: %v", kept)
	}
	// What Engine.Apply does: the kept part first (closing the old name), then all.
	listener.PatchInboundListeners(kept, tunnel.Tunnel, true)
	listener.PatchInboundListeners(next, tunnel.Tunnel, true)

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("the renamed inbound does not listen on %s: %v", addr, err)
	}
	c.Close()
	if _, err := net.Listen("tcp", addr); err == nil {
		t.Fatal("the port is free: the new listener did not bind it")
	}
}

func TestKeptListenersKeepsAppliedNames(t *testing.T) {
	next := map[string]C.InboundListener{"a": nil, "b": nil}
	kept := keptListeners(next, map[string]nodeapi.ListenerStatus{"a": {Name: "a"}, "gone": {Name: "gone"}})
	if _, ok := kept["a"]; !ok || len(kept) != 1 {
		t.Fatalf("kept = %v", kept)
	}
}
