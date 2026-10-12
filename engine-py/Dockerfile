# Python video engine (Phase 8, Path B).
#
# CPU-only sentence-transformers service. Model weights are NOT baked in; they
# are downloaded to a mounted volume (see docs/deployment.md).
FROM python:3.12-slim

ENV PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    HF_HOME=/models/hf \
    TRANSFORMERS_OFFLINE=0

RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# CPU wheels keep the image small and avoid pulling CUDA.
RUN pip install torch --index-url https://download.pytorch.org/whl/cpu

COPY requirements.txt ./
RUN pip install -r requirements.txt

COPY app ./app

RUN useradd --uid 10002 --create-home --shell /usr/sbin/nologin engine \
    && mkdir -p /models/hf \
    && chown -R engine:engine /app /models
USER 10001:10001

ENV PORT=8080 \
    MODEL_ID=google/embeddinggemma-2 \
    FRAMES_PER_SECOND=1 \
    MAX_VIDEO_SECONDS=58 \
    TORCH_THREADS=0

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=180s --retries=3 \
    CMD python -c "import urllib.request,sys; s=__import__('json').loads(__import__('urllib.request',fromlist=['urlopen']).urlopen('http://127.0.0.1:8080/health').read()); exit(0 if s.get('status')=='ok' else 1)"

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8080", "--workers", "1"]
