package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// WantUser is a partial assertion: nil fields are intentionally ignored.
// Pointers make false and empty-string assertions distinct from "do not check".
type WantUser struct {
	ID          *string
	UserName    *string
	DisplayName *string
	Active      *bool
	GivenName   *string
	FamilyName  *string
	WorkEmail   *string
}

type WantUserList struct {
	TotalResults *int
	ItemsPerPage *int
	StartIndex   *int
	UserNames    []string
}

type WantGroup struct {
	ID          *string
	DisplayName *string
	ExternalID  *string
	MemberIDs   []string
}

func RequireUser(t *testing.T, got User, want WantUser) {
	t.Helper()
	if want.ID != nil {
		require.Equal(t, *want.ID, got.ID)
	}
	if want.UserName != nil {
		require.Equal(t, *want.UserName, got.UserName)
	}
	if want.DisplayName != nil {
		require.Equal(t, *want.DisplayName, got.DisplayName)
	}
	if want.Active != nil {
		require.Equal(t, *want.Active, got.Active)
	}
	if want.GivenName != nil {
		require.Equal(t, *want.GivenName, got.Name.GivenName)
	}
	if want.FamilyName != nil {
		require.Equal(t, *want.FamilyName, got.Name.FamilyName)
	}
	if want.WorkEmail != nil {
		require.Equal(t, *want.WorkEmail, workEmail(got))
	}
}

func RequireUserList(t *testing.T, got UserList, want WantUserList) {
	t.Helper()
	if want.TotalResults != nil {
		require.Equal(t, *want.TotalResults, got.TotalResults)
	}
	if want.ItemsPerPage != nil {
		require.Equal(t, *want.ItemsPerPage, got.ItemsPerPage)
	}
	if want.StartIndex != nil {
		require.Equal(t, *want.StartIndex, got.StartIndex)
	}
	if want.UserNames != nil {
		actual := make([]string, 0, len(got.Resources))
		for _, user := range got.Resources {
			actual = append(actual, user.UserName)
		}
		require.ElementsMatch(t, want.UserNames, actual)
	}
}

func RequireGroup(t *testing.T, got Group, want WantGroup) {
	t.Helper()
	if want.ID != nil {
		require.Equal(t, *want.ID, got.ID)
	}
	if want.DisplayName != nil {
		require.Equal(t, *want.DisplayName, got.DisplayName)
	}
	if want.ExternalID != nil {
		require.Equal(t, *want.ExternalID, got.ExternalID)
	}
	if want.MemberIDs != nil {
		actual := make([]string, 0, len(got.Members))
		for _, member := range got.Members {
			actual = append(actual, member.Value)
		}
		require.ElementsMatch(t, want.MemberIDs, actual)
	}
}

func workEmail(user User) string {
	for _, email := range user.Emails {
		if email.Type == "work" {
			return email.Value
		}
	}
	return ""
}

func pointer[T any](value T) *T {
	return &value
}
