package secretstore

import (
	"bytes"
	"context"
	"database/sql"
	"testing"
)

// fakeDB captures the UPDATE that a legacy-key read triggers. QueryRowContext
// is unused by the code under test (open works on already-read columns), so it
// panics rather than pretending to return a row.
type fakeDB struct {
	execs []execCall
	err   error
}

type execCall struct {
	query string
	args  []any
}

func (f *fakeDB) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.execs = append(f.execs, execCall{query: query, args: args})
	return nil, f.err
}

func (f *fakeDB) QueryRowContext(_ context.Context, _ string, _ ...any) *sql.Row {
	panic("QueryRowContext is not expected in these tests")
}

func newTestService(t *testing.T, db dbx, master string, legacy ...string) *Service {
	t.Helper()
	key, err := deriveKey(master)
	if err != nil {
		t.Fatalf("deriveKey(%q): %v", master, err)
	}
	svc := &Service{db: db, key: key}
	for _, lm := range legacy {
		lk, lerr := deriveKey(lm)
		if lerr != nil {
			t.Fatalf("deriveKey(legacy %q): %v", lm, lerr)
		}
		svc.legacy = append(svc.legacy, lk)
	}
	return svc
}

func TestOpenReadsRowsSealedWithTheCurrentKey(t *testing.T) {
	db := &fakeDB{}
	svc := newTestService(t, db, "a-real-32-byte-master-key-value!")

	ct, nonce, err := svc.encrypt([]byte("bot-token"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	got, err := svc.open(context.Background(), "id-1", ct, nonce)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if string(got) != "bot-token" {
		t.Fatalf("open returned %q, want %q", got, "bot-token")
	}
	if len(db.execs) != 0 {
		t.Fatalf("a row already under the current key must not be rewritten, got %d writes", len(db.execs))
	}
}

// The upgrade path: an install that ran on the public dev placeholder gets a
// generated key, and its existing secrets must still open — and must be re-sealed
// under the new key so the public one stops being able to read them.
func TestOpenFallsBackToLegacyKeyAndReEncrypts(t *testing.T) {
	const legacyMaster = "dev-secret-change-me-in-production"
	db := &fakeDB{}

	legacyOnly := newTestService(t, db, legacyMaster)
	ct, nonce, err := legacyOnly.encrypt([]byte("totp-secret"))
	if err != nil {
		t.Fatalf("encrypt with legacy key: %v", err)
	}

	svc := newTestService(t, db, "generated-32-byte-master-key-xx!", legacyMaster)
	got, err := svc.open(context.Background(), "totp.admin.7", ct, nonce)
	if err != nil {
		t.Fatalf("open with legacy fallback: %v", err)
	}
	if string(got) != "totp-secret" {
		t.Fatalf("open returned %q, want %q", got, "totp-secret")
	}

	if len(db.execs) != 1 {
		t.Fatalf("expected exactly one re-encrypt write, got %d", len(db.execs))
	}
	write := db.execs[0]
	if !bytes.Contains([]byte(write.query), []byte("UPDATE secrets")) {
		t.Errorf("unexpected re-encrypt query: %s", write.query)
	}
	// Re-encryption is not an operator-visible change to the secret, so
	// updated_at must be left alone (the audit trail reads it as "rotated").
	if bytes.Contains([]byte(write.query), []byte("updated_at")) {
		t.Errorf("re-encrypt must not bump updated_at: %s", write.query)
	}
	if len(write.args) != 3 || write.args[0] != "totp.admin.7" {
		t.Fatalf("unexpected re-encrypt args: %#v", write.args)
	}
	newCT, ok := write.args[1].([]byte)
	if !ok {
		t.Fatalf("ciphertext arg is %T, want []byte", write.args[1])
	}
	newNonce, ok := write.args[2].([]byte)
	if !ok {
		t.Fatalf("nonce arg is %T, want []byte", write.args[2])
	}
	// The rewritten row must open under the NEW key alone.
	fresh := newTestService(t, &fakeDB{}, "generated-32-byte-master-key-xx!")
	plain, err := fresh.open(context.Background(), "totp.admin.7", newCT, newNonce)
	if err != nil {
		t.Fatalf("re-encrypted row does not open under the new key: %v", err)
	}
	if string(plain) != "totp-secret" {
		t.Fatalf("re-encrypted row holds %q, want %q", plain, "totp-secret")
	}
}

// A failed rewrite must not fail the read: the caller already has its plaintext
// and the next read tries again.
func TestOpenSurvivesAFailedReEncrypt(t *testing.T) {
	const legacyMaster = "dev-secret-change-me-in-production"
	sealed := newTestService(t, &fakeDB{}, legacyMaster)
	ct, nonce, err := sealed.encrypt([]byte("s3-secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	db := &fakeDB{err: sql.ErrConnDone}
	svc := newTestService(t, db, "generated-32-byte-master-key-xx!", legacyMaster)

	got, err := svc.open(context.Background(), "backup_s3_1", ct, nonce)
	if err != nil {
		t.Fatalf("open must succeed despite the failed rewrite: %v", err)
	}
	if string(got) != "s3-secret" {
		t.Fatalf("open returned %q, want %q", got, "s3-secret")
	}
}

func TestOpenReportsFailureWhenNoKeyFits(t *testing.T) {
	sealed := newTestService(t, &fakeDB{}, "some-other-master-key-entirely!!")
	ct, nonce, err := sealed.encrypt([]byte("unreachable"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	svc := newTestService(t, &fakeDB{}, "generated-32-byte-master-key-xx!", "dev-secret-change-me-in-production")
	if _, err := svc.open(context.Background(), "id", ct, nonce); err == nil {
		t.Fatal("open must fail when neither the current nor a legacy key opens the row")
	}
}

// New must not keep a legacy key that derives to the same bytes as the current
// one — that would make every read do redundant work and rewrite rows for
// nothing.
func TestNewSkipsRedundantLegacyKeys(t *testing.T) {
	svc, err := New(&sql.DB{}, "dev-secret-change-me-in-production", "dev-secret-change-me-in-production")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(svc.legacy) != 0 {
		t.Fatalf("legacy keys = %d, want 0 for a duplicate of the current key", len(svc.legacy))
	}
}
