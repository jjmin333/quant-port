// Package media is the blob layer for user-uploaded files.
//
// It exists as its own package, and behind an interface, because image upload
// arrives here first (투자 리포트 본문) but is not a reporting feature: the
// journal, stamps and a future profile picture all need the same three things —
// put bytes somewhere, hand back a URL, serve them back safely. Anything that
// needs those should depend on media.Store, never on a directory path.
//
// Two rules make a later backend swap (S3/GCS/CDN) a wiring change rather than
// a rewrite:
//
//  1. Keys are content-addressed (sha256 of the bytes) and immutable. The same
//     upload twice is one object, and a stored object can be cached forever.
//  2. The content type is decided by sniffing the bytes here, never by trusting
//     the client's declared type or file name. What the caller claims is not
//     evidence of what the file is.
package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

var (
	// ErrNotFound is returned by Open/Delete for a key that is not stored.
	ErrNotFound = errors.New("media: not found")
	// ErrUnsupportedType is returned when the sniffed bytes are not a type we
	// accept. The caller should surface it as a 4xx, not a 5xx.
	ErrUnsupportedType = errors.New("media: unsupported content type")
	// ErrTooLarge is returned when the payload exceeds the caller's limit.
	ErrTooLarge = errors.New("media: file too large")
	// ErrEmpty is returned for a zero-byte upload.
	ErrEmpty = errors.New("media: empty file")
)

// Object is one stored blob's metadata. There is no owner field on purpose:
// keys are content hashes, so the same bytes uploaded by two people are one
// object. Ownership lives with whatever references the URL.
type Object struct {
	Key         string `json:"key"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

// Store is the blob backend. Implementations must be safe for concurrent use.
type Store interface {
	// Put stores data and returns its object. Storing the same bytes twice
	// returns the same key and must not fail.
	Put(ctx context.Context, data []byte, contentType string) (Object, error)
	// Open returns the stored bytes. The caller closes the reader.
	Open(ctx context.Context, key string) (io.ReadCloser, Object, error)
	// Delete removes an object. Deleting a missing key returns ErrNotFound.
	Delete(ctx context.Context, key string) error
}

// imageTypes are the only types accepted. SVG is deliberately absent: an SVG is
// a document that can carry script, so "an image the browser will run" is not
// an image as far as this package is concerned.
var imageTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/gif":  "gif",
	"image/webp": "webp",
}

// keyPattern is the shape BuildKey produces. Serving code must validate against
// it before touching any backend — it is what makes "../../etc/passwd" not a
// key rather than a path traversal.
var keyPattern = regexp.MustCompile(`^[0-9a-f]{2}/[0-9a-f]{2}/[0-9a-f]{64}\.(jpg|png|gif|webp)$`)

// ValidKey reports whether a key can be looked up. Anything else is rejected
// without reaching the backend.
func ValidKey(key string) bool { return keyPattern.MatchString(key) }

// SniffImageType returns the canonical content type of data, or
// ErrUnsupportedType. The sniff, not the upload's declared type, is what gets
// stored and later served.
func SniffImageType(data []byte) (string, error) {
	if len(data) == 0 {
		return "", ErrEmpty
	}
	// http.DetectContentType reads at most 512 bytes and recognises every type
	// in imageTypes, including webp (RIFF….WEBP).
	sniffed := http.DetectContentType(data)
	if idx := strings.IndexByte(sniffed, ';'); idx >= 0 {
		sniffed = strings.TrimSpace(sniffed[:idx])
	}
	if _, ok := imageTypes[sniffed]; !ok {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedType, sniffed)
	}
	return sniffed, nil
}

// Extension returns the file extension for an accepted content type.
func Extension(contentType string) (string, bool) {
	ext, ok := imageTypes[contentType]
	return ext, ok
}

// BuildKey derives the content-addressed key for data.
//
// The two-level fan-out keeps a filesystem backend from putting a million
// entries in one directory, and is harmless for object stores, which read it as
// an ordinary prefix.
func BuildKey(data []byte, contentType string) (string, error) {
	ext, ok := Extension(contentType)
	if !ok {
		return "", ErrUnsupportedType
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	return fmt.Sprintf("%s/%s/%s.%s", hash[0:2], hash[2:4], hash, ext), nil
}
