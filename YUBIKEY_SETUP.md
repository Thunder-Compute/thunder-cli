# YubiKey Code Signing Setup Guide

## Prerequisites
- YubiKey with code signing certificate loaded
- Self-hosted Windows runner (`thunder-yubikey`) configured and running
- Windows SDK installed on the runner
- YubiKey drivers installed

## Step-by-Step Setup

### 1. Run Setup Script on Your Windows Runner

On your Windows self-hosted runner machine:

```powershell
# Download and run the setup script
pwsh ./scripts/setup-yubikey-signing.ps1
```

This script will:
- ✓ Verify Windows SDK (signtool.exe) is installed
- ✓ Detect your YubiKey
- ✓ List available certificates
- ✓ Test signing capability
- ✓ Provide the certificate thumbprint for GitHub secrets

### 2. Configure GitHub Secrets

Go to your repository settings: `Settings → Secrets and variables → Actions`

Add these secrets:

| Secret Name | Value | Description |
|------------|-------|-------------|
| `WINDOWS_CERT_THUMBPRINT` | `<from setup script>` | SHA-1 thumbprint of your code signing cert |
| `YUBIKEY_PIN` | `<your pin>` | PIN to unlock YubiKey (6-8 digits) |
| `TIMESTAMP_SERVER` | `http://timestamp.digicert.com` | RFC 3161 timestamp server |

### 3. Trigger the Release Workflow

The workflow triggers on version tags. To create a release:

```bash
# Create and push a version tag
git tag v1.0.0
git push origin v1.0.0
```

Or create a pre-release for testing:

```bash
git tag v1.0.0-rc.1
git push origin v1.0.0-rc.1
```

### 4. Monitor the Workflow

Watch the workflow run:
- Go to: `Actions → release` in your GitHub repository
- The Windows job will run on your `thunder-yubikey` runner
- Monitor the "Sign MSI files with YubiKey" step

## Troubleshooting

### YubiKey Not Detected
```powershell
# Check smart card readers
Get-PnpDevice -Class SmartCardReader

# Verify YubiKey minidriver is installed
certutil -scinfo
```

### Certificate Not Found
```powershell
# List all certificates with private keys
Get-ChildItem Cert:\CurrentUser\My | Where-Object {$_.HasPrivateKey}

# Check if certificate is code signing type
Get-ChildItem Cert:\CurrentUser\My | ForEach-Object {
    $_ | Select Subject, Thumbprint, @{N='EKU';E={$_.EnhancedKeyUsageList.FriendlyName}}
}
```

### Signing Fails
- Verify YubiKey is plugged in
- Check PIN is correct in GitHub secrets
- Ensure certificate hasn't expired
- Try signing manually first

### Manual Test
```powershell
# Create test file
echo "test" > test.txt

# Sign it (replace THUMBPRINT)
signtool sign /sha1 <THUMBPRINT> /fd sha256 /tr http://timestamp.digicert.com /td sha256 /v test.txt

# Verify signature
signtool verify /pa /v test.txt
```

## Security Best Practices

- 🔒 Keep YubiKey PIN in GitHub Secrets, never commit it
- 🔒 Use a dedicated YubiKey for CI/CD if possible
- 🔒 Restrict access to self-hosted runner machine
- 🔒 Enable audit logging on the runner
- 🔒 Rotate YubiKey PIN periodically
- 🔒 Monitor certificate expiration dates

## Support

If you encounter issues:
1. Run the setup script to verify configuration
2. Check Windows Event Viewer for smart card errors
3. Verify YubiKey firmware is up to date
4. Consult Yubico documentation: https://docs.yubico.com

