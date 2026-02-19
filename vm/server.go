package vm

import (
	"math/big"
	"net/http"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/veilvm/actions"
	"github.com/ava-labs/hypersdk/examples/veilvm/consts"
	vgenesis "github.com/ava-labs/hypersdk/examples/veilvm/genesis"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
)

const JSONRPCEndpoint = "/veilapi"

var _ api.HandlerFactory[api.VM] = (*jsonRPCServerFactory)(nil)

type jsonRPCServerFactory struct{}

func (jsonRPCServerFactory) New(vm api.VM) (api.Handler, error) {
	handler, err := api.NewJSONRPCHandler(consts.Name, NewJSONRPCServer(vm))
	return api.Handler{
		Path:    JSONRPCEndpoint,
		Handler: handler,
	}, err
}

type JSONRPCServer struct {
	vm api.VM
}

func NewJSONRPCServer(vm api.VM) *JSONRPCServer {
	return &JSONRPCServer{vm: vm}
}

type GenesisReply struct {
	Genesis *vgenesis.Genesis `json:"genesis"`
}

func (j *JSONRPCServer) Genesis(_ *http.Request, _ *struct{}, reply *GenesisReply) (err error) {
	reply.Genesis = j.vm.Genesis().(*vgenesis.Genesis)
	return nil
}

type BalanceArgs struct {
	Address codec.Address `json:"address"`
}

type BalanceReply struct {
	Amount uint64 `json:"amount"`
}

func (j *JSONRPCServer) Balance(req *http.Request, args *BalanceArgs, reply *BalanceReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.Balance")
	defer span.End()

	balance, err := storage.GetBalanceFromState(ctx, j.vm.ReadState, args.Address)
	if err != nil {
		return err
	}
	reply.Amount = balance
	return err
}

type MarketArgs struct {
	MarketID ids.ID `json:"market_id"`
}

type MarketReply struct {
	Status          uint8  `json:"status"`
	Outcomes        uint8  `json:"outcomes"`
	ResolutionTime  int64  `json:"resolution_time"`
	ResolvedOutcome uint8  `json:"resolved_outcome"`
	Question        []byte `json:"question"`
}

func (j *JSONRPCServer) Market(req *http.Request, args *MarketArgs, reply *MarketReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.Market")
	defer span.End()

	status, outcomes, resolutionTime, resolvedOutcome, question, err := storage.GetMarketFromState(ctx, j.vm.ReadState, args.MarketID)
	if err != nil {
		return err
	}
	reply.Status = status
	reply.Outcomes = outcomes
	reply.ResolutionTime = resolutionTime
	reply.ResolvedOutcome = resolvedOutcome
	reply.Question = question
	return nil
}

type PoolArgs struct {
	Asset0 uint8 `json:"asset0"`
	Asset1 uint8 `json:"asset1"`
}

type PoolReply struct {
	Asset0   uint8  `json:"asset0"`
	Asset1   uint8  `json:"asset1"`
	FeeBips  uint16 `json:"fee_bips"`
	Reserve0 uint64 `json:"reserve0"`
	Reserve1 uint64 `json:"reserve1"`
	TotalLP  uint64 `json:"total_lp"`
}

func (j *JSONRPCServer) Pool(req *http.Request, args *PoolArgs, reply *PoolReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.Pool")
	defer span.End()

	pool, err := storage.GetPoolFromState(ctx, j.vm.ReadState, args.Asset0, args.Asset1)
	if err != nil {
		return err
	}
	reply.Asset0 = pool.Asset0
	reply.Asset1 = pool.Asset1
	reply.FeeBips = pool.FeeBips
	reply.Reserve0 = pool.Reserve0
	reply.Reserve1 = pool.Reserve1
	reply.TotalLP = pool.TotalLP
	return nil
}

type LPBalanceArgs struct {
	Asset0  uint8         `json:"asset0"`
	Asset1  uint8         `json:"asset1"`
	Address codec.Address `json:"address"`
}

type LPBalanceReply struct {
	Amount uint64 `json:"amount"`
}

func (j *JSONRPCServer) LPBalance(req *http.Request, args *LPBalanceArgs, reply *LPBalanceReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.LPBalance")
	defer span.End()

	amount, err := storage.GetLPBalanceFromState(ctx, j.vm.ReadState, args.Asset0, args.Asset1, args.Address)
	if err != nil {
		return err
	}
	reply.Amount = amount
	return nil
}

type VAIBalanceArgs struct {
	Address codec.Address `json:"address"`
}

type VAIBalanceReply struct {
	Amount uint64 `json:"amount"`
}

