package model

import "strings"

/*
종목 스크리너 — 사용자가 조건을 짜서 종목을 찾는다 (V2 요구 4).

# 새 표도 새 수집도 없다

조건에 쓰는 값이 이미 `stock_scores` 한 줄 안에 전부 들어 있다:

	score / grade          컬럼
	breakdown              축 5개 (기술·수익성·성장·가치·심리수급)
	inputs.raw             ROE·부채비율·유동비율·마진·성장률·PER·PBR·PSR
	inputs.technical       RSI·이평·52주 신고가 대비·5일/20일 수익률
	inputs.marketCap       시가총액

그래서 **거르는 일은 표 하나에서 끝난다.** 종가·등락률은 결과로 남은 200종목에만
붙인다 — 거르는 단계에 조인을 얹으면 하루치 17,919행이 전부 조인을 탄다.

# 화이트리스트가 이 파일의 존재 이유다

필드 이름이 SQL의 JSON 경로가 된다. 사용자가 보낸 문자열을 그대로 경로에 넣으면
주입이고, 넣지 않더라도 **인덱스를 못 타는 조건**을 사용자가 만들 수 있게 된다.
그래서 걸 수 있는 필드는 여기 적힌 것이 전부이고, 경로는 코드에만 있다.

# 커버리지가 이 기능의 정직성이다

실측(2026-09-17, 17,919종목):

	RSI·신고가 대비·이평   16,247 (91%)      시가총액   4,268 (24%)
	ROE·매출 성장          6,216 (35%)       PBR       2,607 (15%)
	부채비율               5,790 (32%)       PER       1,599 (9%)
	                                         심리·수급   571 (3%)

「PER 10 이하」를 걸면 91%의 종목이 **조건을 틀려서가 아니라 잴 수 없어서** 사라진다.
그 둘을 구분해 말하지 않으면 사용자는 「시장에 그런 종목이 없다」로 읽는다. 그래서
모든 응답이 `ScreenFunnel`을 들고 나간다 — 전체 → 시장 → 잴 수 있음 → 조건 통과.
*/

// ScreenFieldGroup은 화면의 조건 묶음이다.
type ScreenFieldGroup string

const (
	ScreenGroupScore     ScreenFieldGroup = "score"
	ScreenGroupFinancial ScreenFieldGroup = "financial"
	ScreenGroupValuation ScreenFieldGroup = "valuation"
	ScreenGroupPrice     ScreenFieldGroup = "price"
	// ScreenGroupSignal — 일정·변화·수급 (2026-09-27 결정 C ③ 문장 검색 확장).
	ScreenGroupSignal ScreenFieldGroup = "signal"
)

var ScreenGroupLabels = map[ScreenFieldGroup]string{
	ScreenGroupScore:     "AI 점수",
	ScreenGroupFinancial: "재무",
	ScreenGroupValuation: "가치",
	ScreenGroupPrice:     "가격·기술",
	ScreenGroupSignal:    "일정·신호",
}

// ScreenGroupOrder는 화면에 쓰는 순서다 — 커버리지가 높은 묶음부터가 아니라
// 사용자가 생각하는 순서(점수 → 재무 → 가치 → 가격)를 따른다.
var ScreenGroupOrder = []ScreenFieldGroup{
	ScreenGroupScore, ScreenGroupFinancial, ScreenGroupValuation, ScreenGroupPrice, ScreenGroupSignal,
}

/*
ScreenField — 걸 수 있는 조건 하나의 정의.

`Expr`은 **코드에만 있는** SQL 조각이다. 사용자가 보내는 것은 `ID`뿐이고, 그 ID가
이 표에 없으면 요청 자체가 거절된다.
*/
type ScreenField struct {
	ID    string           `json:"id"`
	Label string           `json:"label"`
	Group ScreenFieldGroup `json:"group"`
	// Unit은 화면이 값 뒤에 붙일 글자 — "%" | "배" | "점" | "원" | "".
	Unit string `json:"unit"`
	/*
		HigherIsBetter는 **원값**의 방향이다. 부채비율은 낮을수록 좋아서 false다.
		화면이 기본 연산자를 고를 때 쓴다 — ROE는 「이상」, 부채비율은 「이하」.
	*/
	HigherIsBetter bool `json:"higherIsBetter"`
	// Decimals는 표시 자릿수. 점수는 0, 비율은 1, 배수는 2다.
	Decimals int `json:"decimals"`
	// Expr은 WHERE/SELECT에 쓰는 SQL 조각. **응답에 나가지 않는다.**
	Expr string `json:"-"`
	// Scale은 표시용 나눗셈 — 시가총액을 조(兆) 단위로 받기 위한 것. 0이면 안 나눈다.
	Scale float64 `json:"scale,omitempty"`
	// Help — 조건 칸의 ⓘ 설명(처음 쓰는 사람이 뜻과 좋은 방향을 알게, 2026-09-28 요청 5).
	Help string `json:"help,omitempty"`
	// Aliases — 글 태그 `@종목:지표`에서 이 지표를 부르는 이름들(Label은 따로 넣지 않아도 된다).
	Aliases []string `json:"aliases,omitempty"`
}

