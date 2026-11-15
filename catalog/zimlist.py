#!/usr/bin/env python3
"""Pass A of the content index: list what is inside every archive.

For each ZIM in the catalog this fetches only the archive's header, MIME
list, pointer lists and directory block (by range request), and writes

  catalog/.cache/zim/<file>.list.tsv.gz   one line per kept entry:
      index, namespace, mime, front (1 = an article Kiwix lists), cluster, blob, path, title
  catalog/.cache/zim/<file>.meta.json     the archive's own metadata and the
      count of every directory entry, kept or dropped, by kind

Kept: every front article, and every other content entry that is not a
picture, style sheet, font, script or search index (documents, videos,
audio, data files a scraper uses to describe its items). Redirects are
counted, not kept. Results are keyed by the archive's file name, which
carries its edition, so an archive that has not changed is never read again.

Usage: python3 catalog/zimlist.py [--only ID ...] [--jobs 3]
"""
import argparse
import collections
import concurrent.futures as cf
import gzip
import json
import os
import struct
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import zim  # noqa: E402

ROOT = Path(__file__).resolve().parent.parent
CACHE = ROOT / "catalog" / ".cache" / "zim"
CATALOG = ROOT / "site" / "public" / "catalog" / "v1" / "catalog.json"

DROP_PREFIX = ("image/", "font/", "text/css", "application/font", "application/vnd.ms-fontobject", "application/wasm",
               "application/octet-stream+xapian", "application/octet-stream+zimlisting")
DROP_EXACT = {"application/x-font-woff", "application/x-font-ttf", "application/font-woff", "application/font-woff2"}
SCRIPTS = ("application/javascript", "text/javascript")


def clean(s):
    return s.replace("\t", " ").replace("\n", " ").replace("\r", " ")


def keep(mime, front, url):
    if front:
        return True
    if mime.startswith(DROP_PREFIX) or mime in DROP_EXACT:
        return False
    if mime in SCRIPTS:
        # Scripts are machinery, except the few small data files some
        # scrapers ship their item lists in (zimit document libraries).
        base = url.rsplit("/", 1)[-1]
        return base in ("database.js", "data.js") or url.count("/") == 0
    return True


