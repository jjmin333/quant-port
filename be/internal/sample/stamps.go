package sample

import (
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// Stamps returns the demo stamp set.
//
// The point of a sample is not "some rows" — it is that every state the screen
// can render shows up at least once, because a visitor judges the feature by
// what they can see it do. So the set deliberately contains a hit, a miss, an
// early hit closed by an intraday touch, two still counting down, and one
// already opened. Verdict seals, the target gauge, the 개봉 대기 queue and the
// scorecard tabs all have something to draw.
func Stamps(now time.Time) []model.StampView {
	views := []model.StampView{
		// 1. 적중 — 목표에 거의 정확히 닿았다. 성적표의 "±3% 이내" 칸을 채운다.
		stampView(stampSeed{
			id: 1, symbol: "005930", name: "삼성전자", stampedDaysAgo: 40, dueDaysAgo: 10,
			price: 74200, target: 82000, settled: 81300, current: 81300,
			score: 64, currentScore: 68, grade: "C", conviction: 4,
			horizon: model.Horizon1M, note: "HBM 수주 기대 + 외국인 순매수 전환",
			benchReturn: 3.5, peak: 11.0, trough: -1.2,
			news: "삼성전자, HBM4 양산 준비 본격화", publisher: "연합뉴스",
		}, now),

		// 2. 빗나감 — 약세로 봤는데 올랐다. 방향 판정이 무엇을 뜻하는지 한 줄로 보인다.
		stampView(stampSeed{
			id: 2, symbol: "000660", name: "SK하이닉스", stampedDaysAgo: 40, dueDaysAgo: 10,
			price: 198000, target: 185000, settled: 214500, current: 214500,
			score: 71, currentScore: 69, grade: "B", conviction: 3,
			horizon: model.Horizon1M, note: "고점 부담 — 단기 조정 예상",
			benchReturn: 3.5, peak: 9.9, trough: -3.4, held: true,
			news: "SK하이닉스, 엔비디아向 HBM 공급 확대", publisher: "한국경제",
		}, now),

		// 3. 조기 적중 — 장중에 목표를 찍고 되돌아왔다. 이 규칙이 없으면 빗나감이 된다.
		stampView(stampSeed{
			id: 3, symbol: "042660", name: "한화오션", stampedDaysAgo: 62, dueDaysAgo: 4,
			price: 58400, target: 68000, settled: 64100, current: 64100,
			score: 55, currentScore: 63, grade: "C", conviction: 5,
			horizon: model.Horizon3M, note: "특수선 수주 모멘텀",
			benchReturn: 4.1, peak: 18.2, trough: -2.6,
			earlyHitDaysAgo: 28, earlyHitPrice: 68400,
			news: "한화오션, 호주 잠수함 사업 우선협상", publisher: "매일경제",
		}, now),

		// 4. 검증 중 — 아직 세는 중이고 목표와 반대로 가 있다. 게이지의 음수 쪽이 보인다.
		stampView(stampSeed{
			id: 4, symbol: "NVDA", name: "NVIDIA", us: true, stampedDaysAgo: 25, dueDaysAhead: 60,
			price: 178.20, target: 210, settled: 0, current: 171.40,
			score: 82, currentScore: 76, grade: "A", conviction: 5,
			horizon: model.Horizon3M, note: "실적 발표 앞두고 눌림목이라 판단",
			peak: 4.2, trough: -6.1,
			news: "Nvidia's next chip cycle draws record orders", publisher: "Reuters",
		}, now),

		// 5. 검증 중 + 관망 — 밴드 판정을 쓰는 유일한 케이스.
		stampView(stampSeed{
			id: 5, symbol: "AAPL", name: "애플", us: true, stampedDaysAgo: 3, dueDaysAhead: 1,
			price: 228.40, target: 232, settled: 0, current: 231.10,
			score: 70, currentScore: 71, grade: "B", conviction: 2,
			horizon: model.HorizonNextSessionClose, note: "",
			peak: 2.4, trough: -0.8,
			news: "Apple services revenue hits a record", publisher: "Bloomberg",
		}, now),

		// 6. 이미 개봉한 빗나감 — 개봉 대기 큐에 남지 않는 것이 정상임을 보인다.
		stampView(stampSeed{
			id: 6, symbol: "035720", name: "카카오", stampedDaysAgo: 70, dueDaysAgo: 40,
			price: 41300, target: 47000, settled: 38100, current: 38100,
			score: 38, currentScore: 35, grade: "D", conviction: 2,
			horizon: model.Horizon1M, note: "낙폭과대 반등 기대 — 결과적으로 근거가 얕았다",
			benchReturn: 2.1, peak: 2.4, trough: -9.8, opened: true,
			news: "카카오, 광고 매출 둔화 지속", publisher: "전자신문",
		}, now),
	}
	return views
}

// StampSummary aggregates the fixture through the same rule the real screen
// uses, so the tiles cannot disagree with the rows below them.
func StampSummary(now time.Time) model.StampSummary {
	return summarize(Stamps(now))
}

type stampSeed struct {
	id                             int64
	symbol, name                   string
	us                             bool
	stampedDaysAgo                 int
	dueDaysAgo, dueDaysAhead       int
	price, target, settled, current float64
	score, currentScore            int
	grade                          string
	conviction                     int
	horizon                        model.StampHorizonKind
	note                           string
	benchReturn                    float64
	peak, trough                   float64
	earlyHitDaysAgo                int
	earlyHitPrice                  float64
	opened, held                   bool
	news, publisher                string
}

func stampView(s stampSeed, now time.Time) model.StampView {
	stampedAt := ago(now, s.stampedDaysAgo)
	dueAt := ahead(now, s.dueDaysAhead)
	if s.dueDaysAgo > 0 {
		dueAt = ago(now, s.dueDaysAgo)
	}

	currency := model.CurrencyKRW
	asset := model.AssetDomestic
	benchmark := "kospi"
	if s.us {
		currency = model.CurrencyUSD
		asset = model.AssetUS
		benchmark = "sp500"
	}

	stamp := model.Stamp{
		ID: s.id, Symbol: s.symbol, SymbolName: s.name,
		AssetType: asset, Currency: currency,
		StampedAt: stampedAt, TradeDate: dateOf(stampedAt),
		MarketSession: "regular", BenchmarkID: benchmark,
		Price: s.price, Volume: 8_421_500,
		Score: i(s.score), Grade: s.grade,
		Stance:      model.StanceForTarget(s.price, s.target),
		TargetPrice: s.target,
		Conviction:  s.conviction,
		HorizonKind: s.horizon,
		DueAt:       dueAt,
		Note:        s.note,
		Snapshot: model.StampSnapshot{
			Week52High: s.price * 1.28, Week52Low: s.price * 0.72,
			Breakdown: model.ScoreBreakdown{
				Technical: f64(float64(s.score) - 8), Quality: f64(float64(s.score) + 6),
				Growth: f64(float64(s.score) - 3), Valuation: f64(float64(s.score) + 4),
				Sentiment: f64(float64(s.score) + 1),
			},
			Held: s.held,
		},
		Context: model.StampContext{
			News: []model.StampNews{{
				Headline: s.news, PublisherName: s.publisher, PublishedAt: ago(now, s.stampedDaysAgo+1),
			}},
			Disclosures: []model.BriefingEvent{},
			Indices: []model.StampIndex{
				{ID: "kospi", Name: "코스피", Value: 7096.89, ChangeRate: 0.42},
				{ID: "sp500", Name: "S&P 500", Value: 7785.76, ChangeRate: -0.17},
			},
		},
		Chart:     model.StampChart{Interval: "1d", Bars: []model.Candle{}},
		CreatedAt: stampedAt, UpdatedAt: stampedAt,
	}
	if s.opened {
		openedAt := dueAt.Add(6 * time.Hour)
		stamp.OpenedAt = &openedAt
	}
	if s.held {
		stamp.Snapshot.HoldingQuantity = 12
		stamp.Snapshot.HoldingAvgPrice = f64(s.price * 0.91)
	}

	view := model.StampView{Stamp: stamp}
	view.Opened = s.opened
	view.ExpectedReturn = f64(pct(s.price, s.target))
	view.PeakReturn = f64(s.peak)
	view.TroughReturn = f64(s.trough)
	view.Current = &model.StampCurrent{
		Price: s.current, ChangeRate: f64(pct(s.price, s.current)),
		Score: i(s.currentScore), Grade: s.grade,
		Breakdown: model.ScoreBreakdown{
			Technical: f64(float64(s.currentScore) - 4), Quality: f64(float64(s.currentScore) + 6),
			Growth: f64(float64(s.currentScore) - 3), Valuation: f64(float64(s.currentScore) - 2),
			Sentiment: f64(float64(s.currentScore) + 3),
		},
		Source: "quote",
	}
	if stamp.Stance != model.StanceNeutral && s.target != s.price {
		view.TargetProgress = f64((s.current - s.price) / (s.target - s.price) * 100)
	}

	view.Returns = stampReturns(s, stampedAt, now)
	view.Horizon = horizonReturn(s, dueAt)

	switch {
	case s.earlyHitDaysAgo > 0:
		hitDate := dateOf(ago(now, s.earlyHitDaysAgo))
		view.TargetHit = &model.StampTargetHit{Date: hitDate, Price: s.earlyHitPrice}
		view.Verdict = model.VerdictHit
		view.TargetAccuracy = f64(0)
		view.Due = true
	case s.dueDaysAgo > 0:
		view.Due = true
		view.Verdict = verdictOf(stamp.Stance, pct(s.price, s.settled))
		view.TargetAccuracy = f64(absPct(s.settled, s.target))
	default:
		view.Due = false
		view.Verdict = model.VerdictPending
	}
	return view
}

// stampReturns fills the fixed buckets that have actually elapsed. A bucket the
// stamp has not lived through stays empty so the row renders "—" — inventing a
// number there would teach the wrong thing about what the screen means.
func stampReturns(s stampSeed, stampedAt, now time.Time) []model.StampReturn {
	buckets := []struct {
		label string
		days  int
		ratio float64 // 그 시점까지 진행된 총 변화의 비율
	}{
		{"1D", 1, 0.08}, {"1W", 7, 0.31}, {"1M", 30, 0.78}, {"3M", 90, 1.0},
	}
	total := s.current
	if s.settled > 0 {
		total = s.settled
	}
	out := make([]model.StampReturn, 0, len(buckets))
	for _, b := range buckets {
		row := model.StampReturn{Horizon: b.label, BenchmarkID: benchmarkOf(s)}
		asOf := stampedAt.Add(time.Duration(b.days) * day)
		if asOf.After(now) {
			out = append(out, row)
			continue
		}
		price := s.price + (total-s.price)*b.ratio
		ret := pct(s.price, price)
		bench := s.benchReturn * b.ratio
		alpha := ret - bench
		row.AsOfDate = dateOf(asOf)
		row.Price = f64(price)
		row.Return = f64(ret)
		row.BenchmarkRet = f64(bench)
		row.Alpha = f64(alpha)
		out = append(out, row)
	}
	return out
}

func horizonReturn(s stampSeed, dueAt time.Time) *model.StampReturn {
	row := &model.StampReturn{Horizon: "horizon", BenchmarkID: benchmarkOf(s)}
	if s.settled <= 0 {
		return row
	}
	ret := pct(s.price, s.settled)
	alpha := ret - s.benchReturn
	row.AsOfDate = dateOf(dueAt)
	row.Price = f64(s.settled)
	row.Return = f64(ret)
	row.BenchmarkRet = f64(s.benchReturn)
	row.Alpha = f64(alpha)
	return row
}

func benchmarkOf(s stampSeed) string {
	if s.us {
		return "sp500"
	}
	return "kospi"
}

func pct(from, to float64) float64 {
	if from == 0 {
		return 0
	}
	return (to - from) / from * 100
}

func absPct(settled, target float64) float64 {
	if target == 0 {
		return 0
	}
	d := (settled - target) / target * 100
	if d < 0 {
		return -d
	}
	return d
}

func verdictOf(stance model.StampStance, ret float64) model.StampVerdict {
	switch stance {
	case model.StanceBullish:
		if ret > 0 {
			return model.VerdictHit
		}
	case model.StanceBearish:
		if ret < 0 {
			return model.VerdictHit
		}
	case model.StanceNeutral:
		if ret > -model.StampNeutralBandPct && ret < model.StampNeutralBandPct {
			return model.VerdictHit
		}
	}
	return model.VerdictMiss
}

// summarize mirrors service.summarizeStamps. It is duplicated rather than
// imported because internal/service already depends on this package's siblings,
// and a fixture package that reaches back into the service layer would make the
// import graph cyclic.
func summarize(views []model.StampView) model.StampSummary {
	out := model.StampSummary{Total: len(views)}
	symbols := map[string]struct{}{}
	var retSum, alphaSum, errSum float64
	var retN, alphaN, errN int

	for _, v := range views {
		symbols[v.Symbol] = struct{}{}
		if v.Due && !v.Opened {
			out.DueCount++
		}
		if v.TargetHit != nil {
			out.EarlyHits++
		}
		if v.TargetAccuracy != nil {
			errSum += *v.TargetAccuracy
			errN++
			if *v.TargetAccuracy <= model.StampNeutralBandPct {
				out.WithinBand++
			}
		}
		if v.Verdict == model.VerdictHit || v.Verdict == model.VerdictMiss {
			out.Judged++
			if v.Verdict == model.VerdictHit {
				out.Hits++
			}
		}
		if v.Horizon != nil {
			if v.Horizon.Return != nil {
				retSum += *v.Horizon.Return
				retN++
			}
			if v.Horizon.Alpha != nil {
				alphaSum += *v.Horizon.Alpha
				alphaN++
			}
		}
	}

	out.Symbols = len(symbols)
	if out.Judged > 0 {
		rate := float64(out.Hits) / float64(out.Judged) * 100
		out.HitRate = &rate
	}
	if retN > 0 {
		out.AvgReturn = f64(retSum / float64(retN))
	}
	if alphaN > 0 {
		out.AvgAlpha = f64(alphaSum / float64(alphaN))
	}
	if errN > 0 {
		out.AvgTargetError = f64(errSum / float64(errN))
	}
	if len(views) > 0 {
		last := views[0].StampedAt
		for _, v := range views {
			if v.StampedAt.After(last) {
				last = v.StampedAt
			}
		}
		formatted := last.UTC().Format(time.RFC3339)
		out.LastStamped = &formatted
	}
	return out
}
