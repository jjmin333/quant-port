package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

/*
종목별 **마지막 LLM 요약 글**의 영구 보관소 (migrations/065).

Redis 캐시는 24시간이면 사라진다. 그 안에 아무도 그 종목을 열지 않으면 돈 주고 만든
글이 함께 사라졌다 — 마이그레이션 머리글에 그 경위가 적혀 있다. 여기 한 벌을 남겨
두면 캐시가 비어도 「가장 최근에 뽑힌 글」을 되찾을 수 있다.
*/
type StockSummaryNarrativeRepo struct {
	db *sql.DB
}

func NewStockSummaryNarrativeRepo(db *sql.DB) *StockSummaryNarrativeRepo {
	return &StockSummaryNarrativeRepo{db: db}
}

/*
StockSummaryNarrative — 화면을 되살리는 데 필요한 **글만** 담는다.

숫자(시세·재무·점수·시나리오)는 넣지 않는다. 그건 매 빌드가 다시 계산하는 값이고,
여기에 함께 얼려 두면 오래된 숫자가 새 글 옆에 되살아난다.
*/
type StockSummaryNarrative struct {
	Overview      string                      `json:"overview,omitempty"`
	Valuation     string                      `json:"valuation,omitempty"`
	Technical     string                      `json:"technical,omitempty"`
	Insider       string                      `json:"insider,omitempty"`
	News          string                      `json:"news,omitempty"`
	Keywords      []string                    `json:"keywords,omitempty"`
	NewsSentiment *model.NewsSentimentSummary `json:"newsSentiment,omitempty"`
	// AnalyzedAt은 이 글이 실제로 쓰인 시각(RFC3339). 숫자를 다시 계산한 시각과
	// 구분해야 화면이 「어제 글 + 오늘 숫자」를 정직하게 말할 수 있다.
	AnalyzedAt string `json:"analyzedAt,omitempty"`
}

func normalizeNarrativeSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}

// Save replaces one symbol's stored narrative.
func (r *StockSummaryNarrativeRepo) Save(
	ctx context.Context,
	symbol string,
	narrative StockSummaryNarrative,
	analyzedAt time.Time,
) error {
	if r == nil || r.db == nil {
		return nil
	}
	sym := normalizeNarrativeSymbol(symbol)
	if sym == "" {
		return nil
	}
	payload, err := json.Marshal(narrative)
	if err != nil {
		return fmt.Errorf("marshal narrative %s: %w", sym, err)
	}
	if analyzedAt.IsZero() {
		analyzedAt = time.Now()
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO stock_summary_narratives (symbol, payload, analyzed_at)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  payload     = VALUES(payload),
		  analyzed_at = VALUES(analyzed_at)`,
		sym, payload, analyzedAt,
	)
	if err != nil {
		return fmt.Errorf("save narrative %s: %w", sym, err)
	}
	return nil
}

// Get returns the stored narrative, or nil when this symbol has never had one.
func (r *StockSummaryNarrativeRepo) Get(
	ctx context.Context,
	symbol string,
) (*StockSummaryNarrative, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	sym := normalizeNarrativeSymbol(symbol)
	if sym == "" {
		return nil, nil
	}
	var payload []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT payload FROM stock_summary_narratives WHERE symbol = ?`, sym,
	).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get narrative %s: %w", sym, err)
	}
	var out StockSummaryNarrative
	if err := json.Unmarshal(payload, &out); err != nil {
		// 깨진 행 하나 때문에 요약 자체가 실패하면 안 된다 — 규칙 글로 내려간다.
		return nil, nil
	}
	return &out, nil
}

/*
AnalyzedAt — 종목별 **마지막으로 글이 쓰인 시각**을 한 번에 읽는다.

배치의 대상 선정이 「요약이 없는 종목 먼저, 그다음 오래된 순」으로 줄을 세우는 데
쓴다. 글 본문(payload)은 읽지 않는다 — 선정에 필요한 것은 날짜뿐이고, 본문까지
끌어오면 종목당 1~2KB를 수백 개 들고 오게 된다.

심볼 목록을 받지 않고 표 전체를 읽는 이유: 한 행이 16바이트짜리 두 값이고 행 수는
분석해 본 종목 수(상한이 카탈로그 17,900)라, IN 목록을 수백 개 만들어 보내는 것보다
싸고 호출부가 단순하다.
*/
func (r *StockSummaryNarrativeRepo) AnalyzedAt(ctx context.Context) (map[string]time.Time, error) {
	if r == nil || r.db == nil {
		return map[string]time.Time{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT symbol, analyzed_at FROM stock_summary_narratives`)
	if err != nil {
		return nil, fmt.Errorf("list narrative analyzed_at: %w", err)
	}
	defer rows.Close()

	out := map[string]time.Time{}
	for rows.Next() {
		var symbol string
		var at time.Time
		if err := rows.Scan(&symbol, &at); err != nil {
			return nil, fmt.Errorf("list narrative analyzed_at: %w", err)
		}
		out[normalizeNarrativeSymbol(symbol)] = at
	}
	return out, rows.Err()
}

// MigrateStockSummaryNarrativesIfNeeded creates the table on an existing
// database (a fresh one gets it from the full migration run).
func MigrateStockSummaryNarrativesIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "stock_summary_narratives")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir,
		"065_stock_summary_narratives.sql", "AI 요약 글 영구 보관")
}
