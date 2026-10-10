package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

/*
포트폴리오 진단의 재료 — **보유 종목 수와 무관하게 조회 네 번**.

# 왜 종목마다 돌지 않는가

종목별로 실적·내부자·점수·종가를 따로 물으면 20종목에 60회 이상이다. 「항목당 I/O
금지」는 이 저장소의 첫 번째 응답 속도 원칙이다(PRINCIPLES).

대신 **가장 이른 매수일 하나**로 전 종목을 한 번에 긁고, 보유 기간별로 자르는 일은
Go가 한다. 읽는 행이 조금 늘지만 왕복이 네 번으로 고정된다.
*/

type HoldingsDiagnosisRepo struct {
	db *sql.DB
}

func NewHoldingsDiagnosisRepo(db *sql.DB) *HoldingsDiagnosisRepo {
	return &HoldingsDiagnosisRepo{db: db}
}

// DiagnosisEarningsRow is one earnings event for one symbol.
type DiagnosisEarningsRow struct {
	Symbol     string
	Date       string
	FiscalYear sql.NullInt64
	Period     sql.NullString
	EPSAct     sql.NullFloat64
	EPSEst     sql.NullFloat64
}

// EarningsSince returns every earnings event for these symbols on/after `since`.
func (r *HoldingsDiagnosisRepo) EarningsSince(
	ctx context.Context, symbols []string, since string,
) (map[string][]DiagnosisEarningsRow, error) {
	if r == nil || r.db == nil || len(symbols) == 0 {
		return nil, nil
	}
	in, args := symbolPlaceholders(symbols)
	args = append(args, since)
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, DATE_FORMAT(event_date, '%Y-%m-%d'),
		       fiscal_year, fiscal_period, eps_act, eps_est
		  FROM earnings_events
		 WHERE symbol IN (`+in+`) AND event_date >= ?
		 ORDER BY symbol, event_date`, args...)
	if err != nil {
		return nil, fmt.Errorf("diagnosis earnings: %w", err)
	}
	defer rows.Close()

	out := map[string][]DiagnosisEarningsRow{}
	for rows.Next() {
		var e DiagnosisEarningsRow
		if err := rows.Scan(&e.Symbol, &e.Date, &e.FiscalYear, &e.Period, &e.EPSAct, &e.EPSEst); err != nil {
			return nil, err
		}
		out[e.Symbol] = append(out[e.Symbol], e)
	}
	return out, rows.Err()
}

// DiagnosisInsiderRow is one day's insider tally for one symbol.
type DiagnosisInsiderRow struct {
	Symbol     string
	Date       string
	Count      int
	BuyShares  float64
	SellShares float64
}

// InsiderSince aggregates insider trades per (symbol, day) on/after `since`.
//
// 날짜까지 남기는 이유: 보유 기간이 종목마다 다르므로 **Go가 잘라야 한다**. 여기서
// 종목별로 합쳐 버리면 3월에 산 종목과 9월에 산 종목이 같은 기간으로 계산된다.
func (r *HoldingsDiagnosisRepo) InsiderSince(
	ctx context.Context, symbols []string, since string,
) (map[string][]DiagnosisInsiderRow, error) {
	if r == nil || r.db == nil || len(symbols) == 0 {
		return nil, nil
	}
	in, args := symbolPlaceholders(symbols)
	args = append(args, since)
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, DATE_FORMAT(txn_date, '%Y-%m-%d') d, COUNT(*),
		       COALESCE(SUM(CASE WHEN shares_change > 0 THEN shares_change ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN shares_change < 0 THEN -shares_change ELSE 0 END), 0)
		  FROM insider_transactions
		 WHERE symbol IN (`+in+`) AND txn_date >= ?
		 GROUP BY symbol, d`, args...)
	if err != nil {
		return nil, fmt.Errorf("diagnosis insider: %w", err)
	}
	defer rows.Close()

	out := map[string][]DiagnosisInsiderRow{}
	for rows.Next() {
		var i DiagnosisInsiderRow
		if err := rows.Scan(&i.Symbol, &i.Date, &i.Count, &i.BuyShares, &i.SellShares); err != nil {
			return nil, err
		}
		out[i.Symbol] = append(out[i.Symbol], i)
	}
	return out, rows.Err()
}

// DiagnosisCloseRow is the last stored close for one symbol.
type DiagnosisCloseRow struct {
	Symbol string
	Date   string
	Close  float64
	/*
		AvgVolume — 보관 창 안의 **하루 평균 거래량**. 내부자 순매매를 여기에 견줘야
		크기가 생긴다 (`model.DiagnosisInsiderVolumePct`). 0이면 잴 수 없었다는 뜻이고,
		그때는 유의성 문턱을 적용하지 않는다 — 못 잰 것을 「작다」고 부르지 않는다.
	*/
	AvgVolume float64
}

