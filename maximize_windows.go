package main

import "syscall"

func maximizeTerminal() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	user32 := syscall.NewLazyDLL("user32.dll")
	getWin := kernel32.NewProc("GetConsoleWindow")
	showWin := user32.NewProc("ShowWindow")
	hwnd, _, _ := getWin.Call()
	if hwnd != 0 {
		showWin.Call(hwnd, 3) // SW_MAXIMIZE = 3
	}
}
