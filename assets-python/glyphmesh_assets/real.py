"""Optional real text-to-image adapter (used only when GLYPHMESH_ASSET=openai).

Kept import-light: the OpenAI SDK is imported lazily so the offline path never
depends on it.
"""

from __future__ import annotations

import base64
import os
from typing import Dict


def generate_openai(prompt: str, seed: int, width: int, height: int) -> Dict:
    from openai import OpenAI  # lazy import

    client = OpenAI(api_key=os.environ["OPENAI_API_KEY"])
    size = f"{max(width, 256)}x{max(height, 256)}"
    resp = client.images.generate(
        model=os.getenv("GLYPHMESH_IMAGE_MODEL", "gpt-image-1"),
        prompt=prompt,
        size=size,
        n=1,
    )
    b64 = resp.data[0].b64_json
    if not b64:
        raise RuntimeError("no image returned")
    # normalise to a data URL like the offline path
    _ = base64.b64decode(b64)  # validate
    return {
        "kind": "image/png",
        "data_url": "data:image/png;base64," + b64,
        "seed": seed,
    }
