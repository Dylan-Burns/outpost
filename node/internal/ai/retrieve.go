package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Hit is one search result from the library.
type Hit struct {
	Title, Path, Snippet, Book string
}

// Library is the part of the content library that retrieval needs.
type Library interface {
	// Search is a full-text search across the library.
	Search(ctx context.Context, query string, limit int) ([]Hit, error)
	// Suggest finds articles by title, up to perBook in each archive.
	Suggest(ctx context.Context, term string, perBook int) ([]Hit, error)
	ArticleText(ctx context.Context, path string, limit int) (string, error)
}

// Topics names the articles that would answer a question, such as
// "Snakebite" for "my friend was bitten by a snake". People describe an
// emergency in their own words; the library files it under its proper name.
type Topics func(ctx context.Context, question string) []string

type candidate struct {
	hit     Hit
	score   float64
	passage string
}

// Retrieve searches the library for the question and returns up to four
// passages to ground an answer on. topics may be nil.
func Retrieve(ctx context.Context, lib Library, q string, topics Topics) []Source {
	terms := Terms(q)
	// Known emergencies first: they are certain; the model's guesses are not.
	named := EmergencyTopics(q)
	if topics != nil {
		tctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		for _, t := range topics(tctx, q) {
			if !contains(named, t) {
				named = append(named, t)
			}
		}
		cancel()
	}
	// Words of the named topics count as the question's words too.
	all := append([]string(nil), terms...)
	for _, t := range named {
		for _, w := range Terms(t) {
			if !contains(all, w) {
				all = append(all, w)
			}
		}
	}
	if len(all) == 0 {
		return nil
	}
	sctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	byPath := map[string]*candidate{}
	add := func(h Hit, s float64) {
		if c, ok := byPath[h.Path]; ok {
			c.score += s
			if c.hit.Snippet == "" {
				c.hit.Snippet = h.Snippet
			}
			return
		}
		byPath[h.Path] = &candidate{hit: h, score: s}
	}
	// 1. Articles named after the topic.
	suggest := func(t string, exact, prefix float64) {
		hits, _ := lib.Suggest(sctx, t, 3)
		lt := strings.ToLower(t)
		for _, h := range hits {
			ht := strings.ToLower(h.Title)
			switch {
			case ht == lt:
				add(h, exact)
			case strings.HasPrefix(ht, lt):
				add(h, prefix)
			default:
				add(h, 0.5)
			}
		}
	}
	for _, t := range named {
		suggest(t, 6, 2)
		if c := coreTopic(t); c != "" && c != t {
			suggest(c, 4, 1)
		}
	}
	// 2. Full-text search, on the named topics and on the question's words.
	var queries []string
	if len(named) > 0 {
		queries = append(queries, strings.Join(named, " "))
	}
	if len(terms) > 0 {
		queries = append(queries, strings.Join(terms, " "))
	}
	for _, query := range queries {
		hits, _ := lib.Search(sctx, query, 8)
		for i, h := range hits {
			add(h, 1.5-float64(i)*0.15)
		}
	}
	// Titles that share the question's words usually answer it; titles that
	// share none rarely do, whatever the full-text index thought.
	var cands []*candidate
	for _, c := range byPath {
		tw := overlap(c.hit.Title, all)
		c.score += 1.5 * float64(tw)
		if tw == 0 && overlap(c.hit.Snippet, all) < 2 {
			continue
		}
		cands = append(cands, c)
	}
	sort.SliceStable(cands, func(a, b int) bool {
		if cands[a].score != cands[b].score {
			return cands[a].score > cands[b].score
		}
		return cands[a].hit.Path < cands[b].hit.Path
	})
	// Read the best few and keep the passages that actually cover the words.
	var read []*candidate
	seenTitle := map[string]bool{}
	for _, c := range cands {
		if len(read) >= 7 {
			break
		}
		key := strings.ToLower(c.hit.Title)
		if seenTitle[key] {
			continue
		}
		text, err := lib.ArticleText(sctx, c.hit.Path, 60000)
		if err != nil || len(text) < 80 {
			text = c.hit.Snippet
		}
		p := bestPassages(text, all, q, 1800)
		if len(p) < 60 {
			p = c.hit.Snippet
		}
		if p == "" {
			continue
		}
		cover := float64(overlap(p, all)) / float64(len(all))
		// A passage must share a word with the question itself, not only
		// with the model's guess at the topic, which can be wrong.
		if cover == 0 || len(terms) > 0 && overlap(c.hit.Title+" "+p, terms) == 0 {
			continue
		}
		// The same document is often in two archives (a manual in both a
		// medical and a disaster library); keep one copy.
		textKey := passageKey(p)
		if seenTitle[textKey] {
			continue
		}
		seenTitle[key] = true
		seenTitle[textKey] = true
		c.passage = p
		c.score += 3 * cover
		read = append(read, c)
	}
	sort.SliceStable(read, func(a, b int) bool { return read[a].score > read[b].score })
	var out []Source
	for _, c := range read {
		if len(out) == 4 {
			break
		}
		// A source far weaker than the best one is noise that a small model
		// will try to use anyway.
		if len(out) > 0 && c.score < read[0].score*0.35 {
			break
		}
		out = append(out, Source{Label: fmt.Sprintf("S%d", len(out)+1), Title: c.hit.Title, Book: c.hit.Book, Path: c.hit.Path, Text: c.passage})
	}
	return out
}

// overlap counts how many of words occur in s, in any common inflection.
func overlap(s string, words []string) int {
	ls, n := strings.ToLower(s), 0
	for _, w := range words {
		if strings.Contains(ls, stem(w)) {
			n++
		}
	}
	return n
}

// stem strips a common English ending, so "swallowed" matches "swallows"
// and "bleeding" matches "bleeds".
func stem(w string) string {
	for _, suf := range []string{"ing", "ed", "es", "s"} {
		if len(w) > len(suf)+3 && strings.HasSuffix(w, suf) {
			return w[:len(w)-len(suf)]
		}
	}
	return w
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// passageKey identifies a passage by its opening words.
func passageKey(p string) string {
	f := strings.Fields(strings.ToLower(p))
	if len(f) > 30 {
		f = f[:30]
	}
	return "text:" + strings.Join(f, " ")
}
