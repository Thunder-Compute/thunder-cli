#!/usr/bin/env bash
set -euo pipefail

# Builds a macOS .pkg for tnr. If signing variables are provided, it will sign,
# notarize, and staple the installer. Otherwise, it produces an unsigned .pkg.

# Inputs:
#   ARTIFACT_DIR: directory containing built binary "tnr" (required)
#   VERSION: version string (e.g., v1.2.3) (required)
#   OUT_DIR: output directory for the .pkg (default: dist)
#   SIGNING_IDENTITY: optional Developer ID Installer identity
#   APPLE_ID, APPLE_TEAM_ID, APPLE_APP_SPECIFIC_PASSWORD: optional for notarization

ARTIFACT_DIR=${ARTIFACT_DIR:-}
VERSION=${VERSION:-}
OUT_DIR=${OUT_DIR:-dist}

if [[ -z "$ARTIFACT_DIR" || -z "$VERSION" ]]; then
  echo "ARTIFACT_DIR and VERSION are required" >&2
  exit 1
fi

WORKDIR=$(mktemp -d)
PKGROOT="$WORKDIR/pkgroot"
mkdir -p "$PKGROOT/usr/local/bin"

install -m 0755 "$ARTIFACT_DIR/tnr" "$PKGROOT/usr/local/bin/tnr"

IDENTIFIER="com.thunder.tnr"
PKG_NAME="tnr-${VERSION}.pkg"
UNSIGNED_PKG="$WORKDIR/unsigned.pkg"

pkgbuild \
  --root "$PKGROOT" \
  --install-location / \
  --identifier "$IDENTIFIER" \
  --version "${VERSION#v}" \
  "$UNSIGNED_PKG"

mkdir -p "$OUT_DIR"

if [[ -n "${SIGNING_IDENTITY:-}" ]]; then
  PRODUCT="$WORKDIR/tnr-signed.pkg"
  productbuild \
    --package "$UNSIGNED_PKG" \
    --sign "$SIGNING_IDENTITY" \
    "$PRODUCT"

  if [[ -n "${APPLE_ID:-}" && -n "${APPLE_TEAM_ID:-}" && -n "${APPLE_APP_SPECIFIC_PASSWORD:-}" ]]; then
    xcrun notarytool submit "$PRODUCT" \
      --apple-id "$APPLE_ID" \
      --team-id "$APPLE_TEAM_ID" \
      --password "$APPLE_APP_SPECIFIC_PASSWORD" \
      --wait
    xcrun stapler staple "$PRODUCT"
  fi

  mv "$PRODUCT" "$OUT_DIR/$PKG_NAME"
else
  mv "$UNSIGNED_PKG" "$OUT_DIR/$PKG_NAME"
fi

echo "Created $OUT_DIR/$PKG_NAME"


