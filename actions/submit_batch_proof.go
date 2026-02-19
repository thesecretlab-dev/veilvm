package actions

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

const (
	SubmitBatchProofComputeUnits = 8
	MaxProofBytesSize            = 131072
	ExpectedProofHashSize        = 32
	MaxSubmitBatchProofSize      = MaxProofBytesSize + 4096
)

var (
	ErrUnmarshalEmptySubmitBatchProof              = errors.New("cannot unmarshal empty bytes as submit_batch_proof")
	_                                 chain.Action = (*SubmitBatchProof)(nil)
)

type SubmitBatchProof struct {
	MarketID         ids.ID `serialize:"true" json:"market_id"`
	WindowID         uint64 `serialize:"true" json:"window_id"`
	WindowCloseAtMs  int64  `serialize:"true" json:"window_close_at_ms"`
	ProofType        uint8  `serialize:"true" json:"proof_type"`
	PublicInputsHash []byte `serialize:"true" json:"public_inputs_hash"`
	FillsHash        []byte `serialize:"true" json:"fills_hash"`
	Proof            []byte `serialize:"true" json:"proof"`
}

func (*SubmitBatchProof) GetTypeID() uint8 {
	return mconsts.SubmitBatchProofID
}

func (a *SubmitBatchProof) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.MarketKey(a.MarketID)):                  state.Read,
		string(storage.ProofConfigKey()):                       state.Read,
		string(storage.BatchProofKey(a.MarketID, a.WindowID)):  state.All,
		string(storage.VellumProofKey(a.MarketID, a.WindowID)): state.All,
		string(storage.BloodswornKey(actor)):                   state.All,
		string(storage.GlyphKey(a.MarketID, a.WindowID)):       state.All,
	}
}

