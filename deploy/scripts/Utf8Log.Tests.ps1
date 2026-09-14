<#
.SYNOPSIS
    Start-Utf8Log 이 한글을 깨뜨리지 않는지 실제로 돌려 본다.

.DESCRIPTION
    회귀 대상은 dev 서버 요청 로그의 한 줄이다:

        [2026-09-04 00:50:27.319 KST] user=3 name=홍길동 GET /

    고치기 전에는 이 줄이 `name=? ????? ?` 로 남았다. 원인이 프록시가 아니라
    **로그를 받아 적는 PowerShell** 이었으므로(Utf8Log.ps1 머리글), 검증도
    프록시가 아니라 그 파이프를 통과시켜야 한다.

    그래서 이 테스트는 Node 로 같은 모양의 줄을 stdout·stderr 에 내보내고,
    Start-Utf8Log 로 받아 적은 뒤, 파일을 **UTF-8 로 다시 읽어** 원문과 비교한다.
    Node 를 쓰는 이유는 실제 파이프의 반대편이 Next dev 서버(Node)이기 때문이다.

.EXAMPLE
    powershell -NoProfile -File deploy\scripts\Utf8Log.Tests.ps1
#>

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "Utf8Log.ps1")

$failures = 0
function Assert-Equal {
    param([string]$Name, [string]$Expected, [string]$Actual)
    if ($Expected -ceq $Actual) {
        Write-Host "  ok   $Name" -ForegroundColor Green
    } else {
        Write-Host "  FAIL $Name" -ForegroundColor Red
        Write-Host "       expected: $Expected"
        Write-Host "       actual:   $Actual"
        $script:failures++
    }
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("utf8log-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
$logFile = Join-Path $tmp "fe-dev.log"
$script = Join-Path $tmp "emit.js"

# 요청 로그 한 줄 + 한글만 있는 줄 + stderr 한 줄. Next dev 서버가 세 갈래를 다 쓴다.
$stdoutLine = "[2026-09-04 00:50:27.319 KST] user=3 name=홍길동 GET /news/domestic"
$hangulOnly = "종목명 · 삼성전자 · 에스케이하이닉스"
$stderrLine = "경고: 캔들 예산 소진 — 조회를 건너뜁니다"

$js = @"
process.stdout.write($($stdoutLine | ConvertTo-Json) + '\n');
process.stdout.write($($hangulOnly | ConvertTo-Json) + '\n');
process.stderr.write($($stderrLine | ConvertTo-Json) + '\n');
"@
[System.IO.File]::WriteAllText($script, $js, (New-Object System.Text.UTF8Encoding($false)))

Write-Host "Start-Utf8Log:"
try {
    Start-Utf8Log -Command 'node' -Arguments @($script) -LogFile $logFile -Overwrite | Out-Null

    $bytes = [System.IO.File]::ReadAllBytes($logFile)
    # BOM 이 없어야 한다 — 다른 도구가 첫 줄 앞에 보이지 않는 글자를 읽는다.
    if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF) {
        Write-Host "  FAIL log file starts with a UTF-8 BOM" -ForegroundColor Red
        $failures++
    } else {
        Write-Host "  ok   log file has no BOM" -ForegroundColor Green
    }
    # UTF-16LE 로 적히던 회귀: ASCII 사이에 0x00 이 끼어 있으면 그 증상이다.
    if ($bytes -contains 0x00) {
        Write-Host "  FAIL log file contains NUL bytes (UTF-16 regression)" -ForegroundColor Red
        $failures++
    } else {
        Write-Host "  ok   log file is not UTF-16" -ForegroundColor Green
    }

    $lines = [System.Text.Encoding]::UTF8.GetString($bytes) -split "`r?`n" | Where-Object { $_ -ne "" }
    Assert-Equal "요청 로그의 한글 사용자명이 그대로 남는다" $stdoutLine $lines[0]
    Assert-Equal "한글만 있는 줄도 그대로 남는다" $hangulOnly $lines[1]
    Assert-Equal "stderr 로 나간 한글도 그대로 남는다" $stderrLine $lines[2]

    # 앞선 세션이 UTF-16 으로 남긴 파일에 이어 쓰면 한 파일에 두 인코딩이 섞인다.
    # 그런 파일은 옆으로 치우고 새로 시작해야 한다.
    $legacy = Join-Path $tmp "legacy.log"
    [System.IO.File]::WriteAllText($legacy, "옛 줄`r`n", [System.Text.Encoding]::Unicode)
    Start-Utf8Log -Command 'node' -Arguments @($script) -LogFile $legacy | Out-Null
    $legacyBytes = [System.IO.File]::ReadAllBytes($legacy)
    if ($legacyBytes -contains 0x00) {
        Write-Host "  FAIL appended to a UTF-16 log instead of rotating it" -ForegroundColor Red
        $failures++
    } else {
        Write-Host "  ok   UTF-16 로그는 옆으로 치우고 새로 시작한다" -ForegroundColor Green
    }
    if (-not (Get-ChildItem -Path $tmp -Filter "legacy.log.*.utf16.bak")) {
        Write-Host "  FAIL rotated copy of the UTF-16 log was not kept" -ForegroundColor Red
        $failures++
    } else {
        Write-Host "  ok   치워 둔 옛 로그도 남는다" -ForegroundColor Green
    }
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

if ($failures -gt 0) {
    Write-Host "$failures check(s) failed." -ForegroundColor Red
    exit 1
}
Write-Host "All checks passed." -ForegroundColor Green
