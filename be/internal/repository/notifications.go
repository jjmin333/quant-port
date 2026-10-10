package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

/*
앱 안 알림 (2026-10-10 사용자 요청 4 — 웹 푸시가 아니라 헤더 종 아이콘의 알림 칸).

생성기 셋은 전부 DB만 읽고 INSERT IGNORE로 쓴다 — (user_id, dedupe_key)가 유일하므로 몇 번 돌아도 같은 결과다.

	종목     stock_change_events(「오늘의 변화」) × 관심 종목 — 급등락·실적·공시·내부자, 점수 급변(중요도 2 이상)
	단지     새로 적재된 re_trades(id 커서) × 관심 단지 — 단지마다 「새 실거래 N건」, 같은 평형 3년 최고가를 넘으면 「신고가」
	금융상품 공시월이 바뀐 상품(상품별 커서) × 관심 상품 — 기본금리가 바뀌었을 때만
*/

type NotificationRepo struct{ db *sql.DB }

func NewNotificationRepo(db *sql.DB) *NotificationRepo { return &NotificationRepo{db: db} }

// List returns the newest notifications and the unread count.
func (r *NotificationRepo) List(ctx context.Context, userID int64, limit int) ([]model.Notification, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, kind, title, body, href, created_at, read_at IS NOT NULL
		  FROM user_notifications WHERE user_id = ? ORDER BY id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []model.Notification{}
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.Kind, &n.Title, &n.Body, &n.Href, &n.CreatedAt, &n.Read); err != nil {
			return nil, 0, err
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	unread, err := r.Unread(ctx, userID)
	return out, unread, err
}

func (r *NotificationRepo) Unread(ctx context.Context, userID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_notifications WHERE user_id = ? AND read_at IS NULL`, userID).Scan(&n)
	return n, err
}

// MarkRead marks the given ids read — or every unread one when ids is empty.
func (r *NotificationRepo) MarkRead(ctx context.Context, userID int64, ids []int64) error {
	q := `UPDATE user_notifications SET read_at = CURRENT_TIMESTAMP(3) WHERE user_id = ? AND read_at IS NULL`
	args := []any{userID}
	if len(ids) > 0 {
		if len(ids) > 200 {
			ids = ids[:200]
		}
		q += ` AND id IN (` + placeholders(len(ids)) + `)`
		for _, id := range ids {
			args = append(args, id)
		}
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

// Prune drops notifications older than 90 days (a bounded slice per call).
func (r *NotificationRepo) Prune(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM user_notifications WHERE created_at < NOW() - INTERVAL 90 DAY LIMIT 5000`)
	return err
}

/*
GenerateStock — 관심 종목의 「오늘의 변화」.

dedupe 키는 사건 id가 아니라 (종목·대상일·종류·세부)다 — 변화 감지 배치는 같은 날을 다시 돌 때 그날 행을 지우고
새로 넣으므로 id가 바뀐다. 담기 전에 생긴 사건은 알리지 않는다(e.created_at >= w.created_at).
*/
func (r *NotificationRepo) GenerateStock(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT IGNORE INTO user_notifications (user_id, kind, dedupe_key, title, body, href, created_at)
		SELECT w.user_id, 'stock',
		       CONCAT('chg:', e.symbol, ':', DATE_FORMAT(e.detected_for, '%Y%m%d'), ':', e.kind, ':', e.subtype),
		       LEFT(CONCAT(COALESCE(NULLIF(s.name, ''), e.symbol), ' · ', e.title), 200),
		       '관심 종목', CONCAT('/stocks/', e.symbol), e.created_at
		  FROM stock_change_events e
		  JOIN watchlist_items w ON w.symbol = e.symbol AND w.created_at <= e.created_at
		  LEFT JOIN stocks s ON s.symbol = e.symbol
		 WHERE e.created_at >= NOW() - INTERVAL 2 DAY
		   AND (e.kind IN ('price_move', 'earnings', 'disclosure', 'insider')
		        OR (e.kind = 'score_jump' AND e.importance >= 2))`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *NotificationRepo) cursor(ctx context.Context, name string) (string, bool, error) {
	var v string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM notify_cursors WHERE name = ?`, name).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

