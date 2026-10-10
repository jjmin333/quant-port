package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
포트폴리오 진단 — 「샀을 때의 이유가 지금도 유효한가」 (V2 요구 6).

무엇을 **할 수 없는지**와 수익률을 신호에서 왜 뺐는지는 `model/holdings_diagnosis.go`
머리글에 있다. 여기서는 판정 규칙과 비용만 적는다.

# 판정

신호 셋(점수 추세 · 실적 서프라이즈 · 내부자 순매수)에 각각 −1 / 0 / +1을 매기고 더한다.

	합 ≥ +2   강화
	합 ≤ −2   약화
	그 외     유지
	쓸 수 있는 신호가 2개 미만이면 **판단 보류**

네 번째(보류)가 있어야 나머지 셋이 정직해진다. 신호가 하나뿐인데 「유지」라고 하면
그것은 판단이 아니라 기본값이다.

# 비용

보유 종목이 몇이든 **조회 다섯 번**이다: 매매 이력 1 · 점수 1 · 실적 1 · 내부자 1 ·
종가 1. 가장 이른 매수일 하나로 전부 긁고, 보유 기간별로 자르는 일은 Go가 한다.
*/

// HoldingsDiagnosisService answers 「보유 이유가 아직 유효한가」.
type HoldingsDiagnosisService struct {
	repo     *repository.HoldingsDiagnosisRepo
	trades   *repository.HoldingTradesRepo
	holdings *repository.UserHoldingsRepo
	scores   *repository.StockScoreRepo
	settings *ScoreSettingsStore
}

func NewHoldingsDiagnosisService(
	repo *repository.HoldingsDiagnosisRepo,
	trades *repository.HoldingTradesRepo,
	holdings *repository.UserHoldingsRepo,
	scores *repository.StockScoreRepo,
	settings *ScoreSettingsStore,
) *HoldingsDiagnosisService {
	return &HoldingsDiagnosisService{
		repo: repo, trades: trades, holdings: holdings, scores: scores, settings: settings,
	}
}

// Diagnose builds the whole portfolio's diagnosis for one user.
func (s *HoldingsDiagnosisService) Diagnose(
	ctx context.Context, userID int64,
) (*model.HoldingsDiagnosis, error) {
	if s == nil || s.repo == nil || s.holdings == nil {
		return nil, fmt.Errorf("diagnosis service not configured")
	}

	holdings, err := s.holdings.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := &model.HoldingsDiagnosis{Items: []model.HoldingDiagnosis{}}
	if len(holdings) == 0 {
		return out, nil
	}

	symbols := make([]string, 0, len(holdings))
	for _, h := range holdings {
		symbols = append(symbols, strings.ToUpper(strings.TrimSpace(h.Symbol)))
	}

	/*
		매수일은 **매매 이력**에서 온다. `user_holdings`는 수량·평단의 파생 캐시라
		「언제부터 들고 있나」를 모른다. 이력이 없으면(화면에서 직접 입력한 보유)
		보유 기간을 말하지 않고, 신호는 보관 창 전체로 잰다.
	*/
	firstBought := map[string]string{}
	if s.trades != nil {
		trades, err := s.trades.List(ctx, userID)
		if err == nil {
			for _, t := range trades {
				if !strings.EqualFold(t.Side, "buy") {
					continue
				}
				sym := strings.ToUpper(strings.TrimSpace(t.Symbol))
				date := t.TradeDate
				if cur, ok := firstBought[sym]; !ok || date < cur {
					firstBought[sym] = date
				}
			}
		}
	}

	// 전 종목을 한 번에 긁을 하한 — 가장 이른 매수일, 없으면 점수 보관 시작.
	retention, _ := s.scores.EarliestScoredDate(ctx)
	since := retention
	for _, d := range firstBought {
		if since == "" || d < since {
			since = d
		}
	}
	if since == "" {
		since = time.Now().In(model.KST).AddDate(0, -6, 0).Format("2006-01-02")
	}

	scoreRows, err := s.scores.ScoreRangeBatch(ctx, symbols, minDate(retention, since))
	if err != nil {
		return nil, err
	}
	earnings, err := s.repo.EarningsSince(ctx, symbols, since)
	if err != nil {
		return nil, err
	}
	insider, err := s.repo.InsiderSince(ctx, symbols, since)
	if err != nil {
		return nil, err
	}
	closes, err := s.repo.LastCloses(ctx, symbols, since)
	if err != nil {
		return nil, err
	}

	settings := model.DefaultScoreSettings()
	if s.settings != nil {
		settings = s.settings.Get(ctx)
	}

	for _, h := range holdings {
		sym := strings.ToUpper(strings.TrimSpace(h.Symbol))
		item := buildHoldingDiagnosis(holdingInputs{
			holding:     h,
			symbol:      sym,
			firstBought: firstBought[sym],
			retention:   retention,
			scores:      scoreRows[sym],
			earnings:    earnings[sym],
			insider:     insider[sym],
			lastClose:   closes[sym],
			settings:    settings,
		})
		out.Items = append(out.Items, item)
		switch item.Verdict {
		case model.DiagnosisStronger:
			out.Summary.Stronger++
		case model.DiagnosisWeaker:
			out.Summary.Weaker++
		case model.DiagnosisSteady:
			out.Summary.Steady++
		default:
			out.Summary.Unknown++
		}
	}

	/*
		약화가 먼저 온다. 이 화면을 여는 이유는 「무엇이 나빠졌나」이고, 좋아진 종목은
		스크롤해서 봐도 늦지 않다. 같은 판정 안에서는 점수 하락이 큰 순.
	*/
	sort.SliceStable(out.Items, func(i, j int) bool {
		vi, vj := verdictRank(out.Items[i].Verdict), verdictRank(out.Items[j].Verdict)
		if vi != vj {
			return vi < vj
		}
		return scoreDeltaOf(out.Items[i]) < scoreDeltaOf(out.Items[j])
	})

	out.ScoreRetentionFrom = retention
	out.Note = diagnosisNote(retention, out.Items)
	return out, nil
}

