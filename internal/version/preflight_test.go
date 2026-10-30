package version

import "testing"

// TestPreflightFlag is a smoke test that ldflags -X correctly sets the
// Preflight string at link time. Run with:
//
//	go test -ldflags "-X github.com/axottle/axottle/internal/version.Preflight=true" ./internal/version/
//
// Without the ldflag Preflight is "" (default) and IsPreflight() is false.
func TestPreflightFlag(t *testing.T) {
	// Default (no ldflag): empty, not preflight.
	if Preflight != "" && Preflight != "true" {
		t.Errorf("Preflight = %q, want empty or \"true\"", Preflight)
	}
	if Preflight == "true" && !IsPreflight() {
		t.Error("Preflight==true but IsPreflight() returned false")
	}
	if Preflight == "" && IsPreflight() {
		t.Error("Preflight empty but IsPreflight() returned true")
	}
}
