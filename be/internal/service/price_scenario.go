package service

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
목표가 시나리오 (V2 요구 5) — 방법의 근거는 `model/price_scenario.go` 머리글에.

# 조회 경로에 외부 호출도 LLM도 없다

한 종목의 시나리오는 **조회 두 번**이다: 종가 1년치 한 번, 점수 한 줄 한 번.
나머지는 메모리 산수다.

# 방법의 성적은 캐시한다

「이 방법이 얼마나 맞았나」는 종목과 무관한 값이라 종목마다 다시 잴 이유가 없다.
250종목 × 6시점을 실제로 돌려 재고(조회 1회 + 메모리), 오래 캐시한다. 점수·종가가
하루 한 번 바뀌므로 그보다 촘촘히 재는 것은 정확도가 아니라 부하만 사는 일이다.
*/

// PriceScenarioService builds the three-case band for one symbol.
type PriceScenarioService struct {
	closes   *repository.StockDailyCloseRepo
	scores   *ScoreService
	rankings *repository.StockScoreRankingRepo
	stocks   *repository.StockRepo

	mu       sync.RWMutex
	accuracy map[string]scenarioAccuracyEntry
}

type scenarioAccuracyEntry struct {
	data model.ScenarioAccuracy
	at   time.Time
}

/*
scenarioAccuracyTTL — 방법 성적 캐시 수명.

6시간이다. 이 값은 새 거래일이 하나 붙어야 달라지는데, 그 하나가 1,452개 표본에
주는 영향은 소수점 아래다. 짧게 잡을 이유가 없다.
*/
const scenarioAccuracyTTL = 6 * time.Hour

/*
백테스트 표본 크기 — 사용자에게 보여 준 실측(2026-09-18)과 **같은 눈금**이다.

250종목 × 6시점이면 표본 약 1,450개다. 더 키우면 읽는 행이 비례해 늘고, 적중률의
소수점 첫째 자리는 이미 안정적이다.
*/
const (
	scenarioSampleSymbols = 250
	scenarioSampleDates   = 6
	// 시점 간격(일). 30일이면 6시점이 5개월을 덮는다.
	scenarioSampleStrideDays = 30
)

func NewPriceScenarioService(
	closes *repository.StockDailyCloseRepo,
	scores *ScoreService,
	rankings *repository.StockScoreRankingRepo,
	stocks *repository.StockRepo,
) *PriceScenarioService {
	return &PriceScenarioService{
		closes: closes, scores: scores, rankings: rankings, stocks: stocks,
		accuracy: map[string]scenarioAccuracyEntry{},
	}
}

/*
dailyVolatility — 일간 로그수익률의 표준편차.

로그수익률을 쓰는 이유: 가격은 음수가 될 수 없고 비율로 움직인다. 단순 차분으로
재면 고가 종목의 변동성이 과대평가되고, 밴드를 만들 때 하단이 0 아래로 내려갈 수
있다. `exp(±zσ√T)`는 그 둘을 한 번에 막는다.

0 이하 종가는 건너뛴다 — 로그가 정의되지 않고, 그런 행은 데이터 오류다.
*/
func dailyVolatility(points []repository.DailyClosePoint) (float64, int) {
	rets := make([]float64, 0, len(points))
	for i := 1; i < len(points); i++ {
		prev, cur := points[i-1].Close, points[i].Close
		if prev <= 0 || cur <= 0 {
			continue
		}
		rets = append(rets, math.Log(cur/prev))
	}
	if len(rets) < 2 {
		return 0, len(rets)
	}
	mean := 0.0
	for _, r := range rets {
		mean += r
	}
	mean /= float64(len(rets))
	sum := 0.0
	for _, r := range rets {
		d := r - mean
		sum += d * d
	}
	// 표본 표준편차 — 모집단이 아니라 관측된 1년을 표본으로 본다.
	return math.Sqrt(sum / float64(len(rets)-1)), len(rets)
}

// scenarioBand는 현재가와 일간 변동성에서 약세·강세 가격을 만든다.
func scenarioBand(px, dailySigma float64) (bear, bull, horizonVol float64) {
	horizonVol = dailySigma * math.Sqrt(float64(model.ScenarioTradingDays))
	bear = px * math.Exp(-model.ScenarioZ*horizonVol)
	bull = px * math.Exp(model.ScenarioZ*horizonVol)
	return bear, bull, horizonVol
}

