package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

// StockScoreRow is one stored score. Breakdown/Inputs are pre-marshalled JSON
// so the batch can build thousands of rows without re-encoding per write.
type StockScoreRow struct {
	Symbol    string
	TradeDate string
	Score     sql.NullInt64
	ScoreKind string
	Grade     string
	Coverage  float64
	PeerGroup string
	PeerPct   sql.NullInt64
	Breakdown string
	Inputs    string

	// WeightSource is transient — which weight set produced this row. It is not
	// persisted because weights can be edited after a batch, and the API
	// reports the weights in force at read time.
	WeightSource string
}

type StockScoreRepo struct {
	db *sql.DB
}

func NewStockScoreRepo(db *sql.DB) *StockScoreRepo {
	return &StockScoreRepo{db: db}
}

// scoreUpsertChunk keeps each multi-row statement well under MySQL's packet
// limit while still cutting round trips by ~500x versus row-at-a-time.
const scoreUpsertChunk = 500

/*
ScoreRetentionDates — stock_scores가 보관하는 거래일 수.

이 표는 하루 17,906행씩 자란다(전 종목 × 1행). 무게는 `inputs` JSON(평균 825B)이라
행당 약 1.5KB, **연 7.5GB**다 — candle_bars보다 빠르다. 2026-08-23까지 정리 코드가
없었고, 8일치가 이미 211MB였다.

읽는 쪽은 전부 최신 하나만 본다 — `MAX(trade_date)` 아니면 `WHERE trade_date = ?`
(그 인자도 LatestScoredDate다). **과거 점수 시계열을 읽는 기능이 없다.**

30거래일은 관리자 화면에서 최근 추이를 들여다보고 재계산 결과를 직전과 대조할 만큼이다.
이 값이면 정상 상태가 약 540,000행 · 800MB로 고정된다.
*/
const ScoreRetentionDates = 30

