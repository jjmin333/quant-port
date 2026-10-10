package handler

import (
	"net/http"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// HoldingsDiagnosisHandler serves 포트폴리오 진단.
type HoldingsDiagnosisHandler struct {
	svc *service.HoldingsDiagnosisService
}

func NewHoldingsDiagnosisHandler(svc *service.HoldingsDiagnosisService) *HoldingsDiagnosisHandler {
	return &HoldingsDiagnosisHandler{svc: svc}
}

/*
Get serves GET /user/holdings/diagnosis — 「보유 이유가 아직 유효한가」.

보유 종목이 몇이든 **조회 다섯 번**이다(매매 이력 · 점수 · 실적 · 내부자 · 종가).
가장 이른 매수일 하나로 전부 긁고 보유 기간별로 자르는 일은 Go가 한다 —
「항목당 I/O 금지」(PRINCIPLES 응답 속도 원칙 ①).

보유가 없으면 빈 목록이다. 오류가 아니라 「아직 담은 것이 없다」이고, 화면은 그 상태를
이미 그릴 줄 안다.
*/
func (h *HoldingsDiagnosisHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": nil})
		return
	}
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	out, err := h.svc.Diagnose(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}
