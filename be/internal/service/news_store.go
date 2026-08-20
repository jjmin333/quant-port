package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"net/http"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/safego"
	"github.com/redis/go-redis/v9"
)

const (
	newsPollInterval             = 2 * time.Minute
	newsUSCompanyPollInterval    = 8 * time.Minute
	newsNaverKeywordsPerTick     = 18
	newsKakaoKeywordsPerTick     = 4
	newsUSGoogleKeywordsPerTick  = 8
	newsSnapshotKey              = "news:domestic:snapshot:v3"
	newsKeywordCursorKey         = "news:domestic:keyword_cursor:v1"
	newsUSSnapshotKey            = "news:us:snapshot:v1"
	newsUSKeywordCursorKey       = "news:us:keyword_cursor:v1"
	newsUSGoogleKeywordCursorKey = "news:us:google_keyword_cursor:v1"
	newsSnapshotTTL              = 6 * time.Hour
	newsAdminKeywordCacheTTL     = time.Minute
)

var defaultDomesticAdminKeywords = []string{
	"코스피", "코스닥", "증시", "반도체", "원달러", "환율",
	"한국은행", "기준금리", "금리", "엔비디아", "2차전지",
	"배당", "실적", "공매도", "IPO", "ETF", "채권", "유가",
	"비트코인", "가상자산", "VIX", "금값", "나스닥", "S&P 500", "다우",
}

var defaultUSAdminKeywords = []string{
	"S&P 500", "Nasdaq", "Nasdaq 100", "Dow Jones", "Wall Street", "Federal Reserve",
	"crude oil", "oil price", "OPEC", "Iran sanctions", "stock market",
	"earnings", "inflation CPI", "tariff trade war", "semiconductor stocks",
	"VIX", "SOX", "Bitcoin", "gold price", "Treasury yield",
}

type newsKeyword struct {
	Text     string
	Priority int
	Symbol   string
}

type NewsStore struct {
	cfg          config.NewsConfig
	naver        *marketdata.NaverNewsClient
	kakao        *marketdata.KakaoNewsClient
	rss          *marketdata.RSSNewsClient
	finnhubNews  *marketdata.FinnhubNewsClient
	rssUS        *marketdata.RSSNewsClient
	googleNews   *marketdata.GoogleNewsRSSClient
	papago       *marketdata.PapagoClient
	llm          *llm.Client
	repo         *repository.NewsRepo
	keywords     *repository.NewsKeywordRepo
	pollSettings *repository.NewsPollSettingsRepo
	stocks       *repository.StockRepo
	watchlist    *repository.WatchlistRepo
	holdings     *repository.UserHoldingsRepo
	redis        *redis.Client
	embedHTTP    *http.Client
	ogHTTP       *http.Client

	mu            sync.RWMutex
	articles      []model.NewsArticle
	updatedAt     time.Time
	keywordCursor int
	nameIndex     map[string]string            // lowercase name -> symbol (domestic)
	nameIndexAt   time.Time                    // when nameIndex was last built (TTL, see nameIndexTTL)
	nameIndexGen  uint64                       // bumped whenever nameIndex is rebuilt
	symbolMemo    map[string]usSymbolMemoEntry // article text -> extracted symbols (domestic)
	// 관심·보유 종목 합집합. watchlist 쿼리 + 포트폴리오 조회라 싸지 않은데
	// extractSymbols가 기사마다 부른다.
	trackedSymbolsCache []string
	trackedSymbolsAt    time.Time
	listCache           map[string]newsListCacheEntry  // filter key -> built list
	countCache          map[string]newsCountCacheEntry // market -> total count
	// listBuild collapses concurrent rebuilds of the same list key. Set once in
	// NewNewsStore and never reassigned, so it needs no locking of its own.
	listBuild       *quoteRefreshCoordinator
	usArticles      []model.NewsArticle
	usUpdatedAt     time.Time
	usKeywordCursor int
	usGoogleCursor  int
	usNameIndex     map[string]string // lowercase name -> US symbol
	usNameIndexAt   time.Time         // when usNameIndex was last built
	// 관심·보유에서 모은 미국 티커. extractUSSymbols가 **기사마다** 부르는데 그
	// 안쪽은 watchlist 쿼리 + 포트폴리오 조회 + stocks.ListBySymbols라, 목록 한 번에
	// DB 수백 회가 나갔다 (실측: /news/us 한 요청에 portfolio Redis GET만 200회).
	// 이 목록은 관심종목을 담고 빼야 바뀌는 값이라 초 단위로 다시 셀 이유가 없다.
	usIndexSymbolsCache []string
	usIndexSymbolsAt    time.Time
	usNameIndexGen      uint64                       // bumped whenever usNameIndex is rebuilt
	usSymbolMemo        map[string]usSymbolMemoEntry // article text -> extracted symbols
	adminKeywordsCache  map[string]cachedKeywords
	pollSettingsCache   model.NewsPollSettings
	pollSettingsAt      time.Time
}

