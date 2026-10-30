// Package coreapi is the standalone account, project and settings API.
package coreapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/axottle/axottle/internal/audit"
	"github.com/axottle/axottle/internal/auth"
	"github.com/axottle/axottle/internal/authhttp"
	"github.com/axottle/axottle/internal/coredata"
	"github.com/axottle/axottle/internal/db"
	"github.com/axottle/axottle/internal/projects"
	"github.com/axottle/axottle/internal/secretstore"
	"github.com/axottle/axottle/internal/setup"
)

type Config struct {
	DatabaseURL, SecretKey, PublicURL string
	DataDir                           string
	LegacySecretKeys                  []string
	Logger                            *slog.Logger
	UI                                http.Handler
}

type Server struct {
	db                 *db.DB
	auth               *auth.Service
	setup              *setup.Service
	projects           *projects.Service
	audit              *audit.Service
	sessions           *authhttp.Handler
	mux                *http.ServeMux
	credentialBoundary *auth.CredentialBoundaryFile
}

func Open(ctx context.Context, cfg Config) (*Server, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	database, err := db.Open(cfg.DatabaseURL, logger)
	if err != nil {
		return nil, err
	}
	if err := coredata.Migrate(ctx, database.DB); err != nil {
		database.Close()
		return nil, err
	}
	secrets, err := secretstore.New(database.DB, cfg.SecretKey, cfg.LegacySecretKeys...)
	if err != nil {
		database.Close()
		return nil, err
	}
	s := &Server{db: database, auth: auth.NewService(database, logger), setup: setup.NewService(database, logger),
		projects: projects.NewService(database.DB), audit: audit.NewService(database, logger)}
	s.auth.SetSecretStore(secrets)
	boundary := auth.NewCredentialBoundaryFile(cfg.DataDir)
	s.auth.SetCredentialBoundary(boundary.Epoch, boundary.Blocked)
	s.credentialBoundary = boundary
	s.sessions = &authhttp.Handler{Auth: s.auth, Setup: s.setup, PublicURL: cfg.PublicURL,
		LoginGuard: auth.NewLoginGuard(5, 5*time.Minute, 15*time.Minute)}
	s.routes()
	if cfg.UI != nil {
		s.mux.Handle("/", cfg.UI)
	}
	return s, nil
}

func (s *Server) Close() error                    { return s.db.Close() }
func (s *Server) Ready(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.credentialBoundary != nil && s.credentialBoundary.Blocked() {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Retry-After", "2")
		http.Error(w, "Database recovery pending; reconnect after security reconciliation", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("GET /api/v1/setup/status", s.setupStatus)
	s.mux.HandleFunc("GET /api/v1/core/presentation", s.presentation)
	s.mux.HandleFunc("POST /api/v1/setup/admin", s.setupAdmin)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.sessions.Login)
	private := http.NewServeMux()
	private.HandleFunc("GET /api/v1/auth/me", s.sessions.Me)
	private.HandleFunc("GET /api/v1/auth/permissions", s.permissions)
	private.HandleFunc("GET /api/v1/auth/accounts", s.guard(auth.PermAccountsManage, s.listAccounts))
	private.HandleFunc("POST /api/v1/auth/accounts", s.guard(auth.PermAccountsManage, s.createAccount))
	private.HandleFunc("PATCH /api/v1/auth/accounts/{id}", s.guard(auth.PermAccountsManage, s.updateAccount))
	private.HandleFunc("DELETE /api/v1/auth/accounts/{id}", s.guard(auth.PermAccountsManage, s.deleteAccount))
	private.HandleFunc("POST /api/v1/auth/logout", s.sessions.Logout)
	private.HandleFunc("POST /api/v1/auth/change-password", s.changePassword)
	private.HandleFunc("GET /api/v1/auth/totp", s.totpStatus)
	private.HandleFunc("GET /api/v1/auth/totp/status", s.totpStatus)
	private.HandleFunc("POST /api/v1/auth/totp/setup", s.totpSetup)
	private.HandleFunc("POST /api/v1/auth/totp/confirm", s.totpConfirm)
	private.HandleFunc("DELETE /api/v1/auth/totp", s.totpDisable)
	private.HandleFunc("POST /api/v1/auth/totp/disable", s.totpDisable)
	private.HandleFunc("GET /api/v1/projects", s.listProjects)
	private.HandleFunc("POST /api/v1/projects", s.guard(auth.PermSettingsManage, s.createProject))
	private.HandleFunc("GET /api/v1/projects/{id}", s.getProject)
	private.HandleFunc("PATCH /api/v1/projects/{id}", s.guard(auth.PermSettingsManage, s.updateProject))
	private.HandleFunc("GET /api/v1/settings/general", s.guard(auth.PermSettingsView, s.getSettings))
	private.HandleFunc("PATCH /api/v1/settings/general", s.guard(auth.PermSettingsManage, s.updateSettings))
	private.HandleFunc("GET /api/v1/audit", s.guard(auth.PermAuditView, s.listAudit))
	private.HandleFunc("GET /api/v1/audit-log", s.guard(auth.PermAuditView, s.listAudit))
	s.mux.Handle("/api/", s.sessions.Middleware(private))
}

func (s *Server) guard(permission auth.Permission, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !auth.AdminFromContext(r.Context()).Can(permission) {
			failure(w, 403, "permission required")
			return
		}
		next(w, r)
	}
}

