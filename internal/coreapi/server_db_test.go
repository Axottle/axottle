package coreapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/axottle/axottle/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func standaloneDB(t *testing.T) (*Server, Config) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := stdlib.OpenDB(*config)
	schema := fmt.Sprintf("core_only_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	// Exclude public from search_path: private tables in the harness's database
	// must never satisfy a missing core dependency accidentally.
	config.RuntimeParams["search_path"] = schema
	registered := stdlib.RegisterConnConfig(config)
	cfg := Config{DatabaseURL: registered, SecretKey: strings.Repeat("a", 64), PublicURL: "http://core.example", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	s, err := Open(t.Context(), cfg)
	if err != nil {
		_, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		admin.Close()
		stdlib.UnregisterConnConfig(registered)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.Close()
		_, err := admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		if err != nil {
			t.Error(err)
		}
		admin.Close()
		stdlib.UnregisterConnConfig(registered)
	})
	return s, cfg
}

func call(s http.Handler, method, path, body, token, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://core.example"+path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestStandaloneCoreWithoutPrivateTablesDB(t *testing.T) {
	s, cfg := standaloneDB(t)
	assert := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("status=%d want=%d body=%s", w.Code, status, w.Body.String())
		}
	}
	assert(call(s, "GET", "/api/v1/auth/me", "", "", ""), 401)
	assert(call(s, "POST", "/api/v1/setup/admin", `{"username":"owner","password":"core-only-password"}`, "", ""), 201)
	w := call(s, "POST", "/api/v1/auth/login", `{"username":"owner","password":"core-only-password"}`, "", "")
	assert(w, 200)
	var session auth.Session
	if err := json.Unmarshal(w.Body.Bytes(), &session); err != nil || session.Token == "" {
		t.Fatalf("login: %s %v", w.Body.String(), err)
	}
	var cookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "axo_session" {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("browser session cookie protection lost")
	}
	assert(call(s, "GET", "/api/v1/auth/me", "", session.Token, ""), 200)
	assert(call(s, "POST", "/api/v1/setup/admin", `{"username":"intruder","password":"core-only-password"}`, "", ""), 403)
	w = call(s, "POST", "/api/v1/projects", `{"name":"Standalone","description":"No infrastructure"}`, session.Token, "")
	assert(w, 201)
	var project struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &project); err != nil || project.ID == "" {
		t.Fatal(w.Body.String(), err)
	}
	assert(call(s, "GET", "/api/v1/projects/"+project.ID, "", session.Token, ""), 200)
	assert(call(s, "PATCH", "/api/v1/projects/"+project.ID, `{"name":"Renamed"}`, session.Token, ""), 200)
	assert(call(s, "PATCH", "/api/v1/settings/general", `{"settings":{"panel.name":"My Axottle"}}`, session.Token, ""), 200)
	assert(call(s, "PATCH", "/api/v1/settings/general", `{"settings":{"installation.id":"overwrite"}}`, session.Token, ""), 400)
	assert(call(s, "PATCH", "/api/v1/settings/general", `{"settings":{"panel.language":"unsupported"}}`, session.Token, ""), 400)
	assert(call(s, "PATCH", "/api/v1/settings/general", `{"settings":{"panel.timezone":"unknown/timezone"}}`, session.Token, ""), 400)
	if _, err := s.db.Exec(`INSERT INTO system_settings(key,value_json) VALUES('private.credentials','{"token":"private-fixture-only"}'::jsonb)`); err != nil {
		t.Fatal(err)
	}
	presentation := call(s, "GET", "/api/v1/core/presentation", "", "", "")
	assert(presentation, 200)
	var publicPreferences map[string]string
	if err := json.Unmarshal(presentation.Body.Bytes(), &publicPreferences); err != nil || len(publicPreferences) != 3 || publicPreferences["name"] != "My Axottle" || strings.Contains(presentation.Body.String(), "private-fixture") {
		t.Fatalf("public settings projection: %s %v", presentation.Body, err)
	}
	w = call(s, "GET", "/api/v1/audit", "", session.Token, "")
	assert(w, 200)
	if !strings.Contains(w.Body.String(), "project.created") {
		t.Fatal("audit entry missing")
	}
	// Cookie credentials require a verified same-origin browser mutation.
	for _, tc := range []struct {
		origin string
		status int
	}{{"", 403}, {"https://evil.example", 403}, {"http://core.example", 200}} {
		r := httptest.NewRequest("PATCH", "http://core.example/api/v1/settings/general", strings.NewReader(`{"settings":{"panel.name":"Cookie"}}`))
		r.AddCookie(cookie)
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		assert(w, tc.status)
	}
	// Core schemas must contain no private inventory or commercial remnants.
	var privateTables int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name IN ('nodes','agents','support_agents','remnawave_connections','licenses','license_reservations','connection_bundles')`).Scan(&privateTables); err != nil || privateTables != 0 {
		t.Fatalf("private schema leaked: %d %v", privateTables, err)
	}
	// Exercise the shared account service, persisted project grants and immediate
	// revocation against the same minimal schema used by the source-only build.
	account, err := s.auth.CreateUser(t.Context(), auth.CreateUserInput{Username: "scoped", Password: "scoped-core-password", ProjectIDs: []string{project.ID}, Permissions: []string{"settings.view"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := auth.ContextWithAdmin(t.Context(), account)
	list, err := s.projects.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("grant: %+v %v", list, err)
	}
	if _, err := s.db.Exec(`DELETE FROM admin_projects WHERE admin_id=$1`, account.ID); err != nil {
		t.Fatal(err)
	}
	list, err = s.projects.List(ctx)
	if err != nil || len(list) != 0 {
		t.Fatalf("revocation: %+v %v", list, err)
	}
	// A failing module extension rolls the core user back atomically.
	_, err = s.auth.CreateUser(t.Context(), auth.CreateUserInput{Username: "rollback", Password: "rollback-password", ProjectIDs: []string{project.ID}}, func(context.Context, *sql.Tx, *auth.Admin) error { return fmt.Errorf("module refused") })
	if err == nil {
		t.Fatal("module error accepted")
	}
	var rolledBack int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM admins WHERE username='rollback'`).Scan(&rolledBack); err != nil || rolledBack != 0 {
		t.Fatalf("module rollback: %d %v", rolledBack, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	assert(call(restarted, "GET", "/api/v1/auth/me", "", session.Token, ""), 200)
	assert(call(restarted, "POST", "/api/v1/setup/admin", `{"username":"late","password":"late-password"}`, "", ""), 403)
}

func TestConcurrentStandaloneBootstrapDB(t *testing.T) {
	s, _ := standaloneDB(t)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, name := range []string{"first", "second"} {
		wg.Add(1)
		go func() { defer wg.Done(); results <- s.setup.CreateAdmin(t.Context(), name, "parallel-password") }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else {
			t.Logf("bootstrap refusal: %v", err)
		}
	}
	if success != 1 {
		t.Fatalf("bootstrap winners=%d", success)
	}
}

func TestStandaloneAccountHTTPAndProjectBoundariesDB(t *testing.T) {
	s, _ := standaloneDB(t)
	check := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("status=%d want=%d %s", w.Code, status, w.Body)
		}
	}
	check(call(s, "POST", "/api/v1/setup/admin", `{"username":"owner","password":"owner-core-password"}`, "", ""), 201)
	owner, err := s.auth.Login(t.Context(), "owner", "owner-core-password", "")
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.projects.Create(t.Context(), "First", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.projects.Create(t.Context(), "Second", "")
	if err != nil {
		t.Fatal(err)
	}
	create := func(username string, ids []string) string {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"username": username, "password": "account-core-password", "project_ids": ids, "permissions": []string{"*"}})
		w := call(s, "POST", "/api/v1/auth/accounts", string(body), owner.Token, "")
		check(w, 201)
		var value struct{ ID string }
		if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil || value.ID == "" {
			t.Fatal(w.Body.String(), err)
		}
		if strings.Contains(w.Body.String(), "account-core-password") || strings.Contains(w.Body.String(), "password_hash") {
			t.Fatal("account API exposed credentials")
		}
		return value.ID
	}
	managerID := create("manager", []string{first.ID})
	editableID := create("editable", []string{first.ID})
	foreignID := create("foreign", []string{second.ID})
	sharedID := create("shared", []string{first.ID, second.ID})
	manager, err := s.auth.Login(t.Context(), "manager", "account-core-password", "")
	if err != nil {
		t.Fatal(err)
	}
	w := call(s, "GET", "/api/v1/auth/accounts", "", manager.Token, "")
	check(w, 200)
	if strings.Contains(w.Body.String(), foreignID) || !strings.Contains(w.Body.String(), editableID) {
		t.Fatal("scoped account discovery crossed project membership")
	}
	for _, id := range []string{foreignID, sharedID} {
		check(call(s, "PATCH", "/api/v1/auth/accounts/"+id, `{"display_name":"forged"}`, manager.Token, ""), 404)
	}
	check(call(s, "PATCH", "/api/v1/auth/accounts/"+managerID, `{"permissions":["settings.view"]}`, manager.Token, ""), 400)
	check(call(s, "PATCH", "/api/v1/auth/accounts/"+editableID, `{"display_name":"Edited","permissions":["settings.view","audit.view"]}`, manager.Token, ""), 200)
	updated, err := s.auth.GetAccount(t.Context(), editableID)
	if err != nil || updated.DisplayName != "Edited" || !updated.Can(auth.PermAuditView) {
		t.Fatalf("edit did not persist: %+v %v", updated, err)
	}
	// Invalid project grants must be rejected before the profile is written.
	body, _ := json.Marshal(map[string]any{"display_name": "must roll back", "project_ids": []string{second.ID}})
	check(call(s, "PATCH", "/api/v1/auth/accounts/"+editableID, string(body), manager.Token, ""), 404)
	updated, err = s.auth.GetAccount(t.Context(), editableID)
	if err != nil || updated.DisplayName != "Edited" {
		t.Fatal("refused cross-project edit changed the profile")
	}
	activeSession, err := s.auth.Login(t.Context(), "editable", "account-core-password", "")
	if err != nil {
		t.Fatal(err)
	}
	check(call(s, "PATCH", "/api/v1/auth/accounts/"+editableID, `{"status":"disabled"}`, owner.Token, ""), 200)
	check(call(s, "GET", "/api/v1/auth/me", "", activeSession.Token, ""), 401)
	check(call(s, "PATCH", "/api/v1/auth/accounts/"+editableID, `{"status":"active"}`, owner.Token, ""), 200)
	check(call(s, "DELETE", "/api/v1/auth/accounts/"+foreignID, "", manager.Token, ""), 404)
	check(call(s, "DELETE", "/api/v1/auth/accounts/"+foreignID, "", owner.Token, ""), 200)
	check(call(s, "GET", "/api/v1/auth/permissions", "", owner.Token, ""), 200)
}

