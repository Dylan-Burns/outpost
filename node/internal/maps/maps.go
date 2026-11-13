// Package maps installs offline map data: regions cut from the Protomaps
// planet build with the pmtiles tool, a low-zoom world overview, and the
// fonts and icons the map style needs.
package maps

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Extract cuts a region (geometry file, or the whole world when geometry is
// empty) up to maxzoom from the planet build into out. progress receives the
// bytes written so far.
func Extract(ctx context.Context, pmtilesBin, build, geometry string, maxzoom int, out string, progress func(int64)) error {
	if _, err := os.Stat(pmtilesBin); err != nil {
		return errors.New("the pmtiles tool is not installed")
	}
	args := []string{"extract", build, out, "--maxzoom=" + strconv.Itoa(maxzoom), "--download-threads=4"}
	if geometry != "" {
		args = append(args, "--region="+geometry)
	}
	os.Remove(out)
	cmd := exec.CommandContext(ctx, pmtilesBin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &limitedWriter{w: &stderr, n: 64 << 10}
	cmd.Stdout = io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case err := <-done:
			if err != nil {
				os.Remove(out)
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return fmt.Errorf("pmtiles extract failed: %v: %s", err, lastLine(stderr.String()))
			}
			if st, err := os.Stat(out); err != nil || st.Size() < 127 {
				return errors.New("pmtiles extract produced no archive")
			}
			return nil
		case <-tick.C:
			if st, err := os.Stat(out); err == nil && progress != nil {
				progress(st.Size())
			}
		}
	}
}

type limitedWriter struct {
	w io.Writer
	n int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.n <= 0 {
		return len(p), nil
	}
	k := len(p)
	if k > l.n {
		k = l.n
	}
	l.n -= k
	l.w.Write(p[:k])
	return len(p), nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}

// Asset limits: the style assets are fonts (protobuf glyph ranges) and
// sprite sheets, nothing else.
const (
	maxAssetFile  = 8 << 20
	maxAssetTotal = 256 << 20
	maxAssetFiles = 20000
)

// InstallAssets unpacks a .tar.gz of map style assets into dest, accepting
// only regular files under fonts/ and sprites/ with safe names. It unpacks
// into a fresh directory and swaps it in, so a failure leaves the old set.
func InstallAssets(tarGz, dest string) error {
	f, err := os.Open(tarGz)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	staging := dest + ".new"
	os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(staging)
		}
	}()
	tr := tar.NewReader(gz)
	var total int64
	files := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := path.Clean(strings.TrimPrefix(hdr.Name, "./"))
		switch hdr.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
		default:
			continue // no links, devices or other surprises
		}
		if !safeAssetPath(name) {
			return fmt.Errorf("unexpected file in map assets: %q", hdr.Name)
		}
		if hdr.Size > maxAssetFile {
			return fmt.Errorf("map asset too large: %q", name)
		}
		total += hdr.Size
		files++
		if total > maxAssetTotal || files > maxAssetFiles {
			return errors.New("map assets archive is too large")
		}
		target := filepath.Join(staging, filepath.FromSlash(name))
		if !strings.HasPrefix(target, staging+string(filepath.Separator)) {
			return fmt.Errorf("unexpected path %q", name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, io.LimitReader(tr, hdr.Size)); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
	if files == 0 {
		return errors.New("map assets archive is empty")
	}
	old := dest + ".old"
	os.RemoveAll(old)
	if _, err := os.Stat(dest); err == nil {
		if err := os.Rename(dest, old); err != nil {
			return err
		}
	}
	if err := os.Rename(staging, dest); err != nil {
		os.Rename(old, dest)
		return err
	}
	os.RemoveAll(old)
	ok = true
	return nil
}

func safeAssetPath(p string) bool {
	if p == "." || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "../") || strings.Contains(p, "/../") {
		return false
	}
	if !strings.HasPrefix(p, "fonts/") && !strings.HasPrefix(p, "sprites/") {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." || strings.HasPrefix(seg, ".") {
			return false
		}
		for _, r := range seg {
			ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' || r == ' ' || r == '@'
			if !ok {
				return false
			}
		}
	}
	ext := path.Ext(p)
	return ext == ".pbf" || ext == ".png" || ext == ".json"
}
