package staticui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestAdminAssetCaching(t *testing.T) {
	files := fstest.MapFS{
		"index.html":                           {Data: []byte("page")},
		"logo.png":                             {Data: []byte("original")},
		"_assets/mascot.0123456789abcdef.webp": {Data: []byte("optimized")},
		"_next/static/chunks/app-012345.js":    {Data: []byte("javascript")},
	}
	for _, name := range []string{"/", "/logo.png", "/_assets/mascot.0123456789abcdef.webp", "/_next/static/chunks/app-012345.js"} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("GET", name, nil)
			w := httptest.NewRecorder()
			Serve(w, r, files)
			want := "no-cache"
			if strings.HasPrefix(name, "/_") {
				want = "public, max-age=31536000, immutable"
			}
			if w.Code != 200 || w.Header().Get("Cache-Control") != want || w.Header().Get("ETag") == "" {
				t.Fatalf("initial response: %d %v", w.Code, w.Header())
			}
			for _, etag := range []string{w.Header().Get("ETag"), "W/" + w.Header().Get("ETag")} {
				r.Header.Set("If-None-Match", etag)
				cached := httptest.NewRecorder()
				Serve(cached, r, files)
				if cached.Code != http.StatusNotModified || cached.Body.Len() != 0 {
					t.Fatalf("conditional request: %d %s", cached.Code, cached.Body.String())
				}
			}
			head := httptest.NewRecorder()
			Serve(head, httptest.NewRequest("HEAD", name, nil), files)
			if head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("Content-Length") == "" {
				t.Fatalf("HEAD: %d %v", head.Code, head.Header())
			}
		})
	}
	// An unversioned file must invalidate after an update under the same URL.
	first := httptest.NewRecorder()
	Serve(first, httptest.NewRequest("GET", "/logo.png", nil), files)
	files["logo.png"].Data = []byte("updated")
	req := httptest.NewRequest("GET", "/logo.png", nil)
	req.Header.Set("If-None-Match", first.Header().Get("ETag"))
	updated := httptest.NewRecorder()
	Serve(updated, req, files)
	if updated.Code != 200 || updated.Body.String() != "updated" {
		t.Fatal("updated asset was stale")
	}
}

func TestAdminMissingAssetsNeverReturnHTML(t *testing.T) {
	for _, url := range []string{"/logo-missing.png", "/_assets/missing.webp", "/_next/static/missing.js", "/assets/missing", "/mascots/missing.png"} {
		w := httptest.NewRecorder()
		Serve(w, httptest.NewRequest("GET", url, nil), fakeFS())
		if w.Code != 404 || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "ROOT-REDIRECT") {
			t.Fatalf("%s: %d %s", url, w.Code, w.Body.String())
		}
	}
}
