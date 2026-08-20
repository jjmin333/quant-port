package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

var ErrResearchCommentNotFound = errors.New("research comment not found")

// Social reads/writes for 리서치 콜 (docs/RESEARCH_CALL_DESIGN.md §10).
//
// Counts are aggregated from these tables rather than denormalised onto
// research_calls: a like count that can drift from its rows is worse than one
// extra query, and the per-call volumes here are small.

// ResearchCounts is the social tally for one call.
type ResearchCounts struct {
	Likes     int
	Bookmarks int
	Comments  int
}

// CountsFor aggregates reactions and comments for a set of calls in two
// queries, so a 50-row feed does not issue 100 lookups.
func (r *ResearchRepo) CountsFor(ctx context.Context, callIDs []int64) (map[int64]ResearchCounts, error) {
	out := map[int64]ResearchCounts{}
	if len(callIDs) == 0 {
		return out, nil
	}
	placeholders, args := inPlaceholders(callIDs)

	rows, err := r.db.QueryContext(ctx,
		`SELECT call_id, kind, COUNT(*) FROM research_reactions
		  WHERE call_id IN (`+placeholders+`) GROUP BY call_id, kind`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			id   int64
			kind string
			n    int
		)
		if err := rows.Scan(&id, &kind, &n); err != nil {
			rows.Close()
			return nil, err
		}
		c := out[id]
		switch model.ResearchReactionKind(kind) {
		case model.ReactionLike:
			c.Likes = n
		case model.ResearchBookmark:
			c.Bookmarks = n
		}
		out[id] = c
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = r.db.QueryContext(ctx,
		`SELECT call_id, COUNT(*) FROM research_comments
		  WHERE call_id IN (`+placeholders+`) AND deleted_at IS NULL GROUP BY call_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id int64
			n  int
		)
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		c := out[id]
		c.Comments = n
		out[id] = c
	}
	return out, rows.Err()
}

// MyReactions reports which of the given calls the viewer has reacted to.
func (r *ResearchRepo) MyReactions(
	ctx context.Context, userID int64, callIDs []int64,
) (map[int64]map[string]bool, error) {
	out := map[int64]map[string]bool{}
	if userID <= 0 || len(callIDs) == 0 {
		return out, nil
	}
	placeholders, args := inPlaceholders(callIDs)
	args = append([]any{userID}, args...)
	rows, err := r.db.QueryContext(ctx,
		`SELECT call_id, kind FROM research_reactions
		  WHERE user_id = ? AND call_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id   int64
			kind string
		)
		if err := rows.Scan(&id, &kind); err != nil {
			return nil, err
		}
		if out[id] == nil {
			out[id] = map[string]bool{}
		}
		out[id][kind] = true
	}
	return out, rows.Err()
}

// ToggleReaction flips a reaction on/off and reports the resulting state.
func (r *ResearchRepo) ToggleReaction(
	ctx context.Context, callID, userID int64, kind model.ResearchReactionKind,
) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM research_reactions WHERE call_id = ? AND user_id = ? AND kind = ?`,
		callID, userID, string(kind))
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return false, nil
	}
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO research_reactions (call_id, user_id, kind) VALUES (?, ?, ?)
		 ON DUPLICATE KEY UPDATE created_at = created_at`,
		callID, userID, string(kind)); err != nil {
		return false, err
	}
	return true, nil
}

// ---------------------------------------------------------------- comments

