package audit

import (
	"context"
	"testing"
)

func changeByField(changes []Change, field string) (Change, bool) {
	for _, c := range changes {
		if c.Field == field {
			return c, true
		}
	}
	return Change{}, false
}

func TestDiff_ChangedAddedRemovedAndHidden(t *testing.T) {
	before := map[string]any{
		"name":       "Frankfurt-01",
		"ip":         "10.0.0.1",
		"enabled":    true,
		"removed":    "gone",
		"updated_at": "2026-07-15T00:00:00Z",
	}
	after := map[string]any{
		"name":       "Frankfurt-02",         // changed
		"ip":         "10.0.0.1",             // unchanged -> omitted
		"enabled":    false,                  // changed
		"added":      "new",                  // added
		"updated_at": "2026-07-15T01:00:00Z", // hidden via label ""
	}
	labels := map[string]string{
		"name":       "Имя",
		"enabled":    "Включена",
		"updated_at": "", // hidden
	}

	changes := Diff(before, after, labels)

	if _, ok := changeByField(changes, "ip"); ok {
		t.Errorf("unchanged field ip should be omitted")
	}
	if _, ok := changeByField(changes, "updated_at"); ok {
		t.Errorf("hidden field updated_at should be omitted")
	}
	name, ok := changeByField(changes, "name")
	if !ok {
		t.Fatalf("expected name change")
	}
	if name.Label != "Имя" || name.Before != "Frankfurt-01" || name.After != "Frankfurt-02" {
		t.Errorf("unexpected name change: %+v", name)
	}
	added, ok := changeByField(changes, "added")
	if !ok {
		t.Fatalf("expected added field")
	}
	if added.Before != nil || added.After != "new" {
		t.Errorf("added field should have nil before: %+v", added)
	}
	removed, ok := changeByField(changes, "removed")
	if !ok {
		t.Fatalf("expected removed field")
	}
	if removed.Before != "gone" || removed.After != nil {
		t.Errorf("removed field should have nil after: %+v", removed)
	}
	// Field without an explicit label falls back to the raw key.
	if removed.Label != "removed" {
		t.Errorf("expected fallback label 'removed', got %q", removed.Label)
	}
}

func TestDiff_NilSides(t *testing.T) {
	// Creation: before nil -> every after field is an addition.
	created := Diff(nil, map[string]any{"name": "n"}, nil)
	if len(created) != 1 || created[0].Before != nil || created[0].After != "n" {
		t.Errorf("create diff unexpected: %+v", created)
	}
	// Both nil -> no changes.
	if got := Diff(nil, nil, nil); len(got) != 0 {
		t.Errorf("expected no changes for nil/nil, got %+v", got)
	}
}

func TestDiff_NumbersNormalizeAcrossTypes(t *testing.T) {
	type snap struct {
		Port int `json:"port"`
	}
	// int(22) vs a JSON map float64(22) must be treated as equal after the
	// JSON round-trip.
	if got := Diff(snap{Port: 22}, map[string]any{"port": 22.0}, nil); len(got) != 0 {
		t.Errorf("expected numeric equality, got %+v", got)
	}
}

func TestCategoryForActorType(t *testing.T) {
	if CategoryForActorType(ActorSystem) != CategorySystem {
		t.Errorf("system actor should map to system category")
	}
	if CategoryForActorType(ActorAdmin) != CategoryUser {
		t.Errorf("admin actor should map to user category")
	}
	if CategoryForActorType("") != CategoryUser {
		t.Errorf("unknown/empty actor should default to user category")
	}
}

func TestResolveActor_NoAdminIsSystem(t *testing.T) {
	at, aid := ResolveActor(context.Background())
	if at != ActorSystem || aid != ActorSystem {
		t.Errorf("expected system/system without an admin, got %s/%s", at, aid)
	}
}
