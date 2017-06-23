/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
)

func TestServiceRoleParsing(t *testing.T) { TestingT(t) }

type ServiceRoleSuite struct {
}

var _ = Suite(&ServiceRoleSuite{})

func (s *ServiceRoleSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *ServiceRoleSuite) TestServiceRoleExtension(c *C) {
	type Spec struct {
		ServiceRoleSpecV2
		A string `json:"a"`
	}
	type ExtendedServiceRole struct {
		Spec Spec `json:"spec"`
	}
	in := `{"kind": "role", "metadata": {"name": "name1"}, "spec": {"a": "b"}}`
	var role ExtendedServiceRole
	err := utils.UnmarshalWithSchema(GetServiceRoleSchema(`"a": {"type": "string"}`), &role, []byte(in))
	c.Assert(err, IsNil)
	c.Assert(role.Spec.A, Equals, "b")

	// this is a bad type
	in = `{"kind": "role", "metadata": {"name": "name1"}, "spec": {"a": 12}}`
	err = utils.UnmarshalWithSchema(GetServiceRoleSchema(`"a": {"type": "string"}`), &role, []byte(in))
	c.Assert(err, NotNil)
}

func (s *ServiceRoleSuite) TestServiceRoleParse(c *C) {
	testCases := []struct {
		in    string
		role  ServiceRoleV2
		error error
	}{
		{
			in:    ``,
			error: trace.BadParameter("empty input"),
		},
		{
			in:    `{}`,
			error: trace.BadParameter("failed to validate: name: name is required"),
		},
		{
			in:    `{"kind": "role"}`,
			error: trace.BadParameter("failed to validate: name: name is required"),
		},
		{
			in: `{"kind": "role", "metadata": {"name": "name1"}, "spec": {}}`,
			role: ServiceRoleV2{
				Kind:    KindServiceRole,
				Version: V2,
				Metadata: Metadata{
					Name:      "name1",
					Namespace: defaults.Namespace,
				},
				Spec: ServiceRoleSpecV2{},
			},
		},
		{
			in: `{
              "kind": "role", 
              "metadata": {"name": "name1"}, 
              "spec": {
                 "max_session_ttl": "20h",
                 "node_labels": {"a": "b"},
                 "namespaces": ["system", "default"],
                 "resources": {
                    "role": ["read", "write"]
                 }
              }
            }`,
			role: ServiceRoleV2{
				Kind:    KindServiceRole,
				Version: V2,
				Metadata: Metadata{
					Name:      "name1",
					Namespace: defaults.Namespace,
				},
				Spec: ServiceRoleSpecV2{
					MaxSessionTTL: Duration{20 * time.Hour},
					NodeLabels:    map[string]string{"a": "b"},
					Namespaces:    []string{"system", "default"},
					Resources:     map[string][]string{"role": {ActionRead, ActionWrite}},
				},
			},
		},
		{
			in: `kind: role
metadata:
  name: name1
spec:
  max_session_ttl: 20h
  node_labels:
    a: b
  namespaces: ["system", "default"]
  resources:
    role: [read, write]
`,
			role: ServiceRoleV2{
				Kind:    KindServiceRole,
				Version: V2,
				Metadata: Metadata{
					Name:      "name1",
					Namespace: defaults.Namespace,
				},
				Spec: ServiceRoleSpecV2{
					MaxSessionTTL: Duration{20 * time.Hour},
					NodeLabels:    map[string]string{"a": "b"},
					Namespaces:    []string{"system", "default"},
					Resources:     map[string][]string{"role": {ActionRead, ActionWrite}},
				},
			},
		},
	}
	for i, tc := range testCases {
		comment := Commentf("test case %v", i)
		role, err := UnmarshalServiceRole([]byte(tc.in))
		if tc.error != nil {
			c.Assert(err, NotNil, comment)
		} else {
			c.Assert(err, IsNil, comment)
			c.Assert(*role, DeepEquals, tc.role, comment)

			out, err := json.Marshal(*role)
			c.Assert(err, IsNil, comment)

			role2, err := UnmarshalServiceRole(out)
			c.Assert(err, IsNil, comment)
			c.Assert(*role2, DeepEquals, tc.role, comment)
		}
	}
}

