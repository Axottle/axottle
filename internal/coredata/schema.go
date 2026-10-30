// Package coredata owns the database schema required by the standalone core.
package coredata

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	"github.com/axottle/axottle/internal/db"
)

//go:embed schema.sql
var schema string

// Migrate installs only common tables. Legacy installations must first finish
// their existing migrations in the matching engine, which preserves all module
// data and performs historical account conversions before this baseline opens.
func Migrate(ctx context.Context, database *sql.DB) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, db.MigrationLockID); err != nil {
		return err
	}
	var legacy bool
	if err := tx.QueryRowContext(ctx, `SELECT to_regclass('schema_migrations') IS NOT NULL`).Scan(&legacy); err != nil {
		return err
	}
	if legacy {
		var ready bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version='192_remove_commercial_licensing')`).Scan(&ready); err != nil {
			return err
		}
		if !ready {
			return fmt.Errorf("existing Axottle database needs migrations from the matching engine before the core can open it")
		}
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS core_schema_migrations(version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM core_schema_migrations WHERE version='001_common')`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		if _, err := tx.ExecContext(ctx, schema); err != nil {
			return fmt.Errorf("install core schema: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO core_schema_migrations(version) VALUES('001_common')`); err != nil {
			return err
		}
	}
	return tx.Commit()
}
