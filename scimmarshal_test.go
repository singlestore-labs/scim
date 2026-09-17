package scimprotocol_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/memsql/errors"
	scimprotocol "github.com/singlestore-labs/scim"
	"github.com/singlestore-labs/scim/scimerror"
	"github.com/singlestore-labs/scim/scimtag"
	"github.com/singlestore-labs/scim/util"
	"github.com/stretchr/testify/require"
)

func init() {
	scimtag.BuildAllSCIMCharacsCache(TestMarshalObject{})
}

type TestMarshalObject struct {
	scimprotocol.SCIMResourceMarker
	TestUser `scim:"urn:ietf:params:scim:schemas:core:2.0:User"`
}
type TestResourceID struct {
	uuid.UUID
}

var _ json.Marshaler = TestResourceID{}

func (id TestResourceID) MarshalJSON() ([]byte, error) {
	return json.Marshal(uuid.UUID(id.UUID))
}

var _ json.Unmarshaler = (*TestResourceID)(nil)

func (id *TestResourceID) UnmarshalJSON(data []byte) error {
	var uid uuid.UUID
	if err := json.Unmarshal(data, &uid); err != nil {
		return err
	}
	*id = TestResourceID{UUID: uid}
	return nil
}

var _ scimprotocol.PrimaryDataType = TestResourceID{}

func (id TestResourceID) SCIMCompareValue(op scimprotocol.CompareOp, stringValue string, azureAdd bool) (bool, error) {
	if op == "pr" {
		return id.UUID != uuid.Nil, nil
	}
	uid, err := uuid.Parse(stringValue)
	if err != nil {
		return false, err
	}
	switch op {
	case "eq":
		return uuid.UUID(id.UUID) == uid, nil
	case "ne":
		return uuid.UUID(id.UUID) != uid, nil
	default:
		return false, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Errorf("not support compare operation %s on 'TestResourceID'", op))
	}
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
	// special resources
	ID      TestResourceID `scim:"id,returned=always"`
	Members []Members      `scim:"members"`
	Groups  []Groups       `scim:"groups,mutability=readOnly"`
	Manager Manager        `scim:"manager"`
}

type ResourceRef struct {
	Value   TestResourceID `scim:"value"`
	Display string         `scim:"display"`
}

type (
	Manager ResourceRef
	Groups  ResourceRef
	Members ResourceRef
)

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
			ID:              TestResourceID{UUID: uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")},
			Members: []Members{
				{Value: TestResourceID{UUID: uuid.MustParse("223e4567-e89b-12d3-a456-426614174000")}, Display: "member1"},
			},
			Manager: Manager{
				Value: TestResourceID{UUID: uuid.MustParse("323e4567-e89b-12d3-a456-426614174000")},
			},
		},
	}

	expectMarshal := []byte(
		`{
			"always":"",
			"id": "123e4567-e89b-12d3-a456-426614174000",
			"ignoreUnmarshal":"unmarshal related, should no affect",
			"keepEmptyReturn": "",
			"manager": {
        		"value": "323e4567-e89b-12d3-a456-426614174000"
        	},
        	"members": [
        		{
        			"display": "member1",
        			"value": "223e4567-e89b-12d3-a456-426614174000"
        		}
        	],
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
			Members: []Members{
				{Value: TestResourceID{UUID: uuid.MustParse("223e4567-e89b-12d3-a456-426614174000")}, Display: "member1"},
			},
			Manager: Manager{
				Value: TestResourceID{UUID: uuid.MustParse("323e4567-e89b-12d3-a456-426614174000")},
			},
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
			"manager": {
        		"value": "323e4567-e89b-12d3-a456-426614174000"
        	},
        	"members": [
        		{
        			"display": "member1",
        			"value": "223e4567-e89b-12d3-a456-426614174000"
        		}
        	],
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

func TestListResponseSelectedAttributes(t *testing.T) {
	t.Parallel()
	list := scimprotocol.ListResponse[TestMarshalObject]{
		Resources: []TestMarshalObject{{
			TestUser: TestUser{
				Always:        "keep",
				DefaultReturn: "visible",
				RequiredField: "required",
				ID:            TestResourceID{UUID: uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")},
				Members: []Members{
					{Value: TestResourceID{UUID: uuid.MustParse("223e4567-e89b-12d3-a456-426614174000")}, Display: "member1"},
				},
			},
		}},
		StartIndex:   1,
		ItemsPerPage: 1,
		TotalResults: 1,
	}

	cases := map[string]struct {
		selectedAttr    []string
		excludeSelected bool
		expectResource  string
	}{
		// baseline: without selection every default-returned attribute is present,
		// which is what makes the two cases below meaningful
		"no selection": {
			expectResource: `{
				"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
				"id": "123e4567-e89b-12d3-a456-426614174000",
				"always": "keep",
				"defaultReturn": "visible",
				"keepEmptyReturn": "",
				"requiredField": "required",
				"members": [{"value": "223e4567-e89b-12d3-a456-426614174000", "display": "member1"}]
			}`,
		},
		// attributes=members: only 'members' (plus returned=always) survives,
		// 'defaultReturn' and 'requiredField' are dropped even though they have values
		"select members": {
			selectedAttr: []string{"members"},
			expectResource: `{
				"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
				"id": "123e4567-e89b-12d3-a456-426614174000",
				"always": "keep",
				"members": [{"value": "223e4567-e89b-12d3-a456-426614174000", "display": "member1"}]
			}`,
		},
		// excludedAttributes=members: the mirror image, 'members' is the only loss
		"exclude members": {
			selectedAttr:    []string{"members"},
			excludeSelected: true,
			expectResource: `{
				"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
				"id": "123e4567-e89b-12d3-a456-426614174000",
				"always": "keep",
				"defaultReturn": "visible",
				"keepEmptyReturn": "",
				"requiredField": "required"
			}`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			encoded, err := scimprotocol.MarshalWithSelectedAttr(list, tc.selectedAttr, tc.excludeSelected)
			require.NoError(t, err)

			var decoded struct {
				Resources []json.RawMessage `json:"Resources"`
			}
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			require.Len(t, decoded.Resources, 1, string(encoded))
			require.JSONEq(t, tc.expectResource, string(decoded.Resources[0]))
		})
	}
}