func (r *NotificationRepo) setCursor(ctx context.Context, name, value string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO notify_cursors (name, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value = VALUES(value)`, name, value)
	return err
}

/*
GenerateReTrades — 관심 단지에 새로 적재된 매매.

re_trades에는 적재 시각이 없어 id로 센다(AUTO_INCREMENT, 새 행이 더 크다). 커서가 없으면 지금 최대 id만 적고 끝난다.
매매(sale)·해제 아님만. 단지마다 한 줄 — 「새 실거래 3건 · 최근 10-02 84㎡ 23.5억」, 같은 평형 3년 최고가를 넘으면 신고가.
*/
func (r *NotificationRepo) GenerateReTrades(ctx context.Context) (int64, error) {
	var maxID sql.NullInt64
	if err := r.db.QueryRowContext(ctx, `SELECT MAX(id) FROM re_trades`).Scan(&maxID); err != nil || !maxID.Valid {
		return 0, err
	}
	cur, ok, err := r.cursor(ctx, "re_trades_id")
	if err != nil {
		return 0, err
	}
	top := strconv.FormatInt(maxID.Int64, 10)
	if !ok {
		return 0, r.setCursor(ctx, "re_trades_id", top)
	}
	from, _ := strconv.ParseInt(cur, 10, 64)
	if from >= maxID.Int64 {
		return 0, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT w.user_id, w.skey, w.lawd_cd, w.name, t.deal_date, t.area, t.price, t.trade_id
		  FROM (SELECT complex_key, DATE_FORMAT(deal_date, '%Y-%m-%d') deal_date, area, price, id trade_id
		          FROM re_trades
		         WHERE id > ? AND id <= ? AND deal_type = 'sale' AND cancelled = 0 AND price IS NOT NULL
		           AND complex_key IN (SELECT skey FROM re_watchlist WHERE kind = 'complex')) t
		  JOIN re_watchlist w ON w.kind = 'complex' AND w.skey = t.complex_key
		 ORDER BY w.user_id, w.skey, t.deal_date DESC, t.trade_id DESC`, from, maxID.Int64)
	if err != nil {
		return 0, err
	}
	type group struct {
		userID              int64
		key, lawd, name     string
		count               int
		lastDate            string
		lastArea            float64
		lastPrice, lastID   int64
	}
	var groups []*group
	byKey := map[string]*group{}
	for rows.Next() {
		var g group
		var date string
		var area float64
		var price, id int64
		if err := rows.Scan(&g.userID, &g.key, &g.lawd, &g.name, &date, &area, &price, &id); err != nil {
			rows.Close()
			return 0, err
		}
		k := fmt.Sprintf("%d|%s", g.userID, g.key)
		if e, ok := byKey[k]; ok {
			e.count++
			continue
		}
		g.count, g.lastDate, g.lastArea, g.lastPrice, g.lastID = 1, date, area, price, id
		byKey[k] = &g
		groups = append(groups, &g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	var made int64
	for _, g := range groups {
		// 신고가 — 같은 단지·같은 평형(전용 정수 ㎡)의 그 이전 3년 매매 최고가보다 비싸면.
		var prevMax sql.NullInt64
		_ = r.db.QueryRowContext(ctx, `
			SELECT MAX(price) FROM re_trades
			 WHERE complex_key = ? AND deal_type = 'sale' AND cancelled = 0 AND FLOOR(area) = ?
			   AND id <> ? AND deal_date >= ? - INTERVAL 3 YEAR AND deal_date <= ?`,
			g.key, math.Floor(g.lastArea+1e-9), g.lastID, g.lastDate, g.lastDate).Scan(&prevMax)
		high := prevMax.Valid && g.lastPrice > prevMax.Int64
		name := g.name
		if name == "" {
			name = "관심 단지"
		}
		title := fmt.Sprintf("%s 새 실거래 %d건", name, g.count)
		if high {
			title = fmt.Sprintf("%s 신고가 · 새 실거래 %d건", name, g.count)
		}
		body := fmt.Sprintf("최근 %s · %.0f㎡ · %s", g.lastDate[5:], g.lastArea, eokLabel(g.lastPrice))
		href := "/real-estate?lawd=" + g.lawd + "&complex=" + g.key
		res, err := r.db.ExecContext(ctx, `
			INSERT IGNORE INTO user_notifications (user_id, kind, dedupe_key, title, body, href)
			VALUES (?, 're_trade', ?, ?, ?, ?)`,
			g.userID, fmt.Sprintf("re:%s:%d", g.key, g.lastID), title, body, href)
		if err != nil {
			return made, err
		}
		n, _ := res.RowsAffected()
		made += n
	}
	return made, r.setCursor(ctx, "re_trades_id", top)
}

// eokLabel — 만원 단위 가격을 「23.5억」·「9,800만」으로.
func eokLabel(manwon int64) string {
	if manwon >= 10000 {
		return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%.2f", float64(manwon)/10000), "0"), ".0") + "억"
	}
	return fmt.Sprintf("%d만", manwon)
}

/*
GenerateFinRates — 관심 금융상품의 공시월이 바뀌었고 기본금리가 달라졌을 때.

상품마다 커서(마지막으로 본 공시월)를 둔다. 처음 보는 상품은 커서만 적는다 — 담은 순간 지난달 변동이 알림으로 오지 않게.
*/
func (r *NotificationRepo) GenerateFinRates(ctx context.Context) (int64, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT p.id, p.dcls_month, p.product_name, p.company_name
		  FROM fin_products p JOIN fin_watchlist w ON w.product_id = p.id`)
	if err != nil {
		return 0, err
	}
	type prod struct {
		id                  int64
		month, name, company string
	}
	var prods []prod
	for rows.Next() {
		var p prod
		if err := rows.Scan(&p.id, &p.month, &p.name, &p.company); err != nil {
			rows.Close()
			return 0, err
		}
		prods = append(prods, p)
	}
	rows.Close()

	var made int64
	for _, p := range prods {
		name := "fin:" + strconv.FormatInt(p.id, 10)
		cur, ok, err := r.cursor(ctx, name)
		if err != nil {
			return made, err
		}
		if ok && cur == p.month {
			continue
		}
		if ok {
			var now, prev sql.NullFloat64
			_ = r.db.QueryRowContext(ctx, `SELECT base_rate FROM fin_rate_history WHERE product_id = ? AND dcls_month = ?`, p.id, p.month).Scan(&now)
			_ = r.db.QueryRowContext(ctx, `SELECT base_rate FROM fin_rate_history WHERE product_id = ? AND dcls_month < ? ORDER BY dcls_month DESC LIMIT 1`, p.id, p.month).Scan(&prev)
			if now.Valid && prev.Valid && math.Abs(now.Float64-prev.Float64) >= 0.005 {
				arrow := "▲"
				if now.Float64 < prev.Float64 {
					arrow = "▼"
				}
				title := fmt.Sprintf("%s 기본금리 %s %.2f%% → %.2f%%", p.name, arrow, prev.Float64, now.Float64)
				res, err := r.db.ExecContext(ctx, `
					INSERT IGNORE INTO user_notifications (user_id, kind, dedupe_key, title, body, href)
					SELECT user_id, 'fin_rate', ?, ?, ?, ? FROM fin_watchlist WHERE product_id = ?`,
					fmt.Sprintf("fin:%d:%s", p.id, p.month), title, p.company+" · "+p.month[:4]+"."+p.month[4:]+" 공시",
					fmt.Sprintf("/finance/products/%d", p.id), p.id)
				if err != nil {
					return made, err
				}
				n, _ := res.RowsAffected()
				made += n
			}
		}
		if err := r.setCursor(ctx, name, p.month); err != nil {
			return made, err
		}
	}
	return made, nil
}

// AssetPost is one community post or report that tags an asset.
type AssetPost struct {
	Type       string    `json:"type"` // community | research
	ID         int64     `json:"id"`
	Title      string    `json:"title"`
	AuthorName string    `json:"authorName"`
	CreatedAt  time.Time `json:"createdAt"`
}

/*
AssetPosts — 이 단지·지역·상품·금리를 태그한 공개 글(커뮤니티 + 리포트), 최신순. 숨김·삭제·비공개는 뺀다.
(2026-10-10 사용자 요청 4 — 호갱노노의 단지 이야기처럼 자산 화면에서 글을 모아 본다.)
*/
func (r *NotificationRepo) AssetPosts(ctx context.Context, kind, key string, limit int) ([]AssetPost, error) {
	if limit <= 0 || limit > 30 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT * FROM (
		  SELECT 'community' AS t, p.id, p.title, COALESCE(u.display_name, ''), p.created_at
		    FROM content_asset_mentions m
		    JOIN community_posts p ON p.id = m.target_id AND p.status = 'visible'
		    LEFT JOIN users u ON u.id = p.user_id
		   WHERE m.kind = ? AND m.skey = ? AND m.target_type = 'community_post'
		  UNION ALL
		  SELECT 'research', c.id, c.title, COALESCE(u.display_name, ''), c.published_at
		    FROM content_asset_mentions m
		    JOIN research_calls c ON c.id = m.target_id AND c.visibility = 'public' AND c.hidden_at IS NULL
		    LEFT JOIN users u ON u.id = c.user_id
		   WHERE m.kind = ? AND m.skey = ? AND m.target_type = 'research_call'
		) x ORDER BY 5 DESC LIMIT ?`, kind, key, kind, key, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AssetPost{}
	for rows.Next() {
		var p AssetPost
		if err := rows.Scan(&p.Type, &p.ID, &p.Title, &p.AuthorName, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
