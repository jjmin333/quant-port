package marketdata

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

/*
부동산 공공데이터 호출부 (2026-10-01, docs/ASSET_EXPANSION_RESEARCH.md §4). 배치(worker-filings)만 부른다.

	국토부 실거래가   apis.data.go.kr/1613000/RTMSDataSvc{종류}/getRTMSDataSvc{종류}   XML, 키 DATA_GO_KR_API_KEY
	                  API마다 공공데이터포털 「활용신청」 — 안 한 API는 SERVICE_KEY_IS_NOT_REGISTERED_ERROR.
	한국부동산원 R-ONE www.reb.or.kr/r-one/openapi/SttsApiTblData.do                     JSON, 키 REB_RONE_API_KEY
	행안부 법정동코드  apis.data.go.kr/1741000/StanReginCd/getStanReginCdList              JSON, 키 DATA_GO_KR_API_KEY

이용 조건: 국토부·행안부는 공공데이터포털 「이용허락범위 제한 없음」, R-ONE은 약관상 출처표시 조건 자유 이용.
금액은 원천 그대로 만원 단위다.
*/

// RTMSKind — 실거래 한 종류(매매 또는 전월세).
type RTMSKind struct {
	Kind     string // apt · rh · sh · offi · land · nrg · indu · silv
	DealType string // sale · rent
	Service  string // RTMSDataSvc 뒤에 붙는 이름
}

// RTMSKinds — 적재하는 12종. 순서는 적재 순서(아파트 먼저).
var RTMSKinds = []RTMSKind{
	{"apt", "sale", "AptTradeDev"},
	{"apt", "rent", "AptRent"},
	{"offi", "sale", "OffiTrade"},
	{"offi", "rent", "OffiRent"},
	{"rh", "sale", "RHTrade"},
	{"rh", "rent", "RHRent"},
	{"sh", "sale", "SHTrade"},
	{"sh", "rent", "SHRent"},
	{"silv", "sale", "SilvTrade"},
	{"land", "sale", "LandTrade"},
	{"nrg", "sale", "NrgTrade"},
	{"indu", "sale", "InduTrade"},
}

// RTMSRow — 실거래 한 건을 우리 칸으로 옮긴 것. 금액은 만원.
type RTMSRow struct {
	DealDate                time.Time
	UmdNm, Jibun            string
	ComplexKey, ComplexName string
	Category                string
	Area                    float64
	LandArea                *float64
	Floor, BuildYear        *int
	Price, Deposit, Monthly *int64
	ContractType            string
	Direct, Cancelled       bool
	// Road — 도로명 주소(「동대구로 345」). 아파트 매매(AptTradeDev)만 준다 — 단지 검색이 도로명으로도 찾게(2026-10-09).
	Road string
}

type PublicDataClient struct {
	dataKey    string // 공공데이터포털
	roneKey    string // 한국부동산원
	ecosKey    string // 한국은행
	httpClient *http.Client
	budget     *apibudget.Manager
}

func NewPublicDataClient(dataKey, roneKey, ecosKey string, hc *http.Client, budget *apibudget.Manager) *PublicDataClient {
	return &PublicDataClient{dataKey: strings.TrimSpace(dataKey), roneKey: strings.TrimSpace(roneKey),
		ecosKey: strings.TrimSpace(ecosKey), httpClient: hc, budget: budget}
}

func (c *PublicDataClient) DataEnabled() bool { return c != nil && c.dataKey != "" }
func (c *PublicDataClient) RoneEnabled() bool { return c != nil && c.roneKey != "" }
func (c *PublicDataClient) EcosEnabled() bool { return c != nil && c.ecosKey != "" }

// redact removes every key from an error message — 전송 오류(*url.Error)는 URL을 통째로 싣는다.
func (c *PublicDataClient) redact(err error) error {
	if err == nil {
		return nil
	}
	s := err.Error()
	for _, k := range []string{c.dataKey, c.roneKey, c.ecosKey, url.QueryEscape(c.dataKey)} {
		if k != "" {
			s = strings.ReplaceAll(s, k, "REDACTED")
		}
	}
	return redactedErr{msg: s, err: err} // errors.Is(…, apibudget.ErrExhausted)가 살아 있어야 잡이 멈춘다
}

type redactedErr struct {
	msg string
	err error
}

func (e redactedErr) Error() string { return e.msg }
func (e redactedErr) Unwrap() error { return e.err }

