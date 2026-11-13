// Package retrieval connects the assistant's retrieval to the kiwix library.
package retrieval

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Dylan-Burns/outpost/node/internal/ai"
	"github.com/Dylan-Burns/outpost/node/internal/kiwix"
	"github.com/Dylan-Burns/outpost/node/internal/pdfindex"
)

// Kiwix adapts a kiwix manager to ai.Library. Books, when set, limits the
// search to those book names.
type Kiwix struct {
	M     *kiwix.Manager
	Books []string
	PDF   *pdfindex.Index // text of the PDFs inside archives; may be nil
}

// pdfMark separates a PDF's path from its page number in a hit's path. PDF
// viewers in browsers open the page named by #page=N.
const pdfMark = "#page="

func (k Kiwix) Search(ctx context.Context, query string, limit int) ([]ai.Hit, error) {
	res, _, err := k.M.SearchIn(ctx, query, k.Books, 0, limit)
	out := make([]ai.Hit, 0, len(res))
	for _, r := range res {
		out = append(out, ai.Hit{Title: r.Title, Path: r.Path, Snippet: r.Snippet, Book: r.Book})
	}
	// Interleave the PDF passages so neither kind crowds out the other.
	var pdf []ai.Hit
	for _, h := range k.PDF.Search(query, limit) {
		if len(k.Books) > 0 && !k.inBooks(h.Path) {
			continue
		}
		pdf = append(pdf, ai.Hit{Title: fmt.Sprintf("%s, page %d", h.Doc, h.Page), Path: h.Path + pdfMark + strconv.Itoa(h.Page), Snippet: h.Text, Book: h.Book})
	}
	var mixed []ai.Hit
	for i := 0; i < len(out) || i < len(pdf); i++ {
		if i < len(out) {
			mixed = append(mixed, out[i])
		}
		if i < len(pdf) {
			mixed = append(mixed, pdf[i])
		}
	}
	if len(pdf) > 0 {
		err = nil
	}
	return mixed, err
}

// inBooks reports whether a PDF path belongs to one of k.Books (book names
// are the archive names without their date).
func (k Kiwix) inBooks(p string) bool {
	for _, b := range k.Books {
		if strings.HasPrefix(p, "/content/"+b+"_") || strings.HasPrefix(p, "/content/"+b+"/") {
			return true
		}
	}
	return false
}

// Suggest looks a title up in every archive (or only the named ones).
func (k Kiwix) Suggest(ctx context.Context, term string, perBook int) ([]ai.Hit, error) {
	books, err := k.M.Catalog(ctx)
	if err != nil {
		return nil, err
	}
	var out []ai.Hit
	for _, b := range books {
		if len(k.Books) > 0 && !slices.Contains(k.Books, b.Name) {
			continue
		}
		res, err := k.M.Suggest(ctx, b, term, perBook)
		if err != nil {
			continue
		}
		for _, r := range res {
			out = append(out, ai.Hit{Title: r.Title, Path: r.Path, Book: r.Book})
		}
	}
	return out, nil
}

func (k Kiwix) ArticleText(ctx context.Context, path string, limit int) (string, error) {
	if i := strings.Index(path, pdfMark); i > 0 {
		n, err := strconv.Atoi(path[i+len(pdfMark):])
		if c, ok := k.PDF.Get(path[:i], n); err == nil && ok {
			return c.Text, nil
		}
		return "", fmt.Errorf("no such PDF page: %s", path)
	}
	return k.M.ArticleText(ctx, path, limit)
}
