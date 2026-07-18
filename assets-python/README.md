# glyphmesh-assets

The background asset service. Offline it's a **from-scratch PNG encoder**
(`glyphmesh_assets/png.py`, stdlib `zlib` + `struct` only) driving a
deterministic procedural generator — a stand-in for a diffusion model. Set
`GLYPHMESH_ASSET=openai` to route heavy generations to a real backend.

```bash
pip install -e ".[dev]"
python -m glyphmesh_assets.cli demo --prompt "sunset over mountains" --out out.png
glyphmesh-assets serve            # FastAPI on :8000
pytest -q
```

`POST /generate {prompt, seed, width, height}` returns a base64 PNG data URL.
