package actions

import (
	"context"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
)

func TestMintVAIDebtCeiling(t *testing.T) {
	ctx := context.Background()
	actor := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	seedMintState(t, store, actor)
	require.NoError(t, storage.PutVAIState(ctx, store, storage.VAIState{TotalDebt: 1_999_999}))

	_, err := (&MintVAI{To: actor, Amount: 2}).Execute(ctx, nil, store, time.Now().UnixMilli(), actor, ids.Empty)
	require.ErrorIs(t, err, storage.ErrVAIDebtCeilingExceeded)
}

func TestMintVAIEpochMintLimit(t *testing.T) {
	ctx := context.Background()
	actor := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	seedMintState(t, store, actor)
	now := time.Now().UnixMilli()

	_, err := (&MintVAI{To: actor, Amount: 100_000}).Execute(ctx, nil, store, now, actor, ids.Empty)
	require.NoError(t, err)
	_, err = (&MintVAI{To: actor, Amount: 1}).Execute(ctx, nil, store, now+1, actor, ids.Empty)
	require.ErrorIs(t, err, storage.ErrVAIEpochMintLimitExceeded)
}

func TestMintVAIEpochResets(t *testing.T) {
	ctx := context.Background()
	actor := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	seedMintState(t, store, actor)
	now := time.Now().UnixMilli()

	_, err := (&MintVAI{To: actor, Amount: 100_000}).Execute(ctx, nil, store, now, actor, ids.Empty)
	require.NoError(t, err)
	later := now + 3_600*1_000
	_, err = (&MintVAI{To: actor, Amount: 1}).Execute(ctx, nil, store, later, actor, ids.Empty)
	require.NoError(t, err)
}

func TestMintVAIBackingFloor(t *testing.T) {
	ctx := context.Background()
	actor := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	seedMintState(t, store, actor)
	require.NoError(t, storage.PutReserveState(ctx, store, storage.ReserveState{ExogenousReserve: 1_000}))

	_, err := (&MintVAI{To: actor, Amount: 1_001}).Execute(ctx, nil, store, time.Now().UnixMilli(), actor, ids.Empty)
	require.ErrorIs(t, err, storage.ErrBackingRatioViolation)

	_, err = (&MintVAI{To: actor, Amount: 1_000}).Execute(ctx, nil, store, time.Now().UnixMilli(), actor, ids.Empty)
	require.NoError(t, err)
}

func TestMintVAIUnauthorized(t *testing.T) {
	ctx := context.Background()
	actor := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	seedMintState(t, store, actor)
	other := actor
	other[0] ^= 0xff
	_, err := (&MintVAI{To: actor, Amount: 1}).Execute(ctx, nil, store, time.Now().UnixMilli(), other, ids.Empty)
	require.ErrorIs(t, err, storage.ErrUnauthorized)
}

func TestWsVEILLTVMustBeZero(t *testing.T) {
	ctx := context.Background()
	store := chaintest.NewInMemoryStore()
	err := storage.PutRiskConfig(ctx, store, storage.RiskConfig{
		BackingFloorBips: 10_000,
		WSVEILLtvBips:    1,
	})
	require.ErrorIs(t, err, storage.ErrInvalidRiskConfig)

	gov := genesisActor(t)
	require.NoError(t, storage.PutTreasuryConfig(ctx, store, storage.TreasuryConfig{
		Governance:          gov,
		Operations:          gov,
		MaxReleaseBips:      15,
		ReleaseEpochSeconds: 86_400,
	}))
	_, err = (&SetRiskParams{
		BackingFloorBips: 10_000,
		WSVEILLtvBips:    500,
	}).Execute(ctx, nil, store, 0, gov, ids.Empty)
	require.ErrorIs(t, err, storage.ErrInvalidRiskConfig)
}
