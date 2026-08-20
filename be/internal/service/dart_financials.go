package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

const dartAnnualReportCode = "11011"

// dartQuarterlyReportCodes drives 분기별 실적 상세 — DART reports cumulative
// figures under these codes (1Q, 1H cumulative, 9M cumulative, then FY).
// Rank orders them chronologically within a fiscal year for "most recent first" sort.
var dartQuarterlyReportCodes = []struct {
	Code, Label string
	Rank        int
}{
	{"11013", "1Q", 1},
	{"11012", "반기", 2},
	{"11014", "3Q", 3},
	{"11011", "연간", 4},
}

// DART appends "(손실)" to an account name whenever any comparative column
// in that particular filing shows a loss — the same concept can appear as
// "영업이익" in one year's report and "영업이익(손실)" in another for the
// same company. dartAccountNameMatches (below) treats these as the same
// account rather than enumerating every suffixed variant here.
var dartAccountFallbacks = map[string][]string{
	"revenue":             {"매출액", "수익(매출액)"},
	"operating_income":    {"영업이익"},
	"net_income":          {"당기순이익", "분기순이익", "반기순이익"},
	"total_assets":        {"자산총계"},
	"total_equity":        {"자본총계"},
	"total_liabilities":   {"부채총계"},
	"current_assets":      {"유동자산"},
	"current_liabilities": {"유동부채"},
	"eps_basic":           {"기본주당이익", "주당순이익"},
	"eps_diluted":         {"희석주당이익"},
}

type dartAccountRow struct {
	AccountName string `json:"account_nm"`
	Amount      string `json:"thstrm_amount"`
	// AddAmount ("당기 누적") is only present on interim reports (반기/분기) —
	// thstrm_amount there is already the standalone period; AddAmount is the
	// cumulative-since-fiscal-year-start figure, needed to derive Q4 (DART has
	// no standalone-Q4 report code: FY total − 9-month cumulative = Q4).
	AddAmount string `json:"thstrm_add_amount"`
}

func (d *DartClient) FetchAnnualFinancialSnapshots(ctx context.Context, symbol string, year int) (model.FinancialSnapshot, error) {
	if !d.cfg.Enabled() {
		return model.FinancialSnapshot{}, fmt.Errorf("dart api key not configured")
	}
	if err := d.ensureCorpCodes(ctx); err != nil {
		return model.FinancialSnapshot{}, err
	}
	corpCode := d.lookupCorpCode(symbol)
	if corpCode == "" {
		return model.FinancialSnapshot{}, fmt.Errorf("dart: corp_code not found for %s", symbol)
	}

	rows, err := d.fetchFinancialAccounts(ctx, corpCode, year, dartAnnualReportCode)
	if err != nil {
		return model.FinancialSnapshot{}, err
	}

	snap := mapDartRows(rows)
	snap.FiscalYear = year
	snap.FiscalPeriod = "FY"
	snap.ReportCode = dartAnnualReportCode
	snap.PeriodEnd = calendarQuarterEnd(year, 4) // DART annual report = calendar year-end for KRX filers
	return snap, nil
}