func (s *ServiceRoleSuite) TestCheckAccess(c *C) {
	type check struct {
		server    Server
		hasAccess bool
		login     string
	}
	serverA := &ServerV2{
		Metadata: Metadata{
			Name: "a",
		},
	}
	serverB := &ServerV2{
		Metadata: Metadata{
			Name:      "b",
			Namespace: defaults.Namespace,
			Labels:    map[string]string{"role": "worker", "status": "follower"},
		},
	}
	namespaceC := "namespace-c"
	serverC := &ServerV2{
		Metadata: Metadata{
			Name:      "c",
			Namespace: namespaceC,
			Labels:    map[string]string{"role": "db", "status": "follower"},
		},
	}
	testCases := []struct {
		name   string
		roles  []ServiceRoleV2
		checks []check
	}{
		{
			name:  "empty role set has access to nothing",
			roles: []ServiceRoleV2{},
			checks: []check{
				{server: serverA, login: "root", hasAccess: false},
				{server: serverB, login: "root", hasAccess: false},
				{server: serverC, login: "root", hasAccess: false},
			},
		},
		{
			name: "role is limited to default namespace",
			roles: []ServiceRoleV2{
				ServiceRoleV2{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: ServiceRoleSpecV2{
						MaxSessionTTL: Duration{20 * time.Hour},
						Logins:        []string{"admin"},
						NodeLabels:    map[string]string{Wildcard: Wildcard},
						Namespaces:    []string{defaults.Namespace},
					},
				},
			},
			checks: []check{
				{server: serverA, login: "root", hasAccess: false},
				{server: serverA, login: "admin", hasAccess: true},
				{server: serverB, login: "root", hasAccess: false},
				{server: serverB, login: "admin", hasAccess: true},
				{server: serverC, login: "root", hasAccess: false},
				{server: serverC, login: "admin", hasAccess: false},
			},
		},
		{
			name: "role is limited to labels in default namespace",
			roles: []ServiceRoleV2{
				ServiceRoleV2{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: ServiceRoleSpecV2{
						MaxSessionTTL: Duration{20 * time.Hour},
						Logins:        []string{"admin"},
						NodeLabels:    map[string]string{"role": "worker"},
						Namespaces:    []string{defaults.Namespace},
					},
				},
			},
			checks: []check{
				{server: serverA, login: "root", hasAccess: false},
				{server: serverA, login: "admin", hasAccess: false},
				{server: serverB, login: "root", hasAccess: false},
				{server: serverB, login: "admin", hasAccess: true},
				{server: serverC, login: "root", hasAccess: false},
				{server: serverC, login: "admin", hasAccess: false},
			},
		},
		{
			name: "one role is more permissive than another",
			roles: []ServiceRoleV2{
				ServiceRoleV2{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: ServiceRoleSpecV2{
						MaxSessionTTL: Duration{20 * time.Hour},
						Logins:        []string{"admin"},
						NodeLabels:    map[string]string{"role": "worker"},
						Namespaces:    []string{defaults.Namespace},
					},
				},
				ServiceRoleV2{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: ServiceRoleSpecV2{
						MaxSessionTTL: Duration{20 * time.Hour},
						Logins:        []string{"root", "admin"},
						NodeLabels:    map[string]string{Wildcard: Wildcard},
						Namespaces:    []string{Wildcard},
					},
				},
			},
			checks: []check{
				{server: serverA, login: "root", hasAccess: true},
				{server: serverA, login: "admin", hasAccess: true},
				{server: serverB, login: "root", hasAccess: true},
				{server: serverB, login: "admin", hasAccess: true},
				{server: serverC, login: "root", hasAccess: true},
				{server: serverC, login: "admin", hasAccess: true},
			},
		},
	}
	for i, tc := range testCases {

		var set ServiceRoleSet
		for i := range tc.roles {
			set = append(set, &tc.roles[i])
		}
		for j, check := range tc.checks {
			comment := Commentf("test case %v '%v', check %v", i, tc.name, j)
			result := set.CheckAccessToServer(check.login, check.server)
			if check.hasAccess {
				c.Assert(result, IsNil, comment)
			} else {
				c.Assert(trace.IsAccessDenied(result), Equals, true, comment)
			}

		}
	}
}

func (s *ServiceRoleSuite) TestCheckResourceAccess(c *C) {
	type check struct {
		hasAccess bool
		action    string
		namespace string
		resource  string
	}
	testCases := []struct {
		name   string
		roles  []ServiceRoleV2
		checks []check
	}{
		{
			name:  "empty role set has access to nothing",
			roles: []ServiceRoleV2{},
			checks: []check{
				{resource: KindUser, action: ActionWrite, namespace: defaults.Namespace, hasAccess: false},
			},
		},
		{
			name: "user can read sessions in default namespace",
			roles: []ServiceRoleV2{
				ServiceRoleV2{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: ServiceRoleSpecV2{
						Namespaces: []string{defaults.Namespace},
						Resources:  map[string][]string{KindSession: []string{ActionRead}},
					},
				},
			},
			checks: []check{
				{resource: KindSession, action: ActionRead, namespace: defaults.Namespace, hasAccess: true},
				{resource: KindSession, action: ActionWrite, namespace: defaults.Namespace, hasAccess: false},
			},
		},
		{
			name: "user can read sessions in system namespace and write stuff in default namespace",
			roles: []ServiceRoleV2{
				ServiceRoleV2{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: ServiceRoleSpecV2{
						Namespaces: []string{"system"},
						Resources:  map[string][]string{KindSession: []string{ActionRead}},
					},
				},
				ServiceRoleV2{
					Metadata: Metadata{
						Name:      "name2",
						Namespace: defaults.Namespace,
					},
					Spec: ServiceRoleSpecV2{
						Namespaces: []string{defaults.Namespace},
						Resources:  map[string][]string{KindSession: []string{ActionWrite, ActionRead}},
					},
				},
			},
			checks: []check{
				{resource: KindSession, action: ActionRead, namespace: defaults.Namespace, hasAccess: true},
				{resource: KindSession, action: ActionWrite, namespace: defaults.Namespace, hasAccess: true},
				{resource: KindSession, action: ActionWrite, namespace: "system", hasAccess: false},
				{resource: KindServiceRole, action: ActionRead, namespace: defaults.Namespace, hasAccess: false},
			},
		},
	}
	for i, tc := range testCases {

		var set ServiceRoleSet
		for i := range tc.roles {
			set = append(set, &tc.roles[i])
		}
		for j, check := range tc.checks {
			comment := Commentf("test case %v '%v', check %v", i, tc.name, j)
			result := set.CheckResourceAction(check.namespace, check.resource, check.action)
			if check.hasAccess {
				c.Assert(result, IsNil, comment)
			} else {
				c.Assert(trace.IsAccessDenied(result), Equals, true, comment)
			}

		}
	}
}
