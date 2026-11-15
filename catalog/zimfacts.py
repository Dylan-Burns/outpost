#!/usr/bin/env python3
"""Pass B of the content index: measure what pass A listed.

For one archive this reads, by range request:
  - the offset table at the start of every cluster that holds a kept entry,
    which gives each entry's exact size (no decompression of the content);
  - whole clusters only where the archive's family needs the content: small
    descriptor files (a library's document list, a channel's video list),
    pages whose facts are only in the page (a question's tags and dates);
  - each PDF in full, for its page count, and the first and last 256 KB of
    each video, for its length.

Writes catalog/.cache/zim/<file>.facts.jsonl.gz: one JSON object per entry
that has any fact: {"i": index, "s": size, ...facts}. Progress is kept in a
.part file so an interrupted run continues where it stopped.

Usage: python3 catalog/zimfacts.py [--only ID ...] [--jobs 2]
"""
import argparse
import collections
import concurrent.futures as cf
import gzip
import json
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import media  # noqa: E402
import sources  # noqa: E402
import zim  # noqa: E402

ROOT = Path(__file__).resolve().parent.parent
CACHE = ROOT / "catalog" / ".cache" / "zim"
CATALOG = ROOT / "site" / "public" / "catalog" / "v1" / "catalog.json"
BATCH_BYTES = 48 << 20


def load_rows(file):
    rows = []
    with gzip.open(CACHE / (file + ".list.tsv.gz"), "rt", encoding="utf-8") as g:
        for line in g:
            p = line.rstrip("\n").split("\t")
            rows.append(sources.Row(int(p[0]), p[1], p[2], p[3] == "1", int(p[4]), int(p[5]), p[6], p[7]))
    return rows


def log(item, msg):
    print(f"  {item['id']}: {msg}", file=sys.stderr, flush=True)


