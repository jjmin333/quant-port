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

// CommunityImageRawMax is the largest *uploaded* file the community accepts before
// compression. 화면이 올리기 전에 먼저 줄여 보내지만(브라우저 캔버스), 그게 안 되는
// 환경에서도 휴대폰 원본 사진은 받아야 한다.
const CommunityImageRawMax = 15 << 20

// communityImageMaxPixels guards decode memory: 40MP RGBA ≈ 160MB. 휴대폰 사진(12~50MP)
// 중 이보다 큰 것은 화면이 줄여 보내므로 서버에 원본이 올 일이 드물다.
const communityImageMaxPixels = 40_000_000

// communityCompressSlots bounds concurrent decodes — 무료 인스턴스 메모리를 지킨다.
var communityCompressSlots = make(chan struct{}, 2)

// SaveCompressedImage re-encodes an upload (media.CompressImage: ≤1600px JPEG,
// 목표 200KB · 상한 1MB, EXIF 제거) and stores the result.
func (s *MediaService) SaveCompressedImage(ctx context.Context, r io.Reader) (model.MediaUpload, error) {
	if s == nil || s.store == nil {
		return model.MediaUpload{}, fmt.Errorf("media storage is not configured")
	}
	data, err := io.ReadAll(io.LimitReader(r, CommunityImageRawMax+1))
	if err != nil {
		return model.MediaUpload{}, err
	}
	if int64(len(data)) > CommunityImageRawMax {
		return model.MediaUpload{}, media.ErrTooLarge
	}
	if len(data) == 0 {
		return model.MediaUpload{}, media.ErrEmpty
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil && cfg.Width*cfg.Height > communityImageMaxPixels {
		return model.MediaUpload{}, media.ErrTooLarge
	}

	select {
	case communityCompressSlots <- struct{}{}:
	case <-ctx.Done():
		return model.MediaUpload{}, ctx.Err()
	}
	out, err := media.CompressImage(data)
	<-communityCompressSlots
	if err != nil {
		return model.MediaUpload{}, err
	}
	obj, err := s.store.Put(ctx, out.Data, out.ContentType)
	if err != nil {
		return model.MediaUpload{}, err
	}
	return model.MediaUpload{
		Key:         obj.Key,
		URL:         s.URLFor(obj.Key),
		ContentType: obj.ContentType,
		Size:        obj.Size,
		Width:       out.Width,
		Height:      out.Height,
	}, nil
}