/*
PruneOldScores — 최신 N거래일만 남기고 지운다.

날짜 단위로 지운다. 행 단위로 자르면 같은 날짜가 반쪽만 남아 랭킹이 종목 일부만
보여 준다.
*/
func (r *StockScoreRepo) PruneOldScores(ctx context.Context, keepDates int) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	if keepDates <= 0 {
		keepDates = ScoreRetentionDates
	}

	var cutoff sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT trade_date FROM (
			SELECT DISTINCT trade_date FROM stock_scores ORDER BY trade_date DESC LIMIT 1 OFFSET ?
		) t`, keepDates-1).Scan(&cutoff)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil // 보관 기간보다 짧게 쌓여 있다 — 지울 것이 없다.
	}
	if err != nil {
		return 0, fmt.Errorf("score prune cutoff: %w", err)
	}
	if !cutoff.Valid {
		return 0, nil
	}

	res, err := r.db.ExecContext(ctx,
		`DELETE FROM stock_scores WHERE trade_date < ?`, cutoff.String)
	if err != nil {
		return 0, fmt.Errorf("score prune: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (r *StockScoreRepo) UpsertBatch(ctx context.Context, rows []StockScoreRow) error {
	if r == nil || r.db == nil || len(rows) == 0 {
		return nil
	}
	for start := 0; start < len(rows); start += scoreUpsertChunk {
		end := start + scoreUpsertChunk
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[start:end]

		var b strings.Builder
		b.WriteString(`INSERT INTO stock_scores
			(symbol, trade_date, score, score_kind, grade, coverage, peer_group, peer_pct, breakdown, inputs)
			VALUES `)
		args := make([]any, 0, len(chunk)*10)
		for i, row := range chunk {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString("(?,?,?,?,?,?,?,?,?,?)")
			var grade any
			if row.Grade != "" {
				grade = row.Grade
			}
			args = append(args,
				row.Symbol, row.TradeDate, row.Score, row.ScoreKind, grade,
				row.Coverage, row.PeerGroup, row.PeerPct, row.Breakdown, row.Inputs,
			)
		}
		b.WriteString(` ON DUPLICATE KEY UPDATE
			score = VALUES(score),
			score_kind = VALUES(score_kind),
			grade = VALUES(grade),
			coverage = VALUES(coverage),
			peer_group = VALUES(peer_group),
			peer_pct = VALUES(peer_pct),
			breakdown = VALUES(breakdown),
			inputs = VALUES(inputs),
			computed_at = CURRENT_TIMESTAMP(3)`)

		if _, err := r.db.ExecContext(ctx, b.String(), args...); err != nil {
			return fmt.Errorf("upsert stock_scores: %w", err)
		}
	}
	return nil
}

const scoreSelectCols = `sc.symbol, s.name, DATE_FORMAT(sc.trade_date, '%Y-%m-%d'), sc.score,
	sc.score_kind, sc.grade, sc.coverage, sc.peer_group, sc.peer_pct,
	sc.breakdown, sc.inputs, sc.computed_at`

func (r *StockScoreRepo) scan(rows *sql.Rows) (*model.StockScoreView, error) {
	var (
		v                 model.StockScoreView
		name, grade, peer sql.NullString
		score, peerPct    sql.NullInt64
		breakdown, inputs sql.NullString
	)
	if err := rows.Scan(
		&v.Symbol, &name, &v.TradeDate, &score, &v.ScoreKind, &grade,
		&v.Coverage, &peer, &peerPct, &breakdown, &inputs, &v.ComputedAt,
	); err != nil {
		return nil, err
	}
	v.Name = name.String
	v.Grade = grade.String
	v.PeerGroup = peer.String
	if score.Valid {
		s := int(score.Int64)
		v.Score = &s
	}
	if peerPct.Valid {
		p := model.ReadPeerPct(int(peerPct.Int64))
		v.PeerPct = &p
	}
	if breakdown.Valid && breakdown.String != "" {
		_ = json.Unmarshal([]byte(breakdown.String), &v.Breakdown)
	}
	if inputs.Valid && inputs.String != "" {
		_ = json.Unmarshal([]byte(inputs.String), &v.Inputs)
	}
	return &v, nil
}

// Latest returns the newest stored score for one symbol.
func (r *StockScoreRepo) Latest(ctx context.Context, symbol string) (*model.StockScoreView, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+scoreSelectCols+`
		  FROM stock_scores sc
		  LEFT JOIN stocks s ON s.symbol = sc.symbol
		 WHERE sc.symbol = ?
		 ORDER BY sc.trade_date DESC
		 LIMIT 1`, symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	return r.scan(rows)
}

/*
UpdateAxes rewrites one already-stored row's composite, grade, coverage,
breakdown and inputs — everything the 심리·수급 축이 뒤늦게 바꾸는 값들.

**INSERT는 하지 않는다.** 이 축은 요약 파이프라인이 다녀간 종목에만 붙는데, 그
종목이 아직 점수 배치에 잡히지 않았다면(상장 직후·재무 결측) 여기서 행을 만들어
봐야 심리 하나만 든 점수가 된다. 그건 점수가 아니라 뉴스 감성이다 — 배치가 그
종목을 채점하는 날 정상적으로 합류한다.

peer_pct는 건드리지 않는다. 그 값은 같은 날 전 종목을 함께 줄 세워야 나오는데,
한 종목만 고쳐 놓고 순위를 다시 매길 수는 없다. 다음 배치가 바로잡는다.
*/
func (r *StockScoreRepo) UpdateAxes(
	ctx context.Context,
	symbol, tradeDate string,
	score sql.NullInt64,
	grade string,
	coverage float64,
	breakdown, inputs string,
) error {
	if r == nil || r.db == nil {
		return nil
	}
	var gradeArg any
	if grade != "" {
		gradeArg = grade
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE stock_scores
		   SET score = ?, grade = ?, coverage = ?, breakdown = ?, inputs = ?
		 WHERE symbol = ? AND trade_date = ?`,
		score, gradeArg, coverage, breakdown, inputs,
		strings.ToUpper(strings.TrimSpace(symbol)), tradeDate,
	)
	return err
}

