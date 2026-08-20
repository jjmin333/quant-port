package apibudget

import (
	"encoding/json"
	"time"
)

// Provider identifies an external data API integrated by quant-port.
type Provider string

const (
	ProviderGoldPrice       Provider = "goldprice"
	ProviderFRED            Provider = "fred"
	ProviderFinnhub         Provider = "finnhub"
	ProviderFinnhubMigrate  Provider = "finnhub_migrate"
	ProviderFinnhubNews     Provider = "finnhub_news"
	ProviderFinnhubEarnings Provider = "finnhub_earnings"
	ProviderKRX             Provider = "krx"
	ProviderUpbit           Provider = "upbit"
	ProviderDART            Provider = "dart"
	ProviderEdgar           Provider = "edgar"
	ProviderEdgarBulk       Provider = "edgar_bulk"
	ProviderFnGuide         Provider = "fnguide"
	ProviderNaver           Provider = "naver"
	ProviderPapagoText      Provider = "papago_text"
	ProviderPapagoWeb       Provider = "papago_web"
	ProviderPapago          Provider = "papago" // deprecated alias for papago_text
	ProviderKakao           Provider = "kakao"
	ProviderOpenAI          Provider = "openai"
)

// Limits documents enforced quotas. See be/docs/api-quotas.md for sources.
type Limits struct {
	MonthlyQuota int
	DailyQuota   int
	PerMinute    int
	MinInterval  time.Duration
	// TokensPerMinute is a token-based cap (as opposed to request-count),
	// currently only meaningful for LLM providers. 0 means unlimited/unset.
	TokensPerMinute int
	// TokensPerDay is the daily token allowance. LLM free tiers cap both
	// requests and tokens, and a batch of a few hundred symbols hits the
	// token ceiling long before the request ceiling if prompts grow — so
	// this is enforced alongside DailyQuota. 0 means unlimited/unset.
	TokensPerDay int64
}

