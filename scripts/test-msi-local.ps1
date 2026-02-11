#!/usr/bin/env pwsh
# Build a local .msi installer for testing (no signing, no notarization)
# Replicates the goreleaser post-build hook logic from .goreleaser.windows.yaml
# Run on a Windows machine with WiX v4 and Go installed
#
# Usage:
#   .\scripts\test-msi-local.ps1
#   # Then to test install + welcome message:
#   msiexec /i dist\tnr-0.0.0-local-amd64.msi
#
# Prerequisites:
#   - Go 1.25+
#   - .NET SDK 6+ (required for WiX; https://aka.ms/dotnet/download)
#   - WiX v4: dotnet tool install --global wix --version 4.0.6
#     (If you installed wix without --version, uninstall first: dotnet tool uninstall --global wix)
#   - WiX extensions (UI + Util) are fetched automatically via NuGet; or run .\scripts\setup-wix-local.ps1 to set WIX_EXT_* env vars.

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$CliDir = Split-Path -Parent $ScriptDir
$Name = "tnr"
$Version = "0.0.0-local"
$Arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq "Arm64") { "arm64" } else { "amd64" }
$GoArch = $Arch

Write-Host "[BUILD] Building $Name binary for windows/$GoArch..."
Push-Location $CliDir
try {
    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = $GoArch
    go build -o "dist\$Name.exe" .\main.go
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Go build failed"
        exit 1
    }
} finally {
    Pop-Location
}

$BinPath = Join-Path $CliDir "dist\$Name.exe"
Write-Host "[OK] Binary: $BinPath"

# Create temp working directory
$TempDir = Join-Path $env:TEMP "msi-test-$Name-$Arch-$(Get-Random)"
New-Item -ItemType Directory -Path $TempDir -Force | Out-Null
Write-Host "[OK] Working dir: $TempDir"

