package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/examples/veilvm/actions"
	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	"github.com/ava-labs/hypersdk/examples/veilvm/zk"
)

const defaultBatchWindowMs int64 = 5_000

type settleReq struct {
	MarketID    string `json:"marketId"`
	WindowID    uint64 `json:"windowId"`
	ClearPrice  uint64 `json:"clearPrice"`
	TotalVolume uint64 `json:"totalVolume"`
}

func (r *router) nextWindow(nowMs int64) (windowID uint64, closeMs int64) {
	batch := r.batchWindowMs
	if batch <= 0 {
		batch = defaultBatchWindowMs
	}
	q := nowMs / batch
	closeMs = (q + 1) * batch
	if closeMs-nowMs < 1500 {
		closeMs += batch
	}
	return uint64(closeMs), closeMs
}

func (r *router) prover() (*zk.Prover, error) {
	r.proverOnce.Do(func() {
		pk := envOr("ORDER_GROTH16_PK_PATH", filepath.Join("zk-fixture-new", "groth16_shielded_ledger_pk.bin"))
		cache := envOr("ORDER_GROTH16_CCS_PATH", filepath.Join(".local", "zk", "groth16_shielded-ledger-v1-ext.ccs.bin"))
		log.Printf("loading groth16 prover pk=%s cache=%s", pk, cache)
		r.proverInst, r.proverErr = zk.LoadGroth16Prover(pk, cache, mconsts.ProofCircuitShieldedLedgerV1)
		if r.proverErr != nil {
			log.Printf("groth16 prover load failed: %v", r.proverErr)
			return
		}
		log.Printf("groth16 prover ready")
	})
	return r.proverInst, r.proverErr
}

func (r *router) handleSettleBatch(w http.ResponseWriter, req *http.Request) error {
	var in settleReq
	if err := decodeJSON(req, &in); err != nil {
		return err
	}
	marketID, err := parseID(in.MarketID)
	if err != nil {
		return fmt.Errorf("marketId: %w", err)
	}
	windowID := in.WindowID
	if windowID == 0 {
		if rec, ok := r.lookupMarket(marketID.String()); ok && rec.LastWindowID != 0 {
			windowID = rec.LastWindowID
		} else {
			return fmt.Errorf("windowId required (no last commit on this market)")
		}
	}
	clearPrice := in.ClearPrice
	if clearPrice == 0 {
		clearPrice = 1025
	}
	totalVolume := in.TotalVolume
	if totalVolume == 0 {
		totalVolume = 1
	}

	now := time.Now().UnixMilli()
	closeMs := int64(windowID)
	if closeMs%defaultBatchWindowMs != 0 {
		_, closeMs = r.nextWindow(now)
	}

	key, err := r.windows.key(windowID)
	if err != nil {
		return fmt.Errorf("window key: %w", err)
	}
	revealTx, err := r.submit(req.Context(), "reveal_batch", &actions.RevealBatch{
		MarketID:        marketID,
		WindowID:        windowID,
		DecryptionShare: key,
		ValidatorIndex:  0,
	})
	if err != nil {
		return err
	}

	fillsHash := localFillsHash(marketID, windowID, clearPrice, totalVolume)
	prover, err := r.prover()
	if err != nil {
		return fmt.Errorf("prover: %w", err)
	}
	proveStart := time.Now()
	proof, err := prover.ProveClear(marketID, windowID, clearPrice, totalVolume, fillsHash)
	if err != nil {
		return fmt.Errorf("prove: %w", err)
	}
	proveMs := time.Since(proveStart).Milliseconds()

	now = time.Now().UnixMilli()
	if now < closeMs {
		wait := time.Duration(closeMs-now) * time.Millisecond
		log.Printf("settle waiting %s for window close %d", wait, closeMs)
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-req.Context().Done():
			return req.Context().Err()
		case <-timer.C:
		}
		now = time.Now().UnixMilli()
	}
	if now > closeMs+10_000 {
		return fmt.Errorf("proof deadline missed: now=%d close=%d (commit into a new window)", now, closeMs)
	}

	proofTx, err := r.submit(req.Context(), "submit_batch_proof", &actions.SubmitBatchProof{
		MarketID:         marketID,
		WindowID:         windowID,
		WindowCloseAtMs:  closeMs,
		ProofType:        mconsts.ProofTypeGroth16,
		PublicInputsHash: proof.PublicInputsHash,
		FillsHash:        fillsHash,
		Proof:            proof.Envelope,
	})
	if err != nil {
		return err
	}
	clearTx, err := r.submit(req.Context(), "clear_batch", &actions.ClearBatch{
		MarketID:    marketID,
		WindowID:    windowID,
		ClearPrice:  clearPrice,
		TotalVolume: totalVolume,
		FillsHash:   fillsHash,
	})
	if err != nil {
		return err
	}

	writeJSON(w, 200, map[string]any{
		"accepted":         true,
		"status":           "cleared",
		"message":          "revealed, proof-gated, cleared",
		"marketId":         marketID.String(),
		"windowId":         windowID,
		"windowCloseAtMs":  closeMs,
		"clearPrice":       clearPrice,
		"totalVolume":      totalVolume,
		"fillsHash":        "0x" + hex.EncodeToString(fillsHash),
		"commitmentsHash":  "0x" + hex.EncodeToString(proof.CommitmentsHash),
		"nullifiersHash":   "0x" + hex.EncodeToString(proof.NullifiersHash),
		"prevStateRoot":    "0x" + hex.EncodeToString(proof.PrevStateRoot),
		"nextStateRoot":    "0x" + hex.EncodeToString(proof.NextStateRoot),
		"publicInputsHash": "0x" + hex.EncodeToString(proof.PublicInputsHash),
		"revealTxHash":     revealTx,
		"proofTxHash":      proofTx,
		"clearTxHash":      clearTx,
		"proveMs":          proveMs,
	})
	return nil
}

func localFillsHash(marketID ids.ID, windowID, clearPrice, totalVolume uint64) []byte {
	buf := make([]byte, 0, 16+ids.IDLen+24)
	buf = append(buf, []byte("VEIL_FILLS_V1")...)
	buf = append(buf, marketID[:]...)
	var scratch [8]byte
	binary.BigEndian.PutUint64(scratch[:], windowID)
	buf = append(buf, scratch[:]...)
	binary.BigEndian.PutUint64(scratch[:], clearPrice)
	buf = append(buf, scratch[:]...)
	binary.BigEndian.PutUint64(scratch[:], totalVolume)
	buf = append(buf, scratch[:]...)
	return sha256Bytes(buf)
}

func (r *router) warmProver() {
	go func() {
		if _, err := r.prover(); err != nil {
			log.Printf("prover warmup: %v", err)
		}
	}()
}

func defaultWindowKeysPath() string {
	if v := os.Getenv("ORDER_WINDOW_KEYS_PATH"); v != "" {
		return v
	}
	return filepath.Join(".local", "window-keys.json")
}
