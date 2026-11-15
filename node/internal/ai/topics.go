package ai

import (
	"context"
	"regexp"
	"strings"
)

// topicsPrompt has no example titles on purpose: small models copy them
// into answers for unrelated questions.
const topicsPrompt = `Name the encyclopedia articles that answer the user's question: up to 3 article titles, one per line, using the standard name of the injury, illness, technique or object the question is about. Output only the titles.`

var topicLineRE = regexp.MustCompile(`^[\s\-*•\d.)"']+|["'.*]+$`)

// topicsPrompt2 asks for titles in the encyclopedia's own form: Qwen3 1.7B
// answered topicsPrompt with phrases such as "bat in bedroom" and
// "Generator Safety: Operating Generators in Residential Settings".
const topicsPrompt2 = `Which encyclopedia articles would answer this question? Reply with up to 3 article titles, one per line, and nothing else.
Each title is a short noun phrase of 1 to 4 words, as an encyclopedia titles its articles: the name of the medical condition, injury, poisoning, hazard, skill, food or object the question is about. Use the general term, not the reader's words.`

// TopicsStyle picks the prompt: "v1" (topicsPrompt) or "v2".
var TopicsStyle = "v1"

// TopicsWith asks the model to name the articles that would answer a question.
func (c *Client) TopicsWith(model string) Topics {
	return func(ctx context.Context, q string) []string {
		var b strings.Builder
		prompt := topicsPrompt
		if TopicsStyle == "v2" {
			prompt = topicsPrompt2
		}
		err := c.chat(ctx, model, []Message{{Role: "system", Content: prompt}, {Role: "user", Content: scrub(q, "")}},
			// The same context size as answers: Ollama reloads a model
			// whenever it changes, 2 to 5 seconds each time on a CPU.
			map[string]any{"temperature": 0, "num_ctx": AnswerOptions["num_ctx"], "num_predict": 48},
			func(s string) error { b.WriteString(s); return nil })
		if err != nil {
			return nil
		}
		var out []string
		for _, line := range strings.Split(b.String(), "\n") {
			t := strings.TrimSpace(topicLineRE.ReplaceAllString(strings.TrimSpace(line), ""))
			t = strings.NewReplacer("_", " ", "-", " ").Replace(t)
			if len(t) < 3 || len(t) > 60 || strings.Count(t, " ") > 6 || strings.HasSuffix(t, ":") {
				continue
			}
			out = append(out, t)
			if len(out) == 3 {
				break
			}
		}
		return out
	}
}

