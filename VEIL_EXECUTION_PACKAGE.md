# VEIL Execution Package (Low Float, Anti-Dump, Long Runway)

Status: Draft-for-adoption
Date: 2026-02-19
Owner: Protocol team

## 1. Strategic Outcome

Build VEIL so that:

- launch float is intentionally low
- treasury and COL are protocol-controlled, not wallet-discretionary
- there is no fast unlock path that can dump on market participants
- runway is sustained by fee recycling and risk-bounded COL deployment

## 1.1 Implementation Status (2026-02-19)

Completed baseline:

- Local VEIL chain is running with `coreapi` and `veilapi`.
- Proof-gated batch flow is active (`SubmitBatchProof` + fail-closed `ClearBatch` record/commitment/input-hash checks).
- Vellum proof blob storage is active.
- ZK timing instrumentation and benchmark harness are running (`metrics_batch_*.csv`, `summary.md`, `summary.json`).
- Strict verifier runtime is enabled in local profile and synthetic proofs are rejected in fail-closed mode.
- Launch-gate evidence bundle `20260219-184441-launch-gate-evidence` is PASS on shielded-ledger runtime gate.

Pending to reach full whitepaper-grade proof validity:

- Sustained multi-window and long-run shielded-ledger benchmark evidence.
- Full production circuit assurance package (`spec hash`, `VK hash`, vectors, review sign-off).
- Remaining adversarial proof-path drills (malformed proof, backup prover takeover).
- Final safe `max_batch_orders` recommendation under sustained load.

## 2. Non-Negotiable Rules

- Supply is fixed at genesis (no hidden mint path after launch).
- Most supply is locked in VM-enforced COL vault state.
- Unlocks are deterministic and capped per epoch.
- Governance can tune within bounds, but cannot drain locked COL.
- RBS/COL interventions pass the same privacy-preserving execution path.
- Safety controls (drawdown caps, stale-oracle reject, pause) are on-chain and mandatory.

## 3. Token Policy (Recommended v1 Baseline)

Use these as default launch parameters unless governance approves a different set before genesis:

- max launch float: 3-5% of total supply
- locked COL allocation: 80-90% of total supply
- deterministic unlock cadence: epoch-based only
- release cap from locked COL: <= 0.15% of total supply per epoch
- daily COL deploy cap: <= 0.50% of COL NAV
- daily drawdown circuit breaker: pause COL deploy if drawdown > 1.0% of COL NAV
- emergency mode trigger: two consecutive breach days

Token taxonomy (v1):

- `VEIL`: base native asset
- `WVEIL`: non-yield `1:1` wrapped VEIL for EVM bridge/liquidity rails
- `vVEIL`: yield/staking derivative used for emissions and lock/cooldown policy

## 3.1 Approved Launch Financing Baseline

Current approved constants:

- `TOTAL_SUPPLY = 990,999,000 VEIL`
- seed + presale raise target: `$2,000,000`
- total sold at launch financing: `5.0%` (`49,549,950 VEIL`)
- blended sale price (5% sold): `$0.04036331` per VEIL

Seed/presale split (20% seed discount to presale):

- seed tranche: `2.0%` (`19,819,980 VEIL`) at `$0.03509853` -> `$695,652.17`
- presale tranche: `3.0%` (`29,729,970 VEIL`) at `$0.04387316` -> `$1,304,347.83`

Raise usage policy at launch:

- `65%` (`$1,300,000`) -> `VeilTreasury` stability reserve
- `25%` (`$500,000`) -> COL/RBS deployment budgets
- `10%` (`$200,000`) -> operations and security

## 3.2 Approved vVEIL Yield Policy (Anti-Dump Baseline)

- months `0-3`: `18-24%` APY target band (default policy point: `22%`)
- months `4-6`: `14-18%` APY target band (default policy point: `16%`)
- months `7-12`: `10-14%` APY target band (default policy point: `12%`)
- steady state: `8-12%` APY target band (default policy point: `10%`)
- hard APY cap: `30%`
- emission budget cap: `<= 4%` of total supply per year
- unbond cooldown: `14 days`
- APY must be policy-variable and treasury-health dependent; no fixed-yield promise

## 4. Genesis Allocation Table (Must Be Exact Before Launch)

Populate exact values (no placeholders) and enforce sum equals TOTAL_SUPPLY.

- `TOTAL_SUPPLY`
- `COL_VAULT_LOCKED`
- `COL_VAULT_LIVE` (initial deployable tranche)
- `STAKING_BOND_POOL`
- `OPS_TREASURY`
- `ECOSYSTEM_INCENTIVES`
- `CIRCULATING_FLOAT`

Required invariant:

- `COL_VAULT_LOCKED + COL_VAULT_LIVE + STAKING_BOND_POOL + OPS_TREASURY + ECOSYSTEM_INCENTIVES + CIRCULATING_FLOAT == TOTAL_SUPPLY`

## 5. VM Modules Required

