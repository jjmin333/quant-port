package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

func (h *AdminHandler) dbAdminReady(w http.ResponseWriter) bool {
	if h.dbAdmin == nil {
		response.Error(w, http.StatusServiceUnavailable, "database console unavailable")
		return false
	}
	return true
}

// writeDBAdminError maps service errors onto status codes; anything unmapped is
// a 500 with the driver message suppressed.
func writeDBAdminError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrDBTableUnknown):
		response.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrDBTableReadOnly):
		response.Error(w, http.StatusForbidden, "이 테이블은 콘솔에서 읽기 전용입니다")
	case errors.Is(err, service.ErrDBColumnUnknown),
		errors.Is(err, service.ErrDBNoChanges):
		response.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrDBRowNotFound):
		response.Error(w, http.StatusNotFound, "대상 행을 찾을 수 없습니다")
	default:
		response.Error(w, http.StatusInternalServerError, err.Error())
	}
}

func (h *AdminHandler) ListDBTables(w http.ResponseWriter, r *http.Request) {
	if !h.dbAdminReady(w) {
		return
	}
	tables, err := h.dbAdmin.ListTables(r.Context())
	if err != nil {
		writeDBAdminError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": tables})
}

func (h *AdminHandler) DescribeDBTable(w http.ResponseWriter, r *http.Request) {
	if !h.dbAdminReady(w) {
		return
	}
	detail, err := h.dbAdmin.Describe(r.Context(), chi.URLParam(r, "table"))
	if err != nil {
		writeDBAdminError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": detail})
}

func (h *AdminHandler) ListDBRows(w http.ResponseWriter, r *http.Request) {
	if !h.dbAdminReady(w) {
		return
	}
	q := r.URL.Query()
	page, err := h.dbAdmin.ListRows(r.Context(), service.RowQuery{
		Table:    chi.URLParam(r, "table"),
		Limit:    service.ParseIntParam(q.Get("limit"), 50),
		Offset:   service.ParseIntParam(q.Get("offset"), 0),
		Search:   q.Get("q"),
		SortBy:   q.Get("sortBy"),
		SortDesc: !strings.EqualFold(q.Get("sortDir"), "asc"),
	})
	if err != nil {
		writeDBAdminError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": page})
}

type dbRowWriteRequest struct {
	Key    map[string]any `json:"key"`
	Values map[string]any `json:"values"`
}

func decodeDBRowRequest(w http.ResponseWriter, r *http.Request) (dbRowWriteRequest, bool) {
	var body dbRowWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return dbRowWriteRequest{}, false
	}
	return body, true
}

func (h *AdminHandler) CreateDBRow(w http.ResponseWriter, r *http.Request) {
	if !h.dbAdminReady(w) {
		return
	}
	body, ok := decodeDBRowRequest(w, r)
	if !ok {
		return
	}
	n, err := h.dbAdmin.InsertRow(r.Context(), chi.URLParam(r, "table"), body.Values)
	if err != nil {
		writeDBAdminError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"affected": n}})
}

func (h *AdminHandler) UpdateDBRow(w http.ResponseWriter, r *http.Request) {
	if !h.dbAdminReady(w) {
		return
	}
	body, ok := decodeDBRowRequest(w, r)
	if !ok {
		return
	}
	n, err := h.dbAdmin.UpdateRow(r.Context(), chi.URLParam(r, "table"), body.Key, body.Values)
	if err != nil {
		writeDBAdminError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"affected": n}})
}

func (h *AdminHandler) DeleteDBRow(w http.ResponseWriter, r *http.Request) {
	if !h.dbAdminReady(w) {
		return
	}
	body, ok := decodeDBRowRequest(w, r)
	if !ok {
		return
	}
	n, err := h.dbAdmin.DeleteRow(r.Context(), chi.URLParam(r, "table"), body.Key)
	if err != nil {
		writeDBAdminError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"affected": n}})
}
