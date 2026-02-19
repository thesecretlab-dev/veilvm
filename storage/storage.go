package storage

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/metadata"

	smath "github.com/ava-labs/avalanchego/utils/math"
)

type ReadState func(context.Context, [][]byte) ([][]byte, []error)

const (
	balancePrefix         byte = metadata.DefaultMinimumPrefix
	marketPrefix          byte = metadata.DefaultMinimumPrefix + 1
	commitmentPrefix      byte = metadata.DefaultMinimumPrefix + 2
	batchPrefix           byte = metadata.DefaultMinimumPrefix + 3
	oraclePrefix          byte = metadata.DefaultMinimumPrefix + 4
	treasuryConfigPrefix  byte = metadata.DefaultMinimumPrefix + 5
	treasuryStatePrefix   byte = metadata.DefaultMinimumPrefix + 6
	feeRouterConfigPrefix byte = metadata.DefaultMinimumPrefix + 7
	feeRouterStatePrefix  byte = metadata.DefaultMinimumPrefix + 8
	vaiConfigPrefix       byte = metadata.DefaultMinimumPrefix + 9
	vaiStatePrefix        byte = metadata.DefaultMinimumPrefix + 10
	vaiBalancePrefix      byte = metadata.DefaultMinimumPrefix + 11
	poolPrefix            byte = metadata.DefaultMinimumPrefix + 12
	lpBalancePrefix       byte = metadata.DefaultMinimumPrefix + 13
	riskConfigPrefix      byte = metadata.DefaultMinimumPrefix + 14
	reserveStatePrefix    byte = metadata.DefaultMinimumPrefix + 15
	proofConfigPrefix     byte = metadata.DefaultMinimumPrefix + 16
	batchProofPrefix      byte = metadata.DefaultMinimumPrefix + 17
	vellumProofPrefix     byte = metadata.DefaultMinimumPrefix + 18
	bloodswornPrefix      byte = metadata.DefaultMinimumPrefix + 19
	glyphPrefix           byte = metadata.DefaultMinimumPrefix + 20
)

const (
	BalanceChunks         uint16 = 1
	MarketChunks          uint16 = 8
	CommitmentChunks      uint16 = 16
	BatchChunks           uint16 = 4
	OracleChunks          uint16 = 8
	TreasuryConfigChunks  uint16 = 4
	TreasuryStateChunks   uint16 = 4
	FeeRouterConfigChunks uint16 = 2
	FeeRouterStateChunks  uint16 = 4
	VAIConfigChunks       uint16 = 4
	VAIStateChunks        uint16 = 4
	VAIBalanceChunks      uint16 = 1
	PoolChunks            uint16 = 4
	LPBalanceChunks       uint16 = 1
	RiskConfigChunks      uint16 = 4
	ReserveStateChunks    uint16 = 4
	ProofConfigChunks     uint16 = 4
	BatchProofChunks      uint16 = 8
	VellumProofChunks     uint16 = 128
	BloodswornChunks      uint16 = 4
	GlyphChunks           uint16 = 16
)

const (
	bipsDenominator     uint64 = 10_000
	maxVellumProofBytes        = 131_072
)

const (
	MarketStatusActive   uint8 = 0
	MarketStatusResolved uint8 = 1
	MarketStatusDisputed uint8 = 2
)

type TreasuryConfig struct {
	Governance          codec.Address
	Operations          codec.Address
	MaxReleaseBips      uint16
	ReleaseEpochSeconds int64
}

type TreasuryState struct {
	Locked          uint64
	Live            uint64
	Released        uint64
	LastReleaseUnix int64
}

type FeeRouterConfig struct {
	MSRBBips uint16
	COLBips  uint16
	OpsBips  uint16
}

type FeeRouterState struct {
	MSRBBudget uint64
	COLBudget  uint64
	OpsBudget  uint64
}

type VAIConfig struct {
	MintAuthority    codec.Address
	DebtCeiling      uint64
	EpochMintLimit   uint64
	MintEpochSeconds int64
}

type VAIState struct {
	TotalDebt      uint64
	EpochStartUnix int64
	EpochMinted    uint64
}

type RiskConfig struct {
	BackingFloorBips uint32

	VEILLtvBips   uint16
	WVEILLtvBips  uint16
	WSVEILLtvBips uint16

	VEILHaircutBips   uint16
	WVEILHaircutBips  uint16
	WSVEILHaircutBips uint16
}

type ReserveState struct {
	ExogenousReserve uint64
	VAIBuffer        uint64
}

type ProofConfig struct {
	RequireProof      bool
	RequiredProofType uint8
	BatchWindowMs     int64
	ProofDeadlineMs   int64
	ProverAuthority   codec.Address
}

type BatchProofRecord struct {
	ProofType        uint8
	SubmittedAtMs    int64
	WindowCloseAtMs  int64
	Prover           codec.Address
	ProofCommitment  [32]byte
	PublicInputsHash []byte
	FillsHash        []byte
}

type Bloodsworn struct {
	TotalAcceptedProofs uint64
	ActiveStreak        uint64
	LastProofAtMs       int64
	ScarCount           uint32
}

type Glyph struct {
	Class            uint8
	Rarity           uint8
	CreatedAtMs      int64
	Prover           codec.Address
	ProofCommitment  [32]byte
	PublicInputsHash [32]byte
	Entropy          [32]byte
}

type Pool struct {
	Asset0   uint8
	Asset1   uint8
	FeeBips  uint16
	Reserve0 uint64
	Reserve1 uint64
	TotalLP  uint64
}

// ========== Balance ==========

func BalanceKey(addr codec.Address) (k []byte) {
	k = make([]byte, 1+codec.AddressLen+consts.Uint16Len)
	k[0] = balancePrefix
	copy(k[1:], addr[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen:], BalanceChunks)
	return
}

func GetBalance(ctx context.Context, im state.Immutable, addr codec.Address) (uint64, error) {
	_, bal, _, err := getBalance(ctx, im, addr)
	return bal, err
}

func getBalance(ctx context.Context, im state.Immutable, addr codec.Address) ([]byte, uint64, bool, error) {
	k := BalanceKey(addr)
	bal, exists, err := innerGetBalance(im.GetValue(ctx, k))
	return k, bal, exists, err
}

func GetBalanceFromState(ctx context.Context, f ReadState, addr codec.Address) (uint64, error) {
	k := BalanceKey(addr)
	values, errs := f(ctx, [][]byte{k})
	bal, _, err := innerGetBalance(values[0], errs[0])
	return bal, err
}

