<#
.SYNOPSIS
    워치독을 한동안 재워 둔다. 게임·렌더링처럼 기계를 통째로 쓰는 동안 쓴다.

.DESCRIPTION
    `deploy/watchdog.paused` 를 만든다. 이 파일이 있으면 워치독은 아무것도
    되살리지 않고, VBS 런처는 `/health` 조차 찌르지 않는다 — 회차 비용이 0 이다.

    **만료 시각이 있다는 것이 예전과 다른 점이다.** 종전에는 파일을 손으로 만들고
    손으로 지워야 했는데, 지우는 것을 잊으면 워치독이 조용히 꺼진 채로 남았다.
    이제 `until=` 이 지나면 워치독이 그 파일을 스스로 지우고 평소대로 돈다.

    시각을 `yyyyMMddHHmmss` 라는 **정렬 가능한 숫자 문자열**로 적는 이유가 있다.
    같은 파일을 VBS 런처도 읽는데 VBScript 의 `CDate` 는 로캘에 끌려다녀
    ISO 8601 을 믿을 수 없다. 자리수가 고정되면 두 언어 모두 그냥 비교하면 된다.

    스택 자체는 건드리지 않는다. 컨테이너까지 내리고 싶으면 이 명령 뒤에
    `docker compose stop` 을 따로 하면 된다 — 그래야 워치독이 도로 올리지 않는다.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File deploy\scripts\watchdog-pause.ps1
    powershell -ExecutionPolicy Bypass -File deploy\scripts\watchdog-pause.ps1 -Hours 8 -Reason "렌더링"
    powershell -ExecutionPolicy Bypass -File deploy\scripts\watchdog-pause.ps1 -Forever
    powershell -ExecutionPolicy Bypass -File deploy\scripts\watchdog-pause.ps1 -Resume
    powershell -ExecutionPolicy Bypass -File deploy\scripts\watchdog-pause.ps1 -Status
#>

param(
    # 몇 시간 재울지. 기본 4시간 — 한 판 하고 잊어버려도 그날 안에 깨어난다.
    [double]$Hours = 4,
    # 만료 없이 재운다. 예전 동작이고, 깨우는 것을 잊을 수 있으니 기본이 아니다.
    [switch]$Forever,
    # 지금 깨운다.
    [switch]$Resume,
    # 고치지 않고 상태만 본다.
    [switch]$Status,
    [string]$Reason = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$ScriptDir  = $PSScriptRoot
$DeployRoot = Split-Path -Parent $ScriptDir
$PauseFile  = Join-Path $DeployRoot "watchdog.paused"

function Get-PauseUntil {
    if (-not (Test-Path $PauseFile)) { return $null }
    try {
        foreach ($line in (Get-Content $PauseFile -Encoding UTF8)) {
            $m = [regex]::Match($line, '^\s*until\s*=\s*(\S+)')
            if ($m.Success) { return $m.Groups[1].Value }
        }
    } catch { }
    return "forever"
}

function Show-Status {
    if (-not (Test-Path $PauseFile)) {
        Write-Host "워치독: 깨어 있음 (정상 동작 중)"
        return
    }
    $until = Get-PauseUntil
    if ($until -eq "forever") {
        Write-Host "워치독: 재우는 중 — 만료 없음. -Resume 으로 깨워야 한다."
        return
    }
    $now = (Get-Date).ToString("yyyyMMddHHmmss")
    if ($until -gt $now) {
        # 문자열을 다시 시각으로 읽는다. 여기는 PowerShell 이라 믿을 수 있다.
        $dt = [datetime]::ParseExact($until, "yyyyMMddHHmmss", $null)
        $left = [int]((New-TimeSpan -Start (Get-Date) -End $dt).TotalMinutes)
        Write-Host ("워치독: 재우는 중 — {0} 까지 (약 {1}분 남음)" -f $dt.ToString("MM-dd HH:mm"), $left)
    } else {
        Write-Host "워치독: 만료된 표시가 남아 있다 — 다음 회차가 지우고 평소대로 돈다."
    }
}

if ($Status) {
    Show-Status
    exit 0
}

if ($Resume) {
    if (Test-Path $PauseFile) {
        Remove-Item $PauseFile -Force
        Write-Host "워치독을 깨웠다. 다음 회차부터 평소대로 점검한다."
    } else {
        Write-Host "이미 깨어 있다."
    }
    exit 0
}

if ($Forever) {
    $until = "forever"
    $human = "만료 없음"
} else {
    if ($Hours -le 0) {
        Write-Error "-Hours 는 0 보다 커야 한다. 지금 깨우려면 -Resume 을 쓴다."
    }
    $dt = (Get-Date).AddHours($Hours)
    $until = $dt.ToString("yyyyMMddHHmmss")
    $human = $dt.ToString("MM-dd HH:mm") + " 까지"
}

$lines = @(
    "# quant-port watchdog paused",
    "# 이 파일이 있는 동안 워치독은 스택을 되살리지 않는다.",
    "# until 이 지나면 워치독이 이 파일을 스스로 지운다 (형식: yyyyMMddHHmmss 또는 forever).",
    "created=$((Get-Date).ToString('yyyyMMddHHmmss'))",
    "until=$until"
)
if ($Reason -ne "") { $lines += "reason=$Reason" }

# BOM 을 붙인 UTF-8. 읽는 쪽이 셋인데 기본 인코딩이 제각각이다 — 메모장·에디터,
# PowerShell 5.1 의 `Get-Content`(BOM 이 없으면 ANSI 로 읽어 한글이 깨진다),
# 그리고 ANSI 로 읽는 VBS 런처. BOM 이 앞의 둘을 맞춰 주고, VBS 쪽은 `until=` 을
# 줄 앞이 아니라 `InStr` 로 찾으므로 BOM 이 끼어들어도 값을 놓치지 않는다.
$enc = New-Object System.Text.UTF8Encoding($true)
[System.IO.File]::WriteAllText($PauseFile, ($lines -join "`r`n") + "`r`n", $enc)

Write-Host "워치독을 재웠다 — $human"
if ($Reason -ne "") { Write-Host "  사유: $Reason" }
Write-Host "  깨우기: -Resume  ·  상태: -Status"
Write-Host "  스택까지 내리려면 이 뒤에 docker compose stop 을 따로 한다."
