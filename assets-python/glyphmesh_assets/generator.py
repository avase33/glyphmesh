"""Procedural (offline) asset generation.

A prompt is hashed into a deterministic seed; that seed drives a vertical
gradient overlaid with translucent blobs — a stand-in for a diffusion model that
needs no GPU, no weights, and no network. Set ``GLYPHMESH_ASSET=openai`` to route
heavy generations to a real text-to-image backend instead.
"""

from __future__ import annotations

import base64
import hashlib
import os
import random
from typing import Dict

from .png import encode_png


def _seed_for(prompt: str, seed: int) -> int:
    digest = hashlib.sha256(f"{prompt}|{seed}".encode()).hexdigest()
    return int(digest[:16], 16)


def _rand_color(rng: random.Random) -> tuple[int, int, int]:
    return (rng.randint(25, 235), rng.randint(25, 235), rng.randint(25, 235))


def _procedural(prompt: str, seed: int, width: int, height: int) -> bytes:
    rng = random.Random(_seed_for(prompt, seed))
    top = _rand_color(rng)
    bot = _rand_color(rng)

    buf = bytearray(width * height * 3)
    for y in range(height):
        t = y / max(height - 1, 1)
        r = int(top[0] * (1 - t) + bot[0] * t)
        g = int(top[1] * (1 - t) + bot[1] * t)
        b = int(top[2] * (1 - t) + bot[2] * t)
        row = y * width * 3
        for x in range(width):
            i = row + x * 3
            buf[i] = r
            buf[i + 1] = g
            buf[i + 2] = b

    for _ in range(rng.randint(4, 9)):
        cx = rng.randint(0, width - 1)
        cy = rng.randint(0, height - 1)
        rad = rng.randint(12, max(13, width // 3))
        bc = _rand_color(rng)
        r2 = rad * rad
        for y in range(max(0, cy - rad), min(height, cy + rad)):
            dy2 = (y - cy) ** 2
            row = y * width * 3
            for x in range(max(0, cx - rad), min(width, cx + rad)):
                if (x - cx) ** 2 + dy2 <= r2:
                    i = row + x * 3
                    buf[i] = (buf[i] + bc[0]) // 2
                    buf[i + 1] = (buf[i + 1] + bc[1]) // 2
                    buf[i + 2] = (buf[i + 2] + bc[2]) // 2

    return bytes(buf)


def generate_asset(prompt: str, seed: int = 0, width: int = 256, height: int = 256) -> Dict:
    """Return an asset descriptor with a base64 PNG data URL."""
    width = max(16, min(width, 512))
    height = max(16, min(height, 512))

    backend = os.getenv("GLYPHMESH_ASSET", "procedural").lower()
    if backend == "openai":  # optional real adapter
        try:
            from .real import generate_openai

            return generate_openai(prompt, seed, width, height)
        except Exception:
            pass  # fall through to offline generation

    rgb = _procedural(prompt, seed, width, height)
    png = encode_png(width, height, rgb)
    data_url = "data:image/png;base64," + base64.b64encode(png).decode("ascii")
    return {
        "kind": "image/png",
        "data_url": data_url,
        "seed": _seed_for(prompt, seed) % 100000,
    }
