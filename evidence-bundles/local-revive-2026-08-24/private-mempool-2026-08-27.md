# Private mempool 2026-08-27 (local)

Spec: fewer than threshold colluding validators cannot decrypt pre-reveal. t=1 is not a private mempool.

## Now live on local genesis node

- Gossip codec is VTG2, `t=2` `n=3`. Construction rejects `t<2` and `n<2`.
- Ciphertext is held until two committee shares combine. One node private key cannot open the envelope.
- Share announcements (`VTGS`) are X25519-wrapped to the committee, not plaintext shares on the wire.
- Plugin contains `VTG2` and `VTGS`. Chain config has minShares=2 and 3 committee pubs. Node holds only `node0.priv`.
- Tests: `go test ./chain -run 'Gossip|Shamir|Threshold'` PASS including share-exchange 2-of-3 and single-key hold.
- `privacy-loop` still PASS via RPC ingest (proveMs=792).

## Still not the full spec

- Local block producer still accepts plaintext `SubmitTx` (order-router). That is how a one-node chain includes txs. P2P gossip is what is threshold-dark.
- Production 13-of-20, DKG, decrypt-after-batch-close for *order envelopes* (already reveal_batch), and no-RPC-plaintext are not this node.
- A second process with `node1.priv` is required before gossiped txs can enter the mempool on this machine.
