package auth

import (
	"sync"
	"time"
)

// Login lockout defaults: after DefaultMaxLoginFailures failed attempts within
// DefaultLoginWindow, a key is locked out for DefaultLoginLockout.
const (
	DefaultMaxLoginFailures = 10
	DefaultLoginWindow      = 15 * time.Minute
	DefaultLoginLockout     = 15 * time.Minute
)

// LoginGuard throttles brute-force login attempts. It tracks failures per
// opaque key (e.g. username+IP) in a sliding window and locks a key out once
// the failure threshold is reached. It is in-memory and mutex-guarded: state
// resets on process restart, which is acceptable for a throttle. All methods
// are safe to call on a nil *LoginGuard (they no-op / report "not locked").
type LoginGuard struct {
	mu      sync.Mutex
	m       map[string]*loginAttempts
	max     int
	window  time.Duration
	lockout time.Duration
	now     func() time.Time // overridable in tests; defaults to time.Now
}

type loginAttempts struct {
	count       int
	windowStart time.Time
	lockedUntil time.Time
}

// NewLoginGuard builds a guard. Non-positive parameters fall back to the
// package defaults.
func NewLoginGuard(max int, window, lockout time.Duration) *LoginGuard {
	if max <= 0 {
		max = DefaultMaxLoginFailures
	}
	if window <= 0 {
		window = DefaultLoginWindow
	}
	if lockout <= 0 {
		lockout = DefaultLoginLockout
	}
	return &LoginGuard{
		m:       map[string]*loginAttempts{},
		max:     max,
		window:  window,
		lockout: lockout,
		now:     time.Now,
	}
}

// Locked reports whether key is currently locked out.
func (g *LoginGuard) Locked(key string) bool {
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	w := g.m[key]
	return w != nil && g.now().Before(w.lockedUntil)
}

// Fail records a failed attempt for key and locks it out once the failure
// count reaches the threshold within the window.
func (g *LoginGuard) Fail(key string) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	g.gcLocked(now)
	w := g.m[key]
	if w == nil || now.Sub(w.windowStart) > g.window {
		w = &loginAttempts{windowStart: now}
		g.m[key] = w
	}
	w.count++
	if w.count >= g.max {
		w.lockedUntil = now.Add(g.lockout)
		// Start a fresh window after the lockout so the key isn't permanently
		// pinned: once lockedUntil passes, a legitimate user can retry.
		w.count = 0
		w.windowStart = now
	}
}

// Reset clears a key's recorded failures; call after a successful login.
func (g *LoginGuard) Reset(key string) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.m, key)
}

// gcLocked drops cold entries to bound memory. Caller must hold the lock.
func (g *LoginGuard) gcLocked(now time.Time) {
	if len(g.m) < 50_000 {
		return
	}
	for k, w := range g.m {
		if now.After(w.lockedUntil) && now.Sub(w.windowStart) > g.window {
			delete(g.m, k)
		}
	}
}
