package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

// dartInsiderMaxItems caps how many 소유보고 rows one symbol contributes —
// elestock.json has no date-range parameter and returns a corp's entire
// reporting history in one call, which runs to thousands of rows for a
// large-cap with many registered executives.
//
// 화면이 보여주는 것은 30건인데 100건을 담는 이유: 호출은 어차피 한 번이고,
// 나중에 "최근 1년 매수/매도 합계" 같은 집계를 붙일 때 다시 받으러 가지 않아도
// 되는 여유다. 전부 담지 않는 것은 종목 수(약 2,800)와 곱해지기 때문이다.
const dartInsiderMaxItems = 100

// FetchInsiderRows returns 임원ㆍ주요주주 소유보고 (elestock.json) history for
// symbol, most recent first, capped at dartInsiderMaxItems.
//
// 저장 키는 행 내용의 해시다. DART는 이 응답에 접수번호를 주지 않아 자연키가
// 없는데, 같은 보고가 다음 실행에도 같은 내용으로 오므로 해시가 안정적인
// 식별자 역할을 한다 — 재실행이 같은 행을 다시 넣지 않는다.
func (d *DartClient) FetchInsiderRows(ctx context.Context, symbol string) ([]model.InsiderTransactionRow, error) {
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

	u := fmt.Sprintf(
		"https://opendart.fss.or.kr/api/elestock.json?crtfc_key=%s&corp_code=%s",
		url.QueryEscape(d.cfg.APIKey),
		url.QueryEscape(corpCode),
	)
	body, err := d.fetchURL(ctx, u)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		List    []struct {
			RceptDt            string `json:"rcept_dt"`
			Repror             string `json:"repror"`
			IsuExctvRgistAt    string `json:"isu_exctv_rgist_at"`
			IsuExctvOfcps      string `json:"isu_exctv_ofcps"`
			IsuMainShrholdr    string `json:"isu_main_shrholdr"`
			SpStockLmpCnt      string `json:"sp_stock_lmp_cnt"`
			SpStockLmpIrdsCnt  string `json:"sp_stock_lmp_irds_cnt"`
			SpStockLmpRate     string `json:"sp_stock_lmp_rate"`
			SpStockLmpIrdsRate string `json:"sp_stock_lmp_irds_rate"`
		} `json:"list"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	// DART: status "013" means "조회된 데이터가 없습니다" — a normal empty
	// result, not an error (e.g. a symbol with no registered executive filings).
	if payload.Status == "013" {
		return []model.InsiderTransactionRow{}, nil
	}
	if payload.Status != "000" {
		return nil, fmt.Errorf("dart elestock: %s", payload.Message)
	}

	out := make([]model.InsiderTransactionRow, 0, len(payload.List))
	for _, item := range payload.List {
		change := parseDartSignedInt(item.SpStockLmpIrdsCnt)
		txn := model.InsiderTransaction{
			Date:             formatDartDisclosureDate(item.RceptDt),
			ReporterName:     strings.TrimSpace(item.Repror),
			Relation:         dartInsiderRelation(item.IsuExctvRgistAt, item.IsuExctvOfcps, item.IsuMainShrholdr),
			TransactionType:  dartInsiderChangeLabel(change),
			SharesChange:     change,
			ChangeRate:       parseDartSignedFloat(item.SpStockLmpIrdsRate),
			SharesOwnedAfter: parseDartSignedInt(item.SpStockLmpCnt),
			OwnershipRate:    parseDartSignedFloat(item.SpStockLmpRate),
			Source:           "dart",
		}
		out = append(out, model.InsiderTransactionRow{
			Symbol:    symbol,
			FilingKey: dartInsiderFilingKey(item.RceptDt, item.Repror, item.IsuExctvOfcps, item.SpStockLmpCnt, item.SpStockLmpIrdsCnt),
			Txn:       txn,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Txn.Date > out[j].Txn.Date })
	if len(out) > dartInsiderMaxItems {
		out = out[:dartInsiderMaxItems]
	}
	return out, nil
}

// dartInsiderFilingKey builds the stable identity of one 소유보고 row.
//
// 접수일·보고자·직위·보유수량·증감수량을 모두 넣는다. 같은 사람이 같은 날 두
// 건을 신고하는 경우가 있어 접수일+보고자만으로는 한 건이 다른 건을 덮어쓴다.
func dartInsiderFilingKey(parts ...string) string {
	joined := make([]string, 0, len(parts))
	for _, p := range parts {
		joined = append(joined, strings.TrimSpace(p))
	}
	sum := sha1.Sum([]byte(strings.Join(joined, "|")))
	return hex.EncodeToString(sum[:])
}

func dartInsiderRelation(rgistAt, ofcps, mainShrholdr string) string {
	var parts []string
	for _, v := range []string{rgistAt, ofcps, mainShrholdr} {
		v = strings.TrimSpace(v)
		if v != "" && v != "-" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, " · ")
}

func dartInsiderChangeLabel(change *int64) string {
	if change == nil || *change == 0 {
		return "변동없음"
	}
	if *change > 0 {
		return "증가"
	}
	return "감소"
}

func parseDartSignedInt(raw string) *int64 {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, ",", ""))
	if raw == "" || raw == "-" {
		return nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil
	}
	return &v
}

func parseDartSignedFloat(raw string) *float64 {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, ",", ""))
	if raw == "" || raw == "-" {
		return nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil
	}
	return &v
}
