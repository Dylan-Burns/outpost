"""How each kind of archive is shelved.

Pass A lists every entry of an archive; a family turns those entries into
items (a book, a PDF, a video, an article, a question...) and gives each a
section path that follows the source's own shelving. Every kept entry ends up
either in an item or counted under a named reason, so nothing is lost silently.

A family has three hooks:
  wanted(rows)            entries whose content it needs to read (pass B)
  extract(row, data)      small facts from one entry's content
  items(rows, facts)      the items, and a Counter of entries that are not items
"""
import ast
import collections
import csv
import gzip
import html
import io
import json
import re
import urllib.parse
import urllib.request
from collections import namedtuple
from pathlib import Path

import media

Row = namedtuple("Row", "idx ns mime front cluster blob url title")
ROOT = Path(__file__).resolve().parent.parent
CACHE = ROOT / "catalog" / ".cache"
UA = "outpost-catalog-builder/1 (+https://projectoutpost.org)"

MEDIA_MIMES = ("application/pdf", "application/epub+zip")
GENERIC_TITLES = {"cover", "title", "title page", "untitled", "document", "front cover", "contents", "table of contents"}
MONTHS = {m: i + 1 for i, m in enumerate(("january", "february", "march", "april", "may", "june", "july", "august",
                                          "september", "october", "november", "december"))}


def iso_date(s):
    """'03 May 2012' or '2012-05-03T…' -> '2012-05-03'."""
    s = (s or "").strip()
    m = re.match(r"(\d{4}-\d{2}(?:-\d{2})?)", s)
    if m:
        return m.group(1)
    m = re.match(r"(\d{1,2}) ([A-Za-z]+) (\d{4})", s)
    if m and m.group(2).lower() in MONTHS:
        return f"{m.group(3)}-{MONTHS[m.group(2).lower()]:02d}-{int(m.group(1)):02d}"
    return None


def year_month(s):
    m = re.match(r"(\d{4})-(\d{2})", s or "")
    return f"{m.group(1)}-{m.group(2)}" if m else None


def first_line(s, n=200):
    s = re.sub(r"\s+", " ", (s or "")).strip()
    if len(s) <= n:
        return s
    cut = s[:n].rsplit(" ", 1)[0]
    return cut + "…"


def nice_name(url):
    base = url.rstrip("/").rsplit("/", 1)[-1]
    base = urllib.parse.unquote(base)
    base = re.sub(r"\.(pdf|epub|html?|webm|mp4|mp3|ogg)$", "", base, flags=re.I)
    return base.replace("_", " ").strip() or url


class Family:
    name = "generic"
    item_kind = "page"

    def __init__(self, item, meta):
        self.item = item
        self.meta = meta

    def wanted(self, rows):
        return set()

    def extract(self, row, data):
        return None

    def is_item_file(self, row):
        """Should pass B measure this file as a document or video (pages, length)?"""
        return True

    def needs_duration(self, row):
        return True

    def items(self, rows, facts):
        out, skipped = [], collections.Counter()
        for r in rows:
            k = self.kind_of(r)
            if not k:
                skipped[self.skip_reason(r)] += 1
                continue
            out.append(self.make(r, facts, k, self.path_of(r)))
        return out, skipped

    # defaults: front articles and documents are items, the rest is machinery
    def kind_of(self, r):
        if r.mime == "application/pdf":
            return "PDF"
        if r.mime == "application/epub+zip":
            return "e-book"
        if r.mime.startswith("video/") and not r.mime.startswith("video/mp2t"):
            return "video"
        if r.mime.startswith("audio/") and r.mime != "audio/midi":
            return "audio"
        if r.front:
            return self.item_kind
        return None

    def skip_reason(self, r):
        if r.mime.startswith("text/html"):
            return "navigation and listing pages"
        if r.mime in ("application/json", "text/javascript", "application/javascript"):
            return "data files for the reader"
        if r.mime.startswith("text/vtt"):
            return "subtitles"
        return "other support files"

    def path_of(self, r):
        return ()

    def title_of(self, r, facts):
        f = facts.get(r.idx, {})
        t = f.get("title") if r.mime == "application/pdf" else None
        if t and (t.strip().lower() in GENERIC_TITLES or len(t.strip()) < 4):
            t = None
        if r.title and r.title not in ("null", r.url):
            return html.unescape(r.title)
        return t or nice_name(r.url)

    def make(self, r, facts, kind, path, **extra):
        f = facts.get(r.idx, {})
        it = {"t": self.title_of(r, facts), "k": kind, "path": tuple(path), "s": f.get("s"), "idx": r.idx}
        if "pages" in f:
            it["pg"] = f["pages"]
        if "secs" in f:
            it["min"] = round(f["secs"] / 60, 1)
        if "words" in f:
            it["w"] = f["words"]
        if f.get("date") and kind == "PDF":
            it["d"] = f["date"]
        it.update({k: v for k, v in extra.items() if v not in (None, "")})
        return it


# --- MediaWiki: Wikipedia and its sister projects, Appropedia, energypedia ------------

