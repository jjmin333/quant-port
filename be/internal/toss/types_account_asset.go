package toss

type AccountType string

const (
	AccountBrokerage           AccountType = "BROKERAGE"
	AccountOverseasDerivatives AccountType = "OVERSEAS_DERIVATIVES"
	AccountPensionSavings      AccountType = "PENSION_SAVINGS"
	AccountReshoringInvestment AccountType = "RESHORING_INVESTMENT"
)

type Account struct {
	AccountNo   string      `json:"accountNo"`
	AccountSeq  int64       `json:"accountSeq"`
	AccountType AccountType `json:"accountType"`
}

type CurrencyPrice struct {
	KRW string  `json:"krw"`
	USD *string `json:"usd"`
}

type OverviewMarketValue struct {
	Amount           CurrencyPrice `json:"amount"`
	AmountAfterCost  CurrencyPrice `json:"amountAfterCost"`
}

type OverviewProfitLoss struct {
	Amount           CurrencyPrice `json:"amount"`
	AmountAfterCost  CurrencyPrice `json:"amountAfterCost"`
	Rate             string        `json:"rate"`
	RateAfterCost    string        `json:"rateAfterCost"`
}

type OverviewDailyProfitLoss struct {
	Amount CurrencyPrice `json:"amount"`
	Rate   string        `json:"rate"`
}

type MarketValue struct {
	PurchaseAmount  string `json:"purchaseAmount"`
	Amount          string `json:"amount"`
	AmountAfterCost string `json:"amountAfterCost"`
}

type ProfitLoss struct {
	Amount          string `json:"amount"`
	AmountAfterCost string `json:"amountAfterCost"`
	Rate            string `json:"rate"`
	RateAfterCost   string `json:"rateAfterCost"`
}

type DailyProfitLoss struct {
	Amount string `json:"amount"`
	Rate   string `json:"rate"`
}

type Cost struct {
	Commission string  `json:"commission"`
	Tax        *string `json:"tax"`
}

type HoldingsItem struct {
	Symbol               string          `json:"symbol"`
	Name                 string          `json:"name"`
	MarketCountry        MarketCountry   `json:"marketCountry"`
	Currency             Currency        `json:"currency"`
	Quantity             string          `json:"quantity"`
	LastPrice            string          `json:"lastPrice"`
	AveragePurchasePrice string          `json:"averagePurchasePrice"`
	MarketValue          MarketValue     `json:"marketValue"`
	ProfitLoss           ProfitLoss      `json:"profitLoss"`
	DailyProfitLoss      DailyProfitLoss `json:"dailyProfitLoss"`
	Cost                 Cost            `json:"cost"`
}

type HoldingsOverview struct {
	TotalPurchaseAmount CurrencyPrice           `json:"totalPurchaseAmount"`
	MarketValue         OverviewMarketValue     `json:"marketValue"`
	ProfitLoss          OverviewProfitLoss      `json:"profitLoss"`
	DailyProfitLoss     OverviewDailyProfitLoss `json:"dailyProfitLoss"`
	Items               []HoldingsItem          `json:"items"`
}
