package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Dylan-Burns/outpost/node/internal/auth"
	"github.com/Dylan-Burns/outpost/node/internal/catalog"
	"github.com/Dylan-Burns/outpost/node/internal/fsutil"
	"github.com/Dylan-Burns/outpost/node/internal/state"
	"github.com/Dylan-Burns/outpost/node/internal/sysinfo"
)

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	_, sess, signedIn := s.session(r)
	var st state.State
	s.Store.View(func(x *state.State) {
		st.SetupComplete = x.SetupComplete
		st.Settings = x.Settings
		st.Apps = x.Apps
		st.AdminHash = x.AdminHash
	})
	aiVersion := ""
	if s.AI != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
		aiVersion, _ = s.AI.Version(ctx)
		cancel()
	}
	resp := map[string]any{
		"version":         s.Version,
		"nodeName":        st.Settings.NodeName,
		"setupComplete":   st.SetupComplete,
		"claimed":         st.AdminHash != "",
		"signedIn":        signedIn,
		"publicAccess":    st.Settings.PublicAccess,
		"assistantPublic": st.Settings.AssistantPublic,
		"apps":            st.Apps,
		"libraryPort":     s.LibraryPort,
		"libraryRunning":  s.Kiwix != nil && s.Kiwix.Running(),
		"aiAvailable":     aiVersion != "",
		"aiVersion":       aiVersion,
		"latestVersion":   s.latestVersion(),
	}
	if signedIn {
		resp["csrf"] = sess.CSRF
	}
	writeJSON(w, http.StatusOK, resp)
}

// latestVersion returns the newest released version, checked at most every
// six hours and only when the owner allows update checks.
func (s *Server) latestVersion() string {
	var allowed bool
	s.Store.View(func(st *state.State) { allowed = st.Settings.CheckForUpdates })
	s.latestMu.Lock()
	defer s.latestMu.Unlock()
	if !allowed || s.SiteURL == "" || s.Jobs == nil || s.Jobs.DL == nil {
		return s.latest
	}
	if time.Since(s.latestAt) > 6*time.Hour {
		s.latestAt = time.Now()
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			data, err := s.Jobs.DL.FetchSmall(ctx, s.SiteURL+"/releases/latest.json", 64<<10)
			if err != nil {
				return
			}
			var v struct {
				Version string `json:"version"`
			}
			if json.Unmarshal(data, &v) == nil && len(v.Version) < 40 {
				s.latestMu.Lock()
				s.latest = v.Version
				s.latestMu.Unlock()
			}
		}()
	}
	return s.latest
}

type bookView struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	Date        string `json:"date"`
	Category    string `json:"category"`
	Notice      string `json:"notice,omitempty"`
}

func (s *Server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	c := s.Catalog.Current()
	catOf := map[string]string{}
	for _, cat := range c.Categories {
		for _, t := range catalog.TierNames {
			for _, id := range cat.Tiers[t] {
				if catOf[id] == "" {
					catOf[id] = cat.Name
				}
			}
		}
		for _, o := range cat.Options {
			for _, id := range o.Items {
				catOf[id] = cat.Name
			}
		}
	}
	var books []bookView
	s.Store.View(func(st *state.State) {
		for id, in := range st.Installed {
			if in.Kind != catalog.KindZim {
				continue
			}
			b := bookView{ID: id, Title: id, Size: in.Size, Date: in.Date, Category: catOf[id]}
			if it := c.Item(id); it != nil {
				b.Title, b.Description, b.Notice = it.Title, it.Description, it.Notice
			}
			// kiwix-serve runs with --nodatealiases: the file name without
			// its _YYYY-MM date is a stable address.
			b.Path = "/content/" + undated(strings.TrimSuffix(in.File, ".zim"))
			books = append(books, b)
		}
	})
	sort.Slice(books, func(a, b int) bool { return books[a].Title < books[b].Title })
	writeJSON(w, http.StatusOK, map[string]any{"books": books, "running": s.Kiwix != nil && s.Kiwix.Running()})
}

