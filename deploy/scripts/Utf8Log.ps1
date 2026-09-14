<#
.SYNOPSIS
    한글이 깨지지 않는 로그 파이프.

.DESCRIPTION
    2026-09-04 사용자 지적: 「로그에서 name 에 한글이 들어가는 경우 깨진다」.

    실측 (deploy/logs/fe-dev.log):

        [2026-09-04 00:50:27.319 KST] user=3 name=? ????? ? GET /

    프록시가 찍은 값 자체는 멀쩡했다 — `fe/src/lib/access-token-claims.ts` 가
    JWT 페이로드를 바이트로 받아 TextDecoder('utf-8') 로 푸는 경로이고,
    `fe/src/lib/request-log.test.ts` 가 한글 이름을 그대로 지켜 준다.

    깨진 자리는 **로그를 받아 적는 PowerShell** 이었다. 두 군데다.

      1. [Console]::OutputEncoding 이 콘솔 기본 코드페이지(대개 949/437)라,
         Node 가 내보낸 UTF-8 바이트를 PowerShell 이 그 코드페이지로 **디코딩**한다.
         이 시점에 이미 문자열이 망가지므로 뒤에서 무엇을 해도 되살릴 수 없다.

      2. Windows PowerShell 5.1 의 Tee-Object 는 -Encoding 매개변수가 없고
         파일을 UTF-16LE 로 쓴다. 그래서 로그 파일이 UTF-8 도구(git, docker,
         에디터, 이 저장소의 다른 스크립트)에서 한 글자씩 벌어져 읽힌다.

    이 함수는 둘을 같이 고친다: 자식 프로세스의 출력을 UTF-8 로 읽고, 파일에도
    UTF-8 로 적는다. 콘솔에도 그대로 흘려보내므로 Tee-Object 를 대신한다.

    검증은 `deploy/scripts/Utf8Log.Tests.ps1` 이 한다 (한글을 내보내는 짧은
    Node 프로그램을 이 파이프로 돌린 뒤 파일을 UTF-8 로 다시 읽어 비교).
#>

Set-StrictMode -Version Latest

function Start-Utf8Log {
    param(
        [Parameter(Mandatory = $true)][string]$Command,
        [string[]]$Arguments = @(),
        [Parameter(Mandatory = $true)][string]$LogFile,
        # 기본은 이어쓰기 — dev 서버를 다시 띄워도 앞선 세션의 줄이 남는다.
        [switch]$Overwrite
    )

    # ① 자식의 stdout/stderr 를 UTF-8 로 읽는다. 이 줄이 없으면 한글이 여기서 깨진다.
    $prevOut = [Console]::OutputEncoding
    $prevIn = $OutputEncoding
    [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
    $OutputEncoding = New-Object System.Text.UTF8Encoding($false)

    $logDir = Split-Path -Parent $LogFile
    if ($logDir -and -not (Test-Path $logDir)) {
        New-Item -ItemType Directory -Force -Path $logDir | Out-Null
    }

    # ①' 앞선 세션이 UTF-16(옛 Tee-Object)으로 남긴 파일에는 이어 쓰지 않는다.
    #    UTF-8 을 뒤에 붙이면 한 파일 안에 두 인코딩이 섞여, 어느 쪽으로 읽어도
    #    절반이 깨진다. 옆으로 치워 두고 새로 시작한다 — 옛 줄도 잃지 않는다.
    if ((-not $Overwrite) -and (Test-Path $LogFile)) {
        $head = New-Object byte[] 2
        $probe = [System.IO.File]::Open($LogFile, 'Open', 'Read', 'ReadWrite')
        $read = $probe.Read($head, 0, 2)
        $probe.Dispose()
        $isUtf16 = $read -eq 2 -and (($head[0] -eq 0xFF -and $head[1] -eq 0xFE) -or $head[1] -eq 0x00)
        if ($isUtf16) {
            $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
            Move-Item -Path $LogFile -Destination "$LogFile.$stamp.utf16.bak" -Force
        }
    }

    # ② 파일에도 UTF-8(BOM 없음)로 적는다. StreamWriter 를 파이프라인 전체 동안
    #    한 번만 열어 두므로 Add-Content 처럼 줄마다 파일을 여닫지 않는다.
    $writer = New-Object System.IO.StreamWriter(
        $LogFile,
        (-not $Overwrite),
        (New-Object System.Text.UTF8Encoding($false))
    )
    $writer.AutoFlush = $true

    # 2>&1 은 5.1 에서 stderr 한 줄을 ErrorRecord 로 감싼다. 부르는 쪽이
    # ErrorActionPreference='Stop' 이면 그 한 줄이 **종료 오류**가 되어 dev 서버가
    # 첫 경고에서 죽는다. 로그를 받아 적는 자리에서 stderr 는 오류가 아니라 내용이다.
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        # "$_" 로 문자열을 뽑아내야 ErrorRecord 가 스택 블록으로 부풀지 않는다.
        & $Command @Arguments 2>&1 | ForEach-Object {
            $line = "$_"
            $writer.WriteLine($line)
            $line
        }
    } finally {
        $ErrorActionPreference = $prevEap
        $writer.Dispose()
        [Console]::OutputEncoding = $prevOut
        $OutputEncoding = $prevIn
    }
}
