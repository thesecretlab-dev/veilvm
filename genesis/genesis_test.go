package genesis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
)

func TestGenesisJSONTokenomicsFreeze(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "genesis.json"))
	require.NoError(t, err)

	g, _, err := Factory{}.Load(raw, nil, 0, ids.Empty)
	require.NoError(t, err)
	gg := g.(*Genesis)
	require.NotNil(t, gg.Tokenomics)

	require.Equal(t, uint64(990_999_000), gg.Tokenomics.TotalSupply)
	require.Equal(t, uint16(7_000), gg.Tokenomics.FeeRouterMSRBBips)
	require.Equal(t, uint16(2_000), gg.Tokenomics.FeeRouterCOLBips)
	require.Equal(t, uint16(1_000), gg.Tokenomics.FeeRouterOpsBips)
	require.Equal(t, uint16(0), gg.Tokenomics.WSVEILLtvBips)
	require.Equal(t, uint8(mconsts.ProofTypeGroth16), gg.Tokenomics.RequiredProofType)

	require.NoError(t, validateTokenomics(gg.CustomAllocation, gg.Tokenomics))
}

func TestGenesisRejectsWsVEILLTV(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "genesis.json"))
	require.NoError(t, err)
	g, _, err := Factory{}.Load(raw, nil, 0, ids.Empty)
	require.NoError(t, err)
	gg := g.(*Genesis)
	gg.Tokenomics.WSVEILLtvBips = 1
	require.Error(t, validateTokenomics(gg.CustomAllocation, gg.Tokenomics))
}
