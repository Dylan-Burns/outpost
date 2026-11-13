package server

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Dylan-Burns/outpost/node/internal/pdfindex"
)

// RunPDFIndex keeps an index of the PDFs inside the library's archives, so the
// assistant can read survival and medical manuals that kiwix cannot search.
// Text is cached per archive under DataDir, so only new archives cost time.
func (s *Server) RunPDFIndex(ctx context.Context) {
	b := &pdfindex.Builder{
		Base:     s.Kiwix.Base(),
		CacheDir: filepath.Join(s.DataDir, "pdftext"),
		Logf:     s.logf,
	}
	var last string
	wait := 20 * time.Second
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = 2 * time.Minute
		if !s.Kiwix.Running() {
			continue
		}
		books, err := s.Kiwix.Catalog(ctx)
		if err != nil {
			continue
		}
		var arch []pdfindex.Archive
		var keys []string
		for _, bk := range books {
			arch = append(arch, pdfindex.Archive{Content: bk.Content, Title: bk.Title})
			keys = append(keys, bk.Content)
		}
		slices.Sort(keys)
		key := strings.Join(keys, "\n")
		if key == last {
			continue
		}
		t := time.Now()
		ix, err := b.Build(ctx, arch, nil)
		if err != nil {
			continue
		}
		s.pdf.Store(ix)
		last = key
		s.logf("pdf index: %d passages from %d archives in %s", ix.Len(), len(arch), time.Since(t).Round(time.Second))
	}
}
