package actions

import (
	"context"
	"errors"
	"math/big"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

const (
	AssetVEIL uint8 = 0
	AssetVAI  uint8 = 1

	MinPoolFeeBips uint16 = 1
	MaxPoolFeeBips uint16 = 1_000
)

func isSupportedAsset(asset uint8) bool {
	return asset == AssetVEIL || asset == AssetVAI
}

func validateAssetPair(asset0 uint8, asset1 uint8) error {
	if !isSupportedAsset(asset0) || !isSupportedAsset(asset1) {
		return storage.ErrUnsupportedAsset
	}
	if asset0 == asset1 {
		return storage.ErrInvalidAssetPair
	}
	return nil
}

func sortedPair(asset0 uint8, asset1 uint8) (uint8, uint8) {
	if asset0 <= asset1 {
		return asset0, asset1
	}
	return asset1, asset0
}

func mapPairAmounts(assetA uint8, assetB uint8, amountA uint64, amountB uint64, pool storage.Pool) (uint64, uint64, error) {
	if assetA == pool.Asset0 && assetB == pool.Asset1 {
		return amountA, amountB, nil
	}
	if assetA == pool.Asset1 && assetB == pool.Asset0 {
		return amountB, amountA, nil
	}
	return 0, 0, storage.ErrInvalidAssetPair
}

func mapPoolAmountsToPair(pool storage.Pool, assetA uint8, assetB uint8, amount0 uint64, amount1 uint64) (uint64, uint64, error) {
	if assetA == pool.Asset0 && assetB == pool.Asset1 {
		return amount0, amount1, nil
	}
	if assetA == pool.Asset1 && assetB == pool.Asset0 {
		return amount1, amount0, nil
	}
	return 0, 0, storage.ErrInvalidAssetPair
}

func subAssetBalance(
	ctx context.Context,
	mu state.Mutable,
	actor codec.Address,
	asset uint8,
	amount uint64,
) (uint64, error) {
	switch asset {
	case AssetVEIL:
		return storage.SubBalance(ctx, mu, actor, amount)
	case AssetVAI:
		return storage.SubVAIBalance(ctx, mu, actor, amount)
	default:
		return 0, storage.ErrUnsupportedAsset
	}
}

func addAssetBalance(
	ctx context.Context,
	mu state.Mutable,
	actor codec.Address,
	asset uint8,
	amount uint64,
) (uint64, error) {
	switch asset {
	case AssetVEIL:
		return storage.AddBalance(ctx, mu, actor, amount)
	case AssetVAI:
		return storage.AddVAIBalance(ctx, mu, actor, amount)
	default:
		return 0, storage.ErrUnsupportedAsset
	}
}

func mulDiv(a uint64, b uint64, den uint64) (uint64, error) {
	if den == 0 {
		return 0, errors.New("division by zero")
	}
	prod := new(big.Int).Mul(new(big.Int).SetUint64(a), new(big.Int).SetUint64(b))
	q := new(big.Int).Div(prod, new(big.Int).SetUint64(den))
	if !q.IsUint64() {
		return 0, errors.New("overflow")
	}
	return q.Uint64(), nil
}

func minU64(a uint64, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func intSqrt(n uint64) uint64 {
	if n == 0 {
		return 0
	}
	x := n
	y := (x + 1) / 2
	for y < x {
		x = y
		y = (x + n/x) / 2
	}
	return x
}
