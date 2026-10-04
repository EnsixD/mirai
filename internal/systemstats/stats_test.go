package systemstats

import (
	"runtime"
	"testing"
)

func TestCPUCounterIntervals(t *testing.T) {
	cases := []struct {
		name       string
		a, b, c, d uint64
		pct        float64
		ok         bool
	}{
		{"quarter busy", 100, 200, 175, 300, 25, true},
		{"fully busy", 100, 200, 100, 300, 100, true},
		{"idle", 100, 200, 200, 300, 0, true},
		{"reset", 100, 200, 10, 20, 0, false},
		{"no interval", 100, 200, 100, 200, 0, false},
		{"invalid idle", 100, 200, 250, 300, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pct, ok := cpuPercent(c.a, c.b, c.c, c.d)
			if pct != c.pct || ok != c.ok {
				t.Fatalf("got %v %v", pct, ok)
			}
		})
	}
}

func TestHostMemoryAndDisk(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("unsupported host")
	}
	s := New().Read()
	if s.CPUCount < 1 || s.MemTotal == 0 || s.MemUsed > s.MemTotal || s.DiskTotal == 0 || s.DiskUsed > s.DiskTotal {
		t.Fatalf("invalid host snapshot: %+v", s)
	}
}
