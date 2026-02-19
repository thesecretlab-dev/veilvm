package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/veilvm/actions"
	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	vmclient "github.com/ava-labs/hypersdk/examples/veilvm/vm"
	"github.com/ava-labs/hypersdk/examples/veilvm/zk"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	groth16bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

const (
	defaultNodeURL      = "http://127.0.0.1:9660"
	defaultKeyHex       = "637404e6722a0e55a27fd82dcd29f3f0faa6f13d930f32f759e3b8412c4956aeee9d3919f004304c2d44dbc9121f6559fefb9b9c25daec749b0f18f605614461"
	defaultGasSafetyBps = 13_000
	defaultGasReserve   = 250_000
	defaultRefuelAmount = 5_000_000
	maxUint64           = ^uint64(0)
)

type benchConfig struct {
	NodeURL                          string
	ChainID                          string
	PrivateKeyHex                    string
	RefuelPrivateKeyHex              string
	ProofConfigPrivateKeyHex         string
	ProofConfigFallbackPrivateKeyHex string
	ProverAuthorityPrivateKeyHex     string
	StrictFeePreflight               bool
	OutputDir                        string
	ProofMode                        string
	ProofCircuitID                   string
	Groth16PKPath                    string
	Groth16CSCachePath               string
	ProofTamperMode                  string
	BatchSizes                       []int
	WindowsPerSize                   int
	BatchWindowMs                    int64
	ProofDeadlineMs                  int64
	ProofSubmitDelayMs               int64
	PrefundOnly                      bool
	TimeoutMinutes                   int
	GasSafetyBps                     uint64
	GasReserve                       uint64
	RefuelAmount                     uint64
}

type percentileStat struct {
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}

type batchResult struct {
	BatchSize      int                      `json:"batch_size"`
	WindowsRun     int                      `json:"windows_run"`
	MetricsCSV     string                   `json:"metrics_csv"`
	Summary        actions.ZKMetricsSummary `json:"summary"`
	BatchFreezeMs  percentileStat           `json:"batch_freeze_ms"`
	WitnessBuildMs percentileStat           `json:"witness_build_ms"`
	ProofGenMs     percentileStat           `json:"proof_generation_ms"`
	ProofVerifyMs  percentileStat           `json:"proof_verification_ms"`
	AcceptMs       percentileStat           `json:"block_accept_latency_ms"`
}

type benchReport struct {
	GeneratedAt string        `json:"generated_at"`
	Config      benchConfig   `json:"config"`
	Results     []batchResult `json:"results"`
}

type proofBuilder interface {
	Build(publicInputsHash []byte, preimage []byte, batchSize int, windowID uint64, fillsHash []byte) ([]byte, int64, error)
	Description() string
}

func normalizeProofTamperMode(raw string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case "", "none":
		return "", nil
	case "truncate":
		return "truncate", nil
	case "flip-last-byte":
		return "flip-last-byte", nil
	default:
		return "", fmt.Errorf("invalid PROOF_TAMPER_MODE=%q (expected none|truncate|flip-last-byte)", raw)
	}
}

func tamperProofEnvelope(proof []byte, mode string) ([]byte, error) {
	switch mode {
	case "":
		return proof, nil
	case "truncate":
		if len(proof) <= 1 {
			return nil, fmt.Errorf("cannot truncate proof envelope of length %d", len(proof))
		}
		cut := len(proof) / 2
		if cut < 12 {
			cut = len(proof) - 1
		}
		return append([]byte(nil), proof[:cut]...), nil
	case "flip-last-byte":
		if len(proof) == 0 {
			return nil, fmt.Errorf("cannot flip byte in empty proof envelope")
		}
		out := append([]byte(nil), proof...)
		out[len(out)-1] ^= 0x01
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported proof tamper mode %q", mode)
	}
}

type syntheticProofBuilder struct{}

func (syntheticProofBuilder) Build(_ []byte, _ []byte, batchSize int, windowID uint64, fillsHash []byte) ([]byte, int64, error) {
	start := time.Now()
	return buildProofBlob(batchSize, windowID, fillsHash), time.Since(start).Milliseconds(), nil
}

func (syntheticProofBuilder) Description() string { return "synthetic" }

type groth16ProofBuilder struct {
	ccs       constraint.ConstraintSystem
	pk        *groth16bn254.ProvingKey
	circuitID string
}

func (g *groth16ProofBuilder) Build(publicInputsHash []byte, preimage []byte, _ int, _ uint64, _ []byte) ([]byte, int64, error) {
	start := time.Now()
	var assignment frontend.Circuit
	var err error
	switch g.circuitID {
	case mconsts.ProofCircuitClearHashV1:
		assignment, err = zk.NewClearHashAssignment(preimage, publicInputsHash)
		if err != nil {
			return nil, 0, fmt.Errorf("build clear-hash assignment: %w", err)
		}
	case mconsts.ProofCircuitShieldedLedgerV1:
		assignment, err = zk.NewShieldedLedgerAssignment(preimage, publicInputsHash)
		if err != nil {
			return nil, 0, fmt.Errorf("build shielded-ledger assignment: %w", err)
		}
	default:
		return nil, 0, fmt.Errorf("unsupported circuit id %q", g.circuitID)
	}
	fullWitness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	if err != nil {
		return nil, 0, fmt.Errorf("build witness: %w", err)
	}
	publicWitness, err := fullWitness.Public()
	if err != nil {
		return nil, 0, fmt.Errorf("extract public witness: %w", err)
	}
	publicWitnessBytes, err := publicWitness.MarshalBinary()
	if err != nil {
		return nil, 0, fmt.Errorf("marshal public witness: %w", err)
	}
	proofAny, err := groth16.Prove(g.ccs, g.pk, fullWitness)
	if err != nil {
		return nil, 0, fmt.Errorf("prove: %w", err)
	}
	proofBN, ok := proofAny.(*groth16bn254.Proof)
	if !ok {
		return nil, 0, fmt.Errorf("unexpected proof type %T", proofAny)
	}
	var proofBuf bytes.Buffer
	if _, err := proofBN.WriteTo(&proofBuf); err != nil {
		return nil, 0, fmt.Errorf("serialize proof: %w", err)
	}
	proofBytes := proofBuf.Bytes()
	envelope, err := actions.BuildProofEnvelopeWithCircuit(
		mconsts.ProofTypeGroth16,
		g.circuitID,
		proofBytes,
		publicWitnessBytes,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("build proof envelope: %w", err)
	}
	return envelope, time.Since(start).Milliseconds(), nil
}

