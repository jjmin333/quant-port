package repository

import (
	"context"
	"database/sql"
	"strings"
)

/*
애널리스트 의견 이력 · 증권사 리포트 (099, 2026-10-07). 표를 왜 이렇게 두는지는
`migrations/099_analyst_history.sql`. 쓰는 쪽은 worker의 외부 신호 잡뿐이고, 화면은
`GET /stocks/{symbol}/analyst`가 읽는다.
*/

// AnalystHistoryRow — 한 시점의 컨센서스. 해외는 월 1줄, 국내는 값이 바뀐 날 1줄.
type AnalystHistoryRow struct {
	AsOf           string   `json:"asOf"`
	Source         string   `json:"source"`
	Rating         *float64 `json:"rating,omitempty"`
	TargetMean     *float64 `json:"targetMean,omitempty"`
	TargetCurrency string   `json:"targetCurrency,omitempty"`
	Counts         *[5]int  `json:"counts,omitempty"` // 강력매수·매수·보유·매도·강력매도
}

// AnalystReport — 증권사 리포트 한 건(제목·증권사·날짜, 본문은 링크로).
type AnalystReport struct {
	ID        int64  `json:"id"`
	Source    string `json:"source"`
	Broker    string `json:"broker"`
	Title     string `json:"title"`
	Published string `json:"published,omitempty"`
}

// UpsertAnalystHistory — 같은 (종목, 출처, 기준일)이면 덮어쓴다(Finnhub는 같은 달을 매번 다시 준다).
func (r *ExternalSignalRepo) UpsertAnalystHistory(ctx context.Context, symbol string, row AnalystHistoryRow) error {
	var sb, b, h, s, ss any
	if row.Counts != nil {
		sb, b, h, s, ss = row.Counts[0], row.Counts[1], row.Counts[2], row.Counts[3], row.Counts[4]
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO stock_analyst_history
			(symbol, source, as_of, strong_buy, buy, hold, sell, strong_sell, rating, target_mean, target_currency, fetched_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?, NOW(3))
		ON DUPLICATE KEY UPDATE strong_buy=VALUES(strong_buy), buy=VALUES(buy), hold=VALUES(hold),
			sell=VALUES(sell), strong_sell=VALUES(strong_sell), rating=VALUES(rating),
			target_mean=VALUES(target_mean), target_currency=VALUES(target_currency), fetched_at=VALUES(fetched_at)`,
		strings.ToUpper(symbol), row.Source, row.AsOf, sb, b, h, s, ss, row.Rating, row.TargetMean, row.TargetCurrency)
	return err
}

// AnalystHistory — 최근 `limit`개 시점, **오래된 것이 먼저**(차트 순서).
func (r *ExternalSignalRepo) AnalystHistory(ctx context.Context, symbol string, limit int) ([]AnalystHistoryRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(as_of, '%Y-%m-%d'), source, strong_buy, buy, hold, sell, strong_sell,
		       rating, target_mean, target_currency
		  FROM stock_analyst_history WHERE symbol = ?
		 ORDER BY as_of DESC LIMIT ?`, strings.ToUpper(symbol), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AnalystHistoryRow
	for rows.Next() {
		var (
			h               AnalystHistoryRow
			sb, b, hd, s, ss sql.NullInt64
			rating, target  sql.NullFloat64
		)
		if err := rows.Scan(&h.AsOf, &h.Source, &sb, &b, &hd, &s, &ss, &rating, &target, &h.TargetCurrency); err != nil {
			return nil, err
		}
		h.Rating, h.TargetMean = nf(rating), nf(target)
		if sb.Valid || b.Valid || hd.Valid || s.Valid || ss.Valid {
			h.Counts = &[5]int{int(sb.Int64), int(b.Int64), int(hd.Int64), int(s.Int64), int(ss.Int64)}
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// LatestAnalystHistory — 그 출처의 마지막 한 줄(국내 「바뀐 날만 남기기」의 비교 대상). 없으면 nil.
func (r *ExternalSignalRepo) LatestAnalystHistory(ctx context.Context, symbol, source string) (*AnalystHistoryRow, error) {
	var (
		h              AnalystHistoryRow
		rating, target sql.NullFloat64
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT DATE_FORMAT(as_of, '%Y-%m-%d'), source, rating, target_mean
		  FROM stock_analyst_history WHERE symbol = ? AND source = ?
		 ORDER BY as_of DESC LIMIT 1`, strings.ToUpper(symbol), source).
		Scan(&h.AsOf, &h.Source, &rating, &target)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	h.Rating, h.TargetMean = nf(rating), nf(target)
	return &h, nil
}

// UpsertAnalystReports — 이미 있는 리포트는 제목만 갱신(증권사가 제목을 고치는 일이 있다).
func (r *ExternalSignalRepo) UpsertAnalystReports(ctx context.Context, symbol string, reports []AnalystReport) error {
	for _, rep := range reports {
		var pub any
		if rep.Published != "" {
			pub = rep.Published
		}
		if _, err := r.db.ExecContext(ctx, `
			INSERT INTO stock_analyst_reports (source, report_id, symbol, broker, title, published, fetched_at)
			VALUES (?,?,?,?,?,?, NOW(3))
			ON DUPLICATE KEY UPDATE broker=VALUES(broker), title=VALUES(title), published=VALUES(published)`,
			rep.Source, rep.ID, strings.ToUpper(symbol), truncRunes(rep.Broker, 64), truncRunes(rep.Title, 300), pub); err != nil {
			return err
		}
	}
	return nil
}

// AnalystReports — 최근 리포트, 최신 먼저.
func (r *ExternalSignalRepo) AnalystReports(ctx context.Context, symbol string, limit int) ([]AnalystReport, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT report_id, source, broker, title, COALESCE(DATE_FORMAT(published, '%Y-%m-%d'), '')
		  FROM stock_analyst_reports WHERE symbol = ?
		 ORDER BY published DESC, report_id DESC LIMIT ?`, strings.ToUpper(symbol), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AnalystReport
	for rows.Next() {
		var a AnalystReport
		if err := rows.Scan(&a.ID, &a.Source, &a.Broker, &a.Title, &a.Published); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
