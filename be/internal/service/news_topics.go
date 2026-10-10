package service

import (
	"context"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
)

/*
국내 뉴스 카테고리 — 부동산 · 금리 · 예·적금 · 대출 (2026-10-09 사용자 요청 4, 결정 A 6칩).

# 왜 따로 수집하나

국내 수집 큐는 「종목 이름 + 관리자 키워드」이고, 들어온 기사는 **종목 관련성 필터**를 통과해야 남는다
(`passesStockRelevance`). 부동산·예금 기사는 그 필터에 막혀 들어올 길이 없었다. 그래서:

  - **주제 차선**: 네이버 틱(2분)마다 주제 검색어 2개(18개 → 36분에 한 바퀴, 하루 약 +1,440콜 /
    한도 24,000 중 기존 13,250). 이 차선의 기사는 종목 필터 대신 **제목에 주제어가 있어야** 남는다.
  - **분류**: 종목이 안 붙은 국내 기사는 제목 규칙으로 topic을 정한다(종목 차선으로 들어온
    「기준금리 동결에 코스피…」도 금리로). 종목이 붙으면 주식이다 — 「현대건설 재건축 수주」는 주식 뉴스다.
  - **자산 칩**: 주제 기사의 제목에서 시군구·금리 이름을 뽑아 저장한다 — 누르면 자산 미리보기 모달.

해외는 주식만이다(화면도 칩을 숨긴다).
*/

type newsTopicSeed struct {
	topic string
	query string
}

/*
newsTopicSeeds — 관리자 표(news_poll_keywords.topic, migration 101)가 비었거나 읽을 수 없을 때만 쓰는 기본값.
정본은 관리자 「뉴스 관리」 화면이다(2026-10-09 사용자 요청 — 검색어를 코드에서 관리자 키워드로).
*/
var newsTopicSeeds = []newsTopicSeed{
	{"real_estate", "아파트 매매"}, {"real_estate", "아파트 전세"}, {"real_estate", "부동산 시장"},
	{"real_estate", "청약"}, {"real_estate", "재건축"}, {"real_estate", "주택 공급"},
	{"rates", "기준금리"}, {"rates", "국고채 금리"}, {"rates", "한국은행 금리"}, {"rates", "미국 금리"},
	{"deposit", "예금 금리"}, {"deposit", "적금 특판"}, {"deposit", "파킹통장"}, {"deposit", "정기예금"},
	{"loan", "주담대 금리"}, {"loan", "전세대출"}, {"loan", "신용대출 금리"}, {"loan", "DSR"},
}

const newsTopicKeywordsPerTick = 2

// newsTopicTerms — 제목·요약에서 찾는 말. 대소문자 무시.
var newsTopicTerms = map[string][]string{
	"real_estate": {"아파트", "부동산", "집값", "전셋값", "전세가", "월세", "청약", "재건축", "재개발", "분양",
		"주택", "매매가", "실거래", "토지거래허가", "공시가격", "종부세", "오피스텔", "빌라", "미분양"},
	"loan": {"대출", "주담대", "dsr", "ltv", "가계부채", "코픽스", "cofix"},
	"deposit": {"예금", "적금", "파킹통장", "특판", "예금자보호", "수신금리", "예적금", "예·적금"},
	"rates": {"기준금리", "국고채", "국채 금리", "국채금리", "채권금리", "시장금리", "금통위", "금리 인하", "금리인하",
		"금리 인상", "금리인상", "금리 동결", "금리동결", "fomc", "연준"},
}

// newsTopicStockGuards — 제목에 있으면 주제 기사로 보지 않는 증시 말. 「청약」은 공모주 청약에도 쓰인다.
var newsTopicStockGuards = []string{"증시", "코스피", "코스닥", "나스닥", "다우", "s&p", "주가", "ipo", "공모", "상장", "증거금", "etf"}

// 같은 점수면 좁은 주제가 이긴다 — 「주담대 금리 인하」는 금리가 아니라 대출, 「예금 금리 인하」는 예·적금.
var newsTopicTieOrder = []string{"deposit", "loan", "real_estate", "rates"}

