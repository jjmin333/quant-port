package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
)

/*
자산 확장 공공데이터 표(096) — 실거래·지수·금리·공매. 적재는 worker-filings, 조회는 API.
*/

type PublicDataRepo struct {
	db *sql.DB
}

func NewPublicDataRepo(db *sql.DB) *PublicDataRepo { return &PublicDataRepo{db: db} }

// ── 지역 ────────────────────────────────────────────────────────────────────

type ReRegion struct {
	LawdCd    string `json:"lawdCd"`
	Sido      string `json:"sido"`
	Name      string `json:"name"`
	Tier      string `json:"tier"`
	RoneClsID *int   `json:"-"`
}

// UpsertRegions writes the region list. tier는 더 넓은 쪽(full)을 지킨다.
func (r *PublicDataRepo) UpsertRegions(ctx context.Context, regions []marketdata.Region, tier string) error {
	for _, g := range regions {
		if _, err := r.db.ExecContext(ctx, `INSERT INTO re_regions (lawd_cd, sido, name, tier) VALUES (?,?,?,?)
			ON DUPLICATE KEY UPDATE sido = VALUES(sido), name = VALUES(name),
			  tier = IF(tier = 'full' OR VALUES(tier) = 'full', 'full', 'all')`, g.LawdCd, g.Sido, g.Name, tier); err != nil {
			return err
		}
	}
	return nil
}

