package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

// maxPilotSymbols bounds the always-include symbol list. These run on top of
// (and ahead of) the ranking-derived targets, so a long list eats the daily
// budget before the 레이더 list gets any of it — but it no longer has to stay
// tiny, since DailyCap and the quota reserves are what actually protect the
// free tier now.
const maxPilotSymbols = 50

// Bounds on the admin-editable batch knobs. A daily cap above the provider's
// own request quota is meaningless, and a zero cap would silently disable the
// feature in a way that looks like a bug.
const (
	minDailyCap  = 1
	maxDailyCap  = 1000
	maxReserve   = 100_000
	maxTokenHold = 100_000_000
)

type LLMSettingsRepo struct {
	db *sql.DB
}

func NewLLMSettingsRepo(db *sql.DB) *LLMSettingsRepo {
	return &LLMSettingsRepo{db: db}
}

func (r *LLMSettingsRepo) Get(ctx context.Context) (model.LLMSettings, error) {
	if r == nil || r.db == nil {
		return model.DefaultLLMSettings(), nil
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT pilot_symbols, target_mode, daily_cap, quota_reserve, token_reserve, updated_at
		FROM llm_settings WHERE id = 1
	`)
	var raw string
	var s model.LLMSettings
	if err := row.Scan(&raw, &s.TargetMode, &s.DailyCap, &s.QuotaReserve, &s.TokenReserve, &s.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return model.DefaultLLMSettings(), nil
		}
		return model.LLMSettings{}, err
	}
	s.PilotSymbols = parseSymbolList(raw)
	s.TargetMode = s.TargetModeOrDefault()
	return s, nil
}

// Update replaces every admin-editable field. The batch knobs are clamped
// here rather than at the handler, so a direct DB edit or a future caller
// can't push the runner outside its safe range either.
func (r *LLMSettingsRepo) Update(ctx context.Context, in model.LLMSettings) (model.LLMSettings, error) {
	normalized := normalizeSymbolList(in.PilotSymbols)
	mode := in.TargetModeOrDefault()
	_, err := r.db.ExecContext(ctx, `
		UPDATE llm_settings
		SET pilot_symbols = ?, target_mode = ?, daily_cap = ?, quota_reserve = ?, token_reserve = ?
		WHERE id = 1
	`,
		strings.Join(normalized, ","),
		mode,
		clampInt(in.DailyCap, minDailyCap, maxDailyCap),
		clampInt(in.QuotaReserve, 0, maxReserve),
		clampInt64(in.TokenReserve, 0, maxTokenHold),
	)
	if err != nil {
		return model.LLMSettings{}, err
	}
	return r.Get(ctx)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampInt64(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func parseSymbolList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToUpper(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func normalizeSymbolList(symbols []string) []string {
	seen := make(map[string]bool, len(symbols))
	out := make([]string, 0, len(symbols))
	for _, s := range symbols {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	if len(out) > maxPilotSymbols {
		out = out[:maxPilotSymbols]
	}
	return out
}

// MigrateLLMSettingsIfNeeded applies 027_llm_settings.sql.
func MigrateLLMSettingsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "llm_settings")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	path := filepath.Join(migrationsDir, "027_llm_settings.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 027_llm_settings.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("027_llm_settings.sql: %w", err)
		}
	}
	return nil
}

// MigrateLLMRefreshIntervalIfNeeded applies 028_llm_settings_refresh_interval.sql.
func MigrateLLMRefreshIntervalIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	hasCol, err := columnExists(ctx, db, "llm_settings", "refresh_interval_hours")
	if err != nil {
		return err
	}
	if hasCol {
		return nil
	}
	path := filepath.Join(migrationsDir, "028_llm_settings_refresh_interval.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 028_llm_settings_refresh_interval.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if isIgnorableAlterErr(err) || strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				continue
			}
			return fmt.Errorf("028_llm_settings_refresh_interval.sql: %w", err)
		}
	}
	return nil
}

// MigrateLLMBatchSettingsIfNeeded applies 036_llm_batch_settings.sql — the
// 마켓 레이더 batch knobs (target_mode/daily_cap/quota_reserve/token_reserve).
func MigrateLLMBatchSettingsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	hasCol, err := columnExists(ctx, db, "llm_settings", "target_mode")
	if err != nil {
		return err
	}
	if hasCol {
		return nil
	}
	path := filepath.Join(migrationsDir, "036_llm_batch_settings.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 036_llm_batch_settings.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if isIgnorableAlterErr(err) || strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				continue
			}
			return fmt.Errorf("036_llm_batch_settings.sql: %w", err)
		}
	}
	return nil
}
