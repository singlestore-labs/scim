package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOktaSCIM20Suite(t *testing.T) {
	t.Parallel()
	// Sources:
	// https://developer.okta.com/docs/api/openapi/okta-scim/guides/scim-20
	// https://developer.okta.com/docs/guides/scim-provisioning-integration-test/main/
	// https://developer.okta.com/docs/guides/scim-provisioning-integration-prepare/main/

	t.Run("GET Users pagination for connection and import", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		h.ListUsers(ListOpts{StartIndex: 1, Count: intPointer(2)}).
			RequireStatus(http.StatusOK).
			RequireJSONEq(`{
				"schemas": ["urn:ietf:params:scim:api:messages:2.0:ListResponse"],
				"totalResults": 0,
				"Resources": [],
				"startIndex": 1,
				"itemsPerPage": 0
			}`)
		h.MustCreateUser(h.Data.User())
		h.MustCreateUser(h.Data.User())
		h.ListUsers(ListOpts{StartIndex: 1, Count: intPointer(100)}).
			RequireStatus(http.StatusOK).
			RequirePath("totalResults", 2)
	})

	t.Run("GET Groups pagination for connection", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		h.ListGroups(ListOpts{StartIndex: 1, Count: intPointer(100)}).
			RequireStatus(http.StatusOK).
			RequirePath("totalResults", 0).
			RequireJSONContains(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:ListResponse"],"Resources":[]}`)
	})

	t.Run("CRUD user lifecycle", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		input := h.Data.User()
		input.ExternalID = "okta-ext-" + input.UserName
		want := userInputJSON(input)

		created := h.CreateUser(input).
			RequireStatus(http.StatusCreated).
			RequireJSONContains(want).
			User()

		h.ListUsers(ListOpts{Filter: filterExpr("userName", "eq", input.UserName), StartIndex: 1, Count: intPointer(100)}).
			RequireStatus(http.StatusOK).
			RequirePath("totalResults", 1).
			RequirePath("Resources.0.id", created.ID).
			RequirePath("Resources.0.userName", input.UserName)

		h.GetUser(created.ID).
			RequireStatus(http.StatusOK).
			RequireJSONContains(want).
			RequirePath("id", created.ID)

		input.Name.GivenName = "Updated"
		h.ReplaceUser(created.ID, input).
			RequireStatus(http.StatusOK).
			RequireJSONContains(`{"name":{"givenName":"Updated"}}`)
		h.GetUser(created.ID).RequireJSONContains(`{"name":{"givenName":"Updated"}}`)

		h.PatchUser(created.ID, PatchOp{Op: "replace", Value: json.RawMessage(`{"active":false}`)}).
			RequireSuccess()
		h.GetUser(created.ID).RequireJSONContains(`{"active":false}`)

		h.PatchUser(created.ID, PatchOp{Op: "replace", Value: json.RawMessage(`{"active":true}`)}).
			RequireSuccess()
		h.GetUser(created.ID).RequireJSONContains(`{"active":true}`)

		h.PatchUser(created.ID, PatchOp{Op: "replace", Value: json.RawMessage(`{"active":false}`)}).
			RequireSuccess()
		h.ListUsers(ListOpts{Filter: filterExpr("userName", "eq", input.UserName)}).
			RequirePath("Resources.0.active", false)
	})

	t.Run("PUT then GET user as Okta custom apps do", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		input := h.Data.User()
		created := h.MustCreateUser(input)
		input.DisplayName = "Renamed by PUT"
		h.GetUser(created.ID).RequireStatus(http.StatusOK)
		h.ReplaceUser(created.ID, input).RequireStatus(http.StatusOK)
		h.GetUser(created.ID).RequireJSONContains(`{"displayName":"Renamed by PUT"}`)
	})

	t.Run("group create filter get rename members delete", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		user := h.MustCreateUser(h.Data.User())
		input := h.Data.Group()

		group := h.CreateGroup(input).
			RequireStatus(http.StatusCreated).
			RequireJSONContains(map[string]any{"displayName": input.DisplayName}).
			Group()

		h.ListGroups(ListOpts{Filter: filterExpr("displayName", "eq", input.DisplayName), StartIndex: 1, Count: intPointer(100)}).
			RequireStatus(http.StatusOK).
			RequirePath("totalResults", 1).
			RequirePath("Resources.0.id", group.ID)

		h.GetGroup(group.ID).
			RequireStatus(http.StatusOK).
			RequireJSONContains(map[string]any{"id": group.ID, "displayName": input.DisplayName})

		h.PatchGroup(group.ID, PatchOp{Op: "replace", Path: "displayName", Value: toJSONObject(t, "Okta Renamed Group")}).
			RequireSuccess()
		h.GetGroup(group.ID).RequireJSONContains(`{"displayName":"Okta Renamed Group"}`)

		input.DisplayName = "Okta PUT Group"
		h.ReplaceGroup(group.ID, input).RequireStatus(http.StatusOK)
		h.GetGroup(group.ID).RequireJSONContains(`{"displayName":"Okta PUT Group"}`)

		h.PatchGroup(group.ID, PatchOp{
			Op:    "add",
			Path:  "members",
			Value: toJSONObject(t, []map[string]string{{"value": user.ID}}),
		}).RequireSuccess()
		h.GetGroup(group.ID).RequirePath("members.0.value", user.ID)

		h.PatchGroup(group.ID, PatchOp{
			Op:   "remove",
			Path: fmt.Sprintf("members[value eq %q]", user.ID),
		}).RequireSuccess()
		require.Empty(t, h.GetGroup(group.ID).RequireStatus(http.StatusOK).Group().Members)

		h.DeleteGroup(group.ID).RequireStatus(http.StatusNoContent)
	})
}
