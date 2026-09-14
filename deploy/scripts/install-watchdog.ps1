<#
.SYNOPSIS
    watchdog.ps1 을 작업 스케줄러에 등록한다 (부팅 시 + 5분 주기).

.DESCRIPTION
    트리거를 둘 두는 이유가 다르다.

      * **로그온 시** — 2026-09-08 사고의 그 자리. 부팅 직후 Docker Desktop 을
        올리는 것이 96분 중단을 0으로 만드는 유일한 조치다. 지연 1분을 주는 것은
        부팅 직후의 디스크 경합을 피하기 위해서다.
      * **5분 주기** — 부팅과 무관한 사고(엔진 죽음, 컨테이너 크래시 루프)를 잡는다.

    로그온 트리거를 쓰는 이유: Docker Desktop 은 사용자 세션에 붙는 GUI 앱이라
    「사용자가 로그인하지 않아도 실행」으로는 뜨지 않는다. 그래서 이 태스크도
    로그온한 사용자로 돈다. **자동 로그인이 아니면 부팅 후 로그인 전까지는 여전히
    스택이 없다** — 무인 운영이 필요하면 자동 로그인을 켜거나 OCI 로 옮겨야 한다.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File deploy\scripts\install-watchdog.ps1
    powershell -ExecutionPolicy Bypass -File deploy\scripts\install-watchdog.ps1 -Remove
#>

param(
    [string]$TaskName = "quant-port watchdog",
    [int]$IntervalMinutes = 5,
    [switch]$Remove
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$ScriptDir    = $PSScriptRoot
$WatchdogPath = Join-Path $ScriptDir "watchdog.ps1"
$LauncherPath = Join-Path $ScriptDir "watchdog-hidden.vbs"

if (-not (Test-Path $WatchdogPath)) {
    Write-Error "watchdog.ps1 을 찾지 못했다: $WatchdogPath"
}
if (-not (Test-Path $LauncherPath)) {
    Write-Error "watchdog-hidden.vbs 를 찾지 못했다: $LauncherPath"
}

if ($Remove) {
    $existing = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if ($null -eq $existing) {
        Write-Host "등록된 '$TaskName' 태스크가 없다."
        exit 0
    }
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
    Write-Host "'$TaskName' 등록을 해제했다."
    exit 0
}

# powershell.exe 를 직접 부르지 않는다. `-WindowStyle Hidden` 을 줘도 powershell 은
# **콘솔 창을 먼저 만들고 나서** 숨기기 때문에 매 실행마다 창이 번쩍이고 키보드
# 포커스를 빼앗는다 — 5분마다면 쓰던 작업이 끊긴다(2026-09-09 사용자 지적).
# wscript 는 자기 콘솔이 없어서 거기서 띄우면 아무것도 보이지 않는다.
$action = New-ScheduledTaskAction `
    -Execute "wscript.exe" `
    -Argument "`"$LauncherPath`""

# 부팅 후 첫 회: 로그온 1분 뒤.
$logonTrigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
$logonTrigger.Delay = "PT1M"

# 주기 회차: 지금부터 5분 간격으로 무기한.
$repeatTrigger = New-ScheduledTaskTrigger -Once -At (Get-Date).AddMinutes(1) `
    -RepetitionInterval (New-TimeSpan -Minutes $IntervalMinutes)

$settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -StartWhenAvailable `
    -MultipleInstances IgnoreNew `
    -ExecutionTimeLimit (New-TimeSpan -Minutes 15)

$principal = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType Interactive -RunLevel Limited

$existing = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
if ($null -ne $existing) {
    Write-Host "기존 '$TaskName' 태스크를 지우고 다시 등록한다."
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
}

Register-ScheduledTask `
    -TaskName $TaskName `
    -Action $action `
    -Trigger @($logonTrigger, $repeatTrigger) `
    -Settings $settings `
    -Principal $principal `
    -Description "quant-port 스택 헬스 점검 · 자동 복구 · 알림 (deploy/scripts/watchdog.ps1)" | Out-Null

Write-Host ""
Write-Host "등록 완료: $TaskName"
Write-Host "  트리거   로그온 +1분, 그리고 $IntervalMinutes 분마다"
Write-Host "  로그     deploy\logs\watchdog.log"
Write-Host "  알림     deploy\watchdog.config.json 의 WebhookUrl (비어 있으면 로그만)"
Write-Host ""
Write-Host "지금 한 번 돌려보기:"
Write-Host "  Start-ScheduledTask -TaskName '$TaskName'"
Write-Host "고치지 않고 상태만 보기:"
Write-Host "  powershell -ExecutionPolicy Bypass -File deploy\scripts\watchdog.ps1 -CheckOnly"
