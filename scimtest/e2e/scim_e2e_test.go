package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/singlestore-labs/scim/util"
	"github.com/stretchr/testify/require"
)

func TestSCIMDiscovery(t *testing.T) {
	t.Parallel()

	client := NewTestClient(t)
	client.GetServiceProviderConfig().
		RequireStatus(http.StatusOK).
		RequireJSONContains(`{"patch":{"supported":true},"filter":{"supported":true},"bulk":{"supported":false}}`)
	client.GetSchemas().
		RequireStatus(http.StatusOK).
		RequireJSONContains(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:ListResponse"]}`)
	client.GetResourceTypes().RequireStatus(http.StatusOK)
}

func TestSCIMUserLifecycle(t *testing.T) {
	t.Parallel()

	client := NewTestClient(t)
	input := client.Data.User()
	want := userInputJSON(input)

	created := client.CreateUser(input).
		RequireStatus(http.StatusCreated).
		RequireJSONContains(want).
		User()
	require.NotEmpty(t, created.ID)

	client.GetUser(created.ID).
		RequireStatus(http.StatusOK).
		RequireJSONContains(want).
		RequirePath("id", created.ID)

	client.ListUsers(ListOpts{Filter: filterExpr("userName", "eq", input.UserName)}).
		RequireStatus(http.StatusOK).
		RequirePath("totalResults", 1).
		RequirePath("Resources.0.id", created.ID)

	input.DisplayName = "Replaced User"
	client.ReplaceUser(created.ID, input).
		RequireStatus(http.StatusOK).
		RequireJSONContains(`{"displayName":"Replaced User"}`)
	client.GetUser(created.ID).RequirePath("displayName", "Replaced User")

	client.DeleteUser(created.ID).RequireStatus(http.StatusNoContent)
	client.GetUser(created.ID).RequireError(http.StatusNotFound, "")
}

func TestSCIMUserUniqueness(t *testing.T) {
	t.Parallel()

	client := NewTestClient(t)
	input := client.Data.User()
	client.CreateUser(input).RequireStatus(http.StatusCreated)
	client.CreateUser(input).RequireError(http.StatusConflict, "uniqueness")
}

func TestSCIMUserPatch(t *testing.T) {
	t.Parallel()

	client := NewTestClient(t)
	input := client.Data.User()
	input.DisplayName = ""
	created := client.MustCreateUser(input)

	client.PatchUser(created.ID, PatchOp{
		Op:    "add",
		Path:  "displayName",
		Value: toJSONObject(t, "Added Display"),
	}).RequireSuccess()
	client.GetUser(created.ID).RequirePath("displayName", "Added Display")

	client.PatchUser(created.ID, []PatchOp{
		{Op: "replace", Path: `emails[type eq "work"].value`, Value: toJSONObject(t, "updated@example.test")},
		{Op: "replace", Path: "name.familyName", Value: toJSONObject(t, "UpdatedFamily")},
	}).RequireSuccess()
	client.GetUser(created.ID).
		RequirePath("name.familyName", "UpdatedFamily").
		RequirePath("emails.0.value", "updated@example.test")

	client.PatchUser(created.ID, PatchOp{
		Op:    "replace",
		Value: json.RawMessage(`{"active":false}`),
	}).RequireSuccess()
	client.GetUser(created.ID).RequirePath("active", false)

	// RFC 7644 section 3.5.2: operation names are case-insensitive.
	client.PatchUser(created.ID, PatchOp{
		Op:    "REPLACE",
		Path:  "active",
		Value: json.RawMessage(`true`),
	}).RequireSuccess()
	client.GetUser(created.ID).RequirePath("active", true)

	client.PatchUser(created.ID, json.RawMessage(`{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{
			"op": "replace",
			"path": "name.givenName",
			"value": "Raw JSON"
		}]
	}`)).RequireSuccess()
	client.GetUser(created.ID).RequirePath("name.givenName", "Raw JSON")
}

func TestSCIMUserFiltering(t *testing.T) {
	t.Parallel()

	client := NewTestClient(t)
	inputs := []UserInput{client.Data.User(), client.Data.User(), client.Data.User()}
	for _, input := range inputs {
		client.MustCreateUser(input)
	}

	client.ListUsers(ListOpts{Filter: filterExpr("userName", "eq", inputs[0].UserName)}).
		RequireStatus(http.StatusOK).
		RequirePath("totalResults", 1).
		RequirePath("Resources.0.userName", inputs[0].UserName)

	// userName is not caseExact, so string comparison is case-insensitive.
	client.ListUsers(ListOpts{Filter: filterExpr("userName", "eq", strings.ToUpper(inputs[0].UserName))}).
		RequireStatus(http.StatusOK).
		RequirePath("totalResults", 1).
		RequirePath("Resources.0.userName", inputs[0].UserName)

	client.ListUsers(ListOpts{
		Filter: filterExpr(`emails[type eq "work"].value`, "eq", inputs[1].Emails[0].Value),
	}).
		RequireStatus(http.StatusOK).
		RequirePath("totalResults", 1).
		RequirePath("Resources.0.userName", inputs[1].UserName)

	for _, op := range []string{"Eq", "EQ"} {
		client.ListUsers(ListOpts{Filter: filterExpr("userName", op, inputs[2].UserName)}).
			RequireStatus(http.StatusOK).
			RequirePath("totalResults", 1)
	}

	client.ListUsers(ListOpts{Filter: filterExpr("userName", "eq", "missing@example.test")}).
		RequireStatus(http.StatusOK).
		RequireJSONEq(`{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:ListResponse"],
			"totalResults": 0,
			"Resources": [],
			"startIndex": 1,
			"itemsPerPage": 0
		}`)

	scimError := client.ListUsers(ListOpts{Filter: `userName eq`}).
		RequireError(http.StatusBadRequest, "invalidFilter")
	require.NotEmpty(t, scimError.Detail)
}

func TestSCIMUserPagination(t *testing.T) {
	t.Parallel()

	client := NewTestClient(t)
	for range 5 {
		client.MustCreateUser(client.Data.User())
	}

	client.ListUsers(ListOpts{StartIndex: 2, Count: util.ToPtr(2)}).
		RequireStatus(http.StatusOK).
		RequirePath("totalResults", 5).
		RequirePath("itemsPerPage", 2).
		RequirePath("startIndex", 2)
}

func TestSCIMGroupLifecycle(t *testing.T) {
	t.Parallel()

	client := NewTestClient(t)
	input := client.Data.Group()
	group := client.CreateGroup(input).
		RequireStatus(http.StatusCreated).
		RequireJSONContains(map[string]any{"displayName": input.DisplayName}).
		Group()

	client.GetGroup(group.ID).
		RequireStatus(http.StatusOK).
		RequireJSONContains(map[string]any{"id": group.ID, "displayName": input.DisplayName})
	client.ListGroups(ListOpts{Filter: filterExpr("displayName", "eq", input.DisplayName)}).
		RequireStatus(http.StatusOK).
		RequirePath("totalResults", 1).
		RequirePath("Resources.0.id", group.ID)

	input.DisplayName += "-replaced"
	client.ReplaceGroup(group.ID, input).
		RequireStatus(http.StatusOK).
		RequirePath("displayName", input.DisplayName)

	client.DeleteGroup(group.ID).RequireStatus(http.StatusNoContent)
	client.GetGroup(group.ID).RequireError(http.StatusNotFound, "")
}

func TestSCIMGroupUniqueness(t *testing.T) {
	t.Parallel()

	client := NewTestClient(t)
	input := client.Data.Group()
	client.CreateGroup(input).RequireStatus(http.StatusCreated)
	client.CreateGroup(input).RequireError(http.StatusConflict, "uniqueness")
}

func TestSCIMGroupMemberPatch(t *testing.T) {
	t.Parallel()

	client := NewTestClient(t)
	user := client.MustCreateUser(client.Data.User())
	group := client.MustCreateGroup(client.Data.Group())

	client.PatchGroup(group.ID, PatchOp{
		Op:    "add",
		Path:  "members",
		Value: toJSONObject(t, []map[string]string{{"value": user.ID}}),
	}).RequireSuccess()
	client.GetGroup(group.ID).RequirePath("members.0.value", user.ID)

	client.PatchGroup(group.ID, PatchOp{
		Op:   "remove",
		Path: `members[value eq "` + user.ID + `"]`,
	}).RequireSuccess()
	require.Empty(t, client.GetGroup(group.ID).RequireStatus(http.StatusOK).Group().Members)
}

func TestSCIMExcludedAttributes(t *testing.T) {
	t.Parallel()

	client := NewTestClient(t)
	user := client.MustCreateUser(client.Data.User())
	input := client.Data.Group()
	input.Members = []GroupMember{{Value: user.ID}}
	group := client.MustCreateGroup(input)

	client.GetGroup(group.ID).RequirePath("members.0.value", user.ID)
	client.GetGroup(group.ID, ListOpts{ExcludedAttributes: []string{"members"}}).
		RequireStatus(http.StatusOK).
		RequireMissing("members")

	client.ListGroups(ListOpts{Filter: filterExpr("displayName", "eq", input.DisplayName)}).
		RequirePath("Resources.0.members.0.value", user.ID)
	client.ListGroups(ListOpts{
		Filter:             filterExpr("displayName", "eq", input.DisplayName),
		ExcludedAttributes: []string{"members"},
	}).
		RequireStatus(http.StatusOK).
		RequirePath("totalResults", 1).
		RequireMissing("Resources.0.members")
}
