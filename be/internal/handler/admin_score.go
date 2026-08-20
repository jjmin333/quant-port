package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/pkg/response"
)

// Admin endpoints for 점수 관리 (/admin/score).
//
// Weights are relative and always renormalized over the axes that have data,
// so the API deliberately does NOT force them to sum to 100 — it only clamps
// each axis to 0..100 and rejects an all-zero set (which would disable scoring
// entirely).

type scoreSettingsPayload struct {
	Weights       model.ScoreWeights `json:"weights"`
	CoverageFloor float64            `json:"coverageFloor"`
	ShrinkFull    float64            `json:"shrinkFull"`
}

// GetScoreSettings returns global weights plus every per-symbol override.
func (h *AdminHandler) GetScoreSettings(w http.ResponseWriter, r *http.Request) {
	if h.scoreSettings == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{
			"settings": model.DefaultScoreSettings(),
			"symbols":  []any{},
			"axes":     scoreAxisMeta(),
		}})
		return
	}
	ctx := r.Context()
	symbols, err := h.scoreSettings.List(ctx)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if symbols == nil {
		symbols = []model.SymbolScoreWeights{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"settings": h.scoreSettings.Get(ctx),
		"symbols":  symbols,
		"axes":     scoreAxisMeta(),
	}})
}

// scoreAxisMeta lets the admin UI render the weight editor from the server's
// canonical axis list rather than duplicating labels in the frontend.
func scoreAxisMeta() []map[string]string {
	out := make([]map[string]string, 0, len(model.ScoreAxes))
	for _, axis := range model.ScoreAxes {
		out = append(out, map[string]string{
			"id":    string(axis),
			"label": model.ScoreAxisLabels[axis],
		})
	}
	return out
}

// UpdateScoreSettings replaces the global default weights.
func (h *AdminHandler) UpdateScoreSettings(w http.ResponseWriter, r *http.Request) {
	if h.scoreSettings == nil {
		response.Error(w, http.StatusServiceUnavailable, "score settings unavailable")
		return
	}
	var payload scoreSettingsPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	if payload.Weights.Total() <= 0 {
		response.Error(w, http.StatusBadRequest, "가중치 합이 0보다 커야 합니다")
		return
	}
	updated, err := h.scoreSettings.Update(r.Context(), model.ScoreSettings{
		Weights:       payload.Weights,
		CoverageFloor: payload.CoverageFloor,
		ShrinkFull:    payload.ShrinkFull,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": updated})
}

type symbolWeightPayload struct {
	Symbol  string             `json:"symbol"`
	Weights model.ScoreWeights `json:"weights"`
	Note    string             `json:"note"`
}

// UpsertScoreSymbolWeights sets one symbol's weight override.
func (h *AdminHandler) UpsertScoreSymbolWeights(w http.ResponseWriter, r *http.Request) {
	if h.scoreSettings == nil {
		response.Error(w, http.StatusServiceUnavailable, "score settings unavailable")
		return
	}
	var payload symbolWeightPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	symbol := strings.ToUpper(strings.TrimSpace(payload.Symbol))
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol required")
		return
	}
	if payload.Weights.Total() <= 0 {
		response.Error(w, http.StatusBadRequest, "가중치 합이 0보다 커야 합니다")
		return
	}
	err := h.scoreSettings.UpsertSymbol(r.Context(), model.SymbolScoreWeights{
		Symbol:  symbol,
		Weights: payload.Weights,
		Note:    strings.TrimSpace(payload.Note),
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"symbol": symbol}})
}

// DeleteScoreSymbolWeights drops an override so the symbol follows the global
// weights again.
func (h *AdminHandler) DeleteScoreSymbolWeights(w http.ResponseWriter, r *http.Request) {
	if h.scoreSettings == nil {
		response.Error(w, http.StatusServiceUnavailable, "score settings unavailable")
		return
	}
	symbol := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("symbol")))
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol required")
		return
	}
	if err := h.scoreSettings.DeleteSymbol(r.Context(), symbol); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"symbol": symbol}})
}

// RecomputeScores runs the batch on demand so an admin can see the effect of a
// weight change without waiting for the next scheduled run.
//
// Synchronous by design: the batch makes zero upstream API calls and takes a
// few seconds, and the caller wants the resulting distribution back to judge
// whether the new weights produced a sane shape.
func (h *AdminHandler) RecomputeScores(w http.ResponseWriter, r *http.Request) {
	if h.scores == nil {
		response.Error(w, http.StatusServiceUnavailable, "score service unavailable")
		return
	}
	ctx := r.Context()
	tradeDate := strings.TrimSpace(r.URL.Query().Get("tradeDate"))
	if tradeDate == "" {
		latest, err := h.scores.LatestTradeDate(ctx)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		tradeDate = latest
	}
	dist, err := h.scores.Recompute(ctx, tradeDate)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"tradeDate":    tradeDate,
		"distribution": dist,
	}})
}

// ScoreStatus reports the latest batch's coverage for the admin dashboard.
func (h *AdminHandler) ScoreStatus(w http.ResponseWriter, r *http.Request) {
	if h.scores == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{}})
		return
	}
	ctx := r.Context()
	tradeDate, err := h.scores.LatestScoredDate(ctx)
	if err != nil || tradeDate == "" {
		response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"tradeDate": ""}})
		return
	}
	total, scored, err := h.scores.Stats(ctx, tradeDate)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"tradeDate": tradeDate,
		"total":     total,
		"scored":    scored,
	}})
}
