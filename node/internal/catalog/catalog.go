// Package catalog describes the content a node can install: ZIM archives
// (served by kiwix-serve), map regions (extracted from the Protomaps planet
// build), the map style assets, and AI models (pulled through Ollama).
//
// The catalog is a signed JSON document published on the Outpost website. A
// copy is embedded in the binary so a node that has never been online can
// still run the setup wizard; a newer signed copy replaces it when fetched.
package catalog

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/Dylan-Burns/outpost/node/internal/fsutil"
	"github.com/Dylan-Burns/outpost/node/internal/sshsig"
)

// SignatureNamespace is the SSHSIG namespace catalog signatures are made under.
const SignatureNamespace = "outpost-catalog"

// Item kinds.
const (
	KindZim       = "zim"
	KindMapRegion = "map-region"
	KindMapWorld  = "map-world"
	KindMapAssets = "map-assets"
	KindModel     = "model"
)

// Item is one installable unit.
type Item struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Publisher   string    `json:"publisher,omitempty"`
	License     string    `json:"license,omitempty"`
	Size        int64     `json:"size"`
	Date        string    `json:"date,omitempty"`
	Notice      string    `json:"notice,omitempty"` // e.g. "medical": shown next to the item
	URL         string    `json:"url,omitempty"`    // zim, map-assets: file URL; map-region: geometry URL
	File        string    `json:"file,omitempty"`   // zim, map-assets: file name on disk
	SHA256      string    `json:"sha256,omitempty"` // of the file at URL
	Book        string    `json:"book,omitempty"`   // zim: Kiwix book name
	Flavour     string    `json:"flavour,omitempty"`
	Group       string    `json:"group,omitempty"` // map-region: e.g. "United States"
	BBox        []float64 `json:"bbox,omitempty"`  // map-region: west,south,east,north
	Model       string    `json:"model,omitempty"` // model: Ollama name, e.g. "qwen3:1.7b"
	RAMGB       int       `json:"ramGB,omitempty"` // model: memory it needs to run comfortably
	Rank        int       `json:"rank,omitempty"`  // model: 1 is best in Outpost's assistant tests; 0 is untested
	MaxZoom     int       `json:"maxzoom,omitempty"`
}

// Option is one choice of a single-choice category (Wikipedia depth).
type Option struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Items       []string `json:"items"`
}

// Category groups items. Tiered categories offer essential, standard and
// comprehensive sets (each listing its full item set); single-choice
// categories offer options.
type Category struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Icon        string              `json:"icon,omitempty"`
	Kind        string              `json:"kind"` // "tiered" or "single"
	Tiers       map[string][]string `json:"tiers,omitempty"`
	Options     []Option            `json:"options,omitempty"`
	Default     string              `json:"default,omitempty"`
}

// Tier names, smallest first.
var TierNames = []string{"essential", "standard", "comprehensive"}

// Catalog is the whole document.
type Catalog struct {
	Schema     int        `json:"schema"`
	Sequence   int64      `json:"sequence"`
	Generated  string     `json:"generated"`
	MapBuild   string     `json:"mapBuild"` // planet .pmtiles URL that regions are cut from
	Categories []Category `json:"categories"`
	Items      []Item     `json:"items"`

	byID map[string]*Item
}

// Selection is what the owner picked in the wizard or the content manager.
type Selection struct {
	Wikipedia string            `json:"wikipedia"`       // option id of the "wikipedia" category
	Tiers     map[string]string `json:"tiers"`           // category id -> tier name or "" for none
	Regions   []string          `json:"regions"`         // map-region item ids
	Models    []string          `json:"models"`          // model item ids
	Extra     []string          `json:"extra,omitempty"` // individually added item ids
}

var (
	idRE     = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]{0,120}$`)
	sha256RE = regexp.MustCompile(`^[0-9a-f]{64}$`)
	modelRE  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,60}(:[a-z0-9][a-z0-9._-]{0,40})?$`)
)

// Parse decodes and validates a catalog. It does not check a signature.
func Parse(data []byte) (*Catalog, error) {
	var c Catalog
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// ParseSigned verifies the signature, then parses.
func ParseSigned(data, sig []byte, key ed25519.PublicKey) (*Catalog, error) {
	if err := sshsig.Verify(key, SignatureNamespace, data, sig); err != nil {
		return nil, fmt.Errorf("catalog signature: %w", err)
	}
	return Parse(data)
}

func httpsURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil
}

