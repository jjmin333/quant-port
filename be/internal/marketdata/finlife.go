package marketdata

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

/*
금융감독원 「금융상품 한눈에」 Open API (2026-09-29, 금융상품 비교 MVP).

	https://finlife.fss.or.kr/finlifeapi/{서비스}.json?auth=&topFinGrpNo=&pageNo=

인증키(32자리)는 finlife.fss.or.kr에서 회원가입 후 신청한다 — 환경변수 FSS_FINLIFE_API_KEY.
키 없이 부르면 200에 `{"result":{"err_cd":"010","err_msg":"미등록 인증키"}}`가 온다(09-29 실측).
http는 https로 307을 준다. 파킹통장(수시입출금)은 이 API에 없다.

응답은 `result.baseList`(상품 기본정보)와 `result.optionList`(기간·금리 옵션, fin_co_no +
fin_prdt_cd로 상품에 붙는다). 필드는 종류마다 달라 **원문 map 그대로** 넘긴다 — 여기서
숫자를 옮겨 적지 않는다.
*/

const finlifeBaseURL = "https://finlife.fss.or.kr/finlifeapi/"

// FinlifeServices maps our product kind to the API's service name.
var FinlifeServices = map[string]string{
	"deposit":  "depositProductsSearch",
	"saving":   "savingProductsSearch",
	"mortgage": "mortgageLoanProductsSearch",
	"rent":     "rentHouseLoanProductsSearch",
	"credit":   "creditLoanProductsSearch",
}

// FinlifeGroups — 권역 코드. 은행 · 여신전문 · 저축은행 · 보험 · 금융투자.
var FinlifeGroups = []string{"020000", "030200", "030300", "050000", "060000"}

type FinlifePage struct {
	Base      []map[string]any
	Options   []map[string]any
	MaxPageNo int
}

type FinlifeClient struct {
	apiKey     string
	httpClient *http.Client
	budget     *apibudget.Manager
}

func NewFinlifeClient(apiKey string, hc *http.Client, budget *apibudget.Manager) *FinlifeClient {
	return &FinlifeClient{apiKey: strings.TrimSpace(apiKey), httpClient: hc, budget: budget}
}

func (c *FinlifeClient) Enabled() bool { return c != nil && c.apiKey != "" }

// Page fetches one page of a service ("companySearch" or a FinlifeServices value) for one 권역.
func (c *FinlifeClient) Page(ctx context.Context, service, group string, page int) (FinlifePage, error) {
	if !c.Enabled() {
		return FinlifePage{}, fmt.Errorf("finlife: FSS_FINLIFE_API_KEY not configured")
	}
	q := url.Values{"auth": {c.apiKey}, "topFinGrpNo": {group}, "pageNo": {strconv.Itoa(page)}}
	var payload struct {
		Result struct {
			ErrCd     string           `json:"err_cd"`
			ErrMsg    string           `json:"err_msg"`
			MaxPageNo any              `json:"max_page_no"`
			BaseList  []map[string]any `json:"baseList"`
			OptionLst []map[string]any `json:"optionList"`
		} `json:"result"`
	}
	if err := getJSON(ctx, c.httpClient, c.budget, apibudget.ProviderFinlife, finlifeBaseURL+service+".json?"+q.Encode(), &payload); err != nil {
		// 전송 오류(*url.Error)는 URL을 통째로 싣는다 — 키가 배치 원장·로그로 새지 않게 지운다.
		return FinlifePage{}, fmt.Errorf("finlife %s %s: %s", service, group, strings.ReplaceAll(err.Error(), c.apiKey, "REDACTED"))
	}
	r := payload.Result
	if r.ErrCd != "000" {
		// 키가 에러 메시지에 실리지 않게 코드·메시지만.
		return FinlifePage{}, fmt.Errorf("finlife %s %s: err_cd=%s %s", service, group, r.ErrCd, r.ErrMsg)
	}
	return FinlifePage{Base: r.BaseList, Options: r.OptionLst, MaxPageNo: anyInt(r.MaxPageNo)}, nil
}

// anyInt reads a JSON number that may arrive as a string ("3") or a number (3).
func anyInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(x))
		return n
	}
	return 0
}

// FinStr reads a string field from a raw API row ("" when absent or not a string).
func FinStr(row map[string]any, key string) string {
	s, _ := row[key].(string)
	return strings.TrimSpace(s)
}
