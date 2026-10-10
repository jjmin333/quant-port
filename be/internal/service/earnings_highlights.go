package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// EarningsHighlights is the 우측 레일 — 표가 답하지 못하는 세 질문.
//
// 예전 레일은 「컨센서스 상회/하회 TOP」이었는데, 그건 **서프라이즈 탭이 이미 더
// 길게** 보여주는 목록이라 6줄을 반복하는 셈이었다. 여기 셋은 전부 표에 없는 것이다:
// 표는 날짜순이라 「무엇이 큰 발표인가」를 못 말하고, 한 행만 봐서는 「숫자와 주가가
// 엇갈렸다」와 「이 회사가 늘 기대를 넘는다」를 알 수 없다.
type EarningsHighlights struct {
	// Big — 다가오는 대형 발표(시가총액 순).
	Big []model.EarningsEvent `json:"big"`
	// Divergent — 서프라이즈와 주가가 반대로 간 발표.
	Divergent []model.EarningsEvent `json:"divergent"`
	// Consistent — 최근 8분기 컨센서스 적중률 상위 기업.
	Consistent []model.EarningsConsistency `json:"consistent"`
}

const (
	// earningsHighlightsTTL은 「신선한 시간」, earningsHighlightsKeep은 「사는 시간」이다.
	//
	// 예전에는 30분 TTL 하나였고 만료된 요청이 세 질의를 **동기로** 기다렸다. 실측
	// (09-24~25 운영 로그): 이 레일 요청 14건이 **전부** 2.8~3.1초였다 — 방문 간격이
	// 30분보다 길어 사실상 모든 요청이 캐시 만료를 맞았다. 사용자 수와 무관하게
	// 오는 지연이라 「사용자가 적은데 느리다」는 체감과 맞는다.
	//
	// 지금은 값을 하루 넘게 들고 있다가, 신선하지 않으면 그 값을 주고 뒤에서 다시
	// 만든다. 재료가 배치로만 들어오므로 30분 낡은 레일은 문제가 되지 않는다.
	earningsHighlightsTTL    = 30 * time.Minute
	earningsHighlightsKeep   = 36 * time.Hour
	earningsHighlightsKeyFmt = "earnings:highlights:v3:%s"

	// 대형 발표는 앞으로 2주를 본다 — 그보다 멀면 「다가오는」이 아니다.
	highlightsUpcomingDays = 14
	// 다만 **국내는 실적이 2·5·8·11월에 몰려** 시즌 사이에는 2주 안에 아무것도
	// 없다(실측: 08-18 기준 향후 2주 0건). 그때 빈 카드를 두느니 창을 넓혀
	// 「다음에 올 큰 발표」를 보여준다 — 날짜를 함께 적으므로 먼 일이라는 사실은
	// 화면에 남는다.
	//
	// 100일인 것은 **국내 실적 시즌 간격이 약 3개월**이기 때문이다 — 8월 시즌이
	// 끝나면 다음은 11월 초다(실측: 08-18 기준 다음 추정 발표가 11-02로 76일 뒤).
	// 75일로 잡았다가 하루 차이로 놓쳐 카드가 빈 채였다.
	highlightsUpcomingFallbackDays = 100
	// 엇갈린 발표·적중 이력은 최근 실적 시즌을 덮어야 한다.
	highlightsDivergentDays  = 45
	highlightsConsistentDays = 730

	highlightsLimit = 6
	// 적중률을 말하려면 표본이 있어야 한다. 2분기짜리 「2/2 100%」가 8분기
	// 「7/8」보다 위에 오면 목록이 쓸모없어진다.
	highlightsMinQuarters = 4
	// 이 카드만 **4분기로 고정**한다(종목 실적 탭은 earningsHitHistoryQuarters=8 그대로).
	//
	// 두 시장을 한 화면에서 같은 말로 설명해야 하기 때문이다. 해외는 소스가
	// 4분기까지만 주고(무료 티어 제약, migrations/055) 국내는 2년치가 있어서,
	// 창을 안 맞추면 카드 하단 문구가 한쪽에서 반드시 거짓이 된다 — 「최근 8분기
	// 기준」이라고 적어 두고 해외 행에는 4/4가 뜨는 식이다.
	//
	// 국내가 잃는 것이 있다(NH투자증권 7/7 → 4/4). 그 대신 얻는 것은 두 탭의
	// 숫자가 같은 뜻을 갖는 것이고, 표본 깊이는 종목 실적 탭이 8분기로 그대로 답한다.
	highlightsHitHistoryQuarters = 4
)

