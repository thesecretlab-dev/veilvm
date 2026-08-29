# Rebuild platform-cli.exe against the local Windows AvailableBytes stub.
$ErrorActionPreference = "Stop"
$cli = "C:\Users\Justin\src\platform-cli"
$out = "C:\Users\Justin\tools\platform-cli.exe"
$stub = "C:\Users\Justin\src\avalanchego-win-storage\utils\storage\storage_windows.go"
$mingw = "C:\Users\Justin\tools\mingw64\bin"
if (-not (Test-Path $stub)) {
    throw "missing Windows stub: $stub"
}
if (-not (Test-Path "$mingw\gcc.exe")) {
    throw "missing gcc: $mingw\gcc.exe"
}
$env:Path = "$mingw;$env:Path"
$env:CGO_ENABLED = "1"
$env:CC = "gcc"
$env:GOWORK = "off"
Set-Location "C:\Users\Justin\src\avalanchego-win-storage"
go test ./utils/storage -count=1
Set-Location $cli
go build -o $out .
& $out version
Write-Output "wrote $out"
