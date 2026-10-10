<#
.SYNOPSIS
    스택이 살아 있는지 보고, 죽어 있으면 되살리고, 상태가 바뀌면 알린다.

.DESCRIPTION
    2026-09-08 사고에서 나온 스크립트다. 호스트가 커널 버그체크(0x0000000A)로
    죽고 22:08:51에 재부팅됐는데, **스택은 23:44까지 96분을 더 누워 있었다.**

    컨테이너는 13개 전부 `restart: unless-stopped`였다. 설정은 옳았다.
    문제는 그 정책이 **도커 엔진이 살아 있을 때만** 동작한다는 것이다 —
    Docker Desktop이 `AutoStart=False`라 부팅 후 엔진이 뜨지 않았고,
    엔진이 없으니 재시작시킬 주체가 없었다. 총 중단 1시간 37분 중
    재부팅은 60초였고 나머지 96분이 이 구멍이다.

    그래서 이 스크립트가 메우는 자리는 **컨테이너 아래층(엔진)과 위층(관측)**이다.
    컨테이너 층은 이미 도커가 지키므로 건드리지 않는다.

.NOTES
    Windows PowerShell 5.1 기준으로 쓴다(`&&`·삼항·`??` 없음).
    작업 스케줄러 등록은 `install-watchdog.ps1`.
#>

