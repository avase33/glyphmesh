# glyphmesh 🎨

**A collaborative edge-AI design canvas.** You sketch a rough shape; a geometry
engine compiled to **WebAssembly** runs inside a Web Worker, snaps your stroke
to a clean primitive, and a from-scratch **CRDT** server syncs it to everyone in
the room. Heavy image generation runs as a background Python service.

Four languages, each on the layer it's built for:

```
       browser
   ┌───────────────┐     WebSocket /ws     ┌───────────────┐
   │ TS canvas     │ ◀───── ops/cursors ──▶ │ Go sync (CRDT)│
   │  + Web Worker │                        └───────────────┘
   │    ↓ wasm     │      HTTP /generate    ┌───────────────┐
   │ Rust engine   │ ─────────────────────▶ │ Python assets │
   └───────────────┘                        └───────────────┘
```

| Layer | Language | Owns |
| --- | --- | --- |
| **Canvas** | TypeScript | HTML5 drawing, Web Worker, WebSocket client, live cursors |
| **Engine** | Rust → Wasm | RDP simplification + shape recognition, native-speed, off-thread |
| **Sync** | Go | From-scratch WebSocket + LWW-Element-Map CRDT, per-room fan-out |
| **Assets** | Python | From-scratch PNG encoder + procedural (mock diffusion) generator |

Runs **offline** — no external WebSocket library (the server implements RFC 6455
by hand), no Pillow (a from-scratch PNG encoder), and a JS fallback so the
canvas works even before you build the wasm module.

## Quickstart — the pieces, offline

```bash
# Rust engine (RDP + recogniser), native tests
cd engine-rust && cargo test

# Go sync: WebSocket framing + CRDT convergence tests
cd sync-go && go test ./...

# Python assets: from-scratch PNG encoder + procedural generator
cd assets-python && pip install -e ".[dev]" && python -m glyphmesh_assets.cli demo --out asset.png
```

Offline end-to-end check:

```bash
python scripts/verify.py     # RESULT: N passed, 0 failed
```

## Quickstart — the whole app

```bash
# optional: build the real Rust->Wasm engine (falls back to JS without this)
cd engine-rust && ./build-wasm.sh && cd ..

docker compose up --build
# Canvas:  http://localhost:3000   (open two tabs to see live collaboration)
# Sync:    http://localhost:8080/healthz
# Assets:  http://localhost:8000/healthz
```

## The interesting engineering

- **RDP + shape recognition (Rust→Wasm)** — Ramer-Douglas-Peucker simplification
  then geometric classification (closed? round? corner count?) that snaps a
  stroke to a line, rectangle, triangle, circle, or arrow. `engine-rust/src/`
- **From-scratch WebSocket (Go)** — RFC 6455 handshake (`base64(sha1(key+GUID))`)
  and frame codec (FIN/opcode, 7/16/64-bit lengths, client mask XOR) with no
  third-party dependency. `sync-go/internal/ws/`
- **LWW-Element-Map CRDT (Go)** — commutative, idempotent, convergent shape
  merge keyed on `(lamport, replica)`; property-tested for order independence.
  `sync-go/internal/crdt/`
- **From-scratch PNG encoder (Python)** — IHDR/IDAT/IEND chunks, DEFLATE via
  `zlib`, CRC-32 per chunk; drives a deterministic procedural generator.
  `assets-python/glyphmesh_assets/png.py`

## Testing

```bash
make test                     # rust + go + python
cd engine-rust && cargo test
cd sync-go     && go test ./...
cd assets-python && pytest -q
cd canvas-ts   && npm run build
```

## Layout

```
proto/            wasm + sync + asset contracts
canvas-ts/        Next.js canvas, Web Worker, WebSocket client
engine-rust/      Rust->Wasm geometry + shape recogniser (+ build-wasm.sh)
sync-go/          Go CRDT sync server (hand-rolled WebSocket)
assets-python/    from-scratch PNG encoder + procedural asset generator
scripts/verify.py offline end-to-end check
docs/ARCHITECTURE.md
```

## License

MIT © 2026 Akhil Vase
