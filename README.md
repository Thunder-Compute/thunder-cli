<div align="center">
  <img width="900" alt="Thunder Compute Logo" src="https://github.com/user-attachments/assets/671fd268-4261-4508-8cde-1f116491f42e" />
</div>

# Overview

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/Thunder-Compute/thunder-cli/blob/main/LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.25.3-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/Thunder-Compute/thunder-cli)](https://github.com/Thunder-Compute/thunder-cli/releases)
[![macOS](https://img.shields.io/badge/macOS-supported-success)](#installation)
[![Linux](https://img.shields.io/badge/Linux-supported-success)](#installation)
[![Windows](https://img.shields.io/badge/Windows-supported-success)](#installation)

`tnr` is the official command-line interface for Thunder Compute, a high-performance cloud GPU platform built for AI/ML prototyping and experimentation.
Using a proprietary orchestration engine, Thunder Compute delivers fast provisioning, low-latency execution, and low-cost GPU compute designed for developer workflows.

The `tnr` CLI supports:

- Provisioning and managing GPU instances
- Configuring compute resources and instance specifications
- Secure SSH access and session management
- File transfer capabilities (SCP upload and download)
- Port forwarding for accessing remote services locally
- Automated update checks and version management
- Cross-platform support for macOS, Linux, and Windows

# Documentation

- **CLI Reference:** https://www.thundercompute.com/docs/cli-reference
- **API Reference:** https://www.thundercompute.com/docs/api-reference
- **Get Started:** https://www.thundercompute.com/docs/quickstart
- **Troubleshooting:** https://www.thundercompute.com/docs/troubleshooting

# Installation

Install `tnr` using one of the supported methods below. You may also download installers and binaries directly from the [Latest Release](https://github.com/Thunder-Compute/thunder-cli/releases) page.

### macOS

Download the `.pkg` installer for your platform from the [Latest Release](https://github.com/Thunder-Compute/thunder-cli/releases/latest) page (`tnr_*_darwin_arm64.pkg` for Apple Silicon, `tnr_*_darwin_amd64.pkg` for Intel).

**Homebrew:**

```bash
brew tap Thunder-Compute/tnr
brew install tnr
```

---

### Linux

Download `.tar.gz`, `.deb`, `.rpm`, or `.apk` packages from the [Latest Release](https://github.com/Thunder-Compute/thunder-cli/releases/latest) page.

**Install script (recommended):**

```bash
curl -fsSL https://raw.githubusercontent.com/Thunder-Compute/thunder-cli/main/scripts/install.sh | bash
```

---

### Windows

Download the `.msi` installer for your platform from the [Latest Release](https://github.com/Thunder-Compute/thunder-cli/releases/latest) page.

**Scoop:**

```powershell
scoop bucket add tnr https://github.com/Thunder-Compute/scoop-bucket
scoop install tnr
```

**Winget:**

```powershell
winget install Thunder.tnr
```

### Build from Source

```bash
git clone https://github.com/Thunder-Compute/thunder-cli.git
cd thunder-cli
go build -o tnr
./tnr
```

# Quick Start

```bash
tnr login           # Authenticate with Thunder Compute
tnr create          # Create a GPU instance
tnr status          # View instance status
tnr connect 0       # Connect to your instance

# File transfers
tnr scp myfile.py 0:/home/ubuntu/
tnr scp 0:/home/ubuntu/results.txt ./

tnr delete 0        # Delete instance
```

`tnr login` opens a browser and returns to a loopback callback on
`127.0.0.1`. On a headless or remote machine where that callback cannot be
reached, create a named API token in Console Settings and run
`tnr login --token <named_api_token>` instead.

To benchmark a GPU with direct access to the physical device, set `IMAGE_ID` to
the ID of a ready Thunder container image and run:

```sh
tnr sandbox create --image-id "$IMAGE_ID" --gpu h100 --timing-sensitive --json
```

The command returns the sandbox ID when Thunder accepts the request. The sandbox
expires 15 minutes after it becomes ready by default. Omit `--timing-sensitive`
to start a standard sandbox.

# Bug reports and feedback

Run `tnr report` for a guided report in the style of `tnr create`: choose **Report a problem** or **Suggest an improvement**, write a one-sentence summary and the details, then review the report and choose **Send report**, **Edit report**, or **Cancel**. Sign in before starting. `--kind`, `--subject`, and `--details` or `--details-file` fill in their steps, which are then skipped, and supplied text is sent as given. Quitting sends nothing and exits with code 130. Noninteractive reports default to bugs; use `--kind feedback` for product feedback.

```sh
tnr report --subject "Upload connection drops" --details-file ./report.txt --component cli --instance 42
tnr report --kind feedback --subject "Show remaining disk space" --details "Please show disk usage in the instance list." --yes --json
tnr report --subject "Unexpected result" --details-file - --dry-run --json < ./report.txt
```

Noninteractive submission requires `--subject`, `--details` or `--details-file`, and `--yes`. `--json` never prompts. `--details-file -` reads standard input. `--details` and `--details-file` are mutually exclusive. Subject and details limits are 200 and 10,000 Unicode characters; oversized text is rejected instead of truncated. `--dry-run` previews without authentication or submission and identifies the missing server-resolved requester.

Optional context flags are `--component` (`cli`, `console`, `api`, `mcp`, `virtualization`, or `unknown`), either `--instance` or `--sandbox`, and `--occurred-at` (RFC 3339). The report includes the CLI version, build commit and date, OS, and architecture. The server records submission time and resolves the requester from your login or verified API token owner. No logs, environment variables, shell history, or workload files are collected automatically. Only include evidence you intend to share with support, and omit credentials.

Successful submissions print an opaque report reference. Exit zero means a successful submission or dry run. Any failure exits nonzero and retains the entered report in the error output, so you can submit it again. JSON errors include `error`, `message`, and `report`. Reporting skips automatic update and subscription checks.

# License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
