// Package crypto contains the cryptographic building blocks used by Samling:
// key derivation, field level encryption for PII, blind indexes, password
// hashing, TOTP and a streaming AEAD format for blobs, backups and exports.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

// Keys holds the sub keys derived from the master key. Each purpose gets its
// own key so that a weakness in one use does not affect the others.
type Keys struct {
	master []byte
	field  cipher.AEAD
	index  []byte
	Blob   []byte
}

// NewKeys derives the purpose specific keys from a 32 byte master key.
func NewKeys(master []byte) (*Keys, error) {
	if len(master) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}
	derive := func(info string) []byte {
		k, err := hkdf.Key(sha256.New, master, nil, "samling/v1/"+info, 32)
		if err != nil {
			panic(err)
		}
		return k
	}
	block, err := aes.NewCipher(derive("field"))
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Keys{master: master, field: aead, index: derive("index"), Blob: derive("blob")}, nil
}

// Master returns the raw master key (needed for self-contained backups).
func (k *Keys) Master() []byte { return k.master }

// GenerateKey returns a new random 32 byte key.
func GenerateKey() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

// EncryptField encrypts a short string (PII, secrets) with AES-256-GCM. The
// context string is bound as additional data so a ciphertext cannot be moved
// to another column. Empty input gives empty output.
func (k *Keys) EncryptField(plain, context string) string {
	if plain == "" {
		return ""
	}
	nonce := make([]byte, k.field.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	ct := k.field.Seal(nonce, nonce, []byte(plain), []byte(context))
	return "v1:" + base64.RawStdEncoding.EncodeToString(ct)
}

// DecryptField reverses EncryptField.
func (k *Keys) DecryptField(enc, context string) (string, error) {
	if enc == "" {
		return "", nil
	}
	raw, ok := strings.CutPrefix(enc, "v1:")
	if !ok {
		return "", errors.New("unknown field encryption version")
	}
	ct, err := base64.RawStdEncoding.DecodeString(raw)
	if err != nil || len(ct) < k.field.NonceSize() {
		return "", errors.New("malformed ciphertext")
	}
	n := k.field.NonceSize()
	pt, err := k.field.Open(nil, ct[:n], ct[n:], []byte(context))
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// BlindIndex returns a keyed hash that allows equality lookups on encrypted
// values (for example unique e-mail addresses) without storing them in clear.
func (k *Keys) BlindIndex(value string) string {
	if value == "" {
		return ""
	}
	m := hmac.New(sha256.New, k.index)
	m.Write([]byte(strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(m.Sum(nil)[:16])
}

// Token returns a new random URL safe token with n bytes of entropy.
func Token(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// NewID returns a 128 bit identifier in hex that sorts by creation time:
// 48 bits of milliseconds followed by 80 random bits (like UUIDv7). Rows
// created within the same second therefore still have a stable order.
func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	ms := uint64(time.Now().UnixMilli())
	b[0], b[1], b[2], b[3], b[4], b[5] = byte(ms>>40), byte(ms>>32), byte(ms>>24), byte(ms>>16), byte(ms>>8), byte(ms)
	return hex.EncodeToString(b)
}

// RandomID returns a fully random 128 bit identifier in hex, for records
// whose creation time must not be derivable (anonymous ballots).
func RandomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// HashToken hashes a session token for storage, so a leaked database does not
// leak usable sessions.
func HashToken(tok string) string {
	s := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(s[:])
}

// Argon2id parameters (OWASP recommendation: m=19 MiB, t=2, p=1).
const (
	argonTime    = 2
	argonMemory  = 19 * 1024
	argonThreads = 1
	argonKeyLen  = 32
)

// HashPassword returns an argon2id hash in PHC string format.
func HashPassword(pw string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}
	h := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(h))
}

// VerifyPassword checks a password against a PHC formatted argon2id hash in
// constant time.
func VerifyPassword(pw, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	if m > 1<<20 || t > 16 || p == 0 { // refuse absurd parameters
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[4])
	want, err2 := base64.RawStdEncoding.DecodeString(parts[5])
	if err1 != nil || err2 != nil || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// PassphraseKey derives a 32 byte key from a passphrase for backups/exports.
func PassphraseKey(passphrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(passphrase), salt, 3, 64*1024, 2, 32)
}
