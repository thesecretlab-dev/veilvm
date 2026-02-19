package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	vgenesis "github.com/ava-labs/hypersdk/examples/veilvm/genesis"
	"github.com/ava-labs/hypersdk/fees"
	hgenesis "github.com/ava-labs/hypersdk/genesis"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--vm-id" {
		b := make([]byte, 32)
		copy(b, []byte("veilvm"))
		vmID, _ := ids.ToID(b)
		fmt.Println(vmID.String())
		return
	}

	// Generate a fresh ed25519 key
	priv, err := ed25519.GeneratePrivateKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate key: %v\n", err)
		os.Exit(1)
	}

	pub := priv.PublicKey()
	addr := auth.NewED25519Address(pub)

	fmt.Fprintf(os.Stderr, "=== VEIL Genesis Key ===\n")
	fmt.Fprintf(os.Stderr, "Private Key (hex): %s\n", hex.EncodeToString(priv[:]))
	fmt.Fprintf(os.Stderr, "Public Key (hex):  %s\n", hex.EncodeToString(pub[:]))
	fmt.Fprintf(os.Stderr, "Address:           %s\n", addr)

	// Build genesis
	allocs := []*hgenesis.CustomAllocation{
		{
			Address: addr,
			Balance: 49_549_950, // 5.0% launch float from 990,999,000 total supply
		},
	}

	g := &vgenesis.Genesis{
		StateBranchFactor: merkledb.BranchFactor16,
		CustomAllocation:  allocs,
		Rules:             hgenesis.NewDefaultRules(),
		Tokenomics: &vgenesis.Tokenomics{
			TotalSupply: 990_999_000,

			Governance:    addr,
			Operations:    addr,
			MintAuthority: addr,

			COLVaultLocked: 900_000_000,
			COLVaultLive:   41_449_050,

			MaxReleaseBips:      15,     // 0.15% epoch release cap
			ReleaseEpochSeconds: 86_400, // daily epoch

			FeeRouterMSRBBips: 7_000,
			FeeRouterCOLBips:  2_000,
			FeeRouterOpsBips:  1_000,

			VAIDebtCeiling:      2_000_000,
			VAIEpochMintLimit:   100_000,
			VAIMintEpochSeconds: 3_600,

			BackingFloorBips:  10_000,
			VEILLtvBips:       3_000,
			WVEILLtvBips:      3_500,
			WSVEILLtvBips:     0,
			VEILHaircutBips:   6_000,
			WVEILHaircutBips:  5_500,
			WSVEILHaircutBips: 10_000,

			ExogenousReserveInit: 2_000_000,
			VAIBufferInit:        0,

			RequireBatchProof: true,
			RequiredProofType: mconsts.ProofTypeGroth16,
			BatchWindowMs:     5_000,
			ProofDeadlineMs:   10_000,
			ProverAuthority:   addr,
		},
	}

	// Tune for VEIL: faster blocks, larger throughput
	g.Rules.MinBlockGap = 100        // 100ms between blocks
	g.Rules.MinEmptyBlockGap = 500   // 500ms for empty blocks
	g.Rules.ValidityWindow = 120_000 // 2 min validity
	g.Rules.MaxActionsPerTx = 16
	g.Rules.MaxOutputsPerAction = 1

	// Open up throughput limits for testnet
	// Use MaxInt64 (not MaxUint64) so JSON output stays JS-safe (no precision loss)
	const jsMax = 9_007_199_254_740_991 // Number.MAX_SAFE_INTEGER
	g.Rules.WindowTargetUnits = fees.Dimensions{jsMax, jsMax, jsMax, jsMax, jsMax}
	g.Rules.MaxBlockUnits = fees.Dimensions{1_800_000, jsMax, jsMax, jsMax, jsMax}

	g.Rules.NetworkID = 0
	g.Rules.ChainID = ids.Empty

	out, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal genesis: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}
