package auth

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCredentialBoundaryFileSurvivesDatabaseRollback(t *testing.T) {
	dir := t.TempDir()
	file := NewCredentialBoundaryFile(dir)
	var gate credentialBoundary
	gate.set(file.Epoch, file.Blocked)
	old := time.Now().Add(-time.Hour)
	if !gate.accepts(old) {
		t.Fatal("missing recovery state rejected normal credentials")
	}
	path := filepath.Join(dir, "restore", "checkpoint.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	epoch := time.Now().Add(-time.Minute).UTC()
	write := func(pending bool) {
		raw, _ := json.Marshal(map[string]any{"version": 1, "epoch": epoch, "security_pending": pending})
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(true)
	if gate.accepts(time.Now()) {
		t.Fatal("recovery fence admitted newly created credential")
	}
	write(false)
	if gate.accepts(old) {
		t.Fatal("old credential resurrected after recovery fence cleared")
	}
	if !gate.accepts(epoch) || !gate.accepts(time.Now()) {
		t.Fatal("new epoch credential rejected")
	}
	// A separate process/restart must enforce the same durable epoch.
	restarted := NewCredentialBoundaryFile(dir)
	var next credentialBoundary
	next.set(restarted.Epoch, restarted.Blocked)
	if next.accepts(old) {
		t.Fatal("restart lost durable credential epoch")
	}
	for _, raw := range []string{`{`, `{"version":2}`, `{"version":1}{"security_pending":false}`, string(make([]byte, (1<<20)+1))} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if !restarted.Blocked() {
			t.Fatal("invalid recovery state failed open")
		}
	}
}

func TestCredentialBoundaryFailureBetweenCallbacksRejectsReadAndIssuance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "restore", "checkpoint.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	file := NewCredentialBoundaryFile(dir)
	good, _ := json.Marshal(map[string]any{"version": 1, "epoch": time.Now().Add(-time.Hour), "security_pending": false})
	var gate credentialBoundary
	gate.set(file.Epoch, func() bool {
		if err := os.WriteFile(path, good, 0600); err != nil {
			t.Fatal(err)
		}
		blocked := file.Blocked()
		if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
			t.Fatal(err)
		}
		return blocked
	})
	if gate.accepts(time.Now().Add(-time.Minute)) {
		t.Fatal("second checkpoint read failure admitted an existing credential")
	}
	service := &Service{}
	service.SetCredentialBoundary(gate.epoch, gate.blocked)
	if err := service.checkAccess(context.Background(), "fixture"); err == nil {
		t.Fatal("second checkpoint read failure admitted session issuance")
	}

}