func innerGetBalance(v []byte, err error) (uint64, bool, error) {
	if errors.Is(err, database.ErrNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	val, err := database.ParseUInt64(v)
	if err != nil {
		return 0, false, err
	}
	return val, true, nil
}

func SetBalance(ctx context.Context, mu state.Mutable, addr codec.Address, balance uint64) error {
	k := BalanceKey(addr)
	return setBalance(ctx, mu, k, balance)
}

func setBalance(ctx context.Context, mu state.Mutable, key []byte, balance uint64) error {
	return mu.Insert(ctx, key, binary.BigEndian.AppendUint64(nil, balance))
}

func AddBalance(ctx context.Context, mu state.Mutable, addr codec.Address, amount uint64) (uint64, error) {
	key, bal, _, err := getBalance(ctx, mu, addr)
	if err != nil {
		return 0, err
	}
	nbal, err := smath.Add(bal, amount)
	if err != nil {
		return 0, fmt.Errorf("%w: could not add balance (bal=%d, addr=%v, amount=%d)", ErrInvalidBalance, bal, addr, amount)
	}
	return nbal, setBalance(ctx, mu, key, nbal)
}

func SubBalance(ctx context.Context, mu state.Mutable, addr codec.Address, amount uint64) (uint64, error) {
	key, bal, ok, err := getBalance(ctx, mu, addr)
	if !ok {
		return 0, fmt.Errorf("%w: could not subtract (bal=%d, addr=%v, amount=%d)", ErrInvalidBalance, 0, addr, amount)
	}
	if err != nil {
		return 0, err
	}
	nbal, err := smath.Sub(bal, amount)
	if err != nil {
		return 0, fmt.Errorf("%w: could not subtract balance (bal=%d < amount=%d, gap=%d, addr=%v)", ErrInvalidBalance, bal, amount, amount-bal, addr)
	}
	if nbal == 0 {
		return 0, mu.Remove(ctx, key)
	}
	return nbal, setBalance(ctx, mu, key, nbal)
}

// ========== Market ==========

func MarketKey(marketID ids.ID) (k []byte) {
	k = make([]byte, 1+ids.IDLen+consts.Uint16Len)
	k[0] = marketPrefix
	copy(k[1:], marketID[:])
	binary.BigEndian.PutUint16(k[1+ids.IDLen:], MarketChunks)
	return
}

func PutMarket(ctx context.Context, mu state.Mutable, marketID ids.ID, status uint8, outcomes uint8, resolutionTime int64, resolvedOutcome uint8, question []byte) error {
	k := MarketKey(marketID)
	v := make([]byte, 0, 11+len(question))
	v = append(v, status)
	v = append(v, outcomes)
	v = binary.BigEndian.AppendUint64(v, uint64(resolutionTime))
	v = append(v, resolvedOutcome)
	v = append(v, question...)
	return mu.Insert(ctx, k, v)
}

func GetMarket(ctx context.Context, im state.Immutable, marketID ids.ID) (uint8, uint8, int64, uint8, []byte, error) {
	k := MarketKey(marketID)
	v, err := im.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return 0, 0, 0, 0, nil, ErrMarketNotFound
	}
	if err != nil {
		return 0, 0, 0, 0, nil, err
	}
	if len(v) < 11 {
		return 0, 0, 0, 0, nil, fmt.Errorf("invalid market data length: %d", len(v))
	}
	return v[0], v[1], int64(binary.BigEndian.Uint64(v[2:10])), v[10], v[11:], nil
}

func GetMarketFromState(ctx context.Context, f ReadState, marketID ids.ID) (uint8, uint8, int64, uint8, []byte, error) {
	k := MarketKey(marketID)
	values, errs := f(ctx, [][]byte{k})
	if errors.Is(errs[0], database.ErrNotFound) {
		return 0, 0, 0, 0, nil, ErrMarketNotFound
	}
	if errs[0] != nil {
		return 0, 0, 0, 0, nil, errs[0]
	}
	v := values[0]
	if len(v) < 11 {
		return 0, 0, 0, 0, nil, fmt.Errorf("invalid market data length: %d", len(v))
	}
	return v[0], v[1], int64(binary.BigEndian.Uint64(v[2:10])), v[10], v[11:], nil
}

// ========== Commitment ==========

func CommitmentKey(marketID ids.ID, windowID uint64, actor codec.Address) (k []byte) {
	k = make([]byte, 1+ids.IDLen+8+codec.AddressLen+consts.Uint16Len)
	k[0] = commitmentPrefix
	copy(k[1:], marketID[:])
	binary.BigEndian.PutUint64(k[1+ids.IDLen:], windowID)
	copy(k[1+ids.IDLen+8:], actor[:])
	binary.BigEndian.PutUint16(k[1+ids.IDLen+8+codec.AddressLen:], CommitmentChunks)
	return
}

func PutCommitment(ctx context.Context, mu state.Mutable, marketID ids.ID, windowID uint64, actor codec.Address, envelope []byte, commitment []byte) error {
	k := CommitmentKey(marketID, windowID, actor)
	v := make([]byte, 0, len(commitment)+len(envelope))
	v = append(v, commitment...)
	v = append(v, envelope...)
	return mu.Insert(ctx, k, v)
}

// ========== Batch ==========

func BatchKey(marketID ids.ID, windowID uint64) (k []byte) {
	k = make([]byte, 1+ids.IDLen+8+consts.Uint16Len)
	k[0] = batchPrefix
	copy(k[1:], marketID[:])
	binary.BigEndian.PutUint64(k[1+ids.IDLen:], windowID)
	binary.BigEndian.PutUint16(k[1+ids.IDLen+8:], BatchChunks)
	return
}

func PutBatchResult(ctx context.Context, mu state.Mutable, marketID ids.ID, windowID uint64, clearPrice uint64, totalVolume uint64, fillsHash []byte) error {
	k := BatchKey(marketID, windowID)
	v := make([]byte, 0, 16+len(fillsHash))
	v = binary.BigEndian.AppendUint64(v, clearPrice)
	v = binary.BigEndian.AppendUint64(v, totalVolume)
	v = append(v, fillsHash...)
	return mu.Insert(ctx, k, v)
}

// ========== Oracle ==========

func OracleKey(marketID ids.ID, validatorIndex uint32) (k []byte) {
	k = make([]byte, 1+ids.IDLen+4+consts.Uint16Len)
	k[0] = oraclePrefix
	copy(k[1:], marketID[:])
	binary.BigEndian.PutUint32(k[1+ids.IDLen:], validatorIndex)
	binary.BigEndian.PutUint16(k[1+ids.IDLen+4:], OracleChunks)
	return
}

func DisputeKey(marketID ids.ID) (k []byte) {
	return OracleKey(marketID, 0xFFFFFFFF)
}

func PutDispute(ctx context.Context, mu state.Mutable, marketID ids.ID, bond uint64, evidence []byte) error {
	k := DisputeKey(marketID)
	v := make([]byte, 0, 8+len(evidence))
	v = binary.BigEndian.AppendUint64(v, bond)
	v = append(v, evidence...)
	return mu.Insert(ctx, k, v)
}

