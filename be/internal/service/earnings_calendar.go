package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/redis/go-redis/v9"
)

// EarningsCalendarService serves the 실적 캘린더 화면 (읽기 전용).
//
// 적재는 워커(EarningsIngestService)만 한다. 이 서비스가 조회 중에 FnGuide나
// Finnhub을 부르는 일은 없다 — 조회 경로에서 외부 HTTP를 하지 않는다는 규칙
// (AGENT_STATE 응답 속도 원칙 #2)을 지킨다.
type EarningsCalendarService struct {
	events *repository.EarningsEventRepo
	redis  *redis.Client
}

func NewEarningsCalendarService(events *repository.EarningsEventRepo, redisClient *redis.Client) *EarningsCalendarService {
	return &EarningsCalendarService{events: events, redis: redisClient}
}

const (
	// 추정 발표일은 계산이 싸지 않다(작년 한 달치를 읽어 키를 맞춰 본다).
	// 하루 한 번 바뀌면 충분하다 — 재료가 배치로만 들어온다.
	earningsEstimateCacheTTL = 12 * time.Hour
	// 키에 **오늘 날짜**가 들어간다 — 결과가 "지난 추정은 뺀다" 규칙에 따라
	// 오늘에 의존하므로, 날짜 없이 캐시하면 자정을 넘긴 뒤에도 어제 기준 목록이
	// TTL 동안 남는다.
	earningsEstimateKeyFmt = "earnings:estimate:v2:%s:%s:%s"

	// 조회 구간 상한. 월 그리드 한 칸(31일)이 최대 단위다. 상한이 없으면
	// `from=2000-01-01&to=2030-12-31` 한 방으로 표 전체를 뜰 수 있다.
	EarningsRangeMaxDays = 31
	// 심볼 필터가 있을 때의 상한. 목록이 이미 100종목으로 묶여 있어 넓게 봐도
	// 스캔이 터지지 않는다 — 레일의 「내 종목 최근 발표」가 과거 2주~미래 1달을 본다.
	EarningsSymbolRangeMaxDays = 120
	// 「내 종목」 필터의 심볼 상한. 관심+보유를 합쳐도 이보다 많은 사용자는
	// 캘린더에서 전체를 보는 편이 낫다.
	EarningsSymbolsMax = 100
)

// Calendar returns detail rows for a window, with 국내 추정 발표일을 합쳐 준다.
//
// `limit`은 **응답 크기를 위한 것이 아니라 화면을 위한 것**이다. 8월 한 달이
// 3,089행이고 그대로 내리면 gzip 136KB에 DOM 3천 줄이 된다 — 그만큼을 스크롤로
// 읽는 사람은 없다. 총 건수(두 번째 반환값)를 함께 주어 화면이 「3,089건 중
// 300건」이라고 정확히 말할 수 있게 한다.
func (s *EarningsCalendarService) Calendar(
	ctx context.Context, from, to time.Time, market string, symbols []string, limit int,
) ([]model.EarningsEvent, int, error) {
	if s == nil || s.events == nil {
		return []model.EarningsEvent{}, 0, nil
	}
	stored, err := s.events.ListRange(ctx, from, to, market, symbols)
	if err != nil {
		return nil, 0, err
	}

	// 추정은 국내에만 있다. market=us로 좁혀 보는 사람에게 국내 추정을 섞으면
	// 필터가 거짓말을 한다.
	if market != string(model.AssetUS) {
		estimated, err := s.EstimatedDomestic(ctx, from, to)
		if err == nil {
			stored = append(stored, filterEstimated(estimated, symbols)...)
		}
	}

	sort.SliceStable(stored, func(i, j int) bool {
		if stored[i].Date != stored[j].Date {
			return stored[i].Date < stored[j].Date
		}
		return stored[i].Symbol < stored[j].Symbol
	})
	// 연결/개별·해외 회계연도 중복을 접고, 어제보다 오래된 해외 「예정」을
	// 빼는 것은 **자르기 전**이다. 뒤에 하면 300행 안에 같은 회사가 두 번
	// 들어오거나, 히트맵은 줄었는데 목록은 잔류 예정이 남는다.
	stored = model.CollapseCalendarEvents(stored)

	total := len(stored)
	// 자르는 것은 정렬 **뒤**다. 앞에서 자르면 날짜순의 앞부분이 아니라 DB가
	// 돌려준 순서의 앞부분이 남아, 「이 달 앞쪽 300건」이 아니게 된다.
	if limit > 0 && len(stored) > limit {
		stored = stored[:limit]
	}
	if stored == nil {
		stored = []model.EarningsEvent{}
	}
	// 반응·시가총액은 **자른 뒤에** 붙인다 — 내보내지 않을 3,000행의 종가를
	// 받아 올 이유가 없다.
	s.AttachReactions(ctx, stored)
	s.AttachMarketCaps(ctx, stored)
	return stored, total, nil
}

