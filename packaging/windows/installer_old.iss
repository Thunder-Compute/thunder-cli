; Inno Setup script to install tnr into %LocalAppData%\tnr\bin and add to PATH

#define MyAppName "tnr"
#define MyAppVersion "1.0.0"
#define MyAppPublisher "Thunder"
#define MyAppURL "https://github.com/Thunder-Compute/thunder-cli"

[Setup]
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
DefaultDirName={localappdata}\tnr\bin
DisableDirPage=yes
DefaultGroupName=tnr
OutputBaseFilename=tnr-setup
Compression=lzma
SolidCompression=yes
WizardStyle=modern

[Files]
Source: "..\\..\\dist\\tnr_windows_amd64\\tnr.exe"; DestDir: "{app}"; Flags: ignoreversion

[Registry]
; Add install dir to user PATH if not already present
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; \
  ValueData: "{olddata};{app}"; Tasks: addpath

[Tasks]
Name: "addpath"; Description: "Add to PATH for current user"; Flags: unchecked

[Run]
Filename: "{app}\\tnr.exe"; Description: "Launch tnr"; Flags: postinstall nowait skipifsilent