// ========== AMM (UniV2-style pools) ==========

func sortedAssetPair(asset0 uint8, asset1 uint8) (uint8, uint8) {
	if asset0 <= asset1 {
		return asset0, asset1
	}
	return asset1, asset0
}

func PoolKey(asset0 uint8, asset1 uint8) []byte {
	a0, a1 := sortedAssetPair(asset0, asset1)
	k := make([]byte, 1+1+1+consts.Uint16Len)
	k[0] = poolPrefix
	k[1] = a0
	k[2] = a1
	binary.BigEndian.PutUint16(k[3:], PoolChunks)
	return k
}

func LPBalanceKey(asset0 uint8, asset1 uint8, addr codec.Address) []byte {
	a0, a1 := sortedAssetPair(asset0, asset1)
	k := make([]byte, 1+1+1+codec.AddressLen+consts.Uint16Len)
	k[0] = lpBalancePrefix
	k[1] = a0
	k[2] = a1
	copy(k[3:], addr[:])
	binary.BigEndian.PutUint16(k[3+codec.AddressLen:], LPBalanceChunks)
	return k
}

func PutPool(ctx context.Context, mu state.Mutable, pool Pool) error {
	k := PoolKey(pool.Asset0, pool.Asset1)
	a0, a1 := sortedAssetPair(pool.Asset0, pool.Asset1)
	v := make([]byte, 0, 1+1+consts.Uint16Len+consts.Uint64Len*3)
	v = append(v, a0)
	v = append(v, a1)
	v = binary.BigEndian.AppendUint16(v, pool.FeeBips)
	v = binary.BigEndian.AppendUint64(v, pool.Reserve0)
	v = binary.BigEndian.AppendUint64(v, pool.Reserve1)
	v = binary.BigEndian.AppendUint64(v, pool.TotalLP)
	return mu.Insert(ctx, k, v)
}

func GetPool(ctx context.Context, im state.Immutable, asset0 uint8, asset1 uint8) (Pool, error) {
	k := PoolKey(asset0, asset1)
	v, err := im.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return Pool{}, ErrPoolNotFound
	}
	if err != nil {
		return Pool{}, err
	}
	return parsePool(v)
}

func GetPoolFromState(ctx context.Context, f ReadState, asset0 uint8, asset1 uint8) (Pool, error) {
	k := PoolKey(asset0, asset1)
	values, errs := f(ctx, [][]byte{k})
	if errors.Is(errs[0], database.ErrNotFound) {
		return Pool{}, ErrPoolNotFound
	}
	if errs[0] != nil {
		return Pool{}, errs[0]
	}
	return parsePool(values[0])
}

func parsePool(v []byte) (Pool, error) {
	const minLen = 1 + 1 + consts.Uint16Len + consts.Uint64Len*3
	if len(v) < minLen {
		return Pool{}, fmt.Errorf("invalid pool data length: %d", len(v))
	}
	pool := Pool{
		Asset0:   v[0],
		Asset1:   v[1],
		FeeBips:  binary.BigEndian.Uint16(v[2 : 2+consts.Uint16Len]),
		Reserve0: binary.BigEndian.Uint64(v[4 : 4+consts.Uint64Len]),
		Reserve1: binary.BigEndian.Uint64(v[12 : 12+consts.Uint64Len]),
		TotalLP:  binary.BigEndian.Uint64(v[20 : 20+consts.Uint64Len]),
	}
	return pool, nil
}

func GetLPBalance(ctx context.Context, im state.Immutable, asset0 uint8, asset1 uint8, addr codec.Address) (uint64, error) {
	_, bal, _, err := getLPBalance(ctx, im, asset0, asset1, addr)
	return bal, err
}

func GetLPBalanceFromState(ctx context.Context, f ReadState, asset0 uint8, asset1 uint8, addr codec.Address) (uint64, error) {
	k := LPBalanceKey(asset0, asset1, addr)
	values, errs := f(ctx, [][]byte{k})
	bal, _, err := innerGetBalance(values[0], errs[0])
	return bal, err
}

func getLPBalance(ctx context.Context, im state.Immutable, asset0 uint8, asset1 uint8, addr codec.Address) ([]byte, uint64, bool, error) {
	k := LPBalanceKey(asset0, asset1, addr)
	bal, exists, err := innerGetBalance(im.GetValue(ctx, k))
	return k, bal, exists, err
}

func setLPBalance(ctx context.Context, mu state.Mutable, key []byte, balance uint64) error {
	return mu.Insert(ctx, key, binary.BigEndian.AppendUint64(nil, balance))
}

func AddLPBalance(ctx context.Context, mu state.Mutable, asset0 uint8, asset1 uint8, addr codec.Address, amount uint64) (uint64, error) {
	key, bal, _, err := getLPBalance(ctx, mu, asset0, asset1, addr)
	if err != nil {
		return 0, err
	}
	nbal, err := smath.Add(bal, amount)
	if err != nil {
		return 0, fmt.Errorf("%w: could not add LP balance (bal=%d, amount=%d)", ErrInvalidBalance, bal, amount)
	}
	return nbal, setLPBalance(ctx, mu, key, nbal)
}

func SubLPBalance(ctx context.Context, mu state.Mutable, asset0 uint8, asset1 uint8, addr codec.Address, amount uint64) (uint64, error) {
	key, bal, ok, err := getLPBalance(ctx, mu, asset0, asset1, addr)
	if !ok {
		return 0, ErrInsufficientLPBalance
	}
	if err != nil {
		return 0, err
	}
	nbal, err := smath.Sub(bal, amount)
	if err != nil {
		return 0, ErrInsufficientLPBalance
	}
	if nbal == 0 {
		return 0, mu.Remove(ctx, key)
	}
	return nbal, setLPBalance(ctx, mu, key, nbal)
}

// ========== Tokenomics ==========

func singletonKey(prefix byte, chunks uint16) []byte {
	k := make([]byte, 1+consts.Uint16Len)
	k[0] = prefix
	binary.BigEndian.PutUint16(k[1:], chunks)
	return k
}

func TreasuryConfigKey() []byte {
	return singletonKey(treasuryConfigPrefix, TreasuryConfigChunks)
}

func TreasuryStateKey() []byte {
	return singletonKey(treasuryStatePrefix, TreasuryStateChunks)
}

func FeeRouterConfigKey() []byte {
	return singletonKey(feeRouterConfigPrefix, FeeRouterConfigChunks)
}

func FeeRouterStateKey() []byte {
	return singletonKey(feeRouterStatePrefix, FeeRouterStateChunks)
}

func VAIConfigKey() []byte {
	return singletonKey(vaiConfigPrefix, VAIConfigChunks)
}

func VAIStateKey() []byte {
	return singletonKey(vaiStatePrefix, VAIStateChunks)
}

func RiskConfigKey() []byte {
	return singletonKey(riskConfigPrefix, RiskConfigChunks)
}

