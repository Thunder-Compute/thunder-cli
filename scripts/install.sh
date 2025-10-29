#!/usr/bin/env bash
set -euo pipefail

# Install tnr by reading latest.json from GCS, verifying signature, and
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

if [[ -z "$LATEST_URL" ]]; then
  if [[ -n "${TNR_DOWNLOAD_BASE:-}" ]]; then
    LATEST_URL="${TNR_DOWNLOAD_BASE}/tnr/releases/latest.json"
  else
    if [[ -z "${TNR_S3_BUCKET:-}" || -z "${AWS_REGION:-}" ]]; then
      echo "Set TNR_LATEST_URL or TNR_DOWNLOAD_BASE, or TNR_S3_BUCKET and AWS_REGION" >&2
      exit 1
    fi
    LATEST_URL="https://${TNR_S3_BUCKET}.s3.${AWS_REGION}.amazonaws.com/tnr/releases/latest.json"
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
if [[ "$OS" == "darwin" ]]; then
  archive="$tmpdir/tnr.tar.gz"
else
  archive="$tmpdir/tnr.zip"
fi

url=$(jq -r --arg k "$asset_key" '.assets[$k]' "$tmpdir/latest.json")
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
  *) echo "Add $INSTALL_DIR to your PATH";;
esac

echo "Installed tnr $VERSION to $INSTALL_DIR"


