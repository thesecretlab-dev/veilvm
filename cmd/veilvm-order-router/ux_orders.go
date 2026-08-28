package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/examples/veilvm/actions"
)

type marketRec struct {
	MarketID     string `json:"marketId"`
	Question     string `json:"question"`
	CreatedAt    string `json:"createdAt"`
	LastTx       string `json:"lastTx,omitempty"`
	Source       string `json:"sourceName"`
	LastWindowID uint64 `json:"lastWindowId,omitempty"`
}

type uxOrderReq struct {
	MarketID        string  `json:"marketId"`
	Side            string  `json:"side"`
	Outcome         string  `json:"outcome"`
	AmountUsd       float64 `json:"amountUsd"`
	WalletAddress   string  `json:"walletAddress"`
	WalletSignature string  `json:"walletSignature"`
	WalletNonce     string  `json:"walletNonce"`
	NativeNetwork   string  `json:"nativeNetwork"`
	RoutingFeeBps   int     `json:"routingFeeBps"`
	Question        string  `json:"question"`
}

func (r *router) authorized(req *http.Request) bool {
	if req.Header.Get("x-relay-secret") == r.secret {
		return true
	}
	authz := req.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") && strings.TrimSpace(authz[7:]) == r.secret {
		return true
	}
	return false
}

func (r *router) handleMarkets(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, 200, map[string]any{"markets": r.listMarkets()})
}

func (r *router) handleUXOrder(w http.ResponseWriter, req *http.Request) error {
	var in uxOrderReq
	if err := decodeJSON(req, &in); err != nil {
		return err
	}
	net := strings.ToLower(strings.TrimSpace(in.NativeNetwork))
	if net == "polygon" || net == "polygon_native" {
		writeJSON(w, 501, map[string]any{
			"accepted":           false,
			"status":             "passthrough_only",
			"message":            "Polymarket/Polygon is catalog-only. Settlement is VeilVM native, or trade on Polymarket.",
			"errorCode":          "POLYGON_PASSTHROUGH_CATALOG_ONLY",
			"nativeNetwork":      "polygon",
			"settlementNetwork":  "polygon",
			"routingFeeBps":      in.RoutingFeeBps,
			"orderId":            "",
			"veilTxHash":         "",
		})
		return nil
	}
	if in.Side != "buy" && in.Side != "sell" {
		return fmt.Errorf("side must be buy or sell")
	}
	if in.Outcome != "yes" && in.Outcome != "no" {
		return fmt.Errorf("outcome must be yes or no")
	}
	if in.AmountUsd <= 0 {
		return fmt.Errorf("amountUsd must be > 0")
	}
	if strings.TrimSpace(in.WalletAddress) == "" {
		return fmt.Errorf("walletAddress required")
	}
	if strings.TrimSpace(in.WalletSignature) == "" || strings.TrimSpace(in.WalletNonce) == "" {
		return fmt.Errorf("walletSignature and walletNonce required")
	}
	if _, err := parseNonceHex(in.WalletNonce); err != nil {
		return err
	}
	if err := checkRouting("veil_native", in.RoutingFeeBps); err != nil {
		return err
	}

	marketID, question, err := r.ensureMarket(req, in.MarketID, in.Question)
	if err != nil {
		return err
	}

	wallet := strings.ToLower(strings.TrimSpace(in.WalletAddress))
	nonce := strings.ToLower(strings.TrimSpace(in.WalletNonce))
	msg := nativeOrderMessage(r.chainID, marketID.String(), in.Side, in.Outcome, wallet, nonce, in.AmountUsd)
	if err := verifyWalletSig(wallet, msg, in.WalletSignature); err != nil {
		return err
	}

	windowID, _ := r.nextWindow(time.Now().UnixMilli())
	env, commit, nullifier, err := r.buildOpaqueOrderEnvelope(windowID, marketID.String(), in.Side, in.Outcome, in.AmountUsd, wallet, nonce)
	if err != nil {
		return err
	}
	txID, err := r.submit(req.Context(), "commit_order", &actions.CommitOrder{
		MarketID:   marketID,
		WindowID:   windowID,
		Envelope:   env,
		Commitment: commit,
	})
	if err != nil {
		return err
	}
	r.remember(nullifier, txID)
	r.rememberMarket(marketRec{
		MarketID:     marketID.String(),
		Question:     question,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		LastTx:       txID,
		Source:       "VEIL native",
		LastWindowID: windowID,
	})
	writeJSON(w, 200, map[string]any{
		"accepted":           true,
		"status":             "committed",
		"message":            "committed on VeilVM (opaque envelope; not cleared)",
		"windowId":           windowID,
		"orderId":            nullifier,
		"veilTxHash":         txID,
		"oracleTxHash":       "",
		"errorCode":          "",
		"fillPrice":          0,
		"timestamp":          time.Now().UnixMilli(),
		"requiredVeil":       0,
		"balanceVeil":        0,
		"nativeNetwork":      "veil",
		"settlementNetwork":  "veil",
		"routingFeeBps":      0,
		"liquiditySufficient": true,
		"marketId":           marketID.String(),
	})
	return nil
}

