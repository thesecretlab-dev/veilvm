# Ensure the local VEIL stack. Idempotent: start only what is down.
# Does not kill healthy processes. Does not rebuild. Does not run tests.
# Owner: Task Scheduler VEIL-local-genesis-node (ONLOGON). Not the agent job.
# Logs: veilvm\.local\logs\daemon-boot.log
$ErrorActionPreference = "Stop"
$Root = Split-Path $PSScriptRoot -Parent
$Local = Join-Path $Root ".local"
$PluginDir = Join-Path $Local "plugins"
$Data = Join-Path $Local "nodedata"
$Logs = Join-Path $Local "logs"
$Pids = Join-Path $Local "pids"
$Avago = Join-Path $Local "avalanchego.exe"
$Router = Join-Path $Local "veilvm-order-router.exe"
$Vk = Join-Path $Root "zk-fixture-new\groth16_shielded_ledger_vk.bin"
$VmId = "u9GgvekeunSwK4TPF4jj7xLsW1LKkd1Uv9VQZo2SGfrwkejsK"
$ChainFile = Join-Path $Local "local-chain.json"
$Foundry = "C:\Users\Justin\tools\foundry"
$NodeDir = "C:\Program Files\nodejs"
$Anvil = Join-Path $Foundry "anvil.exe"
$NodeExe = Join-Path $NodeDir "node.exe"
$Frontend = "C:\Users\Justin\src\veil\veil-frontend"
$NextCmd = Join-Path $Frontend "node_modules\.bin\next.cmd"
$env:Path = "$Foundry;$NodeDir;$env:Path"

New-Item -ItemType Directory -Force -Path $Logs, $PluginDir, $Data, $Pids | Out-Null
$bootLog = Join-Path $Logs "daemon-boot.log"

function Log($msg) {
  $line = "{0} {1}" -f (Get-Date -Format "o"), $msg
  Add-Content -Path $bootLog -Value $line
  Write-Host $line
}

$mutex = New-Object System.Threading.Mutex($false, "Global\VEILLocalGenesisDaemon")
if (-not $mutex.WaitOne(0)) {
  Log "another ensure is running; skip"
  return
}

function Pid-Alive($name) {
  $f = Join-Path $Pids "$name.pid"
  if (-not (Test-Path $f)) { return $false }
  $id = 0
  if (-not [int]::TryParse(((Get-Content $f -Raw).Trim()), [ref]$id)) { return $false }
  return [bool](Get-Process -Id $id -ErrorAction SilentlyContinue)
}

function Write-Pid($name, $id) {
  Set-Content -Path (Join-Path $Pids "$name.pid") -Value $id -Encoding ascii
}

function Start-Owned($file, $argList, $name, $workDir) {
  $procArgs = @{
    FilePath         = $file
    WorkingDirectory = $workDir
    WindowStyle      = "Hidden"
    PassThru         = $true
  }
  if ($argList -and @($argList).Count -gt 0) { $procArgs.ArgumentList = $argList }
  $proc = Start-Process @procArgs
  Write-Pid $name $proc.Id
  Log "started $name pid=$($proc.Id)"
}

function Adopt-Node($name, $pattern) {
  try {
    $hit = Get-CimInstance Win32_Process -Filter "Name = 'node.exe'" -ErrorAction SilentlyContinue |
      Where-Object { $_.CommandLine -and $_.CommandLine -match $pattern } |
      Select-Object -First 1
    if ($hit) {
      Write-Pid $name $hit.ProcessId
      return $true
    }
  } catch {}
  return $false
}

function Log-Fresh($path, $maxAgeSec) {
  if (-not $path -or -not (Test-Path $path)) { return $false }
  return ((Get-Date) - (Get-Item $path).LastWriteTime).TotalSeconds -lt $maxAgeSec
}

