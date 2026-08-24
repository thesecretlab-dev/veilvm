# D10 opaque intent events

Gateway events emit commitment / nullifier / envelopeHash only.

`VeilOrderIntentGateway.sol` `IntentSubmitted(intentId, commitment, nullifier, envelopeHash, nonce)`  
`VeilLiquidityIntentGateway.sol` `LiquidityIntentSubmitted(...)` same shape.

Envelope bytes stay off-chain (mailbox). Router checks `sha256(envelope) == commitment` then `CommitOrder`.

PASS for current gateway sources.
