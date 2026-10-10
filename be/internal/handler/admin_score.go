package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
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

/*
ScoreExplainWeights serves GET /admin/score/explain-weights —
`?symbol=005930&technical=20&quality=25&growth=20&valuation=25&sentiment=10`.

저장 **전에** 결과를 보는 자리다. 가중치를 눌러 저장한 뒤 배치로 확인하면 17,906종목이
한 번 더 계산되고, 되돌리려면 또 한 번이다. 이 입구는 **저장된 한 행**을 다른 가중치로
다시 합산할 뿐이라 쓰기도 없고 배치도 돌지 않는다.

빠진 축은 현재 설정값을 그대로 쓴다 — 한 축만 바꿔 보는 것이 가장 흔한 쓰임이다.
*/
func (h *AdminHandler) ScoreExplainWeights(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol required")
		return
	}
	if h.scores == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": nil})
		return
	}

	base := model.DefaultScoreSettings().Weights
	if h.scoreSettings != nil {
		base = h.scoreSettings.Get(r.Context()).Weights
	}
	q := r.URL.Query()
	for _, axis := range model.ScoreAxes {
		raw := strings.TrimSpace(q.Get(string(axis)))
		if raw == "" {
			continue
		}
		v, err := strconv.Atoi(raw)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "invalid weight for "+string(axis))
			return
		}
		base.Set(axis, v)
	}

	out, err := h.scores.ExplainWeights(r.Context(), symbol, base)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

/*
ScoreRankingHistoryStatus serves GET /admin/score/ranking-history — 백테스트가
읽을 과거가 얼마나 쌓였나.

화면이 「표본 N거래일」을 말하려면 그 N을 어디선가 알아야 한다. 백테스트 응답도
같은 값을 담지만, 이 입구는 **아직 아무 백테스트도 돌리기 전에** 물을 수 있다.
*/
func (h *AdminHandler) ScoreRankingHistoryStatus(w http.ResponseWriter, r *http.Request) {
	if h.rankingHistory == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": nil})
		return
	}
	out, err := h.rankingHistory.Status(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

/*
BackfillScoreRankingHistory serves POST /admin/score/ranking-history/backfill.

평소에는 점수 배치가 스냅샷과 백필을 함께 부른다. 이 입구가 따로 있는 이유는 **점수를
손으로 재계산한 직후**다 — 그때 순위가 바뀌는데 배치는 다음 날에야 돈다. 그 사이의
백테스트는 화면과 다른 목록을 본다.

이미 채워진 날짜는 건너뛰므로 여러 번 눌러도 안전하다.
*/
func (h *AdminHandler) BackfillScoreRankingHistory(w http.ResponseWriter, r *http.Request) {
	if h.rankingHistory == nil {
		response.Error(w, http.StatusServiceUnavailable, "ranking history service unavailable")
		return
	}
	ctx := r.Context()

	// 최신 회차를 **먼저** 다시 쓴다 — 재계산으로 바뀐 순위가 반영되는 자리다.
	if err := h.rankingHistory.SnapshotLatest(ctx); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	filled, err := h.rankingHistory.Backfill(ctx)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	status, err := h.rankingHistory.Status(ctx)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"filled": filled,
		"status": status,
	}})
}
