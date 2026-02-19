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
	SetRiskParamsComputeUnits = 2
	MaxSetRiskParamsSize      = 256
)

var (
	ErrUnmarshalEmptySetRiskParams              = errors.New("cannot unmarshal empty bytes as set_risk_params")
	_                              chain.Action = (*SetRiskParams)(nil)
)

type SetRiskParams struct {
	BackingFloorBips uint32 `serialize:"true" json:"backing_floor_bips"`

	VEILLtvBips   uint16 `serialize:"true" json:"veil_ltv_bips"`
	WVEILLtvBips  uint16 `serialize:"true" json:"wveil_ltv_bips"`
	WSVEILLtvBips uint16 `serialize:"true" json:"wsveil_ltv_bips"`

	VEILHaircutBips   uint16 `serialize:"true" json:"veil_haircut_bips"`
	WVEILHaircutBips  uint16 `serialize:"true" json:"wveil_haircut_bips"`
	WSVEILHaircutBips uint16 `serialize:"true" json:"wsveil_haircut_bips"`
}

func (*SetRiskParams) GetTypeID() uint8 {
	return mconsts.SetRiskParamsID
}

func (*SetRiskParams) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.TreasuryConfigKey()): state.Read,
		string(storage.RiskConfigKey()):     state.Read | state.Write,
	}
}

func (t *SetRiskParams) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxSetRiskParamsSize),
		MaxSize: MaxSetRiskParamsSize,
	}
	p.PackByte(mconsts.SetRiskParamsID)
	if err := codec.LinearCodec.MarshalInto(t, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalSetRiskParams(bytes []byte) (chain.Action, error) {
	t := &SetRiskParams{}
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptySetRiskParams
	}
	if bytes[0] != mconsts.SetRiskParamsID {
		return nil, fmt.Errorf("unexpected set_risk_params typeID: %d != %d", bytes[0], mconsts.SetRiskParamsID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *SetRiskParams) Execute(
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

	cfg := storage.RiskConfig{
		BackingFloorBips:  t.BackingFloorBips,
		VEILLtvBips:       t.VEILLtvBips,
		WVEILLtvBips:      t.WVEILLtvBips,
		WSVEILLtvBips:     t.WSVEILLtvBips,
		VEILHaircutBips:   t.VEILHaircutBips,
		WVEILHaircutBips:  t.WVEILHaircutBips,
		WSVEILHaircutBips: t.WSVEILHaircutBips,
	}
	if err := storage.PutRiskConfig(ctx, mu, cfg); err != nil {
		return nil, err
	}

	result := &SetRiskParamsResult{
		BackingFloorBips:  cfg.BackingFloorBips,
		VEILLtvBips:       cfg.VEILLtvBips,
		WVEILLtvBips:      cfg.WVEILLtvBips,
		WSVEILLtvBips:     cfg.WSVEILLtvBips,
		VEILHaircutBips:   cfg.VEILHaircutBips,
		WVEILHaircutBips:  cfg.WVEILHaircutBips,
		WSVEILHaircutBips: cfg.WSVEILHaircutBips,
	}
	return result.Bytes(), nil
}

func (*SetRiskParams) ComputeUnits(chain.Rules) uint64 {
	return SetRiskParamsComputeUnits
}

func (*SetRiskParams) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

var _ codec.Typed = (*SetRiskParamsResult)(nil)

type SetRiskParamsResult struct {
	BackingFloorBips uint32 `serialize:"true" json:"backing_floor_bips"`

	VEILLtvBips   uint16 `serialize:"true" json:"veil_ltv_bips"`
	WVEILLtvBips  uint16 `serialize:"true" json:"wveil_ltv_bips"`
	WSVEILLtvBips uint16 `serialize:"true" json:"wsveil_ltv_bips"`

	VEILHaircutBips   uint16 `serialize:"true" json:"veil_haircut_bips"`
	WVEILHaircutBips  uint16 `serialize:"true" json:"wveil_haircut_bips"`
	WSVEILHaircutBips uint16 `serialize:"true" json:"wsveil_haircut_bips"`
}

func (*SetRiskParamsResult) GetTypeID() uint8 {
	return mconsts.SetRiskParamsID
}

func (t *SetRiskParamsResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxSetRiskParamsSize),
		MaxSize: MaxSetRiskParamsSize,
	}
	p.PackByte(mconsts.SetRiskParamsID)
	_ = codec.LinearCodec.MarshalInto(t, p)
	return p.Bytes
}

func UnmarshalSetRiskParamsResult(b []byte) (codec.Typed, error) {
	t := &SetRiskParamsResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}
