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
	rankings    *service.ScoreRankingService
}

func NewScoreHandler(
	svc *service.ScoreService,
	indexScores *service.IndexScoreService,
	rankings *service.ScoreRankingService,
) *ScoreHandler {
	return &ScoreHandler{svc: svc, indexScores: indexScores, rankings: rankings}
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

/*
TopScores serves GET /stocks/scores/top —
`?market=domestic|overseas&axis=composite|technical|…&duration=realtime|1d|1w|1mo&limit=N`.

# 두 갈래

`axis=composite&duration=realtime`(기본값)은 예전 그대로 `ScoreService.TopScores`가
그 자리에서 계산한다. 최신 점수일 한 날의 종합 점수 순이라 인덱스가 정확히 맞고,
실측 0.07초로 요청 경로에 두어도 되는 유일한 조합이다.

나머지 조합은 **배치가 미리 만들어 둔 목록**을 읽는다(`ScoreRankingService`). 기간
창을 훑는 계산은 운영 DB에서 수 초라 여기서 할 수 없다.

배치가 아직 그 조합을 만들지 않았으면(설치 직후, 또는 축을 새로 추가한 뒤 첫 배치
전) 목록 대신 **빈 배열**을 준다 — 오류가 아니라 「아직 없음」이고, 화면은 그 상태를
이미 그릴 줄 안다.
*/
func (h *ScoreHandler) TopScores(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": []any{}})
		return
	}
	q := r.URL.Query()
	market := strings.ToLower(strings.TrimSpace(q.Get("market")))
	if market != "overseas" {
		market = "domestic"
	}
	axis := model.ParseScoreRankingAxis(strings.TrimSpace(q.Get("axis")))
	duration := model.ParseScoreRankingDuration(strings.TrimSpace(q.Get("duration")))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 20
	}

	if axis == model.RankAxisComposite && duration == model.RankDurationRealtime {
		items, _, err := h.svc.TopScores(r.Context(), market, limit)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		// 종합·최신일에서는 순위 값이 곧 종합 점수다 — 화면이 한 자리만 그리면
		// 되도록 여기서도 같은 규약으로 채워 보낸다.
		for i := range items {
			score := items[i].Score
			items[i].RankValue = &score
		}
		response.JSON(w, http.StatusOK, map[string]any{"data": items})
		return
	}

	if h.rankings == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": []any{}})
		return
	}
	view, err := h.rankings.Get(r.Context(), market, axis, duration, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if view == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": []any{}})
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": view.Items})
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
