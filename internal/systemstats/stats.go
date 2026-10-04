// Package systemstats samples the machine hosting the panel, independently of VPN nodes.
package systemstats

import (
	"os"
	"runtime"
	"sync"
	"time"
)

type Snapshot struct {
	CPUCount     int       `json:"cpu_count"`
	CPUPercent   float64   `json:"cpu_percent"`
	CPUAvailable bool      `json:"cpu_available"`
	MemTotal     uint64    `json:"mem_total"`
	MemUsed      uint64    `json:"mem_used"`
	DiskTotal    uint64    `json:"disk_total"`
	DiskUsed     uint64    `json:"disk_used"`
	CollectedAt  time.Time `json:"collected_at"`
}

type Sampler struct {
	mu          sync.Mutex
	idle, total uint64
	valid       bool
	last        Snapshot
}

func New() *Sampler {
	idle, total, ok := cpuTimes()
	return &Sampler{idle: idle, total: total, valid: ok}
}

func cpuPercent(oldIdle, oldTotal, idle, total uint64) (float64, bool) {
	if total <= oldTotal || idle < oldIdle {
		return 0, false
	}
	elapsed, rest := total-oldTotal, idle-oldIdle
	if rest > elapsed {
		return 0, false
	}
	return 100 * float64(elapsed-rest) / float64(elapsed), true
}

func (s *Sampler) Read() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if !s.last.CollectedAt.IsZero() && now.Sub(s.last.CollectedAt) < 2*time.Second {
		return s.last
	}
	out := Snapshot{CPUCount: runtime.NumCPU(), CollectedAt: now.UTC()}
	idle, total, ok := cpuTimes()
	if ok && s.valid {
		out.CPUPercent, out.CPUAvailable = cpuPercent(s.idle, s.total, idle, total)
	}
	s.idle, s.total, s.valid = idle, total, ok
	out.MemTotal, out.MemUsed = memory()
	path := os.Getenv("MIRAI_DATA_DIR")
	if path == "" {
		path = "."
	}
	if _, err := os.Stat(path); err != nil {
		path = "."
	}
	out.DiskTotal, out.DiskUsed = disk(path)
	s.last = out
	return out
}
