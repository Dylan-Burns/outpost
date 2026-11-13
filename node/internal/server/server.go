// Package server is the node's web server: the dashboard and its JSON API on
// one port, and the library (kiwix-serve, proxied) on a second port so the
// archives' own pages and scripts run in a separate browser origin from the
// admin interface.
package server

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Dylan-Burns/outpost/node/internal/ai"
	"github.com/Dylan-Burns/outpost/node/internal/auth"
	"github.com/Dylan-Burns/outpost/node/internal/catalog"
	"github.com/Dylan-Burns/outpost/node/internal/jobs"
	"github.com/Dylan-Burns/outpost/node/internal/kiwix"
	"github.com/Dylan-Burns/outpost/node/internal/state"
)

//go:embed all:ui/dist
var uiFS embed.FS

// Server holds everything the handlers need.
type Server struct {
	Version     string
	Store       *state.Store
	Catalog     *catalog.Provider
	Jobs        *jobs.Manager
	Kiwix       *kiwix.Manager
	AI          *ai.Client
	Logger      *log.Logger
	DataDir     string
	MapsDir     string
	AssetsDir   string
	LibraryPort int
	SiteURL     string // e.g. https://outpost-offline.vercel.app
	// SetupCode returns the current one-time setup code ("" once claimed).
	SetupCode func() string
	// ClearSetupCode is called once the node has been claimed.
	ClearSetupCode func()

	loginLimit *auth.Limiter
	once       sync.Once
	latestMu   sync.Mutex
	latest     string
	latestAt   time.Time
}

const (
	cookieName  = "outpost_session"
	sessionLife = 30 * 24 * time.Hour
	maxBody     = 1 << 20
)

func (s *Server) init() {
	s.once.Do(func() {
		s.loginLimit = auth.NewLimiter(15*time.Minute, 10)
	})
}

func (s *Server) logf(format string, args ...any) {
	if s.Logger != nil {
		s.Logger.Printf(format, args...)
	}
}

// Handler returns the dashboard handler.
func (s *Server) Handler() http.Handler {
	s.init()
	mux := http.NewServeMux()
	// Public (or, in private mode, signed-in) reads.
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/library", s.viewer(s.handleLibrary))
	mux.HandleFunc("GET /api/search", s.viewer(s.handleSearch))
	mux.HandleFunc("GET /api/maps", s.viewer(s.handleMaps))
	mux.HandleFunc("GET /maps/files/{file}", s.viewer(s.handleMapFile))
	mux.HandleFunc("GET /maps/assets/{path...}", s.viewer(s.handleMapAsset))
	mux.HandleFunc("GET /api/assistant", s.assistantUser(s.handleAssistantInfo))
	mux.HandleFunc("POST /api/assistant/chat", s.assistantUser(s.handleChat))
	// Account.
	mux.HandleFunc("POST /api/setup/claim", s.handleClaim)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	// Admin.
	mux.HandleFunc("GET /api/catalog", s.admin(s.handleCatalog))
	mux.HandleFunc("POST /api/selection", s.admin(s.handleSelection))
	mux.HandleFunc("POST /api/setup/complete", s.admin(s.handleSetupComplete))
	mux.HandleFunc("GET /api/jobs", s.admin(s.handleJobs))
	mux.HandleFunc("POST /api/jobs/{id}/cancel", s.admin(s.handleJobCancel))
	mux.HandleFunc("POST /api/jobs/{id}/retry", s.admin(s.handleJobRetry))
	mux.HandleFunc("POST /api/jobs/clear", s.admin(s.handleJobsClear))
	mux.HandleFunc("POST /api/updates/apply", s.admin(s.handleUpdatesApply))
	mux.HandleFunc("POST /api/catalog/refresh", s.admin(s.handleCatalogRefresh))
	mux.HandleFunc("GET /api/system", s.admin(s.handleSystem))
	mux.HandleFunc("PUT /api/settings", s.admin(s.handleSettings))
	mux.HandleFunc("POST /api/password", s.admin(s.handlePassword))
	mux.HandleFunc("GET /", s.handleUI)
	return s.secure(mux)
}

// secure adds security headers, limits bodies and blocks cross-site writes.
func (s *Server) secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; worker-src 'self' blob:; child-src blob:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'; object-src 'none'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if !sameOrigin(r) {
				writeErr(w, http.StatusForbidden, "cross-site request refused")
				return
			}
			if r.ContentLength != 0 && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				writeErr(w, http.StatusUnsupportedMediaType, "send JSON")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		}
		next.ServeHTTP(w, r)
	})
}

// sameOrigin accepts a state-changing request only when the browser says it
// comes from this same origin.
func sameOrigin(r *http.Request) bool {
	if o := r.Header.Get("Origin"); o != "" {
		u, err := url.Parse(o)
		return err == nil && u.Host == r.Host && (u.Scheme == "http" || u.Scheme == "https")
	}
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return true
	case "":
		// Non-browser clients (curl, scripts) send neither header.
		return r.Header.Get("Referer") == "" || refererMatches(r)
	}
	return false
}

