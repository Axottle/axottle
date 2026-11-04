package authhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNativeSessionKeepsGatewayAuthorizationSeparate(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("POST", "https://panel.example/api/v1/auth/logout", nil)
	r.Header.Set("Authorization", "Bearer gateway")
	r.Header.Set(NativeSessionHeader, "native")
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "browser"})
	token, cookie := h.RequestToken(r)
	if token != "native" || cookie {
		t.Fatal("gateway shadowed native session")
	}
	r.Header.Set(NativeSessionHeader, "")
	token, cookie = h.RequestToken(r)
	if token != "" || cookie {
		t.Fatal("empty explicit credential fell back")
	}
	r.Header.Del(NativeSessionHeader)
	token, cookie = h.RequestToken(r)
	if token != "gateway" || cookie {
		t.Fatal("legacy bearer precedence changed")
	}
}

func TestOriginIncludesScheme(t *testing.T) {
	h := &Handler{PublicURL: "https://panel.example"}
	for _, tc := range []struct {
		origin string
		want   bool
	}{{"https://panel.example", true}, {"http://panel.example", false}, {"https://evil.example", false}, {"null", false}, {"https://user@panel.example", false}, {"", false}} {
		r := httptest.NewRequest("POST", "http://internal/api", nil)
		r.Header.Set("Origin", tc.origin)
		if got := h.SameOriginRequest(r); got != tc.want {
			t.Errorf("origin %q=%v", tc.origin, got)
		}
		if got := h.OriginAllowsCredentials(tc.origin, r); got != tc.want {
			t.Errorf("CORS origin %q=%v", tc.origin, got)
		}
	}
}
