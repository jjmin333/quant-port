package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// NewsBriefingRepo persists generated market briefings. MySQL is the source of
// truth here rather than Redis: a briefing costs an LLM call, and losing the
// history would also lose the continuity the next prompt reads back.
type NewsBriefingRepo struct {
	db *sql.DB
}

func NewNewsBriefingRepo(db *sql.DB) *NewsBriefingRepo {
	return &NewsBriefingRepo{db: db}
}

// briefingPayload is the JSON half of the row — everything that has no reason
// to be its own column.
type briefingPayload struct {
	MoodReason  string                  `json:"moodReason,omitempty"`
	Highlights  []string                `json:"highlights,omitempty"`
	WatchPoints []string                `json:"watchPoints,omitempty"`
	CarriedFrom *time.Time              `json:"carriedFrom,omitempty"`
	Facts       model.NewsBriefingFacts `json:"facts"`
}

func (r *NewsBriefingRepo) Insert(ctx context.Context, b model.NewsMarketBriefing) error {
	if r == nil || r.db == nil {
		return nil
	}
	payload, err := json.Marshal(briefingPayload{
		MoodReason:  b.MoodReason,
		Highlights:  b.Highlights,
		WatchPoints: b.WatchPoints,
		CarriedFrom: b.CarriedFrom,
		Facts:       b.Facts,
	})
	if err != nil {
		return err
	}
	var latest any
	if !b.Facts.LatestPublishedAt.IsZero() {
		latest = b.Facts.LatestPublishedAt
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO news_briefings
			(market, generated_at, source, mood, summary, payload, article_count, latest_published_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, b.Market, b.GeneratedAt, b.Source, b.Mood, b.Summary, string(payload),
		b.Facts.ArticleCount, latest)
	return err
}

// Latest returns the most recent briefing for a market, or nil when none has
// been generated yet.
func (r *NewsBriefingRepo) Latest(ctx context.Context, market string) (*model.NewsMarketBriefing, error) {
	list, err := r.Recent(ctx, market, 1)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return &list[0], nil
}

// Recent returns up to limit briefings, newest first.
func (r *NewsBriefingRepo) Recent(ctx context.Context, market string, limit int) ([]model.NewsMarketBriefing, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT market, generated_at, source, mood, summary, payload, latest_published_at
		FROM news_briefings
		WHERE market = ?
		ORDER BY generated_at DESC, id DESC
		LIMIT ?
	`, market, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.NewsMarketBriefing
	for rows.Next() {
		var (
			b       model.NewsMarketBriefing
			rawJSON string
			latest  sql.NullTime
		)
		if err := rows.Scan(&b.Market, &b.GeneratedAt, &b.Source, &b.Mood, &b.Summary, &rawJSON, &latest); err != nil {
			return nil, err
		}
		var p briefingPayload
		if err := json.Unmarshal([]byte(rawJSON), &p); err == nil {
			b.MoodReason = p.MoodReason
			b.Highlights = p.Highlights
			b.WatchPoints = p.WatchPoints
			b.CarriedFrom = p.CarriedFrom
			b.Facts = p.Facts
		}
		if latest.Valid {
			b.Facts.LatestPublishedAt = latest.Time
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// History returns past briefings compressed for the next prompt — newest
// first, oldest last, so the model reads them as a sequence.
// 이력에서 승계 행은 건너뛴다. 승계 행의 문장은 원본 슬롯의 문장과 **글자
// 그대로 같아서**, 넣으면 세 칸짜리 이력의 한 칸이 중복으로 날아가고 프롬프트의
// 「지난 브리핑 문장을 반복하지 마십시오」와도 어긋난다. 원본이 창 밖으로 밀려나지
// 않도록 넉넉히 읽고 limit에서 끊는다.
func (r *NewsBriefingRepo) History(ctx context.Context, market string, limit, summaryChars int) ([]model.NewsBriefingHistoryEntry, error) {
	if limit <= 0 {
		limit = 3
	}
	recent, err := r.Recent(ctx, market, limit*2+2)
	if err != nil {
		return nil, err
	}
	out := make([]model.NewsBriefingHistoryEntry, 0, limit)
	for _, b := range recent {
		if len(out) >= limit {
			break
		}
		if b.Source == model.BriefingSourceCarry {
			continue
		}
		out = append(out, model.NewsBriefingHistoryEntry{
			GeneratedAt: b.GeneratedAt,
			Mood:        b.Mood,
			Summary:     truncateRunes(b.Summary, summaryChars),
			WatchPoints: b.WatchPoints,
		})
	}
	return out, nil
}

// Prune keeps the newest keep rows per market. History only ever reads the last
// handful, so unbounded growth buys nothing.
func (r *NewsBriefingRepo) Prune(ctx context.Context, market string, keep int) error {
	if r == nil || r.db == nil || keep <= 0 {
		return nil
	}
	var cutoff time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT generated_at FROM news_briefings
		WHERE market = ?
		ORDER BY generated_at DESC, id DESC
		LIMIT 1 OFFSET ?
	`, market, keep).Scan(&cutoff)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`DELETE FROM news_briefings WHERE market = ? AND generated_at <= ?`, market, cutoff)
	return err
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

// MigrateNewsBriefingsIfNeeded applies 042_news_briefings.sql when the table is
// missing.
func MigrateNewsBriefingsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "news_briefings")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	path := filepath.Join(migrationsDir, "042_news_briefings.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 042_news_briefings.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("042_news_briefings.sql: %w", err)
		}
	}
	log.Println("mysql: applied 042_news_briefings.sql")
	return nil
}
