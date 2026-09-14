package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

// candleBarsPartitionDDL — bar_interval LIST 파티션.
//
// 배경과 '1M'을 목록에서 뺀 이유는 migrations/057_candle_bars_tiering.sql 헤더에
// 적혀 있다. 두 곳의 정의는 같아야 한다 — 새 설치는 .sql이, 기존 설치는 이쪽이
// 적용한다.
const candleBarsPartitionDDL = `
ALTER TABLE candle_bars
  PARTITION BY LIST COLUMNS(bar_interval) (
    PARTITION p_1m  VALUES IN ('1m'),
    PARTITION p_5m  VALUES IN ('5m'),
    PARTITION p_10m VALUES IN ('10m'),
    PARTITION p_30m VALUES IN ('30m'),
    PARTITION p_1d  VALUES IN ('1d'),
    PARTITION p_etc VALUES IN ('1w')
  )`

/*
MigrateCandleBarsTieringIfNeeded — 캔들 계층 저장의 스키마 준비.

두 단계로 나뉘고 **각각 따로 가드된다**. 한 문장으로 합칠 수 없기 때문이다
(MySQL은 ADD COLUMN과 PARTITION BY를 같은 ALTER에 못 쓴다). 그리고 파티션 ALTER는
표를 통째로 재구축하므로, 이미 파티션된 표에 다시 걸면 **아무 이득 없이 2~3분을
다시 쓴다** — 가드가 성능 문제가 아니라 정확성 문제인 이유다.

부팅을 막지 않는다. MigrateCandleBarsV2IfNeeded와 같은 비동기 경로에서 돈다.
*/
func MigrateCandleBarsTieringIfNeeded(ctx context.Context, db *sql.DB) error {
	exists, err := tableExists(ctx, db, "candle_bars")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	hasCol, err := columnExists(ctx, db, "candle_bars", "src_bars")
	if err != nil {
		return err
	}
	if !hasCol {
		// 맨 뒤에 붙여야 INSTANT가 된다. 위치를 지정하면 재구축이다.
		const addCol = `ALTER TABLE candle_bars
			ADD COLUMN src_bars TINYINT UNSIGNED NOT NULL DEFAULT 1,
			ALGORITHM=INSTANT`
		started := time.Now()
		if _, err := db.ExecContext(ctx, addCol); err != nil {
			return fmt.Errorf("candle_bars: add src_bars: %w", err)
		}
		log.Printf("mysql: candle_bars.src_bars added in %s", time.Since(started).Round(time.Millisecond))
	}

	partitioned, err := tableIsPartitioned(ctx, db, "candle_bars")
	if err != nil {
		return err
	}
	if partitioned {
		return nil
	}

	log.Println("mysql: partitioning candle_bars by bar_interval (table rebuild — writes block until done)")
	started := time.Now()
	if _, err := db.ExecContext(ctx, candleBarsPartitionDDL); err != nil {
		return fmt.Errorf("candle_bars: partition: %w", err)
	}
	log.Printf("mysql: candle_bars partitioned in %s", time.Since(started).Round(time.Second))
	return nil
}

/*
DropCandleBars60mPartitionIfNeeded — 60분봉 계층을 스키마에서 걷어낸다.

60분봉은 2026-08-24에 기능에서 제거됐다. 인터벌 중 혼자 압도적으로 비쌌기
때문이다 — `candleFetchCount`가 `120 × 분`이라 60분봉 첫 화면 한 번이 1분봉
7,200개(Toss `/candles` **36호출**)였고, 그 버킷은 사용자가 차트를 처음 열 때
쓰는 것과 같아서 60분봉을 여는 행위가 다른 사람의 차트를 밀어냈다.

**이 작업은 되돌릴 수 없다.** `DROP PARTITION`은 그 파티션의 데이터를 통째로
지운다. 그래도 안전한 이유는 60분봉이 **파생 데이터**이기 때문이다 — 1분봉과
1일봉이 원본이고, 60분봉은 롤업이 그것을 접어 만든 것이었다. 남은 계층
(1m·5m·10m·30m)과 영구 저장인 1d는 그대로다.

`DELETE`가 아니라 `DROP PARTITION`을 쓰는 이유: DELETE는 행마다 redo/undo를
쌓는데 이 배포 대상의 스토리지 처리량이 **29 MiB/s**다. DROP PARTITION은
파일 단위라 거의 즉시 끝난다.

드롭 뒤에는 `'60m'`을 받아 줄 파티션이 없어 그 값의 INSERT가 **에러**가 된다.
조용히 다른 곳에 들어가는 것보다 낫다 — 코드에서 `Interval60m`을 지웠으므로
그런 쓰기가 남아 있다면 그것이 버그다.
*/
func DropCandleBars60mPartitionIfNeeded(ctx context.Context, db *sql.DB) error {
	exists, err := tableExists(ctx, db, "candle_bars")
	if err != nil || !exists {
		return err
	}

	has, err := partitionExists(ctx, db, "candle_bars", "p_60m")
	if err != nil {
		return err
	}
	if !has {
		return nil
	}

	var rows int64
	// 지우기 전에 규모를 남긴다 — 되돌릴 수 없으므로 무엇을 지웠는지는 알아야 한다.
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM candle_bars PARTITION (p_60m)`).Scan(&rows); err != nil {
		log.Printf("mysql: candle_bars p_60m row count failed (계속 진행): %v", err)
	}

	log.Printf("mysql: dropping candle_bars partition p_60m (60분봉 기능 제거 · %d봉 삭제 · 되돌릴 수 없음)", rows)
	started := time.Now()
	if _, err := db.ExecContext(ctx, `ALTER TABLE candle_bars DROP PARTITION p_60m`); err != nil {
		return fmt.Errorf("candle_bars: drop p_60m: %w", err)
	}
	log.Printf("mysql: candle_bars p_60m dropped in %s", time.Since(started).Round(time.Millisecond))
	return nil
}

// partitionExists reports whether one named partition is defined on the table.
func partitionExists(ctx context.Context, db *sql.DB, table, partition string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.partitions
		WHERE table_schema = DATABASE() AND table_name = ? AND partition_name = ?
	`, table, partition).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("partition check %s.%s: %w", table, partition, err)
	}
	return n > 0, nil
}

// tableIsPartitioned reports whether the table has any partition defined.
// information_schema.partitions always has one row per table — partition_name is
// NULL when the table is not partitioned.
func tableIsPartitioned(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.partitions
		WHERE table_schema = DATABASE() AND table_name = ? AND partition_name IS NOT NULL
	`, table).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("partition check %s: %w", table, err)
	}
	return n > 0, nil
}
