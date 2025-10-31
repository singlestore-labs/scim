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