func undated(name string) string {
	if i := strings.LastIndexByte(name, '_'); i > 0 {
		d := name[i+1:]
		if len(d) == 7 && d[4] == '-' {
			if _, err := strconv.Atoi(d[:4] + d[5:]); err == nil {
				return name[:i]
			}
		}
	}
	return name
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || len(q) > 300 {
		writeErr(w, http.StatusBadRequest, "enter between 1 and 300 characters")
		return
	}
	start, _ := strconv.Atoi(r.URL.Query().Get("start"))
	if start < 0 || start > 10000 {
		start = 0
	}
	if s.Kiwix == nil || !s.Kiwix.Running() {
		writeErr(w, http.StatusServiceUnavailable, "the library is not running")
		return
	}
	res, total, err := s.Kiwix.Search(r.Context(), q, start, 20)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "search failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": res, "total": total, "start": start})
}

type regionView struct {
	ID    string    `json:"id"`
	Name  string    `json:"name"`
	Group string    `json:"group"`
	BBox  []float64 `json:"bbox"`
	File  string    `json:"file"`
}

func (s *Server) handleMaps(w http.ResponseWriter, r *http.Request) {
	c := s.Catalog.Current()
	var regions []regionView
	world, assets := "", false
	s.Store.View(func(st *state.State) {
		for id, in := range st.Installed {
			switch in.Kind {
			case catalog.KindMapRegion:
				rv := regionView{ID: id, Name: id, File: in.File}
				if it := c.Item(id); it != nil {
					rv.Name, rv.Group, rv.BBox = it.Title, it.Group, it.BBox
				}
				regions = append(regions, rv)
			case catalog.KindMapWorld:
				world = in.File
			case catalog.KindMapAssets:
				assets = true
			}
		}
	})
	sort.Slice(regions, func(a, b int) bool { return regions[a].Name < regions[b].Name })
	writeJSON(w, http.StatusOK, map[string]any{"regions": regions, "world": world, "assets": assets})
}

func (s *Server) handleMapFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if !strings.HasSuffix(name, ".pmtiles") {
		http.NotFound(w, r)
		return
	}
	p, err := fsutil.Within(s.MapsDir, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(p)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.pmtiles")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, name, st.ModTime(), f)
}

