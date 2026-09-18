package e2e

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func filterExpr(attribute, op, value string) string {
	return fmt.Sprintf(`%s %s %q`, attribute, op, value)
}

func toJSONObject(t *testing.T, v any) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(v)
	require.NoError(t, err, "marshal test JSON value")
	return encoded
}

func userInputJSON(input UserInput) map[string]any {
	emails := make([]any, 0, len(input.Emails))
	for _, email := range input.Emails {
		emails = append(emails, map[string]any{
			"value":   email.Value,
			"type":    email.Type,
			"primary": email.Primary,
		})
	}
	return map[string]any{
		"userName":    input.UserName,
		"displayName": input.DisplayName,
		"active":      input.Active,
		"name": map[string]any{
			"givenName":  input.Name.GivenName,
			"familyName": input.Name.FamilyName,
		},
		"emails": emails,
	}
}
