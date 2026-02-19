// veilvm-smoke: AMM smoke test — creates pool, mints VAI, adds liquidity, swaps, verifies.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/veilvm/actions"
	vmclient "github.com/ava-labs/hypersdk/examples/veilvm/vm"
)

type Report struct {
	Pass    bool   `json:"pass"`
	Error   string `json:"error,omitempty"`
	Steps   []Step `json:"steps"`
	Summary struct {
		PoolBefore  *PoolState    `json:"pool_before,omitempty"`
		PoolAfter   *PoolState    `json:"pool_after,omitempty"`
		LPBefore    uint64        `json:"lp_before"`
		LPAfter     uint64        `json:"lp_after"`
	} `json:"summary"`
}

type Step struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	TxID   string `json:"tx_id,omitempty"`
	Detail string `json:"detail,omitempty"`
	Error  string `json:"error,omitempty"`
}

type PoolState struct {
	Reserve0 uint64 `json:"reserve0"`
	Reserve1 uint64 `json:"reserve1"`
	TotalLP  uint64 `json:"total_lp"`
	FeeBips  uint16 `json:"fee_bips"`
}

func main() {
	nodeURL := os.Getenv("NODE_URL")
	if nodeURL == "" {
		nodeURL = "http://127.0.0.1:9660"
	}
	chainID := os.Getenv("CHAIN_ID")
	if chainID == "" {
		fmt.Fprintf(os.Stderr, "CHAIN_ID env required\n")
		os.Exit(1)
	}
	pkHex := os.Getenv("PRIVATE_KEY")
	if pkHex == "" {
		// Default: genesis key from veilvm-keygen (governance + mintAuthority)
		pkHex = "637404e6722a0e55a27fd82dcd29f3f0faa6f13d930f32f759e3b8412c4956aeee9d3919f004304c2d44dbc9121f6559fefb9b9c25daec749b0f18f605614461"
	}

	report := &Report{}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if err := run(ctx, nodeURL, chainID, pkHex, report); err != nil {
		report.Pass = false
		report.Error = err.Error()
	} else {
		report.Pass = true
	}

	out, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(out))

	if !report.Pass {
		os.Exit(1)
	}
}

