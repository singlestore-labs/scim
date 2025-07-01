package scimtestv2

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/muir/nchi"
	"github.com/stretchr/testify/require"

	"singlestore.com/helios/scim/scimlib"
	"singlestore.com/helios/scim/scimmodels"
	"singlestore.com/helios/scim/scimmodelsv2"
	"singlestore.com/helios/scim/scimprotocol"
	"singlestore.com/helios/scim/scimprotocol/util"
	"singlestore.com/helios/test/di"
	"singlestore.com/helios/trace"
)

/*
This file only test over the server handler functions
*/

type DynamicFields struct {
	ID    string   `json:"id"`
	Group []any    `json:"groups"`
	Meta  SCIMMeta `json:"meta"`
}
type SCIMMeta struct {
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"lastModified"`
}

func TestSCIMServer(t *testing.T) {
	di.IntegrationTest(t,
		di.InjectTimeLimit(300*time.Second),
		TestSCIMConnection,
		getSCIMRouter,
		func(
			scimLib *scimlib.Library,
			tracer trace.Iface,
			scimConn scimmodels.SCIMConnectionGQLOutput,
			r *nchi.Mux,
		) {
			scimID := scimConn.SCIMID.String()
			baseURL := testSCIMEndpoint + "/" + scimID

			t.Logf("base url %s", baseURL)
			t.Log("test post")
			createdUserJSON, postCode := requestEndpointHelper(t, r, *scimConn.APIKey,
				http.MethodPost, baseURL+"/Users", strings.NewReader(string(ExampleUserCoreJSON)))
			require.Equal(t, http.StatusCreated, postCode, string(createdUserJSON))
			// set expect value
			dynamicFields := DynamicFields{}
			require.NoError(t, json.Unmarshal(createdUserJSON, &dynamicFields))
			expectCreatedJson := replaceFieldFromJSON(t, ExampleUserCoreJSON, "id", dynamicFields.ID) // set id
			expectCreatedJson = replaceFieldFromJSON(t, expectCreatedJson, "groups", nil)             // group should created by with group endpoint
			meta := SCIMMeta{}
			require.NoError(t, json.Unmarshal(createdUserJSON, &meta))
			expectCreatedJson = replaceFieldFromJSON(t, expectCreatedJson, "meta", dynamicFields.Meta)

			require.JSONEq(t,
				string(util.PrettyJSON(t, expectCreatedJson)),
				string(util.PrettyJSON(t, createdUserJSON)))

			t.Log("test get list")
			// set expect value
			var createdUser scimmodelsv2.SCIMUser
			require.NoError(t, scimprotocol.Unmarshal(createdUserJSON, &createdUser))
			createdUser.ID = dynamicFields.ID // unmarshal will not unmarshal id
			createdUser.Meta.Created = dynamicFields.Meta.Created
			createdUser.Meta.LastModified = dynamicFields.Meta.LastModified
			createdUser.Groups = []scimmodelsv2.Group{}
			expectListResp := scimprotocol.ListResponse[scimmodelsv2.SCIMUser]{
				Resources:    []scimmodelsv2.SCIMUser{createdUser},
				StartIndex:   1,
				ItemsPerPage: 100,
				TotalResults: 1,
			}
			expectJson, err := expectListResp.MarshalSCIM()
			require.NoError(t, err)
			usersJSON, getListCode := requestEndpointHelper(t, r, *scimConn.APIKey,
				http.MethodGet, baseURL+"/Users", nil)
			require.Equal(t, http.StatusOK, getListCode)
			require.JSONEq(t, string(util.PrettyJSON(t, expectJson)), string(util.PrettyJSON(t, usersJSON)))

			t.Log("test get one")
			getJSON, getCode := requestEndpointHelper(t, r, *scimConn.APIKey,
				http.MethodGet, baseURL+"/Users/"+createdUser.ID, strings.NewReader(string(ExampleFullUserJSON)))
			require.Equal(t, http.StatusOK, getCode)
			require.JSONEq(t,
				string(util.PrettyJSON(t, expectCreatedJson)),
				string(util.PrettyJSON(t, getJSON)))

			t.Log("test update")
			updateInput := replaceFieldFromJSON(t, ExampleUserCoreJSON, "id", createdUser.ID)
			updateInput = replaceFieldFromJSON(t, updateInput, "timezone", nil)
			updateJSON, updateCode := requestEndpointHelper(t, r, *scimConn.APIKey,
				http.MethodPut, baseURL+"/Users/"+createdUser.ID, strings.NewReader(string(updateInput)))
			require.Equal(t, http.StatusOK, updateCode)
			// set expect value
			dynamicFields = DynamicFields{}
			require.NoError(t, json.Unmarshal(updateJSON, &dynamicFields))
			expectCreatedJson = replaceFieldFromJSON(t, expectCreatedJson, "meta", dynamicFields.Meta)
			expectCreatedJson = replaceFieldFromJSON(t, expectCreatedJson, "timezone", nil)
			require.JSONEq(t,
				string(util.PrettyJSON(t, expectCreatedJson)),
				string(util.PrettyJSON(t, updateJSON)))

			t.Log("test patch")
			patchedJSON, patchedCode := requestEndpointHelper(t, r, *scimConn.APIKey,
				http.MethodPatch, baseURL+"/Users/"+createdUser.ID, strings.NewReader(`
			{
				"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
				"Operations":[{
					"op":"add",
					"value":{
						"displayName":"patrchedName"
					}
				}]
			}`))
			require.Equal(t, http.StatusOK, patchedCode)
			expectPatch := replaceFieldFromJSON(t, expectCreatedJson, "displayName", "patrchedName")
			// ignore meta
			expectPatch = replaceFieldFromJSON(t, expectPatch, "meta", nil)
			patchedJSON = replaceFieldFromJSON(t, patchedJSON, "meta", nil)
			require.JSONEq(t,
				string(util.PrettyJSON(t, expectPatch)),
				string(util.PrettyJSON(t, patchedJSON)))

			t.Log("test delete")
			_, deleteCode := requestEndpointHelper(t, r, *scimConn.APIKey,
				http.MethodDelete, baseURL+"/Users/"+createdUser.ID, nil)
			require.Equal(t, http.StatusNoContent, deleteCode)
		})
}

func requestEndpointHelper(t *testing.T, r *nchi.Mux, apiKey string, httpMethod string, endpoint string, body io.Reader) ([]byte, int) {
	req := httptest.NewRequest(httpMethod, endpoint, body)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	usersJson, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return usersJson, resp.Code
}

func replaceFieldFromJSON(t *testing.T, data []byte, key string, value any) []byte {
	fieldMap := map[string]json.RawMessage{}
	err := json.Unmarshal(data, &fieldMap)
	require.NoError(t, err)
	if value == nil {
		delete(fieldMap, key)
	} else {
		fieldMap[key], err = json.Marshal(value)
		require.NoError(t, err)
	}
	result, err := json.Marshal(fieldMap)
	require.NoError(t, err)
	return result
}