param(
    # 스택을 실제로 고치지 않고 상태만 본다. 등록 전에 눈으로 확인할 때 쓴다.
    [switch]$CheckOnly,
    [int]$HealthTimeoutSec = 5,
    # 엔진이 뜨기를 기다리는 시간. Docker Desktop은 찬 시동이 1~2분 걸린다.
    [int]$EngineWaitSec = 240,
    [switch]$Quiet
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$ScriptDir   = $PSScriptRoot
$DeployRoot  = Split-Path -Parent $ScriptDir
$ProjectRoot = Split-Path -Parent $DeployRoot
$LogDir      = Join-Path $DeployRoot "logs"
$LogFile     = Join-Path $LogDir "watchdog.log"
$StateFile   = Join-Path $LogDir "watchdog-state.json"
$ConfigFile  = Join-Path $DeployRoot "watchdog.config.json"

# 사용자가 일부러 스택을 내렸다는 표시. 이게 있으면 워치독은 아무것도 되살리지
# 않는다 — 없으면 `stop.ps1`로 내린 스택을 5분 뒤에 도로 올려 놓는 물건이 된다.
# 2026-09-15부터 **만료 시각을 갖는다** — `Test-WatchdogPaused` 머리글 참조.
# 파일은 `watchdog-pause.ps1` 이 만든다.
$PauseFile   = Join-Path $DeployRoot "watchdog.paused"

$ComposeProject = "quant-port"

if (-not (Test-Path $LogDir)) {
    New-Item -ItemType Directory -Force -Path $LogDir | Out-Null
}

# ---- 로그 -----------------------------------------------------------------
# Out-File/Set-Content 대신 .NET으로 직접 쓴다. 5.1의 Tee-Object·Out-File은
# 인코딩이 콘솔 코드페이지에 끌려다녀 한글이 깨진다(deploy/scripts/Utf8Log.ps1 참조).

function Write-WatchdogLog {
    param([string]$Level, [string]$Message)

    $stamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
    $line = "[$stamp] $Level $Message"

    try {
        if (Test-Path $LogFile) {
            $size = (Get-Item $LogFile).Length
            if ($size -gt 5MB) {
                Move-Item $LogFile "$LogFile.1" -Force
            }
        }
        [System.IO.File]::AppendAllText($LogFile, $line + "`r`n", [System.Text.Encoding]::UTF8)
    } catch {
        # 로그를 못 써도 점검 자체는 계속한다.
    }

    if (-not $Quiet) {
        if ($Level -eq "ERROR") { Write-Host $line -ForegroundColor Red }
        elseif ($Level -eq "WARN") { Write-Host $line -ForegroundColor Yellow }
        else { Write-Host $line }
    }
}

# ---- 설정 · 상태 -----------------------------------------------------------

function Get-WatchdogConfig {
    $default = [ordered]@{ WebhookUrl = ""; NotifyOnRecovery = $true }
    if (-not (Test-Path $ConfigFile)) { return $default }
    try {
        $raw = Get-Content $ConfigFile -Raw -Encoding UTF8
        $cfg = $raw | ConvertFrom-Json
        if ($cfg.PSObject.Properties.Name -contains "WebhookUrl") { $default.WebhookUrl = $cfg.WebhookUrl }
        if ($cfg.PSObject.Properties.Name -contains "NotifyOnRecovery") { $default.NotifyOnRecovery = $cfg.NotifyOnRecovery }
    } catch {
        Write-WatchdogLog "WARN" "watchdog.config.json 을 읽지 못했다: $($_.Exception.Message)"
    }
    return $default
}

function Get-WatchdogState {
    if (-not (Test-Path $StateFile)) { return "unknown" }
    try {
        $s = (Get-Content $StateFile -Raw -Encoding UTF8) | ConvertFrom-Json
        return $s.Status
    } catch {
        return "unknown"
    }
}

function Set-WatchdogState {
    param([string]$Status)
    try {
        $obj = [ordered]@{ Status = $Status; At = (Get-Date).ToString("o") }
        $json = $obj | ConvertTo-Json -Compress
        [System.IO.File]::WriteAllText($StateFile, $json, [System.Text.Encoding]::UTF8)
    } catch {
        Write-WatchdogLog "WARN" "상태 파일을 쓰지 못했다: $($_.Exception.Message)"
    }
}

# ---- 알림 ------------------------------------------------------------------
# Discord 는 `content`, Slack 은 `text` 를 읽는다. 둘 다 담아 보내면 서로
# 모르는 키를 무시하므로 어느 쪽 웹훅이든 그대로 붙는다.

function Send-WatchdogNotice {
    param([string]$Text)

    $cfg = Get-WatchdogConfig
    if ([string]::IsNullOrWhiteSpace($cfg.WebhookUrl)) {
        Write-WatchdogLog "INFO" "웹훅 미설정 — 알림은 로그에만 남는다 (deploy/watchdog.config.json)"
        return
    }
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        $body = @{ content = $Text; text = $Text } | ConvertTo-Json -Compress
        Invoke-RestMethod -Uri $cfg.WebhookUrl -Method Post -ContentType "application/json; charset=utf-8" `
            -Body ([System.Text.Encoding]::UTF8.GetBytes($body)) -TimeoutSec 15 | Out-Null
        Write-WatchdogLog "INFO" "알림 전송함"
    } catch {
        Write-WatchdogLog "WARN" "알림 전송 실패: $($_.Exception.Message)"
    }
}

# ---- 점검 ------------------------------------------------------------------

function Test-StackHealth {
    # 사용자가 실제로 지나는 길(nginx :80)을 본다. api 를 직접 찌르면 nginx 가
    # 죽은 채로도 「정상」이 나와, 정작 화면이 안 뜨는 상태를 못 잡는다.
    try {
        $res = Invoke-WebRequest -Uri "http://127.0.0.1/health" -UseBasicParsing -TimeoutSec $HealthTimeoutSec
        return ($res.StatusCode -eq 200)
    } catch {
        return $false
    }
}

<#
    의존성까지 살아 있는가 — `/ready` 는 MySQL·Redis 를 실제로 찔러 본다.

    `/health` 와 갈라 두는 이유가 핵심이다. `/health` 는 **liveness 뿐**이라
    프로세스만 살아 있으면 MySQL 이 죽어도 200 을 돌려준다. 그것만 보면 DB 장애가
    「정상」으로 보인다.

    그런데 `/ready` 실패를 **복구 신호로 쓰면 안 된다.** OCI MySQL 이 죽은 것을
    Docker Desktop 재기동으로 고칠 수 없고, 시도하면 고칠 수 없는 조건에서
    재기동을 반복한다. 그래서 역할을 나눈다:

      /health 실패 → 스택이 없다      → **고친다**
      /ready  실패 → 스택은 있는데 의존성이 아프다 → **알리기만 한다**
#>
function Test-StackReady {
    try {
        $res = Invoke-WebRequest -Uri "http://127.0.0.1/ready" -UseBasicParsing -TimeoutSec $HealthTimeoutSec
        # 상태 코드만 보면 안 된다. 핸들러는 **MySQL 이 죽을 때만 503** 이고,
        # Redis 만 죽으면 200 에 `"status":"degraded"` 를 담아 보낸다. Redis 는
        # Toss 토큰 저장소라 죽으면 시세 적재가 끊긴다 — 200 이라고 넘기면 안 된다.
        $ok = $res.StatusCode -eq 200
        if ($ok -and $res.Content -notmatch '"status"\s*:\s*"ready"') { $ok = $false }
        return @{ ok = $ok; detail = $res.Content }
    } catch {
        $detail = $_.Exception.Message
        # 503 이면 본문에 어느 의존성이 아픈지가 들어 있다.
        try {
            $stream = $_.Exception.Response.GetResponseStream()
            $reader = New-Object System.IO.StreamReader($stream)
            $body = $reader.ReadToEnd()
            if ($body) { $detail = $body }
        } catch { }
        return @{ ok = $false; detail = $detail }
    }
}

function Test-DockerEngine {
    try {
        docker info --format "{{.ServerVersion}}" 2>$null | Out-Null
        return ($LASTEXITCODE -eq 0)
    } catch {
        return $false
    }
}

function Test-DockerDesktopRunning {
    $p = Get-Process -Name "Docker Desktop" -ErrorAction SilentlyContinue
    return ($null -ne $p)
}

function Start-DockerDesktop {
    $candidates = @(
        "$env:ProgramFiles\Docker\Docker\Docker Desktop.exe",
        "${env:ProgramFiles(x86)}\Docker\Docker\Docker Desktop.exe",
        "$env:LOCALAPPDATA\Docker\Docker Desktop.exe"
    )
    foreach ($path in $candidates) {
        if (Test-Path $path) {
            Write-WatchdogLog "INFO" "Docker Desktop 기동: $path"
            Start-Process -FilePath $path | Out-Null
            return $true
        }
    }
    Write-WatchdogLog "ERROR" "Docker Desktop 실행 파일을 찾지 못했다"
    return $false
}

function Wait-DockerEngine {
    param([int]$TimeoutSec)
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        if (Test-DockerEngine) { return $true }
        Start-Sleep -Seconds 5
    }
    return $false
}

<#
    Docker Desktop 이 **모달 오류 대화상자**에 걸려 있는지 본다.

    2026-09-09 검증에서 드러난 자리다. 워치독은 Docker Desktop 이 없다는 것을
    정확히 알아채고 기동까지 했는데(01:14:54), 엔진은 끝내 뜨지 않았다 — Docker
    Desktop 이 「An unexpected error occurred」 대화상자를 띄우고 **사람이 Quit 을
    누르기를 기다리고** 있었기 때문이다. 서버에는 누를 사람이 없다.

    원인은 고아가 된 AF_UNIX 소켓 파일이었다. Docker Desktop 이 비정상 종료하면
    `%LOCALAPPDATA%` 아래에 지울 수도 열 수도 없는 소켓 파일이 남고, 다음 기동이
    그것을 remove 하려다 죽는다. 사람이 하는 조치는 그 파일이 든 디렉터리를
    치우는 것뿐이고, Docker 가 다시 만든다. 그래서 자동화할 수 있다.
#>
function Get-DockerBackendError {
    $path = Join-Path $env:LOCALAPPDATA "Docker\backend.error.json"
    if (-not (Test-Path $path)) { return $null }
    try {
        $fi = Get-Item $path
        # 이번 회차의 오류만 본다 — 몇 시간 전 것에 반응하면 멀쩡한 스택을 건드린다.
        if ((New-TimeSpan -Start $fi.LastWriteTime -End (Get-Date)).TotalMinutes -gt 20) { return $null }
        $raw = Get-Content $path -Raw -Encoding UTF8
        $m = [regex]::Match($raw, '"description":"([^"]{0,400})')
        if (-not $m.Success) { return $null }
        return $m.Groups[1].Value
    } catch {
        return $null
    }
}

<#
    고아 소켓이 든 디렉터리를 치운다. **한 번만**, 그리고 아주 좁게.

    안전 조건 셋을 모두 만족할 때만 손댄다:
      1. 오류 메시지가 「remove <경로>」 꼴로 경로를 지목했다
      2. 그 경로가 %LOCALAPPDATA% 아래다
      3. 그 경로에 'ocker' 가 들어간다 (Docker / docker-secrets-engine)
    셋 중 하나라도 어긋나면 아무것도 하지 않는다 — 워치독이 모르는 경로를
    지우는 물건이 되면 안 된다.
#>
function Clear-OrphanedDockerSocket {
    param([string]$ErrorText)

    if ([string]::IsNullOrWhiteSpace($ErrorText)) { return $false }
    if ($ErrorText -notmatch "cannot be accessed") { return $false }

    $m = [regex]::Match($ErrorText, 'remove ([A-Za-z]:[^:]+?):')
    if (-not $m.Success) {
        Write-WatchdogLog "WARN" "오류가 경로를 지목하지 않았다 — 자동 정리를 건너뛴다"
        return $false
    }

    $sockPath = $m.Groups[1].Value -replace '/', '\'
    $dir = Split-Path -Parent $sockPath
    $local = $env:LOCALAPPDATA

    if (-not $dir.StartsWith($local, [System.StringComparison]::OrdinalIgnoreCase)) {
        Write-WatchdogLog "WARN" "정리 대상이 %LOCALAPPDATA% 밖이다 — 건드리지 않는다: $dir"
        return $false
    }
    if ($dir -notmatch "ocker") {
        Write-WatchdogLog "WARN" "정리 대상이 Docker 경로가 아니다 — 건드리지 않는다: $dir"
        return $false
    }
    if (-not (Test-Path $dir)) { return $false }

    # 디렉터리를 지우지 않고 **이름만 바꾼다.** 고아 파일은 지워지지 않지만
    # 옮겨는 지고, Docker 는 원래 이름으로 새로 만든다. 되돌릴 수도 있다.
    $leaf = Split-Path -Leaf $dir
    $newName = "$leaf.orphan-$(Get-Date -Format 'yyyyMMdd-HHmmss')"
    try {
        Get-Process -Name "Docker Desktop", "com.docker.backend", "com.docker.build" -ErrorAction SilentlyContinue |
            Stop-Process -Force -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 5
        Rename-Item -Path $dir -NewName $newName -ErrorAction Stop
        Write-WatchdogLog "INFO" "고아 소켓 디렉터리를 치웠다: $dir -> $newName"
        return $true
    } catch {
        Write-WatchdogLog "ERROR" "고아 소켓 디렉터리 정리 실패: $($_.Exception.Message)"
        return $false
    }
}

<#
    사용자가 일부러 재워 뒀는가 — **그리고 그 재움이 아직 유효한가.**

    2026-09-15에 만료를 붙였다. 종전에는 `watchdog.paused` 를 손으로 만들고 손으로
    지워야 했는데, **지우는 것을 잊으면 워치독이 조용히 꺼진 채로 남는다** — 다음
    사고 때 아무도 깨워 주지 않는다는 뜻이다. 재움은 「게임 한 판」처럼 끝이 있는
    일이므로 끝을 파일에 적게 했다.

    시각을 `yyyyMMddHHmmss` 라는 **정렬 가능한 숫자 문자열**로 적는 이유가 있다.
    같은 파일을 VBS 런처가 먼저 읽는데(빠른 경로), VBScript 의 `CDate` 는 로캘에
    끌려다녀 ISO 8601 을 믿을 수 없다. 자리수가 고정되면 두 언어 모두 문자열
    비교만으로 시각을 비교할 수 있다.

    `until=` 이 없거나 `forever` 면 무기한 — 예전 동작 그대로다.
#>
function Test-WatchdogPaused {
    if (-not (Test-Path $PauseFile)) { return $false }

    $until = "forever"
    try {
        foreach ($line in (Get-Content $PauseFile -Encoding UTF8)) {
            $m = [regex]::Match($line, '^\s*until\s*=\s*(\S+)')
            if ($m.Success) { $until = $m.Groups[1].Value }
        }
    } catch {
        # 못 읽으면 재운 것으로 본다. 이쪽으로 틀리면 복구가 늦어질 뿐이지만,
        # 반대로 틀리면 사용자가 방금 내린 스택을 도로 올려 놓는다.
        Write-WatchdogLog "WARN" "watchdog.paused 를 읽지 못했다 — 재운 것으로 본다"
        return $true
    }

    if ($until -eq "forever") {
        Write-WatchdogLog "INFO" "watchdog.paused 있음(무기한) — 점검을 건너뛴다"
        return $true
    }

    # 둘 다 yyyyMMddHHmmss 이므로 문자열 비교가 곧 시각 비교다.
    if ($until -gt (Get-Date).ToString("yyyyMMddHHmmss")) {
        Write-WatchdogLog "INFO" "watchdog.paused 있음($until 까지) — 점검을 건너뛴다"
        return $true
    }

    Write-WatchdogLog "INFO" "watchdog.paused 만료($until) — 표시를 지우고 평소대로 돈다"
    Remove-Item $PauseFile -Force -ErrorAction SilentlyContinue
    return $false
}

<#
    멈춰 있는 컨테이너를 이름으로 되살린다.

    `docker compose up -d` 를 쓰지 않는 이유가 둘 있다.
      1. fe·nginx 는 `prod` 프로파일이라 프로파일 없이 up 하면 **빠진다.**
      2. up 은 compose 파일과 .env 를 다시 평가해 컨테이너를 재생성할 수 있다.
         워치독이 배포를 하면 안 된다 — 되살리기만 해야 한다.
#>
function Restart-StoppedContainers {
    $names = @()
    try {
        $raw = docker ps -a --filter "label=com.docker.compose.project=$ComposeProject" `
            --filter "status=exited" --filter "status=created" --format "{{.Names}}" 2>$null
        if ($LASTEXITCODE -eq 0 -and $raw) {
            $names = @($raw -split "`n" | Where-Object { $_.Trim() -ne "" })
        }
    } catch {
        Write-WatchdogLog "WARN" "멈춘 컨테이너 목록을 얻지 못했다: $($_.Exception.Message)"
        return 0
    }

    if ($names.Count -eq 0) { return 0 }

    Write-WatchdogLog "INFO" "멈춘 컨테이너 $($names.Count)개 기동: $($names -join ', ')"
    foreach ($n in $names) {
        docker start $n 2>$null | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Write-WatchdogLog "WARN" "$n 기동 실패"
        }
    }
    return $names.Count
}

