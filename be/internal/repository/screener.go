package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

/*
스크리너 조회 — 표 하나를 세 번 본다 (V2 요구 4).

	Counts     깔때기 네 숫자          집계 1회
	Coverage   조건별 측정 종목 수      집계 1회
	Rows       결과 ≤200줄            범위 스캔 + 종가 조인

**거르는 조회에는 조인이 없다.** `stocks`만 eq_ref로 붙는다(시장·상품 구분). 종가와
등락률은 살아남은 200줄에만 붙인다 — 거르는 단계에 얹으면 하루치 17,919행이 전부
조인을 탄다.

# 인덱스

`idx_stock_scores_rank (trade_date, score DESC, peer_pct DESC)`가 있어서
`trade_date = ?`는 범위 스캔이다. 실측(2026-09-17): 점수 70 이상 조건이 붙으면
**1,610행**만 읽고, 조건이 없어도 그날 17,919행으로 끝난다. JSON 조건은 그 위에서
행마다 평가되는데, 전체 0.16초(프로세스 기동 포함)로 측정됐다.
*/

// ScreenerRepo reads the screener's single source table.
type ScreenerRepo struct{ db *sql.DB }

func NewScreenerRepo(db *sql.DB) *ScreenerRepo { return &ScreenerRepo{db: db} }

/*
screenEligibleClause — 랭킹과 **같은 자격**.

축 3개 미만인 점수는 「이 종목이 좋다」가 아니라 「아는 게 그것뿐이다」라는 뜻이고,
지수 추종 상품에는 애초에 `technical` 하나만 붙는다(`stock_score_universe.go` 머리글).
스크리너가 다른 우주를 보면 같은 종목이 랭킹에 없고 스크리너에만 있는 일이 생긴다.

실측(2026-09-17 국내): 4,412종목 중 **2,653종목**이 이 자격을 통과한다. 1,759종목이
빠지므로 이 단계는 깔때기에 **자기 칸으로** 나간다 — 조용히 줄이면 안 되는 크기다.
*/
func screenEligibleClause() (string, []any) {
	product, args := scoreRankableProductClause("s")
	// 추천할 수 있는 크기(SPAC·거래대금·시총 하한, 09-26)도 랭킹과 같이 건다.
	size, sizeArgs := scoreRecommendableClause("sc", "s")
	return product + scoreMinAxesClause("sc") + size, append(args, sizeArgs...)
}

/*
ScreenScope — 조건 이전에 **어디서 찾는가**: 시장(asset_type)과 업종(공통 11섹터).

업종은 2026-09-27에 더했다(문장 검색 「반도체」→ 못 알아들음 문제, 사용자 결정 C ③). 세부
업종(KSIC·SIC 코드)이 아니라 **섹터** 단위다 — 화면에 「업종 = IT」로 보이는 그대로다.
*/
type ScreenScope struct {
	AssetTypes []string
	Sector     string
}

func screenAssetTypeClause(scope ScreenScope) (string, []any) {
	clause := ""
	var args []any
	if len(scope.AssetTypes) > 0 {
		marks := strings.TrimSuffix(strings.Repeat("?,", len(scope.AssetTypes)), ",")
		for _, t := range scope.AssetTypes {
			args = append(args, t)
		}
		clause += " AND s.asset_type IN (" + marks + ")"
	}
	if scope.Sector != "" {
		clause += " AND s.symbol IN (SELECT ss.symbol FROM stock_sectors ss WHERE ss.sector = ?)"
		args = append(args, scope.Sector)
	}
	return clause, args
}

/*
screenRuleClause — 조건 한 줄을 SQL로.

`field.Expr`은 화이트리스트에서 온 **코드 안의 문자열**이다. 사용자가 보낸 값은
`Min`/`Max`뿐이고 그 둘은 placeholder로 나간다.
*/
func screenRuleClause(field model.ScreenField, rule model.ScreenRule) (string, []any) {
	var parts []string
	var args []any
	if rule.Min != nil {
		parts = append(parts, field.Expr+" >= ?")
		args = append(args, *rule.Min)
	}
	if rule.Max != nil {
		parts = append(parts, field.Expr+" <= ?")
		args = append(args, *rule.Max)
	}
	if len(parts) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(parts, " AND "), args
}

