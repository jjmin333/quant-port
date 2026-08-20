// Package fnguide reads 국내 실적발표 일정을 FnGuide Company Guide에서 가져온다.
//
// # 이 패키지가 따로 있는 이유
//
// FnGuide는 이 데이터에 대한 **공개 API를 제공하지 않는다.** 여기서 쓰는 것은
// Company Guide 캘린더 화면(`/Calendar/CompCalendar`)이 스스로 호출하는 내부
// JSON 엔드포인트다. 그래서 두 가지가 참이다 — ① 예고 없이 바뀔 수 있다,
// ② FnGuide 이용약관과의 관계를 프로젝트가 알고 감수한 선택이다
// (`FNGUIDE_ENABLED=false`로 즉시 끌 수 있게 해 두었다).
//
// 접근 지점을 이 패키지 하나로 가둔 것은 그 때문이다. 정식 라이선스(FnSpace
// DataMart)나 다른 소스로 갈아탈 때 고칠 곳이 한 군데여야 한다.
//
// # 왜 이 소스인가 — DART로는 안 되는 것
//
// DART에는 실적발표 **예정** API가 없고, 컨센서스는 애초에 공시 대상이 아니다.
// 반면 이 엔드포인트 하나가 **한 달치 시장 전체**를 준다(2026-08 실측 190건):
// 종목코드·발표일·회계기간·추정치·발표치. 종목당 호출이 아니라 월당 1호출이라
// 부하도 우리 쪽 한도도 작다.
//
// # 실측으로 확인한 응답 규칙 (2026-08)
//
//   - 금액 단위는 **억원**. 삼성전자 2025 결산 매출 3,336,059 ↔ DART 확정치
//     333,605,938,000,000원(= 3,336,059.38억)으로 6자리까지 일치했다.
//   - `_E`는 추정치(컨센서스), `_P`는 발표치. 둘 다 **분기 단독** 값이다(누적 아님).
//   - `FREQ`는 `1Q`·`2Q`·`3Q`·`4Q`와 **`D`(연간 결산)**. 2월 발표분 480건이 `D`라서
//     분기 코드만 받으면 결산 시즌을 통째로 놓친다.
//   - `_G`는 YoY가 아니라 **QoQ**다. 삼성 3Q25 매출 86.1조·G=15.4로 역산한 Q2가
//     74.6조로 실제와 맞았고, 영업이익도 G=185로 역산한 Q4 2025가 20.1조로 맞았다.
//   - `CMP_NM`은 항상 `(연결)` 또는 `(개별)`로 끝나고, 정정 회차는 `[정정]`으로
//     시작한다. 정정은 원 발표와 **별 행으로 공존**한다(삼성 2Q: 07/07 + 07/30).
//   - **미래 달은 빈 배열이다.** 국내에는 실적발표 예정일을 주는 소스가 없다 —
//     이 패키지도 그 문제를 풀지 못한다(예정일 추정은 서비스 계층의 몫).
//   - 이력은 약 2년 롤링. 202408은 466건, 202308은 0건이었다.
package fnguide

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

// KRWUnit — FnGuide 금액 필드의 단위(억원)를 원으로 바꾸는 배수.
//
// 저장 단계에서 곱해 **기준통화 원본 단위**로 눕힌다. 억원을 그대로 저장하면
// 화면마다 "이 값은 억인가 원인가"를 다시 판단해야 하고, 미국 종목은 달러
// 원본이라 두 시장이 서로 다른 규칙을 갖게 된다.
const KRWUnit = 1e8

const defaultBaseURL = "https://wcomp.fnguide.com"

// 캘린더 화면이 보내는 것과 같은 질의값. grp_cd 1=잠정실적 2=이벤트 3=IPO 4=IR.
// 우리가 쓰는 것은 1뿐이지만, 화면과 다른 조합을 보내면 서버가 다르게 답할 수
// 있으므로 관측된 그대로 보낸다.
const (
	queryCapChgTypes = "0,1,2,3,4,5,6,7,11,12,13,14,15,16,17"
	queryGroupCodes  = "1,2,3,4"
)

// Config gates the client. Enabled=false면 호출 자체를 하지 않는다 — 약관 리스크를
// 코드 배포 없이 되돌릴 수 있어야 한다는 요구에서 나온 스위치다.
type Config struct {
	Enabled bool
	BaseURL string
}

