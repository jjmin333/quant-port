package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

/*
토스 유니버스 — 「토스가 이 종목의 거래를 지원하는가」 (migrations/069).

설계의 배경은 migration 069 머리글에 있다. 이 파일이 지키는 규칙 하나:

**모르는 것(NULL)은 숨기지 않는다.** 유니버스 배치가 한 번도 안 돌았거나 새로 들어온
종목은 NULL이고, 그 상태를 「지원 안 함」으로 다루면 배치가 처음 돌기 전까지 화면이
통째로 빈다. 조회 조건은 언제나 `toss_tradable IS NULL OR toss_tradable = 1`이다.
*/
type TossUniverseRepo struct {
	db *sql.DB
}

func NewTossUniverseRepo(db *sql.DB) *TossUniverseRepo {
	return &TossUniverseRepo{db: db}
}

/*
minUniverse — 「분명히 잘못된 응답」의 문턱.

실측 유니버스는 15,000건 단위다. 1,000은 상류 장애·권한 변화·필터 의미 변경으로 빈
배열이나 몇 건만 오는 경우를 걸러 내는 값이고, **반영 전(입력)과 반영 후(결과)
양쪽에서** 같은 문턱으로 쓴다 — 어느 쪽이든 이 아래로 내려가면 화면이 빈다.
*/
const minUniverse = 1000

/*
TossTradableFilter — 조회에 붙이는 SQL 조각.

`alias`는 stocks 테이블의 별칭이다(`st`·`s` 등). 빈 문자열이면 컬럼만 쓴다.

**이 문자열이 한 곳에만 있어야 한다.** 조회마다 조건을 새로 쓰면 어느 화면은 숨기고
어느 화면은 안 숨기는 상태가 되고, 그 차이는 사용자가 「검색에는 나오는데 목록에는
없다」로 만난다.
*/
func TossTradableFilter(alias string) string {
	col := "toss_tradable"
	if alias != "" {
		col = alias + ".toss_tradable"
	}
	return "(" + col + " IS NULL OR " + col + " = 1)"
}