// RTMSPage fetches one page (최대 1000건) of one kind/region/month. total은 전체 건수.
func (c *PublicDataClient) RTMSPage(ctx context.Context, k RTMSKind, lawdCd, ym string, page int) ([]RTMSRow, int, error) {
	if !c.DataEnabled() {
		return nil, 0, fmt.Errorf("molit: DATA_GO_KR_API_KEY not configured")
	}
	u := fmt.Sprintf("https://apis.data.go.kr/1613000/RTMSDataSvc%s/getRTMSDataSvc%s?serviceKey=%s&LAWD_CD=%s&DEAL_YMD=%s&pageNo=%d&numOfRows=1000",
		k.Service, k.Service, url.QueryEscape(c.dataKey), lawdCd, ym, page)
	body, err := getRaw(ctx, c.httpClient, c.budget, apibudget.ProviderMolitRTMS, u, "application/xml", "")
	if err != nil {
		return nil, 0, c.redact(fmt.Errorf("molit %s %s %s: %w", k.Service, lawdCd, ym, err))
	}
	items, header, err := parseRTMSXML(body)
	if err != nil {
		return nil, 0, fmt.Errorf("molit %s %s %s: %w", k.Service, lawdCd, ym, err)
	}
	if header.authMsg != "" {
		return nil, 0, fmt.Errorf("molit %s: %s", k.Service, header.authMsg)
	}
	if header.code != "" && header.code != "000" && header.code != "00" {
		return nil, 0, fmt.Errorf("molit %s %s %s: result %s %s", k.Service, lawdCd, ym, header.code, header.msg)
	}
	out := make([]RTMSRow, 0, len(items))
	for _, it := range items {
		if r, ok := rtmsRow(k, it); ok {
			out = append(out, r)
		}
	}
	return out, header.total, nil
}

type rtmsHeader struct {
	code, msg, authMsg string
	total              int
}

// parseRTMSXML reads <item> children into flat maps. 종류마다 칸이 달라 구조체 대신 map이다.
func parseRTMSXML(body []byte) ([]map[string]string, rtmsHeader, error) {
	dec := xml.NewDecoder(bytes.NewReader(body))
	var (
		h     rtmsHeader
		items []map[string]string
		cur   map[string]string
		field string
	)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, h, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "item" {
				cur = map[string]string{}
			}
			field = t.Name.Local
		case xml.EndElement:
			if t.Name.Local == "item" && cur != nil {
				items = append(items, cur)
				cur = nil
			}
			field = ""
		case xml.CharData:
			v := strings.TrimSpace(string(t))
			if v == "" || field == "" {
				continue
			}
			switch {
			case cur != nil:
				cur[field] = v
			case field == "resultCode":
				h.code = v
			case field == "resultMsg":
				h.msg = v
			case field == "returnAuthMsg":
				h.authMsg = v
			case field == "totalCount":
				h.total, _ = strconv.Atoi(v)
			}
		}
	}
	return items, h, nil
}

func rtmsRow(k RTMSKind, it map[string]string) (RTMSRow, bool) {
	y, _ := strconv.Atoi(it["dealYear"])
	m, _ := strconv.Atoi(it["dealMonth"])
	d, _ := strconv.Atoi(it["dealDay"])
	if y == 0 || m == 0 || d == 0 {
		return RTMSRow{}, false
	}
	r := RTMSRow{
		DealDate:     time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC),
		UmdNm:        it["umdNm"],
		Jibun:        it["jibun"],
		ContractType: it["contractType"],
		Direct:       it["dealingGbn"] == "직거래",
		Cancelled:    it["cdealType"] != "",
		Floor:        optInt(it["floor"]),
		BuildYear:    optInt(it["buildYear"]),
		Price:        optMoney(it["dealAmount"]),
		Deposit:      optMoney(it["deposit"]),
		Monthly:      optMoney(it["monthlyRent"]),
	}
	name := ""
	switch k.Kind {
	case "apt", "silv":
		name = it["aptNm"]
		r.Area = num(it["excluUseAr"])
		r.Category = it["ownershipGbn"]
	case "rh":
		name = it["mhouseNm"]
		r.Area = num(it["excluUseAr"])
		r.LandArea = optFloat(it["landAr"])
		r.Category = it["houseType"]
	case "offi":
		name = it["offiNm"]
		r.Area = num(it["excluUseAr"])
	case "sh":
		r.Area = num(it["totalFloorAr"])
		r.LandArea = optFloat(it["plottageAr"])
		r.Category = it["houseType"]
	case "land":
		r.Area = num(it["dealArea"])
		r.Category = strings.TrimSpace(it["jimok"] + " " + it["landUse"])
	case "nrg", "indu":
		r.Area = num(it["buildingAr"])
		r.LandArea = optFloat(it["plottageAr"])
		r.Category = strings.TrimSpace(it["buildingUse"] + " " + it["landUse"])
	}
	r.ComplexName = name
	r.Road = roadAddress(it["roadNm"], it["roadNmBonbun"], it["roadNmBubun"])
	// 아파트는 단지 일련번호(aptSeq)가 있다. 분양권·전월세 일부는 없어 이름으로 묶는다.
	if seq := it["aptSeq"]; seq != "" {
		r.ComplexKey = seq
	} else if name != "" {
		r.ComplexKey = it["sggCd"] + "|" + r.UmdNm + "|" + it["jibun"] + "|" + name
	}
	return r, true
}

