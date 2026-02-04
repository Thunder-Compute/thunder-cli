#!/usr/bin/env pwsh
# Local MSI build test script
# Replicates the GitHub Actions workflow locally
# This script: sets up WiX → builds Go binary → builds MSI

$ErrorActionPreference = "Stop"

# Get the repository root (parent of scripts directory)
$RepoRoot = Split-Path -Parent $PSScriptRoot

# Change to repository root
Push-Location $RepoRoot
try {
    Write-Host "[TEST] Testing MSI build locally..." -ForegroundColor Cyan
    Write-Host "Working directory: $RepoRoot" -ForegroundColor Gray

    # Step 1: Setup WiX (same as GitHub Actions)
    Write-Host "`n[STEP 1] Setting up WiX..." -ForegroundColor Yellow
    & "$PSScriptRoot\setup-wix-local.ps1"

    if ($LASTEXITCODE -ne 0) {
        Write-Error "WiX setup failed"
        exit 1
    }

    # Step 2: Build Go binary
    Write-Host "`n[STEP 2] Building Go binary..." -ForegroundColor Yellow
    $version = "0.0.6"
    $commit = "local"
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

    Write-Host "[SUCCESS] Binary built: dist/tnr.exe"

    # Step 3: Build MSI (uses existing script)
    Write-Host "`n[STEP 3] Building MSI..." -ForegroundColor Yellow
    & "$PSScriptRoot\build-msi.ps1" `
        -BinaryPath "$RepoRoot\dist\tnr.exe" `
        -Arch "amd64" `
        -Version $version `
        -ProjectName "tnr"

    if ($LASTEXITCODE -ne 0) {
        Write-Error "MSI build failed"
        exit 1
    }

    Write-Host "`n[SUCCESS] MSI build complete! Check dist/ for the MSI file" -ForegroundColor Green
    Get-ChildItem -Path dist -Filter '*.msi' | Format-Table -Property Name, Length, LastWriteTime
} finally {
    Pop-Location
}
