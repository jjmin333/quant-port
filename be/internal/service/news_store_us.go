package service

import (
	"context"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// finnhubUSCategories — 한 회차에 **하나씩** 돈다. 아래 머리글 참조.
var finnhubUSCategories = []string{"general", "merger", "forex", "crypto"}

/*
refreshFinnhubUSGeneral — 해외 일반 뉴스 한 갈래.

# 네 갈래를 한 회차에 돌리다 셋을 잃고 있었다 (2026-09-11 사용자 요청 7)

예전에는 이 함수가 네 카테고리를 **한 회차에 차례로** 불렀다. 그런데 이 티커의 주기는
2분(`newsPollInterval`)이고 `finnhub_news`의 최소 간격도 **2분**이다
(`apibudget` providers). 그래서 매 회차 이렇게 됐다:

	general  → 성공 (지난 회차로부터 2분 지났다)
	merger   → 실패 "finnhub_news min interval 2m0s not elapsed"
	forex    → 실패 (같은 이유)
	crypto   → 실패 (같은 이유)

즉 **merger·forex·crypto는 한 번도 수집되지 않았고**, 남은 것은 회차마다 찍히는
똑같은 로그 세 줄이었다(실측: 같은 초에 3~4줄, 6시간에 11회). 로그가 시끄러운 것이
증상이고 병은 「카테고리 셋이 죽어 있다」였다.

`apibudget.MinInterval`의 머리글이 같은 함정을 이미 적어 두었다 — 「Check는 '지금
부를 수 있나'만 답하므로, 여러 번 불러야 하는 배치는 첫 호출 뒤에 반드시 실패한다 —
국내 실적 백필이 15개월 중 14개월을 그렇게 잃었다.」 같은 실패가 여기 있었다.

# 고친 방식: 회차마다 한 갈래 (로테이션)

한 회차에 한 카테고리만 부른다. 네 갈래가 2분 × 4 = **8분에 한 바퀴**를 돌고,
호출 수는 예전과 **똑같다**(회차당 1회 — 예전에도 실제로 성공한 것은 1회였다).
달라지는 것은 그 한 번이 언제나 general 이 아니라는 점뿐이다.

기다리게 해서 넷을 다 부르는 방법은 택하지 않았다 — 최소 간격이 2분이라 한 회차가
6분을 붙잡고, 그 동안 다른 뉴스 소스의 티커가 밀린다.

커서는 메모리에만 둔다. 재기동하면 general 부터 다시 시작하는데, 그게 손해가 아니다 —
가장 넓은 갈래가 가장 먼저 온다.
*/
func (st *NewsStore) refreshFinnhubUSGeneral(ctx context.Context) {
	if st.finnhubNews == nil || !st.finnhubNews.Enabled() {
		return
	}
	st.rebuildUSNameIndex(ctx)

	st.mu.Lock()
	category := finnhubUSCategories[st.finnhubCategoryCursor%len(finnhubUSCategories)]
	st.finnhubCategoryCursor = (st.finnhubCategoryCursor + 1) % len(finnhubUSCategories)
	st.mu.Unlock()

	items, err := st.finnhubNews.MarketNews(ctx, category)
	if err != nil {
		st.logFetchErr("finnhub_us_general/"+category, err)
		return
	}
	var ingested []model.NewsArticle
	for _, item := range items {
		linked := strings.ToUpper(strings.TrimSpace(item.RelatedSymbol))
		article, ok := st.ingestItem(ctx, item, nil, linked, "us")
		if ok {
			ingested = append(ingested, article)
		}
	}
	st.mergeArticlesForMarket(ctx, ingested, "us")
}

/*
refreshFinnhubUSCompany — 관심·보유 종목의 해외 뉴스.

# 한 회차가 32종목을 부르는데 하나만 들어오고 있었다 (2026-09-13 사용자 요청 3)

이 회차는 큐에서 32종목을 꺼내 하나씩 `/company-news`를 부른다. 그런데 그 호출이
시장 전체 뉴스와 **같은 예산 버킷**(`finnhub_news`, 최소 간격 2분)에 있어서, 첫
종목이 성공하면 나머지 31개가 전부 그 간격에 걸렸다. 실측 로그가 그 모양이다 —
한 틱마다 `finnhub_news min interval 2m0s not elapsed` 가 31줄.

결과는 로그 잡음이 아니라 **수집 실패**였다: 큐 한 바퀴가 8분이 아니라 7시간이 되어
관심 종목 대부분이 하루에 서너 번밖에 갱신되지 않았다. 위 `refreshFinnhubUSGeneral`
머리글이 같은 병(「여러 번 불러야 하는 배치가 첫 호출 뒤에 반드시 실패한다」)을 이미
적어 두었는데, 그때는 시장 전체 뉴스 쪽만 고쳤다.

고친 방식은 로테이션이 아니라 **버킷 분리 + 스스로 간격 두기**다. 이쪽은 종목마다
답이 다르므로 「한 회차에 하나씩」으로는 큐 한 바퀴가 여전히 4시간이다. 전용 버킷
(`finnhub_news_company`, 2초)을 주고 호출부가 그 간격만큼 잠든다 — 32종목 × 2초 =
64초로 8분 주기 안에 끝난다.

간격을 **호출 사이에만** 둔다(첫 종목 앞에서는 안 잔다). 마지막 종목 뒤에 자면 그만큼
회차가 길어질 뿐 아무것도 지키지 못한다.
*/
func (st *NewsStore) refreshFinnhubUSCompany(ctx context.Context) {
	if st.finnhubNews == nil || !st.finnhubNews.Enabled() {
		return
	}
	st.rebuildUSNameIndex(ctx)
	symbols := st.nextUSSymbols(ctx, newsFinnhubCompanySymbolsPerTick)
	if len(symbols) == 0 {
		return
	}
	to := time.Now().UTC().Format("2006-01-02")
	from := time.Now().UTC().Add(-14 * 24 * time.Hour).Format("2006-01-02")
	pace := st.finnhubNews.CompanyNewsPace()

	var ingested []model.NewsArticle
	for i, kw := range symbols {
		if kw.Symbol == "" {
			continue
		}
		if i > 0 && pace > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(pace):
			}
		}
		items, err := st.finnhubNews.CompanyNews(ctx, kw.Symbol, from, to)
		if err != nil {
			st.logFetchErr("finnhub_us_company", err)
			continue
		}
		for _, item := range items {
			linked := kw.Symbol
			if item.RelatedSymbol != "" {
				linked = strings.ToUpper(strings.TrimSpace(item.RelatedSymbol))
			}
			article, ok := st.ingestItem(ctx, item, []string{kw.Text}, linked, "us")
			if ok {
				ingested = append(ingested, article)
			}
		}
	}
	st.mergeArticlesForMarket(ctx, ingested, "us")
}

