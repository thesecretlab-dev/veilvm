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
	SwapExactInComputeUnits = 3
	MaxSwapExactInSize      = 256
)

type SwapExactIn struct {
	AssetIn      uint8  `serialize:"true" json:"asset_in"`
	AssetOut     uint8  `serialize:"true" json:"asset_out"`
	AmountIn     uint64 `serialize:"true" json:"amount_in"`
	MinAmountOut uint64 `serialize:"true" json:"min_amount_out"`
}

func (*SwapExactIn) GetTypeID() uint8 {
	return mconsts.SwapExactInID
}

func (a *SwapExactIn) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.BalanceKey(actor)):              state.Read | state.Write,
		string(storage.VAIBalanceKey(actor)):           state.Read | state.Write,
		string(storage.PoolKey(a.AssetIn, a.AssetOut)): state.Read | state.Write,
	}
}

func (a *SwapExactIn) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxSwapExactInSize),
		MaxSize: MaxSwapExactInSize,
	}
	p.PackByte(mconsts.SwapExactInID)
	if err := codec.LinearCodec.MarshalInto(a, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalSwapExactIn(bytes []byte) (chain.Action, error) {
	a := &SwapExactIn{}
	if len(bytes) == 0 {
		return nil, fmt.Errorf("cannot unmarshal empty bytes as swap_exact_in")
	}
	if bytes[0] != mconsts.SwapExactInID {
		return nil, fmt.Errorf("unexpected swap_exact_in typeID: %d != %d", bytes[0], mconsts.SwapExactInID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		a,
	); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *SwapExactIn) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) ([]byte, error) {
	if err := validateAssetPair(a.AssetIn, a.AssetOut); err != nil {
		return nil, err
	}
	if a.AmountIn == 0 {
		return nil, storage.ErrInvalidSwapAmount
	}
	pool, err := storage.GetPool(ctx, mu, a.AssetIn, a.AssetOut)
	if err != nil {
		return nil, err
	}

	var reserveIn, reserveOut uint64
	inIs0 := false
	switch {
	case pool.Asset0 == a.AssetIn && pool.Asset1 == a.AssetOut:
		inIs0 = true
		reserveIn = pool.Reserve0
		reserveOut = pool.Reserve1
	case pool.Asset0 == a.AssetOut && pool.Asset1 == a.AssetIn:
		inIs0 = false
		reserveIn = pool.Reserve1
		reserveOut = pool.Reserve0
	default:
		return nil, storage.ErrInvalidAssetPair
	}
	if reserveIn == 0 || reserveOut == 0 {
		return nil, storage.ErrInsufficientLiquidity
	}

	if _, err := subAssetBalance(ctx, mu, actor, a.AssetIn, a.AmountIn); err != nil {
		return nil, err
	}

	amountInWithFee, err := mulDiv(a.AmountIn, uint64(10_000-pool.FeeBips), 10_000)
	if err != nil {
		return nil, err
	}
	if amountInWithFee == 0 {
		return nil, storage.ErrInvalidSwapAmount
	}
	den, err := smath.Add(reserveIn, amountInWithFee)
	if err != nil {
		return nil, err
	}
	amountOut, err := mulDiv(reserveOut, amountInWithFee, den)
	if err != nil {
		return nil, err
	}
	if amountOut == 0 || amountOut < a.MinAmountOut {
		return nil, storage.ErrSlippageExceeded
	}
	if amountOut > reserveOut {
		return nil, storage.ErrInsufficientLiquidity
	}

	if inIs0 {
		pool.Reserve0, err = smath.Add(pool.Reserve0, a.AmountIn)
		if err != nil {
			return nil, err
		}
		pool.Reserve1, err = smath.Sub(pool.Reserve1, amountOut)
		if err != nil {
			return nil, err
		}
	} else {
		pool.Reserve1, err = smath.Add(pool.Reserve1, a.AmountIn)
		if err != nil {
			return nil, err
		}
		pool.Reserve0, err = smath.Sub(pool.Reserve0, amountOut)
		if err != nil {
			return nil, err
		}
	}
	if err := storage.PutPool(ctx, mu, pool); err != nil {
		return nil, err
	}
	receiverBalance, err := addAssetBalance(ctx, mu, actor, a.AssetOut, amountOut)
	if err != nil {
		return nil, err
	}

	result := &SwapExactInResult{
		AmountOut:       amountOut,
		ReceiverBalance: receiverBalance,
		Reserve0:        pool.Reserve0,
		Reserve1:        pool.Reserve1,
	}
	return result.Bytes(), nil
}

func (*SwapExactIn) ComputeUnits(chain.Rules) uint64 {
	return SwapExactInComputeUnits
}

func (*SwapExactIn) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

type SwapExactInResult struct {
	AmountOut       uint64 `serialize:"true" json:"amount_out"`
	ReceiverBalance uint64 `serialize:"true" json:"receiver_balance"`
	Reserve0        uint64 `serialize:"true" json:"reserve0"`
	Reserve1        uint64 `serialize:"true" json:"reserve1"`
}

func (*SwapExactInResult) GetTypeID() uint8 {
	return mconsts.SwapExactInID
}

func (r *SwapExactInResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxSwapExactInSize),
		MaxSize: MaxSwapExactInSize,
	}
	p.PackByte(mconsts.SwapExactInID)
	_ = codec.LinearCodec.MarshalInto(r, p)
	return p.Bytes
}

func UnmarshalSwapExactInResult(b []byte) (codec.Typed, error) {
	r := &SwapExactInResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		r,
	); err != nil {
		return nil, err
	}
	return r, nil
}
