# One-command local VEIL stack: node + anvil + router + tests.
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\run-local-stack.ps1
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\run-local-stack.ps1 -SkipTests
param(
  [switch]$SkipTests,
  [switch]$Restart
)

$ErrorActionPreference = "Stop"
$Root = Split-Path $PSScriptRoot -Parent
$Local = Join-Path $Root ".local"
$PluginDir = Join-Path $Local "plugins"
$Data = Join-Path $Local "nodedata"
$Avago = Join-Path $Local "avalanchego.exe"
$Router = Join-Path $Local "veilvm-order-router.exe"
$Vk = Join-Path $Root "zk-fixture-new\groth16_shielded_ledger_vk.bin"
$VmId = "u9GgvekeunSwK4TPF4jj7xLsW1LKkd1Uv9VQZo2SGfrwkejsK"
$ChainFile = Join-Path $Local "local-chain.json"
$Mingw = "C:\Users\Justin\tools\mingw64\bin"
$Foundry = "C:\Users\Justin\tools\foundry"
$NodeDir = "C:\Program Files\nodejs"
$env:Path = "$Mingw;$Foundry;$NodeDir;$env:Path"
$env:CGO_ENABLED = "1"
$env:CC = "gcc"
$env:CGO_CFLAGS = "-O2 -D__BLST_PORTABLE__"

function Wait-Http($url, $seconds) {
  for ($i = 0; $i -lt $seconds; $i++) {
    try {
      $r = Invoke-WebRequest -UseBasicParsing -Uri $url -TimeoutSec 2
      if ($r.StatusCode -ge 200) { return }
    } catch {}
    Start-Sleep -Seconds 1
  }
  throw "timeout waiting for $url"
}

function Stop-Stack {
  Get-Process avalanchego, veilvm-order-router, anvil -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
  Get-Process | Where-Object { $_.ProcessName -like "u9Ggveke*" } | Stop-Process -Force -ErrorAction SilentlyContinue
  Start-Sleep -Seconds 1
  Remove-Item -Force -ErrorAction SilentlyContinue (Join-Path $PluginDir "$VmId.exe~")
}

if ($Restart) { Stop-Stack }

New-Item -ItemType Directory -Force -Path $Local, $PluginDir, $Data | Out-Null

if (-not (Test-Path $Avago)) {
  throw "missing $Avago (build avalanchego first)"
}
if (-not (Test-Path $Vk)) { throw "missing $Vk" }

Write-Host "building veilvm plugin + order-router"
Set-Location $Root
go build -o (Join-Path $PluginDir "$VmId.exe") ./cmd/veilvm
go build -o $Router ./cmd/veilvm-order-router
go build -o (Join-Path $Local "veilvm-smoke.exe") ./cmd/veilvm-smoke

