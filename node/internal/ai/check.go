package ai

import (
	"regexp"
	"strings"
)

var (
	citeRE     = regexp.MustCompile(`\[S\d+\]`)
	listNumRE  = regexp.MustCompile(`(?m)^\s*\d+[.)]\s`)
	numberRE   = regexp.MustCompile(`\d+(?:[.,/]\d+)*`)
	numPhrase  = regexp.MustCompile(`\d+(?:[.,/:–-]\d+)*\s*(?:%|°\s?[CF]?|[a-zA-Z]+)?`)
	thousandRE = regexp.MustCompile(`(\d),(\d{3})`)
)

// UnsupportedNumbers returns the numbers in an answer (with the word after
// each, such as "4 drops") that appear in none of its sources. Small models
// invent doses and times; a number the reader cannot find in a source is the
// first thing to check.
func UnsupportedNumbers(answer string, sources []Source) []string {
	var src strings.Builder
	for _, s := range sources {
		src.WriteString(s.Text)
		src.WriteByte(' ')
	}
	have := map[string]bool{}
	for _, n := range numberRE.FindAllString(thousandRE.ReplaceAllString(src.String(), "$1$2"), -1) {
		have[n] = true
	}
	a := listNumRE.ReplaceAllString(citeRE.ReplaceAllString(answer, " "), " ")
	var out []string
	seen := map[string]bool{}
	for _, phrase := range numPhrase.FindAllString(a, -1) {
		for _, n := range numberRE.FindAllString(thousandRE.ReplaceAllString(phrase, "$1$2"), -1) {
			if !have[n] {
				p := strings.TrimSpace(phrase)
				if !seen[p] {
					seen[p] = true
					out = append(out, p)
				}
				break
			}
		}
	}
	return out
}

// doseRE matches an amount of a medicine or chemical: "500 mg", "2 tablets",
// "10 ml", "4 units".
var doseRE = regexp.MustCompile(`(?i)\b\d+(?:[.,]\d+)?\s?(?:mg|milligrams?|mcg|micrograms?|µg|ml|millilit(?:re|er)s?|tablets?|capsules?|units?)\b`)

// MentionsDose reports whether an answer gives an amount of a medicine or
// chemical. Small models state doses even when told not to: Qwen3 1.7B gave
// a 5-year-old the adult amoxicillin dose it found in an NHS page. The page
// shows a fixed warning beside such an answer.
func MentionsDose(answer string) bool {
	return doseRE.MatchString(citeRE.ReplaceAllString(answer, " "))
}
