# VeilVM

Privacy-native prediction markets on Avalanche, built with HyperSDK.

## Overview

VeilVM implements the VEIL protocol - a custom Avalanche VM for privacy-preserving prediction markets using commit-reveal batch auctions.

## Current Status (2026-02-19)

- Local VEIL chain is running with coreapi and veilapi routes live.
- Proof-gated batch pipeline is active:
  - SubmitBatchProof is required for proof records.
  - ClearBatch enforces commitment hash and canonical public-input hash checks.
- Vellum proof blob storage is wired and queryable.
- ZK timing metrics are instrumented in actions and exposed over JSON-RPC.
- Consensus verifier wiring is active in the local profile:
  - Groth16/PLONK verifier path (gnark, BN254) is installed in-VM
  - runtime config is logged at startup for explicit verifier state
  - local fallback auto-enables strict Groth16 verification when fixture VK exists at `/root/.avalanchego/zk/groth16_clearhash_vk.bin`
  - SubmitBatchProof verifies before state writes when verifier is enabled
- Bench harness cmd/veilvm-zkbench is running and producing:
  - metrics_batch_32.csv, metrics_batch_64.csv, metrics_batch_96.csv, metrics_batch_128.csv
  - summary.md, summary.json
- Bench tx validation now checks indexed execution result (`Result.Success`) instead of only submit + height advance.
- Latest launch-gate evidence bundle is PASS on shielded-ledger runtime gate:
  - bundle: `evidence-bundles/20260219-184441-launch-gate-evidence/bundle.md`
  - pointer: `evidence-bundles/latest-launch-gate-evidence.txt`
  - verdict includes shielded smoke acceptance, synthetic fail-close rejection, and timeout deadline rejection evidence.

Not complete yet:

- Sustained multi-window/long-run 4-6s profile evidence is still pending.
- Malformed-proof and backup-prover-takeover drills are still pending.
- Production gate `G3` still needs the full circuit package (`spec hash + VK hash + proof vectors + security sign-off`).

## Actions

| Action | ID | Description |
|--------|-----|-------------|
| Transfer | 0 | Transfer VEIL tokens |
| CreateMarket | 1 | Create a new prediction market |
| CommitOrder | 2 | Submit encrypted order commitment |
| RevealBatch | 3 | Submit decryption share for batch reveal |
| ClearBatch | 4 | Clear a batch auction (proof-gated) |
| ResolveMarket | 5 | Resolve market with oracle attestation |
| Dispute | 6 | Dispute a market resolution |
| RouteFees | 7 | Split protocol fees across MSRB/COL/Ops |
| ReleaseCOLTranche | 8 | Release treasury COL by epoch cap |
| MintVAI | 9 | Mint VAI against configured reserve/risk gates |
| BurnVAI | 10 | Burn VAI and reduce protocol debt |
| CreatePool | 11 | Create UniV2-style pool |
| AddLiquidity | 12 | Add liquidity to pool |
| RemoveLiquidity | 13 | Remove liquidity from pool |
| SwapExactIn | 14 | Exact-in swap in pool |
| UpdateReserveState | 15 | Governance update of reserve telemetry |
| SetRiskParams | 16 | Governance update of collateral/risk params |
| SubmitBatchProof | 17 | Submit batch proof + Vellum proof blob |
| SetProofConfig | 18 | Governance update of proof requirements |

## ZK Flow (v1)

1. Prover computes `public_inputs_hash = sha256("VEIL_CLEAR_V1" || marketID || windowID || clearPrice || totalVolume || fillsHashLen || fillsHash)`.
2. Prover submits `SubmitBatchProof` with:
   - `public_inputs_hash` (32 bytes)
   - `fills_hash`
   - full proof blob (stored in Vellum proof storage)
3. `SubmitBatchProof` requires:
   - governance-configured prover authority + proof deadlines
   - if verifier is enabled: proof validity check before record is written
4. `ClearBatch` is fail-closed and requires:
   - proof record exists and matches required proof type/deadline
   - proof blob exists in Vellum store and hashes to stored commitment
   - stored `public_inputs_hash` matches canonical hash computed from clear inputs
   - if verifier is enabled: cryptographic proof verification must pass

