package actions

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

const (
	SetProofConfigComputeUnits = 2
	MaxSetProofConfigSize      = 256
)

var (
	ErrUnmarshalEmptySetProofConfig              = errors.New("cannot unmarshal empty bytes as set_proof_config")
	_                               chain.Action = (*SetProofConfig)(nil)
)

type SetProofConfig struct {
	RequireProof      bool          `serialize:"true" json:"require_proof"`
	RequiredProofType uint8         `serialize:"true" json:"required_proof_type"`
	BatchWindowMs     int64         `serialize:"true" json:"batch_window_ms"`
	ProofDeadlineMs   int64         `serialize:"true" json:"proof_deadline_ms"`
	ProverAuthority   codec.Address `serialize:"true" json:"prover_authority"`
}

func (*SetProofConfig) GetTypeID() uint8 {
	return mconsts.SetProofConfigID
}

func (*SetProofConfig) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.TreasuryConfigKey()): state.Read,
		string(storage.ProofConfigKey()):    state.Read | state.Write,
	}
}

func (t *SetProofConfig) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxSetProofConfigSize),
		MaxSize: MaxSetProofConfigSize,
	}
	p.PackByte(mconsts.SetProofConfigID)
	if err := codec.LinearCodec.MarshalInto(t, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalSetProofConfig(bytes []byte) (chain.Action, error) {
	t := &SetProofConfig{}
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptySetProofConfig
	}
	if bytes[0] != mconsts.SetProofConfigID {
		return nil, fmt.Errorf("unexpected set_proof_config typeID: %d != %d", bytes[0], mconsts.SetProofConfigID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *SetProofConfig) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) ([]byte, error) {
	treasuryCfg, err := storage.GetTreasuryConfig(ctx, mu)
	if err != nil {
		return nil, err
	}
	if actor != treasuryCfg.Governance {
		return nil, storage.ErrUnauthorized
	}

	cfg := storage.ProofConfig{
		RequireProof:      t.RequireProof,
		RequiredProofType: t.RequiredProofType,
		BatchWindowMs:     t.BatchWindowMs,
		ProofDeadlineMs:   t.ProofDeadlineMs,
		ProverAuthority:   t.ProverAuthority,
	}
	if err := storage.PutProofConfig(ctx, mu, cfg); err != nil {
		return nil, err
	}

	result := &SetProofConfigResult{
		RequireProof:      cfg.RequireProof,
		RequiredProofType: cfg.RequiredProofType,
		BatchWindowMs:     cfg.BatchWindowMs,
		ProofDeadlineMs:   cfg.ProofDeadlineMs,
		ProverAuthority:   cfg.ProverAuthority,
	}
	return result.Bytes(), nil
}

func (*SetProofConfig) ComputeUnits(chain.Rules) uint64 {
	return SetProofConfigComputeUnits
}

func (*SetProofConfig) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

var _ codec.Typed = (*SetProofConfigResult)(nil)

type SetProofConfigResult struct {
	RequireProof      bool          `serialize:"true" json:"require_proof"`
	RequiredProofType uint8         `serialize:"true" json:"required_proof_type"`
	BatchWindowMs     int64         `serialize:"true" json:"batch_window_ms"`
	ProofDeadlineMs   int64         `serialize:"true" json:"proof_deadline_ms"`
	ProverAuthority   codec.Address `serialize:"true" json:"prover_authority"`
}

func (*SetProofConfigResult) GetTypeID() uint8 {
	return mconsts.SetProofConfigID
}

func (t *SetProofConfigResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxSetProofConfigSize),
		MaxSize: MaxSetProofConfigSize,
	}
	p.PackByte(mconsts.SetProofConfigID)
	_ = codec.LinearCodec.MarshalInto(t, p)
	return p.Bytes
}

func UnmarshalSetProofConfigResult(b []byte) (codec.Typed, error) {
	t := &SetProofConfigResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}
