package jobs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func payload(n int) ([]byte, string) {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i * 7)
	}
	s := sha256.Sum256(b)
	return b, hex.EncodeToString(s[:])
}

func newDL() *Downloader {
	d := NewDownloader("test")
	d.AllowHTTP = true
	d.IdleTimeout = 2 * time.Second
	return d
}

func TestFetchFresh(t *testing.T) {
	data, sum := payload(1 << 20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "f", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	part := filepath.Join(t.TempDir(), "f.part")
	var last int64
	if err := newDL().Fetch(context.Background(), srv.URL, part, sum, int64(len(data)), func(d, _ int64) { last = d }); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(part)
	if !bytes.Equal(got, data) || last != int64(len(data)) {
		t.Fatal("content or progress wrong")
	}
}

func TestFetchResumes(t *testing.T) {
	data, sum := payload(1 << 20)
	var ranges atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			ranges.Add(1)
		}
		http.ServeContent(w, r, "f", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	part := filepath.Join(t.TempDir(), "f.part")
	os.WriteFile(part, data[:300000], 0o644)
	if err := newDL().Fetch(context.Background(), srv.URL, part, sum, int64(len(data)), nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(part)
	if !bytes.Equal(got, data) || ranges.Load() != 1 {
		t.Fatalf("resume failed (ranges=%d)", ranges.Load())
	}
}

func TestFetchServerIgnoresRange(t *testing.T) {
	data, sum := payload(200000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data) // always 200, whole file
	}))
	defer srv.Close()
	part := filepath.Join(t.TempDir(), "f.part")
	os.WriteFile(part, []byte("garbage-prefix"), 0o644)
	if err := newDL().Fetch(context.Background(), srv.URL, part, sum, int64(len(data)), nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(part)
	if !bytes.Equal(got, data) {
		t.Fatal("restart after ignored range failed")
	}
}

func TestFetchChecksumMismatch(t *testing.T) {
	data, _ := payload(5000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "f", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	part := filepath.Join(t.TempDir(), "f.part")
	err := newDL().Fetch(context.Background(), srv.URL, part, "00"+string(make([]byte, 0))+"11111111111111111111111111111111111111111111111111111111111111", int64(len(data)), nil)
	if !errors.Is(err, ErrChecksum) || !IsPermanent(err) {
		t.Fatalf("want checksum error, got %v", err)
	}
	if _, err := os.Stat(part); !os.IsNotExist(err) {
		t.Fatal("bad part file kept")
	}
}

func TestFetchNotFoundIsPermanent(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	err := newDL().Fetch(context.Background(), srv.URL, filepath.Join(t.TempDir(), "p"), "", 10, nil)
	if err == nil || !IsPermanent(err) {
		t.Fatalf("want permanent error, got %v", err)
	}
}

func TestFetchTooMuchData(t *testing.T) {
	data, sum := payload(5000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(append(data, data...))
	}))
	defer srv.Close()
	err := newDL().Fetch(context.Background(), srv.URL, filepath.Join(t.TempDir(), "p"), sum, int64(len(data)), nil)
	if err == nil || !IsPermanent(err) {
		t.Fatalf("oversized body accepted: %v", err)
	}
}

func TestFetchStall(t *testing.T) {
	data, sum := payload(5000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data[:100])
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	defer srv.Close()
	d := newDL()
	d.IdleTimeout = 300 * time.Millisecond
	start := time.Now()
	err := d.Fetch(context.Background(), srv.URL, filepath.Join(t.TempDir(), "p"), sum, int64(len(data)), nil)
	if err == nil || IsPermanent(err) || time.Since(start) > 5*time.Second {
		t.Fatalf("stall not detected as a retryable error: %v after %v", err, time.Since(start))
	}
}

func TestRefusesHTTPRedirect(t *testing.T) {
	d := NewDownloader("t")
	if err := d.Fetch(context.Background(), "http://example.com/x", filepath.Join(t.TempDir(), "p"), "", 1, nil); err == nil || !IsPermanent(err) {
		t.Fatal("plain http accepted")
	}
}

func TestFetchResumingReportsReread(t *testing.T) {
	data, sum := payload(3 << 20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "f", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	part := filepath.Join(t.TempDir(), "f.part")
	// Two thirds of the file are already on disk.
	if err := os.WriteFile(part, data[:2<<20], 0o644); err != nil {
		t.Fatal(err)
	}
	var reread, rereadTotal, got int64
	err := newDL().FetchResuming(context.Background(), srv.URL, part, sum, int64(len(data)),
		func(d, _ int64) { got = d },
		func(d, total int64) { reread, rereadTotal = d, total })
	if err != nil {
		t.Fatal(err)
	}
	if reread != 2<<20 || rereadTotal != 2<<20 {
		t.Fatalf("reread reported %d of %d, want all %d bytes that were on disk", reread, rereadTotal, 2<<20)
	}
	if got != int64(len(data)) {
		t.Fatalf("download progress ended at %d, want %d", got, len(data))
	}
	if b, _ := os.ReadFile(part); !bytes.Equal(b, data) {
		t.Fatal("resumed file differs from the original")
	}
}

func TestFetchResumingStopsWhenCancelled(t *testing.T) {
	data, sum := payload(1 << 20)
	part := filepath.Join(t.TempDir(), "f.part")
	if err := os.WriteFile(part, data, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// No server is needed: a cancelled job must stop before it reads the file back.
	err := newDL().FetchResuming(ctx, "https://example.invalid/f", part, sum, int64(len(data)), nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}
