# VEIL Launch-Gate Evidence Bundle

- Generated: 2026-02-19T18:56:35.556Z
- Node URL: `http://127.0.0.1:9660`
- Chain ID: `gbbsaboFf5jzM721wnMf82ZvZqF64a5i3xhPg4m6vhXeKmgbc`
- Verdict: **PASS**

## Checks

| Check | Status | Duration (s) | Accepted | Rejected | Missed | Output Dir | Notes |
|---|---|---:|---:|---:|---:|---|---|
| shielded-smoke | PASS | 226.1 | 1 | 0 | 0 | `./zkbench-out-evidence-20260219-184441-shielded-smoke` | accepted=1, rejected=0, missed=0 |
| synthetic-negative | PASS | 5.0 | 0 | 0 | 0 | `./zkbench-out-evidence-20260219-184441-synthetic-negative` | expected fail-close proof rejection observed (non-zero exit) |
| timeout-drill-b8 | FAIL | 229.4 | 1 | 0 | 0 | `./zkbench-out-evidence-20260219-184441-timeout-drill-b8` | no missed deadline or rejection observed |
| timeout-drill-b32 | PASS | 237.8 | 0 | 0 | 0 | `./zkbench-out-evidence-20260219-184441-timeout-drill-b32` | expected timeout/deadline failure observed (non-zero exit) |

## Artifacts

- shielded-smoke
  - summary: `zkbench-out-evidence-20260219-184441-shielded-smoke\summary.json`
  - stdout: `evidence-bundles\20260219-184441-launch-gate-evidence\logs\shielded-smoke.stdout.log`
  - stderr: `evidence-bundles\20260219-184441-launch-gate-evidence\logs\shielded-smoke.stderr.log`
- synthetic-negative
  - summary: ``
  - stdout: `evidence-bundles\20260219-184441-launch-gate-evidence\logs\synthetic-negative.stdout.log`
  - stderr: `evidence-bundles\20260219-184441-launch-gate-evidence\logs\synthetic-negative.stderr.log`
- timeout-drill-b8
  - summary: `zkbench-out-evidence-20260219-184441-timeout-drill-b8\summary.json`
  - stdout: `evidence-bundles\20260219-184441-launch-gate-evidence\logs\timeout-drill-b8.stdout.log`
  - stderr: `evidence-bundles\20260219-184441-launch-gate-evidence\logs\timeout-drill-b8.stderr.log`
- timeout-drill-b32
  - summary: ``
  - stdout: `evidence-bundles\20260219-184441-launch-gate-evidence\logs\timeout-drill-b32.stdout.log`
  - stderr: `evidence-bundles\20260219-184441-launch-gate-evidence\logs\timeout-drill-b32.stderr.log`

