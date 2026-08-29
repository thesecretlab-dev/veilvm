# First-time VEIL local chain on this PC. Writes .local/local-chain.json. No miners. No Fuji.
$ErrorActionPreference = 'Stop'
$Root = Split-Path $PSScriptRoot -Parent
$Local = Join-Path $Root '.local'
$Data = Join-Path $Local 'nodedata'
$PluginDir = Join-Path $Local 'plugins'
$Logs = Join-Path $Local 'logs'
$Avago = Join-Path $Local 'avalanchego.exe'
$Vk = Join-Path $Root 'zk-fixture-new\groth16_shielded_ledger_vk.bin'
$VmId = 'u9GgvekeunSwK4TPF4jj7xLsW1LKkd1Uv9VQZo2SGfrwkejsK'
$ChainFile = Join-Path $Local 'local-chain.json'
$Foundry = if ($env:FOUNDRY_DIR) { $env:FOUNDRY_DIR } else { Join-Path $env:USERPROFILE 'tools\foundry' }
$NodeExe = 'C:\Program Files\nodejs\node.exe'
$env:Path = "$Foundry;C:\Program Files\nodejs;$env:Path"

function Wmi-Start($file, $argString, $cwd) {
  $cmd = if ($argString) { '"{0}" {1}' -f $file, $argString } else { '"{0}"' -f $file }
  $r = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{
    CommandLine = $cmd
    CurrentDirectory = $cwd
  }
  if ($r.ReturnValue -ne 0) { throw "WMI Create failed $($r.ReturnValue) for $file" }
  return $r.ProcessId
}

New-Item -ItemType Directory -Force -Path $Data, $PluginDir, $Logs, (Join-Path $Local 'pids'), (Join-Path $Data 'configs\chains') | Out-Null
if (-not (Test-Path $Avago)) { throw "missing $Avago" }
if (-not (Test-Path (Join-Path $PluginDir "$VmId.exe"))) { throw "missing plugin" }
if (-not (Test-Path $Vk)) { throw "missing $Vk" }

$CommitteeDir = Join-Path $Local 'tx-gossip-committee'
$CommitteeTool = Join-Path $Local 'veilvm-gossip-committee.exe'
if (-not (Test-Path (Join-Path $CommitteeDir 'committee.csv'))) {
  New-Item -ItemType Directory -Force -Path $CommitteeDir | Out-Null
  & $CommitteeTool -n 3 -out $CommitteeDir
  if ($LASTEXITCODE -ne 0) { throw 'committee keygen failed' }
}

# Phase 1: genesis node with no --track-subnets so setup-local can create the subnet.
Get-Process avalanchego -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

$bootArgs = @(
  '--network-id=local',
  '--sybil-protection-enabled=false',
  '--http-host=127.0.0.1',
  '--http-port=9660',
  '--staking-port=9661',
  "--plugin-dir=$PluginDir",
  "--data-dir=$Data",
  "--chain-config-dir=$(Join-Path $Data 'configs\chains')",
  '--log-level=info',
  '--public-ip=127.0.0.1',
  '--http-allowed-hosts=localhost,127.0.0.1'
) -join ' '

Write-Host 'starting avalanchego (no track-subnets yet)'
$pid1 = Wmi-Start $Avago $bootArgs $Root
Write-Host "avalanchego pid=$pid1"

$deadline = (Get-Date).AddSeconds(120)
$ok = $false
while ((Get-Date) -lt $deadline) {
  try {
    $h = Invoke-RestMethod 'http://127.0.0.1:9660/ext/health' -TimeoutSec 3
    if ($h.healthy) { $ok = $true; break }
  } catch {}
  Start-Sleep -Seconds 2
}
if (-not $ok) { throw 'avalanchego did not become healthy' }
Write-Host 'node healthy — running setup-local.mjs'

Set-Location (Join-Path $Root 'scripts')
& $NodeExe 'setup-local.mjs'
if ($LASTEXITCODE -ne 0) { throw "setup-local.mjs exit $LASTEXITCODE" }
if (-not (Test-Path $ChainFile)) { throw 'setup-local did not write local-chain.json' }
$meta = Get-Content $ChainFile -Raw | ConvertFrom-Json
Write-Host ("subnet={0} chain={1}" -f $meta.subnetID, $meta.chainID)

Write-Host 'restarting avalanchego with --track-subnets'
Get-Process avalanchego -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Process | Where-Object { $_.ProcessName -like 'u9Ggveke*' } | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2

$cfgDir = Join-Path $Data "configs\chains\$($meta.chainID)"
New-Item -ItemType Directory -Force -Path $cfgDir | Out-Null
$vkJson = $Vk.Replace('\', '\\')
$GossipKeyFile = Join-Path $Local 'tx-gossip.key'
if (-not (Test-Path $GossipKeyFile)) {
  $bytes = New-Object byte[] 32
  [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
  (($bytes | ForEach-Object { $_.ToString('x2') }) -join '') | Set-Content $GossipKeyFile -Encoding ascii
}
$gossipKey = (Get-Content $GossipKeyFile -Raw).Trim()
$x25519 = (Get-Content (Join-Path $CommitteeDir 'node0.priv') -Raw).Trim()
$committee = (Get-Content (Join-Path $CommitteeDir 'committee.csv') -Raw).Trim()
@"
{
  "vm": {
    "txGossipEncryptionRequired": true,
    "txGossipEncryptionKeyHex": "$gossipKey",
    "txGossipThresholdMinShares": 2,
    "txGossipThresholdNodePrivateKeyHex": "$x25519",
    "txGossipThresholdCommitteePublicKeys": "$committee"
  },
  "controller": {
    "enabled": true,
    "zk": {
      "enabled": true,
      "strict": true,
      "groth16VerifyingKeyPath": "$vkJson",
      "requiredCircuitID": "shielded-ledger-v1"
    }
  }
}
"@ | Set-Content (Join-Path $cfgDir 'config.json') -Encoding ascii

$trackArgs = @(
  '--network-id=local',
  '--sybil-protection-enabled=false',
  '--http-host=127.0.0.1',
  '--http-port=9660',
  '--staking-port=9661',
  "--plugin-dir=$PluginDir",
  "--data-dir=$Data",
  "--chain-config-dir=$(Join-Path $Data 'configs\chains')",
  '--log-level=info',
  '--public-ip=127.0.0.1',
  '--http-allowed-hosts=localhost,127.0.0.1',
  "--track-subnets=$($meta.subnetID)"
) -join ' '

$pid2 = Wmi-Start $Avago $trackArgs $Root
Write-Host "avalanchego tracked pid=$pid2"
$deadline = (Get-Date).AddSeconds(120)
$ok = $false
while ((Get-Date) -lt $deadline) {
  try {
    $h = Invoke-RestMethod 'http://127.0.0.1:9660/ext/health' -TimeoutSec 3
    if ($h.healthy) { $ok = $true; break }
  } catch {}
  Start-Sleep -Seconds 2
}
if (-not $ok) { throw 'tracked avalanchego not healthy' }
Write-Host 'CHAIN UP on this machine'
Write-Host ($ChainFile)
Get-Content $ChainFile -Raw
