package genesis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	hgenesis "github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/state"

	smath "github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
)

const (
	bipsDenominator uint64 = 10_000
)

var (
	_ hgenesis.Genesis               = (*Genesis)(nil)
	_ hgenesis.GenesisAndRuleFactory = (*Factory)(nil)
)

type Tokenomics struct {
	TotalSupply uint64 `json:"totalSupply"`

	Governance    codec.Address `json:"governance"`
	Operations    codec.Address `json:"operations"`
	MintAuthority codec.Address `json:"mintAuthority"`

	COLVaultLocked uint64 `json:"colVaultLocked"`
	COLVaultLive   uint64 `json:"colVaultLive"`

	MaxReleaseBips      uint16 `json:"maxReleaseBips"`
	ReleaseEpochSeconds int64  `json:"releaseEpochSeconds"`

	FeeRouterMSRBBips uint16 `json:"feeRouterMsrbBips"`
	FeeRouterCOLBips  uint16 `json:"feeRouterColBips"`
	FeeRouterOpsBips  uint16 `json:"feeRouterOpsBips"`

	VAIDebtCeiling      uint64 `json:"vaiDebtCeiling"`
	VAIEpochMintLimit   uint64 `json:"vaiEpochMintLimit"`
	VAIMintEpochSeconds int64  `json:"vaiMintEpochSeconds"`

	BackingFloorBips uint32 `json:"backingFloorBips"`

	VEILLtvBips   uint16 `json:"veilLtvBips"`
	WVEILLtvBips  uint16 `json:"wVeilLtvBips"`
	WSVEILLtvBips uint16 `json:"wsVeilLtvBips"`

	VEILHaircutBips   uint16 `json:"veilHaircutBips"`
	WVEILHaircutBips  uint16 `json:"wVeilHaircutBips"`
	WSVEILHaircutBips uint16 `json:"wsVeilHaircutBips"`

	ExogenousReserveInit uint64 `json:"exogenousReserveInit"`
	VAIBufferInit        uint64 `json:"vaiBufferInit"`

	RequireBatchProof bool          `json:"requireBatchProof"`
	RequiredProofType uint8         `json:"requiredProofType"`
	BatchWindowMs     int64         `json:"batchWindowMs"`
	ProofDeadlineMs   int64         `json:"proofDeadlineMs"`
	ProverAuthority   codec.Address `json:"proverAuthority"`
}

type Genesis struct {
	StateBranchFactor merkledb.BranchFactor        `json:"stateBranchFactor"`
	CustomAllocation  []*hgenesis.CustomAllocation `json:"customAllocation"`
	Rules             *hgenesis.Rules              `json:"initialRules"`
	Tokenomics        *Tokenomics                  `json:"tokenomics,omitempty"`
}

func (g *Genesis) InitializeState(
	ctx context.Context,
	tracer trace.Tracer,
	mu state.Mutable,
	balanceHandler chain.BalanceHandler,
) error {
	base := &hgenesis.DefaultGenesis{
		StateBranchFactor: g.StateBranchFactor,
		CustomAllocation:  g.CustomAllocation,
		Rules:             g.Rules,
	}
	if err := base.InitializeState(ctx, tracer, mu, balanceHandler); err != nil {
		return err
	}
	if g.Tokenomics == nil {
		return nil
	}

	if err := validateTokenomics(g.CustomAllocation, g.Tokenomics); err != nil {
		return err
	}

	if err := storage.PutTreasuryConfig(ctx, mu, storage.TreasuryConfig{
		Governance:          g.Tokenomics.Governance,
		Operations:          g.Tokenomics.Operations,
		MaxReleaseBips:      g.Tokenomics.MaxReleaseBips,
		ReleaseEpochSeconds: g.Tokenomics.ReleaseEpochSeconds,
	}); err != nil {
		return err
	}
	if err := storage.PutTreasuryState(ctx, mu, storage.TreasuryState{
		Locked:          g.Tokenomics.COLVaultLocked,
		Live:            g.Tokenomics.COLVaultLive,
		Released:        0,
		LastReleaseUnix: 0,
	}); err != nil {
		return err
	}
	if err := storage.PutFeeRouterConfig(ctx, mu, storage.FeeRouterConfig{
		MSRBBips: g.Tokenomics.FeeRouterMSRBBips,
		COLBips:  g.Tokenomics.FeeRouterCOLBips,
		OpsBips:  g.Tokenomics.FeeRouterOpsBips,
	}); err != nil {
		return err
	}
	if err := storage.PutFeeRouterState(ctx, mu, storage.FeeRouterState{}); err != nil {
		return err
	}
	if err := storage.PutVAIConfig(ctx, mu, storage.VAIConfig{
		MintAuthority:    g.Tokenomics.MintAuthority,
		DebtCeiling:      g.Tokenomics.VAIDebtCeiling,
		EpochMintLimit:   g.Tokenomics.VAIEpochMintLimit,
		MintEpochSeconds: g.Tokenomics.VAIMintEpochSeconds,
	}); err != nil {
		return err
	}
	if err := storage.PutVAIState(ctx, mu, storage.VAIState{}); err != nil {
		return err
	}
	if err := storage.PutRiskConfig(ctx, mu, storage.RiskConfig{
		BackingFloorBips:  g.Tokenomics.BackingFloorBips,
		VEILLtvBips:       g.Tokenomics.VEILLtvBips,
		WVEILLtvBips:      g.Tokenomics.WVEILLtvBips,
		WSVEILLtvBips:     g.Tokenomics.WSVEILLtvBips,
		VEILHaircutBips:   g.Tokenomics.VEILHaircutBips,
		WVEILHaircutBips:  g.Tokenomics.WVEILHaircutBips,
		WSVEILHaircutBips: g.Tokenomics.WSVEILHaircutBips,
	}); err != nil {
		return err
	}
	if err := storage.PutReserveState(ctx, mu, storage.ReserveState{
		ExogenousReserve: g.Tokenomics.ExogenousReserveInit,
		VAIBuffer:        g.Tokenomics.VAIBufferInit,
	}); err != nil {
		return err
	}
	if err := storage.PutProofConfig(ctx, mu, storage.ProofConfig{
		RequireProof:      g.Tokenomics.RequireBatchProof,
		RequiredProofType: g.Tokenomics.RequiredProofType,
		BatchWindowMs:     g.Tokenomics.BatchWindowMs,
		ProofDeadlineMs:   g.Tokenomics.ProofDeadlineMs,
		ProverAuthority:   g.Tokenomics.ProverAuthority,
	}); err != nil {
		return err
	}
	return nil
}

