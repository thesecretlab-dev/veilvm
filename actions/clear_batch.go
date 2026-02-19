package actions

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"

	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
)

const (
	ClearBatchComputeUnits = 10
	MaxFillsHashSize       = 64
	MaxClearBatchSize      = 2048
)

var (
	ErrFillsHashEmpty                        = errors.New("fills hash is empty")
	ErrFillsHashTooLarge                     = errors.New("fills hash is too large")
	ErrFillsHashWrongSize                    = errors.New("fills hash has invalid size")
	ErrClearPriceZero                        = errors.New("clear price is zero")
	ErrUnmarshalEmptyClearBatch              = errors.New("cannot unmarshal empty bytes as clear_batch")
	_                           chain.Action = (*ClearBatch)(nil)
)

type ClearBatch struct {
	MarketID    ids.ID `serialize:"true" json:"market_id"`
	WindowID    uint64 `serialize:"true" json:"window_id"`
	ClearPrice  uint64 `serialize:"true" json:"clear_price"`
	TotalVolume uint64 `serialize:"true" json:"total_volume"`
	FillsHash   []byte `serialize:"true" json:"fills_hash"`
}

func (*ClearBatch) GetTypeID() uint8 {
	return mconsts.ClearBatchID
}

func (t *ClearBatch) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.MarketKey(t.MarketID)):                  state.Read,
		string(storage.BatchKey(t.MarketID, t.WindowID)):       state.All,
		string(storage.ProofConfigKey()):                       state.Read,
		string(storage.BatchProofKey(t.MarketID, t.WindowID)):  state.Read,
		string(storage.VellumProofKey(t.MarketID, t.WindowID)): state.Read,
	}
}

func (t *ClearBatch) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxClearBatchSize),
		MaxSize: MaxClearBatchSize,
	}
	p.PackByte(mconsts.ClearBatchID)
	if err := codec.LinearCodec.MarshalInto(t, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalClearBatch(bytes []byte) (chain.Action, error) {
	t := &ClearBatch{}
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptyClearBatch
	}
	if bytes[0] != mconsts.ClearBatchID {
		return nil, fmt.Errorf("unexpected clear_batch typeID: %d != %d", bytes[0], mconsts.ClearBatchID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *ClearBatch) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	timestamp int64,
	actor codec.Address,
	_ ids.ID,
) (_ []byte, err error) {
	start := time.Now()
	var verificationDuration time.Duration
	missedDeadline := false
	acceptedAtMs := int64(0)
	defer func() {
		RecordClearMetric(
			t.MarketID,
			t.WindowID,
			acceptedAtMs,
			verificationDuration,
			time.Since(start),
			missedDeadline,
			err,
		)
	}()

	if t.ClearPrice == 0 {
		return nil, ErrClearPriceZero
	}
	if len(t.FillsHash) == 0 {
		return nil, ErrFillsHashEmpty
	}
	if len(t.FillsHash) > MaxFillsHashSize {
		return nil, ErrFillsHashTooLarge
	}
	if len(t.FillsHash) != ExpectedFillsHashSize {
		return nil, ErrFillsHashWrongSize
	}

	// Verify market exists and is active
	status, _, _, _, _, err := storage.GetMarket(ctx, mu, t.MarketID)
	if err != nil {
		return nil, err
	}
	if status != storage.MarketStatusActive {
		return nil, storage.ErrMarketNotActive
	}

	proofCfg, err := storage.GetProofConfig(ctx, mu)
	if err != nil {
		return nil, err
	}
	if proofCfg.RequireProof {
		// In proof-gated mode, only the configured authority may finalize clears.
		if actor != proofCfg.ProverAuthority {
			return nil, storage.ErrUnauthorized
		}

		verifyStart := time.Now()
		defer func() {
			if verificationDuration == 0 {
				verificationDuration = time.Since(verifyStart)
			}
		}()

		record, err := storage.GetBatchProofRecord(ctx, mu, t.MarketID, t.WindowID)
		if err != nil {
			return nil, err
		}
		if record.ProofType != proofCfg.RequiredProofType {
			return nil, storage.ErrProofTypeMismatch
		}
		if record.SubmittedAtMs > record.WindowCloseAtMs+proofCfg.ProofDeadlineMs {
			missedDeadline = true
			return nil, storage.ErrProofDeadlineMissed
		}
		if !bytes.Equal(record.FillsHash, t.FillsHash) {
			return nil, storage.ErrProofFillsMismatch
		}
		proofBytes, err := storage.GetVellumProof(ctx, mu, t.MarketID, t.WindowID)
		if err != nil {
			return nil, err
		}
		commitment := sha256.Sum256(proofBytes)
		if !bytes.Equal(commitment[:], record.ProofCommitment[:]) {
			return nil, storage.ErrProofCommitmentMismatch
		}
		_, circuitID, _, _, _, err := parseProofEnvelope(proofBytes)
		if err != nil {
			return nil, err
		}
		expectedInputsHash, err := computeExpectedPublicInputsHash(
			circuitID,
			t.MarketID,
			t.WindowID,
			t.ClearPrice,
			t.TotalVolume,
			t.FillsHash,
		)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(record.PublicInputsHash, expectedInputsHash[:]) {
			return nil, storage.ErrProofPublicInputsMismatch
		}
		if err := verifyBatchProofInConsensus(record.ProofType, proofBytes, expectedInputsHash[:]); err != nil {
			return nil, err
		}
		verificationDuration = time.Since(verifyStart)
	}

	// Store batch result
	if err := storage.PutBatchResult(ctx, mu, t.MarketID, t.WindowID, t.ClearPrice, t.TotalVolume, t.FillsHash); err != nil {
		return nil, err
	}
	acceptedAtMs = timestamp

	result := &ClearBatchResult{
		ClearPrice:  t.ClearPrice,
		TotalVolume: t.TotalVolume,
	}
	return result.Bytes(), nil
}

func (*ClearBatch) ComputeUnits(chain.Rules) uint64 {
	return ClearBatchComputeUnits
}

func (*ClearBatch) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

var _ codec.Typed = (*ClearBatchResult)(nil)

type ClearBatchResult struct {
	ClearPrice  uint64 `serialize:"true" json:"clear_price"`
	TotalVolume uint64 `serialize:"true" json:"total_volume"`
}

func (*ClearBatchResult) GetTypeID() uint8 {
	return mconsts.ClearBatchID
}

func (t *ClearBatchResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, 256),
		MaxSize: MaxClearBatchSize,
	}
	p.PackByte(mconsts.ClearBatchID)
	_ = codec.LinearCodec.MarshalInto(t, p)
	return p.Bytes
}

func UnmarshalClearBatchResult(b []byte) (codec.Typed, error) {
	t := &ClearBatchResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}

func computeExpectedPublicInputsHash(
	circuitID string,
	marketID ids.ID,
	windowID uint64,
	clearPrice uint64,
	totalVolume uint64,
	fillsHash []byte,
) ([32]byte, error) {
	circuitID = strings.TrimSpace(circuitID)
	switch circuitID {
	case "", mconsts.ProofCircuitClearHashV1:
		return ComputeClearPublicInputsHash(
			marketID,
			windowID,
			clearPrice,
			totalVolume,
			fillsHash,
		), nil
	case mconsts.ProofCircuitShieldedLedgerV1:
		return ComputeShieldedLedgerPublicInputsHash(
			marketID,
			windowID,
			clearPrice,
			totalVolume,
			fillsHash,
		), nil
	default:
		return [32]byte{}, storage.ErrUnsupportedProofCircuit
	}
}
