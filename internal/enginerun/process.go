// Package enginerun supervises the separately distributed engine. It does not
// import any implementation of the private modules.
package enginerun

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type Process struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error // published by closing done
}

// Start verifies the physical pair before the engine can run migrations or
// launch workers. Runtime identity/readiness is checked independently by core.
func Start(ctx context.Context, binary, version, socket string, output io.Writer) (*Process, error) {
	if binary == "" || version == "" || socket == "" {
		return nil, fmt.Errorf("engine path, release version and socket are required")
	}
	check, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	actual, err := exec.CommandContext(check, binary, "version").Output()
	if err != nil {
		return nil, fmt.Errorf("read engine version: %w", err)
	}
	if strings.TrimSpace(string(actual)) != version {
		return nil, fmt.Errorf("core %s and engine %s must belong to the same release", version, strings.TrimSpace(string(actual)))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cmd := exec.Command(binary, "serve")
	cmd.Env = append(withoutSocket(os.Environ()), "AXOTTLE_ENGINE_SOCKET="+socket)
	cmd.Stdout, cmd.Stderr = output, output
	protectChild(cmd)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start engine: %w", err)
	}
	p := &Process{cmd: cmd, done: make(chan struct{})}
	go func() { p.err = cmd.Wait(); close(p.done) }()
	return p, nil
}

func withoutSocket(env []string) []string {
	out := make([]string, 0, len(env))
	for _, value := range env {
		if !strings.HasPrefix(value, "AXOTTLE_ENGINE_SOCKET=") {
			out = append(out, value)
		}
	}
	return out
}

func (p *Process) Done() <-chan struct{} { return p.done }

func (p *Process) Err() error {
	<-p.done
	return p.err
}

// Stop always reaps the child, including when it exceeds the shutdown deadline.
// This matters on legacy units using KillMode=process to preserve the updater.
func (p *Process) Stop(ctx context.Context) error {
	select {
	case <-p.done:
		return nil
	default:
	}
	_ = p.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		_ = p.cmd.Process.Kill()
		<-p.done
		return fmt.Errorf("engine exceeded shutdown deadline: %w", ctx.Err())
	}
}
