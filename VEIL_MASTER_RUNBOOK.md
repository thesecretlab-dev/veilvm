# VEIL Master Runbook (Execution Order)

Status: Active  
Date: 2026-02-19  
Owner: Protocol team

## 1. Goal

Ship VEIL as a privacy-native Avalanche L1 where:

- order flow is not visible before finalization
- balances are shielded with commitment/nullifier + ZK validity
- execution is fair (uniform batch auctions)
- tokenomics are anti-dump by design (low float, locked COL, deterministic unlocks)

## 2. Source Documents

Use these as the implementation stack in order:

1. `VEIL_V1_NATIVE_PRIVACY_SPEC.md` (protocol and consensus requirements)
2. `VEIL_EXECUTION_PACKAGE.md` (tokenomics, COL, risk controls, launch gates)
3. `VEIL_MASTER_RUNBOOK.md` (this execution order and ownership map)
4. `VEIL_PRODUCTION_LAUNCH_CHECKLIST.md` (hard production go/no-go gates and sign-off board)
5. `VEIL_COMPANION_EVM_PRIMITIVES_CHECKLIST.md` (bridge/EVM primitive rollout gates)
6. `VEIL_ZK_CONSENSUS_4_6S_TRIAL_PROFILE.md` (proof-required 4-6s latency trial override)
7. `VEIL_HANDOFF_2026-02-19.md` (project-wide current-state handoff)

## 3. Current Critical Blockers

## 3.1 Resolved Since Last Review

- Local VEIL VM runtime now boots cleanly in the local profile.
- RPCChainVM plugin handshake succeeds.
- VEIL chain reaches healthy block production.
- Local setup flow supports subnet-auth signing path and end-to-end chain creation.
- Local verifier runtime now logs effective ZK config at startup (`enabled/strict/vk/circuit`).
- Local strict fallback now auto-enables Groth16 verification when fixture VK is mounted in-container.
- `veilvm-zkbench` now validates tx execution success from indexer, preventing false-positive bench passes.

## 3.2 Active Blockers

- Full production assurance package for shielded-ledger circuits is incomplete (spec hash + VK hash + proof vectors + security sign-off).
- 4-6s profile needs sustained benchmark evidence (high-window and long-run reports) after local launch-gate PASS.
- Remaining adversarial proof-path drills (malformed proofs and backup prover takeover) must be executed and archived.
- Companion EVM bridge path still uses placeholder Teleporter/bridge contracts and is not production-ready.

## 4. Execution Plan

## Phase 0: Chain Bring-Up (Must pass before feature work)

Status: complete for local VEIL bring-up baseline.

1. Freeze compatible versions for:
   - AvalancheGo image
   - HyperSDK dependency set
   - VeilVM build target
2. Fix container/runtime ABI mismatch for plugin loading.
3. Validate local setup flow end-to-end on wiped data.
4. Record exact reproducible command sequence in docs.

Deliverable: deterministic local subnet bring-up with VeilVM plugin running.

## Phase 1: Protocol-Critical v1 Invariants

1. Enforce encrypted-mempool only path in production mode.
2. Enforce fail-closed batch execution:
   - no decrypt quorum -> reject batch
   - invalid proof -> reject batch
3. Enforce commitment/nullifier checks and solvency checks.
4. Enforce objective slashing paths only.
5. Enforce deterministic replay artifact outputs.

### Phase 1 Default: Proof-Gated Consensus Mode (v1)

Adopt this baseline unless governance approves a parameter override before launch:

- `batch_window = 5s`
- `proof_deadline = 10s` from batch close
- deterministic prover committee: `5 of 20` with `2` backup provers
- validity rule: no valid proof by deadline -> reject market batch (fail-closed)
- liveness rule: subnet MAY continue non-market/control blocks while market batch is rejected
- objective penalties:
  - missed proof deadline: slash `0.5%`
  - invalid proof submission: slash `5%`
  - repeated misses: jail per slashing policy

