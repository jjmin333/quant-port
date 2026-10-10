package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

/*
「이 종목에 대한 내 기록」 — **조회 한 번** (V2 요구 9).

다섯 표(`user_holdings`·`watchlist_items`·`stamps`·`user_price_targets`·
`research_calls`)를 각각 물으면 종목을 열 때마다 왕복이 다섯이다. 전부
`(user_id, symbol)` 단일 조회라 스칼라 서브쿼리로 묶는다.

각 서브쿼리는 자기 표의 인덱스를 탄다 — 이 앱의 사용자별 표는 전부 `user_id`를
첫 칸으로 둔 키가 있다.
*/

// SymbolContextRepo reads one user's footprint on one symbol.
type SymbolContextRepo struct{ db *sql.DB }

func NewSymbolContextRepo(db *sql.DB) *SymbolContextRepo { return &SymbolContextRepo{db: db} }

/*
Get — 다섯 사실을 한 번에.

`retracted` 리포트는 세지 않는다. 철회한 글을 「썼다」로 세면 화면이 없는 링크를
가리킨다 — 목록에서 이미 빠진 글이다.
*/
func (r *SymbolContextRepo) Get(ctx context.Context, userID int64, symbol string) (model.SymbolContext, error) {
	out := model.SymbolContext{Symbol: symbol}
	if r == nil || r.db == nil || userID == 0 || symbol == "" {
		return out, nil
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))

	var (
		qty      sql.NullFloat64
		avg      sql.NullFloat64
		currency sql.NullString
		watch    sql.NullInt64
		stamps   sql.NullInt64
		reports  sql.NullInt64
		target   sql.NullFloat64
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT
		  (SELECT h.quantity      FROM user_holdings h      WHERE h.user_id = ? AND h.symbol = ? LIMIT 1),
		  (SELECT h.avg_price     FROM user_holdings h      WHERE h.user_id = ? AND h.symbol = ? LIMIT 1),
		  (SELECT h.cost_currency FROM user_holdings h      WHERE h.user_id = ? AND h.symbol = ? LIMIT 1),
		  (SELECT COUNT(*)        FROM watchlist_items w    WHERE w.user_id = ? AND w.symbol = ?),
		  (SELECT COUNT(*)        FROM stamps s             WHERE s.user_id = ? AND s.symbol = ?),
		  (SELECT COUNT(*)        FROM research_calls c     WHERE c.user_id = ? AND c.symbol = ?
		                                                     AND c.status <> 'retracted'),
		  (SELECT t.target_price  FROM user_price_targets t WHERE t.user_id = ? AND t.symbol = ?
		                                                    ORDER BY t.updated_at DESC LIMIT 1)`,
		userID, sym, userID, sym, userID, sym,
		userID, sym, userID, sym, userID, sym, userID, sym,
	).Scan(&qty, &avg, &currency, &watch, &stamps, &reports, &target)
	if err != nil {
		return out, fmt.Errorf("symbol context: %w", err)
	}

	/*
		수량이 0이면 **보유가 아니다.** 전량 매도해도 `user_holdings` 행은 남을 수
		있어서, 행의 존재만 보면 판 종목에 「보유 중」이 붙는다.
	*/
	if qty.Valid && qty.Float64 > 0 {
		out.Held = true
		out.Quantity = qty.Float64
		if avg.Valid {
			out.AvgPrice = &avg.Float64
		}
		out.Currency = currency.String
	}
	out.Watchlisted = watch.Int64 > 0
	out.StampCount = int(stamps.Int64)
	out.ReportCount = int(reports.Int64)
	if target.Valid && target.Float64 > 0 {
		out.TargetPrice = &target.Float64
	}
	return out, nil
}
