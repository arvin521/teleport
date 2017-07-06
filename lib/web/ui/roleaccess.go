package ui

import (
	"time"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	inviteStatus            = "invited"
	activeStatus            = "active"
	userTypeToHide          = "agent"
	roleDefaultAllowedLogin = "!invalid!"
)

var adminResources = []string{
	services.KindRole,
	services.KindUser,
	services.KindOIDC,
	services.KindCertAuthority,
	services.KindReverseTunnel,
	services.KindTrustedCluster,
	services.KindNode,
}

// AdminAccess describes admin access
type AdminAccess struct {
	// Enabled indicates if access is enabled
	Enabled bool `json:"enabled"`
}

// SSHAccess describes shh access
type SSHAccess struct {
	// Logins is a list of allowed logins
	Logins []string `json:"logins"`
	// MaxSessionTTL is max session TLL
	MaxSessionTTL time.Duration `json:"maxTtl"`
	// NodeLabels
	NodeLabels map[string]string `json:"nodeLabels"`
}

// RoleAccess describes a set of role permissions
type RoleAccess struct {
	// Admin describes admin access
	Admin AdminAccess `json:"admin"`
	// SSH describes SSH access
	SSH SSHAccess `json:"ssh"`
}

// MergeAccessSet merges a set of roles by strongest permission
func MergeAccessSet(accessList []*RoleAccess) *RoleAccess {
	uiAccess := RoleAccess{}
	for _, item := range accessList {
		uiAccess.SSH.Logins = utils.Deduplicate(append(uiAccess.SSH.Logins, item.SSH.Logins...))
		uiAccess.Admin.Enabled = item.Admin.Enabled || uiAccess.Admin.Enabled
	}

	return &uiAccess
}

// Apply applies this role access to Teleport Role
func (a *RoleAccess) Apply(teleRole services.Role) {
	a.applyAdmin(teleRole)
	a.applySSH(teleRole)
}

func (a *RoleAccess) init(teleRole services.Role) {
	a.initAdmin(teleRole)
	a.initSSH(teleRole)
}

func (a *RoleAccess) initSSH(teleRole services.Role) {
	a.SSH.MaxSessionTTL = teleRole.GetMaxSessionTTL().Duration
	a.SSH.NodeLabels = teleRole.GetNodeLabels(services.Allow)
	// FIXME: this is a workaround for #1623
	filteredLogins := []string{}
	for _, login := range teleRole.GetLogins(services.Allow) {
		if login != roleDefaultAllowedLogin {
			filteredLogins = append(filteredLogins, login)
		}
	}

	a.SSH.Logins = filteredLogins
}

func (a *RoleAccess) initAdmin(teleRole services.Role) {
	hasAllNamespaces := services.MatchNamespace(
		teleRole.GetNamespaces(services.Allow),
		services.Wildcard)

	rules := teleRole.GetRules(services.Allow)
	a.Admin.Enabled = hasFullAccess(rules, adminResources) && hasAllNamespaces
}

func (a *RoleAccess) applyAdmin(role services.Role) {
	if a.Admin.Enabled {
		allowAllNamespaces(role)
		applyRuleAccess(role, adminResources, services.RW())
	} else {
		rules := role.GetRules(services.Allow)
		delete(rules, services.Wildcard)
		role.SetRules(services.Allow, rules)
		applyRuleAccess(role, adminResources, services.RO())
	}
}

func (a *RoleAccess) applySSH(teleRole services.Role) {
	// FIXME: this is a workaround for #1623
	if len(a.SSH.Logins) == 0 {
		a.SSH.Logins = append(a.SSH.Logins, roleDefaultAllowedLogin)
	}

	teleRole.SetMaxSessionTTL(a.SSH.MaxSessionTTL)
	teleRole.SetLogins(services.Allow, a.SSH.Logins)
	teleRole.SetNodeLabels(services.Allow, a.SSH.NodeLabels)
}

func all() []string {
	return []string{services.Wildcard}
}

func allowAllNamespaces(teleRole services.Role) {
	newNamespaces := utils.Deduplicate(append(teleRole.GetNamespaces(services.Allow), all()...))
	teleRole.SetNamespaces(services.Allow, newNamespaces)
}

func none() []string {
	return nil
}

func hasFullAccess(rules map[string][]string, resources []string) bool {
	for _, resource := range resources {
		hasRead := services.MatchRule(rules, resource, services.ActionRead)
		hasWrite := services.MatchRule(rules, resource, services.ActionWrite)

		if !(hasRead && hasWrite) {
			return false
		}
	}

	return true
}

func applyRuleAccess(role services.Role, resources []string, verbs []string) {
	rules := role.GetRules(services.Allow)
	for _, resource := range resources {
		rules[resource] = verbs
	}

	role.SetRules(services.Allow, rules)
}
