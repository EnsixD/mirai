package node

import (
	"net"
	"sort"
	"sync/atomic"
	"time"

	"mirai/internal/nodeapi"
)

// The connection log: where each slot's traffic goes. The panel shows it in a user's
// card. It is kept in memory only, a bounded number of lines per slot, and never
// reaches the node's own logs or its disk.

const (
	// connKeep is how many closed connections a slot remembers.
	connKeep = 300
	// udpIdle is how long a UDP flow stays open in the log after its last packet.
	udpIdle = time.Minute
	// udpFlows bounds the flows a slot follows at once; past it they are all closed.
	udpFlows = 256
	// namesKeep bounds the site names a slot remembers by address; past it it starts over.
	namesKeep = 1024
)

type udpKey struct{ ip, inbound, dest string }

type udpFlow struct {
	start, last time.Time // guarded by slot.mu
	up, down    atomic.Int64
}

// recordLocked is the log line of a TCP connection; a zero end: still open. s.mu is held.
func (s *slot) recordLocked(c *countingConn, end time.Time) nodeapi.Conn {
	r := nodeapi.Conn{Slot: s.name, IP: c.ip, Inbound: c.inName, Network: "tcp", Dest: c.dest, Host: c.host,
		Start: c.started.Unix(), Up: c.up.Load(), Down: c.down.Load()}
	if r.Host == "" {
		r.Host = s.nameLocked(c.dest)
	}
	if !end.IsZero() {
		r.End = end.Unix()
	}
	return r
}

// flowLocked is the log line of a UDP flow; s.mu is held.
func (s *slot) flowLocked(k udpKey, f *udpFlow, open bool) nodeapi.Conn {
	r := nodeapi.Conn{Slot: s.name, IP: k.ip, Inbound: k.inbound, Network: "udp", Dest: k.dest, Host: s.nameLocked(k.dest),
		Start: f.start.Unix(), Up: f.up.Load(), Down: f.down.Load()}
	if !open {
		r.End = f.last.Unix()
	}
	return r
}

// nameLocked is the site last seen at dest's address: a connection that is still open,
// or a UDP flow, gets the name a finished TCP connection to the same address had.
func (s *slot) nameLocked(dest string) string {
	if len(s.names) == 0 {
		return ""
	}
	host, _, err := net.SplitHostPort(dest)
	if err != nil {
		return ""
	}
	return s.names[host]
}

// logLocked adds a closed connection, dropping the oldest past connKeep; s.mu is held.
func (s *slot) logLocked(c nodeapi.Conn) {
	if len(s.recent) >= connKeep {
		n := copy(s.recent, s.recent[len(s.recent)-connKeep+1:])
		s.recent = s.recent[:n]
	}
	s.recent = append(s.recent, c)
}

// learn keeps the name the sniffer read for a connection to a bare IP.
func (s *slot) learn(c *countingConn, ip, host string) {
	if s.shared {
		return
	}
	s.mu.Lock()
	c.host = host
	if s.names == nil || len(s.names) >= namesKeep {
		s.names = map[string]string{}
	}
	s.names[ip] = host
	// The connection may have been closed and logged before its name was known.
	for i := len(s.recent) - 1; i >= 0 && i >= len(s.recent)-8; i-- {
		if r := &s.recent[i]; r.Host == "" && r.Dest == c.dest && r.IP == c.ip && r.Start == c.started.Unix() {
			r.Host = host
			break
		}
	}
	s.mu.Unlock()
}

// flowOf is the log line of a UDP packet's flow; nil for a shared listener.
func (s *slot) flowOf(ip, inbound, dest string, now time.Time) *udpFlow {
	if s.shared {
		return nil
	}
	k := udpKey{ip, inbound, dest}
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.udp[k]
	if f == nil {
		s.sweepUDPLocked(now, len(s.udp) >= udpFlows)
		if s.udp == nil {
			s.udp = map[udpKey]*udpFlow{}
		}
		f = &udpFlow{start: now}
		s.udp[k] = f
	}
	f.last = now
	return f
}

// sweepUDPLocked moves the flows that went quiet to the closed ones; all of them when
// the slot follows too many. s.mu is held.
func (s *slot) sweepUDPLocked(now time.Time, all bool) {
	for k, f := range s.udp {
		if all || now.Sub(f.last) > udpIdle {
			s.logLocked(s.flowLocked(k, f, false))
			delete(s.udp, k)
		}
	}
}

// Connections is the connection log of the named slots, newest first.
func (r *Registry) Connections(names []string) nodeapi.Connections {
	r.mu.RLock()
	slots := make([]*slot, 0, len(names))
	for _, n := range names {
		if s := r.byName[n]; s != nil {
			slots = append(slots, s)
		}
	}
	r.mu.RUnlock()
	now := r.now()
	out := nodeapi.Connections{Conns: []nodeapi.Conn{}}
	for _, s := range slots {
		s.mu.Lock()
		s.sweepUDPLocked(now, false)
		out.Conns = append(out.Conns, s.recent...)
		for c := range s.conns {
			out.Conns = append(out.Conns, s.recordLocked(c, time.Time{}))
		}
		for k, f := range s.udp {
			out.Conns = append(out.Conns, s.flowLocked(k, f, true))
		}
		s.mu.Unlock()
	}
	sort.SliceStable(out.Conns, func(i, j int) bool { return out.Conns[i].Start > out.Conns[j].Start })
	return out
}