func (g *groth16ProofBuilder) Description() string {
	switch g.circuitID {
	case mconsts.ProofCircuitShieldedLedgerV1:
		return "groth16-shielded-ledger-v1"
	default:
		return "groth16-clearhash-v1"
	}
}

func newProofBuilder(cfg benchConfig) (proofBuilder, error) {
	switch cfg.ProofMode {
	case "synthetic":
		return syntheticProofBuilder{}, nil
	case "groth16":
		var circuit frontend.Circuit
		switch cfg.ProofCircuitID {
		case mconsts.ProofCircuitClearHashV1:
			circuit = &zk.ClearHashCircuit{}
		case mconsts.ProofCircuitShieldedLedgerV1:
			circuit = &zk.ShieldedLedgerCircuitV1{}
		default:
			return nil, fmt.Errorf("unsupported PROOF_CIRCUIT_ID=%q", cfg.ProofCircuitID)
		}
		ccs, err := loadOrCompileGroth16ConstraintSystem(cfg, circuit)
		if err != nil {
			return nil, err
		}
		pk, err := loadGroth16ProvingKey(cfg.Groth16PKPath)
		if err != nil {
			return nil, err
		}
		return &groth16ProofBuilder{ccs: ccs, pk: pk, circuitID: cfg.ProofCircuitID}, nil
	default:
		return nil, fmt.Errorf("unsupported proof mode %q", cfg.ProofMode)
	}
}

func loadGroth16ProvingKey(path string) (*groth16bn254.ProvingKey, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	pk := new(groth16bn254.ProvingKey)
	if _, err := pk.ReadFrom(f); err != nil {
		return nil, err
	}
	return pk, nil
}

func loadOrCompileGroth16ConstraintSystem(
	cfg benchConfig,
	circuit frontend.Circuit,
) (constraint.ConstraintSystem, error) {
	cachePath := strings.TrimSpace(cfg.Groth16CSCachePath)
	if cachePath != "" {
		ccs, err := loadGroth16ConstraintSystem(cachePath)
		if err == nil {
			fmt.Printf("Groth16 CS cache: hit (%s)\n", cachePath)
			return ccs, nil
		}
		fmt.Printf("Groth16 CS cache: miss (%s): %v\n", cachePath, err)
	}

	compileStart := time.Now()
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		return nil, fmt.Errorf("compile groth16 circuit: %w", err)
	}
	fmt.Printf("Groth16 CS compile complete in %s\n", time.Since(compileStart).Round(time.Millisecond))

	if cachePath != "" {
		if err := storeGroth16ConstraintSystem(cachePath, ccs); err != nil {
			fmt.Printf("Groth16 CS cache write skipped (%s): %v\n", cachePath, err)
		} else {
			fmt.Printf("Groth16 CS cache: saved (%s)\n", cachePath)
		}
	}

	return ccs, nil
}

func loadGroth16ConstraintSystem(path string) (constraint.ConstraintSystem, error) {
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

func storeGroth16ConstraintSystem(path string, ccs constraint.ConstraintSystem) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	if _, err := ccs.WriteTo(f); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	_ = os.Remove(path)
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func defaultGroth16CSCachePath(circuitID string) string {
	circuitSource := "zk/clearhash_circuit.go"
	switch circuitID {
	case mconsts.ProofCircuitShieldedLedgerV1:
		circuitSource = "zk/shielded_ledger_circuit.go"
	}
	fingerprint := sourceFingerprintHex("go.mod", "go.sum", circuitSource)
	safeCircuitID := sanitizePathComponent(circuitID)
	return filepath.Join(".", ".cache", "zkbench", fmt.Sprintf("groth16_%s_%s.ccs.bin", safeCircuitID, fingerprint))
}

func sourceFingerprintHex(paths ...string) string {
	h := sha256.New()
	wrote := false
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(b)
		_, _ = h.Write([]byte{0})
		wrote = true
	}
	if !wrote {
		return "nocache"
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:6])
}

func sanitizePathComponent(in string) string {
	if in == "" {
		return "unknown"
	}
	out := strings.Builder{}
	out.Grow(len(in))
	for _, r := range in {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			out.WriteRune(r)
		} else {
			out.WriteRune('_')
		}
	}
	return out.String()
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fatalf("config error: %v", err)
	}

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		fatalf("create output dir: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutMinutes)*time.Minute)
	defer cancel()

	report, err := runBench(ctx, cfg)
	if err != nil {
		fatalf("bench failed: %v", err)
	}

	summaryPath := filepath.Join(cfg.OutputDir, "summary.md")
	if err := writeSummaryMarkdown(summaryPath, report); err != nil {
		fatalf("write summary: %v", err)
	}

	jsonPath := filepath.Join(cfg.OutputDir, "summary.json")
	if err := writeJSON(jsonPath, report); err != nil {
		fatalf("write summary json: %v", err)
	}

	fmt.Printf("Done.\n- Summary: %s\n- JSON: %s\n", summaryPath, jsonPath)
}

