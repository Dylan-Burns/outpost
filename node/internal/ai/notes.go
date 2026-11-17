package ai

import (
	"regexp"
	"slices"
	"strings"
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
var babyRE = regexp.MustCompile(`(?i)\b(baby|babies|infant|newborn|bub|bubs|bubba|pickney|pikin|picken|few months|wee one|in the cot|in her cot|in his cot|\d+[- ]?(day|week|month)s?[- ]old|(one|two|three|four|five|six|seven|eight|nine|ten|eleven)[- ](day|week|month)s?[- ]old|(\d|1[01]|one|two|three|four|five|six|seven|eight|nine|ten|eleven) ?(months?|mths?|mos?)\b)\b`)

// notBabyRE undoes a "months" that is not a baby's age.
var notBabyRE = regexp.MustCompile(`(?i)\b(\d+|one|two|three|four|five|six|seven|eight|nine|ten|eleven) ?(months?|mths?) (ago|pregnant|along|since|later|back|of (the|this)|supply|worth)|months? (pregnant|along)`)

// isBaby reports whether a question is about a baby under one year.
func isBaby(q string) bool {
	if !babyRE.MatchString(q) {
		return false
	}
	// "my wife is 7 months pregnant", "2 months ago": the word "baby" or
	// an age in days or weeks still counts.
	if notBabyRE.MatchString(q) {
		return regexp.MustCompile(`(?i)\b(baby|infant|newborn|bub|bubba|wee one|\d+[- ]?(day|week)s?[- ]old)\b`).MatchString(q)
	}
	return true
}

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
Chemical burn: brush off any dry chemical, take off contaminated clothing, then rinse with plenty of running water for at least 20 minutes. Do not try to neutralise it with vinegar, baking soda or anything else: that can make the burn worse.`, ""},
	{"Poisoning", "Swallowed poison or too many pills", "NHS, Red Cross and poison centre guidance", `Call emergency services or a poison centre straight away, even if the person seems well.
Find out what was swallowed, how much and when, and keep the container or packet to show.
With paracetamol (acetaminophen), people often feel fine at first while the liver is being damaged: they need hospital treatment even if they seem well.
Do not make the person vomit, and do not give them anything to eat or drink unless a poison centre or medical professional tells you to.
If they are unconscious but breathing, put them in the recovery position (on their side) and keep checking their breathing. If they stop breathing normally, start CPR.
If a chemical got on the skin or in the eyes, rinse with plenty of running water.
A swallowed button battery, or one that may have been swallowed, is an emergency even if the child seems well: it can burn through the food pipe within 2 hours. Go to the emergency department now. Do not make them sick and do not wait for symptoms.
Swallowed magnets: more than one magnet, or a magnet and anything metal, can pull together through the bowel wall and tear it. Go to the emergency department now, even if the child seems well.`, ""},
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
If they become unresponsive and stop breathing normally, start CPR.
Chest pain after cocaine, speed or other stimulants is an emergency too: call emergency services and tell them what was taken.`, ""},
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
	{"Medicines in pregnancy", "Medicines in pregnancy", "NHS, ACOG and US FDA guidance", `Paracetamol (acetaminophen) is the usual first choice for pain or fever in pregnancy: take the lowest dose that works for the shortest time.
Do not take ibuprofen, naproxen or other anti-inflammatory painkillers, or aspirin (unless a doctor has prescribed low-dose aspirin), especially from 20 weeks: they can harm the baby's kidneys and heart.
Ask a pharmacist, midwife or doctor before taking any other medicine, including cold and flu remedies, codeine, herbal remedies and antihistamines.
A high temperature, a severe headache, or pain that does not settle needs a call to your midwife or doctor, not only painkillers.`, ""},
	{"Battery fire risk", "Swollen, hot or smoking battery", "US CPSC, NFPA and fire service guidance", `A swollen, very hot, hissing or smoking lithium battery (phone, laptop, power bank, vape, e-bike, e-scooter) can catch fire or explode.
Stop charging and unplug it, and switch the device off if you can do so without pressing on the battery. Do not puncture, press, bend or try to remove a swollen battery yourself.
If it is not smoking and you can move it safely, put it on a non-flammable surface (concrete, a metal tray, sand) away from anything that burns, outside if possible, and keep people and pets away.
If it smokes, hisses, sparks or catches fire, leave the room, close the door, get everyone out and call the fire service. Do not use water on a large lithium battery fire.
Take the device to a repair shop or a battery recycling point; never put it in household rubbish.`, ""},
	{"Fish hook", "Fish hook stuck in the skin", "NHS, Red Cross and wilderness first aid guidance", `Do not yank it out: a barbed hook tears the skin. Cut the fishing line, keep the hook still, and cover it loosely.
Get it removed at a minor injuries unit, urgent care centre or doctor's surgery, especially if it is deep, past the barb, or in the face, eye, a joint, or a tendon. A hook in or near the eye is an emergency: do not touch it, cover both eyes loosely and call emergency services.
Afterwards, wash the wound with soap and water and watch for infection (spreading redness, swelling, pus, fever). Check that tetanus jabs are up to date; get a booster if the last one was more than 10 years ago.`, ""},
	{"Older person unwell", "An older person going downhill", "NHS and Age UK guidance", `An older person who has stopped eating or drinking, or who is suddenly more confused, sleepy or unsteady than usual, may have an infection (often a urine or chest infection), dehydration, or a side effect of medicine. These can become serious quickly.
Call their doctor or a nurse line (NHS 111) today. Call emergency services if they are very hard to wake, struggling to breathe, have a fever with shivering or blotchy skin, have not passed urine all day, or have a new weakness on one side or slurred speech.
Meanwhile offer small sips of fluid often, keep them warm and safe from falls, and make a list of their medicines and when they last ate, drank and passed urine.`, ""},
	{"Testicle pain", "Sudden pain in a testicle", "NHS and American Urological Association guidance", `Sudden, severe pain in a testicle, or swelling with pain in the scrotum or lower belly, can be a twisted testicle (testicular torsion). Go to the emergency department straight away, or call emergency services; do not wait for a doctor's appointment. Surgery within a few hours can save the testicle.
This applies to boys and men of any age, including after a knock or exercise, and even if the pain eases.`, ""},
	{"Bulging fontanelle", "Bulging soft spot in a baby", "NHS and NICE guidance", `A soft spot (fontanelle) on top of a baby's head that bulges or feels tense when the baby is calm and upright, especially with a fever, vomiting, a high-pitched cry, unusual sleepiness or floppiness, can mean meningitis or pressure in the head. Call emergency services or go to the emergency department now.
A sunken soft spot can mean dehydration: get urgent medical advice the same day.`, ""},
	{"Aortic dissection", "Sudden tearing pain in the chest or back", "NHS and American Heart Association guidance", `A sudden, severe, tearing or ripping pain in the chest, back or belly, especially in an older person or someone with high blood pressure, can be a tear in the body's main artery (aortic dissection). Call emergency services now.
Keep them lying still and calm. Do not give aspirin, food or drink. If they become unresponsive and stop breathing normally, start CPR.`, ""},
	{"Belly injury", "Belly injury after a fall or blow", "NHS and American College of Surgeons guidance", `After a fall, crash or blow to the belly (bike handlebars, a trampoline frame, a seatbelt), bleeding inside can start slowly and show up hours later. Call emergency services or go to the emergency department now if the belly is swollen, tight or very tender, if they are pale, sweaty, faint or unusually sleepy, if they vomit, if there is blood in their urine, or if pain spreads to the shoulder.
Keep them lying still and warm and give them nothing to eat or drink until they have been checked.`, ""},
	{"Vital medicine running out", "Running out of a vital medicine", "NHS, CDC and pharmacist guidance", `Do not stop these suddenly or ration them without advice: insulin (type 1 diabetes always needs some insulin), steroid tablets such as prednisolone or hydrocortisone, epilepsy medicines, heart and blood thinning medicines, and many antidepressants. Stopping can be dangerous within a day or two.
Ask any pharmacist, shelter or evacuation centre medical staff, or emergency doctor for an emergency supply: take the packets or a list of names and doses. Do not take someone else's prescription unless a professional tells you to.
Call emergency services if someone who has missed steroids is vomiting, very weak, confused or collapses; someone has a seizure; a person with type 1 diabetes without insulin is vomiting, breathing fast or drowsy; or there is chest pain or severe breathlessness.`, ""},
	{"High blood sugar", "High blood sugar and diabetic ketoacidosis", "NHS and American Diabetes Association guidance", `Signs of high blood sugar: very thirsty, peeing a lot, tired, blurred vision. Keep drinking water or sugar-free drinks, keep taking diabetes medicine and insulin (even when ill or not eating), and check sugar and ketones if you can.
Call emergency services now if someone with diabetes, especially type 1, is vomiting, has tummy pain, is breathing fast and deep, has breath that smells fruity or like pear drops, is very drowsy or confused, or has high ketones. This can be diabetic ketoacidosis, which can kill within hours.
Insulin keeps at room temperature (below about 30 °C) for about 4 weeks once out of the fridge. Keep it out of the sun and never let it freeze.`, ""},
	{"Seriously ill child", "Is my child seriously ill?", "NHS and NICE traffic-light guidance", `Call emergency services now if a child: has blue, grey, very pale or blotchy skin or lips; is struggling to breathe, grunting, breathing very fast, or the skin pulls in between the ribs; is floppy, unresponsive, or cannot stay awake; has a weak, high-pitched or nonstop cry; has a fit; or has a rash that does not fade under a glass.
Get urgent medical advice the same day if they are not drinking, have had no wee or wet nappy for 12 hours, have a temperature of 39 °C (102.2 °F) or more and are under 6 months, or you are worried they are getting worse.
Otherwise: give plenty of fluids, keep them comfortable (not wrapped up), check on them during the night, and trust your instinct if something feels wrong.`, ""},
	{"Severe belly pain", "Sudden severe belly pain", "NHS guidance", `Call emergency services now for sudden, severe belly pain, especially if the belly is hard or rigid, the person faints or looks pale and sweaty, vomits blood or green bile, or has pain with vomiting and no poo or wind.
In anyone who could be pregnant, belly pain on one side with bleeding, fainting or pain at the tip of the shoulder can be an ectopic pregnancy: call emergency services.
Pain that starts around the belly button and moves to the lower right, worse when moving or coughing, may be appendicitis: get seen urgently the same day. Do not take laxatives, and do not eat until a doctor has seen them.`, ""},
	{"Blood clot", "Possible blood clot (leg or lung)", "NHS and American Heart Association guidance", `Call emergency services now for sudden breathlessness, sharp chest pain that is worse when breathing in, coughing up blood, or collapse: a blood clot in the lung can kill quickly.
A swollen, painful, warm or red calf or thigh, usually in one leg, can be a clot in the leg (DVT): get seen the same day. Do not massage the leg.
Clots are more likely after surgery, a long journey or time in bed, in pregnancy or after birth, on the contraceptive pill, or with cancer.`, ""},
	{"Thunderclap headache", "Sudden, worst-ever headache", "NHS guidance", `Call emergency services now for a sudden, severe headache that peaks within about a minute (often called the worst headache ever), especially with a stiff neck, vomiting, confusion, fainting, a fit, weakness, or trouble speaking. It can be bleeding in the brain.
Also call for a headache after a head injury with vomiting or drowsiness, or with fever and a rash that does not fade or a stiff neck.`, ""},
	{"Alcohol withdrawal", "Stopping heavy drinking suddenly", "NHS and NIAAA guidance", `Someone who drinks heavily every day can become dangerously ill within 6 to 72 hours of stopping suddenly: shaking, sweating, anxiety, then sometimes fits, seeing or hearing things, confusion and fever (delirium tremens).
Call emergency services if they have a fit, become confused, see or hear things that are not there, or have a fever.
If you drink heavily, do not stop suddenly on your own: cut down gradually or get medical help to stop. The same applies to sleeping pills and benzodiazepines such as diazepam: do not stop them suddenly.`, ""},
	{"Ice rescue", "Someone has fallen through ice", "RNLI, Red Cross and Royal Life Saving guidance", `Call emergency services. Do not go onto the ice: many rescuers die this way.
Shout to them to stay calm, keep their head up, kick their legs and pull themselves forward flat onto the ice the way they came, then roll or crawl away rather than standing.
From the bank, reach with a long branch, pole, rope or scarf, or throw something that floats; lie down so you are not pulled in.
Once out, get them somewhere warm, remove wet clothes and warm them gently; they need checking for hypothermia and water in the lungs.`, ""},
	{"Trapped person", "Someone trapped or crushed", "FEMA, Red Cross and St John guidance", `Call emergency services and tell them how long the person has been trapped.
Do not try to lift heavy rubble or a vehicle if it could collapse further or crush you. If someone has been crushed under a heavy weight for more than about 15 minutes, releasing them can be dangerous: wait for emergency services unless their life is in immediate danger.
Reassure them, keep them warm, press on any wound that is bleeding, and do not move them if their neck or back may be hurt.
If you are trapped: cover your mouth with cloth, move as little as possible, tap on a pipe or wall or use a whistle so rescuers can find you, and shout only as a last resort.`, ""},
	{"Stab or gunshot", "Stab or gunshot wound", "NHS, Red Cross and St John guidance", `Call emergency services. Press hard on the wound with a clean cloth and keep pressing.
If a knife or object is still in the wound, do not pull it out: press around it.
For a wound to the chest, if it bubbles or sucks air, leave it open or cover it loosely without sealing it, and help them sit in the position that makes breathing easiest.
Lie them down if they feel faint, keep them warm, and give nothing to eat or drink. If they stop breathing normally, start CPR.`, ""},
	{"Vomiting blood", "Vomiting blood or black poo", "NHS guidance", `Call emergency services now if someone vomits a lot of blood, or vomit that looks like coffee grounds, or has black, tarry or very bloody poo, especially if they feel dizzy, faint, pale or sweaty, or take blood thinners. This can be bleeding inside.
Give them nothing to eat or drink. Lay them on their side if they are drowsy, and keep a sample or describe it to the medics.`, ""},
	{"Radiation emergency", "Radiation or nuclear emergency", "Ready.gov, CDC and UK Government guidance", `Get inside a solid building straight away (brick or concrete), go to the middle or the basement, close windows and doors, and turn off fans and air conditioning. Stay inside for at least 24 hours unless officials tell you otherwise: going outside is the most dangerous thing you can do.
If you were outside, take off your outer clothing, put it in a bag, and wash with soap and water (no conditioner).
Listen to the radio for instructions. Take potassium iodide tablets only if officials tell you to. Never drink iodine antiseptic or bleach.`, ""},
	{"Chemical cloud", "Chemical leak or toxic cloud", "Ready.gov and CDC guidance", `Go inside straight away, close all windows and doors, and turn off fans, air conditioning and heating that draw in outside air. Go to an upper inside room with few windows (many chemicals are heavier than air) and seal gaps with wet towels or tape.
If you are outside and cannot get in, move away upwind and uphill. If a chemical got on you, take off your clothes and wash with soap and water.
Listen to the radio and follow official instructions. Call emergency services if anyone has trouble breathing, burning eyes or throat, or collapses.`, ""},
	{"Infant feeding", "Feeding a baby in an emergency", "CDC, WHO and UNICEF guidance", `Breastfeeding is safest in an emergency: keep going, as often as the baby wants.
For formula, ready-to-feed formula is best when tap water may be unsafe. Otherwise make powdered formula only with water that has been boiled, or with bottled water. Never use flood water, and never add extra water to make formula last longer.
Throw away any feed the baby has not finished within about an hour, and clean bottles and teats with boiled water.
Get urgent help if the baby is floppy, unusually sleepy, has very few wet nappies, or has a sunken soft spot.`, ""},
	{"Altitude sickness", "Altitude sickness", "NHS and Wilderness Medical Society guidance", `Headache, nausea, dizziness or tiredness above about 2,500 metres (8,000 feet) is altitude sickness until proven otherwise: stop climbing and rest; do not go higher until it is better.
Go down straight away, at least 300 to 1,000 metres, and get help if they become confused, clumsy or unsteady, are breathless at rest, or cough up frothy spit. These can kill within hours. Never leave them alone.`, ""},
	{"Dental abscess", "Tooth abscess and a swollen face", "NHS guidance", `Call emergency services or go to the emergency department if facial swelling spreads to the eye or neck, or there is trouble breathing, swallowing or opening the mouth, or a high fever.
Otherwise see an urgent dentist as soon as possible. Painkillers from the packet can help. Rinse with warm salty water. Do not put aspirin on the gum (it burns) and do not try to drain it yourself.`, ""},
	{"Poisonous plants", "Wild plants and berries", "Poison centre and US Forest Service guidance", `Do not eat wild plants, berries or roots unless you are completely sure what they are: some common ones, such as water hemlock, foxglove, yew and deadly nightshade, can kill. Taste tests are not safe.
If someone has eaten one, call a poison centre or emergency services. Keep a sample of the plant. Do not make them sick.
Call emergency services now for a fit, drooling, vomiting, confusion, a slow or irregular pulse, or collapse.`, ""},
	{"Spider or scorpion", "Spider bites and scorpion stings", "Poison centre, CDC and ANZCOR guidance", `Most bites and stings only need washing and a cold pack. Call a poison centre for advice.
Call emergency services if a child is stung by a scorpion and has twitching, roving eyes, drooling or trouble breathing, or if anyone has severe spreading pain, cramps, sweating, vomiting or trouble breathing after a bite.
In Australia, treat a funnel-web spider bite like a snakebite: a firm pressure bandage over the bite and up the limb, keep still, call emergency services. For a redback bite, do not bandage: use a cold pack and seek medical help.`, ""},
	{"Rip current", "Caught in a rip current", "NOAA, RNLI and Surf Life Saving guidance", `If you are caught: do not swim against it. Float on your back, raise a hand and shout for help. When you can, swim parallel to the beach until you are out of the current, then swim back in.
If you see someone caught: do not go in after them. Call a lifeguard or emergency services, and throw them something that floats.`, ""},
	{"Severed body part", "Finger or body part cut off", "NHS and St John guidance", `Call emergency services. Press firmly on the wound with a clean cloth and raise it.
Wrap the cut-off part in cling film or a clean plastic bag, then put it in a bag or box with ice, so the part does not touch the ice directly. Do not soak it in water or clean it. Take it with the person.`, ""},
	{"Sudden vision loss", "Sudden loss of sight or a painful red eye", "NHS guidance", `Go to the emergency department or eye casualty now for sudden loss of sight, a curtain or shadow across the vision, new flashes with lots of new floaters, or a painful red eye with headache, blurred vision and vomiting. Call emergency services if sight loss comes with weakness, a drooping face or slurred speech.
Do not wait to see if it gets better.`, ""},
	{"Bee swarm", "Attacked by a swarm of bees or wasps", "CDC NIOSH and Poison Control guidance", `Run straight away to a car or building, covering your face. Do not swat, and do not jump into water: they wait for you to come up.
Scrape stings out with a fingernail or card, then wash and use a cold pack.
Call emergency services if there are many stings (especially in a child or older person), swelling of the face, lips or throat, trouble breathing, dizziness or vomiting.`, ""},
	{"Active attacker", "Someone attacking people nearby", "Ready.gov and UK Counter Terrorism Policing guidance", `Run if there is a safe way out: leave your belongings and help others to leave if you can.
Hide if you cannot get away: lock or block the door, turn off lights, silence your phone and stay out of sight.
Call emergency services when it is safe. Fight only as a last resort, if your life is in danger. When police arrive, keep your hands visible and follow their instructions.`, ""},
	{"Weapons refusal", "Making explosives or weapons", "Outpost", `Outpost cannot help make explosives, weapons or anything meant to hurt or knock out a person.
For tree stumps, a hired stump grinder, digging out the roots, or a tree surgeon are safe ways to clear them. If you are worried that someone means to hurt themselves or others, call emergency services or the police.`, ""},
	{"Stimulant overheating", "Overheating after ecstasy or party drugs", "NHS, FRANK and Drugs and Alcohol guidance", `Call emergency services now if someone who has taken ecstasy (MDMA), speed or another stimulant is very hot, confused, agitated, having a fit, has a racing or irregular heart, or collapses. Tell the call handler what was taken: they will not involve the police just for that.
Move them somewhere cool and quiet, loosen clothing, cool their skin with cool damp cloths, and stay with them.
Let them sip water, no more than about a pint (500 ml) an hour. Drinking a lot of water fast after ecstasy can dangerously lower the salt in the blood: call emergency services if they keep drinking and become confused, sick or drowsy.`, ""},
	{"Confined space", "Someone collapsed in a tank, pit, silo or grain bin", "HSE, OSHA and farm safety guidance", `Do not go in after them: the air in tanks, pits, wells, sewers, slurry pits and silos can kill in seconds, and grain can swallow a rescuer. Many deaths are would-be rescuers.
Call emergency services now and say it is a confined space or grain bin rescue. Keep everyone else out.
Shut off any machinery or augers and lock them off. If you can, ventilate from outside by opening hatches or doors, and talk to the person from outside. Only trained rescuers with breathing equipment should go in.`, ""},
	{"Forged documents", "Fake documents", "Outpost", `Outpost cannot write fake or forged documents, such as doctor's notes or official letters.
If you were ill, in the UK you can self-certify for up to 7 days, and a GP or NHS 111 can give a fit note after that; elsewhere, ask a doctor or pharmacist for a note, or ask your employer what they accept.`, ""},
	{"Bee or wasp sting", "Bee or wasp sting", "NHS and Red Cross guidance", `Scrape a bee sting out sideways with a fingernail or the edge of a card as soon as you can; do not squeeze it with tweezers. Wash the area, hold a cold pack on it for 10 minutes and raise it; painkillers or an antihistamine can help.
Call emergency services if there is swelling of the face, lips, tongue or throat, trouble breathing or swallowing, wheezing, dizziness or fainting, or many stings, and use an adrenaline auto-injector if the person has one.`, ""},
	{"Medicine toxicity when ill", "Lithium and other medicines during sickness", "NHS and BNF guidance", `Vomiting, diarrhoea or not drinking can make some medicines build up to dangerous levels: lithium, digoxin, metformin, some blood pressure and water tablets (ACE inhibitors, ARBs, diuretics), and some diabetes medicines.
Call emergency services if someone on lithium becomes shaky, unsteady, confused, very drowsy, has slurred speech, or has a fit: these are signs of lithium poisoning.
Otherwise contact their doctor, pharmacist or NHS 111 today: they may need a blood test or to pause some tablets for a day or two (sick day rules). Keep sipping fluids. Do not stop or change lithium without advice.`, ""},
	{"Decompression sickness", "After diving: joint pain, tingling or breathlessness", "Divers Alert Network and NHS guidance", `Joint pain, tingling or numbness, unusual tiredness, a rash, dizziness or breathlessness within 48 hours of scuba diving can be decompression sickness (the bends), even after a dive within limits.
Do not fly or go up into mountains: lower air pressure makes it worse. Call emergency services or a diving emergency line (such as DAN) now; it is treated in a hyperbaric chamber.
Lie them down, give 100% oxygen if a kit is available, give water to drink if they are fully awake, and keep a record of the dives.`, ""},
	{"Pressure sore", "Pressure sore that will not heal", "NHS and NICE guidance", `Take the pressure off the sore: change position at least every 2 hours (or more often), use cushions or a pressure-relieving mattress, and avoid sitting or lying on the sore.
Keep the skin clean and dry and cover broken skin with a clean dressing. Do not massage it. Contact a doctor, district nurse or nurse line soon: broken skin needs a nurse to assess it.
Get urgent help the same day if the sore smells, oozes pus, has spreading redness or swelling, or the person has a fever, shivering or confusion, which can mean infection or sepsis.`, ""},
	{"Bleeding after birth", "Heavy bleeding after giving birth", "NHS and RCOG guidance", `Call emergency services or the maternity unit now if, at any time in the weeks after birth, bleeding suddenly gets heavier, soaks a pad in an hour or less, or there are clots bigger than a 50p coin or a plum, or if she feels faint, dizzy, has a racing heart, or bleeding comes with fever, smelly discharge or tummy pain.
While waiting, have her lie down with her feet raised, keep her warm, and keep the pads to show the medics. If she is breastfeeding or holding the baby, have someone else take the baby.`, ""},
	{"Too much insulin", "Too much insulin (a doubled or extra dose)", "NHS and diabetes charity guidance", `Too much insulin can cause a dangerous low blood sugar for hours, and for a long-acting insulin (such as Lantus, Levemir, Tresiba, Toujeo) up to a day or more.
Contact the diabetes team, a doctor or NHS 111 now for advice. Do not take another dose to make up or "cancel" it, and do not skip meals: eat regular meals and extra starchy snacks, and keep fast sugar (glucose tablets, a sugary drink or sweets) next to you.
Check blood sugar every 2 to 4 hours, including overnight, and do not be alone. If sugar falls low, take fast sugar straight away. Call emergency services if they become confused, very drowsy, cannot swallow safely, or have a fit.`, ""},
	{"Self-harm", "When someone is self-harming", "NHS, Mind and Samaritans guidance", `Stay calm and do not confront, shout or punish: that makes hiding more likely. Choose a quiet moment, say what you have noticed, that you care, and that you want to understand; listen more than you talk.
Check any wounds: clean them with water and cover with a clean dressing. Get urgent medical help for deep or gaping cuts, heavy bleeding, burns, or anything swallowed.
Ask directly whether they are thinking about suicide. If they are, or you are worried for their safety now, call emergency services or a crisis line (988 in the US, Samaritans 116 123 in the UK and Ireland).
Book a GP appointment together; young people can be referred to mental health services. Look after yourself too.`, ""},
	{"Venomous fish sting", "Weever fish, stingray or other venomous fish sting", "NHS and Surf Life Saving guidance", `Get out of the water. Soak the stung part in water as hot as can be tolerated (test it with your own hand first, never scalding) for 30 to 90 minutes: heat breaks down the venom and eases the pain. Painkillers help.
Remove any visible spines with tweezers, then clean the wound. Get medical help if pain is severe or lasts, a spine is left in, the wound looks infected, or for a stingray wound to the chest or belly.
Call emergency services if they have trouble breathing, swelling of the face or throat, chest pain or collapse.`, ""},
	{"Illegal request", "Taking or using what is not yours", "Outpost", `Outpost cannot help take or use other people's property or bypass meters or locks.
If you are short of fuel, power or supplies in an emergency, ask neighbours openly, contact local emergency services, a community centre or shelter, or your supplier's emergency line.`, ""},
	{"Drink spiking", "If your drink may have been spiked", "NHS, police and Drinkaware guidance", `Tell someone you trust and stay with them, and tell bar or venue staff. Do not drink any more and do not go home alone or with someone you do not know.
Call emergency services if you or they become very drowsy, confused, cannot stand, are sick and drowsy, have trouble breathing, or collapse; put an unconscious person who is breathing on their side.
Report it to the police as soon as you can: some drugs only show up in urine or blood for a day or so, so ask for a test early.`, ""},
	{"Abuse and threats", "Abuse, threats or being followed", "Police, Refuge and National Domestic Violence Hotline guidance", `If you are in danger now, call emergency services (in the UK, if you cannot speak, call 999 and then press 55 when asked). Go to a public place or somewhere with other people if you are being followed.
Keep evidence: messages, call logs, photos of injuries, dates and times. Tell someone you trust.
Get help to plan how to leave safely, including for children: in the US, the National Domestic Violence Hotline (1-800-799-7233, or text START to 88788); in the UK, the Refuge helpline (0808 2000 247); elsewhere, the police or a local domestic abuse service. The police and courts can issue protection or restraining orders.`, ""},
	{"Child protection", "If you are worried a child is being harmed", "NSPCC, Childhelp and police guidance", `If a child is in immediate danger, call emergency services now.
Otherwise report your concern to the police or to children's services; you do not need proof. In the UK, the NSPCC helpline is 0808 800 5000; in the US, the Childhelp hotline is 1-800-422-4453. For online grooming, save the messages and report them to the police (in the UK, also to CEOP).
If a child tells you something, stay calm, listen, tell them they did the right thing, and do not promise to keep it secret.`, ""},
	{"Bad trip", "Helping someone through a bad trip", "NHS and FRANK guidance", `Stay with them in a quiet, calm, dimly lit place, away from crowds. Speak gently, remind them the feeling is caused by the drug and will pass, and do not leave them alone.
Call emergency services if they are very hot, have a fit, chest pain, trouble breathing, are unconscious or very hard to wake, or might hurt themselves or others. Tell the call handler what was taken.
Do not give them any other drugs or alcohol. Sips of water are fine.`, ""},
	{"Suspicious package", "A suspicious package or bag", "Police and Ready.gov guidance", `Do not touch, move, open, shake or smell it. Move everyone well away from it, at least behind solid walls, and keep people away.
Call the police or emergency services from a safe distance, and do not use a radio or mobile phone right next to it.
If something has already leaked a powder or liquid onto you, do not brush it off with your hands: take off outer clothing, wash with soap and water, and tell the emergency services.`, ""},
	{"Harming someone", "Hurting another person", "Outpost", `Outpost cannot help harm, poison or endanger another person, or make it look like an accident.
If you are afraid you might hurt someone, step away from them and from anything you could use, and call emergency services or a crisis line now; in the US you can call or text 988. If someone is in danger, call emergency services or the police.`, ""},
	{"Cord prolapse", "The cord comes out before the baby", "NHS and RCOG guidance", `Call emergency services now and say the cord is coming out before the baby: this is an emergency for the baby.
Get on your knees and elbows with your bottom raised higher than your head and chest (or lie on your side with a pillow under your hips). This takes the baby's weight off the cord. Stay like this, including on the way to hospital.
Do not push the cord back in and handle it as little as possible. If it is outside, keep it warm and moist with a clean damp cloth.`, ""},
	{"Landslide", "Signs of a landslide or mudslide", "USGS, FEMA and Ready.gov guidance", `Leave now if the ground is cracking or bulging, fences, walls, poles or trees are tilting or moving, doors or windows suddenly stick, you hear cracking, rumbling or trees snapping, or a stream suddenly turns muddy or stops flowing. Move sideways, out of the path, to higher, stable ground; do not go back for belongings.
Tell your neighbours and call the emergency services or the local council so others can be warned. Stay away from the slide area afterwards: more slides can follow, especially in heavy rain.
After heavy rain, watch for these signs and be ready to leave quickly, especially at night.`, ""},
	{"Signalling for rescue", "Signalling to rescuers", "Search and rescue and Ready.gov guidance", `Stay where you are if you can be found there: rescuers search from your last known position, and a person who stays put is found sooner.
Make yourself big and bright: spread bright clothing, a tarp or a sheet on a roof or in the open, wave something bright, and at night use a torch or phone light. Three of anything (three whistle blasts, three flashes, three fires in a triangle) means "help".
Save your phone battery: turn it off between tries, lower the brightness, and send a text with your location, since a text can get through when a call cannot. Call your emergency number even with no signal from your network.`, ""},
	{"Psychosis", "Hearing voices, or feeling watched or poisoned", "NHS and Mind guidance", `If they may hurt themselves or someone else, or are not eating or drinking, get urgent help now: call emergency services or a crisis line (988 in the US; NHS 111, option 2 for mental health, in England).
Stay calm and with them. Speak simply and do not argue about what they hear or believe, but do not agree with it either: say you can see they are frightened and you want to help.
Help them eat and drink something they feel safe with, such as sealed food. A doctor or mental health team can help: this is an illness, and treatment works.`, ""},
	{"Toxic gas refusal", "Making toxic gas", "Outpost", `Outpost cannot help make a toxic gas: it can kill people and pets nearby and spreads further than expected.
For rats or mice, use snap traps or sealed bait stations placed where children and pets cannot reach, block holes with steel wool and cement, remove food and nesting material, or call a pest controller.
Never mix bleach with ammonia, vinegar or other acids or cleaners. If you breathe in fumes, get to fresh air and call emergency services if you are short of breath or coughing.`, ""},
	{"Contaminated water", "Water that smells of fuel or chemicals", "CDC, EPA and water utility guidance", `Do not drink, cook with, or wash food in water that smells or tastes of fuel, oil or chemicals, and do not give it to pets. Boiling does not make it safe: it can concentrate chemicals and release fumes, and filters and purification tablets do not remove them either.
Use bottled or stored water instead, and tell your water supplier or the local council; they will say when the supply is safe.
If someone has drunk it and is coughing, choking, drowsy or vomiting, call emergency services or a poison centre.`, ""},
	{"Missed dialysis", "Missed dialysis", "National Kidney Foundation, NHS and FEMA guidance", `Call emergency services now if they are short of breath (especially lying flat), have chest pain, an irregular or very slow heartbeat, muscle weakness, or confusion: fluid and potassium are building up and this can stop the heart.
Get to any dialysis unit or hospital as soon as you can: tell them you have missed treatment. Your unit or the emergency number can tell you which units are open.
Until then, on an emergency kidney diet: drink as little as you can, eat no salt or salty, processed or tinned food, and avoid high-potassium foods (bananas, oranges, potatoes, tomatoes, dried fruit, chocolate, nuts).`, ""},
	{"Volcanic ash", "Volcanic ash", "USGS, IVHHN and Red Cross guidance", `Stay indoors with windows, doors and vents closed. Outside, wear a well-fitting N95 or FFP2 mask (or a damp cloth if you have none) and goggles, and cover your skin. People with asthma or lung or heart disease should stay inside.
Do not drive if you can avoid it: ash is slippery and blocks visibility and engines. Keep ash out of water tanks: disconnect the downpipes, and do not drink water that looks or tastes of ash.
Clear ash from roofs early, because wet ash is heavy enough to collapse a roof, but only if it is safe: work with someone else, wear a mask, and never climb on a steep or fragile roof. Dampen ash lightly before sweeping it.`, ""},
	{"Baby in hot weather", "Keeping a baby cool in hot weather", "NHS and Red Cross guidance", `A baby under 6 months should have more breastfeeds or formula feeds, not water. Babies over 6 months can also have small sips of cooled boiled water.
Keep them out of the sun and in the coolest room; dress them in a nappy and a light vest, use a cool damp cloth on the skin, and never cover a pram with a blanket, which traps heat. Do not use ice or cold baths.
Get urgent medical help if a baby is floppy, very sleepy, has a temperature of 38 °C (100.4 °F) or more (under 3 months), has fewer wet nappies, a sunken soft spot, or is not feeding.`, ""},
	{"Older person at risk", "If you are worried an older person is being abused", "Age UK, Hourglass and US Adult Protective Services guidance", `If they are in danger now, call emergency services.
Signs include unexplained bruises, weight loss or hunger, being afraid of a carer, a carer keeping their money or bank card, or being kept from seeing others. They may ask you not to make a fuss: you can still get help.
Report it to the local council's adult social care (safeguarding) team or the police; in the US, Adult Protective Services or the Eldercare Locator (1-800-677-1116); in the UK, the Hourglass helpline (0808 808 8141). If a care agency employs the carer, tell the agency too.`, ""},
	{"After a disaster", "Sleep, nightmares and drinking after a disaster", "NHS, SAMHSA and Red Cross guidance", `Trouble sleeping, nightmares, flashbacks, feeling on edge or numb, and drinking more are common after a disaster. They often ease over weeks; if they are not easing after about a month, or are getting worse, help works.
Things that help: talking with people you trust, keeping some routine, sleep and meals at regular times, and cutting back on alcohol, which makes sleep and low mood worse.
Talk to a doctor or a disaster mental health service: in the US the Disaster Distress Helpline (call or text 1-800-985-5990); in the UK, your GP or NHS talking therapies. If you have thoughts of harming yourself, call emergency services or a crisis line.`, ""},
	{"Swallowed battery or magnets", "A swallowed button battery or magnets", "NHS, Great Ormond Street and US Poison Control guidance", `Go to the emergency department now, even if the child seems completely well: a button battery can burn through the food pipe within 2 hours, and two or more magnets (or a magnet and a piece of metal) can pull together through the gut and cause holes.
Do not make them vomit and do not give them anything to eat or drink. Take the packet or the rest of the set with you.
If a child over 1 may have swallowed a button battery and you are more than a few minutes from hospital, US Poison Control advises 2 teaspoons (10 ml) of honey every 10 minutes, up to 6 doses, on the way. Do not give honey to a baby under 1.`, ""},
	{"Serotonin syndrome", "Too much serotonin from mixing medicines", "NHS and US National Library of Medicine guidance", `Mixing an antidepressant (such as sertraline, fluoxetine, citalopram or venlafaxine) with tramadol, some migraine or cough medicines, St John's wort or MDMA can cause serotonin syndrome: sweating, shaking or twitching, a racing heart, fever, agitation or confusion.
Call emergency services now if they have a high temperature, muscle stiffness, jerking, a fit, confusion, or a very fast or irregular heartbeat: this can be life-threatening.
If it is mild, stop the extra medicine and get medical advice today (a pharmacist, doctor or NHS 111). Do not take any more of either medicine until you have been told it is safe.`, ""},
	{"Stranded with a vehicle", "Stranded with a vehicle in remote country", "Australian outback safety and US National Park Service guidance", `Stay with your vehicle: it is far easier for searchers to see than a person walking, it gives shade and shelter, and many people who leave their car to walk for help die of heat or dehydration.
Tell someone or call for help if you have any signal (try higher ground nearby, but come back). Make the car visible: open the bonnet or hood, lay out bright things, and use a mirror, lights or the horn when you hear or see searchers. At night, a small fire away from the car can be seen from far off.
In heat, rest in the shade in the day and save sweat, not water: drink what you need. Do not drink radiator water. If you must leave, only do it if you know exactly where help is and it is close, and walk in the cool of night or early morning.`, ""},
	{"Sickle cell crisis", "Sickle cell disease: when to go to hospital", "NHS and Sickle Cell Society guidance", `With sickle cell disease, go to hospital now (or call emergency services) for: a temperature of 38 °C (100.4 °F) or more; chest pain or trouble breathing (this can be acute chest syndrome, which is life-threatening); pain that home treatment does not control; sudden weakness, numbness, a drooping face or trouble speaking; a painful erection lasting more than 2 hours; very pale skin, unusual tiredness or a swollen tummy in a child; or severe headache.
Do not wait to see if a fever or chest pain settles: infection and chest problems get worse fast. Tell staff about the sickle cell disease straight away.
For a mild pain crisis at home: the usual painkillers, plenty of fluids, keep warm, and rest; get help if it does not ease.`, ""},
	{"Older person fallen", "An older person who has been on the floor for a long time", "NHS, Age UK and Red Cross guidance", `Call emergency services if they have been on the floor for an hour or more, are confused, cannot get up, are in pain (especially in the hip, back or neck), or hit their head. Lying a long time can cause dehydration, pressure damage, low body temperature and kidney damage from muscle breakdown, even when they seem all right.
Do not lift them yourself if they are hurt or cannot get up on their own. Keep them warm with a blanket, put a cushion under the head, and help them change position slightly if they can, to ease pressure.
Give small sips of water only if they are fully alert and can swallow, and do not give food or medicine until a medic has seen them. Note the time they fell, if known.`, ""},
	{"Postpartum psychosis", "Strange beliefs or hearing voices after giving birth", "NHS, Royal College of Psychiatrists and Action on Postpartum Psychosis guidance", `This can be postpartum psychosis, a medical emergency that can start suddenly in the days or weeks after birth. Call emergency services or get her to an emergency department now, especially if she hears voices, believes strange things about the baby, is very confused, or may harm herself or the baby.
Do not leave her alone with the baby until she has been seen. Stay calm and kind, and do not argue with what she believes.
It is an illness, not her fault, and with treatment most women recover fully.`, ""},
	{"Febrile seizure", "A fit with a fever in a young child", "NHS and Red Cross guidance", `Lay the child on their side on the floor, away from anything hard, and stay with them. Do not put anything in their mouth or hold them down. Time the fit.
Call emergency services if it is their first fit, it lasts more than 5 minutes, they do not wake up properly or have trouble breathing afterwards, or they have another fit. Otherwise get urgent medical advice the same day.
Do not put them in a cold or cool bath, sponge them, or wrap them up: cooling does not stop fits and can make them shiver. Take off extra clothing and keep the room comfortably cool. Being sleepy and floppy for up to an hour afterwards is common, but get help if they are still drowsy or hard to wake.`, ""},
	{"Emergency numbers", "Emergency numbers", "Government emergency service guidance", `US and Canada: 911. UK: 999 or 112. Ireland: 112 or 999. Australia: 000 (112 also works from mobiles). New Zealand: 111. South Africa: 10111 for police and 10177 for an ambulance, or 112 from a mobile. India: 112. Across the European Union: 112.
From a mobile, 112 works in many countries even without credit, and your phone may connect it through another network if yours has no signal.
In the UK, if you cannot speak, call 999, then press 55 when asked. In many parts of the US you can text 911 if you cannot call.`, ""},
	{"Alarm chirping", "A smoke or carbon monoxide alarm chirping", "US Fire Administration, NFPA and UK Fire and Rescue guidance", `A short chirp every minute or so usually means the battery is low: replace it straight away (or the whole alarm, if it has a sealed 10-year battery). Do not take the battery out and leave it out: a home without a working alarm is not protected.
Most alarms also chirp at the end of their life, usually after about 10 years: check the date on the back and replace an old alarm.
If it sounds a full, loud alarm, treat it as real: get everyone out, and for a carbon monoxide alarm stay out and call emergency services or your gas emergency line.`, ""},
	{"Jump-starting a car", "Jump-starting a car", "RAC, AA and vehicle maker guidance", `Check the car handbook first: some cars have special jump points. Do not jump a battery that is cracked, leaking, swollen or frozen. Turn both cars off and keep the leads away from moving parts.
1. Red lead: one end to the + (positive) terminal of the flat battery, then the other end to the + terminal of the good battery.
2. Black lead: one end to the - (negative) terminal of the good battery, then the other end to bare, unpainted metal on the dead car's engine or a jump point, away from the battery. Never connect red to black or + to -: it can damage both cars or make a battery explode.
3. Start the good car, wait a few minutes, then start the dead one. Take the leads off in the reverse order: black from the dead car first, then black from the good car, then red from the good car, then red from the dead car.`, ""},
	{"Fire in a building with flats", "Fire alarm in a block of flats", "UK Fire and Rescue, NFPA and Red Cross guidance", `If the fire is in your flat, or smoke is coming in, get out, close the doors behind you, and do not use the lift. Use the stairs. Call the fire service once you are out.
Tell the fire service at once about anyone who cannot get out on their own, such as a neighbour in a wheelchair, and which floor and flat they are on: firefighters will go to them. Do not try to carry someone down the stairs yourself unless they are in immediate danger and you can do it safely.
Many blocks have a "stay put" plan when the fire is not in your flat: stay inside with doors and windows shut unless smoke or heat reaches you or you are told to leave. If you are trapped, go to a room with a window, shut the door, block gaps with towels, and call the fire service to say where you are.`, ""},
	{"Animals in a disaster", "Animals in a flood, fire or storm", "Ready.gov, RSPCA and US Department of Agriculture guidance", `Never put yourself at risk for animals: do not walk, drive or boat into floodwater or towards a fire to reach them.
If there is still time before it arrives, move livestock to higher ground or a safe paddock, or open gates so they can move away from water or fire themselves; animals shut in a shed can drown or burn. Take pets with you if you leave, with food, water and a carrier or lead.
Afterwards, check animals for injuries and give them clean water, and contact a vet or your local council for help with lost or injured animals.`, ""},
	{"Cooking without power", "Cooking when the power is off", "Ready.gov, CDC and UK Fire Service guidance", `Never use a barbecue, charcoal grill, camp stove, patio heater or generator inside a home, garage, tent or flat, even with a window open: they give off carbon monoxide, which can kill without warning.
Cook outside, well away from doors and windows, or on a gas hob that is connected and working normally. Many meals need no cooking: tinned food can be eaten cold, and a thermos keeps water hot for hours.
Never connect a generator to the home's wiring without a proper changeover switch fitted by an electrician: it can kill line workers. Run it outside, at least 6 metres (20 feet) from the building.`, ""},
	{"Throat closing", "Can't swallow saliva, drooling, or a muffled voice", "NHS and US National Library of Medicine guidance", `Call emergency services now if someone cannot swallow their own saliva, is drooling, has a muffled voice, wants to sit up and lean forward to breathe, or has noisy or difficult breathing: the airway may be swelling shut (epiglottitis or a throat abscess).
Keep them calm and sitting up in the position they choose. Do not make them lie down, do not look in their throat or put anything in it, and give nothing to eat or drink, including lozenges.
If they become unconscious or stop breathing, start CPR.`, ""},
	{"Roof leak", "Water coming through the roof or ceiling", "UK Fire and Rescue, Red Cross and insurer guidance", `Keep everyone out of the room if the ceiling is bulging or sagging: it can collapse suddenly. If water is near lights, sockets or the fuse box, switch off the electricity at the main switch, but only if you can reach it without standing in water.
To ease a small bulge you can safely reach from a dry spot, put a bucket underneath and make a small hole in the lowest point with a screwdriver to let the water drain; do not do this if the ceiling is cracking or sagging badly.
Do not go on the roof in a storm or in the dark. Cover what you can inside with plastic sheeting, move valuables, and take photos for insurance. Call a roofer or the emergency services if the structure seems unsafe.`, ""},
	{"Putting out a campfire", "Putting out a campfire", "US Forest Service and Smokey Bear guidance", `Drown it, stir it, drown it again: pour plenty of water over all the embers, not just the red ones, until the hissing stops. Stir the ash and embers with a stick or shovel, scraping any burnt sticks, then pour on more water.
Feel for heat with the back of your hand held close above the ashes (do not touch). If it is too hot to touch, it is too hot to leave: keep adding water and stirring.
If you have no water, mix in plenty of soil or sand and stir, but do not just bury the fire, which can smoulder for hours and reignite. Never leave a fire unattended, even for a short time.`, ""},
	{"Exploitation", "If your employer holds your passport or threatens you", "UK Modern Slavery Helpline, US National Human Trafficking Hotline and ILO guidance", `It is against the law almost everywhere for an employer to keep your passport or ID, make you work without pay, or threaten to report you to immigration to make you work. You can get free, confidential help, whatever your visa status.
If you are in danger now, call emergency services. Otherwise, in the UK call the Modern Slavery and Exploitation Helpline (08000 121 700); in the US, the National Human Trafficking Hotline (1-888-373-7888, or text 233733); in Australia, the Australian Federal Police (131 237) or the Fair Work Ombudsman. Elsewhere, the police or a workers' or migrants' support group.
If it is safe, keep notes and photos of hours, payments and messages, and keep a copy of your passport and visa somewhere they cannot reach.`, ""},
	{"Broken tooth", "A broken or chipped tooth", "NHS and American Dental Association guidance", `Rinse your mouth with warm water and keep any piece of tooth in milk or your own saliva to take to the dentist. Use a cold pack on the cheek for swelling.
Take a usual dose of paracetamol or ibuprofen for pain, following the packet. Cover a sharp edge with dental wax or sugar-free chewing gum, and eat soft food on the other side. Do not try to file or glue it yourself, and never cut into a swelling.
See a dentist as soon as you can. Get urgent help now (an emergency dentist, NHS 111 or an emergency department) if there is heavy bleeding that will not stop, swelling of the face, eye or neck, a fever, or trouble swallowing or breathing.`, ""},
	{"Kidney infection", "Fever with pain in the side or back", "NHS and CDC guidance", `A high temperature, shivering or feeling sick together with pain in the side, lower back or groin can mean a kidney infection, which needs medical treatment (usually antibiotics) the same day: see a doctor or call a nurse line (NHS 111 in the UK) today. Cranberry juice and waiting do not treat it.
Call emergency services if they are confused or very drowsy, breathing fast, have mottled or very pale skin, have not passed urine all day, or are pregnant and very unwell: the infection may have spread to the blood (sepsis).
Meanwhile drink plenty of water and take paracetamol for pain and fever, following the packet.`, ""},
	{"Baby too cold", "Keeping a baby warm when it is cold", "NHS, Lullaby Trust and Red Cross guidance", `Get urgent medical help now if a baby is cold to touch and also floppy, very sleepy, not feeding, breathing oddly, or has pale, blue or blotchy skin: a cold baby can become seriously ill quickly.
Warm them gently: hold them skin to skin against your chest under your clothes and a blanket, cover the head, and keep the room warm if you can. Do not use a hot water bottle, an electric blanket or a heater close to the baby, and do not put them in a hot bath.
A baby under 6 months should have only breast milk or formula, not water, juice or sweet drinks. Keep feeding as normal. In a cold house, a baby needs about one more layer than an adult, but for sleep use a baby sleeping bag or firmly tucked blankets, not a duvet, and do not let them overheat.`, ""},
	{"Urinary retention", "Unable to pass urine", "NHS guidance", `Not being able to pass urine at all, especially with a swollen, hard or painful lower belly, is an emergency: go to an emergency department or call emergency services now. It is common in older men with an enlarged prostate and can damage the kidneys.
Do not wait to see if it passes by itself, and do not drink large amounts of fluid to try to force it.
Get help the same day, too, if someone is passing much less urine than usual, has a fever with pain on passing urine, or is newly confused.`, ""},
	{"Medicines without a fridge", "Keeping insulin and other medicines without a fridge", "CDC, FDA and NHS guidance", `Most insulin in use can be kept at room temperature, below about 25 to 30 °C (77 to 86 °F), for up to 28 days; check the leaflet, because some types last less. Unopened insulin is best kept cool, but do not let it freeze, and throw away insulin that has frozen or been very hot, or that looks cloudy when it should be clear.
To keep it cool without power: an insulated bag or cool box with a cold pack wrapped in a cloth (never touching the insulin), or a clay-pot or evaporative cooling wallet. Keep it out of the sun and out of a hot car.
If insulin may have spoiled and blood sugar is rising, or you are running out, get medical help early: many pharmacies and shelters can store or supply it in an emergency.`, ""},
	{"Hidden paracetamol", "Taking paracetamol in more than one product", "NHS and US FDA guidance", `Many cold and flu remedies (Lemsip, Beechams, Night Nurse, DayQuil, NyQuil, Theraflu and others) and painkillers such as co-codamol contain paracetamol (acetaminophen). Taking them together, or with paracetamol tablets, easily goes over the safe amount.
For most adults the maximum is 4 grams in 24 hours (eight 500 mg tablets), and less for people under 50 kg, who drink a lot of alcohol or have liver problems. Add up the paracetamol in everything taken, using the packets.
If the total is more than that, get urgent medical advice now (NHS 111, a poison centre or an emergency department), even if you feel fine: paracetamol damages the liver without symptoms at first, and treatment works best early. Do not take any more paracetamol in the meantime.`, ""},
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
When in doubt, throw it out. Never taste food to decide whether it is safe.
To thaw meat safely: in the fridge, in cold water changed every 30 minutes, or in the microwave and then cook it straight away. Never thaw it on the counter or in hot water, which lets bacteria grow.`, ""},
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
A blow to the eye: hold a cold cloth gently against it, and get it checked if sight changes or it stays painful.
A contact lens stuck or lost under the eyelid: wash your hands, rinse the eye with sterile saline or lens solution, look in the opposite direction and gently massage the closed lid to move it out. It cannot go behind the eye. Do not put creams in or near the eye. Get it checked if it stays painful, red or blurred.`, ""},
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
Do not take someone else's prescription medicine, even the same medicine at another strength, and never take medicine prescribed for a pet: animal doses and ingredients differ and can be dangerous for people.`, ""},
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
// firstNotes lead the answer whenever they match, in this order. CPR is not
// here: a choking baby who is still conscious needs back blows first, and the
// choking and drowning notes already say when to start CPR.
var firstNotes = []string{"Opioid overdose"}

