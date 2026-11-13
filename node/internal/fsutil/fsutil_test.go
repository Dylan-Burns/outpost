package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeName(t *testing.T) {
	good := []string{"wikipedia_en_top_maxi_2026-01.zim", "map-us-ca.pmtiles", "a"}
	bad := []string{"", ".", "..", "../x", "a/b", ".hidden", "a b", "a\x00b", "é.zim", "a\\b"}
	for _, g := range good {
		if !SafeName(g) {
			t.Errorf("%q should be safe", g)
		}
	}
	for _, b := range bad {
		if SafeName(b) {
			t.Errorf("%q should be unsafe", b)
		}
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	if err := WriteFileAtomic(p, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(p, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "two" {
		t.Fatalf("got %q", b)
	}
	st, _ := os.Stat(p)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("perm %v", st.Mode().Perm())
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 1 {
		t.Fatalf("temp files left behind: %v", ents)
	}
}

func TestDiskSpace(t *testing.T) {
	free, total, err := DiskSpace(t.TempDir())
	if err != nil || total == 0 || free > total {
		t.Fatalf("free=%d total=%d err=%v", free, total, err)
	}
}
