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

	smath "github.com/ava-labs/avalanchego/utils/math"
)

const (
	RouteFeesComputeUnits = 2
	MaxRouteFeesSize      = 256
)

var (
	ErrUnmarshalEmptyRouteFees              = errors.New("cannot unmarshal empty bytes as route_fees")
	_                          chain.Action = (*RouteFees)(nil)
)

type RouteFees struct {
	Amount uint64 `serialize:"true" json:"amount"`
}

func (*RouteFees) GetTypeID() uint8 {
	return mconsts.RouteFeesID
}

func (*RouteFees) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.BalanceKey(actor)):    state.Read | state.Write,
		string(storage.FeeRouterConfigKey()): state.Read,
		string(storage.FeeRouterStateKey()):  state.Read | state.Write,
	}
}

func (t *RouteFees) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxRouteFeesSize),
		MaxSize: MaxRouteFeesSize,
	}
	p.PackByte(mconsts.RouteFeesID)
	if err := codec.LinearCodec.MarshalInto(t, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalRouteFees(bytes []byte) (chain.Action, error) {
	t := &RouteFees{}
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptyRouteFees
	}
	if bytes[0] != mconsts.RouteFeesID {
		return nil, fmt.Errorf("unexpected route_fees typeID: %d != %d", bytes[0], mconsts.RouteFeesID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *RouteFees) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) ([]byte, error) {
	if t.Amount == 0 {
		return nil, storage.ErrInvalidFeeAmount
	}

	cfg, err := storage.GetFeeRouterConfig(ctx, mu)
	if err != nil {
		return nil, err
	}
	stateVal, err := storage.GetFeeRouterState(ctx, mu)
	if err != nil {
		return nil, err
	}

	senderBalance, err := storage.SubBalance(ctx, mu, actor, t.Amount)
	if err != nil {
		return nil, err
	}

	msrbShare := uint64(cfg.MSRBBips) * t.Amount / 10_000
	colShare := uint64(cfg.COLBips) * t.Amount / 10_000
	opsShare := t.Amount - msrbShare - colShare

	nextMSRB, err := smath.Add(stateVal.MSRBBudget, msrbShare)
	if err != nil {
		return nil, err
	}
	nextCOL, err := smath.Add(stateVal.COLBudget, colShare)
	if err != nil {
		return nil, err
	}
	nextOps, err := smath.Add(stateVal.OpsBudget, opsShare)
	if err != nil {
		return nil, err
	}
	stateVal.MSRBBudget = nextMSRB
	stateVal.COLBudget = nextCOL
	stateVal.OpsBudget = nextOps
	if err := storage.PutFeeRouterState(ctx, mu, stateVal); err != nil {
		return nil, err
	}

	result := &RouteFeesResult{
		SenderBalance: senderBalance,
		MSRBBudget:    stateVal.MSRBBudget,
		COLBudget:     stateVal.COLBudget,
		OpsBudget:     stateVal.OpsBudget,
	}
	return result.Bytes(), nil
}

func (*RouteFees) ComputeUnits(chain.Rules) uint64 {
	return RouteFeesComputeUnits
}

func (*RouteFees) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

var _ codec.Typed = (*RouteFeesResult)(nil)

type RouteFeesResult struct {
	SenderBalance uint64 `serialize:"true" json:"sender_balance"`
	MSRBBudget    uint64 `serialize:"true" json:"msrb_budget"`
	COLBudget     uint64 `serialize:"true" json:"col_budget"`
	OpsBudget     uint64 `serialize:"true" json:"ops_budget"`
}

func (*RouteFeesResult) GetTypeID() uint8 {
	return mconsts.RouteFeesID
}

func (t *RouteFeesResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxRouteFeesSize),
		MaxSize: MaxRouteFeesSize,
	}
	p.PackByte(mconsts.RouteFeesID)
	_ = codec.LinearCodec.MarshalInto(t, p)
	return p.Bytes
}

func UnmarshalRouteFeesResult(b []byte) (codec.Typed, error) {
	t := &RouteFeesResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}
