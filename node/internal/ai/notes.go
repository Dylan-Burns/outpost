package ai

import "regexp"

// Notes are short first-aid and safety notes that Outpost ships itself, for
// the emergencies where a small model is most likely to go wrong. The
// library's encyclopedias are written for clinicians: asked about a choking
// adult, Qwen3 1.7B read the Choking article's resuscitation section and
// said to start CPR on someone who was still awake. A note states the lay
// steps, which situation each is for, and the common mistakes, in a form a
// small model can follow. Each is a summary of public guidance (NHS, Red
// Cross, American Heart Association, CDC, USDA) and says so.
//
// A note is added as the first source when a question names its emergency
// (see EmergencyTopics); the library's passages follow it.

// UseNotes turns the notes on. In tests on 75 questions they raised Qwen3
// 1.7B's score from 0.880 to 0.915 and stopped its worst answers (CPR for a
// choking adult who could still cough, a wrong bleach ratio for drinking
// water, "kill the snake").
var UseNotes = true

type note struct {
	topic string // an EmergencyTopics name
	title string
	basis string
	text  string
	who   string // "baby" or "older": only for questions about a baby, or not; "" for all
}

// babyRE recognises a question about a baby under one year. Small models
// copy a note whole: given one note with steps for adults and for babies,
// Qwen3 1.7B and Llama 3.2 3B both told the reader to give a choking baby
// abdominal thrusts. A note for one situation only holds that situation.
var babyRE = regexp.MustCompile(`(?i)\b(baby|babies|infant|newborn|\d+[- ]?(day|week|month)s?[- ]old)\b`)