def list_archive(item, tmpdir, cache=None):
    cache = cache or CACHE
    out = cache / (item["file"] + ".list.tsv.gz")
    meta_path = cache / (item["file"] + ".meta.json")
    if out.exists() and meta_path.exists():
        return json.loads(meta_path.read_text())
    t0 = time.time()
    src = zim.open_source(item["url"], item["size"])
    z = zim.Zim(src)
    up = z.url_ptrs()
    tmp = Path(tmpdir) / (item["file"] + ".dir")
    base, _ = z.fetch_directory(tmp)
    with open(tmp, "rb") as f:
        buf = f.read()
    try:
        mimes = z.mimes

        # Front articles: X/listing/titleOrdered/v1 (libzim 7+), else namespace A html (old archives).
        front = bytearray(len(up))
        idx, d = z.find(buf, base, "X", "listing/titleOrdered/v1")
        listing = "v1"
        if d is not None and d[2] is not None:
            data = z.blob(d[2], d[3])
            for i in struct.unpack("<%dI" % (len(data) // 4), data):
                front[i] = 1
        else:
            listing = "namespace"
            for i, p in enumerate(up):
                mime, ns, *_ = zim.Zim.parse_dirent(buf, p - base)
                if ns == "A" and mime < len(mimes) and mimes[mime].startswith("text/html"):
                    front[i] = 1

        meta = {}
        counts = collections.Counter()
        kept = 0
        tmp_out = out.with_suffix(".part")
        with gzip.open(tmp_out, "wt", encoding="utf-8", compresslevel=6) as g:
            for i, p in enumerate(up):
                mime, ns, cl, bl, url, title, target = zim.Zim.parse_dirent(buf, p - base)
                if mime == zim.REDIRECT:
                    counts["redirect" + (" front" if front[i] else "")] += 1
                    continue
                if mime in (zim.LINKTARGET, zim.DELETED):
                    counts["linktarget"] += 1
                    continue
                m = mimes[mime] if mime < len(mimes) else "?"
                if ns == "M":
                    counts["metadata"] += 1
                    if m.startswith("text/plain") and url in ("Title", "Description", "LongDescription", "Date", "Creator",
                                                              "Publisher", "Tags", "Counter", "Language", "Name", "Flavour",
                                                              "Scraper", "Source", "License"):
                        meta[url] = (cl, bl)
                    continue
                if ns in ("X", "W", "Z"):
                    counts["index " + ns] += 1
                    continue
                f = front[i]
                if keep(m, f, url):
                    kept += 1
                    counts[("front " if f else "kept ") + m.split(";")[0]] += 1
                    g.write(f"{i}\t{ns}\t{m.split(';')[0]}\t{f}\t{cl}\t{bl}\t{clean(url)}\t{clean(title)}\n")
                else:
                    counts["dropped " + m.split(";")[0]] += 1
        tmp_out.rename(out)

        # Metadata values, grouped by cluster so each cluster is read once.
        values = {}
        by_cluster = collections.defaultdict(list)
        for k, (cl, bl) in meta.items():
            by_cluster[cl].append((k, bl))
        for cl, ks in by_cluster.items():
            offs, data = z.cluster(cl)
            for k, bl in ks:
                values[k] = data[offs[bl]:offs[bl + 1]].decode("utf-8", "replace")
        info = {
            "id": item["id"], "file": item["file"], "url": src.url if hasattr(src, "url") else item["url"],
            "size": src.size, "zim_version": f"{z.major}.{z.minor}", "entries": z.entry_count, "clusters": z.cluster_count,
            "front_listing": listing, "kept": kept, "directory_verified_bytes": getattr(z, "verified", 0), "counts": dict(sorted(counts.items())), "metadata": values,
            "fetched_bytes": getattr(src, "bytes", 0), "seconds": round(time.time() - t0, 1),
        }
        tmp_meta = meta_path.with_suffix(".part")
        tmp_meta.write_text(json.dumps(info, indent=1, ensure_ascii=False))
        tmp_meta.rename(meta_path)
        return info
    finally:
        tmp.unlink(missing_ok=True)


def verify_archive(item, tmpdir):
    """List an archive again into a side folder, with the directory checked
    against Kiwix's piece hashes, and mark the existing listing verified only
    if the new one is identical (facts are keyed by its row numbers)."""
    meta_path = CACHE / (item["file"] + ".meta.json")
    old = json.loads(meta_path.read_text())
    if old.get("directory_verified_bytes"):
        return old
    side = CACHE / "verify"
    side.mkdir(exist_ok=True)
    for ext in (".list.tsv.gz", ".meta.json"):
        (side / (item["file"] + ext)).unlink(missing_ok=True)
    info = list_archive(item, tmpdir, side)
    if not info["directory_verified_bytes"]:
        raise RuntimeError("the directory did not match Kiwix's piece hashes")
    a = gzip.open(CACHE / (item["file"] + ".list.tsv.gz")).read()
    b = gzip.open(side / (item["file"] + ".list.tsv.gz")).read()
    if a != b:
        raise RuntimeError("the verified listing differs from the one measured; re-list and re-measure this archive")
    old["directory_verified_bytes"] = info["directory_verified_bytes"]
    tmp_meta = meta_path.with_suffix(".part")
    tmp_meta.write_text(json.dumps(old, indent=1, ensure_ascii=False))
    tmp_meta.rename(meta_path)
    for ext in (".list.tsv.gz", ".meta.json"):
        (side / (item["file"] + ext)).unlink(missing_ok=True)
    return old


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--only", nargs="*")
    ap.add_argument("--jobs", type=int, default=3)
    ap.add_argument("--tmp", default=os.environ.get("TMPDIR", "/tmp"))
    ap.add_argument("--verify", action="store_true", help="check listings made before directories were verified")
    ap.add_argument("--skip", nargs="*", default=[])
    args = ap.parse_args()
    CACHE.mkdir(parents=True, exist_ok=True)
    cat = json.loads(CATALOG.read_text())
    zims = [i for i in cat["items"] if i["kind"] == "zim" and (not args.only or i["id"] in args.only) and i["id"] not in args.skip]
    zims.sort(key=lambda i: i["size"])
    failed = []
    with cf.ThreadPoolExecutor(args.jobs) as ex:
        futs = {ex.submit(verify_archive if args.verify else list_archive, i, args.tmp): i for i in zims}
        for f in cf.as_completed(futs):
            i = futs[f]
            try:
                info = f.result()
                print(f"{i['id']:45} {info['entries']:>10,} entries {info['kept']:>10,} kept  "
                      f"{info['fetched_bytes']/1e6:8.1f} MB  {info['seconds']}s", flush=True)
            except Exception as e:
                failed.append(i["id"])
                print(f"{i['id']:45} FAILED {e!r}", flush=True)
    if failed:
        sys.exit(f"failed: {failed}")


if __name__ == "__main__":
    main()
