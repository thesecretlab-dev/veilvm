# STALE-2026-02

Date: 2026-08-24  
Runlist item: **A05**

Feb 2026 local launch-gate evidence on this tree is **archaeology only**. It does **not** authorize Fuji or mainnet.

## Why stale

- Evidence was collected against a **local** node (`http://127.0.0.1:9660`) on the Josh host.
- Chain IDs in those bundles (`gbbsaboF…`, later `aQ8Ct8…`) are **not** registered on Avalanche mainnet or Fuji (Glacier 2026-08-24).
- `PASS (local)` / `GO FOR PRODUCTION` / `deploymentReady=true` were local-profile results.
- Hardened owner `0xB9a05AFC8eff7eE6a84889Bb9C88A89eAA2f96af` was already lost at the time of those greens.
- Companion Teleporter/bridge addresses in `scripts/companion-evm.addresses.json` are marked **placeholders**.

## Pointers quarantined

| Pointer | Old target | New |
|---|---|---|
| `evidence-bundles/latest-launch-gate-evidence.txt` | `20260219-231603-launch-gate-evidence` `overall_pass=True` | marked `STALE-2026-02` |
| All `20260219-*-launch-gate-evidence/` dirs | kept on disk | do not cite as `latest` |

New launch evidence must be written under dated dirs **after** 2026-08-24 and never reuse these filenames as `latest`.
