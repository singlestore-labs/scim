package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func eqFilter(attribute, value string) string {
	return fmt.Sprintf(`%s eq "%s"`, attribute, value)
}

func jsonObject(v any) json.RawMessage {
	encoded, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return encoded
}

func TestOktaSCIM20Suite(t *testing.T) {
	t.Parallel()
	// Sources:
	// https://developer.okta.com/docs/api/openapi/okta-scim/guides/scim-20
	// https://developer.okta.com/docs/guides/scim-provisioning-integration-test/main/
	// https://developer.okta.com/docs/guides/scim-provisioning-integration-prepare/main/

	t.Run("GET Users pagination for connection and import", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
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
		h := NewHarness(t)
		h.ListGroups(ListOpts{StartIndex: 1, Count: intPointer(100)}).
			RequireStatus(http.StatusOK).
			RequirePath("totalResults", 0).
			RequireJSONContains(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:ListResponse"],"Resources":[]}`)
	})

	t.Run("CRUD user lifecycle", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
		input := h.Data.User()
		input.ExternalID = "okta-ext-" + input.UserName

		created := h.CreateUser(input).
			RequireStatus(http.StatusCreated).
			RequireJSONContains(fmt.Sprintf(`{"userName":%q,"displayName":%q,"active":true}`, input.UserName, input.DisplayName)).
			User()

		h.ListUsers(ListOpts{Filter: eqFilter("userName", input.UserName), StartIndex: 1, Count: intPointer(100)}).
			RequireStatus(http.StatusOK).
			RequirePath("totalResults", 1).
			RequirePath("Resources.0.id", created.ID).
			RequirePath("Resources.0.userName", input.UserName)

		h.GetUser(created.ID).
			RequireStatus(http.StatusOK).
			RequireJSONContains(fmt.Sprintf(`{"id":%q,"userName":%q}`, created.ID, input.UserName))

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
		h.ListUsers(ListOpts{Filter: eqFilter("userName", input.UserName)}).
			RequirePath("Resources.0.active", false)
	})

	t.Run("PUT then GET user as Okta custom apps do", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
		input := h.Data.User()
		created := h.MustCreateUser(input)
		input.DisplayName = "Renamed by PUT"
		h.GetUser(created.ID).RequireStatus(http.StatusOK)
		h.ReplaceUser(created.ID, input).RequireStatus(http.StatusOK)
		h.GetUser(created.ID).RequireJSONContains(`{"displayName":"Renamed by PUT"}`)
	})

	t.Run("group create filter get rename members delete", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
		user := h.MustCreateUser(h.Data.User())
		input := h.Data.Group()

		group := h.CreateGroup(input).
			RequireStatus(http.StatusCreated).
			RequireJSONContains(fmt.Sprintf(`{"displayName":%q}`, input.DisplayName)).
			Group()

		h.ListGroups(ListOpts{Filter: eqFilter("displayName", input.DisplayName), StartIndex: 1, Count: intPointer(100)}).
			RequireStatus(http.StatusOK).
			RequirePath("totalResults", 1).
			RequirePath("Resources.0.id", group.ID)

		h.GetGroup(group.ID).
			RequireStatus(http.StatusOK).
			RequireJSONContains(fmt.Sprintf(`{"id":%q,"displayName":%q}`, group.ID, input.DisplayName))

		h.PatchGroup(group.ID, PatchOp{Op: "replace", Path: "displayName", Value: jsonObject("Okta Renamed Group")}).
			RequireSuccess()
		h.GetGroup(group.ID).RequireJSONContains(`{"displayName":"Okta Renamed Group"}`)

		input.DisplayName = "Okta PUT Group"
		h.ReplaceGroup(group.ID, input).RequireStatus(http.StatusOK)
		h.GetGroup(group.ID).RequireJSONContains(`{"displayName":"Okta PUT Group"}`)

		h.PatchGroup(group.ID, PatchOp{
			Op:    "add",
			Path:  "members",
			Value: jsonObject([]map[string]string{{"value": user.ID}}),
		}).RequireSuccess()
		h.GetGroup(group.ID).RequirePath("members.0.value", user.ID)

		h.PatchGroup(group.ID, PatchOp{
			Op:   "remove",
			Path: fmt.Sprintf(`members[value eq "%s"]`, user.ID),
		}).RequireSuccess()

		h.DeleteGroup(group.ID).RequireStatus(http.StatusNoContent)
	})
}

