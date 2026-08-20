$ErrorActionPreference = "Stop"
$BeRoot = Split-Path -Parent $PSScriptRoot
$ProjectRoot = Split-Path -Parent $BeRoot
Set-Location $ProjectRoot

Write-Host "Stopping Docker API..."
docker compose stop api
Write-Host "Done. Redis still running (docker compose stop redis to stop all)."
