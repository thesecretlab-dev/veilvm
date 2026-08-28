# Hardening + ecosystem 2026-08-27

Local only. Gossip `t=2 n=3`.

## Hardening

- `txGossipEncryptionRequired=true` now **requires** VTG2 threshold (`t>=2`), not shared AES (VTG1).
- Threshold codec rejects outer VTG1. Tests: `TestConfiguredRequiredImpliesThreshold`, `TestThresholdRejectsVTG1`.
- Router native economy: `/native/burn-vai`, `/native/route-fees`, `/native/release-col`.

RPC `SubmitTx` is still plaintext to the local block producer.

## Ecosystem-loop PASS

- mint `0x10b6a6aa…` burn `0x13c37be9…`
- add/swap/remove LP `0x37f9a24b…` / `0x2d3fc0f9…` / `0x455ab6bf…`
- route_fees 70/20/10 budgets 70/20/10
- COL released 1000, locked 899999000

## Opaque rails E2E PASS

Order + liquidity intents on anvil 31337, relayed, gateway state EXECUTED (2).