<#
    api 컨테이너가 **어떤 네트워크에도 붙지 않은 채** 도는 경우를 고친다(2026-09-25 · 09-28 두 번).

    compose 재생성이 중간에 끊기면 옛 컨테이너가 `<id>_quant-port-api` 이름으로 남고, 위의
    Restart-StoppedContainers 가 그것을 start 하면 네트워크 없이 떠서 MySQL 에 "network is
    unreachable" 로 재시작만 반복한다. 09-28 23:25 에 그렇게 떠서 **2시간 20분** 동안 10분마다
    「복구 실패」만 적혔다 — start·restart 로는 안 붙는다.

    처방은 restart.ps1 과 같다: **빌드 없이** api 만 한 번 재생성. 지금 이미지 그대로라 배포가
    아니다(워치독은 배포하지 않는다는 원칙 그대로). api 가 네트워크 0개일 때만 한다.
#>
function Repair-DetachedApi {
    $ids = docker ps -a --filter "label=com.docker.compose.project=$ComposeProject" `
        --filter "label=com.docker.compose.service=api" --format "{{.ID}}" 2>$null
    if ($LASTEXITCODE -ne 0 -or -not $ids) { return $false }
    $detached = $false
    foreach ($id in @($ids -split "`n" | Where-Object { $_.Trim() -ne "" })) {
        $nets = docker inspect $id.Trim() --format "{{len .NetworkSettings.Networks}}" 2>$null
        if ("$nets".Trim() -eq "0") { $detached = $true }
    }
    if (-not $detached) { return $false }

    Write-WatchdogLog "WARN" "api 컨테이너가 네트워크 없이 떠 있다 — 빌드 없이 한 번 재생성한다"
    Push-Location $ProjectRoot
    try {
        docker compose up -d --no-deps --no-build --force-recreate api 2>$null | Out-Null
        $ok = ($LASTEXITCODE -eq 0)
    } finally {
        Pop-Location
    }
    if (-not $ok) { Write-WatchdogLog "WARN" "api 재생성 실패" }
    return $ok
}

