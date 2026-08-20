# Safe US daily-close backfill - chunked steps, full logs.
#
#   cd be
#   .\scripts\backfill-us-safe.ps1
#
# Tail log:
#   Get-Content (Get-ChildItem .\logs\backfill-us-* | Sort LastWriteTime -Desc | Select -First 1).FullName\run.log -Wait -Tail 30

param(
    [string]$Since = "2026-01-01",
    [string]$Market = "us",
    [int]$FetchBatchSize = 100,
    [int]$FetchLogEvery = 10,
    [int]$FetchDelayMs = 400,
    [int]$FetchTimeoutMin = 90,
    [int]$ProjectionTimeoutMin = 20,
    [int]$PauseSec = 15,
    [string]$ApiReadyUrl = "http://localhost:8080/ready"
)

$ErrorActionPreference = "Stop"
$BeRoot = Split-Path $PSScriptRoot -Parent
Set-Location $BeRoot

$ts = Get-Date -Format "yyyyMMdd-HHmmss"
$LogDir = Join-Path $BeRoot "logs\backfill-us-$ts"
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null
$RunLog = Join-Path $LogDir "run.log"

function Add-RunLogLine {
    param([string]$Line)
    for ($try = 0; $try -lt 8; $try++) {
        try {
            Add-Content -Path $RunLog -Value $Line -ErrorAction Stop
            return
        } catch {
            Start-Sleep -Milliseconds 250
        }
    }
    Write-Host "WARN: could not append to run.log (file locked?): $Line"
}

function Write-Log {
    param([string]$Message)
    $line = "{0} {1}" -f (Get-Date -Format "yyyy-MM-dd HH:mm:ss"), $Message
    Write-Host $line
    Add-RunLogLine $line
}

function Test-ApiReady {
    try {
        $r = Invoke-RestMethod -Uri $ApiReadyUrl -TimeoutSec 5
        return ($r.status -eq "ready" -and $r.checks.mysql -eq "ok")
    } catch {
        return $false
    }
}

function Invoke-BackfillStep {
    param(
        [string[]]$BackfillArgs,
        [string]$StepName
    )
    Write-Log "=== $StepName ==="
    Write-Log ("cmd: backfill-daily-closes.exe " + ($BackfillArgs -join " "))

    $exePath = Join-Path $BeRoot "bin\backfill-daily-closes.exe"
    if (-not (Test-Path $exePath)) {
        Write-Log "building backfill-daily-closes.exe ..."
        go build -o bin/backfill-daily-closes.exe ./cmd/backfill-daily-closes
    }

    $stepLog = Join-Path $LogDir ("{0}.log" -f ($StepName -replace '[^\w\-]', '_'))
    $exePath = Join-Path $BeRoot "bin\backfill-daily-closes.exe"

    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        & $exePath @BackfillArgs 2>&1 | ForEach-Object {
            $line = if ($_ -is [System.Management.Automation.ErrorRecord]) { $_.ToString() } else { "$_" }
            Write-Host $line
            Add-RunLogLine $line
            Add-Content -Path $stepLog -Value $line
        }
    } finally {
        $ErrorActionPreference = $prevEap
    }

    if ($LASTEXITCODE -ne 0) {
        Write-Log "ERROR: $StepName exited $LASTEXITCODE - see $stepLog"
        throw "$StepName failed with exit $LASTEXITCODE"
    }
    Write-Log "OK: $StepName finished"
}

Write-Log "log directory: $LogDir"
Write-Log "checking /ready ..."
if (-not (Test-ApiReady)) {
    Write-Log "WARN: API mysql not ready - start MySQL80 before continuing"
}

Invoke-BackfillStep @("-status", "-market=$Market", "-since=$Since", "-timeout-min=5") "status-before"

$round = 0
while ($true) {
    $round++
    Invoke-BackfillStep @(
        "-skip-projection",
        "-market=$Market",
        "-since=$Since",
        "-fetch-history",
        "-fetch-limit=$FetchBatchSize",
        "-fetch-log-every=$FetchLogEvery",
        "-fetch-delay-ms=$FetchDelayMs",
        "-timeout-min=$FetchTimeoutMin"
    ) "fetch-history-round-$round"

    $statusOut = Join-Path $LogDir "status-round-$round.log"
    $statusExe = Join-Path $BeRoot "bin\backfill-daily-closes.exe"
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        $statusLine = (& $statusExe @("-status", "-market=$Market", "-since=$Since", "-timeout-min=5") 2>&1 | ForEach-Object {
            if ($_ -is [System.Management.Automation.ErrorRecord]) { $_.ToString() } else { "$_" }
        }) -join "`n"
    } finally {
        $ErrorActionPreference = $prevEap
    }
    Set-Content -Path $statusOut -Value $statusLine
    Add-RunLogLine $statusLine.Trim()
    Write-Log $statusLine.Trim()

    if ($statusLine -match "history_incomplete=0") {
        Write-Log "fetch-history complete (history_incomplete=0)"
        break
    }
    if ($round -ge 200) {
        Write-Log "WARN: stopping after 200 fetch rounds"
        break
    }
    Write-Log "pausing ${PauseSec}s before next fetch batch ..."
    Start-Sleep -Seconds $PauseSec
}

$months = @(
    @{ Since = "2026-01-01"; Until = "2026-01-31" },
    @{ Since = "2026-02-01"; Until = "2026-02-28" },
    @{ Since = "2026-03-01"; Until = "2026-03-31" },
    @{ Since = "2026-04-01"; Until = "2026-04-30" },
    @{ Since = "2026-05-01"; Until = "2026-05-31" },
    @{ Since = "2026-06-01"; Until = "2026-06-30" },
    @{ Since = "2026-07-01"; Until = "2026-07-31" },
    @{ Since = "2026-08-01"; Until = "2026-08-31" }
)

foreach ($m in $months) {
    Invoke-BackfillStep @(
        "-projection-only",
        "-market=$Market",
        "-since=$($m.Since)",
        "-until=$($m.Until)",
        "-timeout-min=$ProjectionTimeoutMin"
    ) "projection-$($m.Since)-$($m.Until)"

    Write-Log "pausing ${PauseSec}s after projection chunk ..."
    Start-Sleep -Seconds $PauseSec
}

Invoke-BackfillStep @("-status", "-market=$Market", "-since=$Since", "-timeout-min=5") "status-after"
Write-Log "all done - full log: $RunLog"
