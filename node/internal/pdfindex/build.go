package pdfindex

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// cacheVersion changes when the chunking or extraction changes, so old
// cache files are rebuilt.
const cacheVersion = 1

const (
	maxPDF       = 80 << 20 // larger files are skipped
	maxPages     = 600
	maxCrawl     = 400 // HTML pages read per archive when looking for PDFs
	giveUpCrawl  = 40  // stop when this many pages hold no PDF link
	chunkTarget  = 1200
	chunkMax     = 2000
	fetchTimeout = 2 * time.Minute
)

// Archive is one ZIM archive served by kiwix-serve.
type Archive struct {
	Content string // the /content/<Content>/ path element
	Title   string
}

// Builder fetches archives from kiwix-serve and indexes their PDFs.
type Builder struct {
	Base     string // kiwix-serve, e.g. http://127.0.0.1:18080
	CacheDir string
	Client   *http.Client
	Logf     func(format string, args ...any)
	// Extract reads a PDF's pages. Nil reads them in this process, which is
	// only safe for trusted files; the node passes Subprocess.
	Extract func(ctx context.Context, body []byte) ([]string, error)
}

type cacheFile struct {
	Version int     `json:"v"`
	Chunks  []Chunk `json:"c"`
}

// Build returns an index of every archive's PDFs, reading cached text where
// it has it. progress, if set, is told how many archives are done.
func (b *Builder) Build(ctx context.Context, archives []Archive, progress func(done, total int)) (*Index, error) {
	var all []Chunk
	for i, a := range archives {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		chunks, err := b.archive(ctx, a)
		if err != nil {
			b.logf("pdf index: %s: %v", a.Content, err)
		}
		all = append(all, chunks...)
		if progress != nil {
			progress(i+1, len(archives))
		}
	}
	return New(all), nil
}

func (b *Builder) logf(format string, args ...any) {
	if b.Logf != nil {
		b.Logf(format, args...)
	}
}

var safeName = regexp.MustCompile(`[^A-Za-z0-9._-]`)

func (b *Builder) archive(ctx context.Context, a Archive) ([]Chunk, error) {
	cache := filepath.Join(b.CacheDir, safeName.ReplaceAllString(a.Content, "_")+".pdftext.json.gz")
	if chunks, ok := readCache(cache); ok {
		return chunks, nil
	}
	docs, err := b.findPDFs(ctx, a)
	if err != nil {
		return nil, err
	}
	var chunks []Chunk
	for _, d := range docs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		cs, err := b.extract(ctx, a, d)
		if err != nil {
			b.logf("pdf index: %s: %v", d.path, err)
			continue
		}
		chunks = append(chunks, cs...)
	}
	if b.CacheDir != "" {
		if err := writeCache(cache, chunks); err != nil {
			b.logf("pdf index: cache %s: %v", cache, err)
		}
	}
	return chunks, nil
}

func readCache(p string) ([]Chunk, bool) {
	f, err := os.Open(p)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return nil, false
	}
	var c cacheFile
	if err := json.NewDecoder(io.LimitReader(z, 1<<30)).Decode(&c); err != nil || c.Version != cacheVersion {
		return nil, false
	}
	return c.Chunks, true
}

func writeCache(p string, chunks []Chunk) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	z := gzip.NewWriter(f)
	err = json.NewEncoder(z).Encode(cacheFile{Version: cacheVersion, Chunks: nonNil(chunks)})
	if cerr := z.Close(); err == nil {
		err = cerr
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, p)
}

func nonNil(c []Chunk) []Chunk {
	if c == nil {
		return []Chunk{}
	}
	return c
}

type doc struct {
	title string
	path  string // escaped, /content/<archive>/...
}

// zimgit ("nautilus") archives list their documents in database.js.
var (
	dbTitleRE = regexp.MustCompile(`'ti':\s*'((?:[^'\\]|\\.)*)'`)
	dbFilesRE = regexp.MustCompile(`'fp':\s*\[([^\]]*)\]`)
	dbFileRE  = regexp.MustCompile(`'((?:[^'\\]|\\.)*)'`)
	hrefRE    = regexp.MustCompile(`(?is)<a\b[^>]*?\bhref\s*=\s*["']([^"'#]+)["'][^>]*>(.*?)</a>`)
	tagsRE    = regexp.MustCompile(`<[^>]*>`)
)