func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		failure(w, 400, "invalid request body")
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		failure(w, 400, "invalid request body")
		return false
	}
	return true
}
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func failure(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}
func (s *Server) log(r *http.Request, action, kind, id string) {
	a := auth.AdminFromContext(r.Context())
	s.audit.Log(r.Context(), audit.Entry{ActorType: "admin", ActorID: a.Username, Action: action, ResourceType: kind, ResourceID: id})
}

func (s *Server) setupStatus(w http.ResponseWriter, r *http.Request) {
	state, err := s.setup.GetState(r.Context())
	if err != nil {
		failure(w, 500, "setup state unavailable")
		return
	}
	respond(w, 200, state)
}
func (s *Server) setupAdmin(w http.ResponseWriter, r *http.Request) {
	if s.setup.IsSealed(r.Context()) {
		failure(w, 403, "setup is sealed")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		failure(w, 400, "username is required")
		return
	}
	if err := auth.ValidatePassword(req.Password); err != nil {
		failure(w, 400, err.Error())
		return
	}
	if err := s.setup.CreateAdmin(r.Context(), req.Username, req.Password); err != nil {
		failure(w, 409, "bootstrap administrator already exists or could not be created")
		return
	}
	respond(w, 201, map[string]string{"status": "admin created"})
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	list, err := s.projects.List(r.Context())
	if err != nil {
		failure(w, 500, "projects unavailable")
		return
	}
	respond(w, 200, map[string]any{"projects": list})
}
func projectError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, projects.ErrNotFound):
		failure(w, 404, "project not found")
	case errors.Is(err, projects.ErrInvalid):
		failure(w, 400, err.Error())
	default:
		failure(w, 500, "project request failed")
	}
}
func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	p, err := s.projects.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		projectError(w, err)
		return
	}
	respond(w, 200, p)
}
func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	var req projects.UpdateRequest
	if !decode(w, r, &req) {
		return
	}
	p, err := s.projects.Update(r.Context(), r.PathValue("id"), req)
	if err != nil {
		projectError(w, err)
		return
	}
	s.log(r, "project.updated", "project", p.ID)
	respond(w, 200, p)
}
func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	// New projects have no existing members. Only the bootstrap superuser can
	// create this global resource until project-administration grants are split.
	if !auth.AdminFromContext(r.Context()).IsBootstrapSuperuser {
		failure(w, 403, "bootstrap administrator required")
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !decode(w, r, &req) {
		return
	}
	p, err := s.projects.Create(r.Context(), req.Name, req.Description)
	if err != nil {
		projectError(w, err)
		return
	}
	s.log(r, "project.created", "project", p.ID)
	respond(w, 201, p)
}

