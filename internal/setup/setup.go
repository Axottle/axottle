package setup

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/axottle/axottle/internal/db"
	"golang.org/x/crypto/bcrypt"
)

// OperatingMode defines whether Remnawave is required or optional.
type OperatingMode string

const (
	ModeUnset         OperatingMode = ""
	ModeWithRemnawave OperatingMode = "with_remnawave" // full mode: gateway + remnawave sync
	ModeStandalone    OperatingMode = "standalone"     // gateway-only, no remnawave required
)

type State struct {
	// Step completion flags
	ModeSelected     bool `json:"mode_selected"`
	AdminCreated     bool `json:"admin_created"`
	DomainConfigured bool `json:"domain_configured"`
	SetupComplete    bool `json:"setup_complete"`

	// Runtime info (not persisted as steps)
	RemnawaveLinked bool          `json:"remnawave_linked"`
	Mode            OperatingMode `json:"mode"`
	Domain          string        `json:"domain,omitempty"`
	SSLMode         string        `json:"ssl_mode,omitempty"`   // auto / manual / none
	SSLStatus       string        `json:"ssl_status,omitempty"` // pending / active / failed
}

type Service struct {
	db             *db.DB
	logger         *slog.Logger
	enrichState    func(context.Context, *State) error
	provisionAdmin func(context.Context, *sql.Tx, string, string) error
}

func NewService(database *db.DB, logger *slog.Logger) *Service {
	return &Service{db: database, logger: logger}
}

func (s *Service) GetState(ctx context.Context) (*State, error) {
	state := &State{}

	// Check mode
	modeStr := s.getSettingString(ctx, "setup.mode")
	state.Mode = OperatingMode(modeStr)
	state.ModeSelected = modeStr != ""

	// Check admin created
	var adminCount int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admins").Scan(&adminCount); err != nil {
		return nil, fmt.Errorf("read bootstrap administrator: %w", err)
	}
	state.AdminCreated = adminCount > 0

	// Check domain
	state.Domain = s.getSettingString(ctx, "setup.domain")
	state.DomainConfigured = state.Domain != ""
	state.SSLMode = s.getSettingString(ctx, "setup.ssl_mode")
	state.SSLStatus = s.getSettingString(ctx, "setup.ssl_status")

	if s.enrichState != nil {
		if err := s.enrichState(ctx, state); err != nil {
			return nil, err
		}
	}

	// Setup is complete after the bootstrap administrator exists.
	state.SetupComplete = state.AdminCreated
	return state, nil
}

func (s *Service) IsComplete() bool {
	state, err := s.GetState(context.Background())
	if err != nil {
		return false
	}
	return state.SetupComplete
}

func (s *Service) SetMode(ctx context.Context, mode OperatingMode) error {
	if mode != ModeWithRemnawave && mode != ModeStandalone {
		return fmt.Errorf("invalid mode: must be 'with_remnawave' or 'standalone'")
	}
	err := s.saveSetting(ctx, "setup.mode", string(mode))
	if err != nil {
		return fmt.Errorf("save mode: %w", err)
	}
	s.markStepComplete(ctx, "mode", map[string]string{"mode": string(mode)})
	s.logger.Info("setup: mode configured", "mode", mode)
	return nil
}

func (s *Service) CreateAdmin(ctx context.Context, username, password string) error {

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serialize concurrent initial-setup requests, including different usernames.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(1616531331)`); err != nil {
		return err
	}
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM admins").Scan(&count); err != nil {
		return fmt.Errorf("read bootstrap administrator: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("admin already created")
	}

	// The install-time account is the protected bootstrap superuser (§3): full
	// access, active, and flagged so no path can delete, disable or narrow it.
	// A single-superuser partial unique index (migration 085) backs this up.
	id := generateID()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO admins (id, username, display_name, password_hash, role, permissions, status, is_bootstrap_superuser)
		 VALUES ($1, $2, $2, $3, 'admin', '["*"]'::jsonb, 'active', TRUE)`,
		id, username, string(hash),
	)
	if err != nil {
		return fmt.Errorf("create admin: %w", err)
	}

	if s.provisionAdmin != nil {
		if err := s.provisionAdmin(ctx, tx, id, username); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	s.markStepComplete(ctx, "admin", map[string]string{"username": username})
	s.logger.Info("setup: admin created", "username", username)
	return nil
}

