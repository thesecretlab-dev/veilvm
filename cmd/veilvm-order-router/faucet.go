package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/veilvm/actions"
)

const defaultFaucetAmount uint64 = 25_000

func (r *router) actorHex() string {
	s := strings.TrimSpace(fmt.Sprint(r.actor))
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strings.ToLower(s)
	}
	return "0x" + hex.EncodeToString(r.actor[:])
}

func (r *router) veilRPC(method string, params any) (json.RawMessage, error) {
	url := strings.TrimSuffix(r.nodeURL, "/") + "/ext/bc/" + r.chainID + "/veilapi"
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var out struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Error != nil && out.Error.Message != "" {
		return nil, fmt.Errorf("%s", out.Error.Message)
	}
	return out.Result, nil
}

func (r *router) nativeBalance(addr string) (veil uint64, vai uint64) {
	var wrap struct {
		Amount uint64 `json:"amount"`
	}
	if raw, err := r.veilRPC("veilvm.balance", map[string]any{"address": addr}); err == nil {
		_ = json.Unmarshal(raw, &wrap)
		veil = wrap.Amount
	}
	wrap.Amount = 0
	if raw, err := r.veilRPC("veilvm.vaibalance", map[string]any{"address": addr}); err == nil {
		_ = json.Unmarshal(raw, &wrap)
		vai = wrap.Amount
	}
	return veil, vai
}

func parseNativeAddr(s string) (codec.Address, error) {
	raw := mustHex(s)
	var a codec.Address
	switch {
	case len(raw) == len(a):
		copy(a[:], raw)
	case len(raw) == len(a)+1 && raw[0] == 0x00:
		copy(a[:], raw[1:])
	case len(raw) == 33:
		copy(a[:], raw[len(raw)-len(a):])
	default:
		return a, fmt.Errorf("to must be a HyperSDK address (%d bytes, got %d)", len(a), len(raw))
	}
	return a, nil
}

func (r *router) handleFaucet(w http.ResponseWriter, req *http.Request) error {
	var in struct {
		To     string `json:"to"`
		Amount uint64 `json:"amount"`
	}
	if req.Body != nil {
		_ = decodeJSON(req, &in)
	}
	amount := in.Amount
	if amount == 0 {
		amount = defaultFaucetAmount
	}
	if amount > 1_000_000 {
		return fmt.Errorf("faucet amount too large")
	}
	toHex := strings.TrimSpace(in.To)
	var to codec.Address
	if toHex == "" {
		to = r.actor
		toHex = r.actorHex()
	} else {
		parsed, err := parseNativeAddr(toHex)
		if err != nil {
			return err
		}
		to = parsed
		toHex = "0x" + hex.EncodeToString(to[:])
		if len(mustHex(strings.TrimSpace(in.To))) == 33 {
			toHex = strings.ToLower(strings.TrimSpace(in.To))
			if !strings.HasPrefix(toHex, "0x") {
				toHex = "0x" + toHex
			}
		}
	}
	// A no-op transfer to self still hits Deduct and local fee-topup, recapping the actor.
	txID, err := r.submit(req.Context(), "faucet_transfer", &actions.Transfer{
		To:    to,
		Value: amount,
		Memo:  []byte("local-faucet"),
	})
	if err != nil {
		return err
	}
	veil, vai := r.nativeBalance(toHex)
	writeJSON(w, 200, map[string]any{
		"accepted":   true,
		"veilTxHash": txID,
		"to":         toHex,
		"amount":     amount,
		"veil":       veil,
		"vai":        vai,
		"note":       "Local faucet. HyperSDK VEIL, not anvil ETH. Not Fuji.",
	})
	return nil
}

func (r *router) writeHealth(w http.ResponseWriter) {
	actor := r.actorHex()
	veil, vai := r.nativeBalance(actor)
	writeJSON(w, 200, map[string]any{
		"ok":          true,
		"chainId":     r.chainID,
		"markets":     len(r.listMarkets()),
		"proverReady": r.proverInst != nil,
		"actor":       actor,
		"veil":        veil,
		"vai":         vai,
		"local":       true,
		"note":        "Router signs VeilVM txs as this actor. MetaMask 31337 is companion rails, not VEIL.",
	})
}