func TestAzureEntraSCIMValidator(t *testing.T) {
	t.Parallel()
	// Sources:
	// https://learn.microsoft.com/en-us/entra/identity/app-provisioning/scim-validator-tutorial
	// https://learn.microsoft.com/en-us/entra/identity/app-provisioning/use-scim-to-provision-users-and-groups

	t.Run("Create New User then filter then delete", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
		input := h.Data.User()
		created := h.CreateUser(input).
			RequireStatus(http.StatusCreated).
			RequireJSONContains(fmt.Sprintf(`{"userName":%q,"active":true}`, input.UserName)).
			User()
		h.ListUsers(ListOpts{Filter: eqFilter("userName", input.UserName)}).
			RequireStatus(http.StatusOK).
			RequirePath("totalResults", 1).
			RequirePath("Resources.0.id", created.ID).
			RequirePath("Resources.0.userName", input.UserName)
		h.DeleteUser(created.ID).RequireStatus(http.StatusNoContent)
	})

	t.Run("Create Duplicate User returns 409", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
		input := h.Data.User()
		h.CreateUser(input).RequireStatus(http.StatusCreated)
		h.CreateUser(input).RequireError(http.StatusConflict, "uniqueness")
	})

	t.Run("Get User found and not found", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
		created := h.MustCreateUser(h.Data.User())
		h.GetUser(created.ID).
			RequireStatus(http.StatusOK).
			RequireJSONContains(fmt.Sprintf(`{"id":%q,"userName":%q}`, created.ID, created.UserName))
		h.GetUser("missing-user-id").RequireError(http.StatusNotFound, "")
	})

	t.Run("Get User by query zero results", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
		h.ListUsers(ListOpts{Filter: eqFilter("userName", "non-existent user")}).
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
		h := NewHarness(t)
		input := h.Data.User()
		input.DisplayName = ""
		created := h.MustCreateUser(input)
		h.PatchUser(created.ID, PatchOp{Op: "Add", Path: "displayName", Value: jsonObject("Added Display")}).
			RequireSuccess()
		h.ListUsers(ListOpts{Filter: eqFilter("userName", input.UserName)}).
			RequirePath("Resources.0.displayName", "Added Display")
	})

	t.Run("Replace User Attributes including multi-value", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
		input := h.Data.User()
		created := h.MustCreateUser(input)
		h.PatchUser(created.ID, []PatchOp{
			{Op: "Replace", Path: `emails[type eq "work"].value`, Value: jsonObject("updatedEmail@microsoft.com")},
			{Op: "Replace", Path: "name.familyName", Value: jsonObject("updatedFamilyName")},
		}).RequireSuccess()
		h.ListUsers(ListOpts{Filter: eqFilter("userName", input.UserName)}).
			RequirePath("Resources.0.name.familyName", "updatedFamilyName").
			RequirePath("Resources.0.emails.0.value", "updatedEmail@microsoft.com")
	})

	t.Run("Update Joining Property", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
		input := h.Data.User()
		created := h.MustCreateUser(input)
		newName := "updated-" + input.UserName
		h.PatchUser(created.ID, PatchOp{Op: "Replace", Path: "userName", Value: jsonObject(newName)}).
			RequireSuccess()
		h.ListUsers(ListOpts{Filter: eqFilter("userName", newName)}).
			RequirePath("totalResults", 1).
			RequirePath("Resources.0.id", created.ID)
	})

	t.Run("Update Active Attribute to False", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
		input := h.Data.User()
		created := h.MustCreateUser(input)
		h.PatchUser(created.ID, PatchOp{Op: "Replace", Path: "active", Value: json.RawMessage(`false`)}).
			RequireSuccess()
		h.ListUsers(ListOpts{Filter: eqFilter("userName", input.UserName)}).
			RequirePath("Resources.0.active", false)
		h.GetUser(created.ID).RequireJSONContains(`{"active":false}`)
	})

	t.Run("Create New Group then filter then delete", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
		input := h.Data.Group()
		group := h.CreateGroup(input).
			RequireStatus(http.StatusCreated).
			RequireJSONContains(fmt.Sprintf(`{"displayName":%q}`, input.DisplayName)).
			Group()
		h.ListGroups(ListOpts{Filter: eqFilter("displayName", input.DisplayName)}).
			RequireStatus(http.StatusOK).
			RequirePath("totalResults", 1).
			RequirePath("Resources.0.id", group.ID)
		h.DeleteGroup(group.ID).RequireStatus(http.StatusNoContent)
	})

	t.Run("Create Duplicate Group returns 409", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
		input := h.Data.Group()
		h.CreateGroup(input).RequireStatus(http.StatusCreated)
		h.CreateGroup(input).RequireError(http.StatusConflict, "uniqueness")
	})

	t.Run("Get Group excludedAttributes members", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
		user := h.MustCreateUser(h.Data.User())
		input := h.Data.Group()
		input.Members = []GroupMember{{Value: user.ID}}
		group := h.MustCreateGroup(input)
		h.GetGroup(group.ID).RequirePath("members.0.value", user.ID)
		h.ListGroups(ListOpts{
			Filter:             eqFilter("displayName", input.DisplayName),
			ExcludedAttributes: []string{"members"},
		}).RequireStatus(http.StatusOK).RequirePath("totalResults", 1)
	})

	t.Run("Update Group non-member attributes", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
		input := h.Data.Group()
		group := h.MustCreateGroup(input)
		newName := input.DisplayName + "-updated"
		h.PatchGroup(group.ID, PatchOp{Op: "Replace", Path: "displayName", Value: jsonObject(newName)}).
			RequireSuccess()
		h.ListGroups(ListOpts{Filter: eqFilter("displayName", newName)}).
			RequirePath("totalResults", 1).
			RequirePath("Resources.0.displayName", newName)
	})

	t.Run("Update Group add and remove members", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
		user := h.MustCreateUser(h.Data.User())
		group := h.MustCreateGroup(h.Data.Group())
		h.PatchGroup(group.ID, json.RawMessage(fmt.Sprintf(`{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{
				"op": "Add",
				"path": "members",
				"value": [{"$ref": null, "value": %q}]
			}]
		}`, user.ID))).RequireSuccess()
		h.GetGroup(group.ID).RequirePath("members.0.value", user.ID)

		h.PatchGroup(group.ID, json.RawMessage(fmt.Sprintf(`{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{
				"op": "Remove",
				"path": "members",
				"value": [{"$ref": null, "value": %q}]
			}]
		}`, user.ID))).RequireSuccess()
	})

	t.Run("schema discovery", func(t *testing.T) {
		t.Parallel()
		h := NewHarness(t)
		h.GetServiceProviderConfig().RequireStatus(http.StatusOK).RequireJSONContains(`{"patch":{"supported":true},"filter":{"supported":true}}`)
		h.GetSchemas().RequireStatus(http.StatusOK)
		h.GetResourceTypes().RequireStatus(http.StatusOK)
	})
}
