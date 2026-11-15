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
	{rx(`swallow|ingest|\bate\b|drank|drunk some|got into|(took|taken|had|given|gave) .{0,30}(twice|double|two doses|extra dose|by mistake|by accident)|double dose|accidentally (took|taken|gave|given)|drunk|chew|overdos|took too many|took (a bunch|a lot|lots|loads|a handful|some) of|took (a |the )?(whole |entire |full )?(bottle|pack|packet|box)|(took|taken|swallowed|had) (like |about |around |maybe )?\d{2,}`, `pill|tablet|medic|calpol|paracetamol|tylenol|ibuprofen|nurofen|syrup|kerosene|paraffin|petrol|gasoline|lamp oil|turps|white spirit|lighter fluid|antifreeze|bleach|chemical|cleaner|detergent|soap|laundry|pod|lye|drain|poison|berr|mushroom|batter|antifreeze|fuel|petrol|gasoline|paracetamol|tylenol|aspirin|iron|sleeping`), []string{"Poisoning", "Drug overdose"}, nil},
	{rx(`generator|engine|stove|heater|grill|barbecue|charcoal|furnace|boiler|fire`, `headache|dizz|nause|vomit|drowsy|sleepy|confus|faint|collaps`), []string{"Carbon monoxide poisoning"}, nil},
	{rx(`carbon monoxide|\bco (alarm|detector|poisoning|leak)|\bco2? alarm`), []string{"Carbon monoxide poisoning"}, nil},
	{rx(`diabet|insulin|sugar|type ?(1|one|i)\b|\bt1d?\b`, `shak|sweat|confus|dizz|faint|pale|drowsy|unconscious|not making sense|weird`), []string{"Hypoglycemia"}, nil},
	{rx(`bitten|bite|bit\b|bit me|bit my|got (my|his|her|their|him|me)\b|struck`, `snake|rattler|copperhead|cottonmouth|viper|rattle|adder|cobra`), []string{"Snakebite"}, nil},
	{rx(`bitten|bite|bit\b|bit me|bit my`, `dog|cat|animal|fox|bat|raccoon|monkey`), []string{"Dog bite", "Rabies"}, nil},
	{rx(`bee|wasp|hornet|peanut|nut|shellfish|allerg|sting`, `swell|throat|breath|hives|wheez|face`), []string{"Anaphylaxis"}, nil},
	{rx(`heart attack|chest pain|chest pressure|crushing chest|pain in (my|his|her|their) chest|(pressure|tightness|tight|squeez|heavy|heaviness|crushing|pain|ache).{0,25}chest|chest.{0,40}(left arm|down (his|her|my|their|the) arm|jaw)|(jaw|left arm).{0,30}(ache|aching|pain|hurt|heavy)|(sweat|grey|gray|clammy).{0,60}(jaw|left arm)|(jaw|left arm).{0,60}(sweat|grey|gray|clammy)`), []string{"Myocardial infarction", "Chest pain"}, nil},
	{rx(`chok|something stuck in (his|her|their|my) throat|can.?t breathe or (talk|speak)|grabb(ed|ing) (at )?(his|her|their|my) throat|can.?t (speak|talk|cough|breathe) or (cough|breathe|speak|talk)|(eating|food|grape|sweet|nut|meat|steak|dinner|lunch).{0,60}(went (totally |all )?silent|no sound|can.?t (cry|cough|breathe)|face (going|went|turning) (dark|blue|purple|red))|(stuck|lodged).{0,30}(throat|breath|swallow)|(eating|steak|meat|food|bone|sweet|grape|nut).{0,40}(stuck|can.?t breath|cant breath|barely breath)`), []string{"Choking"}, nil},
	{rx(`stroke|(face|mouth|eye|smile|lip|arm|side).{0,20}droop|droop.{0,20}(face|mouth|eye|smile|lip|arm|side)|slurr|one side (of|is) (the |his |her |their )?(face|body)|(face|mouth|smile).{0,25}(lopsided|fallen|dropped|drooping)|(can.?t|cannot) (lift|raise) (his|her|their|one|my) arm|words (are )?(all )?(jumbled|muddled|garbled)`), []string{"Stroke"}, rx(`(heat|sun) ?stroke|ice|shiver|hypotherm|heat|stopped sweating|drunk|alcohol|slurry (pump|pit|tank|wall|mix|store)|swim|golf|tennis|paddl|rowing|brush ?stroke|back ?stroke|breast ?stroke`)},
	{rx(`seiz|convuls|epilep|having a fit|took a fit|a fit on|(had|has|having|took) (a|another) fit|febrile|fitting|jerking|shaking all over`), []string{"Seizure", "Epilepsy"}, nil},
	{rx(`drown|(pulled|got|dragged|fished|lifted) .{0,25}(from|out of|out) (the )?(water|pool|lake|river|sea|bath|pond|canal|surf)|was under (the water|for|a few|a bit)|under ?water|went under|go under|gone under|deep end|went in the (pool|lake|river|water)|fell in(to)? the (pool|lake|river|pond|water)|submerged|swallowed (a lot of )?water`), []string{"Drowning"}, nil},
	{rx(`bleed|blood (is )?(spurt|pour|everywhere)|deep cut|gash|laceration|gush|sliced (his|her|my|their|open)|nearly (off|severed)|pumping|(soaked|soaking|bleeding) (thru|through)|(chopped|cut|sliced) (off|clean off)|came clean off|amputat|severed|slashed (his|her|my|their)`), []string{"Bleeding", "Hemorrhage", "Tourniquet"}, nil},
	{rx(`broke|broken|fractur|snapped|crack`, `bone|arm|leg|ankle|wrist|hip|rib|collarbone|finger|toe|foot|hand|elbow|knee|shoulder|jaw|nose|skull`), []string{"Bone fracture", "Splint (medicine)"}, rx(`broken (glass|jar|bottle|plate|mirror|window|cup|mug|pane)|broke(n)? (the |my |his |her |their )?skin|skin (was |is )?broken`)},
	{rx(`bent (the )?wrong way|wrong angle|bent funny|bent like|(looks|is) bent|funny (angle|shape)|odd angle|weird angle|looks? (wrong|weird|odd|funny|deformed)|out of place|deformed|crooked|sticking out`, `arm|leg|ankle|wrist|finger|toe|elbow|knee|shoulder|hip|bone`), []string{"Bone fracture", "Joint dislocation"}, nil},
	{rx(`burn|scald`), []string{"Burn", "Thermal burn"}, rx(`wood|\blogs?\b|fuel|candle|calorie|\bfat\b|\bcds?\b|dvd|burns best|burn (the |our |my )?(trash|garbage|rubbish|waste|leaves)|heartburn|burn ?out|burning (smell|sensation when)|burning up|burn (it|them|the ticks?) off|burning (like|so bad|bad)|feels? (like it.?s|its) burning`)},
	{rx(`spill|spilt|splash|poured|knocked|tipped|pulled|went (on|over|all over)|touched|grabbed|picked up`, `boiling|hot (water|coffee|tea|oil|soup|grease|pan|pot|stove|iron|metal)|stove|kettle|oven`), []string{"Burn", "Thermal burn"}, nil},
	{rx(`heat ?stroke|sunstroke|overheat|too hot|heat exhaustion|stopped sweating|not sweating|working (outside|in the sun) all day|out in the (heat|sun) all day`), []string{"Heat stroke", "Heat exhaustion"}, nil},
	{rx(`hypotherm|freezing|so cold|shivering uncontrollab|fell in(to)? (cold|icy) water`), []string{"Hypothermia"}, rx(`pipe|freezer|\bcar\b|engine|food|water line|plant|garden|tank|battery`)},
	// "old guy in the snow, shivering stopped and hes mumbling" found
	// articles on whiskey, and the answer said to give him some.
	{rx(`shiver|mumbl|confus|slurr|drowsy|sleepy|clumsy|stumbl|pale and cold|cold to the touch|ice cold`, `snow|cold|freez|soaked|drenched|wet|river|lake|creek|rain|\bice\b|icy|winter|frost|outside all night|out all night|wet and`), []string{"Hypothermia"}, rx(`nappy|nappies|diaper|poo|poop|stool|pipe|freezer|engine|food|plant|garden`)},
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
	{rx(`fire|smoke|flames|burning`, `house|home|building|apartment|flat|hallway|room|kitchen|stairs|trapped|escape|stove|pan|oil|grease|fat`), []string{"Smoke inhalation"}, rx(`start (a |the )?fire|fire ?wood|campfire|tinder|kindling|wood ?stove|wildfire|bush ?fire|forest fire|brush fire`)},
	{rx(`lightning|thunder`), []string{"Lightning", "Lightning burn"}, nil},
	{rx(`live wire|electrocut|electric(al)? shock|shocked by|power line|downed line|(got|had|getting|took) a (bad |big )?shock|(got|been|was|is) shocked|shock (off|from) (a|the)|frayed (lead|wire|cable|cord)|stuck to (the|a) .{0,20}(cable|wire|lead|cord|plug|socket)|(cable|wire|lead|cord).{0,20}(cut through|sliced through|chopped)`), []string{"Electrical injury"}, nil},
	{rx(`nose ?bleed|bleeding (from (the |his |her |their |my )?)?nose|nose.{0,20}(bleed|blood)|epistaxis`), []string{"Nosebleed"}, nil},
	{rx(`tooth|teeth`, `knock|out|fell out|lost|broke`), []string{"Tooth avulsion"}, nil},
	{rx(`altitude|high camp|mountain sickness|above \d+ ?(m|ft|feet|metres|meters)`), []string{"Altitude sickness"}, nil},
	{rx(`panic attack|hyperventilat|anxiety attack|(it|this|doc|doctor|gp).{0,30}(anxiety|panic)|(feel|feels|felt) like (i.?m|im|i am|she.?s|he.?s) (dying|going to die|gonna die)`), []string{"Panic attack"}, nil},
	{rx(`sting|stung|stinger`), []string{"Bee sting"}, nil},
	{rx(`splinter`), []string{"Splinter"}, nil},
	// "how do I clean a dirty cut" found iFixit guides to cleaning a mouse.
	{rx(`\bcuts?\b|wound|gash|graze|scrape|lacerat|sliced`, `clean|wash|dirty|infect|dress|bandage|care|heal|stitch|close`), []string{"Wound care", "Laceration"}, nil},
	{rx(`bug|insect|bead|object|something|pea|bean|stone|battery`, `(in|into|up) (my |his |her |their |the )?(ear|nose)|(ear|nose) (is )?(stuck|blocked)`), []string{"Foreign body"}, nil},
	{rx(`sunburn|sun burn`), []string{"Sunburn"}, nil},
	{rx(`hives|welts|urticaria`), []string{"Hives"}, nil},
	{rx(`dehydrat|rehydration|\bors\b`), []string{"Dehydration"}, nil},
	{rx(`fever|temperature of|high temperature|feverish|febrile|\d\d(\.\d)? ?°? ?[CF]\b|\btemp(erature)? (is |of |reads |was )?(3[7-9]|4[0-2]|9[89]|10[0-6])|feels? (really |very |so )?hot|burning up|thermometer|\b3[89]\.\d\b|\b10[0-4]\.\d\b`), []string{"Fever"}, rx(`yellow fever|scarlet fever|hay fever|rheumatic fever|dengue`)},
	{rx(`mushroom|toadstool|fungus|fungi`), []string{"Mushroom poisoning"}, nil},
	{rx(`rain ?water|rain barrel|cistern`), []string{"Rainwater harvesting"}, nil},
	{rx(`\bcann(ing|ed)\b|water ?bath|pressure can|\bcan (green |the |my |some )?(beans|vegetables|corn|meat|tomato|tomatoes|peas|soup)`), []string{"Canning (food preservation method)", "Botulism"}, nil},
	{rx(`honey`, `baby|infant|month|newborn`), []string{"Botulism"}, nil},
	// Topics with a note added from a standard first-aid and preparedness list.
	{rx(`tentacle|stung (in|by something in) the (water|sea|ocean|surf)`), []string{"Jellyfish sting"}, nil},
	{rx(`chimney|flue|stove ?pipe|wood ?burner|log ?burner|wood ?stove|fireplace`, `fire|roar|glow|red.?hot|flames|sparks|on fire|burning|rumbl|smoke pouring`), []string{"Chimney fire"}, rx(`(get|start|light|keep|lighting|starting) (the |my |a |our )?(wood ?stove|log ?burner|wood ?burner|fireplace|fire)|draw|puffing|smoking back|smokes back|tarry`)},
	{rx(`box jelly|irukandji|chironex|sea wasp`), []string{"Box jellyfish"}, nil},
	{rx(`jelly ?fish|stinger|tentacle|stung (in|by something in) the (water|sea|ocean|surf)`, `tropical|northern australia|queensland|\bqld\b|cairns|darwin|top end|\bnt\b|broome|thailand|philippines|malaysia|indonesia|borneo|vietnam`), []string{"Box jellyfish"}, nil},
	{rx(`power ?lines?|overhead (line|cable|wire)|(line|lines|wire|wires|cable) (came|come|is|are|has|have|got) (down|blown down|on the ground)|downed (line|wire|cable)|pylon|electricity pole|telegraph pole`), []string{"Downed power line"}, rx(`(on|onto|over|across|touching) (our|my|the|his|her|their|a) (car|truck|vehicle|van)|in (our|my|the|his|her|their|a) (car|truck|vehicle|van)|bury|burying|install`)},
	{rx(`shaking (right )?now|(it.?s|its|house|building|bed|room|walls|windows) (is |are )?(shaking|swaying|rattling)`), []string{"Earthquake"}, rx(`shaking all over|all over|eyes|fit|jerk|convuls|seiz|shiver|cold|fever|hands`)},
	{rx(`leave now|evacuat|have to leave|got to go|(got|have) .{0,10}(\d+|ten|five|fifteen) ?(min|mins|minutes)`, `take|grab|pack|bring|most important|what (do|should) (we|i)`), []string{"Go bag"}, nil},
	{rx(`rain ?(tank|water|barrel|butt)|tank water|roof (water|runoff)|water (off|from) the roof|water ?butt`, `drink|safe|ok|okay|clean|treat|boil|possum|bird|rat`), []string{"Water purification"}, nil},
	{rx(`button batter(y|ies)|coin (cell|batter)|lithium coin|disc batter`), []string{"Poisoning"}, rx(`buy|replace|change|spare|order|(what|which) size|size of|what (type|kind)|remote (won.?t|doesn.?t|not) work`)},
	{rx(`(get|start|light|keep|lighting|starting) (the |my |a |our )?(wood ?stove|log ?burner|wood ?burner|fireplace|fire ?pit|stove|burner)|(wood ?stove|log ?burner|fireplace|fire ?pit) (going|to (light|catch|draw))`), []string{"Starting a fire"}, rx(`gas stove|electric stove|camp stove|cooker|hob`)},
	{rx(`jelly ?fish|man o.? war|bluebottle|sea nettle|box jelly`, `sting|stung|got me|got (him|her|them|my)|burn|pee|vinegar|rash|hurts?|pain`), []string{"Jellyfish sting"}, nil},
	{rx(`stitch|suture|glue|gaping|gape|steri.?strip|butterfly (strip|closure)|(cut|gash|slice|laceration).{0,40}(deep|open|wide|won.?t close|see (fat|bone|inside))`), []string{"Cut needing stitches"}, rx(`knitting|sewing|stitch(ing)? (pattern|machine|clothes|fabric)`)},
	{rx(`alcohol|wine|beer|\bdrinks?\b|drinking|vodka|pints?|booze|drunk`, `sleeping (tablet|pill)|zopiclone|zolpidem|ambien|diazepam|valium|xanax|benzo|opioid|codeine|tramadol|oxycodone|morphine|painkiller|antihistamine|benadryl|pregabalin|gabapentin`), []string{"Medicine safety"}, nil},
	{rx(`yellow|jaundice`, `baby|newborn|infant|\d+ ?(day|week)s? old|whites of`), []string{"Newborn jaundice"}, nil},
	{rx(`(her|my|wife.?s|wifes|sister.?s|sisters|partner.?s|daughter.?s|girlfriend.?s|the) waters? (broke|broken|went|gone|have gone|has gone)|baby (is )?coming|wants? to push|(she.?s|shes|she is) pushing|crowning|giving birth|in labou?r|contractions`, `quick|fast|now|can.?t get|snowed|stuck|no (time|way)|won.?t make it|at home|in the car|push|crowning|head`), []string{"Emergency birth"}, rx(`pregnancy test|due date calcul`)},
	{rx(`tsunami|sea (is |has )?(going|gone|went|pulled|drawn|drawing) (way )*(out|back)|water (pulled|drew|went) (way )?back from the (beach|shore)`), []string{"Tsunami"}, nil},
	{rx(`long drop|latrine|pit toilet|outhouse|composting toilet|cesspit|soakaway`, `how far|distance|from (the|our|my|a) (well|water|stream|river|house|borehole)|where (to|should)`), []string{"Emergency toilet"}, nil},
	{rx(`oxygen|concentrator|cpap|ventilator|dialysis|nebuli[sz]er|suction|medical (equipment|device)|powered wheelchair|stairlift|insulin.{0,20}fridge`, `power|outage|load.?shedding|blackout|electricity|generator|battery|no (power|electric)`), []string{"Power outage"}, nil},
	{rx(`(power ?line|wire|cable|line)s?.{0,40}(on|onto|over|across|touching) (our|my|the|his|her|their|a) (car|truck|vehicle|van)|(car|truck|vehicle|van).{0,40}(power ?line|wire down|line down|lines? (on|across))`), []string{"Power line on vehicle"}, nil},
	{rx(`go.?bag|bug.?out|grab.?bag|emergency (kit|bag)|evacuation (kit|bag)|72.?h(ou)?r (kit|bag)|(pack|packing) .{0,20}(evacuat|leave in a hurry)`), []string{"Go bag"}, nil},
	{rx(`croup|barking|bark(s|y)? (like|cough)|seal|whistl|stridor|crowing`, `cough|breath|cry|child|kid|toddler|baby|son|daughter|\d+ ?(yr|yo|year)`), []string{"Croup"}, nil},
	{rx(`red (line|streak)|streak(s|ing)? up|line (going|creeping|spreading|moving) up|(cut|wound|scratch|bite|graze|splinter|blister).{0,60}(hot|pus|swollen|swelling|red|throbb|infect|oozing)`), []string{"Wound infection"}, rx(`bleeding (badly|a lot)|won.?t stop bleeding|spurt|gush`)},
	{rx(`dislocat|popped out|out of (its|the) socket`), []string{"Bone fracture", "Joint dislocation"}, nil},
	{rx(`unconscious|unresponsive|passed out|won.?t wake|wont wake|can.?t wake`, `breathing|snoring|recovery position|what position|on (his|her|their) side|lay (him|her|them)`), []string{"Recovery position"}, rx(`not breathing|isn.?t breathing|stopped breathing|no breath`)},
	{rx(`sepsis|septic|mottled|blotchy|(infection|\buti\b|wound|cut|flu|pneumonia).{0,80}(confus|shiver|shak|breath|cold|clammy|not peed|no pee|hasn.?t peed|very unwell|drowsy)`), []string{"Sepsis"}, nil},
	{rx(`pipes?`, `froze|frozen|freez|ice|burst|thaw`), []string{"Frozen pipes"}, nil},
	{rx(`freezer|fridge`, `refreez|re-freez|ice crystals|thaw|been off|how long|keep .{0,12}(frozen|cold)|stay (frozen|cold)|lid (shut|closed)|door (shut|closed)`), []string{"Food safety"}, rx(`inverter|batter(y|ies)|solar|\bah\b|watt`)},
	{rx(`(signal|phone|internet|power|cell|wifi|electricity|tv|radio|lines?|towers?) (is |are |has |have |both |all |been |gone |went |all )*(down|out|dead|lost)|no (phone )?(signal|internet|power|reception|service)`, `find out|news|what.?s (going on|happening)|updates?|information|warnings?|forecast|hear about`), []string{"Emergency news"}, nil},
	{rx(`sharpen|blunt|dull`, `axe|ax\b|hatchet|knife|machete|saw`), []string{"Sharpening tools"}, nil},
	{rx(`(get|start|light|make|build|keep) (a |the |my )?fire|fire going|fire (won.?t|wont|will not) (light|catch|start)|kindling|tinder`), []string{"Starting a fire"}, rx(`house fire|on fire|caught fire|fire alarm|fire extinguisher|fired`)},
	{rx(`address|where .{0,20}(lives|living|moved|staying)|\bfind\b|locate|track( down)?|phone number`, `\bmy ex\b|\bex.?(wife|husband|partner|girlfriend|boyfriend)|(he|she|they) (lives|moved|is living|is staying)|[a-z]+ [a-z]+ (lives|moved to)|(new|home) address of|live (on (her|his|their) own|alone)|\b\d+ [a-z]+ (lane|road|rd|street|st|avenue|ave|drive|close|way|court|crescent)\b`), []string{"Finding a person"}, nil},
	{rx(`heart failure|\bchf\b|congestive|weak heart`, `swell|swollen|ankles|weight|gain|pounds|lbs|\bkg|breath|propped|pillows|cough|worse|lie (down|flat)|lying (down|flat)`), []string{"Heart failure"}, nil},
	{rx(`suicid|kill (my|him|her|them)sel(f|ves)|want(s|ed)? to die|wish (i|she|he) (was|were) dead|end (my|his|her|their|it all)|no one would miss|better off (dead|without)|self.?harm|hurt(ing)? (my|him|her|them)self|cutting (my|him|her|them)self|goodbye (message|text|note)|(saying|said|texted|sent) .{0,20}goodbye|thanks for everything|(cut|cuts|cutting|burn|burns|burning|scratch(es|ing)?) (her|his|their|my) (wrists?|arms?|legs?|thighs?) on purpose|(on purpose|deliberately).{0,30}(cut|burn|hurt)|(going to|gonna|wants? to|will|about to) end it\b|(don.?t|dont|do not|can.?t) see the point|no point (in )?(living|anymore|going on|to (anything|life))|saving (up )?(my |his |her |their )?(tablets|pills|meds)|(can.?t|cannot) go on|nothing to live for|give up on life|take (all )?my pills`), []string{"Suicide"}, nil},
	{rx(`pregnan|weeks? pregnant|wks pregnant|expecting|due date|my bump`, `headache|vision|flashing|blurr|swell|bleed|blood|spotting|fluid|waters|leak|not moving|moving less|less movement|kicks|cramp|pain|contraction|fell|fall|slipped|fever`), []string{"Pregnancy warning signs"}, nil},
	{rx(`how (much|many)|dose|dosage|max(imum)?`, `ibuprofen|tylenol|paracetamol|acetaminophen|aspirin|benadryl|antihistamine|diphenhydramine|loratadine|cetirizine|advil|motrin|aleve|naproxen|(pills|tablets|medicine|medication|meds)\b.{0,25}\b(can|should|do|to) (i|you|we) take|(can|should) (i|you|we) take`), []string{"Adult medicine dose"}, rx(`child|kid|son|daughter|toddler|baby|infant|year.?old|month.?old|\b\d+ ?(yr|yo)\b`)},
	{rx(`stor(e|ing|age)|stockpile|keep|fill`, `water`, `jug|bottle|container|barrel|tank|how long|how much|milk|gallon|litre|liter|emergency`), []string{"Water storage"}, nil},
	{rx(`batter(y|ies)|inverter|amp ?hours?|\b\d+ ?ah\b|solar (panel|generator)|power station|power bank`, `how long|run|last|power|charge|fridge|freezer|watt|size|enough|cpap|how low|discharg|damag|depth|flat|drain|charg`), []string{"Batteries"}, rx(`swallow|button|\bate\b|mouth|in (his|her|their) (ear|nose)|nose|ear\b`)},
	{rx(`(power|electricity|lights|electric).{0,30}(back on|come back|coming back|restored|when will)|power (outage|cut|is out|went out|has been out|out on)|load.?shedding|rolling blackout|when will .{0,30}(power|electricity)`), []string{"Power outage"}, nil},
	{rx(`raccoon|racoon|fox|skunk|\bbats?\b|coyote|wild animal|stray`, `bit|bite|scratch|fought|fight|attack|saliva|licked|our dog|my dog|the dog|cat`), []string{"Rabies"}, nil},
	{rx(`gas(oline)?|petrol|diesel|kerosene|lighter fluid|accelerant|fuel`, `start|light|get .* going|burn pile|campfire|fire ?pit|bonfire|wood stove|brush pile|on the fire`), []string{"Starting a fire"}, rx(`gas (leak|smell)|smell (of )?gas|natural gas|gas stove|gas station`)},
	{rx(`fell|fall|fallen|tripped|slipped`, `hip|groin|leg (looks |is )?(shorter|turned|twisted)|can.?t (stand|walk|get up|put weight)|cant (stand|walk|get up)|won.?t (stand|walk)`), []string{"Bone fracture"}, nil},
	{rx(`rash|spots|dots|blotch|pinprick`, `glass|press|doesnt fade|doesn.?t (fade|go away)|wont fade|won.?t fade|fever|stiff neck|purple|spots`), []string{"Meningitis"}, rx(`plant|leaf|leaves|tomato|garden|lawn|grass|tree|crop|fruit|paint|wall|fabric|screen`)},
	{rx(`sudden|all of a sudden|today|this morning|overnight`, `confus|not making sense|talking nonsense|disorient|delirious|doesn.?t know where|does not know where`), []string{"Sudden confusion"}, rx(`snow|cold|freez|heat|sun|hot`)},
	{rx(`tetanus|tetnus|tetnis|lockjaw|rusty|stepped on (a |an )?(old )?nail|puncture wound`), []string{"Tetanus"}, nil},
	{rx(`toilet|poop|poo\b|sewage|human waste|latrine|bathroom`, `don.?t work|dont work|not working|no water|can.?t flush|cant flush|won.?t flush|broken|outage|flood|off grid|no plumbing|water.{0,15}(off|out|cut|gone)|no (running )?water|without (running )?water`), []string{"Emergency toilet"}, nil},
	{rx(`no heat|heat(ing)? (is )?(out|off|broken)|furnace|keep (the family|us|everyone|the kids|my family) warm|cold house|house is (freezing|cold)`, `winter|storm|snow|cold|freez|power|outage|ice`), []string{"Winter heating"}, nil},
	{rx(`expired|out of date|old |leftover|left over|someone else|my (wife|husband|mom|mum|dad|friend|partner|son|daughter).?s|\bwith\b|together|mix|combine|same time|\bon (warfarin|blood thinners?|apixaban|eliquis|xarelto|rivaroxaban|clopidogrel|plavix)`, `ibuprofen|tylenol|paracetamol|acetaminophen|aspirin|antibiotic|amoxicillin|pills|medication|medicine|meds\b|tablets|prescription|advil|motrin|naproxen|aleve`), []string{"Medicine safety"}, rx(`how (much|many)|dose`)},
	{rx(`opioid|opiate|heroin|fentanyl|oxy(contin|codone)?\b|morphine|methadone|percocet|narcan|naloxone`), []string{"Opioid overdose"}, nil},
	{rx(`overdos|too many pills|took a lot of|took (something|some stuff|some pills|drugs|a pill)|(on|taking|took|did) (drugs|pills|gear|something at a party)`, `snoring|not breathing|barely breathing|breathing (slow|weird)|blue|wont wake|won.?t wake|can.?t wake|unresponsive|pinpoint`), []string{"Opioid overdose", "Poisoning"}, nil},
	{rx(`drunk|alcohol|vodka|whiskey|tequila|beers?|shots|drinking|on the (cans|piss|lash|sauce|booze|beers|drink)|pissed|hammered|wasted|smashed|bladdered|steaming|legless|paralytic`, `passed out|snoring|unconscious|won.?t wake|can.?t wake|wont wake|throwing up|vomit|puking|breathing|cold|blue|seizure|sleep it off`), []string{"Alcohol poisoning"}, nil},
	{rx(`\bshock\b|pale and (cold|clammy|sweaty)|clammy|cold sweat|weak pulse`, `bleed|blood|injur|accident|crash|burn|fall|fell|hit|cut|allerg`), []string{"Shock"}, rx(`electric|shocked by|live wire`)},
	{rx(`sprain|twist|rolled|turned|went over on`, `ankle|knee|wrist|foot|thumb`), []string{"Sprain"}, nil},
	{rx(`bleach|chlorine`, `ammonia|vinegar|toilet|acid|cleaner|mixed|mixing|together|fumes|gas`), []string{"Toxic gas"}, nil},
	{rx(`\blost\b|cant find|can.?t find|no idea where`, `woods|forest|hik|trail|mountain|desert|wilderness|backcountry|in the bush|out here`), []string{"Lost outdoors"}, rx(`lost (my|our|his|her) (keys|phone|wallet|dog|cat)|lost power`)},
	{rx(`diarrh|vomit|throwing up|puking|puked|threw up|been sick|dark (brown )?(wee|pee|urine)|being sick|stomach bug|gastro|runs|runny (poo|poop|stool|nappy|nappies|diaper)|loose (stool|poo|poop)|watery (poo|poop|stool)|(few|no|hardly any|not many|fewer|less) wet (nappies|nappy|diapers?)|dry (nappies|nappy|diapers?)`), []string{"Dehydration"}, rx(`make (him|her|them|me|you|the baby|my \w+) (vomit|sick|throw up)|induce vomit`)},
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
	{rx(`smell (of )?gas|gas (leak|smell)|leaking gas|smells? (like )?rotten eggs?|rotten eggs? (smell|stink|pong)|smell gas|smells? of gas|propane leak|natural gas`), []string{"Gas leak"}, nil},
	{rx(`earthquake|tremor|quake|ground (is |was |keeps )?(shaking|moving|rolling)|big shake|aftershock|house (is |was )?shaking`), []string{"Earthquake"}, nil},
	{rx(`tornado|twister|funnel cloud|(mobile home|trailer|caravan park).{0,60}(siren|warning|where do we go|basement|shelter)|(siren|warning).{0,60}(mobile home|trailer)`), []string{"Tornado"}, nil},
	{rx(`flood|flash flood|water (is )?rising|water (over|across|on|covering) the road|(drive|driving|go) (through|thru|across) (the |that )?water|water.{0,20}(coming up|rising|over) the (road|street|drive)|(river|creek|stream|burn|beck).{0,20}(coming up|rising|over|burst|up to)|(foot|feet|inches|knee|cm) deep|(get|drive|go|make it) (through|thru|across) .{0,20}(truck|car|van)|road (is )?(under water|flooded)`), []string{"Flood"}, rx(`flood ?light`)},
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
	{rx(`paracetamol|tylenol|acetaminophen|panadol`), []string{"Paracetamol for adults"}, rx(`child|kid|son|daughter|toddler|baby|infant|year.?old|month.?old`)},
	{rx(`ibuprofen|advil|motrin|nurofen`), []string{"Ibuprofen for adults"}, rx(`child|kid|son|daughter|toddler|baby|infant|year.?old|month.?old`)},
	{rx(`generator|genny|genset`, `garage|indoors|inside|shed|porch|door open|window|outlet|wall|socket|plug|wiring|backfeed|back-feed|panel|transfer switch|whole house|power the house|refuel|fuel|petrol|gas(oline)?|rain|wet`), []string{"Generator"}, nil},
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
