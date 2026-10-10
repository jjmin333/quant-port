package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// ExternalSignalHandler serves 종목 화면의 애널리스트·레딧 (2026-09-28). 저장분만 읽는다.
type ExternalSignalHandler struct {
	repo *repository.ExternalSignalRepo
}

func NewExternalSignalHandler(repo *repository.ExternalSignalRepo) *ExternalSignalHandler {
	return &ExternalSignalHandler{repo: repo}
}

// socialPostsShown — 커뮤니티 탭이 넘겨 보는 글 수(적재는 종목당 20).
const socialPostsShown = 20

// Get serves GET /stocks/{symbol}/signals — {analyst, social[최근 30일, 최신 먼저], subreddits,
// reddit{insight, posts, community}}.
func (h *ExternalSignalHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	symbol := strings.ToUpper(strings.TrimSpace(chi.URLParam(r, "symbol")))
	analyst, err := h.repo.Consensus(ctx, symbol)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	social, err := h.repo.SocialHistory(ctx, symbol, 30)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	subs, err := h.repo.SocialBySubreddit(ctx, symbol)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	data := map[string]any{"analyst": analyst, "social": social, "subreddits": subs}
	if len(social) > 0 {
		reddit, err := h.reddit(r, symbol)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		data["reddit"] = reddit
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": data})
}

// 애널리스트 화면이 받는 이력 시점 수(해외 월 1줄 → 5년, 국내 변경일 기준)와 리포트 수.
const (
	analystHistoryShown = 60
	analystReportsShown = 10
)

// Analyst serves GET /stocks/{symbol}/analyst — {consensus, history[오래된 것 먼저], reports[최신 먼저]}.
// 개요 「애널리스트 의견」 카드 전용(2026-10-07). signals와 달리 레딧 표를 건드리지 않는다.
func (h *ExternalSignalHandler) Analyst(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	symbol := strings.ToUpper(strings.TrimSpace(chi.URLParam(r, "symbol")))
	consensus, err := h.repo.Consensus(ctx, symbol)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	history, err := h.repo.AnalystHistory(ctx, symbol, analystHistoryShown)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	reports, err := h.repo.AnalystReports(ctx, symbol, analystReportsShown)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"consensus": consensus, "history": history, "reports": reports,
	}})
}

/*
reddit — 커뮤니티 탭(2026-09-29): ApeWisdom 종목 요약 · 이번 주 인기 글 · 커뮤니티 점수와 AI 요약.

점수는 AI 요약 행이 있으면 그 행의 값(글과 같은 날 계산), 없으면 최신 요약으로 바로 계산한다
(요약 배치는 상위 40종목만 돈다).
*/
func (h *ExternalSignalHandler) reddit(r *http.Request, symbol string) (map[string]any, error) {
	ctx := r.Context()
	insight, err := h.repo.SocialInsightLatest(ctx, symbol)
	if err != nil {
		return nil, err
	}
	posts, err := h.repo.SocialPosts(ctx, symbol, socialPostsShown)
	if err != nil {
		return nil, err
	}
	summary, err := h.repo.SocialSummaryOf(ctx, symbol)
	if err != nil {
		return nil, err
	}
	var community map[string]any
	switch {
	case summary != nil && (insight == nil || summary.SnapDate >= insight.SnapDate):
		community = map[string]any{"score": summary.Score, "stance": service.CommunityStance(summary.Score),
			"date": summary.SnapDate, "source": summary.Source, "body": summary.Body}
	case insight != nil:
		if sc, ok := service.CommunityScore(insight.SentimentPct, insight.Users); ok {
			community = map[string]any{"score": sc, "stance": service.CommunityStance(sc), "date": insight.SnapDate}
			// 요약이 하루 이틀 묵었으면 글은 그대로 보여 준다 — 점수만 오늘 것.
			if summary != nil {
				community["source"], community["body"], community["bodyDate"] = summary.Source, summary.Body, summary.SnapDate
			}
		}
	}
	return map[string]any{"insight": insight, "posts": posts, "community": community}, nil
}

// MarketMood serves GET /social/market-mood — 홈 「미국 개미 분위기」 카드(2026-10-01 요청 3). 저장분만 읽는다.
func (h *ExternalSignalHandler) MarketMood(w http.ResponseWriter, r *http.Request) {
	m, err := service.BuildMarketMood(r.Context(), h.repo)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": m})
}
