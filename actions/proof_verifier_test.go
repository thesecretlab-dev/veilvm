package actions

import (
	"bytes"
	"testing"

	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
)

type captureVerifier struct {
	t          *testing.T
	wantType   uint8
	wantProof  []byte
	wantHash   []byte
	wantWit    []byte
	gotCircuit string
}

func (c *captureVerifier) Verify(
	proofType uint8,
	circuitID string,
	proof []byte,
	publicInputsHash []byte,
	publicWitness []byte,
) error {
	if proofType != c.wantType {
		c.t.Fatalf("unexpected proof type: got=%d want=%d", proofType, c.wantType)
	}
	if !bytes.Equal(proof, c.wantProof) {
		c.t.Fatalf("unexpected proof bytes")
	}
	if !bytes.Equal(publicInputsHash, c.wantHash) {
		c.t.Fatalf("unexpected public inputs hash")
	}
	if !bytes.Equal(publicWitness, c.wantWit) {
		c.t.Fatalf("unexpected public witness")
	}
	c.gotCircuit = circuitID
	return nil
}

func TestBuildProofEnvelopeWithCircuitRoundTrip(t *testing.T) {
	proof := []byte{1, 2, 3, 4}
	witness := []byte{9, 8, 7}
	envelope, err := BuildProofEnvelopeWithCircuit(
		mconsts.ProofTypeGroth16,
		mconsts.ProofCircuitClearHashV1,
		proof,
		witness,
	)
	if err != nil {
		t.Fatalf("build envelope: %v", err)
	}

	proofType, circuitID, parsedProof, parsedWitness, hasEnvelope, err := parseProofEnvelope(envelope)
	if err != nil {
		t.Fatalf("parse envelope: %v", err)
	}
	if !hasEnvelope {
		t.Fatalf("expected envelope")
	}
	if proofType != mconsts.ProofTypeGroth16 {
		t.Fatalf("unexpected proof type: %d", proofType)
	}
	if circuitID != mconsts.ProofCircuitClearHashV1 {
		t.Fatalf("unexpected circuit id: %s", circuitID)
	}
	if !bytes.Equal(parsedProof, proof) {
		t.Fatalf("proof mismatch")
	}
	if !bytes.Equal(parsedWitness, witness) {
		t.Fatalf("witness mismatch")
	}
}

func TestParseProofEnvelopeLegacyRaw(t *testing.T) {
	raw := []byte{0xaa, 0xbb, 0xcc}
	_, circuitID, proof, witness, hasEnvelope, err := parseProofEnvelope(raw)
	if err != nil {
		t.Fatalf("parse raw: %v", err)
	}
	if hasEnvelope {
		t.Fatalf("expected non-envelope mode")
	}
	if circuitID != "" {
		t.Fatalf("unexpected circuit id for raw proof: %q", circuitID)
	}
	if witness != nil {
		t.Fatalf("expected nil witness for raw proof")
	}
	if !bytes.Equal(proof, raw) {
		t.Fatalf("raw proof mismatch")
	}
}

func TestVerifyBatchProofPassesCircuitID(t *testing.T) {
	prevVerifier, prevStrict := getBatchProofVerifier()
	defer ConfigureBatchProofVerifier(prevVerifier, prevStrict)

	proof := []byte{0x01, 0x02}
	witness := []byte{0x03, 0x04}
	hash := []byte{0x05, 0x06}
	envelope, err := BuildProofEnvelopeWithCircuit(
		mconsts.ProofTypeGroth16,
		mconsts.ProofCircuitClearHashV1,
		proof,
		witness,
	)
	if err != nil {
		t.Fatalf("build envelope: %v", err)
	}

	verifier := &captureVerifier{
		t:         t,
		wantType:  mconsts.ProofTypeGroth16,
		wantProof: proof,
		wantHash:  hash,
		wantWit:   witness,
	}
	ConfigureBatchProofVerifier(verifier, true)
	if err := verifyBatchProofInConsensus(mconsts.ProofTypeGroth16, envelope, hash); err != nil {
		t.Fatalf("verify batch proof: %v", err)
	}
	if verifier.gotCircuit != mconsts.ProofCircuitClearHashV1 {
		t.Fatalf("unexpected circuit passed to verifier: %s", verifier.gotCircuit)
	}
}

func TestBuildProofEnvelopeWithCircuitRejectsInvalidCircuitID(t *testing.T) {
	_, err := BuildProofEnvelopeWithCircuit(
		mconsts.ProofTypeGroth16,
		"bad*id",
		[]byte{0x01},
		[]byte{0x02},
	)
	if err == nil {
		t.Fatalf("expected invalid circuit id error")
	}
}