func loadConfig() (benchConfig, error) {
	cfg := benchConfig{
		NodeURL:                          envOr("NODE_URL", defaultNodeURL),
		ChainID:                          os.Getenv("CHAIN_ID"),
		PrivateKeyHex:                    envOr("PRIVATE_KEY", defaultKeyHex),
		RefuelPrivateKeyHex:              strings.TrimSpace(os.Getenv("REFUEL_PRIVATE_KEY")),
		ProofConfigPrivateKeyHex:         strings.TrimSpace(os.Getenv("PROOF_CONFIG_PRIVATE_KEY")),
		ProofConfigFallbackPrivateKeyHex: strings.TrimSpace(os.Getenv("PROOF_CONFIG_FALLBACK_PRIVATE_KEY")),
		ProverAuthorityPrivateKeyHex:     strings.TrimSpace(os.Getenv("PROVER_AUTHORITY_PRIVATE_KEY")),
		StrictFeePreflight:               envBool("STRICT_FEE_PREFLIGHT", false),
		OutputDir:                        envOr("OUTPUT_DIR", filepath.Join(".", "zkbench-out")),
		ProofMode:                        strings.ToLower(envOr("PROOF_MODE", "synthetic")),
		ProofCircuitID:                   strings.TrimSpace(envOr("PROOF_CIRCUIT_ID", mconsts.ProofCircuitClearHashV1)),
		Groth16PKPath:                    os.Getenv("GROTH16_PK_PATH"),
		Groth16CSCachePath:               strings.TrimSpace(os.Getenv("GROTH16_CCS_CACHE_PATH")),
		ProofTamperMode:                  strings.TrimSpace(os.Getenv("PROOF_TAMPER_MODE")),
		BatchWindowMs:                    int64(envInt("BATCH_WINDOW_MS", 5000)),
		ProofDeadlineMs:                  int64(envInt("PROOF_DEADLINE_MS", 10000)),
		ProofSubmitDelayMs:               int64(envInt("PROOF_SUBMIT_DELAY_MS", 0)),
		PrefundOnly:                      envBool("PREFUND_ONLY", false),
		WindowsPerSize:                   envInt("WINDOWS_PER_SIZE", 25),
		TimeoutMinutes:                   envInt("TIMEOUT_MINUTES", 60),
		GasSafetyBps:                     envUint64("GAS_SAFETY_BPS", defaultGasSafetyBps),
		GasReserve:                       envUint64("GAS_RESERVE", defaultGasReserve),
		RefuelAmount:                     envUint64("REFUEL_AMOUNT", defaultRefuelAmount),
	}
	if cfg.ChainID == "" {
		return benchConfig{}, fmt.Errorf("CHAIN_ID is required")
	}
	switch cfg.ProofMode {
	case "synthetic":
	case "groth16":
		if strings.TrimSpace(cfg.Groth16PKPath) == "" {
			return benchConfig{}, fmt.Errorf("GROTH16_PK_PATH is required when PROOF_MODE=groth16")
		}
		switch cfg.ProofCircuitID {
		case mconsts.ProofCircuitClearHashV1, mconsts.ProofCircuitShieldedLedgerV1:
		default:
			return benchConfig{}, fmt.Errorf(
				"invalid PROOF_CIRCUIT_ID=%q (expected %s|%s)",
				cfg.ProofCircuitID,
				mconsts.ProofCircuitClearHashV1,
				mconsts.ProofCircuitShieldedLedgerV1,
			)
		}
		if cfg.Groth16CSCachePath == "" {
			cfg.Groth16CSCachePath = defaultGroth16CSCachePath(cfg.ProofCircuitID)
		}
	default:
		return benchConfig{}, fmt.Errorf("invalid PROOF_MODE=%q (expected synthetic|groth16)", cfg.ProofMode)
	}
	tamperMode, err := normalizeProofTamperMode(cfg.ProofTamperMode)
	if err != nil {
		return benchConfig{}, err
	}
	cfg.ProofTamperMode = tamperMode
	if cfg.GasSafetyBps < 10_000 {
		cfg.GasSafetyBps = 10_000
	}
	if cfg.GasReserve == 0 {
		cfg.GasReserve = defaultGasReserve
	}
	if cfg.RefuelAmount == 0 {
		cfg.RefuelAmount = defaultRefuelAmount
	}
	if cfg.ProofDeadlineMs <= cfg.BatchWindowMs {
		cfg.ProofDeadlineMs = cfg.BatchWindowMs + 1_000
	}
	if cfg.ProofSubmitDelayMs < 0 {
		cfg.ProofSubmitDelayMs = 0
	}
	batchSizes, err := parseBatchSizes(envOr("BATCH_SIZES", "32,64,96,128"))
	if err != nil {
		return benchConfig{}, err
	}
	cfg.BatchSizes = batchSizes
	return cfg, nil
}

