package zk

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/examples/veilvm/actions"
	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	groth16bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

type Prover struct {
	ccs       constraint.ConstraintSystem
	pk        *groth16bn254.ProvingKey
	circuitID string
}

type ProveResult struct {
	Envelope         []byte
	PublicInputsHash []byte
	Preimage         []byte
	FillsHash        []byte
	CommitmentsHash  []byte
	NullifiersHash   []byte
	PrevStateRoot    []byte
	NextStateRoot    []byte
}

func LoadGroth16Prover(pkPath, ccsCachePath, circuitID string) (*Prover, error) {
	circuitID = strings.TrimSpace(circuitID)
	if circuitID == "" {
		circuitID = mconsts.ProofCircuitShieldedLedgerV1
	}
	var circuit frontend.Circuit
	switch circuitID {
	case mconsts.ProofCircuitClearHashV1:
		circuit = &ClearHashCircuit{}
	case mconsts.ProofCircuitShieldedLedgerV1:
		circuit = &ShieldedLedgerCircuitV1{}
	default:
		return nil, fmt.Errorf("unsupported circuit id %q", circuitID)
	}
	ccs, err := loadOrCompileCCS(ccsCachePath, circuit)
	if err != nil {
		return nil, err
	}
	pk, err := loadProvingKey(pkPath)
	if err != nil {
		return nil, err
	}
	return &Prover{ccs: ccs, pk: pk, circuitID: circuitID}, nil
}

func (p *Prover) ProveClear(
	marketID ids.ID,
	windowID uint64,
	clearPrice uint64,
	totalVolume uint64,
	fillsHash []byte,
) (*ProveResult, error) {
	if p == nil || p.pk == nil || p.ccs == nil {
		return nil, fmt.Errorf("prover not loaded")
	}
	if len(fillsHash) != actions.ExpectedFillsHashSize {
		return nil, fmt.Errorf("fills hash must be %d bytes", actions.ExpectedFillsHashSize)
	}
	var preimage []byte
	var digest [32]byte
	var extras actions.ShieldedLedgerPublicInputs
	switch p.circuitID {
	case mconsts.ProofCircuitClearHashV1:
		preimage = actions.BuildClearPublicInputsPreimage(marketID, windowID, clearPrice, totalVolume, fillsHash)
		digest = actions.ComputeClearPublicInputsHash(marketID, windowID, clearPrice, totalVolume, fillsHash)
	default:
		extras = actions.DerivedShieldedLedgerPublicInputs(marketID, windowID, clearPrice, totalVolume, fillsHash)
		preimage = actions.BuildShieldedLedgerPublicInputsPreimage(extras)
		digest = actions.ComputeShieldedLedgerPublicInputsHash(extras)
	}
	var assignment frontend.Circuit
	var err error
	switch p.circuitID {
	case mconsts.ProofCircuitClearHashV1:
		assignment, err = NewClearHashAssignment(preimage, digest[:])
	default:
		assignment, err = NewShieldedLedgerAssignment(preimage, digest[:])
	}
	if err != nil {
		return nil, err
	}
	fullWitness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	if err != nil {
		return nil, fmt.Errorf("witness: %w", err)
	}
	publicWitness, err := fullWitness.Public()
	if err != nil {
		return nil, fmt.Errorf("public witness: %w", err)
	}
	publicWitnessBytes, err := publicWitness.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal public witness: %w", err)
	}
	proofAny, err := groth16.Prove(p.ccs, p.pk, fullWitness)
	if err != nil {
		return nil, fmt.Errorf("prove: %w", err)
	}
	proofBN, ok := proofAny.(*groth16bn254.Proof)
	if !ok {
		return nil, fmt.Errorf("unexpected proof type %T", proofAny)
	}
	var proofBuf bytes.Buffer
	if _, err := proofBN.WriteTo(&proofBuf); err != nil {
		return nil, fmt.Errorf("serialize proof: %w", err)
	}
	envelope, err := actions.BuildProofEnvelopeWithCircuit(
		mconsts.ProofTypeGroth16,
		p.circuitID,
		proofBuf.Bytes(),
		publicWitnessBytes,
	)
	if err != nil {
		return nil, err
	}
	return &ProveResult{
		Envelope:         envelope,
		PublicInputsHash: digest[:],
		Preimage:         preimage,
		FillsHash:        append([]byte(nil), fillsHash...),
		CommitmentsHash:  append([]byte(nil), extras.CommitmentsHash...),
		NullifiersHash:   append([]byte(nil), extras.NullifiersHash...),
		PrevStateRoot:    append([]byte(nil), extras.PrevStateRoot...),
		NextStateRoot:    append([]byte(nil), extras.NextStateRoot...),
	}, nil
}

func loadProvingKey(path string) (*groth16bn254.ProvingKey, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open proving key: %w", err)
	}
	defer f.Close()
	pk := new(groth16bn254.ProvingKey)
	if _, err := pk.ReadFrom(f); err != nil {
		return nil, fmt.Errorf("read proving key: %w", err)
	}
	return pk, nil
}

func loadOrCompileCCS(cachePath string, circuit frontend.Circuit) (constraint.ConstraintSystem, error) {
	cachePath = strings.TrimSpace(cachePath)
	if cachePath != "" {
		if ccs, err := loadCCS(cachePath); err == nil {
			return ccs, nil
		}
	}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		return nil, fmt.Errorf("compile circuit: %w", err)
	}
	if cachePath != "" {
		_ = storeCCS(cachePath, ccs)
	}
	return ccs, nil
}

func loadCCS(path string) (constraint.ConstraintSystem, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ccs := groth16.NewCS(ecc.BN254)
	if _, err := ccs.ReadFrom(f); err != nil {
		return nil, err
	}
	return ccs, nil
}

func storeCCS(path string, ccs constraint.ConstraintSystem) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := ccs.WriteTo(f); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Remove(path)
	return os.Rename(tmp, path)
}


