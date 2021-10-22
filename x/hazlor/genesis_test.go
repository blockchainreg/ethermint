package hazlor_test

import (
	"testing"

	keepertest "github.com/cosmonaut/hazlor/testutil/keeper"
	"github.com/cosmonaut/hazlor/x/hazlor"
	"github.com/cosmonaut/hazlor/x/hazlor/types"
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		// this line is used by starport scaffolding # genesis/test/state
	}

	k, ctx := keepertest.HazlorKeeper(t)
	hazlor.InitGenesis(ctx, *k, genesisState)
	got := hazlor.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	// this line is used by starport scaffolding # genesis/test/assert
}
