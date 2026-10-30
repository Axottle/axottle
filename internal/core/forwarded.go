package core

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func parseTrustedProxies(values []string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			address, addrErr := netip.ParseAddr(value)
			if addrErr != nil {
				return nil, fmt.Errorf("invalid trusted proxy: %q", value)
			}
			prefix = netip.PrefixFrom(address, address.BitLen())
		}
		out = append(out, prefix.Masked())
	}
	return out, nil
}

func (s *Server) trustedProxy(address netip.Addr) bool {
	for _, prefix := range s.trustedProxies {
		if prefix.Contains(address.Unmap()) {
			return true
		}
	}
	return false
}

// proxyRequest accepts forwarding information only from explicitly trusted
// reverse proxies. Walk from the transport peer toward the client and stop at
// the first untrusted hop, ignoring any client-supplied prefixes in the chain.
func (s *Server) proxyRequest(r *http.Request) *http.Request {
	out := r.Clone(r.Context())
	out.URL.Scheme = "http"
	if r.TLS != nil {
		out.URL.Scheme = "https"
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return out
	}
	peer, err := netip.ParseAddr(host)
	if err != nil || !s.trustedProxy(peer) {
		return out
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "https" || proto == "http" {
		out.URL.Scheme = proto
	}
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded == "" {
		forwarded = r.Header.Get("X-Real-IP")
	}
	if forwarded == "" || len(forwarded) > 4096 {
		return out
	}
	parts := strings.Split(forwarded, ",")
	addresses := make([]netip.Addr, len(parts))
	for i, value := range parts {
		address, err := netip.ParseAddr(strings.TrimSpace(value))
		if err != nil || address.Zone() != "" {
			return out
		}
		addresses[i] = address
	}
	for i := len(addresses) - 1; i >= 0; i-- {
		peer = addresses[i]
		if !s.trustedProxy(peer) {
			break
		}
	}
	out.RemoteAddr = net.JoinHostPort(peer.Unmap().String(), "0")
	return out
}
