package setup

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapRequiresPrivateHostToken(t *testing.T) {
	s := &Service{}
	dir := t.TempDir()
	if err := s.ConfigureBootstrap(dir); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "http://panel/api/v1/setup/admin", nil)
	for _, token := range []string{"", "guess"} {
		r.Header.Set(BootstrapTokenHeader, token)
		if s.AuthorizeBootstrap(r) {
			t.Fatal("public bootstrap accepted")
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "bootstrap.token"))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set(BootstrapTokenHeader, strings.TrimSpace(string(raw)))
	if !s.AuthorizeBootstrap(r) {
		t.Fatal("owner token refused")
	}
	restarted := &Service{}
	if err := restarted.ConfigureBootstrap(dir); err != nil {
		t.Fatal(err)
	}
	if !restarted.AuthorizeBootstrap(r) {
		t.Fatal("token changed on restart")
	}
	if err := os.Chmod(filepath.Join(dir, "bootstrap.token"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := restarted.ConfigureBootstrap(dir); err == nil {
		t.Fatal("publicly readable token accepted")
	}
}
