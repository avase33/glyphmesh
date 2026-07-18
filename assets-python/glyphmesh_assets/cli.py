"""glyphmesh-assets CLI: `demo`, `serve`."""

from __future__ import annotations

import argparse
import sys


def main(argv: list[str] | None = None) -> int:
    argv = argv if argv is not None else sys.argv[1:]
    parser = argparse.ArgumentParser(prog="glyphmesh-assets")
    sub = parser.add_subparsers(dest="cmd", required=True)
    p_demo = sub.add_parser("demo", help="generate a procedural asset to a file")
    p_demo.add_argument("--prompt", default="sunset over mountains")
    p_demo.add_argument("--out", default="asset.png")
    p_serve = sub.add_parser("serve", help="run the FastAPI asset server")
    p_serve.add_argument("--host", default="0.0.0.0")
    p_serve.add_argument("--port", type=int, default=8000)

    args = parser.parse_args(argv)

    if args.cmd == "demo":
        import base64

        from .generator import generate_asset

        res = generate_asset(args.prompt, seed=0, width=256, height=256)
        payload = res["data_url"].split(",", 1)[1]
        with open(args.out, "wb") as f:
            f.write(base64.b64decode(payload))
        print(f"wrote {args.out}  (seed={res['seed']}, prompt={args.prompt!r})")
        return 0

    if args.cmd == "serve":
        import uvicorn

        uvicorn.run("glyphmesh_assets.service:app", host=args.host, port=args.port)
        return 0

    return 1


if __name__ == "__main__":
    raise SystemExit(main())
