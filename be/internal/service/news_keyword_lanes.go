package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

/*
검색어 큐를 **생산성으로 가른 세 차선**으로 돌린다 (2026-09-17 사용자 요청).

# 무엇이 늦었나

「최근 15분간 기사가 없는 줄 알았는데, 페이지를 보고 있으니 3분 전~15분 전 기사가
갑자기 한꺼번에 뜬다」 — 읽기 캐시를 의심할 만한 모양이지만 그쪽은 아니었다. 목록
캐시는 15초짜리이고, 폴러가 새 기사를 넣으면 TTL을 안 기다리고 버린다
(`dropListCacheForMarket`). 늦은 것은 **수집**이었다.

	· 국내 큐 = 추적 종목 이름 199 + 관리자 키워드 26 = 225
	· 네이버는 2분마다 그중 18개만 훑고 커서를 돌린다
	· 한 바퀴 = 225/18 = 12.5틱 = **25분**, 그래서 평균 지연은 그 절반인 12.5분

실측이 이 계산과 그대로 맞았다 (2026-09-17, 최근 12시간 `published_at→created_at`):
naver 709건 **평균 723초**(12.05분) · 최소 97초 · 최대 3,042초. 국내 기사의 95%가
이 경로로 들어오므로(어제 naver 1,412건 대 rss 31·yonhap 38) **이 회전 주기가 곧
국내 뉴스의 신선도 바닥**이었다.

# 왜 「한도를 올린다」가 답이 아닌가

네이버 호출은 일 13,250 / 한도 24,000(55%)이라 여유가 있긴 하다. 그런데 틱당 개수만
올리면 **자리를 못 내는 말에도 똑같이 더 쓴다.** 7일 생산성을 세어 보면(실측):

	· 50건 이상 낸 검색어 30개 ← 기사의 대부분이 여기서 나온다
	· 10~49건 30개
	· 10건 미만 165개, 그중 **83개는 7일 동안 0건**

즉 지금은 「뉴스가 안 나오는 종목 83개」가 매 회전마다 시장 키워드와 같은 자리를
차지하고 있었다. 한도를 올리는 것은 그 구조를 그대로 둔 채 값만 미는 일이다.

# 차선

같은 예산(틱당 22)을 **생산성에 비례해** 나눈다. 기본 설정 기준(hot 14 · warm 4 ·
cold 4)으로:

	hot  (7일 50건+)  30개 / 14 = 2.1틱 ≈ **4분** 회전
	warm (7일 10건+)  30개 /  4 = 7.5틱 ≈ 15분 회전
	cold (그 밖)     165개 /  4 = 41틱  ≈ 82분 회전

시장·대형주 뉴스가 12.5분에서 4분으로 내려온다. 대신 **꼬리는 느려진다** — 25분에서
82분이다. 그 자리는 주당 1건 남짓을 내는 종목들이고(165개 × ~1.5건/주 ≈ 하루 35건,
국내 하루 1,400건의 2.5%), 나머지 97%가 세 배 빨라진다. 이 교환이 이 파일의 전부다.

# 관리자 키워드는 cold로 내려가지 않는다

방금 추가한 키워드는 당연히 0건이다. 그것을 「생산성 없음」으로 읽어 82분 차선에
넣으면, 관리자가 무언가를 보려고 넣은 말이 한 시간 반 뒤에 처음 불린다. 사람이
고른 말은 **적어도 warm**이다 — 데이터가 쌓이면 스스로 hot으로 올라간다.

# 차선마다 커서가 따로다

커서를 하나만 두면 차선을 나눈 의미가 없다. 커서는 `market:lane`별로 Redis에
남으므로 재기동해도 제자리에서 이어진다 — 기존 단일 커서와 같은 규칙이다.
*/

type newsKeywordLane string

const (
	newsLaneHot  newsKeywordLane = "hot"
	newsLaneWarm newsKeywordLane = "warm"
	newsLaneCold newsKeywordLane = "cold"
)

// 빠른 차선부터. 예산이 남으면 이 순서로 다시 나눠 준다.
var newsKeywordLaneOrder = []newsKeywordLane{newsLaneHot, newsLaneWarm, newsLaneCold}

const (
	// 생산성을 세는 창. 주 단위라 요일 효과(주말 기사 가뭄)에 흔들리지 않는다.
	newsKeywordCountWindow = 7 * 24 * time.Hour
	// 세어 둔 값을 다시 쓰는 시간. 이 집계는 7일치를 훑으므로(운영 DB는 1 ECPU다)
	// 2분마다 할 일이 아니고, 차선은 그보다 훨씬 천천히 바뀐다.
	newsKeywordCountTTL = time.Hour

	newsLaneHotMinArticles  = 50 // 7일 50건 ≈ 하루 7건
	newsLaneWarmMinArticles = 10
)

/*
keywordArticleCounts — 검색어별 7일 기사 수. 실패하면 **빈 map**을 준다.

빈 map이면 모든 검색어가 cold로 읽히는데, 그래도 회전은 멈추지 않는다(아래
splitLaneBudget이 빈 차선의 몫을 다른 차선으로 넘긴다). 집계 한 번 실패가 수집을
세우는 것보다는 한 시간 동안 예전처럼 도는 편이 낫다.
*/
func (st *NewsStore) keywordArticleCounts(ctx context.Context, market string) map[string]int {
	st.mu.RLock()
	cached, ok := st.keywordCounts[market]
	at := st.keywordCountsAt[market]
	st.mu.RUnlock()
	if ok && time.Since(at) < newsKeywordCountTTL {
		return cached
	}
	if st.repo == nil {
		return nil
	}

	counts, err := st.repo.KeywordArticleCounts(ctx, market, time.Now().Add(-newsKeywordCountWindow))
	if err != nil {
		st.logFetchErr("news-keyword-counts", err)
		counts = map[string]int{}
	}
	if counts == nil {
		counts = map[string]int{}
	}

	st.mu.Lock()
	if st.keywordCounts == nil {
		st.keywordCounts = map[string]map[string]int{}
	}
	if st.keywordCountsAt == nil {
		st.keywordCountsAt = map[string]time.Time{}
	}
	st.keywordCounts[market] = counts
	st.keywordCountsAt[market] = time.Now()
	st.mu.Unlock()
	return counts
}

