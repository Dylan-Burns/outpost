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

func TestMoreNotes(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	want := map[string]string{
		"a stray dog bit my son and broke the skin":                      "Animal bites",
		"my brother overdosed on fentanyl and his lips are blue":         "Opioid overdose",
		"my friend is really drunk and passed out, can we let her sleep": "Alcohol poisoning",
		"after the crash he is pale and clammy with a weak pulse":        "Shock",
		"rolled my ankle on the trail and it's swelling":                 "Sprains",
		"i mixed bleach and vinegar cleaning the shower and now i cough": "Fumes from mixed cleaners",
		"we are lost in the woods and its getting dark":                  "Lost outdoors",
		"my toddler has had diarrhea and vomiting since last night":      "dehydration",
		"heat wave and no ac, is a fan enough for my dad":                "heat waves",
		"wildfire smoke outside and i have asthma":                       "Wildfire smoke",
		"knocked out a tooth playing hockey":                             "Knocked-out tooth",
		"got drain cleaner splashed in my eye":                           "Eye injuries",
		"hurricane is coming tonight should we stay":                     "Hurricane",
		"my friend is having a panic attack":                             "Panic attack",
	}
	for q, title := range want {
		found := false
		for _, n := range Notes(q) {
			if strings.Contains(n.Title, title) {
				found = true
			}
		}
		if !found {
			t.Errorf("%q: notes %v, topics %v; want %q", q, titles(Notes(q)), EmergencyTopics(q), title)
		}
	}
	for _, q := range []string{"how do i set up a smoke alarm", "i lost my keys in the forest", "how to make a fire without smoke", "my dad got an electric shock from the toaster"} {
		for _, n := range Notes(q) {
			if strings.Contains(n.Title, "Wildfire smoke") || strings.Contains(n.Title, "Lost outdoors") || strings.Contains(n.Title, "Shock after") {
				t.Errorf("%q got %s", q, n.Title)
			}
		}
	}
}

func titles(n []Source) []string {
	var out []string
	for _, x := range n {
		out = append(out, x.Title)
	}
	return out
}

func TestThirdSetGaps(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	want := map[string]string{
		"my 4yr old pushed a bead up his nose and its stuck":                                 "nose or ear",
		"been working outside all day headache dizzy stopped sweating now hes confused":      "Heat stroke",
		"found a guy soaked and shivering by the river now hes stopped shivering and sleepy": "Hypothermia",
		"my kid has a fever and a rash that doesnt go away when i press a glass on it":       "Rash that does not fade",
		"my teenager took a bunch of pills i dont know what they were shes drowsy":           "Swallowed poison",
		"my dad is 76 and suddenly confused today not making sense":                          "Sudden confusion",
		"stepped on a rusty nail went in deep do i need a tetnus shot":                       "tetanus",
		"toilet dont work no water how do we deal with poop for a family":                    "toilets do not work",
		"found white mushrooms with gills and a ring on the stem safe to eat?":               "Wild mushrooms",
		"no heat in a winter storm how do i keep the family warm":                            "No heating in winter",
		"can i take ibuprofen with my blood pressure pills":                                  "Mixing, old",
		"antibiotics expired 2 yrs ago ok to take them for my tooth":                         "Mixing, old",
		"dad hit his head on a cabinet, he takes blood thinners but seems fine":              "Head injury",
	}
	for q, title := range want {
		found := false
		for _, n := range Notes(q) {
			if strings.Contains(n.Title, title) {
				found = true
			}
		}
		if !found {
			t.Errorf("%q: notes %v, topics %v; want %q", q, titles(Notes(q)), EmergencyTopics(q), title)
		}
	}
	if n := Notes("dad hit his head, he takes blood thinners"); len(n) == 0 || !strings.Contains(n[0].Text, "blood-thinning") {
		t.Errorf("head injury note lacks blood thinners: %v", titles(n))
	}
	for _, q := range []string{"how much ibuprofen can i give my 4 year old", "what toilet paper is best", "my rash is itchy after hiking"} {
		for _, n := range Notes(q) {
			if strings.Contains(n.Title, "Mixing, old") || strings.Contains(n.Title, "toilets") || strings.Contains(n.Title, "does not fade") {
				t.Errorf("%q got %s", q, n.Title)
			}
		}
	}
}

func TestFourthSetGaps(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	want := map[string]string{
		"our dog fought a raccoon and is bleeding from the face, what do we do":          "Animal bites",
		"can i use a splash of gas to get the burn pile going":                           "Starting a fire safely",
		"my son went under in the pool for a few seconds, coughed a lot, seems fine now": "Drowning",
		"grandma is 81 and fell, her leg looks shorter and she cant stand":               "Broken bones",
	}
	for q, title := range want {
		found := false
		for _, n := range Notes(q) {
			if strings.Contains(n.Title, title) {
				found = true
			}
		}
		if !found {
			t.Errorf("%q: notes %v, topics %v; want %q", q, titles(Notes(q)), EmergencyTopics(q), title)
		}
	}
	for _, q := range []string{"i smell gas near the stove", "how do i light my gas stove when the power is out"} {
		for _, n := range Notes(q) {
			if strings.Contains(n.Title, "Starting a fire") {
				t.Errorf("%q got %s", q, n.Title)
			}
		}
	}
}

