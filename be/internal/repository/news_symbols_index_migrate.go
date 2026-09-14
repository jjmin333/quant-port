package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

/*
MigrateNewsSymbolsIndexIfNeeded — 종목 필터가 인덱스를 타게 만든다.

배경과 「왜 조인 테이블이 아닌가」는 migrations/061_news_symbols_mv_index.sql
머리말에 있다. 두 곳의 정의는 같아야 한다 — 새 설치는 .sql이, 기존 설치는
이쪽이 적용한다.

두 단계이고 각각 따로 가드된다:

	① symbols_json의 JSON null을 []로 정규화 — MVI가 JSON null에서 실패하므로 선행이다.
	② multi-valued index 추가.

부팅을 막지 않는다. 다른 candle_bars 마이그레이션과 같은 비동기 경로에서 돈다.
*/
func MigrateNewsSymbolsIndexIfNeeded(ctx context.Context, db *sql.DB) error {
	exists, err := tableExists(ctx, db, "news_articles")
	if err != nil || !exists {
		return err
	}

	if err := normalizeNewsSymbolsJSON(ctx, db); err != nil {
		return err
	}

	has, err := indexExists(ctx, db, "news_articles", "idx_news_symbols_mv")
	if err != nil {
		return err
	}
	if has {
		return nil
	}

	/*
		`LOCK=SHARED`인 이유 — LOCK=NONE은 **쓸 수 없다.**

		multi-valued index를 만들면 MySQL이 내부적으로 숨은 가상 컬럼을 하나
		추가하는데, 그 조합(ADD COLUMN ... VIRTUAL + ADD INDEX)은 온라인 DDL의
		무잠금 모드를 지원하지 않는다:

			Error 1846 (0A000): LOCK=NONE is not supported.
			Reason: ADD COLUMN col...VIRTUAL, ADD INDEX(col). Try LOCK=SHARED.

		LOCK=SHARED는 **읽기는 그대로 두고 쓰기만 막는다.** 이 표에 쓰는 것은
		뉴스 폴러(2분 주기)와 조회수 증가뿐이고, 93k행 인덱스 빌드는 초 단위라
		그 사이 쓰기는 잠깐 기다렸다 진행된다. 사용자 조회는 영향받지 않는다.

		그래도 장 시간 한복판에 돌 이유는 없다 — 이 마이그레이션은 기동 시
		비동기 경로에서 한 번만 실행되고, 실패하면 다음 기동에 다시 시도한다.
	*/
	log.Println("mysql: adding news_articles multi-valued index on symbols_json (writes briefly block)")
	started := time.Now()
	const ddl = `ALTER TABLE news_articles
		ADD INDEX idx_news_symbols_mv ((CAST(symbols_json->'$' AS CHAR(8) ARRAY))),
		ALGORITHM=INPLACE, LOCK=SHARED`
	if _, err := db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("news_articles: add symbols mv index: %w", err)
	}
	log.Printf("mysql: news_articles symbols index added in %s", time.Since(started).Round(time.Millisecond))
	return nil
}

/*
normalizeNewsSymbolsJSON — `symbols_json`의 JSON null을 빈 배열로 통일한다.

`json.Marshal`이 nil 슬라이스를 `null`로 쓰는 탓에 생긴 값이다(쓰기 쪽은
`marshalJSONStringArray`가 고쳤다). 실측 93,469행 중 28,470행이 그 상태였다.

**청크로 나눠 돈다.** 한 문장으로 전부 고치면 그 트랜잭션이 락을 오래 쥐는데,
이 표에는 뉴스 폴러가 2분마다 쓰고 있고 이 배포 대상의 스토리지 처리량은
29 MiB/s다. 남은 행이 없을 때까지 반복하므로 중간에 끊겨도 다음 기동이 이어받는다.
*/
func normalizeNewsSymbolsJSON(ctx context.Context, db *sql.DB) error {
	const chunk = 5000
	total := int64(0)
	for {
		res, err := db.ExecContext(ctx, `
			UPDATE news_articles
			   SET symbols_json = JSON_ARRAY()
			 WHERE symbols_json IS NOT NULL
			   AND JSON_TYPE(symbols_json) = 'NULL'
			 LIMIT ?`, chunk)
		if err != nil {
			return fmt.Errorf("news_articles: normalize symbols_json: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil || n == 0 {
			break
		}
		total += n
		if ctx.Err() != nil {
			break
		}
	}
	if total > 0 {
		log.Printf("mysql: news_articles symbols_json normalized (%d rows JSON null -> [])", total)
	}
	return nil
}