class MediaWiki(Family):
    name = "mediawiki"
    KINDS = {"wiktionary": "entry", "wikivoyage": "travel guide", "wikiquote": "page", "wikibooks": "chapter",
             "wikiversity": "page", "wikisource": "text", "appropedia": "page", "energypedia": "page"}
    BOOKS = ("wikibooks", "wikiversity", "wikisource", "appropedia", "energypedia")

    def __init__(self, item, meta):
        super().__init__(item, meta)
        book = item["book"]
        self.project = next((k for k in self.KINDS if book.startswith(k)), "")
        self.item_kind = self.KINDS.get(self.project, "article")
        self.subpages = self.project in self.BOOKS

    def items(self, rows, facts):
        out, skipped = [], collections.Counter()
        tops = set()
        if self.subpages:
            for r in rows:
                if r.front and "/" in r.url:
                    tops.add(r.url.split("/", 1)[0])
        for r in rows:
            k = self.kind_of(r)
            if not k:
                skipped[self.skip_reason(r)] += 1
                continue
            path = ()
            if r.url.startswith("_assets_/"):
                path = ("Attached documents",)
            elif self.subpages:
                head = r.url.split("/", 1)[0]
                if head in tops:
                    path = (head.replace("_", " "),)
            kind = k
            if self.subpages and path and path[0] != "Attached documents" and "/" not in r.url:
                kind = {"chapter": "book", "text": "text"}.get(k, k)
            it = self.make(r, facts, kind, path)
            if self.subpages and path and "/" in r.url and path[0] != "Attached documents":
                it["t"] = it["t"].split("/", 1)[1] if "/" in it["t"] else it["t"]
            out.append(it)
        return out, skipped


# --- Stack Exchange ----------------------------------------------------------------------

SE_TAGS = re.compile(r'class="post-tag[^"]*"[^>]*>([^<]+)<')
SE_ASKED = re.compile(r'itemprop="dateCreated" datetime="(\d{4}-\d{2}-\d{2})')
SE_ACTIVE = re.compile(r'Active</span>\s*<time[^>]*datetime="(\d{4}-\d{2}-\d{2})')
SE_SCORE = re.compile(r'id="question"[^>]*data-score="(-?\d+)"')
SE_ANSWERS = re.compile(r'<div id="answer-\d+"')
SE_ANSWERS2 = re.compile(r'data-answerid="\d+"')


def is_question(url):
    """questions/<number>/<slug>, not the per-tag lists under questions/tagged/."""
    p = url.split("/")
    return len(p) == 3 and p[0] == "questions" and p[1].isdigit()


class StackExchange(Family):
    name = "stackexchange"

    def wanted(self, rows):
        return {r.idx for r in rows if r.front and is_question(r.url)}

    def needs_duration(self, row):
        return False

    def extract(self, row, data):
        h = data.decode("utf-8", "replace")
        tags = list(dict.fromkeys(t for t in (html.unescape(t).strip() for t in SE_TAGS.findall(h)) if t))
        x = {}
        if tags:
            x["tags"] = tags[:6]
        m = SE_ASKED.search(h)
        if m:
            x["asked"] = m.group(1)
        m = SE_ACTIVE.search(h)
        if m:
            x["active"] = m.group(1)
        m = SE_SCORE.search(h)
        if m:
            x["score"] = int(m.group(1))
        x["answers"] = len(SE_ANSWERS.findall(h)) or len(SE_ANSWERS2.findall(h))
        x["words"] = media.words(media.html_text(h))
        return x

    MIN_TOPIC = 10

    def items(self, rows, facts):
        out, skipped = [], collections.Counter()
        tag_count = collections.Counter()
        clean = {}
        for i, f in facts.items():
            tags = [t.strip() for t in f.get("tags", []) if t and t.strip()]
            clean[i] = tags
            tag_count.update(tags)
        questions = [r for r in rows if r.front and is_question(r.url)]
        main = {}
        for r in questions:
            tags = clean.get(r.idx) or []
            main[r.idx] = max(tags, key=lambda t: (tag_count[t], t)) if tags else None
        per_main = collections.Counter(main.values())
        topic_of = {i: (m if m and per_main[m] >= self.MIN_TOPIC else ("Other topics" if m else "Not tagged")) for i, m in main.items()}
        per_topic = collections.Counter(topic_of.values())
        for r in questions:
            f = facts.get(r.idx, {})
            topic = topic_of[r.idx]
            path = ("Questions by topic", topic)
            year = (f.get("asked") or "")[:4]
            if year and per_topic[topic] > 200:
                path += (year,)
            it = self.make(r, facts, "question", path)
            tags = clean.get(r.idx) or []
            it.update({k: v for k, v in {"ans": f.get("answers"), "score": f.get("score"), "d": f.get("active") or f.get("asked"),
                                         "asked": f.get("asked"), "tags": ", ".join(tags) if tags else None}.items() if v is not None})
            out.append(it)
        for r in rows:
            if r.front and is_question(r.url):
                continue
            if r.front and r.url.startswith("users/"):
                it = self.make(r, facts, "user profile", ("User profiles",))
                it["t"] = re.sub(r"^User ", "", it["t"])
                out.append(it)
            elif r.front:
                out.append(self.make(r, facts, "page", ("Site pages",)))
            else:
                skipped[self.skip_reason(r)] += 1
        return out, skipped


# --- zimit/nautilus document libraries (the openZIM "zimgit" collections) ----------------