func runBench(ctx context.Context, cfg benchConfig) (*benchReport, error) {
	baseURL := fmt.Sprintf("%s/ext/bc/%s", strings.TrimRight(cfg.NodeURL, "/"), cfg.ChainID)
	coreClient := jsonrpc.NewJSONRPCClient(baseURL)
	indexerClient := indexer.NewClient(baseURL)
	veilClient := vmclient.NewJSONRPCClient(baseURL)

	_, addr, factory, err := loadSigner(cfg.PrivateKeyHex)
	if err != nil {
		return nil, err
	}
	proverAuthority := addr
	if strings.TrimSpace(cfg.ProverAuthorityPrivateKeyHex) != "" {
		_, proverAuthorityAddr, _, err := loadSigner(cfg.ProverAuthorityPrivateKeyHex)
		if err != nil {
			return nil, fmt.Errorf("invalid PROVER_AUTHORITY_PRIVATE_KEY: %w", err)
		}
		proverAuthority = proverAuthorityAddr
	}
	proofConfigSigner := factory
	if strings.TrimSpace(cfg.ProofConfigPrivateKeyHex) != "" {
		_, _, proofConfigFactory, err := loadSigner(cfg.ProofConfigPrivateKeyHex)
		if err != nil {
			return nil, fmt.Errorf("invalid PROOF_CONFIG_PRIVATE_KEY: %w", err)
		}
		proofConfigSigner = proofConfigFactory
	}
	var refuelAddr codec.Address
	var refuelFactory chain.AuthFactory
	hasRefuel := strings.TrimSpace(cfg.RefuelPrivateKeyHex) != ""
	if hasRefuel {
		if _, refuelAddr, refuelFactory, err = loadSigner(cfg.RefuelPrivateKeyHex); err != nil {
			return nil, fmt.Errorf("invalid REFUEL_PRIVATE_KEY: %w", err)
		}
	}

	_, _, chainIDParsed, err := coreClient.Network(ctx)
	if err != nil {
		return nil, fmt.Errorf("network: %w", err)
	}

	currentMaxFee := func() (uint64, error) {
		unitPrices, err := coreClient.UnitPrices(ctx, false)
		if err != nil {
			return 0, err
		}
		return estimateMaxFee(unitPrices), nil
	}

	submitSignedAction := func(name string, signer chain.AuthFactory, action chain.Action) (string, uint64, error) {
		maxFee, err := currentMaxFee()
		if err != nil {
			return "", 0, fmt.Errorf("%s unitprices: %w", name, err)
		}
		_, _, ts, err := coreClient.Accepted(ctx)
		if err != nil {
			return "", maxFee, fmt.Errorf("%s accepted: %w", name, err)
		}
		// Some local profiles can report a stale accepted timestamp when only empty
		// blocks are produced. Anchor expiry to at least wall-clock time so txs
		// don't get rejected as immediately expired.
		anchor := ts
		nowMs := time.Now().UnixMilli()
		if nowMs > anchor {
			anchor = nowMs
		}
		expiry := anchor + 60_000
		expiry = (expiry / 1000) * 1000
		if expiry <= anchor {
			expiry = ((anchor / 1000) + 61) * 1000
		}
		base := chain.Base{
			Timestamp: expiry,
			ChainID:   chainIDParsed,
			MaxFee:    maxFee,
		}
		txBytes, err := chain.SignRawActionBytesTx(
			base,
			[][]byte{action.Bytes()},
			signer,
		)
		if err != nil {
			return "", maxFee, fmt.Errorf("%s sign: %w", name, err)
		}
		txID, err := coreClient.SubmitTx(ctx, txBytes)
		if err != nil {
			return "", maxFee, fmt.Errorf("%s submit: %w", name, err)
		}
		if err := waitForHeightAdvance(ctx, coreClient); err != nil {
			return "", maxFee, fmt.Errorf("%s wait: %w", name, err)
		}
		if err := waitForTxSuccess(ctx, indexerClient, txID, name); err != nil {
			return "", maxFee, err
		}
		return txID.String(), maxFee, nil
	}

	ensureBenchBalance := func(reason string, required uint64) error {
		balance, err := veilClient.Balance(ctx, addr)
		if err != nil {
			return fmt.Errorf("%s bench balance: %w", reason, err)
		}
		if balance >= required {
			return nil
		}

		deficit := required - balance
		if !hasRefuel {
			return fmt.Errorf(
				"%s insufficient bench fee balance: have=%d need=%d deficit=%d (addr=%s). set REFUEL_PRIVATE_KEY or fund this address",
				reason, balance, required, deficit, addr,
			)
		}
		if refuelAddr == addr {
			return fmt.Errorf("%s REFUEL_PRIVATE_KEY must differ from PRIVATE_KEY when auto-refuel is needed", reason)
		}

		refuelMaxFee, err := currentMaxFee()
		if err != nil {
			return fmt.Errorf("%s unitprices for refuel: %w", reason, err)
		}
		topUpAmount := cfg.RefuelAmount
		minTopUp := saturatingAdd(deficit, refuelMaxFee)
		if topUpAmount < minTopUp {
			topUpAmount = minTopUp
		}

		refuelBalance, err := veilClient.Balance(ctx, refuelAddr)
		if err != nil {
			return fmt.Errorf("%s refuel balance: %w", reason, err)
		}
		refuelRequired := saturatingAdd(topUpAmount, refuelMaxFee)
		if refuelBalance < refuelRequired {
			return fmt.Errorf(
				"%s refuel wallet insufficient: have=%d need=%d (transfer=%d + fee=%d) addr=%s",
				reason, refuelBalance, refuelRequired, topUpAmount, refuelMaxFee, refuelAddr,
			)
		}

		fmt.Printf(
			"Auto-refuel (%s): bench=%s have=%d need=%d transfer=%d from=%s\n",
			reason, addr, balance, required, topUpAmount, refuelAddr,
		)
		if _, _, err := submitSignedAction("refuel_transfer", refuelFactory, &actions.Transfer{
			To:    addr,
			Value: topUpAmount,
			Memo:  []byte("zkbench-refuel"),
		}); err != nil {
			return fmt.Errorf("%s refuel transfer: %w", reason, err)
		}

		updatedBalance, err := veilClient.Balance(ctx, addr)
		if err != nil {
			return fmt.Errorf("%s bench balance after refuel: %w", reason, err)
		}
		if updatedBalance < required {
			return fmt.Errorf(
				"%s refuel completed but bench balance still low: have=%d need=%d",
				reason, updatedBalance, required,
			)
		}
		return nil
	}

	submitAction := func(name string, action chain.Action) (string, error) {
		maxFee, err := currentMaxFee()
		if err != nil {
			return "", fmt.Errorf("%s unitprices: %w", name, err)
		}
		requiredBeforeSubmit := saturatingAdd(maxFee, cfg.GasReserve)
		if hasRefuel || cfg.StrictFeePreflight {
			if err := ensureBenchBalance(name+" preflight", requiredBeforeSubmit); err != nil {
				return "", err
			}
		}

		txID, _, err := submitSignedAction(name, factory, action)
		if err == nil {
			return txID, nil
		}
		if !isInsufficientFeeError(err) {
			return "", err
		}
		if !hasRefuel {
			return "", fmt.Errorf("%s insufficient fee balance (set REFUEL_PRIVATE_KEY or fund %s): %w", name, addr, err)
		}

		retryRequired := saturatingAdd(requiredBeforeSubmit, maxFee)
		if err := ensureBenchBalance(name+" retry", retryRequired); err != nil {
			return "", err
		}
		txID, _, err = submitSignedAction(name, factory, action)
		if err != nil {
			return "", err
		}
		return txID, nil
	}

	initialMaxFee, err := currentMaxFee()
	if err != nil {
		return nil, fmt.Errorf("initial unitprices: %w", err)
	}
	initialBalance, err := veilClient.Balance(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("initial bench balance: %w", err)
	}
	plannedTxs := plannedActionCount(cfg)
	requiredBudget := estimateRequiredBudget(plannedTxs, initialMaxFee, cfg.GasSafetyBps, cfg.GasReserve)
	fmt.Printf(
		"Fee preflight: planned_txs=%d max_fee=%d safety_bps=%d reserve=%d required=%d balance=%d\n",
		plannedTxs, initialMaxFee, cfg.GasSafetyBps, cfg.GasReserve, requiredBudget, initialBalance,
	)
	if err := ensureBenchBalance("startup preflight", requiredBudget); err != nil {
		if hasRefuel || cfg.StrictFeePreflight {
			return nil, err
		}
		fmt.Printf("Fee preflight warning: %v\n", err)
		fmt.Printf("Continuing without strict startup preflight; runtime fee checks still apply.\n")
	}
	if cfg.PrefundOnly {
		fmt.Printf("Prefund-only mode: startup preflight satisfied; exiting before benchmark txs.\n")
		return &benchReport{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Config:      cfg,
			Results:     []batchResult{},
		}, nil
	}

	proofs, err := newProofBuilder(cfg)
	if err != nil {
		return nil, err
	}

	if err := setProofConfig(ctx, submitSignedAction, proofConfigSigner, proverAuthority, cfg); err != nil {
		if !isUnauthorizedError(err) {
			return nil, err
		}

		fallbackKeyHex := strings.TrimSpace(cfg.ProofConfigFallbackPrivateKeyHex)
		if fallbackKeyHex == "" &&
			strings.TrimSpace(cfg.ProofConfigPrivateKeyHex) == "" &&
			!strings.EqualFold(strings.TrimSpace(cfg.PrivateKeyHex), defaultKeyHex) {
			// Common local setup: bench key rotates but genesis key retains proof-config authority.
			fallbackKeyHex = defaultKeyHex
		}
		if fallbackKeyHex == "" {
			return nil, err
		}
		_, _, fallbackSigner, loadErr := loadSigner(fallbackKeyHex)
		if loadErr != nil {
			return nil, fmt.Errorf("invalid PROOF_CONFIG_FALLBACK_PRIVATE_KEY: %w", loadErr)
		}
		fmt.Printf("set_proof_config unauthorized with active signer; retrying with fallback proof-config signer.\n")
		if retryErr := setProofConfig(ctx, submitSignedAction, fallbackSigner, proverAuthority, cfg); retryErr != nil {
			return nil, retryErr
		}
	}

	report := &benchReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Config:      cfg,
		Results:     make([]batchResult, 0, len(cfg.BatchSizes)),
	}
	fmt.Printf("Proof mode: %s\n", proofs.Description())
	if cfg.ProofTamperMode != "" {
		fmt.Printf("Proof tamper mode: %s\n", cfg.ProofTamperMode)
	}
	if cfg.ProverAuthorityPrivateKeyHex != "" {
		fmt.Printf("Prover authority override: %s\n", proverAuthority)
	}

	for _, batchSize := range cfg.BatchSizes {
		fmt.Printf("Running batch size %d (%d windows)\n", batchSize, cfg.WindowsPerSize)
		if err := veilClient.ResetZKMetrics(ctx); err != nil {
			return nil, fmt.Errorf("reset metrics batch=%d: %w", batchSize, err)
		}
		marketID := deriveMarketID(batchSize)
		if _, err := submitAction("create_market", &actions.CreateMarket{
			MarketID:       marketID,
			Question:       []byte(fmt.Sprintf("zkbench-%d", batchSize)),
			Outcomes:       2,
			ResolutionTime: time.Now().Unix() + 86_400,
			CreatorBond:    1,
		}); err != nil {
			return nil, err
		}

		for i := 1; i <= cfg.WindowsPerSize; i++ {
			windowID := uint64(i)
			envelope := buildEnvelope(batchSize, windowID)
			commitment := sha256.Sum256(envelope)
			if _, err := submitAction("commit_order", &actions.CommitOrder{
				MarketID:   marketID,
				WindowID:   windowID,
				Envelope:   envelope,
				Commitment: commitment[:],
			}); err != nil {
				return nil, err
			}

			reveal := buildRevealShare(batchSize, windowID)
			if _, err := submitAction("reveal_batch", &actions.RevealBatch{
				MarketID:        marketID,
				WindowID:        windowID,
				DecryptionShare: reveal,
				ValidatorIndex:  1,
			}); err != nil {
				return nil, err
			}

			witnessStart := time.Now()
			fillsHash := buildFillsHash(batchSize, windowID, envelope, reveal)
			witnessMs := time.Since(witnessStart).Milliseconds()

			_, _, acceptedTs, err := coreClient.Accepted(ctx)
			if err != nil {
				return nil, fmt.Errorf("accepted before proof: %w", err)
			}
			windowClose := acceptedTs - (acceptedTs % cfg.BatchWindowMs)
			if windowClose <= 0 {
				windowClose = acceptedTs
			}
			clearPrice := uint64(1000 + (i % 97))
			totalVolume := uint64(batchSize * 100)
			var publicInputsHash [32]byte
			var preimage []byte
			switch cfg.ProofCircuitID {
			case mconsts.ProofCircuitShieldedLedgerV1:
				publicInputsHash = actions.ComputeShieldedLedgerPublicInputsHash(
					marketID,
					windowID,
					clearPrice,
					totalVolume,
					fillsHash,
				)
				preimage = actions.BuildShieldedLedgerPublicInputsPreimage(
					marketID,
					windowID,
					clearPrice,
					totalVolume,
					fillsHash,
				)
			default:
				publicInputsHash = actions.ComputeClearPublicInputsHash(
					marketID,
					windowID,
					clearPrice,
					totalVolume,
					fillsHash,
				)
				preimage = actions.BuildClearPublicInputsPreimage(
					marketID,
					windowID,
					clearPrice,
					totalVolume,
					fillsHash,
				)
			}
			proof, proofMs, err := proofs.Build(publicInputsHash[:], preimage, batchSize, windowID, fillsHash)
			if err != nil {
				return nil, fmt.Errorf("build proof (%s): %w", proofs.Description(), err)
			}
			if cfg.ProofTamperMode != "" {
				proof, err = tamperProofEnvelope(proof, cfg.ProofTamperMode)
				if err != nil {
					return nil, fmt.Errorf("tamper proof (%s): %w", cfg.ProofTamperMode, err)
				}
			}
			if cfg.ProofSubmitDelayMs > 0 {
				delay := time.NewTimer(time.Duration(cfg.ProofSubmitDelayMs) * time.Millisecond)
				select {
				case <-ctx.Done():
					delay.Stop()
					return nil, fmt.Errorf("submit proof delay interrupted: %w", ctx.Err())
				case <-delay.C:
				}
			}

			if _, err := submitAction("submit_batch_proof", &actions.SubmitBatchProof{
				MarketID:         marketID,
				WindowID:         windowID,
				WindowCloseAtMs:  windowClose,
				ProofType:        mconsts.ProofTypeGroth16,
				PublicInputsHash: publicInputsHash[:],
				FillsHash:        fillsHash,
				Proof:            proof,
			}); err != nil {
				return nil, err
			}

			if err := veilClient.RecordZKProverMetrics(
				ctx,
				marketID,
				windowID,
				uint32(batchSize),
				witnessMs,
				proofMs,
			); err != nil {
				return nil, fmt.Errorf("record prover metrics: %w", err)
			}

			if _, err := submitAction("clear_batch", &actions.ClearBatch{
				MarketID:    marketID,
				WindowID:    windowID,
				ClearPrice:  clearPrice,
				TotalVolume: totalVolume,
				FillsHash:   fillsHash,
			}); err != nil {
				return nil, err
			}

			if i%10 == 0 || i == cfg.WindowsPerSize {
				fmt.Printf("  batch=%d window=%d/%d\n", batchSize, i, cfg.WindowsPerSize)
			}
		}

		snap, err := veilClient.ZKMetrics(ctx, 0, true)
		if err != nil {
			return nil, fmt.Errorf("fetch metrics batch=%d: %w", batchSize, err)
		}
		csvPath := filepath.Join(cfg.OutputDir, fmt.Sprintf("metrics_batch_%d.csv", batchSize))
		if err := writeMetricsCSV(csvPath, snap.Windows); err != nil {
			return nil, fmt.Errorf("write csv batch=%d: %w", batchSize, err)
		}

		result := batchResult{
			BatchSize:      batchSize,
			WindowsRun:     len(snap.Windows),
			MetricsCSV:     csvPath,
			Summary:        snap.Summary,
			BatchFreezeMs:  percentileFromWindows(snap.Windows, func(w actions.ZKWindowMetrics) float64 { return float64(w.BatchFreezeMs) }),
			WitnessBuildMs: percentileFromWindows(snap.Windows, func(w actions.ZKWindowMetrics) float64 { return float64(w.WitnessBuildMs) }),
			ProofGenMs:     percentileFromWindows(snap.Windows, func(w actions.ZKWindowMetrics) float64 { return float64(w.ProofGenerationMs) }),
			ProofVerifyMs:  percentileFromWindows(snap.Windows, func(w actions.ZKWindowMetrics) float64 { return float64(w.ProofVerificationMs) }),
			AcceptMs:       percentileFromWindows(snap.Windows, func(w actions.ZKWindowMetrics) float64 { return float64(w.BlockAcceptLatencyMs) }),
		}
		report.Results = append(report.Results, result)
	}

	return report, nil
}

