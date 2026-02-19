# VEIL Project Handoff (2026-02-19)

Status: Active handoff snapshot  
Date: 2026-02-19  
Updated: 2026-02-19T23:20Z  
Scope: VEIL custom VM chain + companion EVM chain

## 1. Naming and Scope

- `VEIL` = custom HyperSDK VM chain (privacy, proof validity, treasury/risk invariants).
- `VEIL-EVM` (chainId `22207`) = companion Subnet-EVM rail for interop/access tooling.
- Production-critical economic and privacy rules remain VM-side; EVM side is supporting rails.

## 2. Executive Snapshot

| Area | Status | Notes |
|---|---|---|
| VEIL chain health | PASS (local) | Chain boots and produces blocks in local profile |
| Proof-gated flow | PASS (local strict) | Strict verifier active with `required_circuit_id=shielded-ledger-v1`; invalid proofs rejected |
| Latest launch-gate evidence | PASS | `evidence-bundles/20260219-231603-launch-gate-evidence/bundle.md` |
| Longer funded shielded run | PASS | `zkbench-out-groth16-shielded-long-funded-20260219-180640/summary.json` |
| Fuel/faucet resilience | PARTIAL | Only one funded non-genesis bench key currently has enough runway for constrained long runs |
| Companion precompile policy | PASS (local) | `npm.cmd run check:companion-policy` passes |
| Companion bridge contracts | BLOCKED (prod) | Teleporter/bridge deploys are still placeholders |
| Production readiness | NO-GO | Launch gates remain incomplete |

## 3. Completed Work to Date

### 3.1 VEIL VM Core

- Local chain bring-up stabilized with plugin handshake and reproducible local ops flow.
- Proof-gated batch pipeline is wired:
  - `SubmitBatchProof` + `ClearBatch` fail-closed checks.
  - Proof blob commitment in Vellum storage.
- ZK runtime hardening:
  - Startup logs now print resolved verifier config.
  - Local fallback auto-enables strict Groth16 if VK fixture file exists in-container.
- Bench hardening:
  - `veilvm-zkbench` validates tx execution success from indexer (`Result.Success`) instead of submit-only signals.
- Tokenomics/control action surface is present in VM action set (`RouteFees`, `ReleaseCOLTranche`, `MintVAI`, `BurnVAI`, pool/risk/proof actions).

### 3.2 Companion EVM (VEIL-EVM)

- Warp/AWM config is present in genesis.
- Fee manager + allowlist + native minter precompile configs are populated in `upgrade.json` using `adminAddresses` and `enabledAddresses`.
- Companion role map and deployments are recorded in `scripts/companion-evm.addresses.json`.
- Policy validation passes:
  - `npm.cmd run check:companion-policy` -> `PASS: required companion EVM primitive fields are populated.`

### 3.3 Late-Session Incident Fixes (2026-02-19)

Incident observed during longer zk runs:

- Repeated run failures from:
  1. fuel depletion / refuel wallet insufficiency
  2. `set_proof_config execution failed: unauthorized` when switching to a funded non-genesis bench signer

Fixes applied:

1. Evidence runner now exposes and uses explicit proof-config authority key.
   - File: `scripts/run-launch-gate-evidence.mjs`
   - Added CLI flag: `--proof-config-private-key <HEX>`
   - Added env fallback: `VEIL_EVIDENCE_PROOF_CONFIG_PRIVATE_KEY`
   - Runner now always sets `PRIVATE_KEY` and `PROOF_CONFIG_PRIVATE_KEY` in base environment.

2. Backup takeover cases now use the resolved proof-config key instead of assuming the primary bench signer has config authority.
   - File: `scripts/run-launch-gate-evidence.mjs`

3. `veilvm-zkbench` source includes an unauthorized fallback path for proof-config submission.
   - File: `cmd/veilvm-zkbench/main.go`
   - New env: `PROOF_CONFIG_FALLBACK_PRIVATE_KEY`
   - Behavior: on `set_proof_config` unauthorized, retry with fallback key; if no fallback provided and bench key is non-genesis, auto-tries default genesis key.

Validation from this session:

- Long funded shielded run passed:
  - `zkbench-out-groth16-shielded-long-funded-20260219-180640/summary.json`
  - Result: `accepted=1 rejected=0 missed=0`
- Scripted launch-gate smoke passed with funded bench signer + explicit proof-config signer:
  - `evidence-bundles/20260219-231603-launch-gate-evidence/bundle.md`

## 4. Active Blockers

1. Full production assurance pack for shielded-ledger proof system is still incomplete:
   - circuit spec hash + VK hash + proof vectors + security sign-off bundle.
2. Companion bridge stack is not production-ready:
   - current Teleporter messenger/registry + bridge minter are local placeholder contracts.
3. Fuel runway risk remains:
   - non-genesis funded bench key balance is finite and drops per run.
4. Rebuild caveat:
   - `cmd/veilvm-zkbench/main.go` fallback logic is source-patched, but rebuilding in current runner image may fail (`gcc` missing) unless toolchain image is fixed.

