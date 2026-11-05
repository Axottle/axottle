package projects

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/axottle/axottle/internal/auth"
)

// AccountPredicate scopes operator discovery, assignments and Telegram access
// using the same grants as the interactive project selector.
// column must be a static, trusted SQL identifier; bind ID(ctx) at parameter.
func AccountPredicate(ctx context.Context, column string, parameter int) string {
	p := fmt.Sprintf("$%d", parameter)
	return "(" + p + "::text='' OR EXISTS (SELECT 1 FROM admins project_admin WHERE project_admin.id=" + column + " AND (project_admin.is_bootstrap_superuser OR project_admin.global_projects OR EXISTS (SELECT 1 FROM admin_projects ap WHERE ap.admin_id=project_admin.id AND ap.project_id=" + p + "))))"
}

func RequireAccount(ctx context.Context, db *sql.DB, adminID string) error {
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM admins a WHERE a.id=$1 AND `+AccountPredicate(ctx, "a.id", 2)+`)`, adminID, ID(ctx)).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

// Support roles and login status belong to an account. A project operator may
// only change an account owned solely by that project; shared accounts and
// the bootstrap user are managed centrally to avoid affecting other projects.
func RequireManagedAccount(ctx context.Context, db *sql.DB, adminID string) error {
	if a := auth.AdminFromContext(ctx); a != nil && a.IsBootstrapSuperuser {
		return nil
	}
	if ID(ctx) == "" {
		if auth.AdminFromContext(ctx) != nil {
			return ErrNotFound
		}
		return nil
	}
	if err := RequireAccount(ctx, db, adminID); err != nil {
		return err
	}
	var allowed bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM admins a WHERE a.id=$1
	        AND NOT a.is_bootstrap_superuser AND NOT a.global_projects AND NOT EXISTS(SELECT 1 FROM admin_projects ap WHERE ap.admin_id=a.id AND ap.project_id<>$2))`, adminID, ID(ctx)).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrNotFound
	}
	return nil
}
