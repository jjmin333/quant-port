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

type NewsPollSettingsRepo struct {
	db *sql.DB
}

func NewNewsPollSettingsRepo(db *sql.DB) *NewsPollSettingsRepo {
	return &NewsPollSettingsRepo{db: db}
}

func (r *NewsPollSettingsRepo) Get(ctx context.Context) (model.NewsPollSettings, error) {
	if r == nil || r.db == nil {
		return model.DefaultNewsPollSettings(), nil
	}
	hasPollMinutes, err := columnExists(ctx, r.db, "news_poll_settings", "domestic_kakao_poll_minutes")
	if err != nil {
		return model.NewsPollSettings{}, err
	}
	query := `
		SELECT domestic_naver_keywords_per_tick, domestic_kakao_keywords_per_tick,
		       us_google_keywords_per_tick, updated_at
		FROM news_poll_settings WHERE id = 1
	`
	if hasPollMinutes {
		query = `
			SELECT domestic_naver_keywords_per_tick, domestic_kakao_keywords_per_tick,
			       domestic_kakao_poll_minutes, us_google_keywords_per_tick, updated_at
			FROM news_poll_settings WHERE id = 1
		`
	}
	row := r.db.QueryRowContext(ctx, query)
	var s model.NewsPollSettings
	if hasPollMinutes {
		if err := row.Scan(
			&s.DomesticNaverKeywordsPerTick,
			&s.DomesticKakaoKeywordsPerTick,
			&s.DomesticKakaoPollMinutes,
			&s.USGoogleKeywordsPerTick,
			&s.UpdatedAt,
		); err != nil {
			if err == sql.ErrNoRows {
				return model.DefaultNewsPollSettings(), nil
			}
			return model.NewsPollSettings{}, err
		}
	} else if err := row.Scan(
		&s.DomesticNaverKeywordsPerTick,
		&s.DomesticKakaoKeywordsPerTick,
		&s.USGoogleKeywordsPerTick,
		&s.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return model.DefaultNewsPollSettings(), nil
		}
		return model.NewsPollSettings{}, err
	}
	return normalizeNewsPollSettings(s), nil
}

func (r *NewsPollSettingsRepo) Update(ctx context.Context, input model.NewsPollSettings) (model.NewsPollSettings, error) {
	input = normalizeNewsPollSettings(input)
	hasPollMinutes, err := columnExists(ctx, r.db, "news_poll_settings", "domestic_kakao_poll_minutes")
	if err != nil {
		return model.NewsPollSettings{}, err
	}
	if hasPollMinutes {
		_, err = r.db.ExecContext(ctx, `
			UPDATE news_poll_settings
			SET domestic_naver_keywords_per_tick = ?,
			    domestic_kakao_keywords_per_tick = ?,
			    domestic_kakao_poll_minutes = ?,
			    us_google_keywords_per_tick = ?
			WHERE id = 1
		`, input.DomesticNaverKeywordsPerTick, input.DomesticKakaoKeywordsPerTick,
			input.DomesticKakaoPollMinutes, input.USGoogleKeywordsPerTick)
	} else {
		_, err = r.db.ExecContext(ctx, `
			UPDATE news_poll_settings
			SET domestic_naver_keywords_per_tick = ?,
			    domestic_kakao_keywords_per_tick = ?,
			    us_google_keywords_per_tick = ?
			WHERE id = 1
		`, input.DomesticNaverKeywordsPerTick, input.DomesticKakaoKeywordsPerTick, input.USGoogleKeywordsPerTick)
	}
	if err != nil {
		return model.NewsPollSettings{}, err
	}
	return r.Get(ctx)
}

func normalizeNewsPollSettings(s model.NewsPollSettings) model.NewsPollSettings {
	def := model.DefaultNewsPollSettings()
	if s.DomesticNaverKeywordsPerTick <= 0 {
		s.DomesticNaverKeywordsPerTick = def.DomesticNaverKeywordsPerTick
	}
	if s.DomesticNaverKeywordsPerTick > 30 {
		s.DomesticNaverKeywordsPerTick = 30
	}
	if s.DomesticKakaoKeywordsPerTick <= 0 {
		s.DomesticKakaoKeywordsPerTick = def.DomesticKakaoKeywordsPerTick
	}
	if s.DomesticKakaoKeywordsPerTick > 10 {
		s.DomesticKakaoKeywordsPerTick = 10
	}
	if s.DomesticKakaoPollMinutes <= 0 {
		s.DomesticKakaoPollMinutes = def.DomesticKakaoPollMinutes
	}
	if s.DomesticKakaoPollMinutes > 60 {
		s.DomesticKakaoPollMinutes = 60
	}
	if s.USGoogleKeywordsPerTick <= 0 {
		s.USGoogleKeywordsPerTick = def.USGoogleKeywordsPerTick
	}
	if s.USGoogleKeywordsPerTick > 30 {
		s.USGoogleKeywordsPerTick = 30
	}
	return s
}

// MigrateNewsPollSettingsIfNeeded applies 021_news_poll_settings.sql.
func MigrateNewsPollSettingsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "news_poll_settings")
	if err != nil {
		return err
	}
	if !exists {
		path := filepath.Join(migrationsDir, "021_news_poll_settings.sql")
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read 021_news_poll_settings.sql: %w", err)
		}
		for _, stmt := range splitSQLStatements(string(body)) {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("021_news_poll_settings.sql: %w", err)
			}
		}
	}
	return MigrateKakaoPollIntervalIfNeeded(ctx, db, migrationsDir)
}

// MigrateKakaoPollIntervalIfNeeded applies 022_kakao_poll_interval.sql.
func MigrateKakaoPollIntervalIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	hasCol, err := columnExists(ctx, db, "news_poll_settings", "domestic_kakao_poll_minutes")
	if err != nil {
		return err
	}
	if hasCol {
		return nil
	}
	path := filepath.Join(migrationsDir, "022_kakao_poll_interval.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 022_kakao_poll_interval.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if isIgnorableAlterErr(err) || strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				continue
			}
			return fmt.Errorf("022_kakao_poll_interval.sql: %w", err)
		}
	}
	return nil
}
