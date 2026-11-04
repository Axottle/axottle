// Package core is the independently buildable open Axottle entry point.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/axottle/axottle/internal/authhttp"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/axottle/axottle/internal/enginebridge"
)

type Config struct {
	Version        string
	EngineSocket   string
	TrustedProxies []string
	Application    http.Handler
	Readiness      func(context.Context) error
}

type Server struct {
	config         Config
	engine         *enginebridge.Client
	proxy          http.Handler
	trustedProxies []netip.Prefix
	statusMu       sync.Mutex
	statusUntil    time.Time
	statusErr      error
	statusDatabase bool
}

func New(cfg Config) (*Server, error) {
	if strings.TrimSpace(cfg.Version) == "" {
		return nil, fmt.Errorf("core version is required")
	}
	s := &Server{config: cfg}
	var err error
	s.trustedProxies, err = parseTrustedProxies(cfg.TrustedProxies)
	if err != nil {
		return nil, err
	}
	if cfg.EngineSocket != "" {
		client, err := enginebridge.NewClient(cfg.EngineSocket)
		if err != nil {
			return nil, err
		}
		s.engine, s.proxy = client, client.Proxy()
	}
	return s, nil
}

func (s *Server) Close() {
	if s.engine != nil {
		s.engine.Close()
	}
}

// A single in-flight probe and a short TTL bound public readiness work.
func (s *Server) readiness(ctx context.Context) (error, bool) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	if time.Now().Before(s.statusUntil) {
		return s.statusErr, s.statusDatabase
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var err error
	database := false
	if s.engine != nil {
		var id enginebridge.Identity
		id, err = s.engine.Identity(ctx)
		if err == nil {
			err = id.Validate(s.config.Version)
		}
	}
	if err == nil && s.config.Readiness != nil {
		err = s.config.Readiness(ctx)
		database = err != nil
	}
	s.statusErr, s.statusDatabase = err, database
	s.statusUntil = time.Now().Add(time.Second)
	return err, database
}

// Refuse ambiguous paths before a downstream router can canonicalize them.
// Decode nested escapes only for validation; the original URL is forwarded.
func publicPathAllowed(u *url.URL) bool {
	p := u.EscapedPath()
	for i := 0; i < 8; i++ {
		decoded, err := url.PathUnescape(p)
		if err != nil {
			return false
		}
		canonical := path.Clean(decoded)
		if canonical == "/_axottle" || strings.HasPrefix(canonical, "/_axottle/") || strings.Contains(decoded, "\\") || strings.Contains(decoded, "//") {
			return false
		}
		for _, segment := range strings.Split(decoded, "/") {
			if segment == "." || segment == ".." {
				return false
			}
		}
		if decoded == p {
			return true
		}
		p = decoded
	}
	return false
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Internal routes are only reachable over the engine's socket, never via
	// the public proxy, including unrecognised future internal endpoints.
	request := s.proxyRequest(r)
	authhttp.SecurityHeaders(w, request.URL.Scheme == "https")
	if !publicPathAllowed(r.URL) {
		http.NotFound(w, r)
		return
	}
	if r.URL.Path == "/health" || r.URL.Path == "/api/v1/core/status" {
		err, database := s.readiness(r.Context())
		code, status := http.StatusOK, "ok"
		if database {
			code, status = http.StatusServiceUnavailable, "database_unavailable"
		}
		if err != nil {
			if code == http.StatusOK {
				code, status = http.StatusServiceUnavailable, "engine_unavailable"
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": status, "version": s.config.Version, "component": "axottle",
			"engine_configured": s.engine != nil,
		})
		return
	}
	if s.engine == nil {
		if s.config.Application != nil {
			request := s.proxyRequest(r)
			// The local API must see only forwarding data verified against the
			// transport peer, just like the Unix engine proxy does.
			for _, key := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Real-IP", "X-Forwarded-Proto"} {
				request.Header.Del(key)
			}
			request.Header.Set("X-Forwarded-Proto", request.URL.Scheme)
			s.config.Application.ServeHTTP(w, request)
			return
		}
		http.Error(w, "Axottle core is running; the engine is not configured", http.StatusServiceUnavailable)
		return
	}
	if err, _ := s.readiness(r.Context()); err != nil {
		http.Error(w, "Axottle engine is unavailable or incompatible", http.StatusServiceUnavailable)
		return
	}
	s.proxy.ServeHTTP(w, s.proxyRequest(r))
}