func (g *Genesis) GetStateBranchFactor() merkledb.BranchFactor {
	return g.StateBranchFactor
}

type Factory struct{}

func (Factory) Load(
	genesisBytes []byte,
	_ []byte,
	networkID uint32,
	chainID ids.ID,
) (hgenesis.Genesis, chain.RuleFactory, error) {
	g := &Genesis{}
	if err := json.Unmarshal(genesisBytes, g); err != nil {
		return nil, nil, err
	}
	if g.StateBranchFactor == 0 {
		g.StateBranchFactor = merkledb.BranchFactor16
	}
	if g.Rules == nil {
		g.Rules = hgenesis.NewDefaultRules()
	}
	g.Rules.NetworkID = networkID
	g.Rules.ChainID = chainID
	if g.Tokenomics != nil {
		applyTokenomicsDefaults(g)
	}
	return g, &hgenesis.ImmutableRuleFactory{Rules: g.Rules}, nil
}

func applyTokenomicsDefaults(g *Genesis) {
	if g.Tokenomics == nil {
		return
	}
	var zero codec.Address
	if len(g.CustomAllocation) > 0 {
		defaultAddr := g.CustomAllocation[0].Address
		if g.Tokenomics.Governance == zero {
			g.Tokenomics.Governance = defaultAddr
		}
		if g.Tokenomics.Operations == zero {
			g.Tokenomics.Operations = defaultAddr
		}
		if g.Tokenomics.MintAuthority == zero {
			g.Tokenomics.MintAuthority = defaultAddr
		}
	}
	if g.Tokenomics.MaxReleaseBips == 0 {
		g.Tokenomics.MaxReleaseBips = 15 // 0.15%
	}
	if g.Tokenomics.ReleaseEpochSeconds == 0 {
		g.Tokenomics.ReleaseEpochSeconds = 86_400
	}
	if g.Tokenomics.FeeRouterMSRBBips == 0 &&
		g.Tokenomics.FeeRouterCOLBips == 0 &&
		g.Tokenomics.FeeRouterOpsBips == 0 {
		g.Tokenomics.FeeRouterMSRBBips = 7_000
		g.Tokenomics.FeeRouterCOLBips = 2_000
		g.Tokenomics.FeeRouterOpsBips = 1_000
	}
	if g.Tokenomics.VAIMintEpochSeconds == 0 {
		g.Tokenomics.VAIMintEpochSeconds = 3_600
	}
	if g.Tokenomics.VAIEpochMintLimit == 0 {
		g.Tokenomics.VAIEpochMintLimit = g.Tokenomics.VAIDebtCeiling
	}
	if g.Tokenomics.BackingFloorBips == 0 {
		g.Tokenomics.BackingFloorBips = 10_000
	}
	if g.Tokenomics.VEILLtvBips == 0 {
		g.Tokenomics.VEILLtvBips = 3_000
	}
	if g.Tokenomics.WVEILLtvBips == 0 {
		g.Tokenomics.WVEILLtvBips = 3_500
	}
	g.Tokenomics.WSVEILLtvBips = 0
	if g.Tokenomics.VEILHaircutBips == 0 {
		g.Tokenomics.VEILHaircutBips = 6_000
	}
	if g.Tokenomics.WVEILHaircutBips == 0 {
		g.Tokenomics.WVEILHaircutBips = 5_500
	}
	if g.Tokenomics.WSVEILHaircutBips == 0 {
		g.Tokenomics.WSVEILHaircutBips = 10_000
	}
	if g.Tokenomics.ExogenousReserveInit == 0 {
		g.Tokenomics.ExogenousReserveInit = g.Tokenomics.VAIDebtCeiling
	}
	if g.Tokenomics.RequiredProofType == 0 {
		g.Tokenomics.RequireBatchProof = true
		g.Tokenomics.RequiredProofType = mconsts.ProofTypeGroth16
	}
	if g.Tokenomics.BatchWindowMs == 0 {
		g.Tokenomics.BatchWindowMs = 5_000
	}
	if g.Tokenomics.ProofDeadlineMs == 0 {
		g.Tokenomics.ProofDeadlineMs = 10_000
	}
	if g.Tokenomics.ProverAuthority == zero {
		g.Tokenomics.ProverAuthority = g.Tokenomics.Governance
	}
}

