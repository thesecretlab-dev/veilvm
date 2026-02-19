# VEIL Production Launch Checklist

Status: Active  
Date: 2026-02-19  
Decision Mode: Hard Go/No-Go

## 1. Purpose

Define the exact production launch gates for:

- `VEIL` custom VM chain (privacy and consensus core)
- companion EVM chain (interop rails only)

Launch is blocked unless every gate below is `PASS` with archived evidence.

## 2. Owner Map

| Role | Responsibility |
|---|---|
| Protocol Lead | Final launch decision, parameter freeze, governance sign-off |
| VM Lead | VeilVM consensus rules, ZK verification path, invariant enforcement |
| Bridge/EVM Lead | AWM/Teleporter/bridge contracts, companion precompile policy |
| Treasury/Risk Lead | COL locks, VAI risk limits, fee router and RBS controls |
| SRE/Infra Lead | validator ops, uptime, monitoring, incident playbooks |
| Security Lead | audit closure, key management, adversarial testing sign-off |

## 3. Gate Board

| Gate ID | Gate | Primary Owner | PASS Criteria | FAIL Criteria | Required Evidence |
|---|---|---|---|---|---|
| G0 | Chain Health Baseline | SRE/Infra Lead | VEIL chain healthy, stable block production, deterministic restart success | unhealthy chain, re-bootstrap drift, unstable block acceptance | health snapshots, restart logs, chain IDs, config hashes |
| G1 | Proof-Gated Consensus | VM Lead | no-proof/no-clear enforced, invalid proof deterministically rejected, strict verifier path enabled | any market clear accepted without valid proof, nondeterministic verifier behavior | `zkbench` reports + rejection tests + verifier config dump |
| G2 | Native Privacy Invariants | VM Lead | encrypted batch flow + commitment/nullifier checks + fail-closed rules pass | bypass path, stale/degraded privacy mode, ambiguous acceptance rules | invariant tests + replay transcripts |
| G3 | Full ZK Circuit Scope | VM Lead + Security Lead | launch circuit includes full shielded-ledger constraints (not clear-hash benchmark-only path) and verifier circuit gate is set to `shielded-ledger-v1` | benchmark/demo circuit remains production path or circuit gate is unset/clearhash | circuit spec hash, VK hash, proof vectors, verifier config dump |
| G4 | Tokenomics + Treasury Locks | Treasury/Risk Lead | locked COL enforced at genesis, release caps enforced on-chain, no hidden mint/unlock route | treasury drain route, bypassable release caps, supply mismatch | genesis hash, supply proof, treasury state proof, unlock simulation |
| G5 | VAI Risk Controls | Treasury/Risk Lead | debt ceiling, per-epoch mint throttle, backing floor, collateral haircuts all enforced | unbounded minting, broken peg controls, stale-oracle bypass | risk parameter snapshot + scenario test outputs |
| G6 | Companion EVM Bridge Readiness | Bridge/EVM Lead | real (non-placeholder) Teleporter/bridge contracts deployed, VEIL<->EVM round-trip tests pass | placeholder contracts, partial bridge path, replay/custody gaps | `scripts/companion-evm.addresses.json`, tx hashes, round-trip test logs |
| G7 | Companion Policy Hardening | Bridge/EVM Lead | `TxAllowList`, `ContractDeployerAllowList`, `NativeMinter` policy checks pass with correct role separation | policy mismatch, overprivileged roles, config hash drift | `npm run check:companion-primitives` + `npm run check:companion-policy` outputs |
| G8 | Security Audit Closure | Security Lead | critical/high findings resolved or accepted with explicit risk sign-off | unresolved critical/high issues without approved exception | audit reports, remediation diffs, exception register |
| G9 | Reliability + Failure Drills | SRE/Infra Lead | missed-proof, malformed-proof, prover-timeout, backup-takeover drills passed and documented | any drill untested or fails without mitigation | drill reports + incident playbook run logs |
| G10 | Key Ceremony + Admin Rotation | Security Lead + Protocol Lead | temporary EOAs rotated to multisig/HSM policy, final signer sets frozen | launch with temporary admin keys or undocumented signer ownership | signer manifest, key-ceremony record, final permission diff |
| G11 | Launch Rehearsal | Protocol Lead | full dry-run from genesis to bridge and market flow completed with deterministic outputs | partial rehearsal, inconsistent outputs, missing rollback plan | rehearsal report, rollback runbook, signed launch packet |

## 4. Hard Launch Rule

All gates `G0` through `G11` are mandatory for production launch.

- If any gate is `FAIL` or `UNSET`: **NO-GO**
- If all gates are `PASS` with evidence: **GO**

## 5. Current Readiness Snapshot (2026-02-19)

| Gate ID | Status | Notes |
|---|---|---|
| G0 | PASS (local) | VEIL chain healthy and running in current local profile |
| G1 | PASS (local) | strict verifier is active and launch-gate bundle `20260219-184441-launch-gate-evidence` passed (shielded-smoke accepted; synthetic fail-close rejection observed) |
| G2 | PASS (local) | fail-closed proof hooks and rejection path wired |
| G3 | IN PROGRESS | local runtime gate is set to `shielded-ledger-v1` and launch-gate checks pass; full production circuit assurance package still pending |
| G4 | IN PROGRESS | treasury/COL lock design implemented, production freeze pending |
| G5 | IN PROGRESS | risk controls present; full production validation pack pending |
| G6 | FAIL | Teleporter/bridge contracts currently documented as placeholders (local-only stubs) |
| G7 | PASS (local) | `check:companion-policy` passes on current role map; production key ceremony and final role freeze still pending |
| G8 | FAIL | external audit closure package not complete |
| G9 | IN PROGRESS | timeout drill evidence captured in launch-gate bundle; malformed-proof and backup-takeover drill evidence still pending |
| G10 | FAIL | temporary admin key posture still present |
| G11 | FAIL | full production rehearsal packet not completed |

Current decision state: **NO-GO FOR PRODUCTION**

## 6. Evidence Index (Required Paths)

- `VEIL_MASTER_RUNBOOK.md`
- `VEIL_EXECUTION_PACKAGE.md`
- `VEIL_V1_NATIVE_PRIVACY_SPEC.md`
- `VEIL_COMPANION_EVM_PRIMITIVES_CHECKLIST.md`
- `VEIL_ZK_CONSENSUS_4_6S_TRIAL_PROFILE.md`
- `VEIL_HANDOFF_2026-02-19.md`
- `scripts/companion-evm.addresses.json`
- `evidence-bundles/latest-launch-gate-evidence.txt`
- `evidence-bundles/20260219-184441-launch-gate-evidence/bundle.json`
- `evidence-bundles/20260219-184441-launch-gate-evidence/bundle.md`
- `evidence-bundles/saved/20260219-184441-launch-gate-evidence.zip`
- `zkbench-out-groth16-live-1w/summary.json`
- `zkbench-out-groth16-live-32-64x3-deadline10s/summary.json`

## 7. Sign-Off Sheet

| Role | Name | Decision (`PASS`/`FAIL`) | Timestamp | Signature Ref |
|---|---|---|---|---|
| Protocol Lead |  |  |  |  |
| VM Lead |  |  |  |  |
| Bridge/EVM Lead |  |  |  |  |
| Treasury/Risk Lead |  |  |  |  |
| SRE/Infra Lead |  |  |  |  |
| Security Lead |  |  |  |  |
