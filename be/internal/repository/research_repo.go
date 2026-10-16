package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

var (
	ErrResearchNotFound = errors.New("research call not found")
	// ErrResearchImmutable guards the frozen half of a call. It should never
	// reach a user — the service never offers those fields for editing — but a
	// future caller reaching for them fails loudly instead of silently
	// rewriting published history.
	ErrResearchImmutable = errors.New("published call values are immutable")
)

type ResearchRepo struct {
	db *sql.DB
}

func NewResearchRepo(db *sql.DB) *ResearchRepo { return &ResearchRepo{db: db} }

const researchCols = `c.id, c.user_id, COALESCE(u.display_name, ''), COALESCE(u.badge, ''), c.topic, c.symbol, c.symbol_name,
	c.asset_type, c.currency, c.title, c.published_at, c.trade_date, c.benchmark_id,
	c.direction, c.base_price, c.target_price, c.stop_price, c.horizon_days, c.conviction,
	c.score_at_pub, c.grade_at_pub, c.contrarian, c.evidence, c.context, c.chart,
	c.body_md, c.body_rev, c.thumbnail_url, c.tags_json, c.visibility, c.status, c.verdict_at, c.verdict_date,
	c.verdict_return, c.verdict_alpha, c.view_count, c.retracted_at, c.hidden_at,
	c.created_at, c.updated_at`

const researchFrom = ` FROM research_calls c LEFT JOIN users u ON u.id = c.user_id`

// ResearchListFilter narrows a call list. Zero values mean "no filter".
type ResearchListFilter struct {
	UserID int64 // 0 = any author
	// Topic — 빈 값은 전체(2026-10-09 요청 9).
	Topic  string
	Symbol string
	Status string
	// Direction filters buy/sell/hold.
	Direction string
	// Contrarian: "1" only contrarian calls, "0" only aligned, "" both.
	Contrarian string
	// PublicOnly restricts to visible, non-hidden calls — the feed's view.
	PublicOnly bool
	// IncludeHidden lets an admin see moderated rows.
	IncludeHidden bool
	Limit         int
	Offset        int
}