$healthy = $false
try {
  $h = Invoke-RestMethod "http://127.0.0.1:9660/ext/health"
  $healthy = [bool]$h.healthy
} catch {}
if (-not $healthy) {
  if (-not (Test-Path $ChainFile)) { throw "missing $ChainFile — run setup-local.mjs once first" }
  $meta = Get-Content $ChainFile -Raw | ConvertFrom-Json
  $cfgDir = Join-Path $Data "configs\chains\$($meta.chainID)"
  New-Item -ItemType Directory -Force -Path $cfgDir | Out-Null
  $vkJson = $Vk.Replace('\', '\\')
  $GossipKeyFile = Join-Path $Local "tx-gossip.key"
  if (-not (Test-Path $GossipKeyFile)) {
    $bytes = New-Object byte[] 32
    [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    (($bytes | ForEach-Object { $_.ToString("x2") }) -join "") | Set-Content $GossipKeyFile -Encoding ascii
  }
  $gossipKey = (Get-Content $GossipKeyFile -Raw).Trim()
  $CommitteeDir = Join-Path $Local "tx-gossip-committee"
  $CommitteeTool = Join-Path $Local "veilvm-gossip-committee.exe"
  if (-not (Test-Path (Join-Path $CommitteeDir "committee.csv"))) {
    if (-not (Test-Path $CommitteeTool)) { throw "missing $CommitteeTool" }
    New-Item -ItemType Directory -Force -Path $CommitteeDir | Out-Null
    & $CommitteeTool -n 3 -out $CommitteeDir
    if ($LASTEXITCODE -ne 0) { throw "gossip committee keygen failed" }
  }
  $x25519 = (Get-Content (Join-Path $CommitteeDir "node0.priv") -Raw).Trim()
  $committee = (Get-Content (Join-Path $CommitteeDir "committee.csv") -Raw).Trim()
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
"@ | Set-Content (Join-Path $cfgDir "config.json") -Encoding ascii

  Write-Host "starting avalanchego"
  $avagoArgs = @(
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
    "--http-allowed-hosts=localhost,127.0.0.1",
    "--track-subnets=$($meta.subnetID)"
  )
  Start-Process -FilePath $Avago -ArgumentList $avagoArgs -WindowStyle Hidden
  Wait-Http "http://127.0.0.1:9660/ext/health" 90
}

$meta = Get-Content $ChainFile -Raw | ConvertFrom-Json
$env:CHAIN_ID = $meta.chainID
$env:NODE_URL = "http://127.0.0.1:9660"
$env:ORDER_CHAIN_ID = $meta.chainID
$env:ORDER_NODE_URL = "http://127.0.0.1:9660"
$env:ORDER_ROUTER_RELAY_SECRET = "local-dev-secret"

$anvilUp = $false
try {
  $r = Invoke-RestMethod "http://127.0.0.1:8545" -Method Post -ContentType "application/json" -Body '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
  $anvilUp = $true
} catch {}
if (-not $anvilUp) {
  Write-Host "starting anvil"
  Start-Process -FilePath (Join-Path $Foundry "anvil.exe") -ArgumentList @("--host","127.0.0.1","--port","8545","--chain-id","31337") -WindowStyle Hidden
  Start-Sleep -Seconds 2
}

Write-Host "deploying companion rails (anvil 31337)"
$env:EVM_RPC_URL = "http://127.0.0.1:8545"
$env:FOUNDRY_PROFILE = "rails"
$env:Path = "$Foundry;$env:Path"
Set-Location $PSScriptRoot
node deploy-rails.mjs
if ($LASTEXITCODE -ne 0) { throw "deploy-rails failed" }
$rails = Get-Content (Join-Path $PSScriptRoot "companion-evm.addresses.json") -Raw | ConvertFrom-Json
$env:ORDER_GATEWAY = $rails.orderIntentGateway
$env:LIQUIDITY_GATEWAY = $rails.liquidityIntentGateway
$env:EVM_RELAY_EXECUTOR_PRIVATE_KEY = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

$env:ORDER_MARKETS_PATH = Join-Path $Local "native-markets.json"
$routerOk = $false
try {
  $h = Invoke-RestMethod "http://127.0.0.1:9098/health"
  $routerOk = [bool]$h.ok
  try { Invoke-RestMethod "http://127.0.0.1:9098/markets" | Out-Null } catch { $routerOk = $false }
} catch {}
if (-not $routerOk) {
  Write-Host "starting order-router"
  Get-Process veilvm-order-router -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
  Start-Sleep -Seconds 1
  Start-Process -FilePath $Router -WorkingDirectory $Root -WindowStyle Hidden
  Wait-Http "http://127.0.0.1:9098/health" 20
}
try {
  $listed = Invoke-RestMethod "http://127.0.0.1:9098/markets"
  $n = @($listed.markets).Count
} catch { $n = 0 }
if ($n -lt 1) {
  Write-Host "seeding native market"
  $headers = @{ "content-type" = "application/json"; "x-relay-secret" = $env:ORDER_ROUTER_RELAY_SECRET }
  Invoke-RestMethod -Uri "http://127.0.0.1:9098/native/create-market" -Method Post -Headers $headers -Body '{"question":"VEIL local native market","outcomes":2,"creatorBond":1}' | Out-Null
}
$relayUp = $false
try {
  Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object { $_.CommandLine -match "relay-opaque-intents" } | Out-Null
} catch {}
Write-Host "starting opaque relayer (watch)"
Start-Process -FilePath (Join-Path $NodeDir "node.exe") -WorkingDirectory $PSScriptRoot -ArgumentList @("relay-opaque-intents.mjs","--watch") -WindowStyle Hidden
Write-Host "stack up: veilvm :9660  anvil :8545  router :9098  rails persisted  relayer watch"
Write-Host "native UX: POST /orders  GET /markets  (frontend VEIL_ORDER_API_BASE=http://127.0.0.1:9098)"
if ($SkipTests) { return }

Write-Host "`n=== native AMM smoke ==="
& (Join-Path $Local "veilvm-smoke.exe")
if ($LASTEXITCODE -ne 0) { throw "veilvm-smoke failed" }

Write-Host "`n=== companion + relayer e2e ==="
Set-Location $PSScriptRoot
$env:ORDER_ROUTER_URL = "http://127.0.0.1:9098"
$env:EVM_RPC_URL = "http://127.0.0.1:8545"
node local-stack-e2e.mjs
if ($LASTEXITCODE -ne 0) { throw "local-stack-e2e failed" }

Write-Host "`nALL LOCAL STACK TESTS PASSED"
