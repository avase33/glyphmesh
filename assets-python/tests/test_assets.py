import base64
import struct
import zlib

from glyphmesh_assets.generator import generate_asset
from glyphmesh_assets.png import encode_png

PNG_SIG = b"\x89PNG\r\n\x1a\n"


def _read_chunks(data):
    assert data[:8] == PNG_SIG
    i = 8
    chunks = []
    while i < len(data):
        (length,) = struct.unpack(">I", data[i : i + 4])
        tag = data[i + 4 : i + 8]
        body = data[i + 8 : i + 8 + length]
        crc = struct.unpack(">I", data[i + 8 + length : i + 12 + length])[0]
        assert crc == (zlib.crc32(tag + body) & 0xFFFFFFFF), f"bad CRC in {tag}"
        chunks.append((tag, body))
        i += 12 + length
    return chunks


def test_encode_png_is_valid():
    w, h = 4, 3
    rgb = bytes([10, 20, 30] * (w * h))
    png = encode_png(w, h, rgb)
    chunks = _read_chunks(png)
    tags = [t for t, _ in chunks]
    assert tags[0] == b"IHDR"
    assert tags[-1] == b"IEND"
    assert b"IDAT" in tags
    # IHDR encodes the right dimensions
    ihdr = dict(chunks)[b"IHDR"]
    ww, hh = struct.unpack(">II", ihdr[:8])
    assert (ww, hh) == (w, h)


def test_encode_png_roundtrips_pixels():
    w, h = 2, 2
    rgb = bytes([1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12])
    png = encode_png(w, h, rgb)
    idat = dict(_read_chunks(png))[b"IDAT"]
    raw = zlib.decompress(idat)
    # each row: 1 filter byte + w*3 colour bytes
    assert raw[0] == 0 and raw[1:7] == rgb[0:6]
    assert raw[7] == 0 and raw[8:14] == rgb[6:12]


def test_generate_is_deterministic():
    a = generate_asset("a cat", seed=1, width=64, height=64)
    b = generate_asset("a cat", seed=1, width=64, height=64)
    c = generate_asset("a dog", seed=1, width=64, height=64)
    assert a["data_url"] == b["data_url"]
    assert a["data_url"] != c["data_url"]
    # payload must be a decodable PNG
    payload = base64.b64decode(a["data_url"].split(",", 1)[1])
    assert payload[:8] == PNG_SIG
