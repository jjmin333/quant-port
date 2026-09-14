package apibudget

// Category buckets providers for the admin API console.
type Category string

const (
	CategoryMarket      Category = "market"
	CategoryCorp        Category = "corp"
	CategoryNews        Category = "news"
	CategoryTranslation Category = "translation"
	CategoryLLM         Category = "llm"
)

// CategoryLabels maps category ids to display labels for the console sidebar.
var CategoryLabels = map[Category]string{
	CategoryMarket:      "시장 데이터",
	CategoryCorp:        "공시 · 재무",
	CategoryNews:        "뉴스",
	CategoryTranslation: "번역",
	CategoryLLM:         "AI 요약",
}

// CatalogEntry documents one provider for the admin API console: what it is
// used for and where it sits, independent of live usage counters.
type CatalogEntry struct {
	Provider Provider `json:"provider"`
	Label    string   `json:"label"`
	Purpose  string   `json:"purpose"`
	Category Category `json:"category"`
}

// catalog is the single source of truth for provider metadata shown in the
// admin API console. Keep in sync with be/docs/api-quotas.md.
var catalog = map[Provider]CatalogEntry{
	ProviderGoldPrice: {
		Provider: ProviderGoldPrice, Category: CategoryMarket,
		Label:   "GoldPrice",
		Purpose: "금 현물 시세 (지수 패널의 gold 카드).",
	},
	ProviderFRED: {
		Provider: ProviderFRED, Category: CategoryMarket,
		Label:   "FRED",
		Purpose: "나스닥·S&P500·다우·VIX·미 국채 10년 등 미국 시리즈 (batch 6개 = 호출 1회).",
	},
	ProviderFinnhub: {
		Provider: ProviderFinnhub, Category: CategoryMarket,
		Label:   "Finnhub",
		Purpose: "미국 ETF 시세(QQQ, SOXX)와 실적 캘린더.",
	},
	ProviderFinnhubMigrate: {
		Provider: ProviderFinnhubMigrate, Category: CategoryMarket,
		Label:   "Finnhub (카탈로그 배치)",
		Purpose: "미국 ETP 카탈로그 동기화(cmd/sync-etp) 전용. 운영 조회와 별도 budget bucket.",
	},
	ProviderKRX: {
		Provider: ProviderKRX, Category: CategoryMarket,
		Label:   "KRX",
		Purpose: "국내 ETF/ETN 카탈로그(etf_bydd_trd, etn_bydd_trd).",
	},
	ProviderUpbit: {
		Provider: ProviderUpbit, Category: CategoryMarket,
		Label:   "Upbit",
		Purpose: "BTC/KRW 시세와 스파크라인.",
	},
	ProviderDART: {
		Provider: ProviderDART, Category: CategoryCorp,
		Label:   "DART",
		Purpose: "국내 기업 공시·corp code, 내부자거래.",
	},
	ProviderEdgar: {
		Provider: ProviderEdgar, Category: CategoryCorp,
		Label:   "SEC EDGAR",
		Purpose: "미국 기업 XBRL 재무제표.",
	},
	ProviderEdgarBulk: {
		Provider: ProviderEdgarBulk, Category: CategoryCorp,
		Label:   "SEC EDGAR (배치)",
		Purpose: "미국 내부자 거래(Form 4) 적재 배치. 같은 소스, 조회 경로와 분리된 버킷 — SEC 공표 한도 10 req/s의 40%.",
	},
	ProviderFinnhubEarnings: {
		Provider: ProviderFinnhubEarnings, Category: CategoryCorp,
		Label:   "Finnhub Earnings",
		Purpose: "해외 실적 캘린더 배치 (예정일 · 장전/장후 · EPS·매출 추정치와 실적). 같은 키, 라이브 폴러와 분리된 버킷.",
	},
	ProviderFinnhubBackfill: {
		Provider: ProviderFinnhubBackfill, Category: CategoryCorp,
		Label:   "Finnhub Earnings (과거 백필)",
		Purpose: "해외 과거 실적 서프라이즈 일회성 백필(/stock/earnings, 종목당 1호출). 캘린더 배치와 분리된 버킷 — 3,000종목을 한 번에 도느라 간격이 다르다.",
	},
	ProviderFnGuide: {
		Provider: ProviderFnGuide, Category: CategoryCorp,
		Label:   "FnGuide",
		Purpose: "국내 실적발표 캘린더 — 잠정실적 발표일과 컨센서스·발표치. 공개 API가 아니어서 한도는 자체 설정.",
	},
	ProviderNaver: {
		Provider: ProviderNaver, Category: CategoryNews,
		Label:   "Naver API Hub",
		Purpose: "국내 뉴스 키워드 검색.",
	},
	ProviderKakao: {
		Provider: ProviderKakao, Category: CategoryNews,
		Label:   "Kakao",
		Purpose: "국내 뉴스 키워드 검색 (Naver 보조).",
	},
	ProviderFinnhubNews: {
		Provider: ProviderFinnhubNews, Category: CategoryNews,
		Label:   "Finnhub News",
		Purpose: "미국 시장 전체 뉴스(/news). 키 미설정 시 RSS로 대체.",
	},

	ProviderFinnhubNewsCompany: {
		Provider: ProviderFinnhubNewsCompany, Category: CategoryNews,
		Label:   "Finnhub Company News",
		Purpose: "미국 종목별 뉴스(/company-news). 관심·보유 종목을 8분마다 32개씩 돈다.",
	},

	ProviderAzureTranslate: {
		Provider: ProviderAzureTranslate, Category: CategoryTranslation,
		Label:   "Azure Translator",
		Purpose: "해외 뉴스 제목 EN→KO. 세는 단위는 문자이며 무료 등급(F0) 월 200만 자.",
	},

	ProviderOpenAI: {
		Provider: ProviderOpenAI, Category: CategoryLLM,
		Label:   "LLM 공유 키",
		Purpose: "국내·해외가 같은 API 키를 쓸 때의 공유 버킷. 키가 시장별로 나뉘면 llm_kr / llm_us를 본다.",
	},
	ProviderLLMKR: {
		Provider: ProviderLLMKR, Category: CategoryLLM,
		Label:   "LLM 국내",
		Purpose: "국내 종목 AI 요약 + 국내 뉴스 시장 브리핑. GEMINI_KR_API_KEY (없으면 GEMINI_API_KEY). 조회 경로는 LLM 0콜.",
	},
	ProviderLLMUS: {
		Provider: ProviderLLMUS, Category: CategoryLLM,
		Label:   "LLM 해외",
		Purpose: "해외 종목 AI 요약 + 해외 뉴스 시장 브리핑. GEMINI_US_API_KEY (없으면 GEMINI_API_KEY). 조회 경로는 LLM 0콜. 제목 번역은 자체 모델(translator 컨테이너)이라 LLM 예산과 무관하다.",
	},
}

// CatalogFor looks up provider metadata, falling back to the raw provider id
// when a provider has no catalog entry yet.
func CatalogFor(p Provider) CatalogEntry {
	if entry, ok := catalog[p]; ok {
		return entry
	}
	return CatalogEntry{Provider: p, Label: string(p), Purpose: "", Category: ""}
}