/*
LastCloses — 종목마다 **가장 최근 종가** 한 줄.

실시간 시세가 아니다. 진단은 하루 한 번 도는 값들(점수·실적·내부자)을 읽는 화면이라
가격만 초 단위로 맞출 이유가 없고, 시세 경로를 끌어오면 이 조회가 업스트림에 묶인다.
화면이 「종가 기준」임을 적는다.
*/
func (r *HoldingsDiagnosisRepo) LastCloses(
	ctx context.Context, symbols []string, since string,
) (map[string]DiagnosisCloseRow, error) {
	if r == nil || r.db == nil || len(symbols) == 0 {
		return nil, nil
	}
	in, symArgs := symbolPlaceholders(symbols)
	// 평균 거래량은 같은 조회에 얹는다 — 질의 하나가 늘면 보유 수와 무관하게
	// 늘지만, 왕복 하나를 아끼는 값은 보유 수와 무관하게 있다. 종목 목록이
	// 두 번 들어가므로 순서는 「종목들 · since · 종목들」이다.
	args := make([]any, 0, len(symArgs)*2+1)
	args = append(args, symArgs...)
	args = append(args, since)
	args = append(args, symArgs...)
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.symbol, DATE_FORMAT(c.trade_date, '%Y-%m-%d'), c.close_price,
		       COALESCE(v.avg_volume, 0)
		  FROM stock_daily_closes c
		  JOIN (
		        SELECT symbol, MAX(trade_date) AS trade_date
		          FROM stock_daily_closes
		         WHERE symbol IN (`+in+`)
		         GROUP BY symbol
		       ) last
		    ON last.symbol = c.symbol AND last.trade_date = c.trade_date
		  LEFT JOIN (
		        SELECT symbol, AVG(volume) AS avg_volume
		          FROM stock_daily_closes
		         WHERE trade_date >= ? AND symbol IN (`+in+`)
		         GROUP BY symbol
		       ) v ON v.symbol = c.symbol`, args...)
	if err != nil {
		return nil, fmt.Errorf("diagnosis closes: %w", err)
	}
	defer rows.Close()

	out := map[string]DiagnosisCloseRow{}
	for rows.Next() {
		var c DiagnosisCloseRow
		if err := rows.Scan(&c.Symbol, &c.Date, &c.Close, &c.AvgVolume); err != nil {
			return nil, err
		}
		out[c.Symbol] = c
	}
	return out, rows.Err()
}

func symbolPlaceholders(symbols []string) (string, []any) {
	seen := map[string]struct{}{}
	args := make([]any, 0, len(symbols))
	marks := make([]string, 0, len(symbols))
	for _, s := range symbols {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		marks = append(marks, "?")
		args = append(args, s)
	}
	return strings.Join(marks, ","), args
}

/*
ScoreRangeBatch — 여러 종목의 점수를 **보관 창 전체**로 한 번에.

`RecentPairs`는 두 회차만 준다(목록의 「어제 대비」용). 진단은 「보관 창의 첫 회차에서
지금까지」를 보므로 창 전체가 필요하다. `inputs` 블롭은 여기서도 읽지 않는다 —
축 점수와 커버리지면 충분하고, 그 블롭이 행당 825B다.

**보관 창보다 오래된 날짜는 `stock_score_history`에서 잇는다** (2026-09-23). 창 안은
원본 표가, 창 밖은 얇은 이력이 답한다 — 경계는 원본 표의 첫 날짜이므로 두 표가 같은
날짜를 두 번 내지 않는다. 이력의 축 값은 소수 둘째 자리까지라 귀인(`attributeChange`)에
써도 정수 점수 재현에 영향이 없다(077 머리글).
*/
func (r *StockScoreRepo) ScoreRangeBatch(
	ctx context.Context, symbols []string, sinceDate string,
) (map[string][]ScoreLightRow, error) {
	if r == nil || r.db == nil || len(symbols) == 0 {
		return nil, nil
	}
	in, symArgs := symbolPlaceholders(symbols)
	args := append([]any{}, symArgs...)
	args = append(args, sinceDate)
	args = append(args, symArgs...)
	args = append(args, sinceDate)
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, DATE_FORMAT(trade_date, '%Y-%m-%d') AS d, score, coverage, breakdown
		  FROM stock_scores
		 WHERE symbol IN (`+in+`) AND trade_date >= ?
		UNION ALL
		SELECT symbol, DATE_FORMAT(trade_date, '%Y-%m-%d') AS d, score, coverage,
		       JSON_OBJECT('technical', technical, 'quality', quality, 'growth', growth,
		                   'valuation', valuation, 'sentiment', sentiment)
		  FROM stock_score_history
		 WHERE symbol IN (`+in+`) AND trade_date >= ?
		   AND trade_date < (SELECT MIN(trade_date) FROM stock_scores)
		 ORDER BY symbol, d DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("score range batch: %w", err)
	}
	defer rows.Close()

	out := map[string][]ScoreLightRow{}
	for rows.Next() {
		var (
			row       ScoreLightRow
			breakdown sql.NullString
		)
		if err := rows.Scan(&row.Symbol, &row.TradeDate, &row.Score, &row.Coverage, &breakdown); err != nil {
			return nil, err
		}
		if breakdown.Valid && breakdown.String != "" {
			_ = json.Unmarshal([]byte(breakdown.String), &row.Breakdown)
		}
		out[row.Symbol] = append(out[row.Symbol], row)
	}
	return out, rows.Err()
}

// EarliestScoredDate is the first date any score survives — the retention
// window of stock_scores or, before it, the thin stock_score_history.
//
// 화면이 「매수 시점과 비교할 수 없습니다」를 말할 때의 근거다. 이력 표가 생긴 뒤로
// 이 날짜는 더 이상 30거래일 전에 묶이지 않는다.
func (r *StockScoreRepo) EarliestScoredDate(ctx context.Context) (string, error) {
	if r == nil || r.db == nil {
		return "", nil
	}
	var d sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT DATE_FORMAT(LEAST(
		         COALESCE((SELECT MIN(trade_date) FROM stock_score_history), '9999-12-31'),
		         COALESCE((SELECT MIN(trade_date) FROM stock_scores), '9999-12-31')), '%Y-%m-%d')`).Scan(&d)
	if err == nil && d.String == "9999-12-31" {
		d.String = ""
	}
	if err != nil {
		return "", fmt.Errorf("earliest scored date: %w", err)
	}
	return d.String, nil
}
