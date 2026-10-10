package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// PriceScenarioHandler serves 목표가 시나리오 (V2 요구 5).
type PriceScenarioHandler struct {
	svc *service.PriceScenarioService
}

func NewPriceScenarioHandler(svc *service.PriceScenarioService) *PriceScenarioHandler {
	return &PriceScenarioHandler{svc: svc}
}

/*
Get serves GET /stocks/{symbol}/scenario.

# 로그인이 필요 없다

시나리오는 종목의 성질이지 사용자의 것이 아니다. 종목 화면이 비로그인에게도 열리므로
이 카드도 같이 열린다.

# 이력이 모자라면 오류가 아니다

갓 상장한 종목은 `unavailable`을 달고 200으로 나간다. 화면은 「아직 이력이
모자랍니다」를 그린다 — 빈 밴드를 그리는 것과 다른 답이다 (요구 8).
*/
func (h *PriceScenarioHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": nil})
		return
	}
	symbol := strings.TrimSpace(chi.URLParam(r, "symbol"))
	out, err := h.svc.Scenario(r.Context(), symbol)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}
