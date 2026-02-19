# VEIL Launch-Gate Evidence Bundle

- Generated: 2026-02-19T21:51:30.803Z
- Node URL: `http://127.0.0.1:9660`
- Chain ID: `gbbsaboFf5jzM721wnMf82ZvZqF64a5i3xhPg4m6vhXeKmgbc`
- Verdict: **FAIL**

## Checks

| Check | Status | Duration (s) | Accepted | Rejected | Missed | Output Dir | Notes |
|---|---|---:|---:|---:|---:|---|---|
| shielded-smoke | PASS | 213.3 | 1 | 0 | 0 | `./zkbench-out-evidence-20260219-213645-shielded-smoke` | accepted=1, rejected=0, missed=0 |
| backup-takeover-primary-fails | PASS | 214.9 | 0 | 0 | 0 | `./zkbench-out-evidence-20260219-213645-backup-takeover-primary-fails` | primary prover rejected under backup authority gate |
| backup-takeover-backup-recovers | FAIL | 0.9 | 0 | 0 | 0 | `./zkbench-out-evidence-20260219-213645-backup-takeover-backup-recovers` | process exit code 1 |
| synthetic-negative | PASS | 5.0 | 0 | 0 | 0 | `./zkbench-out-evidence-20260219-213645-synthetic-negative` | expected fail-close proof rejection observed (non-zero exit) |
| malformed-proof | PASS | 213.1 | 0 | 0 | 0 | `./zkbench-out-evidence-20260219-213645-malformed-proof` | expected malformed-proof rejection observed (non-zero exit) |
| timeout-drill-b8 | PASS | 211.4 | 0 | 0 | 0 | `./zkbench-out-evidence-20260219-213645-timeout-drill-b8` | expected timeout/deadline failure observed (non-zero exit) |

## Artifacts

- shielded-smoke
  - summary: `zkbench-out-evidence-20260219-213645-shielded-smoke\summary.json`
  - stdout: `evidence-bundles\20260219-213645-launch-gate-evidence\logs\shielded-smoke.stdout.log`
  - stderr: `evidence-bundles\20260219-213645-launch-gate-evidence\logs\shielded-smoke.stderr.log`
- backup-takeover-primary-fails
  - summary: ``
  - stdout: `evidence-bundles\20260219-213645-launch-gate-evidence\logs\backup-takeover-primary-fails.stdout.log`
  - stderr: `evidence-bundles\20260219-213645-launch-gate-evidence\logs\backup-takeover-primary-fails.stderr.log`
- backup-takeover-backup-recovers
  - summary: ``
  - stdout: `evidence-bundles\20260219-213645-launch-gate-evidence\logs\backup-takeover-backup-recovers.stdout.log`
  - stderr: `evidence-bundles\20260219-213645-launch-gate-evidence\logs\backup-takeover-backup-recovers.stderr.log`
- synthetic-negative
  - summary: ``
  - stdout: `evidence-bundles\20260219-213645-launch-gate-evidence\logs\synthetic-negative.stdout.log`
  - stderr: `evidence-bundles\20260219-213645-launch-gate-evidence\logs\synthetic-negative.stderr.log`
- malformed-proof
  - summary: ``
  - stdout: `evidence-bundles\20260219-213645-launch-gate-evidence\logs\malformed-proof.stdout.log`
  - stderr: `evidence-bundles\20260219-213645-launch-gate-evidence\logs\malformed-proof.stderr.log`
- timeout-drill-b8
  - summary: ``
  - stdout: `evidence-bundles\20260219-213645-launch-gate-evidence\logs\timeout-drill-b8.stdout.log`
  - stderr: `evidence-bundles\20260219-213645-launch-gate-evidence\logs\timeout-drill-b8.stderr.log`

