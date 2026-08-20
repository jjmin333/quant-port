package repository

import (
	"context"
	"database/sql"
	"fmt"
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
const (
	defaultMaxOpenConns = 50
	defaultMaxIdleConns = 25
	connMaxLifetime     = 30 * time.Minute
	connMaxIdleTime     = 5 * time.Minute
)

type MySQL struct {
	db *sql.DB
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
