# Signs the provided artifact in-place using signtool and a YubiKey-backed certificate.
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$ArtifactPath,

    [Parameter(Mandatory = $false)]
    [string]$SignaturePath
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

$signArgs = @(
    'sign',
    '/sha1', $env:CERT_THUMBPRINT,
    '/fd', 'sha256',
    '/tr', $env:TIMESTAMP_SERVER,
    '/td', 'sha256',
    '/debug',
    '/v',
    $ArtifactPath
)

& $signtool.FullName @signArgs

if ($LASTEXITCODE -ne 0) {
    throw "signtool sign failed for $ArtifactPath"
}

Write-Host 'Signature verification' -ForegroundColor Gray
& $signtool.FullName verify /pa /v $ArtifactPath
if ($LASTEXITCODE -ne 0) {
    throw "signtool verify failed for $ArtifactPath"
}

if ($SignaturePath) {
    $signatureDir = Split-Path -Parent $SignaturePath
    if ($signatureDir -and -not (Test-Path -LiteralPath $signatureDir)) {
        New-Item -ItemType Directory -Path $signatureDir -Force | Out-Null
    }

    # Use signtool to emit a detached PKCS#7 signature so GoReleaser has a file to upload.
    $tempDir = Join-Path -Path $env:TEMP -ChildPath ("msi-signature-" + [guid]::NewGuid())
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

    try {
        $p7Args = @(
            'sign',
            '/sha1', $env:CERT_THUMBPRINT,
            '/fd', 'sha256',
            '/debug',
            '/v',
            '/p7', $tempDir,
            '/p7ce', 'DetachedSignedData',
            $ArtifactPath
        )

        & $signtool.FullName @p7Args
        if ($LASTEXITCODE -ne 0) {
            throw "signtool failed to emit PKCS#7 signature for $ArtifactPath"
        }

        $generated = Join-Path -Path $tempDir -ChildPath ((Split-Path -Leaf $ArtifactPath) + '.p7')
        if (-not (Test-Path -LiteralPath $generated)) {
            throw "Expected PKCS#7 file was not generated at $generated"
        }

        Move-Item -LiteralPath $generated -Destination $SignaturePath -Force
        Write-Host "Detached signature written to $SignaturePath" -ForegroundColor Gray
    }
    finally {
        Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Write-Host "Successfully signed: $ArtifactPath" -ForegroundColor Green