// Comments returns a call's thread, parents first with replies attached.
func (r *ResearchRepo) Comments(ctx context.Context, callID, viewerID int64) ([]model.ResearchComment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.call_id, c.parent_id, c.user_id, COALESCE(u.display_name, ''),
		       c.body, c.deleted_at, c.created_at, c.updated_at
		  FROM research_comments c
		  LEFT JOIN users u ON u.id = c.user_id
		 WHERE c.call_id = ?
		 ORDER BY c.created_at ASC, c.id ASC`, callID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var flat []model.ResearchComment
	for rows.Next() {
		var (
			c         model.ResearchComment
			parentID  sql.NullInt64
			deletedAt sql.NullTime
			createdAt time.Time
			updatedAt time.Time
		)
		if err := rows.Scan(&c.ID, &c.CallID, &parentID, &c.UserID, &c.AuthorName,
			&c.Body, &deletedAt, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if parentID.Valid {
			v := parentID.Int64
			c.ParentID = &v
		}
		c.Deleted = deletedAt.Valid
		if c.Deleted {
			// The row survives so replies keep their anchor, but the text does not.
			c.Body = ""
			c.AuthorName = ""
		}
		c.CanEdit = !c.Deleted && viewerID > 0 && viewerID == c.UserID
		c.CreatedAt = createdAt.UTC()
		c.UpdatedAt = updatedAt.UTC()
		flat = append(flat, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	byID := map[int64]int{}
	out := make([]model.ResearchComment, 0, len(flat))
	for _, c := range flat {
		if c.ParentID == nil {
			byID[c.ID] = len(out)
			out = append(out, c)
		}
	}
	for _, c := range flat {
		if c.ParentID == nil {
			continue
		}
		idx, ok := byID[*c.ParentID]
		if !ok {
			// Parent was hard-removed by an admin — surface the reply at top
			// level rather than dropping it silently.
			out = append(out, c)
			continue
		}
		out[idx].Replies = append(out[idx].Replies, c)
	}
	return out, nil
}

// AddComment inserts a comment. A reply to a reply is flattened onto the
// grandparent — threads deeper than one level turn a research page into a forum.
func (r *ResearchRepo) AddComment(
	ctx context.Context, callID, userID int64, parentID *int64, body string,
) (model.ResearchComment, error) {
	var parent any
	if parentID != nil {
		var grandparent sql.NullInt64
		var parentCall int64
		err := r.db.QueryRowContext(ctx,
			`SELECT call_id, parent_id FROM research_comments WHERE id = ? AND deleted_at IS NULL`,
			*parentID).Scan(&parentCall, &grandparent)
		if errors.Is(err, sql.ErrNoRows) {
			return model.ResearchComment{}, ErrResearchCommentNotFound
		}
		if err != nil {
			return model.ResearchComment{}, err
		}
		if parentCall != callID {
			return model.ResearchComment{}, ErrResearchCommentNotFound
		}
		if grandparent.Valid {
			parent = grandparent.Int64
		} else {
			parent = *parentID
		}
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO research_comments (call_id, parent_id, user_id, body) VALUES (?, ?, ?, ?)`,
		callID, parent, userID, body)
	if err != nil {
		return model.ResearchComment{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.ResearchComment{}, err
	}
	return r.comment(ctx, id, userID)
}

func (r *ResearchRepo) comment(ctx context.Context, id, viewerID int64) (model.ResearchComment, error) {
	var (
		c         model.ResearchComment
		parentID  sql.NullInt64
		deletedAt sql.NullTime
		createdAt time.Time
		updatedAt time.Time
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT c.id, c.call_id, c.parent_id, c.user_id, COALESCE(u.display_name, ''),
		       c.body, c.deleted_at, c.created_at, c.updated_at
		  FROM research_comments c LEFT JOIN users u ON u.id = c.user_id
		 WHERE c.id = ?`, id).
		Scan(&c.ID, &c.CallID, &parentID, &c.UserID, &c.AuthorName, &c.Body,
			&deletedAt, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ResearchComment{}, ErrResearchCommentNotFound
	}
	if err != nil {
		return model.ResearchComment{}, err
	}
	if parentID.Valid {
		v := parentID.Int64
		c.ParentID = &v
	}
	c.Deleted = deletedAt.Valid
	c.CanEdit = !c.Deleted && viewerID > 0 && viewerID == c.UserID
	c.CreatedAt = createdAt.UTC()
	c.UpdatedAt = updatedAt.UTC()
	return c, nil
}

func (r *ResearchRepo) UpdateComment(
	ctx context.Context, id, userID int64, body string,
) (model.ResearchComment, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE research_comments SET body = ? WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		body, id, userID)
	if err != nil {
		return model.ResearchComment{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ResearchComment{}, ErrResearchCommentNotFound
	}
	return r.comment(ctx, id, userID)
}

// DeleteComment soft-deletes so replies keep their anchor.
func (r *ResearchRepo) DeleteComment(ctx context.Context, id, userID int64) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE research_comments SET deleted_at = CURRENT_TIMESTAMP(3), body = ''
		  WHERE id = ? AND user_id = ? AND deleted_at IS NULL`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrResearchCommentNotFound
	}
	return nil
}