try {
    # Copy binary
    Copy-Item $BinPath -Destination (Join-Path $TempDir "$Name.exe") -Force

    # Copy welcome.bat
    $WelcomeSrc = Join-Path $CliDir "packaging\windows\welcome.bat"
    Copy-Item $WelcomeSrc -Destination (Join-Path $TempDir "welcome.bat") -Force
    Write-Host "[OK] Copied welcome.bat"

    # Copy license
    $LicenseSrc = Join-Path $CliDir "packaging\windows\license.rtf"
    if (Test-Path $LicenseSrc) {
        Copy-Item $LicenseSrc -Destination (Join-Path $TempDir "license.rtf") -Force
    }

    # Process WiX template (replace GoReleaser variables)
    $WxsTemplate = Join-Path $CliDir "packaging\windows\app.wxs"
    $WxsContent = Get-Content $WxsTemplate -Raw
    $WxsContent = $WxsContent -replace '\{\{\s*\.ProjectName\s*\}\}', $Name
    $WxsContent = $WxsContent -replace '\{\{\s*\.Version\s*\}\}', $Version
    $WxsContent = $WxsContent -replace '\{\{\s*\.Binary\s*\}\}', $Name
    $WxsContent = $WxsContent -replace '\{\{\s*\.Arch\s*\}\}', $Arch
    $WxsContent | Set-Content -Path (Join-Path $TempDir "app.wxs") -Encoding UTF8
    Write-Host "[OK] Prepared WiX source"

    # Build MSI
    $OutputMsi = Join-Path $TempDir "$Name-$Version-$Arch.msi"

    # Resolve wix: PATH first, then dotnet tools dir (e.g. after: dotnet tool install --global wix)
    $WixExe = $null
    if (Get-Command wix -ErrorAction SilentlyContinue) {
        $WixExe = "wix"
    } else {
        $DotnetToolsWix = Join-Path $env:USERPROFILE ".dotnet\tools\wix.exe"
        if (Test-Path $DotnetToolsWix) {
            $WixExe = $DotnetToolsWix
        }
    }
    if (-not $WixExe) {
        $hasDotnet = Get-Command dotnet -ErrorAction SilentlyContinue
        $sdkHint = if (-not $hasDotnet) {
            "First install .NET SDK 6+ (https://aka.ms/dotnet/download), then in a new shell run:"
        } else {
            "Install WiX v4 (this repo requires v4, not v6) with:"
        }
        Write-Error @"
WiX v4 not found. $sdkHint

  dotnet tool uninstall --global wix   # if you already installed without version
  dotnet tool install --global wix --version 4.0.6

If you don't have .NET SDK, get it from https://aka.ms/dotnet/download. This script will use %USERPROFILE%\.dotnet\tools\wix.exe if that folder is not on your PATH.
"@
        exit 1
    }

    # Require WiX v4 (app.wxs uses v4 schema); v6 uses different schema and extensions
    $wixVersionOutput = & $WixExe --version 2>&1 | Out-String
    if ($wixVersionOutput -match "(\d+)\.\d+\.\d+") {
        $major = [int]$Matches[1]
        if ($major -ge 6) {
            Write-Error @"
This script requires WiX v4 (packaging uses v4 schema). Detected WiX v$major. Install v4 with:

  dotnet tool uninstall --global wix
  dotnet tool install --global wix --version 4.0.6
"@
            exit 1
        }
    }

    # Resolve extension DLLs: use env vars if set (e.g. after setup-wix-local.ps1), else fetch via NuGet (same as CI)
    $UiDll = $env:WIX_EXT_UI_DLL
    $UtilDll = $env:WIX_EXT_UTIL_DLL
    if (-not $UiDll -or -not (Test-Path $UiDll) -or -not $UtilDll -or -not (Test-Path $UtilDll)) {
        Write-Host "[WIX] Resolving extension DLLs via NuGet..."
        $nuget = Join-Path $env:TEMP "nuget.exe"
        if (-not (Test-Path $nuget)) {
            Invoke-WebRequest "https://dist.nuget.org/win-x86-commandline/v6.9.1/nuget.exe" -OutFile $nuget
        }
        $pkgsRoot = Join-Path $env:TEMP "wixexts-local"
        if (-not (Test-Path $pkgsRoot)) {
            New-Item -ItemType Directory -Path $pkgsRoot -Force | Out-Null
        }
        $uiPkg = Join-Path $pkgsRoot "WixToolset.UI.wixext"
        $utilPkg = Join-Path $pkgsRoot "WixToolset.Util.wixext"
        if (-not (Test-Path $uiPkg)) {
            & $nuget install WixToolset.UI.wixext -Version 4.0.6 -OutputDirectory $pkgsRoot -ExcludeVersion
        }
        if (-not (Test-Path $utilPkg)) {
            & $nuget install WixToolset.Util.wixext -Version 4.0.6 -OutputDirectory $pkgsRoot -ExcludeVersion
        }
        $UiDll = (Get-ChildItem $uiPkg -Recurse -Filter "WixToolset.UI.wixext.dll" | Select-Object -First 1).FullName
        $UtilDll = (Get-ChildItem $utilPkg -Recurse -Filter "WixToolset.Util.wixext.dll" | Select-Object -First 1).FullName
        if (-not $UiDll -or -not $UtilDll) {
            Write-Error "Could not locate WiX extension DLLs under $pkgsRoot"
            exit 1
        }
        Write-Host "[OK] UI: $UiDll"
        Write-Host "[OK] Util: $UtilDll"
    }

    Push-Location $TempDir
    try {
        & $WixExe build -ext $UiDll -ext $UtilDll -out $OutputMsi app.wxs
        if ($LASTEXITCODE -ne 0) {
            Write-Error "WiX build failed with exit code $LASTEXITCODE"
            exit 1
        }
    } finally {
        Pop-Location
    }

    if (-not (Test-Path $OutputMsi)) {
        Write-Error "MSI was not created"
        exit 1
    }

    # Copy to dist
    $DistDir = Join-Path $CliDir "dist"
    if (-not (Test-Path $DistDir)) {
        New-Item -ItemType Directory -Path $DistDir -Force | Out-Null
    }
    $FinalMsi = Join-Path $DistDir "$Name-$Version-$Arch.msi"
    Copy-Item $OutputMsi -Destination $FinalMsi -Force

    $Size = [math]::Round((Get-Item $FinalMsi).Length / 1MB, 2)
    Write-Host ""
    Write-Host "[SUCCESS] .msi built: $FinalMsi"
    Write-Host "  Size: $Size MB"
    Write-Host ""
    Write-Host "To test install (opens GUI):  msiexec /i `"$FinalMsi`""
    Write-Host "To silent install:            msiexec /i `"$FinalMsi`" /qn"
    Write-Host "To uninstall:                 msiexec /x `"$FinalMsi`""

} finally {
    Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
    Write-Host "[CLEANUP] Cleaned up temp directory"
}
