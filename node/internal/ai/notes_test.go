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

func TestOlderSetMisses(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	for q, title := range map[string]string{
		"rattlesnake got my leg on a hike, we're 2 hours from the car":                                  "Snakebite",
		"husband 54 has heavy pressure in his chest going down his left arm and hes sweaty":             "Heart attack",
		"my husband sliced his hand open with a knife and it won't stop gushing":                        "Severe bleeding",
		"mom is 78, suddenly cant lift her right arm and words are slurry but she says shes just tired": "Stroke",
		"the only water we have is a muddy pond, how do we make it drinkable":                           "Making water safe",
		"camping at like 8000 ft, creek water is cloudy, how long do i boil it":                         "Making water safe",
		"How do I make oral rehydration solution at home?":                                              "dehydration",
		"I think my shoulder is dislocated":                                                             "Broken bones",
		"found a man unconscious on the floor, he is breathing. what position do i put him in?":         "Unconscious but breathing",
	} {
		ok := false
		for _, n := range Notes(q) {
			ok = ok || strings.Contains(n.Title, title)
		}
		if !ok {
			t.Errorf("%q: notes %v, topics %v; want %q", q, titles(Notes(q)), EmergencyTopics(q), title)
		}
	}
}

func TestSeventhRoundWording(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	for q, title := range map[string]string{
		"baby had runny poos all day hardly any wet nappies today, still feeding a bit but sleepy":                      "dehydration",
		"pulled my nephew out the pool he coughed loads and hes breathing now, seems fine":                              "Drowning",
		"knocked the kettle and it went on my 2 yr olds arm its gone red and blistery":                                  "Burns",
		"heart going mad cant get a breath hands tingly feel like im dying, had this before n the doc said its anxiety": "Panic",
		"my 2 yr old just had a fit shook all over for like a minute shes really hot":                                   "Seizure",
		"kid has a cough like a barking seal and makes a whistly noise breathing in":                                    "Croup",
		"my girl fell off the trampoline and her wrist is bent funny and swelling":                                      "Broken bones",
		"cut my finger gardening few days ago now its hot and swollen and theres a red line creeping up my arm":         "infected",
		"ground shaking!! do i run outside or stand in the doorway":                                                     "Earthquake",
		"freezers been off a day, stuff still has ice crystals in it can i just refreeze it":                            "Food after",
		"charging the old car batteries for my cabin in the shed overnight":                                             "Batteries",
		"whats sarah mitchells phone number at 14 oak lane and does she live on her own":                                "Finding where",
	} {
		ok := false
		for _, n := range Notes(q) {
			ok = ok || strings.Contains(n.Title, title)
		}
		if !ok {
			t.Errorf("%q: notes %v, topics %v; want %q", q, titles(Notes(q)), EmergencyTopics(q), title)
		}
	}
	if n := Notes("baby had runny poos all day hardly any wet nappies today, still feeding a bit but sleepy"); len(n) > 0 && strings.Contains(titles(n)[0], "Hypothermia") {
		t.Errorf("wet nappies read as cold and wet: %v", titles(n))
	}
}

func TestScaldIsNotDrinkingWater(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	n := titles(Notes("my little girl pulled the kettle and boiling water went on her arm"))
	if len(n) != 1 || !strings.Contains(n[0], "Burns") {
		t.Errorf("scald notes: %v", n)
	}
	if n := titles(Notes("how long do i boil water to make it safe to drink")); len(n) == 0 || !strings.Contains(n[0], "safe to drink") {
		t.Errorf("boiling drinking water: %v", n)
	}
}

func TestEighthRoundWording(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	for q, title := range map[string]string{
		"my 2 yr old drank kerosene from the bottle by mistake, coughing a lot, should i make him vomit with salt water": "Swallowed poison",
		"storm knocked a power line onto our car in the driveway and my husband is still inside it":                      "Power line on a car",
		"eating steak alone and somethings stuck i can barely breath no one here wat do i do":                            "Choking (adult",
		"my husbands still holding the hedge trimmer cable and twitching he got shocked":                                 "Electric shock",
		"think my 3 yr old got into the calpol and drank half the bottle maybe more":                                     "Swallowed poison",
		"kid fell off the trampoline banged his head, hes thrown up twice now":                                           "Head injury",
		"my mate texted saying hes going to end it tonight and hes got his dads pills":                                   "suicide",
		"just had a big shake, we're on 9th floor in delhi, should we run down the stairs":                               "Earthquake",
		"what should go in a go bag for my mum shes on insulin and blood thinners":                                       "go bag",
	} {
		ok := false
		for _, n := range Notes(q) {
			ok = ok || strings.Contains(n.Title, title)
		}
		if !ok {
			t.Errorf("%q: notes %v, topics %v; want %q", q, titles(Notes(q)), EmergencyTopics(q), title)
		}
	}
	for _, n := range Notes("my 2 yr old drank kerosene from the bottle by mistake, should i make him vomit") {
		if strings.Contains(n.Title, "dehydration") {
			t.Errorf("kerosene got %s", n.Title)
		}
	}
}

