package auth

import (
	"errors"
	"testing"
)

func TestStoredPermissionsFailClosed(t *testing.T) {
	for _, raw := range []string{"", "null", "[]", "[", `{"*":true}`, `["unknown"]`} {
		permissions, err := decodePermissions([]byte(raw))
		if err == nil && (&Admin{Permissions: permissions}).Can(PermSettingsManage) {
			t.Fatalf("stored grants failed open: %q", raw)
		}
	}
}

func TestDelegationAndGlobalAuthority(t *testing.T) {
	manager := &Admin{Permissions: []string{"accounts.manage"}}
	for _, permissions := range [][]string{{"*"}, {"global.admin"}, {"settings.manage"}} {
		if !errors.Is(ValidateDelegation(manager, permissions), ErrPermissionDelegation) {
			t.Fatalf("manager granted %v", permissions)
		}
	}
	for _, permissions := range [][]string{nil, {}, {"accounts.manage"}} {
		if err := ValidateDelegation(manager, permissions); err != nil {
			t.Fatal(err)
		}
	}
	scoped := &Admin{Permissions: []string{"*"}}
	if scoped.Can(PermSettingsManage) || scoped.Can(PermAuditView) || scoped.Can(PermBackupsRestore) || scoped.Can(Permission("future.operation")) || scoped.FullAccess() {
		t.Fatal("project wildcard acquired global/future authority")
	}
	if !scoped.Can(PermNodesView) {
		t.Fatal("project wildcard lost project access")
	}
	global := &Admin{Permissions: []string{PermissionGlobalAdmin}}
	if !global.Can(PermSettingsManage) || !global.FullAccess() {
		t.Fatal("explicit global grant ignored")
	}
	if err := ValidateDelegation(global, []string{PermissionGlobalAdmin}); err == nil {
		t.Fatal("only installation owner may appoint global administrators")
	}
}

func TestSensitiveAttemptReservation(t *testing.T) {
	g := NewLoginGuard(2, 0, 0)
	if !g.Allow("account") || !g.Allow("account") || g.Allow("account") {
		t.Fatal("attempt quota bypassed")
	}
}
