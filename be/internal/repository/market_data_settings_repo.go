package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/enact/quant-port/be/internal/model"
)

type MarketDataSettingsRepo struct {
	db *sql.DB
}

func NewMarketDataSettingsRepo(db *sql.DB) *MarketDataSettingsRepo {
	return &MarketDataSettingsRepo{db: db}
}

func (r *MarketDataSettingsRepo) Get(ctx context.Context) (model.MarketDataSettings, error) {
	if r == nil || r.db == nil {
		return model.DefaultMarketDataSettings(), nil
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT rankings_poll_realtime_minutes, rankings_poll_short_minutes, rankings_poll_long_minutes,
		       quote_overlay_poll_seconds, rankings_client_poll_seconds,
		       shares_poll_hours, fundamentals_poll_hours, updated_at
		FROM market_data_settings WHERE id = 1
	`)
	var s model.MarketDataSettings
	if err := row.Scan(
		&s.RankingsPollRealtimeMinutes,
		&s.RankingsPollShortMinutes,
		&s.RankingsPollLongMinutes,
		&s.QuoteOverlayPollSeconds,
		&s.RankingsClientPollSeconds,
		&s.SharesPollHours,
		&s.FundamentalsPollHours,
		&s.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return model.DefaultMarketDataSettings(), nil
		}
		return model.MarketDataSettings{}, err
	}
	return normalizeMarketDataSettings(s), nil
}

func (r *MarketDataSettingsRepo) Update(ctx context.Context, input model.MarketDataSettings) (model.MarketDataSettings, error) {
	input = normalizeMarketDataSettings(input)
	_, err := r.db.ExecContext(ctx, `
		UPDATE market_data_settings
		SET rankings_poll_realtime_minutes = ?,
		    rankings_poll_short_minutes = ?,
		    rankings_poll_long_minutes = ?,
		    quote_overlay_poll_seconds = ?,
		    rankings_client_poll_seconds = ?,
		    shares_poll_hours = ?,
		    fundamentals_poll_hours = ?
		WHERE id = 1
	`,
		input.RankingsPollRealtimeMinutes,
		input.RankingsPollShortMinutes,
		input.RankingsPollLongMinutes,
		input.QuoteOverlayPollSeconds,
		input.RankingsClientPollSeconds,
		input.SharesPollHours,
		input.FundamentalsPollHours,
	)
	if err != nil {
		return model.MarketDataSettings{}, err
	}
	return r.Get(ctx)
}

func normalizeMarketDataSettings(s model.MarketDataSettings) model.MarketDataSettings {
	def := model.DefaultMarketDataSettings()
	clampInt := func(v, min, max, fallback int) int {
		if v <= 0 {
			return fallback
		}
		if v < min {
			return min
		}
		if v > max {
			return max
		}
		return v
	}
	s.RankingsPollRealtimeMinutes = clampInt(s.RankingsPollRealtimeMinutes, 1, 60, def.RankingsPollRealtimeMinutes)
	s.RankingsPollShortMinutes = clampInt(s.RankingsPollShortMinutes, 5, 120, def.RankingsPollShortMinutes)
	s.RankingsPollLongMinutes = clampInt(s.RankingsPollLongMinutes, 15, 240, def.RankingsPollLongMinutes)
	s.QuoteOverlayPollSeconds = clampInt(s.QuoteOverlayPollSeconds, 1, 60, def.QuoteOverlayPollSeconds)
	s.RankingsClientPollSeconds = clampInt(s.RankingsClientPollSeconds, 10, 600, def.RankingsClientPollSeconds)
	s.SharesPollHours = clampInt(s.SharesPollHours, 1, 168, def.SharesPollHours)
	s.FundamentalsPollHours = clampInt(s.FundamentalsPollHours, 1, 168, def.FundamentalsPollHours)
	return s
}

// MigrateMarketDataSettingsIfNeeded applies 025_market_data_settings.sql.
func MigrateMarketDataSettingsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "market_data_settings")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	path := filepath.Join(migrationsDir, "025_market_data_settings.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 025_market_data_settings.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("025_market_data_settings.sql: %w", err)
		}
	}
	return nil
}

// MigrateRankingsRealtimeDefaultIfNeeded bumps rankings_poll_realtime_minutes
// from the old default (1) to the new default (5) for rows created before
// that default changed (see DefaultMarketDataSettings). Conditioned on the
// OLD default value so an admin who already customized this away from 1 is
// left alone.
func MigrateRankingsRealtimeDefaultIfNeeded(ctx context.Context, db *sql.DB) error {
	exists, err := tableExists(ctx, db, "market_data_settings")
	if err != nil || !exists {
		return err
	}
	_, err = db.ExecContext(ctx,
		`UPDATE market_data_settings SET rankings_poll_realtime_minutes = 5 WHERE id = 1 AND rankings_poll_realtime_minutes = 1`,
	)
	return err
}
