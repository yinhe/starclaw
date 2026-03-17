//go:build windows

package runtime

import "syscall"

// detachAttr returns Windows-specific process attributes to fully detach the child process.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008, // CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS
	}
}
