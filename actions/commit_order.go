package actions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"

	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
)

const (
	CommitOrderComputeUnits = 2
	MaxEnvelopeSize         = 4096
	MaxCommitmentSize       = 64
	MaxCommitOrderSize      = 8192
)

var (
	ErrEnvelopeEmpty                          = errors.New("envelope is empty")
	ErrEnvelopeTooLarge                       = errors.New("envelope is too large")
	ErrCommitmentEmpty                        = errors.New("commitment is empty")
	ErrCommitmentTooLarge                     = errors.New("commitment is too large")
	ErrUnmarshalEmptyCommitOrder              = errors.New("cannot unmarshal empty bytes as commit_order")
	_                            chain.Action = (*CommitOrder)(nil)
)

type CommitOrder struct {
	MarketID   ids.ID `serialize:"true" json:"market_id"`
	WindowID   uint64 `serialize:"true" json:"window_id"`
	Envelope   []byte `serialize:"true" json:"envelope"`
	Commitment []byte `serialize:"true" json:"commitment"`
}

func (*CommitOrder) GetTypeID() uint8 {
	return mconsts.CommitOrderID
}

func (t *CommitOrder) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.MarketKey(t.MarketID)):                        state.Read,
		string(storage.CommitmentKey(t.MarketID, t.WindowID, actor)): state.All,
	}
}

func (t *CommitOrder) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxCommitOrderSize),
		MaxSize: MaxCommitOrderSize,
	}
	p.PackByte(mconsts.CommitOrderID)
	if err := codec.LinearCodec.MarshalInto(t, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalCommitOrder(bytes []byte) (chain.Action, error) {
	t := &CommitOrder{}
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptyCommitOrder
	}
	if bytes[0] != mconsts.CommitOrderID {
		return nil, fmt.Errorf("unexpected commit_order typeID: %d != %d", bytes[0], mconsts.CommitOrderID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *CommitOrder) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	timestamp int64,
	actor codec.Address,
	_ ids.ID,
) (_ []byte, err error) {
	start := time.Now()
	defer func() {
		RecordCommitMetric(t.MarketID, t.WindowID, timestamp, time.Since(start), err)
	}()

	if len(t.Envelope) == 0 {
		return nil, ErrEnvelopeEmpty
	}
	if len(t.Envelope) > MaxEnvelopeSize {
		return nil, ErrEnvelopeTooLarge
	}
	if len(t.Commitment) == 0 {
		return nil, ErrCommitmentEmpty
	}
	if len(t.Commitment) > MaxCommitmentSize {
		return nil, ErrCommitmentTooLarge
	}

	// Verify market exists and is active
	status, _, _, _, _, err := storage.GetMarket(ctx, mu, t.MarketID)
	if err != nil {
		return nil, err
	}
	if status != storage.MarketStatusActive {
		return nil, storage.ErrMarketNotActive
	}

	// TODO(M2): lock collateral at commit time to make commitments economically binding.
	// Store commitment
	if err := storage.PutCommitment(ctx, mu, t.MarketID, t.WindowID, actor, t.Envelope, t.Commitment); err != nil {
		return nil, err
	}

	result := &CommitOrderResult{WindowID: t.WindowID}
	return result.Bytes(), nil
}

func (*CommitOrder) ComputeUnits(chain.Rules) uint64 {
	return CommitOrderComputeUnits
}

func (*CommitOrder) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

var _ codec.Typed = (*CommitOrderResult)(nil)

type CommitOrderResult struct {
	WindowID uint64 `serialize:"true" json:"window_id"`
}

func (*CommitOrderResult) GetTypeID() uint8 {
	return mconsts.CommitOrderID
}

func (t *CommitOrderResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, 256),
		MaxSize: MaxCommitOrderSize,
	}
	p.PackByte(mconsts.CommitOrderID)
	_ = codec.LinearCodec.MarshalInto(t, p)
	return p.Bytes
}

func UnmarshalCommitOrderResult(b []byte) (codec.Typed, error) {
	t := &CommitOrderResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}
