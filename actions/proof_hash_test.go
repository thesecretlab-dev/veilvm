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

	preimage := BuildShieldedLedgerPublicInputsPreimage(
		marketID,
		windowID,
		clearPrice,
		totalVolume,
		fillsHash,
	)

	domainLen := len(ShieldedLedgerInputsDomainTag)
	marketOffset := domainLen
	windowOffset := marketOffset + ids.IDLen
	clearPriceOffset := windowOffset + 8
	totalVolumeOffset := clearPriceOffset + 8
	fillsLenOffset := totalVolumeOffset + 8
	fillsHashOffset := fillsLenOffset + 2
	expectedLen := fillsHashOffset + ExpectedFillsHashSize

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

	hash := ComputeShieldedLedgerPublicInputsHash(
		marketID,
		11,
		2000,
		6400,
		fillsHash,
	)
	preimage := BuildShieldedLedgerPublicInputsPreimage(
		marketID,
		11,
		2000,
		6400,
		fillsHash,
	)
	want := sha256.Sum256(preimage)
	if hash != want {
		t.Fatalf("hash mismatch")
	}
}
