package state

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestRoundTripAndRollback(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	st, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Update(func(s *State) error { s.Settings.NodeName = "Cabin"; return nil }); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	if err := st.Update(func(s *State) error { s.Settings.NodeName = "Lost"; return boom }); err != boom {
		t.Fatal(err)
	}
	st2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	var name string
	st2.View(func(s *State) { name = s.Settings.NodeName })
	if name != "Cabin" {
		t.Fatalf("name %q", name)
	}
	st.View(func(s *State) { name = s.Settings.NodeName })
	if name != "Cabin" {
		t.Fatalf("rollback failed: %q", name)
	}
}
