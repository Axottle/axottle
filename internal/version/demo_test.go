//go:build demo

package version

import "testing"

func TestDemoCannotEnterReleaseUpdateChannel(t *testing.T) {
	previous := Version
	t.Cleanup(func() { Version = previous })
	for _, label := range []string{"demo-20260911", "v1.2.3", "v1.2.3-preflight.1", "dev"} {
		Version = label
		if IsStamped() {
			t.Fatalf("demo must not enter release updates even with version %q", label)
		}
	}
}
