#!/usr/bin/env bash
set -euo pipefail

# Builds a self-contained release for a non-Docker deploy: a static Go
# binary (cross-compiled for GOOS/GOARCH, default linux/amd64) plus the
# built frontend, packaged as one tarball ready to scp to a VPS.
#
# No Docker or CGO cross-compiler needed — the binary is already fully
# static (pure-Go SQLite driver, CGO_ENABLED=0, see ADR 0003), so this
# cross-compiles cleanly from any machine with Go and Node installed.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="$ROOT_DIR/release"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

echo "==> building frontend"
(cd "$ROOT_DIR/web" && npm ci && npm run build)
cp -r "$ROOT_DIR/web/dist" "$OUT_DIR/dist"

echo "==> building server ($GOOS/$GOARCH)"
(cd "$ROOT_DIR/server" && CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -o "$OUT_DIR/retainer" ./cmd/retainer)

TARBALL="$ROOT_DIR/retainer-$GOOS-$GOARCH.tar.gz"
echo "==> packaging $TARBALL"
tar -C "$OUT_DIR" -czf "$TARBALL" retainer dist

echo "==> done"
echo "    scp $TARBALL to your VPS, then follow the 'Deploy without Docker' section of README.md"
