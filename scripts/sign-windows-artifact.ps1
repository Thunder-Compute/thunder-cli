[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$ArtifactPath,
    [string]$SignaturePath
)

$ErrorActionPreference = 'Stop'

if (-not (Test-Path -LiteralPath $ArtifactPath)) {
    throw "Artifact not found: $ArtifactPath"
}

foreach ($name in 'AZURE_DLIB_PATH', 'AZURE_METADATA_PATH') {
    if (-not [Environment]::GetEnvironmentVariable($name)) {
        throw "$name environment variable is required"
    }
}

$signtool = Get-ChildItem -Path "C:\Program Files (x86)\Windows Kits\10\bin\*\x64\signtool.exe" -ErrorAction SilentlyContinue | Select-Object -First 1
if (-not $signtool) {
    throw 'signtool.exe not found. Install the Windows 10 SDK on the runner.'
}

function Invoke-SignTool {
    param([string[]]$Arguments, [string]$ErrorMessage)
    & $signtool.FullName @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw $ErrorMessage
    }
}

Write-Host "Signing artifact: $ArtifactPath" -ForegroundColor Cyan

Invoke-SignTool -Arguments @(
    'sign',
    '/v', '/debug',
    '/fd', 'SHA256',
    '/tr', 'http://timestamp.acs.microsoft.com',
    '/td', 'SHA256',
    '/dlib', $env:AZURE_DLIB_PATH,
    '/dmdf', $env:AZURE_METADATA_PATH,
    $ArtifactPath
) -ErrorMessage "signtool sign failed for $ArtifactPath"

Invoke-SignTool -Arguments @('verify', '/pa', '/v', $ArtifactPath) -ErrorMessage "signtool verify failed for $ArtifactPath"

if ($SignaturePath) {
    $sigDir = Split-Path -Parent $SignaturePath
    if ($sigDir -and -not (Test-Path -LiteralPath $sigDir)) {
        New-Item -ItemType Directory -Path $sigDir -Force | Out-Null
    }

    $p7Dir = if ($sigDir) { $sigDir } else { '.' }
    $p7Name = (Split-Path -Leaf $ArtifactPath) + '.p7'
    $p7Path = Join-Path -Path $p7Dir -ChildPath $p7Name

    Invoke-SignTool -Arguments @(
        'sign',
        '/v', '/debug',
        '/fd', 'SHA256',
        '/dlib', $env:AZURE_DLIB_PATH,
        '/dmdf', $env:AZURE_METADATA_PATH,
        '/p7', $p7Dir,
        '/p7ce', 'DetachedSignedData',
        '/p7co', '1.3.6.1.4.1.311.2.1.4',
        $ArtifactPath
    ) -ErrorMessage "signtool failed to emit PKCS#7 signature for $ArtifactPath"

    if (-not (Test-Path -LiteralPath $p7Path)) {
        throw "Expected PKCS#7 file was not generated at $p7Path"
    }

    Move-Item -LiteralPath $p7Path -Destination $SignaturePath -Force
    Write-Host "Detached signature written to $SignaturePath" -ForegroundColor Gray
}

Write-Host "Successfully signed: $ArtifactPath" -ForegroundColor Green