var supersedes = map[string][]string{
	// The shock note says to push the source away with something dry and
	// wooden; with a line on a car, everyone must stay clear and inside.
	// The household shock note says to push the source away with wood:
	// deadly near a power line.
	"Downed power line":            {"Electrical injury"},
	"Family reunification":         {"Finding a person"},
	"Power line on vehicle":        {"Electrical injury", "Downed power line"},
	"Nosebleed":                    {"Bleeding"},
	"Shock":                        {"Fainting"},
	"Toxic gas":                    {"Burn"},
	"Generator":                    {"Power outage"},
	"Carbon monoxide poisoning":    {"Dehydration", "Alarm chirping"},
	"Wood stove smoke":             {"Starting a fire", "Smoke inhalation"},
	"Seizure":                      {"Eye injury"},
	"Stroke":                       {"Bone fracture"},
	"Myocardial infarction":        {"Dehydration", "Panic attack"},
	"Sepsis":                       {"Panic attack", "Kidney infection"},
	"Drowning":                     {"Rip current", "Asthma"},
	"Severe belly pain":            {"Fainting"},
	"Illegal request":              {"Weapons refusal"},
	"Baby in hot weather":          {"Heat stroke"},
	"Volcanic ash":                 {"Wildfire"},
	"Baby too cold":                {"Hypothermia"},
	"Poisoning":                    {"Recovery position", "Medicine safety"},
	"Pregnancy warning signs":      {"Medicines in pregnancy", "Anaphylaxis", "Meningitis", "Eye injury", "Severe belly pain"},
	"Febrile seizure":              {"Hypothermia", "Seizure", "Burn"},
	"Postpartum psychosis":         {"Psychosis"},
	"Swallowed battery or magnets": {"Poisoning"},
	"Finding a person":             {"Weapons refusal"},
	"Harming someone":              {"Weapons refusal", "Medicine safety", "Poisoning"},
	"Toxic gas refusal":            {"Toxic gas", "Chemical cloud"},
	"Contaminated water":           {"Water purification"},
	"Too much insulin":             {"Hypoglycemia", "Poisoning"},
	"Venomous fish sting":          {"Jellyfish sting", "Box jellyfish"},
	"Weapons refusal":              {"Opioid overdose", "Poisoning"},
	"Suicide":                      {"Adult medicine dose", "Child medicine dose", "Medicine safety"},
	"Medicine toxicity when ill":   {"Dehydration"},
	"Confined space":               {"Cardiopulmonary resuscitation", "Recovery position", "Stroke"},
	"Belly injury":                 {"Fainting"},
	"Bleeding after birth":         {"Bleeding"},
	"Snakebite":                    {"Bleeding"},
	"Fever":                        {"Medicine safety"},
	"Anaphylaxis":                  {"Bee or wasp sting"},
	"Mushroom poisoning":           {"Dehydration"},
	"Stimulant overheating":        {"Heat stroke", "Dehydration"},
	"Stab or gunshot":              {"Bleeding"},
	"Severed body part":            {"Bleeding"},
	"Ice rescue":                   {"Drowning"},
	"Thunderclap headache":         {"Head injury"},
	"Vomiting blood":               {"Dehydration"},
	"Seriously ill child":          {"Fever"},
	"Poisonous plants":             {"Mushroom poisoning"},
	"Bee swarm":                    {"Anaphylaxis", "Bee or wasp sting"},
	"Sudden vision loss":           {"Eye injury"},
	"Aortic dissection":            {"Myocardial infarction", "Severe belly pain"},
	"Bulging fontanelle":           {"Fever"},
	"Panic attack":                 {"Anaphylaxis"},
	"Food safety":                  {"Power outage"},
	"Hypoglycemia":                 {"Sudden confusion"},
	"Winter car":                   {"Winter heating"},
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
	"Opioid overdose": {"Recovery position", "Alcohol poisoning", "Fainting"},
	// Gasping is not breathing: that person needs CPR, not the recovery
	// position.
	"Cardiopulmonary resuscitation": {"Recovery position"},
	// Notes whose words borrow from another: a jellyfish sting "burning",
	// a tick someone wants to "burn off", bleach for drinking water, a
	// person jerking on a live cable.
	"Jellyfish sting":    {"Burn"},
	"Box jellyfish":      {"Jellyfish sting", "Burn"},
	"Chimney fire":       {"Starting a fire"},
	"Tick":               {"Burn"},
	"Water purification": {"Toxic gas"},
	"Electrical injury":  {"Seizure"},
}

