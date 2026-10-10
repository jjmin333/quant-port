package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/growth"
	"github.com/enact/quant-port/be/pkg/response"
)

/*
AdminGrowthHandler — worker-growth가 만든 보고서를 읽는다(관리자 전용 · 읽기만).

보고서를 만드는 것은 워커다. 여기서 「지금 다시 만들기」를 두지 않은 이유: 실행은 이미
배치 콘솔(`POST /admin/batch/run` · growth_audit)에 있다 — 문을 하나만 둔다.
*/
type AdminGrowthHandler struct {
	reports *growth.ReportReader
}

func NewAdminGrowthHandler(reports *growth.ReportReader) *AdminGrowthHandler {
	return &AdminGrowthHandler{reports: reports}
}

// List serves GET /admin/growth/reports?kind=daily|weekly&limit=30
func (h *AdminGrowthHandler) List(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != "daily" && kind != "weekly" {
		response.Error(w, http.StatusBadRequest, "kind must be daily or weekly")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.reports.List(r.Context(), kind, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// Get serves GET /admin/growth/reports/{id} — id 자리에 "latest"를 주면 ?kind= 의 최신 보고서.
func (h *AdminGrowthHandler) Get(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "id")
	var id int64
	kind := r.URL.Query().Get("kind")
	if raw != "latest" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v <= 0 {
			response.Error(w, http.StatusBadRequest, "id must be a positive integer or \"latest\"")
			return
		}
		id = v
	} else if kind != "" && kind != "daily" && kind != "weekly" {
		response.Error(w, http.StatusBadRequest, "kind must be daily or weekly")
		return
	}
	out, err := h.reports.Get(r.Context(), id, kind)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if out == nil {
		response.Error(w, http.StatusNotFound, "report not found")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}
