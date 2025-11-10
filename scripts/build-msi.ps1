#!/usr/bin/env pwsh
# Build and sign Windows MSI installer using WiX v4 and YubiKey
# This script mirrors the macOS .pkg build process from .goreleaser.macos.yaml
# Called as a post-build hook by GoReleaser for each architecture

param(
    [Parameter(Mandatory=$true)]
    [string]$BinaryPath,
    
    [Parameter(Mandatory=$true)]
    [string]$Arch,
    
    [Parameter(Mandatory=$true)]
    [string]$Version,
    
    [Parameter(Mandatory=$true)]
    [string]$ProjectName
)

$ErrorActionPreference = "Stop"

Write-Host "🔨 Building MSI for $ProjectName $Version ($Arch)"

# Validate binary exists
if (-not (Test-Path $BinaryPath)) {
    Write-Error "Binary not found: $BinaryPath"
    exit 1
}

# Get absolute paths
$BinaryPath = Resolve-Path $BinaryPath
$RepoRoot = Split-Path -Parent $PSScriptRoot
$WxsTemplate = Join-Path $RepoRoot "packaging/windows/app.wxs"

if (-not (Test-Path $WxsTemplate)) {
    Write-Error "WiX template not found: $WxsTemplate"
    exit 1
}

# Create temporary working directory
$TempDir = Join-Path $env:TEMP "msi-build-$ProjectName-$Arch-$(Get-Random)"
New-Item -ItemType Directory -Path $TempDir -Force | Out-Null
Write-Host "Working directory: $TempDir"

try {
    # Copy binary to temp directory (WiX expects it in the working directory)
    $BinaryName = Split-Path -Leaf $BinaryPath
    $WorkingBinary = Join-Path $TempDir "$ProjectName.exe"
    Copy-Item $BinaryPath -Destination $WorkingBinary -Force
    Write-Host "✅ Copied binary to: $WorkingBinary"

    # Read and process WiX template (replace GoReleaser template variables)
    $WxsContent = Get-Content $WxsTemplate -Raw
    $WxsContent = $WxsContent -replace '\{\{\s*\.ProjectName\s*\}\}', $ProjectName
    $WxsContent = $WxsContent -replace '\{\{\s*\.Version\s*\}\}', $Version
    $WxsContent = $WxsContent -replace '\{\{\s*\.Binary\s*\}\}', $ProjectName
    $WxsContent = $WxsContent -replace '\{\{\s*\.Arch\s*\}\}', $Arch
    
    # Write processed WiX source
    $WorkingWxs = Join-Path $TempDir "app.wxs"
    $WxsContent | Set-Content -Path $WorkingWxs -Encoding UTF8
    Write-Host "✅ Prepared WiX source: $WorkingWxs"

    # Build MSI with WiX v4
    $OutputMsi = Join-Path $TempDir "$ProjectName-$Version-$Arch.msi"
    Write-Host "🔧 Building MSI with WiX..."
    
    Push-Location $TempDir
    try {
        wix build -out $OutputMsi app.wxs
        if ($LASTEXITCODE -ne 0) {
            Write-Error "WiX build failed with exit code $LASTEXITCODE"
            exit 1
        }
    } finally {
        Pop-Location
    }

    if (-not (Test-Path $OutputMsi)) {
        Write-Error "MSI was not created: $OutputMsi"
        exit 1
    }

    Write-Host "✅ MSI built successfully: $OutputMsi"

    # Sign the MSI with YubiKey (if credentials are available)
    if ($env:CERT_THUMBPRINT -and $env:TIMESTAMP_SERVER) {
        Write-Host "🔏 Signing MSI with YubiKey..."
        
        # Find signtool.exe
        $SignTool = Get-ChildItem -Path "C:\Program Files (x86)\Windows Kits\10\bin\*\x64\signtool.exe" -ErrorAction SilentlyContinue | 
                    Select-Object -First 1
        
        if (-not $SignTool) {
            Write-Error "signtool.exe not found. Please install Windows SDK."
            exit 1
        }

        Write-Host "Using signtool: $($SignTool.FullName)"

        # Sign with YubiKey certificate
        & $SignTool.FullName sign `
            /sha1 $env:CERT_THUMBPRINT `
            /fd sha256 `
            /tr $env:TIMESTAMP_SERVER `
            /td sha256 `
            /v `
            $OutputMsi

        if ($LASTEXITCODE -ne 0) {
            Write-Error "Failed to sign MSI (exit code: $LASTEXITCODE)"
            exit 1
        }

        Write-Host "✅ MSI signed successfully"

        # Verify the signature
        Write-Host "🔍 Verifying signature..."
        & $SignTool.FullName verify /pa /v $OutputMsi
        
        if ($LASTEXITCODE -ne 0) {
            Write-Warning "Signature verification failed (exit code: $LASTEXITCODE)"
        } else {
            Write-Host "✅ Signature verified"
        }
    } else {
        Write-Warning "⚠️  Skipping MSI signing (CERT_THUMBPRINT or TIMESTAMP_SERVER not set)"
    }

    # Copy signed MSI to dist directory
    $DistDir = Join-Path $RepoRoot "dist"
    if (-not (Test-Path $DistDir)) {
        New-Item -ItemType Directory -Path $DistDir -Force | Out-Null
    }

    $FinalMsi = Join-Path $DistDir "$ProjectName-$Version-$Arch.msi"
    Copy-Item $OutputMsi -Destination $FinalMsi -Force
    Write-Host "✅ Copied MSI to: $FinalMsi"

    # Display file info
    $FileInfo = Get-Item $FinalMsi
    Write-Host "📦 MSI Package:"
    Write-Host "   Path: $($FileInfo.FullName)"
    Write-Host "   Size: $([math]::Round($FileInfo.Length / 1MB, 2)) MB"
    Write-Host "   Modified: $($FileInfo.LastWriteTime)"

} finally {
    # Cleanup temp directory
    if (Test-Path $TempDir) {
        Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-Host "🧹 Cleaned up temporary directory"
    }
}

Write-Host "✨ MSI build complete!"
exit 0

