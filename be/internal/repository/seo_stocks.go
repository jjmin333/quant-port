package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

/*
종목 페이지 SEO — 색인할 종목 목록과 서버가 그릴 요약 (2026-09-24 사용자 결정: 상위 1,000종목만 색인).

# 왜 1,000개인가

점수가 있는 종목은 1만 6천 개다. 대부분은 검색 수요가 없는 소형주이고, 그 주소를 전부 올리면
사이트가 「얇은 문서 더미」로 평가된다(`docs/SEO.md`). 많이 거래되는 종목은 검색 수요가 있고 우리
데이터(점수·섹터·변화)도 두텁다. 운영은 OCI 무료 ARM 1대라 크롤러가 여는 주소 수도 곧 비용이다.

# 줄 세우는 잣대: 시가총액이 아니라 20거래일 평균 거래대금

사용자는 「시가총액 상위」로 정했는데 **저장된 시가총액(`inputs.marketCap`)이 믿을 수 없었다.**
실측(2026-09-24): ADR이 원주 수로 부풀어 아카리 테라퓨틱스가 9,250억 달러, 씨티그룹 3.8조 달러 ·
NXP 1조 달러 — 그대로 세우면 상위 30에 소형 바이오가 선다. 종가 × 거래량은 저장된 두 숫자의
곱이라 그런 오류가 없고, 검색 수요(많이 사고파는 종목)에도 더 가깝다. 시가총액 오류는 별도로
고친다.

# 시장별 몫: 국내 500 · 미국 500 (2026-10-09 사용자 결정)

예전에는 원/달러로 환산해 한 줄로 세웠다. 실측(10-09) 1,000개 중 국내가 **34개**였다 — 미국 대형주
거래대금이 한두 자릿수 커서 셀트리온·카카오·삼성바이오로직스까지 noindex였다. 한국어 검색 수요의
대부분이 국내 종목이라 시장마다 따로 세운다. 시장 안에서는 통화가 같아 환율이 필요 없다.
점수가 있는(랭킹 자격 — ETF·우선주 제외) 종목만 대상이다.
*/

const seoIndexablePerMarket = 500

// SEOIndexEntry is one indexable stock — the sitemap reads the symbol, the /stocks hub the rest.
type SEOIndexEntry struct {
	Symbol      string `json:"symbol"`
	Name        string `json:"name"`
	AssetType   string `json:"assetType"`
	Market      string `json:"market"`
	SectorLabel string `json:"sectorLabel,omitempty"`
	Grade       string `json:"grade,omitempty"`
}

