package model

import (
	"strconv"
	"time"
)

// CollapseDomesticBasis keeps one domestic row per (symbol, fiscalYear,
// fiscalPeriod, date). FnGuide gives both 연결 and 개별 for the same
// announcement; the market calendar treats them as one event and prefers 연결.
//
// 해외·추정 행은 건드리지 않는다 — 미국은 basis가 비어 있고, 추정은 basis를
// 근거 행에서 물려 받지만 시장 캘린더에서 개별·연결 추정이 동시에 생기지
// 않도록 EstimatedDomestic이 이미 한 줄만 만든다.
//
// # 진 행은 버리지 않고 접어 둔다
//
// 처음 구현은 대표만 남기고 나머지를 **버렸다**. 그런데 두 기준이 서로 다른 이야기를
// 할 때가 있다 — 미래에셋생명 2026 2Q는 연결 **+54.6%**, 개별 **−60.8%**였다(같은
// 모양 338건). 대표만 남기면 「왜 이 숫자냐」에 답할 근거가 화면에서 사라지고,
// 지주회사처럼 **개별이 오히려 본질인** 경우도 있다. 그래서 대표의 `Alternates`에
// 넣어 두고, 화면은 기본으로 접었다가 눌러서 펼친다.
//
// 키에 `Date`가 들어가는 것은 의도적이다 — 같은 분기를 **다른 날** 두 번 발표한
// 것은 별개의 사건이고, 합치면 하나가 화면에서 사라진다.
func CollapseDomesticBasis(events []EarningsEvent) []EarningsEvent {
	if len(events) == 0 {
		return events
	}
	type slot struct {
		ev    EarningsEvent
		order int
	}
	best := make(map[string]slot, len(events))
	for i, ev := range events {
		if !collapsibleDomestic(ev) {
			continue
		}
		key := domesticBasisCollapseKey(ev)
		cur, ok := best[key]
		if !ok || domesticBasisRank(ev.Basis) > domesticBasisRank(cur.ev.Basis) {
			best[key] = slot{ev: ev, order: i}
		}
	}
	if len(best) == 0 {
		return events
	}

	// 대표가 아닌 행을 그 대표의 Alternates로 모은다. 원래 순서를 유지하려면
	// 대표가 등장한 자리에 그대로 놓아야 하므로 두 번 훑는다.
	alternates := make(map[string][]EarningsEvent)
	for i, ev := range events {
		if !collapsibleDomestic(ev) {
			continue
		}
		key := domesticBasisCollapseKey(ev)
		if slot, ok := best[key]; ok && slot.order != i {
			// 접히는 행이 들고 있던 대안까지 평평하게 — 트리가 되면 화면이
			// 무한히 펼친다.
			alternates[key] = append(alternates[key], flattenAlternates(ev)...)
		}
	}

	out := make([]EarningsEvent, 0, len(events))
	for i, ev := range events {
		if !collapsibleDomestic(ev) {
			out = append(out, ev)
			continue
		}
		key := domesticBasisCollapseKey(ev)
		slot, ok := best[key]
		if !ok || slot.order != i {
			continue
		}
		if extra := alternates[key]; len(extra) > 0 {
			ev.Alternates = append(flattenAlternates(ev)[1:], extra...)
		}
		out = append(out, ev)
	}
	return out
}

// collapsibleDomestic — 국내 확정·예정 행만 병합 대상이다.
//
// 추정 행을 빼는 이유: 근거(EstimateBasis)를 잃을 수 있고, EstimatedDomestic이
// 이미 (종목·기간)당 한 줄만 만든다.
func collapsibleDomestic(ev EarningsEvent) bool {
	return ev.Market == string(AssetDomestic) && ev.DateStatus != EarningsDateEstimated
}

// flattenAlternates returns [self, ...its alternates] with the nesting removed.
func flattenAlternates(ev EarningsEvent) []EarningsEvent {
	nested := ev.Alternates
	ev.Alternates = nil
	out := make([]EarningsEvent, 0, len(nested)+1)
	out = append(out, ev)
	for _, alt := range nested {
		alt.Alternates = nil
		out = append(out, alt)
	}
	return out
}

