package ui

import teleservices "github.com/gravitational/teleport/lib/services"

// Role describes user role consumed by web ui
type Role struct {
	// Name is role name
	Name string `json:"name"`
	// Access is a set of attributes describing role permissions
	Access RoleAccess `json:"access"`
	// System is a flag indicating if a role is builtin system role
	System bool `json:"system"`
}

// NewRole creates a new instance of UI Role
func NewRole(sRole teleservices.ServiceRole) *Role {
	uiRole := Role{
		Name: sRole.GetName(),
	}

	uiRole.Access.init(sRole)
	return &uiRole
}

// ToTeleRole converts UI Role to Storage Role
func (r *Role) ToTeleRole() (teleservices.ServiceRole, error) {
	teleRole, err := teleservices.NewServiceRole(r.Name, teleservices.ServiceRoleSpecV2{})
	if err != nil {
		return nil, err
	}

	r.Access.Apply(teleRole)
	return teleRole, nil
}
