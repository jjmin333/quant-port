package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

/*
커뮤니티 저장소 (2026-09-27 사용자 요청 4 — migrations/088_community.sql).

카운터는 글·댓글 행에 들고 있고, 원장(reactions·views·reports)은 「한 사람 한 번」을 지키는
데만 쓴다. 카운터를 바꾸는 쓰기는 **원장 쓰기와 같은 트랜잭션**이다 — 둘이 어긋나면 좋아요를
취소했는데 숫자가 안 줄거나, 두 번 눌러 두 번 느는 일이 생긴다.
*/

var (
	ErrCommunityNotFound  = errors.New("community: not found")
	ErrCommunityForbidden = errors.New("community: not the author")
)

// CommunityAutoHideReports — 서로 다른 사람 이만큼이 신고하면 관리자 확인 전까지 숨긴다.
const CommunityAutoHideReports = 3

type CommunityRepo struct{ db *sql.DB }

func NewCommunityRepo(db *sql.DB) *CommunityRepo { return &CommunityRepo{db: db} }

// ------------------------------------------------------------------ write

type CommunityNewSymbol struct {
	Symbol     string
	Price      *float64
	Currency   string
	ChangeRate *float64
	Score      *int
}

type CommunityNewPost struct {
	UserID   int64
	Title    string
	Topic    string
	Body     string
	Symbols  []CommunityNewSymbol
	Articles []model.CommunityArticle
	Images   []model.CommunityImage
}

func (r *CommunityRepo) CreatePost(ctx context.Context, p CommunityNewPost) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx,
		`INSERT INTO community_posts (user_id, title, topic, body) VALUES (?, ?, ?, ?)`, p.UserID, p.Title, p.Topic, p.Body)
	if err != nil {
		return 0, fmt.Errorf("community insert post: %w", err)
	}
	id, _ := res.LastInsertId()
	for i, s := range p.Symbols {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO community_post_symbols
			  (post_id, symbol, position, snap_price, snap_currency, snap_change_rate, snap_score)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, s.Symbol, i, s.Price, s.Currency, s.ChangeRate, s.Score); err != nil {
			return 0, fmt.Errorf("community insert symbol: %w", err)
		}
	}
	for i, a := range p.Articles {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO community_post_articles (post_id, article_id, position, title, source, url, published_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, a.ArticleID, i, a.Title, a.Source, a.URL, a.PublishedAt); err != nil {
			return 0, fmt.Errorf("community insert article: %w", err)
		}
	}
	for i, img := range p.Images {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO community_post_images (post_id, position, url, width, height) VALUES (?, ?, ?, ?, ?)`,
			id, i, img.URL, img.Width, img.Height); err != nil {
			return 0, fmt.Errorf("community insert image: %w", err)
		}
	}
	return id, tx.Commit()
}

// LastPostAt is the author's most recent post time (any status) — the 1분 1글 rule.
func (r *CommunityRepo) LastPostAt(ctx context.Context, userID int64) (time.Time, bool, error) {
	var t sql.NullTime
	err := r.db.QueryRowContext(ctx,
		`SELECT MAX(created_at) FROM community_posts WHERE user_id = ?`, userID).Scan(&t)
	if err != nil {
		return time.Time{}, false, err
	}
	return t.Time, t.Valid, nil
}

// LastCommentAt — 댓글 연타 제한용.
func (r *CommunityRepo) LastCommentAt(ctx context.Context, userID int64) (time.Time, bool, error) {
	var t sql.NullTime
	err := r.db.QueryRowContext(ctx,
		`SELECT MAX(created_at) FROM community_comments WHERE user_id = ?`, userID).Scan(&t)
	if err != nil {
		return time.Time{}, false, err
	}
	return t.Time, t.Valid, nil
}

// UpdatePost edits title/body of the author's own visible post.
// anyOwner — 관리자 수정·삭제(서비스가 권한을 확인한 뒤에만 true, 2026-10-09).
func (r *CommunityRepo) UpdatePost(ctx context.Context, id, userID int64, title, topic, body string, anyOwner bool) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE community_posts SET title = ?, topic = ?, body = ?, edited_at = CURRENT_TIMESTAMP(3)
		 WHERE id = ? AND (user_id = ? OR ?) AND status <> 'deleted'`, title, topic, body, id, userID, anyOwner)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return r.ownershipErr(ctx, "community_posts", id)
	}
	return nil
}

