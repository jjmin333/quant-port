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
	if err := r.decorateComments(ctx, out, viewerID); err != nil {
		return nil, err
	}
	return out, nil
}

/*
decorateComments 는 이미 엮은 트리에 **추천 수·내가 누른 것·글쓴이 활동량**을 얹는다
(2026-09-14 사용자 요청 3).

# 왜 위 질의에 JOIN 하지 않았나

세 값의 출처가 다르고 묶는 단위도 다르다 — 추천은 댓글별 집계, 내가 누른 것은
(댓글, 나) 정확 일치, 활동량은 **사용자별** 집계다. 한 질의에 다 넣으면 사용자별
집계가 댓글 수만큼 되풀이돼 곱해지고(같은 사람이 다섯 줄 달면 다섯 번 센다),
그걸 막으려고 파생 표를 겹치면 읽는 사람이 무엇을 세는 질의인지 알 수 없게 된다.

한 글의 댓글은 많아야 수백 줄이라 **질의 두 번이 싸다.** 그리고 이 함수는 트리를
훑으며 답글까지 같이 채운다 — 답글에 추천이 안 붙으면 「왜 어떤 줄에만 있지」가 된다.
*/
func (r *ResearchRepo) decorateComments(
	ctx context.Context, tree []model.ResearchComment, viewerID int64,
) error {
	// 트리의 모든 줄을 한 번에 가리킨다 — 답글도 댓글이다.
	var each func(fn func(*model.ResearchComment))
	each = func(fn func(*model.ResearchComment)) {
		for i := range tree {
			fn(&tree[i])
			for j := range tree[i].Replies {
				fn(&tree[i].Replies[j])
			}
		}
	}

	var ids, users []int64
	seenUser := map[int64]struct{}{}
	each(func(c *model.ResearchComment) {
		ids = append(ids, c.ID)
		if c.Deleted || c.UserID == 0 {
			return
		}
		if _, dup := seenUser[c.UserID]; dup {
			return
		}
		seenUser[c.UserID] = struct{}{}
		users = append(users, c.UserID)
	})
	if len(ids) == 0 {
		return nil
	}

	likes, mine, err := r.commentLikes(ctx, ids, viewerID)
	if err != nil {
		return err
	}
	activity, err := r.authorActivity(ctx, users)
	if err != nil {
		return err
	}
	each(func(c *model.ResearchComment) {
		c.LikeCount = likes[c.ID]
		_, c.Liked = mine[c.ID]
		// 지워진 줄에는 글쓴이가 없다(위에서 이름도 지웠다) — 활동량만 남기면
		// 「삭제된 댓글입니다」 옆에 300+ 가 서게 된다.
		if !c.Deleted {
			c.AuthorActivity = activity[c.UserID]
		}
	})
	return nil
}

func (r *ResearchRepo) commentLikes(
	ctx context.Context, ids []int64, viewerID int64,
) (map[int64]int, map[int64]struct{}, error) {
	in, args := inPlaceholders(ids)
	rows, err := r.db.QueryContext(ctx, `
		SELECT comment_id, COUNT(*), MAX(user_id = ?)
		  FROM research_comment_likes
		 WHERE comment_id IN (`+in+`)
		 GROUP BY comment_id`, append([]any{viewerID}, args...)...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	counts := map[int64]int{}
	mine := map[int64]struct{}{}
	for rows.Next() {
		var id int64
		var n int
		// `MAX(user_id = ?)`는 행이 없으면 NULL이다 — 그룹이 있는 이상 최소 한 행은
		// 있지만, 비로그인(viewerID=0)에서도 0이 오도록 NullBool로 받는다.
		var liked sql.NullBool
		if err := rows.Scan(&id, &n, &liked); err != nil {
			return nil, nil, err
		}
		counts[id] = n
		if viewerID > 0 && liked.Valid && liked.Bool {
			mine[id] = struct{}{}
		}
	}
	return counts, mine, rows.Err()
}

/*
authorActivity 는 사용자별 **공개 활동 수**를 센다 — 발행한 리포트와 남긴 댓글.

숨김·철회된 리포트는 빼고(`hidden_at`·`retracted_at`), 지운 댓글도 뺀다. 화면에
남아 있지 않은 것으로 「많이 활동한 사람」이 되면 그 배지는 아무것도 말하지 않는다.

두 수를 UNION ALL 로 합쳐 한 번에 센다 — 표가 둘이라 JOIN 하면 곱해진다.
*/
func (r *ResearchRepo) authorActivity(ctx context.Context, users []int64) (map[int64]int, error) {
	out := map[int64]int{}
	if len(users) == 0 {
		return out, nil
	}
	in, args := inPlaceholders(users)
	rows, err := r.db.QueryContext(ctx, `
		SELECT user_id, SUM(n) FROM (
		  SELECT user_id, COUNT(*) AS n FROM research_calls
		   WHERE user_id IN (`+in+`) AND hidden_at IS NULL AND retracted_at IS NULL
		   GROUP BY user_id
		  UNION ALL
		  SELECT user_id, COUNT(*) AS n FROM research_comments
		   WHERE user_id IN (`+in+`) AND deleted_at IS NULL
		   GROUP BY user_id
		) t GROUP BY user_id`, append(append([]any{}, args...), args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

/*
ToggleCommentLike 는 추천을 켜고 끈다. 돌려주는 값은 **켠 뒤의 상태**다.

글 추천(`ToggleReaction`)과 같은 모양이다 — 지우기가 0행이면 없었다는 뜻이므로
그때 넣는다. `INSERT ... ON DUPLICATE` 로는 「눌러서 껐다」를 표현할 수 없다.
*/
func (r *ResearchRepo) ToggleCommentLike(
	ctx context.Context, commentID, userID int64,
) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM research_comment_likes WHERE comment_id = ? AND user_id = ?`,
		commentID, userID)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return false, nil
	}
	/*
		없는 댓글·지워진 댓글에는 추천이 안 붙는다. 외래 키 대신 여기서 막는 이유는
		`deleted_at`까지 함께 봐야 하기 때문이다 — 삭제는 soft delete 라서 행은 남고,
		외래 키는 그 구분을 못 한다.
	*/
	var exists int
	err = r.db.QueryRowContext(ctx,
		`SELECT 1 FROM research_comments WHERE id = ? AND deleted_at IS NULL`, commentID).
		Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrResearchCommentNotFound
	}
	if err != nil {
		return false, err
	}
	if _, err := r.db.ExecContext(ctx,
		`INSERT IGNORE INTO research_comment_likes (comment_id, user_id) VALUES (?, ?)`,
		commentID, userID); err != nil {
		return false, err
	}
	return true, nil
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

// MigrateResearchCommentLikesIfNeeded applies 072_research_comment_likes.sql.
//
// 채워 넣을 과거 자료가 없다 — 추천은 누르는 순간 생기는 것이라, 표가 비어 있는
// 상태가 곧 「아직 아무도 안 눌렀다」로 맞다.
func MigrateResearchCommentLikesIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "research_comment_likes")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir, "072_research_comment_likes.sql", "댓글 추천")
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