/*
screenNum — JSON 값을 숫자로 꺼내는 공통 조각.

`CAST(... AS DECIMAL)`을 쓰는 이유는 `->`가 돌려주는 JSON 값끼리는 문자열 비교가
섞이기 때문이다 — `'9' > '10'`이 참이 되는 그 문제다.
*/
func screenNum(path string) string {
	return "CAST(JSON_EXTRACT(sc.inputs, '" + path + "') AS DECIMAL(24,6))"
}

func screenAxis(axis string) string {
	return "CAST(JSON_EXTRACT(sc.breakdown, '$." + axis + "') AS DECIMAL(8,3))"
}

/*
ScreenFields — 걸 수 있는 조건 전부.

목록의 순서가 화면의 순서다. 커버리지가 낮은 필드(PER 9% · 심리·수급 3%)도 **뺀
것이 아니라 남겨 두고**, 옆에 비중을 적는다 (2026-09-18 사용자 결정 가). 쓸 수 있는
1,599종목의 PER은 진짜 PER이고, 숨기면 그것까지 사라진다.
*/
var ScreenFields = []ScreenField{
	// ── AI 점수 ────────────────────────────────────────────────────────────
	{ID: "score", Label: "종합 점수", Group: ScreenGroupScore, Unit: "점", HigherIsBetter: true, Expr: "sc.score"},
	{ID: "axis.technical", Label: "추세", Group: ScreenGroupScore, Unit: "점", HigherIsBetter: true, Expr: screenAxis("technical")},
	{ID: "axis.quality", Label: "수익성", Group: ScreenGroupScore, Unit: "점", HigherIsBetter: true, Expr: screenAxis("quality")},
	{ID: "axis.growth", Label: "성장", Group: ScreenGroupScore, Unit: "점", HigherIsBetter: true, Expr: screenAxis("growth")},
	{ID: "axis.valuation", Label: "가치", Group: ScreenGroupScore, Unit: "점", HigherIsBetter: true, Expr: screenAxis("valuation")},
	{ID: "axis.sentiment", Label: "심리·수급", Group: ScreenGroupScore, Unit: "점", HigherIsBetter: true, Expr: screenAxis("sentiment")},

	// ── 재무 ───────────────────────────────────────────────────────────────
	{ID: "roe", Label: "ROE", Group: ScreenGroupFinancial, Unit: "%", HigherIsBetter: true, Decimals: 1, Expr: screenNum("$.raw.roe")},
	{ID: "operatingMargin", Label: "영업이익률", Group: ScreenGroupFinancial, Unit: "%", HigherIsBetter: true, Decimals: 1, Expr: screenNum("$.raw.operatingMargin")},
	{ID: "netProfitMargin", Label: "순이익률", Group: ScreenGroupFinancial, Unit: "%", HigherIsBetter: true, Decimals: 1, Expr: screenNum("$.raw.netProfitMargin")},
	{ID: "revenueGrowthYoY", Label: "매출 성장", Group: ScreenGroupFinancial, Unit: "%", HigherIsBetter: true, Decimals: 1, Expr: screenNum("$.raw.revenueGrowthYoY")},
	{ID: "epsGrowthYoY", Label: "EPS 성장", Group: ScreenGroupFinancial, Unit: "%", HigherIsBetter: true, Decimals: 1, Expr: screenNum("$.raw.epsGrowthYoY")},
	// 부채비율·유동비율은 방향이 반대다 — 화면의 기본 연산자가 「이하」가 된다.
	{ID: "debtRatio", Label: "부채비율", Group: ScreenGroupFinancial, Unit: "%", HigherIsBetter: false, Decimals: 1, Expr: screenNum("$.raw.debtRatio")},
	{ID: "currentRatio", Label: "유동비율", Group: ScreenGroupFinancial, Unit: "배", HigherIsBetter: true, Decimals: 2, Expr: screenNum("$.raw.currentRatio")},

	// ── 가치 ───────────────────────────────────────────────────────────────
	{ID: "per", Label: "PER", Group: ScreenGroupValuation, Unit: "배", HigherIsBetter: false, Decimals: 2, Expr: screenNum("$.raw.per")},
	{ID: "pbr", Label: "PBR", Group: ScreenGroupValuation, Unit: "배", HigherIsBetter: false, Decimals: 2, Expr: screenNum("$.raw.pbr")},
	{ID: "psr", Label: "PSR", Group: ScreenGroupValuation, Unit: "배", HigherIsBetter: false, Decimals: 2, Expr: screenNum("$.raw.psr")},

	// ── 가격·기술 ──────────────────────────────────────────────────────────
	{ID: "marketCap", Label: "시가총액", Group: ScreenGroupPrice, Unit: "조원", HigherIsBetter: true, Decimals: 2,
		Expr: screenNum("$.marketCap"), Scale: 1e12},
	{ID: "close", Label: "주가", Group: ScreenGroupPrice, Unit: "원", HigherIsBetter: true, Expr: screenNum("$.close")},
	{ID: "rsi14", Label: "RSI", Group: ScreenGroupPrice, Unit: "", HigherIsBetter: true, Decimals: 1, Expr: screenNum("$.technical.rsi14")},
	{ID: "pctFromHigh52Week", Label: "52주 신고가 대비", Group: ScreenGroupPrice, Unit: "%", HigherIsBetter: true, Decimals: 1,
		Expr: screenNum("$.technical.pctFromHigh52Week")},
	{ID: "pctFromLow52Week", Label: "52주 신저가 대비", Group: ScreenGroupPrice, Unit: "%", HigherIsBetter: true, Decimals: 1,
		Expr: screenNum("$.technical.pctFromLow52Week")},
	{ID: "return5d", Label: "5일 수익률", Group: ScreenGroupPrice, Unit: "%", HigherIsBetter: true, Decimals: 2, Expr: screenNum("$.technical.return5d")},
	{ID: "return20d", Label: "20일 수익률", Group: ScreenGroupPrice, Unit: "%", HigherIsBetter: true, Decimals: 2, Expr: screenNum("$.technical.return20d")},
	{ID: "return60d", Label: "60일 수익률", Group: ScreenGroupPrice, Unit: "%", HigherIsBetter: true, Decimals: 2, Expr: screenNum("$.technical.return60d")},

	// ── 일정·신호 (2026-09-27) ────────────────────────────────────────────
	// 「실적 발표 앞둔」「점수 오른」「외국인이 사는」을 문장으로 찾게 한다. 앞 둘은 행마다 인덱스로
	// 한 번 찌르는 상관 부질의다(earnings_events(symbol, event_date) · stock_scores PK).
	{ID: "daysToEarnings", Label: "실적 발표까지(추정 포함)", Group: ScreenGroupSignal, Unit: "일", HigherIsBetter: false,
		Expr: screenDaysToEarningsExpr},
	{ID: "scoreChange", Label: "점수 변화(전 점수일)", Group: ScreenGroupSignal, Unit: "점", HigherIsBetter: true,
		Expr: "(sc.score - (SELECT p.score FROM stock_scores p WHERE p.symbol = sc.symbol AND p.trade_date < sc.trade_date ORDER BY p.trade_date DESC LIMIT 1))"},
	{ID: "flowNetPct", Label: "외국인·기관 순매수", Group: ScreenGroupSignal, Unit: "%", HigherIsBetter: true, Decimals: 2, Expr: screenNum("$.flowNetPct")},

	// ── 애널리스트 · 레딧 (2026-09-28, 배치 적재 표를 PK로 한 번 찌른다) ──────────────
	{ID: "analystRating", Label: "애널리스트 의견", Group: ScreenGroupSignal, Unit: "", HigherIsBetter: true, Decimals: 2,
		Expr: "(SELECT c.rating FROM stock_analyst_consensus c WHERE c.symbol = sc.symbol)"},
	{ID: "targetPrice", Label: "평균 목표가", Group: ScreenGroupSignal, Unit: "원", HigherIsBetter: true,
		Expr: "(SELECT c.target_mean FROM stock_analyst_consensus c WHERE c.symbol = sc.symbol AND c.target_currency = 'KRW')"},
	{ID: "targetUpside", Label: "목표가 여력", Group: ScreenGroupSignal, Unit: "%", HigherIsBetter: true, Decimals: 1,
		Expr: "(SELECT (c.target_mean / NULLIF(" + screenNum("$.close") + ", 0) - 1) * 100 FROM stock_analyst_consensus c" +
			" WHERE c.symbol = sc.symbol AND c.target_mean > 0 AND c.target_currency = 'KRW')"},
	{ID: "socialMentions", Label: "레딧 언급(24시간)", Group: ScreenGroupSignal, Unit: "회", HigherIsBetter: true,
		Expr: "(SELECT m.mentions FROM social_mentions m WHERE m.symbol = sc.symbol AND m.snap_date >= DATE_SUB(" +
			screenTodayKST + ", INTERVAL 3 DAY) ORDER BY m.snap_date DESC LIMIT 1)"},
}

