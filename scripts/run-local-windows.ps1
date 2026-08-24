# Local VeilVM node on Windows. Does not use Docker.
# First boot: no --track-subnets. After setup-local.mjs, rerun with -TrackSubnet <id>.
param(
  [string]$TrackSubnet = ""
)

$ErrorActionPreference = "Stop"
$Root = Split-Path $PSScriptRoot -Parent
$Local = Join-Path $Root ".local"
$Data = Join-Path $Local "nodedata"
$PluginDir = Join-Path $Local "plugins"
$Avago = Join-Path $Local "avalanchego.exe"
$Vk = Join-Path $Root "zk-fixture-new\groth16_shielded_ledger_vk.bin"
$VmId = "u9GgvekeunSwK4TPF4jj7xLsW1LKkd1Uv9VQZo2SGfrwkejsK"

if (-not (Test-Path $Avago)) { throw "missing $Avago — build avalanchego first" }
if (-not (Test-Path (Join-Path $PluginDir "$VmId.exe"))) { throw "missing veilvm plugin in $PluginDir" }
if (-not (Test-Path $Vk)) { throw "missing shielded ledger VK $Vk" }

New-Item -ItemType Directory -Force -Path $Data | Out-Null

# Plugin subprocess does not inherit VEIL_ZK_* env. Chain config is the only Windows path.
$chainIdFile = Join-Path $Local "local-chain.json"
$chainId = "bdRGUMA7rzZFXjbn1ePTjqhAUfTjW94e69p7qZd4puZ3uEosL"
if (Test-Path $chainIdFile) {
  $meta = Get-Content $chainIdFile -Raw | ConvertFrom-Json
  if ($meta.chainID) { $chainId = $meta.chainID }
  if (-not $TrackSubnet -and $meta.subnetID) { $TrackSubnet = $meta.subnetID }
}
$cfgDir = Join-Path $Data "configs\chains\$chainId"
New-Item -ItemType Directory -Force -Path $cfgDir | Out-Null
$vkJson = $Vk.Replace('\', '\\')
@"
{
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
"@ | Set-Content -Path (Join-Path $cfgDir "config.json") -Encoding ascii

$args = @(
  "--network-id=local",
  "--sybil-protection-enabled=false",
  "--http-host=127.0.0.1",
  "--http-port=9660",
  "--staking-port=9661",
  "--plugin-dir=$PluginDir",
  "--data-dir=$Data",
  "--chain-config-dir=$(Join-Path $Data 'configs\chains')",
  "--log-level=info",
  "--public-ip=127.0.0.1",
  "--http-allowed-hosts=localhost,127.0.0.1"
)
if ($TrackSubnet) {
  $args += "--track-subnets=$TrackSubnet"
}

Write-Host "starting avalanchego network-id=local http=127.0.0.1:9660 plugin=$VmId"
Write-Host "zk circuit=shielded-ledger-v1"
& $Avago @args
