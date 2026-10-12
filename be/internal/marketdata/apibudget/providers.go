package apibudget

import (
	"encoding/json"
	"time"
)

// Provider identifies an external data API integrated by quant-port.
type Provider string

const (
	ProviderGoldPrice      Provider = "goldprice"
	ProviderFRED           Provider = "fred"
	ProviderFinnhub        Provider = "finnhub"
	ProviderFinnhubMigrate Provider = "finnhub_migrate"
	ProviderFinnhubNews    Provider = "finnhub_news"
	// 종목별 뉴스는 버킷이 다르다 — providers.go의 Limits 머리글 참조.
	ProviderFinnhubNewsCompany Provider = "finnhub_news_company"
	ProviderFinnhubEarnings    Provider = "finnhub_earnings"
	ProviderFinnhubBackfill    Provider = "finnhub_backfill"
	// 2026-09-28 사용자 결정: 해외 지표 보조(/stock/metric) + 애널리스트 추천 추이.
	ProviderFinnhubMetrics Provider = "finnhub_metrics"
	// 국내 목표가·투자의견 — m.stock.naver.com(공개 API 아님, Naver API Hub 키 버킷과 별개).
	ProviderNaverStock Provider = "naver_stock"
	// Reddit 종목 언급 수 — ApeWisdom 공개 API(키 없음).
	ProviderApeWisdom Provider = "apewisdom"
	// Reddit 검색 RSS — 종목 커뮤니티 탭의 글 목록(2026-09-29). 키 없음 · IP당 분당 1회(실측).
	ProviderRedditRSS Provider = "reddit_rss"
	ProviderKRX       Provider = "krx"
	// 금감원 「금융상품 한눈에」 Open API — 예·적금·대출 공시(2026-09-29). 하루 한 번 약 60호출.
	ProviderFinlife Provider = "fss_finlife"
	// 자산 확장 공공데이터(2026-10-01) — 배치만. 국토부 실거래가(12종, API마다 일 10,000) · 한국부동산원 R-ONE ·
	// 한국은행 ECOS · 캠코 온비드(API당 일 1,000) · 행안부 법정동코드.
	ProviderMolitRTMS  Provider = "molit_rtms"
	ProviderRone       Provider = "reb_rone"
	ProviderEcos       Provider = "bok_ecos"
	ProviderOnbid      Provider = "kamco_onbid"
	ProviderMoisRegion Provider = "mois_region"
	// 순자산 코인 평가 — 업비트 원화 마켓 전체 시세(2호출). 사용자 수와 무관하게 1분 캐시 뒤에 있다.
	ProviderUpbitBoard Provider = "upbit_board"
	ProviderUpbit      Provider = "upbit"
	ProviderDART       Provider = "dart"
	ProviderEdgar      Provider = "edgar"
	ProviderEdgarBulk  Provider = "edgar_bulk"
	ProviderFnGuide    Provider = "fnguide"
	ProviderNaver      Provider = "naver"

	ProviderKakao Provider = "kakao"
	// ProviderAzureTranslate — 해외 뉴스 제목 EN→KO. 세는 단위가 **문자**다
	// (호출 수가 아니라) — 그쪽 과금이 그렇게 되어 있다.
	ProviderAzureTranslate Provider = "azure_translate"
	// ProviderOpenAI is the shared/legacy LLM bucket — used when KR and US
	// resolve to the same API key. Split keys use ProviderLLMKR / ProviderLLMUS
	// so each Google (or OpenAI) project has its own RPD/TPM counters.
	ProviderOpenAI Provider = "openai"
	ProviderLLMKR  Provider = "llm_kr"
	ProviderLLMUS  Provider = "llm_us"

	/*
		기능별 LLM 한도 (2026-09-27 사용자 결정 C — 「토큰이 많아질 것이므로 사전 조치」).

		키 단위 버킷(llm_kr·llm_us)은 **구글이 세는 것**을 흉내 낸다. 이것들은 그와 별개로
		**우리가 기능마다 떼어 주는 몫**이다 — 새 기능이 하루 한도를 먼저 다 써서 기존
		종목 AI 요약·뉴스 브리핑이 굶는 일을 막는다. 호출은 두 버킷을 **모두** 통과해야 한다
		(키 버킷은 llm.Client가, 기능 버킷은 그 기능의 배치가 본다).
	*/
	ProviderLLMFeatEarnings Provider = "llm_feat_earnings"
	// ProviderLLMFeatSocial — 종목 커뮤니티 탭의 Reddit AI 요약(2026-09-29).
	ProviderLLMFeatSocial Provider = "llm_feat_social"
	// ProviderLLMSub — 부동산·금융상품 AI 전용 키(GEMINI_SUB_API_KEY, 2026-10-01). KR/US 키와 따로 센다.
	ProviderLLMSub Provider = "llm_sub"
	// ProviderLLMFeatAsset — 자산 AI 기능 버킷(금리 한 줄·지역·단지·우대조건).
	ProviderLLMFeatAsset Provider = "llm_feat_asset"
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
	/*
		TokensPerMonth — **한 달** 단위 토큰(또는 문자) 허용량. 0은 무제한이다.

		하루가 아니라 달로 세는 상대가 있어서 생겼다: Azure Translator 무료 등급(F0)은
		**월 200만 자**이고 일일 한도가 없다. 그 상대를 일일 한도로 옮겨 적으면
		(2M/30 ≈ 66K) 뉴스가 몰린 날에 실제로는 남아 있는 한도를 우리가 막는다 —
		「우리 헤드룸이 상대의 잔여와 다른 숫자를 가리키면 상한을 올릴 수 없다」는
		quotaZone 머리글의 규칙이 여기에도 그대로 적용된다.

		번역에서 세는 단위는 토큰이 아니라 **문자**다. 창(월)과 셈(정수 누적)이 같아
		같은 카운터를 쓴다 — RecordTokens에 문자 수를 넘긴다.
	*/
	TokensPerMonth int64
	/*
		SpreadMonthlyTokens — 월 허용량을 **남은 날 수로 나눠** 하루치를 만든다.

		월 한도만 있는 상대의 실패 모양이 나쁘다. 한도를 다 쓰면 남은 날이 통째로
		0이 된다 — 실측(2026-09-13~14): 번역 버그가 이틀에 1.5M자를 태웠고, 그대로
		두면 9월의 남은 열엿새가 전부 영어였다.

		고정된 하루 한도(2M/31 = 64K)로는 **그 상황을 못 고친다.** 이미 1.5M을 쓴
		달에도 하루 64K를 허락하므로 22일쯤 월 벽에 부딪히고, 거기서부터 다시 0이다.

		그래서 하루치를 상수로 두지 않고 **매번 계산**한다:

			오늘치 = (월 한도 − 어제까지 쓴 양) / 오늘 포함 남은 날 수

		평상시에는 2M/31 ≈ 64K와 같은 값이고, 앞에서 과하게 쓴 달에는 저절로 줄어
		남은 날에 고르게 퍼진다(위 사고 뒤 9월 14일 기준 약 30K/일). 반대로 조용한
		달을 보내면 남은 날의 하루치가 늘어난다 — 「몰린 날을 우리가 막는다」는 원래
		걱정도 그만큼 덜어진다.

		TokensPerDay가 함께 있으면 **둘 다** 본다(둘 중 작은 쪽이 실효 한도다).
	*/
	SpreadMonthlyTokens bool
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
		TokensPerMonth  int64 `json:"tokensPerMonth,omitempty"`
	}
	return json.Marshal(alias{
		MonthlyQuota:    l.MonthlyQuota,
		DailyQuota:      l.DailyQuota,
		PerMinute:       l.PerMinute,
		MinIntervalMs:   l.MinInterval.Milliseconds(),
		TokensPerMinute: l.TokensPerMinute,
		TokensPerDay:    l.TokensPerDay,
		TokensPerMonth:  l.TokensPerMonth,
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
	// 지수 ETF 폴러(QQQ·SOXX) 간격과 같아야 한다 — `index_batches.go` finnhub:us 주석.
	ProviderFinnhub: {
		PerMinute:   50,
		MinInterval: 2 * time.Minute,
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
	// FinnhubNewsCompany — 해외 **종목별** 뉴스(/company-news) 전용 버킷.
	//
	// 같은 API 키를 쓰면서 버킷을 나누는 이유는 FinnhubEarnings·FinnhubMigrate와
	// 같다: 앞 버킷의 2분 간격은 **시장 전체 뉴스 한 갈래**를 위한 값인데, 한 회차에
	// 종목 32개를 도는 이 잡은 첫 종목 뒤 31개가 전부 그 간격에 걸린다.
	//
	// 실측(2026-09-13): 한 틱마다 "finnhub_news min interval 2m0s not elapsed" 가
	// 31줄씩 쌓였고, 그 결과 종목 큐 한 바퀴가 8분이 아니라 **7시간**이었다. 로그가
	// 시끄러운 것이 증상이고 병은 「종목별 해외 뉴스가 사실상 안 들어온다」였다.
	//
	// 2초 간격은 Finnhub 무료 등급 실제 상한(분당 60회)의 절반이다. 32종목 × 2초 =
	// 64초라 8분 주기 안에 넉넉히 끝나고, PerMinute 30이 상한을 한 번 더 지킨다.
	ProviderFinnhubNewsCompany: {
		PerMinute:   30,
		MinInterval: 2 * time.Second,
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
	// FinnhubBackfill — 과거 실적 서프라이즈 일회성 백필 전용
	// (cmd/backfill-earnings-surprises → /stock/earnings, 종목당 1호출).
	//
	// 앞의 두 버킷과 나누는 이유가 여기서는 **반대 방향**이다. 캘린더 배치는
	// 하루 수십 호출이라 20초 간격이 아깝지 않지만, 이 잡은 3,000종목을 한 번에
	// 도는지라 20초 간격이면 17시간이다 — 같은 버킷에 두면 잡이 끝나지 않거나
	// 캘린더 배치를 하루 종일 막는다.
	//
	// 간격 1.5초는 분당 40회로 Finnhub 무료 티어 공표 한도 **60/min의 3분의 2**다
	// (docs/api-quotas.md). 상한 45를 정상 속도보다 위에 둔 것은 edgar_bulk와
	// 같은 이유의 안전판이다 — 두 값이 같으면 간격을 지켜도 분당 한도에 걸리고,
	// Check 실패는 그 종목을 통째로 건너뛰게 만든다.
	//
	// 같은 API 키를 쓰는 라이브 경로(ETF 폴러 2회/5분, 실적 라이브 1회/5분)를
	// 합쳐도 피크는 분당 41회라 20회 가까이 남는다. DailyQuota 8000은 3,043종목
	// 한 바퀴(+재시도·재실행)를 덮으면서 폭주 루프는 막는 값이다.
	ProviderFinnhubBackfill: {
		DailyQuota:  8000,
		PerMinute:   45,
		MinInterval: 1500 * time.Millisecond,
	},
	/*
		FinnhubMetrics — 해외 지표·추천 로테이션(종목당 2호출). 같은 키의 다른 경로(회사 뉴스 30/분 ·
		ETF 폴러 · 실적 라이브)와 합쳐 무료 한도 60/분 아래에 두려고 **20/분(3초)**. 하루 5,000은
		로테이션 2,000종목 × 2호출 + 재실행 여유.
	*/
	ProviderFinnhubMetrics: {
		DailyQuota:  5000,
		PerMinute:   25, // 상한 > 정상 속도(3초 = 20/분). 같으면 분 경계에서 걸린다
		MinInterval: 3 * time.Second,
	},
	// NaverStock — 공개 API가 아니라 한도를 스스로 낮게 잡는다(FnGuide와 같은 판단). 국내 전 종목 하루 1바퀴.
	// 상한 60은 정상 속도(1.2초 = 50/분)보다 위 — 같으면 간격을 지켜도 분당 한도에 걸린다(첫 실행에서 74종목이 그랬다).
	ProviderNaverStock: {
		DailyQuota:  4000,
		PerMinute:   60,
		MinInterval: 1200 * time.Millisecond,
	},
	// ApeWisdom — 전 종목 목록 7페이지 남짓을 하루 한 번. 공표 한도가 없어 보수적으로.
	ProviderApeWisdom: {
		// 전체 7 + 서브레딧 9개 ≈ 18페이지 + 종목 페이지 상위 150(2026-09-29) ≈ 하루 170호출 + 재실행
		DailyQuota: 500,
		// 상한은 정상 속도(3초 = 20/분)보다 위에 — 10일 때 서브레딧 8개가 분당 한도에 걸려 비었다(09-29).
		PerMinute:   30,
		MinInterval: 3 * time.Second,
	},
	/*
		Reddit 검색 RSS — 응답 머리의 x-ratelimit가 **분당 1회**를 말한다(2026-09-29 실측: 7초 뒤 두 번째는 429,
		reset은 분 경계). 간격 62초 · 상한은 그보다 위(budget_cap_above_pace). 하루 약 80종목.
	*/
	ProviderRedditRSS: {
		DailyQuota:  300,
		PerMinute:   3,
		MinInterval: 62 * time.Second,
	},
	/*
		금감원 금융상품 Open API — 한도가 명세에 없다. 상품 5종 + 회사 1 × 권역 5 × 페이지 ≈ 하루 60호출.
		1초 간격이면 분당 60이라 상한은 그 위(budget_cap_above_pace).
	*/
	ProviderFinlife: {
		DailyQuota:  1000,
		PerMinute:   90,
		MinInterval: time.Second,
	},
	/*
		국토부 실거래가 — 한도는 **API(종류)마다** 개발계정 일 10,000. 버킷은 12종을 한데 센다:
		하루 상한 60,000은 그 합(12만)보다 낮게 — 전국 12종 첫 적재(아파트 매매 3년 포함 약 4.5만)가 하루에 들어간다
		(2026-10-02). 평소는 하루 약 1만. 간격 0.25초(분당 240)라 분당 상한은 그 위(budget_cap_above_pace).
	*/
	ProviderMolitRTMS: {
		DailyQuota:  60000,
		PerMinute:   300,
		MinInterval: 250 * time.Millisecond,
	},
	// R-ONE — 한도 명시 없음. 하루 수십 호출(지수 표 몇 개 × 지역 전체 1호출).
	ProviderRone: {
		DailyQuota:  2000,
		PerMinute:   90,
		MinInterval: 500 * time.Millisecond,
	},
	// ECOS — 한도 명시 없음. 하루 20여 호출.
	ProviderEcos: {
		DailyQuota:  2000,
		PerMinute:   90,
		MinInterval: 500 * time.Millisecond,
	},
	// 온비드 — 오퍼레이션마다 개발계정 일 1,000. 넷을 한데 세므로 3,500.
	ProviderOnbid: {
		DailyQuota:  3500,
		PerMinute:   120,
		MinInterval: 600 * time.Millisecond,
	},
	// 원화 전체 시세 2호출 × 최대 분당 1회(캐시). 업비트 실제 한도는 초당 10.
	ProviderUpbitBoard: {
		DailyQuota:  4000,
		PerMinute:   6,
		MinInterval: time.Second,
	},
	ProviderMoisRegion: {
		DailyQuota:  200,
		PerMinute:   30,
		MinInterval: time.Second,
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
	/*
		Azure Translator — 해외 제목 EN→KO (2026-09-11 사용자 결정).

		자체 모델(translator 컨테이너)을 걷어 내고 이쪽으로 옮겼다. 무료 등급 **F0은
		월 200만 자**이고 일일 한도가 없다.

		# 하루치를 우리가 스스로 정한 이유 (2026-09-14, 앞의 판단을 뒤집는다)

		처음에는 월 한도만 적었다. 근거는 「일일로 쪼개면 뉴스가 몰린 날에 상대에게
		남아 있는 한도를 우리 쪽이 막는다」였다. 그 말 자체는 맞지만, **월 한도만 있을
		때의 실패 모양**을 보지 않았다.

		실측(2026-09-14): 신규 해외 기사는 하루 429~2,845건이고 제목 평균 68자라
		전량 번역은 하루 약 81K자 = **월 2.5M자**다. 한도가 월 하나뿐이면 25일쯤에
		한도가 닫히고 **남은 엿새는 한 건도 안 옮겨진다.** 「몰린 날 몇 건을 못 옮기는
		것」과 「월말 엿새가 통째로 영어인 것」 중에서는 앞이 낫다 — 목록은 매일 읽히고,
		읽는 사람은 월말에 갑자기 화면이 영어가 되는 것을 「고장」으로 본다.

		그렇다고 **고정 하루 한도**(2M/31 = 64K)를 적지는 않는다. 그것은 이미 앞에서
		과하게 쓴 달을 못 고친다 — 실제로 2026-09-13~14의 번역 버그가 이틀에 1.5M자를
		태웠는데, 64K/일을 계속 허락하면 22일쯤 월 벽에 부딪히고 거기서부터 또 0이다.
		`SpreadMonthlyTokens`는 하루치를 **매번 다시 계산**한다(남은 한도 ÷ 남은 날).
		평상시엔 64K와 같은 값이고, 앞에서 과하게 쓴 달에는 저절로 줄어 남은 날에
		고르게 퍼진다 — 위 사고 다음 날(9/15) 기준 약 30K/일이다.

		나누는 기준은 「**어제까지** 쓴 양」이다. 오늘 쓴 것까지 빼면 하루치가 쓸수록
		줄어들어 끝에 닿지 않는 계단이 된다. 그래서 오늘 이미 과하게 쓴 날은 오늘
		하루만 월 한도(두 번째 벽)가 막고, 내일부터 하루치가 내려간다.

		한도에 걸린 제목은 영어로 남고 **재시도 횟수를 쓰지 않는다** —
		`runUSTranslationBatch`가 「못 보냈다」와 「보냈는데 못 썼다」를 나눈다.

		유료(S1, 100만 자당 $10)로 옮기면 TokensPerMonth를 올리면 된다 — 하루치가
		따라 올라간다. 함께 볼 것: `repository.translationWindowDays`(7일 창)가 수요
		쪽을 줄인다.

		PerMinute 400은 F0의 공표 한도(분당 33,300자 · 초당 요청 수 제한)를 넘지 않게
		잡은 자체 값이다. 한 요청에 제목 여러 건을 싣고(최대 100건) 2분 주기로 도는
		백필이라 실제 분당 호출은 한 자릿수다.
	*/
	ProviderAzureTranslate: {
		TokensPerMonth:      2_000_000,
		SpreadMonthlyTokens: true,
		PerMinute:           400,
	},
	// Papago 항목 셋(papago_text·papago_web·papago)은 2026-09-03에 지웠다.
	ProviderKakao: {
		DailyQuota: 3000,
	},
	// OpenAI-compatible LLM (OpenAI or Gemini via its OpenAI-compat
	// endpoint — see config.buildLLMConfig). Gemini 3.1 Flash-Lite free
	// tier: 15 RPM, 250K TPM, 500 RPD. MinInterval = 60s/15 = 4s.
	//
	// TokensPerDay has no published free-tier figure, so 1M/day is a
	// self-imposed ceiling (~380 AI 요약 calls at ~2.6K tokens). Every
	// value is overridable via LLM_DAILY_QUOTA / LLM_PER_MINUTE /
	// LLM_TOKENS_PER_MINUTE / LLM_TOKENS_PER_DAY.
	//
	// ProviderOpenAI is the shared bucket (one key for both markets).
	// llm_kr / llm_us are used when GEMINI_KR_API_KEY and GEMINI_US_API_KEY
	// (or the OpenAI equivalents) are distinct — Google counts quota per
	// key, so the local counters must not be shared.
	ProviderOpenAI: llmDefaultLimits,
	ProviderLLMKR:  llmDefaultLimits,
	ProviderLLMUS:  llmDefaultLimits,
	ProviderLLMSub: llmDefaultLimits,
	// 실적 관전 포인트 — 국내·해외 합쳐 하루 60콜 · 12만 토큰(콜당 ~2K). 키 버킷(각 500콜·100만
	// 토큰)의 약 6%다. 모자라면 규칙 문장으로 떨어지고 다음 날 이어서 채운다.
	ProviderLLMFeatEarnings: {
		DailyQuota:   60,
		TokensPerDay: 120_000,
	},
	// Reddit AI 요약 — 하루 40종목(콜당 ~2.5K: 글 제목·앞부분 10개 포함) + 수동 재실행 여유.
	ProviderLLMFeatSocial: {
		DailyQuota:   50,
		TokensPerDay: 150_000,
	},
	/*
		자산 AI — 전용 키(llm_sub, 500 RPD·100만 토큰)의 **절반**까지(2026-10-02 사용자: 한도를 넘지 않고 매일 돌 수 있게).
		전국 첫 채우기는 약 350콜(금리 1 · 시군구 ~42 · 단지 ~210 · 우대조건 ~100)이라 이틀에 나눠 찬다.
		그 뒤엔 입력을 반올림해 해시가 「실제로 달라졌을 때」만 다시 불러 하루 수십 콜. 콜당 ~2K.
	*/
	ProviderLLMFeatAsset: {
		DailyQuota:   250,
		TokensPerDay: 600_000,
	},
}

// llmDefaultLimits is Gemini 3.1 Flash-Lite's free-tier request/token
// ceiling, applied to every LLM budget bucket. Split keys each get a full
// copy — they are separate Google projects, not a partition of one 500.
var llmDefaultLimits = Limits{
	DailyQuota:      500,
	PerMinute:       15,
	MinInterval:     4 * time.Second,
	TokensPerMinute: 250_000,
	TokensPerDay:    1_000_000,
}
