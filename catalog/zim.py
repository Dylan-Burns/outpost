"""Read the structure of a ZIM archive without downloading it.

A ZIM keeps its directory (one entry per path: type, cluster, blob, path,
title) in one block, usually at the end of the file, and its content in
clusters. Everything here works the same on a local file or, through HTTP
range requests, on a file on a Kiwix mirror, so the content index can list
every entry of a 221 GB archive by fetching tens of megabytes.

Format: https://wiki.openzim.org/wiki/ZIM_file_format
Standard library only (Python 3.14+ for zstd).
"""
import array
import http.client
import lzma
import os
import struct
import time
import urllib.parse
import urllib.request
from compression import zstd

UA = "outpost-catalog-builder/1 (+https://projectoutpost.org)"

REDIRECT, LINKTARGET, DELETED = 0xFFFF, 0xFFFE, 0xFFFD
MAX_CLUSTER = 512 << 20  # a cluster that inflates past this is refused (real ones are a few MB)
COMP_NONE, COMP_XZ, COMP_ZSTD = 1, 4, 5


class LocalSource:
    def __init__(self, path):
        self.f = open(path, "rb")
        self.size = os.path.getsize(path)
        self.name = path

    def read(self, off, n):
        self.f.seek(off)
        return self.f.read(n)

    def read_many(self, spans):
        return [self.read(o, n) for o, n in spans]

    def copy_to(self, off, n, out):
        self.f.seek(off)
        while n:
            b = self.f.read(min(n, 1 << 22))
            out.write(b)
            n -= len(b)


