package audit

import (
	"context"

	"github.com/axottle/axottle/internal/auth"
)

// Category values for the panel's three-way split of the audit journal.
// "General" is not a stored value — it is simply "no category filter".
const (
	CategoryUser   = "user"   // an operator changed something through the panel
	CategorySystem = "system" // automation / agent / cron changed something
)

// Status values for an audited action.
const (
	StatusOK     = "ok"
	StatusFailed = "failed"
)

// ActorType values.
const (
	ActorAdmin  = "admin"
	ActorSystem = "system"
)

// RequestInfo carries correlation data only. It never determines whether an
// operation happened or which action to record.
type RequestInfo struct {
	Method, Path, RequestID, IP string
}

type ctxKey string

const requestInfoKey ctxKey = "audit.request_info"

func WithRequestInfo(ctx context.Context, info RequestInfo) context.Context {
	return context.WithValue(ctx, requestInfoKey, info)
}

// ResolveActor derives (actorType, actorID) from the request context. An
// authenticated admin => an operator-initiated change ("admin"/username);
// otherwise the change is attributed to the system (agent enrollment, cron,
// automation).
func ResolveActor(ctx context.Context) (actorType, actorID string) {
	if a := auth.AdminFromContext(ctx); a != nil && a.Username != "" {
		return ActorAdmin, a.Username
	}
	return ActorSystem, ActorSystem
}

// CategoryForActorType maps an actor type onto one of the two journal
// categories. This is the single source of truth mirrored by the read API's
// legacy-row inference (actor_type='system' => system, otherwise user).
func CategoryForActorType(actorType string) string {
	if actorType == ActorSystem {
		return CategorySystem
	}
	return CategoryUser
}