class Nautilus(Family):
    name = "nautilus"

    def wanted(self, rows):
        return {r.idx for r in rows if r.url in ("database.js", "data.js")}

    def extract(self, row, data):
        s = data.decode("utf-8", "replace")
        s = s[s.index("[") : s.rindex("]") + 1]
        try:
            recs = ast.literal_eval(s)
        except Exception:
            recs = json.loads(s)
        return {"db": [{"ti": r.get("ti"), "dsc": r.get("dsc"), "aut": r.get("aut"), "fp": r.get("fp") or []} for r in recs]}

    def items(self, rows, facts):
        db = next((f["db"] for f in facts.values() if "db" in f), [])
        by_path = {r.url: r for r in rows}
        used = set()
        out, skipped = [], collections.Counter()
        for rec in db:
            files = [by_path.get("files/" + p) for p in rec["fp"]]
            files = [f for f in files if f]
            if not files:
                continue
            used.update(f.idx for f in files)
            fs = [facts.get(f.idx, {}) for f in files]
            kinds = {f.mime for f in files}
            kind = "PDF" if kinds == {"application/pdf"} else ("video" if all(k.startswith("video/") for k in kinds) else "document")
            it = {"t": html.unescape(rec.get("ti") or nice_name(files[0].url)), "k": kind, "path": (), "idx": files[0].idx,
                  "s": sum(f.get("s") or 0 for f in fs) if all("s" in f for f in fs) else None}
            pages = [f["pages"] for f in fs if "pages" in f]
            if pages:
                it["pg"] = sum(pages)
            secs = [f["secs"] for f in fs if "secs" in f]
            if secs:
                it["min"] = round(sum(secs) / 60, 1)
            dates = [f["date"] for f in fs if f.get("date")]
            if dates:
                it["d"] = max(dates)
            if rec.get("aut") and rec["aut"] != "Various":
                it["by"] = rec["aut"]
            if rec.get("dsc"):
                it["x"] = first_line(html.unescape(rec["dsc"]))
            if len(files) > 1:
                it["files"] = len(files)
            out.append(it)
        for r in rows:
            if r.idx in used:
                continue
            if r.url.startswith("files/"):
                out.append(self.make(r, facts, self.kind_of(r) or "file", ()))
            else:
                skipped[self.skip_reason(r)] += 1
        return out, skipped


# --- YouTube channels (youtube2zim): CrashCourse, Canadian Prepper, QuickGuidesForMedicine

ISO_DUR = re.compile(r"PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?")


def iso_secs(s):
    m = ISO_DUR.fullmatch(s or "")
    if not m:
        return None
    h, mi, se = (int(x or 0) for x in m.groups())
    return h * 3600 + mi * 60 + se


class YouTube(Family):
    name = "youtube"

    def wanted(self, rows):
        return {r.idx for r in rows if r.mime == "application/json" and (r.url.startswith(("videos/", "playlists/"))
                                                                         or r.url in ("channel.json", "playlists.json"))}

    def needs_duration(self, row):
        return False  # the playlists give each video's length

    def extract(self, row, data):
        d = json.loads(data)
        if row.url.startswith("playlists/"):
            return {"pl": {"title": d.get("title"), "videos": [{"slug": v.get("slug"), "id": v.get("id"), "dur": v.get("duration")}
                                                                for v in d.get("videos", [])]}}
        if row.url.startswith("videos/"):
            return {"vj": {"id": d.get("id"), "title": d.get("title"), "desc": first_line(d.get("description"), 180),
                           "date": d.get("publicationDate") or d.get("publication_date") or d.get("upload_date"),
                           "slug": row.url[len("videos/"):-len(".json")]}}
        if row.url == "channel.json":
            return {"ch": {"uploads": d.get("userLongUploadsPlaylist"), "first": d.get("firstPlaylist")}}
        return None

    def items(self, rows, facts):
        pls, vids, ch = [], [], {}
        for r in rows:
            f = facts.get(r.idx, {})
            if "pl" in f:
                pls.append((r, f["pl"]))
            if "vj" in f:
                vids.append((r, f["vj"]))
            if "ch" in f:
                ch = f["ch"]
        uploads = {ch.get("uploads"), ch.get("first")} - {None}
        files = {}
        for r in rows:
            m = re.match(r"videos/([^/]+)/video\.(webm|mp4|mp3|ogg|ogv)$", r.url)
            if m:
                files[m.group(1)] = r
        dur, home = {}, {}
        for r, pl in sorted(pls, key=lambda p: p[1].get("title") or ""):
            slug = r.url[len("playlists/"):-len(".json")]
            for v in pl["videos"]:
                if v.get("dur"):
                    dur[v["id"]] = iso_secs(v["dur"])
                if slug not in uploads and v["id"] not in home:
                    home[v["id"]] = pl.get("title") or slug
        out, skipped, used = [], collections.Counter(), set()
        for r, v in vids:
            fr = files.get(v["id"])
            f = facts.get(fr.idx, {}) if fr else {}
            it = {"t": html.unescape(v.get("title") or v["slug"]), "k": "video", "path": (home.get(v["id"], "Not in a playlist"),),
                  "idx": r.idx, "s": f.get("s")}
            secs = dur.get(v["id"]) or f.get("secs")
            if secs:
                it["min"] = round(secs / 60, 1)
            if year_month(v.get("date")):
                it["d"] = (v.get("date") or "")[:10]
            if v.get("desc"):
                it["x"] = v["desc"]
            out.append(it)
            used.add(r.idx)
            if fr:
                used.add(fr.idx)
        for r in rows:
            if r.idx not in used:
                skipped["pages, playlists, subtitles and player files" if not r.url.startswith("videos/") else "video pages and subtitles"] += 1
        return out, skipped


