package service

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

const dartAnnualReportCode = "11011"

const (
	dartFSConsolidated = "CFS" // 연결재무제표 — 대형·지주사 등 다수가 제출
	dartFSSeparate     = "OFS" // 별도재무제표 — HPSP 등 자회사 없는 상장사는 이것만 있는 경우가 많다
)

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

// 계정명 매칭은 dart_account_match.go로 옮겼다 — DART가 같은 개념을 회사마다,
// 같은 회사의 분기마다 다른 이름으로 주기 때문에 완전 일치 목록으로는 감당이
// 안 됐다(실측 결측률: 매출 21.3% · EPS 32.2%).

type dartAccountRow struct {
	AccountName string `json:"account_nm"`
	// StatementDiv("sj_div")는 이 행이 어느 재무제표의 것인지 말한다
	// (BS/IS/CIS/CF/SCE). 손익 항목을 재무상태표에서 찾지 않게 막는다 —
	// `매출채권`(BS)이 매출로 잡히는 부류의 사고를 원천 차단한다.
	StatementDiv string `json:"sj_div"`
	Amount       string `json:"thstrm_amount"`
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
	rows, err := d.fetchFinancialAccountsFS(ctx, corpCode, bsnsYear, reprtCode, dartFSConsolidated)
	if err == nil {
		return rows, nil
	}
	if !dartFSNoDataErr(err) {
		return nil, err
	}
	return d.fetchFinancialAccountsFS(ctx, corpCode, bsnsYear, reprtCode, dartFSSeparate)
}

func dartFSNoDataErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no data for year")
}

func (d *DartClient) fetchFinancialAccountsFS(ctx context.Context, corpCode string, bsnsYear int, reprtCode, fsDiv string) ([]dartAccountRow, error) {
	u := d.dartFinancialAccountsURL(corpCode, bsnsYear, reprtCode, fsDiv)
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
		if payload.Status == "020" {
			return nil, fmt.Errorf("%w (%s)", ErrDartQuotaExceeded, payload.Message)
		}
		return nil, fmt.Errorf("dart fnlttSinglAcntAll: %s", payload.Message)
	}
	return payload.List, nil
}

func (d *DartClient) dartFinancialAccountsURL(corpCode string, bsnsYear int, reprtCode, fsDiv string) string {
	base := "https://opendart.fss.or.kr/api/fnlttSinglAcntAll.json"
	if d != nil && d.financialAPIBase != "" {
		base = strings.TrimRight(d.financialAPIBase, "/")
	}
	return fmt.Sprintf(
		"%s?crtfc_key=%s&corp_code=%s&bsns_year=%d&reprt_code=%s&fs_div=%s",
		base,
		url.QueryEscape(d.cfg.APIKey),
		url.QueryEscape(corpCode),
		bsnsYear,
		reprtCode,
		fsDiv,
	)
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
			if IsDartQuotaExceeded(err) {
				return nil
			}
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
		yearly, err := d.fetchStandaloneQuartersForYear(ctx, corpCode, currentYear-offset)
		if err != nil {
			if IsDartQuotaExceeded(err) {
				return nil, err
			}
			continue
		}
		out = append(out, yearly...)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("dart: no quarterly history found for %s", symbol)
	}
	if len(out) > wantQuarters {
		out = out[len(out)-wantQuarters:]
	}
	return out, nil
}

