# Thunder Compute CLI (`tnr`)

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/Thunder-Compute/thunder-cli/blob/main/LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.25.3-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/Thunder-Compute/thunder-cli)](https://github.com/Thunder-Compute/thunder-cli/releases)
[![Build](https://github.com/Thunder-Compute/thunder-cli/workflows/snapshot/badge.svg)](https://github.com/Thunder-Compute/thunder-cli/actions)
[![macOS](https://img.shields.io/badge/macOS-supported-success)](#installation)
[![Linux](https://img.shields.io/badge/Linux-supported-success)](#installation)
[![Windows](https://img.shields.io/badge/Windows-supported-success)](#installation)

**Fast, secure, and developer-friendly command-line interface for managing Thunder Compute GPU instances.**

`tnr` is the official CLI for [Thunder Compute](https://www.thundercompute.com), enabling seamless management of cloud GPU instances. Built for speed and ease of use, it handles authentication, instance provisioning, SSH connections, file transfers, and monitoring—all from your terminal.

---

## Features

- **⚡ Quick Instance Provisioning** – Create GPU instances (T4, A100, A100 XL) in seconds with flexible CPU and memory configurations
- **🔐 Automatic SSH Key Management** – Connect securely without manual SSH configuration
- **📦 Built-in File Transfer** – Copy files between local and remote instances with integrated SCP support
- **📊 Real-time Monitoring** – Track instance status, resource usage, and configuration details
- **🎨 Pre-configured Templates** – Launch instances with Ollama, ComfyUI, or WebUI Forge pre-installed
- **🌍 Cross-platform Support** – Native binaries for macOS, Linux, and Windows
- **🔄 Auto-update** – Automatically checks for updates whenever you run commands
- **🎯 Port Forwarding** – Easily expose remote services to your local machine

---

## Installation

### Package Managers

#### macOS / Linux (Homebrew)

```bash
brew tap Thunder-Compute/tap
brew install tnr
```

#### Windows (Scoop)

```powershell
scoop bucket add tnr https://github.com/Thunder-Compute/scoop-bucket
scoop install tnr
```

#### Windows (Winget)

```powershell
winget install Thunder.tnr
```

#### Debian / Ubuntu

Download the latest `.deb` package from [Releases](https://github.com/Thunder-Compute/thunder-cli/releases):

```bash
sudo dpkg -i tnr_*.deb
```

#### RHEL / CentOS / Fedora

Download the latest `.rpm` package from [Releases](https://github.com/Thunder-Compute/thunder-cli/releases):

```bash
sudo rpm -i tnr_*.rpm
```

### Install Scripts

#### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/Thunder-Compute/thunder-cli/main/scripts/install.sh | bash
```

#### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/Thunder-Compute/thunder-cli/main/scripts/install.ps1 | iex
```

### Manual Installation

1. Download the appropriate binary for your platform from [Releases](https://github.com/Thunder-Compute/thunder-cli/releases)
2. Extract the archive
3. Move the `tnr` binary to a directory in your `PATH` (e.g., `/usr/local/bin` on Unix systems)
4. Make it executable (Unix): `chmod +x /usr/local/bin/tnr`

### Build from Source

**Requirements:** Go 1.25.3 or later

```bash
git clone https://github.com/Thunder-Compute/thunder-cli.git
cd thunder-cli
go build -o tnr
```

---

## Quick Start

```bash
# 1. Authenticate with Thunder Compute
tnr login

# 2. Create a new GPU instance (default: A100)
tnr create --gpu a100

# 3. Connect to your instance
tnr connect 0

# 4. Check instance status
tnr status

# 5. Copy files to/from instance
tnr scp ./local-file.txt 0:/remote/path/
tnr scp 0:/remote/file.txt ./local-path/

# 6. Delete instance when done
tnr delete 0
```

---

## Usage

### Authentication

#### Login

Authenticate with Thunder Compute. Opens your browser to generate an API token:

```bash
tnr login
```

The token is stored in `~/.thunder/token`. You can also set the `TNR_API_TOKEN` environment variable for programmatic access.

#### Logout

Remove stored credentials:

```bash
tnr logout
```

### Instance Management

#### Create an Instance

**Basic creation (default: A100 GPU, 4 vCPUs, 32GB RAM):**

```bash
tnr create
```

**Specify GPU type:**

```bash
tnr create --gpu t4       # NVIDIA T4 (16GB VRAM)
tnr create --gpu a100     # NVIDIA A100 (40GB VRAM)
tnr create --gpu a100xl   # NVIDIA A100 (80GB VRAM)
```

**Multiple GPUs:**

```bash
tnr create --gpu a100 --num-gpus 4
```

**Custom CPU configuration:**

```bash
tnr create --vcpus 8  # 8 vCPUs = 64GB RAM
```

**Use a template:**

```bash
tnr create --template ollama      # Ollama LLM server
tnr create --template comfy-ui    # ComfyUI image generation
tnr create --template webui-forge # Stable Diffusion WebUI
```

**Production mode (enhanced stability):**

```bash
tnr create --mode production
```

#### View Instance Status

List all instances with details:

```bash
tnr status
```

Add `--no-wait` to disable automatic status monitoring.

#### Delete an Instance

```bash
tnr delete <instance_id>
```

⚠️ **Warning:** This permanently deletes the instance and all data. Back up important files first.

### Connecting to Instances

#### Basic SSH Connection

```bash
tnr connect <instance_id>
```

#### Port Forwarding

Forward one or more ports to access remote services locally:

```bash
# Forward a single port
tnr connect 0 -t 8000

# Forward multiple ports
tnr connect 0 -t 8000 -t 8080 -t 3000
```

After forwarding, access services at `localhost:<port>`.

### File Transfer (SCP)

Transfer files between your local machine and instances:

```bash
# Upload to instance
tnr scp ./local-file.txt 0:/remote/path/
tnr scp -r ./local-directory/ 0:/remote/path/

# Download from instance
tnr scp 0:/remote/file.txt ./local-path/
tnr scp -r 0:/remote/directory/ ./local-path/
```

**Path formats:**

- Remote: `<instance_id>:/path` (e.g., `0:/home/user/data`)
- Local: Standard absolute or relative paths

### Updates

`tnr` checks for updates automatically before running most commands. When a newer version is required or recommended, the CLI downloads and applies the update for you—no manual command needed.

#### Package manager installations

If you installed via Homebrew, Scoop, or Winget, continue to use your package manager's upgrade command:

```bash
# Homebrew
brew upgrade tnr

# Scoop
scoop update tnr

# Winget
winget upgrade Thunder.tnr
```

#### Manual installations

Download the latest release from GitHub and replace your existing `tnr` binary. Restart the CLI to use the updated version.

#### Disable automatic updates

If you need to prevent automatic updates (e.g., in CI/CD environments or for testing), set the `TNR_NO_SELFUPDATE` environment variable:

```bash
export TNR_NO_SELFUPDATE=1
```

When disabled, the CLI will skip update checks but continue running your commands.

### Shell Completion

Generate shell completion scripts:

```bash
# Bash
tnr completion bash | sudo tee /etc/bash_completion.d/tnr

# Zsh
tnr completion zsh > "${fpath[1]}/_tnr"

# Fish
tnr completion fish > ~/.config/fish/completions/tnr.fish

# PowerShell
tnr completion powershell > tnr.ps1
```

---

## Documentation

- **[CLI Reference](https://www.thundercompute.com/docs/cli-reference)** – Complete command documentation
- **[API Documentation](https://www.thundercompute.com/docs/api-reference)** – REST API reference
- **[Quickstart Guide](https://www.thundercompute.com/docs/quickstart)** – Get started quickly
- **[Technical Specifications](https://www.thundercompute.com/docs/technical-specifications)** – Hardware details
- **[Troubleshooting](https://www.thundercompute.com/docs/troubleshooting)** – Common issues and solutions

### Guides

- [Using Jupyter Notebooks](https://www.thundercompute.com/docs/guides/jupyter-notebooks)
- [Docker on Thunder Compute](https://www.thundercompute.com/docs/guides/docker)
- [Using Ephemeral Storage](https://www.thundercompute.com/docs/guides/ephemeral-storage)
- [Instance Templates](https://www.thundercompute.com/docs/guides/instance-templates)
- [Installing Conda](https://www.thundercompute.com/docs/guides/conda)

---

## Development

### Prerequisites

- Go 1.25.3 or later
- Git

### Setup

```bash
# Clone the repository
git clone https://github.com/Thunder-Compute/thunder-cli.git
cd thunder-cli

# Install dependencies
go mod download

# Build the CLI
go build -o tnr

# Run tests
go test ./... -v

# Run specific tests
go test ./cmd -v
go test ./api -v
```

### Project Structure

```
thunder-cli/
├── api/           # Thunder Compute API client
├── cmd/           # Cobra command definitions
├── internal/      # Internal utilities and version info
├── tui/           # Terminal UI components (Bubble Tea)
├── utils/         # SSH, SCP, and helper utilities
└── main.go        # Entry point
```

### Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for new functionality
4. Ensure all tests pass (`go test ./... -v`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

---

## Support

- **Issues & Bug Reports:** [GitHub Issues](https://github.com/Thunder-Compute/thunder-cli/issues)
- **Documentation:** [Thunder Compute Docs](https://www.thundercompute.com/docs)
- **Contact:** [Thunder Compute Support](https://www.thundercompute.com/contact)
- **Discord:** Join our community for real-time help and discussions

---

## Platform Compatibility

| Platform | Architecture          | Status       |
| -------- | --------------------- | ------------ |
| macOS    | ARM64 (Apple Silicon) | ✅ Supported |
| macOS    | AMD64 (Intel)         | ✅ Supported |
| Linux    | AMD64                 | ✅ Supported |
| Linux    | ARM64                 | ✅ Supported |
| Windows  | AMD64                 | ✅ Supported |

---

## Updating

For detailed update behavior, including how to check your current version and disable auto-updates, see the [Updates](#updates) section above.

**Quick reference:**

- **Package managers:** Use `brew upgrade tnr`, `scoop update tnr`, or `winget upgrade Thunder.tnr`
- **Manual installations:** Download the latest release and reinstall the binary

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

Copyright © 2025 Thunder Compute

---

## Acknowledgments

Built with:

- [Cobra](https://github.com/spf13/cobra) – CLI framework
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) – Terminal UI
- [Lipgloss](https://github.com/charmbracelet/lipgloss) – Terminal styling
- [Go SCP](https://github.com/bramvdbogaerde/go-scp) – File transfer
- [go-selfupdate](https://github.com/creativeprojects/go-selfupdate) – Self-update functionality

---

**[Get Started →](https://www.thundercompute.com/docs/quickstart)** | **[View on GitHub →](https://github.com/Thunder-Compute/thunder-cli)** | **[Report Issue →](https://github.com/Thunder-Compute/thunder-cli/issues)**