func refererMatches(r *http.Request) bool {
	u, err := url.Parse(r.Header.Get("Referer"))
	return err == nil && u.Host == r.Host
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeErr(w, http.StatusRequestEntityTooLarge, "request too large")
		} else {
			writeErr(w, http.StatusBadRequest, "malformed request")
		}
		return false
	}
	if dec.More() {
		writeErr(w, http.StatusBadRequest, "malformed request")
		return false
	}
	return true
}

// clientKey groups requests by client network: the address for IPv4, the
// /64 prefix for IPv6 (a single host usually controls a whole /64).
func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return host
	}
	if ip.To4() == nil {
		return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
	}
	return ip.String()
}

// session returns the valid session for the request, if any.
func (s *Server) session(r *http.Request) (key string, sess state.Session, ok bool) {
	c, err := r.Cookie(cookieName)
	if err != nil || len(c.Value) < 20 || len(c.Value) > 100 {
		return "", sess, false
	}
	key = auth.TokenKey(c.Value)
	s.Store.View(func(st *state.State) { sess, ok = st.Sessions[key] })
	if ok && time.Now().After(sess.Expires) {
		ok = false
	}
	return key, sess, ok
}

func (s *Server) newSession(w http.ResponseWriter, r *http.Request) (string, error) {
	tok, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	csrf, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	now := time.Now()
	err = s.Store.Update(func(st *state.State) error {
		for k, v := range st.Sessions {
			if now.After(v.Expires) {
				delete(st.Sessions, k)
			}
		}
		if len(st.Sessions) > 50 { // drop the oldest when too many
			var oldestK string
			var oldest time.Time
			for k, v := range st.Sessions {
				if oldestK == "" || v.Created.Before(oldest) {
					oldestK, oldest = k, v.Created
				}
			}
			delete(st.Sessions, oldestK)
		}
		st.Sessions[auth.TokenKey(tok)] = state.Session{CSRF: csrf, Created: now, Expires: now.Add(sessionLife)}
		return nil
	})
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: tok, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode,
		Secure: r.TLS != nil, MaxAge: int(sessionLife.Seconds())})
	return csrf, nil
}

func (s *Server) publicAccess() bool {
	var pub bool
	s.Store.View(func(st *state.State) { pub = st.Settings.PublicAccess || !st.SetupComplete })
	return pub
}

// viewer allows the request when the node is public or the user signed in.
func (s *Server) viewer(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.publicAccess() {
			if _, _, ok := s.session(r); !ok {
				writeErr(w, http.StatusUnauthorized, "sign in to use this node")
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) assistantUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var pub bool
		s.Store.View(func(st *state.State) { pub = st.Settings.PublicAccess && st.Settings.AssistantPublic })
		if !pub {
			if _, _, ok := s.session(r); !ok {
				writeErr(w, http.StatusUnauthorized, "sign in to use the assistant")
				return
			}
		}
		next(w, r)
	}
}

// admin requires a session and, for writes, the session's CSRF token.
func (s *Server) admin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, sess, ok := s.session(r)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "sign in as the administrator")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if got := r.Header.Get("X-CSRF-Token"); got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(sess.CSRF)) != 1 {
				writeErr(w, http.StatusForbidden, "missing or wrong CSRF token")
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/maps/") {
		writeErr(w, http.StatusNotFound, "no such endpoint")
		return
	}
	sub, _ := fs.Sub(uiFS, "ui/dist")
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p != "" {
		if f, err := sub.Open(p); err == nil {
			st, _ := f.Stat()
			f.Close()
			if st != nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				http.FileServerFS(sub).ServeHTTP(w, r)
				return
			}
		}
		if strings.Contains(p, ".") && !strings.HasSuffix(p, ".html") {
			http.NotFound(w, r)
			return
		}
	}
	// Single-page app: every other path gets index.html.
	f, err := sub.Open("index.html")
	if err != nil {
		http.Error(w, "interface missing", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	io.Copy(w, f.(io.Reader))
}

// Serve runs the dashboard and library servers until ctx ends.
func (s *Server) Serve(ctx context.Context, dashAddr, libAddr string) error {
	dash := &http.Server{Addr: dashAddr, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 64 << 10}
	lib := &http.Server{Addr: libAddr, Handler: s.LibraryHandler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 64 << 10}
	errc := make(chan error, 2)
	go func() { errc <- dash.ListenAndServe() }()
	go func() { errc <- lib.ListenAndServe() }()
	select {
	case <-ctx.Done():
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			dash.Shutdown(sctx)
			lib.Shutdown(sctx)
			cancel()
			return err
		}
	}
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dash.Shutdown(sctx)
	lib.Shutdown(sctx)
	return nil
}