// MarshalJSON renders MinInterval as milliseconds; the zero-value time.Duration
// otherwise round-trips as a Go duration string, which FE cannot parse.
func (l Limits) MarshalJSON() ([]byte, error) {
	type alias struct {
		MonthlyQuota    int   `json:"monthlyQuota,omitempty"`
		DailyQuota      int   `json:"dailyQuota,omitempty"`
		PerMinute       int   `json:"perMinute,omitempty"`
		MinIntervalMs   int64 `json:"minIntervalMs,omitempty"`
		TokensPerMinute int   `json:"tokensPerMinute,omitempty"`
		TokensPerDay    int64 `json:"tokensPerDay,omitempty"`
	}
	return json.Marshal(alias{
		MonthlyQuota:    l.MonthlyQuota,
		DailyQuota:      l.DailyQuota,
		PerMinute:       l.PerMinute,
		MinIntervalMs:   l.MinInterval.Milliseconds(),
		TokensPerMinute: l.TokensPerMinute,
		TokensPerDay:    l.TokensPerDay,
	})
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
	// FinnhubMigrate — batch catalog jobs only (/stock/symbol). Same API key but
	// separate bucket so migration does not trip operational MinInterval.
	ProviderFinnhubMigrate: {
		PerMinute:   5,
		MinInterval: 30 * time.Second,
	},
	ProviderFinnhubNews: {
		PerMinute:   50,
		MinInterval: 2 * time.Minute,
	},
	// FinnhubEarnings — 실적 캘린더 배치 전용 버킷. 같은 API 키를 쓰지만 버킷을
	// 나눈 이유는 FinnhubMigrate와 똑같다: ProviderFinnhub의 MinInterval 5분은
	// **라이브 ETF 시세 폴러**를 위한 간격이라, 실적 배치가 폴러가
	// 방금 부른 것 때문에 실패한다(실측에서 첫 실행이 바로 그렇게 죽었다).
	// 한도를 나누는 게 아니라 "누가 언제 부르는지"를 나눈다 — 실제 호출 수는
	// 06:00 전체 창 ~18회 + 5분 당일 폴링 ~288회다. 같은 키의 ETF 폴러(5분×2)와
	// 겹쳐도 피크는 분당 한 자릿수다.
	ProviderFinnhubEarnings: {
		PerMinute:   5,
		MinInterval: 20 * time.Second,
	},
	ProviderKRX: {
		DailyQuota:  500,
		PerMinute:   10,
		MinInterval: 2 * time.Second,
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
	// EdgarBulk — 내부자 거래(Form 4) 적재 배치 전용 버킷.
	//
	// ProviderEdgar의 8/min은 **사용자 요청 경로**를 위한 값이다. 종목 하나에
	// 4호출이 드는 조회가 그 버킷을 쓰던 시절에는 동시 접속 두 명이면 두 번째가
	// 빈 화면을 받았다. 이제 조회는 DB만 읽으므로 그 8/min은 남은 온디맨드
	// 경로(재무 스냅샷)의 몫으로 그대로 두고, 배치는 여기서 센다.
	//
	// 간격 300ms는 약 3.3 req/sec, 분당 200회다. SEC가 공표한 fair access 한도는
	// **10 req/sec**이므로 그 3분의 1이다(docs/api-quotas.md). 상한 240/min을
	// 정상 속도보다 위에 둔 것은 안전판이라서다 — 두 값이 같으면 간격을 지켜도
	// 분당 한도에 걸릴 수 있고, Check 실패는 그 종목을 통째로 건너뛰게 만든다.
	//
	// 나눈 것은 한도가 아니라 "누가 언제 부르는지"고, 실제 호출량은 증분 적재라
	// 평시 종목당 1~2회다 — ProviderFinnhubEarnings가 같은 이유로 먼저 이렇게
	// 갈라졌다.
	ProviderEdgarBulk: {
		PerMinute:   240,
		MinInterval: 300 * time.Millisecond,
	},
	// FnGuide — 국내 실적발표 캘린더. 공개 API가 아니라 Company Guide 화면이 쓰는
	// 내부 JSON이므로, 게시된 한도가 없는 대신 **우리가 스스로 낮게 잡는다**.
	// 시즌에는 당월을 하루 여섯 번 다시 받고 20시에만 전월을 겹친다(하루 7호출).
	// 상한 60은 그 위에 관리자 백필(월 단위 2년치 ≈ 24회)까지 한 번에 돌 여유다.
	ProviderFnGuide: {
		DailyQuota:  60,
		PerMinute:   2,
		MinInterval: 30 * time.Second,
	},
	ProviderNaver: {
		DailyQuota: 24000,
		// News poller bursts up to 18 keyword searches every 2 minutes; Naver/API Hub
		// enforces daily quota (and 429 on burst). No artificial spacing between keywords.
	},
	// Papago NCP bills by character (default 5M/day), not by request count,
	// and does not publish an RPM. PerMinute 60 is our burst cap so the
	// 2-minute backfill of 24 headlines stays on Papago; 20/min used to
	// overflow the rest onto Gemini. DailyQuota 6000 is a cost brake, not
	// an NCP figure.
	ProviderPapagoText: {
		DailyQuota:  6000,
		PerMinute:   60,
		MinInterval: 150 * time.Millisecond,
	},
	ProviderPapagoWeb: {
		DailyQuota:  2000,
		PerMinute:   10,
		MinInterval: 300 * time.Millisecond,
	},
	ProviderPapago: {
		DailyQuota:  6000,
		PerMinute:   60,
		MinInterval: 150 * time.Millisecond,
	},
	ProviderKakao: {
		DailyQuota: 3000,
	},
	// OpenAI-compatible LLM provider (OpenAI or, currently, Gemini via its
	// OpenAI-compat endpoint — see config.buildLLMConfig). Set to Gemini
	// 3.1 Flash-Lite's free-tier limits: 15 RPM, 250K TPM, 500 RPD.
	// MinInterval evenly spaces calls at 60s/15 = 4s.
	//
	// TokensPerDay has no published free-tier figure we can pin to, so it is
	// a self-imposed ceiling: 1M/day is ~380 calls at the ~2.6K tokens the
	// AI 요약 prompt actually costs, which covers the daily 마켓 레이더 batch
	// (~240 symbols) with room to spare. Every value here is overridable via
	// env (LLM_DAILY_QUOTA / LLM_PER_MINUTE / LLM_TOKENS_PER_MINUTE /
	// LLM_TOKENS_PER_DAY) so the ceilings can be matched to whatever the
	// provider console actually reports, without a code change.
	ProviderOpenAI: {
		DailyQuota:      500,
		PerMinute:       15,
		MinInterval:     4 * time.Second,
		TokensPerMinute: 250_000,
		TokensPerDay:    1_000_000,
	},
}
