//go:build demo && (dev || unpinned)

package version

// Fail compilation rather than silently mixing standalone demo and dev/preflight.
var _ = demo_build_cannot_be_combined_with_dev_or_unpinned