func (a *SubmitBatchProof) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxSubmitBatchProofSize),
		MaxSize: MaxSubmitBatchProofSize,
	}
	p.PackByte(mconsts.SubmitBatchProofID)
	if err := codec.LinearCodec.MarshalInto(a, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalSubmitBatchProof(bytes []byte) (chain.Action, error) {
	a := &SubmitBatchProof{}
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptySubmitBatchProof
	}
	if bytes[0] != mconsts.SubmitBatchProofID {
		return nil, fmt.Errorf("unexpected submit_batch_proof typeID: %d != %d", bytes[0], mconsts.SubmitBatchProofID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		a,
	); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *SubmitBatchProof) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	timestamp int64,
	actor codec.Address,
	txID ids.ID,
) (_ []byte, err error) {
	start := time.Now()
	missedDeadline := false
	defer func() {
		RecordProofSubmitMetric(
			a.MarketID,
			a.WindowID,
			a.WindowCloseAtMs,
			timestamp,
			time.Since(start),
			missedDeadline,
			err,
		)
	}()

	if a.WindowCloseAtMs <= 0 {
		return nil, storage.ErrInvalidProofEnvelope
	}
	if a.ProofType == 0 {
		return nil, storage.ErrInvalidProofEnvelope
	}
	if len(a.PublicInputsHash) != ExpectedProofHashSize {
		return nil, storage.ErrInvalidProofEnvelope
	}
	if len(a.FillsHash) != ExpectedFillsHashSize {
		return nil, storage.ErrInvalidProofEnvelope
	}
	if len(a.Proof) == 0 || len(a.Proof) > MaxProofBytesSize {
		return nil, storage.ErrInvalidProofEnvelope
	}

	status, _, _, _, _, err := storage.GetMarket(ctx, mu, a.MarketID)
	if err != nil {
		return nil, err
	}
	if status != storage.MarketStatusActive {
		return nil, storage.ErrMarketNotActive
	}

	cfg, err := storage.GetProofConfig(ctx, mu)
	if err != nil {
		return nil, err
	}
	if !cfg.RequireProof {
		return nil, storage.ErrInvalidProofConfig
	}
	if actor != cfg.ProverAuthority {
		return nil, storage.ErrUnauthorized
	}
	if a.ProofType != cfg.RequiredProofType {
		return nil, storage.ErrProofTypeMismatch
	}
	if a.WindowCloseAtMs%cfg.BatchWindowMs != 0 {
		return nil, storage.ErrInvalidProofEnvelope
	}
	if timestamp < a.WindowCloseAtMs {
		missedDeadline = true
		return nil, storage.ErrProofDeadlineMissed
	}
	if timestamp > a.WindowCloseAtMs+cfg.ProofDeadlineMs {
		missedDeadline = true
		return nil, storage.ErrProofDeadlineMissed
	}

	_, err = storage.GetBatchProofRecord(ctx, mu, a.MarketID, a.WindowID)
	if err == nil {
		return nil, storage.ErrProofAlreadySubmitted
	}
	if !errors.Is(err, storage.ErrProofNotFound) {
		return nil, err
	}

	if err := verifyBatchProofInConsensus(cfg.RequiredProofType, a.Proof, a.PublicInputsHash); err != nil {
		return nil, err
	}

	// canonical public-input binding against clear inputs is enforced at clear time.
	commitment := sha256.Sum256(a.Proof)
	record := storage.BatchProofRecord{
		ProofType:        a.ProofType,
		SubmittedAtMs:    timestamp,
		WindowCloseAtMs:  a.WindowCloseAtMs,
		Prover:           actor,
		ProofCommitment:  commitment,
		PublicInputsHash: append([]byte(nil), a.PublicInputsHash...),
		FillsHash:        append([]byte(nil), a.FillsHash...),
	}
	if err := storage.PutBatchProofRecord(ctx, mu, a.MarketID, a.WindowID, record); err != nil {
		return nil, err
	}
	if err := storage.PutVellumProof(ctx, mu, a.MarketID, a.WindowID, a.Proof); err != nil {
		return nil, err
	}
	bloodsworn, err := storage.GetBloodsworn(ctx, mu, actor)
	if err != nil {
		return nil, err
	}
	bloodsworn.TotalAcceptedProofs++
	bloodsworn.ActiveStreak++
	bloodsworn.LastProofAtMs = timestamp
	if err := storage.PutBloodsworn(ctx, mu, actor, bloodsworn); err != nil {
		return nil, err
	}
	glyph := deriveGlyph(
		txID,
		a.MarketID,
		a.WindowID,
		actor,
		commitment,
		a.PublicInputsHash,
		timestamp,
	)
	if err := storage.PutGlyph(ctx, mu, a.MarketID, a.WindowID, glyph); err != nil {
		return nil, err
	}

	result := &SubmitBatchProofResult{
		SubmittedAtMs:    record.SubmittedAtMs,
		ProofCommitment:  record.ProofCommitment[:],
		StoredProofBytes: uint32(len(a.Proof)),
		GlyphClass:       glyph.Class,
		GlyphRarity:      glyph.Rarity,
	}
	return result.Bytes(), nil
}

func (*SubmitBatchProof) ComputeUnits(chain.Rules) uint64 {
	return SubmitBatchProofComputeUnits
}

func (*SubmitBatchProof) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

var _ codec.Typed = (*SubmitBatchProofResult)(nil)

type SubmitBatchProofResult struct {
	SubmittedAtMs    int64  `serialize:"true" json:"submitted_at_ms"`
	ProofCommitment  []byte `serialize:"true" json:"proof_commitment"`
	StoredProofBytes uint32 `serialize:"true" json:"stored_proof_bytes"`
	GlyphClass       uint8  `serialize:"true" json:"glyph_class"`
	GlyphRarity      uint8  `serialize:"true" json:"glyph_rarity"`
}

func (*SubmitBatchProofResult) GetTypeID() uint8 {
	return mconsts.SubmitBatchProofID
}

func (r *SubmitBatchProofResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, 256),
		MaxSize: MaxSubmitBatchProofSize,
	}
	p.PackByte(mconsts.SubmitBatchProofID)
	_ = codec.LinearCodec.MarshalInto(r, p)
	return p.Bytes
}

func UnmarshalSubmitBatchProofResult(b []byte) (codec.Typed, error) {
	r := &SubmitBatchProofResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		r,
	); err != nil {
		return nil, err
	}
	return r, nil
}
