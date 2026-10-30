// Package staticui serves the shared static Next.js UI contract.
package staticui

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

// Serve serves the files produced by Next.js' static export.
//
// App Router emits a text RSC payload next to every HTML page and fetches it
// with the RSC request header during client-side navigation. Those files are
// an implementation detail, but a plain browser request to <route>.txt used to
// expose the payload as a public text page. Keep the files available to the
// Next.js runtime while making every ordinary .txt navigation fail closed.
func Serve(w http.ResponseWriter, r *http.Request, static fs.FS) {
	if strings.EqualFold(path.Ext(r.URL.Path), ".txt") {
		w.Header().Add("Vary", "RSC")
		if r.Header.Get("RSC") != "1" {
			// Do not let a shared cache reuse this 404 for a later RSC fetch even
			// if it does not implement Vary correctly.
			w.Header().Set("Cache-Control", "no-store")
			http.NotFound(w, r)
			return
		}
		serveFlight(w, r, static)
		return
	}

	rawPath := strings.TrimPrefix(r.URL.Path, "/")

	// Candidate files to try in order. Next.js static export emits flat
	// "<route>.html" files (e.g. new.html, setup.html) alongside nested
	// "<route>/<sub>.html" directories — NOT "<route>/index.html". Without
	// the ".html" candidate below, a path like /new resolves the "new"
	// directory (skipped as IsDir), then "new/index.html" (missing), and
	// finally falls through to the root index.html — which redirects back
	// to /new, causing an infinite reload loop.
	candidates := []string{rawPath}
	if trimmed := strings.TrimSuffix(rawPath, "/"); trimmed != "" && !strings.HasSuffix(trimmed, ".html") {
		candidates = append(candidates, trimmed+".html")
	}
	if rawPath == "" || strings.HasSuffix(rawPath, "/") {
		candidates = append(candidates, rawPath+"index.html")
	} else {
		// Also try with /index.html (e.g. "setup" → "setup/index.html")
		candidates = append(candidates, rawPath+"/index.html")
	}
	// Dynamic routes in the static-exported admin UI are emitted once as a
	// placeholder shell, e.g. new/nodes/placeholder.html for /new/nodes/[id].
	// When the browser requests /new/nodes/<real-id> directly, serve the
	// parent placeholder shell and let the client router read the real URL.
	if parent := path.Dir(rawPath); parent != "." && parent != "/" {
		candidates = append(candidates, parent+"/placeholder.html")
	}
	// Always fall back to root index.html (SPA)
	candidates = append(candidates, "index.html")
	// Missing assets must be 404s, never a cacheable HTML redirect.
	if isAssetPath(rawPath) {
		candidates = []string{rawPath}
	}

	for _, p := range candidates {
		f, err := static.Open(p)
		if err != nil {
			continue
		}
		stat, err := f.Stat()
		if err != nil || stat.IsDir() {
			f.Close()
			continue
		}
		defer f.Close()

		ct := mime.TypeByExtension(path.Ext(p))
		if ct == "" {
			ct = "application/octet-stream"
		}
		body, err := io.ReadAll(f)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", ct)
		// Only content-versioned URLs are immutable across releases. HTML and
		// unversioned public files must validate before reuse.
		cacheControl := "no-cache"
		if strings.HasPrefix(p, "_assets/") || strings.HasPrefix(p, "_next/static/") {
			cacheControl = "public, max-age=31536000, immutable"
		}
		w.Header().Set("Cache-Control", cacheControl)
		w.Header().Set("ETag", fmt.Sprintf(`"%x"`, sha256.Sum256(body)))
		http.ServeContent(w, r, p, time.Time{}, bytes.NewReader(body))
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, "not found", http.StatusNotFound)
}

func isAssetPath(name string) bool {
	for _, prefix := range []string{"_assets/", "_next/", "assets/", "mascots/"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".avif", ".gif", ".svg", ".ico", ".js", ".css", ".woff", ".woff2", ".map":
		return true
	}
	return false
}

// Flight requests must never use the HTML/SPA fallback: Next treats an HTML
// response as a full-document navigation, remounting auth and workspaces.
// Dynamic routes have the same placeholder export for both HTML and Flight.
func serveFlight(w http.ResponseWriter, r *http.Request, static fs.FS) {
	w.Header().Set("Cache-Control", "no-store")
	rawPath := strings.TrimPrefix(r.URL.Path, "/")
	if !fs.ValidPath(rawPath) {
		http.NotFound(w, r)
		return
	}
	route := strings.TrimSuffix(rawPath, path.Ext(rawPath))
	if strings.HasSuffix(rawPath, "/index.txt") {
		route = strings.TrimSuffix(rawPath, "/index.txt")
	}
	type candidate struct{ file, routeID string }
	candidates := []candidate{{file: rawPath}, {file: route + ".txt"}}
	if parent := path.Dir(route); parent != "." && parent != "/" {
		candidates = append(candidates, candidate{parent + "/placeholder.txt", path.Base(route)})
	}
	for _, candidate := range candidates {
		body, err := fs.ReadFile(static, candidate.file)
		if err != nil {
			continue
		}
		if candidate.routeID != "" {
			// The export's canonical path, dynamic segment and params contain
			// this JSON string. Keep their identity consistent with the URL so
			// navigating between two resources does not reuse placeholder state.
			routeID := candidate.routeID
			// Flight reserves a leading '$' for references; '$$' is a literal.
			if strings.HasPrefix(routeID, "$") {
				routeID = "$" + routeID
			}
			id, _ := json.Marshal(routeID)
			body = bytes.ReplaceAll(body, []byte(`"placeholder"`), id)
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.NotFound(w, r)
}
