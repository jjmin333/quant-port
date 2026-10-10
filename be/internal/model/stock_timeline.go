package model

/*
종목 타임라인 — 「주가가 왜 움직였는지」를 한 줄에 모은다 (V2 요구 3).

# 새로 모으는 것이 없다

재료는 전부 이미 DB에 있다: 일별 종가 · AI 점수와 그 귀인 · 실적 발표 · 뉴스 · 내부자
거래 · 공시. 이 기능이 하는 일은 **그것들을 같은 날짜 축에 세우는 것**뿐이다.

# 이 화면의 제1 규칙 — 레인마다 시작일이 다르다

실측(005930, 2026-09-18):

	종가    2024-12-16부터 (433일)
	뉴스    2026-01-14부터
	실적    2025-07-08부터 (5건)
	내부자  2026-07-21부터
	공시    2026-08-20부터
	AI 점수 2026-08-07부터 (보관 30거래일)

1년을 그리면 열 달치 내부자·공시·점수 레인이 빈다. 그것은 **「아무 일도 없었다」가
아니라 「수집 전」**이고, 둘을 같은 빈칸으로 그리면 화면이 거짓말을 한다. 그래서 모든
레인이 `AvailableFrom`을 들고 나가고, 화면은 그 앞 구간을 다르게 칠한다 (요구 8).

# 왜 사건을 날짜로 묶는가

삼성전자는 **하루 평균 180건**의 뉴스가 붙는다(실측). 기사마다 점을 찍으면 타임라인이
아니라 벽이 된다. 그래서 뉴스는 **날짜별 건수**로 접고, 제목은 사용자가 그 날을 펼 때
기존 뉴스 목록 API가 답한다 — 이 응답이 기사 본문을 나르지 않는다.
*/

// TimelineLaneID identifies one row of the timeline.
type TimelineLaneID string

const (
	TimelineLaneClose      TimelineLaneID = "close"
	TimelineLaneScore      TimelineLaneID = "score"
	TimelineLaneEarnings   TimelineLaneID = "earnings"
	TimelineLaneNews       TimelineLaneID = "news"
	TimelineLaneInsider    TimelineLaneID = "insider"
	TimelineLaneDisclosure TimelineLaneID = "disclosure"
)

var TimelineLaneLabels = map[TimelineLaneID]string{
	TimelineLaneClose:      "주가",
	TimelineLaneScore:      "AI 점수",
	TimelineLaneEarnings:   "실적",
	TimelineLaneNews:       "뉴스",
	TimelineLaneInsider:    "내부자 거래",
	TimelineLaneDisclosure: "공시",
}

// TimelineLaneOrder is the display order, top to bottom.
var TimelineLaneOrder = []TimelineLaneID{
	TimelineLaneClose,
	TimelineLaneScore,
	TimelineLaneEarnings,
	TimelineLaneNews,
	TimelineLaneInsider,
	TimelineLaneDisclosure,
}

/*
TimelineLane — 한 레인이 **언제부터 기록돼 있는지**.

`AvailableFrom`이 비어 있으면 이 종목에 그 자료가 **한 건도 없다**. 요청한 창보다 늦게
시작하면 그 앞은 「수집 전」이고, 화면은 그 구간에 사건이 없다고 말하면 안 된다.
*/
type TimelineLane struct {
	ID    TimelineLaneID `json:"id"`
	Label string         `json:"label"`
	// AvailableFrom은 이 종목의 **전체 기록** 기준 첫 날이다(요청 창과 무관).
	AvailableFrom string `json:"availableFrom,omitempty"`
	// Count는 요청한 창 안의 건수.
	Count int `json:"count"`
	// CoversWindow: 요청 창의 시작보다 먼저부터 기록이 있는가.
	CoversWindow bool `json:"coversWindow"`
}

// TimelinePoint is one close for the chart.
type TimelinePoint struct {
	Date  string  `json:"date"`
	Close float64 `json:"close"`
	// ChangeRate는 저장된 값을 그대로 쓴다 — 화면이 다시 계산하면 배치와 갈린다.
	ChangeRate *float64 `json:"changeRate,omitempty"`
}

// TimelineScore is that day's AI score and what moved it.
type TimelineScore struct {
	Value *int `json:"value"`
	// Delta / Driver는 직전 회차와 비교했을 때만 있다. 점수 배치가 매일 도는 것이
	// 아니므로(실측 27/30) 첫 회차에는 비어 있다.
	Delta *int `json:"delta,omitempty"`
	// Driver는 Phase 1의 귀인이 만든 한 문장이다 — 「추세 19.2 → 26.9」.
	Driver string `json:"driver,omitempty"`
	// DriverKind: "axis" | "availability" | "shrink".
	DriverKind string `json:"driverKind,omitempty"`
}

// TimelineNews is the day's article count. 제목은 나르지 않는다 — 머리글 참조.
type TimelineNews struct {
	Count int `json:"count"`
}

/*
TimelineInsider — 그날 내부자 거래의 **순증감**.

건수만으로는 방향을 알 수 없고, 매수와 매도를 따로만 보여 주면 읽는 사람이 뺄셈을
한다. 셋 다 낸다.
*/
type TimelineInsider struct {
	Count      int     `json:"count"`
	BuyShares  float64 `json:"buyShares"`
	SellShares float64 `json:"sellShares"`
	NetShares  float64 `json:"netShares"`
}

