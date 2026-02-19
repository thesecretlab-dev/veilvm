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
	ClearHashDigestLen   = sha256.Size
	ClearHashPreimageLen = len(actions.ClearInputsDomainTag) + ids.IDLen + 8 + 8 + 8 + 2 + actions.ExpectedFillsHashSize
)

// ClearHashCircuit proves that Digest == SHA256(Preimage).
//
// Preimage is private witness material. Digest is public.
type ClearHashCircuit struct {
	Preimage [ClearHashPreimageLen]uints.U8
	Digest   [ClearHashDigestLen]uints.U8 `gnark:",public"`
}

func (c *ClearHashCircuit) Define(api frontend.API) error {
	h, err := gnarksha2.New(api)
	if err != nil {
		return err
	}
	uapi, err := uints.New[uints.U32](api)
	if err != nil {
		return err
	}
	h.Write(c.Preimage[:])
	sum := h.Sum()
	if len(sum) != ClearHashDigestLen {
		return fmt.Errorf("unexpected digest size: %d", len(sum))
	}
	for i := 0; i < ClearHashDigestLen; i++ {
		uapi.ByteAssertEq(c.Digest[i], sum[i])
	}
	return nil
}

func NewClearHashAssignment(preimage []byte, digest []byte) (*ClearHashCircuit, error) {
	if len(preimage) != ClearHashPreimageLen {
		return nil, fmt.Errorf("invalid clear-hash preimage size: got=%d expected=%d", len(preimage), ClearHashPreimageLen)
	}
	if len(digest) != ClearHashDigestLen {
		return nil, fmt.Errorf("invalid clear-hash digest size: got=%d expected=%d", len(digest), ClearHashDigestLen)
	}

	out := &ClearHashCircuit{}
	pre := uints.NewU8Array(preimage)
	pub := uints.NewU8Array(digest)
	for i := 0; i < ClearHashPreimageLen; i++ {
		out.Preimage[i] = pre[i]
	}
	for i := 0; i < ClearHashDigestLen; i++ {
		out.Digest[i] = pub[i]
	}
	return out, nil
}
