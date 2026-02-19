package actions

import (
	"context"
	"encoding/binary"
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
	RevealBatchComputeUnits = 3
	MaxDecryptionShareSize  = 512
	MaxRevealBatchSize      = 2048
)

var (
	ErrDecryptionShareEmpty                   = errors.New("decryption share is empty")
	ErrDecryptionShareTooLarge                = errors.New("decryption share is too large")
	ErrUnmarshalEmptyRevealBatch              = errors.New("cannot unmarshal empty bytes as reveal_batch")
	_                            chain.Action = (*RevealBatch)(nil)
)

type RevealBatch struct {
	MarketID        ids.ID `serialize:"true" json:"market_id"`
	WindowID        uint64 `serialize:"true" json:"window_id"`
	DecryptionShare []byte `serialize:"true" json:"decryption_share"`
	ValidatorIndex  uint32 `serialize:"true" json:"validator_index"`
}

func (*RevealBatch) GetTypeID() uint8 {
	return mconsts.RevealBatchID
}

func (t *RevealBatch) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.MarketKey(t.MarketID)):                   state.Read,
		string(storage.OracleKey(t.MarketID, t.ValidatorIndex)): state.All,
	}
}

func (t *RevealBatch) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxRevealBatchSize),
		MaxSize: MaxRevealBatchSize,
	}
	p.PackByte(mconsts.RevealBatchID)
	if err := codec.LinearCodec.MarshalInto(t, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalRevealBatch(bytes []byte) (chain.Action, error) {
	t := &RevealBatch{}
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptyRevealBatch
	}
	if bytes[0] != mconsts.RevealBatchID {
		return nil, fmt.Errorf("unexpected reveal_batch typeID: %d != %d", bytes[0], mconsts.RevealBatchID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *RevealBatch) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	_ codec.Address,
	_ ids.ID,
) (_ []byte, err error) {
	start := time.Now()
	defer func() {
		RecordRevealMetric(t.MarketID, t.WindowID, time.Since(start), err)
	}()

	if len(t.DecryptionShare) == 0 {
		return nil, ErrDecryptionShareEmpty
	}
	if len(t.DecryptionShare) > MaxDecryptionShareSize {
		return nil, ErrDecryptionShareTooLarge
	}

	// Verify market exists and is active
	status, _, _, _, _, err := storage.GetMarket(ctx, mu, t.MarketID)
	if err != nil {
		return nil, err
	}
	if status != storage.MarketStatusActive {
		return nil, storage.ErrMarketNotActive
	}

	// TODO(M2): require oracle/committee authorization for reveal submissions.
	// Store decryption share keyed by validator index
	k := storage.OracleKey(t.MarketID, t.ValidatorIndex)
	v := make([]byte, 0, 8+len(t.DecryptionShare))
	v = binary.BigEndian.AppendUint64(v, t.WindowID)
	v = append(v, t.DecryptionShare...)
	if err := mu.Insert(ctx, k, v); err != nil {
		return nil, err
	}

	result := &RevealBatchResult{ValidatorIndex: t.ValidatorIndex}
	return result.Bytes(), nil
}

func (*RevealBatch) ComputeUnits(chain.Rules) uint64 {
	return RevealBatchComputeUnits
}

func (*RevealBatch) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

var _ codec.Typed = (*RevealBatchResult)(nil)

type RevealBatchResult struct {
	ValidatorIndex uint32 `serialize:"true" json:"validator_index"`
}

func (*RevealBatchResult) GetTypeID() uint8 {
	return mconsts.RevealBatchID
}

func (t *RevealBatchResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, 256),
		MaxSize: MaxRevealBatchSize,
	}
	p.PackByte(mconsts.RevealBatchID)
	_ = codec.LinearCodec.MarshalInto(t, p)
	return p.Bytes
}

func UnmarshalRevealBatchResult(b []byte) (codec.Typed, error) {
	t := &RevealBatchResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}