/*
Scenario — 한 종목의 약세·중립·강세.

이력이 모자라면 **빈 카드가 아니라 사유**를 낸다 (요구 8). 갓 상장한 종목에
0원짜리 밴드를 그리는 것보다 「아직 이력이 모자랍니다」가 정직하다.
*/
func (s *PriceScenarioService) Scenario(ctx context.Context, symbol string) (*model.PriceScenario, error) {
	if s == nil || s.closes == nil {
		return nil, invalidInput("시나리오를 사용할 수 없습니다")
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil, invalidInput("종목을 지정해 주세요")
	}

	out := &model.PriceScenario{Symbol: sym, HorizonDays: model.ScenarioHorizonDays}
	// 종목 한 줄은 **한 번만** 읽는다 — 이름·통화·시장이 전부 여기서 나온다.
	market := "domestic"
	if s.stocks != nil {
		if row, err := s.stocks.GetBySymbol(ctx, sym); err == nil && row != nil {
			out.Name = row.Name
			out.Currency = string(row.Currency)
			if !strings.EqualFold(string(row.AssetType), string(model.AssetDomestic)) {
				market = "overseas"
			}
		}
	}

	since := time.Now().AddDate(0, 0, -model.ScenarioVolWindowDays).Format("2006-01-02")
	points, err := s.closes.ClosesSince(ctx, sym, since)
	if err != nil {
		return nil, err
	}
	if len(points) == 0 {
		out.Unavailable = "no_close"
		return out, nil
	}

	last := points[len(points)-1]
	out.AsOf = last.Date
	out.Close = last.Close

	sigma, n := dailyVolatility(points)
	if n < model.ScenarioMinReturns || sigma <= 0 {
		// 「변동성이 0」이 아니라 「잴 만큼 없다」 — 둘은 다른 말이다.
		out.Unavailable = "not_enough_history"
		return out, nil
	}

	bear, bull, horizonVol := scenarioBand(last.Close, sigma)
	out.VolatilityPct = round2(horizonVol * 100)

	// 현재 PER — 내재 PER의 기준. 없으면 세 점 모두 내재 PER이 비고 화면이 그 줄을 뺀다.
	var per *float64
	if s.scores != nil {
		if v, ok := s.scenarioCurrentPER(ctx, sym); ok {
			per = &v
			out.CurrentPER = &v
		}
	}

	out.Points = []model.ScenarioPoint{
		scenarioPoint(model.ScenarioBear, bear, last.Close, per),
		scenarioPoint(model.ScenarioBase, last.Close, last.Close, per),
		scenarioPoint(model.ScenarioBull, bull, last.Close, per),
	}

	if acc, err := s.Accuracy(ctx, market); err == nil && acc != nil {
		out.Accuracy = acc
	}
	return out, nil
}

func scenarioPoint(kind model.ScenarioKind, price, base float64, per *float64) model.ScenarioPoint {
	p := model.ScenarioPoint{
		Kind:  kind,
		Label: model.ScenarioLabels[kind],
		Price: math.Round(price),
	}
	if base > 0 {
		p.ChangePct = round2((price/base - 1) * 100)
	}
	/*
		내재 PER은 **비례**다. 이익이 그대로일 때 가격이 x배가 되면 배수도 x배가 된다.
		이익 전망을 지어내지 않으므로 「이 가격이면 지금 이익 기준 몇 배」라는 뜻이고,
		앱이 이미 쓰는 PER에서 출발하므로 다른 화면과 어긋나지 않는다.
	*/
	if per != nil && base > 0 && *per > 0 {
		v := round2(*per * price / base)
		p.ImpliedPER = &v
	}
	return p
}

// scenarioCurrentPER reads the app's own PER so two screens cannot disagree.
func (s *PriceScenarioService) scenarioCurrentPER(ctx context.Context, symbol string) (float64, bool) {
	tradeDate, err := s.scores.LatestScoredDate(ctx)
	if err != nil || tradeDate == "" {
		return 0, false
	}
	per, ok, err := s.scores.ScoreInputNumber(ctx, symbol, tradeDate, "$.raw.per")
	if err != nil || !ok || per <= 0 {
		return 0, false
	}
	return per, true
}

