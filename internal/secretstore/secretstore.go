// Package secretstore provides encrypted-at-rest storage for arbitrary
// secret blobs. It is intentionally generic so multiple features (CDN
// provider credentials, future cloud-provider tokens, integration
// credentials, …) can share one table and one master key.
//
// The master key comes from AXOTTLE_SECRET_KEY. Accepted formats:
//   - 32 raw bytes (binary; rare in env vars but supported via base64/hex)
//   - 64 hex characters
//   - 44 base64 characters (standard or URL-safe, with or without padding)
//
// Anything shorter is hashed with SHA-256 to derive a 32-byte key. This
// keeps `dev-secret-change-me-in-production` working in dev without
// crashing the server, while still letting production deployments supply
// a strong random 32-byte key.
//
// We use AES-256-GCM. Each secret gets its own random 12-byte nonce. The
// ciphertext we store is `gcm.Seal(nonce, nonce, plaintext, aad)` MINUS
// the leading nonce — we keep the nonce in a separate column so future
// key-rotation tooling can re-encrypt without touching nonces.
//
// THIS PACKAGE NEVER LOGS PLAINTEXT. The only way plaintext leaves is via
// Get(). Callers must be careful not to surface it on the API.
package secretstore

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("secret not found")

// Service is the high-level API: Put / Get / Delete with AES-GCM under
// the hood. All methods take a *sql.Tx OR a *sql.DB through dbx so they
// can be inlined into a larger transaction (e.g. CDN account creation).
type Service struct {
	db  dbx
	key []byte // 32 bytes
	// legacy holds previously-used master keys, newest first. Get tries them
	// after key and re-encrypts whatever they open, which is how an install
	// that predates generated master keys migrates off the public dev default
	// without a maintenance window. Empty in a fresh install.
	legacy [][]byte
}

// dbx is the subset of database/sql used by Service. *sql.DB and *sql.Tx
// both satisfy it.
type dbx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// New derives a 32-byte AES key from the configured master and returns a
// ready-to-use Service. Empty master is rejected: callers should pass a
// dev placeholder so the SHA-256 fallback kicks in.
//
// legacyMasters are previous master keys, newest first. They are used for
// DECRYPTION ONLY: a row that opens under one of them is immediately
// re-encrypted under master (see Get). An unusable legacy master is skipped
// rather than failing construction — it is a best-effort migration path, not a
// requirement for the store to work.
func New(database *sql.DB, master string, legacyMasters ...string) (*Service, error) {
	if database == nil {
		return nil, errors.New("secretstore: database is nil")
	}
	key, err := deriveKey(master)
	if err != nil {
		return nil, err
	}
	svc := &Service{db: database, key: key}
	for _, lm := range legacyMasters {
		lk, err := deriveKey(lm)
		if err != nil {
			continue
		}
		if bytes.Equal(lk, key) {
			continue
		}
		svc.legacy = append(svc.legacy, lk)
	}
	return svc, nil
}

// WithTx returns a copy of the service that uses tx for all queries. Use
// this when a caller wants to atomically write a secret + an account row.
func (s *Service) WithTx(tx *sql.Tx) *Service {
	if tx == nil {
		return s
	}
	cp := *s
	cp.db = tx
	return &cp
}

