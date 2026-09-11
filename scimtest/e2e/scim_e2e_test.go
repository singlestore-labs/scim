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
		want string
	}{
		{
			name: "replace display name",
			body: func(User) any {
				return PatchOp{Op: "replace", Path: "displayName", Value: json.RawMessage(`"Patched Name"`)}
			},
			want: `{"displayName":"Patched Name"}`,
		},
		{
			name: "azure operation casing deactivates user",
			body: func(User) any {
				return PatchOp{Op: "Replace", Path: "active", Value: json.RawMessage(`false`)}
			},
			want: `{"active":false}`,
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
			want: `{"name":{"familyName":"Raw-JSON-Family"}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client := NewTestClient(t)
			created := client.MustCreateUser(client.Data.User())

			client.PatchUser(created.ID, test.body(created)).RequireSuccess()
			client.GetUser(created.ID).
				RequireStatus(http.StatusOK).
				RequireJSONContains(test.want)
		})
	}
}

func TestFlexibleUserFilters(t *testing.T) {
	t.Parallel()

	client := NewTestClient(t)
	inputs := []UserInput{client.Data.User(), client.Data.User(), client.Data.User()}
	for _, input := range inputs {
		client.MustCreateUser(input)
	}

	t.Run("joining property", func(t *testing.T) {
		client.ListUsers(ListOpts{Filter: fmt.Sprintf(`userName eq "%s"`, inputs[0].UserName)}).
			RequireStatus(http.StatusOK).
			RequirePath("totalResults", 1).
			RequirePath("Resources.0.userName", inputs[0].UserName)
	})

	t.Run("multi-value work email", func(t *testing.T) {
		client.ListUsers(ListOpts{Filter: fmt.Sprintf(`emails[type eq "work" and value eq "%s"]`, inputs[1].Emails[0].Value)}).
			RequireStatus(http.StatusOK).
			RequirePath("totalResults", 1).
			RequirePath("Resources[0].userName", inputs[1].UserName)
	})

	t.Run("no match still returns list response", func(t *testing.T) {
		client.ListUsers(ListOpts{Filter: `userName eq "missing@example.test"`}).
			RequireStatus(http.StatusOK).
			RequireJSONEq(`{
				"schemas": ["urn:ietf:params:scim:api:messages:2.0:ListResponse"],
				"totalResults": 0,
				"Resources": [],
				"startIndex": 1,
				"itemsPerPage": 0
			}`)
	})
}

func TestUserPagination(t *testing.T) {
	t.Parallel()

	client := NewTestClient(t)
	for range 5 {
		client.MustCreateUser(client.Data.User())
	}

	client.ListUsers(ListOpts{StartIndex: 2, Count: intPointer(2)}).
		RequireStatus(http.StatusOK).
		RequirePath("totalResults", 5).
		RequirePath("itemsPerPage", 2).
		RequirePath("startIndex", 2)
}

func TestInvalidFilterReturnsSCIMError(t *testing.T) {
	t.Parallel()

	client := NewTestClient(t)
	scimError := client.ListUsers(ListOpts{Filter: `userName eq`}).
		RequireError(http.StatusBadRequest, "invalidFilter")
	require.NotEmpty(t, scimError.Detail)
}

func TestRawCreateBodyAndDelete(t *testing.T) {
	t.Parallel()

	client := NewTestClient(t)
	input := client.Data.User()
	body := json.RawMessage(fmt.Sprintf(`{
		"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
		"userName": %q,
		"active": true
	}`, input.UserName))

	created := client.CreateUser(body).
		RequireStatus(http.StatusCreated).
		RequireJSONContains(fmt.Sprintf(`{"userName":%q,"active":true}`, input.UserName)).
		User()
	require.NotEmpty(t, created.ID)

	client.GetUser(created.ID).
		RequireStatus(http.StatusOK).
		RequireJSONContains(fmt.Sprintf(`{"id":%q,"userName":%q,"active":true}`, created.ID, input.UserName))
	client.DeleteUser(created.ID).RequireStatus(http.StatusNoContent)
}

func TestGroupMemberPatch(t *testing.T) {
	t.Parallel()

	client := NewTestClient(t)
	user := client.MustCreateUser(client.Data.User())
	groupInput := client.Data.Group()
	group := client.MustCreateGroup(groupInput)

	client.PatchGroup(group.ID, PatchOp{
		Op:    "Add",
		Path:  "members",
		Value: json.RawMessage(fmt.Sprintf(`[{"value":%q}]`, user.ID)),
	}).RequireSuccess()

	client.GetGroup(group.ID).
		RequireStatus(http.StatusOK).
		RequireJSONContains(fmt.Sprintf(`{"id":%q,"displayName":%q}`, group.ID, groupInput.DisplayName)).
		RequirePath("members.0.value", user.ID)
}

func TestDiscoveryEndpoints(t *testing.T) {
	t.Parallel()

	client := NewTestClient(t)
	for name, result := range map[string]Result{
		"service provider config": client.GetServiceProviderConfig(),
		"schemas":                 client.GetSchemas(),
		"resource types":          client.GetResourceTypes(),
	} {
		t.Run(name, func(t *testing.T) {
			result.t = t
			result.RequireStatus(http.StatusOK)
			require.True(t, json.Valid(result.Body), result.String())
		})
	}
}
