package scimtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/memsql/ntest"
	"github.com/muir/nchi"
	scimprotocol "github.com/singlestore-labs/scim"
	"github.com/singlestore-labs/scim/util"
	"github.com/stretchr/testify/require"
)

const (
	testSCIMEndpoint = "/testscim"
)

var getSCIMRouter = func(t ntest.T, tracer util.Trace, storage *Storage) *nchi.Mux {
	r := nchi.NewRouter()
	t.Log("set up server")
	scimServer := NewServer(tracer, storage)
	scimRouter := scimprotocol.SCIMRouter(tracer, scimServer)
	// r.Route(testSCIMEndpoint+"/:scimID", scimRouter)
	r.Route(testSCIMEndpoint, scimRouter)
	return r
}

/*
This file only test over the server handler functions
*/

type DynamicFields struct {
	ID    string   `json:"id"`
	Group []any    `json:"groups"`
	Meta  SCIMMeta `json:"meta"`
}

func TestSCIMServer(t *testing.T) {
	ntest.RunTest(t,
		func() (util.Trace, *Storage) { return util.Trace(t), NewInMemStorage() },
		getSCIMRouter,
		func(
			tracer util.Trace,
			r *nchi.Mux,
		) {
			baseURL := testSCIMEndpoint
			apiKey := DummyToken

			t.Logf("base url %s", baseURL)
			t.Log("test post")
			createdUserJSON, postCode := requestEndpointHelper(t, r, apiKey,
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
				string(util.PrettyJSON(t, createdUserJSON)),
			)

			t.Log("test get list")
			// set expect value
			var createdUser SCIMUser
			require.NoError(t, scimprotocol.Unmarshal(createdUserJSON, &createdUser))
			createdUser.ID = dynamicFields.ID // unmarshal will not unmarshal id
			createdUser.Meta.Created = dynamicFields.Meta.Created
			createdUser.Meta.LastModified = dynamicFields.Meta.LastModified
			createdUser.Groups = []Group{}
			expectResources := []SCIMUser{createdUser}
			expectListResp := scimprotocol.ListResponse[SCIMUser]{
				Resources:    expectResources,
				StartIndex:   1,
				ItemsPerPage: len(expectResources),
				TotalResults: len(expectResources),
			}
			expectJson, err := expectListResp.MarshalSCIM()
			require.NoError(t, err)
			usersJSON, getListCode := requestEndpointHelper(t, r, apiKey,
				http.MethodGet, baseURL+"/Users", nil)
			require.Equal(t, http.StatusOK, getListCode)
			require.JSONEq(t, string(util.PrettyJSON(t, expectJson)), string(util.PrettyJSON(t, usersJSON)))

			t.Log("test get one")
			getJSON, getCode := requestEndpointHelper(t, r, apiKey,
				http.MethodGet, baseURL+"/Users/"+createdUser.ID, strings.NewReader(string(ExampleFullUserJSON)))
			require.Equal(t, http.StatusOK, getCode)
			require.JSONEq(t,
				string(util.PrettyJSON(t, expectCreatedJson)),
				string(util.PrettyJSON(t, getJSON)))

			t.Log("test update")
			updateInput := replaceFieldFromJSON(t, ExampleUserCoreJSON, "id", createdUser.ID)
			updateInput = replaceFieldFromJSON(t, updateInput, "timezone", nil)
			updateJSON, updateCode := requestEndpointHelper(t, r, apiKey,
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
			patchedJSON, patchedCode := requestEndpointHelper(t, r, apiKey,
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
			_, deleteCode := requestEndpointHelper(t, r, apiKey,
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
