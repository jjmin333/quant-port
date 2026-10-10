package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ExternalSignalRepo — 093: 해외 지표 보조 · 애널리스트 컨센서스 · Reddit 언급.
type ExternalSignalRepo struct{ db *sql.DB }

func NewExternalSignalRepo(db *sql.DB) *ExternalSignalRepo { return &ExternalSignalRepo{db: db} }

// ExtMetrics is one stock_ext_metrics row.
type ExtMetrics struct {
	Symbol                            string
	Source                            string
	PER, PBR, PSR                     *float64
	ROE, OperatingMargin, NetMargin   *float64
	RevenueGrowth, EPSGrowth          *float64
	CurrentRatio, DividendYield, Beta *float64
	FetchedAt                         time.Time
}

func (r *ExternalSignalRepo) UpsertExtMetrics(ctx context.Context, m ExtMetrics) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO stock_ext_metrics
			(symbol, source, per, pbr, psr, roe, operating_margin, net_margin, revenue_growth, eps_growth,
			 current_ratio, dividend_yield, beta, fetched_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?, NOW(3))
		ON DUPLICATE KEY UPDATE source=VALUES(source), per=VALUES(per), pbr=VALUES(pbr), psr=VALUES(psr),
			roe=VALUES(roe), operating_margin=VALUES(operating_margin), net_margin=VALUES(net_margin),
			revenue_growth=VALUES(revenue_growth), eps_growth=VALUES(eps_growth),
			current_ratio=VALUES(current_ratio), dividend_yield=VALUES(dividend_yield), beta=VALUES(beta),
			fetched_at=VALUES(fetched_at)`,
		m.Symbol, m.Source, m.PER, m.PBR, m.PSR, m.ROE, m.OperatingMargin, m.NetMargin, m.RevenueGrowth,
		m.EPSGrowth, m.CurrentRatio, m.DividendYield, m.Beta)
	return err
}

// ExtMetricsFresh reads one symbol's row if it is younger than maxAge.
func (r *ExternalSignalRepo) ExtMetricsFresh(ctx context.Context, symbol string, maxAge time.Duration) (*ExtMetrics, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	var m ExtMetrics
	var per, pbr, psr, roe, opm, npm, rg, eg, cr, dy, beta sql.NullFloat64
	err := r.db.QueryRowContext(ctx, `
		SELECT symbol, source, per, pbr, psr, roe, operating_margin, net_margin, revenue_growth, eps_growth,
		       current_ratio, dividend_yield, beta, fetched_at
		  FROM stock_ext_metrics WHERE symbol = ? AND fetched_at >= ?`,
		strings.ToUpper(symbol), time.Now().Add(-maxAge)).
		Scan(&m.Symbol, &m.Source, &per, &pbr, &psr, &roe, &opm, &npm, &rg, &eg, &cr, &dy, &beta, &m.FetchedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.PER, m.PBR, m.PSR = nf(per), nf(pbr), nf(psr)
	m.ROE, m.OperatingMargin, m.NetMargin = nf(roe), nf(opm), nf(npm)
	m.RevenueGrowth, m.EPSGrowth, m.CurrentRatio = nf(rg), nf(eg), nf(cr)
	m.DividendYield, m.Beta = nf(dy), nf(beta)
	return &m, nil
}

func nf(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	x := v.Float64
	return &x
}

// AnalystConsensus is one stock_analyst_consensus row.
type AnalystConsensus struct {
	Symbol                                 string    `json:"symbol"`
	Source                                 string    `json:"source"`
	AsOf                                   string    `json:"asOf,omitempty"`
	StrongBuy, Buy, Hold, Sell, StrongSell *int      `json:"-"`
	Rating                                 *float64  `json:"rating,omitempty"`
	TargetMean                             *float64  `json:"targetMean,omitempty"`
	TargetCurrency                         string    `json:"targetCurrency,omitempty"`
	FetchedAt                              time.Time `json:"fetchedAt"`
	Counts                                 *[5]int   `json:"counts,omitempty"` // 강력매수·매수·보유·매도·강력매도
}

func (r *ExternalSignalRepo) UpsertConsensus(ctx context.Context, c AnalystConsensus) error {
	var asOf any
	if c.AsOf != "" {
		asOf = c.AsOf
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO stock_analyst_consensus
			(symbol, source, as_of, strong_buy, buy, hold, sell, strong_sell, rating, target_mean, target_currency, fetched_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?, NOW(3))
		ON DUPLICATE KEY UPDATE source=VALUES(source), as_of=VALUES(as_of), strong_buy=VALUES(strong_buy),
			buy=VALUES(buy), hold=VALUES(hold), sell=VALUES(sell), strong_sell=VALUES(strong_sell),
			rating=VALUES(rating), target_mean=VALUES(target_mean), target_currency=VALUES(target_currency),
			fetched_at=VALUES(fetched_at)`,
		c.Symbol, c.Source, asOf, c.StrongBuy, c.Buy, c.Hold, c.Sell, c.StrongSell, c.Rating, c.TargetMean, c.TargetCurrency)
	return err
}