func verdictRank(v model.DiagnosisVerdict) int {
	switch v {
	case model.DiagnosisWeaker:
		return 0
	case model.DiagnosisUnknown:
		return 1
	case model.DiagnosisSteady:
		return 2
	default:
		return 3
	}
}

func scoreDeltaOf(item model.HoldingDiagnosis) int {
	if item.Score != nil && item.Score.Delta != nil {
		return *item.Score.Delta
	}
	return 0
}

type holdingInputs struct {
	holding     model.UserHolding
	symbol      string
	firstBought string
	retention   string
	scores      []repository.ScoreLightRow
	earnings    []repository.DiagnosisEarningsRow
	insider     []repository.DiagnosisInsiderRow
	lastClose   repository.DiagnosisCloseRow
	settings    model.ScoreSettings
}

// buildHoldingDiagnosis is the pure core — no DB, no clock beyond the given dates.
func buildHoldingDiagnosis(in holdingInputs) model.HoldingDiagnosis {
	out := model.HoldingDiagnosis{
		Symbol:      in.symbol,
		Name:        in.holding.Name,
		Quantity:    in.holding.Quantity,
		AvgPrice:    in.holding.AvgPrice,
		Currency:    in.holding.CostCurrency,
		FirstBought: in.firstBought,
	}
	if in.lastClose.Close > 0 {
		c := in.lastClose.Close
		out.LastClose = &c
		out.LastCloseDate = in.lastClose.Date
		if in.holding.AvgPrice != nil && *in.holding.AvgPrice > 0 {
			r := btRound2((c/(*in.holding.AvgPrice) - 1) * 100)
			out.ReturnPct = &r
		}
	}
	if in.firstBought != "" {
		if t, err := time.Parse("2006-01-02", in.firstBought); err == nil {
			out.HoldingDays = int(time.Now().In(model.KST).Sub(t).Hours() / 24)
		}
	}

	out.Score = diagnosisScore(in)
	out.Earnings = diagnosisEarnings(in)
	out.Insider = diagnosisInsider(in)
	out.ScoreAtPurchase, out.ScoreAtPurchaseReason = scoreAtPurchase(in)

	out.Signals = []model.DiagnosisSignal{
		scoreTrendSignal(out.Score, in.retention),
		earningsSignal(out.Earnings, in.firstBought),
		insiderSignal(out.Insider, in.firstBought, in.lastClose.AvgVolume),
	}
	out.Verdict, out.SignalCount = verdictFrom(out.Signals)
	return out
}

