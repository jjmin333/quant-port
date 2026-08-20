package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"strings"

	// Registered for image.DecodeConfig only — headers, never full decodes.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/enact/quant-port/be/internal/media"
	"github.com/enact/quant-port/be/internal/model"
)

// MediaService is the app-level entry point for user uploads.
//
// It owns the policy (what is allowed, how big, what URL callers get) while
// media.Store owns the bytes. A second consumer — journal attachments, profile
// pictures — should call this service rather than reaching for a store or a
// directory of its own.
type MediaService struct {
	store    media.Store
	maxBytes int64
	// publicBase, when set, replaces the API path in returned URLs — that is
	// the CDN/object-store hook. Empty means "serve it ourselves".
	publicBase string
}

// mediaAPIPath is where the API serves blobs when publicBase is empty.
const mediaAPIPath = "/api/v1/media/"

func NewMediaService(store media.Store, maxBytes int64, publicBase string) *MediaService {
	if maxBytes <= 0 {
		maxBytes = 5 << 20
	}
	return &MediaService{
		store:      store,
		maxBytes:   maxBytes,
		publicBase: strings.TrimRight(strings.TrimSpace(publicBase), "/"),
	}
}

// MaxBytes is the per-file limit, exposed so the handler can cap the request
// body before reading it all into memory.
func (s *MediaService) MaxBytes() int64 { return s.maxBytes }

// SaveImage validates and stores an uploaded image.
//
// The reader is consumed up to MaxBytes+1 so an oversized upload is rejected
// rather than buffered whole. The declared filename and content type from the
// multipart part are deliberately not parameters: they are claims, and the
// stored type comes from sniffing the bytes.
func (s *MediaService) SaveImage(ctx context.Context, r io.Reader) (model.MediaUpload, error) {
	if s == nil || s.store == nil {
		return model.MediaUpload{}, fmt.Errorf("media storage is not configured")
	}

	data, err := io.ReadAll(io.LimitReader(r, s.maxBytes+1))
	if err != nil {
		return model.MediaUpload{}, err
	}
	if int64(len(data)) > s.maxBytes {
		return model.MediaUpload{}, media.ErrTooLarge
	}

	contentType, err := media.SniffImageType(data)
	if err != nil {
		return model.MediaUpload{}, err
	}

	obj, err := s.store.Put(ctx, data, contentType)
	if err != nil {
		return model.MediaUpload{}, err
	}

	out := model.MediaUpload{
		Key:         obj.Key,
		URL:         s.URLFor(obj.Key),
		ContentType: obj.ContentType,
		Size:        obj.Size,
	}
	// DecodeConfig reads the header only. A format we did not register (webp)
	// simply leaves the dimensions at zero instead of failing the upload.
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		out.Width, out.Height = cfg.Width, cfg.Height
	}
	return out, nil
}

// Open returns a stored blob for serving.
func (s *MediaService) Open(ctx context.Context, key string) (io.ReadCloser, media.Object, error) {
	if s == nil || s.store == nil {
		return nil, media.Object{}, media.ErrNotFound
	}
	if !media.ValidKey(key) {
		return nil, media.Object{}, media.ErrNotFound
	}
	return s.store.Open(ctx, key)
}

// URLFor maps a key to the URL clients embed.
func (s *MediaService) URLFor(key string) string {
	if s.publicBase != "" {
		return s.publicBase + "/" + key
	}
	return mediaAPIPath + key
}
