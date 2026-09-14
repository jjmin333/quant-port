package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// DailyClosePoint is a (trade_date, close) pair. Stamp performance needs the
// date alongside the price — "the close on or before D+30" is only meaningful
// if we can tell whether that close is actually after the stamp.
type DailyClosePoint struct {
	Date  string // YYYY-MM-DD
	Close float64
}

// ClosesSince returns a symbol's closes from `since` (inclusive), ascending.
// One query per symbol lets a whole stamp list resolve every horizon in memory
// instead of issuing four point lookups per stamp.
func (r *StockDailyCloseRepo) ClosesSince(ctx context.Context, symbol, since string) ([]DailyClosePoint, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT trade_date, close_price
		  FROM stock_daily_closes
		 WHERE symbol = ? AND trade_date >= ?
		 ORDER BY trade_date ASC`,
		strings.ToUpper(strings.TrimSpace(symbol)), since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClosePoints(rows)
}

// SeriesRequest asks for one key's closes from `Since` (inclusive).
type SeriesRequest struct {
	Key   string // symbol, or index id for the index variant
	Since string // YYYY-MM-DD
}

// sqlQuerier is the read half of *sql.DB, so the shared helper below can serve
// both repositories without either exposing its handle.
type sqlQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func (r *StockDailyCloseRepo) dbOrNil() sqlQuerier {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db
}

func (r *IndexDailyCloseRepo) dbOrNil() sqlQuerier {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db
}

/*
seriesChunk — 한 문장에 담는 키 수의 상한.

조건이 키마다 하나씩 붙어 문장이 길어진다. 200이면 최악의 경우에도 문장이
수십 KB라 `max_allowed_packet`(기본 64MB)과는 무관하고, 옵티마이저가 다루는
범위 개수도 감당할 만하다.
*/
const seriesChunk = 200

/*
ClosesSinceMulti는 여러 키의 종가를 **한 번의 왕복**으로 받는다.

왜 필요한가: 리서치 피드는 한 페이지의 심볼마다 `ClosesSince`를 한 번씩 불렀다
(docs/PERFORMANCE_AUDIT.md P1-7). 쿼리 하나하나는 PK 레인지 스캔이라 싸지만
(50종목 표본에서 각 0.05ms), **왕복이 심볼 수만큼** 생기고 그동안 커넥션을
붙잡는다. 커넥션 여유가 크지 않은 배포(P1-9)에서는 이쪽이 더 아프다.

키마다 하한을 따로 거는 이유: 공통 하한(가장 이른 날짜)으로 묶으면 문장은 더
짧지만 필요 없는 행을 읽는다. 실측(50키, 서로 다른 하한):

	공통 하한  2.23ms / 2,738행
	개별 하한  3.17ms / 1,660행   ← 39% 적게 읽는다

둘 다 PRIMARY(symbol, trade_date)를 다중 범위로 탄다. 의미가 정확한 쪽을 골랐다 —
성능을 위해 "조금 더 읽어도 되는" 데이터로 바꾸지 않는다.
*/
func (r *StockDailyCloseRepo) ClosesSinceMulti(
	ctx context.Context, reqs []SeriesRequest,
) (map[string][]DailyClosePoint, error) {
	return closesSinceMulti(ctx, r.dbOrNil(), reqs,
		"stock_daily_closes", "symbol", "close_price", strings.ToUpper)
}

// ClosesSinceMulti mirrors the stock variant for benchmark indices.
func (r *IndexDailyCloseRepo) ClosesSinceMulti(
	ctx context.Context, reqs []SeriesRequest,
) (map[string][]DailyClosePoint, error) {
	return closesSinceMulti(ctx, r.dbOrNil(), reqs,
		"index_daily_closes", "index_id", "close_value", strings.ToLower)
}

func closesSinceMulti(
	ctx context.Context,
	db sqlQuerier,
	reqs []SeriesRequest,
	table, keyCol, valueCol string,
	normalize func(string) string,
) (map[string][]DailyClosePoint, error) {
	out := map[string][]DailyClosePoint{}
	if db == nil || len(reqs) == 0 {
		return out, nil
	}

	// 같은 키가 두 번 오면 **더 이른 하한**을 남긴다 — 둘 중 하나만 받으면
	// 늦은 쪽 호출자가 필요한 구간을 못 받는다.
	earliest := map[string]string{}
	order := make([]string, 0, len(reqs))
	for _, req := range reqs {
		key := normalize(strings.TrimSpace(req.Key))
		since := strings.TrimSpace(req.Since)
		if key == "" || since == "" {
			continue
		}
		if cur, ok := earliest[key]; ok {
			if since < cur {
				earliest[key] = since
			}
			continue
		}
		earliest[key] = since
		order = append(order, key)
	}
	if len(order) == 0 {
		return out, nil
	}

	for start := 0; start < len(order); start += seriesChunk {
		end := min(start+seriesChunk, len(order))
		chunk := order[start:end]

		var sb strings.Builder
		sb.WriteString("SELECT " + keyCol + ", trade_date, " + valueCol + " FROM " + table + " WHERE ")
		args := make([]any, 0, len(chunk)*2)
		for i, key := range chunk {
			if i > 0 {
				sb.WriteString(" OR ")
			}
			sb.WriteString("(" + keyCol + " = ? AND trade_date >= ?)")
			args = append(args, key, earliest[key])
		}
		sb.WriteString(" ORDER BY " + keyCol + ", trade_date ASC")

		rows, err := db.QueryContext(ctx, sb.String(), args...)
		if err != nil {
			return nil, err
		}
		err = func() error {
			defer rows.Close()
			for rows.Next() {
				var key string
				var date time.Time
				var value float64
				if err := rows.Scan(&key, &date, &value); err != nil {
					return err
				}
				k := normalize(strings.TrimSpace(key))
				out[k] = append(out[k], DailyClosePoint{
					Date:  date.Format("2006-01-02"),
					Close: value,
				})
			}
			return rows.Err()
		}()
		if err != nil {
			return nil, err
		}
	}

	// 요청했는데 행이 없는 키도 **빈 슬라이스로** 채워 둔다 — 호출자가
	// "없음"과 "아직 안 물어봄"을 구분할 수 있어야 한다.
	for _, key := range order {
		if _, ok := out[key]; !ok {
			out[key] = []DailyClosePoint{}
		}
	}
	return out, nil
}

// ClosesSince mirrors the stock variant for benchmark indices.
func (r *IndexDailyCloseRepo) ClosesSince(ctx context.Context, indexID, since string) ([]DailyClosePoint, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT trade_date, close_value
		  FROM index_daily_closes
		 WHERE index_id = ? AND trade_date >= ?
		 ORDER BY trade_date ASC`,
		strings.ToLower(strings.TrimSpace(indexID)), since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClosePoints(rows)
}

func scanClosePoints(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]DailyClosePoint, error) {
	out := make([]DailyClosePoint, 0)
	for rows.Next() {
		var (
			date  time.Time
			close float64
		)
		if err := rows.Scan(&date, &close); err != nil {
			return nil, err
		}
		out = append(out, DailyClosePoint{Date: date.Format(dailyCloseDateLayout), Close: close})
	}
	return out, rows.Err()
}
