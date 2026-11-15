package ai

import (
	"regexp"
	"slices"
)

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
var babyRE = regexp.MustCompile(`(?i)\b(baby|babies|infant|newborn|bub|bubs|bubba|wee one|in the cot|in her cot|in his cot|\d+[- ]?(day|week|month)s?[- ]old)\b`)

var notes = []note{
	{"Choking", "Choking (adult or child over 1)", "NHS and Red Cross guidance", `These steps are for adults and children over 1 year old.
If the person can cough, speak or breathe, encourage them to keep coughing. Do not hit their back or start CPR while they can cough.
If they cannot cough, speak or breathe: give up to 5 sharp back blows between the shoulder blades with the heel of your hand. If that fails, give up to 5 abdominal thrusts: stand behind them, put a fist just above the belly button, hold it with your other hand and pull sharply inwards and upwards. If the person is heavily pregnant or too large to reach around, give chest thrusts instead: put your fist in the centre of the breastbone and pull sharply straight back. Keep repeating 5 back blows and 5 abdominal thrusts. Call emergency services.
If you are alone and choking: call emergency services if you can (an open line helps), then give yourself abdominal thrusts with your fist above your belly button, or lean over the back of a chair and push down onto it sharply.
If the person becomes unresponsive and stops breathing normally, call emergency services and start CPR.`, "older"},
	{"Choking", "Choking (baby under 1)", "NHS and Red Cross guidance", `These steps are for a baby under 1 year old.
If the baby can cough or cry, let them keep coughing and stay with them.
If the baby cannot cough, cry or breathe: lay the baby face down along your forearm, head lower than the body, supporting the head and jaw. Give up to 5 sharp back blows between the shoulder blades with the heel of your hand.
If that fails, turn the baby face up along your arm and give up to 5 chest thrusts: press two fingers or the heel of one hand in the middle of the chest, just below the nipple line, sharply downwards.
Keep repeating 5 back blows and 5 chest thrusts, and call emergency services. Never use abdominal thrusts on a baby.
If the baby becomes unresponsive and stops breathing normally, call emergency services and start CPR.`, "baby"},
	{"Cardiopulmonary resuscitation", "CPR (adult or child over 1)", "NHS, Red Cross and American Heart Association guidance", `CPR is only for someone who is unresponsive and not breathing normally. These steps are for adults and children over 1 year old.
Call emergency services, or have someone else call, and send someone for a defibrillator (AED) if there is one.
Kneel beside them, put the heel of one hand in the centre of the chest with the other hand on top, and push down hard and fast, about 5 to 6 cm (2 to 2.4 inches) deep for an adult, 100 to 120 times a minute, letting the chest come back up each time. For a child, use one hand and press about a third of the depth of the chest.
If you are trained, give 2 rescue breaths after every 30 compressions. If not, keep doing chest compressions only.
Use the defibrillator as soon as it arrives and follow its spoken instructions.
Keep going until help takes over, the person starts breathing normally, or you are too exhausted to continue.`, "older"},
	{"Cardiopulmonary resuscitation", "CPR (baby under 1)", "NHS, Red Cross and American Heart Association (2025) guidance", `CPR is only for a baby who is unresponsive and not breathing normally. These steps are for a baby under 1 year old.
Shout for help and have someone call emergency services. If you are alone, give CPR for 1 minute first, then call.
Lay the baby on its back on a firm surface and tilt the head back only slightly. Give 5 rescue breaths, covering the baby's mouth and nose with your mouth, each just enough to make the chest rise.
Then give 30 chest compressions in the middle of the chest, just below the nipple line, with the heel of one hand or with both thumbs, your hands wrapped around the chest, pressing down about a third of the depth of the chest (about 4 cm or 1.5 inches), 100 to 120 times a minute.
Then give 2 rescue breaths, and keep repeating 30 compressions and 2 breaths until help takes over or the baby starts breathing normally.`, "baby"},
	{"Bleeding", "Severe bleeding", "NHS, Red Cross and Stop the Bleed guidance", `Press firmly on the wound with a clean cloth or dressing, and keep pressing hard without stopping to look. If blood soaks through, put more cloth on top; do not remove the first layer.
Call emergency services if the bleeding is heavy.
If an object is stuck in the wound, do not pull it out: press around it, not on it.
Tourniquet: only for life-threatening bleeding from an arm or leg that firm pressure does not stop. Put it 5 to 7 cm (2 to 3 inches) above the wound, between the wound and the body, not on a joint. Tighten it until the bleeding stops, note the time, and do not loosen or remove it; leave that to medical staff.
Lay the person down and keep them warm; heavy blood loss can cause shock.`, ""},
	{"Burn", "Burns and scalds", "NHS and Red Cross guidance", `Cool the burn under cool or lukewarm running water for 20 minutes, as soon as possible. Keep the rest of the person warm while you do, especially a baby or small child.
Remove clothing and jewellery near the burn, unless it is stuck to the skin.
Cover the burn loosely with cling film or a clean, non-fluffy dressing.
Do not use ice, iced water, butter, toothpaste or creams. Do not burst blisters.
Call emergency services for burns that are large, deep, white or charred, caused by electricity, or on a baby. Get medical help the same day for burns on the face, hands, feet, joints or genitals, chemical burns, and burns on a young child.
Chemical burn: brush off any dry chemical, then rinse with plenty of running water for at least 20 minutes.`, ""},
	{"Poisoning", "Swallowed poison or too many pills", "NHS, Red Cross and poison centre guidance", `Call emergency services or a poison centre straight away, even if the person seems well.
Find out what was swallowed, how much and when, and keep the container or packet to show.
With paracetamol (acetaminophen), people often feel fine at first while the liver is being damaged: they need hospital treatment even if they seem well.
Do not make the person vomit, and do not give them anything to eat or drink unless a poison centre or medical professional tells you to.
If they are unconscious but breathing, put them in the recovery position (on their side) and keep checking their breathing. If they stop breathing normally, start CPR.
If a chemical got on the skin or in the eyes, rinse with plenty of running water.
A swallowed button battery, or one that may have been swallowed, is an emergency even if the child seems well: it can burn through the food pipe within 2 hours. Go to the emergency department now. Do not make them sick and do not wait for symptoms.`, ""},
	{"Carbon monoxide poisoning", "Carbon monoxide", "CDC and NHS guidance", `Headache, dizziness, nausea, tiredness or confusion in several people sharing a space with a generator, stove, heater, grill or fire is a warning sign of carbon monoxide.
Get everyone into fresh air straight away. Turn off the source only if it is safe, and do not go back inside. Call emergency services.
Treat a carbon monoxide alarm that sounds as real, even if nobody feels ill: the gas has no smell and symptoms can come late. Get everyone out first, then call the fire service or gas company; do not assume it is the battery. A low-battery warning is usually a single chirp about once a minute, not the full alarm.
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
	{"Heart failure", "Heart failure getting worse", "NHS, British Heart Foundation and American Heart Association guidance", `Call emergency services now if they are breathless at rest or cannot speak in full sentences, have chest pain, faint, cough up pink frothy spit, or become confused or very drowsy.
Otherwise, contact their heart failure nurse, doctor or out-of-hours service today if: their weight has gone up by more than 2 kg (4 to 5 lb) in a few days or about 1 kg (2 to 3 lb) overnight, their ankles, legs or belly are more swollen, they are more breathless, or they need more pillows to breathe at night. These are signs fluid is building up, and treatment often needs changing.
Do not change their water tablets (diuretics) or other heart medicines without advice. Keep weighing them every morning, after the toilet and before breakfast, and write it down.`, ""},
	{"Sepsis", "Sepsis (an infection turning dangerous)", "NHS and UK Sepsis Trust guidance", `Call emergency services now if someone with an infection, or who seems very unwell, has any of: confusion or slurred speech; blotchy, mottled, very pale or bluish skin, lips or tongue; a rash that does not fade under a glass; severe breathlessness or fast breathing; no urine all day; or feels like they are going to die. Do not wait for a booked appointment.
In a baby or young child, also call if they are floppy or hard to wake, breathing very fast, have a fit, or feel cold to the touch.
Sepsis needs antibiotics in hospital within hours. Tell the call handler about the infection and ask "could this be sepsis?".`, ""},
	{"Frozen pipes", "Frozen pipes", "Red Cross and US Department of Energy guidance", `Never use a blowtorch, gas burner or any open flame: it can start a fire inside the wall, and boiling the water inside can burst the pipe.
Open the tap the pipe feeds, then warm the frozen section gently: a hairdryer, a heat pad, towels soaked in hot water, or a space heater kept well away from anything that can burn. Start from the tap end and work back.
If a pipe has burst or you cannot reach the frozen part, turn off the water at the stop tap (main shut-off valve) and call a plumber.
To prevent it, keep the heating on low, open cupboard doors under sinks, and let a cold tap drip on very cold nights.`, ""},
	{"Emergency news", "Getting news when power and phone are down", "FEMA and Ready.gov guidance", `Use a battery or wind-up radio, or a car radio, and tune to local radio stations: they carry official warnings and evacuation orders. In the US, NOAA Weather Radio broadcasts weather alerts all the time.
Text messages often get through when calls do not; keep calls short and save phone battery (low power mode, screen dim).
Follow any official warning straight away, even if you cannot confirm it elsewhere. Listen for sirens or loudspeaker announcements, and check on neighbours.
Do not drive around to see what is happening: roads may be flooded, blocked or needed by rescuers.`, ""},
	{"Sharpening tools", "Sharpening an axe or knife by hand", "US Forest Service guidance", `Clamp or wedge the axe head so it cannot move, and wear a glove on the hand that holds the file.
Use a flat file, pushed from the edge towards the body of the blade, following the original angle. Work both sides evenly until a small burr forms along the edge.
Finish with a wetted sharpening stone, in small circles on both sides. With no stone, a flat, fine-grained river stone or the unglazed ring on the bottom of a ceramic mug will do.
Keep fingers behind the edge, and carry and store the axe with its edge covered.`, ""},
	{"Finding a person", "Finding where someone lives", "Outpost", `Outpost cannot find or share where a private person lives or how to reach them, and will not help track anyone down.
If you need to deliver legal papers or sort out child arrangements, a court or a solicitor (lawyer) can do this for you.
If you are worried that someone is in danger, call the police. If you are afraid of someone, police and domestic abuse helplines can help you stay safe.`, ""},
	{"Recovery position", "Unconscious but breathing", "NHS and Red Cross guidance", `Call emergency services.
If they are breathing normally but will not wake, put them on their side in the recovery position: kneel beside them, put the arm nearest you at a right angle, bring the far arm across their chest with the back of their hand against the cheek nearest you, bend the far knee up, then pull on that knee to roll them towards you onto their side. Tilt the head back gently so the airway stays open.
If you think they may have hurt their neck or back, keep them still and only roll them if they are vomiting or their airway is at risk.
Keep checking their breathing until help arrives. If they stop breathing normally, roll them onto their back and start CPR.`, ""},
	{"Croup", "Croup (barking cough in a child)", "NHS guidance", `A cough that sounds like a barking seal, often worse at night, with a hoarse voice, is usually croup.
Call emergency services now if the child is struggling to breathe, the skin between the ribs or at the base of the throat sucks in with each breath, they make a high-pitched noise when breathing in while calm (not only when crying), they are very pale, blue or grey, drowsy or floppy, or they are drooling and cannot swallow.
Otherwise keep the child calm and upright on your lap: crying makes it worse. Give liquids. Contact a doctor or nurse line (NHS 111) the same day; a single steroid dose often helps. Do not put them in a steam-filled room: it does not help and can scald.`, ""},
	{"Wound infection", "A wound getting infected", "NHS guidance", `A wound that gets more painful, hot, swollen or red after the first day or two, or oozes pus, may be infected: see a doctor, pharmacist or nurse the same day, as it may need antibiotics.
A red line or streak spreading from the wound up the arm or leg means the infection is spreading: get medical help urgently, today.
Call emergency services if they also have a high temperature or shivering, feel very unwell, confused, or breathe fast: this can be sepsis.
Meanwhile keep the wound clean with tap water, cover it with a clean dressing, and raise the limb. Mark the edge of the redness with a pen to see if it spreads.`, ""},
	{"Downed power line", "Fallen power line", "Red Cross and electricity utility guidance", `Stay at least 10 metres (33 feet) away from a fallen power line and anything touching it, including a person, a fence, a car or a puddle. Treat every fallen line as live, even if it is not sparking.
Do not try to rescue someone who is touching it or lying near it, and do not try to move the line with anything, even wood: high voltage can arc through the air and the ground. Keep everyone else back.
Call emergency services now and tell them a power line is down and a person is hurt, so they can have the power cut. Only go to the person once the power company or emergency services say it is off. Then, if they are not breathing normally, start CPR.
If you feel tingling in your feet as you get close, shuffle away with small steps, feet together.`, ""},
	{"Power line on vehicle", "Power line on a car", "Red Cross and electricity utility guidance", `Stay inside the vehicle: it is the safest place. Call emergency services and tell them a power line is on the car. Warn others to stay at least 10 metres (about 33 feet) away; nobody should touch the car or the line.
Only get out if the car is on fire. Then open the door without touching the ground, jump clear with both feet together so you never touch the car and the ground at the same time, and shuffle away with small steps, feet together, without lifting them, until you are at least 10 metres away.
Never try to move the line, even with something wooden: treat every fallen line as live.`, ""},
	{"Go bag", "What to pack in a go bag", "FEMA, Ready.gov and Red Cross guidance", `Water (about 4 litres or a gallon per person per day, for 3 days) and food that keeps; a torch, a battery or wind-up radio and spare batteries; a first-aid kit; a whistle; phone charger and power bank; cash; copies of IDs, insurance and medical documents in a waterproof bag; warm clothes, sturdy shoes and a blanket; hygiene items; and anything babies, older people or pets need.
Medicines: at least a week's supply of every prescription, in original packets, with a written list of medicines, doses, allergies and doctors' numbers. Insulin keeps at room temperature (below about 30 °C) for about 4 weeks once out of the fridge: carry it in an insulated bag with a cool pack, never directly on ice, with needles, a glucose meter and fast-acting sugar. For someone on blood thinners, put the medicine name on the list: it matters to anyone treating an injury.
Keep the bag by the door and check it every 6 months.`, ""},
	{"Newborn jaundice", "Yellow skin in a newborn (jaundice)", "NHS and NICE guidance", `Many babies look a little yellow in their first week. Call a midwife, health visitor, doctor or nurse line (NHS 111) the same day if a baby has yellow skin or yellow whites of the eyes, especially in the first 24 hours of life or after 2 weeks.
Get urgent help now (emergency services or A&E) if a yellow baby is very sleepy or hard to wake, feeding poorly, floppy, has a high-pitched cry, or has pale chalky poo or dark urine.
Keep feeding often. Do not put the baby in direct sunlight to treat it, and do not give water or medicines unless a doctor says so: jaundice is treated in hospital with special light if it is needed.`, ""},
	{"Emergency birth", "A baby arriving before help can get there", "NHS and Red Cross guidance", `Call emergency services now and keep the line open: the call handler will talk you through the birth.
Wash your hands. Lay down clean towels and keep the room warm. Let the mother take the position she wants, often on her side or on all fours. Do not try to delay the birth or pull on the baby.
As the head comes, support it gently; the body usually follows with the next contraction. Dry the baby straight away, place them skin to skin on the mother's chest, and cover them both, including the baby's head. If the baby is not breathing or crying after drying and rubbing, follow the call handler's instructions for baby CPR.
Do not cut or pull the cord. The afterbirth (placenta) usually comes within an hour: keep it for the medics. Heavy bleeding afterwards is an emergency: tell the call handler at once.`, ""},
	{"Tsunami", "Tsunami", "US National Weather Service and Red Cross guidance", `If you hear a tsunami warning siren, feel a strong earthquake near the coast, or see the sea pull back unusually far, move at once to high ground or inland, on foot if you can. Do not wait for an official order and do not go to the shore to look.
Go as high and as far inland as you can: at least 30 metres (100 feet) above sea level or 3 kilometres (2 miles) inland. If you cannot get out of the area, go to the upper floors or roof of a strong concrete building.
A tsunami comes as a series of waves and later ones can be bigger: stay away from the coast until officials say it is safe, which can be many hours.`, ""},
	{"Jellyfish sting", "Jellyfish stings", "NHS and Red Cross guidance", `Get out of the water. Rinse the area with seawater, not fresh water. Do not pee on it: urine does not help and can make it worse.
Lift off any tentacles with tweezers, gloves or the edge of a card, not bare hands. Then soak the area in water as hot as can be tolerated (not scalding), or use hot packs, for about 30 minutes; painkillers can help. This is for most jellyfish and the Portuguese man o' war (bluebottle). In tropical waters where box jellyfish live, follow the box jellyfish steps instead.
Call emergency services if they have trouble breathing, chest pain, a fit, swelling of the face or throat, collapse, or a sting over a large area or on the face or genitals, or if a box jellyfish or Portuguese man o' war stung them.`, ""},
	{"Box jellyfish", "Box jellyfish sting (tropical waters)", "ANZCOR and Australian Red Cross guidance", `Call emergency services now. Get the person out of the water and keep them still.
Pour plenty of vinegar over the sting for at least 30 seconds before touching any tentacles; then remove them carefully with tweezers or gloves. If there is no vinegar, rinse with seawater, never fresh water.
Watch their breathing closely: a box jellyfish sting can stop the heart. If they stop breathing normally, start CPR and keep going until help arrives.
Severe pain, sweating, vomiting, or chest or back pain an hour or so later can be Irukandji syndrome: call emergency services.`, ""},
	{"Wood stove smoke", "Wood stove smoking into the room", "US EPA Burn Wise, NFPA and CDC guidance", `Smoke coming into the room also brings carbon monoxide. If anyone has a headache, dizziness, sickness or sleepiness, get everyone into fresh air and call emergency services. Fit a carbon monoxide alarm near any stove.
To stop the smoke: open the stove's air vents fully before lighting, warm a cold flue first by burning a twist of newspaper or small dry kindling, open a nearby window slightly, and open the stove door slowly.
Burn only dry, seasoned wood (split and dried for 6 months or more). Never burn wet or green wood, rubbish, or treated or painted wood: they smoke and coat the chimney with creosote.
Black, tarry liquid or heavy smoke from the flue means creosote, which can catch fire: stop using the stove until a chimney sweep has cleaned and checked the chimney.`, ""},
	{"Chimney fire", "Chimney fire", "NFPA, London Fire Brigade and Red Cross guidance", `A loud roaring or rumbling, a stove pipe or flue glowing red, or flames or sparks from the top of the chimney mean a chimney fire. Get everyone out of the house and call the fire service.
Only if it is safe and quick on the way out: close the stove door and its air vents to starve the fire of air. Do not throw water into a hot stove or onto a glowing pipe: steam and cracking metal can injure you.
Do not go back in. Afterwards, do not use the stove again until a chimney sweep has inspected the chimney, as a chimney fire can crack it.`, ""},
	{"Cut needing stitches", "Does a cut need stitches?", "NHS and Red Cross guidance", `First stop the bleeding: press firmly on the cut with a clean cloth for 10 minutes without lifting it, and raise the injured part.
A cut needs a nurse or doctor (stitches, glue or strips) if its edges gape apart, it is deep or longer than about 2 cm, it is on the face, a joint, the hand or the genitals, it was made by something dirty or a bite, or you can see fat, muscle or bone. Go the same day: wounds are best closed within about 6 to 8 hours.
Call emergency services if bleeding will not stop after 10 minutes of firm pressure, blood is spurting, or the hand or finger beyond the cut is numb, pale or cannot move normally.
Until then, rinse it under running tap water, cover it with a clean dressing, and do not try to close a deep or dirty cut yourself.`, ""},
	{"Family reunification", "Finding family after a disaster", "Red Cross and FEMA guidance", `Register yourself at the evacuation centre or shelter desk and ask staff to check their lists for your family member; centres keep records of who has arrived.
Leave a written note at your home or your agreed meeting place saying you are safe and where you have gone. Text rather than call: texts often get through when calls do not, and a family member out of the area can pass messages between you.
The Red Cross helps reunite families after disasters (in the US, the "Safe and Well" service; worldwide, Restoring Family Links). If a child or someone vulnerable is missing, tell the police or emergency services straight away.`, ""},
	{"Drowning", "Drowning", "Red Cross and European Resuscitation Council guidance", `Do not put yourself in danger: reach or throw something to the person rather than going in, if you can.
Once they are out, if they are not breathing normally, call emergency services, give 5 rescue breaths and then start CPR.
If they are breathing, put them in the recovery position and keep them warm.
Anyone pulled from the water after struggling to breathe should be checked by a doctor, even if they seem well.
For the next day, watch them closely: get urgent medical help if they keep coughing, breathe fast or with effort, have chest pain, become unusually sleepy or confused, vomit, or look pale or blue. Breathing problems can start hours later.`, ""},
	{"Bone fracture", "Broken bones", "NHS and Red Cross guidance", `Keep the injured part still and support it in the position you found it. Do not try to straighten it.
To splint, pad it and fix it to something rigid (a board, rolled newspaper or the other leg) so that the joints above and below the break cannot move. Check that the fingers or toes beyond it stay warm and pink.
If bone is through the skin, cover the wound with a clean dressing and press around it, not on the bone.
A cold pack wrapped in cloth can ease swelling. Do not put weight on a broken leg or ankle. Get medical help.`, ""},
	{"Heat stroke", "Heat stroke and heat exhaustion", "NHS and CDC guidance", `Heat exhaustion (heavy sweating, weakness, headache, dizziness, nausea): move to a cool place, lie down with legs raised, drink water, and cool the skin. They should improve within 30 minutes; if they do not, or they get worse or confused, treat it as heat stroke and call emergency services.
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
Anyone who takes blood-thinning medicine (warfarin, apixaban, rivaroxaban, clopidogrel and others) needs to be checked urgently after any blow to the head, even if they seem fine: bleeding inside the skull can start slowly.
Someone should stay with the person for the first 24 hours and get help if any of these signs appear.`, ""},
	{"Asthma", "Asthma attack", "NHS and Asthma + Lung UK guidance", `Help the person sit upright, and keep them calm. Loosen tight clothing.
If they have a reliever inhaler (usually blue), help them use it: one puff every 30 to 60 seconds, up to 10 puffs. If their reliever is a combination inhaler (MART or AIR, often not blue), it is one puff every 1 to 3 minutes, up to 6 puffs.
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
	{"Electrical injury", "Electric shock", "NHS and Red Cross guidance", `These steps are for household electricity: an appliance, a cable, a socket. For a fallen power line, do not go near at all.
Do not touch the person while they may still be in contact with the electricity. Switch off the power at the mains or unplug the device first.
If you cannot switch it off, stand on something dry that does not conduct electricity (wood, plastic, a rubber mat) and push the source away with a dry wooden object.
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
Do not walk, swim or drive through floodwater, however shallow it looks: the road under it may be washed away and the depth is hard to judge. 15 cm (6 inches) of moving water can knock you down, and 30 cm (1 foot) can carry a car away. Turn around, don't drown.
If water is rising around your car but is not moving fast, get out and go to higher ground. If the car is in fast-moving water, stay inside with your seatbelt off; if water comes into the car, climb onto the roof and call emergency services.
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
	{"Fever", "Fever in a young baby", "NHS guidance", `A baby under 3 months old with a temperature of 38 °C (100.4 °F) or higher needs urgent medical help: call a doctor or emergency services now. Do not wait until morning.
A baby 3 to 6 months old with a temperature of 39 °C (102.2 °F) or higher also needs urgent advice.
Get help at once, at any temperature, if the baby is floppy or hard to wake, will not feed, has a rash that does not fade when a glass is pressed on it, has a seizure, or is breathing fast or with difficulty.
Keep the baby lightly dressed, offer feeds often, and check the nappies for dehydration. Ask a doctor or pharmacist before giving any fever medicine to a baby under 3 months.`, "baby"},
	{"Winter car", "Stuck in a car in snow", "US National Weather Service and Red Cross guidance", `Stay with the car. It shelters you and is easier for rescuers to find; do not walk away in a storm.
Keep the exhaust pipe clear of snow and run the engine only about 10 minutes each hour for heat, with a window open a crack, so carbon monoxide does not build up inside.
Make the car easy to see: tie a bright cloth to the aerial or door, and turn on the inside light while the engine runs.
Keep warm: put on extra clothes, use blankets, huddle together, and move your arms and legs now and then.
Drink fluids. Watch each other for signs of hypothermia and frostbite.`, ""},
	{"Child medicine dose", "Medicine doses for children", "NHS and US FDA guidance", `A child's dose depends on the child's age and weight and on the strength of the medicine, which differs between products (infant drops, children's liquid, tablets). Read the dose from the table on the packet or leaflet of the medicine you have.
Measure liquid medicine with the syringe or spoon that comes with it, not a kitchen spoon.
Leave the gap between doses that the label gives, and never give more doses in 24 hours than it allows.
Do not give two medicines that contain the same ingredient, such as paracetamol (acetaminophen) in a cold remedy and on its own.
Do not give aspirin to anyone under 16. If you are unsure, the child is under 3 months, or has another illness, ask a pharmacist or doctor.`, ""},
	{"Rabies", "Animal bites and rabies", "WHO and CDC guidance", `Wash the bite or scratch straight away with soap and plenty of running water for about 15 minutes, then put on an antiseptic if you have one. Press on it with a clean cloth if it bleeds.
Get medical care the same day for any bite that breaks the skin, and for any bite or scratch from a wild animal (bat, fox, raccoon, skunk, monkey) or an unknown dog or cat, and after any contact with a bat, even with no mark you can see. Rabies vaccine given soon after the bite prevents the disease; once symptoms start it is almost always fatal.
Note what the animal was and where it went, but do not try to catch it bare-handed.
See a doctor if the bite becomes red, swollen, hot or oozing, or you get a fever.`, ""},
	{"Dehydration", "Diarrhoea, vomiting and dehydration", "WHO and NHS guidance", `Keep drinking: small sips often if you are being sick. Oral rehydration solution is best: use sachets if you have them, or make it with 6 level teaspoons of sugar and half a level teaspoon of salt in 1 litre of clean, safe water (WHO recipe).
Babies: keep breastfeeding or giving formula, and offer extra fluids between feeds.
Signs of dehydration: thirst, dark yellow or little urine, dry mouth, dizziness; in a baby, fewer wet nappies, a sunken soft spot on the head, or no tears.
Get medical help urgently for blood in the stool or vomit, a very sleepy, floppy or confused person, no urine all day, severe tummy pain, or a baby, young child or older person who cannot keep fluids down.`, ""},
	{"Tooth avulsion", "Knocked-out tooth", "American Association of Endodontists and NHS guidance", `Pick the tooth up by the crown (the white part), not the root.
If it is dirty, rinse it briefly in milk or with the person's saliva. Do not scrub it or wrap it in tissue.
An adult (permanent) tooth: push it gently back into its socket, the right way round, and bite on a clean cloth to hold it. If you cannot, keep it in milk, or in the cheek of an adult who will not swallow it.
Do not put a baby (milk) tooth back.
See a dentist as soon as possible, ideally within 1 hour: the sooner, the better the tooth's chance.`, ""},
	{"Eye injury", "Eye injuries", "NHS and American Academy of Ophthalmology guidance", `Chemical in the eye: rinse it straight away with clean running water for at least 15 to 20 minutes, holding the eyelids open, with the water running away from the other eye. Take out contact lenses. Then get urgent medical help.
Dust or grit: blink, and rinse with clean water. Do not rub the eye.
Something stuck in or through the eye: do not remove it or press on the eye. Cover the eye loosely (a paper cup works), ask the person to keep both eyes still, and get emergency help.
A blow to the eye: hold a cold cloth gently against it, and get it checked if sight changes or it stays painful.`, ""},
	{"Opioid overdose", "Opioid overdose", "CDC and SAMHSA guidance", `Signs: very slow, noisy or stopped breathing, blue or grey lips, tiny pupils, and the person cannot be woken.
Call emergency services.
Give naloxone (Narcan) if you have it: spray it into one nostril. It is safe to give even if you are not sure it is an opioid overdose. If there is no response after 2 to 3 minutes, give a second dose.
If they are not breathing normally, start CPR. If they are breathing, put them in the recovery position.
Stay with them until help arrives: naloxone can wear off before the opioid does, and breathing can stop again.`, ""},
	{"Alcohol poisoning", "Alcohol poisoning", "NHS guidance", `Call emergency services if the person cannot be woken, has slow or irregular breathing, has a seizure, has pale or blue skin, or is being sick while not fully awake.
Do not leave them to sleep it off. Lie them on their side in the recovery position, so they will not choke if they are sick, keep them warm, and stay with them, checking their breathing.
Do not give coffee or more alcohol, do not put them in a cold shower or make them walk, and do not make them vomit.
If they are awake, give sips of water.`, ""},
	{"Shock", "Shock after an injury", "NHS and Red Cross guidance", `Shock is a life-threatening drop in blood flow, often after heavy bleeding, a serious burn or a severe allergic reaction. Signs: pale, cold, clammy skin, a fast weak pulse, fast breathing, and feeling faint, confused or sleepy.
Call emergency services, and treat the cause if you can, such as pressing on a wound that is bleeding.
Lay the person down and, if their legs are not injured, raise them. Keep them warm with a coat or blanket, and loosen tight clothing.
Do not give them anything to eat or drink. Keep checking their breathing, and start CPR if it stops.`, ""},
	{"Sprain", "Sprains", "NHS guidance", `For the first few days: rest the joint, hold an ice pack wrapped in a cloth on it for up to 20 minutes every 2 to 3 hours, support it with a bandage that is firm but not tight, and keep it raised.
Get medical help if you cannot put any weight on it, it looks misshapen, it is numb or cold below the injury, or the pain is severe: it may be broken. Keep it still and splinted until it is seen.`, ""},
	{"Toxic gas", "Fumes from mixed cleaners", "CDC and poison centre guidance", `Mixing bleach with ammonia, acids (toilet cleaners, vinegar, some descalers) or other cleaners makes toxic chlorine or chloramine gas.
Leave the room straight away into fresh air and take others with you. Open windows only if you can do it on the way out without breathing more fumes.
Call a poison centre or emergency services if there is coughing, wheezing, trouble breathing, chest pain, or eye pain. Rinse eyes or skin with plenty of running water for 15 minutes.
Do not go back in until the room has aired out. Never mix cleaning products.`, ""},
	{"Tropical Cyclone", "Hurricane", "US National Weather Service and FEMA guidance", `Leave if you are told to evacuate, especially from a storm-surge zone or a mobile home.
If you stay, shelter in a small inside room, closet or hallway on the lowest floor that will not flood, away from windows. If you are told to go higher because of flooding, do not go into a closed attic.
Do not go outside when the wind drops in the eye of the storm: it comes back from the other direction.
Keep water (about 4 litres or 1 gallon per person per day), food, medicines, a torch, a battery radio and spare batteries.
Afterwards, keep out of floodwater, away from fallen power lines, and never run a generator indoors.`, ""},
	{"Wildfire", "Wildfire smoke", "US EPA and CDC guidance", `Stay indoors with windows and doors closed. Run air conditioning on recirculate if you have it.
Make one room as clean as you can: a portable HEPA air cleaner, or a box fan with a MERV 13 furnace filter taped to it, helps a lot.
Do not add to the smoke indoors: no candles, frying, smoking or vacuuming.
Outdoors, a well-fitted N95 mask filters smoke; cloth and paper dust masks do not.
People with asthma or heart or lung disease should keep their medicines and plan close, and leave the area if the air stays bad or symptoms get worse.`, ""},
	{"Lost outdoors", "Lost outdoors", "US Forest Service and National Park Service guidance", `Stop as soon as you realise you are lost: sit down, think, look around and plan. People who stay in one place are found sooner.
Stay put, above all if it is getting dark. Get out of wind and rain, keep dry, put on extra layers, and insulate yourself from the ground.
Make yourself easy to find: three blasts on a whistle (the distress signal), bright clothing laid out in the open, a signal fire in a safe clear spot, or a mirror flashed at aircraft.
Save your phone battery. A text to emergency services may go through where a call will not.
Drink the water you have, and do not eat wild plants you are not sure of.`, ""},
	{"Panic attack", "Panic attack", "NHS guidance", `Stay with the person and keep calm. Help them breathe slowly: in through the nose and out through the mouth, counting slowly. Do not use a paper bag.
Reassure them that the attack will pass, usually within 20 minutes.
Chest pain, trouble breathing and a racing heart can also be signs of a heart attack or another emergency: get medical help if it is their first attack, if there is chest pain, or if they do not settle.`, ""},
	{"Heat stroke", "Hot weather and heat waves", "CDC and NHS guidance", `In a heat wave, a fan alone may not keep someone safe once it is hotter than about 32 to 35 °C (90 to 95 °F). Cool showers or baths, wet cloths on the skin, and spending the hottest hours somewhere air-conditioned (a library, shop or cooling centre) help more.
Drink water regularly, before feeling thirsty. Keep curtains closed on sunny windows and avoid hard work in the hottest part of the day.
Check on older people, babies and people who live alone at least twice a day.`, ""},
	{"Foreign body", "Something stuck in the nose or ear", "NHS guidance", `Do not try to dig it out with a finger, cotton bud or tweezers: it can be pushed further in.
Nose: ask the child to blow gently through that nostril while you press the other one closed. If it does not come out, get medical help.
Ear: an insect can sometimes be floated out by tilting the head and pouring in a little warm (not hot) water. Do not do this if the ear is painful or has discharge, which may mean a damaged eardrum.
A button battery or magnets in the nose or ear is an emergency: get help straight away. So is trouble breathing, heavy bleeding or severe pain.`, ""},
	{"Meningitis", "Rash that does not fade", "NHS and Meningitis Research Foundation guidance", `A rash that does not fade when you press the side of a glass firmly against it, in someone who is unwell or has a fever, can be meningitis or sepsis. Call emergency services now.
Other warning signs: a stiff neck, dislike of bright light, very sleepy or hard to wake, confusion, cold hands and feet, pale or blotchy skin, a seizure, or a baby who is floppy or has a high-pitched cry.
Do not wait for a rash: it can appear late or not at all. Trust your instinct if someone is getting worse quickly.`, ""},
	{"Sudden confusion", "Sudden confusion", "NHS guidance", `Confusion that comes on suddenly, over hours or a day, needs urgent medical help the same day, above all in an older person. Common causes are an infection (often of the urine or chest), a stroke, low blood sugar, dehydration, a medicine, or a head injury.
Call emergency services now if they also have a drooping face, a weak arm or slurred speech, chest pain, trouble breathing, a high temperature with drowsiness, or cannot be woken.
If they have diabetes, check their blood sugar if you can; if it is low and they can swallow, give them something sugary.
Keep them safe and calm, and give sips of water if they are awake.`, ""},
	{"Tetanus", "Deep or dirty wounds and tetanus", "CDC and NHS guidance", `Clean the wound well under running water and remove any dirt you can see, then cover it.
A deep or dirty wound, such as a puncture from a rusty nail, a wound with soil or manure in it, or an animal bite, can cause tetanus. Get a tetanus booster if your last one was more than 5 years ago or you do not know when it was, ideally within 48 hours.
Get medical help for a deep puncture of the foot, a wound you cannot clean, or signs of infection: spreading redness, swelling, pus, fever, or stiffness of the jaw or neck.`, ""},
	{"Emergency toilet", "When toilets do not work", "FEMA and CDC guidance", `Line the toilet bowl or a bucket with two strong plastic bags. After each use, cover the waste with a little cat litter, sawdust, soil or ash.
Tie the bags when they are a third to half full and keep them in a covered container, away from people, food and water, until they can be collected or buried.
If you bury waste, dig at least 30 cm (1 foot) deep and at least 60 metres (200 feet) from wells, streams and other water. A pit latrine or long drop should also be at least 30 metres (100 feet) from any well or water source, downhill from it, and with its bottom well above the water table.
Wash hands with soap and water, or use hand sanitiser, every time. Never pour waste into floodwater or near a water source.`, ""},
	{"Mushroom poisoning", "Wild mushrooms", "NHS and poison centre guidance", `Never eat a wild mushroom unless an expert has identified it with certainty: some deadly species look like edible ones. White mushrooms with white gills and a ring on the stem can be death caps or destroying angels, which kill.
Cooking, drying or peeling does not make a poisonous mushroom safe.
If anyone, above all a child, has eaten a wild mushroom: call a poison centre or emergency services straight away, even if they seem fine, because the deadliest kinds cause symptoms only 6 to 24 hours later. Keep a piece of the mushroom to show.`, ""},
	{"Winter heating", "No heating in winter", "CDC and Red Cross guidance", `Heat one room and close off the others. Hang blankets over windows and doors, and gather everyone there.
Wear layers, hats and socks, use blankets and sleeping bags, and have warm drinks and food.
Never heat the house with a gas stove or oven, a grill, a camp stove or a generator: they give off carbon monoxide, which kills. Use a fuel-burning heater only if it is made for indoor use, with fresh air coming in, and keep a working carbon monoxide alarm. A fireplace or wood stove needs a clear chimney.
Check babies, older people and anyone ill often for signs of hypothermia: shivering, confusion, drowsiness.`, ""},
	{"Medicine safety", "Mixing, old and someone else's medicines", "NHS and US FDA guidance", `Ask a pharmacist or doctor before taking two medicines together: some common ones interact. Anti-inflammatory painkillers such as ibuprofen and naproxen can raise blood pressure, weaken blood pressure medicines and harm the kidneys; paracetamol (acetaminophen) is usually the safer painkiller for people on blood pressure medicine, within the label's limits.
Do not take antibiotics that are left over, out of date or prescribed for someone else: they may be the wrong medicine or not enough to work. A tooth or wound infection with swelling or fever needs a dentist or doctor.
Anyone on a blood thinner (warfarin, apixaban, rivaroxaban, clopidogrel) should not take ibuprofen, naproxen or aspirin unless their doctor says so: together they raise the risk of serious bleeding.
Do not drink alcohol with sleeping tablets, opioid painkillers (codeine, tramadol), benzodiazepines (diazepam) or drowsy antihistamines: together they can slow breathing dangerously. If someone has, do not let them sleep alone; if they are very drowsy, hard to wake or breathing slowly, call emergency services and put them on their side.
Do not take someone else's prescription medicine, even the same medicine at another strength.`, ""},
	{"Starting a fire", "Starting a fire safely", "NFPA and US Forest Service guidance", `Never use petrol (gasoline), diesel or other fuel to start or revive a fire: the vapour can explode and the flame can flash back to the can and to you. Use charcoal lighter fluid only on charcoal, and only before it is lit.
Start with dry tinder (dry grass, birch bark, cotton wool, wood shavings), add thin dry kindling, then larger wood once it catches. In wet weather, split wood to reach the dry inside and shelter the fire from rain.
Never light a fire, stove or barbecue inside a tent, car or closed room: it can burn and fills the space with carbon monoxide. Build fires away from tents, buildings and overhanging branches, keep water or soil close, never leave one unattended, and put it out completely with water until it is cold to the touch.`, ""},
	{"Suicide", "Someone may be thinking of suicide", "NHS, Samaritans and 988 Lifeline guidance", `If they may act on it now, or have already hurt themselves, call emergency services.
Stay with them; do not leave them alone. Ask them directly whether they are thinking about suicide: asking does not put the idea in their head, and it lets them talk.
Listen calmly, without arguing or judging. Tell them you care and that you want to help them get through this.
Remove or lock away anything they could use to harm themselves: medicines, firearms, knives, ropes, car keys.
Get help with them: in the US call or text 988 (Suicide and Crisis Lifeline); in the UK and Ireland call Samaritans on 116 123; elsewhere, a local crisis line or the emergency number. Stay with them until help arrives or they are with someone they trust.
If it is you who feels this way, tell someone you trust now, or call one of those numbers.`, ""},
	{"Pregnancy warning signs", "Warning signs in pregnancy", "NHS and ACOG guidance", `Call your midwife, maternity unit or emergency services now for any of these:
a severe headache that will not go away, blurred vision or flashing lights, pain under the ribs, or sudden swelling of the face, hands or feet (signs of pre-eclampsia, which is dangerous for mother and baby);
any bleeding from the vagina; fluid leaking (waters breaking) before 37 weeks; the baby moving less than usual or not at all (do not wait until the next day);
severe or constant tummy pain; a fever; regular contractions before 37 weeks; or a fall or blow to the bump.`, ""},
	{"Adult medicine dose", "Medicine doses for adults", "NHS and US FDA guidance", `Take the adult dose printed on the packet or leaflet, and never more than the most it allows in 24 hours.
Check that nothing else you take contains the same ingredient: cold and flu remedies often contain paracetamol (acetaminophen) or an antihistamine.
Antihistamines such as diphenhydramine (Benadryl) cause drowsiness: do not drive or drink alcohol after taking them.
Ask a pharmacist first if you take other medicines, are pregnant, or have liver, kidney or heart problems.
Hives with swelling of the lips, tongue or throat, or trouble breathing, is an emergency: use an adrenaline auto-injector if there is one and call emergency services.`, ""},
	{"Water storage", "Storing drinking water", "FEMA and CDC guidance", `Store at least 4 litres (1 gallon) of water per person per day, for at least 3 days, and for 2 weeks if you can. Count pets too.
Use food-grade containers: sealed bottled water, or clean plastic drink bottles such as soda bottles. Do not use milk or juice jugs: milk and sugar are hard to wash out, bacteria grow, and the thin plastic breaks down. Never use a container that held chemicals.
Wash containers with soap and water, rinse, then swirl in a solution of 1 teaspoon of unscented bleach to 1 litre (1 quart) of water, and rinse again.
Fill with tap water, close tightly, label with the date, store somewhere cool and dark, and replace every 6 months.`, ""},
	{"Batteries", "Batteries, inverters and what they will run", "US Department of Energy guidance", `Stored energy in watt-hours is the battery's amp-hours times its voltage: a 100 Ah 12 V battery holds about 1,200 Wh.
You can safely use only part of it: about half for a lead-acid battery, about 80 to 90% for a lithium (LiFePO4) one. An inverter loses roughly another 10 to 15%.
A fridge's watt rating is what it draws while the compressor runs, but it runs only part of the time. Its daily use is on its label as kWh per year: divide by 365. Many fridges use 1 to 2 kWh a day.
So divide usable watt-hours by the fridge's daily watt-hours to get days of running. Recharge before a lead-acid battery goes below half full, or it wears out fast.
Charging a lead-acid battery gives off hydrogen, which can explode: charge it somewhere well ventilated, away from flames, sparks and smoking, connect the charger before switching it on and switch it off before disconnecting, and stop if the battery gets hot, swells or smells of rotten eggs.`, ""},
	{"Power outage", "Power cut", "FEMA and Red Cross guidance", `Nobody, this library included, can say when the power will come back. Report the outage to the electricity company if you can reach it, and follow its outage updates or local radio.
Meanwhile keep the fridge and freezer closed, unplug electronics so a surge does not damage them when power returns, and use torches rather than candles.
Never run a generator, grill or camp stove indoors.
Check on neighbours who are older or rely on powered medical equipment. If someone's equipment cannot wait, go somewhere with power or call emergency services.`, ""},
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
	{regexp.MustCompile(`(?i)(purif|disinfect|treat|filter|safe to drink|drinkable|potable|boil).{0,40}water|water.{0,70}(purif|disinfect|safe to drink|drinkable|potable|boil)|(creek|stream|river|pond|lake|rain|well|muddy|cloudy|dirty|brown) ?water|water.{0,20}(muddy|cloudy|dirty|brown)|bleach.{0,30}(water|gallon|litre|liter)`), "Water purification"},
}

// Notes returns the notes for the emergencies a question names, as sources
// without labels. It returns nothing unless UseNotes is set.
// supersedes lists notes whose topic, when it matches, removes a more general
// note that would contradict it, since every model copies a note whole.
var supersedes = map[string][]string{
	// The shock note says to push the source away with something dry and
	// wooden; with a line on a car, everyone must stay clear and inside.
	// The household shock note says to push the source away with wood:
	// deadly near a power line.
	"Downed power line":         {"Electrical injury"},
	"Family reunification":      {"Finding a person"},
	"Power line on vehicle":     {"Electrical injury", "Downed power line"},
	"Nosebleed":                 {"Bleeding"},
	"Shock":                     {"Fainting"},
	"Toxic gas":                 {"Burn"},
	"Generator":                 {"Power outage"},
	"Carbon monoxide poisoning": {"Dehydration"},
	"Wood stove smoke":          {"Starting a fire", "Smoke inhalation"},
	"Seizure":                   {"Eye injury"},
	"Food safety":               {"Power outage"},
	"Hypoglycemia":              {"Sudden confusion"},
	"Winter car":                {"Winter heating"},
	// "boiling water went on her arm" is a scald, not drinking water.
	"Burn": {"Water purification"},
	// Being sick after a head injury is a warning sign of the injury.
	"Head injury": {"Dehydration"},
	// An unrousable drunk person needs the recovery position, not legs up.
	"Alcohol poisoning": {"Fainting", "Dehydration"},
	"Recovery position": {"Fainting"},
	// A severe bleed needs pressure now, not a guide to stitches.
	"Bleeding": {"Cut needing stitches"},
	// The overdose note has the recovery position and naloxone, which
	// must come first.
	"Opioid overdose": {"Recovery position", "Alcohol poisoning"},
	// Gasping is not breathing: that person needs CPR, not the recovery
	// position.
	"Cardiopulmonary resuscitation": {"Recovery position"},
	// Notes whose words borrow from another: a jellyfish sting "burning",
	// a tick someone wants to "burn off", bleach for drinking water, a
	// person jerking on a live cable.
	"Jellyfish sting":    {"Burn"},
	"Box jellyfish":      {"Jellyfish sting"},
	"Chimney fire":       {"Starting a fire"},
	"Tick":               {"Burn"},
	"Water purification": {"Toxic gas"},
	"Electrical injury":  {"Seizure"},
}

// everydayRE recognises a question about something other than an emergency
// that only borrows an emergency word: a cocktail called a hurricane, a
// Minecraft tornado mod, a laptop "having a seizure", bleeding a radiator.
// urgentRE keeps the notes when the question also sounds like a real
// emergency ("my kid ate a mushroom from the garden, help").
var everydayRE = regexp.MustCompile(`(?i)\b(recipes?|cake|cocktail|risotto|movie|film|album|song|band|spotify|netflix|quotes?|game|gaming|minecraft|fortnite|roblox|mod|toys?|lego|theme park|ride at|rollercoaster|course|certification|class(es)? near|price|best deals|cheapest|where to buy|shopping|photoshop|excel|chrome|browser|software|goats?|radiators?|brakes|chainsaw|mower|traffic noise|noise at night|seat hinge|soft close|tattoo|costume|meme|joke|lyrics)\b`)

var urgentRE = regexp.MustCompile(`(?i)\b(help|now|urgent|emergency|911|999|112|ambulance|hospital|bleed(ing)? (badly|a lot|everywhere)|not breathing|unconscious|unresponsive|collapsed|swallowed|\bate\b|drank|bitten|stung|burned|burnt|hurt|injur|choking on|can.?t breathe|passed out|baby|toddler|child|kid|son|daughter|husband|wife|mum|mom|dad|nan|gran|blood|bleeding|spurt\w*|pumping|pouring|gushing|cut (my|his|her|their)|gash|wound|broke|broken|fell|pain|agony|screaming|kicked back|thigh|leg|arm|hand|finger)\b`)

func Notes(q string) []Source {
	if !UseNotes {
		return nil
	}
	if everydayRE.MatchString(q) && !urgentRE.MatchString(q) {
		return nil
	}
	topics := EmergencyTopics(q)
	for _, t := range noteTopics {
		if t.re.MatchString(q) && !contains(topics, t.topic) {
			topics = append(topics, t.topic)
		}
	}
	for specific, general := range supersedes {
		if contains(topics, specific) {
			topics = slices.DeleteFunc(topics, func(t string) bool { return slices.Contains(general, t) })
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
