package actions

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
)

const glyphDomainTag = "VEIL_GLYPH_V1"

const (
	GlyphRarityCommon    uint8 = 1
	GlyphRarityRare      uint8 = 2
	GlyphRarityEpic      uint8 = 3
	GlyphRarityLegendary uint8 = 4
	GlyphRarityMythic    uint8 = 5
)

const (
	GlyphClassAsh     uint8 = 1
	GlyphClassCrown   uint8 = 2
	GlyphClassCipher  uint8 = 3
	GlyphClassEclipse uint8 = 4
	GlyphClassAegis   uint8 = 5
	GlyphClassRelic   uint8 = 6
)

func deriveGlyph(
	txID ids.ID,
	marketID ids.ID,
	windowID uint64,
	prover codec.Address,
	commitment [32]byte,
	publicInputsHash []byte,
	timestamp int64,
) storage.Glyph {
	entropy := deriveGlyphEntropy(txID, marketID, windowID, prover, commitment, publicInputsHash)

	glyph := storage.Glyph{
		Class:           deriveGlyphClass(entropy),
		Rarity:          deriveGlyphRarity(entropy),
		CreatedAtMs:     timestamp,
		Prover:          prover,
		ProofCommitment: commitment,
		Entropy:         entropy,
	}
	copy(glyph.PublicInputsHash[:], publicInputsHash)
	return glyph
}

func deriveGlyphEntropy(
	txID ids.ID,
	marketID ids.ID,
	windowID uint64,
	prover codec.Address,
	commitment [32]byte,
	publicInputsHash []byte,
) [32]byte {
	preimage := make([]byte, 0, len(glyphDomainTag)+ids.IDLen+ids.IDLen+8+codec.AddressLen+32+len(publicInputsHash))
	preimage = append(preimage, glyphDomainTag...)
	preimage = append(preimage, txID[:]...)
	preimage = append(preimage, marketID[:]...)
	var window [8]byte
	binary.BigEndian.PutUint64(window[:], windowID)
	preimage = append(preimage, window[:]...)
	preimage = append(preimage, prover[:]...)
	preimage = append(preimage, commitment[:]...)
	preimage = append(preimage, publicInputsHash...)
	return sha256.Sum256(preimage)
}

func deriveGlyphClass(entropy [32]byte) uint8 {
	switch entropy[2] % 6 {
	case 0:
		return GlyphClassAsh
	case 1:
		return GlyphClassCrown
	case 2:
		return GlyphClassCipher
	case 3:
		return GlyphClassEclipse
	case 4:
		return GlyphClassAegis
	default:
		return GlyphClassRelic
	}
}

func deriveGlyphRarity(entropy [32]byte) uint8 {
	roll := uint32(binary.BigEndian.Uint16(entropy[:2])) * 10_000 / 65_536
	switch {
	case roll < 2:
		return GlyphRarityMythic
	case roll < 20:
		return GlyphRarityLegendary
	case roll < 150:
		return GlyphRarityEpic
	case roll < 800:
		return GlyphRarityRare
	default:
		return GlyphRarityCommon
	}
}
