# Prod FE 배포 — Docker fe 이미지를 재빌드하고 quant-port-fe 를 교체한다.
#
# next build 결과가 이미지에 박히므로 restart 만으로는 코드가 반영되지 않는다.
# nginx 는 app_routes.conf (fe:3000) 일 때 이 컨테이너를 본다.
#
#   .\fe\scripts\deploy.ps1
#   npm run deploy          # fe/ 에서
#
param(
    [switch]$NoCache,
    [switch]$SkipHostFeStop,
    [int]$WaitSeconds = 180
)

$ErrorActionPreference = "Stop"
$FeRoot = Split-Path -Parent $PSScriptRoot
$ProjectRoot = Split-Path -Parent $FeRoot
Set-Location $ProjectRoot

function Stop-HostPortListener {
    param([int]$ListenPort)
    $lines = netstat -ano | Select-String ":$ListenPort\s+.*LISTENING"
    foreach ($line in $lines) {
        $processId = ($line -split '\s+')[-1]
        if ($processId -match '^\d+$' -and [int]$processId -gt 0) {
            $proc = Get-Process -Id ([int]$processId) -ErrorAction SilentlyContinue
            if ($proc -and $proc.Path -notlike "*Docker*") {
                Write-Host "Stopping host process on port $ListenPort (PID $processId, $($proc.ProcessName))..."
                Stop-Process -Id ([int]$processId) -Force -ErrorAction SilentlyContinue
            }
        }
    }
}

function Read-EnvValue {
    param([string]$FilePath, [string]$Key)
    if (-not (Test-Path $FilePath)) { return $null }
    foreach ($line in Get-Content -Path $FilePath -Encoding utf8) {
        if ($line -like "$Key=*") {
            return $line.Substring($Key.Length + 1).Trim()
        }
    }
    return $null
}

function Set-EnvLine {
    param(
        [string]$FilePath,
        [string]$Key,
        [string]$Value
    )
    $lines = if (Test-Path $FilePath) { Get-Content -Path $FilePath -Encoding utf8 } else { @() }
    $prefix = "$Key="
    $updated = $false
    $out = foreach ($line in $lines) {
        if ($line -like "$prefix*") {
            $updated = $true
            "$prefix$Value"
        } else {
            $line
        }
    }
    if (-not $updated) {
        $out += "$prefix$Value"
    }
    Set-Content -Path $FilePath -Value $out -Encoding utf8
}

$envFile = Join-Path $ProjectRoot ".env"
$nginxRoutes = Read-EnvValue -FilePath $envFile -Key "NGINX_ROUTES"
if ($nginxRoutes -eq "app_routes.conf") {
    Write-Host "NGINX_ROUTES=app_routes.conf — already in prod routing mode." -ForegroundColor DarkGray
} elseif ($nginxRoutes -eq "app_routes.host-fe.conf") {
    Write-Host "Switching NGINX_ROUTES: app_routes.host-fe.conf -> app_routes.conf"
} else {
    Write-Host "Setting NGINX_ROUTES=app_routes.conf (was: $(if ($nginxRoutes) { $nginxRoutes } else { '(unset)' }))"
}
Set-EnvLine -FilePath $envFile -Key "NGINX_ROUTES" -Value "app_routes.conf"

if (-not $SkipHostFeStop) {
    Write-Host "Stopping host FE on :3000 (if any)..."
    Stop-HostPortListener -ListenPort 3000
    Start-Sleep -Seconds 1
}

if ($NoCache) {
    Write-Host "Building FE image (--no-cache)..."
    docker compose --profile prod build --no-cache fe
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    Write-Host "Starting Docker FE (quant-port-fe)..."
    docker compose --profile prod up -d fe
} else {
    Write-Host "Building & starting Docker FE (quant-port-fe)..."
    docker compose --profile prod up -d --build fe
}
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "Waiting for FE health (max ${WaitSeconds}s)..."
$deadline = (Get-Date).AddSeconds($WaitSeconds)
$healthy = $false
while ((Get-Date) -lt $deadline) {
    $status = docker inspect --format='{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' quant-port-fe 2>$null
    if ($status -eq "healthy") {
        $healthy = $true
        break
    }
    if ($status -eq "unhealthy") {
        break
    }
    Start-Sleep -Seconds 3
}

if (-not $healthy) {
    Write-Error "FE health check failed or timed out. Logs: docker logs quant-port-fe"
}

Write-Host "FE ready: Docker container quant-port-fe (healthy)" -ForegroundColor Green

Write-Host "Recreating nginx (80/443 -> quant-port-fe)..."
$prevEap = $ErrorActionPreference
$ErrorActionPreference = "Continue"
docker compose --profile prod up -d --no-deps --force-recreate nginx 2>&1 | Out-Null
$ErrorActionPreference = $prevEap
if ($LASTEXITCODE -ne 0) {
    docker compose --profile prod up -d --no-deps --force-recreate nginx
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Write-Host "nginx routes to quant-port-fe (NGINX_ROUTES=app_routes.conf)"
Write-Host "Logs: docker logs -f quant-port-fe"
Write-Host "Back to dev: npm run deploy:dev"
