//go:build windows

package main

import "syscall"

// hideConsole hides the console window of this process on Windows.
// Called before server startup so Spore-launched claw-api.exe runs silently.
// CLI subcommands (token, help, etc.) return before this is called.
func hideConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	user32 := syscall.NewLazyDLL("user32.dll")

	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	showWindow := user32.NewProc("ShowWindow")

	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd != 0 {
		const swHide = 0
		showWindow.Call(hwnd, swHide)
	}
}
