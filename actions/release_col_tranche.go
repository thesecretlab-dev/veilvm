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

	smath "github.com/ava-labs/avalanchego/utils/math"
)

const (
	ReleaseCOLTrancheComputeUnits = 2
	MaxReleaseCOLTrancheSize      = 256
)

var (
	ErrUnmarshalEmptyReleaseCOLTranche              = errors.New("cannot unmarshal empty bytes as release_col_tranche")
	_                                  chain.Action = (*ReleaseCOLTranche)(nil)
)

type ReleaseCOLTranche struct {
	Amount uint64 `serialize:"true" json:"amount"`
}

func (*ReleaseCOLTranche) GetTypeID() uint8 {
	return mconsts.ReleaseCOLTrancheID
}

func (*ReleaseCOLTranche) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.TreasuryConfigKey()): state.Read,
		string(storage.TreasuryStateKey()):  state.Read | state.Write,
	}
}

func (t *ReleaseCOLTranche) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxReleaseCOLTrancheSize),
		MaxSize: MaxReleaseCOLTrancheSize,
	}
	p.PackByte(mconsts.ReleaseCOLTrancheID)
	if err := codec.LinearCodec.MarshalInto(t, p); err != nil {
		panic(err)
	}
	return p.Bytes
}

func UnmarshalReleaseCOLTranche(bytes []byte) (chain.Action, error) {
	t := &ReleaseCOLTranche{}
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptyReleaseCOLTranche
	}
	if bytes[0] != mconsts.ReleaseCOLTrancheID {
		return nil, fmt.Errorf("unexpected release_col_tranche typeID: %d != %d", bytes[0], mconsts.ReleaseCOLTrancheID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *ReleaseCOLTranche) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	timestamp int64,
	actor codec.Address,
	_ ids.ID,
) ([]byte, error) {
	if t.Amount == 0 {
		return nil, storage.ErrInvalidReleaseAmount
	}

	cfg, err := storage.GetTreasuryConfig(ctx, mu)
	if err != nil {
		return nil, err
	}
	if actor != cfg.Governance {
		return nil, storage.ErrUnauthorized
	}

	stateVal, err := storage.GetTreasuryState(ctx, mu)
	if err != nil {
		return nil, err
	}
	nextAllowed := stateVal.LastReleaseUnix + (cfg.ReleaseEpochSeconds * 1_000)
	if stateVal.LastReleaseUnix > 0 && timestamp < nextAllowed {
		return nil, storage.ErrReleaseTooEarly
	}
	if t.Amount > stateVal.Locked {
		return nil, storage.ErrInsufficientLockedCOL
	}
	maxRelease := stateVal.Locked * uint64(cfg.MaxReleaseBips) / 10_000
	if maxRelease == 0 || t.Amount > maxRelease {
		return nil, storage.ErrReleaseCapExceeded
	}
	stateVal.Locked -= t.Amount
	nextLive, err := smath.Add(stateVal.Live, t.Amount)
	if err != nil {
		return nil, err
	}
	nextReleased, err := smath.Add(stateVal.Released, t.Amount)
	if err != nil {
		return nil, err
	}
	stateVal.Live = nextLive
	stateVal.Released = nextReleased
	stateVal.LastReleaseUnix = timestamp
	if err := storage.PutTreasuryState(ctx, mu, stateVal); err != nil {
		return nil, err
	}

	result := &ReleaseCOLTrancheResult{
		Locked:   stateVal.Locked,
		Live:     stateVal.Live,
		Released: stateVal.Released,
	}
	return result.Bytes(), nil
}

func (*ReleaseCOLTranche) ComputeUnits(chain.Rules) uint64 {
	return ReleaseCOLTrancheComputeUnits
}

func (*ReleaseCOLTranche) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

var _ codec.Typed = (*ReleaseCOLTrancheResult)(nil)

type ReleaseCOLTrancheResult struct {
	Locked   uint64 `serialize:"true" json:"locked"`
	Live     uint64 `serialize:"true" json:"live"`
	Released uint64 `serialize:"true" json:"released"`
}

func (*ReleaseCOLTrancheResult) GetTypeID() uint8 {
	return mconsts.ReleaseCOLTrancheID
}

func (t *ReleaseCOLTrancheResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxReleaseCOLTrancheSize),
		MaxSize: MaxReleaseCOLTrancheSize,
	}
	p.PackByte(mconsts.ReleaseCOLTrancheID)
	_ = codec.LinearCodec.MarshalInto(t, p)
	return p.Bytes
}

func UnmarshalReleaseCOLTrancheResult(b []byte) (codec.Typed, error) {
	t := &ReleaseCOLTrancheResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}
