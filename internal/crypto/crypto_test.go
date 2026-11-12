package crypto

import (
	"bytes"
	"crypto/rand"
	"io"
	"strings"
	"testing"
	"time"
)

func TestStreamRoundTrip(t *testing.T) {
	key := GenerateKey()
	for _, size := range []int{0, 1, chunkSize - 1, chunkSize, chunkSize + 1, 3*chunkSize + 17} {
		plain := make([]byte, size)
		rand.Read(plain)
		var buf bytes.Buffer
		w, err := NewEncryptWriter(&buf, key)
		if err != nil {
			t.Fatal(err)
		}
		// write in odd sized pieces to exercise buffering
		for p := plain; len(p) > 0; {
			n := min(len(p), 7777)
			if _, err := w.Write(p[:n]); err != nil {
				t.Fatal(err)
			}
			p = p[n:]
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		if size > 64 && bytes.Contains(buf.Bytes(), plain[:32]) {
			t.Fatal("ciphertext contains plaintext")
		}
		r, err := NewDecryptReader(bytes.NewReader(buf.Bytes()), key)
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(r)
		if err != nil || !bytes.Equal(got, plain) {
			t.Fatalf("size %d: round trip failed: %v", size, err)
		}
	}
}

func encrypt(t *testing.T, key, plain []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, _ := NewEncryptWriter(&buf, key)
	w.Write(plain)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func decryptAll(key, ct []byte) ([]byte, error) {
	r, err := NewDecryptReader(bytes.NewReader(ct), key)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}

func TestStreamDetectsTampering(t *testing.T) {
	key := GenerateKey()
	plain := make([]byte, 3*chunkSize)
	ct := encrypt(t, key, plain)

	flipped := bytes.Clone(ct)
	flipped[len(flipped)/2] ^= 1
	if _, err := decryptAll(key, flipped); err == nil {
		t.Error("bit flip not detected")
	}
	// Truncation at a chunk boundary must be detected (no final chunk).
	cut := 15 + 4 + chunkSize + 16
	if _, err := decryptAll(key, ct[:cut]); err == nil {
		t.Error("truncation not detected")
	}
	// Swapping two chunks must be detected.
	one := 4 + chunkSize + 16
	swapped := bytes.Clone(ct[:15])
	swapped = append(swapped, ct[15+one:15+2*one]...)
	swapped = append(swapped, ct[15:15+one]...)
	swapped = append(swapped, ct[15+2*one:]...)
	if _, err := decryptAll(key, swapped); err == nil {
		t.Error("reordering not detected")
	}
	if _, err := decryptAll(GenerateKey(), ct); err == nil {
		t.Error("wrong key accepted")
	}
}

func TestPassphraseContainer(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewPassphraseWriter(&buf, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("secret data"))
	w.Close()
	r, err := NewPassphraseReader(bytes.NewReader(buf.Bytes()), "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := io.ReadAll(r); string(got) != "secret data" {
		t.Fatalf("got %q", got)
	}
	r, err = NewPassphraseReader(bytes.NewReader(buf.Bytes()), "wrong passphrase!!")
	if err == nil {
		if _, err = io.ReadAll(r); err == nil {
			t.Fatal("wrong passphrase accepted")
		}
	}
}

func TestFieldEncryption(t *testing.T) {
	k, err := NewKeys(GenerateKey())
	if err != nil {
		t.Fatal(err)
	}
	enc := k.EncryptField("anna@example.org", "users.email", "user-1")
	if enc == "" || bytes.Contains([]byte(enc), []byte("anna")) {
		t.Fatal("not encrypted")
	}
	if enc == k.EncryptField("anna@example.org", "users.email", "user-1") {
		t.Fatal("encryption must be randomised")
	}
	if got, err := k.DecryptField(enc, "users.email", "user-1"); err != nil || got != "anna@example.org" {
		t.Fatalf("decrypt: %q %v", got, err)
	}
	if _, err := k.DecryptField(enc, "users.phone", "user-1"); err == nil {
		t.Fatal("ciphertext accepted in another column")
	}
	if _, err := k.DecryptField(enc, "users.email", "user-2"); err == nil {
		t.Fatal("ciphertext accepted in another row")
	}
	if k.BlindIndex("Anna@Example.org ") != k.BlindIndex("anna@example.org") {
		t.Fatal("blind index must normalise")
	}
	other, _ := NewKeys(GenerateKey())
	if other.BlindIndex("anna@example.org") == k.BlindIndex("anna@example.org") {
		t.Fatal("blind index must depend on the key")
	}
}

func TestPassword(t *testing.T) {
	h := HashPassword("a long passphrase")
	if !VerifyPassword("a long passphrase", h) {
		t.Fatal("correct password rejected")
	}
	if VerifyPassword("a long passphrasf", h) || VerifyPassword("", h) || VerifyPassword("x", "") || VerifyPassword("x", "$argon2id$garbage") {
		t.Fatal("wrong password accepted")
	}
	if h == HashPassword("a long passphrase") {
		t.Fatal("hash must be salted")
	}
}

func TestBlobKeysAndRecoveryCodes(t *testing.T) {
	k, _ := NewKeys(GenerateKey())
	a, b := k.BlobKey("aaaa"), k.BlobKey("bbbb")
	if len(a) != 32 || bytes.Equal(a, b) || bytes.Equal(a, k.Blob) {
		t.Fatal("every file must get its own key")
	}
	if !bytes.Equal(a, k.BlobKey("aaaa")) {
		t.Fatal("file keys must be reproducible")
	}
	c1, c2 := NewRecoveryCode(), NewRecoveryCode()
	if len(c1) != 19 || c1 == c2 {
		t.Fatalf("recovery codes: %q %q", c1, c2)
	}
	if NormalizeRecoveryCode(" "+strings.ToUpper(c1)+" ") != strings.ReplaceAll(c1, "-", "") {
		t.Fatal("normalisation")
	}
}

func TestStreamRejectsTrailingData(t *testing.T) {
	key := GenerateKey()
	ct := append(encrypt(t, key, []byte("hello")), []byte("extra")...)
	if _, err := decryptAll(key, ct); err == nil {
		t.Fatal("data after the final chunk must be rejected")
	}
}

func TestIDs(t *testing.T) {
	a := NewID()
	time.Sleep(3 * time.Millisecond)
	b := NewID()
	if len(a) != 32 || a >= b {
		t.Fatalf("ids must sort by creation time: %s %s", a, b)
	}
	// Random ids must not carry the time prefix.
	if RandomID()[:8] == a[:8] && RandomID()[:8] == a[:8] {
		t.Fatal("RandomID looks time based")
	}
}

func TestTOTP(t *testing.T) {
	// RFC 6238 test vector (SHA-1, secret "12345678901234567890"), truncated to 6 digits.
	secret := b32.EncodeToString([]byte("12345678901234567890"))
	for ts, want := range map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"} {
		got, err := TOTPCode(secret, time.Unix(ts, 0))
		if err != nil || got != want {
			t.Errorf("t=%d got %s want %s (%v)", ts, got, want, err)
		}
	}
	now := time.Unix(1234567890, 0)
	if _, ok := VerifyTOTP(secret, "005924", now); !ok {
		t.Error("valid code rejected")
	}
	if _, ok := VerifyTOTP(secret, "005924", now.Add(30*time.Second)); !ok {
		t.Error("one step of skew must be accepted")
	}
	if _, ok := VerifyTOTP(secret, "005924", now.Add(90*time.Second)); ok {
		t.Error("stale code accepted")
	}
	if _, ok := VerifyTOTP(secret, "000000", now); ok {
		t.Error("wrong code accepted")
	}
}
