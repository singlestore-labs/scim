package e2e

import (
	"testing"
)

func TestRequireJSONContainsIgnoresExtraFields(t *testing.T) {
	t.Parallel()
	result := Result{
		t:    t,
		Body: []byte(`{"userName":"a@x","active":true,"id":"generated","meta":{"created":"now"}}`),
	}

	result.RequireJSONContains(`{"userName":"a@x","active":true}`)
}

func TestRequireJSONContainsNestedObject(t *testing.T) {
	t.Parallel()
	result := Result{
		t:    t,
		Body: []byte(`{"name":{"givenName":"Ada","familyName":"Lovelace","formatted":"Ada Lovelace"}}`),
	}

	result.RequireJSONContains(`{"name":{"familyName":"Lovelace"}}`)
}

func TestRequireJSONContainsArrayExact(t *testing.T) {
	t.Parallel()
	result := Result{
		t:    t,
		Body: []byte(`{"emails":[{"value":"a@x","type":"work"}]}`),
	}

	result.RequireJSONContains(`{"emails":[{"value":"a@x","type":"work"}]}`)
}

func TestRequirePathVariants(t *testing.T) {
	t.Parallel()
	result := Result{
		t:    t,
		Body: []byte(`{"totalResults":1,"Resources":[{"userName":"a@x"}]}`),
	}

	result.RequirePath("totalResults", 1)
	result.RequirePath("Resources.0.userName", "a@x")
	result.RequirePath("Resources[0].userName", "a@x")
	result.RequirePath("$.Resources.0.userName", "a@x")
}

func TestRequireJSONEqWholeDocument(t *testing.T) {
	t.Parallel()
	result := Result{
		t:    t,
		Body: []byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:Error"],"status":400,"scimType":"invalidFilter","detail":"x"}`),
	}

	result.RequireJSONEq(`{
		"status": 400,
		"scimType": "invalidFilter",
		"detail": "x",
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:Error"]
	}`)
}
