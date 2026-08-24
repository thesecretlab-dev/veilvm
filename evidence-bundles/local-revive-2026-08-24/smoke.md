# Local revive 2026-08-24 (Windows, no Docker)

Host: this PC. AvalancheGo 1.13.0 + VeilVM plugin (rpcchainvm 39).
Not Fuji. Not mainnet.

Out-of-box runner: `scripts/run-local-stack.ps1`

## IDs

- NodeID: `NodeID-HMqe6QZg8h7Bb3minFk2YruUeGzdhy94H`
- SubnetID: `AkMZ5HpwZRuB1CY7M6HvUmHuVipiRUKD1dTyLmkoQFe8qqMrC`
- ChainID: `bdRGUMA7rzZFXjbn1ePTjqhAUfTjW94e69p7qZd4puZ3uEosL`
- HTTP: `http://127.0.0.1:9660`
- API: `http://127.0.0.1:9660/ext/bc/bdRGUMA7rzZFXjbn1ePTjqhAUfTjW94e69p7qZd4puZ3uEosL/veilapi`
- Companion: anvil `:8545` chainId **31337** (never 22207)
- Relayer: `veilvm-order-router` `:9098`

## Results

| ID | Status | Evidence |
|---|---|---|
| C01 | PASS | `/ext/health` and `/ext/health/readiness` healthy=true. Plugin `u9GgvekeunSwK4TPF4jj7xLsW1LKkd1Uv9VQZo2SGfrwkejsK`. Veil height > 6000 on this node. |
| C02 | PASS | chain-config JSON: `enabled=true strict=true groth16_vk_set=true required_circuit_id=shielded-ledger-v1` |
| C03 | PASS | Native AMM: mint VAI, add liquidity, swap. `veilvm-smoke` + `scripts/local-stack-e2e.mjs`. |
| C04 | PASS (anvil, not dual AvalancheGo EVM) | Companion = anvil 31337. Relayer + order-router in-tree. Dual-chain order+liquidity E2E PASS. |
| C05 | PASS | `evidence-bundles/abandoned-feb-2026/`. Live registry chainId 31337. `check-companion-primitives` fails closed on chainId 22207 and abandoned packets. |
| C06 | PASS (local profile) | `npm run check:prelaunch` → `overallPass=true`, `productionLaunchPass=false`. Artifact: `evidence-bundles/control-tower/prelaunch-readiness-20260824184020.json`. |

## Fixes applied this session

1. HyperSDK `internal/pebble.Compact` matches AvalancheGo: nil limit + empty DB no longer crash with `Compact start is not less than end`.
2. HyperSDK `Keys.WithoutPermissions` no longer injects empty keys (prealloc `len` then `append`).
3. Compose/Dockerfile no longer default `clearhash-v1` or track a dead subnet.
4. Plugin env is not inherited on Windows. ZK config is chain-config JSON under `.local/nodedata/configs/chains/<chainID>/config.json`.
5. `MintVAI` / `AddLiquidity` state keys use `All` so first-time allocate works.
6. JSON-RPC aliases `lpbalance` / `vaistate` / `vaibalance` (gorilla method casing).
7. `go.mod` replace points at sibling `../../hypersdk` (this tree is not `hypersdk/examples/veilvm`).

## How to restart

```
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\run-local-stack.ps1
```

Binaries live in `veilvm/.local/` (gitignored).
