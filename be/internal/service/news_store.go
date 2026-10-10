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
	"sync/atomic"
	"time"

	"net/http"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/safego"
	"github.com/enact/quant-port/be/internal/translate"
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
	// 차선별 커서의 접두사. 실제 키는 여기에 ":hot" 같은 차선 이름이 붙는다
	// (newsLaneCursorKey). 단일 커서 시절 키와 이름을 갈라 둔 것은, 배포 뒤
	// 남아 있는 옛 값이 차선 커서로 잘못 읽히지 않게 하려는 것이다.
	newsDomesticLaneCursorPrefix = "news:domestic:lane_cursor:v1"
	newsUSGoogleLaneCursorPrefix = "news:us:google_lane_cursor:v1"
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
	cfg         config.NewsConfig
	naver       *marketdata.NaverNewsClient
	kakao       *marketdata.KakaoNewsClient
	rss         *marketdata.RSSNewsClient
	finnhubNews *marketdata.FinnhubNewsClient
	rssUS       *marketdata.RSSNewsClient
	googleNews  *marketdata.GoogleNewsRSSClient
	/*
		translator — 해외 제목 EN→KO. **엔진을 직접 들지 않는다.**

		예전에는 여기가 `papago` 하나였고 `news_translate.go`가 그걸 직접 불렀다.
		그러면 구현을 바꾸려면 호출부를 고쳐야 하고, 무엇보다 두 엔진을 나란히 돌려
		비교할 수가 없다 — 자체 모델로 갈아타기 전에 꼭 해야 하는 일이 그것인데.

		Papago는 이제 코드에 없다 — 자격증명이 만료됐고 갱신하지 않기로 했다(09-03).
		지금 엔진은 자체 모델 하나뿐이라, 사슬이 실패하면 그 제목은 **영어로 남는다**.
	*/
	translator *translate.Chain
	/*
		translateBudgetLogged — 번역 예산 소진을 **한 번만** 적기 위한 빗장.

		소진은 사고가 아니라 상태다. 하루 한도에 닿으면 남은 하루 내내 같은 오류가
		오고 수집 경로는 새 기사마다 부르므로, 그대로 찍으면 하루 수백 줄이 된다.
		다시 통과하면 내려가 다음 소진이 한 줄을 받는다(`translateHeadlines`).
	*/
	translateBudgetLogged atomic.Bool
	llm          *llm.Pool
	repo         *repository.NewsRepo
	keywords     *repository.NewsKeywordRepo
	pollSettings *repository.NewsPollSettingsRepo
	stocks       *repository.StockRepo
	watchlist    *repository.WatchlistRepo
	holdings     *repository.UserHoldingsRepo
	redis        *redis.Client
	embedHTTP    *http.Client
	ogHTTP       *http.Client
	// assetTagger — 주제 기사 제목에서 자산 칩(지역·금리)을 뽑는다(news_topics.go). nil이면 칩 없이.
	assetTagger NewsAssetTagger

	// mergeMu — 적재 합치기끼리만 줄 세운다. 읽기 경로는 이것을 잡지 않는다
	// (mergeArticlesForMarket 헤더).
	mergeMu sync.Mutex

	mu        sync.RWMutex
	articles  []model.NewsArticle
	updatedAt time.Time
	/*
		laneCursors — "차선 커서 접두사:차선" -> 다음에 집을 자리.

		Redis가 있으면 그쪽이 정본이고 이 map은 그 앞의 그림자다(Redis가 비어 있는
		동안에도 회전이 제자리걸음을 하지 않게). 차선을 나눈 이유는
		news_keyword_lanes.go 머리글에.
	*/
	laneCursors map[string]int
	// 검색어별 7일 기사 수. market -> 소문자 검색어 -> 건수. 차선을 가르는 값이고
	// 한 시간에 한 번만 다시 센다(newsKeywordCountTTL).
	keywordCounts   map[string]map[string]int
	keywordCountsAt map[string]time.Time
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
	// finnhubCategoryCursor는 해외 일반 뉴스 카테고리 로테이션이다. 한 회차에
	// 한 갈래만 부른다 — 이유는 refreshFinnhubUSGeneral 머리글에.
	finnhubCategoryCursor int
	usNameIndex           map[string]string // lowercase name -> US symbol
	/*
		usTickerIndex — **대문자 티커** -> 심볼. 이름 인덱스와 갈라 둔 이유가 이
		파일에서 제일 중요한 한 가지다.

		예전에는 티커도 소문자로 이름 인덱스에 들어갔다. 그 인덱스는 소문자로
		내린 본문에 맞추므로 「stocks fall **amid** rate fears」가 AMID(ARGENT MID
		CAP ETF) 기사가 되고 「**high** yield」가 HIGH(SIMPLIFY ENHANCED INCOME
		ETF) 기사가 됐다 — 실측 1,000건에서 MOVE 30 · AMID 20 · ALTO 18 · PALO 18 ·
		HIGH 17 · LIVE 15 · TECH 14 …

		티커는 **대문자로 쓰인다.** 원문 대소문자 그대로 경계까지 보고 맞추면
		(`usTickerTokenMatch`) 소문자 낱말과는 애초에 부딪히지 않는다.
	*/
	usTickerIndex map[string]string // UPPERCASE ticker -> US symbol
	// usAliases — `stock_aliases` 표의 한글 별칭 스냅샷. 인덱스와 **같은 주기로**
	// 갈아 끼운다(`rebuildUSNameIndex`). 기사마다 표를 다시 받아 오면 기사 수만큼
	// map 사본이 생기는데, 여기는 목록 요청 하나가 기사 100건을 만드는 자리다.
	usAliases map[string]string // 한글 별칭 -> US symbol
	// usCompanySweeping — 종목별 해외 뉴스 회차가 지금 돌고 있는가. 그 회차는
	// 고루틴으로 나가므로(Run의 select 참조) 겹침을 여기서 막는다.
	usCompanySweeping atomic.Bool
	usNameIndexAt     time.Time // when usNameIndex was last built
	// 관심·보유에서 모은 미국 티커. extractUSSymbols가 **기사마다** 부르는데 그
	// 안쪽은 watchlist 쿼리 + 포트폴리오 조회 + stocks.ListBySymbols라, 목록 한 번에
	// DB 수백 회가 나갔다 (실측: /news/us 한 요청에 portfolio Redis GET만 200회).
	// 이 목록은 관심종목을 담고 빼야 바뀌는 값이라 초 단위로 다시 셀 이유가 없다.
	usIndexSymbolsCache []string
	usIndexSymbolsAt    time.Time
	usNameIndexGen      uint64                       // bumped whenever usNameIndex is rebuilt
	usSymbolMemo        map[string]usSymbolMemoEntry // article text -> extracted symbols
	usNameLookupMemo    map[string]usSymbolMemoEntry // lower name -> stocks.Search 결과(없음 포함)
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
		usTickerIndex:      make(map[string]string),
		adminKeywordsCache: make(map[string]cachedKeywords),
		pollSettingsCache:  model.DefaultNewsPollSettings(),
		listBuild:          newQuoteRefreshCoordinator(),
	}
}

