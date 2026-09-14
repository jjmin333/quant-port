# 도메인 개발 — FE는 호스트 :3000 (npm run dev), nginx는 :80 · :443 → :3000
param(
    [string]$Domain = "quant-port.com",
    [switch]$SkipFirewall,
    [switch]$SkipBackend
)

$ErrorActionPreference = "Stop"
$BeRoot = Split-Path -Parent $PSScriptRoot
$ProjectRoot = Split-Path -Parent $BeRoot
Set-Location $ProjectRoot

function Ensure-TlsCerts {
    param([string]$CertDir, [string]$Cn)
    $fullchain = Join-Path $CertDir "fullchain.pem"
    $privkey = Join-Path $CertDir "privkey.pem"
    if ((Test-Path $fullchain) -and (Test-Path $privkey)) {
        Write-Host "TLS certs: OK ($CertDir)"
        return
    }
    New-Item -ItemType Directory -Force -Path $CertDir | Out-Null
    Write-Host "Generating self-signed TLS cert for $Cn (dev only)..."
    $san = "subjectAltName=DNS:$Cn,DNS:www.$Cn"
    docker run --rm `
        -v "${CertDir}:/certs" `
        alpine/openssl `
        req -x509 -nodes -days 365 -newkey rsa:2048 `
        -keyout /certs/privkey.pem `
        -out /certs/fullchain.pem `
        -subj "/CN=$Cn" `
        -addext $san
    if ($LASTEXITCODE -ne 0) {
        Write-Error "TLS cert generation failed. Install Docker and retry."
    }
}

function Stop-HostPortListener {
    param([int]$ListenPort)
    $lines = netstat -ano | Select-String ":$ListenPort\s+.*LISTENING"
    foreach ($line in $lines) {
        $processId = ($line -split '\s+')[-1]
        if ($processId -match '^\d+$' -and [int]$processId -gt 0) {
            $proc = Get-Process -Id ([int]$processId) -ErrorAction SilentlyContinue
            if ($proc -and $proc.Path -notlike "*Docker*") {
                Write-Host "Stopping process on port $ListenPort (PID $processId, $($proc.ProcessName))..."
                Stop-Process -Id ([int]$processId) -Force -ErrorAction SilentlyContinue
            }
        }
    }
}

function Open-FirewallPort {
    param([int]$Port)
    $ruleName = "Quant Port HTTP(S) ($Port)"
    $existing = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
    if ($existing) {
        Write-Host "Firewall: $ruleName already exists"
        return
    }
    New-NetFirewallRule `
        -DisplayName $ruleName `
        -Direction Inbound `
        -Action Allow `
        -Protocol TCP `
        -LocalPort $Port `
        -Profile Any | Out-Null
    Write-Host "Firewall: added $ruleName" -ForegroundColor Green
}

# Docker Desktop은 공용(Public) 네트워크에서 com.docker.backend.exe 인바운드를
# Block 한다. nginx 80/443 Allow 규칙만으로는 외부 기기 접속이 막힌다(로컬 IP
# 자체 접속은 되는 것처럼 보일 수 있음). Private 전환 + Block 해제가 필요하다.
function Enable-ExternalDockerAccess {
    $profile = Get-NetConnectionProfile |
        Where-Object { $_.InterfaceAlias -notlike "vEthernet*" -and $_.InterfaceAlias -notlike "*WSL*" } |
        Select-Object -First 1

    if ($profile -and $profile.NetworkCategory -eq "Public") {
        try {
            Set-NetConnectionProfile -InterfaceAlias $profile.InterfaceAlias -NetworkCategory Private
            Write-Host "Firewall: $($profile.InterfaceAlias) -> Private" -ForegroundColor Green
        } catch {
            Write-Warning "Could not set network to Private (admin required?): $($_.Exception.Message)"
        }
    }

    $dockerBlocks = Get-NetFirewallRule -DisplayName "Docker Desktop Backend" -ErrorAction SilentlyContinue |
        Where-Object { $_.Direction -eq "Inbound" -and $_.Action -eq "Block" -and $_.Enabled }
    if ($dockerBlocks) {
        try {
            $dockerBlocks | Disable-NetFirewallRule -ErrorAction Stop
            Write-Host "Firewall: disabled Docker Desktop Backend inbound Block ($($dockerBlocks.Count) rule(s))" -ForegroundColor Green
        } catch {
            Write-Warning "Could not disable Docker Desktop Backend block rules (admin required). External devices will fail to connect."
        }
    }
}

function Start-FeDevServer {
    param([int]$Port = 3000)

    Stop-HostPortListener -ListenPort $Port
    Start-Sleep -Seconds 1

    $feRoot = Join-Path $ProjectRoot "fe"
    $logDir = Join-Path $ProjectRoot "deploy\logs"
    New-Item -ItemType Directory -Force -Path $logDir | Out-Null
    $logFile = Join-Path $logDir "fe-dev.log"

    # 로그는 UTF-8 파이프를 거친다 — Tee-Object 를 그대로 쓰면 요청 로그의 한글
    # 사용자명이 깨진다(`deploy/scripts/Utf8Log.ps1` 머리글의 실측 참조).
    $cmd = @"
Set-Location '$feRoot'
`$env:PORT = '$Port'
. '$ProjectRoot\deploy\scripts\Utf8Log.ps1'
Start-Utf8Log -Command 'npm' -Arguments @('run','dev') -LogFile '$logFile'
"@
    Start-Process powershell.exe -ArgumentList @("-NoProfile", "-WindowStyle", "Hidden", "-Command", $cmd) | Out-Null

    $deadline = (Get-Date).AddSeconds(45)
    while ((Get-Date) -lt $deadline) {
        try {
            $res = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/" -UseBasicParsing -TimeoutSec 3
            if ($res.StatusCode -lt 500) {
                Write-Host "FE dev: OK (http://127.0.0.1:$Port)" -ForegroundColor Green
                return
            }
        } catch {
            Start-Sleep -Seconds 2
        }
    }
    Write-Warning "FE dev did not become ready in 45s. Log: $logFile"
}

