package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

/*
질문 칩 4개 → 10개 (2026-09-27 사용자 결정 C ① — 「눌러 보고 싶은 버튼」을 늘린다).

새 질문 여섯 중 다섯의 재료가 여기서 나온다. 전부 **이미 저장된 표**를 종목 키로 찌르는
질의라 LLM도 외부 API도 없다. 여섯째(「내 보유 기준으로」)는 개인 데이터라 이 공개 응답
(엣지 캐시)에 싣지 않고 화면이 로그인 세션으로 따로 계산한다.

재료가 없는 질문은 **칩 자체를 숨긴다** — 해외 종목에 「외국인·기관은」을 내놓고 「자료가
없습니다」라고 답하는 것은 버튼이 아니라 함정이다. 그래서 응답에 `available` 목록을 싣는다.
*/

// StockQAFlows — 투자자별 순매수(국내). 금액이 아니라 **주식 수**다(원 데이터 단위).
type StockQAFlows struct {
	AsOf              string   `json:"asOf"`
	Foreign5          int64    `json:"foreign5"`
	Foreign20         int64    `json:"foreign20"`
	Institution5      int64    `json:"institution5"`
	Institution20     int64    `json:"institution20"`
	ForeignStreak     int      `json:"foreignStreak"` // +n = n일 연속 순매수, -n = 연속 순매도
	InstitutionStreak int      `json:"institutionStreak"`
	ForeignHoldRate   *float64 `json:"foreignHoldRate,omitempty"`
	ForeignHoldPrev   *float64 `json:"foreignHoldRatePrev,omitempty"` // 20거래일 전
	Days              int      `json:"days"`
}

// StockQAEstimate — 애널리스트 EPS 추정치 변화(해외, 06:00 캘린더 적재가 남긴 이력).
type StockQAEstimate struct {
	FiscalYear   int     `json:"fiscalYear"`
	FiscalPeriod string  `json:"fiscalPeriod"`
	Now          float64 `json:"now"`
	NowDate      string  `json:"nowDate"`
	Before       float64 `json:"before"`
	BeforeDate   string  `json:"beforeDate"`
}

// StockQAValuation — 자기 PER·PBR과 같은 섹터·시장의 중앙값.
type StockQAValuation struct {
	PER           *float64 `json:"per,omitempty"`
	PBR           *float64 `json:"pbr,omitempty"`
	SectorPER     *float64 `json:"sectorPer,omitempty"`
	SectorPBR     *float64 `json:"sectorPbr,omitempty"`
	SectorCount   int      `json:"sectorCount"`
	PERPercentile *float64 `json:"perPercentile,omitempty"` // 점수 산식의 동종 백분위(높을수록 싸다)
}

type StockQADisclosure struct {
	Date  string `json:"date"`
	Title string `json:"title"`
}

type StockQAInsider struct {
	Buys  int `json:"buys"`
	Sells int `json:"sells"`
	Days  int `json:"days"`
}

// StockQARisks — 경고 지정 · 내부자 매도 우세 · 위험 공시.
type StockQARisks struct {
	Warnings        []string            `json:"warnings"`
	Insider         *StockQAInsider     `json:"insider,omitempty"`
	RiskDisclosures []StockQADisclosure `json:"riskDisclosures"`
}

// StockQAMore is the material for the six added questions.
type StockQAMore struct {
	Flows       *StockQAFlows       `json:"flows,omitempty"`
	Estimate    *StockQAEstimate    `json:"estimate,omitempty"`
	Valuation   *StockQAValuation   `json:"valuation,omitempty"`
	Risks       StockQARisks        `json:"risks"`
	Disclosures []StockQADisclosure `json:"disclosures"`
}

// riskDisclosureWords — 제목에 이 말이 있으면 「위험 신호」로 든다. 넓게 잡으면 모든 공시가
// 위험이 되므로 자금 조달(희석)·법적 분쟁·거래 제한만.
var riskDisclosureWords = []string{
	"유상증자", "전환사채", "신주인수권부사채", "교환사채", "감자", "소송", "횡령", "배임",
	"불성실공시", "관리종목", "상장폐지", "거래정지", "매매거래정지", "감사의견", "회생", "파산", "부도",
}

// IsRiskDisclosure reports whether a disclosure title is a risk signal.
func IsRiskDisclosure(title string) bool {
	for _, w := range riskDisclosureWords {
		if strings.Contains(title, w) {
			return true
		}
	}
	return false
}

// FlowStreak counts consecutive same-sign days from the newest (nets newest first).
// Positive = 연속 순매수, negative = 연속 순매도, 0 = 오늘 0.
func FlowStreak(nets []int64) int {
	if len(nets) == 0 || nets[0] == 0 {
		return 0
	}
	sign := int64(1)
	if nets[0] < 0 {
		sign = -1
	}
	n := 0
	for _, v := range nets {
		if v*sign <= 0 {
			break
		}
		n++
	}
	return n * int(sign)
}

