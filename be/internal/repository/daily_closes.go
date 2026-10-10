package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

const dailyCloseDateLayout = "2006-01-02"

// StockDailyCloseRow is one persisted row of stock_daily_closes.
type StockDailyCloseRow struct {
	Symbol       string
	TradeDate    time.Time
	Close        float64
	PrevClose    sql.NullFloat64
	ChangeAmount sql.NullFloat64
	ChangeRate   sql.NullFloat64
	Volume       int64
	Currency     string
	Score        sql.NullInt64
	AISummary    string
	/*
		그날의 「핵심 요약」 한 문단 (2026-09-15 사용자 요청 1).

		`AISummary`가 네 갈래 본문을 이어붙인 글인 것과 달리, 이쪽은 같은 호출이 그
		넷을 **전부 근거로 다시 쓴** 한 문단이다. 오늘 배치부터 쌓이므로 과거 행에서는
		빈 문자열이고, 화면은 그때 예전 방식(본문 앞 두 문장)으로 되돌아간다.
	*/
	AIOverview     string
	AnalysisSource string
	Source         string
}

type StockDailyCloseRepo struct {
	db    *sql.DB
	table *tableExistsCache
}

func NewStockDailyCloseRepo(db *sql.DB) *StockDailyCloseRepo {
	return &StockDailyCloseRepo{db: db, table: newTableExistsCache("stock_daily_closes")}
}

// UpsertBatch writes closes for one symbol. Price columns are always restated;
// score/ai_summary are NOT touched here so an analysis written later is never
// clobbered by a re-run of the price ingestion.
func (r *StockDailyCloseRepo) UpsertBatch(ctx context.Context, rows []StockDailyCloseRow) error {
	if r == nil || r.db == nil || len(rows) == 0 {
		return nil
	}

	const chunk = 400
	for start := 0; start < len(rows); start += chunk {
		end := start + chunk
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[start:end]

		var sb strings.Builder
		sb.WriteString(`INSERT INTO stock_daily_closes
			(symbol, trade_date, close_price, prev_close, change_amount, change_rate, volume, currency, ai_summary, source)
			VALUES `)
		args := make([]any, 0, len(batch)*10)
		for i, row := range batch {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?,?,?,?,?,'',?)")
			args = append(args,
				row.Symbol,
				row.TradeDate.Format(dailyCloseDateLayout),
				row.Close,
				nullFloatArg(row.PrevClose),
				nullFloatArg(row.ChangeAmount),
				nullFloatArg(row.ChangeRate),
				row.Volume,
				dailyCloseDefault(row.Currency, "KRW"),
				dailyCloseDefault(row.Source, "candle_bars"),
			)
		}
		sb.WriteString(` ON DUPLICATE KEY UPDATE
			close_price = VALUES(close_price),
			prev_close = VALUES(prev_close),
			change_amount = VALUES(change_amount),
			change_rate = VALUES(change_rate),
			volume = VALUES(volume),
			currency = VALUES(currency),
			source = VALUES(source)`)

		if _, err := r.db.ExecContext(ctx, sb.String(), args...); err != nil {
			return fmt.Errorf("stock_daily_closes upsert: %w", err)
		}
	}
	return nil
}

