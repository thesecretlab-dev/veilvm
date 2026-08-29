package actions

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
)

func seedPrivacyState(t *testing.T, store *chaintest.InMemoryStore, actor codec.Address, marketID ids.ID, requireProof bool) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, storage.PutMarket(ctx, store, marketID, storage.MarketStatusActive, 2, time.Now().Unix()+86_400, 0, []byte("privacy-test")))
	require.NoError(t, storage.PutProofConfig(ctx, store, storage.ProofConfig{
		RequireProof:      requireProof,
		RequiredProofType: mconsts.ProofTypeGroth16,
		BatchWindowMs:     5_000,
		ProofDeadlineMs:   10_000,
		ProverAuthority:   actor,
	}))
}

func TestClearBatchFailsWithoutReveal(t *testing.T) {
	ctx := context.Background()
	actor := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	var marketID ids.ID
	marketID[31] = 7
	seedPrivacyState(t, store, actor, marketID, false)

	fills := sha256.Sum256([]byte("fills"))
	_, err := (&ClearBatch{
		MarketID:    marketID,
		WindowID:    5_000,
		ClearPrice:  1025,
		TotalVolume: 1,
		FillsHash:   fills[:],
	}).Execute(ctx, nil, store, time.Now().UnixMilli(), actor, ids.Empty)
	require.ErrorIs(t, err, storage.ErrRevealThresholdNotMet)
}

func TestRevealThenClearWithoutProofWhenNotRequired(t *testing.T) {
	ctx := context.Background()
	actor := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	var marketID ids.ID
	marketID[30] = 9
	seedPrivacyState(t, store, actor, marketID, false)

	_, err := (&RevealBatch{
		MarketID:        marketID,
		WindowID:        5_000,
		DecryptionShare: []byte("local-window-key-share"),
		ValidatorIndex:  0,
	}).Execute(ctx, nil, store, time.Now().UnixMilli(), actor, ids.Empty)
	require.NoError(t, err)

	n, err := storage.GetRevealShareCount(ctx, store, marketID, 5_000)
	require.NoError(t, err)
	require.Equal(t, uint64(1), n)

	fills := sha256.Sum256([]byte("fills"))
	_, err = (&ClearBatch{
		MarketID:    marketID,
		WindowID:    5_000,
		ClearPrice:  1025,
		TotalVolume: 1,
		FillsHash:   fills[:],
	}).Execute(ctx, nil, store, time.Now().UnixMilli(), actor, ids.Empty)
	require.NoError(t, err)
}

func TestRevealSameValidatorDoesNotDoubleCount(t *testing.T) {
	ctx := context.Background()
	actor := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	var marketID ids.ID
	marketID[29] = 3
	seedPrivacyState(t, store, actor, marketID, false)

	share := &RevealBatch{
		MarketID:        marketID,
		WindowID:        10_000,
		DecryptionShare: []byte("share-a"),
		ValidatorIndex:  0,
	}
	_, err := share.Execute(ctx, nil, store, time.Now().UnixMilli(), actor, ids.Empty)
	require.NoError(t, err)
	share.DecryptionShare = []byte("share-b")
	_, err = share.Execute(ctx, nil, store, time.Now().UnixMilli(), actor, ids.Empty)
	require.NoError(t, err)
	n, err := storage.GetRevealShareCount(ctx, store, marketID, 10_000)
	require.NoError(t, err)
	require.Equal(t, uint64(1), n)
}
