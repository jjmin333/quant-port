param(
    [int]$Port = 8080
)

$lines = netstat -ano | Select-String ":$Port\s+.*LISTENING"
$stopped = $false
foreach ($line in $lines) {
    $processId = ($line -split '\s+')[-1]
    if ($processId -match '^\d+$' -and [int]$processId -gt 0) {
        Stop-Process -Id ([int]$processId) -Force -ErrorAction SilentlyContinue
        Write-Host "Stopped PID $processId on port $Port"
        $stopped = $true
    }
}
if (-not $stopped) {
    Write-Host "No listener on port $Port"
}
