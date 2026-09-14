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
// schemaMigrationTimeout — EnsureSchema 전체에 걸리는 마감.
//
// 60초는 "이미 마이그레이션이 끝난 서버가 다시 뜨는" 평상시에 맞춘 값이다. 큰 표를
// 건드리는 마이그레이션은 여기서 잘린다 — 그러면 api가 기동에 실패하고, 워커도
// 스키마를 기다리다 함께 못 뜬다(전면 정지). 운영 DB는 1 ECPU에 스토리지 29 MiB/s라
// 로컬보다 훨씬 느리므로, 그런 배포에서는 값을 올려 잡을 수 있어야 한다.
//
// 큰 ALTER는 배포에 섞지 말고 따로 돌리는 것이 원칙이다(docs/OCI_MIGRATION_PLAN.md §4.7).
// 이 환경변수는 그 원칙을 못 지킨 배포가 서비스를 통째로 세우지 않게 하는 안전판이다.
func schemaMigrationTimeout() time.Duration {
	return time.Duration(envInt("SCHEMA_MIGRATION_TIMEOUT_SECONDS", 60)) * time.Second
}

/*
EnsureSchema brings the database up to date and wires the caches that read it.

# 왜 얇은 껍데기가 하나 있나 (2026-09-13)

본체(`ensureSchema`)는 **여러 곳에서 일찍 돌아간다** — 필요한 표가 다 있으면
중간에서, 새 DB면 끝에서. 스키마가 준비된 뒤에 해야 하는 일을 본체 안에 적으면
그 출구 중 하나를 반드시 빠뜨린다. 실제로 한 번 빠뜨렸다: 별칭 캐시 배선을 본체
끝에 뒀더니 **운영 DB(표가 다 있는 경로)에서는 한 번도 안 불렸고**, 그래도 아무
오류가 안 났다 — 캐시는 씨앗으로 조용히 돌아가기 때문이다.

그래서 「끝나고 하는 일」은 출구가 하나인 여기에 둔다.
*/
func EnsureSchema(ctx context.Context, db *sql.DB, migrationsDir string) error {
	if err := ensureSchema(ctx, db, migrationsDir); err != nil {
		return err
	}
	// 별칭 캐시는 스키마를 보장하는 **모든** 프로세스(api·워커·cmd)가 쓴다.
	// 프로세스마다 붙이면 빠뜨리는 곳이 생기고, 빠뜨려도 오류가 안 난다.
	SetStockAliasSource(ctx, db)
	return nil
}

func ensureSchema(ctx context.Context, db *sql.DB, migrationsDir string) error {
	ctx, cancel := context.WithTimeout(ctx, schemaMigrationTimeout())
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
		if err := MigrateLLMBatchCapsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateBatchHealthIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateOpsIncidentsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateTossTradableIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateStockAliasesIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateNewsTranslateAttemptsIfNeeded(ctx, db, migrationsDir); err != nil {
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
		if err := MigrateResearchCommentLikesIfNeeded(ctx, db, migrationsDir); err != nil {
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
		if err := MigrateNewsPopularIndexIfNeeded(ctx, db, migrationsDir); err != nil {
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
		if err := MigrateEarningsSurpriseHistoryIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateEarningsSurpriseAnnouncedIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateStockScoreRankingsIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		if err := MigrateStockSummaryNarrativesIfNeeded(ctx, db, migrationsDir); err != nil {
			return err
		}
		log.Println("mysql: schema ok (all required tables present)")
		CheckInnoDBBufferPool(ctx, db)
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
	if err := MigrateResearchCommentLikesIfNeeded(ctx, db, migrationsDir); err != nil {
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
	if err := MigrateNewsPopularIndexIfNeeded(ctx, db, migrationsDir); err != nil {
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
	if err := MigrateEarningsSurpriseHistoryIfNeeded(ctx, db, migrationsDir); err != nil {
		return err
	}
	if err := MigrateEarningsSurpriseAnnouncedIfNeeded(ctx, db, migrationsDir); err != nil {
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
		// bar_time이 DATETIME이 아니면 계층 마이그레이션의 전제가 깨진다.
		// 롤업이 쓰는 버킷 산술이 DATETIME 함수이기 때문이다.
		return
	}
	if err := MigrateCandleBarsTieringIfNeeded(ctx, db); err != nil {
		log.Printf("mysql: candle_bars tiering migration (background): %v", err)
		// 파티션이 준비되지 않았으면 아래 드롭도 의미가 없다.
		return
	}
	// 60분봉 제거(2026-08-24). 계층 마이그레이션 **뒤에** 와야 한다 — 위에서
	// 파티션을 새로 만드는 경우 이미 p_60m 없이 만들어지므로 이 호출은 no-op이다.
	if err := DropCandleBars60mPartitionIfNeeded(ctx, db); err != nil {
		log.Printf("mysql: candle_bars p_60m drop (background): %v", err)
	}
	// 뉴스 종목 필터 인덱스. 같은 비동기 경로에 둔다 — 부팅을 막을 이유가 없다.
	if err := MigrateNewsSymbolsIndexIfNeeded(ctx, db); err != nil {
		log.Printf("mysql: news symbols index (background): %v", err)
	}
}
