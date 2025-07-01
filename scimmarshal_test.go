package scimprotocol_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"singlestore.com/helios/scim/scimprotocol"
	"singlestore.com/helios/scim/scimprotocol/scimtag"
	"singlestore.com/helios/scim/scimprotocol/util"
)

func init() {
	scimtag.BuildAllSCIMCharacsCache(TestMarshalObject{})
}

type TestMarshalObject struct {
	scimprotocol.SCIMResourceMarker
	TestUser `scim:"urn:ietf:params:scim:schemas:core:2.0:User"`
}
type TestUser struct {
	// non-scim field
	NonSCIM string
	// marshal related
	DefaultReturn   string   `scim:"defaultReturn"` // default omitempty
	NeverReturn     string   `scim:"neverReturn,returned=never"`
	KeepEmptyReturn string   `scim:"keepEmptyReturn,returned=keepEmpty"` // not omitempty
	Request         string   `scim:"request,returned=request"`
	Always          string   `scim:"always,returned=always"`
	ArrayEmpty      []string `scim:"arrayEmpty"`
	ArrayNil        []string `scim:"arrayNil"`
	// unmarshal related
	IgnoreUnmarshal string `scim:"ignoreUnmarshal,ignoreUnmarshal"`
	RequiredField   string `scim:"requiredField,required"`
}

var _ scimprotocol.Resource = TestMarshalObject{}

// Marshal been affected by 'returned'
// returned=default will omit empty
// returned=keepEmpty will keep empty
// returned=request will must marshal when it's been requested
// returned=never will never marshal the field even selected
// returned=always will alway marshal the field even not selected
func TestMarshal(t *testing.T) {
	t.Parallel()
	test := TestMarshalObject{
		TestUser: TestUser{
			NonSCIM: "non-scim",
			// marshal related
			Always:          "", // should marshal
			Request:         "should only marshal when requested",
			DefaultReturn:   "", // should omit marshal
			NeverReturn:     "should never marshal",
			KeepEmptyReturn: "", // should not omit
			ArrayEmpty:      []string{},
			ArrayNil:        nil,
			// unmarshal related, should no affect
			IgnoreUnmarshal: "unmarshal related, should no affect",
			RequiredField:   "",
		},
	}

	expectMarshal := []byte(
		`{
			"always":"",
			"ignoreUnmarshal":"unmarshal related, should no affect",
			"keepEmptyReturn": "",
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"]
		}`)

	json, err := scimprotocol.Marshal(test)
	require.NoError(t, err)
	require.Equal(t, string(util.PrettyJSON(t, expectMarshal)), string(util.PrettyJSON(t, json)))
}

// only 'required' and 'ignoreUnmarshal' affect unmarshal,
// else should have no effect
func TestUnmarshal(t *testing.T) {
	t.Parallel()
	expectObject := TestMarshalObject{
		TestUser: TestUser{
			// marshal related, should no affect here
			Always:          "", // should marshal
			Request:         "should no affect",
			DefaultReturn:   "should no affect", // should omit marshal
			NeverReturn:     "should no affect",
			KeepEmptyReturn: "", // should not omit
			// unmarshal related
			IgnoreUnmarshal: "", // should be ignored
			RequiredField:   "require value here",
		},
	}

	testJSON := []byte(
		`{
			"nonSCIM": "non-scim",
			"defaultReturn": "should no affect",
			"neverReturn": "should no affect",
			"request":"should no affect",
			"keepEmptyReturn": "",
			"IgnoreUnmarshal": "should be ignored",
			"requiredField": "require value here",
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"]
		}`)

	var object TestMarshalObject
	err := scimprotocol.Unmarshal(testJSON, &object)
	require.NoError(t, err)
	require.Equal(t, expectObject, object)

	// should have error if input without required
	testErrJSON := []byte(
		`{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"IgnoreUnmarshal": "testIgnoreUnmarshal",
			"defaultReturn": "testDefaultReturn",
			"neverReturn": "testNeverReturn",
			"omitEmptyReturn": "testOmitEmptyReturn"
		}`)

	err = scimprotocol.Unmarshal(testErrJSON, &object)
	require.ErrorContains(t, err, "required")
}
