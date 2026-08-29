package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/veilvm/actions"
	"github.com/ava-labs/hypersdk/examples/veilvm/zk"
)

const (
	defaultListen   = "127.0.0.1:9098"
	defaultNodeURL  = "http://127.0.0.1:9660"
	defaultKeyHex   = "637404e6722a0e55a27fd82dcd29f3f0faa6f13d930f32f759e3b8412c4956aeee9d3919f004304c2d44dbc9121f6559fefb9b9c25daec749b0f18f605614461"
	defaultSecret   = "local-dev-secret"
	minEnvelopeSize = 96
)

type router struct {
	nodeURL     string
	chainID     string
	secret      string
	factory     chain.AuthFactory
	actor       codec.Address
	minEnvelope int
	requireEnv  bool
	evmIngress  bool

	mu          sync.Mutex
	seen        map[string]string
	markets     map[string]marketRec
	marketsFile string
	core        *jsonrpc.JSONRPCClient
	idx         *indexer.Client

	windows        *windowKeys
	batchWindowMs  int64
	proverOnce     sync.Once
	proverInst     *zk.Prover
	proverErr      error
}

type intentReq struct {
	IntentID      string `json:"intentId"`
	MarketKey     string `json:"marketKey"`
	Envelope      string `json:"envelope"`
	Commitment    string `json:"commitment"`
	Nullifier     string `json:"nullifier"`
	MarketType    string `json:"marketType"`
	RoutingFeeBps int    `json:"routingFeeBps"`
	SourceTxHash  string `json:"sourceTxHash"`
	WindowID      uint64 `json:"windowId"`
}

type liqReq struct {
	IntentID     string `json:"intentId"`
	Envelope     string `json:"envelope"`
	Commitment   string `json:"commitment"`
	Nullifier    string `json:"nullifier"`
	Operation    string `json:"operation"`
	Asset0       uint8  `json:"asset0"`
	Asset1       uint8  `json:"asset1"`
	Amount0      uint64 `json:"amount0"`
	Amount1      uint64 `json:"amount1"`
	MinLP        uint64 `json:"minLP"`
	LPAmount     uint64 `json:"lpAmount"`
	MinAmount0   uint64 `json:"minAmount0"`
	MinAmount1   uint64 `json:"minAmount1"`
	AssetIn      uint8  `json:"assetIn"`
	AssetOut     uint8  `json:"assetOut"`
	AmountIn     uint64 `json:"amountIn"`
	MinAmountOut uint64 `json:"minAmountOut"`
	FeeBips      uint16 `json:"feeBips"`
	SourceTxHash string `json:"sourceTxHash"`
}

type createMarketReq struct {
	Question       string `json:"question"`
	Outcomes       uint8  `json:"outcomes"`
	ResolutionTime int64  `json:"resolutionTime"`
	CreatorBond    uint64 `json:"creatorBond"`
	MarketID       string `json:"marketId"`
}

type reply struct {
	Accepted   bool   `json:"accepted"`
	VeilTxHash string `json:"veilTxHash,omitempty"`
	MarketID   string `json:"marketId,omitempty"`
	Error      string `json:"error,omitempty"`
}