func (c Config) baseURL() string {
	if strings.TrimSpace(c.BaseURL) == "" {
		return defaultBaseURL
	}
	return strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
}

type Client struct {
	cfg    Config
	http   *http.Client
	budget *apibudget.Manager
}

func New(cfg Config, budget *apibudget.Manager) *Client {
	return &Client{
		cfg:    cfg,
		http:   &http.Client{Timeout: 20 * time.Second},
		budget: budget,
	}
}

func (c *Client) Enabled() bool { return c != nil && c.cfg.Enabled }

// MinInterval is the spacing a multi-month caller must keep between fetches.
//
// 백필은 여러 달을 잇달아 받는데, 한도 관리자는 "지금 부를 수 있나"만 답한다.
// 그래서 이 값을 노출해 **호출자가 스스로 쉬게** 한다 — 안 그러면 첫 달만
// 성공하고 나머지가 전부 min-interval 오류로 죽는다(실측에서 15개월 중 14개월).
func (c *Client) MinInterval() time.Duration {
	if c == nil || c.budget == nil {
		return 0
	}
	return c.budget.MinInterval(apibudget.ProviderFnGuide)
}

// Preliminary is one 잠정실적발표 row, already normalized: 금액은 원 단위,
// 이름·정정 여부·연결/개별이 분리되어 있고, 회계기간은 (연도, 기간코드)다.
type Preliminary struct {
	Symbol string
	Name   string
	// 정정 회차인가. 원 발표 행과 같은 (종목·기간)을 가리키므로 저장 시 병합된다.
	Revised bool
	// "연결" | "개별"
	Basis string

	AnnouncedAt time.Time
	FiscalYear  int
	// "1Q".."4Q" | "FY"
	FiscalPeriod string
	PeriodEnd    time.Time

	RevenueEst   *float64
	RevenueAct   *float64
	OpIncomeEst  *float64
	OpIncomeAct  *float64
	NetIncomeEst *float64
	NetIncomeAct *float64

	RevenueQoQ   *float64
	OpIncomeQoQ  *float64
	NetIncomeQoQ *float64
}

// rawPreliminary mirrors the wire shape.
//
// 숫자 자리의 값이 **세 가지 형태**로 온다. ① JSON number(`27.73`) ②
// 숫자 문자열(`"56.2"`) ③ **한국어 라벨**(`"적전"`=적자전환, `"흑전"`, `"적지"`).
// ③은 증감률을 퍼센트로 말할 수 없을 때 FnGuide가 대신 쓰는 표기다.
//
// 그래서 전부 flexNumber로 받는다. 처음엔 `json.Number`를 썼는데, 그 타입은
// 디코드 시점에 숫자가 아닌 문자열을 거부한다 — **한 행의 "적전" 하나가 그 달
// 400행을 통째로 날렸다**(실측: 202507·202508이 통째로 실패). 소스가 우리
// 통제 밖일 때 한 행의 예외가 전체를 무효로 만드는 파서는 쓸 수 없다.
type rawPreliminary struct {
	TypeName string     `json:"TYPE_NM"`
	Date     string     `json:"DT"`
	Code     string     `json:"CMP_CD"`
	Name     string     `json:"CMP_NM"`
	YYMM     string     `json:"YYMM"`
	Freq     string     `json:"FREQ"`
	SalesE   flexNumber `json:"SALES_PRC_E"`
	OpE      flexNumber `json:"OPERATION_PROFIT_E"`
	NetE     flexNumber `json:"NET_PROFIT_E"`
	SalesP   flexNumber `json:"SALES_PRC_P"`
	OpP      flexNumber `json:"OPERATION_PROFIT_P"`
	NetP     flexNumber `json:"NET_PROFIT_P"`
	SalesG   flexNumber `json:"SALES_PRC_G"`
	OpG      flexNumber `json:"OPERATION_PROFIT_G"`
	NetG     flexNumber `json:"NET_PROFIT_G"`
}

// flexNumber accepts a JSON number, a numeric string, or a non-numeric string,
// never failing the decode.
//
// 숫자로 읽히지 않는 값은 **없는 값으로 둔다.** "적전"을 0으로 눕히면 적자 전환한
// 기업이 「증감률 0%」로 뜨고, 라벨을 따로 저장하려면 증감률마다 컬럼이 하나씩
// 더 필요하다 — 이 값은 화면의 보조 정보이고(주인공은 기대 vs 발표와 서프라이즈),
// 비어 있으면 화면이 이미 그 칸을 그리지 않는다.
type flexNumber struct {
	value *float64
}

