package actions

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
	"github.com/ava-labs/hypersdk/auth"
)

func genesisActor(t *testing.T) codec.Address {
	t.Helper()
	pk, err := hex.DecodeString("637404e6722a0e55a27fd82dcd29f3f0faa6f13d930f32f759e3b8412c4956aeee9d3919f004304c2d44dbc9121f6559fefb9b9c25daec749b0f18f605614461")
	require.NoError(t, err)
	return auth.NewED25519Address(ed25519.PrivateKey(pk).PublicKey())
}

func seedMintState(t *testing.T, mu state.Mutable, actor codec.Address) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, storage.PutVAIConfig(ctx, mu, storage.VAIConfig{
		MintAuthority:    actor,
		DebtCeiling:      2_000_000,
		EpochMintLimit:   100_000,
		MintEpochSeconds: 3600,
	}))
	require.NoError(t, storage.PutVAIState(ctx, mu, storage.VAIState{}))
	require.NoError(t, storage.PutRiskConfig(ctx, mu, storage.RiskConfig{
		BackingFloorBips: 10_000,
	}))
	require.NoError(t, storage.PutReserveState(ctx, mu, storage.ReserveState{
		ExogenousReserve: 2_000_000,
	}))
}

func TestMintVAIRoundTripStateKeys(t *testing.T) {
	actor := genesisActor(t)
	orig := &MintVAI{To: actor, Amount: 10_000}
	un, err := UnmarshalMintVAI(orig.Bytes())
	require.NoError(t, err)
	got := un.(*MintVAI)
	require.Equal(t, orig.To, got.To)
	require.Equal(t, orig.Amount, got.Amount)
	require.Equal(t, orig.StateKeys(actor, ids.Empty), got.StateKeys(actor, ids.Empty))
}

func TestMintVAIUnscoped(t *testing.T) {
	ctx := context.Background()
	actor := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	seedMintState(t, store, actor)
	mint := &MintVAI{To: actor, Amount: 10_000}
	_, err := mint.Execute(ctx, nil, store, time.Now().UnixMilli(), actor, ids.Empty)
	require.NoError(t, err)
	bal, err := storage.GetVAIBalance(ctx, store, actor)
	require.NoError(t, err)
	require.Equal(t, uint64(10_000), bal)
}

func TestMintVAIScopedStateKeys(t *testing.T) {
	ctx := context.Background()
	actor := genesisActor(t)
	store := chaintest.NewInMemoryStore()
	seedMintState(t, store, actor)
	mint := &MintVAI{To: actor, Amount: 10_000}
	keys := mint.StateKeys(actor, ids.Empty)
	t.Logf("state key count=%d", len(keys))
	for k, p := range keys {
		t.Logf("declared %x perm=%d", []byte(k), p)
	}
	ts := tstate.New(16)
	tsv := ts.NewView(keys, store, len(keys))
	_, err := mint.Execute(ctx, nil, tsv, time.Now().UnixMilli(), actor, ids.Empty)
	require.NoError(t, err, "Execute under declared StateKeys")
}
