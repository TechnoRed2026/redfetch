//go:build windows

package main

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	pGlobalMemory     = kernel32.NewProc("GlobalMemoryStatusEx")
	pGetTickCount64   = kernel32.NewProc("GetTickCount64")
	pGetLogicalDrives = kernel32.NewProc("GetLogicalDrives")
	pGetDiskFreeSpace = kernel32.NewProc("GetDiskFreeSpaceExW")
	pGetDriveType     = kernel32.NewProc("GetDriveTypeW")
	pSetErrorMode     = kernel32.NewProc("SetErrorMode")
	pGetConsoleMode   = kernel32.NewProc("GetConsoleMode")
	pSetConsoleMode   = kernel32.NewProc("SetConsoleMode")
)

type memStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func gb(b uint64) float64 { return float64(b) / (1 << 30) }

func nativeRAM() string {
	m := memStatusEx{Length: uint32(unsafe.Sizeof(memStatusEx{}))}
	if r, _, _ := pGlobalMemory.Call(uintptr(unsafe.Pointer(&m))); r == 0 {
		return ""
	}
	return fmt.Sprintf("%.1f GB / %.1f GB", gb(m.TotalPhys-m.AvailPhys), gb(m.TotalPhys))
}

// ponytail: 64-bit return fits uintptr on amd64/arm64 only; add r2 high bits if 386 ever matters.
func nativeUptime() string {
	ms, _, _ := pGetTickCount64.Call()
	d := time.Duration(ms) * time.Millisecond
	return fmt.Sprintf("%dd %dh %dm", int(d.Hours())/24, int(d.Hours())%24, int(d.Minutes())%60)
}

func nativeDisks() []string {
	pSetErrorMode.Call(1) // SEM_FAILCRITICALERRORS: no "insert disk" dialogs
	mask, _, _ := pGetLogicalDrives.Call()
	var out []string
	for i := 0; i < 26; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		root, err := syscall.UTF16PtrFromString(string(rune('A'+i)) + `:\`)
		if err != nil {
			continue
		}
		// 2=removable 3=fixed 4=remote
		if t, _, _ := pGetDriveType.Call(uintptr(unsafe.Pointer(root))); t < 2 || t > 4 {
			continue
		}
		var free, total, totalFree uint64
		if r, _, _ := pGetDiskFreeSpace.Call(uintptr(unsafe.Pointer(root)), uintptr(unsafe.Pointer(&free)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&totalFree))); r == 0 {
			continue
		}
		out = append(out, fmt.Sprintf("%c: %.1f GB / %.1f GB", 'A'+i, gb(total-totalFree), gb(total)))
	}
	return out
}

func enableVT() bool {
	h, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil {
		return false
	}
	var mode uint32
	if r, _, _ := pGetConsoleMode.Call(uintptr(h), uintptr(unsafe.Pointer(&mode))); r == 0 {
		return false
	}
	r, _, _ := pSetConsoleMode.Call(uintptr(h), uintptr(mode|0x4)) // ENABLE_VIRTUAL_TERMINAL_PROCESSING
	return r != 0
}
