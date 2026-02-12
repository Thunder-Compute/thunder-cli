#!/bin/bash
# Build a local .pkg installer for testing (no signing, no notarization)
# Replicates the goreleaser post-build hook logic from .goreleaser.macos.yaml
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CLI_DIR="$(dirname "$SCRIPT_DIR")"
NAME="tnr"
VERSION="0.0.0-local"
ARCH="$(uname -m)"
# Normalize arch to Go naming
if [ "$ARCH" = "x86_64" ]; then ARCH="amd64"; fi
if [ "$ARCH" = "arm64" ]; then ARCH="arm64"; fi

echo "[BUILD] Building $NAME binary for darwin/$ARCH..."
cd "$CLI_DIR"
CGO_ENABLED=0 GOOS=darwin GOARCH="$ARCH" go build -o "dist/${NAME}" ./main.go
BIN="$CLI_DIR/dist/${NAME}"
echo "[OK] Binary: $BIN"

# Create temp working directory
WORK="$(/usr/bin/mktemp -d -t ${NAME}.pkg.${ARCH}.XXXXXX)"
trap "rm -rf \"$WORK\"" EXIT
echo "[OK] Working dir: $WORK"

# Stage binary into payload root
PKGROOT="$WORK/root"
/bin/mkdir -p "$PKGROOT/usr/local/bin"
/usr/bin/install -m 0755 "$BIN" "$PKGROOT/usr/local/bin/${NAME}"

# Build component .pkg (no scripts)
COMPONENT_PKG="$WORK/component.pkg"
/usr/bin/pkgbuild --root "$PKGROOT" \
  --identifier "com.thundercompute.${NAME}" \
  --version "$VERSION" \
  --install-location "/" \
  "$COMPONENT_PKG"
echo "[OK] Built component .pkg"

# Prepare resources for productbuild (conclusion panel)
MACOS_PKG_DIR="$CLI_DIR/packaging/macos"
RESOURCES="$WORK/resources"
/bin/mkdir -p "$RESOURCES"
/bin/cp "$MACOS_PKG_DIR/conclusion.html" "$RESOURCES/conclusion.html"

# Build distribution .pkg with conclusion panel
PKG_OUT="$CLI_DIR/dist/${NAME}_${VERSION}_darwin_${ARCH}.pkg"
mkdir -p "$CLI_DIR/dist"

/usr/bin/productbuild \
  --distribution "$MACOS_PKG_DIR/distribution.xml" \
  --resources "$RESOURCES" \
  --package-path "$WORK" \
  "$PKG_OUT"

echo ""
echo "[SUCCESS] .pkg built: $PKG_OUT"
echo "  Size: $(du -h "$PKG_OUT" | cut -f1)"
echo ""
echo "To test install:  sudo installer -pkg \"$PKG_OUT\" -target /"
echo "To double-click:  open \"$PKG_OUT\""
echo "To inspect:       pkgutil --payload-files \"$PKG_OUT\""
