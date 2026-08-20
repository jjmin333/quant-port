package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/enact/quant-port/be/docs"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// Admin endpoints for 배치 관리 (/admin/batch).

// GetBatchJobs returns every job with its last run and current coverage.
func (h *AdminHandler) GetBatchJobs(w http.ResponseWriter, r *http.Request) {
	if h.batch == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"jobs": []any{}}})
		return
	}
	ctx := r.Context()
	limit, _ := strconv.Atoi(r.URL.Query().Get("historyLimit"))
	history, err := h.batch.History(ctx, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if history == nil {
		history = []model.BatchJobRun{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"jobs":    h.batch.Catalog(ctx),
		"history": history,
	}})
}

// RunBatchJob starts one job. Returns 409 while another is in flight — the
// fetch jobs share the live app's Toss budget, so overlapping runs would starve
// the app's own polling rather than finish faster.
func (h *AdminHandler) RunBatchJob(w http.ResponseWriter, r *http.Request) {
	if h.batch == nil {
		response.Error(w, http.StatusServiceUnavailable, "batch service unavailable")
		return
	}
	jobID := model.BatchJobID(strings.TrimSpace(r.URL.Query().Get("job")))
	if jobID == "" {
		response.Error(w, http.StatusBadRequest, "job required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	params := service.BatchJobParams{
		Since:  strings.TrimSpace(r.URL.Query().Get("since")),
		Limit:  limit,
		Market: strings.TrimSpace(r.URL.Query().Get("market")),
	}

	if err := h.batch.Trigger(r.Context(), jobID, params); err != nil {
		if errors.Is(err, service.ErrBatchBusy) {
			response.Error(w, http.StatusConflict, err.Error())
			return
		}
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusAccepted, map[string]any{"data": map[string]any{
		"job":    jobID,
		"queued": true,
	}})
}

// GetBatchSchedule serves the operator-facing 배치 시간표 문서.
//
// 디스크 파일이 있으면 그것을 준다(관리자 화면에서 고친 내용). 없으면 embed
// 폴백 — 배포 때 파일을 빠뜨려도 빈 화면이 되지는 않는다.
func (h *AdminHandler) GetBatchSchedule(w http.ResponseWriter, r *http.Request) {
	doc := docs.LoadBatchSchedule()
	response.JSON(w, http.StatusOK, map[string]any{"data": batchSchedulePayload(doc)})
}

// PutBatchSchedule saves an operator edit of BATCH_SCHEDULE.md.
func (h *AdminHandler) PutBatchSchedule(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, docs.BatchScheduleMaxBytes+4096)
	var body struct {
		Markdown string `json:"markdown"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "markdown is required")
		return
	}
	doc, err := docs.SaveBatchSchedule(body.Markdown)
	if err != nil {
		msg := err.Error()
		status := http.StatusBadRequest
		if strings.Contains(msg, "write ") {
			status = http.StatusInternalServerError
		}
		response.Error(w, status, msg)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": batchSchedulePayload(doc)})
}

func batchSchedulePayload(doc docs.BatchScheduleDoc) map[string]any {
	return map[string]any{
		"markdown": doc.Markdown,
		"path":     doc.Path,
		"source":   doc.Source,
		"writable": doc.Writable,
	}
}
