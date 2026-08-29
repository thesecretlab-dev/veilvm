# Privacy loop 2026-08-25

Local Windows. Not Fuji. Does not claim mempool privacy (D06 still FAIL).

## What ran

`node scripts/privacy-loop.mjs` against genesis node `:9660` + router `:9098`.

```
1) router ok chain=bdRGUMA7rzZFXjbn1ePTjqhAUfTjW94e69p7qZd4puZ3uEosL proverReady=true
2) market nRojwjQ27X7escAGHx334jqBNXeMiMCps4G4E8AM8xKzihqN2
3) committed 0x64e1719d85b1f3903f3a2fbfe5d6208ab80bd318838bb7ce522268bb19b98270 window=1787633330000 (not cleared)
4) revealed 0x32da29d0fe6dad765932415da14752bd38dd3f3017fcf5d1461ded07f0daca05
5) proof 0x80505132b48a88a0e9b99006f9debad6f2a56e6584034210f2636a75a97a7482 proveMs=575
6) cleared 0x20ecc9ca7b9aae27a427245d1e8cf5ee227c7d47df6df938ae61ca1791d0c497 fills=0xf62c7c6cec797e3228242f9e85761a927ef130e61b4b8f4257490d848a886a74
PRIVACY-LOOP PASS
```

## What this is

- Native `/orders` commits a `VEILENC1` AES-256-GCM envelope. Chain stores ciphertext + sha256 commitment.
- `ClearBatch` refuses unless ≥1 `RevealBatch` share exists for that market/window.
- Operator `POST /native/settle-batch` posts the window key, proves `shielded-ledger-v1` groth16 (~575ms after CCS cache), submits proof, clears.
- Fills hash is operator-attested (`VEIL_FILLS_V1` domain), not a matching engine.

## What this is not

- Encrypted tx gossip / threshold decrypt (D06). Still not in the v1 binary.
- Fuji or mainnet private markets.
- 13-of-20 reveal committee (action 41 is spec-only).
