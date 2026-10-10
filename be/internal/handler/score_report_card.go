package handler

import (
	"net/http"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// ScoreReportCardHandler serves the AI 점수 성적표.
type ScoreReportCardHandler struct {
	svc      *service.ScoreReportCardService
	settings *service.ScoreSettingsStore
}

// WithSettings lets the admin weight suggestion read the current weights.
func (h *ScoreReportCardHandler) WithSettings(st *service.ScoreSettingsStore) *ScoreReportCardHandler {
	h.settings = st
	return h
}

func NewScoreReportCardHandler(svc *service.ScoreReportCardService) *ScoreReportCardHandler {
	return &ScoreReportCardHandler{svc: svc}
}

/*
Get serves GET /stocks/scores/report-card?market=domestic|us — 저장된 날짜별 IC의 요약.

계산은 점수 배치 꼬리가 한다(`ScoreReportCardService.Compute`). 여기는 읽기만 한다.
*/
func (h *ScoreReportCardHandler) Get(w http.ResponseWriter, r *http.Request) {
	market := strings.TrimSpace(r.URL.Query().Get("market"))
	if market == "" {
		market = "domestic"
	}
	if market != "domestic" && market != "us" {
		response.Error(w, http.StatusBadRequest, "market must be domestic or us")
		return
	}
	out, err := h.svc.Report(r.Context(), market)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

/*
WeightSuggestion serves GET /admin/score/weight-suggestion?market=domestic|us — 성적표 기반
가중치 **제안**(저장 없음). 규칙은 service/score_weight_suggestion.go 머리글.
*/
func (h *ScoreReportCardHandler) WeightSuggestion(w http.ResponseWriter, r *http.Request) {
	market := strings.TrimSpace(r.URL.Query().Get("market"))
	if market == "" {
		market = "domestic"
	}
	if market != "domestic" && market != "us" {
		response.Error(w, http.StatusBadRequest, "market must be domestic or us")
		return
	}
	current := model.DefaultScoreWeights()
	if h.settings != nil {
		current = h.settings.Get(r.Context()).Weights
	}
	out, err := h.svc.SuggestWeights(r.Context(), market, current)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}
