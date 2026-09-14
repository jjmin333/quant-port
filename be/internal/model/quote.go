package model

type QuoteCurrency string

const (
	CurrencyKRW QuoteCurrency = "KRW"
	CurrencyUSD QuoteCurrency = "USD"
)

type QuoteAssetType string

const (
	AssetDomestic QuoteAssetType = "domestic"
	AssetUS       QuoteAssetType = "us"
	AssetETF      QuoteAssetType = "etf"
)

type CandleInterval string

const (
	Interval1m  CandleInterval = "1m"
	Interval5m  CandleInterval = "5m"
	Interval10m CandleInterval = "10m"
	Interval30m CandleInterval = "30m"
	Interval1d  CandleInterval = "1d"
	Interval1w  CandleInterval = "1w"
	Interval1M  CandleInterval = "1M"
)

type MarketIndex struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Value      float64       `json:"value"`
	Change     float64       `json:"change"`
	ChangeRate float64       `json:"changeRate"`
	Currency   QuoteCurrency `json:"currency"`
	Badge      string        `json:"badge,omitempty"`
	Sparkline  []float64     `json:"sparkline,omitempty"`
	UpdatedAt  string        `json:"updatedAt,omitempty"`
	DataKind   IndexDataKind `json:"dataKind,omitempty"`
}

// IndexDataKind describes whether a value is live or end-of-day.
type IndexDataKind string

const (
	IndexDataKindLive IndexDataKind = "live"
	IndexDataKindEOD  IndexDataKind = "eod"
)

type IndexGroupMeta struct {
	UpdatedAt string `json:"updatedAt"`
	Source    string `json:"source"`
}

type QuoteSummary struct {
	Symbol           string            `json:"symbol"`
	Name             string            `json:"name"`
	Market           string            `json:"market"`
	Currency         QuoteCurrency     `json:"currency"`
	AssetType        QuoteAssetType    `json:"assetType"`
	LogoURL          string            `json:"logoUrl,omitempty"`
	CurrentPrice     float64           `json:"currentPrice"`
	Change           float64           `json:"change"`
	ChangeRate       float64           `json:"changeRate"`
	Volume           int64             `json:"volume"`
	TradingValue     float64           `json:"tradingValue"`
	IsWatchlisted    bool              `json:"isWatchlisted"`
	IsHeld           bool              `json:"isHeld"`
	WatchlistMetrics *WatchlistMetrics `json:"watchlistMetrics,omitempty"`
}

type QuoteDetail struct {
	QuoteSummary
	Open          float64 `json:"open"`
	High          float64 `json:"high"`
	Low           float64 `json:"low"`
	PrevClose     float64 `json:"prevClose"`
	RegularClose  float64 `json:"regularClose,omitempty"`
	MarketSession string  `json:"marketSession,omitempty"`
	/*
		마감 세션에서만 채워지는 두 시각(RFC3339). 열려 있는 동안에는 비어 있다.

		`SessionClosedAt`은 **이 가격이 멈춘 시각**이다 — 화면이 「오전 12:33:08 기준」
		처럼 응답을 받은 시각을 찍지 않게 한다. `NextSessionAt`은 다음 창(국내 프리마켓
		08:00 등)이 열리는 시각이고, 화면은 그때까지 시세 폴링을 멈춘다.
		근거와 계산은 `service/market_session_clock.go`.
	*/
	SessionClosedAt string   `json:"sessionClosedAt,omitempty"`
	NextSessionAt   string   `json:"nextSessionAt,omitempty"`
	Week52High      float64  `json:"week52High"`
	Week52Low       float64  `json:"week52Low"`
	MarketCap       float64  `json:"marketCap"`
	PER             *float64 `json:"per"`
	PBR             *float64 `json:"pbr"`
	UpdatedAt       string   `json:"updatedAt"`
}

type OrderBookLevel struct {
	Price      float64 `json:"price"`
	Quantity   int64   `json:"quantity"`
	OrderCount int     `json:"orderCount"`
}

type OrderBook struct {
	Symbol    string           `json:"symbol"`
	Asks      []OrderBookLevel `json:"asks"`
	Bids      []OrderBookLevel `json:"bids"`
	Timestamp string           `json:"timestamp"`
}

type TradeTick struct {
	ID       string  `json:"id"`
	Symbol   string  `json:"symbol"`
	Time     string  `json:"time"`
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
	Side     string  `json:"side"`
}

type Candle struct {
	Time   string  `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`

	// SrcBars — 이 봉이 몇 개의 1분봉에서 나왔는가. 응답에는 싣지 않는다.
	//
	// 계층 저장(candle_bars.src_bars) 때문에 필요하다. 같은 시각을 두 계층이
	// 덮을 수 있고 — Toss 백필이 이미 접힌 구간에 1분봉을 되넣는다 — 그때
	// **더 완전한 쪽**을 골라야 한다. 「고운 계층 우선」으로 고르면 되넣어진
	// 1분봉 한두 개가 완전한 5분봉을 덮어써 거래량이 무너진다
	// (실측 005930 08-19 16:10: 255,170 → 18,402).
	//
	// Toss에서 갓 받은 1분봉은 0이다. 저장 경로가 값을 쓰지 않으므로 DB 기본값
	// 1이 적용되고, 조회 시 0이면 봉 하나로 센다(candleCoverage).
	SrcBars int `json:"-"`
}