/*
UpsertAnalysis attaches the AI 요약 text to an existing date row. Inserting a
placeholder close of 0 would be wrong, so the analysis is only written when the
date row exists — the ingestion always runs first.

**score 컬럼은 여기서 건드리지 않는다** (2026-09-03 §6 점수 통일).

예전에는 이 UPDATE가 `score`도 함께 썼고, 그 값은 요약 파이프라인이 자체적으로
계산한 **또 다른 점수**였다(기술적 지표 + 뉴스 감성만 보는 절대 기준 0~100).
그런데 같은 컬럼을 점수 배치의 `SyncDailyCloseScores`도 쓴다 — 그쪽은 동종 그룹
백분위로 매긴 5축 종합 점수다. 두 파이프라인이 한 칸을 번갈아 덮어쓰니, AI 분석
탭 위쪽의 「오늘의 종합 점수」와 아래 날짜별 목록의 같은 날 점수가 **마지막에
쓴 쪽에 따라 달라졌다.**

이제 이 컬럼의 주인은 `stock_scores` 하나다. 요약은 글만 쓴다.
*/
func (r *StockDailyCloseRepo) UpsertAnalysis(ctx context.Context, a model.DailyCloseAnalysis) (bool, error) {
	if r == nil || r.db == nil {
		return false, nil
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE stock_daily_closes
		   SET ai_summary = ?, ai_overview = ?, analysis_source = ?, analyzed_at = CURRENT_TIMESTAMP(3)
		 WHERE symbol = ? AND trade_date = ?`,
		a.AISummary, a.AIOverview, a.Source, strings.ToUpper(strings.TrimSpace(a.Symbol)), a.TradeDate,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// List returns the most recent `limit` rows for a symbol, newest first.
func (r *StockDailyCloseRepo) List(ctx context.Context, symbol string, limit int) ([]StockDailyCloseRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 60
	}
	/*
		점수는 **`stock_scores`에서 읽는다** (2026-09-03 §6 점수 통일).

		`stock_daily_closes.score`는 배치가 끝나고 `SyncDailyCloseScores`가 복사해
		두는 사본일 뿐이다. 사본을 읽으면 두 가지가 어긋난다: ① 복사가 실패한 날
		(비치명적으로 로그만 남긴다)에는 옛 값이 남고, ② 이 컬럼을 예전에 요약
		파이프라인이 **다른 잣대의 점수**로 덮어써 둔 과거 행들이 그대로 보인다.

		원본을 조인하면 「오늘의 종합 점수」 카드와 이 목록의 같은 날 값이 정의상
		같아진다. 사본은 조인이 비었을 때만 쓴다 — 아직 안 지운 과거 데이터라도
		빈칸보다는 낫고, 배치가 다시 돌면 원본이 이긴다.
	*/
	rows, err := r.db.QueryContext(ctx, `
		SELECT d.symbol, d.trade_date, d.close_price, d.prev_close, d.change_amount, d.change_rate,
		       d.volume, d.currency, COALESCE(sc.score, d.score), d.ai_summary, d.ai_overview,
		       d.analysis_source, d.source
		  FROM stock_daily_closes d
		  LEFT JOIN stock_scores sc
		         ON sc.symbol = d.symbol AND sc.trade_date = d.trade_date
		 WHERE d.symbol = ?
		 ORDER BY d.trade_date DESC
		 LIMIT ?`, strings.ToUpper(strings.TrimSpace(symbol)), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StockDailyCloseRow
	for rows.Next() {
		var row StockDailyCloseRow
		if err := rows.Scan(
			&row.Symbol, &row.TradeDate, &row.Close, &row.PrevClose, &row.ChangeAmount,
			&row.ChangeRate, &row.Volume, &row.Currency, &row.Score, &row.AISummary,
			&row.AIOverview, &row.AnalysisSource, &row.Source,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// Stats reports coverage for operational reporting after a backfill.
func (r *StockDailyCloseRepo) Stats(ctx context.Context, since string) (rowCount, symbolCount int, minDate, maxDate string, err error) {
	if r == nil || r.db == nil {
		return 0, 0, "", "", nil
	}
	var mn, mx sql.NullString
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COUNT(DISTINCT symbol), MIN(trade_date), MAX(trade_date)
		  FROM stock_daily_closes WHERE trade_date >= ?`, since,
	).Scan(&rowCount, &symbolCount, &mn, &mx)
	if err != nil {
		return 0, 0, "", "", err
	}
	return rowCount, symbolCount, mn.String, mx.String, nil
}

// IndexDailyCloseRow is one persisted row of index_daily_closes.
type IndexDailyCloseRow struct {
	IndexID      string
	TradeDate    time.Time
	Close        float64
	PrevClose    sql.NullFloat64
	ChangeAmount sql.NullFloat64
	ChangeRate   sql.NullFloat64
	Currency     string
	Source       string
	// Score/AISummary are written by the index score batch, not by the close
	// upsert — the same one-way split the stock side uses.
	Score     sql.NullInt64
	AISummary string
	/*
		LLMNote/LLMStance는 **그날 행에 붙은 해설**이다.

		종목의 `stock_daily_closes.ai_summary`가 날짜마다 남아 그대로 이력이 되는 것과
		같은 구조다 — 어제 뭐라고 했는지 되짚을 수 있어야 오늘의 말이 믿을 만한지
		판단할 수 있다. 화면은 일별 표의 줄을 펼쳐 그날 것을 본다.
	*/
	LLMNote   string
	LLMStance string
}

type IndexDailyCloseRepo struct {
	db    *sql.DB
	table *tableExistsCache
}

func NewIndexDailyCloseRepo(db *sql.DB) *IndexDailyCloseRepo {
	return &IndexDailyCloseRepo{db: db, table: newTableExistsCache("index_daily_closes")}
}

func (r *IndexDailyCloseRepo) HasTable(ctx context.Context) bool {
	if r == nil || r.db == nil {
		return false
	}
	return r.table.Has(ctx, r.db)
}

func (r *IndexDailyCloseRepo) UpsertBatch(ctx context.Context, rows []IndexDailyCloseRow) error {
	if r == nil || r.db == nil || len(rows) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString(`INSERT INTO index_daily_closes
		(index_id, trade_date, close_value, prev_close, change_amount, change_rate, currency, source)
		VALUES `)
	args := make([]any, 0, len(rows)*8)
	for i, row := range rows {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("(?,?,?,?,?,?,?,?)")
		args = append(args,
			row.IndexID,
			row.TradeDate.Format(dailyCloseDateLayout),
			row.Close,
			nullFloatArg(row.PrevClose),
			nullFloatArg(row.ChangeAmount),
			nullFloatArg(row.ChangeRate),
			dailyCloseDefault(row.Currency, "KRW"),
			row.Source,
		)
	}
	sb.WriteString(` ON DUPLICATE KEY UPDATE
		close_value = VALUES(close_value),
		prev_close = VALUES(prev_close),
		change_amount = VALUES(change_amount),
		change_rate = VALUES(change_rate),
		currency = VALUES(currency),
		source = VALUES(source)`)

	if _, err := r.db.ExecContext(ctx, sb.String(), args...); err != nil {
		return fmt.Errorf("index_daily_closes upsert: %w", err)
	}
	return nil
}

