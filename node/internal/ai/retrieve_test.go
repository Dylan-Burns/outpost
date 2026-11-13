package ai

import (
	"context"
	"strings"
	"testing"
)

type fakeLib struct {
	search  map[string][]Hit
	suggest map[string][]Hit
	text    map[string]string
}

func (f fakeLib) Search(_ context.Context, q string, _ int) ([]Hit, error) { return f.search[q], nil }
func (f fakeLib) Suggest(_ context.Context, t string, _ int) ([]Hit, error) {
	return f.suggest[t], nil
}
func (f fakeLib) ArticleText(_ context.Context, p string, _ int) (string, error) {
	return f.text[p], nil
}

func TestRetrievePrefersNamedTopicOverWordMatch(t *testing.T) {
	lib := fakeLib{
		// The words of "I broke my left ankle today" match phone repair guides.
		search: map[string][]Hit{
			"broke ankle": {{Title: "iPhone SE LCD Replacement", Path: "/content/ifixit/a", Snippet: "the cable broke on the left side"}},
		},
		suggest: map[string][]Hit{
			"Ankle fracture": {{Title: "Ankle fracture", Path: "/content/med/Ankle_fracture"}},
		},
		text: map[string]string{
			"/content/ifixit/a":           "Remove the screen. The cable broke on the left side of the phone.",
			"/content/med/Ankle_fracture": "An ankle fracture is a break of one or more ankle bones.\nTreatment: immobilise the ankle with a splint and keep weight off it.",
		},
	}
	topics := func(context.Context, string) []string { return []string{"Ankle fracture"} }
	src := Retrieve(context.Background(), lib, "I broke my left ankle today I think.", topics)
	if len(src) == 0 || src[0].Title != "Ankle fracture" {
		t.Fatalf("got %+v", src)
	}
	for _, s := range src {
		if strings.Contains(s.Title, "iPhone") {
			t.Fatalf("phone guide kept: %+v", src)
		}
	}
}

func TestRetrieveDropsDuplicatePassages(t *testing.T) {
	page := "Where there is no doctor. Poisoning: if a child swallows medicine, get help at once and keep the container."
	lib := fakeLib{
		search: map[string][]Hit{
			"child swallowed pills": {
				{Title: "Where there is no Doctor, page 150", Path: "/content/a/x.pdf#page=150", Snippet: page},
				{Title: "Where-there-is-no-Doctor-book, page 150", Path: "/content/b/y.pdf#page=150", Snippet: page},
			},
		},
		text: map[string]string{"/content/a/x.pdf#page=150": page, "/content/b/y.pdf#page=150": page},
	}
	src := Retrieve(context.Background(), lib, "child swallowed pills", nil)
	if len(src) != 1 {
		t.Fatalf("want one copy, got %+v", src)
	}
}

func TestTermsDropsConversationalFiller(t *testing.T) {
	got := strings.Join(Terms("I think someone broke their left ankle today"), " ")
	if got != "broke ankle" {
		t.Fatalf("got %q", got)
	}
}

func TestEmergencyTopics(t *testing.T) {
	cases := map[string]string{
		"My child swallowed some pills from the cabinet.":                      "Poisoning",
		"We are running a generator in the garage and everyone has a headache": "Carbon monoxide poisoning",
		"A diabetic friend is shaky, sweaty and confused.":                     "Hypoglycemia",
		"my dog got bitten by a snake":                                         "Snakebite",
		"What are the signs of a heart attack?":                                "Myocardial infarction",
	}
	for q, want := range cases {
		if got := EmergencyTopics(q); len(got) == 0 || got[0] != want {
			t.Errorf("%q: got %v, want %s first", q, got, want)
		}
	}
	for _, q := range []string{"A dog bit my hand and broke the skin.", "Someone collapsed from heat stroke."} {
		for _, got := range EmergencyTopics(q) {
			if got == "Bone fracture" || got == "Stroke" {
				t.Errorf("%q matched %s", q, got)
			}
		}
	}
	if got := EmergencyTopics("How do I tie a bowline knot?"); len(got) != 0 {
		t.Errorf("knot question matched %v", got)
	}
	if coreTopic("Stroke facts and symptoms") != "Stroke" || coreTopic("Snake bite first aid") != "Snake bite" {
		t.Error("coreTopic")
	}
}

func TestOverlapMatchesWordStarts(t *testing.T) {
	if overlap("The human back", []string{"man"}) != 0 {
		t.Fatal("man matched inside human")
	}
	if overlap("A man swallows pills", []string{"man", "swallowed", "pills"}) != 3 {
		t.Fatal("whole words missed")
	}
}
