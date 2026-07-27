package toss

type ExchangeRateResponse struct {
	BaseCurrency    Currency `json:"baseCurrency"`
	QuoteCurrency   Currency `json:"quoteCurrency"`
	Rate            string   `json:"rate"`
	MidRate         string   `json:"midRate"`
	BasisPoint      string   `json:"basisPoint"`
	RateChangeType  string   `json:"rateChangeType"`
	ValidFrom       string   `json:"validFrom"`
	ValidUntil      string   `json:"validUntil"`
}

type KrMarketCalendarResponse struct {
	Today               KrMarketDay `json:"today"`
	PreviousBusinessDay KrMarketDay `json:"previousBusinessDay"`
	NextBusinessDay     KrMarketDay `json:"nextBusinessDay"`
}

type UsMarketCalendarResponse struct {
	Today               UsMarketDay `json:"today"`
	PreviousBusinessDay UsMarketDay `json:"previousBusinessDay"`
	NextBusinessDay     UsMarketDay `json:"nextBusinessDay"`
}

type KrMarketDay struct {
	Date       string          `json:"date"`
	Integrated *IntegratedHour `json:"integrated"`
}

type UsMarketDay struct {
	Date          string                `json:"date"`
	DayMarket     *UsDayMarketSession   `json:"dayMarket"`
	PreMarket     *UsPreMarketSession   `json:"preMarket"`
	RegularMarket *UsRegularMarketSession `json:"regularMarket"`
	AfterMarket   *UsAfterMarketSession `json:"afterMarket"`
}

type IntegratedHour struct {
	PreMarket     *PreMarketSession     `json:"preMarket"`
	RegularMarket *RegularMarketSession `json:"regularMarket"`
	AfterMarket   *AfterMarketSession   `json:"afterMarket"`
}

type PreMarketSession struct {
	StartTime                   string  `json:"startTime"`
	SinglePriceAuctionStartTime *string `json:"singlePriceAuctionStartTime"`
	EndTime                     string  `json:"endTime"`
}

type RegularMarketSession struct {
	StartTime                   string  `json:"startTime"`
	SinglePriceAuctionStartTime *string `json:"singlePriceAuctionStartTime"`
	EndTime                     string  `json:"endTime"`
}

type AfterMarketSession struct {
	StartTime                string  `json:"startTime"`
	SinglePriceAuctionEndTime *string `json:"singlePriceAuctionEndTime"`
	EndTime                  string  `json:"endTime"`
}

type UsDayMarketSession struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type UsPreMarketSession struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type UsRegularMarketSession struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type UsAfterMarketSession struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}