func ReserveStateKey() []byte {
	return singletonKey(reserveStatePrefix, ReserveStateChunks)
}

func ProofConfigKey() []byte {
	return singletonKey(proofConfigPrefix, ProofConfigChunks)
}

func BatchProofKey(marketID ids.ID, windowID uint64) []byte {
	k := make([]byte, 1+ids.IDLen+consts.Uint64Len+consts.Uint16Len)
	k[0] = batchProofPrefix
	copy(k[1:], marketID[:])
	binary.BigEndian.PutUint64(k[1+ids.IDLen:], windowID)
	binary.BigEndian.PutUint16(k[1+ids.IDLen+consts.Uint64Len:], BatchProofChunks)
	return k
}

func VellumProofKey(marketID ids.ID, windowID uint64) []byte {
	k := make([]byte, 1+ids.IDLen+consts.Uint64Len+consts.Uint16Len)
	k[0] = vellumProofPrefix
	copy(k[1:], marketID[:])
	binary.BigEndian.PutUint64(k[1+ids.IDLen:], windowID)
	binary.BigEndian.PutUint16(k[1+ids.IDLen+consts.Uint64Len:], VellumProofChunks)
	return k
}

func BloodswornKey(addr codec.Address) []byte {
	k := make([]byte, 1+codec.AddressLen+consts.Uint16Len)
	k[0] = bloodswornPrefix
	copy(k[1:], addr[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen:], BloodswornChunks)
	return k
}

func GlyphKey(marketID ids.ID, windowID uint64) []byte {
	k := make([]byte, 1+ids.IDLen+consts.Uint64Len+consts.Uint16Len)
	k[0] = glyphPrefix
	copy(k[1:], marketID[:])
	binary.BigEndian.PutUint64(k[1+ids.IDLen:], windowID)
	binary.BigEndian.PutUint16(k[1+ids.IDLen+consts.Uint64Len:], GlyphChunks)
	return k
}

func VAIBalanceKey(addr codec.Address) []byte {
	k := make([]byte, 1+codec.AddressLen+consts.Uint16Len)
	k[0] = vaiBalancePrefix
	copy(k[1:], addr[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen:], VAIBalanceChunks)
	return k
}

func PutTreasuryConfig(ctx context.Context, mu state.Mutable, cfg TreasuryConfig) error {
	v := make([]byte, 0, codec.AddressLen*2+consts.Uint16Len+consts.Uint64Len)
	v = append(v, cfg.Governance[:]...)
	v = append(v, cfg.Operations[:]...)
	v = binary.BigEndian.AppendUint16(v, cfg.MaxReleaseBips)
	v = binary.BigEndian.AppendUint64(v, uint64(cfg.ReleaseEpochSeconds))
	return mu.Insert(ctx, TreasuryConfigKey(), v)
}

func GetTreasuryConfig(ctx context.Context, im state.Immutable) (TreasuryConfig, error) {
	v, err := im.GetValue(ctx, TreasuryConfigKey())
	if errors.Is(err, database.ErrNotFound) {
		return TreasuryConfig{}, ErrInvalidTokenomicsConfig
	}
	if err != nil {
		return TreasuryConfig{}, err
	}
	minLen := codec.AddressLen*2 + consts.Uint16Len + consts.Uint64Len
	if len(v) < minLen {
		return TreasuryConfig{}, fmt.Errorf("%w: treasury config length %d < %d", ErrInvalidTokenomicsConfig, len(v), minLen)
	}
	var cfg TreasuryConfig
	copy(cfg.Governance[:], v[:codec.AddressLen])
	copy(cfg.Operations[:], v[codec.AddressLen:codec.AddressLen*2])
	offset := codec.AddressLen * 2
	cfg.MaxReleaseBips = binary.BigEndian.Uint16(v[offset : offset+consts.Uint16Len])
	offset += consts.Uint16Len
	cfg.ReleaseEpochSeconds = int64(binary.BigEndian.Uint64(v[offset : offset+consts.Uint64Len]))
	return cfg, nil
}

func PutTreasuryState(ctx context.Context, mu state.Mutable, s TreasuryState) error {
	v := make([]byte, 0, consts.Uint64Len*4)
	v = binary.BigEndian.AppendUint64(v, s.Locked)
	v = binary.BigEndian.AppendUint64(v, s.Live)
	v = binary.BigEndian.AppendUint64(v, s.Released)
	v = binary.BigEndian.AppendUint64(v, uint64(s.LastReleaseUnix))
	return mu.Insert(ctx, TreasuryStateKey(), v)
}

func GetTreasuryState(ctx context.Context, im state.Immutable) (TreasuryState, error) {
	v, err := im.GetValue(ctx, TreasuryStateKey())
	if errors.Is(err, database.ErrNotFound) {
		return TreasuryState{}, ErrInvalidTokenomicsConfig
	}
	if err != nil {
		return TreasuryState{}, err
	}
	minLen := consts.Uint64Len * 4
	if len(v) < minLen {
		return TreasuryState{}, fmt.Errorf("%w: treasury state length %d < %d", ErrInvalidTokenomicsConfig, len(v), minLen)
	}
	return TreasuryState{
		Locked:          binary.BigEndian.Uint64(v[:consts.Uint64Len]),
		Live:            binary.BigEndian.Uint64(v[consts.Uint64Len : consts.Uint64Len*2]),
		Released:        binary.BigEndian.Uint64(v[consts.Uint64Len*2 : consts.Uint64Len*3]),
		LastReleaseUnix: int64(binary.BigEndian.Uint64(v[consts.Uint64Len*3 : consts.Uint64Len*4])),
	}, nil
}

func PutFeeRouterConfig(ctx context.Context, mu state.Mutable, cfg FeeRouterConfig) error {
	v := make([]byte, 0, consts.Uint16Len*3)
	v = binary.BigEndian.AppendUint16(v, cfg.MSRBBips)
	v = binary.BigEndian.AppendUint16(v, cfg.COLBips)
	v = binary.BigEndian.AppendUint16(v, cfg.OpsBips)
	return mu.Insert(ctx, FeeRouterConfigKey(), v)
}