func num(s string) float64 {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", ""), 64)
	return f
}

func optFloat(s string) *float64 {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

func optInt(s string) *int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}

func optMoney(s string) *int64 {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

// ── 한국부동산원 R-ONE ─────────────────────────────────────────────────────────

// RoneRow — 통계표 한 값.
type RoneRow struct {
	ClsID              int
	ClsName, ClsFull   string
	ItmID              int
	ItmName            string
	Period, PeriodDesc string
	Value              float64
}

// RoneData fetches one page (최대 1,000행). q는 CLS_ID·WRTTIME_IDTFR_ID 같은 추가 조건.
func (c *PublicDataClient) RoneData(ctx context.Context, statblID, cycle string, q url.Values, page int) ([]RoneRow, int, error) {
	if !c.RoneEnabled() {
		return nil, 0, fmt.Errorf("rone: REB_RONE_API_KEY not configured")
	}
	if q == nil {
		q = url.Values{}
	}
	q.Set("KEY", c.roneKey)
	q.Set("Type", "json")
	q.Set("STATBL_ID", statblID)
	q.Set("DTACYCLE_CD", cycle)
	q.Set("pIndex", strconv.Itoa(page))
	q.Set("pSize", "1000")
	var payload struct {
		Data []struct {
			Head []struct {
				Total  *int `json:"list_total_count"`
				Result *struct {
					Code string `json:"CODE"`
				} `json:"RESULT"`
			} `json:"head"`
			Row []struct {
				ClsID   int      `json:"CLS_ID"`
				ClsNm   string   `json:"CLS_NM"`
				ClsFull string   `json:"CLS_FULLNM"`
				ItmID   int      `json:"ITM_ID"`
				ItmNm   string   `json:"ITM_NM"`
				Period  string   `json:"WRTTIME_IDTFR_ID"`
				Desc    string   `json:"WRTTIME_DESC"`
				Val     *float64 `json:"DTA_VAL"`
			} `json:"row"`
		} `json:"SttsApiTblData"`
		Result *struct {
			Code    string `json:"CODE"`
			Message string `json:"MESSAGE"`
		} `json:"RESULT"`
	}
	u := "https://www.reb.or.kr/r-one/openapi/SttsApiTblData.do?" + q.Encode()
	if err := getJSON(ctx, c.httpClient, c.budget, apibudget.ProviderRone, u, &payload); err != nil {
		return nil, 0, c.redact(fmt.Errorf("rone %s: %w", statblID, err))
	}
	if payload.Result != nil {
		// INFO-200 = 데이터 없음. 오류가 아니다.
		if payload.Result.Code == "INFO-200" {
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("rone %s: %s %s", statblID, payload.Result.Code, payload.Result.Message)
	}
	total := 0
	var out []RoneRow
	for _, blk := range payload.Data {
		for _, h := range blk.Head {
			if h.Total != nil {
				total = *h.Total
			}
		}
		for _, r := range blk.Row {
			if r.Val == nil {
				continue
			}
			out = append(out, RoneRow{ClsID: r.ClsID, ClsName: r.ClsNm, ClsFull: r.ClsFull, ItmID: r.ItmID,
				ItmName: r.ItmNm, Period: r.Period, PeriodDesc: r.Desc, Value: *r.Val})
		}
	}
	return out, total, nil
}

// ── 행안부 법정동코드 ───────────────────────────────────────────────────────────

// Region — 실거래 API가 받는 시군구(법정동 앞 5자리).
type Region struct {
	LawdCd, Sido, Name string
}

// Regions lists every 시군구 that has trades of its own. 하위 구가 있는 시(수원시 등)는 뺀다 —
// 실거래 API는 구 코드(41111)로만 답한다.
func (c *PublicDataClient) Regions(ctx context.Context) ([]Region, error) {
	if !c.DataEnabled() {
		return nil, fmt.Errorf("regions: DATA_GO_KR_API_KEY not configured")
	}
	type row struct {
		RegionCd string `json:"region_cd"`
		SggCd    string `json:"sgg_cd"`
		UmdCd    string `json:"umd_cd"`
		RiCd     string `json:"ri_cd"`
		Name     string `json:"locatadd_nm"`
	}
	var all []row
	for page := 1; page <= 30; page++ {
		// 버킷 간격(1초)보다 조금 길게 — Check는 기다려 주지 않는다(첫 실행에서 2쪽부터 거절됐다).
		if page > 1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(1100 * time.Millisecond):
			}
		}
		var payload struct {
			Std []struct {
				Head []struct {
					Total  *int `json:"totalCount"`
					Result *struct {
						Code string `json:"resultCode"`
						Msg  string `json:"resultMsg"`
					} `json:"RESULT"`
				} `json:"head"`
				Row []row `json:"row"`
			} `json:"StanReginCd"`
			Fault *struct {
				Header struct {
					Msg string `json:"returnAuthMsg"`
				} `json:"cmmMsgHeader"`
			} `json:"OpenAPI_ServiceResponse"`
		}
		u := fmt.Sprintf("https://apis.data.go.kr/1741000/StanReginCd/getStanReginCdList?serviceKey=%s&pageNo=%d&numOfRows=1000&type=json&flag=Y",
			url.QueryEscape(c.dataKey), page)
		if err := getJSON(ctx, c.httpClient, c.budget, apibudget.ProviderMoisRegion, u, &payload); err != nil {
			return nil, c.redact(fmt.Errorf("regions: %w", err))
		}
		if payload.Fault != nil {
			return nil, fmt.Errorf("regions: %s", payload.Fault.Header.Msg)
		}
		n := 0
		for _, b := range payload.Std {
			all = append(all, b.Row...)
			n += len(b.Row)
		}
		if n < 1000 {
			break
		}
	}
	raw := make([]rawRegion, 0, len(all))
	for _, r := range all {
		raw = append(raw, rawRegion{r.RegionCd, r.SggCd, r.UmdCd, r.RiCd, r.Name})
	}
	return pickSigungu(raw), nil
}