/*
classifyNewsTopic — 제목에 주제어가 하나도 없으면 ""(분류 안 함). 요약만으로는 정하지 않는다 —
요약 끝의 「한편 부동산 시장은…」 한 줄로 주식 기사가 부동산 칸에 가면 칩이 거짓말을 한다.
점수 = 제목 일치 × 2 + 요약 일치.
*/
func classifyNewsTopic(title, summary string) string {
	t, s := strings.ToLower(title), strings.ToLower(summary)
	// 제목이 증시 기사라고 말하면 주식이다 — 「美증시, 국채금리 상승에도…」·「IPO 청약 증거금」(10-09 소급 실측).
	for _, g := range newsTopicStockGuards {
		if strings.Contains(t, g) {
			return ""
		}
	}
	best, bestScore := "", 0
	for _, topic := range newsTopicTieOrder {
		titleHits, score := 0, 0
		for _, term := range newsTopicTerms[topic] {
			if strings.Contains(t, term) {
				titleHits++
				score += 2
			}
			if strings.Contains(s, term) {
				score++
			}
		}
		if titleHits > 0 && score > bestScore {
			best, bestScore = topic, score
		}
	}
	return best
}

// IsNewsTopic reports whether t is a known topic (handler validation).
func IsNewsTopic(t string) bool {
	for _, x := range model.NewsTopics {
		if x == t {
			return true
		}
	}
	return false
}

// NewsAssetTagger finds asset chips in a headline (AssetMentionService).
type NewsAssetTagger interface {
	TagHeadline(ctx context.Context, text string) []model.NewsAssetTag
}

// SetAssetTagger wires the asset name index used for topic-article chips. 폴러가 이미 돌고 있을 때
// 불리므로(server.go 순서) 잠금 아래에서 바꾼다.
func (st *NewsStore) SetAssetTagger(t NewsAssetTagger) {
	st.mu.Lock()
	st.assetTagger = t
	st.mu.Unlock()
}

// topicFor decides an article's topic and chips. 국내 · 종목 없음 · 제목에 주제어 — 셋 다일 때만 주식 밖.
func (st *NewsStore) topicFor(ctx context.Context, market, title, summary string, symbols []string) (string, []model.NewsAssetTag) {
	if market != "domestic" || len(symbols) > 0 {
		return model.NewsTopicStock, nil
	}
	topic := classifyNewsTopic(title, summary)
	if topic == "" {
		return model.NewsTopicStock, nil
	}
	st.mu.RLock()
	tagger := st.assetTagger
	st.mu.RUnlock()
	var tags []model.NewsAssetTag
	if tagger != nil {
		tags = tagger.TagHeadline(ctx, title)
	}
	return topic, tags
}

var newsTopicCursor atomic.Int64

// refreshTopics — 네이버 틱마다 주제 검색어 두 개. 커서는 프로세스 메모리(재기동하면 처음부터 — 36분 한 바퀴라 문제없다).
func (st *NewsStore) refreshTopics(ctx context.Context) {
	if st.naver == nil || !st.naver.Enabled() {
		return
	}
	seeds := st.topicKeywords(ctx)
	if len(seeds) == 0 {
		return
	}
	var ingested []model.NewsArticle
	for i := 0; i < min(newsTopicKeywordsPerTick, len(seeds)); i++ {
		seed := seeds[int(newsTopicCursor.Add(1)-1)%len(seeds)]
		items, err := st.naver.Search(ctx, seed.query, 10)
		if err != nil {
			st.logFetchErr("naver-topic", err)
			continue
		}
		for _, item := range items {
			if a, ok := st.ingestTopicItem(ctx, item, seed.query); ok {
				ingested = append(ingested, a)
			}
		}
	}
	st.mergeIngestedArticles(ctx, ingested)
}

// ingestTopicItem — ingestItem과 같은 길이되 종목 관련성 대신 「제목에 주제어」로 거른다.
func (st *NewsStore) ingestTopicItem(ctx context.Context, item marketdata.RawNewsItem, query string) (model.NewsArticle, bool) {
	title := strings.TrimSpace(item.Title)
	if title == "" || classifyNewsTopic(title, "") == "" {
		return model.NewsArticle{}, false
	}
	canonical, original, _ := resolveNewsURLs(item)
	if canonical == "" {
		return model.NewsArticle{}, false
	}
	if original == "" {
		original = canonical
	}
	storedURL := canonicalNewsLink(original)
	if storedURL == "" {
		storedURL = canonicalNewsLink(canonical)
	}
	if storedURL == "" || st.shouldSkipDuplicateIngest(ctx, "domestic", item, title, storedURL) {
		return model.NewsArticle{}, false
	}
	if st.repo != nil {
		if ok, err := st.repo.ExistsByURLHash(ctx, newsURLHash(storedURL)); err == nil && ok {
			_ = st.repo.AppendKeywords(ctx, newsURLHash(storedURL), []string{query})
			return model.NewsArticle{}, false
		}
	}
	return st.normalizeItem(ctx, item, []string{query}, "", "domestic")
}

