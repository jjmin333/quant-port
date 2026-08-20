package service

import (
	"context"
	"fmt"
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
	earningsHighlightsTTL    = 30 * time.Minute
	earningsHighlightsKeyFmt = "earnings:highlights:v2:%s:%s"

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

	nowKST := time.Now().In(model.KST)
	today := time.Date(nowKST.Year(), nowKST.Month(), nowKST.Day(), 0, 0, 0, 0, time.UTC)

	cacheKey := fmt.Sprintf(earningsHighlightsKeyFmt, market, today.Format("2006-01-02"))
	var cached EarningsHighlights
	if s.cacheGet(ctx, cacheKey, &cached) {
		return cached, nil
	}

	out.Big = s.upcomingBig(ctx, today, market)
	out.Divergent = s.divergentAnnouncements(ctx, today, market)
	out.Consistent = s.consistentBeaters(ctx, today, market)

	s.cacheSetTTL(ctx, cacheKey, out, earningsHighlightsTTL)
	return out, nil
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
	rows, err := s.events.ListRange(ctx, today, to, market, nil)
	if err != nil {
		return []model.EarningsEvent{}
	}
	if market != string(model.AssetUS) {
		if estimated, err := s.EstimatedDomestic(ctx, today, to); err == nil {
			rows = append(rows, estimated...)
		}
	}
	rows = model.CollapseCalendarEvents(rows)

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
	rows, err := s.events.Announced(ctx, from, today, market)
	if err != nil {
		return []model.EarningsEvent{}
	}
	rows = model.CollapseCalendarEvents(rows)
	s.AttachReactions(ctx, rows)

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
	rows, err := s.events.Announced(ctx, from, today, market)
	if err != nil {
		return []model.EarningsConsistency{}
	}
	rows = model.CollapseCalendarEvents(rows)

	bySymbol := map[string][]model.EarningsEvent{}
	for _, ev := range rows {
		if ev.Surprise == nil || ev.Surprise.Pct == nil || !materialSurprise(*ev.Surprise) {
			continue
		}
		bySymbol[ev.Symbol] = append(bySymbol[ev.Symbol], ev)
	}

	out := make([]model.EarningsConsistency, 0, len(bySymbol))
	for symbol, events := range bySymbol {
		// Announced는 발표일 내림차순이라 앞에서 8개가 최근 8분기다.
		hist := model.ComputeHitHistory(events, earningsHitHistoryQuarters)
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

func trimEvents(events []model.EarningsEvent, limit int) []model.EarningsEvent {
	if len(events) > limit {
		events = events[:limit]
	}
	if events == nil {
		return []model.EarningsEvent{}
	}
	return events
}
