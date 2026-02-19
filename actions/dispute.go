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
	DisputeComputeUnits = 3
	MaxEvidenceSize     = 4096
	MaxDisputeSize      = 8192
)

var (
	ErrBondZero                           = errors.New("bond is zero")
	ErrEvidenceEmpty                      = errors.New("evidence is empty")
	ErrEvidenceTooLarge                   = errors.New("evidence is too large")
	ErrUnmarshalEmptyDispute              = errors.New("cannot unmarshal empty bytes as dispute")
	_                        chain.Action = (*Dispute)(nil)
)

type Dispute struct {
	MarketID ids.ID `serialize:"true" json:"market_id"`
	Bond     uint64 `serialize:"true" json:"bond"`
	Evidence []byte `serialize:"true" json:"evidence"`
}

func (*Dispute) GetTypeID() uint8 {
	return mconsts.DisputeID
}

func (t *Dispute) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.BalanceKey(actor)):      state.Read | state.Write,
		string(storage.MarketKey(t.MarketID)):  state.Read | state.Write,
		string(storage.DisputeKey(t.MarketID)): state.All,
	}
}

func (t *Dispute) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxDisputeSize),
		MaxSize: MaxDisputeSize,
	}
	p.PackByte(mconsts.DisputeID)
	if err := codec.LinearCodec.MarshalInto(t, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalDispute(bytes []byte) (chain.Action, error) {
	t := &Dispute{}
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptyDispute
	}
	if bytes[0] != mconsts.DisputeID {
		return nil, fmt.Errorf("unexpected dispute typeID: %d != %d", bytes[0], mconsts.DisputeID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Dispute) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) ([]byte, error) {
	if t.Bond == 0 {
		return nil, ErrBondZero
	}
	if len(t.Evidence) == 0 {
		return nil, ErrEvidenceEmpty
	}
	if len(t.Evidence) > MaxEvidenceSize {
		return nil, ErrEvidenceTooLarge
	}

	// Verify market exists and is resolved (can be disputed)
	status, outcomes, resolutionTime, resolvedOutcome, question, err := storage.GetMarket(ctx, mu, t.MarketID)
	if err != nil {
		return nil, err
	}
	if status != storage.MarketStatusResolved {
		return nil, storage.ErrMarketNotResolved
	}

	// Deduct bond from disputer
	newBalance, err := storage.SubBalance(ctx, mu, actor, t.Bond)
	if err != nil {
		return nil, err
	}

	// Update market to disputed status
	if err := storage.PutMarket(ctx, mu, t.MarketID, storage.MarketStatusDisputed, outcomes, resolutionTime, resolvedOutcome, question); err != nil {
		return nil, err
	}

	// Store dispute
	if err := storage.PutDispute(ctx, mu, t.MarketID, t.Bond, t.Evidence); err != nil {
		return nil, err
	}

	result := &DisputeResult{
		DisputerBalance: newBalance,
	}
	return result.Bytes(), nil
}

func (*Dispute) ComputeUnits(chain.Rules) uint64 {
	return DisputeComputeUnits
}

func (*Dispute) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

var _ codec.Typed = (*DisputeResult)(nil)

type DisputeResult struct {
	DisputerBalance uint64 `serialize:"true" json:"disputer_balance"`
}

func (*DisputeResult) GetTypeID() uint8 {
	return mconsts.DisputeID
}

func (t *DisputeResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, 256),
		MaxSize: MaxDisputeSize,
	}
	p.PackByte(mconsts.DisputeID)
	_ = codec.LinearCodec.MarshalInto(t, p)
	return p.Bytes
}

func UnmarshalDisputeResult(b []byte) (codec.Typed, error) {
	t := &DisputeResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}
