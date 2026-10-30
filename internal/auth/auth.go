package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/axottle/axottle/internal/db"
	"github.com/axottle/axottle/internal/secretstore"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

type Admin struct {
	ID          string
	Username    string
	DisplayName string
	Role        string
	// Status is the account lifecycle: "invited", "active" or "disabled".
	Status string
	// IsBootstrapSuperuser marks the single protected install-time account.
	IsBootstrapSuperuser bool
	Permissions          []string
	// HasPassword reports whether a local password login method is set. A
	// Telegram-only user has none (password_hash IS NULL).
	HasPassword bool
	CreatedAt   time.Time
	LastLoginAt *time.Time
}

// Sentinel errors let handlers map failures onto HTTP status codes without
// string-matching. Everything else is treated as a 500.
var (
	ErrUsernameTaken      = errors.New("username already taken")
	ErrAccountNotFound    = errors.New("account not found")
	ErrWrongPassword      = errors.New("current password is incorrect")
	ErrPasswordUnchanged  = errors.New("new password must differ from the current one")
	ErrLastAdmin          = errors.New("cannot delete the last remaining admin")
	ErrEmptyUsername      = errors.New("username is required")
	ErrTOTPRequired       = errors.New("two-factor authentication required")
	ErrTOTPInvalid        = errors.New("invalid authentication code")
	ErrTOTPNotPending     = errors.New("two-factor setup is not pending")
	ErrTOTPAlreadyOn      = errors.New("two-factor authentication is already enabled")
	ErrSecretsUnavailable = errors.New("encrypted secret storage is unavailable")
	ErrUnknownPermission  = errors.New("unknown permission")
	ErrNoPermissions      = errors.New("at least one permission is required")
	ErrLastFullAdmin      = errors.New("cannot revoke full access from the last full-access admin")
	// ErrSuperuserProtected is returned when a protected action is attempted
	// against the bootstrap superuser (delete, disable, narrow permissions,
	// or removing its last local login) — see §3 of the unified-users spec.
	ErrSuperuserProtected = errors.New("the bootstrap superuser is protected")
	// ErrNoLoginMethod is returned when an account would be left with neither a
	// password nor a linked identity, i.e. no way to sign in.
	ErrNoLoginMethod = errors.New("account must keep at least one login method")
	// ErrAccountDisabled is returned when a disabled account attempts to (or is
	// made to) start a session.
	ErrAccountDisabled = errors.New("account is disabled")
)

// AccessDeniedError refuses a session to credentials that are perfectly valid,
// because the account has nothing left to open. A support operator who was
// switched off is exactly that: their password still works, but every section
// they had is closed, so letting them in would leave them staring at a panel
// full of refusals with no idea why. Code travels to the login screen (the
// Telegram callback carries it as ?auth_error=<code>), Reason is the sentence
// the operator reads.
type AccessDeniedError struct {
	Code   string
	Reason string
}

func (e *AccessDeniedError) Error() string { return e.Reason }

// AccessGate is consulted once credentials are verified and BEFORE a session is
// minted. It lives here rather than in the login handlers so every path that
// starts a session — password, Telegram OIDC, anything added later — is covered
// by the same check. The server injects it (SetAccessGate); nil means no gate,
// which is the state during setup and in tests.
type AccessGate func(ctx context.Context, adminID string) error

// superuserErrCode is the SQLSTATE the protect_bootstrap_superuser trigger
// raises (migration 085). It is the database's last-line defence; the service
// layer normally refuses protected actions before they reach it, but if one
// slips through this lets the handler still map it onto ErrSuperuserProtected.
const superuserErrCode = "P0001"

// isSuperuserTriggerViolation reports whether err is the protect trigger firing.
func isSuperuserTriggerViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == superuserErrCode &&
		strings.Contains(pgErr.Message, "bootstrap superuser")
}

// DefaultRole is assigned to accounts created without an explicit role.
// Authorization is enforced by the permission checks for each operation.
const DefaultRole = "admin"

