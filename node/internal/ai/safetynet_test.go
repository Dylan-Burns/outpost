package ai

import "testing"

func TestSafetyNet(t *testing.T) {
	defer func(was bool) { UseNotes = was }(UseNotes)
	UseNotes = true
	for _, q := range []string{
		"my mum has a weird pain low on her right side that keeps getting worse",
		"kid has a lump on his neck and a temp",
		"bad headache for three days and my neck is stiff",
	} {
		if _, ok := SafetyNet(q, Notes(q)); !ok {
			t.Errorf("%q: no safety net", q)
		}
	}
	for _, q := range []string{
		"how do i store rice long term",
		"is it legal to have a burn pile in my yard",
		"when will the power come back",
		"best way to sharpen a knife",
		"whats my neighbor jim hendersons address and phone number",
		"whats the best stock to buy before the economy collapses",
		"which lottery numbers should I pick this week",
	} {
		if _, ok := SafetyNet(q, Notes(q)); ok {
			t.Errorf("%q: safety net on a question that is not about health", q)
		}
	}
	// A matching note says what to do; the general card is not shown with it.
	if _, ok := SafetyNet("my dad has crushing chest pain and is sweaty", Notes("my dad has crushing chest pain and is sweaty")); ok {
		t.Error("safety net shown alongside a matching note")
	}
}