// Highlights builds all three rail lists in one pass.
//
// 하나의 요청으로 묶은 이유: 카드마다 부르면 레일 하나에 세 왕복이고, 셋이 같은
// 표를 읽으므로 서버에서 한 번 읽는 편이 싸다. 실패하면 셋 다 비지만 전부 부가
// 정보라 본문은 그대로 뜬다.
func (s *EarningsCalendarService) Highlights(
	ctx context.Context, market string,
) (EarningsHighlights, error) {
	out := EarningsHighlights{
		Big:        []model.EarningsEvent{},
		Divergent:  []model.EarningsEvent{},
		Consistent: []model.EarningsConsistency{},
	}
	if s == nil || s.events == nil {
		return out, nil
	}

	now := time.Now()
	nowKST := now.In(model.KST)
	today := time.Date(nowKST.Year(), nowKST.Month(), nowKST.Day(), 0, 0, 0, 0, time.UTC)
	day := today.Format("2006-01-02")

	cacheKey := fmt.Sprintf(earningsHighlightsKeyFmt, market)
	build := func(buildCtx context.Context) {
		fresh := EarningsHighlights{
			Big:        s.upcomingBig(buildCtx, today, market),
			Divergent:  s.divergentAnnouncements(buildCtx, today, market),
			Consistent: s.consistentBeaters(buildCtx, today, market),
		}
		s.cacheSetTTL(buildCtx, cacheKey, earningsHighlightsEntry{
			Day: day, BuiltAt: time.Now(), Data: fresh,
		}, earningsHighlightsKeep)
	}

	var cached earningsHighlightsEntry
	if s.cacheGet(ctx, cacheKey, &cached) {
		if cached.isFresh(day, now) {
			return cached.Data, nil
		}
		// 낡았어도 값은 있다 — 그것을 주고 재구성은 뒤로 보낸다. 자정을 넘긴 직후의
		// 어제 레일도 여기로 온다(창이 하루 밀린 목록이 몇 초 보이는 쪽이 3초 멈춤보다 낫다).
		s.highlightsRefresh().runOnce("earnings:highlights:"+market, build)
		return cached.Data, nil
	}

	// 줄 것이 없을 때(첫 기동·키 만료)만 기다린다. 동시에 온 요청은 한 번의 재구성에 얹힌다.
	s.highlightsRefresh().doSync(ctx, "earnings:highlights:"+market, build)
	if s.cacheGet(ctx, cacheKey, &cached) {
		return cached.Data, nil
	}
	// Redis가 없거나 쓰기에 실패했다 — 직접 만들어 준다.
	out.Big = s.upcomingBig(ctx, today, market)
	out.Divergent = s.divergentAnnouncements(ctx, today, market)
	out.Consistent = s.consistentBeaters(ctx, today, market)
	return out, nil
}

// earningsHighlightsEntry — 레일과 그것을 만든 시점. 신선도 판정에 둘 다 필요하다:
// 30분이 안 지났어도 **날짜가 바뀌면** 창(앞으로 14일 · 지난 45일)이 달라진다.
type earningsHighlightsEntry struct {
	Day     string             `json:"day"`
	BuiltAt time.Time          `json:"builtAt"`
	Data    EarningsHighlights `json:"data"`
}

func (e earningsHighlightsEntry) isFresh(day string, now time.Time) bool {
	return e.Day == day && now.Sub(e.BuiltAt) < earningsHighlightsTTL
}

func (s *EarningsCalendarService) highlightsRefresh() *quoteRefreshCoordinator {
	s.refreshOnce.Do(func() { s.refresh = newQuoteRefreshCoordinator() })
	return s.refresh
}

