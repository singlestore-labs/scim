package e2e

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var jsonPathIndex = regexp.MustCompile(`\[(\d+)\]`)

// RequireStatus fails unless the HTTP status matches.
func (r Result) RequireStatus(status int) Result {
	r.t.Helper()
	require.Equal(r.t, status, r.Status, r.String())
	return r
}

// RequireSuccess fails unless the status is 2xx. Use for PATCH, where 200 and 204 are both valid.
func (r Result) RequireSuccess() Result {
	r.t.Helper()
	require.GreaterOrEqual(r.t, r.Status, 200, r.String())
	require.Less(r.t, r.Status, 300, r.String())
	return r
}

// RequireJSONContains asserts that every key/value in expected appears in the
// response body. Extra fields on the server (id, meta, groups) are ignored.
// Nested objects are matched the same way. Arrays at a given path must match exactly.
func (r Result) RequireJSONContains(expected any) Result {
	r.t.Helper()
	want := unmarshalJSONValue(r.t, expected)
	got := unmarshalJSONValue(r.t, r.Body)
	requireJSONContains(r.t, want, got, "$", r.String())
	return r
}

// RequirePath asserts one value at a dotted JSON path.
// Examples: "userName", "Resources.0.userName", "$.totalResults", "Resources[0].id".
func (r Result) RequirePath(path string, expected any) Result {
	r.t.Helper()
	got, err := lookupJSON(unmarshalJSONValue(r.t, r.Body), path)
	require.NoError(r.t, err, r.String())
	require.Equal(r.t, unmarshalJSONValue(r.t, expected), got, "path %s in %s", path, r.String())
	return r
}

// RequireJSONEq compares the entire response body with expected using testify JSONEq.
// Use only for frozen documents with no generated fields (typical: empty list, tiny errors).
func (r Result) RequireJSONEq(expected any) Result {
	r.t.Helper()
	require.JSONEq(r.t, string(marshalExpectedJSON(r.t, expected)), string(r.Body), r.String())
	return r
}

func (r Result) User() User {
	r.t.Helper()
	return decode[User](r.t, r)
}

func (r Result) UserList() UserList {
	r.t.Helper()
	return decode[UserList](r.t, r)
}

func (r Result) Group() Group {
	r.t.Helper()
	return decode[Group](r.t, r)
}

func (r Result) GroupList() GroupList {
	r.t.Helper()
	return decode[GroupList](r.t, r)
}

func (r Result) RequireError(status int, scimType string) SCIMError {
	r.t.Helper()
	r.RequireStatus(status)
	want := map[string]any{
		"schemas": []any{"urn:ietf:params:scim:api:messages:2.0:Error"},
		"status":  status,
	}
	if scimType != "" {
		want["scimType"] = scimType
	}
	r.RequireJSONContains(want)
	return decode[SCIMError](r.t, r)
}

func requireJSONContains(t *testing.T, want, got any, path, response string) {
	t.Helper()
	switch wantValue := want.(type) {
	case map[string]any:
		gotObject, ok := got.(map[string]any)
		require.Truef(t, ok, "%s: expected JSON object, got %T (%s)", path, got, response)
		for key, wantChild := range wantValue {
			gotChild, ok := gotObject[key]
			require.Truef(t, ok, "%s: missing key %q (%s)", path, key, response)
			requireJSONContains(t, wantChild, gotChild, path+"."+key, response)
		}
	case []any:
		wantJSON, err := json.Marshal(wantValue)
		require.NoError(t, err)
		gotJSON, err := json.Marshal(got)
		require.NoError(t, err)
		require.JSONEq(t, string(wantJSON), string(gotJSON), "%s array (%s)", path, response)
	default:
		require.Equalf(t, want, got, "%s (%s)", path, response)
	}
}

func unmarshalJSONValue(t *testing.T, value any) any {
	t.Helper()
	var parsed any
	require.NoError(t, json.Unmarshal(marshalExpectedJSON(t, value), &parsed))
	return parsed
}

func marshalExpectedJSON(t *testing.T, value any) []byte {
	t.Helper()
	switch typed := value.(type) {
	case json.RawMessage:
		return typed
	case []byte:
		return typed
	case string:
		if json.Valid([]byte(typed)) {
			return []byte(typed)
		}
		encoded, err := json.Marshal(typed)
		require.NoError(t, err)
		return encoded
	default:
		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		return encoded
	}
}

func lookupJSON(root any, path string) (any, error) {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "$.")
	if path == "$" {
		return root, nil
	}
	path = jsonPathIndex.ReplaceAllString(path, ".$1")
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return root, nil
	}

	current := root
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			continue
		}
		switch node := current.(type) {
		case map[string]any:
			next, ok := node[part]
			if !ok {
				return nil, fmt.Errorf("json path: key %q not found", part)
			}
			current = next
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("json path: %q is not an array index", part)
			}
			if index < 0 || index >= len(node) {
				return nil, fmt.Errorf("json path: index %d out of range", index)
			}
			current = node[index]
		default:
			return nil, fmt.Errorf("json path: cannot descend into %T at %q", current, part)
		}
	}
	return current, nil
}
