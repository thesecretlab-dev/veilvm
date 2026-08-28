# Local ZK / gossip 2026-08-27 (complete)

Local chain `bdRGUMA7rzZFXjbn1ePTjqhAUfTjW94e69p7qZd4puZ3uEosL` only. Not Fuji. Not mainnet.

## Threshold tx gossip (VTG2)

- `hypersdk/chain/shamir.go`: GF(256) Shamir split/combine
- `hypersdk/chain/tx_gossip_threshold.go`: VTG2 envelope, X25519 wrap of shares, AES-GCM data key
- Local daemon: `txGossipThresholdMinShares=1` and node X25519 private key in gitignored `.local/tx-gossip-x25519.key`
- Empty committee derives 1-of-1 from that private key
- Running plugin contains `VTG2`
- Tests: 1-of-1 roundtrip; auto-committee from priv; 2-of-3 single key fails; 2-of-3 two keys reconstruct

1-of-1 is not a private mempool. A committee t-of-n with t>1 is not running on this node.

## Shielded-ledger statement

Preimage is now domain tag + market + window + price + volume + fills + commitments + nullifiers + prev root + next root (241 bytes). Groth16 still SHA256-binds those public slots. Extra hashes are domain-separated SHA256 of the clear-batch fields, not merkle inclusion or matching inside the circuit.

- Fixture dir `zk-fixture-new`, VK sha256 `7618a647534c5cc47586f8ad778264a8dfc1a5da71e557db13607bfeae07a5a9`
- `go test ./zk -run TestPinned` PASS
- `privacy-loop` PASS: proveMs=715, clear `0xb0066eedf33c201f6df70f5e5ed7ad0e30a546747008172c02088b02853d9785`

Do not claim whitepaper-complete ZK validity.
