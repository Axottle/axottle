package auth

import (
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"empty", "", ErrPasswordTooShort},
		{"too short 7", "1234567", ErrPasswordTooShort},
		{"exactly 8", "12345678", nil},
		{"typical", "correct horse battery", nil},
		{"exactly 72 bytes", strings.Repeat("a", 72), nil},
		{"73 bytes rejected", strings.Repeat("a", 73), ErrPasswordTooLong},
		{"multibyte within limit", strings.Repeat("é", 8), nil}, // 16 bytes
		{"multibyte over byte limit", strings.Repeat("é", 40), ErrPasswordTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("ValidatePassword(%q) = %v, want nil", tt.password, err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("ValidatePassword(%q) = %v, want %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

// A 7-byte multibyte-boundary password must still be rejected as too short so
// the byte floor never accidentally admits a sub-8-byte credential.
func TestValidatePasswordShortMultibyte(t *testing.T) {
	if err := ValidatePassword("éé"); err != ErrPasswordTooShort { // 4 bytes
		t.Fatalf("got %v, want ErrPasswordTooShort", err)
	}
}
