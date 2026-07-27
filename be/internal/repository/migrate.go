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
	"broker_accounts",
	"watchlist_items",
	"portfolio_snapshots",
	"candle_bars",
	"stocks",
	"stock_fundamentals",
}

// EnsureSchema connects via MYSQL_DSN, verifies required tables exist,
// and runs migrations/*.sql only when any table is missing.
func EnsureSchema(ctx context.Context, db *sql.DB, migrationsDir string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
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
		if err := MigrateCandleBarsV2IfNeeded(ctx, db); err != nil {
			return err
		}
		log.Println("mysql: schema ok (all required tables present)")
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
	if err := MigrateCandleBarsV2IfNeeded(ctx, db); err != nil {
		return err
	}

	log.Println("mysql: schema ready")
	return nil
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
