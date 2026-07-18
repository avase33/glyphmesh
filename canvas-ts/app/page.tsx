"use client";

import { useEffect, useRef, useState } from "react";

const SYNC = process.env.NEXT_PUBLIC_SYNC_URL || "ws://localhost:8080";
const ASSETS = process.env.NEXT_PUBLIC_ASSETS_URL || "http://localhost:8000";
const ROOM = "studio";

type Shape = {
  id: string;
  replica: string;
  lamport: number;
  deleted?: boolean;
  kind: string;
  points: number[];
  color: string;
  closed?: boolean;
};

const PALETTE = ["#58a6ff", "#4ec9b0", "#ffb454", "#ff7b72", "#bc8cff", "#f778ba"];

function genId(): string {
  return Math.random().toString(36).slice(2, 8);
}
function dominates(a: Shape, b: Shape): boolean {
  if (a.lamport !== b.lamport) return a.lamport > b.lamport;
  return a.replica > b.replica;
}

function drawAll(
  canvas: HTMLCanvasElement,
  shapes: Shape[],
  cursors: Record<string, { x: number; y: number }>,
  stroke: number[],
) {
  const ctx = canvas.getContext("2d");
  if (!ctx) return;
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  ctx.lineWidth = 2.5;
  ctx.lineJoin = "round";
  ctx.lineCap = "round";

  for (const s of shapes) {
    if (s.deleted || s.points.length < 2) continue;
    ctx.strokeStyle = s.color;
    ctx.beginPath();
    ctx.moveTo(s.points[0], s.points[1]);
    for (let i = 2; i + 1 < s.points.length; i += 2) ctx.lineTo(s.points[i], s.points[i + 1]);
    if (s.closed) ctx.closePath();
    ctx.stroke();
  }

  if (stroke.length >= 4) {
    ctx.strokeStyle = "rgba(230,237,243,0.45)";
    ctx.beginPath();
    ctx.moveTo(stroke[0], stroke[1]);
    for (let i = 2; i + 1 < stroke.length; i += 2) ctx.lineTo(stroke[i], stroke[i + 1]);
    ctx.stroke();
  }

  for (const [id, c] of Object.entries(cursors)) {
    ctx.fillStyle = "#f778ba";
    ctx.beginPath();
    ctx.arc(c.x, c.y, 4, 0, Math.PI * 2);
    ctx.fill();
    ctx.fillStyle = "#8b949e";
    ctx.font = "11px monospace";
    ctx.fillText(id, c.x + 7, c.y - 7);
  }
}

