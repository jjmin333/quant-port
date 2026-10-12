// Command seed-preview fills the community and research-report tables with sample data for screen
// testing (2026-10-12 사용자 요청 — 커뮤니티·리포트가 관리자 전용인 동안, 글이 여러 종목에 흩어져 있을 때
// 화면이 어떻게 보이는지 보려고).
//
//	go run ./cmd/seed-preview -count          # 지금 행 수만
//	go run ./cmd/seed-preview -reset -seed    # 커뮤니티·리포트 전부 지우고 2026-08-01부터 오늘까지 다시 채움
//
// 계정은 seed-NN@quant-port.test — 비밀번호가 없어 로그인할 수 없고, 예약 도메인이라 가입 지표에서 빠진다.
// 글에 붙는 가격·등락·점수·차트는 오늘 값이 아니라 **글 날짜의 실제 종가·점수**다.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

var communityTables = []string{
	"community_reports", "community_views", "community_reactions", "community_comments",
	"community_post_images", "community_post_articles", "community_post_symbols", "community_posts",
}

var researchTables = []string{
	"research_comment_likes", "research_comments", "research_ratings", "research_reactions",
	"research_views", "research_reports", "research_reviews", "research_revisions", "research_calls",
}

var seedSymbols = []string{
	"005930", "000660", "035420", "035720", "005380", "000270", "373220", "207940", "068270",
	"105560", "055550", "012450", "042700", "086520", "247540", "329180", "010140", "064350",
	"NVDA", "AAPL", "TSLA", "MSFT", "AMZN", "GOOGL", "META", "PLTR", "AMD", "AVGO", "TSM", "NFLX",
}

// 미국 종목 위주로 보는 계정(인덱스).
var usFocused = map[int]bool{4: true, 8: true, 16: true, 29: true, 36: true}

const seedEmailLike = "seed-%@quant-port.test"

func main() {
	countOnly := flag.Bool("count", false, "print row counts only")
	reset := flag.Bool("reset", false, "delete every community post and research report (and seed accounts)")
	seed := flag.Bool("seed", false, "insert sample accounts, posts and reports")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	m, err := repository.NewMySQLForCLI(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defer m.Close()
	db := m.DB()
	ctx := context.Background()

	printCounts(ctx, db)
	if *countOnly || (!*reset && !*seed) {
		return
	}
	if *reset {
		if err := resetAll(ctx, db); err != nil {
			log.Fatalf("reset: %v", err)
		}
		log.Println("reset done")
	}
	if *seed {
		if err := seedAll(ctx, db); err != nil {
			log.Fatalf("seed: %v", err)
		}
		log.Println("seed done")
	}
	printCounts(ctx, db)
}

func printCounts(ctx context.Context, db *sql.DB) {
	for _, t := range append(append([]string{}, communityTables...), researchTables...) {
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+t).Scan(&n); err != nil {
			log.Printf("  %-26s ? (%v)", t, err)
			continue
		}
		log.Printf("  %-26s %d", t, n)
	}
	var n int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email LIKE ?`, seedEmailLike).Scan(&n)
	log.Printf("  %-26s %d", "seed users", n)
}

func resetAll(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, t := range append(append([]string{}, communityTables...), researchTables...) {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+t); err != nil {
			return fmt.Errorf("%s: %w", t, err)
		}
	}
	for _, t := range []string{"content_mentions", "content_asset_mentions"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+t+" WHERE target_type IN ('community_post', 'research_call')"); err != nil {
			return fmt.Errorf("%s: %w", t, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE email LIKE ?`, seedEmailLike); err != nil {
		return fmt.Errorf("users: %w", err)
	}
	return tx.Commit()
}

// ---------------------------------------------------------------- data

type closeRow struct {
	date   string
	close  float64
	prev   *float64
	chg    *float64
	volume int64
	score  *int
	ai     string
}

type seedStock struct {
	symbol, name, market, sector string
	assetType                    model.QuoteAssetType
	currency                     model.QuoteCurrency
	closes                       []closeRow
}