// coercionRE recognises a threat of self-harm used to control someone ("if
// i leave he'll hurt himself"): there the abuse note leads, not the crisis
// note, which would tell the reader to stay with him.
var coercionRE = regexp.MustCompile(`(?i)\bif (i|we) (leave|go|break up|end it)\b|\b(he|she|they).?(ll|will|would|.?s gonna|is going to) (hurt|kill|harm) (himself|herself|themselves)\b.{0,40}\b(if|unless|or me|and me)\b|threatens? to (kill|hurt|harm) (himself|herself|themselves)`)

// neverEverydayTopics are kept even when the question also sounds
// everyday or uses a figure of speech: "everyone would be better off
// without me ... im drowning in debt" is a crisis whatever else it says.
var neverEverydayTopics = map[string]bool{"Suicide": true, "Self-harm": true, "Abuse and threats": true, "Child protection": true, "Older person at risk": true}

// refusalTopics are the notes that decline a harmful request.
var refusalTopics = map[string]bool{"Weapons refusal": true, "Forged documents": true, "Illegal request": true, "Finding a person": true, "Harming someone": true, "Toxic gas refusal": true}

// everydayRE recognises a question about something other than an emergency
// that only borrows an emergency word: a cocktail called a hurricane, a
// Minecraft tornado mod, a laptop "having a seizure", bleeding a radiator.
// urgentRE keeps the notes when the question also sounds like a real
// emergency ("my kid ate a mushroom from the garden, help").
var everydayRE = regexp.MustCompile(`(?i)\b(grow(ing)? mushrooms|mouldy grout|grout|before lunch|firelighters|bonfire night|essay|homework|assignment|pub quiz|quiz|trivia|science (experiment|project|fair)|for (school|class|uni|my class|a lesson)|explain\w* .{0,40}to (a|my|the) (\d+ ?(year|yr).?old|kids?|child|class|students?|year \d)|algorithm|motivation|excel|meal prep|snorkel\w*|sales|revenue|startup|quarter|burn rate|bleeding (cash|money)|drowning in (emails?|work|debt|paperwork|admin)|emails?|productivity|choked in the|(team|we|they) choked|coach(es)?|hydraulic|wordpress|plugin|blender|effects|emoji|poker|slang|sourdough|starter|chilli|chili|strobe|nosebleed seats?|stadium|posters?|school project|freestyle|backstroke|breaststroke|100m|swim(ming)? (lesson|technique)|cake|cocktail|risotto|movie|film|album|song|band|spotify|netflix|quotes?|game|gaming|minecraft|fortnite|roblox|mod|toys?|lego|theme park|ride at|rollercoaster|(cpr|first aid|online|training|refresher) course|certification|class(es)? near|price|best deals|cheapest|where to buy|shopping|photoshop|excel|chrome|browser|software|goats?|radiators?|brakes|chainsaw|mower|traffic noise|noise at night|seat hinge|soft close|tattoo|costume|meme|joke|lyrics)\b`)

