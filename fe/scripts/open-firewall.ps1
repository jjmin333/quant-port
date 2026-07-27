# Windows 방화벽 — FE dev 포트 허용 (관리자 권한 필요)
param(
  [int]$Port = 3000
)

$ruleName = "Quant Port FE Dev ($Port)"

$existing = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
if ($existing) {
  Write-Host "Firewall rule already exists: $ruleName"
  exit 0
}

New-NetFirewallRule `
  -DisplayName $ruleName `
  -Direction Inbound `
  -Action Allow `
  -Protocol TCP `
  -LocalPort $Port `
  -Profile Any | Out-Null

Write-Host "Added firewall rule: $ruleName (TCP $Port inbound)" -ForegroundColor Green
