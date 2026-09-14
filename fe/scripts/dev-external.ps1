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
# `-p` 는 npm/PowerShell이 가로채 `next dev -H 0.0.0.0 3000` 이 되어
# `fe\3000` 디렉터리를 찾는 오류가 난다. PORT 환경변수만 쓴다 (deploy-dev.ps1 과 동일).
npm run dev