// upcomingBig — 다가오는 발표를 시가총액 순으로 (R1).
//
// 표는 날짜순이라 「이번 주에 큰 게 오나」를 못 말한다. 국내는 예정일이 전부
// 추정이므로 그 사실이 행마다 배지로 남는다.
func (s *EarningsCalendarService) upcomingBig(
	ctx context.Context, today time.Time, market string,
) []model.EarningsEvent {
	if found := s.upcomingBigWithin(ctx, today, market, highlightsUpcomingDays); len(found) > 0 {
		return found
	}
	// 시즌 사이라 2주 안이 비었다. 넓혀서 「다음에 올 것」을 보여준다.
	return s.upcomingBigWithin(ctx, today, market, highlightsUpcomingFallbackDays)
}

func (s *EarningsCalendarService) upcomingBigWithin(
	ctx context.Context, today time.Time, market string, days int,
) []model.EarningsEvent {
	to := today.AddDate(0, 0, days)
	queryFrom := model.ExpandUSQueryFrom(today, market)
	rows, err := s.events.ListRange(ctx, queryFrom, to, market, nil)
	if err != nil {
		return []model.EarningsEvent{}
	}
	if market != string(model.AssetUS) {
		if estimated, err := s.EstimatedDomestic(ctx, today, to); err == nil {
			rows = append(rows, estimated...)
		}
	}
	rows = s.prepareCalendarRows(ctx, rows, today, to, market)

	// 이미 발표된 것은 「다가오는」이 아니다.
	upcoming := make([]model.EarningsEvent, 0, len(rows))
	for _, ev := range rows {
		if ev.DateStatus == model.EarningsDateConfirmed {
			continue
		}
		upcoming = append(upcoming, ev)
	}
	s.AttachMarketCaps(ctx, upcoming)

	// 시가총액을 모르는 종목은 뺀다 — 「대형 발표」 목록에 크기 미상이 섞이면
	// 정렬 기준이 없는 행이 목록에 남는다.
	sized := make([]model.EarningsEvent, 0, len(upcoming))
	for _, ev := range upcoming {
		if ev.MarketCap != nil {
			sized = append(sized, ev)
		}
	}
	sort.SliceStable(sized, func(i, j int) bool { return *sized[i].MarketCap > *sized[j].MarketCap })
	return trimEvents(sized, highlightsLimit)
}

// divergentAnnouncements — 서프라이즈와 주가가 **반대로** 간 발표 (R2).
//
// 이 화면에서만 만들 수 있는 신호다. 실측: 한국금융지주는 컨센서스를 +27.8%
// 넘겼는데 다음 거래일 −7.3%였고, 엘앤에프는 −58.2% 하회했는데 +7.3%였다.
// 숫자와 시장이 다르게 말한 경우이고, 서프라이즈 목록만으로는 절대 안 보인다.
func (s *EarningsCalendarService) divergentAnnouncements(
	ctx context.Context, today time.Time, market string,
) []model.EarningsEvent {
	from := today.AddDate(0, 0, -highlightsDivergentDays)
	queryFrom := model.ExpandUSQueryFrom(from, market)
	rows, err := s.events.Announced(ctx, queryFrom, today, market)
	if err != nil {
		return []model.EarningsEvent{}
	}
	rows = s.prepareCalendarRows(ctx, rows, from, today, market)

	divergent := make([]model.EarningsEvent, 0, 16)
	for _, ev := range rows {
		if !model.IsDivergentEarnings(ev) {
			continue
		}
		if ev.Surprise != nil && !materialSurprise(*ev.Surprise) {
			continue
		}
		divergent = append(divergent, ev)
	}
	// 어긋난 폭이 큰 순 — 서프라이즈와 반응이 둘 다 클수록 이야깃거리다.
	sort.SliceStable(divergent, func(i, j int) bool {
		return divergenceScore(divergent[i]) > divergenceScore(divergent[j])
	})
	s.AttachMarketCaps(ctx, divergent)
	return trimEvents(divergent, highlightsLimit)
}

func divergenceScore(ev model.EarningsEvent) float64 {
	if ev.Surprise == nil || ev.Surprise.Pct == nil || ev.Reaction == nil {
		return 0
	}
	return math.Abs(*ev.Surprise.Pct) + math.Abs(ev.Reaction.Pct)*3
}

