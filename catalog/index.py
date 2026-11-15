#!/usr/bin/env python3
"""Build the content index: every item in every download, shelved and measured.

Reads the signed catalog (site/public/catalog/v1/catalog.json), the purposes in
catalog/source.json, and what zimlist.py (pass A) and zimfacts.py (pass B)
cached for each archive. Writes, under the output directory (default
catalog/.cache/index, published as /catalog/v1/index/):

  root.json             subjects → collections, with purposes, counts, sizes, totals
  root.json.sig         SSH signature (namespace outpost-content-index), if a key is given
  c/<collection>.json   the collection's section tree; each node with items
                        points at its leaf: [pack number, offset, length]
  p/<collection>.<n>.bin  leaves: gzip members, each a JSON array of item rows
  s/index.json          title search: first key of every block, and where it is
  s/<n>.bin             search blocks: gzip members of [title, [[collection, path, row], ...]]

Every node holds at most 200 rows (tree.LIMIT). The build stops if one does
not, if an archive has not been listed, or if a purpose is missing.

Usage: python3 catalog/index.py [--out DIR] [--key PATH] [--allow-unmeasured]
"""
import argparse
import collections
import gzip
import json
import os
import shutil
import subprocess
import sys
import tempfile
import time
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import sources  # noqa: E402
import tree  # noqa: E402
import zimfacts  # noqa: E402

ROOT = Path(__file__).resolve().parent.parent
CATALOG = ROOT / "site" / "public" / "catalog" / "v1" / "catalog.json"
SOURCE = ROOT / "catalog" / "source.json"
CACHE = ROOT / "catalog" / ".cache" / "zim"
TIERS = ("essential", "standard", "comprehensive")
SEARCH_BLOCK = 1500
NAMESPACE = "outpost-content-index"


def load_facts(file):
    facts = {}
    p = CACHE / (file + ".facts.jsonl.gz")
    if p.exists():
        with gzip.open(p, "rt", encoding="utf-8") as g:
            for line in g:
                d = json.loads(line)
                facts[d["i"]] = d
    return facts, p.exists()


class Search:
    """Title search entries, spilled to disk and sorted with sort(1)."""

    def __init__(self, tmp):
        self.path = Path(tmp) / "search.tsv"
        self.f = open(self.path, "w", encoding="utf-8")
        self.n = 0

    def add(self, col, path, items):
        p = ".".join(map(str, path))
        for row, it in enumerate(items):
            t = it["t"].replace("\t", " ").replace("\n", " ")
            k = tree.norm(t)[:120]
            if not k:
                continue
            self.f.write(f"{k}\t{t}\t{col}\t{p}\t{row}\n")
            self.n += 1

    def write(self, out):
        self.f.close()
        sd = out / "s"
        sd.mkdir(parents=True, exist_ok=True)
        srt = self.path.with_suffix(".sorted")
        env = dict(os.environ, LC_ALL="C")
        subprocess.run(["sort", "-t", "\t", "-k1,1", "-k2,2", "-S", "2G", "-T", str(self.path.parent), "-o", str(srt), str(self.path)],
                       check=True, env=env)
        pack = tree.PackWriter(sd, "s")
        blocks = []
        cur, cur_key = [], None
        last = None

        def flush():
            if cur:
                blocks.append([cur[0][0]] + pack.add([[t, refs] for _k, t, refs in cur]))

        with open(srt, encoding="utf-8") as f:
            for line in f:
                k, t, col, p, row = line.rstrip("\n").split("\t")
                ref = [int(col), p, int(row)]
                if last and last[0] == k and last[1] == t:
                    last[2].append(ref)
                    continue
                if len(cur) >= SEARCH_BLOCK and k != cur[-1][0]:
                    flush()
                    cur = []
                last = [k, t, [ref]]
                cur.append(last)
        flush()
        pack.close()
        (sd / "index.json").write_text(json.dumps({"blocks": blocks, "entries": self.n}, ensure_ascii=False, separators=(",", ":")))
        return len(blocks)