/*
retagTopicsOnce — 이미 들어와 있는 국내 기사(최근 30일, 종목 없음)의 topic·칩을 지금 규칙으로 맞춘다(양방향).
기동마다 돈다: 분류된 행은 topic이 바뀌어 다음 회차 대상에서 빠지고, 남는 것은 주식으로 남은
종목 없는 기사 몇천 행을 한 번 읽는 값이다.
ponytail: 표식 없이 매 기동 재스캔 — 행이 수만이 되면 Redis 표식(retagUntilDone 방식)으로.
*/
func (st *NewsStore) retagTopicsOnce(ctx context.Context) {
	// 자산 칩 색인(SetAssetTagger)은 기동 뒤쪽에서 꽂힌다 — 그 전에 돌면 칩 없이 분류만 된다.
	if st.repo == nil || !sleepCtx(ctx, 90*time.Second) {
		return
	}
	rows, err := st.repo.TopicRetagCandidates(ctx, time.Now().AddDate(0, 0, -30), 20000)
	if err != nil {
		log.Printf("news topic retag: %v", err)
		return
	}
	n := 0
	for _, r := range rows {
		if ctx.Err() != nil {
			return
		}
		topic, tags := st.topicFor(ctx, "domestic", r.Headline, r.Summary, nil)
		// 칩 규칙이 넓어지면(법정동·단지, 10-09) 주제는 그대로여도 칩을 다시 쓴다.
		if topic == r.Topic && sameAssetTags(tags, r.AssetTags) {
			continue
		}
		if err := st.repo.UpdateTopic(ctx, r.ID, topic, tags); err != nil {
			log.Printf("news topic retag %s: %v", r.ID, err)
			return
		}
		n++
	}
	if n > 0 {
		log.Printf("news topic retag: %d / %d articles changed topic", n, len(rows))
		st.dropListCacheForMarket("domestic")
	}
}

func articleTopic(a model.NewsArticle) string {
	if a.Topic == "" {
		return model.NewsTopicStock
	}
	return a.Topic
}

// topicKeywordsCacheKey — 관리자 키워드 캐시(adminKeywordsCache)에 같이 둔다: 관리자가 키워드를 고치면
// InvalidateAdminKeywords가 이것도 함께 비운다. 값은 "topic\t검색어".
const topicKeywordsCacheKey = "domestic:topics"

// topicKeywords — 켜진 주제 검색어(관리자 표). 표를 못 읽으면 씨앗, 표가 비면(전부 껐다) 빈 목록 = 차선 정지.
func (st *NewsStore) topicKeywords(ctx context.Context) []newsTopicSeed {
	st.mu.RLock()
	cached, ok := st.adminKeywordsCache[topicKeywordsCacheKey]
	st.mu.RUnlock()
	var lines []string
	if ok && time.Since(cached.loadedAt) < newsAdminKeywordCacheTTL {
		lines = cached.items
	} else {
		if st.keywords == nil {
			return newsTopicSeeds
		}
		items, err := st.keywords.ListTopicKeywords(ctx)
		if err != nil {
			st.logFetchErr("news-topic-keywords", err)
			return newsTopicSeeds
		}
		for _, it := range items {
			lines = append(lines, it.Topic+"\t"+it.Keyword)
		}
		st.mu.Lock()
		st.adminKeywordsCache[topicKeywordsCacheKey] = cachedKeywords{items: lines, loadedAt: time.Now()}
		st.mu.Unlock()
	}
	out := make([]newsTopicSeed, 0, len(lines))
	for _, l := range lines {
		if t, q, ok := strings.Cut(l, "\t"); ok {
			out = append(out, newsTopicSeed{topic: t, query: q})
		}
	}
	return out
}

func sameAssetTags(a, b []model.NewsAssetTag) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
