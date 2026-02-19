package actions

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
	"github.com/ava-labs/hypersdk/state"

	smath "github.com/ava-labs/avalanchego/utils/math"
)

const (
	MintVAIComputeUnits = 2
	MaxMintVAISize      = 512
)

var (
	ErrUnmarshalEmptyMintVAI              = errors.New("cannot unmarshal empty bytes as mint_vai")
	_                        chain.Action = (*MintVAI)(nil)
)

type MintVAI struct {
	To     codec.Address `serialize:"true" json:"to"`
	Amount uint64        `serialize:"true" json:"amount"`
}

func (*MintVAI) GetTypeID() uint8 {
	return mconsts.MintVAIID
}

func (t *MintVAI) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.VAIConfigKey()):      state.Read,
		string(storage.VAIStateKey()):       state.Read | state.Write,
		string(storage.RiskConfigKey()):     state.Read,
		string(storage.ReserveStateKey()):   state.Read,
		string(storage.VAIBalanceKey(t.To)): state.Read | state.Write,
	}
}

func (t *MintVAI) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxMintVAISize),
		MaxSize: MaxMintVAISize,
	}
	p.PackByte(mconsts.MintVAIID)
	if err := codec.LinearCodec.MarshalInto(t, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalMintVAI(bytes []byte) (chain.Action, error) {
	t := &MintVAI{}
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptyMintVAI
	}
	if bytes[0] != mconsts.MintVAIID {
		return nil, fmt.Errorf("unexpected mint_vai typeID: %d != %d", bytes[0], mconsts.MintVAIID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *MintVAI) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	timestamp int64,
	actor codec.Address,
	_ ids.ID,
) ([]byte, error) {
	if t.Amount == 0 {
		return nil, storage.ErrInvalidVAIAmount
	}
	cfg, err := storage.GetVAIConfig(ctx, mu)
	if err != nil {
		return nil, err
	}
	if actor != cfg.MintAuthority {
		return nil, storage.ErrUnauthorized
	}
	stateVal, err := storage.GetVAIState(ctx, mu)
	if err != nil {
		return nil, err
	}
	if stateVal.EpochStartUnix == 0 {
		stateVal.EpochStartUnix = timestamp
	}
	if timestamp >= stateVal.EpochStartUnix+(cfg.MintEpochSeconds*1_000) {
		stateVal.EpochStartUnix = timestamp
		stateVal.EpochMinted = 0
	}
	nextEpochMinted, err := smath.Add(stateVal.EpochMinted, t.Amount)
	if err != nil {
		return nil, err
	}
	if nextEpochMinted > cfg.EpochMintLimit {
		return nil, storage.ErrVAIEpochMintLimitExceeded
	}
	nextDebt, err := smath.Add(stateVal.TotalDebt, t.Amount)
	if err != nil {
		return nil, err
	}
	if nextDebt > cfg.DebtCeiling {
		return nil, storage.ErrVAIDebtCeilingExceeded
	}
	riskCfg, err := storage.GetRiskConfig(ctx, mu)
	if err != nil {
		return nil, err
	}
	reserveState, err := storage.GetReserveState(ctx, mu)
	if err != nil {
		return nil, err
	}
	if nextDebt > 0 {
		left := new(big.Int).Mul(
			new(big.Int).SetUint64(reserveState.ExogenousReserve),
			new(big.Int).SetUint64(10_000),
		)
		right := new(big.Int).Mul(
			new(big.Int).SetUint64(nextDebt),
			new(big.Int).SetUint64(uint64(riskCfg.BackingFloorBips)),
		)
		if left.Cmp(right) < 0 {
			return nil, storage.ErrBackingRatioViolation
		}
	}
	toBalance, err := storage.AddVAIBalance(ctx, mu, t.To, t.Amount)
	if err != nil {
		return nil, err
	}
	stateVal.EpochMinted = nextEpochMinted
	stateVal.TotalDebt = nextDebt
	if err := storage.PutVAIState(ctx, mu, stateVal); err != nil {
		return nil, err
	}
	result := &MintVAIResult{
		ToBalance: toBalance,
		TotalDebt: stateVal.TotalDebt,
	}
	return result.Bytes(), nil
}

func (*MintVAI) ComputeUnits(chain.Rules) uint64 {
	return MintVAIComputeUnits
}

func (*MintVAI) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

var _ codec.Typed = (*MintVAIResult)(nil)

type MintVAIResult struct {
	ToBalance uint64 `serialize:"true" json:"to_balance"`
	TotalDebt uint64 `serialize:"true" json:"total_debt"`
}

func (*MintVAIResult) GetTypeID() uint8 {
	return mconsts.MintVAIID
}

func (t *MintVAIResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxMintVAISize),
		MaxSize: MaxMintVAISize,
	}
	p.PackByte(mconsts.MintVAIID)
	_ = codec.LinearCodec.MarshalInto(t, p)
	return p.Bytes
}

func UnmarshalMintVAIResult(b []byte) (codec.Typed, error) {
	t := &MintVAIResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}
