# Tape is owned by VEIL-local-genesis-node. A second ONLOGON task was killing and
# relaunching live-activity on every login. This script disables that duplicate.
$ErrorActionPreference = "Stop"
$TaskName = "VEIL-local-activity"

$svc = New-Object -ComObject Schedule.Service
$svc.Connect()
$folder = $svc.GetFolder("\")
try {
  $existing = $folder.GetTask($TaskName)
  $existing.Enabled = $false
  $folder.RegisterTaskDefinition($TaskName, $existing.Definition, 4, $null, $null, 3) | Out-Null
} catch {
  # nothing to disable
}

try { schtasks /Change /TN $TaskName /DISABLE | Out-Null } catch {}
Write-Host "disabled $TaskName (tape starts with VEIL-local-genesis-node only)"
Write-Host "demand helper still exists: scripts\start-live-activity.ps1 (idempotent, no kill)"
