//go:build !windows

package main

// hideConsole is a no-op on non-Windows platforms.
func hideConsole() {}
