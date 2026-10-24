package node

import (
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"mirai/internal/nodeapi"
)

func TestForwardPassesBytesThroughAndLetsThePortGo(t *testing.T) {
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		for {
			c, err := echo.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); _, _ = io.Copy(c, c) }()
		}
	}()

	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := free.Addr().String()
	free.Close()

	f := forwards{log: slog.New(slog.DiscardHandler)}
	if errs := f.apply([]nodeapi.Forward{{Listen: addr, Target: echo.Addr().String()}}); len(errs) != 0 {
		t.Fatalf("apply: %v", errs)
	}
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := c.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 5)
	if _, err := io.ReadFull(c, got); err != nil || string(got) != "hello" {
		t.Fatalf("echo through the forward: %q %v", got, err)
	}
	c.Close()

	// An unchanged forward stays the same listener.
	before := f.open[addr]
	f.apply([]nodeapi.Forward{{Listen: addr, Target: echo.Addr().String()}})
	if f.open[addr] != before {
		t.Fatal("an unchanged forward was reopened")
	}

	f.apply(nil)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("the port is still held after the forward went: %v", err)
	}
	l.Close()
}
