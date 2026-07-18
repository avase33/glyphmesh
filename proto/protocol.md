# glyphmesh wire protocol

Two boundaries: the **Wasm boundary** (TypeScript ↔ Rust, in-process) and the
**sync boundary** (browsers ↔ Go server, over WebSocket). Plus a small HTTP
contract to the Python asset service.

## 1. Wasm boundary (TS ↔ Rust)

A stroke is a flat `[x0, y0, x1, y1, …]` array of `f64`.

- `simplify(points: Float64Array, epsilon: f64) -> Float64Array`
  Ramer-Douglas-Peucker line simplification. Returns a flat array.
- `autocomplete(points: Float64Array) -> string` (JSON):

```json
{
  "kind": "rectangle",
  "confidence": 0.86,
  "points": [x0, y0, x1, y1, x2, y2, x3, y3],
  "closed": true
}
```

`kind` ∈ `line | rectangle | triangle | circle | arrow | freeform`. `points`
is the *idealised* shape the engine suggests (a perfect rectangle from the
bounding box, a regular circle polygon, a straight line, …).

## 2. Sync boundary (browser ↔ Go, WebSocket `/ws?room=<id>&replica=<id>`)

Every message is one JSON object with a `type`.

Client → server / server → clients:

```json
{ "type": "op",     "room": "r1", "op": { ...ShapeOp... } }
{ "type": "cursor", "room": "r1", "replica": "a3f", "x": 220.0, "y": 140.0 }
```

Server → a newly joined client (state snapshot):

```json
{ "type": "snapshot", "room": "r1", "ops": [ ...ShapeOp... ] }
```

### ShapeOp (the CRDT unit)

```json
{
  "id": "s_0007",
  "replica": "a3f",
  "lamport": 42,
  "deleted": false,
  "kind": "rectangle",
  "points": [100, 100, 300, 220],
  "color": "#58a6ff"
}
```

Conflict resolution is **last-writer-wins** on the pair `(lamport, replica)`:
the op with the higher Lamport clock wins; ties break on the lexicographically
larger `replica` id. A tombstone is a normal op with `deleted: true`. This is a
LWW-Element-Map keyed by `id` — commutative, idempotent, convergent.

## 3. Asset service (browser → Python, HTTP)

`POST /generate`

```json
{ "prompt": "sunset over mountains", "seed": 0, "width": 256, "height": 256 }
```

→

```json
{ "kind": "image/png", "data_url": "data:image/png;base64,...", "seed": 771 }
```

Offline this is a deterministic **procedural** generator (a from-scratch PNG
encoder); `GLYPHMESH_ASSET=openai` swaps in a real text-to-image backend.

## Ports

| service | port | protocol |
| --- | --- | --- |
| Go sync | 8080 | WebSocket `/ws`, HTTP `/healthz` |
| Python assets | 8000 | HTTP `/generate` `/healthz` |
| TS canvas | 3000 | HTTP |
