// Package authhttp owns shared session transport and cookie/CSRF protections.
package authhttp

import (
	"encoding/json"
	"errors"
	"github.com/axottle/axottle/internal/auth"
	"github.com/axottle/axottle/internal/setup"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const SessionCookieName = "axo_session"
const AuthFlagCookieName = "axo_authed"

type Handler struct {
	Auth       *auth.Service
	Setup      *setup.Service
	LoginGuard *auth.LoginGuard
	PublicURL  string
	ClientIP   func(*http.Request) string
}

func (s *Handler) clientIP(r *http.Request) string {
	if s.ClientIP != nil {
		return s.ClientIP(r)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func (s *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		TOTPCode string `json:"totp_code"`
	}

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	// A bootstrap administrator must exist before sessions can be created.
	if s.Setup == nil || !s.Setup.IsComplete() {
		writeError(w, http.StatusForbidden, "первичная настройка не завершена: создайте администратора")
		return
	}

	// Brute-force throttle: key on username+IP so repeated failures lock out a
	// single origin without letting an attacker lock a victim out from every
	// IP. The lockout window expires on its own, so a legitimate user recovers.
	guardKey := req.Username + "|" + s.clientIP(r)
	if s.LoginGuard.Locked(guardKey) {
		writeError(w, http.StatusTooManyRequests,
			"too many failed login attempts; please try again later")
		return
	}

	session, err := s.Auth.Login(r.Context(), req.Username, req.Password, req.TOTPCode)
	if err != nil {
		if errors.Is(err, auth.ErrTOTPRequired) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "two-factor authentication required",
				"code":  "TOTP_REQUIRED",
			})
			return
		}
		// The credentials were right and the account still has nothing to open
		// — a switched-off support operator, typically. Say so: repeating
		// "invalid credentials" at somebody whose password IS valid sends them
		// resetting it instead of asking the owner. Not a brute-force signal,
		// so the guard is left alone.
		var denied *auth.AccessDeniedError
		if errors.As(err, &denied) {
			s.LoginGuard.Reset(guardKey)
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": denied.Reason,
				"code":  denied.Code,
			})
			return
		}
		s.LoginGuard.Fail(guardKey)
		if errors.Is(err, auth.ErrSecretsUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "two-factor authentication is temporarily unavailable")
			return
		}
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	s.LoginGuard.Reset(guardKey)

	// Seal the setup surface on the first successful login: the setup wizard
	// runs entirely before any login, so this closes the public /setup/*
	// mutators once the panel is actually operating. Idempotent.
	if s.Setup != nil {
		s.Setup.MarkSealed(r.Context())
	}

	// Set the browser session cookie (§9). The token is still returned in the
	// body for CLI/Bearer clients, but browser sessions live in an HttpOnly
	// cookie and never touch localStorage.
	s.SetSessionCookie(w, r, session)

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      session.Token,
		"expires_at": session.ExpiresAt,
	})
}

func (s *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token, _ := s.RequestToken(r)
	if token == "" {
		writeError(w, http.StatusBadRequest, "no token provided")
		return
	}

	if err := s.Auth.Logout(r.Context(), token); err != nil {
		writeError(w, http.StatusInternalServerError, "logout failed")
		return
	}

	// Clear the browser session cookie (harmless for Bearer clients).
	s.ClearSessionCookie(w, r)

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (s *Handler) Me(w http.ResponseWriter, r *http.Request) {
	admin := auth.AdminFromContext(r.Context())
	if admin == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	writeJSON(w, http.StatusOK, AccountView(*admin))
}

func (s *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, viaCookie := s.RequestToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "authorization required")
			return
		}

		admin, err := s.Auth.ValidateToken(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		// CSRF: a cookie rides along automatically, so a cookie-authenticated
		// state-changing request must prove it came from our own origin. Bearer
		// requests (CLI, integrations) carry no ambient credential and are
		// exempt so they never break on this check (§9).
		if viaCookie && IsUnsafeMethod(r.Method) && !s.SameOriginRequest(r) {
			writeError(w, http.StatusForbidden, "cross-origin request blocked")
			return
		}

		ctx := auth.ContextWithAdmin(r.Context(), admin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestToken returns the session token and whether it came from the browser
// cookie (vs a Bearer header). Bearer wins when both are present, so a CLI token
// is never shadowed by a stale cookie.
func (s *Handler) RequestToken(r *http.Request) (token string, viaCookie bool) {
	if t := auth.TokenFromRequest(r); t != "" {
		return t, false
	}
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		return c.Value, true
	}
	return "", false
}

func IsUnsafeMethod(m string) bool {
	switch m {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// SameOriginRequest reports whether an unsafe cookie-authenticated request came
// from the panel's own origin, checked via the Origin header (preferred) or
// Referer. A request with neither header is rejected — every browser sends one
// on a state-changing fetch/navigation.
func (s *Handler) SameOriginRequest(r *http.Request) bool {
	check := func(raw string) (bool, bool) {
		if raw == "" {
			return false, false
		}
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			return false, true // present but unparseable → treated as mismatch
		}
		return strings.EqualFold(u.Host, r.Host) || strings.EqualFold(u.Host, s.PublicHost()), true
	}
	if ok, present := check(r.Header.Get("Origin")); present {
		return ok
	}
	if ok, present := check(r.Header.Get("Referer")); present {
		return ok
	}
	return false
}

// PublicHost is the host of the configured public URL, for matching an Origin
// when the panel sits behind a proxy that rewrites Host.
func (s *Handler) PublicHost() string {
	if s.PublicURL == "" {
		return ""
	}
	if u, err := url.Parse(s.PublicURL); err == nil {
		return u.Host
	}
	return ""
}

// OriginAllowsCredentials reports whether a CORS Origin may receive credentialed
// responses: only the panel's own origin (same host as the request, or the
// configured public URL's host). Everything else is a public, credential-less
// CORS caller (Bearer clients, unauthenticated endpoints).
func (s *Handler) OriginAllowsCredentials(origin string, r *http.Request) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host) || (s.PublicHost() != "" && strings.EqualFold(u.Host, s.PublicHost()))
}

func (s *Handler) IsSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	return strings.HasPrefix(strings.ToLower(s.PublicURL), "https://")
}

func (s *Handler) SetSessionCookie(w http.ResponseWriter, r *http.Request, session *auth.Session) {
	secure := s.IsSecureRequest(r)
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    session.Token,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	// Readable by the panel's client-side guards (see AuthFlagCookieName).
	http.SetCookie(w, &http.Cookie{
		Name:     AuthFlagCookieName,
		Value:    "1",
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Handler) ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	secure := s.IsSecureRequest(r)
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     AuthFlagCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