// AttachMarketCaps fills 발행주식수 × 최근 종가 for the rows being returned.
//
// 표에서 「무엇이 큰 발표인가」로 줄을 세우는 유일한 기준이라 목록 응답에 실어야
// 한다. 심볼을 모아 **한 쿼리**로 받는다(항목당 I/O 금지).
func (s *EarningsCalendarService) AttachMarketCaps(ctx context.Context, events []model.EarningsEvent) {
	if s == nil || s.events == nil || len(events) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(events))
	symbols := make([]string, 0, len(events))
	for _, ev := range events {
		if _, dup := seen[ev.Symbol]; dup {
			continue
		}
		seen[ev.Symbol] = struct{}{}
		symbols = append(symbols, ev.Symbol)
	}
	caps, err := s.events.MarketCapsFor(ctx, symbols)
	if err != nil {
		// 시가총액은 부가 정보다. 못 구하면 그 열만 비고 표는 그대로 나간다.
		return
	}
	for i := range events {
		if cap, ok := caps[events[i].Symbol]; ok {
			value := cap
			events[i].MarketCap = &value
		}
	}
}

// Stats aggregates a window into the 상단 스코어보드.
//
// 목록과 같은 재료를 쓰되 **상세 행을 내보내지 않는다** — 스코어보드는 숫자 몇
// 개라 상세를 실어 보내면 같은 데이터가 화면에 두 번 온다.
func (s *EarningsCalendarService) Stats(
	ctx context.Context, from, to time.Time, market string,
) (model.EarningsStats, error) {
	if s == nil || s.events == nil {
		return model.EarningsStats{}, nil
	}
	stored, err := s.events.ListRange(ctx, from, to, market, nil)
	if err != nil {
		return model.EarningsStats{}, err
	}
	if market != string(model.AssetUS) {
		if estimated, err := s.EstimatedDomestic(ctx, from, to); err == nil {
			stored = append(stored, estimated...)
		}
	}
	stored = model.CollapseCalendarEvents(stored)
	// 랭킹과 **같은** 유의성 하한을 쓴다. 다르게 두면 스코어보드의 「최대 상회」와
	// 서프라이즈 목록 1위가 서로 다른 종목이 된다.
	return model.BuildEarningsStats(stored, materialSurprise), nil
}

// Summary counts announcements per calendar day — the month heatmap's payload.
func (s *EarningsCalendarService) Summary(
	ctx context.Context, from, to time.Time, market string,
) ([]model.EarningsDaySummary, error) {
	if s == nil || s.events == nil {
		return []model.EarningsDaySummary{}, nil
	}

	// SQL 집계만 쓰면 연결·개별·해외 회계연도 중복이 같은 날 두 번 잡히고,
	// 어제보다 오래된 해외 「예정」 잔류도 칸을 부풀린다. 상세와 같은 collapse
	// 규칙을 적용한 뒤 날짜별로 센다 — 구간 상한(31일)이라 행 수는 감당 가능하다.
	rows, err := s.events.ListRange(ctx, from, to, market, nil)
	if err != nil {
		return nil, err
	}
	if market != string(model.AssetUS) {
		estimated, err := s.EstimatedDomestic(ctx, from, to)
		if err == nil {
			rows = append(rows, estimated...)
		}
	}
	rows = model.CollapseCalendarEvents(rows)

	byDay := map[string]*model.EarningsDaySummary{}
	for _, ev := range rows {
		cell := byDay[ev.Date]
		if cell == nil {
			cell = &model.EarningsDaySummary{Date: ev.Date}
			byDay[ev.Date] = cell
		}
		switch ev.DateStatus {
		case model.EarningsDateScheduled:
			cell.Scheduled++
		case model.EarningsDateEstimated:
			cell.Estimated++
		default:
			cell.Confirmed++
		}
		// 달력 칸 색이 될 상회 비율. 랭킹과 **같은 유의성 하한**을 쓴다 —
		// 컨센서스가 1억원인 종목이 그날의 색을 정하면 안 된다.
		if ev.Surprise != nil && ev.Surprise.Pct != nil && materialSurprise(*ev.Surprise) {
			cell.Compared++
			if *ev.Surprise.Pct >= 0 {
				cell.Beats++
			}
		}
	}

	// 빈 날도 행으로 낸다. 히트맵은 격자라 빠진 날을 클라이언트가 다시 채워야
	// 하는데, 그러면 "0건"과 "응답에 없음"을 화면이 구분하려 들게 된다.
	out := make([]model.EarningsDaySummary, 0, 31)
	loc := calendar.LocationFor(calendar.ScheduleKRX)
	day := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	end := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, loc)
	for ; !day.After(end); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		if cell := byDay[key]; cell != nil {
			out = append(out, *cell)
			continue
		}
		out = append(out, model.EarningsDaySummary{Date: key})
	}
	return out, nil
}