// consistentBeaters — 최근 8분기 컨센서스 적중률 상위 (R3).
//
// 한 번의 서프라이즈는 우연이다. 「8분기 중 7번」은 그 회사의 가이던스가 보수적
// 이라는 뜻이고, 다음 분기를 볼 때 실제로 쓰는 정보다.
//
// SQL 집계 대신 Go에서 세는 이유: 서프라이즈 계산(부호 가드·유의성 하한)이 이미
// 한 곳에 있고, SQL로 옮기면 같은 규칙이 두 벌이 되어 반드시 어긋난다. 2년치
// 국내가 2,650행이라 메모리에서 세도 싸다.
func (s *EarningsCalendarService) consistentBeaters(
	ctx context.Context, today time.Time, market string,
) []model.EarningsConsistency {
	from := today.AddDate(0, 0, -highlightsConsistentDays)
	queryFrom := model.ExpandUSQueryFrom(from, market)
	rows, err := s.events.Announced(ctx, queryFrom, today, market)
	if err != nil {
		return []model.EarningsConsistency{}
	}
	rows = s.prepareCalendarRows(ctx, rows, from, today, market)

	bySymbol := map[string][]model.EarningsEvent{}
	// seen — (종목·회계연도·분기)로 같은 분기를 한 번만 센다. 아래에서 합치는
	// 서프라이즈 이력이 **이미 여기 있는 분기를 포함해** 4분기를 주기 때문이다
	// (실측 AAPL: 이력의 FY2026 Q3 = 캘린더의 FY2026 3Q, est·act까지 동일).
	// 이 가드가 없으면 최근 분기가 두 번 세어져 적중률이 조용히 왜곡된다.
	seen := map[string]struct{}{}
	for _, ev := range rows {
		if ev.Surprise == nil || ev.Surprise.Pct == nil || !materialSurprise(*ev.Surprise) {
			continue
		}
		seen[model.EarningsPeriodMergeKey(ev.Symbol, ev.FiscalYear, ev.FiscalPeriod)] = struct{}{}
		bySymbol[ev.Symbol] = append(bySymbol[ev.Symbol], ev)
	}
	s.appendSurpriseHistory(ctx, market, from, seen, bySymbol)

	out := make([]model.EarningsConsistency, 0, len(bySymbol))
	for symbol, events := range bySymbol {
		// **여기서 다시 정렬해야 한다.** Announced는 발표일 내림차순으로 오지만,
		// 위에서 합친 이력 행은 그 뒤에 그냥 붙었다. ComputeHitHistory가 앞에서
		// 8개를 자르므로, 정렬하지 않으면 「최근 8분기」가 아니라 「캘린더 1분기
		// + 이력 아무거나」가 된다.
		sortByRecency(events)
		hist := model.ComputeHitHistory(events, highlightsHitHistoryQuarters)
		if hist == nil || hist.Quarters < highlightsMinQuarters {
			continue
		}
		out = append(out, model.EarningsConsistency{
			Symbol:  symbol,
			Name:    events[0].Name,
			Market:  events[0].Market,
			History: *hist,
		})
	}

	// 시가총액은 **동률을 깨기 위해서만** 읽는다. 앞의 두 키로 이미 탈락이 확정된
	// 종목은 물어볼 필요가 없어, 먼저 후보를 잘라 낸 뒤 그만큼만 조회한다.
	out = trimToTieGroup(out)
	caps := s.consistencyMarketCaps(ctx, out)

	sort.SliceStable(out, func(i, j int) bool {
		ri := float64(out[i].History.Beats) / float64(out[i].History.Quarters)
		rj := float64(out[j].History.Beats) / float64(out[j].History.Quarters)
		if ri != rj {
			return ri > rj
		}
		// 같은 비율이면 **표본이 많은 쪽**이 위다 — 4/4보다 8/8이 강한 주장이다.
		if out[i].History.Quarters != out[j].History.Quarters {
			return out[i].History.Quarters > out[j].History.Quarters
		}
		// 여기서 시총이 필요한 이유는 해외 데이터의 모양 때문이다. 해외는 표본이
		// **전부 정확히 4분기**라(소스가 4분기만 준다) 앞의 두 키가 통째로 동률이고,
		// 실측 479종목이 4/4로 묶인다. 그 상태에서 중위값으로 바로 넘어가면 「중위
		// 서프라이즈가 가장 큰 6개」가 뽑히는데, 그건 가이던스가 보수적인 회사가
		// 아니라 **컨센서스가 못 맞추는 회사**다(실측 1위 중위 +132.6%).
		// 카드가 답하려는 질문과 정반대라, 동률일 때는 규모로 먼저 가른다.
		//
		// 국내에도 영향이 있다. 4~10분기로 흩어져 있어 해외만큼 심하진 않지만
		// 5/5 동률이 9종목이라(실측), 그 그룹의 순서가 규모 순으로 바뀐다.
		if ci, cj := caps[out[i].Symbol], caps[out[j].Symbol]; ci != cj {
			return ci > cj
		}
		mi, mj := 0.0, 0.0
		if out[i].History.MedianPct != nil {
			mi = *out[i].History.MedianPct
		}
		if out[j].History.MedianPct != nil {
			mj = *out[j].History.MedianPct
		}
		return mi > mj
	})
	if len(out) > highlightsLimit {
		out = out[:highlightsLimit]
	}
	return out
}