# --- TED (ted2zim) --------------------------------------------------------------------------

TED_VIDEO = re.compile(r'src="videos/([^/"]+)/video\.(webm|mp4)"')
TED_SPEAKER = re.compile(r'<p id="speaker">(.*?)</p>', re.S)
TED_DATE = re.compile(r'<div id="date">(.*?)</div>', re.S)
TED_DESC = re.compile(r'<p class="description lang-default">(.*?)</p>', re.S)


class Ted(Family):
    name = "ted"

    def wanted(self, rows):
        return {r.idx for r in rows if r.front}

    def extract(self, row, data):
        h = data.decode("utf-8", "replace")
        x = {}
        m = TED_VIDEO.search(h)
        if m:
            x["vid_path"] = f"videos/{m.group(1)}/video.{m.group(2)}"
        for key, rx in (("by", TED_SPEAKER), ("date", TED_DATE), ("desc", TED_DESC)):
            m = rx.search(h)
            if m:
                x[key] = re.sub(r"\s+", " ", html.unescape(re.sub(r"<[^>]+>", "", m.group(1)))).strip()
        return x

    def items(self, rows, facts):
        by_url = {r.url: r for r in rows}
        out, skipped, used = [], collections.Counter(), set()
        for r in rows:
            if not r.front:
                continue
            f = facts.get(r.idx, {})
            vr = by_url.get(f.get("vid_path", ""))
            vf = facts.get(vr.idx, {}) if vr else {}
            it = {"t": html.unescape(r.title), "k": "talk", "path": (), "idx": r.idx, "s": vf.get("s")}
            if vf.get("secs"):
                it["min"] = round(vf["secs"] / 60, 1)
            if f.get("by"):
                it["by"] = f["by"]
            if iso_date(f.get("date")):
                it["d"] = iso_date(f["date"])
            if f.get("desc"):
                it["x"] = first_line(f["desc"])
            out.append(it)
            used.add(r.idx)
            if vr:
                used.add(vr.idx)
        for r in rows:
            if r.idx not in used:
                skipped["subtitles" if r.mime.startswith("text/vtt") else "player and page files"] += 1
        return out, skipped


# --- Project Gutenberg ------------------------------------------------------------------------