func (st *NewsStore) refreshRSSUS(ctx context.Context) {
	if st.rssUS == nil {
		return
	}
	st.rebuildUSNameIndex(ctx)
	items, err := st.rssUS.FetchAll(ctx)
	if err != nil {
		st.logFetchErr("rss_us", err)
		return
	}
	var ingested []model.NewsArticle
	for _, item := range items {
		article, ok := st.ingestItem(ctx, item, nil, "", "us")
		if ok {
			ingested = append(ingested, article)
		}
	}
	st.mergeArticlesForMarket(ctx, ingested, "us")
}

func (st *NewsStore) loadUSFromStorage(ctx context.Context) {
	if snap, at := st.loadSnapshotFromRedis(ctx, "us"); len(snap) > 0 {
		st.mu.Lock()
		st.usArticles = snap
		st.usUpdatedAt = at
		st.mu.Unlock()
		return
	}
	if st.repo == nil {
		return
	}
	recs, err := st.repo.List(ctx, repository.NewsListParams{
		Market: "us",
		Sort:   "latest",
		Limit:  newsUSListLimit,
	})
	if err != nil || len(recs) == 0 {
		return
	}
	snap := st.recordsToArticles(ctx, recs)
	at, _ := st.repo.LatestUpdatedAt(ctx, "us")
	st.mu.Lock()
	st.usArticles = snap
	st.usUpdatedAt = at
	st.mu.Unlock()
	st.persistSnapshotRedis(ctx, snap, at, "us")
}