RPC helpers:
- `clearinputshash`: compute canonical public-input hash
- `batchproof`: get batch proof metadata
- `vellumproof`: get stored proof blob
- `bloodsworn`: read validator trust profile (non-consensus metadata)
- `glyph`: read proof-derived inscription metadata (non-consensus metadata)

## Verifier Activation

Runtime env toggles:

- `VEIL_ZK_VERIFIER_ENABLED=true`
- `VEIL_ZK_VERIFIER_STRICT=true` (optional fail-close when verifier/key unavailable)
- `VEIL_ZK_GROTH16_VK_PATH=/path/to/groth16.vk` (if `requiredProofType=1`)
- `VEIL_ZK_PLONK_VK_PATH=/path/to/plonk.vk` (if `requiredProofType=2`)
- `VEIL_ZK_REQUIRED_CIRCUIT_ID=clearhash-v1|shielded-ledger-v1` (optional hard gate)

`VEIL_ZK_VERIFIER_ENABLED=true` requires at least one matching VK path to load at startup.

Proof blob format:

- Legacy mode: raw proof bytes (accepted only when verifier mode is disabled)
- Envelope mode (`VZK1`): `magic(4) | proof_type(1) | proof_len(4) | witness_len(4) | proof | public_witness`
- Envelope mode (`VZK2`): `magic(4) | proof_type(1) | circuit_len(1) | proof_len(4) | witness_len(4) | circuit_id | proof | public_witness`

When verifier mode is enabled, an envelope with public witness is required.
`VEIL_ZK_REQUIRED_CIRCUIT_ID` enforces circuit identity at consensus verification time.

## Glyphs and Bloodsworn

- Every accepted `SubmitBatchProof` mints one deterministic `Glyph` from proof metadata and tx entropy.
- Every accepted proof also updates the prover's `Bloodsworn` profile (`total_accepted_proofs`, `active_streak`).
- These systems are explicitly non-consensus-critical:
  - no block validity rules depend on `Glyph` rarity/class
  - no token mint or economic reward is attached by default
  - proof-gating security remains unchanged (commitment + input-hash checks in `ClearBatch`)

## Build

```bash
go build ./...
```

Generate Groth16 fixture keys/proof envelope for verifier rollout:

```bash
go run ./cmd/veilvm-zktool -out ./zk-fixture
```

Generate shielded-ledger fixture artifacts:

```bash
go run ./cmd/veilvm-zktool -circuit shielded-ledger-v1 -out ./zk-fixture-shielded
```

`veilvm-zktool` defaults to `clearhash-v1` and can emit either circuit fixture set:
- clear-hash: `groth16_clearhash_pk.bin` / `groth16_clearhash_vk.bin`
- shielded-ledger: `groth16_shielded_ledger_pk.bin` / `groth16_shielded_ledger_vk.bin`

Run zkbench with real Groth16 proofs (fixture mode):

```bash
PROOF_MODE=groth16 GROTH16_PK_PATH=./zk-fixture/groth16_clearhash_pk.bin CHAIN_ID=<CHAIN_ID> go run ./cmd/veilvm-zkbench
```

Shielded-ledger fixture mode (current local launch-gate path):

```bash
PROOF_MODE=groth16 PROOF_CIRCUIT_ID=shielded-ledger-v1 GROTH16_PK_PATH=./zk-fixture-new/groth16_shielded_ledger_pk.bin CHAIN_ID=<CHAIN_ID> go run ./cmd/veilvm-zkbench
```

Fee safety controls for long bench runs:

- `REFUEL_PRIVATE_KEY`: optional secondary key used to auto-top-up the bench key when fee balance drops.
- `GAS_SAFETY_BPS` (default `13000`): multiplies projected run fee budget (130% default).
- `GAS_RESERVE` (default `250000`): minimum fee buffer to keep before each tx.
- `REFUEL_AMOUNT` (default `5000000`): minimum transfer size used during auto-refuel.
- `STRICT_FEE_PREFLIGHT` (default `false`): when `true`, fail fast on startup/per-tx fee preflight checks even without refuel configured.

`veilvm-zkbench` now performs:

1. Startup fee preflight (strict fail only with `STRICT_FEE_PREFLIGHT=true` or when refuel is configured).
2. Runtime insufficient-fee detection on each action submit.
3. One-shot auto-refuel + retry on insufficient-fee submission errors.

