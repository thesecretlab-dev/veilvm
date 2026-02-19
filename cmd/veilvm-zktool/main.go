package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/examples/veilvm/actions"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	groth16bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"

	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	"github.com/ava-labs/hypersdk/examples/veilvm/zk"
)

func main() {
	var (
		outDir       string
		writeSample  bool
		writeKeyPair bool
		circuitID    string
	)
	flag.StringVar(&outDir, "out", "./zk-fixture", "output directory")
	flag.BoolVar(&writeSample, "sample", true, "write sample proof and envelope artifacts")
	flag.BoolVar(&writeKeyPair, "keys", true, "write groth16 proving/verifying key artifacts")
	flag.StringVar(
		&circuitID,
		"circuit",
		mconsts.ProofCircuitClearHashV1,
		"proof circuit id (clearhash-v1|shielded-ledger-v1)",
	)
	flag.Parse()
	circuitID = strings.TrimSpace(circuitID)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatalf("mkdir %s: %v", outDir, err)
	}

	circuit, err := selectedCircuit(circuitID)
	if err != nil {
		fatalf("select circuit: %v", err)
	}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		fatalf("compile circuit: %v", err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		fatalf("setup: %v", err)
	}

	pkBN, ok := pk.(*groth16bn254.ProvingKey)
	if !ok {
		fatalf("unexpected proving key type %T", pk)
	}
	vkBN, ok := vk.(*groth16bn254.VerifyingKey)
	if !ok {
		fatalf("unexpected verifying key type %T", vk)
	}
	pkName, vkName, samplePrefix, writeCompatArtifacts, err := artifactNames(circuitID)
	if err != nil {
		fatalf("artifact names: %v", err)
	}
	pkPath := filepath.Join(outDir, pkName)
	vkPath := filepath.Join(outDir, vkName)

	if writeKeyPair {
		if err := writeWithWriterTo(pkPath, pkBN); err != nil {
			fatalf("write proving key: %v", err)
		}
		if err := writeWithWriterTo(vkPath, vkBN); err != nil {
			fatalf("write verifying key: %v", err)
		}
		if writeCompatArtifacts {
			// compatibility copies for older clear-hash scripts/envs
			if err := writeWithWriterTo(filepath.Join(outDir, "groth16_hashbinding_pk.bin"), pkBN); err != nil {
				fatalf("write compat proving key: %v", err)
			}
			if err := writeWithWriterTo(filepath.Join(outDir, "groth16_hashbinding_vk.bin"), vkBN); err != nil {
				fatalf("write compat verifying key: %v", err)
			}
		}
	}

	if writeSample {
		preimage, hashBytes, err := buildSamplePreimage(circuitID)
		if err != nil {
			fatalf("build sample preimage: %v", err)
		}
		assignment, err := buildAssignment(circuitID, preimage, hashBytes)
		if err != nil {
			fatalf("build assignment: %v", err)
		}

		fullWitness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
		if err != nil {
			fatalf("build witness: %v", err)
		}
		publicWitness, err := fullWitness.Public()
		if err != nil {
			fatalf("extract public witness: %v", err)
		}
		publicWitnessBytes, err := publicWitness.MarshalBinary()
		if err != nil {
			fatalf("marshal public witness: %v", err)
		}

		proofAny, err := groth16.Prove(ccs, pk, fullWitness)
		if err != nil {
			fatalf("prove: %v", err)
		}
		proofBN, ok := proofAny.(*groth16bn254.Proof)
		if !ok {
			fatalf("unexpected proof type %T", proofAny)
		}
		var proofBuf bytes.Buffer
		if _, err := proofBN.WriteTo(&proofBuf); err != nil {
			fatalf("serialize proof: %v", err)
		}
		proofBytes := proofBuf.Bytes()
		envelope, err := actions.BuildProofEnvelopeWithCircuit(
			mconsts.ProofTypeGroth16,
			circuitID,
			proofBytes,
			publicWitnessBytes,
		)
		if err != nil {
			fatalf("build proof envelope: %v", err)
		}

		if err := os.WriteFile(filepath.Join(outDir, samplePrefix+"_public_inputs_hash.hex"), []byte(hex.EncodeToString(hashBytes)), 0o644); err != nil {
			fatalf("write sample hash: %v", err)
		}
		if err := os.WriteFile(filepath.Join(outDir, samplePrefix+"_preimage.hex"), []byte(hex.EncodeToString(preimage)), 0o644); err != nil {
			fatalf("write sample preimage: %v", err)
		}
		if err := os.WriteFile(filepath.Join(outDir, samplePrefix+"_public_witness.bin"), publicWitnessBytes, 0o644); err != nil {
			fatalf("write sample witness: %v", err)
		}
		if err := os.WriteFile(filepath.Join(outDir, samplePrefix+"_proof.bin"), proofBytes, 0o644); err != nil {
			fatalf("write sample proof: %v", err)
		}
		if err := os.WriteFile(filepath.Join(outDir, samplePrefix+"_proof_envelope.bin"), envelope, 0o644); err != nil {
			fatalf("write sample envelope: %v", err)
		}

		// Preserve historic clear-hash sample names for downstream scripts.
		if circuitID == mconsts.ProofCircuitClearHashV1 {
			if err := os.WriteFile(filepath.Join(outDir, "sample_public_inputs_hash.hex"), []byte(hex.EncodeToString(hashBytes)), 0o644); err != nil {
				fatalf("write compat sample hash: %v", err)
			}
			if err := os.WriteFile(filepath.Join(outDir, "sample_preimage.hex"), []byte(hex.EncodeToString(preimage)), 0o644); err != nil {
				fatalf("write compat sample preimage: %v", err)
			}
			if err := os.WriteFile(filepath.Join(outDir, "sample_public_witness.bin"), publicWitnessBytes, 0o644); err != nil {
				fatalf("write compat sample witness: %v", err)
			}
			if err := os.WriteFile(filepath.Join(outDir, "sample_proof.bin"), proofBytes, 0o644); err != nil {
				fatalf("write compat sample proof: %v", err)
			}
			if err := os.WriteFile(filepath.Join(outDir, "sample_proof_envelope.bin"), envelope, 0o644); err != nil {
				fatalf("write compat sample envelope: %v", err)
			}
		}
	}

	fmt.Printf("ZK fixture generated in %s\n", outDir)
	fmt.Println("Set verifier env on VEIL node:")
	fmt.Printf("  VEIL_ZK_VERIFIER_ENABLED=true\n")
	fmt.Printf("  VEIL_ZK_VERIFIER_STRICT=true\n")
	fmt.Printf("  VEIL_ZK_REQUIRED_CIRCUIT_ID=%s\n", circuitID)
	fmt.Printf("  VEIL_ZK_GROTH16_VK_PATH=%s\n", vkPath)
}

