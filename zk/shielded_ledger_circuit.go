package zk

import (
	"crypto/sha256"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/examples/veilvm/actions"
	"github.com/consensys/gnark/frontend"
	gnarksha2 "github.com/consensys/gnark/std/hash/sha2"
	"github.com/consensys/gnark/std/math/uints"
)

const (
	ShieldedLedgerDigestLen    = sha256.Size
	ShieldedLedgerDomainTagLen = len(actions.ShieldedLedgerInputsDomainTag)

	ShieldedLedgerWindowIDLen   = 8
	ShieldedLedgerClearPriceLen = 8
	ShieldedLedgerTotalVolLen   = 8
	ShieldedLedgerHashLenLen    = 2

	ShieldedLedgerMarketIDOffset     = ShieldedLedgerDomainTagLen
	ShieldedLedgerWindowIDOffset     = ShieldedLedgerMarketIDOffset + ids.IDLen
	ShieldedLedgerClearPriceOffset   = ShieldedLedgerWindowIDOffset + ShieldedLedgerWindowIDLen
	ShieldedLedgerTotalVolOffset     = ShieldedLedgerClearPriceOffset + ShieldedLedgerClearPriceLen
	ShieldedLedgerFillsHashLenOffset = ShieldedLedgerTotalVolOffset + ShieldedLedgerTotalVolLen
	ShieldedLedgerFillsHashOffset    = ShieldedLedgerFillsHashLenOffset + ShieldedLedgerHashLenLen

	ShieldedLedgerPreimageLen = ShieldedLedgerFillsHashOffset + actions.ExpectedFillsHashSize
)

// ShieldedLedgerCircuitV1 binds a domain-separated shielded-ledger preimage to
// a public digest.
//
// This v1 path still uses digest binding, but it now enforces a canonical
// shielded-ledger preimage shape (domain tag, fixed fills-hash length, and
// non-zero critical fields) inside the circuit.
type ShieldedLedgerCircuitV1 struct {
	Preimage [ShieldedLedgerPreimageLen]uints.U8
	Digest   [ShieldedLedgerDigestLen]uints.U8 `gnark:",public"`
}

func (c *ShieldedLedgerCircuitV1) Define(api frontend.API) error {
	h, err := gnarksha2.New(api)
	if err != nil {
		return err
	}
	uapi, err := uints.New[uints.U32](api)
	if err != nil {
		return err
	}

	if err := assertLiteralBytes(
		uapi,
		c.Preimage[:ShieldedLedgerDomainTagLen],
		[]byte(actions.ShieldedLedgerInputsDomainTag),
	); err != nil {
		return err
	}
	if err := assertLiteralBytes(
		uapi,
		c.Preimage[ShieldedLedgerFillsHashLenOffset:ShieldedLedgerFillsHashOffset],
		[]byte{0x00, byte(actions.ExpectedFillsHashSize)},
	); err != nil {
		return err
	}
	assertByteSliceNonZero(
		api,
		c.Preimage[ShieldedLedgerMarketIDOffset:ShieldedLedgerWindowIDOffset],
	)
	assertByteSliceNonZero(
		api,
		c.Preimage[ShieldedLedgerWindowIDOffset:ShieldedLedgerClearPriceOffset],
	)
	assertByteSliceNonZero(
		api,
		c.Preimage[ShieldedLedgerClearPriceOffset:ShieldedLedgerTotalVolOffset],
	)
	assertByteSliceNonZero(
		api,
		c.Preimage[ShieldedLedgerTotalVolOffset:ShieldedLedgerFillsHashLenOffset],
	)
	assertByteSliceNonZero(
		api,
		c.Preimage[ShieldedLedgerFillsHashOffset:ShieldedLedgerPreimageLen],
	)

	h.Write(c.Preimage[:])
	sum := h.Sum()
	if len(sum) != ShieldedLedgerDigestLen {
		return fmt.Errorf("unexpected digest size: %d", len(sum))
	}
	for i := 0; i < ShieldedLedgerDigestLen; i++ {
		uapi.ByteAssertEq(c.Digest[i], sum[i])
	}
	return nil
}

func NewShieldedLedgerAssignment(preimage []byte, digest []byte) (*ShieldedLedgerCircuitV1, error) {
	if len(preimage) != ShieldedLedgerPreimageLen {
		return nil, fmt.Errorf(
			"invalid shielded-ledger preimage size: got=%d expected=%d",
			len(preimage),
			ShieldedLedgerPreimageLen,
		)
	}
	if len(digest) != ShieldedLedgerDigestLen {
		return nil, fmt.Errorf(
			"invalid shielded-ledger digest size: got=%d expected=%d",
			len(digest),
			ShieldedLedgerDigestLen,
		)
	}

	out := &ShieldedLedgerCircuitV1{}
	pre := uints.NewU8Array(preimage)
	pub := uints.NewU8Array(digest)
	for i := 0; i < ShieldedLedgerPreimageLen; i++ {
		out.Preimage[i] = pre[i]
	}
	for i := 0; i < ShieldedLedgerDigestLen; i++ {
		out.Digest[i] = pub[i]
	}
	return out, nil
}

func assertLiteralBytes(uapi *uints.BinaryField[uints.U32], got []uints.U8, want []byte) error {
	if len(got) != len(want) {
		return fmt.Errorf("literal length mismatch: got=%d want=%d", len(got), len(want))
	}
	for i := range want {
		uapi.ByteAssertEq(got[i], uints.NewU8(want[i]))
	}
	return nil
}

func assertByteSliceNonZero(api frontend.API, b []uints.U8) {
	sum := frontend.Variable(0)
	for i := range b {
		sum = api.Add(sum, b[i].Val)
	}
	api.AssertIsEqual(api.IsZero(sum), 0)
}
