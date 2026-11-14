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
			map[string]any{"temperature": 0, "num_ctx": 2048, "num_predict": 48},
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
	{rx(`swallow|ingest|\bate\b|drank|drunk|chew|overdos|took too many`, `pill|tablet|medic|bleach|chemical|cleaner|detergent|poison|berr|mushroom|batter|antifreeze|fuel|petrol|gasoline|paracetamol|tylenol|aspirin|iron`), []string{"Poisoning", "Drug overdose"}, nil},
	{rx(`generator|engine|stove|heater|grill|barbecue|charcoal|furnace|boiler|fire`, `headache|dizz|nause|vomit|drowsy|sleepy|confus|faint|collaps`), []string{"Carbon monoxide poisoning"}, nil},
	{rx(`carbon monoxide`), []string{"Carbon monoxide poisoning"}, nil},
	{rx(`diabet|insulin|sugar`, `shak|sweat|confus|dizz|faint|pale|drowsy|unconscious|not making sense|weird`), []string{"Hypoglycemia"}, nil},
	{rx(`bitten|bite|bit\b|bit me|bit my`, `snake|viper|rattle|adder|cobra`), []string{"Snakebite"}, nil},
	{rx(`bitten|bite|bit\b|bit me|bit my`, `dog|cat|animal|fox|bat|raccoon|monkey`), []string{"Dog bite", "Rabies"}, nil},
	{rx(`bee|wasp|hornet|peanut|nut|shellfish|allerg|sting`, `swell|throat|breath|hives|wheez|face`), []string{"Anaphylaxis"}, nil},
	{rx(`heart attack|chest pain|chest pressure|crushing chest|pain in (my|his|her|their) chest`), []string{"Myocardial infarction", "Chest pain"}, nil},
	{rx(`chok|something stuck in (his|her|their|my) throat|can.t breathe or (talk|speak)`), []string{"Choking"}, nil},
	{rx(`stroke|face (is )?droop|slurred speech|one side (of|is) (the |his |her |their )?(face|body)`), []string{"Stroke"}, rx(`(heat|sun) ?stroke`)},
	{rx(`seiz|convuls|epilep`), []string{"Seizure", "Epilepsy"}, nil},
	{rx(`drown|pulled .* (from|out of) (the )?water|under ?water`), []string{"Drowning"}, nil},
	{rx(`bleed|blood (is )?(spurt|pour|everywhere)|deep cut|gash|laceration`), []string{"Bleeding", "Hemorrhage", "Tourniquet"}, nil},
	{rx(`broke|broken|fractur|snapped|crack`, `bone|arm|leg|ankle|wrist|hip|rib|collarbone|finger|toe|foot|hand|elbow|knee|shoulder|jaw|nose|skull`), []string{"Bone fracture", "Splint (medicine)"}, rx(`broke(n)? (the |my |his |her |their )?skin|skin (was |is )?broken`)},
	{rx(`bent (the )?wrong way|wrong angle|out of place|deformed|crooked|sticking out`, `arm|leg|ankle|wrist|finger|toe|elbow|knee|shoulder|hip|bone`), []string{"Bone fracture", "Joint dislocation"}, nil},
	{rx(`burn|scald`), []string{"Burn", "Thermal burn"}, nil},
	{rx(`heat ?stroke|sunstroke|overheat|too hot|heat exhaustion`), []string{"Heat stroke", "Heat exhaustion"}, nil},
	{rx(`hypotherm|freezing|so cold|shivering uncontrollab|fell in(to)? (cold|icy) water`), []string{"Hypothermia"}, nil},
	{rx(`frostbit|frost nip|frozen (finger|toe|nose|ear)`), []string{"Frostbite"}, nil},
	{rx(`hit (his|her|their|my) head|head injur|knocked out|concuss|fell on (his|her|their|my) head`), []string{"Head injury", "Concussion"}, nil},
	{rx(`not breathing|no pulse|cpr|unresponsive|cardiac arrest|heart stopped`), []string{"Cardiopulmonary resuscitation", "Cardiac arrest"}, nil},
	{rx(`asthma|wheez|inhaler`), []string{"Asthma"}, nil},
	{rx(`power (is |goes |went |has gone |has been |was )?(out|cut|off|down)|outage|blackout|no electricity|power cut`, `food|fridge|freezer|refrigerat|meat|milk`), []string{"Food safety", "Food spoilage"}, nil},
	{rx(`tick`, `remove|bite|bitten|attached|stuck|embedded`), []string{"Tick", "Lyme disease"}, nil},
	// Added after the second test set (dev2): the small model named these
	// "bat in bedroom", "thunderstorm injury" and "Generator Safety: ...".
	{rx(`\bbats?\b`, `bedroom|room|house|sleep|slept|woke|bit|bite|scratch|touch|attic`), []string{"Rabies"}, nil},
	{rx(`rabies|rabid`), []string{"Rabies"}, nil},
	{rx(`generator|grill|charcoal|camp ?stove|propane|kerosene|paraffin heater|gas heater|bbq|barbecue`, `inside|indoors|in the (house|home|garage|basement|tent|cabin|kitchen)|garage|basement|tent|window|enclosed`), []string{"Carbon monoxide poisoning"}, nil},
	{rx(`fire|smoke|flames|burning`, `house|home|building|apartment|flat|hallway|room|kitchen|stairs|trapped|escape|stove|pan|oil|grease|fat`), []string{"Smoke inhalation"}, rx(`start (a |the )?fire|fire ?wood|campfire|tinder|kindling|wood ?stove`)},
	{rx(`lightning|thunder`), []string{"Lightning", "Lightning burn"}, nil},
	{rx(`live wire|electrocut|electric(al)? shock|shocked by|power line|downed line`), []string{"Electrical injury"}, nil},
	{rx(`nose ?bleed|bleeding (from (the |his |her |their |my )?)?nose|nose (is |won.t stop )?bleeding|epistaxis`), []string{"Nosebleed"}, nil},
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
	{rx(`fever|temperature of|high temperature|feverish|febrile|\d\d(\.\d)? ?°? ?[CF]\b`), []string{"Fever"}, rx(`yellow fever|scarlet fever|hay fever|rheumatic fever|dengue`)},
	{rx(`mushroom|toadstool|fungus|fungi`), []string{"Mushroom poisoning"}, nil},
	{rx(`rain ?water|rain barrel|cistern`), []string{"Rainwater harvesting"}, nil},
	{rx(`\bcann(ing|ed)\b|water ?bath|pressure can|\bcan (green |the |my |some )?(beans|vegetables|corn|meat|tomato|tomatoes|peas|soup)`), []string{"Canning (food preservation method)", "Botulism"}, nil},
	{rx(`honey`, `baby|infant|month|newborn`), []string{"Botulism"}, nil},
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
