package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAzureEntraSCIMValidator(t *testing.T) {
	t.Parallel()
	// Sources:
	// https://learn.microsoft.com/en-us/entra/identity/app-provisioning/scim-validator-tutorial
	// https://learn.microsoft.com/en-us/entra/identity/app-provisioning/use-scim-to-provision-users-and-groups
	//
	// Generic SCIM CRUD, filtering, pagination, errors, PATCH casing, and
	// attribute selection are covered in scim_e2e_test.go. These cases exercise
	// Entra-specific PATCH payloads.

	t.Run("filtered add creates a missing multi-value element", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		input := h.Data.User()
		input.Emails = nil
		created := h.MustCreateUser(input)
		h.PatchUser(created.ID, PatchOp{
			Op:    "Add",
			Path:  `emails[type eq "work"].value`,
			Value: toJSONObject(t, "entra@example.test"),
		}).RequireSuccess()
		h.GetUser(created.ID).
			RequirePath("emails.0.type", "work").
			RequirePath("emails.0.value", "entra@example.test")
	})

	t.Run("group member add and remove accepts null $ref", func(t *testing.T) {
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
