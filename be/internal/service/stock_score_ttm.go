package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

/*
국내 재무 비율을 **최근 4분기 합(TTM)**으로 다시 잰다 (2026-10-10, docs/BUSINESS_REVIEW.md §5 D1).

# 무엇이 틀렸나

국내 `stock_fundamentals`는 최신 **사업보고서(연간)**에서 만든다(`fetchDartSnapshots`).
그래서 10월의 PER은 지난해 12월 결산 EPS로 잰 값이었다. 실측: 삼성전자가 분기 영업이익 107조를
낸 주에 PER 39.8배·가치 19점 — 같은 화면의 AI 요약은 분기 실적(+56%)을 말하고 있었다.
실적이 빠르게 움직이는 업종(반도체)일수록 가치·성장 축이 9개월 늦게 따라온다.

# 어떻게 고치나 — 새 호출 없이

분기 단독값은 이미 `financial_periods`(basis='quarter', source='dart')에 쌓여 있다
(`IngestFinancialsDomestic`, 3년 창). 그 가운데 **연속된 최근 4분기**가 있으면 흐름값(매출·영업이익·
순이익)은 합으로, 재고값(자본)은 가장 최근 분기말 값으로 바꾼다. 8분기가 이어지면 성장률도 TTM 대
그 전 TTM으로 잰다. 못 채우면 지금까지처럼 연간 값을 그대로 쓴다.

# 미래 참조

`created_at`이 채점일 다음 날보다 앞선 행만 읽는다 — 그날 우리가 알고 있던 분기만 쓴다. 과거
날짜를 다시 매기면 그 시점엔 분기 행이 없었으므로 연간 값으로 떨어진다(정직한 쪽).

ponytail: EPS 대신 순이익 ÷ 발행주식수(지배·비지배 합)를 쓴다 — 지주사처럼 비지배 몫이 큰
종목은 PER이 조금 낮게 나온다. 지배주주 순이익 컬럼이 생기면 그것으로 바꾼다.
*/

// ttmQuarter is one standalone domestic quarter from financial_periods.
type ttmQuarter struct {
	seq       int // fiscal_year*4 + period_index — 연속 여부 판정용
	label     string
	revenue   sql.NullFloat64
	opIncome  sql.NullFloat64
	netIncome sql.NullFloat64
	equity    sql.NullFloat64
}

// ttmFigures is what the last four (and, when present, the four before) quarters say.
type ttmFigures struct {
	latest    string // 「2026Q2」 — inputs에 남긴다
	revenue   *float64
	opIncome  *float64
	netIncome *float64
	equity    *float64 // 최근 분기말
	avgEquity *float64 // 최근 분기말과 1년 전 분기말의 평균(있을 때)
	prevRev   *float64 // 그 전 4분기 매출 합
	prevNI    *float64 // 그 전 4분기 순이익 합
}

/*
computeTTM — 최신순으로 정렬된 분기들에서 TTM을 만든다. 맨 앞 4개가 **빈틈없이 이어져야** 한다
(분기 하나가 빠진 채 합하면 3분기 실적을 1년으로 읽는다). 합에 들어가는 값이 하나라도 없으면 그
항목만 비운다.
*/
func computeTTM(quarters []ttmQuarter) (ttmFigures, bool) {
	if len(quarters) < 4 || !consecutive(quarters[:4]) {
		return ttmFigures{}, false
	}
	out := ttmFigures{
		latest:    quarters[0].label,
		revenue:   sumQuarters(quarters[:4], func(q ttmQuarter) sql.NullFloat64 { return q.revenue }),
		opIncome:  sumQuarters(quarters[:4], func(q ttmQuarter) sql.NullFloat64 { return q.opIncome }),
		netIncome: sumQuarters(quarters[:4], func(q ttmQuarter) sql.NullFloat64 { return q.netIncome }),
	}
	if quarters[0].equity.Valid && quarters[0].equity.Float64 > 0 {
		e := quarters[0].equity.Float64
		out.equity = &e
		out.avgEquity = &e
	}
	if len(quarters) >= 8 && consecutive(quarters[:8]) {
		out.prevRev = sumQuarters(quarters[4:8], func(q ttmQuarter) sql.NullFloat64 { return q.revenue })
		out.prevNI = sumQuarters(quarters[4:8], func(q ttmQuarter) sql.NullFloat64 { return q.netIncome })
		if out.equity != nil && quarters[4].equity.Valid && quarters[4].equity.Float64 > 0 {
			avg := (*out.equity + quarters[4].equity.Float64) / 2
			out.avgEquity = &avg
		}
	}
	return out, true
}