function Wait-Rest($url, $seconds, $ok) {
  $deadline = (Get-Date).AddSeconds($seconds)
  while ((Get-Date) -lt $deadline) {
    try {
      $r = Invoke-RestMethod -Uri $url -TimeoutSec 8
      if ($ok.Invoke($r)) { return $r }
    } catch {}
    Start-Sleep -Seconds 1
  }
  throw "timeout waiting for $url"
}

function Http-Ok($url) {
  try {
    $r = Invoke-WebRequest -UseBasicParsing -Uri $url -TimeoutSec 8
    return ($r.StatusCode -ge 200 -and $r.StatusCode -lt 400)
  } catch { return $false }
}

function Rails-Live($addrFile) {
  if (-not (Test-Path $addrFile)) { return $false }
  try {
    $doc = Get-Content $addrFile -Raw | ConvertFrom-Json
    if (-not $doc.wveil -or -not $doc.zeroidRegistry -or -not $doc.faucet) { return $false }
    foreach ($addr in @($doc.wveil, $doc.zeroidRegistry, $doc.faucet)) {
      $body = '{"jsonrpc":"2.0","id":1,"method":"eth_getCode","params":["' + $addr + '","latest"]}'
      $r = Invoke-RestMethod "http://127.0.0.1:8545" -Method Post -ContentType "application/json" -Body $body -TimeoutSec 5
      if (-not $r.result -or $r.result -eq "0x" -or $r.result -eq "0x0") { return $false }
    }
    return $true
  } catch { return $false }
}

function Ensure-Worker($name, $pattern, $logFile, $file, $argList, $workDir) {
  if (Pid-Alive $name) { Log "$name already running"; return }
  if (Log-Fresh $logFile 45) { Log "$name log is fresh; skip"; return }
  if ($pattern -and (Adopt-Node $name $pattern)) { Log "$name already running (adopted)"; return }
  Log "starting $name"
  Start-Owned $file $argList $name $workDir
}

