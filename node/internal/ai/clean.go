package ai

import (
	"regexp"
	"strings"
	"unicode"
)

// minEcho is the shortest run of words copied from the instructions that is
// removed. Shorter runs are ordinary phrases ("the most important thing to
// do is"); a model restating its instructions copies ten words or more.
const minEcho = 7

var (
	blankLinesRE       = regexp.MustCompile(`\n{3,}`)
	spaceBeforePunctRE = regexp.MustCompile(`[ \t]+([.,;:!?])`)
	sentenceEndRE      = regexp.MustCompile(`[.!?](\s*\[S\d+\])*[ \t]*|\n`)
	spacesRE           = regexp.MustCompile(`[ \t]{2,}`)
	// What a removed run leaves behind at the start of a line: a colon,
	// "are:", "as follows:", list punctuation.
	leftoverRE  = regexp.MustCompile(`(?m)^[ \t]*(?:[-*•]\s*)?(?:(?:are|is|as follows|such as|like)\s*)?[:;,.]+[ \t]*`)
	emptyItemRE = regexp.MustCompile(`(?m)^[ \t]*(?:[-*•]|\d+[.)])?[ \t]*(?:\*\*)?[ \t]*$`)
)

type span struct{ start, end int }

// declineWord reports whether a run of instruction words is part of the
// wording a decline is told to use, which the answer should keep.
func declineWord(ws []string) bool {
	for _, w := range ws {
		switch w {
		case "library", "cover", "cannot", "search", "covers", "covered":
			return true
		}
	}
	return false
}

// words returns the lower-cased words of s with their byte positions.
func words(s string) (ws []string, at []span) {
	start := -1
	for i, r := range s {
		in := unicode.IsLetter(r) || unicode.IsDigit(r) || r == '\''
		if in && start < 0 {
			start = i
		}
		if !in && start >= 0 {
			ws = append(ws, strings.ToLower(s[start:i]))
			at = append(at, span{start, i})
			start = -1
		}
	}
	if start >= 0 {
		ws = append(ws, strings.ToLower(s[start:]))
		at = append(at, span{start, len(s)})
	}
	return ws, at
}

// CleanAnswer removes the parts of an answer copied from the instructions it
// was given. Qwen3 1.7B, whatever the wording, begins many answers by
// restating them ("Open with the action that matters most when someone could
// be in danger: Apply a splint"); this keeps "Apply a splint".
//
// A copy is a run of at least minEcho answer words that appears, word for
// word and in order, in the instructions. Runs that contain the wording a
// decline is meant to use are kept.
func CleanAnswer(answer, instructions string) string {
	// A model that cannot stop thinking (Qwen3 4B Thinking 2507 ignores
	// think=false) writes its reasoning first; only what follows it is the
	// answer.
	if i := strings.LastIndex(answer, "</think>"); i >= 0 {
		answer = strings.TrimSpace(answer[i+len("</think>"):])
	}
	iw, _ := words(instructions)
	aw, at := words(answer)
	// longest[i] is the length of the longest run starting at answer word i
	// that also occurs in the instructions.
	type run struct{ i, n int }
	var runs []run
	for i := 0; i < len(aw); {
		best := 0
		for k := range iw {
			n := 0
			for i+n < len(aw) && k+n < len(iw) && aw[i+n] == iw[k+n] {
				n++
			}
			if n > best {
				best = n
			}
		}
		if best >= minEcho {
			// A decline's wording is meant to be said; keep all of it.
			if !declineWord(aw[i : i+best]) {
				runs = append(runs, run{i, best})
			}
			i += best
			continue
		}
		i++
	}
	if len(runs) == 0 {
		return answer
	}
	var b strings.Builder
	last := 0
	for _, r := range runs {
		from, to := at[r.i].start, at[r.i+r.n-1].end
		if from < last {
			continue
		}
		// The sentence the run is in: after the last line break, full stop
		// or citation, up to the next one.
		sStart := 0
		for _, sep := range []string{"\n", ". ", "! ", "? ", "] "} {
			if k := strings.LastIndex(answer[:from], sep); k >= 0 && k+len(sep) > sStart {
				sStart = k + len(sep)
			}
		}
		if sStart < last {
			sStart = last
		}
		sEnd := len(answer)
		if k := sentenceEndRE.FindStringIndex(answer[to:]); k != nil {
			sEnd = to + k[1]
		}
		before, _ := words(answer[sStart:from])
		after, _ := words(answer[to:sEnd])
		switch {
		case len(before)+len(after) < 8:
			// Little is left of the sentence: drop it.
			from, to = sStart, sEnd
		case len(before) <= 6:
			// A copied lead-in to a colon: drop it up to the colon.
			if c := strings.Index(answer[to:sEnd], ":"); c >= 0 {
				if between, _ := words(answer[to : to+c]); len(between) <= 4 {
					from, to = sStart, to+c+1
				}
			}
		}
		b.WriteString(answer[last:from])
		last = to
	}
	b.WriteString(answer[last:])
	out := leftoverRE.ReplaceAllString(b.String(), "")
	out = spaceBeforePunctRE.ReplaceAllString(out, "$1")
	out = emptyItemRE.ReplaceAllString(out, "")
	out = spacesRE.ReplaceAllString(out, " ")
	out = blankLinesRE.ReplaceAllString(out, "\n\n")
	return strings.TrimSpace(out)
}

// SystemPrompt returns the instructions BuildPrompt gives for a question.
func SystemPrompt(question string) string { return systemPrompt(question) }
