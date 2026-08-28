# Durable local genesis stack. Run from Task Scheduler, not from the agent job.
# Does not rebuild. Does not run tests. Logs under veilvm\.local\logs
$ErrorActionPreference = "Stop"
$Root = Split-Path $PSScriptRoot -Parent
$Local = Join-Path $Root ".local"
$PluginDir = Join-Path $Local "plugins"
$Data = Join-Path $Local "nodedata"
$Logs = Join-Path $Local "logs"
$Avago = Join-Path $Local "avalanchego.exe"
$Router = Join-Path $Local "veilvm-order-router.exe"
$Vk = Join-Path $Root "zk-fixture-new\groth16_shielded_ledger_vk.bin"
$VmId = "u9GgvekeunSwK4TPF4jj7xLsW1LKkd1Uv9VQZo2SGfrwkejsK"
$ChainFile = Join-Path $Local "local-chain.json"
$Foundry = "C:\Users\Justin\tools\foundry"
$NodeDir = "C:\Program Files\nodejs"
$Anvil = Join-Path $Foundry "anvil.exe"
$NodeExe = Join-Path $NodeDir "node.exe"
$env:Path = "$Foundry;$NodeDir;$env:Path"

New-Item -ItemType Directory -Force -Path $Logs, $PluginDir, $Data | Out-Null
$bootLog = Join-Path $Logs "daemon-boot.log"
function Log($msg) {
  $line = "{0} {1}" -f (Get-Date -Format "o"), $msg
  Add-Content -Path $bootLog -Value $line
  Write-Host $line
}

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

function Start-Logged($file, $argList, $outName, $workDir) {
  $procArgs = @{
    FilePath = $file
    WorkingDirectory = $workDir
    WindowStyle = "Hidden"
  }
  if ($argList -and @($argList).Count -gt 0) { $procArgs.ArgumentList = $argList }
  Start-Process @procArgs
}

if (-not (Test-Path $Avago)) { throw "missing $Avago" }
if (-not (Test-Path $Router)) { throw "missing $Router" }
if (-not (Test-Path $ChainFile)) { throw "missing $ChainFile" }
if (-not (Test-Path $Vk)) { throw "missing $Vk" }
if (-not (Test-Path (Join-Path $PluginDir "$VmId.exe"))) { throw "missing veilvm plugin" }

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
  if (-not (Test-Path $CommitteeTool)) { throw "missing $CommitteeTool (build cmd/veilvm-gossip-committee)" }
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

$healthy = $false
try { $healthy = [bool](Invoke-RestMethod "http://127.0.0.1:9660/ext/health").healthy } catch {}
if (-not $healthy) {
  Get-Process avalanchego -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
  Get-Process | Where-Object { $_.ProcessName -like "u9Ggveke*" } | Stop-Process -Force -ErrorAction SilentlyContinue
  Start-Sleep -Seconds 2
  Remove-Item -Force -ErrorAction SilentlyContinue (Join-Path $PluginDir "$VmId.exe~")
  Log "starting avalanchego genesis node $($meta.chainID)"
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
  Start-Logged $Avago $avagoArgs "avalanchego.stdout.log" $Root
  Wait-Http "http://127.0.0.1:9660/ext/health" 90
  Log "avalanchego healthy"
} else {
  Log "avalanchego already healthy"
}

$anvilUp = $false
try {
  Invoke-RestMethod "http://127.0.0.1:8545" -Method Post -ContentType "application/json" -Body '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}' | Out-Null
  $anvilUp = $true
} catch {}
if (-not $anvilUp) {
  Get-Process anvil -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
  Log "starting anvil 31337"
  Start-Logged $Anvil @("--host","127.0.0.1","--port","8545","--chain-id","31337") "anvil.stdout.log" $Foundry
  Start-Sleep -Seconds 2
}

$env:EVM_RPC_URL = "http://127.0.0.1:8545"
$env:FOUNDRY_PROFILE = "rails"
Log "deploy-rails"
Set-Location $PSScriptRoot
& $NodeExe deploy-rails.mjs
if ($LASTEXITCODE -ne 0) { throw "deploy-rails failed" }
$rails = Get-Content (Join-Path $PSScriptRoot "companion-evm.addresses.json") -Raw | ConvertFrom-Json

$env:ORDER_CHAIN_ID = $meta.chainID
$env:ORDER_NODE_URL = "http://127.0.0.1:9660"
$env:ORDER_ROUTER_RELAY_SECRET = "local-dev-secret"
$env:ORDER_MARKETS_PATH = Join-Path $Local "native-markets.json"
$env:ORDER_GATEWAY = $rails.orderIntentGateway
$env:LIQUIDITY_GATEWAY = $rails.liquidityIntentGateway
$env:EVM_RELAY_EXECUTOR_PRIVATE_KEY = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
$env:ORDER_ROUTER_URL = "http://127.0.0.1:9098"
$env:EVM_RPC_URL = "http://127.0.0.1:8545"

$routerOk = $false
try { $routerOk = [bool](Invoke-RestMethod "http://127.0.0.1:9098/health").ok } catch {}
if (-not $routerOk) {
  Get-Process veilvm-order-router -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
  Start-Sleep -Seconds 1
  Log "starting order-router"
  Start-Logged $Router @() "order-router.stdout.log" $Root
  Wait-Http "http://127.0.0.1:9098/health" 20
}

try { $n = @((Invoke-RestMethod "http://127.0.0.1:9098/markets").markets).Count } catch { $n = 0 }
if ($n -lt 1) {
  Log "seeding native market"
  $headers = @{ "content-type" = "application/json"; "x-relay-secret" = "local-dev-secret" }
  Invoke-RestMethod -Uri "http://127.0.0.1:9098/native/create-market" -Method Post -Headers $headers -Body '{"question":"VEIL local native market","outcomes":2,"creatorBond":1}' | Out-Null
}

Get-CimInstance Win32_Process -ErrorAction SilentlyContinue |
  Where-Object { $_.CommandLine -match "relay-opaque-intents" } |
  ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
Log "starting relayer watch"
Start-Logged $NodeExe @("relay-opaque-intents.mjs","--watch") "relayer.stdout.log" $PSScriptRoot

$Frontend = "C:\Users\Justin\src\veil\veil-frontend"
$NextCmd = Join-Path $Frontend "node_modules\.bin\next.cmd"
$env:VEIL_ORDER_API_BASE = "http://127.0.0.1:9098"
$env:NODE_ENV = "development"
$uiUp = $false
try {
  $ui = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:3000" -TimeoutSec 2
  $uiUp = $ui.StatusCode -ge 200
} catch {}
if (-not $uiUp) {
  if (-not (Test-Path $NextCmd)) { throw "missing $NextCmd" }
  Log "starting frontend :3000"
  Start-Logged $NextCmd @("dev","--webpack","-p","3000") "frontend.stdout.log" $Frontend
  Wait-Http "http://127.0.0.1:3000" 90
}
Log "daemon up 9660/8545/9098/3000 chain=$($meta.chainID)"
