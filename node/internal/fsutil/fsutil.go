// Package fsutil holds small filesystem helpers shared by the node.
package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// WriteFileAtomic writes data to path via a temporary file, fsync and rename,
// so a power cut leaves either the old or the new content, never a mix.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	cleanup := func() { os.Remove(tmp) }
	if _, err := f.Write(data); err != nil {
		f.Close()
		cleanup()
		return err
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		cleanup()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		cleanup()
		return err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		cleanup()
		return err
	}
	SyncDir(dir)
	return nil
}

// SyncDir fsyncs a directory so a rename inside it is durable. Errors are
// ignored: some filesystems do not support syncing directories.
func SyncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		d.Sync()
		d.Close()
	}
}

// DiskSpace reports the bytes available to unprivileged users and the total
// size of the filesystem holding path.
func DiskSpace(path string) (free, total uint64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	bs := uint64(st.Bsize)
	return uint64(st.Bavail) * bs, uint64(st.Blocks) * bs, nil
}

// SafeName reports whether name is a plain file name (no separators, no
// traversal, no hidden-file tricks) built from a conservative alphabet.
func SafeName(name string) bool {
	if name == "" || len(name) > 200 || name == "." || name == ".." || strings.HasPrefix(name, ".") {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return false
		}
	}
	return true
}

// Within joins base and name and guarantees the result stays inside base.
func Within(base, name string) (string, error) {
	if !SafeName(name) {
		return "", errors.New("unsafe file name")
	}
	p := filepath.Join(base, name)
	if filepath.Dir(p) != filepath.Clean(base) {
		return "", errors.New("path escapes its directory")
	}
	return p, nil
}
