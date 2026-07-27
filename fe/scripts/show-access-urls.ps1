# Quant Port FE — 외부 접속 URL 출력
$port = if ($env:PORT) { [int]$env:PORT } else { 3000 }

$ips = Get-NetIPAddress -AddressFamily IPv4 |
  Where-Object {
    $_.IPAddress -notlike "127.*" -and
    $_.PrefixOrigin -ne "WellKnown"
  } |
  Select-Object -ExpandProperty IPAddress -Unique

Write-Host ""
Write-Host "Quant Port FE dev access URLs (port $port)" -ForegroundColor Cyan
Write-Host "  Local:   http://localhost:$port"
foreach ($ip in $ips) {
  Write-Host "  Network: http://${ip}:$port" -ForegroundColor Green
}
Write-Host ""
Write-Host "API는 Next.js rewrite로 같은 origin에서 프록시됩니다 (/api -> BE :8080)." -ForegroundColor DarkGray
Write-Host "외부(인터넷) 접속 시 공유기에서 TCP $port 포트 포워딩이 필요할 수 있습니다." -ForegroundColor DarkGray
Write-Host ""
Write-Host ".env.local — ALLOWED_DEV_ORIGINS (host:port 필수, Next.js 16):" -ForegroundColor Yellow
$originList = @("localhost:$port", "127.0.0.1:$port") + ($ips | ForEach-Object { "${_}:$port" })
Write-Host "  ALLOWED_DEV_ORIGINS=$($originList -join ',')" -ForegroundColor White
Write-Host "  (변경 후 dev 서버 재시작 필요)" -ForegroundColor DarkGray
Write-Host ""
