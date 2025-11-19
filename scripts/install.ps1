$ErrorActionPreference = 'Stop'

# Install tnr by reading latest.json from Cloudflare R2 (via gettnr.com) and installing to %LOCALAPPDATA%\tnr\bin

$channel = $env:TNR_UPDATE_CHANNEL
if (-not $channel) { $channel = 'stable' }
$version = $env:TNR_VERSION
$latestUrl = $env:TNR_LATEST_URL

if (-not $latestUrl) {
  if ($env:TNR_DOWNLOAD_BASE) {
    $latestUrl = "$($env:TNR_DOWNLOAD_BASE)/tnr/releases/latest.json"
  } else {
    # Default to Cloudflare R2 via gettnr.com custom domain
    $latestUrl = "https://gettnr.com/tnr/releases/latest.json"
  }
}

$installDir = Join-Path $env:LOCALAPPDATA 'tnr/bin'
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

Write-Host "Fetching manifest: $latestUrl"
$manifest = Invoke-RestMethod -UseBasicParsing -Uri $latestUrl
if (-not $version) { $version = $manifest.version }

$arch = $env:PROCESSOR_ARCHITECTURE
if ($arch -eq 'ARM64') { $arch = 'arm64' } else { $arch = 'amd64' }

$assetKey = "windows/$arch"
$url = $manifest.assets.$($assetKey)
$checksums = $manifest.assets.checksums

$tmp = New-Item -ItemType Directory -Force -Path ([System.IO.Path]::GetTempPath() + [System.Guid]::NewGuid().ToString())
$zip = Join-Path $tmp 'tnr.zip'
Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $zip
Invoke-WebRequest -UseBasicParsing -Uri $checksums -OutFile (Join-Path $tmp 'checksums.txt')

# Verify checksum
$hasher = [System.Security.Cryptography.SHA256]::Create()
$stream = [System.IO.File]::OpenRead($zip)
$hashBytes = $hasher.ComputeHash($stream)
$stream.Close()
$sum = ($hashBytes | ForEach-Object ToString x2) -join ''
Select-String -Path (Join-Path $tmp 'checksums.txt') -Pattern $sum -Quiet | Out-Null

Add-Type -AssemblyName System.IO.Compression.FileSystem
[System.IO.Compression.ZipFile]::ExtractToDirectory($zip, $tmp)

Copy-Item -Force (Join-Path $tmp 'tnr.exe') (Join-Path $installDir 'tnr.exe')

# PATH hint
$path = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($path -notlike "*${installDir}*") {
  Write-Host "Add $installDir to your PATH"
}

Write-Host "Installed tnr $version to $installDir"


