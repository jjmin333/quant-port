package apibudget

import "time"

// Provider identifies an external data API integrated by quant-port.
type Provider string

const (
	ProviderGoldPrice Provider = "goldprice"
	ProviderFRED      Provider = "fred"
	ProviderFinnhub   Provider = "finnhub"
	ProviderUpbit     Provider = "upbit"
	ProviderDART      Provider = "dart"
	ProviderEdgar     Provider = "edgar"
	ProviderNaver     Provider = "naver"
	ProviderKakao     Provider = "kakao"
)

// Limits documents enforced quotas. See be/docs/api-quotas.md for sources.
type Limits struct {
	MonthlyQuota int
	DailyQuota   int
	PerMinute    int
	MinInterval  time.Duration
}

// DefaultLimits uses conservative headroom below published provider caps.
var DefaultLimits = map[Provider]Limits{
	ProviderGoldPrice: {
		MonthlyQuota: 900,
		PerMinute:    20,
		MinInterval:  90 * time.Minute,
	},
	ProviderFRED: {
		DailyQuota:  5000,
		PerMinute:   100,
		MinInterval: 3 * time.Minute,
	},
	ProviderFinnhub: {
		PerMinute:   50,
		MinInterval: 5 * time.Minute,
	},
	ProviderUpbit: {
		PerMinute:   10,
		MinInterval: 2 * time.Minute,
	},
	ProviderDART: {
		DailyQuota:  18000,
		PerMinute:   60,
		MinInterval: 1 * time.Second,
	},
	ProviderEdgar: {
		PerMinute:   8,
		MinInterval: 150 * time.Millisecond,
	},
	ProviderNaver: {
		DailyQuota: 24000,
		// News poller bursts up to 18 keyword searches every 2 minutes; Naver/API Hub
		// enforces daily quota (and 429 on burst). No artificial spacing between keywords.
	},
	ProviderKakao: {
		DailyQuota: 3000,
	},
}
