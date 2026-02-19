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
	AddLiquidityComputeUnits = 4
	MaxAddLiquiditySize      = 512
)

type AddLiquidity struct {
	Asset0  uint8  `serialize:"true" json:"asset0"`
	Asset1  uint8  `serialize:"true" json:"asset1"`
	Amount0 uint64 `serialize:"true" json:"amount0"`
	Amount1 uint64 `serialize:"true" json:"amount1"`
	MinLP   uint64 `serialize:"true" json:"min_lp"`
}

func (*AddLiquidity) GetTypeID() uint8 {
	return mconsts.AddLiquidityID
}

func (a *AddLiquidity) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.BalanceKey(actor)):                       state.Read | state.Write,
		string(storage.VAIBalanceKey(actor)):                    state.Read | state.Write,
		string(storage.PoolKey(a.Asset0, a.Asset1)):             state.Read | state.Write,
		string(storage.LPBalanceKey(a.Asset0, a.Asset1, actor)): state.Read | state.Write,
	}
}

func (a *AddLiquidity) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxAddLiquiditySize),
		MaxSize: MaxAddLiquiditySize,
	}
	p.PackByte(mconsts.AddLiquidityID)
	if err := codec.LinearCodec.MarshalInto(a, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalAddLiquidity(bytes []byte) (chain.Action, error) {
	a := &AddLiquidity{}
	if len(bytes) == 0 {
		return nil, fmt.Errorf("cannot unmarshal empty bytes as add_liquidity")
	}
	if bytes[0] != mconsts.AddLiquidityID {
		return nil, fmt.Errorf("unexpected add_liquidity typeID: %d != %d", bytes[0], mconsts.AddLiquidityID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		a,
	); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *AddLiquidity) Execute(
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
	if a.Amount0 == 0 || a.Amount1 == 0 {
		return nil, storage.ErrInvalidLiquidityAmount
	}

	pool, err := storage.GetPool(ctx, mu, a.Asset0, a.Asset1)
	if err != nil {
		return nil, err
	}
	amount0Pool, amount1Pool, err := mapPairAmounts(a.Asset0, a.Asset1, a.Amount0, a.Amount1, pool)
	if err != nil {
		return nil, err
	}

	if _, err := subAssetBalance(ctx, mu, actor, a.Asset0, a.Amount0); err != nil {
		return nil, err
	}
	if _, err := subAssetBalance(ctx, mu, actor, a.Asset1, a.Amount1); err != nil {
		return nil, err
	}

	var minted uint64
	if pool.TotalLP == 0 {
		product, err := smath.Mul(amount0Pool, amount1Pool)
		if err != nil {
			return nil, err
		}
		minted = intSqrt(product)
	} else {
		if pool.Reserve0 == 0 || pool.Reserve1 == 0 {
			return nil, storage.ErrInsufficientLiquidity
		}
		lp0, err := mulDiv(amount0Pool, pool.TotalLP, pool.Reserve0)
		if err != nil {
			return nil, err
		}
		lp1, err := mulDiv(amount1Pool, pool.TotalLP, pool.Reserve1)
		if err != nil {
			return nil, err
		}
		minted = minU64(lp0, lp1)
	}
	if minted == 0 || minted < a.MinLP {
		return nil, storage.ErrSlippageExceeded
	}

	nextReserve0, err := smath.Add(pool.Reserve0, amount0Pool)
	if err != nil {
		return nil, err
	}
	nextReserve1, err := smath.Add(pool.Reserve1, amount1Pool)
	if err != nil {
		return nil, err
	}
	nextTotalLP, err := smath.Add(pool.TotalLP, minted)
	if err != nil {
		return nil, err
	}
	pool.Reserve0 = nextReserve0
	pool.Reserve1 = nextReserve1
	pool.TotalLP = nextTotalLP
	if err := storage.PutPool(ctx, mu, pool); err != nil {
		return nil, err
	}
	lpBalance, err := storage.AddLPBalance(ctx, mu, pool.Asset0, pool.Asset1, actor, minted)
	if err != nil {
		return nil, err
	}

	result := &AddLiquidityResult{
		MintedLP:  minted,
		LPBalance: lpBalance,
		Reserve0:  pool.Reserve0,
		Reserve1:  pool.Reserve1,
		TotalLP:   pool.TotalLP,
	}
	return result.Bytes(), nil
}

func (*AddLiquidity) ComputeUnits(chain.Rules) uint64 {
	return AddLiquidityComputeUnits
}

func (*AddLiquidity) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

type AddLiquidityResult struct {
	MintedLP  uint64 `serialize:"true" json:"minted_lp"`
	LPBalance uint64 `serialize:"true" json:"lp_balance"`
	Reserve0  uint64 `serialize:"true" json:"reserve0"`
	Reserve1  uint64 `serialize:"true" json:"reserve1"`
	TotalLP   uint64 `serialize:"true" json:"total_lp"`
}

func (*AddLiquidityResult) GetTypeID() uint8 {
	return mconsts.AddLiquidityID
}

func (r *AddLiquidityResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxAddLiquiditySize),
		MaxSize: MaxAddLiquiditySize,
	}
	p.PackByte(mconsts.AddLiquidityID)
	_ = codec.LinearCodec.MarshalInto(r, p)
	return p.Bytes
}

func UnmarshalAddLiquidityResult(b []byte) (codec.Typed, error) {
	r := &AddLiquidityResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		r,
	); err != nil {
		return nil, err
	}
	return r, nil
}
