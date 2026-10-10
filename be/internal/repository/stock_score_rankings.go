package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

/*
AI 추천 랭킹의 사전 계산 결과를 읽고 쓴다 (migrations/064).

표는 48행으로 고정이다 — 시장 2 × 축 6 × 기간 4. 배치가 같은 행을 덮어쓰고, 조회는
그 한 행의 JSON을 그대로 돌려준다. 왜 계산을 요청 경로에서 걷어냈는지는 마이그레이션
머리글에 실측과 함께 적어 두었다.
*/
type StockScoreRankingRepo struct {
	db *sql.DB
}

func NewStockScoreRankingRepo(db *sql.DB) *StockScoreRankingRepo {
	return &StockScoreRankingRepo{db: db}
}

// ScoreWindowRow is one symbol's best score in a window: the composite and each
// axis, already reduced to a single number per measure.
type ScoreWindowRow struct {
	Symbol    string
	Composite sql.NullFloat64
	Axes      map[model.ScoreAxis]sql.NullFloat64
}

/*
WindowBests — 한 시장·한 창에서 **종목별 최고점을 여섯 측정치 모두** 한 번에 구한다.

여섯 축을 따로 물으면 같은 행을 여섯 번 훑는다. 실측(개발 PC, 국내 30일 창):
축 하나씩 여섯 번이면 합계 10초가 넘고, 이 한 방은 2.08초다(해외 1.30초).
정렬과 상위 잘라내기는 Go가 한다 — 18,000행을 메모리에서 여섯 번 정렬하는 비용은
밀리초 단위이고, SQL에 맡기면 `ORDER BY`마다 filesort가 한 번씩 더 돈다.

`STRAIGHT_JOIN`을 쓰는 이유는 `TopScores`와 같다 — stocks를 먼저 잡으면 17,000행을
전부 읽고 매 행마다 점수를 찔러 본다.
*/
func (r *StockScoreRankingRepo) WindowBests(
	ctx context.Context,
	fromDate, toDate string,
	assetTypes []string,
) ([]ScoreWindowRow, error) {
	if r == nil || r.db == nil || fromDate == "" || toDate == "" || len(assetTypes) == 0 {
		return nil, nil
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(assetTypes)), ",")
	// 지수 추종 상품은 창을 훑기 전에 뺀다. `RankableSymbols`가 어차피 한 번 더
	// 거르지만, 여기서 빼면 집계가 읽는 행 수부터 줄어든다 — 실측 분포로 국내
	// ETF·ETN 1,532 · 해외 ETP 6,726종목이 이 조건에 걸린다.
	productClause, productArgs := scoreRankableProductClause("s")
	query := `
		SELECT sc.symbol,
		       MAX(sc.score),
		       MAX(CAST(sc.breakdown->'$.technical' AS DECIMAL(6,3))),
		       MAX(CAST(sc.breakdown->'$.quality'   AS DECIMAL(6,3))),
		       MAX(CAST(sc.breakdown->'$.growth'    AS DECIMAL(6,3))),
		       MAX(CAST(sc.breakdown->'$.valuation' AS DECIMAL(6,3))),
		       MAX(CAST(sc.breakdown->'$.sentiment' AS DECIMAL(6,3)))
		  FROM stock_scores sc
		  STRAIGHT_JOIN stocks s ON s.symbol = sc.symbol
		 WHERE sc.trade_date BETWEEN ? AND ?
		   AND s.asset_type IN (` + placeholders + `)` + productClause + `
		 GROUP BY sc.symbol`

	args := make([]any, 0, len(assetTypes)+len(productArgs)+2)
	args = append(args, fromDate, toDate)
	for _, t := range assetTypes {
		args = append(args, t)
	}
	args = append(args, productArgs...)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("window bests: %w", err)
	}
	defer rows.Close()

	var out []ScoreWindowRow
	for rows.Next() {
		var row ScoreWindowRow
		var technical, quality, growth, valuation, sentiment sql.NullFloat64
		if err := rows.Scan(
			&row.Symbol, &row.Composite,
			&technical, &quality, &growth, &valuation, &sentiment,
		); err != nil {
			return nil, fmt.Errorf("window bests scan: %w", err)
		}
		row.Axes = map[model.ScoreAxis]sql.NullFloat64{
			model.AxisTechnical: technical,
			model.AxisQuality:   quality,
			model.AxisGrowth:    growth,
			model.AxisValuation: valuation,
			model.AxisSentiment: sentiment,
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

/*
DisplayRows — 정해진 종목들의 **표시용 한 줄**을 최신 점수일 기준으로 가져온다.

`TopScores`가 쓰는 것과 같은 조인이다(종가 LATERAL 포함 — 그 이유는 그쪽 주석에
길게 적혀 있다). 다른 점은 정렬·상한이 없고 **심볼 목록으로 좁힌다**는 것뿐이다:
순위는 이미 Go가 정했고, 여기서는 그 종목들의 이름·등급·종가만 채우면 된다.

48개 목록의 합집합이라 최대 4,800종목이지만 실제로는 겹치는 종목이 많아 훨씬 적다.

# 종가 LATERAL이 INNER인 이유 (2026-09-04 사용자 지적 3)

**여기가 LEFT였다.** 그래서 최근 일주일에 종가가 하나도 없는 종목이 가격 칸을 비운
채로 랭킹에 올랐다 — 실측 09-04 해외 「수익성·1개월」의 2·3·6위가 `ADSEW`·`IRAB`·
`IMSRW`였고, 셋 다 `stock_daily_closes`에 **행이 아예 없다**. Redis 시세도 없어서
`overlayLiveChanges`도 못 메운다(그쪽은 기준가가 없으면 손을 뗀다). 그 결과가
화면의 `—  —` 두 칸이다.

`TopScores`는 같은 함정을 이미 밟고 **INNER로 고쳐 두었다**(08-31 해외 4위 ALDF,
그쪽 LATERAL 주석의 실측). 사전계산 랭킹이 나중에 생기면서 그 결론만 빠졌다.
같은 잣대로 되돌린다 — 자리가 비면 `buildRankingItems`가 다음 종목을 당겨 온다.

# 상품·축 조건

`RankableSymbols`와 **같은 조건**을 여기에도 건다. 순위 목록은 이미 그 집합으로
걸러져 오지만, 이 함수는 표시용 행의 마지막 관문이라 조건이 한쪽에만 있으면
다음에 호출부가 늘었을 때 조용히 새어 나간다.
*/
func (r *StockScoreRankingRepo) DisplayRows(
	ctx context.Context,
	tradeDate string,
	symbols []string,
) (map[string]model.StockScoreListItem, error) {
	if r == nil || r.db == nil || tradeDate == "" || len(symbols) == 0 {
		return map[string]model.StockScoreListItem{}, nil
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(symbols)), ",")
	productClause, productArgs := scoreRankableProductClause("s")
	sizeClause, sizeArgs := scoreRecommendableClause("sc", "s")
	query := `
		SELECT sc.symbol, s.name, s.asset_type, sc.score, sc.grade, sc.peer_pct,
		       d.change_rate, d.close_price, d.prev_close, d.currency,
		       DATE_FORMAT(d.trade_date, '%Y-%m-%d')
		  FROM stock_scores sc
		  STRAIGHT_JOIN stocks s ON s.symbol = sc.symbol
		  JOIN LATERAL (
		        SELECT c.change_rate, c.close_price, c.prev_close, c.currency, c.trade_date
		          FROM stock_daily_closes c
		         WHERE c.symbol = sc.symbol
		           AND c.trade_date >= DATE_SUB(sc.trade_date, INTERVAL ` +
		fmt.Sprint(scoreCloseFallbackDays) + ` DAY)
		         ORDER BY c.trade_date DESC
		         LIMIT 1
		       ) d ON TRUE
		 WHERE sc.trade_date = ? AND sc.symbol IN (` + placeholders + `)` +
		productClause + scoreMinAxesClause("sc") + sizeClause

	args := make([]any, 0, len(symbols)+len(productArgs)+len(sizeArgs)+1)
	args = append(args, tradeDate)
	for _, s := range symbols {
		args = append(args, s)
	}
	args = append(args, productArgs...)
	args = append(args, sizeArgs...)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("display rows: %w", err)
	}
	defer rows.Close()

	out := make(map[string]model.StockScoreListItem, len(symbols))
	for rows.Next() {
		var (
			item       model.StockScoreListItem
			score      sql.NullInt64
			grade      sql.NullString
			peerPct    sql.NullInt64
			changeRate sql.NullFloat64
			closePrice sql.NullFloat64
			prevClose  sql.NullFloat64
			currency   sql.NullString
			closeDate  sql.NullString
		)
		if err := rows.Scan(
			&item.Symbol, &item.Name, &item.AssetType, &score, &grade, &peerPct,
			&changeRate, &closePrice, &prevClose, &currency, &closeDate,
		); err != nil {
			return nil, fmt.Errorf("display rows scan: %w", err)
		}
		item.TradeDate = tradeDate
		if score.Valid {
			item.Score = int(score.Int64)
		}
		if grade.Valid {
			item.Grade = grade.String
		}
		if peerPct.Valid {
			v := model.ReadPeerPct(int(peerPct.Int64))
			item.PeerPct = &v
		}
		if changeRate.Valid {
			v := changeRate.Float64
			item.ChangeRate = &v
		}
		if closePrice.Valid {
			v := closePrice.Float64
			item.ClosePrice = &v
		}
		if prevClose.Valid {
			v := prevClose.Float64
			item.PrevClose = &v
		}
		if currency.Valid {
			item.Currency = currency.String
		}
		if closeDate.Valid {
			item.PriceAsOf = closeDate.String
		}
		out[item.Symbol] = item
	}
	return out, rows.Err()
}

