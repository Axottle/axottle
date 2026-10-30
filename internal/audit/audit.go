package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/axottle/axottle/internal/db"
	"github.com/axottle/axottle/internal/projects"
)

// Entry is one audit-journal record. The first six fields are the original
// (v1) who/what/when payload; the rest were added for the full audit journal:
// a human-readable summary + object name, a before->after state snapshot with a
// pre-computed field-level diff, a status, and request metadata filled by the
// request context.
//
// Category/ActorType/ActorID are derived from the request context when left
// empty (see ResolveActor / CategoryForActorType), so existing call sites keep
// working and automatically gain a category.
type Entry struct {
	Category     string // "user" | "system"; empty => derived from ActorType
	ActorType    string // "admin" | "system"; empty => derived from context
	ActorID      string // username / "system"; empty => derived from context
	Action       string // stable machine action key, e.g. "node.updated"
	ResourceType string // "node" | "connection_unit" | ...
	ResourceID   string // opaque id of the affected object
	ResourceName string // human-readable object name, e.g. "Frankfurt-01"
	Summary      string // human-readable one-liner for the journal row
	Status       string // "ok" | "failed"; empty => "ok"
	Before       any    // object state before the change (nil for create)
	After        any    // object state after the change (nil for delete)
	Changes      []Change
	Method       string // HTTP verb (request context)
	Path         string // request path (request context)
	RequestID    string // correlates with the request log
	IP           string // acting client IP
	Metadata     map[string]any
}

// WithDiff fills Changes from Before/After using labels and returns the entry,
// for fluent construction: audit.Entry{Before: b, After: a, ...}.WithDiff(lbl).
func (e Entry) WithDiff(labels map[string]string) Entry {
	e.Changes = Diff(e.Before, e.After, labels)
	return e
}

type Service struct {
	db     *db.DB
	logger *slog.Logger
}

func NewService(database *db.DB, logger *slog.Logger) *Service {
	return &Service{db: database, logger: logger}
}

// Log writes an audit entry. It is fire-and-forget: a write failure is logged
// but never propagated, so auditing can never break the business operation.
// nil-safe.
func (s *Service) Log(ctx context.Context, entry Entry) {
	if s == nil {
		return
	}

	if err := write(ctx, s.db, entry); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
			return
		}
		if s.logger != nil {
			s.logger.Error("failed to write audit log", "error", err, "action", entry.Action)
		}
	}
}

// WriteTx appends an explicit operation result to the caller's transaction.
// The caller must roll back on error and commit only after this succeeds, so
// the business change and its audit record become durable together.
func WriteTx(ctx context.Context, tx *sql.Tx, entry Entry) error {
	if tx == nil {
		return errors.New("audit requires a transaction")
	}
	return write(ctx, tx, entry)
}

type executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func write(ctx context.Context, executor executor, entry Entry) error {
	if id := projects.ID(ctx); id != "" {
		metadata := make(map[string]any, len(entry.Metadata)+1)
		for key, value := range entry.Metadata {
			metadata[key] = value
		}
		metadata["project_id"] = id
		entry.Metadata = metadata
	}
	if entry.Action == "" {
		return errors.New("audit action is required")
	}
	// Derive actor + category from the request context when the caller left
	// them unset. Category always follows the (possibly derived) actor type so
	// it matches the read API's legacy-row inference.
	if entry.ActorType == "" || entry.ActorID == "" {
		at, aid := ResolveActor(ctx)
		if entry.ActorType == "" {
			entry.ActorType = at
		}
		if entry.ActorID == "" {
			entry.ActorID = aid
		}
	}
	if entry.Category == "" {
		entry.Category = CategoryForActorType(entry.ActorType)
	}
	if entry.Status == "" {
		entry.Status = StatusOK
	}

	info, _ := ctx.Value(requestInfoKey).(RequestInfo)
	if entry.Method == "" {
		entry.Method = info.Method
	}
	if entry.Path == "" {
		entry.Path = info.Path
	}
	if entry.RequestID == "" {
		entry.RequestID = info.RequestID
	}
	if entry.IP == "" {
		entry.IP = info.IP
	}
	payloads := []any{entry.Metadata, entry.Before, entry.After, nonEmptyChanges(entry.Changes)}
	encoded := make([]any, len(payloads))
	for i, payload := range payloads {
		if payload == nil {
			continue
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode audit %s: %w", entry.Action, err)
		}
		encoded[i] = string(data)
	}

	_, err := executor.ExecContext(ctx, `
		INSERT INTO audit_log (
			actor_type, actor_id, action, resource_type, resource_id, metadata_json,
			category, summary, resource_name,
			before_json, after_json, changes_json,
			status, method, path, request_id, ip
		) VALUES (
			$1, $2, $3, $4, $5, $6::jsonb,
			$7, $8, $9,
			$10::jsonb, $11::jsonb, $12::jsonb,
			$13, $14, $15, $16, $17
		)
	`,
		entry.ActorType, entry.ActorID, entry.Action, entry.ResourceType, entry.ResourceID, encoded[0],
		entry.Category, nullStr(entry.Summary), nullStr(entry.ResourceName),
		encoded[1], encoded[2], encoded[3],
		entry.Status, nullStr(entry.Method), nullStr(entry.Path), nullStr(entry.RequestID), nullStr(entry.IP),
	)
	if err != nil {
		return fmt.Errorf("write audit %s: %w", entry.Action, err)
	}
	return nil
}

// nullStr maps "" to a SQL NULL so empty optional columns stay NULL (the read
// API distinguishes NULL/legacy from a set value).
func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nonEmptyChanges(c []Change) any {
	if len(c) == 0 {
		return nil
	}
	return c
}
