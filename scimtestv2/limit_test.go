package scimtestv2

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"singlestore.com/helios/auth/usersync/usersynctasks"
	"singlestore.com/helios/data"
	"singlestore.com/helios/db"
	"singlestore.com/helios/graph"
	"singlestore.com/helios/radagast"
	"singlestore.com/helios/scim/scimlib"
	"singlestore.com/helios/scim/scimmodels"
	"singlestore.com/helios/scim/scimstore"
	"singlestore.com/helios/test/di"
	"singlestore.com/helios/uuid"
)

func TestSCIMResourceLimit(t *testing.T) {
	if !scimmodels.GateSCIMResourcesNumLimit.Enabled() {
		t.Skipf("skip this test with code gate %s disabled", scimmodels.GateSCIMResourcesNumLimit.Name())
	}
	di.IntegrationTest(t,
		di.InjectTimeLimit(10*time.Second),
		TestSCIMConnection,
		func(
			ctx context.Context,
			scimLib *scimlib.Library,
			scimConn1 scimmodels.SCIMConnectionGQLOutput,
			conn *data.Connection,
		) {
			t.Log("set up radagast for submit job only")
			radagast.RegisterKind(&usersynctasks.GrafanaProjectUserSyncTask{})

			t.Logf("create under limit users")
			for i := 1; i <= scimstore.MaxSCIMUser; i++ {
				inputSCIMUser1 := GetCreateSCIMInputV2(scimConn1.SCIMID, strconv.Itoa(i))
				scimuser, err := scimLib.UserCreateV2(ctx, scimConn1.SCIMID, inputSCIMUser1)
				require.NoError(t, err, fmt.Sprintf("create normal SCIM user-%d", i))
				t.Cleanup(func() {
					err := scimLib.UserDeleteV2(ctx, scimConn1.SCIMID, uuid.MustParse[uuid.ObjectID](scimuser.ID))
					require.NoError(t, err)
				})
			}

			t.Log("create one more users exceed limit")
			inputSCIMUser1 := GetCreateSCIMInputV2(scimConn1.SCIMID, strconv.Itoa(scimstore.MaxSCIMUser+1))
			_, err := scimLib.UserCreateV2(ctx, scimConn1.SCIMID, inputSCIMUser1)
			require.Error(t, err)

			t.Log("enable DisableSCIMLimit feature flag")
			err = db.FeatureFlagEnable(ctx, conn, scimConn1.OrgID, graph.FeatureFlagIDDisableSCIMLimit, nil)
			require.NoError(t, err)

			t.Log("create one more users exceed limit, should success after enable DisableSCIMLimit feature flag")
			inputSCIMUser1 = GetCreateSCIMInputV2(scimConn1.SCIMID, strconv.Itoa(scimstore.MaxSCIMUser+1))
			_, err = scimLib.UserCreateV2(ctx, scimConn1.SCIMID, inputSCIMUser1)
			require.NoError(t, err)
		})
}
