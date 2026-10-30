//go:build !linux

package enginerun

import "os/exec"

func protectChild(cmd *exec.Cmd) {}