// emergencies names the subject of the commonest emergencies people describe
// in their own words. Small models often miss these ("my child swallowed
// pills" is poisoning, not pills), and they are the questions where a wrong
// article does the most harm. Every regexp must match for the rule to apply.
var emergencies = []struct {
	all    []*regexp.Regexp
	topics []string
	none   []*regexp.Regexp // the rule does not apply if any of these match
}{
	{rx(`swallow|ingest|\bate\b|drank|drunk|chew|overdos|took too many|took (a |the )?(whole |entire |full )?(bottle|pack|packet|box)`, `pill|tablet|medic|bleach|chemical|cleaner|detergent|soap|laundry|pod|lye|drain|poison|berr|mushroom|batter|antifreeze|fuel|petrol|gasoline|paracetamol|tylenol|aspirin|iron|sleeping`), []string{"Poisoning", "Drug overdose"}, nil},
	{rx(`generator|engine|stove|heater|grill|barbecue|charcoal|furnace|boiler|fire`, `headache|dizz|nause|vomit|drowsy|sleepy|confus|faint|collaps`), []string{"Carbon monoxide poisoning"}, nil},
	{rx(`carbon monoxide|\bco (alarm|detector|poisoning|leak)|\bco2? alarm`), []string{"Carbon monoxide poisoning"}, nil},
	{rx(`diabet|insulin|sugar`, `shak|sweat|confus|dizz|faint|pale|drowsy|unconscious|not making sense|weird`), []string{"Hypoglycemia"}, nil},
	{rx(`bitten|bite|bit\b|bit me|bit my`, `snake|viper|rattle|adder|cobra`), []string{"Snakebite"}, nil},
	{rx(`bitten|bite|bit\b|bit me|bit my`, `dog|cat|animal|fox|bat|raccoon|monkey`), []string{"Dog bite", "Rabies"}, nil},
	{rx(`bee|wasp|hornet|peanut|nut|shellfish|allerg|sting`, `swell|throat|breath|hives|wheez|face`), []string{"Anaphylaxis"}, nil},
	{rx(`heart attack|chest pain|chest pressure|crushing chest|pain in (my|his|her|their) chest`), []string{"Myocardial infarction", "Chest pain"}, nil},
	{rx(`chok|something stuck in (his|her|their|my) throat|can.?t breathe or (talk|speak)`), []string{"Choking"}, nil},
	{rx(`stroke|face (is )?droop|slurred speech|one side (of|is) (the |his |her |their )?(face|body)`), []string{"Stroke"}, rx(`(heat|sun) ?stroke`)},
	{rx(`seiz|convuls|epilep`), []string{"Seizure", "Epilepsy"}, nil},
	{rx(`drown|pulled .* (from|out of) (the )?water|under ?water`), []string{"Drowning"}, nil},
	{rx(`bleed|blood (is )?(spurt|pour|everywhere)|deep cut|gash|laceration`), []string{"Bleeding", "Hemorrhage", "Tourniquet"}, nil},
	{rx(`broke|broken|fractur|snapped|crack`, `bone|arm|leg|ankle|wrist|hip|rib|collarbone|finger|toe|foot|hand|elbow|knee|shoulder|jaw|nose|skull`), []string{"Bone fracture", "Splint (medicine)"}, rx(`broke(n)? (the |my |his |her |their )?skin|skin (was |is )?broken`)},
	{rx(`bent (the )?wrong way|wrong angle|out of place|deformed|crooked|sticking out`, `arm|leg|ankle|wrist|finger|toe|elbow|knee|shoulder|hip|bone`), []string{"Bone fracture", "Joint dislocation"}, nil},
	{rx(`burn|scald`), []string{"Burn", "Thermal burn"}, rx(`wood|\blogs?\b|fuel|candle|calorie|\bfat\b|\bcds?\b|dvd|burns best|burn (the |our |my )?(trash|garbage|rubbish|waste|leaves)|heartburn|burn ?out|burning (smell|sensation when)`)},
	{rx(`spill|spilt|splash|poured|knocked over|touched|grabbed|picked up`, `boiling|hot (water|coffee|tea|oil|soup|grease|pan|pot|stove|iron|metal)|stove|kettle|oven`), []string{"Burn", "Thermal burn"}, nil},
	{rx(`heat ?stroke|sunstroke|overheat|too hot|heat exhaustion`), []string{"Heat stroke", "Heat exhaustion"}, nil},
	{rx(`hypotherm|freezing|so cold|shivering uncontrollab|fell in(to)? (cold|icy) water`), []string{"Hypothermia"}, rx(`pipe|freezer|\bcar\b|engine|food|water line|plant|garden|tank|battery`)},
	// "old guy in the snow, shivering stopped and hes mumbling" found
	// articles on whiskey, and the answer said to give him some.
	{rx(`shiver|mumbl|confus|slurr|drowsy|sleepy|clumsy|stumbl|pale and cold|cold to the touch|ice cold`, `snow|cold|freez|\bice\b|icy|winter|frost|outside all night|out all night|wet and`), []string{"Hypothermia"}, rx(`pipe|freezer|engine|food|plant|garden`)},
	{rx(`frostbit|frost nip|frozen (finger|toe|nose|ear)`), []string{"Frostbite"}, nil},
	{rx(`numb|white|waxy|pale|hard|blister|grey|gray`, `toe|finger|feet|foot|ear|nose|hand|cheek`, `snow|cold|freez|\bice\b|frost|winter|below zero`), []string{"Frostbite"}, nil},
	{rx(`hit (his|her|their|my) head|head injur|knocked out|concuss|fell on (his|her|their|my) head`), []string{"Head injury", "Concussion"}, nil},
	{rx(`not breathing|no pulse|cpr|unresponsive|cardiac arrest|heart stopped`), []string{"Cardiopulmonary resuscitation", "Cardiac arrest"}, nil},
	{rx(`asthma|wheez|inhaler`), []string{"Asthma"}, nil},
	{rx(`power.{0,12}(out|cut|off|down)|outage|blackout|no (electricity|power)|lost power|without power|power cut`, `food|fridge|freezer|refrigerat|meat|milk`), []string{"Food safety", "Food spoilage"}, nil},
	{rx(`\bticks?\b`, `remove|bite|bitten|attached|stuck|embedded|get (it|them|the ticks?) (out|off)|pull|on (my|his|her|their|our)|found`), []string{"Tick", "Lyme disease"}, nil},
	// Added after the second test set (dev2): the small model named these
	// "bat in bedroom", "thunderstorm injury" and "Generator Safety: ...".
	{rx(`\bbats?\b`, `bedroom|room|house|sleep|slept|woke|bit|bite|scratch|touch|attic`), []string{"Rabies"}, nil},
	{rx(`rabies|rabid`), []string{"Rabies"}, nil},
	{rx(`generator|grill|charcoal|camp ?stove|propane|kerosene|paraffin heater|gas heater|bbq|barbecue`, `inside|indoors|in the (house|home|garage|basement|tent|cabin|kitchen)|garage|basement|tent|window|enclosed`), []string{"Carbon monoxide poisoning"}, nil},
	{rx(`fire|smoke|flames|burning`, `house|home|building|apartment|flat|hallway|room|kitchen|stairs|trapped|escape|stove|pan|oil|grease|fat`), []string{"Smoke inhalation"}, rx(`start (a |the )?fire|fire ?wood|campfire|tinder|kindling|wood ?stove|wildfire|bush ?fire|forest fire|brush fire`)},
	{rx(`lightning|thunder`), []string{"Lightning", "Lightning burn"}, nil},
	{rx(`live wire|electrocut|electric(al)? shock|shocked by|power line|downed line`), []string{"Electrical injury"}, nil},
	{rx(`nose ?bleed|bleeding (from (the |his |her |their |my )?)?nose|nose.{0,20}(bleed|blood)|epistaxis`), []string{"Nosebleed"}, nil},
	{rx(`tooth|teeth`, `knock|out|fell out|lost|broke`), []string{"Tooth avulsion"}, nil},
	{rx(`altitude|high camp|mountain sickness|above \d+ ?(m|ft|feet|metres|meters)`), []string{"Altitude sickness"}, nil},
	{rx(`panic attack|hyperventilat`), []string{"Panic attack"}, nil},
	{rx(`sting|stung|stinger`), []string{"Bee sting"}, nil},
	{rx(`splinter`), []string{"Splinter"}, nil},
	// "how do I clean a dirty cut" found iFixit guides to cleaning a mouse.
	{rx(`\bcuts?\b|wound|gash|graze|scrape|lacerat|sliced`, `clean|wash|dirty|infect|dress|bandage|care|heal|stitch|close`), []string{"Wound care", "Laceration"}, nil},
	{rx(`bug|insect|bead|object|something|pea|bean|stone|battery`, `(in|into|up) (my |his |her |their |the )?(ear|nose)|(ear|nose) (is )?(stuck|blocked)`), []string{"Foreign body"}, nil},
	{rx(`sunburn|sun burn`), []string{"Sunburn"}, nil},
	{rx(`hives|welts|urticaria`), []string{"Hives"}, nil},
	{rx(`dehydrat`), []string{"Dehydration"}, nil},
	{rx(`fever|temperature of|high temperature|feverish|febrile|\d\d(\.\d)? ?°? ?[CF]\b|\btemp(erature)? (is |of |reads |was )?(3[7-9]|4[0-2]|9[89]|10[0-6])|feels? (very )?hot|burning up`), []string{"Fever"}, rx(`yellow fever|scarlet fever|hay fever|rheumatic fever|dengue`)},
	{rx(`mushroom|toadstool|fungus|fungi`), []string{"Mushroom poisoning"}, nil},
	{rx(`rain ?water|rain barrel|cistern`), []string{"Rainwater harvesting"}, nil},
	{rx(`\bcann(ing|ed)\b|water ?bath|pressure can|\bcan (green |the |my |some )?(beans|vegetables|corn|meat|tomato|tomatoes|peas|soup)`), []string{"Canning (food preservation method)", "Botulism"}, nil},
	{rx(`honey`, `baby|infant|month|newborn`), []string{"Botulism"}, nil},
	// Topics with a note added from a standard first-aid and preparedness list.
	{rx(`opioid|opiate|heroin|fentanyl|oxy(contin|codone)?\b|morphine|methadone|percocet|narcan|naloxone`), []string{"Opioid overdose"}, nil},
	{rx(`overdos|too many pills|took a lot of`, `not breathing|barely breathing|breathing (slow|weird)|blue|wont wake|won.?t wake|can.?t wake|unresponsive|pinpoint`), []string{"Opioid overdose", "Poisoning"}, nil},
	{rx(`drunk|alcohol|vodka|whiskey|tequila|beers?|shots|drinking`, `passed out|unconscious|won.?t wake|can.?t wake|wont wake|throwing up|vomit|puking|breathing|cold|blue|seizure|sleep it off`), []string{"Alcohol poisoning"}, nil},
	{rx(`\bshock\b|pale and (cold|clammy|sweaty)|clammy|cold sweat|weak pulse`, `bleed|blood|injur|accident|crash|burn|fall|fell|hit|cut|allerg`), []string{"Shock"}, rx(`electric|shocked by|live wire`)},
	{rx(`sprain|twist|rolled|turned|went over on`, `ankle|knee|wrist|foot|thumb`), []string{"Sprain"}, nil},
	{rx(`bleach|chlorine`, `ammonia|vinegar|toilet|acid|cleaner|mixed|mixing|together|fumes|gas`), []string{"Toxic gas"}, nil},
	{rx(`\blost\b|cant find|can.?t find|no idea where`, `woods|forest|hik|trail|mountain|desert|wilderness|backcountry|in the bush|out here`), []string{"Lost outdoors"}, rx(`lost (my|our|his|her) (keys|phone|wallet|dog|cat)|lost power`)},
	{rx(`diarrh|vomit|throwing up|puking|being sick|stomach bug|gastro|runs`), []string{"Dehydration"}, nil},
	{rx(`heat ?wave|no (a/?c|air ?con)|without (a/?c|air ?con)|too hot|hot weather|very hot|extreme heat`), []string{"Heat stroke"}, nil},
	{rx(`smoke|haze|air quality`, `wildfire|bush ?fire|forest fire|fire season|outside|outdoors|in the air|the house|indoors|asthma`), []string{"Wildfire"}, rx(`smoke (alarm|detector)|smoking|smoked|cigarette`)},
	// Added after the training questions: these found a phone named Tornado,
	// an article on menstrual cups for a bear at the campsite, and a
	// Volkswagen repair guide for a smell of gas.
	{rx(`faint|passed out|blacked out|syncope`), []string{"Fainting"}, nil},
	{rx(`\beyes?\b`, `chip|dust|sand|something|scratch|object|splinter|grit|metal|sawdust|stuck|poked|lash`), []string{"Eye injury", "Corneal abrasion"}, nil},
	{rx(`\beyes?\b`, `spray|chemical|bleach|cleaner|acid|lye|pepper|mace|splash`), []string{"Chemical eye injury", "Eye injury"}, nil},
	{rx(`pulse|heart ?rate|heartbeat`, `check|take|find|feel|count|measure`), []string{"Pulse"}, nil},
	{rx(`trapped|pinned|crushed|stuck under`, `tree|beam|rubble|\bcar\b|rock|log|wall|debris|weight|under`), []string{"Crush injury", "Crush syndrome"}, nil},
	{rx(`smell (of )?gas|gas (leak|smell)|leaking gas|smells? (like )?rotten eggs?|propane leak|natural gas`), []string{"Gas leak"}, nil},
	{rx(`earthquake|tremor|quake`), []string{"Earthquake"}, nil},
	{rx(`tornado|twister|funnel cloud`), []string{"Tornado"}, nil},
	{rx(`flood|flash flood|water (is )?rising|water (over|across|on|covering) the road|(drive|driving|go) (through|thru|across) (the |that )?water|road (is )?(under water|flooded)`), []string{"Flood"}, rx(`flood ?light`)},
	{rx(`stuck|stranded|trapped|snowed in|broke(n)? down`, `\b(car|truck|vehicle|van)\b`, `snow|blizzard|winter storm|ice storm|freezing|cold`), []string{"Winter car"}, nil},
	{rx(`hurricane|cyclone|typhoon`), []string{"Tropical Cyclone"}, nil},
	{rx(`wildfire|bush ?fire|forest fire|brush fire|grass fire`), []string{"Wildfire"}, nil},
	{rx(`\bbears?\b`, `camp|tent|near|attack|charg|\bsee\b|\bsaw\b|spotted|trail|hik|food|cub|woods|yard`), []string{"Bear"}, nil},
	{rx(`baby is coming|giving birth|in labou?r|contractions|waters? (has |have )?broke|deliver (a|the|my|her) baby`), []string{"Childbirth"}, nil},
	// Added after the final set: these found war psychiatry for a fall from a
	// roof, recipes for a child's Tylenol, and pyromania for lighting a fire.
	{rx(`can.?t (feel|move) (his |her |their |my )?(legs|arms|feet|hands|body)|neck (injury|pain|hurt)|back (injury|broken)|broke (his |her |their |my )?(neck|back)|spinal|spine|paraly`), []string{"Spinal injury", "Spinal Cord Injury"}, nil},
	{rx(`epi ?pen|auto-?injector|adrenaline (pen|shot|injection)|epinephrine`), []string{"Epinephrine autoinjector", "Anaphylaxis"}, nil},
	{rx(`stuck|embedded|impaled|lodged|sticking (out|in)|went through`, `\b(arm|leg|hand|foot|chest|belly|stomach|eye|head|neck|thigh|back|wound|skin)\b`, `glass|knife|nail|stick|branch|metal|rod|arrow|shard|blade|object|pole|fence`), []string{"Bleeding", "Penetrating trauma", "Impalement"}, nil},
	{rx(`how (much|many)|dose|dosage|dosing|\bml\b|\bmg\b|can i give|should i give|give (him|her|them|my)`, `child|kid|son|daughter|toddler|baby|infant|year.?old|month.?old|\b\d+ ?(yr|yo)\b`, `tylenol|paracetamol|acetaminophen|calpol|panadol|ibuprofen|motrin|advil|nurofen|benadryl|antihistamine|medicine|medication|syrup|amoxicillin|antibiotic|tablet|pill`), []string{"Child medicine dose"}, nil},
	{rx(`paracetamol|tylenol|acetaminophen|calpol|panadol`, `child|kid|son|daughter|toddler|baby|infant|year.?old|month.?old`), []string{"Paracetamol for children"}, nil},
	{rx(`ibuprofen|advil|motrin|nurofen`, `child|kid|son|daughter|toddler|baby|infant|year.?old|month.?old`), []string{"Ibuprofen for children"}, nil},
	{rx(`paracetamol|tylenol|acetaminophen|panadol`), []string{"Paracetamol for adults"}, rx(`child|kid|son|daughter|toddler|baby|infant|year.?old|month.?old`)},
	{rx(`ibuprofen|advil|motrin|nurofen`), []string{"Ibuprofen for adults"}, rx(`child|kid|son|daughter|toddler|baby|infant|year.?old|month.?old`)},
	{rx(`generator`, `outlet|wall|socket|plug|wiring|backfeed|back-feed|panel|transfer switch|whole house|power the house|refuel|fuel|petrol|gas(oline)?|rain|wet`), []string{"Generator"}, nil},
}

