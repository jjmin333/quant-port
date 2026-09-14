# Dev FE 전환 — Docker quant-port-fe 를 내리고 호스트 npm run dev 로 nginx 를 전환한다.
#
# nginx 는 app_routes.host-fe.conf (host:3000) 일 때 dev 서버를 본다.
# deploy.ps1 (prod Docker FE) 의 반대 동작.
#
#   .\fe\scripts\deploy-dev.ps1
#   npm run deploy:dev          # fe/ 에서
#
param(
    [switch]$SkipNginx,
    [switch]$Foreground,
    [int]$Port = 3000,
    [int]$WaitSeconds = 45
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

function Start-FeDevServer {
    param(
        [int]$ListenPort,
        [switch]$RunForeground
    )

    Stop-HostPortListener -ListenPort $ListenPort
    Start-Sleep -Seconds 1

    if ($RunForeground) {
        Write-Host "Starting Next.js dev server on 0.0.0.0:$ListenPort (foreground)..." -ForegroundColor Cyan
        Set-Location $FeRoot
        $env:PORT = "$ListenPort"
        # `-p` 는 PowerShell이 자체 스위치로 먹어 next 가 `fe\3000` 디렉터리를 찾는다 — PORT 만 쓴다.
        npm run dev
        return
    }

    $logDir = Join-Path $ProjectRoot "deploy\logs"
    New-Item -ItemType Directory -Force -Path $logDir | Out-Null
    $logFile = Join-Path $logDir "fe-dev.log"

    $cmd = @"
Set-Location '$FeRoot'
`$env:PORT = '$ListenPort'
. '$ProjectRoot\deploy\scripts\Utf8Log.ps1'
Start-Utf8Log -Command 'npm' -Arguments @('run','dev') -LogFile '$logFile'
"@
    Start-Process powershell.exe -ArgumentList @("-NoProfile", "-WindowStyle", "Hidden", "-Command", $cmd) | Out-Null

    Write-Host "Waiting for FE dev (max ${WaitSeconds}s)..."
    $deadline = (Get-Date).AddSeconds($WaitSeconds)
    while ((Get-Date) -lt $deadline) {
        try {
            $res = Invoke-WebRequest -Uri "http://127.0.0.1:$ListenPort/" -UseBasicParsing -TimeoutSec 3
            if ($res.StatusCode -lt 500) {
                Write-Host "FE dev: OK (http://127.0.0.1:$ListenPort)" -ForegroundColor Green
                Write-Host "Log: $logFile"
                return
            }
        } catch {
            Start-Sleep -Seconds 2
        }
    }
    Write-Warning "FE dev did not become ready in ${WaitSeconds}s. Log: $logFile"
}

$envFile = Join-Path $ProjectRoot ".env"
$nginxRoutes = Read-EnvValue -FilePath $envFile -Key "NGINX_ROUTES"
if ($nginxRoutes -eq "app_routes.host-fe.conf") {
    Write-Host "NGINX_ROUTES=app_routes.host-fe.conf — already in dev routing mode." -ForegroundColor DarkGray
} elseif ($nginxRoutes -eq "app_routes.conf") {
    Write-Host "Switching NGINX_ROUTES: app_routes.conf -> app_routes.host-fe.conf"
} else {
    Write-Host "Setting NGINX_ROUTES=app_routes.host-fe.conf (was: $(if ($nginxRoutes) { $nginxRoutes } else { '(unset)' }))"
}

Set-EnvLine -FilePath $envFile -Key "NGINX_ROUTES" -Value "app_routes.host-fe.conf"

Write-Host "Stopping Docker FE (quant-port-fe)..."
$prevEap = $ErrorActionPreference
$ErrorActionPreference = "Continue"
docker compose --profile prod stop fe 2>&1 | Out-Null
docker compose --profile prod rm -f fe 2>&1 | Out-Null
$ErrorActionPreference = $prevEap

if (-not $SkipNginx) {
    Write-Host "Recreating nginx (80/443 -> host:$Port)..."
    $ErrorActionPreference = "Continue"
    docker compose --profile prod up -d --no-deps --force-recreate nginx 2>&1 | Out-Null
    $ErrorActionPreference = $prevEap
    if ($LASTEXITCODE -ne 0) {
        docker compose --profile prod up -d --no-deps --force-recreate nginx
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
}

Start-FeDevServer -ListenPort $Port -RunForeground:$Foreground

if (-not $Foreground) {
    Write-Host ""
    Write-Host "=== Dev FE ready ===" -ForegroundColor Cyan
    Write-Host "  Direct:  http://127.0.0.1:$Port"
    if (-not $SkipNginx) {
        Write-Host "  nginx:   routes to host:$Port (NGINX_ROUTES=app_routes.host-fe.conf)"
    }
    Write-Host "  Prod FE: stopped (quant-port-fe)"
    Write-Host ""
    Write-Host "Back to prod: npm run deploy"
}
