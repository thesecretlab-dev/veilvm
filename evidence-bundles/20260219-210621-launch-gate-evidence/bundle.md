# VEIL Launch-Gate Evidence Bundle

- Generated: 2026-02-19T21:14:22.405Z
- Node URL: `http://127.0.0.1:9660`
- Chain ID: `gbbsaboFf5jzM721wnMf82ZvZqF64a5i3xhPg4m6vhXeKmgbc`
- Verdict: **FAIL**

## Checks

| Check | Status | Duration (s) | Accepted | Rejected | Missed | Output Dir | Notes |
|---|---|---:|---:|---:|---:|---|---|
| shielded-smoke | PASS | 217.9 | 1 | 0 | 0 | `./zkbench-out-evidence-20260219-210621-shielded-smoke` | accepted=1, rejected=0, missed=0 |
| backup-takeover-primary-fails | PASS | 215.5 | 0 | 0 | 0 | `./zkbench-out-evidence-20260219-210621-backup-takeover-primary-fails` | primary prover rejected under backup authority gate |
| backup-takeover-backup-recovers | FAIL | 1.0 | 0 | 0 | 0 | `./zkbench-out-evidence-20260219-210621-backup-takeover-backup-recovers` | process exit code 1 |

## Artifacts

- shielded-smoke
  - summary: `zkbench-out-evidence-20260219-210621-shielded-smoke\summary.json`
  - stdout: `evidence-bundles\20260219-210621-launch-gate-evidence\logs\shielded-smoke.stdout.log`
  - stderr: `evidence-bundles\20260219-210621-launch-gate-evidence\logs\shielded-smoke.stderr.log`
- backup-takeover-primary-fails
  - summary: ``
  - stdout: `evidence-bundles\20260219-210621-launch-gate-evidence\logs\backup-takeover-primary-fails.stdout.log`
  - stderr: `evidence-bundles\20260219-210621-launch-gate-evidence\logs\backup-takeover-primary-fails.stderr.log`
- backup-takeover-backup-recovers
  - summary: ``
  - stdout: `evidence-bundles\20260219-210621-launch-gate-evidence\logs\backup-takeover-backup-recovers.stdout.log`
  - stderr: `evidence-bundles\20260219-210621-launch-gate-evidence\logs\backup-takeover-backup-recovers.stderr.log`

