// Package blob stores file contents. Two back-ends exist: the local file
// system (default) and any S3 compatible object store. All content is
// encrypted with AES-256-GCM before it reaches the back-end, so neither a
// stolen disk nor the object store operator can read the files.
package blob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/rom/samling/internal/crypto"
)

// Backend is the raw (ciphertext) object store.
type Backend interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, fn func(key string) error) error
}

var keyRe = regexp.MustCompile(`^[a-f0-9]{32}$`)

// ---------- file system ----------

type FS struct{ Dir string }

func NewFS(dir string) (*FS, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &FS{Dir: dir}, nil
}

func (f *FS) path(key string) (string, error) {
	if !keyRe.MatchString(key) { // also prevents path traversal
		return "", errors.New("invalid blob key")
	}
	return filepath.Join(f.Dir, key[:2], key), nil
}

func (f *FS) Put(_ context.Context, key string, r io.Reader) error {
	p, err := f.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}

func (f *FS) Get(_ context.Context, key string) (io.ReadCloser, error) {
	p, err := f.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

func (f *FS) Delete(_ context.Context, key string) error {
	p, err := f.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (f *FS) List(_ context.Context, fn func(string) error) error {
	return filepath.WalkDir(f.Dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !keyRe.MatchString(d.Name()) {
			return err
		}
		return fn(d.Name())
	})
}

// ---------- S3 ----------

type S3Config struct {
	Endpoint, Region, Bucket, AccessKey, SecretKey, Prefix string
	Insecure                                               bool
}

type S3 struct {
	c      *minio.Client
	bucket string
	prefix string
}

func NewS3(cfg S3Config) (*S3, error) {
	c, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: !cfg.Insecure,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, err
	}
	if cfg.Bucket == "" {
		return nil, errors.New("s3 bucket is required")
	}
	return &S3{c: c, bucket: cfg.Bucket, prefix: cfg.Prefix}, nil
}

func (s *S3) Put(ctx context.Context, key string, r io.Reader) error {
	if !keyRe.MatchString(key) {
		return errors.New("invalid blob key")
	}
	_, err := s.c.PutObject(ctx, s.bucket, s.prefix+key, r, -1, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	return err
}

func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if !keyRe.MatchString(key) {
		return nil, errors.New("invalid blob key")
	}
	return s.c.GetObject(ctx, s.bucket, s.prefix+key, minio.GetObjectOptions{})
}

func (s *S3) Delete(ctx context.Context, key string) error {
	if !keyRe.MatchString(key) {
		return errors.New("invalid blob key")
	}
	return s.c.RemoveObject(ctx, s.bucket, s.prefix+key, minio.RemoveObjectOptions{})
}

func (s *S3) List(ctx context.Context, fn func(string) error) error {
	for o := range s.c.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: s.prefix, Recursive: true}) {
		if o.Err != nil {
			return o.Err
		}
		k := o.Key[len(s.prefix):]
		if keyRe.MatchString(k) {
			if err := fn(k); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---------- encrypting store ----------

// Store encrypts on the way in and decrypts on the way out. Each object is
// encrypted with its own key, derived from the storage key (see
// crypto.Keys.BlobKey). Format: "SMLBLOB2" followed by the chunked AEAD
// stream. Objects written by the first release (one shared key, no prefix)
// are still readable.
type Store struct {
	B    Backend
	keys *crypto.Keys
}

const blobMagic = "SMLBLOB2"

func NewStore(b Backend, keys *crypto.Keys) *Store { return &Store{B: b, keys: keys} }

// Put encrypts r and stores it under key.
func (s *Store) Put(ctx context.Context, key string, r io.Reader) error {
	if !keyRe.MatchString(key) {
		return errors.New("invalid blob key")
	}
	pr, pw := io.Pipe()
	go func() {
		if _, err := pw.Write([]byte(blobMagic)); err != nil {
			pw.CloseWithError(err)
			return
		}
		enc, err := crypto.NewEncryptWriter(pw, s.keys.BlobKey(key))
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(enc, r); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.CloseWithError(enc.Close())
	}()
	err := s.B.Put(ctx, key, pr)
	pr.CloseWithError(err) // unblock the writer goroutine on back-end failure
	return err
}

type readCloser struct {
	io.Reader
	c io.Closer
}

func (r readCloser) Close() error { return r.c.Close() }

// Get returns the decrypted content.
func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	rc, err := s.B.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	head := make([]byte, len(blobMagic))
	var dec io.Reader
	if _, err = io.ReadFull(rc, head); err == nil {
		if string(head) == blobMagic {
			dec, err = crypto.NewDecryptReader(rc, s.keys.BlobKey(key))
		} else { // first-release object: shared key, the bytes read are its header
			dec, err = crypto.NewDecryptReader(io.MultiReader(bytes.NewReader(head), rc), s.keys.Blob)
		}
	}
	if err != nil {
		rc.Close()
		return nil, fmt.Errorf("blob %s: %w", key, err)
	}
	return readCloser{dec, rc}, nil
}

func (s *Store) Delete(ctx context.Context, key string) error { return s.B.Delete(ctx, key) }
