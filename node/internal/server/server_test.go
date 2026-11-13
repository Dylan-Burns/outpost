package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dylan-Burns/outpost/node/internal/catalog"
	"github.com/Dylan-Burns/outpost/node/internal/jobs"
	"github.com/Dylan-Burns/outpost/node/internal/release"
	"github.com/Dylan-Burns/outpost/node/internal/sshsig"
	"github.com/Dylan-Burns/outpost/node/internal/state"
)

const testCode = "ABCD-EFGH-JKMN"

type env struct {
	t      *testing.T
	srv    *httptest.Server
	lib    *httptest.Server
	client *http.Client
	csrf   string
	s      *Server
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	st, err := state.Open(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	key, _ := sshsig.ParsePublicKey(release.PublicKey)
	prov, err := catalog.NewProvider(key, catalog.Embedded, catalog.EmbeddedSig, filepath.Join(dir, "cat"), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	code := testCode
	for _, d := range []string{"zim", "maps", "dl"} {
		os.MkdirAll(filepath.Join(dir, d), 0o755)
	}
	jm := &jobs.Manager{Store: st, Catalog: prov.Current, DL: jobs.NewDownloader("t"),
		Paths: jobs.Paths{Zim: filepath.Join(dir, "zim"), Maps: filepath.Join(dir, "maps"), Downloads: filepath.Join(dir, "dl")}}
	s := &Server{Version: "test", Store: st, Catalog: prov, Jobs: jm, DataDir: dir, MapsDir: filepath.Join(dir, "maps"),
		AssetsDir: filepath.Join(dir, "assets"), LibraryPort: 8081,
		SetupCode: func() string { return code }, ClearSetupCode: func() { code = "" }}
	e := &env{t: t, s: s}
	e.srv = httptest.NewServer(s.Handler())
	e.lib = httptest.NewServer(s.LibraryHandler())
	t.Cleanup(e.srv.Close)
	t.Cleanup(e.lib.Close)
	jar, _ := cookiejar.New(nil)
	e.client = &http.Client{Jar: jar}
	return e
}

func (e *env) do(method, path, body string, hdr map[string]string) (int, map[string]any) {
	e.t.Helper()
	req, _ := http.NewRequest(method, e.srv.URL+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Origin", e.srv.URL)
	for k, v := range hdr {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	resp, err := e.client.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	json.Unmarshal(b, &m)
	return resp.StatusCode, m
}

func (e *env) claim() {
	e.t.Helper()
	code, m := e.do("POST", "/api/setup/claim", `{"code":"abcd efgh jkmn","password":"a long enough password"}`, nil)
	if code != 200 {
		e.t.Fatalf("claim: %d %v", code, m)
	}
	e.csrf = m["csrf"].(string)
}

func TestClaimFlow(t *testing.T) {
	e := newEnv(t)
	if code, m := e.do("GET", "/api/status", "", nil); code != 200 || m["claimed"] != false || m["signedIn"] != false {
		t.Fatalf("status %d %v", code, m)
	}
	if code, _ := e.do("POST", "/api/setup/claim", `{"code":"WRONG-CODE-0000","password":"a long enough password"}`, nil); code != 403 {
		t.Fatalf("wrong code: %d", code)
	}
	if code, _ := e.do("POST", "/api/setup/claim", `{"code":"`+testCode+`","password":"short"}`, nil); code != 400 {
		t.Fatalf("short password: %d", code)
	}
	e.claim()
	if code, _ := e.do("POST", "/api/setup/claim", `{"code":"`+testCode+`","password":"another long password"}`, nil); code != 409 {
		t.Fatalf("second claim: %d", code)
	}
	if code, m := e.do("GET", "/api/status", "", nil); code != 200 || m["signedIn"] != true || m["csrf"] != e.csrf {
		t.Fatalf("status after claim %v", m)
	}
}

func TestAdminNeedsSessionAndCSRF(t *testing.T) {
	e := newEnv(t)
	if code, _ := e.do("GET", "/api/catalog", "", nil); code != 401 {
		t.Fatalf("catalog without session: %d", code)
	}
	e.claim()
	if code, m := e.do("GET", "/api/catalog", "", nil); code != 200 || m["catalog"] == nil {
		t.Fatalf("catalog: %d", code)
	}
	sel := `{"selection":{"wikipedia":"top-mini","tiers":{"survival":"essential"},"regions":[],"models":[]},"apps":{"library":true,"maps":true,"assistant":false}}`
	if code, _ := e.do("POST", "/api/selection", sel, nil); code != 403 {
		t.Fatalf("selection without CSRF: %d", code)
	}
	if code, _ := e.do("POST", "/api/selection", sel, map[string]string{"X-CSRF-Token": "nope"}); code != 403 {
		t.Fatalf("selection with wrong CSRF: %d", code)
	}
	code, m := e.do("POST", "/api/setup/complete", sel, map[string]string{"X-CSRF-Token": e.csrf})
	if code != 200 {
		t.Fatalf("complete: %d %v", code, m)
	}
	if n := len(m["items"].([]any)); n < 5 {
		t.Fatalf("resolved %d items", n)
	}
	if code, m := e.do("GET", "/api/jobs", "", nil); code != 200 || len(m["jobs"].([]any)) == 0 {
		t.Fatalf("jobs: %d %v", code, m)
	}
	bad := `{"selection":{"wikipedia":"nope","tiers":{}}}`
	if code, _ := e.do("POST", "/api/selection", bad, map[string]string{"X-CSRF-Token": e.csrf}); code != 400 {
		t.Fatalf("bad selection: %d", code)
	}
	unknown := `{"selection":{"tiers":{}},"evil":true}`
	if code, _ := e.do("POST", "/api/selection", unknown, map[string]string{"X-CSRF-Token": e.csrf}); code != 400 {
		t.Fatalf("unknown field: %d", code)
	}
}

func TestCrossSiteWritesRefused(t *testing.T) {
	e := newEnv(t)
	if code, _ := e.do("POST", "/api/login", `{"password":"x"}`, map[string]string{"Origin": "http://evil.example"}); code != 403 {
		t.Fatalf("cross-origin login: %d", code)
	}
	if code, _ := e.do("POST", "/api/login", `{"password":"x"}`, map[string]string{"Origin": "", "Sec-Fetch-Site": "cross-site"}); code != 403 {
		t.Fatalf("cross-site fetch: %d", code)
	}
	req, _ := http.NewRequest("POST", e.srv.URL+"/api/login", strings.NewReader("password=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", e.srv.URL)
	resp, _ := e.client.Do(req)
	if resp.StatusCode != 415 {
		t.Fatalf("form post: %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestLoginRateLimit(t *testing.T) {
	e := newEnv(t)
	e.claim()
	e.do("POST", "/api/logout", "", nil)
	for i := 0; i < 10; i++ {
		if code, _ := e.do("POST", "/api/login", `{"password":"wrong password!"}`, nil); code != 401 {
			t.Fatalf("attempt %d: %d", i, code)
		}
	}
	if code, _ := e.do("POST", "/api/login", `{"password":"a long enough password"}`, nil); code != 429 {
		t.Fatalf("not rate limited: %d", code)
	}
}

func TestLogoutAndPasswordChange(t *testing.T) {
	e := newEnv(t)
	e.claim()
	if code, _ := e.do("POST", "/api/password", `{"current":"wrong","new":"new long password"}`, map[string]string{"X-CSRF-Token": e.csrf}); code != 401 {
		t.Fatalf("wrong current: %d", code)
	}
	if code, _ := e.do("POST", "/api/password", `{"current":"a long enough password","new":"new long password"}`, map[string]string{"X-CSRF-Token": e.csrf}); code != 200 {
		t.Fatalf("change: %d", code)
	}
	e.do("POST", "/api/logout", "", nil)
	if code, _ := e.do("GET", "/api/catalog", "", nil); code != 401 {
		t.Fatalf("still signed in after logout: %d", code)
	}
	if code, _ := e.do("POST", "/api/login", `{"password":"new long password"}`, nil); code != 200 {
		t.Fatalf("login with new password: %d", code)
	}
}

func TestPrivateMode(t *testing.T) {
	e := newEnv(t)
	e.claim()
	sel := `{"selection":{"tiers":{}},"apps":{"library":true,"maps":true,"assistant":false}}`
	e.do("POST", "/api/setup/complete", sel, map[string]string{"X-CSRF-Token": e.csrf})
	if code, _ := e.do("PUT", "/api/settings", `{"nodeName":"Cabin","publicAccess":false,"assistantPublic":false,"checkForUpdates":false}`, map[string]string{"X-CSRF-Token": e.csrf}); code != 200 {
		t.Fatalf("settings: %d", code)
	}
	if code, _ := e.do("GET", "/api/library", "", nil); code != 200 {
		t.Fatalf("admin cannot read library: %d", code)
	}
	anon := &http.Client{}
	resp, _ := anon.Get(e.srv.URL + "/api/library")
	if resp.StatusCode != 401 {
		t.Fatalf("anonymous library in private mode: %d", resp.StatusCode)
	}
	resp.Body.Close()
	resp, _ = anon.Get(e.lib.URL + "/")
	if resp.StatusCode != 401 {
		t.Fatalf("anonymous library port in private mode: %d", resp.StatusCode)
	}
	resp.Body.Close()
	resp, _ = anon.Get(e.srv.URL + "/api/status")
	if resp.StatusCode != 200 {
		t.Fatalf("status must stay public: %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHeadersAndPaths(t *testing.T) {
	e := newEnv(t)
	resp, _ := http.Get(e.srv.URL + "/")
	if resp.Header.Get("Content-Security-Policy") == "" || resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatal("security headers missing")
	}
	resp.Body.Close()
	for _, p := range []string{"/maps/files/..%2fstate.json", "/maps/files/state.json", "/maps/assets/..%2f..%2fstate.json", "/maps/assets/fonts/../../state.json"} {
		resp, _ := http.Get(e.srv.URL + p)
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if strings.Contains(string(b), "adminHash") || strings.Contains(string(b), "sessions") {
			t.Fatalf("%s leaked state", p)
		}
	}
	resp, _ = http.Get(e.srv.URL + "/wizard")
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), "<") || resp.StatusCode != 200 {
		t.Fatal("SPA fallback failed")
	}
	resp, _ = http.Get(e.srv.URL + "/api/nope")
	if resp.StatusCode != 404 {
		t.Fatal("unknown api not 404")
	}
	resp.Body.Close()
	// The library port is read only.
	resp, _ = http.Post(e.lib.URL+"/", "text/plain", strings.NewReader("x"))
	if resp.StatusCode != 405 {
		t.Fatalf("library POST: %d", resp.StatusCode)
	}
	resp.Body.Close()
}
