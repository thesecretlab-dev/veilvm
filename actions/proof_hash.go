package actions

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/ava-labs/avalanchego/ids"
)

const (
	ClearInputsDomainTag          = "VEIL_CLEAR_V1"
	ShieldedLedgerInputsDomainTag = "VEIL_SHIELDED_LEDGER_V1"
	ExpectedFillsHashSize         = sha256.Size
	ExpectedRootHashSize          = sha256.Size
)

type ShieldedLedgerPublicInputs struct {
	MarketID         ids.ID
	WindowID         uint64
	ClearPrice       uint64
	TotalVolume      uint64
	FillsHash        []byte
	CommitmentsHash  []byte
	NullifiersHash   []byte
	PrevStateRoot    []byte
	NextStateRoot    []byte
}

// BuildClearPublicInputsPreimage canonicalizes clear-batch inputs into the byte
// preimage used for public input hashing.
func BuildClearPublicInputsPreimage(
	marketID ids.ID,
	windowID uint64,
	clearPrice uint64,
	totalVolume uint64,
	fillsHash []byte,
) []byte {
	preimage := make([]byte, 0, len(ClearInputsDomainTag)+ids.IDLen+8+8+8+2+len(fillsHash))
	preimage = append(preimage, ClearInputsDomainTag...)
	preimage = append(preimage, marketID[:]...)

	var scratch [8]byte
	binary.BigEndian.PutUint64(scratch[:], windowID)
	preimage = append(preimage, scratch[:]...)
	binary.BigEndian.PutUint64(scratch[:], clearPrice)
	preimage = append(preimage, scratch[:]...)
	binary.BigEndian.PutUint64(scratch[:], totalVolume)
	preimage = append(preimage, scratch[:]...)

	var hashLen [2]byte
	binary.BigEndian.PutUint16(hashLen[:], uint16(len(fillsHash)))
	preimage = append(preimage, hashLen[:]...)
	preimage = append(preimage, fillsHash...)
	return preimage
}

// ComputeClearPublicInputsHash canonicalizes clear-batch inputs into a single
// domain-separated digest that the prover commits to.
func ComputeClearPublicInputsHash(
	marketID ids.ID,
	windowID uint64,
	clearPrice uint64,
	totalVolume uint64,
	fillsHash []byte,
) [32]byte {
	return sha256.Sum256(BuildClearPublicInputsPreimage(
		marketID,
		windowID,
		clearPrice,
		totalVolume,
		fillsHash,
	))
}

// BuildShieldedLedgerPublicInputsPreimage canonicalizes shielded-ledger proof
// public inputs into a domain-separated preimage.
func BuildShieldedLedgerPublicInputsPreimage(in ShieldedLedgerPublicInputs) []byte {
	fills := in.FillsHash
	preimage := make([]byte, 0, len(ShieldedLedgerInputsDomainTag)+ids.IDLen+8+8+8+2+len(fills)+4*ExpectedRootHashSize)
	preimage = append(preimage, ShieldedLedgerInputsDomainTag...)
	preimage = append(preimage, in.MarketID[:]...)

	var scratch [8]byte
	binary.BigEndian.PutUint64(scratch[:], in.WindowID)
	preimage = append(preimage, scratch[:]...)
	binary.BigEndian.PutUint64(scratch[:], in.ClearPrice)
	preimage = append(preimage, scratch[:]...)
	binary.BigEndian.PutUint64(scratch[:], in.TotalVolume)
	preimage = append(preimage, scratch[:]...)

	var hashLen [2]byte
	binary.BigEndian.PutUint16(hashLen[:], uint16(len(fills)))
	preimage = append(preimage, hashLen[:]...)
	preimage = append(preimage, fills...)
	preimage = append(preimage, pad32(in.CommitmentsHash)...)
	preimage = append(preimage, pad32(in.NullifiersHash)...)
	preimage = append(preimage, pad32(in.PrevStateRoot)...)
	preimage = append(preimage, pad32(in.NextStateRoot)...)
	return preimage
}

func pad32(b []byte) []byte {
	out := make([]byte, ExpectedRootHashSize)
	copy(out, b)
	return out
}

func ComputeShieldedLedgerPublicInputsHash(in ShieldedLedgerPublicInputs) [32]byte {
	return sha256.Sum256(BuildShieldedLedgerPublicInputsPreimage(in))
}

// DerivedShieldedLedgerPublicInputs fills commitments/nullifiers/state-root
// slots with domain-separated SHA256 of the clear-batch fields. Those extra
// public slots are bound by the groth16 digest; they are not merkle proofs.
func DerivedShieldedLedgerPublicInputs(
	marketID ids.ID,
	windowID uint64,
	clearPrice uint64,
	totalVolume uint64,
	fillsHash []byte,
) ShieldedLedgerPublicInputs {
	win := u64be(windowID)
	price := u64be(clearPrice)
	vol := u64be(totalVolume)
	return ShieldedLedgerPublicInputs{
		MarketID:        marketID,
		WindowID:        windowID,
		ClearPrice:      clearPrice,
		TotalVolume:     totalVolume,
		FillsHash:       fillsHash,
		CommitmentsHash: taggedSHA256("VEIL_COMMITMENTS_V1", marketID[:], win, fillsHash),
		NullifiersHash:  taggedSHA256("VEIL_NULLIFIERS_V1", marketID[:], win, fillsHash),
		PrevStateRoot:   taggedSHA256("VEIL_PREV_ROOT_V1", marketID[:], win),
		NextStateRoot:   taggedSHA256("VEIL_NEXT_ROOT_V1", marketID[:], win, fillsHash, price, vol),
	}
}

func u64be(v uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], v)
	out := make([]byte, 8)
	copy(out, b[:])
	return out
}

func taggedSHA256(tag string, parts ...[]byte) []byte {
	h := sha256.New()
	_, _ = h.Write([]byte(tag))
	for _, p := range parts {
		_, _ = h.Write(p)
	}
	return h.Sum(nil)
}