func GetFeeRouterConfig(ctx context.Context, im state.Immutable) (FeeRouterConfig, error) {
	v, err := im.GetValue(ctx, FeeRouterConfigKey())
	if errors.Is(err, database.ErrNotFound) {
		return FeeRouterConfig{}, ErrInvalidFeeRouterConfig
	}
	if err != nil {
		return FeeRouterConfig{}, err
	}
	minLen := consts.Uint16Len * 3
	if len(v) < minLen {
		return FeeRouterConfig{}, fmt.Errorf("%w: fee router config length %d < %d", ErrInvalidFeeRouterConfig, len(v), minLen)
	}
	cfg := FeeRouterConfig{
		MSRBBips: binary.BigEndian.Uint16(v[:consts.Uint16Len]),
		COLBips:  binary.BigEndian.Uint16(v[consts.Uint16Len : consts.Uint16Len*2]),
		OpsBips:  binary.BigEndian.Uint16(v[consts.Uint16Len*2 : consts.Uint16Len*3]),
	}
	if uint64(cfg.MSRBBips)+uint64(cfg.COLBips)+uint64(cfg.OpsBips) != bipsDenominator {
		return FeeRouterConfig{}, fmt.Errorf("%w: sum=%d", ErrInvalidFeeRouterConfig, uint64(cfg.MSRBBips)+uint64(cfg.COLBips)+uint64(cfg.OpsBips))
	}
	return cfg, nil
}

func PutFeeRouterState(ctx context.Context, mu state.Mutable, s FeeRouterState) error {
	v := make([]byte, 0, consts.Uint64Len*3)
	v = binary.BigEndian.AppendUint64(v, s.MSRBBudget)
	v = binary.BigEndian.AppendUint64(v, s.COLBudget)
	v = binary.BigEndian.AppendUint64(v, s.OpsBudget)
	return mu.Insert(ctx, FeeRouterStateKey(), v)
}

func GetFeeRouterState(ctx context.Context, im state.Immutable) (FeeRouterState, error) {
	v, err := im.GetValue(ctx, FeeRouterStateKey())
	if errors.Is(err, database.ErrNotFound) {
		return FeeRouterState{}, ErrInvalidFeeRouterConfig
	}
	if err != nil {
		return FeeRouterState{}, err
	}
	minLen := consts.Uint64Len * 3
	if len(v) < minLen {
		return FeeRouterState{}, fmt.Errorf("%w: fee router state length %d < %d", ErrInvalidFeeRouterConfig, len(v), minLen)
	}
	return FeeRouterState{
		MSRBBudget: binary.BigEndian.Uint64(v[:consts.Uint64Len]),
		COLBudget:  binary.BigEndian.Uint64(v[consts.Uint64Len : consts.Uint64Len*2]),
		OpsBudget:  binary.BigEndian.Uint64(v[consts.Uint64Len*2 : consts.Uint64Len*3]),
	}, nil
}

func PutVAIConfig(ctx context.Context, mu state.Mutable, cfg VAIConfig) error {
	v := make([]byte, 0, codec.AddressLen+consts.Uint64Len*3)
	v = append(v, cfg.MintAuthority[:]...)
	v = binary.BigEndian.AppendUint64(v, cfg.DebtCeiling)
	v = binary.BigEndian.AppendUint64(v, cfg.EpochMintLimit)
	v = binary.BigEndian.AppendUint64(v, uint64(cfg.MintEpochSeconds))
	return mu.Insert(ctx, VAIConfigKey(), v)
}

func GetVAIConfig(ctx context.Context, im state.Immutable) (VAIConfig, error) {
	v, err := im.GetValue(ctx, VAIConfigKey())
	if errors.Is(err, database.ErrNotFound) {
		return VAIConfig{}, ErrInvalidTokenomicsConfig
	}
	if err != nil {
		return VAIConfig{}, err
	}
	minLen := codec.AddressLen + consts.Uint64Len*3
	if len(v) < minLen {
		return VAIConfig{}, fmt.Errorf("%w: VAI config length %d < %d", ErrInvalidTokenomicsConfig, len(v), minLen)
	}
	var cfg VAIConfig
	copy(cfg.MintAuthority[:], v[:codec.AddressLen])
	offset := codec.AddressLen
	cfg.DebtCeiling = binary.BigEndian.Uint64(v[offset : offset+consts.Uint64Len])
	offset += consts.Uint64Len
	cfg.EpochMintLimit = binary.BigEndian.Uint64(v[offset : offset+consts.Uint64Len])
	offset += consts.Uint64Len
	cfg.MintEpochSeconds = int64(binary.BigEndian.Uint64(v[offset : offset+consts.Uint64Len]))
	return cfg, nil
}

func PutVAIState(ctx context.Context, mu state.Mutable, s VAIState) error {
	v := make([]byte, 0, consts.Uint64Len*3)
	v = binary.BigEndian.AppendUint64(v, s.TotalDebt)
	v = binary.BigEndian.AppendUint64(v, uint64(s.EpochStartUnix))
	v = binary.BigEndian.AppendUint64(v, s.EpochMinted)
	return mu.Insert(ctx, VAIStateKey(), v)
}

func GetVAIState(ctx context.Context, im state.Immutable) (VAIState, error) {
	v, err := im.GetValue(ctx, VAIStateKey())
	if errors.Is(err, database.ErrNotFound) {
		return VAIState{}, ErrInvalidTokenomicsConfig
	}
	if err != nil {
		return VAIState{}, err
	}
	minLen := consts.Uint64Len * 3
	if len(v) < minLen {
		return VAIState{}, fmt.Errorf("%w: VAI state length %d < %d", ErrInvalidTokenomicsConfig, len(v), minLen)
	}
	return VAIState{
		TotalDebt:      binary.BigEndian.Uint64(v[:consts.Uint64Len]),
		EpochStartUnix: int64(binary.BigEndian.Uint64(v[consts.Uint64Len : consts.Uint64Len*2])),
		EpochMinted:    binary.BigEndian.Uint64(v[consts.Uint64Len*2 : consts.Uint64Len*3]),
	}, nil
}

func validateRiskConfig(cfg RiskConfig) error {
	if cfg.BackingFloorBips == 0 {
		return ErrInvalidRiskConfig
	}
	if uint64(cfg.VEILLtvBips) > bipsDenominator ||
		uint64(cfg.WVEILLtvBips) > bipsDenominator ||
		uint64(cfg.WSVEILLtvBips) > bipsDenominator {
		return ErrInvalidRiskConfig
	}
	if uint64(cfg.VEILHaircutBips) > bipsDenominator ||
		uint64(cfg.WVEILHaircutBips) > bipsDenominator ||
		uint64(cfg.WSVEILHaircutBips) > bipsDenominator {
		return ErrInvalidRiskConfig
	}
	// v1 hard rule: wsVEIL is not collateral-eligible.
	if cfg.WSVEILLtvBips != 0 {
		return ErrInvalidRiskConfig
	}
	return nil
}

func PutRiskConfig(ctx context.Context, mu state.Mutable, cfg RiskConfig) error {
	if err := validateRiskConfig(cfg); err != nil {
		return err
	}
	v := make([]byte, 0, consts.Uint32Len+consts.Uint16Len*6)
	v = binary.BigEndian.AppendUint32(v, cfg.BackingFloorBips)
	v = binary.BigEndian.AppendUint16(v, cfg.VEILLtvBips)
	v = binary.BigEndian.AppendUint16(v, cfg.WVEILLtvBips)
	v = binary.BigEndian.AppendUint16(v, cfg.WSVEILLtvBips)
	v = binary.BigEndian.AppendUint16(v, cfg.VEILHaircutBips)
	v = binary.BigEndian.AppendUint16(v, cfg.WVEILHaircutBips)
	v = binary.BigEndian.AppendUint16(v, cfg.WSVEILHaircutBips)
	return mu.Insert(ctx, RiskConfigKey(), v)
}

