# Deployment runbook — `embeddings.ngamia.cc`

Target: the existing 8 GB RAM / 100 GB Contabo VPS. Single node, accepted as a
single point of failure for v1. The model weights are **downloaded on the VPS**
and never committed to git.

## 0. Prerequisites on the VPS

```bash
# Docker Engine + compose plugin, git, curl
docker --version && docker compose version
# Hugging Face CLI (downloads the GGUF files)
python3 -m pip install -U "huggingface_hub[cli]"
# No host nginx/certbot: Caddy runs in a container and handles TLS.
# ffmpeg is NOT needed on the host: it ships inside the gateway image.
```

Open ports `80` (ACME + redirect) and `443` in the VPS firewall.

DNS: create an `A` record for `$DOMAIN` (`embeddings.ngamia.cc`) -> the VPS IP.

## 1. Get the code

```bash
sudo mkdir -p /opt/embeddings && sudo chown "$USER" /opt/embeddings
git clone <repo-url> /opt/embeddings
cd /opt/embeddings/embeddings-api
```

## 2. Configure env

```bash
cd deployments && cp .env.example .env && $EDITOR .env
```

Set at minimum:

- `DOMAIN` and `ACME_EMAIL` — the public hostname and Let's Encrypt contact
- `ADMIN_ALLOW_CIDRS` — space-separated CIDRs allowed to call `/admin/*`
- `POSTGRES_PASSWORD` and `ADMIN_TOKEN` — `openssl rand -hex 32`

Pin the engine image to a llama.cpp build made **after 2026-10-06** (PR 30054,
`gemma-embedding2`). Prefer a digest:

```bash
# find and record the digest
docker pull ghcr.io/ggml-org/llama.cpp:server
docker inspect --format='{{index .RepoDigests 0}}' ghcr.io/ggml-org/llama.cpp:server
# then set in .env:
# LLAMA_CPP_IMAGE=ghcr.io/ggml-org/llama.cpp@sha256:<digest>
```

Record the chosen tag/digest in `context/decisions.md`.

## 3. Download and pin the models (on the VPS)

```bash
cd /opt/embeddings/embeddings-api
make models          # BF16 text + projector; also fetches nothing else
# For the Phase 0 comparison, also fetch Q8_0:
scripts/download-models.sh --q8
make verify-models
```

This writes `models/MANIFEST.sha256` with the real SHA-256 sums. The `embed`
container runs `deployments/engine-entrypoint.sh`, which verifies every listed
file before starting llama-server and **refuses to start on a mismatch**.

Files (see the SRS §2):

| file | repo | size |
| --- | --- | --- |
| `embeddinggemma-2-BF16.gguf` | `ggml-org/embeddinggemma-2-GGUF` | 558 MB |
| `mmproj-embeddinggemma-2-BF16.gguf` | `AtomicChat/embeddinggemma-2-GGUF` | 982 MB |
| `embeddinggemma-2-Q8_0.gguf` | `ggml-org/embeddinggemma-2-GGUF` | 310 MB |
| `mmproj-embeddinggemma-2-Q8_0.gguf` | `AtomicChat/embeddinggemma-2-GGUF` | 555 MB |

Pin exact revisions before production (`TEXT_REV`, `PROJ_REV` in the download
script or as env vars) and commit the resulting `MANIFEST.sha256`.

> The projector currently comes from `AtomicChat` because the `ggml-org` page
> listed text files only. Optionally regenerate it from `google/embeddinggemma-2`
> with llama.cpp's conversion script and compare tensors.

## 4. Start the stack (one command)

```bash
cd /opt/embeddings/embeddings-api
./scripts/deploy.sh          # models (if missing) + build + up + wait for ready
```

`deploy.sh` reads `deployments/.env`, downloads and verifies the models when they
are absent, builds every image, starts the stack and waits for the gateway. It is
idempotent — re-run after `git pull` to roll forward.

The stack is **fully containerized**: `postgres`, `embed` (llama.cpp), `gateway`
and `caddy` (automatic TLS). Only Caddy is published, on `80/443`. The gateway
listens on loopback `127.0.0.1:8090` for local smoke tests; the engine and the
optional video engine have no public port. Migrations run at gateway startup.

```bash
make ps
make logs
make smoke                   # curl 127.0.0.1:8090/readyz
docker compose -f deployments/docker-compose.yml logs -f caddy
```

Verify the engine is actually loading the model before moving on:

```bash
docker compose -f deployments/docker-compose.yml logs embed | tail
curl -fsS http://127.0.0.1:8090/readyz   # exercises text + image embed
```

## 5. TLS (Caddy, automatic)

Caddy provisions and renews the Let's Encrypt certificate for `$DOMAIN` on its
own — no host nginx, no certbot. Prerequisites:

- DNS `A`/`AAAA` record for `$DOMAIN` pointing at the VPS.
- Ports **80 and 443** open (80 is needed for the ACME HTTP challenge).

Set `DOMAIN` and `ACME_EMAIL` in `deployments/.env`, then:

```bash
docker compose -f deployments/docker-compose.yml logs -f caddy   # watch issuance
curl -fsS https://$DOMAIN/readyz
```

`/metrics` returns 404 publicly. `/admin/*` is restricted to `ADMIN_ALLOW_CIDRS`
(space-separated CIDRs in `.env`); everything else is proxied to the gateway.

