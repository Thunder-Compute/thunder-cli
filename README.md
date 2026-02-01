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
Using a proprietary orchestration engine, Thunder Compute delivers fast provisioning, low-latency execution, and one of the most cost-effective GPU compute offerings available.

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

- [Download for macOS (Apple Silicon)](https://storage.googleapis.com/thunder-cli/tnr/releases/latest/macos/tnr_darwin_arm64.pkg)
- [Download for macOS (Intel)](https://storage.googleapis.com/thunder-cli/tnr/releases/latest/macos/tnr_darwin_amd64.pkg)

**Homebrew:**

```bash
brew tap Thunder-Compute/tnr
brew install tnr
```

---

### Linux

- [Download for Linux (Intel/AMD)](https://storage.googleapis.com/thunder-cli/tnr/releases/latest/linux/tnr_linux_amd64.tar.gz)
- [Download for Linux (ARM)](https://storage.googleapis.com/thunder-cli/tnr/releases/latest/linux/tnr_linux_arm64.tar.gz)

**Install script (recommended):**

```bash
curl -fsSL https://raw.githubusercontent.com/Thunder-Compute/thunder-cli/main/scripts/install.sh | bash
```

---

### Windows

**Direct download:**

- [Download for Windows (Intel/AMD)](https://storage.googleapis.com/thunder-cli/tnr/releases/latest/windows/tnr_windows_amd64.msi)
- [Download for Windows (ARM)](https://storage.googleapis.com/thunder-cli/tnr/releases/latest/windows/tnr_windows_arm64.msi)

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

# License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
