package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// A 1x1 PNG — the smallest thing that still sniffs as image/png.
var pngBytes = []byte{
	0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89,
}

func TestSniffRejectsNonImages(t *testing.T) {
	for name, data := range map[string][]byte{
		"html":   []byte("<html><script>alert(1)</script></html>"),
		"svg":    []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"script": []byte("#!/bin/sh\nrm -rf /\n"),
	} {
		if _, err := SniffImageType(data); !errors.Is(err, ErrUnsupportedType) {
			t.Fatalf("%s must be rejected, got %v", name, err)
		}
	}
	if _, err := SniffImageType(nil); !errors.Is(err, ErrEmpty) {
		t.Fatal("an empty upload is not an image")
	}
	if ct, err := SniffImageType(pngBytes); err != nil || ct != "image/png" {
		t.Fatalf("png: got %q, %v", ct, err)
	}
}

// The sniff must win over whatever the upload claimed — a .png name on an HTML
// payload is exactly the case this protects against.
func TestKeyExtensionFollowsSniffedType(t *testing.T) {
	key, err := BuildKey(pngBytes, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(key) != ".png" {
		t.Fatalf("want .png, got %q", key)
	}
	if !ValidKey(key) {
		t.Fatalf("BuildKey produced a key its own validator rejects: %q", key)
	}
	if _, err := BuildKey(pngBytes, "image/svg+xml"); !errors.Is(err, ErrUnsupportedType) {
		t.Fatal("svg must not produce a key")
	}
}

func TestValidKeyRejectsTraversal(t *testing.T) {
	for _, key := range []string{
		"../../etc/passwd",
		"ab/cd/../../../etc/passwd.png",
		"ab/cd/notahash.png",
		"/absolute/path.png",
		"ab/cd/" + string(make([]byte, 64)) + ".png",
	} {
		if ValidKey(key) {
			t.Fatalf("%q must not be a valid key", key)
		}
	}
}

func TestLocalStoreRoundTripAndDedupe(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	first, err := store.Put(ctx, pngBytes, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Put(ctx, pngBytes, "image/png")
	if err != nil {
		t.Fatalf("storing identical bytes twice must succeed: %v", err)
	}
	if first.Key != second.Key {
		t.Fatalf("content-addressed keys must match: %s vs %s", first.Key, second.Key)
	}

	rc, obj, err := store.Open(ctx, first.Key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		rc.Close()
		t.Fatal(err)
	}
	// Closed before the Delete below: Windows refuses to remove a file that
	// still has an open handle, so a deferred Close would fail the test on the
	// platform this repo is developed on.
	if err := rc.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pngBytes) {
		t.Fatal("stored bytes differ from what was read back")
	}
	if obj.ContentType != "image/png" || obj.Size != int64(len(pngBytes)) {
		t.Fatalf("metadata: %+v", obj)
	}

	// No leftover temp files: a rename-based write must not litter the tree.
	entries, err := os.ReadDir(filepath.Join(dir, first.Key[0:2], first.Key[3:5]))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want exactly the stored object, got %d entries", len(entries))
	}

	if err := store.Delete(ctx, first.Key); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Open(ctx, first.Key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
}

func TestLocalStoreRefusesInvalidKeys(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Open(context.Background(), "../../etc/passwd"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("traversal must be ErrNotFound, got %v", err)
	}
}
