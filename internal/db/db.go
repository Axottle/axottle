package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	*sql.DB
	logger *slog.Logger
}

// Open establishes a PostgreSQL connection pool.
//
// dsn is a libpq connection string, e.g.
//
//	postgres://axottle:axottle_dev@localhost:5432/axottle?sslmode=disable
func Open(dsn string, logger *slog.Logger) (*DB, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if dsn == "" {
		return nil, fmt.Errorf("empty database DSN")
	}

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	logger.Info("database connected", "driver", "pgx", "dsn", sanitizeDSN(dsn))

	return &DB{DB: sqlDB, logger: logger}, nil
}

// MigrationLockID coordinates common-core and engine schema changes. The
// engine holds a dedicated session lock across its per-file transactions;
// the standalone core takes the same key for its baseline transaction.
const MigrationLockID int64 = 1616531332

// Migrate preserves the development/test API for explicitly selected files.
// Installed engine startup uses its immutable embedded filesystem instead.
func (d *DB) Migrate(directory string) error {
	return d.MigrateFS(context.Background(), os.DirFS(directory))
}

// MigrateFS applies lexically ordered SQL files exactly once using the existing
// schema_migrations keys. Every file and its version record share a transaction.
func (d *DB) MigrateFS(ctx context.Context, source fs.FS) error {
	if source == nil {
		return fmt.Errorf("migration filesystem is required")
	}
	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		return fmt.Errorf("read migrations filesystem: %w", err)
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".sql") {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	conn, err := d.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Close()
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(cleanup, `SELECT pg_advisory_unlock($1::bigint)`, MigrationLockID); err != nil {
			// A failed unlock must never return a session-level lock to the pool.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1::bigint)`, MigrationLockID); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}

	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version TEXT PRIMARY KEY,applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}
	for _, name := range files {
		version := strings.TrimSuffix(name, ".sql")
		var applied bool
		if err := conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if applied {
			continue
		}
		body, err := fs.ReadFile(source, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("execute migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
		if d.logger != nil {
			d.logger.Info("migration applied", "version", version)
		}
	}
	return nil
}

func sanitizeDSN(dsn string) string {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return "[configured PostgreSQL connection]"
	}
	// Reconstruct a small non-secret projection rather than trying to redact
	// every URL/query/escaped-libpq spelling of a password or connection option.
	return fmt.Sprintf("host=%s port=%d user=%s dbname=%s", cfg.Host, cfg.Port, cfg.User, cfg.Database)
}
