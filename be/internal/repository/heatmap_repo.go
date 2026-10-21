package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

/*
HeatmapRepo — 시장 히트맵(`/market/heatmap`, 2026-10-21 티켓 QP-10, 참고: Finviz 맵).

시가총액 상위 N 보통주를 업종별로 묶어 그날 등락률과 **점수 변화**(직전 점수일 대비)를 준다.
점수 변화 색은 경쟁 앱에 없는 우리 데이터라 이 화면의 이유다. 새로 저장하는 것은 없다 —
`stock_daily_closes`(최신 거래일)·`stocks`(상장주식수)·`stock_sectors`·`stock_scores`만 읽는다.
*/
type HeatmapRepo struct {
	db *sql.DB
}

func NewHeatmapRepo(db *sql.DB) *HeatmapRepo { return &HeatmapRepo{db: db} }

// HeatmapTile is one stock on the map.
type HeatmapTile struct {
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name"`
	Sector     string  `json:"sector"`
	MarketCap  float64 `json:"marketCap"`
	ChangeRate float64 `json:"changeRate"`
	Score      *int    `json:"score,omitempty"`
	ScoreDelta *int    `json:"scoreDelta,omitempty"`
}

// HeatmapData is one market's map.
type HeatmapData struct {
	Market    string        `json:"market"`
	TradeDate string        `json:"tradeDate"`
	ScoreDate string        `json:"scoreDate,omitempty"`
	Tiles     []HeatmapTile `json:"tiles"`
}

// heatmapUniverse — 시장별 통화와 보통주 종류(ETF·ETN·ETP 제외).
var heatmapUniverse = map[string]struct {
	currency string
	types    string
}{
	"domestic": {"KRW", `'주권','STOCK'`},
	"us":       {"USD", `'STOCK','DEPOSITARY_RECEIPT'`},
}

// Load reads the latest trading day's top-n by market cap for market ("domestic" | "us").
func (r *HeatmapRepo) Load(ctx context.Context, market string, n int) (*HeatmapData, error) {
	u, ok := heatmapUniverse[market]
	if !ok {
		return nil, fmt.Errorf("unknown market %q", market)
	}
	out := &HeatmapData{Market: market, Tiles: []HeatmapTile{}}
	// 범위를 열흘로 묶어 trade_date 색인만 탄다.
	if err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(DATE_FORMAT(MAX(trade_date), '%Y-%m-%d'), '') FROM stock_daily_closes
		 WHERE trade_date >= CURDATE() - INTERVAL 10 DAY AND currency = ?`, u.currency).Scan(&out.TradeDate); err != nil {
		return nil, fmt.Errorf("heatmap date: %w", err)
	}
	if out.TradeDate == "" {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.symbol, s.name, COALESCE(sec.sector, ''), c.close_price * s.shares_outstanding, COALESCE(c.change_rate, 0)
		  FROM stock_daily_closes c
		  JOIN stocks s ON s.symbol = c.symbol
		  LEFT JOIN stock_sectors sec ON sec.symbol = c.symbol
		 WHERE c.trade_date = ? AND c.currency = ? AND s.asset_type = ?
		   AND s.security_type IN (`+u.types+`) AND s.shares_outstanding > 0
		 ORDER BY c.close_price * s.shares_outstanding DESC
		 LIMIT ?`, out.TradeDate, u.currency, market, n)
	if err != nil {
		return nil, fmt.Errorf("heatmap tiles: %w", err)
	}
	defer rows.Close()
	idx := map[string]int{}
	for rows.Next() {
		var t HeatmapTile
		if err := rows.Scan(&t.Symbol, &t.Name, &t.Sector, &t.MarketCap, &t.ChangeRate); err != nil {
			return nil, err
		}
		idx[t.Symbol] = len(out.Tiles)
		out.Tiles = append(out.Tiles, t)
	}
	if err := rows.Err(); err != nil || len(out.Tiles) == 0 {
		return out, err
	}
	return out, r.attachScores(ctx, out, idx)
}

// attachScores fills Score/ScoreDelta from the latest two composite score dates.
func (r *HeatmapRepo) attachScores(ctx context.Context, out *HeatmapData, idx map[string]int) error {
	var dates []string
	drows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT DATE_FORMAT(trade_date, '%Y-%m-%d') d FROM stock_scores
		 WHERE trade_date >= CURDATE() - INTERVAL 14 DAY ORDER BY d DESC LIMIT 2`)
	if err != nil {
		return fmt.Errorf("heatmap score dates: %w", err)
	}
	for drows.Next() {
		var d string
		if err := drows.Scan(&d); err != nil {
			drows.Close()
			return err
		}
		dates = append(dates, d)
	}
	drows.Close()
	if len(dates) == 0 {
		return nil
	}
	out.ScoreDate = dates[0]
	args := []any{}
	marks := make([]string, 0, len(idx))
	for sym := range idx {
		marks = append(marks, "?")
		args = append(args, sym)
	}
	args = append(args, dates[0], dates[len(dates)-1])
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, DATE_FORMAT(trade_date, '%Y-%m-%d'), score FROM stock_scores
		 WHERE symbol IN (`+strings.Join(marks, ",")+`) AND trade_date IN (?, ?) AND score_kind = 'composite'`, args...)
	if err != nil {
		return fmt.Errorf("heatmap scores: %w", err)
	}
	defer rows.Close()
	prev := map[string]int{}
	for rows.Next() {
		var sym, d string
		var score int
		if err := rows.Scan(&sym, &d, &score); err != nil {
			return err
		}
		if d == dates[0] {
			v := score
			out.Tiles[idx[sym]].Score = &v
		} else {
			prev[sym] = score
		}
	}
	if len(dates) < 2 {
		return rows.Err()
	}
	for sym, i := range idx {
		if cur := out.Tiles[i].Score; cur != nil {
			if p, ok := prev[sym]; ok {
				d := *cur - p
				out.Tiles[i].ScoreDelta = &d
			}
		}
	}
	return rows.Err()
}
