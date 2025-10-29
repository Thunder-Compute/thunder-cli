# To build:

go build -o tnr

# To run tests do:

go test ./... -v

## Install

(NOT IMPLEMENTED YET) Package Managers:

- Homebrew (macOS/Linux):
  - `brew tap Thunder-Compute/tnr`
  - `brew install tnr`
- Windows (Scoop): `scoop bucket add tnr https://github.com/Thunder-Compute/scoop-bucket && scoop install tnr`
- Windows (winget): `winget install Thunder.tnr`
- Debian/Ubuntu: `.deb` from Releases or `apt` repo (via nfpm output)
- RHEL/CentOS/Fedora: `.rpm` from Releases or `yum` repo (via nfpm output)

Scripts (macOS/Linux):

```bash
TNR_S3_BUCKET=<your-bucket> AWS_REGION=<region> \
  bash -c "$(curl -fsSL https://raw.githubusercontent.com/Thunder-Compute/thunder-cli/main/scripts/install.sh)"
```

Windows PowerShell:

```powershell
$env:TNR_S3_BUCKET='<your-bucket>'
$env:AWS_REGION='<region>'
irm https://raw.githubusercontent.com/Thunder-Compute/thunder-cli/main/scripts/install.ps1 | iex
```

(NOT IMPLEMENTED YET) Python (shim, recommended via pipx):

```bash
pipx install tnr
```

## Update

- Package manager installs: use the PM (e.g., `brew upgrade tnr`, `scoop update tnr`, `winget upgrade Thunder.tnr`).
- Standalone: `tnr self-update`.

