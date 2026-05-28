//go:build darwin

package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"testing"

	"golang.org/x/crypto/pbkdf2"
)

// Encrypt a payload using the v10 scheme so the decrypt round-trips.
func encryptV10(t *testing.T, key, pt []byte) []byte {
	t.Helper()
	pt = pkcs7Pad(pt, aes.BlockSize)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	iv := bytes16Spaces()
	ct := make([]byte, len(pt))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, pt)
	return append([]byte("v10"), ct...)
}

func pkcs7Pad(b []byte, sz int) []byte {
	n := sz - len(b)%sz
	pad := make([]byte, n)
	for i := range pad {
		pad[i] = byte(n)
	}
	return append(b, pad...)
}

func TestDeriveChromeAESKey(t *testing.T) {
	// Known vector — Chrome's hard-coded salt + iterations.
	pw := []byte("peanuts")
	key := deriveChromeAESKey(pw)
	if len(key) != 16 {
		t.Fatalf("key len = %d want 16", len(key))
	}
	// Re-derive to confirm determinism.
	key2 := pbkdf2.Key(pw, []byte("saltysalt"), 1003, 16, sha1.New)
	for i := range key {
		if key[i] != key2[i] {
			t.Fatalf("non-determinism at byte %d", i)
		}
	}
}

func TestDecryptChromeCookieV10_RoundTrip(t *testing.T) {
	key := deriveChromeAESKey([]byte("test-pw"))
	want := "xoxd-this-is-a-fake-test-value-12345"
	enc := encryptV10(t, key, []byte(want))
	got, err := decryptChromeCookieV10(enc, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDecryptChromeCookieV10_NotV10(t *testing.T) {
	key := deriveChromeAESKey([]byte("test-pw"))
	_, err := decryptChromeCookieV10([]byte("v20whatever"), key)
	if err == nil {
		t.Fatal("expected error for non-v10 cookie")
	}
}

func TestCandidatePlaintexts_StripsHostHashPrefix(t *testing.T) {
	// Chrome 118+ prepends 32-byte SHA-256(host) to plaintext.
	hostPrefix := make([]byte, 32)
	for i := range hostPrefix {
		hostPrefix[i] = byte(i)
	}
	body := []byte("xoxd-real-token-payload")
	full := append(append([]byte(nil), hostPrefix...), body...)

	cands := candidatePlaintexts(full)
	if len(cands) != 2 {
		t.Fatalf("expected 2 candidates (stripped + full), got %d", len(cands))
	}
	if string(cands[0]) != string(body) {
		t.Errorf("first candidate should be stripped body; got %q", cands[0])
	}
}

func TestCandidatePlaintexts_ShortPlaintextNoStrip(t *testing.T) {
	pt := []byte("xoxd-short")
	cands := candidatePlaintexts(pt)
	if len(cands) != 1 || string(cands[0]) != "xoxd-short" {
		t.Fatalf("short plaintext should return single candidate: %v", cands)
	}
}
