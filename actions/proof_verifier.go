package actions

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
)

const (
	proofEnvelopeMagicV1     = "VZK1"
	proofEnvelopeMagicV2     = "VZK2"
	proofEnvelopeV1HeaderLen = 13 // magic(4) + proof_type(1) + proof_len(4) + witness_len(4)
	proofEnvelopeV2HeaderLen = 14 // magic(4) + proof_type(1) + circuit_len(1) + proof_len(4) + witness_len(4)
	maxProofCircuitIDLen     = 63
)

// BatchProofVerifier performs consensus-critical proof verification.
type BatchProofVerifier interface {
	Verify(
		proofType uint8,
		circuitID string,
		proof []byte,
		publicInputsHash []byte,
		publicWitness []byte,
	) error
}

var (
	proofVerifierMu     sync.RWMutex
	proofVerifier       BatchProofVerifier
	strictProofVerifier bool
)

// ConfigureBatchProofVerifier installs or clears the process-wide proof verifier.
//
// When strict=true and verifier=nil, proof-gated clears fail-closed.
func ConfigureBatchProofVerifier(verifier BatchProofVerifier, strict bool) {
	proofVerifierMu.Lock()
	defer proofVerifierMu.Unlock()

	proofVerifier = verifier
	strictProofVerifier = strict
}

// BuildProofEnvelope creates a canonical VZK1 proof payload:
// magic(4) | proof_type(1) | proof_len(4) | witness_len(4) | proof | public_witness
func BuildProofEnvelope(proofType uint8, proof []byte, publicWitness []byte) ([]byte, error) {
	if proofType == 0 || len(proof) == 0 || len(proof) > MaxProofBytesSize {
		return nil, storage.ErrInvalidProofEnvelope
	}
	if len(publicWitness) == 0 {
		return nil, storage.ErrInvalidProofEnvelope
	}
	if proofEnvelopeV1HeaderLen+len(proof)+len(publicWitness) > MaxProofBytesSize {
		return nil, storage.ErrInvalidProofEnvelope
	}

	out := make([]byte, 0, proofEnvelopeV1HeaderLen+len(proof)+len(publicWitness))
	out = append(out, []byte(proofEnvelopeMagicV1)...)
	out = append(out, proofType)
	out = binary.BigEndian.AppendUint32(out, uint32(len(proof)))
	out = binary.BigEndian.AppendUint32(out, uint32(len(publicWitness)))
	out = append(out, proof...)
	out = append(out, publicWitness...)
	return out, nil
}

// BuildProofEnvelopeWithCircuit creates a VZK2 proof payload:
// magic(4) | proof_type(1) | circuit_len(1) | proof_len(4) | witness_len(4) | circuit_id | proof | public_witness
func BuildProofEnvelopeWithCircuit(
	proofType uint8,
	circuitID string,
	proof []byte,
	publicWitness []byte,
) ([]byte, error) {
	circuitID = strings.TrimSpace(circuitID)
	if proofType == 0 || len(proof) == 0 || len(proof) > MaxProofBytesSize {
		return nil, storage.ErrInvalidProofEnvelope
	}
	if len(publicWitness) == 0 {
		return nil, storage.ErrInvalidProofEnvelope
	}
	if !isValidProofCircuitID(circuitID) {
		return nil, storage.ErrInvalidProofEnvelope
	}
	totalLen := proofEnvelopeV2HeaderLen + len(circuitID) + len(proof) + len(publicWitness)
	if totalLen > MaxProofBytesSize {
		return nil, storage.ErrInvalidProofEnvelope
	}

	out := make([]byte, 0, totalLen)
	out = append(out, []byte(proofEnvelopeMagicV2)...)
	out = append(out, proofType)
	out = append(out, byte(len(circuitID)))
	out = binary.BigEndian.AppendUint32(out, uint32(len(proof)))
	out = binary.BigEndian.AppendUint32(out, uint32(len(publicWitness)))
	out = append(out, []byte(circuitID)...)
	out = append(out, proof...)
	out = append(out, publicWitness...)
	return out, nil
}

func verifyBatchProofInConsensus(
	requiredProofType uint8,
	proofBlob []byte,
	publicInputsHash []byte,
) error {
	proofType, circuitID, proof, witness, hasEnvelope, err := parseProofEnvelope(proofBlob)
	if err != nil {
		return err
	}
	if hasEnvelope && proofType != requiredProofType {
		return storage.ErrProofTypeMismatch
	}

	verifier, strict := getBatchProofVerifier()
	if verifier == nil {
		if strict {
			return storage.ErrProofVerifierUnavailable
		}
		return nil
	}
	if err := verifier.Verify(requiredProofType, circuitID, proof, publicInputsHash, witness); err != nil {
		if errors.Is(err, storage.ErrProofVerifierUnavailable) {
			return err
		}
		return fmt.Errorf("%w: %v", storage.ErrProofVerificationFailed, err)
	}
	return nil
}

