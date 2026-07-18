/// <reference lib="webworker" />
//
// Web Worker that runs the geometry engine off the UI thread. It prefers the
// Rust->Wasm build in /engine (produced by engine-rust/build-wasm.sh) and falls
// back to this pure-TS port of the same algorithms when the wasm artifact is
// absent — so the app works whether or not you've run the wasm build.

type Recognition = {
  kind: string;
  confidence: number;
  points: number[];
  closed: boolean;
};

type Pt = { x: number; y: number };

const CORNER_MAX_ANGLE = 2.4;

let wasm: {
  simplify: (pts: Float64Array, eps: number) => Float64Array;
  autocomplete: (pts: Float64Array) => string;
} | null = null;

// Try to upgrade to the real Rust->Wasm engine; ignore failures.
(async () => {
  try {
    // A variable specifier keeps TypeScript from trying to resolve the module
    // at build time, and webpackIgnore keeps the bundler from bundling it.
    const spec = "/engine/glyphmesh_engine.js";
    const mod: any = await import(/* webpackIgnore: true */ spec);
    await mod.default();
    wasm = { simplify: mod.simplify, autocomplete: mod.autocomplete };
  } catch {
    wasm = null; // stay on the JS fallback
  }
})();

function toPts(flat: number[]): Pt[] {
  const out: Pt[] = [];
  for (let i = 0; i + 1 < flat.length; i += 2) out.push({ x: flat[i], y: flat[i + 1] });
  return out;
}
function toFlat(pts: Pt[]): number[] {
  const out: number[] = [];
  for (const p of pts) out.push(p.x, p.y);
  return out;
}
function dist(a: Pt, b: Pt): number {
  return Math.hypot(a.x - b.x, a.y - b.y);
}
function perp(p: Pt, a: Pt, b: Pt): number {
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  const len = Math.hypot(dx, dy);
  if (len < 1e-12) return dist(p, a);
  return Math.abs((p.x - a.x) * dy - (p.y - a.y) * dx) / len;
}
function rdp(pts: Pt[], eps: number): Pt[] {
  if (pts.length < 3) return pts.slice();
  const a = pts[0];
  const b = pts[pts.length - 1];
  let maxD = 0;
  let idx = 0;
  for (let i = 1; i < pts.length - 1; i++) {
    const d = perp(pts[i], a, b);
    if (d > maxD) {
      maxD = d;
      idx = i;
    }
  }
  if (maxD > eps) {
    const left = rdp(pts.slice(0, idx + 1), eps);
    const right = rdp(pts.slice(idx), eps);
    left.pop();
    return left.concat(right);
  }
  return [a, b];
}
function turn(a: Pt, b: Pt, c: Pt): number {
  const v1 = { x: a.x - b.x, y: a.y - b.y };
  const v2 = { x: c.x - b.x, y: c.y - b.y };
  const dot = v1.x * v2.x + v1.y * v2.y;
  const m1 = Math.hypot(v1.x, v1.y);
  const m2 = Math.hypot(v2.x, v2.y);
  if (m1 < 1e-9 || m2 < 1e-9) return Math.PI;
  return Math.acos(Math.max(-1, Math.min(1, dot / (m1 * m2))));
}

function recognizeJS(flat: number[]): Recognition {
  const pts = toPts(flat);
  if (pts.length < 2)
    return { kind: "freeform", confidence: 0, points: flat, closed: false };

  let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
  for (const p of pts) {
    minX = Math.min(minX, p.x); minY = Math.min(minY, p.y);
    maxX = Math.max(maxX, p.x); maxY = Math.max(maxY, p.y);
  }
  const w = maxX - minX, h = maxY - minY;
  const diag = Math.max(Math.hypot(w, h), 1);
  const eps = Math.max(diag * 0.03, 2);
  const simp = rdp(pts, eps);
  const closed = pts.length > 2 && dist(pts[0], pts[pts.length - 1]) < diag * 0.2;
  const cx = (minX + maxX) / 2, cy = (minY + maxY) / 2;

  if (closed) {
    const radii = pts.map((p) => Math.hypot(p.x - cx, p.y - cy));
    const mean = radii.reduce((a, b) => a + b, 0) / radii.length;
    const varr = radii.reduce((a, b) => a + (b - mean) ** 2, 0) / radii.length;
    const round = mean > 1e-6 ? Math.sqrt(varr) / mean : 1;
    if (round < 0.22 && simp.length >= 5) {
      const poly: number[] = [];
      for (let i = 0; i < 24; i++) {
        const ang = (2 * Math.PI * i) / 24;
        poly.push(cx + mean * Math.cos(ang), cy + mean * Math.sin(ang));
      }
      return { kind: "circle", confidence: Math.min(0.99, 1 - round / 0.22), points: poly, closed: true };
    }
    const verts = simp.slice();
    if (verts.length >= 2 && dist(verts[0], verts[verts.length - 1]) < eps * 1.5) verts.pop();
    const angles = verts.map((_, i) =>
      turn(verts[(i - 1 + verts.length) % verts.length], verts[i], verts[(i + 1) % verts.length]),
    );
    const corners = angles.filter((a) => a < CORNER_MAX_ANGLE).length;
    if (corners === 4)
      return { kind: "rectangle", confidence: 0.85, points: [minX, minY, maxX, minY, maxX, maxY, minX, maxY], closed: true };
    if (corners === 3) {
      const tri: number[] = [];
      angles.forEach((a, i) => { if (a < CORNER_MAX_ANGLE) tri.push(verts[i].x, verts[i].y); });
      return { kind: "triangle", confidence: 0.8, points: tri, closed: true };
    }
    return { kind: "freeform", confidence: 0.4, points: toFlat(simp), closed: true };
  }

  if (simp.length === 2)
    return { kind: "line", confidence: 0.95, points: [pts[0].x, pts[0].y, pts[pts.length - 1].x, pts[pts.length - 1].y], closed: false };

  const chord = dist(pts[0], pts[pts.length - 1]);
  let plen = 0;
  for (let i = 1; i < pts.length; i++) plen += dist(pts[i - 1], pts[i]);
  if (chord > 1e-6 && plen / chord < 1.1)
    return { kind: "line", confidence: 0.85, points: [pts[0].x, pts[0].y, pts[pts.length - 1].x, pts[pts.length - 1].y], closed: false };

  return { kind: "freeform", confidence: 0.5, points: toFlat(simp), closed: false };
}

self.onmessage = (e: MessageEvent) => {
  const { id, points } = e.data as { id: number; points: number[] };
  let recognition: Recognition;
  let engine = "js";
  if (wasm) {
    try {
      recognition = JSON.parse(wasm.autocomplete(Float64Array.from(points)));
      engine = "wasm";
    } catch {
      recognition = recognizeJS(points);
    }
  } else {
    recognition = recognizeJS(points);
  }
  (self as unknown as Worker).postMessage({ id, recognition, engine });
};
