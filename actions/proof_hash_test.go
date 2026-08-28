package actions

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
)

func TestBuildShieldedLedgerPublicInputsPreimageLayout(t *testing.T) {
	var marketID ids.ID
	for i := range marketID {
		marketID[i] = byte(i + 1)
	}

	fillsHash := make([]byte, ExpectedFillsHashSize)
	for i := range fillsHash {
		fillsHash[i] = byte(0x80 + i)
	}

	const (
		windowID    = uint64(7)
		clearPrice  = uint64(1025)
		totalVolume = uint64(4096)
	)

	in := DerivedShieldedLedgerPublicInputs(
		marketID,
		windowID,
		clearPrice,
		totalVolume,
		fillsHash,
	)
	preimage := BuildShieldedLedgerPublicInputsPreimage(in)

	domainLen := len(ShieldedLedgerInputsDomainTag)
	marketOffset := domainLen
	windowOffset := marketOffset + ids.IDLen
	clearPriceOffset := windowOffset + 8
	totalVolumeOffset := clearPriceOffset + 8
	fillsLenOffset := totalVolumeOffset + 8
	fillsHashOffset := fillsLenOffset + 2
	commitmentsOffset := fillsHashOffset + ExpectedFillsHashSize
	nullifiersOffset := commitmentsOffset + ExpectedRootHashSize
	prevRootOffset := nullifiersOffset + ExpectedRootHashSize
	nextRootOffset := prevRootOffset + ExpectedRootHashSize
	expectedLen := nextRootOffset + ExpectedRootHashSize

	if len(preimage) != expectedLen {
		t.Fatalf("unexpected preimage length: got=%d want=%d", len(preimage), expectedLen)
	}
	if !bytes.Equal(preimage[:domainLen], []byte(ShieldedLedgerInputsDomainTag)) {
		t.Fatalf("domain tag mismatch")
	}
	if !bytes.Equal(preimage[marketOffset:windowOffset], marketID[:]) {
		t.Fatalf("market id mismatch")
	}
	if got := binary.BigEndian.Uint64(preimage[windowOffset : windowOffset+8]); got != windowID {
		t.Fatalf("window id mismatch: got=%d want=%d", got, windowID)
	}
	if got := binary.BigEndian.Uint64(preimage[clearPriceOffset : clearPriceOffset+8]); got != clearPrice {
		t.Fatalf("clear price mismatch: got=%d want=%d", got, clearPrice)
	}
	if got := binary.BigEndian.Uint64(preimage[totalVolumeOffset : totalVolumeOffset+8]); got != totalVolume {
		t.Fatalf("total volume mismatch: got=%d want=%d", got, totalVolume)
	}
	if got := binary.BigEndian.Uint16(preimage[fillsLenOffset : fillsLenOffset+2]); got != uint16(ExpectedFillsHashSize) {
		t.Fatalf("fills hash length mismatch: got=%d want=%d", got, ExpectedFillsHashSize)
	}
	if !bytes.Equal(preimage[fillsHashOffset:fillsHashOffset+ExpectedFillsHashSize], fillsHash) {
		t.Fatalf("fills hash mismatch")
	}
	if !bytes.Equal(preimage[commitmentsOffset:nullifiersOffset], in.CommitmentsHash) {
		t.Fatalf("commitments hash mismatch")
	}
	if !bytes.Equal(preimage[nullifiersOffset:prevRootOffset], in.NullifiersHash) {
		t.Fatalf("nullifiers hash mismatch")
	}
	if !bytes.Equal(preimage[prevRootOffset:nextRootOffset], in.PrevStateRoot) {
		t.Fatalf("prev state root mismatch")
	}
	if !bytes.Equal(preimage[nextRootOffset:nextRootOffset+ExpectedRootHashSize], in.NextStateRoot) {
		t.Fatalf("next state root mismatch")
	}
	if bytes.Equal(in.CommitmentsHash, in.NullifiersHash) {
		t.Fatalf("commitments and nullifiers hashes must be domain-separated")
	}
}

func TestComputeShieldedLedgerPublicInputsHashMatchesCanonicalPreimage(t *testing.T) {
	var marketID ids.ID
	for i := range marketID {
		marketID[i] = byte(0xF0 - i)
	}

	fillsHash := make([]byte, ExpectedFillsHashSize)
	for i := range fillsHash {
		fillsHash[i] = byte(i)
	}

	in := DerivedShieldedLedgerPublicInputs(
		marketID,
		11,
		2000,
		6400,
		fillsHash,
	)
	hash := ComputeShieldedLedgerPublicInputsHash(in)
	preimage := BuildShieldedLedgerPublicInputsPreimage(in)
	want := sha256.Sum256(preimage)
	if hash != want {
		t.Fatalf("hash mismatch")
	}
	mut := in
	mut.NextStateRoot = taggedSHA256("VEIL_NEXT_ROOT_V1_TAMPER", mut.NextStateRoot)
	if ComputeShieldedLedgerPublicInputsHash(mut) == hash {
		t.Fatalf("state-root slot must bind the digest")
	}
}