func (d *DartClient) fetchStandaloneQuartersForYear(ctx context.Context, corpCode string, year int) ([]model.FinancialHistoryPeriod, error) {
	rowsByCode := map[string][]dartAccountRow{}
	for _, rc := range dartQuarterlyReportCodes {
		rows, err := d.fetchFinancialAccounts(ctx, corpCode, year, rc.Code)
		if err != nil {
			// 한도 소진은 건너뛸 수 있는 실패가 아니다 — 그 뒤 모든 호출이 같은
			// 이유로 실패한다. 즉시 멈춰 호출자가 배치를 세울 수 있게 한다.
			if IsDartQuotaExceeded(err) {
				return nil, err
			}
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
	return out, nil
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
	pick := func(field string) *int64 { return toInt64Ptr(pickDartTiered(rows, field, dartAmountCurrent)) }

	var snap model.FinancialSnapshot
	snap.Revenue = pick("revenue")
	snap.OperatingIncome = pick("operating_income")
	snap.NetIncome = pick("net_income")
	snap.TotalAssets = pick("total_assets")
	snap.TotalEquity = pick("total_equity")
	snap.TotalLiabilities = pick("total_liabilities")
	snap.CurrentAssets = pick("current_assets")
	snap.CurrentLiabilities = pick("current_liabilities")
	snap.EPSBasic = pickDartTiered(rows, "eps_basic", dartAmountCurrent)
	snap.EPSDiluted = firstNonNilFloat(
		pickDartTiered(rows, "eps_diluted", dartAmountCurrent),
		snap.EPSBasic,
	)
	return snap
}

func toInt64Ptr(v *float64) *int64 {
	if v == nil {
		return nil
	}
	i := int64(mathRound(*v))
	return &i
}

// mapDartCumulativeRows reads thstrm_add_amount (cumulative-since-fiscal-
// year-start) instead of thstrm_amount — only meaningful on interim reports,
// used to derive Q4 (see FetchQuarterlyHistory).
func mapDartCumulativeRows(rows []dartAccountRow) model.FinancialSnapshot {
	pick := func(field string) *int64 { return toInt64Ptr(pickDartTiered(rows, field, dartAmountAdd)) }

	var snap model.FinancialSnapshot
	snap.Revenue = pick("revenue")
	snap.OperatingIncome = pick("operating_income")
	snap.NetIncome = pick("net_income")
	snap.EPSBasic = pickDartTiered(rows, "eps_basic", dartAmountAdd)
	return snap
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

// ErrDartQuotaExceeded — DART 일일 사용한도 소진(status 020).
//
// **다른 오류와 갈라야 한다.** 「그 해에 그 보고서가 없다」(013)는 정상이고 건너뛰면
// 되지만, 한도 소진은 **그 뒤 모든 호출이 똑같이 실패**한다. 구분하지 않으면 배치가
// 남은 수백 종목을 계속 돌면서 아무것도 못 쓰고, 「구멍 그대로 · ok=true」로 보고한다
// (2026-09-02 실측: 705종목 전부 ok=true인데 코호트가 한 종목도 안 줄었다).
var ErrDartQuotaExceeded = errors.New("dart: 일일 사용한도 초과")

// IsDartQuotaExceeded reports whether err is (or wraps) a DART quota rejection.
func IsDartQuotaExceeded(err error) bool {
	return errors.Is(err, ErrDartQuotaExceeded)
}

// dartErrorPayload는 응답이 DART의 거절인지 본다 — JSON과 XML 두 꼴 모두.
//
// **DART는 거절도 HTTP 200으로 준다.** 그래서 전송 계층은 성공이라고 말한다.
// ZIP(corpCode)이나 JSON(재무)을 기대하는 자리에서 이걸 못 알아보면 빈 결과가
// 조용히 만들어진다 — 아무도 실패라고 말하지 않는다.
//
// 실측 2026-09-02: 한도 소진 시 `corpCode.xml`이
// `<result><status>020</status>…</result>` 149바이트를 200으로 돌려줬다.
// 이것을 corp_code 목록으로 파싱하면 **빈 맵**이 되고, 그 뒤 모든 종목이
// 「corp_code not found」로 떨어진다. 705종목이 전부 그렇게 실패하고도
// ok로 보고된 이유다.
func dartErrorPayload(body []byte) (status, message string, ok bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return "", "", false
	}
	var payload struct {
		Status  string `json:"status" xml:"status"`
		Message string `json:"message" xml:"message"`
	}
	switch trimmed[0] {
	case 0x7b: // JSON
		if err := json.Unmarshal(trimmed, &payload); err != nil {
			return "", "", false
		}
	case 0x3c: // XML — corpCode.xml은 오류를 이 꼴로 준다
		if err := xml.Unmarshal(trimmed, &payload); err != nil {
			return "", "", false
		}
	default:
		return "", "", false
	}
	if payload.Status == "" || payload.Status == "000" {
		return "", "", false
	}
	return payload.Status, payload.Message, true
}
