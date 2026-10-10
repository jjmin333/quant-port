param(
    [switch]$Background,
    # API만 다시 올린다(워커는 그대로). 워커를 다시 만들면 **돌고 있던 배치가 죽는다** —
    # 2026-09-23~25 새벽 재배포가 경고 스윕·섹터 동기화를 매번 끊었다. BE 변경이 워커
    # 코드에 닿지 않으면 이것을 쓴다.
    [switch]$ApiOnly,
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

Write-Host "Building & starting Docker stack (redis + translator + sentiment + api + tesseract + batch workers)..."
# translator 는 `legacy-translator` 프로파일에 있다 — 2026-09-15에 번역을 자체
# 모델(GPU)로 되돌리면서 기본 기동에 다시 넣었다. 프로파일 이름을 그대로 두는
# 이유는 되돌리는 길을 한 줄로 남겨 두기 위해서다(.env 의 TRANSLATE_PROVIDER).
# 워커는 성격별 넷(2026-10-01): 시세·종가 / 주식 데이터·점수 / 자산 / AI 콘텐츠(api 바이너리 batch 역할).
$services = @("redis", "translator", "sentiment", "api", "worker-market", "worker-stock", "worker-asset", "worker-content")
if ($ApiOnly) {
    $services = @("redis", "api")
}
# Windows PowerShell 5.1은 네이티브 명령의 stderr 한 줄마다 ErrorRecord를 만들고,
# 위의 ErrorActionPreference=Stop 이 그것을 종료 오류로 바꾼다. docker compose는 빌드
# 진행 로그를 **stderr로** 쓰므로 성공한 빌드의 첫 줄에서 스크립트가 죽었다
# (NativeCommandError). 이 구간만 Continue로 두고 줄을 문자열로 풀어 출력하며,
# 성패는 종료 코드로만 판정한다.
$prevEap = $ErrorActionPreference
$ErrorActionPreference = "Continue"
# 워커를 9→4로 합친 뒤(2026-10-01) 옛 워커 컨테이너가 남아 같은 잡을 두 번 돌리지 않게 이름으로 지운다.
# (--remove-orphans는 프로파일로 빠진 fe·nginx까지 건드릴 수 있어 쓰지 않는다.)
if (-not $ApiOnly) {
    foreach ($old in @("worker-close-domestic", "worker-close-overseas", "worker-scores", "worker-index", "worker-fetch", "worker-research", "worker-earnings", "worker-filings", "worker-growth")) {
        $name = "quant-port-$old"
        if (docker ps -aq --filter "name=^$name$") { docker rm -f $name 2>&1 | Out-Null; Write-Host "removed legacy $name" }
    }
}
docker compose --profile legacy-translator up -d --build @services 2>&1 | ForEach-Object { "$_" }
$composeExit = $LASTEXITCODE
$ErrorActionPreference = $prevEap
if ($composeExit -ne 0) {
    exit $composeExit
}

function Wait-ApiHealthy {
    param([int]$Seconds)
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        try {
            $res = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/health" -UseBasicParsing -TimeoutSec 3
            if ($res.StatusCode -eq 200) {
                return $true
            }
        } catch {
            Start-Sleep -Seconds 2
        }
    }
    return $false
}

Write-Host "Waiting for API health (max ${WaitSeconds}s)..."
$healthy = Wait-ApiHealthy -Seconds $WaitSeconds

# 2026-09-25: 재생성된 api 컨테이너가 **어떤 네트워크에도 붙지 않은 채**(Networks {}) 떠서
# MySQL(host.docker.internal)에 "network is unreachable"로 재시작만 반복했다. 빌드 없이
# 한 번 강제 재생성하면 붙는다 — 그 한 번을 여기서 한다.
if (-not $healthy) {
    $nets = docker inspect quant-port-api --format '{{range $k,$v := .NetworkSettings.Networks}}{{$k}} {{end}}' 2>$null
    Write-Warning "API not healthy (networks: '$nets') — recreating api once without build..."
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    docker compose up -d --no-build --force-recreate api 2>&1 | ForEach-Object { "$_" }
    $ErrorActionPreference = $prevEap
    $healthy = Wait-ApiHealthy -Seconds $WaitSeconds
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