// PromoteRegion makes a region 'full' (사용자가 그 지역 부동산을 등록했을 때).
func (r *PublicDataRepo) PromoteRegion(ctx context.Context, lawdCd string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE re_regions SET tier = 'full' WHERE lawd_cd = ?`, lawdCd)
	return err
}

func (r *PublicDataRepo) SetRoneCls(ctx context.Context, lawdCd string, clsID int) error {
	_, err := r.db.ExecContext(ctx, `UPDATE re_regions SET rone_cls_id = ? WHERE lawd_cd = ?`, clsID, lawdCd)
	return err
}

func (r *PublicDataRepo) Regions(ctx context.Context) ([]ReRegion, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT lawd_cd, sido, name, tier, rone_cls_id FROM re_regions ORDER BY lawd_cd`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReRegion{}
	for rows.Next() {
		var g ReRegion
		var cls sql.NullInt64
		if err := rows.Scan(&g.LawdCd, &g.Sido, &g.Name, &g.Tier, &cls); err != nil {
			return nil, err
		}
		if cls.Valid {
			n := int(cls.Int64)
			g.RoneClsID = &n
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ── 실거래 ──────────────────────────────────────────────────────────────────

// ReplaceTradeSlice swaps one (kind, dealType, region, month) slice and stamps re_sync_state.
func (r *PublicDataRepo) ReplaceTradeSlice(ctx context.Context, kind, dealType, lawdCd, ym string, rows []marketdata.RTMSRow) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM re_trades WHERE lawd_cd = ? AND kind = ? AND deal_type = ? AND deal_ym = ?`,
		lawdCd, kind, dealType, ym); err != nil {
		return err
	}
	const cols = 20
	for start := 0; start < len(rows); start += 200 {
		end := min(start+200, len(rows))
		var sb strings.Builder
		sb.WriteString(`INSERT INTO re_trades (kind, deal_type, lawd_cd, deal_ym, deal_date, umd_nm, jibun, complex_key,
			complex_name, category, area, land_area, floor, build_year, price, deposit, monthly_rent, contract_type, direct, cancelled, road) VALUES `)
		args := make([]any, 0, (end-start)*(cols+1))
		for i, t := range rows[start:end] {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
			args = append(args, kind, dealType, lawdCd, ym, t.DealDate.Format("2006-01-02"), trimRunes(t.UmdNm, 40),
				trimRunes(t.Jibun, 40), trimRunes(t.ComplexKey, 120), trimRunes(t.ComplexName, 100), trimRunes(t.Category, 40),
				t.Area, t.LandArea, t.Floor, t.BuildYear, t.Price, t.Deposit, t.Monthly, trimRunes(t.ContractType, 8), t.Direct, t.Cancelled,
				trimRunes(t.Road, 80))
		}
		if _, err := tx.ExecContext(ctx, sb.String(), args...); err != nil {
			return fmt.Errorf("re_trades insert %s %s %s: %w", lawdCd, kind, ym, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO re_sync_state (kind, deal_type, lawd_cd, deal_ym, rows_n, synced_at)
		VALUES (?,?,?,?,?,CURRENT_TIMESTAMP(3)) ON DUPLICATE KEY UPDATE rows_n = VALUES(rows_n), synced_at = VALUES(synced_at)`,
		kind, dealType, lawdCd, ym, len(rows)); err != nil {
		return err
	}
	return tx.Commit()
}

// SyncedAt — (종류·지역·월)마다 마지막 적재 시각.
func (r *PublicDataRepo) SyncedAt(ctx context.Context) (map[string]time.Time, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT kind, deal_type, lawd_cd, deal_ym, synced_at FROM re_sync_state`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var k, d, l, ym string
		var at time.Time
		if err := rows.Scan(&k, &d, &l, &ym, &at); err != nil {
			return nil, err
		}
		out[SyncKey(k, d, l, ym)] = at
	}
	return out, rows.Err()
}

func SyncKey(kind, dealType, lawdCd, ym string) string {
	return kind + "|" + dealType + "|" + lawdCd + "|" + ym
}

// PruneTrades drops trades older than the keep window.
// 아파트 매매는 더 오래 둔다(aptSaleBeforeYM, 3년 창).
func (r *PublicDataRepo) PruneTrades(ctx context.Context, beforeYM, aptSaleBeforeYM string) (int64, error) {
	const keep = ` AND NOT (kind = 'apt' AND deal_type = 'sale' AND deal_ym >= ?)`
	res, err := r.db.ExecContext(ctx, `DELETE FROM re_trades WHERE deal_ym < ?`+keep, beforeYM, aptSaleBeforeYM)
	if err != nil {
		return 0, err
	}
	_, _ = r.db.ExecContext(ctx, `DELETE FROM re_sync_state WHERE deal_ym < ?`+keep, beforeYM, aptSaleBeforeYM)
	return res.RowsAffected()
}

// ReTrade — 화면·평가가 읽는 거래 한 건. 금액은 원.
type ReTrade struct {
	Kind        string   `json:"kind"`
	DealType    string   `json:"dealType"`
	DealDate    string   `json:"dealDate"`
	UmdNm       string   `json:"umdNm"`
	Jibun       string   `json:"jibun"`
	ComplexKey  string   `json:"complexKey"`
	ComplexName string   `json:"complexName"`
	Category    string   `json:"category,omitempty"`
	Area        float64  `json:"area"`
	LandArea    *float64 `json:"landArea,omitempty"`
	Floor       *int     `json:"floor,omitempty"`
	BuildYear   *int     `json:"buildYear,omitempty"`
	Price       *int64   `json:"price,omitempty"`
	Deposit     *int64   `json:"deposit,omitempty"`
	Monthly     *int64   `json:"monthlyRent,omitempty"`
	Contract    string   `json:"contractType,omitempty"`
	Direct      bool     `json:"direct"`
	Cancelled   bool     `json:"cancelled"`
}

const tradeCols = `kind, deal_type, DATE_FORMAT(deal_date, '%Y-%m-%d'), umd_nm, jibun, complex_key, complex_name, category,
	area, land_area, floor, build_year, price, deposit, monthly_rent, contract_type, direct, cancelled`

func (r *PublicDataRepo) queryTrades(ctx context.Context, q string, args ...any) ([]ReTrade, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReTrade{}
	for rows.Next() {
		var t ReTrade
		var land sql.NullFloat64
		var floor, by sql.NullInt64
		var price, dep, mon sql.NullInt64
		if err := rows.Scan(&t.Kind, &t.DealType, &t.DealDate, &t.UmdNm, &t.Jibun, &t.ComplexKey, &t.ComplexName, &t.Category,
			&t.Area, &land, &floor, &by, &price, &dep, &mon, &t.Contract, &t.Direct, &t.Cancelled); err != nil {
			return nil, err
		}
		t.LandArea = nullF(land)
		t.Floor, t.BuildYear = nullIntPtr(floor), nullIntPtr(by)
		t.Price, t.Deposit, t.Monthly = manwon(price), manwon(dep), manwon(mon)
		out = append(out, t)
	}
	return out, rows.Err()
}

func nullIntPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

// manwon converts the stored 만원 to 원.
func manwon(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64 * 10000
	return &n
}

// ComplexTrades — 한 단지(건물)의 거래, 최신순. since 이후만.
func (r *PublicDataRepo) ComplexTrades(ctx context.Context, complexKey, dealType string, since time.Time) ([]ReTrade, error) {
	return r.queryTrades(ctx, `SELECT `+tradeCols+` FROM re_trades
		WHERE complex_key = ? AND deal_type = ? AND deal_date >= ? ORDER BY deal_date DESC LIMIT 2000`,
		complexKey, dealType, since.Format("2006-01-02"))
}

// AreaTrades — 같은 지역·동·종류의 거래(㎡ 단가 추정용). umd가 비면 시군구 전체.
func (r *PublicDataRepo) AreaTrades(ctx context.Context, lawdCd, kind, umd string, since time.Time) ([]ReTrade, error) {
	q := `SELECT ` + tradeCols + ` FROM re_trades WHERE lawd_cd = ? AND kind = ? AND deal_type = 'sale' AND deal_date >= ?`
	args := []any{lawdCd, kind, since.Format("2006-01-02")}
	if umd != "" {
		q += ` AND umd_nm = ?`
		args = append(args, umd)
	}
	return r.queryTrades(ctx, q+` ORDER BY deal_date DESC LIMIT 3000`, args...)
}

// ReComplex — 검색 결과 한 줄.
type ReComplex struct {
	ComplexKey  string `json:"complexKey"`
	ComplexName string `json:"complexName"`
	Kind        string `json:"kind"`
	LawdCd      string `json:"lawdCd"`
	UmdNm       string `json:"umdNm"`
	Jibun       string `json:"jibun"`
	BuildYear   *int   `json:"buildYear,omitempty"`
	Trades      int    `json:"trades"`
	LastDeal    string `json:"lastDeal"`
	Areas       string `json:"areas"`
}

// SearchComplexes finds complexes by name within a region (핸들러가 lawdCd를 강제한다). 거래 많은 순.
func (r *PublicDataRepo) SearchComplexes(ctx context.Context, lawdCd, kind, q string, limit int) ([]ReComplex, error) {
	where := []string{"complex_key <> ''", "kind = ?"}
	args := []any{kind}
	if lawdCd != "" {
		where = append(where, "lawd_cd = ?")
		args = append(args, lawdCd)
	}
	if q = strings.TrimSpace(q); q != "" && kind == "apt" {
		// 아파트는 단지 목록에서 먼저 찾는다 — 띄어쓰기·도로명·지번까지 맞고, 시군구 없이 전국도 된다(2026-10-09).
		hits, err := r.SearchComplexDirectory(ctx, lawdCd, q, limit)
		if err != nil {
			return nil, err
		}
		if len(hits) == 0 {
			return []ReComplex{}, nil
		}
		marks := strings.TrimSuffix(strings.Repeat("?,", len(hits)), ",")
		where = append(where, "complex_key IN ("+marks+")")
		for _, h := range hits {
			args = append(args, h.Key)
		}
	} else if q != "" {
		where = append(where, "(complex_name LIKE ? OR umd_nm LIKE ?)")
		like := "%" + strings.NewReplacer("%", "", "_", "").Replace(q) + "%"
		args = append(args, like, like)
	}
	rows, err := r.db.QueryContext(ctx, `SELECT complex_key, MAX(complex_name), MAX(lawd_cd), MAX(umd_nm), MAX(jibun), MAX(build_year),
		COUNT(*), DATE_FORMAT(MAX(deal_date), '%Y-%m-%d'),
		SUBSTRING_INDEX(GROUP_CONCAT(DISTINCT ROUND(area) ORDER BY ROUND(area) SEPARATOR ','), ',', 12)
		FROM re_trades WHERE `+strings.Join(where, " AND ")+`
		GROUP BY complex_key ORDER BY COUNT(*) DESC LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReComplex{}
	for rows.Next() {
		c := ReComplex{Kind: kind}
		var by sql.NullInt64
		if err := rows.Scan(&c.ComplexKey, &c.ComplexName, &c.LawdCd, &c.UmdNm, &c.Jibun, &by, &c.Trades, &c.LastDeal, &c.Areas); err != nil {
			return nil, err
		}
		c.BuildYear = nullIntPtr(by)
		out = append(out, c)
	}
	return out, rows.Err()
}

// RegionTradeStats — 지역의 월별 아파트 매매 건수·㎡당 중앙가 근사(평균) — 시장 카드용.
type RegionMonth struct {
	YM        string  `json:"ym"`
	Count     int     `json:"count"`
	AvgPerM2  float64 `json:"avgPerM2"`
	RentCount int     `json:"rentCount"`
}

func (r *PublicDataRepo) RegionMonthly(ctx context.Context, lawdCd, kind string) ([]RegionMonth, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT deal_ym,
		SUM(deal_type = 'sale' AND cancelled = 0),
		COALESCE(AVG(CASE WHEN deal_type = 'sale' AND cancelled = 0 AND direct = 0 AND area > 0 THEN price * 10000 / area END), 0),
		SUM(deal_type = 'rent')
		FROM re_trades WHERE lawd_cd = ? AND kind = ? GROUP BY deal_ym ORDER BY deal_ym`, lawdCd, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RegionMonth{}
	for rows.Next() {
		var m RegionMonth
		if err := rows.Scan(&m.YM, &m.Count, &m.AvgPerM2, &m.RentCount); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ── R-ONE 지수 ───────────────────────────────────────────────────────────────

type ReIndexRow struct {
	StatblID   string
	ClsID      int
	ItmID      int
	Period     string
	PeriodDate time.Time
	ClsName    string
	ClsFull    string
	ItmName    string
	Value      float64
}

func (r *PublicDataRepo) UpsertIndex(ctx context.Context, rows []ReIndexRow) error {
	for start := 0; start < len(rows); start += 300 {
		end := min(start+300, len(rows))
		var sb strings.Builder
		sb.WriteString(`INSERT INTO re_index (statbl_id, cls_id, itm_id, period, period_date, cls_name, cls_full, itm_name, value) VALUES `)
		args := make([]any, 0, (end-start)*9)
		for i, x := range rows[start:end] {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?,?,?,?,?,?)")
			args = append(args, x.StatblID, x.ClsID, x.ItmID, x.Period, x.PeriodDate.Format("2006-01-02"),
				trimRunes(x.ClsName, 40), trimRunes(x.ClsFull, 100), trimRunes(x.ItmName, 40), x.Value)
		}
		sb.WriteString(` ON DUPLICATE KEY UPDATE value = VALUES(value), period_date = VALUES(period_date),
			cls_name = VALUES(cls_name), cls_full = VALUES(cls_full), itm_name = VALUES(itm_name)`)
		if _, err := r.db.ExecContext(ctx, sb.String(), args...); err != nil {
			return err
		}
	}
	return nil
}

// LatestIndexPeriod — 표의 가장 최근 시점(없으면 "").
func (r *PublicDataRepo) LatestIndexPeriod(ctx context.Context, statblID string) (string, error) {
	var p sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT MAX(period) FROM re_index WHERE statbl_id = ?`, statblID).Scan(&p)
	return p.String, err
}