func setProofConfig(
	ctx context.Context,
	submitSignedAction func(string, chain.AuthFactory, chain.Action) (string, uint64, error),
	signer chain.AuthFactory,
	proverAuthority codec.Address,
	cfg benchConfig,
) error {
	_, _, err := submitSignedAction("set_proof_config", signer, &actions.SetProofConfig{
		RequireProof:      true,
		RequiredProofType: mconsts.ProofTypeGroth16,
		BatchWindowMs:     cfg.BatchWindowMs,
		ProofDeadlineMs:   cfg.ProofDeadlineMs,
		ProverAuthority:   proverAuthority,
	})
	if err != nil {
		return fmt.Errorf("set proof config: %w", err)
	}
	return nil
}

func waitForHeightAdvance(ctx context.Context, core *jsonrpc.JSONRPCClient) error {
	_, h0, _, err := core.Accepted(ctx)
	if err != nil {
		return err
	}
	for i := 0; i < 60; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		time.Sleep(500 * time.Millisecond)
		_, h1, _, err := core.Accepted(ctx)
		if err != nil {
			return err
		}
		if h1 > h0 {
			return nil
		}
	}
	return fmt.Errorf("timeout waiting for height advance")
}

func waitForTxSuccess(
	ctx context.Context,
	client *indexer.Client,
	txID ids.ID,
	name string,
) error {
	for i := 0; i < 120; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s tx result wait: %w", name, ctx.Err())
		default:
		}
		resp, found, err := client.GetTxResults(ctx, txID)
		if err != nil {
			return fmt.Errorf("%s tx result lookup: %w", name, err)
		}
		if !found {
			time.Sleep(250 * time.Millisecond)
			continue
		}
		if !resp.Result.Success {
			return fmt.Errorf("%s execution failed: %s", name, string(resp.Result.Error))
		}
		return nil
	}
	return fmt.Errorf("%s tx result timeout: %s", name, txID)
}

