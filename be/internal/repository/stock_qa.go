package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

/*
종목 질문 칩의 재료 (2026-09-24 사용자 결정: 규칙 기반 종목 Q&A, 개요 탭 질문 칩).

LLM을 부르지 않는다 — 답은 저장된 표에서 나온 사실을 문장 틀에 끼운 것이다. 이 파일은
「왜 움직였나」(최근 변화 감지 · 마지막 등락)와 「비슷한 종목」(같은 섹터 · 같은 시장에서
**시가총액이 가까운** 다섯) 두 질문의 재료만 준다. 점수 상위로 고르면 삼성전자 옆에 소형주가
섰다(실측) — 「비슷하다」는 사업(섹터)과 크기가 같다는 뜻이다. 「점수가 왜」·「다음 실적」은 기존 API(`score/explain` ·
`earnings`)가 이미 답을 가지고 있어 화면이 그쪽을 부른다.
*/

// StockQAChange is one detected change for the 「왜 움직였나」 answer.
type StockQAChange struct {
	Date      string   `json:"date"`
	Kind      string   `json:"kind"`
	Direction int      `json:"direction"`
	Title     string   `json:"title"`
	Explained *bool    `json:"explained,omitempty"`
	Magnitude *float64 `json:"magnitude,omitempty"`
}

// StockQAPeer is one same-sector name.
type StockQAPeer struct {
	Symbol string  `json:"symbol"`
	Name   string  `json:"name"`
	Score  float64 `json:"score"`
	Grade  string  `json:"grade,omitempty"`
}

// StockQA bundles the two answers' material.
type StockQA struct {
	Symbol      string          `json:"symbol"`
	LastMove    *StockQAMove    `json:"lastMove,omitempty"`
	Changes     []StockQAChange `json:"changes"`
	SectorLabel string          `json:"sectorLabel,omitempty"`
	Peers       []StockQAPeer   `json:"peers"`
	// More — 질문 칩 확장(2026-09-27) 재료. stock_qa_more.go.
	More *StockQAMore `json:"more,omitempty"`
}

// StockQAMove is the newest session's close-to-close move.
type StockQAMove struct {
	Date       string  `json:"date"`
	ChangeRate float64 `json:"changeRate"`
}

// StockQAFor reads the material for one symbol. Unknown symbol → empty lists, no error.
func StockQAFor(ctx context.Context, db *sql.DB, symbol string) (*StockQA, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	out := &StockQA{Symbol: symbol, Changes: []StockQAChange{}, Peers: []StockQAPeer{}}

	var d string
	var rate sql.NullFloat64
	err := db.QueryRowContext(ctx, `
		SELECT DATE_FORMAT(trade_date, '%Y-%m-%d'), change_rate FROM stock_daily_closes
		 WHERE symbol = ? ORDER BY trade_date DESC LIMIT 1`, symbol).Scan(&d, &rate)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("qa last move: %w", err)
	}
	if err == nil && rate.Valid {
		out.LastMove = &StockQAMove{Date: d, ChangeRate: rate.Float64}
	}

	rows, err := db.QueryContext(ctx, `
		SELECT DATE_FORMAT(event_date, '%Y-%m-%d'), kind, direction, title, explained, magnitude
		  FROM stock_change_events
		 WHERE symbol = ? AND detected_for >= CURDATE() - INTERVAL 30 DAY
		 ORDER BY event_date DESC, importance DESC LIMIT 5`, symbol)
	if err != nil {
		return nil, fmt.Errorf("qa changes: %w", err)
	}
	for rows.Next() {
		var c StockQAChange
		var explained sql.NullBool
		var mag sql.NullFloat64
		if err := rows.Scan(&c.Date, &c.Kind, &c.Direction, &c.Title, &explained, &mag); err != nil {
			rows.Close()
			return nil, err
		}
		if explained.Valid {
			c.Explained = &explained.Bool
		}
		if mag.Valid {
			c.Magnitude = &mag.Float64
		}
		out.Changes = append(out.Changes, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var sector, asset sql.NullString
	err = db.QueryRowContext(ctx, `
		SELECT ss.sector, s.asset_type FROM stocks s
		  LEFT JOIN stock_sectors ss ON ss.symbol = s.symbol WHERE s.symbol = ?`, symbol).Scan(&sector, &asset)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("qa sector: %w", err)
	}
	if !sector.Valid || sector.String == "" {
		return out, nil
	}
	out.SectorLabel = model.Sector(sector.String).Label()
	var myCap sql.NullFloat64
	if err := db.QueryRowContext(ctx, `
		SELECT CAST(JSON_EXTRACT(inputs, '$.marketCap') AS DOUBLE) FROM stock_scores
		 WHERE symbol = ? ORDER BY trade_date DESC LIMIT 1`, symbol).Scan(&myCap); err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("qa market cap: %w", err)
	}
	// 시가총액을 모르면 점수 순서로 물러선다.
	order := ` ORDER BY sc.score DESC`
	var orderArgs []any
	if myCap.Valid && myCap.Float64 > 0 {
		order = ` AND CAST(JSON_EXTRACT(sc.inputs, '$.marketCap') AS DOUBLE) > 0
		 ORDER BY ABS(LN(CAST(JSON_EXTRACT(sc.inputs, '$.marketCap') AS DOUBLE)) - LN(?))`
		orderArgs = []any{myCap.Float64}
	}
	clause, args := scoreRankableProductClause("s")
	q := `
		SELECT sc.symbol, s.name, sc.score, COALESCE(sc.grade, '')
		  FROM stock_scores sc
		  STRAIGHT_JOIN stocks s ON s.symbol = sc.symbol
		  JOIN stock_sectors ss ON ss.symbol = sc.symbol
		 WHERE sc.trade_date = (SELECT MAX(trade_date) FROM stock_scores)
		   AND ss.sector = ? AND s.asset_type = ? AND sc.symbol <> ? AND sc.score IS NOT NULL` + clause + order + `
		 LIMIT 5`
	all := append([]any{sector.String, asset.String, symbol}, args...)
	all = append(all, orderArgs...)
	prow, err := db.QueryContext(ctx, q, all...)
	if err != nil {
		return nil, fmt.Errorf("qa peers: %w", err)
	}
	defer prow.Close()
	for prow.Next() {
		var p StockQAPeer
		if err := prow.Scan(&p.Symbol, &p.Name, &p.Score, &p.Grade); err != nil {
			return nil, err
		}
		out.Peers = append(out.Peers, p)
	}
	return out, prow.Err()
}
