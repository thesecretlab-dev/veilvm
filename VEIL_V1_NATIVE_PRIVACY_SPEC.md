# VEIL Whitepaper-Aligned Protocol Spec (Final)

Status: Final
Date: 2026-02-18
Audience: Protocol, VM, infra, and governance implementers
Source of truth: This document reconciles and finalizes the published whitepaper requirements for implementation.

## 1. Scope and Objective

VEIL is a privacy-native prediction market on a dedicated Avalanche L1 Subnet.  
This spec defines the normative implementation baseline for:

- threshold-encrypted mempool
- shielded ledger with commitment/nullifier model and ZK-SNARK verification
- uniform batch auctions
- VRF-selected oracle committees with BLS attestations and dispute flow
- deterministic replay and selective disclosure
- objective slashing and bonded operator economics

## 2. Architecture Pillars

### 2.1 Encrypted Mempool

- Orders MUST be encrypted to a network committee public key.
- Validators MUST order ciphertext first and only decrypt after batch close/finalization.
- No plaintext mempool path is allowed in production mode.
- Security assumption: fewer than threshold colluding validators cannot decrypt pre-reveal.

### 2.2 Shielded Ledger

- Private state is represented by commitments and nullifiers.
- Nullifier uniqueness MUST prevent double-spend/replay.
- Public chain state MUST expose only aggregate market/settlement data needed for solvency and replay.

### 2.3 Uniform Batch Auction

- Markets clear in discrete windows (target 2-5 seconds).
- All executable orders in a window settle at one uniform clearing price.
- Matching and tie-break rules MUST be deterministic and replayable.

### 2.4 Resolution Layer

- Outcome attestors are VRF-selected committees.
- Committee output is BLS-signed and bound to market ID and resolution epoch.
- A dispute window MUST exist with bonded challenge flow and objective adjudication.

## 3. Consensus, Validators, and Bonding

### 3.1 Validator Set

- Dedicated subnet validator set, target size 20.
- Threshold cryptography parameters are governance-set; default reference threshold is t=13 of n=20.

### 3.2 Sovereign PoS and Privacy Bond

- Operators MUST post slashable VEIL-denominated bond.
- Bond secures liveness, protocol compliance, and privacy workflow participation.
- Bond and validator status transitions MUST be on-chain and replayable.

## 4. ZK and Proof Boundary

Each executable batch MUST include a validity proof that enforces at minimum:

- accepted commitment inclusion constraints
- nullifier uniqueness and non-reuse
- balance conservation including fee accounting
- deterministic clearing constraints
- valid state-root transition from pre-state to post-state

Proof system baseline: Groth16 (whitepaper baseline), with upgrade path controlled by governance.

### 4.1 Proof-Gated Consensus Profile (v1 Default)

This profile is the default launch baseline:

- `batch_window = 5s`
- `proof_deadline = 10s` from batch close
- deterministic prover committee: `5 of 20` with `2` backup provers
- consensus validity requires a valid batch proof before market-state transition
- all validators MUST verify proofs in-VM before accepting transitions

This keeps ZK validity inside consensus-critical rules while avoiding a requirement that every validator generate full proofs for every batch.

## 5. Fail Mode and Liveness Rules

- Market execution is fail-closed.
- If decrypt quorum fails, proof misses deadline, or proof verification fails, batch execution MUST be rejected.
- Chain MAY continue non-market/control blocks.
- New intake MAY be paused by policy during repeated key-epoch failures (see Section 8).

## 6. Slashing and Penalties (Objective Evidence Only)

Only machine-verifiable faults are slashable in v1.

- Missed decrypt share deadline:
  - slash 0.5% bond per miss
  - jail after 3 misses within rolling 24h
- Invalid decrypt share:
  - slash 5%
  - jail 7 days
- DKG/key-rotation non-participation:
  - slash 2% per missed required phase
