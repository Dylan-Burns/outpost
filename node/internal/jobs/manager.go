// Package jobs installs and removes catalog items in the background: it
// downloads and verifies ZIM archives, cuts map regions, pulls AI models,
// and keeps the job list in the node's state so work resumes after a
// restart or a power cut.
package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Dylan-Burns/outpost/node/internal/ai"
	"github.com/Dylan-Burns/outpost/node/internal/catalog"
	"github.com/Dylan-Burns/outpost/node/internal/fsutil"
	"github.com/Dylan-Burns/outpost/node/internal/kiwix"
	"github.com/Dylan-Burns/outpost/node/internal/maps"
	"github.com/Dylan-Burns/outpost/node/internal/state"
)

// Paths are the directories the manager writes to.
type Paths struct {
	Zim       string
	Maps      string
	MapAssets string
	Downloads string
	PMTiles   string // the pmtiles tool
}

// Manager runs install and remove jobs.
type Manager struct {
	Store   *state.Store
	Catalog func() *catalog.Catalog
	DL      *Downloader
	Kiwix   *kiwix.Manager
	AI      *ai.Client
	Paths   Paths
	Logger  *log.Logger
	// MaxAttempts bounds retries of transient failures.
	MaxAttempts int
	// RetryDelay is the first retry delay; it doubles up to 2 minutes.
	RetryDelay time.Duration
	// Reserve is the free space kept untouched on the data disk.
	Reserve int64

	mu      sync.Mutex
	live    map[string]*Live
	cancels map[string]context.CancelFunc
	wake    chan struct{}
	kiwixCh chan struct{}
	seq     int64
}

// Live is the in-memory progress of a running job.
type Live struct {
	Done  int64   `json:"done"`
	Total int64   `json:"total"`
	Speed float64 `json:"speed"` // bytes per second
	Phase string  `json:"phase"`
	t     time.Time
	last  int64
}

// JobView is a job with its live progress, for the API.
type JobView struct {
	state.Job
	Title string `json:"title"`
	Kind  string `json:"kind"`
	Size  int64  `json:"size"`
	Live  *Live  `json:"live,omitempty"`
}

func (m *Manager) init() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.live == nil {
		m.live = map[string]*Live{}
		m.cancels = map[string]context.CancelFunc{}
		m.wake = make(chan struct{}, 1)
		m.kiwixCh = make(chan struct{}, 1)
		if m.MaxAttempts == 0 {
			m.MaxAttempts = 8
		}
		if m.RetryDelay == 0 {
			m.RetryDelay = 5 * time.Second
		}
		if m.Reserve == 0 {
			m.Reserve = 512 << 20
		}
	}
}

func (m *Manager) logf(format string, args ...any) {
	if m.Logger != nil {
		m.Logger.Printf(format, args...)
	}
}

func (m *Manager) poke() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// RebuildLibrary asks for the kiwix library file to be rewritten soon.
func (m *Manager) RebuildLibrary() {
	m.init()
	select {
	case m.kiwixCh <- struct{}{}:
	default:
	}
}

func (m *Manager) newID() string {
	m.mu.Lock()
	m.seq++
	n := m.seq
	m.mu.Unlock()
	return fmt.Sprintf("j%d-%d", time.Now().UnixNano(), n)
}

// Enqueue adds a job unless an equivalent one is already pending.
func (m *Manager) Enqueue(itemID, action string) error {
	m.init()
	if action != "install" && action != "remove" {
		return errors.New("bad action")
	}
	if action == "install" && m.Catalog().Item(itemID) == nil {
		return fmt.Errorf("unknown item %q", itemID)
	}
	err := m.Store.Update(func(s *state.State) error {
		for i := range s.Jobs {
			j := &s.Jobs[i]
			if j.ItemID != itemID || !state.Pending(j.State) {
				continue
			}
			if j.Action == action {
				return nil // already pending
			}
			if j.State != state.JobRunning {
				// An opposite job that has not started is simply withdrawn.
				j.State = state.JobCancel
				j.Updated = time.Now()
			}
		}
		now := time.Now()
		s.Jobs = append(s.Jobs, state.Job{ID: m.newID(), ItemID: itemID, Action: action, State: state.JobQueued, Created: now, Updated: now})
		pruneJobs(s)
		return nil
	})
	if err == nil {
		// A running opposite job is cancelled so the new one wins.
		m.cancelRunning(itemID, opposite(action))
		m.poke()
	}
	return err
}

