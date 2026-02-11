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
#   - WiX v4 (dotnet tool install --global wix)
#   - WiX UI extension (wix extension add WixToolset.UI.wixext)

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

    # Try to find wix.exe
    $WixExe = "wix"
    Push-Location $TempDir
    try {
        & $WixExe build -ext WixToolset.UI.wixext -out $OutputMsi app.wxs
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