func TestPowerLineOnCarHasNoShockNote(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	n := titles(Notes("storm knocked a power line onto our car and my husband is still inside it"))
	if len(n) != 1 || !strings.Contains(n[0], "Power line on a car") {
		t.Errorf("notes %v", n)
	}
}

func TestNinthRoundWording(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	for q, title := range map[string]string{
		"husband sweating gone grey colour says his jaw and left arm ache, he reckons its just indigestion": "Heart attack",
		"splitting wood axe slipped, big toe nearly off":                                                    "Severe bleeding",
		"puked like 6 times since last night now my wee is dark brown n i feel dizzy standing up":           "dehydration",
		"fell off the trampoline forearm looks bent like an S":                                              "Broken bones",
		"had 2 glasses of wine and took my sleeping tablet, feel weird is that bad":                         "Mixing",
		"3 day old looks yellow even the whites of her eyes, really sleepy":                                 "jaundice",
		"wifes waters broke 37 wk baby coming quick she wants to push were snowed in":                       "baby arriving",
		"rivers coming up over the road at the bottom of the lane do we stay or go, got 2 dogs":             "Flood",
		"can i run the genny in the garage w the door open its lashing outside":                             "Generator",
		"loadshedding stage 6 and my mom is on an oxygen concentrator at home, what do we do":               "Power cut",
		"tsunami siren going and you can see the sea going way way out":                                     "Tsunami",
		"digging a long drop, how far does it need to be from the well":                                     "toilets do not work",
	} {
		ok := false
		for _, n := range Notes(q) {
			ok = ok || strings.Contains(n.Title, title)
		}
		if !ok {
			t.Errorf("%q: notes %v, topics %v; want %q", q, titles(Notes(q)), EmergencyTopics(q), title)
		}
	}
}

func TestNinthRoundNoOverreach(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	for q, bad := range map[string]string{
		"flood water gone down, going back in the house tomorrow, carpets soaked, the fuse box got wet":   "baby arriving",
		"my son came off his bike n his wrist looks bent funny like a dinner fork":                        "Severe bleeding",
		"son banged his head at the park, knocked out for a few seconds, now really sleepy and been sick": "dehydration",
	} {
		for _, n := range Notes(q) {
			if strings.Contains(n.Title, bad) {
				t.Errorf("%q wrongly got %q", q, n.Title)
			}
		}
	}
}

func TestTenthRoundWording(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	for q, title := range map[string]string{
		"mates been on the cans all night now hes passed out snoring weird and wont wake, we just leave him to sleep it off yeah?": "Alcohol",
		"son got hit in the head at rugby, now hes throwing up and keeps asking the same thing over and over":                      "Head injury",
		"accidentally took my night pills twice, its the blood pressure ones":                                                      "Swallowed poison",
		"toddler was holding a button battery from the remote and i cant find the other one":                                       "Swallowed poison",
		"how do i get the woodstove going when everythings damp and theres no newspaper":                                           "Starting a fire",
		"jellyfish got me on the leg at the beach, someone said pee on it":                                                         "Jellyfish",
		"how do u know if a cut needs stitches, kitchen knife on my thumb gaping a bit":                                            "stitches",
	} {
		ok := false
		for _, n := range Notes(q) {
			ok = ok || strings.Contains(n.Title, title)
		}
		if !ok {
			t.Errorf("%q: notes %v, topics %v; want %q", q, titles(Notes(q)), EmergencyTopics(q), title)
		}
	}
	for q, bad := range map[string]string{
		"mates been on the cans all night now hes passed out snoring weird and wont wake":                     "Fainting",
		"son got hit in the head at rugby, now hes throwing up and keeps asking the same thing over and over": "dehydration",
		"what size button battery does a car key fob take":                                                    "Swallowed",
		"how do i light the gas stove when the igniter is broken":                                             "Starting a fire",
	} {
		for _, n := range Notes(q) {
			if strings.Contains(n.Title, bad) {
				t.Errorf("%q wrongly got %q", q, n.Title)
			}
		}
	}
}

func TestTenthRoundNoOverreach(t *testing.T) {
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
	for q, bad := range map[string]string{
		"how do I get a bee stinger out of my arm":                                               "Jellyfish",
		"cut my hand open on a broken glass in the palm its pumping out blood and wont stop":     "Broken bones",
		"sliced my palm open on a broken jar quite deep, blood keeps soaking thru the tea towel": "stitches",
	} {
		if has(q, bad) {
			t.Errorf("%q wrongly got %q: %v", q, bad, titles(Notes(q)))
		}
	}
	if q := "cut my hand open on a broken glass in the palm its pumping out blood and wont stop"; !has(q, "Severe bleeding") {
		t.Errorf("%q: %v", q, titles(Notes(q)))
	}
	if q := "my teen cut her wrist on purpose not deep but its bleeding, she doesnt want me to tell anyone"; !has(q, "suicide") {
		t.Errorf("%q: %v", q, titles(Notes(q)))
	}
}

func TestOverdoseNoteComesAlone(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	n := titles(Notes("my roommate took something at a party and now his lips are blue and hes snoring weird and i cant wake him"))
	if len(n) == 0 || !strings.Contains(n[0], "Opioid") {
		t.Errorf("notes %v", n)
	}
}
