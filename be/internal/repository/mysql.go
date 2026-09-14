package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Pool defaults. The API runs a dozen background pollers (market, indices,
// rankings, news x6, portfolio sync, reference data, insights) alongside user
// requests and SSE streams, so a small pool serialises everything behind
// connection waits. Override with MYSQL_MAX_OPEN_CONNS / MYSQL_MAX_IDLE_CONNS
// when the server's max_connections is tighter.
/*
풀 배정 — 서버 상한을 나눠 쓰는 예산이지 프로세스별 희망값이 아니다.

	api 24 + 워커 8개 × 4 = 56  /  max_connections 151  (37%)
	남는 95가 CLI·재배포 중 겹침·여유다.

**예전에는 api 50 + 워커 8×8 = 114로 76%였다.** 여유가 37뿐이라 CLI 두세 개가
각자 50을 잡으면 즉시 고갈됐고, 실제로 그렇게 터졌다(2026-08-22 「Too many
connections」로 DB 전체 정지). 그래서 CLI 기본값을 4로 낮췄지만, 그건 증상만
누른 것이고 배정 자체가 과했다.

**24인 근거는 실측이다.** `/metrics`의 풀 지표(2026-08-24): `open=5`,
`in_use=0`, **`wait_total=0`**. 최대 사용이 상한의 10%였다. 24면 관측치의
5배 여유이고, 대기가 생기면 `quantport_mysql_wait_total`이 0을 벗어나므로
근거를 갖고 다시 올릴 수 있다.

`[HW]` DB가 **1 ECPU**라 커넥션을 늘려도 처리량이 늘지 않는다 — 동시에 의미 있게
실행되는 쿼리가 2~4개다. 그래서 `max_connections`는 **올리지 않는다.** 방향은
반대여야 한다: 상한 근처에 가지 않게 배정을 줄인다.

**주의**: 이 프로세스는 아직 시세 폴러를 함께 돌린다. 폴러를 별도 컨테이너로
떼어내면 여기서 더 낮출 수 있다(계획의 16). 그 전까지는 24를 유지할 것.
*/
const (
	defaultMaxOpenConns = 24
	defaultMaxIdleConns = 12
	connMaxLifetime     = 30 * time.Minute
	connMaxIdleTime     = 5 * time.Minute
)

// defaultCLIMaxOpenConns — 일회성 CLI(백필·점검)가 잡는 커넥션 상한.
//
// **50은 CLI에 맞지 않는다.** 운영 스택이 이미 서버 상한의 대부분을 쓴다 —
// api 50 + 워커 8×8 = 약 114인데 MySQL 기본 `max_connections`는 151이다.
// 여유가 37뿐이라 CLI 두세 개가 각자 50을 잡으면 **즉시 고갈**되고, 그때는
// 앱도 함께 죽는다(2026-08-22 실측: 「Too many connections」로 DB 전체 정지).
//
// CLI는 대개 순차로 쿼리 한 번씩을 낸다. 4면 충분하고, 부족하면
// MYSQL_MAX_OPEN_CONNS로 올릴 수 있다.
const defaultCLIMaxOpenConns = 4

type MySQL struct {
	db *sql.DB
}

// NewMySQLForCLI opens a pool sized for a one-off command, not for the server.
//
// 운영 스택이 도는 중에도 안전하게 붙기 위한 진입점이다. `NewMySQL`을 그대로
// 쓰면 CLI 하나가 서버 커넥션의 3분의 1을 가져간다.
func NewMySQLForCLI(dsn string) (*MySQL, error) {
	// 환경변수가 명시돼 있으면 그것이 이긴다 — 운영자가 정한 값을 덮지 않는다.
	if os.Getenv("MYSQL_MAX_OPEN_CONNS") == "" {
		_ = os.Setenv("MYSQL_MAX_OPEN_CONNS", strconv.Itoa(defaultCLIMaxOpenConns))
	}
	if os.Getenv("MYSQL_MAX_IDLE_CONNS") == "" {
		_ = os.Setenv("MYSQL_MAX_IDLE_CONNS", strconv.Itoa(defaultCLIMaxOpenConns))
	}
	return NewMySQL(dsn)
}

func NewMySQL(dsn string) (*MySQL, error) {
	db, err := sql.Open("mysql", withDefaultDSNTimeouts(dsn))
	if err != nil {
		return nil, err
	}

	maxOpen := envInt("MYSQL_MAX_OPEN_CONNS", defaultMaxOpenConns)
	maxIdle := envInt("MYSQL_MAX_IDLE_CONNS", defaultMaxIdleConns)
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(connMaxLifetime)
	// Release idle connections that outlive a quiet period so the server does
	// not hold sockets open against wait_timeout.
	db.SetConnMaxIdleTime(connMaxIdleTime)

	return &MySQL{db: db}, nil
}

/*
withDefaultDSNTimeouts — DSN에 연결·읽기·쓰기 타임아웃이 없으면 채워 넣는다.

go-sql-driver의 기본값은 **셋 다 무제한**이다. 그래서 DB가 응답하지 않는 동안
쿼리가 영원히 매달리고, 요청 컨텍스트에 마감이 없으면 그대로 프로세스에 쌓인다.
컨텍스트 마감(middleware.RequestTimeout)이 대부분을 잡아 주지만, 마감이 없는
경로 — 폴러와 배치 워커 — 는 여기서만 보호된다.

이미 값이 있으면 건드리지 않는다. 운영자가 DSN에 적어 둔 것이 이겨야 한다.
*/
func withDefaultDSNTimeouts(dsn string) string {
	defaults := []struct{ key, val string }{
		{"timeout", "5s"},      // 연결
		{"readTimeout", "30s"}, // 쿼리 응답 — 배치의 큰 스캔이 있어 넉넉히 둔다
		{"writeTimeout", "30s"},
		// 세션 타임존을 못박는다. **서버 기본값에 기대지 않는다.**
		//
		// DSN의 loc=Asia/Seoul은 "서버가 돌려주는 문자열은 KST 벽시계"라는 뜻인데,
		// 그 전제는 서버 타임존이 KST일 때만 참이다. 로컬 MySQL은 SYSTEM(=KST)이라
		// 맞아떨어졌지만 관리형 MySQL은 대개 UTC다 — 그대로 옮기면 TIMESTAMP 컬럼
		// (이 스키마에 85개)을 읽는 값이 9시간 밀리고, SQL 안의 NOW()가 KST로 저장된
		// DATETIME(candle_bars.bar_time 등 4개)과 어긋난다. 조용히 틀어져서 늦게 발견된다.
		//
		// +09:00을 쓰는 이유: 명명 타임존('Asia/Seoul')은 서버에 tz 테이블이 적재돼
		// 있어야 하고 관리형에서는 보장되지 않는다. 한국은 서머타임이 없어 오프셋
		// 고정이 무손실이다. 값은 SET 문에 들어가므로 URL 인코딩된 따옴표가 필요하다.
		{"time_zone", url.QueryEscape("'" + envStr("MYSQL_TIME_ZONE", "+09:00") + "'")},
	}

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	var b strings.Builder
	b.WriteString(dsn)
	for _, d := range defaults {
		if strings.Contains(dsn, d.key+"=") {
			continue
		}
		b.WriteString(sep)
		b.WriteString(d.key)
		b.WriteString("=")
		b.WriteString(d.val)
		sep = "&"
	}
	return b.String()
}

func envStr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func (m *MySQL) DB() *sql.DB {
	return m.db
}

func (m *MySQL) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return m.db.PingContext(ctx)
}

func (m *MySQL) Status(ctx context.Context) string {
	if err := m.Ping(ctx); err != nil {
		return fmt.Sprintf("down: %v", err)
	}
	return "ok"
}

func (m *MySQL) Close() error {
	if m.db == nil {
		return nil
	}
	return m.db.Close()
}