func opposite(a string) string {
	if a == "install" {
		return "remove"
	}
	return "install"
}

func (m *Manager) cancelRunning(itemID, action string) {
	var ids []string
	m.Store.View(func(s *state.State) {
		for _, j := range s.Jobs {
			if j.ItemID == itemID && j.Action == action && j.State == state.JobRunning {
				ids = append(ids, j.ID)
			}
		}
	})
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range ids {
		if c := m.cancels[id]; c != nil {
			c()
		}
	}
}

func pruneJobs(s *state.State) {
	const keep = 300
	if len(s.Jobs) <= keep {
		return
	}
	var active, finished []state.Job
	for _, j := range s.Jobs {
		if state.Pending(j.State) {
			active = append(active, j)
		} else {
			finished = append(finished, j)
		}
	}
	if n := keep - len(active); n < len(finished) {
		if n < 0 {
			n = 0
		}
		finished = finished[len(finished)-n:]
	}
	s.Jobs = append(finished, active...)
	sort.SliceStable(s.Jobs, func(a, b int) bool { return s.Jobs[a].Created.Before(s.Jobs[b].Created) })
}

// Apply makes the installed set match want: missing items are installed,
// installed items that are no longer wanted are removed.
func (m *Manager) Apply(want []string) error {
	m.init()
	wantSet := map[string]bool{}
	for _, id := range want {
		wantSet[id] = true
	}
	var installs, removes []string
	m.Store.View(func(s *state.State) {
		for _, id := range want {
			if _, ok := s.Installed[id]; !ok {
				installs = append(installs, id)
			}
		}
		for id := range s.Installed {
			if !wantSet[id] {
				removes = append(removes, id)
			}
		}
		// Pending installs that are no longer wanted are dropped too.
		for _, j := range s.Jobs {
			if j.Action == "install" && state.Pending(j.State) && !wantSet[j.ItemID] {
				if _, inst := s.Installed[j.ItemID]; !inst {
					removes = append(removes, j.ItemID)
				}
			}
		}
	})
	sort.Strings(installs)
	sort.Strings(removes)
	for _, id := range removes {
		if err := m.Enqueue(id, "remove"); err != nil {
			return err
		}
	}
	// Smaller items first, so the node becomes useful quickly.
	c := m.Catalog()
	sort.SliceStable(installs, func(a, b int) bool { return c.Item(installs[a]).Size < c.Item(installs[b]).Size })
	for _, id := range installs {
		if err := m.Enqueue(id, "install"); err != nil {
			return err
		}
	}
	return nil
}

// Outdated lists installed ZIM items whose catalog entry names a newer file.
func (m *Manager) Outdated() []string {
	c := m.Catalog()
	var out []string
	m.Store.View(func(s *state.State) {
		for id, in := range s.Installed {
			it := c.Item(id)
			if it != nil && it.Kind == catalog.KindZim && it.File != in.File {
				out = append(out, id)
			}
		}
	})
	sort.Strings(out)
	return out
}