func (st *NewsStore) SetLLM(pool *llm.Pool) {
	st.llm = pool
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
	go safego.Call("news-retag", func() { st.retagUntilDone(ctx) })
	go safego.Call("news-remigrate-market", func() { st.remigrateMarketOnceIfNeeded(ctx) })
	go safego.Call("news-topic-retag", func() { st.retagTopicsOnce(ctx) })

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
			st.refreshTopics(ctx)
		case <-rssTicker:
			st.refreshRSS(ctx)
		case <-rssUSTicker:
			st.refreshRSSUS(ctx)
		case <-finnhubGeneralTicker:
			st.refreshFinnhubUSGeneral(ctx)
		case <-googleNewsTicker:
			st.refreshUSGoogleNews(ctx)
		case <-finnhubCompanyTicker:
			/*
				**이 회차만 고루틴으로 낸다.** 종목별 뉴스는 종목마다 2초씩 스스로
				쉬므로 한 회차가 약 64초를 쓰는데(32종목), 이 select 루프는 뉴스
				소스 **전부**가 공유하는 한 줄이다. 여기서 붙잡고 있으면 그 사이
				naver·rss 틱이 버려진다(`staggeredTicker`는 큐가 아니라 버린다) —
				바로 위 `refreshFinnhubUSGeneral` 머리글이 「기다리게 하지 않는다」를
				택한 이유가 그것이었다.

				겹치지 않게 하는 것은 `usCompanySweeping`이다. 주기(8분)가 한 회차
				(64초)보다 훨씬 길어 평시에는 겹칠 일이 없지만, 상대가 느려지면
				회차가 쌓여 한도를 두 배로 쓴다 — 그때는 **거르는 쪽**이 맞다.
			*/
			if st.usCompanySweeping.CompareAndSwap(false, true) {
				go safego.Call("news-us-company", func() {
					defer st.usCompanySweeping.Store(false)
					st.refreshFinnhubUSCompany(ctx)
				})
			}
		}
	}
}