def measure(item, refresh=False):
    out = CACHE / (item["file"] + ".facts.jsonl.gz")
    if out.exists() and not refresh:
        return out
    part = CACHE / (item["file"] + ".facts.part")
    meta = json.loads((CACHE / (item["file"] + ".meta.json")).read_text())
    rows = load_rows(item["file"])
    fam = sources.family(item, meta)
    src = zim.open_source(item["url"], item["size"])
    z = zim.Zim(src)
    spans = z.cluster_spans()

    done_clusters = set()
    facts = collections.defaultdict(dict)
    if part.exists():
        for line in part.read_text().splitlines():
            rec = json.loads(line)
            if "done" in rec:
                done_clusters.update(rec["done"])
            else:
                facts[rec["i"]].update(rec)
    pf = open(part, "a")

    def flush(recs, clusters):
        for r in recs:
            pf.write(json.dumps(r, ensure_ascii=False) + "\n")
        pf.write(json.dumps({"done": sorted(clusters)}) + "\n")
        pf.flush()

    by_cluster = collections.defaultdict(list)
    for r in rows:
        by_cluster[r.cluster].append(r)
    wanted = fam.wanted(rows)  # set of row indices whose content the family reads
    pdfs = {r.idx for r in rows if r.mime == "application/pdf" and fam.is_item_file(r)}
    videos = {r.idx for r in rows if (r.mime.startswith("video/") or r.mime.startswith("audio/")) and fam.is_item_file(r)
              and not r.mime.startswith(("video/mp2t", "audio/midi", "audio/x-mpegurl")) and fam.needs_duration(r)}

    # 1. Clusters read in full: compressed clusters holding wanted entries.
    #    Their offset tables come with them.
    heads_needed = set(by_cluster) - done_clusters
    t0 = time.time()
    first = {}
    if heads_needed:
        log(item, f"{len(heads_needed)} cluster tables")
        order = sorted(heads_needed, key=lambda c: spans[c][0])
        for s in range(0, len(order), 2000):
            chunk = order[s:s + 2000]
            first.update(zim.cluster_offsets(src, spans, chunk))
    full = sorted({c for c in by_cluster if c not in done_clusters and first.get(c, (False,))[0]
                   and any(r.idx in wanted for r in by_cluster[c])}, key=lambda c: spans[c][0])
    if full:
        log(item, f"{len(full)} clusters to read ({sum(spans[c][1] for c in full) / 1e9:.2f} GB)")

    # Sizes for every cluster not read in full.
    recs, batch_done = [], []
    for c, (comp, offs) in first.items():
        if c in full:
            continue
        for r in by_cluster[c]:
            if r.blob + 1 < len(offs):
                facts[r.idx]["s"] = offs[r.blob + 1] - offs[r.blob]
                recs.append({"i": r.idx, "s": facts[r.idx]["s"]})
        batch_done.append(c)
    # Uncompressed clusters may still hold wanted entries (descriptor files stored raw).
    raw_wanted = [r for r in rows if r.idx in wanted and r.cluster in first and not first[r.cluster][0]
                  and "x" not in facts[r.idx] and r.cluster not in done_clusters]
    if raw_wanted:
        offs_of = {c: first[c][1] for c in {r.cluster for r in raw_wanted}}
        spans_r = [(spans[r.cluster][0] + 1 + offs_of[r.cluster][r.blob], offs_of[r.cluster][r.blob + 1] - offs_of[r.cluster][r.blob])
                   for r in raw_wanted]
        for r, data in zip(raw_wanted, src.read_many(spans_r)):
            x = fam.extract(r, data)
            if x:
                facts[r.idx].update(x)
                recs.append({"i": r.idx, **x})
    flush(recs, batch_done)
    done_clusters.update(batch_done)

    # 2. Full clusters, in large sequential reads.
    i = 0
    read = 0
    total = sum(spans[c][1] for c in full)
    while i < len(full):
        group = [full[i]]
        size = spans[full[i]][1]
        i += 1
        while i < len(full) and size < BATCH_BYTES:
            prev_end = spans[group[-1]][0] + spans[group[-1]][1]
            if spans[full[i]][0] - prev_end > (1 << 20):
                break
            group.append(full[i])
            size += spans[full[i]][1] + (spans[full[i]][0] - prev_end)
            i += 1
        start = spans[group[0]][0]
        end = spans[group[-1]][0] + spans[group[-1]][1]
        blob = z._read_big(start, end - start)
        recs = []
        for c in group:
            o = spans[c][0] - start
            offs, data = zim.decode_cluster(blob[o:o + spans[c][1]])
            for r in by_cluster[c]:
                if r.blob + 1 >= len(offs):
                    continue
                b = data[offs[r.blob]:offs[r.blob + 1]]
                rec = {"i": r.idx, "s": len(b)}
                if r.idx in wanted:
                    try:
                        x = fam.extract(r, b)
                    except Exception as e:  # one odd page must not stop the archive
                        x = {"err": repr(e)[:80]}
                    if x:
                        rec.update(x)
                facts[r.idx].update(rec)
                recs.append(rec)
        flush(recs, group)
        done_clusters.update(group)
        read += end - start
        log(item, f"read {read / 1e9:.2f} of {total / 1e9:.2f} GB, {read / max(1, time.time() - t0) / 1e6:.1f} MB/s")

    # 3. PDFs: the whole file, for pages and date.
    todo = [r for r in rows if r.idx in pdfs and "pages" not in facts[r.idx] and "pdf" not in facts[r.idx]]
    if todo:
        log(item, f"{len(todo)} PDFs")
    for r in todo:
        try:
            comp, offs = first.get(r.cluster) or zim.cluster_offsets(src, spans, [r.cluster])[r.cluster]
            if comp:
                offs, data = z.cluster(r.cluster)
                b = data[offs[r.blob]:offs[r.blob + 1]]
            else:
                b = z._read_big(spans[r.cluster][0] + 1 + offs[r.blob], offs[r.blob + 1] - offs[r.blob])
            x = media.pdf_facts(b)
        except Exception as e:
            x = {"err": repr(e)[:80]}
        x["pdf"] = 1
        facts[r.idx].update(x)
        flush([{"i": r.idx, **x}], [])

    # 4. Videos: first and last 256 KB, for the length.
    todo = [r for r in rows if r.idx in videos and "secs" not in facts[r.idx] and "vid" not in facts[r.idx]]
    if todo:
        log(item, f"{len(todo)} videos")
    for s in range(0, len(todo), 100):
        chunk = todo[s:s + 100]
        where = {}
        for r in chunk:
            comp, offs = first.get(r.cluster) or zim.cluster_offsets(src, spans, [r.cluster])[r.cluster]
            if comp:
                continue  # videos are stored uncompressed; skip the odd one that is not
            base = spans[r.cluster][0] + 1 + offs[r.blob]
            where[r.idx] = (base, offs[r.blob + 1] - offs[r.blob])
        heads = dict(zip(where, src.read_many([(b, min(n, 64 << 10)) for b, n in where.values()])))
        recs, tails = [], {}
        for r in chunk:
            if r.idx not in heads:
                continue
            secs = media.media_seconds(heads[r.idx])
            if secs is None and heads[r.idx][4:8] == b"ftyp":
                tails[r.idx] = r
                continue
            x = {"vid": 1, **({"secs": round(secs)} if secs else {})}
            facts[r.idx].update(x)
            recs.append({"i": r.idx, **x})
        if tails:
            got = src.read_many([(where[i][0] + where[i][1] - min(where[i][1], 512 << 10), min(where[i][1], 512 << 10)) for i in tails])
            for (i, r), tail in zip(tails.items(), got):
                secs = media.media_seconds(heads[i], tail)
                x = {"vid": 1, **({"secs": round(secs)} if secs else {})}
                facts[i].update(x)
                recs.append({"i": i, **x})
        flush(recs, [])

    pf.close()
    tmp = out.with_suffix(".tmp")
    with gzip.open(tmp, "wt", encoding="utf-8") as g:
        for idx in sorted(facts):
            g.write(json.dumps({"i": idx, **{k: v for k, v in facts[idx].items() if k != "i"}}, ensure_ascii=False) + "\n")
    tmp.rename(out)
    part.unlink(missing_ok=True)
    log(item, f"done in {time.time() - t0:.0f}s, {getattr(src, 'bytes', 0) / 1e6:.0f} MB fetched")
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--only", nargs="*")
    ap.add_argument("--skip", nargs="*", default=[])
    ap.add_argument("--jobs", type=int, default=2)
    ap.add_argument("--max-gb", type=float, default=1e9, help="only archives up to this size")
    ap.add_argument("--refresh", action="store_true")
    args = ap.parse_args()
    cat = json.loads(CATALOG.read_text())
    zims = [i for i in cat["items"] if i["kind"] == "zim" and (not args.only or i["id"] in args.only)
            and i["id"] not in args.skip and i["size"] <= args.max_gb * 1e9
            and (CACHE / (i["file"] + ".meta.json")).exists()]
    zims.sort(key=lambda i: i["size"])
    failed = []
    with cf.ThreadPoolExecutor(args.jobs) as ex:
        futs = {ex.submit(measure, i, args.refresh): i for i in zims}
        for f in cf.as_completed(futs):
            i = futs[f]
            try:
                f.result()
                print(f"{i['id']:45} measured", flush=True)
            except Exception as e:
                failed.append(i["id"])
                import traceback
                traceback.print_exc()
                print(f"{i['id']:45} FAILED {e!r}", flush=True)
    if failed:
        sys.exit(f"failed: {failed}")


if __name__ == "__main__":
    main()
