package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

func TestInternalPathVariantsNeverReachApplication(t *testing.T) {
	s, err := New(Config{Version: "test", Application: http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("internal path forwarded") })})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/_axottle", "/_axottle/engine/identity", "//_axottle/engine/identity", "/x/../_axottle/engine/identity", "/%5faxottle/engine/identity", "/%255faxottle/engine/identity", "/x/%2e%2e/_axottle/engine/identity", "/%2f_axottle/engine/identity", "/x%5c..%5c_axottle/engine/identity"} {
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Errorf("%s: %d", path, w.Code)
		}
		if w.Header().Get("Content-Security-Policy") == "" || w.Header().Get("X-Frame-Options") != "DENY" {
			t.Fatal("missing security headers on error")
		}
	}
}

func TestReadinessCoalescesConcurrentRequests(t *testing.T) {
	var calls atomic.Int32
	s, err := New(Config{Version: "test", Readiness: func(context.Context) error { calls.Add(1); return nil }})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/health", nil))
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("readiness probes=%d", calls.Load())
	}
}
