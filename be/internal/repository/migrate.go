package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RequiredTables — BE가 사용하는 테이블. 추가 시 migrations/*.sql 과 함께 갱신.
var RequiredTables = []string{
	"users",
	"watchlist_items",
	"candle_bars",
	"stocks",
	"stock_fundamentals",
}

// EnsureSchema connects via MYSQL_DSN, verifies required tables exist,
// and runs migrations/*.sql only when any table is missing.
func EnsureSchema(ctx context.Context, db *sql.DB, migrationsDir string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}

	missing, err := missingTables(ctx, db, RequiredTables)
	if err != nil {
		return err
	}

	if len(missing) == 0 {
		if err := MigrateFundamentalsV2IfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateAuthIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateExtendedSchemaIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateNewsPreviewIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateNewsTranslationIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateNewsReaderTranslationIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateUserHoldingsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateAdminNewsKeywordsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateNewsPollSettingsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateKakaoPollIntervalIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateWatchlistAddedPriceIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateWatchlistSortOrderIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateMarketDataSettingsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateRankingsRealtimeDefaultIfNeeded(ctx, db); err != nil {
			return err
		}
		if err := MigrateLLMSettingsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateLLMRefreshIntervalIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateLLMBatchSettingsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateDailyClosesIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateStockScoresIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateIndexScoresIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateBatchJobRunsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateJournalNotesIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateJournalNoteSnapshotIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateStampsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateStampTargetPriceIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateResearchIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateResearchSocialIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateResearchThumbnailIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateHoldingTradesIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateNewsIndexKeywordsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateUserCashBalancesIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateNewsBriefingsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateUserPriceTargetsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigratePriceTargetLevelsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateHoldingTradesDedupIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateHoldingTradesPriceMismatchCleanupIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigratePerfIndexesIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateEarningsEventsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateInsiderTransactionsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateReferenceIngestIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateDropBrokerLinkingIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		log.Println("mysql: schema ok (all required tables present)")
		go migrateCandleBarsAsync(db)
		return nil
	}

	log.Printf("mysql: missing tables %v — applying migrations", missing)

	oldBackup, err := tableExists(ctx, db, "old_stock_fundamentals")
	if err != nil {
		return err
	}
	if oldBackup {
		if err := MigrateFundamentalsV2IfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
	} else if err := applyMigrationFiles(ctx, db, migrationsDir); err != nil {
		return err
	}

	stillMissing, err := missingTables(ctx, db, RequiredTables)
	if err != nil {
		return err
	}
	if len(stillMissing) > 0 {
		return fmt.Errorf("schema incomplete after migration: missing %v", stillMissing)
	}

	if err := MigrateFundamentalsV2IfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateAuthIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateExtendedSchemaIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateNewsPreviewIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateNewsTranslationIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateNewsReaderTranslationIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateUserHoldingsIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateAdminNewsKeywordsIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateNewsPollSettingsIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateKakaoPollIntervalIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateWatchlistAddedPriceIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateWatchlistSortOrderIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateMarketDataSettingsIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateRankingsRealtimeDefaultIfNeeded(ctx, db); err != nil {
		return err
	}
	if err := MigrateLLMSettingsIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateLLMRefreshIntervalIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateLLMBatchSettingsIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateDailyClosesIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateStockScoresIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateIndexScoresIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateBatchJobRunsIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateJournalNotesIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateJournalNoteSnapshotIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateStampsIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateStampTargetPriceIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateResearchIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateResearchSocialIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateResearchThumbnailIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateHoldingTradesIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateNewsIndexKeywordsIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateUserCashBalancesIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateNewsBriefingsIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateUserPriceTargetsIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigratePriceTargetLevelsIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateHoldingTradesDedupIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateHoldingTradesPriceMismatchCleanupIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigratePerfIndexesIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateEarningsEventsIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateInsiderTransactionsIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateReferenceIngestIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateDropBrokerLinkingIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	go migrateCandleBarsAsync(db)

	log.Println("mysql: schema ready")
	return nil
}

/*
VerifySchema — 워커 프로세스용. **DDL을 실행하지 않고** 스키마가 준비됐는지만
확인한다.

여태까지는 api와 워커 7개가 전부 기동 시 EnsureSchema를 불렀다. 버전 테이블 없이
MigrateXIfNeeded 약 30개가 각자 "이 테이블/컬럼이 있나"를 마커로 판단하는 구조라,
8개 프로세스가 같은 DDL을 동시에 실행하며 경쟁했다(AGENT_STATE에 isSchemaRaceError를
무시한다고 적혀 있는 것이 그 흔적이다). 로컬에서 무사했던 이유는 스키마가 늘 이미
최신이라 대부분의 마커가 곧바로 빠져나갔기 때문이고, **실제로 DDL이 도는 프로덕션
첫 배포가 가장 위험했다.**

이제 마이그레이터는 api 하나뿐이다. 워커는 필요한 테이블이 보일 때까지 기다렸다가
시작한다 — compose의 depends_on(api: service_healthy)이 이미 순서를 잡아 주지만,
클라우드처럼 기동 순서를 보장하기 어려운 환경에서도 워커가 스스로 버티게 한다.

기다리다 지치면 에러를 낸다. 워커는 restart 정책이 걸려 있어 다시 시도한다.
*/
func VerifySchema(ctx context.Context, db *sql.DB, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	var lastMissing []string

	for {
		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := db.PingContext(checkCtx)
		if err == nil {
			var missing []string
			missing, err = missingTables(checkCtx, db, RequiredTables)
			if err == nil {
				if len(missing) == 0 {
					cancel()
					// EnsureSchema와 다른 문구를 쓴다 — 로그만 보고 "이 프로세스가
					// 마이그레이션을 했는지"를 구분할 수 있어야 한다.
					log.Println("mysql: schema verified (read-only; api is the migrator)")
					return nil
				}
				lastMissing = missing
				err = fmt.Errorf("missing tables: %s", strings.Join(missing, ", "))
			}
		}
		cancel()

		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			if len(lastMissing) > 0 {
				return fmt.Errorf("schema not ready after %s (%w) — is the api container running its migrations?", wait, err)
			}
			return fmt.Errorf("schema not ready after %s: %w", wait, err)
		}

		log.Printf("mysql: waiting for schema (%v)", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

func missingTables(ctx context.Context, db *sql.DB, required []string) ([]string, error) {
	var missing []string
	for _, table := range required {
		var name string
		err := db.QueryRowContext(ctx, `
			SELECT table_name
			FROM information_schema.tables
			WHERE table_schema = DATABASE() AND table_name = ?
		`, table).Scan(&name)
		if err == sql.ErrNoRows {
			missing = append(missing, table)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("check table %s: %w", table, err)
		}
	}
	return missing, nil
}

func applyMigrationFiles(ctx context.Context, db *sql.DB, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, filepath.Join(migrationsDir, e.Name()))
	}
	sort.Strings(files)

	for _, path := range files {
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", path, err)
		}
		for _, stmt := range splitSQLStatements(string(body)) {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("migration %s: %w", filepath.Base(path), err)
			}
		}
	}
	return nil
}

func splitSQLStatements(sqlText string) []string {
	var out []string
	for _, part := range strings.Split(sqlText, ";") {
		stmt := strings.TrimSpace(part)
		if stmt == "" {
			continue
		}
		out = append(out, stmt)
	}
	return out
}

func migrateCandleBarsAsync(db *sql.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := MigrateCandleBarsV2IfNeeded(ctx, db); err != nil {
		log.Printf("mysql: candle_bars migration (background): %v", err)
	}
}
