package actions

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"

	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
)

const (
	ResolveMarketComputeUnits = 5
	MaxSignatureSize          = 256
	MaxResolveMarketSize      = 2048
)

var (
	ErrSignatureEmpty                           = errors.New("signature is empty")
	ErrSignatureTooLarge                        = errors.New("signature is too large")
	ErrUnmarshalEmptyResolveMarket              = errors.New("cannot unmarshal empty bytes as resolve_market")
	_                              chain.Action = (*ResolveMarket)(nil)
)

type ResolveMarket struct {
	MarketID  ids.ID `serialize:"true" json:"market_id"`
	Outcome   uint8  `serialize:"true" json:"outcome"`
	Signature []byte `serialize:"true" json:"signature"`
}

func (*ResolveMarket) GetTypeID() uint8 {
	return mconsts.ResolveMarketID
}

func (t *ResolveMarket) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.MarketKey(t.MarketID)): state.Read | state.Write,
	}
}

func (t *ResolveMarket) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxResolveMarketSize),
		MaxSize: MaxResolveMarketSize,
	}
	p.PackByte(mconsts.ResolveMarketID)
	if err := codec.LinearCodec.MarshalInto(t, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalResolveMarket(bytes []byte) (chain.Action, error) {
	t := &ResolveMarket{}
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptyResolveMarket
	}
	if bytes[0] != mconsts.ResolveMarketID {
		return nil, fmt.Errorf("unexpected resolve_market typeID: %d != %d", bytes[0], mconsts.ResolveMarketID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *ResolveMarket) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	_ codec.Address,
	_ ids.ID,
) ([]byte, error) {
	if len(t.Signature) == 0 {
		return nil, ErrSignatureEmpty
	}
	if len(t.Signature) > MaxSignatureSize {
		return nil, ErrSignatureTooLarge
	}

	// Get current market state
	status, outcomes, resolutionTime, _, question, err := storage.GetMarket(ctx, mu, t.MarketID)
	if err != nil {
		return nil, err
	}
	if status != storage.MarketStatusActive {
		return nil, storage.ErrMarketNotActive
	}
	if t.Outcome >= outcomes {
		return nil, storage.ErrInvalidOutcome
	}

	// TODO: Verify BLS aggregate signature from oracle committee
	// For now, accept any signature (will be enforced in M2+)

	// Update market to resolved
	if err := storage.PutMarket(ctx, mu, t.MarketID, storage.MarketStatusResolved, outcomes, resolutionTime, t.Outcome, question); err != nil {
		return nil, err
	}

	result := &ResolveMarketResult{
		Outcome: t.Outcome,
	}
	return result.Bytes(), nil
}

func (*ResolveMarket) ComputeUnits(chain.Rules) uint64 {
	return ResolveMarketComputeUnits
}

func (*ResolveMarket) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

var _ codec.Typed = (*ResolveMarketResult)(nil)

type ResolveMarketResult struct {
	Outcome uint8 `serialize:"true" json:"outcome"`
}

func (*ResolveMarketResult) GetTypeID() uint8 {
	return mconsts.ResolveMarketID
}

func (t *ResolveMarketResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, 256),
		MaxSize: MaxResolveMarketSize,
	}
	p.PackByte(mconsts.ResolveMarketID)
	_ = codec.LinearCodec.MarshalInto(t, p)
	return p.Bytes
}

func UnmarshalResolveMarketResult(b []byte) (codec.Typed, error) {
	t := &ResolveMarketResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}
