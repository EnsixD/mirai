package node

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"mirai/internal/nodeapi"
)

// forwards holds the node's TCP pass-throughs (nodeapi.Forward) by listen address. They
// live beside mihomo: the bytes go through untouched, so the panel's certificate reaches
// its clients as it is and the node holds no key of the panel's.
type forwards struct {
	mu   sync.Mutex
	log  *slog.Logger
	open map[string]*forwarder
}

type forwarder struct {
	ln     net.Listener
	target string
}

// apply makes the running pass-throughs those of want: unchanged ones keep their
// connections, the rest are closed before new ones bind, so a port can change hands.
// It returns the listen errors by address.
func (f *forwards) apply(want []nodeapi.Forward) map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.open == nil {
		f.open = map[string]*forwarder{}
	}
	next := map[string]string{}
	for _, w := range want {
		next[w.Listen] = w.Target
	}
	for addr, fw := range f.open {
		if next[addr] != fw.target {
			fw.close()
			delete(f.open, addr)
		}
	}
	errs := map[string]string{}
	for addr, target := range next {
		if _, ok := f.open[addr]; ok {
			continue
		}
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			errs[addr] = err.Error()
			f.log.Error("forward listen", "listen", addr, "err", err)
			continue
		}
		fw := &forwarder{ln: ln, target: target}
		f.open[addr] = fw
		go fw.serve(f.log)
		f.log.Info("forward", "listen", addr, "target", target)
	}
	return errs
}

// close stops new connections; open ones run on to the target they started with, so
// an Apply never waits for a download to end.
func (fw *forwarder) close() { _ = fw.ln.Close() }

func (fw *forwarder) serve(log *slog.Logger) {
	for {
		c, err := fw.ln.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Warn("forward accept", "listen", fw.ln.Addr().String(), "err", err)
			}
			return
		}
		go pipe(c, fw.target)
	}
}

// forwardIdle closes a pass-through nobody has written to for this long.
const forwardIdle = 5 * time.Minute

func pipe(client net.Conn, target string) {
	defer client.Close()
	up, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		return
	}
	defer up.Close()
	done := make(chan struct{}, 2)
	cp := func(dst, src net.Conn) {
		buf := make([]byte, 32<<10)
		for {
			_ = src.SetReadDeadline(time.Now().Add(forwardIdle))
			n, err := src.Read(buf)
			if n > 0 {
				if _, werr := dst.Write(buf[:n]); werr != nil {
					dst.Close()
					src.Close()
					break
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					// a reset or the idle deadline: the other direction cannot go on either
					dst.Close()
					src.Close()
					break
				}
				// the other side may still be talking: half-close this direction
				if tc, ok := dst.(*net.TCPConn); ok {
					_ = tc.CloseWrite()
				}
				break
			}
		}
		done <- struct{}{}
	}
	go cp(up, client)
	go cp(client, up)
	<-done
	<-done
}