For the current low-latency proving trial, use `VEIL_ZK_CONSENSUS_4_6S_TRIAL_PROFILE.md` as the active override profile.

Design intent:

- keep privacy and proof verification inside consensus validity rules
- avoid requiring all 20 validators to generate full proofs every batch
- preserve predictable latency while keeping native privacy guarantees

Deliverable: v1 privacy and safety invariants implemented and test-covered.

## Phase 2: Tokenomics + COL at Genesis

1. Finalize exact genesis allocation values (no placeholders).
2. Encode locked COL vault in VM state at genesis.
3. Allow release only via `ReleaseCOLTranche` with epoch cap.
4. Route fees 70/20/10 on-chain with auditable accounting.
5. Implement Olympus-style RBS policy executor with hard risk gates.
6. Implement native UniV2-style pool primitives (VEIL/VAI first) at VM level.
7. Enforce exogenous-reserve-only backing floor for VAI minting.
8. Keep vVEIL collateral-disabled in v1 (`LTV = 0`) and gate VEIL/WVEIL with risk params.
9. Wire dashboard feeds for float, unlocks, COL NAV, and risk utilization.

Deliverable: anti-dump, low-float launch state enforced by consensus rules.

## Phase 2.5: Interop + EVM Companion Rails

This phase is required if VEIL ships contract rails in parallel to VeilVM.

1. Deploy Avalanche Warp Messaging path (`AWM`) and Teleporter contracts for cross-chain messaging.
2. Define canonical bridge routes for VEIL-family and VAI wrappers between VeilVM and companion EVM rails.
3. On companion EVM chain config, enable and test:
   - `NativeMinter`
   - `FeeConfigManager`
   - `ContractDeployerAllowList`
   - `TxAllowList`
4. Deploy base EVM utility contracts expected by tooling:
   - `WVEIL` wrapper
   - `Multicall3`
   - `CREATE2` deterministic deployer (`0x4e59...B4956C`)
   - faucet service for dev/test environments
5. Freeze chain config timestamps, fee recipients, and admin keys before public launch.
6. Complete every MUST test in `VEIL_COMPANION_EVM_PRIMITIVES_CHECKLIST.md` and archive evidence.
7. Populate `scripts/companion-evm.addresses.json` from template and commit addresses + tx hashes.

Hard split of responsibility:

- VeilVM core enforces privacy, batch validity, COL locks, VAI risk gates, and treasury invariants.
- EVM precompiles and bridge contracts provide interoperability and permissioning rails.
- No critical economic invariant may depend only on companion EVM precompile settings.

## Phase 3: SLO and Launch Readiness

1. Run stress and adversarial simulation suite.
2. Run failure playbooks and governance emergency drills.
3. Verify telemetry and disclosure pipeline.
4. Perform launch gate sign-off.

Deliverable: production readiness package with objective pass/fail evidence.

## 5. VM-Level COL Design (Genesis-First)

## 5.1 Required State Objects

- `COL_VAULT_LOCKED`
- `COL_VAULT_LIVE`
- `COL_RELEASE_SCHEDULE`
- `COL_RISK_LIMITS`
- `FEE_ROUTER_STATE`
- `RBS_POLICY_STATE`

## 5.2 Hard Rules

- No transfer from locked vault except `ReleaseCOLTranche`.
- `ReleaseCOLTranche` must enforce per-epoch max release.
- RBS execution must reject on stale data, cap breaches, or drawdown breach.
- Governance cannot bypass locked-vault no-drain rule.

## 5.3 Genesis Procedure

1. Compute and freeze allocation table.
2. Write allocations and lock flags directly in genesis state.
3. Set release schedule and caps in genesis parameters.
4. Set fee router defaults in genesis parameters.
5. Verify invariant sum equals total supply.
6. Run genesis validation script and archive output hash.

## 6. Launch Interaction Model (VAI + Olympus)

At genesis, the economic core is two native VM modules:

- `VAIEngine` (Maker-style stable issuance)
- `VeilTreasury` (bonding, COL deployment, and RBS control)