func main() {
	chainID := strings.TrimSpace(os.Getenv("ORDER_CHAIN_ID"))
	if chainID == "" {
		chainID = strings.TrimSpace(os.Getenv("CHAIN_ID"))
	}
	if chainID == "" {
		log.Fatal("ORDER_CHAIN_ID (or CHAIN_ID) required")
	}
	pkHex := envOr("ORDER_ROUTER_PRIVATE_KEY", defaultKeyHex)
	pkBytes, err := hex.DecodeString(pkHex)
	if err != nil {
		log.Fatalf("ORDER_ROUTER_PRIVATE_KEY: %v", err)
	}
	priv := ed25519.PrivateKey(pkBytes)
	nodeURL := strings.TrimSuffix(envOr("ORDER_NODE_URL", defaultNodeURL), "/")
	base := fmt.Sprintf("%s/ext/bc/%s", nodeURL, chainID)
	r := &router{
		nodeURL:     nodeURL,
		chainID:     chainID,
		secret:      envOr("ORDER_ROUTER_RELAY_SECRET", defaultSecret),
		factory:     auth.NewED25519Factory(priv),
		actor:       auth.NewED25519Address(priv.PublicKey()),
		minEnvelope: envInt("ORDER_ROUTER_MIN_OPAQUE_ENVELOPE_BYTES", minEnvelopeSize),
		requireEnv:  envBool("ORDER_ROUTER_REQUIRE_OPAQUE_ENVELOPE", true),
		evmIngress:  envBool("ORDER_ROUTER_ENABLE_EVM_INGRESS", true),
		seen:        map[string]string{},
		markets:     map[string]marketRec{},
		marketsFile: envOr("ORDER_MARKETS_PATH", ""),
		core:          jsonrpc.NewJSONRPCClient(base),
		idx:           indexer.NewClient(base),
		windows:       newWindowKeys(defaultWindowKeysPath()),
		batchWindowMs: defaultBatchWindowMs,
	}
	r.loadMarkets()
	r.warmProver()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		r.writeHealth(w)
	})
	mux.HandleFunc("/markets", r.handleMarkets)
	mux.HandleFunc("/orders", r.wrap(r.handleUXOrder, false))
	mux.HandleFunc("/evm/intents/execute", r.wrap(r.handleOrderEVM, true))
	mux.HandleFunc("/evm/liquidity/execute", r.wrap(r.handleLiqEVM, true))
	mux.HandleFunc("/intents/native/execute", r.wrap(r.handleOrder, false))
	mux.HandleFunc("/intents/native/liquidity/execute", r.wrap(r.handleLiq, false))
	mux.HandleFunc("/native/create-market", r.wrap(r.handleCreateMarket, false))
	mux.HandleFunc("/native/faucet", r.wrap(r.handleFaucet, false))
	mux.HandleFunc("/native/mint-vai", r.wrap(r.handleMintVAI, false))
	mux.HandleFunc("/native/burn-vai", r.wrap(r.handleBurnVAI, false))
	mux.HandleFunc("/native/route-fees", r.wrap(r.handleRouteFees, false))
	mux.HandleFunc("/native/release-col", r.wrap(r.handleReleaseCOL, false))
	mux.HandleFunc("/native/settle-batch", r.wrap(r.handleSettleBatch, false))
	addr := envOr("ORDER_ROUTER_LISTEN", defaultListen)
	log.Printf("veilvm-order-router listen=%s chain=%s actor=%s", addr, chainID, auth.NewED25519Address(priv.PublicKey()))
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (r *router) wrap(fn func(http.ResponseWriter, *http.Request) error, evm bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if evm && !r.evmIngress {
			writeJSON(w, 403, reply{Error: "evm ingress disabled"})
			return
		}
		if !r.authorized(req) {
			writeJSON(w, 401, reply{Error: "unauthorized"})
			return
		}
		if err := fn(w, req); err != nil {
			log.Printf("%s %s: %v", req.Method, req.URL.Path, err)
			writeJSON(w, 400, reply{Error: err.Error()})
		}
	}
}

func requireEVMIngress(intentID, sourceTxHash, nullifier string) error {
	if strings.TrimSpace(sourceTxHash) == "" {
		return fmt.Errorf("evm ingress requires sourceTxHash from companion submitIntent")
	}
	if strings.TrimSpace(intentID) == "" {
		return fmt.Errorf("evm ingress requires intentId")
	}
	if strings.TrimSpace(nullifier) == "" {
		return fmt.Errorf("evm ingress requires nullifier")
	}
	return nil
}

func (r *router) handleOrderEVM(w http.ResponseWriter, req *http.Request) error {
	var in intentReq
	if err := decodeJSON(req, &in); err != nil {
		return err
	}
	if err := requireEVMIngress(in.IntentID, in.SourceTxHash, in.Nullifier); err != nil {
		return err
	}
	return r.commitIntent(w, req, in)
}

func (r *router) handleOrder(w http.ResponseWriter, req *http.Request) error {
	var in intentReq
	if err := decodeJSON(req, &in); err != nil {
		return err
	}
	return r.commitIntent(w, req, in)
}