// History returns a symbol's recent scores, newest first.
func (r *StockScoreRepo) History(ctx context.Context, symbol string, limit int) ([]model.StockScoreView, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 120 {
		limit = 60
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+scoreSelectCols+`
		  FROM stock_scores sc
		  LEFT JOIN stocks s ON s.symbol = sc.symbol
		 WHERE sc.symbol = ?
		 ORDER BY sc.trade_date DESC
		 LIMIT ?`, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.StockScoreView
	for rows.Next() {
		v, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

// LatestBatch returns the newest score for each of several symbols in one
// query. The holdings screen needs a score per held symbol, and looping Latest()
// there would mean one round trip per row — twenty holdings, twenty queries.
//
// The join picks each symbol's max trade_date rather than a single global date:
// a symbol that stopped being scored (delisted, missing fundamentals) still
// shows its last known score instead of vanishing from the list.
func (r *StockScoreRepo) LatestBatch(ctx context.Context, symbols []string) ([]model.StockScoreView, error) {
	if r == nil || r.db == nil || len(symbols) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(symbols))
	placeholders := make([]string, 0, len(symbols))
	seen := map[string]struct{}{}
	for _, sym := range symbols {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			continue
		}
		if _, dup := seen[sym]; dup {
			continue
		}
		seen[sym] = struct{}{}
		placeholders = append(placeholders, "?")
		args = append(args, sym)
	}
	if len(args) == 0 {
		return nil, nil
	}
	in := strings.Join(placeholders, ",")
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+scoreSelectCols+`
		  FROM stock_scores sc
		  JOIN (
		        SELECT symbol, MAX(trade_date) AS trade_date
		          FROM stock_scores
		         WHERE symbol IN (`+in+`)
		         GROUP BY symbol
		       ) latest
		    ON latest.symbol = sc.symbol AND latest.trade_date = sc.trade_date
		  LEFT JOIN stocks s ON s.symbol = sc.symbol`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.StockScoreView, 0, len(args))
	for rows.Next() {
		v, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

// SyncDailyCloseScores copies the batch's scores onto that date's 일별 종가
// rows, so the existing 일별 시세 list shows a score without a second lookup.
//
// One-way and idempotent, matching the projection rule the close pipeline
// already follows: stock_scores is the source, stock_daily_closes only mirrors.
func (r *StockScoreRepo) SyncDailyCloseScores(ctx context.Context, tradeDate string) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE stock_daily_closes d
		  JOIN stock_scores sc
		    ON sc.symbol = d.symbol AND sc.trade_date = d.trade_date
		   SET d.score = sc.score
		 WHERE d.trade_date = ?`, tradeDate)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

/*
scoreCloseFallbackDays — 등락률을 메울 때 거슬러 올라갈 수 있는 최대 일수.

7일인 이유는 **정상적으로 벌어질 수 있는 간격**을 덮되 그 이상은 덮지 않기 위해서다.
금요일 점수에 붙는 해외 종가는 목요일 것이 정상이고(미국 세션이 하루 늦게 적재된다),
연휴가 끼면 나흘까지 벌어진다. 일주일을 넘겼다면 그건 적재 지연이 아니라 **그 종목의
시세가 끊긴 것**이고, 그때 옛 숫자를 오늘 자리에 놓으면 안 된다.

달력 일수로 센다 — 거래일로 세려면 종목마다 다른 시장 달력을 알아야 하는데, 이 상한이
하는 일(옛 값 걸러내기)에는 그 정밀도가 필요 없다.
*/
const scoreCloseFallbackDays = 7

// TopScores returns the highest-scored symbols for one trade date, restricted
// to the given asset types (domestic vs overseas), for the 종목 랭킹 list.
func (r *StockScoreRepo) TopScores(ctx context.Context, tradeDate string, assetTypes []string, limit int) ([]model.StockScoreListItem, error) {
	if r == nil || r.db == nil || tradeDate == "" || len(assetTypes) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	placeholders := strings.Repeat("?,", len(assetTypes))
	placeholders = placeholders[:len(placeholders)-1]

	// STRAIGHT_JOIN으로 stock_scores를 구동 테이블로 못박는다.
	//
	// 옵티마이저는 자유롭게 두면 stocks를 먼저 잡는다 — asset_type 조건이 거기 있고
	// 카디널리티가 낮아 보이기 때문이다. 그러면 17,070행을 전부 읽어 매 행마다
	// stock_scores를 찔러보고, 모은 결과를 filesort 한 뒤에야 스무 줄을 자른다
	// (실측: 해외 탭 2.10초).
	//
	// 반대로 sc부터 읽으면 idx_stock_scores_rank(trade_date, score DESC, peer_pct DESC)가
	// ORDER BY와 순서가 같아 정렬이 공짜가 되고, LIMIT을 채우는 순간 스캔이 끝난다.
	// 해외는 전체의 75%라 스무 줄을 채우는 데 서른 행 남짓만 읽는다 (실측 0.036초).
	/*
		종가는 **날짜가 정확히 맞는 행이 아니라 가장 최근 행**을 붙인다.

		예전에는 `d.trade_date = sc.trade_date`로 못박았다. 그런데 종가 적재는 종목마다
		실패한다 — 실측 08-29 해외분 「시도 13,503 · 채움 10,239 · 실패 3,253」. 점수는
		전 종목에 매겨지므로, 그날 종가를 못 받은 종목은 **등락률 칸이 통째로 비었다**
		(실측: 국내 랭킹 TOP 10 중 일곱, 전체로는 17,915행 중 3,845행 = 21%).

		LATERAL을 쓰는 이유는 상관 서브쿼리를 **출력되는 행에만** 돌리기 위해서다. PK가
		(symbol, trade_date)라 한 행당 인덱스 역방향 1건이면 끝나고, LIMIT을 채운 뒤
		그 스무 행에 대해서만 도니 위 STRAIGHT_JOIN이 벌어 둔 이득을 까먹지 않는다.

		날짜를 함께 꺼내는 것이 핵심이다 — 메운 값이 어느 날 것인지 화면이 말할 수
		있어야 한다. 값만 채우고 날짜를 숨기면 8/27 등락률이 8/28 라벨 아래에 선다.

		**얼마나 거슬러 올라갈지에는 상한이 있다**(`scoreCloseFallbackDays`). 상한 없이
		「가장 최근」을 집으면 거래가 끊긴 종목의 몇 주 전 값이 올라온다 — 실측 08-31의
		해외 랭킹 스무 줄 중 다섯(ISSC·ALVOW·TALK·ALIS·EMIS)이 **08-18 종가로 0.00%**를
		달고 있었다. 열흘 전 값의 0%는 "보합"으로 읽히지만 실제로는 "최근 시세가 없다"이고,
		그건 빈칸보다 나쁘다.

		그리고 상한을 넘은 종목은 **목록에서 뺀다**(LEFT가 아니라 INNER LATERAL).
		예전에는 값만 비워 내보냈는데, 그러면 「최근 일주일 시세가 없는 종목」이 AI 추천
		랭킹 4위에 92점 A+로 앉아 등락률 칸만 비어 있었다(실측 08-31 해외 4위 ALDF —
		마지막 종가 08-18). 추천 목록에서 그 자리는 거래되는 종목이 가져가야 한다.
		실측 규모: 최신 점수일 기준으로 국내 123 · ETF 1,300 · 해외 2,000종목이 여기
		걸린다(대부분 SPAC 유닛·워런트·신규 상장 ETF).

		**점수일을 상한으로 두지 않는다** (2026-09-04 사용자 지적). 예전에는
		`c.trade_date <= sc.trade_date`가 붙어 있어, 종가가 점수보다 앞서 나가는
		구간에서 목록이 하루 묵은 값을 들었다. 점수 배치는 KST 00:25에 **전날치**를
		매기므로, 국내 장이 닫히는 15:30부터 다음 배치까지 매일 아홉 시간 동안 그
		상태다 — 실측 09-04 00:50, 089790 제이티: 목록 5,200원 +6.78%(09-02 종가),
		같은 순간 종목 상세 5,100원(09-03 종가). 같은 앱의 두 화면이 다른 가격을
		말하고 있었다.

		하한(`scoreCloseFallbackDays`)은 그대로 남는다 — 「거래가 끊긴 종목을
		목록에서 뺀다」는 위의 규칙은 점수일 기준 최근 일주일에 종가가 하나라도
		있는지를 묻는 것이고, 상한과 무관하다.

		**상품·축 조건도 여기 붙는다** (2026-09-04). 「종합·실시간」은 이 함수가
		그 자리에서 계산하고 나머지 47개 조합은 사전계산 표를 읽는데, 조건을 한쪽에만
		적으면 **같은 화면에서 탭만 옮겨도 종목 구성 규칙이 달라진다.** 정의는
		`stock_score_universe.go` 한 곳에 있다.
	*/
	productClause, productArgs := scoreRankableProductClause("s")
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
		strconv.Itoa(scoreCloseFallbackDays) + ` DAY)
		         ORDER BY c.trade_date DESC
		         LIMIT 1
		       ) d ON TRUE
		 WHERE sc.trade_date = ? AND sc.score IS NOT NULL AND s.asset_type IN (` + placeholders + `)` +
		productClause + scoreMinAxesClause("sc") + `
		 ORDER BY sc.score DESC, sc.peer_pct DESC
		 LIMIT ?`

	args := make([]any, 0, len(assetTypes)+len(productArgs)+2)
	args = append(args, tradeDate)
	for _, a := range assetTypes {
		args = append(args, a)
	}
	args = append(args, productArgs...)
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.StockScoreListItem, 0, limit)
	for rows.Next() {
		var (
			it                     model.StockScoreListItem
			name, grade, currency  sql.NullString
			closeDate              sql.NullString
			score                  sql.NullInt64
			peerPct                sql.NullInt64
			changeRate, closePrice sql.NullFloat64
			prevClose              sql.NullFloat64
		)
		if err := rows.Scan(&it.Symbol, &name, &it.AssetType, &score, &grade, &peerPct,
			&changeRate, &closePrice, &prevClose, &currency, &closeDate); err != nil {
			return nil, err
		}
		it.Name = name.String
		it.Grade = grade.String
		it.Currency = currency.String
		it.TradeDate = tradeDate
		if score.Valid {
			it.Score = int(score.Int64)
		}
		if peerPct.Valid {
			p := model.ReadPeerPct(int(peerPct.Int64))
			it.PeerPct = &p
		}
		if changeRate.Valid {
			it.ChangeRate = &changeRate.Float64
		}
		if closePrice.Valid {
			it.ClosePrice = &closePrice.Float64
		}
		if prevClose.Valid {
			it.PrevClose = &prevClose.Float64
		}
		// 출처를 언제나 밝힌다. 점수의 날과 같을 때만 비우던 예전 규칙은 해외
		// 종목에서 늘 어긋났다 — KST 날짜로 매긴 점수 옆에 붙는 미국 종가는 하루
		// 전 것이 정상이라, 그 규칙 아래에서는 해외 랭킹 전체가 「밀린 값」이었다.
		if closeDate.Valid {
			it.PriceAsOf = closeDate.String
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Rank = i + 1
	}
	return out, nil
}

// Stats reports what a given date's batch produced, for the admin page.
func (r *StockScoreRepo) Stats(ctx context.Context, tradeDate string) (total, scored int, err error) {
	if r == nil || r.db == nil {
		return 0, 0, nil
	}
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(score IS NOT NULL), 0)
		  FROM stock_scores WHERE trade_date = ?`, tradeDate).Scan(&total, &scored)
	return total, scored, err
}

