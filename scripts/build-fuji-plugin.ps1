# Compile VeilVM plugin against avalanchego-win-storage (rpc 46).
# Does NOT overwrite the local 1.13 plugin.
$ErrorActionPreference = "Stop"
$Root = Split-Path $PSScriptRoot -Parent
$Avago = "C:\Users\Justin\src\avalanchego-v1.15.0-fuji"
$Mingw = "C:\Users\Justin\tools\mingw64\bin"
$OutDir = Join-Path $Root ".local\fuji\plugins"
$VmId = "u9GgvekeunSwK4TPF4jj7xLsW1LKkd1Uv9VQZo2SGfrwkejsK"
$Out = Join-Path $OutDir "$VmId.exe"
$Mod = Join-Path $Root "go.mod"
$Sum = Join-Path $Root "go.sum"
$Bak = Join-Path $Root "go.mod.bak-fuji-plugin"
$BakSum = Join-Path $Root "go.sum.bak-fuji-plugin"

if (-not (Test-Path "$Mingw\gcc.exe")) { throw "missing gcc" }
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$env:Path = "$Mingw;$env:Path"
$env:CGO_ENABLED = "1"
$env:CC = "gcc"
$env:CGO_CFLAGS = "-O2 -D__BLST_PORTABLE__"
$env:GOWORK = "off"

Copy-Item $Mod $Bak -Force
Copy-Item $Sum $BakSum -Force
try {
  Add-Content -Path $Mod -Value "`nreplace github.com/ava-labs/avalanchego => C:/Users/Justin/src/avalanchego-v1.15.0-fuji`n"
  Set-Location $Root
  Write-Host "go mod tidy against avalanchego 1.15"
  go mod tidy
  Write-Host "building veilvm plugin -> $Out"
  go build -o $Out ./cmd/veilvm
  if ($LASTEXITCODE -ne 0) { throw "plugin build failed" }
  Write-Host "wrote $Out"
} finally {
  Move-Item $Bak $Mod -Force
  Move-Item $BakSum $Sum -Force
}