func consecutive(qs []ttmQuarter) bool {
	for i := 1; i < len(qs); i++ {
		if qs[i-1].seq-qs[i].seq != 1 {
			return false
		}
	}
	return true
}

func sumQuarters(qs []ttmQuarter, get func(ttmQuarter) sql.NullFloat64) *float64 {
	var sum float64
	for _, q := range qs {
		v := get(q)
		if !v.Valid {
			return nil
		}
		sum += v.Float64
	}
	return &sum
}

/*
applyTTM — 연간 값 위에 TTM을 덮는다. 덮는 것은 **TTM으로 잴 수 있는 칸만**이고, 부채비율·
유동비율처럼 분기 표에 재료가 없는 칸은 연간 값 그대로다.
*/
func (in *scoreInput) applyTTM(t ttmFigures) {
	in.ttmPeriod = t.latest
	if t.revenue != nil && *t.revenue != 0 {
		if t.opIncome != nil {
			in.opMargin = pctRatio(*t.opIncome, *t.revenue)
		}
		if t.netIncome != nil {
			in.netMargin = pctRatio(*t.netIncome, *t.revenue)
		}
	}
	if t.netIncome != nil && t.avgEquity != nil {
		in.roe = pctRatio(*t.netIncome, *t.avgEquity)
	}
	if t.revenue != nil && t.prevRev != nil {
		in.revGrowth = pctChange(*t.revenue, *t.prevRev)
	}
	if t.netIncome != nil && t.prevNI != nil {
		// 발행주식수가 같은 기간이면 순이익 성장 = EPS 성장이다.
		in.epsGrowth = pctChange(*t.netIncome, *t.prevNI)
	}
	if in.close <= 0 || in.marketCap <= 0 {
		return
	}
	if t.netIncome != nil {
		// 4분기 합이 적자면 PER은 뜻이 없다 — 연간 흑자 PER을 남기지 않고 비운다(scoreEPS 주석).
		in.per = nil
		if *t.netIncome > 0 && in.shares > 0 {
			v := in.close / (*t.netIncome / in.shares)
			in.per = &v
		}
	}
	if t.equity != nil {
		v := in.marketCap / *t.equity
		in.pbr = &v
	}
	if t.revenue != nil && *t.revenue > 0 {
		v := in.marketCap / *t.revenue
		in.psr = &v
	}
}

// attachDomesticTTM reads the last eight standalone quarters per domestic symbol and
// overlays TTM ratios. 실패하면 연간 값으로 매긴다 — 호출자가 로그만 남긴다.
func (s *ScoreService) attachDomesticTTM(ctx context.Context, inputs []*scoreInput, tradeDate string) (int, error) {
	bySymbol := make(map[string]*scoreInput, len(inputs))
	for _, in := range inputs {
		if in.assetType == "domestic" {
			bySymbol[strings.ToUpper(in.symbol)] = in
		}
	}
	if len(bySymbol) == 0 {
		return 0, nil
	}
	const q = `
		SELECT symbol, fiscal_year, period_index, revenue, operating_income, net_income, total_equity
		  FROM financial_periods
		 WHERE source = 'dart' AND basis = 'quarter'
		   AND period_end IS NOT NULL
		   AND period_end >= DATE_SUB(?, INTERVAL 30 MONTH)
		   AND created_at < DATE_ADD(?, INTERVAL 1 DAY)
		 ORDER BY symbol, fiscal_year DESC, period_index DESC`
	rows, err := s.db.QueryContext(ctx, q, tradeDate, tradeDate)
	if err != nil {
		return 0, fmt.Errorf("ttm quarters: %w", err)
	}
	defer rows.Close()

	applied := 0
	var cur string
	var buf []ttmQuarter
	flush := func() {
		if in := bySymbol[cur]; in != nil {
			if t, ok := computeTTM(buf); ok {
				in.applyTTM(t)
				applied++
			}
		}
		buf = buf[:0]
	}
	for rows.Next() {
		var (
			symbol             string
			year, idx          int
			rev, op, ni, eqity sql.NullFloat64
		)
		if err := rows.Scan(&symbol, &year, &idx, &rev, &op, &ni, &eqity); err != nil {
			return applied, err
		}
		symbol = strings.ToUpper(symbol)
		if symbol != cur {
			flush()
			cur = symbol
		}
		if len(buf) < 8 {
			buf = append(buf, ttmQuarter{
				seq: year*4 + idx, label: fmt.Sprintf("%dQ%d", year, idx),
				revenue: rev, opIncome: op, netIncome: ni, equity: eqity,
			})
		}
	}
	flush()
	return applied, rows.Err()
}