# ---- 본문 ------------------------------------------------------------------

$prevStatus = Get-WatchdogState

if (Test-WatchdogPaused) {
    Set-WatchdogState "paused"
    exit 0
}

if (Test-StackHealth) {
    <#
        헬스가 200 이어도 **컨테이너 점검은 한다.**

        `/health` 는 nginx→api 한 줄만 지난다. 워커 8개나 redis 가 멈춰 있어도
        그 줄은 멀쩡해서 200 이 나온다 — 배치가 통째로 안 도는데 워치독은
        「정상」이라고 적는 상태가 된다. 실제로 09-08 다운 뒤에도 그랬을 수 있다.

        `docker ps` 한 번은 싸고, 여기서 되살리는 것은 아래 복구 경로와 같은
        「멈춘 것만 start」다(배포하지 않는다).
    #>
    $strayStarted = Restart-StoppedContainers
    if ($strayStarted -gt 0) {
        Write-WatchdogLog "WARN" "헬스는 정상인데 멈춘 컨테이너 ${strayStarted}개를 되살렸다"
        Send-WatchdogNotice "quant-port 부분 복구 — $(hostname) 헬스는 정상이었으나 멈춰 있던 컨테이너 ${strayStarted}개를 되살렸습니다."
    }

    # 스택은 서 있다. 이제 의존성까지 보는데, 여기서 아픈 것은 **고치지 않고 알린다**
    # (Test-StackReady 머리글 참조).
    $ready = Test-StackReady

    if ($ready.ok) {
        Set-WatchdogState "healthy"
        if ($prevStatus -eq "down" -or $prevStatus -eq "degraded") {
            Write-WatchdogLog "INFO" "복구 확인 — 스택 정상"
            $cfg = Get-WatchdogConfig
            if ($cfg.NotifyOnRecovery) {
                Send-WatchdogNotice "quant-port 복구됨 — $(hostname) 스택이 다시 정상입니다."
            }
        } else {
            Write-WatchdogLog "INFO" "정상"
        }
        exit 0
    }

    Write-WatchdogLog "WARN" "스택은 떠 있으나 의존성이 아프다 — /ready: $($ready.detail)"
    Set-WatchdogState "degraded"
    if ($prevStatus -ne "degraded") {
        # 상태가 바뀔 때만 알린다 — 매 5분 같은 알림을 보내면 다음부터 아무도 안 본다.
        Send-WatchdogNotice "quant-port 이상 — $(hostname) 앱은 떠 있으나 의존성 점검 실패. 재기동으로 고칠 수 없는 종류입니다(DB·Redis). /ready: $($ready.detail)"
    }
    exit 1
}

