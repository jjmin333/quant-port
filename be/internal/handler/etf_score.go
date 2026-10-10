package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/pkg/response"
)

// ETFScoreHandler serves 국내 ETF 점검 (etf_scores). 계산은 worker-filings가 한다 — 여기는 읽기만.
type ETFScoreHandler struct {
	repo *repository.ETFMetricsRepo
}

func NewETFScoreHandler(repo *repository.ETFMetricsRepo) *ETFScoreHandler {
	return &ETFScoreHandler{repo: repo}
}

// Get serves GET /stocks/{symbol}/etf-check — 없으면 data: null(국내 ETF가 아니거나 표본 부족).
func (h *ETFScoreHandler) Get(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimSpace(chi.URLParam(r, "symbol"))
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol required")
		return
	}
	row, err := h.repo.LatestScore(r.Context(), symbol)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": row})
}

// Top serves GET /etf/scores/top?group=plain|lowvol|leveraged&limit=50 — AI 랭킹의 「국내 ETF」 탭.
func (h *ETFScoreHandler) Top(w http.ResponseWriter, r *http.Request) {
	group := strings.TrimSpace(r.URL.Query().Get("group"))
	if group != "" && group != "plain" && group != "lowvol" && group != "leveraged" {
		response.Error(w, http.StatusBadRequest, "group must be plain, lowvol or leveraged")
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 200 {
			response.Error(w, http.StatusBadRequest, "limit must be 1..200")
			return
		}
		limit = n
	}
	rows, err := h.repo.TopScores(r.Context(), group, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	date := ""
	if len(rows) > 0 {
		date = rows[0].TradeDate
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"tradeDate": date, "items": rows}})
}
