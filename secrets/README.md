# Secrets Directory

Do not commit raw private keys here.

Use environment variables for local runs:
- PRIVATE_KEY
- REFUEL_PRIVATE_KEY
- PROOF_CONFIG_PRIVATE_KEY
- PROVER_AUTHORITY_PRIVATE_KEY

If local fixtures are required, keep them outside git and load via explicit file paths.

## Operator keybox (2026-08-24)

Private hex files live **outside this repo**:

`C:\Users\Justin\tools\veil-keybox\private\`

Public addresses: `evidence-bundles/key-map/operator-2026-08-24.json`

Bitcoin seed-funds receive address (mainnet P2WPKH, BIP84 `m/84'/0'/0'/0/0`):

`bc1qvl34v5qda2v2mj9jpg37makn7qvc3mmagv0uvu`

Mnemonic / WIF: `C:\Users\Justin\tools\veil-keybox\private\btc-seed-funds.json` (not git).

Do not reuse lost hardened owner `0xB9a05AFC8eff7eE6a84889Bb9C88A89eAA2f96af`.