Write-WatchdogLog "WARN" "헬스 실패 — http://127.0.0.1/health 응답 없음"

if ($CheckOnly) {
    Write-WatchdogLog "INFO" "CheckOnly — 복구를 시도하지 않는다"
    Write-WatchdogLog "INFO" "Docker Desktop 실행중=$(Test-DockerDesktopRunning) 엔진응답=$(Test-DockerEngine)"
    exit 1
}

$repaired = @()

if (-not (Test-DockerDesktopRunning)) {
    # 2026-09-08 사고에서 96분을 만든 바로 그 자리다.
    Write-WatchdogLog "WARN" "Docker Desktop 이 떠 있지 않다 — 기동한다"
    if (Start-DockerDesktop) { $repaired += "Docker Desktop 기동" }
}

if (-not (Test-DockerEngine)) {
    Write-WatchdogLog "INFO" "엔진 응답을 최대 ${EngineWaitSec}초 기다린다"
    if (-not (Wait-DockerEngine -TimeoutSec $EngineWaitSec)) {

        # 엔진이 안 떴다 — Docker Desktop 이 모달 대화상자에 걸린 것인지 본다.
        # 걸렸다면 사람이 Quit 을 누르기 전까지 영원히 안 뜬다.
        $backendErr = Get-DockerBackendError
        if ($null -ne $backendErr) {
            Write-WatchdogLog "ERROR" "Docker Desktop 기동 실패: $backendErr"

            if (Clear-OrphanedDockerSocket -ErrorText $backendErr) {
                # 한 번만 다시 시도한다. 두 번째도 실패하면 사람 몫이다.
                Write-WatchdogLog "INFO" "정리 후 Docker Desktop 재기동 — 한 번만 다시 시도한다"
                if (Start-DockerDesktop) {
                    if (Wait-DockerEngine -TimeoutSec $EngineWaitSec) {
                        Write-WatchdogLog "INFO" "정리 후 엔진 기동 성공"
                        $repaired += "고아 소켓 정리 후 엔진 기동"
                    }
                }
            }
        }

        if (-not (Test-DockerEngine)) {
            Write-WatchdogLog "ERROR" "엔진이 뜨지 않았다 — 이번 회차 포기"
            Set-WatchdogState "down"
            if ($prevStatus -ne "down") {
                $detail = "도커 엔진이 뜨지 않았습니다."
                if ($null -ne $backendErr) {
                    # 사람이 화면을 안 봐도 원인을 알 수 있게 오류를 알림에 실어 보낸다.
                    # 이게 없으면 「엔진이 안 뜸」만 오고, 정작 원인은 모달 대화상자다.
                    $detail = "Docker Desktop 이 오류로 멈춰 있습니다(모달 대화상자일 수 있음): $backendErr"
                }
                Send-WatchdogNotice "quant-port 다운 — $(hostname) $detail 사람이 봐야 합니다."
            }
            exit 1
        }
    }
    if ($repaired -notcontains "고아 소켓 정리 후 엔진 기동") {
        $repaired += "엔진 기동 대기"
    }
}