// DeletePost soft-deletes the author's post (or any post, for an admin).
func (r *CommunityRepo) DeletePost(ctx context.Context, id, userID int64, anyOwner bool) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE community_posts SET status = 'deleted' WHERE id = ? AND (user_id = ? OR ?) AND status <> 'deleted'`, id, userID, anyOwner)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return r.ownershipErr(ctx, "community_posts", id)
	}
	return nil
}

func (r *CommunityRepo) ownershipErr(ctx context.Context, table string, id int64) error {
	var n int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+table+` WHERE id = ? AND status <> 'deleted'`, id).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return ErrCommunityNotFound
	}
	return ErrCommunityForbidden
}

// ToggleLike flips a like on a post or comment; returns the new state and count.
func (r *CommunityRepo) ToggleLike(ctx context.Context, targetType string, targetID, userID int64) (bool, int, error) {
	table := communityTargetTable(targetType)
	if table == "" {
		return false, 0, ErrCommunityNotFound
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback() //nolint:errcheck

	var status string
	if err := tx.QueryRowContext(ctx,
		`SELECT status FROM `+table+` WHERE id = ? FOR UPDATE`, targetID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, 0, ErrCommunityNotFound
		}
		return false, 0, err
	}
	if status != model.CommunityStatusVisible {
		return false, 0, ErrCommunityNotFound
	}
	res, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO community_reactions (target_type, target_id, user_id) VALUES (?, ?, ?)`,
		targetType, targetID, userID)
	if err != nil {
		return false, 0, err
	}
	on := true
	delta := 1
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM community_reactions WHERE target_type = ? AND target_id = ? AND user_id = ?`,
			targetType, targetID, userID); err != nil {
			return false, 0, err
		}
		on = false
		delta = -1
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE `+table+` SET like_count = GREATEST(CAST(like_count AS SIGNED) + ?, 0) WHERE id = ?`,
		delta, targetID); err != nil {
		return false, 0, err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT like_count FROM `+table+` WHERE id = ?`, targetID).Scan(&count); err != nil {
		return false, 0, err
	}
	return on, count, tx.Commit()
}

// RecordView counts a viewer once per post.
func (r *CommunityRepo) RecordView(ctx context.Context, postID int64, viewerKey string, userID int64) error {
	var uid any
	if userID > 0 {
		uid = userID
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT IGNORE INTO community_views (post_id, viewer_key, user_id)
		SELECT id, ?, ? FROM community_posts WHERE id = ? AND status = 'visible'`, viewerKey, uid, postID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		_, err = r.db.ExecContext(ctx, `UPDATE community_posts SET view_count = view_count + 1 WHERE id = ?`, postID)
	}
	return err
}

