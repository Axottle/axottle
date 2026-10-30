package core

import (
	"net/http/httptest"
	"testing"
)

func TestForwardedHeadersRequireTrustedTransportPeer(t *testing.T) {
	for _, tc := range []struct {
		name, remote, forwarded, proto, wantPeer, wantProto string
	}{
		{"direct", "192.0.2.1:321", "127.0.0.1", "https", "192.0.2.1:321", "http"},
		{"local reverse proxy", "127.0.0.1:80", "192.0.2.1", "https", "192.0.2.1:0", "https"},
		{"spoofed prefix", "127.0.0.1:80", "127.0.0.1, 192.0.2.1", "https", "192.0.2.1:0", "https"},
		{"trusted proxy chain", "127.0.0.1:80", "192.0.2.1, 10.0.0.2", "https", "192.0.2.1:0", "https"},
		{"untrusted proxy chain", "127.0.0.1:80", "192.0.2.1, 198.51.100.1", "https", "198.51.100.1:0", "https"},
		{"invalid chain", "127.0.0.1:80", "not-an-ip, 192.0.2.1", "https", "127.0.0.1:80", "https"},
		{"invalid scheme", "127.0.0.1:80", "192.0.2.1", "https,http", "192.0.2.1:0", "http"},
		{"IPv6 client", "[::1]:80", "2001:db8::1", "https", "[2001:db8::1]:0", "https"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := New(Config{Version: "v1", TrustedProxies: []string{"127.0.0.1/32", "::1/128", "10.0.0.0/8"}})
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("GET", "/api/v1/nodes", nil)
			r.RemoteAddr = tc.remote
			r.Header.Set("X-Forwarded-For", tc.forwarded)
			r.Header.Set("X-Forwarded-Proto", tc.proto)
			out := s.proxyRequest(r)
			if out.RemoteAddr != tc.wantPeer || out.URL.Scheme != tc.wantProto {
				t.Fatalf("got %s %s, want %s %s", out.RemoteAddr, out.URL.Scheme, tc.wantPeer, tc.wantProto)
			}
			if r.RemoteAddr != tc.remote {
				t.Fatal("mutated inbound request")
			}
		})
	}
	if _, err := New(Config{Version: "v1", TrustedProxies: []string{"*"}}); err == nil {
		t.Fatal("invalid proxy wildcard accepted")
	}
}