func unescapeJS(s string) string {
	return strings.NewReplacer(`\'`, `'`, `\\`, `\`, `\"`, `"`).Replace(s)
}

func (b *Builder) findPDFs(ctx context.Context, a Archive) ([]doc, error) {
	root := "/content/" + a.Content + "/"
	if body, ct, err := b.get(ctx, root+"database.js", 16<<20); err == nil && strings.Contains(ct, "javascript") {
		var docs []doc
		for _, line := range strings.Split(string(body), "\n") {
			fm := dbFilesRE.FindStringSubmatch(line)
			if fm == nil {
				continue
			}
			title := ""
			if tm := dbTitleRE.FindStringSubmatch(line); tm != nil {
				title = unescapeJS(tm[1])
			}
			for _, f := range dbFileRE.FindAllStringSubmatch(fm[1], -1) {
				name := unescapeJS(f[1])
				if strings.HasSuffix(strings.ToLower(name), ".pdf") && !strings.Contains(name, "..") {
					docs = append(docs, doc{title: title, path: root + "files/" + url.PathEscape(name)})
				}
			}
		}
		return docs, nil
	}
	return b.crawl(ctx, root)
}

// crawl walks an archive's HTML pages from its main page and collects the
// PDFs they link to. It gives up early on archives that have none.
func (b *Builder) crawl(ctx context.Context, root string) ([]doc, error) {
	start, err := b.mainPage(ctx, root)
	if err != nil {
		return nil, err
	}
	queue := []string{start}
	seen := map[string]bool{start: true}
	found := map[string]bool{}
	var docs []doc
	sinceFind := 0
	for n := 0; len(queue) > 0 && n < maxCrawl && sinceFind < giveUpCrawl; n++ {
		page := queue[0]
		queue = queue[1:]
		body, ct, err := b.get(ctx, page, 4<<20)
		if err != nil || !strings.Contains(ct, "html") {
			continue
		}
		sinceFind++
		for _, m := range hrefRE.FindAllStringSubmatch(string(body), -1) {
			u, err := url.Parse(strings.TrimSpace(m[1]))
			if err != nil || u.Scheme != "" || u.Host != "" {
				continue
			}
			p := path.Join(path.Dir(page), u.Path)
			if strings.HasSuffix(u.Path, "/") {
				p += "/"
			}
			if !strings.HasPrefix(p, root) {
				continue
			}
			esc := (&url.URL{Path: p}).EscapedPath()
			if strings.HasSuffix(strings.ToLower(p), ".pdf") {
				if !found[esc] {
					found[esc] = true
					title := strings.Join(strings.Fields(tagsRE.ReplaceAllString(m[2], " ")), " ")
					if len(title) < 4 {
						title = strings.TrimSuffix(path.Base(p), path.Ext(p))
					}
					docs = append(docs, doc{title: title, path: esc})
					sinceFind = 0
				}
				continue
			}
			ext := strings.ToLower(path.Ext(p))
			if (ext == "" || ext == ".html" || ext == ".htm") && !seen[esc] {
				seen[esc] = true
				queue = append(queue, esc)
			}
		}
	}
	return docs, nil
}

// mainPage follows the archive root's redirect to its main page.
func (b *Builder) mainPage(ctx context.Context, root string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.Base+root, nil)
	if err != nil {
		return "", err
	}
	c := *b.client()
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	if loc == "" {
		return root, nil
	}
	u, err := url.Parse(loc)
	if err != nil {
		return "", err
	}
	p := u.EscapedPath()
	if !strings.HasPrefix(p, "/") {
		p = path.Join(root, p)
	}
	if !strings.HasPrefix(p, root) {
		return "", errors.New("main page outside the archive")
	}
	return p, nil
}

func (b *Builder) client() *http.Client {
	if b.Client != nil {
		return b.Client
	}
	return &http.Client{Timeout: fetchTimeout}
}

func (b *Builder) get(ctx context.Context, p string, max int64) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.Base+p, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := b.client().Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("kiwix answered %s", resp.Status)
	}
	if resp.ContentLength > max {
		return nil, "", errors.New("too large")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(body)) > max {
		return nil, "", errors.New("too large")
	}
	return body, resp.Header.Get("Content-Type"), nil
}

// extract reads one PDF's text and cuts it into chunks.
func (b *Builder) extract(ctx context.Context, a Archive, d doc) ([]Chunk, error) {
	body, ct, err := b.get(ctx, d.path, maxPDF)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(ct, "pdf") {
		return nil, fmt.Errorf("not a PDF (%s)", ct)
	}
	var pages []string
	if b.Extract != nil {
		pages, err = b.Extract(ctx, body)
	} else {
		pages, err = ExtractPages(body)
	}
	if err != nil {
		return nil, err
	}
	title := d.title
	if title == "" {
		title = strings.TrimSuffix(path.Base(d.path), ".pdf")
	}
	return chunkPages(a.Title, title, d.path, pages), nil
}

// chunkPages joins short pages and splits long ones into passages.
func chunkPages(book, title, docPath string, pages []string) []Chunk {
	var chunks []Chunk
	var cur strings.Builder
	curPage := 0
	flush := func() {
		if t := strings.TrimSpace(cur.String()); len(t) >= 80 {
			chunks = append(chunks, Chunk{Book: book, Doc: title, Path: docPath, Page: curPage, Text: t})
		}
		cur.Reset()
		curPage = 0
	}
	for idx, txt := range pages {
		i := idx + 1
		if txt == "" {
			continue
		}
		for len(txt) > chunkMax {
			cut := strings.LastIndex(txt[:chunkMax], ". ")
			if cut < chunkTarget/2 {
				cut = strings.LastIndex(txt[:chunkMax], " ")
			}
			if cut <= 0 {
				cut = chunkMax
			}
			flush()
			curPage = i
			cur.WriteString(txt[:cut+1])
			flush()
			txt = strings.TrimSpace(txt[cut+1:])
		}
		if curPage == 0 {
			curPage = i
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(txt)
		if cur.Len() >= chunkTarget {
			flush()
		}
	}
	flush()
	return chunks
}