func GetRiskConfig(ctx context.Context, im state.Immutable) (RiskConfig, error) {
	v, err := im.GetValue(ctx, RiskConfigKey())
	if errors.Is(err, database.ErrNotFound) {
		return RiskConfig{}, ErrInvalidRiskConfig
	}
	if err != nil {
		return RiskConfig{}, err
	}
	minLen := consts.Uint32Len + consts.Uint16Len*6
	if len(v) < minLen {
		return RiskConfig{}, ErrInvalidRiskConfig
	}
	cfg := RiskConfig{
		BackingFloorBips:  binary.BigEndian.Uint32(v[:consts.Uint32Len]),
		VEILLtvBips:       binary.BigEndian.Uint16(v[consts.Uint32Len : consts.Uint32Len+consts.Uint16Len]),
		WVEILLtvBips:      binary.BigEndian.Uint16(v[consts.Uint32Len+consts.Uint16Len : consts.Uint32Len+consts.Uint16Len*2]),
		WSVEILLtvBips:     binary.BigEndian.Uint16(v[consts.Uint32Len+consts.Uint16Len*2 : consts.Uint32Len+consts.Uint16Len*3]),
		VEILHaircutBips:   binary.BigEndian.Uint16(v[consts.Uint32Len+consts.Uint16Len*3 : consts.Uint32Len+consts.Uint16Len*4]),
		WVEILHaircutBips:  binary.BigEndian.Uint16(v[consts.Uint32Len+consts.Uint16Len*4 : consts.Uint32Len+consts.Uint16Len*5]),
		WSVEILHaircutBips: binary.BigEndian.Uint16(v[consts.Uint32Len+consts.Uint16Len*5 : consts.Uint32Len+consts.Uint16Len*6]),
	}
	if err := validateRiskConfig(cfg); err != nil {
		return RiskConfig{}, err
	}
	return cfg, nil
}

func PutReserveState(ctx context.Context, mu state.Mutable, s ReserveState) error {
	v := make([]byte, 0, consts.Uint64Len*2)
	v = binary.BigEndian.AppendUint64(v, s.ExogenousReserve)
	v = binary.BigEndian.AppendUint64(v, s.VAIBuffer)
	return mu.Insert(ctx, ReserveStateKey(), v)
}

func GetReserveState(ctx context.Context, im state.Immutable) (ReserveState, error) {
	v, err := im.GetValue(ctx, ReserveStateKey())
	if errors.Is(err, database.ErrNotFound) {
		return ReserveState{}, ErrInvalidReserveState
	}
	if err != nil {
		return ReserveState{}, err
	}
	minLen := consts.Uint64Len * 2
	if len(v) < minLen {
		return ReserveState{}, ErrInvalidReserveState
	}
	return ReserveState{
		ExogenousReserve: binary.BigEndian.Uint64(v[:consts.Uint64Len]),
		VAIBuffer:        binary.BigEndian.Uint64(v[consts.Uint64Len : consts.Uint64Len*2]),
	}, nil
}

func validateProofConfig(cfg ProofConfig) error {
	if cfg.RequiredProofType != mconsts.ProofTypeGroth16 && cfg.RequiredProofType != mconsts.ProofTypePlonk {
		return ErrInvalidProofConfig
	}
	if cfg.BatchWindowMs <= 0 || cfg.ProofDeadlineMs <= 0 {
		return ErrInvalidProofConfig
	}
	var zero codec.Address
	if cfg.ProverAuthority == zero {
		return ErrInvalidProofConfig
	}
	return nil
}

func PutProofConfig(ctx context.Context, mu state.Mutable, cfg ProofConfig) error {
	if err := validateProofConfig(cfg); err != nil {
		return err
	}
	v := make([]byte, 0, 1+1+consts.Uint64Len+consts.Uint64Len+codec.AddressLen)
	if cfg.RequireProof {
		v = append(v, 1)
	} else {
		v = append(v, 0)
	}
	v = append(v, cfg.RequiredProofType)
	v = binary.BigEndian.AppendUint64(v, uint64(cfg.BatchWindowMs))
	v = binary.BigEndian.AppendUint64(v, uint64(cfg.ProofDeadlineMs))
	v = append(v, cfg.ProverAuthority[:]...)
	return mu.Insert(ctx, ProofConfigKey(), v)
}

func GetProofConfig(ctx context.Context, im state.Immutable) (ProofConfig, error) {
	v, err := im.GetValue(ctx, ProofConfigKey())
	if errors.Is(err, database.ErrNotFound) {
		return ProofConfig{}, ErrInvalidProofConfig
	}
	if err != nil {
		return ProofConfig{}, err
	}
	minLen := 1 + 1 + consts.Uint64Len + consts.Uint64Len + codec.AddressLen
	if len(v) < minLen {
		return ProofConfig{}, ErrInvalidProofConfig
	}
	cfg := ProofConfig{
		RequireProof:      v[0] == 1,
		RequiredProofType: v[1],
		BatchWindowMs:     int64(binary.BigEndian.Uint64(v[2 : 2+consts.Uint64Len])),
		ProofDeadlineMs:   int64(binary.BigEndian.Uint64(v[2+consts.Uint64Len : 2+consts.Uint64Len*2])),
	}
	copy(cfg.ProverAuthority[:], v[2+consts.Uint64Len*2:])
	if err := validateProofConfig(cfg); err != nil {
		return ProofConfig{}, err
	}
	return cfg, nil
}

func PutBatchProofRecord(ctx context.Context, mu state.Mutable, marketID ids.ID, windowID uint64, record BatchProofRecord) error {
	const (
		maxPublicInputsHashLen = 32
		maxFillsHashLen        = 64
	)
	if len(record.PublicInputsHash) == 0 || len(record.PublicInputsHash) > maxPublicInputsHashLen {
		return ErrInvalidProofEnvelope
	}
	if len(record.FillsHash) == 0 || len(record.FillsHash) > maxFillsHashLen {
		return ErrInvalidProofEnvelope
	}
	k := BatchProofKey(marketID, windowID)
	v := make([]byte, 0, 1+consts.Uint64Len+consts.Uint64Len+codec.AddressLen+32+consts.Uint16Len+len(record.PublicInputsHash)+consts.Uint16Len+len(record.FillsHash))
	v = append(v, record.ProofType)
	v = binary.BigEndian.AppendUint64(v, uint64(record.SubmittedAtMs))
	v = binary.BigEndian.AppendUint64(v, uint64(record.WindowCloseAtMs))
	v = append(v, record.Prover[:]...)
	v = append(v, record.ProofCommitment[:]...)
	v = binary.BigEndian.AppendUint16(v, uint16(len(record.PublicInputsHash)))
	v = append(v, record.PublicInputsHash...)
	v = binary.BigEndian.AppendUint16(v, uint16(len(record.FillsHash)))
	v = append(v, record.FillsHash...)
	return mu.Insert(ctx, k, v)
}

