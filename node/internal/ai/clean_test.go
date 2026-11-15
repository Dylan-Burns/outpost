package ai

import (
	"strings"
	"testing"
)

func TestCleanAnswer(t *testing.T) {
	PromptStyle = "v5"
	defer func() { PromptStyle = "v2" }()
	sys := systemPrompt("How do I make a splint for a broken arm?")
	got := CleanAnswer("Open with the action that matters most when someone could be in danger: **Apply a splint to immobilize the arm**.\n\nThe other key steps or facts as a numbered list of no more than five short items, most important first, and end each item with the label of the passage it comes from:\n\n1. Pad the splint [S1].\n2. Check circulation [S2].", sys)
	for _, bad := range []string{"matters most when someone", "numbered list of no more", "label of the passage", "Open with", "]:"} {
		if strings.Contains(got, bad) {
			t.Errorf("left %q in:\n%s", bad, got)
		}
	}
	for _, good := range []string{"Apply a splint to immobilize the arm", "1. Pad the splint [S1].", "2. Check circulation [S2]."} {
		if !strings.Contains(got, good) {
			t.Errorf("lost %q:\n%s", good, got)
		}
	}
	plain := "Press firmly on the wound with a clean cloth [S1].\n1. Keep pressing [S1]."
	if CleanAnswer(plain, sys) != plain {
		t.Errorf("changed a clean answer: %q", CleanAnswer(plain, sys))
	}
	decline := "The library does not seem to cover it. Try searching for generator safety."
	if CleanAnswer(decline, sys) != decline {
		t.Errorf("cut a decline: %q", CleanAnswer(decline, sys))
	}
	PromptStyle = "v2"
	v2 := systemPrompt("Someone is bleeding")
	got = CleanAnswer("If someone may be in danger now, start with the most important thing to do, if the sources say it.\nThe most important thing is to press on the wound [S1]. Then give the key steps or facts as a short list, most important first, at most 6 points.\n1. Press hard [S1].", v2)
	if strings.Contains(got, "if the sources say it") || strings.Contains(got, "at most 6 points") || !strings.Contains(got, "press on the wound [S1]") || !strings.Contains(got, "1. Press hard [S1].") {
		t.Errorf("v2 echo:\n%s", got)
	}
}

func TestCleanAnswerDropsReasoning(t *testing.T) {
	got := CleanAnswer("<think>\nOkay, the user asks about bleeding. S1 says press.\n</think>\n\nPress hard on the wound [S1].", "instructions")
	if got != "Press hard on the wound [S1]." {
		t.Errorf("got %q", got)
	}
}

func TestOnlyNotesAndListMarkers(t *testing.T) {
	src := []Source{{Label: "S1", Title: "Outpost note: Frostbite", Note: "x", Text: "Rewarm in warm water."}, {Label: "S2", Title: "Frostbite", Text: "Frostbite is..."}}
	if n := OnlyNotes(src); len(n) != 1 || n[0].Label != "S1" {
		t.Errorf("OnlyNotes: %+v", n)
	}
	if n := OnlyNotes(src[1:]); len(n) != 1 || n[0].Label != "S2" {
		t.Errorf("OnlyNotes without notes: %+v", n)
	}
	if u := UnsupportedNumbers("[1] Do not rub them [S1].\n[2] Rewarm them [S1].", src); len(u) != 0 {
		t.Errorf("list markers counted as numbers: %v", u)
	}
}
