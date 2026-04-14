#!/usr/bin/env bash
set -euo pipefail

# Install tnr by fetching the latest release from GitHub, verifying checksums,
# and installing to ~/.tnr/bin.

VERSION="${TNR_VERSION:-}"
LATEST_URL="${TNR_LATEST_URL:-}"
INSTALL_DIR="${HOME}/.tnr/bin"
GITHUB_API="https://api.github.com/repos/Thunder-Compute/thunder-cli/releases/latest"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in x86_64|amd64) ARCH=amd64;; arm64|aarch64) ARCH=arm64;; *) echo "Unsupported arch: $ARCH" >&2; exit 1;; esac
case "$OS" in darwin) OS=macos;; esac
FILE_OS="$OS"; [[ "$OS" == "macos" ]] && FILE_OS=darwin

for cmd in curl tar gzip; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "Error: $cmd is required but not installed." >&2; exit 1; }
done

ensure_jq() {
  command -v jq >/dev/null 2>&1 && return 0

  echo "jq not found. Attempting to install..."
  local sudo=""; [[ "$(id -u)" -ne 0 ]] && { sudo -n true 2>/dev/null && sudo="sudo" || { echo "jq is required. Install it manually and retry." >&2; exit 1; }; }

  if   command -v apt-get >/dev/null 2>&1; then $sudo apt-get update -qq && $sudo apt-get install -y jq
  elif command -v apk     >/dev/null 2>&1; then $sudo apk add --no-cache jq
  elif command -v dnf     >/dev/null 2>&1; then $sudo dnf install -y jq
  elif command -v yum     >/dev/null 2>&1; then $sudo yum install -y jq
  else echo "Cannot auto-install jq. Install it manually: https://jqlang.github.io/jq/download/" >&2; exit 1
  fi

  command -v jq >/dev/null 2>&1 || { echo "jq installation failed." >&2; exit 1; }
}
ensure_jq

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

# Resolve version and download URLs
if [[ -n "$LATEST_URL" ]]; then
  echo "Fetching manifest: $LATEST_URL"
  curl -fsSL "$LATEST_URL" -o "$tmpdir/manifest.json"
  [[ -z "$VERSION" ]] && VERSION=$(jq -r '.version' "$tmpdir/manifest.json")
  url=$(jq -r --arg k "${OS}/${ARCH}" '.assets[$k]' "$tmpdir/manifest.json")
  checksums=$(jq -r '.assets["checksums"]' "$tmpdir/manifest.json")
elif [[ -n "$VERSION" ]]; then
  VERSION="${VERSION#v}"
  url="https://github.com/Thunder-Compute/thunder-cli/releases/download/v${VERSION}/tnr_${VERSION}_${FILE_OS}_${ARCH}.tar.gz"
  checksums="https://github.com/Thunder-Compute/thunder-cli/releases/download/v${VERSION}/checksums.txt"
else
  echo "Fetching latest release from GitHub..."
  curl -fsSL -H "Accept: application/vnd.github+json" "$GITHUB_API" -o "$tmpdir/release.json"
  VERSION=$(jq -r '.tag_name' "$tmpdir/release.json")
  VERSION="${VERSION#v}"
  url="https://github.com/Thunder-Compute/thunder-cli/releases/download/v${VERSION}/tnr_${VERSION}_${FILE_OS}_${ARCH}.tar.gz"
  checksums="https://github.com/Thunder-Compute/thunder-cli/releases/download/v${VERSION}/checksums.txt"
fi

# Download and verify
echo "Downloading tnr v${VERSION}..."
curl -fL "$url" -o "$tmpdir/tnr.tar.gz"
curl -fsSL "$checksums" -o "$tmpdir/checksums.txt"
sum=$(sha256sum "$tmpdir/tnr.tar.gz" | awk '{print $1}')
grep -q "$sum" "$tmpdir/checksums.txt" || { echo "Checksum mismatch" >&2; exit 1; }

# Extract and install
tar -xzf "$tmpdir/tnr.tar.gz" -C "$tmpdir"
mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmpdir/tnr" "$INSTALL_DIR/tnr"

# Add to PATH if needed
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  PROFILE="$HOME/.profile"
  if   [[ -n "${ZSH_VERSION:-}" ]] || [[ "$SHELL" == */zsh ]];  then PROFILE="$HOME/.zshrc"
  elif [[ -n "${BASH_VERSION:-}" ]] || [[ "$SHELL" == */bash ]]; then PROFILE="$HOME/.bashrc"; [[ -f "$HOME/.bash_profile" ]] && PROFILE="$HOME/.bash_profile"
  fi

  if [[ -w "$(dirname "$PROFILE")" ]] && ! grep -q '.tnr/bin' "$PROFILE" 2>/dev/null; then
    printf '\n# Added by tnr installer\nexport PATH="$HOME/.tnr/bin:$PATH"\n' >> "$PROFILE"
    echo "Added $INSTALL_DIR to PATH in $PROFILE — restart your terminal or run: source $PROFILE"
  else
    echo "Add $INSTALL_DIR to your PATH: export PATH=\"\$HOME/.tnr/bin:\$PATH\""
  fi
fi

echo "Installed tnr v${VERSION} to $INSTALL_DIR"
