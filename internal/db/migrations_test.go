package db

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func migrationDB(t *testing.T) *DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := stdlib.OpenDB(*cfg)
	schema := fmt.Sprintf("migrations_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg.RuntimeParams["search_path"] = schema
	database := &DB{DB: stdlib.OpenDB(*cfg)}
	t.Cleanup(func() {
		database.Close()
		if _, err := admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	return database
}

func TestMigrateFilesystemOrderIdempotencyAndRollbackDB(t *testing.T) {
	database := migrationDB(t)
	source := fstest.MapFS{
		"001_first.sql":                   {Data: []byte(`CREATE TABLE probe(id INTEGER PRIMARY KEY);INSERT INTO probe VALUES(1)`)},
		"002_second.sql":                  {Data: []byte(`INSERT INTO probe VALUES(2);SELECT missing_function()`)},
		"._resource_fork.sql":             {Data: []byte{0, 255, 0}},
		"nested/not_a_root_migration.sql": {Data: []byte(`invalid SQL`)},
	}
	if err := database.MigrateFS(t.Context(), source); err == nil {
		t.Fatal("invalid migration succeeded")
	}
	var rows, versions int
	if err := database.QueryRow(`SELECT COUNT(*) FROM probe`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("failed migration escaped rollback: %d %v", rows, err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&versions); err != nil || versions != 1 {
		t.Fatalf("failed migration recorded: %d %v", versions, err)
	}
	source["002_second.sql"] = &fstest.MapFile{Data: []byte(`INSERT INTO probe VALUES(2)`)}
	for i := 0; i < 2; i++ {
		if err := database.MigrateFS(t.Context(), source); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM probe`).Scan(&rows); err != nil || rows != 2 {
		t.Fatalf("migration replayed or order changed: %d %v", rows, err)
	}
}

func TestConcurrentMigrationSessionsApplyEachVersionOnceDB(t *testing.T) {
	database := migrationDB(t)
	source := fstest.MapFS{"001_once.sql": {Data: []byte(`SELECT pg_sleep(0.03);CREATE TABLE probe(id INTEGER);INSERT INTO probe VALUES(1)`)}}
	var wg sync.WaitGroup
	results := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- database.MigrateFS(t.Context(), source) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	var rows int
	if err := database.QueryRow(`SELECT COUNT(*) FROM probe`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("concurrent migration duplicated data: %d %v", rows, err)
	}
}

func TestCancelledMigrationDoesNotRetainLocksOrPartialChangesDB(t *testing.T) {
	database := migrationDB(t)
	blocker, err := database.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := blocker.ExecContext(t.Context(), `SELECT pg_advisory_lock($1::bigint)`, MigrationLockID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Millisecond)
	err = database.MigrateFS(ctx, fstest.MapFS{"001.sql": {Data: []byte(`CREATE TABLE probe(id INTEGER)`)}})
	cancel()
	if err == nil {
		t.Fatal("migration ignored lock deadline")
	}
	if _, err := blocker.ExecContext(t.Context(), `SELECT pg_advisory_unlock($1::bigint)`, MigrationLockID); err != nil {
		t.Fatal(err)
	}
	blocker.Close()
	ctx, cancel = context.WithTimeout(t.Context(), 40*time.Millisecond)
	err = database.MigrateFS(ctx, fstest.MapFS{"001.sql": {Data: []byte(`CREATE TABLE probe(id INTEGER);SELECT pg_sleep(5);INSERT INTO probe VALUES(1)`)}})
	cancel()
	if err == nil {
		t.Fatal("migration ignored cancellation")
	}
	ctx, cancel = context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if err := database.MigrateFS(ctx, fstest.MapFS{"001.sql": {Data: []byte(`CREATE TABLE probe(id INTEGER);INSERT INTO probe VALUES(2)`)}}); err != nil {
		t.Fatalf("cancelled transaction leaked a table or lock: %v", err)
	}
}

func TestConnectionLoggingNeverIncludesPasswordSpellings(t *testing.T) {
	for _, dsn := range []string{
		"postgres://operator:visible-password@localhost/panel?sslmode=disable",
		"postgres://operator@localhost/panel?password=query-password&sslmode=disable",
		"host=localhost user=operator dbname=panel password='escaped secret' sslmode=disable",
		"broken password=unparseable-secret",
	} {
		logged := sanitizeDSN(dsn)
		for _, secret := range []string{"visible-password", "query-password", "escaped secret", "unparseable-secret"} {
			if strings.Contains(logged, secret) {
				t.Fatalf("connection logging exposed a password")
			}
		}
	}
}
