package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/pkg/response"
)

// HoldingThesisHandler saves a holding's 매수 이유 (살 때 본 축).
type HoldingThesisHandler struct {
	repo *repository.HoldingThesisRepo
}

func NewHoldingThesisHandler(repo *repository.HoldingThesisRepo) *HoldingThesisHandler {
	return &HoldingThesisHandler{repo: repo}
}

// maxThesisAxes — 다섯 축을 다 고르면 그건 이유가 아니라 종합 점수다.
const maxThesisAxes = 3

/*
Put serves PUT /user/holdings/{symbol}/thesis  {"axes": ["growth", "valuation"]}.

빈 배열은 지운다. 모르는 축·중복·넷 이상은 400이다 — 조용히 걸러 저장하면 사용자가 고른
것과 다른 이유로 점검이 돈다(쓰기 경로는 관용보다 거부).
*/
func (h *HoldingThesisHandler) Put(w http.ResponseWriter, r *http.Request) {
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	symbol := strings.TrimSpace(chi.URLParam(r, "symbol"))
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol required")
		return
	}
	var body struct {
		Axes []string `json:"axes"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	if len(body.Axes) > maxThesisAxes {
		response.Error(w, http.StatusBadRequest, "axes: at most 3")
		return
	}
	valid := map[string]bool{}
	for _, a := range model.ScoreAxes {
		valid[string(a)] = true
	}
	seen := map[string]bool{}
	for _, a := range body.Axes {
		if !valid[a] || seen[a] {
			response.Error(w, http.StatusBadRequest, "axes: unknown or duplicate axis "+a)
			return
		}
		seen[a] = true
	}
	if err := h.repo.Set(r.Context(), userID, symbol, body.Axes); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"symbol": strings.ToUpper(symbol), "axes": body.Axes}})
}
