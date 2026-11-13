// Package kiwix runs kiwix-serve for the installed ZIM archives, keeps its
// library file in step with what is installed, and queries its full-text
// search for the unified search page and the assistant.
package kiwix

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Dylan-Burns/outpost/node/internal/fsutil"
)

// Manager supervises one kiwix-serve process.
type Manager struct {
	ServeBin  string // path to kiwix-serve
	ManageBin string // path to kiwix-manage
	ZimDir    string
	Library   string // library XML path
	Port      int    // loopback port kiwix-serve listens on
	Logger    *log.Logger

	mu      sync.Mutex
	running bool
	books   int
	client  *http.Client
}

// Base returns the loopback URL of kiwix-serve.
func (m *Manager) Base() string { return "http://127.0.0.1:" + strconv.Itoa(m.Port) }

// Available reports whether the kiwix binaries are present.
func (m *Manager) Available() bool {
	for _, p := range []string{m.ServeBin, m.ManageBin} {
		if st, err := os.Stat(p); err != nil || st.IsDir() {
			return false
		}
	}
	return true
}

// Running reports whether kiwix-serve is up.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// Books is the number of archives in the library file.
func (m *Manager) Books() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.books
}

// Rebuild writes a fresh library file listing exactly the given ZIM files
// (base names inside ZimDir). kiwix-serve watches the file and reloads it.
// Files kiwix-manage cannot read are reported but do not stop the others.
func (m *Manager) Rebuild(ctx context.Context, files []string) error {
	if !m.Available() {
		return errors.New("kiwix-tools are not installed")
	}
	dir := filepath.Dir(m.Library)
	tmp := filepath.Join(dir, ".library.next.xml")
	os.Remove(tmp)
	if err := os.WriteFile(tmp, []byte("<library version=\"20110515\"></library>\n"), 0o644); err != nil {
		return err
	}
	var failed []string
	count := 0
	for _, f := range files {
		p, err := fsutil.Within(m.ZimDir, f)
		if err != nil {
			failed = append(failed, f)
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		out, err := exec.CommandContext(cctx, m.ManageBin, tmp, "add", p).CombinedOutput()
		cancel()
		if err != nil {
			m.logf("kiwix-manage could not add %s: %v %s", f, err, strings.TrimSpace(string(out)))
			failed = append(failed, f)
			continue
		}
		count++
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		return err
	}
	os.Remove(tmp)
	if err := fsutil.WriteFileAtomic(m.Library, data, 0o644); err != nil {
		return err
	}
	m.mu.Lock()
	m.books = count
	m.mu.Unlock()
	if len(failed) > 0 {
		return fmt.Errorf("could not add %s to the library", strings.Join(failed, ", "))
	}
	return nil
}

func (m *Manager) logf(format string, args ...any) {
	if m.Logger != nil {
		m.Logger.Printf("kiwix: "+format, args...)
	}
}

// Run keeps kiwix-serve running until ctx ends, restarting it with backoff.
func (m *Manager) Run(ctx context.Context) {
	if _, err := os.Stat(m.Library); errors.Is(err, os.ErrNotExist) {
		fsutil.WriteFileAtomic(m.Library, []byte("<library version=\"20110515\"></library>\n"), 0o644)
	}
	backoff := time.Second
	for ctx.Err() == nil {
		if !m.Available() {
			m.logf("kiwix-serve not found at %s; library disabled", m.ServeBin)
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
				continue
			}
		}
		start := time.Now()
		cmd := exec.CommandContext(ctx, m.ServeBin,
			"--library", m.Library, "--monitorLibrary",
			"--address", "127.0.0.1", "--port", strconv.Itoa(m.Port),
			"--nodatealiases", "--blockexternal", "--skipInvalid",
			"--threads", "8", "--searchLimit", "50",
			"--attachToProcess", strconv.Itoa(os.Getpid()))
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if m.Logger != nil {
			cmd.Stderr = m.Logger.Writer()
		}
		if err := cmd.Start(); err != nil {
			m.logf("start failed: %v", err)
		} else {
			m.mu.Lock()
			m.running = true
			m.mu.Unlock()
			err := cmd.Wait()
			m.mu.Lock()
			m.running = false
			m.mu.Unlock()
			if ctx.Err() != nil {
				return
			}
			m.logf("kiwix-serve exited: %v", err)
		}
		if time.Since(start) > time.Minute {
			backoff = time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// Result is one full-text search hit.
type Result struct {
	Title   string `json:"title"`
	Path    string `json:"path"` // e.g. /content/wikipedia_en_top_maxi_2026-01/Water
	Snippet string `json:"snippet"`
	Book    string `json:"book"`
}

type rss struct {
	Channel struct {
		Total string `xml:"http://a9.com/-/spec/opensearch/1.1/ totalResults"`
		Items []struct {
			Title       innerXML `xml:"title"`
			Link        string   `xml:"link"`
			Description innerXML `xml:"description"`
			Book        struct {
				Title string `xml:"title"`
			} `xml:"book"`
		} `xml:"item"`
	} `xml:"channel"`
}

// innerXML keeps an element's markup, so highlighted words in <b> survive.
type innerXML struct {
	Inner string `xml:",innerxml"`
}

var tagRE = regexp.MustCompile(`<[^>]*>`)

// StripTags turns a snippet with <b> marks into plain text.
func StripTags(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(tagRE.ReplaceAllString(s, ""))), " ")
}

func (m *Manager) httpClient() *http.Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.client == nil {
		m.client = &http.Client{Timeout: 20 * time.Second}
	}
	return m.client
}

