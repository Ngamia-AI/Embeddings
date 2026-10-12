#!/usr/bin/env bash
# Verify downloaded model files against models/MANIFEST.sha256.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODELS_DIR="${MODELS_DIR:-$ROOT/models}"
MANIFEST="$MODELS_DIR/MANIFEST.sha256"

[ -f "$MANIFEST" ] || { echo "no manifest at $MANIFEST" >&2; exit 1; }

status=0
while read -r sum file _rest; do
  case "${sum:-}" in ""|\#*) continue ;; esac
  base="$(basename "$file")"
  [ -f "$MODELS_DIR/$base" ] || { echo "missing $base"; status=1; continue; }
  if echo "$sum  $MODELS_DIR/$base" | sha256sum -c - >/dev/null; then
    echo "ok   $base"
  else
    echo "FAIL $base"; status=1
  fi
done < "$MANIFEST"

exit "$status"