- Missed batch proof deadline (assigned prover committee):
  - slash 0.5% per miss
  - jail after repeated misses per governance-configured threshold
- Invalid batch proof submission:
  - slash 5%
  - jail 7 days
- Equivocation/double-sign:
  - slash 15%
  - tombstone
- Premature decryption attempt:
  - slashable only when cryptographically provable on-chain

Slash destination split:

- 50% burn
- 50% insurance/treasury pool

## 7. Oracle Resolution and Dispute

- VRF committee selection MUST be deterministic from finalized chain entropy.
- BLS attestations MUST be threshold-valid and bound to market resolution payload.
- Disputes MUST require challenger bond and fixed dispute window.
- Successful dispute:
  - challenger bond returned
  - faulty committee operators slashed
- Failed dispute:
  - challenger bond forfeited per governance policy

## 8. Key Epoch and Rotation

- Epoch length: governance controlled (default 6h operational baseline).
- Rotation window opens 1h before rollover.
- On rotation failure at rollover:
  - retain last good key for one grace epoch
  - slash non-participants
- If rotation fails in two consecutive epochs:
  - pause new order intake
  - run emergency rotation workflow before resuming intake

## 9. Deterministic Replay and Selective Disclosure

### 9.1 Deterministic Replay

- Every state transition MUST be replayable from ordered commitments, decryption artifacts, proof inputs, and settlement outputs.
- Replay determinism is a protocol requirement for audits and post-mortems.

### 9.2 Selective Disclosure

- Protocol MUST support proving specific regulated facts without global transparency spill.
- Disclosure scope MUST be minimal and request-bound.
- Privacy guarantees for non-requested user activity MUST remain intact.

## 10. Governance and Parameter Control

VEIL token governance controls:

- batch interval and market parameters
- committee sizes and thresholds
- slashing rates and jail durations
- proof-system upgrades
- fee routing and treasury policy

All changes MUST be on-chain, versioned, and replayable.

## 11. Economic Coupling (Part II Alignment)

Fee router baseline:

- 70% to MSRB (market depth bank)
- 20% to POL buyback-and-make
- 10% to operations

Operator economics:

- rewards from protocol fee participation
- slashable VEIL bond for protocol faults
- no dependency on perpetual emissions as a primary security model

## 12. SLOs (Whitepaper Baseline)

Implementation is expected to satisfy these service-level objectives:

- batch clearing latency: 99.9% within 5 seconds
- order privacy: no pre-reveal plaintext exposure by protocol design
- market depth quality: top-10 average spread < 0.5%
- subnet availability: 99.95% uptime
- zk proving performance: average proof generation < 1 second

Scalability objective range:

- target 10k-100k orders per batch under production proving/hardware profiles
- this is a scale objective, not a guarantee for every early deployment phase

## 13. Failure Playbooks (Required)

Playbooks MUST exist and be tested for:

- validator collusion attempts
- oracle committee failure or corruption
- ZK prover degradation/outage
- key-epoch rotation failure
- congestion and delayed settlement
- exploit/emergency halt and staged recovery

Each playbook MUST include: detection signals, safe-mode action, recovery sequence, and governance escalation path.

## 14. Explicit Non-Goals

- global anonymity against legitimate regulated requests
- perfect metadata privacy in all network conditions
- unbounded throughput beyond protocol and proving limits

## 15. v1 Compliance Checklist

A deployment is compliant only if all are true:

- encrypted mempool path is enforced in production
- batch execution is proof-gated and fail-closed
- commitment/nullifier privacy model is active
- VRF+BLS oracle and dispute window are implemented
- deterministic replay artifacts are preserved
- selective disclosure flow exists without global state reveal
- objective slashing is on-chain and enforceable
- fee router follows 70/20/10 baseline (or approved governance override)
- SLO telemetry and incident playbooks are in place

## 16. Implementation Note

HyperSDK action/state design (actions, storage keys, verifier hooks, and epoch machinery) MUST implement this spec without weakening these invariants.