// Search runs a full-text search across every archive with an index.
func (m *Manager) Search(ctx context.Context, query string, start, limit int) ([]Result, int, error) {
	q := url.Values{}
	q.Set("pattern", query)
	q.Set("format", "xml")
	q.Set("start", strconv.Itoa(start))
	q.Set("pageLength", strconv.Itoa(limit))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.Base()+"/search?"+q.Encode(), nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := m.httpClient().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		// kiwix answers 400 when no archive has a full-text index.
		return nil, 0, nil
	}
	var feed rss
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, 0, fmt.Errorf("kiwix search: %w", err)
	}
	total, _ := strconv.Atoi(strings.NewReplacer(",", "", ".", "", " ", "", " ", "").Replace(feed.Channel.Total))
	out := make([]Result, 0, len(feed.Channel.Items))
	for _, it := range feed.Channel.Items {
		if !strings.HasPrefix(it.Link, "/content/") {
			continue
		}
		out = append(out, Result{
			Title:   StripTags(it.Title.Inner),
			Path:    it.Link,
			Snippet: StripTags(it.Description.Inner),
			Book:    StripTags(it.Book.Title),
		})
	}
	return out, total, nil
}

// ArticleText fetches an article and returns its readable text, capped at
// limit bytes. path must be a /content/ path from Search.
func (m *Manager) ArticleText(ctx context.Context, path string, limit int) (string, error) {
	if !strings.HasPrefix(path, "/content/") || strings.Contains(path, "..") {
		return "", errors.New("bad article path")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.Base()+path, nil)
	if err != nil {
		return "", err
	}
	resp, err := m.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kiwix answered %s", resp.Status)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "html") {
		return "", errors.New("not an html article")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	return HTMLToText(string(body), limit), nil
}

var (
	dropRE  = regexp.MustCompile(`(?is)<(script|style|noscript|template|svg|head|nav|footer|table)\b.*?</(script|style|noscript|template|svg|head|nav|footer|table)>`)
	blockRE = regexp.MustCompile(`(?i)</?(p|div|li|h[1-6]|br|section|article|tr|dd|dt|blockquote|pre)\b[^>]*>`)
)

// HTMLToText is a small, dependency-free HTML to text conversion that keeps
// paragraph breaks.
func HTMLToText(s string, limit int) string {
	s = dropRE.ReplaceAllString(s, " ")
	s = blockRE.ReplaceAllString(s, "\n")
	s = tagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if len(line) < 3 {
			continue
		}
		if b.Len()+len(line)+1 > limit {
			break
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
