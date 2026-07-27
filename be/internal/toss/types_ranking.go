package toss

type RankingType string

const (
	RankingMarketTradingAmount        RankingType = "MARKET_TRADING_AMOUNT"
	RankingMarketTradingVolume        RankingType = "MARKET_TRADING_VOLUME"
	RankingTopGainers                 RankingType = "TOP_GAINERS"
	RankingTopLosers                  RankingType = "TOP_LOSERS"
	RankingTossSecuritiesTradingAmount RankingType = "TOSS_SECURITIES_TRADING_AMOUNT"
	RankingTossSecuritiesTradingVolume RankingType = "TOSS_SECURITIES_TRADING_VOLUME"
)

type RankingDuration string

const (
	RankingRealtime RankingDuration = "realtime"
	Ranking1d       RankingDuration = "1d"
	Ranking1w       RankingDuration = "1w"
	Ranking1mo      RankingDuration = "1mo"
	Ranking3mo      RankingDuration = "3mo"
	Ranking6mo      RankingDuration = "6mo"
	Ranking1y       RankingDuration = "1y"
)

type RankingPrice struct {
	LastPrice  string  `json:"lastPrice"`
	BasePrice  string  `json:"basePrice"`
	ChangeRate *string `json:"changeRate"`
}

type RankingItem struct {
	Rank          int          `json:"rank"`
	Symbol        string       `json:"symbol"`
	Currency      Currency     `json:"currency"`
	Price         RankingPrice `json:"price"`
	TradingVolume string       `json:"tradingVolume"`
	TradingAmount string       `json:"tradingAmount"`
}

type RankingResponse struct {
	RankedAt *string       `json:"rankedAt"`
	Rankings []RankingItem `json:"rankings"`
}