- `TokenModule` (fixed supply, transfer constraints)
- `FeeRouterModule` (70/20/10 baseline routing)
- `COLVaultModule` (locked and live tranches, release schedule)
- `RBSModule` (treasury intervention policy and execution)
- `UniV2PoolModule` (native pools, LP accounting, constant-product swaps)
- `RiskModule` (caps, drawdown brakes, stale-data guards)
- `OracleModule` (VRF committee + BLS attestation + dispute window)
- `SlashingModule` (objective faults only)
- `GovernanceModule` (timelock and bounded parameter control)
- `ReplayDisclosureModule` (deterministic replay artifacts + selective disclosure proofs)

## 6. VM Actions Required

- `RouteFees`
- `ReleaseCOLTranche`
- `ExecuteRBSIntervention`
- `RebalanceCOL`
- `PauseCOL`
- `ResumeCOL`
- `SetRBSParams` (timelocked and bounded)
- `SetRiskParams` (timelocked and bounded)
- `SubmitOracleAttestation`
- `SubmitDispute`
- `ApplySlash`
- `FinalizeBatchWithProof`

## 7. Hard Invariants (Consensus-Critical)

- No direct transfer out of `COL_VAULT_LOCKED` except `ReleaseCOLTranche`.
- `ReleaseCOLTranche` rejects if per-epoch cap exceeded.
- `ExecuteRBSIntervention` rejects when:
  - stale oracle
  - quorum/proof failure
  - drawdown cap breached
  - daily deploy cap breached
- Solvency check on every executable batch:
  - `Assets >= Liabilities + required buffers`
- Stablecoin backing invariant:
  - `ExogenousReserve / CirculatingVAI >= backing floor`
  - VAI and VEIL-family assets do not count as hard backing
- Collateral policy invariant (v1):
  - VEIL and WVEIL are collateral-eligible under LTV/haircut limits
  - vVEIL collateral LTV is fixed at 0 in v1
- Any violation causes fail-closed batch rejection.

## 8. Fee Routing and COL Accounting

Baseline router:

- 70% -> MSRB/depth budget
- 20% -> COL buyback-and-make budget
- 10% -> operations budget

Per-epoch accounting outputs:

- COL NAV
- realized and unrealized PnL
- deployed vs undeployed COL
- risk utilization (% of caps)
- treasury runway estimate

## 9. Governance and Key Control

- Timelock for all economic parameter changes (recommend >= 48h).
- Multi-sig/threshold key for governance execution.
- Emergency pause scope: pause deploy and new intake only; no treasury drain privileges.
- Immutable "no-drain" rule for locked COL vault.

## 10. Security and Slashing

Objective slashable conditions only:

- missed decrypt share deadline
- invalid decrypt share
- DKG/key-rotation non-participation
- equivocation/double-sign
- cryptographically provable premature decryption

Slash destination split:

- 50% burn
- 50% insurance/treasury

## 11. Simulation and Validation Program

Must complete before launch:

- 12-month treasury/COL simulation with baseline and stress scenarios
- Monte Carlo volatility scenarios (low/high volume regimes)
- drawdown and cap trigger tests
- unlock schedule abuse tests
- replay determinism tests
- batch proof failure tests
- stale-oracle and delayed-oracle tests

Ship gate condition:

- no test path allows sudden large supply release
- no test path bypasses risk caps or no-drain constraints

## 12. Operational Playbooks (Required)

- market stress and high volatility
- oracle outage/degradation
- prover backlog/degradation
- key-epoch rotation failure
- exploit response and staged restart
- governance emergency actions

Each playbook must define:

- trigger conditions
- immediate safe-mode actions
- recovery sequence
- public incident communication template

## 13. Transparency Package (Anti-Dump Trust Layer)

Publish a live public dashboard at launch:

- total supply
- circulating float
- locked COL balance
- release schedule and next unlock amount
- COL NAV and risk utilization
- fee routing outputs (70/20/10)
- treasury runway

## 14. Launch Gates (Must All Pass)

- [ ] Genesis allocation table finalized and audited
- [ ] No-drain and release-cap invariants proven in tests
- [ ] Fee router verified on-chain
- [ ] RBS risk limits validated under stress
- [ ] Slashing and dispute flows tested end-to-end
- [ ] Replay and selective disclosure pipeline validated
- [ ] Dashboard live and reconciles with chain state
- [ ] Governance timelock + emergency pause tested
- [ ] Companion EVM primitive checklist passed for bridge-enabled launch

## 15. Immediate Next Deliverables

1. Finalize numeric genesis allocation table (exact values).
2. Encode `COLVaultModule` and `ReleaseCOLTranche` constraints in VM.
3. Encode `RBSModule` risk gates and fee-budget consumption.
4. Implement per-epoch accounting outputs and dashboard feed.
5. Run simulation suite and freeze launch parameters.
6. Execute `VEIL_COMPANION_EVM_PRIMITIVES_CHECKLIST.md` and commit address registry + tx hash evidence.
