package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// SymbolContextHandler serves 「이 종목에 대한 내 기록」 (V2 요구 9).
type SymbolContextHandler struct {
	svc *service.SymbolContextService
}

func NewSymbolContextHandler(svc *service.SymbolContextService) *SymbolContextHandler {
	return &SymbolContextHandler{svc: svc}
}

/*
Get serves GET /user/symbols/{symbol}/context.

로그인이 필요하다 — 이 응답은 전부 「내」 기록이다. 비로그인에게는 화면이 줄 자체를
그리지 않으므로 여기서도 401이 맞다.
*/
func (h *SymbolContextHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": nil})
		return
	}
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	out, err := h.svc.Get(r.Context(), userID, strings.TrimSpace(chi.URLParam(r, "symbol")))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}
