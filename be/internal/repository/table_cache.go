package repository

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// tableMissingRecheck is how long a negative existence result is trusted.
// Migrations only ever create tables, so a positive result is cached forever.
const tableMissingRecheck = 30 * time.Second

// tableExistsCache memoizes an information_schema lookup. HasTable guards are
// on nearly every quotes/candles read and write path — several inside
// per-symbol loops — and information_schema queries are far from free, so
// re-asking MySQL on every call was measurable overhead.
type tableExistsCache struct {
	table string

	exists  atomic.Bool
	checkMu sync.Mutex
	checkAt time.Time
}

func newTableExistsCache(table string) *tableExistsCache {
	return &tableExistsCache{table: table}
}

func (c *tableExistsCache) Has(ctx context.Context, db *sql.DB) bool {
	if c.exists.Load() {
		return true
	}

	c.checkMu.Lock()
	defer c.checkMu.Unlock()
	if c.exists.Load() {
		return true
	}
	if !c.checkAt.IsZero() && time.Since(c.checkAt) < tableMissingRecheck {
		return false
	}
	c.checkAt = time.Now()

	var name string
	err := db.QueryRowContext(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = ?
	`, c.table).Scan(&name)
	if err != nil || strings.TrimSpace(name) == "" {
		return false
	}
	c.exists.Store(true)
	return true
}
