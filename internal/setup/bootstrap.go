package setup

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const BootstrapTokenHeader = "X-Axottle-Setup-Token"

// ConfigureBootstrap keeps first-run authority on the installation host. The
// token is never returned by the public status endpoint or written to logs.
func (s *Service) ConfigureBootstrap(dataDir string) error {
	if dataDir == "" {
		return fmt.Errorf("bootstrap requires a data directory")
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return err
	}
	name := filepath.Join(dataDir, "bootstrap.token")
	f, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		var token [32]byte
		if _, err = rand.Read(token[:]); err == nil {
			_, err = f.WriteString(hex.EncodeToString(token[:]) + "\n")
		}
		closeErr := f.Close()
		if err != nil {
			_ = os.Remove(name)
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	} else if !os.IsExist(err) {
		return err
	}
	info, err := os.Lstat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("bootstrap token must be a private regular file")
	}
	raw, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	token, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(token) != 32 {
		return fmt.Errorf("invalid bootstrap token file")
	}
	s.bootstrapToken = strings.TrimSpace(string(raw))
	return nil
}

func (s *Service) AuthorizeBootstrap(r *http.Request) bool {
	return s.bootstrapToken != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get(BootstrapTokenHeader)), []byte(s.bootstrapToken)) == 1
}