// laneOf — 이 검색어가 어느 차선에 서는가.
func laneOf(kw newsKeyword, counts map[string]int) newsKeywordLane {
	n := counts[strings.ToLower(strings.TrimSpace(kw.Text))]
	switch {
	case n >= newsLaneHotMinArticles:
		return newsLaneHot
	case n >= newsLaneWarmMinArticles:
		return newsLaneWarm
	case kw.Symbol == "":
		// 종목이 아닌 것 = 사람이 고른 관리자 키워드. 머리글 참조.
		return newsLaneWarm
	default:
		return newsLaneCold
	}
}

/*
splitLaneBudget — 틱당 호출 예산을 차선에 나눈다.

느린 두 차선에 각각 5분의 1씩, 나머지가 전부 hot이다. 기본값 22면 14·4·4다.
어떤 값을 줘도 각 차선은 최소 1을 갖는다 — 0이면 그 차선은 **영원히** 안 돈다.
*/
func splitLaneBudget(total int) map[newsKeywordLane]int {
	if total <= 0 {
		return map[newsKeywordLane]int{}
	}
	if total <= 3 {
		// 나눌 것이 없다. 전부 빠른 차선에 준다 — 느린 차선 몫은 아래
		// nextKeywordsInLanes의 재분배가 hot에서 먼저 채우므로 손해가 아니다.
		return map[newsKeywordLane]int{newsLaneHot: total}
	}
	side := total / 5
	if side < 1 {
		side = 1
	}
	hot := total - 2*side
	if hot < 1 {
		hot = 1
	}
	return map[newsKeywordLane]int{
		newsLaneHot:  hot,
		newsLaneWarm: side,
		newsLaneCold: side,
	}
}

func newsLaneCursorKey(prefix string, lane newsKeywordLane) string {
	return fmt.Sprintf("%s:%s", prefix, lane)
}

/*
takeFromLane — 한 차선에서 `n`개를 집고 커서를 그만큼 민다.

한 틱 안에서 같은 차선을 두 번 집을 수 있어서(예산 재분배) 커서는 집은 만큼만
민다. 차선 길이를 넘겨 집는 일은 부르는 쪽이 막는다 — 그래야 같은 검색어를 한
틱에 두 번 부르지 않는다.
*/
func (st *NewsStore) takeFromLane(
	ctx context.Context,
	cursorPrefix string,
	lane newsKeywordLane,
	pool []newsKeyword,
	n int,
) []newsKeyword {
	if n <= 0 || len(pool) == 0 {
		return nil
	}
	if n > len(pool) {
		n = len(pool)
	}
	key := newsLaneCursorKey(cursorPrefix, lane)

	st.mu.Lock()
	cursor := st.laneCursors[key]
	if st.redis != nil {
		if v, err := st.redis.Get(ctx, key).Int(); err == nil && v >= 0 {
			cursor = v
		}
	}
	if cursor >= len(pool) {
		cursor = 0
	}
	out := make([]newsKeyword, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, pool[(cursor+i)%len(pool)])
	}
	next := (cursor + n) % len(pool)
	if st.laneCursors == nil {
		st.laneCursors = map[string]int{}
	}
	st.laneCursors[key] = next
	st.mu.Unlock()

	if st.redis != nil {
		_ = st.redis.Set(ctx, key, next, 7*24*time.Hour).Err()
	}
	return out
}

/*
nextKeywordsInLanes — 이번 틱에 부를 검색어들.

**빈 자리는 버리지 않는다.** 어떤 차선이 예산보다 짧으면(hot이 10개인데 14를 받는
식) 남은 몫은 빠른 차선부터 다시 나눈다. 안 그러면 큐가 작은 초기 상태에서
한도의 절반만 쓰고 만다.
*/
func (st *NewsStore) nextKeywordsInLanes(
	ctx context.Context,
	market string,
	cursorPrefix string,
	queue []newsKeyword,
	total int,
) []newsKeyword {
	if len(queue) == 0 || total <= 0 {
		return nil
	}
	if total > len(queue) {
		total = len(queue)
	}

	counts := st.keywordArticleCounts(ctx, market)
	pools := map[newsKeywordLane][]newsKeyword{}
	for _, kw := range queue {
		lane := laneOf(kw, counts)
		pools[lane] = append(pools[lane], kw)
	}

	budget := splitLaneBudget(total)
	taken := map[newsKeywordLane]int{}
	var out []newsKeyword
	for _, lane := range newsKeywordLaneOrder {
		want := budget[lane]
		if want > len(pools[lane]) {
			want = len(pools[lane])
		}
		if want <= 0 {
			continue
		}
		out = append(out, st.takeFromLane(ctx, cursorPrefix, lane, pools[lane], want)...)
		taken[lane] = want
	}

	// 남은 자리 — 빠른 차선부터 채운다.
	for _, lane := range newsKeywordLaneOrder {
		short := total - len(out)
		if short <= 0 {
			break
		}
		room := len(pools[lane]) - taken[lane]
		if room <= 0 {
			continue
		}
		if short > room {
			short = room
		}
		out = append(out, st.takeFromLane(ctx, cursorPrefix, lane, pools[lane], short)...)
		taken[lane] += short
	}
	return out
}