func scanResearch(row interface{ Scan(dest ...any) error }) (model.ResearchCall, error) {
	var (
		c            model.ResearchCall
		assetType    string
		currency     string
		publishedAt  time.Time
		tradeDate    time.Time
		direction    string
		targetPrice  sql.NullFloat64
		stopPrice    sql.NullFloat64
		scoreAtPub   sql.NullInt64
		gradeAtPub   sql.NullString
		contrarian   bool
		evidenceJSON []byte
		ctxJSON      []byte
		chartJSON    []byte
		tagsJSON     []byte
		visibility   string
		status       string
		verdictAt    sql.NullTime
		verdictDate  sql.NullTime
		verdictRet   sql.NullFloat64
		verdictAlpha sql.NullFloat64
		retractedAt  sql.NullTime
		hiddenAt     sql.NullTime
		createdAt    time.Time
		updatedAt    time.Time
	)
	if err := row.Scan(
		&c.ID, &c.UserID, &c.AuthorName, &c.AuthorBadge, &c.Topic, &c.Symbol, &c.SymbolName, &assetType, &currency,
		&c.Title, &publishedAt, &tradeDate, &c.BenchmarkID, &direction, &c.BasePrice,
		&targetPrice, &stopPrice, &c.HorizonDays, &c.Conviction, &scoreAtPub, &gradeAtPub,
		&contrarian, &evidenceJSON, &ctxJSON, &chartJSON, &c.BodyMD, &c.BodyRev, &c.ThumbnailURL, &tagsJSON,
		&visibility, &status, &verdictAt, &verdictDate, &verdictRet, &verdictAlpha,
		&c.ViewCount, &retractedAt, &hiddenAt, &createdAt, &updatedAt,
	); err != nil {
		return model.ResearchCall{}, err
	}

	c.AssetType = model.QuoteAssetType(assetType)
	c.Currency = model.QuoteCurrency(currency)
	c.PublishedAt = publishedAt.UTC()
	c.TradeDate = tradeDate.Format(dailyCloseDateLayout)
	c.Direction = model.ResearchDirection(direction)
	c.Contrarian = contrarian
	c.Visibility = model.ResearchVisibility(visibility)
	c.Status = model.ResearchStatus(status)
	c.CreatedAt = createdAt.UTC()
	c.UpdatedAt = updatedAt.UTC()

	if targetPrice.Valid {
		v := targetPrice.Float64
		c.TargetPrice = &v
	}
	if stopPrice.Valid {
		v := stopPrice.Float64
		c.StopPrice = &v
	}
	if scoreAtPub.Valid {
		v := int(scoreAtPub.Int64)
		c.ScoreAtPub = &v
	}
	c.GradeAtPub = gradeAtPub.String
	if verdictAt.Valid {
		t := verdictAt.Time.UTC()
		c.VerdictAt = &t
	}
	if verdictDate.Valid {
		c.VerdictDate = verdictDate.Time.Format(dailyCloseDateLayout)
	}
	if verdictRet.Valid {
		v := verdictRet.Float64
		c.VerdictReturn = &v
	}
	if verdictAlpha.Valid {
		v := verdictAlpha.Float64
		c.VerdictAlpha = &v
	}
	if retractedAt.Valid {
		t := retractedAt.Time.UTC()
		c.RetractedAt = &t
	}
	if hiddenAt.Valid {
		t := hiddenAt.Time.UTC()
		c.HiddenAt = &t
	}

	// A malformed JSON blob must not sink the whole list — the scalar columns
	// are what the list view renders.
	if len(evidenceJSON) > 0 {
		_ = json.Unmarshal(evidenceJSON, &c.Evidence)
	}
	if len(ctxJSON) > 0 {
		_ = json.Unmarshal(ctxJSON, &c.Context)
	}
	if len(chartJSON) > 0 {
		_ = json.Unmarshal(chartJSON, &c.Chart)
	}
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &c.Tags)
	}
	if c.Context.News == nil {
		c.Context.News = []model.ResearchNews{}
	}
	if c.Context.Disclosures == nil {
		c.Context.Disclosures = []model.BriefingEvent{}
	}
	if c.Context.Indices == nil {
		c.Context.Indices = []model.ResearchIndex{}
	}
	return c, nil
}