// gDropRE finds a dropped final g ("shakin'", "breathin"); the "in"
// must follow at least three letters so "in", "skin" and "pain" stay as
// they are, and words that really end in -in keep their match on the
// original text.
var gDropRE = regexp.MustCompile(`(?i)\b([a-z]{2,}[^aeiou\s'])in'?\b`)

// textSpeakWords are phone shorthand read as the full word, so that every
// rule need not carry both spellings ("hook went thru my thumb").
var textSpeakWords = map[string]string{"w/o": "without", "w/": "with ", "thru": "through", "tho": "though", "u": "you", "ur": "your", "r": "are", "n": "and", "coz": "because", "cos": "because", "cuz": "because", "bc": "because", "wat": "what", "wot": "what", "wen": "when", "abt": "about", "b4": "before", "tonite": "tonight", "2nite": "tonight", "2day": "today", "pls": "please", "plz": "please", "ppl": "people", "dunno": "don't know", "gonna": "going to", "wanna": "want to", "gotta": "got to", "luv": "love", "bf": "boyfriend", "gf": "girlfriend", "yr": "year", "yrs": "years", "hr": "hour", "hrs": "hours", "min": "minute", "mins": "minutes", "yday": "yesterday", "yest": "yesterday", "tmrw": "tomorrow", "tmr": "tomorrow", "2moro": "tomorrow", "nite": "night", "smth": "something", "sth": "something"}

