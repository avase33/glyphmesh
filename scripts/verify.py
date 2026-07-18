#!/usr/bin/env python3
"""Offline end-to-end verifier for glyphmesh's Python asset service.

Encodes a PNG with the from-scratch encoder, verifies its structure and CRCs,
and confirms procedural generation is deterministic — no services, no Pillow.
"""

import struct
import sys
import zlib
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "assets-python"))

PASS, FAIL = 0, 0
PNG_SIG = b"\x89PNG\r\n\x1a\n"


def check(name, cond):
    global PASS, FAIL
    if cond:
        PASS += 1
        print(f"  ok   {name}")
    else:
        FAIL += 1
        print(f"  FAIL {name}")


def chunks(data):
    i, out = 8, []
    while i < len(data):
        (length,) = struct.unpack(">I", data[i : i + 4])
        tag = data[i + 4 : i + 8]
        body = data[i + 8 : i + 8 + length]
        crc = struct.unpack(">I", data[i + 8 + length : i + 12 + length])[0]
        out.append((tag, body, crc == (zlib.crc32(tag + body) & 0xFFFFFFFF)))
        i += 12 + length
    return out


def main() -> int:
    from glyphmesh_assets.generator import generate_asset
    from glyphmesh_assets.png import encode_png

    print("glyphmesh offline verify")

    png = encode_png(8, 6, bytes([120, 60, 200] * 48))
    check("PNG signature", png[:8] == PNG_SIG)
    cs = chunks(png)
    tags = [t for t, _, _ in cs]
    check("IHDR first", tags[0] == b"IHDR")
    check("IEND last", tags[-1] == b"IEND")
    check("all chunk CRCs valid", all(ok for _, _, ok in cs))

    import base64

    a = generate_asset("mountains", seed=1, width=48, height=48)
    b = generate_asset("mountains", seed=1, width=48, height=48)
    c = generate_asset("ocean", seed=1, width=48, height=48)
    check("deterministic for same prompt", a["data_url"] == b["data_url"])
    check("differs for different prompt", a["data_url"] != c["data_url"])
    payload = base64.b64decode(a["data_url"].split(",", 1)[1])
    check("asset decodes to a PNG", payload[:8] == PNG_SIG)

    print(f"\nRESULT: {PASS} passed, {FAIL} failed")
    return 1 if FAIL else 0


if __name__ == "__main__":
    raise SystemExit(main())