func (r *CommunityRepo) AddComment(ctx context.Context, postID int64, parentID *int64, userID int64, body string) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck

	var status string
	if err := tx.QueryRowContext(ctx,
		`SELECT status FROM community_posts WHERE id = ? FOR UPDATE`, postID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrCommunityNotFound
		}
		return 0, err
	}
	if status != model.CommunityStatusVisible {
		return 0, ErrCommunityNotFound
	}
	if parentID != nil {
		// 답글은 한 단계만 — 부모가 이미 답글이면 그 부모(원 댓글)에 단다.
		var pPost int64
		var pParent sql.NullInt64
		if err := tx.QueryRowContext(ctx,
			`SELECT post_id, parent_id FROM community_comments WHERE id = ?`, *parentID).Scan(&pPost, &pParent); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, ErrCommunityNotFound
			}
			return 0, err
		}
		if pPost != postID {
			return 0, ErrCommunityNotFound
		}
		if pParent.Valid {
			root := pParent.Int64
			parentID = &root
		}
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO community_comments (post_id, parent_id, user_id, body) VALUES (?, ?, ?, ?)`,
		postID, parentID, userID, body)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx,
		`UPDATE community_posts SET comment_count = comment_count + 1 WHERE id = ?`, postID); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func (r *CommunityRepo) DeleteComment(ctx context.Context, id, userID int64, anyOwner bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var postID, owner int64
	var status string
	if err := tx.QueryRowContext(ctx,
		`SELECT post_id, user_id, status FROM community_comments WHERE id = ? FOR UPDATE`, id).Scan(&postID, &owner, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCommunityNotFound
		}
		return err
	}
	if status == model.CommunityStatusDeleted {
		return ErrCommunityNotFound
	}
	if owner != userID && !anyOwner {
		return ErrCommunityForbidden
	}
	if _, err := tx.ExecContext(ctx, `UPDATE community_comments SET status = 'deleted' WHERE id = ?`, id); err != nil {
		return err
	}
	if status == model.CommunityStatusVisible {
		if _, err := tx.ExecContext(ctx, `
			UPDATE community_posts SET comment_count = GREATEST(CAST(comment_count AS SIGNED) - 1, 0) WHERE id = ?`, postID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Report records one report and auto-hides at CommunityAutoHideReports. Returns
// whether this call newly hid the target.
func (r *CommunityRepo) Report(ctx context.Context, targetType string, targetID, reporterID int64, reason string) (bool, error) {
	table := communityTargetTable(targetType)
	if table == "" {
		return false, ErrCommunityNotFound
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback() //nolint:errcheck

	var status string
	var owner int64
	if err := tx.QueryRowContext(ctx,
		`SELECT status, user_id FROM `+table+` WHERE id = ? FOR UPDATE`, targetID).Scan(&status, &owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrCommunityNotFound
		}
		return false, err
	}
	if status == model.CommunityStatusDeleted || owner == reporterID {
		return false, ErrCommunityNotFound
	}
	res, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO community_reports (target_type, target_id, reporter_id, reason) VALUES (?, ?, ?, ?)`,
		targetType, targetID, reporterID, reason)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, tx.Commit() // 이미 신고함 — 조용히 성공
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE `+table+` SET report_count = report_count + 1 WHERE id = ?`, targetID); err != nil {
		return false, err
	}
	var reports int
	if err := tx.QueryRowContext(ctx, `SELECT report_count FROM `+table+` WHERE id = ?`, targetID).Scan(&reports); err != nil {
		return false, err
	}
	hid := false
	if status == model.CommunityStatusVisible && reports >= CommunityAutoHideReports {
		if err := setCommunityStatusTx(ctx, tx, targetType, targetID, model.CommunityStatusHidden, "reports"); err != nil {
			return false, err
		}
		hid = true
	}
	return hid, tx.Commit()
}

// SetStatus is the admin switch (visible | hidden). It keeps comment_count honest.
func (r *CommunityRepo) SetStatus(ctx context.Context, targetType string, targetID int64, status string) error {
	if status != model.CommunityStatusVisible && status != model.CommunityStatusHidden {
		return fmt.Errorf("invalid status %q", status)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	reason := ""
	if status == model.CommunityStatusHidden {
		reason = "admin"
	}
	if err := setCommunityStatusTx(ctx, tx, targetType, targetID, status, reason); err != nil {
		return err
	}
	return tx.Commit()
}

func setCommunityStatusTx(ctx context.Context, tx *sql.Tx, targetType string, id int64, status, reason string) error {
	switch targetType {
	case model.CommunityTargetPost:
		res, err := tx.ExecContext(ctx, `
			UPDATE community_posts SET status = ?, hidden_reason = ? WHERE id = ? AND status <> 'deleted'`, status, reason, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrCommunityNotFound
		}
		return nil
	case model.CommunityTargetComment:
		var postID int64
		var prev string
		if err := tx.QueryRowContext(ctx,
			`SELECT post_id, status FROM community_comments WHERE id = ? FOR UPDATE`, id).Scan(&postID, &prev); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrCommunityNotFound
			}
			return err
		}
		if prev == model.CommunityStatusDeleted {
			return ErrCommunityNotFound
		}
		if prev == status {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `UPDATE community_comments SET status = ? WHERE id = ?`, status, id); err != nil {
			return err
		}
		delta := 1
		if status == model.CommunityStatusHidden {
			delta = -1
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE community_posts SET comment_count = GREATEST(CAST(comment_count AS SIGNED) + ?, 0) WHERE id = ?`, delta, postID)
		return err
	}
	return ErrCommunityNotFound
}