/*
scoreAtPurchase — 살 때의 점수.

원래 보관이 30거래일이라 거의 언제나 없었다(실측: 매수 2026-03-03, 보관 시작
2026-08-07). 2026-09-23부터 축 점수 이력(`stock_score_history`)이 쌓이므로 **그 뒤의
매수는 답이 생긴다** — 그 이전 매수는 여전히 없다. 없으면 왜 없는지를 함께 돌려준다 —
빈칸은 「안 변했다」로 읽힌다.
*/
func scoreAtPurchase(in holdingInputs) (*int, string) {
	if in.firstBought == "" {
		return nil, "no_trade_history"
	}
	if in.retention != "" && in.firstBought < in.retention {
		return nil, "out_of_retention"
	}
	// 매수일 **이후** 첫 회차 — 그날 점수가 없을 수 있다(배치가 매일 도는 것은 아니다).
	var best *repository.ScoreLightRow
	for i := range in.scores {
		row := in.scores[i]
		if row.TradeDate < in.firstBought {
			continue
		}
		if best == nil || row.TradeDate < best.TradeDate {
			best = &in.scores[i]
		}
	}
	if best == nil || !best.Score.Valid {
		return nil, "no_data"
	}
	v := int(best.Score.Int64)
	return &v, ""
}

func diagnosisScore(in holdingInputs) *model.DiagnosisScore {
	if len(in.scores) == 0 {
		return nil
	}
	// ScoreRangeBatch는 최신순이다.
	latest := in.scores[0]
	/*
		기준점은 **보유가 시작된 뒤의 첫 회차**다.

		실적·내부자는 `filterAfter`로 보유 기간을 자르는데 점수만 안 잘랐더니,
		08-11에 산 종목의 추세가 08-07(매수 전)부터 잡혔다 — 살기 전의 점수 변화가
		보유 판단에 들어간 것이다. 매수일이 보관 창보다 오래됐으면 창의 첫 회차가
		기준이고(그게 가진 전부다), `FromDate`가 그 사실을 말한다.
	*/
	oldest := in.scores[len(in.scores)-1]
	for i := len(in.scores) - 1; i >= 0; i-- {
		if in.firstBought == "" || in.scores[i].TradeDate >= in.firstBought {
			oldest = in.scores[i]
			break
		}
	}

	out := &model.DiagnosisScore{AsOf: latest.TradeDate}
	if latest.Score.Valid {
		v := int(latest.Score.Int64)
		out.Current = &v
		out.Grade = model.ScoreGrade(v)
	}
	if oldest.TradeDate != latest.TradeDate && oldest.Score.Valid && latest.Score.Valid {
		from := int(oldest.Score.Int64)
		out.FromDate = oldest.TradeDate
		out.FromValue = &from
		d := int(latest.Score.Int64) - from
		out.Delta = &d
	}
	out.Thesis = thesisTrend(in.holding.ThesisAxes, oldest, latest)
	/*
		최근 한 회차의 「왜」는 Phase 1 귀인을 그대로 쓴다 — 진단이 자기만의 이유를
		따로 계산하면 같은 날에 대해 점수 근거 패널과 다른 말을 한다.
	*/
	if len(in.scores) >= 2 {
		prev := lightToView(in.scores[1])
		cur := lightToView(in.scores[0])
		ch := attributeChange(model.ScoreChangePrevDay, &prev, cur, in.settings.Weights, in.settings)
		if ch.Available && len(ch.Drivers) > 0 {
			out.Driver = ch.Drivers[0].Detail
		}
	}
	return out
}

// diagnosisEarnings keeps the most recent event inside the holding period.
func diagnosisEarnings(in holdingInputs) *model.DiagnosisEarnings {
	rows := filterAfter(in.earnings, in.firstBought, func(e repository.DiagnosisEarningsRow) string { return e.Date })
	if len(rows) == 0 {
		return nil
	}
	last := rows[len(rows)-1]
	out := &model.DiagnosisEarnings{Count: len(rows), Date: last.Date}
	if last.FiscalYear.Valid && last.Period.Valid {
		out.Period = fmt.Sprintf("%d %s", last.FiscalYear.Int64, last.Period.String)
	}
	// 추정치가 없으면 서프라이즈도 없다 — 0%는 「예상대로였다」로 읽힌다.
	if last.EPSAct.Valid && last.EPSEst.Valid && math.Abs(last.EPSEst.Float64) > 1e-9 {
		s := btRound2((last.EPSAct.Float64 - last.EPSEst.Float64) / math.Abs(last.EPSEst.Float64) * 100)
		out.Surprise = &s
	}
	return out
}

