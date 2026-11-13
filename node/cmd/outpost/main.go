// Command outpost is the Outpost node: an offline knowledge server for
// hardware you already own.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Dylan-Burns/outpost/node/internal/ai"
	"github.com/Dylan-Burns/outpost/node/internal/auth"
	"github.com/Dylan-Burns/outpost/node/internal/catalog"
	"github.com/Dylan-Burns/outpost/node/internal/fsutil"
	"github.com/Dylan-Burns/outpost/node/internal/jobs"
	"github.com/Dylan-Burns/outpost/node/internal/kiwix"
	"github.com/Dylan-Burns/outpost/node/internal/pdfindex"
	"github.com/Dylan-Burns/outpost/node/internal/release"
	"github.com/Dylan-Burns/outpost/node/internal/server"
	"github.com/Dylan-Burns/outpost/node/internal/sshsig"
	"github.com/Dylan-Burns/outpost/node/internal/state"
)

var version = "dev"

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func usage() {
	fmt.Fprintf(os.Stderr, `Outpost %s — offline knowledge server

Usage:
  outpost serve          run the node (used by the system service)
  outpost setup-code     print the one-time code that claims a new node
  outpost reset-admin    forget the admin password and print a new setup code
  outpost version        print the version

Settings come from flags or environment variables (see "outpost serve -h").
`, version)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		os.Exit(serve(os.Args[2:]))
	case "pdftext": // internal: the PDF reader's sandbox, see pdfindex.Subprocess
		os.Exit(pdfindex.ServeExtract())
	case "setup-code":
		os.Exit(printSetupCode(os.Args[2:]))
	case "reset-admin":
		os.Exit(resetAdmin(os.Args[2:]))
	case "version", "--version", "-v":
		fmt.Println(version)
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

type paths struct {
	data, bin string
}

func commonFlags(fs *flag.FlagSet) *paths {
	p := &paths{}
	fs.StringVar(&p.data, "data", env("OUTPOST_DATA_DIR", "/var/lib/outpost"), "data directory (content and state)")
	fs.StringVar(&p.bin, "bin", env("OUTPOST_BIN_DIR", "/opt/outpost/bin"), "directory holding kiwix-serve, kiwix-manage and pmtiles")
	return p
}

func setupCodePath(data string) string { return filepath.Join(data, "setup-code") }

// ensureSetupCode creates the setup code file when the node is unclaimed.
func ensureSetupCode(data string, claimed bool) (string, error) {
	p := setupCodePath(data)
	if claimed {
		os.Remove(p)
		return "", nil
	}
	if b, err := os.ReadFile(p); err == nil && len(strings.TrimSpace(string(b))) >= 12 {
		return strings.TrimSpace(string(b)), nil
	}
	code, err := auth.NewSetupCode()
	if err != nil {
		return "", err
	}
	return code, fsutil.WriteFileAtomic(p, []byte(code+"\n"), 0o600)
}

func serve(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	p := commonFlags(fs)
	listen := fs.String("listen", env("OUTPOST_LISTEN", ":8080"), "dashboard address")
	libListen := fs.String("library-listen", env("OUTPOST_LIBRARY_LISTEN", ":8081"), "library address")
	kiwixPort := fs.Int("kiwix-port", atoi(env("OUTPOST_KIWIX_PORT", "18080")), "loopback port for kiwix-serve")
	ollama := fs.String("ollama", env("OUTPOST_OLLAMA_URL", "http://127.0.0.1:11434"), "Ollama API address")
	site := fs.String("site", env("OUTPOST_SITE_URL", release.SiteURL), "Outpost website (catalog and releases)")
	fs.Parse(args)

	logger := log.New(os.Stderr, "", log.LstdFlags)
	dirs := map[string]string{
		"zim": filepath.Join(p.data, "content", "zim"), "maps": filepath.Join(p.data, "content", "maps"),
		"assets": filepath.Join(p.data, "content", "map-assets"), "downloads": filepath.Join(p.data, "downloads"),
		"kiwix": filepath.Join(p.data, "kiwix"), "catalog": filepath.Join(p.data, "catalog"),
	}
	if err := os.MkdirAll(p.data, 0o750); err != nil {
		logger.Printf("cannot create %s: %v", p.data, err)
		return 1
	}
	for k, d := range dirs {
		if k == "assets" {
			continue // created when the assets are installed
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			logger.Printf("cannot create %s: %v", d, err)
			return 1
		}
	}
	store, err := state.Open(filepath.Join(p.data, "state.json"))
	if err != nil {
		logger.Printf("state: %v", err)
		return 1
	}
	key, err := sshsig.ParsePublicKey(release.PublicKey)
	if err != nil {
		logger.Printf("release key: %v", err)
		return 1
	}
	dl := jobs.NewDownloader("Outpost/" + version + " (+" + *site + ")")
	catURL := strings.TrimRight(*site, "/") + "/catalog/v1/catalog.json"
	prov, err := catalog.NewProvider(key, catalog.Embedded, catalog.EmbeddedSig, dirs["catalog"], catURL, dl.FetchSmall)
	if err != nil {
		logger.Printf("%v", err)
		return 1
	}
	var claimed bool
	store.View(func(s *state.State) { claimed = s.AdminHash != "" })
	code, err := ensureSetupCode(p.data, claimed)
	if err != nil {
		logger.Printf("setup code: %v", err)
		return 1
	}
	if code != "" {
		logger.Printf("this node is not claimed yet; open the dashboard and enter the setup code (run: sudo " + cliPath + " setup-code)")
	}

	kw := &kiwix.Manager{
		ServeBin: filepath.Join(p.bin, "kiwix-serve"), ManageBin: filepath.Join(p.bin, "kiwix-manage"),
		ZimDir: dirs["zim"], Library: filepath.Join(dirs["kiwix"], "library.xml"), Port: *kiwixPort, Logger: logger,
	}
	aic := ai.New(*ollama)
	jm := &jobs.Manager{
		Store: store, Catalog: prov.Current, DL: dl, Kiwix: kw, AI: aic, Logger: logger,
		Paths: jobs.Paths{Zim: dirs["zim"], Maps: dirs["maps"], MapAssets: dirs["assets"], Downloads: dirs["downloads"], PMTiles: filepath.Join(p.bin, "pmtiles")},
	}
	srv := &server.Server{
		Version: version, Store: store, Catalog: prov, Jobs: jm, Kiwix: kw, AI: aic, Logger: logger,
		DataDir: p.data, MapsDir: dirs["maps"], AssetsDir: dirs["assets"], LibraryPort: portOf(*libListen),
		SiteURL: strings.TrimRight(*site, "/"),
		SetupCode: func() string {
			b, _ := os.ReadFile(setupCodePath(p.data))
			return strings.TrimSpace(string(b))
		},
		ClearSetupCode: func() { os.Remove(setupCodePath(p.data)) },
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go kw.Run(ctx)
	go jm.Run(ctx)
	go srv.RunPDFIndex(ctx)
	go catalogRefresher(ctx, store, prov, logger)
	logger.Printf("Outpost %s: dashboard on %s, library on %s, data in %s", version, *listen, *libListen, p.data)
	if err := srv.Serve(ctx, *listen, *libListen); err != nil {
		logger.Printf("server: %v", err)
		return 1
	}
	return 0
}

func catalogRefresher(ctx context.Context, store *state.Store, prov *catalog.Provider, logger *log.Logger) {
	delay := 30 * time.Second
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = 12 * time.Hour
		var allowed bool
		store.View(func(s *state.State) { allowed = s.Settings.CheckForUpdates })
		if !allowed {
			continue
		}
		rctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		changed, err := prov.Refresh(rctx)
		cancel()
		if err != nil {
			delay = time.Hour // offline is normal; try again later
			continue
		}
		if changed {
			logger.Printf("catalog updated to sequence %d", prov.Current().Sequence)
		}
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func portOf(addr string) int {
	i := strings.LastIndexByte(addr, ':')
	if i < 0 {
		return 0
	}
	return atoi(addr[i+1:])
}

func printSetupCode(args []string) int {
	fs := flag.NewFlagSet("setup-code", flag.ExitOnError)
	p := commonFlags(fs)
	fs.Parse(args)
	b, err := os.ReadFile(setupCodePath(p.data))
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "This node has already been claimed (or has not started yet).")
		fmt.Fprintln(os.Stderr, "To start over with a new administrator, run: sudo "+cliPath+" reset-admin")
		return 1
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read the setup code: %v (try with sudo)\n", err)
		return 1
	}
	fmt.Println(strings.TrimSpace(string(b)))
	return 0
}

// launchdLabel names the macOS job. It must match LABEL in install.sh.
const launchdLabel = "org.outpost-offline.node"

// cliPath is the command the installer puts in a folder only root can change
// (CLI_DIR in install.sh). Messages name it in full: a short "outpost" is
// looked up along the PATH, and on some machines the folders early in the
// PATH can be written without a password.
const cliPath = "/opt/outpost/cli/outpost"

// systemService reports how to stop and start the installed service, when
// this command runs as root on a machine where that service is running.
// Both are nil otherwise: there is then nothing to pause.
func systemService() (stop func() error, start func()) {
	if os.Geteuid() != 0 {
		return nil, nil
	}
	switch runtime.GOOS {
	case "linux":
		if exec.Command("systemctl", "is-active", "--quiet", "outpost").Run() != nil {
			return nil, nil
		}
		stop = func() error {
			if out, err := exec.Command("systemctl", "stop", "outpost").CombinedOutput(); err != nil {
				return fmt.Errorf("%v %s", err, out)
			}
			return nil
		}
		return stop, func() { exec.Command("systemctl", "start", "outpost").Run() }
	case "darwin":
		target := "system/" + launchdLabel
		loaded := func() bool { return exec.Command("launchctl", "print", target).Run() == nil }
		if !loaded() {
			return nil, nil
		}
		stop = func() error {
			exec.Command("launchctl", "bootout", target).Run()
			// launchd lets go of the job a moment after it is told to.
			for i := 0; i < 40 && loaded(); i++ {
				time.Sleep(250 * time.Millisecond)
			}
			if loaded() {
				return errors.New("launchd did not stop the job")
			}
			return nil
		}
		return stop, func() {
			exec.Command("launchctl", "bootstrap", "system", "/Library/LaunchDaemons/"+launchdLabel+".plist").Run()
		}
	}
	return nil, nil
}

func resetAdmin(args []string) int {
	fs := flag.NewFlagSet("reset-admin", flag.ExitOnError)
	p := commonFlags(fs)
	fs.Parse(args)
	stop, start := systemService()
	if stop != nil {
		if err := stop(); err != nil {
			fmt.Fprintf(os.Stderr, "could not stop the service: %v\n", err)
			return 1
		}
	}
	statePath := filepath.Join(p.data, "state.json")
	st, err := state.Open(statePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open %s: %v\n", statePath, err)
		return 1
	}
	if err := st.Update(func(s *state.State) error {
		s.AdminHash = ""
		s.Sessions = map[string]state.Session{}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "cannot write the state: %v\n", err)
		return 1
	}
	os.Remove(setupCodePath(p.data))
	code, err := ensureSetupCode(p.data, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot write the setup code: %v\n", err)
		return 1
	}
	// Keep files owned by the service account.
	if info, err := os.Stat(p.data); err == nil {
		if sys, ok := info.Sys().(*syscall.Stat_t); ok && os.Geteuid() == 0 {
			os.Chown(statePath, int(sys.Uid), int(sys.Gid))
			os.Chown(setupCodePath(p.data), int(sys.Uid), int(sys.Gid))
		}
	}
	if start != nil {
		start()
	}
	fmt.Println("The administrator password was removed. Your content is untouched.")
	fmt.Println("Open the dashboard and claim the node again with this setup code:")
	fmt.Println()
	fmt.Println("    " + code)
	return 0
}
