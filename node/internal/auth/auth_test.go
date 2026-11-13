package auth

import (
	"strings"
	"testing"
	"time"
)

func TestPasswordHash(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("short password accepted")
	}
	h, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$") {
		t.Fatal(h)
	}
	if !VerifyPassword(h, "correct horse battery") {
		t.Fatal("right password rejected")
	}
	if VerifyPassword(h, "correct horse batterY") {
		t.Fatal("wrong password accepted")
	}
	for _, bad := range []string{"", "$argon2id$v=19$m=0,t=1,p=1$AA$AA", "$bcrypt$x", h + "$extra"} {
		if VerifyPassword(bad, "correct horse battery") {
			t.Fatalf("malformed hash accepted: %q", bad)
		}
	}
}

func TestSetupCode(t *testing.T) {
	c, err := NewSetupCode()
	if err != nil {
		t.Fatal(err)
	}
	if len(c) != 14 || strings.Count(c, "-") != 2 {
		t.Fatalf("code %q", c)
	}
	if !SetupCodeMatches(c, strings.ToLower(strings.ReplaceAll(c, "-", " "))) {
		t.Fatal("normalized code rejected")
	}
	if SetupCodeMatches(c, "AAAA-AAAA-AAAA") || SetupCodeMatches("", "") {
		t.Fatal("wrong code accepted")
	}
}

func TestLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewLimiter(time.Minute, 2)
	l.now = func() time.Time { return now }
	if !l.Begin("a") || !l.Begin("a") {
		t.Fatal("first attempts refused")
	}
	if l.Begin("a") {
		t.Fatal("a should be limited")
	}
	if !l.Begin("b") {
		t.Fatal("another client must not be locked out")
	}
	l.Reset("a")
	if !l.Begin("a") {
		t.Fatal("reset did not clear")
	}
	now = now.Add(time.Minute)
	l.Begin("b")
	if !l.Begin("b") {
		t.Fatal("window did not roll")
	}
}

func TestWithHashSlot(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 4)
	for i := 0; i < 4; i++ {
		go WithHashSlot(func() { started <- struct{}{}; <-release })
	}
	for i := 0; i < 4; i++ {
		<-started
	}
	if err := WithHashSlot(func() {}); err != ErrBusy {
		t.Fatal("fifth concurrent hash admitted")
	}
	close(release)
}
