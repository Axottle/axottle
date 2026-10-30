// Package version exposes the single build-time version label for the Axottle
// control plane binary (cmd/control).
//
// Version is injected at build time via -ldflags:
//
//	go build -ldflags="-X github.com/axottle/axottle/internal/version.Version=v1.2.3" \
//	    -o axottle ./cmd/control
//
// When unset (e.g. `go run` / dev builds) it stays "dev", which is also the
// signal that this build was never stamped by the release packager — the
// panel-update subsystem treats "dev" as "unknown, never report an update".
package version

import (
	"strings"
	"time"
)

// Version is the panel build label. Overridden at link time by the release
// packager (scripts/release/*) and the Makefile build target.
var Version = "dev"

// Release is the logical release identity of a component, in the form
// "<component>@<version>" (e.g. "axottle@v1.0.1-beta"). It groups every
// physical build (dist) of the same source version and is the primary key the
// deobfuscation worker uses, together with Dist, to locate the debug bundle.
// Stamped via ldflags by the release pipeline; "axottle@dev" for dev builds.
var Release = "axottle@dev"

// Dist is the unique physical-build identifier, e.g.
// "linux-amd64-a17fc20-b7d8e1" (goos-goarch-shortcommit-buildid). Every
// physical build — including a rebuild of the same version, or a different
// architecture — gets a distinct Dist. The seed is per-Dist. Never expose the
// seed; Dist is the public handle that maps to it inside the debug bundle
// store. "dev" for unstamped builds.
var Dist = "dev"

// Commit is the full git SHA the build was produced from. "unknown" for dev
// builds. Included in crash events so a decoded trace can be tied back to exact
// source even after tags move.
var Commit = "unknown"

// EngineRequired is set only on the core binary in a paired distribution.
// A standalone source build can run independently; an installed pair must not
// report a successful restart if its engine executable has disappeared.
var EngineRequired string

// Preflight enables diagnostic tooling in a packaged build. It never changes
// release trust verification. Stored as a string for ldflags -X.
var Preflight string

// PreflightExpiry limits the lifetime of diagnostic tooling.
// Empty permits diagnostics indefinitely in a locally built binary.
var PreflightExpiry string

// IsPreflight reports whether diagnostic tooling is enabled and unexpired.
func IsPreflight() bool {
	if Preflight != "true" && Preflight != "1" {
		return false
	}
	return !preflightExpired()
}

// preflightExpired reports whether the stamped expiry has passed. An
// unparseable stamp is treated as EXPIRED: a malformed date is far more likely
// to be a tampered binary than a packaging slip, and the failure mode of
// guessing wrong is "diagnostics stay on forever".
func preflightExpired() bool {
	stamp := strings.TrimSpace(PreflightExpiry)
	if stamp == "" {
		return false
	}
	until, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		return true
	}
	return time.Now().After(until)
}

// Current returns the version label. Centralized here so callers don't reach
// into the package var directly and so we have one place to add normalization
// (e.g. trimming a leading "v") if we ever need it.
func Current() string { return Version }

// CurrentRelease returns the logical release identity ("<component>@<version>").
// Used to tag crash events and to select the debug bundle for symbolication.
func CurrentRelease() string { return Release }

// CurrentDist returns the unique physical-build identifier. This, with
// CurrentRelease, is how the deobfuscation worker selects the correct seed and
// source snapshot. The seed itself is never derivable from Dist client-side.
func CurrentDist() string { return Dist }

// CurrentCommit returns the full git SHA this build was produced from, or
// "unknown" for dev builds.
func CurrentCommit() string { return Commit }

// IsStamped reports whether this binary was produced by the release pipeline
// (a real semver-ish label) rather than a dev build. The panel-update
// subsystem refuses to compare a "dev" build against upstream because the
// comparison is meaningless. Standalone demos also stay outside release updates.
func IsStamped() bool { return !Demo && Version != "" && Version != "dev" }
