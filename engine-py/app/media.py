"""Media decoding helpers: data URIs, images and video frame extraction."""

from __future__ import annotations

import base64
import binascii
import glob
import os
import subprocess
import tempfile
from dataclasses import dataclass

from PIL import Image

from . import config


class MediaError(Exception):
    """Raised when a media part cannot be decoded."""


@dataclass
class DecodedDataURI:
    mime: str
    data: bytes


def parse_data_uri(uri: str) -> DecodedDataURI:
    """Decode a base64 data URI or bare base64 string."""
    if not uri:
        raise MediaError("empty media payload")
    raw = uri.strip()
    mime = ""
    if raw.startswith("data:"):
        try:
            header, _, payload = raw.partition(",")
        except ValueError as exc:  # pragma: no cover - partition never raises
            raise MediaError("malformed data URI") from exc
        if ";base64" not in header:
            raise MediaError("only base64 data URIs are supported")
        mime = header[len("data:") :].split(";", 1)[0]
        raw = payload
    try:
        return DecodedDataURI(mime=mime, data=base64.b64decode(raw, validate=False))
    except (binascii.Error, ValueError) as exc:
        raise MediaError("invalid base64 payload") from exc


def decode_image(uri: str) -> Image.Image:
    item = parse_data_uri(uri)
    try:
        buf = tempfile.SpooledTemporaryFile(max_size=8 * 1024 * 1024)
        buf.write(item.data)
        buf.seek(0)
        img = Image.open(buf)
        return img.convert("RGB")
    except OSError as exc:
        raise MediaError("image could not be decoded") from exc


def extract_frames(uri: str) -> list[Image.Image]:
    """Extract up to MAX_FRAMES frames at FPS from a base64 video data URI."""
    item = parse_data_uri(uri)
    if not item.data:
        raise MediaError("empty video payload")
    if not _is_video(item.mime, item.data):
        raise MediaError("unsupported video container")

    with tempfile.TemporaryDirectory(prefix="emb-video-") as tmp:
        src = os.path.join(tmp, "in")
        with open(src, "wb") as fh:
            fh.write(item.data)
        pattern = os.path.join(tmp, "frame-%05d.jpg")
        cmd = [
            config.FFMPEG_PATH,
            "-nostdin",
            "-hide_banner",
            "-loglevel",
            "error",
            "-i",
            src,
            "-t",
            str(config.MAX_VIDEO_SECONDS),
            "-vf",
            f"fps={config.FPS}",
            "-q:v",
            "3",
            pattern,
        ]
        try:
            subprocess.run(
                cmd,
                check=True,
                timeout=config.FFMPEG_TIMEOUT,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.PIPE,
            )
        except subprocess.TimeoutExpired as exc:
            raise MediaError("video decode timed out") from exc
        except subprocess.CalledProcessError as exc:
            raise MediaError("video could not be decoded") from exc

        frames = []
        for path in sorted(glob.glob(os.path.join(tmp, "frame-*.jpg")))[: config.MAX_FRAMES]:
            frames.append(Image.open(path).convert("RGB"))
        if not frames:
            raise MediaError("video contained no decodable frames")
        return frames


def _is_video(mime: str, data: bytes) -> bool:
    if mime.startswith("video/"):
        return True
    if len(data) >= 12 and data[4:8] == b"ftyp":
        return True
    if len(data) >= 4 and data[0:4] == b"\x1a\x45\xdf\xa3":
        return True
    return False