func (r *router) commitIntent(w http.ResponseWriter, req *http.Request, in intentReq) error {
	if err := checkRouting(in.MarketType, in.RoutingFeeBps); err != nil {
		return err
	}
	env, err := r.checkEnvelope(in.Envelope, in.Commitment)
	if err != nil {
		return err
	}
	nullifier := strings.ToLower(strings.TrimSpace(in.Nullifier))
	if hit, ok := r.lookup(nullifier); ok {
		writeJSON(w, 200, reply{Accepted: true, VeilTxHash: hit})
		return nil
	}
	marketID, err := parseID(in.MarketKey)
	if err != nil {
		return fmt.Errorf("marketKey: %w", err)
	}
	windowID := in.WindowID
	if windowID == 0 {
		windowID, _ = r.nextWindow(time.Now().UnixMilli())
	}
	txID, err := r.submit(req.Context(), "commit_order", &actions.CommitOrder{
		MarketID:   marketID,
		WindowID:   windowID,
		Envelope:   env,
		Commitment: mustHex(in.Commitment),
	})
	if err != nil {
		return err
	}
	r.remember(nullifier, txID)
	writeJSON(w, 200, reply{Accepted: true, VeilTxHash: txID})
	return nil
}

func (r *router) handleLiqEVM(w http.ResponseWriter, req *http.Request) error {
	var in liqReq
	if err := decodeJSON(req, &in); err != nil {
		return err
	}
	if err := requireEVMIngress(in.IntentID, in.SourceTxHash, in.Nullifier); err != nil {
		return err
	}
	return r.execLiq(w, req, in)
}

func (r *router) handleLiq(w http.ResponseWriter, req *http.Request) error {
	var in liqReq
	if err := decodeJSON(req, &in); err != nil {
		return err
	}
	return r.execLiq(w, req, in)
}

func (r *router) execLiq(w http.ResponseWriter, req *http.Request, in liqReq) error {
	if r.requireEnv {
		if _, err := r.checkEnvelope(in.Envelope, in.Commitment); err != nil {
			return err
		}
	}
	nullifier := strings.ToLower(strings.TrimSpace(in.Nullifier))
	if hit, ok := r.lookup(nullifier); ok {
		writeJSON(w, 200, reply{Accepted: true, VeilTxHash: hit})
		return nil
	}
	action, name, err := liqAction(in)
	if err != nil {
		return err
	}
	txID, err := r.submit(req.Context(), name, action)
	if err != nil {
		return err
	}
	r.remember(nullifier, txID)
	writeJSON(w, 200, reply{Accepted: true, VeilTxHash: txID})
	return nil
}

func (r *router) handleMintVAI(w http.ResponseWriter, req *http.Request) error {
	var in struct {
		To     string `json:"to"`
		Amount uint64 `json:"amount"`
	}
	if err := decodeJSON(req, &in); err != nil {
		return err
	}
	if in.Amount == 0 {
		in.Amount = 10_000
	}
	pkHex := envOr("ORDER_ROUTER_PRIVATE_KEY", defaultKeyHex)
	pkBytes, err := hex.DecodeString(pkHex)
	if err != nil {
		return err
	}
	to := auth.NewED25519Address(ed25519.PrivateKey(pkBytes).PublicKey())
	if strings.TrimSpace(in.To) != "" {
		raw := mustHex(in.To)
		if len(raw) != len(to) {
			return fmt.Errorf("to must be 33-byte hypersdk address")
		}
		copy(to[:], raw)
	}
	txID, err := r.submit(req.Context(), "mint_vai", &actions.MintVAI{To: to, Amount: in.Amount})
	if err != nil {
		return err
	}
	writeJSON(w, 200, reply{Accepted: true, VeilTxHash: txID})
	return nil
}

func (r *router) handleBurnVAI(w http.ResponseWriter, req *http.Request) error {
	var in struct {
		Amount uint64 `json:"amount"`
	}
	if err := decodeJSON(req, &in); err != nil {
		return err
	}
	if in.Amount == 0 {
		return fmt.Errorf("amount required")
	}
	txID, err := r.submit(req.Context(), "burn_vai", &actions.BurnVAI{Amount: in.Amount})
	if err != nil {
		return err
	}
	writeJSON(w, 200, reply{Accepted: true, VeilTxHash: txID})
	return nil
}

func (r *router) handleRouteFees(w http.ResponseWriter, req *http.Request) error {
	var in struct {
		Amount uint64 `json:"amount"`
	}
	if err := decodeJSON(req, &in); err != nil {
		return err
	}
	if in.Amount == 0 {
		return fmt.Errorf("amount required")
	}
	txID, err := r.submit(req.Context(), "route_fees", &actions.RouteFees{Amount: in.Amount})
	if err != nil {
		return err
	}
	writeJSON(w, 200, reply{Accepted: true, VeilTxHash: txID})
	return nil
}

