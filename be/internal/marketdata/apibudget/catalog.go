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
	ProviderFinlife: {
		Provider: ProviderFinlife, Category: CategoryMarket,
		Label:   "금감원 금융상품 한눈에",
		Purpose: "정기예금·적금·주택담보·전세자금·개인신용대출 공시와 금융회사 홈페이지(finlife Open API). 하루 한 번 약 60호출.",
	},
	ProviderMolitRTMS: {
		Provider: ProviderMolitRTMS, Category: CategoryMarket,
		Label:   "국토부 실거래가",
		Purpose: "아파트·오피스텔·연립·단독·토지·상업·공장 매매와 전월세 실거래(12종). 순자산 부동산 추정가와 부동산 시세 화면.",
	},
	ProviderRone: {
		Provider: ProviderRone, Category: CategoryMarket,
		Label:   "한국부동산원 R-ONE",
		Purpose: "주간·월간 주택가격지수, 공동주택 실거래가격지수, 전월세 전환율, 상업용 임대동향.",
	},
	ProviderEcos: {
		Provider: ProviderEcos, Category: CategoryMarket,
		Label:   "한국은행 ECOS",
		Purpose: "기준금리·국고채·회사채·CD 금리와 예금·주담대 평균금리. 예금·채권 평가와 금리 카드.",
	},
	ProviderOnbid: {
		Provider: ProviderOnbid, Category: CategoryMarket,
		Label:   "캠코 온비드",
		Purpose: "부동산 공매 공고·물건(감정가·최저입찰가·유찰횟수)과 낙찰 결과.",
	},
	ProviderUpbitBoard: {
		Provider: ProviderUpbitBoard, Category: CategoryMarket,
		Label:   "Upbit (보유 코인 평가)",
		Purpose: "순자산의 코인 자산 평가용 원화 마켓 전체 시세. 1분 캐시.",
	},
	ProviderMoisRegion: {
		Provider: ProviderMoisRegion, Category: CategoryMarket,
		Label:   "행안부 법정동코드",
		Purpose: "실거래 조회용 시군구 목록. 주 1회.",
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
	ProviderFinnhubMetrics: {
		Provider: ProviderFinnhubMetrics, Category: CategoryCorp,
		Label:   "Finnhub Metrics",
		Purpose: "해외 지표 보조(PER·PBR·PSR·ROE 등, 본국 상장 기준 비율)와 애널리스트 추천 추이. EDGAR에 없는 값만 채운다.",
	},
	ProviderNaverStock: {
		Provider: ProviderNaverStock, Category: CategoryCorp,
		Label:   "네이버 증권(컨센서스)",
		Purpose: "국내 평균 목표가·투자의견(m.stock.naver.com, 공개 API 아님 — 한도 자체 설정).",
	},
	ProviderApeWisdom: {
		Provider: ProviderApeWisdom, Category: CategoryNews,
		Label:   "ApeWisdom (Reddit)",
		Purpose: "Reddit 종목 언급 수(24시간·전일)·추천 수 + 언급 상위 종목 페이지(감성·언급 사용자·키워드). 키 없음, 하루 한 번.",
	},
	ProviderRedditRSS: {
		Provider: ProviderRedditRSS, Category: CategoryNews,
		Label:   "Reddit 검색 RSS",
		Purpose: "종목 커뮤니티 탭의 이번 주 인기 글(제목·앞부분·링크). 키 없음 · 분당 1회라 하루 약 80종목을 로테이션합니다.",
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
	ProviderLLMFeatEarnings: {
		Provider: ProviderLLMFeatEarnings, Category: CategoryLLM,
		Label:   "LLM 기능 · 실적 관전 포인트",
		Purpose: "향후 7일 실적 발표 종목의 「볼 것」 요약(매일 07:10 배치). 키 버킷과 별개로 이 기능에 떼어 준 몫이라, 다 쓰면 이 기능만 규칙 문장으로 떨어진다.",
	},
	ProviderLLMFeatResearch: {
		Provider: ProviderLLMFeatResearch, Category: CategoryLLM,
		Label:   "LLM 기능 · 리포트 AI 검토",
		Purpose: "공개 리포트의 주장과 데이터 대조 노트(30분마다 새 게시·수정분만). 다 쓰면 이 기능만 규칙 노트로 떨어진다.",
	},
	ProviderLLMSub: {
		Provider: ProviderLLMSub, Category: CategoryLLM,
		Label:   "LLM (자산 전용 키)",
		Purpose: "부동산·금융상품 AI 해설 전용 Gemini 키(GEMINI_SUB_API_KEY). 주식·뉴스 KR/US 키와 따로 센다.",
	},
	ProviderLLMFeatAsset: {
		Provider: ProviderLLMFeatAsset, Category: CategoryLLM,
		Label:   "기능: 자산 AI 해설",
		Purpose: "금리 한 줄·시군구/단지 시장 해설·예적금 우대조건 구조화(asset_ai 잡). 여러 항목을 한 콜에 묶는다.",
	},
	ProviderLLMFeatSocial: {
		Provider: ProviderLLMFeatSocial, Category: CategoryLLM,
		Label:   "LLM 기능 · Reddit 커뮤니티 요약",
		Purpose: "해외 종목 커뮤니티 탭의 AI 핵심 요약(매일 04:20 배치, 언급 상위 40종목). 다 쓰면 이 기능만 규칙 문장으로 떨어진다.",
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