// Median of positive values (PER가 음수·0이면 적자·무의미라 뺀다).
func MedianPositive(vals []float64) (float64, int) {
	pos := make([]float64, 0, len(vals))
	for _, v := range vals {
		if v > 0 {
			pos = append(pos, v)
		}
	}
	if len(pos) == 0 {
		return 0, 0
	}
	sort.Float64s(pos)
	m := len(pos) / 2
	if len(pos)%2 == 1 {
		return pos[m], len(pos)
	}
	return (pos[m-1] + pos[m]) / 2, len(pos)
}

// StockQAMoreFor reads the added questions' material. Missing data → nil sections.
func StockQAMoreFor(ctx context.Context, db *sql.DB, symbol string, now time.Time) (*StockQAMore, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	out := &StockQAMore{Risks: StockQARisks{Warnings: []string{}, RiskDisclosures: []StockQADisclosure{}}, Disclosures: []StockQADisclosure{}}

	// ── 외국인·기관 (국내) ──
	rows, err := db.QueryContext(ctx, `
		SELECT DATE_FORMAT(trade_date, '%Y-%m-%d'), foreign_net, institution_net, foreign_hold_rate
		  FROM stock_investor_flows WHERE symbol = ? ORDER BY trade_date DESC LIMIT 21`, symbol)
	if err != nil {
		return nil, fmt.Errorf("qa flows: %w", err)
	}
	var fNets, iNets []int64
	var holds []sql.NullFloat64
	var asOf string
	for rows.Next() {
		var d string
		var f, i int64
		var hold sql.NullFloat64
		if err := rows.Scan(&d, &f, &i, &hold); err != nil {
			rows.Close()
			return nil, err
		}
		if asOf == "" {
			asOf = d
		}
		fNets, iNets, holds = append(fNets, f), append(iNets, i), append(holds, hold)
	}
	rows.Close()
	if len(fNets) >= 5 {
		fl := &StockQAFlows{AsOf: asOf, Days: min(len(fNets), 20)}
		for k := 0; k < len(fNets) && k < 20; k++ {
			if k < 5 {
				fl.Foreign5 += fNets[k]
				fl.Institution5 += iNets[k]
			}
			fl.Foreign20 += fNets[k]
			fl.Institution20 += iNets[k]
		}
		fl.ForeignStreak = FlowStreak(fNets)
		fl.InstitutionStreak = FlowStreak(iNets)
		if holds[0].Valid {
			v := holds[0].Float64
			fl.ForeignHoldRate = &v
		}
		if last := len(holds) - 1; last >= 19 && holds[last].Valid {
			v := holds[last].Float64
			fl.ForeignHoldPrev = &v
		}
		out.Flows = fl
	}

	// ── 애널리스트 추정치 (해외) ── 가장 최근 스냅샷의 분기, 그리고 25일 이상 전의 값.
	var fy int
	var fp, nowDate string
	var nowEst float64
	err = db.QueryRowContext(ctx, `
		SELECT fiscal_year, fiscal_period, DATE_FORMAT(snap_date, '%Y-%m-%d'), eps_est
		  FROM earnings_estimate_revisions WHERE symbol = ?
		 ORDER BY snap_date DESC, fiscal_year ASC, fiscal_period ASC LIMIT 1`, symbol).Scan(&fy, &fp, &nowDate, &nowEst)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("qa estimate: %w", err)
	}
	if err == nil {
		var beforeDate string
		var beforeEst float64
		err = db.QueryRowContext(ctx, `
			SELECT DATE_FORMAT(snap_date, '%Y-%m-%d'), eps_est FROM earnings_estimate_revisions
			 WHERE symbol = ? AND fiscal_year = ? AND fiscal_period = ? AND snap_date <= DATE_SUB(?, INTERVAL 25 DAY)
			 ORDER BY snap_date DESC LIMIT 1`, symbol, fy, fp, nowDate).Scan(&beforeDate, &beforeEst)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("qa estimate before: %w", err)
		}
		if err == nil {
			out.Estimate = &StockQAEstimate{FiscalYear: fy, FiscalPeriod: fp, Now: nowEst, NowDate: nowDate, Before: beforeEst, BeforeDate: beforeDate}
		}
	}

	// ── 싼가 비싼가 ── 최신 점수 행의 raw PER·PBR과 같은 섹터·시장 중앙값.
	var per, pbr, perPct sql.NullFloat64
	var tradeDate string
	err = db.QueryRowContext(ctx, `
		SELECT DATE_FORMAT(trade_date, '%Y-%m-%d'),
		       CAST(JSON_EXTRACT(inputs, '$.raw.per') AS DOUBLE),
		       CAST(JSON_EXTRACT(inputs, '$.raw.pbr') AS DOUBLE),
		       CAST(JSON_EXTRACT(inputs, '$.percentile.per') AS DOUBLE)
		  FROM stock_scores WHERE symbol = ? ORDER BY trade_date DESC LIMIT 1`, symbol).Scan(&tradeDate, &per, &pbr, &perPct)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("qa valuation: %w", err)
	}
	if err == nil && (per.Valid || pbr.Valid) {
		v := &StockQAValuation{}
		if per.Valid {
			x := per.Float64
			v.PER = &x
		}
		if pbr.Valid {
			x := pbr.Float64
			v.PBR = &x
		}
		if perPct.Valid {
			x := perPct.Float64
			v.PERPercentile = &x
		}
		var sector, asset sql.NullString
		if err := db.QueryRowContext(ctx, `
			SELECT ss.sector, s.asset_type FROM stocks s LEFT JOIN stock_sectors ss ON ss.symbol = s.symbol
			 WHERE s.symbol = ?`, symbol).Scan(&sector, &asset); err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("qa valuation sector: %w", err)
		}
		if sector.Valid && sector.String != "" {
			prow, err := db.QueryContext(ctx, `
				SELECT CAST(JSON_EXTRACT(sc.inputs, '$.raw.per') AS DOUBLE),
				       CAST(JSON_EXTRACT(sc.inputs, '$.raw.pbr') AS DOUBLE)
				  FROM stock_scores sc
				  JOIN stock_sectors ss ON ss.symbol = sc.symbol
				  JOIN stocks s ON s.symbol = sc.symbol
				 WHERE sc.trade_date = ? AND ss.sector = ? AND s.asset_type = ?`, tradeDate, sector.String, asset.String)
			if err != nil {
				return nil, fmt.Errorf("qa valuation peers: %w", err)
			}
			var pers, pbrs []float64
			for prow.Next() {
				var a, b sql.NullFloat64
				if err := prow.Scan(&a, &b); err != nil {
					prow.Close()
					return nil, err
				}
				if a.Valid {
					pers = append(pers, a.Float64)
				}
				if b.Valid {
					pbrs = append(pbrs, b.Float64)
				}
			}
			prow.Close()
			if m, n := MedianPositive(pers); n >= 5 {
				v.SectorPER, v.SectorCount = &m, n
			}
			if m, n := MedianPositive(pbrs); n >= 5 {
				v.SectorPBR = &m
				if n > v.SectorCount {
					v.SectorCount = n
				}
			}
		}
		out.Valuation = v
	}

	// ── 위험 신호 ──
	wrows, err := db.QueryContext(ctx, `SELECT warning_type FROM stock_warnings WHERE symbol = ?`, symbol)
	if err != nil {
		return nil, fmt.Errorf("qa warnings: %w", err)
	}
	for wrows.Next() {
		var w string
		if err := wrows.Scan(&w); err != nil {
			wrows.Close()
			return nil, err
		}
		out.Risks.Warnings = append(out.Risks.Warnings, w)
	}
	wrows.Close()

	since := now.AddDate(0, 0, -90).Format("2006-01-02")
	var buys, sells sql.NullInt64
	if err := db.QueryRowContext(ctx, `
		SELECT SUM(shares_change > 0), SUM(shares_change < 0) FROM insider_transactions
		 WHERE symbol = ? AND txn_date >= ?`, symbol, since).Scan(&buys, &sells); err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("qa insider: %w", err)
	}
	if buys.Int64+sells.Int64 > 0 {
		out.Risks.Insider = &StockQAInsider{Buys: int(buys.Int64), Sells: int(sells.Int64), Days: 90}
	}

	drows, err := db.QueryContext(ctx, `
		SELECT DATE_FORMAT(receipt_date, '%Y-%m-%d'), title FROM stock_disclosures
		 WHERE symbol = ? AND receipt_date >= ? ORDER BY receipt_date DESC, id DESC LIMIT 60`, symbol, since)
	if err != nil {
		return nil, fmt.Errorf("qa disclosures: %w", err)
	}
	recent := now.AddDate(0, 0, -30).Format("2006-01-02")
	for drows.Next() {
		var d StockQADisclosure
		if err := drows.Scan(&d.Date, &d.Title); err != nil {
			drows.Close()
			return nil, err
		}
		if d.Date >= recent && len(out.Disclosures) < 5 {
			out.Disclosures = append(out.Disclosures, d)
		}
		if IsRiskDisclosure(d.Title) && len(out.Risks.RiskDisclosures) < 3 {
			out.Risks.RiskDisclosures = append(out.Risks.RiskDisclosures, d)
		}
	}
	drows.Close()
	return out, nil
}
