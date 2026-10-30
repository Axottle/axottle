package auth

import (
	"errors"
	"reflect"
	"testing"
)

func TestNormalizePermissions(t *testing.T) {
	cases := []struct {
		name    string
		in      []string
		want    []string
		wantErr error
	}{
		{"nil means full access", nil, []string{PermissionAll}, nil},
		{"empty means full access", []string{}, []string{PermissionAll}, nil},
		{"wildcard collapses everything", []string{"nodes.view", "*", "cdn.view"}, []string{PermissionAll}, nil},
		{"valid list sorted and deduped", []string{"nodes.view", "nodes.view", "cdn.view"}, []string{"cdn.view", "nodes.view"}, nil},
		{"blank entries skipped", []string{" nodes.view ", ""}, []string{"nodes.view"}, nil},
		{"unknown rejected", []string{"nodes.view", "bogus.perm"}, nil, ErrUnknownPermission},
		{"only blanks rejected", []string{"", "  "}, nil, ErrNoPermissions},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizePermissions(tc.in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("want error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, got)
			}
		})
	}
}

func TestAdminCan(t *testing.T) {
	full := &Admin{Permissions: []string{PermissionAll}}
	if !full.Can(PermNodesDelete) || !full.FullAccess() {
		t.Fatal("wildcard admin must pass every check")
	}

	limited := &Admin{Permissions: []string{"nodes.view", "cdn.manage"}}
	if !limited.Can(PermNodesView) {
		t.Fatal("granted permission must pass")
	}
	if limited.Can(PermNodesDelete) {
		t.Fatal("missing permission must fail")
	}
	if limited.FullAccess() {
		t.Fatal("limited admin must not report full access")
	}

	var nilAdmin *Admin
	if nilAdmin.Can(PermNodesView) || nilAdmin.FullAccess() {
		t.Fatal("nil admin can do nothing")
	}
}

// Every permission the registry exposes must survive normalization — the UI
// round-trips exactly these strings.
func TestRegistryRoundTrip(t *testing.T) {
	for _, g := range PermissionRegistry() {
		for _, p := range g.Permissions {
			got, err := NormalizePermissions([]string{string(p)})
			if err != nil {
				t.Fatalf("registry permission %q rejected: %v", p, err)
			}
			if len(got) != 1 || got[0] != string(p) {
				t.Fatalf("registry permission %q normalized to %v", p, got)
			}
		}
	}
}