/* ------------------------------------------------------------------------- */
/* 방법의 성적                                                                */
/* ------------------------------------------------------------------------- */

/*
Accuracy — 「이 방법이 지난 시점들에서 얼마나 맞았나」.

# 선행 편향을 막는 한 줄

각 시점 `d`의 변동성은 **`d` 이전 1년**으로만 잰다. `d` 이후 종가가 한 줄이라도
섞이면 그 밴드는 미래를 알고 그린 것이고, 적중률은 그만큼 부풀려진다. Phase 2의
리밸런싱 가드와 같은 자리다.

# 표본을 숨기지 않는다

적중률만 내면 그것이 10건에서 나온 것인지 1,400건에서 나온 것인지 알 수 없다.
`Samples`·`Symbols`·`Dates`를 함께 낸다.
*/
func (s *PriceScenarioService) Accuracy(ctx context.Context, market string) (*model.ScenarioAccuracy, error) {
	if s == nil || s.closes == nil || s.scores == nil || s.rankings == nil {
		return nil, nil
	}
	norm := "domestic"
	if strings.EqualFold(strings.TrimSpace(market), "overseas") {
		norm = "overseas"
	}
	if cached, ok := s.cachedAccuracy(norm); ok {
		return &cached, nil
	}

	tradeDate, err := s.scores.LatestScoredDate(ctx)
	if err != nil || tradeDate == "" {
		return nil, err
	}
	assetTypes := domesticAssetTypes
	if norm == "overseas" {
		assetTypes = overseasAssetTypes
	}
	rankable, err := s.rankings.RankableSymbols(ctx, tradeDate, assetTypes)
	if err != nil {
		return nil, err
	}
	if len(rankable) == 0 {
		return nil, nil
	}

	// 표본은 **결정적으로** 고른다 — 무작위면 새로고침마다 적중률이 흔들린다.
	symbols := make([]string, 0, len(rankable))
	for sym := range rankable {
		symbols = append(symbols, sym)
	}
	sort.Strings(symbols)
	if len(symbols) > scenarioSampleSymbols {
		symbols = symbols[:scenarioSampleSymbols]
	}

	/*
		한 번에 읽는다. 종목마다 물으면 250번의 왕복이 되고, 그것이 이 기능을
		조회 경로에 둘 수 없게 만드는 유일한 이유가 된다.

		하한은 「가장 이른 시점 − 변동성 창」이다. 그보다 이르면 안 읽고, 그보다
		늦으면 첫 시점의 변동성을 못 잰다.
	*/
	last, err := time.Parse("2006-01-02", tradeDate)
	if err != nil {
		return nil, err
	}
	// 마지막 시점은 지평만큼 **과거**여야 결과를 볼 수 있다.
	newest := last.AddDate(0, 0, -model.ScenarioHorizonDays)
	dates := make([]time.Time, 0, scenarioSampleDates)
	for i := 0; i < scenarioSampleDates; i++ {
		dates = append(dates, newest.AddDate(0, 0, -i*scenarioSampleStrideDays))
	}
	oldest := dates[len(dates)-1].AddDate(0, 0, -model.ScenarioVolWindowDays)

	reqs := make([]repository.SeriesRequest, 0, len(symbols))
	for _, sym := range symbols {
		reqs = append(reqs, repository.SeriesRequest{Key: sym, Since: oldest.Format("2006-01-02")})
	}
	series, err := s.closes.ClosesSinceMulti(ctx, reqs)
	if err != nil {
		return nil, err
	}

	acc := scenarioAccuracyFrom(series, dates)
	acc.Market = norm
	acc.MeasuredAt = tradeDate
	acc.Symbols = len(symbols)
	acc.Dates = len(dates)

	s.mu.Lock()
	s.accuracy[norm] = scenarioAccuracyEntry{data: acc, at: time.Now()}
	s.mu.Unlock()
	return &acc, nil
}

