package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type ScoreHandler struct {
	svc         *service.ScoreService
	indexScores *service.IndexScoreService
}

func NewScoreHandler(svc *service.ScoreService, indexScores *service.IndexScoreService) *ScoreHandler {
	return &ScoreHandler{svc: svc, indexScores: indexScores}
}

// Get serves GET /stocks/{symbol}/score — the latest stored score with its
// axis breakdown and the weights that were in force.
func (h *ScoreHandler) Get(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimSpace(chi.URLParam(r, "symbol"))
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol required")
		return
	}
	if h.svc == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": nil})
		return
	}
	view, err := h.svc.Get(r.Context(), symbol)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// A symbol with no batch row yet is a normal state (nothing scored it), not
	// an error — the UI renders "-".
	response.JSON(w, http.StatusOK, map[string]any{"data": view})
}

// GetMany serves GET /stocks/scores/batch?symbols=A,B,C — one request for a
// whole list's scores instead of one per row.
func (h *ScoreHandler) GetMany(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.URL.Query().Get("symbols"))
	if raw == "" {
		response.JSON(w, http.StatusOK, map[string]any{"data": []any{}})
		return
	}
	parts := strings.Split(raw, ",")
	symbols := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			symbols = append(symbols, p)
		}
	}
	// A cap keeps one crafted query from scanning the whole score table.
	const maxBatch = 200
	if len(symbols) > maxBatch {
		symbols = symbols[:maxBatch]
	}
	if h.svc == nil || len(symbols) == 0 {
		response.JSON(w, http.StatusOK, map[string]any{"data": []any{}})
		return
	}
	views, err := h.svc.GetMany(r.Context(), symbols)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if views == nil {
		views = []model.StockScoreView{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": views})
}

// History serves GET /stocks/{symbol}/score/history.
func (h *ScoreHandler) History(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimSpace(chi.URLParam(r, "symbol"))
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol required")
		return
	}
	if h.svc == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": []any{}})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := h.svc.History(r.Context(), symbol, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": rows})
}

// TopScores serves GET /stocks/scores/top?market=domestic|overseas&limit=N —
// the 종목 분석 랭킹 list, sorted by composite score for the latest scored date.
func (h *ScoreHandler) TopScores(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": []any{}})
		return
	}
	market := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("market")))
	if market != "overseas" {
		market = "domestic"
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	items, _, err := h.svc.TopScores(r.Context(), market, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": items})
}

// IndexScores serves GET /indices/scores — every index's latest score and
// rule-based read in one response, for the dashboard cards.
func (h *ScoreHandler) IndexScores(w http.ResponseWriter, r *http.Request) {
	if h.indexScores == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": []any{}})
		return
	}
	rows, err := h.indexScores.LatestAll(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rows == nil {
		rows = []service.IndexScoreSummary{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": rows})
}