func loadSigner(pkHex string) (ed25519.PrivateKey, codec.Address, chain.AuthFactory, error) {
	b, err := hex.DecodeString(pkHex)
	if err != nil {
		return ed25519.EmptyPrivateKey, codec.EmptyAddress, nil, fmt.Errorf("invalid PRIVATE_KEY: %w", err)
	}
	priv := ed25519.PrivateKey(b)
	addr := auth.NewED25519Address(priv.PublicKey())
	factory := auth.NewED25519Factory(priv)
	return priv, addr, factory, nil
}

func deriveMarketID(batchSize int) ids.ID {
	var id ids.ID
	sum := sha256.Sum256([]byte(fmt.Sprintf("zkbench:%d:%d", batchSize, time.Now().UnixNano())))
	copy(id[:], sum[:])
	return id
}

func buildEnvelope(batchSize int, windowID uint64) []byte {
	n := batchSize * 24
	if n < 96 {
		n = 96
	}
	if n > actions.MaxEnvelopeSize {
		n = actions.MaxEnvelopeSize
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = byte((int(windowID) + i*17 + batchSize) & 0xff)
	}
	return out
}

func buildRevealShare(batchSize int, windowID uint64) []byte {
	n := 96 + batchSize
	if n > actions.MaxDecryptionShareSize {
		n = actions.MaxDecryptionShareSize
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = byte((i*11 + int(windowID) + batchSize*3) & 0xff)
	}
	return out
}

