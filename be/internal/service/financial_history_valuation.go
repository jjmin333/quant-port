package service

import (
	"context"

	"github.com/enact/quant-port/be/internal/model"
)

/*
분기별 PER·PBR — **실제로 계산한다.**

예전에는 화면이 `buildTrendSeries(per, symbol, 5)`로 현재 PER 하나에서 5년치를
의사난수로 역산해 그렸다. 심볼 문자열을 해시로 시드 삼는 결정론적 난수라 매번
같은 그림이 나왔고, 그래서 「추이」처럼 보였다 — 재무 데이터가 아니라 장식이었다.

이제 우리가 이미 가진 두 표를 곱한다:

  - `candle_bars` 일봉 — 실측 5년치(2021-10까지 확인)
  - `financial_periods` 분기 — EPS·자본총계·순이익

  PER = 분기말 종가 ÷ 최근 4개 분기 EPS 합(TTM)
  PBR = 분기말 종가 ÷ 주당순자산(자본총계 ÷ 주식수)

**주식수는 그 분기 것을 되찾는다.** EPS의 정의가 「순이익 ÷ 가중평균주식수」이므로
`순이익 ÷ EPS`가 그 분기의 주식수다. 현재 발행주식수를 3년 전 자본총계에 나누면
그 사이 증자·감자가 통째로 왜곡되기 때문에, 저장된 현재 주식수는 되찾기가
실패했을 때만 쓴다.

값을 못 만들면 **넣지 않는다.** TTM은 4개 분기가 다 있어야 하고(EPS 결측률이
DART 32%였다 — 지금은 계정명 매칭을 고쳐 크게 줄었다), 종가가 없으면 그 분기는
비운다. 비면 화면이 선을 끊는다.
*/

// ttmQuarters — TTM(최근 12개월) EPS를 만드는 데 필요한 분기 수.
const ttmQuarters = 4

// applyValuation은 각 분기에 PER·PBR을 채운다. periods는 오래된 순이다.
func (s *FinancialHistoryService) applyValuation(
	ctx context.Context,
	symbol string,
	periods []model.FinancialHistoryPeriod,
	fallbackShares *int64,
) {
	if s == nil || s.candles == nil || len(periods) == 0 {
		return
	}

	for i := range periods {
		p := &periods[i]
		if p.PeriodEnd == nil {
			continue
		}
		closePx, ok, err := s.candles.CloseAtOrBefore(ctx, symbol, model.Interval1d, *p.PeriodEnd)
		if err != nil || !ok || closePx <= 0 {
			// 조회 실패와 「그 시점 시세가 없음」을 구분하지 않는다 — 어느 쪽이든
			// 이 분기의 밸류에이션은 만들 수 없고, 화면은 빈 자리를 그린다.
			continue
		}
		p.CloseAtPeriodEnd = &closePx

		if ttm := ttmEPS(periods, i); ttm != nil && *ttm > 0 {
			per := closePx / *ttm
			p.PER = &per
		}
		if bps := bookValuePerShareAt(*p, fallbackShares); bps != nil && *bps > 0 {
			pbr := closePx / *bps
			p.PBR = &pbr
		}
	}
}

// ttmEPS는 i번째 분기까지의 4개 분기 EPS 합이다. 하나라도 비면 nil —
// 세 분기만 더해 놓고 TTM이라 부르면 PER이 4/3배로 부풀어 「싸 보이는」 종목이 된다.
func ttmEPS(periods []model.FinancialHistoryPeriod, i int) *float64 {
	if i+1 < ttmQuarters {
		return nil
	}
	sum := 0.0
	for j := i - ttmQuarters + 1; j <= i; j++ {
		if periods[j].EPSBasic == nil {
			return nil
		}
		sum += *periods[j].EPSBasic
	}
	return &sum
}

// bookValuePerShareAt — 그 분기의 주당순자산.
func bookValuePerShareAt(p model.FinancialHistoryPeriod, fallbackShares *int64) *float64 {
	if p.TotalEquity == nil || *p.TotalEquity <= 0 {
		return nil
	}
	shares := sharesFromEPS(p)
	if shares == nil {
		shares = fallbackShares
	}
	if shares == nil || *shares <= 0 {
		return nil
	}
	bps := float64(*p.TotalEquity) / float64(*shares)
	return &bps
}

// sharesFromEPS는 순이익 ÷ EPS로 그 분기의 가중평균주식수를 되찾는다.
// EPS가 0에 가까우면 나눗셈이 폭발하므로 거른다.
func sharesFromEPS(p model.FinancialHistoryPeriod) *int64 {
	if p.NetIncome == nil || p.EPSBasic == nil {
		return nil
	}
	eps := *p.EPSBasic
	if eps > -0.0001 && eps < 0.0001 {
		return nil
	}
	shares := int64(mathRound(float64(*p.NetIncome) / eps))
	if shares <= 0 {
		return nil
	}
	return &shares
}