/*
ApplyUniverse — 토스 유니버스 **전체**를 한 번에 반영한다.

# 왜 마켓 단위가 아닌가 (중요)

처음에는 마켓 단위로 쓰려 했는데 **우리 `market` 컬럼과 토스의 market 파라미터가
같은 분류가 아니다.** 실측:

	우리: NASDAQ 5,359 · NYSE 2,793 · ARCX 2,655 · KRX 2,323 · KOSDAQ 1,827
	      BATS 1,544 · OOTC 797 · AMEX 345 · KOSPI 155 · KONEX 107 · US_ETC 13
	토스: KOSPI · KOSDAQ · NYSE · NASDAQ · AMEX · KR_ETC · US_ETC

ARCX·BATS·OOTC(약 5,000종목)는 토스가 `US_ETC`로 답하고 KRX·KONEX는 `KR_ETC`로
답한다. 그래서 「이 마켓의 나머지를 0으로」를 우리 컬럼 기준으로 돌리면 **ARCX 2,655
종목이 통째로 숨는다.** 마켓 이름을 매핑해 맞출 수도 있지만, 그 매핑이 틀리는 날
증상은 「종목이 화면에서 사라진다」이고 그건 조용히 지나가지 않는다.

그래서 마켓을 매핑하지 않는다. 일곱 마켓의 **합집합이 곧 토스 유니버스 전체**이므로,
그 합집합에 없는 NORMAL 종목이 거래 불가다. 우리 분류를 몰라도 판정이 성립한다.

# 그래서 all-or-nothing 이다

일곱 호출이 **전부 성공해야** 반영한다(`complete`). 하나라도 실패한 합집합은 그
마켓만큼 작으므로, 그걸로 「나머지를 0으로」를 돌리면 멀쩡한 종목이 숨는다.
되돌릴 수 없는 종류의 실수라 호출부가 아니라 여기서 막는다.
*/
func (r *TossUniverseRepo) ApplyUniverse(
	ctx context.Context, symbols []string, complete bool,
) (tradable, hidden int64, err error) {
	/*
		**검증이 배선 확인보다 먼저다.**

		예전에는 `r.db == nil` 을 먼저 보고 nil 오류로 빠져나갔다. 그러면 배선이 안 된
		환경에서 「불완전한 수집」·「터무니없이 작은 합집합」이 **조용히 통과**한다 —
		거부해야 하는 입력을 거부했는지 시험할 수도 없다. 이 두 조건은 저장소의
		상태가 아니라 **호출부가 준 값**에 대한 판단이므로 먼저 본다.
	*/
	if !complete {
		return 0, 0, fmt.Errorf("toss universe: 일곱 마켓이 다 오지 않았다 — 반영하지 않는다")
	}
	/*
		합집합이 터무니없이 작으면 반영하지 않는다.

		상류가 빈 배열이나 몇 건만 주는 경우(장애·권한 변화·필터 의미 변경)에 그것을
		「거래 가능한 종목이 이것뿐」으로 받아들이면 화면이 통째로 빈다. 실측 유니버스는
		만 단위이므로 1,000은 「분명히 잘못된 응답」의 문턱이다.
	*/
	if len(symbols) < minUniverse {
		return 0, 0, fmt.Errorf(
			"toss universe: 합집합이 %d건뿐이다 (최소 %d) — 반영하지 않는다",
			len(symbols), minUniverse)
	}

	// 여기까지 왔으면 입력은 옳다. 배선이 없으면 아무 일도 하지 않는다.
	if r == nil || r.db == nil {
		return 0, 0, nil
	}

	/*
		**두 문장을 한 트랜잭션에 넣는다** (2026-09-12 실사고).

		이 함수는 「받은 것을 1로」와 「나머지를 0으로」 두 문장인데, 처음에는 그냥
		차례로 실행했다. 그 사이에 프로세스가 죽으면 **두 번째만 적용된 상태**,
		곧 「전 종목 숨김」이 남는다.

		실제로 그 일이 일어났다: 01:00 예약 실행 중에 배포로 워커가 재시작됐고,
		17,918종목이 전부 `toss_tradable=0`이 되어 **검색이 통째로 비었다**
		(AAPL 조회도 0건). 되돌릴 수 없는 종류의 실패를 「그 사이에 죽지 않는다」는
		가정으로 막고 있었던 것이다.

		트랜잭션이면 커밋 전에 죽어도 예전 상태가 남는다. 거는 비용은 한 번의
		커밋이고, 이 잡은 하루 한 번이다.
	*/
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("toss universe: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	/*
		**초 단위로 자른 시각을 쓴다** (2026-09-12 실사고의 진짜 원인).

		`toss_checked_at`은 `TIMESTAMP`이고 그 정밀도는 **0(초)**이다
		(information_schema 확인). Go의 `time.Now()`는 나노초를 담으므로, 그 값을
		그대로 저장하면 초 단위로 잘려 들어가고 **같은 값을 비교에 쓰면 어긋난다.**
		실험으로 확인:

			SET @t = '2026-09-12 02:00:00.456';
			SELECT CAST(@t AS DATETIME(0)) <> @t;   -- 1 (참!)
			SELECT CAST(@t AS DATETIME(0)) <  @t;   -- 1 (참!)

		즉 「방금 1로 만든 행」이 「이번에 안 온 행」 조건에 걸려 곧바로 0이 된다.
		그 결과가 전 종목 숨김이고, 실제로 그 일이 일어나 검색이 통째로 비었다.

		자르면 저장값과 비교값이 **문자 그대로 같아져** 이 창이 닫힌다. 정밀도에
		기대는 코드를 남기지 않는 것이 요점이다 — 컬럼 정의가 바뀌어도 안전하다.
	*/
	now := time.Now().Truncate(time.Second)
	// 받은 것을 1로. 심볼이 만 단위라 한 문장에 다 넣지 않고 쪼갠다 —
	// 플레이스홀더가 수천 개인 쿼리는 파서와 로그 양쪽에서 비싸다.
	const chunk = 500
	for start := 0; start < len(symbols); start += chunk {
		end := start + chunk
		if end > len(symbols) {
			end = len(symbols)
		}
		part := symbols[start:end]
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(part)), ",")
		args := make([]any, 0, len(part)+1)
		args = append(args, now)
		for _, sym := range part {
			args = append(args, strings.ToUpper(strings.TrimSpace(sym)))
		}
		res, execErr := tx.ExecContext(ctx, `
			UPDATE stocks
			   SET toss_tradable = 1, toss_checked_at = ?
			 WHERE symbol IN (`+placeholders+`)`, args...)
		if execErr != nil {
			return 0, 0, fmt.Errorf("toss universe: mark tradable: %w", execErr)
		}
		n, _ := res.RowsAffected()
		tradable += n
	}

	/*
		이번에 안 온 것을 0으로.

		**심볼 집합으로 판단한다 — 시각으로 하지 않는다.** 예전에는 「방금 찍은
		시각보다 오래된 것」으로 골랐는데, 그 방식은 `toss_checked_at`의 저장 정밀도에
		기댄다(TIMESTAMP는 기본이 초 단위인데 Go의 시각은 나노초다). 비교 한쪽만
		잘리면 방금 1로 만든 행이 곧바로 0이 되고, 그 결과가 위 머리글의 사고다.

		집합 연산은 정밀도와 무관하다: 「이번 응답에 없던 NORMAL 종목」이 정확히
		숨길 대상이다.
	*/
	res, err := tx.ExecContext(ctx, `
		UPDATE stocks
		   SET toss_tradable = 0, toss_checked_at = ?
		 WHERE status = 'NORMAL' AND toss_checked_at <> ?`, now, now)
	if err != nil {
		return 0, 0, fmt.Errorf("toss universe: mark hidden: %w", err)
	}
	hidden, _ = res.RowsAffected()

	/*
		**「전부 숨김」은 불가능해야 한다.**

		앞의 두 문장이 어떤 이유로든 어긋나면 증상은 언제나 같다 — 카탈로그가 통째로
		사라진다. 그 결과를 커밋 전에 막는다. 유니버스가 1,000건 이상 왔는데 남는
		종목이 그보다 적으면 무언가 틀린 것이다.
	*/
	var visible int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM stocks
		 WHERE status = 'NORMAL' AND (toss_tradable IS NULL OR toss_tradable = 1)`,
	).Scan(&visible); err != nil {
		return 0, 0, fmt.Errorf("toss universe: verify: %w", err)
	}
	if visible < minUniverse {
		return 0, 0, fmt.Errorf(
			"toss universe: 반영 후 보이는 종목이 %d건뿐이다 (유니버스 %d) — 롤백한다",
			visible, len(symbols))
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("toss universe: commit: %w", err)
	}
	return tradable, hidden, nil
}

// UniverseCoverage reports how the flag is distributed, for the batch message.
func (r *TossUniverseRepo) UniverseCoverage(ctx context.Context) (tradable, hidden, unknown int, err error) {
	if r == nil || r.db == nil {
		return 0, 0, 0, nil
	}
	err = r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(toss_tradable = 1), 0),
		       COALESCE(SUM(toss_tradable = 0), 0),
		       COALESCE(SUM(toss_tradable IS NULL), 0)
		  FROM stocks WHERE status = 'NORMAL'`).Scan(&tradable, &hidden, &unknown)
	return tradable, hidden, unknown, err
}

