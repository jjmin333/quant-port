package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/media"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type MediaHandler struct {
	svc *service.MediaService
}

func NewMediaHandler(svc *service.MediaService) *MediaHandler {
	return &MediaHandler{svc: svc}
}

// UploadImage accepts one image from a multipart form (`file`, or `image`).
//
// Upload requires an account; serving does not. That split is deliberate: a
// report body is public, so its images must be readable by logged-out readers,
// but only a signed-in author may add bytes to our disk.
func (h *MediaHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		response.Error(w, http.StatusServiceUnavailable, "이미지 저장소가 설정되지 않았습니다")
		return
	}

	max := h.svc.MaxBytes()
	// Cap the request body itself, with a small allowance for multipart
	// framing, so an oversized upload is refused at the socket instead of
	// being buffered to disk by ParseMultipartForm first.
	r.Body = http.MaxBytesReader(w, r.Body, max+(1<<20))
	if err := r.ParseMultipartForm(max + (1 << 20)); err != nil {
		response.Error(w, http.StatusRequestEntityTooLarge,
			"이미지가 너무 큽니다 (최대 "+strconv.FormatInt(max/(1<<20), 10)+"MB)")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	file, _, err := r.FormFile("file")
	if err != nil {
		if file, _, err = r.FormFile("image"); err != nil {
			response.Error(w, http.StatusBadRequest, "이미지 파일이 필요합니다")
			return
		}
	}
	defer file.Close()

	upload, err := h.svc.SaveImage(r.Context(), file)
	if err != nil {
		writeMediaError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{"data": upload})
}

// Serve streams a stored blob.
//
// Keys are content hashes, so a stored object never changes and can be cached
// indefinitely. The two headers below matter more than the cache one: nosniff
// keeps a browser from re-interpreting the bytes as something executable, and
// the CSP makes the response inert even if it somehow were.
func (h *MediaHandler) Serve(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		response.Error(w, http.StatusNotFound, "not found")
		return
	}
	key := chi.URLParam(r, "*")
	rc, obj, err := h.svc.Open(r.Context(), key)
	if err != nil {
		if errors.Is(err, media.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to read media")
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", obj.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(obj.Size, 10))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}

func writeMediaError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, media.ErrTooLarge):
		response.Error(w, http.StatusRequestEntityTooLarge, "이미지가 너무 큽니다")
	case errors.Is(err, media.ErrEmpty):
		response.Error(w, http.StatusBadRequest, "빈 파일입니다")
	case errors.Is(err, media.ErrUnsupportedType):
		response.Error(w, http.StatusUnsupportedMediaType,
			"JPG · PNG · GIF · WebP 이미지만 올릴 수 있습니다")
	default:
		response.Error(w, http.StatusInternalServerError, "이미지 저장에 실패했습니다")
	}
}

// UploadCommunityImage accepts a larger raw file than UploadImage and stores a
// compressed copy (≤1MB, 평균 100~200KB — 2026-09-27 사용자 결정).
func (h *MediaHandler) UploadCommunityImage(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		response.Error(w, http.StatusServiceUnavailable, "이미지 저장소가 설정되지 않았습니다")
		return
	}
	max := int64(service.CommunityImageRawMax)
	r.Body = http.MaxBytesReader(w, r.Body, max+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		response.Error(w, http.StatusRequestEntityTooLarge, "이미지가 너무 큽니다 (최대 15MB)")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	file, _, err := r.FormFile("file")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "이미지 파일이 필요합니다")
		return
	}
	defer file.Close()
	upload, err := h.svc.SaveCompressedImage(r.Context(), file)
	if err != nil {
		writeMediaError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{"data": upload})
}
