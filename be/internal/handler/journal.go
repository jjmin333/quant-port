package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type JournalHandler struct {
	svc *service.JournalService
}

func NewJournalHandler(svc *service.JournalService) *JournalHandler {
	return &JournalHandler{svc: svc}
}

func (h *JournalHandler) List(w http.ResponseWriter, r *http.Request) {
	notes, err := h.svc.List(r.Context(), r.URL.Query().Get("symbol"), r.URL.Query().Get("q"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": notes})
}

// journalNoteBody has no price fields on purpose — the write-time price is
// measured by the server, never stated by the client.
type journalNoteBody struct {
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	Symbol     string   `json:"symbol"`
	SymbolName string   `json:"symbolName"`
	Tags       []string `json:"tags"`
	Pinned     bool     `json:"pinned"`
}

func (b journalNoteBody) input() service.JournalNoteInput {
	return service.JournalNoteInput{
		Title:      b.Title,
		Body:       b.Body,
		Symbol:     b.Symbol,
		SymbolName: b.SymbolName,
		Tags:       b.Tags,
		Pinned:     b.Pinned,
	}
}

func (h *JournalHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body journalNoteBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	note, err := h.svc.Create(r.Context(), body.input())
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{"data": note})
}

func (h *JournalHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body journalNoteBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	note, err := h.svc.Update(r.Context(), id, body.input())
	if err != nil {
		if err == repository.ErrJournalNoteNotFound {
			response.Error(w, http.StatusNotFound, "note not found")
			return
		}
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": note})
}

func (h *JournalHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if err == repository.ErrJournalNoteNotFound {
			response.Error(w, http.StatusNotFound, "note not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]int64{"id": id}})
}
