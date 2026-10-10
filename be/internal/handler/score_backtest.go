package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// ScoreBacktestHandler serves the AI 랭킹 백테스트.
type ScoreBacktestHandler struct {
	svc *service.ScoreBacktestService
}

func NewScoreBacktestHandler(svc *service.ScoreBacktestService) *ScoreBacktestHandler {
	return &ScoreBacktestHandler{svc: svc}
}

/*
Run serves GET /stocks/scores/backtest —
`?market=domestic|overseas&axis=composite|…&topN=20&rebalance=daily|weekly|monthly
 &from=YYYY-MM-DD&to=YYYY-MM-DD&costBps=0`.

# 왜 조회 경로에 두는가

무거워 보이지만 읽기는 두 번이다 — 얇은 랭킹 이력 한 번, 종가 한 번. 나머지는 메모리
산수라 실측이 요청당 수백 밀리초 안이다. 사전 계산으로 옮기면 사용자가 고를 수 있는
조합(시장 2 × 축 6 × 상위 N × 주기 3 × 창)이 곱셈으로 늘어나고, 그 대부분은 아무도
열지 않는다.

# 이력이 없으면 오류가 아니다

이 기능이 켜진 첫날, 또는 요청한 창에 스냅샷이 없는 경우 `data: null`이다. 화면은
「아직 쌓이지 않았습니다」를 그린다 — 0%짜리 곡선을 그리는 것과 다른 답이다.
*/
func (h *ScoreBacktestHandler) Run(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": nil})
		return
	}
	q := r.URL.Query()

	market := strings.ToLower(strings.TrimSpace(q.Get("market")))
	if market != "overseas" {
		market = "domestic"
	}
	topN, _ := strconv.Atoi(q.Get("topN"))
	costBps, _ := strconv.ParseFloat(q.Get("costBps"), 64)

	req := model.BacktestRequest{
		Market:    market,
		Axis:      model.ParseScoreRankingAxis(strings.TrimSpace(q.Get("axis"))),
		TopN:      topN,
		Rebalance: model.ParseBacktestRebalance(strings.TrimSpace(q.Get("rebalance"))),
		FromDate:  strings.TrimSpace(q.Get("from")),
		ToDate:    strings.TrimSpace(q.Get("to")),
		CostBps:   costBps,
		// `?benchmarks=kospi,kosdaq`. 비우면 시장별 기본값 — 화이트리스트를 지나지
		// 못한 id는 조용히 버려진다(`normalizeBacktestRequest`).
		Benchmarks: splitCSV(q.Get("benchmarks")),
	}

	out, err := h.svc.Run(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// splitCSV trims and drops empties — `?benchmarks=kospi,,kosdaq` 같은 입력이
// 빈 문자열을 화이트리스트에 던지지 않도록.
func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

/*
Benchmarks serves GET /stocks/scores/backtest/benchmarks — 시장별로 고를 수 있는
지수 목록.

화면이 한국어 이름을 하드코딩하지 않게 하는 입구다. 카탈로그가 늘면 화면은 그대로 둔 채
목록만 길어진다 — 그리고 **여기 없는 지수는 백테스트가 받지 않는다**(같은 화이트리스트).
*/
func (h *ScoreBacktestHandler) Benchmarks(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"catalog":  model.BacktestBenchmarkCatalog,
		"defaults": model.DefaultBacktestBenchmarks,
	}})
}