/*
ScreenCounts — 깔때기.

한 번의 집계로 네 숫자를 전부 낸다. 네 번 물으면 같은 범위를 네 번 스캔한다.

	total       그날 점수가 계산된 전체 (시장 무관)
	inMarket    시장으로 줄인 뒤
	eligible    랭킹 자격을 갖춘 종목
	measurable  **건 조건을 전부 잴 수 있는** 종목 (값이 NULL이 아닌)
	passed      조건까지 통과한 종목

`measurable`과 `passed`의 차이가 「조건을 틀렸다」이고, `eligible`과 `measurable`의
차이가 「재보지 못했다」다. 이 둘을 한 숫자로 합치면 화면이 거짓말을 하게 된다.
*/
func (r *ScreenerRepo) ScreenCounts(
	ctx context.Context,
	tradeDate string,
	scope ScreenScope,
	rules []model.ScreenRule,
) (model.ScreenFunnel, error) {
	var out model.ScreenFunnel
	if r == nil || r.db == nil || tradeDate == "" {
		return out, nil
	}

	marketClause, marketArgs := screenAssetTypeClause(scope)
	eligibleClause, eligibleArgs := screenEligibleClause()

	// 조건에 쓰인 필드가 전부 NULL이 아닌지 — 「잴 수 있음」의 정의.
	var measurableParts []string
	var filterClause strings.Builder
	var filterArgs []any
	for _, rule := range rules {
		field, ok := model.LookupScreenField(rule.Field)
		if !ok || !rule.Active() {
			continue
		}
		measurableParts = append(measurableParts, "("+field.Expr+") IS NOT NULL")
		clause, args := screenRuleClause(field, rule)
		filterClause.WriteString(clause)
		filterArgs = append(filterArgs, args...)
	}
	measurableExpr := "1"
	if len(measurableParts) > 0 {
		measurableExpr = strings.Join(measurableParts, " AND ")
	}

	/*
		SUM(조건)으로 한 번에 센다. `inMarket`은 시장 절, `eligible`은 그 위에 자격 절,
		`measurable`은 그 위에 NULL 검사, `passed`는 그 위에 사용자 조건이다 —
		**각 단계가 앞 단계를 포함한다**는 것이 깔때기의 뜻이다.
	*/
	query := `
		SELECT COUNT(*),
		       SUM(inMarket), SUM(inMarket AND eligible),
		       SUM(inMarket AND eligible AND measurable),
		       SUM(inMarket AND eligible AND measurable AND passed)
		  FROM (
		    SELECT
		      CASE WHEN 1=1` + marketClause + ` THEN 1 ELSE 0 END AS inMarket,
		      CASE WHEN 1=1` + eligibleClause + ` THEN 1 ELSE 0 END AS eligible,
		      CASE WHEN ` + measurableExpr + ` THEN 1 ELSE 0 END AS measurable,
		      CASE WHEN 1=1` + filterClause.String() + ` THEN 1 ELSE 0 END AS passed
		      FROM stock_scores sc
		      STRAIGHT_JOIN stocks s ON s.symbol = sc.symbol
		     WHERE sc.trade_date = ?
		  ) x`

	args := make([]any, 0, len(marketArgs)+len(eligibleArgs)+len(filterArgs)+1)
	args = append(args, marketArgs...)
	args = append(args, eligibleArgs...)
	args = append(args, filterArgs...)
	args = append(args, tradeDate)

	var total, inMarket, eligible, measurable, passed sql.NullInt64
	if err := r.db.QueryRowContext(ctx, query, args...).
		Scan(&total, &inMarket, &eligible, &measurable, &passed); err != nil {
		return out, fmt.Errorf("screen counts: %w", err)
	}
	out.Total = int(total.Int64)
	out.InMarket = int(inMarket.Int64)
	out.Eligible = int(eligible.Int64)
	out.Measurable = int(measurable.Int64)
	out.Passed = int(passed.Int64)
	return out, nil
}