func getBatchProofVerifier() (BatchProofVerifier, bool) {
	proofVerifierMu.RLock()
	defer proofVerifierMu.RUnlock()

	return proofVerifier, strictProofVerifier
}

func parseProofEnvelope(blob []byte) (
	proofType uint8,
	circuitID string,
	proof []byte,
	witness []byte,
	hasEnvelope bool,
	err error,
) {
	if len(blob) < len(proofEnvelopeMagicV1) {
		return 0, "", blob, nil, false, nil
	}
	switch {
	case bytes.Equal(blob[:len(proofEnvelopeMagicV1)], []byte(proofEnvelopeMagicV1)):
		return parseProofEnvelopeV1(blob)
	case bytes.Equal(blob[:len(proofEnvelopeMagicV2)], []byte(proofEnvelopeMagicV2)):
		return parseProofEnvelopeV2(blob)
	default:
		return 0, "", blob, nil, false, nil
	}
}

func parseProofEnvelopeV1(blob []byte) (
	proofType uint8,
	circuitID string,
	proof []byte,
	witness []byte,
	hasEnvelope bool,
	err error,
) {
	if len(blob) < proofEnvelopeV1HeaderLen {
		return 0, "", nil, nil, false, storage.ErrInvalidProofEnvelope
	}
	proofType = blob[4]
	proofLen := int(binary.BigEndian.Uint32(blob[5:9]))
	witnessLen := int(binary.BigEndian.Uint32(blob[9:13]))
	if proofLen <= 0 || witnessLen <= 0 {
		return 0, "", nil, nil, false, storage.ErrInvalidProofEnvelope
	}

	totalLen := proofEnvelopeV1HeaderLen + proofLen + witnessLen
	if len(blob) != totalLen {
		return 0, "", nil, nil, false, storage.ErrInvalidProofEnvelope
	}

	proofStart := proofEnvelopeV1HeaderLen
	proofEnd := proofStart + proofLen
	witnessEnd := proofEnd + witnessLen

	proof = append([]byte(nil), blob[proofStart:proofEnd]...)
	witness = append([]byte(nil), blob[proofEnd:witnessEnd]...)
	return proofType, "", proof, witness, true, nil
}

func parseProofEnvelopeV2(blob []byte) (
	proofType uint8,
	circuitID string,
	proof []byte,
	witness []byte,
	hasEnvelope bool,
	err error,
) {
	if len(blob) < proofEnvelopeV2HeaderLen {
		return 0, "", nil, nil, false, storage.ErrInvalidProofEnvelope
	}
	proofType = blob[4]
	circuitLen := int(blob[5])
	proofLen := int(binary.BigEndian.Uint32(blob[6:10]))
	witnessLen := int(binary.BigEndian.Uint32(blob[10:14]))
	if circuitLen <= 0 || circuitLen > maxProofCircuitIDLen || proofLen <= 0 || witnessLen <= 0 {
		return 0, "", nil, nil, false, storage.ErrInvalidProofEnvelope
	}

	totalLen := proofEnvelopeV2HeaderLen + circuitLen + proofLen + witnessLen
	if len(blob) != totalLen {
		return 0, "", nil, nil, false, storage.ErrInvalidProofEnvelope
	}

	circuitStart := proofEnvelopeV2HeaderLen
	circuitEnd := circuitStart + circuitLen
	circuitID = string(blob[circuitStart:circuitEnd])
	if !isValidProofCircuitID(circuitID) {
		return 0, "", nil, nil, false, storage.ErrInvalidProofEnvelope
	}
	proofStart := circuitEnd
	proofEnd := proofStart + proofLen
	witnessEnd := proofEnd + witnessLen

	proof = append([]byte(nil), blob[proofStart:proofEnd]...)
	witness = append([]byte(nil), blob[proofEnd:witnessEnd]...)
	return proofType, circuitID, proof, witness, true, nil
}

func isValidProofCircuitID(circuitID string) bool {
	if len(circuitID) == 0 || len(circuitID) > maxProofCircuitIDLen {
		return false
	}
	for _, r := range circuitID {
		if r > 127 {
			return false
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}
