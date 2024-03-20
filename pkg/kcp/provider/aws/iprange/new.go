package iprange

import (
	"context"
	"fmt"
	"github.com/kyma-project/cloud-manager/pkg/composed"
	iprangetypes "github.com/kyma-project/cloud-manager/pkg/kcp/iprange/types"
)

func New(stateFactory StateFactory) composed.Action {
	return func(ctx context.Context, st composed.State) (error, context.Context) {
		logger := composed.LoggerFromCtx(ctx)
		state, err := stateFactory.NewState(ctx, st.(iprangetypes.State))
		if err != nil {
			err = fmt.Errorf("error creating new aws iprange state: %w", err)
			logger.Error(err, "Error")
			return composed.StopAndForget, nil
		}
		return composed.BuildSwitchAction(
			"awsIpRange",
			composed.ComposeActions(
				"awsIpRange-non-delete",
				addFinalizer,
				preventCidrEdit,
				splitRangeByZones,
				ensureShootZonesAndRangeSubnetsMatch,
				loadVpc,
				checkCidrOverlap,
				checkCidrBlockStatus,
				extendVpcAddressSpace,
				loadSubnets,
				findCloudResourceSubnets,
				checkSubnetOverlap,
				createSubnets,
				updateSuccessStatus,
				composed.StopAndForgetAction,
			),
			composed.NewCase(
				composed.MarkedForDeletionPredicate,
				composed.ComposeActions(
					"awsIpRange-delete",
					removeReadyCondition,
					loadVpc,
					loadSubnets,
					findCloudResourceSubnets,

					deleteSubnets,
					waitSubnetsDeleted,

					disassociateVpcAddressSpace,
					waitCidrBlockDisassociated,

					removeFinalizer,
				),
			),
		)(ctx, state)
	}
}