func (r *IndexDailyCloseRepo) List(ctx context.Context, indexID string, limit int) ([]IndexDailyCloseRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 60
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT index_id, trade_date, close_value, prev_close, change_amount, change_rate,
		       currency, source, score, ai_summary, llm_note, llm_stance
		  FROM index_daily_closes
		 WHERE index_id = ?
		 ORDER BY trade_date DESC
		 LIMIT ?`, strings.ToLower(strings.TrimSpace(indexID)), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []IndexDailyCloseRow
	for rows.Next() {
		var row IndexDailyCloseRow
		if err := rows.Scan(
			&row.IndexID, &row.TradeDate, &row.Close, &row.PrevClose,
			&row.ChangeAmount, &row.ChangeRate, &row.Currency, &row.Source,
			&row.Score, &row.AISummary, &row.LLMNote, &row.LLMStance,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *IndexDailyCloseRepo) LastCloseBefore(ctx context.Context, indexID, date string) (float64, bool, error) {
	if r == nil || r.db == nil {
		return 0, false, nil
	}
	var close float64
	err := r.db.QueryRowContext(ctx, `
		SELECT close_value FROM index_daily_closes
		 WHERE index_id = ? AND trade_date < ?
		 ORDER BY trade_date DESC LIMIT 1`,
		strings.ToLower(strings.TrimSpace(indexID)), date,
	).Scan(&close)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return close, true, nil
}

func nullFloatArg(v sql.NullFloat64) any {
	if !v.Valid {
		return nil
	}
	return v.Float64
}

func nullIntArg(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func dailyCloseDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

// MigrateDailyClosesIfNeeded applies 029_daily_closes.sql.
func MigrateDailyClosesIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "stock_daily_closes")
	if err != nil {
		return err
	}
	indexExists, err := tableExists(ctx, db, "index_daily_closes")
	if err != nil {
		return err
	}
	if exists && indexExists {
		return nil
	}
	path := filepath.Join(migrationsDir, "029_daily_closes.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 029_daily_closes.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("029_daily_closes.sql: %w", err)
		}
	}
	return nil
}

/*
MigrateDailyCloseAIOverviewIfNeeded adds the per-day 핵심 요약 column on an
existing database (2026-09-15 사용자 요청 1).

표가 아니라 **컬럼**을 더하는 마이그레이션이라 `MigrateDailyClosesIfNeeded`에
얹을 수 없다 — 그쪽은 「표가 있으면 아무것도 안 한다」로 끝나므로, 이미 도는 DB는
표가 있다는 이유로 이 컬럼을 영영 못 받는다.
*/
func MigrateDailyCloseAIOverviewIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "stock_daily_closes")
	if err != nil {
		return err
	}
	// 표 자체가 없으면 029가 만들고, 그 정의에는 이 컬럼이 아직 없다 — 다음 기동에서
	// 이 함수가 채운다. 여기서 ALTER를 먼저 치면 「없는 표」 오류로 기동이 막힌다.
	if !exists {
		return nil
	}
	hasCol, err := columnExists(ctx, db, "stock_daily_closes", "ai_overview")
	if err != nil {
		return err
	}
	if hasCol {
		return nil
	}
	path := filepath.Join(migrationsDir, "073_daily_close_ai_overview.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 073_daily_close_ai_overview.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			/*
				**「이미 있다」는 실패가 아니다.** 실측에서 두 번 밟았다(2026-09-15 기동):

				  ① API 와 워커 여덟이 동시에 스키마를 맞추므로 하나가 이기고 나머지는
				     끝난 일을 보러 온다(`isSchemaRaceError` 머리글).
				  ② 이 표는 커서 ALTER 가 클라이언트 연결보다 오래 산다 — 연결이
				     끊겨 `invalid connection` 을 받았는데 서버에서는 컬럼이 붙고
				     있었고, 다음 기동의 `columnExists` 가 아직 false 를 봤다.

				둘 다 「이 컬럼이 있다」로 끝나는 상황이라, 여기서 오류를 내면 기동이
				막히고 서비스가 통째로 안 뜬다.
			*/
			if isSchemaRaceError(err) {
				continue
			}
			return fmt.Errorf("073_daily_close_ai_overview.sql: %w", err)
		}
	}
	log.Println("mysql: applied 073_daily_close_ai_overview.sql")
	return nil
}