var textSpeakRE = regexp.MustCompile(`(?i)(\bw/o\b|\bw/|\b(?:thru|tho|u|ur|r|n|coz|cos|cuz|bc|wat|wot|wen|abt|b4|tonite|2nite|2day|pls|plz|ppl|dunno|gonna|wanna|gotta|luv|bf|gf|yr|yrs|hr|hrs|mins?|yday|yest|tmrw|tmr|2moro|nite|smth|sth)\b)`)

func textSpeak(q string) string {
	q = textSpeakRE.ReplaceAllStringFunc(q, func(w string) string { return textSpeakWords[strings.ToLower(w)] })
	return misspeltRE.ReplaceAllStringFunc(q, func(w string) string { return misspelt[strings.ToLower(w)] })
}

// misspelt are common misspellings of words the rules depend on, read as
// the right word alongside the original ("hair straightners", "siezure").
var misspelt = map[string]string{
	"siezure": "seizure", "seizur": "seizure", "sezure": "seizure", "seazure": "seizure",
	"unconcious": "unconscious", "unconscous": "unconscious", "unconsious": "unconscious", "unconsciuos": "unconscious",
	"breathe": "breathe", "breth": "breath", "brething": "breathing", "breating": "breathing", "breathng": "breathing",
	"asma": "asthma", "asthama": "asthma", "athsma": "asthma", "astma": "asthma", "ashtma": "asthma",
	"anaphalaxis": "anaphylaxis", "anafylaxis": "anaphylaxis", "anaphylaxsis": "anaphylaxis", "anaphylactic": "anaphylactic",
	"diabetik": "diabetic", "diabeties": "diabetes", "diabetis": "diabetes", "diabetec": "diabetic", "diebetic": "diabetic",
	"insulen": "insulin", "insullin": "insulin", "insolin": "insulin",
	"ambulence": "ambulance", "ambulanse": "ambulance", "ambulnce": "ambulance",
	"hospitle": "hospital", "hosptial": "hospital", "hospial": "hospital",
	"bleding": "bleeding", "bleedin": "bleeding", "bleading": "bleeding", "bledding": "bleeding",
	"poisen": "poison", "poision": "poison", "posion": "poison", "poisoned": "poisoned", "posioned": "poisoned",
	"overdoes": "overdose", "overdosed": "overdosed", "ovedose": "overdose", "overdos": "overdose",
	"paracetemol": "paracetamol", "paracetomol": "paracetamol", "parcetamol": "paracetamol", "paracetamole": "paracetamol",
	"ibuprofin": "ibuprofen", "ibuprophen": "ibuprofen", "iboprofen": "ibuprofen", "ibruprofen": "ibuprofen",
	"nebulizor": "nebuliser", "nebuliser": "nebuliser", "nebulizer": "nebuliser", "nebuliza": "nebuliser",
	"inhailer": "inhaler", "inhaller": "inhaler", "inhealer": "inhaler",
	"choaking": "choking", "chocking": "choking", "chokeing": "choking",
	"straightners": "straighteners", "straighners": "straighteners", "straightner": "straightener",
	"temprature": "temperature", "tempreture": "temperature", "temperture": "temperature", "temp": "temp",
	"feaver": "fever", "fevor": "fever", "fevar": "fever",
	"vomitting": "vomiting", "vommiting": "vomiting", "vomitting.": "vomiting", "vomitin": "vomiting",
	"diarrea": "diarrhoea", "diarhea": "diarrhoea", "diahrrea": "diarrhoea", "diarrhea": "diarrhoea", "diorrhea": "diarrhoea",
	"dihydrated": "dehydrated", "dehidrated": "dehydrated", "dehydraited": "dehydrated",
	"hypothermia": "hypothermia", "hypothermea": "hypothermia", "hyperthermia": "hyperthermia",
	"frostbit": "frostbite", "frost bite": "frostbite",
	"carbon monoxcide": "carbon monoxide", "monoxcide": "monoxide", "monoxide": "monoxide",
	"genorator": "generator", "generater": "generator", "genarator": "generator",
	"electricuted": "electrocuted", "electracuted": "electrocuted", "electocuted": "electrocuted",
	"concusion": "concussion", "consussion": "concussion", "concusion.": "concussion",
	"fractered": "fractured", "fracterd": "fractured", "fracture": "fracture",
	"strok": "stroke", "storke": "stroke",
	"miscarrage": "miscarriage", "miscariage": "miscarriage", "miscarage": "miscarriage",
	"preganant": "pregnant", "pregnent": "pregnant", "pregant": "pregnant", "preggers": "pregnant",
	"suicidle": "suicidal", "suicidel": "suicidal", "suiside": "suicide", "sucide": "suicide", "suiced": "suicide",
	"tornadoe": "tornado", "hurricaine": "hurricane", "huricane": "hurricane", "earthqauke": "earthquake", "earthquack": "earthquake",
	"flooding": "flooding", "floodin": "flooding",
}

