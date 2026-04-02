package tool

import (
	"context"
	"os/exec"

	"github.com/yinhe/starclaw/internal/procutil"
)

// hiddenCmd wraps exec.Command with hidden console window on Windows.
func hiddenCmd(name string, arg ...string) *exec.Cmd {
	return procutil.Command(name, arg...)
}

// hiddenCmdCtx wraps exec.CommandContext with hidden console window on Windows.
func hiddenCmdCtx(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return procutil.CommandContext(ctx, name, arg...)
}
