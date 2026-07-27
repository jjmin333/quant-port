package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"net/http"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/redis/go-redis/v9"
)

const (
	newsPollInterval       = 2 * time.Minute
	newsKakaoPollInterval  = 20 * time.Minute
	newsNaverKeywordsPerTick = 18
	newsKakaoKeywordsPerTick = 4
	newsSnapshotKey        = "news:domestic:snapshot:v1"
	newsKeywordCursorKey   = "news:domestic:keyword_cursor:v1"
	newsSnapshotTTL        = 6 * time.Hour
)

var adminNewsKeywords = []string{
	"코스피", "코스닥", "증시", "반도체", "원달러", "환율",
	"한국은행", "기준금리", "금리", "트럼프", "엔비디아", "AI",
	"2차전지", "바이오", "자동차", "배당", "실적", "공매도",
	"외국인", "기관", "IPO", "ETF", "채권", "유가",
}

type newsKeyword struct {
	Text     string
	Priority int
	Symbol   string
}

type NewsStore struct {
	cfg       config.NewsConfig
	naver     *marketdata.NaverNewsClient
	kakao     *marketdata.KakaoNewsClient
	rss       *marketdata.RSSNewsClient
	repo      *repository.NewsRepo
	stocks    *repository.StockRepo
	watchlist *repository.WatchlistRepo
	portfolio *PortfolioService
	redis     *redis.Client
	embedHTTP *http.Client

	mu          sync.RWMutex
	articles    []model.NewsArticle
	updatedAt   time.Time
	keywordCursor int
	nameIndex   map[string]string // lowercase name -> symbol
}

func NewNewsStore(
	cfg config.NewsConfig,
	naver *marketdata.NaverNewsClient,
	kakao *marketdata.KakaoNewsClient,
	rss *marketdata.RSSNewsClient,
	repo *repository.NewsRepo,
	stocks *repository.StockRepo,
	watchlist *repository.WatchlistRepo,
	portfolio *PortfolioService,
	redisClient *redis.Client,
) *NewsStore {
	return &NewsStore{
		cfg:       cfg,
		naver:     naver,
		kakao:     kakao,
		rss:       rss,
		repo:      repo,
		stocks:    stocks,
		watchlist: watchlist,
		portfolio: portfolio,
		redis:     redisClient,
		embedHTTP: &http.Client{Timeout: 8 * time.Second},
		nameIndex: make(map[string]string),
	}
}

func (st *NewsStore) Run(ctx context.Context) {
	st.loadFromStorage(ctx)
	st.refreshRSS(ctx)
	st.refreshNaver(ctx)

	naverTicker := time.NewTicker(newsPollInterval)
	rssTicker := time.NewTicker(newsPollInterval)
	kakaoTicker := time.NewTicker(newsKakaoPollInterval)
	defer naverTicker.Stop()
	defer rssTicker.Stop()
	defer kakaoTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-naverTicker.C:
			st.refreshNaver(ctx)
		case <-rssTicker.C:
			st.refreshRSS(ctx)
		case <-kakaoTicker.C:
			st.refreshKakao(ctx)
		}
	}
}

func (st *NewsStore) List(ctx context.Context, filter model.NewsListFilter) ([]model.NewsArticle, time.Time) {
	if st.repo != nil {
		recs, err := st.repo.List(ctx, repository.NewsListParams{
			Market:   filter.Market,
			SourceID: filter.SourceID,
			Sort:     filter.Sort,
			Query:    filter.Query,
			Symbol:   filter.Symbol,
			Limit:    filter.Limit,
		})
		if err == nil {
			articles := st.recordsToArticles(ctx, recs)
			market := filter.Market
			if market == "" {
				market = "domestic"
			}
			at, _ := st.repo.LatestUpdatedAt(ctx, market)
			return filterArticles(articles, filterWithoutTextSearch(filter)), at
		}
	}

	st.mu.RLock()
	snap := st.articles
	at := st.updatedAt
	st.mu.RUnlock()

	if len(snap) == 0 {
		snap, at = st.loadSnapshotFromRedis(ctx)
	}
	if len(snap) == 0 && st.repo != nil {
		recs, err := st.repo.List(ctx, repository.NewsListParams{
			Market:   filter.Market,
			SourceID: filter.SourceID,
			Sort:     filter.Sort,
			Limit:    filter.Limit,
		})
		if err == nil {
			snap = st.recordsToArticles(ctx, recs)
			if at.IsZero() {
				at, _ = st.repo.LatestUpdatedAt(ctx, filter.Market)
			}
		}
	}

	return filterArticles(snap, filter), at
}

