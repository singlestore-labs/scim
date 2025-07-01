package scimtestv2

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/stretchr/testify/require"
	"singlestore.com/helios/data"
	"singlestore.com/helios/scim/scimlib"
	"singlestore.com/helios/scim/scimmodels"
	"singlestore.com/helios/scim/scimmodelsv2"
	"singlestore.com/helios/scim/scimstore"
	"singlestore.com/helios/test/di"
	"singlestore.com/helios/trace"
)

func removeTabAndNewLine(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", "")
	return s
}

func TestFilterToSQL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		filter   string
		wantSQL  string
		wantArgs []any
	}{
		{
			filter:   `userName eq "bjensen" and name.familyName co "O'Malley"`,
			wantSQL:  `SELECT x FROM xTable WHERE ((SCIMUsers.userName = $1 AND SCIMUsers.formalName->>'FamilyName' ILIKE $2))`,
			wantArgs: []any{"bjensen", "%O'Malley%"},
		},
		{
			filter: `userName eq "bjensen" and name.familyName co "O'Malley" 
					or emails[type eq "work" and value co "@example.com"] 
					and not (emails co "example.not.co" or emails.value co "example.not.co")`,
			wantSQL: `SELECT x FROM xTable 
						CROSS JOIN LATERAL jsonb_to_recordset(SCIMUsers.emails) AS cemails(
							value text,
							display text,
							"primary" boolean,
							type text
						) WHERE (
							(SCIMUsers.userName = $1 AND SCIMUsers.formalName->>'FamilyName' ILIKE $2) 
							OR (((cemails.type = $3 AND cemails.value ILIKE $4)) 
								AND ((cemails.value NOT ILIKE $5) AND (cemails.value NOT ILIKE $6)))
						)`,
			wantArgs: []any{"bjensen", "%O'Malley%", "work", "%@example.com%", "%example.not.co%", "%example.not.co%"},
		},
	}

	for _, c := range cases {
		sg := scimstore.NewSCIMSQL("SCIMUsers", reflect.TypeOf(scimmodelsv2.SCIMUser{}))
		builder := sq.Select("x").From("xTable")
		builder, err := sg.AddFilterToSQL(builder, c.filter)
		require.NoError(t, err)
		q, args, err := builder.ToSql()
		require.NoError(t, err)
		require.Equal(t, removeTabAndNewLine(c.wantSQL), removeTabAndNewLine(q))
		require.Equal(t, c.wantArgs, args)
	}
}

func TestSQLFilter(t *testing.T) {
	di.IntegrationTest(t,
		di.InjectTimeLimit(30*time.Second),
		TestSCIMConnection,
		func(
			scimStore *scimstore.Store,
			scimLib *scimlib.Library,
			tracer trace.Iface,
			scimConn scimmodels.SCIMConnectionGQLOutput,
			conn *data.Connection,
			ctx context.Context,
		) {
			t.Log("create SCIM user1")
			inputSCIMUser1 := GetCreateSCIMInputV2(scimConn.SCIMID, "1")
			scimUser1, err := scimLib.UserCreateV2(ctx, scimConn.SCIMID, inputSCIMUser1)
			require.NoError(t, err, "create normal SCIM user1")
			t.Log("create SCIM user2")
			inputSCIMUser2 := GetCreateSCIMInputV2(scimConn.SCIMID, "2")
			scimUser2, err := scimLib.UserCreateV2(ctx, scimConn.SCIMID, inputSCIMUser2)
			require.NoError(t, err, "create normal SCIM user1")

			inputFilter := fmt.Sprintf(`userName eq "%s" and name.formatted eq "%s"`, scimUser1.UserName, scimUser1.Name.Formatted)
			outputSCIMUsers, err := scimStore.GetAllSCIMUserV2(ctx, conn, scimConn.SCIMID, func(q sq.SelectBuilder) sq.SelectBuilder {
				sg := scimstore.NewSCIMSQL("SCIMUsers", reflect.TypeOf(scimmodelsv2.SCIMUser{}))
				q, err := sg.AddFilterToSQL(q, inputFilter)
				require.NoError(t, err)
				return q
			})
			require.NoError(t, err)
			require.Equal(t, []scimmodelsv2.SCIMUser{scimUser1}, outputSCIMUsers)

			inputFilter = fmt.Sprintf(`emails[type eq "%s" and value co "%s"]`, scimUser2.Emails[0].Type, scimUser2.Emails[0].Value)
			outputSCIMUsers, err = scimStore.GetAllSCIMUserV2(ctx, conn, scimConn.SCIMID, func(q sq.SelectBuilder) sq.SelectBuilder {
				sg := scimstore.NewSCIMSQL("SCIMUsers", reflect.TypeOf(scimmodelsv2.SCIMUser{}))
				q, err := sg.AddFilterToSQL(q, inputFilter)
				require.NoError(t, err)
				return q
			})
			require.NoError(t, err)
			require.Equal(t, []scimmodelsv2.SCIMUser{scimUser2}, outputSCIMUsers)

			inputFilter = fmt.Sprintf(`not (emails co "%s" or emails.primary ne %s)`, scimUser2.Emails[0].Value, "true")
			outputSCIMUsers, err = scimStore.GetAllSCIMUserV2(ctx, conn, scimConn.SCIMID, func(q sq.SelectBuilder) sq.SelectBuilder {
				sg := scimstore.NewSCIMSQL("SCIMUsers", reflect.TypeOf(scimmodelsv2.SCIMUser{}))
				q, err := sg.AddFilterToSQL(q, inputFilter)
				require.NoError(t, err)
				return q
			})
			require.NoError(t, err)
			require.Equal(t, []scimmodelsv2.SCIMUser{scimUser1}, outputSCIMUsers)
		})
}