/*
scenarioAccuracyFrom — 순수 함수. DB도 시계도 안 본다.

각 (종목, 시점)마다 ① 시점 이전 1년으로 변동성을 재고 ② 밴드를 만들고 ③ 지평
뒤의 첫 종가와 맞춰 본다. 어느 한 조각이라도 없으면 그 표본은 **세지 않는다** —
0으로 채우면 없는 관측이 적중률을 끌어내린다.
*/
func scenarioAccuracyFrom(
	series map[string][]repository.DailyClosePoint,
	dates []time.Time,
) model.ScenarioAccuracy {
	var (
		samples            int
		hits               int
		below, above       int
		baseErrSum         float64
	)

	for _, points := range series {
		if len(points) < model.ScenarioMinReturns {
			continue
		}
		for _, d := range dates {
			ds := d.Format("2006-01-02")
			future := d.AddDate(0, 0, model.ScenarioHorizonDays).Format("2006-01-02")
			volFrom := d.AddDate(0, 0, -model.ScenarioVolWindowDays).Format("2006-01-02")

			window := scenarioSlice(points, volFrom, ds)
			px, okPx := scenarioLastOnOrBefore(points, ds)
			fut, okFut := scenarioFirstOnOrAfter(points, future)
			if !okPx || !okFut || px <= 0 {
				continue
			}
			sigma, n := dailyVolatility(window)
			if n < model.ScenarioMinReturns || sigma <= 0 {
				continue
			}

			bear, bull, _ := scenarioBand(px, sigma)
			samples++
			switch {
			case fut < bear:
				below++
			case fut > bull:
				above++
			default:
				hits++
			}
			baseErrSum += math.Abs(fut-px) / px
		}
	}

	out := model.ScenarioAccuracy{Samples: samples}
	if samples == 0 {
		return out
	}
	n := float64(samples)
	out.HitRate = round2(float64(hits) / n * 100)
	out.MissBelowPct = round2(float64(below) / n * 100)
	out.MissAbovePct = round2(float64(above) / n * 100)
	out.BaseErrorPct = round2(baseErrSum / n * 100)
	return out
}

// scenarioSlice는 [from, to] 구간만 잘라 준다. points는 날짜 오름차순이다.
func scenarioSlice(points []repository.DailyClosePoint, from, to string) []repository.DailyClosePoint {
	lo := sort.Search(len(points), func(i int) bool { return points[i].Date >= from })
	hi := sort.Search(len(points), func(i int) bool { return points[i].Date > to })
	if lo >= hi {
		return nil
	}
	return points[lo:hi]
}

func scenarioLastOnOrBefore(points []repository.DailyClosePoint, date string) (float64, bool) {
	i := sort.Search(len(points), func(i int) bool { return points[i].Date > date })
	if i == 0 {
		return 0, false
	}
	return points[i-1].Close, true
}

func scenarioFirstOnOrAfter(points []repository.DailyClosePoint, date string) (float64, bool) {
	i := sort.Search(len(points), func(i int) bool { return points[i].Date >= date })
	if i >= len(points) {
		return 0, false
	}
	return points[i].Close, true
}

func (s *PriceScenarioService) cachedAccuracy(market string) (model.ScenarioAccuracy, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.accuracy[market]
	if !ok || time.Since(entry.at) > scenarioAccuracyTTL {
		return model.ScenarioAccuracy{}, false
	}
	return entry.data, true
}

/*
ScoreInputNumber — `stock_scores.inputs`에서 숫자 하나.

설명 전체(`Explain`, 15KB)를 받아 한 값만 쓰는 것을 막는 통로다. 경로는 **호출부
코드에만** 있고 사용자 입력이 닿지 않는다 — 스크리너의 화이트리스트와 같은 이유다.
*/
func (s *ScoreService) ScoreInputNumber(
	ctx context.Context, symbol, tradeDate, path string,
) (float64, bool, error) {
	if s == nil || s.db == nil || symbol == "" || tradeDate == "" {
		return 0, false, nil
	}
	var v sql.NullFloat64
	err := s.db.QueryRowContext(ctx,
		`SELECT CAST(JSON_EXTRACT(inputs, ?) AS DECIMAL(24,6))
		   FROM stock_scores WHERE symbol = ? AND trade_date = ?`,
		path, strings.ToUpper(strings.TrimSpace(symbol)), tradeDate).Scan(&v)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v.Float64, v.Valid, nil
}