func filterWithoutTextSearch(filter model.NewsListFilter) model.NewsListFilter {
	out := filter
	out.Query = ""
	out.Symbol = ""
	return out
}

func (st *NewsStore) loadFromStorage(ctx context.Context) {
	if snap, at := st.loadSnapshotFromRedis(ctx); len(snap) > 0 {
		st.mu.Lock()
		st.articles = snap
		st.updatedAt = at
		st.mu.Unlock()
		return
	}
	if st.repo == nil {
		return
	}
	recs, err := st.repo.List(ctx, repository.NewsListParams{
		Market: "domestic",
		Sort:   "latest",
		Limit:  200,
	})
	if err != nil || len(recs) == 0 {
		return
	}
	snap := st.recordsToArticles(ctx, recs)
	at, _ := st.repo.LatestUpdatedAt(ctx, "domestic")
	st.mu.Lock()
	st.articles = snap
	st.updatedAt = at
	st.mu.Unlock()
	st.persistSnapshotRedis(ctx, snap, at)
}

func (st *NewsStore) recordsToArticles(ctx context.Context, recs []repository.NewsArticleRecord) []model.NewsArticle {
	out := make([]model.NewsArticle, 0, len(recs))
	for _, rec := range recs {
		publisher := marketdata.ResolvePublisher(rec.URL)
		out = append(out, model.NewsArticle{
			ID:            rec.ID,
			Market:        rec.Market,
			Headline:      rec.Headline,
			Summary:       rec.Summary,
			SourceID:      rec.SourceID,
			PublisherID:   publisher.ID,
			PublisherName: publisher.Name,
			Tags:          rec.Tags,
			Symbols:       rec.Symbols,
			PublishedAt:   rec.PublishedAt,
			ViewCount:     rec.ViewCount,
			URL:           rec.URL,
			EmbedPolicy:   string(marketdata.ResolveEmbedPolicy(ctx, st.embedHTTP, st.redis, rec.URL)),
			Highlighted:   rec.Highlighted,
		})
	}
	return out
}

// FirstRSSArticle returns the latest ingested RSS (or RSS-feed yonhap) article with a URL.
func (st *NewsStore) FirstRSSArticle(ctx context.Context) (model.NewsArticle, bool) {
	articles, _ := st.List(ctx, model.NewsListFilter{
		Market: "domestic",
		Sort:   "latest",
		Limit:  100,
	})
	var fallback model.NewsArticle
	var hasFallback bool
	for _, a := range articles {
		if strings.TrimSpace(a.URL) == "" {
			continue
		}
		if a.SourceID != "rss" && a.SourceID != "yonhap" {
			continue
		}
		if strings.Contains(a.URL, "hankyung.com") {
			return a, true
		}
		if !hasFallback {
			fallback = a
			hasFallback = true
		}
	}
	if hasFallback {
		return fallback, true
	}
	return model.NewsArticle{}, false
}

