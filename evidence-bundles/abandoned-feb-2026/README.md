# Abandoned Feb 2026 companion L1

Date: 2026-08-24  
Runlist: **C05**

Do **not** reuse this packet. Do **not** fund it. Do **not** point new code at it.

## What died

A half-created companion / VEILPOS L1 from February 2026. Bootstrap validator:

`NodeID-D26idWcd6WaRS5vhrNhwMxLaG8f7WVztC` (weight 100, disconnected)

Connected weight was 1. 20% churn cap blocked `removeValidator`. Restoring the set needed ~399 AVAX or the lost D26 identity. NodeIDs from that set are not on the Primary Network now.

The address registry that targeted it (illegal companion `chainId` 22207, placeholder Teleporter/bridge) is archived here as `companion-evm.addresses.json`.

## Why 22207 is illegal on companion

`22207` is VeilVM’s HyperSDK **app id**, not an EVM chain id. Companion `eth_chainId` must be a distinct EVM id (local anvil = 31337).

## What local uses instead

- VeilVM: Windows AvalancheGo `network-id=local` HTTP `:9660`
- Companion rails: Foundry anvil `chainId` 31337 HTTP `:8545`
- Relayer: in-tree `cmd/veilvm-order-router` `:9098`

New Fuji/mainnet companion is a **new** Subnet-EVM L1 (Phase F). Operator keys: `evidence-bundles/key-map/operator-2026-08-24.json`. Lost owner `0xB9a05AFC8eff7eE6a84889Bb9C88A89eAA2f96af` is do-not-deploy-under.

## Code paths

`scripts/companion-evm.addresses.json` is the **live** registry. It must not copy this packet. `scripts/check-companion-primitives.mjs` fails closed if `chainId` is 22207 or if this abandoned file is passed in.
