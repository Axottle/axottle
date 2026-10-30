package enginebridge

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Proxy keeps the original Host, URL, cookies and streaming responses. Only the
// trusted core constructs forwarding headers; the engine performs all of its
// existing session, project and module permission checks.
func (c *Client) Proxy() http.Handler {
	target := &url.URL{Scheme: "http", Host: "engine"}
	return &httputil.ReverseProxy{
		Transport:     c.transport,
		FlushInterval: -1,
		Rewrite: func(p *httputil.ProxyRequest) {
			p.SetURL(target)
			p.Out.Host = p.In.Host
			p.Out.Header.Del("X-Real-IP")
			p.Out.Header.Del("Forwarded")
			p.SetXForwarded()
			if scheme := p.In.URL.Scheme; scheme == "http" || scheme == "https" {
				p.Out.Header.Set("X-Forwarded-Proto", scheme)
			}
			p.Out.Header.Set(PeerHeader, p.In.RemoteAddr)
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "Axottle engine is unavailable", http.StatusServiceUnavailable)
		},
	}
}
