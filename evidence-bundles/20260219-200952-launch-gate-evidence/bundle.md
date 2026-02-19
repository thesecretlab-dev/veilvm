# VEIL Launch-Gate Evidence Bundle

- Generated: 2026-02-19T20:32:23.439Z
- Node URL: `http://127.0.0.1:9660`
- Chain ID: `gbbsaboFf5jzM721wnMf82ZvZqF64a5i3xhPg4m6vhXeKmgbc`
- Verdict: **FAIL**

## Checks

| Check | Status | Duration (s) | Accepted | Rejected | Missed | Output Dir | Notes |
|---|---|---:|---:|---:|---:|---|---|
| shielded-smoke | PASS | 266.7 | 1 | 0 | 0 | `./zkbench-out-evidence-20260219-200952-shielded-smoke` | accepted=1, rejected=0, missed=0 |
| synthetic-negative | PASS | 6.1 | 0 | 0 | 0 | `./zkbench-out-evidence-20260219-200952-synthetic-negative` | expected fail-close proof rejection observed (non-zero exit) |
| malformed-proof | PASS | 262.2 | 0 | 0 | 0 | `./zkbench-out-evidence-20260219-200952-malformed-proof` | expected malformed-proof rejection observed (non-zero exit) |
| timeout-drill-b8 | FAIL | 266.1 | 1 | 0 | 0 | `./zkbench-out-evidence-20260219-200952-timeout-drill-b8` | no missed deadline or rejection observed |
| timeout-drill-b32 | FAIL | 259.7 | 1 | 0 | 0 | `./zkbench-out-evidence-20260219-200952-timeout-drill-b32` | no missed deadline or rejection observed |
| backup-takeover-primary-fails | PASS | 259.9 | 0 | 0 | 0 | `./zkbench-out-evidence-20260219-200952-backup-takeover-primary-fails` | primary prover rejected under backup authority gate |
| backup-takeover-backup-recovers | FAIL | 1.0 | 0 | 0 | 0 | `./zkbench-out-evidence-20260219-200952-backup-takeover-backup-recovers` | process exit code 1 |

## Artifacts

- shielded-smoke
  - summary: `zkbench-out-evidence-20260219-200952-shielded-smoke\summary.json`
  - stdout: `evidence-bundles\20260219-200952-launch-gate-evidence\logs\shielded-smoke.stdout.log`
  - stderr: `evidence-bundles\20260219-200952-launch-gate-evidence\logs\shielded-smoke.stderr.log`
- synthetic-negative
  - summary: ``
  - stdout: `evidence-bundles\20260219-200952-launch-gate-evidence\logs\synthetic-negative.stdout.log`
  - stderr: `evidence-bundles\20260219-200952-launch-gate-evidence\logs\synthetic-negative.stderr.log`
- malformed-proof
  - summary: ``
  - stdout: `evidence-bundles\20260219-200952-launch-gate-evidence\logs\malformed-proof.stdout.log`
  - stderr: `evidence-bundles\20260219-200952-launch-gate-evidence\logs\malformed-proof.stderr.log`
- timeout-drill-b8
  - summary: `zkbench-out-evidence-20260219-200952-timeout-drill-b8\summary.json`
  - stdout: `evidence-bundles\20260219-200952-launch-gate-evidence\logs\timeout-drill-b8.stdout.log`
  - stderr: `evidence-bundles\20260219-200952-launch-gate-evidence\logs\timeout-drill-b8.stderr.log`
- timeout-drill-b32
  - summary: `zkbench-out-evidence-20260219-200952-timeout-drill-b32\summary.json`
  - stdout: `evidence-bundles\20260219-200952-launch-gate-evidence\logs\timeout-drill-b32.stdout.log`
  - stderr: `evidence-bundles\20260219-200952-launch-gate-evidence\logs\timeout-drill-b32.stderr.log`
- backup-takeover-primary-fails
  - summary: ``
  - stdout: `evidence-bundles\20260219-200952-launch-gate-evidence\logs\backup-takeover-primary-fails.stdout.log`
  - stderr: `evidence-bundles\20260219-200952-launch-gate-evidence\logs\backup-takeover-primary-fails.stderr.log`
- backup-takeover-backup-recovers
  - summary: ``
  - stdout: `evidence-bundles\20260219-200952-launch-gate-evidence\logs\backup-takeover-backup-recovers.stdout.log`
  - stderr: `evidence-bundles\20260219-200952-launch-gate-evidence\logs\backup-takeover-backup-recovers.stderr.log`