LCC = {
    "A": "General works", "B": "Philosophy, psychology, religion", "C": "Auxiliary sciences of history", "D": "World history",
    "E": "History of the Americas", "F": "History of the Americas (local)", "G": "Geography, anthropology, recreation",
    "H": "Social sciences", "J": "Political science", "K": "Law", "L": "Education", "M": "Music",
    "N": "Fine arts", "P": "Language and literature", "Q": "Science", "R": "Medicine", "S": "Agriculture",
    "T": "Technology", "U": "Military science", "V": "Naval science", "Z": "Bibliography and library science",
}
LCC_SUB = {
    "AC": "Collections, series", "AE": "Encyclopedias", "AG": "Dictionaries and reference works", "AI": "Indexes",
    "AM": "Museums, collectors", "AN": "Newspapers", "AP": "Periodicals", "AS": "Academies and learned societies",
    "AY": "Yearbooks, almanacs", "AZ": "History of scholarship",
    "BC": "Logic", "BD": "Speculative philosophy", "BF": "Psychology", "BH": "Aesthetics", "BJ": "Ethics",
    "BL": "Religions, mythology", "BM": "Judaism", "BP": "Islam", "BQ": "Buddhism", "BR": "Christianity",
    "BS": "The Bible", "BT": "Doctrinal theology", "BV": "Practical theology", "BX": "Christian denominations",
    "CB": "History of civilization", "CC": "Archaeology", "CD": "Diplomatics, archives", "CE": "Technical chronology",
    "CJ": "Numismatics", "CN": "Inscriptions", "CR": "Heraldry", "CS": "Genealogy", "CT": "Biography",
    "DA": "Great Britain", "DB": "Austria", "DC": "France", "DD": "Germany", "DE": "Greco-Roman world", "DF": "Greece",
    "DG": "Italy", "DH": "Low Countries", "DJ": "Netherlands", "DK": "Russia, Soviet Union, Poland", "DL": "Northern Europe, Scandinavia",
    "DP": "Spain, Portugal", "DQ": "Switzerland", "DR": "Balkan Peninsula", "DS": "Asia", "DT": "Africa", "DU": "Oceania",
    "DX": "Romanies",
    "GA": "Mathematical geography, cartography", "GB": "Physical geography", "GC": "Oceanography", "GE": "Environmental sciences",
    "GF": "Human ecology", "GN": "Anthropology", "GR": "Folklore", "GT": "Manners and customs", "GV": "Recreation, leisure, sport",
    "HA": "Statistics", "HB": "Economic theory", "HC": "Economic history and conditions", "HD": "Industries, land use, labour",
    "HE": "Transportation and communications", "HF": "Commerce", "HG": "Finance", "HJ": "Public finance",
    "HM": "Sociology", "HN": "Social history and conditions", "HQ": "The family, marriage, women", "HS": "Societies, clubs",
    "HT": "Communities, classes, races", "HV": "Social welfare, criminology", "HX": "Socialism, communism, anarchism",
    "JA": "Political science (general)", "JC": "Political theory", "JF": "Political institutions", "JK": "Political institutions (United States)",
    "JN": "Political institutions (Europe)", "JQ": "Political institutions (Asia, Africa, Oceania)", "JS": "Local government",
    "JV": "Colonies, emigration", "JX": "International law (old)", "JZ": "International relations",
    "KD": "Law of the United Kingdom", "KF": "Law of the United States",
    "LA": "History of education", "LB": "Theory and practice of education", "LC": "Special aspects of education",
    "LD": "Individual institutions (United States)", "LT": "Textbooks",
    "ML": "Literature on music", "MT": "Instruction and study of music",
    "NA": "Architecture", "NB": "Sculpture", "NC": "Drawing, design, illustration", "ND": "Painting", "NE": "Print media",
    "NK": "Decorative arts", "NX": "Arts in general",
    "PA": "Greek and Latin language and literature", "PB": "Modern languages, Celtic", "PC": "Romance languages",
    "PD": "Germanic and Scandinavian languages", "PE": "English language", "PF": "West Germanic languages",
    "PG": "Slavic, Baltic and Albanian literature", "PH": "Uralic and Basque languages", "PJ": "Oriental languages and literatures",
    "PK": "Indo-Iranian languages", "PL": "Languages of East Asia, Africa, Oceania", "PM": "Indigenous American and artificial languages",
    "PN": "Literature (general)", "PQ": "French, Italian, Spanish and Portuguese literature", "PR": "English literature",
    "PS": "American literature", "PT": "German, Dutch and Scandinavian literature", "PZ": "Fiction and juvenile belles lettres",
    "QA": "Mathematics", "QB": "Astronomy", "QC": "Physics", "QD": "Chemistry", "QE": "Geology", "QH": "Natural history, biology",
    "QK": "Botany", "QL": "Zoology", "QM": "Human anatomy", "QP": "Physiology", "QR": "Microbiology",
    "RA": "Public aspects of medicine", "RB": "Pathology", "RC": "Internal medicine", "RD": "Surgery", "RE": "Ophthalmology",
    "RF": "Otorhinolaryngology", "RG": "Gynaecology and obstetrics", "RJ": "Paediatrics", "RK": "Dentistry", "RL": "Dermatology",
    "RM": "Therapeutics, pharmacology", "RS": "Pharmacy and materia medica", "RT": "Nursing", "RV": "Botanic and folk medicine",
    "RX": "Homeopathy", "RZ": "Other systems of medicine",
    "SB": "Plant culture", "SD": "Forestry", "SF": "Animal culture", "SH": "Aquaculture, fisheries", "SK": "Hunting",
    "TA": "Engineering (general), civil engineering", "TC": "Hydraulic engineering", "TD": "Environmental technology, sanitary engineering",
    "TE": "Highway engineering", "TF": "Railroad engineering", "TG": "Bridge engineering", "TH": "Building construction",
    "TJ": "Mechanical engineering and machinery", "TK": "Electrical engineering, electronics", "TL": "Motor vehicles, aeronautics",
    "TN": "Mining engineering, metallurgy", "TP": "Chemical technology", "TR": "Photography", "TS": "Manufactures",
    "TT": "Handicrafts, arts and crafts", "TX": "Home economics, cooking",
    "UA": "Armies", "UB": "Military administration", "UC": "Military maintenance and transportation", "UD": "Infantry",
    "UE": "Cavalry", "UF": "Artillery", "UG": "Military engineering", "UH": "Other military services",
    "VA": "Navies", "VB": "Naval administration", "VK": "Navigation, merchant marine", "VM": "Naval architecture, shipbuilding",
}
PG_CATALOG = "https://www.gutenberg.org/cache/epub/feeds/pg_catalog.csv.gz"


def gutenberg_catalog():
    path = CACHE / "pg_catalog.csv.gz"
    if not path.exists():
        req = urllib.request.Request(PG_CATALOG, headers={"User-Agent": UA})
        with urllib.request.urlopen(req, timeout=300) as r:
            data = r.read()
        CACHE.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)
    out = {}
    with gzip.open(path, "rt", encoding="utf-8") as f:
        for rec in csv.DictReader(f):
            out[rec["Text#"]] = rec
    return out


def lcc_path(locc):
    codes = [c.strip() for c in (locc or "").split(";") if c.strip()]
    if not codes:
        return ("Not classified",)
    c = sorted(codes, key=lambda c: (len(c) < 2, c))[0]
    top = c[0]
    cls = f"{top} · {LCC.get(top, 'Other')}"
    if len(c) >= 2 and c[:2] in LCC_SUB:
        return (cls, f"{c[:2]} · {LCC_SUB[c[:2]]}")
    if len(c) >= 2 and c[1].isalpha():
        return (cls, c[:2])
    return (cls, f"{top} · general")


def author_key(authors):
    a = (authors or "").split(";")[0].strip()
    a = re.sub(r",?\s*\d{3,4}\??-.*$", "", a).strip().rstrip(",")
    return a


