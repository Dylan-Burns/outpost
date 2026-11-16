package ai

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

var stemOK = regexp.MustCompile(`^(s|es|ed|d|ing|in|er|ers|y|ie|ies|ful|ly|ish|less|ness|ache|aches|ing|wave|water|ed|bite|fall|ed|ings|ward|wards|band|bands|ache|cut|cuts)$`)

func TestInside(t *testing.T) {
	b, _ := os.ReadFile("/private/tmp/claude-501/-Users-dylanburns-Documents-GitHub-offline-library/6db6a898-04a5-4f27-8ce4-fd84e46c49a4/scratchpad/nc/vocab.txt")
	short := map[string]bool{}
	for _, w := range strings.Fields("bee bees nut nuts ate sting cut cuts fit fits hit ill pain ache arm leg eye ear hip lip rib gum toe head hand back neck heart cold heat hot burn fire gas oil car cat dog rat bat bit bite tick ant fall fell drop sick dead die dying shot gun knife wet ice sun salt hurt bump rash lump swim pool sea flood storm wind snow rain smoke ash bleed blood cord fly flu tea pan pot cook hair nail skin bone baby kid son mum dad nan gran pill pills dose old age bed lie cry pee poo wee spit cough chest belly gut stab trip slip run walk stuck lost dark app gen co fit od ate eat sip tap jab nap lit wick dip sob tin can jaw") {
		short[w] = true
	}
	seen := map[string]bool{}
	for _, w := range strings.Fields(string(b)) {
		for i, e := range emergencies {
			for _, r := range e.all {
				for _, loc := range r.FindAllStringIndex(w, -1) {
					m := w[loc[0]:loc[1]]
					if m == w || !short[m] {
						continue
					}
					if loc[0] == 0 && stemOK.MatchString(w[loc[1]:]) {
						continue
					}
					k := fmt.Sprint(i, m, w)
					if !seen[k] {
						seen[k] = true
						fmt.Printf("rule %d %v: %q inside %q\n", i, e.topics, m, w)
					}
				}
			}
		}
	}
}