// SEOIndexableStocks returns the newest score date and, per market, the top stocks by 20-session
// trade value (domestic first, each in rank order).
func SEOIndexableStocks(ctx context.Context, db *sql.DB) (string, []SEOIndexEntry, error) {
	var date sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT DATE_FORMAT(MAX(trade_date), '%Y-%m-%d') FROM stock_scores`).Scan(&date); err != nil {
		return "", nil, fmt.Errorf("seo latest score date: %w", err)
	}
	if !date.Valid {
		return "", nil, nil
	}
	clause, args := scoreRankableProductClause("s")
	q := `
		SELECT symbol, name, market, asset_type, sector, grade FROM (
			SELECT sc.symbol, s.name, COALESCE(s.market, '') AS market, s.asset_type,
			       COALESCE(ss.sector, '') AS sector, COALESCE(sc.grade, '') AS grade,
			       ROW_NUMBER() OVER (PARTITION BY s.asset_type ORDER BY AVG(c.close_price * c.volume) DESC) AS rn
			  FROM stock_scores sc
			  STRAIGHT_JOIN stocks s ON s.symbol = sc.symbol
			  JOIN stock_daily_closes c ON c.symbol = sc.symbol AND c.trade_date >= ? - INTERVAL 30 DAY
			  LEFT JOIN stock_sectors ss ON ss.symbol = sc.symbol
			 WHERE sc.trade_date = ? AND sc.score IS NOT NULL AND s.asset_type IN ('domestic', 'us')` + clause + `
			 GROUP BY sc.symbol, s.name, s.market, s.asset_type, ss.sector, sc.grade
			HAVING COUNT(*) >= 10
		) ranked
		 WHERE rn <= ?
		 ORDER BY asset_type, rn`
	all := append([]any{date.String, date.String}, args...)
	all = append(all, seoIndexablePerMarket)
	rows, err := db.QueryContext(ctx, q, all...)
	if err != nil {
		return "", nil, fmt.Errorf("seo indexable: %w", err)
	}
	defer rows.Close()
	out := make([]SEOIndexEntry, 0, 2*seoIndexablePerMarket)
	for rows.Next() {
		var e SEOIndexEntry
		var sector string
		if err := rows.Scan(&e.Symbol, &e.Name, &e.Market, &e.AssetType, &sector, &e.Grade); err != nil {
			return "", nil, err
		}
		if sector != "" {
			e.SectorLabel = model.Sector(sector).Label()
		}
		out = append(out, e)
	}
	return date.String, out, rows.Err()
}

// SEOStockSummary is what the server renders into a stock page's HTML.
type SEOStockSummary struct {
	Symbol      string              `json:"symbol"`
	Name        string              `json:"name"`
	Market      string              `json:"market"`
	AssetType   string              `json:"assetType"`
	SectorLabel string              `json:"sectorLabel,omitempty"`
	TradeDate   string              `json:"tradeDate,omitempty"`
	Score       *float64            `json:"score,omitempty"`
	Grade       string              `json:"grade,omitempty"`
	Axes        map[string]*float64 `json:"axes,omitempty"`
	Close       *float64            `json:"close,omitempty"`
	ChangeRate  *float64            `json:"changeRate,omitempty"`
	Currency    string              `json:"currency,omitempty"`
	CloseDate   string              `json:"closeDate,omitempty"`
	// RecentChange is the newest detected change (최근 30일) — 「오늘의 변화」의 한 줄.
	RecentChange *SEOChange `json:"recentChange,omitempty"`
	Indexable    bool       `json:"indexable"`
}

// SEOChange is one change-event line.
type SEOChange struct {
	Date  string `json:"date"`
	Title string `json:"title"`
}

// SEOStockSummaryFor reads one symbol's summary (nil when the symbol is unknown).
func SEOStockSummaryFor(ctx context.Context, db *sql.DB, symbol string) (*SEOStockSummary, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	out := &SEOStockSummary{Symbol: symbol}
	var sector sql.NullString
	err := db.QueryRowContext(ctx, `
		SELECT s.name, COALESCE(s.market, ''), s.asset_type, ss.sector
		  FROM stocks s LEFT JOIN stock_sectors ss ON ss.symbol = s.symbol
		 WHERE s.symbol = ?`, symbol).Scan(&out.Name, &out.Market, &out.AssetType, &sector)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("seo stock: %w", err)
	}
	if sector.Valid {
		out.SectorLabel = model.Sector(sector.String).Label()
	}

	var date, grade sql.NullString
	var score sql.NullFloat64
	var breakdown []byte
	err = db.QueryRowContext(ctx, `
		SELECT DATE_FORMAT(trade_date, '%Y-%m-%d'), score, grade, breakdown
		  FROM stock_scores WHERE symbol = ? ORDER BY trade_date DESC LIMIT 1`, symbol).
		Scan(&date, &score, &grade, &breakdown)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("seo score: %w", err)
	}
	if err == nil {
		out.TradeDate = date.String
		if score.Valid {
			out.Score = &score.Float64
		}
		out.Grade = grade.String
		axes := map[string]*float64{}
		if len(breakdown) > 0 && json.Unmarshal(breakdown, &axes) == nil {
			out.Axes = axes
		}
	}

	var closeDate, currency sql.NullString
	var closeV, chg sql.NullFloat64
	err = db.QueryRowContext(ctx, `
		SELECT DATE_FORMAT(trade_date, '%Y-%m-%d'), close_price, change_rate, currency
		  FROM stock_daily_closes WHERE symbol = ? ORDER BY trade_date DESC LIMIT 1`, symbol).
		Scan(&closeDate, &closeV, &chg, &currency)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("seo close: %w", err)
	}
	if err == nil {
		out.CloseDate, out.Currency = closeDate.String, currency.String
		if closeV.Valid {
			out.Close = &closeV.Float64
		}
		if chg.Valid {
			out.ChangeRate = &chg.Float64
		}
	}

	var chDate, chTitle string
	err = db.QueryRowContext(ctx, `
		SELECT DATE_FORMAT(event_date, '%Y-%m-%d'), title FROM stock_change_events
		 WHERE symbol = ? AND detected_for >= CURDATE() - INTERVAL 30 DAY
		 ORDER BY detected_for DESC, importance DESC LIMIT 1`, symbol).Scan(&chDate, &chTitle)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("seo change: %w", err)
	}
	if err == nil {
		out.RecentChange = &SEOChange{Date: chDate, Title: chTitle}
	}
	return out, nil
}
