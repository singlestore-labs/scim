package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserPatchCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body func(User) any
		want func(User) WantUser
	}{
		{
			name: "replace display name",
			body: func(User) any {
				return PatchOp{Op: "replace", Path: "displayName", Value: json.RawMessage(`"Patched Name"`)}
			},
			want: func(User) WantUser {
				return WantUser{DisplayName: pointer("Patched Name")}
			},
		},
		{
			name: "azure operation casing deactivates user",
			body: func(User) any {
				return PatchOp{Op: "Replace", Path: "active", Value: json.RawMessage(`false`)}
			},
			want: func(User) WantUser {
				return WantUser{Active: pointer(false)}
			},
		},
		{
			name: "complete raw patch document",
			body: func(User) any {
				return json.RawMessage(`{
					"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
					"Operations": [{
						"op": "replace",
						"path": "name.familyName",
						"value": "Raw-JSON-Family"
					}]
				}`)
			},
			want: func(User) WantUser {
				return WantUser{FamilyName: pointer("Raw-JSON-Family")}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			harness := NewHarness(t)
			created := harness.MustCreateUser(harness.Data.User())

			got := harness.PatchThenGetUser(created.ID, test.body(created))

			RequireUser(t, got, test.want(created))
		})
	}
}

func TestFlexibleUserFilters(t *testing.T) {
	t.Parallel()

	harness := NewHarness(t)
	inputs := []UserInput{harness.Data.User(), harness.Data.User(), harness.Data.User()}
	for _, input := range inputs {
		harness.MustCreateUser(input)
	}

	tests := []struct {
		name       string
		filter     string
		wantNames  []string
		totalCount int
	}{
		{
			name:       "joining property",
			filter:     fmt.Sprintf(`userName eq "%s"`, inputs[0].UserName),
			wantNames:  []string{inputs[0].UserName},
			totalCount: 1,
		},
		{
			name:       "multi-value work email",
			filter:     fmt.Sprintf(`emails[type eq "work" and value eq "%s"]`, inputs[1].Emails[0].Value),
			wantNames:  []string{inputs[1].UserName},
			totalCount: 1,
		},
		{
			name:       "no match still returns list response",
			filter:     `userName eq "missing@example.test"`,
			wantNames:  []string{},
			totalCount: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := harness.MustListUsers(ListOpts{Filter: test.filter})
			RequireUserList(t, got, WantUserList{
				TotalResults: pointer(test.totalCount),
				ItemsPerPage: pointer(test.totalCount),
				UserNames:    test.wantNames,
			})
		})
	}
}

func TestUserPagination(t *testing.T) {
	t.Parallel()

	harness := NewHarness(t)
	for range 5 {
		harness.MustCreateUser(harness.Data.User())
	}

	got := harness.MustListUsers(ListOpts{StartIndex: 2, Count: intPointer(2)})
	RequireUserList(t, got, WantUserList{
		TotalResults: pointer(5),
		ItemsPerPage: pointer(2),
		StartIndex:   pointer(2),
	})
}

func TestRawCreateBodyAndDelete(t *testing.T) {
	t.Parallel()

	harness := NewHarness(t)
	input := harness.Data.User()
	body := json.RawMessage(fmt.Sprintf(`{
		"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
		"userName": %q,
		"active": true
	}`, input.UserName))

	created := harness.MustCreateUser(body)
	RequireUser(t, harness.MustGetUser(created.ID), WantUser{
		ID:       pointer(created.ID),
		UserName: pointer(input.UserName),
		Active:   pointer(true),
	})
	harness.MustDeleteUser(created.ID)
}

func TestGroupMemberPatch(t *testing.T) {
	t.Parallel()

	harness := NewHarness(t)
	user := harness.MustCreateUser(harness.Data.User())
	groupInput := harness.Data.Group()
	group := harness.MustCreateGroup(groupInput)

	got := harness.PatchThenGetGroup(group.ID, PatchOp{
		Op:    "Add",
		Path:  "members",
		Value: json.RawMessage(fmt.Sprintf(`[{"value":%q}]`, user.ID)),
	})

	RequireGroup(t, got, WantGroup{
		ID:          pointer(group.ID),
		DisplayName: pointer(groupInput.DisplayName),
		MemberIDs:   []string{user.ID},
	})
}

func TestDiscoveryEndpoints(t *testing.T) {
	t.Parallel()

	harness := NewHarness(t)
	for name, result := range map[string]Result{
		"service provider config": harness.GetServiceProviderConfig(),
		"schemas":                 harness.GetSchemas(),
		"resource types":          harness.GetResourceTypes(),
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, http.StatusOK, result.Status, result.String())
			require.True(t, json.Valid(result.Body), result.String())
		})
	}
}