func run(ctx context.Context, nodeURL, chainID, pkHex string, report *Report) error {
	baseURL := fmt.Sprintf("%s/ext/bc/%s", nodeURL, chainID)

	// Parse private key
	pkBytes, err := hex.DecodeString(pkHex)
	if err != nil {
		return fmt.Errorf("invalid PRIVATE_KEY hex: %w", err)
	}
	priv := ed25519.PrivateKey(pkBytes)
	factory := auth.NewED25519Factory(priv)
	addr := auth.NewED25519Address(priv.PublicKey())

	// Init clients
	coreClient := jsonrpc.NewJSONRPCClient(baseURL)
	veilClient := vmclient.NewJSONRPCClient(baseURL)

	// Helper: build, sign, submit a single-action tx
	submitAction := func(name string, action chain.Action) (string, error) {
		_, _, chainIDParsed, err := coreClient.Network(ctx)
		if err != nil {
			return "", fmt.Errorf("network: %w", err)
		}
		_ = chainIDParsed

		// Get current timestamp for validity window
		_, _, ts, err := coreClient.Accepted(ctx)
		if err != nil {
			return "", fmt.Errorf("accepted: %w", err)
		}

		// Get unit prices for fee estimation
		unitPrices, err := coreClient.UnitPrices(ctx, true)
		if err != nil {
			return "", fmt.Errorf("unitPrices: %w", err)
		}

		// Estimate max fee (generous)
		maxFee := uint64(0)
		for i := 0; i < len(unitPrices); i++ {
			maxFee += unitPrices[i] * 10000
		}
		if maxFee < 100000 {
			maxFee = 100000
		}

		// Align timestamp to 1000ms boundary (HyperSDK requirement)
		expiry := ts + 60_000
		expiry = (expiry / 1000) * 1000
		if expiry <= ts {
			expiry = ((ts / 1000) + 61) * 1000
		}
		base := chain.Base{
			Timestamp: expiry,
			ChainID:   chainIDParsed,
			MaxFee:    maxFee,
		}

		txBytes, err := chain.SignRawActionBytesTx(
			base,
			[][]byte{action.Bytes()},
			factory,
		)
		if err != nil {
			return "", fmt.Errorf("sign: %w", err)
		}

		txID, err := coreClient.SubmitTx(ctx, txBytes)
		if err != nil {
			return "", fmt.Errorf("submit: %w", err)
		}

		// Wait for inclusion — poll until height advances
		_, h0, _, _ := coreClient.Accepted(ctx)
		for i := 0; i < 30; i++ {
			time.Sleep(1 * time.Second)
			_, h1, _, _ := coreClient.Accepted(ctx)
			if h1 > h0 {
				break
			}
		}
		return txID.String(), nil
	}

	// ── Step 1: Query initial balances ──
	{
		bal, err := veilClient.Balance(ctx, addr)
		if err != nil {
			return fmt.Errorf("balance query: %w", err)
		}
		vaiBal, err := veilClient.VAIBalance(ctx, addr)
		if err != nil {
			// VAI balance may not exist yet
			vaiBal = 0
		}
		report.Steps = append(report.Steps, Step{
			Name:   "query_initial_balances",
			Pass:   true,
			Detail: fmt.Sprintf("VEIL=%d VAI=%d addr=%s", bal, vaiBal, addr),
		})
	}

	// ── Step 2: Create VEIL/VAI pool (asset0=0, asset1=1, fee=30bps) ──
	{
		txID, err := submitAction("create_pool", &actions.CreatePool{
			Asset0:  actions.AssetVEIL,
			Asset1:  actions.AssetVAI,
			FeeBips: 30,
		})
		step := Step{Name: "create_pool", TxID: txID}
		if err != nil {
			step.Error = err.Error()
			step.Pass = false
			report.Steps = append(report.Steps, step)
			return fmt.Errorf("create_pool: %w", err)
		}
		step.Pass = true
		step.Detail = "VEIL/VAI pool created, fee=30bps"
		report.Steps = append(report.Steps, step)
	}

	// ── Step 3: Verify pool via RPC ──
	{
		pool, err := veilClient.Pool(ctx, actions.AssetVEIL, actions.AssetVAI)
		if err != nil {
			return fmt.Errorf("pool query after create: %w", err)
		}
		report.Summary.PoolBefore = &PoolState{
			Reserve0: pool.Reserve0,
			Reserve1: pool.Reserve1,
			TotalLP:  pool.TotalLP,
			FeeBips:  pool.FeeBips,
		}
		report.Steps = append(report.Steps, Step{
			Name:   "query_pool_created",
			Pass:   pool.FeeBips == 30 && pool.Reserve0 == 0 && pool.Reserve1 == 0,
			Detail: fmt.Sprintf("fee=%d reserve0=%d reserve1=%d totalLP=%d", pool.FeeBips, pool.Reserve0, pool.Reserve1, pool.TotalLP),
		})
	}

	// ── Step 4: Mint VAI (we need VAI to add liquidity) ──
	{
		mintAmount := uint64(10_000)
		txID, err := submitAction("mint_vai", &actions.MintVAI{
			To:     addr,
			Amount: mintAmount,
		})
		step := Step{Name: "mint_vai", TxID: txID}
		if err != nil {
			step.Error = err.Error()
			step.Pass = false
			report.Steps = append(report.Steps, step)
			return fmt.Errorf("mint_vai: %w", err)
		}
		step.Pass = true
		step.Detail = fmt.Sprintf("minted %d VAI to %s", mintAmount, addr)
		report.Steps = append(report.Steps, step)
	}

	// ── Step 5: Query LP balance before ──
	lpBefore := uint64(0)
	{
		lp, err := veilClient.LPBalance(ctx, actions.AssetVEIL, actions.AssetVAI, addr)
		if err != nil {
			lp = 0
		}
		lpBefore = lp
		report.Summary.LPBefore = lpBefore
		report.Steps = append(report.Steps, Step{
			Name:   "query_lp_before",
			Pass:   true,
			Detail: fmt.Sprintf("LP=%d", lp),
		})
	}

	// ── Step 6: Add liquidity (10000 VEIL + 10000 VAI) ──
	{
		txID, err := submitAction("add_liquidity", &actions.AddLiquidity{
			Asset0:  actions.AssetVEIL,
			Asset1:  actions.AssetVAI,
			Amount0: 10_000,
			Amount1: 10_000,
			MinLP:   1,
		})
		step := Step{Name: "add_liquidity", TxID: txID}
		if err != nil {
			step.Error = err.Error()
			step.Pass = false
			report.Steps = append(report.Steps, step)
			return fmt.Errorf("add_liquidity: %w", err)
		}
		step.Pass = true
		step.Detail = "added 10000 VEIL + 10000 VAI"
		report.Steps = append(report.Steps, step)
	}

	// ── Step 7: Verify pool has reserves ──
	{
		pool, err := veilClient.Pool(ctx, actions.AssetVEIL, actions.AssetVAI)
		if err != nil {
			return fmt.Errorf("pool query after liquidity: %w", err)
		}
		ok := pool.Reserve0 > 0 && pool.Reserve1 > 0 && pool.TotalLP > 0
		report.Steps = append(report.Steps, Step{
			Name:   "query_pool_after_liquidity",
			Pass:   ok,
			Detail: fmt.Sprintf("reserve0=%d reserve1=%d totalLP=%d", pool.Reserve0, pool.Reserve1, pool.TotalLP),
		})
		if !ok {
			return fmt.Errorf("pool reserves not set after add_liquidity")
		}
	}

	// ── Step 8: Swap 100 VEIL → VAI ──
	{
		txID, err := submitAction("swap_exact_in", &actions.SwapExactIn{
			AssetIn:      actions.AssetVEIL,
			AssetOut:     actions.AssetVAI,
			AmountIn:     100,
			MinAmountOut: 1,
		})
		step := Step{Name: "swap_veil_to_vai", TxID: txID}
		if err != nil {
			step.Error = err.Error()
			step.Pass = false
			report.Steps = append(report.Steps, step)
			return fmt.Errorf("swap: %w", err)
		}
		step.Pass = true
		step.Detail = "swapped 100 VEIL → VAI"
		report.Steps = append(report.Steps, step)
	}

	// ── Step 9: Final state ──
	{
		pool, err := veilClient.Pool(ctx, actions.AssetVEIL, actions.AssetVAI)
		if err != nil {
			return fmt.Errorf("pool query after swap: %w", err)
		}
		report.Summary.PoolAfter = &PoolState{
			Reserve0: pool.Reserve0,
			Reserve1: pool.Reserve1,
			TotalLP:  pool.TotalLP,
			FeeBips:  pool.FeeBips,
		}

		lp, _ := veilClient.LPBalance(ctx, actions.AssetVEIL, actions.AssetVAI, addr)
		report.Summary.LPAfter = lp

		// Verify invariants: reserve0 increased (VEIL added), reserve1 decreased (VAI removed)
		before := report.Summary.PoolBefore
		after := report.Summary.PoolAfter
		swapOK := after.Reserve0 > 10_000 && after.Reserve1 < 10_000
		report.Steps = append(report.Steps, Step{
			Name:   "verify_swap_invariants",
			Pass:   swapOK,
			Detail: fmt.Sprintf("before(r0=%d,r1=%d) after(r0=%d,r1=%d) lp=%d", before.Reserve0, before.Reserve1, after.Reserve0, after.Reserve1, lp),
		})
		if !swapOK {
			return fmt.Errorf("swap invariants failed")
		}
	}

	// ── Step 10: RPC smoke — pool & lpbalance ──
	{
		pool, err := veilClient.Pool(ctx, actions.AssetVEIL, actions.AssetVAI)
		if err != nil {
			report.Steps = append(report.Steps, Step{Name: "rpc_pool", Pass: false, Error: err.Error()})
			return fmt.Errorf("rpc pool: %w", err)
		}
		report.Steps = append(report.Steps, Step{
			Name:   "rpc_pool",
			Pass:   true,
			Detail: fmt.Sprintf("reserve0=%d reserve1=%d totalLP=%d feeBips=%d", pool.Reserve0, pool.Reserve1, pool.TotalLP, pool.FeeBips),
		})

		lp, err := veilClient.LPBalance(ctx, actions.AssetVEIL, actions.AssetVAI, addr)
		if err != nil {
			report.Steps = append(report.Steps, Step{Name: "rpc_lpbalance", Pass: false, Error: err.Error()})
			return fmt.Errorf("rpc lpbalance: %w", err)
		}
		report.Steps = append(report.Steps, Step{
			Name:   "rpc_lpbalance",
			Pass:   lp > 0,
			Detail: fmt.Sprintf("lp_balance=%d", lp),
		})
	}

	return nil
}