// General preferences are a positive allowlist: this endpoint cannot return or
// overwrite credentials, integration configuration, master keys or module state.
var preferenceKeys = map[string]int{"panel.name": 120, "panel.language": 16, "panel.timezone": 80}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT key,value_json FROM system_settings WHERE key IN ('panel.name','panel.language','panel.timezone') ORDER BY key`)
	if err != nil {
		failure(w, 500, "settings unavailable")
		return
	}
	defer rows.Close()
	values := map[string]json.RawMessage{}
	for rows.Next() {
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			failure(w, 500, "settings unavailable")
			return
		}
		values[key] = value
	}
	if rows.Err() != nil {
		failure(w, 500, "settings unavailable")
		return
	}
	respond(w, 200, map[string]any{"settings": values})
}
func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Settings map[string]string `json:"settings"`
	}
	if !decode(w, r, &req) {
		return
	}
	if len(req.Settings) == 0 {
		failure(w, 400, "settings are required")
		return
	}
	for key, value := range req.Settings {
		limit, ok := preferenceKeys[key]
		if !ok || len(value) > limit {
			failure(w, 400, "unknown or invalid preference")
			return
		}
		if key == "panel.language" && value != "ru" && value != "en" {
			failure(w, 400, "unsupported panel language")
			return
		}
		if key == "panel.timezone" {
			if _, err := time.LoadLocation(value); err != nil {
				failure(w, 400, "unknown timezone")
				return
			}
		}
		if key == "panel.name" && strings.TrimSpace(value) == "" {
			failure(w, 400, "panel name is required")
			return
		}
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		failure(w, 500, "settings update failed")
		return
	}
	defer tx.Rollback()
	for key, value := range req.Settings {
		raw, _ := json.Marshal(value)
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO system_settings(key,value_json) VALUES($1,$2::jsonb) ON CONFLICT(key) DO UPDATE SET value_json=EXCLUDED.value_json,updated_at=NOW()`, key, string(raw)); err != nil {
			failure(w, 500, "settings update failed")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		failure(w, 500, "settings update failed")
		return
	}
	s.log(r, "settings.updated", "settings", "general")
	s.getSettings(w, r)
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	a := auth.AdminFromContext(r.Context())
	rows, err := s.db.QueryContext(r.Context(), `SELECT id,actor_type,actor_id,action,COALESCE(resource_type,''),COALESCE(resource_id,''),created_at
	 FROM audit_log WHERE $1 OR EXISTS(SELECT 1 FROM admins WHERE id=$2 AND global_projects)
	 OR metadata_json->>'project_id' IN (SELECT project_id FROM admin_projects WHERE admin_id=$2)
	 ORDER BY id DESC LIMIT 100`, a.IsBootstrapSuperuser, a.ID)
	if err != nil {
		failure(w, 500, "audit unavailable")
		return
	}
	defer rows.Close()
	entries := []map[string]any{}
	for rows.Next() {
		var id int64
		var actorType, actorID, action, kind, resource string
		var created time.Time
		if err := rows.Scan(&id, &actorType, &actorID, &action, &kind, &resource, &created); err != nil {
			failure(w, 500, "audit unavailable")
			return
		}
		entries = append(entries, map[string]any{"id": id, "actor_type": actorType, "actor_id": actorID, "action": action, "resource_type": kind, "resource_id": resource, "created_at": created})
	}
	if rows.Err() != nil {
		failure(w, 500, "audit unavailable")
		return
	}
	respond(w, 200, map[string]any{"entries": entries})
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Old string `json:"old_password"`
		New string `json:"new_password"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := auth.ValidatePassword(req.New); err != nil {
		failure(w, 400, err.Error())
		return
	}
	token, _ := s.sessions.RequestToken(r)
	a := auth.AdminFromContext(r.Context())
	if err := s.auth.ChangePassword(r.Context(), a.ID, req.Old, req.New, token); err != nil {
		failure(w, 400, "password change refused")
		return
	}
	s.log(r, "auth.password.changed", "admin", a.ID)
	respond(w, 200, map[string]string{"status": "password changed"})
}
func (s *Server) totpStatus(w http.ResponseWriter, r *http.Request) {
	value, err := s.auth.TOTPStatus(r.Context(), auth.AdminFromContext(r.Context()).ID)
	if err != nil {
		failure(w, 500, "two-factor state unavailable")
		return
	}
	respond(w, 200, value)
}
func (s *Server) totpSetup(w http.ResponseWriter, r *http.Request) {
	a := auth.AdminFromContext(r.Context())
	value, err := s.auth.BeginTOTPSetup(r.Context(), a.ID, a.Username)
	if err != nil {
		failure(w, 400, "two-factor setup refused")
		return
	}
	respond(w, 200, value)
}
func (s *Server) totpConfirm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := s.auth.ConfirmTOTP(r.Context(), auth.AdminFromContext(r.Context()).ID, req.Code); err != nil {
		failure(w, 400, "invalid authentication code")
		return
	}
	respond(w, 200, map[string]string{"status": "enabled"})
}
func (s *Server) totpDisable(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := s.auth.DisableTOTP(r.Context(), auth.AdminFromContext(r.Context()).ID, req.Password, req.Code); err != nil {
		failure(w, 400, "two-factor disable refused")
		return
	}
	respond(w, 200, map[string]string{"status": "disabled"})
}
