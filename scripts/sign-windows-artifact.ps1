# Signs the provided artifact in-place using signtool and a YubiKey-backed certificate.
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$ArtifactPath
)

$ErrorActionPreference = 'Stop'

if (-not (Test-Path -LiteralPath $ArtifactPath)) {
    throw "Artifact not found: $ArtifactPath"
}

if (-not $env:CERT_THUMBPRINT) {
    throw 'CERT_THUMBPRINT environment variable is required'
}

if (-not $env:TIMESTAMP_SERVER) {
    throw 'TIMESTAMP_SERVER environment variable is required'
}

# Resolve signtool.exe from Windows SDK installations
$signtool = Get-ChildItem -Path "C:\Program Files (x86)\Windows Kits\10\bin\*\x64\signtool.exe" -ErrorAction SilentlyContinue | Select-Object -First 1
if (-not $signtool) {
    throw 'signtool.exe not found. Install the Windows 10 SDK on the runner.'
}

Write-Host "Signing artifact: $ArtifactPath" -ForegroundColor Cyan

& $signtool.FullName sign `
    /sha1 $env:CERT_THUMBPRINT `
    /fd sha256 `
    /tr $env:TIMESTAMP_SERVER `
    /td sha256 `
    /debug `
    /v `
    $ArtifactPath

if ($LASTEXITCODE -ne 0) {
    throw "signtool sign failed for $ArtifactPath"
}

Write-Host 'Signature verification' -ForegroundColor Gray
& $signtool.FullName verify /pa /v $ArtifactPath
if ($LASTEXITCODE -ne 0) {
    throw "signtool verify failed for $ArtifactPath"
}

Write-Host "Successfully signed: $ArtifactPath" -ForegroundColor Green