func (s *Server) handleMapAsset(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("path")
	clean := filepath.Clean("/" + rel)
	if clean != "/"+rel || strings.Contains(rel, "\\") {
		http.NotFound(w, r)
		return
	}
	p := filepath.Join(s.AssetsDir, filepath.FromSlash(clean))
	if !strings.HasPrefix(p, filepath.Clean(s.AssetsDir)+string(filepath.Separator)) {
		http.NotFound(w, r)
		return
	}
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		// Missing glyph ranges are normal; answer with an empty tile.
		if strings.HasSuffix(p, ".pbf") {
			w.Header().Set("Content-Type", "application/x-protobuf")
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if strings.HasSuffix(p, ".pbf") {
		w.Header().Set("Content-Type", "application/x-protobuf")
	}
	http.ServeFile(w, r, p)
}

// --- account ---------------------------------------------------------------

func (s *Server) handleClaim(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	key := clientKey(r)
	if !s.admit(r) {
		writeErr(w, http.StatusTooManyRequests, "too many attempts; wait 15 minutes")
		return
	}
	var claimed bool
	s.Store.View(func(st *state.State) { claimed = st.AdminHash != "" })
	if claimed {
		writeErr(w, http.StatusConflict, "this node already has an administrator")
		return
	}
	code := ""
	if s.SetupCode != nil {
		code = s.SetupCode()
	}
	if ok, _ := s.checkCredential(func() bool { return auth.SetupCodeMatches(code, req.Code) }); !ok {
		writeErr(w, http.StatusForbidden, "that setup code is not right")
		return
	}
	var hash string
	if err := auth.CheckPasswordPolicy(req.Password); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var herr error
	if err := auth.WithHashSlot(func() { hash, herr = auth.HashPassword(req.Password) }); err != nil {
		writeErr(w, http.StatusTooManyRequests, "the node is busy; try again in a moment")
		return
	}
	if herr != nil {
		writeErr(w, http.StatusBadRequest, herr.Error())
		return
	}
	err := s.Store.Update(func(st *state.State) error {
		if st.AdminHash != "" {
			return errors.New("already claimed")
		}
		st.AdminHash = hash
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusConflict, "this node already has an administrator")
		return
	}
	if s.ClearSetupCode != nil {
		s.ClearSetupCode()
	}
	s.loginLimit.Reset(key)
	csrf, err := s.newSession(w, r)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not sign in")
		return
	}
	s.logf("node claimed from %s", key)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "csrf": csrf})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	key := clientKey(r)
	if !s.admit(r) {
		writeErr(w, http.StatusTooManyRequests, "too many attempts; wait 15 minutes")
		return
	}
	var hash string
	s.Store.View(func(st *state.State) { hash = st.AdminHash })
	ok, busy := s.checkCredential(func() bool { return hash != "" && auth.VerifyPassword(hash, req.Password) })
	if busy {
		writeErr(w, http.StatusTooManyRequests, "the node is busy; try again in a moment")
		return
	}
	if !ok {
		writeErr(w, http.StatusUnauthorized, "wrong password")
		return
	}
	s.loginLimit.Reset(key)
	csrf, err := s.newSession(w, r)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not sign in")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "csrf": csrf})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if key, _, ok := s.session(r); ok {
		s.Store.Update(func(st *state.State) error { delete(st.Sessions, key); return nil })
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	key := clientKey(r)
	if !s.admit(r) {
		writeErr(w, http.StatusTooManyRequests, "too many attempts; wait 15 minutes")
		return
	}
	if err := auth.CheckPasswordPolicy(req.New); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var hash, nh string
	s.Store.View(func(st *state.State) { hash = st.AdminHash })
	var herr error
	ok, busy := s.checkCredential(func() bool {
		if !auth.VerifyPassword(hash, req.Current) {
			return false
		}
		nh, herr = auth.HashPassword(req.New)
		return true
	})
	if busy {
		writeErr(w, http.StatusTooManyRequests, "the node is busy; try again in a moment")
		return
	}
	if !ok {
		writeErr(w, http.StatusUnauthorized, "the current password is wrong")
		return
	}
	if herr != nil {
		writeErr(w, http.StatusBadRequest, herr.Error())
		return
	}
	s.loginLimit.Reset(key)
	// Changing the password signs out every other browser.
	cur, _, _ := s.session(r)
	s.Store.Update(func(st *state.State) error {
		st.AdminHash = nh
		for k := range st.Sessions {
			if k != cur {
				delete(st.Sessions, k)
			}
		}
		return nil
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// --- content ---------------------------------------------------------------

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	c := s.Catalog.Current()
	var sel catalog.Selection
	var apps state.Apps
	installed := map[string]state.Installed{}
	s.Store.View(func(st *state.State) {
		sel = st.Selection
		apps = st.Apps
		for k, v := range st.Installed {
			installed[k] = v
		}
	})
	free, total, _ := fsutil.DiskSpace(s.DataDir)
	var used int64
	for _, in := range installed {
		used += in.Size
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"catalog":   c,
		"selection": sel,
		"apps":      apps,
		"installed": installed,
		"outdated":  s.Jobs.Outdated(),
		"disk":      map[string]any{"free": free, "total": total, "used": used, "partial": s.Jobs.PartialBytes()},
		"system":    sysinfo.Collect(),
	})
}

type selectionReq struct {
	Selection catalog.Selection `json:"selection"`
	Apps      *state.Apps       `json:"apps,omitempty"`
}