var misspeltRE = regexp.MustCompile(`(?i)\b(siezure|seizur|sezure|seazure|unconcious|unconscous|unconsious|unconsciuos|breth|brething|breating|breathng|asma|asthama|athsma|astma|ashtma|anaphalaxis|anafylaxis|anaphylaxsis|diabetik|diabeties|diabetis|diabetec|diebetic|insulen|insullin|insolin|ambulence|ambulanse|ambulnce|hospitle|hosptial|hospial|bleding|bleading|bledding|poisen|poision|posion|posioned|overdoes|ovedose|overdos|paracetemol|paracetomol|parcetamol|paracetamole|ibuprofin|ibuprophen|iboprofen|ibruprofen|nebulizor|nebulizer|nebuliza|inhailer|inhaller|inhealer|choaking|chocking|chokeing|straightners|straighners|straightner|temprature|tempreture|temperture|feaver|fevor|fevar|vommiting|vomitting|diarrea|diarhea|diahrrea|diarrhea|diorrhea|dihydrated|dehidrated|dehydraited|hypothermea|frostbit|monoxcide|genorator|generater|genarator|electricuted|electracuted|electocuted|concusion|consussion|fractered|fracterd|strok|storke|miscarrage|miscariage|miscarage|preganant|pregnent|pregant|preggers|suicidle|suicidel|suiside|sucide|suiced|tornadoe|hurricaine|huricane|earthqauke|earthquack)\b`)

