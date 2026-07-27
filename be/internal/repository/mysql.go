package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type MySQL struct {
	db *sql.DB
}

func NewMySQL(dsn string) (*MySQL, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	return &MySQL{db: db}, nil
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
