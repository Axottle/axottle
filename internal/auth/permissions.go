package auth

import (
	"fmt"
	"sort"
	"strings"
)

// Permission identifies an explicit panel capability.
type Permission string

// PermissionAll grants known project capabilities. It cannot authorize global
// settings, backups, audit or future capabilities. Installation-wide authority
// requires the separate global.admin grant or the protected bootstrap account.
const PermissionAll = "*"
const PermissionGlobalAdmin = "global.admin"

const (
	// Nodes — the node fleet, agents, install recipes, maintenance.
	PermNodesView    Permission = "nodes.view"
	PermNodesCreate  Permission = "nodes.create"
	PermNodesEdit    Permission = "nodes.edit"
	PermNodesDelete  Permission = "nodes.delete"
	PermNodesOperate Permission = "nodes.operate"

	// Connection Units (Flow Builder).
	PermUnitsView   Permission = "connection_units.view"
	PermUnitsCreate Permission = "connection_units.create"
	PermUnitsEdit   Permission = "connection_units.edit"
	PermUnitsDelete Permission = "connection_units.delete"
	PermUnitsApply  Permission = "connection_units.apply"

	// Remnawave integration: connections, upstream users/hosts, subscription
	// contexts and app parameter sets (subscription shaping).
	PermRemnawaveView   Permission = "remnawave.view"
	PermRemnawaveManage Permission = "remnawave.manage"

	// Customer accounts and Internal Squads in the selected project.
	PermSubscribersView   Permission = "subscribers.view"
	PermSubscribersManage Permission = "subscribers.manage"
	PermGroupsView        Permission = "subscriber_groups.view"
	PermGroupsManage      Permission = "subscriber_groups.manage"

	// GeoAssets library + per-node assignment/sync.
	PermGeoAssetsView   Permission = "geo_assets.view"
	PermGeoAssetsManage Permission = "geo_assets.manage"

	// Health Center: dashboards, probes, diagnostics, health settings.
	PermHealthView   Permission = "health.view"
	PermHealthManage Permission = "health.manage"

	// Anti-abuse: incidents, signals, QoS overrides, abuse policy.
	PermAbuseView   Permission = "abuse.view"
	PermAbuseManage Permission = "abuse.manage"

	// Security: detector status, subscription-leak, egress guard,
	// security profiles.
	PermSecurityView   Permission = "security.view"
	PermSecurityManage Permission = "security.manage"

	// CDN provider accounts, DNS connections, domain pools, operations.
	PermCdnView   Permission = "cdn.view"
	PermCdnManage Permission = "cdn.manage"

	// SelfSteal / decoy sites, both the global registry and node-scoped
	// deployments.
	PermSelfStealView   Permission = "selfsteal.view"
	PermSelfStealManage Permission = "selfsteal.manage"

	// Backups. Restore is deliberately a separate grant on top of manage:
	// running a restore overwrites live state.
	PermBackupsView    Permission = "backups.view"
	PermBackupsManage  Permission = "backups.manage"
	PermBackupsRestore Permission = "backups.restore"

	// E-Agent (Axosun AI diagnostics): operate = run/confirm reports,
	// manage = provider profiles and the action policy.
	PermEAgentView    Permission = "eagent.view"
	PermEAgentOperate Permission = "eagent.operate"
	PermEAgentManage  Permission = "eagent.manage"

	// Administrator audit journal.
	PermAuditView Permission = "audit.view"

	// Panel accounts. Managers may delegate only capabilities they already hold.
	PermAccountsManage Permission = "accounts.manage"

	// General panel settings: notifications, integrations, panel/agent
	// updates, cloud providers, internal tokens.
	PermSettingsView   Permission = "settings.view"
	PermSettingsManage Permission = "settings.manage"

	// Client-support module. A single entry grant: the module has its own
	// per-operator role matrix (internal/support) that governs everything
	// inside.
	PermSupportAccess Permission = "support.access"

	// Status page. Split finer than most domains here because the actions
	// differ in who they affect: editing a component is internal, publishing
	// an incident is a public announcement, and deploying touches a server.
	// One grant would have meant that whoever may rename a component may also
	// tell the customer's users that everything is down.
	PermStatuspageView        Permission = "statuspage.view"
	PermStatuspageEdit        Permission = "statuspage.edit"
	PermStatuspageIncidents   Permission = "statuspage.incidents"
	PermStatuspageMaintenance Permission = "statuspage.maintenance"
	PermStatuspagePublish     Permission = "statuspage.publish"
	PermStatuspageDeploy      Permission = "statuspage.deploy"

	// Subscription page. Two grants rather than the status page's six: there is
	// nothing here that changes what the public sees except deploying the page
	// itself, so the finer split would be inventing distinctions the section
	// does not have. Everything that reaches a server, a DNS zone or Remnawave's
	// settings is subscription_page.deploy; reading what is deployed is view.
	PermSubscriptionPageView   Permission = "subscription_page.view"
	PermSubscriptionPageDeploy Permission = "subscription_page.deploy"
)

// PermissionGroup is one human-facing branch of the permission tree — one
// panel domain with its selectable actions. Key doubles as the i18n namespace
// on the frontend (permissions.group.<key>, permissions.perm.<permission>).
type PermissionGroup struct {
	Key         string       `json:"key"`
	Permissions []Permission `json:"permissions"`
}

