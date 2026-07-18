"""A minimal, dependency-free PNG encoder (8-bit RGB).

Uses only the standard library: ``struct`` for chunk framing, ``zlib`` for the
IDAT DEFLATE stream and the CRC-32 that every PNG chunk carries. No Pillow.
"""

from __future__ import annotations

import struct
import zlib

_SIGNATURE = b"\x89PNG\r\n\x1a\n"


def _chunk(tag: bytes, data: bytes) -> bytes:
    crc = zlib.crc32(tag + data) & 0xFFFFFFFF
    return struct.pack(">I", len(data)) + tag + data + struct.pack(">I", crc)


def encode_png(width: int, height: int, rgb: bytes) -> bytes:
    """Encode a row-major ``width*height*3`` byte buffer as a PNG image."""
    if len(rgb) != width * height * 3:
        raise ValueError("rgb buffer size does not match width*height*3")

    # IHDR: 8-bit depth, colour type 2 (truecolour RGB), no interlace
    ihdr = struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)

    # each scanline is prefixed with filter type 0 (None)
    stride = width * 3
    raw = bytearray()
    for y in range(height):
        raw.append(0)
        raw.extend(rgb[y * stride : (y + 1) * stride])

    idat = zlib.compress(bytes(raw), 9)
    return _SIGNATURE + _chunk(b"IHDR", ihdr) + _chunk(b"IDAT", idat) + _chunk(b"IEND", b"")
