#!/usr/bin/env bash
#
# One-command deploy for embeddings.ngamia.cc.
#
#   ./scripts/deploy.sh              # download/verify models if missing, then up
#   ./scripts/deploy.sh --skip-models
#   ./scripts/deploy.sh --video      # also start the Python video engine
#
# Reads deployments/.env, builds all images, starts the stack and waits for the
# gateway to become ready. Idempotent: re-run after a git pull to roll forward.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

ENV_FILE="$ROOT/deployments/.env"
COMPOSE=(docker compose -f "$ROOT/deployments/docker-compose.yml")
MODELS_DIR="${MODELS_DIR:-$ROOT/models}"
SKIP_MODELS=0
VIDEO=0

for arg in "$@"; do
  case "$arg" in
    --skip-models) SKIP_MODELS=1 ;;
    --video) VIDEO=1 ;;
    -h|--help) sed -n '2,10p' "$0"; exit 0 ;;
    *) echo "unknown option: $arg" >&2; exit 2 ;;
  esac
done

log()  { printf '\033[36m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[33m warn:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[31merror:\033[0m %s\n' "$*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || die "docker is not installed"
docker compose version >/dev/null 2>&1 || die "the docker compose plugin is not installed"

[ -f "$ENV_FILE" ] || die "missing $ENV_FILE (copy deployments/.env.example and edit)"
set -a; . "$ENV_FILE"; set +a

: "${DOMAIN:?set DOMAIN in deployments/.env}"
: "${ACME_EMAIL:?set ACME_EMAIL in deployments/.env}"
: "${ADMIN_TOKEN:?set ADMIN_TOKEN in deployments/.env}"
: "${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD in deployments/.env}"
[ "$ADMIN_TOKEN" = "change-me" ] && die "ADMIN_TOKEN is still the placeholder"
[ "$POSTGRES_PASSWORD" = "change-me" ] && die "POSTGRES_PASSWORD is still the placeholder"

log "target host: $DOMAIN"

if [ "${VIDEO_ENABLED:-false}" = "true" ]; then VIDEO=1; fi
PROFILE_ARGS=()
if [ "$VIDEO" = "1" ]; then
  PROFILE_ARGS=(--profile video)
  log "video engine: enabled"
fi

if [ "$SKIP_MODELS" = "0" ]; then
  if ls "$MODELS_DIR"/*.gguf >/dev/null 2>&1; then
    log "models present; verifying against MANIFEST.sha256"
    MODELS_DIR="$MODELS_DIR" "$ROOT/scripts/verify-models.sh"
  else
    log "downloading models into $MODELS_DIR (this takes a while)"
    MODELS_DIR="$MODELS_DIR" "$ROOT/scripts/download-models.sh"
  fi
else
  warn "skipping model download/verify"
fi

log "building and starting the stack"
"${COMPOSE[@]}" "${PROFILE_ARGS[@]}" --env-file "$ENV_FILE" up -d --build

log "waiting for the gateway (loopback :${GATEWAY_PORT:-8090}/readyz)"
ok=0
for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:${GATEWAY_PORT:-8090}/healthz" >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 2
done
if [ "$ok" = "1" ]; then
  log "gateway is up"
else
  warn "gateway did not answer /healthz yet; check: ${COMPOSE[*]} logs gateway"
fi

echo
"${COMPOSE[@]}" ps
echo
log "done. Public endpoint: https://$DOMAIN"
echo "  - first TLS issuance can take ~30s: ${COMPOSE[*]} logs -f caddy"
echo "  - smoke test:  curl -fsS https://$DOMAIN/v1/models -H 'Authorization: Bearer <key>'"
echo "  - create the first app/key: see docs/deployment.md section 6"
