package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

/*
부팅 시 InnoDB 버퍼 풀이 데이터 규모에 비해 너무 작은지 확인한다.

# 왜 필요한가 (2026-08-22에 겪은 것)

로컬 MySQL이 **기본값 128MB**로 돌고 있었는데 `candle_bars`가 **1,915MB**였다.
전체를 훑는 배치 쿼리가 버퍼 풀에 들어가질 못하니 InnoDB가 빈 블록을 찾다
실패했고(`Difficult to find free blocks in the buffer pool`), 결국 서버가
**두 번 교착**했다 — CPU도 IO도 0인데 테이블 접근만 영원히 멈추는 상태라
원인을 찾는 데 오래 걸렸다.

버퍼 풀을 4GB로 올리자 같은 쿼리가 **15분+ 실패 → 47초 성공**, 단순 COUNT는
20초 타임아웃 → 20ms 안쪽이 됐다.

# 왜 로그 한 줄인가

고치는 건 우리가 못 한다(`my.ini` + 서비스 재시작은 서버 운영자의 몫). 대신
**증상이 나오기 전에 눈에 띄게** 만든다. 관리형 DB(RDS·Cloud SQL)는 인스턴스
메모리의 약 75%로 자동 설정하므로 이 경고가 뜰 일이 없다 — 뜬다면 자체 관리
인스턴스에서 기본값을 안 바꾼 것이다.
*/

// bufferPoolWarnRatio — 가장 큰 테이블 대비 이 비율보다 작으면 경고한다.
//
// 0.5인 것은 전체 스캔이 반드시 죽는 선이 아니라 **위험해지기 시작하는 선**이라
// 그렇다. 실측 사고는 0.07(128MB / 1,915MB)이었다.
const bufferPoolWarnRatio = 0.5

// CheckInnoDBBufferPool logs the buffer pool size against the largest table.
//
// 실패해도 부팅을 막지 않는다 — 진단용이지 요구사항이 아니다.
func CheckInnoDBBufferPool(ctx context.Context, db *sql.DB) {
	if db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var poolBytes int64
	if err := db.QueryRowContext(ctx, `SELECT @@innodb_buffer_pool_size`).Scan(&poolBytes); err != nil {
		return
	}

	var table sql.NullString
	var tableBytes sql.NullInt64
	err := db.QueryRowContext(ctx, `
		SELECT table_name, data_length + index_length
		  FROM information_schema.tables
		 WHERE table_schema = DATABASE() AND engine = 'InnoDB'
		 ORDER BY data_length + index_length DESC
		 LIMIT 1`).Scan(&table, &tableBytes)
	if err != nil || !tableBytes.Valid || tableBytes.Int64 <= 0 {
		log.Printf("mysql: innodb_buffer_pool_size = %s", humanBytes(poolBytes))
		return
	}

	if float64(poolBytes) >= float64(tableBytes.Int64)*bufferPoolWarnRatio {
		log.Printf("mysql: innodb_buffer_pool_size = %s (최대 테이블 %s = %s) — ok",
			humanBytes(poolBytes), table.String, humanBytes(tableBytes.Int64))
		return
	}

	log.Printf("mysql: ⚠ innodb_buffer_pool_size = %s 인데 최대 테이블 %s 가 %s 다. "+
		"전체를 훑는 배치 쿼리가 버퍼 풀에 안 들어가면 InnoDB가 빈 블록을 못 찾아 "+
		"서버가 멈출 수 있다. my.ini의 innodb_buffer_pool_size를 올릴 것 "+
		"(관리형 DB는 인스턴스 메모리의 약 75%%로 자동 설정된다).",
		humanBytes(poolBytes), table.String, humanBytes(tableBytes.Int64))
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fGB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%dMB", n/(1<<20))
	default:
		return fmt.Sprintf("%dKB", n/(1<<10))
	}
}