func domesticBasisCollapseKey(ev EarningsEvent) string {
	return ev.Symbol + "|" + ev.Date + "|" +
		strconv.Itoa(ev.FiscalYear) + "|" + ev.FiscalPeriod
}

func domesticBasisRank(basis string) int {
	switch basis {
	case "연결":
		return 2
	case "개별":
		return 1
	default:
		return 0
	}
}

// CollapseCalendarEvents is the calendar/list read path: domestic 연결/개별,
// overseas same-print fiscal-year dups, then stale US 「예정」 drop. Call this
// instead of any step alone so the heatmap, surprises, and table count the
// same events.
func CollapseCalendarEvents(events []EarningsEvent) []EarningsEvent {
	return DropStaleOverseasScheduled(
		CollapseOverseasFiscalYearDup(CollapseDomesticBasis(events)),
		time.Now(),
	)
}

// DropStaleOverseasScheduled hides US rows that are still 「예정」 after their
// KST calendar day has passed.
//
// # 왜 숨기는가 (2026-08-21)
//
// `dateStatus`는 날짜가 아니라 Finnhub `epsActual`/`revenueActual` 유무다.
// 소스가 숫자를 안 주면 지난 날짜도 「예정」으로 남는다. 실측 8/21 히트맵:
// 8/19는 발표 26·예정 37, 8/12는 발표 131·예정 140. 그중 상당수는 이미 다른
// 날에 발표됐거나(AARD 8/11 발표·Finnhub는 8/19 예정) 실적치를 영영 안 준다.
// 「예정」은 *아직 안 나왔다*는 뜻이어야 하는데, 이틀 전 칸에 그 배지를 달면
// 캘린더가 거짓 일정을 말한다. 배지만 「데이터 없음」으로 바꿔도 시총 순 표의
// 절반이 빈 행이라 가시성을 해친다. 그래서 **조회에서 뺀다**. DB는 그대로 둔다
// — 적재가 나중에 실적치를 채우면 확정으로 다시 나온다.
//
// # KST 표시일 기준 (2026-08-27)
//
// DB `event_date`는 미국 거래일 그대로지만, 여기서는 OverseasDisplayDate로
// amc/dmh를 +1한 **한국 캘린더일**과 KST 오늘을 비교한다. 오늘 칸의 장후 발표는
// 오늘까지 「예정」으로 남긴다. Finnhub이 숫자를 안 준 채로 어제 칸에만 남는
// 잔류는 display date < KST today 로 걸러진다.
//
// 국내·확정·추정은 건드리지 않는다. 국내는 예정이 없고, 확정은 이미 발표된
// 행이며, 추정은 작년 동기로 우리가 만든 국내 전용 상태다.
func DropStaleOverseasScheduled(events []EarningsEvent, now time.Time) []EarningsEvent {
	if len(events) == 0 {
		return events
	}
	floor := overseasScheduledKeepFrom(now)
	out := make([]EarningsEvent, 0, len(events))
	for _, ev := range events {
		if staleOverseasScheduled(ev, floor) {
			continue
		}
		out = append(out, ev)
	}
	return out
}

// overseasScheduledKeepFrom is KST today. Scheduled rows with display date >=
// today stay visible; older scheduled rows drop.
func overseasScheduledKeepFrom(now time.Time) string {
	kst := now.In(KST)
	today := time.Date(kst.Year(), kst.Month(), kst.Day(), 0, 0, 0, 0, KST)
	return today.Format("2006-01-02")
}

func staleOverseasScheduled(ev EarningsEvent, floor string) bool {
	if ev.Market != string(AssetUS) || ev.DateStatus != EarningsDateScheduled {
		return false
	}
	display := OverseasDisplayDate(ev.Date, ev.Session)
	if display == "" {
		return false
	}
	return display < floor
}

