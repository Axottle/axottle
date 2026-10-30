package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"time"
)

// CreateUserInput is the single request shape for provisioning a user, used by
// both the settings "add user" flow and the support "add operator" flow (§5).
type CreateUserInput struct {
	DisplayName    string
	ProjectIDs     []string
	GlobalProjects bool
	// Username is supplied by the caller, including external login adapters.
	Username string
	// Password may be omitted only when a provisioner adds a login identity.
	Password    string
	Permissions []string
	// Status defaults to "active" when empty.
	Status string
}

// ProvisionUser attaches optional module state to a new account in the same
// transaction. An error rolls back the account, login identities and grants.
// The module owns its schema and authorization; the core never imports it.
type ProvisionUser func(context.Context, *sql.Tx, *Admin) error

// CreateUser provisions the account, optional login identity and project grants.
// At least one login method must exist before commit. A provisioner may add a
// linked identity; its provider implementation remains outside the core.
func (s *Service) CreateUser(ctx context.Context, in CreateUserInput, provisioners ...ProvisionUser) (*Admin, error) {
	hasPassword := strings.TrimSpace(in.Password) != ""
	hasProvisioner := false
	for _, p := range provisioners {
		if p != nil {
			hasProvisioner = true
		}
	}
	if !hasPassword && !hasProvisioner {
		return nil, ErrNoLoginMethod
	}

	username := strings.TrimSpace(in.Username)
	if username == "" {
		return nil, ErrEmptyUsername
	}

	displayName := strings.TrimSpace(in.DisplayName)
	if displayName == "" {
		displayName = username
	}

	status := in.Status
	switch status {
	case "":
		status = "active"
	case "invited", "active", "disabled":
	default:
		return nil, fmt.Errorf("invalid status %q", status)
	}

	perms, err := NormalizePermissions(in.Permissions)
	if err != nil {
		return nil, err
	}
	permsJSON, err := marshalPermissions(perms)
	if err != nil {
		return nil, err
	}

	if len(in.ProjectIDs) == 0 {
		return nil, ErrProjectsRequired
	}

	var passwordHash sql.NullString
	if hasPassword {
		hash, herr := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if herr != nil {
			return nil, fmt.Errorf("hash password: %w", herr)
		}
		passwordHash = sql.NullString{String: string(hash), Valid: true}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	id, _ := generateID()
	createdAt := time.Now()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO admins (id, username, display_name, password_hash, role, permissions, status, created_at, global_projects)
		 VALUES ($1, $2, $3, $4, 'admin', $5, $6, $7, $8)`,
		id, username, displayName, passwordHash, permsJSON, status, createdAt, in.GlobalProjects,
	); err != nil {
		return nil, mapCreateUserErr(err)
	}

	if err := insertProjectGrants(ctx, tx, id, in.ProjectIDs); err != nil {
		return nil, err
	}

	created := &Admin{
		ID: id, Username: username, DisplayName: displayName, Role: DefaultRole,
		Status: status, Permissions: perms, HasPassword: hasPassword, CreatedAt: createdAt,
	}
	for _, provision := range provisioners {
		if provision != nil {
			if err := provision(ctx, tx, created); err != nil {
				return nil, err
			}
		}
	}

	if !hasPassword {
		var identity bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM auth_identities WHERE admin_id=$1)`, id).Scan(&identity); err != nil {
			return nil, err
		}
		if !identity {
			return nil, ErrNoLoginMethod
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, mapCreateUserErr(err)
	}

	return created, nil
}

// mapCreateUserErr turns a Postgres constraint violation from CreateUser into
// the right sentinel so the handler can pick a 409 and an explanatory message.
func mapCreateUserErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "username"):
			return ErrUsernameTaken
		}
	}
	return fmt.Errorf("create user: %w", err)
}

// marshalPermissions is the JSON encoding used for the admins.permissions
// column, factored out so CreateUser and CreateAccount agree on the shape.
func marshalPermissions(perms []string) ([]byte, error) {
	b, err := json.Marshal(perms)
	if err != nil {
		return nil, fmt.Errorf("encode permissions: %w", err)
	}
	return b, nil
}

// CountLoginableAccounts counts active users with a password or linked identity.
func (s *Service) CountLoginableAccounts(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM admins a
		WHERE a.status <> 'disabled'
		  AND (a.password_hash IS NOT NULL
		       OR EXISTS (SELECT 1 FROM auth_identities i WHERE i.admin_id = a.id))
	`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count loginable accounts: %w", err)
	}
	return n, nil
}

// TOTPEnabledByAdmin returns the set of admin ids with confirmed TOTP, for the
// accounts list's auth_methods.totp flag.
func (s *Service) TOTPEnabledByAdmin(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT admin_id FROM admin_totp WHERE confirmed_at IS NOT NULL")
	if err != nil {
		return nil, fmt.Errorf("list totp: %w", err)
	}
	defer rows.Close()
	out := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan totp: %w", err)
		}
		out[id] = true
	}
	return out, rows.Err()
}

// IssueSession mints a session after authentication and applies the same account
// status and access checks as a local password login.
func (s *Service) IssueSession(ctx context.Context, adminID string) (*Session, error) {
	var status string
	if err := s.db.QueryRowContext(ctx,
		"SELECT status FROM admins WHERE id = $1", adminID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("lookup account: %w", err)
	}
	if status == "disabled" {
		return nil, ErrAccountDisabled
	}
	if err := s.checkAccess(ctx, adminID); err != nil {
		return nil, err
	}
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	expiresAt := time.Now().Add(24 * time.Hour)
	sessionID, _ := generateID()
	if _, err := s.db.ExecContext(ctx,
		"INSERT INTO sessions (id, admin_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)",
		sessionID, adminID, hashToken(token), expiresAt); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	s.db.ExecContext(ctx, "UPDATE admins SET last_login_at = $1 WHERE id = $2", time.Now(), adminID)
	return &Session{Token: token, ExpiresAt: expiresAt}, nil
}

// GetTelegramIdentityBySubject resolves a linked Telegram identity by its OIDC
// subject. ErrIdentityNotFound when none matches.
