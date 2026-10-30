package audit

import (
	"encoding/json"
	"reflect"
	"sort"
)

// Change is one field-level difference between the before and after state of an
// audited object. Label is the human-readable field name (Russian in the
// panel); Field is the raw JSON key it came from.
type Change struct {
	Field  string `json:"field"`
	Label  string `json:"label"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

// Diff computes the field-level difference between before and after. Each side
// is marshaled to JSON and compared key-by-key at the top level, so pass flat
// snapshot structs/maps (a nested object is compared as one whole value).
//
// labels maps a JSON field name to its human-readable label; a missing entry
// falls back to the raw field name. A field whose name maps to "" is skipped —
// use that to hide noisy/internal columns (updated_at, revision, …). Either
// side may be nil (creation: before=nil; deletion: after=nil), in which case
// every differing field shows a nil on that side.
func Diff(before, after any, labels map[string]string) []Change {
	bm := toMap(before)
	am := toMap(after)

	seen := map[string]struct{}{}
	keys := make([]string, 0, len(bm)+len(am))
	for k := range bm {
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			keys = append(keys, k)
		}
	}
	for k := range am {
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	changes := make([]Change, 0, len(keys))
	for _, k := range keys {
		label, mapped := labels[k]
		if mapped && label == "" {
			continue // explicitly hidden
		}
		if label == "" {
			label = k
		}
		bv := bm[k]
		av := am[k]
		if reflect.DeepEqual(bv, av) {
			continue
		}
		changes = append(changes, Change{Field: k, Label: label, Before: bv, After: av})
	}
	return changes
}

// toMap renders v as a JSON object map. Returns an empty map for nil or for a
// value that does not marshal to a JSON object. The round-trip normalizes types
// (numbers become float64, etc.) so reflect.DeepEqual compares like-for-like.
func toMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return map[string]any{}
	}
	m := map[string]any{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return map[string]any{}
	}
	return m
}
