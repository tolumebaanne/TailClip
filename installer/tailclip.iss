[Setup]
AppName=TailClip Agent
AppVersion=1.0.0
DefaultDirName={autopf}\TailClip
DefaultGroupName=TailClip
UninstallDisplayIcon={app}\TailClipAgent.exe
Compression=lzma2
SolidCompression=yes
OutputDir=..\distribution
OutputBaseFilename=TailClip_Installer
PrivilegesRequired=admin
ArchitecturesAllowed=x64
ArchitecturesInstallIn64BitMode=x64
SetupIconFile=compiler:SetupClassicIcon.ico

[Files]
Source: "..\distribution\TailClipAgent.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "TailClipAgent.vbs"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{commonstartup}\TailClip Agent"; Filename: "{app}\TailClipAgent.vbs"; WorkingDir: "{app}"

[Run]
Filename: "{app}\TailClipAgent.vbs"; Description: "Start TailClip Agent now"; Flags: postinstall shellexec skipifsilent runhidden

[Code]
var
  ComponentPage: TWizardPage;
  AgentRadio: TRadioButton;
  HubRadio: TRadioButton;

  ConfigPage: TWizardPage;
  DeviceNameEdit: TEdit;
  AuthTokenEdit: TEdit;
  HubIPEdit: TEdit;
  HubPortEdit: TEdit;

procedure InitializeWizard;
var
  LblDeviceName, LblAuthToken, LblHubIP, LblHubPort: TLabel;
begin
  { --- Component Selection Page --- }
  ComponentPage := CreateCustomPage(wpWelcome, 'Select Component', 'Choose which component you want to install.');

  AgentRadio := TRadioButton.Create(ComponentPage);
  AgentRadio.Parent := ComponentPage.Surface;
  AgentRadio.Caption := 'TailClip Agent (Connects to a Hub to sync clipboard)';
  AgentRadio.Left := 0;
  AgentRadio.Top := 20;
  AgentRadio.Width := ComponentPage.SurfaceWidth;
  AgentRadio.Checked := True;

  HubRadio := TRadioButton.Create(ComponentPage);
  HubRadio.Parent := ComponentPage.Surface;
  HubRadio.Caption := 'TailClip Hub (Coming Soon)';
  HubRadio.Left := 0;
  HubRadio.Top := AgentRadio.Top + AgentRadio.Height + 10;
  HubRadio.Width := ComponentPage.SurfaceWidth;
  HubRadio.Enabled := False;

  { --- Configuration Page --- }
  ConfigPage := CreateCustomPage(ComponentPage.ID, 'Configuration', 'Enter settings for the TailClip Agent.');

  LblDeviceName := TLabel.Create(ConfigPage);
  LblDeviceName.Parent := ConfigPage.Surface;
  LblDeviceName.Caption := 'Device Name (e.g. Work-PC):';
  LblDeviceName.Left := 0;
  LblDeviceName.Top := 0;

  DeviceNameEdit := TEdit.Create(ConfigPage);
  DeviceNameEdit.Parent := ConfigPage.Surface;
  DeviceNameEdit.Left := 0;
  DeviceNameEdit.Top := LblDeviceName.Top + LblDeviceName.Height + 4;
  DeviceNameEdit.Width := ConfigPage.SurfaceWidth;

  LblAuthToken := TLabel.Create(ConfigPage);
  LblAuthToken.Parent := ConfigPage.Surface;
  LblAuthToken.Caption := 'Auth Token:';
  LblAuthToken.Left := 0;
  LblAuthToken.Top := DeviceNameEdit.Top + DeviceNameEdit.Height + 12;

  AuthTokenEdit := TEdit.Create(ConfigPage);
  AuthTokenEdit.Parent := ConfigPage.Surface;
  AuthTokenEdit.Left := 0;
  AuthTokenEdit.Top := LblAuthToken.Top + LblAuthToken.Height + 4;
  AuthTokenEdit.Width := ConfigPage.SurfaceWidth;

  LblHubIP := TLabel.Create(ConfigPage);
  LblHubIP.Parent := ConfigPage.Surface;
  LblHubIP.Caption := 'Hub IP Address (e.g. 100.x.x.x):';
  LblHubIP.Left := 0;
  LblHubIP.Top := AuthTokenEdit.Top + AuthTokenEdit.Height + 12;

  HubIPEdit := TEdit.Create(ConfigPage);
  HubIPEdit.Parent := ConfigPage.Surface;
  HubIPEdit.Left := 0;
  HubIPEdit.Top := LblHubIP.Top + LblHubIP.Height + 4;
  HubIPEdit.Width := ConfigPage.SurfaceWidth;

  LblHubPort := TLabel.Create(ConfigPage);
  LblHubPort.Parent := ConfigPage.Surface;
  LblHubPort.Caption := 'Hub Port:';
  LblHubPort.Left := 0;
  LblHubPort.Top := HubIPEdit.Top + HubIPEdit.Height + 12;

  HubPortEdit := TEdit.Create(ConfigPage);
  HubPortEdit.Parent := ConfigPage.Surface;
  HubPortEdit.Left := 0;
  HubPortEdit.Top := LblHubPort.Top + LblHubPort.Height + 4;
  HubPortEdit.Width := ConfigPage.SurfaceWidth;
  HubPortEdit.Text := '8080';
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  
  if CurPageID = ConfigPage.ID then
  begin
    if Trim(DeviceNameEdit.Text) = '' then
    begin
      MsgBox('Device Name cannot be empty.', mbError, MB_OK);
      Result := False;
    end
    else if Trim(AuthTokenEdit.Text) = '' then
    begin
      MsgBox('Auth Token cannot be empty.', mbError, MB_OK);
      Result := False;
    end
    else if Trim(HubIPEdit.Text) = '' then
    begin
      MsgBox('Hub IP Address cannot be empty.', mbError, MB_OK);
      Result := False;
    end
    else if Trim(HubPortEdit.Text) = '' then
    begin
      MsgBox('Hub Port cannot be empty.', mbError, MB_OK);
      Result := False;
    end;
  end;