// trimToTieGroup drops candidates that cannot reach the top `highlightsLimit`
// on the first two keys alone.
//
// 시총 조회가 종목 수에 비례해 느려서 둔 가지치기다(실측 해외 1,555종목 조회에
// 콜드 3.5초). 적중률·분기 수만으로 정렬해 보면, **6위의 (적중률, 분기 수)보다
// 확실히 낮은 종목은 시총이 얼마든 6위 안에 못 든다** — 시총은 그 두 키 다음에
// 오는 세 번째 키이기 때문이다. 그런 종목은 물어보지 않는다.
//
// 동률 그룹은 통째로 남긴다. 해외는 479종목이 (100%, 4분기)로 묶여 있어 그 안의
// 순서를 정하려면 그만큼은 알아야 한다.
func trimToTieGroup(rows []model.EarningsConsistency) []model.EarningsConsistency {
	if len(rows) <= highlightsLimit {
		return rows
	}
	rank := func(r model.EarningsConsistency) (float64, int) {
		return float64(r.History.Beats) / float64(r.History.Quarters), r.History.Quarters
	}
	sort.SliceStable(rows, func(i, j int) bool {
		ri, qi := rank(rows[i])
		rj, qj := rank(rows[j])
		if ri != rj {
			return ri > rj
		}
		return qi > qj
	})
	// 6위와 같은 등급까지가 경계다. 그 뒤로 같은 등급이 이어지면 함께 남긴다.
	cutRate, cutQuarters := rank(rows[highlightsLimit-1])
	end := highlightsLimit
	for end < len(rows) {
		r, q := rank(rows[end])
		if r != cutRate || q != cutQuarters {
			break
		}
		end++
	}
	return rows[:end]
}

// consistencyMarketCaps looks up caps for the surviving candidates, once.
//
// 결과는 레일 전체가 30분 캐시되므로 이 질의는 그 주기로만 돈다.
//
// 못 구한 종목은 0으로 남아 동률 그룹의 맨 뒤로 간다 — 시총을 모르는 종목을
// 카드 상단에 올리는 것보다 낫다.
func (s *EarningsCalendarService) consistencyMarketCaps(
	ctx context.Context, rows []model.EarningsConsistency,
) map[string]float64 {
	if s == nil || s.events == nil || len(rows) == 0 {
		return map[string]float64{}
	}
	symbols := make([]string, 0, len(rows))
	for _, r := range rows {
		symbols = append(symbols, r.Symbol)
	}
	caps, err := s.events.MarketCapsFor(ctx, symbols)
	if err != nil {
		// 시총은 동률을 깨는 용도라, 없으면 예전처럼 중위값으로 갈린다.
		log.Printf("earnings highlights: consistency market caps 조회 실패: %v", err)
		return map[string]float64{}
	}
	return caps
}