func selectedCircuit(circuitID string) (frontend.Circuit, error) {
	switch strings.TrimSpace(circuitID) {
	case mconsts.ProofCircuitClearHashV1:
		return &zk.ClearHashCircuit{}, nil
	case mconsts.ProofCircuitShieldedLedgerV1:
		return &zk.ShieldedLedgerCircuitV1{}, nil
	default:
		return nil, fmt.Errorf("unsupported circuit id: %q", circuitID)
	}
}

func artifactNames(circuitID string) (pkName string, vkName string, samplePrefix string, writeCompat bool, err error) {
	switch strings.TrimSpace(circuitID) {
	case mconsts.ProofCircuitClearHashV1:
		return "groth16_clearhash_pk.bin", "groth16_clearhash_vk.bin", "sample_clearhash", true, nil
	case mconsts.ProofCircuitShieldedLedgerV1:
		return "groth16_shielded_ledger_pk.bin", "groth16_shielded_ledger_vk.bin", "sample_shielded_ledger", false, nil
	default:
		return "", "", "", false, fmt.Errorf("unsupported circuit id: %q", circuitID)
	}
}

func buildAssignment(circuitID string, preimage []byte, hashBytes []byte) (frontend.Circuit, error) {
	switch strings.TrimSpace(circuitID) {
	case mconsts.ProofCircuitClearHashV1:
		return zk.NewClearHashAssignment(preimage, hashBytes)
	case mconsts.ProofCircuitShieldedLedgerV1:
		return zk.NewShieldedLedgerAssignment(preimage, hashBytes)
	default:
		return nil, fmt.Errorf("unsupported circuit id: %q", circuitID)
	}
}

func buildSamplePreimage(circuitID string) ([]byte, []byte, error) {
	var marketID ids.ID
	if _, err := rand.Read(marketID[:]); err != nil {
		return nil, nil, fmt.Errorf("rand marketID: %w", err)
	}
	fills := make([]byte, actions.ExpectedFillsHashSize)
	if _, err := rand.Read(fills); err != nil {
		return nil, nil, fmt.Errorf("rand fills hash: %w", err)
	}
	windowID := uint64(1)
	clearPrice := uint64(1025)
	totalVolume := uint64(3200)
	var preimage []byte
	switch strings.TrimSpace(circuitID) {
	case mconsts.ProofCircuitClearHashV1:
		preimage = actions.BuildClearPublicInputsPreimage(marketID, windowID, clearPrice, totalVolume, fills)
		if len(preimage) != zk.ClearHashPreimageLen {
			return nil, nil, fmt.Errorf("invalid clear-hash sample preimage length: got=%d expected=%d", len(preimage), zk.ClearHashPreimageLen)
		}
	case mconsts.ProofCircuitShieldedLedgerV1:
		preimage = actions.BuildShieldedLedgerPublicInputsPreimage(marketID, windowID, clearPrice, totalVolume, fills)
		if len(preimage) != zk.ShieldedLedgerPreimageLen {
			return nil, nil, fmt.Errorf("invalid shielded-ledger sample preimage length: got=%d expected=%d", len(preimage), zk.ShieldedLedgerPreimageLen)
		}
	default:
		return nil, nil, fmt.Errorf("unsupported circuit id: %q", circuitID)
	}
	hash := sha256.Sum256(preimage)
	return preimage, hash[:], nil
}

func writeWithWriterTo(path string, obj io.WriterTo) error {
	buf := &bytes.Buffer{}
	if _, err := obj.WriteTo(buf); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