// plainWords maps brand names and slang to the words the rules use:
// "12 panadol" is paracetamol, "chundered" is vomiting, "keeled over" is
// collapsed. Each blind review round found a harmful answer behind a word
// like these.
var plainWordList = []struct {
	re   *regexp.Regexp
	word string
}{
	{regexp.MustCompile(`(?i)\b(panadol|calpol|crocin|dolo|panamax|tempra|disprol|mapap|tylenol)\b`), "paracetamol"},
	{regexp.MustCompile(`(?i)\b(nurofen|advil|motrin|brufen|calprofen)\b`), "ibuprofen"},
	{regexp.MustCompile(`(?i)\b(aleve|naprosyn)\b`), "naproxen"},
	{regexp.MustCompile(`(?i)\b(disprin|bayer|ecotrin)\b`), "aspirin"},
	{regexp.MustCompile(`(?i)\b(piriton|zyrtec|claritin|clarityn|allegra|telfast)\b`), "antihistamine"},
	{regexp.MustCompile(`(?i)\b(chunder(ed|ing)?|spew(ed|ing)?|puk(ed|ing)|hurl(ed|ing)|barf(ed|ing)?|upchuck(ed|ing)?|heav(ed|ing) up|technicolou?r yawn)\b`), "vomiting"},
	{regexp.MustCompile(`(?i)\b(keel(ed)? over|conk(ed)? out|dropped like a stone|hit the deck|went down like a sack|flaked out)\b`), "collapsed"},
	{regexp.MustCompile(`(?i)\b(bloods|claret)\b`), "blood"},
	{regexp.MustCompile(`(?i)\b(gasping|can.?t catch (my|his|her|their) breath|puffed out|winded and can.?t)\b`), "breathless"},
	{regexp.MustCompile(`(?i)\b(bub|bubba|bairn|wean|pickney|wee one|lil one|little one)\b`), "baby"},
	{regexp.MustCompile(`(?i)\b(nan|nana|nanna|gran|granny|grandad|grandpa|pop|poppa|oma|opa)\b`), "elderly"},
	{regexp.MustCompile(`(?i)\b(genny|genset)\b`), "generator"},
	{regexp.MustCompile(`(?i)\b(ute|bakkie|lorry|motor)\b`), "car"},
}

