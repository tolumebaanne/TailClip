Set objFSO = CreateObject("Scripting.FileSystemObject")
strScriptDir = objFSO.GetParentFolderName(WScript.ScriptFullName)
Set WshShell = CreateObject("WScript.Shell")
WshShell.Run Chr(34) & strScriptDir & "\agent.exe" & Chr(34), 0
Set fso = Nothing
Set WshShell = Nothing
