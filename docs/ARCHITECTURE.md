# glyphmesh architecture

A collaborative vector canvas where AI runs on the *edge* — inside the browser —
and only heavy generation is offloaded. Each language owns its layer; one JSON
contract (`proto/protocol.md`) connects them.

```
                         browser tab
   ┌──────────────────────────────────────────────┐
   │ Canvas · TypeScript                            │
   │  pointer strokes ─▶ Web Worker                 │
   │                       │  (off the UI thread)   │
   │                       ▼                        │
   │                  Engine · Rust→Wasm            │
   │              RDP simplify + recognise          │
   │  cleaned shape ◀──────┘                        │
   └───────┬───────────────────────────┬────────────┘
           │ WebSocket /ws (ops+cursors)│ HTTP /generate
           ▼                            ▼
   ┌──────────────────┐        ┌──────────────────┐
   │ Sync · Go        │        │ Assets · Python  │
   │ RFC6455 + LWW    │        │ PNG + procedural │
   │ CRDT per room    │        │ generation       │
   └──────────────────┘        └──────────────────┘
```

## Why each language

| Layer | Language | Reason |
| --- | --- | --- |
| Canvas | **TypeScript** | The UI, HTML5 canvas, and Web Worker orchestration live here. |
| Engine | **Rust → Wasm** | Native-speed geometry inside the browser sandbox, off the UI thread. |
| Sync | **Go** | Cheap goroutines for many concurrent WebSocket collaborators. |
| Assets | **Python** | Where generative models live; here a from-scratch procedural stand-in. |

## Flow

1. The user draws a stroke. TypeScript streams the raw points to a **Web
   Worker** so the main thread keeps painting at full frame rate.
2. The worker runs the Rust engine (Wasm): **Ramer-Douglas-Peucker** simplifies
   the polyline, then a geometric recogniser measures whether it's closed, how
   round it is, and how many corners it has, and snaps it to the nearest ideal
   primitive (line, rectangle, triangle, circle, arrow) — the "AI autocomplete."
   If the wasm artifact isn't built, an identical TypeScript port runs instead.
3. The cleaned shape becomes a **ShapeOp** with a Lamport clock and replica id,
   is applied locally, and is sent to the Go sync server over a hand-rolled
   WebSocket.
4. The server merges the op into that room's **LWW-Element-Map** CRDT and fans
   it out to every other collaborator; new joiners receive a snapshot of the
   live shapes. Cursors are forwarded live but not stored.
5. When the user asks for a generated asset, the browser calls the Python
   service, which returns a base64 PNG built by a from-scratch encoder.

## Conflict resolution

Shapes form a LWW-Element-Map keyed by `id`. Each op carries `(lamport,
replica)`; on conflict the higher Lamport clock wins, ties break on the larger
replica id. Deletes are tombstone ops (`deleted: true`). Apply is commutative
and idempotent, so any two replicas that have seen the same ops converge to the
same document regardless of arrival order — the property test in
`sync-go/internal/crdt` checks exactly this.

## The WebSocket

`sync-go/internal/ws` implements RFC 6455 directly: the handshake computes
`Sec-WebSocket-Accept = base64(sha1(key + GUID))`, and the frame codec handles
the FIN/opcode byte, 7/16/64-bit payload lengths, and client-to-server masking
(the payload is XOR-ed with the 4-byte mask key). Server frames are unmasked.
This is the "efficient bitwise serialization protocol" with zero dependencies.

## Offline-first

- **Engine**: a JS fallback mirrors the Rust algorithms, so the canvas works
  before you run `build-wasm.sh`.
- **Sync**: no external WebSocket library — RFC 6455 by hand.
- **Assets**: no Pillow — a from-scratch PNG encoder + deterministic procedural
  generator. `GLYPHMESH_ASSET=openai` swaps in a real text-to-image backend.
