package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

/*
기사별 감성·사건 라벨. 표를 왜 두는지는 `migrations/079_news_article_sentiment.sql`.
*/

// NewsSentimentRepo owns news_article_sentiment.
type NewsSentimentRepo struct {
	db *sql.DB
}

func NewNewsSentimentRepo(db *sql.DB) *NewsSentimentRepo {
	return &NewsSentimentRepo{db: db}
}

// UnlabeledArticle is one article waiting for a label.
type UnlabeledArticle struct {
	ID       string
	Market   string // domestic | us
	Headline string
}

/*
Unlabeled — `since` 이후 **종목이 붙은** 기사 중 아직 라벨이 없는 것, 최신순.

종목이 없는 기사(시장 전반)는 심리 축에 들어갈 자리가 없어 매기지 않는다. 한도를
넘으면 최신이 먼저다 — 오늘 기사가 다음 점수 배치에 들어가는 것이 오래된 기사를
채우는 것보다 급하다.
*/
func (r *NewsSentimentRepo) Unlabeled(ctx context.Context, since time.Time, limit int) ([]UnlabeledArticle, error) {
	if r == nil || r.db == nil || limit <= 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT n.id, n.market, n.headline
		  FROM news_articles n
		  LEFT JOIN news_article_sentiment s ON s.article_id = n.id
		 WHERE n.published_at >= ?
		   AND s.article_id IS NULL
		   AND JSON_LENGTH(n.symbols_json) > 0
		 ORDER BY n.published_at DESC
		 LIMIT ?`, since, limit)
	if err != nil {
		return nil, fmt.Errorf("news sentiment unlabeled: %w", err)
	}
	defer rows.Close()
	var out []UnlabeledArticle
	for rows.Next() {
		var a UnlabeledArticle
		if err := rows.Scan(&a.ID, &a.Market, &a.Headline); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// NewsSentimentLabel is one stored label.
type NewsSentimentLabel struct {
	ArticleID string
	Score     float64
	Label     string
	EventType string // "" → NULL
	Model     string
}

// UpsertLabels writes labels in one statement per chunk.
func (r *NewsSentimentRepo) UpsertLabels(ctx context.Context, labels []NewsSentimentLabel) error {
	if r == nil || r.db == nil || len(labels) == 0 {
		return nil
	}
	const chunk = 200
	for start := 0; start < len(labels); start += chunk {
		end := min(start+chunk, len(labels))
		part := labels[start:end]
		marks := make([]string, 0, len(part))
		args := make([]any, 0, len(part)*5)
		for _, l := range part {
			marks = append(marks, "(?, ?, ?, ?, ?)")
			var event any
			if l.EventType != "" {
				event = l.EventType
			}
			args = append(args, l.ArticleID, l.Score, l.Label, event, l.Model)
		}
		if _, err := r.db.ExecContext(ctx, `
			INSERT INTO news_article_sentiment (article_id, score, label, event_type, model)
			VALUES `+strings.Join(marks, ",")+`
			ON DUPLICATE KEY UPDATE
			  score = VALUES(score), label = VALUES(label),
			  event_type = VALUES(event_type), model = VALUES(model),
			  labeled_at = CURRENT_TIMESTAMP(3)`, args...); err != nil {
			return fmt.Errorf("news sentiment upsert: %w", err)
		}
	}
	return nil
}

// SymbolNewsSentiment is one symbol's labelled-article tally over a window.
type SymbolNewsSentiment struct {
	Total    int
	Positive int
	Negative int
	Neutral  int
	// AvgScore is the mean model score, −1..1.
	AvgScore float64
}

/*
SymbolSentimentSince — 종목별 라벨 집계 한 번에.

`symbols_json`을 펼쳐 종목마다 센다(JSON_TABLE). 2주 창 약 6만 기사 × 평균 1.3종목에서
실측 2초 안팎 — 점수 배치가 하루 한 번 부르는 값이다.
*/
func (r *NewsSentimentRepo) SymbolSentimentSince(ctx context.Context, since time.Time) (map[string]SymbolNewsSentiment, error) {
	out := map[string]SymbolNewsSentiment{}
	if r == nil || r.db == nil {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT jt.sym,
		       COUNT(*),
		       SUM(s.label = 'positive'),
		       SUM(s.label = 'negative'),
		       SUM(s.label = 'neutral'),
		       AVG(s.score)
		  FROM news_articles n
		  JOIN news_article_sentiment s ON s.article_id = n.id
		  JOIN JSON_TABLE(n.symbols_json, '$[*]' COLUMNS (sym VARCHAR(32) PATH '$')) jt
		 WHERE n.published_at >= ?
		 GROUP BY jt.sym`, since)
	if err != nil {
		return nil, fmt.Errorf("news sentiment by symbol: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			sym           string
			v             SymbolNewsSentiment
			pos, neg, neu sql.NullInt64
			avg           sql.NullFloat64
		)
		if err := rows.Scan(&sym, &v.Total, &pos, &neg, &neu, &avg); err != nil {
			return nil, err
		}
		v.Positive, v.Negative, v.Neutral = int(pos.Int64), int(neg.Int64), int(neu.Int64)
		v.AvgScore = avg.Float64
		out[strings.ToUpper(sym)] = v
	}
	return out, rows.Err()
}

// MigrateNewsSentimentIfNeeded creates the table on an existing database.
func MigrateNewsSentimentIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "news_article_sentiment")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir,
		"079_news_article_sentiment.sql", "기사별 감성·사건 라벨")
}
