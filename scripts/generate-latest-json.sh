#!/usr/bin/env bash
set -euo pipefail

# Generate a latest.json manifest pointing to versioned assets in Cloudflare R2 (via gettnr.com).
# Inputs:
#   VERSION (e.g., v1.2.3)
#   TNR_DOWNLOAD_BASE (optional, e.g., https://gettnr.com or custom CDN)
#   CHANNEL (stable|beta) default stable

VERSION=${VERSION:-}
DOWNLOAD_BASE=${TNR_DOWNLOAD_BASE:-}
CHANNEL=${CHANNEL:-stable}

if [[ -z "$VERSION" ]]; then
  echo "VERSION is required" >&2
  exit 1
fi

if [[ -n "$DOWNLOAD_BASE" ]]; then
  base="${DOWNLOAD_BASE}/tnr/releases/${VERSION}"
else
  # Default to Cloudflare R2 via gettnr.com custom domain
  base="https://gettnr.com/tnr/releases/${VERSION}"
fi

cat > latest.json <<JSON
{
  "version": "${VERSION}",
  "channel": "${CHANNEL}",
  "assets": {
    "darwin/arm64": "${base}/tnr_${VERSION}_darwin_arm64.tar.gz",
    "darwin/amd64": "${base}/tnr_${VERSION}_darwin_amd64.tar.gz",
    "linux/arm64": "${base}/tnr_${VERSION}_linux_arm64.tar.gz",
    "linux/amd64": "${base}/tnr_${VERSION}_linux_amd64.tar.gz",
    "windows/arm64": "${base}/tnr_${VERSION}_windows_arm64.zip",
    "windows/amd64": "${base}/tnr_${VERSION}_windows_amd64.zip",
    "macos/pkg": "${base}/tnr-${VERSION}.pkg",
    "windows/installer": "${base}/tnr-setup.exe",
    "checksums": "${base}/checksums.txt"
  }
}
JSON

echo "latest.json generated for ${VERSION}"


