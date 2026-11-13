package pdfindex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

// ExtractPages returns the text of each page of a PDF. The PDF reader is not
// written for hostile input; run it through Subprocess outside of tests.
func ExtractPages(body []byte) (pages []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			pages, err = nil, fmt.Errorf("unreadable PDF: %v", r)
		}
	}()
	// "%PDF-1.2 " then a line break is valid, but the reader wants the break
	// right after the version. Replacing the space keeps every offset.
	if len(body) > 9 && bytes.HasPrefix(body, []byte("%PDF-1.")) && (body[8] == ' ' || body[8] == '\t') {
		body[8] = '\n'
	}
	r, err := pdf.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, err
	}
	n := r.NumPage()
	for i := 1; i <= n && i <= maxPages; i++ {
		pg := r.Page(i)
		txt := ""
		if !pg.V.IsNull() {
			if t, err := pg.GetPlainText(nil); err == nil {
				txt = strings.Join(strings.Fields(t), " ")
			}
		}
		pages = append(pages, txt)
	}
	return pages, nil
}

const (
	extractTimeout = 3 * time.Minute
	extractMemory  = 1 << 30
	maxPagesJSON   = 256 << 20
)

// Subprocess returns an extractor that runs ExtractPages in a child process
// (the program at exe, with the given arguments), so that a PDF that loops,
// exhausts the stack or eats memory costs one document, not the node.
// The child must call ServeExtract.
func Subprocess(exe string, args ...string) func(ctx context.Context, body []byte) ([]string, error) {
	return func(ctx context.Context, body []byte) ([]string, error) {
		ctx, cancel := context.WithTimeout(ctx, extractTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, exe, args...)
		cmd.Stdin = bytes.NewReader(body)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GOMEMLIMIT=%d", extractMemory), "GOMAXPROCS=1")
		var out bytes.Buffer
		cmd.Stdout = &limitWriter{w: &out, n: maxPagesJSON}
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return nil, errors.New("PDF took too long to read")
			}
			return nil, fmt.Errorf("PDF reader failed: %v", err)
		}
		var pages []string
		if err := json.Unmarshal(out.Bytes(), &pages); err != nil {
			return nil, fmt.Errorf("PDF reader output: %w", err)
		}
		return pages, nil
	}
}

// ServeExtract is the child side of Subprocess: it reads a PDF on stdin and
// writes its pages as a JSON list of strings on stdout.
func ServeExtract() int {
	debug.SetMemoryLimit(extractMemory)
	debug.SetMaxStack(64 << 20)
	body, err := io.ReadAll(io.LimitReader(os.Stdin, maxPDF+1))
	if err != nil || len(body) > maxPDF {
		fmt.Fprintln(os.Stderr, "PDF missing or too large")
		return 1
	}
	pages, err := ExtractPages(body)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := json.NewEncoder(os.Stdout).Encode(pages); err != nil {
		return 1
	}
	return 0
}

type limitWriter struct {
	w io.Writer
	n int64
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > l.n {
		return 0, errors.New("PDF text too large")
	}
	l.n -= int64(len(p))
	return l.w.Write(p)
}