# 엔진이 떴으면 `restart: unless-stopped` 가 스스로 대부분을 올린다.
# 잠깐 주고 나서, 그래도 남아 있는 것만 손으로 올린다.
Start-Sleep -Seconds 10
$started = Restart-StoppedContainers
if ($started -gt 0) { $repaired += "컨테이너 ${started}개 기동" }
if (Repair-DetachedApi) { $repaired += "네트워크 없는 api 재생성" }

# 헬스가 돌아오기를 기다린다 — 컨테이너 기동 직후는 아직 아니다.
$ok = $false
$deadline = (Get-Date).AddSeconds(90)
while ((Get-Date) -lt $deadline) {
    if (Test-StackHealth) { $ok = $true; break }
    Start-Sleep -Seconds 5
}

if ($ok) {
    Set-WatchdogState "healthy"
    $what = "확인만"
    if ($repaired.Count -gt 0) { $what = ($repaired -join ", ") }
    Write-WatchdogLog "INFO" "복구 완료 ($what)"
    Send-WatchdogNotice "quant-port 자동 복구됨 — $(hostname): $what"
    exit 0
}

Write-WatchdogLog "ERROR" "복구 실패 — 헬스가 90초 안에 돌아오지 않았다"
Set-WatchdogState "down"
if ($prevStatus -ne "down") {
    Send-WatchdogNotice "quant-port 다운 — $(hostname) 자동 복구에 실패했습니다. docker logs quant-port-api 를 보세요."
}
exit 1
