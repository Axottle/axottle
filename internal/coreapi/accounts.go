package coreapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/axottle/axottle/internal/audit"
	"github.com/axottle/axottle/internal/auth"
	"github.com/axottle/axottle/internal/authhttp"
	"github.com/axottle/axottle/internal/projects"
)

// A project-scoped account manager must select one of their own projects.
// Wildcard permissions never substitute for current project membership.
func (s *Server) accountWorkspace(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
	a := auth.AdminFromContext(r.Context())
	id := strings.TrimSpace(r.Header.Get("X-Axottle-Project"))
	if id == "" && a.IsBootstrapSuperuser {
		return r, true
	}
	if id == "" {
		list, err := s.projects.List(r.Context())
		if err != nil {
			failure(w, 500, "projects unavailable")
			return r, false
		}
		if len(list) != 1 {
			failure(w, 409, "select an accessible project")
			return r, false
		}
		id = list[0].ID
	}
	if _, err := s.projects.Get(r.Context(), id); err != nil {
		projectError(w, err)
		return r, false
	}
	return r.WithContext(projects.WithWorkspace(r.Context(), id, "")), true
}

func accountError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrAccountNotFound), errors.Is(err, projects.ErrNotFound):
		failure(w, 404, "account not found")
	case errors.Is(err, auth.ErrUsernameTaken):
		failure(w, 409, "username already taken")
	case errors.Is(err, auth.ErrSuperuserProtected), errors.Is(err, auth.ErrLastAdmin), errors.Is(err, auth.ErrLastFullAdmin):
		failure(w, 409, "account protection prevents this change")
	case errors.Is(err, auth.ErrProjectsRequired), errors.Is(err, auth.ErrUnknownPermission), errors.Is(err, auth.ErrNoPermissions), errors.Is(err, auth.ErrEmptyUsername), errors.Is(err, auth.ErrNoLoginMethod):
		failure(w, 400, err.Error())
	default:
		failure(w, 500, "account request failed")
	}
}

func (s *Server) accountView(r *http.Request, a auth.Admin) (map[string]any, error) {
	v := authhttp.AccountView(a)
	ids, err := s.auth.ProjectIDs(r.Context(), a.ID)
	if err != nil {
		return nil, err
	}
	totp, err := s.auth.TOTPStatus(r.Context(), a.ID)
	if err != nil {
		return nil, err
	}
	v["project_ids"] = ids
	v["auth_methods"] = map[string]bool{"password": a.HasPassword, "totp": totp.Enabled}
	return v, nil
}

func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request) {
	r, ok := s.accountWorkspace(w, r)
	if !ok {
		return
	}
	accounts, err := s.auth.ListAccounts(r.Context())
	if err != nil {
		accountError(w, err)
		return
	}
	list := []map[string]any{}
	for _, a := range accounts {
		if err := projects.RequireAccount(r.Context(), s.db.DB, a.ID); err != nil {
			if errors.Is(err, projects.ErrNotFound) {
				continue
			}
			accountError(w, err)
			return
		}
		view, err := s.accountView(r, a)
		if err != nil {
			accountError(w, err)
			return
		}
		list = append(list, view)
	}
	respond(w, 200, map[string]any{"accounts": list})
}

func (s *Server) validateAccountProjects(r *http.Request, ids []string) error {
	if len(ids) == 0 {
		return auth.ErrProjectsRequired
	}
	for _, id := range ids {
		if _, err := s.projects.Get(r.Context(), id); err != nil {
			return projects.ErrNotFound
		}
	}
	return nil
}

func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	r, ok := s.accountWorkspace(w, r)
	if !ok {
		return
	}
	var req struct {
		Username       string   `json:"username"`
		DisplayName    string   `json:"display_name"`
		Password       string   `json:"password"`
		Permissions    []string `json:"permissions"`
		ProjectIDs     []string `json:"project_ids"`
		GlobalProjects bool     `json:"global_projects"`
	}
	if !decode(w, r, &req) {
		return
	}
	actor := auth.AdminFromContext(r.Context())
	if !actor.IsBootstrapSuperuser && (req.GlobalProjects || len(req.ProjectIDs) != 1 || req.ProjectIDs[0] != projects.ID(r.Context())) {
		failure(w, 403, "account must belong only to the selected project")
		return
	}
	if err := auth.ValidatePassword(req.Password); err != nil {
		failure(w, 400, err.Error())
		return
	}
	if err := s.validateAccountProjects(r, req.ProjectIDs); err != nil {
		accountError(w, err)
		return
	}
	created, err := s.auth.CreateUser(r.Context(), auth.CreateUserInput{Username: req.Username, DisplayName: req.DisplayName, Password: req.Password, Permissions: req.Permissions, ProjectIDs: req.ProjectIDs, GlobalProjects: req.GlobalProjects})
	if err != nil {
		accountError(w, err)
		return
	}
	s.log(r, "auth.account.created", "admin", created.ID)
	view, err := s.accountView(r, *created)
	if err != nil {
		accountError(w, err)
		return
	}
	respond(w, 201, view)
}

