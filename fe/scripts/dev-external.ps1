# 외부 접속 가능한 FE dev 서버 시작
param(
  [int]$Port = 3000,
  [switch]$OpenFirewall
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

if ($OpenFirewall) {
  Write-Host "Opening Windows Firewall for port $Port (requires admin)..." -ForegroundColor Yellow
  & "$PSScriptRoot\open-firewall.ps1" -Port $Port
}

$env:PORT = "$Port"
& "$PSScriptRoot\show-access-urls.ps1"

Write-Host "Starting Next.js dev server on 0.0.0.0:$Port ..." -ForegroundColor Cyan
npm run dev -- -p $Port
