#!/usr/bin/env python3
"""Build the signed Outpost content catalog.

Resolves every curated ZIM to the newest file in the Kiwix library with its
exact size and the SHA-256 Kiwix publishes, cuts map region outlines from
Natural Earth and measures each region with a pmtiles dry run, packs the map
style assets, reads model sizes from the Ollama registry, then writes and
signs site/public/catalog/v1/catalog.json and copies it into the node.

Usage: python3 catalog/build.py --pmtiles PATH [--key ~/.config/outpost-release/release_ed25519]
       python3 catalog/build.py --rehost     (the site moved: same content, new address, signed again)
Standard library only.
"""
import argparse
import concurrent.futures as cf
import gzip
import hashlib
import io
import json
import os
import re
import shutil
import subprocess
import sys
import tarfile
import time
import urllib.request
import xml.etree.ElementTree as ET
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SITE_URL = "https://projectoutpost.org"
OPDS = "https://opds.library.kiwix.org/catalog/v2/entries?count=-1&lang={lang}"
MIRROR = "https://download.kiwix.org/zim/"
NE = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/v5.1.2/geojson/"
UA = "outpost-catalog-builder/1 (+https://projectoutpost.org)"
NS = {"a": "http://www.w3.org/2005/Atom"}
CACHE = ROOT / "catalog" / ".cache"


def fetch(url, binary=True, cache=False, tries=4):
    path = CACHE / hashlib.sha256(url.encode()).hexdigest()
    if cache and path.exists():
        data = path.read_bytes()
    else:
        for i in range(tries):
            try:
                req = urllib.request.Request(url, headers={"User-Agent": UA})
                with urllib.request.urlopen(req, timeout=120) as r:
                    data = r.read()
                break
            except Exception as e:
                if i == tries - 1:
                    raise RuntimeError(f"{url}: {e}")
                time.sleep(2 * (i + 1))
        if cache:
            CACHE.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
    return data if binary else data.decode()


def head_size(url):
    req = urllib.request.Request(url, method="HEAD", headers={"User-Agent": UA})
    for i in range(4):
        try:
            with urllib.request.urlopen(req, timeout=60) as r:
                return int(r.headers["Content-Length"]), r.geturl()
        except Exception as e:
            if i == 3:
                raise RuntimeError(f"HEAD {url}: {e}")
            time.sleep(2 * (i + 1))


def opds_index():
    idx = {}
    for lang in ("eng", "mul"):
        root = ET.fromstring(fetch(OPDS.format(lang=lang), cache=True))
        for e in root.findall("a:entry", NS):
            name = e.findtext("a:name", "", NS)
            flavour = e.findtext("a:flavour", "", NS) or ""
            link = None
            for l in e.findall("a:link", NS):
                if l.get("type") == "application/x-zim":
                    link = l.get("href")
            if not link:
                continue
            issued = e.findtext("{http://purl.org/dc/terms/}issued", "", NS) or e.findtext("a:updated", "", NS)
            ent = {
                "name": name, "flavour": flavour, "title": e.findtext("a:title", "", NS),
                "summary": e.findtext("a:summary", "", NS), "issued": issued,
                "author": (e.find("a:author", NS).findtext("a:name", "", NS) if e.find("a:author", NS) is not None else ""),
                "link": link.removesuffix(".meta4"),
            }
            key = (name, flavour)
            if key not in idx or ent["issued"] > idx[key]["issued"]:
                idx[key] = ent
    return idx


FLAVOUR_LABEL = {"maxi": "", "nopic": " (text only)", "mini": " (introductions only)"}


def license_for(name, licenses):
    for prefix, lic in licenses.items():
        if prefix in name:
            return lic
    return "See the archive's own notice"


def resolve_zim(ref, idx, src):
    name, _, flavour = ref.partition(":")
    ent = idx.get((name, flavour))
    if ent is None:
        raise SystemExit(f"not in the Kiwix library: {ref}")
    rel = ent["link"].split("/zim/", 1)[1]
    url = MIRROR + rel
    file = rel.rsplit("/", 1)[1]
    size, final = head_size(url)
    sha = fetch(url + ".sha256", binary=False).split()[0].lower()
    assert re.fullmatch(r"[0-9a-f]{64}", sha), (ref, sha)
    m = re.search(r"_(\d{4}-\d{2})\.zim$", file)
    item_id = name + (("_" + flavour) if flavour else "")
    notice = "medical" if name in src["notices"]["medical"] else ""
    title = ent["title"] + FLAVOUR_LABEL.get(flavour, "")
    return {
        "id": item_id.lower(), "kind": "zim", "title": title, "description": ent["summary"],
        "publisher": ent["author"], "license": license_for(name, src["licenses"]), "size": size,
        "date": m.group(1) if m else "", "notice": notice, "url": url, "file": file, "sha256": sha,
        "book": name, "flavour": flavour,
    }


