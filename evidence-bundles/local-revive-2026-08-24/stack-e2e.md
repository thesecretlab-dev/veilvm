# Local stack tests 2026-08-24

Out-of-box runner: `scripts/run-local-stack.ps1`

## Native AMM (`veilvm-smoke`)

PASS: mint VAI, add VEIL/VAI liquidity, swap VEIL→VAI, `lpbalance` RPC.

## Dual-chain E2E (`scripts/local-stack-e2e.mjs`)

PASS:

1. Create market on VeilVM
2. Companion anvil (chainId 31337) order gateway `submitIntent`
3. Relayer → VeilVM `CommitOrder` → `markIntentExecuted` (state 2)
4. Mint VAI on VeilVM
5. Liquidity gateway `submitIntent` → `AddLiquidity` → marked EXECUTED

## Fixes in this pass

- `MintVAI` / `AddLiquidity` state keys use `All` (first-time allocate)
- JSON-RPC aliases `lpbalance` / `vaistate` / `vaibalance` (gorilla method casing)
- HyperSDK `Keys.WithoutPermissions` no longer injects empty keys
- Smoke is idempotent on a live pool
- Order-router `/native/mint-vai` + `/native/create-market`
