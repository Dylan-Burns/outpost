#!/usr/bin/env python3
"""Tests for the content index tools. Run: python3 -I catalog/test_index.py"""
import collections
import gzip
import json
import random
import string
import struct
import sys
import tempfile
import unittest
import zlib
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import media  # noqa: E402
import sources  # noqa: E402
import tree  # noqa: E402
import zim  # noqa: E402
from compression import zstd  # noqa: E402


def words(n, seed=1):
    r = random.Random(seed)
    return ["".join(r.choice(string.ascii_letters + "éß ") for _ in range(r.randint(2, 18))) or "x" for _ in range(n)]


def leaves(node, path=()):
    if node.items:
        yield path, node.items
    for c in node.children.values():
        yield from leaves(c, path + (c.label,))


class TreeTest(unittest.TestCase):
    def check_sizes(self, n, sections=0):
        r = random.Random(n)
        items = [{"t": t, "path": ((f"S{r.randrange(sections)}",) if sections else ()), "s": 1} for t in words(n, n)]
        root = tree.build(items)
        self.assertEqual(tree.check(root), n)
        seen = collections.Counter(id(it) for _p, its in leaves(root) for it in its)
        self.assertEqual(len(seen), n)
        self.assertTrue(all(v == 1 for v in seen.values()), "an item appears twice")
        return root

    def test_small_stays_flat(self):
        root = self.check_sizes(150)
        self.assertFalse(root.children)

    def test_limit_holds_at_every_scale(self):
        for n in (201, 1999, 2001, 40_000, 250_000):
            self.check_sizes(n)

    def test_sections_kept_and_loose_items_shelved(self):
        items = [{"t": t, "path": ("Physics",)} for t in words(500, 2)] + [{"t": t, "path": ()} for t in words(300, 3)]
        root = tree.build(items)
        self.assertEqual(tree.check(root), 800)
        self.assertIn("Physics", root.children)
        self.assertIn("Other pages", root.children)

    def test_ranges_are_in_order_and_labelled(self):
        root = self.check_sizes(5000)
        labels = [c.label for c in root.children.values()]
        self.assertEqual(labels[0], "A")

    def test_serialise_and_read_back(self):
        items = [{"t": t, "path": (), "s": i, "k": "page", "idx": i} for i, t in enumerate(words(1000, 4))]
        root = tree.build(items)
        with tempfile.TemporaryDirectory() as d:
            pack = tree.PackWriter(Path(d), "x", cap=4096)
            got = []
            t = tree.serialise(root, pack, lambda p, its: got.extend(its))
            pack.close()
            self.assertEqual(t["n"], 1000)
            self.assertEqual(t["s"], sum(range(1000)))
            self.assertGreater(pack.n, 0, "small cap should spread leaves over several packs")
            node = t
            while "c" in node:
                node = node["c"][0]
            p, off, ln = node["r"]
            rows = json.loads(gzip.decompress((Path(d) / f"x.{p}.bin").read_bytes()[off:off + ln]))
            self.assertEqual(len(rows), node["n"])
            self.assertNotIn("path", rows[0])
            self.assertEqual(len(got), 1000)

    def test_editions_sum_per_column(self):
        items = [{"t": t, "path": (), "s": 10, "ss": [10, 4, None if i % 2 else -1], "k": "article", "idx": i} for i, t in enumerate(words(450, 5))]
        root = tree.build(items)
        with tempfile.TemporaryDirectory() as d:
            pack = tree.PackWriter(Path(d), "y")
            t = tree.serialise(root, pack)
            pack.close()
        self.assertEqual(t["ss"], [4500, 1800, 0])


def minimal_pdf(pages, compressed=False):
    kids = " ".join(f"{3 + i} 0 R" for i in range(pages))
    objs = [b"1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj",
            f"2 0 obj << /Type /Pages /Kids [{kids}] /Count {pages} >> endobj".encode()]
    objs += [f"{3 + i} 0 obj << /Type /Page /Parent 2 0 R >> endobj".encode() for i in range(pages)]
    body = b"\n".join(objs)
    if compressed:
        z = zlib.compress(body)
        body = b"9 0 obj << /Type /ObjStm /Filter /FlateDecode >>\nstream\n" + z + b"\nendstream endobj"
    return b"%PDF-1.7\n" + body + b"\n/Info << /ModDate (D:20190304120000) /Title (Water treatment) >>\n%%EOF"