func (s *Server) updateAccount(w http.ResponseWriter, r *http.Request) {
	r, ok := s.accountWorkspace(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if err := projects.RequireManagedAccount(r.Context(), s.db.DB, id); err != nil {
		accountError(w, err)
		return
	}
	target, err := s.auth.GetAccount(r.Context(), id)
	if err != nil {
		accountError(w, err)
		return
	}
	var req struct {
		DisplayName *string   `json:"display_name"`
		Status      *string   `json:"status"`
		Permissions *[]string `json:"permissions"`
		ProjectIDs  *[]string `json:"project_ids"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.DisplayName == nil && req.Status == nil && req.Permissions == nil && req.ProjectIDs == nil {
		failure(w, 400, "account change is required")
		return
	}
	actor := auth.AdminFromContext(r.Context())
	if actor.ID == id && (req.Status != nil || req.Permissions != nil || req.ProjectIDs != nil) {
		failure(w, 400, "cannot change your own access")
		return
	}
	// Validate all supplied fields before invoking any write service.
	if req.Status != nil && *req.Status != "active" && *req.Status != "invited" && *req.Status != "disabled" {
		failure(w, 400, "invalid account status")
		return
	}
	if req.Permissions != nil {
		perms, err := auth.NormalizePermissions(*req.Permissions)
		if err != nil {
			accountError(w, err)
			return
		}
		req.Permissions = &perms
		if target.IsBootstrapSuperuser && perms[0] != auth.PermissionAll {
			accountError(w, auth.ErrSuperuserProtected)
			return
		}
	}
	if target.IsBootstrapSuperuser && (req.ProjectIDs != nil || req.Status != nil && *req.Status != "active") {
		accountError(w, auth.ErrSuperuserProtected)
		return
	}
	if req.ProjectIDs != nil {
		if err := s.validateAccountProjects(r, *req.ProjectIDs); err != nil {
			accountError(w, err)
			return
		}
		if !actor.IsBootstrapSuperuser && (len(*req.ProjectIDs) != 1 || (*req.ProjectIDs)[0] != projects.ID(r.Context())) {
			failure(w, 403, "account must stay in the selected project")
			return
		}
	}
	updated, err := s.auth.UpdateUser(r.Context(), id, auth.UpdateUserInput{DisplayName: req.DisplayName, Status: req.Status, Permissions: req.Permissions, ProjectIDs: req.ProjectIDs}, func(ctx context.Context, tx *sql.Tx, a *auth.Admin) error {
		return audit.WriteTx(ctx, tx, audit.Entry{ActorType: "admin", ActorID: actor.Username, Action: "auth.account.updated", ResourceType: "admin", ResourceID: a.ID})
	})
	if err != nil {
		accountError(w, err)
		return
	}
	view, err := s.accountView(r, *updated)
	if err != nil {
		accountError(w, err)
		return
	}
	respond(w, 200, view)
}

func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	r, ok := s.accountWorkspace(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if id == auth.AdminFromContext(r.Context()).ID {
		failure(w, 400, "cannot delete your own account")
		return
	}
	if err := projects.RequireManagedAccount(r.Context(), s.db.DB, id); err != nil {
		accountError(w, err)
		return
	}
	if err := s.auth.DeleteAccount(r.Context(), id); err != nil {
		accountError(w, err)
		return
	}
	s.log(r, "auth.account.deleted", "admin", id)
	respond(w, 200, map[string]string{"status": "account deleted"})
}

func (s *Server) permissions(w http.ResponseWriter, r *http.Request) {
	groups := []auth.PermissionGroup{}
	for _, g := range auth.PermissionRegistry() {
		if g.Key == "accounts" || g.Key == "settings" || g.Key == "audit" {
			groups = append(groups, g)
		}
	}
	respond(w, 200, map[string]any{"groups": groups})
}
