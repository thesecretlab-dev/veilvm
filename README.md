# VeilVM

**A custom Avalanche VM for privacy-preserving prediction markets, built on [HyperSDK](https://github.com/ava-labs/hypersdk).**

Not a fork of Subnet-EVM â€” a purpose-built execution environment with native ZK proof verification, commit-reveal batch auctions, and a shielded ledger.

ChainId `22207` Â· Built in Go Â· Avalanche L1

## Why a Custom VM?

EVM-based chains leak information. Order flow is visible in the mempool, trade sizes are public, and market manipulation is trivial. VeilVM solves this at the execution layer:

- **Encrypted order commitments** â€” Orders are committed as hashes, revealed only during batch clearing
- **Proof-gated settlement** â€” Batches clear only when a valid ZK proof is submitted and verified at consensus
- **Shielded ledger** â€” Balance and position privacy via ZK-SNARK proofs (Groth16/PLONK, BN254)
- **Native fee routing** â€” Protocol fees split across market-specific buyback (MSRB), chain-owned liquidity (COL), and operations

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

A parallel EVM chain hosts DeFi primitives that bridge to VeilVM:

- **WVEIL** â€” Wrapped VEIL (ERC-20) via Teleporter bridge
- **wsVEIL** â€” Rebase-wrapping staked VEIL (Olympus-style)
- **VAI** â€” VEIL-native stablecoin
- **Bond Vaults** â€” Discount VEIL acquisition via LP/DAI bonds
- **Intent Gateways** â€” Cross-chain order and liquidity routing

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