func TestAtomicUserEditRollsBackFieldsGrantsAndSessionRevocationDB(t *testing.T) {
	s, _ := standaloneDB(t)
	if err := s.setup.CreateAdmin(t.Context(), "owner", "atomic-owner-password"); err != nil {
		t.Fatal(err)
	}
	p, err := s.projects.Create(t.Context(), "Atomic", "")
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.auth.CreateUser(t.Context(), auth.CreateUserInput{Username: "editable", Password: "atomic-user-password", ProjectIDs: []string{p.ID}, Permissions: []string{"settings.view"}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := s.auth.Login(t.Context(), "editable", "atomic-user-password", "")
	if err != nil {
		t.Fatal(err)
	}
	name, status := "must roll back", "disabled"
	permissions := []string{"*"}
	badGrants := []string{"missing-project"}
	if _, err := s.auth.UpdateUser(t.Context(), a.ID, auth.UpdateUserInput{DisplayName: &name, Status: &status, Permissions: &permissions, ProjectIDs: &badGrants}); err == nil {
		t.Fatal("invalid grant committed")
	}
	goodGrants := []string{p.ID}
	if _, err := s.auth.UpdateUser(t.Context(), a.ID, auth.UpdateUserInput{DisplayName: &name, Status: &status, Permissions: &permissions, ProjectIDs: &goodGrants}, func(context.Context, *sql.Tx, *auth.Admin) error {
		return fmt.Errorf("extension refused after session revocation")
	}); err == nil {
		t.Fatal("extension failure committed")
	}
	updated, err := s.auth.GetAccount(t.Context(), a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.DisplayName != a.DisplayName || updated.Status != "active" || updated.FullAccess() {
		t.Fatalf("partial edit escaped rollback: %+v", updated)
	}
	ids, err := s.auth.ProjectIDs(t.Context(), a.ID)
	if err != nil || len(ids) != 1 || ids[0] != p.ID {
		t.Fatalf("grants changed: %v %v", ids, err)
	}
	if _, err := s.auth.ValidateToken(t.Context(), session.Token); err != nil {
		t.Fatalf("rolled-back edit revoked session: %v", err)
	}
}
