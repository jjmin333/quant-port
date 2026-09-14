package sample

import (
	"context"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func now(t *testing.T) time.Time {
	t.Helper()
	return time.Date(2026, 8, 16, 3, 0, 0, 0, time.UTC)
}

// stubQuotes stands in for the market data the fixtures now read.
//
// The menus used to carry their own prices, so a test could assert "the sample
// shows a gain next to a loss" directly. With real quotes that property belongs
// to the market, not to this package — what is still ours, and what these tests
// pin down, is that each number is *derived from* the right source: 담은 뒤 from
// the close on the day the row was created, 전일·전주·전월 from the metrics the
// signed-in list uses, 목표가 from the 등록가.
type stubQuotes struct {
	current map[string]float64
	// history is keyed by closeKey(symbol, date) so one symbol can have a
	// different close on each fixture date.
	history map[string]float64
	bars    map[string][]model.Candle
	// metricsFor records which addedAt each symbol was asked about.
	askedAdded map[string]time.Time
}

func (s *stubQuotes) BatchQuoteSummaries(_ context.Context, symbols []string) []model.QuoteSummary {
	out := make([]model.QuoteSummary, 0, len(symbols))
	for _, sym := range symbols {
		price, ok := s.current[sym]
		if !ok {
			continue
		}
		out = append(out, model.QuoteSummary{Symbol: sym, CurrentPrice: price})
	}
	return out
}

func (s *stubQuotes) WatchlistMetricsFor(
	_ context.Context, item *model.QuoteSummary, addedAt time.Time,
) *model.WatchlistMetrics {
	if item == nil || item.CurrentPrice <= 0 {
		return nil
	}
	if s.askedAdded == nil {
		s.askedAdded = map[string]time.Time{}
	}
	s.askedAdded[item.Symbol] = addedAt

	// Stand-in numbers: the real implementation is exercised by the service
	// package. What matters here is that the block reaches the payload at all.
	week := item.CurrentPrice * 0.97
	month := item.CurrentPrice * 0.92
	weekRate := (item.CurrentPrice - week) / week * 100
	monthRate := (item.CurrentPrice - month) / month * 100
	item.ChangeRate = 1.5
	return &model.WatchlistMetrics{
		AddedAt:             addedAt.UTC().Format(time.RFC3339),
		ChangeRatePrevClose: 1.5,
		PrevClose:           item.CurrentPrice * 0.985,
		PrevWeekClose:       week,
		ChangeRatePrevWeek:  &weekRate,
		PrevMonthClose:      month,
		ChangeRatePrevMonth: &monthRate,
		Week52High:          item.CurrentPrice * 1.3,
		Week52Low:           item.CurrentPrice * 0.6,
	}
}

func (s *stubQuotes) DailyCloseAtOrBefore(
	_ context.Context, symbol string, cutoff time.Time,
) (float64, bool) {
	v, ok := s.history[closeKey(symbol, cutoff)]
	if !ok || v <= 0 {
		return 0, false
	}
	return v, true
}

func (s *stubQuotes) DailyBars(_ context.Context, symbol string, _ int) []model.Candle {
	return s.bars[symbol]
}

// newStub gives every fixture symbol a current price and a close on each date
// the fixtures ask about. Half the symbols are seeded to have risen since they
// were added and half to have fallen, so the direction-sensitive assertions
// below test the derivation rather than the market.
func newStub(t *testing.T, at time.Time) *stubQuotes {
	t.Helper()
	current := map[string]float64{
		"005930": 270000, "035720": 52000, "000660": 980000,
		"042660": 84300, "NVDA": 215.15, "AAPL": 268.40,
	}
	// gainers rose since purchase, losers fell.
	factor := map[string]float64{
		"005930": 0.55, "000660": 0.70, "NVDA": 0.80, // bought cheaper → gain
		"035720": 1.35, "AAPL": 1.20, "042660": 1.10, // bought dearer → loss
	}

	s := &stubQuotes{
		current: current,
		history: map[string]float64{},
		bars:    map[string][]model.Candle{},
	}

	// Every day the fixtures could reference.
	for sym, price := range current {
		for d := 0; d <= 400; d++ {
			day := ago(at, d)
			s.history[closeKey(sym, endOfDay(day))] = price * factor[sym]
		}
		// A flat series is enough: rangeSince only needs highs and lows.
		bars := make([]model.Candle, 0, 120)
		for d := 120; d >= 0; d-- {
			day := ago(at, d).In(model.KST)
			bars = append(bars, model.Candle{
				Time:  day.Format("2006-01-02"),
				High:  price * 1.05,
				Low:   price * 0.85,
				Close: price,
			})
		}
		s.bars[sym] = bars
	}
	return s
}

// A sample exists to show what the feature does, so the thing worth asserting
// is coverage of the screen's states — not row counts. If a future edit trims
// the set down to five happy-path hits, the seals, the gauge and half the
// scorecard stop having anything to draw and the demo quietly gets worse.
func TestStampsCoverEveryRenderedState(t *testing.T) {
	views := Stamps(now(t))

	var hit, miss, pending, early, opened, dueUnopened, neutral, bearish int
	for _, v := range views {
		switch v.Verdict {
		case model.VerdictHit:
			hit++
		case model.VerdictMiss:
			miss++
		case model.VerdictPending:
			pending++
		}
		if v.TargetHit != nil {
			early++
		}
		if v.Opened {
			opened++
		}
		if v.Due && !v.Opened {
			dueUnopened++
		}
		switch v.Stance {
		case model.StanceNeutral:
			neutral++
		case model.StanceBearish:
			bearish++
		}
	}

	for _, c := range []struct {
		name string
		got  int
	}{
		{"적중", hit}, {"빗나감", miss}, {"검증 중", pending},
		{"장중 조기 적중", early}, {"이미 개봉함", opened},
		{"개봉 대기", dueUnopened}, {"관망", neutral}, {"약세", bearish},
	} {
		if c.got == 0 {
			t.Errorf("샘플에 %q 상태가 하나도 없다 — 그 화면 요소는 빈 채로 보인다", c.name)
		}
	}
}

// Stance is derived from the target on the real create path. A fixture that
// hand-set a contradictory pair would show a combination the app cannot produce.
func TestStampsStanceMatchesTarget(t *testing.T) {
	for _, v := range Stamps(now(t)) {
		want := model.StanceForTarget(v.Price, v.TargetPrice)
		if v.Stance != want {
			t.Errorf("%s: stance=%q but target %v vs price %v implies %q",
				v.Symbol, v.Stance, v.TargetPrice, v.Price, want)
		}
	}
}

// The tiles are computed from the rows, so a summary that disagrees with them
// would have the header contradicting the list directly underneath it.
func TestStampSummaryAgreesWithRows(t *testing.T) {
	at := now(t)
	views := Stamps(at)
	got := StampSummary(at)

	if got.Total != len(views) {
		t.Fatalf("Total = %d, want %d", got.Total, len(views))
	}
	var judged, hits int
	for _, v := range views {
		if v.Verdict == model.VerdictHit || v.Verdict == model.VerdictMiss {
			judged++
			if v.Verdict == model.VerdictHit {
				hits++
			}
		}
	}
	if got.Judged != judged || got.Hits != hits {
		t.Fatalf("Judged/Hits = %d/%d, want %d/%d", got.Judged, got.Hits, judged, hits)
	}
	if got.HitRate == nil {
		t.Fatal("HitRate should be set — the fixture has judged stamps")
	}
	if got.AvgTargetError == nil {
		t.Fatal("AvgTargetError should be set — the scorecard's second axis would read '—'")
	}
}

// Dates are relative so the demo never looks abandoned. A fixture with a
// deadline already behind "검증 중" rows would render a countdown of D-negative.
func TestStampDatesStayRelativeToNow(t *testing.T) {
	at := now(t)
	for _, v := range Stamps(at) {
		if v.StampedAt.After(at) {
			t.Errorf("%s: stamped in the future", v.Symbol)
		}
		if v.Verdict == model.VerdictPending && !v.DueAt.After(at) {
			t.Errorf("%s: pending but the deadline has passed", v.Symbol)
		}
		if v.Due && v.TargetHit == nil && v.DueAt.After(at) {
			t.Errorf("%s: marked due but the deadline is still ahead", v.Symbol)
		}
	}
}

// Every menu has to return something: an empty fixture would land the visitor
// back on the empty screen the sample exists to replace.
func TestEveryMenuFixtureIsPopulated(t *testing.T) {
	at := now(t)
	ctx := context.Background()
	q := newStub(t, at)

	if len(Stamps(at)) == 0 {
		t.Error("stamps fixture is empty")
	}
	if len(Watchlist(ctx, at, q)) == 0 {
		t.Error("watchlist fixture is empty")
	}
	if len(Holdings(ctx, at, q)) == 0 {
		t.Error("holdings fixture is empty")
	}
	if len(Journal(ctx, at, q)) == 0 {
		t.Error("journal fixture is empty")
	}
	if len(Targets(ctx, at, q)) == 0 {
		t.Error("targets fixture is empty")
	}
}

// 이 테스트가 잡는 것: 「전주」·「전월」 열이 모든 행에서 —로 뜨던 버그. 원인은 DB에
// 종가가 없어서가 아니라 예시 payload가 그 블록을 아예 안 실어 보낸 것이었다.
func TestWatchlistCarriesPeriodMetrics(t *testing.T) {
	at := now(t)
	q := newStub(t, at)
	rows := Watchlist(context.Background(), at, q)
	if len(rows) == 0 {
		t.Fatal("watchlist fixture is empty")
	}
	for _, w := range rows {
		if w.Metrics == nil {
			t.Fatalf("%s: watchlistMetrics 없음 — 전일·전주·전월이 전부 —로 뜬다", w.Symbol)
		}
		if w.Metrics.ChangeRatePrevWeek == nil {
			t.Errorf("%s: 전주 등락률이 비었다", w.Symbol)
		}
		if w.Metrics.ChangeRatePrevMonth == nil {
			t.Errorf("%s: 전월 등락률이 비었다", w.Symbol)
		}
		if w.Metrics.AddedAt != w.CreatedAt {
			t.Errorf("%s: metrics.addedAt=%q, row.createdAt=%q — 둘이 갈리면 「담은 뒤」 기준일이 두 개가 된다",
				w.Symbol, w.Metrics.AddedAt, w.CreatedAt)
		}
		if w.Week52High <= 0 || w.Week52Low <= 0 {
			t.Errorf("%s: 52주 레인지가 비었다 — 레인지 바가 안 그려진다", w.Symbol)
		}
	}
}

// 현재가·담은 가격이 픽스처 상수가 아니라 시세·과거 종가에서 와야 한다.
func TestWatchlistPricesComeFromMarketData(t *testing.T) {
	at := now(t)
	q := newStub(t, at)
	for _, w := range Watchlist(context.Background(), at, q) {
		if got, want := w.Price, q.current[w.Symbol]; got != want {
			t.Errorf("%s: price=%v, 시세는 %v — 현재가가 시세에서 오지 않았다", w.Symbol, got, want)
		}
		added, _ := time.Parse(time.RFC3339, w.CreatedAt)
		want := q.history[closeKey(w.Symbol, endOfDay(added))]
		if w.AddedPrice != want {
			t.Errorf("%s: addedPrice=%v, 담은 날 종가는 %v", w.Symbol, w.AddedPrice, want)
		}
	}
}

// 담은 뒤·평단은 방향이 갈려야 손익 색이 양쪽 다 나온다. 시세는 통제할 수 없으므로
// 스텁으로 양쪽을 만들고, **파생이 방향을 옳게 옮기는지**를 본다.
func TestHoldingsAndWatchlistDeriveBothDirections(t *testing.T) {
	at := now(t)
	ctx := context.Background()
	q := newStub(t, at)

	var gain, loss int
	for _, h := range Holdings(ctx, at, q) {
		if h.AvgPrice == nil {
			t.Fatalf("%s: holdings fixture needs an average price", h.Symbol)
		}
		if h.Price > *h.AvgPrice {
			gain++
		} else if h.Price < *h.AvgPrice {
			loss++
		}
	}
	if gain == 0 || loss == 0 {
		t.Errorf("보유 파생이 이익 %d건 손실 %d건 — 평단이 매수일 종가를 안 따라간다", gain, loss)
	}

	var up, down int
	for _, w := range Watchlist(ctx, at, q) {
		if w.AddedPrice == 0 {
			t.Errorf("%s: 담은 가격이 없으면 「담은 뒤」 열이 —로 남는다", w.Symbol)
		}
		if w.Price > w.AddedPrice {
			up++
		} else if w.Price < w.AddedPrice {
			down++
		}
	}
	if up == 0 || down == 0 {
		t.Errorf("관심 파생이 상승 %d건 하락 %d건 — 「담은 뒤」가 담은 날 종가를 안 따라간다", up, down)
	}
}

// 평단은 **산 날**의 종가여야 한다. 아무 날짜나 쓰면 담은 뒤 수익률이 조용히 틀린다.
func TestHoldingsAveragePriceIsTheCloseOnTheBuyDate(t *testing.T) {
	at := now(t)
	q := newStub(t, at)
	bySymbol := map[string]holdingSeed{}
	for _, s := range holdingSeeds {
		bySymbol[s.symbol] = s
	}
	for _, h := range Holdings(context.Background(), at, q) {
		seed := bySymbol[h.Symbol]
		want := q.history[closeKey(h.Symbol, endOfDay(ago(at, seed.boughtDaysAgo)))]
		if h.AvgPrice == nil || *h.AvgPrice != want {
			t.Errorf("%s: avgPrice=%v, %d일 전 종가는 %v", h.Symbol, h.AvgPrice, seed.boughtDaysAgo, want)
		}
	}
}

// 고정 메모가 없으면 「고정한 메모」 카드가 샘플에서도 비어 있다.
func TestJournalHasPinnedAndLinkedNotes(t *testing.T) {
	var pinned, linked int
	for _, n := range Journal(context.Background(), now(t), newStub(t, now(t))) {
		if n.Pinned {
			pinned++
		}
		if n.Symbol != "" {
			linked++
		}
	}
	if pinned == 0 {
		t.Error("고정한 메모가 없다 — 레일의 고정 카드가 빈 채로 보인다")
	}
	if linked == 0 {
		t.Error("종목에 연결된 메모가 없다 — 종목별 메모 카드가 빈 채로 보인다")
	}
}

// 「작성 당시 → 현재」 띠는 양쪽 끝이 다 있어야 그려진다.
func TestJournalLinkedNotesHaveBothPriceEnds(t *testing.T) {
	at := now(t)
	q := newStub(t, at)
	for _, n := range Journal(context.Background(), at, q) {
		if n.Symbol == "" {
			continue
		}
		if n.PriceAtWrite == nil || *n.PriceAtWrite <= 0 {
			t.Errorf("%s: 작성 당시 가격이 없다", n.Symbol)
		}
		if n.Price <= 0 {
			t.Errorf("%s: 현재가가 없다", n.Symbol)
		}
		if n.PriceDate == "" {
			t.Errorf("%s: 작성 당시 가격의 날짜가 없다", n.Symbol)
		}
	}
}

// 목표가·손절가는 등록가에서 파생된다 — 고정 숫자를 쓰면 실제 시세가 그 위로
// 지나가는 순간 모든 행이 「도달」이 되고 다른 상태가 화면에서 사라진다.
func TestTargetLevelsDeriveFromTheBasePrice(t *testing.T) {
	at := now(t)
	q := newStub(t, at)
	bySymbol := map[string]targetSeed{}
	for _, s := range targetSeeds {
		bySymbol[s.symbol] = s
	}

	var withStop, withSecond int
	for _, tg := range Targets(context.Background(), at, q) {
		seed := bySymbol[tg.Symbol]
		if tg.BasePrice == nil || *tg.BasePrice <= 0 {
			t.Fatalf("%s: 등록가가 없다", tg.Symbol)
		}
		base := *tg.BasePrice
		if want := q.history[closeKey(tg.Symbol, endOfDay(ago(at, seed.daysAgo)))]; base != want {
			t.Errorf("%s: basePrice=%v, 등록일 종가는 %v", tg.Symbol, base, want)
		}
		if want := roundTick(base*(1+seed.targetPct/100), seed.currency); tg.TargetPrice != want {
			t.Errorf("%s: targetPrice=%v, 등록가 %v의 %+.0f%%는 %v",
				tg.Symbol, tg.TargetPrice, base, seed.targetPct, want)
		}
		if tg.StopPrice != nil {
			withStop++
			if *tg.StopPrice >= base {
				t.Errorf("%s: 손절가 %v가 등록가 %v보다 높다", tg.Symbol, *tg.StopPrice, base)
			}
		}
		if tg.TargetPrice2 != nil {
			withSecond++
			if *tg.TargetPrice2 <= tg.TargetPrice {
				t.Errorf("%s: 2차 목표 %v가 1차 %v 이하다", tg.Symbol, *tg.TargetPrice2, tg.TargetPrice)
			}
		}
	}
	if withStop == 0 {
		t.Error("손절가가 있는 행이 없다 — 진행축의 손절 구간이 안 그려진다")
	}
	if withSecond == 0 {
		t.Error("2차 목표가 있는 행이 없다 — 2단 목표 표시가 안 그려진다")
	}
	// 손절가 없는 행도 하나는 있어야 「선택」 필드가 비었을 때의 모양이 보인다.
	var withoutStop bool
	for _, s := range targetSeeds {
		if s.stopPct == 0 {
			withoutStop = true
		}
	}
	if !withoutStop {
		t.Error("손절가 없는 행이 없다 — 그 상태의 화면을 못 보여준다")
	}
}