func (n *flexNumber) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(string(data))
	if s == "" || s == "null" {
		return nil
	}
	// 문자열이면 따옴표를 벗긴다. 그 안이 숫자면 값이고, 아니면(한국어 라벨) 없는 값.
	if len(s) >= 2 && s[0] == '"' {
		var unquoted string
		if err := json.Unmarshal(data, &unquoted); err != nil {
			return nil
		}
		s = strings.TrimSpace(unquoted)
	}
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	n.value = &v
	return nil
}

// Float returns a copy of the parsed value, or nil when absent/non-numeric.
func (n flexNumber) Float() *float64 {
	if n.value == nil {
		return nil
	}
	v := *n.value
	return &v
}

// FetchPreliminary returns every 잠정실적발표 in one calendar month (yyyymm).
//
// 월 단위인 것은 우리 선택이 아니라 엔드포인트의 형태다. 대신 한 번에 시장
// 전체가 오므로, 종목이 늘어도 호출 수는 늘지 않는다.
func (c *Client) FetchPreliminary(ctx context.Context, year int, month time.Month) ([]Preliminary, error) {
	if !c.Enabled() {
		return nil, nil
	}
	if c.budget != nil {
		if err := c.budget.Check(ctx, apibudget.ProviderFnGuide); err != nil {
			return nil, err
		}
	}

	yyyymm := fmt.Sprintf("%04d%02d", year, int(month))
	url := fmt.Sprintf(
		"%s/Calendar/getListPreliminary?dt=%s&capchg_typ=%s&grp_cd=%s",
		c.cfg.baseURL(), yyyymm, queryCapChgTypes, queryGroupCodes,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// 이 셋이 없으면 서버가 화면 밖 호출로 보고 다르게 답한다. 쿠키·로그인은 불필요.
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; quant-port/1.0)")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", c.cfg.baseURL()+"/Calendar/CompCalendar")

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		if c.budget != nil {
			c.budget.BlockFor(ctx, apibudget.ProviderFnGuide, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		return nil, fmt.Errorf("fnguide preliminary %s: status 429", yyyymm)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fnguide preliminary %s: status %d", yyyymm, res.StatusCode)
	}

	body, err := readAll(res)
	if err != nil {
		return nil, fmt.Errorf("fnguide preliminary %s: %w", yyyymm, err)
	}
	rows, err := ParseDataset(body)
	if err != nil {
		return nil, fmt.Errorf("fnguide preliminary %s: %w", yyyymm, err)
	}
	if c.budget != nil {
		c.budget.Record(ctx, apibudget.ProviderFnGuide)
	}
	return rows, nil
}