// Create publishes a call and records revision 1 in the same transaction, so a
// call can never exist without the revision history claiming to be complete.
func (r *ResearchRepo) Create(ctx context.Context, c model.ResearchCall) (model.ResearchCall, error) {
	evidence, err := json.Marshal(c.Evidence)
	if err != nil {
		return model.ResearchCall{}, err
	}
	situation, err := json.Marshal(c.Context)
	if err != nil {
		return model.ResearchCall{}, err
	}
	chart, err := json.Marshal(c.Chart)
	if err != nil {
		return model.ResearchCall{}, err
	}
	tags, err := json.Marshal(c.Tags)
	if err != nil {
		return model.ResearchCall{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ResearchCall{}, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO research_calls (
			user_id, topic, symbol, symbol_name, asset_type, currency, title, published_at,
			trade_date, benchmark_id, direction, base_price, target_price, stop_price,
			horizon_days, conviction, score_at_pub, grade_at_pub, contrarian,
			evidence, context, chart, body_md, body_rev, thumbnail_url, tags_json, visibility, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?)
	`,
		c.UserID, researchTopicOrStock(c.Topic), c.Symbol, c.SymbolName, string(c.AssetType), string(c.Currency), c.Title,
		c.PublishedAt.UTC(), c.TradeDate, c.BenchmarkID, string(c.Direction), c.BasePrice,
		// nullFloat64 (not nullFloat) — nullFloat drops non-positive values.
		nullFloat64(c.TargetPrice), nullFloat64(c.StopPrice), c.HorizonDays, c.Conviction,
		nullIntArg(c.ScoreAtPub), nullString(c.GradeAtPub), c.Contrarian,
		string(evidence), string(situation), string(chart), c.BodyMD, c.ThumbnailURL, string(tags),
		string(c.Visibility), string(researchStatusOrOpen(c.Status)),
	)
	if err != nil {
		return model.ResearchCall{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.ResearchCall{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO research_revisions (call_id, rev, title, body_md) VALUES (?, 1, ?, ?)
	`, id, c.Title, c.BodyMD); err != nil {
		return model.ResearchCall{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.ResearchCall{}, err
	}
	return r.GetByID(ctx, id)
}

// GetByID reads a call regardless of owner. Visibility is the service's call to
// enforce — the batch worker and the admin console both need unfiltered reads.
func (r *ResearchRepo) GetByID(ctx context.Context, id int64) (model.ResearchCall, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+researchCols+researchFrom+` WHERE c.id = ?`, id)
	c, err := scanResearch(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ResearchCall{}, ErrResearchNotFound
	}
	return c, err
}

func (r *ResearchRepo) List(ctx context.Context, f ResearchListFilter) ([]model.ResearchCall, error) {
	query := `SELECT ` + researchCols + researchFrom + ` WHERE 1 = 1`
	args := []any{}

	if f.UserID > 0 {
		query += ` AND c.user_id = ?`
		args = append(args, f.UserID)
	}
	if f.PublicOnly {
		query += ` AND c.visibility = 'public'`
	}
	if f.Topic != "" {
		query += ` AND c.topic = ?`
		args = append(args, f.Topic)
	}
	if !f.IncludeHidden {
		query += ` AND c.hidden_at IS NULL`
	}
	if f.Symbol != "" {
		// 그 종목 리포트 + 본문에 그 종목을 태그한 리포트(2026-09-28).
		query += ` AND (c.symbol = ? OR c.id IN (SELECT target_id FROM content_mentions WHERE symbol = ? AND target_type = 'research_call'))`
		args = append(args, strings.ToUpper(f.Symbol), strings.ToUpper(f.Symbol))
	}
	if f.Status != "" {
		query += ` AND c.status = ?`
		args = append(args, f.Status)
	}
	if f.Direction != "" {
		query += ` AND c.direction = ?`
		args = append(args, f.Direction)
	}
	switch f.Contrarian {
	case "1":
		query += ` AND c.contrarian = 1`
	case "0":
		query += ` AND c.contrarian = 0`
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query += ` ORDER BY c.published_at DESC, c.id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, max(f.Offset, 0))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ResearchCall, 0)
	for rows.Next() {
		c, err := scanResearch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListAllForUser reads every call an author owns, including retracted and
// hidden ones. The scorecard must count what the author would rather hide.
func (r *ResearchRepo) ListAllForUser(ctx context.Context, userID int64) ([]model.ResearchCall, error) {
	return r.List(ctx, ResearchListFilter{UserID: userID, IncludeHidden: true, Limit: 200})
}

// ListOpen returns calls the verdict batch still has to judge, oldest first.
func (r *ResearchRepo) ListOpen(ctx context.Context, limit int) ([]model.ResearchCall, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+researchCols+researchFrom+
			` WHERE c.status = ? ORDER BY c.published_at ASC LIMIT ?`,
		string(model.StatusOpen), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ResearchCall, 0)
	for rows.Next() {
		c, err := scanResearch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateBody writes the only editable prose and bumps the revision counter.
// The frozen columns are deliberately absent from this statement.
//
// The thumbnail rides along and does not bump the revision: it is presentation,
// not an argument, so swapping the picture must not read as "edited 5 times".
func (r *ResearchRepo) UpdateBody(
	ctx context.Context, userID, id int64, title, bodyMD, thumbnailURL string, tags []string, visibility string,
) (model.ResearchCall, error) {
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return model.ResearchCall{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ResearchCall{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		rev      int
		curBody  string
		curTitle string
	)
	err = tx.QueryRowContext(ctx,
		`SELECT body_rev, body_md, title FROM research_calls WHERE id = ? AND user_id = ? FOR UPDATE`,
		id, userID).Scan(&rev, &curBody, &curTitle)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ResearchCall{}, ErrResearchNotFound
	}
	if err != nil {
		return model.ResearchCall{}, err
	}

	// Only bump the revision when the prose actually changed — flipping
	// visibility should not manufacture a fake "edited 5 times".
	changed := curBody != bodyMD || curTitle != title
	nextRev := rev
	if changed {
		nextRev = rev + 1
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE research_calls
		   SET title = ?, body_md = ?, body_rev = ?, thumbnail_url = ?, tags_json = ?, visibility = ?
		 WHERE id = ? AND user_id = ?
	`, title, bodyMD, nextRev, thumbnailURL, string(tagsJSON), visibility, id, userID); err != nil {
		return model.ResearchCall{}, err
	}
	if changed {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO research_revisions (call_id, rev, title, body_md) VALUES (?, ?, ?, ?)`,
			id, nextRev, title, bodyMD); err != nil {
			return model.ResearchCall{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.ResearchCall{}, err
	}
	return r.GetByID(ctx, id)
}

func researchTopicOrStock(t string) string {
	if t == "" {
		return model.TopicStock
	}
	return t
}

func researchStatusOrOpen(s model.ResearchStatus) model.ResearchStatus {
	if s == "" {
		return model.StatusOpen
	}
	return s
}

// Delete removes a call and every row hanging off it (no FKs, so by hand).
// The scorecard reads research_calls, so the call leaves the hit rates too.
func (r *ResearchRepo) Delete(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmts := []string{
		`DELETE FROM research_comment_likes WHERE comment_id IN (SELECT id FROM research_comments WHERE call_id = ?)`,
		`DELETE FROM research_comments WHERE call_id = ?`,
		`DELETE FROM research_reactions WHERE call_id = ?`,
		`DELETE FROM research_ratings WHERE call_id = ?`,
		`DELETE FROM research_views WHERE call_id = ?`,
		`DELETE FROM research_reports WHERE call_id = ?`,
		`DELETE FROM research_revisions WHERE call_id = ?`,
		`DELETE FROM research_reviews WHERE call_id = ?`,
		`DELETE FROM content_mentions WHERE target_type = 'research_call' AND target_id = ?`,
		`DELETE FROM content_asset_mentions WHERE target_type = 'research_call' AND target_id = ?`,
	}
	for _, q := range stmts {
		if _, err := tx.ExecContext(ctx, q, id); err != nil {
			return err
		}
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM research_calls WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrResearchNotFound
	}
	return tx.Commit()
}

// Retract withdraws a call without deleting it. The row keeps counting toward
// the author's retract rate.
func (r *ResearchRepo) Retract(ctx context.Context, userID, id int64) (model.ResearchCall, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE research_calls
		   SET status = ?, retracted_at = CURRENT_TIMESTAMP(3)
		 WHERE id = ? AND user_id = ? AND status != ?
	`, string(model.StatusRetracted), id, userID, string(model.StatusRetracted))
	if err != nil {
		return model.ResearchCall{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Already retracted, or not ours — GetByID plus an owner check resolves which.
		c, err := r.GetByID(ctx, id)
		if err != nil {
			return model.ResearchCall{}, err
		}
		if c.UserID != userID {
			return model.ResearchCall{}, ErrResearchNotFound
		}
		return c, nil
	}
	return r.GetByID(ctx, id)
}

// SetVerdict records the batch's judgement. Called only by the verdict worker.
func (r *ResearchRepo) SetVerdict(
	ctx context.Context, id int64, status model.ResearchStatus,
	verdictDate string, ret, alpha *float64,
) error {
	var date any
	if verdictDate != "" {
		date = verdictDate
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE research_calls
		   SET status = ?, verdict_at = CURRENT_TIMESTAMP(3), verdict_date = ?,
		       verdict_return = ?, verdict_alpha = ?
		 WHERE id = ? AND status = ?
	`, string(status), date, nullFloat64(ret), nullFloat64(alpha), id, string(model.StatusOpen))
	return err
}

func (r *ResearchRepo) Revisions(ctx context.Context, callID int64) ([]model.ResearchRevision, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, call_id, rev, title, body_md, created_at
		   FROM research_revisions WHERE call_id = ? ORDER BY rev DESC`, callID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ResearchRevision, 0)
	for rows.Next() {
		var (
			rv        model.ResearchRevision
			createdAt time.Time
		)
		if err := rows.Scan(&rv.ID, &rv.CallID, &rv.Rev, &rv.Title, &rv.BodyMD, &createdAt); err != nil {
			return nil, err
		}
		rv.CreatedAt = createdAt.UTC()
		out = append(out, rv)
	}
	return out, rows.Err()
}

func (r *ResearchRepo) RevisionCount(ctx context.Context, callID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM research_revisions WHERE call_id = ?`, callID).Scan(&n)
	return n, err
}

// CountSince powers the daily publish cap.
func (r *ResearchRepo) CountSince(ctx context.Context, userID int64, since time.Time) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM research_calls WHERE user_id = ? AND published_at >= ?`,
		userID, since.UTC()).Scan(&n)
	return n, err
}

// LastForSymbol powers the "same symbol again already?" guard.
func (r *ResearchRepo) LastForSymbol(ctx context.Context, userID int64, symbol string) (time.Time, bool, error) {
	var at time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT published_at FROM research_calls
		 WHERE user_id = ? AND symbol = ? AND status != ?
		 ORDER BY published_at DESC LIMIT 1`,
		userID, strings.ToUpper(strings.TrimSpace(symbol)), string(model.StatusRetracted),
	).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return at.UTC(), true, nil
}

// SymbolSummary aggregates public calls for the stock page block.
func (r *ResearchRepo) SymbolSummary(ctx context.Context, symbol string) (model.ResearchSymbolSummary, error) {
	out := model.ResearchSymbolSummary{Symbol: strings.ToUpper(strings.TrimSpace(symbol))}
	rows, err := r.db.QueryContext(ctx, `
		SELECT direction, status, COUNT(*)
		  FROM research_calls
		 WHERE symbol = ? AND visibility = 'public' AND hidden_at IS NULL
		 GROUP BY direction, status`, out.Symbol)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			direction string
			status    string
			n         int
		)
		if err := rows.Scan(&direction, &status, &n); err != nil {
			return out, err
		}
		st := model.ResearchStatus(status)
		if st == model.StatusRetracted {
			continue
		}
		out.Total += n
		switch model.ResearchDirection(direction) {
		case model.DirectionBuy:
			out.Buy += n
		case model.DirectionSell:
			out.Sell += n
		case model.DirectionHold:
			out.Hold += n
		}
		if st == model.StatusOpen {
			out.Open += n
		}
		if st.Resolved() {
			out.Resolved += n
			if st.Hit() {
				out.Hits += n
			}
		}
	}
	return out, rows.Err()
}

// StatusCounts is the verdict job's coverage: how many calls sit in each state.
func (r *ResearchRepo) StatusCounts(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM research_calls GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var (
			status string
			n      int
		)
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = n
	}
	return out, rows.Err()
}

// AddReport records a moderation report. A repeated report by the same user is
// a no-op rather than an error — the button should not punish a double click.
func (r *ResearchRepo) AddReport(ctx context.Context, callID, userID int64, reason, detail string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO research_reports (call_id, user_id, reason, detail)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE reason = VALUES(reason), detail = VALUES(detail)
	`, callID, userID, reason, detail)
	return err
}

// SetHidden is the admin moderation switch.
func (r *ResearchRepo) SetHidden(ctx context.Context, id int64, hidden bool) error {
	var expr string
	if hidden {
		expr = `hidden_at = CURRENT_TIMESTAMP(3)`
	} else {
		expr = `hidden_at = NULL`
	}
	res, err := r.db.ExecContext(ctx, `UPDATE research_calls SET `+expr+` WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrResearchNotFound
	}
	return nil
}

// MigrateResearchIfNeeded applies 037_research_calls.sql.
func MigrateResearchIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "research_calls")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	path := filepath.Join(migrationsDir, "037_research_calls.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 037_research_calls.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("037_research_calls.sql: %w", err)
		}
	}
	return nil
}
