package handler

import (
	"net/http"
	"regexp"

	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/pkg/response"
)

/*
DailyTopHandler — 날짜별 「AI 점수 상위」 보관 (2026-10-21 티켓 QP-12, 참고: Danelfin 「오늘의 상위 점수」).

백테스트용으로 이미 매일 쌓는 `score_ranking_history`(종합 축, 깊이 200)에서 상위 20만 읽는다.
새로 저장하는 것은 없다 — 화면 `/rankings/YYYY-MM-DD`가 롱테일 검색어(「10월 20일 AI 점수 상위 종목」)의
착지점이 되고, 직전 보관일 대비 순위 변화를 함께 보여 준다.
*/
type DailyTopHandler struct {
	repo *repository.ScoreRankingHistoryRepo
}

func NewDailyTopHandler(repo *repository.ScoreRankingHistoryRepo) *DailyTopHandler {
	return &DailyTopHandler{repo: repo}
}

const dailyTopDepth = 20

var dailyTopDateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// Get serves GET /stocks/scores/daily-top?date=YYYY-MM-DD (비우면 최신).
func (h *DailyTopHandler) Get(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date != "" && !dailyTopDateRE.MatchString(date) {
		response.Error(w, http.StatusBadRequest, "date must be YYYY-MM-DD")
		return
	}
	ctx := r.Context()
	// 국내 종합 축의 날짜가 기준이다 — 두 시장이 같은 배치에서 같이 쌓인다.
	dates, err := h.repo.Dates(ctx, "domestic", "composite")
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "ranking history unavailable")
		return
	}
	idx := -1
	for i, d := range dates {
		if d == date || (date == "" && i == 0) {
			idx = i
			break
		}
	}
	if idx < 0 {
		response.Error(w, http.StatusNotFound, "no ranking for that date")
		return
	}
	date = dates[idx]
	prev, next := "", ""
	if idx+1 < len(dates) {
		prev = dates[idx+1]
	}
	if idx > 0 {
		next = dates[idx-1]
	}
	out := map[string]any{"date": date, "prevDate": prev, "nextDate": next, "dates": dates}
	for _, market := range []string{"domestic", "overseas"} {
		rows, err := h.repo.TopOn(ctx, market, "composite", date, prev, dailyTopDepth)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "ranking history unavailable")
			return
		}
		if rows == nil {
			rows = []repository.DailyTopRow{}
		}
		out[market] = rows
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}