/*
ScreenCoverage — 필드마다 **이 시장에서** 몇 종목이 측정됐나.

분모를 전체가 아니라 시장으로 잡는 이유: 국내만 보는 사람에게 해외 커버리지는 뜻이
없다. 자격(`eligible`)까지 통과한 종목을 세는 이유: 그것이 실제로 결과에 오를 수 있는
집합이기 때문이다.

필드 수만큼 `SUM()`이 붙지만 스캔은 **한 번**이다. 실측 23개 필드에 0.2초 미만.
*/
func (r *ScreenerRepo) ScreenCoverage(
	ctx context.Context,
	tradeDate string,
	scope ScreenScope,
	fields []model.ScreenField,
) (map[string]int, int, error) {
	if r == nil || r.db == nil || tradeDate == "" || len(fields) == 0 {
		return nil, 0, nil
	}
	marketClause, marketArgs := screenAssetTypeClause(scope)
	eligibleClause, eligibleArgs := screenEligibleClause()

	sums := make([]string, 0, len(fields))
	for _, f := range fields {
		sums = append(sums, "SUM(("+f.Expr+") IS NOT NULL)")
	}

	query := `SELECT COUNT(*), ` + strings.Join(sums, ", ") + `
		  FROM stock_scores sc
		  STRAIGHT_JOIN stocks s ON s.symbol = sc.symbol
		 WHERE sc.trade_date = ?` + marketClause + eligibleClause

	args := make([]any, 0, 1+len(marketArgs)+len(eligibleArgs))
	args = append(args, tradeDate)
	args = append(args, marketArgs...)
	args = append(args, eligibleArgs...)

	// Scan 대상은 분모 하나 + 필드 수만큼.
	cells := make([]sql.NullInt64, len(fields)+1)
	targets := make([]any, len(cells))
	for i := range cells {
		targets[i] = &cells[i]
	}
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(targets...); err != nil {
		return nil, 0, fmt.Errorf("screen coverage: %w", err)
	}

	out := make(map[string]int, len(fields))
	for i, f := range fields {
		out[f.ID] = int(cells[i+1].Int64)
	}
	return out, int(cells[0].Int64), nil
}

/*
ScreenRows — 결과 ≤200줄.

여기서만 `stock_daily_closes`를 붙인다. 살아남은 줄에만 붙으므로 조인 비용이 결과
크기에 비례한다 — 거르는 단계에 두면 하루치 전체에 비례한다.

`LATERAL`을 쓰는 이유는 랭킹 목록과 같다: 점수일에 종가가 아직 없는 종목이 있어서
**그 날짜 이하의 가장 최근 종가**를 집어야 한다.
*/
// screenWhere — ScreenRows·ScreenSymbols가 같이 쓰는 조건절(점수일 · 시장 · 자격 · 조건).
func screenWhere(tradeDate string, scope ScreenScope, rules []model.ScreenRule) (string, []any) {
	marketClause, marketArgs := screenAssetTypeClause(scope)
	eligibleClause, eligibleArgs := screenEligibleClause()
	var filterClause strings.Builder
	args := append([]any{tradeDate}, marketArgs...)
	args = append(args, eligibleArgs...)
	for _, rule := range rules {
		field, ok := model.LookupScreenField(rule.Field)
		if !ok || !rule.Active() {
			continue
		}
		clause, ruleArgs := screenRuleClause(field, rule)
		filterClause.WriteString(clause)
		args = append(args, ruleArgs...)
	}
	return "sc.trade_date = ?" + marketClause + eligibleClause + filterClause.String(), args
}

// ScreenHit is one passing symbol, without prices — preset cards only need who passed.
type ScreenHit struct {
	Symbol string
	Name   string
}

