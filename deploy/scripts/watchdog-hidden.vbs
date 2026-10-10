' Launches the quant-port watchdog, and usually answers "is the stack up?"
' without starting PowerShell at all.
'
' Why this file exists (2026-09-09):
'   The scheduled task used to run powershell.exe directly with
'   -WindowStyle Hidden. That is not enough. powershell.exe allocates a
'   console window FIRST and only then hides it, so a console flashed on
'   screen every run and stole keyboard focus. wscript.exe has no console of
'   its own, so running powershell from here with intWindowStyle = 0 shows
'   nothing at all -- no flash, no focus steal.
'
' Why it grew a fast path (2026-09-15):
'   Measured, a plain round cost ~1.1s: two process creations (wscript ->
'   powershell, the latter a cold .NET start plus an AMSI scan of a 20KB
'   script) and a `docker ps` that wakes the Docker backend and the WSL2 VM.
'   Twelve of those an hour is a periodic disk and memory burst, and the user
'   felt it as stutter while gaming (2026-09-15 report).
'
'   But the question that matters most of the time -- "is the stack
'   answering?" -- needs none of it. ServerXMLHTTP runs in this process, so
'   the happy path is one small process and about 50ms. PowerShell is started
'   ONLY when it has something to do:
'     * /health did not answer 200 -- something is actually wrong, or
'     * the deep check is due -- the `docker ps` sweep for stopped containers
'       and the /ready dependency probe, every DEEP_CHECK_MINUTES.
'
'   So the cheap check keeps its short cadence while the expensive one runs
'   twice an hour. Recovery latency is unchanged: a down stack fails /health
'   here and PowerShell starts on that very round.
'
' Kept ASCII-only on purpose: wscript reads .vbs as ANSI, so non-ASCII text
' here would be mangled. The Korean commentary lives in watchdog.ps1.

Option Explicit

' How often the full PowerShell check runs. Between those, only /health.
Const DEEP_CHECK_MINUTES = 30

' The same door the users go through (nginx :80). Hitting api directly would
' report "fine" while nginx is dead -- see Test-StackHealth in watchdog.ps1.
Const HEALTH_URL = "http://127.0.0.1/health"

Dim shell, fso
Dim scriptDir, deployRoot, ps1Path, pausePath, fastStamp, deepStamp
Dim isHealthy, needDeep, cmd, rc

Set shell = CreateObject("WScript.Shell")
Set fso = CreateObject("Scripting.FileSystemObject")

scriptDir  = fso.GetParentFolderName(WScript.ScriptFullName)
deployRoot = fso.GetParentFolderName(scriptDir)
ps1Path    = fso.BuildPath(scriptDir, "watchdog.ps1")
pausePath  = fso.BuildPath(deployRoot, "watchdog.paused")
fastStamp  = fso.BuildPath(deployRoot, "logs\watchdog-fast.stamp")
deepStamp  = fso.BuildPath(deployRoot, "logs\watchdog-deep.stamp")

' Paused on purpose -- do not even probe. Checked here rather than only in
' watchdog.ps1 so that a paused watchdog costs nothing at all.
If PauseActive() Then WScript.Quit 0

isHealthy = HealthOk()
needDeep = DeepDue()

If isHealthy And Not needDeep Then
    ' Nothing to do. Leave a mark so the fast path is still observable --
    ' the log itself stays PowerShell's, in Korean and UTF-8.
    Touch fastStamp, "healthy " & NowStamp()
    WScript.Quit 0
End If

If Not fso.FileExists(ps1Path) Then
    ' Non-zero tells Task Scheduler something is wrong; it shows up as
    ' LastTaskResult instead of failing silently.
    WScript.Quit 2
End If

' Stamp before the run, not after: a failing stack makes every round take the
' slow path anyway (health is false), and stamping after would let a hung or
' killed PowerShell keep the deep check permanently "due".
Touch deepStamp, "deep " & NowStamp()

