#!/usr/bin/env python3
"""Build, stage and (optionally) deploy the content index.

The index (hundreds of MB) is served by its own small static Vercel project,
so deploying the site from any checkout never has to carry it. Each build
lives under its own folder, named by its time, and is cached forever; the
previous build stays up beside it, so a site still pointing at it keeps
working until the site is deployed with the new root.json.

  python3 catalog/publish_index.py --host https://index.projectoutpost.org [--deploy]

Steps: build with catalog/index.py into catalog/.cache/index-builds/<build>,
stage the newest two builds and a vercel.json into catalog/.cache/index-site,
copy root.json and root.json.sig into site/public/catalog/v1/index/ (they are
committed with the site), and with --deploy run `vercel deploy --prod` in the
staging folder. The staging folder is linked to the Vercel project once by
hand (`vercel link`), and stays linked.
"""
import argparse
import json
import shutil
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
BUILDS = ROOT / "catalog" / ".cache" / "index-builds"
STAGE = ROOT / "catalog" / ".cache" / "index-site"
SITE_INDEX = ROOT / "site" / "public" / "catalog" / "v1" / "index"
KEEP = 2

VERCEL_JSON = {
    "$schema": "https://openapi.vercel.sh/vercel.json",
    "framework": None,
    "buildCommand": "",
    "outputDirectory": ".",
    "headers": [
        {"source": "/(.*)", "headers": [
            {"key": "Access-Control-Allow-Origin", "value": "*"},
            {"key": "Access-Control-Allow-Methods", "value": "GET, HEAD, OPTIONS"},
            {"key": "Access-Control-Allow-Headers", "value": "Range"},
            {"key": "Access-Control-Expose-Headers", "value": "Content-Range, Content-Length, Accept-Ranges"},
            {"key": "Cache-Control", "value": "public, max-age=31536000, immutable"},
            {"key": "X-Content-Type-Options", "value": "nosniff"},
        ]},
        {"source": "/(.*)\\.bin", "headers": [{"key": "Content-Type", "value": "application/octet-stream"}]},
    ],
}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--host", required=True, help="the index project's address, e.g. https://index.projectoutpost.org")
    ap.add_argument("--deploy", action="store_true")
    ap.add_argument("--reuse", help="stage an existing build instead of building")
    args = ap.parse_args()
    host = args.host.rstrip("/")
    if not host.startswith("https://"):
        sys.exit("--host must be https")

    if args.reuse:
        build = args.reuse
    else:
        build = datetime.now(timezone.utc).strftime("%Y%m%dT%H%MZ")
        out = BUILDS / build
        subprocess.run([sys.executable, "-I", str(ROOT / "catalog" / "index.py"), "--out", str(out), "--base", f"{host}/{build}"], check=True)
    src = BUILDS / build
    root = json.loads((src / "root.json").read_text())
    if root.get("base") != f"{host}/{build}":
        sys.exit(f"{build}: root.json names {root.get('base')!r}, not {host}/{build}")
    if not (src / "root.json.sig").exists():
        sys.exit(f"{build}: root.json is not signed")

    # Stage the newest KEEP builds.
    builds = sorted(p.name for p in BUILDS.iterdir() if (p / "root.json").exists())
    keep = sorted(set(builds[-KEEP:]) | {build})
    STAGE.mkdir(parents=True, exist_ok=True)
    for p in STAGE.iterdir():
        if p.is_dir() and p.name not in keep and p.name != ".vercel":
            shutil.rmtree(p)
    for b in keep:
        if not (STAGE / b).exists():
            shutil.copytree(BUILDS / b, STAGE / b)
    (STAGE / "vercel.json").write_text(json.dumps(VERCEL_JSON, indent=1))
    (STAGE / "index.html").write_text("<!doctype html><title>Outpost content index</title><p>Data for "
                                      "<a href=\"https://www.projectoutpost.org/catalog\">projectoutpost.org/catalog</a>.</p>\n")

    # The site carries the small signed root of the build it shows.
    SITE_INDEX.mkdir(parents=True, exist_ok=True)
    for name in ("root.json", "root.json.sig"):
        shutil.copy(src / name, SITE_INDEX / name)
    files = sum(1 for _ in STAGE.rglob("*") if _.is_file())
    size = sum(p.stat().st_size for p in STAGE.rglob("*") if p.is_file())
    print(f"staged {keep} in {STAGE}: {files} files, {size / 1e6:.0f} MB; site root.json -> {build}", file=sys.stderr)

    if args.deploy:
        if not (STAGE / ".vercel").exists():
            sys.exit(f"link {STAGE} to the index project first: cd {STAGE} && vercel link")
        subprocess.run(["vercel", "deploy", "--prod", "--yes"], cwd=STAGE, check=True)


if __name__ == "__main__":
    main()