// appendSurpriseHistory folds in past quarters that arrived without an
// announcement date (해외 전용 — migrations/055).
//
// 이 표가 필요한 이유는 소스 제약 하나다: Finnhub 무료 티어의 실적 캘린더는
// **과거 30일까지만** 준다. 그래서 해외는 캘린더만으로는 종목당 1분기밖에 못
// 모으고, 최소 표본 4분기를 넘기는 종목이 0개였다. `/stock/earnings`가 주는
// 4분기를 일회성으로 받아 둔 것이 이 표다(cmd/backfill-earnings-surprises).
//
// 국내는 이 표에 행이 없다 — FnGuide가 2년치를 주므로 캘린더만으로 충분하다.
// 그래서 시장별 분기를 여기에 두지 않는다: 국내는 자연히 0행이 되어 동작이
// 예전과 완전히 같다.
func (s *EarningsCalendarService) appendSurpriseHistory(
	ctx context.Context,
	market string,
	from time.Time,
	seen map[string]struct{},
	bySymbol map[string][]model.EarningsEvent,
) {
	if s == nil || s.history == nil {
		return
	}
	rows, err := s.history.ListMarketSince(ctx, market, from)
	if err != nil {
		// 카드 하나가 얕아질 뿐이라 조용히 넘어간다 — 레일 전체를 비우는 것보다 낫다.
		log.Printf("earnings highlights: surprise history 조회 실패: %v", err)
		return
	}
	mergeSurpriseHistory(rows, seen, bySymbol)
}

// mergeSurpriseHistory folds history rows into the per-symbol buckets, skipping
// quarters the calendar already supplied.
//
// 순수 함수로 떼어 둔 이유는 여기가 이 기능에서 **조용히 틀리기 가장 쉬운 곳**이기
// 때문이다. 중복 가드가 빠지면 최근 분기가 두 번 세어지고, 그 오류는 화면에
// 「4/4 상회」처럼 그럴듯하게 나타나 아무도 눈치채지 못한다.
func mergeSurpriseHistory(
	rows []model.EarningsSurpriseRow,
	seen map[string]struct{},
	bySymbol map[string][]model.EarningsEvent,
) {
	for _, row := range rows {
		key := row.PeriodKey()
		if _, dup := seen[key]; dup {
			continue
		}
		ev := row.AsEvent()
		// 캘린더에서 온 행과 **같은 관문**을 통과시킨다. 기준선 하한을 여기서
		// 건너뛰면 컨센서스 0.01달러짜리 종목이 적중률 상위를 차지한다.
		if ev.Surprise == nil || ev.Surprise.Pct == nil || !materialSurprise(*ev.Surprise) {
			continue
		}
		seen[key] = struct{}{}
		bySymbol[ev.Symbol] = append(bySymbol[ev.Symbol], ev)
	}
}

// sortByRecency orders a symbol's quarters newest first.
//
// 두 출처의 날짜 축이 다르다 — 캘린더 행은 **발표일**, 이력 행은 **회계기간
// 종료일**이다. 섞어 정렬해도 되는 이유는 간격 때문이다: 발표는 기간 종료 뒤
// 한 달 안팎에 나오고 분기는 석 달 간격이라, 둘을 같은 축에 놓아도 분기 순서가
// 뒤집히지 않는다. 같은 분기가 두 출처에서 오는 경우는 이미 위에서 하나로 접혔다.
func sortByRecency(events []model.EarningsEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		return recencyAnchor(events[i]) > recencyAnchor(events[j])
	})
}

// recencyAnchor picks whichever date the row actually has ("2006-01-02", so
// 문자열 비교가 곧 날짜 비교다).
func recencyAnchor(ev model.EarningsEvent) string {
	if ev.Date != "" {
		return ev.Date
	}
	return ev.PeriodEnd
}

func trimEvents(events []model.EarningsEvent, limit int) []model.EarningsEvent {
	if len(events) > limit {
		events = events[:limit]
	}
	if events == nil {
		return []model.EarningsEvent{}
	}
	return events
}
