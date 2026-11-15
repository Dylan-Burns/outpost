package ai

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestParaphraseRecall guards the note rules against eval/assistant/
// paraphrases.json: 534 questions (6 per note) written by someone who saw
// only the note titles, and 60 decoys that share their words ("how do i
// bleed my radiators", "hurricane cocktail recipe"). Every blind review
// round found harmful answers where a rule missed everyday wording, so a
// change that loses recall, or matches a decoy, fails here first. Raise the
// floor when recall improves.
func TestParaphraseRecall(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	b, err := os.ReadFile("../../../eval/assistant/paraphrases.json")
	if err != nil {
		t.Fatal(err)
	}
	var d struct {
		Positive []struct{ Title, Q string }
		Negative []string
	}
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	hit := 0
	for _, c := range d.Positive {
		for _, n := range Notes(c.Q) {
			if strings.TrimPrefix(n.Title, "Outpost note: ") == c.Title {
				hit++
				break
			}
		}
	}
	const floor = 395
	if hit < floor {
		t.Errorf("paraphrase recall %d of %d, below the floor of %d", hit, len(d.Positive), floor)
	}
	for _, q := range d.Negative {
		if n := Notes(q); len(n) > 0 {
			t.Errorf("decoy %q got %v", q, titles(n))
		}
	}
	t.Logf("paraphrase recall %d of %d", hit, len(d.Positive))
}
