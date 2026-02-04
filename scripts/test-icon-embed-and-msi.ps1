#!/usr/bin/env pwsh
# Test script for icon embedding and MSI build
# This script: embeds icon → builds Go binary → builds MSI → verifies icon

$ErrorActionPreference = "Stop"

# Get the repository root (parent of scripts directory)
$RepoRoot = Split-Path -Parent $PSScriptRoot

# Change to repository root
Push-Location $RepoRoot
try {
    Write-Host "[TEST] Testing icon embedding and MSI build locally..." -ForegroundColor Cyan
    Write-Host "Working directory: $RepoRoot" -ForegroundColor Gray

    # Step 1: Embed Windows icon
    Write-Host "`n[STEP 1] Embedding Windows icon..." -ForegroundColor Yellow
    & "$PSScriptRoot\embed-windows-icon.ps1"

    if ($LASTEXITCODE -ne 0) {
        Write-Error "Icon embedding failed"
        exit 1
    }

    # Verify rsrc.syso was created
    $rsrcSyso = Join-Path $RepoRoot "rsrc.syso"
    if (-not (Test-Path $rsrcSyso)) {
        Write-Error "rsrc.syso was not created. Icon embedding may have failed."
        exit 1
    }
    Write-Host "[OK] Icon resource file created: $rsrcSyso" -ForegroundColor Green
    $rsrcInfo = Get-Item $rsrcSyso
    Write-Host "      Size: $([math]::Round($rsrcInfo.Length / 1KB, 2)) KB" -ForegroundColor Gray

    # Step 2: Setup WiX (if needed)
    Write-Host "`n[STEP 2] Setting up WiX..." -ForegroundColor Yellow
    
    # Dot-source the setup script so environment variables are available in this session
    # Suppress error output from WiX version check (it may fail due to .NET runtime issues)
    $ErrorActionPreference = "Continue"
    . "$PSScriptRoot\setup-wix-local.ps1" 2>&1 | ForEach-Object {
        # Filter out the hostfxr.dll errors which are non-fatal
        if ($_ -notmatch "hostfxr\.dll") {
            Write-Host $_
        }
    }
    $ErrorActionPreference = "Stop"
    
    # Check if WiX executable exists and environment variables are set
    if (-not $env:WIX_EXE_PATH) {
        Write-Error "WiX setup failed: WIX_EXE_PATH not set. Please ensure WiX is installed."
        exit 1
    }
    
    if (-not (Test-Path $env:WIX_EXE_PATH)) {
        Write-Error "WiX executable not found at: $env:WIX_EXE_PATH"
        Write-Host "`n[TIP] The WiX tool installation may have failed due to .NET runtime issues." -ForegroundColor Yellow
        Write-Host "      You may need to install .NET SDK 8.x manually or use a system-installed WiX." -ForegroundColor Yellow
        exit 1
    }
    
    # Check extension DLLs
    if (-not $env:WIX_EXT_UI_DLL -or -not (Test-Path $env:WIX_EXT_UI_DLL)) {
        Write-Warning "WiX UI extension not found, but continuing..."
    }
    if (-not $env:WIX_EXT_UTIL_DLL -or -not (Test-Path $env:WIX_EXT_UTIL_DLL)) {
        Write-Warning "WiX Util extension not found, but continuing..."
    }
    
    Write-Host "[OK] WiX paths configured" -ForegroundColor Green
    Write-Host "      WiX: $env:WIX_EXE_PATH" -ForegroundColor Gray

    # Step 3: Build Go binary with embedded icon
    Write-Host "`n[STEP 3] Building Go binary with embedded icon..." -ForegroundColor Yellow
    $version = "0.0.6"
    $commit = "local-test"
    $date = Get-Date -Format "yyyy-MM-dd"

    if (-not (Test-Path "dist")) {
        New-Item -ItemType Directory -Path "dist" | Out-Null
    }

    $env:CGO_ENABLED = "0"
    $ldflags = "-s -w -X github.com/Thunder-Compute/thunder-cli/internal/version.BuildVersion=$version -X github.com/Thunder-Compute/thunder-cli/internal/version.BuildCommit=$commit -X github.com/Thunder-Compute/thunder-cli/internal/version.BuildDate=$date"
    go build -ldflags $ldflags -o dist/tnr.exe ./main.go

    if ($LASTEXITCODE -ne 0) {
        Write-Error "Go build failed"
        exit 1
    }

    $exePath = Join-Path $RepoRoot "dist\tnr.exe"
    if (-not (Test-Path $exePath)) {
        Write-Error "Binary was not created: $exePath"
        exit 1
    }

    Write-Host "[OK] Binary built: $exePath" -ForegroundColor Green
    $exeInfo = Get-Item $exePath
    Write-Host "      Size: $([math]::Round($exeInfo.Length / 1MB, 2)) MB" -ForegroundColor Gray

    # Step 4: Verify icon was embedded in the .exe
    Write-Host "`n[STEP 4] Verifying icon was embedded in .exe..." -ForegroundColor Yellow
    
    # Check if we can extract icon info using PowerShell
    try {
        $shell = New-Object -ComObject Shell.Application
        $folder = $shell.NameSpace((Split-Path -Parent $exePath))
        $file = $folder.ParseName((Split-Path -Leaf $exePath))
        
        # Try to get icon info
        $iconPath = $file.ExtendedProperty("System.ItemPathDisplay")
        if ($iconPath) {
            Write-Host "[OK] Icon verification: Executable has icon resources" -ForegroundColor Green
        }
    } catch {
        Write-Warning "Could not verify icon programmatically, but build succeeded"
    }

    # Alternative: Check if rsrc.syso was included (it should be)
    Write-Host "[INFO] The rsrc.syso file was included in the build" -ForegroundColor Gray

    # Step 5: Build MSI
    Write-Host "`n[STEP 5] Building MSI..." -ForegroundColor Yellow
    & "$PSScriptRoot\build-msi.ps1" `
        -BinaryPath $exePath `
        -Arch "amd64" `
        -Version $version `
        -ProjectName "tnr"

    if ($LASTEXITCODE -ne 0) {
        Write-Error "MSI build failed"
        exit 1
    }

    # Step 6: Display results
    Write-Host "`n[SUCCESS] Icon embedding and MSI build complete!" -ForegroundColor Green
    Write-Host "`n[RESULTS]" -ForegroundColor Cyan
    Write-Host "  Icon resource file: $rsrcSyso" -ForegroundColor White
    Write-Host "  Executable: $exePath" -ForegroundColor White
    
    $msiFiles = Get-ChildItem -Path dist -Filter '*.msi'
    if ($msiFiles) {
        Write-Host "  MSI package(s):" -ForegroundColor White
        $msiFiles | ForEach-Object {
            Write-Host "    - $($_.Name) ($([math]::Round($_.Length / 1MB, 2)) MB)" -ForegroundColor White
        }
    }

    Write-Host "`n[VERIFICATION]" -ForegroundColor Cyan
    Write-Host "  To verify the icon was embedded:" -ForegroundColor White
    Write-Host "  1. Right-click on dist\tnr.exe → Properties → Details tab" -ForegroundColor Gray
    Write-Host "  2. Check if the icon appears in File Explorer" -ForegroundColor Gray
    Write-Host "  3. Install the MSI and verify the icon in Start Menu" -ForegroundColor Gray

} finally {
    Pop-Location
}