// CollapseOverseasFiscalYearDup keeps one US row per (symbol, date, fiscalPeriod).
//
// Finnhub duplicates some prints with a second fiscal year (Target 2026 2Q and
// 2027 2Q on 2026-08-19, same actuals). Unique key (symbol, fiscal_year,
// fiscal_period, basis) stores both. That is not two announcements — it is one
// print with a mis-tagged year. The copy with estimates is the real label;
// the other is dropped, not folded into Alternates (unlike 연결/개별).
//
// Same-day 2Q and FY stay two rows — those are distinct periods.
func CollapseOverseasFiscalYearDup(events []EarningsEvent) []EarningsEvent {
	if len(events) == 0 {
		return events
	}
	type slot struct {
		ev    EarningsEvent
		order int
	}
	best := make(map[string]slot, len(events))
	for i, ev := range events {
		if !collapsibleOverseas(ev) {
			continue
		}
		key := overseasFYCollapseKey(ev)
		cur, ok := best[key]
		if !ok {
			best[key] = slot{ev: ev, order: i}
			continue
		}
		best[key] = slot{ev: mergeOverseasFYDup(cur.ev, ev), order: cur.order}
	}
	if len(best) == 0 {
		return events
	}

	out := make([]EarningsEvent, 0, len(events))
	for i, ev := range events {
		if !collapsibleOverseas(ev) {
			out = append(out, ev)
			continue
		}
		slot, ok := best[overseasFYCollapseKey(ev)]
		if !ok || slot.order != i {
			continue
		}
		out = append(out, slot.ev)
	}
	return out
}

func collapsibleOverseas(ev EarningsEvent) bool {
	return ev.Market == string(AssetUS)
}

func overseasFYCollapseKey(ev EarningsEvent) string {
	return ev.Symbol + "|" + ev.Date + "|" + ev.FiscalPeriod
}

func mergeOverseasFYDup(a, b EarningsEvent) EarningsEvent {
	winner, loser := a, b
	if preferOverseasFYDup(b, a) {
		winner, loser = b, a
	}
	out := winner
	if out.Session == "" {
		out.Session = loser.Session
	}
	if loser.DateStatus == EarningsDateConfirmed {
		out.DateStatus = EarningsDateConfirmed
	}
	out.Revenue = mergeEarningsMetric(out.Revenue, loser.Revenue)
	out.OpIncome = mergeEarningsMetric(out.OpIncome, loser.OpIncome)
	out.NetIncome = mergeEarningsMetric(out.NetIncome, loser.NetIncome)
	out.EPS = mergeEarningsMetric(out.EPS, loser.EPS)
	out.Surprise = out.HeadlineSurprise()
	out.RevenueSurprise = ComputeSurprise(EarningsMetricRevenue, out.Revenue)
	return out
}

// preferOverseasFYDup reports whether cand should replace cur as the displayed
// row. Estimates beat actuals-only (the mis-year copy), then more actuals,
// then the higher fiscal year.
func preferOverseasFYDup(cand, cur EarningsEvent) bool {
	if c, u := overseasEstimateCount(cand), overseasEstimateCount(cur); c != u {
		return c > u
	}
	if c, u := overseasActualCount(cand), overseasActualCount(cur); c != u {
		return c > u
	}
	return cand.FiscalYear > cur.FiscalYear
}

func overseasEstimateCount(ev EarningsEvent) int {
	n := 0
	if ev.EPS.Estimate != nil {
		n++
	}
	if ev.Revenue.Estimate != nil {
		n++
	}
	return n
}

func overseasActualCount(ev EarningsEvent) int {
	n := 0
	if ev.EPS.Actual != nil {
		n++
	}
	if ev.Revenue.Actual != nil {
		n++
	}
	return n
}

func mergeEarningsMetric(dst, src EarningsMetric) EarningsMetric {
	if dst.Estimate == nil {
		dst.Estimate = src.Estimate
	}
	if dst.Actual == nil {
		dst.Actual = src.Actual
	}
	if dst.QoQ == nil {
		dst.QoQ = src.QoQ
	}
	return dst
}