class MediaTest(unittest.TestCase):
    def test_pdf_pages_plain_and_in_object_streams(self):
        self.assertEqual(media.pdf_facts(minimal_pdf(7))["pages"], 7)
        self.assertEqual(media.pdf_facts(minimal_pdf(12, compressed=True))["pages"], 12)
        f = media.pdf_facts(minimal_pdf(1))
        self.assertEqual(f["date"], "2019-03")
        self.assertEqual(f["title"], "Water treatment")

    def test_mp4_duration(self):
        mvhd = b"\x00\x00\x00\x6cmvhd" + bytes([0, 0, 0, 0]) + b"\0" * 8 + struct.pack(">II", 1000, 95_000)
        head = b"\x00\x00\x00\x18ftypisom" + b"\0" * 12 + mvhd
        self.assertAlmostEqual(media.media_seconds(head), 95.0)

    def test_webm_duration(self):
        head = b"\x1a\x45\xdf\xa3" + b"\0" * 20 + b"\x2a\xd7\xb1\x83\x0f\x42\x40" + b"\x44\x89\x88" + struct.pack(">d", 61_500.0)
        self.assertAlmostEqual(media.media_seconds(head), 61.5)

    def test_words(self):
        self.assertEqual(media.words(media.html_text("<p>Boil water for <b>one</b> minute.</p><script>x y z</script>")), 5)


class ZimTest(unittest.TestCase):
    def test_cluster_head_needs_whole_first_block(self):
        payload = b"".join(struct.pack("<I", 4 * 4 + i * 100) for i in range(4)) + bytes(300)
        raw = bytes([zim.COMP_ZSTD]) + zstd.compress(payload)
        comp, offs = zim.cluster_head(raw)
        self.assertTrue(comp)
        self.assertEqual(offs, [16, 116, 216, 316])
        self.assertEqual(zim.decode_cluster(raw)[0], offs)
        comp, offs = zim.cluster_head(raw[:4])
        self.assertIsNone(offs, "a truncated prefix must ask for more, not guess")

    def test_cluster_bomb_refused(self):
        raw = bytes([zim.COMP_ZSTD]) + zstd.compress(b"\0" * (zim.MAX_CLUSTER + 1024))
        with self.assertRaises(ValueError):
            zim.decode_cluster(raw)


def rows(*specs):
    out = []
    for i, (url, mime, front, title) in enumerate(specs):
        out.append(sources.Row(i, "C", mime, front, 0, i, url, title or url))
    return out


ITEM = {"id": "x", "book": "", "file": "x.zim"}


class SourcesTest(unittest.TestCase):
    def test_stack_exchange_topics_and_profiles(self):
        r = rows(("questions/1/a", "text/html", True, "Q one"), ("questions/2/b", "text/html", True, "Q two"),
                 ("users/5/ann", "text/html", True, "User Ann"), ("questions/tagged/x", "text/html", False, ""))
        fam = sources.StackExchange(dict(ITEM, book="cooking.stackexchange.com_en_all"), {})
        facts = {0: {"tags": ["\n  ", "bread", "yeast"], "asked": "2014-02-01", "answers": 2},
                 1: {"tags": ["bread"], "asked": "2020-01-01", "answers": 0}}
        its, sk = fam.items(r, facts)
        q = [i for i in its if i["k"] == "question"]
        self.assertEqual({i["path"] for i in q}, {("Questions by topic", "Other topics")})
        self.assertEqual(q[0]["tags"], "bread, yeast")
        self.assertEqual([i["t"] for i in its if i["k"] == "user profile"], ["Ann"])
        self.assertEqual(sum(sk.values()), 1)

    def test_every_row_is_an_item_or_counted(self):
        r = rows(("A", "text/html", True, ""), ("B.pdf", "application/pdf", False, ""), ("c.js", "text/javascript", False, ""))
        its, sk = sources.Family(ITEM, {}).items(r, {})
        self.assertEqual(len(its) + sum(sk.values()), len(r))

    def test_gutenberg_lcc(self):
        self.assertEqual(sources.lcc_path("TK"), ("T · Technology", "TK · Electrical engineering, electronics"))
        self.assertEqual(sources.lcc_path(""), ("Not classified",))
        self.assertEqual(sources.author_key("Twain, Mark, 1835-1910; Smith, A"), "Twain, Mark")

    def test_ted_dates(self):
        self.assertEqual(sources.iso_date("03 May 2012"), "2012-05-03")
        self.assertEqual(sources.iso_date("2012-05-03T10:00"), "2012-05-03")


if __name__ == "__main__":
    unittest.main()
