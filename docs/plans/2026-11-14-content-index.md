# Plan: the Content page as a complete, nested index of every item

Date: 2026-11-14. Repo: `outpost`. Page: `/catalog` ("Content" in the nav), `site/app/catalog/page.tsx`.

## 1. What is wanted

One simple table of drop-downs. Open a subject, then a collection, then its sections, and arrive at the
individual things: each book, PDF, video, article, map region and model. Every row carries its data (size,
page count or other length, last updated) and says what it is and why it is in the library. Nothing that
counts toward the advertised total may be missing, and no drop-down may open onto thousands of loose rows.

## 2. Where things stand (measured 2026-11-14)

- The page lists **410 downloads**, not their contents: 90 Kiwix archives (ZIM), 312 map regions, a world
  overview, map fonts, 6 AI models. Gutenberg is one row of 221 GB; Wikipedia is one row of 127 GB.
- The catalog (`site/public/catalog/v1/catalog.json`, built and signed by `catalog/build.py` from
  `catalog/source.json`) holds only per-download facts: title, one-line description, publisher, licence,
  size, edition date, SHA-256. It has no "purpose" field and nothing below the download.
- Sizes in today's catalog: all 90 archives 811.3 GB; maps 307.2 GB; models 25.0 GB. The largest install
  a person can select (biggest Wikipedia, every subject at Comprehensive, every map region) is 992.3 GB,
  or 1,017 GB with all models. That is the nearest match to the "991 GB" figure; the string 991 is not
  in the source, so the number shown today is computed. The 811 GB and 992 GB differ because some
  archives are alternative editions of each other (six Wikipedia editions; Gutenberg subject shelves
  that the full Gutenberg replaces).
- **The contents can be listed without downloading the archives.** A ZIM keeps its directory (path,
  title, type of every entry) in one block at the end of the file, and download.kiwix.org answers range
  requests. Measured: Gutenberg 1,562,243 entries in the last 80 MB of 221 GB; full Wikipedia
  30,142,538 entries (articles, redirects, pictures) in the last 2.4 GB of 127 GB; the 20 MB Water
  library parsed in full with a 30-line reader. Listing all 90 archives is a few GB of transfer.