class HttpSource:
    """Byte ranges of one file on a mirror, over one kept-alive connection.

    download.kiwix.org redirects each request to a mirror; the mirror is
    resolved once and then used for every range."""

    MAX_RANGES = 120  # per request; keeps the Range header well under 8 KB

    def __init__(self, url, expect_size=None):
        self.name = url
        final, size = self._resolve(url)
        if expect_size and size != expect_size:
            raise RuntimeError(f"{url}: mirror has {size} bytes, catalog says {expect_size}")
        self.url = final
        self.size = size
        u = urllib.parse.urlsplit(final)
        self.host, self.path, self.https = u.netloc, u.path, u.scheme == "https"
        self.conn = None
        self.bytes = 0
        self.requests = 0

    MIRRORS = []  # mirror bases ("https://host/kiwix/zim/") that answered before

    @classmethod
    def _resolve(cls, url):
        """A mirror that answered before is tried first (Kiwix's redirector can
        take most of a minute); otherwise the redirect is followed once."""
        rel = url.split("/zim/", 1)[1] if "/zim/" in url else None
        for base in list(cls.MIRRORS) if rel else []:
            try:
                return cls._probe(base + rel)
            except Exception:
                continue
        u, n = cls._follow(url)
        if rel and u.endswith("/" + rel):
            base = u[: -len(rel)]
            if base not in cls.MIRRORS:
                cls.MIRRORS.append(base)
        return u, n

    @staticmethod
    def _probe(u):
        parts = urllib.parse.urlsplit(u)
        if parts.scheme != "https":
            raise RuntimeError(f"not https: {u}")
        c = http.client.HTTPSConnection(parts.netloc, timeout=15) if parts.scheme == "https" else http.client.HTTPConnection(parts.netloc, timeout=15)
        c.request("GET", parts.path, headers={"Range": "bytes=0-0", "User-Agent": UA})
        r = c.getresponse()
        r.read()
        c.close()
        if r.status != 206:
            raise RuntimeError(f"HTTP {r.status}")
        return u, int(r.getheader("Content-Range").rsplit("/", 1)[1])

    @staticmethod
    def _follow(url):
        """Follow Kiwix's redirects by hand, with short timeouts, to one mirror.
        The size comes from a one-byte range read (some mirrors stall on HEAD)."""
        last = None
        for attempt in range(8):
            u = url
            try:
                for _ in range(6):
                    parts = urllib.parse.urlsplit(u)
                    cls = http.client.HTTPSConnection if parts.scheme == "https" else http.client.HTTPConnection
                    c = cls(parts.netloc, timeout=15)
                    c.request("GET", parts.path, headers={"Range": "bytes=0-0", "User-Agent": UA})
                    r = c.getresponse()
                    r.read()
                    c.close()
                    if r.status in (301, 302, 303, 307, 308):
                        nxt = urllib.parse.urljoin(u, r.getheader("Location"))
                        if urllib.parse.urlsplit(nxt).scheme != "https":
                            raise RuntimeError(f"refusing a redirect away from https: {nxt}")
                        if nxt.rsplit("/", 1)[-1] != url.rsplit("/", 1)[-1]:
                            raise RuntimeError(f"mirror redirected to a different file: {nxt}")
                        u = nxt
                        continue
                    if r.status == 206:
                        return u, int(r.getheader("Content-Range").rsplit("/", 1)[1])
                    raise RuntimeError(f"HTTP {r.status}")
                raise RuntimeError("too many redirects")
            except Exception as e:
                last = e
                time.sleep(2 * (attempt + 1))
        raise RuntimeError(f"{url}: {last!r}")

    def _connect(self):
        cls = http.client.HTTPSConnection if self.https else http.client.HTTPConnection
        self.conn = cls(self.host, timeout=30)

    def _get(self, rng):
        for i in range(8):
            try:
                if self.conn is None:
                    self._connect()
                self.conn.request("GET", self.path, headers={"Range": "bytes=" + rng, "User-Agent": UA})
                r = self.conn.getresponse()
                if r.status in (429, 500, 502, 503, 504):
                    r.read()
                    raise RuntimeError(f"HTTP {r.status}")
                if r.status != 206:
                    r.read()
                    raise RuntimeError(f"{self.url}: HTTP {r.status} for a range")
                self.requests += 1
                return r
            except Exception as e:
                import sys
                print(f"  retry {i + 1} {self.host}: {e!r}", file=sys.stderr, flush=True)
                try:
                    self.conn.close()
                except Exception:
                    pass
                self.conn = None
                if i == 7:
                    raise
                time.sleep(min(60, 2 ** i))

    def read(self, off, n):
        if n <= 0:
            return b""
        for i in range(5):
            r = self._get(f"{off}-{off + n - 1}")
            try:
                data = r.read()
            except Exception as e:
                import sys
                print(f"  re-read {self.host}: {e!r}", file=sys.stderr, flush=True)
                self.conn = None
                continue
            if len(data) == n:
                self.bytes += n
                return data
        raise RuntimeError(f"{self.url}: short read at {off}")

    def copy_to(self, off, n, out):
        """Stream a large range into a file, resuming after a dropped connection."""
        done = 0
        while done < n:
            r = self._get(f"{off + done}-{off + n - 1}")
            try:
                while True:
                    b = r.read(1 << 20)
                    if not b:
                        break
                    out.write(b)
                    done += len(b)
                    self.bytes += len(b)
            except Exception:
                self.conn = None
        if done != n:
            raise RuntimeError(f"{self.url}: copied {done} of {n}")

    def read_many(self, spans):
        """Several ranges per request (multipart/byteranges); results in input order."""
        out = [None] * len(spans)
        order = sorted(range(len(spans)), key=lambda i: spans[i][0])
        for s in range(0, len(order), self.MAX_RANGES):
            chunk = order[s:s + self.MAX_RANGES]
            if len(chunk) == 1:
                i = chunk[0]
                out[i] = self.read(*spans[i])
                continue
            want = {}
            for i in chunk:
                o, n = spans[i]
                want.setdefault((o, o + n - 1), []).append(i)
            for attempt in range(5):
                r = self._get(",".join(f"{a}-{b}" for a, b in want))
                ctype = r.getheader("Content-Type", "")
                try:
                    body = r.read()
                except Exception:
                    self.conn = None
                    continue
                self.bytes += len(body)
                if "multipart/byteranges" not in ctype:
                    # One range came back (the server merged them): cut it up.
                    cr = r.getheader("Content-Range", "")
                    start = int(cr.split()[1].split("-")[0])
                    for (a, b), idxs in want.items():
                        for i in idxs:
                            out[i] = body[a - start:b - start + 1]
                    break
                boundary = ctype.split("boundary=")[1].strip().strip('"').encode()
                parts = body.split(b"--" + boundary)
                got = {}
                for p in parts:
                    head, sep, data = p.partition(b"\r\n\r\n")
                    if not sep:
                        continue
                    for line in head.split(b"\r\n"):
                        if line.lower().startswith(b"content-range:"):
                            a, b = line.split()[2].split(b"/")[0].split(b"-")
                            got[(int(a), int(b))] = data[: int(b) - int(a) + 1]
                for (a, b), idxs in want.items():
                    if (a, b) in got:
                        for i in idxs:
                            out[i] = got[(a, b)]
                    else:
                        # The server merged overlapping ranges: find the part containing it.
                        for (ga, gb), d in got.items():
                            if ga <= a and b <= gb:
                                for i in idxs:
                                    out[i] = d[a - ga:b - ga + 1]
                                break
                if all(out[i] is not None and len(out[i]) == spans[i][1] for i in chunk):
                    break
            else:
                raise RuntimeError(f"{self.url}: multipart read failed")
        return out