func (j *JSONRPCServer) VAIBalance(req *http.Request, args *VAIBalanceArgs, reply *VAIBalanceReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.VAIBalance")
	defer span.End()

	balance, err := storage.GetVAIBalanceFromState(ctx, j.vm.ReadState, args.Address)
	if err != nil {
		return err
	}
	reply.Amount = balance
	return nil
}

type VAIStateReply struct {
	DebtCeiling      uint64        `json:"debt_ceiling"`
	EpochMintLimit   uint64        `json:"epoch_mint_limit"`
	MintEpochSeconds int64         `json:"mint_epoch_seconds"`
	MintAuthority    codec.Address `json:"mint_authority"`
	TotalDebt        uint64        `json:"total_debt"`
	EpochStartUnix   int64         `json:"epoch_start_unix"`
	EpochMinted      uint64        `json:"epoch_minted"`
}

func (j *JSONRPCServer) VAIState(req *http.Request, _ *struct{}, reply *VAIStateReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.VAIState")
	defer span.End()

	im, err := j.vm.ImmutableState(ctx)
	if err != nil {
		return err
	}
	cfg, err := storage.GetVAIConfig(ctx, im)
	if err != nil {
		return err
	}
	stateVal, err := storage.GetVAIState(ctx, im)
	if err != nil {
		return err
	}
	reply.DebtCeiling = cfg.DebtCeiling
	reply.EpochMintLimit = cfg.EpochMintLimit
	reply.MintEpochSeconds = cfg.MintEpochSeconds
	reply.MintAuthority = cfg.MintAuthority
	reply.TotalDebt = stateVal.TotalDebt
	reply.EpochStartUnix = stateVal.EpochStartUnix
	reply.EpochMinted = stateVal.EpochMinted
	return nil
}

type TreasuryReply struct {
	Governance          codec.Address `json:"governance"`
	Operations          codec.Address `json:"operations"`
	MaxReleaseBips      uint16        `json:"max_release_bips"`
	ReleaseEpochSeconds int64         `json:"release_epoch_seconds"`
	Locked              uint64        `json:"locked"`
	Live                uint64        `json:"live"`
	Released            uint64        `json:"released"`
	LastReleaseUnix     int64         `json:"last_release_unix"`
}

func (j *JSONRPCServer) Treasury(req *http.Request, _ *struct{}, reply *TreasuryReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.Treasury")
	defer span.End()

	im, err := j.vm.ImmutableState(ctx)
	if err != nil {
		return err
	}
	cfg, err := storage.GetTreasuryConfig(ctx, im)
	if err != nil {
		return err
	}
	stateVal, err := storage.GetTreasuryState(ctx, im)
	if err != nil {
		return err
	}
	reply.Governance = cfg.Governance
	reply.Operations = cfg.Operations
	reply.MaxReleaseBips = cfg.MaxReleaseBips
	reply.ReleaseEpochSeconds = cfg.ReleaseEpochSeconds
	reply.Locked = stateVal.Locked
	reply.Live = stateVal.Live
	reply.Released = stateVal.Released
	reply.LastReleaseUnix = stateVal.LastReleaseUnix
	return nil
}

type FeeRouterReply struct {
	MSRBBips uint16 `json:"msrb_bips"`
	COLBips  uint16 `json:"col_bips"`
	OpsBips  uint16 `json:"ops_bips"`

	MSRBBudget uint64 `json:"msrb_budget"`
	COLBudget  uint64 `json:"col_budget"`
	OpsBudget  uint64 `json:"ops_budget"`
}

func (j *JSONRPCServer) FeeRouter(req *http.Request, _ *struct{}, reply *FeeRouterReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.FeeRouter")
	defer span.End()

	im, err := j.vm.ImmutableState(ctx)
	if err != nil {
		return err
	}
	cfg, err := storage.GetFeeRouterConfig(ctx, im)
	if err != nil {
		return err
	}
	stateVal, err := storage.GetFeeRouterState(ctx, im)
	if err != nil {
		return err
	}
	reply.MSRBBips = cfg.MSRBBips
	reply.COLBips = cfg.COLBips
	reply.OpsBips = cfg.OpsBips
	reply.MSRBBudget = stateVal.MSRBBudget
	reply.COLBudget = stateVal.COLBudget
	reply.OpsBudget = stateVal.OpsBudget
	return nil
}

type RiskReply struct {
	BackingFloorBips uint32 `json:"backing_floor_bips"`

	VEILLtvBips   uint16 `json:"veil_ltv_bips"`
	WVEILLtvBips  uint16 `json:"wveil_ltv_bips"`
	WSVEILLtvBips uint16 `json:"wsveil_ltv_bips"`

	VEILHaircutBips   uint16 `json:"veil_haircut_bips"`
	WVEILHaircutBips  uint16 `json:"wveil_haircut_bips"`
	WSVEILHaircutBips uint16 `json:"wsveil_haircut_bips"`
}