func buildFillsHash(batchSize int, windowID uint64, envelope []byte, reveal []byte) []byte {
	h := sha256.New()
	loops := 50 + batchSize*5
	for i := 0; i < loops; i++ {
		h.Write(envelope)
		h.Write(reveal)
		var b [8]byte
		v := uint64(i) ^ windowID ^ uint64(batchSize)
		for j := 0; j < 8; j++ {
			b[j] = byte(v >> (8 * j))
		}
		h.Write(b[:])
	}
	return h.Sum(nil)
}

func buildProofBlob(batchSize int, windowID uint64, fillsHash []byte) []byte {
	n := 2048 + batchSize*64
	if n > MaxProofBytesSize {
		n = MaxProofBytesSize
	}
	out := make([]byte, n)
	seed := sha256.Sum256(append(fillsHash, byte(batchSize), byte(windowID)))
	copy(out, seed[:])
	for i := 32; i < len(out); i++ {
		out[i] = out[i-32] ^ byte((i*13+batchSize)&0xff)
	}
	// Simulate proving work proportional to batch size.
	acc := seed[:]
	for i := 0; i < 80+batchSize*8; i++ {
		tmp := sha256.Sum256(acc)
		acc = tmp[:]
	}
	copy(out[len(out)-32:], acc)
	return out
}

const MaxProofBytesSize = 131072

