param(
    [switch]$Background,
    [int]$Port = 8080,
    [int]$WaitSeconds = 25
)

$ErrorActionPreference = "Stop"
$BeRoot = Split-Path -Parent $PSScriptRoot
$ProjectRoot = Split-Path -Parent $BeRoot
Set-Location $ProjectRoot

function Stop-HostPortListener {
    param([int]$ListenPort)
    $lines = netstat -ano | Select-String ":$ListenPort\s+.*LISTENING"
    foreach ($line in $lines) {
        $processId = ($line -split '\s+')[-1]
        if ($processId -match '^\d+$' -and [int]$processId -gt 0) {
            $proc = Get-Process -Id ([int]$processId) -ErrorAction SilentlyContinue
            if ($proc -and $proc.Path -notlike "*Docker*") {
                Write-Host "Stopping host process on port $ListenPort (PID $processId)..."
                Stop-Process -Id ([int]$processId) -Force -ErrorAction SilentlyContinue
            }
        }
    }
}

if (-not (Test-Path "$BeRoot\.env")) {
    Write-Error "be/.env 가 없습니다. cp be/.env.example be/.env 후 JWT_SECRET 등을 설정하세요."
}

Write-Host "Stopping legacy host API on port $Port (if any)..."
Stop-HostPortListener -ListenPort $Port
Start-Sleep -Seconds 1

Write-Host "Building & starting Docker stack (redis + api + tesseract + batch workers)..."
docker compose up -d --build redis api worker-close-domestic worker-close-overseas worker-scores worker-index worker-fetch worker-research worker-earnings worker-filings
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

Write-Host "Waiting for API health (max ${WaitSeconds}s)..."
$deadline = (Get-Date).AddSeconds($WaitSeconds)
$healthy = $false
while ((Get-Date) -lt $deadline) {
    try {
        $res = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/health" -UseBasicParsing -TimeoutSec 3
        if ($res.StatusCode -eq 200) {
            $healthy = $true
            break
        }
    } catch {
        Start-Sleep -Seconds 2
    }
}

if (-not $healthy) {
    Write-Error "API health check failed. Logs: docker logs quant-port-api"
}

$prevEap = $ErrorActionPreference
$ErrorActionPreference = "Continue"
$logs = docker logs quant-port-api 2>&1 | Out-String
$ErrorActionPreference = $prevEap
if ($logs -match "import: tesseract ready") {
    Write-Host "Tesseract: OK (import: tesseract ready)"
} elseif ($logs -match "import: tesseract not found") {
    Write-Error @"
Tesseract NOT available inside container.
Check: docker exec quant-port-api tesseract --version
Rebuild: docker compose build --no-cache api
Logs: docker logs quant-port-api
"@
} else {
    Write-Warning "Tesseract startup log not found yet — check: docker logs quant-port-api"
}

Write-Host "API ready: http://127.0.0.1:$Port/health"
Write-Host "Logs: docker logs -f quant-port-api"

if (-not $Background) {
    docker logs -f quant-port-api
}