func (r *router) ensureMarket(req *http.Request, rawID, question string) (ids.ID, string, error) {
	rawID = strings.TrimSpace(rawID)
	if rawID != "" {
		id, err := parseID(rawID)
		if err == nil && id != ids.Empty {
			if rec, ok := r.lookupMarket(id.String()); ok {
				return id, rec.Question, nil
			}
			return id, question, nil
		}
	}
	if strings.TrimSpace(question) == "" {
		question = "VEIL native market"
	}
	var in createMarketReq
	in.Question = question
	in.Outcomes = 2
	in.CreatorBond = 1
	in.ResolutionTime = time.Now().Unix() + 86_400
	sum := sha256Sum([]byte(fmt.Sprintf("ux-market:%d:%s", time.Now().UnixNano(), question)))
	var marketID ids.ID
	copy(marketID[:], sum[:])
	txID, err := r.submit(req.Context(), "create_market", &actions.CreateMarket{
		MarketID:       marketID,
		Question:       []byte(question),
		Outcomes:       2,
		ResolutionTime: in.ResolutionTime,
		CreatorBond:    1,
	})
	if err != nil {
		return ids.Empty, "", err
	}
	r.rememberMarket(marketRec{
		MarketID:  marketID.String(),
		Question:  question,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		LastTx:    txID,
		Source:    "VEIL native",
	})
	return marketID, question, nil
}

func (r *router) buildOpaqueOrderEnvelope(windowID uint64, marketID, side, outcome string, amount float64, wallet, nonceHex string) (env, commit []byte, nullifier string, err error) {
	nonce, err := parseNonceHex(nonceHex)
	if err != nil {
		return nil, nil, "", err
	}
	body := fmt.Sprintf("veil-order-v1|%s|%s|%s|%.8f|%s|%x", marketID, side, outcome, amount, wallet, nonce)
	key, err := r.windows.key(windowID)
	if err != nil {
		return nil, nil, "", err
	}
	env, err = sealEnvelope(key, []byte(body))
	if err != nil {
		return nil, nil, "", err
	}
	if envelopeLooksPlaintext(env) {
		return nil, nil, "", fmt.Errorf("refusing to commit plaintext envelope")
	}
	if len(env) < minEnvelopeSize {
		return nil, nil, "", fmt.Errorf("sealed envelope too small: %d", len(env))
	}
	sum := sha256Sum(env)
	commit = sum[:]
	nf := sha256Sum(nonce)
	nullifier = "0x" + fmt.Sprintf("%x", nf[:])
	return env, commit, nullifier, nil
}

func sha256Sum(b []byte) [32]byte {
	return sha256.Sum256(b)
}

func (r *router) rememberMarket(rec marketRec) {
	if rec.MarketID == "" {
		return
	}
	if rec.Source == "" {
		rec.Source = "VEIL native"
	}
	r.mu.Lock()
	r.markets[rec.MarketID] = rec
	path := r.marketsFile
	snapshot := make([]marketRec, 0, len(r.markets))
	for _, m := range r.markets {
		snapshot = append(snapshot, m)
	}
	r.mu.Unlock()
	r.persistMarkets(path, snapshot)
}

func (r *router) lookupMarket(id string) (marketRec, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.markets[id]
	return m, ok
}

func (r *router) listMarkets() []marketRec {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]marketRec, 0, len(r.markets))
	for _, m := range r.markets {
		out = append(out, m)
	}
	return out
}

func (r *router) loadMarkets() {
	if r.marketsFile == "" {
		r.marketsFile = filepath.Join(".local", "native-markets.json")
	}
	raw, err := os.ReadFile(r.marketsFile)
	if err != nil {
		return
	}
	var list []marketRec
	if json.Unmarshal(raw, &list) != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range list {
		if m.MarketID != "" {
			r.markets[m.MarketID] = m
		}
	}
}

func (r *router) persistMarkets(path string, list []marketRec) {
	if path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	raw, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o644)
}
