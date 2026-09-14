' Launches the quant-port watchdog with no visible window.
'
' Why this file exists (2026-09-09):
'   The scheduled task used to run powershell.exe directly with
'   -WindowStyle Hidden. That is not enough. powershell.exe allocates a
'   console window FIRST and only then hides it, so a console flashes on
'   screen every run and steals keyboard focus. Every 5 minutes that is a
'   visible interruption while someone is using the machine.
'
'   wscript.exe has no console of its own. Running powershell from here with
'   intWindowStyle = 0 shows nothing at all -- no flash, no focus steal.
'
' Kept ASCII-only on purpose: wscript reads .vbs as ANSI, so non-ASCII text
' here would be mangled. The Korean commentary lives in watchdog.ps1.

Option Explicit

Dim shell, fso, scriptDir, ps1Path, cmd, rc

Set shell = CreateObject("WScript.Shell")
Set fso = CreateObject("Scripting.FileSystemObject")

scriptDir = fso.GetParentFolderName(WScript.ScriptFullName)
ps1Path = fso.BuildPath(scriptDir, "watchdog.ps1")

If Not fso.FileExists(ps1Path) Then
    ' Non-zero tells Task Scheduler something is wrong; it shows up as
    ' LastTaskResult instead of failing silently.
    WScript.Quit 2
End If

cmd = "powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File """ & ps1Path & """ -Quiet"

' 0 = hidden window. True = wait, so the task's duration and LastTaskResult
' reflect the watchdog itself, and MultipleInstances=IgnoreNew can actually
' prevent overlap while a slow recovery is in progress.
rc = shell.Run(cmd, 0, True)

WScript.Quit rc
