$ErrorActionPreference = 'Stop'

# Install tnr by fetching the latest release from GitHub and installing to %LOCALAPPDATA%\tnr\bin

$version = $env:TNR_VERSION
$latestUrl = $env:TNR_LATEST_URL
$githubApi = "https://api.github.com/repos/Thunder-Compute/thunder-cli/releases/latest"

$installDir = Join-Path $env:LOCALAPPDATA 'tnr/bin'
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

$arch = $env:PROCESSOR_ARCHITECTURE
if ($arch -eq 'ARM64') { $arch = 'arm64' } else { $arch = 'amd64' }

if ($latestUrl) {
  # Custom manifest path (for enterprise mirrors)
  Write-Host "Fetching manifest: $latestUrl"
  $manifest = Invoke-RestMethod -UseBasicParsing -Uri $latestUrl
  if (-not $version) { $version = $manifest.version }

  $assetKey = "windows/$arch"
  $url = $manifest.assets.$($assetKey)
  $checksums = $manifest.assets.checksums
} elseif ($version) {
  # Version explicitly specified - construct URLs directly
  $version = $version -replace '^v', ''
  $tag = "v$version"
  $url = "https://github.com/Thunder-Compute/thunder-cli/releases/download/$tag/tnr_${version}_windows_${arch}.zip"
  $checksums = "https://github.com/Thunder-Compute/thunder-cli/releases/download/$tag/checksums-windows.txt"
} else {
  # Default: fetch latest release from GitHub Releases API
  Write-Host "Fetching latest release from GitHub..."
  $release = Invoke-RestMethod -UseBasicParsing -Uri $githubApi -Headers @{ Accept = "application/vnd.github+json" }
  $tag = $release.tag_name
  $version = $tag -replace '^v', ''
  $url = "https://github.com/Thunder-Compute/thunder-cli/releases/download/$tag/tnr_${version}_windows_${arch}.zip"
  $checksums = "https://github.com/Thunder-Compute/thunder-cli/releases/download/$tag/checksums-windows.txt"
}

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