function Test-EdgeHttp {
    param(
        [string]$Url,
        [int]$TimeoutSec = 5
    )
    try {
        $res = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec $TimeoutSec
        return $res.StatusCode -lt 500
    } catch {
        return $false
    }
}

function Show-ConnectivityReport {
    param([string]$DomainName)

    $publicIp = (Invoke-RestMethod -Uri "https://api.ipify.org" -TimeoutSec 5).Trim()
    Write-Host ""
    Write-Host "=== Connectivity check ===" -ForegroundColor Cyan
    Write-Host "  Public IP (this host): $publicIp"

    try {
        $dnsIp = (Resolve-DnsName $DomainName -Type A -ErrorAction Stop |
            Where-Object { $_.IPAddress } |
            Select-Object -First 1 -ExpandProperty IPAddress)
        Write-Host "  DNS A ($DomainName): $dnsIp"
        if ($dnsIp -ne $publicIp) {
            Write-Warning "DNS does not point to this host's public IP. Update the A record to $publicIp (port 80/443 will fail from the internet until fixed)."
        }
    } catch {
        Write-Warning "Could not resolve $DomainName"
    }

    $lanIp = Get-NetIPAddress -AddressFamily IPv4 |
        Where-Object { $_.IPAddress -notlike "127.*" -and $_.PrefixOrigin -ne "WellKnown" } |
        Select-Object -First 1 -ExpandProperty IPAddress

    $checks = @(
        @{ Label = "FE :3000 (local)"; Url = "http://127.0.0.1:3000/" },
        @{ Label = "nginx :80 (local)"; Url = "http://127.0.0.1/" },
        @{ Label = "nginx :80 (LAN)"; Url = "http://${lanIp}/" },
        @{ Label = "nginx :80 (public IP)"; Url = "http://${publicIp}/" },
        @{ Label = "nginx :80 (domain)"; Url = "http://${DomainName}/" }
    )

    foreach ($check in $checks) {
        $ok = Test-EdgeHttp -Url $check.Url
        $mark = if ($ok) { "OK" } else { "FAIL" }
        $color = if ($ok) { "Green" } else { "Yellow" }
        Write-Host "  [$mark] $($check.Label)" -ForegroundColor $color
    }

    Write-Host ""
    Write-Host "If outside access on :3000 works but :80 does not, forward TCP 80 and 443 on your router to this PC ($publicIp)." -ForegroundColor DarkGray
    Write-Host ""
}

function Merge-CsvEnvValue {
    param(
        [string]$Existing,
        [string[]]$Add
    )
    $set = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
    foreach ($item in ($Existing -split ",")) {
        $trimmed = $item.Trim()
        if ($trimmed) { [void]$set.Add($trimmed) }
    }
    foreach ($item in $Add) {
        $trimmed = $item.Trim()
        if ($trimmed) { [void]$set.Add($trimmed) }
    }
    return ($set | Sort-Object) -join ","
}

