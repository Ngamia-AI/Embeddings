"""Configuration for the Python video engine, sourced from the environment."""

import os


def _env(name: str, default: str) -> str:
    v = os.environ.get(name)
    return v if v not in (None, "") else default


def _int(name: str, default: int) -> int:
    try:
        return int(_env(name, str(default)))
    except ValueError:
        return default


def _float(name: str, default: float) -> float:
    try:
        return float(_env(name, str(default)))
    except ValueError:
        return default


def _bool(name: str, default: bool) -> bool:
    v = os.environ.get(name)
    if v is None:
        return default
    return v.strip().lower() in ("1", "true", "yes", "on")


MODEL_ID = _env("MODEL_ID", "google/embeddinggemma-2")

# fp16 is forbidden by the SRS. Only bfloat16 (opt-in) and float32 are allowed.
DEVICE = "cuda" if _bool("USE_CUDA", False) else "cpu"
DTYPE = "bfloat16" if _bool("BF16", False) else "float32"

FFMPEG_PATH = _env("FFMPEG_PATH", "ffmpeg")
FPS = _float("FRAMES_PER_SECOND", 1.0)
MAX_VIDEO_SECONDS = _float("MAX_VIDEO_SECONDS", 58.0)
MAX_FRAMES = max(1, int(MAX_VIDEO_SECONDS * FPS))

TOKENS_PER_IMAGE = 280
TOKENS_PER_FRAME = 140
TOKENS_PER_AUDIO_SEC = 25

BATCH_SIZE = _int("BATCH_SIZE", 4)
TORCH_THREADS = _int("TORCH_THREADS", 0)  # 0 lets torch decide
FFMPEG_TIMEOUT = _int("FFMPEG_TIMEOUT", 120)