// IndexClasses — 표에 있는 지역 분류(cls_id, 이름, 전체 이름).
func (r *PublicDataRepo) IndexClasses(ctx context.Context, statblID string) ([]ReIndexRow, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT cls_id, MAX(cls_name), MAX(cls_full) FROM re_index WHERE statbl_id = ? GROUP BY cls_id`, statblID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReIndexRow
	for rows.Next() {
		var x ReIndexRow
		if err := rows.Scan(&x.ClsID, &x.ClsName, &x.ClsFull); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// IndexPoint — 시계열 한 점.
type IndexPoint struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

// IndexSeries — (표, 지역, 항목)의 시계열(since 이후, 오래된 순). clsName이 비지 않으면 cls_id 대신 이름으로 찾는다.
func (r *PublicDataRepo) IndexSeries(ctx context.Context, statblID string, clsID, itmID int, since time.Time) ([]IndexPoint, error) {
	q := `SELECT DATE_FORMAT(period_date, '%Y-%m-%d'), value FROM re_index WHERE statbl_id = ? AND cls_id = ? AND period_date >= ?`
	args := []any{statblID, clsID, since.Format("2006-01-02")}
	if itmID > 0 {
		q += ` AND itm_id = ?`
		args = append(args, itmID)
	}
	rows, err := r.db.QueryContext(ctx, q+` ORDER BY period_date`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IndexPoint{}
	for rows.Next() {
		var p IndexPoint
		if err := rows.Scan(&p.Date, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// IndexClsByName finds a class id in a table by name and an optional parent hint in the full name.
func (r *PublicDataRepo) IndexClsByName(ctx context.Context, statblID, name, hint string) (int, bool, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT cls_id, cls_full FROM re_index WHERE statbl_id = ? AND cls_name = ?`, statblID, name)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	best, found := 0, false
	for rows.Next() {
		var id int
		var full string
		if err := rows.Scan(&id, &full); err != nil {
			return 0, false, err
		}
		if !found || (hint != "" && strings.Contains(full, hint)) {
			best, found = id, true
		}
	}
	return best, found, rows.Err()
}