var notes = []note{
	{"Choking", "Choking (adult or child over 1)", "NHS and Red Cross guidance", `These steps are for adults and children over 1 year old.
If the person can cough, speak or breathe, encourage them to keep coughing. Do not hit their back or start CPR while they can cough.
If they cannot cough, speak or breathe: give up to 5 sharp back blows between the shoulder blades with the heel of your hand. If that fails, give up to 5 abdominal thrusts: stand behind them, put a fist just above the belly button, hold it with your other hand and pull sharply inwards and upwards. Keep repeating 5 back blows and 5 abdominal thrusts. Call emergency services.
If the person becomes unresponsive and stops breathing normally, call emergency services and start CPR.`, "older"},
	{"Choking", "Choking (baby under 1)", "NHS and Red Cross guidance", `These steps are for a baby under 1 year old.
If the baby can cough or cry, let them keep coughing and stay with them.
If the baby cannot cough, cry or breathe: lay the baby face down along your forearm, head lower than the body, supporting the head and jaw. Give up to 5 sharp back blows between the shoulder blades with the heel of your hand.
If that fails, turn the baby face up along your arm and give up to 5 chest thrusts: press two fingers in the middle of the chest, just below the nipple line, sharply downwards.
Keep repeating 5 back blows and 5 chest thrusts, and call emergency services. Never use abdominal thrusts on a baby.
If the baby becomes unresponsive and stops breathing normally, call emergency services and start CPR.`, "baby"},
	{"Cardiopulmonary resuscitation", "CPR", "NHS, Red Cross and American Heart Association guidance", `CPR is only for someone who is unresponsive and not breathing normally.
Call emergency services, or have someone else call, and send someone for a defibrillator (AED) if there is one.
Adult: kneel beside them, put the heel of one hand in the centre of the chest with the other hand on top, and push down hard and fast, about 5 to 6 cm (2 to 2.4 inches) deep, 100 to 120 times a minute, letting the chest come back up each time.
Child: use one hand; baby: use two fingers. Press down about a third of the depth of the chest.
If you are trained, give 2 rescue breaths after every 30 compressions. If not, keep doing chest compressions only.
Use the defibrillator as soon as it arrives and follow its spoken instructions.
Keep going until help takes over, the person starts breathing normally, or you are too exhausted to continue.`, ""},
	{"Bleeding", "Severe bleeding", "NHS, Red Cross and Stop the Bleed guidance", `Press firmly on the wound with a clean cloth or dressing, and keep pressing hard without stopping to look. If blood soaks through, put more cloth on top; do not remove the first layer.
Call emergency services if the bleeding is heavy.
If an object is stuck in the wound, do not pull it out: press around it, not on it.
Tourniquet: only for life-threatening bleeding from an arm or leg that firm pressure does not stop. Put it 5 to 7 cm (2 to 3 inches) above the wound, between the wound and the body, not on a joint. Tighten it until the bleeding stops, note the time, and do not loosen or remove it; leave that to medical staff.
Lay the person down and keep them warm; heavy blood loss can cause shock.`, ""},
	{"Burn", "Burns and scalds", "NHS and Red Cross guidance", `Cool the burn under cool or lukewarm running water for 20 minutes, as soon as possible.
Remove clothing and jewellery near the burn, unless it is stuck to the skin.
Cover the burn loosely with cling film or a clean, non-fluffy dressing.
Do not use ice, iced water, butter, toothpaste or creams. Do not burst blisters.
Get medical help for burns that are large, deep, white or charred, on the face, hands, feet, joints or genitals, caused by chemicals or electricity, or on a baby or young child.
Chemical burn: brush off any dry chemical, then rinse with plenty of running water for at least 20 minutes.`, ""},
	{"Poisoning", "Swallowed poison or too many pills", "NHS, Red Cross and poison centre guidance", `Call emergency services or a poison centre straight away, even if the person seems well.
Find out what was swallowed, how much and when, and keep the container or packet to show.
Do not make the person vomit, and do not give them anything to eat or drink unless a poison centre or medical professional tells you to.
If they are unconscious but breathing, put them in the recovery position (on their side) and keep checking their breathing. If they stop breathing normally, start CPR.
If a chemical got on the skin or in the eyes, rinse with plenty of running water.`, ""},
	{"Carbon monoxide poisoning", "Carbon monoxide", "CDC and NHS guidance", `Headache, dizziness, nausea, tiredness or confusion in several people sharing a space with a generator, stove, heater, grill or fire is a warning sign of carbon monoxide.
Get everyone into fresh air straight away. Turn off the source only if it is safe, and do not go back inside. Call emergency services.
Never run a generator, camp stove, grill or fuel heater inside a home, garage, tent or vehicle, even with doors or windows open. Keep a generator outside, at least 6 metres (20 feet) from windows, doors and vents.`, ""},
	{"Hypoglycemia", "Low blood sugar", "NHS and American Diabetes Association guidance", `Shakiness, sweating, confusion or odd behaviour in someone with diabetes may be low blood sugar.
If they are awake and can swallow safely: give fast sugar, such as glucose tablets, a small glass of fruit juice or regular (not diet) soft drink, or sweets. Check again after 15 minutes and repeat if they are no better, then give a snack.
If they are unconscious or cannot swallow safely: give nothing by mouth. Put them in the recovery position, use a glucagon kit if there is one and you know how, and call emergency services.`, ""},
	{"Snakebite", "Snakebite", "WHO and CDC guidance", `Get the person to medical care as quickly as possible; antivenom is the treatment.
Keep them calm and as still as possible, and keep the bitten limb still.
Remove rings, watches and tight clothing near the bite before swelling starts.
Do not cut the wound, suck out venom, apply ice, or try to catch or kill the snake. Do not use a tight tourniquet.
Note the time of the bite and what the snake looked like, if it can be done safely.`, ""},
	{"Anaphylaxis", "Severe allergic reaction (anaphylaxis)", "NHS and Red Cross guidance", `Signs include swelling of the throat, lips or tongue, trouble breathing or swallowing, wheezing, and feeling faint.
Use an adrenaline (epinephrine) auto-injector straight away if there is one: press it into the outer thigh, through clothing if needed, and hold it as its instructions say. Call emergency services.
Lie the person down with their legs raised; if breathing is hard, let them sit up. Do not let them stand or walk.
If there is no improvement after 5 minutes and a second auto-injector is available, use it.
They need hospital care even if they recover.`, ""},
	{"Myocardial infarction", "Heart attack", "NHS and American Heart Association guidance", `Chest pain or pressure that may spread to the arm, jaw, neck or back, with sweating, shortness of breath or nausea, may be a heart attack.
Call emergency services straight away.
Help them sit down and rest, and keep them calm.
If they are not allergic to aspirin and have not been told to avoid it, give them one adult aspirin to chew slowly.
If they become unresponsive and stop breathing normally, start CPR.`, ""},
	{"Stroke", "Stroke", "NHS and American Stroke Association guidance (FAST)", `Think FAST. Face: has one side drooped? Arms: can they lift both arms and keep them up? Speech: is it slurred or confused? Time: call emergency services straight away.
Note the time the symptoms started.
Do not give them anything to eat or drink, and do not give aspirin; some strokes are caused by bleeding.
If they are unconscious but breathing, put them in the recovery position.`, ""},
	{"Seizure", "Seizure", "NHS and Epilepsy Foundation guidance", `Protect the person from injury: move hard or sharp objects away and cushion their head.
Do not hold them down and do not put anything in their mouth.
Time the seizure. When it stops, roll them onto their side (recovery position) and stay with them until they are fully awake.
Call emergency services if the seizure lasts more than 5 minutes, they have another one, it is their first seizure, they are hurt, have trouble breathing, or it happened in water.
A seizure with a high fever in a young child (febrile seizure) should be checked by a doctor. Do not put the child in a cold bath.`, ""},
	{"Drowning", "Drowning", "Red Cross and European Resuscitation Council guidance", `Do not put yourself in danger: reach or throw something to the person rather than going in, if you can.
Once they are out, if they are not breathing normally, call emergency services, give 5 rescue breaths and then start CPR.
If they are breathing, put them in the recovery position and keep them warm.
Anyone pulled from the water after struggling to breathe should be checked by a doctor, even if they seem well.`, ""},
	{"Bone fracture", "Broken bones", "NHS and Red Cross guidance", `Keep the injured part still and support it in the position you found it. Do not try to straighten it.
To splint, pad it and fix it to something rigid (a board, rolled newspaper or the other leg) so that the joints above and below the break cannot move. Check that the fingers or toes beyond it stay warm and pink.
If bone is through the skin, cover the wound with a clean dressing and press around it, not on the bone.
A cold pack wrapped in cloth can ease swelling. Do not put weight on a broken leg or ankle. Get medical help.`, ""},
	{"Heat stroke", "Heat stroke and heat exhaustion", "NHS and CDC guidance", `Heat exhaustion (heavy sweating, weakness, headache, dizziness, nausea): move to a cool place, lie down with legs raised, drink water, and cool the skin. They should improve within 30 minutes.
Heat stroke is an emergency: a very high temperature with confusion, collapse, a seizure, or hot skin that may have stopped sweating. Call emergency services and cool them as fast as you can: cold water on the skin, wet sheets and a fan, ice packs in the armpits, neck and groin, or a cold water bath.
Give sips of water only if they are awake and can swallow.`, ""},
	{"Hypothermia", "Hypothermia", "NHS and Red Cross guidance", `Signs: shivering, pale cold skin, slurred speech, clumsiness, confusion, drowsiness. Shivering may stop as it gets worse.
Move the person somewhere warm and dry, out of the wind, and handle them gently. Remove wet clothing.
Wrap them in dry blankets or a sleeping bag, cover the head, and insulate them from the ground. Warm the body's core first; skin-to-skin contact helps.
If they are fully awake and can swallow, give warm sweet drinks.
Do not use a hot bath, do not rub their arms or legs, and do not give alcohol.
Get medical help. If they stop breathing normally, start CPR.`, ""},
	{"Frostbite", "Frostbite", "NHS and Mayo Clinic guidance", `Get to a warm place. Do not rub the frostbitten area, and do not walk on frostbitten feet if it can be avoided.
Do not rewarm it if it could freeze again before you reach shelter.
Rewarm in warm (not hot) water, about 37 to 39 °C, for 15 to 30 minutes, or with body heat. Do not use a fire, heater or hot water bottle directly on it; numb skin burns easily.
Cover it loosely with clean dry dressings and get medical help.`, ""},
	{"Head injury", "Head injury", "NHS guidance", `Get emergency help if the person was knocked out even briefly, is confused or very drowsy, vomits, has a seizure, has a worsening headache, has clear fluid or blood from the nose or ears, has trouble speaking, seeing or walking, or has weakness on one side.
If a neck injury is possible, keep the head and neck still.
Someone should stay with the person for the first 24 hours and get help if any of these signs appear.`, ""},
	{"Asthma", "Asthma attack", "NHS and Asthma + Lung UK guidance", `Help the person sit upright, and keep them calm. Loosen tight clothing.
If they have a reliever inhaler (usually blue), help them use it: one puff every 30 to 60 seconds, up to 10 puffs.
If there is no inhaler, keep them sitting up and breathing slowly and steadily.
Call emergency services if they do not get better, are getting worse, are too breathless to speak, or their lips turn blue.`, ""},
	{"Food safety", "Food after a power cut", "USDA and FoodSafety.gov guidance", `Keep the fridge and freezer doors closed.
A closed fridge keeps food cold for about 4 hours. A full freezer keeps its temperature for about 48 hours, a half-full one about 24 hours.
Throw away meat, fish, dairy, eggs and leftovers that have been above 4 °C (40 °F) for more than 2 hours.
Frozen food that still has ice crystals, or is at 4 °C (40 °F) or below, can be refrozen.
When in doubt, throw it out. Never taste food to decide whether it is safe.`, ""},
	{"Home canning", "Home canning", "USDA guidance", `Low-acid foods, which include most vegetables such as green beans, corn and potatoes, and all meat, poultry and fish, must be processed in a pressure canner. A boiling water bath does not get hot enough to kill botulism spores in them.
A boiling water bath is only for acidic foods: most fruits, jams, and pickles made with a tested recipe.
Tomatoes need added acid (bottled lemon juice or citric acid) before canning.
Use a tested recipe and processing time. Throw away any can that bulges, leaks, spurts or smells bad, without tasting it.`, ""},
	{"Smoke inhalation", "Fire at home", "Red Cross, NFPA and fire service guidance", `House fire: get everyone out and stay out. Do not stop to collect belongings, and never go back inside. Call the fire service once you are outside.
Smoke rises: crawl low under it, where the air is cleaner. Before opening a door, feel it and the handle with the back of your hand; if they are hot, use another way out. Close doors behind you.
If you are trapped, close the door, block gaps with cloth, and signal from a window.
If your clothes catch fire: stop, drop to the ground and roll, covering your face.
Pan of oil or fat on fire: turn off the heat if you can do it safely and cover the pan with a lid, baking tray or fire blanket. Leave it covered. Never throw water on it, and do not try to carry the pan. If the fire spreads, get out.`, ""},
	{"Lightning", "Lightning", "US National Weather Service guidance", `When thunder roars, go indoors: no place outside is safe when a thunderstorm is near.
Shelter in a closed building or a hard-topped metal vehicle with the windows shut. Tents, picnic shelters and open-sided huts do not protect you.
If you are caught outside with no shelter: get off hilltops, ridges and other high ground, and away from open fields, water, metal fences and isolated tall trees. Never shelter under a lone tree. A group should spread out.
Wait 30 minutes after the last thunder before going back out.
A person struck by lightning carries no electric charge and is safe to touch. Call emergency services, and if they are not breathing normally, start CPR.`, ""},
	{"Electrical injury", "Electric shock", "NHS and Red Cross guidance", `Do not touch the person while they may still be in contact with the electricity. Switch off the power at the mains or unplug the device first.
If you cannot switch it off, stand on something dry that does not conduct electricity (wood, plastic, a rubber mat) and push the source away with a dry wooden object. Stay at least 18 metres (20 yards) away from high-voltage power lines and call emergency services.
Once it is safe, call emergency services. If they are not breathing normally, start CPR.
Cool any burns with running water. Anyone who has had an electric shock should be checked by a doctor, even if they seem fine.`, ""},
	{"Nosebleed", "Nosebleed", "NHS guidance", `Sit down and lean forward, not back, so blood does not run down the throat.
Pinch the soft part of the nose, just above the nostrils, firmly for 10 to 15 minutes without letting go, and breathe through the mouth.
Do not tilt the head back and do not lie down.
Get medical help if the bleeding has not stopped after 10 to 15 minutes of pinching, is very heavy, makes the person weak or dizzy, followed a blow to the head, or the person takes blood-thinning medicine, or if it is a young child who has frequent nosebleeds.`, ""},
	{"Tick", "Removing a tick", "CDC guidance", `Use fine-tipped tweezers. Grasp the tick as close to the skin as you can and pull upward with steady, even pressure. Do not twist or jerk it.
If mouth parts stay in the skin, remove them with the tweezers if you can easily; otherwise leave them and let the skin heal.
Clean the bite and your hands with soap and water or rubbing alcohol.
Do not use nail polish, petroleum jelly, a match or other heat to make the tick detach; remove it as quickly as possible.
See a doctor if a rash, fever, aches or tiredness follow in the weeks after the bite.`, ""},
	{"Fainting", "Fainting", "NHS guidance", `Lay the person down and raise their legs, for example on a chair or bag, to help blood reach the brain. Loosen tight clothing and give them fresh air.
Most people come round within a minute. Check that they are breathing.
If they do not come round within a minute, call emergency services and put them in the recovery position. If they are not breathing normally, start CPR.
Do not give them anything to eat or drink until they are fully awake and alert.
Get medical advice if they fainted for no clear reason, were injured, have chest pain or a pounding heartbeat, or are pregnant.`, ""},
	{"Gas leak", "Smell of gas", "gas utility and fire service guidance", `A smell like rotten eggs may be a gas leak.
Do not switch lights or appliances on or off, do not use a phone or anything else that could make a spark inside, and do not light a flame or smoke.
Open doors and windows on your way out if you can do it quickly. Get everyone out of the building and keep people away.
From a safe distance outside, call the gas company or emergency services.
Turn the gas off at the meter or the propane tank only if you know how and can do it safely. Do not go back inside until you are told it is safe.`, ""},
	{"Earthquake", "Earthquake", "Red Cross, FEMA and USGS guidance", `During the shaking: drop to your hands and knees, cover your head and neck under a sturdy table or desk (or against an inside wall), and hold on until the shaking stops. This is "Drop, Cover and Hold On".
Stay inside until the shaking stops. Do not run outside, and do not stand in a doorway. Keep away from windows, glass and things that could fall.
If you are outside, move away from buildings, trees, streetlights and power lines. If you are driving, stop away from bridges and overpasses and stay in the car.
Afterwards, expect aftershocks. Check for injuries, gas leaks and damage, and leave a damaged building.
Near the coast, if the shaking was strong or long, move to high ground straight away: a tsunami may follow.`, ""},
	{"Tornado", "Tornado", "US National Weather Service guidance", `Go to a basement or storm shelter. If there is none, go to a small room without windows on the lowest floor, such as a bathroom, closet or inside hallway.
Get under something sturdy and cover your head and neck with your arms. Blankets, a mattress or a helmet give extra protection.
Mobile homes and vehicles are not safe in a tornado: go to a sturdy building or shelter before it arrives.
If you are caught outside with no shelter, lie flat in a ditch or low spot and cover your head. Do not shelter under a bridge or overpass.
Stay in shelter until the storm has passed.`, ""},
	{"Flood", "Flood", "US National Weather Service and Red Cross guidance", `Move to higher ground straight away if you are told to, or if water is rising.
Do not walk, swim or drive through floodwater: 15 cm (6 inches) of moving water can knock you down, and 30 cm (1 foot) can carry a car away. Turn around, don't drown.
If your car is caught in rising water, get out and move to higher ground if you can do so safely.
Stay away from power lines and electrical equipment in or near water.
Floodwater can carry sewage and chemicals: do not drink it, and wash with clean water after touching it.
Do not go back home until the authorities say it is safe.`, ""},
	{"Wildfire", "Wildfire", "CAL FIRE and Red Cross guidance", `Leave as soon as you are told to evacuate, or earlier if you feel unsafe: wildfires move fast.
Take your emergency kit, wear long sleeves, long trousers and sturdy shoes, and cover your nose and mouth. Close windows and doors behind you and follow the evacuation route you are given.
If you are trapped in a car: park away from trees and brush, close the windows and vents, lie on the floor and cover yourself with a wool blanket or coat.
If you are trapped at home: stay inside, away from outside walls and windows, with the doors closed. A house gives more protection than the open.
Smoke can harm people far from the fire: stay indoors with windows closed when the air is smoky.`, ""},
	{"Bear", "Bear encounter", "US National Park Service guidance", `Stay calm and do not run: running can make a bear chase you.
Talk calmly so it knows you are human, stand your ground, make yourself look large, and pick up small children. Back away slowly, sideways if you can, and leave the bear a way out.
Have bear spray ready and know how to use it.
If a brown or grizzly bear attacks, lie flat on your stomach with your hands behind your neck and play dead. If a black bear attacks, or any bear attacks you in your tent, fight back hard, aiming at its face.
At camp, store food and anything scented in a bear-proof container, a locked car or hung out of reach, and cook and eat away from where you sleep.`, ""},
	{"Generator", "Generator safety", "US Consumer Product Safety Commission and FEMA guidance", `Never plug a generator into a wall socket or the house wiring ("backfeeding"). It can electrocute you, your neighbours and utility workers repairing the lines, and can start a fire. To power the house, have an electrician install a transfer switch; otherwise plug appliances straight into the generator with heavy-duty outdoor extension cords rated for the load.
Run it outside only, at least 6 metres (20 feet) from windows, doors and vents. Never run it in a home, garage, basement or shed, even with doors open: its carbon monoxide can kill.
Keep it dry: do not run it in the rain or on wet ground unless it has a cover made for it, and touch it only with dry hands.
Turn it off and let it cool before refuelling. Store fuel in approved containers, away from the generator and any flame.`, ""},
	{"Spinal injury", "Possible spinal injury", "NHS and Red Cross guidance", `After a fall from a height, a blow to the head or neck, or a crash, assume the neck or back may be injured if the person has neck or back pain, numbness, tingling or weakness in the arms or legs, or cannot feel or move them.
Do not move them unless they are in immediate danger, such as fire or rising water. Tell them to keep still.
Keep the head and neck still in the position you found them: kneel behind their head and hold it steady with your hands on either side.
Call emergency services. Keep them warm and keep checking their breathing.
If they stop breathing normally, start CPR. If they vomit or you must leave them, roll them onto their side keeping the head, neck and body in line, with help if you can.`, ""},
	{"Water purification", "Making water safe to drink", "CDC guidance", `Boiling is the surest method: bring clear water to a rolling boil for 1 minute (3 minutes above 2,000 metres or 6,500 feet), then let it cool.
If the water is cloudy, let it settle and filter it through a clean cloth first.
Household bleach can disinfect water when boiling is not possible: use plain unscented bleach and check its strength on the label. For clear water, add 8 drops (1/8 teaspoon) of 6% bleach, or 6 drops of 8.25% bleach, per gallon (3.8 litres). Stir and wait at least 30 minutes before drinking; the water should smell slightly of chlorine. Double the amount for cloudy or very cold water.
Filters remove germs only if they are rated for it, and most do not remove viruses; disinfect the water as well when you can.`, ""},
}