func filterArticles(items []model.NewsArticle, filter model.NewsListFilter) []model.NewsArticle {
	market := filter.Market
	if market == "" {
		market = "domestic"
	}
	out := make([]model.NewsArticle, 0, len(items))
	for _, a := range items {
		if a.Market != market {
			continue
		}
		if src := strings.TrimSpace(filter.SourceID); src != "" && src != "all" && a.SourceID != src {
			continue
		}
		if q := strings.TrimSpace(filter.Query); q != "" {
			qLower := strings.ToLower(q)
			if !strings.Contains(strings.ToLower(a.Headline), qLower) &&
				!strings.Contains(strings.ToLower(a.Summary), qLower) {
				continue
			}
		}
		out = append(out, a)
	}

	sortMode := filter.Sort
	if sortMode == "" {
		sortMode = "latest"
	}
	if sortMode == "popular" {
		sort.Slice(out, func(i, j int) bool {
			if out[i].ViewCount == out[j].ViewCount {
				return out[i].PublishedAt.After(out[j].PublishedAt)
			}
			return out[i].ViewCount > out[j].ViewCount
		})
	} else {
		sort.Slice(out, func(i, j int) bool {
			return out[i].PublishedAt.After(out[j].PublishedAt)
		})
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (st *NewsStore) persistSnapshotRedis(ctx context.Context, articles []model.NewsArticle, at time.Time) {
	if st.redis == nil {
		return
	}
	payload, err := json.Marshal(struct {
		Articles  []model.NewsArticle `json:"articles"`
		UpdatedAt time.Time           `json:"updatedAt"`
	}{Articles: articles, UpdatedAt: at})
	if err != nil {
		return
	}
	_ = st.redis.Set(ctx, newsSnapshotKey, payload, newsSnapshotTTL).Err()
}

func (st *NewsStore) loadSnapshotFromRedis(ctx context.Context) ([]model.NewsArticle, time.Time) {
	if st.redis == nil {
		return nil, time.Time{}
	}
	raw, err := st.redis.Get(ctx, newsSnapshotKey).Bytes()
	if err != nil || len(raw) == 0 {
		return nil, time.Time{}
	}
	var snap struct {
		Articles  []model.NewsArticle `json:"articles"`
		UpdatedAt time.Time           `json:"updatedAt"`
	}
	if json.Unmarshal(raw, &snap) != nil {
		return nil, time.Time{}
	}
	return snap.Articles, snap.UpdatedAt
}

func (st *NewsStore) mergeArticles(ctx context.Context, incoming []model.NewsArticle) {
	if len(incoming) == 0 {
		return
	}

	st.mu.Lock()
	byID := make(map[string]model.NewsArticle, len(st.articles)+len(incoming))
	for _, a := range st.articles {
		byID[a.ID] = a
	}
	for _, a := range incoming {
		if prev, ok := byID[a.ID]; ok {
			if prev.ViewCount > a.ViewCount {
				a.ViewCount = prev.ViewCount
			}
			if prev.Highlighted {
				a.Highlighted = true
			}
		}
		byID[a.ID] = a
	}
	merged := make([]model.NewsArticle, 0, len(byID))
	for _, a := range byID {
		merged = append(merged, a)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].PublishedAt.After(merged[j].PublishedAt)
	})
	if len(merged) > 300 {
		merged = merged[:300]
	}
	st.articles = merged
	st.updatedAt = time.Now()
	at := st.updatedAt
	st.mu.Unlock()

	st.persistSnapshotRedis(ctx, merged, at)
}

func newsURLHash(link string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(strings.ToLower(link))))
	return hex.EncodeToString(sum[:])
}

func newsArticleID(urlHash string) string {
	if len(urlHash) >= 16 {
		return urlHash[:16]
	}
	return urlHash
}

func classifyNewsTags(headline string) []string {
	h := strings.ToLower(headline)
	switch {
	case strings.Contains(h, "속보"):
		return []string{"속보"}
	case strings.Contains(h, "분석") || strings.Contains(h, "전망") || strings.Contains(h, "리포트"):
		return []string{"분석"}
	default:
		return nil
	}
}

func (st *NewsStore) logFetchErr(source string, err error) {
	if err != nil {
		log.Printf("news %s: %v", source, err)
	}
}
