#!/usr/bin/env bash
# Builds the itch.io / GitHub Pages ready web build into dist/web.
# Usage: scripts/build-web.sh [--zip]
set -euo pipefail

cd "$(dirname "$0")/.."
OUT=dist/web
rm -rf "$OUT"
mkdir -p "$OUT"

GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o "$OUT/game.wasm" ./cmd/game

GOROOT="$(go env GOROOT)"
WASM_EXEC="$GOROOT/lib/wasm/wasm_exec.js"          # Go 1.24+
[ -f "$WASM_EXEC" ] || WASM_EXEC="$GOROOT/misc/wasm/wasm_exec.js"  # older Go
cp "$WASM_EXEC" "$OUT/wasm_exec.js"
cp web/index.html "$OUT/index.html"

echo "Built $OUT ($(du -h "$OUT/game.wasm" | cut -f1) game.wasm)"

if [ "${1:-}" = "--zip" ]; then
  ZIP=dist/shift-command-web.zip
  rm -f "$ZIP"
  # index.html must be at the zip root for itch.io.
  (cd "$OUT" && zip -q -r "../$(basename "$ZIP")" .)
  echo "Zipped $ZIP"
fi