func (c *Catalog) validate() error {
	if c.Schema != 1 {
		return fmt.Errorf("catalog: unsupported schema %d", c.Schema)
	}
	if c.MapBuild != "" && (!httpsURL(c.MapBuild) || !strings.HasSuffix(c.MapBuild, ".pmtiles")) {
		return errors.New("catalog: map build must be an https .pmtiles URL")
	}
	c.byID = make(map[string]*Item, len(c.Items))
	for i := range c.Items {
		it := &c.Items[i]
		if !idRE.MatchString(it.ID) {
			return fmt.Errorf("catalog: bad item id %q", it.ID)
		}
		if _, dup := c.byID[it.ID]; dup {
			return fmt.Errorf("catalog: duplicate item %q", it.ID)
		}
		if it.Size < 0 {
			return fmt.Errorf("catalog: %s: negative size", it.ID)
		}
		switch it.Kind {
		case KindZim, KindMapAssets:
			if !httpsURL(it.URL) || !fsutil.SafeName(it.File) || !sha256RE.MatchString(it.SHA256) {
				return fmt.Errorf("catalog: %s: needs an https url, a safe file name and a sha256", it.ID)
			}
			if it.Kind == KindZim && !strings.HasSuffix(it.File, ".zim") {
				return fmt.Errorf("catalog: %s: zim file must end in .zim", it.ID)
			}
		case KindMapRegion:
			if !httpsURL(it.URL) || !sha256RE.MatchString(it.SHA256) {
				return fmt.Errorf("catalog: %s: region needs an https geometry url and sha256", it.ID)
			}
			if len(it.BBox) != 4 {
				return fmt.Errorf("catalog: %s: region needs a bbox", it.ID)
			}
		case KindMapWorld:
		case KindModel:
			if !modelRE.MatchString(it.Model) {
				return fmt.Errorf("catalog: %s: bad model name %q", it.ID, it.Model)
			}
		default:
			return fmt.Errorf("catalog: %s: unknown kind %q", it.ID, it.Kind)
		}
		c.byID[it.ID] = it
	}
	seen := map[string]bool{}
	for _, cat := range c.Categories {
		if !idRE.MatchString(cat.ID) || seen[cat.ID] {
			return fmt.Errorf("catalog: bad or duplicate category %q", cat.ID)
		}
		seen[cat.ID] = true
		switch cat.Kind {
		case "tiered":
			for tier, ids := range cat.Tiers {
				if !validTier(tier) {
					return fmt.Errorf("catalog: %s: unknown tier %q", cat.ID, tier)
				}
				if err := c.checkRefs(cat.ID, ids); err != nil {
					return err
				}
			}
		case "single":
			for _, o := range cat.Options {
				if err := c.checkRefs(cat.ID, o.Items); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("catalog: %s: unknown category kind %q", cat.ID, cat.Kind)
		}
	}
	return nil
}

func validTier(t string) bool {
	for _, n := range TierNames {
		if n == t {
			return true
		}
	}
	return false
}

func (c *Catalog) checkRefs(cat string, ids []string) error {
	for _, id := range ids {
		if c.byID[id] == nil {
			return fmt.Errorf("catalog: %s refers to unknown item %q", cat, id)
		}
	}
	return nil
}

// Item returns the item with id, or nil.
func (c *Catalog) Item(id string) *Item { return c.byID[id] }

// Category returns the category with id, or nil.
func (c *Catalog) Category(id string) *Category {
	for i := range c.Categories {
		if c.Categories[i].ID == id {
			return &c.Categories[i]
		}
	}
	return nil
}

// Resolve turns a selection into the sorted list of item ids it needs,
// adding the shared map items when any region is chosen. Unknown ids are
// reported as an error so a stale client cannot silently drop content.
func (c *Catalog) Resolve(sel Selection) ([]string, error) {
	want := map[string]bool{}
	add := func(id string) error {
		if c.byID[id] == nil {
			return fmt.Errorf("unknown item %q", id)
		}
		want[id] = true
		return nil
	}
	if sel.Wikipedia != "" {
		cat := c.Category("wikipedia")
		if cat == nil {
			return nil, errors.New("catalog has no wikipedia category")
		}
		found := false
		for _, o := range cat.Options {
			if o.ID == sel.Wikipedia {
				found = true
				for _, id := range o.Items {
					want[id] = true
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown wikipedia option %q", sel.Wikipedia)
		}
	}
	for catID, tier := range sel.Tiers {
		if tier == "" || tier == "none" {
			continue
		}
		cat := c.Category(catID)
		if cat == nil || cat.Kind != "tiered" {
			return nil, fmt.Errorf("unknown category %q", catID)
		}
		ids, ok := cat.Tiers[tier]
		if !ok {
			return nil, fmt.Errorf("category %q has no tier %q", catID, tier)
		}
		for _, id := range ids {
			want[id] = true
		}
	}
	for _, id := range sel.Regions {
		if it := c.byID[id]; it == nil || it.Kind != KindMapRegion {
			return nil, fmt.Errorf("unknown map region %q", id)
		}
		want[id] = true
	}
	for _, id := range sel.Models {
		if it := c.byID[id]; it == nil || it.Kind != KindModel {
			return nil, fmt.Errorf("unknown model %q", id)
		}
		want[id] = true
	}
	for _, id := range sel.Extra {
		if err := add(id); err != nil {
			return nil, err
		}
	}
	if len(sel.Regions) > 0 {
		for _, it := range c.Items {
			if it.Kind == KindMapWorld || it.Kind == KindMapAssets {
				want[it.ID] = true
			}
		}
	}
	out := make([]string, 0, len(want))
	for id := range want {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

// TotalSize sums the sizes of the given items.
func (c *Catalog) TotalSize(ids []string) int64 {
	var n int64
	for _, id := range ids {
		if it := c.byID[id]; it != nil {
			n += it.Size
		}
	}
	return n
}
