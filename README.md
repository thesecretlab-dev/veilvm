# VeilVM

**A custom Avalanche VM for privacy-preserving prediction markets, built on [HyperSDK](https://github.com/ava-labs/hypersdk).**

Not a fork of Subnet-EVM â€” a purpose-built execution environment with native ZK proof verification, commit-reveal batch auctions, and a shielded ledger.

ChainId `22207` Â· Built in Go Â· Avalanche L1

## Why a Custom VM?

EVM-based chains leak information. Order flow is visible in the mempool, trade sizes are public, and market manipulation is trivial. VeilVM solves this at the execution layer:

- **Encrypted order commitments** — `VEILENC1` envelopes; window key revealed on `RevealBatch`
- **Proof-gated settlement** — `ClearBatch` requires groth16 `shielded-ledger-v1` (digest-bound public slots, not in-circuit matching)
- **Threshold tx gossip** — VTG2 Shamir + X25519, fail-closed `t>=2`. Local 2-of-3. Shared AES is not a private mempool.
- **Native fee routing** — 70/20/10 MSRB / COL / ops, plus native VAI and AMM (actions 7–14)

## Actions

| ID | Action | Description |
|----|--------|-------------|
| 0 | `Transfer` | Transfer VEIL tokens |
| 1 | `CreateMarket` | Create a prediction market |
| 2 | `CommitOrder` | Submit encrypted order commitment |
| 3 | `RevealBatch` | Submit decryption share for batch reveal |
| 4 | `ClearBatch` | Clear batch auction (proof-gated) |
| 5 | `ResolveMarket` | Resolve with oracle attestation |
| 6 | `Dispute` | Dispute a market resolution |
| 7 | `RouteFees` | Split fees across MSRB/COL/Ops |
| 8 | `ReleaseCOLTranche` | Release treasury COL by epoch cap |
| 9â€“10 | `MintVAI` / `BurnVAI` | VAI stablecoin operations |
| 11â€“14 | `CreatePool` / `AddLiquidity` / `RemoveLiquidity` / `SwapExactIn` | Native UniV2-style DEX |
| 15â€“16 | `UpdateReserveState` / `SetRiskParams` | Governance updates |
| 17 | `SubmitBatchProof` | Submit ZK proof + Vellum proof blob |
| 18 | `SetProofConfig` | Governance proof requirements |

**v1 is these 19 actions.** IDs 19–41 in older ANIMA/handshake docs are spec-only and are not in this binary. Public copy must say 19, not 22/41/42. See `veil-docs/architecture/VEIL_STACK.md`.

## ZK Proof Pipeline

```
Prover                          VeilVM Consensus
  â”‚                                    â”‚
  â”œâ”€ Compute clearPrice, volume,       â”‚
  â”‚  fillsHash from revealed orders    â”‚
  â”‚                                    â”‚
  â”œâ”€ Hash: sha256("VEIL_CLEAR_V1" â•‘    â”‚
  â”‚  marketID â•‘ windowID â•‘ clearPrice  â”‚
  â”‚  â•‘ totalVolume â•‘ fillsHash)        â”‚
  â”‚                                    â”‚
  â”œâ”€ Generate Groth16 proof â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â–ºâ”‚
  â”‚  (SubmitBatchProof)                â”‚â”€â”€ Verify proof (gnark BN254)
  â”‚                                    â”‚â”€â”€ Store in Vellum proof storage
  â”‚                                    â”‚â”€â”€ Mint Glyph inscription
  â”‚                                    â”‚â”€â”€ Update Bloodsworn profile
  â”‚                                    â”‚
  â”œâ”€ ClearBatch â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â–ºâ”‚
  â”‚                                    â”‚â”€â”€ Verify proof record exists
  â”‚                                    â”‚â”€â”€ Match public_inputs_hash
  â”‚                                    â”‚â”€â”€ Verify Vellum blob integrity
  â”‚                                    â”‚â”€â”€ Execute settlement (fail-closed)
```

**Proof Envelopes**: `VZK1` (proof + witness) and `VZK2` (+ circuit ID). Circuit identity enforced at consensus when `VEIL_ZK_REQUIRED_CIRCUIT_ID` is set.

## Companion EVM

Companion EVM is **rails**, not a second protocol. v1 rails: WVEIL, intent gateways, bridge minter, test faucet. Native VAI/AMM/COL live only as VeilVM actions. Local Teleporter is a mock. Olympus / Maker / meme contracts in `veil-contracts` are parked.

See [veil-contracts](https://github.com/thesecretlab-dev/veil-contracts) for the full Solidity suite.

## Build & Run

```bash
# Build
go build ./...

# Run the VM
go run ./cmd/veilvm

# Generate ZK fixture keys
go run ./cmd/veilvm-zktool -out ./zk-fixture
go run ./cmd/veilvm-zktool -circuit shielded-ledger-v1 -out ./zk-fixture-shielded

# Run ZK benchmarks with real Groth16 proofs
PROOF_MODE=groth16 GROTH16_PK_PATH=./zk-fixture/groth16_clearhash_pk.bin \
  go run ./cmd/veilvm-zkbench

# Docker (local profile with strict verifier)
docker compose -f docker-compose.local.yml up -d --build

# Smoke test
node scripts/smoke-local.mjs --chain-id <CHAIN_ID>
```

## RPC Extensions

| Method | Description |
|--------|-------------|
| `clearinputshash` | Compute canonical public-input hash |
| `batchproof` | Get batch proof metadata |
| `vellumproof` | Get stored proof blob |
| `bloodsworn` | Read validator trust profile |
| `glyph` | Read proof-derived inscription metadata |

## Ecosystem

| Component | Repo |
|-----------|------|
| Smart Contracts | [veil-contracts](https://github.com/thesecretlab-dev/veil-contracts) |
| Frontend | [veil-frontend](https://github.com/thesecretlab-dev/veil-frontend) |
| Identity (ZK) | [zeroid](https://github.com/thesecretlab-dev/zeroid) |
| Agent Runtime | [anima-runtime](https://github.com/thesecretlab-dev/anima-runtime) |
| Documentation | [veil-docs](https://github.com/thesecretlab-dev/veil-docs) |

## Links

- **Protocol**: [veil.markets](https://veil.markets)
- **Lab**: [thesecretlab.app](https://thesecretlab.app)
- **Research**: [LatentSync Paper](https://thesecretlab.app/research/latentsync/)

---

*Markets that can't be front-run. Proofs that can't be faked.*