// Put stores plaintext under id (caller-generated). If id already exists,
// the row is updated in place and updated_at is bumped. metadata is a
// free-form JSON blob for non-secret hints (e.g. filename); pass nil if
// you have nothing.
func (s *Service) Put(ctx context.Context, id, kind string, plaintext []byte, metadata map[string]any) error {
	if id == "" {
		return errors.New("secretstore: id is empty")
	}
	if kind == "" {
		return errors.New("secretstore: kind is empty")
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	ct, nonce, err := s.encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO secrets (id, kind, value_encrypted, nonce, metadata_json, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			kind = EXCLUDED.kind,
			value_encrypted = EXCLUDED.value_encrypted,
			nonce = EXCLUDED.nonce,
			metadata_json = EXCLUDED.metadata_json,
			updated_at = NOW()
	`, id, kind, ct, nonce, metaJSON)
	if err != nil {
		return fmt.Errorf("upsert secret: %w", err)
	}
	return nil
}

// Get returns the decrypted plaintext for id and the metadata blob.
// Returns ErrNotFound if the row is missing.
func (s *Service) Get(ctx context.Context, id string) ([]byte, map[string]any, error) {
	if id == "" {
		return nil, nil, errors.New("secretstore: id is empty")
	}
	var (
		ct, nonce []byte
		metaRaw   []byte
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT value_encrypted, nonce, metadata_json
		FROM secrets
		WHERE id = $1
	`, id).Scan(&ct, &nonce, &metaRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("select secret: %w", err)
	}
	pt, err := s.open(ctx, id, ct, nonce)
	if err != nil {
		return nil, nil, fmt.Errorf("decrypt: %w", err)
	}
	meta := map[string]any{}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &meta)
	}
	return pt, meta, nil
}

// Delete removes the secret. Soft-delete is left to callers (they can
// keep the row and just unlink). No error if the row is already gone.
func (s *Service) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("secretstore: id is empty")
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM secrets WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}
	return nil
}

// UpdatedAt returns the last write time for id (used for audit).
func (s *Service) UpdatedAt(ctx context.Context, id string) (time.Time, error) {
	var t time.Time
	err := s.db.QueryRowContext(ctx, `SELECT updated_at FROM secrets WHERE id = $1`, id).Scan(&t)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ErrNotFound
	}
	return t, err
}

// ─── crypto ───────────────────────────────────────────────────────────────

func (s *Service) encrypt(plaintext []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	return ct, nonce, nil
}

func (s *Service) decrypt(ct, nonce []byte) ([]byte, error) {
	return decryptWith(s.key, ct, nonce)
}

func decryptWith(key, ct, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, err
	}
	return pt, nil
}

// open decrypts a stored row under the current key, falling back to the legacy
// keys. A row that only opens under a legacy key is re-sealed under the current
// one right away, so an install that was created before the installers
// generated a master key migrates itself as its secrets get used.
//
// Two deliberate choices:
//
//   - updated_at is NOT bumped. Re-encryption is not a change to the secret,
//     and the audit trail reads updated_at as "the operator rotated this".
//   - a failed re-encrypt is swallowed. The caller already has its plaintext;
//     the next read tries again. Failing the Get would turn a cosmetic problem
//     into an outage.
func (s *Service) open(ctx context.Context, id string, ct, nonce []byte) ([]byte, error) {
	pt, err := decryptWith(s.key, ct, nonce)
	if err == nil {
		return pt, nil
	}
	for _, lk := range s.legacy {
		pt, legacyErr := decryptWith(lk, ct, nonce)
		if legacyErr != nil {
			continue
		}
		if newCT, newNonce, encErr := s.encrypt(pt); encErr == nil {
			_, _ = s.db.ExecContext(ctx, `
				UPDATE secrets SET value_encrypted = $2, nonce = $3 WHERE id = $1
			`, id, newCT, newNonce)
		}
		return pt, nil
	}
	return nil, err
}

// deriveKey accepts hex(64), base64(44±2), raw(32), or anything else
// (which it SHA-256s into 32 bytes). Empty master is rejected.
func deriveKey(master string) ([]byte, error) {
	master = strings.TrimSpace(master)
	if master == "" {
		return nil, errors.New("secretstore: master key is empty (set AXOTTLE_SECRET_KEY)")
	}
	if len(master) == 64 {
		if k, err := hex.DecodeString(master); err == nil && len(k) == 32 {
			return k, nil
		}
	}
	for _, dec := range []func(string) ([]byte, error){
		base64.RawStdEncoding.DecodeString,
		base64.StdEncoding.DecodeString,
		base64.RawURLEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
	} {
		if k, err := dec(master); err == nil && len(k) == 32 {
			return k, nil
		}
	}
	if len(master) == 32 {
		return []byte(master), nil
	}
	// Fallback: SHA-256 the input. Lets dev defaults like
	// `dev-secret-change-me-in-production` work without crashing.
	h := sha256.Sum256([]byte(master))
	return h[:], nil
}
