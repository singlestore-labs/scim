package scimprotocol_test

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	scimprotocol "github.com/singlestore-labs/scim"
	"github.com/stretchr/testify/require"
)

func TestPatchOnUUID(t *testing.T) {
	t.Parallel()

	testUser := TestMarshalObject{
		TestUser: TestUser{
			ID: TestResourceID{UUID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")},
			Members: []Members{{
				Value: TestResourceID{UUID: uuid.MustParse("223e4567-e89b-12d3-a456-426614174000")}, Display: "member1"}},
			Groups: []Groups{{
				Value: TestResourceID{UUID: uuid.MustParse("123e4567-e89b-12d3-a456-426614174001")}, Display: "member1"}},
			Manager: Manager{
				Value: TestResourceID{UUID: uuid.MustParse("323e4567-e89b-12d3-a456-426614174000")},
			},
		},
	}
	expectUser := testUser
	cases := []struct {
		path  string
		value []byte
		want  func(err error, msg string)
	}{
		{
			path:  "id",
			value: []byte(`"550e8400-e29b-41d4-a716-446655440001"`),
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				expectUser.ID = TestResourceID{UUID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")}
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		{
			path:  "members[display eq \"member1\"].value",
			value: []byte(`"550e8400-e29b-41d4-a716-446655440001"`),
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				expectUser.Members[0].Value = TestResourceID{UUID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")}
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		{
			path:  "groups[display eq \"member1\"].value",
			value: []byte(`"550e8400-e29b-41d4-a716-446655440001"`),
			want: func(err error, msg string) {
				require.ErrorContains(t, err, "mutability", msg)
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		{
			path: "groups[display eq \"member1\"]",
			value: []byte(`{
				value: "550e8400-e29b-41d4-a716-446655440001",
				display: "member1"
			}`),
			want: func(err error, msg string) {
				require.ErrorContains(t, err, "mutability", msg)
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		{
			path:  "manager.value",
			value: []byte(`"550e8400-e29b-41d4-a716-446655440001"`),
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				expectUser.Manager.Value = TestResourceID{UUID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")}
				require.Equal(t, expectUser, testUser, msg)
			},
		},
	}

	for _, c := range cases {
		t.Logf("start test patch add at: %s", c.path)

		v := reflect.ValueOf(&testUser).Elem() // get settable reflect value
		path, err := scimprotocol.ParsePath(c.path)
		require.NoError(t, err, "should parse patch path, %s", c.path)
		coreSchema, _, err := scimprotocol.GetSchemaURIFromResource(v.Type(), nil)
		require.NoError(t, err, "should to get core schema from %s", v.Type())
		pathNode := path.GetNodes(coreSchema)

		err = scimprotocol.Patch(v, pathNode, "add", c.value)
		c.want(err, c.path)
	}

}