def round_coords(obj, nd=3):
    if isinstance(obj, list):
        if obj and isinstance(obj[0], (int, float)):
            return [round(obj[0], nd), round(obj[1], nd)]
        return [round_coords(o, nd) for o in obj]
    return obj


def bbox_of(coords):
    xs, ys = [], []

    def walk(c):
        if c and isinstance(c[0], (int, float)):
            xs.append(c[0]); ys.append(c[1])
        else:
            for x in c:
                walk(x)
    walk(coords)
    return [round(min(xs), 3), round(min(ys), 3), round(max(xs), 3), round(max(ys), 3)]


def slug(s):
    return re.sub(r"[^a-z0-9]+", "-", s.lower()).strip("-")


def regions():
    countries = json.loads(fetch(NE + "ne_50m_admin_0_countries.geojson", cache=True))
    admin1 = json.loads(fetch(NE + "ne_50m_admin_1_states_provinces.geojson", cache=True))
    out = []
    continents = {}
    for f in countries["features"]:
        p = f["properties"]
        g = f["geometry"]
        if not g:
            continue
        polys = g["coordinates"] if g["type"] == "MultiPolygon" else [g["coordinates"]]
        cont = p.get("CONTINENT")
        if cont in ("Antarctica", "Seven seas (open ocean)"):
            continue
        continents.setdefault(cont, []).extend(polys)
        a3 = (p.get("ADM0_A3") or "").lower()
        name = p.get("NAME_EN") or p.get("NAME")
        out.append({"id": f"map-{a3}", "title": name, "group": cont, "coords": polys})
    for cont, polys in continents.items():
        out.append({"id": f"map-continent-{slug(cont)}", "title": cont, "group": "Continents", "coords": polys})
    groups = {"USA": "United States", "CAN": "Canada", "AUS": "Australia"}
    for f in admin1["features"]:
        p = f["properties"]
        a3 = p.get("adm0_a3")
        if a3 not in groups or not f["geometry"]:
            continue
        g = f["geometry"]
        polys = g["coordinates"] if g["type"] == "MultiPolygon" else [g["coordinates"]]
        code = (p.get("postal") or slug(p.get("name", "")))[:12].lower()
        out.append({"id": f"map-{a3.lower()}-{slug(code)}", "title": p.get("name"), "group": groups[a3], "coords": polys})
    seen = set()
    uniq = []
    for r in out:
        if r["id"] in seen:
            continue
        seen.add(r["id"])
        uniq.append(r)
    return uniq