## Run

```bash
go run ./cmd/veilvm
```

## Local Ops

Security defaults in `docker-compose.local.yml`:

- admin API disabled (`--api-admin-enabled=false`)
- allowed hosts restricted to local callers (`localhost`, `127.0.0.1`, `host.docker.internal`)
- RPC and staking ports exposed only on localhost (`127.0.0.1`)
- verifier env is strict-enabled with required circuit id `clearhash-v1` for the default local profile

Override the local verifier gate to shielded-ledger fixture keys:

```powershell
# generate shielded-ledger fixture artifacts if not already present
go run ./cmd/veilvm-zktool -circuit shielded-ledger-v1 -out ./zk-fixture-shielded
Copy-Item ./zk-fixture-shielded/groth16_shielded_ledger_vk.bin ./zk-fixture-new/groth16_shielded_ledger_vk.bin -Force

$env:VEIL_ZK_REQUIRED_CIRCUIT_ID="shielded-ledger-v1"
$env:VEIL_ZK_GROTH16_VK_PATH="/root/.avalanchego/zk/groth16_shielded_ledger_vk.bin"
docker compose -f docker-compose.local.yml up -d --build
```

This override is a local runtime switch only; production gate `G3` still requires full shielded-ledger circuit scope evidence.

Recurring local ops issues (and fast recovery):

1. Docker daemon hangs / CLI timeouts.
   - Symptom: `docker version` or `docker ps` stalls/timeouts.
   - Recovery: restart Docker Desktop, then re-run the compose command.
2. Wrong stack responds on `9650` while VeilVM `9660` is down.
   - Symptom: evidence runner falls back to `http://127.0.0.1:9650` and health stays `503`.
   - Recovery: bring up VeilVM node explicitly:
     - `docker compose -f docker-compose.local.yml up -d --build node`
3. Verifier circuit gate resets to `clearhash-v1` after container restart.
   - Symptom: `proof circuit mismatch` during shielded-ledger evidence runs.
   - Recovery:
     - `$env:VEIL_ZK_REQUIRED_CIRCUIT_ID="shielded-ledger-v1"`
     - `$env:VEIL_ZK_GROTH16_VK_PATH="/root/.avalanchego/zk/groth16_shielded_ledger_vk.bin"`
     - `docker compose -f docker-compose.local.yml up -d --build node`
4. Readiness stays `503` right after node restart.
   - Symptom: health payload reports bootstrapping not finished.
   - Recovery: wait until `http://127.0.0.1:9660/ext/health/readiness` returns `200`, then start evidence runs.

Launch-gate evidence runner:

```powershell
cd scripts
npm.cmd run evidence:preflight
npm.cmd run evidence:launch-gates
```

Latest saved evidence artifacts:

- `evidence-bundles/latest-launch-gate-evidence.txt`
- `evidence-bundles/saved/latest-launch-gate-evidence.txt`
- `evidence-bundles/saved/20260219-184441-launch-gate-evidence.zip`

Smoke test:

```bash
# validate an existing chain
node scripts/smoke-local.mjs --chain-id <CHAIN_ID>

# validate full setup flow (CreateSubnet + CreateBlockchain)
# for fresh-volume testing, wipe docker volumes first
node scripts/smoke-local.mjs --run-setup
```

Create a fresh VEIL chain on the already-tracked subnet (no DB wipe):

```bash
cd scripts
node create-chain-on-subnet.mjs
```

## Protocol Docs

- `VEIL_MASTER_RUNBOOK.md` - execution order and launch critical path
- `VEIL_PRODUCTION_LAUNCH_CHECKLIST.md` - hard production go/no-go gates with owners and evidence requirements
- `VEIL_V1_NATIVE_PRIVACY_SPEC.md` - whitepaper-aligned protocol requirements
- `VEIL_EXECUTION_PACKAGE.md` - tokenomics, COL, risk controls, launch gates
- `VEIL_COMPANION_EVM_PRIMITIVES_CHECKLIST.md` - bridge/EVM primitive rollout checklist and pass conditions
- `VEIL_WHITEPAPER_ALIGNMENT_MATRIX.md` - claim-by-claim whitepaper alignment checklist
- `VEIL_HANDOFF_2026-02-19.md` - project-wide handoff snapshot and immediate next actions
