package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalStore keeps blobs on the filesystem under BaseDir.
//
// This is the first backend, not the only intended one. Everything specific to
// "files in a directory" is in this file; swapping in S3 means writing another
// Store and changing one line in server wiring. In Docker the directory must be
// a named volume — the writable layer is discarded on every recreate, and an
// uploaded image outliving one deploy is the whole point.
type LocalStore struct {
	baseDir string
}

func NewLocalStore(baseDir string) (*LocalStore, error) {
	dir := strings.TrimSpace(baseDir)
	if dir == "" {
		return nil, errors.New("media: base dir required")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("media: base dir: %w", err)
	}
	if err := os.MkdirAll(abs, 0o775); err != nil {
		return nil, fmt.Errorf("media: base dir: %w", err)
	}
	return &LocalStore{baseDir: abs}, nil
}

// BaseDir is exposed for startup logging and health output only.
func (s *LocalStore) BaseDir() string { return s.baseDir }

func (s *LocalStore) Put(_ context.Context, data []byte, contentType string) (Object, error) {
	key, err := BuildKey(data, contentType)
	if err != nil {
		return Object{}, err
	}
	path := s.path(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o775); err != nil {
		return Object{}, err
	}

	obj := Object{Key: key, ContentType: contentType, Size: int64(len(data))}
	if _, err := os.Stat(path); err == nil {
		// Same bytes already stored. Keys are content hashes, so this is a
		// success, not a conflict.
		return obj, nil
	}

	// Write to a temp file in the same directory and rename, so a crash or a
	// concurrent upload of the same bytes can never leave a half-written file
	// under a key that claims to be that content's hash.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return Object{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeded

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return Object{}, err
	}
	if err := tmp.Close(); err != nil {
		return Object{}, err
	}
	if err := os.Chmod(tmpName, 0o664); err != nil {
		return Object{}, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return Object{}, err
	}
	return obj, nil
}

func (s *LocalStore) Open(_ context.Context, key string) (io.ReadCloser, Object, error) {
	if !ValidKey(key) {
		return nil, Object{}, ErrNotFound
	}
	path := s.path(key)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, Object{}, ErrNotFound
		}
		return nil, Object{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, Object{}, ErrNotFound
		}
		return nil, Object{}, err
	}
	return f, Object{Key: key, ContentType: contentTypeOfKey(key), Size: info.Size()}, nil
}

func (s *LocalStore) Delete(_ context.Context, key string) error {
	if !ValidKey(key) {
		return ErrNotFound
	}
	if err := os.Remove(s.path(key)); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// path maps a key to a file. ValidKey has already ruled out anything that is
// not two hex bytes, a hash and a known extension, so no separator or ".." can
// reach here.
func (s *LocalStore) path(key string) string {
	return filepath.Join(s.baseDir, filepath.FromSlash(key))
}

func contentTypeOfKey(key string) string {
	ext := strings.TrimPrefix(filepath.Ext(key), ".")
	for ct, e := range imageTypes {
		if e == ext {
			return ct
		}
	}
	return "application/octet-stream"
}