// ---- settings -------------------------------------------------------------

type ScoreSettingsRepo struct {
	db *sql.DB
}

func NewScoreSettingsRepo(db *sql.DB) *ScoreSettingsRepo {
	return &ScoreSettingsRepo{db: db}
}

func (r *ScoreSettingsRepo) Get(ctx context.Context) (model.ScoreSettings, error) {
	if r == nil || r.db == nil {
		return model.DefaultScoreSettings(), nil
	}
	var s model.ScoreSettings
	err := r.db.QueryRowContext(ctx, `
		SELECT w_technical, w_quality, w_growth, w_valuation, w_sentiment,
		       coverage_floor, shrink_full, updated_at
		  FROM score_settings WHERE id = 1`).Scan(
		&s.Weights.Technical, &s.Weights.Quality, &s.Weights.Growth,
		&s.Weights.Valuation, &s.Weights.Sentiment,
		&s.CoverageFloor, &s.ShrinkFull, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.DefaultScoreSettings(), nil
		}
		return model.ScoreSettings{}, err
	}
	s.Weights = s.Weights.Clamped()
	return s, nil
}

func (r *ScoreSettingsRepo) Update(ctx context.Context, s model.ScoreSettings) (model.ScoreSettings, error) {
	if r == nil || r.db == nil {
		return model.DefaultScoreSettings(), nil
	}
	w := s.Weights.Clamped()
	floor := clampRatio(s.CoverageFloor, 0, 0.9, 0.20)
	full := clampRatio(s.ShrinkFull, 0.1, 1.0, 0.60)
	_, err := r.db.ExecContext(ctx, `
		UPDATE score_settings
		   SET w_technical = ?, w_quality = ?, w_growth = ?, w_valuation = ?, w_sentiment = ?,
		       coverage_floor = ?, shrink_full = ?
		 WHERE id = 1`,
		w.Technical, w.Quality, w.Growth, w.Valuation, w.Sentiment, floor, full)
	if err != nil {
		return model.ScoreSettings{}, err
	}
	return r.Get(ctx)
}