// ParseDataset decodes a `{"dataset":[…]}` payload into normalized rows.
//
// 파싱을 네트워크에서 떼어 놓은 이유는 시험 때문이다. 정정 접두·연결/개별·
// `FREQ=D`·억원 배수 — 이 네 규칙이 기능의 정확성을 통째로 좌우하는데, 넷 다
// 실제 응답을 보고 알아낸 것이라 픽스처로 못 박아 두지 않으면 조용히 어긋난다.
func ParseDataset(body []byte) ([]Preliminary, error) {
	var payload struct {
		Dataset []rawPreliminary `json:"dataset"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := make([]Preliminary, 0, len(payload.Dataset))
	for _, row := range payload.Dataset {
		p, ok := parseRow(row)
		if !ok {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func readAll(res *http.Response) ([]byte, error) {
	// 응답이 시즌 달에도 100KB대라 통째로 읽는다. 상한을 둔 것은 엔드포인트가
	// 바뀌어 HTML 오류 페이지를 돌려줄 때 메모리를 먹지 않게 하기 위한 것이다.
	return io.ReadAll(io.LimitReader(res.Body, 16<<20))
}

func parseRow(row rawPreliminary) (Preliminary, bool) {
	code := strings.TrimSpace(row.Code)
	if code == "" {
		return Preliminary{}, false
	}
	announced, err := time.ParseInLocation("2006/01/02", strings.TrimSpace(row.Date), time.UTC)
	if err != nil {
		return Preliminary{}, false
	}
	year, period, ok := parseFiscal(row.YYMM, row.Freq)
	if !ok {
		return Preliminary{}, false
	}

	name, basis, revised := parseName(row.Name)
	p := Preliminary{
		Symbol:       code,
		Name:         name,
		Revised:      revised,
		Basis:        basis,
		AnnouncedAt:  announced,
		FiscalYear:   year,
		FiscalPeriod: period,
		PeriodEnd:    parsePeriodEnd(row.YYMM),

		RevenueEst:   scaledKRW(row.SalesE),
		RevenueAct:   scaledKRW(row.SalesP),
		OpIncomeEst:  scaledKRW(row.OpE),
		OpIncomeAct:  scaledKRW(row.OpP),
		NetIncomeEst: scaledKRW(row.NetE),
		NetIncomeAct: scaledKRW(row.NetP),

		RevenueQoQ:   number(row.SalesG),
		OpIncomeQoQ:  number(row.OpG),
		NetIncomeQoQ: number(row.NetG),
	}
	return p, true
}

// parseName splits "[정정]삼성전자(연결)" into name / basis / revised.
//
// 접두를 떼지 않으면 종목명이 「[정정]삼성전자」로 저장되고, 접미를 떼지 않으면
// 같은 회사가 「삼성전자(연결)」과 「삼성전자(개별)」 두 종목처럼 보인다.
func parseName(raw string) (name, basis string, revised bool) {
	name = strings.TrimSpace(raw)
	for {
		// 관측된 것은 `[정정]` 하나뿐이지만, 다른 대괄호 표지가 붙어도
		// 이름을 오염시키지 않게 앞쪽 대괄호 묶음을 전부 벗긴다.
		if !strings.HasPrefix(name, "[") {
			break
		}
		close := strings.Index(name, "]")
		if close < 0 {
			break
		}
		if strings.Contains(name[:close], "정정") {
			revised = true
		}
		name = strings.TrimSpace(name[close+1:])
	}
	if strings.HasSuffix(name, ")") {
		if open := strings.LastIndex(name, "("); open >= 0 {
			inner := strings.TrimSpace(name[open+1 : len(name)-1])
			if inner == "연결" || inner == "개별" {
				basis = inner
				name = strings.TrimSpace(name[:open])
			}
		}
	}
	return name, basis, revised
}

// parseFiscal maps ("2026/06", "2Q") → (2026, "2Q") and ("2025/12", "D") →
// (2025, "FY").
//
// 연도는 YYMM에서 읽는다 — 발표일 연도가 아니다. 12월 결산은 이듬해 1~2월에
// 발표되므로, 발표일에서 연도를 뽑으면 2025 결산이 2026년 것으로 들어간다.
func parseFiscal(yymm, freq string) (int, string, bool) {
	parts := strings.Split(strings.TrimSpace(yymm), "/")
	if len(parts) < 1 {
		return 0, "", false
	}
	year, err := strconv.Atoi(parts[0])
	if err != nil || year < 1900 {
		return 0, "", false
	}
	switch f := strings.ToUpper(strings.TrimSpace(freq)); f {
	case "1Q", "2Q", "3Q", "4Q":
		return year, f, true
	case "D":
		// D = 결산(연간). FY로 눕혀 미국 쪽 표기와 한 어휘를 쓴다.
		return year, "FY", true
	default:
		return 0, "", false
	}
}

// parsePeriodEnd turns "2026/06" into 2026-06-30 (해당 월의 마지막 날).
func parsePeriodEnd(yymm string) time.Time {
	parts := strings.Split(strings.TrimSpace(yymm), "/")
	if len(parts) != 2 {
		return time.Time{}
	}
	year, err1 := strconv.Atoi(parts[0])
	month, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || month < 1 || month > 12 {
		return time.Time{}
	}
	// 다음 달 1일에서 하루를 빼면 윤년·월말 길이를 따로 다룰 필요가 없다.
	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	return first.AddDate(0, 1, -1)
}

// scaledKRW converts an 억원 figure to 원.
func scaledKRW(n flexNumber) *float64 {
	v := n.Float()
	if v == nil {
		return nil
	}
	scaled := *v * KRWUnit
	return &scaled
}

// number passes a ratio through unscaled — 증감률에 억원 배수를 곱하면 안 된다.
func number(n flexNumber) *float64 { return n.Float() }