func diagnosisInsider(in holdingInputs) *model.DiagnosisInsider {
	rows := filterAfter(in.insider, in.firstBought, func(r repository.DiagnosisInsiderRow) string { return r.Date })
	if len(rows) == 0 {
		return nil
	}
	out := &model.DiagnosisInsider{}
	for _, r := range rows {
		out.Count += r.Count
		out.BuyShares += r.BuyShares
		out.SellShares += r.SellShares
	}
	out.NetShares = out.BuyShares - out.SellShares
	return out
}

// filterAfter keeps rows on/after `since`. 빈 since는 전부 통과 — 매매 이력이 없는
// 보유는 보관 창 전체로 본다.
func filterAfter[T any](rows []T, since string, dateOf func(T) string) []T {
	if since == "" {
		return rows
	}
	out := make([]T, 0, len(rows))
	for _, r := range rows {
		if dateOf(r) >= since {
			out = append(out, r)
		}
	}
	return out
}

/* -------------------------------------------------------------------------- */
/* 신호                                                                        */
/* -------------------------------------------------------------------------- */

func scoreTrendSignal(sc *model.DiagnosisScore, retention string) model.DiagnosisSignal {
	sig := model.DiagnosisSignal{
		ID:    model.SignalScoreTrend,
		Label: model.DiagnosisSignalLabels[model.SignalScoreTrend],
	}
	/*
		매수 이유가 적혀 있으면 **그 축들**로 판단한다 (2026-09-24 사용자 결정). 「성장 때문에
		샀다」는 사람에게 종합 점수의 변화는 다른 축(추세·가치)의 움직임에 묻힌 답이다. 문턱은
		종합과 같다 — 축 평균이 종합보다 조금 더 흔들리지만, 문턱을 따로 두면 두 판정이 같은
		말을 하는지 비교할 수 없다.
	*/
	if sc != nil && sc.Thesis != nil && sc.Thesis.Delta != nil && sc.Thesis.FromValue != nil && sc.Thesis.Current != nil {
		t := sc.Thesis
		sig.Label = "매수 이유(" + t.Label + ")"
		sig.Available = true
		sig.Detail = fmt.Sprintf("%s %d → %d (%+d)", trimDate(t.FromDate), *t.FromValue, *t.Current, *t.Delta)
		switch {
		case *t.Delta >= model.DiagnosisScoreDelta:
			sig.Direction = 1
		case *t.Delta <= -model.DiagnosisScoreDelta:
			sig.Direction = -1
		}
		if abs(*t.Delta) >= 2*model.DiagnosisScoreDelta {
			sig.Strong = true
		}
		return sig
	}
	if sc == nil || sc.Delta == nil || sc.FromValue == nil || sc.Current == nil {
		sig.Reason = "no_data"
		if sc != nil && sc.Current != nil {
			// 점수는 있는데 비교할 옛 회차가 없다 — 보관 창에 한 회차뿐이다.
			sig.Reason = "out_of_retention"
		}
		return sig
	}
	sig.Available = true
	sig.Detail = fmt.Sprintf("%s %d → %d (%+d)", trimDate(sc.FromDate), *sc.FromValue, *sc.Current, *sc.Delta)
	switch {
	case *sc.Delta >= model.DiagnosisScoreDelta:
		sig.Direction = 1
	case *sc.Delta <= -model.DiagnosisScoreDelta:
		sig.Direction = -1
	}
	// 문턱의 두 배를 넘으면 이 신호 하나로도 판정한다 (`model.DiagnosisSignal.Strong`).
	if abs(*sc.Delta) >= 2*model.DiagnosisScoreDelta {
		sig.Strong = true
	}
	_ = retention
	return sig
}