func (s *seedStock) asOf(date string) *closeRow {
	i := sort.Search(len(s.closes), func(i int) bool { return s.closes[i].date > date })
	if i == 0 {
		return nil
	}
	return &s.closes[i-1]
}

func (s *seedStock) us() bool { return s.assetType == model.AssetUS || s.assetType == model.AssetETF }

type seedUser struct {
	id     int64
	name   string
	joined time.Time
	favs   []int
	weight float64
}

func loadStocks(ctx context.Context, db *sql.DB) ([]*seedStock, error) {
	stocks := repository.NewStockRepo(db)
	var out []*seedStock
	for _, sym := range seedSymbols {
		st, err := stocks.GetBySymbol(ctx, sym)
		if err != nil || st == nil {
			log.Printf("skip %s: not in catalog", sym)
			continue
		}
		s := &seedStock{symbol: sym, name: st.Name, market: st.Market, assetType: st.AssetType, currency: st.Currency, sector: seedSectors[sym]}
		if s.currency == "" {
			s.currency = model.CurrencyKRW
		}
		rows, err := db.QueryContext(ctx, `
			SELECT d.trade_date, d.close_price, d.prev_close, d.change_rate, d.volume,
			       COALESCE(sc.score, d.score), COALESCE(d.ai_summary, '')
			  FROM stock_daily_closes d
			  LEFT JOIN stock_scores sc ON sc.symbol = d.symbol AND sc.trade_date = d.trade_date
			 WHERE d.symbol = ? AND d.trade_date >= '2026-06-01'
			 ORDER BY d.trade_date`, sym)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var (
				d         time.Time
				r         closeRow
				prev, chg sql.NullFloat64
				score     sql.NullInt64
			)
			if err := rows.Scan(&d, &r.close, &prev, &chg, &r.volume, &score, &r.ai); err != nil {
				rows.Close()
				return nil, err
			}
			r.date = d.Format("2006-01-02")
			if prev.Valid {
				v := prev.Float64
				r.prev = &v
			}
			if chg.Valid {
				v := chg.Float64
				r.chg = &v
			}
			if score.Valid {
				v := int(score.Int64)
				r.score = &v
			}
			if r.close > 0 {
				s.closes = append(s.closes, r)
			}
		}
		rows.Close()
		if len(s.closes) == 0 {
			log.Printf("skip %s: no closes", sym)
			continue
		}
		fillScoreGaps(s.closes)
		out = append(out, s)
	}
	return out, nil
}

// fillScoreGaps copies the nearest day's score (within 10 rows) into days that have none — the score
// batch history starts late August for most symbols, and a sample without a score hides the score chip.
// ponytail: 가상 데이터라 이웃 날 점수로 채운다. 그날 실제 점수가 필요하면 그 날짜 점수 배치를 다시 돌릴 것.
func fillScoreGaps(rows []closeRow) {
	for i := range rows {
		if rows[i].score != nil {
			continue
		}
		for d := 1; d <= 10; d++ {
			if j := i + d; j < len(rows) && rows[j].score != nil {
				rows[i].score = rows[j].score
				break
			}
			if j := i - d; j >= 0 && rows[j].score != nil {
				rows[i].score = rows[j].score
				break
			}
		}
	}
}

// ---------------------------------------------------------------- seed

func seedAll(ctx context.Context, db *sql.DB) error {
	rng := rand.New(rand.NewSource(20261012))
	now := time.Now()
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, model.KST)

	stocks, err := loadStocks(ctx, db)
	if err != nil {
		return err
	}
	if len(stocks) < 5 {
		return fmt.Errorf("only %d stocks with closes", len(stocks))
	}
	var krIdx, usIdx []int
	for i, s := range stocks {
		if s.us() {
			usIdx = append(usIdx, i)
		} else {
			krIdx = append(krIdx, i)
		}
	}

	users, err := insertUsers(ctx, db, rng, start, len(stocks), krIdx, usIdx)
	if err != nil {
		return err
	}
	log.Printf("users: %d", len(users))

	posts, err := seedCommunity(ctx, db, rng, users, stocks, start, now)
	if err != nil {
		return err
	}
	log.Printf("community posts: %d", posts)

	calls, err := seedResearch(ctx, db, rng, users, stocks, start, now)
	if err != nil {
		return err
	}
	log.Printf("research reports: %d", calls)
	return nil
}

