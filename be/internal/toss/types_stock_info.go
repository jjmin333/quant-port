package toss

type StockInfo struct {
	Symbol             string          `json:"symbol"`
	Name               string          `json:"name"`
	EnglishName        string          `json:"englishName"`
	ISINCode           string          `json:"isinCode"`
	Market             string          `json:"market"`
	SecurityType       string          `json:"securityType"`
	IsCommonShare      bool            `json:"isCommonShare"`
	Status             string          `json:"status"`
	Currency           Currency        `json:"currency"`
	ListDate           *string         `json:"listDate"`
	DelistDate         *string         `json:"delistDate"`
	SharesOutstanding  string          `json:"sharesOutstanding"`
	LeverageFactor     *string         `json:"leverageFactor"`
	KoreanMarketDetail *KrMarketDetail `json:"koreanMarketDetail"`
}

type KrMarketDetail struct {
	LiquidationTrading    bool  `json:"liquidationTrading"`
	NxtSupported          bool  `json:"nxtSupported"`
	KrxTradingSuspended   bool  `json:"krxTradingSuspended"`
	NxtTradingSuspended   *bool `json:"nxtTradingSuspended"`
}

type StockWarning struct {
	WarningType string  `json:"warningType"`
	Exchange    *string `json:"exchange"`
	StartDate   *string `json:"startDate"`
	EndDate     *string `json:"endDate"`
}
