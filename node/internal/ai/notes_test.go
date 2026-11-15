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
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = false
	if n := Notes("An adult is choking and can't breathe"); len(n) != 0 {
		t.Fatalf("notes while off: %v", n)
	}
	UseNotes = true
	n := Notes("An adult is choking and can't breathe or talk. What do I do?")
	if len(n) != 1 || !strings.Contains(n[0].Title, "Choking") || !strings.Contains(n[0].Text, "abdominal thrusts") || strings.Contains(n[0].Text, "forearm") {
		t.Fatalf("choking: %+v", n)
	}
	n = Notes("my 6 month old baby is choking and can't cry")
	if len(n) != 1 || !strings.Contains(n[0].Text, "chest thrusts") || strings.Contains(n[0].Text, "pull sharply inwards and upwards") {
		t.Fatalf("baby choking: %+v", n)
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
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
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
	libraryOnly(t)
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
	libraryOnly(t)
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

func TestMentionsDose(t *testing.T) {
	for _, a := range []string{"The dosage for amoxicillin by mouth is 500 mg, 3 times a day [S2].", "Give 2 tablets every 4 hours.", "Inject 4 units of insulin.", "Take 10 ml of syrup."} {
		if !MentionsDose(a) {
			t.Errorf("missed %q", a)
		}
	}
	for _, a := range []string{"Cool the burn for 20 minutes [S1].", "Press on the wound [S2][S3].", "Boil the water for 1 minute.", "Call 911."} {
		if MentionsDose(a) {
			t.Errorf("flagged %q", a)
		}
	}
}

func TestTrainingSetRules(t *testing.T) {
	has := func(q, want string) bool {
		for _, g := range EmergencyTopics(q) {
			if g == want {
				return true
			}
		}
		return false
	}
	yes := map[string]string{
		"what do I do if a tornado is coming":                    "Tornado",
		"a bear is near our campsite":                            "Bear",
		"I smell gas in the house":                               "Gas leak",
		"my toes are numb and white after hiking in the snow":    "Frostbite",
		"someone fainted at a family dinner, what should I do":   "Fainting",
		"I spilled boiling coffee on my lap and it's blistering": "Burn",
		"wood chip flew into my eye and it's scratchy":           "Eye injury",
		"my friend took a whole bottle of sleeping pills":        "Poisoning",
		"someone's leg is trapped under a fallen tree":           "Crush injury",
		"a wildfire is approaching our house":                    "Wildfire",
	}
	for q, want := range yes {
		if !has(q, want) {
			t.Errorf("%q: got %v, want %s", q, EmergencyTopics(q), want)
		}
	}
	no := map[string]string{
		"how do I keep pipes from freezing when the heat is out": "Hypothermia",
		"what wood burns best in a fire":                         "Burn",
		"how long do I need to boil water to make it safe":       "Burn",
		"a wildfire is approaching our house":                    "Smoke inhalation",
		"how do I make a bear-proof food bag":                    "",
	}
	for q, not := range no {
		if not != "" && has(q, not) {
			t.Errorf("%q matched %v", q, EmergencyTopics(q))
		}
	}
}

func TestFinalSetGaps(t *testing.T) {
	has := func(q, want string) bool {
		for _, g := range EmergencyTopics(q) {
			if g == want {
				return true
			}
		}
		return false
	}
	for q, want := range map[string]string{
		"someone fell off the roof and can't feel their legs":           "Spinal injury",
		"how do I use an epipen":                                        "Anaphylaxis",
		"there's a big piece of glass stuck in his arm":                 "Bleeding",
		"how many tylenol can I give my 3 year old":                     "Paracetamol for children",
		"how much ibuprofen can an adult take":                          "Ibuprofen for adults",
		"can I plug my generator into a wall outlet to power the house": "Generator",
	} {
		if !has(q, want) {
			t.Errorf("%q: got %v, want %s", q, EmergencyTopics(q), want)
		}
	}
	if has("how many tylenol can I give my 3 year old", "Paracetamol for adults") {
		t.Error("child question got the adult page")
	}
}

func TestIndependentSetGaps(t *testing.T) {
	has := func(q, want string) bool {
		for _, g := range EmergencyTopics(q) {
			if g == want {
				return true
			}
		}
		return false
	}
	for q, want := range map[string]string{
		"my 7 yr old nose wont stop bleeding should i tip his head back":            "Nosebleed",
		"co alarm keeps going off and my head hurts and the kids feel sick":         "Carbon monoxide poisoning",
		"power been out 6 hours is the food in the fridge still ok":                 "Food safety",
		"my 2 month old feels hot temp is 38.4 should i worry or wait till morning": "Fever",
		"found a tick on my kid how do i get it out":                                "Tick",
		"water over the road on my way home looks maybe a foot deep":                "Flood",
		"stuck in my car in a snowstorm should i walk for help or stay put":         "Winter car",
		"my husband cant breathe or talk he is choking":                             "Choking",
	} {
		if !has(q, want) {
			t.Errorf("%q: got %v, want %s", q, EmergencyTopics(q), want)
		}
	}
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	if n := Notes("my 2 month old feels hot temp is 38.4"); len(n) == 0 || !strings.Contains(n[0].Text, "under 3 months") {
		t.Errorf("baby fever note: %+v", n)
	}
	if n := Notes("my 10 year old has a fever of 39"); len(n) != 0 {
		t.Errorf("baby fever note for an older child: %+v", n)
	}
	for _, q := range []string{"how do i check the tick marks on a tape measure", "my power drill is broken"} {
		if g := EmergencyTopics(q); len(g) != 0 {
			t.Errorf("%q matched %v", q, g)
		}
	}
}

func TestBabyCPRAndChildDoseNotes(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	n := Notes("how do you do cpr on a newborn baby")
	if len(n) == 0 || !strings.Contains(n[0].Text, "two fingers") || strings.Contains(n[0].Text, "heel of one hand") {
		t.Errorf("baby CPR: %+v", n)
	}
	n = Notes("my husband collapsed and is not breathing, how do i do cpr")
	if len(n) == 0 || !strings.Contains(n[0].Text, "heel of one hand") || strings.Contains(n[0].Text, "nipple line") {
		t.Errorf("adult CPR: %+v", n)
	}
	n = Notes("how much tylenol can i give my 2 year old she weighs 26 lbs")
	found := false
	for _, x := range n {
		if strings.Contains(x.Title, "children") && !doseRE.MatchString(x.Text) {
			found = true
		}
	}
	if !found {
		t.Errorf("child dose note missing or states a dose: %+v", n)
	}
}

func TestHypothermiaInPlainWords(t *testing.T) {
	for _, q := range []string{"old guy in the snow shivering stopped and hes mumbling, got him in the car", "my son was out all night in the cold and now hes confused and clumsy"} {
		found := false
		for _, g := range EmergencyTopics(q) {
			if g == "Hypothermia" {
				found = true
			}
		}
		if !found {
			t.Errorf("%q: got %v", q, EmergencyTopics(q))
		}
	}
}