func insertUsers(ctx context.Context, db *sql.DB, rng *rand.Rand, start time.Time, nStocks int, krIdx, usIdx []int) ([]*seedUser, error) {
	users := make([]*seedUser, 0, len(seedNicknames))
	for i, nick := range seedNicknames {
		// 앞 절반은 8월 전에, 나머지는 9월 하순까지 차례로 들어온다 — 글 수가 시간에 따라 늘어 보이게.
		var joined time.Time
		if i < len(seedNicknames)/2 {
			joined = start.Add(-time.Duration(rng.Intn(20*24)+24) * time.Hour)
		} else {
			joined = start.Add(time.Duration(rng.Intn(55*24)) * time.Hour)
		}
		joined = joined.Add(time.Duration(rng.Intn(3600)) * time.Second)
		name := nick
		var id int64
		for attempt := 0; ; attempt++ {
			res, err := db.ExecContext(ctx, `
				INSERT INTO users (email, display_name, password_hash, role, created_at, updated_at)
				VALUES (?, ?, NULL, 'user', ?, ?)`,
				fmt.Sprintf("seed-%02d@quant-port.test", i+1), name, joined, joined)
			if err == nil {
				id, _ = res.LastInsertId()
				break
			}
			if attempt < 3 && strings.Contains(err.Error(), "Duplicate entry") && strings.Contains(err.Error(), "display") {
				name = nick + strconv.Itoa(rng.Intn(90)+10)
				continue
			}
			return nil, fmt.Errorf("user %s: %w", nick, err)
		}
		pool := krIdx
		if usFocused[i] || (rng.Float64() < 0.3 && len(usIdx) > 0) {
			pool = usIdx
		}
		favs := make([]int, 0, 5)
		for k := 0; k < 3+rng.Intn(3); k++ {
			favs = append(favs, pool[rng.Intn(len(pool))])
		}
		if rng.Float64() < 0.4 {
			favs = append(favs, rng.Intn(nStocks))
		}
		users = append(users, &seedUser{id: id, name: name, joined: joined, favs: favs, weight: 1 / math.Pow(float64(i%17)+1, 0.7)})
	}
	return users, nil
}

func pickUser(rng *rand.Rand, users []*seedUser, at time.Time, except int64) *seedUser {
	total := 0.0
	for _, u := range users {
		if u.joined.Before(at) && u.id != except {
			total += u.weight
		}
	}
	if total == 0 {
		return nil
	}
	x := rng.Float64() * total
	for _, u := range users {
		if !u.joined.Before(at) || u.id == except {
			continue
		}
		x -= u.weight
		if x <= 0 {
			return u
		}
	}
	return nil
}

// 글이 몰리는 시간대 — 출근길·점심·장 마감 직후·밤(미장).
var postHours = []int{7, 8, 8, 9, 10, 12, 12, 13, 15, 15, 16, 18, 20, 21, 22, 22, 23, 23, 0}

func randomTimeOn(rng *rand.Rand, day time.Time) time.Time {
	h := postHours[rng.Intn(len(postHours))]
	return day.Add(time.Duration(h)*time.Hour + time.Duration(rng.Intn(3600))*time.Second)
}

func kstDate(t time.Time) string { return t.In(model.KST).Format("2006-01-02") }

