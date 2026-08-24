# VEIL source inventory

Date: 2026-08-24  
Operator PC: `C:\Users\Justin` (Grok Build, 3090 host)  
Runlist item: **A01**  
Status: **PASS** (GitHub org + this PC). Josh machine tree **not present**.

## Trees on this PC (cloned 2026-08-24)

All clones live under `C:\Users\Justin\src\veil\`.

| Repo | Branch | HEAD SHA | Last commit | Actions 19–41? |
|---|---|---|---|---|
| `thesecretlab-dev/veilvm` | main | `9ce05eec1a3bc11df1def10d5a134e4c29803ac3` | 2026-02-28 — Remove private repo notes | **No.** `consts/types.go` IDs **0–18** only. |
| `thesecretlab-dev/veil-docs` | main | `1847a9c5707526b882e7820c38e4b8b7d5c50352` | 2026-08-24 — mainnet runlist | n/a (docs) |
| `thesecretlab-dev/veil-contracts` | main | `7ca0665edc52a0be39cc79f13f57ea5bfd910011` | 2026-02-25 | n/a |
| `thesecretlab-dev/veil-frontend` | main | `db804119be0dc23de2c60c7fec6695671dab1a84` | 2026-07-06 — un-claim mainnet | n/a |
| `thesecretlab-dev/zeroid` | main | `7457f5bb1c1be4929765193f9ef740bf1fa07d81` | 2026-02-28 | n/a |
| `thesecretlab-dev/anima` | main | `410c65fe473880b2041a87f0a15fe4bec5dc2c7e` | 2026-02-28 | claims 42 actions in docs |
| `thesecretlab-dev/veildb` | master | `b811160d6a9a2b3003ca3e679947bf0243995cc5` | 2026-02-27 | n/a |
| `thesecretlab-dev/veil-internal` | main | `cd8415969d9c64578dfb1c32cd8fbb3c611599d3` | 2026-02-28 | handshake claims 0–41 |

Not cloned (A07 names them as optional extras, not exit-gate): `veil-wallet`, `anima-runtime`, `anima-dashboard`, `maestro`, `leviathan`.

## Trees **not** found

| Location | Result |
|---|---|
| `C:\Users\Josh\hypersdk\examples\veilvm` | **Missing.** This is a Justin PC, not the Josh operator host. |
| Any `hypersdk` parent for `go.mod` `replace => ../../` | **Missing.** `veilvm/go.mod` still assumes the VM lives at `hypersdk/examples/veilvm`. |
| Zip / backup of the 42-action tree | **Not on this disk** (searched home + `src`). |

Handshake evidence that a 42-action tree existed **on Josh's machine**:

- `veil-internal/claude-handshake/backend-protocol-review-2026-02-22.md` cites `C:\Users\Josh\Desktop\veil-automaton\docs\veil-native-api-v1-mapping.md` for IDs `0..41`.
- `veil-frontend/docs/ANIMA_ARCHITECTURE.md` and `anima/packages/orchestrator/docs/ANIMA_ARCHITECTURE.md` list domains Bonds 19–23, Yield 24–29, Staking 30–34, Oracle 35–36, Reputation 37–38, Admin 39–41.
- `veil-docs/specs/BOND_MARKETS_V2.md` specifies actions 19–28 in prose. **No matching `actions/*.go` files** in GitHub `veilvm`.

## A02 recommendation (pending tag)

Promote GitHub `veilvm` **`9ce05eec1a3bc11df1def10d5a134e4c29803ac3`** as `VEILVM_CANONICAL_SHA`.

Do **not** wait for the Josh 42-action tree. Port 19–41 from specs into this SHA, or officially reduce the public claim to 19 actions (IDs 0–18). Tag `veilvm-canonical-2026-08-24` after `go build ./...` is green (blocked today by missing HyperSDK parent).
