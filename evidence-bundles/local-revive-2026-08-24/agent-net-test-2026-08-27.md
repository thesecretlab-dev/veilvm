# Local network test 2026-08-27

Chain `bdRGUMA7rzZFXjbn1ePTjqhAUfTjW94e69p7qZd4puZ3uEosL` at 127.0.0.1:9660. Not Fuji. Not mainnet.

Five local qwen agents were launched in parallel and stalled on the 3090 (turn 1, no completed scripts). The same checks were then run directly.

| Check | Result |
| --- | --- |
| `/ext/health` | PASS healthy, chain check present |
| `platform.getBlockchains` | PASS listed |
| `veilvm.genesis` | PASS initialRules |
| proofConfig | PASS require_proof, groth16 type 1, 5s window |
| gossip config | PASS AES required + threshold minShares=1, x25519 key present (not printed) |
| running plugin | PASS 64165754 bytes, VTG2 + VEIL_COMMITMENTS_V1 |
| smoke-local.mjs | PASS height 12758 -> 12788 |
| top-to-bottom.mjs | PASS order `0x27ca76d2…` window 1787878660000 |
| privacy-loop.mjs | PASS proveMs=795 clear `0xd1c804b6…` |
| anvil 31337 rails | PASS gateway/wVEIL/bridge bytecode deployed; EOAs empty as expected |
| frontend :3000 | PASS HTTP 200; `/api/health` 404 (no such route) |

Do not claim a private mempool (local gossip is 1-of-1). Polygon passthrough returned 501 catalog-only.