// noteTopics adds rules for subjects that have a note but are not on the
// emergencies list.
var noteTopics = []struct {
	re    *regexp.Regexp
	topic string
}{
	{regexp.MustCompile(`(?i)\bcann(ing|ed)\b|\bcan (green |the |my |some )?(beans|vegetables|corn|meat|tomato|tomatoes|potato|carrot|peas|soup)|water ?bath|pressure can`), "Home canning"},
	{regexp.MustCompile(`(?i)(purif|disinfect|treat|filter|safe to drink|drinkable|potable|boil).{0,40}water|water.{0,40}(purif|disinfect|safe to drink|drinkable|potable)|bleach.{0,30}(water|gallon|litre|liter)`), "Water purification"},
}

// Notes returns the notes for the emergencies a question names, as sources
// without labels. It returns nothing unless UseNotes is set.
func Notes(q string) []Source {
	if !UseNotes {
		return nil
	}
	topics := EmergencyTopics(q)
	for _, t := range noteTopics {
		if t.re.MatchString(q) && !contains(topics, t.topic) {
			topics = append(topics, t.topic)
		}
	}
	var out []Source
	for _, t := range topics {
		baby := babyRE.MatchString(q)
		for _, n := range notes {
			if n.topic == t && (n.who == "" || n.who == "baby" && baby || n.who == "older" && !baby) {
				out = append(out, Source{Title: "Outpost note: " + n.title, Book: "Summary of " + n.basis, Text: n.text, Note: n.text})
			}
		}
		if len(out) == 2 {
			break
		}
	}
	return out
}