// UpdateAll queues the download of every outdated item.
func (m *Manager) UpdateAll() (int, error) {
	ids := m.Outdated()
	for _, id := range ids {
		if err := m.Enqueue(id, "install"); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// Cancel stops a queued or running job.
func (m *Manager) Cancel(jobID string) error {
	m.init()
	found := false
	err := m.Store.Update(func(s *state.State) error {
		for i := range s.Jobs {
			if s.Jobs[i].ID == jobID && state.Pending(s.Jobs[i].State) {
				s.Jobs[i].State = state.JobCancel
				s.Jobs[i].Updated = time.Now()
				found = true
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !found {
		return errors.New("no such pending job")
	}
	m.mu.Lock()
	if c := m.cancels[jobID]; c != nil {
		c()
	}
	m.mu.Unlock()
	return nil
}

// Retry requeues a failed or cancelled job.
func (m *Manager) Retry(jobID string) error {
	var item, action string
	m.Store.View(func(s *state.State) {
		for _, j := range s.Jobs {
			if j.ID == jobID && (j.State == state.JobFailed || j.State == state.JobCancel) {
				item, action = j.ItemID, j.Action
			}
		}
	})
	if item == "" {
		return errors.New("no such failed job")
	}
	return m.Enqueue(item, action)
}

// ClearFinished drops finished jobs from the list.
func (m *Manager) ClearFinished() error {
	return m.Store.Update(func(s *state.State) error {
		var keep []state.Job
		for _, j := range s.Jobs {
			if state.Pending(j.State) {
				keep = append(keep, j)
			}
		}
		s.Jobs = keep
		return nil
	})
}

// Snapshot returns all jobs with live progress, newest first.
func (m *Manager) Snapshot() []JobView {
	m.init()
	c := m.Catalog()
	var out []JobView
	m.Store.View(func(s *state.State) {
		for _, j := range s.Jobs {
			v := JobView{Job: j, Title: j.ItemID}
			if it := c.Item(j.ItemID); it != nil {
				v.Title, v.Kind, v.Size = it.Title, it.Kind, it.Size
			} else if in, ok := s.Installed[j.ItemID]; ok {
				v.Kind, v.Size = in.Kind, in.Size
			}
			out = append(out, v)
		}
	})
	m.mu.Lock()
	for i := range out {
		if l := m.live[out[i].ID]; l != nil && out[i].State == state.JobRunning {
			cp := *l
			out[i].Live = &cp
		}
	}
	m.mu.Unlock()
	sort.SliceStable(out, func(a, b int) bool { return out[a].Created.After(out[b].Created) })
	return out
}

// Run processes jobs until ctx ends: two workers for files and one for
// models, plus the library rebuilder.
func (m *Manager) Run(ctx context.Context) {
	m.init()
	// Jobs that were running when the node stopped start again.
	m.Store.Update(func(s *state.State) error {
		for i := range s.Jobs {
			if s.Jobs[i].State == state.JobRunning || s.Jobs[i].State == state.JobWaiting {
				s.Jobs[i].State = state.JobQueued
			}
		}
		return nil
	})
	var wg sync.WaitGroup
	for _, lane := range []string{"files", "files", "models"} {
		wg.Add(1)
		go func(lane string) {
			defer wg.Done()
			m.worker(ctx, lane)
		}(lane)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.libraryLoop(ctx)
	}()
	m.RebuildLibrary()
	wg.Wait()
}

func laneOf(c *catalog.Catalog, s *state.State, j state.Job) string {
	kind := ""
	if it := c.Item(j.ItemID); it != nil {
		kind = it.Kind
	} else if in, ok := s.Installed[j.ItemID]; ok {
		kind = in.Kind
	}
	if kind == catalog.KindModel {
		return "models"
	}
	return "files"
}

// claim marks the oldest queued job of a lane as running.
func (m *Manager) claim(lane string) (state.Job, bool) {
	var claimed state.Job
	ok := false
	c := m.Catalog()
	m.Store.Update(func(s *state.State) error {
		running := map[string]bool{}
		for _, j := range s.Jobs {
			if j.State == state.JobRunning {
				running[j.ItemID] = true
			}
		}
		for i := range s.Jobs {
			j := &s.Jobs[i]
			if j.State != state.JobQueued || running[j.ItemID] || laneOf(c, s, *j) != lane {
				continue
			}
			j.State = state.JobRunning
			j.Attempts++
			j.Updated = time.Now()
			j.Error = ""
			claimed = *j
			ok = true
			return nil
		}
		return nil
	})
	return claimed, ok
}

func (m *Manager) worker(ctx context.Context, lane string) {
	for ctx.Err() == nil {
		job, ok := m.claim(lane)
		if !ok {
			select {
			case <-ctx.Done():
				return
			case <-m.wake:
				m.poke() // let the other workers look too
				time.Sleep(50 * time.Millisecond)
			case <-time.After(5 * time.Second):
			}
			continue
		}
		jctx, cancel := context.WithCancel(ctx)
		m.mu.Lock()
		m.cancels[job.ID] = cancel
		m.live[job.ID] = &Live{Phase: "starting", t: time.Now()}
		m.mu.Unlock()
		err := m.runJob(jctx, job)
		cancel()
		m.mu.Lock()
		delete(m.cancels, job.ID)
		delete(m.live, job.ID)
		m.mu.Unlock()
		if ctx.Err() != nil {
			return // shutting down: the job stays "running" and resumes next start
		}
		m.finish(job, err)
	}
}

func (m *Manager) finish(job state.Job, err error) {
	retryIn := time.Duration(0)
	m.Store.Update(func(s *state.State) error {
		for i := range s.Jobs {
			j := &s.Jobs[i]
			if j.ID != job.ID {
				continue
			}
			j.Updated = time.Now()
			switch {
			case j.State == state.JobCancel:
				// Cancelled while running; keep it cancelled.
			case err == nil:
				j.State = state.JobDone
				j.Error = ""
			case errors.Is(err, context.Canceled):
				j.State = state.JobCancel
			case IsPermanent(err) || j.Attempts >= m.MaxAttempts:
				j.State = state.JobFailed
				j.Error = err.Error()
			default:
				j.State = state.JobWaiting
				j.Error = err.Error()
				retryIn = m.RetryDelay << (j.Attempts - 1)
				if retryIn > 2*time.Minute {
					retryIn = 2 * time.Minute
				}
			}
		}
		return nil
	})
	if err != nil {
		m.logf("job %s (%s %s): %v", job.ID, job.Action, job.ItemID, err)
	}
	if retryIn > 0 {
		time.AfterFunc(retryIn, func() {
			m.Store.Update(func(s *state.State) error {
				for i := range s.Jobs {
					if s.Jobs[i].ID == job.ID && s.Jobs[i].State == state.JobWaiting {
						s.Jobs[i].State = state.JobQueued
					}
				}
				return nil
			})
			m.poke()
		})
	}
}

func (m *Manager) setLive(id string, fn func(l *Live)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if l := m.live[id]; l != nil {
		fn(l)
	}
}

func (m *Manager) progress(id string) func(done, total int64) {
	return func(done, total int64) {
		m.setLive(id, func(l *Live) {
			now := time.Now()
			if dt := now.Sub(l.t).Seconds(); dt >= 1 {
				inst := float64(done-l.last) / dt
				if inst < 0 {
					inst = 0
				}
				if l.Speed == 0 {
					l.Speed = inst
				} else {
					l.Speed = 0.7*l.Speed + 0.3*inst
				}
				l.t, l.last = now, done
			}
			l.Done, l.Total = done, total
		})
	}
}

func (m *Manager) phase(id, p string) { m.setLive(id, func(l *Live) { l.Phase = p }) }

func (m *Manager) checkSpace(need int64) error {
	free, _, err := fsutil.DiskSpace(m.Paths.Downloads)
	if err != nil {
		return nil // cannot tell; let the write fail if it must
	}
	if int64(free) < need+m.Reserve {
		return errPermanent{fmt.Errorf("not enough disk space: %s needed, %s free", humanBytes(need+m.Reserve), humanBytes(int64(free)))}
	}
	return nil
}

func humanBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}

func (m *Manager) runJob(ctx context.Context, job state.Job) error {
	if job.Action == "remove" {
		return m.remove(ctx, job)
	}
	c := m.Catalog()
	it := c.Item(job.ItemID)
	if it == nil {
		return errPermanent{fmt.Errorf("%s is not in the catalog", job.ItemID)}
	}
	switch it.Kind {
	case catalog.KindZim:
		return m.installZim(ctx, job, it)
	case catalog.KindMapAssets:
		return m.installAssets(ctx, job, it)
	case catalog.KindMapWorld, catalog.KindMapRegion:
		return m.installMap(ctx, job, it, c.MapBuild)
	case catalog.KindModel:
		return m.installModel(ctx, job, it)
	}
	return errPermanent{fmt.Errorf("cannot install items of kind %s", it.Kind)}
}

func (m *Manager) record(it *catalog.Item, file string, size int64) error {
	return m.Store.Update(func(s *state.State) error {
		s.Installed[it.ID] = state.Installed{ID: it.ID, Kind: it.Kind, File: file, Size: size, SHA256: it.SHA256, Date: it.Date, InstalledAt: time.Now()}
		return nil
	})
}

func (m *Manager) previous(id string) (state.Installed, bool) {
	var in state.Installed
	var ok bool
	m.Store.View(func(s *state.State) { in, ok = s.Installed[id] })
	return in, ok
}

func (m *Manager) installZim(ctx context.Context, job state.Job, it *catalog.Item) error {
	if prev, ok := m.previous(it.ID); ok && prev.File == it.File {
		return nil // already installed
	}
	dest, err := fsutil.Within(m.Paths.Zim, it.File)
	if err != nil {
		return errPermanent{err}
	}
	part, err := fsutil.Within(m.Paths.Downloads, it.File+".part")
	if err != nil {
		return errPermanent{err}
	}
	have := int64(0)
	if st, err := os.Stat(part); err == nil {
		have = st.Size()
	}
	if err := m.checkSpace(it.Size - have); err != nil {
		return err
	}
	m.phase(job.ID, "downloading")
	if err := m.DL.Fetch(ctx, it.URL, part, it.SHA256, it.Size, m.progress(job.ID)); err != nil {
		return err
	}
	m.phase(job.ID, "installing")
	if err := os.Rename(part, dest); err != nil {
		return err
	}
	fsutil.SyncDir(m.Paths.Zim)
	prev, hadPrev := m.previous(it.ID)
	if err := m.record(it, it.File, it.Size); err != nil {
		return err
	}
	if hadPrev && prev.File != it.File {
		if old, err := fsutil.Within(m.Paths.Zim, prev.File); err == nil {
			os.Remove(old)
		}
	}
	m.RebuildLibrary()
	return nil
}

func (m *Manager) installAssets(ctx context.Context, job state.Job, it *catalog.Item) error {
	if prev, ok := m.previous(it.ID); ok && prev.SHA256 == it.SHA256 {
		return nil
	}
	part, err := fsutil.Within(m.Paths.Downloads, it.File+".part")
	if err != nil {
		return errPermanent{err}
	}
	if err := m.checkSpace(it.Size * 3); err != nil {
		return err
	}
	m.phase(job.ID, "downloading")
	if err := m.DL.Fetch(ctx, it.URL, part, it.SHA256, it.Size, m.progress(job.ID)); err != nil {
		return err
	}
	m.phase(job.ID, "unpacking")
	err = maps.InstallAssets(part, m.Paths.MapAssets)
	os.Remove(part)
	if err != nil {
		return errPermanent{err}
	}
	return m.record(it, "", it.Size)
}

func (m *Manager) installMap(ctx context.Context, job state.Job, it *catalog.Item, build string) error {
	if build == "" {
		return errPermanent{errors.New("the catalog names no map build")}
	}
	file := it.ID + ".pmtiles"
	dest, err := fsutil.Within(m.Paths.Maps, file)
	if err != nil {
		return errPermanent{err}
	}
	if err := m.checkSpace(it.Size + it.Size/10); err != nil {
		return err
	}
	geometry := ""
	maxzoom := it.MaxZoom
	if it.Kind == catalog.KindMapRegion {
		m.phase(job.ID, "preparing")
		data, err := m.DL.FetchSmall(ctx, it.URL, 16<<20)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != it.SHA256 {
			return ErrChecksum
		}
		geometry, err = fsutil.Within(m.Paths.Downloads, it.ID+".geojson")
		if err != nil {
			return errPermanent{err}
		}
		if err := os.WriteFile(geometry, data, 0o644); err != nil {
			return err
		}
		defer os.Remove(geometry)
		if maxzoom == 0 {
			maxzoom = 15
		}
	} else if maxzoom == 0 {
		maxzoom = 6
	}
	tmp, err := fsutil.Within(m.Paths.Downloads, it.ID+".tmp.pmtiles")
	if err != nil {
		return errPermanent{err}
	}
	m.phase(job.ID, "downloading")
	prog := m.progress(job.ID)
	err = maps.Extract(ctx, m.Paths.PMTiles, build, geometry, maxzoom, tmp, func(n int64) { prog(n, it.Size) })
	if err != nil && ctx.Err() == nil {
		// Planet builds are kept for a limited time; fall back to the newest.
		if latest, lerr := m.latestMapBuild(ctx); lerr == nil && latest != build {
			m.logf("map build %s failed (%v); trying %s", build, err, latest)
			err = maps.Extract(ctx, m.Paths.PMTiles, latest, geometry, maxzoom, tmp, func(n int64) { prog(n, it.Size) })
		}
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	st, err := os.Stat(tmp)
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	fsutil.SyncDir(m.Paths.Maps)
	return m.record(it, file, st.Size())
}

// MapBuildsURL lists the Protomaps planet builds.
var MapBuildsURL = "https://build-metadata.protomaps.dev/builds.json"

func (m *Manager) latestMapBuild(ctx context.Context) (string, error) {
	data, err := m.DL.FetchSmall(ctx, MapBuildsURL, 4<<20)
	if err != nil {
		return "", err
	}
	var builds []struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(data, &builds); err != nil || len(builds) == 0 {
		return "", errors.New("no map builds listed")
	}
	key := builds[len(builds)-1].Key
	if !fsutil.SafeName(key) || !strings.HasSuffix(key, ".pmtiles") {
		return "", errors.New("unexpected map build name")
	}
	return "https://build.protomaps.com/" + key, nil
}

func (m *Manager) installModel(ctx context.Context, job state.Job, it *catalog.Item) error {
	if m.AI == nil {
		return errPermanent{errors.New("the AI assistant is not installed on this node")}
	}
	if _, err := m.AI.Version(ctx); err != nil {
		return fmt.Errorf("the AI service (Ollama) is not answering: %w", err)
	}
	m.phase(job.ID, "downloading")
	if err := m.AI.Pull(ctx, it.Model, m.progress(job.ID)); err != nil {
		return err
	}
	return m.record(it, "", it.Size)
}

func (m *Manager) remove(ctx context.Context, job state.Job) error {
	in, ok := m.previous(job.ItemID)
	if !ok {
		// Not installed: clean up any partial download.
		if it := m.Catalog().Item(job.ItemID); it != nil && it.File != "" {
			if p, err := fsutil.Within(m.Paths.Downloads, it.File+".part"); err == nil {
				os.Remove(p)
			}
		}
		return nil
	}
	m.phase(job.ID, "removing")
	switch in.Kind {
	case catalog.KindZim:
		if p, err := fsutil.Within(m.Paths.Zim, in.File); err == nil {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		defer m.RebuildLibrary()
	case catalog.KindMapRegion, catalog.KindMapWorld:
		if p, err := fsutil.Within(m.Paths.Maps, in.File); err == nil {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	case catalog.KindMapAssets:
		if err := os.RemoveAll(m.Paths.MapAssets); err != nil {
			return err
		}
	case catalog.KindModel:
		it := m.Catalog().Item(in.ID)
		if m.AI != nil && it != nil {
			if err := m.AI.Delete(ctx, it.Model); err != nil {
				return err
			}
		}
	}
	return m.Store.Update(func(s *state.State) error {
		delete(s.Installed, job.ItemID)
		return nil
	})
}

// libraryLoop rewrites the kiwix library, at most every two seconds.
func (m *Manager) libraryLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.kiwixCh:
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
		if m.Kiwix == nil || !m.Kiwix.Available() {
			continue
		}
		var files []string
		m.Store.View(func(s *state.State) {
			for _, in := range s.Installed {
				if in.Kind == catalog.KindZim {
					files = append(files, in.File)
				}
			}
		})
		sort.Strings(files)
		if err := m.Kiwix.Rebuild(ctx, files); err != nil {
			m.logf("library: %v", err)
		}
	}
}

// PartialBytes is the space taken by unfinished downloads.
func (m *Manager) PartialBytes() int64 {
	var n int64
	filepath.WalkDir(m.Paths.Downloads, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}
