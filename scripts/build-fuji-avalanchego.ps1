# Build Fuji-capable avalanchego 1.15 (Windows AvailableBytes stub).
# Does NOT overwrite the local 1.13 genesis node at .local\avalanchego.exe.
$ErrorActionPreference = "Stop"
$Root = Split-Path $PSScriptRoot -Parent
$OutDir = Join-Path $Root ".local\fuji"
$Src = "C:\Users\Justin\src\avalanchego-v1.15.0-fuji"
$Mingw = "C:\Users\Justin\tools\mingw64\bin"
$Out = Join-Path $OutDir "avalanchego.exe"
if (-not (Test-Path "$Mingw\gcc.exe")) { throw "missing gcc" }
if (-not (Test-Path (Join-Path $Src "main\main.go"))) { throw "missing $Src\main\main.go" }

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$env:Path = "$Mingw;$env:Path"
$env:CGO_ENABLED = "1"
$env:CC = "gcc"
$env:CGO_CFLAGS = "-O2 -D__BLST_PORTABLE__"
$env:GOWORK = "off"

Set-Location $Src
Write-Host "building avalanchego 1.15 -> $Out"
go build -o $Out ./main
if ($LASTEXITCODE -ne 0) { throw "go build avalanchego failed" }
& $Out --version
Write-Host "wrote $Out"
