# Changelog

Format: [Keep a Changelog](https://keepachangelog.com/); versioning: [SemVer](https://semver.org/).

## [0.1.0] - 2026-07-17

Initial release — a four-language collaborative edge-AI design canvas.

### Added
- **Rust→Wasm engine**: Ramer-Douglas-Peucker simplification and a geometric
  shape recogniser (line / rectangle / triangle / circle / arrow / freeform)
  that snaps rough strokes to ideal primitives. `wasm-bindgen` exports, native
  unit tests, and a `build-wasm.sh` for `wasm-pack`.
- **Go sync server**: a from-scratch RFC 6455 WebSocket implementation
  (handshake + frame codec, no third-party library) and a LWW-Element-Map CRDT
  with Lamport-clock conflict resolution, per-room fan-out, and snapshots for
  joiners. WebSocket and CRDT convergence tests.
- **Python asset service**: a from-scratch PNG encoder (zlib + CRC-32, no
  Pillow) driving a deterministic procedural generator, an async FastAPI
  `/generate`, an optional real text-to-image adapter, CLI, and tests.
- **Next.js canvas**: HTML5 drawing that offloads geometry to a Web Worker
  (Wasm-preferred, JS fallback), live multiplayer shapes and cursors over
  WebSocket, and an AI-asset panel.
- Shared protocol, docker-compose, per-service Dockerfiles, multi-language CI,
  Makefile, offline verifier, MIT license.
