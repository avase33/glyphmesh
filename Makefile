.PHONY: up down test test-rust test-go test-python build-web wasm demo verify

up:
	docker compose up --build

down:
	docker compose down

test: test-rust test-go test-python

test-rust:
	cd engine-rust && cargo test

test-go:
	cd sync-go && go test ./...

test-python:
	cd assets-python && pip install -e ".[dev]" && pytest -q

build-web:
	cd canvas-ts && npm install && npm run build

# Build the Rust engine to WebAssembly (requires wasm-pack).
wasm:
	cd engine-rust && ./build-wasm.sh

# Offline: generate a procedural asset to a PNG file (from-scratch encoder).
demo:
	cd assets-python && python -m glyphmesh_assets.cli demo --out ../asset.png

verify:
	python scripts/verify.py
