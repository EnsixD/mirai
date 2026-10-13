package node

import (
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
)

type udpKey struct{ ip, inbound, dest string }

type udpFlow struct {
	start, last time.Time // guarded by slot.mu
	up, down    atomic.Int64
}

func (c *countingConn) record(end time.Time) nodeapi.Conn {
	r := nodeapi.Conn{Slot: c.slot.name, IP: c.ip, Inbound: c.inName, Network: "tcp", Dest: c.dest,
		Start: c.started.Unix(), Up: c.up.Load(), Down: c.down.Load()}
	if !end.IsZero() {
		r.End = end.Unix()
	}
	return r
}

func (f *udpFlow) record(s *slot, k udpKey, open bool) nodeapi.Conn {
	r := nodeapi.Conn{Slot: s.name, IP: k.ip, Inbound: k.inbound, Network: "udp", Dest: k.dest,
		Start: f.start.Unix(), Up: f.up.Load(), Down: f.down.Load()}
	if !open {
		r.End = f.last.Unix()
	}
	return r
}

// logLocked adds a closed connection, dropping the oldest past connKeep; s.mu is held.
func (s *slot) logLocked(c nodeapi.Conn) {
	if len(s.recent) >= connKeep {
		n := copy(s.recent, s.recent[len(s.recent)-connKeep+1:])
		s.recent = s.recent[:n]
	}
	s.recent = append(s.recent, c)
}

func (s *slot) setDest(c *countingConn, dest string) {
	s.mu.Lock()
	// A closed connection is in the log already, under the address it was opened with.
	if _, open := s.conns[c]; open {
		c.dest = dest
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
			s.logLocked(f.record(s, k, false))
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
			out.Conns = append(out.Conns, c.record(time.Time{}))
		}
		for k, f := range s.udp {
			out.Conns = append(out.Conns, f.record(s, k, true))
		}
		s.mu.Unlock()
	}
	sort.SliceStable(out.Conns, func(i, j int) bool { return out.Conns[i].Start > out.Conns[j].Start })
	return out
}
