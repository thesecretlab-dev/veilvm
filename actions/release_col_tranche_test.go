package actions

import (
	"context"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

const (
	testLocked          = uint64(10_000_000)
	testLive            = uint64(1_000_000)
	testMaxReleaseBips  = uint16(15) // 0.15%
	testEpochSeconds    = int64(86_400)
	testMaxRelease      = uint64(15_000) // 10_000_000 * 15 / 10_000
)

func seedTreasury(t *testing.T, mu state.Mutable, gov codec.Address) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, storage.PutTreasuryConfig(ctx, mu, storage.TreasuryConfig{
		Governance:          gov,
		Operations:          gov,
		MaxReleaseBips:      testMaxReleaseBips,
		ReleaseEpochSeconds: testEpochSeconds,
	}))
	require.NoError(t, storage.PutTreasuryState(ctx, mu, storage.TreasuryState{
		Locked:          testLocked,
		Live:            testLive,
		Released:        0,
		LastReleaseUnix: 0,
	}))
}

func TestReleaseCOLTrancheHappyPath(t *testing.T) {
	ctx := context.Background()
	gov := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	seedTreasury(t, store, gov)

	now := time.Now().UnixMilli()
	act := &ReleaseCOLTranche{Amount: testMaxRelease}
	out, err := act.Execute(ctx, nil, store, now, gov, ids.Empty)
	require.NoError(t, err)

	st, err := storage.GetTreasuryState(ctx, store)
	require.NoError(t, err)
	require.Equal(t, testLocked-testMaxRelease, st.Locked)
	require.Equal(t, testLive+testMaxRelease, st.Live)
	require.Equal(t, testMaxRelease, st.Released)
	require.Equal(t, now, st.LastReleaseUnix)

	res, err := UnmarshalReleaseCOLTrancheResult(out)
	require.NoError(t, err)
	got := res.(*ReleaseCOLTrancheResult)
	require.Equal(t, st.Locked, got.Locked)
	require.Equal(t, st.Live, got.Live)
}

func TestReleaseCOLTrancheUnauthorized(t *testing.T) {
	ctx := context.Background()
	gov := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	seedTreasury(t, store, gov)
	other := codec.Address{1, 2, 3}

	_, err := (&ReleaseCOLTranche{Amount: 1}).Execute(ctx, nil, store, time.Now().UnixMilli(), other, ids.Empty)
	require.ErrorIs(t, err, storage.ErrUnauthorized)

	st, err := storage.GetTreasuryState(ctx, store)
	require.NoError(t, err)
	require.Equal(t, testLocked, st.Locked)
}

func TestReleaseCOLTrancheZeroAmount(t *testing.T) {
	ctx := context.Background()
	gov := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	seedTreasury(t, store, gov)
	_, err := (&ReleaseCOLTranche{Amount: 0}).Execute(ctx, nil, store, time.Now().UnixMilli(), gov, ids.Empty)
	require.ErrorIs(t, err, storage.ErrInvalidReleaseAmount)
}

func TestReleaseCOLTrancheDrainLockedFails(t *testing.T) {
	ctx := context.Background()
	gov := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	seedTreasury(t, store, gov)
	_, err := (&ReleaseCOLTranche{Amount: testLocked}).Execute(ctx, nil, store, time.Now().UnixMilli(), gov, ids.Empty)
	require.ErrorIs(t, err, storage.ErrReleaseCapExceeded)

	st, err := storage.GetTreasuryState(ctx, store)
	require.NoError(t, err)
	require.Equal(t, testLocked, st.Locked, "locked COL must be unchanged after a drain attempt")
}

func TestReleaseCOLTrancheEpochCap(t *testing.T) {
	ctx := context.Background()
	gov := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	seedTreasury(t, store, gov)
	_, err := (&ReleaseCOLTranche{Amount: testMaxRelease + 1}).Execute(ctx, nil, store, time.Now().UnixMilli(), gov, ids.Empty)
	require.ErrorIs(t, err, storage.ErrReleaseCapExceeded)
}

func TestReleaseCOLTrancheTooEarly(t *testing.T) {
	ctx := context.Background()
	gov := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	seedTreasury(t, store, gov)
	now := time.Now().UnixMilli()
	_, err := (&ReleaseCOLTranche{Amount: 1}).Execute(ctx, nil, store, now, gov, ids.Empty)
	require.NoError(t, err)
	_, err = (&ReleaseCOLTranche{Amount: 1}).Execute(ctx, nil, store, now+1, gov, ids.Empty)
	require.ErrorIs(t, err, storage.ErrReleaseTooEarly)
}

func TestReleaseCOLTrancheInsufficientLocked(t *testing.T) {
	ctx := context.Background()
	gov := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	require.NoError(t, storage.PutTreasuryConfig(ctx, store, storage.TreasuryConfig{
		Governance:          gov,
		Operations:          gov,
		MaxReleaseBips:      10_000,
		ReleaseEpochSeconds: testEpochSeconds,
	}))
	require.NoError(t, storage.PutTreasuryState(ctx, store, storage.TreasuryState{Locked: 10, Live: 0}))
	_, err := (&ReleaseCOLTranche{Amount: 11}).Execute(ctx, nil, store, time.Now().UnixMilli(), gov, ids.Empty)
	require.ErrorIs(t, err, storage.ErrInsufficientLockedCOL)
}

func TestReleaseCOLTrancheScopedStateKeys(t *testing.T) {
	ctx := context.Background()
	gov := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	seedTreasury(t, store, gov)
	act := &ReleaseCOLTranche{Amount: testMaxRelease}
	keys := act.StateKeys(gov, ids.Empty)
	ts := tstate.New(16)
	tsv := ts.NewView(keys, store, len(keys))
	_, err := act.Execute(ctx, nil, tsv, time.Now().UnixMilli(), gov, ids.Empty)
	require.NoError(t, err)
}