func (j *JSONRPCServer) Risk(req *http.Request, _ *struct{}, reply *RiskReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.Risk")
	defer span.End()

	im, err := j.vm.ImmutableState(ctx)
	if err != nil {
		return err
	}
	cfg, err := storage.GetRiskConfig(ctx, im)
	if err != nil {
		return err
	}
	reply.BackingFloorBips = cfg.BackingFloorBips
	reply.VEILLtvBips = cfg.VEILLtvBips
	reply.WVEILLtvBips = cfg.WVEILLtvBips
	reply.WSVEILLtvBips = cfg.WSVEILLtvBips
	reply.VEILHaircutBips = cfg.VEILHaircutBips
	reply.WVEILHaircutBips = cfg.WVEILHaircutBips
	reply.WSVEILHaircutBips = cfg.WSVEILHaircutBips
	return nil
}

type ReserveReply struct {
	ExogenousReserve uint64 `json:"exogenous_reserve"`
	VAIBuffer        uint64 `json:"vai_buffer"`

	TotalDebt        uint64 `json:"total_debt"`
	BackingFloorBips uint32 `json:"backing_floor_bips"`
	BackingRatioBips uint64 `json:"backing_ratio_bips"`
	MeetsFloor       bool   `json:"meets_floor"`
}

func (j *JSONRPCServer) Reserve(req *http.Request, _ *struct{}, reply *ReserveReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.Reserve")
	defer span.End()

	im, err := j.vm.ImmutableState(ctx)
	if err != nil {
		return err
	}
	reserveState, err := storage.GetReserveState(ctx, im)
	if err != nil {
		return err
	}
	riskCfg, err := storage.GetRiskConfig(ctx, im)
	if err != nil {
		return err
	}
	vaiState, err := storage.GetVAIState(ctx, im)
	if err != nil {
		return err
	}

	reply.ExogenousReserve = reserveState.ExogenousReserve
	reply.VAIBuffer = reserveState.VAIBuffer
	reply.TotalDebt = vaiState.TotalDebt
	reply.BackingFloorBips = riskCfg.BackingFloorBips

	if vaiState.TotalDebt == 0 {
		reply.BackingRatioBips = 0
		reply.MeetsFloor = true
		return nil
	}

	num := new(big.Int).Mul(new(big.Int).SetUint64(reserveState.ExogenousReserve), new(big.Int).SetUint64(10_000))
	den := new(big.Int).SetUint64(vaiState.TotalDebt)
	ratio := new(big.Int).Div(num, den)
	if ratio.IsUint64() {
		reply.BackingRatioBips = ratio.Uint64()
	} else {
		reply.BackingRatioBips = ^uint64(0)
	}
	reply.MeetsFloor = reply.BackingRatioBips >= uint64(riskCfg.BackingFloorBips)
	return nil
}

type ProofConfigReply struct {
	RequireProof      bool          `json:"require_proof"`
	RequiredProofType uint8         `json:"required_proof_type"`
	BatchWindowMs     int64         `json:"batch_window_ms"`
	ProofDeadlineMs   int64         `json:"proof_deadline_ms"`
	ProverAuthority   codec.Address `json:"prover_authority"`
}

func (j *JSONRPCServer) ProofConfig(req *http.Request, _ *struct{}, reply *ProofConfigReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.ProofConfig")
	defer span.End()

	im, err := j.vm.ImmutableState(ctx)
	if err != nil {
		return err
	}
	cfg, err := storage.GetProofConfig(ctx, im)
	if err != nil {
		return err
	}

	reply.RequireProof = cfg.RequireProof
	reply.RequiredProofType = cfg.RequiredProofType
	reply.BatchWindowMs = cfg.BatchWindowMs
	reply.ProofDeadlineMs = cfg.ProofDeadlineMs
	reply.ProverAuthority = cfg.ProverAuthority
	return nil
}

type BatchProofArgs struct {
	MarketID ids.ID `json:"market_id"`
	WindowID uint64 `json:"window_id"`
}

type BatchProofReply struct {
	ProofType        uint8         `json:"proof_type"`
	SubmittedAtMs    int64         `json:"submitted_at_ms"`
	WindowCloseAtMs  int64         `json:"window_close_at_ms"`
	Prover           codec.Address `json:"prover"`
	ProofCommitment  []byte        `json:"proof_commitment"`
	PublicInputsHash []byte        `json:"public_inputs_hash"`
	FillsHash        []byte        `json:"fills_hash"`
}

