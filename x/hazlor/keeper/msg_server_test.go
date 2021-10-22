package keeper_test

import (
	"context"
	"testing"

	keepertest "github.com/cosmonaut/hazlor/testutil/keeper"
	"github.com/cosmonaut/hazlor/x/hazlor/keeper"
	"github.com/cosmonaut/hazlor/x/hazlor/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func setupMsgServer(t testing.TB) (types.MsgServer, context.Context) {
	k, ctx := keepertest.HazlorKeeper(t)
	return keeper.NewMsgServerImpl(*k), sdk.WrapSDKContext(ctx)
}