class Gutenberg(Family):
    name = "gutenberg"
    BOOK = re.compile(r"^(.*)\.(\d+)$")
    EPUB = re.compile(r"^(.*)\.(\d+)\.epub$")
    COVER = re.compile(r"^(.*)_cover\.(\d+)$")

    def needs_duration(self, row):
        return False

    def items(self, rows, facts):
        cat = gutenberg_catalog()
        books = collections.defaultdict(dict)
        other = []
        for r in rows:
            m = self.EPUB.match(r.url) if r.mime == "application/epub+zip" else None
            if m:
                books[m.group(2)]["epub"] = r
                continue
            m = self.COVER.match(r.url) if r.front else None
            if m:
                books[m.group(2)]["cover"] = r
                continue
            m = self.BOOK.match(r.url) if r.mime.startswith("text/html") else None
            if m and not r.front and m.group(2) in cat:
                books[m.group(2)]["html"] = r
                continue
            other.append(r)
        out, skipped = [], collections.Counter()
        for bid, parts in books.items():
            rec = cat.get(bid, {})
            title = rec.get("Title") or html.unescape((parts.get("cover") or parts.get("html") or parts.get("epub")).title)
            title = re.sub(r"\s+", " ", title).strip()
            sizes = [facts.get(p.idx, {}).get("s") for k, p in parts.items() if k != "cover"]
            it = {"t": title, "k": "book", "idx": (parts.get("html") or parts.get("epub") or parts["cover"]).idx,
                  "s": sum(sizes) if sizes and all(s is not None for s in sizes) else None}
            auth = author_key(rec.get("Authors"))
            sec = lcc_path(rec.get("LoCC"))
            it["path"] = sec + ((auth[:1].upper() if auth[:1].isalpha() else "Other authors"),) if auth else sec + ("Anonymous and various",)
            if auth:
                it["by"] = auth
            if rec.get("Issued"):
                it["d"] = rec["Issued"]
            if rec.get("Subjects"):
                it["x"] = first_line(rec["Subjects"].replace(";", " ·"), 160)
            fm = [k for k in ("html", "epub") if k in parts]
            if fm:
                it["fmt"] = " + ".join({"html": "HTML", "epub": "EPUB"}[k] for k in fm)
            it["gid"] = int(bid)
            out.append(it)
        for r in other:
            if r.front and self.BOOK.match(r.url):
                skipped["author pages"] += 1
            elif r.mime == "audio/midi":
                skipped["music files inside books"] += 1
            else:
                skipped[self.skip_reason(r)] += 1
        return out, skipped


# --- LibreTexts (mindtouch2zim) ---------------------------------------------------------------

class LibreTexts(Family):
    name = "libretexts"

    def wanted(self, rows):
        return {r.idx for r in rows if r.url == "content/shared.json"}

    def extract(self, row, data):
        d = json.loads(data)
        return {"pages": [[p["id"], p["title"], p["path"]] for p in d.get("pages", [])]}

    def items(self, rows, facts):
        pages = next((f["pages"] for f in facts.values() if isinstance(f.get("pages"), list)), [])
        by_path = {p[2]: p for p in pages}
        children = collections.Counter(p[2].rsplit("/", 1)[0] for p in pages if "/" in p[2])
        rows_by = {r.url: r for r in rows}
        out, used = [], set()
        for pid, title, path in pages:
            if children.get(path) or path in ("", "Bookshelves", "Courses"):
                continue
            parts = path.split("/")
            section = []
            for i in range(1, len(parts)):
                anc = by_path.get("/".join(parts[:i]))
                if anc and anc[2] not in ("", "Bookshelves", "Courses"):
                    section.append(anc[1])
            if parts[0] == "Courses":
                section = ["Courses"] + section
            hr, cr = rows_by.get(f"index/page_{pid}"), rows_by.get(f"content/page_content_{pid}.json")
            if not (hr or cr):
                continue  # in the online book tree but not in this archive
            sz = [facts.get(r.idx, {}).get("s") for r in (hr, cr) if r]
            it = {"t": html.unescape(title), "k": "page", "path": tuple(section[:4]), "idx": (hr or cr).idx if (hr or cr) else -1,
                  "s": sum(sz) if sz and all(s is not None for s in sz) else None}
            out.append(it)
            used.update(r.idx for r in (hr, cr) if r)
        skipped = collections.Counter()
        for r in rows:
            if r.idx in used:
                continue
            if r.url.startswith(("index/page_", "content/page_content_")):
                skipped["section index pages"] += 1
            elif r.mime == "application/pdf" or r.mime.startswith("video/"):
                out.append(self.make(r, facts, self.kind_of(r), ("Attached files",)))
            else:
                skipped[self.skip_reason(r)] += 1
        return out, skipped


# --- web archives (warc2zim): NHS, irp.fas.org, CD3WD, recipe sites --------------------------

