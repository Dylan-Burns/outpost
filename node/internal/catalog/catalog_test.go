package catalog

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/Dylan-Burns/outpost/node/internal/release"
	"github.com/Dylan-Burns/outpost/node/internal/sshsig"
)

func key(t *testing.T) []byte {
	k, err := sshsig.ParsePublicKey(release.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestEmbeddedCatalogVerifies(t *testing.T) {
	c, err := ParseSigned(Embedded, EmbeddedSig, key(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Items) < 100 || c.Category("wikipedia") == nil {
		t.Fatalf("catalog looks thin: %d items", len(c.Items))
	}
	tampered := bytes.Replace(Embedded, []byte("download.kiwix.org"), []byte("download.kiwix.orh"), 1)
	if _, err := ParseSigned(tampered, EmbeddedSig, key(t)); err == nil {
		t.Fatal("tampered catalog accepted")
	}
}

func TestResolve(t *testing.T) {
	c, _ := ParseSigned(Embedded, EmbeddedSig, key(t))
	var region string
	for _, it := range c.Items {
		if it.Kind == KindMapRegion {
			region = it.ID
			break
		}
	}
	ids, err := c.Resolve(Selection{Wikipedia: "top-mini", Tiers: map[string]string{"survival": "essential", "medicine": ""}, Regions: []string{region}})
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	for _, want := range []string{"wikipedia_en_top_mini", "zimgit-water_en", region, "map-world", "map-assets"} {
		if !set[want] {
			t.Errorf("missing %s in %v", want, ids)
		}
	}
	if set["wikipedia_en_medicine_maxi"] {
		t.Error("medicine tier 'none' still added items")
	}
	for _, bad := range []Selection{
		{Wikipedia: "nope"},
		{Tiers: map[string]string{"survival": "huge"}},
		{Tiers: map[string]string{"nope": "essential"}},
		{Regions: []string{"zimgit-water_en"}},
		{Models: []string{"map-world"}},
		{Extra: []string{"../../etc"}},
	} {
		if _, err := c.Resolve(bad); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
}

func TestProviderRefusesRollback(t *testing.T) {
	served := map[string][]byte{"u": Embedded, "u.sig": EmbeddedSig}
	fetch := func(ctx context.Context, url string, limit int64) ([]byte, error) {
		if b, ok := served[url]; ok {
			return b, nil
		}
		return nil, errors.New("404")
	}
	p, err := NewProvider(key(t), Embedded, EmbeddedSig, t.TempDir(), "u", fetch)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := p.Refresh(context.Background())
	if err != nil || changed {
		t.Fatalf("same sequence should not change anything: %v %v", changed, err)
	}
	served["u.sig"] = []byte("-----BEGIN SSH SIGNATURE-----\nAAAA\n-----END SSH SIGNATURE-----")
	if _, err := p.Refresh(context.Background()); err == nil {
		t.Fatal("bad signature accepted")
	}
}

func TestParseRejectsUnsafeItems(t *testing.T) {
	bad := []string{
		`{"schema":1,"items":[{"id":"x","kind":"zim","title":"x","size":1,"url":"http://a/x.zim","file":"x.zim","sha256":"` + string(bytes.Repeat([]byte("a"), 64)) + `"}]}`,
		`{"schema":1,"items":[{"id":"x","kind":"zim","title":"x","size":1,"url":"https://a/x.zim","file":"../x.zim","sha256":"` + string(bytes.Repeat([]byte("a"), 64)) + `"}]}`,
		`{"schema":1,"items":[{"id":"x","kind":"model","title":"x","size":1,"model":"a b; rm"}]}`,
		`{"schema":1,"mapBuild":"https://x/evil.sh","items":[]}`,
		`{"schema":2,"items":[]}`,
		`{"schema":1,"items":[],"unknown":1}`,
	}
	for _, b := range bad {
		if _, err := Parse([]byte(b)); err == nil {
			t.Errorf("accepted %s", b)
		}
	}
}
