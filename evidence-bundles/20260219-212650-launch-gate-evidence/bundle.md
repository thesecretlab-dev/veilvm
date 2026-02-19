# VEIL Launch-Gate Evidence Bundle

- Generated: 2026-02-19T21:34:11.422Z
- Node URL: `http://127.0.0.1:9660`
- Chain ID: `gbbsaboFf5jzM721wnMf82ZvZqF64a5i3xhPg4m6vhXeKmgbc`
- Verdict: **PASS**

## Checks

| Check | Status | Duration (s) | Accepted | Rejected | Missed | Output Dir | Notes |
|---|---|---:|---:|---:|---:|---|---|
| shielded-smoke | PASS | 215.0 | 1 | 0 | 0 | `./zkbench-out-evidence-20260219-212650-shielded-smoke` | accepted=1, rejected=0, missed=0 |
| timeout-drill-b8 | PASS | 209.9 | 0 | 0 | 0 | `./zkbench-out-evidence-20260219-212650-timeout-drill-b8` | expected timeout/deadline failure observed (non-zero exit) |

## Artifacts

- shielded-smoke
  - summary: `zkbench-out-evidence-20260219-212650-shielded-smoke\summary.json`
  - stdout: `evidence-bundles\20260219-212650-launch-gate-evidence\logs\shielded-smoke.stdout.log`
  - stderr: `evidence-bundles\20260219-212650-launch-gate-evidence\logs\shielded-smoke.stderr.log`
- timeout-drill-b8
  - summary: ``
  - stdout: `evidence-bundles\20260219-212650-launch-gate-evidence\logs\timeout-drill-b8.stdout.log`
  - stderr: `evidence-bundles\20260219-212650-launch-gate-evidence\logs\timeout-drill-b8.stderr.log`