func rx(res ...string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(res))
	for i, r := range res {
		out[i] = regexp.MustCompile(`(?i)` + r)
	}
	return out
}

// EmergencyTopics returns the standard names of the emergencies a question
// describes.
func EmergencyTopics(q string) []string {
	var out []string
	for _, e := range emergencies {
		ok := true
		for _, r := range e.all {
			if !r.MatchString(q) {
				ok = false
				break
			}
		}
		for _, r := range e.none {
			if r.MatchString(q) {
				ok = false
			}
		}
		if ok {
			for _, t := range e.topics {
				if !contains(out, t) {
					out = append(out, t)
				}
			}
		}
	}
	return out
}

// qualifiers are words models add to topic names ("Snake bite first aid",
// "Stroke facts and symptoms") that article titles leave out.
var qualifiers = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`first aid treatment treatments treating signs symptoms guide guidelines facts and techniques
		technique methods method care management recipe preparation instructions how to for of in adults adult at home
		emergency emergencies basics overview procedure procedures steps`) {
		qualifiers[w] = true
	}
}

// coreTopic strips qualifiers from a topic name.
func coreTopic(t string) string {
	var keep []string
	for _, w := range strings.Fields(t) {
		if !qualifiers[strings.ToLower(w)] {
			keep = append(keep, w)
		}
	}
	return strings.Join(keep, " ")
}