func (r *ExternalSignalRepo) Consensus(ctx context.Context, symbol string) (*AnalystConsensus, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	var (
		c               AnalystConsensus
		asOf            sql.NullString
		sb, b, h, s, ss sql.NullInt64
		rating, target  sql.NullFloat64
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT symbol, source, DATE_FORMAT(as_of, '%Y-%m-%d'), strong_buy, buy, hold, sell, strong_sell,
		       rating, target_mean, target_currency, fetched_at
		  FROM stock_analyst_consensus WHERE symbol = ?`, strings.ToUpper(symbol)).
		Scan(&c.Symbol, &c.Source, &asOf, &sb, &b, &h, &s, &ss, &rating, &target, &c.TargetCurrency, &c.FetchedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.AsOf = asOf.String
	c.Rating, c.TargetMean = nf(rating), nf(target)
	if sb.Valid || b.Valid || h.Valid || s.Valid || ss.Valid {
		c.Counts = &[5]int{int(sb.Int64), int(b.Int64), int(h.Int64), int(s.Int64), int(ss.Int64)}
	}
	return &c, nil
}

// SocialRow is one social_mentions row.
type SocialRow struct {
	Symbol       string `json:"-"`
	SnapDate     string `json:"date"`
	Mentions     int    `json:"mentions"`
	MentionsPrev int    `json:"mentionsPrev"`
	Upvotes      int    `json:"upvotes"`
	Rank         int    `json:"rank"`
	RankPrev     *int   `json:"rankPrev,omitempty"`
}

// ReplaceSocialDay writes one day's all-stocks snapshot (다시 돌면 그날 줄을 덮는다).
func (r *ExternalSignalRepo) ReplaceSocialDay(ctx context.Context, date string, rows []SocialRow) error {
	return r.ReplaceSocialDaySource(ctx, date, "apewisdom", rows)
}

// ReplaceSocialDaySource — 출처별(all-stocks = "apewisdom", 서브레딧 = "r/<이름>").
func (r *ExternalSignalRepo) ReplaceSocialDaySource(ctx context.Context, date, source string, rows []SocialRow) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM social_mentions WHERE snap_date = ? AND source = ?`, date, source); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO social_mentions (symbol, snap_date, source, mentions, mentions_prev, upvotes, rank_now, rank_prev)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE mentions = VALUES(mentions)`,
			row.Symbol, date, source, row.Mentions, row.MentionsPrev, row.Upvotes, row.Rank, row.RankPrev); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM social_mentions WHERE snap_date < DATE_SUB(?, INTERVAL 180 DAY)`, date); err != nil {
		return err
	}
	return tx.Commit()
}

// SocialSource is one subreddit's count for a symbol on the latest day.
type SocialSource struct {
	Source       string `json:"source"`
	Mentions     int    `json:"mentions"`
	MentionsPrev int    `json:"mentionsPrev"`
	Upvotes      int    `json:"upvotes"`
}

