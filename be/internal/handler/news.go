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

type NewsHandler struct {
	store    *service.NewsStore
	reader   *service.NewsReaderService
	briefing *service.NewsBriefingService
}

func NewNewsHandler(
	store *service.NewsStore,
	reader *service.NewsReaderService,
	briefing *service.NewsBriefingService,
) *NewsHandler {
	return &NewsHandler{store: store, reader: reader, briefing: briefing}
}

// Briefing serves the stored 「AI 시장 브리핑」 for a market.
//
// This endpoint never generates. Generation costs an LLM call and belongs to
// the scheduler alone (see NewsBriefingService) — a read path that could
// generate would make the day's spend scale with traffic. When nothing has
// been generated yet the response carries a null payload and the card simply
// does not render.
func (h *NewsHandler) Briefing(w http.ResponseWriter, r *http.Request) {
	market := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "market")))
	if market != "domestic" && market != "us" {
		response.Error(w, http.StatusBadRequest, "market must be domestic or us")
		return
	}
	if h.briefing == nil {
		response.JSON(w, http.StatusOK, map[string]any{"data": nil})
		return
	}
	briefing, err := h.briefing.Latest(r.Context(), market)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": briefing})
}

/*
Dashboard serves GET /news/{market}/dashboard — 홈이 쓰는 만큼만 담은 목록.

홈은 기사 100건을 통째로 받아(98KB) 그중 **여덟 건만 화면에 그린다**. 나머지 92건은
「오늘의 키워드」 집계와 속보 개수에만 쓰이는데, 그 집계가 읽는 값은 제목·태그·
발행시각·키워드뿐이다. 본문 요약·이미지 URL·원문 URL·포털 URL은 한 번도 안 읽힌 채
회선을 지나갔다.

그래서 **앞 N건은 그대로, 나머지는 집계에 필요한 필드만** 남긴다. 배열 하나에
같은 타입으로 담기므로 화면 쪽 집계 코드는 손대지 않아도 된다 — 집계 규칙을 서버로
옮겨 적으면 두 언어에 같은 규칙이 두 벌 생기고, 언젠가 조용히 어긋난다.

`fullCount`는 "여기까지는 온전한 기사"라는 표시다. 화면이 모달을 열 때 쓸 수 있는
범위이기도 하다 — 잘려 나간 기사를 열면 본문도 링크도 없는 창이 뜬다.
*/
const (
	dashboardDefaultFull  = 8
	dashboardDefaultLimit = 100
	dashboardMaxFull      = 30
)

func (h *NewsHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	market := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "market")))
	if market != "domestic" && market != "us" {
		response.Error(w, http.StatusBadRequest, "market must be domestic or us")
		return
	}
	if h.store == nil {
		response.JSON(w, http.StatusOK, map[string]any{
			"data": []model.NewsArticle{},
			"meta": map[string]any{"count": 0, "fullCount": 0},
		})
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = dashboardDefaultLimit
	}
	full, _ := strconv.Atoi(q.Get("full"))
	if full <= 0 {
		full = dashboardDefaultFull
	}
	if full > dashboardMaxFull {
		full = dashboardMaxFull
	}

	articles, updatedAt := h.store.List(r.Context(), model.NewsListFilter{
		Market:   market,
		SourceID: "all",
		Sort:     "latest",
		Limit:    limit,
	})
	if full > len(articles) {
		full = len(articles)
	}

	out := make([]model.NewsArticle, len(articles))
	copy(out, articles)
	for i := full; i < len(out); i++ {
		out[i] = digestArticle(out[i])
	}

	meta := map[string]any{"count": len(out), "fullCount": full}
	if !updatedAt.IsZero() {
		meta["updatedAt"] = updatedAt
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out, "meta": meta})
}

