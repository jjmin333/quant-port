package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

const dartAnnualReportCode = "11011"

var dartAccountFallbacks = map[string][]string{
	"revenue":            {"매출액", "수익(매출액)"},
	"operating_income":   {"영업이익", "영업이익(손실)"},
	"net_income":         {"당기순이익", "당기순이익(손실)", "분기순이익", "분기순이익(손실)"},
	"total_assets":       {"자산총계"},
	"total_equity":       {"자본총계"},
	"total_liabilities":  {"부채총계"},
	"current_assets":     {"유동자산"},
	"current_liabilities": {"유동부채"},
	"eps_basic":          {"기본주당이익", "주당순이익"},
	"eps_diluted":        {"희석주당이익"},
}

type dartAccountRow struct {
	AccountName string `json:"account_nm"`
	Amount      string `json:"thstrm_amount"`
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

func pickDartAmount(rows []dartAccountRow, names []string) *int64 {
	v := pickDartFloat(rows, names)
	if v == nil {
		return nil
	}
	i := int64(mathRound(*v))
	return &i
}

func pickDartFloat(rows []dartAccountRow, names []string) *float64 {
	nameSet := make(map[string]struct{}, len(names))
	for _, name := range names {
		nameSet[name] = struct{}{}
	}

	var best *float64
	var bestAbs float64
	for _, row := range rows {
		if _, ok := nameSet[strings.TrimSpace(row.AccountName)]; !ok {
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