/*
weekendTickGate — 주말에는 폴링 틱을 솎아 낸다.

**평일은 손대지 않는다.** 뉴스 신선도는 이 서비스의 체감 품질이고, 실적·공시
뉴스가 장 마감 뒤에도 나오므로 「장외에는 늦게 받아도 된다」가 성립하지 않는다.
그리고 뉴스 폴링은 사용자 수와 무관한 **고정 비용**이라 트래픽이 늘어도 커지지
않는다 — 이번 성능 작업의 기준(「트래픽이 늘면 무너지는가」)에 걸리지 않는다.

주말만 다르다. 장이 열리지 않아 기사 생산량 자체가 적어서, 주기를 늘려도
**실시간성 손실이 사실상 0**이면서 고정 소비의 상당 부분을 아낀다.
실측(2026-08-23, 사용자 0명): naver 일 5,538 / 한도 24,000(23%),
**kakao 일 1,170 / 한도 3,000(39%)** — kakao가 한도에 가장 가깝다.

# 벽시계 분으로 세다가 해외 뉴스를 통째로 잃었다 (2026-09-13)

예전 구현은 `now.Minute()%14 != 0`이면 건너뛰었다. 「14분에 한 번」이라고 읽히지만
그것은 **매 분 한 번씩 물어볼 때만** 맞는 말이다. 우리 티커는 매 분이 아니라
**자기 격자 위에서만** 깨어난다:

	· 2분 주기 티커는 한 시간에 30번, 전부 **같은 홀짝의 분**에 깨어난다. 격자가
	  홀수 쪽이면 {0,14,28,42,56}과 **영원히 겹치지 않는다** — 그 소스는 주말 내내
	  한 번도 안 돈다. 어느 쪽이 될지는 프로세스가 언제 떴는가로 정해진다.
	· 8분 주기 티커(해외 종목별 뉴스)는 분이 4의 배수 간격으로만 서서 {14, 42}
	  둘만 잡는다. 의도한 14분이 아니라 **평균 60분**이다.

실측(2026-09-13 일요일, 20시간): `finnhub_news` 호출 **10건**(의도대로면 약 85건),
로그에 남은 종목별 뉴스 틱은 03:18·04:43·05:15·06:42·07:14·08:42·09:14·10:42 —
**42분과 14분만** 남아 있다. 해외 일반 뉴스와 구글 뉴스는 그 20시간 동안 사실상
0이었고, 사용자가 「해외 뉴스를 못 가져온다」고 지적한 것이 이 상태다.

병은 「완화가 과했다」가 아니라 **판정 기준이 티커와 다른 축에 있었다**는 것이다.
그래서 벽시계 분을 보지 않고 **그 티커가 마지막으로 통과한 뒤 얼마나 지났는지**를
본다. 격자·오프셋·주기가 무엇이든 주말 실효 주기가 14분 이상으로 일정해지고,
주기가 14분보다 긴 티커는 원래 주기 그대로 돈다(늦추지 않는다).

게이트는 **티커마다 하나**다. 하나를 공유하면 먼저 깨어난 소스가 나머지의 몫을
써 버려, 지금 고치는 것과 같은 「어떤 소스는 영원히 안 돈다」가 다시 생긴다.
*/
type weekendTickGate struct {
	lastPassed time.Time
}

// weekendNewsMinGap — 주말에 한 티커가 두 번 통과하는 사이의 최소 간격.
const weekendNewsMinGap = 14 * time.Minute

func isWeekendKST(now time.Time) bool {
	switch now.In(kst).Weekday() {
	case time.Saturday, time.Sunday:
		return true
	default:
		return false
	}
}

