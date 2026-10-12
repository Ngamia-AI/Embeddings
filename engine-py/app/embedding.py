"""Model wrapper around sentence-transformers for text, image and video."""

from __future__ import annotations

import logging
from dataclasses import dataclass, field
from typing import Any

import numpy as np

from . import config
from .media import MediaError, decode_image, extract_frames

log = logging.getLogger("engine.embedding")


@dataclass
class Usage:
    prompt_tokens: int = 0
    total_tokens: int = 0
    images: int = 0
    video_seconds: float = 0.0

    def add(self, other: "Usage") -> None:
        self.prompt_tokens += other.prompt_tokens
        self.total_tokens += other.total_tokens
        self.images += other.images
        self.video_seconds += other.video_seconds


@dataclass
class EmbedResult:
    vectors: list[list[float]] = field(default_factory=list)
    usage: Usage = field(default_factory=Usage)


class EmbeddingError(Exception):
    """Raised for input the model cannot embed."""


class Embedder:
    def __init__(self) -> None:
        self.model = None
        self.dtype = None
        self.device = config.DEVICE

    def load(self) -> None:
        import torch
        from sentence_transformers import SentenceTransformer

        if config.TORCH_THREADS > 0:
            torch.set_num_threads(config.TORCH_THREADS)

        dtype = torch.float32
        if config.DTYPE == "bfloat16":
            dtype = torch.bfloat16
        if dtype == torch.float16:  # defensive: never allowed
            dtype = torch.float32
        self.dtype = dtype

        log.info("loading model %s device=%s dtype=%s", config.MODEL_ID, self.device, dtype)
        self.model = SentenceTransformer(
            config.MODEL_ID,
            device=self.device,
            trust_remote_code=True,
            model_kwargs={"torch_dtype": dtype},
        )
        self.model.eval()

    # -- encoders ---------------------------------------------------------

    def _encode(self, items: list[Any]) -> np.ndarray:
        out = self.model.encode(
            items,
            batch_size=config.BATCH_SIZE,
            normalize_embeddings=True,
            convert_to_numpy=True,
            show_progress_bar=False,
        )
        return np.asarray(out, dtype=np.float32)

    def encode_texts(self, texts: list[str]) -> np.ndarray:
        return self._encode(list(texts))

    def encode_images(self, images: list[Any]) -> np.ndarray:
        return self._encode(list(images))

    # -- item pipeline ----------------------------------------------------

    def embed_items(self, items: list[list[dict]]) -> EmbedResult:
        result = EmbedResult()
        for parts in items:
            vector, usage = self.embed_item(parts)
            result.vectors.append(vector)
            result.usage.add(usage)
        return result

    def embed_item(self, parts: list[dict]) -> tuple[list[float], Usage]:
        if not parts:
            raise EmbeddingError("empty item")

        part_vectors: list[np.ndarray] = []
        usage = Usage()

        for part in parts:
            kind = part.get("type")
            if kind == "text":
                text = part.get("text") or ""
                part_vectors.append(self.encode_texts([text])[0])
                n = self.count_text_tokens(text)
                usage.prompt_tokens += n
                usage.total_tokens += n
            elif kind in ("image_url", "input_image"):
                uri = _media_url(part, kind)
                image = decode_image(uri)
                part_vectors.append(self.encode_images([image])[0])
                usage.images += 1
                usage.prompt_tokens += config.TOKENS_PER_IMAGE
                usage.total_tokens += config.TOKENS_PER_IMAGE
            elif kind == "video_url":
                uri = _media_url(part, kind)
                frames = extract_frames(uri)
                frame_vecs = self.encode_images(frames)
                part_vectors.append(_pool(frame_vecs))
                seconds = len(frames) / config.FPS
                usage.video_seconds += seconds
                tokens = len(frames) * config.TOKENS_PER_FRAME
                usage.prompt_tokens += tokens
                usage.total_tokens += tokens
            elif kind == "input_audio":
                raise EmbeddingError("audio is not supported by the Python engine")
            else:
                raise EmbeddingError(f"unsupported part type: {kind}")

        combined = _pool(np.vstack(part_vectors))
        return combined.tolist(), usage

    def count_text_tokens(self, text: str) -> int:
        tok = getattr(self.model, "tokenizer", None)
        if tok is not None:
            try:
                return len(tok.encode(text, add_special_tokens=True))
            except Exception:  # pragma: no cover - tokenizer quirks
                pass
        return max(1, len(text) // 4)


def _media_url(part: dict, kind: str) -> str:
    node = part.get(kind)
    if isinstance(node, str):
        return node
    if isinstance(node, dict):
        return node.get("url") or node.get("data") or ""
    return ""


def _pool(vectors: np.ndarray) -> np.ndarray:
    """Mean-pool L2-normalized vectors and renormalize to unit length."""
    if vectors.ndim == 1:
        vectors = vectors.reshape(1, -1)
    mean = vectors.mean(axis=0)
    norm = np.linalg.norm(mean)
    if norm == 0:
        return mean
    return mean / norm