func clampRatio(v, lo, hi, fallback float64) float64 {
	if v <= 0 {
		return fallback
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ListSymbolWeights returns every per-symbol override, joined to the stock name
// so the admin table can show 종목명 rather than bare codes.
func (r *ScoreSettingsRepo) ListSymbolWeights(ctx context.Context) ([]model.SymbolScoreWeights, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT w.symbol, COALESCE(s.name, ''), w.w_technical, w.w_quality, w.w_growth,
		       w.w_valuation, w.w_sentiment, w.note, w.updated_at
		  FROM score_symbol_weights w
		  LEFT JOIN stocks s ON s.symbol = w.symbol
		 ORDER BY w.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.SymbolScoreWeights, 0)
	for rows.Next() {
		var it model.SymbolScoreWeights
		if err := rows.Scan(&it.Symbol, &it.Name,
			&it.Weights.Technical, &it.Weights.Quality, &it.Weights.Growth,
			&it.Weights.Valuation, &it.Weights.Sentiment, &it.Note, &it.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *ScoreSettingsRepo) UpsertSymbolWeights(ctx context.Context, it model.SymbolScoreWeights) error {
	if r == nil || r.db == nil {
		return nil
	}
	symbol := strings.ToUpper(strings.TrimSpace(it.Symbol))
	if symbol == "" {
		return fmt.Errorf("symbol required")
	}
	w := it.Weights.Clamped()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO score_symbol_weights
			(symbol, w_technical, w_quality, w_growth, w_valuation, w_sentiment, note)
		VALUES (?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE
			w_technical = VALUES(w_technical),
			w_quality   = VALUES(w_quality),
			w_growth    = VALUES(w_growth),
			w_valuation = VALUES(w_valuation),
			w_sentiment = VALUES(w_sentiment),
			note        = VALUES(note)`,
		symbol, w.Technical, w.Quality, w.Growth, w.Valuation, w.Sentiment, it.Note)
	return err
}

func (r *ScoreSettingsRepo) DeleteSymbolWeights(ctx context.Context, symbol string) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM score_symbol_weights WHERE symbol = ?`,
		strings.ToUpper(strings.TrimSpace(symbol)))
	return err
}

// MigrateStockScoresIfNeeded applies 030_stock_scores.sql.
func MigrateStockScoresIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "stock_scores")
	if err != nil {
		return err
	}
	settingsExists, err := tableExists(ctx, db, "score_settings")
	if err != nil {
		return err
	}
	if exists && settingsExists {
		return nil
	}
	path := filepath.Join(migrationsDir, "030_stock_scores.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 030_stock_scores.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("030_stock_scores.sql: %w", err)
		}
	}
	return nil
}

