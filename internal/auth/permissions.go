package auth

import (
	"fmt"
	"sort"
	"strings"
)

// Permission identifies a single panel capability an admin account may hold,
// e.g. "nodes.edit". Permissions are stored per-account (admins.permissions,
// JSONB array of strings) and checked on every request by the route guards in
// internal/server. The special value PermissionAll grants every capability,
// including ones added in future releases; it is what every pre-RBAC account
// is backfilled with, so upgrading changes nothing until an operator narrows
// an account explicitly.
type Permission string

// PermissionAll is the wildcard entry meaning "full administrator access".
// NormalizePermissions collapses any list containing it to exactly ["*"], so
// full access is always stored in one canonical shape.
const PermissionAll = "*"

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

	// Panel admin accounts. A single grant: whoever manages accounts can
	// grant themselves anything anyway, so a separate "view" tier would be
	// security theatre.
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
		{Key: "nodes", Permissions: []Permission{PermNodesView, PermNodesCreate, PermNodesEdit, PermNodesDelete, PermNodesOperate}},
		{Key: "connection_units", Permissions: []Permission{PermUnitsView, PermUnitsCreate, PermUnitsEdit, PermUnitsDelete, PermUnitsApply}},
		{Key: "remnawave", Permissions: []Permission{PermRemnawaveView, PermRemnawaveManage}},
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
// from the API: entries are trimmed and de-duplicated, any list containing
// the wildcard collapses to exactly ["*"], unknown entries are rejected, and
// nil/empty input means full access (the pre-RBAC default, which also keeps
// older API clients that never send the field working unchanged).
func NormalizePermissions(perms []string) ([]string, error) {
	if len(perms) == 0 {
		return []string{PermissionAll}, nil
	}
	seen := make(map[string]struct{}, len(perms))
	out := make([]string, 0, len(perms))
	for _, raw := range perms {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		if p == PermissionAll {
			return []string{PermissionAll}, nil
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
	sort.Strings(out)
	return out, nil
}

// Can reports whether the admin holds the given permission (directly or via
// the full-access wildcard). A nil admin can do nothing.
func (a *Admin) Can(perm Permission) bool {
	if a == nil {
		return false
	}
	for _, p := range a.Permissions {
		if p == PermissionAll || p == string(perm) {
			return true
		}
	}
	return false
}

// FullAccess reports whether the account holds the wildcard grant.
func (a *Admin) FullAccess() bool {
	if a == nil {
		return false
	}
	for _, p := range a.Permissions {
		if p == PermissionAll {
			return true
		}
	}
	return false
}