export default function Page() {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const workerRef = useRef<Worker | null>(null);
  const shapesRef = useRef<Record<string, Shape>>({});
  const cursorsRef = useRef<Record<string, { x: number; y: number }>>({});
  const strokeRef = useRef<number[]>([]);
  const drawingRef = useRef(false);
  const lamportRef = useRef(0);
  const replicaRef = useRef(genId());
  const seqRef = useRef(0);
  const pendingRef = useRef<Map<number, string>>(new Map()); // reqId -> color
  const reqRef = useRef(0);
  const lastCursorRef = useRef(0);

  const [status, setStatus] = useState("connecting");
  const [engine, setEngine] = useState("js");
  const [count, setCount] = useState(0);
  const [prompt, setPrompt] = useState("sunset over mountains");
  const [asset, setAsset] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const redraw = () => {
    const c = canvasRef.current;
    if (c) drawAll(c, Object.values(shapesRef.current), cursorsRef.current, strokeRef.current);
  };

  const applyOp = (op: Shape) => {
    const cur = shapesRef.current[op.id];
    if (!cur || dominates(op, cur)) {
      shapesRef.current[op.id] = op;
      if (op.lamport > lamportRef.current) lamportRef.current = op.lamport;
      setCount(Object.values(shapesRef.current).filter((s) => !s.deleted).length);
      redraw();
    }
  };

  const sendJSON = (obj: unknown) => {
    const ws = wsRef.current;
    if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify(obj));
  };

  const commit = (rec: { kind: string; points: number[]; closed: boolean }, color: string) => {
    lamportRef.current += 1;
    const op: Shape = {
      id: `${replicaRef.current}_${seqRef.current++}`,
      replica: replicaRef.current,
      lamport: lamportRef.current,
      kind: rec.kind,
      points: rec.points,
      color,
      closed: rec.closed,
    };
    applyOp(op);
    sendJSON({ type: "op", room: ROOM, op });
  };

  // worker
  useEffect(() => {
    const worker = new Worker(new URL("./engine-worker.ts", import.meta.url));
    worker.onmessage = (e: MessageEvent) => {
      const { id, recognition, engine: eng } = e.data as {
        id: number;
        recognition: { kind: string; points: number[]; closed: boolean };
        engine: string;
      };
      setEngine(eng);
      const color = pendingRef.current.get(id) ?? PALETTE[0];
      pendingRef.current.delete(id);
      if (recognition && recognition.points.length >= 2) commit(recognition, color);
    };
    workerRef.current = worker;
    return () => worker.terminate();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // websocket sync
  useEffect(() => {
    const ws = new WebSocket(`${SYNC}/ws?room=${ROOM}&replica=${replicaRef.current}`);
    wsRef.current = ws;
    ws.onopen = () => setStatus("live");
    ws.onclose = () => setStatus("offline");
    ws.onerror = () => setStatus("offline");
    ws.onmessage = (e: MessageEvent) => {
      try {
        const msg = JSON.parse(e.data);
        if (msg.type === "snapshot") {
          for (const op of msg.ops as Shape[]) applyOp(op);
        } else if (msg.type === "op" && msg.op) {
          applyOp(msg.op as Shape);
        } else if (msg.type === "cursor" && msg.replica !== replicaRef.current) {
          cursorsRef.current[msg.replica] = { x: msg.x, y: msg.y };
          redraw();
        }
      } catch {
        /* ignore */
      }
    };
    return () => ws.close();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const relPoint = (e: React.PointerEvent<HTMLCanvasElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    return { x: e.clientX - rect.left, y: e.clientY - rect.top };
  };

  const onDown = (e: React.PointerEvent<HTMLCanvasElement>) => {
    drawingRef.current = true;
    const p = relPoint(e);
    strokeRef.current = [p.x, p.y];
    e.currentTarget.setPointerCapture(e.pointerId);
  };

  const onMove = (e: React.PointerEvent<HTMLCanvasElement>) => {
    const p = relPoint(e);
    const now = performance.now();
    if (now - lastCursorRef.current > 40) {
      lastCursorRef.current = now;
      sendJSON({ type: "cursor", room: ROOM, replica: replicaRef.current, x: p.x, y: p.y });
    }
    if (!drawingRef.current) return;
    strokeRef.current.push(p.x, p.y);
    redraw();
  };

  const onUp = () => {
    if (!drawingRef.current) return;
    drawingRef.current = false;
    const stroke = strokeRef.current;
    strokeRef.current = [];
    if (stroke.length < 4) {
      redraw();
      return;
    }
    const id = reqRef.current++;
    const color = PALETTE[Math.floor(Math.random() * PALETTE.length)];
    pendingRef.current.set(id, color);
    workerRef.current?.postMessage({ id, points: stroke });
  };

  const generate = async () => {
    setBusy(true);
    try {
      const res = await fetch(`${ASSETS}/generate`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ prompt, seed: 0, width: 256, height: 256 }),
      });
      const data = await res.json();
      setAsset(data.data_url);
    } catch {
      setAsset(null);
    } finally {
      setBusy(false);
    }
  };

  const clearAll = () => {
    for (const s of Object.values(shapesRef.current)) {
      lamportRef.current += 1;
      const tomb: Shape = { ...s, deleted: true, lamport: lamportRef.current, replica: replicaRef.current };
      applyOp(tomb);
      sendJSON({ type: "op", room: ROOM, op: tomb });
    }
  };

  useEffect(() => {
    redraw();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <main style={{ padding: 20, maxWidth: 1000, margin: "0 auto" }}>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline" }}>
        <h1 style={{ fontSize: 22, margin: 0 }}>glyphmesh</h1>
        <div style={{ fontSize: 13, color: "#8b949e" }}>
          engine: <b style={{ color: engine === "wasm" ? "#4ec9b0" : "#ffb454" }}>{engine}</b>
          {"  ·  "}sync: <b style={{ color: status === "live" ? "#4ec9b0" : "#ff7b72" }}>{status}</b>
          {"  ·  "}shapes: {count}
        </div>
      </header>
      <p style={{ color: "#8b949e", fontSize: 13, marginTop: 6 }}>
        draw a rough shape — the edge engine snaps it to a clean primitive and
        syncs it to everyone in the room.
      </p>

      <canvas
        ref={canvasRef}
        width={960}
        height={520}
        onPointerDown={onDown}
        onPointerMove={onMove}
        onPointerUp={onUp}
        style={{
          width: "100%",
          background: "#0d1117",
          border: "1px solid #30363d",
          borderRadius: 10,
          touchAction: "none",
          cursor: "crosshair",
        }}
      />

      <section style={{ display: "flex", gap: 10, marginTop: 14, alignItems: "center" }}>
        <input
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          placeholder="describe an asset…"
          style={{
            flex: 1,
            padding: "8px 10px",
            background: "#0d1117",
            border: "1px solid #30363d",
            borderRadius: 8,
            color: "#e6edf3",
          }}
        />
        <button onClick={generate} disabled={busy} style={btn("#238636")}>
          {busy ? "generating…" : "AI asset"}
        </button>
        <button onClick={clearAll} style={btn("#6e2c2c")}>
          clear
        </button>
      </section>

      {asset && (
        <div style={{ marginTop: 14 }}>
          <div style={{ color: "#8b949e", fontSize: 12, marginBottom: 6 }}>
            generated asset (procedural, offline)
          </div>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={asset}
            alt="generated asset"
            width={256}
            height={256}
            style={{ border: "1px solid #30363d", borderRadius: 8 }}
          />
        </div>
      )}
    </main>
  );
}

function btn(bg: string): React.CSSProperties {
  return {
    padding: "8px 14px",
    background: bg,
    color: "#fff",
    border: "none",
    borderRadius: 8,
    cursor: "pointer",
    fontSize: 14,
  };
}
