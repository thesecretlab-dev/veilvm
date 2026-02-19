# VEIL ZK-Consensus 4-6s Trial Profile

Status: Trial Override (Phase 1, Revalidated)  
Date: 2026-02-19  
Owners: Protocol + VM + Infra

## 1. Objective

Run VEIL with proof-required consensus and target market block times of `4-6s` without relaxing privacy or validity guarantees.

This profile targets 4-6s block acceptance while keeping proof-required validity in consensus.

## 2. Security Invariants (Must Hold)

- No proof, no market state transition (fail-closed).
- Every validator verifies the ZK proof before accepting a market-transition block.
- Proof binds pre-state root, post-state root, commitments, nullifiers, and fee accounting.
- Invalid/malformed proofs are rejected deterministically.
- Liveness fallback is allowed only for non-market/empty blocks.

## 3. Consensus Model (Option 2, Practical Form)

- Proof is part of consensus validity (block is invalid without it).
- Proposer includes the proof in the block proposal.
- All validators perform in-consensus proof verification.
- Prover responsibility rotates with proposer role; backup provers are pre-assigned to avoid misses.

## 4. Trial Parameters (Genesis/Config Targets)

| Parameter | Trial Value | Notes |
|---|---:|---|
| `target_block_time` | `5s` | Acceptable envelope is 4-6s |
| `batch_window` | `5s` | One auction window per market block |
| `max_batch_orders` | `64` | Raise only after benchmarks pass |
| `max_ciphertext_bytes` | `256 KiB` | Hard cap for prover predictability |
| `proof_deadline` | `10.0s` from slot start | `4.2s` produced missed-proof windows under local trial load |
| `validator_verify_budget` | `<= 800ms` | Per block, p99 target |
| `prover_committee` | `5 active + 2 backup` | Deterministic assignment |
| `validator_set` | `20` (`t=13`) | Matches current VEIL design |
| `proof_system` | `Groth16` (fixed circuit) | No dynamic circuit switching in trial |
| `proof_size_limit` | `<= 256 KiB` | Reject larger payloads |

## 5. Slot Timing Budget (5s Target)

| Stage | Budget |
|---|---:|
| Encrypted tx intake + freeze | `0.8s` |
| Batch construction + witness build | `0.9s` |
| Proof generation | `2.5s` |
| Proof verification + vote path | `0.8s` |
| Slack | `0.0-0.5s` |

## 6. Launch Gates For This Profile

- Prover latency: `p95 <= 3.8s`, `p99 <= 4.6s` on reference hardware.
- Verifier latency: `p99 <= 120ms` per validator.
- End-to-end block time: `p95 <= 6s`.
- Missed-proof rate: `< 0.5%` over 10k blocks.
- Invalid-proof acceptance: `0` (hard requirement).

## 7. Implementation Status (Current)

Completed:

1. Structured timing instrumentation added for:
   - batch freeze time
   - witness build time
   - proof generation time
   - proof verification time
   - final block acceptance latency
2. Load-test harness implemented for batch sizes 32, 64, 96, 128.
3. Report artifact generation implemented:
   - per-size CSV outputs (metrics_batch_*.csv)
   - rollup outputs (summary.md, summary.json)
4. Quick smoke matrix run completed on live local VEIL chain.
5. Consensus verifier hooks wired at both `SubmitBatchProof` and `ClearBatch` (strict fail-close mode available).
6. `cmd/veilvm-zkbench` supports `PROOF_MODE=groth16` with PK-backed VZK1 proof envelopes for verifier-path runs.
7. VM startup now logs effective verifier config and local strict fallback auto-enables verifier when fixture VK is present.
8. `veilvm-zkbench` now verifies tx execution success via indexer (`Result.Success`), removing submit-only false positives.

Pending:

1. Expand strict shielded-ledger acceptance evidence from one-window launch-gate coverage to sustained multi-window and long-run coverage.
2. Archive production assurance artifacts for the circuit gate (`spec hash`, `VK hash`, vectors, review sign-off).
3. Long-run report target (10k-block equivalent coverage per size) with p50/p95/p99 stability evidence.
4. Failure drills:
   - malformed proof injection
   - backup prover takeover
5. Final recommendation on safe max_batch_orders under sustained load.

## 8. Latest Evidence (2026-02-19)

- Runtime verifier config (local node startup): `enabled=true strict=true groth16_vk_set=true required_circuit_id=shielded-ledger-v1`
- Launch-gate evidence bundle: `evidence-bundles/20260219-184441-launch-gate-evidence/bundle.md`

Launch-gate checks (`runner=docker`, `chain_id=gbbsaboFf5jzM721wnMf82ZvZqF64a5i3xhPg4m6vhXeKmgbc`):

- `shielded-smoke`: PASS (`accepted=1`, `rejected=0`, `missed=0`)
- `synthetic-negative`: PASS (expected fail-close rejection observed, non-zero exit)
- `timeout-drill-b8`: intermediate FAIL (no timeout at batch `8`)
- `timeout-drill-b32`: PASS (expected `proof deadline missed` observed, non-zero exit)

Primary artifact paths:

- `evidence-bundles/latest-launch-gate-evidence.txt`
- `evidence-bundles/20260219-184441-launch-gate-evidence/bundle.json`
- `evidence-bundles/saved/20260219-184441-launch-gate-evidence.zip`

Historical note:

- Earlier local runs that appeared to accept Groth16 before tx-result validation hardening are not sufficient as launch-gate evidence on their own.

## 9. Decision Rule After Trial

- If all launch gates pass at `64` orders: keep `5s` target and continue.
- If prover p99 misses but verifier is healthy: reduce `max_batch_orders` and rerun.
- If misses persist at `32` orders: move to staged proving (proof for next block) while keeping fail-closed market transitions.
