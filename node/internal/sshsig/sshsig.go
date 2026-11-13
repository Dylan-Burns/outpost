// Package sshsig verifies OpenSSH "SSHSIG" detached signatures made with
// ed25519 keys (ssh-keygen -Y sign). Release artifacts and the content catalog
// are signed this way, so the installer can check them with ssh-keygen and the
// node can check them without any extra tooling.
package sshsig

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

const magic = "SSHSIG"

// ParsePublicKey parses an authorized_keys style line: "ssh-ed25519 AAAA... comment".
func ParsePublicKey(line string) (ed25519.PublicKey, error) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 2 || fields[0] != "ssh-ed25519" {
		return nil, errors.New("sshsig: only ssh-ed25519 public keys are supported")
	}
	blob, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return nil, fmt.Errorf("sshsig: bad key encoding: %w", err)
	}
	return parseKeyBlob(blob)
}

func parseKeyBlob(blob []byte) (ed25519.PublicKey, error) {
	r := reader{b: blob}
	algo, err := r.str()
	if err != nil {
		return nil, err
	}
	if string(algo) != "ssh-ed25519" {
		return nil, errors.New("sshsig: key is not ssh-ed25519")
	}
	k, err := r.str()
	if err != nil {
		return nil, err
	}
	if len(k) != ed25519.PublicKeySize || len(r.b) != 0 {
		return nil, errors.New("sshsig: malformed ed25519 key")
	}
	return ed25519.PublicKey(bytes.Clone(k)), nil
}

// Verify checks an armored SSHSIG signature over message for the given
// namespace, made by key. It returns nil only if the signature is valid.
func Verify(key ed25519.PublicKey, namespace string, message, armored []byte) error {
	blob, err := unarmor(armored)
	if err != nil {
		return err
	}
	r := reader{b: blob}
	if len(r.b) < len(magic) || string(r.b[:len(magic)]) != magic {
		return errors.New("sshsig: missing SSHSIG magic")
	}
	r.b = r.b[len(magic):]
	ver, err := r.u32()
	if err != nil || ver != 1 {
		return errors.New("sshsig: unsupported version")
	}
	pkBlob, err := r.str()
	if err != nil {
		return err
	}
	ns, err := r.str()
	if err != nil {
		return err
	}
	reserved, err := r.str()
	if err != nil {
		return err
	}
	hashAlg, err := r.str()
	if err != nil {
		return err
	}
	sigBlob, err := r.str()
	if err != nil {
		return err
	}
	if len(r.b) != 0 {
		return errors.New("sshsig: trailing data")
	}
	pk, err := parseKeyBlob(pkBlob)
	if err != nil {
		return err
	}
	if !pk.Equal(key) {
		return errors.New("sshsig: signed by an unexpected key")
	}
	if string(ns) != namespace {
		return fmt.Errorf("sshsig: namespace %q, want %q", ns, namespace)
	}
	var digest []byte
	switch string(hashAlg) {
	case "sha512":
		d := sha512.Sum512(message)
		digest = d[:]
	case "sha256":
		d := sha256.Sum256(message)
		digest = d[:]
	default:
		return errors.New("sshsig: unsupported hash algorithm")
	}
	sr := reader{b: sigBlob}
	sigAlg, err := sr.str()
	if err != nil {
		return err
	}
	if string(sigAlg) != "ssh-ed25519" {
		return errors.New("sshsig: signature is not ssh-ed25519")
	}
	sig, err := sr.str()
	if err != nil || len(sig) != ed25519.SignatureSize || len(sr.b) != 0 {
		return errors.New("sshsig: malformed signature")
	}
	var signed bytes.Buffer
	signed.WriteString(magic)
	writeStr(&signed, ns)
	writeStr(&signed, reserved)
	writeStr(&signed, hashAlg)
	writeStr(&signed, digest)
	if !ed25519.Verify(key, signed.Bytes(), sig) {
		return errors.New("sshsig: signature does not verify")
	}
	return nil
}

func unarmor(armored []byte) ([]byte, error) {
	s := strings.TrimSpace(string(armored))
	const begin, end = "-----BEGIN SSH SIGNATURE-----", "-----END SSH SIGNATURE-----"
	if !strings.HasPrefix(s, begin) || !strings.HasSuffix(s, end) {
		return nil, errors.New("sshsig: not an armored SSH signature")
	}
	body := strings.Join(strings.Fields(s[len(begin):len(s)-len(end)]), "")
	return base64.StdEncoding.DecodeString(body)
}

type reader struct{ b []byte }

func (r *reader) u32() (uint32, error) {
	if len(r.b) < 4 {
		return 0, errors.New("sshsig: truncated")
	}
	v := binary.BigEndian.Uint32(r.b)
	r.b = r.b[4:]
	return v, nil
}

func (r *reader) str() ([]byte, error) {
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	if uint64(n) > uint64(len(r.b)) {
		return nil, errors.New("sshsig: truncated")
	}
	s := r.b[:n]
	r.b = r.b[n:]
	return s, nil
}

func writeStr(w *bytes.Buffer, s []byte) {
	var l [4]byte
	binary.BigEndian.PutUint32(l[:], uint32(len(s)))
	w.Write(l[:])
	w.Write(s)
}