cmd = "powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File """ & ps1Path & """ -Quiet"

' 0 = hidden window. True = wait, so the task's duration and LastTaskResult
' reflect the watchdog itself, and MultipleInstances=IgnoreNew can actually
' prevent overlap while a slow recovery is in progress.
rc = shell.Run(cmd, 0, True)

WScript.Quit rc

' ---------------------------------------------------------------------------

' yyyyMMddHHmmss -- fixed width on purpose, so that comparing the strings
' compares the times. VBScript's CDate follows the machine locale and cannot
' be trusted to read a timestamp back; watchdog.ps1 writes this same shape.
Function NowStamp()
    Dim d
    d = Now
    NowStamp = Year(d) & Pad(Month(d)) & Pad(Day(d)) & _
               Pad(Hour(d)) & Pad(Minute(d)) & Pad(Second(d))
End Function

Function Pad(n)
    Pad = Right("0" & n, 2)
End Function

' True while the user has deliberately parked the watchdog. An "until=" line
' that has passed is not a pause -- the file is removed and the round goes on,
' so a forgotten pause cannot silently disable the watchdog for weeks.
Function PauseActive()
    Dim ts, line, untilStamp, pos

    PauseActive = False
    If Not fso.FileExists(pausePath) Then Exit Function

    untilStamp = ""
    On Error Resume Next
    Set ts = fso.OpenTextFile(pausePath, 1)
    If Err.Number <> 0 Then
        ' Unreadable pause file: honour it. Erring towards "paused" only
        ' delays recovery; erring the other way restarts a stack the user
        ' just stopped.
        Err.Clear
        On Error GoTo 0
        PauseActive = True
        Exit Function
    End If
    Do Until ts.AtEndOfStream
        ' InStr rather than Left(): the file is UTF-8 with a BOM so that it
        ' reads correctly in editors and in PowerShell, and this reads it as
        ' ANSI. Anchoring on the substring means a BOM -- or any other junk
        ' ahead of the key -- cannot hide the value. '#' lines are comments.
        line = LCase(Trim(ts.ReadLine))
        If Left(line, 1) <> "#" Then
            pos = InStr(line, "until=")
            If pos > 0 Then untilStamp = Trim(Mid(line, pos + 6))
        End If
    Loop
    ts.Close
    On Error GoTo 0

    If untilStamp = "" Or LCase(untilStamp) = "forever" Then
        PauseActive = True
        Exit Function
    End If

    If untilStamp > NowStamp() Then
        PauseActive = True
        Exit Function
    End If

    On Error Resume Next
    fso.DeleteFile pausePath, True
    On Error GoTo 0
End Function

' 200 from /health. Anything else -- non-200, timeout, no ServerXMLHTTP at all
' -- returns False, which only means "let watchdog.ps1 decide". The fallback
' is the old behaviour, never a missed problem.
Function HealthOk()
    Dim http

    HealthOk = False
    On Error Resume Next
    Set http = CreateObject("MSXML2.ServerXMLHTTP.6.0")
    If Err.Number <> 0 Then Exit Function

    ' resolve, connect, send, receive (ms). Without these a stuck socket
    ' would hold the task open until the 15 minute execution limit.
    http.setTimeouts 2000, 3000, 3000, 5000
    ' 1 = SXH_PROXY_SET_DIRECT. Loopback needs no proxy, and skipping the
    ' lookup avoids a WPAD round trip on every single round.
    http.setProxy 1
    http.open "GET", HEALTH_URL, False
    http.send
    If Err.Number = 0 Then HealthOk = (http.status = 200)
    On Error GoTo 0
End Function

' Modification time is the record -- nothing to parse, and the touch below is
' one small write.
Function DeepDue()
    Dim f
    DeepDue = True
    If Not fso.FileExists(deepStamp) Then Exit Function
    On Error Resume Next
    Set f = fso.GetFile(deepStamp)
    If Err.Number <> 0 Then Exit Function
    DeepDue = (DateDiff("n", f.DateLastModified, Now) >= DEEP_CHECK_MINUTES)
    On Error GoTo 0
End Function

Sub Touch(path, text)
    Dim ts
    On Error Resume Next
    Set ts = fso.CreateTextFile(path, True)
    If Err.Number <> 0 Then Exit Sub
    ts.WriteLine text
    ts.Close
    On Error GoTo 0
End Sub
