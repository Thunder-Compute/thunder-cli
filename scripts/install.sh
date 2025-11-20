#!/usr/bin/env bash
set -euo pipefail

# Install tnr by reading latest.json from Google Cloud Storage (via download.thundercompute.com), verifying checksums, and
# installing to ~/.tnr/bin.

CHANNEL=${TNR_UPDATE_CHANNEL:-stable}
VERSION=${TNR_VERSION:-}
LATEST_URL=${TNR_LATEST_URL:-}
INSTALL_DIR="${HOME}/.tnr/bin"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH=amd64;;
  arm64|aarch64) ARCH=arm64;;
  *) echo "Unsupported arch: $ARCH" >&2; exit 1;;
esac

# Check for required commands
check_deps() {
  local missing=()
  command -v curl >/dev/null 2>&1 || missing+=("curl")
  command -v tar >/dev/null 2>&1 || missing+=("tar")
  command -v gzip >/dev/null 2>&1 || missing+=("gzip")
  
  if [[ ${#missing[@]} -gt 0 ]]; then
    echo "Error: Missing required commands: ${missing[*]}" >&2
    echo "Please install: ${missing[*]}" >&2
    exit 1
  fi
}

check_deps

# Install jq if missing
ensure_jq() {
  if command -v jq >/dev/null 2>&1; then
    return 0
  fi
  
  echo "jq not found. Attempting to install..."
  
  # Check if running as root (use id -u for better compatibility)
  local is_root=false
  if [[ "$(id -u)" -eq 0 ]]; then
    is_root=true
  fi
  
  # Try to detect if we can install packages
  local can_install=false
  local install_cmd=""
  
  if command -v apt-get >/dev/null 2>&1; then
    # Debian/Ubuntu
    if [[ "$is_root" == "true" ]]; then
      can_install=true
      install_cmd="apt-get update -qq && apt-get install -y jq"
    elif sudo -n true 2>/dev/null; then
      can_install=true
      install_cmd="sudo apt-get update -qq && sudo apt-get install -y jq"
    else
      echo "jq is required. Please install it manually:" >&2
      echo "  sudo apt-get update && sudo apt-get install -y jq" >&2
      exit 1
    fi
  elif command -v apk >/dev/null 2>&1; then
    # Alpine
    if [[ "$is_root" == "true" ]]; then
      can_install=true
      install_cmd="apk add --no-cache jq"
    elif sudo -n true 2>/dev/null; then
      can_install=true
      install_cmd="sudo apk add --no-cache jq"
    else
      echo "jq is required. Please install it manually:" >&2
      echo "  sudo apk add --no-cache jq" >&2
      exit 1
    fi
  elif command -v yum >/dev/null 2>&1; then
    # RHEL/CentOS 7
    if [[ "$is_root" == "true" ]]; then
      can_install=true
      install_cmd="yum install -y jq"
    elif sudo -n true 2>/dev/null; then
      can_install=true
      install_cmd="sudo yum install -y jq"
    else
      echo "jq is required. Please install it manually:" >&2
      echo "  sudo yum install -y jq" >&2
      exit 1
    fi
  elif command -v dnf >/dev/null 2>&1; then
    # Fedora/RHEL/CentOS 8+
    if [[ "$is_root" == "true" ]]; then
      can_install=true
      install_cmd="dnf install -y jq"
    elif sudo -n true 2>/dev/null; then
      can_install=true
      install_cmd="sudo dnf install -y jq"
    else
      echo "jq is required. Please install it manually:" >&2
      echo "  sudo dnf install -y jq" >&2
      exit 1
    fi
  fi
  
  if [[ "$can_install" == "true" ]]; then
    echo "Installing jq..."
    if eval "$install_cmd"; then
      if command -v jq >/dev/null 2>&1; then
        echo "✓ jq installed successfully"
        return 0
      fi
    fi
  fi
  
  # If we get here, installation failed or not supported
  echo "Error: jq is required but could not be installed automatically." >&2
  echo "Please install jq manually: https://stedolan.github.io/jq/download/" >&2
  exit 1
}

ensure_jq

if [[ -z "$LATEST_URL" ]]; then
  if [[ -n "${TNR_DOWNLOAD_BASE:-}" ]]; then
    LATEST_URL="${TNR_DOWNLOAD_BASE}/tnr/releases/latest.json"
  else
    # Default to Google Cloud Storage via download.thundercompute.com custom domain
    LATEST_URL="https://download.thundercompute.com/tnr/releases/latest.json"
  fi
fi

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

echo "Fetching manifest: $LATEST_URL"
curl -fsSL "$LATEST_URL" -o "$tmpdir/latest.json"

if [[ -z "$VERSION" ]]; then
  VERSION=$(jq -r '.version' "$tmpdir/latest.json")
fi

asset_key="${OS}/${ARCH}"
url=$(jq -r --arg k "$asset_key" '.assets[$k]' "$tmpdir/latest.json")

# Detect archive extension from URL - Linux uses .tar.gz, Windows uses .zip
if [[ "$url" == *.tar.gz ]]; then
  archive="$tmpdir/tnr.tar.gz"
elif [[ "$url" == *.zip ]]; then
  archive="$tmpdir/tnr.zip"
  # Check for unzip if we need it
  if ! command -v unzip >/dev/null 2>&1; then
    echo "Error: unzip is required for .zip archives but not installed." >&2
    exit 1
  fi
else
  echo "Unknown archive format in URL: $url" >&2
  exit 1
fi

checksums=$(jq -r '.assets["checksums"]' "$tmpdir/latest.json")

echo "Downloading $url"
curl -fL "$url" -o "$archive"
curl -fsSL "$checksums" -o "$tmpdir/checksums.txt"

echo "Verifying checksum"
sum=$(sha256sum "$archive" | awk '{print $1}')
grep -q "$sum" "$tmpdir/checksums.txt" || { echo "Checksum mismatch" >&2; exit 1; }

mkdir -p "$INSTALL_DIR"

echo "Extracting"
if [[ "$archive" == *.tar.gz ]]; then
  tar -xzf "$archive" -C "$tmpdir"
else
  unzip -q "$archive" -d "$tmpdir"
fi

install -m 0755 "$tmpdir/tnr" "$INSTALL_DIR/tnr"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) 
    echo ""
    echo "Add $INSTALL_DIR to your PATH:"
    echo "  export PATH=\"\$HOME/.tnr/bin:\$PATH\""
    echo ""
    echo "Or add to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
    echo "  echo 'export PATH=\"\$HOME/.tnr/bin:\$PATH\"' >> ~/.bashrc"
    ;;
esac

echo "✓ Installed tnr $VERSION to $INSTALL_DIR"


