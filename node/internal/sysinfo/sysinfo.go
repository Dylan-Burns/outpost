// Package sysinfo reports the host's hardware so the wizard can warn about
// disk space and suggest AI models that fit in memory.
package sysinfo

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Info describes the host.
type Info struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	CPU      string `json:"cpu"`
	Cores    int    `json:"cores"`
	MemBytes uint64 `json:"memBytes"`
	GPU      string `json:"gpu,omitempty"`
	Hostname string `json:"hostname"`
	Uptime   int64  `json:"uptimeSeconds"`
}

var started = time.Now()

// Collect gathers what it can; missing facts are left empty.
func Collect() Info {
	in := Info{OS: runtime.GOOS, Arch: runtime.GOARCH, Cores: runtime.NumCPU(), Uptime: int64(time.Since(started).Seconds())}
	in.Hostname, _ = os.Hostname()
	switch runtime.GOOS {
	case "linux":
		if f, err := os.Open("/proc/meminfo"); err == nil {
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				if strings.HasPrefix(sc.Text(), "MemTotal:") {
					fs := strings.Fields(sc.Text())
					if len(fs) >= 2 {
						kb, _ := strconv.ParseUint(fs[1], 10, 64)
						in.MemBytes = kb * 1024
					}
				}
			}
			f.Close()
		}
		if f, err := os.Open("/proc/cpuinfo"); err == nil {
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				if k, v, ok := strings.Cut(sc.Text(), ":"); ok && strings.TrimSpace(k) == "model name" {
					in.CPU = strings.TrimSpace(v)
					break
				}
			}
			f.Close()
		}
		in.GPU = linuxGPU()
	case "darwin":
		if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
			in.MemBytes, _ = strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
		}
		if out, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
			in.CPU = strings.TrimSpace(string(out))
		}
	}
	return in
}

func linuxGPU() string {
	if out, err := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader").Output(); err == nil {
		if s := strings.TrimSpace(strings.Split(string(out), "\n")[0]); s != "" {
			return s
		}
	}
	out, err := exec.Command("lspci").Output()
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(string(out), "\n") {
		if strings.Contains(l, "VGA compatible controller") || strings.Contains(l, "3D controller") || strings.Contains(l, "Display controller") {
			if _, v, ok := strings.Cut(l, ": "); ok {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}
