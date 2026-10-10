package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// ScoreCustomHandler serves AI 랭킹의 「내 기준」 — re-weighted ranking and saved weights.
type ScoreCustomHandler struct {
	svc  *service.CustomRankingService
	repo *repository.UserScoreWeightsRepo
}

func NewScoreCustomHandler(svc *service.CustomRankingService, repo *repository.UserScoreWeightsRepo) *ScoreCustomHandler {
	return &ScoreCustomHandler{svc: svc, repo: repo}
}

/*
Ranking serves GET /stocks/scores/custom-ranking?market=domestic&w=30,25,20,15,10&limit=50.

가중치를 쿼리로 받는다 — 로그인하지 않아도 슬라이더를 움직여 볼 수 있어야 한다. 순서는
추세·수익성·성장·가치·심리수급(model.ScoreAxes). 형식이 틀리면 400이다 — 조용히 기본값으로
바꾸면 화면은 사용자가 고른 가중치로 나온 줄 안다.
*/
func (h *ScoreCustomHandler) Ranking(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	market := strings.TrimSpace(q.Get("market"))
	if market == "" {
		market = "domestic"
	}
	weights, err := parseWeights(q.Get("w"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	out, err := h.svc.CustomRanking(r.Context(), market, weights, limit)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// GetWeights serves GET /user/score-weights (saved or the global defaults).
func (h *ScoreCustomHandler) GetWeights(w http.ResponseWriter, r *http.Request) {
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	weights, saved, err := h.repo.Get(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !saved {
		weights = model.DefaultScoreWeights()
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"weights": weights, "saved": saved}})
}

// PutWeights serves PUT /user/score-weights.
func (h *ScoreCustomHandler) PutWeights(w http.ResponseWriter, r *http.Request) {
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	var body model.ScoreWeights
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := validateWeights(body); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.repo.Put(r.Context(), userID, body); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"weights": body, "saved": true}})
}

func parseWeights(raw string) (model.ScoreWeights, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return model.DefaultScoreWeights(), nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) != len(model.ScoreAxes) {
		return model.ScoreWeights{}, errBadWeights
	}
	var out model.ScoreWeights
	for i, axis := range model.ScoreAxes {
		v, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err != nil {
			return model.ScoreWeights{}, errBadWeights
		}
		out.Set(axis, v)
	}
	return out, validateWeights(out)
}

type weightsError string

func (e weightsError) Error() string { return string(e) }

const errBadWeights = weightsError("w must be five integers 0..100 (추세,수익성,성장,가치,심리수급) with a positive sum")

// validateWeights rejects out-of-range or all-zero weights — 쓰기 경로는 관용보다 거부.
func validateWeights(w model.ScoreWeights) error {
	sum := 0
	for _, axis := range model.ScoreAxes {
		v := w.Get(axis)
		if v < 0 || v > 100 {
			return errBadWeights
		}
		sum += v
	}
	if sum <= 0 {
		return errBadWeights
	}
	return nil
}
