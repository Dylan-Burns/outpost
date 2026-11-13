// Package pdfindex makes the PDF documents inside ZIM archives searchable.
//
// Several survival and medical libraries (zimgit collections, the military
// medicine manuals) are PDFs wrapped in a ZIM file. kiwix-serve cannot search
// inside them, so without this index the assistant never reads them. The
// index finds each archive's PDFs, extracts their text page by page, keeps it
// in a cache file per archive, and ranks passages with BM25.
package pdfindex

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// Chunk is a passage of one PDF: one page, or a few short pages together.
type Chunk struct {
	Book   string `json:"b"` // archive title, e.g. "Water Treatment Library"
	Doc    string `json:"d"` // document title
	Path   string `json:"p"` // /content/<archive>/<file>.pdf
	Page   int    `json:"n"` // first page, from 1
	Text   string `json:"t"`
	terms  map[string]int
	length int
}

// Index ranks chunks by BM25.
type Index struct {
	chunks []Chunk
	post   map[string][]int // term -> chunk ids
	avgLen float64
}

// Tokens splits text into lower-case words of two or more letters or digits.
func Tokens(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// New indexes the chunks.
func New(chunks []Chunk) *Index {
	ix := &Index{chunks: chunks, post: map[string][]int{}}
	total := 0
	for i := range ix.chunks {
		c := &ix.chunks[i]
		c.terms = map[string]int{}
		for _, t := range Tokens(c.Doc + " " + c.Text) {
			if len(t) < 2 {
				continue
			}
			c.terms[t]++
			c.length++
		}
		for t := range c.terms {
			ix.post[t] = append(ix.post[t], i)
		}
		total += c.length
	}
	if len(chunks) > 0 {
		ix.avgLen = float64(total) / float64(len(chunks))
	}
	return ix
}

// Len is the number of chunks.
func (ix *Index) Len() int {
	if ix == nil {
		return 0
	}
	return len(ix.chunks)
}

// Hit is a ranked chunk.
type Hit struct {
	Chunk
	Score float64
}

// Search returns the best n chunks for the query words. Words of the
// document's title count twice, so a manual about the subject outranks a
// passing mention.
func (ix *Index) Search(query string, n int) []Hit {
	if ix.Len() == 0 {
		return nil
	}
	const k1, b = 1.2, 0.75
	N := float64(len(ix.chunks))
	scores := map[int]float64{}
	seen := map[string]bool{}
	for _, t := range Tokens(query) {
		if len(t) < 3 || seen[t] {
			continue
		}
		seen[t] = true
		ids := ix.post[t]
		if len(ids) == 0 {
			continue
		}
		idf := math.Log(1 + (N-float64(len(ids))+0.5)/(float64(len(ids))+0.5))
		for _, id := range ids {
			c := &ix.chunks[id]
			tf := float64(c.terms[t])
			s := idf * tf * (k1 + 1) / (tf + k1*(1-b+b*float64(c.length)/ix.avgLen))
			if strings.Contains(strings.ToLower(c.Doc), t) {
				s *= 2
			}
			scores[id] += s
		}
	}
	out := make([]Hit, 0, len(scores))
	for id, s := range scores {
		out = append(out, Hit{Chunk: ix.chunks[id], Score: s})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Score != out[b].Score {
			return out[a].Score > out[b].Score
		}
		return out[a].Path+string(rune(out[a].Page)) < out[b].Path+string(rune(out[b].Page))
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// Get returns the chunk at a path and page.
func (ix *Index) Get(path string, page int) (Chunk, bool) {
	if ix == nil {
		return Chunk{}, false
	}
	for _, c := range ix.chunks {
		if c.Path == path && c.Page == page {
			return c, true
		}
	}
	return Chunk{}, false
}
