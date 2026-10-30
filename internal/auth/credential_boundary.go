package auth

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// credentialBoundary is neutral account security: an externally persisted
// epoch invalidates credentials resurrected by a database rollback. Callbacks
// keep the standalone account package independent of the recovery executor.
type credentialBoundary struct {
	mu      sync.RWMutex
	epoch   func() time.Time
	blocked func() bool
}

func (b *credentialBoundary) set(epoch func() time.Time, blocked func() bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.epoch = epoch
	b.blocked = blocked
}
func (b *credentialBoundary) accepts(created time.Time) bool {
	b.mu.RLock()
	epoch, blocked := b.epoch, b.blocked
	b.mu.RUnlock()
	if blocked != nil && blocked() {
		return false
	}
	return epoch == nil || !created.Before(epoch())
}
func (s *Service) SetCredentialBoundary(epoch func() time.Time, blocked func() bool) {
	s.credentialBoundary.set(epoch, blocked)
}

// CredentialBoundaryFile lets a standalone account server honor the same
// recovery fence without importing the optional engine. Missing files denote
// an installation with no database recovery. Unreadable/malformed files fail
// closed; the file is bounded and read again for each authorization.
type CredentialBoundaryFile struct{ path string }

func NewCredentialBoundaryFile(dataDir string) *CredentialBoundaryFile {
	if dataDir == "" {
		return &CredentialBoundaryFile{}
	}
	return &CredentialBoundaryFile{path: filepath.Join(dataDir, "restore", "checkpoint.json")}
}

type credentialBoundaryState struct {
	Version int       `json:"version"`
	Epoch   time.Time `json:"epoch"`
	Pending bool      `json:"security_pending"`
}

func (b *CredentialBoundaryFile) read() (credentialBoundaryState, error) {
	var state credentialBoundaryState
	if b.path == "" {
		return state, nil
	}
	f, err := os.Open(b.path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return state, err
	}
	if len(raw) > 1<<20 {
		return state, errors.New("credential boundary file too large")
	}
	if err = json.Unmarshal(raw, &state); err != nil {
		return state, err
	}
	if state.Version != 1 {
		return state, errors.New("unsupported credential boundary version")
	}
	return state, nil
}
func (b *CredentialBoundaryFile) Epoch() time.Time {
	state, err := b.read()
	if err != nil {
		// Blocked and Epoch are separate callbacks. A checkpoint read may fail
		// after Blocked already succeeded; never turn that later failure into a
		// zero epoch that admits restored credentials or new issuance.
		return time.Date(9999, time.December, 31, 23, 59, 59, 0, time.UTC)
	}
	return state.Epoch
}
func (b *CredentialBoundaryFile) Blocked() bool {
	state, err := b.read()
	return err != nil || state.Pending || time.Now().Before(state.Epoch)
}
