package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

/*
FetchIndustryCode — 한 종목의 DART 기업개황 `induty_code`(KSIC)와 종목명.

종목명을 함께 돌려주는 이유: 지주회사 코드는 이름으로만 금융지주를 가를 수 있다
(`model.SectorFromKSICNamed`).

`company.json` 한 호출이다. 간격은 `fetchURL`이 기다려서 지키고, 다른 DART 배치와의
순서는 부르는 쪽이 `dartGate`로 맞춘다(insider_ingest.go와 같다).

DART 상태 `013`(조회된 데이터 없음)은 실패가 아니라 「이 소스에 없다」다 — 빈 코드로 돌려준다.
*/
func (d *DartClient) FetchIndustryCode(ctx context.Context, symbol string) (code, name string, err error) {
	if !d.cfg.Enabled() {
		return "", "", fmt.Errorf("dart api key not configured")
	}
	if err := d.ensureCorpCodes(ctx); err != nil {
		return "", "", err
	}
	corpCode := d.lookupCorpCode(symbol)
	if corpCode == "" {
		// 우선주(001275 · 00279K)는 corpCode 목록에 없다 — 발행 법인은 보통주와
		// 같으므로 끝자리를 0으로 바꾼 보통주 코드로 묻는다. 산업은 법인의 속성이다.
		if base := krCommonShareCode(symbol); base != "" {
			corpCode = d.lookupCorpCode(base)
		}
	}
	if corpCode == "" {
		return "", "", fmt.Errorf("dart: corp_code not found for %s", symbol)
	}
	u := fmt.Sprintf(
		"https://opendart.fss.or.kr/api/company.json?crtfc_key=%s&corp_code=%s",
		url.QueryEscape(d.cfg.APIKey),
		url.QueryEscape(corpCode),
	)
	body, err := d.fetchURL(ctx, u)
	if err != nil {
		return "", "", err
	}
	var payload struct {
		Status     string `json:"status"`
		Message    string `json:"message"`
		IndutyCode string `json:"induty_code"`
		StockName  string `json:"stock_name"`
		CorpName   string `json:"corp_name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", fmt.Errorf("dart company %s: %w", symbol, err)
	}
	switch payload.Status {
	case "000":
		name := strings.TrimSpace(payload.StockName)
		if name == "" {
			name = strings.TrimSpace(payload.CorpName)
		}
		return strings.TrimSpace(payload.IndutyCode), name, nil
	case "013":
		return "", "", nil
	}
	return "", "", fmt.Errorf("dart company %s: %s %s", symbol, payload.Status, payload.Message)
}

// krCommonShareCode maps a 6-character KRX preferred-share code to its common
// share (끝자리 → 0). 보통주 자신이나 모양이 다른 코드는 "".
func krCommonShareCode(symbol string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	if len(s) != 6 || s[5] == '0' {
		return ""
	}
	return s[:5] + "0"
}