func (r *router) handleReleaseCOL(w http.ResponseWriter, req *http.Request) error {
	var in struct {
		Amount uint64 `json:"amount"`
	}
	if err := decodeJSON(req, &in); err != nil {
		return err
	}
	if in.Amount == 0 {
		return fmt.Errorf("amount required")
	}
	txID, err := r.submit(req.Context(), "release_col_tranche", &actions.ReleaseCOLTranche{Amount: in.Amount})
	if err != nil {
		return err
	}
	writeJSON(w, 200, reply{Accepted: true, VeilTxHash: txID})
	return nil
}

func (r *router) handleCreateMarket(w http.ResponseWriter, req *http.Request) error {
	var in createMarketReq
	if err := decodeJSON(req, &in); err != nil {
		return err
	}
	if in.Outcomes < 2 {
		in.Outcomes = 2
	}
	if in.CreatorBond == 0 {
		in.CreatorBond = 1
	}
	if in.ResolutionTime == 0 {
		in.ResolutionTime = time.Now().Unix() + 86_400
	}
	var marketID ids.ID
	if strings.TrimSpace(in.MarketID) != "" {
		parsed, err := parseID(in.MarketID)
		if err != nil {
			return err
		}
		marketID = parsed
	} else {
		sum := sha256.Sum256([]byte(fmt.Sprintf("local-market:%d:%s", time.Now().UnixNano(), in.Question)))
		copy(marketID[:], sum[:])
	}
	q := []byte(in.Question)
	if len(q) == 0 {
		q = []byte("local-stack-market")
	}
	txID, err := r.submit(req.Context(), "create_market", &actions.CreateMarket{
		MarketID:       marketID,
		Question:       q,
		Outcomes:       in.Outcomes,
		ResolutionTime: in.ResolutionTime,
		CreatorBond:    in.CreatorBond,
	})
	if err != nil {
		return err
	}
	id := marketID.String()
	r.rememberMarket(marketRec{MarketID: id, Question: string(q), CreatedAt: time.Now().UTC().Format(time.RFC3339), LastTx: txID, Source: "VEIL native"})
	writeJSON(w, 200, reply{Accepted: true, VeilTxHash: txID, MarketID: id})
	return nil
}

func (r *router) submit(ctx context.Context, name string, action chain.Action) (string, error) {
	_, _, chainIDParsed, err := r.core.Network(ctx)
	if err != nil {
		return "", fmt.Errorf("network: %w", err)
	}
	_, _, ts, err := r.core.Accepted(ctx)
	if err != nil {
		return "", fmt.Errorf("accepted: %w", err)
	}
	unitPrices, err := r.core.UnitPrices(ctx, true)
	if err != nil {
		return "", fmt.Errorf("unitPrices: %w", err)
	}
	// Lock only what a local tx actually needs. The old 10_000× / 100_000 floor
	// drained the genesis actor (MaxFee is reserved in full at submit).
	maxFee := uint64(0)
	for i := 0; i < len(unitPrices); i++ {
		maxFee += unitPrices[i] * 48
	}
	if maxFee < 2_000 {
		maxFee = 2_000
	}
	if maxFee > 20_000 {
		maxFee = 20_000
	}
	expiry := ts + 60_000
	expiry = (expiry / 1000) * 1000
	if expiry <= ts {
		expiry = ((ts / 1000) + 61) * 1000
	}
	txBytes, err := chain.SignRawActionBytesTx(chain.Base{
		Timestamp: expiry,
		ChainID:   chainIDParsed,
		MaxFee:    maxFee,
	}, [][]byte{action.Bytes()}, r.factory)
	if err != nil {
		return "", fmt.Errorf("sign %s: %w", name, err)
	}
	txID, err := r.core.SubmitTx(ctx, txBytes)
	if err != nil {
		return "", fmt.Errorf("submit %s: %w", name, err)
	}
	if err := r.waitTx(ctx, txID, name); err != nil {
		return txID.String(), err
	}
	return "0x" + hex.EncodeToString(txID[:]), nil
}