func (j *JSONRPCServer) BatchProof(req *http.Request, args *BatchProofArgs, reply *BatchProofReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.BatchProof")
	defer span.End()

	im, err := j.vm.ImmutableState(ctx)
	if err != nil {
		return err
	}
	rec, err := storage.GetBatchProofRecord(ctx, im, args.MarketID, args.WindowID)
	if err != nil {
		return err
	}

	reply.ProofType = rec.ProofType
	reply.SubmittedAtMs = rec.SubmittedAtMs
	reply.WindowCloseAtMs = rec.WindowCloseAtMs
	reply.Prover = rec.Prover
	reply.ProofCommitment = rec.ProofCommitment[:]
	reply.PublicInputsHash = append([]byte(nil), rec.PublicInputsHash...)
	reply.FillsHash = append([]byte(nil), rec.FillsHash...)
	return nil
}

type VellumProofArgs struct {
	MarketID ids.ID `json:"market_id"`
	WindowID uint64 `json:"window_id"`
}

type VellumProofReply struct {
	Proof []byte `json:"proof"`
	Size  uint32 `json:"size"`
}

func (j *JSONRPCServer) VellumProof(req *http.Request, args *VellumProofArgs, reply *VellumProofReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.VellumProof")
	defer span.End()

	proof, err := storage.GetVellumProofFromState(ctx, j.vm.ReadState, args.MarketID, args.WindowID)
	if err != nil {
		return err
	}
	reply.Proof = proof
	reply.Size = uint32(len(proof))
	return nil
}

type BloodswornArgs struct {
	Address codec.Address `json:"address"`
}

type BloodswornReply struct {
	TotalAcceptedProofs uint64 `json:"total_accepted_proofs"`
	ActiveStreak        uint64 `json:"active_streak"`
	LastProofAtMs       int64  `json:"last_proof_at_ms"`
	ScarCount           uint32 `json:"scar_count"`
	TrustBips           uint32 `json:"trust_bips"`
	TrustTier           string `json:"trust_tier"`
}

func (j *JSONRPCServer) Bloodsworn(req *http.Request, args *BloodswornArgs, reply *BloodswornReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.Bloodsworn")
	defer span.End()

	bloodsworn, err := storage.GetBloodswornFromState(ctx, j.vm.ReadState, args.Address)
	if err != nil {
		return err
	}
	reply.TotalAcceptedProofs = bloodsworn.TotalAcceptedProofs
	reply.ActiveStreak = bloodsworn.ActiveStreak
	reply.LastProofAtMs = bloodsworn.LastProofAtMs
	reply.ScarCount = bloodsworn.ScarCount
	reply.TrustBips, reply.TrustTier = scoreBloodswornTrust(bloodsworn)
	return nil
}

type GlyphArgs struct {
	MarketID ids.ID `json:"market_id"`
	WindowID uint64 `json:"window_id"`
}

type GlyphReply struct {
	Class            uint8         `json:"class"`
	Rarity           uint8         `json:"rarity"`
	CreatedAtMs      int64         `json:"created_at_ms"`
	Prover           codec.Address `json:"prover"`
	ProofCommitment  []byte        `json:"proof_commitment"`
	PublicInputsHash []byte        `json:"public_inputs_hash"`
	Entropy          []byte        `json:"entropy"`
}

func (j *JSONRPCServer) Glyph(req *http.Request, args *GlyphArgs, reply *GlyphReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.Glyph")
	defer span.End()

	glyph, err := storage.GetGlyphFromState(ctx, j.vm.ReadState, args.MarketID, args.WindowID)
	if err != nil {
		return err
	}
	reply.Class = glyph.Class
	reply.Rarity = glyph.Rarity
	reply.CreatedAtMs = glyph.CreatedAtMs
	reply.Prover = glyph.Prover
	reply.ProofCommitment = glyph.ProofCommitment[:]
	reply.PublicInputsHash = glyph.PublicInputsHash[:]
	reply.Entropy = glyph.Entropy[:]
	return nil
}

type ClearInputsHashArgs struct {
	MarketID    ids.ID `json:"market_id"`
	WindowID    uint64 `json:"window_id"`
	ClearPrice  uint64 `json:"clear_price"`
	TotalVolume uint64 `json:"total_volume"`
	FillsHash   []byte `json:"fills_hash"`
}

type ClearInputsHashReply struct {
	PublicInputsHash []byte `json:"public_inputs_hash"`
}