// ScreenSymbols lists every symbol passing the rules on tradeDate, best score first.
// 시세 조인이 없어 ScreenRows보다 가볍다 — 프리셋 카드의 「어제 대비 새로 들어온 종목」이 쓴다(2026-10-10).
func (r *ScreenerRepo) ScreenSymbols(ctx context.Context, tradeDate string, scope ScreenScope, rules []model.ScreenRule) ([]ScreenHit, error) {
	if r == nil || r.db == nil || tradeDate == "" {
		return nil, nil
	}
	where, args := screenWhere(tradeDate, scope, rules)
	rows, err := r.db.QueryContext(ctx, `
		SELECT sc.symbol, s.name FROM stock_scores sc STRAIGHT_JOIN stocks s ON s.symbol = sc.symbol
		 WHERE `+where+` ORDER BY sc.score DESC, sc.symbol`, args...)
	if err != nil {
		return nil, fmt.Errorf("screen symbols: %w", err)
	}
	defer rows.Close()
	var out []ScreenHit
	for rows.Next() {
		var h ScreenHit
		if err := rows.Scan(&h.Symbol, &h.Name); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// PrevScoredDate — tradeDate 바로 앞 점수일. 없으면 빈 값.
func (r *ScreenerRepo) PrevScoredDate(ctx context.Context, tradeDate string) (string, error) {
	var d sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT DATE_FORMAT(MAX(trade_date), '%Y-%m-%d') FROM stock_scores WHERE trade_date < ?`, tradeDate).Scan(&d)
	return d.String, err
}

func (r *ScreenerRepo) ScreenRows(
	ctx context.Context,
	tradeDate string,
	scope ScreenScope,
	rules []model.ScreenRule,
	cellFields []model.ScreenField,
	sortExpr string,
	sortAsc bool,
	limit int,
) ([]model.ScreenRow, error) {
	if r == nil || r.db == nil || tradeDate == "" {
		return nil, nil
	}
	whereClause, whereArgs := screenWhere(tradeDate, scope, rules)

	// 결과표의 열 — 사용자가 건 조건들. 전부 코드 안의 식이다.
	cellSelects := make([]string, 0, len(cellFields))
	for _, f := range cellFields {
		cellSelects = append(cellSelects, "("+f.Expr+")")
	}
	cellClause := ""
	if len(cellSelects) > 0 {
		cellClause = ", " + strings.Join(cellSelects, ", ")
	}

	direction := " DESC"
	if sortAsc {
		direction = " ASC"
	}
	if sortExpr == "" {
		sortExpr = "sc.score"
	}

	query := `
		SELECT sc.symbol, s.name, s.asset_type, sc.score, sc.grade,
		       d.close_price, d.change_rate, d.prev_close, d.currency,
		       DATE_FORMAT(d.trade_date, '%Y-%m-%d')` + cellClause + `
		  FROM stock_scores sc
		  STRAIGHT_JOIN stocks s ON s.symbol = sc.symbol
		  LEFT JOIN LATERAL (
		        SELECT c.close_price, c.change_rate, c.prev_close, c.currency, c.trade_date
		          FROM stock_daily_closes c
		         WHERE c.symbol = sc.symbol AND c.trade_date <= sc.trade_date
		         ORDER BY c.trade_date DESC
		         LIMIT 1
		       ) d ON TRUE
		 WHERE ` + whereClause + `
		 ORDER BY (` + sortExpr + `) IS NULL, (` + sortExpr + `)` + direction + `, sc.symbol ASC
		 LIMIT ?`

	args := append(whereArgs, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("screen rows: %w", err)
	}
	defer rows.Close()

	out := make([]model.ScreenRow, 0, limit)
	for rows.Next() {
		var (
			row        model.ScreenRow
			score      sql.NullInt64
			grade      sql.NullString
			closePrice sql.NullFloat64
			changeRate sql.NullFloat64
			prevClose  sql.NullFloat64
			currency   sql.NullString
			closeDate  sql.NullString
		)
		cells := make([]sql.NullFloat64, len(cellFields))
		targets := []any{&row.Symbol, &row.Name, &row.AssetType, &score, &grade,
			&closePrice, &changeRate, &prevClose, &currency, &closeDate}
		for i := range cells {
			targets = append(targets, &cells[i])
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		if score.Valid {
			v := int(score.Int64)
			row.Score = &v
		}
		row.Grade = grade.String
		if closePrice.Valid {
			row.Close = &closePrice.Float64
		}
		if changeRate.Valid {
			row.ChangeRate = &changeRate.Float64
		}
		if prevClose.Valid {
			row.PrevClose = &prevClose.Float64
		}
		row.Currency = currency.String
		// 출처를 언제나 밝힌다 — 랭킹 행과 같은 규칙이다(`stock_scores.go` 참고).
		// 점수의 날과 같을 때만 비우면 해외 종목이 늘 「밀린 값」으로 읽힌다.
		if closeDate.Valid {
			row.PriceAsOf = closeDate.String
		}

		row.Cells = make([]model.ScreenCell, 0, len(cellFields))
		for i, f := range cellFields {
			cell := model.ScreenCell{Field: f.ID}
			if cells[i].Valid {
				v := cells[i].Float64
				if f.Scale > 0 {
					v /= f.Scale
				}
				row.Cells = append(row.Cells, model.ScreenCell{Field: f.ID, Value: &v})
				continue
			}
			// 값이 없으면 **빈칸이 아니라 nil**로 내려간다 — 화면이 「-」로 그린다.
			row.Cells = append(row.Cells, cell)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ScreenSymbolMeta is what the reference-stock and tag paths need besides values.
type ScreenSymbolMeta struct {
	Name      string
	AssetType string
	// Sector — 공통 11섹터 키(없으면 ""). 「닮은꼴」 정렬이 같은 업종을 앞에 둔다(2026-10-01 요청 1).
	Sector string
}

/*
SymbolValues — 종목 몇 개의 스크리너 값 전부(화면 단위). 자격 조건을 걸지 않는다 — 기준 종목·
글 태그는 랭킹 밖 종목도 가리킬 수 있다. 스크리너 결과표와 같은 식(`Expr`)이라 값이 어긋나지 않는다.
*/
func (r *ScreenerRepo) SymbolValues(
	ctx context.Context, tradeDate string, symbols []string,
) (map[string]map[string]float64, map[string]ScreenSymbolMeta, error) {
	values := map[string]map[string]float64{}
	meta := map[string]ScreenSymbolMeta{}
	if r == nil || r.db == nil || tradeDate == "" || len(symbols) == 0 {
		return values, meta, nil
	}
	in, symArgs := symbolPlaceholders(symbols)
	if in == "" {
		return values, meta, nil
	}
	cols := make([]string, 0, len(model.ScreenFields))
	for _, f := range model.ScreenFields {
		cols = append(cols, "("+f.Expr+")")
	}
	args := append([]any{tradeDate}, symArgs...)
	rows, err := r.db.QueryContext(ctx, `
		SELECT sc.symbol, s.name, s.asset_type, COALESCE(ss.sector, ''), `+strings.Join(cols, ", ")+`
		  FROM stock_scores sc
		  JOIN stocks s ON s.symbol = sc.symbol
		  LEFT JOIN stock_sectors ss ON ss.symbol = sc.symbol
		 WHERE sc.trade_date = ? AND sc.symbol IN (`+in+`)`, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("screen symbol values: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sym, name, assetType, sector string
		cells := make([]sql.NullFloat64, len(model.ScreenFields))
		targets := []any{&sym, &name, &assetType, &sector}
		for i := range cells {
			targets = append(targets, &cells[i])
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, nil, err
		}
		v := map[string]float64{}
		for i, f := range model.ScreenFields {
			if !cells[i].Valid {
				continue
			}
			x := cells[i].Float64
			if f.Scale > 0 {
				x /= f.Scale
			}
			v[f.ID] = x
		}
		values[sym] = v
		meta[sym] = ScreenSymbolMeta{Name: name, AssetType: assetType, Sector: sector}
	}
	return values, meta, rows.Err()
}

/*
ScreenQuantiles — 조건 칸의 「보통 25~75%」 가이드(2026-09-28 요청 5). 자격 종목의 값을 한 번에
읽어 Go에서 나눈다 — 필드마다 정렬 질의를 26번 하지 않는다. 카탈로그 캐시(15분) 안에서만 불린다.
*/
func (r *ScreenerRepo) ScreenQuantiles(
	ctx context.Context, tradeDate string, scope ScreenScope, fields []model.ScreenField,
) (map[string][3]float64, error) {
	out := map[string][3]float64{}
	if r == nil || r.db == nil || tradeDate == "" || len(fields) == 0 {
		return out, nil
	}
	marketClause, marketArgs := screenAssetTypeClause(scope)
	eligibleClause, eligibleArgs := screenEligibleClause()
	cols := make([]string, 0, len(fields))
	for _, f := range fields {
		cols = append(cols, "("+f.Expr+")")
	}
	args := append([]any{tradeDate}, marketArgs...)
	args = append(args, eligibleArgs...)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+strings.Join(cols, ", ")+`
		  FROM stock_scores sc
		  STRAIGHT_JOIN stocks s ON s.symbol = sc.symbol
		 WHERE sc.trade_date = ?`+marketClause+eligibleClause, args...)
	if err != nil {
		return nil, fmt.Errorf("screen quantiles: %w", err)
	}
	defer rows.Close()
	series := make([][]float64, len(fields))
	cells := make([]sql.NullFloat64, len(fields))
	targets := make([]any, len(fields))
	for i := range cells {
		targets[i] = &cells[i]
	}
	for rows.Next() {
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		for i := range cells {
			if cells[i].Valid {
				series[i] = append(series[i], cells[i].Float64)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, f := range fields {
		if q, ok := Quartiles(series[i]); ok {
			if f.Scale > 0 {
				for j := range q {
					q[j] /= f.Scale
				}
			}
			out[f.ID] = q
		}
	}
	return out, nil
}

// Quartiles returns p25·p50·p75 (nearest rank). 표본이 20개 미만이면 가이드로 쓰지 않는다.
func Quartiles(v []float64) ([3]float64, bool) {
	if len(v) < 20 {
		return [3]float64{}, false
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	at := func(p float64) float64 { return s[int(p*float64(len(s)-1)+0.5)] }
	return [3]float64{at(.25), at(.5), at(.75)}, true
}
