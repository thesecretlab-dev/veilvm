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
	BurnVAIComputeUnits = 2
	MaxBurnVAISize      = 256
)

var (
	ErrUnmarshalEmptyBurnVAI              = errors.New("cannot unmarshal empty bytes as burn_vai")
	_                        chain.Action = (*BurnVAI)(nil)
)

type BurnVAI struct {
	Amount uint64 `serialize:"true" json:"amount"`
}

func (*BurnVAI) GetTypeID() uint8 {
	return mconsts.BurnVAIID
}

func (*BurnVAI) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.VAIBalanceKey(actor)): state.Read | state.Write,
		string(storage.VAIStateKey()):        state.Read | state.Write,
	}
}

func (t *BurnVAI) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxBurnVAISize),
		MaxSize: MaxBurnVAISize,
	}
	p.PackByte(mconsts.BurnVAIID)
	if err := codec.LinearCodec.MarshalInto(t, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalBurnVAI(bytes []byte) (chain.Action, error) {
	t := &BurnVAI{}
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptyBurnVAI
	}
	if bytes[0] != mconsts.BurnVAIID {
		return nil, fmt.Errorf("unexpected burn_vai typeID: %d != %d", bytes[0], mconsts.BurnVAIID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *BurnVAI) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) ([]byte, error) {
	if t.Amount == 0 {
		return nil, storage.ErrInvalidVAIAmount
	}
	actorBalance, err := storage.SubVAIBalance(ctx, mu, actor, t.Amount)
	if err != nil {
		return nil, err
	}
	stateVal, err := storage.GetVAIState(ctx, mu)
	if err != nil {
		return nil, err
	}
	if t.Amount > stateVal.TotalDebt {
		return nil, storage.ErrVAIDebtInvariant
	}
	stateVal.TotalDebt -= t.Amount
	if err := storage.PutVAIState(ctx, mu, stateVal); err != nil {
		return nil, err
	}
	result := &BurnVAIResult{
		ActorBalance: actorBalance,
		TotalDebt:    stateVal.TotalDebt,
	}
	return result.Bytes(), nil
}

func (*BurnVAI) ComputeUnits(chain.Rules) uint64 {
	return BurnVAIComputeUnits
}

func (*BurnVAI) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

var _ codec.Typed = (*BurnVAIResult)(nil)

type BurnVAIResult struct {
	ActorBalance uint64 `serialize:"true" json:"actor_balance"`
	TotalDebt    uint64 `serialize:"true" json:"total_debt"`
}

func (*BurnVAIResult) GetTypeID() uint8 {
	return mconsts.BurnVAIID
}

func (t *BurnVAIResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxBurnVAISize),
		MaxSize: MaxBurnVAISize,
	}
	p.PackByte(mconsts.BurnVAIID)
	_ = codec.LinearCodec.MarshalInto(t, p)
	return p.Bytes
}

func UnmarshalBurnVAIResult(b []byte) (codec.Typed, error) {
	t := &BurnVAIResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}
