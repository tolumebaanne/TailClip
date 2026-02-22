Set objFSO = CreateObject("Scripting.FileSystemObject")
strScriptDir = objFSO.GetParentFolderName(WScript.ScriptFullName)
Set WshShell = CreateObject("WScript.Shell")
strProgramData = WshShell.ExpandEnvironmentStrings("%PROGRAMDATA%")
strConfigFile = strProgramData & "\TailClip\agent.config.json"
WshShell.Run Chr(34) & strScriptDir & "\TailClipAgent.exe" & Chr(34) & " " & Chr(34) & strConfigFile & Chr(34), 0
Set objFSO = Nothing
Set WshShell = Nothing
