package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type ImportHandler struct {
	svc *service.ImportJobService
}

func NewImportHandler(svc *service.ImportJobService) *ImportHandler {
	return &ImportHandler{svc: svc}
}

func (h *ImportHandler) UploadScreenshots(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 64 << 20
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	var meta []model.ImportImageMeta
	if raw := r.FormValue("meta"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &meta); err != nil {
			response.Error(w, http.StatusBadRequest, "invalid meta json")
			return
		}
	}

	files := r.MultipartForm.File["images"]
	if len(files) == 0 {
		files = r.MultipartForm.File["images[]"]
	}
	if len(files) == 0 {
		response.Error(w, http.StatusBadRequest, "images required")
		return
	}

	jobID, err := h.svc.StartScreenshotsImport(r.Context(), files, meta)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusAccepted, map[string]any{"data": map[string]string{"jobId": jobID}})
}

func (h *ImportHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	snap, err := h.svc.GetJob(r.Context(), jobID)
	if err != nil {
		response.Error(w, http.StatusNotFound, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": snap})
}

func (h *ImportHandler) CommitJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	var req model.ImportCommitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if req.ReplaceExisting == false && !req.AddToWatchlist {
		req.ReplaceExisting = true
	}
	count, watchAdded, err := h.svc.CommitJob(r.Context(), jobID, req)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"holdingsCount":  count,
			"watchlistAdded": watchAdded,
		},
	})
}
