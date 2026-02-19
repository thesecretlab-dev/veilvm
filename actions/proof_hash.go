package actions

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/ava-labs/avalanchego/ids"
)

const (
	ClearInputsDomainTag         = "VEIL_CLEAR_V1"
	ShieldedLedgerInputsDomainTag = "VEIL_SHIELDED_LEDGER_V1"
	ExpectedFillsHashSize        = sha256.Size
)

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
func BuildShieldedLedgerPublicInputsPreimage(
	marketID ids.ID,
	windowID uint64,
	clearPrice uint64,
	totalVolume uint64,
	fillsHash []byte,
) []byte {
	preimage := make([]byte, 0, len(ShieldedLedgerInputsDomainTag)+ids.IDLen+8+8+8+2+len(fillsHash))
	preimage = append(preimage, ShieldedLedgerInputsDomainTag...)
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

// ComputeShieldedLedgerPublicInputsHash computes the canonical digest for
// shielded-ledger-v1 proof public inputs.
func ComputeShieldedLedgerPublicInputsHash(
	marketID ids.ID,
	windowID uint64,
	clearPrice uint64,
	totalVolume uint64,
	fillsHash []byte,
) [32]byte {
	return sha256.Sum256(BuildShieldedLedgerPublicInputsPreimage(
		marketID,
		windowID,
		clearPrice,
		totalVolume,
		fillsHash,
	))
}
