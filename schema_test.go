package scimprotocol_test

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	scimprotocol "github.com/singlestore-labs/scim"
	"github.com/singlestore-labs/scim/scimtag"
	"github.com/singlestore-labs/scim/util"
	"github.com/stretchr/testify/require"
)

/*
https://datatracker.ietf.org/doc/html/rfc7643#section-8.7
*/
func init() {
	scimtag.BuildAllSCIMCharacsCache(exampleResource{})
}

type exampleResource struct {
	CoreUser                      `scim:"urn:ietf:params:scim:schemas:core:2.0:User"`
	scimprotocol.ResourceTypeMeta `scim:"meta"`
}

type CoreUser struct {
	UserName string       `scim:"userName,required,caseExact"`
	Emails   []Multivalue `scim:"emails"`
}

type Multivalue struct {
	Value string `scim:"value"`
}

func TestGetSchema(t *testing.T) {
	t.Parallel()
	schemas, err := scimprotocol.GetResourceSchema(reflect.TypeOf(exampleResource{}))
	require.NoError(t, err)
	require.ElementsMatch(t, []scimprotocol.AttributeSchema{
		{
			Name:        "userName",
			Type:        "string",
			MultiValued: false,
			Required:    true,
			CaseExact:   util.ToPtr(true),
			Mutability:  scimtag.ReadWrite,
			Returned:    scimtag.Default,
			Uniqueness:  util.ToPtr("none"),
		},
		{
			Name:        "emails",
			Type:        "complex",
			MultiValued: true,
			Mutability:  scimtag.ReadWrite, // azure required
			Required:    false,
			Uniqueness:  util.ToPtr("none"),
			SubAttributes: []scimprotocol.AttributeSchema{
				{
					Name:        "value",
					Type:        "string",
					MultiValued: false,
					Required:    false,
					CaseExact:   util.ToPtr(false),
					Mutability:  scimtag.ReadWrite,
					Returned:    scimtag.Default,
					Uniqueness:  util.ToPtr("none"),
				},
			},
		},
	}, schemas[0].Attributes)
	require.Equal(t, "urn:ietf:params:scim:schemas:core:2.0:User", schemas[0].ID)

	schemaResponse := scimprotocol.ListResponse[scimprotocol.Schema]{
		Resources:    schemas,
		StartIndex:   1,
		ItemsPerPage: 20,
		TotalResults: len(schemas),
	}
	respJSON, err := scimprotocol.Marshal(schemaResponse)
	require.NoError(t, err)
	require.JSONEq(t, string(sortSchemasJSON(t, []byte(expectSchemaResponseJSON))), string(sortSchemasJSON(t, respJSON)))
}

// sortSchemasJSON used to sort json array because assert.JSONEq not support un-ordered json array
func sortSchemasJSON(t *testing.T, input []byte) []byte {
	obj := map[string]any{}
	err := json.Unmarshal(input, &obj)
	require.NoError(t, err)
	untypedResources, ok := obj["Resources"]
	require.True(t, ok)

	listResources, ok := untypedResources.([]any)
	require.True(t, ok)
	for _, ur := range listResources {
		r, ok := ur.(map[string]any)
		require.True(t, ok, ur)
		attributesList, ok := r["attributes"].([]any)
		require.True(t, ok)

		sort.Slice(attributesList, func(i, j int) bool {
			return attributesList[i].(map[string]any)["name"].(string) < attributesList[j].(map[string]any)["name"].(string)
		})
		r["attributes"] = attributesList
	}
	sort.Slice(listResources, func(i, j int) bool {
		return listResources[i].(map[string]any)["id"].(string) < listResources[j].(map[string]any)["id"].(string)
	})
	obj["Resources"] = listResources
	result, err := json.Marshal(obj)
	require.NoError(t, err)
	return result
}

var expectSchemaResponseJSON = `{
  "schemas": [
    "urn:ietf:params:scim:api:messages:2.0:ListResponse"
  ],
  "totalResults": 1,
  "Resources": [
    {
      "id": "urn:ietf:params:scim:schemas:core:2.0:User",
      "attributes": [
        {
          "name": "userName",
          "type": "string",
          "multiValued": false,
          "required": true,
          "caseExact": true,
		  "mutability": "readWrite",
          "returned": "default",
          "uniqueness": "none"
        },
        {
          "name": "emails",
          "type": "complex",
          "multiValued": true,
          "required": false,
		  "mutability": "readWrite",
          "returned": "default",
          "uniqueness": "none",
          "subAttributes": [
            {
              "name": "value",
              "type": "string",
              "multiValued": false,
              "required": false,
              "caseExact": false,
			  "mutability": "readWrite",
          	  "returned": "default",
              "uniqueness": "none"
            }
          ]
        }
      ]
    }
  ],
  "startIndex": 1,
  "itemsPerPage": 20
}`
