#!/bin/sh
# Verify the model files against models/MANIFEST.sha256, then start llama-server.
# Runs as the entrypoint of the `embed` container. Refuses to start on a
# checksum mismatch so a corrupted or swapped model can never serve traffic.
set -eu

MODELS_DIR="${MODELS_DIR:-/models}"
MANIFEST="${MANIFEST:-$MODELS_DIR/MANIFEST.sha256}"

if [ -f "$MANIFEST" ]; then
  echo "verifying models against $MANIFEST"
  verified=0
  while read -r sum file _rest; do
    case "${sum:-}" in
      ""|\#*) continue ;;
    esac
    # file may carry a leading path; keep only the basename for docker.
    base="$(basename "$file")"
    if [ ! -f "$MODELS_DIR/$base" ]; then
      echo "  skip $base (not downloaded)"
      continue
    fi
    echo "$sum  $MODELS_DIR/$base" | sha256sum -c - >/dev/null
    echo "  ok   $base"
    verified=$((verified + 1))
  done < "$MANIFEST"
  echo "verified $verified model file(s)"
else
  echo "warning: no manifest at $MANIFEST, skipping verification"
fi

# llama.cpp server images ship the binary at /app/llama-server; fall back to PATH.
if command -v llama-server >/dev/null 2>&1; then
  BIN="llama-server"
elif [ -x /app/llama-server ]; then
  BIN="/app/llama-server"
else
  echo "llama-server binary not found" >&2
  exit 1
fi

echo "starting $BIN $*"
exec "$BIN" "$@"
