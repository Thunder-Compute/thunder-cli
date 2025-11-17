# Thunder Compute CLI (`tnr`)

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/Thunder-Compute/thunder-cli/blob/main/LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.25.3-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/Thunder-Compute/thunder-cli)](https://github.com/Thunder-Compute/thunder-cli/releases)
[![macOS](https://img.shields.io/badge/macOS-supported-success)](#installation)
[![Linux](https://img.shields.io/badge/Linux-supported-success)](#installation)
[![Windows](https://img.shields.io/badge/Windows-supported-success)](#installation)

## Overview

`tnr` is the official command-line interface for Thunder Compute, a cloud GPU platform designed for AI/ML prototyping. Built on a proprietary orchestration stack, Thunder Compute delivers the most cost-effective GPU compute available.

This comprehensive CLI reference enables you to manage GPU and CPU instances, configure compute resources, establish SSH connections, transfer files, and handle all aspects of your compute resources directly from the terminal.

## Documentation

- CLI Reference: https://www.thundercompute.com/docs/cli-reference
- API Reference: https://www.thundercompute.com/docs/api-reference
- Get Started: https://www.thundercompute.com/docs/quickstart
- Troubleshooting: https://www.thundercompute.com/docs/troubleshooting

## Installation

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/Thunder-Compute/thunder-cli/main/scripts/install.sh | bash
```

```bash
brew tap Thunder-Compute/tnr
brew install tnr
```

### Windows

```powershell
scoop bucket add tnr https://github.com/Thunder-Compute/scoop-bucket
scoop install tnr
```

```powershell
winget install Thunder.tnr
```

### Build from Source

```bash
git clone https://github.com/Thunder-Compute/thunder-cli.git
cd thunder-cli
go build -o tnr

# Then run
./tnr
```

## Quick Start

```bash
# 1. Authenticate with Thunder Compute
tnr login

# 2. Create a new GPU instance
tnr create

# 3. Check instance status
tnr status

# 4. Connect to your instance
tnr connect 0

# 5. Copy files to/from instance
tnr scp myfile.py 0:/home/ubuntu/
tnr scp 0:/home/ubuntu/results.txt ./

# 6. Delete instance when done
tnr delete 0
```

## Usage

### Authentication

```bash
tnr login
tnr logout
```

### Instances

```bash
tnr create
tnr create --gpu t4
tnr create --template ollama
tnr status
tnr delete <id>
```

### Connecting

```bash
tnr connect <id>
tnr connect <id> -t 8000
tnr connect <id> -t 8000 -t 8080
```

### File Transfer

```bash
tnr scp ./local 0:/remote
tnr scp 0:/remote ./local
tnr scp -r ./dir 0:/remote
tnr scp -r 0:/remote/dir ./
```

### Updates

```bash
brew upgrade tnr
scoop update tnr
winget upgrade Thunder.tnr
```

### Shell Completion

```bash
tnr completion bash > /etc/bash_completion.d/tnr
tnr completion zsh  > "${fpath[1]}/_tnr"
tnr completion fish > ~/.config/fish/completions/tnr.fish
tnr completion powershell > tnr.ps1
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

Copyright © 2025 Thunder Compute

## Acknowledgments

Built with:

- Cobra
- Bubble Tea
- Lipgloss
- GoReleaser