def tier_of(cat, iid):
    if cat["kind"] == "single":
        return next((o["label"] for o in cat["options"] if iid in o["items"]), "")
    return next((t.capitalize() for t in TIERS if iid in (cat.get("tiers") or {}).get(t, [])), "")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default=str(ROOT / "catalog" / ".cache" / "index"))
    ap.add_argument("--key", default=os.path.expanduser("~/.config/outpost-release/release_ed25519"))
    ap.add_argument("--no-sign", action="store_true")
    ap.add_argument("--only", nargs="*", help="build only these collections (for testing)")
    args = ap.parse_args()

    t0 = time.time()
    cat = json.loads(CATALOG.read_text())
    src = json.loads(SOURCE.read_text())
    purposes = src["purposes"]
    items = {i["id"]: i for i in cat["items"]}
    out = Path(args.out)
    if out.exists():
        shutil.rmtree(out)
    for d in ("c", "p", "s"):
        (out / d).mkdir(parents=True)
    tmp = tempfile.mkdtemp(prefix="outpost-index-", dir=str(ROOT / "catalog" / ".cache"))
    search = Search(tmp)

    subjects = []
    collections_order = []  # collection ids, index used by search refs
    seen_in = collections.defaultdict(list)
    for c in cat["categories"]:
        ids = [x for o in c.get("options", []) for x in o["items"]] + [x for t in TIERS for x in (c.get("tiers") or {}).get(t, [])]
        for x in dict.fromkeys(ids):
            seen_in[x].append(c["id"])

    missing = [s for s in [c["id"] for c in cat["categories"]] + ["maps", "models"] if s not in purposes["subjects"]]
    missing += [i for i in items if items[i]["kind"] == "zim" and i not in purposes["items"]]
    if missing:
        sys.exit(f"purpose missing for: {missing}")

    def build_collection(cid, title, its, skipped, extra):
        root = tree.build(its)
        n = tree.check(root)
        pack = tree.PackWriter(out / "p", cid)
        col = len(collections_order)
        collections_order.append(cid)
        t = tree.serialise(root, pack, lambda path, leaf: search.add(col, path, leaf))
        pack.close()
        t["l"] = title
        (out / "c" / f"{cid}.json").write_text(json.dumps(t, ensure_ascii=False, separators=(",", ":")))
        rec = {"id": cid, "title": title, "items": n, "itemBytes": t["s"], "unmeasured": t.get("u", 0),
               "kinds": dict(collections.Counter(i["k"] for i in its).most_common()),
               "other": dict(skipped.most_common()), "packs": pack.n + 1}
        rec.update(extra)
        return rec

    built = {}
    stats = collections.Counter()
    for c in cat["categories"]:
        ids = [x for o in c.get("options", []) for x in o["items"]] + [x for t in TIERS for x in (c.get("tiers") or {}).get(t, [])]
        cols = []
        for iid in dict.fromkeys(ids):
            it = items[iid]
            if args.only and iid not in args.only:
                continue
            if iid not in built:
                meta_p = CACHE / (it["file"] + ".meta.json")
                if not meta_p.exists():
                    sys.exit(f"{iid}: not listed yet (run catalog/zimlist.py)")
                meta = json.loads(meta_p.read_text())
                rows = zimfacts.load_rows(it["file"])
                facts, measured = load_facts(it["file"])
                fam = sources.family(it, meta)
                its, skipped = fam.items(rows, facts)
                skipped["redirects (other names for the same page)"] += sum(v for k, v in meta["counts"].items() if k.startswith("redirect"))
                for k, v in meta["counts"].items():
                    if k.startswith("dropped "):
                        kind = k[len("dropped "):]
                        label = ("pictures" if kind.startswith("image/") else "fonts" if "font" in kind else
                                 "styles and scripts" if kind in ("text/css", "application/javascript", "text/javascript", "application/wasm")
                                 else "search index" if "xapian" in kind or "zimlisting" in kind else "other support files")
                        skipped[label] += v
                md = meta.get("metadata", {})
                extra = {"purpose": purposes["items"][iid], "description": it.get("description") or md.get("Description", ""),
                         "longDescription": md.get("LongDescription", ""), "edition": it.get("date") or md.get("Date", "")[:7],
                         "built": md.get("Date", ""), "size": it["size"], "license": it.get("license", ""),
                         "publisher": it.get("publisher") or md.get("Publisher", ""), "creator": md.get("Creator", ""),
                         "file": it["file"], "sha256": it["sha256"], "entries": meta["entries"], "measured": measured,
                         "notice": it.get("notice", ""), "family": fam.name, "kind": "zim"}
                built[iid] = build_collection(iid, it["title"], its, skipped, extra)
                stats["items"] += built[iid]["items"]
                print(f"  {iid:45} {built[iid]['items']:>10,} items  {time.time() - t0:6.0f}s", file=sys.stderr, flush=True)
            rec = dict(built[iid])
            rec["tier"] = tier_of(c, iid)
            rec["alsoIn"] = [s for s in seen_in[iid] if s != c["id"]]
            cols.append(rec)
        tiers = {}
        if c["kind"] == "tiered":
            tiers = {t: sum(items[x]["size"] for x in c["tiers"].get(t, [])) for t in TIERS}
        else:
            tiers = {o["label"]: sum(items[x]["size"] for x in o["items"]) for o in c["options"] if o["items"]}
        subjects.append({"id": c["id"], "name": c["name"], "description": c["description"], "purpose": purposes["subjects"][c["id"]],
                         "kind": c["kind"], "sizes": tiers, "collections": cols})

    # Maps: one collection of regions shelved by group, and the shared files.
    regions = [i for i in cat["items"] if i["kind"] == "map-region"]
    reg_items = [{"t": r["title"], "k": "map region", "path": (r.get("group") or "Other",), "s": r["size"], "idx": 0,
                  "zoom": r.get("maxzoom"), "bbox": r.get("bbox"), "x": purposes["map-region"]} for r in regions]
    build_date = cat["mapBuild"].rsplit("/", 1)[-1][:8]
    build_date = f"{build_date[:4]}-{build_date[4:6]}-{build_date[6:8]}"
    for r in reg_items:
        r["d"] = build_date
    map_cols = [build_collection("map-regions", "Map regions", reg_items, collections.Counter(), {
        "purpose": "Street-level maps (zoom 15) of each region, cut from the Protomaps planet build. Pick only the regions you need.",
        "edition": build_date[:7], "size": sum(r["size"] for r in regions), "license": regions[0]["license"],
        "publisher": regions[0]["publisher"], "kind": "maps", "measured": True})]
    shared = [i for i in cat["items"] if i["kind"] in ("map-world", "map-assets")]
    map_cols.append(build_collection("map-shared", "World overview, fonts and icons",
                                     [{"t": i["title"], "k": "map data", "path": (), "s": i["size"], "idx": 0,
                                       "x": purposes["items"].get(i["id"], i.get("description", "")), "by": i.get("publisher")} for i in shared],
                                     collections.Counter(), {"purpose": "Downloaded once with the first map region.",
                                                             "size": sum(i["size"] for i in shared), "kind": "maps", "measured": True,
                                                             "license": "; ".join(dict.fromkeys(i.get("license", "") for i in shared))}))
    subjects.append({"id": "maps", "name": "Maps", "description": f"{len(regions)} regions at street level.",
                     "purpose": purposes["subjects"]["maps"], "kind": "maps", "collections": map_cols})

    models = [i for i in cat["items"] if i["kind"] == "model"]
    model_items = [{"t": m["title"], "k": "AI model", "path": (), "s": m["size"], "idx": 0, "x": m.get("description"),
                    "ram": m.get("ramGB"), "ollama": m.get("model"), "rank": m.get("rank")} for m in models]
    subjects.append({"id": "models", "name": "AI models", "description": "Optional, through Ollama.",
                     "purpose": purposes["subjects"]["models"], "kind": "models",
                     "collections": [build_collection("models", "AI models", model_items, collections.Counter(), {
                         "purpose": purposes["model"] + " Choose one that fits your computer's memory.",
                         "size": sum(m["size"] for m in models), "kind": "models", "measured": True,
                         "license": "See each model's licence"})]})

    nblocks = search.write(out)
    shutil.rmtree(tmp, ignore_errors=True)

    # Totals that add up in public.
    zims = [i for i in cat["items"] if i["kind"] == "zim"]
    every = sum(i["size"] for i in cat["items"])
    wiki = next(c for c in cat["categories"] if c["id"] == "wikipedia")
    biggest_wiki = max((sum(items[x]["size"] for x in o["items"]), o["label"]) for o in wiki["options"])
    largest = set(o for o in next(o for o in wiki["options"] if o["label"] == biggest_wiki[1])["items"])
    for c in cat["categories"]:
        if c["kind"] == "tiered":
            largest.update(c["tiers"]["comprehensive"])
    largest_bytes = sum(items[x]["size"] for x in largest)
    all_items = sum(col["items"] for col in built.values()) + len(regions) + len(shared) + len(models)
    root = {
        "schema": 1, "generated": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "catalogSequence": cat["sequence"], "catalogGenerated": cat["generated"], "limit": tree.LIMIT,
        "collections": collections_order, "searchBlocks": nblocks,
        "totals": {
            "downloads": len(cat["items"]), "archives": len(zims), "mapRegions": len(regions), "models": len(models),
            "items": all_items, "unmeasured": sum(col["unmeasured"] for col in built.values()),
            "everyFile": every, "archiveBytes": sum(i["size"] for i in zims),
            "mapBytes": sum(i["size"] for i in regions + shared), "modelBytes": sum(m["size"] for m in models),
            "largestLibrary": largest_bytes,
            "largestLibraryRule": f"Wikipedia “{biggest_wiki[1]}” and every subject at Comprehensive, without the smaller editions they replace",
            "largestWithMaps": largest_bytes + sum(i["size"] for i in regions + shared),
            "largestWithEverything": largest_bytes + sum(i["size"] for i in regions + shared) + sum(m["size"] for m in models),
        },
        "subjects": subjects,
    }
    (out / "root.json").write_text(json.dumps(root, ensure_ascii=False, separators=(",", ":")))
    if not args.no_sign and Path(args.key).exists():
        subprocess.run(["ssh-keygen", "-Y", "sign", "-f", args.key, "-n", NAMESPACE, str(out / "root.json")], check=True, capture_output=True)
    files = sum(1 for _ in out.rglob("*") if _.is_file())
    size = sum(p.stat().st_size for p in out.rglob("*") if p.is_file())
    print(f"index: {all_items:,} items in {len(collections_order)} collections; {files} files, {size / 1e6:.1f} MB; "
          f"{time.time() - t0:.0f}s", file=sys.stderr)


if __name__ == "__main__":
    main()