SIZE_RE = re.compile(r"archive size of ([\d.]+) ([kMGT]?B)")
UNITS = {"B": 1, "kB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12}


def dry_run(pmtiles, build, geometry, maxzoom):
    args = [pmtiles, "extract", build, "/tmp/outpost-dry.pmtiles", f"--maxzoom={maxzoom}", "--dry-run"]
    if geometry:
        args.append(f"--region={geometry}")
    for i in range(3):
        p = subprocess.run(args, capture_output=True, text=True, timeout=1800)
        m = SIZE_RE.search(p.stderr + p.stdout)
        if m:
            return int(float(m.group(1)) * UNITS[m.group(2)])
        time.sleep(3)
    raise RuntimeError(f"dry run failed for {geometry}: {p.stderr[-300:]}")


def deterministic_tar(files, out):
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w", format=tarfile.USTAR_FORMAT) as tf:
        for name in sorted(files):
            data = files[name]
            ti = tarfile.TarInfo(name)
            ti.size = len(data)
            ti.mtime = 0
            ti.mode = 0o644
            ti.uid = ti.gid = 0
            ti.uname = ti.gname = ""
            tf.addfile(ti, io.BytesIO(data))
    raw = buf.getvalue()
    gz = io.BytesIO()
    with gzip.GzipFile(fileobj=gz, mode="wb", mtime=0, compresslevel=9) as g:
        g.write(raw)
    out.write_bytes(gz.getvalue())


def map_assets(src):
    commit = src["maps"]["assets_commit"]
    data = fetch(f"https://codeload.github.com/protomaps/basemaps-assets/tar.gz/{commit}", cache=True)
    files = {}
    with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as tf:
        for m in tf.getmembers():
            if not m.isfile():
                continue
            rel = m.name.split("/", 1)[1] if "/" in m.name else m.name
            keep = any(rel.startswith(f"fonts/{f}/") and rel.endswith(".pbf") for f in src["maps"]["fonts"])
            keep |= any(rel in (f"sprites/{s}.json", f"sprites/{s}.png") for s in src["maps"]["sprites"])
            if keep:
                files[rel] = tf.extractfile(m).read()
    if len(files) < 100:
        raise SystemExit(f"map assets look incomplete ({len(files)} files)")
    name = f"map-assets-{commit[:8]}.tar.gz"
    out = ROOT / "site" / "public" / "assets" / name
    out.parent.mkdir(parents=True, exist_ok=True)
    deterministic_tar(files, out)
    b = out.read_bytes()
    return {
        "id": "map-assets", "kind": "map-assets", "title": "Map fonts and icons",
        "description": "Noto Sans glyphs and the Protomaps icon sprites the map style needs.",
        "publisher": "Protomaps; Google Noto fonts", "license": "SIL Open Font License 1.1; CC0",
        "size": len(b), "url": f"{SITE_URL}/assets/{name}", "file": name, "sha256": hashlib.sha256(b).hexdigest(),
    }


def model_size(name):
    n, _, tag = name.partition(":")
    req = urllib.request.Request(f"https://registry.ollama.ai/v2/library/{n}/manifests/{tag or 'latest'}",
                                 headers={"Accept": "application/vnd.docker.distribution.manifest.v2+json", "User-Agent": UA})
    with urllib.request.urlopen(req, timeout=60) as r:
        d = json.load(r)
    return sum(l["size"] for l in d["layers"]) + d["config"]["size"]


def publish(cat, out_dir, key):
    """Write the catalog, sign it, and copy both into the node."""
    data = json.dumps(cat, indent=1, ensure_ascii=False).encode() + b"\n"
    out = out_dir / "catalog.json"
    out.write_bytes(data)
    sig = out.with_suffix(".json.sig")
    if sig.exists():
        sig.unlink()
    subprocess.run(["ssh-keygen", "-Y", "sign", "-f", key, "-n", "outpost-catalog", str(out)], check=True, capture_output=True)
    emb = ROOT / "node" / "internal" / "catalog" / "embedded"
    emb.mkdir(parents=True, exist_ok=True)
    shutil.copy(out, emb / "catalog.json")
    shutil.copy(sig, emb / "catalog.json.sig")
    print(f"catalog: {len(cat['items'])} items, sequence {cat['sequence']}", file=sys.stderr)


def rehost(out_dir, key):
    """The site has a new address. Point every file the site itself serves at
    SITE_URL, leave everything else exactly as it was, and sign again with a
    higher sequence number so nodes take the new copy."""
    cat = json.loads((out_dir / "catalog.json").read_text())
    moved = 0
    for item in cat["items"]:
        m = re.match(r"https://[^/]+(/(?:assets|catalog/v1/regions)/.+)$", item.get("url", ""))
        if m and not item["url"].startswith(SITE_URL + "/"):
            item["url"] = SITE_URL + m.group(1)
            moved += 1
    cat["sequence"] = max(int(time.time()), cat["sequence"] + 1)
    cat["generated"] = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    print(f"rehost: {moved} addresses now at {SITE_URL}", file=sys.stderr)
    publish(cat, out_dir, key)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--pmtiles")
    ap.add_argument("--key", default=os.path.expanduser("~/.config/outpost-release/release_ed25519"))
    ap.add_argument("--jobs", type=int, default=6)
    ap.add_argument("--skip-maps", action="store_true", help="reuse region sizes from the previous catalog")
    ap.add_argument("--rehost", action="store_true", help="only move the site's own files to SITE_URL and sign again")
    args = ap.parse_args()
    out_dir = ROOT / "site" / "public" / "catalog" / "v1"
    if args.rehost:
        rehost(out_dir, args.key)
        return
    if not args.pmtiles:
        ap.error("--pmtiles is required")
    src = json.loads((ROOT / "catalog" / "source.json").read_text())
    prev = {}
    prev_path = out_dir / "catalog.json"
    if prev_path.exists():
        prev = {i["id"]: i for i in json.loads(prev_path.read_text())["items"]}

    print("Kiwix library…", file=sys.stderr)
    idx = opds_index()
    refs = set()
    for o in src["wikipedia"]["options"]:
        refs.update(o["items"])
    tiers = {}
    for c in src["categories"]:
        acc = []
        t = {}
        for tier in ("essential", "standard", "comprehensive"):
            lst = c[tier]
            items = (acc + [x for x in lst if x != "+"]) if lst and lst[0] == "+" else list(lst)
            seen = []
            for x in items:
                if x not in seen:
                    seen.append(x)
            t[tier] = seen
            acc = seen
            refs.update(seen)
        tiers[c["id"]] = t
    items = {}
    with cf.ThreadPoolExecutor(8) as ex:
        for ref, it in zip(sorted(refs), ex.map(lambda r: resolve_zim(r, idx, src), sorted(refs))):
            items[ref] = it
            print(f"  {it['id']:50} {it['size']/1e9:8.2f} GB  {it['file']}", file=sys.stderr)

    def ids(lst):
        return [items[r]["id"] for r in lst]

    categories = [{
        "id": "wikipedia", "name": src["wikipedia"]["name"], "description": src["wikipedia"]["description"],
        "icon": "globe", "kind": "single", "default": src["wikipedia"]["default"],
        "options": [{"id": o["id"], "label": o["label"], "description": o["description"], "items": ids(o["items"])} for o in src["wikipedia"]["options"]],
    }]
    for c in src["categories"]:
        categories.append({"id": c["id"], "name": c["name"], "description": c["description"], "icon": c["icon"],
                           "kind": "tiered", "tiers": {k: ids(v) for k, v in tiers[c["id"]].items()}})
    all_items = list(items.values())

    print("Models…", file=sys.stderr)
    for m in src["models"]:
        all_items.append({"id": "model-" + slug(m["model"]), "kind": "model", "title": m["title"], "description": m["description"],
                          "size": model_size(m["model"]), "model": m["model"], "ramGB": m["ramGB"], "license": "See the model's licence"})

    print("Map assets…", file=sys.stderr)
    all_items.append(map_assets(src))

    builds = json.loads(fetch("https://build-metadata.protomaps.dev/builds.json"))
    build = "https://build.protomaps.com/" + builds[-1]["key"]
    print(f"Map build {build}", file=sys.stderr)
    region_dir = ROOT / "site" / "public" / "catalog" / "v1" / "regions"
    region_dir.mkdir(parents=True, exist_ok=True)
    regs = regions()
    world = {"id": "map-world", "kind": "map-world", "title": "World overview", "description": "The whole world at low detail, under your regions.",
             "publisher": "Protomaps, OpenStreetMap contributors", "license": "ODbL 1.0 (© OpenStreetMap contributors)", "maxzoom": src["maps"]["world_maxzoom"]}
    jobs = []
    for r in regs:
        gj = {"type": "FeatureCollection", "features": [{"type": "Feature", "properties": {"name": r["title"]},
              "geometry": {"type": "MultiPolygon", "coordinates": round_coords(r["coords"])}}]}
        data = json.dumps(gj, separators=(",", ":")).encode()
        path = region_dir / f"{r['id']}.geojson"
        path.write_bytes(data)
        r["sha256"] = hashlib.sha256(data).hexdigest()
        r["path"] = path
        r["bbox"] = bbox_of(r["coords"])
        jobs.append(r)
    if args.skip_maps and prev:
        world["size"] = prev.get("map-world", {}).get("size", 45_000_000)
        for r in jobs:
            r["size"] = prev.get(r["id"], {}).get("size", 0)
    else:
        print(f"Measuring {len(jobs)} regions…", file=sys.stderr)
        world["size"] = dry_run(args.pmtiles, build, None, src["maps"]["world_maxzoom"])
        with cf.ThreadPoolExecutor(args.jobs) as ex:
            futs = {ex.submit(dry_run, args.pmtiles, build, str(r["path"]), src["maps"]["region_maxzoom"]): r for r in jobs}
            for i, f in enumerate(cf.as_completed(futs)):
                r = futs[f]
                r["size"] = f.result()
                print(f"  [{i+1}/{len(jobs)}] {r['title']:40} {r['size']/1e9:7.2f} GB", file=sys.stderr)
    all_items.append(world)
    for r in sorted(jobs, key=lambda r: (r["group"], r["title"])):
        all_items.append({"id": r["id"], "kind": "map-region", "title": r["title"], "group": r["group"], "size": r["size"],
                          "url": f"{SITE_URL}/catalog/v1/regions/{r['id']}.geojson", "sha256": r["sha256"], "bbox": r["bbox"],
                          "maxzoom": src["maps"]["region_maxzoom"], "publisher": "Protomaps, OpenStreetMap contributors",
                          "license": "ODbL 1.0 (© OpenStreetMap contributors)"})

    cat = {"schema": 1, "sequence": int(time.time()), "generated": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
           "mapBuild": build, "categories": categories, "items": all_items}
    publish(cat, out_dir, args.key)


if __name__ == "__main__":
    main()
