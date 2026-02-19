# VEIL Whitepaper Alignment Matrix

Status: Final check matrix  
Date: 2026-02-19  
Purpose: Ensure implementation and launch gates match the published VEIL whitepaper claims.

## 1. Technical Architecture (Part I)

| Whitepaper Commitment | Required Implementation | Source Artifact |
|---|---|---|
| Threshold-encrypted mempool | Encrypted order intake only in production mode; decrypt shares required post-finalization | `VEIL_V1_NATIVE_PRIVACY_SPEC.md` |
| Proof-gated consensus validity | Batch proof required before market-state transition; default `5s` window, `10s` proof deadline, deterministic prover committee | `VEIL_V1_NATIVE_PRIVACY_SPEC.md` |
| Shielded ledger (commitment/nullifier + ZK) | Commitment/nullifier state machine, nullifier uniqueness checks, proof-gated execution | `VEIL_V1_NATIVE_PRIVACY_SPEC.md` |
| Uniform batch auctions | Deterministic 2-5s windows, single clearing price per window | `VEIL_V1_NATIVE_PRIVACY_SPEC.md` |
| VRF-selected oracle committees + BLS attestations | Deterministic committee selection, threshold BLS attest, dispute window | `VEIL_V1_NATIVE_PRIVACY_SPEC.md` |
| Objective slashing | Machine-verifiable faults only, explicit slash schedule | `VEIL_V1_NATIVE_PRIVACY_SPEC.md` |
| Deterministic replay + selective disclosure | Replay artifacts mandatory; request-scoped disclosure proofs | `VEIL_V1_NATIVE_PRIVACY_SPEC.md` |
| SLO-backed market quality | Latency/depth/privacy/uptime/proving targets measured and monitored | `VEIL_V1_NATIVE_PRIVACY_SPEC.md` |

## 2. Token Economics (Part II)

| Whitepaper Commitment | Required Implementation | Source Artifact |
|---|---|---|
| Fee router 70/20/10 | Enforce 70% MSRB, 20% COL buyback-and-make, 10% operations | `VEIL_EXECUTION_PACKAGE.md` |
| Compounding liquidity model (POL/COL) | Track deployable vs locked treasury, NAV, budgeted interventions | `VEIL_EXECUTION_PACKAGE.md` |
| Native liquidity rails | VM-level UniV2-style pool primitives for VEIL/VAI liquidity and LP accounting | `VEIL_EXECUTION_PACKAGE.md` |
| Stablecoin hard-backing discipline | Mint gate enforces exogenous reserve floor; VAI/VEIL-family assets excluded from hard backing metric | `VEIL_EXECUTION_PACKAGE.md` |
| Collateral eligibility policy | VEIL + WVEIL risk-gated; vVEIL collateral disabled in v1 (`LTV=0`) | `VEIL_EXECUTION_PACKAGE.md` |
| No rent-seeking emissions dependency | Fixed supply policy + explicit governance/timelock controls | `VEIL_EXECUTION_PACKAGE.md` |
| Low-float launch financing discipline | Encode `990,999,000` total supply with `5%` seed+presale cap and deterministic treasury lock posture | `VEIL_EXECUTION_PACKAGE.md` |
| Staking yield anti-dump controls | Bounded vVEIL APY bands, APY hard cap, annual emission cap, and cooldown enforcement | `VEIL_EXECUTION_PACKAGE.md` |
| Operator skin-in-the-game | VEIL-denominated slashable bond and objective penalties | `VEIL_V1_NATIVE_PRIVACY_SPEC.md` |
| Governance control over economic params | Timelocked, bounded parameter updates only | `VEIL_EXECUTION_PACKAGE.md` |

## 2.1 Interop and EVM Companion Requirements

These are required for full ecosystem usability when VEIL includes EVM-facing rails:

| Capability | Scope | Launch Requirement | Source Artifact |
|---|---|---|---|
| Avalanche Warp Messaging (`AWM`) + Teleporter | Cross-chain bridge/messaging | MUST be deployed and integration-tested before bridge-enabled launch | `VEIL_COMPANION_EVM_PRIMITIVES_CHECKLIST.md` |
| `NativeMinter` | Companion EVM chain only | MUST be enabled if mint/burn flows are used for bridged native wrappers | `VEIL_COMPANION_EVM_PRIMITIVES_CHECKLIST.md` |
| `FeeConfigManager` | Companion EVM chain only | SHOULD be enabled to avoid hard-forking for fee tuning | `VEIL_COMPANION_EVM_PRIMITIVES_CHECKLIST.md` |
| `ContractDeployerAllowList` | Companion EVM chain only | MUST be enabled for permissioned deployment posture | `VEIL_COMPANION_EVM_PRIMITIVES_CHECKLIST.md` |
| `TxAllowList` | Companion EVM chain only | MUST be enabled for permissioned transaction posture | `VEIL_COMPANION_EVM_PRIMITIVES_CHECKLIST.md` |
| `Multicall3` + `CREATE2` deployer + faucet | Companion EVM chain only | SHOULD be present for tooling and dev UX | `VEIL_COMPANION_EVM_PRIMITIVES_CHECKLIST.md` |

Core boundary rule:

- VeilVM remains the source of truth for privacy, proof validity, treasury/COL controls, and VAI risk invariants.
- EVM companion features are interoperability and access rails, not substitutes for VeilVM consensus invariants.

## 3. Low-Float and Anti-Dump Requirements

These are consistent with whitepaper goals and required for launch posture:

- fixed total supply at genesis
- most supply locked in protocol-controlled COL vault state
- deterministic tranche releases with hard per-epoch caps
- immutable no-drain rule on locked COL vault
- public accounting for float, unlock schedule, and COL NAV

Primary source: `VEIL_EXECUTION_PACKAGE.md`

## 4. Genesis-Level Implementation Checks

Must be true before mainnet launch:

1. All allocation values are explicit and sum exactly to total supply.
2. Locked COL state is encoded directly in genesis.
3. Release schedule and risk caps are encoded in genesis params.
4. Fee router defaults are encoded in genesis params.
5. Genesis validation output is archived and reproducible.
6. Companion EVM primitive checklist is green (for bridge-enabled launch mode).

Primary source: `VEIL_MASTER_RUNBOOK.md`

## 5. Open Risks To Close Before Launch

Current blockers to resolve before protocol claims are operational:

1. Groth16/PLONK verifier wiring is present, but strict runtime currently rejects purported valid Groth16 envelopes due witness-length mismatch (`32 vs 33`).
2. Full shielded-ledger circuit suite is not yet the active launch path (`clearhash-v1` trial circuit is still in use).
3. Long-run 4-6s benchmark evidence and failure-drill artifacts are still pending.
4. Full stress/simulation pass for COL and RBS risk controls is still pending.
5. Companion EVM primitive rollout remains a launch gate for bridge-enabled mode (current Teleporter/bridge deploys are placeholders).

Primary source: `VEIL_MASTER_RUNBOOK.md`

## 6. Final Alignment Verdict

The finalized docs are internally aligned with the whitepaper if and only if all launch gates pass:

- `VEIL_V1_NATIVE_PRIVACY_SPEC.md`
- `VEIL_EXECUTION_PACKAGE.md`
- `VEIL_MASTER_RUNBOOK.md`

This matrix should be used as the sign-off checklist for implementation reviews and governance launch approval.
