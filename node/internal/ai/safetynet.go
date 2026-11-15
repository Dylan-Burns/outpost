package ai

import "regexp"

// A blind review of answers to 60 questions written without sight of Outpost
// found every harmful answer from the smallest model on a question that no
// note matched, and nearly all of them failed the same way: they did not say
// to get help now (a stroke "slept off", Calpol for a 9-week-old with a fever,
// waiting out a rash that does not fade). For a health question with no note,
// the page shows this card above the model's answer, so the signs that need
// help now are in front of the reader whatever the model writes. The card is
// not given to the model: it would copy the list instead of answering.

var healthRE = regexp.MustCompile(`(?i)\b(pain|hurts?|hurting|ache|aching|sore|bleed|bleeding|blood|breath|breathing|wheez|unconscious|passed out|faint|collapsed\b|collapses on|fever|temp\b|temperature|vomit|throw(ing|n)? up|threw up|being sick|rash|swell|swollen|lump|injur|wound|cut (my|his|her|their)|bite|bitten|sting|stung|broke|broken|sprain|fell|fall(en)?\b|dizzy|confus|drowsy|sleepy|floppy|limp|numb\b|numbness|tingl|weak|headache|migraine|chest|stomach|tummy|belly|diarr|poo(p)?\b|infect|pus|cough|sick|ill\b|unwell|medicine|medication|pills?|tablets?|dose|overdos|poison|allerg|seiz|fit\b|fits\b|pregnan|baby|newborn|toddler|child|kid|elderly|nan\b|grandma|grandpa|grandad|symptom|doctor|hospital|a&e|\ber\b|ambulance|911|999|112|111|nurse|eye|ear|tooth|teeth|skin|blister|choke|choking|drown|hypotherm|frostbite|heat ?stroke|sunburn|shiver|clammy|pale|blue lips|mental|anxi|panic|depress|suicid|self.?harm)`)

// notHealthRE keeps the card off questions that only borrow a body word.
var notHealthRE = regexp.MustCompile(`(?i)\b(burn (pile|barrel|wood|ban)|fell (a|the) tree|tree fell|power (cut|fell)|eye ?bolt|ear ?plugs? for|kid(s)? (activities|games|toys)|baby (monitor|wipes|food recipe)|stock|market|economy|lottery|address|election|president)`)

const safetyNetText = `Call emergency services now for any of these: not breathing normally or struggling to breathe; unconscious or very hard to wake; chest pain or pressure; a face drooping, an arm weak or speech slurred (stroke); severe bleeding that will not stop; a fit lasting more than 5 minutes; swelling of the lips, tongue or throat; sudden confusion; a rash that does not fade when a glass is pressed on it; poisoning or an overdose, even if they seem well; or someone who may harm themselves.
A baby under 3 months with a temperature of 38 °C (100.4 °F) or more, or any baby or child who is floppy, unusually sleepy, or not feeding, needs urgent medical help now.
If you are not sure, call: an emergency call handler or a nurse line (NHS 111 in the UK) can tell you whether to go in.`

// SafetyNet returns the "when to get help now" card for a health question
// that no Outpost note matched, and false otherwise.
func SafetyNet(q string, sources []Source) (Source, bool) {
	if !UseNotes || !healthRE.MatchString(q) || notHealthRE.MatchString(q) {
		return Source{}, false
	}
	for _, s := range sources {
		if s.Note != "" {
			return Source{}, false
		}
	}
	return Source{Title: "Outpost note: When to get help now", Book: "Summary of NHS and Red Cross guidance", Note: safetyNetText}, true
}