def open_source(loc, expect_size=None):
    if loc.startswith("http://") or loc.startswith("https://"):
        return HttpSource(loc, expect_size)
    return LocalSource(loc)


class Zim:
    def __init__(self, src):
        self.src = src
        h = src.read(0, 80)
        magic, self.major, self.minor = struct.unpack("<IHH", h[:8])
        if magic != 72173914:
            raise ValueError(f"{src.name}: not a ZIM file")
        (self.entry_count, self.cluster_count, self.url_ptr_pos, self.title_ptr_pos, self.cluster_ptr_pos,
         self.mime_list_pos, self.main_page, self.layout_page, self.checksum_pos) = struct.unpack("<IIQQQQIIQ", h[24:80])
        m = src.read(self.mime_list_pos, 8192)
        mimes = []
        pos = 0
        while True:
            end = m.index(b"\0", pos)
            if end == pos:
                break
            mimes.append(m[pos:end].decode())
            pos = end + 1
        self.mimes = mimes
        self._cluster_ptrs = None
        self._url_ptrs = None

    # --- pointer lists -------------------------------------------------------

    def url_ptrs(self):
        if self._url_ptrs is None:
            a = array.array("Q")
            a.frombytes(self._read_big(self.url_ptr_pos, 8 * self.entry_count))
            self._url_ptrs = a
        return self._url_ptrs

    def cluster_ptrs(self):
        if self._cluster_ptrs is None:
            a = array.array("Q")
            a.frombytes(self._read_big(self.cluster_ptr_pos, 8 * self.cluster_count))
            self._cluster_ptrs = a
        return self._cluster_ptrs

    def _read_big(self, off, n):
        out = bytearray()
        step = 64 << 20
        while len(out) < n:
            out += self.src.read(off + len(out), min(step, n - len(out)))
        return bytes(out)

    def cluster_spans(self):
        """(start, compressed length) of every cluster, in cluster order."""
        ptrs = self.cluster_ptrs()
        bounds = sorted(set(list(ptrs) + [p for p in (self.url_ptr_pos, self.title_ptr_pos, self.cluster_ptr_pos,
                                                      self.checksum_pos, self.src.size) if p < (1 << 63)]))
        import bisect
        spans = []
        up = self.url_ptrs()
        dirent_lo = min(up) if len(up) else self.src.size
        for p in ptrs:
            j = bisect.bisect_right(bounds, p)
            end = bounds[j] if j < len(bounds) else self.src.size
            if p < dirent_lo < end:
                end = dirent_lo
            spans.append((p, end - p))
        return spans

    # --- directory -----------------------------------------------------------

    def fetch_directory(self, tmp_path):
        """Copy the directory block to a local file. Returns (base offset, path)."""
        up = self.url_ptrs()
        lo, hi = min(up), max(up)
        n = hi - lo + 4096
        n = min(n, self.src.size - lo)
        with open(tmp_path, "wb") as out:
            self.src.copy_to(lo, n, out)
        return lo, tmp_path

    @staticmethod
    def parse_dirent(buf, pos):
        mime, plen, ns = struct.unpack_from("<HBc", buf, pos)
        if mime == REDIRECT:
            target = struct.unpack_from("<I", buf, pos + 8)[0]
            cluster = blob = None
            p = pos + 12
        elif mime in (LINKTARGET, DELETED):
            target = cluster = blob = None
            p = pos + 8
        else:
            target = None
            cluster, blob = struct.unpack_from("<II", buf, pos + 8)
            p = pos + 16
        e = buf.index(b"\0", p)
        url = buf[p:e].decode("utf-8", "replace")
        e2 = buf.index(b"\0", e + 1)
        title = buf[e + 1:e2].decode("utf-8", "replace")
        return mime, ns.decode("latin-1"), cluster, blob, url, title or url, target

    def find(self, buf, base, ns, url):
        """Binary search the URL-ordered directory for one path."""
        up = self.url_ptrs()
        lo, hi = 0, len(up) - 1
        key = (ns, url)
        while lo <= hi:
            mid = (lo + hi) // 2
            d = self.parse_dirent(buf, up[mid] - base)
            k = (d[1], d[4])
            if k == key:
                return mid, d
            if k < key:
                lo = mid + 1
            else:
                hi = mid - 1
        return None, None

    # --- content -------------------------------------------------------------

    def cluster(self, idx):
        """Whole decompressed cluster: (offsets, data)."""
        start, n = self.cluster_spans_cached()[idx]
        raw = self._read_big(start, n)
        return decode_cluster(raw)

    def cluster_spans_cached(self):
        if not hasattr(self, "_spans"):
            self._spans = self.cluster_spans()
        return self._spans

    def blob(self, cluster, blob):
        info, data = self.cluster(cluster)
        return data[info[blob]:info[blob + 1]]


