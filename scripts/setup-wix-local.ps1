#!/usr/bin/env pwsh
# Setup WiX v4 for local development
# Replicates the GitHub Actions WiX installation process
# This script sets up WiX v4.0.6 as a .NET tool and downloads required extensions

$ErrorActionPreference = "Stop"

Write-Host "[SETUP] Setting up WiX v4 for local development..."

# Install .NET SDK 8.x (if not already installed)
$dotnetInstallDir = Join-Path $env:TEMP "dotnet-local"
if (-not (Test-Path "$dotnetInstallDir\dotnet.exe")) {
    Write-Host "[INSTALL] Installing .NET SDK 8.x..."
    Invoke-WebRequest "https://dot.net/v1/dotnet-install.ps1" -OutFile "$env:TEMP\dotnet-install.ps1"
    & "$env:TEMP\dotnet-install.ps1" -Channel 8.0 -InstallDir $dotnetInstallDir
} else {
    Write-Host "[SUCCESS] .NET SDK already installed"
}

# Add .NET to PATH for this session
$env:PATH = "$dotnetInstallDir;$env:PATH"
& "$dotnetInstallDir\dotnet.exe" --version

# Install WiX v4 as a .NET tool
$wixDir = Join-Path $env:TEMP "wix-local"
Write-Host "[INSTALL] Installing WiX v4.0.6..."
& "$dotnetInstallDir\dotnet.exe" tool install wix --version 4.0.6 --tool-path $wixDir

# Add WiX to PATH for this session
$env:PATH = "$wixDir;$env:PATH"
$env:WIX_EXE_PATH = "$wixDir\wix.exe"

# Download WiX extension DLLs via NuGet
Write-Host "[DOWNLOAD] Downloading WiX extensions..."
$nuget = Join-Path $env:TEMP "nuget.exe"
if (-not (Test-Path $nuget)) {
    Invoke-WebRequest "https://dist.nuget.org/win-x86-commandline/v6.9.1/nuget.exe" -OutFile $nuget
}

$pkgsRoot = Join-Path $env:TEMP "wixexts-local"
& $nuget install WixToolset.UI.wixext -Version 4.0.6 -OutputDirectory $pkgsRoot -ExcludeVersion
& $nuget install WixToolset.Util.wixext -Version 4.0.6 -OutputDirectory $pkgsRoot -ExcludeVersion

# Locate extension DLLs
$uiDll = Get-ChildItem "$pkgsRoot\WixToolset.UI.wixext" -Recurse -Filter WixToolset.UI.wixext.dll | Select-Object -First 1
$utilDll = Get-ChildItem "$pkgsRoot\WixToolset.Util.wixext" -Recurse -Filter WixToolset.Util.wixext.dll | Select-Object -First 1

$env:WIX_EXT_UI_DLL = $uiDll.FullName
$env:WIX_EXT_UTIL_DLL = $utilDll.FullName

Write-Host "[SUCCESS] WiX setup complete!"
Write-Host "   WiX Path: $env:WIX_EXE_PATH"
Write-Host "   UI Extension: $env:WIX_EXT_UI_DLL"
Write-Host "   Util Extension: $env:WIX_EXT_UTIL_DLL"
& "$wixDir\wix.exe" --version

Write-Host ""
Write-Host "[TIP] To use WiX in this session, run:"
Write-Host "   . .\scripts\setup-wix-local.ps1"
Write-Host ""
Write-Host "   Or run your build script in the same PowerShell session"

