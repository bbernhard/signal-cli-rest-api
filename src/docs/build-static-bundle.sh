#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCS_DIR="$ROOT_DIR/src/docs"
INDEX_FILE="$DOCS_DIR/index.html"
OUT_DIR="${1:?output directory argument is required}"
DOCS_VERSION="${2:?docs version argument is required}"
NORMALIZED_DOCS_VERSION="${DOCS_VERSION#v}"

echo "Normalized docs version: $NORMALIZED_DOCS_VERSION"

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

cp "$DOCS_DIR/swagger.json" "$OUT_DIR/swagger.json"
cp "$INDEX_FILE" "$OUT_DIR/index.html"
touch "$OUT_DIR/.nojekyll"

jq --arg version "$NORMALIZED_DOCS_VERSION" '.info.version = $version' "$OUT_DIR/swagger.json" > "$OUT_DIR/swagger.json.tmp"
mv "$OUT_DIR/swagger.json.tmp" "$OUT_DIR/swagger.json"

echo "Static bundle created at: $OUT_DIR"
