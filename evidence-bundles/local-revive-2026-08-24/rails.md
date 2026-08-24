# Local companion rails 2026-08-24

Anvil chainId **31337**. Not Fuji. Teleporter is `LocalTeleporter` (mock), not ICTT.

## Deployed (`scripts/deploy-rails.mjs`)

| Contract | Address |
|---|---|
| WVEIL | `0x9A676e781A523b5d0C0e43731313A708CB607508` |
| Faucet | `0x0B306BF915C4d645ff596e518fAf3F9669b97016` |
| Order gateway | `0x959922bE3CAee4b8Cd9a407cc3ac1C251C2007B1` |
| Liquidity gateway | `0x9A9f2CCfdE556A7E9Ff0848998Aa4a0CFD8863AE` |
| LocalTeleporter | `0x68B1D87F95878fE05B998F19b66F4baba5De1aed` |
| VeilBridgeMinter | `0x3Aa5ebB10DC797CAC828524e59A333d0A371443c` |

`npm run check:companion-primitives` PASS (local-mock profile).

## Relayer e2e (persisted gateways, not a fresh deploy)

- Order `submitIntent` → `/evm/intents/execute` → `CommitOrder` → `markIntentExecuted` state **2**
- Liquidity same, state **2**
- Veil txs `0x43fc08d8…` (order), `0x30a185cc…` (liq)

C04 local dual-chain = this seam. D09 local = this registry. Fuji ICTT is still Phase F.