/*
screenDaysToEarningsExpr — 다음 발표까지 며칠.

저장된 다가오는 발표가 있으면 그것, 없으면 **작년 같은 무렵 발표일 + 1년**(국내 캘린더의
`EstimateNextAnnouncement`와 같은 규칙, 거래일 맞춤만 뺐다 — 「14일 안」 같은 거르기에는
하루이틀 차이가 결과를 바꾸지 않는다). 국내 예정 발표는 대부분 저장 행이 아니라서, 추정이
없으면 국내에서 이 조건을 잴 수 있는 종목이 0이었다(2026-09-27 실측). 최근 30일 안에 이미
발표했으면 추정하지 않는다 — 그 분기는 끝났다.
*/
var screenDaysToEarningsExpr = "COALESCE(" +
	"(SELECT DATEDIFF(MIN(e.event_date), " + screenTodayKST + ") FROM earnings_events e WHERE e.symbol = sc.symbol AND e.event_date >= " + screenTodayKST + "), " +
	"(SELECT DATEDIFF(DATE_ADD(MIN(e.event_date), INTERVAL 1 YEAR), " + screenTodayKST + ") FROM earnings_events e" +
	" WHERE e.symbol = sc.symbol" +
	" AND e.event_date BETWEEN DATE_SUB(" + screenTodayKST + ", INTERVAL 1 YEAR) AND DATE_ADD(DATE_SUB(" + screenTodayKST + ", INTERVAL 1 YEAR), INTERVAL 45 DAY)" +
	" AND NOT EXISTS (SELECT 1 FROM earnings_events r WHERE r.symbol = sc.symbol" +
	" AND r.event_date BETWEEN DATE_SUB(" + screenTodayKST + ", INTERVAL 30 DAY) AND " + screenTodayKST + ")))"

