//go:build !linux && !windows

package systemstats

func cpuTimes() (uint64, uint64, bool) { return 0, 0, false }
func memory() (uint64, uint64)         { return 0, 0 }
func disk(string) (uint64, uint64)     { return 0, 0 }