func writeMetricsCSV(path string, windows []actions.ZKWindowMetrics) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{
		"market_id",
		"window_id",
		"batch_size_hint",
		"commit_count",
		"reveal_count",
		"first_commit_at_ms",
		"window_close_at_ms",
		"proof_submitted_at_ms",
		"clear_accepted_at_ms",
		"batch_freeze_ms",
		"witness_build_ms",
		"proof_generation_ms",
		"proof_verification_ms",
		"proof_submit_latency_ms",
		"block_accept_latency_ms",
		"commit_exec_us",
		"reveal_exec_us",
		"proof_submit_exec_us",
		"clear_exec_us",
		"missed_deadline",
		"rejected",
		"last_error",
	}
	if err := w.Write(header); err != nil {
		return err
	}
	for _, row := range windows {
		record := []string{
			row.MarketID,
			strconv.FormatUint(row.WindowID, 10),
			strconv.FormatUint(uint64(row.BatchSizeHint), 10),
			strconv.FormatUint(uint64(row.CommitCount), 10),
			strconv.FormatUint(uint64(row.RevealCount), 10),
			strconv.FormatInt(row.FirstCommitAtMs, 10),
			strconv.FormatInt(row.WindowCloseAtMs, 10),
			strconv.FormatInt(row.ProofSubmittedAtMs, 10),
			strconv.FormatInt(row.ClearAcceptedAtMs, 10),
			strconv.FormatInt(row.BatchFreezeMs, 10),
			strconv.FormatInt(row.WitnessBuildMs, 10),
			strconv.FormatInt(row.ProofGenerationMs, 10),
			strconv.FormatInt(row.ProofVerificationMs, 10),
			strconv.FormatInt(row.ProofSubmitLatencyMs, 10),
			strconv.FormatInt(row.BlockAcceptLatencyMs, 10),
			strconv.FormatUint(row.CommitExecUs, 10),
			strconv.FormatUint(row.RevealExecUs, 10),
			strconv.FormatUint(row.ProofSubmitExecUs, 10),
			strconv.FormatUint(row.ClearExecUs, 10),
			strconv.FormatBool(row.MissedDeadline),
			strconv.FormatBool(row.Rejected),
			row.LastError,
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	return nil
}

func percentileFromWindows(windows []actions.ZKWindowMetrics, selectFn func(actions.ZKWindowMetrics) float64) percentileStat {
	values := make([]float64, 0, len(windows))
	for _, w := range windows {
		v := selectFn(w)
		if v >= 0 {
			values = append(values, v)
		}
	}
	return percentileStat{
		P50: percentile(values, 0.50),
		P95: percentile(values, 0.95),
		P99: percentile(values, 0.99),
	}
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	if p <= 0 {
		return cp[0]
	}
	if p >= 1 {
		return cp[len(cp)-1]
	}
	idx := int(math.Ceil(float64(len(cp))*p)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}

func writeSummaryMarkdown(path string, report *benchReport) error {
	var b strings.Builder
	b.WriteString("# VEIL ZK Bench Summary\n\n")
	b.WriteString(fmt.Sprintf("Proof mode: `%s`\n\n", report.Config.ProofMode))
	if report.Config.ProofMode == "synthetic" {
		b.WriteString("This run benchmarks proof-gated VEIL batch flow with synthetic order-equivalent payload sizes.\n\n")
	} else {
		b.WriteString("This run benchmarks proof-gated VEIL batch flow with cryptographic Groth16 payloads.\n\n")
	}
	b.WriteString("| Batch Size | Windows | Freeze p95 (ms) | Witness p95 (ms) | Prove p95 (ms) | Verify p95 (ms) | Accept p95 (ms) | CSV |\n")
	b.WriteString("|---:|---:|---:|---:|---:|---:|---:|---|\n")
	for _, r := range report.Results {
		b.WriteString(fmt.Sprintf(
			"| %d | %d | %.2f | %.2f | %.2f | %.2f | %.2f | `%s` |\n",
			r.BatchSize,
			r.WindowsRun,
			r.BatchFreezeMs.P95,
			r.WitnessBuildMs.P95,
			r.ProofGenMs.P95,
			r.ProofVerifyMs.P95,
			r.AcceptMs.P95,
			r.MetricsCSV,
		))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func writeJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func parseBatchSizes(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid batch size %q", p)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid batch sizes")
	}
	return out, nil
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func envInt(name string, fallback int) int {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envUint64(name string, fallback uint64) uint64 {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func envBool(name string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "t", "yes", "y", "on":
		return true
	case "0", "false", "f", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func estimateMaxFee(unitPrices fees.Dimensions) uint64 {
	maxFee := uint64(0)
	for i := 0; i < len(unitPrices); i++ {
		if unitPrices[i] > maxUint64/10_000 {
			return maxUint64
		}
		dimFee := unitPrices[i] * 10_000
		if maxFee > maxUint64-dimFee {
			maxFee = maxUint64
			break
		}
		maxFee += dimFee
	}
	if maxFee < 100_000 {
		maxFee = 100_000
	}
	return maxFee
}

func plannedActionCount(cfg benchConfig) int {
	// set_proof_config + (create_market + 4 actions per window) per batch size.
	perBatch := 1 + (4 * cfg.WindowsPerSize)
	return 1 + (len(cfg.BatchSizes) * perBatch)
}

func estimateRequiredBudget(txCount int, maxFee uint64, gasSafetyBps uint64, gasReserve uint64) uint64 {
	if txCount <= 0 {
		return gasReserve
	}

	v := new(big.Int).SetUint64(maxFee)
	v.Mul(v, new(big.Int).SetUint64(uint64(txCount)))
	v.Mul(v, new(big.Int).SetUint64(gasSafetyBps))
	v.Div(v, big.NewInt(10_000))
	v.Add(v, new(big.Int).SetUint64(gasReserve))
	if !v.IsUint64() {
		return maxUint64
	}
	return v.Uint64()
}

func saturatingAdd(a, b uint64) uint64 {
	if a > maxUint64-b {
		return maxUint64
	}
	return a + b
}

func isInsufficientFeeError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "insufficient balance") && strings.Contains(s, "fee")
}

func isUnauthorizedError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unauthorized")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