func (d *DartClient) fetchFinancialAccounts(ctx context.Context, corpCode string, bsnsYear int, reprtCode string) ([]dartAccountRow, error) {
	u := fmt.Sprintf(
		"https://opendart.fss.or.kr/api/fnlttSinglAcntAll.json?crtfc_key=%s&corp_code=%s&bsns_year=%d&reprt_code=%s&fs_div=CFS",
		url.QueryEscape(d.cfg.APIKey),
		url.QueryEscape(corpCode),
		bsnsYear,
		reprtCode,
	)
	body, err := d.fetchURL(ctx, u)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Status  string           `json:"status"`
		Message string           `json:"message"`
		List    []dartAccountRow `json:"list"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.Status != "000" {
		if payload.Status == "013" {
			return nil, fmt.Errorf("dart: no data for year %d", bsnsYear)
		}
		return nil, fmt.Errorf("dart fnlttSinglAcntAll: %s", payload.Message)
	}
	return payload.List, nil
}

// FetchQuarterlySnapshots returns this fiscal year's filed 1Q/반기/3Q/연간
// reports, most recent period first. Falls back to last year when none of
// this year's have been filed yet (e.g. early January before 1Q is due).
func (d *DartClient) FetchQuarterlySnapshots(ctx context.Context, symbol string) ([]model.FinancialSnapshot, error) {
	if !d.cfg.Enabled() {
		return nil, fmt.Errorf("dart api key not configured")
	}
	if err := d.ensureCorpCodes(ctx); err != nil {
		return nil, err
	}
	corpCode := d.lookupCorpCode(symbol)
	if corpCode == "" {
		return nil, fmt.Errorf("dart: corp_code not found for %s", symbol)
	}

	year := time.Now().Year()
	out := d.fetchQuarterlyForYear(ctx, corpCode, year)
	if len(out) == 0 {
		out = d.fetchQuarterlyForYear(ctx, corpCode, year-1)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("dart: no quarterly data found for %s", symbol)
	}
	return out, nil
}

func (d *DartClient) fetchQuarterlyForYear(ctx context.Context, corpCode string, year int) []model.FinancialSnapshot {
	var out []model.FinancialSnapshot
	ranks := map[string]int{}
	for _, rc := range dartQuarterlyReportCodes {
		rows, err := d.fetchFinancialAccounts(ctx, corpCode, year, rc.Code)
		if err != nil {
			continue // not yet filed for this period
		}
		snap := mapDartRows(rows)
		snap.FiscalYear = year
		snap.FiscalPeriod = rc.Label
		snap.ReportCode = rc.Code
		ranks[rc.Code] = rc.Rank
		out = append(out, snap)
	}
	sort.Slice(out, func(i, j int) bool { return ranks[out[i].ReportCode] > ranks[out[j].ReportCode] })
	return out
}

// FetchQuarterlyHistory returns up to `years` years of standalone-quarter
// financial history (oldest first) for the 재무제표 탭's detail modal. Q1-Q3
// come directly from DART's thstrm_amount (already standalone on interim
// reports); Q4 has no dedicated DART report and is derived as
// FY total − 9-month cumulative. Balance-sheet fields are point-in-time and
// need no such adjustment. Years with nothing filed yet are skipped.
func (d *DartClient) FetchQuarterlyHistory(ctx context.Context, symbol string, years int) ([]model.FinancialHistoryPeriod, error) {
	if !d.cfg.Enabled() {
		return nil, fmt.Errorf("dart api key not configured")
	}
	if err := d.ensureCorpCodes(ctx); err != nil {
		return nil, err
	}
	corpCode := d.lookupCorpCode(symbol)
	if corpCode == "" {
		return nil, fmt.Errorf("dart: corp_code not found for %s", symbol)
	}
	if years <= 0 {
		years = 3
	}
	wantQuarters := years * 4

	// Fetch one extra calendar year beyond the requested window. DART calls
	// are scoped per fiscal year, so early in a calendar year "최근 1년"
	// would otherwise only return the 1-2 quarters already filed for the
	// current year instead of a true trailing 4 — fetch years+1 years of
	// data, then trim to the most recent wantQuarters below.
	currentYear := time.Now().Year()
	var out []model.FinancialHistoryPeriod
	for offset := years; offset >= 0; offset-- {
		out = append(out, d.fetchStandaloneQuartersForYear(ctx, corpCode, currentYear-offset)...)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("dart: no quarterly history found for %s", symbol)
	}
	if len(out) > wantQuarters {
		out = out[len(out)-wantQuarters:]
	}
	return out, nil
}

func (d *DartClient) fetchStandaloneQuartersForYear(ctx context.Context, corpCode string, year int) []model.FinancialHistoryPeriod {
	rowsByCode := map[string][]dartAccountRow{}
	for _, rc := range dartQuarterlyReportCodes {
		rows, err := d.fetchFinancialAccounts(ctx, corpCode, year, rc.Code)
		if err != nil {
			continue // not yet filed for this period
		}
		rowsByCode[rc.Code] = rows
	}

	var out []model.FinancialHistoryPeriod
	if rows, ok := rowsByCode["11013"]; ok {
		out = append(out, historyPeriod(year, 1, mapDartRows(rows)))
	}
	if rows, ok := rowsByCode["11012"]; ok {
		out = append(out, historyPeriod(year, 2, mapDartRows(rows)))
	}
	var nineMonthCumulative model.FinancialSnapshot
	haveNineMonth := false
	if rows, ok := rowsByCode["11014"]; ok {
		out = append(out, historyPeriod(year, 3, mapDartRows(rows)))
		nineMonthCumulative = mapDartCumulativeRows(rows)
		haveNineMonth = true
	}
	if rows, ok := rowsByCode["11011"]; ok && haveNineMonth {
		annual := mapDartRows(rows)
		q4 := model.FinancialSnapshot{
			Revenue:            subtractInt64(annual.Revenue, nineMonthCumulative.Revenue),
			OperatingIncome:    subtractInt64(annual.OperatingIncome, nineMonthCumulative.OperatingIncome),
			NetIncome:          subtractInt64(annual.NetIncome, nineMonthCumulative.NetIncome),
			EPSBasic:           subtractFloat64(annual.EPSBasic, nineMonthCumulative.EPSBasic),
			TotalAssets:        annual.TotalAssets,
			TotalEquity:        annual.TotalEquity,
			TotalLiabilities:   annual.TotalLiabilities,
			CurrentAssets:      annual.CurrentAssets,
			CurrentLiabilities: annual.CurrentLiabilities,
		}
		out = append(out, historyPeriod(year, 4, q4))
	}
	return out
}

// historyPeriod converts a snapshot into a history period, computing
// per-period margins/ratios the same way ComputeStoredFundamentals does
// (fundamentals_calc.go) but independently per quarter rather than YoY.
func historyPeriod(year, quarterIndex int, snap model.FinancialSnapshot) model.FinancialHistoryPeriod {
	p := model.FinancialHistoryPeriod{
		FiscalYear:  year,
		PeriodIndex: quarterIndex,
		PeriodEnd:   calendarQuarterEnd(year, quarterIndex),
	}
	ApplyPeriodRatios(&p, snap)
	return p
}

// calendarQuarterEnd assumes a calendar fiscal year (true for the vast
// majority of KRX-listed companies, including all symbols this derivation
// path is used for).
func calendarQuarterEnd(year, quarter int) *time.Time {
	t := time.Date(year, time.Month(quarter*3)+1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
	return &t
}

func subtractInt64(a, b *int64) *int64 {
	if a == nil || b == nil {
		return nil
	}
	v := *a - *b
	return &v
}

func subtractFloat64(a, b *float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	v := *a - *b
	return &v
}

func (d *DartClient) FindLatestAnnualSnapshots(ctx context.Context, symbol string) (current, prior model.FinancialSnapshot, err error) {
	year := time.Now().Year()
	for offset := 0; offset < 5; offset++ {
		tryYear := year - offset
		snap, err := d.FetchAnnualFinancialSnapshots(ctx, symbol, tryYear)
		if err != nil {
			continue
		}
		current = snap
		prior, _ = d.FetchAnnualFinancialSnapshots(ctx, symbol, tryYear-1)
		return current, prior, nil
	}
	return current, prior, fmt.Errorf("dart: no annual financial data found for %s", symbol)
}

func mapDartRows(rows []dartAccountRow) model.FinancialSnapshot {
	var snap model.FinancialSnapshot
	snap.Revenue = pickDartAmount(rows, dartAccountFallbacks["revenue"])
	snap.OperatingIncome = pickDartAmount(rows, dartAccountFallbacks["operating_income"])
	snap.NetIncome = pickDartAmount(rows, dartAccountFallbacks["net_income"])
	snap.TotalAssets = pickDartAmount(rows, dartAccountFallbacks["total_assets"])
	snap.TotalEquity = pickDartAmount(rows, dartAccountFallbacks["total_equity"])
	snap.TotalLiabilities = pickDartAmount(rows, dartAccountFallbacks["total_liabilities"])
	snap.CurrentAssets = pickDartAmount(rows, dartAccountFallbacks["current_assets"])
	snap.CurrentLiabilities = pickDartAmount(rows, dartAccountFallbacks["current_liabilities"])
	snap.EPSBasic = pickDartFloat(rows, dartAccountFallbacks["eps_basic"])
	snap.EPSDiluted = firstNonNilFloat(
		pickDartFloat(rows, dartAccountFallbacks["eps_diluted"]),
		snap.EPSBasic,
	)
	return snap
}

// mapDartCumulativeRows reads thstrm_add_amount (cumulative-since-fiscal-
// year-start) instead of thstrm_amount — only meaningful on interim reports,
// used to derive Q4 (see FetchQuarterlyHistory).
func mapDartCumulativeRows(rows []dartAccountRow) model.FinancialSnapshot {
	var snap model.FinancialSnapshot
	snap.Revenue = pickDartAddAmount(rows, dartAccountFallbacks["revenue"])
	snap.OperatingIncome = pickDartAddAmount(rows, dartAccountFallbacks["operating_income"])
	snap.NetIncome = pickDartAddAmount(rows, dartAccountFallbacks["net_income"])
	snap.EPSBasic = pickDartAddFloat(rows, dartAccountFallbacks["eps_basic"])
	return snap
}

func pickDartAddAmount(rows []dartAccountRow, names []string) *int64 {
	v := pickDartAddFloat(rows, names)
	if v == nil {
		return nil
	}
	i := int64(mathRound(*v))
	return &i
}

func pickDartAddFloat(rows []dartAccountRow, names []string) *float64 {
	var best *float64
	var bestAbs float64
	for _, row := range rows {
		if !dartAccountNameMatchesAny(row.AccountName, names) {
			continue
		}
		if strings.TrimSpace(row.AddAmount) == "" {
			continue
		}
		val := parseDartAmount(row.AddAmount)
		abs := val
		if abs < 0 {
			abs = -abs
		}
		if best == nil || abs > bestAbs {
			v := val
			best = &v
			bestAbs = abs
		}
	}
	return best
}

// dartAccountNameMatchesAny reports whether accountName is one of names, or
// one of names with DART's conditional "(손실)" suffix appended (see
// dartAccountFallbacks doc comment).
func dartAccountNameMatchesAny(accountName string, names []string) bool {
	accountName = strings.TrimSpace(accountName)
	for _, name := range names {
		if accountName == name || accountName == name+"(손실)" {
			return true
		}
	}
	return false
}

func pickDartAmount(rows []dartAccountRow, names []string) *int64 {
	v := pickDartFloat(rows, names)
	if v == nil {
		return nil
	}
	i := int64(mathRound(*v))
	return &i
}

func pickDartFloat(rows []dartAccountRow, names []string) *float64 {
	var best *float64
	var bestAbs float64
	for _, row := range rows {
		if !dartAccountNameMatchesAny(row.AccountName, names) {
			continue
		}
		val := parseDartAmount(row.Amount)
		abs := val
		if abs < 0 {
			abs = -abs
		}
		if best == nil || abs > bestAbs {
			v := val
			best = &v
			bestAbs = abs
		}
	}
	return best
}

func parseDartAmount(raw string) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "-" {
		return 0
	}
	raw = strings.ReplaceAll(raw, ",", "")
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return v
}

func mathRound(v float64) int64 {
	if v >= 0 {
		return int64(v + 0.5)
	}
	return int64(v - 0.5)
}