- What the directory does not give: per-item size for compressed text, page counts, word counts, video
  length, per-item dates, real titles for some documents (the Water library's PDFs are named
  `Water (1).pdf`; their titles sit in the archive's own `database.js`). Those need the archive's
  content, see section 6.
- Scale to plan for: roughly 20 to 25 million real items once redirects, pictures, scripts and
  duplicate editions are removed (Wikipedia about 7 M articles, Wiktionary several million, Stack
  Exchange a few million questions, Gutenberg about 75,000 books, thousands of videos and PDFs).

## 3. Approaches considered

1. **A static, sharded index built with the catalog (chosen).** `catalog/` gains an indexer that writes
   a tree of small JSON files; the page renders the top levels itself and fetches a node when its
   drop-down opens. Fits the product: static site, signed data, no server to run or pay for, and the
   same files can later ship to the dashboard for offline use.
2. A database or search service behind an API. Easy search, but adds a running service the site does
   not have, and data that is not signed. Rejected.
3. Everything inline in the page. Impossible at 20 M rows. Rejected.
4. Link each collection to Kiwix's online viewer. Does not show items on our page, no sizes, no
   purpose. Rejected.

## 4. The tree

Four kinds of level, always in this order. A node never shows more than **200 rows**; a larger node is
split automatically (rule in 4.3).

```
Subject            Medicine · Survival · Repair · Homestead · Education · Science · Computing ·
                   Reference · Books · Wikipedia · Maps · AI models
└ Collection       one download: "Project Gutenberg Library", "iFixit", "Water Treatment Library"
  └ Section(s)     the source's own shelves, one to three levels deep
    └ Item         one book / PDF / video / article / question / guide / region / model
```

### 4.1 What a row shows

| Level | Columns |
|---|---|
| Subject | name, what it is for, collections, items, size |
| Collection | title, **why it is included**, edition (last updated), items, size, licence, which tier brings it (Essential / Standard / Comprehensive), publisher, file name and SHA-256 behind a "details" toggle |
| Section | name, items, size |
| Item | title, kind (book, PDF, video, article, Q&A, guide, simulation), **length** (pages for PDFs and books that have them, words for articles and texts, minutes for video and audio, answers for Q&A), size, last updated, author or source, one line on what it is |

Empty cells are allowed only where the source truly has no such fact (an article has no page count);
a fact that has not been measured yet reads "not yet measured", never an estimate.

### 4.2 Sections per source

Each collection is shelved the way its publisher shelves it, so a reader recognises the shelves.

| Source | Sections | Item | Extra facts and where from |
|---|---|---|---|
| Project Gutenberg (all, and LCC shelves Q, R, S, T) | Library of Congress class → subclass → author initial | book | author, subjects, release date, language from Gutenberg's own catalog file; formats held (HTML, EPUB, PDF) and their sizes |
| Document libraries (zimgit: Medicine, Water, Knots, Food, Post-disaster) | the library's own folders/tags from its `database.js` | PDF | real title and description from `database.js`; pages, PDF date from the file |
| Wikipedia (6 editions), WikiMed, MDWiki, subject Wikipedias, Vikidia | first letter → next letters, split by 4.3 | article | words, pictures count, size |
| Wiktionary | first letter → next letters | entry | size |
| Wikibooks, Wikiversity, Wikisource | book or course (the part of the title before `/`) → chapters | chapter | words |
| Wikivoyage | continent → country → destination, from its breadcrumb | guide | words |
| Wikiquote | first letter | page | words |
| Stack Exchange sites (22) | most-used tag of the question → year asked | question | answers, score, asked and last-active dates |
| Khan Academy | subject → course → unit, from its own topic pages | video or exercise | minutes, size |
| CrashCourse, TED sets, Canadian Prepper | series or playlist | video | minutes, published date, size |
| iFixit | device category → device | guide | steps, last edited |
| LibreTexts (8) | bookshelf → book → chapter | page | words |
| Appropedia, energypedia, CD3WD | the site's top categories; CD3WD by its folder tree | page or document | pages for PDFs, words otherwise |
| NHS Medicines, Military Medicine, Surviving Residency, freeCodeCamp, DevDocs, PhET, the two cooking sites | the site's own index page order | page, manual, lesson, simulation | as available |
| Maps | continent → country → state or province (existing `group`) | region | size, zoom, bounding box, map build date; plus the world overview and fonts |
| AI models | none | model | size, memory needed, Ollama name |

Where one collection is another with something removed (Wikipedia text-only, introductions-only;
Gutenberg shelf Q inside full Gutenberg) it still appears as its own collection row with its own size
and purpose, and its item list is the same list filtered, stored once.

### 4.3 The split rule (no bucket of 10,000)

When a section would hold more than 200 items it is divided by the next natural key from 4.2; when
those run out, by title prefix: A, B, C… then Aa–Af, Ag–Am… with ranges chosen so every leaf holds
100 to 200 rows. Labels show the range and count ("Ma – Me · 184 articles"). The indexer fails the
build if any node exceeds 200 children.

### 4.4 Purpose

- **Subject and collection:** written by hand. Add `purpose` to all 10 subjects and 90 archives in
  `catalog/source.json` (one or two plain sentences: what need it answers when the network is gone,
  and why this edition). Maps and models get one each. 102 short texts; the build fails if one is missing.
- **Section:** from the source's own shelf name and description, no new writing.
- **Item:** its own description where the source has one (Gutenberg subjects, document descriptions,
  video descriptions, a question's title); the page shows the collection's purpose above the item
  table so every item is read in its context. Per-item purposes are not invented by a model.

## 5. Index files

Written by a new `catalog/index.py`, read by the page.

```
site/public/catalog/v1/index/
  root.json            subjects → collections, with counts, sizes, purposes; sha256 of each collection manifest
  root.json.sig        signed with the release key, like catalog.json
  <collection>/manifest.json   sections tree (ids, labels, counts, sizes), sha256 of each leaf
  <collection>/<node>.json     one leaf: up to 200 item rows as compact arrays
  search/<abc>.json    title words by their first three letters → [collection, node, row]
```

Node ids are derived from the path so a rebuild that changes nothing produces identical files.
Estimated 150,000 to 200,000 small files, 300 to 600 MB compressed. Task 9 measures this against the
host's limits; if it does not fit in the site deployment, the same tree is served from object storage
under the same path and the page code does not change.

## 6. Getting the facts: two passes

**Pass A, the listing (cheap, do first).** For each archive: range-read the header, the MIME list and
the directory block; keep entries that are content (drop redirects, pictures inside articles, scripts,
styles, search indexes); classify by type and path pattern per source; attach titles. Gives every item
with title, kind and section. A few GB of transfer in total; results cached under `catalog/.cache` by
archive SHA-256 so an unchanged archive is never read again.

**Pass B, the measurements (heavy, per archive).** Size, pages, words, minutes and dates need content:
- Files stored as their own blobs (PDF, EPUB, video, audio): size from the cluster offset table by
  range request; PDF page count and date by reading the PDF's trailer and page tree (ranges, or the
  whole file when small); video length from the container header.
- Compressed text (articles, Q&A, chapters): the archive is streamed once, cluster by cluster, and
  each entry's uncompressed size and word count recorded. Nothing is kept on disk except the result.
- Source catalogs fill the rest without touching the archive: Gutenberg's `pg_catalog.csv`, the Stack
  Exchange data dump's post dates and tags, a library's `database.js`.

Pass B runs smallest archive first (the 60 archives under 2 GB are about 35 GB together) and ends with
Khan Academy, Gutenberg and Wikipedia. The page is useful after pass A and fills in as pass B lands;
until then a column reads "not yet measured".

## 7. The page

- One table. Rows are native `<details>` drop-downs styled as table rows, so the keyboard, screen
  readers and find-in-page work. Subjects and collections are rendered on the server (no script
  needed to see all 410 downloads and their purposes, as today). Opening a collection fetches its
  manifest; opening a leaf fetches its 200 rows.
- A search box above the table: type a title, get matching items with their path ("Books › Project
  Gutenberg › Technology › Engineering › …"); choosing one opens the tree to that row. Collections
  and sections match instantly from `root.json`; item titles use the `search/` shards.
- Every open node is in the address (`/catalog#books/gutenberg_en_all/T/TK/…`) so a row can be linked.
- A totals row that adds up in public: number of downloads, number of items, total size of all
  files, and "largest possible install" with the rule for alternative editions spelled out, so the
  headline figure on the site and this table are the same computed number. Any hand-typed total
  elsewhere on the site is replaced by this one.
- Look: unchanged house style (black, white, greys, Archivo and Martian Mono, the existing
  `.cat-items` rules). Sizes right-aligned in mono. On phones the item table drops to title, length,
  size, with the rest under the row's own drop-down.
- The tier summary (Essential / Standard / Comprehensive sizes per subject) and the jump links stay,
  in the subject row.

## 8. Not in this plan

- The same browser inside the Outpost dashboard, offline, showing what is actually installed. The
  index files are designed to be reusable for it, but it needs a signed node release. Follow-up.
- Listing every map tile or every picture inside an article. A map region and an article are the items.
- Full-text search of contents. Search here is over titles.
- Any change to what the library contains.

## 9. Work, in order

Each task ends with its check. Tasks 1 to 6 give a complete, organised page; 7 and 8 fill in measurements.

1. **Purposes.** Add `purpose` to subjects, archives, maps and models in `catalog/source.json`; carry
   it through `build.py` into `catalog.json` (new optional field, older nodes ignore it; confirm with
   `node/internal/catalog` tests). Check: build fails on a missing purpose; signature verifies.
2. **ZIM directory reader.** `catalog/zimindex.py`: header, MIME list, directory, by range request or
   local file, with cache. Check: unit tests against the 20 MB Water library (818 entries, 7 PDFs,
   matching today's probe) and one mid-size archive compared entry-for-entry with `zimdump list`.
3. **Classifiers per source.** One small module per row of table 4.2 that turns raw entries into
   items with a section path; a default (title-prefix) classifier for anything unrecognised so no
   archive is ever left unlisted. Check: for every archive, items + dropped entries = directory
   entry count, and the dropped ones are only the listed non-content kinds.
4. **Tree builder and split rule.** Write manifests, leaves, `root.json`, signature. Check: no node
   over 200 children; every item appears exactly once per collection; rebuild is byte-identical;
   sum of collection sizes equals the catalog's.
5. **Page.** Server-rendered subject and collection rows; client component for lazy sections and
   leaves; deep links; totals row; phone layout. Check: with scripts off, all 410 downloads and
   purposes are readable; with scripts on, an end-to-end test opens a known book, PDF, video,
   article, question and map region by path.
6. **Search.** Build `search/` shards; search box. Check: 200 randomly sampled item titles, each
   found and opened by the test.
7. **Pass B, small and medium archives.** Blob sizes, PDF pages, video minutes, text sizes and word
   counts, source catalogs (Gutenberg, Stack Exchange). Check: per collection, item sizes sum to
   within 2% of the archive's size minus its dropped entries; 20 hand-checked PDFs have the right
   page count.
8. **Pass B, the three giants** (Khan Academy 180 GB, Gutenberg 221 GB, Wikipedia 127 GB), streamed,
   resumable. Check: as task 7; "not yet measured" no longer appears anywhere.
9. **Hosting and release.** Measure file count, total size and build time; choose site deployment or
   object storage; add the index build to `scripts/release.sh` so a new catalog cannot ship without
   its index; add a CI check that `root.json` matches `catalog.json`'s sequence. Then deploy and
   check the live page.

## 10. Decisions taken here (say if any is wrong)

- "Item" for encyclopedias and Q&A sites means the article or the question. So "every single item"
  includes about 7 M Wikipedia articles, reached through letter ranges and search.
- Alternative editions are shown as separate collections sharing one item list.
- Lengths are pages only where pages exist; words, minutes or answers elsewhere.
- Item purposes come from the source's own descriptions plus the collection's hand-written purpose.
- Measurements that are not done read "not yet measured" rather than a guess.

## 11. As built (2026-11-14/15), and where it differs from the plan

- **Purposes** live in `catalog/source.json` (`purposes`) and in the signed index root, not in
  `catalog.json`. The catalog the nodes read is unchanged, so no node release or catalog re-signing
  was needed for this.
- **Listing** (`catalog/zimlist.py`) read the directory of all 90 archives: 113.5 million entries,
  50.0 million kept as candidate items, 8.3 GB of range reads. Directories are now read in whole
  4 MB pieces and checked against Kiwix's SHA-1 piece hashes from the metalink.
- **Measuring** (`catalog/zimfacts.py`) gets every item's exact size from the offset table at the
  start of its cluster, without reading the content. Content is read only where a family needs it:
  Stack Exchange questions (tags, dates, answers, score, words), iFixit guides and device pages,
  TED talk pages, Khan Academy lesson pages, YouTube channel descriptors, LibreTexts' page tree,
  document-library records, and the articles of wikis up to 5 GB (words). PDFs are read whole for
  page counts; videos from their first 64 KB for their length.
- **Words are not counted** for the full Wikipedias, Wiktionary and Gutenberg: their text runs to
  tens of gigabytes. Their rows show size, not length.
- **Items are what each file really contains.** "Best of Wikipedia" holds 224,146 articles (the
  50,000 most read and the articles they link to); Stack Exchange archives include user profiles;
  each is shown, under its own shelf.
- **Editions share one tree** (Wikipedia with pictures, text only, introductions only; the same for
  Wikivoyage, Wikiquote and the subject Wikipedias). Rows carry a size per edition.
- **Search** matches the start of a title (as Kiwix's own title search does), across every
  collection, plus collection names and purposes anywhere in the text.
- **Hosting:** trees, leaves and search blocks are served by a separate static Vercel project
  (`catalog/publish_index.py`), each build in its own folder, cached forever, the previous build
  kept beside it. The site carries only the signed `root.json`, which names where its files are.
- **Security review fixes:** https-only mirrors, same file name required on redirect, cluster and
  PDF stream decompression capped, every index file's SHA-256 listed in the signed root.
