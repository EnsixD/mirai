package systemstats

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

var kernel = windows.NewLazySystemDLL("kernel32.dll")
var getSystemTimes = kernel.NewProc("GetSystemTimes")
var globalMemoryStatus = kernel.NewProc("GlobalMemoryStatusEx")

func cpuTimes() (idle, total uint64, ok bool) {
	var i, k, u windows.Filetime
	result, _, _ := getSystemTimes.Call(uintptr(unsafe.Pointer(&i)), uintptr(unsafe.Pointer(&k)), uintptr(unsafe.Pointer(&u)))
	if result == 0 {
		return
	}
	value := func(t windows.Filetime) uint64 { return uint64(t.HighDateTime)<<32 | uint64(t.LowDateTime) }
	// Windows kernel time includes idle time.
	return value(i), value(k) + value(u), true
}

func memory() (total, used uint64) {
	var s struct {
		Length, Load                                                                          uint32
		TotalPhys, AvailPhys, TotalPage, AvailPage, TotalVirtual, AvailVirtual, AvailExtended uint64
	}
	s.Length = uint32(unsafe.Sizeof(s))
	result, _, _ := globalMemoryStatus.Call(uintptr(unsafe.Pointer(&s)))
	if result == 0 {
		return
	}
	total = s.TotalPhys
	if s.AvailPhys <= total {
		used = total - s.AvailPhys
	}
	return
}

func disk(path string) (total, used uint64) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	var available, free uint64
	if windows.GetDiskFreeSpaceEx(p, &available, &total, &free) != nil {
		return 0, 0
	}
	if free <= total {
		used = total - free
	}
	return
}