// 서프라이즈 랭킹의 **유의성 하한**.
//
// 이 값이 없으면 랭킹이 쓸모없어진다. 실측에서 1위가 **+5,498%**였는데, 영업이익
// 컨센서스가 1억원이었고 발표가 56억원이었다 — 산술은 맞지만 「시장을 놀라게 한
// 발표」가 아니라 기준선이 0에 가까웠을 뿐이다. 이런 행이 상위를 전부 차지하면
// 목록에서 정작 알아야 할 대형주 서프라이즈가 밀려난다.
//
// 하한 아래 행도 캘린더 본문에는 서프라이즈와 함께 그대로 나온다 — 감추는 것이
// 아니라 **크기 순 줄 세우기에서만** 빼는 것이다(퍼센트 없는 라벨 행과 같은 취급).
const (
	// 영업이익 50억원. 국내 중소형주의 분기 영업이익이 100~1,000억원대라,
	// 이보다 작은 기준선에서 나온 배수는 회사 규모의 이야기지 서프라이즈가 아니다.
	surpriseMinOpIncomeKRW = 5e9
	// EPS 0.10달러. 미국 소형주의 컨센서스가 0.01달러인 경우가 흔하고, 그때
	// 0.03달러 차이가 +300%가 된다.
	surpriseMinEPSUSD = 0.10
)

// Surprises ranks announcements by how far the actual landed from consensus.
//
// order == "worst"면 하회 순, 그 밖은 상회 순.
//
// 두 가지를 랭킹에서 뺀다 — ① 퍼센트가 없는 행(흑자전환 등: 크기를 비교할 수 없다)
// ② 기준선이 유의성 하한 아래인 행(위 상수 참고). 둘 다 캘린더 본문에서는 보인다.
func (s *EarningsCalendarService) Surprises(
	ctx context.Context, from, to time.Time, market, order string, limit int,
) ([]model.EarningsEvent, error) {
	if s == nil || s.events == nil {
		return []model.EarningsEvent{}, nil
	}
	rows, err := s.events.Announced(ctx, from, to, market)
	if err != nil {
		return nil, err
	}
	rows = model.CollapseCalendarEvents(rows)
	ranked := make([]model.EarningsEvent, 0, len(rows))
	for _, ev := range rows {
		if ev.Surprise == nil || ev.Surprise.Pct == nil {
			continue
		}
		if !materialSurprise(*ev.Surprise) {
			continue
		}
		ranked = append(ranked, ev)
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := *ranked[i].Surprise.Pct, *ranked[j].Surprise.Pct
		if order == "worst" {
			return a < b
		}
		return a > b
	})
	if limit > 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}
	if ranked == nil {
		ranked = []model.EarningsEvent{}
	}
	return ranked, nil
}

// materialSurprise reports whether the consensus base is large enough that a
// percentage gap says something about the company rather than about the base.
func materialSurprise(s model.EarningsSurprise) bool {
	base := s.Estimate
	if base < 0 {
		base = -base
	}
	if s.Metric == model.EarningsMetricEPS {
		return base >= surpriseMinEPSUSD
	}
	return base >= surpriseMinOpIncomeKRW
}