type rawRegion struct{ code, sgg, umd, ri, name string }

// pickSigungu keeps 시군구 rows (읍면동 000 · 리 00 · 시군구 ≠ 000) and drops a 시 whose 구 exist.
func pickSigungu(rows []rawRegion) []Region {
	seen := map[string]Region{}
	for _, r := range rows {
		if len(r.code) < 5 || r.sgg == "000" || r.sgg == "" || strings.TrimLeft(r.umd, "0") != "" || strings.TrimLeft(r.ri, "0") != "" {
			continue
		}
		parts := strings.Fields(r.name)
		if len(parts) < 2 {
			continue
		}
		seen[r.code[:5]] = Region{LawdCd: r.code[:5], Sido: parts[0], Name: strings.Join(parts[1:], " ")}
	}
	out := make([]Region, 0, len(seen))
	for code, reg := range seen {
		// 41110(수원시)과 41111(장안구)처럼 앞 4자리가 같은 하위 구가 있으면 시는 뺀다.
		if code[4] == '0' {
			hasChild := false
			for other := range seen {
				if other != code && other[:4] == code[:4] {
					hasChild = true
					break
				}
			}
			if hasChild {
				continue
			}
		}
		out = append(out, reg)
	}
	return out
}

// SeoulRegions — 법정동코드 API를 쓸 수 없을 때(활용신청 전)의 최소 목록.
var SeoulRegions = []Region{
	{"11110", "서울특별시", "종로구"}, {"11140", "서울특별시", "중구"}, {"11170", "서울특별시", "용산구"},
	{"11200", "서울특별시", "성동구"}, {"11215", "서울특별시", "광진구"}, {"11230", "서울특별시", "동대문구"},
	{"11260", "서울특별시", "중랑구"}, {"11290", "서울특별시", "성북구"}, {"11305", "서울특별시", "강북구"},
	{"11320", "서울특별시", "도봉구"}, {"11350", "서울특별시", "노원구"}, {"11380", "서울특별시", "은평구"},
	{"11410", "서울특별시", "서대문구"}, {"11440", "서울특별시", "마포구"}, {"11470", "서울특별시", "양천구"},
	{"11500", "서울특별시", "강서구"}, {"11530", "서울특별시", "구로구"}, {"11545", "서울특별시", "금천구"},
	{"11560", "서울특별시", "영등포구"}, {"11590", "서울특별시", "동작구"}, {"11620", "서울특별시", "관악구"},
	{"11650", "서울특별시", "서초구"}, {"11680", "서울특별시", "강남구"}, {"11710", "서울특별시", "송파구"},
	{"11740", "서울특별시", "강동구"},
}

// roadAddress — 「동대구로」 + 「00345」 + 「00000」 → 「동대구로 345」(부번이 있으면 「345-2」).
func roadAddress(name, bon, bu string) string {
	name = strings.TrimSpace(name)
	b, _ := strconv.Atoi(strings.TrimSpace(bon))
	if name == "" || b == 0 {
		return ""
	}
	out := name + " " + strconv.Itoa(b)
	if s, _ := strconv.Atoi(strings.TrimSpace(bu)); s > 0 {
		out += "-" + strconv.Itoa(s)
	}
	return out
}
