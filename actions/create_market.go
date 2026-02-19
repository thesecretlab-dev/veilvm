package actions

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
	"github.com/ava-labs/hypersdk/state"

	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
)

const (
	CreateMarketComputeUnits = 5
	MaxCreateMarketSize      = 4096
	MaxQuestionSize          = 1024
)

var (
	ErrInvalidOutcomes                         = errors.New("outcomes must be >= 2")
	ErrInvalidResolutionTime                   = errors.New("invalid resolution time")
	ErrCreatorBondZero                         = errors.New("creator bond is zero")
	ErrQuestionTooLarge                        = errors.New("question too large")
	ErrUnmarshalEmptyCreateMarket              = errors.New("cannot unmarshal empty bytes as create_market")
	_                             chain.Action = (*CreateMarket)(nil)
)

type CreateMarket struct {
	MarketID       ids.ID `serialize:"true" json:"market_id"`
	Question       []byte `serialize:"true" json:"question"`
	Outcomes       uint8  `serialize:"true" json:"outcomes"`
	ResolutionTime int64  `serialize:"true" json:"resolution_time"`
	CreatorBond    uint64 `serialize:"true" json:"creator_bond"`
}

func (*CreateMarket) GetTypeID() uint8 {
	return mconsts.CreateMarketID
}

func (a *CreateMarket) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.BalanceKey(actor)):     state.Read | state.Write,
		string(storage.MarketKey(a.MarketID)): state.All,
	}
}

func (a *CreateMarket) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxCreateMarketSize),
		MaxSize: MaxCreateMarketSize,
	}
	p.PackByte(mconsts.CreateMarketID)
	if err := codec.LinearCodec.MarshalInto(a, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalCreateMarket(bytes []byte) (chain.Action, error) {
	a := &CreateMarket{}
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptyCreateMarket
	}
	if bytes[0] != mconsts.CreateMarketID {
		return nil, fmt.Errorf("unexpected create_market typeID: %d != %d", bytes[0], mconsts.CreateMarketID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		a,
	); err != nil {
		return nil, err
	}
	if len(a.Question) > MaxQuestionSize {
		return nil, ErrQuestionTooLarge
	}
	return a, nil
}

func (a *CreateMarket) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) ([]byte, error) {
	if a.Outcomes < 2 {
		return nil, ErrInvalidOutcomes
	}
	if a.ResolutionTime <= 0 {
		return nil, ErrInvalidResolutionTime
	}
	if a.CreatorBond == 0 {
		return nil, ErrCreatorBondZero
	}
	if len(a.Question) > MaxQuestionSize {
		return nil, ErrQuestionTooLarge
	}

	// Reject duplicate market IDs to avoid silent overwrites.
	_, _, _, _, _, err := storage.GetMarket(ctx, mu, a.MarketID)
	if err == nil {
		return nil, storage.ErrMarketExists
	}
	if !errors.Is(err, storage.ErrMarketNotFound) {
		return nil, err
	}

	// Deduct creator bond
	senderBalance, err := storage.SubBalance(ctx, mu, actor, a.CreatorBond)
	if err != nil {
		return nil, err
	}

	// Store market
	if err := storage.PutMarket(ctx, mu, a.MarketID, storage.MarketStatusActive, a.Outcomes, a.ResolutionTime, 0, a.Question); err != nil {
		return nil, err
	}

	result := &CreateMarketResult{SenderBalance: senderBalance}
	return result.Bytes(), nil
}

func (*CreateMarket) ComputeUnits(chain.Rules) uint64 {
	return CreateMarketComputeUnits
}

func (*CreateMarket) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

var _ codec.Typed = (*CreateMarketResult)(nil)

type CreateMarketResult struct {
	SenderBalance uint64 `serialize:"true" json:"sender_balance"`
}

func (*CreateMarketResult) GetTypeID() uint8 {
	return mconsts.CreateMarketID
}

func (r *CreateMarketResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, 256),
		MaxSize: MaxCreateMarketSize,
	}
	p.PackByte(mconsts.CreateMarketID)
	_ = codec.LinearCodec.MarshalInto(r, p)
	return p.Bytes
}

func UnmarshalCreateMarketResult(b []byte) (codec.Typed, error) {
	r := &CreateMarketResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		r,
	); err != nil {
		return nil, err
	}
	return r, nil
}
