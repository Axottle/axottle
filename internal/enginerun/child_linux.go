package enginerun

import (
	"os/exec"
	"syscall"
)

func protectChild(cmd *exec.Cmd) {
	// systemd intentionally leaves the updater alive when its parent stops.
	// The engine, however, must also stop if the core crashes or is SIGKILLed.
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}
