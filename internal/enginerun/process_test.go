package enginerun

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, version, serve string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "engine")
	script := "#!/bin/sh\nif [ \"$1\" = version ]; then printf '%s\\n' '" + version + "'; exit; fi\n" + serve + "\n"
	if err := os.WriteFile(file, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestRejectsMixedReleaseBeforeStartingEngine(t *testing.T) {
	binary := fixture(t, "v2.0.0", "exit 91")
	_, err := Start(context.Background(), binary, "v1.0.0", "/tmp/unused-engine.sock", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "same release") {
		t.Fatalf("mixed release accepted: %v", err)
	}
}

func TestStopReapsChildAndPassesSocket(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "socket")
	stopped := filepath.Join(dir, "stopped")
	t.Setenv("AXOTTLE_ENGINE_SOCKET", "untrusted-inherited-value")
	binary := fixture(t, "v1.0.0", `printf '%s' "$AXOTTLE_ENGINE_SOCKET" > '`+record+`'
trap 'printf stopped > "`+stopped+`"; exit 0' TERM
while :; do sleep 0.05; done`)
	p, err := Start(context.Background(), binary, "v1.0.0", "/tmp/expected.sock", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if b, _ := os.ReadFile(record); string(b) == "/tmp/expected.sock" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if b, _ := os.ReadFile(record); string(b) != "/tmp/expected.sock" {
		p.cmd.Process.Kill()
		t.Fatalf("incorrect socket environment: %q", b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Done():
	default:
		t.Fatal("child was not reaped")
	}
	if b, _ := os.ReadFile(stopped); string(b) != "stopped" {
		t.Fatal("engine did not receive graceful termination")
	}
}

func TestUnexpectedExitIsReported(t *testing.T) {
	binary := fixture(t, "v1.0.0", "exit 23")
	p, err := Start(context.Background(), binary, "v1.0.0", "/tmp/unused.sock", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Done():
	case <-time.After(3 * time.Second):
		p.cmd.Process.Kill()
		t.Fatal("engine exit was not observed")
	}
	if err := p.Err(); err == nil || !strings.Contains(err.Error(), "23") {
		t.Fatalf("lost exit failure: %v", err)
	}
}

func TestExpiredShutdownKillsAndReapsUnresponsiveEngine(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	binary := fixture(t, "v1.0.0", `trap '' TERM
printf ready > '`+ready+`'
while :; do sleep 0.05; done`)
	p, err := Start(context.Background(), binary, "v1.0.0", "/tmp/unused.sock", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Stop(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("missing forced-shutdown failure: %v", err)
	}
	select {
	case <-p.Done():
	default:
		t.Fatal("unresponsive child survived shutdown")
	}
}