func (j *JSONRPCServer) ClearInputsHash(req *http.Request, args *ClearInputsHashArgs, reply *ClearInputsHashReply) error {
	_, span := j.vm.Tracer().Start(req.Context(), "Server.ClearInputsHash")
	defer span.End()

	hash := actions.ComputeClearPublicInputsHash(
		args.MarketID,
		args.WindowID,
		args.ClearPrice,
		args.TotalVolume,
		args.FillsHash,
	)
	reply.PublicInputsHash = hash[:]
	return nil
}

type ZKMetricsArgs struct {
	Limit          uint32 `json:"limit"`
	IncludeWindows bool   `json:"include_windows"`
}

func (j *JSONRPCServer) ZKMetrics(req *http.Request, args *ZKMetricsArgs, reply *actions.ZKMetricsSnapshot) error {
	_, span := j.vm.Tracer().Start(req.Context(), "Server.ZKMetrics")
	defer span.End()

	limit := 0
	includeWindows := true
	if args != nil {
		limit = int(args.Limit)
		includeWindows = args.IncludeWindows
	}
	*reply = actions.GetZKMetricsSnapshot(limit, includeWindows)
	return nil
}

// Alias for requester title-casing (e.g. "zkmetrics" -> "Zkmetrics").
func (j *JSONRPCServer) Zkmetrics(req *http.Request, args *ZKMetricsArgs, reply *actions.ZKMetricsSnapshot) error {
	return j.ZKMetrics(req, args, reply)
}

type ZKMetricsResetReply struct {
	OK bool `json:"ok"`
}

func (j *JSONRPCServer) ZKMetricsReset(req *http.Request, _ *struct{}, reply *ZKMetricsResetReply) error {
	_, span := j.vm.Tracer().Start(req.Context(), "Server.ZKMetricsReset")
	defer span.End()

	actions.ResetZKMetrics()
	reply.OK = true
	return nil
}

// Alias for requester title-casing (e.g. "zkmetricsreset" -> "Zkmetricsreset").
func (j *JSONRPCServer) Zkmetricsreset(req *http.Request, args *struct{}, reply *ZKMetricsResetReply) error {
	return j.ZKMetricsReset(req, args, reply)
}

type RecordZKProverMetricsArgs struct {
	MarketID          ids.ID `json:"market_id"`
	WindowID          uint64 `json:"window_id"`
	BatchSizeHint     uint32 `json:"batch_size_hint"`
	WitnessBuildMs    int64  `json:"witness_build_ms"`
	ProofGenerationMs int64  `json:"proof_generation_ms"`
}

type RecordZKProverMetricsReply struct {
	Recorded bool `json:"recorded"`
}

func (j *JSONRPCServer) RecordZKProverMetrics(req *http.Request, args *RecordZKProverMetricsArgs, reply *RecordZKProverMetricsReply) error {
	_, span := j.vm.Tracer().Start(req.Context(), "Server.RecordZKProverMetrics")
	defer span.End()

	if args == nil {
		reply.Recorded = false
		return nil
	}
	actions.RecordProverStageMetrics(
		args.MarketID,
		args.WindowID,
		args.BatchSizeHint,
		args.WitnessBuildMs,
		args.ProofGenerationMs,
	)
	reply.Recorded = true
	return nil
}

// Alias for requester title-casing (e.g. "recordzkprovermetrics" -> "Recordzkprovermetrics").
func (j *JSONRPCServer) Recordzkprovermetrics(req *http.Request, args *RecordZKProverMetricsArgs, reply *RecordZKProverMetricsReply) error {
	return j.RecordZKProverMetrics(req, args, reply)
}

func scoreBloodswornTrust(b storage.Bloodsworn) (uint32, string) {
	if b.TotalAcceptedProofs == 0 {
		return 0, "Unproven"
	}
	base := uint64(5_000)
	proofBoost := b.TotalAcceptedProofs * 15
	if proofBoost > 3_500 {
		proofBoost = 3_500
	}
	streakBoost := b.ActiveStreak * 25
	if streakBoost > 1_500 {
		streakBoost = 1_500
	}
	penalty := uint64(b.ScarCount) * 700

	score := base + proofBoost + streakBoost
	if penalty >= score {
		score = 0
	} else {
		score -= penalty
	}
	if score > 10_000 {
		score = 10_000
	}

	switch {
	case score >= 9_000:
		return uint32(score), "Mythic"
	case score >= 7_500:
		return uint32(score), "Legend"
	case score >= 6_000:
		return uint32(score), "Proven"
	case score >= 4_000:
		return uint32(score), "Rising"
	case score > 0:
		return uint32(score), "Fragile"
	default:
		return 0, "Unproven"
	}
}
