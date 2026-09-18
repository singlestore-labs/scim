package e2e

import (
	"net/http"
	"testing"

	"github.com/singlestore-labs/scim/util"
)

func TestOktaSCIM20Suite(t *testing.T) {
	t.Parallel()
	// Sources:
	// https://developer.okta.com/standards/SCIM/SCIMFiles/Okta-SCIM-20-SPEC-Test.json
	// https://developer.okta.com/docs/api/openapi/okta-scim/guides/scim-20
	// https://developer.okta.com/docs/guides/scim-provisioning-integration-test/main/
	// https://developer.okta.com/docs/guides/scim-provisioning-integration-prepare/main/
	//
	// Generic SCIM CRUD, filtering, pagination, errors, and group behavior are
	// covered in scim_e2e_test.go. These cases preserve Okta-specific request
	// sequences and assertions.

	t.Run("SPEC test lists one complete user for connection", func(t *testing.T) {
		t.Parallel()
		h := NewTestClient(t)
		created := h.MustCreateUser(h.Data.User())

		h.ListUsers(ListOpts{StartIndex: 1, Count: util.ToPtr(1)}).
			RequireStatus(http.StatusOK).
			RequirePath("itemsPerPage", 1).
			RequirePath("startIndex", 1).
			RequirePath("Resources.0.id", created.ID).
			RequirePath("Resources.0.userName", created.UserName).
			RequirePath("Resources.0.name.givenName", created.Name.GivenName).
			RequirePath("Resources.0.name.familyName", created.Name.FamilyName).
			RequirePath("Resources.0.active", true).
			RequirePath("Resources.0.emails.0.value", created.Emails[0].Value)
	})
}
