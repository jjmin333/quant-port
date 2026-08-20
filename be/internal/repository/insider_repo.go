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

const insiderDateLayout = "2006-01-02"

// InsiderRepo persists 내부자 거래.
//
// MySQL이 source of truth다. Redis 캐시를 두지 않는 이유는 이 데이터의 성격
// 때문이다 — 하루 한 번 갱신되고 과거 이력은 바뀌지 않으므로, 캐시가 줄여줄
// 것은 외부 호출이 아니라 인덱스 하나 타는 SELECT뿐이다.
//
// "다음에 누구를 받을지"와 "받은 적이 있는지"는 여기 없다 — 모든 종목 단위
// 배치가 공유하는 SyncStateRepo가 맡는다.
type InsiderRepo struct {
	db *sql.DB
}

func NewInsiderRepo(db *sql.DB) *InsiderRepo {
	return &InsiderRepo{db: db}
}

// Upsert writes one symbol's transactions. Re-running a batch over filings it
// already stored updates them in place — the unique key is (symbol, source,
// filing_key, line_no), which every source can produce deterministically.
func (r *InsiderRepo) Upsert(ctx context.Context, rows []model.InsiderTransactionRow) (int, error) {
	if r == nil || r.db == nil || len(rows) == 0 {
		return 0, nil
	}

	// 날짜가 없거나 못 읽는 행은 버린다. Form 4는 거래일과 보고기간이 둘 다 빈
	// 문서가 드물게 있고, DART 날짜 파싱이 실패하면 원문("20260101")이 그대로
	// 온다 — 어느 쪽이든 DATE 컬럼에 넣으면 그 종목의 INSERT 전체가 죽는다.
	// 한 행 때문에 나머지를 잃지 않도록 여기서 거른다.
	rows = filterDatedInsiderRows(rows)
	if len(rows) == 0 {
		return 0, nil
	}

	const chunk = 300
	written := 0
	for start := 0; start < len(rows); start += chunk {
		end := start + chunk
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[start:end]

		var sb strings.Builder
		sb.WriteString(`INSERT INTO insider_transactions
			(symbol, source, filing_key, line_no, txn_date,
			 reporter_name, relation, transaction_type,
			 shares_change, change_rate, shares_owned_after,
			 ownership_rate, price_per_share)
			VALUES `)
		args := make([]any, 0, len(batch)*13)
		for i, row := range batch {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?,?,?,?,?,?,?,?,?,?)")
			t := row.Txn
			args = append(args,
				strings.ToUpper(strings.TrimSpace(row.Symbol)),
				t.Source,
				row.FilingKey,
				row.LineNo,
				t.Date,
				t.ReporterName,
				t.Relation,
				t.TransactionType,
				nullInt64(t.SharesChange),
				nullFloat64(t.ChangeRate),
				nullInt64(t.SharesOwnedAfter),
				nullFloat64(t.OwnershipRate),
				nullFloat64(t.PricePerShare),
			)
		}
		sb.WriteString(`
			ON DUPLICATE KEY UPDATE
			  txn_date = VALUES(txn_date),
			  reporter_name = VALUES(reporter_name),
			  relation = VALUES(relation),
			  transaction_type = VALUES(transaction_type),
			  shares_change = VALUES(shares_change),
			  change_rate = VALUES(change_rate),
			  shares_owned_after = VALUES(shares_owned_after),
			  ownership_rate = VALUES(ownership_rate),
			  price_per_share = VALUES(price_per_share)`)

		if _, err := r.db.ExecContext(ctx, sb.String(), args...); err != nil {
			return written, fmt.Errorf("insider upsert: %w", err)
		}
		written += len(batch)
	}
	return written, nil
}

func filterDatedInsiderRows(rows []model.InsiderTransactionRow) []model.InsiderTransactionRow {
	out := rows[:0:0]
	for _, row := range rows {
		if _, err := time.Parse(insiderDateLayout, strings.TrimSpace(row.Txn.Date)); err != nil {
			continue
		}
		out = append(out, row)
	}
	return out
}

// ListForSymbol returns the most recent transactions for one symbol.
func (r *InsiderRepo) ListForSymbol(ctx context.Context, symbol string, limit int) ([]model.InsiderTransaction, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 30
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT txn_date, reporter_name, relation, transaction_type,
		       shares_change, change_rate, shares_owned_after,
		       ownership_rate, price_per_share, source
		FROM insider_transactions
		WHERE symbol = ?
		ORDER BY txn_date DESC, id DESC
		LIMIT ?`,
		strings.ToUpper(strings.TrimSpace(symbol)), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.InsiderTransaction, 0, limit)
	for rows.Next() {
		var (
			t    model.InsiderTransaction
			date time.Time
		)
		if err := rows.Scan(
			&date, &t.ReporterName, &t.Relation, &t.TransactionType,
			&t.SharesChange, &t.ChangeRate, &t.SharesOwnedAfter,
			&t.OwnershipRate, &t.PricePerShare, &t.Source,
		); err != nil {
			return nil, err
		}
		t.Date = date.Format(insiderDateLayout)
		out = append(out, t)
	}
	return out, rows.Err()
}

// KnownFilingKeys returns the filing keys already stored for a symbol, so an
// incremental run can skip downloading filings it has.
//
// 이 함수가 증분 적재의 전부다. EDGAR는 종목당 submissions.json 1회로 최근
// 공시 목록을 주는데, 그중 이미 가진 accession을 빼면 평시에 내려받을 XML은
// 0~2건이다 — 첫 적재만 비싸고 이후는 거의 공짜다.
func (r *InsiderRepo) KnownFilingKeys(ctx context.Context, symbol, source string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	if r == nil || r.db == nil {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT filing_key FROM insider_transactions
		WHERE symbol = ? AND source = ?`,
		strings.ToUpper(strings.TrimSpace(symbol)), source)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return out, err
		}
		out[key] = struct{}{}
	}
	return out, rows.Err()
}

// MigrateInsiderTransactionsIfNeeded applies 052_insider_transactions.sql
// (내부자 거래 표 + 모든 종목 단위 배치가 공유하는 symbol_sync_state).
func MigrateInsiderTransactionsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "symbol_sync_state")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir, "052_insider_transactions.sql", "내부자 거래 + 적재 상태")
}

// MigrateReferenceIngestIfNeeded applies 053_reference_ingest.sql
// (분기 재무 · 공시 · 종목 경고).
func MigrateReferenceIngestIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "financial_periods")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir, "053_reference_ingest.sql", "분기 재무·공시·종목 경고")
}

// applyMigrationFile runs one migration file, tolerating the concurrent-DDL
// race that this deployment shape produces.
func applyMigrationFile(ctx context.Context, db *sql.DB, migrationsDir, name, label string) error {
	body, err := os.ReadFile(filepath.Join(migrationsDir, name))
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			// 컨테이너 여럿이 함께 부팅해 같은 CREATE TABLE을 동시에 내면 진
			// 쪽이 에러를 받는다 — 이 배포 형태에서는 정상적인 결과다.
			if isSchemaRaceError(err) {
				continue
			}
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	log.Printf("mysql: applied %s (%s)", name, label)
	return nil
}
