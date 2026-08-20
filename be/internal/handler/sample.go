package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/enact/quant-port/be/internal/sample"
	"github.com/enact/quant-port/be/pkg/response"
)

// SampleHandler serves the demo dataset for the "내 투자" menus.
//
// These routes are deliberately **outside** the auth group. Every real endpoint
// behind these screens requires a session, so without a public path a
// logged-out visitor can be shown nothing at all and has to decide whether to
// sign up from an empty page. Nothing here reads or writes user data — the
// payloads are fixtures — so there is no session to check.
type SampleHandler struct{}

func NewSampleHandler() *SampleHandler { return &SampleHandler{} }

// cacheFor lets the CDN/browser hold a sample for a while. The fixtures only
// change when the binary does, but the dates inside are relative to now, so the
// window stays short enough that "3일 전" never drifts into being wrong.
const sampleCacheSeconds = 300

func (h *SampleHandler) write(w http.ResponseWriter, payload any) {
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", sampleCacheSeconds))
	response.JSON(w, http.StatusOK, map[string]any{"data": payload})
}

func (h *SampleHandler) Stamps(w http.ResponseWriter, r *http.Request) {
	h.write(w, sample.Stamps(time.Now().UTC()))
}

func (h *SampleHandler) StampSummary(w http.ResponseWriter, r *http.Request) {
	h.write(w, sample.StampSummary(time.Now().UTC()))
}

func (h *SampleHandler) Watchlist(w http.ResponseWriter, r *http.Request) {
	h.write(w, sample.Watchlist(time.Now().UTC()))
}

func (h *SampleHandler) Holdings(w http.ResponseWriter, r *http.Request) {
	h.write(w, sample.Holdings(time.Now().UTC()))
}

func (h *SampleHandler) Journal(w http.ResponseWriter, r *http.Request) {
	h.write(w, sample.Journal(time.Now().UTC()))
}

func (h *SampleHandler) Targets(w http.ResponseWriter, r *http.Request) {
	h.write(w, sample.Targets(time.Now().UTC()))
}