class Web(Family):
    name = "web"

    def __init__(self, item, meta):
        super().__init__(item, meta)
        book = item["book"]
        self.item_kind = "recipe" if "cooking" in book else ("medicine" if book.startswith("nhs") else "page")
        self.depth = 2 if "cd3wd" in book else 0

    def path_of(self, r):
        parts = r.url.split("/")[1:-1]
        if re.search(r"/tags?/", r.url):
            return ("Tag pages",)
        if "cd3wd" in self.item["book"]:
            # Collections and documents: CD3WD/AGRIC/R0041E/... -> ("Agric", "R0041E")
            parts = [p for p in parts if p and p.upper() != "CD3WD"] or parts
            return tuple(p for p in parts[:2])
        if self.item["book"].startswith("nhs"):
            parts = [p for p in parts if p != "medicines"]
            return (parts[0].replace("-", " ").capitalize(),) if parts else ()
        return ()

    def kind_of(self, r):
        k = super().kind_of(r)
        if k and r.mime.startswith("text/html") and re.search(r"/tags?/", r.url):
            return "tag page"
        if k and r.mime.startswith("text/html") and self.item["book"].startswith("nhs") and r.url.count("/") <= 3:
            return "medicine"
        if k and r.mime.startswith("text/html") and self.item["book"].startswith("nhs"):
            return "page"
        return k

    def title_of(self, r, facts):
        t = super().title_of(r, facts)
        return re.sub(r"\s*[-|]\s*(NHS|Based Cooking|FOSS cooking)\s*$", "", t)


# --- small sites with their own layout -----------------------------------------------------------

class PathSections(Family):
    """Items are front pages; the first path segments are the sections (DevDocs, freeCodeCamp)."""
    name = "paths"

    def __init__(self, item, meta, strip=0, depth=1, kind="page"):
        super().__init__(item, meta)
        self.strip, self.depth, self.item_kind = strip, depth, kind

    def path_of(self, r):
        parts = r.url.split("/")[self.strip:-1]
        return tuple(p.replace("-", " ").replace("_", " ").capitalize() for p in parts[: self.depth])


class Phet(Family):
    name = "phet"
    item_kind = "simulation"

    def kind_of(self, r):
        if r.front and r.url.endswith(".html"):
            return "simulation"
        return super().kind_of(r) if not r.front else None


# --- iFixit ----------------------------------------------------------------------------------

IFX_LINK = re.compile(r'href="\.\./(Device|Guide|Teardown)/([^"#?]+)"')
IFX_STEPS = re.compile(r'>\s*Steps\s*</p>\s*<p class="item-value[^"]*">\s*(\d+)')
IFX_TIME = re.compile(r'Time Required\s*</p>\s*<p class="item-value[^"]*">\s*([^<]+?)\s*<')
IFX_DIFF = re.compile(r'details-item difficulty row difficulty-(\d)')
IFX_DIFF_NAMES = {"1": "Very easy", "2": "Easy", "3": "Moderate", "4": "Difficult", "5": "Very difficult"}


class IFixit(Family):
    name = "ifixit"

    def wanted(self, rows):
        return {r.idx for r in rows if r.front and r.url.startswith(("Device/", "Guide/", "Teardown/")) or r.url == "home/home"}

    def extract(self, row, data):
        h = data.decode("utf-8", "replace")
        if row.url.startswith(("Device/", "home/")):
            links = []
            for kind, target in IFX_LINK.findall(h):
                t = urllib.parse.unquote(target)
                if (kind, t) not in links:
                    links.append((kind, t))
            return {"links": [f"{k}/{t}" for k, t in links if f"{k}/{t}" != row.url]}
        x = {"words": media.words(media.html_text(h))}
        m = IFX_STEPS.search(h)
        if m:
            x["steps"] = int(m.group(1))
        m = IFX_TIME.search(h)
        if m:
            x["time"] = html.unescape(m.group(1))
        m = IFX_DIFF.search(h)
        if m:
            x["diff"] = IFX_DIFF_NAMES.get(m.group(1))
        return x

    def items(self, rows, facts):
        by_url = {r.url: r for r in rows}
        title = {r.url: html.unescape(r.title).replace("****", "").strip() for r in rows}
        children = {}
        for r in rows:
            f = facts.get(r.idx, {})
            if "links" in f:
                children[r.url] = [l for l in f["links"] if urllib.parse.unquote(l) in by_url or l in by_url]
        # Device tree: breadth first from the home page's categories, so every
        # device hangs under the shortest chain of categories that lists it.
        parent = {}
        tops = [l for l in children.get("home/home", []) if l.startswith("Device/")]
        queue = list(dict.fromkeys(tops))
        for t in queue:
            parent.setdefault(t, None)
        i = 0
        while i < len(queue):
            d = queue[i]
            i += 1
            for c in children.get(d, []):
                if c.startswith("Device/") and c not in parent:
                    parent[c] = d
                    queue.append(c)
        owner = {}
        for d in queue:  # guides belong to the deepest device that lists them
            for c in children.get(d, []):
                if c.startswith(("Guide/", "Teardown/")):
                    owner[c] = d

        def chain(d):
            out = []
            while d:
                out.append(title.get(d) or nice_name(d))
                d = parent.get(d)
            return tuple(reversed(out))

        out, skipped = [], collections.Counter()
        for r in rows:
            if not r.front:
                skipped["user pages" if r.url.startswith("User/") else self.skip_reason(r)] += 1
                continue
            f = facts.get(r.idx, {})
            if r.url.startswith(("Guide/", "Teardown/")):
                d = owner.get(r.url) or owner.get(urllib.parse.quote(r.url, safe="/+"))
                path = chain(d)[:5] if d else ("Guides not listed under a device",)
                it = self.make(r, facts, "teardown" if r.url.startswith("Teardown/") else "repair guide", path)
                it.update({k: v for k, v in {"steps": f.get("steps"), "time": f.get("time"), "diff": f.get("diff")}.items() if v})
                it["t"] = title[r.url]
                out.append(it)
            elif r.url.startswith("Device/"):
                path = chain(parent.get(r.url))[:5] if r.url in parent else ("Devices not in the category tree",)
                it = self.make(r, facts, "device page", path)
                it["t"] = title[r.url]
                out.append(it)
            else:
                out.append(self.make(r, facts, "page", ("Help and information pages",)))
        return out, skipped


