package catalog

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Dylan-Burns/outpost/node/internal/fsutil"
)

// Fetcher downloads a small file.
type Fetcher func(ctx context.Context, url string, limit int64) ([]byte, error)

// Provider holds the catalog in use and replaces it with newer signed ones.
type Provider struct {
	Key      ed25519.PublicKey
	CacheDir string // where the last accepted remote catalog is kept
	URL      string // remote catalog URL; the signature is URL + ".sig"
	Fetch    Fetcher

	mu  sync.RWMutex
	cur *Catalog
}

// NewProvider starts from the embedded catalog, then a cached newer one.
func NewProvider(key ed25519.PublicKey, embedded, embeddedSig []byte, cacheDir, url string, fetch Fetcher) (*Provider, error) {
	p := &Provider{Key: key, CacheDir: cacheDir, URL: url, Fetch: fetch}
	c, err := ParseSigned(embedded, embeddedSig, key)
	if err != nil {
		return nil, fmt.Errorf("embedded catalog: %w", err)
	}
	p.cur = c
	if data, err := os.ReadFile(filepath.Join(cacheDir, "catalog.json")); err == nil {
		if sig, err := os.ReadFile(filepath.Join(cacheDir, "catalog.json.sig")); err == nil {
			if cc, err := ParseSigned(data, sig, key); err == nil && cc.Sequence > c.Sequence {
				p.cur = cc
			}
		}
	}
	return p, nil
}

// Current returns the catalog in use.
func (p *Provider) Current() *Catalog {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cur
}

// Refresh fetches the remote catalog and switches to it when its signature
// is valid and its sequence number is higher (no rollback to older ones).
func (p *Provider) Refresh(ctx context.Context) (bool, error) {
	if p.URL == "" || p.Fetch == nil {
		return false, errors.New("no catalog URL configured")
	}
	data, err := p.Fetch(ctx, p.URL, 32<<20)
	if err != nil {
		return false, err
	}
	sig, err := p.Fetch(ctx, p.URL+".sig", 64<<10)
	if err != nil {
		return false, err
	}
	c, err := ParseSigned(data, sig, p.Key)
	if err != nil {
		return false, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if c.Sequence <= p.cur.Sequence {
		return false, nil
	}
	if err := os.MkdirAll(p.CacheDir, 0o755); err == nil {
		fsutil.WriteFileAtomic(filepath.Join(p.CacheDir, "catalog.json.sig"), sig, 0o644)
		fsutil.WriteFileAtomic(filepath.Join(p.CacheDir, "catalog.json"), data, 0o644)
	}
	p.cur = c
	return true, nil
}
