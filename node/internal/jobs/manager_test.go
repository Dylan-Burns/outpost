package jobs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Dylan-Burns/outpost/node/internal/ai"
	"github.com/Dylan-Burns/outpost/node/internal/catalog"
	"github.com/Dylan-Burns/outpost/node/internal/state"
)

type fixture struct {
	m      *Manager
	dir    string
	fails  atomic.Int32 // number of 503s to send before success
	cancel context.CancelFunc
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	f := &fixture{dir: dir}
	good := bytes.Repeat([]byte("zimdata-"), 4096)
	sum := sha256.Sum256(good)
	mux := http.NewServeMux()
	mux.HandleFunc("/good.zim", func(w http.ResponseWriter, r *http.Request) {
		if f.fails.Load() > 0 {
			f.fails.Add(-1)
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		http.ServeContent(w, r, "good.zim", time.Time{}, bytes.NewReader(good))
	})
	mux.HandleFunc("/bad.zim", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not what the catalog says"))
	})
	mux.HandleFunc("/slow.zim", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100000")
		w.Write(make([]byte, 10))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"version":"1"}`) })
	mux.HandleFunc("/api/pull", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"status":"downloading","digest":"x","total":10,"completed":10}`)
		fmt.Fprintln(w, `{"status":"success"}`)
	})
	mux.HandleFunc("/api/delete", func(w http.ResponseWriter, r *http.Request) {})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cat := &catalog.Catalog{Schema: 1, Items: []catalog.Item{
		{ID: "good", Kind: "zim", Title: "Good", URL: srv.URL + "/good.zim", File: "good_2026-01.zim", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(good))},
		{ID: "bad", Kind: "zim", Title: "Bad", URL: srv.URL + "/bad.zim", File: "bad.zim", SHA256: hex.EncodeToString(sum[:]), Size: 25},
		{ID: "slow", Kind: "zim", Title: "Slow", URL: srv.URL + "/slow.zim", File: "slow.zim", SHA256: hex.EncodeToString(sum[:]), Size: 100000},
		{ID: "m1", Kind: "model", Title: "Model", Model: "tiny:1b", Size: 10},
	}}
	data, _ := json.Marshal(cat)
	parsed, err := catalog.Parse(data)
	if err != nil {
		// http URLs are not valid in a real catalog; build the index directly.
		parsed = mustIndex(t, cat)
	}
	st, err := state.Open(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	paths := Paths{Zim: filepath.Join(dir, "zim"), Maps: filepath.Join(dir, "maps"), MapAssets: filepath.Join(dir, "assets"), Downloads: filepath.Join(dir, "dl")}
	for _, p := range []string{paths.Zim, paths.Maps, paths.Downloads} {
		os.MkdirAll(p, 0o755)
	}
	dl := NewDownloader("test")
	dl.AllowHTTP = true
	dl.IdleTimeout = 5 * time.Second
	f.m = &Manager{Store: st, Catalog: func() *catalog.Catalog { return parsed }, DL: dl, AI: ai.New(srv.URL), Paths: paths, RetryDelay: 50 * time.Millisecond, MaxAttempts: 4, Reserve: 1}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	f.cancel = func() { cancel(); <-done }
	go func() { f.m.Run(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		<-done
		time.Sleep(100 * time.Millisecond) // let retry timers that fired finish their writes
	})
	return f
}

func mustIndex(t *testing.T, c *catalog.Catalog) *catalog.Catalog {
	t.Helper()
	// Swap http for https in a copy just to pass validation, then restore.
	cp := *c
	cp.Items = append([]catalog.Item(nil), c.Items...)
	orig := map[string]string{}
	for i := range cp.Items {
		orig[cp.Items[i].ID] = cp.Items[i].URL
		if cp.Items[i].URL != "" {
			cp.Items[i].URL = "https://example.invalid/" + cp.Items[i].ID
		}
	}
	data, _ := json.Marshal(&cp)
	parsed, err := catalog.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"good", "bad", "slow", "m1"} {
		parsed.Item(id).URL = orig[id]
	}
	return parsed
}

