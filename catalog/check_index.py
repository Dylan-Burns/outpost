#!/usr/bin/env python3
"""Check the content index root the site ships against the signed catalog.

  python3 catalog/check_index.py

Passes when site/public/catalog/v1/index/root.json is signed with the release
key (namespace outpost-content-index), and every download in the catalog
appears in it with the same file and SHA-256, so the Content page never
describes files the catalog no longer offers. Exits 0 (or 0 with a note when
there is no index yet), 1 with the reasons otherwise. Standard library and
ssh-keygen only, so it runs on any CI image.
"""
import json
import re
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
CATALOG = ROOT / "site" / "public" / "catalog" / "v1" / "catalog.json"
INDEX = ROOT / "site" / "public" / "catalog" / "v1" / "index" / "root.json"
NAMESPACE = "outpost-content-index"


def release_key():
    sh = (ROOT / "install" / "install.sh").read_text()
    m = re.search(r'RELEASE_KEY="(ssh-ed25519 [A-Za-z0-9+/=]+)', sh)
    if not m:
        sys.exit("release key not found in install/install.sh")
    return m.group(1)


def main():
    if not INDEX.exists():
        print("no content index root yet; nothing to check")
        return
    problems = []
    sig = INDEX.with_suffix(".json.sig")
    if not sig.exists():
        problems.append("root.json is not signed")
    else:
        with tempfile.NamedTemporaryFile("w", suffix=".allowed", delete=False) as f:
            f.write(f"outpost-release namespaces=\"{NAMESPACE}\" {release_key()}\n")
        with open(INDEX, "rb") as data:
            r = subprocess.run(["ssh-keygen", "-Y", "verify", "-f", f.name, "-I", "outpost-release", "-n", NAMESPACE, "-s", str(sig)],
                               stdin=data, capture_output=True, text=True)
        Path(f.name).unlink()
        if r.returncode != 0:
            problems.append(f"root.json signature does not verify: {r.stderr.strip() or r.stdout.strip()}")
    cat = json.loads(CATALOG.read_text())
    try:
        root = json.loads(INDEX.read_text())
    except ValueError as e:
        problems.append(f"root.json is not valid JSON: {e}")
        print("content index does not match the catalog:\n  " + "\n  ".join(problems))
        sys.exit(1)
    # The page fetches the index from root.base; the site's CSP must allow that host.
    base = root.get("base", "")
    if base.startswith("https://"):
        host = "/".join(base.split("/")[:3])
        csp = (ROOT / "site" / "vercel.json").read_text()
        m = re.search(r"connect-src ([^;\"]*)", csp)
        if not m or host not in m.group(1).split():
            problems.append(f"site/vercel.json's connect-src does not allow {host}, where the index is served")
    cols = {c["id"]: c for s in root["subjects"] for c in s["collections"]}
    for it in cat["items"]:
        if it["kind"] != "zim":
            continue
        c = cols.get(it["id"])
        if not c:
            problems.append(f"{it['id']} is in the catalog but not in the content index")
        elif c.get("sha256") != it["sha256"] or c.get("file") != it["file"]:
            problems.append(f"{it['id']}: the index describes {c.get('file')}, the catalog offers {it['file']}")
    zims = {i["id"] for i in cat["items"] if i["kind"] == "zim"}
    for cid, c in cols.items():
        if c.get("kind") == "zim" and cid not in zims:
            problems.append(f"{cid} is in the content index but no longer in the catalog")
    regions = sum(1 for i in cat["items"] if i["kind"] == "map-region")
    if root["totals"]["mapRegions"] != regions:
        problems.append(f"the index counts {root['totals']['mapRegions']} map regions, the catalog has {regions}")
    models = sum(1 for i in cat["items"] if i["kind"] == "model")
    if root["totals"]["models"] != models:
        # A model-only catalog update must not block CI; the page notes it until the index is rebuilt.
        print(f"note: the index lists {root['totals']['models']} AI models, the catalog has {models}; "
              "rebuild with catalog/publish_index.py")
    if problems:
        print("content index does not match the catalog:\n  " + "\n  ".join(problems))
        sys.exit(1)
    print(f"content index {root['generated']} matches catalog sequence {cat['sequence']}: "
          f"{root['totals']['items']:,} items in {len(cols)} collections")


if __name__ == "__main__":
    main()