func validateTokenomics(allocs []*hgenesis.CustomAllocation, t *Tokenomics) error {
	if t == nil {
		return nil
	}
	if t.ReleaseEpochSeconds <= 0 {
		return fmt.Errorf("%w: releaseEpochSeconds must be > 0", storage.ErrInvalidTokenomicsConfig)
	}
	if t.MaxReleaseBips == 0 || uint64(t.MaxReleaseBips) > bipsDenominator {
		return fmt.Errorf("%w: maxReleaseBips must be in [1,10000]", storage.ErrInvalidTokenomicsConfig)
	}
	if t.VAIMintEpochSeconds <= 0 {
		return fmt.Errorf("%w: vaiMintEpochSeconds must be > 0", storage.ErrInvalidTokenomicsConfig)
	}
	if t.VAIDebtCeiling == 0 {
		return fmt.Errorf("%w: vaiDebtCeiling must be > 0", storage.ErrInvalidTokenomicsConfig)
	}
	if t.VAIEpochMintLimit == 0 || t.VAIEpochMintLimit > t.VAIDebtCeiling {
		return fmt.Errorf("%w: vaiEpochMintLimit must be in [1, vaiDebtCeiling]", storage.ErrInvalidTokenomicsConfig)
	}
	if t.BackingFloorBips == 0 {
		return fmt.Errorf("%w: backingFloorBips must be > 0", storage.ErrInvalidRiskConfig)
	}
	if t.WSVEILLtvBips != 0 {
		return fmt.Errorf("%w: wsVeilLtvBips must be 0 in v1", storage.ErrInvalidRiskConfig)
	}
	if uint64(t.VEILLtvBips) > bipsDenominator ||
		uint64(t.WVEILLtvBips) > bipsDenominator ||
		uint64(t.WSVEILLtvBips) > bipsDenominator {
		return fmt.Errorf("%w: collateral LTV bips out of range", storage.ErrInvalidRiskConfig)
	}
	if uint64(t.VEILHaircutBips) > bipsDenominator ||
		uint64(t.WVEILHaircutBips) > bipsDenominator ||
		uint64(t.WSVEILHaircutBips) > bipsDenominator {
		return fmt.Errorf("%w: collateral haircut bips out of range", storage.ErrInvalidRiskConfig)
	}
	if t.RequiredProofType != mconsts.ProofTypeGroth16 && t.RequiredProofType != mconsts.ProofTypePlonk {
		return fmt.Errorf("%w: requiredProofType must be groth16 or plonk", storage.ErrInvalidProofConfig)
	}
	if t.BatchWindowMs <= 0 || t.ProofDeadlineMs <= 0 {
		return fmt.Errorf("%w: batchWindowMs and proofDeadlineMs must be > 0", storage.ErrInvalidProofConfig)
	}
	var zero codec.Address
	if t.ProverAuthority == zero {
		return fmt.Errorf("%w: proverAuthority must be set", storage.ErrInvalidProofConfig)
	}
	feeSum := uint64(t.FeeRouterMSRBBips) + uint64(t.FeeRouterCOLBips) + uint64(t.FeeRouterOpsBips)
	if feeSum != bipsDenominator {
		return fmt.Errorf("%w: fee router bips sum=%d", storage.ErrInvalidFeeRouterConfig, feeSum)
	}
	var allocSum uint64
	for _, alloc := range allocs {
		next, err := smath.Add(allocSum, alloc.Balance)
		if err != nil {
			return err
		}
		allocSum = next
	}
	total, err := smath.Add(allocSum, t.COLVaultLocked)
	if err != nil {
		return err
	}
	total, err = smath.Add(total, t.COLVaultLive)
	if err != nil {
		return err
	}
	if t.TotalSupply != 0 && total != t.TotalSupply {
		return fmt.Errorf(
			"%w: totalSupply=%d, computed=%d",
			storage.ErrInvalidTokenomicsConfig,
			t.TotalSupply,
			total,
		)
	}
	return nil
}
