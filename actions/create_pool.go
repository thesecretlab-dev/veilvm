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
	CreatePoolComputeUnits = 3
	MaxCreatePoolSize      = 256
)

var (
	ErrUnmarshalEmptyCreatePool              = errors.New("cannot unmarshal empty bytes as create_pool")
	_                           chain.Action = (*CreatePool)(nil)
)

type CreatePool struct {
	Asset0  uint8  `serialize:"true" json:"asset0"`
	Asset1  uint8  `serialize:"true" json:"asset1"`
	FeeBips uint16 `serialize:"true" json:"fee_bips"`
}

func (*CreatePool) GetTypeID() uint8 {
	return mconsts.CreatePoolID
}

func (a *CreatePool) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.TreasuryConfigKey()):         state.Read,
		string(storage.PoolKey(a.Asset0, a.Asset1)): state.All,
	}
}

func (a *CreatePool) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxCreatePoolSize),
		MaxSize: MaxCreatePoolSize,
	}
	p.PackByte(mconsts.CreatePoolID)
	if err := codec.LinearCodec.MarshalInto(a, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalCreatePool(bytes []byte) (chain.Action, error) {
	a := &CreatePool{}
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptyCreatePool
	}
	if bytes[0] != mconsts.CreatePoolID {
		return nil, fmt.Errorf("unexpected create_pool typeID: %d != %d", bytes[0], mconsts.CreatePoolID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		a,
	); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *CreatePool) Execute(
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
	if a.FeeBips < MinPoolFeeBips || a.FeeBips > MaxPoolFeeBips {
		return nil, storage.ErrInvalidPoolFee
	}
	cfg, err := storage.GetTreasuryConfig(ctx, mu)
	if err != nil {
		return nil, err
	}
	if actor != cfg.Governance {
		return nil, storage.ErrUnauthorized
	}
	_, err = storage.GetPool(ctx, mu, a.Asset0, a.Asset1)
	if err == nil {
		return nil, storage.ErrPoolExists
	}
	if !errors.Is(err, storage.ErrPoolNotFound) {
		return nil, err
	}
	asset0, asset1 := sortedPair(a.Asset0, a.Asset1)
	pool := storage.Pool{
		Asset0:   asset0,
		Asset1:   asset1,
		FeeBips:  a.FeeBips,
		Reserve0: 0,
		Reserve1: 0,
		TotalLP:  0,
	}
	if err := storage.PutPool(ctx, mu, pool); err != nil {
		return nil, err
	}
	result := &CreatePoolResult{
		Asset0:  pool.Asset0,
		Asset1:  pool.Asset1,
		FeeBips: pool.FeeBips,
	}
	return result.Bytes(), nil
}

func (*CreatePool) ComputeUnits(chain.Rules) uint64 {
	return CreatePoolComputeUnits
}

func (*CreatePool) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

var _ codec.Typed = (*CreatePoolResult)(nil)

type CreatePoolResult struct {
	Asset0  uint8  `serialize:"true" json:"asset0"`
	Asset1  uint8  `serialize:"true" json:"asset1"`
	FeeBips uint16 `serialize:"true" json:"fee_bips"`
}

func (*CreatePoolResult) GetTypeID() uint8 {
	return mconsts.CreatePoolID
}

func (r *CreatePoolResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxCreatePoolSize),
		MaxSize: MaxCreatePoolSize,
	}
	p.PackByte(mconsts.CreatePoolID)
	_ = codec.LinearCodec.MarshalInto(r, p)
	return p.Bytes
}

func UnmarshalCreatePoolResult(b []byte) (codec.Typed, error) {
	r := &CreatePoolResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		r,
	); err != nil {
		return nil, err
	}
	return r, nil
}