/*
Keywords serves GET /news/{market}/keywords — 「오늘의 키워드」 집계.

화면이 기사를 받아 세지 않고 **답만 받는다**. 표본이 1,000건이라 목록을 넘기면
요약본만으로도 300KB가 넘고, 그 전에 `NewsStore.List`의 국내 200건 상한에 걸려
1,000건을 달라고 해도 200건이 왔다. 집계 규칙은 `service.NewsStore.TopKeywords` 참고.
*/
func (h *NewsHandler) Keywords(w http.ResponseWriter, r *http.Request) {
	market := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "market")))
	if market != "domestic" && market != "us" {
		response.Error(w, http.StatusBadRequest, "market must be domestic or us")
		return
	}
	if h.store == nil {
		response.JSON(w, http.StatusOK, map[string]any{
			"data": service.NewsTopKeywords{Keywords: nil},
		})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.store.TopKeywords(r.Context(), market, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

/*
Symbols serves GET /news/{market}/symbols — 「많이 언급된 종목」 집계.

`Keywords`와 같은 이유로 같은 모양이다. 화면이 목록을 세면 그 목록이 덮는 시간만
세게 되는데, 국내 최신 100건이 덮는 시간이 **34분이었다**(실측 2026-09-01).
집계 규칙은 `service.NewsStore.TopSymbols` 참고 — 응답의 종목명은 `stocks`에서
붙여 보내므로, 화면은 시세 배치를 기다리며 종목코드를 대신 그릴 일이 없다.
*/
func (h *NewsHandler) Symbols(w http.ResponseWriter, r *http.Request) {
	market := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "market")))
	if market != "domestic" && market != "us" {
		response.Error(w, http.StatusBadRequest, "market must be domestic or us")
		return
	}
	if h.store == nil {
		response.JSON(w, http.StatusOK, map[string]any{
			"data": service.NewsTopSymbols{Symbols: nil},
		})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.store.TopSymbols(r.Context(), market, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// digestArticle keeps only what the 키워드 집계와 속보 판정 read.
//
// 제목은 남긴다 — 속보 판정이 `[속보]` 표기를 제목에서 찾고, 「같은 이슈」 묶음도
// 제목으로 센다.
func digestArticle(a model.NewsArticle) model.NewsArticle {
	return model.NewsArticle{
		ID:          a.ID,
		Market:      a.Market,
		Headline:    a.Headline,
		HeadlineKo:  a.HeadlineKo,
		SourceID:    a.SourceID,
		Tags:        a.Tags,
		Symbols:     a.Symbols,
		Keywords:    a.Keywords,
		PublishedAt: a.PublishedAt,
	}
}

func (h *NewsHandler) ListUS(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		response.JSON(w, http.StatusOK, map[string]any{
			"data": []model.NewsArticle{},
			"meta": map[string]any{"count": 0, "totalCount": 0},
		})
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	filter := model.NewsListFilter{
		Market:   "us",
		SourceID: q.Get("source"),
		Sort:     q.Get("sort"),
		Query:    q.Get("q"),
		Symbol:   q.Get("symbol"),
		Limit:    limit,
	}
	if filter.Sort == "" {
		filter.Sort = "latest"
	}
	if filter.SourceID == "" {
		filter.SourceID = "all"
	}

	articles, updatedAt := h.store.List(r.Context(), filter)
	meta := map[string]any{
		"count":      len(articles),
		"totalCount": h.store.Count(r.Context(), "us"),
	}
	if !updatedAt.IsZero() {
		meta["updatedAt"] = updatedAt
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"data": articles,
		"meta": meta,
	})
}

func (h *NewsHandler) ListDomestic(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		response.JSON(w, http.StatusOK, map[string]any{
			"data": []model.NewsArticle{},
			"meta": map[string]any{"count": 0, "totalCount": 0},
		})
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	filter := model.NewsListFilter{
		Market:   "domestic",
		SourceID: q.Get("source"),
		Sort:     q.Get("sort"),
		Query:    q.Get("q"),
		Symbol:   q.Get("symbol"),
		Limit:    limit,
	}
	if filter.Sort == "" {
		filter.Sort = "latest"
	}
	if filter.SourceID == "" {
		filter.SourceID = "all"
	}

	articles, updatedAt := h.store.List(r.Context(), filter)
	meta := map[string]any{
		"count":      len(articles),
		"totalCount": h.store.Count(r.Context(), "domestic"),
	}
	if !updatedAt.IsZero() {
		meta["updatedAt"] = updatedAt
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"data": articles,
		"meta": meta,
	})
}

func (h *NewsHandler) UsDemo(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		response.Error(w, http.StatusServiceUnavailable, "news store not configured")
		return
	}
	article, ok := h.store.FirstRSSArticle(r.Context())
	if !ok {
		response.Error(w, http.StatusNotFound, "no rss articles available yet")
		return
	}
	tags := append([]string{}, article.Tags...)
	tags = appendUniqueTag(tags, "Reader API")
	tags = appendUniqueTag(tags, "RSS")
	article.Tags = tags
	response.JSON(w, http.StatusOK, map[string]any{
		"data": model.NewsUsDemoArticle{
			NewsArticle: article,
			ReaderMode:  true,
		},
	})
}

func appendUniqueTag(tags []string, tag string) []string {
	for _, t := range tags {
		if t == tag {
			return tags
		}
	}
	return append(tags, tag)
}

func (h *NewsHandler) Reader(w http.ResponseWriter, r *http.Request) {
	if h.reader == nil {
		response.Error(w, http.StatusServiceUnavailable, "reader not configured")
		return
	}
	rawURL := r.URL.Query().Get("url")
	market := r.URL.Query().Get("market")
	articleID := r.URL.Query().Get("articleId")
	content, err := h.reader.Extract(r.Context(), rawURL, service.ReaderExtractOptions{
		Market:    market,
		ArticleID: articleID,
	})
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": content})
}

func (h *NewsHandler) RecordView(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		response.JSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		response.Error(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := h.store.IncrementView(r.Context(), id); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