// IndexItems — 표의 항목(itm_id, 이름).
func (r *PublicDataRepo) IndexItems(ctx context.Context, statblID string, clsID int) (map[int]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT itm_id, itm_name FROM re_index WHERE statbl_id = ? AND cls_id = ?`, statblID, clsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]string{}
	for rows.Next() {
		var id int
		var n string
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// ── 금리 ────────────────────────────────────────────────────────────────────

func (r *PublicDataRepo) UpsertRates(ctx context.Context, seriesID string, obs []IndexPoint, periods []string) error {
	for i, o := range obs {
		if _, err := r.db.ExecContext(ctx, `INSERT INTO macro_rates (series_id, period, period_date, value) VALUES (?,?,?,?)
			ON DUPLICATE KEY UPDATE value = VALUES(value), period_date = VALUES(period_date), fetched_at = CURRENT_TIMESTAMP(3)`,
			seriesID, periods[i], o.Date, o.Value); err != nil {
			return err
		}
	}
	return nil
}

func (r *PublicDataRepo) LatestRatePeriod(ctx context.Context, seriesID string) (string, error) {
	var p sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT MAX(period) FROM macro_rates WHERE series_id = ?`, seriesID).Scan(&p)
	return p.String, err
}

// RateSeries — since 이후 시계열(오래된 순).
func (r *PublicDataRepo) RateSeries(ctx context.Context, seriesID string, since time.Time) ([]IndexPoint, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DATE_FORMAT(period_date, '%Y-%m-%d'), value FROM macro_rates
		WHERE series_id = ? AND period_date >= ? ORDER BY period_date`, seriesID, since.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IndexPoint{}
	for rows.Next() {
		var p IndexPoint
		if err := rows.Scan(&p.Date, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// LatestRates — 계열마다 마지막 두 값(지금·직전).
func (r *PublicDataRepo) LatestRates(ctx context.Context) (map[string][2]IndexPoint, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT series_id, DATE_FORMAT(period_date, '%Y-%m-%d'), value FROM (
		SELECT series_id, period_date, value, ROW_NUMBER() OVER (PARTITION BY series_id ORDER BY period_date DESC) rn FROM macro_rates
		) t WHERE rn <= 2 ORDER BY series_id, period_date DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][2]IndexPoint{}
	for rows.Next() {
		var id string
		var p IndexPoint
		if err := rows.Scan(&id, &p.Date, &p.Value); err != nil {
			return nil, err
		}
		pair := out[id]
		if pair[0].Date == "" {
			pair[0] = p
		} else {
			pair[1] = p
		}
		out[id] = pair
	}
	return out, rows.Err()
}

// ── 온비드 ──────────────────────────────────────────────────────────────────

type OnbidNotice struct {
	PbancMngNo, Name, OrgName, PrptDiv string
	OpbdAt                             *time.Time
}

func (r *PublicDataRepo) UpsertNotices(ctx context.Context, list []OnbidNotice) error {
	for _, n := range list {
		if _, err := r.db.ExecContext(ctx, `INSERT INTO onbid_notices (pbanc_mng_no, name, org_name, prpt_div, opbd_at)
			VALUES (?,?,?,?,?) ON DUPLICATE KEY UPDATE name = VALUES(name), org_name = VALUES(org_name),
			prpt_div = VALUES(prpt_div), opbd_at = VALUES(opbd_at)`,
			n.PbancMngNo, trimRunes(n.Name, 300), trimRunes(n.OrgName, 100), trimRunes(n.PrptDiv, 20), n.OpbdAt); err != nil {
			return err
		}
	}
	return nil
}

// NoticesTodo — 물건을 아직 안 받은(또는 다시 받을) 공고(개찰 가까운 순).
func (r *PublicDataRepo) NoticesTodo(ctx context.Context, limit int) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT pbanc_mng_no FROM onbid_notices WHERE detail_done = 0
		AND (opbd_at IS NULL OR opbd_at >= NOW() - INTERVAL 14 DAY) ORDER BY opbd_at LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *PublicDataRepo) MarkNoticeDone(ctx context.Context, pbancMngNo string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE onbid_notices SET detail_done = 1 WHERE pbanc_mng_no = ?`, pbancMngNo)
	return err
}

