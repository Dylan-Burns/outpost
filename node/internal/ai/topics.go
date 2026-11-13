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

// TopicsWith asks the model to name the articles that would answer a question.
func (c *Client) TopicsWith(model string) Topics {
	return func(ctx context.Context, q string) []string {
		var b strings.Builder
		err := c.chat(ctx, model, []Message{{Role: "system", Content: topicsPrompt}, {Role: "user", Content: scrub(q, "")}},
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
	{rx(`burn|scald`), []string{"Burn", "Thermal burn"}, nil},
	{rx(`heat ?stroke|sunstroke|overheat|too hot|heat exhaustion`), []string{"Heat stroke", "Heat exhaustion"}, nil},
	{rx(`hypotherm|freezing|so cold|shivering uncontrollab|fell in(to)? (cold|icy) water`), []string{"Hypothermia"}, nil},
	{rx(`frostbit|frost nip|frozen (finger|toe|nose|ear)`), []string{"Frostbite"}, nil},
	{rx(`hit (his|her|their|my) head|head injur|knocked out|concuss|fell on (his|her|their|my) head`), []string{"Head injury", "Concussion"}, nil},
	{rx(`not breathing|no pulse|cpr|unresponsive|cardiac arrest|heart stopped`), []string{"Cardiopulmonary resuscitation", "Cardiac arrest"}, nil},
	{rx(`asthma|wheez|inhaler`), []string{"Asthma"}, nil},
	{rx(`power (is )?(out|cut|off)|outage|blackout|no electricity`, `food|fridge|freezer|refrigerat|meat|milk`), []string{"Food safety", "Food spoilage"}, nil},
	{rx(`tick`, `remove|bite|bitten|attached|stuck|embedded`), []string{"Tick", "Lyme disease"}, nil},
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