func fmtPrice(c model.QuoteCurrency, v float64) string {
	if c == model.CurrencyUSD {
		return fmt.Sprintf("$%.2f", v)
	}
	s := strconv.FormatInt(int64(math.Round(v)), 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String() + "원"
}

func fmtChg(v *float64) string {
	if v == nil {
		return "보합"
	}
	return fmt.Sprintf("%+.1f%%", *v)
}

func scoreText(s *int) (string, string) {
	if s == nil {
		return "—", "—"
	}
	return strconv.Itoa(*s), model.ScoreGrade(*s)
}

func fill(tpl string, s, s2 *seedStock, r, r2 *closeRow, t time.Time) string {
	score, grade := "—", "—"
	price, chg := "", ""
	name, name2, score2 := "", "", "—"
	if s != nil && r != nil {
		name, price, chg = s.name, fmtPrice(s.currency, r.close), fmtChg(r.chg)
		score, grade = scoreText(r.score)
	}
	if s2 != nil && r2 != nil {
		name2 = s2.name
		score2, _ = scoreText(r2.score)
	}
	return strings.NewReplacer(
		"{name2}", name2, "{score2}", score2, "{name}", name, "{price}", price, "{chg}", chg,
		"{score}", score, "{grade}", grade, "{month}", strconv.Itoa(int(t.In(model.KST).Month())),
	).Replace(tpl)
}

func seedCommunity(ctx context.Context, db *sql.DB, rng *rand.Rand, users []*seedUser, stocks []*seedStock, start, now time.Time) (int, error) {
	total := int(now.Sub(start).Hours()/24) + 1
	posts := 0
	for d := 0; d < total; d++ {
		day := start.AddDate(0, 0, d)
		n := 1 + int(float64(d)/float64(total)*4) + rng.Intn(3)
		if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
			n = (n + 1) / 2
		}
		for k := 0; k < n; k++ {
			at := randomTimeOn(rng, day)
			if at.After(now) {
				continue
			}
			author := pickUser(rng, users, at, 0)
			if author == nil {
				continue
			}
			var (
				tp     textPair
				s1, s2 *seedStock
				r1, r2 *closeRow
				date   = kstDate(at)
				roll   = rng.Float64()
			)
			pickStock := func() *seedStock {
				if rng.Float64() < 0.7 {
					return stocks[author.favs[rng.Intn(len(author.favs))]]
				}
				return stocks[rng.Intn(len(stocks))]
			}
			switch {
			case roll < 0.12:
				tp = postsGeneral[rng.Intn(len(postsGeneral))]
			case roll < 0.22:
				s1, s2 = pickStock(), pickStock()
				if s1 == s2 {
					s2 = stocks[(indexOf(stocks, s1)+1)%len(stocks)]
				}
				r1, r2 = s1.asOf(date), s2.asOf(date)
				if r1 == nil || r2 == nil {
					continue
				}
				tp = postsCompare[rng.Intn(len(postsCompare))]
			default:
				s1 = pickStock()
				r1 = s1.asOf(date)
				if r1 == nil {
					continue
				}
				pool := postsFlat
				if r1.chg != nil && rng.Float64() < 0.85 {
					switch {
					case *r1.chg >= 2.5:
						pool = postsUp
					case *r1.chg <= -2.5:
						pool = postsDown
					}
				}
				tp = pool[rng.Intn(len(pool))]
			}
			title := fill(tp.title, s1, s2, r1, r2, at)
			body := fill(tp.body, s1, s2, r1, r2, at)

			res, err := db.ExecContext(ctx, `
				INSERT INTO community_posts (user_id, title, topic, body, created_at, updated_at)
				VALUES (?, ?, 'stock', ?, ?, ?)`, author.id, title, body, at, at)
			if err != nil {
				return posts, fmt.Errorf("post: %w", err)
			}
			postID, _ := res.LastInsertId()
			for pos, pair := range []struct {
				s *seedStock
				r *closeRow
			}{{s1, r1}, {s2, r2}} {
				if pair.s == nil {
					continue
				}
				if _, err := db.ExecContext(ctx, `
					INSERT INTO community_post_symbols
					  (post_id, symbol, position, snap_price, snap_currency, snap_change_rate, snap_score, snap_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
					postID, pair.s.symbol, pos, pair.r.close, string(pair.s.currency), pair.r.chg, pair.r.score, at); err != nil {
					return posts, fmt.Errorf("post symbol: %w", err)
				}
			}

			popular := rng.Float64() < 0.12
			nComments := rng.Intn(4)
			if popular {
				nComments = 6 + rng.Intn(10)
			}
			sentiment := commentsGeneral
			if r1 != nil && r1.chg != nil && *r1.chg >= 2.5 {
				sentiment = append(append([]string{}, commentsGeneral...), commentsUp...)
			} else if r1 != nil && r1.chg != nil && *r1.chg <= -2.5 {
				sentiment = append(append([]string{}, commentsGeneral...), commentsDown...)
			}
			var commentIDs []int64
			comments := 0
			for c := 0; c < nComments; c++ {
				cat := at.Add(time.Duration(5+rng.Intn(36*60)) * time.Minute)
				if cat.After(now) {
					continue
				}
				var parent *int64
				text := sentiment[rng.Intn(len(sentiment))]
				who := pickUser(rng, users, cat, author.id)
				if len(commentIDs) > 0 && rng.Float64() < 0.25 {
					p := commentIDs[rng.Intn(len(commentIDs))]
					parent = &p
					text = fill(replies[rng.Intn(len(replies))], s1, nil, r1, nil, cat)
					if strings.Contains(text, "  근처") || strings.HasPrefix(text, " 근처") {
						text = "맞아요 ㅎㅎ"
					}
					if rng.Float64() < 0.5 {
						who = author // 작성자가 답글을 다는 게 흔하다
					}
				}
				if who == nil {
					continue
				}
				res, err := db.ExecContext(ctx, `
					INSERT INTO community_comments (post_id, parent_id, user_id, body, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?)`, postID, parent, who.id, text, cat, cat)
				if err != nil {
					return posts, fmt.Errorf("comment: %w", err)
				}
				cid, _ := res.LastInsertId()
				commentIDs = append(commentIDs, cid)
				comments++
				if likes := likeUsers(rng, users, cat, who.id, rng.Intn(4)); len(likes) > 0 {
					if err := insertCommunityLikes(ctx, db, model.CommunityTargetComment, cid, likes, cat, now, rng); err != nil {
						return posts, err
					}
					if _, err := db.ExecContext(ctx, `UPDATE community_comments SET like_count = ?, updated_at = created_at WHERE id = ?`, len(likes), cid); err != nil {
						return posts, err
					}
				}
			}

			nLikes := rng.Intn(6) + comments/2
			if popular {
				nLikes += 5 + rng.Intn(20)
			}
			likes := likeUsers(rng, users, at, author.id, nLikes)
			if err := insertCommunityLikes(ctx, db, model.CommunityTargetPost, postID, likes, at, now, rng); err != nil {
				return posts, err
			}
			views := len(likes)*(6+rng.Intn(10)) + comments*4 + 5 + rng.Intn(40)
			if _, err := db.ExecContext(ctx, `
				UPDATE community_posts SET like_count = ?, comment_count = ?, view_count = ?, updated_at = created_at
				 WHERE id = ?`, len(likes), comments, views, postID); err != nil {
				return posts, err
			}
			posts++
		}
	}
	return posts, nil
}

func indexOf(stocks []*seedStock, s *seedStock) int {
	for i, x := range stocks {
		if x == s {
			return i
		}
	}
	return 0
}

// likeUsers picks up to n distinct users who had joined by `at`, never the author.
func likeUsers(rng *rand.Rand, users []*seedUser, at time.Time, author int64, n int) []*seedUser {
	seen := map[int64]bool{author: true}
	var out []*seedUser
	for tries := 0; len(out) < n && tries < n*4; tries++ {
		u := users[rng.Intn(len(users))]
		if seen[u.id] || !u.joined.Before(at) {
			continue
		}
		seen[u.id] = true
		out = append(out, u)
	}
	return out
}

func insertCommunityLikes(ctx context.Context, db *sql.DB, targetType string, targetID int64, likers []*seedUser, after, now time.Time, rng *rand.Rand) error {
	for _, u := range likers {
		at := after.Add(time.Duration(1+rng.Intn(48*60)) * time.Minute)
		if at.After(now) {
			at = now
		}
		if _, err := db.ExecContext(ctx, `
			INSERT IGNORE INTO community_reactions (target_type, target_id, user_id, created_at) VALUES (?, ?, ?, ?)`,
			targetType, targetID, u.id, at); err != nil {
			return fmt.Errorf("reaction: %w", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------- research

func roundPrice(c model.QuoteCurrency, v float64) float64 {
	if c == model.CurrencyUSD {
		return math.Round(v*100) / 100
	}
	step := 10.0
	switch {
	case v >= 100000:
		step = 500
	case v >= 10000:
		step = 50
	}
	return math.Round(v/step) * step
}

func benchmarkFor(s *seedStock) string {
	if s.us() {
		return "sp500"
	}
	if strings.Contains(strings.ToUpper(s.market), "KOSDAQ") {
		return "kosdaq"
	}
	return "kospi"
}

func seedResearch(ctx context.Context, db *sql.DB, rng *rand.Rand, users []*seedUser, stocks []*seedStock, start, now time.Time) (int, error) {
	repo := repository.NewResearchRepo(db)
	candles := repository.NewCandleBarRepo(db)
	span := now.Sub(start)
	const target = 45

	times := make([]time.Time, 0, target)
	for i := 0; i < target; i++ {
		day := start.Add(time.Duration(rng.Int63n(int64(span - 24*time.Hour))))
		day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, model.KST)
		times = append(times, randomTimeOn(rng, day))
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })

	lastCall := map[string]time.Time{} // user|symbol → 30일 쿨다운을 흉내 낸다
	made := 0
	for _, at := range times {
		if at.After(now) {
			continue
		}
		var author *seedUser
		for tries := 0; tries < 10 && author == nil; tries++ {
			u := users[seedAnalysts[rng.Intn(len(seedAnalysts))]]
			if u.joined.Before(at) {
				author = u
			}
		}
		if author == nil {
			continue
		}
		var s *seedStock
		var r *closeRow
		for tries := 0; tries < 10; tries++ {
			cand := stocks[rng.Intn(len(stocks))]
			if rng.Float64() < 0.5 {
				cand = stocks[author.favs[rng.Intn(len(author.favs))]]
			}
			key := fmt.Sprintf("%d|%s", author.id, cand.symbol)
			if last, ok := lastCall[key]; ok && at.Sub(last) < 30*24*time.Hour {
				continue
			}
			if row := cand.asOf(kstDate(at)); row != nil {
				s, r = cand, row
				lastCall[key] = at
				break
			}
		}
		if s == nil {
			continue
		}

		dir := model.DirectionBuy
		switch roll := rng.Float64(); {
		case roll < 0.12:
			dir = model.DirectionHold
		case roll < 0.27:
			dir = model.DirectionSell
		}
		base := r.close
		var targetPx, stopPx *float64
		upside := 0.0
		switch dir {
		case model.DirectionBuy:
			upside = 0.12 + rng.Float64()*0.23
			t := roundPrice(s.currency, base*(1+upside))
			st := roundPrice(s.currency, base*(1-0.07-rng.Float64()*0.05))
			targetPx, stopPx = &t, &st
		case model.DirectionSell:
			upside = -(0.10 + rng.Float64()*0.15)
			t := roundPrice(s.currency, base*(1+upside))
			st := roundPrice(s.currency, base*(1+0.07+rng.Float64()*0.05))
			targetPx, stopPx = &t, &st
		}
		horizon := []int{30, 60, 90}[rng.Intn(3)]

		text := genericText
		if st, ok := sectorTexts[s.sector]; ok {
			text = st
		}
		thesis := buyTheses[rng.Intn(len(buyTheses))]
		dirKo := "매수"
		switch dir {
		case model.DirectionHold:
			thesis, dirKo = holdTheses[rng.Intn(len(holdTheses))], "중립"
		case model.DirectionSell:
			thesis, dirKo = sellTheses[rng.Intn(len(sellTheses))], "매도"
		}
		score, grade := scoreText(r.score)
		scoreComment := "업종 안에서 중간 정도다"
		if r.score != nil && *r.score >= 70 {
			scoreComment = "업종 안에서 상위권이다"
		} else if r.score != nil && *r.score < 45 {
			scoreComment = "업종 평균을 밑돈다 — 그만큼 기대가 덜 반영돼 있다고 본다"
		}
		perm := rng.Perm(len(text.points))
		rperm := rng.Perm(len(text.risks))
		var b strings.Builder
		fmt.Fprintf(&b, "## 한 줄 요약\n%s에 대해 %d일 관점 **%s** 의견입니다. 기준가 %s", s.name, horizon, dirKo, fmtPrice(s.currency, base))
		if targetPx != nil {
			fmt.Fprintf(&b, ", 목표가 %s(%+.0f%%)", fmtPrice(s.currency, *targetPx), upside*100)
		}
		b.WriteString(".\n\n## 투자 포인트\n")
		fmt.Fprintf(&b, "1. %s.\n2. %s.\n3. 발행 시점 종합 점수는 %s점(%s)으로 %s.\n\n", text.points[perm[0]], text.points[perm[1]], score, grade, scoreComment)
		fmt.Fprintf(&b, "## 리스크\n- %s\n- %s\n\n", text.risks[rperm[0]], text.risks[rperm[1]])
		b.WriteString("## 대응 전략\n")
		switch dir {
		case model.DirectionBuy:
			fmt.Fprintf(&b, "한 번에 사기보다 두세 번에 나눠 담고, 손절가 %s를 이탈하면 판단을 다시 하겠습니다.\n", fmtPrice(s.currency, *stopPx))
		case model.DirectionSell:
			fmt.Fprintf(&b, "보유 중이라면 반등 시 비중을 줄이는 쪽을 권합니다. %s를 넘어서면 이 의견은 틀린 것으로 보겠습니다.\n", fmtPrice(s.currency, *stopPx))
		default:
			b.WriteString("다음 실적에서 이익률 방향을 확인한 뒤 의견을 다시 정하겠습니다.\n")
		}
		b.WriteString("\n> 개인적인 분석이며 투자 권유가 아닙니다.")

		var prev *float64 = r.prev
		gaps := []string{"quote", "briefing", "news", "disclosures"}
		if r.ai == "" {
			gaps = append(gaps, "aiSummary")
		}
		chart := model.ResearchChart{Interval: string(model.Interval1d), Bars: []model.Candle{}}
		nextDay := kstDate(at.AddDate(0, 0, 1)) + "T00:00:00+09:00"
		if bars, _, err := candles.ListBefore(ctx, s.symbol, model.Interval1d, nextDay, 120); err == nil {
			sort.Slice(bars, func(i, j int) bool { return bars[i].Time < bars[j].Time })
			chart.Bars = bars
		}
		if len(chart.Bars) == 0 {
			gaps = append(gaps, "chart")
		}
		var gradePtr string
		if r.score != nil {
			gradePtr = model.ScoreGrade(*r.score)
		}
		call := model.ResearchCall{
			UserID: author.id, Topic: "stock", Symbol: s.symbol, SymbolName: s.name,
			AssetType: s.assetType, Currency: s.currency,
			Title:       fmt.Sprintf("%s, %s", s.name, thesis),
			PublishedAt: at, TradeDate: r.date, BenchmarkID: benchmarkFor(s),
			Direction: dir, BasePrice: base, TargetPrice: targetPx, StopPrice: stopPx,
			HorizonDays: horizon, Conviction: 40 + rng.Intn(46),
			ScoreAtPub: r.score, GradeAtPub: gradePtr,
			Evidence: model.ResearchEvidence{PrevClose: prev, ChangeRate: r.chg, Volume: r.volume},
			Context:  model.ResearchContext{AISummary: r.ai, DataGaps: gaps},
			Chart:    chart, BodyMD: b.String(), Tags: tagsBySector[s.sector],
			Visibility: model.VisibilityPublic, Status: model.StatusOpen,
		}
		created, err := repo.Create(ctx, call)
		if err != nil {
			return made, fmt.Errorf("research %s: %w", s.symbol, err)
		}
		if created.UserID != author.id {
			return made, fmt.Errorf("research %d: stored user_id %d, want %d", created.ID, created.UserID, author.id)
		}

		views := 30 + rng.Intn(400)
		if _, err := db.ExecContext(ctx, `UPDATE research_calls SET created_at = ?, updated_at = ?, view_count = ? WHERE id = ?`, at, at, views, created.ID); err != nil {
			return made, err
		}
		if _, err := db.ExecContext(ctx, `UPDATE research_revisions SET created_at = ? WHERE call_id = ?`, at, created.ID); err != nil {
			return made, err
		}
		if err := seedResearchSocial(ctx, db, rng, users, author, created.ID, at, now); err != nil {
			return made, err
		}
		made++
	}
	return made, nil
}

func seedResearchSocial(ctx context.Context, db *sql.DB, rng *rand.Rand, users []*seedUser, author *seedUser, callID int64, at, now time.Time) error {
	clamp := func(t time.Time) time.Time {
		if t.After(now) {
			return now
		}
		return t
	}
	for _, kind := range []struct {
		name string
		n    int
	}{{"like", 3 + rng.Intn(28)}, {"bookmark", rng.Intn(7)}} {
		for _, u := range likeUsers(rng, users, at, author.id, kind.n) {
			if _, err := db.ExecContext(ctx, `
				INSERT IGNORE INTO research_reactions (call_id, user_id, kind, created_at) VALUES (?, ?, ?, ?)`,
				callID, u.id, kind.name, clamp(at.Add(time.Duration(10+rng.Intn(72*60))*time.Minute))); err != nil {
				return fmt.Errorf("research reaction: %w", err)
			}
		}
	}
	for _, u := range likeUsers(rng, users, at, author.id, rng.Intn(9)) {
		if _, err := db.ExecContext(ctx, `
			INSERT IGNORE INTO research_ratings (call_id, user_id, clarity, evidence, risk, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, callID, u.id, 3+rng.Intn(3), 2+rng.Intn(4), 3+rng.Intn(3),
			clamp(at.Add(time.Duration(30+rng.Intn(96*60))*time.Minute)), clamp(at.Add(time.Duration(30+rng.Intn(96*60))*time.Minute))); err != nil {
			return fmt.Errorf("research rating: %w", err)
		}
	}
	var ids []int64
	for c := 0; c < rng.Intn(7); c++ {
		cat := at.Add(time.Duration(20+rng.Intn(72*60)) * time.Minute)
		if cat.After(now) {
			continue
		}
		var parent *int64
		who := pickUser(rng, users, cat, author.id)
		text := reportComments[rng.Intn(len(reportComments))]
		if len(ids) > 0 && rng.Float64() < 0.35 {
			p := ids[rng.Intn(len(ids))]
			parent, who, text = &p, author, []string{"의견 감사합니다!", "말씀하신 부분 다음 업데이트에 반영해 볼게요", "네 그 리스크도 지켜보고 있습니다"}[rng.Intn(3)]
		}
		if who == nil {
			continue
		}
		res, err := db.ExecContext(ctx, `
			INSERT INTO research_comments (call_id, parent_id, user_id, body, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`, callID, parent, who.id, text, cat, cat)
		if err != nil {
			return fmt.Errorf("research comment: %w", err)
		}
		id, _ := res.LastInsertId()
		ids = append(ids, id)
		for _, u := range likeUsers(rng, users, cat, who.id, rng.Intn(3)) {
			if _, err := db.ExecContext(ctx, `
				INSERT IGNORE INTO research_comment_likes (comment_id, user_id, created_at) VALUES (?, ?, ?)`,
				id, u.id, clamp(cat.Add(time.Hour))); err != nil {
				return fmt.Errorf("research comment like: %w", err)
			}
		}
	}
	return nil
}