// ConfigureDomain sets the domain and SSL mode for this installation.
func (s *Service) ConfigureDomain(ctx context.Context, domain, sslMode string) error {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" {
		return fmt.Errorf("domain required")
	}
	if sslMode == "" {
		sslMode = "auto"
	}
	if sslMode != "auto" && sslMode != "manual" && sslMode != "none" {
		return fmt.Errorf("ssl_mode must be auto, manual, or none")
	}

	if err := s.saveSetting(ctx, "setup.domain", domain); err != nil {
		return err
	}
	if err := s.saveSetting(ctx, "setup.ssl_mode", sslMode); err != nil {
		return err
	}
	if sslMode == "none" || sslMode == "manual" {
		s.saveSetting(ctx, "setup.ssl_status", sslMode)
	} else {
		s.saveSetting(ctx, "setup.ssl_status", "pending")
	}

	s.markStepComplete(ctx, "domain", map[string]string{"domain": domain, "ssl_mode": sslMode})
	s.logger.Info("setup: domain configured", "domain", domain, "ssl_mode", sslMode)
	return nil
}

// CheckDNS performs a basic DNS lookup to verify domain points somewhere.
type DNSCheckResult struct {
	Domain  string   `json:"domain"`
	IPs     []string `json:"ips"`
	OK      bool     `json:"ok"`
	Message string   `json:"message"`
}

func (s *Service) CheckDNS(ctx context.Context, domain string) (*DNSCheckResult, error) {
	result := &DNSCheckResult{Domain: domain}

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resolver := &net.Resolver{}
	addrs, err := resolver.LookupHost(ctxTimeout, domain)
	if err != nil {
		result.OK = false
		result.Message = fmt.Sprintf("DNS lookup failed: %v", err)
		return result, nil
	}

	result.IPs = addrs
	result.OK = len(addrs) > 0
	if result.OK {
		result.Message = fmt.Sprintf("Domain resolves to %s", strings.Join(addrs, ", "))
	} else {
		result.Message = "No IP addresses found"
	}
	return result, nil
}

// MarkSealed permanently closes the unauthenticated /setup/* mutators. It is
// called on the first successful admin login. The setup wizard runs entirely
// BEFORE that login (admin → login), so
// sealing on login closes the bootstrap window without ever breaking the
// wizard. Idempotent.
func (s *Service) MarkSealed(ctx context.Context) {
	s.markStepComplete(ctx, "sealed", map[string]bool{"sealed": true})
}

// IsSealed reports whether setup has been sealed (see MarkSealed). Once sealed,
// the public setup endpoints reject mutating requests so a live, operating
// panel cannot have its domain/TLS or mode reconfigured by an
// unauthenticated caller.
func (s *Service) IsSealed(ctx context.Context) bool {
	return s.isStepComplete(ctx, "sealed")
}

func (s *Service) isStepComplete(ctx context.Context, key string) bool {
	var completed bool
	err := s.db.QueryRowContext(ctx, "SELECT completed FROM setup_state WHERE key = $1", key).Scan(&completed)
	if err != nil {
		return false
	}
	return completed
}

func (s *Service) markStepComplete(ctx context.Context, key string, data any) {
	dataJSON, _ := json.Marshal(data)
	s.db.ExecContext(ctx, `
		INSERT INTO setup_state (key, completed, data_json, updated_at)
		VALUES ($1, TRUE, $2::jsonb, NOW())
		ON CONFLICT (key) DO UPDATE SET completed = TRUE, data_json = EXCLUDED.data_json, updated_at = NOW()
	`, key, string(dataJSON))
}

func (s *Service) saveSetting(ctx context.Context, key, value string) error {
	valJSON, _ := json.Marshal(value)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO system_settings (key, value_json, updated_at)
		VALUES ($1, $2::jsonb, NOW())
		ON CONFLICT (key) DO UPDATE SET value_json = EXCLUDED.value_json, updated_at = NOW()
	`, key, string(valJSON))
	return err
}

func (s *Service) getSettingString(ctx context.Context, key string) string {
	var valJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT value_json FROM system_settings WHERE key = $1", key).Scan(&valJSON)
	if err != nil {
		return ""
	}
	var s2 string
	if err := json.Unmarshal(valJSON, &s2); err != nil {
		return ""
	}
	return s2
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
