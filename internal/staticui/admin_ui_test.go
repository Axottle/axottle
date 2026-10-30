package staticui

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// fakeFS builds an in-memory fstest.MapFS that mirrors the layout Next.js
// static export produces: flat "<route>.html" files plus nested "<route>/"
// directories for sub-routes. This is the exact shape that triggered the
// /new bootloop (root index.html redirecting back to /new).
func fakeFS() fs.FS {
	m := map[string]*fstest.MapFile{
		"index.html":                            {Data: []byte("ROOT-REDIRECT-TO-NEW")},
		"new.html":                              {Data: []byte("NEW-PAGE-OK")},
		"new/abuse.html":                        {Data: []byte("ABUSE-PAGE")},
		"new/nodes.html":                        {Data: []byte("NODES-PAGE")},
		"new/nodes.txt":                         {Data: []byte("NODES-RSC")},
		"new/nodes/placeholder.html":            {Data: []byte("NODE-DETAIL-SHELL")},
		"new/nodes/placeholder.txt":             {Data: []byte(`0:{"c":["","new","nodes","placeholder"],"segment":["id","placeholder","d"],"params":{"id":"placeholder"}}`)},
		"new/connection-units.html":             {Data: []byte("CONNECTION-UNITS-PAGE")},
		"new/connection-units.txt":              {Data: []byte("CONNECTION-UNITS-RSC")},
		"new/connection-units/placeholder.html": {Data: []byte("CONNECTION-UNIT-DETAIL-SHELL")},
		"new/geo-assets.html":                   {Data: []byte("GEO-ASSETS-PAGE")},
		"new/geo-assets/placeholder.html":       {Data: []byte("GEO-ASSET-DETAIL-SHELL")},
		"setup.html":                            {Data: []byte("SETUP-PAGE")},
		"login.html":                            {Data: []byte("LOGIN-PAGE")},
	}
	return fstest.MapFS(m)
}

func TestServeAdminUI_DynamicFlightPreservesNavigation(t *testing.T) {
	for _, tc := range []struct{ url, id string }{
		{"/new/nodes/node-123.txt?_rsc=navigation", `"node-123"`},
		{"/new/nodes/node-456.txt", `"node-456"`},
		{"/new/nodes/node-123/index.txt", `"node-123"`},
		{"/new/nodes/node%22quoted.txt", `"node\"quoted"`},
		{"/new/nodes/%24L1.txt", `"$$L1"`},
	} {
		t.Run(tc.url, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			req.Header.Set("RSC", "1")
			rec := httptest.NewRecorder()
			Serve(rec, req, fakeFS())
			body := rec.Body.String()
			if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") {
				t.Fatalf("Flight request returned %d %s: %s", rec.Code, rec.Header().Get("Content-Type"), body)
			}
			if strings.Contains(body, `"placeholder"`) || strings.Count(body, tc.id) != 3 {
				t.Fatalf("Flight path, segment and params must use %s: %s", tc.id, body)
			}
		})
	}
}

func TestServeAdminUI_FlightNeverFallsBackToHTML(t *testing.T) {
	for _, url := range []string{"/anything.txt", "/new/unknown.txt", "/new/geo-assets/missing.txt"} {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("RSC", "1")
		rec := httptest.NewRecorder()
		Serve(rec, req, fakeFS())
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "SHELL") || strings.Contains(rec.Body.String(), "ROOT-REDIRECT") {
			t.Fatalf("Flight %s used an HTML fallback: %d %s", url, rec.Code, rec.Body.String())
		}
	}
}

func TestServeAdminUI_FlightTrailingSlash(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/new/nodes/index.txt", nil)
	req.Header.Set("RSC", "1")
	rec := httptest.NewRecorder()
	Serve(rec, req, fakeFS())
	if rec.Code != http.StatusOK || rec.Body.String() != "NODES-RSC" {
		t.Fatalf("trailing-slash Flight returned %d %s", rec.Code, rec.Body.String())
	}
}

func TestServeAdminUI_HidesFlightPayloadFromDirectRequests(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{name: "exported payload", path: "/new/connection-units.txt"},
		{name: "case insensitive extension", path: "/new/connection-units.TXT"},
		{name: "unknown text path", path: "/anything.txt"},
		{name: "query string", path: "/new/connection-units.txt?_rsc=direct-navigation"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()

			Serve(rec, req, fakeFS())

			if rec.Code != http.StatusNotFound {
				t.Fatalf("GET %s status = %d, want %d", tc.path, rec.Code, http.StatusNotFound)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("GET %s Cache-Control = %q, want no-store", tc.path, got)
			}
			if got := rec.Header().Get("Vary"); got != "RSC" {
				t.Fatalf("GET %s Vary = %q, want RSC", tc.path, got)
			}
			if strings.Contains(rec.Body.String(), "CONNECTION-UNITS-RSC") {
				t.Fatalf("GET %s exposed the RSC payload", tc.path)
			}
		})
	}
}

func TestServeAdminUI_AllowsNextFlightRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/new/connection-units.txt?_rsc=client-navigation", nil)
	req.Header.Set("RSC", "1")
	req.Header.Set("Next-Router-State-Tree", `["",{}]`)
	rec := httptest.NewRecorder()

	Serve(rec, req, fakeFS())

	if rec.Code != http.StatusOK {
		t.Fatalf("RSC request status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "CONNECTION-UNITS-RSC" {
		t.Fatalf("RSC request body = %q, want exported payload", got)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("RSC request Content-Type = %q, want text/plain", got)
	}
	if got := rec.Header().Get("Vary"); got != "RSC" {
		t.Fatalf("RSC request Vary = %q, want RSC", got)
	}
}

func TestResolveAdminUIPath_NextFlatExport(t *testing.T) {
	cases := []struct {
		path   string
		wantIn string // substring expected in served body
		desc   string
	}{
		{"/new", "NEW-PAGE-OK", "flat /new must serve new.html, not root index"},
		{"/new/", "NEW-PAGE-OK", "trailing slash /new/ must also resolve"},
		{"/setup", "SETUP-PAGE", "setup page"},
		{"/login", "LOGIN-PAGE", "login page"},
		{"/new/abuse", "ABUSE-PAGE", "nested sub-route /new/abuse"},
		{"/new/nodes/node-123", "NODE-DETAIL-SHELL", "dynamic node detail route uses placeholder shell"},
		{"/new/connection-units/cu-123", "CONNECTION-UNIT-DETAIL-SHELL", "dynamic connection unit detail route uses placeholder shell"},
		{"/new/geo-assets/asset-123", "GEO-ASSET-DETAIL-SHELL", "dynamic geo asset detail route uses placeholder shell"},
		{"/", "ROOT-REDIRECT", "root serves index.html"},
		{"/nonexistent", "ROOT-REDIRECT", "unknown path falls back to SPA root"},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			body := servePath(t, fakeFS(), c.path)
			if !strings.Contains(body, c.wantIn) {
				t.Fatalf("GET %s: body=%q, want substring %q", c.path, body, c.wantIn)
			}
			if c.path == "/new" && strings.Contains(body, "ROOT-REDIRECT") {
				t.Fatalf("GET /new served root index.html — bootloop regression")
			}
		})
	}
}

// servePath drives the same candidate-resolution logic as handleAdminUI
// against a synthetic fs.FS, without needing the real embed.
func servePath(t *testing.T, fsys fs.FS, urlPath string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, urlPath, nil)
	rec := httptest.NewRecorder()
	Serve(rec, req, fsys)
	return rec.Body.String()
}
