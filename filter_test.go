package scimprotocol_test

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	scimprotocol "github.com/singlestore-labs/scim"
	"github.com/stretchr/testify/require"
)

func TestFilter(t *testing.T) {
	t.Parallel()
	cases := []struct {
		filter string
		want   bool
		err    func(t require.TestingT, err error, msgAndArgs ...interface{})
	}{
		// test filter in uuid type
		{
			filter: `id eq "550e8400-e29b-41d4-a716-446655440000"`,
			want:   true,
			err:    require.NoError,
		},
		{
			filter: `id ne "550e8400-e29b-41d4-a716-446655440000"`,
			want:   false,
			err:    require.NoError,
		},
		{
			filter: `id pr`,
			want:   true,
			err:    require.NoError,
		},
		{
			filter: `id co "550e8400-e29b-41d4-a716-446655440000"`,
			want:   false,
			err:    require.Error,
		},
	}

	for _, c := range cases {
		expression, err := scimprotocol.ParseFilter(c.filter)
		require.NoError(t, err, c.filter)

		testUser := TestMarshalObject{
			TestUser: TestUser{
				ID: TestResourceID{UUID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")},
			},
		}
		v := reflect.ValueOf(testUser)
		result, err := expression.Eval(v, false)
		c.err(t, err, c.filter)

		require.Equal(t, c.want, result, c.filter)
	}
}