func (f *fixture) waitJob(t *testing.T, item, action string, want ...string) state.Job {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var found *state.Job
		f.m.Store.View(func(s *state.State) {
			for i := len(s.Jobs) - 1; i >= 0; i-- {
				if s.Jobs[i].ItemID == item && s.Jobs[i].Action == action {
					j := s.Jobs[i]
					found = &j
					break
				}
			}
		})
		if found != nil {
			for _, w := range want {
				if found.State == w {
					return *found
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("job %s %s did not reach %v", action, item, want)
	return state.Job{}
}

func TestInstallRetryRemove(t *testing.T) {
	f := newFixture(t)
	f.fails.Store(2)
	if err := f.m.Apply([]string{"good", "m1"}); err != nil {
		t.Fatal(err)
	}
	j := f.waitJob(t, "good", "install", state.JobDone)
	if j.Attempts != 3 {
		t.Fatalf("attempts %d, want 3", j.Attempts)
	}
	f.waitJob(t, "m1", "install", state.JobDone)
	if _, err := os.Stat(filepath.Join(f.dir, "zim", "good_2026-01.zim")); err != nil {
		t.Fatal("zim not in place")
	}
	var n int
	f.m.Store.View(func(s *state.State) { n = len(s.Installed) })
	if n != 2 {
		t.Fatalf("installed %d", n)
	}
	if err := f.m.Apply(nil); err != nil {
		t.Fatal(err)
	}
	f.waitJob(t, "good", "remove", state.JobDone)
	f.waitJob(t, "m1", "remove", state.JobDone)
	if _, err := os.Stat(filepath.Join(f.dir, "zim", "good_2026-01.zim")); !os.IsNotExist(err) {
		t.Fatal("zim not removed")
	}
}

func TestChecksumFailureIsFinal(t *testing.T) {
	f := newFixture(t)
	f.m.Enqueue("bad", "install")
	j := f.waitJob(t, "bad", "install", state.JobFailed)
	if j.Attempts != 1 || j.Error == "" {
		t.Fatalf("attempts %d error %q", j.Attempts, j.Error)
	}
	var installed bool
	f.m.Store.View(func(s *state.State) { _, installed = s.Installed["bad"] })
	if installed {
		t.Fatal("bad file recorded as installed")
	}
}

func TestCancelRunning(t *testing.T) {
	f := newFixture(t)
	f.m.Enqueue("slow", "install")
	j := f.waitJob(t, "slow", "install", state.JobRunning)
	time.Sleep(100 * time.Millisecond)
	if err := f.m.Cancel(j.ID); err != nil {
		t.Fatal(err)
	}
	f.waitJob(t, "slow", "install", state.JobCancel)
	// Removing the item cleans up the partial download.
	f.m.Enqueue("slow", "remove")
	f.waitJob(t, "slow", "remove", state.JobDone)
	if _, err := os.Stat(filepath.Join(f.dir, "dl", "slow.zim.part")); !os.IsNotExist(err) {
		t.Fatal("partial download left behind")
	}
}

func TestEnqueueDedupAndUnknown(t *testing.T) {
	f := newFixture(t)
	f.cancel() // keep jobs queued
	time.Sleep(50 * time.Millisecond)
	f.m.Enqueue("good", "install")
	f.m.Enqueue("good", "install")
	if err := f.m.Enqueue("nope", "install"); err == nil {
		t.Fatal("unknown item accepted")
	}
	f.m.Enqueue("good", "remove") // withdraws the queued install
	pending := 0
	f.m.Store.View(func(s *state.State) {
		for _, j := range s.Jobs {
			if state.Pending(j.State) {
				pending++
			}
		}
	})
	if pending != 1 {
		t.Fatalf("pending %d, want 1", pending)
	}
}