func earningsSignal(e *model.DiagnosisEarnings, firstBought string) model.DiagnosisSignal {
	sig := model.DiagnosisSignal{
		ID:    model.SignalEarnings,
		Label: model.DiagnosisSignalLabels[model.SignalEarnings],
	}
	if e == nil {
		// 보유 기간 중 발표가 없었다 — 자료가 없는 것과 다르다.
		sig.Reason = "no_data"
		if firstBought != "" {
			sig.Detail = "보유 기간 중 실적 발표가 없었습니다"
		}
		return sig
	}
	if e.Surprise == nil {
		// 발표는 있었지만 비교할 컨센서스가 없다.
		sig.Reason = "no_estimate"
		sig.Detail = fmt.Sprintf("%s 발표 (컨센서스 없음)", e.Period)
		return sig
	}
	sig.Available = true
	sig.Detail = fmt.Sprintf("%s EPS 서프라이즈 %+.1f%%", e.Period, *e.Surprise)
	switch {
	case *e.Surprise >= model.DiagnosisSurprisePct:
		sig.Direction = 1
	case *e.Surprise <= -model.DiagnosisSurprisePct:
		sig.Direction = -1
	}
	if math.Abs(*e.Surprise) >= 2*model.DiagnosisSurprisePct {
		sig.Strong = true
	}
	return sig
}

func insiderSignal(i *model.DiagnosisInsider, firstBought string, avgVolume float64) model.DiagnosisSignal {
	sig := model.DiagnosisSignal{
		ID:    model.SignalInsider,
		Label: model.DiagnosisSignalLabels[model.SignalInsider],
	}
	if i == nil || i.Count == 0 {
		sig.Reason = "no_data"
		if firstBought != "" {
			sig.Detail = "보유 기간 중 내부자 거래가 없었습니다"
		}
		return sig
	}
	sig.Available = true
	sig.Detail = fmt.Sprintf("%d건 · 순%s", i.Count, signedShares(i.NetShares))
	switch {
	case i.NetShares > 0:
		sig.Direction = 1
	case i.NetShares < 0:
		sig.Direction = -1
	}
	/*
		하루 거래량에 견줘 본다 — 순매매에는 크기 개념이 없기 때문이다.

		문턱에 못 미치면 방향을 0으로 되돌리되 신호는 **남긴다.** 「재 봤더니 크지 않았다」는
		「못 쟀다」와 다른 말이고, 화면은 그 둘을 다르게 읽어야 한다.
	*/
	if avgVolume > 0 {
		share := math.Abs(i.NetShares) / avgVolume
		sig.Detail += fmt.Sprintf(" · 하루 거래량의 %s", volumeShare(share))
		if share < model.DiagnosisInsiderVolumePct {
			sig.Direction = 0
		}
	}
	return sig
}

// volumeShare는 아주 작은 비중도 0%로 뭉개지 않는다 — 0.011%는 「없음」이 아니라 「작음」이다.
func volumeShare(v float64) string {
	pct := v * 100
	switch {
	case pct >= 10:
		return fmt.Sprintf("%.0f%%", pct)
	case pct >= 1:
		return fmt.Sprintf("%.1f%%", pct)
	}
	return fmt.Sprintf("%.2f%%", pct)
}

func signedShares(v float64) string {
	abs := math.Abs(math.Round(v))
	switch {
	case v > 0:
		return "매수 " + groupDigits(abs) + "주"
	case v < 0:
		return "매도 " + groupDigits(abs) + "주"
	}
	return "변동 없음"
}

/*
groupDigits — 천 단위 쉼표.

실측(2026-09-18) 화면에 「순매도 5634565주」가 그대로 나갔다. 일곱 자리는 쉼표 없이
읽히지 않고, 이 값은 다른 종목의 네 자리와 **나란히** 서기 때문에 자릿수를 세지
못하면 크기 비교 자체가 안 된다.
*/
func groupDigits(v float64) string {
	digits := fmt.Sprintf("%.0f", math.Abs(v))
	lead := len(digits) % 3
	if lead == 0 {
		lead = 3
	}
	out := digits[:lead]
	for i := lead; i < len(digits); i += 3 {
		out += "," + digits[i:i+3]
	}
	return out
}