// SocialBySubreddit — 가장 최근 스냅샷 날의 서브레딧별 언급(많은 순).
func (r *ExternalSignalRepo) SocialBySubreddit(ctx context.Context, symbol string) ([]SocialSource, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT source, mentions, mentions_prev, upvotes FROM social_mentions
		 WHERE symbol = ? AND source LIKE 'r/%'
		   AND snap_date = (SELECT MAX(snap_date) FROM social_mentions WHERE symbol = ? AND source LIKE 'r/%')
		 ORDER BY mentions DESC`, strings.ToUpper(symbol), strings.ToUpper(symbol))
	if err != nil {
		return nil, fmt.Errorf("social by subreddit: %w", err)
	}
	defer rows.Close()
	out := []SocialSource{}
	for rows.Next() {
		var s SocialSource
		if err := rows.Scan(&s.Source, &s.Mentions, &s.MentionsPrev, &s.Upvotes); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SocialHistory — 한 종목의 최근 n일(최신 먼저).
func (r *ExternalSignalRepo) SocialHistory(ctx context.Context, symbol string, days int) ([]SocialRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(snap_date, '%Y-%m-%d'), mentions, mentions_prev, upvotes, rank_now, rank_prev
		  FROM social_mentions WHERE symbol = ? AND source = 'apewisdom'
		 ORDER BY snap_date DESC LIMIT ?`, strings.ToUpper(symbol), days)
	if err != nil {
		return nil, fmt.Errorf("social history: %w", err)
	}
	defer rows.Close()
	out := []SocialRow{}
	for rows.Next() {
		var s SocialRow
		var rp sql.NullInt64
		if err := rows.Scan(&s.SnapDate, &s.Mentions, &s.MentionsPrev, &s.Upvotes, &s.Rank, &rp); err != nil {
			return nil, err
		}
		if rp.Valid {
			v := int(rp.Int64)
			s.RankPrev = &v
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

/*
SocialLatestBetween — 변화 감지용: 종목마다 [from, to] 안의 가장 최근 스냅샷.
미국 장이 KST 새벽에 닫히고 스냅샷은 그 뒤 날짜에 찍히므로 감지 쪽이 며칠 뒤까지 본다.
*/
func (r *ExternalSignalRepo) SocialLatestBetween(ctx context.Context, from, to string) ([]SocialRow, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.symbol, DATE_FORMAT(m.snap_date, '%Y-%m-%d'), m.mentions, m.mentions_prev, m.upvotes, m.rank_now, m.rank_prev
		  FROM social_mentions m
		  JOIN (SELECT symbol, MAX(snap_date) d FROM social_mentions
		         WHERE source = 'apewisdom' AND snap_date BETWEEN ? AND ? GROUP BY symbol) x
		    ON x.symbol = m.symbol AND x.d = m.snap_date
		 WHERE m.source = 'apewisdom'`, from, to)
	if err != nil {
		return nil, fmt.Errorf("social latest: %w", err)
	}
	defer rows.Close()
	var out []SocialRow
	for rows.Next() {
		var s SocialRow
		var rp sql.NullInt64
		if err := rows.Scan(&s.Symbol, &s.SnapDate, &s.Mentions, &s.MentionsPrev, &s.Upvotes, &s.Rank, &rp); err != nil {
			return nil, err
		}
		if rp.Valid {
			v := int(rp.Int64)
			s.RankPrev = &v
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// USListedSymbols — ApeWisdom 티커를 우리 카탈로그(미국 보통주·ETF)에 맞춘다.
func (r *ExternalSignalRepo) USListedSymbols(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT symbol, name FROM stocks WHERE status = 'NORMAL' AND asset_type IN ('us', 'etf')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var s, n string
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		out[strings.ToUpper(s)] = n
	}
	return out, rows.Err()
}
