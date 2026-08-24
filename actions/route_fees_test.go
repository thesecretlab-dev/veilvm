package actions

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

func seedFeeRouter(t *testing.T, mu state.Mutable, actor codec.Address, bal uint64) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, storage.PutFeeRouterConfig(ctx, mu, storage.FeeRouterConfig{
		MSRBBips: 7_000,
		COLBips:  2_000,
		OpsBips:  1_000,
	}))
	require.NoError(t, storage.PutFeeRouterState(ctx, mu, storage.FeeRouterState{}))
	require.NoError(t, storage.SetBalance(ctx, mu, actor, bal))
}

func TestRouteFeesSeventyTwentyTen(t *testing.T) {
	ctx := context.Background()
	actor := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	const amount = uint64(10_000)
	seedFeeRouter(t, store, actor, amount)

	_, err := (&RouteFees{Amount: amount}).Execute(ctx, nil, store, 0, actor, ids.Empty)
	require.NoError(t, err)

	st, err := storage.GetFeeRouterState(ctx, store)
	require.NoError(t, err)
	require.Equal(t, uint64(7_000), st.MSRBBudget)
	require.Equal(t, uint64(2_000), st.COLBudget)
	require.Equal(t, uint64(1_000), st.OpsBudget)
	require.Equal(t, amount, st.MSRBBudget+st.COLBudget+st.OpsBudget)

	bal, err := storage.GetBalance(ctx, store, actor)
	require.NoError(t, err)
	require.Equal(t, uint64(0), bal)
}

func TestRouteFeesRemainderGoesToOps(t *testing.T) {
	ctx := context.Background()
	actor := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	seedFeeRouter(t, store, actor, 1)

	_, err := (&RouteFees{Amount: 1}).Execute(ctx, nil, store, 0, actor, ids.Empty)
	require.NoError(t, err)
	st, err := storage.GetFeeRouterState(ctx, store)
	require.NoError(t, err)
	require.Equal(t, uint64(0), st.MSRBBudget)
	require.Equal(t, uint64(0), st.COLBudget)
	require.Equal(t, uint64(1), st.OpsBudget)
}

func TestRouteFeesZeroRejected(t *testing.T) {
	ctx := context.Background()
	actor := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	seedFeeRouter(t, store, actor, 10)
	_, err := (&RouteFees{Amount: 0}).Execute(ctx, nil, store, 0, actor, ids.Empty)
	require.ErrorIs(t, err, storage.ErrInvalidFeeAmount)
}

func TestFeeRouterConfigMustSum10000(t *testing.T) {
	ctx := context.Background()
	store := chaintest.NewInMemoryStore()
	err := storage.PutFeeRouterConfig(ctx, store, storage.FeeRouterConfig{
		MSRBBips: 7_000,
		COLBips:  2_000,
		OpsBips:  500,
	})
	require.ErrorIs(t, err, storage.ErrInvalidFeeRouterConfig)
}
