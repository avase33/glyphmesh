"""Async FastAPI asset service. Heavy generation runs in a thread pool so the
event loop stays responsive."""

from __future__ import annotations

import asyncio

from fastapi import FastAPI
from pydantic import BaseModel

from .generator import generate_asset

app = FastAPI(title="glyphmesh assets", version="0.1.0")


class GenerateRequest(BaseModel):
    prompt: str = "abstract"
    seed: int = 0
    width: int = 256
    height: int = 256


class GenerateResponse(BaseModel):
    kind: str
    data_url: str
    seed: int


@app.get("/healthz")
def healthz() -> dict:
    return {"status": "ok"}


@app.post("/generate", response_model=GenerateResponse)
async def generate(req: GenerateRequest) -> GenerateResponse:
    result = await asyncio.to_thread(
        generate_asset, req.prompt, req.seed, req.width, req.height
    )
    return GenerateResponse(**result)
