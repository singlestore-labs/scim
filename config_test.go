package scimprotocol

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceProviderConfig(t *testing.T) {
	t.Parallel()
	c := Config{
		PatchSupported:     true,
		BulkSupported:      false,
		BulkMaxOperations:  1000,
		BulkMaxPayloadSize: 1048576,
		FilterSupported:    true,
		FilterMaxResult:    200,
		ChangePassword:     false,
		SortSupported:      false,
		EtagSupported:      false,
		AuthenticationSchemas: []AuthenticationScheme{
			{
				Name:             "OAuth Bearer Token",
				Description:      "Authentication scheme using the OAuth Bearer Token Standard",
				SpecURI:          "http://www.rfc-editor.org/info/rfc6750",
				DocumentationURI: "http://example.com/help/oauth.html",
				Type:             AuthTypeOauthBearerToken,
				Primary:          true,
			},
		},
	}
	result, err := c.MarshalSCIM("https://example.com/v2/ServiceProviderConfig")
	require.NoError(t, err)
	require.JSONEq(t,
		`{
		"schemas":
			["urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"],
		"documentationUri": "http://example.com/help/scim.html",
		"patch": {
			"supported":true
		},
		"bulk": {
			"supported":false,
			"maxOperations":1000,
			"maxPayloadSize":1048576
		},
		"filter": {
			"supported":true,
			"maxResults": 200
		},
		"changePassword": {
			"supported":false
		},
		"sort": {
			"supported":false
		},
		"etag": {
			"supported":false
		},
		"authenticationSchemes": [
			{
			"name": "OAuth Bearer Token",
			"description":
				"Authentication scheme using the OAuth Bearer Token Standard",
			"specUri": "http://www.rfc-editor.org/info/rfc6750",
			"documentationUri": "http://example.com/help/oauth.html",
			"type": "oauthbearertoken",
			"primary": true
			}
		],
		"meta": {
			"location": "https://example.com/v2/ServiceProviderConfig",
			"resourceType": "ServiceProviderConfig",
			"created": "2010-01-23T04:56:22Z",
			"lastModified": "2011-05-13T04:42:34Z"
		}
	}`, string(result))
}