// OnbidItem — 공매 물건 한 건.
type OnbidItem struct {
	CltrMngNo    string     `json:"cltrMngNo"`
	PbctCdtnNo   int64      `json:"pbctCdtnNo"`
	PbancMngNo   string     `json:"pbancMngNo"`
	OnbidCltrNo  *int64     `json:"onbidCltrNo,omitempty"`
	Name         string     `json:"name"`
	Address      string     `json:"address"`
	Sido         string     `json:"sido"`
	UsageL       string     `json:"usageL"`
	UsageM       string     `json:"usageM"`
	UsageS       string     `json:"usageS"`
	PrptDiv      string     `json:"prptDiv"`
	DspsMethod   string     `json:"dspsMethod"`
	OrgName      string     `json:"orgName"`
	Appraisal    *int64     `json:"appraisal,omitempty"`
	MinBid       *int64     `json:"minBid,omitempty"`
	MinBidRatio  *float64   `json:"minBidRatio,omitempty"`
	FailCount    int        `json:"failCount"`
	BidStart     *time.Time `json:"bidStart,omitempty"`
	BidEnd       *time.Time `json:"bidEnd,omitempty"`
	OpbdAt       *time.Time `json:"opbdAt,omitempty"`
	Result       string     `json:"result"`
	WinningBid   *int64     `json:"winningBid,omitempty"`
	ValidBidders *int       `json:"validBidders,omitempty"`
}

func (r *PublicDataRepo) UpsertOnbidItems(ctx context.Context, items []OnbidItem) error {
	for _, it := range items {
		if _, err := r.db.ExecContext(ctx, `INSERT INTO onbid_items (cltr_mng_no, pbct_cdtn_no, pbanc_mng_no, onbid_cltr_no, name,
			address, sido, usage_l, usage_m, usage_s, prpt_div, dsps_method, org_name, appraisal, min_bid, min_bid_ratio,
			fail_count, bid_start, bid_end, opbd_at, result) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON DUPLICATE KEY UPDATE pbanc_mng_no = VALUES(pbanc_mng_no), name = VALUES(name), address = VALUES(address),
			  sido = VALUES(sido), usage_l = VALUES(usage_l), usage_m = VALUES(usage_m), usage_s = VALUES(usage_s),
			  prpt_div = VALUES(prpt_div), dsps_method = VALUES(dsps_method), org_name = VALUES(org_name),
			  appraisal = VALUES(appraisal), min_bid = VALUES(min_bid), min_bid_ratio = VALUES(min_bid_ratio),
			  fail_count = VALUES(fail_count), bid_start = VALUES(bid_start), bid_end = VALUES(bid_end), opbd_at = VALUES(opbd_at),
			  result = IF(VALUES(result) = '', result, VALUES(result))`,
			it.CltrMngNo, it.PbctCdtnNo, it.PbancMngNo, it.OnbidCltrNo, trimRunes(it.Name, 300), trimRunes(it.Address, 300),
			trimRunes(it.Sido, 20), trimRunes(it.UsageL, 40), trimRunes(it.UsageM, 40), trimRunes(it.UsageS, 40),
			trimRunes(it.PrptDiv, 20), trimRunes(it.DspsMethod, 8), trimRunes(it.OrgName, 100), it.Appraisal, it.MinBid,
			it.MinBidRatio, it.FailCount, it.BidStart, it.BidEnd, it.OpbdAt, it.Result); err != nil {
			return err
		}
	}
	return nil
}

