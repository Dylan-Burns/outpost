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

// Limiter is a fixed-window failure counter per key (client address) plus a
// global ceiling, to slow down password and setup-code guessing.
type Limiter struct {
	mu     sync.Mutex
	window time.Duration
	perKey int
	global int
	start  time.Time
	counts map[string]int
	total  int
	now    func() time.Time
}

// NewLimiter allows perKey failures per key and global failures overall in
// each window.
func NewLimiter(window time.Duration, perKey, global int) *Limiter {
	return &Limiter{window: window, perKey: perKey, global: global, counts: map[string]int{}, now: time.Now}
}

func (l *Limiter) roll() {
	if now := l.now(); now.Sub(l.start) >= l.window {
		l.start = now
		l.counts = map[string]int{}
		l.total = 0
	}
}

// Allowed reports whether key may try again now.
func (l *Limiter) Allowed(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.roll()
	return l.counts[key] < l.perKey && l.total < l.global
}

// Fail records a failed attempt.
func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.roll()
	l.counts[key]++
	l.total++
}

// Reset clears a key after a success.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.counts, key)
}
