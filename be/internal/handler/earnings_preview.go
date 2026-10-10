package handler

import (
	"net/http"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// EarningsPreviewHandler serves the stored 「실적 관전 포인트」 (LLM 0 on read).
type EarningsPreviewHandler struct {
	svc *service.EarningsPreviewService
}

func NewEarningsPreviewHandler(svc *service.EarningsPreviewService) *EarningsPreviewHandler {
	return &EarningsPreviewHandler{svc: svc}
}

// List — GET /earnings/previews?market=domestic|us : 향후 7일 발표 종목의 저장된 관전 포인트.
func (h *EarningsPreviewHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.List(r.Context(), r.URL.Query().Get("market"))
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": rows})
}

// Digest — GET /earnings/digest?market=domestic|us : 실적 화면 맨 위 AI 요약(저장분, 2026-10-09).
// 배치가 아직 안 돌았으면 data가 null이다 — 화면은 카드를 숨긴다.
func (h *EarningsPreviewHandler) Digest(w http.ResponseWriter, r *http.Request) {
	row, err := h.svc.Digest(r.Context(), r.URL.Query().Get("market"))
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": row})
}
