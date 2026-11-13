package pdfindex

import "testing"

func TestSearchRanksTitleAndTerms(t *testing.T) {
	ix := New([]Chunk{
		{Book: "Survival", Doc: "Water Treatment", Path: "/content/w/a.pdf", Page: 3, Text: "Add 8 drops of unscented bleach per gallon of clear water and wait 30 minutes."},
		{Book: "Survival", Doc: "Gardening", Path: "/content/g/b.pdf", Page: 9, Text: "Water the beans every morning."},
		{Book: "Survival", Doc: "First Aid", Path: "/content/f/c.pdf", Page: 45, Text: "Apply direct pressure to the wound to control bleeding."},
	})
	hits := ix.Search("bleach water treatment", 2)
	if len(hits) == 0 || hits[0].Doc != "Water Treatment" {
		t.Fatalf("got %+v", hits)
	}
	if c, ok := ix.Get("/content/f/c.pdf", 45); !ok || c.Doc != "First Aid" {
		t.Fatal("Get failed")
	}
	var nilIx *Index
	if nilIx.Search("x", 3) != nil || nilIx.Len() != 0 {
		t.Fatal("nil index should be empty")
	}
}