// indexClosesHasColumn reports whether index_daily_closes already carries a column.
func indexClosesHasColumn(ctx context.Context, db *sql.DB, column string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.COLUMNS
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'index_daily_closes'
		   AND COLUMN_NAME = ?`, column).Scan(&count)
	return count > 0, err
}

// applyIndexClosesMigration runs one migration file, tolerating a column that a
// concurrent boot already added.
func applyIndexClosesMigration(ctx context.Context, db *sql.DB, migrationsDir, file string) error {
	body, err := os.ReadFile(filepath.Join(migrationsDir, file))
	if err != nil {
		return fmt.Errorf("read %s: %w", file, err)
	}
	if _, err := db.ExecContext(ctx, string(body)); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return fmt.Errorf("%s: %w", file, err)
		}
	}
	return nil
}

// MigrateIndexScoresIfNeeded applies 031_index_scores.sql, 062_index_tags.sql
// and 063_index_llm_note.sql.
//
// Both belong to the same writer — the index score batch — so they share one
// entry point rather than each needing its own hook in migrate.go's two call
// sites, which is exactly the kind of thing that gets wired in one place and
// forgotten in the other.
func MigrateIndexScoresIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	hasScore, err := indexClosesHasColumn(ctx, db, "score")
	if err != nil {
		return err
	}
	if !hasScore {
		if err := applyIndexClosesMigration(ctx, db, migrationsDir, "031_index_scores.sql"); err != nil {
			return err
		}
	} else {
		// Columns already exist — but an earlier revision of this migration created
		// ai_summary without a default and left source at VARCHAR(16), which made
		// every index close write fail. MODIFY is idempotent, so repair in place
		// rather than requiring a manual fix.
		repairs := []string{
			`ALTER TABLE index_daily_closes MODIFY COLUMN ai_summary TEXT NOT NULL DEFAULT ('')`,
			`ALTER TABLE index_daily_closes MODIFY COLUMN source VARCHAR(32) NOT NULL DEFAULT ''`,
		}
		for _, stmt := range repairs {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("031 repair: %w", err)
			}
		}
	}

	hasTags, err := indexClosesHasColumn(ctx, db, "tags")
	if err != nil {
		return err
	}
	if !hasTags {
		if err := applyIndexClosesMigration(ctx, db, migrationsDir, "062_index_tags.sql"); err != nil {
			return err
		}
	}

	hasNote, err := indexClosesHasColumn(ctx, db, "llm_note")
	if err != nil {
		return err
	}
	if !hasNote {
		if err := applyIndexClosesMigration(ctx, db, migrationsDir, "063_index_llm_note.sql"); err != nil {
			return err
		}
	}
	return nil
}

/*
PeerGroupsBefore — `tradeDate` 직전 배치가 각 종목에 매겼던 동종 그룹.

점수 배치가 그룹을 **이어 붙이기** 위해 읽는다. 그룹은 (자산군:시장:시총 3분위)로
정해지는데, 시총은 매일 주가만큼 움직이고 분위 경계선도 그날 분포에서 다시 잡힌다.
그래서 경계 근처 종목은 하루걸러 cap2↔cap3를 오갔고, 시총을 못 받은 날에는 분위가
아예 빠졌다 — 실측 08-27→08-28에 **17,913종목 중 9,936종목(55%)이 소속을 바꿨다**.

동종 그룹이 바뀌면 백분위가 바뀌고, 백분위가 바뀌면 재무제표가 그대로여도 수익성·
성장 점수가 움직인다. 하루 만에 종합 점수가 평균 6.2점, 22%는 10점 넘게 뛰던 이유의
큰 몫이 여기 있었다.

없으면 빈 맵이다 — 첫 실행이나 보관 기간 밖이면 그냥 오늘 값으로 정하면 된다.
*/
func (r *StockScoreRepo) PeerGroupsBefore(ctx context.Context, tradeDate string) (map[string]string, error) {
	if r == nil || r.db == nil || tradeDate == "" {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT sc.symbol, sc.peer_group
		  FROM stock_scores sc
		 WHERE sc.trade_date = (
		       SELECT MAX(trade_date) FROM stock_scores WHERE trade_date < ?
		   )
		   AND sc.peer_group <> ''`, tradeDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var symbol, peer string
		if err := rows.Scan(&symbol, &peer); err != nil {
			return nil, err
		}
		out[symbol] = peer
	}
	return out, rows.Err()
}