type cachedKeywords struct {
	items    []string
	loadedAt time.Time
}

func NewNewsStore(
	cfg config.NewsConfig,
	naver *marketdata.NaverNewsClient,
	kakao *marketdata.KakaoNewsClient,
	rss *marketdata.RSSNewsClient,
	finnhubNews *marketdata.FinnhubNewsClient,
	rssUS *marketdata.RSSNewsClient,
	googleNews *marketdata.GoogleNewsRSSClient,
	papago *marketdata.PapagoClient,
	repo *repository.NewsRepo,
	keywords *repository.NewsKeywordRepo,
	pollSettings *repository.NewsPollSettingsRepo,
	stocks *repository.StockRepo,
	watchlist *repository.WatchlistRepo,
	holdings *repository.UserHoldingsRepo,
	redisClient *redis.Client,
) *NewsStore {
	return &NewsStore{
		cfg:                cfg,
		naver:              naver,
		kakao:              kakao,
		rss:                rss,
		finnhubNews:        finnhubNews,
		rssUS:              rssUS,
		googleNews:         googleNews,
		papago:             papago,
		repo:               repo,
		keywords:           keywords,
		pollSettings:       pollSettings,
		stocks:             stocks,
		watchlist:          watchlist,
		holdings:           holdings,
		redis:              redisClient,
		embedHTTP:          &http.Client{Timeout: 8 * time.Second},
		ogHTTP:             &http.Client{Timeout: 3 * time.Second},
		nameIndex:          make(map[string]string),
		usNameIndex:        make(map[string]string),
		adminKeywordsCache: make(map[string]cachedKeywords),
		pollSettingsCache:  model.DefaultNewsPollSettings(),
		listBuild:          newQuoteRefreshCoordinator(),
	}
}

func (st *NewsStore) SetLLM(client *llm.Client) {
	st.llm = client
}

func (st *NewsStore) Run(ctx context.Context) {
	st.loadFromStorage(ctx)
	st.loadUSFromStorage(ctx)
	st.refreshRSS(ctx)
	st.refreshRSSUS(ctx)
	st.refreshNaver(ctx)
	st.refreshKakao(ctx)
	st.refreshFinnhubUSGeneral(ctx)
	st.refreshUSGoogleNews(ctx)
	go st.backfillMissingImages(ctx)
	go st.backfillUSTranslations(ctx)
	// 종목 추출 규칙이 바뀐 뒤 한 번 소급 적용한다. DB만 읽고 쓰므로 외부
	// 할당량을 쓰지 않고, 시장별 Redis 표식으로 재기동마다 반복되지 않는다.
	go safego.Call("news-retag", func() { st.retagOnceIfNeeded(ctx) })
	go safego.Call("news-remigrate-market", func() { st.remigrateMarketOnceIfNeeded(ctx) })

	// The five same-interval sources are offset against each other so they do
	// not all fire on the same tick; otherwise every poll period queued five
	// upstream fetches back to back on this single goroutine.
	step := newsPollInterval / 5
	naverTicker := staggeredTicker(ctx, newsPollInterval, 0)
	rssTicker := staggeredTicker(ctx, newsPollInterval, step)
	rssUSTicker := staggeredTicker(ctx, newsPollInterval, 2*step)
	finnhubGeneralTicker := staggeredTicker(ctx, newsPollInterval, 3*step)
	googleNewsTicker := staggeredTicker(ctx, newsPollInterval, 4*step)
	finnhubCompanyTicker := staggeredTicker(ctx, newsUSCompanyPollInterval, step/2)

	go st.runKakaoPoller(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-naverTicker:
			st.refreshNaver(ctx)
		case <-rssTicker:
			st.refreshRSS(ctx)
		case <-rssUSTicker:
			st.refreshRSSUS(ctx)
		case <-finnhubGeneralTicker:
			st.refreshFinnhubUSGeneral(ctx)
		case <-googleNewsTicker:
			st.refreshUSGoogleNews(ctx)
		case <-finnhubCompanyTicker:
			st.refreshFinnhubUSCompany(ctx)
		}
	}
}

