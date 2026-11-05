package authhttp

import "net/http"

// SecurityHeaders applies to the public entry point, including static UI and
// error responses. Next's static export uses inline hydration scripts; keep
// that allowance explicit until the exporter can supply per-page CSP hashes.
func SecurityHeaders(w http.ResponseWriter, secure bool) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; font-src 'self' data:; connect-src 'self' https: wss:; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
	if secure {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
	}
}
