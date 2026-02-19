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
	UpdateReserveStateComputeUnits = 2
	MaxUpdateReserveStateSize      = 256
)

var (
	ErrUnmarshalEmptyUpdateReserveState              = errors.New("cannot unmarshal empty bytes as update_reserve_state")
	_                                   chain.Action = (*UpdateReserveState)(nil)
)

type UpdateReserveState struct {
	ExogenousReserve uint64 `serialize:"true" json:"exogenous_reserve"`
	VAIBuffer        uint64 `serialize:"true" json:"vai_buffer"`
}

func (*UpdateReserveState) GetTypeID() uint8 {
	return mconsts.UpdateReserveStateID
}

func (*UpdateReserveState) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.TreasuryConfigKey()): state.Read,
		string(storage.ReserveStateKey()):   state.Read | state.Write,
	}
}

func (t *UpdateReserveState) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxUpdateReserveStateSize),
		MaxSize: MaxUpdateReserveStateSize,
	}
	p.PackByte(mconsts.UpdateReserveStateID)
	if err := codec.LinearCodec.MarshalInto(t, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalUpdateReserveState(bytes []byte) (chain.Action, error) {
	t := &UpdateReserveState{}
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptyUpdateReserveState
	}
	if bytes[0] != mconsts.UpdateReserveStateID {
		return nil, fmt.Errorf("unexpected update_reserve_state typeID: %d != %d", bytes[0], mconsts.UpdateReserveStateID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *UpdateReserveState) Execute(
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

	next := storage.ReserveState{
		ExogenousReserve: t.ExogenousReserve,
		VAIBuffer:        t.VAIBuffer,
	}
	if err := storage.PutReserveState(ctx, mu, next); err != nil {
		return nil, err
	}

	result := &UpdateReserveStateResult{
		ExogenousReserve: next.ExogenousReserve,
		VAIBuffer:        next.VAIBuffer,
	}
	return result.Bytes(), nil
}

func (*UpdateReserveState) ComputeUnits(chain.Rules) uint64 {
	return UpdateReserveStateComputeUnits
}

func (*UpdateReserveState) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

var _ codec.Typed = (*UpdateReserveStateResult)(nil)

type UpdateReserveStateResult struct {
	ExogenousReserve uint64 `serialize:"true" json:"exogenous_reserve"`
	VAIBuffer        uint64 `serialize:"true" json:"vai_buffer"`
}

func (*UpdateReserveStateResult) GetTypeID() uint8 {
	return mconsts.UpdateReserveStateID
}

func (t *UpdateReserveStateResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxUpdateReserveStateSize),
		MaxSize: MaxUpdateReserveStateSize,
	}
	p.PackByte(mconsts.UpdateReserveStateID)
	_ = codec.LinearCodec.MarshalInto(t, p)
	return p.Bytes
}

func UnmarshalUpdateReserveStateResult(b []byte) (codec.Typed, error) {
	t := &UpdateReserveStateResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}