// EstimatedDomestic derives 국내 예정 발표일 from last year's announcements.
//
// # 왜 추정이 필요한가
//
// 국내에는 실적발표 예정일을 주는 소스가 **없다.** DART에 예정 API가 없고,
// FnGuide 캘린더도 미래 달은 빈 배열이다(202609·202610 실측). 그래서 이 추정이
// 없으면 캘린더의 국내 미래 칸은 영원히 비어 있고, 국내 위주 사용자는 이 화면에서
// 자기 종목을 앞으로 한 번도 못 본다.
//
// # 방법과 그 한계
//
// 「작년 같은 분기를 언제 발표했나」를 1년 뒤로 옮기고 KRX 거래일로 스냅한다.
// 기업의 결산·공시 절차는 해마다 비슷한 시점에 돌아가므로 며칠 단위로는 맞지만,
// **정확한 날짜는 아니다.** 그래서 저장하지 않고, `EstimateBasis`에 근거를 실어
// 화면이 「추정 · 작년 8/3 발표」로 밝힐 수 있게 한다.
//
// 실제 발표가 들어오면 추정은 **스스로 사라진다** — 같은 (종목·회계기간)에
// 저장된 행이 있으면 여기서 빼기 때문이다. 저장해 두었다면 배치가 그 행을
// 지우러 다녀야 하고, 한 번 놓치면 같은 분기가 두 줄로 남는다.
func (s *EarningsCalendarService) EstimatedDomestic(
	ctx context.Context, from, to time.Time,
) ([]model.EarningsEvent, error) {
	if s == nil || s.events == nil {
		return nil, nil
	}
	nowKST := time.Now().In(model.KST)
	fromKey, toKey := from.Format("2006-01-02"), to.Format("2006-01-02")

	var cached []model.EarningsEvent
	cacheKey := fmt.Sprintf(earningsEstimateKeyFmt, fromKey, toKey, nowKST.Format("2006-01-02"))
	if s.cacheGet(ctx, cacheKey, &cached) {
		return cached, nil
	}

	// 작년 같은 구간. 하루를 앞뒤로 넓히는 이유는 스냅 때문이다 — 작년 8/1
	// 발표가 올해 거래일로 밀려 구간 안으로 들어올 수 있고, 반대도 있다.
	lastFrom := from.AddDate(-1, 0, -7)
	lastTo := to.AddDate(-1, 0, 7)
	prior, err := s.events.LastYearConfirmed(ctx, lastFrom, lastTo)
	if err != nil {
		return nil, err
	}
	if len(prior) == 0 {
		return nil, nil
	}
	// 작년 행도 연결·개별이 쌍으로 온다. 추정은 종목·기간당 한 줄만 내보내고
	// 연결을 우선한다 — 개별을 고르면 시장 캘린더와 종목 탭 추정일이 어긋난다.
	prior = model.CollapseDomesticBasis(prior)

	// 이미 발표된 (종목·기간)을 걸러내기 위한 키 집합. 구간을 넉넉히 잡는 것은
	// 실제 발표가 추정일보다 이르거나 늦을 수 있어서다 — 좁게 보면 8/3에 이미
	// 발표한 종목이 8/20 추정으로 한 번 더 뜬다.
	confirmed, err := s.events.ConfirmedPeriodKeys(ctx, from.AddDate(0, -3, 0), to.AddDate(0, 3, 0))
	if err != nil {
		return nil, err
	}

	// **지난 추정일은 내지 않는다.**
	//
	// 추정은 「앞으로 언제쯤 발표할까」에 답하는 값이다. 그 날짜가 이미 지났는데
	// 발표가 안 들어왔다면 그건 일어난 일이 아니라 빗나간 추측이고, 캘린더의
	// 과거 칸은 일어난 일만 말해야 한다.
	//
	// 실측에서 이게 왜 필요한지 드러났다 — 8월 14일 목록이 **173건**이었는데 실제
	// 발표는 20건이고 나머지 129건이 「추정 8/14」였다. 진짜 발표가 빗나간 추측
	// 아래 묻혔다.
	today := time.Date(nowKST.Year(), nowKST.Month(), nowKST.Day(), 0, 0, 0, 0, time.UTC)

	out := make([]model.EarningsEvent, 0, len(prior))
	seen := map[string]struct{}{}
	for _, ev := range prior {
		last, err := time.ParseInLocation("2006-01-02", ev.Date, time.UTC)
		if err != nil {
			continue
		}
		targetYear := ev.FiscalYear + 1
		key := repository.EarningsPeriodKey(ev.Symbol, targetYear, ev.FiscalPeriod)
		if _, done := confirmed[key]; done {
			continue
		}
		// 작년 같은 달에 정정까지 여러 행이 있었으면 종목·기간당 한 번만 추정한다.
		if _, dup := seen[key]; dup {
			continue
		}

		guess, basis := EstimateNextAnnouncement(last, ev.FiscalPeriod)
		if guess.Before(today) {
			continue
		}
		if guess.Before(truncateDay(from)) || guess.After(truncateDay(to)) {
			continue
		}
		seen[key] = struct{}{}

		out = append(out, model.EarningsEvent{
			Symbol:        ev.Symbol,
			Name:          ev.Name,
			Market:        string(model.AssetDomestic),
			Date:          guess.Format("2006-01-02"),
			DateStatus:    model.EarningsDateEstimated,
			FiscalYear:    targetYear,
			FiscalPeriod:  ev.FiscalPeriod,
			Basis:         ev.Basis,
			Currency:      ev.Currency,
			Source:        ev.Source,
			EstimateBasis: basis,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].Symbol < out[j].Symbol
	})
	s.cacheSet(ctx, cacheKey, out)
	return out, nil
}

