package actions

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	smath "github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

const (
	RemoveLiquidityComputeUnits = 4
	MaxRemoveLiquiditySize      = 512
)

type RemoveLiquidity struct {
	Asset0     uint8  `serialize:"true" json:"asset0"`
	Asset1     uint8  `serialize:"true" json:"asset1"`
	LPAmount   uint64 `serialize:"true" json:"lp_amount"`
	MinAmount0 uint64 `serialize:"true" json:"min_amount0"`
	MinAmount1 uint64 `serialize:"true" json:"min_amount1"`
}

func (*RemoveLiquidity) GetTypeID() uint8 {
	return mconsts.RemoveLiquidityID
}

func (a *RemoveLiquidity) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.BalanceKey(actor)):                       state.Read | state.Write,
		string(storage.VAIBalanceKey(actor)):                    state.Read | state.Write,
		string(storage.PoolKey(a.Asset0, a.Asset1)):             state.Read | state.Write,
		string(storage.LPBalanceKey(a.Asset0, a.Asset1, actor)): state.Read | state.Write,
	}
}

func (a *RemoveLiquidity) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxRemoveLiquiditySize),
		MaxSize: MaxRemoveLiquiditySize,
	}
	p.PackByte(mconsts.RemoveLiquidityID)
	if err := codec.LinearCodec.MarshalInto(a, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalRemoveLiquidity(bytes []byte) (chain.Action, error) {
	a := &RemoveLiquidity{}
	if len(bytes) == 0 {
		return nil, fmt.Errorf("cannot unmarshal empty bytes as remove_liquidity")
	}
	if bytes[0] != mconsts.RemoveLiquidityID {
		return nil, fmt.Errorf("unexpected remove_liquidity typeID: %d != %d", bytes[0], mconsts.RemoveLiquidityID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		a,
	); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *RemoveLiquidity) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) ([]byte, error) {
	if err := validateAssetPair(a.Asset0, a.Asset1); err != nil {
		return nil, err
	}
	if a.LPAmount == 0 {
		return nil, storage.ErrInvalidLiquidityAmount
	}

	pool, err := storage.GetPool(ctx, mu, a.Asset0, a.Asset1)
	if err != nil {
		return nil, err
	}
	if pool.TotalLP == 0 || pool.Reserve0 == 0 || pool.Reserve1 == 0 {
		return nil, storage.ErrInsufficientLiquidity
	}
	lpBalance, err := storage.GetLPBalance(ctx, mu, pool.Asset0, pool.Asset1, actor)
	if err != nil {
		return nil, err
	}
	if lpBalance < a.LPAmount {
		return nil, storage.ErrInsufficientLPBalance
	}

	out0Pool, err := mulDiv(a.LPAmount, pool.Reserve0, pool.TotalLP)
	if err != nil {
		return nil, err
	}
	out1Pool, err := mulDiv(a.LPAmount, pool.Reserve1, pool.TotalLP)
	if err != nil {
		return nil, err
	}
	if out0Pool == 0 || out1Pool == 0 {
		return nil, storage.ErrInsufficientLiquidity
	}
	min0Pool, min1Pool, err := mapPairAmounts(a.Asset0, a.Asset1, a.MinAmount0, a.MinAmount1, pool)
	if err != nil {
		return nil, err
	}
	if out0Pool < min0Pool || out1Pool < min1Pool {
		return nil, storage.ErrSlippageExceeded
	}

	nextReserve0, err := smath.Sub(pool.Reserve0, out0Pool)
	if err != nil {
		return nil, err
	}
	nextReserve1, err := smath.Sub(pool.Reserve1, out1Pool)
	if err != nil {
		return nil, err
	}
	nextTotalLP, err := smath.Sub(pool.TotalLP, a.LPAmount)
	if err != nil {
		return nil, err
	}
	pool.Reserve0 = nextReserve0
	pool.Reserve1 = nextReserve1
	pool.TotalLP = nextTotalLP
	if err := storage.PutPool(ctx, mu, pool); err != nil {
		return nil, err
	}
	nextLPBalance, err := storage.SubLPBalance(ctx, mu, pool.Asset0, pool.Asset1, actor, a.LPAmount)
	if err != nil {
		return nil, err
	}
	if _, err := addAssetBalance(ctx, mu, actor, pool.Asset0, out0Pool); err != nil {
		return nil, err
	}
	if _, err := addAssetBalance(ctx, mu, actor, pool.Asset1, out1Pool); err != nil {
		return nil, err
	}
	out0, out1, err := mapPoolAmountsToPair(pool, a.Asset0, a.Asset1, out0Pool, out1Pool)
	if err != nil {
		return nil, err
	}

	result := &RemoveLiquidityResult{
		Amount0Out: out0,
		Amount1Out: out1,
		LPBalance:  nextLPBalance,
		Reserve0:   pool.Reserve0,
		Reserve1:   pool.Reserve1,
		TotalLP:    pool.TotalLP,
	}
	return result.Bytes(), nil
}

func (*RemoveLiquidity) ComputeUnits(chain.Rules) uint64 {
	return RemoveLiquidityComputeUnits
}

func (*RemoveLiquidity) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

type RemoveLiquidityResult struct {
	Amount0Out uint64 `serialize:"true" json:"amount0_out"`
	Amount1Out uint64 `serialize:"true" json:"amount1_out"`
	LPBalance  uint64 `serialize:"true" json:"lp_balance"`
	Reserve0   uint64 `serialize:"true" json:"reserve0"`
	Reserve1   uint64 `serialize:"true" json:"reserve1"`
	TotalLP    uint64 `serialize:"true" json:"total_lp"`
}

func (*RemoveLiquidityResult) GetTypeID() uint8 {
	return mconsts.RemoveLiquidityID
}

func (r *RemoveLiquidityResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxRemoveLiquiditySize),
		MaxSize: MaxRemoveLiquiditySize,
	}
	p.PackByte(mconsts.RemoveLiquidityID)
	_ = codec.LinearCodec.MarshalInto(r, p)
	return p.Bytes
}

func UnmarshalRemoveLiquidityResult(b []byte) (codec.Typed, error) {
	r := &RemoveLiquidityResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		r,
	); err != nil {
		return nil, err
	}
	return r, nil
}
