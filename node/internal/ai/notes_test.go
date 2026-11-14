package ai

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestMoreEmergencyTopics(t *testing.T) {
	has := func(q, want string) bool {
		for _, g := range EmergencyTopics(q) {
			if g == want {
				return true
			}
		}
		return false
	}
	yes := map[string]string{
		"we found a bat in the bedroom when we woke up":                   "Rabies",
		"is it ok to run a generator inside the house if I open a window": "Carbon monoxide poisoning",
		"our house is on fire and the hallway is full of smoke":           "Smoke inhalation",
		"a pan of oil caught fire on the stove":                           "Smoke inhalation",
		"we are camping on a hill and a thunderstorm is coming":           "Lightning",
		"someone touched a live wire and collapsed":                       "Electrical injury",
		"my kid has a nosebleed that won't stop":                          "Nosebleed",
		"there is a bug stuck in my ear":                                  "Foreign body",
		"Is it safe to can green beans in a water bath?":                  "Botulism",
		"can I give honey to my 6 month old for a cough":                  "Botulism",
	}
	for q, want := range yes {
		if !has(q, want) {
			t.Errorf("%q: got %v, want %s", q, EmergencyTopics(q), want)
		}
	}
	no := map[string]string{
		"How do I start a fire when everything is wet?": "Smoke inhalation",
		"what makes good tinder for a campfire":         "Smoke inhalation",
		"how do I safely use a wood stove in my cabin":  "Smoke inhalation",
		"what is the best bat for baseball":             "Rabies",
		"How do I tie a bowline knot?":                  "",
	}
	for q, not := range no {
		got := EmergencyTopics(q)
		if not == "" && len(got) > 0 || not != "" && has(q, not) {
			t.Errorf("%q matched %v", q, got)
		}
	}
}

func TestNotes(t *testing.T) {
	UseNotes = false
	if n := Notes("An adult is choking and can't breathe"); len(n) != 0 {
		t.Fatalf("notes while off: %v", n)
	}
	UseNotes = true
	defer func() { UseNotes = false }()
	n := Notes("An adult is choking and can't breathe or talk. What do I do?")
	if len(n) != 1 || !strings.Contains(n[0].Title, "Choking") || !strings.Contains(n[0].Text, "abdominal thrusts") {
		t.Fatalf("choking: %+v", n)
	}
	if n := Notes("Is it safe to can green beans in a water bath?"); len(n) == 0 || !strings.Contains(n[0].Text, "pressure canner") {
		t.Fatalf("canning: %+v", n)
	}
	if n := Notes("how much bleach per gallon to make water safe to drink"); len(n) == 0 || !strings.Contains(n[0].Title, "safe to drink") {
		t.Fatalf("water: %+v", n)
	}
	if n := Notes("How do I tie a bowline knot?"); len(n) != 0 {
		t.Fatalf("knot: %+v", n)
	}
	// Every note names its basis and every topic is one a rule can produce.
	for _, x := range notes {
		if x.basis == "" || x.text == "" {
			t.Errorf("note %q is incomplete", x.title)
		}
	}
}

func TestRetrievePutsNotesFirst(t *testing.T) {
	UseNotes = true
	defer func() { UseNotes = false }()
	lib := fakeLib{
		suggest: map[string][]Hit{"Choking": {{Title: "Choking", Path: "/content/med/Choking"}}},
		text:    map[string]string{"/content/med/Choking": "Choking is when breathing is blocked.\nTreatment: back blows and abdominal thrusts."},
	}
	src := Retrieve(context.Background(), lib, "An adult is choking and can't breathe", nil)
	if len(src) != 2 || src[0].Label != "S1" || !strings.HasPrefix(src[0].Title, "Outpost note") || src[1].Label != "S2" || src[1].Title != "Choking" {
		t.Fatalf("got %+v", src)
	}
}

func TestRetrieveTrustsRuleTopics(t *testing.T) {
	// "touched a live wire" shares no word with "Electrical injury".
	lib := fakeLib{
		suggest: map[string][]Hit{"Electrical injury": {{Title: "Electrical injury", Path: "/content/med/Electrical_injury"}}},
		text:    map[string]string{"/content/med/Electrical_injury": "An electrical injury is damage from current passing through the body.\nTreatment: switch off the power before touching the person."},
	}
	src := Retrieve(context.Background(), lib, "someone touched a live wire and collapsed", nil)
	if len(src) == 0 || src[0].Title != "Electrical injury" {
		t.Fatalf("got %+v", src)
	}
}

func TestRetrieveTakesTwoPagesPerPDF(t *testing.T) {
	var hits []Hit
	text := map[string]string{}
	for i := 1; i <= 5; i++ {
		p := fmt.Sprintf("/content/cook/recipes.pdf#page=%d", i)
		hits = append(hits, Hit{Title: fmt.Sprintf("Camping Recipes, page %d", i), Path: p, Book: "Food"})
		text[p] = fmt.Sprintf("Page %d. Camping on a hill: a storm recipe for camping stew, thunderstorm cake and hill bread.", i)
	}
	lib := fakeLib{search: map[string][]Hit{"camping hill thunderstorm coming": hits}, text: text}
	src := Retrieve(context.Background(), lib, "we are camping on a hill and a thunderstorm is coming", nil)
	if len(src) != 2 {
		t.Fatalf("got %d pages of one PDF: %+v", len(src), src)
	}
}
