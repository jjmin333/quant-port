package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/media"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
)

// A 1x1 transparent PNG.
var testPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func newMediaHandler(t *testing.T, maxBytes int64) *MediaHandler {
	t.Helper()
	store, err := media.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return NewMediaHandler(service.NewMediaService(store, maxBytes, ""))
}

// uploadRequest builds the multipart request a browser sends. contentType is
// what the *client claims* — several tests below depend on it being a lie.
func uploadRequest(t *testing.T, field, filename, contentType string, body []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	header := make(map[string][]string)
	header["Content-Disposition"] = []string{
		`form-data; name="` + field + `"; filename="` + filename + `"`,
	}
	header["Content-Type"] = []string{contentType}
	part, err := w.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/media/images", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestUploadImageStoresAndServesRoundTrip(t *testing.T) {
	h := newMediaHandler(t, 5<<20)

	rec := httptest.NewRecorder()
	h.UploadImage(rec, uploadRequest(t, "file", "shot.png", "image/png", testPNG))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: want 201, got %d (%s)", rec.Code, rec.Body.String())
	}

	var payload struct {
		Data model.MediaUpload `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	up := payload.Data
	if up.URL != "/api/v1/media/"+up.Key {
		t.Fatalf("url must point at the serve route: %q", up.URL)
	}
	if up.ContentType != "image/png" || up.Size != int64(len(testPNG)) {
		t.Fatalf("metadata: %+v", up)
	}
	if up.Width != 1 || up.Height != 1 {
		t.Fatalf("dimensions must come from the header: %dx%d", up.Width, up.Height)
	}

	// Serve it back through the router so the wildcard param is populated the
	// same way the real route does it.
	r := chi.NewRouter()
	r.Get("/api/v1/media/*", h.Serve)

	got := httptest.NewRecorder()
	r.ServeHTTP(got, httptest.NewRequest(http.MethodGet, up.URL, nil))
	if got.Code != http.StatusOK {
		t.Fatalf("serve: want 200, got %d", got.Code)
	}
	if !bytes.Equal(got.Body.Bytes(), testPNG) {
		t.Fatal("served bytes differ from the uploaded ones")
	}
	if ct := got.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("content type: %q", ct)
	}
	// The headers that keep user-uploaded bytes from being run as anything.
	if got.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("nosniff is required on user-uploaded content")
	}
	if got.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("served media must carry a CSP")
	}
}

// The declared content type is a claim, not evidence: HTML sent as image/png
// with a .png name must still be refused.
func TestUploadImageRefusesNonImageBytes(t *testing.T) {
	h := newMediaHandler(t, 5<<20)
	rec := httptest.NewRecorder()
	h.UploadImage(rec, uploadRequest(t, "file", "evil.png", "image/png",
		[]byte("<html><script>alert(1)</script></html>")))
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("want 415, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestUploadImageRefusesSVG(t *testing.T) {
	h := newMediaHandler(t, 5<<20)
	rec := httptest.NewRecorder()
	h.UploadImage(rec, uploadRequest(t, "file", "logo.svg", "image/svg+xml",
		[]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)))
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("svg is a scriptable document, want 415, got %d", rec.Code)
	}
}

func TestUploadImageEnforcesSizeLimit(t *testing.T) {
	h := newMediaHandler(t, 64) // bytes
	big := append(append([]byte{}, testPNG...), bytes.Repeat([]byte{0}, 512)...)
	rec := httptest.NewRecorder()
	h.UploadImage(rec, uploadRequest(t, "file", "big.png", "image/png", big))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d (%s)", rec.Code, rec.Body.String())
	}
}

// The field name the FE uses is `file`; `image` is accepted as an alias so a
// second caller does not have to know which one we picked.
func TestUploadImageAcceptsImageFieldName(t *testing.T) {
	h := newMediaHandler(t, 5<<20)
	rec := httptest.NewRecorder()
	h.UploadImage(rec, uploadRequest(t, "image", "shot.png", "image/png", testPNG))
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestServeRejectsTraversalAndUnknownKeys(t *testing.T) {
	h := newMediaHandler(t, 5<<20)
	r := chi.NewRouter()
	r.Get("/api/v1/media/*", h.Serve)

	for _, path := range []string{
		"/api/v1/media/ab/cd/notahash.png",
		"/api/v1/media/ab/cd/" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" + ".png",
		"/api/v1/media/etc/passwd",
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: want 404, got %d", path, rec.Code)
		}
	}
}

func TestUploadWithoutFileIsBadRequest(t *testing.T) {
	h := newMediaHandler(t, 5<<20)
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("note", "no file here")
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/media/images", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())

	rec := httptest.NewRecorder()
	h.UploadImage(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
	if _, err := io.ReadAll(rec.Body); err != nil {
		t.Fatal(err)
	}
}
