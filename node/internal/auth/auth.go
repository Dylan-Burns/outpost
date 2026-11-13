// Package auth handles the single admin account: the one-time setup code
// that proves the person claiming a fresh node has shell access to it, the
// argon2id password hash, browser sessions, and login rate limiting.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters: the OWASP minimum profile, light enough for a
// low-end mini PC (19 MiB, two passes).
const (
	argonTime    = 2
	argonMemory  = 19 * 1024
	argonThreads = 1
	argonKeyLen  = 32
	saltLen      = 16
)

// MinPasswordLen is the shortest admin password accepted.
const MinPasswordLen = 10

// HashPassword returns an encoded argon2id hash.
func HashPassword(pw string) (string, error) {
	if err := CheckPasswordPolicy(pw); err != nil {
		return "", err
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// CheckPasswordPolicy rejects passwords that are too short or too long.
func CheckPasswordPolicy(pw string) error {
	if len([]rune(pw)) < MinPasswordLen {
		return fmt.Errorf("the password must have at least %d characters", MinPasswordLen)
	}
	if len(pw) > 256 {
		return errors.New("the password is too long")
	}
	return nil
}

// VerifyPassword checks pw against an encoded hash in constant time.
func VerifyPassword(encoded, pw string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var m uint32
	var t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	if m == 0 || m > 1<<20 || t == 0 || t > 10 || p == 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 || len(want) > 64 {
		return false
	}
	if len(pw) > 256 {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// setupAlphabet leaves out look-alike characters (0/O, 1/I/L).
const setupAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

// NewSetupCode returns a code like "K7QF-M2XP-9HTD" (about 59 bits).
func NewSetupCode() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	var sb strings.Builder
	for i, c := range b {
		if i > 0 && i%4 == 0 {
			sb.WriteByte('-')
		}
		sb.WriteByte(setupAlphabet[int(c)%len(setupAlphabet)])
	}
	return sb.String(), nil
}

// NormalizeSetupCode upper-cases and strips spaces and dashes.
func NormalizeSetupCode(s string) string {
	s = strings.ToUpper(s)
	return strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s)
}

// SetupCodeMatches compares two codes in constant time after normalizing.
func SetupCodeMatches(want, got string) bool {
	a, b := NormalizeSetupCode(want), NormalizeSetupCode(got)
	if a == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// NewToken returns a random URL-safe token with 256 bits of entropy.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// TokenKey is how a session token is stored: its SHA-256 in hex.
func TokenKey(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// Limiter is a fixed-window attempt counter per key (client network). An
// attempt is counted when it is admitted, before the credential is checked,
// so parallel requests cannot slip past the limit. There is deliberately no
// global ceiling: one noisy client must not lock the owner out.
type Limiter struct {
	mu     sync.Mutex
	window time.Duration
	perKey int
	start  time.Time
	counts map[string]int
	now    func() time.Time
}

// NewLimiter allows perKey attempts per key in each window.
func NewLimiter(window time.Duration, perKey int) *Limiter {
	return &Limiter{window: window, perKey: perKey, counts: map[string]int{}, now: time.Now}
}

func (l *Limiter) roll() {
	if now := l.now(); now.Sub(l.start) >= l.window {
		l.start = now
		l.counts = map[string]int{}
	}
}

// Begin admits and counts an attempt, or reports false when key is over
// its limit.
func (l *Limiter) Begin(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.roll()
	if l.counts[key] >= l.perKey {
		return false
	}
	l.counts[key]++
	return true
}

// Reset clears a key after a successful attempt.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.counts, key)
}

// Throttle is an account-wide failure budget. Past the budget it admits at
// most one attempt per interval across all clients and rejects the rest at
// once (no queue), so a distributed guesser gets about one try per interval.
type Throttle struct {
	mu       sync.Mutex
	window   time.Duration
	budget   int
	interval time.Duration
	start    time.Time
	failures int
	last     time.Time
	now      func() time.Time
}

// NewThrottle allows budget failures per window before throttling to one
// attempt per interval.
func NewThrottle(window time.Duration, budget int, interval time.Duration) *Throttle {
	return &Throttle{window: window, budget: budget, interval: interval, now: time.Now}
}

func (t *Throttle) rollLocked(now time.Time) {
	if now.Sub(t.start) >= t.window {
		t.start = now
		t.failures = 0
	}
}

// Succeeded un-counts an admitted attempt that turned out to be right.
func (t *Throttle) Succeeded() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.failures > 0 {
		t.failures--
	}
}

// Allow admits an attempt and counts it as a failure until Succeeded is
// called, so attempts still in flight count against the budget. Past the
// budget one attempt per interval is admitted; the rest are refused.
// The owner is never starved for good: clients that signed in before skip
// the throttle, and "outpost reset-admin" on the machine always works.
func (t *Throttle) Allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	t.rollLocked(now)
	if t.failures >= t.budget {
		if now.Sub(t.last) < t.interval {
			return false
		}
		t.last = now
	}
	t.failures++
	return true
}

// hashSlots bounds concurrent argon2 computations (19 MiB each).
var hashSlots = make(chan struct{}, 4)

// ErrBusy means too many password checks are already running.
var ErrBusy = errors.New("busy")

// WithHashSlot runs fn while holding one of a few argon2 slots, or returns
// ErrBusy at once when all are taken.
func WithHashSlot(fn func()) error {
	select {
	case hashSlots <- struct{}{}:
		defer func() { <-hashSlots }()
		fn()
		return nil
	default:
		return ErrBusy
	}
}
