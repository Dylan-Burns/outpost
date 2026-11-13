package maps

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

type entry struct {
	name string
	typ  byte
	body string
	link string
}

func makeTar(t *testing.T, entries []entry) string {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Typeflag: e.typ, Mode: 0o644, Size: int64(len(e.body)), Linkname: e.link}
		if e.typ != tar.TypeReg {
			h.Size = 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if e.typ == tar.TypeReg {
			tw.Write([]byte(e.body))
		}
	}
	tw.Close()
	gz.Close()
	p := filepath.Join(t.TempDir(), "a.tar.gz")
	os.WriteFile(p, buf.Bytes(), 0o644)
	return p
}

func TestInstallAssets(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "assets")
	p := makeTar(t, []entry{
		{name: "fonts/", typ: tar.TypeDir},
		{name: "fonts/Noto Sans Regular/0-255.pbf", typ: tar.TypeReg, body: "glyphs"},
		{name: "sprites/v4/light@2x.png", typ: tar.TypeReg, body: "png"},
		{name: "sprites/v4/light.json", typ: tar.TypeReg, body: "{}"},
		{name: "fonts/evil-link.pbf", typ: tar.TypeSymlink, link: "/etc/passwd"},
	})
	if err := InstallAssets(p, dest); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dest, "fonts", "Noto Sans Regular", "0-255.pbf"))
	if err != nil || string(b) != "glyphs" {
		t.Fatalf("font missing: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dest, "fonts", "evil-link.pbf")); !os.IsNotExist(err) {
		t.Fatal("symlink was extracted")
	}
}

func TestInstallAssetsRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../escape.pbf", "fonts/../../escape.pbf", "/abs/fonts/x.pbf", "other/x.pbf", "fonts/x.sh", "fonts/.hidden.pbf"} {
		dest := filepath.Join(t.TempDir(), "assets")
		p := makeTar(t, []entry{{name: name, typ: tar.TypeReg, body: "x"}})
		if err := InstallAssets(p, dest); err == nil {
			t.Errorf("%q accepted", name)
		}
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			t.Errorf("%q: destination created on failure", name)
		}
	}
}

func TestInstallAssetsKeepsOldOnFailure(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "assets")
	good := makeTar(t, []entry{{name: "fonts/a/0-255.pbf", typ: tar.TypeReg, body: "v1"}})
	if err := InstallAssets(good, dest); err != nil {
		t.Fatal(err)
	}
	bad := makeTar(t, []entry{{name: "fonts/a/0-255.pbf", typ: tar.TypeReg, body: "v2"}, {name: "../x.pbf", typ: tar.TypeReg, body: "x"}})
	if err := InstallAssets(bad, dest); err == nil {
		t.Fatal("bad archive accepted")
	}
	b, _ := os.ReadFile(filepath.Join(dest, "fonts", "a", "0-255.pbf"))
	if string(b) != "v1" {
		t.Fatalf("old assets lost: %q", b)
	}
}
