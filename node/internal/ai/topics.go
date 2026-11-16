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
	{rx(`swallow|ingest|\bate\b|drank|drunk some|got into|bit into|bitten into|chewed (on |a |one )?|sucked on|mouthful of|(took|taken|had|given|gave) .{0,30}(twice|double|two doses|extra dose|by mistake|by accident)|double dose|accidentally (took|taken|gave|given)|drunk|chew|overdos|took too many|took (a bunch|a lot|lots|loads|a handful|some) of|took (a |the )?(whole |entire |full )?(bottle|pack|packet|box)|(took|taken|swallowed|had) (like |about |around |maybe )?\d{2,}`, `pill|tablet|medic|calpol|paracetamol|tylenol|ibuprofen|nurofen|syrup|kerosene|paraffin|petrol|gasoline|lamp oil|turps|white spirit|lighter fluid|antifreeze|bleach|chemical|cleaner|detergent|soap|laundry|pod|lye|drain|poison|berr|mushroom|batter|antifreeze|fuel|petrol|gasoline|paracetamol|tylenol|aspirin|iron|sleeping`), []string{"Poisoning", "Drug overdose"}, nil},
	{rx(`generator|genny|genset|engine|stove|heater|grill|barbecue|bbq|charcoal|furnace|boiler|fire`, `headache|threw up|thrown up|dizz|nause|vomit|drowsy|sleepy|confus|faint|collaps`), []string{"Carbon monoxide poisoning"}, nil},
	{rx(`carbon monoxide|\bco (alarm|detector|poisoning|leak)|\bco2? alarm`), []string{"Carbon monoxide poisoning"}, nil},
	{rx(`diabet|insulin|injections? every|sugar|type ?(1|one|i)\b|\bt1d?\b`, `shak|sweat|confus|dizz|faint|pale|drowsy|unconscious|not making sense|weird|clammy|grumpy|trembl|muddled|find (his|her|their|my) words|skipped (lunch|a meal|breakfast)`), []string{"Hypoglycemia"}, nil},
	{rx(`bitten|bite|bit\b|bit me|bit my|got (my|his|her|their|him|me)\b|struck`, `snake|rattler|copperhead|cottonmouth|viper|rattle|adder|cobra`), []string{"Snakebite"}, nil},
	{rx(`bitten|bite|bit\b|bit me|bit my`, `dog|cat|animal|fox|bat|raccoon|monkey`), []string{"Dog bite", "Rabies"}, nil},
	{rx(`bee|wasp|hornet|peanut|nut|shellfish|allerg|sting`, `swell|throat|breath|hives|wheez|face`), []string{"Anaphylaxis"}, nil},
	{rx(`heart attack|chest pain|chest pressure|crushing chest|pain in (my|his|her|their) chest|(pressure|tightness|tight|squeez|heavy|heaviness|crushing|pain|ache).{0,25}chest|chest.{0,40}(left arm|down (his|her|my|their|the) arm|jaw)|(jaw|left arm).{0,30}(ache|aching|pain|hurt|heavy)|(sweat|grey|gray|clammy).{0,60}(jaw|left arm)|(jaw|left arm).{0,60}(sweat|grey|gray|clammy)`), []string{"Myocardial infarction", "Chest pain"}, nil},
	{rx(`chok|something stuck in (his|her|their|my) throat|can.?t breathe or (talk|speak)|grabb(ed|ing) (at )?(his|her|their|my) throat|can.?t (speak|talk|cough|breathe) or (cough|breathe|speak|talk)|(eating|food|grape|sweet|nut|meat|steak|dinner|lunch).{0,60}(went (totally |all )?silent|no sound|can.?t (cry|cough|breathe)|face (going|went|turning) (dark|blue|purple|red))|(stuck|lodged).{0,30}(throat|breath|swallow)|(eating|steak|meat|food|bone|sweet|grape|nut).{0,40}(stuck|can.?t breath|cant breath|barely breath)`), []string{"Choking"}, nil},
	{rx(`stroke|(face|mouth|eye|smile|lip|arm|side).{0,20}droop|droop.{0,20}(face|mouth|eye|smile|lip|arm|side)|slurr|one side (of|is) (the |his |her |their )?(face|body)|(face|mouth|smile).{0,25}(lopsided|fallen|dropped|drooping)|(can.?t|cannot) (lift|raise) (his|her|their|one|my) arm|words (are )?(all )?(jumbled|muddled|garbled)`), []string{"Stroke"}, wx(`slurry (shed|pit|tank|store|lagoon|gas|pump|spreader)|(into|the) slurry\b|(heat|sun) ?stroke|ice\b|shiver|hypotherm|heat|stopped sweating|drunk|alcohol|slurry (pump|pit|tank|wall|mix|store)|swim|golf|tennis|paddl|rowing|brush ?stroke|back ?stroke|breast ?stroke|overdos|on purpose|took .{0,20}(pills|tablets)`)},
	{rx(`seiz|convuls|epilep|jerk(in|ing)? all over|eyes rolled( back)?|shaking on the floor|having a fit|took a fit|a fit on|(had|has|having|took) (a|another) fit|febrile|fitting|jerking|shaking all over`), []string{"Seizure", "Epilepsy"}, wx(`laptop|computer|phone|app\b|screen|chrome|browser|software`)},
	{rx(`drown|(pulled|got|dragged|fished|lifted) .{0,25}(from|out of|out) (the )?(water|pool|lake|river|sea|bath|pond|canal|surf|dam|creek|loch|lough|quarry|ocean|harbour|harbor|reservoir|billabong)|was under (the water|for|a few|a bit)|under ?water|went under|go under|gone under|deep end|went in the (pool|lake|river|water)|fell in(to)? the (pool|lake|river|pond|water)|submerged|swallowed (a lot of )?water`), []string{"Drowning"}, nil},
	{rx(`bleed|blood (is )?(spurt|pour|everywhere)|deep cut|gash|laceration|gush|sliced (his|her|my|their|open)|nearly (off|severed)|pumping|(soaked|soaking|bleeding) (thru|through)|(chopped|cut|sliced) (off|clean off)|came clean off|amputat|severed|slashed (his|her|my|their)`), []string{"Bleeding", "Hemorrhage", "Tourniquet"}, nil},
	{rx(`broke|broken|fractur|snapped|crack`, `bone|arm|leg|ankle|wrist|hip|rib|collarbone|finger|toe|foot|hand|elbow|knee|shoulder|jaw|nose|skull`), []string{"Bone fracture", "Splint (medicine)"}, wx(`broken (glass|jar|bottle|plate|mirror|window|cup|mug|pane)|broke(n)? (the |my |his |her |their )?skin|skin (was |is )?broken`)},
	{rx(`bent (the )?wrong way|wrong angle|bent funny|bent like|(looks|is) bent|funny (angle|shape)|odd angle|weird angle|looks? (wrong|weird|odd|funny|deformed)|out of place|deformed|crooked|sticking out`, `arm|leg|ankle|wrist|finger|toe|elbow|knee|shoulder|hip|bone`), []string{"Bone fracture", "Joint dislocation"}, nil},
	{rx(`burn|scald`), []string{"Burn", "Thermal burn"}, wx(`wood|\blogs?\b|fuel|candle|calorie|\bfat\b|\bcds?\b|dvd|burns best|burn (the |our |my )?(trash|garbage|rubbish|waste|leaves)|heartburn|burn ?out|burning (smell|sensation when)|burning up|burn (it|them|the ticks?) off|burning (like|so bad|bad)|feels? (like it.?s|its) burning|(stomach|tummy|chest|throat|eyes?|pee|wee|feet) (is |are )?burning|burning (stomach|tummy|chest|throat|eyes?|pee|wee|when i pee|feet)|heartburn`)},
	{rx(`spill|spilt|splash|poured|knocked|tipped|pulled|went (on|over|all over)|touched|grabbed|picked up`, `boiling|hot (water|coffee|tea|oil|soup|grease|pan|pot|stove|iron|metal)|stove|kettle|oven`), []string{"Burn", "Thermal burn"}, nil},
	{rx(`heat ?stroke|sunstroke|overheat|too hot|heat exhaustion|stopped sweating|not sweating|working (outside|in the sun) all day|out in the (heat|sun) all day`), []string{"Heat stroke", "Heat exhaustion"}, nil},
	{rx(`hypotherm|freezing|so cold|shivering uncontrollab|fell in(to)? (cold|icy) water`), []string{"Hypothermia"}, wx(`pipe|freezer|\bcar\b|engine|food|water line|plant|garden|tank|battery`)},
	// "old guy in the snow, shivering stopped and hes mumbling" found
	// articles on whiskey, and the answer said to give him some.
	{rx(`shiver|mumbl|confus|slurr|drowsy|sleepy|clumsy|stumbl|pale and cold|cold to the touch|ice cold`, `snow|cold|freez|soaked|drenched|wet|river|lake|creek|rain|\bice\b|icy|winter|frost|outside all night|out all night|wet and`), []string{"Hypothermia"}, wx(`nappy|nappies|diaper|poo|poop|stool|pipe|freezer|engine|food|plant|garden`)},
	{rx(`frostbit|frost nip|frozen (finger|toe|nose|ear)`), []string{"Frostbite"}, nil},
	{rx(`numb|white|waxy|pale|hard|blister|grey|gray`, `toe|finger|feet|foot|ear|nose|hand|cheek`, `snow|cold|freez|\bice\b|frost|winter|below zero|digging .{0,20}out|shovel`), []string{"Frostbite"}, nil},
	{rx(`hit (his|her|their|my) head|(hit|knocked|whacked|smacked|kicked|struck) (in|on) the head|blow to the head|head (knock|bump|clash)|(bang|bump|knock|smack|whack)(ed)? (his|her|their|my) head|head injur|knocked out|concuss|fell on (his|her|their|my) head`), []string{"Head injury", "Concussion"}, nil},
	{rx(`not breathing|not really breathing|gasp(ing|y)|no pulse|cpr|unresponsive|cardiac arrest|heart stopped`), []string{"Cardiopulmonary resuscitation", "Cardiac arrest"}, nil},
	{rx(`asthma|wheez|inhaler`), []string{"Asthma"}, nil},
	{rx(`power.{0,12}(out|cut|off|down)|outage|blackout|no (electricity|power)|lost power|without power|power cut`, `food|fridge|freezer|refrigerat|meat|milk`), []string{"Food safety", "Food spoilage"}, nil},
	{rx(`\bticks?\b`, `remove|bite|bitten|attached|stuck|embedded|get (it|them|the ticks?) (out|off)|pull|on (my|his|her|their|our)|found`), []string{"Tick", "Lyme disease"}, nil},
	// Added after the second test set (dev2): the small model named these
	// "bat in bedroom", "thunderstorm injury" and "Generator Safety: ...".
	{rx(`\bbats?\b`, `bedroom|room|house|sleep|slept|woke|bit|bite|scratch|touch|attic`), []string{"Rabies"}, nil},
	{rx(`rabies|rabid`), []string{"Rabies"}, nil},
	{rx(`generator|grill|charcoal|camp ?stove|propane|kerosene|paraffin heater|gas heater|bbq|barbecue`, `inside|indoors|in the (house|home|garage|basement|tent|cabin|kitchen)|garage|basement|tent|window|enclosed`), []string{"Carbon monoxide poisoning"}, nil},
	{rx(`fire|smoke|flames|burning`, `house|home|building|apartment|flat|hallway|room|kitchen|stairs|trapped|escape|stove|pan|oil|grease|fat`), []string{"Smoke inhalation"}, wx(`start (a |the )?fire|fire ?wood|campfire|tinder|kindling|wood ?stove|wildfire|bush ?fire|forest fire|brush fire`)},
	{rx(`lightning|thunder`), []string{"Lightning", "Lightning burn"}, wx(`lightning (mcqueen|cable|connector|port|charger|adapter|network|fast|quick)|thunder (cat\b|cake|bolts? (movie|film))|thunderbird`)},
	{rx(`live wire|electrocut|electric(al)? shock|shocked by|power line|downed line|(got|had|getting|took) a (bad |big )?shock|(got|been|was|is) shocked|shock (off|from) (a|the)|frayed (lead|wire|cable|cord)|stuck to (the|a) .{0,20}(cable|wire|lead|cord|plug|socket)|(cable|wire|lead|cord).{0,20}(cut through|sliced through|chopped)`), []string{"Electrical injury"}, nil},
	{rx(`nose ?bleed|bleeding (from (the |his |her |their |my )?)?nose|nose.{0,20}(bleed|blood)|epistaxis`), []string{"Nosebleed"}, nil},
	{rx(`tooth|teeth`, `knock|out|fell out|lost|broke`), []string{"Tooth avulsion"}, nil},
	{rx(`altitude|high camp|mountain sickness|above \d+ ?(m|ft|feet|metres|meters)`), []string{"Altitude sickness"}, nil},
	{rx(`panic attack|hyperventilat|(walls|room) (are |were |was )?clos|rush of (terror|panic|fear|dread)|(heart|chest).{0,40}(doc|doctor|gp|hospital) (says|said|told me).{0,25}(fine|nothing wrong|ok)|exam hall|(supermarket|shop|queue|bus|train|crowd).{0,80}(felt like|going to die|gonna die|couldn.?t breathe)|happen(s|ed)? .{0,25}times (a|this) (week|day)|anxiety attack|(it|this|doc|doctor|gp).{0,30}(anxiety|panic)|(feel|feels|felt) like (i.?m|im|i am|she.?s|he.?s) (dying|going to die|gonna die)`), []string{"Panic attack"}, nil},
	{rx(`sting|stung|stinger`), []string{"Bee sting"}, nil},
	{rx(`splinter`), []string{"Splinter"}, nil},
	// "how do I clean a dirty cut" found iFixit guides to cleaning a mouse.
	{rx(`\bcuts?\b|wound|gash|graze|scrape|lacerat|sliced`, `clean|wash|dirty|infect|dress|bandage|care|heal|stitch|close`), []string{"Wound care", "Laceration"}, nil},
	{rx(`bug|insect|bead|object|something|pea|bean|stone|battery`, `(in|into|up) (my |his |her |their |the )?(ear|nose)|(ear|nose) (is )?(stuck|blocked)`), []string{"Foreign body"}, nil},
	{rx(`sunburn|sun burn`), []string{"Sunburn"}, nil},
	{rx(`hives|welts|urticaria`), []string{"Hives"}, nil},
	{rx(`dehydrat|rehydration|\bors\b`), []string{"Dehydration"}, nil},
	{rx(`fever|temperature of|high temperature|feverish|febrile|\d\d(\.\d)? ?°? ?[CF]\b|\btemp(erature)? (is |of |reads |was )?(3[7-9]|4[0-2]|9[89]|10[0-6])|feels? (really |very |so )?hot|burning up|thermometer|\b3[89]\.\d\b|\b10[0-4]\.\d\b`), []string{"Fever"}, wx(`yellow fever|scarlet fever|hay fever|rheumatic fever|dengue`)},
	{rx(`mushroom|toadstool|fungus|fungi`), []string{"Mushroom poisoning"}, nil},
	{rx(`rain ?water|rain barrel|cistern`), []string{"Rainwater harvesting"}, nil},
	{rx(`\bcann(ing|ed)\b|water ?bath|pressure can|\bcan (green |the |my |some )?(beans|vegetables|corn|meat|tomato|tomatoes|peas|soup)`), []string{"Canning (food preservation method)", "Botulism"}, nil},
	{rx(`honey`, `baby|infant|month|newborn`), []string{"Botulism"}, nil},
	// Topics with a note added from a standard first-aid and preparedness list.
	{rx(`power ?lines?|(electric|power|live) (wire|cable|line)s?|wires? (down|came down)`, `(still|stuck|trapped|we.?re|were|im|i.?m|i am|kids?|we are) (in|inside) (the |my |our )?(car|truck|ute|van|vehicle)?|in the (car|truck|ute|van)|(car|truck|ute|van|vehicle)`, `(still )?inside|in the (car|truck|ute|van)|get out|run for it|jump out`), []string{"Power line on vehicle"}, nil},
	{rx(`wood ?stove|log ?burner|wood ?burner|fireplace|chimney|flue|stove ?pipe`, `smok(y|ing|es) (into|back|the (room|house|cabin))|making .{0,20}smoky|smoky|puff(ing|s)? (smoke )?back|won.?t draw|doesn.?t draw|creosote|tar(ry)?\b|black (stuff|liquid)`), []string{"Wood stove smoke"}, wx(`roar|glowing|red.?hot|flames (from|out)|on fire`)},
	{rx(`carbon mon\w*|carbon mono|\bco\b (alarm|detector|monitor)|(alarm|detector).{0,20}\bco\b`), []string{"Carbon monoxide poisoning"}, nil},
	{rx(`mushies|shrooms|toadstools?|fungi|fungus`, `ate|eaten|eating|picked|foraged|safe|spewing|sick|cramps|vomit`), []string{"Mushroom poisoning"}, wx(`magic|trip|grow (my|our)|athlete|toenail|nail fungus`)},
	{rx(`lightning|electrical storm`, `storm|golf|outside|outdoors|hide|shelter|field|hill|boat|lake|tent|camping|struck|hit|open|beach|pitch`), []string{"Lightning"}, wx(`lightning (mcqueen|cable|connector|port|charger)|bolt`)},
	{rx(`pregnan|preggo|months along|weeks? pregnant|wks pregnant|expecting`, `ibuprofen|nurofen|advil|motrin|aspirin|naproxen|aleve|codeine|tramadol|painkillers?|medicines?|tablets|pills|take (anything|something)|cold and flu|antihistamine|antibiotics?`), []string{"Medicines in pregnancy"}, nil},
	{rx(`batter(y|ies)|power ?bank|vape|e.?bike|e.?scooter|hoverboard`, `swell|swollen|swole|bulg|puff(ed|ing|y)|popping out|pushing out|hot to (the )?touch|overheat|smok|hiss|sizzl|burning smell|chemical smell`), []string{"Battery fire risk"}, wx(`swallow|button|car battery (died|dead|flat)`)},
	{rx(`magnets?|magnetic (balls?|beads?)`, `swallow|ate|eaten|gulped|in (his|her|their) mouth|down (his|her|their) throat|missing`), []string{"Poisoning"}, wx(`fridge|stuck to|magnet (fishing|toys? for)`)},
	{rx(`fish ?hook|hook (stuck|caught|went|is) (in|into|through)|fishing hook`), []string{"Fish hook"}, nil},
	{rx(`(nan|gran|grandma|grandad|grandpa|granny|mum|mom|dad|father|mother|elderly|old (man|lady|woman)|\b(7|8|9)\d\b)`, `stopped (eating|drinking)|(not|isn.?t|barely|hardly) (eating|drinking)|off (her|his|their) food|(more|getting|very|so) (confused|muddled|sleepy|drowsy|unsteady)|going downhill|not (him|her)self`), []string{"Older person unwell"}, wx(`baby|pregnan`)},
	{rx(`puffer|inhaler|ventolin|salbutamol|albuterol|blue pump|reliever`, `breath|wheez|blue|tight|empty|run out|ran out|not working|gasp`), []string{"Asthma"}, nil},
	{rx(`testic|testes|scrotum|ball ?sack|(his|my|one of (his|my)|in the|left|right) (balls?|nuts|nads|bollocks|privates).{0,25}(pain|hurt|agony|swollen|swelling|killing|throbb)|(pain|hurt|agony|swollen|swelling) (in|of) (his|my|one of (his|my)|the) (balls?|nuts|nads|groin)|(kicked|kneed|hit) in the (balls|nuts|nads|groin)`), []string{"Testicle pain"}, wx(`dog\b|cat\b|horse|tennis|golf|basket`)},
	{rx(`soft spot|fontanel`, `bulg|swell|raised|puff|tense|sunken|sinking|dip`), []string{"Bulging fontanelle"}, nil},
	{rx(`\b(tearing|ripping|torn)\b`, `pain|back|chest|stomach|belly`), []string{"Aortic dissection"}, wx(`muscle|ligament|acl|hamstring|calf|tendon|nail|skin|paper|clothes|jeans|ripped (jeans|clothes)`)},
	{rx(`belly|tummy|stomach|abdomen|abdominal`, `fell|fall|hit|kicked|handlebar|frame|crash|seatbelt|landed|punched`, `swell|swollen|tight|hard|tender|bruis|pale|vomit|sick|shoulder|blood`), []string{"Belly injury"}, wx(`bug|virus|ate|food|period|pregnan|constipat`)},
	{rx(`chest (is |feels )?(crushing|heavy|on fire)|(coke|cocaine|lines|speed|meth|ice|pills|stimulant).{0,80}(chest|heart (is )?(going|racing|pounding))|heart (is )?(going mental|racing|pounding).{0,60}(chest|coke|lines|cocaine)`), []string{"Myocardial infarction"}, nil},
	{rx(`(sea|ocean|water) (just |has |suddenly |had )?(went|gone|going|pulled|drawn|drew|draining|dropped) (way |right )*(out|back)|fish (flapping|stranded|left) on the (sand|beach|seabed)`), []string{"Tsunami"}, wx(`\btide\b`)},
	{rx(`every ?thing.?s? (is )?shaking|(whole )?(house|building|flat|floor|ground) (is )?shaking|shaking right now`), []string{"Earthquake"}, wx(`shaking all over|eyes|fit\b|jerk|convuls|seiz|shiver|cold|fever|hands\b`)},
	{rx(`(drain cleaner|bleach|caustic|lye|acid|oven cleaner|chemical|paint stripper|cement)`, `(on|onto|splashed|spilt|spilled|poured).{0,30}(skin|arm|hand|leg|face|forearm|foot|feet|chest)|burn(ing|s)?|sting(ing|s)?`), []string{"Burn"}, wx(`eye\b|swallow|drank|ate|fumes|mixed`)},
	{rx(`(hot|roasting|baking|boiling|sweltering|locked) .{0,40}\bcar\b|\bcar\b.{0,60}(roasting|baking|boiling|sweltering|windows up)`, `slumped|unconscious|bright red|not responding|confused|floppy|left in|locked in|collapsed|baby|kid|child|dog`), []string{"Heat stroke"}, nil},
	{rx(`(bug|insect|moth|fly|spider|beetle|earwig|ant|cockroach) (in|inside|stuck in|crawled into|up) (my|his|her|their|the)? ?(kids? |sons? |daughters? )?ear|buzzing (in|inside) (the|my|her|his|their) ear`), []string{"Foreign body"}, nil},
	{rx(`contact( lens)?.{0,40}(stuck|lost|gone up|under (my|the) (eye)?lid|can.?t get (it )?out|cant get (it )?out)`), []string{"Eye injury"}, nil},
	{rx(`stung.{0,50}(swimming|in the (water|sea|ocean|surf)|at the beach|off (the )?(beach|coast|cairns|darwin|townsville|broome))|(whip|ladder|criss.?cross) (lines|marks|welts|stripes)`, `cairns|darwin|townsville|broome|queensland|\bqld\b|top end|tropical|thailand|philippines|malaysia|indonesia|vietnam|borneo|hawaii`), []string{"Box jellyfish"}, nil},
	{rx(`stung.{0,50}(swimming|in the (water|sea|ocean|surf)|at the beach)|(whip|ladder|criss.?cross) (lines|marks|welts|stripes)`), []string{"Jellyfish sting"}, nil},
	{rx(`(took|taken|swallowed|had|ate) (a )?(whole )?(lot|load|bunch|handful|fistful|heap|pile|stack|mouthful)s? of (pills|tablets|meds|paracetamol|panadol|tylenol|calpol|ibuprofen|nurofen|advil|aspirin|sleeping (pills|tablets)|antidepressants)|(way |far |much )?too much (calpol|paracetamol|panadol|tylenol|medicine|syrup|ibuprofen|nurofen|advil|cough (mixture|syrup))|(gave|given|give) (him|her|them|my \w+|the (baby|kid|child)).{0,40}(double|twice|too much|extra|the whole bottle|two doses)|the whole bottle|(took|swallowed|taken).{0,30}(pills|tablets).{0,30}(don.?t know|dont know|idk|no idea) what`), []string{"Poisoning"}, nil},
	{rx(`(took|swallowed|taken).{0,40}(pills|tablets|meds|paracetamol|panadol|tylenol|ibuprofen|nurofen|packet|bottle).{0,100}(dumped|broke up|break.?up|fight|row|argument|wants to die|doesn.?t want to (wake|live)|just wants to sleep|said goodbye)`), []string{"Suicide"}, nil},
	{rx(`(nipped|scratched|bit|bitten|licked).{0,40}(street dog|stray|monkey|bat|fox|raccoon|feral)|(street dog|stray dog|monkey|bat).{0,40}(nipped|scratched|bit|bite|licked)`), []string{"Rabies"}, nil},
	{rx(`(fell|fallen|came down|landed).{0,40}(onto|on) (his|her|their|my) head|(chundered|spewed|vomited|threw up|been sick).{0,60}(drowsy|sleepy|confused)`, `head|ladder|roof|fell|fall|knock`), []string{"Head injury"}, nil},
	{rx(`stopped shivering|paradoxical|(wants|trying|tried) to (take|strip|pull) (off )?(his|her|their) (clothes|jacket|coat|layers)( off)?|says (he.?s|she.?s|they.?re) (boiling|hot|warm).{0,60}(snow|cold|alps|mountain|ice|freezing|winter)`), []string{"Hypothermia"}, nil},
	{rx(`(pulled|knocked|tipped|spilled|spilt|dropped).{0,40}(cup of (tea|coffee)|cuppa|tea|coffee|mug|teapot|kettle|pot of)`), []string{"Burn"}, nil},
	{rx(`pipe ?bomb|make (a |an )?(bomb|explosive|grenade|molotov)|homemade (explosive|bomb)|how (to|do i) make (napalm|thermite|chloroform|ricin|nerve gas|a gun)|knock (someone|somebody|him|her) out with|poison (someone|somebody|him|her|my (husband|wife|boss|neighbour))`), []string{"Weapons refusal"}, wx(`bath ?bomb|bomb(ed)? (my|the) (exam|test)|photo ?bomb|dirty bomb|seed ?bomb|bomb shelter`)},
	{rx(`(dog|cat|horse|pet|vet|animal).?s? (tramadol|meds|medicine|medication|tablets|pills|painkillers?|antibiotics?|gabapentin|prednisolone)|(tramadol|meds|medicine|tablets|pills|painkillers?|antibiotics?) (prescribed )?(for|from) (the|my|our) (dog|cat|horse|pet|vet)|(vet|animal|pet|dog|cat) (medicine|meds|tablets|pills)`), []string{"Medicine safety"}, nil},
	{rx(`ecstasy|ecstacy|\bmdma\b|\bmolly\b|pingers|\bpills? at (a|the) (party|festival|rave)|took something at (a|the) (party|festival|rave|club)`, `jaw|clench|grind|burning up|overheat|too hot|very hot|boiling|chugging|drinking (loads|lots|litres|liters|so much) of water|won.?t stop drinking|confus|fit|heart (racing|pounding)|sweat`), []string{"Stimulant overheating"}, nil},
	{rx(`grain (bin|silo|bunker)|\bsilo\b|manure pit|slurry (pit|tank|store)|septic tank|sewer|cesspit|confined space|(water|storage|fuel|oil|wine|brewing) tank|\bvat\b|down (a|the) well|inside (the|a) (tank|bin|pit|well)`, `went (down|in|into)|gone (down|in)|fell (in|into)|climbed (in|into|down)|not moving|collapsed|passed out|overcome|unconscious|stuck|buried|sinking|went under`), []string{"Confined space"}, nil},
	{rx(`fake|forge|forged|make it look (official|real)|pretend (i was|to be) (ill|sick)|doctor.?s note (saying|that says)|sick note (saying|that says)`, `doctor|sick note|medical|fit note|letter|certificate|prescription|covid|work|school`), []string{"Forged documents"}, wx(`fake (tan|nails|news|plant|blood for halloween)`)},
	{rx(`(would it take|does it take|to|enough to) kill (someone|somebody|him|her|them|a person|my \w+)\b|(toxic|poison(ous)?|deadly|lethal) gas|gas (to|that will) (kill|hurt|knock)|(sort|deal with|get rid of|get back at|hurt|kill|poison) (out )?(my|the|our|a) (neighbou?r|boss|husband|wife|ex|teacher|landlord)`), []string{"Weapons refusal"}, wx(`(smell|leak|alarm|mixed by accident|accidentally mixed|cleaning)|carbon monoxide|exhaust`)},
	{rx(`(took|taken|had|swallowed) (like |about |around |maybe |nearly )?(\d{2,}|a dozen|a dozen or so) (panadol|paracetamol|tylenol|acetaminophen|nurofen|ibuprofen|advil|aspirin|tablets|pills|caps)`), []string{"Poisoning"}, nil},
	{rx(`(storm|thunderstorm|lightning) (is |right |directly )?(overhead|on top of us|above us|right above)|(storm|lightning).{0,40}(lie flat|lie down|crouch|golf course|open field|on the hill)`), []string{"Lightning"}, wx(`lightning (mcqueen|cable|bolt)`)},
	{rx(`bloods? (is |are |keeps )?(spurt|pump|pour|gush|everywhere|squirt)`), []string{"Bleeding"}, wx(`nose|period|blood (test|pressure|sugar)`)},
	{rx(`(grape|food|sweet|nut|popcorn|sausage|meat|lolly|candy|chip).{0,50}(coughin|coughing|gagging|goin red|going red|choking)`), []string{"Choking"}, nil},
	{rx(`pins (n|and|&) needles.{0,60}(heart|breath|dying|chest)|(had|get|got) this before when (i.?m |im )?(stressed|anxious|panicking)|think (i.?m|im) (dying|going to die|gonna die).{0,60}(stress|anxious|before)`), []string{"Panic attack"}, nil},
	{rx(`(ground|house|floor|building|everything) (just )?(started|starts|began) (shaking|rolling|moving|swaying)|(run outside|stand in (a|the) doorway)`), []string{"Earthquake"}, wx(`shaking all over|fit\b|seiz|shiver|fever|hands\b`)},
	{rx(`(make|get) (my|the|him|her|them|someone) .{0,20}(ill|sick|poorly)|without (him|her|them) knowing|spike (his|her|their|someone.?s) (drink|food)|put something in (his|her|their) (drink|food)`), []string{"Weapons refusal"}, wx(`how (do|can) i (tell|know)|stop (getting|making)|salt ?water|make (him|her|them|the (baby|child|kid|dog\b)) (sick|vomit|throw up) (to|with|so)|should i make|swallowed|chewed|\bate\b|drank|throw up|vomit`)},
	{rx(`tourniquet`), []string{"Bleeding"}, nil},
	{rx(`(took|taken|had|given|gave) (my |his |her |their |the )?[a-z]+( [a-z]+)? (tablets? |pills? |meds? |dose )?(twice|two times|double|again by mistake)|accidentally (took|taken|had|gave|given) (my |his |her |their )?[a-z]+ (twice|again|two)`), []string{"Poisoning"}, wx(`(shower|bath|walk|nap|test|exam|photo|class|lesson|call)s? twice`)},
	{rx(`eye of the (storm|hurricane|cyclone|typhoon)|(storm|wind|hurricane|cyclone).{0,30}(suddenly )?(went|gone|goes) (calm|quiet|still|dead calm)`), []string{"Tropical Cyclone"}, nil},
	{rx(`(bee|wasp|hornet) (sting|stung)|stung by (a |some )?(bee|wasp|hornet)|(bee )?stinger (still )?(in|stuck|out)|sting (is )?still in`), []string{"Bee or wasp sting"}, wx(`jelly|sea|beach|swimming|scorpion|nettle|eye\b|cleaner|bleach|chemical`)},
	{rx(`knock(s|ed)? (someone|somebody|him|her|them|people|a person) out|(gas|spray|drug|chemical)s? (that|to|which) (knocks?|puts?) (someone|people|him|her|them) (out|to sleep)|get (back|even) at (my|the|our|a) \w+`), []string{"Weapons refusal"}, wx(`boxing|fight scene|anaesthetic for (my|the) (dog\b|cat\b)`)},
	{rx(`(letterhead|sign it|signed by|forge (a |the )?signature|copy (a |the )?signature)|doctor.?s? note (on|with|from|signed)|sick note (on|with|from|signed)`), []string{"Forged documents"}, nil},
	{rx(`lithium|digoxin|metformin`, `vomit|throwing up|sick|diarrh|the runs|runny|shak|unsteady|wobbly|confus|drowsy|not drinking|dehydrat`), []string{"Medicine toxicity when ill"}, nil},
	{rx(`slurry|manure (pit|shed|store)|silage (pit|clamp)`, `went in|gone in|collapsed|passed out|go in|drag|overcome|fumes|gas|not moving`), []string{"Confined space"}, nil},
	{rx(`(kicked|kneed|punched|hit|butted|trampled|gored|rammed|stomped|landed) .{0,30}(guts|belly|stomach|tummy|abdomen|ribs)`, `worse|faint|dizzy|pale|sick|vomit|swell|hard|blood|pain`), []string{"Belly injury"}, nil},
	{rx(`(can.?t|cant|cannot|unable to) (put|take|stand|bear) (any )?(weight|on it)|(massive|huge|swollen|fat) (and|n) (purple|black|blue)|(ankle|foot|knee|wrist|leg).{0,30}(gone|is|went) (purple|black|blue|massive)`), []string{"Bone fracture"}, nil},
	{rx(`scuba|diving|dived|\bdove\b|dive (yesterday|today|trip)|decompression|the bends`, `joint|ache|achy|tingl|numb|itch|rash|breath|dizzy|fly|flight|tired|pain`), []string{"Decompression sickness"}, wx(`dumpster|stage dive|diving board|swan dive|nose ?dive`)},
	{rx(`pressure (sore|ulcer)|bed ?sores?|(sore|wound|ulcer).{0,40}(bum|bottom|heel|hip|tailbone|sacrum|back).{0,40}(won.?t|wont|not) heal|(wheelchair|bed.?bound|bedridden|in bed all day).{0,60}(sore|wound|red patch|broken skin)`), []string{"Pressure sore"}, nil},
	{rx(`(after|since) (giving )?birth|post.?partum|postnatal|new mum|days? after (the )?(birth|delivery|having (the|my|her) baby)|(had|delivered) (the|my|her) baby`, `bleed|blood|clots?|soak(ing|ed)? (a|through)|pad|haemorrhag|hemorrhag`), []string{"Bleeding after birth"}, nil},
	{rx(`(would|does|will|do i need|to) (it )?take to (just )?(not wake( up)?|never wake( up)?|die|end it|kill (me|myself|him|her)|go to sleep (forever|for good))|(not|never) wake up (again)?|lethal (dose|amount)|fatal (dose|amount)|overdose on purpose|how (much|many) .{0,40}(to die|to kill|is (lethal|fatal|deadly)|would kill)|sleep (forever|and never wake)`), []string{"Suicide"}, wx(`baby|newborn|toddler|dog\b|cat\b`)},
	{rx(`elephant.{0,20}(sat|sitting|sittin|on)|chest.{0,60}left arm|left arm.{0,60}chest`), []string{"Myocardial infarction"}, nil},
	{rx(`needle (on the (ground|floor)|next to|beside|in (his|her|their) arm)|syringe|shooting up|injecting|(slumped|unresponsive|blue lips|lips (are )?blue|barely breathing).{0,80}(needle|syringe|foil|pills|powder)`), []string{"Opioid overdose"}, wx(`diabet|insulin|epi.?pen|vaccine|knitting|sewing`)},
	{rx(`insulin|lantus|levemir|tresiba|toujeo|novorapid|humalog|fiasp|humulin|apidra|abasaglar`, `twice|double|extra|too much|by mistake|accident|again|already (done|had|taken|injected)|forgot (i|id|i.?d) (already|done)`), []string{"Too much insulin"}, wx(`dog\b|cat\b|post.?mortem|show up`)},
	{rx(`self.?harm|(lines|rows|lots|loads) of (little |small |thin |fresh )?cuts|(little|small|thin) cuts (all )?(on|up|over) (her|his|their|my) (thighs?|arms?|wrists?|legs?|stomach)|cuts? .{0,20}on purpose|cutting (her|him|them|my)self|(cuts|scars|burns).{0,60}(hid(e|ing)|trackies|long sleeves|covers? (them|it) up)`), []string{"Self-harm"}, nil},
	{rx(`no point in (anything|life|it all|going on)|gave (away )?(the dog|my (dog|cat|things|stuff))|wrote (some )?(letters|notes|goodbye)|giving (my |all my )?(things|stuff|belongings) away|sorry for everything|tying up loose ends|won.?t be (around|here) (much longer|soon)`), []string{"Suicide"}, nil},
	{rx(`(took|taken|swallowed|had) (a )?(load|lot|bunch|handful|whole (packet|box|strip)|stack)s? of (her|his|their|my)? ?(sertraline|citalopram|fluoxetine|prozac|amitriptyline|mirtazapine|zopiclone|diazepam|quetiapine|antidepressants|sleeping (pills|tablets))`), []string{"Poisoning"}, nil},
	{rx(`(stopped|quit|not) sweating|skin (is |feels )?(hot (and|n) dry|dry (and|n) hot)`, `heat|hot|sun|roof|confus|dizzy|work|outside|phoenix|desert|field`), []string{"Heat stroke"}, nil},
	{rx(`weever|stingray|sting ?ray|stonefish|stone fish|lionfish|sea urchin|catfish (spine|sting)`), []string{"Venomous fish sting"}, nil},
	{rx(`\bcat(egory)? ?[1-5]\b.{0,20}(coming|hurricane|cyclone|typhoon|storm|landfall)|tape (the|my|our) windows`), []string{"Tropical Cyclone"}, nil},
	{rx(`post.?mortem|wouldn.?t show up|won.?t show up|undetectable|untraceable|without (a trace|getting caught|leaving a trace)|get away with`), []string{"Weapons refusal"}, wx(`stain|spill|mould|mold|cleaning`)},
	{rx(`siphon .{0,40}(neighbou?r|someone|their|his|her|other)|(get|break) into (their|his|her|the neighbou?r.?s|next door.?s) (shed|house|garage|car|place)|take (their|his|her|the neighbou?rs?.?) \w+ (while|when)|steal|nick (some|the|their)|hot.?wire|break into|bypass (the|my|our|a) (electric|gas)?.?meter|fiddle (the|my) meter|tap (into )?(the|my|next door.?s|neighbou?r.?s) (power|electric|supply)`), []string{"Illegal request"}, wx(`steal(ing)? (a )?(glance|look|kiss|the show|base)|hotwire (my|our) own|locked out of (my|our) own`)},
	{rx(`ending it|end(ing)? (her|his|their|my) (own )?life|(has|have|got|made) a plan (to|for how to) (die|end it|kill|end (her|his|their|my) life)|(ending it|suicide|kill(ing)? (her|him|them|my)sel|dying).{0,40}(has|have|got) a plan|thinking (about|of) (ending|killing (her|him|them|my)sel|dying|not being here)|doesn.?t want to be (here|alive)( anymore)?|wants? to disappear (for good|forever)`), []string{"Suicide"}, wx(`relationship|marriage|business|contract|lease|subscription|game`)},
	{rx(`(period|periods) (is |are |was )?(late|overdue|missed)|missed (a |her |my )?period|might be pregnant|could be pregnant|pregnancy test`, `pain|cramp|faint|dizzy|bleed|shoulder|collapse`), []string{"Severe belly pain"}, nil},
	{rx(`defrost|thaw|frozen (chicken|meat|turkey|mince|fish)`, `chicken|meat|turkey|mince|beef|pork|fish|sausage|steak|joint`), []string{"Food safety"}, nil},
	{rx(`(look|looking) after .{0,30}forever|(going|gone) away .{0,20}(not going anywhere|for good|forever)`), []string{"Suicide"}, nil},
	{rx(`blood .{0,20}(spurt|pump|pour|gush|squirt)\w*|(spurt|pump|gush|squirt)\w* (out )?blood`), []string{"Bleeding"}, wx(`nose|period|blood (test|pressure|sugar)`)},
	{rx(`(hit|knocked down|run over) by (a|the) (car|truck|lorry|bus|van|motorbike|bike)|(car|bike|motorbike|motorcycle|quad|horse) (accident|crash|smash)|came off (his|her|their|my) (motorbike|horse|quad)`, `neck|back|spine|can.?t feel|cant feel|tingl|numb|trying to (stand|get up|move)|won.?t keep still|head`), []string{"Spinal injury"}, nil},
	{rx(`spik(ed|ing)|(put|slipped|dropped) something in (my|her|his|their|our) (drink|food)|something in my drink`), []string{"Drink spiking"}, nil},
	{rx(`(hits|beats|hurts|chokes|strangles|threatens|threatening) (me|us|my|the kids)|domestic (abuse|violence)|abusive (ex|partner|husband|wife|boyfriend|girlfriend)|(following|follows|stalking|stalks|stalker) (me|her|him)|stalker|restraining order|threat(en)?(ing|s)? to kill (me|us)|(making|made|keeps making) threats|(scared|afraid|frightened) (of|for my life from) (my|him|her)`), []string{"Abuse and threats"}, nil},
	{rx(`child abuse|(being )?groom(ed|ing)|bruises (everywhere|all over).{0,40}(scared|kid|child)|(kid|child|children|son|daughter).{0,40}(bruises|scared of (his|her) (dad|mum|mom|parents))|report (child )?abuse|(man|someone) .{0,30}(taking photos|watching) .{0,20}(kids|children|playground)`), []string{"Child protection"}, nil},
	{rx(`bad trip|tripping (badly|out)|freaking out on (shrooms|acid|lsd|mushrooms|ketamine)|(shrooms|acid|lsd|mushrooms|ket)\b.{0,40}(panick|paranoid|scared|freaking|seeing things)`), []string{"Bad trip"}, nil},
	{rx(`suspicious (package|parcel|bag|device)|parcel bomb|unattended (bag|package|parcel)|(package|parcel|bag) .{0,30}(ticking|wires|leaking powder)`), []string{"Suspicious package"}, nil},
	{rx(`gumm(y|ies)|edibles?|weed|cannabis|\bthc\b|brownie`, `ate|eaten|swallowed|got into|baby|toddler|child|kid|dog|sleepy|wobbly|floppy|drowsy`), []string{"Poisoning"}, wx(`plant|grow|garden|lawn|killer`)},
	// From a stress test of 12 life-threatening situations, 12 phrasings
	// each, by someone who saw only the situation names.
	{rx(`(collapsed|went down|fell down|slumped|dropped|keeled|hit the deck|fell over).{0,70}(not moving|no breath|not breathing|isn.?t breathing|isnt breathing|can.?t see (him|her|them) breathing|cant see (him|her) breathing|chest (isn.?t|isnt|not) moving|not respond\w*|isn.?t respond\w*|isnt respond\w*|nothing|gulping|gasp\w*|snoring gasps)|(gulping|gasping) (thing|every few)|won.?t wake up and (i )?(can.?t|cant) see (him|her|them) breathing|no breath coming`), []string{"Cardiopulmonary resuscitation"}, nil},
	{rx(`(put|had|got|ate) (a |some )?(bit|piece|chunk) of \w+ in (his|her|their) mouth|in (his|her|their) mouth.{0,40}(no noise|not making any (noise|sound)|can.?t breathe|cant breathe|silent)|swallowed a (small )?(toy|piece|bit|button|coin).{0,40}(can.?t|cant) breathe|choke for something|no fit cry|not making any noise`), []string{"Choking"}, nil},
	{rx(`(got|pulled|dragged|bring|brought|fished) (him|her|them|am) out.{0,50}(blue|not breathing|no dey breathe|cough\w*|limp|floppy|grey)|(head|keeps) going under|splashing and can.?t (shout|call)|hasn.?t surfaced|not (come up|surfaced)|swallowed (loads|lots|a lot|heaps) of water|fall inside (the )?(river|water|sea|pool)`), []string{"Drowning"}, nil},
	{rx(`(pain|ache|aching).{0,30}spread\w* (to|down|into) (my|his|her|their) (arm|jaw|back|neck)|chest (is )?(hurt\w*|pain\w*|tight\w*|aching).{0,40}(sweat\w*|spew|sick|vomit\w*|clammy)|holding (his|her|my|their) chest`), []string{"Myocardial infarction"}, nil},
	{rx(`(confused|confusion|not making sense).{0,50}(shiver\w*|breathing (fast|quick)|fast breathing|mottled|blotchy)|(shiver\w*|breathing (fast|quick)).{0,50}(confused|not making sense)`), []string{"Sepsis"}, wx(`cold|freez|outside all night|out all night|hypotherm|snow|wet|soaked|hills|mountain|river|lake|sea|ice\b`)},
	{rx(`needles? (next to|beside|by|near|on the (ground|floor))|pinpoint pupils|snort(ed|ing)? (something|a line|coke|powder).{0,40}(breath|wake|blue)`), []string{"Opioid overdose"}, wx(`diabet|insulin|epi.?pen`)},
	{rx(`went (all )?rigid|thrash\w*|bit (her|his|their) tongue|wet (him|her|them)sel(f|ves)|shaking violently|dey shake|eye(s)? don turn|eyes (rolled|rolling) (back|up)`), []string{"Seizure"}, wx(`shiver|cold|fever|dog\b|cat\b|engine`)},
	{rx(`(mouth|face|smile|lip)s? .{0,20}(crooked|twist\w*|droop\w*|saggy|lopsided|isn.?t working|not working|fallen|dropped)|(blind|lost (her|his|their) (vision|sight)) .{0,15}(in one eye|suddenly|just now)|(vision|sight) .{0,10}suddenly|went blind in one eye|(left|right) (side|arm|leg) (is |feels |went |gone )?(all )?(numb|dead|weak|heavy|floppy)|arm is dead|(can.?t|cant|cannot) (lift|raise|move) (his|her|their|my) (right |left )?arm|one side of (his|her|their) (smile|face)`), []string{"Stroke"}, wx(`chest|elephant|face (dropped|fell) when|when (she|he|they) heard|news|tooth|dentist|botox|filler|after (the )?(gym|workout)`)},
	{rx(`(brought|bring|took|taken|moved|put) (the |our |a )?(braai|bbq|barbecue|grill|generator|genny|gen|heater|camp stove) (in|inside|indoors)|(braai|bbq|barbecue|grill|generator|genny|\bgen\b) (is |was )?(in|inside) (the )?(house|home|kitchen|room|tent|garage)|better when (i|we|they) (leave|go out|get out)`), []string{"Carbon monoxide poisoning"}, nil},
	{rx(`(lips|tongue|face|mouth)s? (are |is |gone |went |look(s|ing)? )?(huge|massive|enormous|so big|blown up|puffed up)|voice (gone|is|going|sounds) (hoarse|croaky|funny|weird)|\bhoarse\b`, `sting|stung|bee|wasp|hornet|ate|eaten|nut|peanut|prawn|shellfish|allerg|medicine|tablet|antibiotic`), []string{"Anaphylaxis"}, nil},
	// From a crisis stress test: 60 indirect ways people say they or
	// someone they know may be thinking of suicide.
	{rx(`(don.?t|dont|do not|doesn.?t|does not) want to (be here|live|exist|wake up|be alive|go on|be around)|want(s)? (it|everything|this|it all) (all )?to (stop|end)|want(s)? the pain to stop|(just )?want(s)? to be gone|wanna be gone|disappear (and|for good|forever)|not come back|what.?s the point (of|in) (carrying on|living|going on|anything|it all|trying)|no point (carrying on|going on|trying)|(thinking|thought|think) about not waking up|not waking up (sounds|would be) (nice|good|better)|(i|to) (just )?(not wake up|never wake up)|(don.?t|dont) want to wake up`), []string{"Suicide"}, wx(`baby|newborn|toddler|dog\b|cat\b|alarm|in the morning|phone|laptop|computer`)},
	{rx(`(gave|giving|givin|given) (away )?(his|her|their|my) (stuff|things|games|xbox|belongings|possessions|prized)|won.?t need (it|them) where|suddenly (calm|peaceful|happy)|(calm|peaceful|happy) .{0,20}(almost peaceful|after months)|(search(ed|ing)?|looking up|looked up|googl\w*|research\w*) .{0,25}(ways|how|methods?) to (die|kill (my|him|her|them)sel|end (my|his|her|their|it) (life|all))`), []string{"Suicide"}, nil},
	{rx(`can.?t do this (no more|anymore|any more)|(i.?m|im|i am) (so )?done( with (it all|everything|life))?\.?$|cant do this no more|done with (it all|everything|life)|can.?t see a way out|cant see a way out|(had|having) enough of (it all|everything|life)|after having enough|cyaan (tek|take) it|tek it nuh more|waan dead|want(s)? (to )?dead\b|burden on|better off (with|without) (me|him|her)|better off if (i|he|she|they) (was|were|wasn.?t|weren.?t)|life insurance`), []string{"Suicide"}, wx(`traffic|work is|homework|exam|job|diet|gym`)},
	{rx(`driv(e|ing) (the |my |his |her )?car into|step(ping)? in front of (a|the) (train|bus|truck|lorry|car)|jump(ing)? (off|from) (a|the) (bridge|cliff|building|roof|car park)|(standing|sitting) on (the|a) (bridge|ledge|edge|railway)|rope in the|noose|hang(ing)? (my|him|her|them)sel|(staring at|in front of me).{0,20}(pills|tablets|bottle)|bottle of pills in front|stockpil\w* (my |his |her )?(meds|pills|tablets)|cleaned (his|her) gun|good innings|ready to go( now)?|look after .{0,30}forever|(nobody|no one|noone|no1) (would|will|wud|d) (miss|care|notice)|(don.?t|dont|can.?t|cant) see myself (making it|being (here|around)|lasting)|goodbye .{0,30}(the last time|for good|forever)|like (it.?s|its|it was) the last time|(wrote|written|writing) (letters|notes) to (each of us|everyone|all of us|the family)|like (he.?s|hes|she.?s|shes|they.?re) saying goodbye|(pills|tablets|took|swallowed|overdos|cut).{0,60}(won.?t|wont) let me call|(swallowed|took|taken|necked) (a bunch|a load|loads|a handful|lots|heaps) of (pills|tablets|meds|paracetamol|panadol|tylenol|sleeping)|see you on the other side|(going away|look after .{0,30}) (forever|for good)|tonight.?s the night|do something stupid|if anything happens to me|not (going to|gonna) stick around|(won.?t|not) (see|make it to) (christmas|my birthday|the weekend|next year)|\bkms\b|\bkys\b|unaliv\w*|topp(ing|ed)? (my|me|him|her)sel(f|ves)|top (my|me)self|off (my|me)self|go bush and not come back|might as well end it`), []string{"Suicide"}, wx(`dog\b|cat\b|ready to go (out|home|now\?)|crisps|paint|tie-?dye`)},
	{rx(`took (all|every|everything|the lot of) .{0,25}(tablets|pills|meds|medicine|in the (cupboard|cabinet|drawer))|overdosed|(drank|swallowed|took|taken|ate) .{0,40}on purpose|took everything (in|from|out of) (the|my|her|his) (cupboard|cabinet|drawer|medicine)|took everything (with|and) (vodka|wine|alcohol|booze|beer)`), []string{"Poisoning", "Suicide"}, nil},
	{rx(`(keep|kept|been|started) cutting( again)?|cut(ting)? (again|too deep)|cuts? on (her|his|their) (arms?|legs?|thighs?|wrists?) .{0,40}(says|said|claims) (it.?s|its|it was) (the cat|an accident|nothing)`), []string{"Self-harm"}, nil},
	{rx(`garage .{0,40}(car|engine) (was |is )?(running|on)|(car|engine) (running|on) in (the|a) garage|hose(pipe)? (from|in|to|on) the exhaust`), []string{"Carbon monoxide poisoning", "Suicide"}, nil},
	{rx(`jump(ed|s)? (off|from) (a|the) (bridge|cliff)`), []string{"Drowning", "Suicide"}, wx(`for fun|dare|swim|summer|holiday`)},
	// From an audit of missing topics against NHS, Red Cross, CDC and
	// FEMA lists.
	{rx(`(run|running|ran|out) (out )?of (my |his |her |their )?(insulin|meds|medication|medicine|tablets|pills|steroids|prednisolone|hydrocortisone|inhaler|epilepsy|warfarin|blood thinners)|(no|can.?t get|cant get|no more|last of (my|his|her)) (insulin|steroids|prednisolone|epilepsy (meds|tablets)|warfarin)|pharmacy (is )?(closed|shut).{0,40}(tablets|meds|insulin|pills)|ration(ing)? (my|his|her)? ?(insulin|tablets|meds)`), []string{"Vital medicine running out"}, nil},
	{rx(`(high|very high) (blood )?sugar|sugars? (is |are |at )?(high|2\d|3\d)|hyperglyc|\bdka\b|ketoacidosis|ketones|fruity breath|pear ?drops?`), []string{"High blood sugar"}, nil},
	{rx(`(child|kid|toddler|son|daughter|boy|girl|\d+ ?(yr|yo|year)s?.?old)`, `grunting|(ribs|chest|skin) (sucking|pulling) in|breathing (really |very )?fast|can.?t stay awake|cant stay awake|won.?t wake|wont wake|floppy|mottled|blotchy|high.?pitched cry|not (drinking|weeing|peeing)|no (wee|pee|wet nappy)|grey|gray|blue lips`), []string{"Seriously ill child"}, wx(`choking|swallowed|drown|pool|hit (his|her) head|fell`)},
	{rx(`(belly|tummy|stomach|abdomen|abdominal|appendix|appendicitis)`, `(severe|really bad|agony|excruciating|unbearable|worst|doubled over|rigid|hard as a board|sudden)|lower right|right side.{0,20}(pain|hurt)|shoulder tip|ectopic|(pregnan|period).{0,40}(pain|bleed)`), []string{"Severe belly pain"}, wx(`fell|fall|hit|kicked|punched|handlebar|frame|seatbelt|period pain|cramps? from|bug|food poison|ate`)},
	{rx(`ectopic|appendicitis|appendix (burst|pain)|burst appendix`), []string{"Severe belly pain"}, nil},
	{rx(`blood clot|\bdvt\b|pulmonary embol|\bpe\b|(calf|leg|thigh) (is |gone )?(swollen|swelling|red|hot|warm|painful).{0,60}(flight|flew|surgery|op|operation|bed|pill|pregnan|plane)|(flight|flew|surgery|operation).{0,60}(calf|leg) .{0,20}(swollen|swelling|red|hot|painful)|coughing (up )?blood`), []string{"Blood clot"}, wx(`nose|vomit`)},
	{rx(`(worst|sudden|thunderclap|exploding|hit by a hammer|out of nowhere).{0,30}headache|headache.{0,40}(worst|sudden|out of nowhere|exploded|like being hit|thunderclap)`), []string{"Thunderclap headache"}, wx(`hangover|screens?|migraine (again|as usual)`)},
	{rx(`withdrawal|withdrawals|detox|(stopped|stop|quit|quitting|ran out of) (drinking|the drink|booze|alcohol|vodka|beer|benzos|valium|diazepam|xanax|sleeping (pills|tablets))|delirium tremens|\bthe dts\b|the shakes? .{0,30}(drink|booze|alcohol)`), []string{"Alcohol withdrawal"}, wx(`nicotine|vap|cigarett|smoking|caffeine|coffee|phone|social media`)},
	{rx(`(fell|fallen|gone|went|broke) (through|thru|into) (the )?ice|through the ice|(ice|frozen) (pond|lake|river|canal|dam).{0,40}(fell|in the water|cracked|went in)`), []string{"Ice rescue"}, nil},
	{rx(`trapped|pinned|stuck under|buried|crushed|under (the )?rubble`, `rubble|collapsed|debris|beam|wall|roof|car|tractor|vehicle|building|earthquake|quake|landslide|mudslide|under`), []string{"Trapped person"}, wx(`trapped (in|inside) (the|my|our) (car\b|house) (by|in) (snow|flood)|snowed in|snow|drift|whiteout|blizzard|trapped wind`)},
	{rx(`stab(bed|bing)?|knifed|shot|gunshot|bullet|been shot|got shot|impaled`), []string{"Stab or gunshot"}, wx(`(tetanus|tetnus|tet|flu|covid|insulin|b12|booster|allergy|vitamin|cortisone|steroid) shots?|(last|his|her|their|my) shots|shots were|shot (glass|of|put)|photo|vaccin|jab|flu shot|booster|screenshot|shots? of|snapshot|insulin shot|shot at (goal|the)|hot ?shot|shot(gun)? cartridge`)},
	{rx(`(vomit|vomiting|throwing up|threw up|sick|puking|spewing)(ed)?.{0,25}(blood|coffee ?grounds?)|(poo|poop|stool|stools|bowel movement)s? (is |are |was |were )?(black|tarry|like tar|bloody|full of blood)|black (tarry )?(poo|poop|stools?)`), []string{"Vomiting blood"}, wx(`nose ?bleed|period|beetroot|iron tablets|blueberries`)},
	{rx(`radiation|radioactive|nuclear|fallout|dirty bomb|potassium iodide|iodine tablets`), []string{"Radiation emergency"}, wx(`purif|water treatment|(treat|clean|disinfect)(ing)? .{0,15}water|radiation (therapy|treatment)|radiotherapy|microwave|phone radiation|wifi radiation|nuclear family|x.?ray|mri|ct scan`)},
	{rx(`chemical (leak|spill|cloud|plant|fire|release)|toxic (cloud|leak|fumes from (the|a) (factory|plant|train))|train derail|tanker (crash|spill|leak)|gas cloud|shelter in place|ammonia leak|chlorine (leak|cloud|gas)`), []string{"Chemical cloud"}, wx(`mixed|mixing|bleach and|cleaners?|toilet`)},
	{rx(`formula|bottle.?feed|breast.?feed|breastfeed|baby milk|feeding (the |my )?(baby|bub|newborn)`, `water|boil|flood|outage|power|emergency|run out|ran out|stretch|dilut|clean|sterili|fridge|storm|shelter|hurricane|evacuat`), []string{"Infant feeding"}, nil},
	{rx(`altitude|\bams\b|high camp|base camp|(above|over|at) \d,?\d{3} ?(m|metres|meters|ft|feet)|(mountain|summit|hiking up|climbing|trek).{0,60}(headache|nause|sick|dizzy|breathless|confused)`), []string{"Altitude sickness"}, wx(`plane|flight|cabin pressure|hit (my|his|her|their) head|head injury|bang(ed)? (my|his|her) head|fell|crash|came off`)},
	{rx(`(tooth|teeth|gum|dental|molar|jaw)`, `abscess|swollen|swelling|puffed|pus|throbb|face (is )?(swollen|swelling|puffy)|cheek (is )?(swollen|swelling|up)`), []string{"Dental abscess"}, wx(`knocked out|fell out|teething|baby teeth|wisdom tooth coming`)},
	{rx(`(ate|eaten|eating|chewed|swallowed|picked|tasted)`, `berr(y|ies)|wild plants?|leaves|roots?|hemlock|foxglove|yew|nightshade|lily|laburnum|daffodil|holly|mistletoe|poison (ivy|oak)`), []string{"Poisonous plants"}, wx(`mushroom|fung|toadstool|recipe|shop|supermarket|blueberries in|strawberr|raspberr|salad`)},
	{rx(`spider|scorpion|funnel.?web|redback|black widow|brown recluse`), []string{"Spider or scorpion"}, wx(`spider.?man|spider plant|spider web decoration|spider (mite|veins)`)},
	{rx(`\brip\b|rip current|rip tide|riptide|undertow|(dragged|pulled|swept) out to sea|(current|water) (is )?(pulling|dragging) (me|him|her|them|us) out`), []string{"Rip current"}, wx(`rip (off|up|out|it|the|my|jeans)|rest in peace|r\.i\.p`)},
	{rx(`(finger|thumb|toe|hand|ear|fingertip|tip of (my|his|her) finger).{0,30}(cut off|chopped off|sliced off|came off|severed|clean off|off completely|amputated)|(cut|chopped|sliced) (off )?(the |my |his |her )?(tip of )?(finger|thumb|toe)s? (off|clean off)`), []string{"Severed body part"}, nil},
	{rx(`(sudden(ly)?|just|all of a sudden).{0,30}(can.?t see|cant see|went blind|lost (my |his |her )?(sight|vision))|curtain (over|across|coming down)|shadow (over|across) (my|his|her) (vision|eye)|new floaters|flashes (and|with) floaters|(red|painful) eye.{0,40}(vomit|headache|blurr)`), []string{"Sudden vision loss"}, nil},
	{rx(`swarm|(lots|loads|dozens|hundreds|heaps|tons) of (bees|wasps|hornets|stings)|stung (all over|loads of times|many times|\d{2,} times)|(bee|wasp|hornet)s? (nest|hive) .{0,30}(attack|chas|stung)`), []string{"Bee swarm"}, wx(`swarm of (people|customers|fans)`)},
	{rx(`active shooter|gunman|shooter|someone (is )?shooting|man with a (knife|gun|machete)|attacker|terror(ist)? attack|people (are )?being stabbed|running at people|rampage|lockdown at (school|work|the mall)`), []string{"Active attacker"}, wx(`shooter (game|games)|first.?person shooter|photo shoot`)},
	// From a second paraphrase test (450, regional voices, teenagers,
	// older people, symptoms described, never named).
	{rx(`water (comin|coming|pouring|rushing|getting) (in|into)|house (is )?filling (up )?(with|up)|(river|creek|burn|water) (has )?(come|coming) up (into|to)|knee.?deep water|drains (are )?backing up`), []string{"Flood"}, nil},
	{rx(`(lips|tongue|throat|face|eyes).{0,25}(puff|swell|swole|thick|tight|closing)|throat (is )?(closing|tight)|(ate|eaten|had|some).{0,60}(nut|peanut|cashew|prawn|shrimp|shellfish|satay|sesame).{0,80}(hives|blotch|swell|puff|tight|wheez|thick)`), []string{"Anaphylaxis"}, wx(`pregnan|preggo|weeks|wks|months along|headache`)},
	{rx(`snorin(g)? (weird|funny|strange|loud)|foil next to|gouch(ed|ing)?|(pain|strong) (tablets|pills|killers|meds).{0,80}(slow breath|very slow|grey|gray|drowsy|blue)|did a line|(blue|grey|gray) (fingertips|lips|fingers).{0,60}(line|pills|took|party|coke)`), []string{"Opioid overdose"}, wx(`cold|snow|swim|water|cans|drunk|drinking|beers?|vodka|booze|pissed|wine|alcohol`)},
	{rx(`whiffs? of gas|(bad|rotten) eggs?|sulph?ur(ous)? smell|pong like the (stove|gas|cooker|hob)|(stove|cooker|hob).?s been left on|smells? like (the )?gas`), []string{"Gas leak"}, wx(`from (my|your) hands|on (my|your) hands|filling up|at the (pump|station)|fridge|food|eggs? (gone off|in the fridge)`)},
	{rx(`(down|gone|went) (down )?the wrong way|(clutching|holding|grabbing) (at )?(his|her|their) (throat|neck)|gobstopper|(goin|going|gone|turning) (purple|blue|red) (and )?(no sound|no noise|silent|can.?t)|(grape|sweet|cheerios?|toy|somethin|something|mash|lolly|nut).{0,50}(no (noise|sound|cry)|not makin(g)? any sound|silent|can.?t cry|cannot cry|no cry)|went dead quiet holding`), []string{"Choking"}, nil},
	{rx(`hot to (the )?touch|hot (n|and) floppy|like a hot water bottle`, `baby|newborn|infant|week|month|bub|wee one`), []string{"Fever"}, nil},
	{rx(`whole (packet|pack|box|bottle|strip) of|opened? .{0,20}(pods|bottle|pills|tablets)|(stuff|liquid|blue|green|powder) (round|around) (her|his|their) mouth|antifreeze|weed ?killer|(with|holding) the (bleach|antifreeze|weedkiller|pills|tablets) bottle`), []string{"Poisoning"}, wx(`buy|recipe`)},
	{rx(`(wife|she|partner|daughter|sister|girlfriend|her).{0,30}(pushin|has to push|needs to push|wants to push|is pushing)|(see|seen) (the|its) head|can see (the )?hair|babys? (is )?coming now|baby.?s coming (now|fast|quick)`), []string{"Emergency birth"}, nil},
	{rx(`\b\d+ ?(wks|weeks)\b.{0,40}(kick|bleed|fluid|swell|headache|pain)|gush of (clear )?fluid|(not|isn.?t|stopped|less) kicking`), []string{"Pregnancy warning signs"}, wx(`old\b|baby is \d+`)},
	{rx(`smile (is |was |went )?(all )?(crooked|lopsided|wonky|droop)|(can.?t|cant|cannot) find (his|her|their|my) words|(hand|arm|leg|side) (has )?(gone|went) (weak|numb|dead|floppy)|dropped (his|her|their) (cup|mug|fork)|mouth (went|is|has gone) (crooked|droop|lopsided)`), []string{"Stroke"}, nil},
	{rx(`face down in (the )?(bath|pool|water|pond)|fell (in|into) the (garden )?pond|slipped off.{0,40}(jetty|boat|dock|pier|wharf)|in the murky water`), []string{"Drowning"}, nil},
	{rx(`touched a (live )?wire|(key|fork|knife|finger|pin)s? (in|into) (a|the) (socket|plug|outlet)|(hit|drilled into|drilling into) a (cable|wire)|drilling into .{0,20}(cable|wire)`), []string{"Electrical injury"}, nil},
	{rx(`(cable|wire|line)s? (has )?(fallen|fell|lying|draped|come down) (across|on|over) (the |our |my )?(bonnet|hood|windscreen|windshield|roof)|wires? on (the |our |my )?car|(slid|skidded|crashed|clipped|hit|ran) (into )?.{0,30}(telegraph pole|power pole|electricity pole|a pole)`), []string{"Power line on vehicle"}, nil},
	{rx(`(doesn.?t|does not|dont|don.?t) see the point|how many (tablets|pills) would do it|searched (for )?how many (pills|tablets)`), []string{"Suicide"}, nil},
	{rx(`went stiff.{0,30}shak|gone (all )?rigid|(arms|legs|limbs).{0,20}(twitch|jerk)|started shakin(g)? on the (ground|floor)`), []string{"Seizure"}, wx(`cold|shiver|fever`)},
	{rx(`(dizzy|sick|headache|collapsed|fainted).{0,50}(in the heat|in this heat|boiling|hot day|in the sun|heatwave)|boiling hot to (the )?touch`), []string{"Heat stroke"}, nil},
	{rx(`fell in(to)? the (loch|lake|river|sea|canal|lough|reservoir|dam)|soaked through.{0,80}(lips (are )?blue|shaking|shivering|shiverin)|sleet.{0,80}(lips|shaking|shiver)`), []string{"Hypothermia"}, nil},
	{rx(`(cut|piercing|wound|sore|bite|scratch|blister).{0,50}(manky|gone (red|bad)|infected|pus).{0,80}(confus|shiver|sleepy|burnin|fever|hot|drowsy|breath)`), []string{"Sepsis"}, nil},
	{rx(`(headaches?|sick|dizzy|nause).{0,40}(only|worse) (when|inside)|better when (we|i|they) (go|get) out(side)?|alarm (keeps )?beeping.{0,40}(not (the )?smoke|ceiling)|carbon monoxide alarm|co alarm`), []string{"Carbon monoxide poisoning"}, nil},
	{rx(`(great deal|lot|lots|loads) of blood|blood (is |keeps )?(coming|pumping|pouring|gushing|spurting)`), []string{"Bleeding"}, wx(`nose|period|blood (test|pressure|sugar)`)},
	{rx(`(big |thick |black )?cable (fell|fallen|came down|is down) in (our|the|my) (yard|garden|street|road)|(line|cable|wire)s? down in (the|a) puddle|(wire|cable|line) .{0,20}(smokin|smoking|sparking|sparks)`), []string{"Downed power line"}, wx(`car\b|truck|ute\b|van\b|bonnet|windscreen`)},
	{rx(`elephant (sittin|sitting|on)|chest (is |feels )?(tight|heavy).{0,40}(sweat|sick|clammy|grey)`), []string{"Myocardial infarction"}, nil},
	{rx(`chest (is )?(going|moving) up and down|chest (is )?moving.{0,40}(won.?t|wont|not)|(won.?t|wont|will not) open (his|her|their) eyes`), []string{"Recovery position"}, nil},
	{rx(`(whacked|smacked|hit|banged|bumped|knocked) (my |his |her |their )?head|seein(g)? double`), []string{"Head injury"}, nil},
	{rx(`(just )?stopped breathing|not moving or breathing|(isn.?t|not|stopped) breathing (on|at|in)`), []string{"Cardiopulmonary resuscitation"}, nil},
	{rx(`crackl|popping|black flakes|roaring`, `chimney|flue|stove|wall|burner|fireplace`), []string{"Chimney fire"}, nil},
	// Wider wording, from a stress test of 534 paraphrases written by
	// someone who saw only the note titles.
	{rx(`no heat\b|keep(ing)? (us |everyone |the house )?warm|stay(ing)? warm|freezing in the house|house is freezing|boiler (is )?(broken|off|out|packed in)|radiators? (not working|stopped)|heating (is )?(broken|off|out)|gas (has been |is |was )?(off|cut)`, `freez|cold|winter|snow|baby|old|no heat`), []string{"Winter heating"}, wx(`hypotherm|fell in|shivering uncontroll|\b(car\b|truck|ute\b|van\b|vehicle)\b|outside|outdoors|tent|camp|woods|bush|hiking|trail`)},
	{rx(`tidal wave|(big |huge )?wave warning|(earthquake|quake|shake).{0,40}(coast|beach|seaside|shore|go uphill)|water (has )?gone (way )?out|sea (has )?(gone|went) (way )?out`), []string{"Tsunami"}, nil},
	{rx(`\bchf\b|water tablets|diuretic|furosemide|frusemide|(swollen|puffy) (ankles|legs|feet).{0,60}(breath|weight|pillows|lying)|(ankles|legs|feet) (are )?(really |very )?(swollen|puffy).{0,60}(breath|weight|pillows|lying)|pillows to breathe|sleep(s|ing)? sitting up to breathe|(gained|put on) \d+ ?(kg|lb|lbs|pounds|kilos).{0,30}(days?|week|overnight).{0,30}(puffy|swollen|breath)`), []string{"Heart failure"}, wx(`pregnan|weeks`)},
	{rx(`bottl(e|ing) (tomatoes|beans|veg)|botulism|preserv(e|ing) .{0,25}jars|mason jars?|jars? (didn.?t|didnt|did not|not) seal|lids? (didn.?t|didnt|did not|not) seal|put (them|it) up in (mason )?jars|\bcanning\b|pressure canner|water ?bath`), []string{"Home canning"}, nil},
	{rx(`smok(e|y) (air|from (the )?(bush|wild)?fires?)|air (is )?(thick|full|heavy) (with|of) smoke|bad air quality|air quality (is )?(bad|poor|terrible|unhealthy)|clean (the )?air inside|smoke .{0,25}(cough|headache|wheez)`), []string{"Wildfire"}, wx(`cigarette|vape|smoking|smoke alarm|smoke detector`)},
	{rx(`(touching|on|across|over|draped) .{0,15}(electric|power) lines|electric(al)? lines? (across|over|on) (my|our|the) (car|ute|truck|van)|(wire|line|cable) (draped|lying|resting|dragging) (on|over|across) (the |our |my )?(car|ute|truck|van|bonnet|roof)|wire draped|jump out of (a|the) car`), []string{"Power line on vehicle"}, nil},
	{rx(`(pea|bead|rubber|eraser|lego|little piece|piece of|crayon|stone|bean|toy|button|sweet|peanut).{0,40}(up|in|into) (his|her|their|my)? ?(nose|nostril|ear|lughole)|something (out of|stuck in|up) (an?|the|his|her|my) (ear|nose|nostril)`), []string{"Foreign body"}, nil},
	{rx(`fire on the hill|embers|ash (is )?(raining|falling)|sky (has )?(gone|went|is|turned) orange|veld ?fire|bush ?fire|grass ?fire|fire .{0,20}(closer|heading|coming|moving) (to|towards|toward|our|at|by)`), []string{"Wildfire"}, wx(`fireplace|log burner|campfire`)},
	{rx(`whetstone|(file|hone) (my |the |a )?(axe|knife|blade|machete)|(machete|knife|axe|blade)s? (won.?t|wont|doesn.?t|doesnt|will not) cut`), []string{"Sharpening tools"}, nil},
	{rx(`emergency (radio|broadcast)|radio (stations?|frequenc)|wind.?up radio|official (announcements?|updates|news|info)`), []string{"Emergency news"}, nil},
	{rx(`(blood|bleeding|dripping|pouring|claret|red).{0,25}(nose|nostril|schnozz|hooter|conk)|(nose|nostril|schnozz|hooter|conk).{0,25}(blood|bleeding|dripping|pouring|claret)`), []string{"Nosebleed"}, nil},
	{rx(`\btick out|burn a tick|(bug|thing with legs|black thing|little black).{0,40}(buried|stuck|embedded|dug) (in|into) (my|his|her|their|the)? ?(skin|scalp|head|leg|arm|back)`), []string{"Tick"}, nil},
	{rx(`no water to flush|flush the (loo|toilet)|emergency (toilet|loo)|bucket (loo|toilet)|without plumbing`), []string{"Emergency toilet"}, nil},
	{rx(`\bshock\b.{0,20}(first aid|steps|signs)|going cold and (grey|gray|pale)|(lost|losing) (a lot|lots|loads) of blood|bled a lot.{0,80}(cold|sweaty|pale|grey|gray|faint|breathing fast)`), []string{"Shock"}, wx(`electric|shocked by|got a shock`)},
	{rx(`(mix|mixing|mixed|two different).{0,20}(cleaning products|cleaners|sprays|chemicals)|toxic (smell|fumes) from cleaning|cleaning the (toilet|loo|bathroom|shower).{0,80}(smell|fumes|cough|eyes|chest)`), []string{"Toxic gas"}, nil},
	{rx(`grizzly|black bear|brown bear|bear spray|play dead|(big brown|large|huge) animal with (two |her )?cubs|bears? (on|near|in) (the |our )?(trail|path|camp|tent)`), []string{"Bear"}, nil},
	{rx(`hit by (a )?(bolt|lightning)|bolt from the sky|thunder|flash(es)? .{0,20}(over|above|on top|right on)|shelter under a tree in a storm`), []string{"Lightning"}, wx(`thunder (cat\b|cake|bolts? (movie|film)|down under)|thunderbird`)},
	{rx(`\bblackout\b|no (electricity|power) in the (house|home|street|flat)|\boutage\b|(street|house|town|whole street|estate|neighbourhood|neighborhood) (went|has gone|gone|is|has been) dark`), []string{"Power outage"}, wx(`blackout (curtains|blinds|drunk)|blacked out`)},
	{rx(`disorient|acting (strange|weird|odd|funny)|(thinks|thought) it.?s (19|20)\d\d|(doesn.?t|doesnt|didn.?t|does not) recogni[sz]e|delirium|delirious|sudden memory loss`), []string{"Sudden confusion"}, wx(`\b(dog\b|cat\b|phone|computer|laptop|car\b|printer|app\b)\b`)},
	{rx(`(split up|separated|lost|can.?t find|cant find|locate|find (my|our)|looking for|trace|tracing).{0,40}(family|relatives?|cousin|brother|sister|mum|mom|dad|kids?|children|gran|grandma|grandad|grandpa|husband|wife|partner|son|daughter)`, `evacuat|earthquake|flood|fire|disaster|shelter|centre|center|storm|hurricane|cyclone|tsunami|no phone|phones? (are |is )?(down|dead)|chaos`), []string{"Family reunification"}, nil},
	{rx(`gag(ging|s)?.{0,40}(blue|silent|can.?t breathe|cant breathe|wont come up|won.?t come up|no sound)|back (slaps?|blows?)|something stuck|stuck.{0,20}(can.?t|cant) breathe|(can.?t|cant) breathe.{0,15}stuck|holding (his|her|their) (neck|throat).{0,40}(no sound|can.?t (make|talk|speak|breathe)|cant)|(milk|apple|puree|food|sweet|grape|button|coin|lolly|sausage).{0,40}(blue|can.?t get air|cant get air|can.?t breathe|cant breathe|not breathing|spluttering|silent)|spluttering.{0,30}(colou?r|blue|air)`), []string{"Choking"}, nil},
	{rx(`compressions|chest (pushes|presses)|rescue breaths?|blow in (his|her|their) mouth|press on .{0,20}chest|restart (the|his|her) heart|(isn.?t|isnt|not|stopped|no) breathin\b|limp and blue|not moving.{0,30}not breath`), []string{"Cardiopulmonary resuscitation"}, nil},
	{rx(`out cold|recovery position|(collapsed|passed out|won.?t wake|wont wake|not responding|unresponsive).{0,40}(breathing|chest (is )?moving)|breathing but (not responding|won.?t wake|wont wake|unconscious)`), []string{"Recovery position"}, wx(`not breathing|isn.?t breathing|stopped breathing`)},
	{rx(`too much (to )?(drink|booze)|whole bottle of (vodka|whisk(e)?y|gin|rum|wine|tequila|spirits)|dangerously drunk|alcohol poisoning|(booze|drink|drunk|drinking|vodka|beers?|wine|shots|18th|21st).{0,90}(wont wake|won.?t wake|can.?t wake|cant wake|breathing slow|slow breathing|pale)`), []string{"Alcohol poisoning"}, nil},
	{rx(`fang marks?|two (little )?(holes|puncture marks?)|suck (out )?the venom|(nipped|bitten|got) by a (rattler|snake|adder|viper|copperhead)|rattler`), []string{"Snakebite"}, nil},
	{rx(`noggin|hit (his|her|their|my)? ?head|clash(ed)? (heads|with another player)|asking the same (thing|question)s?`), []string{"Head injury"}, nil},
	{rx(`zapped|(got|had) a (jolt|belt)|thrown back.{0,30}(mains|wire|light|socket|fitting)|stuck to the (angle grinder|drill|tool|appliance)|(can.?t|cant) let go`), []string{"Electrical injury"}, nil},
	{rx(`(electric(al)?|power|black|thick) (cable|wire|line)s? .{0,20}(down|hanging|lying|across|in (a|the) (puddle|road|street))|(cable|wire)s? (down|hanging off|lying) (in|on|across) (the )?(street|road|footpath|garden|yard|puddle)|wire hanging off (a |the )?pole|cable lying in a puddle`), []string{"Downed power line"}, wx(`(on|onto|over|across|touching|on top of|landed on|fell on|fallen on) (our|my|the|his|her|their|a) (car\b|truck|vehicle|van\b|ute\b)|in (our|my|the|his|her|their|a) (car\b|truck|vehicle|van\b)`)},
	{rx(`\bfast test|\bbe ?fast\b|(can.?t|cant|couldn.?t) (see out of one eye|hold (his|her|their|a) cup)|smile (is |was |went )?(crooked|lopsided|wonky|droop)|words (came|coming) out (all )?wrong|one eye.{0,30}(arm|numb)`), []string{"Stroke"}, nil},
	{rx(`rescue someone in the water|struggling in the (sea|water|river|lake|surf|pool)|slipped under`), []string{"Drowning"}, nil},
	{rx(`\bhypo\b|hypoglyc|(sugar|glucose) (level |reading )?(is )?(low|crash\w*|dropping|[0-3](\.\d)?\b)|sugar level [0-3]`), []string{"Hypoglycemia"}, nil},
	{rx(`tentacle|stung (in|by something in) the (water|sea|ocean|surf)`), []string{"Jellyfish sting"}, nil},
	{rx(`chimney|flue|stove ?pipe|wood ?burner|log ?burner|wood ?stove|fireplace`, `fire|roar|glow|red.?hot|flames|sparks|on fire|burning|rumbl|smoke pouring`), []string{"Chimney fire"}, wx(`(get|start|light|keep|lighting|starting) (the |my |a |our )?(wood ?stove|log ?burner|wood ?burner|fireplace|fire)|draw|puffing|smoking back|smokes back|tarry`)},
	{rx(`box jelly|irukandji|chironex|sea wasp`), []string{"Box jellyfish"}, nil},
	{rx(`jelly ?fish|stinger|tentacle|stung (in|by something in) the (water|sea|ocean|surf)`, `tropical|northern australia|queensland|\bqld\b|cairns|darwin|top end|\bnt\b|broome|thailand|philippines|malaysia|indonesia|borneo|vietnam`), []string{"Box jellyfish"}, nil},
	{rx(`power ?lines?|overhead (line|cable|wire)|(line|lines|wire|wires|cable) (came|come|is|are|has|have|got) (down|blown down|on the ground)|downed (line|wire|cable)|pylon|electricity pole|telegraph pole`), []string{"Downed power line"}, wx(`(on|onto|over|across|touching|on top of|landed on|fell on|fallen on) (our|my|the|his|her|their|a) (car\b|truck|vehicle|van\b|ute\b)|in (our|my|the|his|her|their|a) (car\b|truck|vehicle|van\b)|bury|burying|install`)},
	{rx(`shaking (right )?now|(it.?s|its|house|building|bed|room|walls|windows) (is |are )?(shaking|swaying|rattling)`), []string{"Earthquake"}, wx(`shaking all over|all over|eyes|fit\b|jerk|convuls|seiz|shiver|cold|fever|hands\b`)},
	{rx(`leave now|evacuat|have to leave|got to go|(got|have) .{0,10}(\d+|ten|five|fifteen) ?(min|mins|minutes)`, `take|grab|pack|bring|most important|what (do|should) (we|i)`), []string{"Go bag"}, nil},
	{rx(`rain ?(tank|water|barrel|butt)|tank water|roof (water|runoff)|water (off|from) the roof|water ?butt`, `drink|safe|ok|okay|clean|treat|boil|possum|bird|rat`), []string{"Water purification"}, nil},
	{rx(`button batter(y|ies)|coin (cell|batter)|lithium coin|disc batter`), []string{"Poisoning"}, wx(`buy|replace|change|spare|order|(what|which) size|size of|what (type|kind)|remote (won.?t|doesn.?t|not) work`)},
	{rx(`(get|start|light|keep|lighting|starting) (the |my |a |our )?(wood ?stove|log ?burner|wood ?burner|fireplace|fire ?pit|stove|burner)|(wood ?stove|log ?burner|fireplace|fire ?pit) (going|to (light|catch|draw))`), []string{"Starting a fire"}, wx(`pilot|igniter|gas heater|boiler|gas stove|electric stove|camp stove|cooker|hob`)},
	{rx(`jelly ?fish|man o.? war|bluebottle|sea nettle|box jelly`, `sting|stung|got me|got (him|her|them|my)|burn|pee|vinegar|rash|hurts?|pain`), []string{"Jellyfish sting"}, nil},
	{rx(`stitch|suture|glue|gaping|gape|steri.?strip|butterfly (strip|closure)|(cut|gash|slice|laceration).{0,40}(deep|open|wide|won.?t close|see (fat|bone|inside))`), []string{"Cut needing stitches"}, wx(`knitting|sewing|stitch(ing)? (pattern|machine|clothes|fabric)`)},
	{rx(`alcohol|wine|beer|\bdrinks?\b|drinking|vodka|pints?|booze|drunk`, `sleeping (tablet|pill)|zopiclone|zolpidem|ambien|diazepam|valium|xanax|benzo|opioid|codeine|tramadol|oxycodone|morphine|painkiller|antihistamine|benadryl|pregabalin|gabapentin`), []string{"Medicine safety"}, nil},
	{rx(`yellow|jaundice`, `baby|newborn|infant|\d+ ?(day|week)s? old|whites of`), []string{"Newborn jaundice"}, nil},
	{rx(`(her|my|wife.?s|wifes|sister.?s|sisters|partner.?s|daughter.?s|girlfriend.?s|the) waters? (broke|broken|went|gone|have gone|has gone)|baby (is )?coming|wants? to push|(she.?s|shes|she is) pushing|crowning|giving birth|in labou?r|contractions`, `quick|fast|now|can.?t get|snowed|stuck|no (time|way)|won.?t make it|at home|in the car|push|crowning|head`), []string{"Emergency birth"}, wx(`pregnancy test|due date calcul`)},
	{rx(`tsunami|sea (is |has )?(going|gone|went|pulled|drawn|drawing) (way )*(out|back)|water (pulled|drew|went) (way )?back from the (beach|shore)`), []string{"Tsunami"}, nil},
	{rx(`long drop|latrine|pit toilet|outhouse|composting toilet|cesspit|soakaway`, `how far|distance|from (the|our|my|a) (well|water|stream|river|house|borehole)|where (to|should)`), []string{"Emergency toilet"}, nil},
	{rx(`oxygen|concentrator|cpap|ventilator|dialysis|nebuli[sz]er|suction|medical (equipment|device)|powered wheelchair|stairlift|insulin.{0,20}fridge`, `power|outage|load.?shedding|blackout|electricity|generator|battery|no (power|electric)`), []string{"Power outage"}, nil},
	{rx(`(power ?line|wire|cable|line)s?.{0,40}(on|onto|over|across|touching|on top of|landed on|fell on|fallen on) (our|my|the|his|her|their|a) (car|truck|vehicle|van|ute)|(car|truck|vehicle|van).{0,40}(power ?line|wire down|line down|lines? (on|across))`), []string{"Power line on vehicle"}, nil},
	{rx(`go.?bag|bug.?out|grab.?bag|emergency (kit|bag)|evacuation (kit|bag)|72.?h(ou)?r (kit|bag)|(pack|packing) .{0,20}(evacuat|leave in a hurry)`), []string{"Go bag"}, nil},
	{rx(`croup|barking|bark(s|y)? (like|cough)|seal|whistl|stridor|crowing`, `cough|breath|cry|child|kid|toddler|baby|son|daughter|\d+ ?(yr|yo|year)`), []string{"Croup"}, nil},
	{rx(`red (line|streak)|streak(s|ing)? up|line (going|creeping|spreading|moving) up|(cut|wound|scratch|bite|graze|splinter|blister).{0,60}(hot|pus|swollen|swelling|red|throbb|infect|oozing)`), []string{"Wound infection"}, wx(`bleeding (badly|a lot)|won.?t stop bleeding|spurt|gush`)},
	{rx(`dislocat|popped out|out of (its|the) socket`), []string{"Bone fracture", "Joint dislocation"}, nil},
	{rx(`unconscious|unresponsive|passed out|won.?t wake|wont wake|can.?t wake`, `breathing|snoring|recovery position|what position|on (his|her|their) side|lay (him|her|them)`), []string{"Recovery position"}, wx(`not breathing|isn.?t breathing|stopped breathing|no breath`)},
	{rx(`sepsis|septic|mottled|blotchy|(infection|\buti\b|wound|cut|flu|pneumonia).{0,80}(confus|shiver|shak|breath|cold|clammy|not peed|no pee|hasn.?t peed|very unwell|drowsy)`), []string{"Sepsis"}, nil},
	{rx(`pipes?`, `froze|frozen|freez|ice|burst|thaw`), []string{"Frozen pipes"}, nil},
	{rx(`freezer|fridge`, `refreez|re-freez|ice crystals|thaw|been off|how long|keep .{0,12}(frozen|cold)|stay (frozen|cold)|lid (shut|closed)|door (shut|closed)`), []string{"Food safety"}, wx(`inverter|batter(y|ies)|solar|\bah\b|watt`)},
	{rx(`(signal|phone|internet|power|cell|wifi|electricity|tv|radio|lines?|towers?) (is |are |has |have |both |all |been |gone |went |all )*(down|out|dead|lost)|no (phone )?(signal|internet|power|reception|service)`, `find out|news|what.?s (going on|happening)|updates?|information|warnings?|forecast|hear about`), []string{"Emergency news"}, nil},
	{rx(`sharpen|blunt|dull`, `axe|ax\b|hatchet|knife|machete|saw`), []string{"Sharpening tools"}, nil},
	{rx(`(get|start|light|make|build|keep) (a |the |my )?fire|fire going|fire (won.?t|wont|will not) (light|catch|start)|kindling|tinder`), []string{"Starting a fire"}, wx(`pilot|igniter|boiler|gas heater|house fire|on fire|caught fire|fire alarm|fire extinguisher|fired`)},
	{rx(`address|where .{0,20}(lives|living|moved|staying)|\bfind\b|locate|track( down)?|phone number`, `\bmy ex\b|\bex.?(wife|husband|partner|girlfriend|boyfriend)|(he|she|they) (lives|moved|is living|is staying)|[a-z]+ [a-z]+ (lives|moved to)|(new|home) address of|live (on (her|his|their) own|alone)|\b\d+ [a-z]+ (lane|road|rd|street|st|avenue|ave|drive|close|way|court|crescent)\b`), []string{"Finding a person"}, nil},
	{rx(`heart failure|\bchf\b|congestive|weak heart`, `swell|swollen|ankles|weight|gain|pounds|lbs|\bkg|breath|propped|pillows|cough|worse|lie (down|flat)|lying (down|flat)`), []string{"Heart failure"}, nil},
	{rx(`suicid|kill (my|him|her|them)sel(f|ves)|want(s|ed)? to die|wish (i|she|he) (was|were) dead|end (my|his|her|their) (own )?life|end it all|no one would miss|better off (dead|without)|self.?harm|hurt(ing)? (my|him|her|them)self|cutting (my|him|her|them)self|goodbye (message|text|note)|(said|saying|texted|sent|posted) .{0,20}goodbye (message|text|note|to (everyone|us all|the world)|like)|goodbye (message|text|note)|thanks for everything|(cut|cuts|cutting|burn|burns|burning|scratch(es|ing)?) (her|his|their|my) (wrists?|arms?|legs?|thighs?) on purpose|(on purpose|deliberately).{0,30}(cut|burn|hurt)|(going to|gonna|wants? to|will|about to) end it\b|(don.?t|dont|do not|can.?t) see the point|no point (in )?(living|anymore|going on|to (anything|life))|saving (up )?(my |his |her |their )?(tablets|pills|meds)|(can.?t|cannot) go on|nothing to live for|give up on life|take (all )?my pills`), []string{"Suicide"}, nil},
	{rx(`pregnan|preggo|months along|weeks? pregnant|wks pregnant|expecting|due date|my bump`, `headache|vision|flashing|blurr|swell|bleed|blood|spotting|fluid|waters|leak|not moving|moving less|less movement|kicks|cramp|pain|contraction|fell|fall|slipped|fever`), []string{"Pregnancy warning signs"}, nil},
	{rx(`how (much|many)|dose|dosage|max(imum)?`, `ibuprofen|tylenol|paracetamol|acetaminophen|aspirin|benadryl|antihistamine|diphenhydramine|loratadine|cetirizine|advil|motrin|aleve|naproxen|(pills|tablets|medicine|medication|meds)\b.{0,25}\b(can|should|do|to) (i|you|we) take|(can|should) (i|you|we) take`), []string{"Adult medicine dose"}, wx(`\b(dog|cat|pet|vet|horse|animal)s?\b|child|kid|son|daughter|toddler|baby|infant|year.?old|month.?old|\b\d+ ?(yr|yo)\b`)},
	{rx(`stor(e|ing|age)|stockpile|keep|fill`, `water`, `jug|bottle|container|barrel|tank|how long|how much|milk|gallon|litre|liter|emergency`), []string{"Water storage"}, nil},
	{rx(`batter(y|ies)|inverter|amp ?hours?|\b\d+ ?ah\b|solar (panel|generator)|power station|power bank`, `how long|run|last|power|charge|fridge|freezer|watt|size|enough|cpap|how low|discharg|damag|depth|flat|drain|charg`), []string{"Batteries"}, wx(`phone battery (drain|dies|life)|power bank deals|tv remote|swallow|button|\bate\b|mouth|in (his|her|their) (ear\b|nose)|nose|ear\b`)},
	{rx(`(power|electricity|lights|electric).{0,30}(back on|come back|coming back|restored|when will)|power (outage|cut|is out|went out|has been out|out on)|load.?shedding|rolling blackout|when will .{0,30}(power|electricity)`), []string{"Power outage"}, nil},
	{rx(`raccoon|racoon|fox|skunk|\bbats?\b|coyote|wild animal|stray`, `bit|bite|scratch|fought|fight|attack|saliva|licked|our dog|my dog|the dog|cat`), []string{"Rabies"}, nil},
	{rx(`gas(oline)?|petrol|diesel|kerosene|lighter fluid|accelerant|fuel`, `start|light|get .* going|burn pile|campfire|fire ?pit|bonfire|wood stove|brush pile|on the fire`), []string{"Starting a fire"}, wx(`pilot|igniter|gas heater|boiler|gas (leak|smell)|smell (of )?gas|natural gas|gas stove|gas station`)},
	{rx(`fell|fall|fallen|tripped|slipped`, `hip|groin|leg (looks |is )?(shorter|turned|twisted)|can.?t (stand|walk|get up|put weight)|cant (stand|walk|get up)|won.?t (stand|walk)`), []string{"Bone fracture"}, nil},
	{rx(`rash|spots|dots|blotch|pinprick`, `glass|press|doesnt fade|doesn.?t (fade|go away)|wont fade|won.?t fade|fever|stiff neck|purple|spots`), []string{"Meningitis"}, wx(`seein(g)? spots|spots (in|before) (my|her|his|their) eyes|seeing stars|plant|leaf|leaves|tomato|garden|lawn|grass|tree|crop|fruit|paint|wall|fabric|screen`)},
	{rx(`sudden|all of a sudden|today|this morning|overnight`, `confus|not making sense|talking nonsense|disorient|delirious|doesn.?t know where|does not know where`), []string{"Sudden confusion"}, wx(`snow|cold|freez|heat|sun|hot`)},
	{rx(`tetanus|tetnus|tetnis|lockjaw|rusty|stepped on (a |an )?(old )?nail|puncture wound`), []string{"Tetanus"}, nil},
	{rx(`toilet|poop|poo\b|sewage|human waste|latrine|bathroom`, `don.?t work|dont work|not working|no water|can.?t flush|cant flush|won.?t flush|broken|outage|flood|off grid|no plumbing|water.{0,15}(off|out|cut|gone)|no (running )?water|without (running )?water`), []string{"Emergency toilet"}, wx(`seat|hinge|lid|cistern|handle`)},
	{rx(`no heat|heat(ing)? (is )?(out|off|broken)|furnace|keep (the family|us|everyone|the kids|my family) warm|cold house|house is (freezing|cold)`, `winter|storm|snow|cold|freez|power|outage|ice`), []string{"Winter heating"}, nil},
	{rx(`expired|out of date|old |leftover|left over|someone else|my (wife|husband|mom|mum|dad|friend|partner|son|daughter).?s|\bwith\b|together|mix|combine|same time|\bon (warfarin|blood thinners?|apixaban|eliquis|xarelto|rivaroxaban|clopidogrel|plavix)`, `ibuprofen|tylenol|paracetamol|acetaminophen|aspirin|antibiotic|amoxicillin|pills|medication|medicine|meds\b|tablets|prescription|advil|motrin|naproxen|aleve`), []string{"Medicine safety"}, wx(`how (much|many)|dose|with (food|meals?|milk|water|breakfast|dinner)`)},
	{rx(`opioid|opiate|heroin|fentanyl|oxy(contin|codone)?\b|morphine|methadone|percocet|narcan|naloxone`), []string{"Opioid overdose"}, nil},
	{rx(`overdos|too many pills|took a lot of|took (a load of|loads of|a handful of|a bunch of|lots of|heaps of) (pills|tablets)|took (something|some stuff|some pills|drugs|a pill)|(on|taking|took|did) (drugs|pills|gear|something at a party)`, `snoring|not breathing|barely breathing|breathing (slow|weird)|blue|wont wake|won.?t wake|can.?t wake|unresponsive|pinpoint`), []string{"Opioid overdose", "Poisoning"}, nil},
	{rx(`drunk|alcohol|vodka|whiskey|tequila|beers?|shots|drinking|on the (cans|piss|lash|sauce|booze|beers|drink)|pissed|hammered|wasted|smashed|bladdered|steaming|legless|paralytic`, `passed out|snoring|unconscious|won.?t wake|can.?t wake|wont wake|throwing up|vomit|puking|breathing|cold|blue|seizure|sleep it off`), []string{"Alcohol poisoning"}, nil},
	{rx(`\bshock\b|pale and (cold|clammy|sweaty)|clammy|cold sweat|weak pulse`, `bleed|blood|injur|accident|crash|burn|fall|fell|hit|cut|allerg`), []string{"Shock"}, wx(`electric|shocked by|live wire`)},
	{rx(`sprain|twist|rolled|turned|went over on`, `ankle|knee|wrist|foot|thumb`), []string{"Sprain"}, nil},
	{rx(`bleach|chlorine`, `ammonia|vinegar|toilet|acid|cleaner|mixed|mixing|together|fumes|gas`), []string{"Toxic gas"}, nil},
	{rx(`\blost\b|cant find|can.?t find|no idea where`, `woods|forest|hik|trail|mountain|desert|wilderness|backcountry|in the bush|out here`), []string{"Lost outdoors"}, wx(`lost (my|our|his|her) (keys|phone|wallet|dog\b|cat\b)|lost power`)},
	{rx(`diarrh|vomit|throwing up|puking|puked|threw up|been sick|dark (brown )?(wee|pee|urine)|being sick|stomach bug|gastro|runs|runny (poo|poop|stool|nappy|nappies|diaper)|loose (stool|poo|poop)|watery (poo|poop|stool)|(few|no|hardly any|not many|fewer|less) wet (nappies|nappy|diapers?)|dry (nappies|nappy|diapers?)`), []string{"Dehydration"}, wx(`make (him|her|them|me|you|the baby|my \w+) (vomit|sick|throw up)|induce vomit`)},
	{rx(`heat ?wave|no (a/?c|air ?con)|without (a/?c|air ?con)|too hot|hot weather|very hot|extreme heat`), []string{"Heat stroke"}, nil},
	{rx(`smoke|haze|air quality`, `wildfire|bush ?fire|forest fire|fire season|outside|outdoors|in the air|the house|indoors|asthma`), []string{"Wildfire"}, wx(`smoke (alarm|detector)|smoking|smoked|cigarette`)},
	// Added after the training questions: these found a phone named Tornado,
	// an article on menstrual cups for a bear at the campsite, and a
	// Volkswagen repair guide for a smell of gas.
	{rx(`faint|passed out|blacked out|syncope`), []string{"Fainting"}, nil},
	{rx(`\beyes?\b`, `chip|dust|sand|something|scratch|object|splinter|grit|metal|sawdust|stuck|poked|lash`), []string{"Eye injury", "Corneal abrasion"}, nil},
	{rx(`\beyes?\b`, `spray|chemical|bleach|cleaner|acid|lye|pepper|mace|splash`), []string{"Chemical eye injury", "Eye injury"}, nil},
	{rx(`pulse|heart ?rate|heartbeat`, `check|take|find|feel|count|measure`), []string{"Pulse"}, nil},
	{rx(`trapped|pinned|crushed|stuck under`, `tree|beam|rubble|\bcar\b|rock|log|wall|debris|weight|under`), []string{"Crush injury", "Crush syndrome"}, nil},
	{rx(`smell (of )?gas|gas (leak|smell)|leaking gas|smells? (like )?rotten eggs?|rotten eggs? (smell|stink|pong)|smell gas|smells? of gas|propane leak|natural gas`), []string{"Gas leak"}, wx(`from (my|your) hands|on (my|your) hands|filling up|at the (pump|station)`)},
	{rx(`earthquake|tremor|quake|ground (is |was |keeps )?(shaking|moving|rolling)|big shake|aftershock|house (is |was )?shaking`), []string{"Earthquake"}, nil},
	{rx(`tornado|twister|funnel cloud|(mobile home|trailer|caravan park).{0,60}(siren|warning|where do we go|basement|shelter)|(siren|warning).{0,60}(mobile home|trailer)`), []string{"Tornado"}, nil},
	{rx(`flood|flash flood|water (is )?rising|water (over|across|on|covering) the road|(drive|driving|go) (through|thru|across) (the |that )?water|water.{0,20}(coming up|rising|over) the (road|street|drive)|(river|creek|stream|burn|beck).{0,20}(coming up|rising|over|burst|up to)|(foot|feet|inches|knee|cm) deep|(get|drive|go|make it) (through|thru|across) .{0,20}(truck|car|van)|road (is )?(under water|flooded)`), []string{"Flood"}, wx(`flood ?light`)},
	{rx(`stuck|stranded|trapped|snowed in|broke(n)? down|traffic.{0,10}not moving`, `\b(car|truck|vehicle|van|ute)\b|interstate|motorway|highway|freeway|engine running`, `white ?out|snow|blizzard|winter storm|ice storm|freezing|cold`), []string{"Winter car"}, nil},
	{rx(`hurricane|cyclone|typhoon`), []string{"Tropical Cyclone"}, nil},
	{rx(`wildfire|bush ?fire|forest fire|brush fire|grass fire|fire is .{0,15}\d+ ?(miles?|mi|km|kilometres|kilometers) (off|away)`), []string{"Wildfire"}, nil},
	{rx(`\bbears?\b`, `camp|tent|near|attack|charg|\bsee\b|\bsaw\b|spotted|trail|hik|food|cub|woods|yard`), []string{"Bear"}, nil},
	{rx(`baby is coming|giving birth|in labou?r|contractions|waters? (has |have )?broke|deliver (a|the|my|her) baby`), []string{"Childbirth"}, nil},
	// Added after the final set: these found war psychiatry for a fall from a
	// roof, recipes for a child's Tylenol, and pyromania for lighting a fire.
	{rx(`can.?t (feel|move) (his |her |their |my )?(legs|arms|feet|hands|body)|neck (injury|pain|hurt)|back (injury|broken)|broke (his |her |their |my )?(neck|back)|spinal|spine|paraly`), []string{"Spinal injury", "Spinal Cord Injury"}, nil},
	{rx(`epi ?pen|auto-?injector|adrenaline (pen|shot|injection)|epinephrine`), []string{"Epinephrine autoinjector", "Anaphylaxis"}, nil},
	{rx(`stuck|embedded|impaled|lodged|jammed|sticking (out|in)|poking out|went through`, `\b(calf|shin|knee|palm|buttock|bum|arm|leg|hand|foot|chest|belly|stomach|eye|head|neck|thigh|back|wound|skin)\b`, `glass|knife|nail|stick|branch|metal|rod|arrow|shard|blade|object|pole|fence`), []string{"Bleeding", "Penetrating trauma", "Impalement"}, nil},
	{rx(`how (much|many)|dose|dosage|dosing|\bml\b|\bmg\b|can i give|should i give|give (him|her|them|my)`, `child|kid|son|daughter|toddler|baby|infant|year.?old|month.?old|\b\d+ ?(yr|yo)\b`, `tylenol|paracetamol|acetaminophen|calpol|panadol|ibuprofen|motrin|advil|nurofen|benadryl|antihistamine|medicine|medication|syrup|amoxicillin|antibiotic|tablet|pill`), []string{"Child medicine dose"}, nil},
	{rx(`paracetamol|tylenol|acetaminophen|calpol|panadol`, `child|kid|son|daughter|toddler|baby|infant|year.?old|month.?old`), []string{"Paracetamol for children"}, nil},
	{rx(`ibuprofen|advil|motrin|nurofen`, `child|kid|son|daughter|toddler|baby|infant|year.?old|month.?old`), []string{"Ibuprofen for children"}, nil},
	{rx(`paracetamol|tylenol|acetaminophen|panadol`), []string{"Paracetamol for adults"}, wx(`child|kid|son|daughter|toddler|baby|infant|year.?old|month.?old`)},
	{rx(`ibuprofen|advil|motrin|nurofen`), []string{"Ibuprofen for adults"}, wx(`child|kid|son|daughter|toddler|baby|infant|year.?old|month.?old`)},
	{rx(`generator|genny|genset`, `where|how long|safe|put it|place it|garage|indoors|inside|shed|porch|door open|window|outlet|wall|socket|plug|wiring|backfeed|back-feed|panel|transfer switch|whole house|power the house|refuel|fuel|petrol|gas(oline)?|rain|wet`), []string{"Generator"}, wx(`spark plug|oil change|service|wont start|won.?t start|pull cord|carb`)},
}

// wx compiles a rule's exclusions so each must start at a word boundary:
// "ice" (to skip "ice cold" questions) must not cancel a rule because the
// question says "advice", nor "app" because it says "happened", nor "cat"
// because it says "category". Stems still match ("hypotherm", "freez").
func wx(patterns ...string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		out[i] = regexp.MustCompile(`(?i)\b(?:` + p + `)`)
	}
	return out
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