func (s *Server) applySelection(w http.ResponseWriter, req selectionReq, complete bool) {
	c := s.Catalog.Current()
	sel := req.Selection
	if sel.Tiers == nil {
		sel.Tiers = map[string]string{}
	}
	apps := state.Apps{Library: true, Maps: true}
	if req.Apps != nil {
		apps = *req.Apps
	}
	// Disabled apps contribute nothing.
	if !apps.Library {
		sel.Wikipedia = ""
		sel.Tiers = map[string]string{}
		sel.Extra = nil
	}
	if !apps.Maps {
		sel.Regions = nil
	}
	if !apps.Assistant {
		sel.Models = nil
	}
	if len(sel.Regions) > 400 || len(sel.Models) > 50 || len(sel.Extra) > 500 || len(sel.Tiers) > 100 {
		writeErr(w, http.StatusBadRequest, "selection too large")
		return
	}
	want, err := c.Resolve(sel)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	err = s.Store.Update(func(st *state.State) error {
		st.Selection = sel
		st.Apps = apps
		if complete {
			st.SetupComplete = true
		}
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not save the selection")
		return
	}
	// Installed items of a kind the catalog has no say over (e.g. the
	// model list when the assistant is off) still follow the selection.
	if err := s.Jobs.Apply(want); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "items": want, "size": c.TotalSize(want)})
}

func (s *Server) handleSelection(w http.ResponseWriter, r *http.Request) {
	var req selectionReq
	if !readJSON(w, r, &req) {
		return
	}
	s.applySelection(w, req, false)
}

func (s *Server) handleSetupComplete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		selectionReq
		NodeName string `json:"nodeName"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if name := strings.TrimSpace(req.NodeName); name != "" {
		if len(name) > 60 {
			writeErr(w, http.StatusBadRequest, "the name is too long")
			return
		}
		s.Store.Update(func(st *state.State) error { st.Settings.NodeName = name; return nil })
	}
	s.applySelection(w, req.selectionReq, true)
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"jobs": s.Jobs.Snapshot()})
}

func (s *Server) handleJobCancel(w http.ResponseWriter, r *http.Request) {
	if err := s.Jobs.Cancel(r.PathValue("id")); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleJobRetry(w http.ResponseWriter, r *http.Request) {
	if err := s.Jobs.Retry(r.PathValue("id")); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleJobsClear(w http.ResponseWriter, r *http.Request) {
	if err := s.Jobs.ClearFinished(); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUpdatesApply(w http.ResponseWriter, r *http.Request) {
	n, err := s.Jobs.UpdateAll()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "queued": n})
}

func (s *Server) handleCatalogRefresh(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	changed, err := s.Catalog.Refresh(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "could not fetch the catalog: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "changed": changed, "sequence": s.Catalog.Current().Sequence})
}

func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	free, total, _ := fsutil.DiskSpace(s.DataDir)
	var models any
	if s.AI != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		models, _ = s.AI.List(ctx)
		cancel()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"system":  sysinfo.Collect(),
		"disk":    map[string]any{"free": free, "total": total, "path": s.DataDir},
		"version": s.Version,
		"catalog": map[string]any{"sequence": s.Catalog.Current().Sequence, "generated": s.Catalog.Current().Generated},
		"library": map[string]any{"running": s.Kiwix != nil && s.Kiwix.Running(), "books": s.kiwixBooks()},
		"models":  models,
	})
}

func (s *Server) kiwixBooks() int {
	if s.Kiwix == nil {
		return 0
	}
	return s.Kiwix.Books()
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	var req state.Settings
	if !readJSON(w, r, &req) {
		return
	}
	req.NodeName = strings.TrimSpace(req.NodeName)
	if req.NodeName == "" || len(req.NodeName) > 60 {
		writeErr(w, http.StatusBadRequest, "the name must have 1 to 60 characters")
		return
	}
	s.Store.Update(func(st *state.State) error { st.Settings = req; return nil })
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
