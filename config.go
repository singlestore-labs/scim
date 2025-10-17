package scimprotocol

import (
	"encoding/json"
)

type Config struct {
	ItemsPerPage          int
	PatchSupported        bool
	BulkSupported         bool
	BulkMaxOperations     int
	BulkMaxPayloadSize    int
	FilterSupported       bool
	FilterMaxResult       int
	ChangePassword        bool
	SortSupported         bool
	EtagSupported         bool
	AuthenticationSchemas []AuthenticationScheme
}

func (r Config) MarshalSCIM(reqURL string) ([]byte, error) {
	authJson, err := json.Marshal(r.AuthenticationSchemas)
	if err != nil {
		return nil, err
	}
	outputFormat := map[string]any{
		"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		// TODO: replace with single store scim doc
		"documentationUri": "http://example.com/help/scim.html",
		"patch": map[string]any{
			"supported": r.PatchSupported,
		},
		"bulk": map[string]any{
			"supported":      r.BulkSupported,
			"maxOperations":  r.BulkMaxOperations,
			"maxPayloadSize": r.BulkMaxPayloadSize,
		},
		"filter": map[string]any{
			"supported":  r.FilterSupported,
			"maxResults": r.FilterMaxResult,
		},
		"changePassword": map[string]any{
			"supported": r.ChangePassword,
		},
		"sort": map[string]any{
			"supported": r.SortSupported,
		},
		"etag": map[string]any{
			"supported": r.EtagSupported,
		},
		"authenticationSchemes": json.RawMessage(authJson),
		"meta": map[string]any{
			"location":     reqURL,
			"resourceType": "ServiceProviderConfig",
			"created":      "2010-01-23T04:56:22Z",
			"lastModified": "2011-05-13T04:42:34Z",
		},
	}
	return json.Marshal(outputFormat)
}

type AuthType string

const AuthTypeOauthBearerToken AuthType = "oauthbearertoken"

type AuthenticationScheme struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	SpecURI          string   `json:"specUri"`
	DocumentationURI string   `json:"documentationUri"`
	Type             AuthType `json:"type"`
	Primary          bool     `json:"primary"`
}
