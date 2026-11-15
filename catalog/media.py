"""Measure one item from its bytes: pages of a PDF, minutes of a video, words of a page.

Standard library only, except ffprobe (from ffmpeg) as a fallback for video
containers this module cannot read itself.
"""
import html
import re
import shutil
import struct
import subprocess
import zlib

# --- PDF ----------------------------------------------------------------------

_PAGES_COUNT = re.compile(rb"/Type\s*/Pages\b[^>]*?/Count\s+(\d+)|/Count\s+(\d+)[^>]*?/Type\s*/Pages\b", re.S)
_PAGE = re.compile(rb"/Type\s*/Page(?![a-zA-Z])")
_STREAM = re.compile(rb"stream\r?\n(.*?)\r?\nendstream", re.S)
_INFO_DATE = re.compile(rb"/(ModDate|CreationDate)\s*\(D:(\d{4})(\d{2})?(\d{2})?")
_INFO_TITLE = re.compile(rb"/Title\s*\(((?:\\.|[^\\)])*)\)")


MAX_STREAM = 64 << 20  # an object stream that inflates past this is not read


def _inflated(data):
    for m in _STREAM.finditer(data):
        try:
            yield zlib.decompressobj().decompress(m.group(1), MAX_STREAM)
        except Exception:
            continue


def pdf_facts(data):
    """{'pages': n, 'date': 'YYYY-MM', 'title': str} from a whole PDF file.

    The root page tree's /Count is the page count. In PDFs whose objects sit
    in compressed object streams the tree is found after inflating them.
    Falls back to counting page objects."""
    out = {}
    counts = [int(a or b) for a, b in _PAGES_COUNT.findall(data)]
    if not counts:
        for chunk in _inflated(data):
            counts += [int(a or b) for a, b in _PAGES_COUNT.findall(chunk)]
    if counts:
        out["pages"] = max(counts)
    else:
        n = len(_PAGE.findall(data))
        if not n:
            n = sum(len(_PAGE.findall(c)) for c in _inflated(data))
        if n:
            out["pages"] = n
    dates = {}
    for kind, y, mo, _d in _INFO_DATE.findall(data):
        if 1980 <= int(y) <= 2100:
            dates[kind] = y.decode() + ("-" + mo.decode() if mo else "")
    if dates:
        out["date"] = dates.get(b"ModDate") or dates.get(b"CreationDate")
    t = _INFO_TITLE.search(data)
    if t:
        raw = t.group(1)
        try:
            if raw.startswith(b"\xfe\xff"):
                title = raw[2:].decode("utf-16-be", "ignore")
            else:
                title = raw.decode("latin-1")
            title = re.sub(r"\\(.)", r"\1", title).strip()
            if 3 <= len(title) <= 200 and not title.lower().startswith(("microsoft word", "untitled")):
                out["title"] = title
        except Exception:
            pass
    return out


# --- video and audio ----------------------------------------------------------

def _mp4_duration(data):
    i = data.find(b"mvhd")
    if i < 4:
        return None
    v = data[i + 4]
    try:
        if v == 1:
            ts, dur = struct.unpack(">IQ", data[i + 24:i + 36])
        else:
            ts, dur = struct.unpack(">II", data[i + 16:i + 24])
    except struct.error:
        return None
    return dur / ts if ts else None


def _ebml_duration(data):
    """WebM/Matroska: Segment Info carries TimecodeScale (2AD7B1) and Duration (4489, float)."""
    i = data.find(b"\x44\x89")
    if i < 0:
        return None
    size_byte = data[i + 2]
    if size_byte == 0x88:
        val = struct.unpack(">d", data[i + 3:i + 11])[0]
    elif size_byte == 0x84:
        val = struct.unpack(">f", data[i + 3:i + 7])[0]
    else:
        return None
    scale = 1_000_000
    j = data.find(b"\x2a\xd7\xb1")
    if 0 <= j < i + 64:
        n = data[j + 3] & 0x7F if data[j + 3] & 0x80 else None
        if n and n <= 8:
            scale = int.from_bytes(data[j + 4:j + 4 + n], "big")
    return val * scale / 1e9


def media_seconds(head, tail=b""):
    """Duration in seconds from the first (and, for MP4, last) bytes of a file."""
    if head[4:8] == b"ftyp":
        return _mp4_duration(head) or (_mp4_duration(tail) if tail else None)
    if head[:4] == b"\x1a\x45\xdf\xa3":
        d = _ebml_duration(head)
        if d:
            return d
    if shutil.which("ffprobe"):
        try:
            p = subprocess.run(["ffprobe", "-v", "quiet", "-show_entries", "format=duration", "-of", "csv=p=0", "-i", "pipe:0"],
                               input=head, capture_output=True, timeout=30)
            return float(p.stdout.strip())
        except Exception:
            return None
    return None


# --- text -----------------------------------------------------------------------

_DROP = re.compile(r"<(script|style|svg|noscript|head)\b.*?</\1\s*>", re.S | re.I)
_TAG = re.compile(r"<[^>]+>")
_WORD = re.compile(r"[^\W_]+(?:['’-][^\W_]+)*")


def html_text(h):
    h = _DROP.sub(" ", h)
    return html.unescape(_TAG.sub(" ", h))


def words(text):
    return len(_WORD.findall(text))
