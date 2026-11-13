package kiwix

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Book is one archive kiwix-serve is serving.
type Book struct {
	Name    string // book name, e.g. wikipedia_en_medicine
	Content string // the /content/<Content>/ path element
	Title   string
}

// Catalog lists the archives kiwix-serve is serving.
func (m *Manager) Catalog(ctx context.Context) ([]Book, error) {
	body, err := m.get(ctx, "/catalog/v2/entries?count=500", 8<<20)
	if err != nil {
		return nil, err
	}
	var feed struct {
		Entries []struct {
			Title string `xml:"title"`
			Name  string `xml:"name"`
			Links []struct {
				Type string `xml:"type,attr"`
				Href string `xml:"href,attr"`
			} `xml:"link"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("kiwix catalog: %w", err)
	}
	var out []Book
	for _, e := range feed.Entries {
		for _, l := range e.Links {
			if l.Type == "text/html" && strings.HasPrefix(l.Href, "/content/") {
				out = append(out, Book{Name: e.Name, Title: e.Title, Content: strings.Trim(strings.TrimPrefix(l.Href, "/content/"), "/")})
			}
		}
	}
	return out, nil
}

// Suggest returns up to limit articles of one archive whose titles match term.
func (m *Manager) Suggest(ctx context.Context, b Book, term string, limit int) ([]Result, error) {
	q := url.Values{}
	q.Set("content", b.Content)
	q.Set("term", term)
	q.Set("count", fmt.Sprint(limit))
	body, err := m.get(ctx, "/suggest?"+q.Encode(), 1<<20)
	if err != nil {
		return nil, err
	}
	var items []struct {
		Value string `json:"value"`
		Kind  string `json:"kind"`
		Path  string `json:"path"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("kiwix suggest: %w", err)
	}
	var out []Result
	for _, it := range items {
		if it.Kind != "path" || it.Path == "" || strings.Contains(it.Path, "..") {
			continue
		}
		out = append(out, Result{Title: it.Value, Path: "/content/" + b.Content + "/" + strings.TrimPrefix(it.Path, "/"), Book: b.Title})
	}
	return out, nil
}

func (m *Manager) get(ctx context.Context, path string, max int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.Base()+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kiwix answered %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, max))
}
