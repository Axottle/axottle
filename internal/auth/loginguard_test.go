package auth

import (
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// TestLoginGuardLockout proves the guard locks a key out once the failure
// threshold is reached and releases it once the lockout window elapses. The
// clock is injected so window expiry is deterministic.
func TestLoginGuardLockout(t *testing.T) {
	g := NewLoginGuard(3, 15*time.Minute, 15*time.Minute)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	g.now = func() time.Time { return now }

	key := "admin|1.2.3.4"
	if g.Locked(key) {
		t.Fatal("key should not be locked initially")
	}

	// Two failures: still under the threshold of three.
	g.Fail(key)
	g.Fail(key)
	if g.Locked(key) {
		t.Fatal("should not be locked after 2/3 failures")
	}

	// Third failure trips the lockout.
	g.Fail(key)
	if !g.Locked(key) {
		t.Fatal("should be locked after reaching the failure threshold")
	}

	// Still locked partway through the window.
	now = base.Add(14 * time.Minute)
	if !g.Locked(key) {
		t.Fatal("should still be locked within the lockout window")
	}

	// Released once the window passes — a legitimate user is never pinned.
	now = base.Add(16 * time.Minute)
	if g.Locked(key) {
		t.Fatal("should be released after the lockout window expires")
	}
}

// TestLoginGuardWindowResetsCounter proves failures in separate sliding
// windows do not accumulate into a lockout.
func TestLoginGuardWindowResetsCounter(t *testing.T) {
	g := NewLoginGuard(3, 15*time.Minute, 15*time.Minute)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	g.now = func() time.Time { return now }

	key := "admin|ip"
	g.Fail(key)
	g.Fail(key)

	// Advance beyond the window; the counter starts fresh.
	now = base.Add(16 * time.Minute)
	g.Fail(key)
	g.Fail(key)
	if g.Locked(key) {
		t.Fatal("failures across separate windows must not accumulate into a lockout")
	}
}

// TestLoginGuardReset proves a successful login (Reset) clears the counter.
func TestLoginGuardReset(t *testing.T) {
	g := NewLoginGuard(3, 15*time.Minute, 15*time.Minute)
	key := "admin|ip"
	g.Fail(key)
	g.Fail(key)
	g.Reset(key)
	g.Fail(key)
	g.Fail(key)
	if g.Locked(key) {
		t.Fatal("Reset should clear the failure counter")
	}
}

// TestLoginGuardNilSafe proves the zero/nil guard is inert rather than panicking.
func TestLoginGuardNilSafe(t *testing.T) {
	var g *LoginGuard
	if g.Locked("x") {
		t.Fatal("nil guard must report not locked")
	}
	g.Fail("x")  // must not panic
	g.Reset("x") // must not panic
}

// TestDummyBcryptHashRunsRealCompare proves the user-not-found login path runs
// a genuine bcrypt comparison: the dummy hash is a well-formed bcrypt hash that
// never matches a real password, so comparing against it costs a full bcrypt
// evaluation (equalising timing with the user-found path).
func TestDummyBcryptHashRunsRealCompare(t *testing.T) {
	h := dummyBcryptHash()
	if len(h) == 0 {
		t.Fatal("dummy hash must not be empty")
	}

	err := bcrypt.CompareHashAndPassword(h, []byte("any-password"))
	if err == nil {
		t.Fatal("dummy hash must never match a real password")
	}
	// A mismatch error (not a malformed-hash error) confirms the hash is valid
	// bcrypt and the compare actually ran.
	if !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		t.Fatalf("dummy hash must be a valid bcrypt hash, got: %v", err)
	}

	// Computed once and stable across calls.
	if string(dummyBcryptHash()) != string(h) {
		t.Fatal("dummy hash must be stable across calls")
	}
}
