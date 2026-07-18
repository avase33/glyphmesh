#!/usr/bin/env bash
# Build the engine to WebAssembly and drop the artifact where the Next.js Web
# Worker looks for it. Requires wasm-pack (https://rustwasm.github.io/wasm-pack/).
#
#   cargo install wasm-pack   # once
#   ./build-wasm.sh
set -euo pipefail
cd "$(dirname "$0")"

OUT="../canvas-ts/public/engine"
wasm-pack build --release --target web --out-dir "$OUT" --out-name glyphmesh_engine
echo "wasm written to $OUT — the Web Worker will now prefer it over the JS fallback."
