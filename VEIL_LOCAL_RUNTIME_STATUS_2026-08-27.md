# VEIL local runtime status

Canonical copy lives in **veil-docs**: `specs/VEIL_LOCAL_RUNTIME_STATUS_2026-08-27.md`.

Keep this file in lockstep when that spec changes.

Date: 2026-08-27. Local ≠ Fuji ≠ mainnet.

- Gossip: VTG2 Shamir + X25519, **t=2 n=3**. Encryption-required implies threshold. Outer VTG1 rejected. One key cannot decrypt.
- Orders: `VEILENC1` envelopes; reveal then groth16 `shielded-ledger-v1` clear (digest-bound slots, not in-circuit matching).
- RPC `SubmitTx` on this solo node is still plaintext ingest.
- Economy: native VAI mint/burn, AMM, fee router 70/20/10, COL tranche — `scripts/ecosystem-loop.mjs`.
- Rails: anvil 31337 intent gateways + relayer — `scripts/local-stack-e2e.mjs`.
