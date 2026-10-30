package auth

import (
	"errors"
	"unicode/utf8"
)

// Password policy. Kept deliberately small: the panel is an operator tool with
// a handful of admin accounts, not a public sign-up surface, so we enforce a
// sane floor without frustrating rules that push operators toward weaker,
// written-down passwords.
const (
	// MinPasswordLen mirrors the floor enforced by the setup wizard
	// (handlers_setup.go) so the bar to change a password or create an
	// account never drops below the bar to create the very first admin.
	MinPasswordLen = 8

	// MaxPasswordLen bounds the input at bcrypt's hard limit. bcrypt silently
	// ignores everything past the 72nd byte, so a longer password would give
	// the operator a false sense of strength and, worse, let a truncated
	// prefix authenticate. Reject it explicitly instead of truncating.
	MaxPasswordLen = 72
)

var (
	// ErrPasswordTooShort / ErrPasswordTooLong carry operator-facing prose;
	// handlers surface err.Error() directly.
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong  = errors.New("password must be at most 72 bytes")
)

// ValidatePassword reports whether a candidate password satisfies the policy.
// Length is measured in bytes for the upper bound (bcrypt's limit is a byte
// limit) and in bytes for the lower bound as well; an 8-byte floor is a
// conservative choice for multi-byte scripts and never rejects an 8-character
// ASCII password.
func ValidatePassword(password string) error {
	// Byte length, not rune count: bcrypt truncates on bytes.
	if len(password) < MinPasswordLen {
		return ErrPasswordTooShort
	}
	if len(password) > MaxPasswordLen {
		return ErrPasswordTooLong
	}
	// Guard against invalid UTF-8 sneaking into a stored credential; every
	// legitimate client sends UTF-8 JSON.
	if !utf8.ValidString(password) {
		return errors.New("password contains invalid characters")
	}
	return nil
}