type TimelineDisclosure struct {
	Title     string `json:"title"`
	ReceiptNo string `json:"receiptNo,omitempty"`
	Filer     string `json:"filer,omitempty"`
}

/*
TimelineEarnings — 그날 발표된 실적.

`Surprise`는 **추정치가 있을 때만** 계산한다. 컨센서스가 없는데 0%로 적으면
「예상대로였다」가 되는데, 사실은 「비교할 예상이 없었다」이다.
*/
type TimelineEarnings struct {
	Period  string `json:"period"`
	Session string `json:"session,omitempty"`
	// Status: "actual"(발표치) | "estimate"(예정·추정)
	Status      string   `json:"status"`
	RevenueAct  *float64 `json:"revenueActual,omitempty"`
	RevenueEst  *float64 `json:"revenueEstimate,omitempty"`
	OpIncomeAct *float64 `json:"opIncomeActual,omitempty"`
	EPSAct      *float64 `json:"epsActual,omitempty"`
	EPSEst      *float64 `json:"epsEstimate,omitempty"`
	// Surprise는 EPS 기준 (추정 대비 %). 추정이 없거나 0이면 비어 있다.
	Surprise *float64 `json:"surprise,omitempty"`
	Currency string   `json:"currency,omitempty"`
}

/*
TimelineDay — 사건이 있었던 하루.

`Reasons`는 **이 날이 왜 목록에 있는지**를 적는다. 큰 등락 때문일 수도 있고 실적
발표 때문일 수도 있는데, 그 구분이 없으면 「급등한 날」과 「공시가 있었던 날」이 같은
줄로 보인다.
*/
type TimelineDay struct {
	Date        string   `json:"date"`
	Close       *float64 `json:"close,omitempty"`
	ChangeRate  *float64 `json:"changeRate,omitempty"`
	Significant bool     `json:"significant"`
	Reasons     []string `json:"reasons,omitempty"`

	Score       *TimelineScore       `json:"score,omitempty"`
	Earnings    *TimelineEarnings    `json:"earnings,omitempty"`
	News        *TimelineNews        `json:"news,omitempty"`
	Insider     *TimelineInsider     `json:"insider,omitempty"`
	Disclosures []TimelineDisclosure `json:"disclosures,omitempty"`
}

// StockTimeline is the whole payload.
type StockTimeline struct {
	Symbol   string `json:"symbol"`
	Name     string `json:"name,omitempty"`
	Currency string `json:"currency,omitempty"`
	FromDate string `json:"fromDate"`
	ToDate   string `json:"toDate"`

	Closes []TimelinePoint `json:"closes"`
	Days   []TimelineDay   `json:"days"`
	Lanes  []TimelineLane  `json:"lanes"`

	/*
		MoveThreshold — 「큰 등락」의 문턱(%).

		종목마다 다르다. 하루 8%가 예사인 종목과 1%도 드문 종목에 같은 잣대를 대면 한쪽은
		전부 급등이고 다른 쪽은 아무 일도 없다. 이 창의 표준편차에서 뽑고, 그 값을 응답에
		적는다 — 화면이 「±N% 이상을 큰 등락으로 봤습니다」라고 말할 수 있어야 한다.
	*/
	MoveThreshold float64 `json:"moveThreshold"`
	// Volatility는 문턱의 근거 — 이 창의 일간 등락률 표준편차(%).
	Volatility float64 `json:"volatility"`
}

// TimelineMoveFloor / TimelineMoveCeil bound the per-symbol threshold.
//
// 문턱이 표준편차만 따르면 변동성이 아주 낮은 종목에서 0.4%짜리 날이 전부 「급등」이
// 되고, 아주 높은 종목에서는 15% 미만이 전부 조용한 날이 된다. 사람이 「큰 움직임」이라
// 부르는 범위 안에 가둔다.
const (
	TimelineMoveFloor = 2.0
	TimelineMoveCeil  = 10.0
	// TimelineMoveSigma — 표준편차의 몇 배를 문턱으로 삼을까.
	TimelineMoveSigma = 1.5
	// TimelineScoreJump — 이만큼 점수가 움직이면 그 자체로 표시할 만한 날이다.
	TimelineScoreJump = 3
)

/*
TimelineNewsItem — 하루를 펼쳤을 때의 기사 한 줄.

타임라인 본문에는 **건수만** 나간다. 이 타입은 사용자가 그 날을 열었을 때만 쓰인다 —
90일 창에 하루 180건을 다 실으면 응답이 메가바이트가 된다.
*/
type TimelineNewsItem struct {
	ID string `json:"id"`
	// Headline은 화면에 그대로 나갈 제목이다. 한국어 번역이 있으면 그쪽을 쓴다.
	Headline    string `json:"headline"`
	URL         string `json:"url"`
	Publisher   string `json:"publisher,omitempty"`
	PublishedAt string `json:"publishedAt"`
	ViewCount   int    `json:"viewCount"`
}

// TimelineDayNews is the expanded day's article list.
type TimelineDayNews struct {
	Symbol string             `json:"symbol"`
	Date   string             `json:"date"`
	Items  []TimelineNewsItem `json:"items"`
}