func plainWords(q string) string {
	var out []string
	for _, w := range plainWordList {
		if w.re.MatchString(q) {
			out = append(out, w.word)
		}
	}
	return strings.Join(out, " ")
}

// metaphorRE recognises figures of speech and machinery that use injury
// words: these never need a note, even though "bleeding" also sounds urgent.
var metaphorRE = regexp.MustCompile(`(?i)\b((stop|stops|keep|keeps) .{0,30}boiling over|boil(s|ing)? over\b|cast iron (pan|skillet|pot|griddle)|which aftersun|best aftersun|after.?sun (lotion|cream|gel)|chok\w* with laughter|(got|were|we got) (absolutely )?smoked\b|house of cards|d&d|dnd|dungeons|casts? (a )?(lightning|fireball)|stabbed by the spines|cactus|rawhide|dog chew|bleed\w* (a |the |my |our )?radiators?|radiators? .{0,15}bleed\w*|(excel|spreadsheet|chart|laptop|computer|pc|code|server|browser|app)\b.{0,30}chok\w*|gave me (a|an absolute|a mini) heart attack|bleeding (goals|points|runs|money|subscribers)|(nearly|almost) (passed out|died|fainted) (from|of|with) (boredom|laughing|embarrassment)|jump.?start (my|your|our|the) (motivation|career|day|morning|week|business|fitness)|choking on \d+|(screen|paint|ink|colou?rs?|dye|portfolio|market|stocks?|budget|cash|money|customers)\s.{0,20}bleed\w*|bleed(ing)? (edge|colou?r|through the (tape|paint))|stroke of (midnight|luck|genius)|drowning (my )?sorrows|under ?water with|choked? (up|laughing|on (my|his|her) words)|choking up|nearly choked laughing|face (dropped|fell) when|fit of (zoomies|giggles|laughter|rage|pique)|(engine|motor|bearing|pump|gearbox)s? seized|seized up|mottled (paint|effect|finish)|for (my |a )?(biology|science|health|first aid) (homework|class|essay|project|exam)|(writing|for) a quiz|what does cpr stand for|what is a heart attack exactly|cheap date|hairball|(phone|laptop|computer|console|tablet|engine|car) (overheated|froze|crashed)|heartburn after|which antacid|plant leaves|leaves are drooping|bleed(ing)? (cash|money|users|customers|subscribers)|burn rate|bleed (a|the|my) (hydraulic|jack|radiators?|brakes?|lines?|system|pump|forks?))\b`)

var urgentRE = regexp.MustCompile(`(?i)\b(water ris\w*|rising water|floodwater|flood water|(cyclone|hurricane|typhoon|tornado|storm) (is )?(coming|tonight|tomorrow|hitting|due|on its way|about to hit)|(didn.?t|didnt|did not) sound like a joke|not (a )?joke|wasn.?t joking|wasnt joking|bridge|help|now|urgent|emergency|911|999|112|ambulance|hospital|bleed(ing)? (badly|a lot|everywhere)|not breathing|unconscious|unresponsive|collapsed|swallowed|\bate\b|drank|bitten|stung|burned|burnt|hurt|injur|choking on|can.?t breathe|passed out|baby|toddler|child|kid|son|daughter|husband|wife|mum|mom|dad|nan|gran|bloods?|bleeding|spurt\w*|pumping|pouring|gushing|cut (my|his|her|their)|gash|wound|broke|broken|fell|pain|agony|screaming|kicked back|thigh|leg|arm|hand|finger)\b`)

func Notes(q string) []Source {
	if !UseNotes {
		return nil
	}
	topics := EmergencyTopics(q)
	// A refusal is never dropped as everyday or a figure of speech: "mess
	// with someone's car brakes so it looks like an accident" mentions
	// brakes, which on their own are a car-repair question.
	if metaphorRE.MatchString(q) || everydayRE.MatchString(q) && !urgentRE.MatchString(q) {
		var keep []string
		for _, t := range topics {
			if refusalTopics[t] || neverEverydayTopics[t] {
				keep = append(keep, t)
			}
		}
		if len(keep) == 0 {
			return nil
		}
		topics = keep
	}
	// Also read "shakin", "breathin", "goin blue" as "shaking", "breathing",
	// "going blue": people type as they speak, and every rule would
	// otherwise need both spellings.
	// Text-speak too ("thru", "u", "ur", "n"): read alongside the original.
	if g := textSpeak(gDropRE.ReplaceAllString(q, "${1}ing")); g != q {
		for _, t := range EmergencyTopics(g) {
			if !contains(topics, t) {
				topics = append(topics, t)
			}
		}
		q = q + " " + g
	}
	// Brand names and slang: add the plain word, keeping the original.
	if extra := plainWords(q); extra != "" {
		// Match the plain words together with the question: many rules
		// need two words at once ("took 20" and a medicine).
		for _, t := range EmergencyTopics(q + " " + extra) {
			if !contains(topics, t) {
				topics = append(topics, t)
			}
		}
		q = q + " " + extra
	}
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
	// Notes whose first step cannot wait come first whatever matched
	// first: naloxone for an opioid overdose.
	for i := len(firstNotes) - 1; i >= 0; i-- {
		if j := slices.Index(topics, firstNotes[i]); j > 0 {
			topics = append([]string{firstNotes[i]}, slices.Delete(topics, j, j+1)...)
		}
	}
	// The crisis note leads when someone is thinking of harming themselves
	// but has not yet taken or done anything: once they have, the poisoning
	// or bleeding note comes first, because that is what can kill now.
	// It also follows a note about someone else being the danger (abuse, or a
	// request to hurt someone), which is what the reader needs first; the
	// crisis note still shows.
	if j := slices.Index(topics, "Suicide"); j > 0 && !slices.Contains(topics, "Poisoning") && !slices.Contains(topics, "Bleeding") && !(slices.Contains(topics, "Abuse and threats") && coercionRE.MatchString(q)) && !slices.Contains(topics, "Harming someone") {
		topics = append([]string{"Suicide"}, slices.Delete(topics, j, j+1)...)
	}
	for _, other := range []string{"Abuse and threats", "Harming someone"} {
		if other == "Abuse and threats" && !coercionRE.MatchString(q) {
			continue
		}
		if j, k := slices.Index(topics, "Suicide"), slices.Index(topics, other); j >= 0 && k > j {
			topics = slices.Insert(slices.Delete(topics, k, k+1), j, other)
		}
	}
	var out []Source
	for _, t := range topics {
		baby := isBaby(q)
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