# --- Khan Academy (kolibri2zim) -----------------------------------------------------------------

KHAN_CRUMBS = re.compile(r'aria-label="breadcrumb"(.*?)</(?:nav|ol)>', re.S)
KHAN_LI = re.compile(r"<li[^>]*>(.*?)</li>", re.S)
KHAN_VIDEO = re.compile(r'src="\.?/?([0-9a-f]{32}\.(?:webm|mp4))"')
KHAN_DESC = re.compile(r'<p[^>]*class="[^"]*(?:description|lead)[^"]*"[^>]*>(.*?)</p>', re.S)


def _text(h):
    return re.sub(r"\s+", " ", html.unescape(re.sub(r"<[^>]+>", " ", h))).strip()


class Khan(Family):
    name = "khan"

    def wanted(self, rows):
        return {r.idx for r in rows if r.front}

    def needs_duration(self, row):
        return row.url.endswith((".webm", ".mp4"))

    def extract(self, row, data):
        h = data.decode("utf-8", "replace")
        x = {}
        m = KHAN_CRUMBS.search(h)
        if m:
            x["crumbs"] = [_text(li) for li in KHAN_LI.findall(m.group(1))]
        m = KHAN_VIDEO.search(h)
        if m:
            x["video"] = m.group(1)
        m = KHAN_DESC.search(h)
        if m:
            x["desc"] = _text(m.group(1))[:300]
        return x

    def items(self, rows, facts):
        by_url = {r.url: r for r in rows}
        folders = collections.defaultdict(list)
        for r in rows:
            if "/" in r.url and r.mime == "application/json":
                folders[r.url.split("/", 1)[0]].append(r)
        out, skipped, used = [], collections.Counter(), set()
        for r in rows:
            if not r.front:
                continue
            f = facts.get(r.idx, {})
            crumbs = [c for c in f.get("crumbs", []) if c]
            path = tuple(crumbs[1:-1])[:5] if len(crumbs) > 2 else ()
            vr = by_url.get(f.get("video", ""))
            ex = folders.get(r.url, [])
            if vr:
                vf = facts.get(vr.idx, {})
                it = {"t": html.unescape(r.title), "k": "video", "path": path, "idx": r.idx, "s": vf.get("s")}
                if vf.get("secs"):
                    it["min"] = round(vf["secs"] / 60, 1)
                used.add(vr.idx)
            elif ex:
                qs = [e for e in ex if "/images/" not in e.url]
                sz = [facts.get(e.idx, {}).get("s") for e in ex]
                it = {"t": html.unescape(r.title), "k": "exercise", "path": path, "idx": r.idx,
                      "s": sum(sz) if all(v is not None for v in sz) else None, "qs": len(qs)}
                used.update(e.idx for e in ex)
            else:
                skipped["topic and course pages"] += 1
                used.add(r.idx)
                continue
            if f.get("desc"):
                it["x"] = first_line(f["desc"])
            out.append(it)
            used.add(r.idx)
        for r in rows:
            if r.idx in used or r.front:
                continue
            if r.url.endswith(".vtt"):
                skipped["subtitles"] += 1
            elif r.url.endswith((".webm", ".mp4")):
                out.append(self.make(r, facts, "video", ("Videos not linked from a lesson",)))
            else:
                skipped[self.skip_reason(r)] += 1
        return out, skipped


def family(item, meta):
    b = item["book"]
    if b.startswith(("wikipedia_", "wiktionary_", "wikivoyage_", "wikiquote_", "wikibooks_", "wikiversity_", "wikisource_",
                     "vikidia_", "mdwiki_", "appropedia_", "energypedia_")):
        return MediaWiki(item, meta)
    if "stackexchange.com" in b or b.startswith(("askubuntu.com", "superuser.com", "stackoverflow.com", "serverfault.com")):
        return StackExchange(item, meta)
    if b.startswith("zimgit-"):
        return Nautilus(item, meta)
    if b.startswith("ted_"):
        return Ted(item, meta)
    if b.startswith(("crashcourse_", "canadian-prepper_", "quickguidesformedicine_")):
        return YouTube(item, meta)
    if b.startswith("gutenberg_"):
        return Gutenberg(item, meta)
    if b.startswith("libretexts.org_"):
        return LibreTexts(item, meta)
    if b.startswith(("nhs.uk_", "irp.fas.org_", "cd3wdproject.org_", "based.cooking_", "foss.cooking_")):
        return Web(item, meta)
    if b.startswith("devdocs_"):
        return PathSections(item, meta, 0, 1, "reference page")
    if b.startswith("freecodecamp_"):
        return PathSections(item, meta, 1, 2, "lesson")
    if b.startswith("phet_"):
        return Phet(item, meta)
    if b.startswith("ifixit_"):
        return IFixit(item, meta)
    if b.startswith("khanacademy_"):
        return Khan(item, meta)
    return Family(item, meta)
