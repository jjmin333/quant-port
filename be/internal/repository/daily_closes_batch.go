package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// CloseAt is one stored close.
type CloseAt struct {
	Date  string  `json:"d"`
	Close float64 `json:"c"`
}

/*
ClosesForSymbols — 여러 종목의 일별 종가를 **한 번에** 읽는다(포트폴리오 위험 카드, 2026-09-24).

보유 종목마다 `/stocks/{symbol}/daily-closes`를 부르면 스무 종목에 스무 요청이다. 여기는
IN 한 번이다 — PK(symbol, trade_date)라 종목당 범위 탐색으로 끝난다.
*/
func ClosesForSymbols(ctx context.Context, db *sql.DB, symbols []string, since string) (map[string][]CloseAt, error) {
	out := map[string][]CloseAt{}
	if db == nil || len(symbols) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(symbols)+1)
	for _, s := range symbols {
		args = append(args, s)
	}
	args = append(args, since)
	rows, err := db.QueryContext(ctx, `
		SELECT symbol, DATE_FORMAT(trade_date, '%Y-%m-%d'), close_price
		  FROM stock_daily_closes
		 WHERE symbol IN (`+strings.TrimSuffix(strings.Repeat("?,", len(symbols)), ",")+`)
		   AND trade_date >= ? AND close_price > 0
		 ORDER BY symbol, trade_date`, args...)
	if err != nil {
		return nil, fmt.Errorf("closes batch: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sym string
		var c CloseAt
		if err := rows.Scan(&sym, &c.Date, &c.Close); err != nil {
			return nil, err
		}
		out[sym] = append(out[sym], c)
	}
	return out, rows.Err()
}
