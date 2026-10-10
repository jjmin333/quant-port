package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

/*
리포트 「AI 검토 노트」 저장소 (2026-09-27 사용자 결정 C ④ — migrations/090_research_reviews.sql).

사실표에는 **리포트의 주장(제목·방향·목표가·본문 앞부분)** 과 **우리 데이터(발행 때와 지금의
점수·가격, 이후 기사·공시 제목)** 를 나란히 넣는다. 본문은 앞 1,800자만 — 긴 리포트 하나가
하루 몫을 먹지 않게.
*/

type ResearchReviewRepo struct{ db *sql.DB }

func NewResearchReviewRepo(db *sql.DB) *ResearchReviewRepo { return &ResearchReviewRepo{db: db} }

// ResearchReviewTarget is one public call that needs (re)review.
type ResearchReviewTarget struct {
	CallID  int64
	BodyRev int
}

// Pending lists public, live calls from the last 30 days with no review for the current body revision.
func (r *ResearchReviewRepo) Pending(ctx context.Context, since time.Time, limit int) ([]ResearchReviewTarget, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.body_rev FROM research_calls c
		  LEFT JOIN research_reviews v ON v.call_id = c.id
		 WHERE c.visibility = 'public' AND c.hidden_at IS NULL AND c.retracted_at IS NULL
		   AND c.topic = 'stock' -- 분석 리포트(부동산·금융상품)는 검토 대상이 아니다 — 검토 문항이 목표가·근거 기준이다
		   AND c.published_at >= ?
		   AND (v.call_id IS NULL OR v.body_rev < c.body_rev
		        -- 규칙 노트(LLM 한도·밤 503)는 6시간 뒤 AI로 다시 시도한다.
		        OR (v.source = 'rule' AND v.generated_at < DATE_SUB(NOW(3), INTERVAL 6 HOUR)))
		 ORDER BY v.call_id IS NOT NULL, c.published_at ASC LIMIT ?`, since, limit)
	if err != nil {
		return nil, fmt.Errorf("research review pending: %w", err)
	}
	defer rows.Close()
	var out []ResearchReviewTarget
	for rows.Next() {
		var t ResearchReviewTarget
		if err := rows.Scan(&t.CallID, &t.BodyRev); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ResearchReviewFacts is what the model sees.
type ResearchReviewFacts struct {
	Title          string             `json:"title"`
	Symbol         string             `json:"symbol"`
	Name           string             `json:"name"`
	Direction      string             `json:"direction"`
	PublishedDate  string             `json:"publishedDate"`
	BasePrice      float64            `json:"basePrice"`
	TargetPrice    *float64           `json:"targetPrice,omitempty"`
	StopPrice      *float64           `json:"stopPrice,omitempty"`
	HorizonDays    int                `json:"horizonDays"`
	Body           string             `json:"reportBody"`
	ScoreAtPublish *int               `json:"aiScoreAtPublish,omitempty"`
	ScoreNow       *int               `json:"aiScoreNow,omitempty"`
	AxesNow        map[string]float64 `json:"axesNow,omitempty"`
	PriceNow       *float64           `json:"priceNow,omitempty"`
	ReturnSincePub *float64           `json:"returnSincePublishPct,omitempty"`
	PER            *float64           `json:"perAtPublish,omitempty"`
	PBR            *float64           `json:"pbrAtPublish,omitempty"`
	HeadlinesSince []string           `json:"headlinesSincePublish,omitempty"`
	FilingsSince   []string           `json:"disclosuresSincePublish,omitempty"`
	BodyRev        int                `json:"-"`
}

const researchReviewBodyMax = 1800

// Facts builds the fact sheet for one call.
func (r *ResearchReviewRepo) Facts(ctx context.Context, callID int64) (*ResearchReviewFacts, error) {
	f := &ResearchReviewFacts{}
	var evidence []byte
	var pub time.Time
	var target, stop sql.NullFloat64
	var score sql.NullInt64
	var asset string
	err := r.db.QueryRowContext(ctx, `
		SELECT title, symbol, symbol_name, asset_type, direction, published_at, base_price, target_price, stop_price,
		       horizon_days, body_md, score_at_pub, evidence, body_rev
		  FROM research_calls WHERE id = ?`, callID).Scan(&f.Title, &f.Symbol, &f.Name, &asset, &f.Direction, &pub,
		&f.BasePrice, &target, &stop, &f.HorizonDays, &f.Body, &score, &evidence, &f.BodyRev)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrResearchNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("research review facts: %w", err)
	}
	f.PublishedDate = pub.In(time.FixedZone("KST", 9*3600)).Format("2006-01-02")
	f.TargetPrice, f.StopPrice = nullF(target), nullF(stop)
	if score.Valid {
		v := int(score.Int64)
		f.ScoreAtPublish = &v
	}
	f.Body = clipRunes(strings.TrimSpace(f.Body), researchReviewBodyMax)
	var ev struct {
		PER *float64 `json:"per"`
		PBR *float64 `json:"pbr"`
	}
	if json.Unmarshal(evidence, &ev) == nil {
		f.PER, f.PBR = ev.PER, ev.PBR
	}

	var sc sql.NullInt64
	var bd []byte
	if err := r.db.QueryRowContext(ctx, `
		SELECT score, breakdown FROM stock_scores WHERE symbol = ? ORDER BY trade_date DESC LIMIT 1`, f.Symbol).Scan(&sc, &bd); err == nil {
		if sc.Valid {
			v := int(sc.Int64)
			f.ScoreNow = &v
		}
		f.AxesNow = numericTopLevel(bd)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var closeNow float64
	if err := r.db.QueryRowContext(ctx, `
		SELECT close_price FROM stock_daily_closes WHERE symbol = ? ORDER BY trade_date DESC LIMIT 1`, f.Symbol).Scan(&closeNow); err == nil && closeNow > 0 {
		f.PriceNow = &closeNow
		if f.BasePrice > 0 {
			ret := round1((closeNow - f.BasePrice) / f.BasePrice * 100)
			f.ReturnSincePub = &ret
		}
	}

	newsMarket := "domestic"
	if asset != "domestic" {
		newsMarket = "us"
	}
	nrow, err := r.db.QueryContext(ctx, `
		SELECT COALESCE(NULLIF(headline_ko, ''), headline)
		  FROM news_articles FORCE INDEX (idx_news_market_published)
		 WHERE market = ? AND published_at >= ? AND JSON_CONTAINS(symbols_json, JSON_QUOTE(?))
		 ORDER BY published_at DESC LIMIT 5`, newsMarket, pub, f.Symbol)
	if err != nil {
		return nil, fmt.Errorf("research review news: %w", err)
	}
	for nrow.Next() {
		var h string
		if err := nrow.Scan(&h); err != nil {
			nrow.Close()
			return nil, err
		}
		f.HeadlinesSince = append(f.HeadlinesSince, clipRunes(h, 90))
	}
	nrow.Close()

	drow, err := r.db.QueryContext(ctx, `
		SELECT title FROM stock_disclosures WHERE symbol = ? AND receipt_date >= ? ORDER BY receipt_date DESC LIMIT 3`,
		f.Symbol, f.PublishedDate)
	if err != nil {
		return nil, fmt.Errorf("research review disclosures: %w", err)
	}
	for drow.Next() {
		var t string
		if err := drow.Scan(&t); err != nil {
			drow.Close()
			return nil, err
		}
		f.FilingsSince = append(f.FilingsSince, clipRunes(t, 90))
	}
	drow.Close()
	return f, nil
}

func (r *ResearchReviewRepo) Upsert(ctx context.Context, callID int64, bodyRev int, body []byte, source, model string, tokens int64) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO research_reviews (call_id, body_rev, body, source, model, tokens, generated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP(3))
		ON DUPLICATE KEY UPDATE body_rev = VALUES(body_rev), body = VALUES(body), source = VALUES(source),
		  model = VALUES(model), tokens = VALUES(tokens), generated_at = CURRENT_TIMESTAMP(3)`,
		callID, bodyRev, string(body), source, model, tokens)
	return err
}

// ResearchReviewRow is the stored note as served.
type ResearchReviewRow struct {
	CallID      int64           `json:"callId"`
	BodyRev     int             `json:"bodyRev"`
	Body        json.RawMessage `json:"body"`
	Source      string          `json:"source"`
	GeneratedAt time.Time       `json:"generatedAt"`
}

// Get returns the note for a public, live call; (nil, nil) when there is none.
func (r *ResearchReviewRepo) Get(ctx context.Context, callID int64) (*ResearchReviewRow, error) {
	var row ResearchReviewRow
	var body []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT v.call_id, v.body_rev, v.body, v.source, v.generated_at
		  FROM research_reviews v JOIN research_calls c ON c.id = v.call_id
		 WHERE v.call_id = ? AND c.visibility = 'public' AND c.hidden_at IS NULL AND c.retracted_at IS NULL`, callID).
		Scan(&row.CallID, &row.BodyRev, &body, &row.Source, &row.GeneratedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	row.Body = json.RawMessage(body)
	return &row, nil
}

// MigrateResearchReviewsIfNeeded creates research_reviews on an existing database.
func MigrateResearchReviewsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "research_reviews")
	if err != nil || exists {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "090_research_reviews.sql", "리포트 AI 검토")
}
