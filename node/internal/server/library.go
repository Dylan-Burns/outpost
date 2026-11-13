package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

const unavailablePage = `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Library starting</title><style>body{font:16px/1.5 system-ui,sans-serif;max-width:36rem;margin:15vh auto;padding:0 1rem;color:#222}</style>
<h1>The library is starting</h1><p>Outpost is loading your archives. This page will work again in a few seconds.</p>
<p><a href="javascript:location.reload()">Try again</a></p>`

const signInPage = `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Sign in</title><style>body{font:16px/1.5 system-ui,sans-serif;max-width:36rem;margin:15vh auto;padding:0 1rem;color:#222}</style>
<h1>This library is private</h1><p>Sign in on the Outpost dashboard of this node, then come back.</p>`

const libraryCSP = "default-src 'self' data: blob:; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-src 'self'; form-action 'self'; frame-ancestors 'self'; base-uri 'self'; object-src 'none'"

// LibraryHandler proxies kiwix-serve on the library port. It runs in a
// separate browser origin from the dashboard, so scripts inside archives
// cannot reach the admin interface's pages.
func (s *Server) LibraryHandler() http.Handler {
	target, _ := url.Parse("http://127.0.0.1")
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			t := *target
			t.Host = strings.TrimPrefix(s.Kiwix.Base(), "http://")
			pr.SetURL(&t)
			pr.Out.Host = t.Host
			pr.Out.Header.Del("Cookie") // kiwix has no use for the dashboard session
		},
		FlushInterval: 100 * time.Millisecond,
		ModifyResponse: func(resp *http.Response) error {
			h := resp.Header
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("X-Frame-Options", "SAMEORIGIN")
			// Archive pages may carry scripts; keep them from reaching other
			// hosts (no tracking, no probing the LAN from visitors' browsers).
			h.Set("Content-Security-Policy", libraryCSP)
			h.Set("Cross-Origin-Resource-Policy", "same-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
			h.Del("Server")
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Retry-After", "5")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(unavailablePage))
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "read only", http.StatusMethodNotAllowed)
			return
		}
		if !s.publicAccess() {
			if _, _, ok := s.session(r); !ok {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(signInPage))
				return
			}
		}
		if s.Kiwix == nil || !s.Kiwix.Running() {
			proxy.ErrorHandler(w, r, nil)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}
