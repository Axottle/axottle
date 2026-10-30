package audit

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

type recordingExecutor struct {
	args []any
	err  error
}

func (e *recordingExecutor) ExecContext(_ context.Context, _ string, args ...any) (sql.Result, error) {
	e.args = args
	return nil, e.err
}

func TestWriteExplicitEntryMetadata(t *testing.T) {
	ctx := WithRequestInfo(t.Context(), RequestInfo{Method: "POST", Path: "/nodes", RequestID: "req-1", IP: "127.0.0.1"})
	exec := &recordingExecutor{}
	if err := write(ctx, exec, Entry{Action: "node.created", ResourceID: "n1", ResourceName: "Node one"}); err != nil {
		t.Fatal(err)
	}
	for i, want := range map[int]any{0: "system", 1: "system", 2: "node.created", 4: "n1", 6: "system", 8: "Node one", 12: "ok", 13: "POST", 14: "/nodes", 15: "req-1", 16: "127.0.0.1"} {
		if exec.args[i] != want {
			t.Errorf("argument %d = %v, want %v", i, exec.args[i], want)
		}
	}
	if err := write(ctx, exec, Entry{Action: "node.created", Method: "explicit", Path: "/explicit"}); err != nil {
		t.Fatal(err)
	}
	if exec.args[13] != "explicit" || exec.args[14] != "/explicit" {
		t.Fatal("explicit metadata was overwritten")
	}
}

func TestWriteRejectsIncompleteAudit(t *testing.T) {
	for _, entry := range []Entry{{}, {Action: "node.created", After: make(chan int)}, {Action: "node.created", Metadata: map[string]any{"bad": make(chan int)}}} {
		exec := &recordingExecutor{}
		if err := write(t.Context(), exec, entry); err == nil {
			t.Fatal("expected validation error")
		}
		if exec.args != nil {
			t.Fatal("invalid entry reached database")
		}
	}
	failure := errors.New("database unavailable")
	if err := write(t.Context(), &recordingExecutor{err: failure}, Entry{Action: "node.created"}); !errors.Is(err, failure) {
		t.Fatalf("write lost underlying error: %v", err)
	}
	if err := WriteTx(t.Context(), nil, Entry{Action: "node.created"}); err == nil {
		t.Fatal("nil transaction accepted")
	}
}