func GetBatchProofRecord(ctx context.Context, im state.Immutable, marketID ids.ID, windowID uint64) (BatchProofRecord, error) {
	const (
		maxPublicInputsHashLen = 32
		maxFillsHashLen        = 64
	)
	k := BatchProofKey(marketID, windowID)
	v, err := im.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return BatchProofRecord{}, ErrProofNotFound
	}
	if err != nil {
		return BatchProofRecord{}, err
	}
	minLen := 1 + consts.Uint64Len + consts.Uint64Len + codec.AddressLen + 32 + consts.Uint16Len + consts.Uint16Len
	if len(v) < minLen {
		return BatchProofRecord{}, ErrInvalidProofEnvelope
	}
	rec := BatchProofRecord{
		ProofType:       v[0],
		SubmittedAtMs:   int64(binary.BigEndian.Uint64(v[1 : 1+consts.Uint64Len])),
		WindowCloseAtMs: int64(binary.BigEndian.Uint64(v[1+consts.Uint64Len : 1+consts.Uint64Len*2])),
	}
	offset := 1 + consts.Uint64Len*2
	copy(rec.Prover[:], v[offset:offset+codec.AddressLen])
	offset += codec.AddressLen
	copy(rec.ProofCommitment[:], v[offset:offset+32])
	offset += 32

	if len(v[offset:]) < consts.Uint16Len {
		return BatchProofRecord{}, ErrInvalidProofEnvelope
	}
	pihLen := int(binary.BigEndian.Uint16(v[offset : offset+consts.Uint16Len]))
	offset += consts.Uint16Len
	if pihLen <= 0 || pihLen > maxPublicInputsHashLen || len(v[offset:]) < pihLen+consts.Uint16Len {
		return BatchProofRecord{}, ErrInvalidProofEnvelope
	}
	rec.PublicInputsHash = append([]byte(nil), v[offset:offset+pihLen]...)
	offset += pihLen

	fillsLen := int(binary.BigEndian.Uint16(v[offset : offset+consts.Uint16Len]))
	offset += consts.Uint16Len
	if fillsLen <= 0 || fillsLen > maxFillsHashLen || len(v[offset:]) < fillsLen {
		return BatchProofRecord{}, ErrInvalidProofEnvelope
	}
	rec.FillsHash = append([]byte(nil), v[offset:offset+fillsLen]...)
	return rec, nil
}

func PutVellumProof(ctx context.Context, mu state.Mutable, marketID ids.ID, windowID uint64, proof []byte) error {
	if len(proof) == 0 || len(proof) > maxVellumProofBytes {
		return ErrInvalidVellumProof
	}
	k := VellumProofKey(marketID, windowID)
	v := make([]byte, 0, consts.Uint32Len+len(proof))
	v = binary.BigEndian.AppendUint32(v, uint32(len(proof)))
	v = append(v, proof...)
	return mu.Insert(ctx, k, v)
}

func GetVellumProof(ctx context.Context, im state.Immutable, marketID ids.ID, windowID uint64) ([]byte, error) {
	k := VellumProofKey(marketID, windowID)
	v, err := im.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, ErrVellumProofNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(v) < consts.Uint32Len {
		return nil, ErrInvalidVellumProof
	}
	proofLen := int(binary.BigEndian.Uint32(v[:consts.Uint32Len]))
	if proofLen <= 0 || proofLen > maxVellumProofBytes || len(v) != consts.Uint32Len+proofLen {
		return nil, ErrInvalidVellumProof
	}
	return append([]byte(nil), v[consts.Uint32Len:]...), nil
}

func GetVellumProofFromState(ctx context.Context, f ReadState, marketID ids.ID, windowID uint64) ([]byte, error) {
	k := VellumProofKey(marketID, windowID)
	values, errs := f(ctx, [][]byte{k})
	if errors.Is(errs[0], database.ErrNotFound) {
		return nil, ErrVellumProofNotFound
	}
	if errs[0] != nil {
		return nil, errs[0]
	}
	v := values[0]
	if len(v) < consts.Uint32Len {
		return nil, ErrInvalidVellumProof
	}
	proofLen := int(binary.BigEndian.Uint32(v[:consts.Uint32Len]))
	if proofLen <= 0 || proofLen > maxVellumProofBytes || len(v) != consts.Uint32Len+proofLen {
		return nil, ErrInvalidVellumProof
	}
	return append([]byte(nil), v[consts.Uint32Len:]...), nil
}

func PutBloodsworn(ctx context.Context, mu state.Mutable, addr codec.Address, bloodsworn Bloodsworn) error {
	v := make([]byte, 0, consts.Uint64Len+consts.Uint64Len+consts.Uint64Len+consts.Uint32Len)
	v = binary.BigEndian.AppendUint64(v, bloodsworn.TotalAcceptedProofs)
	v = binary.BigEndian.AppendUint64(v, bloodsworn.ActiveStreak)
	v = binary.BigEndian.AppendUint64(v, uint64(bloodsworn.LastProofAtMs))
	v = binary.BigEndian.AppendUint32(v, bloodsworn.ScarCount)
	return mu.Insert(ctx, BloodswornKey(addr), v)
}

func GetBloodsworn(ctx context.Context, im state.Immutable, addr codec.Address) (Bloodsworn, error) {
	v, err := im.GetValue(ctx, BloodswornKey(addr))
	if errors.Is(err, database.ErrNotFound) {
		return Bloodsworn{}, nil
	}
	if err != nil {
		return Bloodsworn{}, err
	}
	minLen := consts.Uint64Len + consts.Uint64Len + consts.Uint64Len + consts.Uint32Len
	if len(v) < minLen {
		return Bloodsworn{}, ErrInvalidBloodsworn
	}
	return Bloodsworn{
		TotalAcceptedProofs: binary.BigEndian.Uint64(v[:consts.Uint64Len]),
		ActiveStreak:        binary.BigEndian.Uint64(v[consts.Uint64Len : consts.Uint64Len*2]),
		LastProofAtMs:       int64(binary.BigEndian.Uint64(v[consts.Uint64Len*2 : consts.Uint64Len*3])),
		ScarCount:           binary.BigEndian.Uint32(v[consts.Uint64Len*3 : consts.Uint64Len*3+consts.Uint32Len]),
	}, nil
}