// EstimateNextAnnouncement shifts a prior announcement one year forward and
// snaps it to a KRX trading day, returning the guess and its human-readable
// basis.
//
// **추정 규칙이 여기 한 곳에만 있어야 한다.** 캘린더 화면(EstimatedDomestic)과
// 종목 실적 탭(nextEstimatedEvent)이 둘 다 이 함수를 부른다. 규칙이 두 벌이면
// 같은 종목의 「다음 발표 추정」이 화면마다 다른 날짜가 되고, 사용자는 어느
// 쪽을 믿어야 할지 알 수 없다.
func EstimateNextAnnouncement(prior time.Time, period string) (time.Time, string) {
	guess := snapToKRXTradingDay(prior.AddDate(1, 0, 0))
	basis := fmt.Sprintf("작년 %s는 %d/%d 발표", period, int(prior.Month()), prior.Day())
	return guess, basis
}

// snapToKRXTradingDay moves a date forward to the next KRX trading day.
//
// 앞으로 미는 이유: 발표가 휴장일에 걸리면 기업은 보통 그 다음 영업일에 낸다.
// 상한 10일은 연휴(추석·설)에 무한히 도는 것을 막는 안전장치다.
func snapToKRXTradingDay(day time.Time) time.Time {
	for i := 0; i < 10; i++ {
		if calendar.IsTradingDay(calendar.ScheduleKRX, day) {
			return day
		}
		day = day.AddDate(0, 0, 1)
	}
	return day
}

func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// filterEstimated applies the 「내 종목」 symbol filter to derived rows. 저장된
// 행은 SQL에서 걸러지지만 추정은 메모리에서 만들어지므로 여기서 한 번 더 건다.
func filterEstimated(rows []model.EarningsEvent, symbols []string) []model.EarningsEvent {
	if len(symbols) == 0 {
		return rows
	}
	want := make(map[string]struct{}, len(symbols))
	for _, s := range symbols {
		want[s] = struct{}{}
	}
	out := make([]model.EarningsEvent, 0, len(rows))
	for _, ev := range rows {
		if _, ok := want[ev.Symbol]; ok {
			out = append(out, ev)
		}
	}
	return out
}

func (s *EarningsCalendarService) cacheGet(ctx context.Context, key string, out any) bool {
	if s.redis == nil {
		return false
	}
	raw, err := s.redis.Get(ctx, key).Bytes()
	if err != nil || len(raw) == 0 {
		return false
	}
	return json.Unmarshal(raw, out) == nil
}

func (s *EarningsCalendarService) cacheSet(ctx context.Context, key string, value any) {
	s.cacheSetTTL(ctx, key, value, earningsEstimateCacheTTL)
}

func (s *EarningsCalendarService) cacheSetTTL(ctx context.Context, key string, value any, ttl time.Duration) {
	if s.redis == nil {
		return
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = s.redis.Set(ctx, key, payload, ttl).Err()
}

// ClampEarningsRange enforces the window ceiling, keeping `from` and moving
// `to` in — 사용자가 넓게 물었을 때 빈 응답보다 앞부분이라도 주는 게 낫다.
//
// `symbolFiltered`면 상한이 훨씬 넓다. 31일 상한은 **스캔을 묶기 위한 것**인데,
// 심볼 목록(≤100)이 이미 그 일을 하고 있어 같은 제약을 두 번 걸 이유가 없다.
// 레일의 「내 종목 최근 발표」가 과거 2주~미래 1달(44일)을 봐야 해서 필요해졌다.
func ClampEarningsRange(from, to time.Time, symbolFiltered bool) (time.Time, time.Time) {
	if to.Before(from) {
		from, to = to, from
	}
	limit := EarningsRangeMaxDays
	if symbolFiltered {
		limit = EarningsSymbolRangeMaxDays
	}
	max := from.AddDate(0, 0, limit-1)
	if to.After(max) {
		to = max
	}
	return from, to
}
