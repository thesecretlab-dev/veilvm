# VEIL Companion EVM Primitives Checklist

Status: Active  
Date: 2026-02-19  
Scope: Required for bridge-enabled launch with companion EVM rails

## 1. Purpose

Define the exact primitive set needed so VEIL can bridge and interoperate safely without weakening VeilVM consensus invariants.

## 2. MUST-Have Primitives

| Primitive | Why it is required | Pass condition |
|---|---|---|
| `AWM` + `Teleporter` | Canonical cross-chain messaging for VeilVM <-> EVM rail | Bidirectional message test passes with finalized tx hashes recorded |
| `NativeMinter` | Controlled mint/burn for wrapped bridge assets (`WVEIL`, wrappers) | Unauthorized mint/burn reverts; authorized bridge role succeeds |
| `TxAllowList` | Permissioned transaction posture | Non-allowlisted sender tx is rejected on-chain |
| `ContractDeployerAllowList` | Restrict deploy surface on permissioned rail | Non-allowlisted deploy attempt fails on-chain |
| Companion chain config freeze | Prevent timestamp/fee/admin drift at launch | Final `config.json` hash and admin set archived |

## 3. SHOULD-Have Primitives

| Primitive | Why it is recommended | Pass condition |
|---|---|---|
| `FeeConfigManager` | Tune fees without hard fork | Governance/admin fee update tx tested |
| `Multicall3` | Standard read batching for dashboards/frontends | Read batch integration test passes |
| `CREATE2` deployer (`0x4e59...B4956C`) | Deterministic contract addresses | Deterministic deploy smoke test passes |
| Faucet (dev/test only) | Fast iteration for test wallets | Rate-limited faucet smoke test passes |

## 4. Canonical Asset Model

- `VEIL`: native asset on VeilVM.
- `WVEIL`: `1:1` wrapped representation for EVM rails and bridges; non-yield.
- `vVEIL`: staking/yield derivative; not a bridge primitive and not collateral-enabled in v1 (`LTV = 0`).
- Critical treasury, COL, and VAI risk invariants remain VeilVM consensus rules, not EVM-only policy.

## 5. Required Acceptance Tests

1. Teleporter message round-trip: VeilVM -> EVM and EVM -> VeilVM.
2. Bridge flow: lock/burn VEIL path and mint/release WVEIL path with replay protection.
3. Unauthorized tx submission rejected by `TxAllowList`.
4. Unauthorized contract deployment rejected by `ContractDeployerAllowList`.
5. Unauthorized `NativeMinter` call rejected; authorized bridge operator call succeeds.
6. Companion EVM chain config and precompile activation timestamps match frozen launch config.
7. Address registry file and deployment tx hashes are committed to repo docs.

Allowlist correctness rule:

- `adminAddresses` and `enabledAddresses` must be disjoint for each allowlist precompile (`TxAllowList`, `ContractDeployerAllowList`, `NativeMinter`).
- Admin role already implies enabled access; duplicate membership can invalidate config verification.

## 6. Required Artifacts

- `scripts/companion-evm.addresses.template.json` (template)
- `scripts/companion-evm.addresses.json` (generated from template)
- deployment tx hash log for each primitive
- final companion chain config hash and signer set
- primitive registry validation command: `npm run check:companion-primitives` (from `scripts/`)
- policy cross-check command (upgrade + roles): `npm run check:companion-policy` (from `scripts/`)

## 7. Current Local Snapshot (2026-02-19)

Current state from `scripts/companion-evm.addresses.json` and `avalanche-l1-docker/data/avalanche-cli/subnets/VEIL2/upgrade.json`:

- `chainId=22207`, RPC path populated and live in local env
- `TxAllowList`, `ContractDeployerAllowList`, and `NativeMinter` are enabled via `adminAddresses`/`enabledAddresses`
- `npm run check:companion-policy` currently returns `PASS`
- Teleporter messenger/registry and bridge minter addresses are populated, but the deployed contracts are placeholder/minimal local versions and must be replaced for production
- upgrade config hash (current local): `F5A5D890A33E475CDC519C4ABB271BFEE5BD38FF8DD8237634FB1AE83E58FA43`

Known script issue:

- `npm run check:companion-primitives` currently resolves the default registry path as `scripts/scripts/companion-evm.addresses.json` and fails unless explicit args are passed.

## 8. Parallel Ownership Split

- Claude: companion EVM config, precompile enablement, contract deployment, and bridge smoke tests.
- Codex: VeilVM-side invariant enforcement, bridge policy checks, and launch gate integration.

## 9. Launch Gate Rule

Bridge-enabled launch is blocked until every MUST item in this checklist is green with reproducible evidence.
