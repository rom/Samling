package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
)

// Streaming authenticated encryption.
//
// Format:  magic(8) | noncePrefix(7) | chunk*
// chunk:   len(4, big endian, ciphertext length) | AES-256-GCM(ciphertext+tag)
// nonce:   noncePrefix(7) | counter(4) | last(1)
//
// Every chunk is authenticated, chunks cannot be reordered (counter) and the
// stream cannot be truncated (the final chunk carries the "last" flag).

const (
	streamMagic = "SMLSTRM1"
	chunkSize   = 64 * 1024
)

var ErrCorrupt = errors.New("encrypted stream is corrupt or the key is wrong")

type encWriter struct {
	w      io.Writer
	aead   cipher.AEAD
	prefix []byte
	buf    []byte
	ctr    uint32
	closed bool
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// NewEncryptWriter returns a WriteCloser that encrypts everything written to
// it. Close must be called to flush the final chunk.
func NewEncryptWriter(w io.Writer, key []byte) (io.WriteCloser, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	prefix := make([]byte, 7)
	if _, err := rand.Read(prefix); err != nil {
		return nil, err
	}
	if _, err := w.Write(append([]byte(streamMagic), prefix...)); err != nil {
		return nil, err
	}
	return &encWriter{w: w, aead: aead, prefix: prefix, buf: make([]byte, 0, chunkSize)}, nil
}

func nonce(prefix []byte, ctr uint32, last bool) []byte {
	n := make([]byte, 12)
	copy(n, prefix)
	binary.BigEndian.PutUint32(n[7:], ctr)
	if last {
		n[11] = 1
	}
	return n
}

func (e *encWriter) flush(last bool) error {
	ct := e.aead.Seal(nil, nonce(e.prefix, e.ctr, last), e.buf, nil)
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(ct)))
	if _, err := e.w.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := e.w.Write(ct); err != nil {
		return err
	}
	e.ctr++
	if e.ctr == 0 {
		return errors.New("stream too large")
	}
	e.buf = e.buf[:0]
	return nil
}

func (e *encWriter) Write(p []byte) (int, error) {
	if e.closed {
		return 0, errors.New("write after close")
	}
	total := len(p)
	for len(p) > 0 {
		// Only flush a full chunk when more data follows, so that the last
		// chunk is always written by Close with the "last" flag.
		if len(e.buf) == chunkSize {
			if err := e.flush(false); err != nil {
				return 0, err
			}
		}
		n := min(chunkSize-len(e.buf), len(p))
		e.buf = append(e.buf, p[:n]...)
		p = p[n:]
	}
	return total, nil
}

func (e *encWriter) Close() error {
	if e.closed {
		return nil
	}
	e.closed = true
	return e.flush(true)
}

type decReader struct {
	r      io.Reader
	aead   cipher.AEAD
	prefix []byte
	buf    []byte
	ctr    uint32
	done   bool
}

// NewDecryptReader returns a reader that decrypts and authenticates a stream
// produced by NewEncryptWriter.
func NewDecryptReader(r io.Reader, key []byte) (io.Reader, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	hdr := make([]byte, len(streamMagic)+7)
	if _, err := io.ReadFull(r, hdr); err != nil || string(hdr[:8]) != streamMagic {
		return nil, ErrCorrupt
	}
	return &decReader{r: r, aead: aead, prefix: hdr[8:]}, nil
}

func (d *decReader) Read(p []byte) (int, error) {
	for len(d.buf) == 0 {
		if d.done {
			return 0, io.EOF
		}
		var hdr [4]byte
		if _, err := io.ReadFull(d.r, hdr[:]); err != nil {
			return 0, ErrCorrupt // truncated: no final chunk seen
		}
		n := binary.BigEndian.Uint32(hdr[:])
		if n > chunkSize+uint32(d.aead.Overhead()) || n < uint32(d.aead.Overhead()) {
			return 0, ErrCorrupt
		}
		ct := make([]byte, n)
		if _, err := io.ReadFull(d.r, ct); err != nil {
			return 0, ErrCorrupt
		}
		pt, err := d.aead.Open(nil, nonce(d.prefix, d.ctr, false), ct, nil)
		if err != nil {
			pt, err = d.aead.Open(nil, nonce(d.prefix, d.ctr, true), ct, nil)
			if err != nil {
				return 0, ErrCorrupt
			}
			d.done = true
			// Nothing may follow the final chunk.
			var extra [1]byte
			if n, _ := d.r.Read(extra[:]); n > 0 {
				return 0, ErrCorrupt
			}
		}
		d.ctr++
		d.buf = pt
	}
	n := copy(p, d.buf)
	d.buf = d.buf[n:]
	return n, nil
}

// Passphrase protected container: magic(8) | salt(16) | stream.
const passMagic = "SMLPASS1"

// NewPassphraseWriter encrypts a stream with a key derived from a passphrase.
func NewPassphraseWriter(w io.Writer, passphrase string) (io.WriteCloser, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	if _, err := w.Write(append([]byte(passMagic), salt...)); err != nil {
		return nil, err
	}
	return NewEncryptWriter(w, PassphraseKey(passphrase, salt))
}

// NewPassphraseReader decrypts a stream written by NewPassphraseWriter.
func NewPassphraseReader(r io.Reader, passphrase string) (io.Reader, error) {
	hdr := make([]byte, len(passMagic)+16)
	if _, err := io.ReadFull(r, hdr); err != nil || string(hdr[:8]) != passMagic {
		return nil, ErrCorrupt
	}
	return NewDecryptReader(r, PassphraseKey(passphrase, hdr[8:]))
}