// staggeredTicker fires every interval, with the first tick delayed by offset.
// Ticks are dropped rather than queued when the consumer is busy, matching
// time.Ticker semantics. The goroutine exits when ctx is cancelled.
func staggeredTicker(ctx context.Context, interval, offset time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	go func() {
		defer close(ch)
		if offset > 0 {
			delay := time.NewTimer(offset)
			select {
			case <-ctx.Done():
				delay.Stop()
				return
			case <-delay.C:
			}
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				select {
				case ch <- now:
				default:
				}
			}
		}
	}()
	return ch
}

/*
listCacheTTL — 같은 목록 요청을 다시 계산하기까지 기다리는 시간.

이 목록을 만드는 일은 DB 조회 한 번으로 끝나지 않는다. 기사마다 관련성 판정과
미리보기 이미지 결정을 다시 하고, 그 둘이 이름 인덱스를 훑는다. 결과는 **폴러가
새 기사를 넣을 때만** 바뀌는데, 방문자 수만큼 다시 계산했다.

15초는 뉴스 목록에서 사람이 알아챌 수 없는 지연이고 — 폴링 주기 자체가 분 단위다 —
동시 접속자가 몇이든 이 계산이 15초에 한 번으로 묶인다.
*/
const listCacheTTL = 15 * time.Second

type newsListCacheEntry struct {
	articles  []model.NewsArticle
	updatedAt time.Time
	builtAt   time.Time
}

func newsListCacheKey(f model.NewsListFilter) string {
	return strings.Join([]string{
		f.Market, f.SourceID, f.Sort, f.Query, f.Symbol, strconv.Itoa(f.Limit),
	}, "\x00")
}

func (st *NewsStore) listFromCache(key string) ([]model.NewsArticle, time.Time, bool) {
	articles, at, fresh, _ := st.listFromCacheAllowStale(key)
	if !fresh {
		return nil, time.Time{}, false
	}
	return articles, at, true
}

// listFromCacheAllowStale also reports entries past their TTL, so the caller can
// serve the old list while a rebuild runs in the background. `exists` says an
// entry was found at all; `fresh` says it is still within listCacheTTL.
func (st *NewsStore) listFromCacheAllowStale(key string) (articles []model.NewsArticle, at time.Time, fresh, exists bool) {
	st.mu.RLock()
	entry, ok := st.listCache[key]
	st.mu.RUnlock()
	if !ok {
		return nil, time.Time{}, false, false
	}
	return entry.articles, entry.updatedAt, time.Since(entry.builtAt) <= listCacheTTL, true
}

func (st *NewsStore) rememberList(key string, articles []model.NewsArticle, at time.Time) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.listCache == nil {
		st.listCache = map[string]newsListCacheEntry{}
	}
	// 필터 조합(검색어·종목)은 사용자가 만드는 값이라 키가 무한히 늘 수 있다.
	// 만료된 것부터 걷어내고, 그래도 넘치면 통째로 비운다.
	if len(st.listCache) >= 64 {
		for k, v := range st.listCache {
			if time.Since(v.builtAt) > listCacheTTL {
				delete(st.listCache, k)
			}
		}
		if len(st.listCache) >= 64 {
			st.listCache = map[string]newsListCacheEntry{}
		}
	}
	st.listCache[key] = newsListCacheEntry{articles: articles, updatedAt: at, builtAt: time.Now()}
}

