#!/usr/bin/env pwsh
# Embed Windows icon into executable using rsrc tool
# Generates rsrc.syso file that Go automatically includes when building

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$iconPath = Join-Path $repoRoot "packaging/icons/tnr.ico"

if (-not (Test-Path $iconPath)) {
    Write-Warning "Icon file not found at $iconPath. Skipping icon embedding."
    exit 0
}

$rsrcPath = $null
$rsrc = Get-Command rsrc -ErrorAction SilentlyContinue
if ($rsrc) {
    $rsrcPath = $rsrc.Source
} else {
    Write-Host "Installing rsrc tool..." -ForegroundColor Yellow
    go install github.com/akavel/rsrc@latest
    if ($LASTEXITCODE -ne 0) {
        Write-Warning "Failed to install rsrc tool. Skipping icon embedding."
        exit 0
    }
    
    $gopath = go env GOPATH
    $rsrcPath = Join-Path $gopath "bin/rsrc.exe"
    if (-not (Test-Path $rsrcPath)) {
        Write-Warning "rsrc tool not found after installation. Skipping icon embedding."
        exit 0
    }
}

$tempDir = Join-Path $env:TEMP "rsrc-$(Get-Random)"
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

try {
    $manifestPath = Join-Path $tempDir "app.manifest"
    $manifestContent = @"
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity
    version="1.0.0.0"
    processorArchitecture="*"
    name="tnr"
    type="win32"
  />
  <description>tnr</description>
</assembly>
"@
    [System.IO.File]::WriteAllText($manifestPath, $manifestContent, [System.Text.UTF8Encoding]::new($false))

    $sysoPath = Join-Path $tempDir "rsrc.syso"
    
    Push-Location $tempDir
    try {
        & $rsrcPath -ico $iconPath -manifest $manifestPath -o $sysoPath
        if ($LASTEXITCODE -ne 0) {
            Write-Warning "rsrc failed to generate resource file. Skipping icon embedding."
            exit 0
        }
    } finally {
        Pop-Location
    }

    $sysoDest = Join-Path $repoRoot "rsrc.syso"
    Copy-Item $sysoPath -Destination $sysoDest -Force
    Write-Host "[OK] Icon resource file generated: $sysoDest" -ForegroundColor Green
    
} finally {
    if (Test-Path $tempDir) {
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Write-Host "[SUCCESS] Icon embedding setup complete" -ForegroundColor Green
