#!/usr/bin/env pwsh
# YubiKey Signing Setup Script for thunder-yubikey runner
# Run this on your Windows self-hosted runner

Write-Host "=== YubiKey Code Signing Setup ===" -ForegroundColor Cyan
Write-Host ""

# Check if running as Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Warning "This script should be run as Administrator for best results"
}

# 1. Check if Windows SDK is installed (contains signtool.exe)
Write-Host "1. Checking for Windows SDK (signtool.exe)..." -ForegroundColor Yellow
$signtool = Get-ChildItem -Path "C:\Program Files (x86)\Windows Kits\10\bin\*\x64\signtool.exe" -ErrorAction SilentlyContinue | Select-Object -First 1

if ($signtool) {
    Write-Host "   ✓ Found signtool: $($signtool.FullName)" -ForegroundColor Green
} else {
    Write-Host "   ✗ signtool.exe not found!" -ForegroundColor Red
    Write-Host "   Download Windows SDK from: https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/" -ForegroundColor Yellow
    Write-Host "   Or install via: choco install windows-sdk-10.1" -ForegroundColor Yellow
    exit 1
}

# 2. Check if YubiKey is connected
Write-Host ""
Write-Host "2. Checking for YubiKey..." -ForegroundColor Yellow
$smartCards = Get-PnpDevice -Class SmartCardReader | Where-Object {$_.Status -eq "OK"}
if ($smartCards) {
    Write-Host "   ✓ Smart card reader(s) detected:" -ForegroundColor Green
    $smartCards | ForEach-Object { Write-Host "     - $($_.FriendlyName)" -ForegroundColor Gray }
} else {
    Write-Host "   ✗ No smart card reader detected!" -ForegroundColor Red
    Write-Host "   Please ensure YubiKey is plugged in and drivers are installed." -ForegroundColor Yellow
    exit 1
}

# 3. List available certificates in the certificate store
Write-Host ""
Write-Host "3. Checking certificates in store..." -ForegroundColor Yellow
$certs = Get-ChildItem Cert:\CurrentUser\My | Where-Object {$_.HasPrivateKey -eq $true}

if ($certs.Count -eq 0) {
    Write-Host "   ✗ No certificates with private keys found!" -ForegroundColor Red
    Write-Host "   Please import your code signing certificate to the YubiKey." -ForegroundColor Yellow
} else {
    Write-Host "   ✓ Found $($certs.Count) certificate(s) with private keys:" -ForegroundColor Green
    foreach ($cert in $certs) {
        Write-Host ""
        Write-Host "   Certificate:" -ForegroundColor Cyan
        Write-Host "     Subject:    $($cert.Subject)" -ForegroundColor Gray
        Write-Host "     Thumbprint: $($cert.Thumbprint)" -ForegroundColor Gray
        Write-Host "     Expires:    $($cert.NotAfter)" -ForegroundColor Gray
        Write-Host "     Issuer:     $($cert.Issuer)" -ForegroundColor Gray
        
        # Check if it's a code signing cert
        $hasCodeSigning = $cert.EnhancedKeyUsageList | Where-Object {$_.FriendlyName -eq "Code Signing"}
        if ($hasCodeSigning) {
            Write-Host "     Type:       Code Signing Certificate ✓" -ForegroundColor Green
            Write-Host ""
            Write-Host "   >>> USE THIS THUMBPRINT FOR GITHUB SECRET: $($cert.Thumbprint)" -ForegroundColor Yellow -BackgroundColor DarkBlue
        }
    }
}

# 4. Test signing capability
Write-Host ""
Write-Host "4. Testing signing capability..." -ForegroundColor Yellow
$testFile = "$env:TEMP\test-signing-$(Get-Random).txt"
"Test file for code signing" | Out-File -FilePath $testFile

$codeSigCert = $certs | Where-Object {$_.EnhancedKeyUsageList | Where-Object {$_.FriendlyName -eq "Code Signing"}} | Select-Object -First 1

if ($codeSigCert) {
    Write-Host "   Testing with certificate: $($codeSigCert.Subject)" -ForegroundColor Gray
    Write-Host "   You may be prompted for your YubiKey PIN..." -ForegroundColor Yellow
    
    try {
        & $signtool.FullName sign `
            /sha1 $codeSigCert.Thumbprint `
            /fd sha256 `
            /tr "http://timestamp.digicert.com" `
            /td sha256 `
            /v `
            $testFile 2>&1 | Out-Null
        
        if ($LASTEXITCODE -eq 0) {
            Write-Host "   ✓ Signing test SUCCESSFUL!" -ForegroundColor Green
            Write-Host "   Your YubiKey is properly configured for code signing." -ForegroundColor Green
        } else {
            Write-Host "   ✗ Signing test FAILED!" -ForegroundColor Red
            Write-Host "   Check if YubiKey PIN is correct and certificate is valid." -ForegroundColor Yellow
        }
    } catch {
        Write-Host "   ✗ Signing test FAILED with error: $($_.Exception.Message)" -ForegroundColor Red
    } finally {
        Remove-Item -Path $testFile -Force -ErrorAction SilentlyContinue
    }
} else {
    Write-Host "   ⚠ No code signing certificate found to test with." -ForegroundColor Yellow
}

# 5. Summary and next steps
Write-Host ""
Write-Host "=== Setup Summary ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "Next steps to configure GitHub Secrets:" -ForegroundColor Yellow
Write-Host "1. Go to: https://github.com/<YOUR_REPO>/settings/secrets/actions" -ForegroundColor Gray
Write-Host "2. Add these secrets:" -ForegroundColor Gray
Write-Host ""

if ($codeSigCert) {
    Write-Host "   WINDOWS_CERT_THUMBPRINT = $($codeSigCert.Thumbprint)" -ForegroundColor Cyan
}
Write-Host "   YUBIKEY_PIN = <your_yubikey_pin>" -ForegroundColor Cyan
Write-Host "   TIMESTAMP_SERVER = http://timestamp.digicert.com" -ForegroundColor Cyan
Write-Host ""
Write-Host "3. Ensure YubiKey stays plugged into the runner during builds!" -ForegroundColor Yellow
Write-Host ""

