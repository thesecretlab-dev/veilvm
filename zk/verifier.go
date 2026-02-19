package zk

import (
	"bytes"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/consensys/gnark-crypto/ecc"
	bn254fr "github.com/consensys/gnark-crypto/ecc/bn254/fr"
	groth16bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	plonkbn254 "github.com/consensys/gnark/backend/plonk/bn254"
	"github.com/consensys/gnark/backend/witness"

	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
)

type Config struct {
	Groth16VerifyingKeyPath string
	PlonkVerifyingKeyPath   string
	RequiredCircuitID       string
}

type Verifier struct {
	groth16VK         *groth16bn254.VerifyingKey
	plonkVK           *plonkbn254.VerifyingKey
	requiredCircuitID string
}

func NewVerifier(cfg Config) (*Verifier, error) {
	v := &Verifier{}

	if p := strings.TrimSpace(cfg.Groth16VerifyingKeyPath); p != "" {
		vk, err := loadGroth16VK(p)
		if err != nil {
			return nil, fmt.Errorf("load groth16 vk: %w", err)
		}
		v.groth16VK = vk
	}
	if p := strings.TrimSpace(cfg.PlonkVerifyingKeyPath); p != "" {
		vk, err := loadPlonkVK(p)
		if err != nil {
			return nil, fmt.Errorf("load plonk vk: %w", err)
		}
		v.plonkVK = vk
	}
	if v.groth16VK == nil && v.plonkVK == nil {
		return nil, storage.ErrProofVerifierUnavailable
	}
	v.requiredCircuitID = strings.TrimSpace(cfg.RequiredCircuitID)
	if v.requiredCircuitID != "" && !isSupportedCircuitID(v.requiredCircuitID) {
		return nil, fmt.Errorf("%w: %s", storage.ErrUnsupportedProofCircuit, v.requiredCircuitID)
	}

	return v, nil
}

func (v *Verifier) Verify(
	proofType uint8,
	circuitID string,
	proof []byte,
	publicInputsHash []byte,
	publicWitness []byte,
) error {
	circuitID = normalizeCircuitID(circuitID)
	if v.requiredCircuitID != "" && circuitID != v.requiredCircuitID {
		return storage.ErrProofCircuitMismatch
	}
	if !isSupportedCircuitID(circuitID) {
		return storage.ErrUnsupportedProofCircuit
	}

	switch proofType {
	case mconsts.ProofTypeGroth16:
		if v.groth16VK == nil {
			return storage.ErrProofVerifierUnavailable
		}
		return verifyGroth16(v.groth16VK, circuitID, proof, publicInputsHash, publicWitness)
	case mconsts.ProofTypePlonk:
		if v.plonkVK == nil {
			return storage.ErrProofVerifierUnavailable
		}
		return verifyPlonk(v.plonkVK, circuitID, proof, publicInputsHash, publicWitness)
	default:
		return storage.ErrProofTypeMismatch
	}
}

func loadGroth16VK(path string) (*groth16bn254.VerifyingKey, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vk := new(groth16bn254.VerifyingKey)
	if _, err := vk.ReadFrom(f); err != nil {
		return nil, err
	}
	return vk, nil
}

func loadPlonkVK(path string) (*plonkbn254.VerifyingKey, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vk := new(plonkbn254.VerifyingKey)
	if _, err := vk.ReadFrom(f); err != nil {
		return nil, err
	}
	return vk, nil
}

func verifyGroth16(
	vk *groth16bn254.VerifyingKey,
	circuitID string,
	proofBytes []byte,
	publicInputsHash []byte,
	publicWitnessBytes []byte,
) error {
	proof := new(groth16bn254.Proof)
	if _, err := proof.ReadFrom(bytes.NewReader(proofBytes)); err != nil {
		return err
	}
	publicWitness, err := buildPublicWitnessVector(circuitID, publicInputsHash, publicWitnessBytes)
	if err != nil {
		return err
	}
	return groth16bn254.Verify(proof, vk, publicWitness)
}

func verifyPlonk(
	vk *plonkbn254.VerifyingKey,
	circuitID string,
	proofBytes []byte,
	publicInputsHash []byte,
	publicWitnessBytes []byte,
) error {
	proof := new(plonkbn254.Proof)
	if _, err := proof.ReadFrom(bytes.NewReader(proofBytes)); err != nil {
		return err
	}
	publicWitness, err := buildPublicWitnessVector(circuitID, publicInputsHash, publicWitnessBytes)
	if err != nil {
		return err
	}
	return plonkbn254.Verify(proof, vk, publicWitness)
}

func buildPublicWitnessVector(
	circuitID string,
	publicInputsHash []byte,
	publicWitnessBytes []byte,
) (bn254fr.Vector, error) {
	if len(publicWitnessBytes) == 0 {
		return nil, storage.ErrInvalidProofEnvelope
	}

	w, err := witness.New(ecc.BN254.ScalarField())
	if err != nil {
		return nil, err
	}
	if err := w.UnmarshalBinary(publicWitnessBytes); err != nil {
		return nil, err
	}

	publicW, err := w.Public()
	if err == nil {
		w = publicW
	}

	vec, ok := w.Vector().(bn254fr.Vector)
	if !ok {
		return nil, fmt.Errorf("unexpected witness vector type %T", w.Vector())
	}
	if len(vec) == 0 {
		return nil, storage.ErrInvalidProofEnvelope
	}

	switch circuitID {
	case mconsts.ProofCircuitClearHashV1:
		if err := validateDigestVector(publicInputsHash, vec); err != nil {
			return nil, err
		}
	case mconsts.ProofCircuitShieldedLedgerV1:
		if err := validateDigestVector(publicInputsHash, vec); err != nil {
			return nil, err
		}
	default:
		return nil, storage.ErrUnsupportedProofCircuit
	}

	return vec, nil
}

func normalizeCircuitID(circuitID string) string {
	circuitID = strings.TrimSpace(circuitID)
	if circuitID == "" {
		// Backward compatibility for VZK1 envelopes:
		// treat missing circuit metadata as clear-hash v1.
		return mconsts.ProofCircuitClearHashV1
	}
	return circuitID
}

func isSupportedCircuitID(circuitID string) bool {
	switch circuitID {
	case mconsts.ProofCircuitClearHashV1:
		return true
	case mconsts.ProofCircuitShieldedLedgerV1:
		return true
	default:
		return false
	}
}

func hashToFieldElement(publicInputsHash []byte) (bn254fr.Element, error) {
	var el bn254fr.Element
	if len(publicInputsHash) == 0 {
		return el, storage.ErrInvalidProofEnvelope
	}

	field := ecc.BN254.ScalarField()
	value := new(big.Int).SetBytes(publicInputsHash)
	value.Mod(value, field)
	el.SetBigInt(value)
	return el, nil
}

func validateDigestVector(publicInputsHash []byte, vec bn254fr.Vector) error {
	switch len(vec) {
	case 1:
		expected, err := hashToFieldElement(publicInputsHash)
		if err != nil {
			return err
		}
		if !vec[0].Equal(&expected) {
			return storage.ErrProofPublicInputsMismatch
		}
		return nil
	case ClearHashDigestLen:
		if len(publicInputsHash) != ClearHashDigestLen {
			return storage.ErrInvalidProofEnvelope
		}
		for i := 0; i < ClearHashDigestLen; i++ {
			if !vec[i].IsUint64() || vec[i].Uint64() != uint64(publicInputsHash[i]) {
				return storage.ErrProofPublicInputsMismatch
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported digest witness length: %d", len(vec))
	}
}
