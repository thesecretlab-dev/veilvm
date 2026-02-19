package zk

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/examples/veilvm/actions"
	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	groth16bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

func TestVerifyGroth16ShieldedLedgerRoundTrip(t *testing.T) {
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &ShieldedLedgerCircuitV1{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	preimage := buildSampleShieldedPreimage(t)
	digest := sha256.Sum256(preimage)
	assignment, err := NewShieldedLedgerAssignment(preimage, digest[:])
	if err != nil {
		t.Fatalf("assignment: %v", err)
	}

	fullWitness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("new witness: %v", err)
	}
	publicWitness, err := fullWitness.Public()
	if err != nil {
		t.Fatalf("public witness: %v", err)
	}
	publicWitnessBytes, err := publicWitness.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal public witness: %v", err)
	}

	proofAny, err := groth16.Prove(ccs, pk, fullWitness)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	proof, ok := proofAny.(*groth16bn254.Proof)
	if !ok {
		t.Fatalf("unexpected proof type %T", proofAny)
	}
	var proofBuf bytes.Buffer
	if _, err := proof.WriteTo(&proofBuf); err != nil {
		t.Fatalf("serialize proof: %v", err)
	}

	vkBN, ok := vk.(*groth16bn254.VerifyingKey)
	if !ok {
		t.Fatalf("unexpected verifying key type %T", vk)
	}
	if err := verifyGroth16(vkBN, mconsts.ProofCircuitShieldedLedgerV1, proofBuf.Bytes(), digest[:], publicWitnessBytes); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestBuildPublicWitnessVectorShieldedLedgerHashMismatch(t *testing.T) {
	preimage := buildSampleShieldedPreimage(t)
	digest := sha256.Sum256(preimage)

	assignment, err := NewShieldedLedgerAssignment(preimage, digest[:])
	if err != nil {
		t.Fatalf("assignment: %v", err)
	}
	fullWitness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("new witness: %v", err)
	}
	publicWitness, err := fullWitness.Public()
	if err != nil {
		t.Fatalf("public witness: %v", err)
	}
	publicWitnessBytes, err := publicWitness.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal public witness: %v", err)
	}

	badHash := append([]byte(nil), digest[:]...)
	badHash[0] ^= 0x01

	_, err = buildPublicWitnessVector(mconsts.ProofCircuitShieldedLedgerV1, badHash, publicWitnessBytes)
	if err == nil {
		t.Fatalf("expected mismatch error")
	}
	if err != storage.ErrProofPublicInputsMismatch {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShieldedLedgerCircuitRejectsZeroWindowID(t *testing.T) {
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &ShieldedLedgerCircuitV1{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	pk, _, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	preimage := buildSampleShieldedPreimage(t)
	for i := 0; i < ShieldedLedgerWindowIDLen; i++ {
		preimage[ShieldedLedgerWindowIDOffset+i] = 0
	}
	digest := sha256.Sum256(preimage)
	assignment, err := NewShieldedLedgerAssignment(preimage, digest[:])
	if err != nil {
		t.Fatalf("assignment: %v", err)
	}
	fullWitness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("new witness: %v", err)
	}
	if _, err := groth16.Prove(ccs, pk, fullWitness); err == nil {
		t.Fatalf("expected proving failure for zero window id")
	}
}

func buildSampleShieldedPreimage(t *testing.T) []byte {
	t.Helper()

	var marketID ids.ID
	for i := range marketID {
		marketID[i] = byte(i + 1)
	}
	fillsHash := make([]byte, actions.ExpectedFillsHashSize)
	for i := range fillsHash {
		fillsHash[i] = byte(0xA0 + i)
	}
	preimage := actions.BuildShieldedLedgerPublicInputsPreimage(
		marketID,
		17,
		2500,
		7200,
		fillsHash,
	)
	if len(preimage) != ShieldedLedgerPreimageLen {
		t.Fatalf("unexpected preimage len: got=%d want=%d", len(preimage), ShieldedLedgerPreimageLen)
	}
	return preimage
}