func GetBloodswornFromState(ctx context.Context, f ReadState, addr codec.Address) (Bloodsworn, error) {
	k := BloodswornKey(addr)
	values, errs := f(ctx, [][]byte{k})
	if errors.Is(errs[0], database.ErrNotFound) {
		return Bloodsworn{}, nil
	}
	if errs[0] != nil {
		return Bloodsworn{}, errs[0]
	}
	v := values[0]
	minLen := consts.Uint64Len + consts.Uint64Len + consts.Uint64Len + consts.Uint32Len
	if len(v) < minLen {
		return Bloodsworn{}, ErrInvalidBloodsworn
	}
	return Bloodsworn{
		TotalAcceptedProofs: binary.BigEndian.Uint64(v[:consts.Uint64Len]),
		ActiveStreak:        binary.BigEndian.Uint64(v[consts.Uint64Len : consts.Uint64Len*2]),
		LastProofAtMs:       int64(binary.BigEndian.Uint64(v[consts.Uint64Len*2 : consts.Uint64Len*3])),
		ScarCount:           binary.BigEndian.Uint32(v[consts.Uint64Len*3 : consts.Uint64Len*3+consts.Uint32Len]),
	}, nil
}

func PutGlyph(ctx context.Context, mu state.Mutable, marketID ids.ID, windowID uint64, glyph Glyph) error {
	if glyph.Class == 0 || glyph.Rarity == 0 || glyph.CreatedAtMs <= 0 {
		return ErrInvalidGlyph
	}
	k := GlyphKey(marketID, windowID)
	v := make([]byte, 0, 1+1+consts.Uint64Len+codec.AddressLen+32+32+32)
	v = append(v, glyph.Class)
	v = append(v, glyph.Rarity)
	v = binary.BigEndian.AppendUint64(v, uint64(glyph.CreatedAtMs))
	v = append(v, glyph.Prover[:]...)
	v = append(v, glyph.ProofCommitment[:]...)
	v = append(v, glyph.PublicInputsHash[:]...)
	v = append(v, glyph.Entropy[:]...)
	return mu.Insert(ctx, k, v)
}

func GetGlyph(ctx context.Context, im state.Immutable, marketID ids.ID, windowID uint64) (Glyph, error) {
	k := GlyphKey(marketID, windowID)
	v, err := im.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return Glyph{}, ErrGlyphNotFound
	}
	if err != nil {
		return Glyph{}, err
	}
	minLen := 1 + 1 + consts.Uint64Len + codec.AddressLen + 32 + 32 + 32
	if len(v) < minLen {
		return Glyph{}, ErrInvalidGlyph
	}
	glyph := Glyph{
		Class:       v[0],
		Rarity:      v[1],
		CreatedAtMs: int64(binary.BigEndian.Uint64(v[2 : 2+consts.Uint64Len])),
	}
	offset := 2 + consts.Uint64Len
	copy(glyph.Prover[:], v[offset:offset+codec.AddressLen])
	offset += codec.AddressLen
	copy(glyph.ProofCommitment[:], v[offset:offset+32])
	offset += 32
	copy(glyph.PublicInputsHash[:], v[offset:offset+32])
	offset += 32
	copy(glyph.Entropy[:], v[offset:offset+32])
	return glyph, nil
}

func GetGlyphFromState(ctx context.Context, f ReadState, marketID ids.ID, windowID uint64) (Glyph, error) {
	k := GlyphKey(marketID, windowID)
	values, errs := f(ctx, [][]byte{k})
	if errors.Is(errs[0], database.ErrNotFound) {
		return Glyph{}, ErrGlyphNotFound
	}
	if errs[0] != nil {
		return Glyph{}, errs[0]
	}
	v := values[0]
	minLen := 1 + 1 + consts.Uint64Len + codec.AddressLen + 32 + 32 + 32
	if len(v) < minLen {
		return Glyph{}, ErrInvalidGlyph
	}
	glyph := Glyph{
		Class:       v[0],
		Rarity:      v[1],
		CreatedAtMs: int64(binary.BigEndian.Uint64(v[2 : 2+consts.Uint64Len])),
	}
	offset := 2 + consts.Uint64Len
	copy(glyph.Prover[:], v[offset:offset+codec.AddressLen])
	offset += codec.AddressLen
	copy(glyph.ProofCommitment[:], v[offset:offset+32])
	offset += 32
	copy(glyph.PublicInputsHash[:], v[offset:offset+32])
	offset += 32
	copy(glyph.Entropy[:], v[offset:offset+32])
	return glyph, nil
}

func GetVAIBalance(ctx context.Context, im state.Immutable, addr codec.Address) (uint64, error) {
	_, bal, _, err := getVAIBalance(ctx, im, addr)
	return bal, err
}

func GetVAIBalanceFromState(ctx context.Context, f ReadState, addr codec.Address) (uint64, error) {
	k := VAIBalanceKey(addr)
	values, errs := f(ctx, [][]byte{k})
	bal, _, err := innerGetBalance(values[0], errs[0])
	return bal, err
}

func getVAIBalance(ctx context.Context, im state.Immutable, addr codec.Address) ([]byte, uint64, bool, error) {
	k := VAIBalanceKey(addr)
	bal, exists, err := innerGetBalance(im.GetValue(ctx, k))
	return k, bal, exists, err
}

func setVAIBalance(ctx context.Context, mu state.Mutable, key []byte, balance uint64) error {
	return mu.Insert(ctx, key, binary.BigEndian.AppendUint64(nil, balance))
}

func AddVAIBalance(ctx context.Context, mu state.Mutable, addr codec.Address, amount uint64) (uint64, error) {
	key, bal, _, err := getVAIBalance(ctx, mu, addr)
	if err != nil {
		return 0, err
	}
	nbal, err := smath.Add(bal, amount)
	if err != nil {
		return 0, fmt.Errorf("%w: could not add VAI balance (bal=%d, addr=%v, amount=%d)", ErrInvalidBalance, bal, addr, amount)
	}
	return nbal, setVAIBalance(ctx, mu, key, nbal)
}

func SubVAIBalance(ctx context.Context, mu state.Mutable, addr codec.Address, amount uint64) (uint64, error) {
	key, bal, ok, err := getVAIBalance(ctx, mu, addr)
	if !ok {
		return 0, fmt.Errorf("%w: could not subtract VAI (bal=%d, addr=%v, amount=%d)", ErrInvalidBalance, 0, addr, amount)
	}
	if err != nil {
		return 0, err
	}
	nbal, err := smath.Sub(bal, amount)
	if err != nil {
		return 0, fmt.Errorf("%w: could not subtract VAI balance (bal=%d < amount=%d, gap=%d, addr=%v)", ErrInvalidBalance, bal, amount, amount-bal, addr)
	}
	if nbal == 0 {
		return 0, mu.Remove(ctx, key)
	}
	return nbal, setVAIBalance(ctx, mu, key, nbal)
}