function Ensure-EnvLine {
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
            $existing = $line.Substring($prefix.Length)
            "$prefix$(Merge-CsvEnvValue -Existing $existing -Add ($Value -split ','))"
        } else {
            $line
        }
    }
    if (-not $updated) {
        $out += "$prefix$Value"
    }
    Set-Content -Path $FilePath -Value $out -Encoding utf8
}

function Ensure-DomainOrigins {
    param([string]$DomainName)
    $beEnv = Join-Path $BeRoot ".env"
    $feEnv = Join-Path $ProjectRoot "fe\.env.local"

    $corsOrigins = @(
        "https://$DomainName",
        "http://$DomainName",
        "https://www.$DomainName",
        "http://www.$DomainName",
        "http://${DomainName}:3000",
        "https://${DomainName}:3000"
    ) -join ","

    Ensure-EnvLine -FilePath $beEnv -Key "CORS_ALLOWED_ORIGINS" -Value $corsOrigins
    Write-Host "CORS: be/.env ← $DomainName origins" -ForegroundColor DarkGray

    $devOrigins = @(
        $DomainName,
        "${DomainName}:80",
        "${DomainName}:443",
        "www.$DomainName",
        "www.${DomainName}:80",
        "www.${DomainName}:443",
        "localhost:3000"
    ) -join ","

    if (-not (Test-Path $feEnv)) {
        Copy-Item (Join-Path $ProjectRoot "fe\.env.local.example") $feEnv
    }
    Ensure-EnvLine -FilePath $feEnv -Key "ALLOWED_DEV_ORIGINS" -Value $devOrigins
    Write-Host "Next.js: fe/.env.local ← ALLOWED_DEV_ORIGINS ($DomainName)" -ForegroundColor DarkGray
}

if (-not (Test-Path "$BeRoot\.env")) {
    Write-Error "be/.env 가 없습니다. cp be/.env.example be/.env 후 설정하세요."
}

# compose 변수 — 리포 루트 .env
$envFile = Join-Path $ProjectRoot ".env"
@(
    "NGINX_HTTP_PORT=80",
    "NGINX_HTTPS_PORT=443",
    "NGINX_CONF=nginx.dual.conf",
    "NGINX_ROUTES=app_routes.host-fe.conf",
    "NGINX_CERT_DIR=./deploy/certs"
) | Set-Content -Path $envFile -Encoding utf8

Ensure-TlsCerts -CertDir (Join-Path $ProjectRoot "deploy\certs") -Cn $Domain

Ensure-DomainOrigins -DomainName $Domain

if (-not $SkipBackend) {
    & "$BeRoot\scripts\restart.ps1" -Background
}

Write-Host "Starting nginx (80/443 → host:3000, no Docker FE)..."
$prevEap = $ErrorActionPreference
$ErrorActionPreference = "Continue"
docker compose --profile prod stop fe 2>&1 | Out-Null
docker compose --profile prod rm -f fe 2>&1 | Out-Null
docker compose --profile prod up -d --no-deps nginx 2>&1 | Out-Null
$ErrorActionPreference = $prevEap
if ($LASTEXITCODE -ne 0) {
    docker compose --profile prod up -d --no-deps nginx
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if (-not $SkipFirewall) {
    try {
        Enable-ExternalDockerAccess
        Open-FirewallPort -Port 80
        Open-FirewallPort -Port 443
        Open-FirewallPort -Port 3000
    } catch {
        Write-Warning "Firewall rules need admin. Run as Administrator or add TCP 80/443/3000 manually."
    }
}

Start-FeDevServer -Port 3000

# nginx 설정·볼륨 변경 반영 (fe 컨테이너는 띄우지 않음)
$prevEap = $ErrorActionPreference
$ErrorActionPreference = "Continue"
docker compose --profile prod up -d --no-deps --force-recreate nginx 2>&1 | Out-Null
$ErrorActionPreference = $prevEap

Show-ConnectivityReport -DomainName $Domain

Write-Host ""
Write-Host "=== Domain dev stack ready ===" -ForegroundColor Cyan
Write-Host "  FE dev:  http://127.0.0.1:3000  (direct)"
Write-Host "  nginx:   http://$Domain/  -> :3000"
Write-Host "  nginx:   https://$Domain/ -> :3000 (self-signed unless you replace deploy/certs/)"
Write-Host "  FE log:  deploy/logs/fe-dev.log"
Write-Host ""