// MigrateTossTradableIfNeeded adds the columns on an existing database.
func MigrateTossTradableIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	has, err := columnExists(ctx, db, "stocks", "toss_tradable")
	if err != nil || has {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "069_toss_tradable.sql", "토스 유니버스 플래그")
}

/*
UpdateLocalNameIfChanged — 한국어 종목명이 **실제로 달라졌을 때만** 쓴다.

돌아온 값은 바뀐 행 수(0 또는 1)다.

`WHERE name <> ?`가 이 함수의 전부다. 13,000행을 매일 전부 UPDATE 하면 운영 DB가
그만큼의 쓰기를 하고(1 ECPU · 스토리지 29 MiB/s), `updated_at`이 매일 움직여
「언제 실제로 바뀌었나」를 잃는다. 이름이 바뀌는 일은 드물고, 바뀔 때가 바로 뉴스
별칭이 틀려 있던 때다.

이 함수를 StockRepo 가 아니라 이 파일에 둔 이유: 유니버스 잡만 쓰는 좁은 쓰기이고,
`stocks.go`의 Upsert 계열(전체 컬럼을 덮는다)과 섞이면 「이름만 고치려다 다른 칸을
지우는」 실수가 생긴다.
*/
func (r *TossUniverseRepo) UpdateLocalNameIfChanged(ctx context.Context, symbol, name string) (int, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	name = strings.TrimSpace(name)
	if symbol == "" || name == "" {
		return 0, nil
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE stocks SET name = ? WHERE symbol = ? AND name <> ?`,
		name, symbol, name)
	if err != nil {
		return 0, fmt.Errorf("rename %s: %w", symbol, err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