func communityTargetTable(targetType string) string {
	switch targetType {
	case model.CommunityTargetPost:
		return "community_posts"
	case model.CommunityTargetComment:
		return "community_comments"
	}
	return ""
}

// ------------------------------------------------------------------- read

// CommunityPostRow is a post row before author/badge decoration.
type CommunityPostRow struct {
	model.CommunityPost
	UserID int64
}

const communityPostCols = `p.id, p.user_id, p.title, p.topic, p.body, p.status, p.hidden_reason, p.like_count,
	p.comment_count, p.view_count, p.report_count, p.created_at, p.edited_at`

func scanCommunityPost(sc interface{ Scan(...any) error }) (CommunityPostRow, error) {
	var row CommunityPostRow
	var edited sql.NullTime
	if err := sc.Scan(&row.ID, &row.UserID, &row.Title, &row.Topic, &row.Body, &row.Status, &row.HiddenReason,
		&row.LikeCount, &row.CommentCount, &row.ViewCount, &row.ReportCount, &row.CreatedAt, &edited); err != nil {
		return row, err
	}
	if edited.Valid {
		t := edited.Time
		row.EditedAt = &t
	}
	row.Author.ID = row.UserID
	return row, nil
}

func (r *CommunityRepo) GetPost(ctx context.Context, id int64) (CommunityPostRow, error) {
	row, err := scanCommunityPost(r.db.QueryRowContext(ctx,
		`SELECT `+communityPostCols+` FROM community_posts p WHERE p.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return row, ErrCommunityNotFound
	}
	return row, err
}

type CommunityListQuery struct {
	// Sort: "latest" (default) | "popular"
	Sort   string
	Symbol string
	// Topic — 빈 값은 전체(2026-10-09 요청 9).
	Topic string
	// AuthorID lists one author's posts (「내 활동」) — includes hidden ones.
	AuthorID int64
	// BeforeID pages latest/author lists; Offset pages popular.
	BeforeID int64
	Offset   int
	Limit    int
	// PopularSince bounds the popular window.
	PopularSince time.Time
}

// ListPosts returns one page. The caller asks for Limit+1 to learn whether more exist.
func (r *CommunityRepo) ListPosts(ctx context.Context, q CommunityListQuery) ([]CommunityPostRow, error) {
	where := []string{}
	args := []any{}
	if q.AuthorID > 0 {
		where = append(where, "p.user_id = ?", "p.status <> 'deleted'")
		args = append(args, q.AuthorID)
	} else {
		where = append(where, "p.status = 'visible'")
	}
	if q.Symbol != "" {
		// 붙인 글 + 본문에 태그한 글(2026-09-28) — 종목 화면 「커뮤니티」 탭이 둘 다 본다.
		where = append(where, `(p.id IN (SELECT post_id FROM community_post_symbols WHERE symbol = ?)
			OR p.id IN (SELECT target_id FROM content_mentions WHERE symbol = ? AND target_type = 'community_post'))`)
		args = append(args, q.Symbol, q.Symbol)
	}
	if q.Topic != "" {
		where = append(where, "p.topic = ?")
		args = append(args, q.Topic)
	}
	order := "p.id DESC"
	limitClause := " LIMIT ?"
	if q.Sort == "popular" && q.AuthorID == 0 {
		where = append(where, "p.created_at >= ?")
		args = append(args, q.PopularSince)
		order = communityHotScoreSQL + " DESC, p.id DESC"
		limitClause = " LIMIT ? OFFSET ?"
	} else if q.BeforeID > 0 {
		where = append(where, "p.id < ?")
		args = append(args, q.BeforeID)
	}
	args = append(args, q.Limit)
	if q.Sort == "popular" && q.AuthorID == 0 {
		args = append(args, q.Offset)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+communityPostCols+` FROM community_posts p
		 WHERE `+strings.Join(where, " AND ")+`
		 ORDER BY `+order+limitClause, args...)
	if err != nil {
		return nil, fmt.Errorf("community list: %w", err)
	}
	defer rows.Close()
	var out []CommunityPostRow
	for rows.Next() {
		row, err := scanCommunityPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// communityHotScoreSQL — 인기 정렬. 좋아요·댓글이 조회보다 훨씬 무겁다(조회는 20회 = 좋아요 1/3).
const communityHotScoreSQL = `(p.like_count * 3 + p.comment_count * 2 + p.view_count / 20)`

// CommunityHotScore mirrors communityHotScoreSQL for the 「인기」 label on list rows.
func CommunityHotScore(likes, comments, views int) float64 {
	return float64(likes*3+comments*2) + float64(views)/20
}

// AttachSymbols fills Symbols (with stock names) for the given posts.
func (r *CommunityRepo) AttachSymbols(ctx context.Context, posts []CommunityPostRow) error {
	if len(posts) == 0 {
		return nil
	}
	ids, index := communityPostIndex(posts)
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.post_id, s.symbol, COALESCE(st.name, ''), COALESCE(st.asset_type, ''),
		       s.snap_price, s.snap_currency, s.snap_change_rate, s.snap_score, s.snap_at
		  FROM community_post_symbols s
		  LEFT JOIN stocks st ON st.symbol = s.symbol
		 WHERE s.post_id IN (`+placeholders(len(ids))+`)
		 ORDER BY s.post_id, s.position`, ids...)
	if err != nil {
		return fmt.Errorf("community symbols: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var postID int64
		var s model.CommunitySymbol
		var price, rate sql.NullFloat64
		var score sql.NullInt64
		if err := rows.Scan(&postID, &s.Symbol, &s.Name, &s.AssetType, &price, &s.SnapCurrency, &rate, &score, &s.SnapAt); err != nil {
			return err
		}
		if price.Valid {
			v := price.Float64
			s.SnapPrice = &v
		}
		if rate.Valid {
			v := rate.Float64
			s.SnapChangeRate = &v
		}
		if score.Valid {
			v := int(score.Int64)
			s.SnapScore = &v
		}
		if i, ok := index[postID]; ok {
			posts[i].Symbols = append(posts[i].Symbols, s)
		}
	}
	return rows.Err()
}

// AttachArticles fills Articles (all of them — at most 3 per post).
func (r *CommunityRepo) AttachArticles(ctx context.Context, posts []CommunityPostRow) error {
	if len(posts) == 0 {
		return nil
	}
	ids, index := communityPostIndex(posts)
	rows, err := r.db.QueryContext(ctx, `
		SELECT post_id, article_id, title, source, COALESCE(url, ''), published_at
		  FROM community_post_articles
		 WHERE post_id IN (`+placeholders(len(ids))+`)
		 ORDER BY post_id, position`, ids...)
	if err != nil {
		return fmt.Errorf("community articles: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var postID int64
		var a model.CommunityArticle
		var pub sql.NullTime
		if err := rows.Scan(&postID, &a.ArticleID, &a.Title, &a.Source, &a.URL, &pub); err != nil {
			return err
		}
		if pub.Valid {
			t := pub.Time
			a.PublishedAt = &t
		}
		if i, ok := index[postID]; ok {
			posts[i].Articles = append(posts[i].Articles, a)
			posts[i].ArticleCount++
		}
	}
	return rows.Err()
}

// AttachImages fills Images; firstOnly keeps just the thumbnail (list rows).
func (r *CommunityRepo) AttachImages(ctx context.Context, posts []CommunityPostRow, firstOnly bool) error {
	if len(posts) == 0 {
		return nil
	}
	ids, index := communityPostIndex(posts)
	rows, err := r.db.QueryContext(ctx, `
		SELECT post_id, url, width, height FROM community_post_images
		 WHERE post_id IN (`+placeholders(len(ids))+`)
		 ORDER BY post_id, position`, ids...)
	if err != nil {
		return fmt.Errorf("community images: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var postID int64
		var img model.CommunityImage
		if err := rows.Scan(&postID, &img.URL, &img.Width, &img.Height); err != nil {
			return err
		}
		i, ok := index[postID]
		if !ok {
			continue
		}
		posts[i].ImageCount++
		if !firstOnly || len(posts[i].Images) == 0 {
			posts[i].Images = append(posts[i].Images, img)
		}
	}
	return rows.Err()
}

// LikedBy reports which of the targets userID has liked.
func (r *CommunityRepo) LikedBy(ctx context.Context, targetType string, ids []int64, userID int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	if userID <= 0 || len(ids) == 0 {
		return out, nil
	}
	args := []any{targetType, userID}
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT target_id FROM community_reactions
		 WHERE target_type = ? AND user_id = ? AND target_id IN (`+placeholders(len(ids))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// CommunityCommentRow is a comment before author decoration.
type CommunityCommentRow struct {
	model.CommunityComment
	UserID int64
}

// Comments returns a post's comments oldest-first. Deleted/hidden comments stay
// in the list as tombstones when they have replies, so a thread does not lose its root.
func (r *CommunityRepo) Comments(ctx context.Context, postID int64) ([]CommunityCommentRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, post_id, parent_id, user_id, body, status, like_count, report_count, created_at
		  FROM community_comments WHERE post_id = ? ORDER BY created_at, id`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CommunityCommentRow
	for rows.Next() {
		var c CommunityCommentRow
		var parent sql.NullInt64
		if err := rows.Scan(&c.ID, &c.PostID, &parent, &c.UserID, &c.Body, &c.Status, &c.LikeCount, &c.ReportCount, &c.CreatedAt); err != nil {
			return nil, err
		}
		if parent.Valid {
			p := parent.Int64
			c.ParentID = &p
		}
		c.Author.ID = c.UserID
		out = append(out, c)
	}
	return out, rows.Err()
}

// ------------------------------------------------------------ badges input

// CommunityAuthorStats is everything the badge rules read for one user.
type CommunityAuthorStats struct {
	DisplayName   string
	Role          string
	Badge         string // 게시 계정(role=persona)의 뱃지 문구 — 관리자가 정한다
	JoinedAt      time.Time
	Posts         int
	Comments      int
	LikesReceived int
	Reads         int
	JudgedStamps  int
	Notes         int
}

// AuthorStats gathers badge inputs for a page's authors in a fixed number of queries.
func (r *CommunityRepo) AuthorStats(ctx context.Context, userIDs []int64) (map[int64]*CommunityAuthorStats, error) {
	out := map[int64]*CommunityAuthorStats{}
	if len(userIDs) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(userIDs))
	for _, id := range userIDs {
		args = append(args, id)
		out[id] = &CommunityAuthorStats{}
	}
	in := placeholders(len(userIDs))

	type q struct {
		sql string
		set func(s *CommunityAuthorStats, v int)
	}
	counts := []q{
		{`SELECT user_id, COUNT(*) FROM community_posts WHERE status <> 'deleted' AND user_id IN (` + in + `) GROUP BY user_id`,
			func(s *CommunityAuthorStats, v int) { s.Posts = v }},
		{`SELECT user_id, COUNT(*) FROM community_comments WHERE status <> 'deleted' AND user_id IN (` + in + `) GROUP BY user_id`,
			func(s *CommunityAuthorStats, v int) { s.Comments = v }},
		{`SELECT user_id, COALESCE(SUM(like_count), 0) FROM community_posts WHERE status <> 'deleted' AND user_id IN (` + in + `) GROUP BY user_id`,
			func(s *CommunityAuthorStats, v int) { s.LikesReceived = v }},
		{`SELECT user_id, COUNT(*) FROM community_views WHERE user_id IN (` + in + `) GROUP BY user_id`,
			func(s *CommunityAuthorStats, v int) { s.Reads = v }},
		{`SELECT user_id, COUNT(*) FROM stamps WHERE user_id IN (` + in + `)
		    AND due_at IS NOT NULL AND due_at <= CURRENT_TIMESTAMP(3) GROUP BY user_id`,
			func(s *CommunityAuthorStats, v int) { s.JudgedStamps = v }},
		{`SELECT user_id, COUNT(*) FROM journal_notes WHERE user_id IN (` + in + `) GROUP BY user_id`,
			func(s *CommunityAuthorStats, v int) { s.Notes = v }},
	}
	for _, c := range counts {
		rows, err := r.db.QueryContext(ctx, c.sql, args...)
		if err != nil {
			return nil, fmt.Errorf("community author stats: %w", err)
		}
		for rows.Next() {
			var id int64
			var v int
			if err := rows.Scan(&id, &v); err != nil {
				rows.Close()
				return nil, err
			}
			if s, ok := out[id]; ok {
				c.set(s, v)
			}
		}
		rows.Close()
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, display_name, COALESCE(role, ''), badge, created_at FROM users WHERE id IN (`+in+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var st CommunityAuthorStats
		if err := rows.Scan(&id, &st.DisplayName, &st.Role, &st.Badge, &st.JoinedAt); err != nil {
			return nil, err
		}
		if s, ok := out[id]; ok {
			s.DisplayName, s.Role, s.Badge, s.JoinedAt = st.DisplayName, st.Role, st.Badge, st.JoinedAt
		}
	}
	return out, rows.Err()
}

// HeldSymbols returns, per user, which of symbols they currently hold.
func (r *CommunityRepo) HeldSymbols(ctx context.Context, userIDs []int64, symbols []string) (map[int64]map[string]bool, error) {
	out := map[int64]map[string]bool{}
	if len(userIDs) == 0 || len(symbols) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(userIDs)+len(symbols))
	for _, id := range userIDs {
		args = append(args, id)
	}
	for _, s := range symbols {
		args = append(args, s)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT user_id, symbol FROM user_holdings
		 WHERE quantity > 0 AND user_id IN (`+placeholders(len(userIDs))+`)
		   AND symbol IN (`+placeholders(len(symbols))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var sym string
		if err := rows.Scan(&id, &sym); err != nil {
			return nil, err
		}
		if out[id] == nil {
			out[id] = map[string]bool{}
		}
		out[id][strings.ToUpper(sym)] = true
	}
	return out, rows.Err()
}

// ------------------------------------------------------------- moderation

// ModerationPosts lists posts that were reported or hidden, newest first.
func (r *CommunityRepo) ModerationPosts(ctx context.Context, limit int) ([]CommunityPostRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+communityPostCols+` FROM community_posts p
		 WHERE p.status <> 'deleted' AND (p.report_count > 0 OR p.status = 'hidden')
		 ORDER BY p.status = 'hidden' DESC, p.report_count DESC, p.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CommunityPostRow
	for rows.Next() {
		row, err := scanCommunityPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ModerationComments lists reported or hidden comments.
func (r *CommunityRepo) ModerationComments(ctx context.Context, limit int) ([]CommunityCommentRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, post_id, parent_id, user_id, body, status, like_count, report_count, created_at
		  FROM community_comments
		 WHERE status <> 'deleted' AND (report_count > 0 OR status = 'hidden')
		 ORDER BY status = 'hidden' DESC, report_count DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CommunityCommentRow
	for rows.Next() {
		var c CommunityCommentRow
		var parent sql.NullInt64
		if err := rows.Scan(&c.ID, &c.PostID, &parent, &c.UserID, &c.Body, &c.Status, &c.LikeCount, &c.ReportCount, &c.CreatedAt); err != nil {
			return nil, err
		}
		if parent.Valid {
			p := parent.Int64
			c.ParentID = &p
		}
		c.Author.ID = c.UserID
		out = append(out, c)
	}
	return out, rows.Err()
}

/*
testAuthorEmail — 시험 계정(e2e·seed-preview)의 메일. growth의 realUser와 같은 예약 도메인이되
게시 계정(`@persona.invalid`)은 **빼지 않는다** — 그 글은 일부러 공개하는 글이다.

2026-10-21 점검(티켓 QP-07): 공개 글 211개 중 209개가 `@quant-port.test` 시험 글이었고, 같은
seed가 여러 번 들어가 자체 크롤 경고 452건 중 389건(고아·제목/설명 중복·얇은 본문)을 만들었다.
*/
const testAuthorEmail = `(u.email LIKE '%.test' OR u.email LIKE '%.example' OR u.email LIKE '%.localhost' OR u.email LIKE 'qae2e%')`

// IsTestAuthor reports whether userID is a test account (see testAuthorEmail).
func (r *CommunityRepo) IsTestAuthor(ctx context.Context, userID int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users u WHERE u.id = ? AND `+testAuthorEmail, userID).Scan(&n)
	return n > 0, err
}

// SitemapPosts lists visible post ids with their last change, newest first. 시험 계정 글은 뺀다.
func (r *CommunityRepo) SitemapPosts(ctx context.Context, limit int) ([]CommunitySitemapRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.updated_at FROM community_posts p JOIN users u ON u.id = p.user_id
		WHERE p.status = 'visible' AND NOT `+testAuthorEmail+` ORDER BY p.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CommunitySitemapRow
	for rows.Next() {
		var row CommunitySitemapRow
		if err := rows.Scan(&row.ID, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

type CommunitySitemapRow struct {
	ID        int64     `json:"id"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ------------------------------------------------------------- helpers

func communityPostIndex(posts []CommunityPostRow) ([]any, map[int64]int) {
	ids := make([]any, 0, len(posts))
	index := make(map[int64]int, len(posts))
	for i, p := range posts {
		ids = append(ids, p.ID)
		index[p.ID] = i
	}
	return ids, index
}

// CommunityAIOverview is a stored AI summary's head (LLM 0 — 배치가 써 둔 글).
type CommunityAIOverview struct {
	Text string
	At   string
}

// AIOverviews reads the stored AI overview for symbols (첫 300자).
func (r *CommunityRepo) AIOverviews(ctx context.Context, symbols []string) (map[string]CommunityAIOverview, error) {
	out := map[string]CommunityAIOverview{}
	if len(symbols) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(symbols))
	for _, s := range symbols {
		args = append(args, strings.ToUpper(s))
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, COALESCE(JSON_UNQUOTE(JSON_EXTRACT(payload, '$.overview')), ''),
		       DATE_FORMAT(analyzed_at, '%Y-%m-%d')
		  FROM stock_summary_narratives WHERE symbol IN (`+placeholders(len(symbols))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sym, text, at string
		if err := rows.Scan(&sym, &text, &at); err != nil {
			return nil, err
		}
		text = clipRunes(strings.TrimSpace(text), 300)
		if text != "" {
			out[strings.ToUpper(sym)] = CommunityAIOverview{Text: text, At: at}
		}
	}
	return out, rows.Err()
}

// ArticlesByIDs looks up news articles to attach (한국어 제목이 있으면 그것).
func (r *CommunityRepo) ArticlesByIDs(ctx context.Context, ids []string) (map[string]model.CommunityArticle, error) {
	out := map[string]model.CommunityArticle{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, COALESCE(NULLIF(headline_ko, ''), headline),
		       COALESCE(NULLIF(publisher_name, ''), source_id), url, published_at
		  FROM news_articles WHERE id IN (`+placeholders(len(ids))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var a model.CommunityArticle
		var pub time.Time
		if err := rows.Scan(&a.ArticleID, &a.Title, &a.Source, &a.URL, &pub); err != nil {
			return nil, err
		}
		a.Title = clipRunes(a.Title, 300)
		a.PublishedAt = &pub
		out[a.ArticleID] = a
	}
	return out, rows.Err()
}

// KnownSymbols returns the subset of symbols present in the stock catalog.
func (r *CommunityRepo) KnownSymbols(ctx context.Context, symbols []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(symbols) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(symbols))
	for _, s := range symbols {
		args = append(args, s)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT symbol FROM stocks WHERE symbol IN (`+placeholders(len(symbols))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out[strings.ToUpper(s)] = true
	}
	return out, rows.Err()
}
