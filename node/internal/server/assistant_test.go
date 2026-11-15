package server

import (
	"testing"

	"github.com/Dylan-Burns/outpost/node/internal/ai"
)

func TestAnswerWithNote(t *testing.T) {
	note := []ai.Source{{Label: "S1", Title: "Outpost note: Carbon monoxide", Note: "Get everyone out."}, {Label: "S2", Title: "Carbon monoxide poisoning"}}
	library := []ai.Source{{Label: "S1", Title: "Carbon monoxide poisoning"}}
	qwen := modelView{Model: "qwen3:1.7b", Size: 1_360_000_000, Rank: 4}
	llama := modelView{Model: "llama3.2:3b", Size: 2_020_000_000, Rank: 1}
	tiny := modelView{Model: "someone:1b", Size: 900_000_000}
	big := modelView{Model: "qwen3:8b", Size: 5_230_000_000}
	cases := []struct {
		m    modelView
		src  []ai.Source
		want bool
	}{
		{qwen, note, true},     // the smallest model: the note answers
		{tiny, note, true},     // unranked and small
		{llama, note, false},   // a better model writes its own answer
		{big, note, false},     // unranked and large
		{qwen, library, false}, // no note: the model answers from the library
	}
	for _, c := range cases {
		if got := answerWithNote(c.m, c.src); got != c.want {
			t.Errorf("%s with %d sources: got %v, want %v", c.m.Model, len(c.src), got, c.want)
		}
	}
}