func (r *router) waitTx(ctx context.Context, txID ids.ID, name string) error {
	for i := 0; i < 60; i++ {
		resp, found, err := r.idx.GetTxResults(ctx, txID)
		if err != nil {
			return fmt.Errorf("%s indexer: %w", name, err)
		}
		if found {
			if !resp.Result.Success {
				return fmt.Errorf("%s execution failed: %s", name, string(resp.Result.Error))
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("%s tx result timeout: %s", name, txID)
}

func (r *router) checkEnvelope(envelopeHex, commitmentHex string) ([]byte, error) {
	env := mustHex(envelopeHex)
	if r.requireEnv {
		if len(env) < r.minEnvelope {
			return nil, fmt.Errorf("envelope too small: %d < %d", len(env), r.minEnvelope)
		}
		sum := sha256.Sum256(env)
		want := mustHex(commitmentHex)
		if len(want) != 32 {
			return nil, fmt.Errorf("commitment must be 32 bytes")
		}
		if hex.EncodeToString(sum[:]) != hex.EncodeToString(want) {
			return nil, fmt.Errorf("sha256(envelope) != commitment")
		}
	}
	if len(env) == 0 {
		return nil, fmt.Errorf("envelope empty")
	}
	return env, nil
}

func (r *router) lookup(nullifier string) (string, bool) {
	if nullifier == "" {
		return "", false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.seen[nullifier]
	return v, ok
}

func (r *router) remember(nullifier, txID string) {
	if nullifier == "" {
		return
	}
	r.mu.Lock()
	r.seen[nullifier] = txID
	r.mu.Unlock()
}

func liqAction(in liqReq) (chain.Action, string, error) {
	op := strings.ToLower(strings.TrimSpace(in.Operation))
	switch op {
	case "create_pool", "0":
		fee := in.FeeBips
		if fee == 0 {
			fee = 30
		}
		return &actions.CreatePool{Asset0: in.Asset0, Asset1: in.Asset1, FeeBips: fee}, "create_pool", nil
	case "add_liquidity", "1":
		return &actions.AddLiquidity{
			Asset0: in.Asset0, Asset1: in.Asset1,
			Amount0: in.Amount0, Amount1: in.Amount1, MinLP: in.MinLP,
		}, "add_liquidity", nil
	case "remove_liquidity", "2":
		return &actions.RemoveLiquidity{
			Asset0: in.Asset0, Asset1: in.Asset1,
			LPAmount: in.LPAmount, MinAmount0: in.MinAmount0, MinAmount1: in.MinAmount1,
		}, "remove_liquidity", nil
	case "swap_exact_in", "3":
		return &actions.SwapExactIn{
			AssetIn: in.AssetIn, AssetOut: in.AssetOut,
			AmountIn: in.AmountIn, MinAmountOut: in.MinAmountOut,
		}, "swap_exact_in", nil
	default:
		return nil, "", fmt.Errorf("unsupported liquidity operation %q", in.Operation)
	}
}

func checkRouting(marketType string, bps int) error {
	switch strings.ToLower(strings.TrimSpace(marketType)) {
	case "", "veil_native", "veil":
		if bps != 0 {
			return fmt.Errorf("veil_native routingFeeBps must be 0")
		}
	case "polygon_native", "polygon":
		if bps < 3 {
			return fmt.Errorf("polygon_native routingFeeBps must be >= 3")
		}
	}
	return nil
}

func decodeJSON(req *http.Request, dest any) error {
	defer req.Body.Close()
	b, err := io.ReadAll(io.LimitReader(req.Body, 1<<20))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, dest); err != nil {
		return fmt.Errorf("json: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func parseID(s string) (ids.ID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return ids.Empty, fmt.Errorf("empty id")
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		b, err := hex.DecodeString(s[2:])
		if err != nil {
			return ids.Empty, err
		}
		var id ids.ID
		if len(b) != len(id) {
			return ids.Empty, fmt.Errorf("want 32 bytes, got %d", len(b))
		}
		copy(id[:], b)
		return id, nil
	}
	return ids.FromString(s)
}

func mustHex(s string) []byte {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil
	}
	return b
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func envBool(k string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(k)))
	switch v {
	case "1", "true", "t", "yes", "on":
		return true
	case "0", "false", "f", "no", "off":
		return false
	default:
		return def
	}
}

func envInt(k string, def int) int {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n < 0 {
		return def
	}
	return n
}