end;

function NormalizeDeviceID(DeviceName: String): String;
var
  i: Integer;
  c: Char;
begin
  Result := '';
  for i := 1 to Length(DeviceName) do
  begin
    c := DeviceName[i];
    if (c >= 'A') and (c <= 'Z') then
      Result := Result + Chr(Ord(c) + 32)
    else if ((c >= 'a') and (c <= 'z')) or ((c >= '0') and (c <= '9')) then
      Result := Result + c
    else if c = ' ' then
      Result := Result + '-'
    else
      Result := Result + '-';
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  ConfigDir: String;
  ConfigFile: String;
  JsonContent: TArrayOfString;
  DeviceID: String;
  HubUrl: String;
begin
  if CurStep = ssPostInstall then
  begin
    ConfigDir := ExpandConstant('{localappdata}\TailClip');
    if not DirExists(ConfigDir) then
    begin
      CreateDir(ConfigDir);
    end;
    
    ConfigFile := ConfigDir + '\agent.config.json';
    DeviceID := NormalizeDeviceID(Trim(DeviceNameEdit.Text));
    HubUrl := 'http://' + Trim(HubIPEdit.Text) + ':' + Trim(HubPortEdit.Text);
    
    SetArrayLength(JsonContent, 18);
    JsonContent[0] := '{';
    JsonContent[1] := '    "_comments": {';
    JsonContent[2] := '        "device_id": "Unique identifier for this device. Use a short, descriptive slug.",';
    JsonContent[3] := '        "device_name": "Human-readable name shown in logs and notifications.",';
    JsonContent[4] := '        "hub_url": "Full URL to the TailClip hub server.",';
    JsonContent[5] := '        "auth_token": "Shared secret for authenticating with the hub.",';
    JsonContent[6] := '        "enabled": "Set to false to temporarily disable clipboard sync without removing the config.",';
    JsonContent[7] := '        "poll_interval_ms": "How often (in milliseconds) to check for local clipboard changes.",';
    JsonContent[8] := '        "notify_enabled": "Show desktop notifications when clipboard content is synced from another device."';
    JsonContent[9] := '    },';
    JsonContent[10] := '    "device_id": "' + DeviceID + '",';
    JsonContent[11] := '    "device_name": "' + Trim(DeviceNameEdit.Text) + '",';
    JsonContent[12] := '    "hub_url": "' + HubUrl + '",';
    JsonContent[13] := '    "auth_token": "' + Trim(AuthTokenEdit.Text) + '",';
    JsonContent[14] := '    "enabled": true,';
    JsonContent[15] := '    "poll_interval_ms": 1000,';
    JsonContent[16] := '    "notify_enabled": true';
    JsonContent[17] := '}';
    
    SaveStringsToUTF8File(ConfigFile, JsonContent, False);
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  ConfigDir: String;
begin
  if CurUninstallStep = usPostUninstall then
  begin
    ConfigDir := ExpandConstant('{localappdata}\TailClip');
    if DirExists(ConfigDir) then
    begin
      DelTree(ConfigDir, True, True, True);
    end;
  end;
end;
