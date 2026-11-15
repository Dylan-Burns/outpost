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

// MaxSources and PassageBudget bound what the model reads: at most this many
// passages of at most this many bytes each.
var (
	MaxSources    = 4
	PassageBudget = 1800
)

type candidate struct {
	hit     Hit
	score   float64
	passage string
	byWords bool // the full-text search on the question's own words found it
	byRule  bool // an article titled after a known emergency the question describes
}

// Retrieve searches the library for the question and returns up to four
// passages to ground an answer on. topics may be nil.
func Retrieve(ctx context.Context, lib Library, q string, topics Topics) []Source {
	terms := Terms(q)
	// Known emergencies first: they are certain; the model's guesses are not.
	named := EmergencyTopics(q)
	ruled := len(named)
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
	suggest := func(t string, exact, prefix float64, rule bool) {
		hits, _ := lib.Suggest(sctx, t, 3)
		lt := strings.ToLower(t)
		for _, h := range hits {
			ht := strings.ToLower(h.Title)
			switch {
			case ht == lt:
				add(h, exact)
				if rule {
					byPath[h.Path].byRule = true
				}
			case strings.HasPrefix(ht, lt):
				add(h, prefix)
			default:
				add(h, 0.5)
			}
		}
	}
	for i, t := range named {
		suggest(t, 6, 2, i < ruled)
		if c := coreTopic(t); c != "" && c != t {
			suggest(c, 4, 1, false)
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
	for qi, query := range queries {
		hits, _ := lib.Search(sctx, query, 8)
		for i, h := range hits {
			add(h, 1.5-float64(i)*0.15)
			if qi == len(queries)-1 && len(terms) > 0 {
				byPath[h.Path].byWords = true
			}
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
	// At most two pages of one PDF: four pages of "Camping Recipes" once
	// filled every place for a question about a thunderstorm on a campsite.
	perDoc := map[string]int{}
	for _, c := range cands {
		if len(read) >= 7 {
			break
		}
		key := strings.ToLower(c.hit.Title)
		if seenTitle[key] {
			continue
		}
		doc := c.hit.Book + "\x00" + pdfDoc(c.hit.Title)
		if perDoc[doc] >= 2 {
			continue
		}
		text, err := lib.ArticleText(sctx, c.hit.Path, 60000)
		if err != nil || len(text) < 80 {
			text = c.hit.Snippet
		}
		p := bestPassages(text, all, q, PassageBudget)
		if len(p) < 60 {
			p = c.hit.Snippet
		}
		if p == "" {
			continue
		}
		cover := float64(overlap(p, all)) / float64(len(all))
		// A passage must share words with the question itself, not only with
		// the model's guess at the topic, which can be wrong ("Back" for a
		// question about an unconscious man's position). One word will do for
		// what the question's own words found, or when the title has it; a
		// topic's other finds need two.
		// An article whose title is exactly a known emergency the question
		// describes needs none: "touched a live wire" shares no word with
		// "Electrical injury".
		need := 1
		if !c.byWords && overlap(c.hit.Title, terms) == 0 {
			need = min(2, len(terms))
		}
		if c.byRule {
			need = 0
		}
		if cover == 0 || len(terms) > 0 && overlap(c.hit.Title+" "+p, terms) < need {
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
		perDoc[doc]++
		c.passage = p
		c.score += 3 * cover
		read = append(read, c)
	}
	sort.SliceStable(read, func(a, b int) bool { return read[a].score > read[b].score })
	// Outpost's own notes come first, then the library's passages.
	out := Notes(q)
	if NotesOnly && len(out) > 0 {
		for i := range out {
			out[i].Label = fmt.Sprintf("S%d", i+1)
		}
		return out
	}
	for i := range out {
		out[i].Label = fmt.Sprintf("S%d", i+1)
	}
	fromLib := 0
	for _, c := range read {
		if fromLib == MaxSources {
			break
		}
		// A source far weaker than the best one is noise that a small model
		// will try to use anyway.
		if fromLib > 0 && c.score < read[0].score*0.35 {
			break
		}
		fromLib++
		out = append(out, Source{Label: fmt.Sprintf("S%d", len(out)+1), Title: c.hit.Title, Book: c.hit.Book, Path: c.hit.Path, Text: c.passage})
	}
	return out
}

// overlap counts how many of words occur in s, in any common inflection.
// A word must start a word of s: "man" is not in "human".
func overlap(s string, words []string) int {
	ls, n := " "+strings.ToLower(s), 0
	for _, w := range words {
		st := stem(w)
		for i := strings.Index(ls, st); i >= 0; {
			if c := ls[i-1]; !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
				n++
				break
			}
			j := strings.Index(ls[i+1:], st)
			if j < 0 {
				break
			}
			i += 1 + j
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

// pdfDoc returns the document name of a PDF page's title ("Camping
// Recipes, page 18" is "Camping Recipes"), or the title itself.
func pdfDoc(title string) string {
	if i := strings.LastIndex(title, ", page "); i > 0 {
		return title[:i]
	}
	return title
}

// passageKey identifies a passage by its opening words.
func passageKey(p string) string {
	f := strings.Fields(strings.ToLower(p))
	if len(f) > 30 {
		f = f[:30]
	}
	return "text:" + strings.Join(f, " ")
}