> Prefer host nginx? Keep `docker compose` for app/db/engines only and use the
> sample in `deployments/nginx/embeddings.ngamia.cc.conf` (add `--profile` none,
> remove the `caddy` service). Caddy is the supported default.

## 5b. Enable the Python video engine (optional, Phase 8)

```bash
# in deployments/.env
VIDEO_ENABLED=true
./scripts/deploy.sh --video      # or: make engine-py-up
```

The video engine has no public port and is memory-capped at 3 GB. See §12 for the
full flow and the SRS Phase 8 gate before enabling it in production.

## 6. Create the first app and key

```bash
ADMIN=... # your ADMIN_TOKEN
curl -s https://embeddings.ngamia.cc/admin/v1/apps \
  -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' \
  -d '{"name":"example-app","modalities":["text","image"],"rps":10,"burst":20,"media_rps":2,"max_batch":32,"max_media":8}' | jq .
# take data.id, then:
curl -s https://embeddings.ngamia.cc/admin/v1/apps/<id>/keys \
  -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' \
  -d '{"class":"live"}' | jq -r .data.key   # shown once
```

## 7. Smoke test from outside

```bash
KEY=emb_live_...
curl -s https://embeddings.ngamia.cc/v1/embeddings \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"model":"embeddinggemma-2","input":"bei ya mchele Kariakoo"}' | jq '.data[0].embedding | length'
```

Also test one image and one voice note per `context/api-contracts.md`. Confirm
the engine port is unreachable from outside:

```bash
nmap -p 8080 <vps-ip>   # closed/filtered
```

## 8. Operations

- Logs: `make logs` (JSON, no content).
- Metrics: `curl -s http://127.0.0.1:8090/metrics` (bind locally; alert when
  `/readyz` fails).
- Retention: raw events are purged hourly after `RAW_EVENT_RETENTION`
  (default 30 days); daily rollups are kept indefinitely.
- Backups: dump the `pgdata` volume; models are re-downloadable.

## 9. Rollback

```bash
# keep the previous gateway image tag and engine tag/digest in .env history
docker compose -f deployments/docker-compose.yml --env-file deployments/.env up -d
# the models volume is untouched
```

## 10. RAM budget

```text
postgres        512 MB
embed (llama)    3 GB
gateway         512 MB
                ------
always-on       4 GB

embed-py         3 GB   (only with VIDEO_ENABLED=true)
```

Keep the Python video engine **off** unless it is being benchmarked or the RAM
budget has been raised: with it running, the stack can want ~7 GB. Alert when
`/readyz` fails; measure actual RSS under load and record it in
`docs/benchmark.md` and `context/decisions.md`.

## 11. Security checklist

- [ ] TLS only; `/metrics` denied; `/admin/` allow-listed.
- [ ] Keys hashed at rest; `ADMIN_TOKEN` high-entropy, not in git.
- [ ] Containers non-root; gateway `read_only` + tmpfs; no-new-privileges.
- [ ] Engine has no public port.
- [ ] Video engine (when enabled) has no public port and is memory-capped.
- [ ] Caddy publishes only 80/443; `/metrics` returns 404; `/admin/*` limited
      to `ADMIN_ALLOW_CIDRS`.
- [ ] Images decoded in Go; audio through ffmpeg with a timeout and pipe-only
      protocol; body capped at 25 MB at Caddy and the gateway.

## 12. Video engine (Phase 8, Path B)

Video is the last modality and ships behind a flag. The gateway never embeds
video itself; when enabled it registers the Python `sentence-transformers`
engine and routes any item containing a `video_url` part to it. The SRS gate is
that video stays off until it passes parity, a 58 s clip, and the RAM budget.

Do **not** enable video until `docs/benchmark.md` records those three results.

Start the Python engine (CPU-only, 3 GB cap, no public port):

```bash
cd /opt/embeddings/embeddings-api
# first start downloads weights into the hfmodels volume (several GB)
docker compose -f deployments/docker-compose.yml --profile video up -d embed-py
docker compose -f deployments/docker-compose.yml logs -f embed-py   # wait for /health ok
```

Then flip the flag so the gateway routes video to it:

```bash
# in deployments/.env
VIDEO_ENABLED=true
docker compose -f deployments/docker-compose.yml up -d gateway
```

Grant the `video` modality to the app that needs it, then test with a short
clip (base64 data URI, <= `VIDEO_MAX_BYTES`, and <= 58 s so frames stay in
context):

```bash
curl -s https://embeddings.ngamia.cc/v1/embeddings \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"model":"embeddinggemma-2","input":[{"content":[{"type":"video_url","video_url":{"url":"data:video/mp4;base64,<...>"}}]}]}' \
  | jq '.usage'
```

How the Python engine embeds a clip: it samples frames at `FRAMES_PER_SECOND`
(1), extracts at most `MAX_VIDEO_SECONDS` (58), encodes each frame with the
vision encoder, then mean-pools and L2-normalizes to one vector per item. It
uses `float32` (default) or `bfloat16` (`EMBED_PY_BF16=true`); **fp16 is never
allowed**. The gateway still owns prefixes, truncation and re-normalization, so
the public vector is identical in shape to every other modality.

To roll video back out, set `VIDEO_ENABLED=false` and stop the service:

```bash
docker compose -f deployments/docker-compose.yml --profile video stop embed-py
```