// Save replaces one (market, axis, duration) leaderboard.
func (r *StockScoreRankingRepo) Save(ctx context.Context, view model.ScoreRankingView) error {
	if r == nil || r.db == nil {
		return nil
	}
	payload, err := json.Marshal(view.Items)
	if err != nil {
		return fmt.Errorf("marshal ranking payload: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO stock_score_rankings (market, axis, duration, trade_date, from_date, payload)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  trade_date = VALUES(trade_date),
		  from_date  = VALUES(from_date),
		  payload    = VALUES(payload)`,
		view.Market, string(view.Axis), string(view.Duration),
		view.TradeDate, view.FromDate, payload,
	)
	if err != nil {
		return fmt.Errorf("save ranking %s/%s/%s: %w", view.Market, view.Axis, view.Duration, err)
	}
	return nil
}

// Get reads one stored leaderboard. Returns nil (no error) when the batch has
// not produced it yet — a fresh install, or a new axis added since the last run.
func (r *StockScoreRankingRepo) Get(
	ctx context.Context,
	market string,
	axis model.ScoreRankingAxis,
	duration model.ScoreRankingDuration,
) (*model.ScoreRankingView, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	var (
		view      model.ScoreRankingView
		tradeDate string
		fromDate  string
		payload   []byte
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT DATE_FORMAT(trade_date, '%Y-%m-%d'), DATE_FORMAT(from_date, '%Y-%m-%d'),
		       payload, computed_at
		  FROM stock_score_rankings
		 WHERE market = ? AND axis = ? AND duration = ?`,
		market, string(axis), string(duration),
	).Scan(&tradeDate, &fromDate, &payload, &view.ComputedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ranking: %w", err)
	}

	view.Market = market
	view.Axis = axis
	view.Duration = duration
	view.TradeDate = tradeDate
	view.FromDate = fromDate
	if err := json.Unmarshal(payload, &view.Items); err != nil {
		// 저장된 JSON이 깨졌다면 목록을 못 그리는 것이지 요청이 실패할 일은 아니다 —
		// 다음 배치가 덮어쓴다.
		log.Printf("score ranking %s/%s/%s: bad payload: %v", market, axis, duration, err)
		view.Items = nil
	}
	if view.Items == nil {
		view.Items = []model.StockScoreListItem{}
	}
	return &view, nil
}

// RankedSymbol is one symbol's position inside one stored leaderboard.
type RankedSymbol struct {
	Axis     model.ScoreRankingAxis
	Duration model.ScoreRankingDuration
	Symbol   string
	Rank     int
}

/*
ListSymbolRanks — 한 시장의 **저장된 랭킹 전부**를 심볼·순위만 남겨 돌려준다.

AI 요약 배치의 대상 선정이 쓰는 입구다. `Get`을 축·기간 조합마다 부르면 한 번의
선정에 24번 질의하게 되는데, 이 표는 시장당 24행이라 한 방으로 읽는 편이 DB에도
호출부에도 낫다(운영 DB가 1 ECPU다).

화면용 필드(가격·등락률·이름)는 여기서 버린다. 선정에 필요한 것은 「어느 목록의
몇 위인가」뿐이고, 나머지는 24개 목록 × 100행어치 문자열을 그대로 들고 다니는
비용만 된다.
*/
func (r *StockScoreRankingRepo) ListSymbolRanks(ctx context.Context, market string) ([]RankedSymbol, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT axis, duration, payload FROM stock_score_rankings WHERE market = ?`, market)
	if err != nil {
		return nil, fmt.Errorf("list score rankings: %w", err)
	}
	defer rows.Close()

	var out []RankedSymbol
	for rows.Next() {
		var axis, duration string
		var payload []byte
		if err := rows.Scan(&axis, &duration, &payload); err != nil {
			return nil, fmt.Errorf("list score rankings: %w", err)
		}
		var items []model.StockScoreListItem
		if err := json.Unmarshal(payload, &items); err != nil {
			// 깨진 목록 하나 때문에 선정 전체가 실패하면 안 된다 — 다음 배치가
			// 덮어쓰고, 그때까지는 나머지 23개 목록으로 고른다.
			log.Printf("score ranking %s/%s/%s: bad payload: %v", market, axis, duration, err)
			continue
		}
		for _, item := range items {
			symbol := strings.ToUpper(strings.TrimSpace(item.Symbol))
			if symbol == "" || item.Rank <= 0 {
				continue
			}
			out = append(out, RankedSymbol{
				Axis:     model.ScoreRankingAxis(axis),
				Duration: model.ScoreRankingDuration(duration),
				Symbol:   symbol,
				Rank:     item.Rank,
			})
		}
	}
	return out, rows.Err()
}

// MigrateStockScoreRankingsIfNeeded creates the precomputed-ranking table on an
// existing database (a fresh one gets it from the full migration run).
func MigrateStockScoreRankingsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "stock_score_rankings")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir,
		"064_stock_score_rankings.sql", "AI 추천 랭킹 사전 계산 표")
}
