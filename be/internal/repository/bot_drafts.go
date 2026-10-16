package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

/*
봇 초안 · 게시 계정 (105_bot_drafts.sql 머리글).

초안은 저장된 브리핑·종목 요약·실적 프리뷰·점수를 템플릿에 채운 글이다. 관리자가 고쳐서 게시 계정
(users.role='persona')으로 올린다. 게시 계정은 users의 일반 행이라 글·댓글 쪽은 바꿀 것이 없다.
*/

const (
	BotDraftCommunity = "community"
	BotDraftResearch  = "research"

	BotDraftStatusDraft     = "draft"
	BotDraftStatusPublished = "published"
	BotDraftStatusDiscarded = "discarded"

	RolePersona = "persona"
)

var ErrBotDraftNotFound = errors.New("bot draft not found")

type BotDraft struct {
	ID           int64        `json:"id"`
	Kind         string       `json:"kind"`
	SlotAt       time.Time    `json:"slotAt"`
	Symbol       string       `json:"symbol"`
	SymbolName   string       `json:"symbolName"`
	Market       string       `json:"market"`
	Length       string       `json:"length"`
	Reason       string       `json:"reason"`
	Title        string       `json:"title"`
	Body         string       `json:"body"`
	Articles     []BotArticle `json:"articles"`
	Status       string       `json:"status"`
	PublishedRef *int64       `json:"publishedRef,omitempty"`
	PublishedBy  *int64       `json:"publishedBy,omitempty"`
	CreatedAt    time.Time    `json:"createdAt"`
}

type BotDraftRepo struct{ db *sql.DB }

func NewBotDraftRepo(db *sql.DB) *BotDraftRepo { return &BotDraftRepo{db: db} }