func TestFifthRoundNotes(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	want := map[string]string{
		"my 15 yr old just told me she wants to die and no one would miss her":                  "suicide",
		"my son is having a fit on the floor jerking been about 2 min":                          "Seizure",
		"im 33 wks pregnant and got a really bad headache wont go away, seeing flashing lights": "Warning signs in pregnancy",
		"on warfarin for my afib, got a bad back, ok to take advil or aleve":                    "Mixing, old",
		"broke out in hives all over how many benadryl can i take im 190 lbs":                   "doses for adults",
		"can i store emergency water in old milk jugs washed out":                               "Storing drinking water",
		"got a 100ah 12v battery and an inverter how long will it run a 60 watt fridge":         "Batteries",
		"when will the power come back on on my street, the utility wont answer":                "Power cut",
	}
	for q, title := range want {
		found := false
		for _, n := range Notes(q) {
			if strings.Contains(n.Title, title) {
				found = true
			}
		}
		if !found {
			t.Errorf("%q: notes %v, topics %v; want %q", q, titles(Notes(q)), EmergencyTopics(q), title)
		}
	}
	if n := Notes("on warfarin, ok to take advil"); len(n) == 0 || !strings.Contains(n[0].Text, "blood thinner") {
		t.Errorf("medicine note lacks blood thinners: %v", titles(n))
	}
	for _, q := range []string{"will these jeans fit me", "how do i keep fit off grid", "how much water do chickens need", "how many tablets of yeast for bread"} {
		for _, n := range Notes(q) {
			if strings.Contains(n.Title, "Seizure") || strings.Contains(n.Title, "doses for adults") || strings.Contains(n.Title, "suicide") {
				t.Errorf("%q got %s", q, n.Title)
			}
		}
	}
}

func TestAdultDoseGenericWords(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	for _, q := range []string{"how many pills can i take for a bad headache", "whats the max dose of advil"} {
		ok := false
		for _, n := range Notes(q) {
			ok = ok || strings.Contains(n.Title, "doses for adults")
		}
		if !ok {
			t.Errorf("%q: notes %v", q, titles(Notes(q)))
		}
	}
}

func TestPoolAndHeartFailureNotes(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	for q, title := range map[string]string{
		"pulled my 5yo out of the pool he was under a few seconds, coughed a lot now acting normal, can i just let him play": "Drowning",
		"husband has heart failure, ankles way more swollen and he gained like 3 pounds since yesterday":                     "Heart failure",
	} {
		ok := false
		for _, n := range Notes(q) {
			ok = ok || strings.Contains(n.Title, title)
		}
		if !ok {
			t.Errorf("%q: notes %v", q, titles(Notes(q)))
		}
	}
}

func TestSixthRoundWording(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	has := func(q, title string) bool {
		for _, n := range Notes(q) {
			if strings.Contains(n.Title, title) {
				return true
			}
		}
		return false
	}
	for q, title := range map[string]string{
		"nan's face has gone droopy on one side and her words are all slurred":               "Stroke",
		"my baby is 9 weeks old and feels really hot, thermometer says 38.4":                 "Fever in a young baby",
		"my mate took like 20 paracetamol 2 hours ago, she says shes fine now":               "Swallowed poison",
		"i dont see the point anymore, ive been saving up my tablets":                        "suicide",
		"hubby is type 1, gone all shaky sweaty and confused":                                "Low blood sugar",
		"dad had a water infection, now hes confused, shivering, skin looks blotchy":         "Sepsis",
		"toddler has a temp and some purple dots that dont go when i press a glass on them":  "Rash that does not fade",
		"husband got a shock off a frayed lead":                                              "Electric shock",
		"how long will my chest freezer keep stuff frozen if i keep the lid shut":            "Food after a power cut",
		"water coming up the road, its only like a foot deep i can get through in the truck": "Flood",
		"smoke getting really thick, fire is maybe 5 miles off, nobody has told us to leave": "Wildfire",
		"pipes have frozen under the sink, can i thaw em with a blowtorch":                   "Frozen pipes",
		"water's been off for days how do we deal with the toilet":                           "toilets do not work",
		"phone signal and power both down how do i find out whats going on with the storm":   "Getting news",
		"lead acid batteries on my little solar setup, how low can i let them go":            "Batteries",
		"everything is damp after the rain how do i get a fire going for warmth":             "Starting a fire",
		"axe is blunt as anything, no grinder, how do i sharpen it by hand":                  "Sharpening",
		"whats my ex sarah jenkins new address she moved to leeds last month":                "Finding where someone lives",
	} {
		if !has(q, title) {
			t.Errorf("%q: notes %v, topics %v; want %q", q, titles(Notes(q)), EmergencyTopics(q), title)
		}
	}
	for q, bad := range map[string]string{
		"my tomato plants are drooping in the heat":                 "Stroke",
		"how do i fix a slurry pump":                                "Stroke",
		"best swim stroke for beginners":                            "Stroke",
		"purple spots on my tomato leaves":                          "Rash",
		"how do i find someone lost in the woods":                   "Finding where",
		"how do i find north without a compass":                     "Finding where",
		"power is out from the storm, is the food in the fridge ok": "Getting news",
		"the house caught fire what do i do":                        "Starting a fire",
		"she is still holding the baby":                             "Electric shock",
		"how do i make a fire extinguisher work":                    "Starting a fire",
	} {
		if has(q, bad) {
			t.Errorf("%q wrongly got %q: %v", q, bad, titles(Notes(q)))
		}
	}
}

func TestSixthRoundNoOverreach(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	for q, bad := range map[string]string{
		"my friend fell through the ice into the lake and is shivering and slurring words":   "Stroke",
		"hiking in the heat all day my friend stopped sweating and is confused and slurring": "Stroke",
		"my 2 yr old might of swallowed one of those flat button batteries from the remote":  "Batteries",
		"got a 100ah 12v battery and an inverter how long will it run a 60 watt fridge":      "Food after",
		"tornado warning just went off, we're in a mobile home no basement where do we go":   "Getting news",
	} {
		for _, n := range Notes(q) {
			if strings.Contains(n.Title, bad) {
				t.Errorf("%q wrongly got %q", q, n.Title)
			}
		}
	}
}
