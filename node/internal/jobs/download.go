package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// ErrChecksum means the bytes downloaded do not match the catalog's SHA-256.
var ErrChecksum = errors.New("checksum mismatch: the downloaded file is not the one the catalog names")

// errPermanent marks errors a retry cannot fix.
type errPermanent struct{ error }

func (e errPermanent) Unwrap() error { return e.error }

// IsPermanent reports whether err should not be retried.
func IsPermanent(err error) bool {
	var p errPermanent
	return errors.As(err, &p) || errors.Is(err, ErrChecksum) || errors.Is(err, context.Canceled)
}

// Downloader fetches files over HTTPS with resume and integrity checks.
type Downloader struct {
	Client      *http.Client
	UserAgent   string
	IdleTimeout time.Duration // abort a transfer that stalls this long
	// AllowHTTP permits plain http URLs (tests only).
	AllowHTTP bool
}

// NewDownloader returns a downloader whose redirects must stay on https.
func NewDownloader(userAgent string) *Downloader {
	d := &Downloader{UserAgent: userAgent, IdleTimeout: 60 * time.Second}
	d.Client = &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			TLSHandshakeTimeout:   20 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			MaxIdleConnsPerHost:   4,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "https" && !d.AllowHTTP {
				return errPermanent{fmt.Errorf("refusing redirect to non-https URL %s", req.URL.Redacted())}
			}
			return nil
		},
	}
	return d
}

// Progress receives the bytes on disk and the expected total (0 if unknown).
type Progress func(done, total int64)

// Fetch downloads url into partPath, resuming from whatever partPath holds,
// and checks the SHA-256 of the whole file when wantSHA is not empty. On a
// checksum mismatch partPath is removed so the next attempt starts over.
func (d *Downloader) Fetch(ctx context.Context, url, partPath, wantSHA string, size int64, progress Progress) error {
	if !strings.HasPrefix(url, "https://") && !(d.AllowHTTP && strings.HasPrefix(url, "http://")) {
		return errPermanent{fmt.Errorf("refusing non-https URL")}
	}
	f, err := os.OpenFile(partPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return errPermanent{err}
	}
	defer f.Close()

	// Hash whatever is already on disk so a resumed file is still verified
	// end to end.
	h := sha256.New()
	have, err := io.Copy(h, f)
	if err != nil {
		return err
	}
	if size > 0 && have > size {
		// Longer than the catalog says: start over.
		if err := f.Truncate(0); err != nil {
			return err
		}
		f.Seek(0, io.SeekStart)
		h.Reset()
		have = 0
	}
	if size <= 0 || have < size {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return errPermanent{err}
		}
		if d.UserAgent != "" {
			req.Header.Set("User-Agent", d.UserAgent)
		}
		if have > 0 {
			req.Header.Set("Range", "bytes="+strconv.FormatInt(have, 10)+"-")
		}
		resp, err := d.Client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusPartialContent && have > 0:
			if !rangeStartsAt(resp.Header.Get("Content-Range"), have) {
				return fmt.Errorf("server sent an unexpected range %q", resp.Header.Get("Content-Range"))
			}
		case resp.StatusCode == http.StatusOK:
			if have > 0 { // server ignored the range: start over
				if err := f.Truncate(0); err != nil {
					return err
				}
				h.Reset()
				have = 0
			}
		case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && have > 0:
			// The file may already be complete; fall through to verification.
		case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusForbidden:
			return errPermanent{fmt.Errorf("the server answered %s", resp.Status)}
		default:
			return fmt.Errorf("the server answered %s", resp.Status)
		}
		if resp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
			if _, err := f.Seek(have, io.SeekStart); err != nil {
				return err
			}
			total := size
			if total <= 0 && resp.ContentLength > 0 {
				total = have + resp.ContentLength
			}
			body := &idleReader{r: resp.Body, timeout: d.IdleTimeout, cancel: func() { resp.Body.Close() }}
			buf := make([]byte, 256*1024)
			last := time.Now()
			for {
				n, rerr := body.Read(buf)
				if n > 0 {
					if _, werr := f.Write(buf[:n]); werr != nil {
						return errPermanent{fmt.Errorf("writing to disk: %w", werr)}
					}
					h.Write(buf[:n])
					have += int64(n)
					if size > 0 && have > size {
						f.Truncate(0)
						return errPermanent{errors.New("the server sent more data than the catalog lists")}
					}
					if progress != nil && time.Since(last) > 250*time.Millisecond {
						progress(have, total)
						last = time.Now()
					}
				}
				if rerr == io.EOF {
					break
				}
				if rerr != nil {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					return rerr
				}
			}
			if progress != nil {
				progress(have, total)
			}
		}
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if size > 0 && have != size {
		return fmt.Errorf("download ended early (%d of %d bytes)", have, size)
	}
	if wantSHA != "" {
		got := hex.EncodeToString(h.Sum(nil))
		if !strings.EqualFold(got, wantSHA) {
			f.Close()
			os.Remove(partPath)
			return ErrChecksum
		}
	}
	return nil
}

func rangeStartsAt(cr string, off int64) bool {
	// "bytes 100-199/200"
	cr = strings.TrimSpace(cr)
	if !strings.HasPrefix(cr, "bytes ") {
		return false
	}
	rest := cr[len("bytes "):]
	dash := strings.IndexByte(rest, '-')
	if dash < 0 {
		return false
	}
	start, err := strconv.ParseInt(rest[:dash], 10, 64)
	return err == nil && start == off
}

// idleReader aborts a body read that makes no progress for timeout.
type idleReader struct {
	r       io.Reader
	timeout time.Duration
	cancel  func()
}

func (ir *idleReader) Read(p []byte) (int, error) {
	if ir.timeout <= 0 {
		return ir.r.Read(p)
	}
	t := time.AfterFunc(ir.timeout, ir.cancel)
	n, err := ir.r.Read(p)
	if !t.Stop() && err != nil {
		return n, fmt.Errorf("the transfer stalled for %v", ir.timeout)
	}
	return n, err
}

// FetchSmall downloads a small file (at most limit bytes) into memory.
func (d *Downloader) FetchSmall(ctx context.Context, url string, limit int64) ([]byte, error) {
	if !strings.HasPrefix(url, "https://") && !(d.AllowHTTP && strings.HasPrefix(url, "http://")) {
		return nil, errPermanent{fmt.Errorf("refusing non-https URL")}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if d.UserAgent != "" {
		req.Header.Set("User-Agent", d.UserAgent)
	}
	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusForbidden:
		return nil, errPermanent{fmt.Errorf("the server answered %s", resp.Status)}
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("the server answered %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errors.New("response too large")
	}
	return b, nil
}