They run as one coupled system with three loops:

1. Issuance loop (`VAIEngine`):
   - `MintVAI`, `BurnVAI`, `LiquidateCDP`
   - strict debt ceiling and per-epoch mint throttle
2. Treasury loop (`VeilTreasury`):
   - `BondDeposit`, `BondRedeem`, `DeployCOL`, `RebalanceCOL`, `BuybackMake`
   - vested claims only; no instant liquid VEIL emissions
3. Controller loop (`RiskKernel` + `FeeRouter`):
   - peg/stability controls, stale-oracle rejects, drawdown breakers
   - fee flow kept on-chain and auditable (`70/20/10`)

Launch sequence at T0:

1. Initialize `VAIEngine`, `VeilTreasury`, `RiskKernel`, and `FeeRouter` in genesis state.
2. Keep VEIL float constrained via locked treasury state and deterministic release caps.
3. Allow conservative VAI minting under debt/LTV bounds.
4. Route bond inflows to treasury reserves and deploy COL under strict risk caps.
5. Auto-pause mint/bond/RBS paths on peg breaks, stale oracle, or drawdown breaches.

Hard requirements:

- no direct drain path from locked treasury
- no instant bond payout path
- bounded governance parameter ranges with timelock
- non-reflexive launch preference (exogenous collateral support at genesis)

## 7. Tokenomics Integrity Controls

To preserve low float and runway:

- fixed total supply
- no hidden mint routes
- deterministic unlocks only
- bounded governance parameter ranges
- mandatory timelock on economic changes
- public real-time accounting outputs

### Approved Numeric Baseline (Current)

Use these exact constants unless governance formally revises them before genesis:

- `TOTAL_SUPPLY = 990,999,000 VEIL`
- financing raise target: `$2,000,000`
- sold in seed+presale: `5.0%` (`49,549,950 VEIL`)
- blended financing price: `$0.04036331`
- seed: `2.0%` at `$0.03509853` (`$695,652.17`)
- presale: `3.0%` at `$0.04387316` (`$1,304,347.83`)
- raise usage split: `65%` stability reserve / `25%` COL-RBS / `10%` ops-security
- vVEIL policy bands: `18-24%` (M0-3), `14-18%` (M4-6), `10-14%` (M7-12), `8-12%` steady
- vVEIL hard controls: APY cap `30%`, emission cap `<= 4%` supply/year, unbond cooldown `14 days`

## 8. Testing Program (Minimum)

1. Runtime compatibility and chain boot tests
2. Setup script integration tests on fresh data
3. Batch privacy and proof failure-path tests
4. Nullifier/double-spend rejection tests
5. COL release-cap and no-drain invariant tests
6. RBS stress tests (volatility, stale-oracle, cap breach)
7. Replay determinism and selective-disclosure tests

## 9. Collaboration Rules (Shared Repo)

- Claim files before editing.
- Keep diffs narrow to owned scope.
- Avoid cross-file reformat churn.
- Treat genesis artifacts as coordinated edits.

## 10. Immediate Next Actions

1. Keep verifier mode pinned to `shielded-ledger-v1` in local profile and archive verifier config dumps with each evidence run.
2. Expand from one-window launch-gate PASS to sustained benchmark matrix coverage (32/64/96/128) and publish p50/p95/p99 artifact sets.
3. Execute remaining proof-path failure drills (malformed proof, backup takeover) and capture pass/fail evidence.
4. Package full shielded-ledger circuit assurance artifacts (spec hash, VK hash, vectors, review notes) for gate `G3`.
5. Finalize numeric genesis allocation table for COL strategy and enforce invariants in tests.
6. Complete COLVaultModule + ReleaseCOLTranche hard invariants.
7. Implement RBSModule with strict risk-gated execution.
8. Complete AWM/Teleporter + companion EVM precompile and allowlist rollout tests.
9. Finalize VEIL_COMPANION_EVM_PRIMITIVES_CHECKLIST.md with committed address registry and tx hash evidence.
