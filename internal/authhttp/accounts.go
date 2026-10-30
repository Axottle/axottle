package authhttp

import "github.com/axottle/axottle/internal/auth"

// AccountView is shared by the standalone API and module composition. Login
// secrets and provider-specific identities are deliberately absent.
func AccountView(a auth.Admin) map[string]any {
	return map[string]any{
		"id": a.ID, "username": a.Username, "display_name": a.DisplayName,
		"role": a.Role, "status": a.Status, "is_bootstrap_superuser": a.IsBootstrapSuperuser,
		"permissions": a.Permissions, "has_password": a.HasPassword,
		"created_at": a.CreatedAt, "last_login_at": a.LastLoginAt,
	}
}
