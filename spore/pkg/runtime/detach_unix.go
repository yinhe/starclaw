//go:build !windows

package runtime

import "syscall"

// detachAttr returns Unix-specific process attributes to detach the child process.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setsid: true,
	}
}
