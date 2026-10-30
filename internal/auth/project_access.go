package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

var ErrProjectsRequired = errors.New("Выберите хотя бы один проект")

// ProjectIDs are explicit grants. Only the protected superuser has implicit
// access to projects created in the future; the permission wildcard is local.
func (s *Service) ProjectIDs(ctx context.Context, adminID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT project_id FROM admin_projects WHERE admin_id=$1 ORDER BY project_id`, adminID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func insertProjectGrants(ctx context.Context, tx *sql.Tx, adminID string, ids []string) error {
	if len(ids) == 0 {
		return ErrProjectsRequired
	}
	for _, id := range ids {
		if strings.TrimSpace(id) == "" {
			return ErrProjectsRequired
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO admin_projects(admin_id, project_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, adminID, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) SetProjectIDs(ctx context.Context, adminID string, ids []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var super bool
	if err := tx.QueryRowContext(ctx, `SELECT is_bootstrap_superuser FROM admins WHERE id=$1 FOR UPDATE`, adminID).Scan(&super); err != nil {
		return err
	}
	if super {
		return ErrSuperuserProtected
	}
	if len(ids) == 0 {
		return ErrProjectsRequired
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM admin_projects WHERE admin_id=$1`, adminID); err != nil {
		return err
	}
	if err := insertProjectGrants(ctx, tx, adminID, ids); err != nil {
		return err
	}
	return tx.Commit()
}