// ReopenNotices — 입찰이 끝났는데 결과가 비어 있는 회차가 있는 공고는 다시 받는다(상태가 바뀌었을 것).
func (r *PublicDataRepo) ReopenNotices(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE onbid_notices n SET detail_done = 0 WHERE detail_done = 1 AND EXISTS (
		SELECT 1 FROM onbid_items i WHERE i.pbanc_mng_no = n.pbanc_mng_no AND i.result = ''
		  AND i.bid_end < NOW() - INTERVAL 1 HOUR AND i.bid_end > NOW() - INTERVAL 14 DAY)`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ResultTodo — 낙찰됐는데 낙찰가를 아직 안 받은 회차(결과상세 1호출씩).
func (r *PublicDataRepo) ResultTodo(ctx context.Context, limit int) ([]OnbidItem, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT cltr_mng_no, pbct_cdtn_no FROM onbid_items
		WHERE result = '낙찰' AND winning_bid IS NULL AND result_checked = 0
		ORDER BY bid_end DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OnbidItem
	for rows.Next() {
		var it OnbidItem
		if err := rows.Scan(&it.CltrMngNo, &it.PbctCdtnNo); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *PublicDataRepo) SetOnbidResult(ctx context.Context, cltrMngNo string, pbctCdtnNo int64, result string, winning *int64, bidders *int) error {
	_, err := r.db.ExecContext(ctx, `UPDATE onbid_items SET result = ?, winning_bid = ?, valid_bidders = ?, result_checked = 1
		WHERE cltr_mng_no = ? AND pbct_cdtn_no = ?`, result, winning, bidders, cltrMngNo, pbctCdtnNo)
	return err
}

// OnbidQuery — 공매 목록 필터.
type OnbidQuery struct {
	Sido     string
	Usage    string // usage_m 부분 일치
	MinFails int
	MaxRatio float64 // 최저입찰가/감정가 상한(%) — 0이면 없음
	Status   string  // open(입찰 전·중) · closed(결과 있음)
	Method   string  // 매각 · 임대
	Sort     string  // opbd · ratio · fails · appraisal · score
	Limit    int
	Offset   int
}

func (r *PublicDataRepo) ListOnbid(ctx context.Context, q OnbidQuery) ([]OnbidItem, int, error) {
	// 공고 유형이 부동산이어도 유가증권·회원권·자동차가 섞여 온다(10-01 실측) — 부동산만.
	where := []string{"i.usage_l = '부동산'"}
	args := []any{}
	if q.Status == "closed" {
		where = append(where, "i.result <> ''")
	} else {
		where = append(where, "i.bid_end >= NOW()", "i.result = ''")
	}
	if q.Sido != "" {
		where = append(where, "i.sido = ?")
		args = append(args, q.Sido)
	}
	if q.Usage != "" {
		where = append(where, "(i.usage_m LIKE ? OR i.usage_s LIKE ?)")
		u := "%" + q.Usage + "%"
		args = append(args, u, u)
	}
	if q.MinFails > 0 {
		where = append(where, "i.fail_count >= ?")
		args = append(args, q.MinFails)
	}
	if q.MaxRatio > 0 {
		where = append(where, "i.min_bid_ratio <= ?")
		args = append(args, q.MaxRatio)
	}
	if q.Method != "" {
		where = append(where, "i.dsps_method = ?")
		args = append(args, q.Method)
	}
	order := "i.bid_end"
	switch q.Sort {
	case "ratio":
		order = "i.min_bid_ratio IS NULL, i.min_bid_ratio"
	case "fails":
		order = "i.fail_count DESC, i.bid_end"
	case "appraisal":
		order = "i.appraisal DESC"
	case "score": // asset_scores(kind=onbid) 키는 "관리번호|조건번호" — 점수 없는 물건은 뒤로.
		order = "(SELECT s.score FROM asset_scores s WHERE s.kind = 'onbid' AND s.skey = CONCAT(i.cltr_mng_no, '|', i.pbct_cdtn_no)) IS NULL, " +
			"(SELECT s.score FROM asset_scores s WHERE s.kind = 'onbid' AND s.skey = CONCAT(i.cltr_mng_no, '|', i.pbct_cdtn_no)) DESC, i.bid_end"
	}
	if q.Status == "closed" {
		order = "i.bid_end DESC"
	}
	return r.listOnbidWhere(ctx, strings.Join(where, " AND "), args, order, q.Limit, q.Offset)
}

func (r *PublicDataRepo) listOnbidWhere(ctx context.Context, w string, args []any, order string, limit, offset int) ([]OnbidItem, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM onbid_items i WHERE `+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	// 기관명은 공고에 있다(물건 응답엔 없음).
	rows, err := r.db.QueryContext(ctx, `SELECT i.cltr_mng_no, i.pbct_cdtn_no, i.pbanc_mng_no, i.onbid_cltr_no, i.name, i.address, i.sido,
		i.usage_l, i.usage_m, i.usage_s, i.prpt_div, i.dsps_method, COALESCE(NULLIF(i.org_name, ''), n.org_name, ''), i.appraisal,
		i.min_bid, i.min_bid_ratio, i.fail_count, i.bid_start, i.bid_end, i.opbd_at, i.result, i.winning_bid, i.valid_bidders
		FROM onbid_items i LEFT JOIN onbid_notices n ON n.pbanc_mng_no = i.pbanc_mng_no
		WHERE `+w+` ORDER BY `+order+` LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []OnbidItem{}
	for rows.Next() {
		var it OnbidItem
		var cltrNo, appr, minBid, win sql.NullInt64
		var ratio sql.NullFloat64
		var bs, be, op sql.NullTime
		var bidders sql.NullInt64
		if err := rows.Scan(&it.CltrMngNo, &it.PbctCdtnNo, &it.PbancMngNo, &cltrNo, &it.Name, &it.Address, &it.Sido,
			&it.UsageL, &it.UsageM, &it.UsageS, &it.PrptDiv, &it.DspsMethod, &it.OrgName, &appr, &minBid, &ratio,
			&it.FailCount, &bs, &be, &op, &it.Result, &win, &bidders); err != nil {
			return nil, 0, err
		}
		it.OnbidCltrNo, it.Appraisal, it.MinBid, it.WinningBid = nullInt64Ptr(cltrNo), nullInt64Ptr(appr), nullInt64Ptr(minBid), nullInt64Ptr(win)
		it.MinBidRatio = nullF(ratio)
		it.ValidBidders = nullIntPtr(bidders)
		it.BidStart, it.BidEnd, it.OpbdAt = nullT(bs), nullT(be), nullT(op)
		out = append(out, it)
	}
	return out, total, rows.Err()
}

// OnbidStats — 최근 결과의 시도·용도별 낙찰가율(낙찰가/감정가) 중앙 근사(평균)와 건수.
type OnbidStat struct {
	Sido     string  `json:"sido"`
	UsageM   string  `json:"usageM"`
	Sold     int     `json:"sold"`
	Failed   int     `json:"failed"`
	AvgRatio float64 `json:"avgRatio"`
}

func (r *PublicDataRepo) OnbidStats(ctx context.Context) ([]OnbidStat, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT sido, usage_m, SUM(result = '낙찰'), SUM(result = '유찰'),
		COALESCE(AVG(CASE WHEN result = '낙찰' AND appraisal > 0 THEN winning_bid * 100.0 / appraisal END), 0)
		FROM onbid_items WHERE result IN ('낙찰', '유찰') AND bid_end > NOW() - INTERVAL 90 DAY AND sido <> ''
		GROUP BY sido, usage_m HAVING COUNT(*) >= 3 ORDER BY COUNT(*) DESC LIMIT 60`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OnbidStat{}
	for rows.Next() {
		var s OnbidStat
		if err := rows.Scan(&s.Sido, &s.UsageM, &s.Sold, &s.Failed, &s.AvgRatio); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func nullT(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

// RegionTrades — 한 지역·종류의 since 이후 거래 전부(점수 계산용 — 매매·전월세 둘 다).
func (r *PublicDataRepo) RegionTrades(ctx context.Context, lawdCd, kind string, since time.Time) ([]ReTrade, error) {
	return r.queryTrades(ctx, `SELECT `+tradeCols+` FROM re_trades WHERE lawd_cd = ? AND kind = ? AND deal_date >= ?`,
		lawdCd, kind, since.Format("2006-01-02"))
}

// OpenOnbidItems — 입찰 마감 전 부동산 회차 전부(점수 계산용).
func (r *PublicDataRepo) OpenOnbidItems(ctx context.Context) ([]OnbidItem, error) {
	items, _, err := r.ListOnbid(ctx, OnbidQuery{Status: "open", Limit: 100000})
	return items, err
}

// OnbidRounds — 한 물건의 모든 회차(입찰 시작 순).
func (r *PublicDataRepo) OnbidRounds(ctx context.Context, cltrMngNo string) ([]OnbidItem, error) {
	all, _, err := r.listOnbidWhere(ctx, "i.cltr_mng_no = ?", []any{cltrMngNo}, "i.bid_start", 200, 0)
	return all, err
}

// ComplexesSales — 여러 단지의 매매(관심 단지 「새 거래·신고가」용), 최신순. since 이후만.
func (r *PublicDataRepo) ComplexesSales(ctx context.Context, keys []string, since time.Time) ([]ReTrade, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	args := []any{since.Format("2006-01-02")}
	for _, k := range keys {
		args = append(args, k)
	}
	return r.queryTrades(ctx, `SELECT `+tradeCols+` FROM re_trades
		WHERE deal_type = 'sale' AND deal_date >= ? AND complex_key IN (`+strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")+`)
		ORDER BY deal_date DESC LIMIT 6000`, args...)
}

// ComplexIdentity — 단지 키의 이름·시군구(관심 담기 검증용). 거래가 없으면 ok=false.
func (r *PublicDataRepo) ComplexIdentity(ctx context.Context, key string) (name, lawd string, ok bool, err error) {
	err = r.db.QueryRowContext(ctx, `SELECT complex_name, lawd_cd FROM re_trades WHERE complex_key = ? LIMIT 1`, key).Scan(&name, &lawd)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	return name, lawd, err == nil, err
}

/*
아파트 단지 목록 re_complexes (migration 101, 2026-10-09) — 단지 찾기의 색인.

실거래 표(아파트만 250만 행)를 LIKE로 훑는 대신 단지 한 줄씩(4만여 행)에서 찾는다: 띄어쓰기를 뺀 이름,
도로명(「동대구로 345」), 법정동+지번(「범어동 35-1」). 그래서 시군구를 고르지 않아도 전국에서 찾을 수 있다.
*/

// RebuildComplexDirectory refreshes re_complexes from the apartment trades (worker-asset, 매일). 돌려주는 값은 목록의 단지 수.
func (r *PublicDataRepo) RebuildComplexDirectory(ctx context.Context) (int64, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO re_complexes (complex_key, lawd_cd, name, name_norm, umd_nm, jibun, road, build_year, trades, last_deal)
		SELECT complex_key, MAX(lawd_cd), MAX(complex_name), REPLACE(MAX(complex_name), ' ', ''), MAX(umd_nm), MAX(jibun), MAX(road),
		       MAX(build_year), COUNT(*), MAX(deal_date)
		  FROM re_trades WHERE kind = 'apt' AND complex_key <> '' AND complex_name <> ''
		 GROUP BY complex_key
		ON DUPLICATE KEY UPDATE lawd_cd = VALUES(lawd_cd), name = VALUES(name), name_norm = VALUES(name_norm),
		  umd_nm = VALUES(umd_nm), jibun = VALUES(jibun), road = IF(VALUES(road) <> '', VALUES(road), road),
		  build_year = VALUES(build_year), trades = VALUES(trades), last_deal = VALUES(last_deal)`)
	if err != nil {
		return 0, err
	}
	// RowsAffected는 「바뀐 행」이라 바뀐 게 없는 날 0이 찍혀 비어 보였다 — 목록 크기를 센다.
	var n int64
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM re_complexes`).Scan(&n)
	return n, err
}

// ComplexHit is one directory match (헤더 검색 · 단지 찾기).
type ComplexHit struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	LawdCd   string `json:"lawdCd"`
	Region   string `json:"region"` // 「대구 수성구」
	UmdNm    string `json:"umdNm"`
	Jibun    string `json:"jibun,omitempty"`
	Road     string `json:"road,omitempty"`
	Trades   int    `json:"trades"`
	LastDeal string `json:"lastDeal,omitempty"`
}

// ComplexSearchTerms splits q for the directory: 띄어쓰기를 뺀 이름 조각 하나와, 주소로 읽을 원문.
func ComplexSearchTerms(q string) (norm, addr string) {
	q = strings.NewReplacer("%", "", "_", "").Replace(strings.TrimSpace(q))
	return strings.ReplaceAll(q, " ", ""), q
}

// SearchComplexDirectory — 이름(띄어쓰기 무시) · 도로명 · 「법정동 지번」 중 하나에 맞는 단지, 거래 많은 순.
// lawdCd가 비면 전국. 시군구 이름이 앞에 붙은 검색어(「수성구 범어역우방유쉘」)도 이름 부분으로 찾는다.
func (r *PublicDataRepo) SearchComplexDirectory(ctx context.Context, lawdCd, q string, limit int) ([]ComplexHit, error) {
	norm, addr := ComplexSearchTerms(q)
	if norm == "" {
		return []ComplexHit{}, nil
	}
	where := `(c.name_norm LIKE ? OR c.road LIKE ? OR CONCAT(c.umd_nm, c.jibun) LIKE ?
	           OR ? LIKE CONCAT('%', REPLACE(g.name, ' ', ''), '%', c.name_norm, '%'))`
	args := []any{"%" + norm + "%", "%" + addr + "%", "%" + norm + "%", norm}
	if lawdCd != "" {
		where += " AND c.lawd_cd = ?"
		args = append(args, lawdCd)
	}
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.complex_key, c.name, c.lawd_cd, COALESCE(CONCAT(g.sido, ' ', g.name), ''), c.umd_nm, c.jibun, c.road,
		       c.trades, COALESCE(DATE_FORMAT(c.last_deal, '%Y-%m-%d'), '')
		  FROM re_complexes c LEFT JOIN re_regions g ON g.lawd_cd = c.lawd_cd
		 WHERE `+where+`
		 ORDER BY (c.name_norm = ?) DESC, c.trades DESC
		 LIMIT ?`, append(args[:len(args)-1:len(args)-1], norm, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ComplexHit{}
	for rows.Next() {
		var h ComplexHit
		if err := rows.Scan(&h.Key, &h.Name, &h.LawdCd, &h.Region, &h.UmdNm, &h.Jibun, &h.Road, &h.Trades, &h.LastDeal); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// ComplexDirEntry is one directory row for the asset name index.
type ComplexDirEntry struct {
	Key, Name, LawdCd, UmdNm string
	Trades                   int
}

// ComplexDirectoryEntries — 단지 목록 전부(4만여 행, 글 태그·미리보기·기사 칩 이름 색인용 — 30분 메모리).
func (r *PublicDataRepo) ComplexDirectoryEntries(ctx context.Context) ([]ComplexDirEntry, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT complex_key, name, lawd_cd, umd_nm, trades FROM re_complexes ORDER BY trades DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ComplexDirEntry
	for rows.Next() {
		var e ComplexDirEntry
		if err := rows.Scan(&e.Key, &e.Name, &e.LawdCd, &e.UmdNm, &e.Trades); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