/*
verdictFrom — 신호를 더해 판정한다.

쓸 수 있는 신호가 둘 미만이면 **판단 보류**다. 하나로 판정하면 그것은 판단이 아니라
그 신호의 별명이고, 사용자는 그 차이를 알 수 없다.
*/
func verdictFrom(signals []model.DiagnosisSignal) (model.DiagnosisVerdict, int) {
	sum, n, strong := 0, 0, false
	for _, s := range signals {
		if !s.Available {
			continue
		}
		n++
		// 강한 신호는 두 몫 — 아무 말도 안 하는 신호가 판정을 뒤집지 못하게.
		w := 1
		if s.Strong {
			w = model.DiagnosisStrongWeight
		}
		sum += w * s.Direction
		if s.Strong && s.Direction != 0 {
			strong = true
		}
	}
	/*
		신호가 둘 미만이면 보류 — **다만 그 하나가 아주 크면 판정한다.**

		실측(2026-09-18): 카카오는 매수 시점 75점이 29점으로 46점 떨어졌는데 실적·내부자
		기록이 없어 「판단 보류」가 됐다. 사용자가 가장 봐야 할 종목이 가장 흐린 답을
		받은 것이다. 약한 것은 증거의 다양성이지 강도가 아니다.
	*/
	if n == 0 || (n < model.DiagnosisMinSignals && !strong) {
		return model.DiagnosisUnknown, n
	}
	if n < model.DiagnosisMinSignals {
		// 강한 신호 하나 — 그 신호의 방향을 그대로 따른다.
		if sum > 0 {
			return model.DiagnosisStronger, n
		}
		return model.DiagnosisWeaker, n
	}
	switch {
	case sum >= model.DiagnosisVerdictThreshold:
		return model.DiagnosisStronger, n
	case sum <= -model.DiagnosisVerdictThreshold:
		return model.DiagnosisWeaker, n
	}
	return model.DiagnosisSteady, n
}

/*
diagnosisNote — 보관 한계를 사람 문장으로.

「매수 시점과 비교할 수 없습니다」만 쓰면 고장으로 읽힌다. 몇 종목이 그런지와 점수가
언제부터 남아 있는지를 함께 적는다.
*/
func diagnosisNote(retention string, items []model.HoldingDiagnosis) string {
	if retention == "" {
		return ""
	}
	out := 0
	for _, it := range items {
		if it.ScoreAtPurchaseReason == "out_of_retention" {
			out++
		}
	}
	if out == 0 {
		return ""
	}
	return fmt.Sprintf(
		"AI 점수 기록이 %s부터 남아 있어 %d종목은 매수 시점 점수와 비교할 수 없습니다. 대신 기록이 있는 기간의 추세로 판단했습니다.",
		trimDate(retention), out)
}

func trimDate(d string) string {
	if len(d) == 10 {
		return d[5:]
	}
	return d
}

func minDate(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	if a < b {
		return a
	}
	return b
}

/*
thesisTrend — 매수 이유 축들의 평균을 매수 뒤 첫 회차와 지금 비교한다.

두 날 **모두** 값이 있는 축만 평균한다 — 한쪽에만 있는 축을 섞으면 축이 생기거나 사라진
것이 변화로 읽힌다(V2_PLAN §0과 같은 함정).
*/
func thesisTrend(axes []string, from, to repository.ScoreLightRow) *model.DiagnosisThesis {
	if len(axes) == 0 {
		return nil
	}
	labels := make([]string, 0, len(axes))
	var sumFrom, sumTo float64
	n := 0
	for _, id := range axes {
		axis := model.ScoreAxis(id)
		labels = append(labels, model.ScoreAxisLabels[axis])
		a, b := from.Breakdown.Get(axis), to.Breakdown.Get(axis)
		if a == nil || b == nil {
			continue
		}
		sumFrom += *a
		sumTo += *b
		n++
	}
	t := &model.DiagnosisThesis{Axes: axes, Label: strings.Join(labels, "·")}
	if n == 0 || from.TradeDate == to.TradeDate {
		return t
	}
	f := int(math.Round(sumFrom / float64(n)))
	c := int(math.Round(sumTo / float64(n)))
	d := c - f
	t.FromDate, t.FromValue, t.Current, t.Delta = from.TradeDate, &f, &c, &d
	return t
}