try {
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

  $env:VEIL_LOCAL_FEE_TOPUP = "1"

  $healthy = $false
  try { $healthy = [bool](Invoke-RestMethod "http://127.0.0.1:9660/ext/health" -TimeoutSec 5).healthy } catch {}
  $avagoProc = Get-Process avalanchego -ErrorAction SilentlyContinue
  if ($healthy) {
    if ($avagoProc) { Write-Pid "avalanchego" $avagoProc.Id }
    Log "avalanchego already healthy"
  } elseif ($avagoProc) {
    Log "avalanchego pid=$($avagoProc.Id) not healthy yet; waiting (will not kill)"
    Wait-Rest "http://127.0.0.1:9660/ext/health" 90 { param($r) [bool]$r.healthy }
    Write-Pid "avalanchego" $avagoProc.Id
    Log "avalanchego healthy"
  } else {
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
    Start-Owned $Avago $avagoArgs "avalanchego" $Root
    Wait-Rest "http://127.0.0.1:9660/ext/health" 90 { param($r) [bool]$r.healthy }
    Log "avalanchego healthy"
  }

  $anvilUp = $false
  try {
    Invoke-RestMethod "http://127.0.0.1:8545" -Method Post -ContentType "application/json" -Body '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}' -TimeoutSec 3 | Out-Null
    $anvilUp = $true
  } catch {}
  if ($anvilUp) {
    $ap = Get-Process anvil -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($ap) { Write-Pid "anvil" $ap.Id }
    Log "anvil already up"
  } else {
    Log "starting anvil 31337"
    Start-Owned $Anvil @("--host", "127.0.0.1", "--port", "8545", "--chain-id", "31337") "anvil" $Foundry
    Start-Sleep -Seconds 2
  }

  $addrFile = Join-Path $PSScriptRoot "companion-evm.addresses.json"
  $env:EVM_RPC_URL = "http://127.0.0.1:8545"
  $env:FOUNDRY_PROFILE = "rails"
  if (Rails-Live $addrFile) {
    Log "companion rails already live"
  } else {
    Log "deploy-rails"
    Set-Location $PSScriptRoot
    & $NodeExe deploy-rails.mjs
    if ($LASTEXITCODE -ne 0) { throw "deploy-rails failed" }
  }
  $rails = Get-Content $addrFile -Raw | ConvertFrom-Json

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
  try { $routerOk = [bool](Invoke-RestMethod "http://127.0.0.1:9098/health" -TimeoutSec 5).ok } catch {}
  $routerProc = Get-Process veilvm-order-router -ErrorAction SilentlyContinue
  if ($routerOk) {
    if ($routerProc) { Write-Pid "order-router" $routerProc.Id }
    Log "order-router already up"
  } elseif ($routerProc) {
    Log "order-router pid=$($routerProc.Id) not ready; waiting (will not kill)"
    Wait-Rest "http://127.0.0.1:9098/health" 30 { param($r) [bool]$r.ok }
    Write-Pid "order-router" $routerProc.Id
    Log "order-router up"
  } else {
    Log "starting order-router"
    Start-Owned $Router @() "order-router" $Root
    Wait-Rest "http://127.0.0.1:9098/health" 20 { param($r) [bool]$r.ok }
  }

  try { $n = @((Invoke-RestMethod "http://127.0.0.1:9098/markets" -TimeoutSec 5).markets).Count } catch { $n = 0 }
  if ($n -lt 1) {
    Log "seeding native market"
    $headers = @{ "content-type" = "application/json"; "x-relay-secret" = "local-dev-secret" }
    Invoke-RestMethod -Uri "http://127.0.0.1:9098/native/create-market" -Method Post -Headers $headers -Body '{"question":"VEIL local native market","outcomes":2,"creatorBond":1}' | Out-Null
  }

  Ensure-Worker "relayer" "relay-opaque-intents" $null $NodeExe @("relay-opaque-intents.mjs", "--watch") $PSScriptRoot

  $env:VEIL_ORDER_API_BASE = "http://127.0.0.1:9098"
  $env:NODE_ENV = "development"
  if (Http-Ok "http://127.0.0.1:3000") {
    Log "frontend already up"
  } else {
    if (-not (Test-Path $NextCmd)) { throw "missing $NextCmd" }
    Log "starting frontend :3000"
    Start-Owned $NextCmd @("dev", "--webpack", "-p", "3000") "frontend" $Frontend
    $deadline = (Get-Date).AddSeconds(90)
    while ((Get-Date) -lt $deadline) {
      if (Http-Ok "http://127.0.0.1:3000") { break }
      Start-Sleep -Seconds 1
    }
    if (-not (Http-Ok "http://127.0.0.1:3000")) { throw "timeout waiting for http://127.0.0.1:3000" }
  }

  if (Http-Ok "http://127.0.0.1:8787/health") {
    Log "mesh already up"
  } else {
    Log "starting mesh :8787"
    Start-Owned $NodeExe @("mesh/server.mjs") "mesh" $Frontend
    $deadline = (Get-Date).AddSeconds(20)
    while ((Get-Date) -lt $deadline) {
      if (Http-Ok "http://127.0.0.1:8787/health") { break }
      Start-Sleep -Seconds 1
    }
    if (-not (Http-Ok "http://127.0.0.1:8787/health")) { Log "mesh did not answer /health yet; left running" }
  }

  $env:ORDER_ROUTER_URL = "http://127.0.0.1:9098"
  $env:ORDER_ROUTER_RELAY_SECRET = "local-dev-secret"
  $env:CAST_BIN = Join-Path $Foundry "cast.exe"
  $env:EVM_RELAY_EXECUTOR_PRIVATE_KEY = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
  Ensure-Worker "live-activity" "live-activity.mjs" (Join-Path $Logs "live-activity.log") $NodeExe @("live-activity.mjs") $PSScriptRoot

  Log "ensure done 9660/8545/9098/3000/8787 chain=$($meta.chainID)"
} finally {
  try { $mutex.ReleaseMutex() } catch {}
  $mutex.Dispose()
}
