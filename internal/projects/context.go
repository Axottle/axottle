package projects

import (
	"context"
	"strings"
)

type workspaceKey struct{}
type Workspace struct{ ProjectID, ConnectionID string }

func WithWorkspace(ctx context.Context, projectID, connectionID string) context.Context {
	return context.WithValue(ctx, workspaceKey{}, Workspace{projectID, connectionID})
}
func Current(ctx context.Context) Workspace                { w, _ := ctx.Value(workspaceKey{}).(Workspace); return w }
func ID(ctx context.Context) string                        { return Current(ctx).ProjectID }
func WithoutWorkspace(ctx context.Context) context.Context { return WithWorkspace(ctx, "", "") }

// Predicate is a SQL boolean expression for a trusted column expression. The
// identifier comes from source code; the only value is quoted as a PostgreSQL
// escape string literal (including apostrophes and backslashes). Background workers without
// a workspace keep their inventory, and derive the owner of each queued job.
func Predicate(ctx context.Context, column string) string {
	if id := ID(ctx); id != "" {
		return column + " = E'" + strings.NewReplacer("\\", "\\\\", "'", "''").Replace(id) + "'"
	}
	return "TRUE"
}
