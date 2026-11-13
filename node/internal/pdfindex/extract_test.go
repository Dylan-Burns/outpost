package pdfindex

import (
	"context"
	"os"
	"strings"
	"testing"
)

// The test binary doubles as the PDF reader's child process.
func TestMain(m *testing.M) {
	if os.Getenv("PDFINDEX_CHILD") == "1" {
		os.Exit(ServeExtract())
	}
	os.Exit(m.Run())
}

func TestSubprocessContainsBadPDF(t *testing.T) {
	t.Setenv("PDFINDEX_CHILD", "1")
	extract := Subprocess(os.Args[0])
	if _, err := extract(context.Background(), []byte("%PDF-1.4\nnot really a pdf")); err == nil {
		t.Fatal("garbage read as a PDF")
	}
}

func TestChunkPages(t *testing.T) {
	long := strings.Repeat("Apply pressure to the wound. ", 120)
	chunks := chunkPages("B", "First Aid", "/content/x/a.pdf", []string{"Short cover page text that is long enough to keep around for the index.", "", long})
	if len(chunks) < 2 {
		t.Fatalf("got %d chunks", len(chunks))
	}
	for _, c := range chunks {
		if len(c.Text) > chunkMax+chunkTarget {
			t.Fatalf("chunk too long: %d", len(c.Text))
		}
	}
	if chunks[len(chunks)-1].Page != 3 {
		t.Fatalf("last chunk page %d", chunks[len(chunks)-1].Page)
	}
}
