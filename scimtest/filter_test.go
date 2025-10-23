package scimtest

import (
	"reflect"
	"testing"

	scimprotocol "github.com/singlestore-labs/scim"
	"github.com/singlestore-labs/scim/scimtag"
	"github.com/stretchr/testify/require"
)

func init() {
	scimtag.BuildAllSCIMCharacsCache(SCIMUser{})
}

func TestFilterExpressionEvaluation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		filter string
		want   bool
	}{
		{
			filter: `userName eq "bjensen"`,
			want:   false,
		},
		{
			filter: `userName co "bjensen"`,
			want:   true,
		},
		{
			filter: `name.familyName co "O'Malley"`,
			want:   false,
		},
		{
			filter: `emails[type eq "work" and value co "@example.com"]`,
			want:   true,
		},
		{
			filter: `emails[type eq "work" and value co "@example1.com"]`,
			want:   false,
		},
	}

	for _, c := range cases {
		expression, err := scimprotocol.ParseFilter(c.filter)
		require.NoError(t, err, c.filter)

		v := reflect.ValueOf(ExampleUserCore)
		result, err := expression.Eval(v, false)
		require.NoError(t, err, c.filter)

		require.Equal(t, c.want, result, c.filter)
	}
}

func TestFilterEvaluation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		filter string
		want   bool
	}{
		{
			filter: `userName eq "bjensen"`,
			want:   false,
		},
		{
			filter: `userName co "bjensen"`,
			want:   true,
		},
		{
			filter: `name.familyName co "O'Malley"`,
			want:   false,
		},
		{
			filter: `emails[type eq "work" and value co "@example.com"]`,
			want:   true,
		},
		{
			filter: `emails co "example.com" or emails.value co "example.org"`,
			want:   true,
		},
		{
			filter: `userType eq "Employee" and not (emails co "example.com" or emails.value co "example.org")`,
			want:   false,
		},
		{
			filter: `urn:ietf:params:scim:schemas:core:2.0:User:userName sw "J"`,
			want:   false,
		},
		{
			filter: `schemas eq "urn:ietf:params:scim:schemas:core:2.0:User"`,
			want:   true,
		},
		// check caseExact
		{
			filter: `displayName eq "babs Jensen"`,
			want:   false,
		},
		{
			filter: `displayName eq "Babs Jensen"`,
			want:   true,
		},
		// test priority of 'and' and 'or'
		// when 'and' first precedence cause different result with other kind of precedence?
		// t or t and f -> 'and' first = true
		// 				->  others = false
		{
			filter: `userType eq "Employee" or userType eq "Employee" and userType ne "Employee"`,
			want:   true,
		},
	}

	for _, c := range cases {
		expression, err := scimprotocol.ParseFilter(c.filter)
		require.NoError(t, err, c.filter)

		v := reflect.ValueOf(ExampleUserCore)
		result, err := expression.Eval(v, false)
		require.NoError(t, err, c.filter)

		require.Equal(t, c.want, result, c.filter)
	}
}
