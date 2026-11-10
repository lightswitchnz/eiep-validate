package main

import (
	"os"
	"syscall"
	"unsafe"
)

func enableVT() bool {
	const enableVirtualTerminalProcessing = 0x0004
	k := syscall.NewLazyDLL("kernel32.dll")
	h := syscall.Handle(os.Stdout.Fd())
	var mode uint32
	if r, _, _ := k.NewProc("GetConsoleMode").Call(uintptr(h), uintptr(unsafe.Pointer(&mode))); r == 0 {
		return false
	}
	r, _, _ := k.NewProc("SetConsoleMode").Call(uintptr(h), uintptr(mode|enableVirtualTerminalProcessing))
	return r != 0
}
