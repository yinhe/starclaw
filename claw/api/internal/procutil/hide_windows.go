//go:build windows

package procutil

import (
	"os/exec"
	"syscall"
)

// HideWindow sets SysProcAttr to hide the console window on Windows.
func HideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	} else {
		cmd.SysProcAttr.HideWindow = true
	}
}
