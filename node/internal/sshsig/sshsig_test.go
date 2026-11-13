package sshsig

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestAgainstSSHKeygen signs with the real ssh-keygen and verifies here.
func TestAgainstSSHKeygen(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not installed")
	}
	dir := t.TempDir()
	key := filepath.Join(dir, "k")
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v %s", err, out)
	}
	msg := filepath.Join(dir, "msg")
	if err := os.WriteFile(msg, []byte("hello catalog\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("ssh-keygen", "-Y", "sign", "-f", key, "-n", "outpost-test", msg).CombinedOutput(); err != nil {
		t.Fatalf("sign: %v %s", err, out)
	}
	pubLine, _ := os.ReadFile(key + ".pub")
	pub, err := ParsePublicKey(string(pubLine))
	if err != nil {
		t.Fatal(err)
	}
	sig, _ := os.ReadFile(msg + ".sig")
	data, _ := os.ReadFile(msg)
	if err := Verify(pub, "outpost-test", data, sig); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := Verify(pub, "other-namespace", data, sig); err == nil {
		t.Fatal("wrong namespace accepted")
	}
	if err := Verify(pub, "outpost-test", []byte("hello catalog!\n"), sig); err == nil {
		t.Fatal("tampered message accepted")
	}
	// A different key must be rejected.
	key2 := filepath.Join(dir, "k2")
	exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key2).Run()
	pub2Line, _ := os.ReadFile(key2 + ".pub")
	pub2, _ := ParsePublicKey(string(pub2Line))
	if err := Verify(pub2, "outpost-test", data, sig); err == nil {
		t.Fatal("signature accepted under a different key")
	}
	// Garbage must not panic.
	for _, bad := range []string{"", "-----BEGIN SSH SIGNATURE-----\nAAAA\n-----END SSH SIGNATURE-----", "-----BEGIN SSH SIGNATURE-----\n!!!\n-----END SSH SIGNATURE-----"} {
		if err := Verify(pub, "outpost-test", data, []byte(bad)); err == nil {
			t.Fatalf("garbage accepted: %q", bad)
		}
	}
}
