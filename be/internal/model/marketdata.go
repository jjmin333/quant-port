package model

import "time"

// Market data cache / stream revision and snapshot types.

const (
	MarketDataQuoteCacheSec      = 1
	MarketDataOrderBookCacheSec  = 1
	MarketDataTradesCacheSec     = 1
	MarketDataIndicesCacheSec    = 5
	MarketDataListCacheSec       = 3              // catalog quotes only; user flags stripped before cache (see stripQuoteListForCache)
	MarketDataCandleHistCacheSec = 3600           // completed daily+ bars
	MarketDataCandleTailCacheSec = 90             // intraday tail
	MarketDataLastGoodCacheSec   = 30 * 24 * 3600 // stale detail/trades when market closed
)

type CandleSnapshot struct {
	Symbol     string         `json:"symbol"`
	Interval   CandleInterval `json:"interval"`
	Completed  []Candle       `json:"completed"`
	Forming    *Candle        `json:"forming,omitempty"`
	NextBefore *string        `json:"nextBefore,omitempty"`
	Rev        int64          `json:"rev"`
	Source     string         `json:"source"`
}

type QuoteTerminalSnapshot struct {
	Detail    QuoteDetail `json:"detail"`
	OrderBook OrderBook   `json:"orderBook"`
	Trades    []TradeTick `json:"trades"`
	Source    string      `json:"source"`
	/*
		이 스냅샷을 **만든** 시각 (2026-09-05 사용자 지적 6).

		Detail.UpdatedAt은 「가격이 정해진 시각」이라 다른 질문에 답한다. 여기 있는 것은
		「캐시에 언제 넣었나」이고, 그 값으로 `EnsureTerminalSnapshot`이 **신선한지**와
		**살아 있는지**를 나눠 본다 — 신선하면 그대로 주고, 낡았으면 그것도 주되 뒤에서
		새로 만든다(stale-while-revalidate).

		예전 캐시에는 이 칸이 없다. 그때는 zero value가 되어 「낡음」으로 읽히므로,
		배포 직후 한 번 뒤에서 새로 만들고 그다음부터 제대로 돈다.
	*/
	CachedAt time.Time `json:"cachedAt,omitempty"`
}

// StreamEvent is pushed over SSE to chart clients.
type StreamEvent struct {
	Type     string         `json:"type"` // snapshot | bar_update | bar_close
	Symbol   string         `json:"symbol"`
	Interval CandleInterval `json:"interval,omitempty"`
	Bar      *Candle        `json:"bar,omitempty"`
	Forming  *Candle        `json:"forming,omitempty"`
	Rev      int64          `json:"rev"`
}

// QuoteStreamEvent is pushed over SSE for live header/detail price updates.
type QuoteStreamEvent struct {
	Type          string  `json:"type"` // quote_update
	Symbol        string  `json:"symbol"`
	CurrentPrice  float64 `json:"currentPrice"`
	Change        float64 `json:"change"`
	ChangeRate    float64 `json:"changeRate"`
	PrevClose     float64 `json:"prevClose"`
	RegularClose  float64 `json:"regularClose,omitempty"`
	MarketSession string  `json:"marketSession,omitempty"`
	UpdatedAt     string  `json:"updatedAt"`
}

/*
ListPriceEvent — 목록 화면 한 줄의 **움직이는 부분만** 담은 이벤트.

`QuoteStreamEvent`와 달리 등락률·기준가가 없다. 의도한 것이다:

	기준가(prevClose)는 **세션 안에서 변하지 않는 값**이다. 목록이 처음 뜰 때
	한 번 받아 두면 그 뒤로는 가격만 있으면 등락률을 화면에서 낼 수 있다.

	반대로 서버가 등락률을 채워 보내려면 종목마다 기준가를 풀어야 하고, 그건
	`resolvePrevClose` 경로라 **종목당 4~5 SELECT**다(실측 2026-09-04:
	`/stocks/scores/top` 20줄이 100 SELECT). 2초마다 그걸 하면 푸시로 옮긴
	이유가 사라진다 — 뷰어 수와 무관해야 할 작업이 다시 뷰어 수에 비례한다.

그래서 이벤트는 가격·통화·시각 셋뿐이고, 나머지 산수는 이미 기준가를 들고 있는
화면이 한다(`fe/src/hooks/use-quote-stream.ts`).
*/
type ListPriceEvent struct {
	Type      string  `json:"type"` // list_price
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Currency  string  `json:"currency,omitempty"`
	UpdatedAt string  `json:"updatedAt"`
}
