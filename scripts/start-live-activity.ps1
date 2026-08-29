# Demand helper for the local tape. Idempotent: does not kill a running tape.
# Daily owner is VEIL-local-genesis-node (start-local-daemon.ps1). Do not register this at logon.
$ErrorActionPreference = "Stop"
$Root = Split-Path $PSScriptRoot -Parent
$Logs = Join-Path $Root ".local\logs"
$Pids = Join-Path $Root ".local\pids"
$NodeExe = "C:\Program Files\nodejs\node.exe"
$Foundry = "C:\Users\Justin\tools\foundry"
New-Item -ItemType Directory -Force -Path $Logs, $Pids | Out-Null
$env:Path = "$Foundry;C:\Program Files\nodejs;$env:Path"
$env:ORDER_ROUTER_URL = "http://127.0.0.1:9098"
$env:ORDER_ROUTER_RELAY_SECRET = "local-dev-secret"
$env:CAST_BIN = Join-Path $Foundry "cast.exe"
$env:EVM_RELAY_EXECUTOR_PRIVATE_KEY = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

$pidFile = Join-Path $Pids "live-activity.pid"
if (Test-Path $pidFile) {
  $id = 0
  if ([int]::TryParse(((Get-Content $pidFile -Raw).Trim()), [ref]$id)) {
    if (Get-Process -Id $id -ErrorAction SilentlyContinue) {
      Write-Host "live-activity already running pid=$id"
      return
    }
  }
}
$logFile = Join-Path $Logs "live-activity.log"
if ((Test-Path $logFile) -and (((Get-Date) - (Get-Item $logFile).LastWriteTime).TotalSeconds -lt 45)) {
  Write-Host "live-activity log is fresh; skip"
  return
}

$ok = $false
try { $ok = [bool](Invoke-RestMethod "http://127.0.0.1:9098/health" -TimeoutSec 3).ok } catch {}
if (-not $ok) { throw "router not up on 9098; start VEIL-local-genesis-node first" }

$script = Join-Path $PSScriptRoot "live-activity.mjs"
$created = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{
  CommandLine      = "`"$NodeExe`" `"$script`""
  CurrentDirectory = $PSScriptRoot
}
if (-not $created -or $created.ReturnValue -ne 0) {
  throw "Win32_Process.Create failed: $($created.ReturnValue)"
}
Set-Content -Path $pidFile -Value $created.ProcessId -Encoding ascii
Write-Host "live-activity pid=$($created.ProcessId) log=$logFile"
