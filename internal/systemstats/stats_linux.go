package systemstats

import (
	"golang.org/x/sys/unix"
	"os"
	"strconv"
	"strings"
)

func cpuTimes() (idle, total uint64, ok bool) {
	raw, err := os.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	line, _, _ := strings.Cut(string(raw), "\n")
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return
	}
	// guest and guest_nice are already included in user/nice, so exclude them.
	for i, v := range fields[1:] {
		if i >= 8 {
			break
		}
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		total += n
		if i == 3 || i == 4 {
			idle += n
		}
	}
	return idle, total, true
}

func memory() (total, used uint64) {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return
	}
	var available uint64
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		n, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			total = n * 1024
		case "MemAvailable:":
			available = n * 1024
		}
	}
	if available <= total {
		used = total - available
	}
	return
}

func disk(path string) (total, used uint64) {
	var s unix.Statfs_t
	if unix.Statfs(path, &s) != nil || s.Bsize <= 0 {
		return
	}
	total = s.Blocks * uint64(s.Bsize)
	// Used space excludes free blocks, including filesystem-reserved free space.
	if s.Bfree <= s.Blocks {
		used = (s.Blocks - s.Bfree) * uint64(s.Bsize)
	}
	return
}