// ---------------------------------------------------------------- ratings

func (r *ResearchRepo) UpsertRating(ctx context.Context, rt model.ResearchRating) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO research_ratings (call_id, user_id, clarity, evidence, risk)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE clarity = VALUES(clarity), evidence = VALUES(evidence), risk = VALUES(risk)
	`, rt.CallID, rt.UserID, rt.Clarity, rt.Evidence, rt.Risk)
	return err
}

func (r *ResearchRepo) RatingSummary(ctx context.Context, callID int64) (*model.ResearchRatingSummary, error) {
	var (
		n                        int
		clarity, evidence, risky sql.NullFloat64
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*), AVG(clarity), AVG(evidence), AVG(risk)
		   FROM research_ratings WHERE call_id = ?`, callID).
		Scan(&n, &clarity, &evidence, &risky)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	out := &model.ResearchRatingSummary{
		Count:    n,
		Clarity:  clarity.Float64,
		Evidence: evidence.Float64,
		Risk:     risky.Float64,
	}
	out.Overall = (out.Clarity + out.Evidence + out.Risk) / 3
	return out, nil
}

func (r *ResearchRepo) MyRating(ctx context.Context, callID, userID int64) (*model.ResearchRating, error) {
	if userID <= 0 {
		return nil, nil
	}
	var (
		rt        model.ResearchRating
		createdAt time.Time
		updatedAt time.Time
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT call_id, user_id, clarity, evidence, risk, created_at, updated_at
		   FROM research_ratings WHERE call_id = ? AND user_id = ?`, callID, userID).
		Scan(&rt.CallID, &rt.UserID, &rt.Clarity, &rt.Evidence, &rt.Risk, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rt.CreatedAt = createdAt.UTC()
	rt.UpdatedAt = updatedAt.UTC()
	return &rt, nil
}

// ---------------------------------------------------------------- views

// researchViewWindow is how long the same viewer stops counting again.
const researchViewWindow = 30 * time.Minute

// RecordView bumps the counter only when this viewer has not been seen on this
// call recently. Returns true when the counter actually moved.
func (r *ResearchRepo) RecordView(ctx context.Context, callID int64, viewerKey string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO research_views (call_id, viewer_key) VALUES (?, ?)
		ON DUPLICATE KEY UPDATE
			viewed_at = IF(viewed_at < NOW(3) - INTERVAL ? SECOND, NOW(3), viewed_at)
	`, callID, viewerKey, int(researchViewWindow.Seconds()))
	if err != nil {
		return false, err
	}
	// affected: 1 = inserted, 2 = updated (window elapsed), 0 = untouched.
	n, _ := res.RowsAffected()
	if n == 0 {
		return false, nil
	}
	if _, err := r.db.ExecContext(ctx,
		`UPDATE research_calls SET view_count = view_count + 1 WHERE id = ?`, callID); err != nil {
		return false, err
	}
	return true, nil
}

func inPlaceholders(ids []int64) (string, []any) {
	buf := make([]byte, 0, len(ids)*2)
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '?')
		args = append(args, id)
	}
	return string(buf), args
}

// MigrateResearchSocialIfNeeded applies 038_research_social.sql.
func MigrateResearchSocialIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "research_reactions")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	path := filepath.Join(migrationsDir, "038_research_social.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 038_research_social.sql: %w", err)
	}
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("038_research_social.sql: %w", err)
		}
	}
	return nil
}

// MigrateResearchThumbnailIfNeeded applies 049_research_thumbnail.sql.
//
// Existing reports keep an empty thumbnail and fall back to the stock logo on
// screen — nothing to backfill, since no image was ever chosen for them.
func MigrateResearchThumbnailIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "research_calls")
	if err != nil || !exists {
		return err
	}
	has, err := columnExists(ctx, db, "research_calls", "thumbnail_url")
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	path := filepath.Join(migrationsDir, "049_research_thumbnail.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read 049_research_thumbnail.sql: %w", err)
	}
	applied := false
	for _, stmt := range splitSQLStatements(string(body)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if isSchemaRaceError(err) {
				continue
			}
			return fmt.Errorf("049_research_thumbnail.sql: %w", err)
		}
		applied = true
	}
	if applied {
		log.Println("mysql: research_calls migrated (thumbnail_url)")
	}
	return nil
}
