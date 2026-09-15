package e2e

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAzureEntraSCIMValidator(t *testing.T) {
	t.Parallel()
	// Sources:
	// https://learn.microsoft.com/en-us/entra/identity/app-provisioning/scim-validator-tutorial
	// https://learn.microsoft.com/en-us/entra/identity/app-provisioning/use-scim-to-provision-users-and-groups

	t.Run("schema discovery", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		h.GetServiceProviderConfig().
			RequireStatus(http.StatusOK).
			RequireJSONContains(`{"patch":{"supported":true},"filter":{"supported":true}}`)
		h.GetSchemas().RequireStatus(http.StatusOK)
		h.GetResourceTypes().RequireStatus(http.StatusOK)
	})

	t.Run("Create New User then filter then delete", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		input := h.Data.User()
		created := h.CreateUser(input).
			RequireStatus(http.StatusCreated).
			RequireJSONContains(map[string]any{"userName": input.UserName, "active": true}).
			User()
		h.ListUsers(ListOpts{Filter: filterExpr("userName", "eq", input.UserName)}).
			RequireStatus(http.StatusOK).
			RequirePath("totalResults", 1).
			RequirePath("Resources.0.id", created.ID).
			RequirePath("Resources.0.userName", input.UserName)
		h.DeleteUser(created.ID).RequireStatus(http.StatusNoContent)
	})

	t.Run("Create Duplicate User returns 409", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		input := h.Data.User()
		h.CreateUser(input).RequireStatus(http.StatusCreated)
		h.CreateUser(input).RequireError(http.StatusConflict, "uniqueness")
	})

	t.Run("Get User found and not found", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		created := h.MustCreateUser(h.Data.User())
		h.GetUser(created.ID).
			RequireStatus(http.StatusOK).
			RequireJSONContains(map[string]any{"id": created.ID, "userName": created.UserName})
		h.GetUser("missing-user-id").RequireError(http.StatusNotFound, "")
	})

	t.Run("Get User by query zero results", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		h.ListUsers(ListOpts{Filter: filterExpr("userName", "eq", "non-existent user")}).
			RequireStatus(http.StatusOK).
			RequireJSONEq(`{
				"schemas": ["urn:ietf:params:scim:api:messages:2.0:ListResponse"],
				"totalResults": 0,
				"Resources": [],
				"startIndex": 1,
				"itemsPerPage": 0
			}`)
	})

	t.Run("Add Attributes", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		input := h.Data.User()
		input.DisplayName = ""
		created := h.MustCreateUser(input)
		h.PatchUser(created.ID, PatchOp{Op: "Add", Path: "displayName", Value: toJSONObject(t, "Added Display")}).
			RequireSuccess()
		h.ListUsers(ListOpts{Filter: filterExpr("userName", "eq", input.UserName)}).
			RequirePath("Resources.0.displayName", "Added Display")
	})

	t.Run("Replace User Attributes including multi-value", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		input := h.Data.User()
		created := h.MustCreateUser(input)
		h.PatchUser(created.ID, []PatchOp{
			{Op: "Replace", Path: `emails[type eq "work"].value`, Value: toJSONObject(t, "updatedEmail@microsoft.com")},
			{Op: "Replace", Path: "name.familyName", Value: toJSONObject(t, "updatedFamilyName")},
		}).RequireSuccess()
		h.ListUsers(ListOpts{Filter: filterExpr("userName", "eq", input.UserName)}).
			RequirePath("Resources.0.name.familyName", "updatedFamilyName").
			RequirePath("Resources.0.emails.0.value", "updatedEmail@microsoft.com")
	})

	t.Run("PATCH op values are case insensitive", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		created := h.MustCreateUser(h.Data.User())
		h.PatchUser(created.ID, PatchOp{Op: "Replace", Path: "active", Value: json.RawMessage(`false`)}).
			RequireSuccess()
		h.GetUser(created.ID).RequireJSONContains(`{"active":false}`)
		h.PatchUser(created.ID, PatchOp{Op: "ADD", Path: "displayName", Value: toJSONObject(t, "Azure Cased")}).
			RequireSuccess()
		h.GetUser(created.ID).RequireJSONContains(`{"displayName":"Azure Cased"}`)
	})

	t.Run("Update Joining Property", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		input := h.Data.User()
		created := h.MustCreateUser(input)
		newName := "updated-" + input.UserName
		h.PatchUser(created.ID, PatchOp{Op: "Replace", Path: "userName", Value: toJSONObject(t, newName)}).
			RequireSuccess()
		h.ListUsers(ListOpts{Filter: filterExpr("userName", "eq", newName)}).
			RequirePath("totalResults", 1).
			RequirePath("Resources.0.id", created.ID)
	})

	t.Run("Update Active Attribute to False", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		input := h.Data.User()
		created := h.MustCreateUser(input)
		h.PatchUser(created.ID, PatchOp{Op: "Replace", Path: "active", Value: json.RawMessage(`false`)}).
			RequireSuccess()
		h.ListUsers(ListOpts{Filter: filterExpr("userName", "eq", input.UserName)}).
			RequirePath("Resources.0.active", false)
		h.GetUser(created.ID).RequireJSONContains(`{"active":false}`)
	})

	t.Run("Create New Group then filter then delete", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		input := h.Data.Group()
		group := h.CreateGroup(input).
			RequireStatus(http.StatusCreated).
			RequireJSONContains(map[string]any{"displayName": input.DisplayName}).
			Group()
		h.ListGroups(ListOpts{Filter: filterExpr("displayName", "eq", input.DisplayName)}).
			RequireStatus(http.StatusOK).
			RequirePath("totalResults", 1).
			RequirePath("Resources.0.id", group.ID)
		h.DeleteGroup(group.ID).RequireStatus(http.StatusNoContent)
	})

	t.Run("Create Duplicate Group returns 409", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		input := h.Data.Group()
		h.CreateGroup(input).RequireStatus(http.StatusCreated)
		h.CreateGroup(input).RequireError(http.StatusConflict, "uniqueness")
	})

	t.Run("Get Group excludedAttributes members", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		user := h.MustCreateUser(h.Data.User())
		input := h.Data.Group()
		input.Members = []GroupMember{{Value: user.ID}}
		group := h.MustCreateGroup(input)
		h.GetGroup(group.ID).RequirePath("members.0.value", user.ID)
		h.GetGroup(group.ID, ListOpts{ExcludedAttributes: []string{"members"}}).
			RequireStatus(http.StatusOK).
			RequireMissing("members")
		h.ListGroups(ListOpts{Filter: filterExpr("displayName", "eq", input.DisplayName)}).
			RequireStatus(http.StatusOK).
			RequirePath("Resources.0.members.0.value", user.ID)
		h.ListGroups(ListOpts{
			Filter:             filterExpr("displayName", "eq", input.DisplayName),
			ExcludedAttributes: []string{"members"},
		}).
			RequireStatus(http.StatusOK).
			RequirePath("totalResults", 1).
			RequireMissing("Resources.0.members")
	})

	t.Run("Update Group non-member attributes", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		input := h.Data.Group()
		group := h.MustCreateGroup(input)
		newName := input.DisplayName + "-updated"
		h.PatchGroup(group.ID, PatchOp{Op: "Replace", Path: "displayName", Value: toJSONObject(t, newName)}).
			RequireSuccess()
		h.ListGroups(ListOpts{Filter: filterExpr("displayName", "eq", newName)}).
			RequirePath("totalResults", 1).
			RequirePath("Resources.0.displayName", newName)
	})

	t.Run("Update Group add and remove members", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		user := h.MustCreateUser(h.Data.User())
		group := h.MustCreateGroup(h.Data.Group())
		h.PatchGroup(group.ID, toJSONObject(t, map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{{
				"op":   "Add",
				"path": "members",
				"value": []map[string]any{{
					"$ref":  nil,
					"value": user.ID,
				}},
			}},
		})).RequireSuccess()
		h.GetGroup(group.ID).RequirePath("members.0.value", user.ID)

		h.PatchGroup(group.ID, toJSONObject(t, map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{{
				"op":   "Remove",
				"path": "members",
				"value": []map[string]any{{
					"$ref":  nil,
					"value": user.ID,
				}},
			}},
		})).RequireSuccess()
		require.Empty(t, h.GetGroup(group.ID).RequireStatus(http.StatusOK).Group().Members)
	})
}