type Session struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Service struct {
	credentialBoundary credentialBoundary
	db                 *db.DB
	logger             *slog.Logger
	secrets            *secretstore.Service
	// accessGate refuses a session to an account that has nothing left to open.
	// Injected by the server, which can see the other modules; nil = no gate.
	accessGate AccessGate
}

// SetSecretStore enables encrypted-at-rest TOTP secrets after the server has
// initialised its shared secret store.
func (s *Service) SetSecretStore(secrets *secretstore.Service) { s.secrets = secrets }

// SetAccessGate installs the check every session-minting path runs after the
// credentials pass. See AccessGate.
func (s *Service) SetAccessGate(gate AccessGate) { s.accessGate = gate }

func (s *Service) checkAccess(ctx context.Context, adminID string) error {
	if !s.credentialBoundary.accepts(time.Now()) {
		return errors.New("credential issuance unavailable during recovery")
	}
	if s.accessGate == nil {
		return nil
	}
	return s.accessGate(ctx, adminID)
}

// RevokeSessions terminates every live session of an account. Called when
// somebody's access is taken away: without it the change only lands whenever
// the victim happens to log out, and until then a switched-off operator keeps
// reading the tickets they were switched off from.
func (s *Service) RevokeSessions(ctx context.Context, adminID string) error {
	if adminID == "" {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE admin_id = $1", adminID); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	return nil
}

func NewService(database *db.DB, logger *slog.Logger) *Service {
	return &Service{db: database, logger: logger}
}

func (s *Service) Login(ctx context.Context, username, password, totpCode string) (*Session, error) {
	var admin Admin
	// password_hash is NULL for Telegram-only users, so scan into a NullString.
	var passwordHash sql.NullString
	var status string

	err := s.db.QueryRowContext(ctx,
		"SELECT id, username, role, status, password_hash FROM admins WHERE username = $1",
		username,
	).Scan(&admin.ID, &admin.Username, &admin.Role, &status, &passwordHash)
	if err != nil || !passwordHash.Valid {
		// Either the user was not found, or the account has no local password
		// (Telegram-only). Still run a bcrypt comparison against a fixed dummy
		// hash so both paths cost roughly the same as the found-with-password
		// path below, preventing username enumeration via response timing. The
		// result is discarded; the error is identical to a bad password.
		_ = bcrypt.CompareHashAndPassword(dummyBcryptHash(), []byte(password))
		return nil, errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash.String), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	// A disabled account may not sign in even with valid credentials.
	if status == "disabled" {
		return nil, errors.New("invalid credentials")
	}

	// A confirmed TOTP setup turns login into a two-step form. The password is
	// checked first, then the same credentials are resubmitted with the code.
	var secretID string
	var lastCounter int64
	err = s.db.QueryRowContext(ctx, `
		SELECT secret_id, last_used_counter
		FROM admin_totp
		WHERE admin_id = $1 AND confirmed_at IS NOT NULL
	`, admin.ID).Scan(&secretID, &lastCounter)
	if err == nil {
		if strings.TrimSpace(totpCode) == "" {
			return nil, ErrTOTPRequired
		}
		if s.secrets == nil {
			return nil, ErrSecretsUnavailable
		}
		secret, _, getErr := s.secrets.Get(ctx, secretID)
		if getErr != nil {
			return nil, fmt.Errorf("read totp secret: %w", getErr)
		}
		counter, ok := validateTOTPCounter(string(secret), totpCode, time.Now())
		if !ok || counter <= lastCounter || !s.credentialBoundary.accepts(time.Unix(counter*30, 0)) {
			return nil, ErrTOTPInvalid
		}
		res, updateErr := s.db.ExecContext(ctx, `
			UPDATE admin_totp SET last_used_counter = $1, updated_at = NOW()
			WHERE admin_id = $2 AND last_used_counter < $1
		`, counter, admin.ID)
		if updateErr != nil {
			return nil, fmt.Errorf("record totp counter: %w", updateErr)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return nil, ErrTOTPInvalid
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("read totp login state: %w", err)
	}

	// The password is right; the question left is whether this account still has
	// anywhere to go. Checked before the session exists, so a refusal leaves
	// nothing to clean up.
	if err := s.checkAccess(ctx, admin.ID); err != nil {
		return nil, err
	}

	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	tokenHash := hashToken(token)
	expiresAt := time.Now().Add(24 * time.Hour)

	sessionID, _ := generateID()
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO sessions (id, admin_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)",
		sessionID, admin.ID, tokenHash, expiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	s.db.ExecContext(ctx, "UPDATE admins SET last_login_at = $1 WHERE id = $2", time.Now(), admin.ID)

	return &Session{Token: token, ExpiresAt: expiresAt}, nil
}

type TOTPStatus struct {
	Enabled bool `json:"enabled"`
	Pending bool `json:"pending"`
}

type TOTPSetup struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}

func (s *Service) TOTPStatus(ctx context.Context, adminID string) (TOTPStatus, error) {
	var confirmed bool
	err := s.db.QueryRowContext(ctx, `
		SELECT confirmed_at IS NOT NULL FROM admin_totp WHERE admin_id = $1
	`, adminID).Scan(&confirmed)
	if errors.Is(err, sql.ErrNoRows) {
		return TOTPStatus{}, nil
	}
	if err != nil {
		return TOTPStatus{}, fmt.Errorf("read totp status: %w", err)
	}
	return TOTPStatus{Enabled: confirmed, Pending: !confirmed}, nil
}

func (s *Service) BeginTOTPSetup(ctx context.Context, adminID, username string) (*TOTPSetup, error) {
	if s.secrets == nil {
		return nil, ErrSecretsUnavailable
	}
	status, err := s.TOTPStatus(ctx, adminID)
	if err != nil {
		return nil, err
	}
	if status.Enabled {
		return nil, ErrTOTPAlreadyOn
	}
	secret, err := GenerateTOTPSecret()
	if err != nil {
		return nil, fmt.Errorf("generate totp secret: %w", err)
	}
	secretID := "totp.admin." + adminID
	if err := s.secrets.Put(ctx, secretID, "admin_totp", []byte(secret), map[string]any{"admin_id": adminID}); err != nil {
		return nil, fmt.Errorf("store totp secret: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO admin_totp (admin_id, secret_id, confirmed_at, last_used_counter, created_at, updated_at)
		VALUES ($1, $2, NULL, 0, NOW(), NOW())
		ON CONFLICT (admin_id) DO UPDATE SET
			secret_id = EXCLUDED.secret_id, confirmed_at = NULL,
			last_used_counter = 0, updated_at = NOW()
	`, adminID, secretID); err != nil {
		_ = s.secrets.Delete(ctx, secretID)
		return nil, fmt.Errorf("start totp setup: %w", err)
	}
	return &TOTPSetup{Secret: secret, URI: TOTPProvisioningURI(secret, username, "Axottle")}, nil
}

func (s *Service) ConfirmTOTP(ctx context.Context, adminID, code string) error {
	if s.secrets == nil {
		return ErrSecretsUnavailable
	}
	var secretID string
	var confirmed bool
	err := s.db.QueryRowContext(ctx, `
		SELECT secret_id, confirmed_at IS NOT NULL FROM admin_totp WHERE admin_id = $1
	`, adminID).Scan(&secretID, &confirmed)
	if err != nil || confirmed {
		return ErrTOTPNotPending
	}
	secret, _, err := s.secrets.Get(ctx, secretID)
	if err != nil {
		return fmt.Errorf("read totp secret: %w", err)
	}
	counter, ok := validateTOTPCounter(string(secret), code, time.Now())
	if !ok || !s.credentialBoundary.accepts(time.Unix(counter*30, 0)) {
		return ErrTOTPInvalid
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE admin_totp
		SET confirmed_at = NOW(), last_used_counter = $1, updated_at = NOW()
		WHERE admin_id = $2 AND confirmed_at IS NULL
	`, counter, adminID)
	if err != nil {
		return fmt.Errorf("confirm totp: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrTOTPNotPending
	}
	return nil
}

func (s *Service) DisableTOTP(ctx context.Context, adminID, password, code string) error {
	var passwordHash, secretID string
	var lastCounter int64
	err := s.db.QueryRowContext(ctx, `
		SELECT a.password_hash, t.secret_id, t.last_used_counter
		FROM admins a JOIN admin_totp t ON t.admin_id = a.id
		WHERE a.id = $1 AND t.confirmed_at IS NOT NULL
	`, adminID).Scan(&passwordHash, &secretID, &lastCounter)
	if err != nil {
		return ErrAccountNotFound
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		return ErrWrongPassword
	}
	if s.secrets == nil {
		return ErrSecretsUnavailable
	}
	secret, _, err := s.secrets.Get(ctx, secretID)
	if err != nil {
		return fmt.Errorf("read totp secret: %w", err)
	}
	counter, ok := validateTOTPCounter(string(secret), code, time.Now())
	if !ok || counter <= lastCounter || !s.credentialBoundary.accepts(time.Unix(counter*30, 0)) {
		return ErrTOTPInvalid
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM admin_totp WHERE admin_id = $1", adminID); err != nil {
		return fmt.Errorf("disable totp: %w", err)
	}
	return nil
}

func (s *Service) ValidateToken(ctx context.Context, token string) (*Admin, error) {
	tokenHash := hashToken(token)

	var admin Admin
	var permsRaw []byte
	var hasPassword bool
	var issuedAt time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT a.id, a.username, a.display_name, a.role, a.status,
		       a.is_bootstrap_superuser, a.permissions, a.password_hash IS NOT NULL, s.created_at
		FROM sessions s JOIN admins a ON s.admin_id = a.id
		WHERE s.token_hash = $1 AND s.expires_at > $2
	`, tokenHash, time.Now()).Scan(&admin.ID, &admin.Username, &admin.DisplayName, &admin.Role,
		&admin.Status, &admin.IsBootstrapSuperuser, &permsRaw, &hasPassword, &issuedAt)
	if err != nil || !s.credentialBoundary.accepts(issuedAt) {
		return nil, errors.New("invalid or expired token")
	}
	admin.HasPassword = hasPassword
	// Permissions ride along on every request, so a change takes effect on
	// the target's very next request — no session invalidation needed.
	admin.Permissions, err = decodePermissions(permsRaw)
	if err != nil {
		return nil, fmt.Errorf("decode permissions: %w", err)
	}

	return &admin, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	tokenHash := hashToken(token)
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = $1", tokenHash)
	return err
}

// CreateAccount provisions a new admin account. username and password must
// already be non-empty and policy-valid (callers use ValidatePassword); this
// method still normalises the username and maps a unique-violation onto
// ErrUsernameTaken so the handler can return 409 rather than 500. permissions
// is validated via NormalizePermissions (nil means full access). The created
// Admin is returned so the caller can echo it back without a second query.
func (s *Service) CreateAccount(ctx context.Context, username, password, role string, permissions []string, projectIDs ...[]string) (*Admin, error) {
	var ids []string
	if len(projectIDs) > 0 {
		ids = projectIDs[0]
	}
	return s.CreateUser(ctx, CreateUserInput{Username: username, Password: password, Permissions: permissions, ProjectIDs: ids})
}

// CountAccounts returns the number of admin accounts.
func (s *Service) CountAccounts(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admins").Scan(&n); err != nil {
		return 0, fmt.Errorf("count accounts: %w", err)
	}
	return n, nil
}

// ListAccounts returns every admin account, newest first, without password
// material. The bootstrap superuser is ordered first (it is the anchor account)
// and every other account by creation time, newest first.
func (s *Service) ListAccounts(ctx context.Context) ([]Admin, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, display_name, role, status, is_bootstrap_superuser,
		       permissions, password_hash IS NOT NULL, created_at, last_login_at
		FROM admins
		ORDER BY is_bootstrap_superuser DESC, created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var out []Admin
	for rows.Next() {
		var a Admin
		var permsRaw []byte
		if err := rows.Scan(&a.ID, &a.Username, &a.DisplayName, &a.Role, &a.Status,
			&a.IsBootstrapSuperuser, &permsRaw, &a.HasPassword, &a.CreatedAt, &a.LastLoginAt); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		if a.Permissions, err = decodePermissions(permsRaw); err != nil {
			return nil, fmt.Errorf("decode permissions: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAccount reads one account by id, without password material.
func (s *Service) GetAccount(ctx context.Context, id string) (*Admin, error) {
	var a Admin
	var permsRaw []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, display_name, role, status, is_bootstrap_superuser,
		       permissions, password_hash IS NOT NULL, created_at, last_login_at
		FROM admins WHERE id = $1`, id).Scan(&a.ID, &a.Username, &a.DisplayName, &a.Role,
		&a.Status, &a.IsBootstrapSuperuser, &permsRaw, &a.HasPassword, &a.CreatedAt, &a.LastLoginAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}
	if a.Permissions, err = decodePermissions(permsRaw); err != nil {
		return nil, fmt.Errorf("decode permissions: %w", err)
	}
	return &a, nil
}

// UpdateAccountPermissions replaces an account's permission list. It refuses
// to leave the panel without a single full-access admin: narrowing the last
// account that still holds the wildcard is rejected with ErrLastFullAdmin.
// The "don't narrow your own account" guard lives in the handler, which knows
// the acting admin. Takes effect on the target's next request — permissions
// are re-read from the DB by ValidateToken every time.
func (s *Service) UpdateAccountPermissions(ctx context.Context, id string, permissions []string) (*Admin, error) {
	perms, err := NormalizePermissions(permissions)
	if err != nil {
		return nil, err
	}

	if perms[0] != PermissionAll {
		// The bootstrap superuser must keep full access (§3).
		var isSuperuser bool
		if err := s.db.QueryRowContext(ctx,
			"SELECT is_bootstrap_superuser FROM admins WHERE id = $1", id).Scan(&isSuperuser); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrAccountNotFound
			}
			return nil, fmt.Errorf("lookup account: %w", err)
		}
		if isSuperuser {
			return nil, ErrSuperuserProtected
		}

		var fullAdmins int
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM admins WHERE permissions @> '["*"]'::jsonb AND id <> $1`, id,
		).Scan(&fullAdmins); err != nil {
			return nil, fmt.Errorf("count full-access admins: %w", err)
		}
		if fullAdmins == 0 {
			return nil, ErrLastFullAdmin
		}
	}

	permsJSON, err := json.Marshal(perms)
	if err != nil {
		return nil, fmt.Errorf("encode permissions: %w", err)
	}

	var admin Admin
	var permsRaw []byte
	err = s.db.QueryRowContext(ctx, `
		UPDATE admins SET permissions = $1 WHERE id = $2
		RETURNING id, username, display_name, role, status, is_bootstrap_superuser,
		          permissions, password_hash IS NOT NULL, created_at, last_login_at
	`, permsJSON, id).Scan(&admin.ID, &admin.Username, &admin.DisplayName, &admin.Role,
		&admin.Status, &admin.IsBootstrapSuperuser, &permsRaw, &admin.HasPassword,
		&admin.CreatedAt, &admin.LastLoginAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		if isSuperuserTriggerViolation(err) {
			return nil, ErrSuperuserProtected
		}
		return nil, fmt.Errorf("update permissions: %w", err)
	}
	if admin.Permissions, err = decodePermissions(permsRaw); err != nil {
		return nil, fmt.Errorf("decode permissions: %w", err)
	}
	return &admin, nil
}

// UpdateAccountProfile updates an account's display name and/or status. Nil
// fields are left unchanged. The bootstrap superuser cannot be disabled (§3):
// setting its status to anything but "active" is refused with
// ErrSuperuserProtected (the DB trigger is the backstop). Returns the updated
// account.
func (s *Service) UpdateAccountProfile(ctx context.Context, id string, displayName, status *string) (*Admin, error) {
	if status != nil {
		switch *status {
		case "invited", "active", "disabled":
		default:
			return nil, fmt.Errorf("invalid status %q", *status)
		}
		if *status != "active" {
			var isSuperuser bool
			if err := s.db.QueryRowContext(ctx,
				"SELECT is_bootstrap_superuser FROM admins WHERE id = $1", id).Scan(&isSuperuser); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, ErrAccountNotFound
				}
				return nil, fmt.Errorf("lookup account: %w", err)
			}
			if isSuperuser {
				return nil, ErrSuperuserProtected
			}
		}
	}

	var admin Admin
	var permsRaw []byte
	err := s.db.QueryRowContext(ctx, `
		UPDATE admins SET
			display_name = COALESCE($2, display_name),
			status       = COALESCE($3, status)
		WHERE id = $1
		RETURNING id, username, display_name, role, status, is_bootstrap_superuser,
		          permissions, password_hash IS NOT NULL, created_at, last_login_at
	`, id, displayName, status).Scan(&admin.ID, &admin.Username, &admin.DisplayName, &admin.Role,
		&admin.Status, &admin.IsBootstrapSuperuser, &permsRaw, &admin.HasPassword,
		&admin.CreatedAt, &admin.LastLoginAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		if isSuperuserTriggerViolation(err) {
			return nil, ErrSuperuserProtected
		}
		return nil, fmt.Errorf("update account profile: %w", err)
	}
	// If the account was disabled, terminate its sessions immediately.
	if status != nil && *status == "disabled" {
		if _, derr := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE admin_id = $1", id); derr != nil {
			s.logger.Error("disable account: revoke sessions", "error", derr, "admin_id", id)
		}
	}
	if admin.Permissions, err = decodePermissions(permsRaw); err != nil {
		return nil, fmt.Errorf("decode permissions: %w", err)
	}
	return &admin, nil
}

// decodePermissions unmarshals the admins.permissions JSONB column. A missing
// value (pre-migration row read inside the migration window) degrades to full
// access, matching the column default.
func decodePermissions(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return []string{PermissionAll}, nil
	}
	var perms []string
	if err := json.Unmarshal(raw, &perms); err != nil {
		return nil, err
	}
	if len(perms) == 0 {
		return []string{PermissionAll}, nil
	}
	return perms, nil
}

// DeleteAccount removes an admin by id. It refuses to delete the final
// remaining admin so an operator can never lock everyone out of the panel;
// the caller is responsible for the "don't delete yourself" guard, which needs
// the acting admin's id. Deleting cascades to that admin's sessions (FK ON
// DELETE CASCADE), logging them out immediately.
func (s *Service) DeleteAccount(ctx context.Context, id string) error {
	// The bootstrap superuser can never be deleted (§3). Checked here so the
	// caller gets ErrSuperuserProtected rather than the raw trigger violation.
	var isSuperuser bool
	err := s.db.QueryRowContext(ctx,
		"SELECT is_bootstrap_superuser FROM admins WHERE id = $1", id).Scan(&isSuperuser)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAccountNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup account: %w", err)
	}
	if isSuperuser {
		return ErrSuperuserProtected
	}

	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admins").Scan(&count); err != nil {
		return fmt.Errorf("count admins: %w", err)
	}
	if count <= 1 {
		return ErrLastAdmin
	}

	res, err := s.db.ExecContext(ctx, "DELETE FROM admins WHERE id = $1", id)
	if err != nil {
		if isSuperuserTriggerViolation(err) {
			return ErrSuperuserProtected
		}
		return fmt.Errorf("delete account: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrAccountNotFound
	}
	return nil
}

// ChangePassword verifies the account's current password, then rotates it to a
// new (already policy-valid) password. On success every OTHER session for the
// account is revoked — a password change should log out other devices — while
// keepToken (the caller's current session) is preserved so the operator who
// initiated the change stays logged in. Pass keepToken == "" to revoke all
// sessions.
func (s *Service) ChangePassword(ctx context.Context, adminID, oldPassword, newPassword, keepToken string) error {
	var passwordHash string
	err := s.db.QueryRowContext(ctx,
		"SELECT password_hash FROM admins WHERE id = $1", adminID,
	).Scan(&passwordHash)
	if err != nil {
		return ErrAccountNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(oldPassword)); err != nil {
		return ErrWrongPassword
	}
	// Reject a no-op change so "rotate the credential" actually rotates it.
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(newPassword)) == nil {
		return ErrPasswordUnchanged
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if _, err := s.db.ExecContext(ctx,
		"UPDATE admins SET password_hash = $1 WHERE id = $2", string(newHash), adminID,
	); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	// Invalidate other sessions. A failure here is logged but not fatal: the
	// password is already changed, and stale sessions still expire on their
	// own 24h TTL.
	if _, err := s.db.ExecContext(ctx,
		"DELETE FROM sessions WHERE admin_id = $1 AND token_hash <> $2",
		adminID, hashToken(keepToken),
	); err != nil {
		s.logger.Error("change password: revoke other sessions", "error", err, "admin_id", adminID)
	}

	return nil
}

// Reauthenticate verifies a user's current password and, when TOTP is enabled,
// a valid TOTP code — a fresh re-auth challenge for a sensitive action (§7:
// linking/unlinking the superuser's Telegram). Returns ErrWrongPassword or
// ErrTOTPInvalid/ErrTOTPRequired on failure. A password-less account cannot be
// reauthenticated by password and is rejected with ErrWrongPassword.
func (s *Service) Reauthenticate(ctx context.Context, adminID, password, totpCode string) error {
	var passwordHash sql.NullString
	if err := s.db.QueryRowContext(ctx,
		"SELECT password_hash FROM admins WHERE id = $1", adminID).Scan(&passwordHash); err != nil {
		return ErrAccountNotFound
	}
	if !passwordHash.Valid {
		return ErrWrongPassword
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash.String), []byte(password)) != nil {
		return ErrWrongPassword
	}
	// If TOTP is confirmed, require a valid code too.
	var secretID string
	var lastCounter int64
	err := s.db.QueryRowContext(ctx, `
		SELECT secret_id, last_used_counter FROM admin_totp
		WHERE admin_id = $1 AND confirmed_at IS NOT NULL`, adminID).Scan(&secretID, &lastCounter)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read totp state: %w", err)
	}
	if strings.TrimSpace(totpCode) == "" {
		return ErrTOTPRequired
	}
	if s.secrets == nil {
		return ErrSecretsUnavailable
	}
	secret, _, gerr := s.secrets.Get(ctx, secretID)
	if gerr != nil {
		return fmt.Errorf("read totp secret: %w", gerr)
	}
	counter, ok := validateTOTPCounter(string(secret), totpCode, time.Now())
	if !ok || counter <= lastCounter || !s.credentialBoundary.accepts(time.Unix(counter*30, 0)) {
		return ErrTOTPInvalid
	}
	return nil
}

// Context helpers

type contextKey string

const adminContextKey contextKey = "admin"

func ContextWithAdmin(ctx context.Context, admin *Admin) context.Context {
	return context.WithValue(ctx, adminContextKey, admin)
}

func AdminFromContext(ctx context.Context) *Admin {
	admin, _ := ctx.Value(adminContextKey).(*Admin)
	return admin
}

func TokenFromRequest(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// Helpers

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505), e.g. inserting a duplicate admins.username.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

var (
	dummyHashOnce sync.Once
	dummyHash     []byte
)

// dummyBcryptHash returns a fixed bcrypt hash, computed once from a random
// password, used to equalise the timing of the user-not-found login path with
// the user-found path. Because it is derived from random bytes no real
// password can ever match it, so comparing against it always fails while still
// costing a full bcrypt evaluation.
func dummyBcryptHash() []byte {
	dummyHashOnce.Do(func() {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			// Unreachable in practice; fall back to a fixed valid bcrypt hash
			// so the compare still exercises the bcrypt path (bcrypt of the
			// string "unused", which no login can produce a match for).
			dummyHash = []byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")
			return
		}
		h, err := bcrypt.GenerateFromPassword(buf, bcrypt.DefaultCost)
		if err != nil {
			dummyHash = []byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")
			return
		}
		dummyHash = h
	})
	return dummyHash
}