def decode_cluster(raw):
    """Decompress a whole cluster and return (offset list, payload)."""
    comp = raw[0] & 0x0F
    ext = bool(raw[0] & 0x10)
    if comp in (0, COMP_NONE):
        data = raw[1:]
    elif comp in (COMP_ZSTD, COMP_XZ):
        d = zstd.ZstdDecompressor() if comp == COMP_ZSTD else lzma.LZMADecompressor()
        data = d.decompress(raw[1:], max_length=MAX_CLUSTER)
        if len(data) >= MAX_CLUSTER and not d.eof:
            raise ValueError(f"cluster inflates past {MAX_CLUSTER} bytes")
    else:
        raise ValueError(f"cluster compression {comp}")
    w = 8 if ext else 4
    fmt = "<Q" if ext else "<I"
    first = struct.unpack_from(fmt, data, 0)[0]
    offs = list(struct.unpack_from(("<%dQ" if ext else "<%dI") % (first // w), data, 0))
    return offs, data


def cluster_head(raw, more=None):
    """Offsets of every blob in a cluster from the first bytes of it.

    raw is a prefix of the cluster. For an uncompressed cluster the offset
    table is read directly; for zstd/xz the prefix is decompressed only as far
    as the table. Returns (compressed, offsets) or (compressed, None) when the
    prefix was too short (the caller fetches more)."""
    comp = raw[0] & 0x0F
    ext = bool(raw[0] & 0x10)
    w = 8 if ext else 4
    fmt = "<Q" if ext else "<I"
    if comp in (0, COMP_NONE):
        data = raw[1:]
    else:
        try:
            if comp == COMP_ZSTD:
                d = zstd.ZstdDecompressor()
            else:
                d = lzma.LZMADecompressor()
            data = d.decompress(raw[1:], max_length=1 << 22)
        except Exception:
            return comp not in (0, COMP_NONE), None
    if len(data) < w:
        return comp not in (0, COMP_NONE), None
    first = struct.unpack_from(fmt, data, 0)[0]
    if len(data) < first:
        return comp not in (0, COMP_NONE), None
    offs = list(struct.unpack_from(("<%dQ" if ext else "<%dI") % (first // w), data, 0))
    return comp not in (0, COMP_NONE), offs


def cluster_offsets(src, spans, idxs, workers=None):
    """Blob offset tables of the given clusters, read from their first bytes.

    Uncompressed clusters need a few KB; compressed ones need their first
    zstd/xz block, so the prefix grows until the table decodes. Returns
    {cluster index: (compressed, offsets)}."""
    out = {}
    want = {i: 4096 for i in idxs}
    while want:
        order = sorted(want)
        reads = src.read_many([(spans[i][0], min(want[i], spans[i][1])) for i in order])
        nxt = {}
        for i, raw in zip(order, reads):
            comp, offs = cluster_head(raw)
            if offs is not None:
                out[i] = (comp, offs)
            elif want[i] >= spans[i][1]:
                raise ValueError(f"cluster {i}: offset table does not decode")
            else:
                nxt[i] = (want[i] * 16) if comp else (want[i] * 4)
        want = nxt
    return out