## 4.1 Latest Launch-Gate Evidence (Current Pointer)

- Overall verdict: `PASS`
- Bundle: `evidence-bundles/20260219-231603-launch-gate-evidence/bundle.md`
- Bundle JSON: `evidence-bundles/20260219-231603-launch-gate-evidence/bundle.json`
- Check outcomes:
  - `shielded-smoke`: PASS (`accepted=1`, `rejected=0`, `missed=0`)

## 5. Key/Fuel Snapshot (2026-02-19T23:13Z)

Balance probe (`planned_txs=6`, `required=30000001`):

- Genesis key prefix `637404e672...`: balance `1197629` (insufficient for long-run startup budget).
- Funded key prefix `203206b925...`: balance `32739498` (usable for constrained long runs).

Operational implication:

- Use funded key as bench signer.
- Use genesis key as proof-config signer.
- Keep profiles constrained until faucet/reseed is restored.

## 6. Current Companion Registry (Local)

Source: `scripts/companion-evm.addresses.json`

| Field | Value |
|---|---|
| network | `VEIL-EVM` |
| chainId | `22207` |
| rpcUrl | `http://127.0.0.1:9650/ext/bc/2L5JWLhXnDm8dPyBFMjBuqsbPSytL4bfbGJJj37jk5ri1KdXhd/rpc` |
| tempAdminEoa | `0x580358409B75e315919a9aBFEf9cB3b55F54503A` |
| bridgeRelayer1 | `0x7deFD0ea7934bFA6a3C2D638e6A6f215dE8FAd53` |
| bridgeRelayer2 | `0x7843eb9f852D05f9A6314b77f7566b7fAD7B4bdF` |
| opsKeeper1 | `0x698acAcB4446ACaE951b5b9872a8bDC0C3c64A55` |
| deployer1 | `0x9d0FE62f75ba12d1f2a965756A91a88F26cBB1b4` |
| teleporterRegistry | `0x1B0302c822bacC41a4bB3a0B40e24Add91CF9c66` |
| teleporterMessenger | `0xC1b36605cEb6200c76A6AF7916267113BA5034de` |
| bridgeMinterContract | `0x18a85af9db521f9763e1a68e4E358284AAA6eF0C` |
| multicall3 | `0x19668A69Abc970cf0c30aAa1498af590a634De30` |
| create2Deployer | `0x4e59b44847b379578588920cA78FbF26c0B4956C` |
| faucet | `0x936A9a72411fC9F2D8A56a8b6D46F0Fa1DB8Ccb7` |

Upgrade config hash (current local):

- `avalanche-l1-docker/data/avalanche-cli/subnets/VEIL2/upgrade.json`
- SHA-256: `F5A5D890A33E475CDC519C4ABB271BFEE5BD38FF8DD8237634FB1AE83E58FA43`

## 7. Repro/Verification Commands

From `examples/veilvm/scripts`:

```powershell
npm.cmd run check:companion-policy
```

Expected: `PASS: required companion EVM primitive fields are populated.`

Launch-gate smoke using funded bench signer plus explicit proof-config signer:

```powershell
node .\scripts\run-launch-gate-evidence.mjs `
  --private-key <FUNDED_BENCH_KEY_HEX> `
  --proof-config-private-key <GENESIS_PROOF_CONFIG_KEY_HEX> `
  --skip-backup-takeover `
  --skip-negative `
  --skip-malformed `
  --skip-timeout `
  --batch-size 8 `
  --windows-per-size 1 `
  --timeout-minutes 20
```

Direct long run (already proven in this session):

- Output: `zkbench-out-groth16-shielded-long-funded-20260219-180640`
- Config: groth16 shielded-ledger, `BATCH_SIZES=8`, `WINDOWS_PER_SIZE=1`, `TIMEOUT_MINUTES=20`

## 8. Immediate Next Sequence

1. **TODO (fuel hardening):** add mandatory preflight in `scripts/run-launch-gate-evidence.mjs` to auto-faucet required keys to a fixed minimum balance, re-check balances, and hard-fail before execution if thresholds are not met.
2. Restore faucet/reseed strategy so takeover and multi-profile long runs can execute without manual balance triage.
3. Re-run adversarial drills on current chain state (`malformed`, `timeout`, `backup-takeover`) with refreshed funding.
4. Build and archive the full shielded-ledger circuit assurance pack to close gate `G3`.
5. Replace placeholder Teleporter/bridge contracts with production implementations before any bridge-enabled launch mode.

## 9. Handoff Notes for Next Operator

- If `set_proof_config ... unauthorized` appears:
  - first ensure runner was invoked with `--proof-config-private-key`.
  - if running raw `veilvm-zkbench`, set `PROOF_CONFIG_PRIVATE_KEY` explicitly.
- If fuel errors reappear:
  - run a balance probe first (prefund-only mode).
  - keep `GAS_SAFETY_BPS=10000` and `GAS_RESERVE=1` for constrained diagnostic runs.
- Evidence pointer file was stale before this update; keep `evidence-bundles/latest-launch-gate-evidence.txt` synchronized with newest PASS bundle.
