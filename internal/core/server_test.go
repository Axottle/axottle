package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/axottle/axottle/internal/enginebridge"
)

// Use a short path: Darwin's sockaddr_un is shorter than a typical t.TempDir.
func engineFixture(t *testing.T, version string, handler http.Handler) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "axbridge-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "s")
	listener, err := enginebridge.ListenUnix(socket)
	if err != nil {
		t.Fatal(err)
	}
	identity := enginebridge.IdentityHandler(version)
	module := enginebridge.RestorePeer(handler)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == enginebridge.IdentityPath {
			identity.ServeHTTP(w, r)
			return
		}
		module.ServeHTTP(w, r)
	})}
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() { _ = srv.Close() })
	return socket
}

func TestStandaloneCore(t *testing.T) {
	srv, err := New(Config{Version: "v1.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	for path, code := range map[string]int{"/health": 200, "/api/v1/core/status": 200, "/api/v1/nodes": 503, "/_axottle/engine/identity": 404} {
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != code {
			t.Errorf("%s: %d, want %d", path, w.Code, code)
		}
	}
}

func TestLocalAPIUsesVerifiedForwardingAndDatabaseReadiness(t *testing.T) {
	for _, tc := range []struct{ peer, wantScheme, wantPeer string }{
		{"198.51.100.9:1234", "http", "198.51.100.9:1234"},
		{"127.0.0.1:1234", "https", "192.0.2.8:0"},
	} {
		t.Run(tc.peer, func(t *testing.T) {
			called := false
			var unavailable bool
			srv, err := New(Config{Version: "v1.2.3", TrustedProxies: []string{"127.0.0.1/32"},
				Readiness: func(context.Context) error {
					if unavailable {
						return fmt.Errorf("database unavailable")
					}
					return nil
				},
				Application: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					called = true
					if r.Header.Get("X-Forwarded-Proto") != tc.wantScheme || r.URL.Scheme != tc.wantScheme || r.RemoteAddr != tc.wantPeer {
						t.Errorf("unverified transport information reached local API: %s %s %s", r.Header.Get("X-Forwarded-Proto"), r.URL.Scheme, r.RemoteAddr)
					}
					for _, key := range []string{"X-Real-IP", "X-Forwarded-For", "X-Forwarded-Host", "Forwarded"} {
						if r.Header.Get(key) != "" {
							t.Errorf("raw %s reached local API", key)
						}
					}
					w.WriteHeader(204)
				})})
			if err != nil {
				t.Fatal(err)
			}
			defer srv.Close()
			r := httptest.NewRequest("GET", "http://panel.example/api/v1/auth/me", nil)
			r.RemoteAddr = tc.peer
			r.Header.Set("X-Forwarded-Proto", "https")
			r.Header.Set("X-Forwarded-For", "192.0.2.8")
			r.Header.Set("Forwarded", "for=evil")
			r.Header.Set("X-Real-IP", "192.0.2.9")
			r.Header.Set("X-Forwarded-Host", "evil.example")
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, r)
			if !called || w.Code != 204 {
				t.Fatalf("local API was not dispatched: %d", w.Code)
			}
			unavailable = true
			w = httptest.NewRecorder()
			srv.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))
			if w.Code != 503 || !strings.Contains(w.Body.String(), "database_unavailable") {
				t.Fatalf("false readiness: %d %s", w.Code, w.Body)
			}
		})
	}
}

func TestProxyPreservesSessionScopeAndTransportPeer(t *testing.T) {
	called := false
	socket := engineFixture(t, "v1.2.3", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		body, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || r.URL.RequestURI() != "/api/v1/nodes?id=a%2Fb" || string(body) != "payload" {
			t.Errorf("request changed: %s %s %s", r.Method, r.URL, body)
		}
		if r.Host != "panel.example" || r.Header.Get("Cookie") != "session=secret" || r.Header.Get("X-Axottle-Project") != "project-1" {
			t.Error("host, session or project lost")
		}
		if r.RemoteAddr != "192.0.2.9:1234" || r.Header.Get("X-Forwarded-For") != "192.0.2.9" || r.Header.Get(enginebridge.PeerHeader) != "" || r.Header.Get("X-Real-IP") != "" || r.Header.Get("Forwarded") != "" {
			t.Error("untrusted peer headers survived")
		}
		w.Header().Add("Set-Cookie", "session=new; HttpOnly; Secure")
		w.WriteHeader(http.StatusCreated)
	}))
	srv, err := New(Config{Version: "v1.2.3", EngineSocket: socket})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	r := httptest.NewRequest("POST", "http://panel.example/api/v1/nodes?id=a%2Fb", strings.NewReader("payload"))
	r.RemoteAddr = "192.0.2.9:1234"
	r.Header.Set("Cookie", "session=secret")
	r.Header.Set("X-Axottle-Project", "project-1")
	r.Header.Set("Forwarded", "for=127.0.0.1")
	r.Header.Set("X-Real-IP", "127.0.0.1")
	r.Header.Set("X-Forwarded-For", "127.0.0.1")
	r.Header.Set(enginebridge.PeerHeader, "127.0.0.1:1")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if !called || w.Code != 201 || w.Header().Get("Set-Cookie") == "" {
		t.Fatalf("proxy response: %d %s", w.Code, w.Body)
	}
}

func TestMissingOrMismatchedEngineBlocksTraffic(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		t.Run(fmt.Sprint(mismatch), func(t *testing.T) {
			dir, err := os.MkdirTemp("", "axmissing-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)
			socket := filepath.Join(dir, "s")
			if mismatch {
				socket = engineFixture(t, "v9.0.0", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("incompatible engine received request") }))
			}
			srv, err := New(Config{Version: "v1.2.3", EngineSocket: socket})
			if err != nil {
				t.Fatal(err)
			}
			defer srv.Close()
			for _, path := range []string{"/health", "/api/v1/nodes"} {
				w := httptest.NewRecorder()
				srv.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
				if w.Code != 503 {
					t.Errorf("%s: %d", path, w.Code)
				}
			}
		})
	}
}

func TestStreamingIsNotBuffered(t *testing.T) {
	release := make(chan struct{})
	socket := engineFixture(t, "v1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	srv, err := New(Config{Version: "v1", EngineSocket: socket})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	front := httptest.NewServer(srv)
	defer front.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", front.URL+"/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	defer resp.Body.Close()
	buf := make([]byte, len("data: first\n\n"))
	_, err = io.ReadFull(resp.Body, buf)
	close(release)
	if err != nil || string(buf) != "data: first\n\n" {
		t.Fatalf("stream was buffered: %q %v", buf, err)
	}
}

func TestReadinessReportsConfiguredEngine(t *testing.T) {
	socket := engineFixture(t, "v1", http.NotFoundHandler())
	srv, err := New(Config{Version: "v1", EngineSocket: socket})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))
	var status map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || status["engine_configured"] != true {
		t.Fatalf("readiness: %d %v", w.Code, status)
	}
}
