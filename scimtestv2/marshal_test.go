package scimtestv2

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"singlestore.com/helios/scim/scimmodelsv2"
	"singlestore.com/helios/scim/scimprotocol"
	"singlestore.com/helios/scim/scimprotocol/util"
)

func TestSCIMMarshalJSON(t *testing.T) {
	t.Parallel()
	userJson, err := scimprotocol.Marshal([]byte(`{"t":"v"}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"t":"v"}`, string(userJson), "should match marshal json")
}

func TestSCIMMarshalUserWithoutExtension(t *testing.T) {
	t.Parallel()
	testUser := ExampleUserCore
	testJSON := ExampleUserCoreJSON
	userJson, err := scimprotocol.Marshal(testUser)
	require.NoError(t, err)
	require.JSONEq(t, string(util.PrettyJSON(t, testJSON)), string(util.PrettyJSON(t, userJson)), "should match marshal json")
}

func TestSCIMUnmarshalUserWithoutExtension(t *testing.T) {
	t.Parallel()
	var user scimmodelsv2.SCIMUser
	err := scimprotocol.Unmarshal(ExampleUserCoreJSON, &user)
	require.NoError(t, err)
	expect := ExampleUserCore
	expect.ID = ""
	require.Equal(t, expect, user)
}

func TestSCIMUnmarshalUserWithExtension(t *testing.T) {
	t.Parallel()
	{
		t.Log("test unmarshal required")
		var user scimmodelsv2.SCIMUser
		err := scimprotocol.Unmarshal(replaceFieldFromJSON(t, ExampleUserWithExtensionJSON, "userName", nil), &user)
		require.Error(t, err)
	}
	{
		t.Log("test marshal with extra data")
		var user scimmodelsv2.SCIMUser
		err := scimprotocol.Unmarshal(ExampleFullUserJSON, &user)
		require.NoError(t, err)
		expect := ExampleUserWithExtension
		expect.ID = ""
		require.Equal(t, expect, user)
	}
}

func TestResourceTypeMarhsal(t *testing.T) {
	t.Parallel()
	expectJSONResourceType := []byte(`
	{
		"schemas": [
			"urn:ietf:params:scim:schemas:core:2.0:ResourceType"
		],
		"id": "User",
		"name": "User",
		"endpoint": "/User",
		"meta": {
			"location": "https://example.com/v2/ResourceTypes/User",
			"resourceType": "ResourceType"
		},
		"schema": "urn:ietf:params:scim:schemas:core:2.0:User"
	}`)
	userResourceType := scimprotocol.ResourceType{
		ID:                 "User",
		Name:               "User",
		Endpoint:           "/User",
		ResourceObjectType: reflect.TypeOf(scimmodelsv2.SCIMUser{}), // no need content
		Meta: scimprotocol.ResourceTypeMeta{
			Location: "https://example.com/v2/ResourceTypes/User",
		},
	}
	jsonUserResourceType, err := scimprotocol.Marshal(userResourceType)
	require.NoError(t, err)
	require.JSONEq(t, string(util.PrettyJSON(t, expectJSONResourceType)), string(util.PrettyJSON(t, jsonUserResourceType)))
}

func TestSCIMSelectMarshalUserWithExtension(t *testing.T) {
	t.Parallel()
	testUser := ExampleUserWithExtension
	attributes := []string{"userName", "name.givenName", "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User:employeeNumber", "emails.value"}
	userJson, err := scimprotocol.ResourceMarshal(testUser, attributes, false)
	require.NoError(t, err)
	require.Equal(t, string(util.PrettyJSON(t, []byte(`{
        	"emails": [
        		{
        			"value": "bjensen@example.com"
        		},
        		{
        			"value": "babs@jensen.org"
        		}
        	],
        	"id": "2819c223-7f76-453a-919d-413861904646",
			"meta": {
        		"created": "0001-01-01T00:00:00Z",
        		"lastModified": "0001-01-01T00:00:00Z"
        	},
        	"name": {
        		"givenName": "Barbara"
        	},
        	"schemas": [
        		"urn:ietf:params:scim:schemas:core:2.0:User"
        	],
        	"userName": "bjensen@example.com"
        }`))), string(util.PrettyJSON(t, userJson)), "should match marshal json")
}

func TestSCIMSelectMarshalUserWithoutExtension(t *testing.T) {
	t.Parallel()
	testUser := ExampleUserWithExtension
	attributes := []string{"userName", "name.givenName", "emails.value"}
	userJson, err := scimprotocol.ResourceMarshal(testUser, attributes, false)
	require.NoError(t, err)
	require.JSONEq(t, string(util.PrettyJSON(t, []byte(`{
			"id": "2819c223-7f76-453a-919d-413861904646",
        	"emails": [
        		{
        			"value": "bjensen@example.com"
        		},
        		{
        			"value": "babs@jensen.org"
        		}
        	],
			"meta": {
        		"created": "0001-01-01T00:00:00Z",
        		"lastModified": "0001-01-01T00:00:00Z"
        	},
        	"name": {
        		"givenName": "Barbara"
        	},
        	"schemas": [
        		"urn:ietf:params:scim:schemas:core:2.0:User"
        	],
        	"userName": "bjensen@example.com"
        }`))), string(util.PrettyJSON(t, userJson)), "should match marshal json")
}

func TestSCIMExcludeSelectMarshalUserWithExtension(t *testing.T) {
	t.Parallel()
	testUser := ExampleUserWithExtension
	attributes := []string{"userName", "name.givenName", "emails.value"}
	userJson, err := scimprotocol.ResourceMarshal(testUser, attributes, true)
	require.NoError(t, err)
	require.Equal(t, string(util.PrettyJSON(t, []byte(`{
        	"active": true,
        	"displayName": "Babs Jensen",
        	"emails": [
        		{
        			"primary": true,
        			"type": "work"
        		},
        		{
        			"primary": false,
        			"type": "home"
        		}
        	],
        	"externalId": "701984",
        	"groups": [
        		{
        			"$ref": "https://example.com/v2/Groups/e9e30dba-f08f-4109-8486-d5c6a331660a",
        			"display": "Tour Guides"
        		},
        		{
        			"$ref": "https://example.com/v2/Groups/fc348aa8-3835-40eb-a20b-c726e15c55b5",
        			"display": "Employees"
        		},
        		{
        			"$ref": "https://example.com/v2/Groups/71ddacd2-a8e7-49b8-a5db-ae50d0a5bfd7",
        			"display": "US Employees"
        		}
        	],
        	"id": "2819c223-7f76-453a-919d-413861904646",
			"meta": {
        		"created": "0001-01-01T00:00:00Z",
        		"lastModified": "0001-01-01T00:00:00Z"
        	},
        	"name": {
        		"familyName": "Jensen",
        		"formatted": "Ms. Barbara J Jensen, III",
        		"honorificPrefix": "Ms.",
        		"honorificSuffix": "III",
        		"middleName": "Jane"
        	},
        	"schemas": [
        		"urn:ietf:params:scim:schemas:core:2.0:User"
        	],
        	"timezone": "America/Los_Angeles",
        	"title": "Tour Guide",
        	"userType": "Employee"
        }`))), string(util.PrettyJSON(t, userJson)), "should match marshal json")
}
