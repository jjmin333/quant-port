package toss

type MarketIndicatorPriceResponse struct {
	Symbol    string  `json:"symbol"`
	Timestamp *string `json:"timestamp"`
	LastPrice string  `json:"lastPrice"`
}

type MarketIndicatorCandle struct {
	Timestamp  string `json:"timestamp"`
	OpenPrice  string `json:"openPrice"`
	HighPrice  string `json:"highPrice"`
	LowPrice   string `json:"lowPrice"`
	ClosePrice string `json:"closePrice"`
	Volume     string `json:"volume"`
}

type MarketIndicatorCandlePageResponse struct {
	Candles    []MarketIndicatorCandle `json:"candles"`
	NextBefore *string                 `json:"nextBefore"`
}

type InvestorTradingAmount struct {
	BuyAmount  string `json:"buyAmount"`
	SellAmount string `json:"sellAmount"`
}

type InstitutionTradingBreakdown struct {
	FinancialInvestment       InvestorTradingAmount `json:"financialInvestment"`
	Insurance                 InvestorTradingAmount `json:"insurance"`
	Trust                     InvestorTradingAmount `json:"trust"`
	PrivateEquityFund         InvestorTradingAmount `json:"privateEquityFund"`
	Bank                      InvestorTradingAmount `json:"bank"`
	OtherFinancialInstitution InvestorTradingAmount `json:"otherFinancialInstitution"`
	PensionFund               InvestorTradingAmount `json:"pensionFund"`
}

type InstitutionTradingAmount struct {
	BuyAmount  string                      `json:"buyAmount"`
	SellAmount string                      `json:"sellAmount"`
	Breakdown  InstitutionTradingBreakdown `json:"breakdown"`
}

type InvestorTradingRecord struct {
	Date             string                   `json:"date"`
	UpdatedAt        string                   `json:"updatedAt"`
	Individual       InvestorTradingAmount    `json:"individual"`
	Foreigner        InvestorTradingAmount    `json:"foreigner"`
	Institution      InstitutionTradingAmount `json:"institution"`
	OtherCorporation InvestorTradingAmount    `json:"otherCorporation"`
}

type InvestorTradingResponse struct {
	NextUntil *string                 `json:"nextUntil"`
	Records   []InvestorTradingRecord `json:"records"`
}

type InvestorTradingInterval string

const (
	InvestorInterval1d  InvestorTradingInterval = "1d"
	InvestorInterval1w  InvestorTradingInterval = "1w"
	InvestorInterval1mo InvestorTradingInterval = "1mo"
	InvestorInterval1y  InvestorTradingInterval = "1y"
)
