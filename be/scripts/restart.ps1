param(
    [switch]$Background,
    [int]$Port = 8080
)

$ErrorActionPreference = "Stop"
$BeRoot = Split-Path -Parent $PSScriptRoot
Set-Location $BeRoot

function Stop-PortListener {
    param([int]$ListenPort)
    $lines = netstat -ano | Select-String ":$ListenPort\s+.*LISTENING"
    foreach ($line in $lines) {
        $processId = ($line -split '\s+')[-1]
        if ($processId -match '^\d+$' -and [int]$processId -gt 0) {
            Stop-Process -Id ([int]$processId) -Force -ErrorAction SilentlyContinue
        }
    }
}

Write-Host "Stopping process on port $Port (if any)..."
Stop-PortListener -ListenPort $Port
Start-Sleep -Seconds 1

New-Item -ItemType Directory -Force -Path "$BeRoot\bin" | Out-Null
Write-Host "Building be/bin/api.exe ..."
go build -o "$BeRoot\bin\api.exe" ./cmd/api
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

$exe = Join-Path $BeRoot "bin\api.exe"
Write-Host "Built $exe"

if ($Background) {
    New-Item -ItemType Directory -Force -Path "$BeRoot\tmp" | Out-Null
    $log = Join-Path $BeRoot "tmp\api.log"
    $errLog = Join-Path $BeRoot "tmp\api.err.log"
    Start-Process -FilePath $exe -WorkingDirectory $BeRoot `
        -RedirectStandardOutput $log -RedirectStandardError $errLog -NoNewWindow
    Write-Host "Started in background (logs: tmp\api.log)"
} else {
    & $exe
}