func (st *NewsStore) List(ctx context.Context, filter model.NewsListFilter) ([]model.NewsArticle, time.Time) {
	cacheKey := newsListCacheKey(filter)
	if articles, at, ok := st.listFromCache(cacheKey); ok {
		return articles, at
	}

	if st.repo != nil {
		/*
			같은 목록을 동시에 다시 만드는 것을 한 번으로 묶는다.

			캐시가 만료되거나 폴러가 새 기사를 넣어 캐시를 버린 직후(2분마다)에는
			들어오는 요청이 전부 미스다. 지금까지는 그 요청들이 **각자** DB를 읽고
			기사마다 관련성 판정과 미리보기 이미지 결정을 다시 했다. 단일 요청이면
			60 ms인 일이 40 동시 요청에서는 1.7~6.6초가 됐다(실측).

			doSync는 시세 쪽에서 쓰던 것을 그대로 쓴다 — 소유자 하나만 만들고
			나머지는 done 채널에서 기다리다 만들어진 값을 캐시에서 읽는다.
			대기는 quoteRefreshWait(5초)에서 끊기므로, 재구성이 이상하게 길어져도
			요청이 무한정 잡혀 있지 않고 아래 스냅샷 폴백으로 내려간다.
		*/
		build := func(buildCtx context.Context) {
			// 앞선 소유자가 이미 채웠으면 다시 만들지 않는다.
			if _, _, ok := st.listFromCache(cacheKey); ok {
				return
			}
			recs, err := st.repo.List(buildCtx, repository.NewsListParams{
				Market:   filter.Market,
				SourceID: filter.SourceID,
				Sort:     filter.Sort,
				Query:    filter.Query,
				Symbol:   filter.Symbol,
				Limit:    filter.Limit,
			})
			if err != nil {
				// 캐시에 아무것도 남기지 않는다 — 호출자가 아래 폴백으로 간다.
				return
			}
			articles := st.recordsToArticles(buildCtx, recs)
			market := filter.Market
			if market == "" {
				market = "domestic"
			}
			at, _ := st.repo.LatestUpdatedAt(buildCtx, market)
			out := st.filterArticles(buildCtx, articles, filterWithoutTextSearch(filter))
			st.rememberList(cacheKey, out, at)
		}

		/*
			만료됐지만 값이 남아 있으면 그것을 그대로 주고 재구성은 뒤로 보낸다
			(카운트에 쓴 것과 같은 방식).

			singleflight만으로는 만료 시점의 요청 하나가 재구성을 기다린다 —
			40 동시 요청 실측에서 p50 1.2초였다. 목록은 15초 TTL로 이미 그만큼
			낡을 수 있는 값이라, 몇 초 더 낡은 것과 1.2초 멈춤 중에는 전자가 낫다.

			**폴러가 새 기사를 넣으면 이 경로를 타지 않는다** — ingest는 listCache를
			통째로 비우므로 남아 있는 값이 없고, 아래 동기 경로로 내려가 새 기사가
			바로 보인다. 즉 "새 기사는 즉시"라는 보장은 그대로다.
		*/
		if stale, at, fresh, exists := st.listFromCacheAllowStale(cacheKey); exists && !fresh {
			if st.listBuild != nil {
				st.listBuild.runOnce("news:list:"+cacheKey, build)
			}
			return stale, at
		}

		if st.listBuild != nil {
			st.listBuild.doSync(ctx, "news:list:"+cacheKey, build)
		} else {
			build(ctx)
		}
		if articles, at, ok := st.listFromCache(cacheKey); ok {
			return articles, at
		}
	}

	st.mu.RLock()
	var snap []model.NewsArticle
	var at time.Time
	if filter.Market == "us" {
		snap = st.usArticles
		at = st.usUpdatedAt
	} else {
		snap = st.articles
		at = st.updatedAt
	}
	st.mu.RUnlock()

	if len(snap) == 0 {
		market := filter.Market
		if market == "" {
			market = "domestic"
		}
		snap, at = st.loadSnapshotFromRedis(ctx, market)
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

	return st.filterArticles(ctx, snap, filter), at
}

/*
countCache — 목록 응답의 meta.totalCount 한 칸을 위한 캐시.

목록 자체는 listCacheTTL로 이미 묶여 있었는데 이 카운트만 캐시 밖에 있었다.
그래서 방문자 수만큼 `SELECT COUNT(1) … WHERE market=?`이 나갔고, **해외에서만**
비쌌다 — NewsRepo.Count가 market="us"일 때 `AND COALESCE(headline_ko,”) != ”`을
덧붙이는데 이 술어는 인덱스를 못 타서 해당 market 행을 전부 읽고 TEXT 컬럼에
함수를 적용한다. 실측 /news/us 660~2,270ms 대 /news/domestic 13ms(행은 국내가 3배
많다). 정작 목록 본 쿼리는 밀리초 단위다.

목록과 같은 TTL·같은 무효화 지점을 쓴다. 새 기사가 들어오면 listCache와 함께
버려지므로 카운트가 목록보다 낡는 일은 없다.
*/
type newsCountCacheEntry struct {
	n       int
	builtAt time.Time
	// refreshing marks that a background recount is already in flight, so a
	// burst of requests arriving on an expired entry starts one query, not one
	// query each.
	refreshing bool
}

// newsCountRefreshTimeout bounds the background recount. The measured US query
// is ~1.6s; anything past this means the DB is in trouble and retrying on the
// next request is better than piling goroutines up against it.
const newsCountRefreshTimeout = 20 * time.Second

func (st *NewsStore) Count(ctx context.Context, market string) int {
	if market == "" {
		market = "domestic"
	}
	if st.repo == nil {
		st.mu.RLock()
		n := len(st.articles)
		st.mu.RUnlock()
		return n
	}

	st.mu.RLock()
	entry, ok := st.countCache[market]
	st.mu.RUnlock()

	if ok {
		if time.Since(entry.builtAt) <= listCacheTTL {
			return entry.n
		}
		// 만료됐지만 값은 있다 — 낡은 값을 그대로 주고 갱신은 뒤로 보낸다.
		//
		// 그냥 여기서 다시 세면 15초마다 한 요청이 1.6초를 뒤집어쓴다(해외 카운트
		// 실측). 이 숫자는 목록 위에 "총 18,775건"으로 찍히는 표시용이라, 몇 초
		// 낡은 값과 정확한 값의 차이를 화면에서 구분할 수 없다. 반면 1.6초 멈춤은
		// 누구나 알아챈다.
		st.startCountRefresh(market)
		return entry.n
	}

	// 값이 아예 없을 때만 동기로 센다 — 기동 직후와 폴러가 캐시를 버린 직후뿐이다.
	n, err := st.repo.Count(ctx, market)
	if err != nil {
		// 캐시하지 않는다 — 실패한 0을 15초 동안 돌려주면 화면이 "기사 0건"이라고
		// 말한다. 다음 요청이 다시 시도하게 둔다.
		return 0
	}
	st.storeCount(market, n)
	return n
}

// startCountRefresh recounts one market in the background. It deliberately does
// not take the caller's context: that context is cancelled as soon as the
// response is written, which would abort the very refresh the next request
// depends on.
func (st *NewsStore) startCountRefresh(market string) {
	st.mu.Lock()
	entry, ok := st.countCache[market]
	if !ok || entry.refreshing {
		st.mu.Unlock()
		return
	}
	entry.refreshing = true
	st.countCache[market] = entry
	st.mu.Unlock()

	safego.Go("news-count-refresh", func() {
		ctx, cancel := context.WithTimeout(context.Background(), newsCountRefreshTimeout)
		defer cancel()

		n, err := st.repo.Count(ctx, market)
		if err != nil {
			// 실패하면 refreshing만 내려 다음 요청이 다시 시도하게 둔다. 낡은
			// 값은 그대로 남아 화면이 빈 숫자를 보지 않는다.
			st.mu.Lock()
			if cur, ok := st.countCache[market]; ok {
				cur.refreshing = false
				st.countCache[market] = cur
			}
			st.mu.Unlock()
			return
		}
		st.storeCount(market, n)
	})
}

func (st *NewsStore) storeCount(market string, n int) {
	st.mu.Lock()
	defer st.mu.Unlock()
	// 폴러가 그사이 countCache를 통째로 버렸을 수 있다(ingest 무효화).
	if st.countCache == nil {
		st.countCache = map[string]newsCountCacheEntry{}
	}
	st.countCache[market] = newsCountCacheEntry{n: n, builtAt: time.Now()}
}

func filterWithoutTextSearch(filter model.NewsListFilter) model.NewsListFilter {
	out := filter
	out.Query = ""
	out.Symbol = ""
	return out
}

func (st *NewsStore) loadFromStorage(ctx context.Context) {
	if snap, at := st.loadSnapshotFromRedis(ctx, "domestic"); len(snap) > 0 {
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
	st.persistSnapshotRedis(ctx, snap, at, "domestic")
}

func (st *NewsStore) recordsToArticles(ctx context.Context, recs []repository.NewsArticleRecord) []model.NewsArticle {
	out := make([]model.NewsArticle, 0, len(recs))
	for _, rec := range recs {
		original := strings.TrimSpace(rec.OriginalURL)
		if original == "" {
			original = strings.TrimSpace(rec.URL)
		}
		canonical := original
		portal := strings.TrimSpace(rec.PortalURL)
		if portal == "" && isNaverPortalURL(rec.URL) {
			portal = strings.TrimSpace(rec.URL)
		}

		publisherID := strings.TrimSpace(rec.PublisherID)
		publisherName := strings.TrimSpace(rec.PublisherName)
		if publisherID == "" || publisherName == "" || publisherID == "unknown" || publisherID == "newspim" {
			pub := marketdata.ResolvePublisherFromArticle(portal, original)
			publisherID = pub.ID
			publisherName = pub.Name
		}

		previewMode := strings.TrimSpace(rec.PreviewMode)
		if previewMode == "" {
			previewMode = string(marketdata.DecidePreviewMode(ctx, st.embedHTTP, st.redis, original))
		}
		if rec.Market == "us" && previewMode == string(marketdata.PreviewModeSummary) && marketdata.ReaderAllowedHost(original) {
			previewMode = string(marketdata.PreviewModeReader)
		}

		imageURL := rec.ImageURL
		if rec.Market == "us" {
			imageURL = st.resolveUSNewsPreviewImage(ctx, original, rec.Headline, rec.Summary, rec.Symbols, rec.ImageURL)
		}

		embedURL := original
		embedPolicy := marketdata.ResolveEmbedPolicy(ctx, st.embedHTTP, st.redis, embedURL)

		out = append(out, model.NewsArticle{
			ID:            rec.ID,
			Market:        rec.Market,
			Headline:      rec.Headline,
			HeadlineKo:    rec.HeadlineKo,
			Summary:       rec.Summary,
			SummaryKo:     rec.SummaryKo,
			SourceID:      rec.SourceID,
			PublisherID:   publisherID,
			PublisherName: publisherName,
			Tags:          rec.Tags,
			Symbols:       rec.Symbols,
			Keywords:      rec.Keywords,
			PublishedAt:   rec.PublishedAt,
			ViewCount:     rec.ViewCount,
			URL:           canonical,
			OriginalURL:   original,
			PortalURL:     portal,
			ImageURL:      imageURL,
			PreviewMode:   previewMode,
			EmbedPolicy:   string(embedPolicy),
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
		if !isDomesticRSSSource(a.SourceID) {
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

func (st *NewsStore) filterArticles(ctx context.Context, items []model.NewsArticle, filter model.NewsListFilter) []model.NewsArticle {
	market := filter.Market
	if market == "" {
		market = "domestic"
	}
	out := make([]model.NewsArticle, 0, len(items))
	for _, a := range items {
		if a.Market != market {
			continue
		}
		if market == "domestic" && !isStockRelatedArticle(ctx, st, a) {
			continue
		}
		if market == "us" && !isUSTranslationReady(a) {
			continue
		}
		if market == "us" && !isUSStockRelatedArticle(ctx, st, a) && !isFinancialJuiceArticle(a) {
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
		if sym := strings.ToUpper(strings.TrimSpace(filter.Symbol)); sym != "" {
			if !articleHasSymbol(a, sym) {
				continue
			}
		}
		out = append(out, a)
	}

	out = dedupeNewsArticles(out)

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
			if out[i].PublishedAt.Equal(out[j].PublishedAt) {
				if market == "us" {
					return usNewsQualityScore(out[i]) > usNewsQualityScore(out[j])
				}
				return domesticNewsQualityScore(out[i]) > domesticNewsQualityScore(out[j])
			}
			return out[i].PublishedAt.After(out[j].PublishedAt)
		})
	}

	limit := filter.Limit
	if limit <= 0 {
		if filter.Market == "us" {
			limit = newsUSListLimit
		} else {
			limit = 100
		}
	}
	if limit > 200 && filter.Market != "us" {
		limit = 200
	}
	if limit > newsUSListLimit && filter.Market == "us" {
		limit = newsUSListLimit
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func articleHasSymbol(a model.NewsArticle, sym string) bool {
	for _, s := range a.Symbols {
		if strings.ToUpper(strings.TrimSpace(s)) == sym {
			return true
		}
	}
	return false
}

func (st *NewsStore) persistSnapshotRedis(ctx context.Context, articles []model.NewsArticle, at time.Time, market string) {
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
	_ = st.redis.Set(ctx, snapshotKeyForMarket(market), payload, newsSnapshotTTL).Err()
}

func snapshotKeyForMarket(market string) string {
	if market == "us" {
		return newsUSSnapshotKey
	}
	return newsSnapshotKey
}

func (st *NewsStore) loadSnapshotFromRedis(ctx context.Context, market string) ([]model.NewsArticle, time.Time) {
	if st.redis == nil {
		return nil, time.Time{}
	}
	raw, err := st.redis.Get(ctx, snapshotKeyForMarket(market)).Bytes()
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

func (st *NewsStore) mergeArticlesForMarket(ctx context.Context, incoming []model.NewsArticle, market string) {
	if len(incoming) == 0 {
		return
	}
	if market == "" {
		market = "domestic"
	}

	st.mu.Lock()
	var existing []model.NewsArticle
	if market == "us" {
		existing = st.usArticles
	} else {
		existing = st.articles
	}
	byID := make(map[string]model.NewsArticle, len(existing)+len(incoming))
	for _, a := range existing {
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
			a.Keywords = mergeArticleKeywords(prev.Keywords, a.Keywords)
		}
		byID[a.ID] = a
	}
	merged := make([]model.NewsArticle, 0, len(byID))
	for _, a := range byID {
		merged = append(merged, a)
	}
	merged = dedupeNewsArticles(merged)
	sortNewsArticlesLatest(merged)
	if len(merged) > mergeCapForMarket(market) {
		merged = merged[:mergeCapForMarket(market)]
	}
	at := time.Now()
	if market == "us" {
		st.usArticles = merged
		st.usUpdatedAt = at
	} else {
		st.articles = merged
		st.updatedAt = at
	}
	// 새 기사가 들어왔으니 이미 만들어 둔 목록은 낡았다. TTL을 기다리지 않고
	// 지금 버린다 — 폴러가 방금 넣은 기사가 다음 요청에 바로 뜨게.
	// 총 건수도 같은 이유로 함께 버린다(목록만 새 기사를 반영하고 카운트는
	// 옛 값이면 화면의 두 숫자가 어긋난다).
	st.listCache = nil
	st.countCache = nil
	st.mu.Unlock()

	st.persistSnapshotRedis(ctx, merged, at, market)
}

func mergeCapForMarket(market string) int {
	if market == "us" {
		return newsUSMergeCap
	}
	return 300
}

func newsURLHash(link string) string {
	sum := sha256.Sum256([]byte(canonicalNewsLink(link)))
	return hex.EncodeToString(sum[:])
}

func isDomesticRSSSource(sourceID string) bool {
	switch strings.ToLower(strings.TrimSpace(sourceID)) {
	case "rss", "yonhap", "tokenpost", "coinreaders", "blockmedia":
		return true
	default:
		return false
	}
}

func newsArticleID(urlHash string) string {
	if len(urlHash) >= 16 {
		return urlHash[:16]
	}
	return urlHash
}

func classifyNewsTags(headline, sourceID, articleURL string) []string {
	if isFinancialJuiceSource(sourceID, articleURL) {
		return financialJuiceNewsTags()
	}
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