// PermissionRegistry returns every known permission grouped by panel domain,
// in UI display order. This is served to the frontend (GET
// /api/v1/auth/permissions) so the permission picker never hard-codes the
// list.
func PermissionRegistry() []PermissionGroup {
	return []PermissionGroup{
		{Key: "global", Permissions: []Permission{Permission(PermissionGlobalAdmin)}},
		{Key: "nodes", Permissions: []Permission{PermNodesView, PermNodesCreate, PermNodesEdit, PermNodesDelete, PermNodesOperate}},
		{Key: "connection_units", Permissions: []Permission{PermUnitsView, PermUnitsCreate, PermUnitsEdit, PermUnitsDelete, PermUnitsApply}},
		{Key: "remnawave", Permissions: []Permission{PermRemnawaveView, PermRemnawaveManage}},
		{Key: "subscribers", Permissions: []Permission{PermSubscribersView, PermSubscribersManage}},
		{Key: "subscriber_groups", Permissions: []Permission{PermGroupsView, PermGroupsManage}},
		{Key: "geo_assets", Permissions: []Permission{PermGeoAssetsView, PermGeoAssetsManage}},
		{Key: "health", Permissions: []Permission{PermHealthView, PermHealthManage}},
		{Key: "abuse", Permissions: []Permission{PermAbuseView, PermAbuseManage}},
		{Key: "security", Permissions: []Permission{PermSecurityView, PermSecurityManage}},
		{Key: "cdn", Permissions: []Permission{PermCdnView, PermCdnManage}},
		{Key: "selfsteal", Permissions: []Permission{PermSelfStealView, PermSelfStealManage}},
		{Key: "backups", Permissions: []Permission{PermBackupsView, PermBackupsManage, PermBackupsRestore}},
		{Key: "eagent", Permissions: []Permission{PermEAgentView, PermEAgentOperate, PermEAgentManage}},
		{Key: "audit", Permissions: []Permission{PermAuditView}},
		{Key: "support", Permissions: []Permission{PermSupportAccess}},
		{Key: "statuspage", Permissions: []Permission{
			PermStatuspageView, PermStatuspageEdit, PermStatuspageIncidents,
			PermStatuspageMaintenance, PermStatuspagePublish, PermStatuspageDeploy,
		}},
		{Key: "subscription_page", Permissions: []Permission{
			PermSubscriptionPageView, PermSubscriptionPageDeploy,
		}},
		{Key: "settings", Permissions: []Permission{PermSettingsView, PermSettingsManage}},
		{Key: "accounts", Permissions: []Permission{PermAccountsManage}},
	}
}

// knownPermissions indexes the registry for O(1) validation.
var knownPermissions = func() map[Permission]struct{} {
	m := make(map[Permission]struct{})
	for _, g := range PermissionRegistry() {
		for _, p := range g.Permissions {
			m[p] = struct{}{}
		}
	}
	return m
}()

// NormalizePermissions validates and canonicalises a permission list coming
// from the API: entries are trimmed and de-duplicated, unknown entries are
// rejected, and nil/empty input grants no capabilities. Global access must be explicit.
func NormalizePermissions(perms []string) ([]string, error) {
	if len(perms) == 0 {
		return []string{}, nil
	}
	seen := make(map[string]struct{}, len(perms))
	out := make([]string, 0, len(perms))
	for _, raw := range perms {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}

		if p == PermissionAll {
			if _, dup := seen[p]; !dup {
				seen[p] = struct{}{}
				out = append(out, p)
			}
			continue
		}
		if _, ok := knownPermissions[Permission(p)]; !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownPermission, p)
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, ErrNoPermissions
	}
	if _, global := seen[PermissionGlobalAdmin]; global {
		return []string{PermissionGlobalAdmin}, nil
	}
	sort.Strings(out)
	return out, nil
}

// Can reports whether the admin holds the given permission (directly or via
// a scoped wildcard or an explicit global grant). A nil admin can do nothing.
func (a *Admin) Can(perm Permission) bool {
	if a == nil {
		return false
	}
	for _, p := range a.Permissions {
		if p == PermissionGlobalAdmin || p == PermissionAll && (a.IsBootstrapSuperuser || ProjectPermission(perm)) || p == string(perm) {
			return true
		}
	}
	return false
}

// FullAccess reports installation-wide authority, independent of project membership.
func (a *Admin) FullAccess() bool {
	if a == nil {
		return false
	}
	for _, p := range a.Permissions {
		if p == PermissionGlobalAdmin || p == PermissionAll && a.IsBootstrapSuperuser {
			return true
		}
	}
	return false
}

// ValidateDelegation prevents account managers from minting privileges they do
// not hold. Only the protected installation owner may delegate the global
// grant; project membership does not limit that grant's authority.
func ValidateDelegation(actor *Admin, permissions []string) error {
	if actor == nil || actor.IsBootstrapSuperuser {
		return nil
	}
	for _, p := range permissions {
		if p == PermissionGlobalAdmin {
			return ErrPermissionDelegation
		}
		if p == PermissionAll {
			for known := range knownPermissions {
				if ProjectPermission(known) && !actor.Can(known) {
					return ErrPermissionDelegation
				}
			}
			continue
		}
		if !actor.Can(Permission(p)) {
			return ErrPermissionDelegation
		}
	}
	return nil
}

// HasWildcard checks a normalized permission list, including an empty list.
func HasWildcard(permissions []string) bool {
	for _, p := range permissions {
		if p == PermissionAll {
			return true
		}
	}
	return false
}

// ProjectPermission is a positive allowlist: a new capability never silently
// joins the project wildcard. Its resource handlers must also enforce membership.
func ProjectPermission(p Permission) bool {
	switch p {
	case PermAccountsManage, PermNodesView, PermNodesCreate, PermNodesEdit, PermNodesDelete, PermNodesOperate,
		PermUnitsView, PermUnitsCreate, PermUnitsEdit, PermUnitsDelete, PermUnitsApply,
		PermRemnawaveView, PermRemnawaveManage, PermSupportAccess,
		PermSubscribersView, PermSubscribersManage, PermGroupsView, PermGroupsManage:
		return true
	}
	return false
}