// screenTodayKST — DB 시간대와 무관하게 KST 오늘. 오전 9시 전 UTC 날짜로 하루 어긋나지 않게.
const screenTodayKST = "DATE(CONVERT_TZ(UTC_TIMESTAMP(), '+00:00', '+09:00'))"

// screenFieldByID는 ID로 찾는 표. 요청이 보낸 이름은 **반드시** 여기를 통과해야 한다.
var screenFieldByID = func() map[string]ScreenField {
	m := make(map[string]ScreenField, len(ScreenFields))
	for _, f := range ScreenFields {
		m[f.ID] = f
	}
	return m
}()

// LookupScreenField는 화이트리스트 조회다. 두 번째 값이 false면 요청을 거절한다.
func LookupScreenField(id string) (ScreenField, bool) {
	f, ok := screenFieldByID[strings.TrimSpace(id)]
	return f, ok
}

/*
ScreenRule — 조건 한 줄.

`Min`·`Max` 둘 다 포인터다. 하나만 주면 「이상」·「이하」, 둘 다 주면 범위다. 둘 다
없으면 **조건이 아니다** — 그런 줄은 거르는 데 쓰이지 않고, 결과표의 열로만 나간다
(「PER로 거르지는 않되 PER은 보고 싶다」가 실제로 흔한 요구다).
*/
type ScreenRule struct {
	Field string   `json:"field"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
}

// Active는 이 줄이 실제로 거르는 조건인지.
func (r ScreenRule) Active() bool { return r.Min != nil || r.Max != nil }

/*
ScreenQuery — 요청 전체.

`Market`은 랭킹과 같은 값을 쓴다("domestic" | "overseas"). 정렬은 **필드 ID**로만
받는다 — 컬럼 이름을 받으면 그것도 주입 통로다.
*/
type ScreenQuery struct {
	Market  string       `json:"market"`
	// Sector — 공통 11섹터 키(Sector). 비우면 전 업종 (2026-09-27 결정 C ③).
	Sector  string       `json:"sector,omitempty"`
	Rules   []ScreenRule `json:"rules"`
	SortBy  string       `json:"sortBy,omitempty"`
	SortAsc bool         `json:"sortAsc,omitempty"`
	Limit   int          `json:"limit,omitempty"`
	// Like — 기준 종목. 있으면 정렬이 「닮은 순」(같은 업종 먼저 · 점수 축 거리)으로 바뀐다(2026-10-01 요청 1).
	Like string `json:"like,omitempty"`
}

const (
	// ScreenMaxRules — 조건 수 상한. 넘으면 거절한다.
	ScreenMaxRules = 12
	// ScreenDefaultLimit / ScreenMaxLimit — 결과 행 수. 랭킹과 같은 눈금이다.
	ScreenDefaultLimit = 100
	ScreenMaxLimit     = 200
)

/*
ScreenFunnel — 이 화면이 다른 스크리너와 갈라지는 자리.

	Total       그날 점수가 계산된 전체 종목
	InMarket    시장(국내/해외)으로 줄인 뒤
	Measurable  **건 조건을 전부 잴 수 있는** 종목
	Passed      그중 조건을 통과한 종목

`InMarket − Measurable`이 「조건을 틀린 것이 아니라 재보지 못한」 종목 수다. 이 숫자를
안 보여 주면 사용자는 결과 0을 시장 탓으로 읽는다.
*/
type ScreenFunnel struct {
	Total    int `json:"total"`
	InMarket int `json:"inMarket"`
	// Eligible — 랭킹과 **같은 자격**을 갖춘 종목 (축 3개 이상 · 지수 추종 상품 제외).
	Eligible   int `json:"eligible"`
	Measurable int `json:"measurable"`
	Passed     int `json:"passed"`
}

// ScreenCell은 결과표 한 칸 — 값과 「잴 수 없음」을 구분해 담는다.
type ScreenCell struct {
	Field string   `json:"field"`
	Value *float64 `json:"value"`
}

// ScreenRow는 결과 한 종목.
type ScreenRow struct {
	Symbol     string   `json:"symbol"`
	Name       string   `json:"name,omitempty"`
	AssetType  string   `json:"assetType,omitempty"`
	Score      *int     `json:"score,omitempty"`
	Grade      string   `json:"grade,omitempty"`
	Close      *float64 `json:"close,omitempty"`
	ChangeRate *float64 `json:"changeRate,omitempty"`
	Currency   string   `json:"currency,omitempty"`

	/*
		아래 셋은 **랭킹 행(StockScoreListItem)과 같은 뜻·같은 이름**이다
		(2026-09-23 QA P0-01).

		두 화면이 같은 종목에 다른 등락률을 적던 이유가 이것이었다 — 랭킹은 장중에
		확정 종가를 실시간 값으로 덮고 그 사실을 `PriceAsOf`로 말하는데, 스크리너에는
		그 세 칸이 아예 없어 덮을 수도, 덮었다고 말할 수도 없었다. 뜻이 갈리지 않게
		이름까지 그대로 가져온다.
	*/

	// PriceAsOf는 `Close`가 어느 날 것인가(YYYY-MM-DD). 실시간으로 덮였으면 "live".
	PriceAsOf string `json:"priceAsOf,omitempty"`
	// PrevClose는 기준가를 고르는 재료 — 화면에는 나가지 않는다.
	PrevClose *float64 `json:"-"`
	// BasePrice는 화면이 스트림 가격으로 등락률을 다시 낼 때 쓰는 기준가.
	// 비어 있으면 화면은 등락률을 **그리지 않는다**(0.00%로 때우지 않는다).
	BasePrice *float64 `json:"basePrice,omitempty"`

	// Cells는 **조건에 쓴 필드**만 담는다 — 화면의 열이 곧 사용자가 건 조건이다.
	Cells []ScreenCell `json:"cells,omitempty"`
}

/*
ScreenFieldCoverage — 필드 하나가 그날 몇 종목에서 측정됐나.

`InMarket`이 분모다. 전체 17,919 중 몇이 아니라 **지금 고른 시장에서** 몇인지를
말해야 화면의 숫자와 맞는다 — 국내만 보는 사람에게 해외 커버리지는 뜻이 없다.
*/
type ScreenFieldCoverage struct {
	Field string `json:"field"`
	Count int    `json:"count"`
}

// ScreenResult는 응답 전체.
type ScreenResult struct {
	TradeDate string       `json:"tradeDate"`
	Market    string       `json:"market"`
	Funnel    ScreenFunnel `json:"funnel"`
	Rows      []ScreenRow  `json:"rows"`
	// Truncated는 상한에 걸려 잘렸는지. 잘린 것을 안 말하면 「29종목뿐」으로 읽힌다.
	Truncated bool `json:"truncated"`
	// Coverage는 **건 조건들**의 측정 종목 수. 조건을 고치기 전에 이유가 보인다.
	Coverage []ScreenFieldCoverage `json:"coverage,omitempty"`
}

/*
ScreenCatalog — 화면이 조건 목록을 그릴 때 받는 것.

필드 정의에 **그날의 커버리지**를 얹어 보낸다. 사용자가 조건을 누르기 전에 「이건
9%만 측정됩니다」가 보여야 한다 — 결과가 0이 된 뒤에 알려 주면 늦다.
*/
type ScreenCatalogField struct {
	ScreenField
	// Count/InMarket은 이 시장에서 측정된 종목 수와 그 분모.
	Count    int `json:"count"`
	InMarket int `json:"inMarket"`
	// Quartiles — 자격 종목의 25·50·75% 값(화면 단위). 「보통 p25~p75」 가이드. 표본이 적으면 없다.
	Quartiles *[3]float64 `json:"quartiles,omitempty"`
}

// ScreenSymbolValues — 한 종목의 스크리너 값 전부 (`GET /stocks/{symbol}/screen-values`).
type ScreenSymbolValues struct {
	Symbol    string             `json:"symbol"`
	Name      string             `json:"name"`
	AssetType string             `json:"assetType"`
	Market    string             `json:"market"` // domestic | overseas — 스크리너 시장 탭
	Sector    string             `json:"sector,omitempty"`
	TradeDate string             `json:"tradeDate"`
	Values    map[string]float64 `json:"values"`
}

type ScreenCatalog struct {
	TradeDate string               `json:"tradeDate"`
	Market    string               `json:"market"`
	InMarket  int                  `json:"inMarket"`
	Fields    []ScreenCatalogField `json:"fields"`
}

/*
UserScreen — 사용자가 이름 붙여 저장한 조건 (V2 요구 4 「저장 가능」).

# DroppedRules가 있는 이유

지표는 사라진다. 저장할 때 있던 필드가 나중에 없어지면 그 조건은 **말없이 빠지는
대신** 몇 개가 빠졌는지로 남는다 — 화면이 「조건 1개는 더 이상 쓸 수 없습니다」를
말할 수 있어야, 사용자가 예전과 다른 결과를 보고 고장이라고 여기지 않는다 (요구 8).
*/
type UserScreen struct {
	ID      int64        `json:"id"`
	Name    string       `json:"name"`
	Market  string       `json:"market"`
	Rules   []ScreenRule `json:"rules"`
	SortBy  string       `json:"sortBy,omitempty"`
	SortAsc bool         `json:"sortAsc,omitempty"`
	// DroppedRules는 지금은 쓸 수 없어 빠진 조건 수.
	DroppedRules int    `json:"droppedRules,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
}

// UserScreenNameMax — 이름 길이 상한. 표의 VARCHAR(60)과 같은 값이다.
const UserScreenNameMax = 60