// allow reports whether this ticker may fire now, and records the pass.
func (g *weekendTickGate) allow(now time.Time) bool {
	if isWeekendKST(now) && !g.lastPassed.IsZero() && now.Sub(g.lastPassed) < weekendNewsMinGap {
		return false
	}
	g.lastPassed = now
	return true
}

// staggeredTicker fires every interval, with the first tick delayed by offset.
// Ticks are dropped rather than queued when the consumer is busy, matching
// time.Ticker semantics. The goroutine exits when ctx is cancelled.
//
// 주말 완화는 이 고루틴이 가진 게이트가 판정한다 — weekendTickGate 머리글 참조.
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
		var gate weekendTickGate
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if !gate.allow(now) {
					continue
				}
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
	// canonical — 필터에 사용자 입력이 없는 키인가. 축출 계층을 가른다
	// (rememberList 헤더 참조).
	canonical bool
	// filter — 이 목록을 만든 조건. 적재 직후 뒤에서 다시 만들 때 쓴다(warmListsAfterIngest).
	filter model.NewsListFilter
}

func newsListCacheKey(f model.NewsListFilter) string {
	return strings.Join([]string{
		f.Market, f.SourceID, f.Sort, f.Query, f.Symbol, f.Topic, strconv.Itoa(f.Limit),
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

/*
캐시 상한과 축출 정책.

키에는 `Query`·`Symbol` 같은 **사용자 입력**이 들어가므로 키 공간이 무한하다.
예전 코드도 그걸 알고 상한 64를 뒀지만, 넘치면 **통째로 비웠다**. 그래서
서로 다른 검색어 64개가 들어오면 **기본 목록(실사용자가 실제로 보는 것)까지
함께 날아갔다** — 종목 상세가 `?symbol=`을 붙여 뉴스를 읽으므로, 사용자 64명이
서로 다른 종목을 보기만 해도 그렇게 됐다.

두 계층으로 가른다:

	canonical — 필터에 사용자 입력이 없는 키(market×source×sort×limit). 조합이
	            유한하고(대략 수십 개) **실사용자가 대부분 보는 것**이다.
	ad-hoc    — 검색어·종목이 섞인 키. 무한히 늘 수 있으므로 LRU로 자른다.

canonical은 ad-hoc이 아무리 몰려도 밀려나지 않는다. 이게 이 변경의 전부다.
*/
const (
	newsListCanonicalMax = 64
	newsListAdHocMax     = 48
)

// isCanonicalListKey reports whether the filter carries no user-supplied text.
func isCanonicalListKey(f model.NewsListFilter) bool {
	return strings.TrimSpace(f.Query) == "" && strings.TrimSpace(f.Symbol) == ""
}

func (st *NewsStore) rememberList(key string, filter model.NewsListFilter, articles []model.NewsArticle, at time.Time) {
	canonical := isCanonicalListKey(filter)
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.listCache == nil {
		st.listCache = map[string]newsListCacheEntry{}
	}

	limit := newsListAdHocMax
	if canonical {
		limit = newsListCanonicalMax
	}
	st.evictListLocked(canonical, limit)

	st.listCache[key] = newsListCacheEntry{
		articles: articles, updatedAt: at, builtAt: time.Now(), canonical: canonical, filter: filter,
	}
}

/*
evictListLocked — 같은 계층에서만 자리를 만든다.

만료된 것을 먼저 걷고, 그래도 넘치면 **가장 오래 전에 만들어진 것 하나**를
지운다. 전면 삭제가 아니라 1건 축출이라는 점이 예전 코드와의 차이다.

`builtAt` 기준이라 엄밀히는 LRU가 아니라 FIFO다. 목록 캐시는 TTL이 15초로
짧아 「오래 안 쓰인 것」과 「오래 전에 만들어진 것」이 사실상 같고, 접근 시각을
따로 갱신하면 읽기 경로가 쓰기 잠금을 잡아야 한다 — 그 대가가 이득보다 크다.
*/
func (st *NewsStore) evictListLocked(canonical bool, limit int) {
	count := func() int {
		n := 0
		for _, v := range st.listCache {
			if v.canonical == canonical {
				n++
			}
		}
		return n
	}

	if count() < limit {
		return
	}
	for k, v := range st.listCache {
		if v.canonical == canonical && time.Since(v.builtAt) > listCacheTTL {
			delete(st.listCache, k)
		}
	}
	for count() >= limit {
		var oldestKey string
		var oldestAt time.Time
		for k, v := range st.listCache {
			if v.canonical != canonical {
				continue
			}
			if oldestKey == "" || v.builtAt.Before(oldestAt) {
				oldestKey, oldestAt = k, v.builtAt
			}
		}
		if oldestKey == "" {
			return
		}
		delete(st.listCache, oldestKey)
	}
}

/*
dropListCacheForMarket — 한 시장의 목록 캐시만 버린다.

캐시 키는 `market`을 **맨 앞 필드**로 만든다(newsListCacheKey). 그래서 접두사
비교로 그 시장 것만 고를 수 있다. 호출자가 이미 st.mu를 잡고 있다.
*/
func (st *NewsStore) dropListCacheForMarket(market string) {
	if market == "" {
		market = "domestic"
	}
	prefix := market + "\x00"
	for k := range st.listCache {
		if strings.HasPrefix(k, prefix) {
			delete(st.listCache, k)
		}
	}
}

// listBuilder returns the rebuild step for one list key. 요청 경로(List)와 적재 직후
// 예열(warmListsAfterIngest)이 같은 것을 쓴다 — 두 벌이면 언젠가 어긋난다.
func (st *NewsStore) listBuilder(filter model.NewsListFilter, cacheKey string) func(context.Context) {
	return func(buildCtx context.Context) {
		// 앞선 소유자가 이미 채웠으면 다시 만들지 않는다.
		if _, _, ok := st.listFromCache(cacheKey); ok {
			return
		}
		t0 := time.Now()
		recs, err := st.repo.List(buildCtx, repository.NewsListParams{
			Market:   filter.Market,
			SourceID: filter.SourceID,
			Sort:     filter.Sort,
			Query:    filter.Query,
			Symbol:   filter.Symbol,
			Topic:    filter.Topic,
			Limit:    filter.Limit,
		})
		if err != nil {
			// 캐시에 아무것도 남기지 않는다 — 호출자가 아래 폴백으로 간다.
			return
		}
		tDB := time.Since(t0)
		articles := st.recordsToArticles(buildCtx, recs)
		tConv := time.Since(t0) - tDB
		market := filter.Market
		if market == "" {
			market = "domestic"
		}
		at, _ := st.repo.LatestUpdatedAt(buildCtx, market)
		out := st.filterArticles(buildCtx, articles, filterWithoutTextSearch(filter))
		st.rememberList(cacheKey, filter, out, at)
		// 느린 재구성만 남긴다 — 어느 단계인지가 다음 조사의 출발점이다(09-25 후속2).
		if total := time.Since(t0); total > newsListSlowBuild {
			log.Printf("news list rebuild slow: market=%s symbol=%q q=%q limit=%d rows=%d total=%s db=%s convert=%s filter=%s",
				market, filter.Symbol, filter.Query, filter.Limit, len(recs), total.Round(time.Millisecond),
				tDB.Round(time.Millisecond), tConv.Round(time.Millisecond), (total - tDB - tConv).Round(time.Millisecond))
		}
	}
}

const newsListSlowBuild = 500 * time.Millisecond

/*
newsListWarmWindow — 적재 직후 뒤에서 다시 만들 목록의 범위(최근에 누가 본 것).

적재는 시장 목록 캐시를 통째로 버린다(「새 기사는 즉시」). 그래서 버린 뒤 **처음 온
요청이 재구성을 동기로 기다렸다.** 폴러가 2분마다 돌고 방문 간격은 그보다 길어, 실측
(09-24~25 운영 로그) 뉴스 대시보드는 해외 p50 243ms · 국내 p95 1.7초였고, DB가 바쁜
순간엔 doSync 대기 상한(5초)을 그대로 맞았다(두 시장 동시에 5.0 · 5.2초).

그래서 버린 직후 **최근 이 창 안에 만들어진 목록**을 뒤에서 다시 만든다. canonical
목록과 **종목 목록**(보유·종목 화면이 종목마다 부른다 — 예열 전 해외 적재 직후 첫
요청 ~230ms)이 대상이다. 자유 검색어 키는 다시 부를지 모르므로 예열하지 않는다.
종목 키는 무한히 늘 수 있어 최근 것 newsListWarmSymbolMax개로 자른다. 아무도 안 보는
시간엔 키가 창 밖으로 나가 예열도 멈춘다 — 빈 사이트에 2분마다 DB를 읽지 않는다.
*/
const (
	newsListWarmWindow    = 10 * time.Minute
	newsListWarmSymbolMax = 16
)

// recentWarmFiltersLocked — 호출자가 st.mu를 잡고 있다.
func (st *NewsStore) recentWarmFiltersLocked(market string) []model.NewsListFilter {
	if market == "" {
		market = "domestic"
	}
	prefix := market + "\x00"
	var out []model.NewsListFilter
	type symbolKey struct {
		filter  model.NewsListFilter
		builtAt time.Time
	}
	var symbols []symbolKey
	for k, v := range st.listCache {
		if !strings.HasPrefix(k, prefix) || time.Since(v.builtAt) > newsListWarmWindow {
			continue
		}
		switch {
		case v.canonical:
			out = append(out, v.filter)
		case strings.TrimSpace(v.filter.Query) == "" && strings.TrimSpace(v.filter.Symbol) != "":
			symbols = append(symbols, symbolKey{v.filter, v.builtAt})
		}
	}
	sort.Slice(symbols, func(i, j int) bool { return symbols[i].builtAt.After(symbols[j].builtAt) })
	if len(symbols) > newsListWarmSymbolMax {
		symbols = symbols[:newsListWarmSymbolMax]
	}
	for _, s := range symbols {
		out = append(out, s.filter)
	}
	return out
}

// warmListsAfterIngest rebuilds the given lists in the background, one run per key.
func (st *NewsStore) warmListsAfterIngest(filters []model.NewsListFilter) {
	if st.repo == nil || st.listBuild == nil {
		return
	}
	for _, f := range filters {
		key := newsListCacheKey(f)
		st.listBuild.runOnce("news:list:"+key, st.listBuilder(f, key))
	}
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
		build := st.listBuilder(filter, cacheKey)

		/*
			만료됐지만 값이 남아 있으면 그것을 그대로 주고 재구성은 뒤로 보낸다
			(카운트에 쓴 것과 같은 방식).

			singleflight만으로는 만료 시점의 요청 하나가 재구성을 기다린다 —
			40 동시 요청 실측에서 p50 1.2초였다. 목록은 15초 TTL로 이미 그만큼
			낡을 수 있는 값이라, 몇 초 더 낡은 것과 1.2초 멈춤 중에는 전자가 낫다.

			**폴러가 새 기사를 넣으면 이 경로를 타지 않는다** — ingest는 그 시장의
			listCache를 비우므로 남아 있는 값이 없고, 아래 동기 경로로 내려가 새 기사가
			바로 보인다. 즉 "새 기사는 즉시"라는 보장은 그대로다. 최근 본 목록은
			ingest가 곧바로 뒤에서 다시 만들어 두므로(warmListsAfterIngest) 대개 여기까지
			오지 않고, 와도 진행 중인 재구성에 얹힌다.
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
			Topic:    filter.Topic,
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
			Topic:         rec.Topic,
			AssetTags:     rec.AssetTags,
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
		if t := strings.TrimSpace(filter.Topic); t != "" && articleTopic(a) != t {
			continue
		}
		// 주제 기사(부동산·금리 …)는 종목 관련성 판정 밖이다 — 그 판정이 바로 이 기사들을 막던 문이다.
		if market == "domestic" && articleTopic(a) == model.NewsTopicStock && !isStockRelatedArticle(ctx, st, a) {
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

	/*
		합치기(특히 dedupe)는 **쓰기 잠금 밖에서** 한다.

		예전에는 st.mu.Lock을 잡은 채 이 전부를 했다. dedupe는 기사마다 제목 정규화
		(정규식)를 하고 쌍을 비교해서, 실측(09-25) 해외 1,000건에 **780ms**·국내 461ms가
		걸렸다(데스크톱 CPU — 운영 2코어에선 더). 그동안 목록 캐시를 읽는 **모든 뉴스
		요청이 RLock에서 섰다** — 폴러가 2분마다 돌므로, 방문자가 받은 1~5초 멈춤의
		정체였다(캐시가 살아 있는데도 대시보드 1.07초).

		그래서: 스냅샷을 읽고(RLock) → 잠금 없이 합치고 → 잠깐 Lock으로 갈아 끼운다.
		mergeMu가 합치기끼리만 줄 세운다(읽기는 이것을 기다리지 않는다). 그 사이
		스냅샷이 통째로 바뀌었으면(재적재·정리) 예전처럼 잠금 안에서 다시 합치고,
		자리에서 고친 필드(조회수·이미지·번역)는 carryInPlaceNewsEdits가 옮겨 싣는다.
	*/
	st.mergeMu.Lock()
	defer st.mergeMu.Unlock()

	snapshot := func() []model.NewsArticle {
		if market == "us" {
			return st.usArticles
		}
		return st.articles
	}
	st.mu.RLock()
	existing := snapshot()
	st.mu.RUnlock()
	merged := mergeNewsSnapshot(existing, incoming, market)

	st.mu.Lock()
	if current := snapshot(); sameArticleSlice(current, existing) {
		carryInPlaceNewsEdits(current, merged)
	} else {
		merged = mergeNewsSnapshot(current, incoming, market)
	}
	at := time.Now()
	if market == "us" {
		st.usArticles = merged
		st.usUpdatedAt = at
	} else {
		st.articles = merged
		st.updatedAt = at
	}
	/*
		새 기사가 들어왔으니 이미 만들어 둔 목록은 낡았다. TTL을 기다리지 않고
		지금 버린다 — 폴러가 방금 넣은 기사가 다음 요청에 바로 뜨게. 총 건수도
		같은 이유로 함께 버린다(목록만 새 기사를 반영하고 카운트는 옛 값이면
		화면의 두 숫자가 어긋난다).

		**단, 이 market 것만 버린다.** 예전에는 `listCache = nil`로 통째로 비웠다.
		그런데 뉴스 폴러는 2분 주기이고 국내·해외를 따로 적재하므로, **미국 기사
		하나가 들어올 때마다 국내 목록 캐시까지 날아갔다.** 캐시 수명의 상한이
		사실상 폴링 주기(2분)로 묶여 있었고, 그래서 미스가 반복됐다 —
		미스 1건의 비용은 실측 298~320ms다.

		시장이 둘뿐이라 키를 훑어 지우는 비용은 무시할 만하다(항목 수가 아래
		newsListCacheMaxEntries로 묶여 있다).
	*/
	warm := st.recentWarmFiltersLocked(market)
	st.dropListCacheForMarket(market)
	delete(st.countCache, market)
	st.mu.Unlock()

	st.persistSnapshotRedis(ctx, merged, at, market)
	st.warmListsAfterIngest(warm)
}

// mergeNewsSnapshot — 잠금 없이 도는 순수 함수. 규칙은 예전 그대로다: 같은 ID면 들어온
// 것을 쓰되 조회수는 큰 쪽, 강조는 한 번 켜지면 유지, 키워드는 합친다.
func mergeNewsSnapshot(existing, incoming []model.NewsArticle, market string) []model.NewsArticle {
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
	return merged
}

// sameArticleSlice — 같은 배열의 같은 길이인가(통째로 갈아 끼워졌는지만 본다).
func sameArticleSlice(a, b []model.NewsArticle) bool {
	if len(a) != len(b) {
		return false
	}
	return len(a) == 0 || &a[0] == &b[0]
}

// carryInPlaceNewsEdits — 합치는 동안 스냅샷 **자리에서** 고쳐진 필드를 결과로 옮긴다
// (news_store_backfill의 이미지·조회수, news_translate의 번역). 호출자가 st.mu를 잡고 있다.
func carryInPlaceNewsEdits(current, merged []model.NewsArticle) {
	if len(current) == 0 || len(merged) == 0 {
		return
	}
	idx := make(map[string]int, len(merged))
	for i := range merged {
		idx[merged[i].ID] = i
	}
	for _, c := range current {
		i, ok := idx[c.ID]
		if !ok {
			continue
		}
		m := &merged[i]
		if c.ViewCount > m.ViewCount {
			m.ViewCount = c.ViewCount
		}
		if c.Highlighted {
			m.Highlighted = true
		}
		if m.ImageURL == "" && c.ImageURL != "" {
			m.ImageURL = c.ImageURL
		}
		if m.HeadlineKo == "" && c.HeadlineKo != "" {
			m.HeadlineKo = c.HeadlineKo
			m.SummaryKo = c.SummaryKo
		}
	}
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