func (r *BotDraftRepo) Insert(ctx context.Context, drafts []BotDraft) error {
	for _, d := range drafts {
		if d.Articles == nil {
			d.Articles = []BotArticle{}
		}
		ids, _ := json.Marshal(d.Articles)
		if _, err := r.db.ExecContext(ctx, `
			INSERT INTO bot_drafts (kind, slot_at, symbol, symbol_name, market, length, reason, title, body, articles)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			d.Kind, d.SlotAt.UTC(), d.Symbol, clipRunes(d.SymbolName, 100), d.Market, d.Length,
			clipRunes(d.Reason, 200), clipRunes(d.Title, 200), d.Body, string(ids)); err != nil {
			return fmt.Errorf("bot draft insert: %w", err)
		}
	}
	return nil
}

const botDraftCols = `id, kind, slot_at, symbol, symbol_name, market, length, reason, title, body, articles,
	status, published_ref, published_by, created_at`

func scanBotDraft(row interface{ Scan(...any) error }) (BotDraft, error) {
	var d BotDraft
	var ids []byte
	var ref, by sql.NullInt64
	if err := row.Scan(&d.ID, &d.Kind, &d.SlotAt, &d.Symbol, &d.SymbolName, &d.Market, &d.Length, &d.Reason,
		&d.Title, &d.Body, &ids, &d.Status, &ref, &by, &d.CreatedAt); err != nil {
		return d, err
	}
	_ = json.Unmarshal(ids, &d.Articles)
	if d.Articles == nil {
		d.Articles = []BotArticle{}
	}
	if ref.Valid {
		d.PublishedRef = &ref.Int64
	}
	if by.Valid {
		d.PublishedBy = &by.Int64
	}
	return d, nil
}

// List — 최신 회차부터. status가 빈 값이면 전부.
func (r *BotDraftRepo) List(ctx context.Context, kind, status string, limit int) ([]BotDraft, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	q := `SELECT ` + botDraftCols + ` FROM bot_drafts WHERE kind = ?`
	args := []any{kind}
	if status != "" {
		q += ` AND status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY slot_at DESC, id ASC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BotDraft{}
	for rows.Next() {
		d, err := scanBotDraft(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *BotDraftRepo) Get(ctx context.Context, id int64) (BotDraft, error) {
	d, err := scanBotDraft(r.db.QueryRowContext(ctx, `SELECT `+botDraftCols+` FROM bot_drafts WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return d, ErrBotDraftNotFound
	}
	return d, err
}

// Discard — 아직 초안인 것만 버린다(게시한 기록은 지우지 않는다).
func (r *BotDraftRepo) Discard(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE bot_drafts SET status = 'discarded' WHERE id = ? AND status = 'draft'`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrBotDraftNotFound
	}
	return nil
}

func (r *BotDraftRepo) MarkPublished(ctx context.Context, id, ref, by int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE bot_drafts SET status = 'published', published_ref = ?, published_by = ? WHERE id = ?`, ref, by, id)
	return err
}

// Prune — 게시하지 않은 초안 중 before보다 오래된 것을 지운다.
func (r *BotDraftRepo) Prune(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM bot_drafts WHERE status <> 'published' AND created_at < ?`, before.UTC())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RecentSymbols — since 이후 초안을 만든 (kind, 종목). 같은 종목이 회차마다 반복되지 않게.
func (r *BotDraftRepo) RecentSymbols(ctx context.Context, since time.Time) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT kind, symbol FROM bot_drafts WHERE created_at >= ?`, since.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var kind, sym string
		if err := rows.Scan(&kind, &sym); err != nil {
			return nil, err
		}
		out[kind+":"+sym] = true
	}
	return out, rows.Err()
}

// LatestCloseAI — 종목별 가장 최근 종가 행의 AI 한 줄(ai_summary)과 등락률. 한 줄은 LLM이 쓴 것만 —
// 규칙 문장(analysis_source='rule')은 숫자 나열이라 글 머리에 서면 기계처럼 읽힌다.
type BotCloseAI struct {
	TradeDate  string
	ChangeRate float64
	Summary    string
}

func (r *BotDraftRepo) LatestCloseAI(ctx context.Context, symbols []string) (map[string]BotCloseAI, error) {
	out := map[string]BotCloseAI{}
	if len(symbols) == 0 {
		return out, nil
	}
	in, args := symbolPlaceholders(symbols)
	rows, err := r.db.QueryContext(ctx, `
		SELECT d.symbol, DATE_FORMAT(d.trade_date, '%Y-%m-%d'), COALESCE(d.change_rate, 0),
		       IF(d.analysis_source = 'llm', d.ai_summary, '')
		  FROM stock_daily_closes d
		  JOIN (SELECT symbol, MAX(trade_date) td FROM stock_daily_closes
		         WHERE symbol IN (`+in+`) AND trade_date >= DATE_SUB(CURDATE(), INTERVAL 10 DAY)
		         GROUP BY symbol) m ON m.symbol = d.symbol AND m.td = d.trade_date`, args...)
	if err != nil {
		return nil, fmt.Errorf("bot close ai: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sym string
		var v BotCloseAI
		if err := rows.Scan(&sym, &v.TradeDate, &v.ChangeRate, &v.Summary); err != nil {
			return nil, err
		}
		v.Summary = strings.TrimSpace(v.Summary)
		out[strings.ToUpper(sym)] = v
	}
	return out, rows.Err()
}

// BotArticle — 초안에 붙일 최근 기사(번역 제목이 있으면 그것).
type BotArticle struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func (r *BotDraftRepo) RecentArticles(ctx context.Context, symbol string, since time.Time, limit int) ([]BotArticle, error) {
	// MEMBER OF 라야 idx_news_symbols_mv를 탄다(stock_timeline.go 머리글).
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, COALESCE(NULLIF(headline_ko, ''), headline)
		  FROM news_articles
		 WHERE ? MEMBER OF (symbols_json->'$') AND published_at >= ?
		 ORDER BY published_at DESC LIMIT ?`, strings.ToUpper(symbol), since.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("bot articles: %w", err)
	}
	defer rows.Close()
	out := []BotArticle{}
	for rows.Next() {
		var a BotArticle
		if err := rows.Scan(&a.ID, &a.Title); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- 게시 계정

type BotPersona struct {
	ID          int64     `json:"id"`
	DisplayName string    `json:"displayName"`
	Badge       string    `json:"badge"`
	Posts       int       `json:"posts"`
	Calls       int       `json:"calls"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (r *BotDraftRepo) Personas(ctx context.Context) ([]BotPersona, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, u.display_name, u.badge, u.created_at,
		       (SELECT COUNT(*) FROM community_posts p WHERE p.user_id = u.id AND p.status <> 'deleted'),
		       (SELECT COUNT(*) FROM research_calls c WHERE c.user_id = u.id)
		  FROM users u WHERE u.role = 'persona' ORDER BY u.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BotPersona{}
	for rows.Next() {
		var p BotPersona
		if err := rows.Scan(&p.ID, &p.DisplayName, &p.Badge, &p.CreatedAt, &p.Posts, &p.Calls); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreatePersona — 비밀번호가 빈 값이라 bcrypt 비교가 늘 실패한다(로그인 불가).
func (r *BotDraftRepo) CreatePersona(ctx context.Context, displayName, badge string) (int64, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	email := "persona-" + hex.EncodeToString(b[:]) + "@persona.invalid"
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO users (email, display_name, password_hash, role, badge) VALUES (?, ?, '', 'persona', ?)`,
		email, displayName, badge)
	if err != nil {
		if isDuplicateKey(err) {
			return 0, duplicateUserErr(err)
		}
		return 0, err
	}
	return res.LastInsertId()
}

func (r *BotDraftRepo) UpdatePersona(ctx context.Context, id int64, displayName, badge string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET display_name = ?, badge = ? WHERE id = ? AND role = 'persona'`, displayName, badge, id)
	if err != nil {
		if isDuplicateKey(err) {
			return duplicateUserErr(err)
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// 값이 같아도 0이다 — 게시 계정이 맞는지 따로 본다.
		if ok, err := r.IsPersona(ctx, id); err != nil || !ok {
			if err == nil {
				err = ErrUserNotFound
			}
			return err
		}
	}
	return nil
}

// IsPersona — 관리자가 「이 계정으로」 쓸 수 있는지의 유일한 문. 실제 사용자 id면 false.
func (r *BotDraftRepo) IsPersona(ctx context.Context, id int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id = ? AND role = 'persona'`, id).Scan(&n)
	return n > 0, err
}

// BotActivity — 게시 계정이 쓴 글에 다른 사람이 단 댓글(최근 것부터).
type BotActivity struct {
	Kind        string    `json:"kind"` // community | research
	CommentID   int64     `json:"commentId"`
	TargetID    int64     `json:"targetId"`
	TargetTitle string    `json:"targetTitle"`
	PersonaID   int64     `json:"personaId"`
	PersonaName string    `json:"personaName"`
	AuthorName  string    `json:"authorName"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (r *BotDraftRepo) Activity(ctx context.Context, since time.Time, limit int) ([]BotActivity, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT * FROM (
		  SELECT 'community' k, c.id, p.id tid, p.title, pu.id pid, pu.display_name pn, cu.display_name an, c.body, c.created_at
		    FROM community_comments c
		    JOIN community_posts p ON p.id = c.post_id
		    JOIN users pu ON pu.id = p.user_id AND pu.role = 'persona'
		    JOIN users cu ON cu.id = c.user_id
		   WHERE c.created_at >= ? AND c.status = 'visible' AND cu.role <> 'persona'
		  UNION ALL
		  SELECT 'research', c.id, rc.id, rc.title, pu.id, pu.display_name, cu.display_name, c.body, c.created_at
		    FROM research_comments c
		    JOIN research_calls rc ON rc.id = c.call_id
		    JOIN users pu ON pu.id = rc.user_id AND pu.role = 'persona'
		    JOIN users cu ON cu.id = c.user_id
		   WHERE c.created_at >= ? AND c.deleted_at IS NULL AND cu.role <> 'persona'
		) t ORDER BY created_at DESC LIMIT ?`, since.UTC(), since.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("bot activity: %w", err)
	}
	defer rows.Close()
	out := []BotActivity{}
	for rows.Next() {
		var a BotActivity
		if err := rows.Scan(&a.Kind, &a.CommentID, &a.TargetID, &a.TargetTitle, &a.PersonaID, &a.PersonaName,
			&a.AuthorName, &a.Body, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
