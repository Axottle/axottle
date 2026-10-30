// Package core is the independently buildable open Axottle entry point.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"strings"

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

func (s *Server) engineStatus(ctx context.Context) error {
	if s.engine == nil {
		return nil
	}
	id, err := s.engine.Identity(ctx)
	if err != nil {
		return err
	}
	return id.Validate(s.config.Version)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Internal routes are only reachable over the engine's socket, never via
	// the public proxy, including unrecognised future internal endpoints.
	if strings.HasPrefix(r.URL.Path, "/_axottle/") {
		http.NotFound(w, r)
		return
	}
	if r.URL.Path == "/health" || r.URL.Path == "/api/v1/core/status" {
		err := s.engineStatus(r.Context())
		code, status := http.StatusOK, "ok"
		if err == nil && s.config.Readiness != nil {
			err = s.config.Readiness(r.Context())
			if err != nil {
				code, status = http.StatusServiceUnavailable, "database_unavailable"
			}
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
	if err := s.engineStatus(r.Context()); err != nil {
		http.Error(w, "Axottle engine is unavailable or incompatible", http.StatusServiceUnavailable)
		return
	}
	s.proxy.ServeHTTP(w, s.proxyRequest(r))
}
