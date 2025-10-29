#!/usr/bin/env bash
set -euo pipefail

# Generate a latest.json manifest pointing to versioned assets in S3 (or CloudFront).
# Inputs:
#   VERSION (e.g., v1.2.3)
#   BUCKET (S3 bucket name)
#   AWS_REGION (required if TNR_DOWNLOAD_BASE unset)
#   TNR_DOWNLOAD_BASE (optional, e.g., https://dxxxxx.cloudfront.net)
#   CHANNEL (stable|beta) default stable

VERSION=${VERSION:-}
BUCKET=${BUCKET:-}
AWS_REGION=${AWS_REGION:-}
DOWNLOAD_BASE=${TNR_DOWNLOAD_BASE:-}
CHANNEL=${CHANNEL:-stable}

if [[ -z "$VERSION" || -z "$BUCKET" ]]; then
  echo "VERSION and BUCKET are required" >&2
  exit 1
fi

if [[ -z "$DOWNLOAD_BASE" ]]; then
  if [[ -z "$AWS_REGION" ]]; then
    echo "AWS_REGION is required when TNR_DOWNLOAD_BASE is not set" >&2
    exit 1
  fi
  base="https://${BUCKET}.s3.${AWS_REGION}.amazonaws.com/tnr/releases/${VERSION}"
else
  base="${DOWNLOAD_BASE}/tnr/releases/${VERSION}"
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


