package scimtestv2

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"singlestore.com/helios/scim/scimmodelsv2"
	"singlestore.com/helios/scim/scimprotocol"
)

func TestPatchAdd(t *testing.T) {
	t.Parallel()

	testUser := ExampleUserCore.Copy()
	expectUser := testUser
	cases := []struct {
		path  string
		value []byte
		want  func(err error, msg string)
	}{
		{
			path:  "userName",
			value: []byte(`"addedUserName"`),
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				expectUser.UserName = "addedUserName"
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		{
			path:  "name.familyName",
			value: []byte(`"addedFamilyName"`),
			want: func(err error, msg string) {
				require.NoError(t, err)
				expectUser.Name.FamilyName = "addedFamilyName"
				require.Equal(t, expectUser, testUser, msg)
			},
		},

		{ // test non-duplicate
			path: `emails`,
			value: []byte(`
				[
					{
						"value":"addbabs@jensen.org",
						"type":"home"
					}
				]
		   `),
			want: func(err error, msg string) {
				require.NoError(t, err)
				expectUser.Emails = append(expectUser.Emails, scimmodelsv2.Email{
					Value: "addbabs@jensen.org",
					Type:  "home",
				})
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		{ // test duplicate
			path: `emails`,
			value: []byte(`
				[
					{
						"value":"addbabs@jensen.org",
						"type":"home"
					}
				]
		   `),
			want: func(err error, msg string) {
				require.NoError(t, err)
				require.ElementsMatch(t, expectUser.Emails, testUser.Emails, msg)
			},
		},
		{
			path: `emails[type eq "work"]`,
			value: []byte(`
				{
					"value":"addAfterFilter@jensen.org"
				}`),
			want: func(err error, msg string) {
				require.NoError(t, err)
				expectUser.Emails[0].Value = `addAfterFilter@jensen.org`
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		{ // azure support, add email by filter....
			path:  `emails[type eq "other"].value`,
			value: []byte(`"addbyFilter@jensen.org"`),
			want: func(err error, msg string) {
				require.NoError(t, err)
				expectUser.Emails = append(expectUser.Emails, scimmodelsv2.Email{
					Type:  "other",
					Value: "addbyFilter@jensen.org",
				})
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		{
			path:  `emails[type eq "work"].value`,
			value: []byte(`"addbabs@jensen.org"`),
			want: func(err error, msg string) {
				require.NoError(t, err)
				expectUser.Emails[0].Value = `addbabs@jensen.org`
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		// 1. need patch on root 2. add patch duplicate at array (what to check unique?)
		{
			path: "name",
			value: []byte(`
			{
				"givenName": "John",
				"familyName": "Doe"
			}`),
			want: func(err error, msg string) {
				require.NoError(t, err)
				expectUser.Name.GivenName = "John"
				expectUser.Name.FamilyName = "Doe"
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		{ // update primary
			path: "",
			value: []byte(`
			{
				"emails":[
				{
					"value":"add4babs@jensen.org",
					"type":"home",
					"primary":true
				}
				],
				"userName":"Babs"
			}`),
			want: func(err error, msg string) {
				require.NoError(t, err)
				expectUser.Emails[0].Primary = false
				expectUser.Emails = append(expectUser.Emails, scimmodelsv2.Email{
					Value:   "add4babs@jensen.org",
					Type:    "home",
					Primary: true,
				})
				expectUser.UserName = "Babs"
				require.ElementsMatch(t, expectUser.Emails, testUser.Emails)
			},
		},
		{ // test immutable
			path: `groups`,
			value: []byte(`
				[
					{
						"value":"group value",
						"display":"group1"
					}
				]
		   `),
			want: func(err error, msg string) {
				require.Error(t, err)
				require.ErrorContains(t, err, "mutability")
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

func TestPatchReplace(t *testing.T) {
	t.Parallel()

	testUser := ExampleUserCore.Copy()
	expectUser := testUser
	cases := []struct {
		// op    string
		path  string
		value []byte
		want  func(err error, errMsg string)
	}{
		{
			path:  "userName",
			value: []byte(`"addedUserName"`),
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				expectUser.UserName = "addedUserName"
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		{
			path:  "name.familyName",
			value: []byte(`"addedFamilyName"`),
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				expectUser.Name.FamilyName = "addedFamilyName"
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		{ // test non-duplicate
			path: `emails`,
			value: []byte(`
				[
					{
						"value":"addbabs@jensen.org",
						"type":"home"
					}
				]
		   `),
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				expectUser.Emails = []scimmodelsv2.Email{
					{
						Value: "addbabs@jensen.org",
						Type:  "home",
					},
				}
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		{ // test duplicate
			path: `emails`,
			value: []byte(`
				[
					{
						"value":"addbabs@jensen.org",
						"type":"home"
					}
				]
		   `),
			want: func(err error, msg string) {
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		{
			path: `emails[type eq "home"]`,
			value: []byte(`
				{
					"value":"addAfterFilter@jensen.org",
					"type":"home"
				}`),
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				expectUser.Emails[0] = scimmodelsv2.Email{
					Value: `addAfterFilter@jensen.org`,
					Type:  "home",
				}
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		{
			path:  `emails[type eq "home"].value`,
			value: []byte(`"addbabs@jensen.org"`),
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				expectUser.Emails[0].Value = `addbabs@jensen.org`
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		// 1. need patch on root 2. add patch duplicate at array (what to check unique?)
		{
			path: "name",
			value: []byte(`
			{
				"givenName": "John",
				"familyName": "Doe"
			}`),
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				expectUser.Name = scimmodelsv2.Name{
					GivenName:  "John",
					FamilyName: "Doe",
				}
				require.Equal(t, expectUser, testUser, msg)
			},
		},
		{ // update primary
			path: "",
			value: []byte(`
			{
				"emails":[
					{
						"value":"add4babs@jensen.org",
						"type":"home",
						"primary":true
					}
				],
				"userName":"Babs"
			}`),
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				expectUser.Emails = []scimmodelsv2.Email{
					{
						Value:   "add4babs@jensen.org",
						Type:    "home",
						Primary: true,
					},
				}
				expectUser.UserName = "Babs"
				require.Equal(t, expectUser, testUser, msg)
			},
		},
	}
	for _, c := range cases {
		t.Logf("start test patch replace at: %s", c.path)

		v := reflect.ValueOf(&testUser).Elem() // get settable reflect value
		path, err := scimprotocol.ParsePath(c.path)
		require.NoError(t, err, "should parse patch path, %s", c.path)
		coreSchema, _, err := scimprotocol.GetSchemaURIFromResource(v.Type(), nil)
		require.NoError(t, err, "should to get core schema from %s", v.Type())
		pathNode := path.GetNodes(coreSchema)

		err = scimprotocol.Patch(v, pathNode, "replace", c.value)
		c.want(err, c.path)
	}
}

func TestPatchRemove(t *testing.T) {
	t.Parallel()

	testUser := ExampleUserCore.Copy()
	expectUser := testUser
	cases := []struct {
		path  string
		value []byte
		want  func(err error, msg string)
	}{
		{
			path: "userType",
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				expectUser.UserType = ""
				require.Equal(t, expectUser, testUser, err, msg)
			},
		},
		{
			path: "name.familyName",
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				expectUser.Name.FamilyName = ""
				require.Equal(t, expectUser, testUser, err, msg)
			},
		},
		{
			path: `emails[type eq "work"]`,
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				expectUser.Emails = expectUser.Emails[1:len(expectUser.Emails)]
				require.Equal(t, expectUser, testUser, err, msg)
			},
		},
		{ // test non-duplicate
			path: `roles`,
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				expectUser.Roles = nil
				require.Equal(t, expectUser, testUser, err, msg)
			},
		},
		{ // test duplicate
			path: `roles`,
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				require.Equal(t, expectUser, testUser, err, msg)
			},
		},
		// 1. need patch on root 2. add patch duplicate at array (what to check unique?)
		{
			path: "name",
			// ignore value
			value: []byte(`
			{
				"givenName": "John",
				"familyName": "Doe"
			}`),
			want: func(err error, msg string) {
				require.NoError(t, err, msg)
				expectUser.Name = scimmodelsv2.Name{}
				require.Equal(t, expectUser, testUser, err, msg)
			},
		},
		{ // update primary, path required for remove
			path: "",
			value: []byte(`
			{
				"emails":[
					{
						"value":"add4babs@jensen.org",
						"type":"home",
						"primary":true
					}
				],
				"userName":"Babs"
			}`),
			want: func(err error, msg string) {
				require.Error(t, err, msg)
			},
		},
		{ // test 'required'
			path: "userName",
			want: func(err error, msg string) {
				require.Error(t, err, msg)
				temp := expectUser.UserName
				// "userName still been removed"
				expectUser.UserName = ""
				require.Equal(t, expectUser, testUser, err, msg)
				// recover for future test
				expectUser.UserName = temp
				testUser.UserName = temp
			},
		},
	}

	for _, c := range cases {
		t.Logf("start test patch remove at: %s", c.path)

		v := reflect.ValueOf(&testUser).Elem() // get settable reflect value
		path, err := scimprotocol.ParsePath(c.path)
		require.NoError(t, err, "should parse patch path, %s", c.path)
		coreSchema, _, err := scimprotocol.GetSchemaURIFromResource(v.Type(), nil)
		require.NoError(t, err, "should to get core schema from %s", v.Type())
		pathNode := path.GetNodes(coreSchema)

		err = scimprotocol.Patch(v, pathNode, "remove", c.value)
		c.want(err, c.path)
	}
}
