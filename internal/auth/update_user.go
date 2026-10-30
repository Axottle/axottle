package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type UpdateUserInput struct {
	DisplayName *string
	Status      *string
	Permissions *[]string
	ProjectIDs  *[]string
}

// UpdateUser commits profile, permissions, memberships and module extensions
// together. A failed grant, extension or audit write leaves no partial edit.
func (s *Service) UpdateUser(ctx context.Context, id string, in UpdateUserInput, provisioners ...ProvisionUser) (*Admin, error) {
	if in.Status != nil && *in.Status != "active" && *in.Status != "invited" && *in.Status != "disabled" {
		return nil, fmt.Errorf("invalid account status")
	}
	var permissions any
	if in.Permissions != nil {
		p, err := NormalizePermissions(*in.Permissions)
		if err != nil {
			return nil, err
		}
		in.Permissions = &p
		encoded, err := marshalPermissions(p)
		if err != nil {
			return nil, err
		}
		permissions = encoded
	}
	if in.ProjectIDs != nil && len(*in.ProjectIDs) == 0 {
		return nil, ErrProjectsRequired
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Account edits are infrequent. Serialize them with legacy account writers
	// as well as this path while ordinary session reads remain available.
	if _, err := tx.ExecContext(ctx, `LOCK TABLE admins IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return nil, err
	}
	var current Admin
	if err := tx.QueryRowContext(ctx, `SELECT is_bootstrap_superuser FROM admins WHERE id=$1 FOR UPDATE`, id).Scan(&current.IsBootstrapSuperuser); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	if current.IsBootstrapSuperuser && (in.ProjectIDs != nil || in.Status != nil && *in.Status != "active" || in.Permissions != nil && (*in.Permissions)[0] != PermissionAll) {
		return nil, ErrSuperuserProtected
	}
	if in.Permissions != nil && (*in.Permissions)[0] != PermissionAll {
		var others int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM admins WHERE id<>$1 AND permissions @> '["*"]'::jsonb`, id).Scan(&others); err != nil {
			return nil, err
		}
		if others == 0 {
			return nil, ErrLastFullAdmin
		}
	}
	var raw []byte
	if err := tx.QueryRowContext(ctx, `UPDATE admins SET display_name=COALESCE($2,display_name),status=COALESCE($3,status),permissions=COALESCE($4::jsonb,permissions)
	 WHERE id=$1 RETURNING id,username,display_name,role,status,is_bootstrap_superuser,permissions,password_hash IS NOT NULL,created_at,last_login_at`, id, in.DisplayName, in.Status, permissions).Scan(&current.ID, &current.Username, &current.DisplayName, &current.Role, &current.Status, &current.IsBootstrapSuperuser, &raw, &current.HasPassword, &current.CreatedAt, &current.LastLoginAt); err != nil {
		if isSuperuserTriggerViolation(err) {
			return nil, ErrSuperuserProtected
		}
		return nil, err
	}
	current.Permissions, err = decodePermissions(raw)
	if err != nil {
		return nil, err
	}
	if in.ProjectIDs != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM admin_projects WHERE admin_id=$1`, id); err != nil {
			return nil, err
		}
		if err := insertProjectGrants(ctx, tx, id, *in.ProjectIDs); err != nil {
			return nil, err
		}
	}
	if in.Status != nil && *in.Status == "disabled" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE admin_id=$1`, id); err != nil {
			return nil, err
		}
	}
	for _, provision := range provisioners {
		if provision != nil {
			if err := provision(ctx, tx, &current); err != nil {
				return nil, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &current, nil
}
