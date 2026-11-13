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

// Throttle is an account-wide failure budget. Past the budget, every
// attempt waits its turn behind one shared lock and a fixed delay, so a
// distributed guesser gets roughly one try per delay in total, while the
// owner can still sign in (slowly) instead of being locked out.
type Throttle struct {
	mu       sync.Mutex
	gate     sync.Mutex
	window   time.Duration
	budget   int
	delay    time.Duration
	start    time.Time
	failures int
	now      func() time.Time
	sleep    func(time.Duration)
}

// NewThrottle allows budget failures per window before throttling.
func NewThrottle(window time.Duration, budget int, delay time.Duration) *Throttle {
	return &Throttle{window: window, budget: budget, delay: delay, now: time.Now, sleep: time.Sleep}
}

// Fail records a failed attempt.
func (t *Throttle) Fail() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rollLocked()
	t.failures++
}

func (t *Throttle) rollLocked() {
	if now := t.now(); now.Sub(t.start) >= t.window {
		t.start = now
		t.failures = 0
	}
}

// Over reports whether the budget is spent.
func (t *Throttle) Over() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rollLocked()
	return t.failures >= t.budget
}

// Do runs fn, first queueing behind other throttled attempts and waiting
// the delay when the budget is spent.
func (t *Throttle) Do(fn func()) {
	if !t.Over() {
		fn()
		return
	}
	t.gate.Lock()
	defer t.gate.Unlock()
	t.sleep(t.delay)
	fn()
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
