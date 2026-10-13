# Session Archive

- **09-23 (외부 QA 기획서 재검증 — 재현된 P0는 하나였다)** — P0 10건 중 7건이 현재 코드에서 재현되지 않았다. 점수 설명 가능성(`/score/explain`의 축별 기여도·가중치·`computedAt`), 스크리너 결측 깔때기(`PER ≤ 20` 실측 2,630→1,584→1,055, 정책은 제외), Alpha 산식, Playwright 176건이 **이미 있었고**, 로딩 문구 잔존·홈 빈 영역은 하이드레이션 후 실측에서 0건이었다(기획서는 09-20~22 SSR 선반입 이전의 공개 사이트를 보고 쓰였다). **재현된 하나는 진짜였다**: 랭킹은 SSE 실시간 시세를 덮고 스크리너는 안 덮어, 같은 종목이 장중에만 다른 등락률이 됐다. 이 결함은 **스스로를 숨긴다** — 장이 닫힌 02:05 실측에서 두 응답이 소수점 열여섯 자리까지 같았다(073570 · 2095원 · +1.6990291262135921). 고친 모양은 **규칙 한 벌, 적용 여러 곳**: `list_price_overlay.go`(BE)·`list-price-overlay.ts`(FE)가 판정만 하고 두 호출자가 자기 필드 이름으로 적어 넣는다 — 기준가 고르기 한 줄이 갈리면 미국 애프터마켓에서만 한 세션씩 밀리고 그건 눈으로 안 갈린다. 서버만 고치면 첫 응답만 같아지고 화면을 열어 둔 채 몇 분이 지나면 다시 갈리므로 스크리너도 같은 SSE 허브를 구독하게 했다(시장당 연결 하나라 연결 수는 안 는다). `/screener`에 없던 가격 열·기준 시점도 넣었다(문구는 랭킹의 `priceAsOfLabel`을 그대로). 신규 E2E는 **값 비교만으로는 장 마감 중 언제나 통과하므로** `basePrice`·`priceAsOf` 계약의 존재를 함께 건다. ⚠️ E2E 실패 11건은 앱 결함이 아니라 **09-14 「비로그인은 예시를 바로 켠다」 결정 이전을 검사하는 낡은 테스트**다(빈 localStorage로도 예시 모드 · 「예시 끄기」를 누르면 안내 화면 복귀를 실측) — 통과시키려 단언을 바꾸는 것과 구분이 안 되는 자리라 사용자 확인 전까지 손대지 않았다. BE·FE 배포됨 · 미커밋

> 과거 이력. 기본 세션에서는 읽지 않음 — 맥락 필요할 때만.

## Format
`YYYY-MM-DD | 요약 | 결과 | 메모(선택)`

## Log
- 2026-09-17 | 뉴스가 늦게 뜨는 원인 · 수집 차선 3단 분리 + 뉴스 화면 90초 | ok | 읽기 캐시 무혐의(목록 15초·ingest 무효화). 원인은 검색어 큐 225개를 틱당 18개로 한 줄 회전(25분, 실측 평균 723초). news_keyword_lanes.go: hot(7일 50건+, 30개, 4.3분)·warm(10건+·관리자, 33개, 16.5분)·cold(171개, 85분), 차선별 커서. 틱당 22(네이버 일 15,840/24,000). FE NEWS_REFRESH_MS 90초(뉴스 화면만, 홈은 5분). go test·tsc PASS, Docker Tesseract OK
- 2026-08-21 | v0.9.0 뉴스 브리핑 LLM 입력 | ok | 실적 6/10+서버 점수, Toss 수급(국내·5분 캐시), FRED DGS10 홈 티커, 이력 3+watchPoints, 헤드라인 재정렬. go test/vet/build, Docker Tesseract OK, /quotes/indices us10y=4.65. 릴리스 `docs/releases/v0/v0.9/v0.9.0.md`, fe 0.9.0
- 2026-08-21 | 해외 과거 예정 숨김 | ok | DropStaleOverseasScheduled(US scheduled < KST 어제). 적재는 유지. BATCH_SCHEDULE earnings_overseas 메모·절. go test·Docker
- 2026-08-21 | 해외 과거 예정 숨김 방향 | ok | 어제까지는 예정 유지. 그 이전 예정은 조회에서 제외(배지 신설 비추천). DB는 유지. 미구현
- 2026-08-21 | 해외 실적일 KST 이동 여부 | ok | DB 날짜 롤오버 하지 않음. 23:55+실적없음은 AMC 미발표·소형주 공백과 구분 불가. 저장=미국 세션일
- 2026-08-21 | 해외 실적일 TZ | ok | Finnhub date=미국 세션 달력일. KST 환산 없음. KST는 조회 창·배치 시각만
- 2026-08-21 | 해외 실적 TGT 중복 제거 | ok | Finnhub가 같은 발표를 연도만 다르게 2행. 적재 merge + 조회 CollapseOverseasFiscalYearDup. 2Q/FY는 유지
- 2026-08-21 | 발표 후에도 실적 시각 없음 | ok | Finnhub·earnings_events 모두 날짜+세션만. 시계 시각 컬럼 없음
- 2026-08-21 | Finnhub 실적 실시간·시각 여부 | ok | 5분은 Finnhub 반영 후 지연. 시계 시각 없음(bmo/amc만). NVDA 11/17 amc
- 2026-08-21 | 해외 실적 표 매출 짝·시총 정렬 | ok | 매출을 실적/예측으로. 기본 정렬 시가총액 내림차순
- 2026-08-21 | 중대형주 Finnhub 실적 누락 여부 | ok | 있음. ROST·PS 대형, XP·VIPS·BZ 중형. 소형만의 문제는 아님. 날짜 오차·미기입이 원인
- 2026-08-21 | Nasdaq·EDGAR·AV 실적 필드 대조 | ok | Nasdaq EPS만(매출·시각 거의 없음). EDGAR 실적만. AV는 엔드포인트마다 조각. Finnhub 3종을 한 곳에서 다 주는 무료 소스는 없음
- 2026-08-21 | Finnhub 소형주 공백·무료 차선 확인 | ok | Nasdaq 캘린더가 4종 실제일+EPS 있음. SEC 8-K 2.02도 있음. AV 25/일 불가
- 2026-08-21 | Finnhub 4종 Q2 실적 전 엔드포인트 확인 | ok | 캘린더 실제일(8/11–13) 0건·8/19 null. earnings·financials-reported도 Q1까지. 과거 분기는 있음
- 2026-08-21 | 스크린샷 해외 실적 8종 실제 발표일 | ok | 예정 4종은 8/19 아님(AARD 8/11, ADGM 8/11, ACRV 8/12, ALMS 8/13). 확정 4종(ADI·ANTA·ALVO·ARAY)은 8/19 맞음. Finnhub 추정일 잔류
- 2026-08-21 | 해외 실적 과거 예정 원인 | ok | 적재 실패 아님. 예정=실적 없음(의도). Finnhub 캘린더가 추정일을 안 지움(AARD 8/11 발표·8/19 잔류). 라이브 창은 어제~오늘만
- 2026-08-20 | 배치 시간표 메뉴 열 | ok | 시각과 잡 사이에 사이드바/종목탭 이름. 운영 중 관리자 화면에서 수정 가능
- 2026-08-20 | 배치 시간표 메모 열 + 관리자 편집 | ok | BATCH_SCHEDULE.md 표 우측 메모. PUT /admin/batch/schedule이 디스크 파일 저장, embed 폴백. Docker 바인드 마운트. 배치 관리 화면에서 수정/저장
- 2026-08-20 | 국내 실적 배치 주기·적재 확인 | ok | 시즌 10–20시/2h 실측(오늘 10·12·14·16 성공 273행). 18시 발표는 18시 슬롯 레이스면 20시. 저장은 upsert 정상, 오늘 updated_at 0은 소스 무변경. 캘린더 FE 캐시 5분
- 2026-08-20 | 토스 계좌 연동 FE/BE 제거 | ok | 시세 Toss 유지. 연동 OAuth·포트폴리오 동기화·인사이트 LLM 스케줄러·주문 API 삭제. 폴러/뉴스는 관심+수동보유. 마이그레이션 054. extendedMigrations에서 007 재생성 제거. 옛 URL `/` 리다이렉트. go test PASS, fe tsc PASS, Docker healthy
- 2026-08-20 | 실적 홈 카드·종목 탭 설계안 | ok | 적재 공백 vs 미표시 분리. 국내 예정일은 소스 없음(추정 유지). 해외 이력 45일·탭이 hit/reaction 미표시·홈이 거래대금 기준이 본 문제. 유료 소스 보류
- 2026-08-20 | LLM 사용처·일 처리량 확인 | ok | 종목요약·뉴스브리핑·홈 포트폴리오브리핑. 지수 규칙 0콜. 오늘 272콜=종목~210+브리핑52. 번역 Papago 687
- 2026-08-20 | Gemini 503 1회 재시도 + 번역 Papago 전용 | ok | 공식 503≠쿼터, Record는 2xx만. 재시도 1회/4s. 폴백 제거, papago_text 20→60/min, 일 6000 유지. 유닛+Docker Tesseract OK
- 2026-09-10 | 시장 브리핑 국내·해외 LLM dry-run 1회씩(저장 없음) | ok | `cmd/dry-run-news-briefing all` · domestic 3833tok · us 4242tok · rawCompletion·storedFields 전달 · KR 13:30·US 13:00 generatedAt 미변경
- 2026-09-10 | SK하이닉스 AI 요약 LLM dry-run 1회(저장 없음) | ok | `cmd/dry-run-stock-summary`+`DryRunLLM`·`GenerateStockSummaryRaw` 신설. 000660 재생성 원본·필드별 저장값·기존 DB 대조 제공. analyzedAt·overview 미변경 확인
- 2026-08-20 | 이전 AI 요약 실패 10건 재처리 없이 판정 | ok | JSON 꼬리 2건만 신파서 대상. completion 미저장이라 과거 기록은 그대로. 503 7·간격 1은 무관
- 2026-08-20 | LLM JSON 파싱 꼬리 `}` 복구 | ok | Unmarshal→Decoder 첫 값. 펜스·선행문구 스킵. 요약·브리핑 공통. 재호출 없음. 유닛+Docker Tesseract OK
- 2026-08-20 | AI 요약 수동 배치 실패 종목 원인 분석 | ok | Redis run KR:2026-08-20(105ok/3fail)·US:2026-08-19(107ok/7fail). 503 7 · JSON `}` 2 · min-interval 1. 재시도·carry-over 없음. 코드 미수정
- 2026-08-19 | 국내·해외 실적 배치 동시 실행 | ok | FnGuide/Finnhub 버킷 분리라 병렬 OK. worker-earnings가 catch-up·스케줄을 시장별 고루틴으로 동시 기동
- 2026-08-19 | LLM 현재 차단 여부 확인 | ok | 지금 미차단. 아침만 PDT 전날 500. KST 오늘 174, blocked_until 없음, 22:30 브리핑 성공
- 2026-08-19 | LLM 일 500 소비 내역 | ok | 지수 0콜. 랭킹 KR+US≈200. 뉴스 LLM은 브리핑 2시장. 초과 주범은 US 제목 번역 Papago 초과분. 8/18 openai 500 / papago 3631. PDT 하루에 저녁 US+밤새 번역+아침 KR이 겹침. 미수정
- 2026-08-19 | LLM 핵심 요약이 데이터를 못 내는 원인 조사 | ok | Gemini 무료 500 RPD 소진 + KST/PDT 일자 불일치. 오늘 KR 1/106 llm, US 92/96. 번역·브리핑 동일 버킷. thinking JSON 파싱·타임아웃·실적/내부자 facts 공백은 부수. 미수정
- 2026-08-19 | 펀더멘털 배치 규모·실적/재무 탭 소스 확인 | ok | 9,993행, 미국 7,121은 07-20 시드(4h46m). 24h 폴러 미실행. 실적분기·재무차트는 온디맨드 companyfacts
- 2026-08-19 | EDGAR 사용처 전수·개선안 | ok | companyfacts 3화면 중복, 내부자만 Form4 XML. 8/min 자체예산. 제안: companyfacts 공유캐시 + Table I 스킵 + 빈배열 미캐시 + 예산 상향
- 2026-08-19 | EDGAR 8/min은 자체 예산, SEC는 10req/s | ok | 공식 fair access 초당 10. Form 4 3건 상한은 우리 apibudget 때문
- 2026-08-19 | CRWV 내부자 공백 원인 조사 | ok | SEC Form 4 431건 존재. 폴링 실패 아님(온디맨드). 최근 3건이 Table II만이라 빈 배열 24h 캐시. 4번째 신고에 Class A 매도 24건
- 2026-08-19 | AI 추천 랭킹 실시간 시세 일봉 모달 | ok | 「실시간 시세」클릭이 `/market` 이동 대신 모달. `/quotes` 개요 일봉을 `QuoteDailyChart`로 추출·공유. 열 때만 시세·캔들 구독. tsc PASS
- 2026-08-19 | AI 추천 랭킹 기본 종목 삼성전자 고정 | ok | `/analysis` URL에 symbol 없을 때 포트폴리오 최대 보유(예: ARM) 대신 005930 고정. `analysis/page.tsx`에서 fetchPortfolioSummary 제거
- 2026-08-18 | 실적 화면 정렬 테이블 · 성과형 달력 · 레일 교체 | ok | 카드행→9열 정렬 표(칩 제거, 글자 표기), 신설 열 시가총액(shares×종가, 커버리지 100%)·매출 서프라이즈(93~95%)·현재가(배치 1회). 달력 칸 색 = 그날 상회 비율(발표 전은 중립). 레일을 대형 발표·엇갈린 발표(증권사 예측→실제 명시)·꾸준한 기업·내 종목 최근 발표(−2주~+1달)로 교체, 상회/하회 TOP 제거. `/earnings/highlights` 신설(1요청·Redis 30분), 심볼 필터 시 구간 상한 31→120일. 실측 교정 2건: 국내 R1이 시즌 사이라 빈 카드 → 100일 폴백(75일은 하루 차이로 놓침), 캐시 v1이 옛 결과를 계속 반환 → v2. go/tsc/eslint/next build PASS, 브라우저 DOM 실측 완료
- 2026-08-18 | 실적 메뉴 국내·해외 분리 + 위젯 4종 | ok | 사이드바 2항목(`/earnings/domestic`·`/earnings/us`), W1 발표 후 주가 반응(세션별 종가 쌍 + 인접성 가드 — 목업 초안의 「+28.2%」가 커버리지 공백 때문에 7일치였음을 실측으로 교정), W2 실적 전용 모달(분기 추이 기대vs발표 막대·적중 이력·개별 접기), W3 스코어보드(평균·중위 병기 — 08-06 중위 +4.3% vs 평균 −3.3%), W4 해외 장전·장후 타임라인, 월/주/일 토글(useSyncExternalStore), 기본 날짜 자동 이동. **`CollapseDomesticBasis` 중복 구현 정리** — 둘이 연달아 돌아 「개별 접기」가 죽은 코드였던 것을 하나로 합침. 보류: 모달 관련 뉴스(symbols_json LIKE 0.83초). go/tsc/eslint/next build PASS, 브라우저 DOM 실측 완료(스크린샷은 패널 미표시로 불가)
- 2026-08-17 | 실적 메뉴 신설 — 시장 전체 실적 캘린더 + 컨센서스·서프라이즈 | ok | `earnings_events`(051) + FnGuide 국내 잠정실적·Finnhub 해외 캘린더 적재(worker-earnings), `/earnings` 화면(월 히트맵·일자 목록·서프라이즈 랭킹), 국내 예정일은 작년 동기 추정. 실측으로 5건 교정: FnGuide 억원 단위(DART 대조 6자리 일치)·`_G`=QoQ·`FREQ=D`=연간, `"적전"` 라벨로 한 달 전체 파싱 실패, Finnhub 1500행 상한(앞부분 소실 → 7일 조각), 서프라이즈 유의성 하한, 지난 추정 억제. go/tsc/eslint/next build PASS, DOM·네트워크 실측 완료(스크린샷은 Browser 패널 미표시로 불가)
- 2026-08-17 | 홈 오늘의 뉴스 행 높이 축소 | ok | LatestNewsCard: 언론사 제거, 종목·지수 칩을 메타 줄로 이동(브리핑 카드와 높이 정렬)
- 2026-08-15 | 총 자산 카드 텍스트 전용 재디자인 | ok | Metric 칸(테두리·회색 배경) 제거, 흰 카드+타이포만, 원화·달러 현금 동일 크기. eslint·tsc PASS
- 2026-08-15 | 종목 모달 통일·헤더 검색 전역화·뉴스 레일 정리 | ok | StockQuickLookModal 공용화(관심·보유 래퍼), CTA→/analysis?tab=ai, 헤더 검색 stock 기본 모드+자동완성, 기사 모달 닫힘 버그(목록 임시 close+스크롤 복원), 뉴스 25→15건·8→10종목, 총자산 원/달러 분리. tsc·build·eslint PASS, 검색 실측 /analysis?symbol=102780&tab=ai
- 2026-08-15 | 내 포트폴리오·목표가 UI 수정 | ok | 뉴스 기사 모달 z겹침, 일괄등록 drafts 미초기화, 현금 Metric·일괄추가 이동, 목표가 localStorage 문구 삭제. tsc PASS
- 2026-08-15 | 매매 이력 가격 불일치 정리(2차) | ok | trade_date 당일 종가 없을 때 직전 종가 fallback. ARM 등 8종목 16건 추가 삭제. 총 32건
- 2026-08-15 | 매매 이력 가격 불일치 정리 | ok | trade_date 종가≠단가 8종목 holding_trades 16건 삭제, source manual 복원. MigrateHoldingTradesPriceMismatchCleanupIfNeeded
- 2026-08-15 | 메뉴 IA·내 포트폴리오·목표가·일괄등록 | ok | 사이드바 순위/내투자/커뮤니티, 뉴스모달 필터·기사팝업, bulk-register API+dedup, 총자산 현금 통합·일괄추가, user_price_targets API. go build·tsc·Docker PASS
- 2026-08-14 | 보유종목 탭 UI·기능 개선 | ok | 현금→종목 추가 우상단 다이얼로그·총자산 카드 통합, 평가금액 추이 점수 하단+일괄 등록, 뉴스 더보기 모달, 목표가 빠른 추가·종목명 검색, localStorage만 확인. tsc PASS
- 2026-08-13 | 매매 기록 입력 UX | ok | TradeDateField(오늘/어제·한글 표시), 첫 기록 시 수량·평단 자동 채움, tsc PASS
- 2026-08-13 | 3장 스크린샷 LG전자·phantom 행 | ok | mixed locale(원+$), 16전자→LG전자, ARM qty OCR, garbage filter, Docker OCR 3-fixture PASS
- 2026-08-13 | 스크린샷 OCR 평단 XOVR·SK하이닉스 | ok | materiality·garbage return PnL·KRW/USD 추론·satellite finance·SK 이름 보정, Docker OCR fixture PASS
- 2026-08-13 | 뉴스 RSS 확장·중복 방지·지수 태그 | ok | 국내 피드 7→8(크립토 3·연합 경제), 더팩트 제거, domestic headline dedupe, index:kospi 등 태그, 040 키워드 seed, go test PASS
- 2026-08-13 | 시세/관심종목 OCR name-only | ok | quote list parser, qty/avg empty, overseas_watchlist_quotes.png fixture
- 2026-08-13 | Kiwoom 삼성전자 OCR 평단 5원 + phantom 제거 | ok | avg 359260, broker/latin debris filter, domestic_kiwoom_samsung.png fixture, Docker PASS
- 2026-08-13 | Kiwoom ETF OCR 평단가 (KODEX/TIGER) | ok | KODEX 21476·TIGER 52094, inline finance·finance column pass·mixed PnL parser·fragment fix, Docker Tesseract PASS
- 2026-08-13 | OCR 평단가 SDI/NAVER 부호·스냅 보정 | ok | SDI 485500·NAVER 260000·삼성전자 364500, reconcileParsedFinance, import 테스트 PASS
- 2026-08-13 | OCR 평단가 dot-thousands/~손실 파싱 | ok | 1.852.010원→1852010, 삼성전자 avg 364500, import 테스트 PASS
- 2026-08-13 | OCR resolver 삼성5미/16전자 live catalog 매칭 | ok | fuzzy subsequence tail + prefix 전용 경로, MySQL 실측 006400/066570, import 테스트 PASS
- 2026-08-13 | 보유종목 OCR 이상 종목 제거 + 7종목 (하드코딩 제거) | ok | TEADS/ALDEL/HAVER 등 phantom 제거, locale/fragment 필터·CollapseSimilarRows·orphan qty·좌측 upscale-only, Docker Tesseract 7종목 PASS
- 2026-08-13 | 보유종목 스크린샷 OCR 정확도 개선 (다중 패스 + 파서/퍼지) | ok | 사용자 테스트 이미지 7종목 exact match. pipeline 다중 OCR+다수결, ocr_fix(삼성5미/16전자/a→4주), testdata/fixture+Docker 통합테스트 PASS
- 2026-08-10 | 리서치 콜 스탬프 결합 해제 + Phase 2~3(리더보드·반응·댓글·3축 평가) | ok | 스냅샷 타입 리서치 소유로 분리 + close_series.go 중립 추출, 038 소셜 테이블, 알파 기준 리더보드(연율화·클램프·빈도/반복 페널티·최소표본 10), 유닛 14건, 브라우저+픽스처 실측
- 2026-08-09 | 리서치 콜 Phase 1 구현 (037 + ResearchService + worker-research + /research 화면 3종) | ok | 실행 일치도(계좌 연동) 제외 확정. 불변 콜값·알파 판정·실측 도달확률·자동 초안(LLM 0콜). 유닛 24건, 브라우저 실발행 + 판정 배치 실측. 관리자 화면은 자격증명 없어 미확인
- 2026-08-09 | 리서치 콜(공개 종목 논증 + 검증 가능 예측) 설계안 작성 | ok | docs/RESEARCH_CALL_DESIGN.md 신규. Valley AI 벤치마크 분해 → 알파 채점·불변 콜값·실행 일치도·룰기반 자동 초안으로 차별화. 코드 변경 없음
- 2026-08-09 | AI 요약 배치 수동 실행이 휴장일에도 종가 기준으로 돌도록 수정 | ok | batchRunWindow.asOf + calendar.LastSessionCloseAt로 휴장일 가드 우회, 유닛 1건, 관리자 로그인 없어 브라우저 클릭 미확인
- 2026-08-09 | AI 요약 배치를 배치 관리에서 수동 실행 가능하게 연동 | ok | 신규 잡 ID 2개(국내/해외), RunBatchNow + 시장별 동시실행 가드, API 프로세스 내 큐 워커, 유닛 5건, 관리자 로그인 없어 브라우저 클릭 미확인
- 2026-07-17 | agent 진행 기록 체계 구축 | ok | AGENT_PROGRESS.md 생성, AGENTS.md 규칙 추가
- 2026-07-17 | dashboard insights 3컴포넌트 전체 구현 | ok | history API, risk rules, briefing+LLM cache, FE 연동
- 2026-07-18 | FE stub 11페이지 API 연동 | ok | risk/portfolio/calendar/disclosures/financials/news/ai-analysis/transactions/journal/targets/backtest
- 2026-07-18 | Quotes market data Phase 0–4 | ok | Redis+Poller+SSE, candle_bars base-only, AGENT_STATE 갱신
- 2026-07-25 | Quotes 장 마감 DB-first 정합 | ok | shouldLivePollSymbol, loadLatestCandlesFromDB, 과거 DB-only, AGENT_STATE canonical 정책
- 2026-07-25 | Quotes cold-start backfill | ok | backfillClosedMarketFromToss, price last-good; MySQL persist TBD
- 2026-07-25 | MySQL terminal persist (011) | ok | quote_snapshots, orderbook, trade_ticks, symbol_last_prices; MySQL+Redis write-through
- 2026-07-30 | StockChartTerminal component | ok | chart/trades/orderbook extracted from quotes-detail-panel for stock-workspace reuse
- 2026-08-03 | KOSPI assetType + fundamentals repair | ok | domestic mis-tag caused prev-close quotes + EDGAR overwrite; repair on startup
- 2026-08-03 | GET /quotes KOSPI live price | ok | shouldLivePollSymbol+finalizeQuoteDetail use quoteAssetType; rehydrate with trades/book/ranking
- 2026-08-03 | Single price resolver (BE) | ok | ResolveLiveQuotePrice; rankings enrich; prevClose guard; no completed-daily in live session
- 2026-08-03 | BE-only prices + FE cleanup | ok | newestTradePrice; 1d candle live patch; removed FE batch/applyLiveQuote guards
- 2026-08-03 | Continuous Toss polling | ok | shouldLivePollSymbol session-agnostic; poller always refreshes when Toss on
- 2026-08-03 | Admin Redis cache management | ok | BE cache_admin + admin/cache UI; list/view/edit/delete with namespace filters
- 2026-08-04 | Redis terminal cache unify | ok | envelope single-key; stale hot/:last divergence fix; market price + rankings v1 cleanup
- 2026-08-04 | quotes:detail Redis 제거 | ok | detail=terminal(1s)+MySQL; startup purge 81 keys; ensureQuoteDetail→refreshTerminalSnapshot
- 2026-08-05 | 성능/구조 리팩토링 (BE+FE) | ok | poller 겹침방지+동시성제한+active/pinned 분리; fxForSymbol 제거(Toss 5→3콜); candle multi-row upsert; HasTable/bar_time 타입 캐싱; withOverlayCache; pool 50/25; graceful shutdown; StartPolling 분리; singleflight; 뉴스티커 stagger; FE 차트 dynamic import + apiRequest 공통화 + 공백깨짐 2파일 복구
- 2026-08-05 | DB 최적화 + 관리자 DB 콘솔 | ok | candle_bars 재구축(행단위→단일패스, 30분+ 미완료→27초, 517MB→196MB); 중복 PK 인덱스 제거; ranking 90일 보관정책; /admin/database (테이블 용도 카탈로그 + 화이트리스트 기반 행 CRUD, 민감컬럼 마스킹); FE 레이아웃 이슈는 dev 서버 CSS 캐시가 원인(소스 무결)
- 2026-08-06 | v0.4.0: 헤더/차트 시세 정합 + Toss 429 구조 개선 | ok | 가격 이중계산 제거(틱 내 공유)+forming봉 배열순서 의존 제거+FE REST 폴백; candles 델타페치(36페이지→1)+통계 DB-first로 429 94%→소수; pinned 미조회 종목 호가/체결 갱신 제거로 trades/orderbook 75/59→0; Retry-After 실값 사용; BE 전용, CSS 미변경 → docs/releases/v0/v0.4/v0.4.0.md
- 2026-08-06 | 랭킹 5분 벽시계 정렬 폴링 + 기업정보 애니메이션 제거 + UI회귀 조치 + 설계안 2건 | ok | realtime 랭킹 1분→5분(Truncate 기반 00:00/00:05 정렬), DB 기존값 마이그레이션; 매출·산업구성 파이차트 애니메이션 끔; 모바일 풀스크린 회귀는 소스 정상 확인 후 dev 캐시 재기동(육안 미확인, 사용자 확인 요청); 랭킹 실시간가격 오버레이 설계안 + 실적/전망/내부자거래 무료API 설계안(DART elestock.json으로 내부자거래 완전 커버 확인) 제시, 미구현
- 2026-08-06 | 마켓 레이더 "1분 주기" 표시 오류 수정 + 관리자 API 관리 신설 | ok | 표시 텍스트가 BE 5분 설정과 무관하게 하드코딩("1분 주기")된 것이 원인 — rankingRefreshHint에 useMarketDataSettings 실측값 전달, DEFAULTS 폴백도 5로 정정 (실제 폴링 주기 자체는 이전부터 정상, 표시 텍스트만 어긋났었음); 신규 `/admin/api` — apibudget에 provider별 용도/카테고리 카탈로그(catalog.go) 추가, Limits에 커스텀 MarshalJSON(ms 단위) 추가, AdminHandler.APIQuotas가 카탈로그+실시간 사용량(분/일/월, 마지막 호출, 차단 상태) 병합 응답, FE는 카테고리별 카드 대시보드 + 사이드바 "API 관리" 메뉴 추가, 기존 관리자 홈의 인라인 "API 할당량" 표는 제거하고 링크로 대체; go build/tsc/eslint/next build 통과, Docker 재기동 후 public 헬스 엔드포인트로 JSON 직렬화 실측 확인 (관리자 세션으로 화면 육안 확인은 미완료 — DB admin 권한 부여가 auto-classifier에 차단됨, 사용자 확인 필요)
- 2026-08-07 | v0.5.0: 실적 탭 구현 | ok | 신규 `GET /stocks/{symbol}/earnings`(국내 DART 잠정실적/정기보고서 필터+reprt_code 4종 분기, 미국 Finnhub 일정+EDGAR 최근 5기간, Redis 24h 캐시, DB 스키마 변경 없음); 구현 중 발견한 버그 2건 수정 — (1) DART curl TLS 폴백이 curl.exe+Windows 게이트라 Docker(Linux)에서 항상 스킵되어 DART 실시간 호출 전반이 조용히 실패하던 것(dart_http.go OS 게이트 제거), (2) EDGAR 분기 추출이 3개월 단일분기와 6/9개월 YTD 누적을 구분 못해 애플 Q2/Q3 매출이 $254B/$364B로 부풀려지던 것(xbrl_select.go에 60~100일 표준분기 길이 검증 추가, 수정 후 $109~144B 정상 범위); FE `EarningsTab` 신규 컴포넌트, 워크스페이스+레거시 /analysis 양쪽 배선; go build/vet, tsc/eslint/next build, 005930·AAPL·MSFT 실측, 데스크톱·모바일 브라우저 확인 완료 → `docs/releases/v0/v0.5/v0.5.0.md`
- 2026-08-07 | 재무제표 탭 상세 모달(토스 스타일) + 실데이터 다분기 히스토리 | ok | 신규 `GET /stocks/{symbol}/financials/history?years=N`: 국내 DART 1~3분기 thstrm_amount(단독분기 실측 확인) 직접 + 4분기는 연간−9개월누적 역산, 재무상태표는 시점값 그대로; 미국 EDGAR는 기존 분기 함수에 BS concept 추출만 추가(API 비용 0). 라이브 검증 중 버그 3건 발견·수정 — DART 반기 순이익 계정명("반기순이익")이 폴백 목록에 없어 누락(계정명 매칭을 접미사 허용 방식으로 일반화), SEC XBRL 동일 분기가 필터마다 다른 fy 태그로 중복 보고되어 AAPL 실측에서 실제로 중복 발생(매칭 키에서 FY 제거, FP+End만 사용), BS 항목에 분기 duration 검증이 잘못 걸리지 않도록 별도 instant 매처 분리; FE `FinancialDetailModal`(수익성/성장성/안정성 3-preset, bar+line 콤보차트+표+연도토글+헤드라인) 신규, 기존 가짜 배수 차트 데이터를 실제 히스토리로 교체; go build/vet·tsc 통과, 005930·AAPL 브라우저 클릭 플로우(모달 열기→연도토글→닫기) 데스크톱 확인, 모바일은 카드 렌더만 확인(모달 클릭은 이 환경 도구 제약으로 미확인). 버전 릴리스 문서화는 사용자 요청 없어 보류(다음 요청 시 v0.6.0)
- 2026-08-07 | 전망 탭 구현 + DART/EDGAR 최신성 비교 + 마켓 레이더 탭 유지 버그 | ok | 전망 탭은 BE 변경 없이 기존 엔드포인트(financials/history, earnings의 calendar) 재사용해 실적추세·밸류에이션(PER/PBR+PSR 신규 FE 계산)·공시일정·컨센서스 안내 4섹션 구성(`CalendarSection`을 earnings-tab.tsx에서 export해 공유), DART 잠정실적 본문 파싱(스펙 4번)은 자유형식이라 범위 밖 명시; DART/EDGAR 최신성 비교는 SEC company_tickers.json 실측으로 실제 20-F 파일러 7종목만 매핑(SK텔레콤·KB금융·신한지주·우리금융지주·POSCO홀딩스·한국전력·LG디스플레이 — 삼성전자·SK하이닉스는 무보증 ADR이라 매핑 대상 아님, 사용자 예시가 원래 해당 없는 케이스였음을 확인), PeriodEnd 비교로 EDGAR가 진짜 더 최신일 때만 교체(20-F가 연간뿐이라 DART가 구조적으로 항상 이김, 실측 확인); 부수로 FetchAnnualFinancialSnapshots가 PeriodEnd를 아예 안 채우던 걸 발견해 calendarQuarterEnd 재사용해 채움; 마켓 레이더 탭 리셋 버그는 market/page.tsx의 selectSymbol/URL-sync 두 곳 강제 setWorkspaceTab("overview") 제거로 해결, 헤더 검색으로 종목 전환 브라우저 테스트 확인; go build/vet, tsc/eslint/next build, cmd/seed-fundamentals 로컬 실행으로 017670·005930 실측, 브라우저로 전망 탭 렌더 + 탭 유지 확인
- 2026-08-07 | Gemini 실측 한도 반영(RPM15/RPD500/TPM250K) + 요청 기록 종목명 표시 | ok | 사용자가 Gemini 3.1 Flash-Lite 공식 무료 티어 한도 직접 제공 — apibudget 값을 정확한 수치로 교체(DailyQuota:500, PerMinute:15, TokensPerMinute:250000, 기존은 서드파티 집계 사이트 기반 추정치였음); TPM 실측 추적 신규 구현 — apibudget.Manager에 분당 토큰 카운터(RecordTokens, Redis IncrBy+2분TTL) 추가, Check()가 이번 분 누적 토큰이 한도 이상이면 차단(사후 누적 기준), llm/client.go가 OpenAI 호환 응답의 usage.total_tokens를 실제 파싱해 기록(실측: 2회 호출로 14,771 토큰 기록 확인); /admin/llm·/admin/api 양쪽에 TPM 사용량 바 추가; 요청 INPUT 기록에 종목명 필드 추가(로깅 시점 stock.Name 그대로 사용) + /admin/llm에 종목명 드롭다운 필터, 로그 항목도 종목명 우선 표시(코드는 보조 표기)로 수정; 사이드바 버그는 사용자가 재보고했으나 실행 중 dev 서버의 실제 컴파일된 JS 번들을 직접 열어 검사해 수정 코드가 정상 반영·기존 버그 패턴 부재 확인, 브라우저 캐시 문제로 판단해 강력 새로고침 요청. go build/vet/test, Docker 재기동 후 005930·000660 재검증(TPM/종목명/한도값 모두 확인), tsc/eslint/next build. 버전 릴리스는 미진행(요청 시 v0.8.0 이후 누적분으로 문서화)
- 2026-08-07 | v0.8.0: 사이드바 활성 탭 버그 수정 + LLM 모델 관리 상세화(호출 주기·API 한도·요청 INPUT 기록) | ok | 사이드바 버그 — /admin이 다른 모든 관리자 페이지의 접두사라 특수 케이스 때문에 항상 "뉴스 관리"도 활성으로 보이던 것을 특수 케이스 제거로 수정; llm_settings에 refresh_interval_hours 컬럼 추가(1~24h, columnExists 기반 마이그레이션), time.Ticker를 매 주기 재조회하는 sleep 루프로 전환해 관리자가 런타임에 주기 변경 가능; /admin/llm에 openai provider 호출량 그래프 인라인 표시; 신규 요청 INPUT 기록 — GenerateStockSummary 호출마다 factsJSON+성공여부를 Redis 캡드 리스트(llm:requests:v1:log, 최근30건·7일TTL)에 기록, GET /admin/llm-requests + 펼쳐보기 UI, Redis LRANGE로 실제 factsJSON 기록 확인. 버그 재발견+재수정 — 재검증 중 스케줄러 파일럿 종목 간격 문제(005930 llm/000660 rule)가 재현되어 6초 간격 추가로 재확인. go build/vet/test, Docker 재기동 실측, 관리자 엔드포인트 401 확인, tsc/eslint/next build, /admin/llm 미인증 리다이렉트 확인. 관리자 로그인 화면 육안 확인은 이전 세션들과 동일 사유로 미완료. v0.7.0 이후 누적분(관리자 메뉴 개편 포함) 전부 묶어 문서화, README 인덱스 갱신, fe/package.json 0.7.0→0.8.0 → `docs/releases/v0/v0.8/v0.8.0.md`
- 2026-08-07 | 관리자 메뉴 개편 — LLM 모델 관리 신설 + 뉴스/데이터베이스 관리 명칭 정리 | ok | 파일럿 종목을 env에서 DB 설정으로 전환(신규 llm_settings 테이블, market_data_settings와 동일 패턴, LLMSettingsStore가 요청마다 즉시 재조회해 관리자 편집이 재기동 없이 반영); 신규 /admin/llm(LLM 모델 관리) — provider 상태 + 파일럿 종목 편집 폼(쉼표구분, 최대10개) + 종목별 캐시 상태, /admin/api에서는 파일럿 섹션 제거하고 원래 목적(provider별 호출량/한도 그리드)만 유지; 사이드바 "관리자"→"뉴스 관리", "데이터베이스"→"데이터베이스 관리"로 명칭 정리. 버그 발견+수정 — 스케줄러가 파일럿 종목을 간격 없이 연속 갱신해 2번째 종목부터 apibudget MinInterval(4s)에 매번 걸려 조용히 규칙 기반으로만 캐시되던 것을 실측(005930 llm / 000660 rule로 확인) 후 발견, 심볼간 6초 간격 추가로 수정 후 재검증(둘 다 llm 확인). go build/vet/test, Docker 재기동 실측, /admin/llm-settings 401 정상 차단, /admin/llm 미인증 리다이렉트 확인, tsc/eslint/next build. 관리자 화면 로그인 확인은 이전 세션과 동일 사유로 미완료. 버전 릴리스는 미진행(요청 시 v0.7.0 이후 누적분으로 문서화)
- 2026-08-07 | v0.7.0: Gemini 연동 + 2종목(005930·000660) 파일럿 제한 + 관리자 모니터링 | ok | config.buildLLMConfig()로 OPENAI_API_KEY 우선/GEMINI_API_KEY 폴백(OpenAI 호환 엔드포인트+gemini-3.1-flash-lite 기본값, 코드 변경 없이 provider 전환 가능); STOCK_SUMMARY_LLM_SYMBOLS env(기본 005930,000660)로 LLM 호출만 파일럿 제한(규칙 기반은 전 종목 유지), 스케줄러도 파일럿만 예열; apibudget 한도를 Gemini 무료 티어 공개치보다 보수적으로 조정(800/일,10/분,4s간격); 신규 GET /admin/stock-summary(읽기전용)+/admin/api "AI 요약 파일럿" 섹션. 실측: 005930·000660 모두 실제 Gemini 응답(source:llm, 재무·기술·내부자 수치 실데이터 일치, 뉴스 감성 카운트 기사수 일치), AAPL 등 비파일럿은 source:rule 고정+LLM 미호출 확인, apibudget 사용량 증가 확인. go build/vet/test, tsc/eslint/next build, 005930·000660 데스크톱+모바일 브라우저 확인(관리자 화면 로그인 확인은 이전 세션과 동일 사유로 미완료). v0.6.0 이후 누적분(1년 트레일링 버그 수정 포함) 전부 묶어 문서화, README 인덱스 갱신, fe/package.json 0.6.0→0.7.0 → `docs/releases/v0/v0.7/v0.7.0.md`
- 2026-08-07 | 뉴스 탭 + AI 3줄 요약(밸류에이션/기술적분석/내부자/뉴스) 신규 구현 | ok | 뉴스 탭은 기존 fetchStockNews+미사용이던 StockNewsPanel 재사용, BE 변경 없음. AI 요약은 유사 기능 부재 확인 후 Plan Mode 설계 승인 거쳐 구현 — RSI/MACD/ADX/SMA(신규 technical_indicators.go, Wilder 방식, 유닛테스트 포함) candle_bars 200일봉에서 실계산(허구 수치 없음); 비용 통제를 위해 탭 4개를 LLM 호출 1회로 통합 생성(GenerateStockSummary 신규, chatCompletion 공용 헬퍼로 리팩터), insights.go와 동일한 stale-while-revalidate를 심볼 기준으로 이식(StockSummaryService, Redis stock:summary:v1 24h/stale 6h), 예열은 MarketDataStore.AllTrackedSymbols(관심+보유)만 6h 주기; 뉴스 감성은 LLM이 기사별 라벨만 반환하고 집계는 서버가 직접 계산(기사 수와 항상 일치); apibudget에 ProviderOpenAI/CategoryLLM 신설 + budget Check/Record/429 연결; 버그 발견 — AllUsage() 하드코딩 목록에 openai 누락되어 /admin/api 미노출되던 것 실측 후 수정. OPENAI_API_KEY 미설정 환경이라 폴백(규칙기반 실수치 문장) 경로만 라이브 검증, LLM 경로는 코드리뷰로만 확인(사용자 환경에서 키 설정 후 재확인 권장). go build/vet/test, 005930·AAPL /summary 실측, Redis 캐시/TTL 확인, 스케줄러 자동예열 확인, tsc/eslint/next build, 6개 탭 데스크톱+모바일 브라우저 확인. 버전 릴리스는 미진행(요청 시 v0.6.0 이후 누적분과 함께 문서화)
- 2026-08-07 | 재무제표 상세 모달 "1년" 트레일링 윈도우 버그 수정 | ok | DartClient.FetchQuarterlyHistory가 연도 단위 조회라 "1년" 선택 시 올해 제출분만(연초엔 1~2분기) 반환하던 버그 — years+1개 연도를 가져온 뒤 최근 years*4개 분기로 트림하는 방식으로 수정, EDGAR(미국)은 원래 최근 N개 신고기간 방식이라 버그 없었음(대조군). Redis 캐시 삭제 후 005930 실측(1년=25년6월~26년3월 트레일링4분기, 3년=12분기 정상), 브라우저로 모달 "1년" 클릭 결과 확인. v0.6.0 이후 패치, 버전 태그 미부여
- 2026-08-07 | v0.6.0: 내부자 거래 탭 구현 + 이전 미출시 작업(전망 탭·재무제표 상세 모달·DART/EDGAR 최신성 비교·마켓 레이더 탭 유지) 묶어 출시 | ok | 신규 `GET /stocks/{symbol}/insider`: 국내는 DART elestock.json(임원ㆍ주요주주 소유보고, 날짜 필터 없어 전체 이력 중 최근 30건) 직접 매핑; 미국은 submissions.json으로 최근 Form 4/4-A 목록을 얻은 뒤 각 신고 원본 XML을 개별 fetch해 보고자·관계·거래코드(SEC 고정 19종)·수량·단가·거래후보유 파싱(최근 3건 제한). 버그 발견+수정 — EDGAR apibudget(분당 8회, 앱 전체 공유)이 US 심볼 1건당 최대 7회 소비라 연속 조회 시 소진되는데, 기존 코드는 개별 fetch 실패를 "신고 없음"으로 오인해 빈 배열을 24시간 캐시(MSFT로 실측 재현) — "시도했는데 전부 실패"면 에러 반환해 캐시 안 하도록 수정 + 신고 상한 5→3 하향, AAPL/MSFT/TSLA/NVDA/AMZN으로 재현·수정 검증. FE `InsiderTab`(국내/미국 통합 테이블) 신규, 워크스페이스+레거시 /analysis 양쪽 배선(8개 탭 전부 실구현 완료, AnalysisPlaceholderTab import 제거); go build/vet, tsc/eslint/next build, 데스크톱·모바일 브라우저 확인(다른 세션의 실행 중 dev 서버에 직접 접속 — 이 세션 자체 서버는 포트 충돌로 기동 불가해 .claude/launch.json에 autoPort:true 추가); v0.6.0 릴리스 문서에 이전 세션 미출시 작업 전부 묶어 문서화, README 인덱스 갱신, fe/package.json 0.5.0→0.6.0 → `docs/releases/v0/v0.6/v0.6.0.md`
- 2026-08-07 | LLM 재생성 00:00 KST 고정 + 관리자 수동 즉시 생성 + 사이드바 버그 진짜 원인 수정 + 종가 적재 설계안 | ok | 사이드바 버그 실제 원인 발견 — 이전 세션이 `/admin` 특수 케이스를 제거했지만 남은 일반 규칙(`pathname.startsWith(item.href + "/")`)이 `/admin`에 대해 `/admin/`으로 평가돼 모든 admin 하위 페이지에서 여전히 뉴스 관리가 활성이었음(이전 세션의 "브라우저 캐시" 결론은 오진). 항목별 독립 판정을 버리고 전체 nav href 중 최장 일치 1개만 활성(activeNavHref)으로 변경, 실행 중 dev 서버 번들에서 함수+allNavHrefs를 직접 추출·평가해 /admin/llm→/admin/llm, /admin→/admin 검증. 백그라운드 재생성 00:00:00 KST 고정 — refresh_interval_hours 노브를 모델/repo/handler/FE에서 제거(DB 컬럼은 NOT NULL DEFAULT라 마이그레이션 없이 미사용 방치), nextScheduledRefresh(KST 자정, 유닛테스트 5케이스) 루프로 교체, 기동 시 즉시 스윕도 제거해 재기동이 추가 과금이 되지 않도록 함. 수동 즉시 생성 신규 — POST/GET /api/v1/admin/llm/refresh, 파일럿 게이트 의도적 우회(임의 종목 지정이 목적, apibudget 한도는 그대로), 종목당 6초 간격 순차 실행, 종목별 pending/running/done/error + source(llm|rule) 추적, 동시 1건 제한(409), StockSummaryService.Start(ctx)로 bg context 보관해 종료 시 수동 잡도 취소, 요청 로그에 manual 플래그. 종가 적재 설계안(docs/DAILY_CLOSE_DESIGN.md, 미구현) — 종목 종가는 candle_bars 1d 그대로(종가 스냅샷 테이블 신설은 완전 중복이라 비권장), 지수는 신규 index_daily_closes(market_indices는 PK=index_id 최신 1행이라 이력 불가), LLM 일자별은 신규 stock_daily_analyses, 점수는 서버 계산+LLM은 문장만(뉴스 감성 집계와 동일 원칙), 00:00 KST 기준 trade_date가 시장별로 다른 함정(미국은 그 시각이 장중) 명시, 부가 제안으로 점수 적중률(score(D) vs return(D+1) 정렬 필수)·지수 대비 알파·이벤트 오버레이. go build/vet/test(신규 유닛테스트 포함), Docker 재기동(Tesseract OK), 신규 admin 라우트 401 확인, tsc/eslint/next build. 관리자 로그인 필요 화면(/admin/llm) 육안 확인은 이전 세션들과 동일 사유로 미완료 — 수동 즉시 생성 UI와 실제 LLM 호출은 사용자 확인 필요
- 2026-08-07 | 종목 뉴스 탭 페이지네이션 | ok | StockNewsPanel(default variant, 워크스페이스/analysis 뉴스 탭)이 limit=5로 호출되어 딱 5건만 표시되고 더 볼 방법이 없던 것이 "잘려 보이는" 원인 — 서버 페이지네이션은 NewsStore.List가 SQL LIMIT 후에도 관련성/중복 필터를 추가로 거는 구조라 페이지당 정확 건수를 보장 못해 배제, 대신 fetch 상한 60건 + 클라이언트 8건씩 페이지네이션(prev/next + 번호, pageWindow 로직 node로 검증)으로 구현, 심볼 전환 시 페이지 1 리셋, 두 호출부의 limit={5} 제거. tsc/eslint/next build 통과. /stocks/[symbol]은 일반 로그인 세션 필요해 이 환경에서 브라우저 실사용 확인은 못함(테스트 계정 자격증명 없음) — 사용자 확인 필요
- 2026-08-08 | 마켓 레이더 반응형 정비 + 일별 종가/AI점수 적재 구현 | ok | 치명 버그 발견: StockChartTerminal이 1024~1279px에서 flex-col+overflow-hidden인데 호가 aside가 shrink-0(525px)이라 차트+체결 컬럼이 0px로 압살돼 노트북 폭에서 차트 탭이 통째로 안 보이던 것 실측 확인 후 xl 미만 스크롤 단일컬럼/xl 이상 좌우분할로 재작성. 메트릭 레일 브레이크포인트(railFrom lg|xl) 도입 — 마켓 레이더는 랭킹 리스트가 320px를 이미 쓰는데 레일이 lg부터 떠서 1024px에서 탭 본문이 208px만 남던 것을 xl로 미뤄 448px 확보, 레일이 숨는 구간엔 StockMetricsGrid 신규로 시세/거래/재무/52주범위 제공(기존 모바일엔 접근 경로 자체가 없었음). 로딩 레일 hidden 누락으로 모바일 220px 죽은 컬럼 렌더되던 것, ?symbol= 딥링크가 모바일에서 리스트로 튕기던 것(paramsKey 리셋 effect가 마운트 시 실행), 모바일 랭킹 행에 랭킹 기준값 미노출, 탭바 활성탭 미스크롤, TOP5 데스크톱 전용, 리스트 pb-9 죽은 여백 모두 수정. 일별 종가: 029_daily_closes.sql(stock_daily_closes+index_daily_closes, score NULL→FE "-", ai_summary '' ), 시장별 정규장 마감+20분 적재 스케줄러, GET /stocks/{symbol}/daily-closes + /indices/{id}/daily-closes, FE 일별 시세 탭(워크스페이스+/analysis). 설계안 대비 변경: candle_bars 1d가 날짜당 2행(00:00이 전일 종가 복제)+주말 행을 갖고 있어 직독 시 등락률이 0%↔2배로 튀는 것을 실측(005930 -9.85%/0%/+3.64%) → ROW_NUMBER로 날짜당 마지막 봉만+주말 제외+LAG 정규화한 단방향 멱등 투영 테이블로 전환, 수정 후 +2.05%/+0.69%/-0.34% 정상, 기존 주말 행 378건 삭제. candle_bars 중복행 생성 경로는 미특정(후속 과제). 점수는 서버 계산(RSI30/추세25/MACD20/뉴스15/52주10, 결측은 비중 제외 후 재정규화, RSI는 70 부근 정점 후 하락) 유닛테스트 5건, LLM은 문장만, 실측 000070 score=75. 적재 커버리지 미완 — 활성 17,901종목 중 일봉 보유분만 가능해 세션 종료 시 1,469종목/6,131행, 나머지는 Toss 초당 5회 한도라 -fetch-missing 반복 실행 필요(재개 가능, 429 Retry-After 준수). go build/vet/test, Docker 재기동, API 실측(005930·AAPL·000070), 브라우저 375/1024/1440px 실측, tsc/eslint/next build
- 2026-08-08 | 탭 잘림 버그 수정 + 전망 탭 제거 + AI 분석 탭 신설 + 좌측 레일 축소 | ok | 치명 버그: scrollMain이 (compact || overview)라 데스크톱에서 개요 외 모든 탭이 overflow-hidden이고 스크롤 수단이 아예 없어 넘치는 내용이 잘려 있었음 — 실측 잘림 내부자거래 910px/1521px, 뉴스 484px, 재무제표 165px, 밸류에이션 80px, effectiveTab !== "chart"로 변경(차트 터미널만 자체 스크롤 소유). 전망 탭 삭제(워크스페이스+/analysis 탭목록·렌더 제거, 참조 0된 outlook-tab.tsx 삭제). AI 분석 탭 신설 — 개요 바로 우측에 배치, 헤더 중앙 빈 공간에 한 줄 요약(truncate 1줄 고정으로 헤더 높이 불변)+"전체 분석 →", 전체 분석은 AIAnalysisTab(4섹션+일별 종가/점수 리스트)에서 제공하며 기존 일별 시세 탭을 흡수 대체. 헤더가 넓은화면 중앙/좁은화면 자체행 두 곳에 같은 노드를 렌더하므로 fetch를 상위 useStockSummary 1회로 올리고 라인은 프레젠테이션 컴포넌트로 분리(중복요청 제거, StrictMode 이중마운트와 동일 수준까지 실측). 탭별 StockSummaryBanner 전면 제거(헤더+AI탭으로 일원화돼 중복이고 세로공간을 먹어 잘림을 키우던 요인), 참조 0된 stock-summary-banner.tsx 삭제. 좌측 레일은 시세+52주 범위만 유지하고 거래/재무/PER·PBR 설명 제거(기업정보·밸류에이션 탭과 중복), 모바일 StockMetricsGrid도 동일 범위로 축소 및 1열 카드+내부 2열 재배치, 미사용 exchangeRate prop·import 정리. 1440/1024/375px 실측 검증(탭 9개, 레일 2섹션, 헤더 AI라인 데스크톱 중앙 x=966 w=366 / 모바일 자체행 w=315, 전 탭 overflow-y auto, 가로 오버플로 0), tsc/eslint/next build 통과
- 2026-08-08: 종합 점수 전 종목 구현(`030_stock_scores.sql`, `cmd/compute-scores`, 5축 순위정규화+도메인가드+커버리지수축, 17,901종목 5초/평균49.9·표준편차12.6) + 관리자 `/admin/score`(전역·종목별 가중치, 즉시 재계산) + `-fetch-stale` 백필 모드 신설(8월7일 종가 누락 819종목 원인 해소, 커버리지 9,147) + AI 요약 카드 재설계·keywords 필드 신설 + **candle_bars 1d bar_time KST/UTC 혼재 버그 발견(미해결, trade_date 하루 밀림)**
- 2026-08-08: candle_bars bar_time 시간대 버그 수정(loc=Local + 컨테이너 UTC로 일봉이 D-1 15:00 저장 → trade_date 하루 밀림; 일봉은 KST 벽시계 문자열 저장으로 전환, cmd/repair-candle-tz로 149,803행 무손실 병합 보정; 드라이런이 '1M'→'1m' 대소문자 매칭과 제자리 UPDATE 유니크 충돌 2건을 사전 차단) + DAILY_CLOSE_DESIGN §12.1 오진 정정 + LLM 입력 76% 압축(10,728→2,621자, 원지표/분기제표/기사요약 제거하고 서버 분류 결과만 전달, 종합 점수·피어 백분위를 프롬프트에 투입해 축 간 인과 설명하도록 재작성)
- 2026-08-08: 룰 기반 시장 분석 prefix 신설(투자 포인트 + 상승/하락 시나리오, 가격대는 SMA/스윙/52주 등 실제 지표에서만 유래, LLM 미사용이라 전 종목 제공) + 탭별 점수 노출(AI 분석=전체 분해, 각 탭=해당 축) + 지수 점수·요약 도입(7/1부터 9개 지수 241행 백필, Toss 지표캔들/FRED SeriesHistory 신규/ETF proxy, 스파크라인은 날짜 없어 배제, 지수는 기술적 단독·횡단면 순위 미적용) + 재무 금액 통화 표기(영업적자가 "—"로 숨겨지던 버그 동반 수정) + DB 관리 카탈로그 8개 보강. 배치 관리 메뉴는 미착수
- 2026-08-08: 관리자 배치 관리 메뉴 신설(032_batch_job_runs, 5개 잡 목록·수동실행·커버리지·실행이력 영속화, DB전용/외부API 배지 구분, 동시 1건 제한 409, 재시작 시 stale running 정리) + 대시보드 지수 카드 점수 노출(GET /indices/scores 일괄 조회 신규 — 카드당 1회 조회는 개요를 느리게 함) + 일별 종가 전 종목 백필 완주(1,469→16,225/17,901, 436,199행; 점수 16,215종목 재산출 평균50.0·표준편차12.6; 잔여 1,676은 Toss가 400 주는 미국 우선주 티커) + batch_job_runs DB 카탈로그 추가
- 2026-08-08: 배치 잡을 API 프로세스에서 독립 워커 프로세스 5개(worker-close-domestic/overseas/scores/index/fetch)로 완전 분리. batch_job_runs를 큐로 전환(033, params JSON + queued 상태), API는 enqueue만 담당하고 실행은 워커가 4초 폴링으로 클레임(ClaimNext, MySQL UPDATE ORDER BY LIMIT 원자적). 국내/해외 일별 종가를 asset_type 필터로 완전 독립화(기존엔 트리거 시각만 분리, SQL은 전 종목 통합이었음). be/Dockerfile 하나에 api+worker 5개 바이너리 함께 빌드, docker-compose command:로 분기. 배포 직후 worker-close-overseas 크래시 발견(calendar.ScheduleAlways loc=nil로 SessionCloseAt 패닉 — 기존엔 이 경로가 호출된 적이 없어 잠재해있던 버그) 즉시 수정+회귀테스트. 큐→클레임→실행→기록 전 과정 실측 검증
- 2026-08-08: 전 서비스 시간 표기 KST 전수 감사. 컨테이너가 UTC로 도는 걸 실측 확인(candle_bars 시간대 버그와 같은 근본 조건) 후 동일 클래스 버그 6건을 백엔드에서 추가 발견: portfolio_history.go(포트폴리오 차트 라벨이 실제 UTC였음, 회귀테스트 추가), dart.go 오늘 공시 조회 2곳, earnings.go 실적캘린더, market_data_store.go 오늘 공시 캐시키 4곳, market_index_store_batch.go EOD 완료 마커. 프론트는 toLocaleString/DateString/TimeString 20여 곳에 timeZone:"Asia/Seoul" 명시 추가(format.ts에 formatDateTimeKST/formatTimeKST 신설, 관리자 패널 4곳 중복 구현 통일), chart-time.ts의 크로스헤어 시각이 축 라벨과 달리 로컬 getter를 써서 브라우저 비KST 환경에서 어긋나던 버그 수정. news_store_us.go의 명시적 .UTC()는 저자 의도로 판단해 미변경. go/tsc/next build 전체 통과, Docker 6개 컨테이너 재기동 확인
- 2026-08-09 | 종목 분석 탭 국내/해외 점수 랭킹 패널 | ok | GET /stocks/scores/top(market=domestic\|overseas) 신규, LatestScoredDate로 오늘 배치 미실행 시 어제로 자동 폴백; FE ScoreRankingPanel(크라운 top3·보유/관심뱃지·행클릭→해당종목 AI분석) AnalysisClient 상단에 접기 가능 섹션으로 삽입; 회원가입으로 테스트 계정 만들어 국내/해외 토글·행 클릭 이동·375px 무오버플로 브라우저 실측 확인
- 2026-08-09 | 랭킹 크라운→금/은/동 메달 전환 + "AI 투자일지"를 "메모장"으로 개편(백엔드 영속화 신규) | ok | (1) score-ranking-panel.tsx: Crown→Medal 아이콘, 1/2/3위 각각 실제 금(#FFB300)/은(#9CA3AF)/동(#B36A3C) 색상 적용. (2) 메모장: 기존 localStorage-only 구현(기기 간 미동기화)을 사용자 확인 후 백엔드 DB 연동으로 전환 — 신규 `journal_notes` 테이블(034 마이그레이션, MySQL=source of truth 원칙), CRUD `GET/POST/PUT/DELETE /api/v1/journal/notes`(user_id 소유권 검증), DB 관리 카탈로그에 추가. FE는 기존 title/body만 있던 메모를 종목 연동(StockCatalogSearchField 재사용, 클릭 시 AI분석 탭 이동)·고정(pin)·수정·검색/정렬(최신순·작성순) 갖춘 노트 UI로 전면 재작성, 사이드바 라벨 "AI 투자일지"→"메모장"(BookOpen→NotebookPen 아이콘). go build/vet, tsc/next build 통과, Docker 재기동 후 회원가입 테스트 계정으로 메모 생성·종목연동·고정·수정·검색·삭제 전 플로우 API 레벨 실측 확인(브라우저 자동화 도구의 window.confirm 자동취소로 삭제 버튼 클릭 자체는 fetch 직접호출로 대체 검증), 랭킹 메달 색상 getComputedStyle로 실측 확인
- 2026-08-09 | 스탬프(Stamp) 기능 신설 — 순간 스냅샷 + 가설 + 자동 채점 | ok | 사용자 요청(타임캡슐형 기록)을 설계 단계에서 재해석: 보존만으로는 재방문 동기가 없어 저장 시 가벼운 가설(관점 강세/약세/관망 + 검증기간 7/30/90일)을 붙이고 기간이 지나면 "개봉 대기"로 떠올라 적중률·평균 알파가 쌓이는 형태로 승인받고 구현(docs/STAMP_DESIGN.md). BE: 035_stamps.sql + StampRepo/StampService/StampHandler, /api/v1/stamps CRUD + /summary + /state + /{id}/open(전부 user_id 소유권 검증). 불변 스냅샷(시세·점수5축·뉴스5·공시·지수5·차트120봉)과 조회 시 계산(수익률 1D/1W/1M/3M·벤치마크 대비 알파·판정) 분리 — 스냅샷 UPDATE 금지로 적중률 사후조작 차단. 공시는 이력 테이블이 없어 캐시 전용 복사 + 없으면 dataGaps 명시(외부 호출 증가 0). 수익률 경계는 "목표일 이하 마지막 종가"만 쓰면 금요일 스탬프의 1D가 영구 공백이라 스탬프 이후 첫 종가를 목표일+5일 이내로만 허용. 실측 중 버그 2건 발견·수정: 장마감 시 라이브 거래량 0 박제(→일별종가 폴백), repository.nullFloat이 음수/0을 NULL로 만들어 하락 종목 등락률 유실(→nullFloat64). FE: 사이드바 기록>스탬프(/stamps), 종목 헤더 관심버튼 옆 도장 버튼(클릭=즉시저장 후 패널에서 관점/기간/메모 인라인, 5분 쿨다운은 서버 409가 진실), 목록은 요약 4타일+필터+타임라인/종목별, 행 확장 시 당시vs현재·점수5축 변화·박제차트+이후 종가·수익률/알파 표·당시 뉴스/시장/AI요약. go build/vet/test(신규 유닛 15건), tsc/next build, Docker 재기동 후 005930·000660·AAPL·035420 실제 스탬프 생성 실측(US는 NY 거래일+sp500 벤치마크 확인), 409 쿨다운·PATCH 가설반영·375px 무오버플로 확인
- 2026-08-09 | 레이아웃 아케타입 도입 — 마켓 레이더 중앙 정렬 통일 | ok | 사용자 지적("마켓 레이더만 전체 화면")의 실제 원인은 full-bleed가 아니라 헤더/본문 컬럼 불일치였음 — PageHeader는 news-page-shell(중앙 68%)을 쓰는데 /market의 main만 full-bleed라 1440px에서 제목이 x=415 w=830, 리스트가 x=220 w=1220으로 어긋나 있었음. 폭을 하나로 맞추는 대신 중앙 정렬 규칙·거터는 공유하고 max-width만 성격별로 고르는 아케타입 2종 도입: reading(.page-shell, 기존 값 유지) / wide(.page-shell-wide, min(72rem,100%)). PageHeader에 width prop + PageShellWidth 타입·pageShellClass 헬퍼를 page-content-shell.tsx에 두어 헤더와 본문이 같은 토큰을 강제로 공유하게 함. 상한은 112rem → 72rem으로 두 번 시도했다가 최종적으로 전부 되돌려 --content-max-wide: 100%로 확정. 112rem은 뷰포트 2012px 이상에서만 잘려 사용자 화면(1908px)에서 변화가 0이었고("똑같은데?"), 밀도(차트 폭 1018→484) vs 일관성 트레이드오프를 실측 표로 제시해 72rem을 골랐으나 실제로 써본 사용자가 "너무 보기 불편하다"고 되돌리기를 요청 — 1908px에서 컬럼 1152/여백268로 reading(1148)과 4px 차까지 갔지만 워크스페이스가 1318→830으로 줄어드는 대가가 컸음. 교훈: 터미널형은 밀도가 곧 기능이라 폭을 문서형에 맞추지 않는다. 아케타입이 통일하는 것은 "헤더 컬럼과 본문 컬럼이 같은 규칙을 쓴다"까지이고, 실제 해결된 원래 문제도 그것(헤더/본문 어긋남). 되돌리는 과정에서 market/page.tsx가 page-shell(= reading 컬럼 830px)로 바뀌어 있는 것과 이 세션이 만들지 않은 랭킹 리스트 접기 토글(listOpen + ChevronLeft/Right + 12px 스트립)이 들어와 있는 것을 발견 — 전자는 page-shell-wide로 교정, 후자는 "reading으로 좁아진 폭 보완"이 목적이라 주석에 명시돼 있었고 그 전제가 사라져 사용자 요청으로 삭제(state·import·JSX 전부, 워크스페이스 1308→1320 회복). 부수로 만든 버그 1건 수정: PageHeader wide 분기에 --content-gutter(4vw)를 줬더니 1908px에서 제목이 76px 들여써져(x=296) 리스트(x=221)와 새로 어긋남 → 터미널 패널과 같은 px-3으로 교체(제목 x=233 ↔ 첫 컨트롤 x=236). .page-shell-wide 좌우 경계선도 제거(상한 100%라 항상 꽉 차서 좌측선이 사이드바 테두리와 겹쳐 이중선). 다시 상한을 넣으려면 반드시 min(<값>,100%) 형태 — 100% 항이 없으면 AppShell overflow-x-auto가 가로 스크롤을 만듦. 적용: /market, /quotes(지수 바 포함), /stocks/[symbol]. .news-page-shell→.page-shell 리네임(뉴스 전용이 아니었음). 최종 실측 1024/1440/1908 — 헤더·본문 x/w 완전 일치하며 콘텐츠 폭 전체 사용(1908에서 x=220 w=1688), 리스트/워크스페이스 320/472 · 320/888 · 368/1308, 전 구간 가로 오버플로 0. tsc/next build 통과. 미확인: 로그인 필요 데이터라 차트·호가 2단 분할 렌더 상태는 못 봄(시세 로딩 중에서 멈춤) — StockChartTerminal 미변경 + 최종 폭이 원래대로라 회귀 가능성 낮음
- 2026-08-09 | 전 메뉴 페이지 헤더 통일 | ok | 사용자가 점수 관리/배치 관리/마켓 레이더 3개 헤더 스크린샷을 나란히 보여주며 "다 다르다" 지적. 점수 관리·배치 관리는 실제 버그였음 — PageHeader가 PageScrollShell 안쪽에 중첩돼 있어 page-shell 거터+py가 이중으로 씌워짐(다른 관리자 메뉴는 PageHeader를 PageScrollShell 앞 형제로 둠) → 두 파일 모두 형제 구조로 이동해 해결. 마켓 레이더는 버그가 아니라 앞선 세션에서 확정한 wide 아케타입 자체가 사용자 화면(1706px)에서 reading 메뉴들과 여백·헤더 높이가 달라 보이는 설계였음 — 재확인 질문 후 사용자가 "완전 통일" 선택 + 차트 공간 보완책으로 리스트 접기 기능 요청. PageHeader width prop 제거(기본 reading) + 본문 page-shell-wide→page-shell로 교체, 랭킹 리스트와 워크스페이스 사이에 접기/펼치기 토글 버튼(listOpen state, lucide ChevronLeft/Right) 신설. /quotes·/stocks/[symbol]은 동일 wide 패턴이라 향후 동일 요청 가능성 있으나 이번엔 마켓 레이더만 지목돼 미변경. tsc 통과, 브라우저에서 리스트 접기/펼치기 토글 실측 확인. 관리자 페이지(점수·배치)는 로그인 자격증명 없어 육안 미확인, 코드 구조만 다른 정상 관리자 페이지와 일치시킴
- 2026-08-09 | AI 요약을 마켓 레이더 1일 top100(파생 제외) 자동 배치로 전환 + LLM 토큰 한도 이중 가드 | ok | 사용자 질문("전 종목 LLM은 불가할 텐데 한도 내 최대 몇 종목?")에 실측으로 답: Redis apibudget 카운터상 openai 일 사용량이 11~49건뿐이라 500 RPD 중 400 가까이 유휴, 요청 로그의 factsJSON 2.3KB 기준 호출당 약 2.5~3K 토큰이라 RPD가 유일한 병목(15 RPM×3K=45K vs TPM 250K). 대상은 사용자가 "마켓 레이더 1일 top100에서 파생상품 제외"로 지정 — 저장된 랭킹 4슬롯(KR/US × 거래대금/거래량)을 실제로 뽑아 stocks.security_type으로 분류해 KR 155→105, US 171→136 = 241종목임을 사전 확인 후 설계 승인받고 구현. 선정기(llm_batch_targets.go)는 poller가 이미 저장한 payload만 읽어 외부 호출 0, ETF/ETN/ETP 제외(model.IsFundamentalsApplicable은 ETF·ETN만 알아 미국 ETP 11건이 통과하므로 재사용하지 않음), 최상위 랭크로 우선순위. 토큰 가드가 이번 작업의 핵심 신규 안전장치 — apibudget에 TokensPerDay/tokens_day 카운터 신설, 기존 TPM은 사후 차단이라 큰 호출 하나가 넘길 수 있었던 걸 CheckTokens(추정치 사전 차단)로 보강, llm.EstimateTokens는 한글 1자=1토큰·라틴 3자=1토큰으로 과대 추정해 안전측, usage.total_tokens 미보고 시 추정치를 대신 적립해 미계측 호출이 조용히 넘지 않게 함, 전 한도 env 재정의 가능(TPD 기본 1M은 공표값 아닌 자체 상한임을 문서·주석에 명시). 스케줄은 사용자 요청대로 개장 전 마감으로 역산 — KR 08:00 KST, US 08:30 ET(현지시각으로 표현해 서머타임 자동 추종, 서머타임 21:30 KST/표준시 22:30 KST), 대상수×6초+10분 역산 시작·최대 2시간 클램프, 휴장일 미실행(calendar.IsTradingDay), 재기동이 윈도 안이면 기록 없을 때만 이어서 실행. 예산 가드는 시작 전 Headroom에서 예비분(요청 80·토큰 12만)을 뺀 만큼으로 대상을 미리 자르고 실행 중에도 매 종목 전 데드라인·잔여 한도 확인 후 첫 위반에서 중단·이월. 부수로 발견한 실제 문제 2건: (1) 캐시 staleAfter 6h 때문에 파일럿 종목을 조회만 해도 LLM이 재호출돼 241종목이면 조회 트래픽만으로 하루 최대 4배 — LLM 경로를 배치+수동 2개로 한정하고 규칙 재빌드가 직전 LLM 서술을 승계하도록 변경(뉴스 감성 집계도 함께 승계하지 않으면 감성 배지와 일별 점수 뉴스 축이 사라짐, analyzedAt 신설로 문장 작성 시각 보존), (2) PreviewBatchTargets가 batchTargets를 그대로 호출해 관리자 페이지를 열 때마다 이월 큐를 비우던 문제를 구현 중 발견해 미리 수정. 설정은 036 마이그레이션으로 target_mode/daily_cap/quota_reserve/token_reserve 추가하고 pilot_symbols를 "항상 포함되는 고정 종목"으로 의미 전환(TEXT, 10→50). 마이그레이션 함정: splitSQLStatements가 세미콜론 단순 분할이라 SQL 주석 안 세미콜론이 문을 쪼개 기동 2회 실패 — 주석에 경고 명시. go build/vet/test 전체(신규 유닛 20건 — 스케줄 6·선정 5·토큰 추정 5·예산 6), Docker 재기동(Tesseract OK, 036 적용, 6개 컨테이너 정상), 실제 MySQL/Redis로 선정 결과 실측(KR 105/US 136), tokens:day 카운터 적립 확인, 신규 admin 라우트 401, tsc/next build 통과. /admin/llm 육안 확인은 관리자 자격증명이 없어 미완료 — 첫 배치는 08-10 07:xx KST(KR)에 실행 예정이라 결과 확인 필요
- 2026-08-10 | 리서치 → 리포트 개편(리더보드 삭제·보상제도 미도입, 마크다운 블로그형 작성/렌더, 중앙 1단 레이아웃) | ok | 사용자 지시 4건을 그대로 이행. (1) 명칭: 메뉴 리서치→리포트, 객체 리서치 콜→투자 리포트, FE 경로 /research/*→/reports/*. API·테이블·컴포넌트 파일명은 research 유지(코드에서 찾을 땐 research로 검색해야 함을 AGENT_STATE·타입 파일 주석에 명시). (2) 리더보드 제거: FE 페이지/클라이언트, BE 라우트·핸들러·research_leaderboard.go(연율화 alpha×페널티·기간/시장 필터)·모델 2종·ResearchRepo.ResolvedForLeaderboard·유닛 11건 삭제. AuthorProfile은 research_authors.go로 옮겨 유지 — 그 사람의 이전 리포트 결과는 글을 읽을 때 필요한 정보지만 작성자 간 순위는 리포트를 대회 참가작으로 만들고 대회는 보상 없이 유지되지 않는다(유사투자자문 규제선). 보상·시즌 대회는 도입하지 않음을 설계문서 §9/§16/§17에 확정 기록. (3) 마크다운: 의존성 없이 lib/markdown.ts 파서 + ui/markdown.tsx 렌더러 신규 — 공개 사용자 작성물이라 dangerouslySetInnerHTML을 일절 쓰지 않고 React 엘리먼트로만 렌더, 링크는 http(s)/mailto/내부경로만 허용(javascript: 등은 앵커 대신 평문), HTML 주석은 파싱 전 제거해 서버 초안의 안내 문구가 독자에게 새지 않게 함. ui/markdown-editor.tsx는 툴바 10종 + 작성/미리보기 탭이고 미리보기는 상세 화면과 같은 렌더러를 재사용. 서버 초안(research_draft.go)도 한 줄 요약/투자 논거/무엇이 확인되면 틀린 것인가/발행 시점 데이터 골격으로 재구성(LLM 0콜 유지). (4) UI: 08-09에 2단이라 wide로 뒀던 작성·상세를 reading 1단 중앙 정렬로 전환 — 에디터가 컬럼 전체 폭을 쓰므로 236px로 눌리던 문제가 사라지고 모바일에서 순서가 뒤엉키던 것도 함께 해결. 목록은 블로그 인덱스형 카드(제목·발췌·읽는 시간·조회/좋아요/댓글), 상세는 제목→요약 카드→본문→발행 시점 근거→반응 순서. 관리자 라벨(배치 카드·DB 카탈로그)도 리포트 명칭으로 동기화. 검증: go build/vet/test 전체, tsc/next build(라우트 목록에 /reports*만 남고 /research* 소멸), eslint 신규 3파일 무경고, Docker 8컨테이너 재기동(Tesseract OK), 브라우저 실측 — 사이드바 단일 항목·리더보드 없음, /research·/research/leaderboard·/reports/leaderboard 전부 404, 상세 렌더, 작성 화면 초안 1157자 자동 주입, 미리보기에서 표 정렬(left/right/center)·중첩 목록·코드블록·인용·취소선 렌더 및 javascript: 링크 평문화 확인, 375px 목록/상세/작성 가로 오버플로 0. 미확인: 관리자 자격증명이 없어 /admin/batch·/admin/database의 변경된 라벨은 육안 미확인(문자열 상수만 변경)
- 2026-08-10 | 리포트 사용성 2차(목록 필터 재구성·2줄 초안·본문 이미지 업로드 기반 구축) | ok | 사용자 지시 4건. (1) 스코프 탭을 전체 리포트(기본·왼쪽)/내 리포트로 바꿈 — 처음 들어온 사람에게 빈 "내 리포트"를 보여주면 화면의 용도를 알 수 없다. (2) 상태 필터(검증 중·판정 완료·철회) 제거, 좌측 의견 필터 + 역발상, 우측 끝 정렬 최신/조회 순/추천 순으로 재배치(ml-auto, 좁은 화면에선 아랫줄 우측). 조회·추천은 동점 시 최신순 폴백. 알파순·판정 임박 정렬과 그 헬퍼(alphaOf, isResolvedStatus)는 사용처가 사라져 삭제. (3) 에디터 프리필을 방향별 2줄 안내로 축소(HTML 주석→평문)하고 MarkdownEditor에 clearOnFirstEdit prop 신설 — 안내가 그대로일 때만 muted로 보이고 첫 클릭/포커스에 사라지며 글자수도 0으로 센다. 손대지 않은 안내는 발행 시 빈 본문으로 치환해 게시 자체를 막음. 기존 긴 자동 초안은 buildResearchDataBlock→preview.dataMd로 분리해 "발행 시점 데이터 넣기" 버튼으로만 삽입(LLM 0콜 유지). (4) 이미지 업로드는 리포트 기능이 아니라 앱 공통 경로로 설계: be/internal/media에 Store 인터페이스 + 첫 구현 LocalStore, MediaService(정책), MediaHandler(POST /api/v1/media/images 인증 · GET /api/v1/media/* 공개). 키는 내용 sha256(ab/cd/hash.ext)이라 중복 저장이 없고 불변이라 immutable 캐시가 성립. 타입은 서버가 스니핑해 결정(클라이언트가 주장한 Content-Type·파일명은 근거가 아니며 image/png이라 선언한 HTML은 415), SVG는 스크립트를 담는 문서라 이미지로 취급하지 않음, 서빙에 nosniff + CSP default-src 'none'; sandbox, 키 정규식 검증이 곧 경로 탈출 방지. 객체 스토리지 전환 시 바꾸는 것은 Store 구현 하나와 MEDIA_PUBLIC_BASE_URL. Docker에는 quant_port_media named volume 신설(import temp와 달리 영구 보관이라 writable layer면 재기동마다 소실) + Dockerfile에서 디렉터리를 미리 만들어 소유권이 볼륨에 복사되게 함. FE는 lib/media-api.ts(공통 업로더) + 에디터 사진 버튼·붙여넣기·끌어놓기, 마크다운 파서/렌더러에 이미지 노드 추가하되 src는 링크와 같은 화이트리스트라 javascript:·data:는 img가 아니라 alt 텍스트로 렌더. 업로드 중 타이핑이 삽입에 덮이지 않게 latest ref 사용. 검증: go build/vet/test 전체(신규 유닛 15건 — media store 5·media handler 6·draft 3), tsc/next build, eslint 신규 파일 무경고, Docker 8컨테이너 재기동(Tesseract OK, media store 로그 확인), 브라우저 실측(목록 버튼 배치·기본값·정렬 우측정렬 1280/375·오버플로 0, 임시 하네스로 안내 2줄 muted→클릭 시 삭제→이미지 붙여넣기 삽입→미리보기 img 렌더→위험 src 평문화 확인 후 하네스 삭제, 비로그인 401·경로 탈출 404). 미확인: 세션 만료 + 계정 생성·비밀번호 입력을 하지 않는 정책이라 로그인 상태의 실제 업로드와 "발행 시점 데이터 넣기" 버튼은 브라우저로 확인 못 함 — 핸들러 레벨 테스트(201·저장 왕복·헤더·415/413/400)로 대체. 남은 것: 이미지 GC·업로드 쿼터·리사이즈 없음
- 2026-08-10 | shadcn/ui 전환 1단계 — 기반 구축(토큰 레이어·`--accent`→`--brand`·dark 변형 연결) | ok | 사용자 질문("shadcn으로 전체 UI를 바꿀 수 있나")에 스택 점검 결과로 답한 뒤 두 가지를 확인받고 착수: 최종 룩앤필은 **shadcn 기본 스타일로 리디자인**, 이번 범위는 **1단계(기반)만**. 스택은 그대로 맞았음 — Next 16.2.10 + React 19.2.4 + Tailwind v4 + `@/*` 별칭 + `cn()`(clsx+tailwind-merge) + lucide, shadcn CLI 4.16.2가 Next 16을 문제없이 감지했고 `shadcn add` 전후 `globals.css` diff가 0이라 CSS는 CLI에 맡기지 않고 전부 수작업으로 넣음(init에 맡기면 자기 `:root` 블록을 뒤에 붙이면서 기존 `[data-theme=...]` 팔레트를 특정도 동률·순서 우위로 덮는다). 이번 작업의 핵심은 토큰 이름 충돌 — `--accent`가 이 앱에선 브랜드 블루(링크·활성)인데 shadcn에선 은은한 hover 표면이라 의미가 정반대여서, 그대로 두면 shadcn 컴포넌트의 hover 배경이 전부 파래짐. 앱 쪽을 `--brand`/`--brand-muted`로 옮기고(65파일 273곳 일괄, `--accent-foreground`도 `--brand-foreground`로) `--accent`를 shadcn에 넘김. `--background`·`--border`는 이름·의미가 같고 값만 달라 1단계에선 정의하지 않고 기존 값 유지(덮으면 아직 교체하지 않은 화면 전부가 그 자리에서 바뀜) — 2단계로 이월. shadcn 토큰 값은 레지스트리 neutral 원본(`ui.shadcn.com/r/colors/neutral.json`)을 fetch해 그대로 넣어 기억에 의존하지 않게 함. `dark:` 변형은 `.dark` 클래스가 아니라 `<html data-theme>`이고 어두운 테마가 셋(dark·midnight·general)이라 `@custom-variant dark (&:is([data-theme=...], ... *))`로 연결. 일부러 넣지 않은 것 2개(둘 다 2단계): `--radius-*` 스케일은 shadcn 전용 토큰이 아니라 Tailwind 기본 반경을 덮어써서 실제로 켜봤더니 로그인 버튼 `rounded-lg`가 8px→10px로 바뀌었고 앱 전체 341곳(md 105·lg 194·xl 39·sm 3)이 동시에 움직임, shadcn base 레이어(`* { @apply border-border }`, `body { bg-background text-foreground }`)는 색 미지정 테두리 전체와 body 글자색을 즉시 바꿈. 검증: tsc·next build(37페이지)·eslint 통과, 브라우저 실측에서 4테마 전 토큰 값이 작업 전 기준선과 완전 동일하고 `/login` 제출 버튼(bg rgb(37,99,235)·h 44px·radius 8px)·입력창·`rounded-xl` 카드 12px 그대로, 375/1280 오버플로 0. 임시 하네스 페이지로 shadcn Button 6변형×4테마를 실측(light `--primary` lab 7.78% / 어두운 테마 3종 lab 90.95%, `dark:bg-destructive/60`·`dark:bg-input/30` 적용 확인) 후 하네스 삭제. 실측 함정 2개를 문서화 — 테마를 바꾸고 `getComputedStyle`을 바로 읽으면 shadcn의 `transition-all` 때문에 전이 시작값(이전 테마 색)이 나오고, 기다렸다 읽으면 `PreferencesProvider`가 `data-theme`를 되돌려 놓아, 이 둘 때문에 "dark에서 토큰이 안 먹는다"는 잘못된 결론을 한 번 냈다가 전이를 죽이고 동기 측정으로 바로잡음. 신규 의존성 radix-ui·class-variance-authority·tw-animate-css이고 돌고 있던 dev 서버는 새 패키지를 못 찾아 `Can't resolve 'tw-animate-css'`로 죽으므로 설치 후 재시작이 필요(사용자가 띄워둔 3000번 서버를 이 이유로 재시작함). 기존 버그 1건 발견·미수정: `--brand-foreground`(옛 `--accent-foreground`)가 정의된 적 없는데 3곳에서 사용 중이라 글자색이 상속으로 떨어짐 — 2단계 Button 교체로 자연 해소 예정. 산출물: docs/SHADCN_MIGRATION.md(단계·충돌표·교체하지 않을 영역), fe/DESIGN.md에 shadcn 섹션 + 팔레트 표의 낡은 profit/loss 값 정정(#10B981/#EF4444 → #EF4444/#3B82F6)
- 2026-08-11 | 버그 수정 — 종합 점수 배치가 8/7 고정으로 산출 | ok | 사용자 제보("오늘 돌아간 JOB이 8월 7일 기준으로 계산됨")로 시작. `worker-scores` 00:25 KST 로그를 보니 실제로 `2026-08-07 — 17902 rows`. `ScoreService.LatestTradeDate()`는 `stock_daily_closes`의 MAX(trade_date)를 그대로 쓰는데, 그 테이블을 채우는 `DailyCloseService.RunMarketScheduler`/`nextTrigger`가 항상 `now` 이후의 다음 트리거만 계산해서 KRX 마감+20분(15:50 KST) 창이 이미 지난 뒤 컨테이너가 재기동되면 그 세션이 영구 스킵됨을 발견 — 전 세션이 8/10 17:50 KST(마감창 이후)에 `docker compose up --build`로 재기동한 게 정확히 이 경우였음. candle_bars엔 8/10 1d 봉이 이미 정상 존재(별도 폴러가 채움), 프로젝션 단계만 누락. 이 프로젝트는 BE 변경마다 Docker 재기동이 관행이라 재기동 타이밍이 마감 직후에 걸릴 위험이 상존. 즉시 복구: `backfill-daily-closes`로 8/10 프로젝션(외부 호출 0, 355행) → `compute-scores`로 8/10 재산출(16,215 scored, healthy) → `batch_job_runs`에 수동 enqueue해 이미 떠 있는 worker-scores가 지수까지 재계산(KR 지수는 별도 폴러라 애초 영향 없었음, US 지수는 미국장 미마감이라 8/7 유지가 정상임을 확인). 근본 수정: `RunMarketScheduler`가 루프 진입 전 `catchUpMissedSessions`를 한 번 실행해 각 스케줄의 가장 최근에 이미 지난 마감+지연 시각을 역방향 탐색(`lastCompletedTrigger`)해 즉시 재적재 — 멱등 UPSERT+외부 호출 0이라 매 기동마다 무조건 실행해도 안전. 신규 유닛 3건(재기동 갭 재현·주말 스킵·정상 케이스). 검증: go build/vet/test 전체 통과, Docker 8컨테이너 재기동(Tesseract OK) — 이번 재기동 자체가 버그 유발과 같은 타이밍이었는데 로그에 `daily close: krx 2026-08-10 — ...`(캐치업 작동)·`daily close: us_equity 2026-08-07 — ...`(정상, 미마감)로 실측 확인. worker-scores 자신의 00:25 스케줄도 같은 구조이나 다음날 자연 치유되고 수동 큐도 있어 이번엔 손대지 않음
- 2026-08-11 | UI/UX 설계 인터뷰(D1~D18) + shadcn 2단계 구현(토큰 전환·프리미티브·셸) | ok | 사용자가 "질문하며 디자인을 구체화하고, 승인 전엔 구현하지 말라"는 진행 방식을 지정 — 영역을 12개로 쪼개고 한 번에 하나씩 물어 결정 18건을 누적했다(근거·버린 대안 전문은 docs/archive/ui-spec-interview.md, 확정본은 docs/UI_SPEC.md 17절). 인터뷰 중 내가 제시한 근거가 사실과 달라 결정을 한 번 뒤집었다: "사이드바에 없는 화면 6종의 진입 경로를 ⌘K 팔레트가 메운다"고 했으나 실제로는 그 화면들이 page-stubs의 준비 중 안내 페이지이거나 ACCOUNT_FEATURES_VISIBLE=false로 꺼둔 기능이었고, 검색으로만 도달하는 실제 화면은 종목 상세 하나뿐이었다 — 팔레트의 IA 보완재 역할이 성립하지 않아 도입하지 않기로 정정하고 사이드바 항목 수도 22개→일반 사용자 10개로 바로잡았다. 확정된 주요 결정: 좌측 사이드바 유지 + 전체 폭 헤더(56px, 검색·계정 좌표 고정을 우선), 검색은 입력창만(팔레트 보류), 헤더 우측은 계정 드롭다운 하나로 통합(알림 버튼은 배지 3이 하드코딩된 미구현이라 제거, 설정은 사이드바와 중복이라 드롭다운 안으로), 사이드바 48px 아이콘 레일 + 접힘 상태 저장, 활성 표시는 사이드바·탭·칩 전부 중립(파랑이 이 앱에선 하락을 뜻해 손익 색과 의미가 겹친다 — 색이 튀는 곳을 숫자로 한정), 밀도 촘촘(행 30px·본문 12px·컨트롤 32px, 마켓 레이더 한 화면 20종목), 뉴스=행/리포트=카드(콘텐츠 성격 우선), 테마 라이트+다크 2종(midnight 제거, 다크 #000→#252525), 짧은 건 모달 긴 건 우측 시트, 모바일 표는 중요 3열+행 펼치기, 관리자 데스크탑 전용, 등락은 색+부호(색맹 대응, 화살표 제거). 결정 과정에서 목업 3종을 만들어 실제로 보여주고 골랐다(셸 배치 비교·사이드바 배경 A/B/C·모바일 표 A/B/C, 주요 화면 4개 목업은 docs/mockups/shadcn-shell-v2.html에 밀도·테마 토글 포함). 승인 후 2단계 구현: 토큰 값을 shadcn neutral로 넘기고 --radius-* 스케일과 base 레이어를 켰으며, 핵심 판단은 이 앱 고유 토큰을 shadcn 토큰의 별칭으로 만든 것(--surface: var(--card), --text-primary: var(--foreground) …) — var(--surface)를 쓰는 137곳을 건드리지 않고 색만 넘어간다. Card/Badge는 레지스트리 버전으로 갈아끼우지 않았는데, shadcn CardHeader가 grid+px-6 기반이라 이 앱의 flex justify-between와 규칙이 달라 호출부 137곳 헤더 정렬이 한 번에 어긋나기 때문이고 둘 다 동작 없는 표현 컴포넌트라 토큰만 바꾸면 결과가 같다. dropdown-menu·tooltip·sheet·dialog·alert-dialog·tabs·separator·sonner·command·input·label 설치(sonner는 next-themes 의존을 걷어내고 PreferencesProvider를 읽게 고침), button/input은 h-8·text-xs로 조정하되 input의 모바일 글자만 text-base로 남김(16px 미만이면 iOS가 포커스 시 확대). 테마 축소는 preferences.ts가 저장된 midnight/general을 dark로 변환하지 않으면 기존 사용자가 light로 튕기므로 마이그레이션을 함께 넣었고 layout.tsx 초기화 스크립트·설정 카드·@custom-variant·news-badge 선택자까지 정리했다. 셸은 헤더 56px + 계정 드롭다운, 사이드바 --sidebar 토큰·48px 레일·접힘 상태 저장(좁은 화면에선 저장값 미적용 — 오버레이라 앱을 가린다), PageHeader 우측 동작 영역 신설, 종목 탭 바 중립 활성 + 우측 페이드. 작업 중 실수 1건: globals.css를 줄 번호로 splice하다 경계를 잘못 잡아 레이아웃 :root 블록의 뒷부분을 날렸고 즉시 복원함. 실측 함정 하나를 새로 배웠다 — document.querySelectorAll("header")가 2개를 반환하는데 하나는 React SSR 스트리밍용 <div id="S:0" hidden> 안의 복사본이라 offsetParent로 걸러야 하고, 이걸 모르고 숨은 복사본을 재느라 "사이드바가 안 펼쳐진다"는 오진을 한 번 했다. 검증: tsc·next build(37페이지)·eslint 통과, 브라우저 실측으로 헤더 56px·사이드바 220↔48px·--sidebar 배경(라이트 #FAFAFA/다크 #343434)·활성 메뉴 중립(oklch 0.93/0.269, weight 500)·버튼 32px/12px/radius 8px·양쪽 테마 오버플로 0 확인. 미확인: 로그인 세션이 없어 계정 드롭다운 실제 개폐, 관리자 화면, 종목 워크스페이스 탭 바는 육안 확인 못 함. 남은 것: 3~4단계와 앱 전역 raw button 182곳·input 48곳의 컴포넌트 이관(그 전까지는 밀도가 컴포넌트로 수렴하지 않음)
- 2026-08-11 | UI 후속 — 사이드 이펙트 수정·전체 폭 전환·홈 종합 대시보드 | ok | 사용자가 참고 화면(월가아재 라운지)을 첨부하며 세 가지를 요청: 앞선 shadcn 작업의 사이드 이펙트 해결, 대부분 메뉴가 중앙 컬럼만 쓰던 것을 전체 화면으로 전환해 데이터를 더 보여줄 것, 대시보드에 여러 메뉴의 주요 내용을 종합할 것. 스크린샷을 찍을 수 없는 환경이라 계산 스타일 감사로 회귀를 찾았고 가장 큰 것이 라이트 모드에서 카드가 배경과 구분되지 않는 문제였다 — shadcn neutral 기본값은 --background와 --card가 모두 순백이라 페이지 전체가 흰 판 하나가 되고 헤어라인 테두리만 남는다(카드 21개가 깔리는 화면에서 정보 구조가 사라짐). 페이지 배경만 oklch(0.97)=#F5F5F5로 한 단 내려 단차를 복구했고 다크는 이미 #252525/#343434 단차가 있어 두었다. 두 번째는 StatCard가 자체 p-3 sm:p-4를 유지해 Card(p-2.5)와 안쪽 여백이 달라 보이던 것 — 같은 값으로 통일하고 라벨·수치 크기도 촘촘 밀도에 맞췄다. 전체 폭 전환은 PageScrollShell/PageHeader의 기본 아케타입을 reading에서 wide로 바꾸는 방식으로 했고(명시적으로 reading을 넘기던 곳이 없어 기본값 변경만으로 27개 화면이 한 번에 전환), 거터는 .page-shell-wide가 padding-inline .75rem로 갖게 하면서 PageHeader의 px-3 특례를 제거했다 — 거터가 두 겹이면 과거 마켓 레이더가 겉돌아 보이던 헤더/본문 경계 어긋남이 재발한다. 직접 page-shell 클래스를 쓰던 admin 3개·news·settings도 wide로 교체했고, reading은 긴 산문을 읽는 리포트 상세·리포트 작성에만 명시적으로 남겼다(PageHeader에도 같은 값을 넘겨야 컬럼이 맞음). 실측으로 1280px에서 홈 본문 컬럼이 714 → 1050px가 됐다. 홈 대시보드는 dashboard-overview.tsx를 신설해 지수 바 아래에 거래대금 상위 8 + 오늘의 뉴스 6을 2단으로, 그 아래 최신 투자 리포트 4를 카드 그리드로 배치했다. 설계상 핵심은 블록마다 독립적으로 실패하게 만든 것 — 마켓 레이더가 죽어도 뉴스는 떠야 하고 리포트가 401이어도 나머지는 보여야 하므로 블록별로 fetch·로딩·빈 상태를 따로 갖는다. 랭킹 API 응답에 종목명이 없어 fetchQuotesBatchClient로 이름을 채웠다(종목 코드만 보여주면 훑을 수 없다). 검증: tsc·next build(37페이지)·eslint 통과, 브라우저 실측으로 홈 3블록 모두 실데이터 렌더(삼성전자·SK하이닉스 등 8종목·실제 뉴스 6건·리포트 2건), 라이트/다크 배경-카드 단차, 마켓 레이더 제목 x=232 ↔ 본문 x=232 정렬, 1280/375 오버플로 0 확인. 남은 것: 컴포넌트별 자체 패딩(p-4 등)이 여럿 남아 밀도가 아직 완전히 수렴하지 않았고 raw button 182곳·input 48곳 이관도 그대로다
- 2026-08-11 | 전체 폭 후속 — 화면별 다단 재구성(본문 + 우측 레일) | ok | 사용자가 자기 화면(종목 분석)과 참고 화면(네이버 증권·토스증권) 스크린샷을 첨부하며, 앞 세션의 전체 폭 전환 이후 페이지 구성이 그대로라 컴포넌트가 가로로 늘어나기만 한다는 문제를 제기하고 마켓 레이더를 제외한 페이지들을 여러 관련 컴포넌트로 재구성해달라고 요청(목업 데이터 허용). 방침은 본문 폭을 다시 좁히지 않고 남는 폭을 같은 맥락의 위젯으로 채우는 것으로 잡았다. 먼저 공용 프리미티브 두 개를 세웠다 — layout/page-columns.tsx의 PageColumns(본문 + 우측 레일, 종목 분석만 좌측 레일까지 3단)와 ui/section-card.tsx(SectionCard·SectionEmpty·SectionSkeleton·SectionKeyValue). 홈 대시보드가 들고 있던 사설 SectionCard를 여기로 승격해 위젯 크롬이 화면마다 갈라지지 않게 했다. 브레이크포인트 규칙은 레일이 xl(1280) 이상에서만 컬럼이 되고 그 아래에서는 본문 밑으로 쌓이는 것, 컬럼 정의에 minmax(0,1fr)를 쓰는 것(1fr만 쓰면 그리드 아이템의 min-width가 auto라 안쪽 긴 표가 컬럼을 밀어내 레일이 잘린다), 3단 화면의 우측 레일은 2xl 미만에서 두 컬럼을 가로지르며 grid-cols-2로 흐르고 sticky를 끄는 것(본문 아래에 있는 블록을 붙여두면 스크롤이 거기서 멈춘 것처럼 보인다)이다. 재구성한 화면은 8개. 종목 분석은 화면 폭을 가로지르던 랭킹 배너를 좌측 레일로 내리고(고정 max-h 420px를 풀어 레일 높이를 채우되 좁은 화면에서만 420px 유지) 우측에 stock-side-rail.tsx를 신설해 시세 스냅샷·52주 위치 막대·종합 점수 5축·관련 뉴스 5·이 종목 리포트를 상시 배치했다 — 탭을 옮겨도 맥락을 잃지 않는 것이 목적이다. 뉴스는 지수 흐름·오늘의 키워드·많이 본 뉴스·최신 리포트를 레일에 뒀고, 키워드는 서버에 다시 묻지 않고 화면에 이미 있는 기사에서 세도록 했다(집계의 근거가 곧 목록이라 둘이 어긋날 수 없다). 누르면 피드가 그 키워드로 걸러지고 제목 옆에 해제 칩이 뜬다. 리포트는 성적표 4타일을 레일로 옮기고 많이 다룬 종목·많이 본 리포트·채점 방식 안내를 더했으며 순위표는 여전히 두지 않았다. 관심종목은 가끔 쓰는 "종목 추가" 검색이 매번 목록을 아래로 밀던 것을 레일로 옮겼고, 스탬프는 요약 타일과 종목 칩 벽(종목이 스무 개면 세 줄로 접혀 목록을 밀어냄)을 레일 세로 목록으로 바꿨다. 메모장은 검색·종목 필터를 레일로 보내고 정렬만 목록 헤더에 남겼다. 목표가는 입력 폼을 레일로 옮기면서 목록에 현재가와 목표까지 남은 %를 추가했는데(fetchQuotesBatchClient), 현재가가 없으면 —로 두고 0%로 그리지 않는다(없는 값을 "이미 도달"로 만들지 않기 위해). 보유종목은 총 평가금액·오늘 손익·비중 상위를 레일에 뒀고 원/달러가 섞이므로 환율로 원화 환산하되 환율을 못 받았으면 일부만 더한 값을 총 평가금액이라 부르지 않고 합계를 보류한다. 손대지 않은 것은 마켓 레이더(요청 제외), 설정(데스크탑에서 896×640 모달이라 애초에 늘어나지 않음을 실측 확인), 종목 워크스페이스·시세(이미 터미널형 다단). 작업 중 기존 버그 1건을 고쳤다 — 홈 "최신 투자 리포트"의 방향 배지가 자체 맵(long/short/neutral)을 쓰는데 API 값은 buy/sell/hold라 모든 리포트가 폴백인 "관망"으로 찍히고 있었고, 리포트 화면과 같은 DirectionBadge로 교체했다. 실측 중 내 코드의 결함도 하나 잡았다 — 관심종목 강세/약세를 정렬 후 위 3/아래 3으로 자르면 종목이 다섯 개 이하일 때 같은 종목이 양쪽에 나오고 보합이 "약세"로 불려서, 부호로 먼저 거른 뒤 자르도록 고쳤다. 새 실측 함정 하나: 이 브라우저 창은 document.visibilityState가 hidden이라 useMarketIndices의 isPageVisible() 게이트에 걸려 지수가 영영 오지 않는다 — visibilityState를 덮어쓰고 visibilitychange를 쏴야 로드되며, 이걸 모르면 "지수 위젯이 안 뜬다"고 오진한다(앱은 정상). 검증: tsc·next build(37페이지) 통과, eslint 신규 파일 10개 무경고이고 수정 파일에 새 문제 0(남은 set-state-in-effect는 전부 이전부터 있던 것 — 처음엔 git stash로 전후를 비교했으나 이 저장소는 이전 세션들의 미커밋 변경이 대량이라 stash가 작업트리 전체를 되돌려 비교가 성립하지 않았고, 파일별로 직접 확인해 바로잡았다). 브라우저 실측은 1600px 3단(300/854/340, 오버플로 0)·1280px 2단 + 레일 2×2 격자·375px 전 화면 세로 스택 오버플로 0, 종목 분석에서 랭킹 클릭 시 본문과 우측 레일이 함께 전환(RF머트리얼즈 38,400원·89점 A+), 뉴스 키워드 "유가 — 21건" 클릭 시 피드가 100 → 정확히 21건, 목표가는 삼성전자 +24.5%·SK하이닉스 "도달"로 계산 확인. 보유종목·관심종목 레일은 로그인 게이트라 임시 하네스 페이지로 교차통화 합계를 검증하고(원 2,400,000 + $1,000×환율 = 3,817,710원, 일간손익 -1,266원, 비중 62.9/37.1) 하네스를 삭제했다. 미확인: 로그인 세션이 없어 관심종목·보유종목·스탬프·메모장의 실데이터 화면은 육안으로 못 봤다(레일 자체는 하네스와 빈 상태로 확인). 관리자 7개 화면은 이번 범위 밖으로 두었다
- 2026-08-11 | dev 서버 CSS 캐시 사고 규명 + 홈 대시보드 재구성 | ok | 사용자가 "색감이나 선이 너무 많이 바뀌어 가시성이 나빠졌다, 예쁘게 꾸며달라"고 신고. 디자인을 손대기 전에 대비를 실측했는데 카드 배경이 rgba(0,0,0,0)이고 --card·--foreground·--muted-foreground가 전부 빈 값이라 원인이 CSS 소스가 아님을 직감했고, 브라우저가 실제로 로드한 스타일시트를 fetch해 grep한 결과 --card: oklch가 없고 --brand도 없으며 옛 4테마 팔레트(#0b0f17/#000/#f1f5f9)가 그대로 들어 있었다 — 사용자가 띄워둔 3000번 dev 서버(PID 36808)가 shadcn 토큰 마이그레이션 이전부터 떠 있어 옛 번들을 계속 내보내고 있었던 것. 디스크의 globals.css는 2테마·@theme inline 온전했다. 그 상태에서 새 컴포넌트의 shadcn 유틸이 전부 무효로 해석되어 bg-card는 투명(카드 구분 소멸), border는 rgb(15,23,42) 거의 검정(선이 전부 진함), text-muted-foreground는 본문과 동일(위계 소멸), bg-accent·bg-muted는 투명이 됐고 이것이 신고 내용과 정확히 일치했다. 서버 kill 후 .next/dev·.next/cache를 지우고 재기동해 해결. 이 사고는 08-05에 이어 두 번째라 메모리(css_ui_regression_caution)에 "사용자의 시각 불만은 전제부터 검증하라 — served CSS를 fetch해 현재 소스에만 있는 토큰의 존재 여부로 stale을 판별하고, 그렇다면 재디자인이 아니라 서버를 재시작하라"는 절차와 "stale 번들에서 잰 색·대비 수치는 무의미하다(기하는 유효)"는 주의를 추가했다. 정상화 후 다시 재보니 텍스트 대비는 원래 문제가 없었고(본문 19.8) 약한 것은 구조 분리였다 — --border 0.922가 흰 카드 위에서 1.26이라 사실상 보이지 않았고 카드가 스무 개 깔리는 화면에서 서로 녹아붙었다. 0.87(1.48)로 올리고 다크는 1 0 0 / 10% → 16%(1.63)로, --muted-foreground는 0.556(4.74)이 본문 크기 기준으로는 AA를 넘지만 이 앱 보조 텍스트가 대부분 10~11px이라 부족해 0.52(5.49)로 내렸다(다크 6.94). 이어 사용자 요청("대시보드를 정말 알차고 예쁘게")으로 홈을 재구성했다. 가장 큰 문제는 지수 12종이 8.5rem 카드로 3그룹에 깔려 첫 화면 600px이 전부 지수였다는 것 — MarketHeroStrip을 신설해 매일 보는 5종(KOSPI·KOSDAQ·USD/KRW·나스닥·S&P500)만 스파크라인과 추세 점수칩을 달아 한 줄로 눕히고, 나머지는 AllIndicesSection(기본 접힘)에 그대로 남겼다(줄인 것이지 버린 게 아니다). DashboardOverview는 오늘의 점수 TOP(신규 — 이 앱만의 자산인 5축 종합 점수 랭킹)과 거래대금 상위에 각각 국내/해외 토글을 달고 오늘의 뉴스 7건과 함께 3열로, 그 아래 최신 투자 리포트 4장을 배치했다. 시장 토글을 블록마다 따로 둔 것은 의도적이다 — 하나로 묶으면 "국내 점수를 보면서 해외 거래대금을 보는" 조합이 막히는데 그 조합이야말로 대시보드에서 하고 싶은 일이다. 토글 시 로딩 상태로 되돌리는 데 setBlock(LOADING)을 이펙트에서 부르면 렌더가 한 번 더 돌고(react-hooks/set-state-in-effect) 그 한 프레임 동안 이전 시장 데이터가 새 탭 밑에 남으므로, 토글은 바깥 컴포넌트에 두고 본문만 key로 재마운트하는 구조로 바꿨다. 검증: tsc·next build 통과, eslint 신규 파일 무경고(대시보드 잔존 3건은 기존 asset-*), 브라우저 실측으로 홈 전 블록 실데이터 렌더(KOSPI 6,345.53 +0.73%·오스코텍 90 A+·삼성전자 241,000원·실제 뉴스 7건·리포트 2건)와 토글 4종 실제 전환(점수 오스코텍↔트리플플래그, 거래대금 삼성전자↔마이크론), 전체 지수 42px↔840px, 1600/1280/375 오버플로 0, 라이트·다크 양쪽 확인. 실측 함정 둘을 다시 기록했다 — document.querySelector('main')이 SSR 스트리밍용 숨은 복사본(height 0)을 먼저 반환해 "데이터가 안 온다"고 한 번 오진했고, key로 재마운트되는 카드는 DOM 노드가 교체되므로 클릭 전에 잡아둔 참조가 detached가 되어 토글이 안 먹는 것처럼 보였다(둘 다 측정 코드의 문제였고 앱은 정상)
- 2026-08-12 | 종목 분석 심화(점수 신호등·레일 위젯 3종·랭킹 보기 전환) | ok | 사용자가 "각 페이지별로 추가·수정할 컴포넌트를 먼저 물어보고 진행하라"고 진행 방식을 지정해 메모리에 저장했고, 첫 대상으로 종목 분석을 골랐다. 제안을 글로만 하지 말고 샘플을 달라고 해서 docs/mockups/analysis-widgets-v1.html을 만들어(옵션 0~6, 앱 실제 토큰, 라이트/다크 토글) 선택받은 뒤 착수했다. 목업을 만들며 기존 버그를 하나 찾았다 — --success와 --danger가 정의된 적이 없는데 13곳에서 쓰이고 있어(관리자 배치·점수 패널, 시장 브리핑, 종합 점수 전부) var()가 상속색으로 떨어져 성공/실패와 등급 색이 모두 검정으로 렌더되고 있었다(브라우저 실측으로 확인). 두 토큰을 처음 정의하고 점수용으로는 사용자가 고른 신호등 3색(--score-high 초록 / --score-mid 주황 / --score-low 빨강)을 신설했다. 빨강이 이 앱에서 상승(--profit)이기도 해서 의미 충돌을 목업에 명시했고 사용자가 알고도 신호등을 선택했으므로, 점수 색은 점수를 말하는 자리(등급 배지·점수 숫자·5축)에만 쓰고 등락률 칸은 profit/loss가 계속 주인이 되도록 주석으로 못 박았다. 같은 tone 맵이 5개 파일에 복사돼 있었고 그중 mid만 파랑(--brand)이라 화면마다 중간 점수 색이 달랐던 것도 원인이라, lib/stock-score-api.ts에 SCORE_TONE_TEXT/BG/BORDER/BADGE를 두고 전부 그걸 import하게 했다. 우측 레일에는 세 카드를 새로 만들었다. stock-daily-close-card는 최근 30거래일을 면적 차트와 표로 함께 보여주는데(사용자가 "둘 다" 선택) 차트는 흐름을, 표는 정확한 날짜·값을 맡고, 면적 색은 마지막 하루가 아니라 구간 전체 방향으로 정한다(한 달 내내 오른 종목이 마지막 날 하락으로 파랗게 되는 것을 막기 위해). stock-schedule-card는 실적·공시를 한 타임라인에 섞되 DART/Finnhub 출처 배지를 남기고 예정 건을 위로 올린다. stock-insider-card는 최근 90일 매수/매도 건수와 순증 주식 수를 요약하는데, sharesChange가 없는 행은 건수에서만 세고 순증에서 빼며(없는 값을 0으로 더하면 "변화 없음"이라는 없는 사실이 된다) 90일 컷오프는 useMemo가 아니라 응답 도착 시점에 계산한다(Date.now()를 memo 안에서 부르면 순수하지 않아 react-hooks/purity에 걸린다). 점수 5축은 사용자 결정으로 가로 막대에서 레이더로 교체했고(막대는 AI 분석 탭에 이미 있다) recharts 없이 SVG로 직접 그렸는데, 값이 없는 축을 0으로 찍지 않는 것이 핵심이다 — 커버리지가 낮은 종목에서 0은 "가치 점수가 바닥"이라는 없는 사실을 만들며 실제로 삼성전자의 심리·수급 축이 —로 비어 있었다. 랭킹 패널은 사용자가 옵션 5를 모달이 아니라 목록 보기 전환으로 재해석해 요청했으므로 간단(기본·종목+점수)과 넓게(심볼·가격·스파크라인·등락) 두 밀도를 두고 localStorage에 저장했는데, useState 초기화로 읽으면 SSR과 어긋나고 이펙트 setState로 덮으면 렌더가 한 번 더 돌아서 useSyncExternalStore로 서버 스냅샷을 따로 주는 방식을 썼다. 정렬 헤더(종목·등락·점수)와 등락 강도 배경(±2% 미만은 거의 칠하지 않음)도 함께 넣었다. 여기서 데이터 문제를 하나 발견했는데 점수 랭킹 API가 국내 종목에 changeRate와 closePrice를 아예 주지 않아 등락 칸이 전부 —이고 등락 정렬이 무의미했다 — 넓게 보기일 때만 fetchQuotesBatchClient로 보강해 해결했다(간단 보기는 등락을 쓰지 않으므로 요청하지 않는다). 메달은 기본 정렬일 때만 표시하도록 고쳤다. 등락순으로 정렬한 목록에 금메달이 세 번째 줄이 아닌 곳에 박히면 그 자체가 순위표처럼 잘못 읽히기 때문이다. 검증: tsc·next build 통과, eslint 신규 6파일 무경고(잔존 9건은 전부 이전부터 있던 파일), 대비 실측으로 신호등이 라이트 5.02/5.18/6.47·다크 10.29/7.92/6.48로 전부 AA를 넘는 것을 확인했고, 브라우저에서 레일 7카드가 실데이터로 렌더되는 것(삼성전자 73점 A·동종 상위 3%·커버리지 90%, 30거래일 -20.31%, DART 공시 2건, 내부자 매수 4·매도 26·순증 -18,093주)과 넓게 보기의 시세 보강(씨젠 32,350원 +5.20%)·히트셀 12개·스파크라인 17개, 1600/375 가로 오버플로 0을 확인했다. 남은 페이지(관심종목·보유종목, 뉴스·리포트, 기록 3종)는 같은 방식으로 목업을 먼저 만들어 선택받고 진행한다
- 2026-08-13 | API 패닉 방어 + graceful shutdown | partial | 스크린샷 임포트 테스트 크래시 근본=SSE stream hub race(폴러 Publish vs SSE cancel), OCR 아님. stream hub RLock + safego + import panic→failed + Shutdown import WaitIdle. go test PASS. Docker 재배포 미완
- 2026-08-13 | SSE stream hub concurrent map panic 수정 | partial | streamHub/quoteStreamHub Publish가 RUnlock 후 inner map range → fatal error: concurrent map iteration and map write로 API 크래시. RLock을 순회·non-blocking send까지 유지하도록 수정 + 동시 테스트 추가. go test PASS. Docker Desktop 기동 불가로 재기동 미완 — 사용자가 Docker Desktop 재시작 후 be/scripts/restart.ps1 -Background 필요
- 2026-08-13 | 스크린샷 임포트 비증권 이미지 방어 강화 | ok | 채팅/커뮤니티·UI 아티팩트 거부, RejectPreviewAfterResolve, FE failed→upload+경고, non_brokerage_chat.png Docker OCR PASS, screenshot_guard 10건 + pipeline fixture 전체 PASS, Docker API 재기동 채팅/커뮤니티·UI 아티팩트(7시간 전·검색/코드) 거부, RejectPreviewAfterResolve, FE failed→upload+경고, non_brokerage_chat.png Docker OCR PASS, screenshot_guard 10건 + pipeline fixture 전체 PASS, Docker API 재기동
- 2026-08-13 | 스크린샷 임포트 비증권 이미지 방어 | ok | RejectNonBrokerageScreenshot(보유/시세 UI·금액·수량 신호), import job per-image 거부 + preview 신뢰도 게이트, screenshot_guard_test 6건 + go test ./internal/import/... PASS, Docker API 재기동
- 2026-08-13 | 국내 ETF OCR KODEX/TIGER 분리 | ok | mergeKey hangul-only로 TIGER 누락 수정, domestic_etf_kiwoom.png fixture, go test PASS, Docker 재기동
- 2026-08-13 | 해외주식 OCR 맵러빈 resolver 동점 수정 | ok | *러빈+app*가 Applied/Apple 등과 0.86 동점→margin 실패. applovin 전용으로 좁혀 live MySQL 맵러빈→APP(앱러빈) 확인, Docker 재기동
- 2026-08-13 | 해외주식 스크린샷 OCR 5종목 파싱 | ok | 해외 보유 스크린샷에서 어플라이드 4·아이렌 55·ARM 50·앱러빈 15·오라클 40주 exact. ADR/percent-qty/UTF-8 suffix·FuzzyUnify(해외 전용). fixture overseas_holdings.png + go test ./internal/import/... PASS, Docker API 재기동 사용자가 "종목 분석 탭에서 점수 관련 숫자를 전부 초록으로 표기하는데 일반 글씨색으로 바꾸고, 종합 점수를 평가하는 수치만 신호등(0~35 빨강·35~65 주황·65~100 초록)으로 해달라"고 요청. 이 문장을 화면 전체에 대한 것으로 읽고 범위를 물어본 뒤(종합 점수는 세 자리 모두 유지, 등급은 신호등·레이더는 중립, 홈·마켓 레이더까지 동일 규칙) 5축 막대와 숫자, AxisScoreBadge, 레이더 폴리곤, 펀더멘털 종합 지표와 ScoreRing(종목 분석 밸류에이션 탭과 재무 화면 두 곳), 일별 종가 AI 점수까지 전부 중립으로 바꿨다. 그러나 사용자가 종목 랭킹 패널 스크린샷을 첨부하며 정정했다 — 자기 의견은 "종목 랭킹에서 나오는 점수 숫자를 신호등 말고 검정/흰색으로" 하나였고 나머지는 롤백하라는 것. 물어본 것 자체는 맞았지만 질문이 이미 잘못된 전제(화면 전체) 위에 서 있었고 선택지 세 개가 모두 그 전제를 공유해서 사용자가 전제를 바로잡을 자리가 없었다 — 다음에 "점수 색"류 지적을 받으면 범위를 묻기 전에 어느 위젯을 보고 하는 말인지부터 특정해야 한다. 롤백 후 남긴 변경은 둘이다: scoreTone()의 mid 하한을 45에서 35로 내린 것(첫 요청에 숫자로 명시됐고 정정 대상이 아니었다)과 종목 랭킹 행의 점수·등급만 SCORE_NEUTRAL_TEXT(text-foreground)로 바꾼 것. 중립은 상수로 남겼는데 text-foreground를 그냥 적으면 다음 사람이 색 지정 누락으로 보고 신호등으로 되돌리기 때문이고, 유일한 사용처가 랭킹 목록임을 주석에 적었다. 두 번째 요청은 랭킹 패널의 넓게 보기가 잘리는 문제였다 — 좌측 레일이 300px 고정인데 넓게 행은 순위 28 + 아바타 + 이름 + 스파크라인 54 + 등락 62 + 점수 40에 여백까지 필요해서 이름 몫으로 20px대만 남고 종목명이 한 글자로 잘렸다(스크린샷의 "R", "오", "인"). 세 가지 안(레일 420px 확장 / 행을 2줄로 / 둘 다)을 제시해 레일 확장을 선택받았다. 구현의 핵심은 보기 밀도 상태를 패널 밖으로 꺼낸 것 — 컬럼 폭은 PageColumns가 쥐고 있어서 페이지와 패널이 같은 값을 봐야 하는데, localStorage의 storage 이벤트는 다른 탭에서만 발생해 같은 화면의 두 구독자를 잇지 못한다. lib/ranking-view.ts에 모듈 스코프 구독자 집합 + useSyncExternalStore(서버 스냅샷은 compact 고정이라 하이드레이션 불일치 없음)로 작은 스토어를 만들고 PageColumns에 wideLead prop을 더했다. 검증: tsc 통과, eslint 신규 파일 무경고(잔존 4건은 전부 이전부터 있던 set-state-in-effect). 브라우저 실측으로 랭킹 90·88·A+가 라이트 lab(2.75)·다크 lab(98.26)로 본문색을 따르고 종합 점수 73은 초록(다크 rgb(74,222,128))·5축 83/79/66 초록과 52 주황·레이더 초록으로 롤백이 실제로 됐음을 확인했고, 폭은 1280에서 420px 766px ↔ 300px 886px로 토글에 맞춰 실시간 전환(스토어가 페이지까지 닿는다는 증거), 1600에서 3단 420/734/340, 375에서 단일 컬럼이며 전 구간 가로 오버플로 0, 넓게 보기에서 잘린 텍스트 0개(최장 "현대코퍼레이션홀딩스" 112px 완전 표시)를 확인했다. 마켓 레이더 지수 점수칩은 손대지 않았다
- 2026-08-13 | 보유종목 현금(원·달러) 포트폴리오 관리 | ok | 사용자가 보유종목 탭에서 원·달러 현금을 직접 입력해 총 자산에 합산하고, 「현금 추가」 버튼 위치 의견을 요청했다. BE에 user_cash_balances(041) + GET/PUT /user/cash-balances를 추가하고 매매 이력·Toss 예수금과 분리했다. FE는 우측 레일 AddHoldingCard 바로 아래 CashBalanceCard(종목 추가와 같은 맥락), holdings-calc에 cashKRW/cashUSD/totalAssetsKRW·mergeCashIntoTotals, 요약 카드는 현금이 있으면 「총 자산」+ 주식/현금 breakdown, 구성 도넛에 **자산** 탭(주식 vs 현금), 종목 비중 분모를 totalAssetsKRW로 변경. 달러 현금·달러 종목은 환율 없으면 합산 보류. go build/test, tsc, Docker 재기동(Tesseract OK)
- 2026-08-12 | 보유종목 스크린샷 가져오기 레일 이동 | ok | 사용자가 스크린샷 가져오기가 목록 아래에 있어 발견성이 떨어진다고 해서, 종목 직접 추가 바로 아래 우측 레일에 SectionCard 크기의 진입 카드를 두고 본문은 Dialog로 연도록 옮겼다(placement=rail). tsc 통과
- 2026-08-12 | 보유종목 화면 전면 개편 + 매입 이력(holding_trades) 신설 | ok | 지난 세션에 정한 방식대로 먼저 docs/mockups/holdings-widgets-v1.html(옵션 11개, 앱 실제 토큰, 라이트/다크 토글)을 만들어 선택받고 착수했다. 목업을 만들며 "데이터는 이미 있는데 화면에 없는 것" 두 건을 찾았다 — 스크린샷 임포트 마법사가 평단가를 읽어 UserHolding.avgPrice로 저장해 왔는데 목록이 그 값을 쓰지 않아 수익률도 평가손익도 화면에 아예 없었고, deleteUserHoldingClient()는 정의만 되고 앱 전체에서 호출부가 0곳이라 잘못 인식된 종목을 지우는 유일한 방법이 스크린샷 재업로드(전체 교체)였다. 또 08-11의 --success/--danger와 같은 종류의 사고를 하나 더 발견했는데, @theme inline이 --color-chart-1: var(--chart-1)을 매핑하고 있으면서 --chart-1~--chart-5 자체는 정의된 적이 없어 그대로 쓰면 도넛 조각이 전부 상속색 한 덩어리가 된다 — 라이트/다크 양쪽에 정의하면서 이 색은 범주 구분용이고 상승/하락이나 점수에 쓰지 않는다고 주석에 못 박았다(뜻이 있는 색을 무늬로 쓰면 둘 다 의미를 잃는다). 사용자 선택 중 가장 큰 것은 평가금액 추이를 "합성 곡선 말고 매입 이력 저장 기능을 먼저 만들고 진짜 이력으로"(3-C) 한 것이라, BE에 039_holding_trades.sql과 repo/service/handler를 새로 세웠다(GET/POST/DELETE /user/holdings/trades, GET /user/holdings/valuation?days=). 시계열은 날짜별 실제 보유수량을 거래 이력에서 복원해 그날 종가에 곱해 더하는데, 계산을 BE에 둔 이유는 FE에서 하면 종목 수만큼 fetch가 나가고 기기 시계마다 답이 달라지기 때문이다. x축은 종목별 거래일의 합집합으로 잡았고(한 심볼의 달력을 쓰면 미국 종목만 거래한 날이 사라진다) 값을 못 매긴 종목과 이력이 아예 없는 종목은 숨기지 않고 응답에 이름으로 담아 화면이 "이 곡선에 무엇이 빠졌는지" 말하게 했다 — 일부가 빠진 곡선을 "내 포트폴리오"라고 부르면 안 된다. 평단은 이동평균법으로, 매도는 수량만 줄이고 평단을 재계산하지 않으며(재계산하면 이익 실현이 남은 포지션의 원가를 낮춰 실제보다 잘 산 것처럼 보인다) 전량 매도 후 재매수는 평단이 새로 시작하고 과매도는 공매도가 아니라 오타로 보고 0에서 막는다 — 유닛 6건으로 고정했다. user_holdings는 이력이 있는 종목에 한해 파생 캐시(source='trades')가 되어 기존 리더(quotes held 필터·대시보드·스탬프)를 하나도 건드리지 않게 했고, 전량 매도 시 보유행만 지우고 이력은 남기되(실현손익이 거기 있다) 사용자가 보유를 삭제하면 이력까지 지운다(재추가 시 옛 원가를 상속하지 않도록). 점수 배치 API(GET /stocks/scores/batch)도 새로 뚫었는데 보유 종목 수만큼 단건 요청이 나가던 것을 1회로 줄이기 위해서고, MAX(trade_date) 조인이라 점수가 끊긴 종목도 마지막 값이 나와 목록에서 사라지지 않는다. FE에서는 손익 계산을 lib/holdings-calc.ts 한 곳에 모았다 — 위젯마다 따로 더하면 같은 화면에서 레일 요약과 표 합계가 어긋난다. 규칙 셋은 전부 "모르는 값을 0으로 그리지 않는다"의 변형인데, 평단이 없는 종목은 손익이 null이고 합계 수익률의 분자·분모에서 함께 빠지며(0으로 두면 매입가 0원짜리라 수익률이 +∞로 뛴다), 환율을 못 받았는데 달러 종목이 있으면 원화 합계를 보류하고, 시세가 없으면 평가금액이 null이다. 본문은 요약 큰 카드 1장(2-B)·평가금액 추이(실선 평가·점선 매입 — 둘의 간격이 곧 평가손익이고 평가금액만 있으면 오른 게 시장 때문인지 더 샀기 때문인지 구분되지 않는다)·구성 도넛 종목+통화(4-B)·오늘 손익 기여도(5-A, 등락률이 아니라 금액 순 — 5% 오른 1% 비중 종목보다 1% 오른 대장주가 지갑에는 큰 일이다)·보유 점수 카드(평가금액 가중 평균, 클릭 시 5축 레이더 모달)·표↔카드 전환 목록(1-C)·접힌 스크린샷 마법사(10-A, 작업이 시작되면 자동으로 펼침) 순으로 쌓았고, 레일에서는 본문과 겹치게 된 보유 요약·비중 상위·많이 움직인 종목을 뺐다 — 특히 총 평가금액을 레일에 두면 안 되는데 레일은 xl 미만에서 본문 밑으로 쌓여 이 화면에서 제일 먼저 보는 값이 맨 아래로 가기 때문이다. 남은 자리는 직접 추가·다가오는 일정·목표가·보유 종목 뉴스·지수로 채웠다. 직접 추가는 수량을 덮어쓰지 않고 매수 기록으로 저장하는데, 그래야 평가금액 추이에 그 종목이 매수일부터 들어간다(수량만 덮어쓰면 언제부터 갖고 있었는지가 어디에도 남지 않는다). 목표가 저장소는 targets-client.tsx 사설에서 lib/price-targets.ts로 승격했다 — 보유 화면도 같은 목표가를 읽는데 저장 키가 두 파일에 있으면 한쪽만 고쳤을 때 조용히 갈라진다. useSyncExternalStore 구독으로 만들어 목표가 화면에서 저장하면 보유 화면 카드도 함께 갱신되고, 그 김에 targets-client에 이전부터 있던 set-state-in-effect 1건과 mounted 플래그도 사라졌다. 이번에 반복해서 필요했던 패턴 하나를 기록해 둔다 — 로딩 boolean을 이펙트에서 동기로 켜면 캐스케이드 렌더로 린트에 걸리므로 "어떤 요청에 대한 결과인지"를 데이터와 함께 들고 loaded?.key === key로 판정하고, 목록 새로고침도 load() 호출이 아니라 버전 번호를 올려 이펙트를 다시 돌린다(이펙트에서 부른 함수 안의 setState도 이펙트 본문 것으로 취급된다). 실측 중 내 코드의 결함을 하나 잡았는데, 도넛에서 상위 6개에 색을 주고 나머지를 기타로 묶으면 6번째 색이 회색이라 기타와 붙어 구분되지 않는 회색 두 조각이 생겼다 — 이름 있는 색은 다섯 개까지로 줄이고 회색을 기타 전용으로 남겼다. 검증: go build/vet/test 전체 통과(신규 유닛 6건 포함), tsc·next build(37페이지) 통과, eslint 신규 파일 무경고(잔존 2건은 전부 손대지 않은 screenshot-crop-canvas.tsx), Docker 8컨테이너 재기동(Tesseract OK)과 039 마이그레이션 적용 확인(worker-scores 로그 + 13컬럼 실조회 — API 로그에 적용 줄이 없어 처음엔 미적용을 의심했는데 워커가 먼저 만들고 API는 이미 있는 것을 본 것이었다), scores/batch 실호출 200(5개 요청 → 4개 응답, 미존재 심볼은 빠짐, SK하이닉스 55점 B 실데이터). 화면은 로그인 게이트라 임시 하네스로 7종목 픽스처(평단 없는 종목 1개 포함)를 렌더해 총 평가 24,108,903원·매입 19,923,545원(평단 없는 카카오 제외)·손익 +2,093,358원 +10.51%이 서로 맞물리는 것과 카카오의 평단·손익이 모두 —인 것, 도넛 5색+기타, 점수 가중평균 66.0에 A3·B·C2·D1, 상세 모달 486×552 렌더, 표↔카드 전환의 localStorage 저장, 1600에서 994px 340px 2단·1280·375 전 구간 가로 오버플로 0(표는 자기 컨테이너 안에서 스크롤), 다크 토큰 정상 해석을 확인하고 하네스를 삭제했다. 참고로 Next 16에서 언더스코어로 시작하는 라우트 폴더는 private이라 404가 나므로 하네스 경로 이름에 주의해야 한다. 남은 것: 로그인 세션이 없어 실데이터 화면은 육안 미확인이고, 매입 시점 환율(fx_rate)은 컬럼만 두고 입력 UI가 없어 환차손익 분리는 여전히 불가하다
- 2026-08-14 | 관심종목 탭 UI 개편 (행 재설계 + 툴바 + 퀵룩 모달 + 레일 3장) | ok | 사용자가 "메뉴마다 가시성과 데이터를 보완하고 싶다, 관심종목부터, 작업 전에는 항상 html 샘플과 선택지를 달라"고 요청했다. 코드를 먼저 읽고 목업 docs/mockups/watchlist-widgets-v1.html(현재 행 + A/B/C 행 리디자인 + D 툴바 + E 퀵룩 모달 + F 레일, 앱 실제 토큰·라이트/다크 토글)을 만들어 제시했고 C안(스파크라인 제외, 모바일에서 2줄로 접히지 않게)+D+E+F를 선택받았다. 목업을 만들며 "데이터는 이미 있는데 화면에 없는 것"을 찾았는데 GET /watchlist가 addedPrice·createdAt을 내려주고 있었는데도 watchlist-client가 .symbol만 뽑고 나머지를 버려서, 관심종목의 핵심 질문인 "담고 나서 어떻게 됐나"가 화면 어디에도 없었다 — 행의 「담은 뒤」 열과 레일 WatchlistSinceAddedCard로 살렸고 옛 항목은 값이 없어 —로 둔다(수익이라 부르지 않고 관찰 기록으로 표현). 행은 5칸 동일 무게 지표를 점수 배지·52주 레인지 바·기간 3칸·담은 뒤·현재가·임박 일정 점으로 재구성해 높이를 100px에서 62px로 줄였다. 가장 중요한 발견은 브레이크포인트를 뷰포트로 잡으면 안 된다는 것이었다 — 처음에 sm:/md:/lg:로 짜고 실측했더니 컨테이너 520~700px 구간에서 행이 가로로 넘쳤는데, 이 폭이 바로 1280px 화면에서 좌측 사이드바와 우측 레일 300px을 뺀 실제 본문 컬럼 폭이다. 즉 뷰포트 기준으로는 "데스크탑이니 다 보여줘도 된다"고 판단하지만 정작 컬럼에는 자리가 없다. Tailwind v4 컨테이너 쿼리(@container + @2xl/@3xl/@5xl)로 바꾸고 WatchlistRangeBar에는 dense prop을 두어 모달(넓음)과 행(좁을 수 있음)이 같은 컴포넌트를 쓰되 라벨 노출 기준만 다르게 했다. 현재가 칸은 고정 폭이 삐져나오는 원인이어서(1,234,500원류는 nowrap이라 잘리지 않고 넘친다) min-width로 바꿨는데, 오른쪽 정렬이라 폭이 달라도 끝선은 모든 행에서 같고 남는 자리는 종목명이 가져간다. 퀵룩 모달은 행 클릭이 /stocks/{symbol}로 페이지를 떠나던 것을 대체하는데, 관심종목을 보는 일은 하나를 파고드는 게 아니라 여럿을 견주는 일이라 그 이동이 비쌌다 — 40일 종가·52주 위치·담은 뒤·5축 점수·다가오는 일정·최근 뉴스·보유에 추가 폼(createHoldingTradeClient 매수 기록)을 담고 데이터는 열 때만 조회한다. 툴바는 정렬을 저장하지 않는 보기 모드로 만들고 「내 순서」에서만 드래그 핸들을 노출했는데, 정렬된 화면에서 드롭을 받으면 화면 순서가 그대로 저장돼 내가 정한 순서가 조용히 덮어써지기 때문이다. 요청 수 관리로는 점수를 GET /stocks/scores/batch 1회로 묶고 5초 폴링에서 뺐으며(하루 한 번 산출되는 값), 실적 일정은 종목당 1요청이라 useUpcomingEvents로 상한 12를 두고 행 표식과 레일 카드가 결과 하나를 나눠 쓰게 했다 — 이때 조회 대상을 visibleRows로 잡으면 등락률 정렬에서 5초 폴링마다 상위 12종목이 바뀌어 실적 API를 열두 번씩 다시 부르게 되므로 내 순서 기준으로 고정했다. 뉴스 모달은 보유종목 것을 복사하지 않고 components/stocks/symbol-news-modal.tsx로 승격해 제목·수식어만 프롭으로 받게 하고 HoldingsNewsModal을 얇은 래퍼로 남겼다. 검증: tsc·eslint 통과(신규 파일 무경고). 로그인 세션이 없어 실데이터 화면은 임시 하네스로 확인했다 — 컨테이너 278~1198px 전 구간에서 행 높이 62px 고정에 가로 오버플로 0, 375px 모바일도 1줄(종목명·점수·52주 %·현재가+등락률), 퀵룩 모달 486×602에 캔들/실적/뉴스 API 전부 200과 실제 헤드라인 렌더까지 확인 후 하네스를 삭제했다. 브라우저 패널이 표시되지 않아 스크린샷은 못 찍고 DOM 실측으로 대신했다. 남은 것: 로그인 상태의 실화면 육안 확인, 그리고 목업 F안의 레일 3장은 넣었지만 릴리스 노트는 작성하지 않았다
- 2026-08-14 | 관심종목 행 정돈(A안) + 종목명 깜빡임 수정 + 모달 차트 공용화 | ok | 사용자가 앞 세션 결과 스크린샷을 붙이며 셋을 지적했다 — 행 디자인이 이상하다, 종목명이 5초마다 종목코드로 바뀌었다 돌아온다, 모달 차트가 변동만 크고 금액대가 없어 안 읽힌다(보유종목에서도 쓰는 것 같으니 같이). 깜빡임은 선택지가 없는 버그라 먼저 원인을 팠는데, BE `zeroQuoteSummary`가 `toss.GetStock` 실패 시 `Name`에 **심볼을 그대로** 넣어 응답하고 있었다. 빈 문자열이 아니라서 FE `mergeQuoteSummary`의 `!next.name && prev.name` 가드를 그대로 통과해 5초 폴링 중 한 번만 실패해도 진짜 이름이 코드로 덮어써졌다. `isPlaceholderName`(빈 값 또는 name===symbol)으로 판정해 마지막 정상 이름을 지키게 했고 `market`·`logoUrl`도 같은 이유로 가드를 더했다. BE는 손대지 않았는데 `Name`이 비어 있지 않다고 가정하는 다른 소비자가 있을 수 있고, 마지막 정상값 유지는 원래 이 병합 레이어가 존재하는 이유이기 때문이다. 나머지 둘은 목업(docs/mockups/watchlist-row-v2.html)으로 행 3안·차트 2안을 제시해 A안(표 헤더)+차트 2안을 선택받았다. 행 문제를 뜯어보니 넷이었다 — 기간 3칸에 136px를 나눠 45px씩이라 +16.85%(48px 필요)가 잘렸고, 52주 저·고가가 9.3만/38만 압축 표기라 자릿수를 다시 세야 했고, 레인지 칸만 3줄이라 세로 기준선이 없었고, 「전일·전주·전월·담은 뒤」 라벨이 종목 수만큼 인쇄돼 숫자보다 라벨이 많았다. A안은 라벨을 목록 맨 위 헤더 한 줄로 올리고 행에는 값만 남기는 것이라 행이 62→48px로 낮아졌는데, 이때 헤더와 행의 칸이 1px이라도 어긋나면 표가 아니라 그냥 어긋난 두 줄이 되므로 폭·접힘 규칙을 watchlist-columns.tsx의 WL_COL 한 곳에 두고 양쪽이 같은 상수를 보게 했다. 실측하며 정렬 결함 둘을 잡았다 — 헤더가 행보다 정확히 1px 왼쪽에 섰는데 행은 Card라 테두리 1px이 더 있어서였고(헤더 패딩을 px-[11px]로), 현재가를 min-width로 두니 행마다 값 길이가 달라 그 앞 칸들이 밀려 헤더의 점수·52주가 행의 점수·52주 위에 서지 않았다(고정 폭 w-24/@2xl:w-28로). 값이 계단처럼 어긋나는 것은 WatchlistCell이 값 18px·보조 12px로 높이를 고정하고 보조 줄이 없는 칸도 자리를 비워 해결했다. 레인지 바의 파랑→빨강 그라데이션은 없앴는데 이 앱에서 그 두 색은 하락·상승이라는 뜻을 이미 갖고 있어 무늬로 쓰면 둘 다 의미를 잃기 때문이고, 중립 트랙에 브랜드색 채움 하나만 남겼다. 차트는 components/charts/price-history-chart.tsx로 새로 만들어 관심종목 퀵룩과 보유종목 상세가 함께 쓰게 했다 — 이전 스파크라인은 세로를 늘 최저~최고로 꽉 채워 0.5% 등락도 40% 급등과 똑같이 극적으로 보였고 금액이 없어 그 급등이 몇 %인지 알 방법이 없었으며 선이 상자 가장자리에 닿아 잘렸다. 최고·최저 금액 축과 위아래 여백, 면적, 기간 시작가 회색 점선, 끝점 도트, 우상단 기간 변화율을 더했고 보유 쪽은 여기에 평단 점선+라벨을 얹어 140×44 미니 스파크라인을 대체했다(그 크기에는 축을 넣을 자리가 없어 점선이 무엇인지 화면이 말하지 못했다). 금액 라벨을 SVG text가 아니라 옆 HTML 칸으로 뺀 이유는 preserveAspectRatio="none"으로 가로를 늘리면 text도 찌그러지기 때문이고, 세로 배율이 1이라(뷰박스 높이=CSS 높이) 뷰박스 y를 그대로 px로 써서 라벨 위치를 맞췄다. 실데이터로 확인하다 내 문구의 거짓을 하나 잡았는데, 하단에 "40거래일"이라고 썼지만 일별 종가 커버리지가 성근 종목은 40개 봉이 12-16~08-14 8개월에 걸쳐 있었다 — 세는 것은 봉의 개수이므로 "종가 40개"로 바꾸고 기간은 양옆 날짜가 답하게 했다. useSparklineCandles는 축 날짜용으로 times도 반환하게 넓혔고, 그 김에 이전부터 있던 set-state-in-effect를 {key, points, times} 패턴으로 없애 loading을 파생값으로 만들었다. 검증: tsc·eslint 통과(잔존 1건은 손대지 않은 market-radar-top5-chart.tsx). 로그인 게이트라 임시 하네스로 실측 — 컨테이너 278~1198px 전 구간 행 48px 고정에 가로 오버플로 0, 340~1200px 전 폭에서 헤더와 모든 행의 모든 칸 좌우 좌표가 완전 일치, 값 8칸이 전부 top 92px 같은 선, 375px에서 헤더 「종목·점수·52주·현재가」에 1,398,500원까지 1줄, 모달 486×931에 실데이터 차트(274,500원/94,400원 축 · +165.47%)를 확인하고 하네스를 삭제했다. 브라우저 패널이 표시되지 않아 이번에도 스크린샷 대신 DOM 실측이다
- 2026-08-14 | 관심종목 행 전폭 사용 + 52주 위젯 원복 | ok | 사용자가 스크린샷과 함께 "종목카드 전체를 안 쓰고 60%만 쓰는 게 이상하다, 52주 위치는 이전 버전이 더 예쁘니 되돌려라(높이 높아져도 됨), 질문하지 말고 바로 진행"이라고 지적했다. 빈 공간의 원인은 앞 턴에서 헤더 정렬을 맞추려고 모든 칸을 고정 폭으로 잡은 것이었다 — flex 컨테이너의 남는 폭이 전부 마지막 칸(현재가) 오른쪽에 몰려 넓은 화면에서 카드의 40%가 비었다. WL_COL을 flex-[n] 비율 + min-w 바닥 조합으로 바꿔 칸들이 함께 늘어나게 했고, 헤더가 같은 상수를 쓰므로 늘어나도 정렬이 유지된다(1200px에서 종목 187·52주 209·기간 각 110·현재가 143). 52주 위젯은 저·고가 값 / 그라데이션 트랙+마커 / 「상단 63%」 3줄 형태로 되돌렸는데, 앞 턴에 그라데이션을 뺀 근거(파랑·빨강은 이 앱에서 하락·상승이라는 뜻이 있다)는 여전히 맞지만 사용자가 명시적으로 이전 쪽이 예쁘다고 판단했으므로 그 결정을 주석에 남기고 원복했다. 행 높이는 48→66px로 늘었고 사용자가 허용한 범위다. 실측 중 정렬 함정을 하나 잡았는데, 칸 상수에 접힘용으로 들어 있는 @2xl:block이 WatchlistCell이 붙인 flex를 이겨서 justify-center가 조용히 무효가 되고 있었다 — 그래서 전일·전주·전월·담은 뒤만 값이 6px 위로 붙어 있었다. 가운데 정렬을 패딩(py-[6px])으로 바꿔 display가 무엇이든 같게 동작하게 했고, 점수 배지도 22px에서 19px로 줄여 WatchlistScoreCell로 값 줄에 앉혔다(22px 배지를 44px 칸 정중앙에 두면 옆 숫자들보다 5px 내려간다). 검증: tsc·eslint 통과, 임시 하네스로 컨테이너 300~1200px 전 구간에서 헤더와 모든 행의 정렬 어긋난 칸 0·가로 오버플로 0·행 높이 66px 고정, 값 줄이 전부 top 6px, 375px에서 카드 351px에 마지막 칸과 휴지통 사이 빈 공간 0(정상 gap 6px)을 확인하고 하네스를 삭제했다
- 2026-08-15 | 목표가 useSyncExternalStore 무한 루프 수정 | ok | `/targets` 런타임 오류 "The result of getServerSnapshot should be cached to avoid an infinite loop" — `usePriceTargets`의 세 번째 인자 `() => []`가 매 호출마다 새 배열을 반환해 React가 무한 루프로 감지. 모듈 스코프 빈 배열(`serverSnapshot`)과 `getServerSnapshot`으로 고정 참조 반환
- 2026-08-15 | 국내주식·해외주식(뉴스) 화면 개편 + 뉴스 종목 태깅 파이프라인 수정 | ok | 사용자가 "메뉴마다 가시성과 데이터를 보완하고 싶다, 국내주식·해외주식부터, 작업 전에는 항상 html 샘플과 선택지를 달라"고 요청했다. 사이드바를 읽어 두 메뉴가 뉴스 그룹의 /news/domestic·/news/us이고 NewsClient 하나를 market prop만 바꿔 공유한다는 것을 확인한 뒤 목업 docs/mockups/news-market-widgets-v1.html(A 본문 상단 3안 / B 필터 2안 / C 행 3안 / D 모달 3안 / E 레일 4안, 앱 실제 토큰·라이트 다크 토글)을 만들어 제시했고 A1·A2·A3 / B1·B2 / C1·C2 / D1·D2 / E 전부를 선택받았다. 사용자가 A3에 LLM을 쓰고 싶다는 것과 비로그인 유도를 함께 요청해서 2차 목업(news-market-widgets-v2-llm-login.html)으로 LLM 입력 범위 4안·출력 스키마 3안·주기 3안과 비로그인 처리 6안을 제시했고 L2+직전 브리핑 10건 / 출력 최대한 길게+모달 / 장중 30분·그 외 1시간 / G1·G4·G6을 선택받았다. 착수 전에 실데이터를 확인한 것이 이번 세션의 갈림길이었는데, A1(오늘의 이슈 종목)과 C1(행 종목 칩)이 기사의 symbols에 의존하는데 라이브 API를 재보니 국내 최근 100건 중 6자리 종목코드가 붙은 것이 2건뿐이고 그 둘마저 연예 기사였으며(김종국 재산설·최고민수 삼전닉스) 해외는 73건 중 51건이 Visa, 37건이 Mastercard로 태깅돼 있었다(리플·레딧 기사에 붙어 있었다). 즉 두 위젯의 전제가 깨져 있었다. 원인은 세 겹이었다. 첫째로 국내 이름 인덱스를 rebuildNameIndex가 trackedSymbols() 즉 누군가 관심·보유에 담은 종목으로만 만들고 있어서 아무도 담지 않은 종목은 기사 제목에 이름이 나와도 영원히 태깅되지 않았다 — ListDomesticNameCatalog()로 asset_type='domestic' 전 종목(4,406개)을 읽게 바꾸고 poll tick마다 MySQL을 다시 읽지 않도록 30분 TTL을 뒀다. 둘째로 해외는 addUSNameIndexKeys가 idx[lower(sym)]=sym으로 티커 자체를 이름 인덱스 키에 넣고 extractUSSymbols가 경계 없는 strings.Contains로 맞춰서, 티커 V가 키 "v"가 되어 알파벳 v가 하나라도 든 모든 기사에 Visa가 붙었다 — 티커 매칭을 usTickerTokenMatch(원문 대문자·양쪽 경계)로 분리하고 3자 미만 티커는 자유 텍스트에서 받지 않게 해 $V·NYSE: V 같은 명시 표기(ExtractCashtags·ExtractExchangeTickers)만 통과시켰다. 셋째가 가장 결정적이었는데, 인덱스도 매처도 고쳤고 단위 테스트도 통과하는데 재태깅을 돌려도 국내가 여전히 0건이었다. 스윕 시작 시점 프로브는 [005930]을 돌려주는데 레코드별로는 전부 []여서 한참 헤맸고, 결국 레코드 경로에만 있는 차이가 rec.Summary를 붙인 text뿐임을 확인하고 로그를 레코드 단위로 찍어 direct=[005930] next=[] 를 잡았다 — normalizeItem과 retagRecord가 이름 매칭 결과를 marketdata.MergeTickerSymbols에 통과시키는데 이 함수는 넘겨받은 모든 값을 isLikelyTicker(1~5자 영문 대문자)로 거르므로 숫자가 든 국내 6자리 코드가 통째로 삭제되고 있었다. 즉 수집 경로도 처음부터 국내 심볼을 버려 왔다. mergeTickerCandidates를 만들어 기존 심볼은 재검사하지 않고 새로 들어오는 라틴 후보만 티커 규칙으로 거르게 했다. 여기까지 고치자 국내 18/100이 되었는데 정밀도 문제가 둘 남았다 — 링크드(193250)가 "링크드인 채용 사기"에, 디바이스(187870)가 일반 문장에 붙었다. 한글은 조사가 이름에 바로 붙어서 뒤쪽 경계를 요구할 수 없는데(삼성전자가·삼성전자는), 아무 한글이나 허용하면 이름이 더 긴 단어의 머리로 걸린다. 그래서 뒤에 올 수 있는 음절을 조사 화이트리스트로 좁히고, 업계 일반명사와 겹치는 상호(디바이스·시스템·솔루션·바이오 등)를 denylist에 넣어 종목코드가 본문에 같이 나올 때만 인정하게 했다. 그런데 조사 규칙만으로는 두산에너빌리티가 걸릴 때 두산도 같이 걸리는 문제가 남는데 "에"가 조사이기도 하기 때문이다 — extractSymbols에 최장 일치를 넣어 더 긴 이름이 같이 매치되면 짧은 쪽을 버리게 했다. 최종 실측은 국내 0→18/100(SK하이닉스·삼성전자·한화오션·두산에너빌리티·한국항공우주·현대모비스 등 전부 정상), 해외 Visa 51→1이다. 재태깅은 기동 시 1회 돌고 news:retag:v1:{market} Redis 표식으로 재기동마다 반복되지 않게 했는데, 여기서 또 한 번 막혔다 — 스윕이 MySQL을 15,774건 고쳤는데 화면은 그대로였다. loadFromStorage가 Redis 스냅샷이 있으면 DB를 아예 보지 않기 때문이고, 스윕 후 시장별 스냅샷 키를 지우고 다시 적재하는 reloadAfterRetag를 붙여 해결했다. 최종 스윕은 국내 59,663건 스캔 29,215건 수정, 해외 13,108건 스캔이다. A3 브리핑은 BE에 news_briefings(042) + repo + service + 스케줄러 + GET /news/{market}/briefing(읽기 전용)로 세웠다. 설계에서 가장 중요했던 제약은 stock_summary.go에 못박힌 지출 규칙인데 — LLM 호출 진입점은 예약 배치와 관리자 수동 실행 둘뿐이고 일반 조회는 캐시가 아무리 낡았어도 LLM을 부르지 않는다 — 이것을 지켜야 하루 비용이 방문자 수가 아니라 배치가 계획한 건수와 같아진다. 그래서 조회 엔드포인트는 저장된 것만 읽고 생성 기능이 없으며, 브리핑이 아직 없으면 카드를 아예 그리지 않는다. 프롬프트 입력은 헤드라인 40건(매체당 최대 5건으로 제한해 통신사 하나가 그날의 성격을 혼자 정하지 않게 했다) + 지수 5개 + 언급 상위 종목 10개(시세 포함) + 속보 수 + 세션 상태 + 직전 브리핑 10건이고, 직전 브리핑은 전문이 아니라 150자 압축본으로 넣었다. 출력을 "최대한 길게"로 잡으면 그 긴 출력이 다음 10회의 입력으로 누적되어 회당 1만 토큰을 넘고 기존 AI 요약 배치(하루 62만)와 합쳐 100만 한도를 넘기기 때문인데, 압축본이면 회당 5천 이하로 떨어져 하루 30만 수준이 된다. 이 계산을 사용자에게 표로 제시하고 압축 history + 새 기사 0건이면 스킵을 선택받았다. 슬롯은 벽시계 기준(장중 30분·그 외 1시간)으로 잡아 07:00·08:00·09:00·09:30처럼 예측 가능하게 하고 같은 슬롯은 재기동해도 한 번만 돌게 했으며, 비용은 슬롯 수가 아니라 직전 실행 이후 새 기사가 있는지로 정해진다 — 기사가 늘지 않는 야간·주말이 사실상 공짜가 되는 지점이다. 캘린더에는 SessionPhase/InRegularSession을 새로 뺐는데 기존 shouldPollStatic은 개장 5분 전을 장중으로 쳐서 주기 판단에는 맞지 않았다. 실제 생성을 확인하다 결함을 하나 잡았는데 국내는 source=llm으로 잘 나왔지만 해외가 매번 규칙 기반으로 떨어졌다 — 두 시장이 연달아 호출되어 제공자 최소 간격 4초에 걸린 것이라 스케줄러가 시장 사이에 Limits().MinInterval+1초를 기다리게 했다. LLM이 꺼졌거나 실패하면 규칙 기반 문장으로 자동 대체하고 카드에 "규칙" 배지를 단다. FE는 lib/news-derive.ts에 파생 집계를 모으고(언급 상위·24시간 흐름·시간 묶음·출처 분포·제목 유사도 클러스터) 전부 이미 받아 둔 기사에서 계산해 추가 요청을 0으로 뒀는데, 같은 화면의 숫자를 서버에 다시 물으면 집계와 목록이 어긋날 수 있어서다(레일의 오늘의 키워드가 이미 쓰던 규칙). 시세는 useSymbolQuotes 하나를 이슈 스트립·행 종목 칩·레일 내 종목 뉴스·모달 관련 종목이 공유해 화면당 배치 요청 1회로 묶었다. 비로그인 처리는 세 곳에서 같은 원칙을 쓴다 — 내 종목 pill은 인기 종목 3개로 필터가 진짜로 걸리고 pill과 안내 띠 두 곳에 예시임을 적으며(잠긴 버튼은 나중에가 되지만 써 본 기능은 내 걸로 바꾸고 싶다가 된다), 모달 액션은 흐리게 하지 않고 누르면 이 기사 기준으로 무엇이 저장되는지 보여주며(스탬프: SK하이닉스 1,667,000원을 봉인), 로그인 링크에 ?next=/news/domestic?article={id}로 기사 id를 실어 보내 로그인 후 보던 모달로 돌아오게 했다. D2 클러스터는 제목 토큰 자카드 0.34로 잡되 12시간 창을 더했는데, 유사도만 보면 "업비트, ALLO 입출금 중단"과 며칠 전 "빗썸, POL 입출금 중단"처럼 문장 틀만 같은 기사가 묶이기 때문이고, 정확도가 완벽하지 않아 화면에는 "관련 있어 보이는 기사"로 적었다. E4 매크로 카드는 목업의 미 10년물·달러인덱스·WTI 대신 실제로 수집되는 VIX·필라델피아 반도체·나스닥100 선물·금·비트코인으로 채웠다 — 없는 줄을 만들어 두지 않기 위해서다. E1 다가오는 일정도 매크로 지표 일정(FOMC·CPI) 소스가 없어 실적·공시만 담았다. 검증: go build/vet 통과, go test는 internal/import/pipeline의 TestParseHoldingsTextOverseasMergedPasses 1건 실패인데 git diff로 internal/import/ 변경 0줄임을 확인한 이전부터 있던 실패다(OCR 이름 파싱, 이번 작업과 무관). tsc 통과, eslint 신규 파일 무경고(use-news-symbols는 loaded?.key 패턴으로 set-state-in-effect를 없앴고, 잔존 건은 news-client에 원래 있던 것과 손대지 않은 파일들). Docker API 재기동(Tesseract OK) + 042 마이그레이션 적용 로그 확인. 브라우저 실측으로 국내 화면에서 브리핑 카드(강세 배지·4지표·전체 분석 읽기)·이슈 스트립(SK하이닉스 +4.65% 3건 등 실시세)·뉴스 흐름·동적 종목 pill·시간 구분선·레일 5장을 확인하고, 내 종목 pill 클릭 시 99건→4건 필터와 "지금은 인기 종목 기준입니다 — 삼성전자 · SK하이닉스 · NAVER" 배너, 모달의 관련 종목 패널·스탬프 팝오버·?article= 포함 로그인 링크, 375px 가로 오버플로 0(스트립 3열), 해외 화면의 규칙 배지·매크로 카드·번역 완료 토글, data-theme=dark 토큰 정상 해석을 확인했다. 실측 중 함정이 하나 있었는데 앱 전역에 main이 2개 렌더되어(watchlist 등 손대지 않은 페이지도 동일) 첫 번째 사본에 클릭이 먹지 않아 한동안 상태가 안 바뀌는 줄 알았다 — 두 번째 인스턴스에서는 정상이고 이 중복 자체는 이번 변경과 무관한 기존 현상이다. 브라우저 패널이 표시되지 않아 이번에도 스크린샷 대신 DOM 실측이다. 남은 것: 로그인 상태 실화면 육안 확인, 릴리스 노트 미작성, main 2개 중복 현상 원인 미규명
- 2026-08-15 | 뉴스 코인리더스 필터 + 지수 카드 필터 통일 | ok | 코인리더스 RSS는 주식·증시 키워드 없으면 ingest 차단(hasStockMarketKeyword, crypto 단독 제외). 지수 pulse 카드는 기사 N건 표시·클릭 시 지수 키워드 필터(종목 카드와 동일). go test TestPassesStockRelevance*, tsc, Docker API 재기동
- 2026-08-15 | 뉴스 화면 레이아웃 정리 (국내·해외) | ok | 「내 종목」 pill을 전체 다음 2번째로 옮기고 다른 pill과 동일 색상. 우측 레일 지수 흐름 제거 후 중앙 NewsIssueStrip에 지수 5개를 이슈 종목과 같은 카드 형태로 선행 배치(pickMarketIndices 공유). 레일 순서를 오늘의 키워드→다가오는 일정→많이 본 뉴스→내 종목 뉴스→출처 분포→최신 투자 리포트로 통일(해외 NewsMacroCard 제거). tsc 통과
- 2026-08-15 | 기사 모달 관련 종목/지수 카드 통일 + 미니 차트 정렬 버그 | ok | 사용자가 스크린샷과 함께 네 가지를 지적했다 — 애플 카드의 차트가 현재가와 안 맞는다, 관심·메모·스탬프 버튼을 카드 상단에 넣고 우상단 점수가 무엇인지 처음 보는 사람도 알게 해라, 아마존·애플 기사인데 애플 카드만 있다, 그리고 이런 모달·지수 카드를 쓰는 곳을 전부 통일하고 종목 클릭은 AI 추천 랭킹으로 보내라. 차트 버그는 데이터가 아니라 정렬 문제였다 — useSparklineCandles의 takeCloses가 slice(-40)으로 끝에서 40개를 집는데, 일봉 응답의 순서가 출처마다 다르다. DB 경로는 repository에서 오름차순으로 뒤집어 주지만 Toss 일봉 tail은 최신이 앞인 내림차순으로 오고(BE newestCandleIndex의 주석이 "descending from the Toss daily tail"이라고 같은 사실을 적고 있다), 그래서 최신 40개가 아니라 가장 오래된 40개를 왼쪽부터 거꾸로 그리고 있었다. 실측으로 AAPL 스냅샷 200건의 첫 원소가 2026-08-14이고 마지막이 2025-10-28인 것을 확인했고, 화면의 12-23~10-28이라는 거꾸로 된 축이 그 증거였다. candleTimeSortKey로 정렬한 뒤 자르도록 고쳤는데 이건 캔들스틱 차트가 이미 쓰던 방식이라(compareChartTime 정렬) BE 계약을 건드리지 않는 쪽이 맞다고 봤다. 교정 후 06-18~08-14 / $275~$340로 현재가 $305.78과 맞는다. UI는 StockMiniCard·IndexMiniCard를 새로 만들어 같은 껍데기를 쓰게 했고, 액션을 카드 안으로 넣은 것이 단순한 배치 변경이 아니었다 — 예전 모달 하단의 관심·메모·스탬프는 symbols[0]에만 작용해서 아마존·애플 기사에서 관심 추가를 누르면 애플만 담겼고 화면 어디에도 그 말이 없었다. 이제 버튼은 자기 카드의 종목에만 붙고, 종목이 하나도 없는 기사에서만 기사 단위 메모·스탬프를 아래에 남긴다. 점수 배지는 StockScoreChip으로 뽑아 종합점수 30/100 C에 ⓘ 팝오버(5축 이름·"매매 신호가 아닙니다"·축별 보기 링크)를 달았는데, 예전에는 숫자 하나에 title 속성만 있어서 마우스를 올려야 보이고 모바일에는 아예 없었다. 퀵룩 모달의 사설 배지도 같은 것으로 교체했다. 탭을 없애고 카드를 전부 펼친 뒤 지수 카드를 앞에 세웠고(articleDashboardIndexIds), 점수는 useStockScores 배치 1요청·일정은 종목당 1요청으로 묶었다. 실측에서 세 가지가 더 나왔다. 첫째로 아마존 카드가 "시세를 불러오지 못했습니다"에 통화까지 원화로 떨어졌는데, allArticleSymbols의 60종목 상한 밖이라 화면 공유 배치에 없던 것이라 패널이 빠진 것만 한 번 더 부르게 했다. 둘째로 아마존 태그 자체가 DB에 없었다 — 미국 이름 인덱스에서 AMZN의 마스터명이 Amazon.com이고 법인 접미사 목록에 .com이 없어 키가 amazon.com 하나뿐이라 "Amazon rising"이라 쓴 기사는 전부 놓치고 있었다(Apple·MSFT·META·NVDA·INTC는 멀쩡해서 더 안 보였다). DB에서 Amazon과 Apple이 함께 나오는 10건을 뽑아 AMZN이 붙은 것이 본문에 Amazon.com이 그대로 적힌 SEC 공시 1건뿐임을 확인하고 trimHeadlineNameTail(.com 등 도메인 꼬리 + alphabet a 같은 클래스 한 글자)을 넣었다. 재태깅 표식을 v2로 올려 스윕을 다시 돌렸고 us 13,503건 중 335건이 수정되어 문제의 Fortune 기사가 ["AAPL"]에서 ["AAPL","AMZN"]이 됐다. 셋째로 지수 판정이 영어 원문만 보고 있었는데 해외 기사는 화면에 한국어 번역이 뜬다 — "Seoul's best day ever"의 번역문이 「서울 증시가 역대 최고일을 기록」인데도 코스피가 안 붙던 이유라 FE articleText에 headlineKo·summaryKo를 포함시키고 코스피 규칙에 서울 증시·seoul stocks를 BE·FE 양쪽에 추가했다. 링크 목적지는 analysisSymbolHref(symbol,"ai")로 통일했고(뉴스 칩·대시보드 랭킹·보유 미리보기·목표가 카드·관심 레일) 「터미널」·「일봉」이라고 적혀 있는 링크만 /stocks/{symbol}로 남겼다 — 라벨이 목적지를 약속하고 있으면 그 약속을 지키는 쪽이 맞다. 마지막으로 지수 차트가 KOSPI 7,096.89를 「7,097원」으로 찍고 있어서 PriceHistoryChart에 valueFormat="plain"을 더했다. 검증: go build/test(TestAddUSNameIndexKeysTrimsDomainAndClassTails·TestTrimHeadlineNameTailKeepsOrdinaryNames 신규), tsc·eslint 신규 파일 무경고·next build 통과, Docker 재기동(Tesseract OK) 후 재태깅 로그 확인(domestic 59,972/15, us 13,503/335). 브라우저 실측으로 문제의 기사에서 KOSPI·애플·아마존 3카드와 각 카드의 액션·점수 팝오버·/analysis?symbol=…&tab=ai 링크를 확인했다. 실측에서 한참 헤맨 함정이 하나 있는데 지수 카드가 끝까지 안 떠서 코드를 계속 뒤졌지만, useMarketIndices의 load가 isPageVisible() 가드로 시작하고 브라우저 패널이 표시되지 않은 상태에서는 visibilityState가 hidden이라 지수 fetch가 아예 나가지 않는 것이었다 — visibilityState를 강제로 visible로 덮고 visibilitychange를 쏘자 바로 나왔다. 다음에 숨겨진 패널에서 검증할 때 같은 함정을 다시 밟지 않게 AGENT_STATE에도 적어 뒀다. 남은 것: 로그인 상태에서 관심 추가·스탬프 실동작 육안 확인(로그인 게이트), 릴리스 노트 미작성
- 2026-08-15 | 스탬프 화면 개편 + 스탬프 v2(예상 주가 필수·자유 만기·조기 적중) | ok | 사용자가 "메뉴마다 가시성과 데이터를 보완하고 싶다, 이번엔 스탬프 페이지, 착수 전에는 항상 샘플과 선택지를 달라"고 요청했다. 코드를 먼저 읽어 보니 데이터는 이미 풍부한데 화면이 거의 안 쓰고 있었다 — GET /stamps 한 번에 스탬프별 returns[1D/1W/1M/3M](수익률·벤치마크·알파), verdict, conviction, tags, snapshot.breakdown, context가 전부 실려 오는데 화면에는 적중률·평균 알파 두 줄과 관점 분포 막대뿐이고 나머지는 행을 펼쳐야 나왔다. 목업 docs/mockups/stamps-widgets-v1.html(A 성과 스트립 3안 / B 목록 행 3안 / C 상세 2안 / D 개봉 모달 / E 레일 6안 / F 벤치마크 비교)을 만들어 제시했고 A1(단 적중률 클릭 시 A2를 모달로) / B1+도장 실링 / C1+D / E2·E3·E6을 선택받았다. 그런데 B 선택에 "예상 주가를 필수로 받고 검증 기간을 이번 장 종료·다음 장 종료·1주·1개월·3개월·임의 지정으로 자유화하라, 스키마가 바뀌니 기존 값은 전부 삭제하라"가 붙어 있었고 이건 스키마·판정·생성 흐름까지 연쇄되는 변경이라 2차 목업(stamps-widgets-v2-target-price.html)으로 생성 흐름 2안·판정 규칙 3안·스키마 변경안을 제시했다. 판정은 2-C(방향으로 적중/빗나감을 가리고 목표가 오차를 별도 축으로) + 장중 도달 시 적중 종료, 생성은 1-A(폼 먼저), 기존 9건은 TRUNCATE를 선택받았다. 승인된 1-A 폼에 관점 버튼이 없다는 것을 발견하고 한 번 더 물어 예상 주가에서 파생하는 쪽을 택했는데, 이게 결과적으로 이번 설계의 중심이 됐다 — stance를 target_price vs 당시가에서 파생하면(±3% 관망) "약세인데 목표가는 위" 같은 모순이 구조적으로 불가능해지고 입력도 하나 준다. 밴드 상수는 model.StampNeutralBandPct 하나만 두고 판정 쪽 neutralBandPct가 그걸 재사용하게 했는데 두 개면 관망으로 접수해 놓고 다른 밴드로 채점하는 일이 생긴다. 만기는 horizon_days(int)로는 "이번 장 종료"를 표현할 수 없어 horizon_kind + due_at으로 바꿨고, due_at은 생성 시점에 캘린더로 확정해 저장한다 — 읽을 때 계산하면 휴장일 목록이 갱신될 때 이미 판정된 과거 스탬프의 만기가 움직인다. 모든 만기가 장 마감 시각에 착지하므로 판정에는 항상 실제 종가가 있고, 시장별 캘린더는 scheduleForStamp가 고르되 benchmarkForStock·marketTradeDate와 US/ETF 분기를 일부러 똑같이 맞췄다(셋이 어긋나면 한 스탬프의 만기·벤치마크·거래일이 서로 다른 시장 기준이 된다). 조기 적중은 candle_bars 일봉 고/저로 [stamped_at, due_at]을 훑어 강세는 high≥target, 약세는 low≤target이면 그 날짜로 확정하고 즉시 개봉 대기로 올린다 — 만기 종가만 보면 "목표에 닿았다가 되돌아온" 예측이 빗나감으로 기록되는데 그건 사용자가 틀린 지점이 아니다. 여기서 두 가지를 일부러 제외했는데 스탬프 당일은 그날 고가가 스탬프보다 앞설 수 있어 이미 지나간 가격을 적중으로 쳐주게 되고, 관망은 닿을 방향 자체가 없다. 예측 수정은 5분(쿨다운과 같은 창)만 허용하고 이후 ErrStampPredictionLocked로 막았다 — 오타는 고쳐야 하지만 결과를 보고 목표를 옮길 수 있으면 적중률이 아무 뜻도 갖지 않는다. 리포지토리도 UpdateAnnotation(note·tags·conviction, 영구 개방)과 UpdatePrediction(서비스가 유예 창을 검사)으로 갈랐다. 마이그레이션 046은 TRUNCATE + ALTER인데 적용 직후 로그에 Can't DROP 'horizon_days'(1091)가 찍혔다 — API와 워커 6개가 동시에 부팅해 같은 DDL을 함께 실행하고 진 쪽이 에러를 뱉는 것으로, 스키마 최종 상태는 정상이지만 매 재기동마다 6개 컨테이너가 에러를 남긴다. isSchemaRaceError(1060·1061·1091)로 무시하게 고쳤고 재기동 후 에러 0을 확인했다. FE는 실링(sealKindOf로 적중·빗나감·개봉·검증 D-N을 가르고 테두리 스타일로 상태를 한 겹 더 구분), 목표가 게이지(예상한 폭 중 몇 %가 실현됐는지 — 등락률만 적으면 +9.6%가 목표 +10.5% 대비 거의 다 온 것인지 목표 +30% 대비 3분의 1도 못 온 것인지 알 수 없다), 성적표 모달(관점·기간·확신도·정확도 4탭 + 누적 알파 + 벤치마크 비교), 개봉 모달(대기분을 한 장씩 넘기며 결과 공개 + 회고 한 줄을 기존 note 뒤에 덧붙임 — 원래 이유를 지우면 나중에 판단의 앞뒤가 사라진다), 상세 모달 전환(관심·보유·뉴스와 조작감 통일), 레일 3카드다. E-6(스탬프 후 최대 등락)은 목업 단계에서 FE 추가 요청이 필요하다고 적었지만 조기 적중 판정이 어차피 일봉 고/저를 읽으므로 peakReturn·troughReturn을 같은 스캔에 실어 BE가 내려주게 해서 추가 요청 0이 됐다. 집계는 전부 lib/stamp-stats.ts에서 이미 받아둔 배열로 계산한다. 실측 중 두 가지를 고쳤는데 목표가가 행 메타와 게이지 라벨에 두 번 찍혀서 게이지에 showTargetPrice 플래그를 뒀고, 정확도 탭 각주가 해당 건이 0일 때 "0건 있습니다"라는 비문이 되어 있어 조건부로 갈랐다. 검증: go build/vet/test(stamp_horizon_test.go 신규 8건 — 장중/장후 session_close, 주말 건너뛰기, 휴장일 롤오프, custom 파싱 실패 폴백, US 캘린더 / stamp_performance_test.go의 조기 적중·당일 제외·관망 제외·만기 초과 케이스), tsc·next build(37페이지)·eslint(만진 파일 잔여 1건은 use-stamps와 동일한 기존 fetch-on-mount 패턴), Docker 재기동(Tesseract OK, 046 에러 0). 브라우저 실측은 로그인 게이트라 목록을 fetch 스텁으로 채워 확인했고 — 타일 4개(50%·평균 오차 8.9%·평균 알파 +2.66%·개봉 대기 2건), 행 6개와 실링 5종, 게이지(목표의 91% 도달 / 반대로 126% / 목표의 190% 도달), 레일 3카드, 성적표 모달 4탭, 개봉 모달(삼성전자 적중 1/2, 5축 변화가 5 이상만 표시되는 것 포함), 상세 모달 전 섹션을 확인했다. 생성 폼은 실데이터로 됐는데 삼성전자 274,500원 화면에서 목표 310,000 입력 시 관망 0.0%가 강세 +12.9%로 파생되고, 오늘이 토요일이라 「이번 장 종료」가 08-17(월)로 잡히며, 「직접」은 날짜를 고르기 전까지 저장 버튼이 비활성인 것을 확인했다. 실측 함정 둘: 브라우저 패널이 표시되지 않아 스크린샷이 안 되고 CSS 애니메이션도 안 돌아 Radix 모달이 data-state=closed로 DOM에 남는데 이건 환경 아티팩트다. 그리고 JS로 라우팅을 흔들면 dev 모드에서 main이 여러 개 쌓여 첫 번째 사본을 읽으면 빈 화면으로 보이므로 스탬프 목록이 든 main을 골라 읽어야 한다. 남은 것: 로그인 상태 실화면 육안 확인, 릴리스 노트 미작성, 기존 실패인 internal/import/pipeline OCR 테스트 1건은 이번에도 그대로(손대지 않은 영역)
- 2026-08-16 | OCR 해외 보유종목 ARM 홀딩스 접미사 오류 수정 | ok | 지난 세션부터 실패해 있던 TestParseHoldingsTextOverseasMergedPasses가 want "ARM 홀딩스(ADR)"에 got "ARM 홀딩스(600)"을 내고 있었다. 원인은 textparse/fuzzy_merge.go의 normalizeListingDisplayName에서 두 정규식의 포함 관계였다 — listingSuffix가 `(?i)\s*\([A-Za-z0-9.\-]{2,10}\)\s*$`로 문자군에 0-9를 함께 받고, digitListingSuffix는 `\(\d{2,4}\)`라 **후자가 전자의 부분집합**이다. 그런데 코드는 listingSuffix를 먼저 찾고 그 결과가 빈 문자열일 때만 숫자 괄호를 (ADR)로 고치는 순서였으므로, 숫자 접미사가 실제로 들어온 경우에는 listingSuffix가 항상 먼저 잡아 (600)을 그대로 살렸고 복구 분기는 정작 필요할 때 도달하지 않는 죽은 코드였다. OCR 원문이 `.404홀딩스(600)`(전체 프레임)과 `ARM #3-(ADR)`(좌측 컬럼)이라 두 패스를 합칠 때 이름 복구가 (ADR) 쪽을 골라야 하는데 (600)이 남은 것이다. 진단은 임시 테스트로 두 정규식의 FindString을 나란히 찍어 확인했다(".404홀딩스(600)" → listingSuffix="(600)" digitListingSuffix="(600)"). 수정은 꼬리 괄호를 listingSuffix로 **한 번만** 뽑은 뒤 그 내용이 숫자뿐인지로 분류하는 방식으로 바꿨다 — 실제 상장 구분자는 (ADR)·(NYSE)처럼 글자로 되어 있고 숫자만 남은 것은 OCR이 (ADR)을 잘못 읽은 결과라는 것이 판단 근거이며, 이렇게 하면 digitListingSuffix의 {2,4} 길이 제한 밖인 (12345) 같은 경우도 함께 복구된다. isDigitsOnlyParen 헬퍼를 추가했다. 회귀 테스트 둘을 붙였는데 하나는 normalizeListingDisplayName의 입출력 표(숫자 접미사 복구 / 글자 섞인 (NYSE)는 보존 / 홀딩스·ARM 아닌 이름은 경로 미진입)이고, 다른 하나는 두 정규식이 여전히 겹친다는 사실 자체를 고정해 둔 것이다 — 나중에 정규식을 손대는 사람이 순서 가드를 무심코 지우고 같은 버그를 되살리지 않게 하려는 것이다. 검증: go build/vet 통과, go test ./... 전체 green(이번 수정 전까지 유일한 실패였던 pipeline 패키지 포함). 이 실패는 스탬프 작업과 무관한 기존 결함이었고 손대지 않은 영역이라 지난 세션에는 보고만 하고 넘겼던 것을 사용자 요청으로 이번에 처리했다
- 2026-08-16 | 「내 투자」 빈 화면 개편 + 서버 예시 데이터 | ok | 사용자가 스탬프 0건 화면을 캡처해 보내며 "내 투자의 각 메뉴는 사용자마다 자기 데이터를 갖는 구조인데, 비로그인이나 데이터가 없는 유저는 이게 어떤 메뉴인지 모를 것 같다. 메뉴 한 줄 설명을 보완하고, 샘플 데이터를 고정으로 하나씩 만들어 우측/좌측 상단 버튼으로 확인하게 해달라"고 요청했다. 코드를 먼저 읽어 세 가지를 확인했다 — PageHeader에 이미 actions 슬롯이 있고 주석에 "페이지 단위 동작은 본문이 아니라 여기에 넣는다(UI_SPEC §7)"고 적혀 있어 말한 자리가 정확히 그곳이라는 것, 설명 문구 다섯 중 넷이 목적 문장이 아니라 기능 나열(`손익 · 구성 · 매매 이력`)이고 목적 문장인 건 스탬프뿐이라 그걸 예시로 든 것으로 보인다는 것, 그리고 화면이 답해야 할 상태가 셋(비로그인 / 로그인+0건 / 데이터 있음)인데 취급이 제각각이고 특히 스탬프만 AuthRequiredShell 밖에 있어 비로그인 시 "스탬프를 불러오지 못했습니다" 에러가 뜬다는 것이다. 목업 docs/mockups/empty-state-sample-v1.html(A 진입 3안 / B 예시 표시 3안 / C 버튼 노출 3안 / D 빈 상태 2안 / E 문구 before-after 표 / F 비로그인 2안 / G 저장 형태 2안)로 제시했고 A-1(화면을 그대로 예시로 채움) / D-1(안내 카드+3단계로 교체, 추가로 스탬프 화면에서도 종목을 쉽게 추가할 버튼) / E 5개 전부 교체 / C-3(0건이면 강조·있으면 약하게)을 선택받았다. A-1을 고르면서 "서버에 각 목업 데이터를 하나씩 넣어놓고 백엔드에서 데이터가 없거나 비로그인이면 목업을 제공한다"는 단서가 붙었는데, 이게 내 기본값(FE 상수)과 다른 지점이라 그대로 따랐고 실제로 그쪽이 맞았다 — 이 다섯 화면의 진짜 엔드포인트가 전부 인증 뒤에 있어서 공개 경로가 없으면 비로그인 방문자에게는 FE 상수 말고 줄 방법이 아예 없다. 다만 "제공한다"가 자동인지 버튼인지는 D-1 선택과 앞뒤가 맞지 않을 수 있어 한 번 더 물어 버튼 클릭 시로, 저장 형태는 데모 계정 DB 시드 대신 BE 고정 픽스처로 확정받았다. 데모 계정을 쓰지 않은 이유는 그 행이 사용자 테이블에 그대로 들어가 전체 통계·배치·관리자 화면 곳곳에 제외 조건을 달아야 하기 때문이고, 픽스처는 애초에 쓰이지 않으므로 쿼리에 샐 수가 없다. BE는 internal/sample에 메뉴별 픽스처를 두고 handler/sample.go + 공개 라우트 /api/v1/sample/*(stamps, stamps/summary, watchlist, holdings, journal/notes, price-targets)로 냈다. 픽스처를 고르는 기준은 개수가 아니라 화면 상태의 커버리지로 잡았다 — 스탬프는 적중·빗나감·검증 중·장중 조기 적중·이미 개봉·관망을 한 번씩 담아 실링 5종과 게이지 음수 쪽, 개봉 대기 큐, 성적표 4탭이 전부 그릴 것을 갖게 했고, 보유·관심은 이익과 손실을 섞어 손익 색이 한쪽으로만 쏠리지 않게 했으며, 메모는 고정·종목연결을 각각 넣어 레일 카드가 비지 않게 했고, 목표가는 도달 1건을 넣어 「도달」 상태가 나오게 했다. 이 기준을 테스트로 고정했는데(sample_test.go 8건) 단순 행 수 검사가 아니라 상태 커버리지·stance 파생 일치·요약과 행의 합치·상대 날짜 검사라, 나중에 픽스처를 손대며 해피패스만 남기면 실패한다. 날짜는 전부 now 기준 상대값이라 시간이 지나도 예시가 방치된 것처럼 보이지 않는다. FE는 components/common/sample-mode.tsx(useSampleMode는 상태를 ?sample=1 URL에 둔다 — 새로고침 유지·뒤로가기 이탈·링크 공유가 공짜로 따라온다), SampleModeButton(C-3 강약), SampleRibbon, SampleWrap 점선 틀, 그리고 menu-empty-state.tsx(D-1)와 lib/menu-copy.ts로 정리했다. 문구를 한 파일에 모은 이유는 헤더와 빈 화면이 같은 약속을 해야 하기 때문인데, 헤더는 "채점합니다"라고 하고 빈 화면은 "저장하세요"라고만 하면 화면이 두 말을 하는 셈이 된다. 예시 표시색은 --sample 보라를 새로 뒀다 — 빨강·파랑은 이 앱에서 상승·하락이라는 뜻을 이미 갖고 있어 예시 띠에 쓰면 시세 신호로 읽히고, 브랜드 파랑도 하락색과 같은 계열이라 같은 이유로 피했다. AuthRequiredShell에 sampleAvailable을 더해 ?sample=1이면 로그인 패널 대신 본문을 통과시키고, 그 김에 스탬프·메모장·목표가도 같은 껍데기 안으로 넣어 앞서 확인한 스탬프 에러 화면을 없앴다. 스탬프에는 요청대로 「스탬프 찍기」 버튼과 StampCreateDialog(종목 검색 → 시세 조회 → 폼)를 붙였는데, 이때 도장 버튼 팝오버와 폼이 두 벌이 되는 것을 막으려고 StampPredictionForm으로 먼저 뽑았다. 실측에서 세 가지를 잡았다 — 관심종목 현재가 칸이 NaN이었는데 행이 등락률과 변화폭을 함께 그리는데 합성 시세에 change를 안 채웠기 때문이고, 보유종목이 평가손익 +231%를 찍었는데 시세 폴러가 픽스처 가격을 실시간 가격으로 덮어써서 평단만 픽스처 값으로 남았기 때문이라(삼성전자 74,200 → 274,500) 예시 모드에서는 폴링과 평가금액 추이 차트를 껐고, 목표가도 같은 이유로 실시간 시세 대신 픽스처 시세를 쓰게 했다 — 안 그러면 "1차 목표 도달"이라 적어둔 행이 실제 시세에 따라 미도달로 뒤집혀 메모와 화면이 어긋난다. 스탬프 게이지에서도 장중 도달 행이 "도달"이라고 적어 놓고 "목표의 59% 도달"이라 말하는 모순을 고쳤다(되돌아온 현재가로 진행률을 재고 있었다). 검증: go build/vet/test 전체 green(sample_test.go 신규 8건 포함), tsc·eslint(만진 파일 0건)·next build(37페이지), Docker 재기동 후 /api/v1/sample/* 6개가 인증 없이 200. 브라우저 실측은 로그아웃 상태로 다섯 화면을 전부 확인했다 — 스탬프(타일 50%·평균오차 8.9%·실링 5종·개봉 대기 3건·개봉 예정 2건), 관심종목(6행·담은 뒤 평균 +4.81%·NaN 0), 보유종목(총매입 24,138,306원 / 평가 26,475,308원 / +9.68%로 손계산과 일치, authentication 에러 0), 메모장(5건·고정 2건·종목 연결 3건), 목표가(5건 중 도달 1건). 로그인+0건 화면도 스텁으로 재현해 "없습니다" 8번이 0번이 되고 3단계 안내와 두 버튼이 뜨는 것을 확인했다. 남은 것: 로그인 상태에서 실제 데이터가 있는 화면의 예시 토글 육안 확인, 릴리스 노트 미작성
- 2026-08-16 | 뉴스 품질 개선 — 국내 암호화폐 오탐 + 해외 Google News | ok | 사용자가 국내 뉴스 화면 캡처(코인리더스 100%·BTC·바이낸스 등 순수 코인 기사)와 해외 뉴스의 news 출처 비증권 기사 문제를 지적했다. 원인은 이중이었다 — ingest에서 coinreaders만 엄격했지만 list 시 isStockRelatedArticle이 crypto 키워드를 hasStrictFinanceSignal로 다시 통과시켰고, strongMarketTerms에 btc·가상자산이 들어 있어 매수·배당만 있는 코인 기사도 통과했다. 수정: crypto 단독 키워드를 strongMarketTerms에서 제거, isDomesticCryptoOnlyArticle + passesDomesticCryptoSourceGate로 ingest·list·cleanup 공통화, 코인 RSS 전부 증시 앵커(코스피·증시·상장 등) 또는 국내 종목 심볼 없으면 차단, 코인리더스 RSS 폴링 제거, domesticNewsQualityScore로 프리미엄 매체 우선 정렬. 해외는 google_news에 제목 시장 앵커(hasUSTitleMarketAnchor) 필수 게이트 추가. go test relevance 전부 green, Docker 재기동 OK. DB에 남은 코인리더스 행은 list 필터로 즉시 제외 — 완전 삭제는 admin cleanup 수동 실행 필요
- 2026-08-16 | 뉴스 종목·지수 매칭 개선 | ok | 사용자가 기사 하단 종목/지수 칩 매칭 품질 문제(금→금 지수, BTC→BTC 지수, Micron→MU 등)를 지적. US 이름 인덱스가 ~30종 한정이어 Micron 등 대부분 미매칭, 지수는 tags에만 저장되고 카드에는 symbols만 표시, 번역 후 한국어 텍스트 미반영이 원인. ListUSNameCatalog+한국어 별칭+possessive ingest, 금/BTC 지수 규칙 확장, 번역 후 index 재태깅, retag v3(symbols+tags), FE NewsArticleChips로 지수·종목 칩 통합 표시. go test green, Docker 재기동 후 v3 retag 자동 실행
- 2026-08-16 | 종목 헤더 액션 통일 — 메모장·목표가 | ok | 실시간 차트 우측 상단에 관심·스탬프·리포트만 있고 메모장·목표가 관리 연동이 없었고, AI 추천 랭킹 종목 헤더도 동일 버튼 세트가 없었다. StockSymbolActionBar(5종) 공통화, StockWorkspaceHeader·AnalysisClient 적용, useWatchlistToggle로 분석 화면 관심 토글. FE만 — Docker 재기동 없음
- 2026-08-16 | 비로그인 유저에게도 동일한 빈 화면 안내 적용 | ok | 사용자가 "비로그인 유저에게도 동일하게 적용"이라고만 했는데, 직전 작업에서 예시 모드는 이미 로그아웃 상태로 다섯 화면 전부 실측해 둔 상태라 무엇이 빠졌는지부터 확인했다. 빠진 것은 예시가 아니라 **기본 화면**이었다 — 로그인+0건 회원은 D-1 안내 카드(무엇을 해주는 곳인지 · 3단계 · 버튼 두 개)를 보는데, 비로그인 방문자는 헤더의 예시 버튼만 받고 본문은 여전히 밋밋한 LoginRequiredPanel("로그인 후 이용할 수 있는 기능입니다")이었다. 그 패널이 말해주는 것은 로그인이 필요하다는 사실뿐이고, 정작 설명이 필요한 쪽은 아직 안 써 본 사람인데 그 사람만 설명을 못 보고 가입 여부를 정해야 했다. AuthRequiredShell의 sampleAvailable 불리언을 menu 키(MenuKey)로 바꿔 문구를 MENU_COPY에서 직접 찾게 하고, 비로그인일 때 LoggedOutIntro가 0건 회원과 **같은 MenuEmptyState·같은 headline/description/steps**를 쓰되 버튼만 「로그인하고 시작하기 / 예시로 둘러보기 / 회원가입」으로 바꾸도록 했다. 같은 컴포넌트를 재사용한 것이 핵심인데, 두 상태에 각각 화면을 만들면 문구가 갈라져 같은 메뉴를 두 번 처음 만나게 된다. 아이콘도 같은 이유로 components/common/menu-icon.tsx에 모았다 — 로그인 게이트에서 본 그림과 로그인 뒤 빈 화면의 그림이 다르면 연결이 끊긴다. 다섯 클라이언트가 각자 들고 있던 lucide 아이콘 import는 menuIcon()으로 교체하고 미사용 import를 정리했다. 검증: tsc·eslint(만진 파일 0건)·next build(37페이지). 브라우저 실측은 로그아웃 상태로 /stamps /holdings /watchlist /journal /targets 다섯 곳의 기본 진입을 전부 확인해 안내 카드·3단계·세 버튼이 뜨는 것을 봤고, 관심종목에서 「예시로 둘러보기」 → ?sample=1로 6행 실데이터(담은 뒤 +20.06% 등, NaN 0) → 「예시 끄기」 → 다시 안내 카드로 돌아오는 왕복도 확인했다. 남은 것: 로그인 상태에서 실제 데이터가 있는 화면의 예시 토글 육안 확인, 릴리스 노트 미작성
- 2026-08-16 | 메모장·목표가·리포트 3화면 개편 | ok | 사용자가 "가시성과 좋은 데이터"를 위해 세 화면에 컴포넌트·모달을 추가하거나 레이아웃을 보완해달라고 했고, 착수 전 항상 샘플과 선택지를 달라는 단서를 붙였다. 세 화면의 현재 코드와 데이터 모양을 먼저 읽어 각 화면이 실제로 무엇을 못 하고 있는지부터 특정했다 — 메모장은 글자만 남아 두 달 뒤에 열면 그때 주가가 없고, 목표가는 「목표까지 +11.4%」 숫자 하나뿐이라 그게 가까운 건지 먼 건지가 안 읽히며 손절가를 적을 칸이 아예 없고, 리포트는 발행 시점 캔들·5축 점수·목표 거리(chart.bars·evidence.breakdown·toTargetPct/toStopPct)가 **이미 응답에 실려 오는데 목록이 하나도 안 쓰고** 있었다. 목업 3종(docs/mockups/journal-widgets-v1.html·targets-widgets-v1.html·reports-widgets-v1.html)에 레이아웃 후보와 위젯 후보를 BE 변경 여부까지 표시해 냈고, 선택은 메모장 J-A/J-B/J-C + L2 2단 인박스, 목표가 T-A~T-D 전부 + "행/카드 두 보기를 전환", 리포트 R-A/R-D + R2 가로형 + **썸네일 신규 요청**(없으면 종목 로고 대체)이었다. 스키마는 셋(047 journal_notes.price_at_write·price_date·tags_json / 048 user_price_targets.base_price·base_date·stop_price·target_price_2 / 049 research_calls.thumbnail_url). 설계에서 붙든 원칙은 "잰 값과 적은 값을 섞지 않는다"였다 — 가격 필드는 전부 요청 바디에서 빼 서버가 저장 순간에 재고(capturePrice·captureBasePrice), Update는 가격을 건드리지 않으며, 재등록 시 base_price는 COALESCE로 보존한다(목표를 올릴 때마다 등록가가 리셋되면 「등록 후 +14.9%」가 영원히 0 근처를 맴돈다). 도달일·최고/최저는 **컬럼으로 두지 않고** 스탬프 조기 적중과 같은 방식으로 candle_bars 일봉 고/저를 읽을 때 계산했는데, 저장하면 목표가를 수정한 순간 옛 목표 기준의 도달일이 남아 어긋나기 때문이고, 방향은 등록가 기준으로 판정해 하방 목표를 고가로 기다리다 영영 판정 안 되는 구멍을 막았다. 메모는 스탬프와 달리 적중/빗나감을 매기지 않기로 했다 — 예측이 아니라 기록이라 숫자만 붙인다. 리포트 썸네일은 새 인프라 없이 기존 /api/v1/media/images를 재사용했고(키가 내용 해시), 작성 폼은 업로드·본문 첫 이미지 가져오기·안 함 셋을 주되 기본은 안 함이고 화면이 종목 로고로 대신한다 — 모든 작성자에게 그림을 요구하지 않으면서 목록이 들쭉날쭉해지지 않는 유일한 방법이다. 값이 공개 카드의 img src로 직행하므로 normalizeThumbnailURL로 javascript:·data:·프로토콜 상대(//)를 차단했고, 표지 변경은 body_rev를 올리지 않게 했다(논거가 아니라 표현이라 "수정 5회"로 읽히면 안 된다). 실측에서 두 가지를 잡았다 — 예시 모드 목표가 카드가 「현재 214,500원」 옆에 실제 캔들 범위 1,359,000~2,959,000원짜리 차트를 그려 한 카드가 서로 다른 두 종목을 말하고 있었고(useSparklineCandles를 !sample로 차단, 메모 상세 모달도 동일), 상단 타일 「도달 임박 1」과 레일 「도달 임박 2」가 같은 화면에서 다른 숫자를 말해(레일은 손절 근접까지 셈) 레일을 「곧 결판납니다」로 바꿨다. 047 마이그레이션은 로그가 안 찍혔는데 컬럼은 정상이었다 — 7개 컨테이너가 동시 부팅해 다른 쪽이 ALTER 레이스에서 이기고 이쪽은 isSchemaRaceError로 삼킨 것으로, 이 배포 형태의 정상 결과다(로그 대신 DB를 직접 조회해 3개 테이블 컬럼을 확인했다). 검증: go build/vet/test green, tsc·eslint(신규 파일 0건, 잔여 5건은 전부 기존 fetch-on-mount)·next build(37페이지), Docker 재기동(Tesseract OK). 브라우저 실측 — 메모장 예시 5건(삼성 +8.5%·NVDA +8.3%·카카오 −7.5%가 픽스처 계산과 일치, 태그 4종 집계, 작성 후 평균 +3.1%·플러스마이너스 2/1이 손계산과 일치), 목표가 예시 5건(도달/임박/손절근접 배지, 행↔카드 토글이 localStorage에 유지, 레일 5카드), 리포트는 실데이터 2건으로 5축 막대·목표까지 +10.0%/손절 −29.9%·퀵룩 모달 전 섹션·「곧 판정됩니다」 D-84/D-85까지 확인했다. 남은 것: 로그인 상태에서 메모 작성→가격 박제, 목표가 등록→등록가 박제, 썸네일 업로드 왕복 육안 확인(전부 인증 뒤라 미실측), 릴리스 노트 미작성
- 2026-08-16 | 홈 대시보드 개편 — 클릭 = 모달 4종 + AI 시장 브리핑 + 브리핑 우선 배치 | ok | 사용자가 홈에 "더 좋은 가시성과 좋은 데이터"를 원했고, 특히 대시보드에서 누르는 종목·지수·컴포넌트를 타 메뉴처럼 **모달**로 열어 비로그인·신규 방문자가 화면을 떠나지 않게 해달라고 했다(착수 전 샘플과 선택지 필수). 먼저 홈 코드와 라우터를 읽어 두 가지를 특정했다 — ① 네 블록이 **전부 이탈**한다(지수→/market, 종목→/analysis, 뉴스→/news/domestic, 리포트→/reports/{id})는 것, ② **서버가 이미 주는데 홈이 안 쓰는 공개 엔드포인트**가 다섯이라는 것(/news/{market}/briefing, /indices/scores의 aiSummary, /indices/{id}/daily-closes, /market-calendar, /sample/*). 목업 docs/mockups/dashboard-widgets-v2.html에 A(모달 4종)·B(지수 모달 깊이 I1~I3)·C(새 위젯 W1~W6)·D(레이아웃 L1~L3)로 나눠 냈고, 각 항목에 "기존 API로 가능/신규 필요"와 열 때 나가는 요청 수를 표시했다. 급등·급락 TOP은 랭킹 API가 거래대금·거래량만 지원하고 등락률 정렬이 없어 **제안하지 않은 이유까지 적었다**. 선택은 I2 표준형 · W1만 · L3 브리핑 우선 · 모달 4종 전부였다. 모달 셋은 앱에 이미 있던 것(StockQuickLookModal·NewsArticleModal·ResearchQuickLookModal)을 연결만 했고 신규는 지수 모달 하나였는데, 그 사이를 메우려고 SymbolQuickLookModal을 새로 뒀다 — 공용 종목 퀵룩은 이름·통화·현재가가 채워진 객체를 요구하는 반면 홈의 점수 랭킹은 종가만, 브리핑 언급 종목은 심볼·이름뿐이라, 화면마다 "시세를 어디서 구해 오나"를 다시 짜면 같은 종목이 자리마다 다른 정보로 열린다. 목록이 시세를 들고 있으면 seed로 요청 0, 없으면 열 때 배치 1회다. L3는 사용자에게 제시할 때 이미 위험을 적어 뒀던 대로(브리핑 미생성 시간대에 좌측이 빈다) DashboardTopSection에서 막았다 — 브리핑이 없으면 2단을 아예 만들지 않고 지수를 전폭으로 되돌리고, 국내·해외를 함께 받아 한쪽만 보고 배치를 정했다가 나머지 도착 때 화면이 뛰는 것도 없앴다. 실측에서 설계를 하나 뒤집었다: 지수 모달의 기간 토글을 1개월/3개월/1년으로 짰는데 API를 직접 찔러 보니 limit=66을 줘도 코스피는 31행(07-02부터)만 오고 usd-krw는 5행, gold는 6행이었다 — 캘린더 이름을 붙이면 「3개월」과 「1년」이 똑같은 차트를 그려 화면이 없는 기간을 있다고 말한다. 최대치를 한 번만 받아 클라이언트에서 자르고(토글마다 요청이 나가지 않는다) 앞 버튼보다 실제로 더 많은 봉을 그리는 단계만 남기는 방식으로 바꿨으며, 이름도 거래일 수(20일/전체)로 고쳤다. 같은 실측에서 금·비트코인은 score가 null인데 aiSummary는 오는 것을 보고, 점수 유무로 블록을 감추면 있는 문장까지 사라지므로 문장 기준으로 렌더하게 고쳤다. 「전체 지수」 카드도 같은 모달을 열게 했다 — 접힌 걸 펼쳐 VIX를 눌렀는데 거기서만 아무 일도 안 일어나면 같은 그림의 카드가 자리에 따라 다르게 동작하는 셈이다. MiniToggle을 components/ui로 승격하고 MoodBadge를 뉴스 카드에서 export해 시황 색 정의가 갈라지지 않게 했다. FE만 — Docker 재기동 없음. 검증: tsc·eslint(신규 파일 0건)·next build(37페이지) green, API 실측(브리핑 국내·해외 둘 다 source:"llm", indices/scores 12종, kospi 일별 종가 31행 중 6행에 점수, 국내 100건 중 index:kospi 태그 8건). 남은 것: **화면 육안 검증 미완** — Claude Browser 패널이 표시되지 않으면 requestAnimationFrame이 안 불려 React가 커밋을 못 한다(fetch는 전부 200인데 DOM은 스켈레톤에서 정지, 연결된 Chrome도 없음). 패널을 띄운 뒤 레이아웃·모달 4종 재확인 필요. 릴리스 노트 미작성
- 2026-08-16 | 홈 대시보드 2차 — 지수 자동 흐름 스트립·4열 재편·리포트 카드 강화 | ok | 1차 결과를 본 사용자가 셋을 요청했다: ① 모달 우상단 X가 헤더 글자와 겹침 ② 「홈」 제목 밴드 자리에 다른 데이터 제안 ③ 최신 투자 리포트 카드가 빈약함. ①은 버그라 바로 고쳤다 — 닫기 버튼이 top-4 right-4(16px)인데 헤더가 px-4(16px)뿐이라 정확히 포개졌고, DialogHeader 기본값으로는 못 막는다(소비자가 주는 px-4가 tailwind-merge에서 나중에 와 pr을 덮는다). 같은 구조의 모달 7종에 pr-10. ②③은 목업으로 선택지를 냈고(H0~H3 / R0~R3), 사용자는 H를 닫고 대신 참고 사진 두 장을 주며 "여러 지수를 한눈에 보되 가로로 나열, 점수와 클릭 모달 포함"을 요구했다. 목업 v4로 S1(가로 1줄 스크롤)·S2(6×2)·S3(얇은 티커)와 레이아웃 영향 P1·P2를 냈는데, 그 전에 API를 직접 찔러 제약 셋을 먼저 확인해 목업에 박아 넣었다 — 스파크라인이 12종 중 9종에만 있고(원/달러·나스닥100 선물·필라델피아 반도체는 빈 배열), 점수도 9종만 있으며(금·비트코인·원/달러 없음), 참고 사진의 「소비지표 부진」 같은 코멘트 배지는 우리 aiSummary가 "KOSPI — 최근 5거래일 +11.5%, 20일선 위, RSI 52.4(neutral)" 형태의 서술문이라 잘라 쓰면 어색하다. 사용자 결정은 S1 + 스크롤 대신 애니메이션, P2 변형(뉴스를 우측에, 하단 4열의 남은 두 칸은 컴포넌트 추가), 전체 지수 섹션 제거, 리포트 R2였다. 남은 두 칸은 다시 목업 v5로 C1~C4를 내 C2(다가오는 일정)·C3(오늘의 키워드)가 선택됐다. 마퀴는 목록을 세 벌 이어 붙였는데 두 벌이면 초광폭에서 되감을 때 오른쪽 끝이 비기 때문이고, hover·focus 정지는 장식이 아니라 기능이다(셀이 모달을 여는 버튼이라 움직이는 동안 조준이 안 된다). 사본은 aria-hidden+tabIndex=-1로 스크린 리더와 탭 순서에서 뺐고 prefers-reduced-motion이면 아예 흐르지 않는다. 구현 중 C2 설계가 실측으로 뒤집혔다 — BE 주석이 "DART has no forward schedule API, so domestic symbols only ever produce filed events"라고 못 박고 있고 거래대금 상위 국내 8종목의 upcoming이 실제로 0건이라, 「다가오는 일정」이라는 이름으로 국내 종목을 담으면 영원히 빈 카드가 된다. 이름을 「실적·공시」로 바꾸고 예정(미국 Finnhub)과 최근 제출(국내 DART)을 한 목록에 두되 날짜 표기로 구분했다(D-9 ↔ 2일 전) — 섞어 놓고 구분하지 않는 것만이 거짓말이지, 「2일 전 반기보고서 제출」은 알 값어치가 있다. 첫 실측에서 SK스퀘어 하나가 기재정정까지 세 줄을 먹어 일곱 줄이 전부 같은 공시였고, 예정/제출 배지만 달고 라벨을 툴팁에 숨겼더니 일곱 줄이 「제출」 일곱 개가 돼서, 종목당 1건으로 줄이고 라벨을 행에 드러냈다. 같은 실측에서 날짜 형식이 출처마다 다른 것도 잡았다 — Finnhub는 8/26처럼 연도 없이 주는데 화면들이 date.slice(5)로 잘라 써서 미국 일정이 「6」으로 뜨고 있었다. lib/earnings-date.ts를 새로 두고(연도 없는 값은 올해로 읽되 반년 이상 과거면 내년) 종목 퀵룩의 같은 버그도 고쳤다. 키워드 칩의 목적지 /news/domestic?keyword=는 파라미터가 없어서 새로 뚫었는데, 초기값으로만 읽고 [market] 초기화 이펙트가 마운트에서 돌지 않도록 prevMarket ref를 뒀다(안 그러면 화면이 뜨자마자 스스로 지운다). 리포트 카드는 목록 응답에 chart.bars·evidence.breakdown·progress·toTargetPct·조회·좋아요가 이미 다 실려 오는 것을 실측으로 확인하고 채웠으므로 추가 요청이 0이고, 사용자 요청대로 좌우 두 칸 모두 flex-1·mt-auto로 바닥까지 채워 빈자리를 없앴다. 목표까지 거리는 서버의 toTargetPct를 그대로 쓴다(화면에서 다시 구하면 목록과 상세가 다른 기준가로 다른 숫자를 말한다). 삭제: MarketOverview(그리드)·MarketHeroStrip·AllIndicesSection·DashboardTopSection. FE만 — Docker 재기동 없음. 검증: tsc·eslint(신규·수정 파일 0건, 저장소 잔여는 전부 기존 set-state-in-effect)·next build(37페이지), API 실측(브리핑 국내·해외 llm, 지수 12종, 코스피 일별 31행 중 6행 점수, 기사 100건에서 키워드 14개, 실적·공시 24건→7행). 남은 것: 화면 육안 검증 미완 — Claude Browser 패널이 표시되지 않으면 requestAnimationFrame이 안 불려 React가 커밋을 못 하고(fetch는 200인데 DOM은 스켈레톤 정지) 연결된 Chrome도 없다. 자동 흐름·hover 정지·4열·리포트 카드는 패널을 띄운 뒤 확인 필요. 릴리스 노트 미작성
- 2026-08-16 | 지수 스트립 수동 스크롤 + 점수 AI 표시 | ok | 사용자가 둘을 요청했다: ① 애니메이션이 멈췄을 때 스크롤바는 안 보이되 스크롤이 되게 ② 점수가 AI 평가라는 걸 처음 쓰는 사람이 알 수 있게 AI 표시. ①은 구현 방식을 바꿔야 했다 — CSS transform 마퀴는 컨테이너를 overflow:hidden으로 잠가야 해서 멈춰도 볼 수 있는 건 화면에 걸린 일곱 칸뿐이다. 흐름을 scrollLeft로 옮기니 손으로 미는 것과 저절로 흐르는 것이 같은 속성이라 싸우지 않고, .hide-scrollbar로 스크롤바만 감췄다. 처음엔 pointerenter/leave + pointerdown/up으로 멈춤을 잡았는데 검토 중 모바일에서 깨지는 걸 발견했다 — 터치 팬이 시작되면 브라우저가 pointercancel을 쏘고 pointerleave가 따라와, 손가락이 아직 화면에 있는데 멈춤이 풀려 띠가 손가락과 싸운다. 그래서 포인터는 마우스 hover와 키보드 포커스만 받고, 사용자 조작은 "스크롤 위치가 우리가 지정한 값과 어긋났다"는 사실로 판정해 1.2초 유예하도록 바꿨다(휠·트랙패드·손가락·관성이 전부 한 규칙으로 잡힌다). 멈춘 동안 모은 carry는 버린다 — 안 그러면 놓는 순간 훌쩍 건너뛴다. 같은 검토에서 이음매 버그도 잡았다: 벌마다 pr-2를 들고 있는데 바깥 flex에도 gap-2를 줘서 벌과 벌 사이만 16px이 됐고, 무한히 이어져 보여야 할 띠에서 그 자리가 눈에 띈다. 세 벌을 이어 붙이고 가운데 벌에서 시작하는 것은 뒤로 밀 때 시작점에 부딪히지 않게 하기 위해서다. ②는 표기 전에 사실부터 확인했다 — index_score.go:116이 "Written from computed values only — no LLM"이라고 명시하고 aiSummary도 "최근 5거래일 +11.5%, 20일선 위, RSI 52.4(neutral)" 형태의 템플릿 문장이라, 이 점수는 LLM이 아니라 규칙 기반 기술 점수다. 앱이 이미 이 영역을 「AI 추천 랭킹」·「AI 분석」으로 부르고 있어 배지에는 요청대로 AI를 넣되 툴팁에 20일선·RSI로 산출한다는 것과 매매 신호가 아니라는 것을 적었고, 사용자에게도 표기가 실제와 다르다는 점을 알렸다. 모달 섹션 제목도 「AI 추세 점수」로 맞춰 같은 배지가 화면마다 다른 뜻이 되지 않게 했다. FE만 — Docker 재기동 없음. 검증: tsc·eslint(만진 파일 0건)·next build(37페이지). 남은 것: 화면 육안 검증 미완 — Browser 패널이 표시되지 않아 requestAnimationFrame이 안 불려 React가 커밋을 못 한다. 자동 흐름 속도·hover 정지·손으로 밀기·이음매·AI 배지는 패널을 띄운 뒤 확인 필요. 릴리스 노트 미작성
- 2026-08-17 | 실적 캘린더 BE 연결·개별 collapse | ok | FnGuide 연결+개별 쌍이 시장 캘린더에 두 줄로 노출(035420 8/7 실측). CollapseDomesticBasis(연결 우선)를 Calendar·Summary·Surprises·EstimatedDomestic에 적용. go test green, Docker 재기동(Tesseract OK), API 035420 8/7 2→1행
- 2026-08-17 | 실적 캘린더 FE duplicate key 수정 | ok | dev 콘솔 duplicate key(035420-2026-08-07 등). API가 같은 종목·분기·날짜를 다른 수치로 두 줄 반환하는데 React key가 symbol-date만 써 충돌. earningsEventKey() 공통화 후 캘린더·레일·서프라이즈·종목 탭 적용. FE만 — Docker 재기동 없음. tsc·eslint green, /earnings 8/7 선택 후 콘솔 경고 0건 | 사용자가 금리 등 종목·지수 키워드가 없는 기사를 열었을 때 우측 사이드바에 메모·스탬프 버튼이 활성화되는 오류를 지적했다. `NewsArticleContext`를 읽어 원인을 특정했다 — `hasEntities`(symbols 또는 dashboard index 매칭)가 false일 때 `NewsGatedActionBar`로 기사 단위 메모·스탬프 폴백을 띄우는 로직이 08-15 카드 통일 작업 때 의도적으로 남겨진 것이었다. 그러나 실제 동작은 `/journal`·`/stamps`로만 이동하고 종목·기사 링크가 저장되지 않아 폴백을 제거했다. 이어서 같은 기사에서 우측 검은 빈 칸도 불필요하다는 피드백 — iframe 모달은 `{context && <aside>}`인데 `<NewsArticleContext />` 엘리먼트 자체는 truthy라 내용이 없어도 칸이 남았다. `articleModalContextHasContent`(관련 종목·지수 또는 `relatedArticles` 묶음)로 host에서 context를 undefined로 두어 aside를 숨겼다. FE만 — Docker 재기동 없음. 검증: eslint(만진 파일 0건)
- 2026-08-17 | 출시 전 응답 속도 개선 — FE·BE 실측 기반 | ok | 사용자가 "한 페이지가 카드 여러 장이고 API를 많이 부르는 구조라 살짝 느리다, VM 인스턴스면 더 느려질 것까지 대비해 FE·BE 양쪽 개선안을 설계하고 진행해달라"고 했고, **특정 API가 데이터를 너무 많이 불러 느린 경우 기능 축소까지 고려 중**이라는 단서를 붙였다. 그래서 고치기 전에 전부 쟀다 — curl로 공개 엔드포인트를 훑어 느린 다섯을 특정하고(news/us 1.6초·news/domestic 0.5초·scores/top 해외 2.0초·research/calls 72KB·financials/history 4초 cold), 원인은 EXPLAIN·redis-cli monitor·limit별 스케일링으로 갈랐다. 「limit을 줄여도 안 빨라지면 고정 비용, 비례하면 항목당 비용」이 가장 빨리 갈리는 지점이었다. 기능 축소가 걸린 셋(뉴스 100건·리포트 200봉 차트·실적 8요청)은 선택지를 만들어 물었고 전부 권장안이 선택됐다. 인덱스 둘을 050으로 넣었다 — 랭킹 쿼리가 stocks 17,070행을 풀스캔+filesort 하고 있었고(해외 탭 2.10초), idx_stock_scores_rank(trade_date, score DESC, peer_pct DESC)로 정렬을 인덱스에서 공짜로 받고 LIMIT에서 끊게 하니 0.036초가 됐다(옵티마이저는 자유롭게 두면 asset_type 조건이 있는 stocks를 먼저 잡아서 STRAIGHT_JOIN으로 구동 테이블을 못박아야 했다). 다른 하나는 목록 응답의 meta.updatedAt 한 칸을 채우는 MAX(updated_at)이 해당 market의 3만여 행을 읽던 것으로(727ms), (market, updated_at) 인덱스로 0.45ms가 됐다 — 정작 목록 본 쿼리는 0.65ms였다. 인덱스는 이름이 마커라 기존 tableExists/columnExists 방식이 안 맞아 indexExists를 새로 뒀고, 두 인덱스가 서로 다른 테이블이라 하나만 마커로 쓰면 나머지가 영원히 안 붙는 함정을 피했다. **드롭 후 재기동으로 재적용까지 실측 확인**했다. gzip은 아예 없었다 — chimw.Compress(5) 한 줄로 98KB가 29KB가 됐고, SSE는 chi 기본 압축 목록 밖이라 그대로 흐르며 compressResponseWriter가 Flush를 위임하는 것도 소스로 확인했다. 뉴스 목록이 이번 작업의 절반이었는데 **세 층이었고 첫 진단이 틀렸다**: ① 기사마다 ResolveEmbedPolicy가 호스트별 Redis GET을 쏴 요청 하나에 342회였고(monitor로 news:embed 키 0회까지 확인) 캐시 미스면 남의 사이트를 8초 타임아웃으로 받아 오기까지 했다 → 호스트 in-process 캐시 + 배경 조사. 그런데 **고치고 재니 시간이 그대로였다** ② 진짜 원인은 extractUSSymbols가 기사마다 이름 인덱스 4만 항목을 훑는 것으로 요청당 800만 회였다(관련성 판정 + 미리보기 이미지로 기사당 두 번) → 인덱스 세대(gen)+본문을 키로 메모하고, 인덱스가 다시 만들어지면 세대를 올려 옛 답이 조용히 재사용되지 않게 했다. 국내도 같은 구조라 같이 고쳤다 ③ trackedSymbols·usIndexSymbols가 기사마다 watchlist 전수 조회 + 포트폴리오 조회를 해 portfolio Redis GET만 200회였다(AGENT_STATE TODO에 있던 AllTrackedSymbols 캐싱 건) → 1분 TTL. 마지막에 목록 자체를 15초 캐시하되 ingest 시 즉시 무효화했다. 결과 us 1.35초 → 0.09초(cold)/0.027초(warm), domestic 0.42초 → 0.05초. 부수적으로 resolveUSNewsPreviewImage가 **결과를 쓰지도 않을 전수 스캔을 먼저** 돌리고 있어(기사 고유 이미지가 있으면 심볼이 몇 개든 원본을 그대로 쓴다) 값싼 관문을 앞으로 옮겼다. 홈 전용 뉴스 엔드포인트는 앞 8건만 온전히 주고 나머지 92건은 제목·태그·키워드만 남기는 방식으로 만들었다 — 집계 규칙(속보 판정·키워드 세기)을 서버로 옮겨 적으면 같은 규칙이 두 언어에 두 벌 생겨 언젠가 어긋나므로, 배열 하나에 같은 타입으로 담아 화면 집계 코드를 그대로 뒀다(98KB/gzip 29KB → 35KB/gzip 10KB). fullCount 밖 기사는 본문·링크가 없어 모달 pool과 목록 카드 양쪽에서 잘라 넘겼다. 리포트 목록 차트는 chart.bars 200봉 OHLCV(건당 19KB)를 chart.closes 60점으로 바꿨는데, model.Candle에 omitempty를 붙이면 quotes API 계약까지 바뀌므로 ResearchClosePoint 타입을 따로 두고 둘 중 하나만 채우게 했다(상세는 bars 그대로). 실적은 배치 엔드포인트를 신설해 8요청을 1로 줄였고 상한 20을 뒀다 — 심볼이 URL로 오므로 상한이 없으면 누구나 수백 종목짜리 팬아웃을 시킬 수 있다. FE에서는 「랭킹 → 배치 시세 → 실적」 3단 waterfall을 깼다: 일정에 필요한 건 종목 코드뿐이라 랭킹 직후 onSymbols로 올려 시세와 일정이 나란히 나가게 했고(네트워크 로그에서 두 요청이 같은 밀리초에 출발하는 것 확인), 이름은 행에 담지 않고 그릴 때 찾게 했다 — 담으면 일정이 시세보다 먼저 왔을 때의 빈 이름이 그대로 굳는다. 요청 캐시는 라이브러리를 들이지 않고 lib/fetch-cache.ts로 진행 중 요청 합치기 + 짧은 재사용 둘만 했다(랭킹·점수·실적 5분, 뉴스 30초, 배치 시세 5초 — 시세를 캐시하려는 게 아니라 같은 순간 두 카드가 같은 목록을 부르는 것만 거른다). 키에 사용자 정보가 없어 로그인·로그아웃에서 clearFetchCache()를 건다. 홈 API 요청 17개 → 11개, 최장 체인 3단 → 2단. 작업 중 실수 하나 — redis-cli monitor를 끊으려고 shutdown nosave를 써서 Redis를 내렸다(기동 시 Redis 필수라 위험했다). 즉시 재기동해 mysql/redis 둘 다 ok 확인했고, 이후로는 timeout 4 redis-cli monitor로 바꿨다. 또 monitor로 본 Redis 트래픽 대부분이 백그라운드 폴러 것이어서 info commandstats 델타로 잰 첫 수치(342 → 1293)가 오염돼 있었다 — 키 패턴으로 좁혀야 의미가 있었다. 검증: go build/vet/test green, tsc·eslint(신규·수정 파일 0건, 잔여 2건은 기존 dead code)·next build(37페이지), Docker 재기동(Tesseract OK), 마이그레이션 드롭→재적용. 브라우저 실측 — 홈에서 news/domestic/dashboard·earnings/batch 200과 quotes/batch 동시 출발, 응답 모양(뉴스 100건 중 fullCount 8·50번째 url 없음 / research closes 60·bars 없음 / earnings 심볼별 calendar), 리포트 화면 스파크라인 3개가 viewBox="0 0 320 48"에 60점 좌표로 실제 렌더, 뉴스 화면 기사 199건 정상. 남은 것: **스크린샷 육안 검증 미완** — Browser 패널이 표시되지 않아 compositing이 안 된다(DOM·네트워크로는 전부 확인). financials/history cold 4초는 외부 API라 손대지 않았고(캐시는 이미 있다) 워밍 배치가 필요하다. 릴리스 노트 미작성
- 2026-08-17 | 뉴스 시장 분류 — 국내 경로 해외 종목 기사 해외 탭 이동 | ok | 사용자가 엔비디아 관련 기사(토큰포스트 RSS 등 국내 수집 경로)가 국내 주식 뉴스 탭에 노출되는 문제를 지적. 원인: `inferNewsMarket`의 `inTitle`이 티커(NVDA) 문자열만 검사해 한글명「엔비디아」는 제목 가중치(3.0)를 못 받고 usScore=1.0으로 domestic 유지. `symbolsMentionedInTitle` 추가(extractUSSymbols·extractSymbols·cashtag 등 제목 전용). 기존 DB 행은 `RemigrateDomesticToUS` 1회 스윕(`news:remigrate-market:v1:`). BE unit test 2건 추가. Docker 재기동(Tesseract OK). 릴리스 노트 미작성
- 2026-08-18 | 예시 둘러보기·FE 런타임 오류 수정 | ok | 비로그인 「예시로 둘러보기」 클릭/새로고침 시 sample=false race로 인증 API 401·빈 화면 — `useSampleMode` URL 동기 읽기 + `isSampleModeActive()` fetch 가드, journal/holdings/watchlist/targets Suspense. earnings `latestDayWithEvents` 선언 순서, sidebar-toggle hydration. tsc green, 브라우저 /stamps?sample=1·/earnings/domestic 확인
- 2026-08-18 | 국내 실적 표 열·정렬 조정 | ok | 시가총액 열(발표일 다음)·영업이익·매출 예측/실적(bold)·기본 시가총액 내림차순. FE만
- 2026-08-18 | 일별 종가·실적 반응 2026-01~ 백필 | ok | 국내 fetch-history 14,213/14,217. 해외: -market=us CLI·fetch-history 배치 추가 — MySQL80 중지로 실행 보류
- 2026-08-19 | 출시 전 전체 점검서 작성 (docs/PRE_LAUNCH_OPTIMIZATION.md) | ok | 클라우드 배포 전 리팩토링·최적화 점검. 코드를 고치지 않고 **실측만** 했다 — 살아 있는 Docker 스택(uptime 11h)에 curl로 공개 엔드포인트를 훑고, 컨테이너 액세스 로그 2,222건을 집계하고, `docker stats`·`redis-cli INFO`로 인프라를 쟀다. 발견: ① 레이트 리밋이 아예 없고 `/quotes/batch?symbols=`가 개수 무제한(공개 경로) ② `/auth/me`·`/market/data-settings`까지 30초를 찍은 구간이 있었는데 이는 호스트 네트워크 단절 때의 값이라 코드 결함은 아니나 **핸들러에 자체 타임아웃이 없어 WriteTimeout 30초가 유일한 방어선**인 것은 문제 ③ Redis `maxmemory 0` + `noeviction`, 427MB·23,719키 중 약 2,200개가 TTL 없음 ④ api+워커7 = 8프로세스가 각각 `defaultMaxOpenConns=50` → 최대 400 커넥션(MySQL 기본 151) ⑤ 8프로세스 전부가 기동 시 `EnsureSchema` 실행 — 버전 테이블 없는 `MigrateXIfNeeded` 약 30개가 경쟁, `isSchemaRaceError`로 삼키는 구조라 **빈 DB 첫 배포가 가장 위험** ⑥ FE에 Dockerfile·compose 서비스·`output: standalone` 전부 없음. P1: `/news/us` 660~2,270ms의 원인은 `NewsRepo.Count`의 `COALESCE(headline_ko,'')!=''`가 인덱스를 못 타 18,726행 풀스캔인데(도메스틱은 59,238행인데도 13ms — 이 술어가 없어서다) 15초 리스트 캐시가 목록만 덮고 핸들러의 Count는 캐시 밖 호출이라 매번 지불 / `/rankings` US 900~1,110ms·`/quotes` 콜드 4~5초(최대 20초)는 `EnrichRankingPrices`·`applyLivePrices`가 항목마다 7단 폴백을 **직렬로** 타는 N+1이며 그중 `directTossPrice`는 조회 경로에서 외부 HTTP를 부른다(AGENT_STATE 원칙 위반) / SSE가 in-process hub라 다중 인스턴스 fan-out 불가. 측정 중 `/quotes?market=domestic`이 20초 타임아웃을 냈는데 `market`은 이 엔드포인트의 파라미터가 아니었고(실제로는 `filter`), 올바른 파라미터로 재측정하니 3ms — **틀린 파라미터로 잰 첫 수치를 그대로 쓸 뻔했다**. 페이지별 API 맵은 import 그래프 정적 추적으로 만들었으나 공유 컴포넌트 때문에 상위 집합만 나온다(admin 7페이지가 `lib/admin.ts` 하나를 공유해 전부 25개로 동일). 정확한 런타임 호출은 브라우저 실측이 필요한데 **Browser 패널이 표시되지 않아 못 했다** — 문서 부록 A-4에 후속 작업으로 남겼다. 잘 되어 있어 건드리지 말 것으로 따로 정리한 것: 배치 워커 분리(idle 6~9MB), LLM 지출 통제(월 2,193콜), apibudget Redis 공유 카운터, gzip(98KB→29KB), `050_perf_indexes`, `isPageVisible()` 가드, graceful shutdown 순서. 코드 변경 없음 — 문서와 AGENT_STATE TODO 포인터만
- 2026-08-19 | 출시 전 체크리스트 — 작은 항목부터 개선 (8건) | ok | 점검서의 배포 전 체크리스트를 작업량 순으로 처리. 시작 전 baseline(go build/vet/test green, 20패키지)을 잡고 매 항목마다 Docker 재기동 후 실측 검증했다. **① 액세스 로그**(P2-3) — `<reqID> METHOD PATH STATUS SIZE DURATION`으로 확장. 여기가 이번 작업에서 가장 조심한 지점으로, ResponseWriter를 평범한 struct로 감싸면 SSE가 죽는다(`w.(http.Flusher)` 단언 실패 + `http.NewResponseController`가 밑단 `SetWriteDeadline`을 못 찾아 조용히 실패 → WriteTimeout 30초가 스트림을 끊음). chi v5.3.1 소스에서 `basicWriter.Unwrap()`과 `compressResponseWriter.Unwrap()`을 직접 확인하고 `chimw.NewWrapResponseWriter`를 채택, **SSE가 45초까지 유지되는 것을 실측**(30초에 안 끊김)했고 gzip도 로그상 28.3kB/95.5kB로 구분 확인. 덤으로 Recoverer가 `http.ErrAbortHandler`를 재패닉하게 했다 — 안 하면 SSE 사용자가 탭 닫을 때마다 스택 트레이스가 쌓여 정상 종료가 장애처럼 보인다. **② 뉴스 카운트 캐시**(P1-1) — `/news/us` 워밍 660~2,270ms → **2.5ms**. 원인은 `NewsRepo.Count`가 us에만 붙이는 `COALESCE(headline_ko,'')!=''`가 인덱스를 못 타 18,726행 풀스캔인데, 15초 리스트 캐시가 목록만 덮고 핸들러의 Count는 캐시 밖 호출이라 매 요청 지불한 것. 단순 캐시만 하면 15초마다 한 요청이 1.6초를 뒤집어쓰므로 **stale-while-revalidate**(만료 시 낡은 값 반환 + 배경 goroutine 갱신, refreshing 플래그로 스탬피드 차단, 요청 ctx 대신 자체 ctx — 응답 쓰면 취소되므로)로 만들었다. **진단이 한 번 바뀌었다**: 콜드 2.5초를 목록 재구성 탓으로 봤는데 limit별로 캐시 키를 비껴가며 재보니 25/50/100/200이 13~272ms로 깔끔히 비례 — 느린 건 카운트 하나였다. 첫 측정 6.9초는 재기동 직후 백필과 겹친 값. **③ 심볼 목록 상한**(P0-1 일부) — 문서에 "`/quotes/batch`가 개수 제한 없음"이라 썼는데 **틀렸다**(`batchQuoteSummariesMax=100` 이미 존재). 전수 점검하니 진짜 무제한은 `/stocks`와 `/market-indicators/prices`였고 둘 다 FE 호출부가 0이라 안전하게 100 적용. 파싱은 `handler/symbols.go`로 통일(중복 제거 전 개수로 자르면 결과가 조용히 줄어서 파싱 상한은 4배). 500개 테스트에서 `/stocks`가 429를 뱉어 **미등록 심볼이 Toss 업스트림을 때리고 시세 폴러 한도를 잠식**하는 벡터가 남아 있음을 확인(문서화). `limit` 상한 강제도 문서에 남겼으나 quotes 200·search 200·news 200/300으로 **이미 전부 클램프**되어 있었다(정정). **④ 워커 커넥션 풀**(P0-4) — 7워커 × 기본 50 = 350 → `MYSQL_MAX_OPEN_CONNS=8`로 56. api 50은 부하 테스트 전까지 안 건드림. **⑤ Redis 상한**(P0-3) — `maxmemory 1gb` + `volatile-lru`. 정책은 TTL 없는 키를 먼저 조사해서 정했다: `md:rev:<symbol>` 2,204개(123KB)뿐인데 `bumpRev`의 INCR 캐시 무효화 카운터라 유실되면 리비전이 되감겨 낡은 캐시가 새 것으로 보인다 — allkeys-lru는 이걸 버릴 수 있고 volatile-lru는 정확히 이것만 보호한다. **⑥ 마이그레이션 단일화**(P0-5) — `repository.VerifySchema`(DDL 없이 대기) 신설, 워커 7개가 `EnsureSchema` 대신 호출, compose에 `depends_on: api service_healthy`, `BootstrapWorker`에서 안 쓰는 `migrationsDir` 인자 제거(호출부 7곳 + filepath import). 로그 문구가 둘 다 같아 구분이 안 돼 워커 쪽을 `schema verified (read-only; api is the migrator)`로 분리. `compose down` 후 전체 기동으로 redis→api→워커7 순서 실증. **⑦ .gitignore** — `tmp-*.json` 13개가 "커밋 트리에 남아 있다"고 썼으나 전부 untracked였다(정정). 삭제는 사용자 파일이라 안 하고 `/tmp-*.json`만 추가. **부수 발견**: 이 코드베이스는 gofmt를 거친 적이 없다 — internal·cmd 아래 466개가 미준수이고 원인은 줄바꿈이 아니라 `var(...)` 정렬에 탭 대신 공백을 쓴 것(HEAD 상태에서도 동일). 재포맷은 작업 중 트리에 거대한 diff를 만들어 안 했고, CI에 gofmt 게이트를 처음부터 넣으면 안 된다는 주의를 문서에 적었다. 검증 중 실수 하나 — `git show HEAD:f | gofmt -l /dev/stdin`으로 "HEAD는 준수"라 결론냈는데 Windows에서 gofmt가 /dev/stdin을 못 읽어 항상 빈 출력이었다(무효한 방법). git diff로 다시 확인해 정정. 전 구간 회귀: 주요 엔드포인트 200·SSE 정상·패닉 0·워커 7개 정상. **남은 것**: 레이트 리밋(임계값이 사용성에 직결돼 사용자 확인 대기), P0-2 요청 타임아웃(P1-2 이후), P1-2 목록 N+1, 뉴스 목록 캐시 stampede(40동시 요청에서 1.7~6.6초 실측), 뉴스 인덱스 마이그레이션(사용자가 스키마 변경 제외 지시), FE Dockerfile. 릴리스 노트 미작성
- 2026-08-19 | 레이트 리밋 · 뉴스 stampede · CI (3건) | ok | 사용자가 "사용성이 저하되지 않도록, 저하 가능성이 있으면 선택지 제안 후 작업"을 조건으로 걸었고, 스키마 변경은 이번 범위에서 제외 지시. **① 뉴스 목록 stampede** — TTL 만료 시 40 동시 요청이 각자 목록을 재구성해 p50 2.0s/max 6.6s였다. 새 추상화를 만들지 않고 시세 쪽 `quoteRefreshCoordinator.doSync`를 그대로 재사용(소유자 하나만 빌드, 나머지는 done 채널 대기, 5초에 끊고 스냅샷 폴백) → p50 1.21s / max 1.41s, 분포가 0.88~1.41로 좁아진 것이 "한 번만 만든다"의 신호. 동시 응답 20개가 모두 같은 totalCount(18,846)로 정합성 확인. 주의점 하나: 원래 코드는 빌드 후 값을 직접 return했는데 캐시에 쓰고 다시 읽는 구조로 바꾸면서, `rememberList`가 64개 초과 시 맵을 통째로 비우고 **그 다음 키를 넣는다**는 것을 확인하고서야 재읽기가 반드시 적중함이 보장됐다. **② 레이트 리밋**(P0-1) — 로그인=user_id 10rps/burst20, 비로그인=IP 30rps/burst60. 임계값은 FE 폴링 실측(/quotes 화면 1.4 req/s, 탭3개 4 req/s)의 2.5배. **인증 미들웨어 뒤**에 그룹별로 걸었다 — 앞에 두면 컨텍스트에 사용자가 없어 로그인 사용자까지 공유 IP 버킷에 떨어져 설계 의도가 무너진다. 버킷은 idle 10분 회수(없으면 IP마다 영구 누적되는 느린 누수), `RATE_LIMIT_ENABLED=false`로 재배포 없이 끌 수 있게 했다. config가 middleware를 import하면 middleware→service→config 순환이라 config에 같은 모양의 구조체를 따로 두고 배선 지점에서 변환. 단위 테스트 9개(버스트·리필·장시간 유휴 후 상한 초과 금지·키 독립·NAT 뒤 두 사용자 분리·비활성 통과·429+Retry-After·sweep). **③ XFF 위조 우회 — 이번 작업의 핵심 발견** — chi의 `middleware.RealIP`가 X-Forwarded-For를 조건 없이 믿고 RemoteAddr를 덮어써서, 레이트 리밋 키로 쓰는 순간 헤더만 바꾸면 매번 새 버킷을 받는다. 자체 `middleware/realip.go`로 교체: (1) loopback·사설 peer에서 온 헤더만 신뢰 (2) nginx/ALB가 **덧붙인다**는 점을 이용해 **오른쪽 hop부터** 읽고 신뢰 대역을 건너뛴다(맨 왼쪽은 공격자가 심는 값). 테스트 9개 추가. **측정 방법에서 두 번 틀렸고 둘 다 스스로 잡았다**: (가) 처음 "XFF 회전 300발 전부 통과 = 우회 확정"이라 했는데 순차 curl이라 프로세스 생성 오버헤드로 애초에 한도 미만이었다 — 동시 요청으로 다시 재고서야 진짜 그림이 나왔다(고정 XFF 55차단 / 덧붙임형 45차단 / 단일값 회전만 0차단). (나) `TestAllPrivateChainKeepsLeftmost`가 실패했는데 테스트가 아니라 코드가 틀렸다(오른쪽부터 걸으며 첫 유효값을 잡아 rightmost를 반환) — 매 반복 덮어써 leftmost가 남게 수정. **④ 배포 요구사항 발견** — 개발 서버 3000을 통해 XFF 회전 150 동시를 보내니 150건 전부 통과: **Next의 rewrite는 클라이언트 XFF를 그대로 BE에 전달한다**. 현 구조를 그대로 배포하면 비로그인 IP 한도는 우회된다(로그인 user_id 한도는 토큰 기반이라 무관하게 정상). 리버스 프록시를 앞에 두는 것이 해법이고 P0-6과 묶여 사용자 결정 대기. TRUST_PROXY_HEADERS=false는 쓰면 안 된다 — 모든 비로그인 사용자가 Node IP 하나를 공유하게 된다. **⑤ CI**(P2-4) — go build/vet/`test -race` + tsc + next build를 게이트로, lint는 continue-on-error. **게이트를 걸기 전에 전부 로컬 실증**했다: tsc 통과, next build 통과, lint는 71 errors(기존 부채, FE는 이번에 안 건드림)라 비차단으로 내렸다. `-race`는 로컬에 gcc가 없어 못 돌리므로 **golang:1.25-bookworm 컨테이너에서 미리 실행**해 전 패키지 통과를 확인하고 넣었다(이번에 추가한 배경 goroutine 3종의 경쟁 조건 검증도 겸함). 전 구간 회귀: 주요 엔드포인트 200, SSE 정상, 패닉 0, 컨테이너 9개 정상. 남은 것: P0-6 FE 배포(프록시 결정 대기), P0-2 요청 타임아웃, P1-2 목록 N+1, 뉴스 인덱스 마이그레이션(스키마 변경 제외 지시), 목록 stale-while-revalidate. 릴리스 노트 미작성
- 2026-08-19 | 내부자 거래 온디맨드 → 배치 적재 (`worker-filings`) | ok | 사용자가 제기한 구조 문제: "대부분의 데이터를 외부 API에서 요청 시점에 가져오고 있어, 사용자가 늘면 한도에 걸려 아무 데이터도 못 준다". 조사해 보니 요청 시점에 외부를 부르는 경로는 6개였다 — 내부자 거래, 재무 이력, 국내 실적 탭(캘린더·분기), `/disclosures/today`(보유종목 수만큼 DART), `/stocks/{symbol}/warnings`, insights 실적 이벤트. 전부 **Redis만** 캐시하고 MySQL 영속이 없다. 반대로 펀더멘털·점수·일별종가·`earnings_events`·뉴스·지수는 이미 배치+DB라 패턴은 존재했고 이 6개가 빠져 있었다. 사용자 승인 아래 **내부자 거래를 템플릿으로 먼저** 구현. **핵심 발견 — EDGAR 한도를 우리가 75배 낮게 잡고 있었다**: `docs/api-quotas.md`가 SEC 공표치 10 req/sec를 적어 두고도 적용값은 8/min이었다. 그 값 때문에 종목당 Form 4를 **3건만** 읽었고, 최근 3건이 전부 파생거래(옵션·RSU)면 비파생 표가 비어 **정상 응답으로 빈 화면**이 나왔다 — 사용자가 말한 "아무 데이터도 못 읽어오는 종목"의 정체가 이것이다. 상한을 올리는 게 아니라 적재를 배치로 옮기는 문제로 다뤘다. **구현**: `052_insider_transactions.sql`(거래 표 + `insider_sync_state`) / `repository/insider_repo.go`(멱등 upsert·`KnownFilingKeys`·`SelectTargets`·`Coverage`·`WasSynced`) / `service/insider_ingest.go` / `cmd/worker-filings` + compose 서비스 / apibudget `edgar_bulk` 버킷(240/min, 300ms) / `EdgarClient.UseBatchBudget()`+`pace()` / `InsiderTabService`는 DB만 읽도록 재작성. **등급제 대상 선정**은 요일 배분표가 아니라 ORDER BY 하나로 했다 — 관심·보유 중 24h 지난 것 → 미수집 → `last_synced_at` 오래된 순. limit만 바꾸면 주기가 알아서 조절된다. **증분이 이 설계의 핵심**: `submissions.json`의 accession을 DB와 대조해 없는 것만 XML을 받는다. 실측으로 확인 — 같은 8종목 첫 실행 54초/579행, 재실행 **9초/신규 0행**, 총 행수 579 유지(중복 없음). DART는 접수번호를 안 줘서 행 내용 SHA-1을 filing_key로 썼고, 한 응답에 동일 해시가 둘 나올 때를 위해 line_no로 가른다. **`pace()`가 필요한 이유**: `apibudget.Check`는 간격이 안 지나면 기다리지 않고 **에러를 반환한다** — 요청 경로에서는 맞지만 배치에서는 두 번째 호출부터 전부 떨어진다. **날짜 가드**: Form 4에 거래일·보고기간이 둘 다 빈 문서가 있어, 한 행 때문에 그 종목 INSERT 전체가 죽지 않도록 repo에서 파싱 실패 행을 거른다. **'empty' 상태를 'ok'와 분리한 것**이 화면 문구를 가른다 — 없으면 배치가 아직 안 온 종목이 "내역 없음"으로 보인다(실측: MSFT는 "아직 수집되지 않은 종목입니다"). **실측 검증**: 해외 8종목 579행(AMAT·APP·ARM·AVGO·CLS·NVDA·ORCL·TSM, 관심·보유가 먼저 선택됨 = 등급제 동작 확인) / 국내 6종목 581행(삼성전자·SK하이닉스·현대차·삼성SDI·네이버 등) / 엔드포인트 3케이스(국내·해외·미수집) 정상 / `/health/api-quotas`에 `edgar`(13)와 `edgar_bulk`(193)가 분리 집계. **삽질 하나**: 마이그레이션을 리포 루트에 새 `migrations/`를 만들어 넣었는데 실제 경로는 `be/migrations/`라 api가 기동 실패했다(로그로 즉시 발견, 이동 후 정상). **문서**: `docs/BATCH_SCHEDULE.md` 신설 — 시간표(00:25 점수 / 06:00 해외실적 / 07:00 해외내부자 / 18:30 국내실적 / 20:00 국내내부자)와 버킷 분리 현황, 남은 5개 온디맨드 경로를 한곳에. compose 워커 주석의 커넥션 산식도 7→8워커로 갱신(8×8=64 + API 50 = 114 < 151). 사용자 방침: **갱신이 잦은 데이터(공시·경고)는 등급제가 아니라 전 종목 매일**이 맞다 → 다음 대상에 적용. 릴리스 노트 미작성
- 2026-08-19 | 조회 시점 외부 호출 전면 제거 (남은 5경로) + 배치 관리 현행화 | ok | 앞선 세션에서 내부자 거래를 템플릿으로 옮겼고, 이번엔 나머지를 **일괄** 처리했다. 결과: **사용자 조회 경로에서 외부 HTTP 호출이 0이 됐다**(시세/호가/체결 제외 — 폴러 구조가 따로 있음). **① 공통화 먼저** — 잡이 4개로 늘어나 `insider_sync_state`를 `symbol_sync_state`(kind 컬럼)로 일반화하고 `SyncStateRepo` 하나가 등급제 SELECT를 독점하게 했다. 052를 아직 배포 전이라 새 마이그레이션을 얹지 않고 **파일 자체를 고쳐** 마이그레이션 잔해를 만들지 않았다(로컬 DB만 drop 후 재적용). 복제를 막은 이유는 그 ORDER BY가 등급제의 전부라서다 — 복제되면 우선순위가 잡마다 조용히 갈라진다. **② 공시는 종목별→날짜별 스윕으로 뒤집었다.** DART `list.json`이 corp_code 없이 날짜만으로 그날 접수분 전체를 준다는 것을 컨테이너에서 직접 호출해 확인했고(2026-08-18 실측 397건, `stock_code` 포함), page_count=100이면 하루가 2~4호출이다. 종목별로 돌면 4,400호출이 드는 일이다. 이게 「오늘의 공시」가 **사용자 수 × 보유 종목 수**로 호출이 늘던 유일한 경로였다. **③ 분기 재무에서 basis를 갈랐다 — 이번 작업에서 가장 조심한 지점.** 재무제표 상세 모달과 실적 탭 「분기 실적」이 같은 데이터처럼 보였지만 **다른 것을 보고 있었다**: 국내 DART는 「반기」·「연간」 **누적**으로 보고하고 실적 탭은 그대로 쓰는데, 상세 모달은 4분기−3분기누적으로 **분기값**을 만든다. 한 축으로 합쳤으면 어느 한쪽 화면 숫자가 조용히 바뀌었을 것이다. `basis`를 유니크 키에 넣어 둘 다 담았고, 미국은 원래 두 화면이 같은 EDGAR 응답을 쓰므로 `quarter` 한 벌만 저장한다. 실측 확인: 000660 상세 모달=분기값(2024 3Q 매출 17.5조), 실적 탭=누적(「반기」·「1Q」). **④ 종목 경고는 전 종목 매일**(사용자 방침: 갱신 잦은 데이터는 로테이션이 아니라 전수). 종목별 **교체**로 구현했다 — upsert만 하면 해제된 지정이 영원히 남아 정상 종목에 경고가 붙는다. **⑤ 발견한 버그 두 개, 둘 다 실측에서 잡았다.** (가) **게이트 재진입 교착** — DART 잡 충돌을 막으려 `providerGate`(뮤텍스+간격)를 만들었는데, 공시 스윕이 게이트를 잡은 채 페이지 루프에서 다시 잡으려 했다. Go 뮤텍스는 재진입이 안 되므로 영구 교착이고, 같은 게이트를 쓰는 국내 재무 배치까지 함께 멈췄다 — 10분 넘게 `running`인 것을 보고 발견했다. 게이트는 **실제 HTTP 하나를 감싸는 가장 좁은 범위**에서만 잡도록 고쳤다. (나) **Toss 경고 초당 5회** — 120ms(초당 8회) 페이스로 돌렸더니 429. 응답 헤더가 `limit=5 resetSec=1`로 알려주고 있었다. 250ms + 429 시 1.5초 후 1회 재시도로 바꿔 40종목 0에러. **⑥ API 프로세스의 배경 폴링도 제거** — `MarketDataStore.Run`이 실적(Finnhub)·경고(Toss)·공시(DART)를 주기적으로 받고 있었다. 조회 경로만 고치고 이걸 두면 요청 처리 프로세스가 여전히 외부를 부르고, 인스턴스를 늘리면 같은 일을 인스턴스 수만큼 반복한다. 이제 달력 갱신만 남았다. 죽은 `Ensure*`/`refresh*` 6개 메서드 삭제. 삭제하면서 **`GetTodayDisclosures`가 읽던 Redis 키를 아무도 안 쓰게 되는 것**을 발견해(스탬프·리포트가 발행 시점 공시 스냅샷을 굳히는 데 쓴다) 그 읽기를 표 쪽으로 돌렸다 — 안 봤으면 두 기능이 조용히 빈 값으로 굳었을 것이다. **⑦ 배치 관리 현행화** — 잡이 20개가 되어 `BatchJobCategory`(적재/집계·판정/AI)로 묶고, 커버리지 라벨을 새 키에 맞췄고, 「최대 종목」칸이 공시 스윕에서만 **일수**를 뜻하므로 라벨·기본값·상한을 잡별로 바꿨다. 실행 버튼 비활성화도 전역 → **잡별**로 고쳤다(BE의 중복 방지가 원래 잡 단위 `HasPending`인데 FE만 전역으로 막고 있었다 — 2시간짜리 경고 배치가 도는 동안 다른 잡을 한 번도 못 돌리는 상태였다). **⑧ 문서를 화면에서 읽게 했다** — `be/docs/BATCH_SCHEDULE.md`를 `go:embed`로 바이너리에 심고 `GET /api/v1/admin/batch/schedule`로 서빙, 관리자 화면이 기존 `Markdown` 컴포넌트로 그대로 렌더한다(접이식). 이미지에 파일을 복사하는 방식을 안 쓴 이유는 복사를 빠뜨려도 기동이 성공해서 화면에서만 조용히 비기 때문이다. 문서에는 시간표·버킷 분리·수동 실행 안내(잡별 limit 의미·등급제 순서·커버리지 읽는 법·겹쳐 돌려도 되는지)·겪은 함정을 담았다. **실측 검증**: 6개 잡을 **동시에** 큐에 넣어 DART 3개가 겹치게 만들고 전부 0에러 확인(공시 1s/4행, 국내재무 23s/42행, 국내내부자 26s/481행, 경고 14s/40종목, 해외재무 4s/62행, 해외내부자 46s/262행). 읽기 경로 5종 실측(국내·해외 재무 이력, 실적 탭 분기 누적/분기, 경고 OVERHEATED, 미수집 안내 문구). go build/vet/test green, tsc green, next build green. **미검증 1건**: 관리자 화면의 시각 확인은 못 했다 — Browser 패널이 표시되지 않아 스크린샷이 안 된다(기존에 알려진 제약). 프로덕션 빌드에 `/admin/batch`가 정상 포함된 것까지만 확인. 릴리스 노트 미작성
- 2026-08-19 | FE 배포 + nginx 리버스 프록시 (P0-6) | ok | 사용자가 "리버스 프록시를 앞에"로 결정. 성능이 아니라 **보안**이 근거였다 — Next rewrite가 클라이언트 XFF를 그대로 BE에 넘겨 IP 레이트 리밋이 우회됐다. 산출물: `fe/Dockerfile`(multi-stage + standalone; `.next/static`·`public`은 standalone에 안 들어가 따로 COPY — 빠뜨리면 CSS·JS 404), `next.config.ts`에 `output: "standalone"`(rewrite는 개발용으로 남김, 프로덕션은 nginx가 /api/를 먼저 가로챔), `deploy/nginx/nginx.conf` + `proxy_headers.conf`, compose에 fe·nginx를 **`profiles: ["prod"]`**로 추가. 프로파일로 뺀 덕에 `docker compose up -d`와 `restart.ps1`(워커 8개 명시 실행)은 그대로다 — 평소 `npm run dev` 흐름 불변. **작업 중 내가 만든 버그와 그걸 잡은 경위가 이번의 핵심**: 처음엔 공통 프록시 헤더를 server 블록에 한 번만 적었는데, nginx의 proxy_set_header는 상속이 merge가 아니라 **replace**라 각 location의 `proxy_set_header Connection '';` 한 줄 때문에 X-Forwarded-For가 통째로 사라졌고 막으려던 우회가 그대로 열려 있었다. proxy_headers.conf로 빼서 location 6곳 전부 include 하도록 수정. 이걸 잡은 건 **측정을 고쳤기 때문**이다 — 200 동시 요청 테스트가 세 조건 모두 "전부 통과"로 나와 아무것도 구분을 못 했는데, 경과 시간을 같이 재보니 200건이 ~5초에 퍼져 허용치(60 burst + 30/s×5s ≈ 210) 아래였다. 임시 override 파일로 한도를 1rps/burst5로 낮추자 신호가 분명해졌다: 직접:8080 XFF회전 40/0(우회 재현), nginx:8088 XFF회전 5/35(차단), nginx XFF고정 5/35(동일 = 헤더 무시 확인). 테스트 후 override 파일 삭제하고 기본 한도 복원 확인. 검증: FE 렌더 200(73ms, _next/static 서빙), API 3종 200, SSE 즉시 스트리밍 + **70초 유지**(nginx 기본 60s read timeout을 3600s로, proxy_buffering off). **배포 시 필수로 남긴 것**: `api`의 `8080:8080` 매핑 제거 — 우회가 남는 유일한 경로가 그 포트다(로컬은 테스트 편의로 유지). 회귀: go build/vet/test 0 FAIL, tsc OK, 기본 프로파일에 fe/nginx 미포함 확인. 릴리스 노트 미작성
- 2026-08-19 | 요청 타임아웃 · /rankings 병렬화 · 폴러 문제 발견 | ok | **① 우선순위를 사용자가 바로잡았다** — 내가 "/quotes 목록 콜드 29~46초"를 최악으로 보고했는데 사용자가 "우리 quotes는 FE에서 안 쓰고 있지 않냐"고 물었고, 확인해 보니 **사이드바 네비게이션에 /quotes가 없는 고아 라우트**였다(상세 화면 링크와 loginNext로만 닿는다). 실제로 쓰이는 건 /rankings였고 「실시간 차트」(/market)가 열자마자 count=100으로 부른다. 앞선 "공유 셸에 있다"는 내 서술은 정적 import 추적의 과다 계상이었다(정정). **② /rankings 병렬화** — count 8/25/50/100에서 KR 0.15/0.29/0.61/1.44초, US 0.22/0.49/1.24/2.36초로 건수에 정확히 비례했다(직렬의 지문). 동시도 8로 병렬화해 count=100이 KR 1.44→0.158초, US 2.36→0.168초, 비례도 사라졌다. **사용자가 "병렬화하면 toss 호출량 문제 없나"를 물어 그게 이번 측정의 핵심이 됐다** — Redis INFO stats 델타로 재니 count=100 요청 하나가 Redis 명령 약 700회(항목당 7회)를 냈고 왕복 2ms를 곱하면 1.4초로 실측과 정확히 맞았다. 즉 비용은 Toss가 아니라 Redis 왕복이었고, 격리 측정에서 랭킹 1회당 `rate-limit exceeded` 증가는 0건이었다. 가격 해석 로직은 손대지 않아 랭킹 lastPrice가 /quotes/{symbol} currentPrice와 일치함을 확인(000660 1514000 등), 반복 요청도 결정적. race 검증은 golang 컨테이너에서 통과. **③ P0-2 요청 타임아웃 — 원인이 문서보다 나빴다.** Redis를 pause하고 재니 WriteTimeout 30초가 **아무 역할도 안 했다**: 그 값은 응답을 쓰기 시작한 뒤부터 재므로, 아무것도 쓰지 않고 멈춘 핸들러는 영원히 붙잡힌다 — /quotes/{symbol}과 terminal-snapshot이 120초를 넘겨도 무응답이었다(FE가 2초마다 폴링하는 경로). middleware.RequestTimeout(기본 10초, REQUEST_TIMEOUT_SECONDS)로 컨텍스트 마감을 걸어 12.1초에 응답하도록 했다. **제외 경로를 정하며 하나 걸러냈다** — SSE와 업로드는 예상했지만, `/admin/score/recompute`가 전 종목 점수를 **동기로** 재계산한다는 걸 코드에서 확인하고 /admin/* 전체를 제외했다(배치 트리거는 큐에 넣고 202 반환이라 무관). MySQL DSN에도 timeout/readTimeout/writeTimeout을 **없을 때만** 채웠다(go-sql-driver 기본이 셋 다 무제한, 운영자 지정값은 덮어쓰지 않음). 테스트 10개. **④ 체크리스트 검증 항목 수행** — Redis pause 상태에서 /ready가 degraded+200, /health 200, 뉴스·점수·실적 200으로 nil-safe 동작 확인(설계대로). **⑤ 신규 발견: 배경 폴러가 Toss 한도를 상시 초과** — 처음엔 병렬화 탓인가 의심했으나, API 재시작으로 활성 집합을 비우고 **요청을 하나도 보내지 않은 30초에 417건**(≈14/s)이 나왔다. 분포는 /trades 33 · /orderbook 26로 시세가 아니라 체결·호가다. 랭킹 1회 호출 후에는 오히려 277건으로 줄어(캐시 워밍) 사용자 요청과 무관함이 확인됐다. market_poller.go 주석은 이 압력을 이미 줄였다고 적고 있어 문서와 현실이 어긋나 있다. P2-1(idle CPU 22~26%)과 뿌리가 같을 가능성이 높다 — 측정과 기록까지만 하고 미착수. 회귀: 엔드포인트 11종 200, nginx 경유 FE·API 200, SSE 정상, 패닉 0, go build/vet/test 0 FAIL. 남은 것: 폴러 조사(P2-0/P2-1), /quotes 라우트 정리(사용자가 정리로 결정), 뉴스 목록 SWR, 배포 시 8080 매핑 제거. 릴리스 노트 미작성
- 2026-08-19 | 라우트 정리 · 메인 느림 진단 · P2-0 해결 | ok | **① 라우트/미사용 코드 정리** — `/quotes` 페이지(사이드바 미등록 고아)와 그 전용 코드 제거: QuotesSymbolList·useQuotesList·QuotesDetailPanel·MarketIndicesBar(+Lazy). **중요한 함정 둘을 검증으로 걸렀다**: (가) `QuotesSymbolList`를 grep하면 market/page.tsx가 걸리는데 그건 `QuotesSymbolListFooter`의 부분 문자열이었다(단어 경계로 재확인). (나) `fetchQuotesClient`는 **관심 종목 화면이 filter=watchlist로 쓰고 있어** BE `GET /api/v1/quotes`는 살려야 했다 — 고아는 페이지지 엔드포인트가 아니다. 유입 링크 4곳(상세 「돌아가기」, loginNext 기본값, 헤더 검색 분기)은 `/market`으로 정리. BE는 라우트 170개를 FE 참조와 diff해 10개 후보를 뽑았고, `/health`·`/ready`(인프라), `/media/*`(리포트 이미지 URL이 응답에 담겨 옴)를 제외하니 `/insights/diagnostics`만 문서 언급 0·참조 0이라 핸들러·서비스·모델까지 제거. 나머지 5종(health/api-quotas·market-calendar·market-indicators/prices·rankings/history·watchlist/bulk)은 **문서에 기재**돼 있어(목업이 "예정된 사용처"로 참조하거나 릴리스 노트에 신설 기록) 임의로 지우지 않고 판단 항목으로 남겼다. `/backtest`·`/portfolio`·`/risk`·`/transactions`는 ACCOUNT_FEATURES_VISIBLE=false로 **의도적 파킹**이라 고아가 아니다. 리다이렉트 스텁도 옛 북마크용이라 유지. **② 메인 화면 느림 — 재현 실패, 원인 추정 보고.** Redis maxmemory(evicted_keys 0, 77MB/1GB)·레이트리밋(429 0건)·타임아웃(잘린 흔적 없음)·dev 재컴파일(라우트별 1·2차 모두 0.3초)을 근거로 배제했고, 홈 API 전체가 기동 직후에도 2~76ms였다. 가장 유력한 건 **내가 이 세션에서 API를 8회 이상 재시작**해 인프로세스 캐시가 반복적으로 날아간 것. 재발 시 시각을 받으면 액세스 로그로 특정 가능하다고 안내. **③ P2-0 해결** — 근본 원인은 `enrichSummaryLivePrice`가 **목록 항목마다** `poller.Touch()`를 부른 것. Touch는 "보는 중" 표시이고 폴러는 그 종목을 1초마다 체결·호가까지 통째로 받는다(대상 40개=초당 80호출, 한도 각 15/s). market_poller.go 주석은 목록에 이 처리를 안 한다고 적고 있어 **주석과 코드가 어긋나 있었다**. 7분 배경 관측이 결정적이었다 — 분당 598·414·686·661·585·469·**126**으로 마지막 분 급락이 marketActiveTTL(5분) 만료와 일치, 즉 사용자가 보고 있으면 영구 포화. 조치 셋: (1) 목록 Touch 제거 (2) 폴러 fast 틱에 세션 게이트(`filterLiveSessionSymbols` — shouldLivePollSymbol은 요청 경로 11곳에서 다른 의미로 쓰여 전역 변경 대신 폴러에만) (3) **1·2를 넣자 목록이 회귀했다**(폴러가 안 데워 주니 매 요청이 종목별 업스트림을 침, limit=40에서 **58초**) → `withListMode` ctx 플래그로 목록 경로의 EnsurePrice·directTossPrice·ensurePriceMicrostructure를 차단. 결과 무요청 30초 exceeded 230~253→70, /quotes limit=40 58초→1.65초, limit=100 워밍 0.004초. 검증: 목록 가격이 상세와 정확히 일치(005930 251750 등), SSE 정상, race 통과, FAIL 0. **측정에서 두 번 헛짚었다** — (가) `grep -c "GET /api/v1"`이 Toss 업스트림 로그를 세서 "365 요청/30초"가 나왔는데 실제 들어온 요청은 2건(헬스체크)이었다(액세스 로그 정규식으로 재측정) (나) `grep -oc '"symbol"'`은 줄 수를 세므로 랭킹 건수가 1로 나왔다. 트레이드오프 기록: 아무도 안 보는 종목은 목록에서 배치 마지막가로 보인다(급변동 시 한두 틱). 남은 것: 뉴스 목록 SWR, BE 엔드포인트 5종 판단, 배포 시 8080 매핑 제거. 릴리스 노트 미작성
- 2026-08-19 | 국내 실적 시즌 폴링 C안 (catch-up + 2시간 간격) | ok | 사용자가 C를 비용이 작으면 진행하라 해서 구현. FnGuide는 월당 1호출·상한 60/일이라 시즌 하루 7회면 한도의 1/8. **스케줄**: 시즌(1–2·4–5·7–8·10–11월) 10·12·14·16·18·20시 당월, 20시만 전월(정정). 비시즌 18:30 당월+전월. **기동 catch-up**은 마지막 성공이 지난 슬롯보다 오래면 즉시 재수신. 국내는 짧게 동기로(구멍 메우기), 해외는 비동기 — Finnhub 조각 수신이 수분 걸려 큐를 막지 않게. 배포 실측: 8/14 DB 6→88행, 달력 confirmed 5→87. 8/17–19는 FnGuide에도 0건. 테스트는 시즌 판정·슬롯·전월 여부·catch-up 조건. BATCH_SCHEDULE·api-quotas·관리자 스펙 문구 갱신.
- 2026-08-19 | 국내 실적 달력 14일 이후 공백 조사·배치 설계 제안 | ok | 사용자가 첨부 두 장으로 "14일 이후 데이터/결괏값을 못 가져온다, 마이그레이션 후 결과 폴링 배치가 없는 것 아니냐"고 물었다. **배치는 있다** — `earnings_domestic` 매일 18:30, 8/17 수동 백필 2,791행 + 8/18 스케줄 358행. 공백의 실체는 셋이다. (1) `worker-earnings`에 기동 catch-up이 없다. 종가 워커는 재시작 때 놓친 세션을 메우는데 실적 워커는 `nextSlot`이 미래만 본다. 오늘 19:24 재시작으로 18:30을 건너뛰었다. (2) FnGuide는 추정치 `_E`와 발표치 `_P`를 **같은 월 응답**에 주고, 이미 지난 날짜에도 개별 행을 나중에 붙인다. 라이브 8/14=89건 vs DB=6건이 그 증거. 별도 결과 API를 만드는 문제가 아니라 당월을 시즌 중에 다시 받는 문제. (3) 8/17~19는 FnGuide에도 0건 — 국내 2Q 시즌이 14일에 끝나는 패턴(작년 8/14 200건 이후도 0). 두 번째 스크린샷의 건수+점은 발표 전(추정)이지 빠진 실적이 아니다. 설계만 제안하고 미구현. 선택지: A 기동 catch-up만 / B catch-up+시즌 4슬롯(12·16·18:30·21시, 권장) / C catch-up+시즌 2시간 간격.
- 2026-08-19 | 남은 체크리스트 4건 (뉴스 SWR · 리소스 리밋 · /metrics · P2-1 측정) | ok | **① 뉴스 목록 stale-while-revalidate** — singleflight만으로는 TTL 만료 시점의 요청 하나가 재구성을 기다려 40동시 p50 1.21초였다. `listFromCacheAllowStale`로 만료 여부와 존재 여부를 분리하고, 만료된 값이 있으면 그대로 주면서 `runOnce`로 배경 재구성. **결과 p50 1.21s→0.003s, max 1.41s→0.007s(400배).** 핵심은 ingest 시 listCache를 nil로 만드는 기존 동작을 건드리지 않은 것 — 새 기사가 들어오면 stale 값 자체가 없어 동기 재구성을 타므로 "새 기사는 즉시" 보장이 유지된다(테스트로 명시). **측정에서 한 번 헷갈렸다**: 40동시 첫 시도가 1.145초로 개선이 없어 보였는데, 그 사이 폴러 ingest가 캐시를 비워 의도된 동기 경로를 탄 것이었다. 같은 키로 stale이 확실히 있는 상태에서 재측정해 0.003초를 확인했다. **② 컨테이너 리소스 리밋** — 12개 전부(api 1G/2cpu, redis 1.5G/1cpu — maxmemory 1gb + 오버헤드, 워커 8개 256M/0.5cpu는 YAML 앵커로, fe 512M, nginx 128M). 실측 사용량의 18배 이상으로 넉넉히 잡았다 — 좁게 잡아 OOM kill이 배치를 조용히 죽이는 쪽이 더 위험하다. compose 파일이 CRLF/LF 혼재라 perl 멀티라인 치환이 안 먹어 sed 단일행 삽입으로 우회했고, **docker inspect로 실제 적용을 확인**했다(compose의 deploy.resources가 swarm 없이도 먹는지 확인 필요했음). **③ /metrics** — prometheus/client_golang을 새로 들이는 대신 텍스트 포맷을 직접 찍었다(카운터 몇 개와 이미 구조체로 있는 풀 통계뿐이라). 노출 지표는 "이번 점검에서 몰라서 막혔던 것" 기준: **MySQL 풀의 WaitCount/WaitDuration**(열려 있는 체크리스트 항목 `MYSQL_MAX_OPEN_CONNS` 결정의 근거 — 현재 0으로 풀 충분 확인), goroutine 수(Redis 장애 때 쌓이는지), 상태코드별 요청 수(에러율). nginx는 /metrics를 프록시하지 않아 내부에서만 닿는다(8088에서 404 확인). **④ P2-1 idle CPU 재측정 실패** — 7분 배경 관측을 돌렸으나 그 사이 내가 재빌드를 2회(로그 카운터 리셋으로 확인) 하고 뉴스 테스트를 돌려 60%→0.7%→55%→108%로 진동, **오염된 데이터라 쓸 수 없다**고 판단하고 P2-1은 열어 뒀다. 또 원래 기록(22~26%)은 장 마감 시각 측정이고 지금은 양 시장이 열려 있어 애초에 공정한 비교가 아니었다. **⑤ 문서 정리** — 체크리스트에 완료된 항목의 오래된 사본 5줄(P0-6·P1-2·P1-3·뉴스 singleflight)이 남아 있어 제거. 회귀: 엔드포인트 10종 200, SSE 정상, nginx 경유 정상, MySQL 풀 대기 0, 패닉 0, go build/vet/test 0 FAIL. 남은 것: BE 엔드포인트 5종 판단(문서에 기재돼 있어 사용자 결정 필요), 뉴스 인덱스 마이그레이션(스키마 변경 제외 지시), 배포용 항목들(8080 매핑 제거·시크릿 매니저·CORS 실도메인·미디어 백업·관리형 DB), 부하 테스트, 빈 DB 리허설, P2-1·P2-2. 릴리스 노트 미작성

- 2026-08-21 | v0.9.1 해외 실적 반응 날짜 비교 | ok | calendarDay(KST DATE vs UTC). 장전 49→48. TGT+4.3% HD−0.1%. 8/20 AMC 3건·CRGO 잔류. Docker Tesseract OK. `docs/releases/v0/v0.9/v0.9.1.md`, fe 0.9.1

- 2026-08-21 | 실적 캘린더 월그리드 UI | ok | 이전/다음달 화살표 중앙 배치. 칸 %→상회/하회/예정 절대건수. `@container`로 격자 자체 폭 기준(화면 폭 아님) 반응형: `@2xl`(≈672px) 이상은 라벨+정확한 숫자 3줄(상회·하회·예정 reconcile), 좁으면 아이콘 2묶음(▲▼, 99+ 캡 — 3묶음은 43px에서 한 자리 수끼리도 오버플로해 예정 숫자는 툴팁·클릭 상세로 이동). `fe/src/components/earnings/earnings-month-grid.tsx`

- 2026-08-21 | 해외 과거 실적 서프라이즈 백필 (v0.9.2) | ok | 사용자가 "Finnhub 과거 실적 백필 job을 일회성으로, API 한도 초과·버그 없이"를 요청했고 **요청한 형태로는 만들 수 없다는 것이 첫 결론**이었다. 코드를 쓰기 전에 실제로 호출해 봤다 — `/calendar/earnings`는 과거 30일 밖을 **HTTP 200 + 빈 배열**로 답한다(경계 2026-07-22, 오늘 −30일과 정확히 일치; 07-13~19·2025-08·2024-08 전부 0행). 오류가 아니라 빈 배열이라 "그날 발표가 없었다"와 구분되지 않는다. DB 실측도 맞아떨어졌다: 해외 확정 3,043종목이 전부 1분기씩(2종목만 2분기)이라 카드 최소 표본 4분기를 넘는 종목이 **0개**, 국내는 FnGuide 2년치로 403종목. **08-20 AGENT_STATE의 「해외 2년 1회 백필」 계획은 이 실측으로 정정**된다. 대안 `/stock/earnings`는 종목당 **정확히 4분기**(limit=8·16을 줘도 4행), EPS만, `/stock/revenue-estimate`는 403. 사용자에게 여기서 멈추고 물었고 "새 테이블 + 확정 3,043종목"을 택했다. **가장 중요한 설계 판단은 표를 나눈 것**이다 — `/stock/earnings`가 주는 날짜는 발표일이 아니라 회계기간 종료일인데, `earnings_events`의 upsert가 `event_date = LEAST(event_date, VALUES(event_date))`라 그대로 넣으면 이미 맞는 발표일이 한 달 이른 날로 **덮어써지고**(AAPL 07-30 → 06-30) 히트맵이 분기말에 수백 건씩 뭉친다. 코드를 쓰기 전에 upsert를 읽어서 걸렀다. → `earnings_surprise_history`(055) + 일회성 `cmd/backfill-earnings-surprises`, 카드만 두 표를 합친다. **병합에서 조용히 틀릴 곳 셋을 실측으로 찾아 테스트로 고정**: ① `/stock/earnings`의 4분기에는 캘린더가 이미 넣은 최근 분기가 포함된다(AAPL FY2026 3Q가 양쪽에 est·act까지 동일) → `(symbol, fiscal_year, fiscal_period)` 중복 가드가 없으면 적중률이 왜곡되는데 화면에는 「4/4」처럼 그럴듯하게 나온다 ② 응답이 기간순이 아니다(TGT: 2026-09-30, 2025-09-30, 2026-06-30, 2026-03-31) → 직접 정렬 ③ 정렬 축이 둘(발표일 vs 기간종료일)인데 `ComputeHitHistory`가 앞에서 8개를 자르므로 합친 뒤 재정렬 필요. 한도는 `finnhub_backfill` 신규 버킷 1.5초·45/min·8000/일 — 캘린더 버킷(20초)을 쓰면 3,000종목에 17시간이라 **가른 이유가 기존과 반대 방향**이다. 401/403 즉시 중단, 429만 1회 재시도, **연속 20회 실패 차단기**(한도 소진 시 남은 3,000종목을 75분 헛도는 것 방지 — 코드 재검토에서 발견해 추가). 진행 로그를 루프 끝에 뒀더니 100번째가 빈 응답이면 그 구간이 조용해져 앞으로 옮겼다(실제로 한 번 안 찍혀서 발견). **검증**: 5종목 dry-run → 실제 저장 → `earnings_events` 무변조 확인(AAPL 07-30 유지, us 7,065행·기간 동일)이 핵심이었다. API 재기동 후 카드가 MSFT 4/4(중위 6.30)·NVDA 4/4(2.88)·AAPL 3/4(1.72)·WMT 3/4(1.54)로 뜨고 **손계산과 일치**, AAPL이 5분기가 아니라 4분기인 것이 중복 제거의 증거. TGT는 유의미 분기 3개라 미표시(하한 정상). 국내 6건 그대로. 테스트 18개(병합·정렬·하한·발표일 미유출 12 + HTTP 401/403/429/500/빈배열/파싱 6), go build/vet/test 0 FAIL. **남은 것**: 대상이 `confirmed` 기준이라 NVDA처럼 직전 발표가 30일 창 밖인 1,500여 종목이 기본 후보에서 빠진다(별 작업). FE 카드 하단 문구가 「최근 8분기 기준 / 8분기 중 7번」인데 해외는 4분기 — 행에는 실제 표본(4/4)과 툴팁이 나오므로 문구만 어긋난다, UI라 미변경.

- 2026-08-22 | 해외 과거 캘린더 (EDGAR 발표일) + 2025년 종가 · v0.9.3 | ok | 사용자가 "종목별로 받은 데이터를 DB에 넣었으면 캘린더도 채울 수 있지 않냐"고 물은 것이 이번의 출발점이고, **맞는 지적이었다** — API 한계와 우리 DB의 한계는 다른 문제다. 막던 것은 발표일 부재였다: Finnhub `/stock/earnings`가 주는 날짜는 달력 분기로 **반올림한 기간말**이라, 그대로 올리면 2026-06-30에 3,007종목이 뭉친다(측정해서 보여 줬다). 대안을 찾다 **EDGAR 8-K Item 2.02**가 진짜 발표일을 준다는 걸 확인했고, 사용자가 "B까지 한 번에"를 택했다. **매칭 규칙을 코드 쓰기 전에 8종목으로 검증한 게 결정적이었다** — 45일 허용오차로 잡았더니 AMAT(48일)이 탈락해 60일로 넓혔고(분기 간격 90일이라 안전), NVDA·WMT·TGT처럼 회계연도가 1월에 끝나는 종목은 **gap이 음수**(-42일)라 부호로 거르면 안 된다는 것도 그때 알았다. 가장 중요한 방어는 **단조성 가드**다 — 한 칸만 밀려도 모든 분기가 그럴듯한 날짜를 갖고 화면에는 「채워졌다」로만 보인다. 승격은 `ON DUPLICATE KEY UPDATE id=id`로 기존 행을 한 줄도 안 건드리게 했다(일반 Upsert면 `LEAST(event_date,…)`가 소스가 준 진짜 발표일을 우리 역산값으로 덮는다). **독립 검증이 잘 나왔다**: 겹친 8분기 중 7개가 캘린더 날짜와 정확히 일치했고, 1개는 알려진 TGT 이중 라벨이라 가드가 지켰다. 결과 2,770종목/10,571분기 매칭·실패 0·26분, 캘린더 7,664행 추가(2026-07-20 → 2009-11-13). 뭉침 검사가 최종 증거다 — 2026-06-30이 **0건**(기간말을 썼다면 3,007건), 피크는 실제 시즌 목요일(2026-08-06 533건). **그다음이 이번 세션 최악의 사고였다.** 2025년 종가 투영을 돌렸는데 30초에 잘려서 "끝났다"고 오해하고 연달아 더 띄웠다 — 클라이언트만 끊겼지 **서버 쿼리는 계속 돌고 있었다**. 커넥션 151개가 고갈돼(앱이 이미 114 사용, CLI 기본 풀이 50) MySQL이 멈췄고, 사용자께 관리자 권한 재시작을 **세 번** 요청드렸다(내 세션은 서비스 제어·my.ini 쓰기·데이터 디렉터리 읽기가 전부 Access denied였다). 첫 재시작은 StopPending에서 교착(스레드 186개), 강제 종료 후에도 InnoDB 접근만 무한 대기(SELECT 1은 즉시 되는데 테이블은 전부 멈춤, CPU 0·IO 0). **사용자가 붙여 준 에러 로그 한 줄이 전부를 풀었다** — `Difficult to find free blocks in the buffer pool`. `innodb_buffer_pool_size`가 **기본값 128MB**인데 `candle_bars`가 **1,915MB·1,386만 행**이라, 전체 스캔이 애초에 들어갈 수 없었다. 4GB로 올리자 같은 투영이 **15분+ 실패 → 47초 성공**, 단순 COUNT는 20초 타임아웃 → 4~25ms가 됐다. 즉 내 백필만의 문제가 아니라 앱 전체가 겪던 병목이었다. **내가 세 번 틀린 것**: ① "월 단위로 쪼개면 가볍다" — `dailyCloseSessionBars`에 빈 필터를 넘겨 전체 스캔이라 비용이 같았고, 쪼갤수록 같은 스캔을 반복했다 ② "30초에 끝났다" — 클라이언트만 끊김 ③ "연달아 돌려도 된다" — 중첩으로 커넥션 고갈. 코드 수정: 일일 잡 `IngestStockCloses`가 처음부터 하던 대로 **날짜를 안쪽 스캔에 밀어넣고 30일 리드인**(없으면 구간 첫날 등락률이 통째로 NULL). 종가 페치는 전 종목(13,000+)을 훑는 기존 경로 대신 `-symbols-file`을 추가해 **대상 771종목만** 돌렸다(766 성공, 실패 5는 전부 Toss 미등록). **최종 검증**: 2025년 발표 종가 커버리지 76.5% → **99.7%**, 2025-08-27(candle_bars에 아예 없던 구간)에서 NVDA −3.3%가 손계산(180.17→174.18)과 일치, 2025-10-30에서 애플 −0.4%·어드밴스 오토 파츠 −8.0%. 테스트 8개 추가, go build/vet/test 0 FAIL, tsc OK, Docker 재기동 Tesseract OK. **클라우드 산정도 실측으로 확정**(사용자 질문): DB 3.3GB(legacy 517MB 제외 시 2.8GB) + 앱 5.2GB → 분리 배포 DB 8GB 권장, 단일 호스트 16GB. 관리형 DB는 버퍼 풀을 75%로 자동 설정하므로 이번 문제가 원천적으로 없다. **남은 것**: `candle_bars_legacy_v1` 517MB DROP, CLI 기본 풀 50 축소, Finnhub 인증 헤더 전환, confirmed 기준이라 빠지는 1,507종목.

- 2026-08-22 | 백필 정리 · 운영 안전장치 (v0.10.0) | ok | 사용자가 "일회성 테이블·코드 정리하고 상용 재발 방지 검토"를 요청했다. **가장 중요한 판단은 「일회성」의 경계를 실측으로 다시 그은 것**이다 — 사용자는 `earnings_surprise_history`를 일회성 산출물로 봤지만, 재보니 **런타임 데이터 소스**였다. 카드가 매 조회에 읽고, 730일 창 안에 `earnings_events`에 없는 **966분기**를 공급하며, 지우면 후보에서 빠지는 종목에 스페이스X $1,743B·엑슨 $688B·인텔 $465B·알리바바/ARM(ADR는 8-K가 없어 발표일을 못 붙인다)이 있었다. 그래서 **테이블과 읽기 경로는 남기고**(4MB, 30분 캐시 뒤, 일일 배치가 4분기를 쌓으면 자연히 불필요해지는 전환기 자산), 백필 커맨드 둘도 새 환경 부트스트랩 수단이라 유지했다(스케줄러 미등록). 지운 것은 진단용 임시 커맨드·임시 심볼 파일·키가 남았던 로그뿐이다. 카드 상위 6종목이 `earnings_events`만으로도 4분기를 채운다는 것도 확인해 뒀다(나중에 지울 때의 영향 측정용). **재발 방지 둘**: ① `NewMySQLForCLI`(풀 4) — 사고의 직접 원인이 CLI 기본 풀 50이었다. 운영이 이미 114/151을 쓰는데(api가 `MYSQL_MAX_OPEN_CONNS` 미설정이라 50) 여유가 37뿐이었다. 커맨드 13개 이관, 서버 경로(`server.go`·`worker_bootstrap.go`)는 50 유지, 환경변수 명시 시 우선. ② `CheckInnoDBBufferPool` — 부팅 때 최대 InnoDB 테이블과 버퍼 풀을 비교해 50% 미만이면 경고. 이번 교착이 **CPU도 IO도 0인데 테이블 접근만 영원히 멈추는** 형태라 에러 로그 없이는 원인을 못 찾았기 때문에, 증상 전에 눈에 띄게 하는 것이 목적이다. 실측 로그 `innodb_buffer_pool_size = 4.0GB (최대 테이블 candle_bars = 1.9GB) — ok`. 문턱 50%는 사고 당시가 7%(128MB/1,915MB)였던 것 기준. **상용 설정 권고를 실측으로 문서화**: 관리형 DB(RDS·Cloud SQL)는 버퍼 풀을 인스턴스 메모리의 75%로 자동 설정하므로 이 문제가 원천적으로 없고, 자체 설치면 반드시 명시해야 한다. DB 3.3GB(legacy 517MB 제외 시 2.8GB) + 앱 5.2GB → 분리 배포 DB 8GB 권장·단일 호스트 16GB, 1~2GB 초소형 인스턴스는 max_connections 85~170이라 위험. CLI 실행 규칙도 못박았다(앱 멈추고 하나씩, 클라이언트 타임아웃은 서버 쿼리를 안 멈춘다, readTimeout 명시). 회귀: go build/vet/test 0 FAIL, 서버 풀 무변조 확인, Docker 재기동 Tesseract OK, `/health` 200, 백필 데이터 14,729/11,980행 유지. 남은 것: `candle_bars_legacy_v1` 517MB DROP, Finnhub 인증 헤더 전환, 상용 배포 전 버퍼 풀·max_connections 확정.

- 2026-08-22 | 실적 탭 발표 후 주가 — 등락률에 실제 종가 병기 | ok | 사용자가 실적 일정 탭에서 발표 후 주가를 등락률(예: +10%)로만 보여주면 1년 전 등 오래된 발표를 볼 때 그 시점 주가 수준을 알 수 없다고 지적. `ReactionValue`(fe/src/components/earnings/earnings-shared.tsx)에 `currency` prop을 추가하고 `reaction.close`를 등락률 앞에 `formatReactionPrice`로 함께 표기(KRW는 `110,000원`, USD는 `$110.00`). 호출부 5곳(테이블·이벤트로우·사이드레일·서프라이즈뷰·상세모달) 모두 currency 전달하도록 수정. FE만 — Docker 재기동 없음. tsc 통과, 이미 떠 있던 로컬 dev 서버(포트 3000, 사용자가 별도로 실행 중)로 국내(`302,500원 ↗ +2.7%`)·해외(`$48.30 ↘ -3.4%`) 양쪽 실측 확인.

- 2026-08-22 | 실적 표 「현재가」 폰트를 「발표 후 주가」에 맞춤 | ok | 사용자가 두 열 폰트가 안 맞는다고 지적. earnings-table.tsx의 현재가 셀이 font-mono만 걸려 있고 크기·굵기 지정이 없어 테이블 기본값(12px, 400)을 썼고, 등락률도 text-[10px]로 더 작았다. ReactionValue와 같은 구조(가격·등락률 모두 font-semibold, 셀에 text-[11px] tabular-nums)로 맞춤. 콘솔에서 computed style로 두 셀 모두 11px/600/Geist Mono 일치 확인.

- 2026-08-22 | 모바일 「내 투자」 예시 모드 오류 조사·수정 | ok | usePriceTargets() loadFromServer가 게스트 401을 catch 없이 던져 uncaught rejection. 홀딩스 레일·메모 모달이 sample 여부와 무관히 늘 구독. price-targets.ts에 catch{} 추가. 5개 메뉴 모바일 뷰포트로 재현·수정 확인. tsc 통과. FE만

- 2026-08-22 | 모바일 「내 투자」 예시 모드 오류 진짜 원인·수정 | ok | crypto.randomUUID는 보안 컨텍스트 전용 — 폰이 LAN IP(HTTP)로 개발 서버 접속 시 undefined, PC는 localhost라 안 걸림. bulk-add-holdings-dialog.tsx emptyRow()가 마운트 즉시 호출(레일 안 AddHoldingCard, sample 모드에서도 항상 마운트) → TypeError. 로컬 키일 뿐이라 카운터로 교체. randomUUID 삭제 후 재현·수정 확인. 부수로 price-targets.ts loadFromServer 401 uncaught rejection도 catch 추가. tsc 통과. FE만

- 2026-08-22 | 예시 데이터 실측 시세 전환 | ok | 전주·전월 —의 원인은 DB 결측이 아니라 sample payload에 watchlistMetrics가 없던 것. internal/sample이 Quotes 인터페이스로 시세·과거종가를 읽도록 전환(현재가·전일·전주·전월·52주·담은가/평단/등록가/작성당시). buildWatchlistMetrics→WatchlistMetricsFor 분리로 로그인 경로와 공용. 목표가는 등록가 ±%로 파생. FE sample 가드 제거(실시간 시세 허용). 상태 다양성 단언 테스트 → 스텁 기반 파생 테스트로 교체. go test/tsc/lint PASS, Docker Tesseract OK

- 2026-08-22 | nginx 업스트림 DNS 재해석 + fe 헬스체크 | ok | upstream{}이 컨테이너 IP를 기동 시 1회만 캐싱해 api·fe 재기동마다 502(실측 21시간). resolver 127.0.0.11 + 변수 proxy_pass로 전환, 실측(api IP 강제 변경 172.18.0.2→14) nginx 재기동 없이 즉시 복구 확인. fe 서비스 healthcheck 신설(node -e http GET), nginx depends_on fe: service_healthy로. compose config·4개 컨테이너 healthy 확인

- 2026-08-22 | next dev 강제종료 후 전체 라우트 404 원인·수정 | ok | Turbopack dev 캐시(.next/dev)가 강제 kill로 반쪽 상태 — 매니페스트엔 라우트 있는데 서버는 Next 내장 not-found 응답. .next/dev 삭제 후 재기동으로 해결. 프로덕션은 next build(사전 컴파일)+standalone server.js만 써서 이 캐시 자체가 없어 구조적으로 무관함을 확인

- 2026-08-22 | QA 분석 + 테스트 스위트 신설 (e2e/) | ok | 사용자가 FE/BE 전수 분석으로 user journey·invariant를 뽑고 승인 후 테스트를 구현해 달라고 요청했다. 분석에서 P0~P3 분류와 invariant 60여 개를 정리했고, 승인 후 Playwright 스위트를 만들었다(111 tests: api 8 spec · ui 4 spec). **가장 중요한 판단은 「무엇을 브라우저로 도느냐」의 경계**였다 — 인가·소유권·평단 회계·동결 계약은 전부 HTTP 계약이라 API 계층에서 돌고, 브라우저는 HttpOnly 쿠키 세션·예시 모드 분기·검색 자동완성 폼처럼 **브라우저 없이는 재현 자체가 안 되는 것**만 맡겼다. 계정을 테스트마다 새로 만든 것도 원칙이 아니라 도메인 때문이다(쿨다운·상한이 전부 계정 단위). 쓰는 과정에서 제품 결함 4건이 나왔고 그중 2건은 \`test.fail()\`로 박아 뒀다 — 고쳐지면 테스트가 빨개져서 알려 준다. 관리자 콘솔 스펙 11개는 자격증명이 없어 skip 상태이고, 테스트가 관리자 계정을 만들지 않는 것은 의도다(DefaultAdminEmail로 가입하면 실제 관리자가 만들어진다).' + nl2;

- 2026-08-23 | 캔들 계층 저장 (파티션 + 롤업 캐스케이드) | ok | OCI MySQL HeatWave Always Free(1~2 ECPU/8GiB/50GB) 이전 검토에서 출발했다. 실측으로 병목이 CPU(평균 0.16코어)·IOPS(2.9/s)가 아니라 **1분봉 증가**임을 확인했고(하루 61만 행·1.5GB/월), 사용자가 「삭제 대신 해상도 강등」을 제안해 `1m→3일→5m→7일→10m→14일→30m→30일→60m→60일→삭제` 5계층으로 구현했다. **사용자 제안 중 `60m→1일봉` 한 칸만 반려**했다 — `1d`는 이미 독립적으로 더 정확하고(원본 세션 거래량) 더 길며(2009~), 접으면 같은 PK를 덮어써 `stock_daily_closes`·`stock_scores`까지 오염된다(실측 005930 79,033주 부족). 그 칸은 롤업이 아니라 삭제로 바꿨다. **작업 순서를 「스키마 → 조회 → 적재」로 잡은 것이 핵심**이다: 접어 놓고 못 읽는 구간이 생기면 복구가 안 되므로 조회 계층화를 롤업보다 먼저 배포했다. 되돌릴 수 없는 단계는 전부 사전 승인을 받았고(파티션 ALTER·롤업 실행), 그 전에 운영 표본 4종(조밀 KR/조밀 US/중간/희소)을 샌드박스 표에 복사해 캐스케이드를 전수 대조했다. **샌드박스가 데이터 손상 버그를 잡았다** — 컷오프가 버킷 한가운데를 갈라 거래량이 실행마다 깎이는 문제(NVDA 22:20봉 33,105→22,069→11,036)를 버킷 경계 스냅으로 막았다. 카나리아(2종목) 검증 중 **두 번째 버그**도 나왔다: Toss 백필이 이미 접힌 구간에 1분봉을 되넣어 「고운 계층 우선」 규칙이 완전한 5분봉을 부분 1분봉으로 덮어썼다(005930 255,170→18,402). `src_bars`를 `model.Candle`까지 실어 「덮개가 두꺼운 쪽 우선」으로 바꿨다. 부수적으로 계층 저장 이전부터 있던 **집계 차트 페이지네이션 버그**(5분봉 구멍 / 30·60분봉 겹침)도 커서를 접은 결과에서 뽑도록 고쳤다. 결과: candle_bars 1,915MB→512MB, 1d 배치 스캔 13.9M행→3.7M행(1.59초), 60분봉 차트가 19% 종목만 첫 페이지를 채우던 것에서 전 종목 14일치로. 검증 — 백업 대조 5m/10m/30m 2,897봉 불일치 0, 1d 3,823,273행 전부 src_bars=1, 페이지네이션 16조합×8페이지 중복 0, Go 단위 테스트 15종. `migrations/057`, 백업 `candle_bars_backup_20260822`(검증 완료까지 보존), 레거시 `candle_bars_legacy_v1`·`old_stock_fundamentals` 정리.

- 2026-08-23 | 계층 롤업 검증 + 1일봉 밀림 버그 수정 + 폴러 churn 차단 | ok | 사용자가 롤업 결과를 항목별로 검증해 달라고 요청했다(차트 로딩·배치 시각·계층 전환·60분봉 삭제 타당성·1분봉 재유입 충돌·성능 이점·그 밖의 데이터 문제). **검증에서 세 가지가 나왔다.** ① **1분봉 재유입 충돌은 설계대로 처리된다** — 30개 중 5개만 남은 최악의 부분 재유입에서도 차트는 5분봉 기준 정확값(1,495,495)을 그리고, 다음 배치가 재유입분을 전부 지우며(잔여 0), 5분봉은 `src_bars` 가드로 무변(12개·3,054,390·src=5)이었다. 90일 전 잔여도 캐스케이드 한 번에 정리된다. ② **모달 차트는 영향 없다** — `QuoteDailyChartModal`은 `interval="1d"` 하드코딩이고 1일봉은 롤업 대상이 아니다(모달 15회에 1분봉 증가 6·1일봉 0). ③ **사용자가 「8-14 잔여 1분봉은 왜 있냐」고 물은 것이 두 개의 실제 버그로 이어졌다** — 롤업이 놓친 게 아니라 폴러가 계속 되넣고 있었다. 원인 하나는 `intraday1mFetchTarget`의 gap이 **벽시계**인데 Toss는 **거래시간 봉**을 준다는 것(일요일 gap 1,751분 → 1,766봉 → 2.4거래일 전까지), 다른 하나는 **1분봉 계층이 완전히 빈 종목**을 「저장된 봉 없음」으로 보고 7,200봉을 통째로 받아 오는 것(66종목이 30초마다 왕복). 전자는 `intradayDeltaMaxBars=720`, 후자는 `latestIntradayBarTime`이 **모든 계층**을 보도록 고쳤다 — 접힌 봉도 우리가 이미 가진 것이다. 실측 결과 3일 밖 1분봉 쓰기가 **분당 123행 → 0행**. **별개로 기존 데이터 버그 두 건을 찾아 고쳤다.** 주요 13종목(NVDA·005930 등)의 1일봉 5,192행이 자정이 아닌 15:00에 저장돼 하루씩 밀려 있었고, 같은 날짜에 봉이 두 개(3,995 종목-날짜)라 `stock_daily_closes` 투영이 `ORDER BY bar_time DESC`로 **밀린 쪽을 골라** 랭킹 입력이 틀려 있었다(005930 08-06 종가 235,000, 정답 230,000). 5,192행이 **전부 이후 자정 봉의 정확한 사본**임을 전수 확인한 뒤 삭제 → 재투영 → 점수 재계산. 중복 날짜 3,995→0, 일요일 일봉 117→0, 1일봉 차트 중복 77→0. 재투영이 `readTimeout=30s`에 걸려 한 번 실패한 것은 클라우드 이전 권고(180초)의 실증이다. 그리고 `stock_scores`가 정리 없이 **연 7.5GB**로 자라는 것을 발견해 30거래일 보관을 신설했다(읽는 쪽이 전부 최신 하나만 본다). 배치 시각은 사용자 지시대로 05:30 유지.

- 2026-08-23 | 확장성 분석 + 예시 화면 캐싱 + 만료 토큰 정리 배치 | ok | 사용자가 "트래픽이 늘면 무너질 곳"을 찾아 달라고 요청했다. **가장 큰 것은 예상 밖의 곳에 있었다** — 비로그인 예시 화면이 방문자당 460~623 DB 쿼리를 쓰는데 서버측 캐시가 없었다(nginx `proxy_cache off` + `Cache-Control` 헤더뿐). 가장 자주 열리는 화면이 가장 비싼 화면이라, DB 전체 부하 241쿼리/초 기준 **동시 방문 10명이면 25배**가 된다. 픽스처가 6·5종목 고정이라 Redis 캐시 키가 하나뿐이어서 효율이 최대였다. **사용자가 「빈 값이 캐싱되면 사용자가 오류로 느낀다」고 짚은 것이 정확했다** — 예시 행은 시세를 못 얻으면 통째로 빠지는 구조라(`menus.go`), 시세가 잠깐 흔들린 순간의 짧은 목록을 캐싱하면 TTL 동안 모든 방문자가 깨진 첫 화면을 본다. 그래서 캐싱 조건을 「픽스처 행이 전부 살아 있을 것」(`shouldCacheSample`)으로 두고, 짧으면 응답은 하되 저장하지 않아 다음 요청이 다시 시도하게 했다. 캐시 미스가 몰릴 때를 위해 singleflight도 넣었다. 실측 460~623 → 1~39쿼리. 두 번째로 `user_refresh_tokens`에 삭제 코드가 0건인 것을 발견해 `data_retention` 배치(04:30)를 신설했다 — 활성 사용자당 연 365행씩 쌓이던 것이고, 만료 후 7일 유예를 둬서 재발급 직후의 「없는 토큰」과 「만료된 토큰」을 구분할 수 있게 했다. 큐 경로로 검증(2,737→1,305행). **분석에서 나온 나머지 항목은 보고만 하고 손대지 않았다**: `WatchlistMetricsFor`가 종목당 반복 조회라 관심종목 수 × 사용자 수 × 5초 폴링으로 늘어나는 것, 폴러 `pinned`가 전 사용자 관심·보유 종목 합집합이라 LIMIT 없이 커지는 것(현재 3,671종목, 이미 30초 틱이 스킵되는 중). 그리고 앞선 분석에서 `research_views`를 「조회당 1행·무한 증가」로 적은 것은 **틀렸다** — `UNIQUE(call_id, viewer_key)`로 중복이 제거되고, 조회수를 그 행 수로 세므로 지우면 조회수가 줄어든다. 정리 대상이 아니다. SSE도 `Publish`가 non-blocking이라 설계상 문제가 없었다.

- 2026-08-23 | 관심종목 폴링 부하 제거 + 배치 큐 데드락 수정 | ok | 사용자가 "전일·전주·전월은 변하지 않으니 5초 폴링할 필요가 없다"고 지적했고, 코드로 확인해 보니 정확했다 — 기준선 5개가 전부 1d 종가/일봉에서 나오고 세션 중 정적인데, `filter=watchlist` 폴링이 매번 `enrichWatchlistQuotes` → 종목마다 `buildWatchlistMetrics`를 돌아 전부 다시 조회하고 있었다. **다만 FE 분리 대신 서버측 캐싱을 택했다**: 등락률 계산을 FE로 옮기면 세션별 기준선 규칙(`computeQuoteChange`의 장전·장중·장후 분기)이 서버와 FE 두 곳에 생겨 갈라진다 — 레포에 이미 "데모용 두 번째 구현을 두면 갈라진다"는 같은 판단이 있다. 그리고 캐싱 지점을 `WatchlistMetricsFor` 전체가 아니라 **가장 무거운 DB 읽기 하나**(`listDailyBarsFromDB`, 최대 280행)로 좁혔다 — 기준선을 구조체로 빼면 `current`를 쓰는 가드(`isSamePrice`)가 함께 옮겨져 판정이 미묘하게 달라질 수 있어서다. 읽기만 캐싱하면 값이 바뀔 수 없다. 실측 행 읽기 3,824~26,501 → 133~1,804이고, 캐시 유무로 응답을 대조해 48개 지표 필드가 완전히 일치함을 확인했다. **작업 중에 내가 만든 회귀도 하나 잡았다**: 앞서 큐 워커를 셋으로 늘린 탓에 배치 클레임에서 데드락이 났다(`Error 1213`). 클레임 쿼리가 `(job_id, started_at)` 인덱스로 그 잡의 13행 전부를 잠그고 filesort하는데, `status`가 인덱스에 없어 `done` 행까지 잠기고 갭 락이 인접 job_id에 걸쳤다. `(job_id, status, id)` 인덱스(rows 13→1, filesort 제거)에 `FOR UPDATE SKIP LOCKED`를 더해, 워커를 더 붙여도 재발하지 않게 했다. 동시 큐 투입으로 재현 시험해 데드락 0건 확인. tickFast/tickSlow는 분석과 측정만 하고 코드는 손대지 않았다.

- 2026-08-23 | v0.10.1 — 폴러 확장성 (예열 축소·가격 분리·라운드로빈) | ok | 사용자가 이 작업을 버전으로 묶어 롤백 가능하게 해 달라고 했는데, **git 상태가 의도와 달랐다** — v0.9.0 태그가 이틀 전이고 v0.9.1~v0.10.0은 문서만 있고 커밋이 없어, v0.9.0으로 되돌리면 캔들 계층 저장과 1일봉 밀림 수정까지 사라진다. 그걸 먼저 알리고 「지금까지를 v0.10.0으로 확정, 폴러 작업을 v0.10.1로」 합의했다(커밋은 하지 않고 문서 기록만). **구현 넷**: ① tickSlow 예열 대상을 `cared`(관심∪보유)∪active로 축소 ② `tickPrice`를 tickFast에서 분리(자기 티커·자기 busy 플래그) ③ `RefreshPrices`가 한 묶음 실패로 나머지를 버리지 않게 ④ `capActiveTargets` 라운드로빈. **테스트는 요청대로 「장중처럼」 만들었다** — httptest 가짜 Toss 서버를 세우고 엔드포인트별 실제 호출 수를 센다. 폴러 문제는 「어느 종목을 얼마나 자주 부르는가」라 상류를 몇 번 때리는지 직접 세지 않으면 드러나지 않는다. 실측 `/candles` rate-limit 경고 451→105건(77%↓). **작업 중 내 분석의 전제가 틀린 것을 발견해 정정했다**: 「랭킹 추적분 3,652종목」이라 한 것은 `md:rev:*` Redis 키(TTL 없이 누적된 캔들 리비전 카운터)를 센 것이고 실제 pinned는 44~150이었다. 다만 그 과정에서 **더 정확한 원인**을 찾았다 — pinned에 자기 참조 고리가 있어(tickSlow가 훑으면 EnsurePrices→Touch→activeSymbols→pinned 유지) 아무도 안 봐도 스스로 살아 있었다. 가격 커버리지가 146→44로 줄어 사용자에게 선택을 물었고(정상으로 보고 장중 재측정), 온디맨드 경로가 동작함을 실측으로 확인했다(pinned에 없던 068270 요청 시 캐시 0→1, 현재가 192,200). **Touch 2계층 분리는 하지 않았다** — 배치 가격이 장중에 쓸 만한 주기를 내는지 확인한 뒤 판단하기로 했다.

- 2026-08-23 | 테스트가 찾은 제품 결함 4건 수정 | ok | 사용자가 분석 리포트를 기준으로 실제 제품 결함만 고치고, 수정 영향이 갈 만한 다른 기능까지 회귀 검증하라고 요청했다. **가장 중요한 판단은 「어디를 고칠 것인가」였다** — 예시 모드가 인증 API를 부르는 결함을 처음에는 보유 화면 렌더 지점에서 막았는데, 그 훅과 스토어를 다섯 화면이 공유한다는 점을 근거로 교차 화면 회귀 스위트를 새로 쓰자마자 `/journal`·`/targets`가 같은 401을 내는 것이 드러났다. 렌더 지점이 아니라 스토어에서 막는 것이 맞았고, 그 판단은 추론이 아니라 테스트가 강제했다. 두 번째 판단은 예시 모드의 진실을 주소창에서 모듈 값으로 옮긴 것이다 — 켤 때의 한 프레임 지연을 덮으려던 OR이 끌 때 정확히 같은 이유로 고장나 있었으므로, 덮개를 하나 더 얹는 대신 방향을 뒤집었다. 그러면서 생기는 새 위험(모듈 전역 값이 다른 화면까지 따라감, 실패 시 재시도 폭주)을 스스로 찾아 각각 막고 테스트를 붙였다. 린터가 지적한 effect 내 setState는 렌더 중 조정으로 바꿨는데, 이건 스타일 문제가 아니라 초기화 전 한 프레임이 그려지는 문제라 증상과 직결된다.' + nl2;

- 2026-08-25 | 롤업 장중 검증 → 백업 DROP, 미국장 세션 오판 수정 | ok | 검증은 백업의 원본 1분봉으로 현재 티어를 역산 대조하는 방식으로 했다 — 5m 1,109버킷 OHLCV+src_bars 불일치 0, 연쇄 롤업 10m 3,576·30m 1,306버킷 불일치 0, 장중 5인터벌 라이브 응답 전부 200에 버킷 경계 정확. 일봉 「누락」 5,192행은 전부 TIME=15:00으로 `cmd/repair-candle-tz`가 의도적으로 지운 타임존 중복이었다(컨테이너 UTC가 KST 자정을 전날 15:00으로 기록). `candle_bars_backup_20260822` DROP으로 DB 3.05GB→1.81GB(0.55초). **삭제 전 재확인이 판단을 바꿨다** — 「백업에만 있는 4년치 1분봉 13.9M봉」이라 봤던 것이 실제로는 보존정책 초과분 **14,161봉/30종목**뿐이었다(13.9M은 대부분 현재 테이블에도 있는 최근 구간). 희소해서 백테스트 가치가 없었지만 105KB로 떠 두고 지웠다. **검증이 잡아낸 진짜 버그**: `detectUsSession`이 달력의 `today`만 봤는데, 미국 정규장은 KST 22:30~05:00으로 자정을 넘어 KST 00:00 이후에는 진행 중인 세션이 `previousBusinessDay`에 남는다 — 매 정규장의 뒷부분 4시간 30분이 통째로 "closed"였다. 원래 있던 오판이지만 직전 작업에서 `tickSlow`에 `filterLiveSessionSymbols`를 넣으면서 차트 적재 중단으로 승격됐다(장중 AAPL 1분봉 775초 지연). `detectUsSessionAcrossDays`로 두 날짜를 보게 고쳐 지연 775초→3초, 공백 18봉 소급 적재, 캘린더 rate-limit 매분→2건. 회귀 테스트는 수정 전 실패·후 통과를 확인했다 — 시간대 의존 버그라 테스트가 없으면 재발해도 안 보인다. gofmt가 doc comment의 `''`를 타이포그래피 따옴표로 바꾸는 것(Go 1.19+)도 `daily_close.go`에서 문구 변경으로 회피.

- 2026-08-25 | Toss REST+WS 자격증명 공존 실측·설계 | ok | 「WS를 닫으면 토큰이 죽는다」는 전제를 프로브(`be/cmd/probe-toss-coexist`)로 반증했다 — 정상 종료·강제 절단·종료 +60초 모두 그 토큰의 REST가 200이었고, 같은 토큰으로 재연결도 3/3 성공했다. 진범은 종료가 아니라 재연결이 부른 재발급(`IssueFresh`)이었고, 그것은 프로세스 기동마다 한 번씩 일어난다. 두 앱 × (REST + WS 2연결) 45초에 REST 30/30 200으로 공존을 확인했고 앱당 연결 2·연결당 구독 100·WS는 MARKET_DATA 쿼터 비소비를 확정했다. 얻을 것은 WS 슬롯 200→400과 REST failover 복구. 설계는 docs/TOSS_TOKEN_DESIGN.md, 구현은 승인 대기. 측정 동안 api·worker-fetch를 정지했다가 복구했다.

- 2026-08-25 | OCI 이관 사전 진단 — `docs/OCI_MIGRATION_PLAN.md` | ok | 리포에 소스·compose·스키마가 다 있어 추정 대신 실측으로 썼다: 컨테이너 12개 유휴 RSS 합 237.5MiB(api 37.9), `/metrics`(pool open=8/max=24, wait_total=0), MySQL 3.1일 상태값(85 QPS · Innodb_data_written 2.41GB=평균 9.0KiB/s=29MiB/s의 0.03% · 버퍼풀 히트 99.9896% · 락 대기 22,387건 평균 122ms · Sort_merge_passes 83,861 · Connections 1.33/s), DB 1.84GB/47표. **결론: 병목은 CPU도 IOPS도 아니라 「경계가 없는 것」이다.** P0 12건 중 셋이 실측에서 바로 나왔다 — ① DB 서버 tz가 지금 KST인데 DSN이 `loc=Asia/Seoul`이라 관리형(UTC 기본)으로 옮기면 **TIMESTAMP 85컬럼이 9시간 밀린다**(DATETIME 4컬럼은 안전, 대신 SQL의 `NOW()` 비교가 깨진다 — `repair-candle-tz`를 낳은 그 계열) ② `docker system df` 빌드 캐시 **31.59GB** vs 부트 볼륨 50GiB인데 compose에 `logging:`이 한 줄도 없다 ③ `6379:6379`·`8080:8080`이 0.0.0.0 게시라 인증 없는 Redis 노출 + nginx XFF 덮어쓰기 우회(이미 실측으로 잡았던 레이트리밋 구멍이 되살아난다). 그리고 `import_job.go:265`가 이미지당 고루틴 1개 × tesseract 최대 10패스인데 업로드 장수 상한이 없어 사용자 1명이 2 OCPU를 점유할 수 있다. **HeatWave는 기각** — 데이터가 버퍼풀에 통째로 들어가고(히트 99.99%) 집계는 이미 배치가 표로 만들어 두므로, 가장 희소한 1 ECPU를 change propagation에 쓰는 셈이다. 기존 설계 8가지(워커 분리·SKIP LOCKED 큐·풀 배정 56·캔들 계층·candle_bars 보조인덱스 0)는 건드리지 않는다. 코드 변경 없음 — 문서만.' >> docs/archive/sessions.md && tail -1 docs/archive/sessions.md | cut -c1-120

- 2026-08-25 | 차트·호가 화면 REST → 웹소켓 전환 + 자격증명 공존 구현 | ok | 계측을 먼저 넣어(`quantport_toss_rest_path_total{path}`) 「구독이 붙었는데도 화면의 REST가 그대로」임을 확인한 것이 이 작업의 출발점이었다 — 시청자 1명·30초에 /orderbook +19, /trades +19, /stocks +19이고 같은 구간 웹소켓 프레임은 1,257건이었다. 구독은 폴러만 대체했고 요청 경로(터미널 스냅샷, 캐시 TTL 1초)는 구독을 보지 않았으며, 구독 종목이 배치 prefetch에서 빠지는 바람에 /stocks는 오히려 늘었다(STOCK 버킷 5/s → 시청자 8명에 소진). 체결 링버퍼·호가 메모리 유지 + Redis 0.5초/MySQL 3초 스로틀·스냅샷 조립의 구독 우선(planTerminalSources)·stocks 30초 캐시로 고쳤고, 「조용한 종목」과 「끊긴 구독」을 IsLive/IsFed로 갈랐다(후자는 서버의 subscriptions 확인 + 5분 유예). 자격증명은 IssueFresh를 삭제하고 TokenAfter+CAS invalidate로 공유해 REST failover와 WS 400슬롯을 동시에 얻었다. 최종 실측 20건/40초에 orderbook 0·trades 0·stocks 3, 구독 응답 20/20, 인증 오류 0, 토큰 발급 0. 정확성은 -compare 프로브로 최우선 호가 괴리 0.0000%·체결 순서 일치를 확인했다. 전체 테스트·race(빌더 이미지)·롤백까지 통과. KRX 개장 중 실측은 남았다.

- 2026-08-25 | OCI P0 적용 + 역할별 배포 스택 분리 | ok | 사용자가 배치 코드를 `batch/` 폴더로 옮겨 달라고 했는데 **Go의 internal 가시성 규칙에 막힌다** — 워커 8개가 전부 `internal/service`를 쓰고 `be/internal/...`은 `be/` 트리 안에서만 import된다. 옮기려면 모듈 루트를 올려 630 파일 경로 변경 + 383 파일 import 수정이 필요하고 그러고도 `internal/service`는 공유라 결합이 안 풀린다. 물어봤고 사용자가 **폴더 이동 취소, 역할별 일괄 구동 스크립트로 대체**하기로 결정. 만든 것: `deploy/scripts/{app,batch}-stack.sh` + `_stack.sh`, `deploy/compose/{app.oci,batch}.yml`, `deploy/nginx/{nginx.tls.conf,app_routes.conf,tls_site.conf.example}`, `.env.{app,batch}.example`, `deploy/README.md`. **분리의 핵심 제약을 코드에서 찾았다 — 배치 VM은 자체 Redis를 띄우면 안 된다**(`worker_bootstrap.go`가 Toss 토큰 스토어·apibudget·지수 스냅샷을 API와 Redis로 공유하고, Toss는 client_id당 활성 토큰이 하나라 갈라지면 서로를 401로 만든다). 코드 4건: OCR 동시성을 GOMAXPROCS로 제한(`ocr/limit.go` + import_job 팬아웃 + OMP_THREAD_LIMIT=1) · DSN에 `time_zone=+09:00` 주입 · EnsureSchema 타임아웃 환경변수화 · `/metrics`에 gomaxprocs·ocr 상한 노출. compose: 로그 10m×3 전 서비스 · cpu_shares(api 2048:나머지 1024:워커 256) · stop_grace_period · GOMEMLIMIT=800MiB · **api·redis loopback 게시** · nginx 설정을 NGINX_CONF로 선택. 검증: 전 컨테이너 healthy · nginx→api 200 · GOMAXPROCS=2(호스트 nproc 24) · DSN 타임존 실접속 `session=+09:00`(대조군 SYSTEM) · Go 24패키지 통과(신규 테스트 7) · compose 렌더 3종 · nginx -t 평문·TLS + 음성 대조 · 배치 스택 별 프로젝트로 실기동해 8개 준비 배너 확인. **실행해서 잡은 것 넷**: `maxImportImages=10`이 이미 있었다(직전 보고의 「상한 없음」은 내 오류, 문서 수정) · batch 헬스체크 grep의 `mysql:`이 정상 로그 `mysql: schema verified`에 걸려 8개 전부 오탐 · compose ports는 오버레이에서 추가 병합이라 8088이 VM에 남을 뻔함 · Git Bash 경로 변환으로 첫 nginx -t가 기본 설정을 검사하고 통과(MSYS_NO_PATHCONV=1로 재검증 + 음성 대조 추가).' >> docs/archive/sessions.md && tail -1 docs/archive/sessions.md | cut -c1-100

- 2026-08-25 | KRX 개장 중 재측정 + 토큰 만료 방어 로직 | ok | 국내 정규장에서 같은 측정을 다시 했다 — 폴링 90회(/orderbook 33·/trades 33·/stocks 24) → 구독 1회, 구독 응답 20/20. 국내 정규장은 tickFast까지 같은 호출을 얹어서 미국 세션보다 격차가 컸다. 2부의 미검증 항목이던 국내 호가 10단과 값 정확성(최우선 매도 괴리 0.0000%)을 확인했다. 토큰 만료 방어는 다섯 갈래로 넣었다: 만료 전 토큰 폴백, 발급 재시도(영구 4xx는 제외), 401 재시도 3회, 조회 한정 일시적 재처리, 그리고 측정 중 발견한 「경합 없어도 발급 전 2초 대기」 제거(2.0초 → 0.03초). 검증은 토큰을 실제로 3번 죽여서 했다 — 173/173 성공, retry_401 정확히 +3, issued 정확히 +3, 웹소켓 생존. 셸로 먼저 시도했을 때는 cut이 시크릿을 잘라 아무것도 안 죽인 채 「무중단」으로 보였고, 그래서 프로브가 새 토큰의 200을 확인한 뒤에만 사망으로 기록하게 했다. 실제 24시간 경계는 아직 안 지나 봤다.

- 2026-08-28 | Index "AI 56" badge → rule-based keyword chips | ok | index_tags.go + migration 062 (tags column); FE lib/index-tags.ts maps codes→labels/tones; strip 1 chip, modal 3; contradicting-direction chips hidden intraday; score kept in modal; cell 200→224px.

- 2026-08-28 | usd-krw/sox/nq100 missing sparkline & change rate | ok | market_index_history.go fills gaps from index_daily_closes (30min cache, one query, no new API calls). All 13 indices now charted; usd-krw rate -0.83%.

- 2026-08-28 | be/.env UTF-8 BOM silently disabled all host CLI credentials | ok | godotenv aborts the whole parse on one bad line and Load()'s error was discarded; MYSQL_DSN default masked it. Stripped BOM; config.loadDotEnv() now trims BOM bytes and logs parse failure. Containers were unaffected (compose env_file).

- 2026-08-28 | Index history backfill since 2026-02-01 | partial | 10 indices / 1,435 rows. usd-krw fails: Toss indicator candles return 400 for the USD/KRW symbol, so it only accumulates forward via the daily poller (14 points today).

- 2026-08-31 | 홈 화면 11건 (지수 정합성 · 점수 안정화 · 키워드 서버 집계) | ok | 브리핑 칩을 스트립과 같은 라이브 스냅샷으로 통일. FRED 하루 지연으로 폴러가 전일 종가를 오늘 자리에 찍던 것을 고치고(indexLiveDate) 유령 행 정리. usd-krw는 FRED DEXKOUS로 갈아타 15→148행, btc는 업비트 일봉으로 22→200행 (gold는 여전히 날짜 있는 소스 없음). 동종 그룹 히스테리시스로 소속 이탈 55%→0.4%, 일간 |Δ점수| 6.17→2.79. attachTechnicals가 채점 날짜를 무시하던 것도 수정. 랭킹 등락률 LATERAL 폴백(결측 21%). 지수 키워드 13/13 + LLM 해설(마이그레이션 063, 12/13 생성 8,244토큰). 「한 눈에 보기」 모달. 키워드 집계 서버 이전 100→1,000건(65KB→2KB). 브리핑 개장 1시간 전 슬롯. 실적 카드 기본 해외+날짜순. iOS 검색창 확대 수정. 로고 -store-us + 월간 갱신 스크립트(해외 전량 저장 ~87MB는 보류).';

- 2026-08-31 | 홈 후속 5건 | ok | 퀵룩이 열자마자 닫히던 버그: useModalHistory가 StrictMode의 이펙트 재실행에서 push→back→push→pop으로 자기 pop에 닫혔다(되돌리기를 지연·취소 방식으로 변경). 지수 모달의 규칙 지표 나열·각주 제거하고 국면 라벨을 MoodBadge와 같은 색 배지로. 누적 등락률 칩에 부호 기반 색(방향 없는 지수는 무채색 유지). 랭킹 등락률이 전일 종가 대비인지 검증(20,000행 불일치 0) + 폴백 상한 7일 도입(10일 전 종가로 0.00%를 보이던 해외 5종목 제거). 해외 로고 11,205개 로컬 저장(104MB).' + seol;

- 2026-08-31 | 지수 색 규칙 통일 | ok | 국면 라벨 「상승·중립·하락」으로 축약(옛 두 어절 표기는 접두사로 흡수), 하락=파랑. 원/달러·VIX·미 국채만 칩을 무채색으로 빼던 예외 제거 — 같은 셀의 등락률 숫자가 이미 방향색이라 규칙이 둘로 갈려 있었다. 스트립 칩 데스크톱 2개·모바일 1개(CSS 브레이크포인트, JS 측정 없음). 해설이 점수와 다른 행에 쓰여 화면에서 묻히던 버그 수정(noteTargetDate).' + seol;

- 2026-08-31 | 중첩 모달 · 지수 해설 이력 · 흰 로고 · 홈 상단 높이 | ok | popstate 리스너를 모달마다 달아 둬 위 모달의 back()이 아래까지 닫던 것을 모듈 스택으로 수정(맨 위 하나만, 자기 pop은 삼킴). 지수 해설을 일별 표에서 날짜별로 펼쳐 보게 하고(llm_note/llm_stance를 daily-closes로), index_notes 배치에 이름 부여. 흰 아트워크 로고(SOXL·QQQ·SOXS)에 어두운 판 — 받을 때 밝기를 재 매니페스트 tone에 기록, 기준은 「흰 배경에서 보이는 면적 <5%」(966/15,504). 홈 상단은 뉴스 칸을 행 높이 계산에서 빼 브리핑이 높이를 정하고 뉴스가 채우도록.' + seol;

- 2026-08-31 | 종목 검색 로직 오류 일괄 수정 | ok | 라틴 질의 4글자 이상에서 국내 행을 통째로 제외하던 SQL 규칙 제거 — HPSP·NAVER·SOOP·KT&G·S-Oil이 코드로만 찾아지고 이름으로는 0건이었다. 오탐 방지는 자산군 제외가 아니라 부분 문자열 금지(단어 일치·접두만)로 이동, 국내 english_name은 한 칸 아래로만 인정해 로마자 검색(samsung/hyundai) 복구. 이름 전체 일치 9,000 칸 신설(lg·HD가 남의 티커에 밀리던 문제). 숫자로 시작하는 질의(3M·180)가 심볼 접두만 보고 반환하던 조기 return 제거. IsLatinOnlyQuery를 ContainsHangul로 통일(자모 질의 0건). 파생상품 표지 substring 오탐(Ultragenyx·Aetna) → 단어 단위. 후보 상한 절단 대비 ORDER BY 우선순위 + LIKE 와일드카드 이스케이프. 유닛 테스트 8종 추가, Docker 재기동 후 실측 확인. 남은 공백: 한글 발음→영문 상장명(「네이버」) 별칭 표 미구현.

- 2026-08-31 | 로고 톤 3분류 · AI 요약 배치를 마감 후로 | ok | 어두운 판이 통하는 로고(투명 배경 위 흰 그림, 487개)와 통하지 않는 로고(불투명 흰 이미지, 479개)를 갈라 후자는 모노그램으로 — SOXS가 어두운 동그라미 안 흰 상자로 보이던 것. 종목 AI 요약을 다음 개장 전(08:00/08:30 ET)에서 그날 마감 후(16:30~18:30 KST / 18:00~20:00 ET)로 이동: 읽는 값이 마지막 세션 종가뿐이라 내용은 같은데 마감 후 저녁 내내 어제 분석이 떠 있었다. earliestOn 바닥으로 종가 적재 전 시작을 막고 스케줄 테스트 재작성.' + seol;

- 2026-08-31 | 검색 별칭 표 (한글 발음 → 영문 상장명) | ok | 「네이버」·「포스코」·「숲」·「케이티앤지」가 0건이던 반대 방향 공백을 stock_search_alias.go로 메움. 브랜드 단위 68개(회사 58 + ETF 10), 값은 상장명 표기 그대로(kt&g·s-oil — 기호를 지운 ktg로 적으면 후보 질의 LIKE가 못 찾는다). 입력 중 접두 허용(네이→NAVER), 짧은 접두는 읽기 짧은 순 4개까지. 점수는 이름 매치의 절반이라 한글 이름 직접 매치를 밀어내지 않는다. 표의 모든 값을 DB 상장명과 대조(arirang은 0건이라 제외). 후보 ORDER BY에도 별칭 접두를 실어 1,000행 상한에 강한 매치가 남게 함. 테스트 5종 추가, Docker 재기동 후 13개 질의 실측.

- 2026-08-31 | 검색 자동완성 행 레이아웃 (토스식) | ok | 「코드 · 이름 … 시장」 한 줄에서 이름(위) + 종목코드(아래, 작게) + 오른쪽 현재가·등락률로. 행 높이는 py-2→py-1 + leading-tight로 유지(36 → 36.75px). 시세는 /quotes/batch 한 번으로 받고, 로딩 중에는 자리표시자 대신 폭만 잡아 목록이 흔들리지 않게 함. 0.00%는 무채색(지수 칩과 같은 규칙). 헤더 검색·모달 종목 고르기가 같은 컴포넌트라 동시 적용. 라이트/다크 · 1280px/375px 실측.

- 2026-09-01 | AI 추천 랭킹 등락률 — 회색 표시 제거 · 장중 실시간 · 종가 적재 회귀 수정 | ok | 회색의 원인은 「점수의 날 vs 종가의 날」 비교였다(점수는 KST, 미국 종가는 하루 뒤 = 해외 전 줄 회색). 색은 방향만 뜻하게 하고 각주를 없앴으며 출처는 툴팁(priceAsOf: live/날짜)으로 옮겼다. 그 뒤 진짜 원인: worker-fetch의 fetch-stale이 asOf를 LatestStoredTradeDate에서 유도해 **채우려는 세션이 절대 기준이 되지 못했다** — 실측 08-31 국내 대상 4,287 → 516, 08-31 종가 391종목만 적재. 마감 후 큐잉이 세션 날짜를 asOf로 실어 보내도록 고쳤고(재시작 catch-up도 큐잉), 실행 후 SessionCloseCoverage로 실제 커버리지를 배치 메시지에 남긴다. 랭킹 등락률은 장중(프리·정규·애프터)에 QuotesService.LiveChanges로 덮고 마감 후에는 종가를 쓴다(기준가는 전일 종가로 통일 — 상세 화면의 시간외 기준과 다름). 최근 7일 종가가 없는 종목은 목록에서 제외(ALDF 등, 국내 123·ETF 1,300·해외 2,000). 검증: asOf=2026-08-31로 4,021종목 재적재 확인, 홈·분석 랭킹 회색 0줄·각주 없음·0.00% 무채색.

- 2026-09-01 | 위 작업 사이드 이펙트 3건 수정 | ok | (1) 캔들 예산 고갈 — resolvePrevClose가 캐시 미스 시 fetchPrevClose(전일 정규장 1분봉 페이지네이션)로 내려가, 랭킹 실시간 오버레이가 홈 방문마다 20종목분을 태웠다(API 로그 `candles MARKET_DATA_CHART primary exhausted` 도배). listMode에서는 저장된 1일봉으로 대체(prevCloseFromDailyBars, 오늘 봉은 건너뜀) + 랭킹 행의 prev_close를 힌트로 전달. 수정 후 3분간 고갈 0건, 해외 20줄 중 18줄 live, 0.45초. (2) 전 종목 재수집 뒤 후속 투영이 since=2026-01-01 한 문장이라 readTimeout(i/o timeout → invalid connection) — BackfillStocksFromCandleBarsChunked(45일 창 + lead-in 30일)로 분할. (3) 재시작 catch-up이 매번 전 종목 재수집을 깨움 — SessionCloseCoverage 90% 미만일 때만 큐잉(실측 국내 4,286/4,412 생략 · 해외 0/13,503 필요), fetchDelay 250ms→400ms로 캔들 한도의 절반만 사용. 확인: 08-31 국내 종가 보유 391 → 4,286/4,412, /quotes 목록 등락률 회귀 없음, go test ./internal/... 23개 패키지 통과.

- 2026-09-01 | v1.0.0 — 타이포그래피 통일 (Pretendard 단일 폰트 · 이름 있는 크기 계단) | ok | 폰트는 이미 Pretendard였고 문제는 크기였다: 홈 브리핑 본문 381자가 12px/19.5px, 홈 화면 최대 본문이 14px, 9px 39개·10px 149개. 크기가 하드코딩 1,500곳(text-[11px] 495 · text-[10px] 439 · text-xs 410 · 9px 36 …)에 흩어져 UI_SPEC의 「본문 12px·보조 11px」이 지켜질 방법이 없었다. globals.css @theme inline에 계단 7개 신설(micro 11 · cell 12 · note 13 · read 14/1.7 · article 15/1.85 · section 15 · page 19) 후 192개 컴포넌트 기계 치환 — 사용자 선택은 「A·보수」(밀도 유지, 읽는 글과 하한만 상향). 숫자는 Geist Mono 제거 후 font-num(Pretendard + tabular-nums)로 통일, 고정폭이 실제로 필요한 24곳만 font-code(시스템 글꼴). **함정: tailwind-merge가 모르는 text-*를 글자 색으로 분류해 cn("text-note … text-foreground")에서 크기를 조용히 지웠다** — utils.ts에서 extendTailwindMerge로 스케일 등록(새 계단 추가 시 TEXT_SCALE 갱신 필수). 손으로 판단한 자리: 읽는 글 16곳 승격, 위젯 제목은 15px가 아니라 13px(15px에서 「AI 추천 랭킹 TOP」 4줄 접힘), 제목과 중복되던 링크 문구 2개 제거, 죽은 삼항 7곳 정리, lightweight-charts fontFamily 지정(캔버스라 CSS 미상속 — 앱에서 유일한 비-Pretendard였다), recharts tick 9·10 → 11 (20곳). 검증: tsc·next build 통과, ESLint 신규 0건, / ·/news/domestic ·/analysis ·/login 최소 11px·가로 스크롤 없음, 라이트·다크 · 1280/375 실측. 미실측: 로그인 필요 화면(마켓 레이더·워크스페이스·보유·관심)과 캔들 차트 축 글꼴(코드 확인만).

- 2026-09-01 | v1.0.1 — 컨트롤 높이 통일(A1) · 가격 단위 축소(C2) | ok | ① **견본 충실도 먼저 수정.** 사용자가 「서비스 폰트가 견본과 다르다」고 지적 → 실측하니 **글꼴은 동일**(같은 문자열·같은 스타일 폭이 141.53/81.02/68.06/61.83/80.91px로 소수점까지 일치, 둘 다 Pretendard Variable 1.3.9 가변축 45–920). 다른 건 v1 견본에 내가 임의로 넣은 크기였다 — 헤더 가격 앱 24px/600/자간 normal vs 견본 26px/700/−0.52px, 등락 12px vs 14px, 목록이 세로 3단 vs 가로 한 줄. v2(`docs/mockups/controls-numbers-v2.html`)는 앱 컴포넌트에서 값을 옮겨 오고 **앱에서 잰 기준 폭을 넣어 스스로 대조**한다(어긋나면 배지가 경고). ② **A1**: 종목 헤더 여섯 개가 「같은 스타일 문자열 네 벌」이었고 앱 전체로는 손으로 쓴 알약이 16개 파일 — 한 페이지(/reports) 안에서 높이가 24.2·26.2·28·32·32.2px 다섯 가지, 모서리 세 가지였다. 넷을 `<Button variant="outline" size="sm">`으로 합치고 호출부에는 색(`HEADER_ACTION_CLASS`)만 남겼다. `Button`의 `sm`에 아이콘 13px을 넣어 글자(12px)보다 작게 — 계단 xs 12 · sm 13 · 기본 14. 실측: 여섯 개 전부 28px·12px/500·r8px·아이콘 13px. ③ **C2**: `formatPrice()`가 문자열 하나를 돌려줘 값·단위를 못 나눴고, **직접 호출이 92곳(39파일)** vs 컴포넌트 23곳이라 컴포넌트만 고치면 한 페이지에 축소된 단위와 아닌 것이 섞인다(실측 /analysis 문자열 가격 74개). `PriceDisplay`에 `size="inherit"` 신설(가장 작은 sm이 14px이라 표 셀에서 못 써 호출부가 문자열로 도망갔던 것) + `PRICE_UNIT_CLASS`(0.85em·muted)를 세 컴포넌트에 적용 + **JSX 위치 `{formatPrice(a,b)}` 46건을 27파일에서 기계 치환**. 타입 오류 4건은 행 컴포넌트 `value`를 ReactNode로 확장. 문자열 잔여 46건은 title/aria-label/템플릿이라 그대로. 퍼센트는 건드리지 않음(값의 일부로 읽힌다). 실측: /analysis 단위 축소 75 · 문자열 0, 홈 12 · 0. ④ 곁다리: `stock-daily-close-card` 일별 종가 줄만 tabular-nums 누락 → 좌우 자릿수 어긋남 수정. 검증: tsc·build 통과, ESLint 76 → 75(신규 0). 미실측: 로그인 필요 화면(마켓 레이더·워크스페이스·보유·관심).

- 2026-09-01 | 손수 만든 버튼 전부 buttonVariants로 통합 (v1.0.1 2차) | ok | 앱 전체에 손으로 쓴 컨트롤 116곳 · 크기 지문 35가지. 첫 시도(엘리먼트를 <Button>으로 교체)는 닫는 태그 매칭이 틀려 파일 여러 개가 깨져 되돌렸다 — 중첩 스캔이 mutate 중인 src 인덱스를 참조. 대신 **className만** cn(buttonVariants({variant,size}), …색)으로 바꾸는 방식으로 104곳 변환(구조 무변경 → 조건부 active·pill 모양 보존). 세로 패딩 → 계단 매핑: py-0.5·py-1 → xs(24) · py-1.5 → sm(28) · py-2·px-4 → 기본(32) · h-9 → lg(36), 현재 높이와 가장 가까운 칸이라 눈에 띄는 변화 없음. 남긴 12곳은 버튼이 아님(탭 rounded-t · 실적 캘린더 날짜 칸 · 스탬프 예측 폼 세그먼트 · 세로 패딩 없는 칩). 실측 /reports: 24.2/32.0/32.2px·모서리 8·10·full → 24/28/32px·모서리 8px. **사고: `rm -rf src && cp -r`로 백업 복구하자 Turbopack 파일 감시가 끊겨 전 라우트 404** — 재컴파일 유도로 복구 안 됨, 사용자 승인 후 dev 서버 재시작(PID kill + launch.json fe-dev)으로 해결. next build는 내내 통과했으므로 소스 문제 아님. 검증: tsc·build 통과, ESLint 75(신규 0).

- 2026-09-01 | v1.0.3 — /quotes/batch가 심볼을 조용히 버리던 문제 | ok | **원인은 쿼터도 동시성도 아니라 「목록인데 목록 모드가 아니었던 것」.** \`BatchQuoteSummaries\`가 심볼마다 \`EnsureQuoteDetail\`(상세 화면용 경로)을 부르면서 \`withListMode\`를 안 붙여, 종목당 상류 호출이 목록 길이에 그대로 비례했다 — \`fetchPrevClose\`(전 거래일 1분봉 페이지네이션) · \`refreshTerminalSnapshot\` · \`backfillClosedMarketFromToss\` · \`resolveRegularClose→fetchRegularSessionClose\`. 실측 42심볼 한 요청에 **\`/candles\` 142회**(한도 초과 35 · failover 9 · 타임아웃 47), \`REQUEST_TIMEOUT_SECONDS\`=10s가 컨텍스트를 끊자 미해결 슬롯이 \`filled[i]=false\`로 버려졌다. **반복 시 반환 집합이 매번 달라지는 것**이 심볼 문제가 아니라는 결정적 증거였다(「7개씩 나누면 된다」던 관찰은 그 7개가 늘 성공하던 쪽이라 생긴 착시). 수정: \`withListMode(withOverlayCache(ctx))\` + \`fetchPriceMap\` 가격 배치 1회 선취 + \`ensureQuoteDetailCached\`/\`loadClosedMarketQuoteDetail\`/\`resolveRegularClose\` 세 곳에 목록 모드 게이트. **42개 요청: 29개·13.9초 → 41개·0.32초·상류 캔들 0회.** 마지막 상류 호출은 \`resolveRegularClose\`였다 — \`quoteChangeUsesRegularClose(\"closed\")\`가 true라 마감 세션엔 국내 전 종목이 그 분기를 탄다(US는 \"pre\"라 0건인 것이 단서). 목록에서 0을 주면 기준이 전일 종가로 돌아가는데, 공식 일별 종가 대비 **평균오차 2.47%p → 0.83%p · 정확일치 0 → 15/39**로 오히려 정확해졌고 홈 랭킹과 값이 같아졌다. 제안 ①②③ 중 **①(meta)만 채택** — ②재시도·③상한 축소는 상류 호출을 안 줄여 인프라 제약(OCI Always Free·DB 1 ECPU)에서 피할 방향이고, 원인을 없앤 뒤엔 재시도할 것도 없다. 응답에 \`missing\`·\`truncated\`·\`deadline\` 추가(실측 105심볼 → truncated 5, \`179720\` 머니무브(KONEX, daily_closes 0행) → missing, deadline false). FE: 뉴스 칩의 종목코드 7종 중 **6종은 「못 받아서」가 아니라 \`allArticleSymbols\` cap 60에 잘려 「묻지 않아서」** — 90으로 상향(서버 100에서 역산), 코드 표시 7종 → 1종. **\`Cannot access \x27mentions\x27 before initialization\`은 소스 문제 아님** — 현재 파일에 TDZ 없고 낡은 Turbopack HMR 모듈이 원인, \`.next/dev\`·\`.next/cache\` 삭제 후 dev 재시작으로 소멸(재현 안 됨). 검증: go build/vet/test · tsc · eslint 신규 0. **미실측: 장중 동작**(측정 전부 마감 후) · 로그인 필요 화면.';

- 2026-09-01 | v1.0.4 — 등락률 기준 「전일 종가」·현재가 「시간외까지 끝난 최종가」로 통일 | ok | **같은 종목의 같은 날 등락률이 화면마다 달랐다** — 실측 마감 후 SK하이닉스 목록 **+2.36%** vs 상세 **−0.06%**. 원인 둘: (ㄱ) `computeQuoteChange`가 `after`·`closed`에서 기준을 「당일 정규장 종가」로 갈아탔고 (ㄴ) `resolvePrevClose`의 1순위 랭킹 `basePrice`가 **세션 기준가라 마감 후 오늘 종가로 갱신**된다(`market:refprice:v3:005930` = 260,000 = 그날 정규장 종가). 둘 다 마감 후엔 오늘 종가가 기준이라 나오던 숫자는 하루 등락률이 아니라 **시간외 등락률**이었다(삼성전자 −2.11% vs 실제 −1.16%). 사용자 결정 「시간외까지 마감된 게 진짜 종가」에 따라 현재가는 최종가, 기준은 전일 종가로 통일. `computeQuoteChange(current, prevClose)`로 축소, `resolvePrevClose`는 **저장된 1일봉의 「오늘이 아닌 최신 종가」**를 1순위로(장중·마감 후 모두 안 움직여 마감 순간에 값이 안 튄다), 낡은 시계열은 `prevCloseStaleWindow`(7일)로 걸러 랭킹 기준가에 폴백. **현재가도 경로마다 달랐다** — 마감 세션에서 상세는 업스트림 최종가, 목록은 저장 1일봉(000660 1,652,000 vs 1,692,000) → `resolveBestLivePrice` closed 분기에서 배치 최종가를 1일봉보다 먼저, 배치는 가격맵을 조건 없이 seed. **v0.10 계열의 「마감 후엔 확정 종가」 결정을 뒤집는 것**이라 관련 주석도 수정. FE에도 같은 규칙이 박혀 있어(`quoteChangeBaseline()`이 세션 보고 `regularClose`로 갈아탐) 서버만 고치면 등락률과 기준선이 어긋난다 → 언제나 `prevClose`, 라벨은 상수 `QUOTE_CHANGE_BASIS_LABEL`(「전일 종가 대비」)로. 검증: 목록 vs 상세 **12종목 전부 일치**, 39종목 전부 기준가 = 전일 종가, 시간외 무변동 22종목은 공식 일별 등락률과 정확히 일치, 배치 42심볼 0.29~0.38초·상류 캔들 0회, tsc 통과·ESLint 75(신규 0). 의도된 차이: 시간외에 움직인 17종목은 거래소 공식 종가와 다르다(사용자에게 고지 후 진행). **사고: 전 메뉴 404 — 코드가 아니라 dev 서버였다.** 사용자는 nginx(:80)로 보는데 `.env`의 `NGINX_ROUTES=app_routes.host-fe.conf`가 nginx를 호스트 `npm run dev`(:3000)로 프록시한다. dev 재시작 후 **`fe/package.json` 버전 수정이 Next 16 dev의 자기 재시작을 유발**했고, 그 인스턴스가 `.next/dev` 락은 쥔 채 라우트 해석이 깨져 `/news/*`가 not-found → :3000 사망. PID 강제 종료 후 재기동, 전 라우트 200 재확인(:3000 23개 · :80 11개). **dev 서버가 도는 중엔 `fe/package.json`을 건드리지 말 것.**

- 2026-09-01 | `Cannot access 'mentions' before initialization` 후속 — 원인 미확정, 완화만 | partial | 사용자 브라우저에서 재발(오버레이가 Next를 **(stale)** 표시, `/news/us`). dev 로그에 4회 기록(스택 `src_0azqlys._.js:12608` @ `NewsClient`)됐지만 **깨끗한 탭에서는 다섯 자극 전부 재현 실패** — 콜드 로드 · 공백 편집 HMR 6회 · 국내→해외 클라이언트 이동 · 훅 목록이 바뀌는 구조 편집을 페이지 열어 둔 채 적용 · dev 서버 재시작. 로그 오류에 내 콘솔엔 없는 `[rhwp]` 확장 메시지가 붙어 **다른 브라우저(사용자 탭)** 발신임을 확인. nginx 캐시 가설은 기각(`:80`·`:3000` ETag·Content-Length 동일, `Cache-Control: no-cache, must-revalidate`). 컴파일된 청크에서도 `const [mentions]`(12503행)가 모든 read(12682·12690·12958·13030)보다 앞이라 **현재 빌드에는 TDZ가 없다**. 조치: ⓐ 컴포넌트에서 유일하게 `useEffect` 세 개 뒤에 있던 `const [mentions, setMentions]`를 상단 상태 블록으로 이동(오류가 지목한 바로 그 바인딩이고 `useState`가 이펙트 뒤에 오는 자리를 없애는 편이 안전 — **다만 원인이라는 증명은 못 했다**, A/B에서 옛 배치도 재현 안 됨) ⓑ `.next/dev`·`.next/cache` 삭제 후 dev 서버 재기동. 검증: `/news/us`·`/news/domestic` 콜드 로드 정상(칩 70개, TDZ 없음), 재시작 후 dev 로그 TDZ **0건**, tsc 통과·ESLint 75(신규 0), nginx `:80` 3경로 200. **사용자 탭은 하드 리로드 필요.** 재발 시 재현 조건(직전 동작)을 받아 재조사할 것.

- 2026-09-01 | rhwp 재확인 — 증상 소멸, 원인 미증명 · 「지켜보기」 결정 | partial | 사용자가 확장을 다시 켜고 뉴스 화면을 열었을 때 **오류 0건**(02:09:52, [rhwp] 로그로 확인), 리포터 수신도 0건. 마지막 발생 00:50:43. **그러나 우리가 고친 것이 아니다** — 그 사이 news-client.tsx는 한 줄도 안 바뀌었고 NewsClient가 든 청크(src_0azqlys._.js)도 그대로이며 선언 12503행/첫 사용 12682행으로 위치까지 같다. 달라진 것은 페이지가 추가로 부르는 청크뿐(오류 경계·리포터)이라 **번들 구성이 바뀌면 재발 가능**. v0.9와의 유일한 구조적 차이(quoteSymbols useMemo가 mentions를 참조)는 남아 있으나 증명이 없고 비용도 0이 아니라(실측: 언급 상위 6개 중 국내 2·해외 1개가 등락률 없이 이름만) 사용자 결정으로 **되돌리지 않고 지켜보기**. 재발하면 안전망이 셸을 지키고 화면이 확장을 끄라 안내하며 **원본 스택이 자동으로 로그에 남는다** — 그걸로 정확히 고친다. 임시 검증 코드 잔여 0건 확인, 전 라우트 200.\n" >> docs/archive/sessions.md; tail -1 docs/archive/sessions.md | cut -c1-70

- **2026-09-02** — v1.1.0 실시간 차트 개편: 5초 시세 폴링(`/rankings/prices` 신설) · 랭킹 응답에 종목명 · 자산구분/전일종가 캐시로 DB 조회 658→43 · 목록 가격 5분 staleness 수정 · 헤더/TOP5/안내문단 삭제 · 탭+칩 컨트롤 · 하트/메모 아이콘(앱 전체 통일) · 숨겨진 탭 무한 로딩 수정\n' >> docs/archive/sessions.md && tail -2 docs/archive/sessions.md

- **2026-09-02** — v1.2.0 화면 상단 되찾기 + 브리핑 폴백: 사용자 화면 15곳 `PageHeader` 삭제(홈 171→99px · 관심종목 191→99) · 동작 버튼은 `PageActions`(본문 첫 줄 우측) · 종목 상세 265→149px(검색 입력줄을 돋보기로 · 액션 6개를 관심·메모·⋯로) · 브리핑 LLM 실패 시 재시도 1회(한도 초과·503 제외) 후 직전 AI 문장 승계(`llm_carry`+`carriedFrom`, 3시간·같은 서울 날짜) · `apibudget` 오류를 `ErrExhausted`/`ErrPacing`으로 분류 · 빈 summary도 로그+재시도";

- **2026-09-02** — v1.3.0 재무 비율 누락 수정 + 실적 탭 재작성: `financial_periods`에 비율 칼럼이 없어 적재 때 계산한 마진이 버려지고 읽기 경로에도 계산이 없어 **전 종목의 `operatingMargin`/`netProfitMargin`/`debtRatio`/`currentRatio`가 API에서 통째로 누락**(재무제표 「이익률」 카드와 상세 모달의 비율 선이 처음부터 빈 채였다) → `applyPeriodRatios`를 읽기 경로에 추가 · 분기 축 라벨은 지워진 게 아니라 recharts `preserveEnd`가 솎아낸 것(1440px에서 12개 중 6개) → 두 줄 표기(`3월`/`26년`, 연도는 바뀔 때만)로 375px에서도 12개 겹침 0, **데이터 키는 고유 라벨 유지**(band scale이 같은 값을 한 칸으로 합친다) · 실적 요약을 「가장 최근 발표(분기)」+「확정 실적(연간)」 2층으로 갈라 FY2025가 언제 것인지 명시(`fiscalPeriod`·`periodEnd`를 응답에 추가, 날짜만 — RFC3339로 내면 UTC 되읽기에 하루 잃는다) · YoY/QoQ/서프라이즈 한국어화(공용 `periodLabel`은 실적 일정 화면과 공유하므로 미변경) · AI 분석 탭 일별 목록 월 단위 페이지(**추가 요청 0건** — 이미 받아 둔 60일치를 나누기만)

- **2026-09-02** — v1.4.0 「빈 차트」 다섯 원인 · 지어낸 숫자 제거: AI 분석 목록을 달 단위 → **30거래일 고정 페이지**(달마다 17~23일로 들쭉날쭉했고 월초엔 첫 화면에 1행) · **애니메이션이 원인의 하나였다** — recharts 등장 효과가 rAF로 `stroke-dasharray`를 늘리는데 **창이 가려지면 rAF가 0회/초**라 선이 1.2%(`5.36px 450.29px`)에서 영구 정지, `visibilityState`는 'visible'이라 코드로 알 수 없다 → 6카드+모달 전 시리즈 `isAnimationActive={false}` · **DART 계정명 사전이 좁았다**(실측: SK하이닉스 EPS `기본주당순이익(손실)`, 2024 1Q 매출 `영업수익` — 같은 회사가 분기마다 이름을 바꿔 써서 구멍이 차트 *중간*에 났다; 분기 행 NULL률 매출 21.3%·EPS 32.2%, 누적 표로 메울 수 있는 건 984건 중 **0건**) → 정규화(괄호 통째 제거·기간 표지 제거) + **`sj_div`로 재무제표 구분** + 계층 매칭으로 교체, **부분 일치는 금지**(`수익`을 부분 일치하면 `금융수익`이 매출이 된다) · EDGAR 태그 확장(은행 `RevenuesNetOfInterestExpense`, IFRS `ProfitLossFromOperatingActivities` 등) · **PER·PBR을 실제로 계산**(`buildTrendSeries`는 심볼 해시 시드 의사난수였다 — 이 카드만 연도 축이었던 이유) → 일봉(실측 2021-10까지) × 분기 재무, TTM 4분기 필수, **주식수는 순이익÷EPS로 그 분기 것을 되찾는다**(현재 주식수를 3년 전 자본총계에 나누면 증자가 왜곡된다), PK LIMIT 1 조회 12회 · **추정 폴백 삭제**(현재값 ×0.72~1.00으로 5년치를 지어냈고 **실측 2,516종목**이 보고 있었다) → 「아직 수집되지 않았습니다」 · `?? 0` → `null`(EPS 0 직선이 SK하이닉스 실제 62,044원을 0이라 말하고 있었다) · `-holes-only` 배치 신설(구멍 있는 종목만, 종목당 DART 4호출, 성패는 `CountHoles` 감소로 판정 — 대상이 이미 분기를 가져 「분기 있음」으로는 전부 성공이 된다; 국내 1,151·미국 2,048종목) · 000660 재적재 실측 2024Q1 매출 `—`→12.4조, EPS 12분기 전부, PER 5.1~65.2배. **남은 것: 밸류에이션 탭 `MultipleCard`도 같은 `buildTrendSeries`로 5년 중앙값·Low/Mid/High 판정을 지어낸다 — 별건이라 미착수.**

- **2026-09-02** — v1.4.1 공용 페이지네이션 · 삼성전자 빈 칸 규명 · 「배수」 화면 삭제: 일별 목록을 뉴스 탭과 **같은 번호 페이지네이션**으로(30행/페이지는 이미 그랬고 모양만 달랐다) — `pageWindow`+마크업을 `ui/page-nav.tsx`로 꺼내 **뉴스 탭도 같은 컴포넌트를 쓴다**(두 벌로 두면 반드시 갈라진다), 적재량 60→120행으로 올려 4페이지 · **삼성전자 빈 칸은 딱 1칸(2023Q4 매출)인데 화면에서는 4곳으로 번졌다**(매출 막대 1 + 이익률 2 + 성장률 2 — 다음 분기의 직전 비교값까지 죽는다; 「칸 수」와 「보이는 구멍」이 다른 이유) → DART 원본 확인 결과 **같은 해 두 보고서가 매출 계정명을 다르게 썼다**: 2023 연간(11011)은 `영업수익` 258.9조, 2023 3Q(11014)는 `수익(매출액)` 191.2조, 옛 사전은 연간 쪽을 못 잡아 `nil − 191.2조`로 4분기가 사라졌다 — SK하이닉스와 같은 원인이되 그쪽은 분기끼리, 이쪽은 연간↔분기 사이 · v1.4.0 파서로 재적재하니 **0칸**, 2023Q4 매출 67.8조(=258.9−191.2 계산 일치) · 국내 구멍 종목 배치 실행(1,151 → 706 감소 확인) · **밸류에이션 「배수」 화면 삭제** — 지표 5개 중 4개가 「지표」 화면과 그대로 중복이고 고유한 건 PSR 하나뿐이었으며, 나머지(스파크라인·5Y 중앙/범위·과거5년 대비 Low/Mid/High·산업 대비·NTM 추정)는 **전부 `buildTrendSeries` 의사난수에서 나온 값**이었다(업종 중앙값 `med5y*1.05`, NTM `value*0.92`) — 재무제표 가짜 추세선보다 나쁜 이유는 **장식이 아니라 판정**을 지어낸 숫자에서 뽑았고 `parseValuationSubTab` 기본값이 `multiples`라 탭을 열면 이 화면이 먼저 보였다는 점 · PSR은 지표 판넬로 편입(주가×주식수÷매출, 셋 중 하나라도 없으면 지표를 안 만든다 — 0은 「PSR 0배」라는 거짓말), `analysis-chart-utils.ts` **파일째 삭제**(참조 0건 확인) · **지표 8개 전부에 입문자용 한 줄 풀이**(무엇을 무엇으로 나눈 값인지 + 높/낮으면 무슨 뜻인지 + 어느 쪽이 유리한지; 「저평가」를 단정하지 않고 「~라는 의미입니다」까지만 — 투자 자문이 아니다).

- **2026-09-02** — v1.4.1 후속: 재적재 뒤 남은 결측을 다시 파서 **계정명 패턴 둘을 더 찾았다.** 제일파마홀딩스(002620) `지배기업 소유주지분 기본주당손익` → IFRS 주당이익은 원래 지배기업 소유주 귀속 기준이라 접두사만 붙었을 뿐 개념이 같다, 계층을 다 소진한 뒤 **한 번만** 접두사를 떼고 1순위를 다시 보게 함(`AllowOwnerPrefix`, EPS 전용 — 순이익은 총액과 귀속분이 실제로 다른 값이라 안 켠다). 반면 대상(001680)은 연결재무제표에 `우선주기본주당순이익`만 있어 **정당한 결측**이다 — 채우면 다른 주식 종류의 값을 이 종목 EPS라고 말하게 된다(테스트로 거부를 못 박음). 그 과정에서 **정규화 버그**를 잡았다: 기간 표지를 `ReplaceAll(out,"분기","")`로 지우면 낱말 경계를 넘어 자른다 — `지배기업소유주지분기본주당손익`의 「지분기본」이 걸려 `지배기업소유주지본주당손익`이 됐다. 표지는 늘 `순` 앞에 붙으므로 `당기순`·`분기순`·`반기순` → `순`으로 그 자리에서만 떼도록 수정(테스트가 먼저 잡았다). **운영 실수 기록: 첫 배치가 살아 있는 줄 모르고 두 번째를 띄워 한동안 두 프로세스가 동시에 DART를 쳤다** — 실패 0건·거절 0건이었지만 같은 종목을 두 번 읽어 호출을 낭비했다. 배치 재실행 전 생존 확인 필요. 국내 코호트 1,151 → 602. **다음: 배치 종료 후 `-holes-only` 재실행**(귀속 접두사 수정이 실행 중 바이너리에 없다) + 미국 2,048종목.

- **2026-09-02** — v1.4.1 후속 2: **배치가 705종목을 ok=705로 보고했는데 실제로는 아무것도 안 했다.** 코호트가 한 종목도 안 줄어 파고드니 원인이 세 겹, 전부 「실패가 성공처럼 보이는」 종류였다. ⓐ **DART는 거절도 HTTP 200으로 준다** — 한도 소진 시 `corpCode.xml`이 ZIP 대신 149바이트 `<result><status>020</status>…</result>`를 200으로 주고, 그걸 corp_code 목록으로 파싱하면 오류 없이 **빈 맵**이 되어 24시간 캐시로 굳는다 → 그 뒤 모든 종목이 「corp_code not found」(원인에서 한참 떨어진 곳에서, 종목별 문제처럼 터진다). ⓑ **예산 계기가 0을 가리켰다** — `fetchURLHTTP`만 Check/Record를 하고 **curl 폴백은 안 했는데, Windows에서는 Go TLS 협상이 DART와 실패해 모든 호출이 curl로 간다** → 2만 회를 부르고 020을 받는 동안 `used_day=0 / headroom=18000`, 그래서 `backfillMaxSymbols` 가드가 아무것도 못 막았다. ⓒ **종목당 호출 수를 4로 잡고 있었다** — 실제로는 `FetchQuarterlyHistory`가 (years+1)×보고서 4종=16회 + `FetchQuarterlySnapshots` 4~8회로 **20회 초과**, 1,151종목이면 23,000회로 처음부터 한도 초과 계획이었다. 여기에 내 **동시 실행 실수**까지 겹쳤다. **고침:** `dartErrorPayload`(JSON·XML 두 꼴 모두 인식, 020 → `ErrDartQuotaExceeded`) · 빈 corp_code 맵 설치 차단 · Check/Record를 `fetchURL` 한 곳으로 올려 어느 전송을 타든 잡히게 · `waitForBudget`으로 `ErrPacing`이면 기다렸다 재확인(`MinInterval` 주석이 말하는 용법 — 예전엔 두 번째 호출부터 거절돼 「그 분기 미제출」로 오해됐다) · 한도 소진을 위로 전파해 배치 중단 · 종목당 호출 수 4 → 24/48 정정. 실측(수정 후): `ok=0 failed=1 stopped=한도 소진`으로 즉시 중단, `used_day`가 0→22로 **드디어 움직인다**. 테스트 2종 추가(거절 응답 인식·한도 판별). **교훈: 「ok=N failed=0」을 성공의 증거로 읽지 말 것 — 계기가 고장 나면 배치는 조용히 아무 일도 안 하면서 성공을 보고한다.** 다음: DART 한도 회복 후 `-holes-only` 재실행(국내 602 + 미국 2,048).

- **2026-09-03 (후속)** — Papago 완전 제거: 사용자 결정으로 키를 갱신하지 않기로 해 폴백으로 남기지 않고 코드에서 지웠다(\`PapagoClient\` · \`translate.Papago\` 엔진 · \`PapagoConfig\`/\`PAPAGO_CLIENT_*\` · apibudget 항목 셋 · 리더 본문 번역 \`translateUSReader\`) · **폴백이 없다는 뜻이라** 자체 모델이 실패한 제목은 목록에서 사라지는 게 아니라 **영어로 남고**, 화면이 「번역 안 됨」과 「원래 영어」를 구분하지 않으므로 그 건수는 \`Chain.Snapshot()\`의 Rejected로만 보인다 — 그 수가 조용히 오르는 것이 유일한 위험 · **저장된 리더 본문 번역 32건은 보존**(새로 만들지 않을 뿐), 본문을 자체 모델로 안 옮기는 이유는 제목과 성격이 달라서다(사람이 기다리고 기사 한 건이 2,000단어) · **밀린 번역 14.2%→0.3%**(4,141건 중 11건) · 재비교 15건에서 대등, 자체가 나은 예는 Papago가 \`200 DMA\`를 「200달러 DMA」로 지어낸 건 · 뒤집힌 오역 둘만 규칙 추가(\`oversold\`→과매출, \`wildfire\`→야간 화재) — 어색한 것과 뜻이 뒤집힌 것은 다르게 다룬다 · 한계: 이름 잠그기는 상장 종목만 덮어 인물 이름은 여전히 틀린다.'+nl;

- **2026-09-03** — v1.5.0 사용자 지적 10건: **같은 종목·같은 날짜의 종합 점수가 화면마다 달랐고, 원인은 잣대가 아니라 「주인이 둘」이었다.** `stock_daily_closes.score` 한 칸에 점수 배치(5축·동종 그룹 백분위)와 요약 파이프라인(`stockSummaryScore` — 기술지표+뉴스 감성만, 절대 임계값)이 서로를 모른 채 번갈아 값을 써 넣고 있어서, AI 분석 탭 위쪽 「오늘의 종합 점수」와 바로 아래 같은 날짜 행이 마지막에 쓴 쪽에 따라 다른 숫자를 말했다 → 요약은 그 칸에 **글만** 쓰고(`UpsertAnalysis`에서 score 제거, `stockSummaryScore` 삭제), 조회는 `stock_scores`를 조인해 원본을 읽는다(실측 000660: 66 = 66) · **PER이 가치 축에서 통째로 빠져 있었다** — SK하이닉스는 `eps_basic`이 비어 있고 밸류에이션 탭은 순이익÷발행주식수로 되짚어 PER 27.6배를 멀쩡히 보여 주는데(`enrichDerivedEPS`) 점수 쪽엔 그 갈래가 없어 PBR·PSR 둘로만 매겼고, 그 동종 백분위가 2.6·4.1이라 **가치 축이 3.3점**으로 바닥에 붙어 있었다 → `scoreEPS`(희석→기본→되짚기), 적자는 여전히 PER 없음(「PER 없음」과 「PER 나쁨」은 다른 말) · **심리·수급 축 신설** — 가중치 10%를 차지하면서 언제나 `nil`이던 축을, 요약 파이프라인이 **이미 모으고 있던** 뉴스 감성·내부자 거래로 채운다(`ApplySentiment`, LLM 호출 증가 0). 내부자는 건수가 아니라 **주식 수**로 재고(1주 매수 세 건이 100만 주 매도를 이기면 안 된다) 극단은 15/85로 억제, 커버리지 기여분을 `inputs.sentimentConfidence`에 남겨 **멱등**(두 번 적용해도 coverage 1.0·score 66 고정); 실측 000660 sentiment 42.0 · 0.9→1.0 · 69→66 · 축을 더하자 **같은 화면에 66과 69가 함께** 떠서(브리핑이 적용 전 점수로 조립됨 — §점수통일과 똑같은 버그가 새로 생김) 적용 후 재조회해 브리핑만 다시 쓴다 · **관심 하트가 꺼져 보이던 이유는 세 화면이 관심 목록을 각자 받아 왔기 때문**이고 그 전 프레임은 전부 「관심 없음」이었다(마켓 레이더는 `requestIdleCallback` 뒤로 미루기까지 해서 목록이 언제나 빈 하트로 한 번 그려졌다) → `lib/watchlist-store` 한 벌 + 구독, 두 번째 마운트는 네트워크를 안 기다린다; 부수로 종목 분석 랭킹의 **표시 전용** 하트를 진짜 토글로(`<a>` 안의 `<button>`은 무효라 링크 바깥 형제로) · **「실시간」이 새벽 2시에도 켜져 있었다**(시세 객체만 있으면 찍었다) → 세션 창은 종목마다 다르므로(국내 NXT 08:00~20:00 대 일반 09:00~, 미국은 서머타임) 브라우저에서 다시 계산하지 않고 서버가 토스 달력으로 판정해 내려보내는 `quote.marketSession`을 읽기만 한다; 실측 000660 regular→실시간 · AAPL day→**데이마켓** · 퀵룩 모달·체결 패널도 같이 · 상세 헤더 버튼 **6→3**(관심·메모는 랭킹 행에 이미 있고 리포트 작성은 가장 무거운데 가장 드물다 — 스탬프 생성·보유종목 등록·목표가 설정만, 12→11px, 셋이면 한 줄에 들어가 「⋯ 접기」 분기도 삭제) · **태블릿에서 헤더가 무너지던 이유는 왼쪽 블록이 `flex-1`만 갖고 끝까지 짜부라진 것**(이름 「S…」, 「실시 / 간」 세로 분리, 각주 4줄) → `basis-64`로 최소 폭을 줘 대신 버튼이 다음 줄로 양보, 실측 768×1024 한 줄·잘림 0 · **개요 AI 핵심 요약이 언제나 밸류에이션 이야기였다**(`stockSummaryHeadline`이 `valuation`을 그대로 반환) → `Overview` 신설, LLM이 **같은 호출로** 그 문단까지 쓰고(1,200→1,400 토큰) 없으면 서버가 네 섹션 첫 문장을 엮는다(`composeOverview`, 읽기 경로에서도 채워 기존 캐시 24h 동안 안 좁아진다) · 밸류에이션 카드의 **세 번째 점수**(자체 20~90점 + 「강력 매수/매수/중립/관망」 — 매매 의견이었다) 삭제 후 서버 가치 점수로, 「핵심 요약」 배지·태그 줄(종목명·티커·국내주식·DART — 전부 헤더에 이미 있다)·각주·산출 방식 설명 제거. 테스트 13종 추가, `go build`·`go test ./internal/...`·`tsc --noEmit` 통과. **미결**: §PER은 `stock_scores`를 다시 써야 반영된다(`POST /admin/score/recompute` 또는 야간 배치). **사고**: 검증 중 캐시를 낡게 만들려고 PowerShell로 Redis 키를 되썼는데 `Set-Content -Encoding utf8`이 **BOM**을 붙여 Go의 `json.Unmarshal`이 거부 → 캐시 미스로 판단해 규칙 기반 재빌드, `carryOverNarrative`가 같은 키를 읽으므로 **000660의 09-02 자 LLM 서술이 유실**(다음 배치/관리자 수동 1회로 복구). **교훈: 라이브 Redis 값을 PowerShell로 되쓰지 말 것.**

- **2026-09-03 (롤백)** — 홈 「한 화면에 담기」 되돌림: 사용자가 그 화면을 마음에 들어 하지 않아(카드가 짧아지고 안에 스크롤이 생기는 모습) 홈만 이전 상태로 — 페이지 스크롤 복귀, 「최신 투자 리포트」 카드 복원(실측 1512×820에서 내용 1,402px/뷰포트 764px) · 걷어낸 것은 \`@custom-variant fit\` · page.tsx의 fit 클래스 · dashboard-overview의 행 배치와 리포트 카드 제거 · 카드 넷의 내부 스크롤 \`bodyClassName\` · IndexTickerStrip의 className prop · **git checkout을 쓰지 않았다** — 그 파일들에 세션 전부터 있던 미커밋 변경이 섞여 있어(events-card는 273줄 중 내 것이 2줄, latest-reports-card·news-meta-line은 안 건드림) git으로 되돌리면 기존 작업까지 날아간다, 그래서 내가 넣은 것만 역으로 제거 · **남긴 것**: 「전체 보기 +N」 접힘·오른쪽 정렬, 브리핑 언급 종목 1,000건·10건 이상, 지수 띠 드래그(실측 120px·grab), 사이드바 레일, 뉴스 화면 정리, 로고, 번역, 태그 정정 · 덤으로 앞선 셸 이스케이프 사고로 깨져 있던 dashboard-news-cards.tsx 주석을 원문 복구.'+nl;

- **2026-09-03 (v1.6.0 · 사용자 지적 14건)** — **Redis I/O 99% 감축**: 캔들 스냅샷(`md:snap`, 실측 290,459B)과 형성봉(107B)이 한 키에 있어 `refreshFormingBars`가 2초마다 간격 다섯 개씩 290KB를 읽고 되썼다 → `md:form:{종목}:{간격}`으로 분리, 읽기는 MGET 한 번, 쓰기는 `SET … XX KEEPTTL`(입력 5,510→27 kbps, 출력 6,031→496 kbps, 3초 표본 `md:snap` GET/SET 109/108→0/0) · 랭킹 기준가 조회는 슬롯당 가격 표 20초 재사용(종목마다 18KB×4를 다시 받던 것) · 지수 키워드는 배치가 「지워질 수 없는 태그」를 하나는 보장(FE 모순 필터에 `streak_down:3` 하나뿐이던 넷이 통째로 사라지던 문제, 다음 점수 배치부터) · 뉴스 코인 필터에서 「선물·etf·매수」 등 코인도 쓰는 앵커 제거 + 알트코인 전용 규칙(비트코인은 지수라 제외) · 뉴스 종목 태깅에 한글 별칭표 도입(`NAVER`↔「네이버」, 세 글자 이상·상장명 정확 일치·결과 하나) · 홈 접힘 규약 통일(줄 수 기준·기본 펼침·버튼은 제목 오른쪽, `docs/UI_SPEC.md` §14, `ui/chip-collapse.tsx`) · 홈 「최신 투자 리포트」 삭제 · 모바일 종목 헤더 134→57px(`docs/mockups/stock-header-mobile-v1.html` B안, 동작 셋은 ⋯ 시트, 데스크톱 무변경) · 보유 등록은 종목 고정+현재가 기본값, 목표가는 모달(둘 다 키보드 자동 노출 없음, 비로그인은 저장 때 로그인) · 메모 모달은 폼이 먼저+지난 메모 3건 · 개요 요약 「상세 분석 ›」 우측 상단+태그 1줄 · 밸류에이션 점수 좌측 상단+FY 요약 줄 삭제 · 일봉 오른쪽 16px 확보(「2026-09」 잘림) · Go 27건 + vitest 8건 추가, `miniredis` 테스트 의존성 추가 · **`git stash`를 쓰다 미커밋 작업을 날릴 뻔했다** — 이 저장소는 작업 트리에 세션 전 변경이 잔뜩 있어 stash/checkout 계열을 쓰면 안 된다(pop이 index.lock으로 실패해 파일 둘이 HEAD 상태로 남았고, lock을 지우고 pop해 복구).

- **2026-09-03 (v1.8.0 · 사용자 지적 8건)** — **AI 추천 랭킹에 축·기간이 생겼다**: 「거래대금/거래량」 자리에 종합 + 5축(추세·수익성·성장·가치·심리·수급), 그 아래 실시간/1일/1주/1개월. 축 6 × 기간 4 × 시장 2 = 48개 목록을 **점수 배치가 미리 만들어** `stock_score_rankings`(migrations/064)에 넣고 조회는 그 한 행만 읽는다 — 요청 경로에서 계산하지 않기로 한 근거는 실측이다(개발 PC 기준 축 하나·1주 창 MAX **3.87초**, 1개월 0.61초, 종합 1개월 1.11초이고 운영 DB는 1 ECPU라 몇 배). 한 판을 시장당 「기간 4회 훑기 + 표시용 조회 1회」로 접었다(여섯 측정치를 한 SQL로 받아 정렬만 Go가 한다 — 축마다 따로 물으면 같은 행을 여섯 번 훑는다), **전체 재계산 실측 5.6초 · 48행 × 100종목**. 생성 컬럼 + 인덱스 5개 안은 기각(286,530행 표에 인덱스 다섯을 더 얹는 방향). 최초 1회는 `cmd/compute-score-rankings`, 이후엔 worker-scores가 **prune보다 먼저** 돌린다(보관 기간을 넘긴 행을 지운 뒤 돌리면 1개월 창이 남은 날짜만 본다) · **좌측 목록을 실시간 차트와 같은 부품으로 다시 지었다** — 두 화면을 같은 2단으로 맞춘 뒤에도 목록만 달랐다(한쪽은 툴바, 다른 쪽은 카드 안의 표에 「간단/넓게」 토글과 정렬 헤더). 모양을 `market-radar/radar-toolbar-parts.tsx`가 혼자 쥐고 둘이 그것만 쓴다(클래스 복사로 맞추면 다음에 한쪽만 고쳐지며 다시 벌어진다), 행은 `score-ranking-row.tsx`가 같은 규격으로 그리고 아이콘 버튼은 마켓 레이더 것을 그대로 import, 메모 창은 `RankingMemoTarget`으로 일반화해 둘이 공유. `score-ranking-panel.tsx` 삭제 · **종합 점수 카드를 AI 분석 → 개요 맨 위로**(사용자 결정) — 점수는 거의 모든 종목에 있는데 AI 글은 17,916 중 **744종목(4%)**뿐이라, 널리 쓰이는 숫자가 가장 드물게 열리는 탭 뒤에 있었다. 높이는 숫자를 머리줄로 끌어올려 줄였다(점수·등급은 제목 옆, 동종 순위·커버리지는 날짜 위 두 줄; 실측 모바일 375px에서 머리줄 한 줄) · **「AI 핵심 요약」은 LLM 글이 있을 때만 선다** — 규칙 문장으로 채우던 것을 그만뒀다(그 문장은 바로 아래 점수 카드가 숫자로 말하는 것을 되풀이하고, 그 제목 밑에 규칙 글이 있으면 제목이 거짓말이 된다) · **그 글이 캐시 만료로 사라지고 있었다** — 이어 받는 유일한 경로가 Redis(24시간)라 하루 동안 아무도 안 연 종목은 돈 주고 만든 글을 잃었다 → `stock_summary_narratives`(migrations/065)에 종목당 한 행, 캐시가 비면 거기서 되살리고, 캐시에서 살린 글도 표에 한 번 더 적어 기존 종목이 하루 안에 이관된다(실측: 000660 캐시가 6시간을 넘긴 첫 조회에서 행 생성, analyzed_at은 글이 실제로 쓰인 17:55 KST) · **요청 로그에 KST 시각과 사용자** — Next dev 로그는 상태·소요만 말하고 형식을 바꿀 수 없어 프록시가 한 줄을 앞에 찍는다(`[2026-09-03 23:10:22.418 KST] user=42 name=홍길동 GET /news/domestic`), 사용자는 쿠키 토큰에서 읽되 **검증하지 않는다**(인가는 전부 BE), BE 토큰에 `name` 클레임 추가(옛 토큰은 이메일 로컬 파트로 버티고 15분 뒤 채워진다), 시각은 ICU에 기대지 않고 +09:00 고정 오프셋 산술. 이 참에 `middleware.ts` → **`proxy.ts`**(Next 16 폐기 규약, `/dashboard` 307로 동작 확인) · **실시간 차트 시계가 5분마다 튀던 것**은 그 자리가 순위 스냅샷 시각이어서였다(가격은 1초마다 갱신) → 장중엔 마지막 시세 응답 시각 + 초록 점, 마감엔 스냅샷 시각 + 회색 점(실측 US 장중 11:55:07 → 3초 뒤 11:55:10, KR 마감은 「5분 주기 순위 갱신」 툴팁 + 회색) · **접힌 사이드바에서 상위 메뉴 제거**(관리만 남김 — 하위가 「뉴스」·「API」처럼 짧아 그 줄이 맥락이다), 하위 이름을 스스로 설명하게 바꾸고(차트→실시간 차트, 랭킹→AI 랭킹, 국내→국내 뉴스/국내 실적) 64px에서 두 줄까지 `break-keep`으로 접는다(실측 12개 전부 한 줄에 들어가고 잘림 없음) · **버그 둘**: 스탬프 폼이 화면 밖에서 열렸다 — `absolute right-0 w-[292px]`가 버튼 오른쪽 끝(x=246)에 맞춰지며 왼쪽이 **x=−46**이 됐고 조상의 `overflow:hidden`이 잘라, 예상 주가 칸 왼쪽에 손이 닿지 않았다(폭을 줄여서는 못 고친다 — 기준이 버튼이다) → `sm` 미만에서 `fixed`로 화면에 매단다(`lib/anchored-popover.ts`, 같은 모양 팝오버 둘에도 적용; 실측 375px에서 x=12·폭 351·세로 안 넘침, 768px에서는 기존 방식 유지) / 검색 결과가 안 사라지던 것은 옵션이 `onMouseDown`을 막아 입력이 포커스를 쥔 채 남고 「빈 입력 + 포커스」가 곧 **「최근 본 종목」의 조건**이라 방금 고른 종목이 얹힌 목록이 같은 자리에 다시 뜬 것(모바일은 자판까지) → 고르면 포커스를 뗀다(실측: 삼성전자 선택 후 패널 없음·입력 빈 값·포커스 해제·주소 `?symbol=005930`) · **덤**: 모바일 개요 일봉 카드 밑 **흰 띠 40px** 제거(차트 컨테이너 `min-h-[240px]`가 못박혀 200px 차트 아래가 비었다 — 실측 카드 295→255px), 「밸류에이션 →」 하단 전폭 링크 삭제 · **되돌린 계획**: 규칙 기반 「시장 분석」 카드를 AI 핵심 요약의 대체재로 키우려던 안(샘플 3안까지 만들었다)은 사용자가 점수 카드 이동으로 방향을 바꿔 중단 — 시장 분석 카드는 손대지 않았다 · 테스트 vitest 45 → **85건**, Go **+8건**, `tsc --noEmit` 무오류.

- **2026-09-04 (v1.9.0 · 사용자 지적 8건)** — **장이 닫힌 뒤에도 화면은 계속 묻고 있었고, 같은 종목이 두 가격이었다**: 089790 제이티가 같은 순간에 AI 추천 랭킹 5,200원 +6.78% · 종목 상세 5,100원 0.00%. 둘 다 틀렸고 원인이 달랐다 — 목록은 표시용 종가 조인에 `c.trade_date <= sc.trade_date`가 박혀 있어(점수 배치는 00:25에 **전날치**를 매기므로 국내 마감 15:30부터 다음 배치까지 **매일 아홉 시간** 이틀 전 종가를 들었다) 상한을 걷었고(하한 7일은 「거래 끊긴 종목 제외」라는 별개 규칙이라 유지), 상세는 `prevCloseFromDailyBars`가 「그 시장 기준 **오늘**」 이후 봉을 건너뛰는데 KST 09-04 00:59에 현재가가 09-03 종가이고 「오늘」이 09-04라 그 봉이 스스로의 기준가가 됐다 → **`priceSessionDate()`**(현재가가 속한 세션의 날짜 — 장중이면 오늘, 닫혔으면 마지막으로 닫힌 세션)로 바꿨고 그 값이 기준가 캐시 키이기도 해서 개장하는 순간 캐시가 함께 넘어간다. 실측 양쪽 **5,100원 −1.92%** · **거래소 달력이 세션의 경계 시각까지 답한다**(`be/internal/service/market_session_clock.go` 신규) — 어제·오늘·내일 세 영업일의 거래 창을 모아 `sessionClosedAt`(마지막으로 끝난 창)과 `nextSessionAt`(가장 이른 다음 창)을 고른다(세 날을 다 보는 이유는 `detectUsSessionAcrossDays`와 같다 — 미국 정규장은 KST 자정을 넘는다). 「다음」은 정규장이 아니라 **프리마켓**이다(사용자 요구: 「국장 기준 오전 8시와 같이 장이 일부 열리는 시점부터는 호출」). 실측 국내 마감 중 `sessionClosedAt=2026-09-03T11:00Z`(KST 20:00) · `nextSessionAt=2026-09-03T23:00Z`(KST 08:00), 같은 순간 해외는 `regular`에 두 시각 모두 빈 값 · **마감 중 시계가 「지금」을 찍지 않는다** — 상세 헤더는 `updatedAt`(응답을 만든 시각)이라 새벽 0시 33분에 열면 「오전 12:33:08 기준」이라고 적었는데 그 값은 전날 20:00에 멈춘 것이다 → 날짜와 함께 `09.03 오후 08:00:00`, 실시간 차트 툴바도 **같은 함수**(`lib/market-session.ts`의 `formatSessionStamp`)를 쓴다(같은 순간을 두 화면이 다르게 적으면 어느 쪽이 진짜인지 헷갈린다), 시계 우선순위는 **마감 > 실시간 시세 > 순위 스냅샷**(전환 직후 앞 시장의 `pricesAt`이 남아 마감 목록 위에 「1초 전」을 찍는 것을 막는다) · **값이 바뀔 수 없으면 묻지 않는다** — 마감이면 `terminal-snapshot` 2초 폴링·체결 SSE·`candles/snapshot` 2초 폴링·봉 SSE·`rankings/prices` 1초 폴링을 전부 걷는다(**실측 `/market?market=KR&symbol=000660` 16초에 33건 → 3건**), 같은 코드로 해외 장중은 무변경(10초에 terminal 5·candles 6·prices 11), 깨어나는 길 셋은 `nextSessionAt` 타이머(개장 2초 뒤)·60초 순위 폴링·탭이 다시 보이는 순간이고 시각을 못 받으면 5분마다 되묻는다(「모른다」를 「닫혔다」로 굳히면 화면이 개장을 영영 못 알아챈다) · **시장 전환 시 시계가 앞 시장 값을 들고 있던 것**은 `pricesAt`만 지우고 `data`를 남겨서였다 → 둘 다 지우고 새 목록을 받은 그 자리에서 가격을 한 번 당긴다(**실측 국내 648ms · 해외 774ms**에 확정) · **로그의 한글 깨짐(`name=? ????? ?`)은 프록시가 아니라 PowerShell이었다** — `access-token-claims.ts`는 바이트 → `TextDecoder('utf-8')`로 제대로 풀고 `request-log.test.ts`도 통과한다. 깨진 곳은 ⓐ `[Console]::OutputEncoding`이 콘솔 코드페이지(949)라 Node의 UTF-8 바이트를 그걸로 디코딩하고 ⓑ 5.1의 `Tee-Object`가 `-Encoding`이 없어 파일을 **UTF-16LE**로 쓴 것 → `deploy/scripts/Utf8Log.ps1`의 `Start-Utf8Log`(자식 출력 UTF-8 읽기 + BOM 없는 UTF-8 쓰기 + 옛 UTF-16 파일은 `.utf16.bak`으로 회전)와 `Utf8Log.Tests.ps1`(한글을 stdout/stderr로 내보내는 Node 프로그램을 통과시켜 파일을 UTF-8로 되읽어 비교, 7개 검사). **부수 발견: `.ps1`은 BOM이 없으면 5.1이 ANSI로 읽어 한글 문자열이 깨지고 파싱까지 죽는다** — 손댄 스크립트 넷 + `be/scripts/restart.ps1`에 UTF-8 BOM 추가. 같은 병의 다른 자리인 **바이트 자르기** 셋(`sync_state_repo.go`의 `message[:255]`, `holding_trades.go`의 `Note[:200]`, 이미 옳던 `price_target.go`)을 `internal/textutil.TruncateRunes`로 통일(MySQL VARCHAR도 utf8mb4에서 문자를 센다) · **나머지 넷**: `RadarSegmented`가 블록이라 폭 100%였고 「심리·수급」 오른쪽에 빈 테두리가 남던 것 → `w-fit max-w-full`(실시간 차트 쪽은 flex 행이라 이미 내용 크기였으므로 이 한 줄로 두 화면이 같아진다) / 접힌 레일에서 **그룹 이름을 전부** 제거(v1.8.0이 남긴 「관리」 한 줄이 레일 중간에 홀로 선 제목이 되어 걷어낸 이유를 그 자리에서 되돌리고 있었다)하고 관리자 하위를 「뉴스 관리」식으로 바꿨으며 폭 64 → **72px**(10px 「실시간 차트」가 51px인데 64px에서 글자에 주어지는 폭이 52px이라 한 글자 차이로 두 줄, 실측 72px에서 59px·12항목 전부 한 줄) / 모바일 스탬프 폼·확인 패널을 **정중앙 `Dialog`**로(v1.8.0이 화면 아래 가장자리에 붙여 잘림은 막았지만 467px짜리 판을 자판이 덮었다, 옆 버튼 둘은 이미 모달이었다, 실측 375×812에서 x=25·y=171·326×470) / 개요 종합 점수 카드에서 「커버리지 %」·「서버 산출 · 날짜」 제거하고 **모바일에서도 `56 / 100`**(커버리지 경고 문장은 남긴다 — 그건 이 종목에만 해당하는 사실이다) · **CSS 변수 변경이 dev 서버 캐시에 걸려** 파일이 72px인데 서버는 64px을 계속 내보냈다 — 재기동으로만 풀렸다(`css_ui_regression_caution`의 그 증상) · 테스트 Go **+11건**(세션 시계 6 · 글자 자르기 5), vitest 85 → **89건**, PowerShell 7개 검사, `tsc --noEmit` 무오류.

- **2026-09-04 (v1.10.0 · 사용자 요청 3건)** — **가격을 묻지 않고 받는다**: 화면마다 1·2·5초로 흩어져 있던 가격 갱신을 **2초 하나로 통일**(BE `marketPricePollInterval` 1→2초, 마켓 랭킹 1→2초, 보유·관심 5→2초, AI 랭킹은 **갱신 자체가 없었다**)하고 숫자를 `fe/src/lib/price-cadence.ts` 한 곳으로 모은 뒤, 그 갱신을 폴링에서 **다심볼 SSE 푸시**로 옮겼다 · **왜 푸시인가**: 값을 만드는 일은 서버 폴러가 이미 2초에 한 번 하고 있고 폴링이 늘리는 것은 **꺼내 가는 왕복**뿐이라 그것만 뷰어 수에 정비례한다 — 실측 요청당 `/rankings/prices` 4 SELECT · `/quotes/batch` 20(종목당 1) · **`/stocks/scores/top` 100**(종목당 5, 기준가 해석)이라 AI 랭킹을 2초 폴링으로 만들면 **뷰어당 초당 50 SELECT**이고 운영 DB는 1 ECPU다 · **WebSocket이 아니라 SSE**(사용자 선택): 가격은 서버→클라 단방향이라 WS의 양방향이 살 자리가 없고, SSE는 이미 이 저장소에서 돈다(`/quotes/{symbol}/stream`·`quote-stream`·허브·하트비트·오버플로·메트릭·테스트, nginx `proxy_read_timeout 3600s`, `http2 on`이라 오리진당 6연결 제한도 없다) — WS로 가면 Upgrade 프록시·인증 핸드셰이크·재연결 백오프·ping/pong을 새로 쓰고 **얻는 게 없다**(EventSource는 재연결을 브라우저가 한다) · **설계**: 발행자는 `MarketDataStore.cacheMarketPrice` **한 곳**(가격이 캐시에 써지는 유일한 길목이라 배치 폴러 `tickPrice`와 토스 웹소켓 체결 `applyStreamPrice`가 둘 다 지난다), 구독은 `listPriceHub.SubscribeMany`가 **채널 하나를 심볼 수만큼의 집합에 넣는다**(심볼마다 채널을 만들어 고루틴으로 합치면 뷰어당 고루틴이 심볼 수만큼이다), `HasSubscribers`로 아무도 안 보는 종목은 이벤트를 **만들지도 않는다**(가격 쓰기는 추적 종목 수백 개에 2초마다 돈다), 이벤트는 **가격만**이고 기준가(`basePrice`, 신규 필드)는 세션 안에서 안 변하니 목록이 뜰 때 한 번 받아 등락률 산수는 화면이 한다(서버가 틱마다 종목별로 풀면 그 100 SELECT가 그대로 돌아온다) · **마감이면 붙지 않는다**(사용자 추가 요청): 핸들러가 열기 전에 시계를 보고(v1.9.0의 `MarketClockFor`) 닫혀 있으면 `session` 이벤트 하나만 주고 끊으며 그 안의 `nextSessionAt`으로 화면이 개장하는 순간 다시 붙는다, 열려 있는 동안에도 30초마다 다시 보고, **「모른다」를 「닫혔다」로 굳히지 않는다**(달력 한 번 실패에 화면이 영영 멈춘다) · **폴링은 안 없앴다** — 스트림이 붙은 동안 30초로 물러나고 끊기면 2초로 돌아온다(SSE가 조용한 것과 「가격이 안 변한 것」은 화면에서 구분되지 않아서, 폴링까지 0이면 낡은 값을 든 채 멀쩡해 보인다) · **실측**: 마켓 레이더 45초에 `/rankings/prices` **1건**(1초 폴링이면 45, 2초면 23) · 브라우저 수신 **~1.85초**(4종목 24.1초에 52이벤트) · API 직접 20초에 44 · nginx 경유 20초에 40(버퍼링 없음) · 89종목 15초에 616이벤트(그중 값이 실제로 바뀐 종목은 2 — 미국 프리마켓이라 대부분 안 움직인다) · 마켓 레이더 10초에 8줄 중 6줄 이동하고 등락률이 가격을 따라감(SOXL 111.85/+4.79% → 111.96/+4.89%) · **AI 랭킹 품질 조건**(요청 2): `repository/stock_score_universe.go`(신규)가 「랭킹에 오를 수 있는 종목」을 한 곳에 정의하고 **두 조회 경로가 그것만 쓴다**(「종합·실시간」은 요청 경로 계산, 나머지 47조합은 사전계산 표 — 조건을 각자 적으면 같은 화면에서 탭만 옮겨도 종목 구성 규칙이 달라진다). **ETF/ETN은 `asset_type`으로 못 거른다** — 실측 분포로 미국 상장 ETF 2,418은 `us`/`ETP`, 국내 ETF 1,156·ETN 376은 `domestic`/`ETF|ETN`이라 `security_type`을 함께 봐야 한다(`UPPER(TRIM())` 정규화). 축은 **최신 점수일 기준 3개 이상**(사용자 선택) — 기간 창 전체로 세면 `ADSEW`가 1개월 창에서 4축이라 **걸러 달라고 지목된 바로 그 종목이 안 걸린다**(09-03 breakdown은 `{technical}` 하나뿐). 자격은 **상위 N을 자르기 전에** 적용한다(잘라 낸 뒤에 거르면 100줄 뽑아 놓고 상당수를 버려 목록이 짧아진다). 남는 종목 국내 2,722 · 해외 4,552 · **ETF 0**(ETF에는 애초에 `technical` 하나뿐) · **AI 랭킹 가격이 안 뜨던 원인**(지적 3)은 `StockScoreRankingRepo.DisplayRows`의 **`LEFT JOIN LATERAL`**이었다 — 최근 일주일에 종가가 하나도 없는 종목이 가격 칸을 비운 채 랭킹에 올랐고, 실측 09-04 해외 「수익성·1개월」 2·3·6위 `ADSEW`·`IRAB`·`IMSRW`는 `stock_daily_closes`에 **행이 아예 없다**(Redis 시세도 없어 `overlayLiveChanges`도 기준가가 없어 손을 뗀다). **`TopScores`는 같은 함정을 이미 밟고 INNER로 고쳐 두었는데**(08-31 해외 4위 ALDF) v1.8.0에서 사전계산 랭킹이 생기며 그 결론만 빠졌다 — 위 「조건을 한 파일로」가 정확히 이걸 막으려는 것이다. INNER로 되돌렸고 자리가 비면 `buildRankingItems`가 다음 종목을 당겨 온다. **덤으로 그 엔드포인트가 100 SELECT·194ms → 2 SELECT·80ms**(남은 종목은 전부 종가·시세가 있어 기준가가 캐시에서 풀리고 `resolvePrevClose`의 DB 폴백이 사라진다) · **이 변경과 무관한 발견**: `overseas/valuation/realtime`이 0줄인데 09-03 점수 배치가 해외 9,197종목 **전부**에 `valuation`을 안 썼기 때문이다(09-02는 2,494) — 1일·1주·1개월은 창이 09-02를 포함해 정상 · **FE 구조**: 스트림 가격은 **상태에 저장하지 않고 렌더 중 파생값으로** 얹는다(이펙트에서 setState를 하면 틱마다 렌더가 두 번 돌고 100줄 목록에서 그 두 배가 그대로 보인다) — 마켓 레이더의 플래시는 누적 카운터가 필요해서 스트림 맵이 심볼별 `seq`·`prevPrice`를 들고 있고 파생 함수는 더하기만 한다. 보유·관심은 **한 목록에 두 시장이 섞여** `useQuoteStreamByMarket`이 시장별로 나눠 연결 둘을 연다(하나로 주면 KRX 마감 15:30에 미국 종목까지 끊기거나 그 반대) · 예시(sample) 모드는 스트림에 안 붙는다(픽스처 숫자를 실시간 값이 덮으면 평단과 어긋난다) · 테스트 Go **+16건**(자격 조건 4 · 허브 9 · 마감 스트림 4), vitest 89 → **108건**, `tsc --noEmit` 무오류, ESLint는 새 파일 0오류이고 기존 파일은 **세션 전 baseline과 동일**(use-market-rankings 1 · use-score-rankings 2는 전부 기존 것) · 사전계산 랭킹 재생성 실측 **5.539초**(v1.8.0의 5.6초 대비 무변화) · **남은 확인**: 데스크톱·모바일 UI 눈 확인, 로그인 상태의 보유·관심 실측(이번 세션은 비로그인이라 타입·단위 테스트로만 확인).

- **2026-09-05 (v1.12.0 · 사용자 요청 7건)** — **같은 것은 한 번만 묻고, 한 벌로만 그린다.** ([릴리스 노트](../releases/v1/v1.12/v1.12.0.md)) · **「사용자가 적은데도 가끔 엄청 느리다」의 원인을 실측으로 짚었다** — 마감 중 `terminal-snapshot` 20회 연속 호출에서 대부분 1.3~2ms인데 **서너 번이 200~300ms**였고, 느린 쪽이 전부 **캐시가 막 만료된 요청**이었다. 이 스냅샷의 Redis TTL이 1초라 만료된 요청은 캐시가 빈 채로 상류(Toss)를 **동기로 부르고 기다렸다** — 그 200ms가 왕복 시간이고, 캐시 만료는 혼자 써도 오므로 **사용자 수와 무관한 것**도 이걸로 설명된다. 「사는 시간」(`terminalSnapshotTTL` 60초)과 「신선한 시간」(`MarketDataQuoteCacheSec` 1초)을 나눠 stale-while-revalidate로 바꿨다: 신선하면 그대로 주고, 낡았으면 **그 값을 주되 갱신만 `runOnce`로 뒤로 보낸다**. 화면이 어차피 1초마다 묻고 있으므로 한 박자 전 값이 한 박자 기다리는 것보다 낫고, 캐시가 아예 비었을 때(그 종목 첫 조회)만 예전처럼 기다린다. `model.QuoteTerminalSnapshot`에 `CachedAt`을 넣어 판정하고(없는 예전 캐시는 낡은 것으로 봐서 배포 직후 한 번 뒤에서 갱신), **`loadQuoteDetailFromTerminalCache`는 신선함을 다시 본다** — 상세 헤더 가격 사다리의 한 칸이라 60초 된 값을 「지금 값」으로 올리면 v1.11.0에서 고친 「목록과 상세가 다른 가격」이 되살아난다. 실측 재측정 25회 중 콜드 1회(290ms)를 빼고 **전부 4.6~11.7ms**, 스파이크 0. Go 시험 8건(경계 7 · TTL > 신선도 관계 1) · **토스 한도를 배치가 혼자 쓰고 있었다** — 기동 로그에 `rate-limit low: limit=5 remaining=1 resetSec=1`이 여섯 줄. 한도는 **자격증명당 1초에 5회**인데 랭킹 배치 간격이 `rankingsPollFetchPause = 100ms`(초당 10회, 클라이언트가 primary·secondary를 번갈아 쓰므로 **키마다 정확히 5**)였다. `remaining=1`은 그 1초 창에 사용자 요청이 하나 들어올 자리밖에 없다는 뜻이고, 둘이 들어오면 뒤엣것은 창 리셋까지 기다린다. **200ms**로 늘려 키마다 초당 2.5회 — 창의 절반을 사용자에게 남긴다(기동 첫 적재가 0.8→1.6초로 늘지만 화면은 DB·Redis만 읽으므로 아무도 안 본다). 실측 기동 시 경고 **6 → 1건**(남은 하나는 지수 캔들 경로) · **한 화면이 같은 것을 여러 번 묻고 있었다** — 비로그인 홈에서 `/quotes/batch`가 **다섯 건** 나갔는데 **000660·005930이 다섯 중 넷에 들어 있었다**(심볼 합 32 = 실제 23종목, 이 경로는 종목당 1 SELECT이고 운영 DB는 1 ECPU). `cachedGet`이 이미 있었지만 키가 **URL 전체**라 심볼 목록이 한 글자만 달라도 다른 키다 → `lib/quote-batch`가 40ms 창(마이크로태스크로는 부족하다 — 카드들의 `useEffect`는 같은 커밋에서 돌지 않는다) 안의 요청을 합쳐 한 번 부르고 **종목 단위로** 캐시해서, 목록이 조금씩 다른 카드들이 서로의 결과를 쓴다. 가격 SSE도 같은 이유로 **넷**이 열렸고(겹치는 심볼 집합 넷) 카드의 목록이 데이터 도착에 따라 바뀔 때마다 끊고 다시 붙었다(사용자가 적은 「SSE 연결이 자주 끊겼다가 다시 붙음」) → `lib/list-price-hub`가 **시장(assetType)당 하나**로 모으고 합집합이 바뀌면 60ms 기다렸다 다시 연다. **마감 중에는 아예 열지 않는다**(`closedUntil`) — 예전엔 카드가 뜰 때마다 `reconcile`이 「연결이 없네」를 보고 다시 열고 서버는 그때마다 `session` 하나를 주고 끊었다(홈 20초에 6회). 답이 「월요일 08:00에 열립니다」로 정해져 있는데 여섯 번 물은 셈. 허브는 EventSource를 직접 모르고 `StreamPort`(브라우저 이음매는 `list-price-hub-browser`)를 받으므로 node 환경 vitest가 합집합·재연결·유예를 직접 시험한다 · **`auth/refresh` 폭주** — 비로그인에게 401은 정상인데 `sessionFetch`가 401마다 갱신을 붙여, 탭이 보일 때마다 `auth/me → 401` + `auth/refresh → 401`이 한 벌씩 나갔다(쿠키가 없으니 **답이 바뀔 수 없는** 요청이다). 쿠키가 HttpOnly라 물어볼 수 없으므로 **해 보고 나온 답을 기억한다**: 401·403을 받으면 그 뒤로 안 묻고, 로그인·회원가입 성공이 `markRefreshUsable`로 다시 연다. **5xx·네트워크 실패는 굳히지 않는다** — 서버가 잠깐 흔들렸다고 로그인한 사람이 쫓겨나면 안 된다 · **홈 API 합계 30여 건 → 13건, 중복 0**(`quotes/batch` 5→1 · `prices/stream` 4→2 · `auth/refresh` 401마다→1) · **백그라운드 탭에서도 SSE를 유지한다**(요청 5, **3시간**은 사용자 지정) — 가격 스트림 셋(목록·체결·봉)이 전부 `visibilitychange`에서 닫고 있었는데, 붙는 값(요청·구독 등록·스냅샷 한 벌)이 붙어 있는 값보다 훨씬 무겁다. 판정은 `lib/stream-awake`(순수, node 시험 가능), 시계는 `hooks/use-stream-awake`와 허브. **시계가 아니라 시각으로 판정한다**(덮개를 닫으면 타이머가 늦게 깨고 브라우저는 숨은 탭 타이머를 늦춘다). 다시 보일 때는 **`readyState === CLOSED`인 연결만** 되살린다 — 살아 있는 것을 다시 열면 스냅샷을 한 벌 더 받는 셈이라 끊지 않으려던 일을 스스로 되돌린다. **폴링은 그대로 숨으면 멈춘다**(스트림은 뷰어가 늘어도 DB 작업이 0이지만 폴링은 왕복이 뷰어 수에 정비례한다) · **홈 지수 카드를 눌러도 모달이 안 뜨던 범인은 `setPointerCapture` 한 줄**(지적 3) — 띠를 끌 수 있게 `pointerdown`에서 곧바로 잡았는데, **포인터가 잡혀 있으면 브라우저는 `click`을 누른 요소가 아니라 잡은 요소**(스크롤 뷰포트 div)에 보낸다. 원인이 오래 안 잡힌 이유가 여기 있다 — 스크립트 `cell.click()`은 정상적으로 열리고(캡처는 실제 포인터 입력에만 걸린다) 손으로만 안 되므로 「가끔 안 눌린다」로 보였다. **4px 문턱을 넘긴 뒤에 잡고**(try로 감싼다 — 끝난 포인터 id면 `NotFoundError`인데 캡처는 덤이지 끌기의 조건이 아니다), 덤으로 **손을 대고 있는 동안 띠를 멈춘다**(멈춤 조건 셋 hover·포커스·모달이 전부 PC의 것이라 터치에서는 아무것도 안 멈췄고, 누른 셀과 뗀 셀이 다르면 click은 공통 조상인 컨테이너로 간다). 실측 실제 클릭으로 KOSPI 모달 열림 확인. **키워드 칩은 카드에서 하나만** — 「데스크톱 둘, 모바일 하나」였는데 **데스크톱에서도** 둘째 칩이 지수 이름을 덮었다(셀 224px 고정, 이름만 `flex-1 truncate`라 줄어드는 것은 언제나 이름). `maxCompact` prop은 유일한 사용처가 사라져 함께 제거. 모달은 `max={3}` 그대로 · **홈 뉴스 칩이 「코…」로 뭉개지던 것**(지적 2)은 측정이 **「+N」의 폭을 빼먹어서**였다 — 측정 줄은 칩을 `META_CHIP_CAP=3`까지만 그리는데 「보여줄 개수 = 잰 개수」인 경우 숨김을 0으로 봐서, 실제로 함께 그려지는 「+8」이 계산에 없었다. 모자란 폭만큼 flex가 칩을 눌러 줄인 결과가 「코…」다. 숨김을 **총 개수 기준**(`totalCount - shown`)으로 세고 후보 「+N」들의 폭을 미리 재며, 칩에 `shrink-0`을 줘 눌리지 않게 하고, 하나도 안 들어가면 **0개**를 그린다(`+11 · 11분 전`이 뭉개진 이름보다 많은 것을 말한다 — `NewsArticleChips`가 칩 0개여도 `+N`은 남기도록 early return 조건을 고쳤다). 줄 수는 **폰 5 · 태블릿 8 · PC 14**(단계는 사용자 지정) — **받는 개수는 그대로 열넷**이고 CSS(`hidden md:list-item` / `hidden lg:list-item`)로 감춘다(요청 URL이 폭마다 갈리면 캐시가 갈라지고, 폭을 재서 정하면 하이드레이션이 깨진다). 실측 375/820/1400px에서 5/8/14. **주의: 브라우저 패널이 숨겨져 있으면 ResizeObserver가 안 돌아 리사이즈 후 값이 낡은 채로 보인다** — 스크린샷을 한 번 찍어 페인트를 강제해야 실측이 맞는다 · **차트 우측 현재가 표를 헤더와 같은 값에 묶었다**(지적 4) — lightweight-charts는 그 자리에 **마지막 봉의 종가**를 적는데(`lastValueVisible` 기본값) 헤더는 `detail.currentPrice`라 **두 값이 서로 다른 길에서 온다**. 평소에는 `applyLivePrice`가 형성봉을 현재가로 덧칠해 같지만, 그 덧칠은 **형성봉이 있을 때만** 되고 없는 구간(일봉 장 초반·프리·애프터)에서는 표가 조용히 어제 종가에 머문다. `CandlestickChart`에 `livePrice` prop을 두고 시리즈 기본 표를 끈 뒤 `createPriceLine`으로 직접 그린다(점선이라 기본선과 눈으로 구분된다). **봉 데이터는 안 건드린다** — 일봉의 종가는 정의가 있는 값이라 시간외 가격으로 덮으면 다른 종류의 거짓이다. 세 호출부(상세·워크스페이스·일봉 카드)가 전부 자기 헤더 값을 넘긴다 · **중복 카드 통합**(요청 7): 오늘의 키워드 2벌 → `news/trending-keywords-card`(홈은 접힘·각주·나가기, 레일은 그 자리에서 좁히기 — `variant`/`onSelect`로만 가른다), 다가오는 일정 3벌 → `earnings/personal-schedule-card`(뉴스 레일만 `showQuotes`), 내 종목 뉴스 2벌 → `stocks/symbol-news-card`(두 파일의 diff가 제목 두 줄과 필드 이름 두 개뿐이었다 — 모달은 08-14에 이미 합쳐져 있었다), 주요 실적 일정 2벌 → `earnings/major-earnings-card`(홈은 날짜순+시세, 레일은 시총순+배지·시총), 하이라이트 조회 → `hooks/use-earnings-highlights`. **보유 일정이 종목당 1요청(최대 12)이던 것도 `useUpcomingEvents` 배치 1건으로** — 09-04에 관심·뉴스는 옮겼는데 이 카드만 남아 있었다(실측 배치 1 · 종목별 0). **가져오는 자리는 카드가 아니라 화면/공유 훅**. 미사용 카드 6·모달 2는 **전부 삭제**(사용자 선택, import 0건 확인) · **비로그인 저장 버튼 문구**(요청 1) — 창 안의 저장 버튼이 셋 다 「로그인하고 저장」이라 창을 오가면 어느 것이 무엇인지 사라졌다(제목은 열 때 한 번 읽고 눈은 버튼으로 내려간다) → 「로그인하고 스탬프 남기기 / 포트폴리오 관리하기 / 목표가 관리하기」. **비활성화하지 않는다**(사용자 선택: 셋 다 목표가 방식). **규칙이 폼 자신에게 붙은 것이 핵심** — 예전에는 폼을 여는 버튼이 막았는데 그건 버튼이 아는 규칙이라 다른 자리에서 폼을 띄우면 함께 오지 않고, 실제로 **보유 화면의 「예시로 둘러보기」로 들어가면 활성 버튼이 그대로 보였다**(사용자가 본 화면이 이것이었다). 목표가 모달은 **로그인 검사를 값 검사보다 앞으로** 옮겼다(비로그인에게 「목표가는 0보다 커야 합니다」를 먼저 내밀면 헛걸음이 한 번 더 붙는다) · **손대지 않은 것**: `md:rev:*` TTL 없는 키 3,729개(13.7%)는 `redis_census.go`가 **의도된 것**이라고 적어 두었다(사라지면 낡은 캐시가 새 것으로 보인다, 약 123KB, 감시만) · **테스트**: vitest **122 → 173건**(+51: 메타 칩 맞춤 10 · stream-awake 8 · quote-batch 13 · list-price-hub 13 · session-fetch 7), Go **+8건**, `tsc --noEmit` 무오류, ESLint 신규·수정 파일 **0오류**(기존 2건 `stock-detail-client`의 `StatPrice`는 세션 전 baseline과 동일) · **도구 메모**: Git Bash의 `sed -i`는 CRLF 파일을 LF로 통째로 바꾸고(autocrlf=true라 git diff에는 안 보인다), `node -e "…"` 안의 백틱은 **bash가 명령 치환으로 먹는다**(한글 주석의 `` `이름` ``이 통째로 사라졌다) — 다중 행 편집은 스크래치패드의 `edit.mjs`(old/new를 파일로 받아 줄바꿈 보존)로 한다 · **남은 확인**: **개장 중 실측**(차트 표와 헤더가 갈라지던 구간 · 백그라운드 3시간 유지 · 장중 SSE 합집합), **로그인 상태**(이번에도 비로그인·예시 모드로만 확인).

- **2026-09-05 (v1.12.1 · 사용자 요청 4건)** — **옆에 설 카드가 없는 날에도 제 높이로 선다.** ([릴리스 노트](../releases/v1/v1.12/v1.12.1.md)) 홈 뉴스 「PC에서 5줄」은 캐시가 아니라 배치 — `lg:absolute` 자식이 부모 높이를 못 만들어 브리핑 없는 날 칸이 바닥값 352px에 갇혔다(실측 5/14줄 → 778px·14줄). 판단을 `lib/dashboard-news-layout` 한 곳으로(vitest 3) · 홈 랭킹 두 카드 7줄 통일(거래대금은 9를 받아 걸러 7, AI 제목의 배치일 제거 → 278=278px) · 모바일 종목 헤더 동작 셋 기본 펼침 · 비로그인도 스탬프·보유·목표가 창을 열되 저장 버튼 비활성 + 「로그인하기 →」 한 줄(비로그인 POST 0건 실측) · npm test 18/176 · tsc 0 · eslint 0\n' >> docs/archive/sessions.md; tail -2 docs/archive/sessions.md | cut -c1-100

- **2026-09-05 (v1.13.0 · 사용자 요청 9건)** — **판단할 것이 없는 요청은 엣지에서 끊고, 곁눈질은 창으로 연다.** ([릴리스 노트](../releases/v1/v1.13/v1.13.0.md)) wp-admin·php·zip 스캔이 Next까지 들어와 요청당 232ms(`application-code: 195ms`)를 태우고 있었다 → `deploy/nginx/block_noise.conf` 한 벌을 두 app_routes 맨 위에서 include(정규식 location은 순서대로, 첫 매치가 이긴다)해 dev·prod가 같은 규칙을 쓴다. 확장자·숨김 파일(`.well-known` 예외)·CMS 경로를 **444**로 끊고 로그도 안 남긴다(양이 본질). 우리 라우트 이름은 전부 제외 — `/admin`은 우리 화면, 막는 것은 `/administrator`. 실측 스캐너 10종 차단 · 앱/API/정적 12종 200. nginx 재생성 필요(마운트 추가) · 모바일 종목 헤더 각주의 `shrink-0`이 「줄이지 마라」라서 상자 밖으로 나가 잘렸다(375px: 남은 자리 18px / 필요 150px, 해외는 원화 환산까지 달아 축약으로는 못 넣는다) → `flex-wrap`, 각주 끝 483 → 351=줄 끝 · 뉴스 카드 메타 줄을 3칸 격자로(언론사 4.75rem 고정) — 키워드 시작 x가 164/183으로 흔들리던 것이 열 장 전부 184 · 종목 뉴스 탭만 `window.open`으로 앱 밖에 나가던 것을 공통 기사 모달로 · 「🚨 속보 N」 필터(0이면 잠금, 건수와 목록은 `lib/news-breaking-filter` 같은 함수) · 종목·지수 칩을 `QuickLookProvider`(AppShell 1개)로 모아 **어디서 눌러도 같은 창** — props로 내리면 한 군데는 반드시 빠진다. 지수 폴링은 골랐을 때만 마운트(30초 폴러) · 종목 퀵룩: 「일별 종가」 20/60/전체 · AI 핵심 요약(llm 행만) · 최근 일별 표(지수와 공용) · 면책 삭제 · 헤더에 관심·메모장·보유·스탬프·목표가. 기간 규칙 공용화하며 「전체」 라벨 교정(31행에 「60일」이라 적던 것) · 실적 0건인 달에 스코어보드 감추고 달력 정중앙 안내(달 이동 줄은 남긴다 — 감추면 갇힌다), 판정·문구는 `lib/earnings-empty` 한 곳 · 실적 모바일 개선안 3안은 **제안만** `docs/mockups/earnings-responsive-v1.html`(실측: 상단 589px이라 목록이 첫 화면 밖, 표는 9열 중 3열) · vitest 176 → 199 · tsc 0 · eslint 80 problems로 작업 전과 동일 · **남은 확인**: 로그인 상태에서 퀵룩 헤더 다섯 동작 실제 저장 경로, 개장 중 일별 종가 토글

- **2026-09-06 (전면 QA 1회차)** — **빨간 테스트 11건은 제품 결함이 아니라 제품 변경이 테스트를 데려가지 않은 것이었다.** 보유 기본 보기 표→카드(09-06)로 `getByRole("row")` 9개, 폰 「목록으로」 제거(09-03)로 2개가 빗나가 스위트가 회귀를 못 알리는 상태였다 → 보기 무관 `holdingItem()` 헬퍼로 모으고, 마켓 레이더를 폰(제스처)·태블릿(버튼)으로 갈라 640~1023px 띠를 처음 덮었다(실측 375·390 없음 / 768 있음 / 1440 없음). P1 4건 수정: ① 기본이 된 보유 카드가 `onClick`만 달린 `<div>`라 **키보드로 못 눌렸다**(`market-radar-row.tsx` 패턴 적용, 표 행은 `role="row"` 유지한 채 키보드만 — 회귀 테스트는 `.click()` 없이 포커스+Enter, 수정 전 실패 확인) ② `auth-provider`가 `fetchMe()`의 **모든** 실패를 로그아웃으로 처리해 **429·5xx 한 번에 로그인 사용자가 화면에서 쫓겨났다**(`session-fetch.ts`는 이미 401·403만 굳히고 있었는데 프로바이더가 무효화) → `lib/auth-error.ts`. e2e 전체 실행에서만 나던 간헐 실패 2건의 정체 ③ **보유분 초과 매도가 이력에 남았다**(10주 보유·50주 매도 → 200, 이력 50주 / 실현손익 10주분) → 쓰기에서 거부, 조회의 깎기는 안전망으로 유지, **날짜 기준 재생**으로 소급 입력까지, **중복 검사 뒤 배치**(앞에 두면 재전송이 초과로 오인 — 실측) · 기존 e2e가 관용을 고정하고 있어 **사용자 결정** 후 갱신 ④ 남의 댓글 수정·삭제가 **502**(권한은 정상, `ErrResearchCommentNotFound` 미매핑 — 이 앱이 이미 두 번 고친 계열) → 404. P2 2건: 없는 심볼 201 저장 → 쓰기 경로 카탈로그 검사 404(사용자 결정) · `role="tab"`의 `aria-pressed`(axe critical) 제거. **e2e 139/11 failed → 159/0(3회 연속) · fe 203 · go 전체 통과 · 가로 스크롤 4뷰포트×16화면 0건 · IDOR 전수 차단 확인.** 남김: 목록 행 버튼 중첩(레이아웃 변경, 제안만) · `--text-maged` 아님 `--text-muted` 3.23:1 AA 미달(전역 토큰, 승인 대기). 산출물 `docs/QA_REPORT.md`·`QA_INVENTORY.md`·`QA_HUMAN_REVIEW.md`

- **2026-09-06 (QA 2차 — 승인 적용 + 미진행 영역)** — **승인 전에 실측을 다시 하니 내 첫 보고가 두 군데 틀렸다.** `--text-muted`는 「전반적 미달」이 아니라 **라이트만** 미달이었고(다크는 이미 4.54:1), 라이트는 오히려 더 나빴다 — 흰 카드 3.23 / **페이지 배경 위 2.97**(뉴스의 시각·건수는 대부분 그 배경 위에 앉는다). 그래서 제안값도 `#767676`이 아니라 `#707070`. 버튼 중첩은 반대로 **더 가벼웠다**: 크로미움에서 탭 순서는 이미 정상이고(행→관심→메모) 안쪽 버튼도 접근성 트리에 정상 노출 — 남는 건 규격 위반과 다른 리더에서의 위험. 승인용 미리보기(실제 캡처 전후 슬라이더 + 변경 지점 지도 + Tab 데모)를 만들어 확인받고 적용했다. ① 라이트 토큰 한 줄 교체, 다크 불변, 레이아웃 이동 0(영향 요소 뉴스 135·차트 1,342·상세 50·보유 8, 전부 각주·라벨) ② **버튼 중첩 5곳 해소** — 주 동작만 실제 `<button>`, 아이콘·칩은 형제. 뉴스 카드만 **덮개 버튼 + 칩 `z-10`**(칩이 링크라 클릭 영역을 제목으로 줄이면 손해) → **`nested-interactive` 전 페이지 0건** ③ 없는 종목 요약 500 → 404(핸들러가 모든 실패를 500으로 덮고 있었다; 화면엔 안 보이지만 오타가 5xx 알람을 울렸다 — 조사 중 「BUG-05 재발」로 오판했는데 그 패널은 `main` 밖에 그려진다) ④ `/reports` `dl`의 `div > p` 구조 위반 → 보조 설명을 `dd` 안으로 ⑤ **스탬프 예측 잠금**을 순수 함수로 떼어 경계 6케이스(5분 안 기다리고) ⑥ **커버리지 구멍 G-1~G-4 해소** — `market-data.spec.ts` 18건, 값이 아니라 구조 불변식 ⑦ **미진행 영역 실측**: 성능(화면당 2~18요청·중복 1종·**마감 중 30초 0건**) · 데이터(목록가=상세가 **0.00bps**·점수 3경로 일치) · 운영(스캐너 10종 차단·우리 경로 200) ⑧ `admin-gating` 3건 실패는 **dev 서버 라우트 매니페스트 유실**이었다 — `touch`만으로(내용 동일) 복구, 재시작 안 함. **e2e 139/11 failed → 176/0(3회 연속) · fe 203 · go 통과 · eslint 79(전 80).** 남은 결정: 브랜드 파랑·등락 빨강 대비 18묶음(대부분 4.0~4.48, 문턱 바로 아래) — 정체성·상승하락 규약이라 임의 변경 금지

- **2026-09-06 (사용자 요청 9건 — 1~3 적용 · 4~9 시안)** — **모달 안에서 화면을 떠나면 그 화면이 한 프레임 만에 튕겨 나갔다.** 비로그인으로 종목 퀵룩의 관심을 누르면 로그인 화면이 열렸다가 곧바로 되돌아왔다 — `useModalHistory`가 닫힐 때 자기 히스토리 항목을 `history.back()`으로 걷는데, **떠나서 닫힌 경우** 그 위에 올라온 것은 자기 항목이 아니라 방금 연 화면이다(실측 추적 `push:(same)` → `push:/login?next=/news/domestic` → `back` → `@/login` → `@/news/domestic`). 되돌리기 직전 `history.state`에 표식(`quantPortModal`)이 있을 때만 걷는다 — Next 라우터가 `pushState`를 감싸도 우리 표식은 남고(`{quantPortModal:true, __NA:true, __PRIVATE_NEXTJS_INTERNALS_TREE:…}`) 라우터가 만든 항목에는 없어서 판정이 성립한다. 판정만 `lib/modal-history`로 떼어 단위 6케이스, **모달 안의 모든 이탈**이 함께 고쳐진다 · 메모 폼에 401을 막는 자리가 **비어 있었다**(다 적고 저장해야 「저장에 실패했습니다」) → 스탬프 창과 같은 형태로 「로그인하고 메모 추가」·`disabled`·`LoginRequiredNote`, 취소는 함께 감춤(저장할 수 없는 폼에 지울 것이 없다) · 뉴스 카드 메타 줄은 사용자가 **B안** 선택 — 375px에서 글 칸 259px 중 언론사가 76px 고정(29%)이고 키워드 칸 139px(54%)이 **키워드 없는 기사에선 통째로 빈칸**이었다 → 고정폭을 걷고 상한만 유지(66~68px), 09-05의 배지 기둥 정렬은 여기서 포기(사용자 판단: 시작점보다 줄 길이). 키워드 칩 상한도 좁은 화면만 9rem → 7rem · **시안 갤러리 `/dev/samples`** — 여섯 화면 × 「현재·A안·B안」 × 375/768/1280을 **진짜 `<iframe>`**으로(폭을 줄이면 미디어 쿼리가 바뀌어 그 해상도의 배치가 아니게 된다 — 맞추는 것은 `scale()`), 「현재」는 실서비스 경로 `?sample=1`. 375×812 실측한 **공통 원인은 하나** — `PageColumns`의 레일이 `xl` 미만에서 본문 아래로 쏟아진다(관심 1,587px·스탬프 1,616px·목표가 1,131px·리포트 1,032px·메모장 812px)이고, **그 레일 첫 칸이 화면의 주 동작**이다(종목 추가·메모 검색·목표가 추가 폼 331px). 실적만 결이 다르다: 스코어보드 228 + 달력 345 = **573px(70%)** 뒤에야 첫 회사, 목록은 10칸 표라 폭 **1,060px을 325px 창**에서 민다. **fe 209(+6) · tsc 0 · eslint 79(작업 전과 동일) · e2e ui 47/0 · api 129/0.** 사고: `git checkout -- docs/archive/sessions.md`로 **미커밋 세션 이력 65건을 날렸다**(HEAD는 08-21 커밋) — `~/.claude/projects` 트랜스크립트에서 append 호출을 시각순으로 긁어 전량 복구했고, 되돌리기 명령은 이 저장소에서 쓰지 않는다. · **시안 2차(같은 날 재작업)** — 1차 목업은 사용자 평가 「현재보다 훨씬 저 퀄리티」로 폐기했다. 실제 행 하나가 들고 있던 것(컨테이너 쿼리 칸 아홉 `WL_COL`·52주 그라데이션 막대·점수 배지 톤·값 플래시)을 목업이 하나도 안 갖고 있었으니 비교 대상이 아니라 열화판이었다. 방향을 뒤집어 **실서비스 컴포넌트를 그대로** 띄우고 껍데기 한 겹만 바꾼다(`lib/layout-lab` 컨텍스트 + `components/layout/page-rail-lab`) — 기본값이 「지금 그대로」라 실서비스 불변이고 **1280에서 세 시안의 격자·폭·높이가 완전히 동일**(실측 `934px 300px` / aside 300×712). 실적은 사용자 결정대로 스코어보드·상세 표를 유지하고 달력만 `EarningsDayStrip`으로 — 월 격자가 말하던 것(건수·상회 비율·예정/추정 점·오늘 링·배경 농도·범례)을 하나도 빼지 않고 **345 → 156px**, 첫 회사 행 y **723 → 534**(812 안으로 들어왔다). 나머지 다섯은 원인이 하나(레일이 `xl` 미만에서 본문 아래로 쏟아지고 **그 첫 칸이 주 동작**)라 `fold`(본문 아래 38px 접힘) / `sheet`(본문 **위** 38px 줄 → 아래에서 올라오는 92dvh 시트)로 갈라 견준다: 관심 2,139→590 · 스탬프 2,875→1,297 · 메모장 1,279→505 · 목표가 1,810→717 · 리포트 3,022→2,029, **카드·필드 삭제 0**. 시안 라우트가 앱 껍데기를 빼면서 `HeaderSearchProvider`·`QuickLookProvider`·`ShellModeProvider` 셋은 남겨야 했다. **e2e ui 47/0 · api 129/0 재확인.**

- **2026-09-06 (퀵룩 「종목 분석 보기」 무반응 + prod 재배포)** — **눌러도 아무 일도 없었다.** 09-06 오전에 넣은 표식 판정(`quantPortModal`)은 **떠나는 쪽이 히스토리를 먼저 만졌을 때만** 성립하는데, Next `<Link>`의 이동은 트랜지션 안에서 갈 화면의 페이로드를 기다린다 — 그동안 모달은 이미 언마운트돼 되돌리기가 예약되고, 둘의 순서가 **갈 화면이 얼마나 빨리 준비되는가**로 갈렸다. 실측: `/watchlist`(분석 화면 준비됨) `replace:/watchlist(__NA) → push:/analysis?symbol=005930` 성공 / `/holdings`(준비 안 됨) `back → replace:/holdings(__NA) → pop@/holdings` — **이동이 통째로 사라졌다.** 처음 누를 때는 늘 느린 쪽이라 사실상 항상 고장이었고, 가끔 되는 것처럼 보인 것이 이 경합 때문이다. 표식을 **언제 읽어도** 경합은 남으므로 판정을 바꾸지 않고 **떠나는 쪽이 미리 말하게** 했다 — `lib/modal-history`의 `leaveModal(close)`가 판정 한 번을 건너뛴다. 적용은 「닫기와 이동이 한 클릭 안에 있는」 세 자리뿐(종목 퀵룩 바닥 「종목 분석 보기」, 지수 퀵룩 바닥 「관련 뉴스 더보기」·「실시간 차트」) — 모달 안의 다른 링크들(메모장·스탬프·리포트 레일)은 `onClose`를 부르지 않아 라우터가 먼저 항목을 만들고, 기존 판정이 그대로 맞는다. 남는 히스토리 한 칸은 주소가 같아 뒤로가기 한 번이면 원래 화면(실측 확인). e2e 회귀 `quick-look-leave.spec.ts` — **수정 전 실패 확인 후** 통과. **fe 213(+4 단위) · tsc 0 · eslint 79(작업 전과 동일) · e2e ui 48/0 · api 129/0(12 skipped).** 배포: `fe/scripts/deploy.ps1`(스크립트 수정 없음) → `quant-port-fe` healthy · nginx `app_routes.conf`, 배포본 번들에서 `onClick:()=>(0,C.leaveModal)(L)` 확인, prod 대상 e2e ui 48/0 + 375px·데스크톱 실측 통과. 도중 호스트 dev 서버(:3000)가 스스로 죽어 502 — `preview_start fe-dev`로 되살렸고(배포가 다시 정리) 원인은 확인하지 못했다.

- **2026-09-07~08 (관심종목 전면 개편 — 시안 4종)** — 사용자 지적(사용성·가시성 · 전일/전주/전월의 낮은 중요도)에서 출발해 화면을 다시 짰다. 시안은 목업이 아니라 **실제로 도는 화면** 넷이고(`/dev/samples`의 `watchlist-v2`), 몸통 하나를 공유해 차이가 배치에만 있다. 판정을 `lib/watchlist-signals.ts`로 떼어 신호 칩을 만들고(단위 9케이스), 목표가·메모·스탬프를 목록에 처음으로 끌어왔다(요청은 목록당 3건). 375px 총높이 현재 2,273 → A 1,860 · C 1,838. fe 222 · tsc 0 · 실서비스 파일 무변경. 피드백 대기.

- **2026-09-07 (메모장 전면 재설계 — 시안 3종 제시 · 피드백 대기)** — 사용자 지적: 「사용성·가시성이 떨어지고, 실제 메모장처럼 직관적이지도 않고, 크게 쓸 메리트를 모르겠다」. 1280·375 실측으로 원인을 셋으로 좁혔다. ① **가장 넓은 칸이 가장 오래 비어 있다** — 2단(목록 290px · 본문 나머지)인데 들어오는 순간 그 본문이 「왼쪽에서 메모를 고르면 여기에 펼쳐집니다」다. 첫인상이 빈 상자다. ② **쓰는 일이 가장 찾기 어렵다** — 「새 메모 작성」이 목록 카드 *안쪽*의 점선 버튼이고, 375px에서는 예시 모드에서 아예 안 그려지며 로그인 상태에서도 목록 헤더 아래에 묻힌다. ③ **우리 데이터가 메모에 닿지 않는다** — 저장 순간 박제되는 `journal_notes.price_at_write`가 이 앱의 메모를 다른 메모장과 다르게 만드는 유일한 지점인데, 화면에서는 행 끝의 작은 % 하나와 우측 레일 맨 아래 두 줄이 전부다. 종목을 붙여도 시세·차트·점수·퀵룩 어디로도 이어지지 않는다. 09-06 교훈(「목업은 열화판이 된다」)을 따라 **새 HTML 목업을 만들지 않고** 기존 시안 갤러리 `/dev/samples`에 「2. 메모장 개편」을 얹었다 — 375/768/1280 진짜 `<iframe>` 3종, 「현재」는 지금 화면 그대로가 비교 기준. 셋 다 ①②는 같은 처방(본문 한 컬럼 · 맨 위 작성 바 · 상세는 모달)이고 갈리는 것은 ③을 무엇으로 갚느냐다 — **A 기록장**(시간: 월별 연대기 + 고정 메모 별도 묶음 + 「답이 오고 있는 메모」 가로 띠 = 쓴 뒤 ±10% 넘게 움직인 것) / **B 종목 노트북**(종목: 메모를 종목 아래로 묶고 카드 머리에 현재가·등락·`MarketRadarSparkline`·그 종목 글의 평균 성과, 종목 없는 메모는 맨 아래 한 묶음) / **C 복기 보드**(성과: 성적표 4타일 + 크게 맞은 순서 다이버징 막대 + 태그별 성적 + 30일 넘게 안 열어본 메모). 그리는 조각은 전부 실서비스 것이다 — 상세는 `JournalNoteModal`(작성 시점 기준선 얹은 차트·목표가 줄 포함), 작성은 `JournalNoteForm`, 태그 칩은 `JournalTagChips`, 아바타·퀵룩·스파크라인도 동일. 코드는 `fe/src/components/journal/lab/` 안에만 있고 실서비스 `/journal`은 불변. 행만 시안 픽스처 16건(`lab/fixture.ts`)인 이유: 서버 예시는 5건이라 월별 묶음·종목 묶음·성과 분포가 아무 말을 하지 않고, 서버 픽스처를 늘리면 `sample.Journal`이 캐시 없이 요청마다 시세와 그날 종가를 다시 재서 예시 모드의 외부 호출이 종목 수만큼 늘어난다. 작성 당시 가격은 **값이 아니라 성과율을 박아 두고 현재가에서 역산**한다 — 값을 박으면 며칠 뒤 실제 주가가 움직였을 때 그 값이 최근 40봉 밖으로 밀려 모달 차트의 「작성 시점」 기준선이 조용히 사라진다(차트가 범위 밖 마커를 그리지 않는다). 실측 중 고친 것 둘: 고정 메모를 지금처럼 목록 맨 위로 올리면 **8월에 쓴 메모가 「2026년 9월」 머리글 아래 앉아** 연대기가 거짓말이 되어 고정을 별도 묶음으로 뺐고, 성과 막대의 두 가격 칸을 폭 고정으로 바꿨다(「1,787,000원」과 「$172.27」이 한 목록에 있으면 행마다 0점이 20px 넘게 어긋나 길이 비교가 안 된다). **tsc 0 · eslint 0(신규 파일).** 확정되면 고른 안을 `journal-client.tsx`로 옮기고 `lab/`·레지스트리 항목·view 라우트 분기를 함께 걷는다.

- **2026-09-08 (「목표가 예측」 새 메뉴 — 목표가 관리 + 스탬프 통합 구현)** — BE 무변경. 두 테이블을 FE에서 하나의 `Prediction`으로 접고(`lib/prediction-types.ts`), 기한 유무가 저장 경로를 가르게 했다(기한 있음 → 스탬프 / 무기한 → 목표가). 「도달」(가격 사건)과 「적중」(기한 채점)을 색부터 갈랐고 적중률·도달률을 분모부터 나눴다. 사이드바 두 항목을 한 자리로 합침. 검증: tsc 0 · eslint 79(동일) · vitest 242(+20) · e2e ui 60/0 · api 129/0 · next build 성공 · 375~1440px 실측. 미배포.

- **2026-09-08 (SEO prod 배포 + 네이버 소유확인 태그)** — `layout.tsx`에 `naver-site-verification` 메타 추가. Cloudflare 301(apex→www·http→https) 사용자 선행 완료(curl 4조합 통과). `fe/scripts/deploy.ps1` prod 배포(fe healthy). 실서비스: naver 태그·robots·sitemap·OG·아이콘 확인. **사용자 — 네이버 소유확인 클릭 → 구글 DNS TXT → sitemap 제출.** 미커밋.

- **2026-09-08 (SEO 2차 — 공개 콘텐츠 서버 렌더)** — 등록을 마친 뒤 실측하니 **색인 대상 8페이지의 본문이 전부 사이드바**였다(`/news/domestic` 1,827자 중 내용은 「불러오는 중…」뿐). 원인 둘 — 화면을 전부 클라이언트에서 그린다는 것, 그리고 **1차에서 넣은 `Disallow: /api/`**. 구글은 렌더링 중 fetch에도 robots.txt를 적용해서, 내용을 `/api/v1/...`에서 받는 이 앱에선 **구글 렌더러조차 데이터를 못 받는다**. 공개 조회 9개 접두어만 `Allow`로 되살렸다. 네이버 Yeti는 JS를 사실상 안 돌려 그것만으로도 부족 → 서버 렌더. `research-api.server.ts`·`news-api.server.ts`는 **쿠키를 일부러 안 보내고**(개인화 응답이 공용 캐시에 앉는 것을 막는다 · 필요한 건 크롤러와 같은 비로그인 시야) `revalidate`로 접으며(60~600초) 실패를 삼킨다. 클라이언트는 `background` 모드로 덮어 서버가 찍은 글이 스피너로 지워지지 않게 했다. 뉴스는 정렬·검색 파라미터가 없을 때만 선반입(`lib/news-ssr.ts`). 결과: `/news/domestic` **1,827 → 20,823자**, `/reports` 상세 링크 **0 → 3개**, `/reports/29` 사이드바뿐 → 2,550자. 그 밖에 리포트별 `generateMetadata`(비공개·철회·숨김은 noindex) · 사이트맵에 리포트 상세 · `Article`/`BreadcrumbList` JSON-LD · 약관·개인정보(`/legal/*`, YMYL 신뢰 신호). **실적 SSR은 근거를 대고 뺐다** — 기본 구간이 `오늘..오늘`인데 오늘 국내 실적 0건이고 1년의 대부분이 비어 크롤러에게 빈 표가 나간다(진짜 수단은 종목별·월별 URL). tsc 0 · eslint 77(79→77) · vitest 242 · build 성공 · SSR 실측 · 하이드레이션 오류 0. 미배포·미커밋. `/legal/privacy`의 사업자 정보는 사람이 채워야 한다.

- **2026-09-08 (「동종 상위 N%」 표기 뒤집힘 수정)** — 오라클 LLM 글이 「상위 94%」와 「상위 6%」를 오갔다. 원인은 한 페이로드에 섞인 반대 방향 두 규약 — `peer_pct`는 상위 N%(낮을수록 우수), `inputs.percentile.*`는 백분위(높을수록 우수)인데 `stock_summary_facts.go`가 후자를 `roePeerPct`로 그대로 넘기고 프롬프트가 둘을 묶어 「higher is better」라고 못 박고 있었다(ORCL: peerPct=1 · percentile.roe=94.36). 페이로드를 상위 N% 한 방향으로 통일(`*TopPercent`)하고 프롬프트를 교체했으며, 「동종 상위 0%」(미국 상위 100 중 20종목)를 `model.ReadPeerPct` 클램프로 배치·조회 3곳에서 함께 막았다. 회귀 테스트 2건. **재시작 보류** — 다른 세션이 `internal/service`를 동시 편집 중이라 트리 전체 배포를 피했다(사용자 결정). 미커밋.

- **2026-09-08 (AI 요약 대상 3층 확장 · 상한 350 · 쿼터 하루 경계 태평양)** — 대상이 실시간 차트 순위 하나뿐이라 매일 같은 종목이 다시 구워지던 것을 세 층으로 늘렸다(① 레이더 ② AI 추천 랭킹 회전 ③ 사용자 보유·관심 미분석, 3층은 2층 앞에 30자리 보장). 2층은 기간 단계가 아니라 **순위**로 돈다 — `realtime`과 `1d`가 실측으로 같은 목록이라(차이 0건) 기간 단계는 한 단계가 통째로 비고 1주·1개월 탭이 나흘간 빈 채 남는다. 기간은 가중치(1.0/1.3/1.6)로만 쓰고 「요약 없음 → `analyzed_at` 오래된 순」으로 6~7일에 한 바퀴, 종합축 상위 30만 매일. 상한 250→350·토큰예비 120K→200K(`migrations/066`, 근거는 호출당 실측 2,258/1,939토큰과 브리핑 몫). LLM 일일 카운터를 **태평양 자정** 기준으로(Gemini가 그렇게 센다 — KST로 세면 해외 배치 시각에 과대 예약). 창 계산의 종목당 시간을 6초→12초 실측값으로(안 고치면 350건에서 매일 꼬리가 이월). 단위 테스트 14건 + 실 DB 드라이런(LLM 0콜) 검증, 국내 807후보→350 · 해외 681→350. Docker 재시작으로 배포(트리에 있던 「동종 상위 N%」 수정도 함께 나감). 미커밋.

- **2026-09-09 (배치 감시 체계 + 누락분 재처리)** — 「해외 배치가 09-04 이후 멈췄다」는 내 보고가 오진이었다(09-07은 Labor Day). 그런데 그걸 구분하려고 달력 JSON을 열어야 했다는 것이 문제였고, 확인 중에 진짜 구멍이 나왔다 — **`llm_summary_*`·`news_briefing_*`가 원장에 한 줄도 안 남아** 관리자 화면이 마지막 실행을 08-20으로 알고 있었다(매일 돌고 있었는데도). 넷을 넣었다: ② 예약 실행을 전부 `batch_job_runs`에(브리핑은 건너뛴 슬롯까지 — 조용한 주말과 죽은 스케줄러는 결과가 같아 보인다), ① 기대 시각을 기계가 계산(`batch_expectation.go`, 달력 붙은 잡은 휴장일에 기대 자체가 없다), ③ 워치독을 새 컨테이너가 아니라 worker-scores의 15분 잡으로(감시자를 또 감시할 것을 만들지 않는다 — API는 `batch_health.computed_at` 나이로 워치독의 죽음을 본다), ④ 관리자 배너·사이드바 뱃지·`/metrics` 게이지 둘. 실측이 내 함정 둘을 잡았다 — 워커가 `HH:MM:56`에 떠서 1초 이르다는 이유로 매일 「늦음」이 뜨던 것(15분 지터)과, AI 요약은 기한이 아니라 **창의 시작**을 기대로 써야 한다는 것(원장은 시작 시각을 남기고 이 배치는 기한에서 거꾸로 빼서 시작한다). 곁다리로 `MarkStaleRunsFailed`가 조건 없이 running 전부를 죽이던 것에 6시간 나이 조건(API 재기동이 다른 컨테이너의 실행을 죽은 것으로 기록하고 있었다). 재처리: 09-08 재기동에 죽은 `financials_domestic` 재실행(limit 200) + Redis에만 있던 AI 요약 실행 28건을 원장으로 이관 → 워치독 「정상 24개」. 테스트 14건 추가·전체 통과·`next build` 성공. BE 배포, **FE 미배포**(배너·뱃지는 프로덕션 빌드라 배포 필요, SEO 미배포분과 함께 나감). 미해결: `financials_domestic`이 200종목 처리라고 보고하는데 실제 진행은 8종목(기존 동작, 별건).

- **2026-09-09 (09-08 다운 원인 분석 · 호스트 워치독 · 시세 묶음 격리)** — 09-08 22:07 호스트가 커널 버그체크 `0x0000000A`로 죽고 60초 만에 재부팅됐는데 스택은 23:44까지 **96분을 더 누워 있었다**(총 1시간 37분). 컨테이너 13개 전부 `restart: unless-stopped`였지만 **그 정책은 도커 엔진이 살아 있을 때만 동작한다** — Docker Desktop이 `AutoStart=False`라 엔진이 안 떴고 재시작시킬 주체가 없었다. 사용자 짐작(「사용자가 많아져 부하로 꺼졌다」)은 실측으로 기각 — 크래시 시점 요청량이 오히려 낮았고(초당 3.75건), compose 상한 합계가 CPU 10/24코어·메모리 7.2/16GB라 스택이 최대로 돌아도 기계의 절반을 못 쓰며, 유일한 CPU 소비자 `translator`(llama.cpp **CPU 전용**, cpus 1.5)는 크래시 순간까지 107줄/분 평행선이었고, WHEA·열·리소스고갈 이벤트가 0건이다. 과부하 종료는 버그체크를 안 남기는데 덤프가 남았다 → 드라이버 폴트(특정은 관리자 권한 필요해 미완). 워치독(`deploy/scripts/watchdog.ps1`·`install-watchdog.ps1`) 도입, 로그온+1분·5분 주기 등록. `compose up` 대신 라벨로 멈춘 컨테이너만 `docker start`(fe·nginx가 `prod` 프로파일이라 빠지고, up은 `.env`를 재평가해 재생성한다 — 워치독이 배포를 하면 안 된다). **엔진 복구 실측이 한계를 드러냈다** — Docker Desktop을 죽이고 시험하니 감지·기동·판정·알림은 정상인데 복구가 실패했다. 모달 오류 대화상자가 사람의 클릭을 기다렸기 때문이고(원인은 고아 AF_UNIX 소켓), 그 정리를 워치독에 넣었다. 남는 한계는 Docker Desktop이 사용자 세션에 붙는 GUI 앱이라는 것 — 무인 운영이면 WSL2 `dockerd`나 OCI로 가야 한다. 별건으로, 크래시 20초 전 로그에서 **해외 종목 하나(QXO)가 국내 39종목 시세를 매 틱 떨어뜨리던** 버그를 찾아 이등분 격리로 고쳤다(테스트 5건). nginx 정지 → 20초 복구 실측, 최종 13/13·헬스 200. 릴리즈 `v1.13.1`. 미완: 웹훅 URL 미설정, BSOD 드라이버 특정.
  - **(같은 날 후속)** 사용자 지적 「5분마다 프로그램이 떠서 작업이 끊긴다」 — 스케줄러가 `powershell.exe`를 직접 부른 탓이다. `-WindowStyle Hidden`은 소용없다: powershell은 콘솔을 **먼저 만들고 나서** 숨기고, Windows 11은 그 콘솔을 **Windows Terminal**로 띄운다. 창 열거 대조 실험으로 확정(구방식 → `WindowsTerminal` 뜸 / `wscript.exe`+vbs → 새 창 0개). `watchdog-hidden.vbs`(ASCII 전용 — wscript는 `.vbs`를 ANSI로 읽는다)를 거치도록 재등록했고, `Run(...,0,True)`로 기다려 종료 코드를 넘겨 `LastTaskResult`·`IgnoreNew`가 그대로 의미를 갖는다.
  - **(같은 날 후속 2 — 놓친 배치 재처리 · 워치독 사각 셋)** 사용자 질문 둘에서 나왔다. ① 「불가피한 다운에도 제대로 복구되나」 → 실패 모드를 훑다 **사각 둘**을 찾았다: `/health`는 nginx→api 한 줄만 지나서 **워커 8개가 멈춰도 200**이고(워치독이 「정상」이라 적는다), 게다가 liveness 뿐이라 **MySQL이 죽어도 200**이다. 헬스 정상일 때도 컨테이너 census를 보게 하고(실측: `worker-filings` 정지 → `/health` 200 유지 → 워치독이 잡아 되살림), `/ready`를 **복구가 아니라 알림 신호**로 붙였다 — OCI MySQL 장애를 Docker Desktop 재기동으로 고칠 수 없으니 시도하면 고칠 수 없는 조건에서 재기동만 반복한다. Redis만 죽으면 `/ready`가 **200에 `degraded`**를 담아 보내(핸들러는 MySQL일 때만 503) 상태 코드만 보면 놓치므로 본문 `status`까지 본다. ② 「다운 중 안 돈 배치 재처리」 → 워치독이 `late`인 잡을 큐에 넣는다(`trigger_kind='catchup'`). **「늦었으면 다시 돌린다」는 틀렸다** — 허용 목록으로 하고 **빼는 것에 이유를 달았다**: `llm_summary_*`는 돈이 들고 창 밖에서 돌면 다음 날 예산을 먹으며(LLM 규칙상 사람이 정한다), `news_briefing_*`는 슬롯이 지나면 다음 슬롯이 덮어 의미가 없고, `daily_close_*`는 자기 catch-up이 이미 있어 두 번 한다. 안전장치 넷: `failed`·`stuck` 제외(「안 돌았다」가 아니다) · 회차당 2개 상한(한꺼번에 밀어 넣으면 외부 한도에 걸려 채우려던 것마저 실패) · `HasPending` · `manual`과 갈라 표시(누른 적 없는 「수동」이 보이면 원인을 잘못 짚는다, FE는 「재처리」). 원장 실측으로 전 경로 확인 — `EnqueueAs`→워커 claim→`run 1412 done 만료 리프레시 토큰 40행 정리`. ③ 사용자 지적 「5분마다 창이 떠서 끊긴다」 → `-WindowStyle Hidden`이 무력한 이유는 powershell이 콘솔을 **먼저 만들고** 숨기는데 Windows 11이 그것을 **Windows Terminal**로 띄우기 때문. 창 열거 대조로 확정(구방식 `WindowsTerminal` 뜸 / `wscript.exe`+vbs 0개). 테스트 6건 추가·전체 통과·tsc 통과·BE/FE 배포·13/13. 릴리즈 `v1.13.2`.
  - **(같은 날 후속 3 — Redis 폴백이 원칙으로만 있었다)** 사용자 질문 「DB·Redis는 restart로 복구 못 하나 / Redis 다운 시 캐시 없이 DB에서 가져오는 방어로직」에서 나왔다. **그 방어로직은 이미 설계 원칙(AGENT_STATE 개발 원칙 5)으로 있었는데, 실측해 보니 지켜지지 않았다** — Redis를 내리자 목록·지수·환율·랭킹은 200이었지만 **종목 요약이 502를 10.00초, 터미널 스냅샷이 404를 10.00초** 걸려 돌려줬다. 폴백 코드는 멀쩡했고 **거기 닿기 전에 `REQUEST_TIMEOUT_SECONDS`(10초)가 먼저 끝난 것**이다. 원인 둘: go-redis 기본 `DialTimeout 5초`·`MaxRetries 3`(정상 다이얼은 1ms 미만이라 장애용 값), 그리고 **ISP 와일드카드 DNS** — 컨테이너가 없으면 `redis`가 NXDOMAIN이어야 하는데 도커 리졸버가 상위로 넘겨 ISP가 `218.38.137.27`(quant-port.com을 가로채던 그 IP)을 돌려주고 「응답 없는 호스트」가 되어 타임아웃 끝까지 간다. **타임아웃만 줄여선 안 됐다** — 500ms로 해도 그대로 10초였다(한 요청이 Redis를 여러 번 부르므로 곱해진다). 고칠 것은 「이미 아는 실패를 몇 번 다시 확인하는가」였고, go-redis 훅으로 회로 차단기를 붙였다(**26곳 호출부 무수정**). **판정을 셋으로 갈라야 했다** — 둘로 갈랐더니 회로가 5초마다 진동했다. ISP 가로채기 서버가 **6379 TCP를 받아 주기** 때문에 다이얼은 성공하고 프로토콜에서 실패하는데 그 오류가 `net.Error`가 아니라 「실패 아님=성공」으로 접혔던 것. `nil`·`redis.Nil`=살아있음 / 연결실패=죽음 / **프로토콜·명령 오류=중립**, 그리고 **다이얼 성공은 회로를 닫지 않는다**(닫는 근거는 「Redis가 명령에 답했다」뿐). 결과 8회 연속 전부 200, 요약 0.19초. 테스트 10건. **재시작 복구 가능 여부**: Redis는 컨테이너라 워치독 census가 이미 되살리고, **MySQL은 로컬 Windows 서비스인데 실패 복구 동작이 비어 있다**(`sc qfailure` FAILURE_ACTIONS 없음) — 크래시하면 Windows가 아무것도 안 한다. `sc failure` 설정은 관리자 권한이라 사용자 몫. 웹훅은 사용자 요청으로 이번엔 보류. 릴리즈 `v1.13.3`.
- **2026-09-11 (사용자 요청 9건 — Toss 목록 429 · DART 하루치 소진 · Azure 번역 · UI 5건)** — ① **목록 화면 진입 한 번이 Toss 429를 90건** 만들고 있었다(실측: `/quotes/batch` 1요청 → orderbook 50 · trades 40, 1.76초, 그때 폴러 터미널 집합 **0**). 임시 스택 추적으로 경로 확정 — `BatchQuoteSummariesResult → … → loadQuoteDetailPersisted → finalizeQuoteDetail → ensurePriceMicrostructure`. `listMode` 가드가 막던 것은 `refreshTerminalSnapshot` **한 분기**뿐이고 캐시·DB 히트라는 흔한 경우가 다른 문으로 들어와 종목마다 2호출을 냈다. 같은 사고를 `/stocks`에서 이미 고쳤는데 **그때는 호출 지점 하나만** 고쳤던 것이라, 이번엔 가드를 `ensurePriceMicrostructure` 안에 뒀다(지점 셋, 앞으로 늘어남 — 빠진 하나는 화면이 아니라 상류 한도에서만 드러난다). **90건 → 0건, 1.76초 → 106ms.** ② **DART 18,000을 「원장에 없는 잡」이 다 썼다** — api reference data poller의 `RefreshAllStored`가 `stock_fundamentals` 전 종목(12,037)을 7시간 돌았고 국내 2,800 × 실측 6.4호출 ≈ 하루치 전부. 09-10 19:29에 한도가 비어 그날 DART 잡 셋이 한 종목도 못 불렀다. 원장 기록·예산 예약·고정 시각이 **동시에 없었고** 주기가 api 기동 시각을 따라갔다. `SyncStateRepo` 등급제로 편입(`kind='fundamentals'`, 회차 국내 400·해외 600, **DART 예약 9,000** — 근거는 정상일 실측 7,895, 배선 없으면 no-op). ③ **warnings 호출의 19%가 매일 같은 404**였다(17,918 중 3,388이 `stock-not-found`인데 `error`로 기록돼 30일 쿨다운이 안 걸렸다). `toss.IsStockNotFound`로 갈랐고 `warningsPause` 250→340ms(한도 5/s의 80%에 붙어 있었고 429는 프로세스별 failover로 api까지 번진다). ④ **번역을 Azure로 전환** — 자체 모델은 `legacy-translator` 프로파일로 내렸고, `apibudget`에 월 창(`TokensPerMonth`)을 새로 넣어 F0 2,000,000자를 강제한다(문자는 보내기 전에 센다). 첫 보고의 월 3.3M자는 **과대였다** — 한국어 소스 기사가 섞였고 영어 원문만 세면 **월 2.17M자로 8% 초과**. 키는 아직 미설정(번역 0건). ⑤ UI 5건 — 외부 진입 기본 탭 `ai`→`overview`(11곳) · AI 카드를 시장 분석 위로 · 점수 집계 시각 각주(최신 행이 비었을 때만 강조) · **모바일 칩 2줄 레이아웃 붕괴**(375px 83행 중 15행 두 줄·14행 잘림 → `measureMetaChipLayout` 재사용한 `NewsChipRow`, 실측 152행 전부 0) · **속보 pill이 눌려 보이지 않던 것**(상태는 옳고 `hover:bg-accent`가 브랜드 칠을 덮었다 — 누른 직후가 바로 그 상태) · 국내/해외 기본값을 열린 시장으로(`/market/clocks` 신설, 서버가 판정). ⑥ 일별 시세 요약글 4섹션 합본(앞으로 쌓이는 것만 — 과거는 원문이 없어 소급 불가). 테스트 +33건(Go 17 · vitest 8 · 그 외), BE/FE 전체 통과·배포·라이브 검증. ⑦ **(같은 세션 후속)** 해외 구멍 메우기가 회차마다 3,250여 종목을 실패하고 다음 회차에 또 시도했다(하루 두 번 = 6,500여 Toss 호출, 해외 커버리지 75% 고정). 카탈로그 NORMAL이 Toss 취급 집합의 상위집합이라서다. 조건 셋을 **모두** 만족할 때만 30일 제외 — `stock-not-found` 코드 · `last_success_at IS NULL`(한 번도 성공한 적 없음) · 30일 쿨다운. 임시 행 3개로 대상 쿼리를 직접 검증하고 정리했다(not_found 30일 내 제외 · error 유지 · 31일 전 유지). 커버리지 분모에서도 같은 종목을 빼 「90% 미만」 경고가 다시 의미를 갖게 했다. ⑧ **번역 물량 재측정으로 첫 보고를 정정** — 월 3.3M자가 아니라 **2.17M자**(한국어 소스 기사가 섞여 있었다). 한도 초과가 65%가 아니라 8%라 사용자가 결정을 바꿨다: 수집량은 그대로, 월 예산 가드만. 미완: Toss 공용 리미터, terminal-snapshot 1초 설계, React #418, 워커 관측성, Finnhub 로그 중복, earnings skip 13, FRED partial. ⑨ **(같은 세션 후속 2 — #7 전체 · 전수 점검 · 사건 기록 테이블)** **사건 원장 `ops_incidents` 신설**(migration 068) — `batch_job_runs`는 실행된 것만 알고 `batch_health`는 한 행이라, 「아직 깨져 있나 · 언제부터 · 몇 번째 재발인가」에 답하는 표가 없었다(09-10 DART 소진 조사 때 로그가 이미 사라져 있었다). **사건을 줄이 아니라 상태로** 저장한다 — (kind,subject)당 열린 사건 하나를 DB 유니크 인덱스가 보장하고, 다시 보이면 occurrences만 오르며 **로그는 처음 열릴 때만**(그전엔 15분마다 같은 줄, financials_domestic 하나가 18회+). 풀리면 닫고 재발은 새 행. 기록자는 워치독 + **apibudget 소진 훅**(계층 유지를 위해 훅, 5분 스로틀 + 고루틴). `GET /api/v1/admin/ops/incidents`. 수명주기를 실제 DB에 돌려 확인하고 시험 행 정리. **예약 잡이 하루 두 번 돌던 버그** — `data_retention`·`candle_rollup`·`disclosures_daily`(DART 두 배). 원인은 `time.After(time.Until(슬롯))`가 **이르게 깨어난 것을 확인하지 않은** 것이고(벽시계가 뒤로 밀린다 — 매일 −0.4초→−4.0초로 더 일찍 깼다), 느린 잡만 멀쩡했던 건 claim 락이 막았을 뿐이다. `WaitForSlot`+`SlotGate`를 **슬롯 루프 7개 전부**에 적용. **Toss 「price API 위주」 검토** — 방향은 맞지만 **한도가 그룹별**이라는 전제가 어긋난다: `/prices`·`/orderbook`·`/trades`가 `MARKET_DATA` 한 버킷(limit=15)을 공유하므로 목표는 「price만 쓰자」가 아니라 **「버킷마다 꼭 할 일만」**이다. `/candles`는 대체 불가 확인(`PriceResponse`에 OHLC·거래량 없음). **가장 큰 남은 기회는 미사용 `/stocks/all`**(하루 7호출로 Toss 취급 유니버스 확정, 전용 버킷) — 사용자 결정 필요. **비-price 축소**: 환율 15초→1분/10분(5,760→1,440, 실측 분당 변동 0.011%), 순위는 마감 시장 스킵(2,304→약 900). 배포 후 실측 확인(환율 4/분→1/분). **전수 점검 추가 발견 넷**: 해외 뉴스 카테고리 셋이 한 번도 수집되지 않았다(티커 2분 = 최소간격 2분 → 첫 갈래만 성공, 로테이션으로 수정·호출 수 동일) · FRED 계기가 실사용의 1/12를 말했다(`RecordN` 신설) · 관리자 콘솔에서 제공자 둘 누락(`AllProviders()` 추출) · **소스 파일 넷이 HEAD에 망가진 채 커밋돼 있었다**(줄마다 빈 줄, fred.go 543줄 중 250줄 — 비어 있지 않은 줄 동일성을 증명하고 정리). **React #418** 원인 특정 — `formatRelativeTime`의 `Date.now()`가 SSR/hydration에서 갈린다, `suppressHydrationWarning`을 텍스트를 직접 담은 요소 6곳에. 표 13행 중 11행 닫힘. 남은 둘(공용 리미터 + 프로세스별 failover)이 다음 차례. BE/FE 전체 통과·배포·13/13 헬시·429 0건.
- **2026-09-13 (사용자 요청 5건 — 해외 뉴스 수집 정지 · 별칭 표 · AI 카드 통일 · 번역기 철거)** — ① **해외 뉴스가 멈춰 있었고 원인이 둘, 둘 다 「간격」이었다.** ⓐ 주말 틱 게이트가 **벽시계 분**을 봤다(`now.Minute()%14 != 0`이면 스킵). 「14분에 한 번」으로 읽히지만 **매 분 물어볼 때만** 참이다 — 2분 주기 티커는 한 시간 내내 같은 홀짝의 분에만 서므로 격자가 홀수면 {0,14,28,42,56}과 **영원히 안 겹치고**(어느 쪽인지는 프로세스 기동 시각이 정한다), 8분 주기 티커는 {14,42} 둘만 잡아 평균 60분이 된다. 실측(일요일 20시간): `finnhub_news` 호출 **10건**(의도대로면 약 85건), 종목별 뉴스 틱 로그가 03:18·04:43·05:15·06:42·07:14·08:42·09:14·10:42 — **42분과 14분만** 남아 있었다. 예전 구현을 임시 시험에 넣어 확정했다(해외 일반 뉴스 티커, 최악 기동 시각에서 **6시간에 0회**). 고친 것은 판정 축이다 — 벽시계가 아니라 **그 티커가 마지막으로 통과한 뒤 지난 시간**(`weekendTickGate`, 티커마다 하나 — 공유하면 먼저 깬 소스가 남의 몫을 쓴다). 새 시험은 실제 주기·오프셋 그대로 기동 시각을 1분씩 밀며 돌려 **6시간에 최소 22회**를 센다. ⓑ 종목별 뉴스가 **32종목 중 하나만** 들어왔다 — `/company-news`가 시장 전체 뉴스와 같은 버킷(최소 간격 2분)이라 첫 종목 뒤 31개가 전부 걸렸다(틱마다 `min interval 2m0s not elapsed` 31줄, 큐 한 바퀴 8분 → **약 7시간**). 같은 파일 바로 위 머리글이 **같은 병을 이미 적어 뒀는데 그때는 시장 전체 쪽만** 고쳤다. 전용 버킷 `finnhub_news_company`(2초 = 실상한 60/min의 절반) + 호출부 자가 페이싱. **그리고 그 회차를 select 루프 밖으로 냈다** — 내가 만든 대기가 다른 소스를 밀어낼 뻔했다(뉴스 소스 전부가 select 한 줄을 나눠 쓰고 `staggeredTicker`는 큐가 아니라 버린다. 같은 이유로 일반 뉴스 쪽이 「기다리지 않는다」를 택했는데 내가 그 함정에 들어갈 뻔했다). 고루틴 + `usCompanySweeping` 겹침 가드(겹치면 기다리지 않고 거른다). 배포 후 `min interval` 로그 **0건**. ② **별칭을 `stock_aliases` 표로 옮겼다**(migration 070, 씨앗 198행 = brand 76 · symbol 122). **옮긴 이유는 속도가 아니다** — map 조회는 공짜고, 문제는 한 줄 고치려면 배포를 해야 했다는 것. 씨앗 **원문은 Go 소스이고 SQL INSERT는 거기서 생성**했으며 시험이 두 벌의 동일성을 센다(`TestSeedMatchesMigration`). 씨앗 심볼 111개는 실 카탈로그에 전수 대조(존재·NORMAL·토스 거래가능). 매칭은 계속 메모리에서 한다(10분 TTL 스냅샷, DB 실패 시 씨앗으로 버팀) — 기사 제목마다 훑는 자리라 SQL을 넣으면 느려진다. 읽는 자리 셋을 다 이었다: 앱 검색(브랜드는 LIKE 확장, **심볼은 후보에 직접 넣고 이름 일치와 같은 점수** — 그 전 「구글」 검색 결과는 ACE 구글밸류체인 같은 **ETF뿐**이었다), 뉴스 태깅 인덱스, 구글 뉴스 검색어 큐. 일상어와 부딪히는 별칭은 뺐고(「블록」은 「블록 체인」이 띄어쓰기로 오면 조사 규칙이 못 막고, 「타깃」은 「타깃 주가」가 더 흔하다) 그 목록을 시험으로 못박았다. 옮기며 **한 번 빠뜨렸다** — 「마이크론」은 상장명이 「마이크론 테크놀로지」라 이름 색인으로 안 닿는데 기존 시험이 잡았다. ③ **AI 카드를 한 컴포넌트로 합쳤다** — 같은 LLM 글의 요약(개요 탭)과 본문(AI 분석 탭)이 **다른 카드 모양**이라 근거를 보려면 탭을 옮겨야 했고 옮기면 요약이 사라졌다. `StockAISummaryCard` + 오른쪽 위 `MiniToggle`(간단/상세), **기본값 양쪽 다 간단**·상태 저장 안 함. 개요 탭만 `hideWithoutLLM`(규칙 문장이 「AI」 제목 밑에 서면 제목이 거짓말이 되는 09-03 결정 유지). 데스크톱·모바일 375px 실측. ④ **번역기 컨테이너를 내렸는데, 내리기 전에 번역이 0건이었다** — 키가 들어와 있는데도 이틀째 `engines=local`이었고 **조용한 실패가 둘 겹쳤다**: 키 이름이 포털 표기(`AZURE_TRANSLATOR_API_KEY_FIRST`)였고, 설령 맞았어도 `docker-compose.yml`의 `environment:`에 있던 `"${AZURE_TRANSLATOR_KEY:-}"`가 **`env_file`을 빈 문자열로 덮었다**(compose에서 environment가 이긴다). 키 없는 엔진은 조용히 사슬에서 빠지므로 오류가 안 난다. 셋 다 고쳤다 — 코드가 포털 이름도 읽고, compose에서 그 세 줄을 빼 다른 비밀값과 같은 길로 들어오고, **지역을 실측으로 확정**(지역 없이 401001 → `koreacentral` 200). 배포 후 `engines=azure` + 실제 한국어 제목 + 문자 계수까지 확인하고 나서 컨테이너를 **제거**했다(`stop`만 하면 워치독이 5분 안에 되살린다). 13 → 12 컨테이너, 모델 볼륨은 되돌리는 길이라 남겼다. ⑤ **검증하다 오태깅 하나를 더 잡았다(요청 밖)** — 「OpenAI delaying IPO **amid** AI safety concerns」에 AMID(ARGENT MID CAP ETF)가 붙어 있었다. 09-03 v6 재태깅이 **바로 이 종목을 잡았다고 적어 둔 자리**인데(「MOVE 30 · AMID 20 · ALTO 18 …」) 그때 고친 건 티커를 대문자 인덱스로 옮긴 것이고, 토스가 `stocks.name`에도 「AMID」를 주므로 **소문자 이름 인덱스라는 두 번째 문**이 열려 있었다(09-12~13 이틀에 다시 20건). 이름이 티커와 같고 그 티커가 낱말이면 이름 인덱스에 안 넣는다(명시 표기 `$AMID`로는 그대로 붙는다). 재태깅 v9로 소급했고 **완주를 확인했다** — 국내 `scanned=93,968 changed=172`(9분 39초) · 해외 `scanned=46,671 changed=3,789`(4분 34초), 실패 0, **AMID 28건 → 0건**. 해외 변경이 3,789건으로 큰 이유는 같은 스윕이 **새 별칭 122개도 소급**했기 때문이다(표본: 「Amazon.com vs. Walt Disney」→ AMZN·DIS). 다만 스윕은 **완주해야 마커가 남는다** — 세션 초반 재배포 다섯 번 동안 매번 잘렸으므로, 규칙을 바꾸고 재태깅을 기대한다면 한 번은 15분쯤 재배포 없이 두어야 한다. ⑥ **내가 낸 배선 구멍을 배포 후 로그에서 잡았다** — 별칭 캐시를 DB에 붙이는 줄을 `EnsureSchema` 본체 끝에 뒀는데 그 함수는 필요한 표가 다 있으면 **중간에서 돌아간다**(「mysql: schema ok」). 운영 DB에서는 한 번도 안 불렸고 **아무 오류도 안 났다**(캐시가 씨앗으로 조용히 돌아간다 — 이번 세션 Azure 키와 같은 모양의 침묵). 마이그레이션은 그 지점 앞이라 표·198행은 정상이었고 달라지는 건 「표를 고쳐도 반영 안 됨」 하나뿐이라 더 안 보였다. `EnsureSchema`를 출구 하나짜리 껍데기로 만들고 「끝나고 하는 일」을 거기 뒀다. 배포 후 `stock aliases: 76 brand · 122 symbol` 확인. BE 25패키지·FE 250시험 전부 통과, tsc 0, BE/FE 배포·라이브 검증(배포 10분 만에 해외 finnhub 기사 89건 — 그 전 하루 종일이 88건이었다).
- **09-14** 사용자 요청 4건 — Azure 번역 자리표시자 실패 루프 차단(숫자→글자, 재시도 상한 migration 071, 묶음 요청, 월 한도를 남은 날로 나눈 하루치) · 별칭 관리 화면 `/admin/aliases` · AI 카드 「AI 핵심 요약」 정리(생성시각·분석탭 링크 제거, 상세는 산문, 종목 모달 간단/상세) · 리포트 읽기 흐름 개편(퀵룩 모달 제거, 본문 우선, 댓글 승격). 릴리스 v1.14.0. 배포됨 · 미커밋
- **2026-09-15 (GitHub 퍼블릭 레포 최초 공개)** — 로컬 전용이던 저장소를 `jjmin333/quant-port` 퍼블릭 레포로 처음 공개했다. 푸시 전에 `gitleaks`(go install로 설치)로 전체 git 히스토리 + staged diff를 스캔 — `be/.env`·`be/.cookies_batchtest.txt`·`deploy/certs/privkey.pem` 등 실제 키가 담긴 파일은 전부 이미 `.gitignore`로 제외돼 있었고(확인: `git ls-files`에 없음), 히스토리에도 유출 없음(찾은 건 toss-openapi.json의 문서 예시 토큰과 테스트 픽스처뿐, 둘 다 가짜). `.gitignore`에 `deploy/logs/`를 통째로 추가(기존엔 `watchdog.log*`만 걸림, `fe-dev.log`가 새 커밋에 딸려 들어갈 뻔함). 스크래치 디버그 파일 2개(`be/-l.txt`, `be/_tmp_median.go`)와 커밋에 잘못 딸려 들어간 이름 없는 파일(`--`, SQL 시드 조각)은 제외·제거. v0.9.0 이후 쌓인 495개 수정 + 1011개 신규(배치 워커 다수·별칭/검색·캔들 롤업/티어링·Azure 번역 전환·QA/성능 문서)를 `release: v1.0 public snapshot` 한 커밋으로 푸시. GitHub 계정은 둘 로그인돼 있어(jjmin333 active / jejeongmin7 비active) 사용자에게 확인 후 jjmin333 선택.
- **09-14 (후속 — 사용자 요청 4건)** ① **별칭 하나에 대상 여럿.** 사용자가 「구글」에 GOOGL·GOOG을 둘 다 만들어 뒀는데 검색에는 GOOG만 나왔다. 표의 고유 키가 `(alias, kind, target)`이라 **행 둘은 처음부터 저장돼 있었고**, 못 담은 것은 캐시 스냅샷의 `map[string]string`이었다 — 나중에 읽힌 행이 앞 행을 덮었고 읽기에 `ORDER BY`가 없어 **어느 쪽이 남을지는 MySQL이 정했다.** `map[string][]string` + `ORDER BY id`로 고치고 쓰는 쪽을 갈랐다: **검색은 전부**(어느 쪽을 찾는지는 검색한 사람만 안다), **태깅은 첫 칸 하나**(기사는 회사 이야기지 주식 종류 이야기가 아니다 — 둘 다 붙으면 한 줄에 같은 회사가 두 번 서고 시세도 두 벌). 고치다 **끈 행 하나가 별칭 전체를 지우던 버그**를 같이 잡았다(`delete(snap.brand, alias)` — 대상이 하나뿐이던 시절엔 같은 뜻이었다). 행 접는 규칙을 `sql.Rows` 반복문 밖 `aliasFold`로 꺼냈다 — 그 버그가 시험에 안 닿은 이유가 반복문 안이었기 때문이다. 배포 후 실측: 「구글」 → GOOGL·GOOG 둘 다 상위. 관리 화면에 `대상 N` 배지. ② **종목 모달 「간단」이 낱말을 쪼갰다.** `line-clamp-3`은 줄 수로 자르므로 끊기는 자리를 글자 너비가 정한다(사용자 화면: 「…효과를 상쇄하」). 개요 탭은 원래 서버가 갈래마다 첫 문장만 뽑아 엮은 요약을 쓰는데 모달엔 그게 없어 화면에서 같은 규칙을 적용했다(`firstSentences`, 앞 두 문장). 마침표 **뒤 공백**을 함께 봐야 「PER 135.4배」의 소수점에 안 걸린다. 요약이 원문과 같으면 토글을 안 세운다. ③ **리포트 읽기·쓰기 개편.** 읽기는 **다시 2단** — 세 번째다. ①2단(좁은 화면에서 순서 뒤집힘으로 폐기) → ②한 컬럼·계기판을 본문 뒤로(09-14 오전) → ③2단. ②의 진단은 맞았지만 대가가 있었다: **읽는 도중 목표가를 볼 수 없다.** ①이 틀렸던 건 배치가 아니라 좁은 화면 처리였고, 지금은 한 DOM에서 폭이 순서를 정한다(`lg` 미만이면 계기판이 ②의 자리로 쌓인다). 반응 줄을 제목 밑으로 올렸다(커뮤니티 글인데 추천·댓글 수를 다 읽은 사람만 봤다). 쓰기는 **본문을 2순위로** — 사용자가 「작성할 때 너무 사용성이 떨어져 쓰고 싶지가 않다」고 적은 자리다. 입력칸에 닿기까지 카드 다섯 장을 지나야 했는데, 실은 의견 값에 전부 기본값이 있어 **하나도 안 만져도 발행된다.** 의견은 접고 머리줄이 현재 값을 한 줄로 말한다(오류는 접힌 채로도 밖으로). 댓글은 추천(migration 072)·작성자 배지·활동량 배지(구간으로 접는다)·상대 시각·답글 접기. 답글·수정 입력칸이 **아직 한 줄이었다** — 09-14 오전에 본 댓글만 고치고 옆 칸을 빠뜨린 것. ④ **예시 데이터를 묻지 않는다.** 안내 카드로 먼저 묻던 것을 바로 켜는 것으로 바꾸고, 카드는 **예시를 끈 사람**에게 보이게 자리를 옮겼다. 켜는 층 셋(사용자 선택 > 주소창 > 자동)에서 **사용자 선택이 언제나 이긴다** — 끈 사람에게 다시 켜 주면 눌러도 안 되는 버튼이 된다. **브라우저 실측으로 버그 하나를 잡았다**: 한 화면에서 「내 것이 없나」에 답하는 곳이 둘(껍데기=로그인 안 함→참, 본문=목록 0건→받기 전엔 거짓)인데 한 칸에 덮어써서 서로를 지웠다(로그 `true→false→false`, 화면엔 고치려던 카드가 그대로). 기여자마다 한 표씩 **OR로 합치도록** 바꿨다. 판정 규칙을 `lib/sample-mode-state.ts`로 떼어 시험을 붙였다 — **끌 때만 고장나는** 종류라 눈으로는 안 갈린다(09-02에도 같은 모양). ⑤ 072를 넣다 구조적 위험을 봤다 — 마이그레이션 적용 길이 둘이고(빈 DB는 전부 자동, **도는 DB는 손으로 등록**) 잊으면 운영에만 표가 없는데 개발에선 증상이 0이다. `TestEveryMigrationFileIsReferenced`로 센다(일회성 8건은 이유와 함께 면제). FE 250→**282**시험, BE 전체 통과, `go vet`·tsc·eslint 새 경고 0. BE/FE 배포·라이브 검증. **확인 못 한 것**: 댓글 추천 클릭 경로(로그인 자격 없음 — 라우트 401과 응답 필드까지만 실측), LLM 해설 있는 종목 모달 화면. 릴리스 v1.15.0. 배포됨 · 미커밋
- **09-15 (워치독 주기 끊김 — 회차를 싸게 만들었다)** — 사용자가 「게임·무거운 작업 중에 한 번씩 크게 끊긴다, 워치독을 넣은 뒤로 그렇다」고 알려 왔다. 실측이 원인을 갈랐다 — 한 회차 **830ms** 중 점검 자체는 거의 없고 **점검을 준비하는 값**이 대부분이었다: `powershell.exe` 콜드 스타트(20KB 스크립트 AMSI 검사 포함)와 WSL2 VM 을 깨우는 `docker ps`. 로그도 같은 말을 했다 — Docker Desktop 을 띄운 4회 중 셋은 **부팅 직후(로그온 트리거)라 의도된 동작**이었고 켜져 있던 PC 에서 도로 올린 것은 09-09 한 번뿐이다. 범인은 「점검한다」가 아니라 **「점검하려고 매번 새 프로세스를 둘 띄운다」**였다. 그래서 **평소 회차에서 PowerShell 과 docker CLI 를 뺐다** — `watchdog-hidden.vbs` 가 `ServerXMLHTTP` 로 `/health` 만 찌르고 끝낸다(**95ms · 프로세스 1개**). PowerShell 은 **헬스 실패 또는 30분에 한 번 깊은 점검**(`docker ps` 훑기·`/ready`)일 때만 뜬다 — 시간당 12회 → 2회, 총 점유 ~10초 → ~2초. **복구 지연은 그대로**(헬스 실패는 그 회차에 바로 PowerShell 을 띄운다 — 죽은 포트로 돌린 사본으로 실측). 주기 5→10분, 우선순위 7(BELOW_NORMAL)→10(IDLE): 늘어난 최악 지연 5분은 이 워치독이 메우려던 구멍이 **96분**이었으므로 치를 만한 값이다. **`watchdog.paused` 에 만료를 붙였다**(`watchdog-pause.ps1 -Hours 4` · `-Resume` · `-Status` · `-Forever`) — 종전에는 손으로 만들고 손으로 지워야 해서 **지우는 것을 잊으면 워치독이 조용히 꺼진 채로 남았다**. 시각을 `yyyyMMddHHmmss` 로 적는 이유는 **읽는 쪽이 둘**이라서다 — VBScript 의 `CDate` 는 로캘에 끌려다녀 ISO 8601 을 믿을 수 없고, 자리수가 고정되면 두 언어 모두 문자열 비교가 곧 시각 비교다. VBScript 함정 둘을 밟았다: `until` 은 예약어(`Do ... Until`)라 변수로 못 쓰고, 변수 이름이 함수 이름과 겹치면(`deepDue` vs `Function DeepDue`) 「다시 정의된 이름」으로 죽는다 — 둘 다 `wscript` 로는 **조용히** 실패해 `cscript` 로 봐야 보인다. 검증: 빠른 경로·깊은 경로·헬스 실패 대체 경로·일시정지·만료 자동 삭제·재등록된 태스크(PT10M, Priority 10, LastTaskResult 0) 실측. 남은 가능성 — 이 뒤에도 끊긴다면 범인은 **상시 떠 있는 Docker Desktop/WSL2**(`.wslconfig` 없음, 최대 16GB)이거나 **배치 워커 8개의 정기 작업**이다.
- **09-15 후속 (사용자 요청 4건 — 모달 요약·화면 메타 관리·안 보이는 탭·보던 종목)** — ① **모달 「간단」이 자른 글이었다.** 그 자리는 일별 해설(`ai_summary`)의 앞 두 문장인데, 그 해설은 LLM이 쓴 네 갈래를 **순서대로 이어붙인 글**이라 앞 두 문장은 언제나 밸류에이션 이야기다 — 「간단」이라는 이름으로 네 갈래 중 하나만 말하고 있었다(개요 탭이 09-03에 고친 실패와 같은 모양). 같은 호출이 네 갈래 **전부를 근거로 다시 쓴** 한 문단(`overview`)을 이미 내놓지만, 그 값이 사는 곳은 Redis 캐시(24h)와 종목당 한 벌짜리 표라 **그날의 글이 안 남는다.** 모달은 「9-15 기준」이라고 날짜를 적고 글을 보여 주므로 날짜와 글이 같은 행에 있어야 한다 — 073으로 `stock_daily_closes.ai_overview`를 만들고 배치가 본문과 요약을 둘 다 쌓게 했다. 과거 행은 소급 안 한다(그날 원문이 없다) — 비면 화면이 예전 방식으로 돌아간다(`lib/daily-close-ai-summary`, 시험 5). **073에서 기동이 두 번 막혔다**: `invalid connection` 뒤 `Duplicate column` — ALTER가 **클라이언트 연결보다 오래 살아서** 연결은 끊겼는데 서버에선 컬럼이 붙고 있었고 다음 기동의 `columnExists`가 아직 false를 봤다. 같은 패키지의 `isSchemaRaceError`(API+워커 여덟이 동시에 스키마를 맞추는 배포 모양 때문에 만든 것)를 073·074에 물렸다 — **큰 ALTER를 기동에 섞을 때의 기본값**. ② **화면 제목·설명을 배포 없이 고친다**(`/admin/seo`, 074 `page_meta`). 탭 이름과 검색 문구가 Next 소스 상수라 한 줄 고치려면 이미지를 다시 구워야 했다(09-13 별칭과 같은 이유). 규칙은 하나 — **덮어쓰기지 정답이 아니다**: 빈 칸은 코드 기본값이 이기고 전부 비우면 행이 **지워진다**(저장과 되돌리기가 한 입구), 그래서 표가 통째로 없어도 사이트는 그대로 돈다. 기본값 20벌을 화면 파일에서 `lib/page-meta-routes`로 모았다 — 흩어져도 사이트는 잘 돌지만 「어떤 화면을 고칠 수 있고 기본값이 무엇인가」에 실행 시점에 답할 수 없다. `export const metadata` → `generateMetadata()`(덮어쓰기를 **렌더 시점에** 읽어야 한다), **루트 레이아웃까지** — 홈 제목이 거기 `title.default`에 있다. `noindex`는 세 값(안 정했다/노출/제외): 둘로 합치면 한 번 뺐던 화면을 기본값으로 되돌릴 방법이 사라진다. canonical은 그 판정을 따라간다(「세지 말라」면서 정규 주소를 다는 것은 모순). **시험이 실제 버그를 잡았다** — BE 경로 정규화가 소문자로 내려 `/reports/authors/[userId]`가 `[userid]`가 됐다(저장은 되고 오류도 없는데 제목만 안 바뀐다); 경로는 화면에서 **고르는** 값이라 오타 방어가 필요 없어 대소문자 보존으로 되돌렸다. 캐시 주기 300→**60초**(Next 재검증이 stale-while-revalidate라 5분이면 최악 10분을 기다리며 「저장이 안 됐나」를 의심한다). ③ **안 보이는 탭은 서버를 안 두드린다.** 09-05의 「숨어도 3시간」을 사용자가 뒤집었다 — 모니터 둘에 창 다섯씩이면 보는 것은 둘인데 열 탭이 전부 Toss 시세를 당긴다. 유예 **30분**(사용자 지정). 함께 **가드가 빠져 있던 폴링 네 곳**(보유·관심·관심 실험실·재무)을 막았다 — 가드를 `refreshQuotes` 안에 넣으면 첫 적재까지 막히고 부르는 자리마다 `if`를 적으면 또 빠뜨리므로 「주기적으로 부르는 일」 자체를 `useVisibleInterval` 한 이름으로 만들었다. 숨어 있는 동안 건너뛴 회차가 있으면 **다시 보이는 그 순간** 한 번 부른다(사용자의 「즉시 재개」). 실측: 보이면 ~6초마다 1회 → 숨기면 60초 동안 **0회** → 보이게 하면 기다림 없이 1회. ④ **보던 종목 하나는 숨어도 남는다.** 유예가 지나도 연결을 **닫는 대신 좁힌다** — 보유 서른 종목이 뜬 탭이 숨으면 서른이 아니라 「열어 둔 종목」 하나가 남는다(토스가 다른 화면에서도 클릭해 둔 종목의 시세만 띄우는 그 규칙). 핀은 `lib/pinned-symbols`(React 밖 — 허브가 React를 몰라야 시험이 DOM을 안 쓴다), 거는 곳은 종목 상세 화면과 열려 있는 모달. **교집합이 핵심**이다 — 핀만 보고 열면 아무도 구독 안 하는 시장에 **새 연결이 생기고**, 상세 화면은 자기 체결 스트림을 따로 열고 있어 같은 종목에 연결이 둘 난다. FE 282→**312**시험, BE 전체 통과, tsc·`next build`·`go vet` 깨끗, eslint 새 경고 0. BE·FE 배포 후 라이브 실측(제목 덮어쓰기·noindex·되돌리기, 폴링 가드 양방향, 모달 간단/상세). **확인 못 한 것**: `/admin/seo` 실제 편집(관리자 로그인 자격 없음 — 라우트 200과 API 왕복까지), 30분 유예 뒤의 스트림 좁힘(시험은 가짜 시계로 덮었다). 미커밋

- **2026-09-16 (홈이 열어 둔 채로 늙지 않는다 — 사용자 요청 1건)** — 「하루가 지나도 24시간 전 데이터가 그대로 떠 있다」. 홈의 카드는 저마다 `useEffect`에서 **한 번 받고 그대로 서 있었고**, 가격만 스트림·폴링이 밀어 줘서(`use-live-quotes` · 지수 30초) 화면이 살아 있는 것처럼 보였다 — 정작 「오늘의 시장 브리핑」과 「실시간 뉴스」는 마운트 시각의 글이라 어제 브리핑에 「오늘」이라고 적혀 있었다. 주기는 **뒤에서 값을 만드는 주기**를 따른다(`lib/dashboard-refresh`): **5분** — 브리핑(배치 장중 30분·그 외 1시간) · 실시간 뉴스 · 오늘의 키워드(칩과 각주 둘 다) · 거래대금 상위(서버 랭킹 스냅샷 5분) / **30분** — AI 추천 랭킹 · 주요 실적 일정 · 지수 스트립 점수 배지(전부 하루 한 번 도는 배치). **한 카드 안에서 두 시각이 섞이지 않게** 한다 — 키워드 카드는 칩을 자기가 받고 각주만 부모가 주는데, 부모만 갱신했더니 각주는 지금·칩은 마운트 시각이 됐다. 하루에 한 번 바뀌는 값을 5분마다 묻는 것은 스물아홉 번이 헛걸음이다. 규칙 셋: ① 전부 `useVisibleInterval`을 지나 **안 보이는 탭은 회차를 건너뛰고** 돌아오는 순간 한 번 메운다(09-15 요청 3의 훅) ② **갱신의 실패는 화면을 비우지 않는다** — 첫 적재의 실패만 「없음」이고 갱신 실패는 「아직 그것」이다(`loadedRef`) ③ **주기 > 클라이언트 캐시 TTL**이어야 실제로 왕복한다 — 실적 하이라이트 캐시가 정확히 5분이라 느린 쪽이 30분이고, 그 관계를 `dashboard-refresh.test.ts`가 잠근다. 두 랭킹 카드는 「받고·다시 받고·갱신 실패는 지킨다」를 `useRefreshingBlock` 한 이름으로 묶었다. **남긴 것**: `useOpenMarket`의 열린-시장 기본값은 여전히 마운트 때 한 번 — 안 눌렀다는 이유로 보던 목록을 국내↔해외로 갈아 끼우는 편이 더 놀랍다. 홈 외 화면은 손대지 않았다. fe tsc 0 · eslint 변경 파일 0 · vitest 314/315(+3, 기존 실패 `list-price-hub.test.ts` 1건은 작업 전부터). `fe/scripts/deploy.ps1`로 배포 → `quant-port-fe` healthy, **실측**으로 5분 뒤 브리핑·뉴스·랭킹 요청이 한 번씩 더 나가는 것을 확인. 배포 스크립트를 파이프로 감싸면 PS 5.1이 docker stderr를 오류로 올려 nginx 재생성 앞에서 끊긴다. 미커밋

- **09-15 후속2 (「보던 종목」은 브라우저 탭 제목이었다)** — 사진을 화면 오버레이로 읽고 떠 있는 시세 위젯을 제안했다가 「화면 표기가 아니라 **브라우저 탭 제목**」이라는 답을 받았다. 그 한마디가 요청 3·4의 앞뒤를 맞춘다 — **탭 제목은 그 탭이 안 보이는 동안에도 탭 스트립에 그대로 보인다.** 안 보이는 탭의 구독을 다 끊으면서도 종목 하나만 남겨야 하는 이유가 그것이었다. `$141.91 -1.98% | 오라클` — 가격이 맨 앞인 이유는 탭이 여럿이면 제목이 앞에서부터 잘려서다(어느 종목인지는 그 탭을 연 사람이 이미 알고, 모르는 것은 「지금 얼마인가」다). 등락률은 **스트림에 없다**(가격만 온다) — 스냅샷 한 벌에서 전일 종가를 받아 두고 새 가격마다 다시 센다; 스냅샷의 `changeRate`를 그대로 쓰면 「가격은 움직이는데 등락률만 멈춰 있다」가 된다. **탭 제목 컴포넌트가 곧 그 종목의 구독자다** — 핀은 「무엇을 살려 둘까」만 답하고 구독을 만들지 않으므로, 종목 화면을 떠나면 아무도 구독하지 않아 남길 것도 없어진다; 껍데기에 하나 떠 있는 이 컴포넌트가 그 자리를 메운다(실측: `/news/domestic`에서 열린 스트림이 `symbols=005930` 하나뿐). 수명은 **탭이 닫힐 때까지**(사용자 결정 「열자마자 뜨고 ×까지 유지」)라 핀을 active(화면의 생애)와 lastViewed(탭의 생애)로 갈랐고, 새로고침을 견디려 `sessionStorage`를 쓴다 — **탭 하나에만 살고 탭과 함께 죽는** 수명이 여기서 원하는 것과 정확히 같다. **브라우저 실측이 버그를 잡았다**: 메뉴를 옮기면 제목이 페이지 제목으로 돌아간 채 다시 안 얹혔다 — `<title>` 엘리먼트에 `MutationObserver`를 달았는데 Next는 라우트가 바뀔 때 그 **엘리먼트를 갈아끼운다**(옵저버가 떨어져 나간 옛 노드를 보고 있었다). `<head>`를 지켜보게 고쳤다. 유예는 **30분 → 1분**으로 사용자가 직접 내렸다 — 30분은 요청이 겨냥한 상황을 하나도 못 덮는 값이었다(탭을 오가는 동안에는 어떤 탭도 30분 연속으로 숨어 있지 않아 열 탭이 전부 붙어 있는 것과 같다). 라이브 실측(미국장 중·탭 강제 숨김): 숨긴 지 40초 `$330.11` → 75초(유예 초과) `$330.12` → 95초 `$330.17`로 **제목이 계속 흘렀고**, 같은 20초 동안 `/quotes/batch` 폴링은 **0회**, 다시 보이게 하면 즉시 1회. FE 312→**326**시험, BE 전체 통과, tsc·`next build` 깨끗, eslint 새 경고 0. 배포됨 · 미커밋

- **2026-09-16 (해외 뉴스가 19시에 멈춘 원인 · 번역을 로컬 GPU로 되돌렸다)** — 증상은 둘(해외 목록이 18:11에, 해외 브리핑이 19:01에 멈춤)인데 원인은 한 줄이었다: `usTranslationReadySQL`이 해외 목록·카운트에 `COALESCE(headline_ko,'') != ''`를 붙여서 **번역이 안 되면 기사가 늦는 게 아니라 안 보인다.** 수집은 멀쩡했고(finnhub 회사 뉴스 하루 2,589호출) Azure 번역만 10:14에 말랐다 — 그 뒤 화면에 남은 것은 번역이 필요 없는 한국어 소스뿐이었고 그마저 18:11이 마지막. 브리핑은 `buildFacts`가 그 필터된 목록을 읽어 `LatestPublishedAt`이 안 밀리자 슬롯마다 건너뛰었다. **낭비를 DB로 재구성해 검증**: 09-13·14의 정상값은 341,121자(신규 영어 제목 + 그날 처리한 밀린 구간)인데 실제는 1,519,423자 — **1,178,302자, 월 한도 200만의 59%가 같은 제목 반복 전송**(09-14만 14.1배, 요청/성공 19회). 그래서 09-15의 하루치가 30,036자(수요의 1/3)였다. 상한 3에 걸려 영구 제외된 행은 1건뿐이라 「무덤」 가설은 기각. **9월 남은 기간은 로컬로 돈다** — 이 컨테이너를 내렸던 이유 셋이 전부 CPU 전용 전제였는데 이 기계엔 RTX 4070 12GB가 있고 Docker에 nvidia 런타임이 이미 있었다. `server-cuda` 이미지 · **LMT-60-4B Q8_0**(같은 계열이라 프롬프트·자리표시자·glossary가 그대로 유효) · `-ngl 99` · 슬롯 4. 볼륨이 `/root/.cache/llama.cpp`에 걸려 있어 모델 4.3GB가 마운트 **밖**으로 받아지고 있었다(캐시 뿌리로 교체). 이름 잠그기의 구멍 둘을 실측으로 잡았다 — 카탈로그에 없는 이름을 모델이 낱말로 읽고(Perplexity→당혹감 · Anthropic→인간학 · Yahoo Finance→야후 재정, 30일 실측 출처 표기 1,374건) 달러 기호만 떨어지면(`$QPZAA`→`QPZAA`) 되돌리기가 실패했다. `builtinNamePairs`와 관대한 복원으로 고치니 20건 중 19 → **30건 중 30 통과·이름 유실 0**. 밀린 2,895건(7일 창 밖 580 포함)은 `cmd/backfill-translations`+`ListMissingTranslationWindow`로 **36건까지** 비웠다(2.3건/초, 13분 25초). **드레인 도중 번역기를 재시작했다가 615건을 태웠고 그게 결함을 드러냈다** — `Local`이 요청 실패를 삼키고 빈 결과를 돌려줘서 「엔진이 죽었다」가 「이 제목은 못 옮긴다」로 세어졌다(상한 3 = 영구 제외). 보낸 것이 전부 실패했을 때만 오류를 올리도록 고치고 카운터를 되돌려 재실행. 메모리는 6G로도 모자랐다 — GPU 전층 오프로드인데 호스트 anon이 6.2GB(WSL2 CUDA는 VRAM을 호스트 익명 메모리로 잡는다), `memory.events` max 100,937회·VM 스왑 3GB·속도 6.8→1.5건/초. 10G로 올려 4.2GB에서 안정. 시험 +9(translate 8 · repository DB 1) — 그중 하나를 쓰다 `t.Cleanup`이 `defer db.Close()` 뒤에 도는 바람에 운영 표에 시험 행 둘을 남겼다(defer로 교체, 오류를 삼키지 않게). 재빌드·13/13 헬시, 라이브 확인(목록 12분 전 기사까지 · 브리핑 01:36 재생성). 미결: 10월 이후 로컬 상주냐 Azure S1(월 약 $22)이냐, 그리고 번역기가 죽으면 목록이 사라지는 게이트 단일 실패점. 미커밋

- **2026-09-16 후속 (「탭을 계속 띄워놓고 있었는데 제대로 안 바뀌던데」)** — 원인이 둘이었다. **① 이미 열려 있던 탭에는 새 코드가 없다** — FE는 컨테이너라 배포(00:20 KST) 이전에 연 탭은 옛 번들을 들고 있다(새로고침 한 번이 필요하고, 이건 변경의 성질이지 버그가 아니다). **② 진짜 결함**: `useVisibleInterval`이 「숨어 있는 동안 회차가 **울렸는가**」를 표식으로 들고 있다가 돌아올 때 메웠는데, 그 전제가 **숨은 탭에서도 타이머가 돈다**는 것이었다. 크롬은 5분 넘게 숨은 탭의 타이머를 분당 한 번으로 내리고 조건이 맞으면 탭을 **얼린다**(콜백 0회), 창이 완전히 가려져도 `hidden`이며, 노트북을 덮으면 타이머는 멈춘 채 시각만 흐른다 — 그래서 몇 시간이 지나도 표식이 false인 채라 **돌아와도 아무 일이 없었다.** 이제 **마지막으로 부른 시각** 하나만 센다(`lib/refresh-due` + 시험 6): 깨어날 기회는 넷(회차 · 다시 보임 · 창 포커스 · bfcache `pageshow`)이지만 질문은 하나 — 「마지막으로 부른 지 한 주기가 지났는가」. 같은 함정을 `lib/stream-awake`가 먼저 밟았다(**「울렸으니 지났다」는 믿을 수 없다**). 부수 효과로 잠깐 숨었다 돌아올 때의 헛걸음도 사라졌다. 이 훅을 쓰는 보유·관심·관심 실험실·종목 상세 펀더멘털 폴링이 함께 고쳐졌다. **실측**(가짜 시계): 6분 경과를 흉내 내고 `visibilitychange`를 쏘자 홈의 5분 그룹 다섯 요청이 즉시 다시 나갔고, 시간을 안 돌린 채 `visibilitychange`+`focus`를 쏘자 **한 건도 안 나갔다.** fe tsc 0 · eslint 변경 파일 0 · vitest 332/332(+6) — 앞 줄에 적은 `list-price-hub.test.ts` 실패는 깜빡이는 시험이었다(연속 재실행 통과). 배포됨 · 미커밋

- **2026-09-16 후속 (홈 밖의 화면도 늙지 않는다 · 지수 해설이 한 거래일 뒤에 있던 이유 · 컨테이너 로그 일제 점검)** — 사용자 요청 3건. **① 갱신을 앱 전체로 넓혔다**: `lib/dashboard-refresh` → `lib/screen-refresh`(이름이 곧 범위다). 5분 — 뉴스 목록·브리핑·언급 종목·오늘의 키워드·많이 본 뉴스·리포트 피드(사람이 지금 만드는 값). 30분 — AI 추천 랭킹·실적 일정/집계·다가오는 일정·목표가 예측(하루 한 번 도는 배치의 산물, DB가 1 ECPU다). 갱신은 스피너를 안 띄우고 실패해도 화면을 안 비운다 — 뉴스는 새로고침 단추와 **같은 문**(`refreshKey`)으로 들어가되 `silentRefreshKey`로 가른다(불리언이면 먼저 도는 이펙트가 내려 버려 나머지 둘이 「조용한 회차」를 모른다 → **회차 번호**를 든다). 실측: `/news/domestic` 301초에 목록·브리핑·언급 종목이 한 번씩 더. **② 브리핑 상단 지수 칩이 눌린다** — 전역 `QuickLookProvider.openIndex`로 연다(카드가 모달을 직접 들면 지수 스냅샷·점수 훅을 한 벌 더 든다). **③ 「지수 LLM이 안 돈다」는 세 고장이었다.** (A) **한 번의 실패가 열셋을 데려갔다** — 09-16 00:25 회차가 생성 0·실패 11·**토큰 0**이고 실패가 연속이라는 모양이 원인을 좁혔다. 간격 대기가 **성공한 회차에만** 있어 실패 뒤 다음 호출이 `ErrPacing`에 막혔고, 429의 `BlockFor` 구간에 나머지가 `ErrExhausted`로 떨어졌다. 간격을 성공·실패 무관으로 두고, `ErrExhausted`면 멈추고 `HaltReason`을 남긴다. (B) **실패한 날을 통째로 잃었다** — 자동 호출자가 00:25 하나뿐. **16:30 만회 슬롯**을 넣었다: Gemini 무료 한도가 태평양 자정(**16:00 KST**)에 리셋되는데 00:25는 그 날치의 8시간째이자 17:10 국내 요약(706,392토큰·stop=quota) 뒤라, 지수 해설은 줄곧 **남은 것으로** 부르고 있었다. `force` 없이 빈 칸만 채우므로 평소엔 SELECT 열셋으로 끝난다. (C) **`us10y`가 `dailyCloseIndexSchedules`에 없었다** — 없는 키는 빈 ScheduleID라 `IngestIndexCloses`가 **오류도 로그도 없이** 건너뛴다. 라이브 값은 FRED에서 바로 읽어 멀쩡했고 **일별 종가에 기대는 것만** 08-27에 멈춰 있었다(종가표·추세 점수·키워드·해설, 3주). `TestEveryCatalogIndexHasACloseSchedule`이 카탈로그와의 동수를 잠근다. 복구: `backfill-index-closes -since=2026-08-20`(225행·13개 재채점) + `index-notes` 2회(20건, 13,830토큰) → 열두 지수 전부 해설이 점수와 같은 행에. `gold`만 27봉이라 문턱 30 미만(사흘 뒤 자동). **④ 컨테이너 13개 24시간 로그**: 5xx 0 · nginx 오류 0. `/earnings/highlights` 3초 초과 15회(캐시 만료), stock summary LLM 타임아웃 8건, toss failover 15건(설계대로지만 MARKET_DATA 여유 감소 신호), kakao 2,224/3,000, `/reports` **React #418**(`daysUntilDue`가 렌더 중 `Date.now()`를 읽어 SSR/클라 D-N이 갈릴 수 있다), 공개 저장소 이후 `.git/config`·`.env` 스캔, 번역기 프롬프트 캐시 축출 436회(truncated 0), worker-fetch 종목당 한 줄 4,353줄. fe tsc 0 · eslint 새 오류 0 · vitest 332/332 · go build·vet·test 통과(+1). 배포됨 · 미커밋. **남긴 것**: `/reports` #418 미수정(요청 밖) · 「렌더 중 시계 읽기」 전수 점검 안 함 · 워치독이 내일 16:30까지 `index_notes` 「늦음」 1건을 띄운다(첫 슬롯 전) · `/journal`은 본인만 쓰는 데이터라 주기 없음.

- **2026-09-17 (V2 착수 — AI 점수에 「왜」를 붙였다)** — 사용자 V2 요구 10건. 전부가 기대는 토대를 먼저 놓았다: `GET /stocks/{symbol}/score/explain` — **저장된 배치 산출물만** 읽어 점수의 근거와 변화 원인을 조립한다(실측 0.23초 · 15KB · 쿼리 1회 · LLM 0 · 외부 호출 0). 조사해 보니 재료는 이미 전부 DB에 있었다 — `stock_scores.inputs`가 지표 원값 · 피어 백분위 · **어느 그룹에서 순위를 매겼는지** · 도메인 밖이라 버린 값 · 기술적 지표 17종을 날짜마다 이고 있다. 없던 것은 데이터가 아니라 입구였다. **귀인이 근사가 아니다**: 종합 점수가 저장값만으로 정확히 재현되고(005930 09-16 raw 47.64 → 48, 오차 0), 가중치·축 유무·축소율을 고정하면 raw는 축 점수에 선형이라 두 날의 차이를 ① 축 값 변화 ② **축이 생기거나 사라져 분모가 바뀜** ③ 커버리지에 따른 축소율 변화로 **잔차 없이** 가른다(남는 건 정수 반올림뿐이고 그것도 `roundingResidual`로 적는다). **그래서 보인 것**: 점수의 하루 변동은 시장이 아니라 축 유무가 만든다 — 전일 대비 **AAPL +11점 중 +10.75가 「가치 축이 사라져서」**, 005930 +2 중 −0.65와 035720 −2 중 −1.62가 심리·수급 축 소멸. 설명 기능 없이는 보이지 않던 사실이고, 이걸 「추세가 좋아졌다」로 말하면 점수가 신뢰를 잃는다. 없음을 0으로 안 때운다 — 축 결측을 `no_data`/`not_applicable`(ETF)로 가르고, 버린 값은 도메인과 함께 보여 주며(035720 EPS 성장률 1170.9%), 비교 대상이 없으면 `no_prior_row`·`score_withheld`로 거절한다. 전주 창은 「7회차 전」이 아니라 **달력 7일 이전의 가장 가까운 회차**(배치가 매일 돌지 않는다 — 실측 26/30). 요구 8의 점수 몫도 포함 — `provenance`(기준일·계산 시각·actual/partial·출처). 시험 17개는 고정값을 지어내지 않고 **운영 DB에서 읽은 실제 행**으로 잠갔다. go build·vet·test 전체 통과 · Tesseract OK · 배포됨 · 미커밋. 계획 전문 `docs/V2_PLAN.md`(Phase 1~9). **남긴 것**: FE 없음(화면은 제안 후 착수) · Phase 2 백테스트는 `stock_scores` 30거래일 보관 제약 때문에 얇은 랭킹 이력 테이블(연 35MB)이 먼저 필요하다.

- **2026-09-17 후속 (점수 설명을 화면 넷에 붙였다)** — 사용자가 A·B·C·D **전부** 진행하되 ① 개요 카드에 「데이터 없음」 글자를 덧붙이지 말 것(「-」가 같은 말이다) ② 설명문으로 자리를 먹지 말 것 ③ **사용자 문장은 ~입니다·~합니다** ④ **모바일 가독성 최우선, 상하면 AI 핵심 요약처럼 간단/상세로 가를 것**을 정했다. **A** 개요 점수 카드 한 줄(「전일 +2 · 추세 변화 · 근거 ›」, 축 막대 아래 카드 테두리 위, +28px, 원인은 가장 큰 하나만 — 둘이면 줄이 접혀 최상단이 두 층이 된다). **B** AI 분석 탭 `score-explain-panel.tsx` — `MiniToggle` 간단/상세(기본 간단, 상태 저장 없음). 간단=구성 막대+전일 상위 3+기준일, 상세=+전주 창·드라이버별 지표 변화·축별 지표 표·제외된 값·출처. **표를 4열로 두지 않았다**(375px에서 「지표·값·순위·비교집단」은 어느 칸도 못 읽는다) — 비교 집단은 **축 머리에 한 번**, 혼자 다른 집단인 지표만 자기 줄에. 설명문 대신 **숫자 둘을 나란히**: `33.3% (설정 30)`. **C** 보유·관심 「점수 변화」 카드는 **목록 행이 아니라 레일**로 갔다 — 관심 목록 점수 칸은 36px, 보유 목록엔 점수 칸이 없어 사유를 넣으면 옆 칸부터 잘린다. 행마다 15KB도 못 부르므로 요약 입구를 새로 냈다: `GET /stocks/scores/changes`(델타 1 + 최대 원인 1, 10종목 2.6KB · 0.25초 · 쿼리 1회, 윈도 함수로 종목당 최근 2행 · `inputs` 블롭은 안 읽는다). 레일 폭에선 「가치·심리·수급 축 결측」이 잘려 정작 「결측」이 사라져 `compactDriverLabel`(「축 결측」)을 따로 뒀고 온전한 문장은 `title`에. **D** 관리자 `GET /admin/score/explain-weights` — 저장된 한 행을 다른 가중치로 다시 합산(쓰기·배치 없음). 가중치는 **분모에도 들어가 선형이 아니라서** 축을 정해진 순서로 하나씩 바꾸며 증분을 적고, 합만 정확하다는 것과 그 순서를 응답·화면이 함께 밝힌다. **읽히게 만든 것**: `domestic:KOSPI:cap3` → 「국내 KOSPI 대형주」(`peerGroupLabel`), 출처에서 테이블 이름 제거, 그리고 **구성 막대에 있던 축이 근거 목록에서만 사라지지 않게** 했다(심리·수급은 뉴스 감성·내부자 거래에서 나와 지표 표가 없다 — 그 사실을 한 줄로 적는다). **실측**(컨테이너 재빌드 후, 운영 데이터): 375px에서 개요 한 줄·간단·상세(지표 변화 11줄, 축별 표 4축 12지표)·레일 카드 전부 가로 넘침 0, 1280px 동일. 관심 레일 실제 출력 — 애플 72 **+11 축 결측** · NVIDIA 68 +8 축 결측 · 삼성전자 48 +2 추세 · 카카오 29 −2 축 결측 · SK하이닉스 79 +1 추세. go build·vet·test 통과(시험 +7, 누적 24) · fe tsc 0 · eslint 새 오류 0. 배포됨 · 미커밋. **남긴 것**: D는 관리자 로그인이 필요해 **화면 실측을 못 했다**(단위 시험 4개·타입 검사까지만).

- **2026-09-18 (백테스트 — 점수가 맞았는지 처음으로 재 봤다)** — V2 Phase 2. `stock_scores`가 30거래일에서 잘려(연 7.5GB) 백테스트가 읽을 과거가 없었고, 필요한 건 그 표의 0.5%인 「그날 상위 200」뿐이라 **얇은 표**를 만들었다: `score_ranking_history`(시장 2 × 축 6 × 200 = 하루 2,400행 · **연 35MB**, `rank`가 MySQL 8 예약어라 `rank_no`). 점수 배치 꼬리에서 스냅샷 + 백필을 부른다 — **정리보다 먼저**(랭킹 사전계산과 같은 이유). 재료는 기존 `WindowBests`(하루짜리 창)라 새 무거운 쿼리가 없고, 자격 집합(`RankableSymbols`)도 화면 랭킹과 같은 것을 **자르기 전에** 건다. 백필 실측 **16초**에 27거래일 확보. **엔진이 지키는 셋**: ① **미래를 보지 않는다** — D일 랭킹은 D일 종가로 계산되므로 수익은 D+1부터다(`latestOnOrBefore(dates, prev)`; `<=`면 「그날 폭등한 종목을 그날 아침에 고른」 게 되어 어떤 랭킹이든 좋아 보인다). 시험이 3일 +100% 종목을 3일 랭킹에만 심고 그 폭등을 한 푼도 못 먹는지 잰다. ② **없는 종가를 0%로 안 센다**(거래정지를 보합으로 세면 나머지가 희석 — 시험이 +10%가 +5%로 떨어지는지 잰다). ③ **짧으면 비운다** — 28거래일 Sharpe가 4.65였다(운을 실력으로 읽은 숫자). 연 환산·Sharpe는 40거래일부터, 변동성만 20거래일부터. **결과**(국내 · 상위 20 · 주간 · 28거래일, KOSPI +7.30%): 심리·수급 +9.35%p(표본 11일) · 성장 +6.77%p · 추세 −2.26%p · 수익성 −5.15%p · **종합 −8.89%p** · 가치 −9.29%p — **종합이 다섯 축 중 꼴찌다.** 28거래일이 아무것도 증명하진 않지만 「가중 평균이 신호를 상쇄한다」가 측정 가능한 가설이 됐다. `GET /stocks/scores/backtest` 0.24초 · 2.9KB · 읽기 2회. 사전 계산으로 안 옮긴 이유는 조합이 곱셈으로 느는데 대부분 안 열리기 때문. **관리자 계정 생성** — 지난 회차에 못 끝낸 D의 화면 실측을 마쳤다(`admin@quantport.local`/`admin1234`, 앱이 8자 미만을 거절한다; `users.role='admin'`). 005930 가치 가중치 15 → 40 → 48점이 43점, 드라이버 −5.02가 exactDelta와 일치, 저장 없음. ⚠️ **로컬 개발 DB 픽스처**(`host.docker.internal:3306`)이고 nginx가 0.0.0.0에 붙어 있으니 공개 배포 전 삭제·변경. go build·vet·test 통과(시험 +12, 누적 36) · 배포됨 · 미커밋. **남긴 것**: FE 없음 — 배치 제안 `docs/mockups/score-backtest-v1.html`(곡선·숫자 전부 실제 결과). 그 문서의 진짜 질문은 「28거래일 결과를 사용자에게 보여 줄까」다.

- **2026-09-18 후속 (벤치마크를 여럿으로 · 배치 제안을 실제 페이지 위에서 다시)** — 사용자 요청 2건. **① 내 제안이 틀렸던 것을 고쳤다**: 「`/analysis` 사이드 레일 카드」라고 적었는데 **그 화면에는 사이드 레일이 없다.** 실제 구조는 두 칼럼(왼쪽 랭킹 20rem · 오른쪽 `StockWorkspaceClient`)이고, **종목을 고르기 전 오른쪽은 「종목을 선택하면 상세가 표시됩니다」 한 줄뿐인 빈 화면**이다. 그 빈 칸이 제자리다 — 고르기 전이 정확히 「이 랭킹을 믿을까」를 묻는 순간이고, 고르면 워크스페이스가 그 자리를 가져가며, 기존 화면에서 밀려나는 것이 **없다**(지금 그 자리엔 회색 안내문 한 줄). 모바일은 빈 칸이 없어(`isCompact`) 목록 위 접힌 한 줄(+30px). **② 벤치마크 다중화**: 국내 KOSPI·KOSDAQ / 해외 S&P 500·나스닥 종합·나스닥 100·필라델피아 반도체·다우존스, 기본은 국내 둘·해외 셋(넷을 넘기면 어느 선이 내 것인지 안 읽힌다). 실측이 곧바로 이유를 보여 줬다 — 국내 종합 초과수익이 **KOSPI 대비 −8.89%p인데 KOSDAQ 대비 −4.52%p**다. **화이트리스트**인 이유: 카탈로그의 VIX·미 국채 10년·USD/KRW·비트코인·금도 전부 일별 종가가 있지만(2026-02-02부터 155~157거래일, 랭킹 이력보다 훨씬 깊다) 주식 바구니의 벤치마크가 아니다 — 숫자는 나오는데 뜻이 없고 화면에 나가면 성적으로 읽힌다. **지수마다 휴장일이 다르므로** 각 지수는 포트폴리오와 같은 달력 위에서 자기 종가가 연속인 구간만 재고 **실제로 잰 날 수(`days`)를 행마다** 내보낸다(빠진 날을 0%로 세면 초과수익이 부풀려진다 — 시험이 03-03이 빈 KOSPI로 잰다). **이력 없는 지수는 행을 안 만든다**(0%짜리 벤치마크는 포트폴리오가 이긴 것처럼 읽힌다); 기준의 이력이 없으면 머리줄 기준을 실제로 잰 첫 번째로 옮긴다. `GET /stocks/scores/backtest/benchmarks`가 시장별 목록·기본값을 줘서 화면이 한국어 이름을 하드코딩하지 않는다. go build·vet·test 통과(시험 +4, 누적 40) · 배포됨 · 미커밋. **남긴 것**: FE 없음(제안 `docs/mockups/score-backtest-v2-analysis-page.html`) · **종목 하나의 벤치마크**(`benchmarkForStock` — 리포트·스탬프)는 손대지 않았다: 리포트는 발행 시점에 벤치마크를 **동결**하므로 바꾸면 앞으로 발행되는 것만 달라지고 과거와 기준이 갈린다.

- **2026-09-18 후속2 (백테스트 화면을 붙였다 — 그리고 「빈 칸」이 없다는 것을 알았다)** — 사용자 승인대로 FE 착수. **제안이 또 틀렸다**: 「종목을 고르기 전 빈 오른쪽 칸」은 **사용자에게 거의 닿지 않는다** — 넓은 화면은 주소에 종목이 없으면 기본 종목(005930)을 자동으로 연다(`analysis/page.tsx`의 `DEFAULT_SYMBOL`). 코드를 읽을 때가 아니라 **1440px로 열어 봤을 때** 알았다(오른쪽에 삼성전자가 떠 있었다). 진입 동작을 말없이 바꾸지 않고 **문을 따로 달았다**: 목록 툴바 아래 접힌 한 줄(`ScoreBacktestSummaryRow`, 28px) → 넓은 화면은 오른쪽 칸이, 좁은 화면은 오버레이가 성적으로 바뀐다. 종목을 고르면 닫힌다(고르는 행동의 뜻이 「이 종목을 보겠다」이므로). 기본값을 성적으로 바꾸려면 `workspaceSymbol`의 `defaultSymbol` 갈래 한 줄을 지우면 된다. 목록에 `afterToolbar` 슬롯을 열었다 — 부모에서 그리면 국내/해외 탭보다 **위로** 올라가 시장을 고르기도 전에 그 시장 성적이 먼저 보인다. **패널이 지키는 것**: 없는 숫자를 0으로 안 채운다(연 환산·Sharpe가 비면 「—」) · 표본을 숨기지 않는다 · **벤치마크를 색으로 안 가른다**(빨강·파랑은 이 앱에서 상승·하락이다 — 지수는 점선 간격, 포트폴리오 선만 결과의 색). 차트는 **라이브러리 없이** 인라인 SVG(점 30개에 차트 엔진은 번들만 키운다); 휴장으로 빈 지수 값은 직전 값을 이어 그린다(선을 끊으면 「지수가 사라졌다」로 읽힌다 — 수익률은 서버가 그 날을 빼고 계산하므로 이음은 그림에만 있다). **실측**: 1440px에서 접힌 줄 → 곡선 3개·KPI 5·비교 지수 표·경고, 손잡이가 실제로 다시 돈다(매주 −1.59% → 매일 −1.00% → 상위 100·매일 +0.46%, API 실측과 일치), 종목 선택 시 닫힘. 375px 「‹ 목록」 오버레이, 가로 넘침 0. 승률 42.86% → 42.9%. fe tsc 0 · eslint 새 오류 0(내 훅의 `set-state-in-effect`는 저장소 패턴대로 파생값으로 바꿔 없앴다; `analysis-client`의 React Compiler 경고는 기존 5건 → 1건) · go 전체 통과 · 배포됨 · 미커밋. **남긴 것**: 종목 하나의 벤치마크(`benchmarkForStock`)는 손대지 않았다 — 리포트가 발행 시점에 동결한다.

- **2026-09-18 후속3 (종목 타임라인 BE — 「설명이 안 되는 날」까지 보여 준다)** — V2 Phase 3. `GET /stocks/{symbol}/timeline?days=90`, **새 표·새 수집 없이** 다섯 표(종가·점수·실적·뉴스·내부자·공시)를 같은 날짜 축에 세운다. 실측 0.42초 · 16.7KB · 읽기 여섯(전부 인덱스). **뉴스는 SQL이 접는다** — 삼성전자에 붙은 기사 15,232건(하루 평균 180건)을 행으로 가져오면 수 MB라, `MEMBER OF`로 `idx_news_symbols_mv`를 타고 `GROUP BY`로 날짜별 건수만 받는다(0.21초; `JSON_CONTAINS`면 전체 스캔). **제1 규칙 — 레인마다 시작일이 다르다**: 주가 2024-12-16 · 뉴스 2026-01-14 · 실적 2025-07-08 · 내부자 2026-07-21 · 공시 2026-08-20 · 점수 2026-08-07. 1년을 그리면 열 달치가 비는데 그건 「아무 일 없었다」가 아니라 **「수집 전」**이라, 모든 레인이 `availableFrom`·`coversWindow`를 들고 나간다(요구 8). **고르는 규칙 둘**: ① 크게 움직인 날 — 문턱이 **종목마다 다르다**(창의 표준편차 ×1.5, 2~10%로 가둠; 005930은 변동성 5.82%라 ±8.73%) ② 분명한 사건(실적·공시·내부자·점수 ±3 이상)은 **등락과 무관하게** 남긴다(「실적을 냈는데 주가가 안 움직였다」도 답이다). **뉴스 건수로는 고르지 않는다** — 그러면 거의 모든 날이 남는다. **실측**(90일 → 39일): 07-31 +21.31%(뉴스 732·내부자 2) · 07-28 −14.51%(뉴스 990) · 07-07 −9.06%(실적 발표) · **07-13 −10.12%인데 뉴스 1건**. 마지막 줄이 이 기능의 성격을 정한다 — 우리 데이터로 설명이 안 되는 날이고, 그걸 그대로 보여 주는 게 정직성이다. 점수의 「왜」는 **Phase 1 귀인 엔진을 그대로** 쓴다(`attributeChange`) — 따로 계산하면 같은 날에 두 화면이 다른 말을 한다. 컨센서스가 없으면 서프라이즈를 만들지 않는다(추정 없이 0%면 「예상대로였다」가 되는데 사실은 「비교할 예상이 없었다」). go build·vet·test 통과(시험 +8, 누적 48) · 배포됨 · 미커밋. **남긴 것**: FE 없음 — 제안 `docs/mockups/stock-timeline-v1.html`. 배치 후보 셋(B. AI 분석 탭 하단 권장 · A. 새 탭 — 탭 열 개째라 `stock-workspace-tab-bar.tsx` 주석의 「아홉이 폰 폭을 넘친다」와 부딪힌다 · C. 개요 차트 토글). 뉴스 제목은 안 싣는다 — 기사별 감성이 저장돼 있지 않고 하루 180건이라, 날짜를 누르면 기존 뉴스 목록 API가 답하는 쪽을 제안.

- **2026-09-18 후속4 (타임라인 화면 — 「+21.31%」를 누르면 그날 아침 기사가 나온다)** — Phase 3 FE, **AI 분석 탭 하단**(점수 근거 바로 다음). 새 탭을 안 만든 이유는 탭이 이미 아홉이고 `stock-workspace-tab-bar.tsx` 주석이 「아홉이 폰 폭을 넘친다」고 이미 적고 있어서다. **날짜를 펼치면 그날 기사**가 오는데, 기존 뉴스 목록 API에 날짜 필터를 **더하지 않았다** — `NewsStore.List`는 15초 TTL 캐시·싱글플라이트·만료분 선공급이 얹힌 앱에서 가장 손이 간 읽기 경로이고, 날짜 필터는 사람마다 다른 날을 펴므로 거의 재사용되지 않는 캐시 키를 섞는다. 타임라인이 자기 조회를 든다(`/timeline/news?date=`, 0.29초). **실측이 잡아 준 결함 둘**: ① 정렬이 `view_count DESC, published_at DESC`였는데 이 DB의 조회수가 사실상 전부 0이라 결과가 조용히 **「그날 마지막 20분」**이 됐다(07-31을 열었더니 23:21~23:40 다섯 건). 동점일 때 **이른 시각**을 먼저 두자 「한국투자證 목표가 65만원 상향」·「UBS 내년 HBM 1위」·「[속보] 코스피 13% 폭등…삼성전자 21%」가 올라왔다 — 하루를 설명하는 기사는 장 전과 장중에 나온다(조회수가 쌓이면 첫 키가 다시 일하도록 규칙은 남겼다). ② 내부자 **매도가 「+381주」**로 나왔다 — 매수·매도는 크기이지 방향이 아니다. 방향이 뜻을 갖는 건 순증감뿐이라 `formatSignedShares`를 따로 뒀다. **실측**(375px·1440px): 07-31 행을 펴면 「내부자 거래 2건 — 매수 3,045주 · 매도 381주 · 순+2,664주」와 「그날의 뉴스 전체 732건 중」 열두 줄, 가로 넘침 0. 「수집 전」 레인 칩(주황)은 AI 점수 08-07 · 내부자 07-21 · 공시 08-20. 문턱 줄 「±8.73% 이상을 큰 등락으로 봤습니다 (이 기간 변동성 5.82%)」. fe tsc 0 · eslint 새 오류 0 · go 전체 통과 · 배포됨 · 미커밋. **남긴 것**: 기사별 감성은 여전히 저장돼 있지 않다(요약 파이프라인이 그때그때 센다) — 뉴스 레인은 건수와 제목까지. 남은 단계 4·5·6·7·8·9.

- **2026-09-18 후속5 (보유 이유 점검 — 수익률을 판정에서 빼자 답이 달라졌다)** — V2 Phase 6, BE+FE. `GET /user/holdings/diagnosis`가 보유 종목마다 「살 때의 이유가 지금도 유효한가」를 **강화/유지/약화/판단 보류**로 답한다. 근거 셋(AI 점수 추세 · 실적 서프라이즈 · 내부자 거래)이고 **못 쓴 근거도 목록에 남긴다** — 무엇을 보고 판단했는지보다 무엇을 **못 보고** 판단했는지가 더 중요할 때가 있다. 보유 수와 무관하게 조회 5회. **수익률은 판정에 넣지 않았다**: 「올랐으니 이유가 강화됐다」는 결과를 근거로 되돌리는 말이라, 넣는 순간 이 기능은 「오른 종목은 좋은 종목」이라고 말하는 값비싼 장치가 된다. 실측이 그 분리를 증명한다 — 삼성전자 **+37.6%에 「약화」**, 엔비디아 +21.8%에 「유지」. **실측이 잡아 준 결함 셋**: ① 점수 추세가 **매수 전부터** 잡혔다(08-11 매수인데 08-07부터) — 실적·내부자만 보유 기간으로 자르고 점수를 안 잘랐다. ② 「신호 둘 미만이면 보류」 규칙이 **가장 강한 증거를 묻었다** — 카카오는 매수 시점 75점이 29점으로 **−46**인데 실적·내부자 기록이 없어 「판단 보류」가 됐다. 사용자가 가장 봐야 할 종목이 가장 흐린 답을 받은 것이다. 약한 것은 증거의 **다양성**이지 강도가 아니라서, 문턱의 두 배를 넘는 신호 하나는 혼자 판정하게 했다. ③ **아무 말도 안 하는 증거가 판정을 뒤집었다** — 카카오는 강한 −46 하나로 「약화」인데 삼성전자는 강한 −13 옆에 방향 0짜리 신호가 붙자 합이 문턱에 못 미쳐 「유지」가 됐다. 침묵을 더했다고 답이 바뀌면 그건 판정이 아니라 신호 개수 세기다 → 강한 신호는 두 몫을 센다. 그 옆에서 **내부자 순매매에 크기 개념이 없다는 것**도 드러났다: 애플 3건·4,316주가 엔비디아 563만 주와 똑같이 −1로 세어져 점수 +22를 상쇄했다. 하루 거래량 대비로 재면 애플 0.011% · 삼성 0.077% · 하이닉스 0.27% · 엔비디아 4.7%라 **1%** 아래는 방향을 0으로 둔다 — 다만 신호를 **버리지는 않는다**. 「재 봤으나 방향이 없음(=)」과 「잴 수 없었음(−)」은 화면이 달리 말해야 하는 두 상태다(요구 8). **매수 시점 점수는 거의 언제나 없다** — 점수는 30거래일만 보관하는데 매수는 그보다 오래됐다. 빈칸이 「안 변했다」로 읽히므로 사유를 함께 낸다. FE는 목업 `docs/mockups/holdings-diagnosis-v1.html`로 배치 A/B/C를 제시해 **A(보유 요약 바로 아래)**를 선택받았다. 간단/상세 `MiniToggle`, 375px에서는 근거 수·수익률을 접는다. 판정 색에 **빨강·파랑을 안 썼다** — 이 앱에서 그 둘은 상승·하락이라 같은 줄에서 색 하나가 두 가지를 말하게 된다. 관심종목에는 안 붙였다(매수 이력이 없어 보유 기간이 없고, 남는 근거가 점수 하나뿐이라 판정이 언제나 보류가 된다). **실측**: 779px·375px, 라이트·다크, 간단·상세 모두 가로 넘침 0. 「순매도 5634565주」가 쉼표 없이 나가던 것도 고쳤다(일곱 자리는 네 자리와 나란히 서면 크기 비교가 안 된다). go build·vet·test 통과(진단 시험 15) · fe tsc 0 · eslint 새 오류 0 · 배포됨 · 미커밋. ⚠️ **기존 결함 발견(내 코드 아님, 미수정)**: 시험용 보유를 넣다가 `holding_trades`가 **API 재시작마다 사라지는** 것을 봤다. `MigrateHoldingTradesPriceMismatchCleanupIfNeeded`가 시작마다 돌면서 체결가가 그날 종가와 0.01 이상 다른 이력을 전부 지운다 — 실제 체결가는 장중가라 거의 언제나 종가와 다르다. 재현·확인 완료(52,000원 → 재시작 후 소멸, 실제 종가 186,000원 → 생존). 쓰기 경로는 아무 가격이나 받는데 시작 경로가 지우므로, 손으로 적은 매수 기록이 말없이 사라진다. 별도 작업으로 분리했다. **남은 단계**: 4(스크리너) · 5(Bull/Base/Bear 목표가) · 7(리포트 성과 집계) · 8(신뢰도 표기 전역화) · 9(통합 흐름).

- **2026-09-18 후속6 (종목 스크리너 — 결과가 0일 때 「왜 0인지」를 말한다)** — V2 Phase 4, BE+FE, 새 메뉴 `/screener`(사용자 결정 A). **새 표도 새 수집도 새 배치도 없다** — 조건에 쓰는 값이 이미 `stock_scores` 한 줄 안에 전부 있었다(`inputs.raw`의 ROE·부채비율·PER·PBR, `inputs.technical`의 RSI·52주 신고가 대비·이평, `inputs.marketCap`). `idx_stock_scores_rank(trade_date, score DESC)`가 있어 점수 조건이 붙으면 1,610행 범위 스캔이고, 조건이 없어도 그날 17,919행으로 끝난다 — 실측 **0.18초**. **이 화면이 경쟁 서비스와 갈라지는 자리는 속도가 아니라 깔때기다**: 전체 → 시장 → 점수 자격 → 잴 수 있음 → 조건 통과. 보통의 스크리너는 마지막 숫자만 말하고, 그래서 0이 나오면 사용자가 **시장을 탓한다.** 실측(국내): 17,919 → 4,412 → 2,653 → 2,544 → 50이고 화면이 «109종목은 조건을 틀린 것이 아니라 그 값을 잴 수 없어 빠졌습니다»를 적는다. 0종목일 때는 가장 좁은 조건을 지목한다 — 실측에서 「심리·수급 60점 이상」을 더하자 잴 수 있음이 2,653 → 314로 떨어지며 «가장 좁은 조건은 심리·수급입니다 — 2,653종목 중 325종목에만 값이 있습니다»가 떴다. **랭킹과 같은 우주를 본다**(축 3개 이상 · 지수 추종 상품 제외) — 다른 우주를 보면 같은 종목이 랭킹에 없고 스크리너에만 있게 된다. 국내 4,412 → 2,653으로 40%가 빠지는 큰 단계라 조용히 줄이지 않고 깔때기에 자기 칸으로 냈다. **커버리지 숫자가 목업과 달랐다**: 목업은 전체 17,919이 분모였는데 실제 분모는 자격 통과 2,653이다. 그 안에서는 재무 91~99% · PBR 98% · 시가총액 100% · PER 60%로 훨씬 높고, 진짜 낮은 것은 **심리·수급 12%** 하나다. 커버리지를 API가 동적으로 내려주므로 화면 설계는 바뀌지 않았다. **화이트리스트가 안전장치다**(`model.ScreenFields`) — 필드 이름이 곧 JSON 경로이자 정렬 식이라 사용자 문자열을 그대로 쓰면 주입이고, 모르는 필드는 **조용히 버리지 않고 거절한다**(버리면 조건이 빠진 채 결과가 나오는데 화면은 걸린 줄 안다). **실측이 잡아 준 결함 둘**: ① **저장 왕복이 깨졌다** — 시가총액 「1조」의 조회용 환산값 1e12(원)을 그대로 저장하니 다시 불러 보낼 때 1e12에 또 1e12가 곱해져 결과가 언제나 0이 됐다. 환산을 실행 지점 한 곳으로 몰고 저장은 화면 단위 그대로 둔다(덤으로 저장된 JSON이 사람이 읽을 수 있는 숫자로 남는다). ② **점수 열이 두 번 나왔다** — 조건에 건 필드가 결과표의 열이 되는데 종합 점수는 전용 열이 이미 있어 「88 88」이 나란히 섰다. 배치 A를 고른 근거는 **실측 치수**였다: 1440px에서 `/analysis`의 목록 열은 320px이고 거기엔 이미 시장 탭·축 6개·기간 4개·성적 줄이 있다. 조건 편집기를 넣으면 결과가 화면 밖으로 밀려, 스크리너의 핵심 동작인 「조건을 조금 바꿔 결과 수를 본다」가 성립하지 않는다(목업 `docs/mockups/screener-v2-placement.html`에 320px 실물 크기 비교). **실측**: 1440px 조건 352px·결과 전폭 8열, 375px 깔때기 3칸·조건은 손잡이로 접힘, 가로 넘침 0. 저장 → 목록 → 클릭 불러오기 → 실행까지 왕복 확인(삼성전자우 155.10조 · PBR 0.36). go build·vet·test 통과(스크리너 시험 11) · fe tsc 0 · eslint 새 오류 0(sidebar의 `set-state-in-effect`는 커밋본 319줄에 이미 있던 것) · 배포됨 · 미커밋. ⚠️ 시험용으로 Git Bash `curl`로 넣은 저장 조건 3건의 이름이 깨져 있었는데 **셸 인코딩 문제이지 앱 문제가 아니다** — 브라우저에서 저장한 「저평가 대형주」는 정상이고, 깨진 행은 지웠다. **남은 단계**: 5(Bull/Base/Bear 목표가) · 7(리포트 성과 집계) · 8(신뢰도 표기 전역화) · 9(통합 흐름).

- **2026-09-19 (목표가 시나리오 — 한 숫자를 찍지 않기로 한 이유를 숫자로 댄다)** — V2 Phase 5, BE+FE. `GET /stocks/{symbol}/scenario`가 약세·중립·강세 셋과 **그 방법의 실측 성적**을 함께 낸다. 밸류에이션 탭 맨 위(사용자 결정 A), 90일 고정 지평. **출발점은 제약이었다**: 이 앱에 증권사 목표가가 안 들어오고 사용자 목표가는 2건·스탬프 3건이라 「과거 예측 오차」를 집계할 원본이 없다. 그래서 시나리오를 우리 데이터로 도출하고 그 방법 자체를 백테스트했다 — Phase 2에서 랭킹에 한 것과 같다. **PER 기반을 먼저 재보고 접었다**: 앱의 PER은 연간 순이익 기준인데 TTM으로 다시 만들면 종목 화면과 어긋나고(실측 화승알앤에이 1.58 vs 앱 3.05), 과거 PER은 30거래일만 남아 90일 지평을 맞춰 볼 수 없다. 가격 이력은 21개월치라 **선행 편향 없이** 만들고 검증할 수 있어 그쪽으로 갔다. 세 후보 실측(국내 250~300종목 × 6시점 × 90일): 변동성 z=1.28이 **적중 72.3%에 폭 30%**, z=1.65가 82.6%에 40%, 52주 고저가 **60.6%에 54%** — 52주 범위는 **더 넓은데 덜 맞는다**. 종목별 편차도 극단적이어서 삼성전자의 52주 저점은 현재가의 −73%라 약세 시나리오로 쓸 수 없었다. Base는 **현재가**다: 200일 이평으로 두면 평균 오차가 22.9% → 29.0%로 **더 나빠진다** — 장기 평균 회귀라는 그럴듯한 가정이 실제로는 더 틀렸다. **점수로 방향을 기울이지 않았다** — Phase 2에서 종합 점수 상위 바구니가 KOSPI를 8.89%p 밑돌았으므로, 그 점수로 목표가를 밀면 이미 틀렸다고 측정해 둔 것을 예측에 다시 넣는 일이 된다. **배포본 실측**(표본 1,444 = 250종목 × 6시점): 적중 **70.1%** · 중립 평균 오차 **24.0%** · 아래 이탈 22.6% vs 위 이탈 7.3%로 **하방이 3.1배**다. 약세 시나리오를 그대로 「최악」으로 읽으면 안 된다는 뜻이라 화면이 그 문장을 적는다. 목업의 72.3%와 배포본 70.1%가 다른 것은 시점 격자가 달라서이고(목업은 고정 월말, 배포본은 최신일 −90일에서 30일 간격), 화면이 표본·시점·기준일을 함께 적으므로 스스로를 설명한다. **내재 PER이 숫자에 뜻을 준다**: 「418,728원」은 아무 말도 안 하지만 「그 가격이면 PER 63.4배」는 판단할 수 있다. 가격에 비례해 늘린 값이라 이익 전망을 지어내지 않고 아래 지표표(PER 39.3배)와 어긋나지 않는다. PER 보유 종목이 60%뿐이라 상세 보기에만 뒀다. **시험이 지키는 것**: 로그수익률이라 가격 수준이 변동성을 못 바꾸고 밴드 하단이 0 아래로 못 간다 · 각 시점의 변동성은 그 시점 **이전**으로만 잰다(잠잠하다 폭등하는 종목으로 시험 — 미래를 보면 적중, 안 보면 위로 이탈) · 이력이 모자란 표본은 0으로 세지 않고 **아예 세지 않는다**. **실측**: 1440px·375px 모두 가로 넘침 0, 가격 축 마커가 4%/39.3%/96%로 비율 유지(일차 변환이라 간격 비가 보존된다), 밸류에이션 탭에서 카드 바로 아래 PER 39.3배 지표가 이어진다. 첫 호출 0.81초·캐시 0.05초(6시간). go build·vet 통과 · 시나리오 시험 6 · fe tsc 0 · eslint 새 오류 0(workspace-client의 `set-state-in-effect` 3건은 커밋본 233·244줄에 이미 있던 것) · 배포됨 · 미커밋. **남은 단계**: 7(리포트 성과 집계) · 8(신뢰도 표기 전역화) · 9(통합 흐름).

- **2026-09-19 후속2 (신뢰도 표기 — 조사해 보니 절반은 이미 돼 있었다)** — V2 Phase 8, 범위 「가」(핵심 경로만, 사용자 결정). **FE만, BE 변경 0.** 서버가 이미 `dataSource`·`periodEnd`·`fetchedAt`·`coverageTier`·내부자 `source`를 실어 보내고 있었고, 화면이 그걸 안 쓰고 있었을 뿐이다. **먼저 센 것**: 화면 코드에 「—」가 210곳, 오류를 조용히 빈 값으로 바꾸는 자리가 98곳(`.catch(() => …)`). 그리고 `earnings_events` 19,159건에서 「—」 하나가 최소 네 가지 뜻이었다 — 아직 발표 전 3,601(18.8%) · 기록 없음 4,189(21.9%) · 비교할 예상 없음 134 · 날짜조차 미확정 4,905(25.6%). **그런데 조사해 보니 절반은 이미 규약을 지키고 있었다**: 실적 화면은 `DateStatusBadge`로 확정·예정·추정을 이미 다른 배지로 그리고 있었고(그 파일 주석에 「절대 같은 모양으로 그리지 않는다」고 적혀 있다), 점수는 Phase 1의 `ScoreProvenance`가 이미 있다. 둘 다 손대지 않았다. ⚠️ **목업에 그렸던 「일별 시세 · 휴장」 전후는 일어나지 않는 상황이었다** — `daily-close-tab.tsx`는 `pageRows.map`으로 **받은 행만** 그려서 날짜 축에 빈칸이 안 생긴다. 「휴장」을 말할 자리가 없어 만들지 않았다(UI 전제를 코드로 확인하기 전에 목업에 넣은 것이 이번에도 틀렸다). **실제로 빈 곳은 셋**이었다: ① 재무 종합 지표 카드에 커버리지 등급만 있고 **기준·출처가 없었다** — 커버리지는 「얼마나 채워졌나」이고 이것은 「언제 것이고 어디서 왔나」라 다른 사실이다(실측 「기준 25년 연간 · 출처 DART」). 2026-09-03에 사용자가 뺀 각주를 되살린 것이 아니라는 점이 중요하다 — 그것은 모든 종목·모든 방문에 똑같이 붙는 산문이었고, 이것은 종목마다 다른 사실 두 개를 배지 줄에 얹은 것이다. ② 내부자 탭은 `source`가 응답에 실려 오는데 화면 언급이 **0건**이었다(실측: 국내 「출처 DART」, 해외 「출처 EDGAR」). ③ 재무제표 차트가 `catch(() => setHistory(null))`로 **실패를 「없음」으로 삼키고 있었다** — 우리가 고장 난 경우와 공시가 아직 없는 경우가 화면에서 똑같았고, 사용자는 전자를 후자로 읽고 기다렸다. 오류 상태를 따로 들고 「다시」를 단다. **「—」 자체는 건드리지 않았다** (2026-09-17 사용자 결정: 「데이터 없음과 `-`의 의미가 같다」). 가장 흔한 상태에 장식을 붙이면 표가 표식으로 덮여 정작 값이 안 읽힌다 — 그래서 표기는 **다섯 개를 넘기지 않는다**(「—」 그대로 · 발표 예정일 · 휴장 · 다시 · 추정). 새 부품 `lib/data-reliability.ts`와 `components/ui/provenance-stamp.tsx`(`ProvenanceStamp`·`RetryMark`)를 남겨, 앞으로 새 화면은 이걸 쓴다. 도장은 **카드마다 하나**다 — 값마다 달면 같은 문구가 표 전체에 반복된다. **실측**: 1440px·375px 모두 가로 넘침 0, 배지 줄 오른쪽 끝에 도장이 붙는다. 재시도는 `window.fetch`를 막아 실패를 만든 뒤 확인했다 — 「불러오지 못했습니다 · — · 다시」가 뜨고, fetch를 되돌리고 「다시」를 누르니 차트가 복구됐다. fe tsc 0 · eslint 새 오류 0(insider-tab의 `set-state-in-effect` 2건은 커밋본 24·25줄에 이미 있던 것) · 배포됨 · 미커밋. **Phase 7은 11월로 미뤘다**(사용자 결정) — 리포트 1,647건 중 판정된 것이 1건뿐이고 나머지는 전부 90일 지평에 최초 발행이 8/10이라 **첫 만기가 11-08**이다. 배치가 고장 난 것이 아니라 시간이 안 된 것이고, 진행 중 1,561건에는 목표가가 **0건**이라 목표가 오차도 지금은 만들 수 없다. 배치는 A(목록 위 접힌 줄)로 정해 두었다. **남은 단계**: 9(통합 흐름) · 7(11월 이후).

- **2026-09-20 (통합 흐름 — 화면은 다 있었고, 잇는 길이 없었다)** — V2 Phase 9, 범위 「가」(사용자 결정). BE+FE. **V2의 마지막 단계**이고 Phase 7만 11월로 남았다. **조사부터가 결론이었다**: 발견→근거→판단→반영→추적 다섯 단계 화면은 Phase 1~6으로 전부 만들어져 있었고, 잇는 부품 `analysisSymbolHref`도 이미 있었다. 문제는 그 부품이 **메모장·관심종목·스탬프·리포트 넷에서만** 쓰이고 목표가·보유·스크리너는 안 쓴다는 것이었다. **가장 큰 구멍은 종목 화면**이었다 — 사용자에 대해 아는 것이 `quote.isHeld` **한 비트뿐**이라, 내가 관심에 담았는지·스탬프를 찍었는지·목표가를 적었는지·리포트를 썼는지·보유 이유 점검이 뭐라 했는지를 전혀 몰랐고, **이미 보유 중인데도 「보유종목 등록」을 똑같이 내밀고 있었다.** 그래서 `GET /user/symbols/{symbol}/context` 하나와 헤더 바로 아래 「내 기록」 한 줄을 만들었다. 칩 하나가 문 하나이고, 헤더(이 종목이 무엇인가)와 탭 사이에 둔다 — 탭 아래로 내리면 탭을 옮길 때마다 줄이 사라졌다 나타난다. **조회는 한 번**이다: 다섯 표(`user_holdings`·`watchlist_items`·`stamps`·`user_price_targets`·`research_calls`)를 따로 물으면 종목을 열 때마다 왕복이 다섯인데, 전부 `(user_id, symbol)` 단일 조회라 스칼라 서브쿼리로 묶었다. 실측 **0.054~0.087초**(보유 종목은 점검 판정 포함). **판정은 다시 계산하지 않고 빌려 쓴다** — `HoldingsDiagnosisService`를 부른다. 두 벌로 만들면 언젠가 갈라지고, 그때 사용자는 보유 화면에서 「약화」를 보고 종목 화면에서 「유지」를 본다. 어느 쪽을 믿을지 알 수 없는 상태가 가장 나쁘다. **보유 중일 때만** 부른다. 수량 0은 **보유가 아니다** — 전량 매도해도 `user_holdings` 행이 남을 수 있어서, 행의 존재만 보면 판 종목에 「보유 중」이 붙는다. ⚠️ **내가 앞 단계에서 빠뜨린 링크 둘도 채웠다**: ① **보유 이유 점검 카드가 클릭 불가**였다 — 「카카오 약화 −46」을 보고도 그 근거로 갈 수 없었고, 보유 **목록**은 되는데 Phase 6에서 만든 내 카드만 빠져 있었다. 추적(5)에서 근거(2)로 되돌아가는 유일한 자리라 흐름이 거기서 끊겨 있었다. ② 목표가 화면에 종목 링크가 **0건**이었다. **행동 버튼이 아니라 상태 표시로 설계했다** — 2026-09-03에 헤더 액션을 여섯에서 셋으로 줄인 결정이 있어 같은 영역이 붐비는 것에 주의했고, 이 줄은 누르면 그 기록이 있는 곳으로 갈 뿐 새 창을 열거나 무언가를 만들지 않는다. **없는 것도 남긴다**(관심 미등록 = 점선 칩) — 「아직 안 했다」도 흐름의 한 칸이고 다음에 할 일을 가리킨다. 다만 전부 비면 줄 자체를 안 그린다: 점선 칩 여섯 개는 안내가 아니라 잡음이다. 비로그인이면 아예 부르지 않는다(응답이 전부 「내」 기록이라 401이 정상). 칩 순서가 곧 우선순위다 — **확인이 필요한 「약화」가 맨 앞**이고, 좁은 화면에서 가로로 흐르므로 스크롤 없이 먼저 보인다. 색은 **약화만 경고색**이다(강화·유지까지 칠하면 줄이 알록달록해져 정작 볼 것이 안 보인다). **실측**: 1440px·375px 모두 가로 넘침 0, 카카오 화면에 「내 기록 · 점검 약화 근거 2 › · 보유 20주 @40,700 › · 관심 미등록」이 헤더와 탭 사이에 선다. 보유 화면의 점검 카드 5종목이 전부 `/analysis?symbol=…`로 연결된다. go build·vet 통과 · 시험 +2 · fe tsc 0 · eslint 새 오류 0(workspace-client의 `set-state-in-effect` 3건은 기존) · 배포됨 · 미커밋.

- **2026-09-20 (번역 전환 4일 점검 · 배치 재처리의 구멍 둘)** — 09-16 전환분은 4일째 문제없다: 번역기 재시작 0·OOM 0, 미번역 해외 기사 하루 1~13건(전부 상한 도달분 = 영어 유입의 0.7%), 해외 브리핑 매시간 생성, Azure는 전환 이후 **0자**(월 1,559,454에서 정지). 숨김 게이트를 유지한 결정도 문제를 안 냈다 — 번역이 유입을 따라잡아서다. 점검하다 **번역과 무관한 구멍 둘**을 찾아 고쳤다. **① 재시작에 잘린 실행이 재처리에서 영구히 빠졌다 — 한 달 69건 · 8개 잡 · 20일.** 배포·재기동으로 죽은 실행을 `MarkStaleRunsFailed`가 error로 닫으면 판정이 `failed`가 되고 `batch_catchup.go`가 건너뛴다. 제외 규칙 자체는 옳지만(「돌았는데 깨졌다」는 사람 몫) 잘린 실행은 **「못 돌았다」**이고 원인도 이미 안다 — 실측 09-08~20 실패 18건 중 11건이 이 경우. 원장에 기계가 읽을 표식(`result.interrupted`)을 남기고 `failed`라도 그 표식이 있으면 받는다(허용 목록은 그대로 — 예외가 답하는 것은 「왜 실패했는가」뿐이다). 한국어 문구를 되읽지 않는다: 문구는 화면 몫. **② 주말마다 국내 종가 거짓 경보** — 09-19(토) 67회 · 09-20(일)도. 15:50 슬롯은 잡 자신이 KRX 달력으로 거르는데(`daily_close.go`의 IsTradingDay) 기대만 매일이었다(실측 2주: 평일 1회 · 주말 0회). 기대에 `schedule: ScheduleKRX`를 달았다. 해외 종가(05:20 KST)는 손대지 않았다 — `schedule`을 달면 시각이 뉴욕 현지로 해석돼 05:20 ET가 되고 이 잡은 KST 벽시계에 매달려 있다(「달력은 미국, 시계는 KST」를 적을 칸이 없다). 시험 +5(재처리 4 · 기대 1), BE 전체 통과·재빌드·13/13 헬시. **미결**: 상류 점검(5xx)으로 실패한 잡은 여전히 하루를 잃는다 — 09-20 01:00·01:30 토스 점검에 걸린 `toss_universe`·`warnings_sync`가 19시간째 열린 critical 사건이었고, 다시 큐에 넣는 것은 DB 쓰기 권한에 막혀 못 했다(관리자 「배치 관리」에서 수동 실행하면 풀린다). 근본 수정은 `toss.IsUpstreamUnavailable`(5xx·code=maintenance)을 만들어 ①과 같은 표식을 붙이는 것 — 단 한도 소진(DART)은 같은 날 재시도가 무의미하므로 그 표식에서 빼야 한다. 미커밋

- **2026-09-20 후속 (SSR/CSR 점검 — 「갱신 중…」의 정체는 실패가 아니라 구조였다)** — 사용자 요청 10항목, **FE만, BE 변경 0**. **먼저 잰 것이 결론의 절반**이었다: 공개 API 여덟 개가 전부 비로그인 200(2.9~14.8ms, 가장 느린 `/earnings/highlights` 758ms), CORS 문제 없음(nginx 동일 origin), **hydration 불일치 경고 0건**. 즉 **실패한 것이 하나도 없었고** 홈과 `/analysis`가 순수 CSR이었을 뿐이다 — 서버 HTML에 「오늘의 시장 **갱신 중…** / AI 추천 랭킹 TOP / 거래대금 상위」처럼 구획 이름만 있고 숫자도 종목명도 **0자**였다(Googlebot UA 실측). `/news/domestic`은 이미 SSR이었고(기사 100건, `news-api.server.ts`) 그 패턴을 홈·`/analysis`로 넓혔다(`lib/home-api.server.ts`, 쿠키 없이 공개 데이터만 · `catch`로 빈 씨앗 · 재검증 30/60초). **`/calendar`의 빈 HTML도 예외가 아니었다** — 런타임 오류가 아니라 `redirect()`를 부르는 정적 라우트라 200 + 빈 셸 + flight 안의 `NEXT_REDIRECT`였다(실측 28,848바이트에 읽을 글자 0자). ⚠️ **`force-dynamic`으로는 안 고쳐진다 — 실제로 해 보고 알았다**: `(main)` 레이아웃의 네비 셸이 페이지가 던지기 전에 흘러나가 헤더가 이미 나간 뒤라 상태코드를 못 바꾼다. `next.config.ts`의 `redirects()`로 옮겨 **308**이 됐다(라우팅 단계라 레이아웃이 시작조차 안 한다). **진짜 결함 셋**: ① `use-market-indices`의 `catch {}`가 실패를 삼켜 `updatedAt`이 null로 남고 화면이 「갱신 중…」 + 스켈레톤을 **영원히** 그렸다 — 새로고침 말고 빠져나갈 길이 없었다. 같은 파일의 `isPageVisible()` 가드가 **첫 적재까지** 막고 있어서 백그라운드 탭으로 연 홈은 누를 때까지 비어 있었다(브라우저 패널을 hidden으로 두고 재현 → 지수 0종, visibility를 켜자 13종). 가드의 본뜻은 「안 보는 탭에 **폴링**을 돌리지 않는다」이지 「내용을 만들지 않는다」가 아니다 — 회차 갱신만 막게 고쳤다. ② **타임아웃이 사실상 없었다** — 앱 전체에서 상한이 걸린 fetch가 `fetchMarketIndicesClient` 하나뿐이었고 `apiRequest`·`serverApiRequest`를 포함한 나머지는 무제한이었다. `inflightRefresh`(모두가 함께 기다리는 promise 하나)에도 상한이 없어 그 요청이 안 끝나면 401을 받은 화면 전부가 같이 멈출 수 있었다. `lib/fetch-timeout.ts`(브라우저 8초 · SSR 3초 · 2s→4s 재시도, 4xx·429는 재시도 안 함)로 모든 경로에 걸었다. ③ `dashboard-overview`의 `Block`에 `error`가 없어 **첫 적재 실패가 `empty`로** 떨어졌다 — 「아직 산출된 점수가 없습니다」는 사실이 아니었고, 사용자는 「없다」를 읽고 기다리기를 그만둔다. **반대 방향 오류도 찾았다**: 「거래대금 상위」는 장이 닫혀 빈 목록이 와도 「불러오지 못했습니다」라고 말했다 — 실측 일요일 `GET /api/v1/rankings` → **200 · 60바이트 · 0건**, 즉 주말과 마감 뒤 내내 **성공한 요청이 실패로 표시**되고 있었다. `useMarketClosed`(서버 `marketSession` 판정을 읽기만 한다)로 「장이 열리면 표시됩니다 · 월 09:00」과 갈랐다. 기준시각·stale은 `lib/data-freshness.ts` + `useFreshness` + `StaleBadge`로 넣었고 **두 「기준」을 섞지 않는다** — 갱신 시각(늙을 수 있다, 주기의 3배)과 거래일(주말에 금요일이어도 정상, 절대 낡음이라 말하지 않는다). 후자에 벽시계를 들이대는 것이 09-19에 국내 종가가 하루 67회 거짓 경보를 낸 바로 그 함정이다. 카드 기준시각은 제목 옆이 아니라 **바닥**에 뒀다 — 2026-09-05에 그 자리의 배치일을 뺀 결정(제목이 두 줄로 접혀 카드가 12px 높아졌다)을 되돌리지 않기 위해 `SectionCard`에 `footer` 슬롯을 더했다. ⚠️ **고치는 중에 두 번 틀렸고 둘 다 실측이 잡았다.** ⓐ 서버 fetch에 중단 신호를 달았더니 **냉시동 첫 렌더가 세 번 터졌다**(`controller[kState].transformAlgorithm is not a function`) — Next 데이터 캐시가 응답 본문을 `TransformStream`으로 가르는 중에 신호가 얹히면 그 갈래가 깨진다. 요청 수와 오류 수가 셋으로 맞았고, 신호를 떼자 사라졌다. 서버에서는 **끊지 않고 기다리기만** 한다(`fetchWithDeadline`) — 필요한 것은 TTFB 상한이지 취소가 아니고, 남은 요청은 백그라운드에서 캐시를 채운다. `fetchWithTimeout`이 `typeof window === "undefined"`면 그쪽으로 갈라져 공유 모듈(`news-api.ts` 등)도 자동으로 안전하다. **처음 진단은 「재시작 로그에 3건」이었는데 `docker restart`가 이전 인스턴스 로그를 보존해서 벌어진 착시였다** — `--force-recreate`로 정말 차가운 캐시를 만들어야 검증이 된다. ⓑ 줄 수 상수를 `"use client"` 모듈에서 서버 컴포넌트로 import 했더니 **`undefined`**가 왔다(Next가 서버 그래프에서 클라이언트 참조로 바꿔친다). `String(undefined)`가 쿼리에 실려 BE가 그 값을 무시하고 **기본 20건**을 줬고, 홈 카드가 일곱 줄이 아니라 **스무 줄**로 서서 2026-09-05의 카드 높이 결정이 그대로 깨졌다 — **tsc도 eslint도 통과하고 요청만 조용히 틀렸다.** 평범한 모듈(`lib/dashboard-rows.ts`)로 옮기고 런타임 가드(`Number.isInteger`)를 달았다. **실측(고친 뒤)**: `/` TTFB 중앙값 11.8→16.2ms · 사람이 읽는 글자 **0→2,608자**, `/analysis` 12.4→25.5ms · **0→5,401자**, `/calendar` 200 빈 셸→**308**(4.4ms), `/news/domestic` 26.6ms 그대로(이미 SSR이었다). 냉시동 오류 0. fe tsc 0 · eslint 새 오류 0 · 단위시험 334→359(+25: fetch-timeout 13 · data-freshness 12) · **새 E2E 12건 전부 통과**(`e2e/tests/ui/ssr-hydration.spec.ts`: JS 없는 HTML · hydration 불일치 · 직접 진입↔내부 이동 일치 · 모바일 폭 · 로그인/비로그인 일치 · 요청 실패 시 「다시 시도」). 시험을 쓰다가도 두 번 배웠다 — 가짜 fetch가 **이미 취소된 신호를 즉시 거절하지 않아** 자기 테스트를 5초 세웠고(진짜 fetch는 동기적으로 거절한다), vitest가 `environment: "node"`라 `window`를 안 세우면 중단 검사들이 **다른 함수를 검사하며 통과**했다. ⚠️ **기존 UI E2E 10건은 이 작업 전부터 실패하고 있다** — 전부 「비로그인 안내 화면」을 기대하는데 커밋된 `useAutoSample`(09-14 결정: 「비로그인에게는 묻지 않고 예시를 켠다」)이 그 화면을 건너뛴다. `auth-required-shell.tsx`는 이번에 손대지 않았고 내 변경 경로에도 없다. 시험이 낡은 것이라 **고치지 않았다 — 사용자 판단 몫.** ⚠️ **남은 것 둘**: `fe/src/app/(main)/calendar/page.tsx`가 이제 도달 불가능한 죽은 파일인데 이 세션에 삭제 권한이 없어 남겼다. 해외장만 열린 시간대(대략 23:30~06:00 KST)에는 홈 AI 랭킹이 토글 확정 전까지 국내 목록을 잠깐 보여준다 — 없애려면 서버가 `/market/clocks`를 읽어 열린 시장을 정해야 하고 그건 `useOpenMarket`의 「왜 null로 시작하나」 설계를 바꾸는 일이라 남겨 뒀다. 배포됨 · 미커밋.

- **2026-09-22 (`/market`·`/screener` SSR 선반입 — 붙여넣은 QA 기획서를 먼저 재고 두 화면만 골랐다)** — 사용자가 외부 QA/개선 기획서(P0 10건 · P1 10건 · 76절)를 통째로 붙였다. **먼저 잰 것이 결론의 전부였다**: 그 문서는 공개 사이트 크롤링으로 쓰여 현재 코드와 크게 어긋난다. 문서가 「만들라」는 것 대부분이 이미 있다 — 점수 breakdown·변화 이유·가중치 공개(P0-06·07, P1-02)는 `/score/explain` + `score-explain-panel`·`score-movers-card`·`score-radar-chart`로 돌고 `ScoreView`가 `breakdown`·`coverage`·`computedAt`·`tradeDate`·`weights`·`peerPct`를 이미 싣는다. 「AI 기준일 ≠ 현재가 기준일」(P0-02·03)은 `lib/data-freshness.ts`의 ⓐ(갱신 시각)/ⓑ(거래일) 구분이 바로 그것이고, 0/null/오류 구분(P0-04·09)은 `LoadErrorState`·`RetryMark`·`MenuEmptyState`, 출처 표기는 `data-reliability.ts`+`ProvenanceStamp`가 한다. 스크리너 저장·Stock Workspace·Market Radar(P1-01·07·09)도 있다. **문서 §2의 `dataStatus`/`delaySeconds`/`sessionStatus` 전역 도입은 기각했다** — 이 앱은 같은 문제를 ⓐ/ⓑ 어휘로 풀었고 그 어휘는 주말 거짓 경보(09-19 하루 67회)를 피하려고 일부러 만든 것이라, 새 필드를 얹으면 두 체계가 공존한다. §51 Confidence도 `coverage`+`CoverageFloor`/`ShrinkFull`이 이미 그 역할이다. **진짜로 재현된 결함은 하나**였다: 09-20에 홈·`/analysis`가 SSR로 넘어갈 때 남은 두 화면. 운영 HTML의 사람이 읽는 글자 `/market` **450자**·`/screener` **473자**, 양쪽 다 종목명 0개 → 고친 뒤 **3,033자 / 3,026자**(SK하이닉스·삼성전자·리튬포어스·파미셀), TTFB 17.9/16.7ms. `/market`은 `page.tsx`가 통째로 `"use client"`라 본문을 `market-radar-page-client.tsx`로 **그대로** 옮기고 페이지를 서버 컴포넌트로 되돌렸다(`lib/market-api.server.ts`, 쿠키 없이 공개 `/rankings`, `fetchWithDeadline`, 재검증 60초). **기본 손잡이일 때만 심는다** — 실측으로 갈렸다: `/market`은 HTML에 `rankedAt`, `/market?market=US`는 `"seed":null`·79KB. `/screener`는 조건 목록(GET)과 **기본 조건 결과(POST)** 둘 다 심었다 — 조건 목록만으로는 HTML에 지표 이름만 들어가고 종목명은 결과 표에 있다. ⚠️ **POST는 Next 데이터 캐시가 안 받는다**(16.2.10 `fetch` 레퍼런스의 캐시·메모이제이션이 전부 GET 기준이고 `cacheComponents` 미사용이라 이전 모델) → 기본 조건이 고정값이므로 `unstable_cache`(300초)로 감싸고 안쪽 fetch는 `cache:"no-store"`로 이중 캐시를 막았다. 기본 조건 리터럴이 `rules`·`ranQuery` 두 곳에 있었고 서버가 세 번째가 되므로 `defaultScreenRules()` 한 곳으로 모았다 — 어긋나면 「score ≥ 70」이라 적힌 칸 옆에서 다른 조건의 결과가 보이고 그 차이는 화면에 안 드러난다. **함정 둘을 미리 막았다**: ① `useMarketRankings`의 초기화 이펙트가 마운트에도 도는데 `setData(null)`·`setLoading(true)`를 그대로 두면 서버가 그린 100행이 하이드레이션 직후 사라졌다 돌아온다(= 선반입 전과 똑같이 「랭킹 로딩 중…」이 스쳐 간다) → `seedPendingRef`로 **첫 실행만** 건너뛰고 이후 손잡이 전환은 전과 같이 비운다(09-04 결정 유지). `prevRanksRef`는 선반입이 있어도 비운다 — 안 그러면 새로고침한 사람에게 비교할 앞이 없는 순위 화살표가 뜬다. ② hydration 안전을 **확인하고** 썼다: `watchlist-store`는 모듈 전역 빈 집합이고 localStorage를 안 읽어 서버·브라우저 첫 프레임이 둘 다 비어 있다(브라우저 실측 hydration 경고 0건). **검증**: fe tsc 0 · eslint 새 오류 0(`market-radar-page-client.tsx`의 React Compiler 경고 6건은 **원본에서 그대로 따라왔다** — git HEAD의 `market/page.tsx`를 따로 린트해 같은 6건임을 확인, 수정은 별건) · `next build` green(둘 다 ƒ dynamic) · 로컬 프로덕션 서버 브라우저 실측(랭킹 100행 · 스크리너 깔때기 2,653→183 · 마운트 뒤 `/screen/fields`·`POST /screen` 재요청 200 = 씨앗이 얼지 않음 · 1440px 스크린샷). BE 변경 0. ⚠️ **미배포**(`NGINX_ROUTES=app_routes.conf` 컨테이너 모드 → fe 이미지 재빌드 필요) · 미커밋. **남은 것**: `market/layout.tsx`가 아직 메타데이터를 들고 있다 — 원래 이유(페이지가 클라이언트 컴포넌트)는 사라졌지만 같은 값을 두 곳에서 내보내지 않으려고 뒀다.

- **2026-09-22 후속 (전수 점검 — 같은 결함이 셋 더 있었다 · 배포)** — 앞 작업을 끝내고 **짐작하지 말고 전부 세자**고 정해 전 라우트(공개 16 · 로그인 7)의 서버 HTML에서 사람이 읽는 글자 수를 쟀다(`<script>`/`<template>` 제거 후). **① `redirect()`를 부르는 페이지가 넷이었다 — 09-20이 `/calendar` 하나만 고쳤다.** 실측 `/ai-analysis`·`/financials`·`/disclosures`가 전부 **200 + 27자**(빈 셸). `/dashboard`만 307이었는데 운이 아니라 `src/proxy.ts`가 렌더 전에 가로채기 때문이다(다섯 번씩 찍어 상태코드가 안 흔들리는 것도 확인 — 경쟁 조건이 아니다). 셋을 `next.config.ts`의 `redirects()`로 옮겨 **308**. **조각(`#ai-analysis`·`#disclosures`)은 뗐다** — 그 id를 가진 요소가 `/analysis`에 **하나도 없다**(grep 0건), 아무 데도 안 데려다주는 조각이다. 도달 불가능해진 페이지 파일 넷(`calendar` 포함)을 **지웠다** — 09-20이 「삭제 권한이 없어 남겼다」고 적어 둔 그 파일들이다. **② 상한 없는 fetch가 둘 남아 있었다** — 09-20이 `lib/fetch-timeout.ts`로 앱 전체에 상한을 걸 때 `market-clocks.ts`와 `news-briefing-api.ts`가 빠졌다. 전자가 특히 아픈 자리다: `cachedGet`이 **모두가 함께 기다리는 promise 하나**를 들고 있어 이 요청이 안 끝나면 시장 세션을 묻는 카드가 전부 같이 멈춘다(09-20이 `inflightRefresh`에서 고친 것과 같은 모양). `client-error-report.ts`의 `void fetch`는 **일부러 남겼다** — `sendBeacon` 대체용 `keepalive` 전송이라 상한을 걸면 목적이 깨진다. ⚠️ **tsc가 지운 파일 때문에 터졌다**: `.next/dev/types/validator.ts`(예전 `next dev`가 남긴 생성물)가 삭제된 라우트를 계속 참조했다 → `.next/dev` 삭제 후 재빌드로 통과. 「tsc가 틀렸다면 코드가 아니라 `.next`가 낡은 것」의 반대 방향 사례. **배포**: `fe/scripts/deploy.ps1`(파이프로 감싸지 않고 그대로) → fe 이미지 재빌드 · 컨테이너 교체 · nginx 재생성. compose 의존으로 api도 함께 재생성됐다. 13개 컨테이너 healthy, `/health` ok, **Tesseract 5.3.0 확인**. 운영 실측 `/market` **3,033자** · `/screener` **3,026자**(둘 다 종목명 포함), 옛 경로 넷 전부 308/307. ⚠️ **배포 중에 별건 결함을 하나 찾았고 고치지 않았다**: fe 냉시동 첫 렌더에서 `⨯ TypeError: controller[kState].transformAlgorithm is not a function`이 난다. 09-20이 「`docker restart`가 옛 로그를 보존해 생긴 착시」라고 결론지은 그 오류인데 **착시가 아니라 실재한다** — 새로 만든 컨테이너의 부팅 5초 뒤 로그에 있다. **내 변경 탓이 아닌 것도 실측으로 갈랐다**: 냉시동 직후 `/market` → 오류 0, `/` → 즉시 오류, `/screener` → 오류 0. 즉 **`/`에서만** 난다. 같은 `next: { revalidate }` 패턴을 쓰는 새 페이지 둘은 멀쩡하고, 차이는 홈이 **`Promise.all`로 revalidate fetch 둘을 동시에** 부른다는 것이다(`app/(main)/page.tsx`) — 냉시동으로 데이터 캐시가 빈 상태의 동시 쓰기가 터지는 것으로 보인다. 영향은 **재기동 뒤 첫 요청 한 번**이고 각 함수가 `catch`로 빈 씨앗을 주므로 화면은 선다(운영 `/` 200 · 1,631자). 순차로 바꾸면 TTFB에 둘의 합이 붙어 그 파일이 명시적으로 기각한 선택이 되므로 **별도 측정 사이클이 필요하다.** **남은 큰 것 둘(손대지 않음)**: ⓐ `/stocks/[symbol]`이 570자로 거의 비었지만 이건 버그가 아니라 **기록된 보류**다(그 파일 주석: 「서버에서 종목 내용을 그리게 되면 그때 색인 대상으로 올린다」) — 종목 수천 개를 색인에 올리는 SEO 정책 판단이 먼저다. ⓑ `/earnings/domestic`·`/earnings/us`가 CSR(1,138 / 1,195자, 종목명 0)인데 `/market`처럼 못 옮긴다: 보기 단위가 **localStorage**에 있고(`useEarningsRangeMode`) 날짜가 「데이터 있는 날」로 자동 이동하는데 서버는 그 둘을 알 수 없어 씨앗이 틀리면 hydration이 어긋난다. 미커밋.
- **2026-09-23 후속 (경쟁 분석)** — FnGuide·증권사 MTS·Seeking Alpha·TipRanks·TradingView 대비 기능 비교와 3/6/12개월 우선순위를 `docs/COMPETITIVE_ANALYSIS.md`로 정리. 핵심 축 다섯(LLM 범위·전 종목 커버리지·설명 가능성·변화 감지·개인화 포트폴리오)을 가치·난이도·차별화로 평가. 설명 가능성이 가성비 최고(가치 4·난이도 2·차별화 5), 3개월 첫 항목은 매매 이력 재시작 삭제 결함. 코드 변경 0.
- **2026-09-23 후속2 (로드맵 3개월 착수)** — ① 매매 이력 재시작 삭제 결함 수정(기동 경로에서 정리 제거 · CLI `-dry-run`도 지우던 것 함께) ② `stock_score_history`(077) 축 점수 장기 보관 — 30거래일 537,387행 백필, 보유 이유 점검이 30거래일 밖을 이어 읽음, 심리 축은 `UpdateAxes`가 이력도 갱신 ③ 가치 축 격일 소멸의 원인 = 점수 배치가 **당일 종가만** 붙임(00:25 배치 vs 05:20 미국 종가) → 최근 10일 내 최신 종가로, 미국 가치 축 0→3,728종목 ④ `stock_sectors`(078) + `sector_sync`(02:30) — KSIC·SIC → 11섹터, 우선주는 보통주로, 지주회사는 이름으로 금융만 가르고 나머지 비움. 첫 회차 국내 722 · 미국 1,786. 피어그룹 교체는 사용자 결정 대기. 보고서의 「포트폴리오 분석 없음」 정정.
- **2026-09-23 후속3 (로드맵 3개월 나머지)** — 사용자 결정 다섯(재무 3축만 섹터 피어 · 로컬 감성 모델 KR-FinBert-SC+FinBERT · 새 「오늘의 변화」 메뉴 · 도넛 섹터 탭). 섹터 피어 커버리지 80%에서 자동으로 켜짐(국내 82.7 · 미국 90.6). `be/sentiment` 컨테이너 + 기사 라벨 22,489건 → 심리 축 9→7,341종목, 요약 경로 `ApplySentiment` 삭제. `/changes` — 7종 규칙 감지, 축 유무·비교 집단·**산출 방식 판(`scoreMethodVersion`)** 변화에 의한 점수 급변 제외, 「설명 불가」 표시. 실측으로 ETN 노출·중요도 쏠림·봉투 이중 해제 결함을 잡음. 신규 시험: 배선(Migrate 호출)·섹터 피어·사건 분류·변화 규칙·FE 2.
- **2026-09-23 후속4 (6개월 항목 셋)** — 결정: 수급 축 보류 · 알림은 앱 안 배지부터 · 내 가중치는 AI 랭킹에 · 성적표는 지금 공개. 사이드바 「변화」 배지 + 홈 한 줄 띠(내 종목 변화, 종목 수 기준). 점수 성적표(스피어만 IC, 5거래일, 상위20 초과) — 겹치는 창으로 t 보정(추세 −2.8→−1.26), 국내 전 축 「의미 없음」. 「내 기준 점수」 — 저장 축 점수 재합산, 기본 가중치 = 저장 점수. 발견: 우선주가 상위에 선다(보통주 EPS로 가치 평가) — 결정 대기. 로그인 화면은 실측 못 함(비밀번호 입력 안 함).
- **2026-09-24 (6개월 항목 나머지 셋 + 우선주)** — 우선주를 랭킹 우주에서 제외(코드 끝자리 규칙, 실측 114/114), 재계산 뒤 상위에 우선주 0. 점수 설명 한 문장(FE 템플릿). 내 포트폴리오 vs 지수(TWR). 매수 이유 태그(별도 표 — 보유 전체 교체가 행을 지우므로) → 보유 이유 점검이 그 축 평균으로 판단. ③④는 로그인 화면이라 실측 못 함.
- **2026-09-24 후속 (1년 로드맵 일괄)** — 결정: 문장 스크리너는 규칙 파서(LLM 0) · 컨센서스는 미국 적재만 · 수급은 호출량 확인 뒤 진행 · 가중치는 제안 도구만. 수급: 종목별 매매동향은 별도 버킷 `STOCK_TRADING_TREND`(실측 한도 10/s) — `groups.go`가 `STOCK`으로 잘못 묶던 것 분리, 국내 보통주 2,641종목 60일 백필(158,165행 · 실패 0), 심리·수급 축에 20일 순매수/거래량 성분(미국 불변), 판 2026-09-24. 미국 EPS 추정치 변화 이력(첫 스냅샷 3,502). 관리자 가중치 제안(90일 전 거절). 스크리너 「문장으로 찾기」(못 알아들은 말 표시). 국내 ETF 점검(KRX 다음 영업일 공개 · 괴리·추적오차·유동성·규모 · 일반/채권·금리/레버리지 집단) + 종목 카드 + AI 랭킹 「국내 ETF」 탭. 보유 「위험」 카드(현재 비중 가정 변동성·MDD·베타·상관 쌍, 종가 배치 엔드포인트). BE·FE 재배포.
- **2026-09-24 후속2 (보류 항목)** — 결정: SEO 상위 1,000 · 종목 질문 칩 · 위험 카드 「바꿔 보기」 · 사용 지표 보류. SEO 잣대는 저장 시가총액 오류(ADR 부풀림 — 아카리 9,250억 달러)로 20일 거래대금으로 대체, 목록 기동 선채움+SWR(콜드 3.7초 > SSR 3초), 보이는 요약 한 줄 + 상위만 index·canonical, 사이트맵 1,000. 질문 칩 넷(새 `/stocks/{symbol}/qa`, 비교 종목은 같은 섹터 시총 근접순). 바꿔 보기(비중·섹터·위험 전→후). 곁가지: 점수 문장 조사 수정, 위험 카드가 시세 틱마다 재요청·재마운트하던 결함(키에 평가금액) 수정. 시가총액 오류는 별도 작업 칩.
- **2026-09-25 (시장 브리핑 자정 이후 규칙 문장)** — 원인: gemini-3.1-flash-lite가 매일 KST 22~03시 503(두 키 모두), 슬롯당 2회만 시도 + 승계가 3시간·서울 자정에서 끊겨 00시부터 규칙. 개선: 슬롯 안 15분 재시도(실패 시 행 안 씀) · 승계 날 경계를 개장 준비 시각으로(국내 08:00 KST · 미국 08:30 ET) · 국내 장 밖끼리는 3시간 창 없음 · 규칙/승계 뒤엔 조용한 슬롯도 다시 물음 · 브리핑 전용 503 대체 모델 gemini-3.5-flash-lite(2.5-flash-lite는 목록엔 있으나 404). 실측: 국내 01:12 21:00 문장 승계 → 01:14 AI 복구, 해외는 대체 모델도 503. go test 전체 통과, BE 재배포.
- **2026-09-25 후속 (브리핑 503 재시도 간격)** — 사용자 제안: 503이 한도를 안 올리면 계속 재시도. 우리 apibudget은 503을 세지 않음(Record는 <400만) 확인, Google 쪽은 문서에 없어 슬롯 안 재시도 15→5분 + 시장별 브리핑 날(개장 준비 시각 기준)당 60회 상한. 상한을 서울 자정으로 세면 503 구간 한가운데서 리셋되는 것을 시험이 잡아 briefingDay로 교체. go test 전체 통과, BE 재배포.
- **2026-09-25 후속2 (전체 느림 — 로그 분석)** — nginx·api 로그 경로별 p50/p95로 특정: 느린 것은 전부 캐시 만료/무효화 직후 동기 재구성(방문 간격 > 캐시 수명). 실적 레일 SWR(3초→3~5ms) · 뉴스 적재 직후 최근 본 canonical 목록 예열 · 해외 이름 검색 메모(없음 포함) + 이름 인덱스 내용 불변이면 세대 유지(해외 목록 재구성 536ms→~43ms) · 뉴스 합치기/dedupe를 쓰기 잠금 밖으로(해외 1,000건 780ms 동안 모든 목록 읽기가 섰다). 대시보드 최대 5ms, 보유 종목 뉴스 1.7~2.0초→~190ms. 배포 · Tesseract OK · 미커밋.
- **2026-09-25 후속3 (남은 것 전부)** — 경고·섹터 배치는 멈춤이 아니라 새벽 재배포로 죽고 running으로 남아 만회가 건너뜀 → 워커 기동 시 자기 잡의 running 행을 닫음(interrupted). Toss 토큰 발급 무기한 대기 상한 15초. 종목 첫 진입 시세·호가·체결 병렬(272~540→125~233ms). 스크리너 결과 캐시·조건 목록 SWR(267→4ms). 종목 뉴스까지 적재 직후 예열. restart.ps1 NativeCommandError 수정 · -ApiOnly · 네트워크 없이 뜬 api 자가 재생성(배포 중 약 2분 중단 발생·복구). 미커밋.
- **2026-09-25 성장 감시** — Growth Agent 스펙을 코드만의 별도 컨테이너 worker-growth로(`growth_audit` 07:30 · 사이트맵 크롤 + GSC/GA4/Bing 선택 + DB 활동 개수 → 규칙 분석 → 일간/주간 보고서, growth_* 표만 씀·시험으로 강제). GA4 태그(FE 미배포 — 개인정보처리방침 문안 대기). 첫 실행에서 e2e 계정 292개가 가입 KPI를 부풀린 것 · 공개 주소 간헐 정지(오리진 앞) 발견 · 종목 1,000페이지 내부 링크 0.
- **2026-09-25 성장 감시 후속** — 개인정보처리방침에 GA 문안(1·2·4·5항) · FE 재배포(GA page_view 실측) · 관리자 `/admin/growth` 보고서 화면(자체 Markdown 렌더러, innerHTML 없음).
- **2026-09-26 캐싱·인덱스 점검** — 서버 경로는 이미 빠름(p95 대부분 <40ms, Redis 94%). 배치 큐 폴러 31개를 프로세스당 하나로(유휴 SQL 31→4.9/초, 트랜잭션 7.75→0, 새 연결 1.27→0.02/초). 공개 API 46경로에 엣지 캐시 헤더 + 신원 조회 시 자동 private(IdentityProbe) — Cloudflare 캐시 규칙은 사용자 몫(docs/EDGE_CACHE.md). interpolateParams 기각(JSON에 _binary). BE+워커 배포 · 미커밋.
- **2026-09-26 재방문 개선** — 추천 랭킹 하한(SPAC·미국 우선주·30일 거래대금 5억원/$2M·시총 500억/$3억, `inputs.avgTradeValue`) · 가입 직후 `/welcome` 관심 종목 고르기 + `next` 복귀(열린 리다이렉트 수정) · 홈 맨 위 「내 종목」 카드(`GET /user/today`: 점수 변화·24h 뉴스·변화 한 줄 + 실시간 가격·실적 D-n). BE+FE 배포, 가입→고르기→홈 브라우저 실측.
- **2026-09-26 외부 방문자·크롤러 점검** — Googlebot UA로 공개 경로 SSR + 비로그인 브라우저 + API↔DB 대조. BE 배포: 비로그인 200 `/auth/session` · 닫힌 세션 시가·고가·저가를 확정 일봉으로(가짜 값·「—」 제거) · 순위 등락률을 종목 화면 기준가로 통일 · 순위 신규 상장 카탈로그 등록 · 사이트맵↔noindex 3시간 유예 · 뉴스 사건은 제목이 회사를 부를 때만. FE(미배포): 401 제거 · /news·/earnings HTTP 리다이렉트 · 홈 종가 기준일 · 종목 H1 하나 · 공개 화면 sr-only H1. 미커밋.
- **2026-09-26 외부 방문자 점검 후속** — FE 배포(401 제거·리다이렉트·H1 실측 확인) · 마감 중 정규장 종가를 확정 일봉으로 · 52주 범위를 실제 52주로(400봉을 훑던 것) · 종목 설명 160자·제목 45자 상한. 변화 감지 9/23·24 재생성은 사용자 몫(DB 삽입 권한 거부). 미커밋.
- 09-26 불필요 코드 정리: deadcode 함수 ~115개·internal/mock·일회성 cmd 5개·잡파일 제거, go test 통과, 미배포 (AGENT_STATE 참고)
- **2026-09-26 기존 결함 정리** — 브리핑 재배포 중복 LLM 생성 차단(seedFromStored) · 지수 「낡은 값」 오판(checkedAt) · 변화 요약 이중 요청 공유 · 승계 배지 「어제」 표기 · 죽은 띠 삭제. BE+FE 배포.
- **2026-09-26 후속 (엣지 캐시 확인)** — CF 규칙 켰다는데 API 전부 DYNAMIC(규칙 미매칭, 존 캐시는 동작) → EDGE_CACHE.md에 점검 순서, 사용자 확인 대기. 엣지 경로 5개 추가(/quotes/indices 등). 중복 인덱스 제거 마이그레이션은 권한 거부로 보류. API만 재배포.
- **2026-09-27 (엣지 캐시 재확인)** — CF 규칙 수정 후 재측정: MISS→HIT 확인, 개인화 경로는 BYPASS 유지. 사소한 것: CF가 Cache-Control max-age를 4시간으로 덮어씀(오리진은 0) — FE는 전부 cache:no-store라 지금은 무해, Browser Cache TTL을 Respect Existing Headers로 바꾸는 게 안전. 코드 변경 없음.
- **2026-09-27 (요청 1~5)** — 예측·메모장 무한 재적재·배치 관리 20초 수정 · 홈 카드 넘침·관심 종목 오늘 신호 줄 · 아이디 가입(닉네임 유니크, 중복 360 번호) · 커뮤니티 신설(088, 종목·기사 붙이기·이미지 압축·뱃지·신고) · AI: 질문 칩 10개 · 실적 관전 포인트(089) · 스크리너 업종/신호 필드·사전 후보(091) · 리포트 AI 검토(090), 기능별 LLM 버킷. BE+FE 배포.
- **2026-09-28 (요청 1~7)** — 관리자 아이디 jjmin321 · 공유 버튼(modal-history 경합) · 글/리포트 데이터 태그(092 content_mentions, @칩·@종목:지표·#카드, 편집기 자동완성·미리보기) · 종목 커뮤니티 탭 · 변화 칩 AND+개수 · 스크리너 가이드·기준 종목 채우기 · 지표 로테이션 정지·한도 덮어쓰기(2,098)·ADR 통화 혼합 PER 수정 · Finnhub/Naver 목표가·Reddit(ApeWisdom) 조사. BE+FE 배포.
- **2026-09-28 후속** — 외부 신호 093: Finnhub 해외 지표 보조(EDGAR 빈 칸만, 점수 |ext)·추천 추이, 네이버 국내 목표가·의견, ApeWisdom 레딧 언급(오늘의 변화 급증·과열). worker-filings 잡 3개, 밸류에이션 탭 애널리스트 카드, 스크리너/태그 필드 4개. BE+FE 배포.
- **2026-09-28 밤** — 수정 화면 태그 도우미 · 로그인 복귀 쿼리 유지 · XBRL 통화 혼합 근본 수정(TSM 영업마진 45.7%, 55종목 재적재) · 재무제표 탭 공시 통화 표기 · 레딧 서브레딧 적재·잡음 티커 · 레딧 탭/점수 제안서(결정 대기).
- **2026-09-29 새벽** — Reddit 커뮤니티 탭(094: ApeWisdom 종목 페이지 감성·키워드, Reddit 검색 RSS 인기 글+번역, 규칙 커뮤니티 점수, LLM 요약 04:20) · 심리 축 Reddit 성분(미국, 0.3, |reddit) · 종목 AI 요약 사실표 reddit · API 네트워크 이탈 장애 2h20m 복구 + 워치독 Repair-DetachedApi · slot 로그 소음 제거
- **2026-09-29 새벽 (2)** — 금융상품 비교 MVP: 095 fin_products·fin_companies·fin_watchlist, worker-filings `fin_products` 05:10(금감원 finlife Open API, 키 대기), /fin/products·/fin/watchlist API, FE 금융상품 그룹(예·적금 · 대출 · 계산기 + 상세), 세후 이자·총이자 순위, docs/FINANCE_PRODUCTS.md. 배포 중 Docker 엔진 멈춤 7분 장애 → docker desktop restart 복구.
- **2026-10-01 새벽** — 요청 1~5: 스크리너 닮은꼴(like 정렬·축 −15·칩, 무반응·보통 잘림 결함) · Reddit/LLM 사용량 검토 · 홈 「미국 개미 분위기」(/social/market-mood, 시장 글 _MARKET, AI 한 줄) · 글 태그 퀵룩 모달·옆 미리보기 · 금융상품 키 없음 진단·SlotGate·조건 상단 개편·계산기 연결. API·FE·worker-filings 배포.
- **2026-10-01 오후** — 금감원 키 등록(.env) · fin_products 첫 수집(상품 1,047·회사 173, API 대조 일치) · 목록↔상세 숫자 일치(prefill) · 회사명 정리 · 최근 인상 표시 · 예금자보호 안내. worker-filings·FE 배포.
- **2026-10-01 저녁** — 자산군 확장 조사: 국토부 실거래·R-ONE·ECOS 실측, 온비드·금융위·크립토·매물 약관/라이선스 대조, 아파트 추정가 PoC 3단지, 데이터 모델(user_assets·asset_valuations·user_liabilities) 제안 → docs/ASSET_EXPANSION_RESEARCH.md. 기존 라이선스 위험 3건(S&P·다우/BTC/KRX) 보고, .env.example 키 정리, FINANCE_PRODUCTS 약관 항목 갱신.
- **2026-10-01 밤** — 순자산 확장 구현: 096 스키마, worker-filings 잡 4개(re_trades·re_index·onbid·ecos_rates), 평가 엔진·API·FE 자산 메뉴 5화면, 지수 띠 FRED 표기·SPY/DIA·KRX 출처. 배포 직후 지수 스냅샷 길이 패닉(3~4분) 수정.
- **2026-10-02 새벽** — 2단계: 순자산을 내 포트폴리오로 통합, 부동산 시세·공매·예적금 벤치마킹 개편, 자산 점수(asset_scores)·AI(asset_ai, 별도 키), 홈 자산 카드·지수 띠 국내 금리, 워커 9→4(+content) 통합, 배포.
- **2026-10-02 아침** — 최초 일괄 적재: 실거래 전국 12종·아파트 매매 3년(387만 행, 토지 일부 남음), 점수·AI 전국화. 워커 이미지 태그 함정(api 이미지 공유로 수정), 429 처리·예약 중복 가드·AI 간격 버그 수정.
- **2026-10-02 아침(2)** — 홈 하단 빈 영역(sr-only가 body 기준 → 셸 relative) 수정, 부동산 시세 시도·전국 단계(re_sido·re_nation 점수, re_area AI 요약), 점수 근거 카드·배지 팝오버(부동산·예적금·대출·공매). API만 배포, FE·worker-asset은 승인 대기.
- 10-03 배치 관리: 제때 돌지 않은 배치 일괄 실행 버튼(BE run-overdue·선행 보류·제외 규칙, FE 배너 버튼). 미배포.
- 10-03 배치 관리: 「끝까지 이어 실행」 버튼(FE 루프) 추가, api·fe 배포.
- 2026-10-03 로고 누락 원인(tone blank 오판 395개 · 국내 원격 폴백 FMP .KS 404) 수정, 델타 갱신 실행, 배포는 사용자 대기
- 2026-10-03 밤: 부동산·금리 AI 브리핑(홈) · 부동산 R1~R4(차트·비교·관심·정렬/공매 묶기) · 금융 F1~F4(내 우대조건·가입 필터·내 대출 비교·금리 화면) · 자산 데이터 태그(@아파트·@지역·@상품·@금리) — 098 · api·worker-asset 배포, FE 재빌드 대기


## AGENT_STATE 이관 원문 (2026-10-05)

> 옛 `docs/AGENT_STATE.md`의 Now·TODO·Last를 **고치지 않고** 옮겼다. 원칙 섹션은 `docs/PRINCIPLES.md`로 갔다.
> 찾을 때: `Grep "<키워드>" docs/archive/sessions.md` → 해당 줄 근처만 Read.

### Now (09-11 ~ 10-03)
- **10-03 밤 (부동산·금리 AI 브리핑 · 부동산 R1~R4 · 금융 F1~F4 · 자산 데이터 태그 · api·worker-asset 배포됨 · FE 컨테이너 재빌드 대기 · 미커밋)** — 제안 `docs/mockups/asset-ai-tags-v1.html`, 사용자 선택: 홈 B · R1~R4 · F1~F4 전부 · 태그 A(공매 제외).
  **홈**: 금리·아파트 숫자 카드 2장 → 「부동산·금리 브리핑」 한 장(`dashboard/asset-pulse-cards.tsx` AssetBriefingCard, 탭 부동산|금리, 분위기·제목·문단·지켜볼 것 + 지역/금리 칩). 옆 칸 = 관심 단지 새 거래 · 예금 TOP · 공매.
  BE `asset_ai` 새 단계 `briefs`(`service/asset_ai_brief.go`, 노트 kind brief/re·rates, 쪽마다 1콜·해시 건너뜀, 분위기는 규칙 BriefMood*) · `/home/assets`에 `briefing`(`homeBriefing`: 없으면 re_area 00·rates 노트로 같은 모양 폴백, headline 빈칸) · `/macro/rates`에 `brief`.
  **아직 brief 노트 없음** — 내일 10:20 asset_ai가 처음 씀(지금은 폴백 문장). 바로 보려면 관리자 배치에서 asset_ai 수동 실행.
  **R1** `lib/re-trades.ts`(평형 = 전용 정수 ㎡, 공급 평 근사 ×1.33, 3개월 이동평균) · TradeScatter에 이동평균선. **R2** `wealth/complex-compare.tsx`(주소 `cmp=시군구:키` 최대 4, 평당가 이동평균 겹친 선 + 지표표, go()가 cmp 유지).
  **R3** 098 `re_watchlist` + `/realestate/watch` GET/PUT/DELETE(`handler/re_watch.go`, 이름은 서버가 정함, 50개) · `SummarizeWatchTrades`(최근 30일·신고가=같은 평형 3년 최고 초과, 직거래 제외) · `wealth/re-watch.tsx`(★ · 내 포트폴리오 「관심 부동산」 · 홈 카드).
  **R4** 랭킹 `peakDrop`(TopScores "-" 접두 = 작은 순)·`peakPct` 정렬 칩 · 공매 같은 건물 묶기(`lib/onbid-group.ts`, `/auctions/onbid?pageSize=10~120`, FE 120 → 1쪽 카드 1장→32장).
  **F1** `lib/fin-conds.ts` 우대조건 칩 8종 → 내 금리(가산폭 0이면 안 더함, 최고금리 상한) · basis "mine" · **F2** 가입 방법(비대면/영업점)·대상(청년/시니어/사업자) 칩 · **F3** 대출 「내 대출과 비교」(부채 잔액×금리 차÷12) · **F4** 금리 화면 브리핑 + 금리 차 추이(장단기·예대, `lib/rate-spread.ts`) + 내 금리 vs 시장 평균.
  **태그** `@아파트[시군구 단지]`·`@지역[..]`·`@상품[회사 상품]`·`@금리[..]` + `:지표`·`#` 카드. 정규식 BE `mention_parse.go`·FE `mention-text.ts` 같이 바꿈(종류 낱말 대안이 맨 앞, 대괄호 60자).
  해석 `service/asset_mentions.go`(메모리 이름 색인 30분: 단지 1.3만·상품·지역·금리 별칭) · 저장 098 `content_asset_mentions`(종목 표와 분리, 쓴 순간 값 고정) · `GET /mentions/assets` 자동완성 · 지표 `model/asset_mention.go`.
  FE `mentions/asset-mention.tsx`(칩·카드·미리보기 모달) · 자동완성 종류 칩·「데이터 넣기」 자산 모드·도움말. 실DB 확인 시험 `TestAssetMentionPreviewDB`(ASSET_MENTION_TEST_DSN 있을 때만).
  시험: Go service/llm/handler/model/repository 통과 · vitest 65파일 503 · tsc/eslint(손댄 파일) 깨끗. dev 서버 실측: 홈 카드·비교·공매 묶기·예적금 칩·금리 화면.
  **남은 것**: FE 컨테이너 재빌드(사용자 승인) · 로그인 화면(★·내 대출·내 금리·글쓰기 태그)은 실계정 없이 육안 미확인(단위·DB 시험만).
- **10-03 밤 로고 누락(LITE·CLS·국내 신규) · 코드+데이터 끝 · FE 재빌드·API 재시작은 사용자 승인 대기 · 미커밋** — 원인 둘.
  ① `stocklogo/tone.go`가 흰 판 위 가는 회색 글자·노란 로고를 `blank`(모노그램)로 떨굼 — 395개(LITE·CLS·SWKS·LEN·CAVA·ADI·RBLX…). 투명 40% 미만은 잉크(흰색에서 채널 48↑) 0.5%면 정상, 없으면 투명 5%↑ → light 판. 남은 blank는 KNX 1개. `-retone`으로 매니페스트 갱신, 시험 `TestLogoTone`.
  ② 로고 갱신은 **예약 배치가 아니라 손으로 도는 월간 스크립트**(`be/scripts/refresh-logos.ps1`)인데 09-02 뒤로 안 돌았고, 목록에 없는 국내 종목의 원격 폴백이 FMP `.KS`(신규 상장 전부 404)였다 → BE `LogoURL`·FE `stockLogoUrls` 국내 폴백을 네이버 CDN으로. 델타 갱신 실행: KR +20 · US +12. 거래대금 상위 1000 중 남은 누락은 원천에 그림이 없는 KR 5 · US 7.
  스크립트가 출력 리다이렉트 시 PS 5.1 stderr로 끊기던 것 고침. **배포**: `docker compose --profile prod up -d --build fe` + `bescriptsestart.ps1 -ApiOnly -Background`. 월간 자동 실행(작업 스케줄러)은 제안만.
- **10-03 배치 관리 「제때 돌지 않은 배치」 일괄 실행 + 「끝까지 이어 실행」(FE 루프, 잡당 1회) · api·fe 배포됨 · 미커밋** — 배너 「일괄 실행」 → `POST /admin/batch/run-overdue?dryRun=1`로 계획 확인창 → 승인 시 큐 등록.
  규칙은 `be/internal/service/batch_overdue.go`: 제외(뉴스 브리핑·워치독·stuck·이미 대기) + `batchRunAfter` 선행 보류(종가→롤업·판정·점수→꼬리, 적재→asset_scores→asset_ai, 글·언급→커뮤니티 요약). 보류분은 선행이 끝난 뒤 다시 누름. 시험 `TestPlanOverdue`. 배포: api 재시작 + FE 재빌드.
- **10-02 오후 (사용자가 FE·api·worker-asset 배포 14:25 → 실측 확인 · 관리자 배치 화면 수정은 FE 재빌드 대기)**
  **16:03 asset_ai(run 6402) 재실행**: re_area 16곳 전부 llm으로 바뀜(llm_feat_asset 오늘 89/250, 막힘 없음 · 단지 해설 단계는 계속 도는 중). 전남광주가 「조사 지역 중 가장 높은 상승률」이라 지어냄(실제 경기·서울이 높음) → area 프롬프트에 「시도 항목은 다른 시도와 비교 금지」 추가(미배포, 입력 해시가 같아 다음 변화 때 다시 씀).
  실측: 홈 문서 높이 900=뷰포트(전 1567) · /real-estate?sido=11 실데이터(시도 점수 96 「시도 15곳 중 1위」, 25구 분포, 월별 거래 합). asset_scores 수동 2회 정상(re_sido 15·re_nation 1).
  **AI 요약이 규칙 문장이던 원인**: llm_feat_asset 하루 250콜을 새벽 일괄 채우기가 다 씀(Gemini 하루 = 태평양시 → **16:00 KST 리셋**). 16:02에 asset_ai를 큐에 넣는 백그라운드 작업 걸어 둠.
  **「실행 눌러도 동작 없음」**: POST는 3번 다 202·실행됨. 화면이 (1) 알림을 페이지 맨 위에만 띄워 아래 카드에서 안 보였고 (2) 몇 초짜리 잡은 queued가 보이기 전에 끝나 anyRunning이 한 번도 참이 안 돼 폴링이 안 돌았고 (3) 요청 중이면 실행 직후 새로고침을 건너뜀 →
  토스트 + 실행 후 90초 따라가기 + 건너뛴 새로고침 다시 받기(admin-batch-panel.tsx). 가짜 관리자 세션(iframe fetch 스텁)으로 dev 서버에서 확인. **FE 재빌드해야 반영.**
- **10-02 아침 08시 (FE 결함·부동산 시도/전국 단계 · API만 배포 · FE·worker-asset 재생성은 사용자 승인 대기 · 미커밋)**
  **① 홈 아래 회색 빈 영역(심각)**: Reddit 카드 버튼의 `sr-only`(absolute)가 위치 조상이 없어 body 기준으로 문서를 1567px로 늘림 →
  `app-shell` 루트·`PageScrollShell` main에 `relative`(모든 화면 공통 해결). 21개 메뉴 iframe 실측: 다른 화면은 문제 없음.
  **② 부동산 시세 시도/전국**: 주소 `?sido=11` = 구를 안 골라도 시도 전체(머리 요약·시군구 오름/내림 막대·많이 오른/약한 곳·시도 점수 카드·시군구 표·시도 단지 랭킹·시도 R-ONE 지수).
  전국 = 시도별/시군구 전체 표 + 같은 머리 요약. BE `asset_scores`의 re_region 단계가 `re_sido`(시도끼리 백분위) · `re_nation`(점수 없이 사실)도 만든다 —
  월별 거래 13개월을 facts에 실어 조회 때 큰 합계 쿼리 없음. 시군구 facts에 rank/of. API: overview에 `area`(+전국은 `sidos`), market에 `sido=`.
  **③ AI 요약 전국·시도**: asset_ai 새 단계 `re_area`(kind area 프롬프트, 160자 2~3문장, 5개씩 ≈ 4콜/회, 해시 건너뜀).
  **④ 점수 근거**: `ScoreCard`(종목 종합 점수 카드 모양, 축별 백분위 막대 + 원값 + 비중 + 비교 묶음 + 순위) · 목록 배지 `ScoreBadge kind=`를 누르면 같은 카드가 팝오버로
  (부동산·예적금·대출·공매 전부). 시험: Go `TestAreaAggFacts` · vitest 487 · tsc/eslint(손댄 파일) 깨끗. dev 서버 + fetch 스텁으로 시도·전국·팝오버·375px 확인.
  **남은 것(사용자 승인 필요 — 자동 모드가 운영 배포를 막음)**: `docker compose --profile prod up -d --build fe` ·
  onbid(07:40 시작, kamco 429 대기 중) 끝난 뒤 worker-asset `--force-recreate --no-build` → `asset_scores` → `asset_ai` 수동 실행(re_area 첫 채우기).
  **발견만**: 예적금 「우대 의존」 같은 값 동점이 많아 0.00%p(최선)도 백분위 55(pctRank 중간 순위) · 공매 목록이 같은 건물 호실 20여 건으로 도배(건물 묶기 후보) ·
  세종(36) 시군구 없음(법정동코드 적재 대기) · `전남광주통합특별시` 칩은 「전남광주」, R-ONE 지수는 전남 분류를 씀.
- **10-01 밤~10-02 새벽 (2단계: 포트폴리오 통합 · 벤치마킹 개편 · 자산 점수·AI · 워커 9→4 · 배포됨 · 미커밋)** — 문서 `docs/ASSET_EXPANSION_RESEARCH.md` §13.
  **메뉴**: 순자산 메뉴 제거 → 「내 포트폴리오」(/holdings) 하나(`holdings/holdings-wealth-section.tsx`: 내 자산 전체·자산 추가·부동산/금융상품/부채/금리). 금감원 상품 고르기(`FinProductPicker`).
  주소 /real-estate · /real-estate/auctions · /holdings/assets/[id] · /finance/rates, 옛 /wealth/*는 redirect.
  **화면**: 부동산 시세 재작성(`wealth/real-estate-client.tsx` 시도·시군구 칩 → 지역표/단지 랭킹 → 단지 평형 칩·요약·실거래표) · 공매 카드형(`auctions-client.tsx`, 회차 이력, 정렬 score 추가 BE) ·
  예적금/대출 점수·우대조건 AI 칩·지난달 대비(`finance/fin-shared.tsx` FinIntelLine/RateDelta) · 홈 4카드(`dashboard/asset-pulse-cards.tsx`, `/home/assets`) · 지수 띠 kr-base·ktb3·ktb10(% · bp).
  **BE**: 097(asset_scores·asset_ai_notes·fin_rate_history) · `asset_scores` 09:50 · `asset_ai` 10:20(키 GEMINI_SUB_API_KEY=버킷 llm_sub + llm_feat_asset 120콜/40만, 묶음·해시 건너뜀·max_tokens).
  **워커**: `cmd/worker`(WORKER_ROLES) → worker-market·worker-stock·worker-asset + worker-content(api 바이너리 SERVER_ROLE=batch). api는 SERVER_ROLE=api. 옛 cmd/worker-* 삭제.
  배포: 전 워커 + api + fe. **worker-stock 교체 03:11**(warnings_sync 끝난 직후 — 밤 주식 잡이 01:30~05:47 끊김 없이 이어져 빈 틈이 없음, 막 시작한 ext_metrics만 끊김).
  **함정**: 워커 서비스마다 build가 있어 이미지 태그가 따로 → `-ApiOnly` 뒤 `--no-build` 재생성이 22시 옛 이미지로 떴다(새 코드 전부 미적용, worker-stock은 No such image).
  → compose의 워커 4개를 `image: quant-port-api:latest`(pull_policy never)로. 이제 Image id가 api와 같은지로 확인.
  **10-02 새벽 최초 일괄 적재(사용자 결정)**: re_trades 전국 12종 + 아파트 매매 36개월(보관 48) · 회당 15,000·버킷 60,000 · 우리 예산 소진이면 멈춤(redact가 ErrExhausted를 잃던 것 고침) ·
  `RunScheduledJob`이 queued/running이면 예약 건너뜀(03:30 확인). 6251(상한 6만) 03:37 시작, 분당 ~65구간이라 약 11시간 → 끝나면 asset_scores → asset_ai 체인.
  asset_ai: llm_feat_asset 250/60만, 전국 시군구 + 시군구별 상위 5단지, 입력 반올림, 옛 AI 글 보존 · **간격 버그**(503 내부 재시도 뒤 4초 간격 거절 → 전부 규칙) 고침 · 배열만 준 응답 감싸기(시험).
  첫 정상 실행 51콜·6.5만 토큰(지역 12·단지 49·우대조건 253).
  **일괄 적재 결과(07:42)**: re_trades 387만 행·1.3GB — 254 시군구 × 12종 13개월 완료, 아파트 매매 2023-11부터(145만). **토지만 1,977구간 남음**(LandTrade 하루 한도 429 → 그 종류만 제외, 내일 03:30 예약이 이어받음).
  고친 것: 429/버킷 차단을 「하루 소진」으로 보고 회차 전체를 멈추던 것 → 429 난 종류만 제외·차단 풀리면 계속 · 수동 큐 2시간 시한(context deadline)에 한 번 끊김(예약 회차엔 시한 없음).
  asset_scores: 단지 12,948 · 시군구 254 · 상품 1,035 · 공매 6,596. asset_ai 누적: 지역 254 · 단지 1,299(일부 규칙, 다음 날 AI로) · 우대조건 269. 오늘 llm_feat_asset 약 240/250 사용.
  **남은 것**: 법정동코드 전국 적재(re_trades가 mois_region 간격 거절 — 1.1초 대기 넣은 새 이미지로 재시도) · asset_ai 첫 실행 후 우대조건 칩 확인(밤 22~03시는 Gemini 503 잦음) ·
  단지 좌표/지도 없음 · 공매 관심 목록 없음 · 실거래 점그래프 x축이 시간 비례 아님(기존 TradeScatter).
- **10-01 저녁~밤 (순자산 확장 「전체 개발 일괄」 · BE 전체+FE 배포됨 · 미커밋)** — 조사 `docs/ASSET_EXPANSION_RESEARCH.md`(§12 구현).
  **라이선스 결정 반영**: 지수 띠에 FRED 권리자 표기 · ETF 점검/랭킹 「출처: KRX 통계정보」 · 지수 띠에 SPY·DIA(Finnhub, 이력은 stock_daily_closes — index_closes 백필 167행) · 업비트 그대로.
  **BE**: 096(user_assets·asset_valuations·user_liabilities·re_regions·re_trades·re_sync_state·re_index·macro_rates·onbid_items·onbid_notices, re_index에 (statbl_id, cls_name) 인덱스).
  worker-filings 새 잡 `re_trades` 03:30(국토부 12종, 호출 6,000/회, 끝에 전 자산 재평가) · `re_index` 06:00(R-ONE 11표) · `onbid` 07:40 · `ecos_rates` 09:20.
  평가 `service/wealth_valuation.go`(창 3→6→12개월+지수, 직거래·해제 제외) · `service/wealth.go` · API `handler/wealth.go`(/wealth·/realestate/*·/macro/rates·/auctions/onbid·/crypto/markets).
  첫 실행: 실거래 765,596건(서울 12종 13개월) · R-ONE 11표 · ECOS 21계열 17,417값 · 온비드 공고 4,292·회차 29,623(429 몇 번 → 간격 1.1초).
  **FE**: 사이드바 「자산」(순자산 /wealth · 자산 상세 /wealth/assets/[id] · 부동산 시세 · 부동산 공매 · 금리). 375px 확인(그리드 자식 min-w-0로 잘림 수정).
  **사고**: 배포 직후 API 재시작 루프 약 3~4분(18:16~18:20) — 지수 카탈로그에 spy·dia를 넣었는데 Redis 옛 스냅샷(13칸)을 위치로 덮어써 패닉 →
  `snapshotOrCatalog`가 `alignIndicesToCatalog`를 거치게 고침. 카탈로그를 늘릴 땐 스냅샷 길이를 의심할 것.
  **남은 것**: 행안부 법정동코드 활용신청(사용자) — 전까지 전국 아파트 없이 서울만 · 테스트 계정 `qae2ewealth1001`(자산 6·부채 1, e2e 정리 대상) ·
  현금흐름 원장(배당·이자·임대료)·TWR/XIRR 없음(가격 차익만) · 공매 관심 목록 · 단지 면적 칩 묶기(84.6/84.39/84.89가 따로) · 내일 03:30 재평가 후 헬리오 28.1→29.5억 반영 확인 ·
  go 시험 중 `TestIntegrationToss_acceptsUnder100InSingleDeclare`(실 Toss WS 401)만 실패 — 이번 변경과 무관.
- **10-01 오후 (금감원 키 등록 · 금융상품 실데이터 · worker-filings 재생성 + FE 배포 · 미커밋)** — 사용자가 키를 줌 → `be/.env` `FSS_FINLIFE_API_KEY`
  (gitignore 확인). worker-filings `--force-recreate --no-deps`, `fin_products` 수동 1회: 예금 431 · 적금 335 · 주담 111 · 전세 58 · 신용 112 · 회사 173.
  DB ↔ 그 자리 API 대조 일치(수협 신용 5.18%, 우리저축 12개월 4.00/4.07%). curl 기본 UA는 서버가 무응답(Go UA는 정상, 문서 §3에 적음).
  실데이터로 고친 것: 목록(복리 4.07%·350,818원)과 상세 기본 옵션(단리 4.00%·338,400원) 불일치 → 목록이 금액·기간·방식·금리를 상세에 실어 같은 옵션으로 열림 ·
  회사명 「주식회사」 떼기(표시만) · 우대 없는 상품의 같은 숫자 두 번 숨김 · 주담대/전세 「전월 평균이 지금 범위보다 낮음 — 최근 인상」 · 예금자보호 1억원 안내.
  vitest 484. **남은 것**: 내일 05:10 예약 수집이 한 번만 도는지(SlotGate) · 대표전화 형식(0518119400) 그대로.
- **10-01 새벽 (요청 1~5 · API·FE·worker-filings 배포됨 · 미커밋)** — 제안서 `docs/mockups/requests-1001-v1.html`, 결정 4개 모두 추천안.
  **① 스크리너**: 결함 — 스크리너 안 종목 화면에서 「이 종목보다 나은 종목 찾기」 무반응(한 번만 처리하던 `likeHandled` → 값마다 · 오른쪽 종목 화면 닫음),
  「보통 …」이 측정률·기준 값에 밀려 잘림(칸 아래 둘째 줄로), 해외 시가총액 「보통 0~0」(1 미만 유효숫자 2). **결정 B 닮은꼴**: `ScreenQuery.like` →
  서버 정렬식 `screenLikeSortExpr`(축 5개, 모자란 만큼 제곱 · 넘친 만큼 ¼, 다른 업종 +400), 채우기 = 종합 ≥ 기준 · 축 −15 · PER·ROE·부채비율은 열만.
  결과 칩(나은 점 파랑·모자란 점 회색). 실측 삼성전자 5→105 · MU 2→26(MRVL·MPWR·ALAB·PLTR·AMD…). `screen-values`에 `sector`.
  **② 검토만**: 번역기 = 자체 GPU(LMT-60 4B) 하루 2,069요청·가동률 1.1%, Reddit은 ~10%. LLM = Reddit 요약 US 키 하루 40콜(콜 12%·토큰 6%).
  빠듯한 건 **KR 키**(376~394콜·88~95만 토큰/일, 상한 100만) — 국내 종목 AI 요약 배치. 최적화 후보(미적용): 변화 작으면 전날 요약 승계(40→~15콜).
  **③ 미국 개미 분위기**: `GET /social/market-mood`(`service/social_market.go` — 지수 ETF 감성 60%+상위 종목 40%, 55% 기준 ×2.2, 규칙) · 홈 카드
  `dashboard/reddit-mood-card.tsx`(점수·ETF·AI 한 줄 | 언급·낙관:비관·급증·서브레딧·14일 막대 | 시장 인기 글 2). 시장 글 = social_posts 첫 RSS 1회
  (`MarketTop`, stocks+investing+StockMarket top day) → 심볼 `_MARKET`, AI 한 줄 = social_summary 첫 항목(같은 프롬프트, 하루 1콜). 오늘 +23 약한 낙관.
  **④ 태그**: `@종목`·`@종목:지표` 누르면 종목 퀵룩 모달(글 안 떠남), 점수 필드는 게이지 · 쓰는 화면 ≥1280px 옆 미리보기 상시
  (`mentions/compose-side-preview.tsx`, 0.4초 디바운스 · 좁으면 토글) — 커뮤니티 글쓰기·리포트 작성. 수정 화면(커뮤니티 상세·리포트 상세)은 토글 그대로.
  **⑤ 금융상품**: 빈 원인 = `FSS_FINLIFE_API_KEY` 없음(**사용자가 키 발급 예정**). 같이 고침: worker-filings 예약이 `time.After` 조기 깸으로 하루 두 번
  (fin_products 05:09:54+05:10:00) → `WaitForSlot`+`SlotGate`(전 잡). 화면: 「랭킹」 탭 제거 · 조건(금액 칩·기간 칩·금융권 칩) 맨 위 · 나머지 「상세 조건」 ·
  행마다 「이 금리로 계산」(`calcHref`/`readCalcPrefill`, 계산기가 값 채워 열림). 가짜 데이터(페이지 fetch 스텁)로 순위·링크·계산기 일치(304,560원) 확인.
  테스트: vitest 478→483 · Go 새 시험 6(닮은꼴 2·분위기 2·중복 글 1·…) 전부 통과. 테스트 계정 `qae2e…`(가입 1건, e2e 정리 대상) 만들어 글쓰기 화면 확인 — 글은 안 올림.
  **남은 것**: 키 받으면 .env + worker-filings 재생성 + `fin_products` 수동 실행 · 건수·금감원 대조 · 시장 글 8개 수동 수집 완료(05:50) ·
  내일 04:20 시장 한 줄에 글 포함되는지 · 닮은꼴 가중치(¼·400) 사용자 반응 · 금융상품 대출 가짜 데이터 확인은 안 함(예·적금만).
- **09-29 새벽 (금융상품 비교 MVP · API·FE·worker-filings 배포됨 · 미커밋 · 키 대기)** — 문서 `docs/FINANCE_PRODUCTS.md`.
  **①** 095(`fin_products` 원문 base/options JSON · `fin_companies` · `fin_watchlist`), 잡 `fin_products`(worker-filings **05:10**, 버킷 `fss_finlife` 1초,
  금감원 「금융상품 한눈에」 Open API 5종 × 권역 5 + 회사). **인증키 없음** → 사용자가 finlife.fss.or.kr에서 발급해 `be/.env`에 `FSS_FINLIFE_API_KEY` 넣고
  worker-filings 재생성해야 데이터가 찬다(그 전까지 잡은 「not configured」 오류, 화면은 「아직 수집된 공시가 없습니다」). 파킹통장은 공시 API가 없어 제외.
  **②** FE 사이드바 그룹 「금융상품」(예·적금 비교 `/finance/savings` · 대출 금리 비교 `/finance/loans` · 금융 계산기 `/finance/calculator`, 탭은 `?tab=`) + 상세 `/finance/products/[id]`.
  순위·계산은 FE(`lib/fin-products-api.ts` rankSavings/rankLoans, `lib/fin-calc.ts`) — 예·적금 기본금리 세후 이자, 대출 전월 평균금리 총이자, 최저금리는 「광고 금리」 별도. 관심 상품(로그인), 비교(3개).
  테스트: vitest 13(fin-calc 7 · 순위 6) + Go 2. 가짜 fetch 스텁으로 목록·비교 렌더 확인(모바일 375 가로 스크롤 없음).
  **③ 사고**: `restart.ps1 -ApiOnly` 재빌드 중 **Docker 엔진이 멈춤**(04:25~04:32 전면 다운) — 워치독은 「떠 있지만 응답 없는」 엔진을 못 고친다. `docker desktop restart`로 복구.
  그 여파로 04:32 social_posts(03:23 시작) 끊김, api가 `<id>_quant-port-api`로 남아 `--force-recreate`로 정리. 04:37 manual social_posts·social_summary가 큐에서 다시 돔(내가 넣은 것 아님).
  **남은 것**: 키 발급 → 첫 수집 결과(종류별 건수) 확인 · 금감원 사이트 수치와 2~3개 대조 · 법률 검토 항목(문서 §8) · worker-filings SlotGate 누락(별도 작업 칩).
- **09-29 새벽 (Reddit 커뮤니티 탭 · 점수·AI 요약 · 로그 분석 · API·FE·worker-filings 배포됨 · 미커밋)** —
  **①** 094(`social_insights`·`social_posts`·`social_summaries`). `social_mentions`(00:05)가 언급 상위 150종목의 **ApeWisdom 종목 페이지**(HTML)에서
  언급 사용자·감성%·WSB 감성·키워드를 받음(`marketdata/reddit_social.go`). 새 잡 `social_posts`(worker-filings 00:30): **Reddit 검색 RSS**(JSON은 403, RSS만 열림 ·
  **분당 1회** 실측 → 버킷 `reddit_rss` 62초) wsb·stocks·investing·StockMarket·options 이번 주 인기 → 티커가 실제로 적힌 글만(제목 먼저) → 번역기(TRANSLATE_*, compose에 env 추가)로
  제목·앞부분 번역, 80종목 로테이션. 새 잡 `social_summary`(API 04:20, 기능 버킷 `llm_feat_social` 50콜·15만, 상위 40): 레딧 여론 요약·태그·낙관/주의 근거.
  **커뮤니티 점수는 규칙**(`CommunityScore`: 감성을 보통 55%·사전 20명으로 줄여 ×2.2, 5명 미만 없음, ±25 낙관/비관) — LLM은 그 점수에 맞춰 씀.
  **②** 점수: 심리 축에 Reddit 성분(미국만, 가중치 0.3 = 축의 23%, 판 `|reddit`, inputs `redditSentiment`, 스냅샷 창 [-2일,+1일]). 종목 AI 요약 사실표에 `reddit`(|점수|≥25·언급 2배일 때만 한 구절).
  **③** FE `community/stock-reddit-panel.tsx`(머리글 점수+게이지 · AI 핵심 요약/전체 분석 · Reddit 버즈 · 인기 글 5개씩, 번역 기본·원문 토글), `stock-social-line.tsx` 삭제. `/signals`에 `reddit{insight,posts,community}`.
  **④ 로그 분석**: fe `TypeError: fetch failed` 929건 = **전부 API 장애(09-28 23:25~09-29 01:46 KST)** 중 Next 재검증 실패 — 워치독이 compose 재생성이 끊겨 남은 `<id>_quant-port-api`를
  `docker start` → 네트워크 0개로 떠 2시간 20분 「복구 실패」 반복. 컨테이너 재생성으로 복구, `watchdog.ps1`에 `Repair-DetachedApi`(api 네트워크 0개면 빌드 없이 재생성) 추가.
  nginx 5xx·499도 전부 그 구간. `transformAlgorithm is not a function` 3건(스트림 중단, 무해) 남김. `WaitForSlot` 1초 미만 조기 깸 로그 제거(worker-earnings 하루 288줄).
  **실행 확인**: 종목 요약 150 · 글(첫 8종목 90건·번역 170, 나머지는 재큐 5477 진행 중) · AI 요약 40/40(첫 실행은 `llm_us` 간격을 뉴스 브리핑과 나눠 21건이 규칙으로 → 간격 거절이면 한 번 기다려 재시도, 재실행 AI 21).
  **04:25 Docker Desktop 다운**(secrets-engine 소켓 — 워치독이 04:32 자동 복구) 직후 API 재배포가 `<id>_quant-port-api` 이름 충돌로 꼬여 ~2분 중단 → Created 스텁 제거·이름 복구로 정상(최신 이미지·네트워크 1).
  불용어 보강(after·giving…)은 배포됨, 오늘 치 키워드엔 반영 안 됨(내일 00:05부터).
  **남은 것**: 내일 00:25 점수에 `|reddit` 붙는지 · 레딧 성분 IC(40거래일 뒤, 떨어지면 가중치 0) · Reddit RSS 이용은 공식 API 약관 회색지대(사용자 판단) ·
  공유 버튼(첨부 화면)은 안 만듦 · 장애 중 클라이언트 폴링 무백오프(TSM 스냅샷 6회/분) 개선 후보.
- **09-28 밤 (수정 화면 태그 · 개선 · 레딧 검토 · API+FE 배포, worker-filings는 financials_domestic 끝난 뒤 자동 배포 대기 · 미커밋)** —
  ① 커뮤니티 글 **수정 화면**에 태그 도우미·미리보기(`CommunityPostPreview`로 뽑아 공용). ② 개선: 비로그인 글쓰기 진입의 로그인 복귀가 쿼리(article·symbols)를 버리던 것 ·
  밸류에이션 도장 「EDGAR + Finnhub · 보조 값은 최근 12개월」 · **XBRL 통화 혼합 근본 수정**(20-F는 같은 값을 현지 통화+달러 환산치로 두 번 실음 → 회사당 주 통화 하나만,
  `primaryMonetaryUnit`/`keepFactUnit`) — TSM 영업마진 1%→45.7%, 성장 −97%→+33.9%, 외국 발행사 55종목 재적재(혼합 0) · 재무제표 탭에 공시 통화(TWD) 표기·가짜 분기 PER 제거·
  라벨 2년 어긋난 행 제거. ③ 레딧: 서브레딧 9개 적재 추가(과거 없음 → 오늘부터), 잡음 티커(BYD·DTE·IP·JUST…) 제외. 글·감성은 불가(Reddit 403, 공식 API 승인·유료).
  제안서 `docs/mockups/reddit-tab-v1.html` — 사용자는 **Reddit 공식 API를 무료로 쓸 수 있는지 먼저** 물음(답: 다중 사용자 서비스는 상업 분류 가능성 높음 · 2026-06부터 모든 앱 수동 승인 · 표시 시 출처·48시간 삭제 의무) — 키 제공 여부 결정 뒤 탭·인기 목록·점수 진행.
  버킷 상한 < 간격 실수 셋 수정(naver_stock 60 · apewisdom 30 · finnhub_metrics 25) — 새 버킷은 상한을 정상 속도보다 위로. 서브레딧 9개 적재 확인(1,248행).
- **09-28 (사용자 요청 1~7 · BE+FE 배포됨 · 미커밋)** — 제안서 `docs/mockups/data-tags-screener-v1.html`, 결정은 메모리 `data_tags_and_screener_decisions_0928`.
  **①** 관리자 `jjmin321@naver.com` → 아이디 `jjmin321`(DB id=3, role=admin 유지, `DefaultAdminEmail` 상수도) · 로그인 「이메일이 아이디」 문구 삭제.
  **② 버그**: 「커뮤니티에 공유」 무반응 = `lib/modal-history` 경합(onClose → back()이 이동을 밀어냄) → `NewsCommunityShareLink`(leaveModal) 셋 셸 공용, 관련 종목 카드 `onNavigate`도 leaveModal.
  **② 태그**(092 `content_mentions`): `@종목` 칩 · `@종목:지표`(쓴 순간 값 + 「→ 지금」) · `#종목` 그 줄 아래 카드 · `@[띄어쓰기 이름]`. 지표 = 스크리너 필드(Help·Aliases 추가, `model/screener_help.go`).
  해석은 서버 한 곳(`service/mention_parse.go`, 조사 떼기·최장 이름), FE는 같은 정규식 + 서버 `tokens{raw,len}`만 본다(`lib/mentions.ts`·`lib/mention-text.ts`).
  커뮤니티 글·리포트 본문 둘 다(생성·수정 Check+Sync, 상세 Load), 편집기 자동완성(@→종목, :→지표+값)·「데이터 넣기」·「태그 도움말」(첫 1회 자동)·미리보기(`POST /mentions/preview`).
  **③** 종목 「커뮤니티」 탭(글+리포트 한 목록, 붙인·태그한 것 모두 — 두 목록 API의 symbol 필터가 mentions도 봄). 개요 끝 카드는 탭으로 연결.
  **④** 오늘의 변화: 09-25 「공시」 0건은 추석 휴장(데이터 정상). 칩 여러 개 = **AND**(모두 가진 종목), 칩에 종목 수·0이면 비활성.
  **⑤** 스크리너: ▲▼ 제거(휠 변경도 막음) · ⓘ 설명 · 「보통 p25~p75」(카탈로그 `quartiles`) · 기준 종목(값 칩, 누르면 채움) · 「이 종목 이상으로 채우기」(점수6+PER·ROE·부채비율) · `?like=` + 점수 카드 버튼. `GET /stocks/{symbol}/screen-values`.
  **⑥ 발견·수정(지표 로테이션)**: 09-22 이후 한 번도 안 돎(`lastFundamentals=time.Now()` → 기동+24h인데 api 매일 재배포) + 도는 날엔 8/min 요청 버킷을 1.1초 간격으로 두드려 **2,098종목이 한도 실패 행으로 덮임**(TSM 포함).
  → DB 마지막 시도 시각에서 재개 · API의 EDGAR 클라이언트 `UseBatchBudget()` · 예산 거절은 행을 안 덮고 회차 중단 · partial도 지킴. **통화 가드**: 20-F(현지 통화, TSM은 TWD+USD 혼합)를 ADR 달러 가격과 나눠
  SONY PER 0.025·TM 0.26 → 가치 축 98 → 금액·EPS 비우기(혼합이면 비율도). 가드 56종목, PER<1이던 70종목 재적재함(점수는 오늘 00:25 반영).
  **⑥·⑦ 조사만(제안 대기)**: TSM 밸류에이션 = Finnhub 무료 `/stock/metric`(PER 28.3·PBR 7.27, 본국 상장 기준 비율) · 추천 추이 무료, 목표가는 유료 · 국내 목표가는 Naver integration `consensusInfo`.
  Reddit = 공식 API는 2025말부터 승인제·상업 유료 → ApeWisdom(무키, 언급 수) 1단계 제안, 번역은 텍스트 받는 2단계에서만.
  **후속(같은 날, 사용자 결정: 해외 전 종목 보조 · 해외 의견+국내 목표가 · Reddit 1단계)** — 093(`stock_ext_metrics`·`stock_analyst_consensus`·`social_mentions`),
  worker-filings 새 잡 셋: `social_mentions` 00:05(ApeWisdom, 낱말 티커 제외) · `ext_metrics_overseas` 03:00(Finnhub metric+추천, 등급제 1,500, 버킷 `finnhub_metrics` 20/분) ·
  `analyst_domestic` 06:30(네이버 `consensusInfo`, 버킷 `naver_stock` 1.2초). 보조는 **EDGAR 빈 칸만**: 점수(`applyExtMetrics`, 판에 `|ext`)·지표 화면(`supplement`, 보조만 있으면 basic 등급).
  화면: 밸류에이션 탭 「애널리스트 의견」 카드 · 커뮤니티 탭 Reddit 줄 · 오늘의 변화 「레딧」(급증·과열, `socialEvent`) · 스크리너/태그 필드 4개(애널리스트 의견·평균 목표가·목표가 여력·레딧 언급).
  `GET /stocks/{symbol}/signals`. 레딧은 점수에 **아직 안 씀**(검증 뒤).
  **남은 것**: 새 잡 첫 실행 확인(워치독 catch-up) · 태그 실글 작성 확인(미리보기·저장 왕복은 확인, 실제 게시는 안 함) · 커뮤니티 글 **수정 화면**엔 태그 도우미 없음 · 레딧 신호 IC 검증 후 점수 반영 여부.
- **09-27 (사용자 요청 1~5 · 전부 배포됨 · 미커밋)** — 제안서 `docs/mockups/community-ai-v1.html`, 결정은 메모리 `community_and_ai_decisions_0927`.
  **① 버그**: 목표가 예측·메모장 무한 재적재(0건 계정의 예시 자동 켜기 표가 뒤집히는 루프 → 429 → 다른 화면까지 「요청이 너무 잦습니다」) —
  `useAutoSample(boolean|null)`+`nextAutoVote`. 배치 관리 20초 — 커버리지 3분 캐시·뒤에서 재집계(`coveragePending`)·일봉 커버리지 PK 조회·폴링 겹침 제거.
  **② 화면**: 홈 내 종목 뉴스 줄(grid minmax — WebKit 추정, Chromium 재현 불가). 관심 종목 상단 「오늘 내 관심」 띠 + 행마다 뉴스·변화·AI 한 줄(`/user/today`의 `aiOverview`).
  **③ 가입**: 아이디(`users.email` 칸 재사용 — 기존 회원은 이메일이 아이디)·닉네임 2~10자 유니크(**첫 배포에서 중복 360개에 번호**)·비밀번호 8~15자(가입·변경만).
  e2e 접두사 `qae2e`(growth·cleanup 반영) — **e2e 실행은 안 함**. 개인정보처리방침 1항 갱신(아이디·커뮤니티 이미지).
  **④ 커뮤니티**(088): `/community`(SSR·색인·사이트맵·DiscussionForumPosting) · 글/댓글(답글 1단)/좋아요/조회(1인 1회)/신고(3명 자동 숨김, `/admin/community`) ·
  종목 붙이기(작성 시 가격·점수 스냅샷 + 지금 점수·AI 요약·30일 선·실적 D-n·뉴스 2) · 기사 붙이기(뉴스 모달 「커뮤니티에 공유」) · 이미지(브라우저 1600px 축소 +
  서버 재인코딩 ≤1MB, 실측 4.7MB→186KB, EXIF·GPS 제거, `golang.org/x/image` 추가) · 뱃지(운영자·주주(이 글 종목 보유)·활동·기록 습관·새내기, 순위·적중률 없음) ·
  금칙어(욕설·리딩방) · 글 1분/댓글 10초 · 종목 개요 「OO 이야기」 카드. 테스트 글 id=1은 확인 뒤 소프트 삭제함(테스트 계정 관심 2종목은 남아 있음).
  **⑤ AI**: (a) 질문 칩 4→10(애널리스트·외국인기관·싼가·위험·공시·내 보유, LLM 0, 재료 없으면 칩 숨김 — `stock_qa_more.go`).
  (b) 실적 관전 포인트(089, 매일 07:10, 국내는 작년 발표일 추정 포함, 기능 버킷 `llm_feat_earnings` 60콜·12만) — **아직 한 번도 안 돎**, 관리자 「지금 실행」 또는 내일 07:10.
  (c) 스크리너: 업종(11섹터) 범위 + 새 필드 `daysToEarnings`(추정 포함, 국내 0→251)·`scoreChange`·`flowNetPct`, 문장 파서 확장(「실적 발표 앞둔 반도체 중 외국인이 사는 저평가」 전부 인식),
  못 알아들은 낱말 수집 → 월 07:40 LLM 후보(091, `llm_feat_screener`) → `/admin/screener-phrases` 승인 → 파서가 합침.
  (d) 리포트 AI 검토 노트(090, 30분마다, 공개 리포트만, `llm_feat_research` 30콜·8만, 규칙 노트는 6시간 뒤 재시도).
  기능 버킷은 apibudget의 가상 provider — 키 버킷과 **둘 다** 통과해야 호출. 조회 경로 LLM 0 유지. `BATCH_SCHEDULE.md`에 새 잡 3줄.
  **남은 것**: 관전 포인트·사전 후보 첫 실행 확인, e2e 스위트 실행, Safari 실기기 확인(홈 카드), 커뮤니티 이미지 캡션·수정, 예측·리포트 붙이기(2차).
- **09-27 (엣지 캐시 재확인 — 이제 동작함 · 미커밋)** — 사용자가 CF 설정을 고친 뒤 재측정: 여러 경로에서 `MISS` 뒤 즉시
  `HIT`/`Age` 확인(`/quotes/indices`·`/stocks/scores/top`·`/rankings`·`/stocks/005930`·`/news/domestic`·`/news/us`·
  `/market-calendar/KR`·`/earnings/highlights`). 개인화 경로(`/sample/price-targets`)는 여전히 `private, no-store` +
  `BYPASS` — 새지 않음 확인. **남은 사소한 것**: CF가 응답 `Cache-Control`의 `max-age`를 오리진 값(0)이 아니라
  **`14400`(4시간, 아마 「Browser Cache TTL」이 「오리진 존중」이 아닌 고정값)으로 덮어쓴다.** FE의 모든 브라우저發 fetch가
  `cache: "no-store"`를 명시해(전수 확인: market-api.ts 전부·api-client.ts) 지금은 영향이 없지만, 앞으로 `no-store` 없이
  fetch를 추가하면 4시간 묵은 값을 볼 수 있다 — CF의 Browser Cache TTL을 「Respect Existing Headers」로 바꾸는 편이 안전.
  사용자에게 보고, 급하지 않으면 다음에.
- **09-26 후속 (엣지 캐시 경로 추가 · API만 배포 · 미커밋)** — 엣지 경로 추가 5개: `/quotes/indices`(최다 호출, 서버 60초+엣지
  75초 < FE 낡음 180초) · `/news/domestic` · `/news/us` · `/news/{market}/symbols` · `/rankings/history` — 전부 신원 조회
  없음 확인. 실시간 가격·캔들·`/news/reader`는 제외.
  **보류(권한 거부)**: `088` financial_periods 중복 인덱스 제거 마이그레이션 — 운영 DB 스키마 변경이라 자동 모드가 막음, 사용자 판단.
  `financials_domestic` 실행 중이라 `restart.ps1 -ApiOnly`로 API만 재생성(배치 유지 확인).
- **09-26 후속 (기존 결함 정리 · BE+FE · 배포됨 · 미커밋)** ① 뉴스 브리핑이 워커 재배포마다 **끝난 슬롯을 AI로 다시 생성**(LLM 중복) →
  `briefingRunState.seedFromStored`: 최신 행이 같은 슬롯의 `llm`이면 기동 실행 건너뜀(규칙·승계면 예전처럼 재시도), 시험 추가.
  ② 지수 띠 「낡은 값」이 밤·주말 상시 → 서버 `updatedAt`은 **값이 바뀐 시각**이라 멈춤. `MarketIndexStore.checkedAt`(회차 완료 시각) +
  API `checkedAt`, FE는 그것을 서버 회차 60초×3으로 판정(`INDEX_SERVER_LOOP_MS`). ③ 사이드바 두 벌이 `/user/changes/summary` 2회 →
  `cachedGet` 사용자별 30초 공유. ④ 브리핑 승계 배지 자정 넘김 「어제 21:00」(`formatCarryClock`, vitest 4). ⑤ 안 쓰는 `my-changes-strip.tsx` 삭제.
  확인만(결함 아님): 비로그인 401 없음(`/auth/session`), 사이트맵·noindex 불일치·H1 중복은 같은 날 다른 세션이 이미 고침.
  **남은 것**: 로그인 화면 실측(테스트 계정 `onboarding-…@quant-port.test`에 보유 3종목 매매 기록을 넣어 둠 — /holdings 지수 비교·매수 이유 태그·
  진단·위험 카드 확인 전), 비로그인 홈 가입 유도·종목 허브 페이지(SEO 고아 1,000) 제안, D 아침 이메일.
- **09-26 (외부 방문자·크롤러 관점 점검 — 버그처럼 보이는 곳 · 데이터 신뢰성 · BE+FE 배포됨 · 미커밋)**

  사용자 요청: 외부 사용자·검색엔진 봇이 버그로 여기거나 데이터가 어긋나 보이는 곳을 전수 검증·개선. **방법**: nginx 경유 Googlebot UA로
  공개 22경로 SSR HTML(상태·제목·robots·canonical·H1·본문) + 공개 도메인을 비로그인 브라우저로 열어 콘솔·4xx + API↔DB 대조.
  **BE(배포됨, api + worker-scores)**: ① `GET /auth/session` — 비로그인도 200(`refreshable`), 첫 방문자 콘솔 401 두 줄 제거용(FE가 써야 효과).
  ② 장 닫힌 시세의 시가·고가·저가가 **가짜**(시가=전일종가·고가=저가=현재가·거래량 0, 또는 0 → 화면 「—」): 세션 날짜를 벽시계 「오늘」로
  잡던 것 → `priceSessionDate` + 닫힌 세션은 **확정 일봉**(`closedSessionDailyStats`), 캐시 빈 첫 요청도 일봉으로 즉답. 실측 삼성전자
  282,500/286,500/280,750·3,204만 주, AAPL 거래량 6만→2,164만. ③ 순위 등락률 ≠ 종목 화면(삼성 +3.62% vs +3.24%): 마감 중 순위만
  토스 basePrice(NXT 기준) → 일봉 직전 종가 우선(장중 오버레이와 같은 규칙). ④ 신규 상장이 순위에 코드로(「0010S0」): 카탈로그 추가는
  화면 열람 때뿐 → 순위 폴러가 카탈로그 밖 종목을 `Stocks`로 등록(6시간 재질의 억제). ⑤ 사이트맵엔 있는데 noindex(URBN·TGTX): 목록 교체
  직후 FE 캐시 어긋남 → 페이지 판정은 최근 3시간 옛 목록까지 합집합. ⑥ 남의 뉴스가 종목의 「최근 변화」(NVDA에 코어위브 상향·팔란티어
  하향·배당주 3선): 태그=본문 언급 전부 → 사건은 **제목이 그 회사를 부를 때만**(한글명·영문 핵심어·티커). 변화 감지 수동 실행에 `params.date`.
  **FE(후속 요청에서 배포됨)**: `/auth/session` 사용 · 비로그인 스탬프 401 제거 · `/news`·`/earnings` HTTP 307(`redirect()`는
  200+빈 셸이었다, 페이지 파일 삭제) · 홈 「09-24 종가 기준」→ `priceAsOf`(추석 휴장) · 종목 요약 줄 종가에 날짜 · 종목 H1 두 개→sr-only 하나
  (서버가 아는 이름 `initialName`, 끼워진 곳은 h2) · 공개 9화면 sr-only H1(`PageHeading`).
  **후속(같은 날, 「추가 개선·남은 것 진행」)**: ⑦ 마감 중 「정규장 종가」 줄이 사라졌다(오늘의 정규장 구간만 찾음) → 닫힌 세션은
  확정 일봉 종가(하이닉스 현재가 1,862,000 · 정규장 종가 1,863,000 — SEO 줄과 맞음). ⑧ 52주 범위가 봉 400개(1.6년)를 훑었다(하이닉스
  최저 162,700 → 335,500 근처) → `week52RangeFromBars`가 최근 봉 기준 52주만, 네 군데 계산을 이 함수 하나로. ⑨ 종목 설명 160자·제목 45자
  상한(`stockPageTitle` · 기사 제목만 줄임). 실측: 비로그인 콘솔 4xx 0(`/auth/session` 200), `/news`·`/earnings` 307, 공개 화면 H1 하나.
  **사용자 몫**: 이미 저장된 9/23·9/24 변화 행 재생성 — 큐에 `change_detect` + `{"date":"2026-09-23"}`
  · `{"date":"2026-09-24"}`(DB 삽입이 권한 분류기에 막힘; 관리자 화면 수동 실행은 최신일만).
  **남긴 것**: 짧은 한글명 오탐(DAVE 「데이브」가 인명 「데이브 마자」 기사에 걸림) · 번역 오류(「111월」) · 없는 종목 주소 200(noindex+클라이언트 「찾을 수 없음」이라 두었다) · 설명 160자 초과 86건(info).
  검증: go build·vet·`go test ./internal/...` 통과(새 시험: SEO 유예 2 · 순위 카탈로그 1 · 뉴스 사건 귀속 2 · 요약 문장 1) · FE tsc 0(삭제한 두 페이지의
  낡은 .next 타입만) · vitest 410 · 공개 도메인에서 삼성전자 시가·고가·저가 표시 확인(브라우저 패널 숨김이면 「시세 로딩 중」 — 메모리의 그 함정).
- **09-26 불필요 코드 정리 실행 (사용자 승인 · BE만 · 미배포 · 미커밋)** — deadcode 도달 불가 함수 ~115개 제거(백업 scratchpad
  round1/2.tar), 미참조 `internal/mock` 패키지, 일회성 cmd 5개(repair-candle-tz·cleanup-holding-trades·tzcheck·probe-toss-ws·
  verify-financials-cohort) + 빈 cmd 폴더 9개, 매매 이력 가격 불일치 정리 잔재(상수·타입·배선 시험), be 잡파일(exe·bin·logs·tmp 탐색 스크립트).
  go build·vet·`go test ./...` 통과. **남은 것**: `ranking_snapshot_entries` DROP(분류기 차단 — 덤프는 `be/backups/ranking_snapshot_entries_20260926.sql.gz`,
  `/rankings/history`·prune 루프도 그대로), quotes_enrich.go·quote_change.go의 3함수(다른 세션 편집 중), FE 미참조 18파일·`cmdk`/`next-themes`,
  `metrics.CacheHit/Miss` 배선 결정, BE 재배포(`restart.ps1 -ApiOnly -Background`는 도는 배치 확인 후).
- **09-26 불필요 코드·DB·캐시 전수 점검 (조사)**

  DB 72표 전부 Go에서 참조됨(0행 표도 살아 있는 기능 — 드롭 대상 없음). `deadcode`(api+워커 9개 기준) 도달 불가 함수 ~130개,
  그중 116개는 시험에서도 안 불림. 죽은 경로 예: 옛 랭킹 캐시 V1(`market_rankings_cache.go`), 지수 직접 fetch(`market_index_store_fetch.go`),
  리더 번역(`news_reader.go` 5개), 계좌 연동 잔재(`toss/redis_token_store.go` 전체 · `WithAccount` · `postJSON`/`delete`).
  **`metrics.CacheHit/CacheMiss`를 아무도 안 불러 `/metrics`의 접두사별 캐시 적중 지표가 늘 비어 있다.** 일회성 도구 후보:
  repair-candle-tz · cleanup-holding-trades(+배선 시험) · tzcheck · probe-toss-ws · verify-financials-cohort, 빈 cmd 폴더 9개.
  **남긴 것**: backfill-earnings-*(v0.10.0에서 「새 환경 부트스트랩 유일 수단」 결정), probe-toss-coexist(TOSS_TOKEN_DESIGN 도구), compare-translation.
  FE 미참조 18파일(ui/command·label·separator·tabs, mock-fundamentals, quotes-mock-data, stock-detail-client 등) + 미사용 의존성 `cmdk`·`next-themes`.
  Redis: `news:retag:v1~v8` 16키(현행 v9, TTL로 자연 소멸), TTL 없는 키는 의도된 `md:rev:*`뿐. `.env`: ENCRYPTION_MASTER_KEY·AUTH_ALLOW_GLOBAL_TOSS·PAPAGO_* 미사용.
- **09-26 재방문 개선 (랭킹 품질 하한 · 가입 직후 온보딩 · 홈 「내 종목」 카드 · BE+FE · 배포됨 · 미커밋)**

  사용자 요청 「한 번 쓰고 마는 게 아니라 계속 쓰게」 → 조사 후 제안 A·B·C·D 중 **A·B·C 승인**(D 아침 이메일은 발송 서비스·방침 결정 대기).
  발견: ① 첫 화면 「AI 추천 랭킹」 1위가 동전주·SPAC(해외 상위 20 중 10종목이 하루 거래대금 $1M 미만, AESP·WTG SPAC; 국내 12종목이 2억 원 이하)
  ② 가입 → 홈인데 홈이 비로그인과 똑같음, 0종목 안내 없음, 가입 링크들이 `?next=`를 버림 ③ 이메일·푸시 기반 없음(재방문 신호가 앱 안에만).
  **A** `stock_score_universe.go` `scoreRecommendableClause`(사용자 「중간」): SPAC(영문 `ACQUISITION`·국내 `스팩`) · **미국 우선주**(한글명 `우선주`,
  ADR 제외 — BMNP가 보통주 주식 수로 시총 576억$ 잡혀 2위였음; 영문 PREFERRED는 PFBC 은행 오탐) · 30일 평균 거래대금 5억원/$2M · 시총 500억/$3억
  (주식 수 모르면 거래대금만). 적용: 실시간 TopScores · 사전계산(RankableSymbols·DisplayRows → 랭킹·이력 스냅샷·가격 시나리오) · 내 기준 랭킹 · 스크리너 자격.
  **안 건 곳**: 변화 감지·성적표(IC)·수급·SEO·Q&A. 값은 점수 행 `inputs.avgTradeValue`(`StockScoreRepo.FillAvgTradeValue`, 키 없는 행만 채움 —
  점수 배치 끝 + `ComputeAll` 시작). **키 없는 옛 행은 통과** → 과거 스냅샷·백테스트 소급 변경 없음. 점수 자체 불변이라 `scoreMethodVersion` 안 올림.
  실측 09-24 자격: 국내 2,767→1,044 · 해외 6,780→2,250. 새 1위 국내 파미셀·프로텍, 해외 BMNR·노바백스·퀀텀 컴퓨팅.
  **B** 가입 → `/welcome?next=`(인기 국내 12·해외 12·ETF 4 칩 + 검색, 순서대로 담기 — 담을 때 서버가 시세 조회라 병렬 금지) → next로 복귀.
  `lib/auth-routes` `safeNext`/`signupHref`/`welcomeHref` — **로그인 `next`의 열린 리다이렉트(`//host`) 막음**. 로그인 화면·관문 3곳 가입 링크가 next 실음.
  `/welcome` noindex(page-meta-routes·robots). **C** `GET /user/today`(`repository/my_today.go`, handler `my_today.go`): 관심·보유 합집합(보유 먼저, 최대 40) ·
  점수 두 날 차이 · 24h 기사 수(**날짜 인덱스 FORCE** — MV 인덱스로 가면 인기 종목 0.99초 → 0.13초; 해외는 번역된 것만) · 오늘 변화 대표 한 줄.
  응답 13~15ms. FE `dashboard/my-stocks-card.tsx` 홈 맨 위(로그인만, 비로그인 첫인상 불변) — 가격은 `useLiveQuotes`, 실적 D-n(14일 안)은 `useUpcomingEvents`
  재사용. 줄 순서 = 사건 가중치 → 서버 순서(**등락률로 안 세움** — 틱마다 줄이 도망). 0종목이면 「관심 종목 고르기/보유 종목 등록」.
  옛 `MyChangesStrip` 띠는 이 카드가 대체(파일은 미추적이라 남겨 둠 — 안 쓰임, 지워도 됨).
  검증: go test repository/service · vitest 48파일 409 · tsc 0 · 새/수정 파일 eslint 0 · BE(전체 1회 + API만·worker-scores만) · FE 재배포 · Tesseract OK ·
  `compute-score-rankings` 재실행. 브라우저 실측: 비로그인 홈 카드 없음 · `/login?next=/watchlist` 가입 링크 next 유지 → 테스트 계정(`@quant-port.test`) 가입 →
  `/welcome?next=/watchlist` → 칩 4 + 검색 1 → 5개 담고 `/watchlist` 복귀 → 홈 카드(삼성전자 70▲1·뉴스 46·변화 한 줄, 테슬라 52▼1) · 행 클릭 모달 ·
  모바일 375 · 0종목 빈 상태. **발견(별도 작업 칩)**: 워커 재배포 후 뉴스 브리핑이 **이미 끝난 18:00 슬롯을 다시 생성**(LLM 중복 호출).
  **남은 것**: D(아침 요약 이메일 — 발송 서비스·개인정보처리방침), 비로그인 홈의 가입 유도 한 줄은 범위 밖으로 둠.
- **09-26 (캐싱·인덱스 전수 점검 — 배치 큐 폴러 통합 · 공개 API 엣지 캐시 헤더 · BE · 배포됨 · 미커밋)**

  사용자 요청: 캐싱·인덱스 등에서 큰 개선점을 찾아 개선. **먼저 잰 것**: api 로그 12h 경로별 p95 대부분 <40ms, nginx `/stocks/:s`
  p50 29ms, Redis 적중 94%·28MB, 배치 DB 비용 작음(점수 계산 25초), 큰 표 인덱스는 쿼리 패턴과 맞음. ⚠️ `quantport` DB 사용자는
  performance_schema 권한이 없다 → `SHOW GLOBAL STATUS` 델타(Com_*·Connections)로 쟀다.
  ① **배치 큐 폴러** — 잡마다 4초 폴러 31개가 틱마다 `BEGIN → SELECT … FOR UPDATE SKIP LOCKED → ROLLBACK`: 유휴에도 **초당 트랜잭션
  7.75 · SQL 31 · 새 연결 1.27**(한 프로세스 폴러들이 동시에 깨어 워커 풀 유휴 상한 2를 넘겨 매 틱 연결을 열고 닫았다, 연결마다 SET 2회).
  → 프로세스당 폴러 하나(`batchQueueHub`)가 `QueuedJobIDs`(잠금 없음, 커버링 인덱스 0.06ms) 한 번으로 확인하고 대기 잡만 기존
  `ClaimNext`로 집는다. 같은 잡 겹침 없음 · 다른 잡 동시 · 종료 시 실행 끝까지 대기 유지(시험 3). **실측 SQL 31→4.9/초 · BEGIN 7.75→0 ·
  새 연결 1.27→0.02 · SET 2.5→0**. `candle_rollup`을 큐에 넣어 1틱 안 픽업·완료 확인.
  ② **공개 API 엣지 캐시** — 오리진 2~10ms인데 공개 도메인은 요청마다 ~270ms(전부 `DYNAMIC`), 저녁엔 1~16초. `middleware.EdgeCache`로
  공개 조회 46경로에 `public, max-age=0, s-maxage, stale-while-revalidate`(4등급). **개인화 차단은 코드가**: 처리 중 `authctx.UserID`를
  한 번이라도 부르면 `private, no-store`(`authctx.IdentityProbe`) — 실제로 `/sample/journal/notes`·`/sample/price-targets`가 걸렸다.
  **사용자 몫: Cloudflare 캐시 규칙 1개**(docs/EDGE_CACHE.md — `/api/v1/` Eligible + Edge TTL 「cache-control 있으면 따르고 없으면 우회」).
  규칙 전에는 효과 0(확인: 헤더는 CF 통과, 여전히 DYNAMIC).
  **기각·보류**: DSN `interpolateParams`(드라이버 v1.10이 `[]byte`를 `_binary'…'`로 넣어 JSON 컬럼 쓰기가 깨진다), `financial_periods`
  중복 인덱스(유니크 키와 같은 컬럼, 7MB — 이득 작고 인덱스 제거 마이그레이션은 전용 Go 배선이 필요).
  검증: go build · vet · `go test ./...` 전부 통과 · BE+워커 재배포(Tesseract OK).
- **09-25 후속4 (공개 도메인 간헐 멈춤 — Cloudflare ↔ 오리진 구간 · nginx만 · reload · 미커밋)**

  성장 에이전트 세션 제보: 공개 도메인 요청이 간헐적으로 8~30초·524, nginx에는 흔적이 없음. **내가 앞서 잰 것은 전부 localhost였다** —
  실사용자 경로는 브라우저 → Cloudflare(엣지 **HKG**) → 공인 IP(이 PC가 **218.233.8.175를 직접** 가짐, 공유기 NAT 없음) → Docker Desktop
  포트 중계 → nginx. 실측(20:55~21:00, 20초 간격 16회, 고유 쿼리): **9/16이 1~16초**, 그런데 16건 모두 nginx에 한 번씩 도착해 **3~5ms**에 처리.
  클라이언트→엣지 연결은 늘 ~50ms. 오리진 직접(동시 60)은 11~29ms. 업로드 55KB/s(포화 아님). 호스트 TCP 재전송이 CF 연결당 ~1씩 증가 →
  **엣지 ↔ KT 구간 손실**(2·3초 = SYN 재전송 간격, 30초 = 1+2+4+8+16). CF 엣지 ping은 손실 0 — 저녁 시간대 간헐.
  **조치(코드로 가능한 것)**: ① nginx keepalive 65s → **905s**(CF 오리진 유휴 900초보다 길게) + keepalive_requests 10000, 세 nginx*.conf 모두 —
  새 연결(손실 구간에서 SYN 재전송을 맞는 순간) 수를 줄인다. ② Next public 파일이 전부 `max-age=0`이라 CF가 매번 오리진에 재검증하러
  왔다 → `/logos/` 하루 · robots·sitemap·favicon 1시간 + stale-while-revalidate(두 app_routes). 로고·robots·favicon **CF HIT 확인**.
  **남은 것(사용자 몫 — CF 계정 필요)**: sitemap.xml은 CF가 XML을 기본 캐시 안 해 DYNAMIC(캐시 규칙 필요). HTML·API는 여전히 손실 구간을
  건넌다 — 근본책은 **Cloudflare Tunnel**(오리진이 밖으로 상시 연결, 들어오는 SYN 없음) 또는 대시보드의 HTTP/2 to Origin. 사용자 결정 대기.
- **09-25 성장 감시 (검색 유입·활성화 감시 컨테이너 worker-growth · GA4 태그 · 관리자 보고서 화면 · BE+FE 배포됨 · 미커밋)**

  사용자 결정: 붙여 준 「Growth Agent」 스펙을 **AI 에이전트가 아니라 코드만으로 도는 별도 컨테이너**로. 미커밋 변경은 **손대지 않음**
  (그래서 PR 생성 대신 보고서가 「고칠 자리」를 적는다). GA4는 사용자가 준 측정 ID `G-T8RNF91MRH`로 설치.
  **worker-growth** (`be/internal/growth`, `cmd/worker-growth`, 표 `087_growth.sql` 다섯 개, 잡 `growth_audit` 매일 07:30 KST ·
  감시 기대 · catch-up 허용): 사이트맵 1,013 URL 크롤(robots·noindex·canonical·제목/설명·H1·서버 본문·응답·OG·JSON-LD·들어오는 링크,
  어제와 비교해 새/해결 문제) → Search Console(합계 · 검색어×페이지 · URL 검사 하루 200 · 사이트맵) · GA4 Data API · Bing(자격 증명
  있을 때만, 없으면 「미설정」) → DB 활동(가입·로그인·첫 보유·종목 추가·리포트 — **개수만**) → 규칙 분석(문턱 검색어 · CTR 부족 · 종목
  검색 수요인데 noindex · 종목 페이지 미노출 · 의도별 콘텐츠 후보 · 실적 예정 · 내부 링크 · 유통 후보) → 일간 보고서, 월요일 주간(Top 20).
  보고서: `be/var/growth-reports/*.md` + `GET /admin/growth/reports[/{id|latest}]`. LLM 0. **쓰기는 growth_* 표뿐** — `store_guard_test.go`가
  소스의 쓰기 문장 대상을 센다. Google 인증은 표준 라이브러리 JWT(의존성 없음). 설정은 `docs/GROWTH.md`(서비스 계정 JSON →
  `be/secrets/`, `.gitignore`·`be/.dockerignore` 처리).
  **첫 실행에서 잡은 것**: ① 가입 293 중 **292가 e2e 계정(`@quant-port.test`)** — RFC 2606 예약 도메인 제외 규칙 추가, 28일 매번 재집계.
  ② 공개 주소가 **간헐적으로 8~30초 멈춤 · CF 524**(nginx 로그엔 요청 자체가 없음 = 오리진 앞 구간) — 호스트에서 사이트맵조차 못 받았다.
  → 페이지 크롤은 **nginx 직접**(`GROWTH_CRAWL_ORIGIN=http://nginx`, p95 28ms), 공개 경로는 **도달성 탐침**(5곳×3회, 재시도 없음)으로 따로 잰다
  — 20:55 실측 **15회 중 3초 초과 8 · 실패 1**(sitemap 30초). 원인 미조사, 성능 세션에 전달. **라이브 사용자·Googlebot이 겪는 문제.**
  실측 발견(진짜): **종목 페이지 1,000/1,000이 어디서도 링크되지 않음**(허브 SSR 링크 0), 종목 페이지 H1 두 개가 둘 다 심볼만(「005930」),
  약관 2페이지 고아, `/earnings/domestic` 본문 얇음. 오류(error)는 0.
  **후속(사용자 「권장 방향으로 모두 진행」)**: 개인정보처리방침 1·2·4·5항에 GA 문안 적용(시행일 09-25), FE 재배포(`fe/scripts/deploy.ps1`
  — api도 같이 재생성됨, health·네트워크·Tesseract 확인). 브라우저 실측 gtag 로드 + `g/collect` page_view 전송(이 확인 방문 1건이 GA에 들어감).
  관리자 **`/admin/growth`** 화면(주간/일간 목록 + 본문, 자체 Markdown 렌더러 — innerHTML 없음, vitest 4). 로그인 없이 열면 홈으로 튕김 확인,
  렌더러는 실제 보고서 두 장으로 확인(표 5·6개). 관리자 로그인 상태 화면 확인은 못 했다(비밀번호 입력 불가).
  **남은 것(사람만 가능)**: Google 서비스 계정 JSON(`be/secrets/`) · `GROWTH_GA4_PROPERTY_ID` · `GROWTH_BING_API_KEY`, GA 내부 트래픽 필터,
  개인정보처리방침 7항 운영자 정보, 공개 경로 손실(성능 세션 진단: CF 에지 ↔ KT 회선 구간, Tunnel 제안).
  검증: `go test ./internal/growth`(13) · service/handler/repository 통과 · tsc 0 · 새 파일 eslint 0 · BE·FE 재배포 · 컨테이너 첫 실행 1,013/1,013.
- **09-25 후속3 (후속2의 남은 것 전부 — 배치 끊김 · 종목 첫 진입 · 스크리너 · 종목 뉴스 예열 · 배포 스크립트 · BE · 배포됨 · 미커밋)**

  ① **경고·섹터 배치가 「멈춘」 것이 아니라 재배포로 죽고 원장에 running으로 남았다.** 09-23·24 경고 스윕은 하루 **2종목**만
  처리됐고, 09-25는 01:29 경고·02:29:59 섹터가 **02:31 워커 재기동**(새벽 재배포)에 죽은 뒤 16시간 running이었다. running 행을
  닫는 `MarkStaleRunsFailed`는 **API 기동 때 · 6시간 넘은 것만**이라 만회(batch_catchup)가 「돌고 있다」로 보고 건너뛰었다.
  DB 대기 가설은 기각했다(DSN 기본 `readTimeout=30s` — 워커 쿼리는 30초 넘게 못 선다. 서버 `lock_wait_timeout`은 1년).
  → `BatchJobRunRepo.MarkInterruptedRuns` + `service.CloseInterruptedRuns`: **워커가 기동하면 자기 잡의 running 행(기동 전 시작)을
  나이와 상관없이 닫는다**(interrupted 표식 → 워치독 만회가 15분 안에 다시 넣음). 잡↔워커 소유는 겹치지 않음을 확인(8개 워커 전부 적용,
  worker-filings는 `slotJobs()`). **실전 확인**: 20:35 다른 세션(성장 에이전트)의 전체 재빌드가 18:59부터 돌던 경고 스윕을 끊었는데, worker-filings 기동 로그에 「재기동 전 끊긴 실행 2건을 닫았다」가 찍히고 **같은 초에 만회 실행(4774)이 시작**됐다.
  ⓑ 잠복 결함도 막았다: Toss 토큰 발급 `issue()`가 `http.DefaultClient`(무기한) + 배경 갱신 `context.Background()` — 걸리면
  `refreshing`이 안 풀리고 같은 프로세스의 모든 Toss 호출이 ctx를 안 보는 `Cond.Wait`에서 영원히 선다. → 발급 전용 클라이언트 15초 상한.
  ② **종목 첫 진입(terminal-snapshot 캐시 없음)**: 시세→호가→체결을 **차례로** 불렀다 → 셋을 동시에(`refreshTerminalSnapshot`). 호출 수 동일.
  실측(미국 프리마켓, 처음 여는 6종목): 125~233ms(전: 272~540ms), AMD 1건만 1.27초(Toss 응답 지연 — 한도 로그 없음).
  ③ **스크리너**: 결과(시세 덮기 전)를 거래일+정규화 조건 키로 15분 메모리 캐시(128개 상한), 시세는 요청마다 덮는다.
  조건 목록(`/screen/fields`)은 SWR. 실측 국내 267ms → **4~6ms**, 조건 목록 171 → 3ms. 해외 적중 ~110ms는 프리마켓 실시간 가격 조회
  (`LiveChanges` — 랭킹과 같은 규칙)라 그대로 둠.
  ④ **해외 종목 뉴스 적재 직후 ~230ms**: 예열 대상을 최근 10분 안에 본 **종목 목록**까지(자유 검색어 제외, 최근 16개 상한 · `recentWarmFiltersLocked`).
  ⑤ **restart.ps1**: compose 구간만 `ErrorActionPreference=Continue` + 줄을 문자열로 풀고 종료 코드로 판정(NativeCommandError 해결 — 실행 확인).
  `-ApiOnly` 추가(워커를 다시 만들면 도는 배치가 죽는다). ⚠️ **배포 중 API 컨테이너가 네트워크 없이(`Networks {}`) 떠서 MySQL
  unreachable로 재시작 반복 — 약 2분 중단**, `--no-build --force-recreate api`로 복구. 재발 대비로 스크립트가 health 실패 시 한 번 그렇게
  재생성하고 다시 기다린다. 참고: `-Background` 없이 돌리면 끝에 `docker logs -f`가 붙어 안 끝난다(설계대로).
  ⚠️ 이 배포 중 API가 **`087_growth.sql`(검색 유입 감시)** 마이그레이션을 적용했다 — 이 세션의 변경이 아니다(작업 트리의 다른 미커밋 작업).
  **검증**: go build · vet · service/handler/repository/toss 시험 통과(새 시험: 스크리너 키·상한, 예열 대상 종목 포함·상한). `service`의
  `TestIntegrationToss_acceptsUnder100InSingleDeclare`가 가끔 실 Toss WS 401로 실패 — 외부 통합 시험, 이 변경과 무관.
- **09-25 후속2 (「서비스 전체가 느리다」 — 컨테이너 로그 분석 · 대기 넷 제거 · BE · 배포됨 · 미커밋)**

  **먼저 잰 것**: nginx(36h)·api(16h) 로그의 경로별 p50/p95. 서버 자체는 대부분 빠르다(`/` p95 30ms, `/quotes/batch` 6ms, 자원 여유 —
  Docker VM 15.5GB 중 ~10.5GB, CPU 24코어 유휴). 느린 것은 **전부 「캐시가 비었을 때 첫 요청이 재구성을 동기로 기다리는」 모양**이었고,
  방문 간격이 캐시 수명보다 길어 **사실상 매 방문이 그 첫 요청**이었다 → 「사용자가 적은데 전체가 느리다」와 맞는다.
  ⚠️ 컨테이너 로그 시각은 **UTC**다(KST = +9h).

  ① **`/earnings/highlights` 14건 전부 2.8~3.1초** — 30분 TTL 하나, 만료 시 세 질의 동기. → SWR: 「신선 30분 · 사는 시간 36h」
  봉투(`{day, builtAt, data}`, 키 `earnings:highlights:v3:{market}`), 낡으면 주고 `runOnce`로 뒤에서, 날짜가 바뀌어도 낡음 처리.
  실측: 냉시동 3.6초(키 없음, 한 번뿐) → builtAt을 31분 전으로 되돌려 강제한 낡음 상태에서 **3~5ms**, 5초 안에 builtAt 갱신 확인.
  ② **뉴스 대시보드 해외 p50 243ms · 국내 p95 1.7초 · 두 시장 동시 5.0/5.2초** — 폴러(2분)가 적재마다 그 시장 목록 캐시를 버려 다음
  방문자가 동기 재구성(5초는 doSync 대기 상한). → 버리기 직전 **최근 10분 안에 만든 canonical 목록만** 적재 직후 뒤에서 다시 만든다
  (`warmListsAfterIngest`, 재구성 코드는 `listBuilder` 하나로 — 요청 경로와 공유). 검색어·종목 키는 예열 안 함, 아무도 안 보면 예열도 멈춤.
  ③ **보유 화면 종목별 뉴스 1.7~2.0초**(SQL은 6ms) — 프로파일로 갈랐다: 해외 목록 100건 중 7건이 `lookupUSSymbolByName`에서
  `stocks.Search`로 DB에 감(건당 ~38ms, 합 267ms, 답은 거의 「없음」 — "Fed's" 등). → 이름별 결과 메모(**없음 포함**, 조회 오류는 안 기억,
  이름 인덱스 세대와 같이 비움). 그리고 **이름 인덱스가 30분마다 재구성되며 내용이 같아도 세대를 올려** 메모(본문→심볼)를 통째로 비웠다
  → `maps.Equal`로 바뀐 때만 세대 증가(국내·해외 둘 다). 해외 목록 재구성 536ms → 메모가 살아 있으면 recordsToArticles **1.5ms** + 필터 41ms.

  ④ **캐시가 살아 있는데도 대시보드 1.07초** — ①~③ 배포 후 8분 관측에서 남은 스파이크. 느린 재구성 로그(아래)엔 안 잡혀 재구성이 아니라
  **잠금**이었다: `mergeArticlesForMarket`이 st.mu **쓰기 잠금 안에서** 합치기+dedupe(기사마다 제목 정규식)를 했다 — 실측 해외 1,000건
  **780ms**·국내 461ms(데스크톱). 그동안 목록 캐시를 읽는 모든 요청이 RLock에서 섰다(폴러 2분 주기). → 스냅샷(RLock) → 잠금 없이
  `mergeNewsSnapshot` → 짧은 Lock으로 교체. `mergeMu`는 합치기끼리만 줄 세움. 스냅샷이 통째로 바뀌었으면 잠금 안에서 다시 합치고,
  자리 수정(조회수·이미지·번역)은 `carryInPlaceNewsEdits`가 옮긴다.
  **진단 로그 추가**: `news list rebuild slow: … total db convert filter`(500ms 초과만).
  **최종 실측(8분 · 10초 간격 · 적재 여러 회 포함)**: 대시보드 국내·해외 **최대 5ms**(전: 해외 1.8초·1.07초 스파이크), 국내 종목 뉴스 최대 75ms,
  해외 종목 뉴스는 해외 적재 직후 1회 ~230ms(검색어·종목 키는 예열 안 함 — 다음 후보). 브라우저 `/holdings` 종목 뉴스 1.7~2.0초 → **184~198ms**.

  **손대지 않은 것**: `terminal-snapshot` 첫 진입 ~300ms(기록된 설계 — 줄 것이 없을 때만 기다림), `POST /stocks/screen` ~300ms(집계 3회, 호출 드묾),
  batch watchdog 경고 둘(`sector_sync` 09-24 이후 · `warnings_sync` 09-22 이후 성공 없음 — 속도와 무관, 별건).
  **검증**: go build · vet · `go test ./internal/service ./internal/handler ./internal/repository` 통과(새 시험 4: 레일 신선도 · 예열 대상 선별 · 자리 수정 옮기기 · 스냅샷 동일성)
  · BE 재배포(restart.ps1은 PS 5.1 NativeCommandError로 죽어 같은 compose 명령을 Bash로) · `/health` ok · **Tesseract ready**.
- **09-25 (시장 브리핑 자정 이후 규칙 문장 — 원인 · 개선 · BE · 배포됨 · 미커밋)**

  **원인**: 주 모델 gemini-3.1-flash-lite가 **매일 KST 22:00~03:00(미국 낮)에 503 「high demand」**로 몇 시간씩 막힌다
  (09-23 · 09-24 두 밤, 국내·해외 두 키 모두 → 키가 아니라 모델 용량). 슬롯마다 두 번(4초 간격)만 묻고 다음 슬롯(장 밖 1시간)까지
  기다렸다. 22~23시는 직전 AI 문장 승계로 가려졌지만 승계는 **3시간 창 + 서울 날짜가 같을 것**이라 00:00에 두 시장 모두 규칙으로.

  **개선 (사용자 결정: 1~4 전부 + 무료 대체 모델)**:
  ① **슬롯 안 재시도** — AI 문장 없이 끝난 슬롯은 **5분마다** 다시(장 밖 슬롯 11회 · 장중 5회, 후속 요청으로 15→5분). 실패한 재시도는 행을 쓰지 않는다
  (`upgradeOnly`). 한도 초과는 재시도 안 함. 원장에는 슬롯 첫 실행만. `briefingRunState.due/settle`.
  503은 우리 예산에 안 잡히지만 Google이 세는지는 문서에 없어 **시장별 브리핑 날당 60회 상한**(`briefingRetryDailyCap`). 날은
  서울 자정이 아니라 `briefingDay`(개장 준비 시각)로 센다 — 자정은 503 구간 한가운데라 한 밤에 상한을 두 번 쓰게 된다(시험이 잡음).
  Google이 503을 세는지는 AI Studio 사용량(모델별 요청 수)과 우리 성공 수를 하루 비교하면 알 수 있다.
  ② **승계의 날 경계 = 시장의 개장 준비 시각**(국내 08:00 KST · 미국 08:30 ET, `briefingDay` + 새 `calendar.SessionOpenAt`). 자정 아님.
  ③ **국내 장 밖끼리는 3시간 창 없음**(마감 뒤~08:00, 지수 불변). 해외는 3시간 그대로. 승계 탐색 깊이 8→16.
  ④ **조용한 슬롯 건너뛰기는 직전 행이 AI 문장일 때만**(규칙·승계 행이면 새 기사 없어도 다시 묻는다; LLM 꺼짐이면 예전대로).
  ⑤ **503 대체 모델** — 브리핑만 opt-in(`chatCompletionWithFallback`): 주 모델 503 → 재시도를 **대체 모델**로(시도 수는 그대로 둘).
  기본 `gemini-3.5-flash-lite`, `OPENAI_FALLBACK_MODEL`(`-`면 끔). 무료 키(결제 미연결)라 청구 없음, 한도는 모델별. 종목 요약·번역은
  같은 모델 재시도 그대로. ⚠️ **gemini-2.5-flash-lite는 ListModels에 나오지만 호출하면 404 「no longer available to new users」**
  (첫 배포에서 실측 → 3.5로 교체).

  **실측(01:12~01:15 KST)**: 국내 21:00 문장이 01:12에 승계됨(예전 규칙이면 00:00부터 규칙) → 01:14 재기동 실행에서 주 모델 성공, AI 문장
  복구. 해외는 주 3.1 · 대체 3.5 **둘 다 503**(01:15) → 00:30 문장 승계 + 15분 뒤 재시도 예약. 무료 키는 혼잡 시간에 모델을 바꿔도
  같이 막힐 수 있다 — 재시도가 틈을 잡는 쪽이 주 효과일 수 있다. 며칠 밤 `news briefing .*fallback` · `retry recovered` 로그로 확인할 것.
  남은 것: 카드 배지는 승계 시각을 「AI 21:00」처럼 시각만 적는다 — 자정을 넘긴 승계면 「어제 21:00」이 더 정확(FE, 미착수).

  **검증**: go build · vet · test 전체 통과(새 시험: 승계 경계 13 · 재시도 상태 3 · 대체 모델 2) · BE 재배포 2회(Tesseract OK).

- **09-24 후속2 (보류 항목 — 종목 페이지 SEO · 규칙 기반 종목 Q&A · 바꿔 보기 · BE+FE · 배포됨 · 미커밋)**

  사용자 결정: SEO는 **상위 1,000종목만 색인**, Q&A는 **종목 개요 질문 칩**(자유 입력 없음), what-if는 **위험 카드 안
  「바꿔 보기」**, 알림용 사용 지표는 **보류**. 막힌 채 둔 것: 국내 컨센서스(FnGuide는 확정 행뿐 — 예정 행 0 실측 · 라이선스),
  어닝콜(원문 소스 없음), 가중치 적용 · 미국 EPS 리비전 축 편입(표본 시간), 웹 푸시·이메일(지표 뒤).

  **① 종목 페이지 SEO.** `GET /seo/stocks`(색인 대상) · `GET /seo/stocks/{symbol}`(서버 렌더 요약 + indexable). ⚠️ 잣대를
  **시가총액이 아니라 최근 20거래일 평균 거래대금(원화 환산)**으로 바꿨다 — 저장된 `inputs.marketCap`이 ADR 등에서 부풀어
  (아카리 9,250억 달러 · 씨티 3.8조 · NXP 1조) 상위 30에 소형 바이오가 섰다. 시가총액 오류는 별도 작업 칩으로 뗐다(스크리너
  시총 조건 · 피어 크기 구간도 같은 값을 읽는다). 목록 쿼리 차가울 때 3.7초 > FE SSR 제한 3초라 **기동 때 미리 채우고
  만료되면 낡은 목록을 주며 뒤에서 갱신**(실측 재기동 직후 2ms). FE `stocks/[symbol]/page.tsx`: 상위 종목만 index +
  canonical(대문자 심볼) + 종목 이름 제목 · 요약 문장 설명, 나머지는 noindex 그대로. 서버가 **보이는 요약 한 줄**(이름 ·
  시장 · 섹터 · 점수 · 종가 · 최근 변화)을 페이지 위에 그린다(숨긴 글자 = 속임수). 사이트맵에 종목 1,000개(총 1,013).
  Yeti는 Next의 HTML 제한 봇 목록에 있어 메타가 `<head>`로 간다(스트리밍 아님).

  **② 종목 질문 칩.** 개요 점수 카드 아래 「왜 움직였나 · 점수가 왜 이런가 · 다음 실적은 · 비슷한 종목은」 — 누를 때만 부르고
  LLM 0. 새 `GET /stocks/{symbol}/qa`(마지막 등락 · 최근 30일 변화 감지 5 · 같은 섹터 비교 종목), 점수·실적은 기존 API.
  답마다 근거 줄. 비교 종목은 처음에 점수 상위였는데 삼성전자 옆에 소형주가 서서 **같은 섹터 안 시가총액이 가까운 순**으로
  (삼성전자 → SK하이닉스·삼성전기·LG전자…, 애플 → 엔비디아·알파벳·MSFT·TSMC). ⚠️ 「왜 움직였나」 첫 줄에 삼성중공업 기사가
  삼성전자로 붙어 나왔다 — 뉴스 종목 태깅 오탐이 변화 감지를 타고 올라온 것(「변화」 메뉴도 같음, 이번엔 손대지 않음).
  곁가지: 점수 한 문장의 조사 오류 수정(「추세(99점)이」 → 「추세(99점)가」, `subjectParticle` 받침 규칙 + 시험).

  **③ 바꿔 보기.** 위험 카드 접힌 줄 — 종목 검색 + 수량(음수 = 매도) → 그 종목 비중 · 가장 큰 섹터 · 변동성 · MDD · 베타의
  지금→바꾸면. 가격은 저장된 마지막 종가(달러는 저장 원/달러), 저장 없음, 권유 문구 없음. `applyWhatIf`·`sectorShares` 시험.
  실측(예시 보유): 현대차 +30주 → 비중 0→11.2% · IT 96→86% · 변동성 73.7→71.0% · 베타 1.01→0.97.
  ⚠️ 이 과정에서 위험 카드 결함을 잡았다: 불러오기 키에 **평가금액**을 넣어 실시간 시세 틱마다 종가를 다시 부르고 카드 속이
  새로 마운트됐다(펼친 「바꿔 보기」가 1초 안에 닫힘). 키를 종목 목록으로 바꿨다.

  **검증**: go vet·test 전체 · fe tsc 0 · vitest 399 · 새/바꾼 파일 eslint 0 · BE·FE 재배포(Tesseract OK). Yeti UA로 HTML 실측
  (005930 · AAPL index+canonical, 900110 noindex), 브라우저로 질문 칩 넷 · 바꿔 보기 실측.

- **09-24 후속 (1년 로드맵 일괄 — 수급 축 · 미국 EPS 리비전 적재 · 가중치 제안 도구 · 문장 스크리너 · 국내 ETF 점검 · 포트폴리오 위험 · BE+FE · 배포됨 · 미커밋)**

  사용자 결정: 자연어 스크리너는 **규칙 파서**(LLM 규칙 예외 없음) · 컨센서스는 **보유 데이터로 미국만**(적재만, 축 편입은 표본 뒤)
  · 수급은 **호출량 확인 뒤 진행** · 가중치 재설계는 **도구만** · 문장 입력은 조건판 맨 위 · ETF는 **국내, 종목 화면 + 랭킹 ETF 탭**
  · 위험은 **보유 페이지 새 카드**.

  **① 수급 축.** 종목별 매매동향은 `/stocks/{symbol}/investor-trading` = **`STOCK_TRADING_TREND` 버킷**(사용자 시세
  `MARKET_DATA`·경고 스윕 `STOCK`과 다름). 첫 응답 헤더로 **한도 초당 10** 실측(`toss.logFirstSeenLimit` — 그룹별 한도를
  처음 볼 때 한 줄). ⚠️ `groups.go`가 이 경로를 `/stocks` 접두사로 `STOCK`에 넣고 있었다 — 429가 경고 스윕 failover를
  보조 키로 넘길 뻔. 매매동향 다섯 경로를 별도 그룹으로(시험 포함). 적재 `investor_flow_sync`(worker-filings 20:30, 국내
  보통주 약 2,760 × 1호출 · 0.5초, 처음 보는 종목은 60일 한 호출, **확정치만** — 개인 값이 비면 잠정). 표 084
  `stock_investor_flows`(400일 보관). 축: 외국인+기관 20거래일 순매수 ÷ 같은 날 거래량 → 50+35·tanh(r/0.1), 가중 뉴스 0.6
  · 내부자 0.4 · 수급 0.55(상대값 — 수급 없는 미국은 **점수 불변**, 시험). 판 `2026-09-24` + 종목별 `|flow`(적재가 닿는 날
  자동 전환) + 성적표 changelog. ⚠️ 배포 직후 워치독이 새 잡을 「놓친 잡」으로 보고 **바로 전체 백필을 돌렸다**(이번엔
  원하던 일) — 새 예약 잡을 추가하면 첫 배포에 한 번 돈다는 것을 알고 넣을 것.

  **② 미국 EPS 리비전 적재.** 085 `earnings_estimate_revisions` — 06:00 해외 실적 일정 적재가 `eps_est`를 덮어쓰기 전에
  **값이 바뀐 날만** 남긴다(INSERT…SELECT 한 문장, 예정 약 3,600행 대상). 축 편입은 표본이 찬 뒤 결정.

  **③ 가중치 제안 도구(관리자).** `GET /admin/score/weight-suggestion?market=` — 유의(|t|≥2) 양의 IC 축 비례와 현재 값을
  반씩 섞고 축마다 최소 5, 합 100. **모든 축 90일 미만이면 거절**(현재 상태). 저장 없음 — 관리자 점수 화면 카드의
  「편집기에 채우기」 후 사람이 저장.

  **④ 문장 스크리너.** `lib/screener-nl.ts` 규칙 파서(필드 별칭 긴 것부터 · 이상/이하/초과/미만/넘는/~ · 조·억·만원 ·
  저평가·과매도·대형주 같은 말 → 칩 하나 · 시장 말). **못 알아들은 말은 회색으로 표시**(「반도체」를 조용히 버리지
  않는다). 조건을 갈아 끼우고 바로 찾는다. 시험 8.

  **⑤ 국내 ETF 점검.** KRX `etf_bydd_trd` 실측 필드(NAV · 순자산 · 거래대금 · 기초지수 · 지수 등락률), **다음 영업일
  공개**(09-24 03시에 09-23은 빈 배열). 086 `etf_daily_metrics` · `etf_scores`. `etf_score` 잡(worker-filings 10:00, 평일
  하루 1호출, 첫 실행 90일). 괴리율(20일) · 추적오차(60일, 베타 회귀 잔차, 3% 넘는 날은 분배로 제외, **lag 0/1 중 작은
  쪽** — 해외 지수 시차) · 유동성 · 규모를 집단(일반 / 채권·금리 = 기초지수 연 변동성 5% 미만 / 레버리지·인버스 = 베타>1.5 또는 음수) 백분위 평균.
  ⚠️ 첫 산출은 집단이 둘이라 「일반」 상위 5개가 전부 MMF·CD였다(원래 안 움직이는 상품) — 채권·금리를 떼자 KODEX 200TR ·
  금현물 · RISE 200. 총보수 없음. 실측: 64일 70,603행, 1,161종목(일반 907 · 채권·금리 116 · 레버리지 138).
  API `GET /stocks/{symbol}/etf-check` · `GET /etf/scores/top`. FE: 국내 ETF 종목 개요의 점수 자리에 「ETF 점검」(없으면
  원래 카드로), AI 랭킹 시장 탭에 「국내 ETF」(축·기간 손잡이 없이 집단 토글).

  **⑥ 포트폴리오 위험.** 보유 페이지 자산 배분 옆 「위험」 카드 — **현재 비중 고정** 가정의 연 변동성 · 최대낙폭 · 지수
  베타(원화 비중 ≥ 50%면 코스피, 아니면 S&P 500) · 상관 0.6 넘는 쌍 셋. 종가는 새 `GET /stocks/daily-closes/batch`
  (40종목 · 400일 상한 초과는 400) 한 번. 환율 미반영을 카드가 밝힌다. 시험 4.

  **검증**: go build·vet·test 전체 · fe tsc 0 · vitest 389 · 새 파일 eslint 0(기존 analysis-client 9건 · admin-score-panel
  1건은 이번 변경 전부터) · BE·FE 재배포(Tesseract OK). 실측: 수급 백필 2,641종목 · 158,165행 · 실패 0 → 재계산 뒤 수급
  성분 2,558종목, 심리·수급 축 7,341 → 7,854종목(순매수/거래량 절반이 ±5% 안, 8%가 ±20% 넘음). 추정치 첫 스냅샷 3,502.
  브라우저: 스크리너 「반도체 PER 15 이하 시총 1조 이상 ROE 10% 넘는 국내」 → 조건 3 · 「반도체」 표시 · 40종목. 069500 ETF
  점검 94점 · 「국내 ETF」 탭 목록. 예시 보유 위험 카드(변동성 73.7% · MDD −39.1% · 코스피 베타 1.01 · 삼성전자·하이닉스 상관
  0.88) — 저장 종가에 하이닉스 07-31 +26.4% 같은 큰 일간 변동이 있어 개별 연 변동성이 86~103%다(계산 오류 아님).
  ⚠️ 가중치 제안 API는 관리자 인증 뒤라 단위 시험만.

- **09-24 (우선주 랭킹 제외 · 점수 설명 한 문장 · 내 포트폴리오 vs 지수 · 매수 이유 태그 · BE+FE · 배포됨 · 미커밋)**

  사용자 결정: 우선주 **랭킹에서 제외**, 다음 묶음 **셋 다**(지수 비교 · 매수 이유 · 설명 한 문장).

  **① 우선주 제외.** `scoreRankableProductClause`에 「국내 · 코드 끝자리 ≠ 0」 제외를 더했다 — 랭킹·스크리너·백테스트·
  성적표·변화·내 기준이 한 규칙을 공유한다. 실측: 끝자리 ≠ 0인 국내 114종목이 **전부** 우선주, 이름에 「우」가 든 보통주
  3개는 끝자리 0. 점수 자체는 그대로(판 안 올림 — 산출이 아니라 비교 대상 변화). 재계산 뒤 국내 AI 랭킹 상위에 우선주 0
  (1위 대덕전자1우 → 리튬포어스). 종목 화면 점수 카드에 「우선주는 … 제외됩니다」 한 줄(실측 35320K).

  **② 점수 설명 한 문장.** `explainSummarySentence`(FE, LLM 0) — 가장 크게 받친 축 · 가장 낮은 축 · 전일 원인 상위 둘,
  원인이 축 결측·충실도면 「데이터가 바뀐 몫」. 실측 005930: 「종합 69점 — 추세(99점)이 가장 크게 받쳤고, 가치(12점)이
  가장 낮습니다. 어제보다 -2점(가치 변화 -1.9 · 수익성 변화 -0.3).」

  **③ 내 포트폴리오 vs 지수.** 평가금액 카드 안 「지수와 비교」 접힘 줄. **TWR**(매입금액 변화를 현금 흐름으로 빼기 —
  추가 매수가 수익으로 보이지 않게) vs 백테스트 화이트리스트 지수(휴장일은 직전 종가, 시작일 종가 없으면 비교 거절).

  **④ 매수 이유 태그.** `holding_theses`(083, **별도 표** — `PUT /user/holdings`가 행을 지웠다 다시 넣어 열이면 가져오기마다
  사라진다) · `PUT /user/holdings/{symbol}/thesis`(축 화이트리스트 · 중복 · 셋 초과 400) · 보유 상세 「살 때 본 것」 칩.
  태그가 있으면 보유 이유 점검의 점수 신호가 **그 축들의 평균**(두 날 모두 있는 축만)으로 바뀐다 — 라벨 「매수 이유(성장)」.

  ⚠️ ③④는 로그인 데이터가 있어야 보이는 화면이라 **실측 못 했다** — 단위 시험(TWR 입금 제거 · 지수 정렬 · 태그 신호 ·
  한쪽 날에만 있는 축 제외)과 타입까지.
  ⚠️ `gofmt` 형식 정리 루프(`tr | gofmt | sed > tmp && mv`)는 gofmt가 실패하면 **빈 파일로 덮는다**(파이프 종료 코드가
  sed 것). 이번엔 LF 파일이라 피했다 — 앞으로는 출력이 비었는지 보고 옮긴다.

  **검증**: go build·vet·test 전체 · fe tsc 0 · vitest 377 · 새 파일 eslint 0 · BE·FE 재배포(Tesseract OK) · 00:25 예약 배치가
  새 연결 그대로 스스로 완주(라벨 521 · 변화 757 · 성적표).
- **09-23 후속4 (6개월 항목 셋 — 앱 안 알림 · 점수 성적표 · 내 기준 점수 · BE+FE · 배포됨 · 미커밋)**

  사용자 결정: 수급 축 **보류**, 알림 **앱 안 배지부터**, 내 가중치 **AI 랭킹에**, 성적표 **지금 공개**.

  **① 앱 안 알림.** `GET /user/changes/summary`(종목 **수** — 이벤트 수가 아니다) → 사이드바 「변화」 배지
  (`useMyChangeSummary`, 30분) + 홈 네 칸 줄 위 한 줄 띠(`MyChangesStrip`, 로그인·변화 있을 때만).
  ⚠️ **로그인 상태 화면은 실측 못 했다** — 비밀번호 입력은 내가 하지 않는다. 빌드·타입·API 401까지만.

  **② 점수 성적표.** `score_ic_daily`(081) + `score_report_card` 잡(점수 배치 꼬리, 5거래일 지난 날만) +
  `GET /stocks/scores/report-card` + 백테스트 패널 아래 「축별 예측력」. 스피어만 IC · 상위20 초과 · 판정 단어.
  미래 참조 없음(다음 세션부터). 26일 백필 65초. **t를 겹치는 5일 창으로 보정한다** — 첫 계산에서 추세 축이
  t −2.8로 「거꾸로 유의」가 떴는데 26일은 독립 표본 약 5개라 보정 t −1.26이 맞다. 결과(국내): 종합 IC −0.022 ·
  전 축 「의미 없음」. `scoreMethodChanges`(방식 변경 이력)의 마지막 날짜 = `scoreMethodVersion`을 시험이 지킨다.

  **③ 내 기준 점수.** `user_score_weights`(082) · `GET /stocks/scores/custom-ranking?w=` · `GET/PUT /user/score-weights`
  · AI 랭킹 툴바 아래 「내 기준으로 보기」 → 슬라이더 5 + 「기본 대비 ±」 목록. 저장된 축 점수를 `compositeReconstruct`로
  다시 합산만 한다(배치 없음, 시장별 재료 10분 캐시). 기본 가중치 = 저장 점수(시험 · 실측 ±0). 쓰기는 거부형 검증(0..100, 합>0).

  **발견 — 고치지 않음(결정 필요)**: **우선주가 랭킹 상위에 선다.** 기본 랭킹 1위가 대덕전자1우, 국내 상위 50 중 4개.
  우선주가 보통주의 EPS·자본으로 가치 지표를 받아 가격만 싸 보인다. 고치면 산출 방식이 바뀐다(판 올림 필요).

  **검증**: go build·vet·test 전체 · fe tsc 0 · vitest 371 · 새 파일 eslint 0(analysis-client의 9건은 기존 React
  Compiler 경고) · BE·FE 재배포(Tesseract OK) · 브라우저: 성적표 구역, 내 기준 재정렬(성장 50 → +4들).
- **09-23 후속3 (로드맵 3개월 나머지 — 섹터 피어 · 로컬 감성 모델 · 「오늘의 변화」 · 도넛 섹터 탭 · BE+FE · 배포됨 · 미커밋)**

  사용자: 「모든 작업을 일괄로 권장 방향, 선택이 필요하면 물어봐 달라」. 물어서 정한 것 다섯:
  피어그룹 = **재무 3축만 섹터**, 기사 라벨 = **로컬 모델**, 변화 = **새 메뉴**, 도넛 = **섹터 탭 추가**,
  모델 = **KR-FinBert-SC + ProsusAI/finbert**(둘 다 라이선스 표기 없음 · pickle — 사용자 수용).

  **① 섹터 피어그룹.** `peerLevels(in, fundamental)` — 재무 지표만 사슬 맨 앞에 `asset:sector`.
  얇은 섹터는 기존 `minMetricSamples` 사슬로 내려간다. 자산유형별 **섹터 커버리지 80%에서 자동으로 켜진다**
  (`sectorPeerMinCoverage`) — 반쯤 켜면 같은 지표가 두 잣대로 섞인다. 섹터 3회차 뒤 **국내 82.7% · 미국 90.6%로
  둘 다 켜졌다.** 화면 이름 「국내 IT 기준」(`peerGroupLabel`에 섹터 키). 종합 백분위(`peer_pct`)는 시총 그룹 그대로.

  **② 전 종목 심리 축 — 9종목 → 7,341종목.** `be/sentiment`(Python·CPU, torch≥2.6이라 pickle은 weights_only) +
  `news_article_sentiment`(079) + `news_sentiment` 잡(점수 배치 첫 단계). 14일 백필 **22,489건 · 실패 0**, 64건 0.8초.
  시험 문장 6/6 정확. 점수 배치가 라벨(14일) + 내부자(180일) 집계 두 쿼리로 **직접** 매긴다(`attachSentiment`).
  요약 파이프라인의 `ApplySentiment`는 **지웠다** — 원천이 둘이면 00:25~08:00 사이 축이 생겼다 사라진다(30일 8,444번).
  ⚠️ `news_articles`만 `utf8mb4_unicode_ci`다 — 079를 거기에 맞췄다(처음엔 조인이 collation 오류로 실패).

  **③ 「오늘의 변화」 (`/changes`, 사이드바 순위 그룹).** `stock_change_events`(080) + `change_detect` 잡(점수 배치 마지막).
  급등락(종목별 90일 σ×2.5, 4~15%) · 거래량 5배 · 점수 ±8 · 뉴스 사건(제목 규칙 `ClassifyNewsEvent`) · 공시(허용 목록
  `ClassifyDisclosureEvent`) · 실적(예상 없으면 서프라이즈 안 만듦) · 내부자(평균 거래량 20%). ETF·ETN 제외(랭킹과 같은
  `scoreRankableProductClause`) · 소형주 문턱(500억/3억$, 관심·보유는 예외) · 휴장일 회차는 이미 올린 사건 재게시 금지.
  **점수 급변은 셋을 뺀다**: 축 유무 변화 · 비교 집단 변화(`inputs.rankedIn`) · **산출 방식 판 변화**(`inputs.method` —
  `scoreMethodVersion`, 방식을 바꾸면 올린다). 09-23 실측: 658건, 급등락 74 중 56이 「설명 불가」(대부분 뉴스 수집 밖의 미국
  중소형주 — 배지 설명은 「우리 기록에 없다」이지 「원인이 없다」가 아니다). 점수 급변 0(방식 변경일이라 100종목 제외).
  실측으로 잡은 결함 셋: 채권 ETN이 거래량 270배로 맨 위(→ 상품 제외), 꼬리 긴 종류가 순위 독점(→ 중요도 상한 5·2.5),
  `apiRequest`가 `{data}`를 이미 벗기는데 한 번 더 읽음(→ 칩에 내부 키 노출).

  **④ 보유 도넛 섹터 탭.** 보유 API·예시 API가 `sector`·`sectorLabel`을 싣는다(`SampleHandler.WithSectors`). 미분류는
  흐린 회색 조각(기타의 회색과 구분). 예시 모드 실측: IT 89.2% · 커뮤니케이션 3.2%.

  **덤**: `TestEveryMigrateIfNeededIsCalled` — 079 등록을 빠뜨릴 뻔해서 「정의만 있고 안 불리는 Migrate…IfNeeded」를 센다.
  기존 `MigrateNewsIfNeeded`가 원래부터 미호출이라 예외로 적었다(확인 대상).

  **검증**: go build·vet·test 전체 · fe tsc 0 · vitest 371(신규 4 파일) · Docker 재배포 여러 번(Tesseract OK) · fe 컨테이너 배포 ·
  브라우저 실측(도넛 섹터 탭, `/changes` 데스크톱·375px 가로 넘침 0, 필터 조회, 설명 패널 「국내 IT 기준」).
  ⚠️ 오늘 새벽 재배포가 종목 경고(01:30, 2시간) 실행을 끊었다 — 워치독이 재처리 큐에 넣었다.

  **남은 것**: 6개월 항목 일부는 후속4에서 했다.
- **09-23 후속2 (로드맵 3개월 착수 — 결함 1 · 이력 · 섹터 재료 · 가치 축 튐의 진짜 원인 · BE · 배포됨 · 미커밋)**

  사용자: 「권장 방향으로 진행」(`docs/COMPETITIVE_ANALYSIS.md` §4 3개월 표). 순서대로 넷을 했다.

  **① 매매 이력이 재시작마다 지워지던 결함 — 고쳤다.** `MigrateHoldingTradesPriceMismatchCleanupIfNeeded`를
  기동 경로에서 **걷어냈다**(함수 삭제, `migrate.go` 호출 둘 제거). 판정이 「체결가 ≠ 그날(없으면 직전) 종가」라
  장중 체결 기록이 거의 전부 걸렸다. 덤으로 **`cmd/cleanup-holding-trades -dry-run`조차 지우고 있었다** —
  CLI가 `EnsureSchema`를 먼저 부르고 그 안에 이 정리가 있었다. `holding_trades_cleanup_wiring_test.go`가
  「정리 호출은 CLI 한 곳뿐」을 지킨다. 이미 지워진 기록은 되살릴 수 없다.

  **② 축 점수 장기 보관 — `stock_score_history`(077).** 종목 × 날짜 × (종합 · 다섯 축 DECIMAL(5,2) · 커버리지).
  복사는 DB 안에서 한 문장(`scoreHistoryCopySQL`), 점수 배치 꼬리(`SnapshotDate`)와 백필이 부른다. 남아 있던
  30거래일을 즉시 채웠다: **537,387행 · 10초 · 48MB(행당 약 90B → 연 약 400MB)**. 없는 축은 NULL로 남는다(AAPL 가치).
  보유 이유 점검(`ScoreRangeBatch`·`EarliestScoredDate`)이 30거래일 밖을 이 표에서 잇는다 — **09-23 이후 매수는
  「매수 시점 점수」가 생긴다.**
  ⚠️ **심리 축은 00:25가 아니라 08:00 요약 배치가 얹는다** — 날짜 단위 복사만 두면 이력의 심리 축이 늘 빈다.
  `UpdateAxes`가 같은 문장으로 그 종목·날짜를 다시 복사한다.

  **③ 가치 축이 격일로 사라지던 진짜 원인 — 고쳤다.** 30거래일 이력으로 재 보니 가치 축 유무가 **81,124번**
  바뀌었다(하루 약 2,800종목). 축 유무가 바뀐 비교는 19%인데 **점수 절대 변동의 38%, 5점 이상 급변의 44%**.
  원인: `loadUniverse`가 종가를 `trade_date = 기준일`로 **정확히 그날 것만** 붙였다. 점수 배치는 00:25 KST,
  미국 종가 적재는 05:20 KST라 **미국 종목은 당일 종가가 사실상 없었다**(09-22 실측: 당일 종가 4,262종목 · 미국 0).
  기준일 이전 **최근 10일 안의 가장 최근 종가**로 바꾸고(`scoreCloseLookbackDays`), 쓴 날짜를 `inputs.closeDate`에
  남긴다. 재계산 결과(09-22): 종가 붙는 종목 4,262 → **14,436**, **미국 가치 축 0 → 3,728종목**, 미국 점수 3,555개
  변동(5점 이상 1,596) · 국내 25개(대부분 1점). AAPL 71→63 · NVDA 82→75 · MSFT 70→65 — 이제 격일로 안 튄다.
  ⚠️ 미국 랭킹 우주가 바뀐다(축 3개 이상 자격에 가치 축이 합류). 백테스트는 09-23 전후로 규칙이 다르다.
  ⚠️ 이 재계산에서 해설이 비어 있던 지수 6개에 **LLM 해설 6건**이 생성됐다(`GenerateNotes(force=false)`, 수동 실행).

  **④ 섹터 재료 — `stock_sectors`(078) + `sector_sync` 배치(worker-filings 02:30).** 국내 DART `company.json`의
  `induty_code`(KSIC), 미국 SEC submissions의 `sic`을 받아 공통 11섹터로 옮긴다(`model/sector.go`, 대형주 실제 코드로
  시험 고정). 로테이션(`SelectTargets`) + 90일 신선도, 기본 국내 1,000 · 미국 3,000. 우선주는 보통주 코드로 대신
  묻는다(`krCommonShareCode` — 유한양행우·두산우 확인). 64992(비금융 지주)는 산업재 — 실측으로 삼양홀딩스·한국앤컴퍼니가
  여기 든다. 피어그룹 교체는 후속3에서 했다(재무 3축 · 커버리지 80% 자동).
  **첫 회차(33분)**: 국내 722 · 미국 1,786 분류, 원천에 없음 국내 278(ETF 등) · 미국 1,194(ETF·워런트 — CIK 없음), 실패 0.
  ⚠️ **지주회사는 코드로 못 가른다** — 신한지주·iM금융지주(금융)와 삼양홀딩스·LG(비금융)가 같은 `64992`, HDC·효성은
  `649`(기타 금융업)였다. `SectorFromKSICNamed`: 지주 코드에서는 이름(금융·스팩·캐피탈·인베스트·투자, 명시 목록
  「신한지주」)으로 금융만 가르고 **나머지는 비운다**. 56행을 지우고 다시 받아 확인했다(LG·CJ·롯데지주·HDC·효성 → 비움,
  신한지주·iM금융지주·스팩 → 금융). 두 번째 회차(run 4101)를 큐에 넣었다 — 이후는 매일 02:30이 이어서 돈다.

  **잰 것(⑤ 준비)**: 평일 기사 약 5,500건/일(국내 2,500 · 미국 3,000), 종목 태깅 3,500~5,000건. 2주간 뉴스가 붙은 종목
  **3,664개**(국내 1,603 · 미국 2,061)인데 오늘 심리 축이 있는 종목은 **9개**. 기사 라벨이 생기면 닿을 범위다.

  **보고서 정정**: 초안이 「포트폴리오 단위 분석이 전혀 없다」고 적었는데 틀렸다 — 보유 화면에 구성 도넛(종목·통화) ·
  오늘 손익 기여도 · 평가금액 가중 점수 · 평가금액 추이가 이미 있다. 도넛 주석이 「섹터 컬럼이 없어 섹터 탭을 못 한다」고
  적고 있었고 ④가 그 재료다.

  **검증**: go build · vet · `go test ./internal/...` 전부 통과(신규: 정리 배선 1 · 섹터 매핑 3 · 우선주 1). Docker 재배포
  (Tesseract 5.3.0 OK). 섹터 실행 중에는 워커를 끊지 않으려고 api·worker-scores만 `docker compose up -d --build --no-deps`로
  다시 올렸다(같은 Docker 경로, Tesseract 로그 확인).

  **남은 결정(사용자)**: ⓐ 피어그룹을 섹터로 바꿀지(커버리지 수치 첨부) ⓑ 기사 라벨을 LLM 배치(하루 약 100콜)로 할지
  로컬 분류 모델로 할지 ⓒ 변화 피드 · 도넛 섹터 탭 화면 배치(UI는 제안 → 승인).
- **09-23 후속 (경쟁 분석 · 개발 우선순위 — 코드 변경 0 · 문서만)**

  사용자 요청: FnGuide·증권사 MTS·Seeking Alpha·TipRanks·TradingView 대비 기능 비교와 3/6/12개월
  우선순위. 전문 → `docs/COMPETITIVE_ANALYSIS.md`. **결론: 우위는 「LLM이 무엇을 말하나」가 아니라
  「점수를 얼마나 정직하게 설명·검증하나」**(잔차 0 귀인 · 「모른다」 상태 · 자기 백테스트). LLM 요약·
  어닝콜·「왜 움직였나」는 이미 포화(토스 AI 시그널·AI 어닝콜 270만, SA·TipRanks·TradingView).
  **3개월 순서**: ① 매매 이력 재시작 삭제 결함(`MigrateHoldingTradesPriceMismatchCleanupIfNeeded`,
  09-18 재현 · 여전히 미수정 — 개인화의 전제) ② 섹터 분류 → 피어그룹 ③ 축 소멸 노이즈 억제
  ④ 축 점수 얇게 장기 보관 ⑤ 기사 구조화 라벨 → 심리 축 12% 확대(**일 유입 건수 실측 먼저**)
  ⑥ 규칙 기반 변화 감지 + 앱 안 피드 ⑦ 포트폴리오 X-ray. 결정 대기: 자연어 스크리너·대화형 Q&A는
  조회 경로 LLM 규칙과 충돌, 컨센서스 축은 데이터 라이선스, 처방형 개인화는 규제 검토.
- **09-23 (외부 QA 기획서 재검증 — P0 10건 중 재현된 것은 하나였다 · BE+FE · 배포됨 · 미커밋)**

  사용자가 외부 QA 기획서(P0 7절 · P1 6절 · 22절)를 다시 붙였다. 09-22과 같은 계열의
  문서이고, **이번에도 먼저 잰 것이 결론의 전부였다.**

  **재현되지 않은 것 (기획서가 「없다」고 적었지만 이미 있는 것).** `/score/explain`이
  축별 `score`·`weight`·`weightShare`·`contribution`·지표 원값·`percentile`·`rankedIn`·
  `coverage`·`shrinkFactor`·`computedAt`을 전부 준다(P0-07 · P1 §8이 요구한 것보다 많다).
  스크리너 결측 처리(P1 §10)는 깔때기가 답한다 — `PER ≤ 20` 실측 `eligible 2,630 →
  measurable 1,584 → passed 1,055`, 정책은 **제외**이고 화면에 드러난다. Alpha(P1 §12)는
  `benchmarkReturn`·`alpha`·`basePrice`로 모델에 있고 발행 스냅샷은 불변. E2E(§16)는
  Playwright 176건이 이미 있었다.

  **P0-03(로딩 문구 잔존)·P0-04(홈 빈 영역)는 거짓이었다.** 하이드레이션 후 실측:
  `/`·`/market`·`/screener`·`/analysis` 전부 **로딩 문구 0건**, 홈은 지수 13종 ·
  브리핑 본문 · 기사 182건이 실제로 렌더된다. 기획서는 공개 사이트 크롤링으로 쓰여
  09-20~22의 SSR 선반입 이전 상태를 보고 있다.

  **재현된 결함은 P0-01 하나 — 그런데 그건 진짜였고, 구조적이었다.**
  `useScoreRankings`는 SSE 실시간 시세를 행에 덮고 `priceAsOf`를 `live`로 바꾸는데
  `ScreenerService.Screen()`은 `overlayLiveChanges()`를 **부르지 않았다.** 둘 다 같은
  `stock_daily_closes`를 같은 모양의 LATERAL로 읽으므로 **장이 닫혀 있는 동안에는 두
  응답이 소수점 열여섯 자리까지 같다**(02:05 실측: 리튬포어스 073570이 양쪽 다 2095원 ·
  +1.6990291262135921). 그 우연이 결함을 숨기고 있었다 — 기획서가 본 「랭킹 +1.70% vs
  스크리너 +0.49%」는 **장중에만 보이는 얼굴**이다.

  **고친 방법: 규칙 한 벌, 적용 여러 곳.** `be/internal/service/list_price_overlay.go`
  (신규)가 판정만 한다(`ResolveListPrices`) — 기준가 고르기(「저장된 종가가 이미 그
  시장의 오늘 것인가」)가 두 곳에서 갈리면 **미국 애프터마켓에서만 한 세션씩 밀리고
  그건 눈으로 안 갈린다.** 적용은 호출자가 한다(두 행 타입의 필드 이름이 `ClosePrice` ↔
  `Close`로 다르고, 이름을 맞추려면 응답 계약이 통째로 흔들린다). `ScreenRow`에
  `priceAsOf`·`basePrice`·`PrevClose`를 얹고 SQL에 `d.prev_close`·`d.trade_date`를 더했다.
  FE도 같은 모양으로 갈랐다 — `lib/list-price-overlay.ts`가 판정, 랭킹과 스크리너가 적용.

  ⚠️ **서버만 고치면 모자란다.** 첫 응답은 같아지지만 랭킹만 SSE로 계속 흐르므로,
  화면을 열어 둔 채 몇 분만 지나면 다시 갈린다. 그래서 `useScreenLivePrices`로
  스크리너도 같은 허브에 구독을 건다(허브가 시장당 연결 하나라 연결 수는 안 는다).

  **P0-02 일부도 함께 고쳤다** — `/screener`에 기준 시점이 **아예 없었다**(응답에
  `tradeDate`가 있는데 화면이 안 썼다). 가격 열도 없어 두 화면을 대조할 수조차 없었다.
  둘 다 넣었고 문구는 랭킹이 쓰던 `priceAsOfLabel`을 그대로 쓴다 — 같은 뜻을 두 화면이
  다른 말로 하면 그 자체가 불일치다.

  ⚠️ **E2E 실패 11건은 낡은 테스트다 — 앱 결함이 아니다.** `logged-out-sample`과
  `sample-mode-screens`가 「비로그인 `/holdings`는 안내 화면을 본다」를 검사하는데,
  `auth-required-shell.tsx`가 **2026-09-14 사용자 요청 4**로 「로그인하지 않은
  방문자에게는 묻지 않고 예시를 켠다」로 바뀌었다. 브라우저 실측으로 갈랐다:
  localStorage·쿠키가 **전부 비어 있는데도** 예시 모드이고(즉 기본값), 「예시 끄기」를
  누르면 안내 화면이 그대로 나온다(죽은 코드가 아니다). QA_INVENTORY의 「176 passed /
  0 failed」는 09-06 측정이라 그 변경 이전이다. **고치려면 현재 결정을 다시 확인받아야
  하므로 손대지 않았다** — 통과시키려고 단언을 바꾸는 것과 구분이 안 되는 자리다.

  **검증**: `go build`/`vet`/`go test ./...` 전체 통과(신규 유닛 2건), fe tsc 0 ·
  `npm test` 367 통과(신규 8건) · `next build` green, BE·FE Docker 재빌드 후 배포
  (Tesseract 5.3.0 OK). 운영 실측: 두 응답이 `close`·`changeRate`·`priceAsOf`·`basePrice`
  네 칸 모두 일치, 화면도 양쪽 `2,095원 +1.70%`. 375px 가로 overflow 0건.
  신규 E2E `tests/api/price-consistency.spec.ts` 3건 — **값 비교만으로는 장 마감 중에
  고쳐도 안 고쳐도 통과하므로**, 계약(`basePrice`·`priceAsOf`의 존재) 비교를 함께 건다.

- **09-22 후속 (전수 점검 — 같은 결함이 셋 더 있었다 · FE · 배포됨 · 미커밋)**

  앞 작업을 끝내고 **짐작하지 말고 전부 세자**고 정했다. 전 라우트(공개 16 · 로그인 7)를
  훑어 서버 HTML의 사람이 읽는 글자 수를 쟀다(`<script>`/`<template>` 제거 후).

  **① `redirect()`를 부르는 페이지가 넷이었다 — 09-20이 `/calendar` 하나만 고쳤다.**
  실측: `/ai-analysis`·`/financials`·`/disclosures`가 전부 **200 + 27자**(빈 셸).
  `/dashboard`만 **307**이었는데, 운이 아니라 `src/proxy.ts`가 렌더 전에 가로채기
  때문이다(다섯 번씩 찍어 상태코드가 안 흔들리는 것도 확인 — 경쟁 조건이 아니다).
  셋을 `next.config.ts`의 `redirects()`로 옮겨 **308**. **조각(`#ai-analysis`·
  `#disclosures`)은 뗐다** — 그 id를 가진 요소가 `/analysis`에 **하나도 없다**(grep 0건).
  이제 도달 불가능해진 페이지 파일 넷(`calendar` 포함)을 **지웠다** — 09-20이 「삭제
  권한이 없어 남겼다」고 적어 둔 그 파일들이다.

  **② 상한 없는 fetch가 둘 남아 있었다.** 09-20이 `lib/fetch-timeout.ts`로 앱 전체에
  상한을 걸었는데 `market-clocks.ts`와 `news-briefing-api.ts`가 빠져 있었다. 전자는
  특히 아픈 자리다 — `cachedGet`이 **모두가 함께 기다리는 promise 하나**를 들고 있어
  이 요청이 안 끝나면 시장 세션을 묻는 카드가 전부 같이 멈춘다(09-20이
  `inflightRefresh`에서 고친 것과 같은 모양). `client-error-report.ts`의 `void fetch`는
  **일부러 남겼다** — `sendBeacon` 대체용 `keepalive` 전송이라 상한을 걸면 목적이 깨진다.

  ⚠️ **tsc가 지운 파일 때문에 터졌다.** `.next/dev/types/validator.ts`(예전 `next dev`가
  남긴 생성물)가 삭제된 라우트를 계속 참조했다. `.next/dev`를 지우고 다시 빌드하니
  통과. 「tsc가 틀렸다고 하면 코드가 아니라 `.next`가 낡은 것」의 반대 방향 사례다.

  **배포**: `fe\scripts\deploy.ps1`(파이프로 감싸지 않고 그대로) → fe 이미지 재빌드,
  컨테이너 교체, nginx 재생성. api도 함께 재생성됐다(compose 의존). 13개 컨테이너
  healthy, `/health` ok, **Tesseract 5.3.0 확인**. 운영 실측: `/market` **3,033자** ·
  `/screener` **3,026자**(둘 다 종목명 포함), 옛 경로 넷 전부 308/307.

  ⚠️ **배포 중에 별건 결함을 하나 찾았다 — 고치지 않았다.** fe 냉시동 첫 렌더에서
  `⨯ TypeError: controller[kState].transformAlgorithm is not a function`이 난다.
  09-20이 「`docker restart`가 옛 로그를 보존해 생긴 착시」라고 결론지은 그 오류인데,
  **착시가 아니라 실재한다** — 새로 만든 컨테이너의 부팅 5초 뒤 로그에 있다.
  **내 변경 탓이 아니라는 것도 실측으로 갈랐다**: 냉시동 직후 `/market` → 오류 0,
  `/` → 즉시 오류, `/screener` → 오류 0. 즉 **`/` 에서만** 난다. 같은
  `next: { revalidate }` 패턴을 쓰는 내 새 페이지 둘은 멀쩡하다. 차이는 홈이
  **`Promise.all`로 revalidate fetch 둘을 동시에** 부른다는 것(`app/(main)/page.tsx`) —
  냉시동으로 데이터 캐시가 비어 있을 때 그 동시 쓰기가 터지는 것으로 보인다.
  영향은 **재기동 뒤 첫 요청 한 번**이고 각 함수가 `catch`로 빈 씨앗을 주므로 화면은
  선다(운영 `/` 실측 200 · 1,631자). 순차로 바꾸면 TTFB에 둘의 합이 붙어
  그 파일이 명시적으로 기각한 선택이 되므로, **고치려면 별도 측정 사이클이 필요하다.**

  **남은 큰 것 둘 (손대지 않음)**: ⓐ `/stocks/[symbol]`이 **570자**로 거의 비어 있다 —
  다만 이건 버그가 아니라 **기록된 보류**다(그 파일 주석: 「서버에서 종목 내용을
  그리게 되면 그때 색인 대상으로 올린다」). 종목 수천 개를 색인 대상으로 올리는
  결정이라 SEO 정책 판단이 먼저다. ⓑ `/earnings/domestic`·`/earnings/us`가 CSR이다
  (1,138 / 1,195자, 종목명 0). 여기는 `/market`처럼 못 옮긴다 — 보기 단위가
  **localStorage**에 있고(`useEarningsRangeMode`) 날짜가 「데이터 있는 날」로 자동
  이동하는데, 서버는 그 둘을 알 수 없어 씨앗이 틀리면 hydration이 어긋난다.

- **09-22 (`/market`·`/screener` SSR 선반입 — QA 기획서 76절을 먼저 잰 뒤 두 화면만 골랐다 · FE · 배포됨 · 미커밋)**

  사용자가 외부 QA/개선 기획서(P0 10 · P1 10 · 76절)를 통째로 붙였다. **먼저 잰 것이
  결론의 전부였다** — 그 문서는 공개 사이트 크롤링으로 쓰여 지금 코드와 크게 어긋난다.

  **문서의 P0 대부분은 이미 있다.** 점수 breakdown·변화 이유·가중치 공개(P0-06·07,
  P1-02)는 `/score/explain` + `score-explain-panel`·`score-movers-card`·
  `score-radar-chart`로 돌고 있고, `ScoreView`가 `breakdown`·`coverage`·`computedAt`·
  `tradeDate`·`weights`·`weightSource`·`peerPct`를 이미 싣는다. 「AI 기준일 ≠ 현재가
  기준일」(P0-02·03)은 `lib/data-freshness.ts`의 ⓐ/ⓑ 구분이 바로 그것이고, 0/null/오류
  구분(P0-04·09)은 `LoadErrorState`·`RetryMark`·`MenuEmptyState`가, 출처 표기는
  `data-reliability.ts`+`ProvenanceStamp`가 한다. 스크리너 저장·Stock Workspace·
  Market Radar(P1-07·09·01)도 이미 있다. **문서 §2의 `dataStatus`/`delaySeconds`/
  `sessionStatus` 전역 도입은 기각** — 이 앱은 같은 문제를 ⓐ/ⓑ 어휘로 풀었고 그 어휘는
  주말 거짓 경보(09-19 하루 67회)를 피하려고 일부러 만든 것이다. 새 필드를 얹으면 두
  체계가 공존한다. §51 Confidence도 `coverage`+`CoverageFloor`/`ShrinkFull`이 이미 그
  역할이라 이름만 다르다.

  **실측으로 재현된 결함은 하나였다** — 09-20에 홈·`/analysis`가 SSR로 넘어갈 때 **남은
  두 화면**. 운영 서버 HTML의 사람이 읽는 글자: `/market` **450자**(「마켓 레이더 로딩
  중…/랭킹 로딩 중…/차트 패널 준비 중…」), `/screener` **473자**(「스크리너를 불러오는
  중…/조건 목록을 불러오는 중입니다…」), 양쪽 다 **종목명 0개**. 고친 뒤 로컬 프로덕션
  빌드에서 **3,033자 / 3,026자**(SK하이닉스·삼성전자·리튬포어스·파미셀), TTFB 17.9 /
  16.7ms.

  `/market`은 `page.tsx`가 통째로 `"use client"`였다. 본문을
  `components/market-radar/market-radar-page-client.tsx`로 **그대로** 옮기고 페이지를
  서버 컴포넌트로 되돌렸다(`lib/market-api.server.ts`, 쿠키 없이 공개 `/rankings`,
  `fetchWithDeadline`, 재검증 60초). **기본 손잡이일 때만 심는다** — `?market=US`에
  국내 목록을 심으면 한 프레임 동안 틀린 목록이 선다. 실측으로 갈렸다: `/market` →
  HTML에 `rankedAt` 있음, `/market?market=US` → `"seed":null` · 79KB.

  `/screener`는 조건 목록(GET)과 **기본 조건의 결과(POST)** 둘 다 심었다. 조건 목록만
  심으면 HTML에 들어가는 건 지표 이름뿐이고 종목명은 결과 표에 있다. ⚠️ **POST는 Next
  데이터 캐시가 안 받는다**(16.2.10 `fetch` 레퍼런스의 캐시·메모이제이션이 전부 GET
  기준, `cacheComponents` 미사용이라 이전 모델). 기본 조건이 고정값이라
  `unstable_cache`(300초)로 감쌌고, 안쪽 fetch는 `cache:"no-store"`로 이중 캐시를 막았다.
  기본 조건 리터럴이 `rules`·`ranQuery` 두 곳에 있었는데 서버가 세 번째가 되므로
  `screener-api.ts`의 `defaultScreenRules()` 한 곳으로 모았다 — 어긋나면 「score ≥ 70」이라
  적힌 칸 옆에서 다른 조건의 결과가 보이고 그 차이는 화면에 안 드러난다.

  **함정 둘을 미리 막았다.** ① `useMarketRankings`의 초기화 이펙트는 마운트에도 도는데
  거기서 `setData(null)`·`setLoading(true)`를 그대로 두면 서버가 그린 100행이 하이드레이션
  직후 **사라졌다가 돌아온다** — 선반입 전과 똑같이 「랭킹 로딩 중…」이 스쳐 지나간다.
  `seedPendingRef`로 **첫 실행만** 건너뛰고 이후 손잡이 전환은 전과 같이 비운다(09-04
  「국내→해외 시계가 늦게 바뀐다」 결정 유지). `prevRanksRef`는 선반입이 있어도 비운다 —
  안 그러면 새로고침한 사람에게 비교할 앞이 없는 순위 화살표가 뜬다. ② hydration 안전을
  **확인하고** 썼다: `watchlist-store`는 모듈 전역 빈 집합이고 localStorage를 안 읽어
  서버·브라우저 첫 프레임이 둘 다 비어 있다. 브라우저 실측 hydration 경고 **0건**.

  **검증**: fe tsc 0 · eslint 새 파일 0건(단 `market-radar-page-client.tsx`에 React
  Compiler 경고 6건이 **원본에서 그대로 따라왔다** — git HEAD의 `market/page.tsx`를
  따로 린트해 같은 6건임을 확인했다. 수정은 별건) · `next build` green(`/market`·
  `/screener` 둘 다 ƒ dynamic) · 로컬 프로덕션 서버에서 브라우저 실측(랭킹 100행 · 스크리너
  깔때기 2,653→183 · 마운트 뒤 `/screen/fields`·`POST /screen` 재요청 200 = 씨앗이 얼지
  않음 · 1440px 스크린샷).

  **배포됨** — `fe/scripts/deploy.ps1`로 fe 이미지를 재빌드했다(같은 날 「후속」 항목
  참조). BE 변경 0. `.claude/launch.json`에 검증용 `fe-prod`(`next start -p 3200`)
  항목을 더했다 — 필요 없으면 지워도 된다.

  **남은 것**: `market/layout.tsx`가 아직 메타데이터를 들고 있다. 원래 이유(페이지가
  클라이언트 컴포넌트)는 사라졌지만 같은 값을 두 곳에서 내보내지 않으려고 그대로 뒀다 —
  옮길 때는 페이지로 옮기고 그 파일을 지우는 한 걸음이어야 한다.

- **09-20 후속 (SSR/CSR 점검 — 「갱신 중…」의 정체는 실패가 아니라 구조였다 · FE · 배포됨 · 미커밋)**

  사용자 요청 10항목. **먼저 잰 것이 결론의 절반이다**: 공개 API 여덟 개가 전부
  비로그인 200(2.9~14.8ms, 가장 느린 `/earnings/highlights`가 758ms), CORS 문제
  없음(nginx 동일 origin), **hydration 불일치 경고 0건**. 즉 **실패한 것이 없었다** —
  홈과 `/analysis`가 순수 CSR이었을 뿐이다. 서버 HTML에 「오늘의 시장 **갱신 중…** /
  AI 추천 랭킹 TOP / 거래대금 상위」처럼 **구획 이름만** 있고 숫자도 종목명도 0자였다.
  `/news/domestic`은 이미 SSR이었고(기사 100건) 그 패턴을 둘로 넓혔다.

  **`/calendar`의 빈 HTML도 예외가 아니었다** — 런타임 오류가 아니라 `redirect()`를
  부르는 정적 라우트라 200 + 빈 셸 + flight 안의 `NEXT_REDIRECT`였다. `force-dynamic`
  으로는 **안 고쳐진다**(실제로 해 봤다): `(main)` 레이아웃의 셸이 페이지가 던지기
  전에 흘러나가 헤더가 이미 나간다. `next.config.ts`의 `redirects()`로 옮겨 **308**.

  **진짜 결함 셋.** ① `use-market-indices`의 `catch {}`가 실패를 삼켜 `updatedAt`이
  null로 남고 화면이 「갱신 중…」 + 스켈레톤을 **영원히** 그렸다. 같은 파일의
  `isPageVisible()` 가드가 **첫 적재까지** 막아, 백그라운드 탭으로 연 홈은 누를
  때까지 비어 있었다(브라우저 패널 hidden으로 재현 → 0종, visibility 켜자 13종).
  ② **타임아웃이 사실상 없었다** — 앱 전체에서 상한이 걸린 fetch가 하나뿐이었다.
  ③ `dashboard-overview`의 `Block`에 `error`가 없어 **첫 적재 실패가 `empty`로**
  떨어졌다(「아직 산출된 점수가 없습니다」). 반대 방향 오류도 찾았다: 「거래대금
  상위」는 **장이 닫혀 빈 목록이 와도** 「불러오지 못했습니다」라고 말했다 —
  주말·마감 뒤 내내 성공한 요청이 실패로 표시됐다(실측 일요일 200·60바이트·0건).

  **고치는 중에 두 번 틀렸고 둘 다 실측이 잡았다.** ⓐ 서버 fetch에 중단 신호를
  달았더니 냉시동 첫 렌더가 세 번 터졌다(`controller[kState].transformAlgorithm is
  not a function`) — Next 데이터 캐시가 본문을 가르는 중에 신호가 얹혀서다. 서버에서는
  **끊지 않고 기다리기만** 한다(`fetchWithDeadline`). ⓑ 줄 수 상수를 `"use client"`
  모듈에서 서버 컴포넌트로 import 했더니 **`undefined`**가 왔다 — `limit=undefined`가
  쿼리에 실려 BE 기본 20건이 돌아왔고, 홈 카드가 일곱 줄이 아니라 **스무 줄**로 서서
  2026-09-05의 카드 높이 결정이 깨졌다. tsc도 eslint도 통과하고 **요청만 조용히
  틀렸다.** 평범한 모듈(`lib/dashboard-rows.ts`)로 옮기고 런타임 가드를 달았다.

  **실측(고친 뒤)**: `/` TTFB 중앙값 11.8→16.2ms·사람이 읽는 글자 0→2,608자,
  `/analysis` 12.4→25.5ms·0→5,401자, `/calendar` 200 빈 셸→**308**(4.4ms),
  `/news/domestic` 26.6ms 그대로. 냉시동 오류 0. fe tsc 0 · eslint 새 오류 0 ·
  단위시험 334→359(+25) · **새 E2E 12건 전부 통과**(JS 없는 HTML · hydration · 직접
  진입↔내부 이동 · 모바일 · 로그인/비로그인 · 요청 실패).

  ⚠️ **기존 UI E2E 10건은 이 작업 전부터 실패하고 있다** — 전부 「비로그인 안내
  화면」을 기대하는데, 커밋된 `useAutoSample`(09-14 결정: 「비로그인에게는 묻지 않고
  예시를 켠다」)이 그 화면을 건너뛴다. `auth-required-shell.tsx`는 이번에 손대지
  않았고 내 변경 경로에도 없다. **시험이 낡은 것이라 고치지 않았다 — 사용자 판단 몫.**

  ⚠️ **남은 것 둘.** `fe/src/app/(main)/calendar/page.tsx`는 이제 **도달 불가능한
  죽은 파일**인데 이 세션에 삭제 권한이 없어 남겼다(지워야 한다). 그리고 해외장만
  열린 시간대(대략 23:30~06:00 KST)에는 홈 AI 랭킹이 토글 확정 전까지 국내 목록을
  잠깐 보여준다 — 없애려면 서버가 `/market/clocks`를 읽어 열린 시장을 정해야 하고,
  그건 `useOpenMarket`의 설계(「왜 null로 시작하나」)를 바꾸는 일이라 남겨 두었다.

- **09-20 (번역 전환 4일 경과 점검 · 배치 재처리의 구멍 둘 — BE · 배포됨 · 미커밋)**

  **09-16 전환분은 4일째 문제없다.** 번역기 컨테이너 재시작 0회·OOM 0회, 미번역
  해외 기사가 하루 1~13건(전부 상한 도달분, 영어 유입의 0.7%), 해외 브리핑이 매시간
  생성, Azure는 전환 이후 **0자**(월 누적 1,559,454에서 멈춰 있다). 게이트를 유지한
  결정도 실제로 문제를 안 냈다 — 번역이 유입을 따라잡고 있어서다.

  점검하다 **번역과 무관한 구멍 둘**을 찾았고 둘 다 고쳤다.

  **① 재시작에 잘린 실행이 재처리에서 영구히 빠졌다 — 한 달에 69건 · 8개 잡 · 20일.**
  배포나 재기동으로 도중에 죽은 실행은 `MarkStaleRunsFailed`가 `error`로 닫는데,
  판정이 `failed`가 되면 `batch_catchup.go`가 건너뛴다. 그 제외 규칙 자체는 옳다
  (「돌았는데 깨졌다」는 사람이 봐야 한다) — 하지만 잘린 실행은 「**못 돌았다**」이고
  원인도 이미 안다. 실측 09-08~09-20 실패 18건 중 **11건이 이 경우**였다. 원장에
  기계가 읽을 표식(`result.interrupted`)을 남기고, `failed`라도 그 표식이 있으면
  재처리 대상으로 받는다. 허용 목록은 그대로다 — 이 예외가 답하는 것은 「왜
  실패했는가」뿐이고 「다시 돌려도 되는 잡인가」는 여전히 따로 답한다. 한국어 문구를
  되읽어 판정하지 않는 이유도 같다: 문구는 화면 몫이다.

  **② 주말마다 국내 종가가 거짓 경보를 냈다** — 09-19(토) 하루에 67회, 09-20(일)도.
  15:50 슬롯은 잡 자신이 KRX 달력으로 거르는데(`daily_close.go`의 `IsTradingDay`)
  기대만 **매일**로 적혀 있었다. 실측 2주: 평일 1회씩 · 토·일 0회. 기대에
  `schedule: ScheduleKRX`를 달아 감시가 잡과 같은 달력을 보게 했다. 해외 종가
  (05:20 KST)는 **손대지 않았다** — `schedule`을 달면 시각 해석이 뉴욕 현지로
  바뀌어(`LocationFor`) 05:20 ET가 되고, 이 잡은 KST 벽시계에 매달려 있다. 「달력은
  미국, 시계는 KST」를 표현할 칸이 없다.

  시험 +5(재처리 4 · 기대 1). 남은 것: **상류 점검(5xx)으로 실패한 잡은 여전히
  하루를 잃는다** — 09-20 01:00·01:30에 토스 점검에 걸린 `toss_universe`·
  `warnings_sync`가 19시간째 열린 사건으로 남아 있었다(이 둘은 재처리 대상이지만
  「진짜 실패」로 분류돼 빠진다). 다시 큐에 넣는 것은 DB 쓰기 권한에 막혀 못 했다 —
  관리자 「배치 관리」에서 두 잡을 수동 실행하면 풀린다. 근본 수정은 `toss.Error`에
  `IsUpstreamUnavailable`(5xx·code=maintenance)을 두고 ①과 같은 표식을 붙이는 것이다.
  한도 소진(DART)은 같은 날 다시 돌려도 소용없으므로 **그 표식에서 빼야 한다.**

- **09-18 후속4 (타임라인 화면 — 「+21.31%」를 누르면 그날 아침 기사가 나온다 · 사용자 승인 · BE·FE · 배포됨 · 미커밋)**

  Phase 3 FE. **AI 분석 탭 하단**, 점수 근거 바로 다음이다 — 그 탭이 이미 「왜」를 다루는
  자리이고, 새 탭을 만들면 탭이 열 개째가 된다(주석이 이미 「아홉이 폰 폭을 넘친다」고
  적고 있다).

  **날짜를 펼치면 그날 기사가 온다.** 기존 뉴스 목록 API에 날짜 필터를 **더하지 않았다** —
  `NewsStore.List`는 15초 TTL 캐시 · 싱글플라이트 · 만료분 선공급이 얹힌, 앱에서 손이
  가장 많이 간 읽기 경로다. 날짜 필터는 사람마다 다른 날을 펼치므로 **거의 재사용되지
  않는 캐시 키**를 그 경로에 섞는다. 타임라인이 자기 조회를 든다(0.29초).

  **실측이 잡아 준 결함 둘.**

  **① 「많이 본 순」이 조용히 「그날 마지막 20분」이 됐다.** 정렬이
  `view_count DESC, published_at DESC`였는데 이 DB의 조회수는 사실상 전부 0이라 첫 키가
  아무것도 가르지 못한다. 07-31(+21.31%)을 열었더니 23:21~23:40 다섯 건이었다. 동점일 때
  **이른 시각**을 먼저 두자 「한국투자證 목표가 65만원 상향」·「UBS: 내년 HBM 1위」·
  「[속보] 코스피 13% 폭등…삼성전자 21%」가 올라왔다 — **하루를 설명하는 기사는 장 전과
  장중에 나온다.** 조회수가 쌓이면 첫 키가 다시 일을 하도록 규칙은 남겼다.

  **② 내부자 매도가 「+381주」로 나왔다.** `formatShares`가 부호를 붙이고 있었는데,
  매수·매도는 **크기이지 방향이 아니다**. 방향이 뜻을 갖는 자리는 순증감뿐이라
  `formatSignedShares`를 따로 뒀다.

  **실측**(운영 데이터, 375px·1440px). 07-31 행을 펼치면 「내부자 거래 2건 — 매수 3,045주 ·
  매도 381주 · 순+2,664주」와 「그날의 뉴스 전체 732건 중」 열두 줄이 나온다. 가로 넘침 0.
  「수집 전」 레인 칩(주황)은 AI 점수 2026-08-07 · 내부자 2026-07-21 · 공시 2026-08-20으로
  뜬다. 문턱 줄은 「±8.73% 이상을 큰 등락으로 봤습니다 (이 기간 변동성 5.82%)」.

  fe tsc 0 · eslint 새 오류 0 · go build·vet·test 통과(누적 48).

  **남긴 것**: 기사별 감성은 **여전히 저장돼 있지 않다**(요약 파이프라인이 그때그때 센다) —
  타임라인의 뉴스 레인은 건수와 제목까지다. 남은 단계는 4·5·6·7·8·9.

- **09-18 후속3 (종목 타임라인 BE — 「설명이 안 되는 날」까지 보여 준다 · 사용자 승인 · BE · 배포됨 · 미커밋)**

  V2 Phase 3. `GET /stocks/{symbol}/timeline?days=90` — **새 표도 새 수집도 없다.**
  다섯 표를 같은 날짜 축에 세울 뿐이다. 실측 **0.42초 · 16.7KB · 읽기 여섯**(전부 인덱스).

  **뉴스는 SQL이 접는다.** 삼성전자에 붙은 기사가 15,232건, 하루 평균 180건이다. 행으로
  가져오면 한 요청이 수 MB다. `MEMBER OF`로 multi-valued index(`idx_news_symbols_mv`)를
  타고 `GROUP BY` 로 날짜별 건수만 받는다 — 실측 0.21초. `JSON_CONTAINS`로 쓰면 전체
  스캔이다(`news_repo.go`에 같은 주석이 있다).

  **제1 규칙 — 레인마다 시작일이 다르다.** 실측(005930): 주가 2024-12-16 · 뉴스 2026-01-14 ·
  실적 2025-07-08 · 내부자 2026-07-21 · 공시 2026-08-20 · 점수 2026-08-07. 1년을 그리면
  열 달치 레인이 비는데 그것은 **「아무 일 없었다」가 아니라 「수집 전」**이다. 모든 레인이
  `availableFrom`·`coversWindow`를 들고 나간다 (요구 8).

  **고르는 규칙 둘.** ① 크게 움직인 날 — 문턱이 **종목마다 다르다**(그 창의 표준편차 ×1.5,
  2~10%로 가둠). 005930은 변동성 5.82%라 ±8.73%였다. ② 분명한 사건이 있던 날 — 실적 ·
  공시 · 내부자 · 점수 ±3 이상. ②는 등락과 무관하게 남긴다(「실적을 냈는데 주가가 안
  움직였다」도 답이다). **뉴스 건수로는 고르지 않는다** — 그러면 거의 모든 날이 남는다.

  **실측 결과** (90일 → 39일): 07-31 **+21.31%**(뉴스 732 · 내부자 2) · 07-28 −14.51%
  (뉴스 990) · 07-07 −9.06%(**실적 발표**) · **07-13 −10.12%인데 뉴스 1건**. 마지막 줄이
  이 기능의 성격을 정한다 — **우리 데이터로 설명이 안 되는 날**이고, 그것을 그대로
  보여 주는 것이 정직성이다.

  **점수의 「왜」는 Phase 1 엔진을 그대로 쓴다**(`attributeChange`). 타임라인이 자기만의
  이유를 따로 계산하면 같은 날에 대해 두 화면이 다른 말을 한다.

  **컨센서스가 없으면 서프라이즈를 만들지 않는다.** 추정치 없이 0%로 적으면 「예상대로였다」가
  되는데 사실은 「비교할 예상이 없었다」다.

  go build·vet·test 통과(시험 +8, 누적 48).

  **남긴 것**: FE 없음 — 배치 제안 `docs/mockups/stock-timeline-v1.html`(차트·사건·레인이
  전부 실제 응답). 배치 후보 셋: **B. AI 분석 탭 하단**(권장 — 그 탭이 이미 「왜」를 다루고
  탭을 안 늘린다) · A. 새 탭(탭 열 개째 — 주석이 이미 「아홉이 폰 폭을 넘친다」고 적고 있다) ·
  C. 개요 차트 토글. **뉴스 제목은 안 싣는다** — 기사별 감성은 저장돼 있지 않고(요약
  파이프라인이 그때그때 센다) 하루 180건이라, 날짜를 누르면 기존 뉴스 목록 API가 답하는
  쪽을 제안한다.

- **09-18 후속2 (백테스트 화면을 붙였다 — 그리고 「빈 칸」이 없다는 것을 알았다 · 사용자 승인 · FE · 배포됨 · 미커밋)**

  **먼저 고친 것: 제안이 또 틀렸다.** 「종목을 고르기 전 빈 오른쪽 칸에 둔다」고 했는데,
  넓은 화면은 주소에 종목이 없으면 **기본 종목(005930)을 자동으로 연다**
  (`analysis/page.tsx`의 `DEFAULT_SYMBOL`, `analysis-client.tsx`의 `workspaceSymbol`).
  그 빈 칸은 코드에는 있어도 사용자에게는 거의 닿지 않는다. **붙여 보고서야 알았다** —
  1440px로 열었더니 오른쪽에 삼성전자가 떠 있었다.

  진입 동작을 말없이 바꾸지 않고 **문을 따로 달았다**: 목록 툴바 아래의 접힌 한 줄
  (`ScoreBacktestSummaryRow`, 높이 28px). 누르면 넓은 화면은 오른쪽 칸이, 좁은 화면은
  오버레이가 성적으로 바뀐다. **종목을 고르면 닫힌다** — 고르는 행동의 뜻은 「이 종목을
  보겠다」이므로 결과도 그래야 한다. 기본값을 성적으로 바꾸려면 `workspaceSymbol`의
  `defaultSymbol` 갈래를 지우면 된다(한 줄).

  **패널이 지키는 것.** 없는 숫자를 0으로 채우지 않는다(연 환산·Sharpe가 비면 「—」,
  왜 비었는지는 경고 줄이 말한다) · 표본을 숨기지 않는다 · **벤치마크를 색으로 가르지
  않는다**(이 앱에서 빨강·파랑은 상승·하락이다 — 지수는 점선 간격으로만 구분하고,
  포트폴리오 선만 결과의 색을 쓴다).

  **차트에 라이브러리를 들이지 않았다.** 점 30개짜리 꺾은선에 차트 엔진을 붙이면
  번들만 커진다. 휴장으로 빈 지수 값은 **직전 값을 이어 그린다** — 선을 끊으면 며칠씩
  빈 구간이 「지수가 사라졌다」로 읽힌다. 수익률은 서버가 그 날을 빼고 계산하므로 이
  이음은 그림에만 있고 값을 만들지 않는다.

  **실측**(운영 데이터). 1440px: 접힌 줄 → 오른쪽 칸이 곡선 3개(포트폴리오·KOSPI·KOSDAQ) ·
  KPI 5 · 비교 지수 표 · 경고로 바뀐다. 손잡이가 실제로 다시 돈다 — 매주 −1.59% →
  **매일 −1.00%** → **상위 100·매일 +0.46%**(API 실측과 일치). 종목을 고르면 닫힌다.
  375px: 「‹ 목록」 오버레이, 가로 넘침 0. 승률 42.86% → **42.9%**로 자릿수를 줄였다.

  fe tsc 0 · eslint 새 오류 0(내 훅의 `set-state-in-effect` 한 건은 저장소 패턴대로
  파생값으로 바꿔 없앴다) · go 전체 통과.

  **남긴 것**: 종목 하나의 벤치마크(`benchmarkForStock`)는 **손대지 않았다** — 리포트는
  발행 시점에 벤치마크를 동결하므로 바꾸면 앞으로 발행되는 것만 달라진다.

- **09-18 후속 (벤치마크를 여럿으로 · 배치 제안을 실제 페이지 위에서 다시 · 사용자 요청 2건 · BE · 배포됨 · 미커밋)**

  **① 제안이 틀렸던 것을 고쳤다.** 앞 줄에서 「`/analysis` 사이드 레일 카드」라고
  적었는데 **그 화면에는 사이드 레일이 없다.** 실제 구조는 두 칼럼이다 —
  왼쪽 랭킹 목록(20rem), 오른쪽은 `StockWorkspaceClient`, 그리고 **종목을 고르기 전
  오른쪽은 「종목을 선택하면 상세가 표시됩니다」 한 줄뿐인 빈 화면**이다
  (`analysis-client.tsx`). 그 빈 칸이 제자리다: 고르기 **전**이 정확히 「이 랭킹을
  믿을까」를 묻는 순간이고, 고르는 순간 워크스페이스가 그 자리를 가져가며, 기존
  화면에서 밀려나는 것이 없다. 모바일은 빈 칸이 없으므로(`isCompact`, 상세가 오버레이)
  목록 위 **접힌 한 줄(+30px)**로 서고 눌러야 편다.

  **② 벤치마크가 하나가 아니다.** 국내 KOSPI·KOSDAQ / 해외 S&P 500·나스닥 종합·
  나스닥 100·필라델피아 반도체·다우존스. 기본은 국내 둘, 해외 셋(S&P 500 · 나스닥 100 ·
  SOX) — **넷을 넘기면 어느 선이 내 것인지 안 읽힌다.** 실측이 곧바로 이유를 보여 줬다:
  국내 종합의 초과수익이 **KOSPI 대비 −8.89%p인데 KOSDAQ 대비로는 −4.52%p**다.
  하나로만 재면 절반이 안 보인다.

  **화이트리스트다.** 카탈로그에는 VIX · 미 국채 10년 · USD/KRW · 비트코인 · 금도 있고
  전부 일별 종가가 쌓여 있다(2026-02-02부터 155~157거래일 — 랭킹 이력보다 훨씬 깊다).
  그러나 **주식 바구니의 벤치마크가 아니다** — 변동성 지수나 채권 금리와 수익률을
  견주면 숫자는 나오는데 뜻이 없고, 화면에 한 번 나가면 사람은 그것을 성적으로 읽는다.

  **지수마다 휴장일이 다르다.** 각 지수는 포트폴리오와 **같은 달력** 위에서 굴리되
  자기 종가가 연속으로 있는 구간만 잰다. 빠진 날을 0%로 세면 지수가 덜 움직인 것이
  되고 그만큼 초과수익이 부풀려진다 — 그래서 **실제로 잰 날 수(`days`)를 행마다**
  내보낸다. 시험이 03-03이 빈 KOSPI로 그 성질을 잰다.

  **이력이 없는 지수는 행을 만들지 않는다.** 0%짜리 벤치마크가 서 있으면 포트폴리오가
  시장을 이긴 것처럼 읽힌다. 기준(primary)의 이력이 없으면 머리줄 기준을 **실제로 잰
  첫 번째**로 옮긴다.

  `GET /stocks/scores/backtest/benchmarks`가 시장별 목록과 기본값을 준다 — 화면이
  한국어 이름을 하드코딩하지 않는다.

  **남긴 것**: FE 없음. 제안 → `docs/mockups/score-backtest-v2-analysis-page.html`
  (v1은 카드 모양만, v2가 실제 페이지 구조 위). **종목 하나의 벤치마크**(`benchmarkForStock`,
  리포트·스탬프)는 **건드리지 않았다** — 리포트는 발행 시점에 벤치마크를 동결하므로
  바꾸면 앞으로 발행되는 것만 달라지고 과거와 기준이 갈린다. 별도 결정으로 남긴다.

- **09-18 (백테스트 — 점수가 맞았는지 처음으로 재 봤다 · 사용자 요청 1건 · BE · 배포됨 · 미커밋)**

  V2 Phase 2. 그리고 그 결과가 **편치 않다.**

  **새 표는 얇다.** `stock_scores`는 30거래일에서 잘리므로(연 7.5GB) 백테스트가 읽을
  과거가 없었다. 필요한 것은 그 표의 0.5%인 「그날 상위 200은 누구였나」뿐이라
  `score_ranking_history`(시장 2 × 축 6 × 200 = **하루 2,400행 · 연 35MB**)를 만들었다.
  점수 배치 꼬리에서 스냅샷 + 백필을 부른다 — **정리(prune)보다 먼저**, 같은 이유다.
  기존 30거래일치 백필이 **16초**에 끝나 첫날 27거래일을 확보했다.

  **엔진이 지키는 것 셋.** ① **미래를 보지 않는다** — D일 랭킹은 D일 종가로 계산되므로
  수익은 D+1부터다. `latestOnOrBefore(rebalanceDates, prev)`의 `prev`가 그 전부이고,
  `<=`로 두면 「그날 폭등한 종목을 그날 아침에 고른」 것이 되어 **어떤 랭킹이든 좋아
  보인다.** 시험이 3일에 +100% 폭등하는 종목을 3일 랭킹에만 심어 놓고 그 폭등을 한 푼도
  못 먹는지 잰다. ② **없는 종가를 0%로 세지 않는다** — 거래정지를 「보합」으로 세면
  나머지 종목이 희석된다(시험: +10%가 +5%로 떨어지는지). ③ **표본이 짧으면 비운다** —
  28거래일 Sharpe가 4.65로 나왔다(운을 실력으로 읽은 숫자다). 연 환산과 Sharpe는
  40거래일부터, 변동성(서술 통계)만 20거래일부터.

  **그래서 나온 숫자** (국내 · 상위 20 · 주간 · 28거래일, KOSPI +7.30%):
  심리·수급 **+9.35%p**(표본 11일) · 성장 **+6.77%p** · 추세 −2.26%p · 수익성 −5.15%p ·
  **종합 −8.89%p** · 가치 −9.29%p. **종합 점수가 다섯 축 중 가장 나쁘다.** 28거래일은
  아무것도 증명하지 않지만, 「가중 평균이 서로 다른 신호를 상쇄한다」가 이제 **측정
  가능한 가설**이다 — 관리자 「가중치 바꿔 보기」와 이어 붙이면 고치면서 잴 수 있다.

  `GET /stocks/scores/backtest` 실측 **0.24초 · 2.9KB · 읽기 2회**(얇은 이력 + 종가).
  사전 계산으로 안 옮긴 이유: 조합(시장 × 축 × 상위 N × 주기 × 창)이 곱셈으로 느는데
  그 대부분을 아무도 열지 않는다.

  **관리자 계정을 만들었다** — 지난 회차에 화면 실측을 못 했던 D를 끝냈다.
  `admin@quantport.local` / `admin1234`(앱 규칙이 8자 이상이라 `admin`은 거절된다).
  `users.role='admin'`. 실측: 005930 가치 가중치 15 → 40 → **48점이 43점**, 드라이버
  −5.02가 exactDelta와 일치, 저장은 일어나지 않았다. ⚠️ 이 계정은 **로컬 개발 DB**의
  픽스처다(`host.docker.internal:3306`). nginx가 0.0.0.0:80/443에 붙어 있으므로 공개
  배포 전에는 지우거나 비밀번호를 바꾼다.

  **남긴 것**: FE 없음 — 배치 제안은 `docs/mockups/score-backtest-v1.html`(레일 카드 ·
  전폭 패널 · 관리자 전용 셋, **곡선과 숫자가 전부 실제 백테스트 결과**). 「28거래일
  결과를 사용자에게 보여 줄까」가 그 문서의 진짜 질문이다.

- **09-17 후속 (설명을 화면에 붙였다 — A·B·C·D 네 자리 · 사용자 요청 1건 · BE·FE · 배포됨 · 미커밋)**

  앞 줄의 `/score/explain`을 화면 넷에 붙였다. 사용자 결정: **넷 다**, 다만 ① 개요
  카드에 「데이터 없음」 글자를 덧붙이지 말 것(「-」가 이미 같은 말이다) ② 설명문으로
  자리를 먹지 말 것 ③ **사용자에게 나가는 문장은 ~입니다·~합니다** ④ **모바일 가독성이
  최우선이고, 데이터가 많아 상하면 AI 핵심 요약처럼 간단/상세로 가를 것.**

  **A — 개요 점수 카드에 한 줄.** 「전일 +2 · 추세 변화 · 근거 ›」. 축 막대 **아래,
  카드 테두리 위**에 서므로 막대와 섞이지 않는다. 누르면 AI 분석 탭으로 간다
  (`onOpenExplain` — 이미 `TabScore`가 쓰던 통로와 같은 모양). 높이 +28px.
  원인은 **가장 큰 것 하나만** 쓴다 — 둘을 쓰면 줄이 접히고 개요 최상단이 두 층이 된다.

  **B — AI 분석 탭 설명 패널**(`score-explain-panel.tsx`). 간단/상세는 `MiniToggle`
  그대로, 기본 「간단」, 상태 저장 없음. 간단 = 구성 막대 + 전일 변화(상위 3) + 기준일.
  상세 = 거기에 전주 창 · 드라이버별 지표 변화 · 축별 지표 표 · 제외된 값 · 출처.
  **표를 4열로 두지 않았다** — 375px에서 「지표·값·순위·비교집단」은 어느 칸도 못 읽는다.
  비교 집단은 **축 머리에 한 번**, 그 축에서 혼자 다른 집단인 지표만 자기 줄에 밝힌다.
  설명문 대신 **숫자 둘을 나란히** 적는다: `33.3% (설정 30)` — 빈 축이 있어 분모가
  줄었다는 사실이 문장 없이 읽힌다.

  **C — 보유·관심 「점수 변화」 카드**(`score-movers-card.tsx`). **목록 행이 아니라
  레일**로 갔다: 관심 목록의 점수 칸은 폭 36px이고 보유 목록엔 점수 칸이 아예 없어,
  375px에서 그 줄에 사유를 넣으면 옆 칸 수익률부터 잘린다. 사유는 읽혀야 값이 있다.
  목록 행마다 15KB를 부를 수도 없어 **요약 입구**를 새로 냈다 —
  `GET /stocks/scores/changes?symbols=…`(종목당 델타 1 + 최대 원인 1, 10종목 2.6KB ·
  0.25초 · **쿼리 1회**). 레일 폭에서는 「가치·심리·수급 축 결측」이 잘려 정작
  「결측」이 사라지길래 **`compactDriverLabel`**(「축 결측」·「추세」)을 따로 뒀고
  온전한 문장은 `title`에 남겼다.

  **D — 관리자 「가중치 바꿔 보기」.** `GET /admin/score/explain-weights` — **저장된 한
  행**을 다른 가중치로 다시 합산한다. 쓰기도 배치도 없다(저장 후 배치로 확인하면
  17,906종목이 한 번 더 돌고, 되돌리려면 또 한 번이다). 축 값과 달리 **가중치는
  분모에도 들어가 선형이 아니라서** 축을 정해진 순서로 하나씩 바꾸며 증분을 적고,
  합만 정확하다는 사실과 그 순서(`order`)를 응답과 화면이 함께 밝힌다.

  **읽히게 만든 것 둘.** `domestic:KOSPI:cap3`은 배치의 내부 키다 — 가장 자주 읽히는
  칸에 그게 서 있으면 거기서 멈춘다. `peerGroupLabel`로 「국내 KOSPI 대형주」가 됐다.
  출처 목록에서도 테이블 이름을 걷어 냈다(「일별 종가」·「분기 재무제표」). 그리고
  **구성 막대에 있던 축이 근거 목록에서만 사라지지 않게** 했다 — 심리·수급은 뉴스
  감성·내부자 거래에서 나와 지표 표가 없으므로, 그 사실을 한 줄로 적는다.

  **실측**(운영 데이터·컨테이너 재빌드 후). 375px: 개요 한 줄 · 간단 · 상세(지표 변화
  11줄, 축별 표 4축 12지표) · 레일 카드 — 가로 넘침 0. 1280px 동일. 관심 레일 실제
  출력: 애플 72 **+11 축 결측** · NVIDIA 68 +8 축 결측 · 삼성전자 48 +2 추세 ·
  카카오 29 −2 축 결측 · SK하이닉스 79 +1 추세. go build·vet·test 전체 통과(시험 +7,
  누적 24) · fe tsc 0 · eslint 새 오류 0(기존 `set-state-in-effect`는 손대지 않은
  파일에도 있다).

  **남긴 것**: **D는 화면 실측을 못 했다** — 관리자 로그인이 필요하고 계정을 만들지
  않았다(단위 시험 4개와 타입 검사까지만). Phase 2~9는 `docs/V2_PLAN.md` 그대로.

- **09-20 (통합 흐름 — 화면은 다 있었고 길이 없었다 · V2 Phase 9 · BE+FE · 배포됨 · 미커밋)**

  `GET /user/symbols/{symbol}/context` + 종목 화면의 「내 기록」 한 줄. 범위 「가」
  (사용자 결정). **V2 마지막 단계** — Phase 7만 11월로 남았다.

  **조사 결과**: 다섯 단계 화면(발견·근거·판단·반영·추적)은 Phase 1~6으로 전부
  만들어져 있었고, 잇는 부품(`analysisSymbolHref`)도 이미 있었다. **그 부품이
  거의 안 쓰이는 게 문제**였다 — 메모장·관심·스탬프·리포트 넷뿐이고 목표가·보유·
  스크리너는 안 썼다.

  **가장 큰 구멍은 종목 화면이었다.** 사용자에 대해 아는 것이 `quote.isHeld`
  **한 비트뿐**이라, 이미 보유 중인데도 「보유종목 등록」을 똑같이 내밀고 있었다.

  **조회 한 번**으로 끝낸다 — 다섯 표를 따로 물으면 종목을 열 때마다 왕복이
  다섯이다. 전부 `(user_id, symbol)` 단일 조회라 스칼라 서브쿼리로 묶었다.
  실측 **0.054~0.087초**(보유 종목은 점검 판정 포함).

  **판정은 빌려 쓴다** — `HoldingsDiagnosisService`를 부른다. 두 벌로 만들면
  언젠가 갈라지고, 그때 보유 화면은 「약화」 종목 화면은 「유지」를 말한다.
  **보유 중일 때만** 부른다(안 가진 종목에 「보유 이유」가 없다).

  **내가 앞 단계에서 빠뜨린 링크 둘도 채웠다**:

    ① 보유 이유 점검 카드가 **클릭 불가**였다 — 「카카오 약화 −46」을 보고도 그
       근거로 갈 수 없었다. 보유 **목록**은 되는데 Phase 6에서 만든 카드만 빠졌다.
       추적(5)→근거(2)로 되돌아가는 **유일한 자리**라 흐름이 거기서 끊겨 있었다.
    ② 목표가 화면에 종목 링크가 **0건**이었다.

  **행동 버튼이 아니라 상태 표시다.** 2026-09-03에 헤더 액션을 여섯에서 셋으로
  줄인 결정이 있어 같은 영역이 붐비는 것에 주의했다 — 이 줄은 누르면 그 기록이
  있는 곳으로 갈 뿐 새 창을 열거나 무언가를 만들지 않는다.

  **없는 것도 남긴다**(점선 칩) — 「아직 안 했다」도 흐름의 한 칸이다. 다만 전부
  비면 줄 자체를 안 그린다: 점선 칩 여섯 개는 안내가 아니라 잡음이다.
- **09-19 후속2 (신뢰도 표기 — 절반은 이미 돼 있었다 · V2 Phase 8 · FE만 · 배포됨 · 미커밋)**

  범위 「가」(핵심 경로만, 사용자 결정). **BE 변경 0** — 서버가 이미 `dataSource` ·
  `periodEnd` · `fetchedAt` · `coverageTier` · 내부자 `source`를 실어 보내는데 화면이
  안 쓰고 있었다.

  **조사해 보니 절반은 이미 규약을 지키고 있었다**:

    실적    `DateStatusBadge`로 확정·예정·추정을 이미 구분 (그 파일 주석에
            「절대 같은 모양으로 그리지 않는다」고 적혀 있다) — **손대지 않았다**
    점수    Phase 1의 `ScoreProvenance` — 이미 있다
    시세    있는 행만 그리므로 날짜 축에 빈칸이 안 생긴다 → **「휴장」을 말할 자리가 없다**

  ⚠️ 목업에 그렸던 「일별 시세 · 휴장」 전후는 **일어나지 않는 상황**이었다.
  `daily-close-tab.tsx`는 `pageRows.map`으로 받은 행만 그린다. 만들지 않았다.

  **실제로 빈 곳은 셋이었다**:

    ① 재무 종합 지표 카드 — 커버리지 등급만 있고 **기준·출처가 없었다**.
       커버리지는 「얼마나 채워졌나」이고 이것은 「언제 것이고 어디서 왔나」다.
       2026-09-03에 뺀 각주를 되살린 것이 아니다 — 그것은 모든 종목에 똑같이
       붙는 산문이었고, 이것은 종목마다 다른 사실 둘이다(실측: 「기준 25년 연간 ·
       출처 DART」).
    ② 내부자 탭 — `source`가 응답에 있는데 화면 언급이 **0건**이었다
       (실측: 국내 DART · 해외 EDGAR).
    ③ 재무제표 차트 — `catch(() => setHistory(null))`로 **실패를 「없음」으로 삼켰다.**
       오류 상태를 따로 들고 「다시」를 단다.

  **「—」는 건드리지 않았다** (2026-09-17 사용자 결정: 데이터 없음과 `-`의 뜻이 같다).
  가장 흔한 상태에 장식을 붙이면 표가 표식으로 덮인다.

  새 부품: `lib/data-reliability.ts` · `components/ui/provenance-stamp.tsx`
  (`ProvenanceStamp` · `RetryMark`). 앞으로 새 화면은 이것을 쓴다.
- **09-19 (목표가 시나리오 — 한 숫자를 찍지 않는다 · V2 Phase 5 · BE+FE · 배포됨 · 미커밋)**

  `GET /stocks/{symbol}/scenario` — 약세·중립·강세 셋과 **그 방법의 실측 성적**.
  밸류에이션 탭 맨 위 (사용자 결정 A). 조회 두 번(종가 1년치·점수 한 줄), 첫 호출
  0.81초(백테스트 포함) · 캐시 0.05초.

  **증권사 목표가가 안 들어온다.** 사용자 목표가 2건 · 스탬프 3건이라 「과거 예측
  오차」를 집계할 원본이 없다. 그래서 시나리오를 우리 데이터로 도출하고 **그 방법
  자체를 백테스트**했다 — Phase 2에서 랭킹에 한 것과 같다.

  **PER 기반은 두 번 막혔다**: ① 앱의 PER은 연간 순이익 기준인데 TTM으로 다시 만들면
  종목 화면과 어긋난다(실측: 화승알앤에이 1.58 vs 앱 3.05) ② 과거 PER은
  `stock_scores`가 30거래일만 보관해 90일 지평을 맞춰 볼 수 없다.

  **그래서 가격 이력으로 만든다.** 종가 21개월치라 선행 편향 없이 만들고 검증한다.
  세 후보 실측(국내 250~300종목 × 6시점 × 90일):

    변동성 z=1.28    적중 72.3%   폭 약 30%   ← 채택 (사용자 결정)
    변동성 z=1.65    적중 82.6%   폭 약 40%
    52주 고점·저점    적중 60.6%   폭 54%     ← **더 넓은데 덜 맞는다**

  52주 범위는 종목별 편차도 극단적이라(삼성전자의 52주 저점은 현재가의 −73%) 약세
  시나리오로 쓸 수 없었다. Base는 **현재가**다 — 200일 이평으로 두면 오차가 22.9% →
  29.0%로 **더 나빠진다.** 장기 평균 회귀 가정이 실제로는 더 틀렸다.

  **점수로 방향을 기울이지 않는다.** Phase 2에서 종합 점수 상위 바구니가 KOSPI를
  8.89%p 밑돌았다 — 그 점수로 목표가를 밀면 이미 틀렸다고 측정해 둔 것을 예측에
  다시 넣는 일이 된다.

  **배포본의 실측**(250종목 × 6시점 = 표본 1,444): 적중 **70.1%** · 중립 평균 오차
  **24.0%** · 아래 이탈 22.6% vs 위 이탈 7.3%(**3.1배**). 목업에 적었던 72.3%와
  다른 이유는 시점 격자가 다르기 때문이고(목업은 고정 월말, 배포본은 최신일 −90일에서
  30일 간격), 화면이 표본·시점·기준일을 함께 적으므로 스스로를 설명한다.

  **내재 PER이 숫자에 뜻을 준다**: 「418,728원」은 아무 말도 안 하지만 「그 가격이면
  PER 63.4배」는 판단할 수 있다. 가격에 비례해 늘린 값이라 이익 전망을 지어내지
  않고, 앱이 쓰는 PER에서 출발하므로 아래 지표표와 어긋나지 않는다. PER이 있는
  종목이 60%뿐이라 **상세 보기에만** 둔다.

  ⚠️ 적중률은 **방법의 성적이지 종목의 성적이 아니다.** 종목별 표본은 6건뿐이라
  화면 문장을 「이 방법은」으로 시작해 그 구분을 잃지 않게 했다.
- **09-18 후속6 (종목 스크리너 — 「없는 것」과 「못 잰 것」을 가른다 · V2 Phase 4 · BE+FE · 배포됨 · 미커밋)**

  새 메뉴 `/screener` (사용자 결정 A). `POST /stocks/screen` · `GET /stocks/screen/fields` ·
  `GET/POST/DELETE /user/screens`. **새 수집도 새 배치도 없다** — 조건에 쓰는 값이 이미
  `stock_scores` 한 줄 안에 전부 있다(`inputs.raw`에 ROE·부채비율·PER·PBR, `inputs.technical`에
  RSI·52주 신고가 대비·이평, `inputs.marketCap`에 시가총액). 실측 **0.18초**.

  **깔때기가 이 화면의 존재 이유다**: 전체 → 시장 → 점수 자격 → 잴 수 있음 → 조건 통과.
  보통의 스크리너는 마지막 숫자만 말하고, 그래서 0이 나오면 사용자가 **시장을 탓한다.**
  실측(2026-09-17 국내): 17,919 → 4,412 → 2,653 → 2,544 → 50이고, 화면이 «109종목은
  조건을 틀린 것이 아니라 그 값을 잴 수 없어 빠졌습니다»를 적는다 (요구 8).

  **랭킹과 같은 우주를 본다** — 축 3개 이상 · 지수 추종 상품 제외(`scoreMinAxesClause`).
  다른 우주를 보면 같은 종목이 랭킹에 없고 스크리너에만 있게 된다. 국내 4,412 →
  2,653으로 40%가 빠지는 큰 단계라 **깔때기에 자기 칸으로** 나간다.

  **커버리지 숫자가 목업과 달랐다.** 목업은 전체 17,919 기준이었는데 실제 분모는
  자격 통과 2,653이다. 그 안에서는 재무 91~99% · PBR 98% · 시가총액 100% · PER 60%로
  훨씬 높고, 진짜 낮은 것은 **심리·수급 12%** 하나다. 설계는 그대로 — 커버리지를
  API가 동적으로 내려주므로 화면이 바뀔 것은 없었다.

  **화이트리스트가 안전장치다**(`model.ScreenFields`). 필드 이름이 곧 JSON 경로이자
  정렬 식이라, 사용자가 보낸 문자열을 그대로 쓰면 주입이다. 모르는 필드는 **조용히
  버리지 않고 거절한다** — 버리면 조건이 빠진 채 결과가 나오는데 화면은 걸린 줄 안다.

  **실측이 잡아 준 결함 둘**:

    ① **저장 왕복이 깨졌다.** 시가총액 「1조」를 조회용으로 환산한 1e12(원)을 그대로
       저장하니, 화면이 다시 불러 보낼 때 1e12에 또 1e12가 곱해져 결과가 언제나 0이
       됐다. 환산을 **실행 지점 한 곳**으로 몰고 저장은 화면 단위 그대로 둔다.
    ② **점수 열이 두 번 나왔다.** 조건에 건 필드가 결과표의 열이 되는데, 종합 점수는
       전용 열이 이미 있어 「88 88」이 나란히 섰다.

  ⚠️ 시험용으로 Git Bash `curl`로 넣은 저장 조건 3건의 이름이 깨져 있었다 — **셸
  인코딩 문제이지 앱 문제가 아니다**(브라우저에서 저장한 「저평가 대형주」는 정상).
  깨진 행은 지웠다.
- **09-18 후속5 (보유 이유 점검 — 「올랐으니 좋다」를 판정에서 뺐다 · V2 Phase 6 · BE+FE · 배포됨 · 미커밋)**

  `GET /user/holdings/diagnosis` — 보유 종목마다 「살 때의 이유가 지금도 유효한가」를
  **강화/유지/약화/판단 보류**로 답한다. 근거 셋: AI 점수 추세 · 실적 서프라이즈 ·
  내부자 거래. 보유 수와 무관하게 **조회 5회**(`ScoreRangeBatch`·`EarningsSince`·
  `InsiderSince`·`LastCloses`·`EarliestScoredDate`).

  **수익률은 판정에 안 쓴다.** 「올랐으니 이유가 강화됐다」는 결과를 근거로 되돌리는
  말이라, 넣는 순간 이 기능은 「오른 종목은 좋은 종목」이라고 말하는 값비싼 장치가
  된다. 실측이 그 분리를 보여 준다 — 삼성전자 **+37.6%에 「약화」**, 엔비디아
  **+21.8%에 「유지」**, SK하이닉스 +96.9%에 「강화」.

  **매수 시점 점수는 거의 언제나 없다.** 점수는 30거래일만 보관하는데(행당 1.5KB ×
  17,906종목/일 = 연 7.5GB) 대부분의 매수는 그보다 오래됐다. 빈칸으로 두면 화면이
  「안 변했다」로 읽으므로 `ScoreAtPurchaseReason`으로 **왜 없는지**를 낸다 (요구 8).

  **실측이 잡아 준 결함 셋** — 셋 다 데이터를 넣어 보기 전에는 안 보였다:

    ① 점수 추세가 **매수 전부터** 잡혔다(08-11 매수인데 08-07부터). 실적·내부자는
       보유 기간으로 잘랐는데 점수만 안 잘랐다 — 살기 전의 변화가 보유 판단에 든 것.
    ② 「신호 둘 미만이면 보류」가 **가장 강한 증거를 묻었다**: 카카오는 매수 시점
       75점이 29점으로 **−46**인데 실적·내부자 기록이 없어 「판단 보류」가 됐다.
       약한 것은 증거의 **다양성**이지 강도가 아니다 → `Strong`(문턱의 두 배) 도입.
    ③ **침묵하는 신호가 판정을 뒤집었다.** 카카오는 강한 −46 하나로 「약화」인데
       삼성전자는 강한 −13 옆에 방향 0짜리 신호가 붙자 합이 문턱(2)에 못 미쳐
       「유지」가 됐다 → 강한 신호는 두 몫(`DiagnosisStrongWeight`).

  **내부자 순매매에는 크기 개념이 없었다.** 애플 3건·4,316주가 엔비디아 563만 주와
  똑같이 −1로 세어져 점수 +22를 상쇄했다. 하루 거래량 대비로 재면 애플 0.011% ·
  삼성 0.077% · 하이닉스 0.27% · 엔비디아 4.7% — **1%** 아래는 방향을 0으로 둔다
  (`DiagnosisInsiderVolumePct`). 신호를 **버리지는 않는다**: 「재 봤으나 방향이 없음(=)」과
  「잴 수 없었음(−)」은 화면이 달리 말해야 하는 두 상태다.

  FE는 **보유 요약 바로 아래**(사용자 결정 A) · 간단/상세 `MiniToggle`. 375px에서는
  근거 수와 수익률을 접는다 — 접히는 둘은 판정의 근거가 아니라 곁가지다.

  ⚠️ **기존 결함 발견(내 코드 아님, 미수정)**: `MigrateHoldingTradesPriceMismatchCleanupIfNeeded`
  가 **API 시작마다** 돌면서 체결가가 그날 종가와 0.01 이상 다른 매매 이력을 전부
  지운다. 실제 체결가는 장중가라 거의 언제나 종가와 다르다 → 사용자가 손으로 적은
  매수 기록이 다음 재시작에 **말없이 사라진다**. 재현·확인 완료(52,000원 → 재시작 후
  소멸, 186,000원(실제 종가) → 생존). 별도 작업으로 분리했다.

- **09-17 (V2 착수 — 점수에 「왜」를 붙였다 · 사용자 요청 1건 · BE · 배포됨 · 미커밋)**

  V2 요구 10건을 받았고, 그 전부가 기대는 토대 하나를 먼저 놓았다: **AI 점수의
  근거와 변화 원인을 저장분만으로 설명하는 API.** `GET /stocks/{symbol}/score/explain`
  (실측 0.23초 · 15KB · **쿼리 1회 · LLM 0 · 외부 호출 0**). 계획 전문 → `docs/V2_PLAN.md`.

  **왜 이것이 먼저인가.** 조사해 보니 재료는 이미 전부 DB에 있었다 —
  `stock_scores.inputs`가 종목·날짜마다 지표 원값 · 피어 백분위 · **어느 그룹에서
  순위를 매겼는지** · 도메인 밖이라 버린 값 · 기술적 지표 17종을 통째로 이고 있다.
  없던 것은 데이터가 아니라 **그걸 읽어 주는 입구**였다.

  **귀인이 근사가 아니다.** 종합 점수는 저장값만으로 정확히 재현된다(005930 09-16:
  raw 47.64 → 48, 오차 0). 가중치·축 유무·축소율을 고정하면 raw는 축 점수에 선형이라,
  두 날의 차이를 셋으로 **잔차 없이** 가른다 — ① 축 값이 움직였다 ② 축이 생기거나
  사라져 **분모가 바뀌었다** ③ 커버리지가 바뀌어 중립으로 당기는 정도가 달라졌다.
  남는 것은 정수 반올림뿐이고 그 값도 `roundingResidual`로 적어 내보낸다.

  **그래서 보인 것 — 점수의 하루 변동은 시장이 아니라 축 유무가 만든다.** 실측
  전일 대비: **AAPL +11점 중 +10.75가 「가치 축이 사라져서」**(회사엔 아무 일도
  없었다) · 005930 +2 중 −0.65가 심리·수급 축 소멸 · 035720 −2 중 −1.62가 같은 원인.
  이걸 「추세가 좋아졌다」로 설명하면 우리 점수는 신뢰를 잃는다. **설명 기능을 만들기
  전에는 이 사실이 보이지도 않았다.**

  **없음을 0으로 때우지 않는다.** 축이 빈 이유를 `no_data`(아직 없다)와
  `not_applicable`(ETF라 재무제표가 원래 없다)로 가르고, 버린 값은 도메인과 함께
  그대로 보여 준다(035720 EPS 성장률 1170.9%, 도메인 −1000~1000 — 성장 축이 25인
  이유가 그것이다). 비교할 이전 회차가 없으면 `no_prior_row`, 한쪽 날의 점수가
  보류였으면 `score_withheld`로 **거절한다** — 0으로 답하지 않는다.

  **전주는 「7회차 전」이 아니라 달력 7일 이전의 가장 가까운 회차다.** 점수 배치가
  매일 도는 것이 아니라서(실측 26/30) 회차로 세면 종목마다 다른 날을 가리킨다.

  요구 8(신뢰성 표기)의 점수 부분도 같이 들어갔다 — 응답이 `provenance`로 기준일 ·
  계산 시각 · `actual`/`partial` · 출처 표를 들고 나간다.

  파일: `model/score_explain.go` · `service/stock_score_explain.go` ·
  `service/stock_score_explain_test.go`(17개 — 고정값을 지어내지 않고 **운영 DB에서
  읽은 실제 행**으로 잠갔다) · `handler/score.go: Explain` · router 1줄.
  go build·vet·test 전체 통과. Tesseract OK.

  **남긴 것**: FE 없음 — 화면은 **제안 후 착수**한다(메모리 규칙). Phase 2~9는
  `docs/V2_PLAN.md`에 단계로 적어 두었고, Phase 2(백테스트)는 `stock_scores`가
  30거래일만 보관한다는 제약 때문에 **얇은 랭킹 이력 테이블**(연 35MB)이 먼저 필요하다.

- **09-17 (뉴스가 늦게 뜨던 이유는 캐시가 아니라 수집이었다 — 사용자 요청 1건 · BE·FE · 배포됨 · 미커밋)**

  「최근 15분간 기사가 없는 줄 알았는데, 페이지를 보고 있으니 3분 전~15분 전 기사가
  갑자기 한꺼번에 뜬다. 캐시만 먼저 보는 것 아닌가」 — **읽기 캐시는 범인이 아니었다.**
  목록 캐시는 15초짜리이고, 폴러가 새 기사를 넣으면 TTL을 안 기다리고 버린다
  (`dropListCacheForMarket`). nginx도 `proxy_cache off`이고 목록 fetch는 `no-store`다.

  **늦은 것은 수집이었고, 원인은 큐 하나를 한 줄로 돌린 것이었다.** 국내 검색어 큐는
  추적 종목 이름 199 + 관리자 키워드 26 = 225개인데 네이버는 2분마다 **18개씩만**
  훑었다. 한 바퀴 25분 → 평균 지연 12.5분. 실측이 그대로 맞았다 —
  `published_at→created_at` 최근 12시간, naver 709건 **평균 723초**(12.05분) · 최소 97초 ·
  최대 3,042초. 국내 기사의 95%가 이 경로다(어제 naver 1,412 대 rss 31·yonhap 38).

  **한도를 올리는 것은 답이 아니었다** — 7일 생산성을 세어 보니 50건 이상 낸 검색어가
  30개, 10~49건이 33개, 나머지 171개는 10건 미만이고 그중 83개는 **7일 동안 0건**이다.
  즉 뉴스가 안 나오는 종목이 시장 키워드와 같은 자리를 차지하고 있었다. 틱당 개수만
  올리면 그 구조를 둔 채 값만 미는 일이 된다.

  **그래서 큐를 생산성 차선 셋으로 갈랐다**(`news_keyword_lanes.go`). 차선마다 커서가
  따로 돌고, 예산(틱당 22 = hot 14 · warm 4 · cold 4)은 `splitLaneBudget`이 나눈다:

  | 차선 | 기준(7일) | 검색어 | 회전 |
  |---|---|---|---|
  | hot | 50건+ | 30 | **4.3분** |
  | warm | 10건+ · 관리자 키워드 | 33 | 16.5분 |
  | cold | 그 밖 | 171 | 85분 |

  **관리자 키워드는 0건이어도 cold로 안 내려간다** — 방금 추가한 말이 85분 뒤에 처음
  불리면 사람이 고른 의미가 없다. **꼬리는 느려진다**(25분 → 85분). 그 자리는 주당
  1건 남짓을 내는 종목들이라 하루 35건 안팎(국내 1,400건의 2.5%)이 늦어지고 나머지
  97%가 세 배 빨라진다 — 이 교환이 이번 변경의 전부다. 해외 구글 뉴스도 같은 함수를
  쓴다. 생산성 집계는 1시간 캐시(`NewsRepo.KeywordArticleCounts`, `(market, published_at)`
  인덱스, `keywords_json[0]`만 센다 — 그 칸이 **그 기사를 데려온 검색어**다).

  **나머지 절반은 화면이었다.** 뉴스 목록은 5분 주기라(`SCREEN_REFRESH_MS`) 수집이
  아무리 빨라져도 최대 5분이 그대로 얹혔다. 뉴스 화면만 `NEWS_REFRESH_MS`(90초)로
  내렸다 — 아래로는 클라이언트 캐시 30초·집계 60초(`NEWS_AGGREGATE_TTL`)보다 길어야
  왕복이 실제로 일어나고(`screen-refresh.test.ts`가 잠근다), 위로는 수집 틱 2분보다
  촘촘해도 새로 올 것이 없다. **홈은 5분 그대로다** — 카드가 열 장이고 가장 많이 열리는
  화면인데 DB는 1 ECPU 하나다.

  **비용**: 네이버 일 13,250 → 15,840 / 한도 24,000(55% → 66%). **카카오는 손대지
  않았다** — 이미 일 2,450 / 3,000(82%)이다.

  **실측(배포 후)**: 네이버 `minuteCount`=22(= 새 예산), Redis에 `news:domestic:lane_cursor:v1:{hot,warm,cold}`
  세 개가 각자 전진, `/news/domestic`을 열어 두면 90초마다 목록·브리핑·언급 종목이 한 번씩 더 나간다.
- **09-16 후속 (홈 밖의 화면도 늙지 않는다 · 지수 해설이 한 거래일 뒤에 있던 이유 · 컨테이너 로그 일제 점검 — 사용자 요청 3건 · FE·BE · 배포됨 · 미커밋)**

  **① 갱신을 홈 밖으로 넓혔다.** 09-16 앞 작업은 홈만 고쳤고(`lib/dashboard-refresh`),
  나머지 화면은 그대로 마운트 시각에 묶여 있었다 — 「실시간」을 내건 뉴스 목록이
  열어 둔 탭에서는 새 기사를 한 건도 받지 않았다. 파일을 `lib/screen-refresh`로
  옮겼다(`SCREEN_REFRESH_MS` 5분 · `SCREEN_SLOW_REFRESH_MS` 30분). **이름이 곧
  범위**라, 「dashboard-」로 두면 다음 사람이 홈 전용 규칙으로 읽는다.

  주기는 **뒤에서 값을 만드는 주기**가 정한다 — 「메뉴 중요도」의 실질은 이것이다.
  5분: 뉴스 목록·브리핑·언급 종목(수집이 쉬지 않는다) · 오늘의 키워드 · 많이 본
  뉴스(조회수는 사람이 지금 만든다) · 리포트 피드(사람이 아무 때나 쓴다).
  30분: AI 추천 랭킹 · 실적 일정/집계 · 다가오는 일정 · 목표가 예측(전부 하루 한 번
  도는 배치의 산물이라 5분 주기는 스물아홉 번이 헛걸음이고, 여기 DB는 1 ECPU다).
  `/market` 랭킹과 보유·관심의 시세는 이미 폴링이 있어 손대지 않았다.

  **갱신은 스피너를 띄우지 않고, 실패해도 화면을 비우지 않는다.** 뉴스 화면은 새로고침
  단추와 **같은 문**(`refreshKey`)으로 들어가되 `silentRefreshKey`로 갈랐다 — 표식이
  불리언이면 먼저 도는 이펙트가 내려 버려 나머지 둘(브리핑·언급 종목)이 「조용한
  회차였다」를 알 수 없다. 그래서 **회차 번호**를 든다.

  **실측**: `/news/domestic`을 열어 두고 301초에 `/news/domestic?sort=latest` ·
  `/briefing` · `/symbols?limit=6`이 **한 번씩 더** 나갔다(브라우저 패널은 숨어 있으면
  `visibilityState`를 강제해야 폴링이 산다 — 09-15 메모).

  **② 브리핑 상단 지수 칩이 눌린다.** 같은 카드에서 아래 「많이 언급된 종목」은
  눌리는데 위 지수 줄만 안 눌렸다. 창은 전역 제공자(`QuickLookProvider.openIndex`)가
  연다 — 카드가 모달을 직접 들면 지수 스냅샷과 점수 훅을 한 벌 더 들어야 하는데,
  제공자는 **고른 순간에만** 그것들을 마운트한다.

  **③ 「지수 LLM이 안 도는 것 같다」 — 세 개의 서로 다른 고장이었다.**

  **원인 A: 한 번의 실패가 열셋을 데려갔다.** DB로 보니 열세 지수 중 열하나의 해설이
  점수보다 **정확히 한 거래일 뒤**에 있었고(화면은 점수가 앉은 행을 읽으므로 그 자리는
  규칙 문장뿐이다), `stock_scores` 09-16 00:25 회차가 **생성 0 · 실패 11 · 토큰 0**.
  토큰 0이면 모델은 한 번도 답하지 않았고, 실패가 **연속 열하나**라는 모양이 원인을
  좁힌다 — 따로 열한 번 실패한 게 아니라 **한 번 실패한 뒤 나머지가 딸려 갔다.**
  길이 둘이었다: 간격 대기(`waitInterval`)가 **성공한 회차에만** 있어서 실패 뒤 다음
  호출이 곧바로 나가 `ErrPacing`에 막혔고, 429를 받으면 `BlockFor`가 건 차단 구간에
  나머지가 전부 `ErrExhausted`로 떨어졌다. 이제 간격은 **성공·실패를 안 가리고**,
  `ErrExhausted`를 만나면 남은 지수를 부르지 않고 `HaltReason`에 사유를 적는다 —
  「실패 11」과 「예산이 닫혀 11칸을 못 물어봤다」는 기록에서 갈라져야 한다.

  **원인 B: 실패한 날을 하루 통째로 잃었다.** 자동 호출자가 00:25 하나뿐이라 그
  회차가 실패하면 다음 회차는 24시간 뒤다. **16:30 만회 슬롯**을 넣었다
  (`runIndexNoteCatchupSchedule`). 시각은 우연이 아니다 — Gemini 무료 한도는 태평양
  자정(**16:00 KST**)에 리셋되고, 00:25는 그 날치의 8시간째이자 17:10 국내 요약 배치가
  하루 토큰의 70%를 가져간 **뒤**다(실측 09-16: 706,392토큰 · stop=quota). 지수 해설은
  줄곧 **남은 것으로** 부르고 있었다. `force`는 쓰지 않는다 — 만회는 빈 칸을 채우는
  일이고, 채울 것이 없으면 SELECT 열셋으로 끝나므로 「실패했을 때만」이라는 조건을
  따로 들 이유가 없다(조건은 또 하나의 틀릴 자리다).

  **원인 C — 표에서 한 줄이 빠져 있었다.** `us10y`가 `dailyCloseIndexSchedules`에
  없었다. 없는 키는 빈 ScheduleID라 어떤 세션과도 같지 않고, `IngestIndexCloses`가
  **오류도 로그도 없이 건너뛴다.** 라이브 값은 FRED에서 바로 읽으므로 홈 스트립과
  브리핑은 멀쩡했고 — **일별 종가에 기대는 것만** 2026-08-27에 멈춰 있었다(모달의
  종가표·추세 점수·키워드·LLM 해설, 3주). 남아 있던 08-27까지의 행은 폴러가 아니라
  백필 명령이 넣은 것이었다(`source='fred'`). 이 표는 `marketIndexCatalog`와 같은 수여야
  하고, `TestEveryCatalogIndexHasACloseSchedule`이 그것을 잠근다.

  복구: `backfill-index-closes -since=2026-08-20`(225행 · 13개 재채점) → `index-notes`
  두 번(11건 + 9건, 토큰 13,830). 지금은 **열두 지수 전부 해설이 점수와 같은 행에**
  있다. 남은 하나는 `gold` — 27봉뿐이라 `indexNoteMinBars`(30)에 못 미친다. 날짜 있는
  이력 소스가 없어 일별 배치로만 쌓이므로 사흘쯤 뒤 저절로 들어온다.

  **④ 컨테이너 13개 24시간 로그 점검.** 5xx 0건 · nginx 오류 0건. 찾은 것:
  `/earnings/highlights`가 24시간에 15번 **3초**를 넘겼고(캐시 만료 구간, 홈이 부른다),
  `stock summary llm fallback …context deadline exceeded` 8건, `toss api failover …
  primary exhausted` 15건(설계된 동작이지만 MARKET_DATA 여유가 줄고 있다는 신호),
  kakao 일일 2,224/3,000(74%), `/reports`에서 **React #418**(하이드레이션 불일치 —
  `daysUntilDue`가 렌더 중에 `Date.now()`를 읽어 SSR과 클라이언트의 D-N이 갈릴 수 있다),
  공개 저장소 이후 `.git/config`·`.env` 스캔 트래픽, 번역기 프롬프트 캐시 축출 436회
  (truncated는 0 — 품질 영향 없음), worker-fetch가 종목당 한 줄씩 4,353줄.

  fe tsc 0 · eslint 변경 파일 새 오류 0(`use-score-rankings`의 2건은 이번 작업 전부터
  있던 것) · vitest **332/332** · go build·vet·test 전부 통과(+1 시험). BE·FE 배포,
  라이브 확인(지수 칩 → 모달에 LLM 문단 · 뉴스 5분 갱신 실측 · 실적·랭킹·리포트 3화면
  콘솔 오류 0).

  **남긴 것**: ① `/reports`의 #418은 원인만 짚고 고치지 않았다(요청 밖 화면). ②
  `daysUntilDue`류 「렌더 중 시계 읽기」가 다른 화면에도 있는지 훑지 않았다. ③ 워치독이
  `index_notes`를 새로 감시하므로 **내일 16:30까지 「늦음」 한 건**이 떠 있다(첫 슬롯이
  아직 안 돌았다는 사실 그대로다). ④ 메모장(`/journal`)은 주기를 안 붙였다 — 본인만
  쓰는 데이터라 화면 밖에서 바뀌지 않는다.

- **09-16 (해외 뉴스가 19시에 멈춘 이유 · 번역을 로컬 GPU로 되돌렸다 — BE·인프라 · 미커밋)**

  **증상은 둘인데 원인은 한 줄이었다.** 해외 뉴스 목록이 18:11 기사에서 멈췄고
  해외 브리핑이 19:01에 멈췄다(국내는 둘 다 정상). 원인은 `news_repo.go`의
  `usTranslationReadySQL` — 해외 목록·카운트 질의가 `COALESCE(headline_ko,'') != ''`를
  붙인다. **번역이 안 되면 기사가 늦는 게 아니라 안 보인다.** 수집은 멀쩡했고
  (finnhub 회사 뉴스 하루 2,589호출, 마지막이 1분 전) Azure 번역만 10:14에 멈춰
  있었다. 그 뒤로 화면에 남은 것은 번역이 필요 없는 **한국어 소스 기사뿐**이었고,
  그마저 18:11이 마지막이었다. 브리핑은 `buildFacts`가 그 필터된 목록을 읽어
  `LatestPublishedAt`이 안 밀리자 슬롯마다 「새 기사 없음」으로 건너뛰었다.

  **번역이 왜 10:14에 말랐나 — 이틀 전의 낭비가 이달 내내 갚인다.** Azure F0는 월
  200만 자이고 `SpreadMonthlyTokens`가 남은 한도를 남은 날로 나눈다. 09-13에
  426,939자, 09-14에 **1,092,484자**(12,812요청)를 썼고, 그래서 09-15의 하루치가
  30,036자였다 — 실측 수요(하루 8만~9만)의 3분의 1이다. **정상값을 DB로 재구성해
  검증했다**: 그 이틀에 들어온 영어 제목과 그날 처리된 밀린 구간을 합치면
  **341,121자**(09-13 263,604 · 09-14 77,517)여야 했다. 실제는 1,519,423자 —
  **1,178,302자, 월 한도의 59%가 같은 제목 반복 전송이었다.** 09-14만 보면 14.1배
  (요청/성공 ≈ 19회). `runUSTranslationBatch` 주석이 예고한 그대로이고, 재시도 상한은
  한도가 이미 타 버린 뒤에 들어왔다. 지금 상한(3)에 걸려 영구 제외된 행은 **1건뿐**이라
  「무덤」 가설은 기각됐다.

  **9월 남은 기간은 로컬로 돈다** (`TRANSLATE_PROVIDER=local`). 이 컨테이너를 내렸던
  이유 셋(건당 0.9~2.8초 · 상시 1.5코어 · 적재 중 통째로 영어)은 전부 **CPU 전용이라는
  전제**에서 나왔는데, 이 기계에는 RTX 4070 12GB가 있고 Docker Desktop에 nvidia 런타임이
  이미 있다(컨테이너에서 `nvidia-smi` 확인). 이미지를 `server-cuda`로, 모델을
  **LMT-60-4B Q8_0**로(같은 계열이라 프롬프트·자리표시자·glossary가 그대로 유효),
  `-ngl 99`·슬롯 4·메모리 6G(예전 2G는 실측 RSS 1.6GB에 여유 0.4GB뿐 — OOM의 가장
  유력한 자리)로 올렸다. VRAM 6.1GB 점유, 드레인 중 GPU 94%.

  **볼륨 경로가 바뀌어 있었다.** 요즘 `-hf`는 모델을 `/root/.cache/huggingface/hub`에
  받는데 볼륨은 `/root/.cache/llama.cpp`에 걸려 있었다 — 4.3GB가 마운트 **밖**으로
  받아지고 컨테이너를 다시 만들 때마다 또 받는다. 캐시 뿌리(`/root/.cache`)를 통째로
  걸었다.

  **이름 잠그기에 구멍이 둘 있었다** (4B 실측 20건에서 드러났다). ① 카탈로그에 없는
  이름은 모델이 **낱말로 읽는다** — Perplexity→「당혹감」, Anthropic→「인간학」,
  Yahoo Finance→「야후 재정」. 상장사만 보던 `namePairsFor`로는 못 막으므로
  `builtinNamePairs`를 뒀다(30일 실측에서 헤드라인 꼬리 출처는 Yahoo Finance만
  1,374건). ② 모델이 **달러 기호만 떨어뜨리면**(`$QPZAA`→`QPZAA`) 되돌리기가
  실패했다 — 20건 중 1건. 둘을 고치고 다시 재니 **30건 중 30건 통과·이름 유실 0**.

  밀린 2,895건(7일 창 **밖** 580건 포함)을 비우려고 `cmd/backfill-translations`와
  `ListMissingTranslationWindow`를 만들었다 — 창은 문자에 돈이 붙는 엔진을 전제로 한
  절약이라 로컬로 도는 동안에는 지킬 것이 없다. 원문은 **한 번도 덮인 적이 없다**
  (제목 빈 행 0 · URL 100%) — 번역은 `headline_ko`라는 별도 칸에만 쓰인다.
  **결과: 2,895 → 36건**(초당 2.3~2.4, 합 13분 25초). 남은 36건은 상한 3에 닿은
  것들이고, 대부분 **한 제목에 회사 이름이 셋**인 줄이다(「Transocean 6%, Valaris 5%,
  Noble 4%」) — 자리표시자 하나만 유실돼도 설계상 실패다. 목록에서는 계속 안 보인다
  (사용자 결정: 숨김 유지).

  **드레인 도중 번역기를 재시작했다가 615건을 태웠다 — 그리고 그게 코드 결함을
  드러냈다.** `Local`이 요청 실패를 조용히 삼키고 빈 결과만 돌려줘서, 부르는 쪽이
  「대답은 왔는데 쓸 수 없다」로 읽고 `translate_attempts`를 올렸다. 몇 초 만에
  615건이 상한에 닿아 큐에서 **영구히** 빠졌다 — `translateHeadlines` 머리글이
  「못 보낸 회차는 세지 않는다」고 적어 둔 바로 그 규칙이 로컬 엔진에서만 안 지켜지고
  있었다. **보낸 것이 전부 실패했을 때만** 오류를 올리도록 고치고(한 건 실패는 그대로
  부분 성공), 태운 615건의 카운터를 0으로 되돌려 다시 돌렸다.

  메모리는 6G로도 모자랐다 — GPU에 전층을 올렸는데 호스트 `anon`이 6.2GB였다
  (WSL2 CUDA는 VRAM 할당을 호스트 익명 메모리로 잡고, 페이지 캐시와 달리 회수되지
  않는다). `memory.events`의 max가 100,937회, VM 스왑 3GB, 드레인 6.8→1.5건/초.
  oom_kill은 0이었지만 그건 운이다 — 10G로 올리니 4.2GB에서 안정됐다.

  **시험을 쓰다 운영 표에 시험 행 둘을 남겼다** — `t.Cleanup`이 `defer db.Close()`
  **뒤에** 돌아 닫힌 풀에 DELETE를 쐈고, 오류를 `_ =`로 삼키고 있었다. defer로 바꾸고
  오류를 올리게 했다. 시험 +9건(translate 8 · repository DB 1, 후자는
  `NEWS_TRANSLATE_TEST_DSN` 있을 때만). BE 전체 통과·재빌드·13/13 헬시, 라이브에서
  해외 목록이 12분 전 기사까지·브리핑 01:36 재생성 확인.

  남은 것: Azure는 10월 1일에 리셋되지만 실측 수요가 월 2.17M자라 F0로는 사고가
  없어도 모자란다 — 로컬 상주냐 S1 유료(월 약 $22)냐는 미결. 그리고 **로컬로 도는
  동안 `usTranslationReadySQL` 게이트는 여전히 단일 실패점이다** — 번역기가 죽으면
  새 기사가 목록에서 사라지고 브리핑이 멈춘다(오늘 증상 그대로).

- **09-16 후속 (「탭을 계속 띄워놓고 있었는데 제대로 안 바뀌던데」 · 배포됨 · 미커밋)**

  갱신을 붙여 배포한 뒤에도 사용자의 탭은 그대로였다. 원인이 둘이다.

  **① 이미 열려 있던 탭에는 새 코드가 없다.** FE는 컨테이너라 배포 시각(09-16 00:20
  KST) **이전에 연 탭은 옛 번들**을 들고 있다 — 새로고침 한 번이 필요하다. 이건 이번
  변경의 성질이지 버그가 아니다.

  **② 진짜 결함: 「회차가 울렸는가」로 판정하고 있었다.** `useVisibleInterval`은 숨어
  있는 동안 회차가 울리면 표식을 켜 두었다가 돌아올 때 메웠는데, 그 전제가 **숨은
  탭에서도 타이머가 돈다**는 것이었다. 크롬은 5분 넘게 숨은 탭의 타이머를 분당 한
  번으로 내리고, 조건이 맞으면 탭을 **얼린다**(콜백 0회). 창이 다른 창에 완전히
  가려져도 `hidden`이고, 노트북을 덮으면 타이머는 멈춘 채 시각만 흐른다. 그래서 몇
  시간이 지나도 표식은 false — 돌아와도 **아무 일이 없었다.**

  이제 **마지막으로 부른 시각** 하나만 센다(`lib/refresh-due`, 시험 6). 깨어날 기회는
  넷(회차 · 다시 보임 · 창 포커스 · bfcache `pageshow`)이지만 **질문은 하나다** —
  「마지막으로 부른 지 한 주기가 지났는가」. 같은 함정을 `lib/stream-awake`가 먼저
  밟았고 거기 적어 둔 문장이 그대로다: **「울렸으니 지났다」는 믿을 수 없다.** 부수
  효과로 30초 숨었다 돌아올 때의 헛걸음도 사라졌다 — 값이 아직 한 주기보다 젊으면
  부르지 않는다.

  이 훅은 홈만의 것이 아니다 — 보유·관심·관심 실험실·종목 상세 펀더멘털 폴링이 같이
  고쳐졌다. fe tsc 0 · eslint 변경 파일 0 · vitest **332/332**(+6). 앞선 기록의
  `list-price-hub.test.ts` 실패는 **깜빡이는 시험**이었다(연속 재실행에서 통과) —
  「이번 작업 전부터 있던 실패」라는 표현은 정확하지 않았다.

- **09-16 (홈이 열어 둔 채로 늙지 않는다 — 사용자 요청 1건 · FE만 · 배포됨 · 미커밋)**

  홈의 카드는 저마다 `useEffect`에서 **한 번 받고 그대로 서 있었다.** 가격만은
  스트림과 폴링이 밀어 줘서(`use-live-quotes` · 지수 30초) 숫자가 움직였고, 그래서
  화면이 살아 있는 것처럼 보였다 — 정작 「오늘의 시장 브리핑」과 「실시간 뉴스」는
  마운트 시각의 글이었다. 하루를 열어 두면 **어제 브리핑에 「오늘」이라고 적혀 있었다.**

  주기는 **뒤에서 값을 만드는 주기**를 따른다(`lib/dashboard-refresh`). 5분 — 브리핑
  (배치가 장중 30분·그 외 1시간) · 실시간 뉴스 · 오늘의 키워드(칩과 각주 둘 다) ·
  거래대금 상위(서버 랭킹 스냅샷이 5분). 30분 — AI 추천 랭킹 · 주요 실적 일정 · 지수
  스트립의 점수 배지(전부 하루 한 번 도는 배치). 하루에 한 번 바뀌는 값을 5분마다
  묻는 것은 스물아홉 번이 헛걸음이고, 여기 DB는 1 ECPU 하나다.

  **한 카드 안에서 두 시각이 섞이지 않게** 한다 — 키워드 카드는 칩을 자기가 받고
  각주의 속보 수만 부모가 준다. 부모만 갱신했더니 각주는 지금, 칩은 마운트 시각이
  됐다. 지수 스트립도 같은 모양이다(값 30초 · 점수 배지 한 번). 갱신을 붙일 때
  **그 카드가 스스로 받는 값이 따로 있는지**부터 본다.

  규칙 셋. ① **안 보이는 탭은 세지 않는다** — 전부 `useVisibleInterval`을 지나고,
  돌아오는 순간 건너뛴 회차를 한 번 메운다(09-15 요청 3에서 만든 훅). ② **갱신의
  실패는 화면을 비우지 않는다** — 첫 적재의 실패만 「없음」이고 갱신의 실패는 「아직
  그것」이다. 낡은 값을 고치러 와서 값을 통째로 없애면 더 나쁘다. ③ **주기는 캐시
  TTL보다 길어야** 실제로 왕복한다 — 실적 하이라이트의 클라이언트 캐시가 정확히
  5분이라 느린 쪽이 30분인 이유가 이것이다. 그 관계는 `dashboard-refresh.test.ts`가
  잠근다(값 하나를 만지면 걸린다).

  **남긴 것**: `useOpenMarket`의 「열린 시장이 기본값」은 마운트 때 한 번 정해진다 —
  열어 둔 채 장이 열려도 토글이 따라가지 않는다. 사람이 안 눌렀다는 이유로 보고 있던
  목록을 국내↔해외로 갈아 끼우는 편이 더 놀랍다고 봤다. 다른 화면(뉴스·마켓 등)도
  손대지 않았다 — 요청이 가리킨 곳이 홈이다.

  fe tsc 0 · eslint 변경 파일 0 · vitest 314/315(+3, 기존 실패 1건
  `list-price-hub.test.ts`는 이번 작업 전부터 있던 것). 배포 `fe/scripts/deploy.ps1`
  → `quant-port-fe` healthy. **실측**: 홈을 띄워 두고 5분 뒤
  `/news/{domestic,us}/briefing` · `/news/{domestic,us}/dashboard?full=14` ·
  `/news/domestic/keywords?limit=14` · `/rankings?...MARKET_TRADING_AMOUNT` · `/quotes/batch`가 **한 번씩 더** 나갔고, 30분 쪽(`/stocks/scores/top` · `/earnings/highlights` · `/indices/scores`)은 그 회차에 **나가지 않았다.**
  브라우저 패널은 숨어 있으면 `visibilityState`를 강제해야 폴링이 산다(09-15 메모).
  **배포 스크립트를 파이프로 감싸지 말 것** — PS 5.1이 docker stderr 한 줄을 오류로
  올려 health 대기와 nginx 재생성 앞에서 끊긴다(컨테이너는 이미 새로 떠 있다).

- **09-15 후속 (사용자 요청 4건 — 모달 요약·화면 메타 관리·안 보이는 탭·보던 종목 · 배포됨 · 미커밋)**

  **① 모달의 「간단」이 자른 글이었다.** 그 자리는 일별 해설(`ai_summary`)의 앞 두
  문장이었는데, 그 해설은 LLM이 쓴 네 갈래를 **순서대로 이어붙인 글**이라 앞 두
  문장은 언제나 밸류에이션 이야기다 — 「간단」이라는 이름을 달고 네 갈래 중 하나만
  말하고 있었다(개요 탭이 2026-09-03에 같은 이유로 고친 실패). 같은 호출이 네 갈래
  **전부를 근거로 다시 쓴** 한 문단(`overview`)을 이미 내놓고 있었지만, 그 값이 사는
  곳은 Redis 캐시(24시간)와 종목당 한 벌짜리 표라 **그날의 글이 남지 않았다.** 모달은
  「9-15 기준」이라고 날짜를 적고 글을 보여 주므로 날짜와 글이 같은 행에 있어야 한다 —
  마이그레이션 073으로 `stock_daily_closes.ai_overview`를 만들고 배치가 본문과 요약을
  둘 다 쌓게 했다. 과거 행은 소급하지 않는다(그날의 원문이 이미 없다) — 비면 화면이
  예전 방식으로 돌아간다(`lib/daily-close-ai-summary`).

  **073에서 기동이 두 번 막혔다** — `invalid connection` 뒤 `Duplicate column`.
  ALTER가 **클라이언트 연결보다 오래 살아서** 연결은 끊겼는데 서버에서는 컬럼이 붙고
  있었고, 다음 기동의 `columnExists`가 아직 false를 봤다. 같은 패키지에 이미 있던
  `isSchemaRaceError`(API + 워커 여덟이 동시에 스키마를 맞추는 배포 모양 때문에 만든
  것)를 073·074에 물렸다. **이 저장소에서 큰 ALTER를 기동에 섞을 때의 기본값**이다.

  **② 화면 제목·설명을 배포 없이 고친다** (`/admin/seo`). 탭 이름과 검색 결과 문구가
  Next 소스의 상수라 한 줄 고치려면 이미지를 다시 구워야 했다 — 2026-09-13에 별칭을
  표로 옮긴 이유와 같다. 규칙은 하나다: **`page_meta`는 덮어쓰기지 정답이 아니다.**
  빈 칸은 코드 기본값이 이기고, 전부 비우면 그 행이 **지워진다**(저장과 되돌리기가 한
  입구). 그래서 표가 통째로 없어도 사이트는 지금 그대로 돈다. 기본값 20벌을 화면
  파일에서 `lib/page-meta-routes`로 모았다 — 흩어져 있어도 사이트는 잘 돌지만
  「어떤 화면을 고칠 수 있고 지금 기본값이 무엇인가」에 실행 시점에 답할 수 없다.
  `export const metadata` → `generateMetadata()` (덮어쓰기를 **렌더 시점에** 읽어야
  한다). **루트 레이아웃까지** 함수로 바꿨다 — 홈 제목이 거기 `title.default`에 있다.
  `noindex`는 세 값이다(안 정했다/노출/제외): 둘로 합치면 한 번 뺐던 화면을 기본값으로
  되돌릴 방법이 사라진다. canonical은 그 판정을 따라간다 — 「세지 말라」면서 정규
  주소를 다는 것은 모순이다.

  **시험이 실제 버그를 잡았다**: BE의 경로 정규화가 소문자로 내리고 있어
  `/reports/authors/[userId]`가 `[userid]`가 됐다 — 저장은 되고 오류도 안 나는데
  제목만 안 바뀐다. 경로는 관리 화면에서 **고르는** 값이라 오타 방어가 필요 없어
  대소문자 보존으로 되돌렸다. 캐시 주기는 **60초**(300초에서 내렸다) — Next 재검증이
  stale-while-revalidate라 5분이면 최악 10분을 기다리며 「저장이 안 됐나」를 의심한다.

  **③ 안 보이는 탭은 서버를 두드리지 않는다.** 09-05에 정한 「숨어도 3시간 붙어
  있는다」를 사용자가 뒤집었다 — 모니터 둘에 창이 다섯씩이면 보는 것은 둘인데 열
  탭이 전부 Toss 시세를 당긴다. 유예는 **1분**(처음 30분으로 잡았다가 사용자가 직접 내렸다 — 아래 ④-2). 함께 **폴링 가드가 빠져
  있던 네 곳**을 찾아 막았다(보유·관심·관심 실험실·재무) — 가드를 `refreshQuotes`
  안에 넣으면 첫 적재까지 막히고 부르는 자리마다 `if`를 적으면 또 빠뜨리므로,
  「주기적으로 부르는 일」 자체를 `useVisibleInterval` 한 이름으로 만들었다. 숨어 있는
  동안 건너뛴 회차가 있으면 **다시 보이는 그 순간** 한 번 부른다(사용자가 적은 「즉시
  재개」). 실측: 보이는 동안 ~6초마다 1회 → 숨기면 60초 동안 **0회** → 보이게 하면
  기다림 없이 1회.

  **④ 보고 있던 종목 하나는 숨어도 남는다.** 유예가 지나도 연결을 **닫는 대신
  좁힌다** — 보유 서른 종목이 뜬 탭이 숨으면 서른이 아니라 「열어 둔 종목」 하나가
  남는다(토스가 다른 화면에서도 클릭해 둔 종목의 시세만 띄워 주는 그 규칙).
  핀은 `lib/pinned-symbols`(React 밖 — 허브가 React를 몰라야 시험이 DOM을 안 쓴다),
  거는 곳은 종목 상세 화면과 열려 있는 모달. **교집합이 핵심이다** — 핀만 보고 열면
  아무도 구독하지 않는 시장에 **새 연결이 생기고**, 상세 화면은 자기 체결 스트림을
  따로 열고 있어 같은 종목에 연결이 둘 난다. 여기서 하는 일은 좁히는 것뿐이다.

  **④-2 그 「보던 종목」이 서는 자리는 브라우저 탭 제목이다 (사용자 지적).**
  사진을 화면 오버레이로 읽고 위젯을 제안했다가 「화면 표기가 아니라 **브라우저 탭
  제목**」이라는 답을 받았다. 그 한마디가 요청 3과 4의 앞뒤를 맞춘다 — **탭 제목은
  그 탭이 안 보이는 동안에도 탭 스트립에 그대로 보인다.** 안 보이는 탭의 구독을 전부
  끊으면서도 종목 하나만 남겨야 하는 이유가 그것이었다.

  `$141.91 -1.98% | 오라클` — 가격이 맨 앞인 이유는 탭이 여럿이면 제목이 앞에서부터
  잘리기 때문이다. 어느 종목인지는 그 탭을 연 사람이 이미 알고, 모르는 것은 「지금
  얼마인가」다. 등락률은 **스트림에 없다**(가격만 온다) — 스냅샷 한 벌에서 전일
  종가를 받아 두고 새 가격이 올 때마다 다시 센다. 스냅샷의 `changeRate`를 그대로
  쓰면 「가격은 움직이는데 등락률만 멈춰 있다」가 된다.

  **탭 제목 컴포넌트가 곧 그 종목의 구독자다.** 핀은 「무엇을 살려 둘까」만 답하고
  구독을 만들지 않으므로, 종목 화면을 떠나면 아무도 그 종목을 구독하지 않아 남길
  것도 없어진다. 껍데기에 하나 떠 있는 이 컴포넌트가 그 자리를 메운다 — 실측으로
  `/news/domestic`에서 열려 있는 스트림이 `symbols=005930` **하나뿐**이었다.

  수명은 **탭이 닫힐 때까지**(사용자 결정: 「열자마자 뜨고 × 까지 유지」). 그래서
  핀을 「지금 열려 있다(active)」와 「마지막으로 봤다(lastViewed)」 둘로 갈랐다 —
  지우는 기준이 다르다(화면의 생애 vs 탭의 생애). 새로고침을 견디려고 `sessionStorage`를
  쓴다: **탭 하나에만 살고 탭과 함께 죽는** 수명이 여기서 원하는 것과 정확히 같다.

  **브라우저 실측이 버그를 하나 잡았다** — 메뉴를 옮기면 제목이 페이지 제목으로
  돌아간 채 다시 안 얹혔다. `<title>` 엘리먼트에 `MutationObserver`를 달았는데,
  Next는 라우트가 바뀔 때 그 **엘리먼트를 갈아끼운다** — 옵저버가 문서에서 떨어져
  나간 옛 노드를 보고 있었다. `<head>`를 지켜보게 고쳤다(교체까지 잡힌다).

  **유예는 30분 → 1분으로 사용자가 직접 내렸다.** 30분은 요청이 겨냥한 상황을 하나도
  못 덮는 값이었다 — 탭을 오가며 쓰는 동안에는 어떤 탭도 30분 연속으로 숨어 있지
  않아서 열 탭이 전부 붙어 있는 것과 같았다.

  라이브 실측(미국장 중, 탭 강제 숨김): 숨긴 지 40초 `$330.11` → 75초(유예 초과)
  `$330.12` → 95초 `$330.17`로 **제목이 계속 흘렀고**, 같은 20초 동안 `/quotes/batch`
  폴링은 **0회**였다. 다시 보이게 하면 즉시 1회. 두 규칙이 한 화면에서 같이 선다.
  FE 312→**326**시험.

  FE 282→**326**시험, BE 전체 통과, tsc·`next build`·`go vet` 깨끗, eslint 새 경고 0.
  BE·FE 배포 후 라이브 실측(제목 덮어쓰기·noindex·되돌리기, 폴링 가드 양방향, 모달의
  간단/상세). **확인 못 한 것**: 관리 화면 `/admin/seo`의 실제 편집 동작(관리자 로그인
  자격 없음 — 라우트 200과 API 왕복까지만), 30분 유예 뒤의 스트림 좁힘(시험은 가짜
  시계로 덮었다).

- **09-15 (워치독 주기 끊김 — 회차를 싸게 만들었다 · 미커밋)**

  사용자가 「게임하거나 무거운 작업 중에 한 번씩 크게 끊긴다, 워치독을 넣은 뒤로
  그렇다」고 알려 왔다. 실측이 원인을 갈랐다 — 한 회차 **830ms** 중 점검 자체는
  거의 없고 **점검을 준비하는 값**이 대부분이었다: `powershell.exe` 콜드 스타트
  (20KB 스크립트 AMSI 검사 포함)와 WSL2 VM 을 깨우는 `docker ps`. 로그도 같은 말을
  했다 — Docker Desktop 을 띄운 4회 중 셋은 **부팅 직후(로그온 트리거)라 의도된
  동작**이었고, 켜져 있던 PC 에서 도로 올린 것은 09-09 한 번뿐이다. 범인은
  「점검한다」가 아니라 **「점검하려고 매번 새 프로세스를 둘 띄운다」**였다.

  그래서 **평소 회차에서 PowerShell 과 docker CLI 를 뺐다.** VBS 런처가
  `ServerXMLHTTP` 로 `/health` 만 찌르고 끝낸다(**95ms · 프로세스 1개**).
  PowerShell 은 **헬스가 실패했거나 30분에 한 번 깊은 점검**(`docker ps` 훑기 ·
  `/ready`)일 때만 뜬다 — 시간당 12회 → 2회, 총 점유 ~10초 → ~2초.
  **복구 지연은 그대로다**: 헬스가 실패하면 그 회차에 바로 PowerShell 이 뜬다
  (죽은 포트로 돌린 사본으로 실측 확인). 곁들여 주기 5→10분, 우선순위
  7(BELOW_NORMAL)→10(IDLE). 늘어난 최악 복구 지연 5분은 이 워치독이 메우려던
  구멍이 **96분**이었으므로 치를 만한 값이다.

  **`watchdog.paused` 에 만료를 붙였다**(`watchdog-pause.ps1 -Hours 4`). 종전에는
  손으로 만들고 손으로 지워야 했는데 **지우는 것을 잊으면 워치독이 조용히 꺼진 채로
  남는다** — 재움은 「게임 한 판」처럼 끝이 있는 일이라 끝을 파일에 적게 했다.
  시각을 `yyyyMMddHHmmss` 로 적는 이유는 **읽는 쪽이 둘**이기 때문이다 — VBScript 의
  `CDate` 는 로캘에 끌려다녀 ISO 8601 을 믿을 수 없고, 자리수가 고정되면 두 언어
  모두 문자열 비교만으로 시각을 비교한다.

  VBScript 함정 둘을 밟았다: `until` 은 예약어(`Do ... Until`)라 변수로 못 쓰고,
  변수 이름이 함수 이름과 같으면(`deepDue` vs `Function DeepDue`) 「다시 정의된
  이름」으로 죽는다. 둘 다 `wscript` 로는 **조용히** 실패해 `cscript` 로 봐야 보인다.

  남은 가능성: 이 조치 뒤에도 끊긴다면 범인은 워치독이 아니라 **상시 떠 있는
  Docker Desktop/WSL2**(`.wslconfig` 가 없어 최대 16GB 까지 가져갈 수 있다)이거나
  **배치 워커 8개의 정기 작업**이다.

- **09-14 후속 (사용자 요청 4건 — 별칭 다중 대상 · 모달 요약 절단 · 리포트 읽기·쓰기 개편 · 예시 데이터 기본값 · 배포됨 · 미커밋)**

  **① 별칭 하나에 대상이 여럿일 수 있어야 했다 (가장 깊었다).**
  사용자가 「구글」에 GOOGL과 GOOG을 둘 다 만들어 뒀는데 검색에는 GOOG만 나왔다.
  표의 고유 키가 `(alias, kind, target)`이라 **행 둘은 처음부터 저장돼 있었고**,
  관리 화면도 둘 다 받아 줬다. 깨진 곳은 그 표를 읽어 들이는 **캐시 스냅샷**이다 —
  값이 `map[string]string`이라 나중에 읽힌 행이 앞 행을 덮었고, 읽기에 `ORDER BY`가
  없어 **어느 쪽이 살아남는지는 MySQL이 정했다.**

  `map[string][]string` + `ORDER BY id`로 고치고 **쓰는 쪽을 둘로 갈랐다** — 이
  갈림이 핵심이다. **검색은 전부를 본다**(어느 쪽을 찾는지는 검색한 사람만 안다).
  **태깅은 첫 칸 하나만**(`PrimarySymbolNameAliases`) — 둘 다 붙이면 「구글 실적
  발표」 한 줄에 같은 회사가 두 번 서고 시세도 두 벌이 붙는데, 기사는 회사 이야기지
  주식 종류 이야기가 아니다. 대표는 **먼저 적은 것**이라 나중에 대상을 덧붙여도
  이미 붙은 태그가 조용히 옮겨 가지 않는다.

  고치다 **끈 행 하나가 별칭 전체를 지우던 버그**를 같이 잡았다(`delete(snap.brand,
  alias)` — 대상이 하나뿐이던 시절엔 「끄기 = 삭제」가 같은 뜻이었다). 행 접는 규칙을
  `sql.Rows` 반복문 **밖으로** 꺼내 `aliasFold`로 만들었다 — 그 버그가 시험에 안
  닿은 이유가 정확히 반복문 안에 있었기 때문이다. 배포 후 실측: 「구글」 → GOOGL·GOOG
  둘 다 상위. 관리 화면에 `대상 N` 배지를 붙였다(나란한 두 행이 「같은 별칭의 두
  대상」인지 「중복 등록」인지 표만 봐서는 알 수 없다 — 사용자가 헷갈린 자리가 거기다).

  **② 종목 모달의 「간단」이 낱말을 쪼갰다.** `line-clamp-3`은 **줄 수**로 자르므로
  끊기는 자리를 글자 너비가 정한다(사용자 화면: 「…효과를 상쇄하」). 개요 탭은 원래
  그렇게 안 한다 — 서버가 갈래마다 **첫 문장**만 뽑아 엮은 요약을 쓴다. 모달엔 그
  서버 요약이 없어 같은 규칙을 화면에서 적용했다(`firstSentences`, 앞 두 문장).
  마침표 **뒤에 공백**이 오는지 함께 봐야 「PER 135.4배」의 소수점에 안 걸린다.
  요약이 원문과 같으면 토글을 아예 안 세운다.

  **③ 리포트 읽기는 다시 2단 — 세 번째다.** ①2단(좁은 화면에서 순서가 뒤집혀 폐기)
  → ②한 컬럼·계기판을 본문 뒤로(09-14 오전) → ③2단. ②의 진단은 맞았지만 대가가
  있었다: **읽는 도중에는 목표가를 볼 수 없다.** ①이 틀렸던 것은 배치가 아니라
  **좁은 화면의 처리**였고, 지금은 한 DOM에서 **폭이 순서를 정한다**(`lg` 미만이면
  계기판이 ②가 정한 자리로 그대로 쌓인다). 반응 줄을 제목 바로 밑으로 올렸다 —
  커뮤니티의 글인데 추천·댓글 수를 다 읽은 사람만 볼 수 있었다.

  **쓰기는 본문을 2순위로.** 사용자가 「작성할 때 너무 사용성이 떨어져서 쓰고 싶지가
  않다」고 적은 자리다. 입력칸에 닿기까지 카드 다섯 장을 지나야 했는데, 실은 의견 값에
  전부 기본값이 있어 **하나도 안 만져도 발행된다.** 의견은 접고 머리줄이 현재 값을
  한 줄로 말한다(오류는 접힌 채로도 밖으로 나온다). 댓글은 추천(migration 072)·
  작성자 배지·활동량 배지(구간으로 접는다)·상대 시각·답글 접기. 답글·수정 입력칸이
  **아직 한 줄이었다** — 09-14 오전에 본 댓글만 고치고 옆 칸을 빠뜨린 것이다.

  **④ 예시 데이터를 묻지 않는다.** 안내 카드로 먼저 묻던 것을 **바로 켜는** 것으로
  바꾸고, 카드는 **예시를 끈 사람**에게 보이도록 자리를 옮겼다. 켜는 층이 셋인데
  (사용자 선택 > 주소창 > 자동) **사용자 선택이 언제나 이긴다** — 끈 사람에게 「아직
  데이터가 없으니까」라며 다시 켜 주면 그 버튼은 눌러도 안 되는 버튼이 된다.

  **브라우저 실측으로 버그 하나를 잡았다.** 한 화면에서 「내 것이 없나」에 답하는 곳이
  **둘**이다 — 껍데기(로그인 안 함 → 곧바로 참)와 본문(목록 0건 → 받아 보기 전엔
  거짓). 값을 한 칸에 덮어쓰니 서로를 지웠다: 껍데기가 켜서 본문이 마운트되고 → 본문이
  「아직 모른다」로 덮어 꺼지고 → 화면엔 고치려던 그 카드가 그대로 남았다(로그
  `true→false→false`). 기여자마다 한 표씩 들고 **OR로 합치도록** 바꿨다. 판정 규칙
  전체를 `lib/sample-mode-state.ts`로 떼어 시험을 붙였다 — 이 규칙은 **끌 때만
  고장나는** 종류라 눈으로는 안 갈린다(09-02에도 같은 모양의 버그가 있었다).

  **⑤ 마이그레이션 배선을 세는 시험.** 072를 넣다 봤다 — 적용되는 길이 둘이고(빈 DB는
  전부 자동, **도는 DB는 손으로 등록**) 잊으면 운영 DB에만 표가 없는데 개발에서는
  증상이 0이다(늘 빈 DB로 시작한다). `TestEveryMigrationFileIsReferenced`가 센다.
  일회성 8건은 **이유와 함께** 면제했다 — 이유 없이 목록만 늘면 껍데기가 된다.

  FE 250 → **282**시험, BE 전체 통과, `go vet`·tsc·eslint 새 경고 0. BE/FE 배포·
  라이브 검증. **확인 못 한 것**: 댓글 추천 **클릭** 경로(관리자/로그인 자격이 없어
  라우트 401과 응답 필드까지만 실측), LLM 해설이 있는 종목 모달 화면.

  릴리스 노트: `docs/releases/v1/v1.15/v1.15.0.md`
- **09-14 (사용자 요청 4건 — Azure 번역 실패 루프 · 별칭 관리 화면 · AI 카드 정리 · 리포트 읽기 개편 · 배포됨 · 미커밋)**

  **① Azure 번역이 이틀 동안 사실상 전부 실패하면서 청구서만 쌓였다 (가장 컸다).**
  사용자 요청은 「현재 문제 없는지 확인」이었는데, 실측이 정반대였다. 회사 이름을 잠그는
  자리표시자 `$QPZ0`을 Azure가 **`$QPZ 0`으로 쪼개거나 `$QPZ년`으로 망가뜨린다** — 그
  모양은 **자체 모델(LMT-60) 기준으로 고른 것**이고 엔진을 바꾸면서 그대로 깨졌다.
  `RestoreNames`가 실패하면 빈 문자열을 주고(주어 잃은 제목을 안 내보낸다는 규칙),
  빈 `headline_ko`는 다음 회차 질의에 **또 잡힌다.** 실측 09-14: 요청 **12,559건** ·
  **1,069,514자**, 그날 새 기사는 **511건** — **약 96%가 낭비**. 9월 누계가 이틀 만에
  **1,496,453 / 2,000,000자(74.8%)**였다.

  고친 것은 접미사를 숫자에서 **글자**로 바꾼 것이다(실측: `$QPZ0` 0/3 · `$AAPL0` 0/3 ·
  `$QPZA` **3/3**). 두 글자 고정폭 — 한 글자면 이름 27개째부터 `$QPZA`가 `$QPZAA`의
  접두가 되어 되돌리기가 서로를 먹는다. Azure 공식 동적 사전(`<mstrans:dictionary>`)도
  4/4 통과했지만 **마크업이 그대로 과금돼**(이름 하나에 70자, 자리표시자는 6자) 기각.

  **원인은 하나였지만 구조에 셋을 뒀다** — 다음 실패도 같은 모양으로, 조용히 돌아오기
  때문이다. ㉠ **재시도 상한**(`translate_attempts`, migration 071) 세 번이면 영어로
  남긴다. **예산에 걸려 못 보낸 회차는 안 센다** — 그건 제목의 문제가 아니라 그날
  형편의 문제라, 한도 찬 날이 그 제목들의 기회를 영구히 태우면 안 된다. ㉡ **묶음
  요청** — 100건을 싣는 API에 1건짜리 슬라이스를 스물네 번 보내고 있었다(배포 후 실측
  24건 → 요청 1회). ㉢ **7일 창**(`translationWindowDays`) — 상한에 닿은 행의 무덤을
  매 회차 지나가지 않는다.

  **그리고 하루치를 상수로 두지 않는다.** 월 한도만 있으면 다 쓴 순간 남은 날이 통째로
  0이 되고, 고정 하루 한도(2M/31=64K)는 **이미 과하게 쓴 달을 못 고친다**(22일쯤 월 벽).
  `SpreadMonthlyTokens`가 매번 계산한다 — `(월 한도 − 어제까지) / 오늘 포함 남은 날`.
  평상시 64K, 이번 사고 다음 날 기준 **약 30K/일**. 나누는 기준이 「어제까지」인 이유는,
  오늘 쓴 것까지 빼면 하루치가 쓸수록 줄어 끝에 닿지 않는 계단이 되기 때문이다.
  **소진 로그는 상태가 바뀔 때만 적는다** — 수집 경로가 새 기사마다 부르므로 그대로
  찍으면 하루 수백 줄이고, 그런 로그 옆에서는 진짜 오류도 안 보인다.

  배포 후 08:22~08:45 **23분간 문자 증가 0**(그 전에는 시간당 약 45K). 검증용으로 한도를
  잠깐 열어 2회 요청에 50여 제목을 옮기고(3,916자) 이름 복원까지 확인한 뒤 카운터를
  실제 값으로 되돌렸다.

  **② 별칭 관리 화면을 만들었다** (`/admin/aliases`). 09-13에 표로 옮긴 이유가
  「배포 없이 고친다」였는데 화면이 없으면 그 이유가 절반만 이뤄진다. DB 콘솔로 안 되는
  이유 셋: 갈래마다 「대상」의 뜻이 다르고(brand=영문 표기 소문자 / symbol=심볼 대문자),
  **심볼 오타가 조용히 실패하며**(없는 심볼은 카탈로그 대조로 거부한다), 쓰기 뒤 캐시를
  다시 읽어야 한다. 한 글자 별칭도 거부 — 「A」 하나면 세상의 절반이 그 종목에 붙는다.
  실제 DB에 대고 한 바퀴 확인했고(추가 → 중복 409 → 캐시 반영 → 끄기 → 캐시에서 빠짐 →
  같은 값 저장 오류 없음 → 삭제 → 404 → 카운트 원복 76/122/0) 넣은 행은 지웠다.
  **관리자 로그인 자격이 없어 화면 클릭 확인은 못 했다** — 서버 쪽만 실측이다.

  **③ AI 카드를 「AI 핵심 요약」 하나로 정리했다**(사용자 요청 2). 생성 시각 줄과
  「분석 탭 ›」 링크를 걷고(간단/상세가 생긴 순간 같은 일을 하는 문이 둘이었다) 제목을
  종목 모달과 같은 이름으로 맞췄다. **상세는 네 칸 그리드에서 이어진 산문으로** — 칸을
  나누면 넷이 다른 이야기처럼 읽히는데 실제로는 한 번의 호출이 쓴 한 편의 글이다.
  종목 모달에도 같은 토글을 달았다(기본 간단, 종목을 옮기면 다시 접힌다).

  **④ 리포트는 글을 누르면 글이 나오게 했다**(사용자 요청 4). 카드를 누르면 계기판
  모달이 떴고 **그 모달에는 댓글이 없어서**, 커뮤니티 메뉴인데 답을 다는 길이 두 번
  건너야 닿았다. 모달을 없애고 카드·레일 항목을 진짜 앵커로 바꿨다. 목록 카드에서
  스파크라인·5축·가격대 축·진행 막대를 상세로 내렸고(모바일 표지 235px 배너 → 88px
  사각형, 화면당 1장 → 3장), 상세는 **본문을 계기판 앞으로** 옮겼다(종목·목표가만 한 줄
  띠로 남긴다 — 통째로 내리면 무엇에 대한 글인지 모르고 읽는다). 댓글은 「독자 반응」
  절을 얻고 한 줄 입력칸이 여러 줄이 됐다. **비로그인 등록이 401로 조용히 죽던 것**을
  로그인 안내로 바꿨다.

  릴리스 노트: `docs/releases/v1/v1.14/v1.14.0.md`
- **09-13 (사용자 요청 5건 — 해외 뉴스 수집 정지 · 별칭 표 · AI 카드 통일 · 번역기 컨테이너 철거 · 배포됨 · 미커밋)**

  **① 해외 뉴스가 멈춰 있었다 — 원인이 둘이고 둘 다 「간격」이었다.** 사용자 지적
  「해외 뉴스를 가지고 오고 있지 못하다」의 실체다.

  **ⓐ 주말 틱 게이트가 벽시계 분을 봤다.** `skipNewsTick`은 `now.Minute()%14 != 0`
  이면 건너뛰었다. 「14분에 한 번」으로 읽히지만 그것은 **매 분 물어볼 때만** 참이다.
  우리 티커는 자기 격자 위에서만 깨어난다 — 2분 주기 티커는 한 시간에 **같은 홀짝의
  분에만** 서므로 격자가 홀수 쪽이면 {0,14,28,42,56}과 **영원히 안 겹친다**(어느
  쪽이 될지는 프로세스가 언제 떴는지가 정한다). 8분 주기 티커는 {14, 42} 둘만 잡아
  평균 60분이 된다. 실측(일요일 20시간): `finnhub_news` 호출 **10건**(의도대로면 약
  85건), 종목별 뉴스 틱 로그가 03:18·04:43·05:15·06:42·07:14·08:42·09:14·10:42 —
  **42분과 14분만** 남았다. 예전 구현을 시험에 넣어 확인했다: 해외 일반 뉴스 티커가
  최악의 기동 시각에서 **6시간에 0회**. 고친 것은 축을 바꾼 것이다 — 벽시계 분이
  아니라 **그 티커가 마지막으로 통과한 뒤 얼마나 지났는지**를 본다(`weekendTickGate`,
  게이트는 티커마다 하나 — 공유하면 먼저 깬 소스가 남의 몫을 쓴다). 새 시험은 실제
  주기·오프셋 그대로, 기동 시각을 1분씩 옮겨 가며 돌린다(**6시간에 최소 22회**).

  **ⓑ 종목별 뉴스가 32종목 중 하나만 들어왔다.** `/company-news`가 시장 전체 뉴스와
  **같은 버킷**(`finnhub_news`, 최소 간격 2분)에 있어서, 한 회차의 첫 종목이 성공하면
  나머지 31개가 전부 그 간격에 걸렸다 — 로그에 `min interval 2m0s not elapsed`가
  틱마다 31줄. 큐 한 바퀴가 8분이 아니라 **약 7시간**이었다. 같은 파일 바로 위
  `refreshFinnhubUSGeneral` 머리글이 **같은 병을 이미 적어 두었는데** 그때는 시장
  전체 뉴스 쪽만 고쳤다. 이번엔 전용 버킷(`finnhub_news_company`, 2초 = 실제 상한
  60/min의 절반)을 주고 호출부가 스스로 잔다(32 × 2초 = 64초로 8분 주기 안에 끝난다).

  **그리고 그 회차를 select 루프 밖으로 냈다** — 내가 만든 대기가 그대로 다른 소스를
  밀어낼 뻔했다. 뉴스 소스 전부가 select 한 줄을 나눠 쓰는데 여기서 64초를 붙잡으면
  그 사이 naver·rss 틱이 **버려진다**(`staggeredTicker`는 큐가 아니다). 바로 위
  `refreshFinnhubUSGeneral` 머리글이 「기다리게 하지 않는다」를 택한 이유가 그것이고,
  같은 함정에 내가 들어갈 뻔했다. 고루틴으로 내고 겹침만 `usCompanySweeping`으로 막는다
  (겹치면 기다리지 않고 **거른다** — 쌓으면 한도를 두 배로 쓴다).

  배포 후 `min interval` 로그 **0건**, 첫 20분에 해외 finnhub 기사 **158건**
  (그 전에는 하루 종일이 88건이었다).

  **② 별칭을 표로 옮겼다**(`stock_aliases`, migration 070, 씨앗 198행). 사용자 요청은
  「구글 검색 시 알파벳 A·C가 나오게」와 「별도 표로 만들되 토스가 안 주는 별칭도
  관리」였다. **옮긴 이유는 속도가 아니다** — map 조회는 애초에 공짜고, 문제는 **한 줄
  고치려면 배포를 해야 했다**는 것이다. 갈래 둘: `brand`(한글 읽기→영문 표기, 76행 —
  「에스케이」 하나로 SK 계열이 다 잡힌다) · `symbol`(한글 별칭→심볼, 122행 — 「구글」의
  상장명은 「알파벳 A」라 어떤 LIKE로도 안 이어진다). **씨앗의 원문은 Go 소스이고 SQL
  INSERT는 거기서 생성**했다(`stock_alias_seed.go` → 시험 `TestSeedMatchesMigration`이
  두 벌의 동일성을 센다). 씨앗 심볼 111개는 전부 실 카탈로그에 대조해 존재·NORMAL·
  토스 거래가능을 확인했다. 매칭은 여전히 메모리에서 한다(10분 TTL 스냅샷) — 기사
  제목마다 훑는 자리라 SQL을 넣으면 **지금보다 느려진다**. 읽는 자리 셋을 다 이었다:
  앱 검색(브랜드는 LIKE 확장, **심볼은 후보에 직접 넣고 이름 일치와 같은 점수** — 그
  전에는 「구글」 검색 결과가 ACE 구글밸류체인 같은 **ETF뿐**이었다), 뉴스 태깅 인덱스,
  구글 뉴스 검색어 큐(추적 종목의 별칭만). **일상어와 부딪히는 별칭은 뺐고**(「블록」은
  「블록 체인」이 띄어쓰기로 오면 조사 규칙이 못 막는다, 「타깃」은 「타깃 주가」가 더
  흔하다) 그 목록을 시험으로 못박았다. 표로 옮기며 **한 번 빠뜨렸다** — 「마이크론」은
  토스 상장명이 「마이크론 테크놀로지」라 이름 색인으로 닿지 않는데 기존 시험이 잡았다.

  **③ AI 카드를 한 컴포넌트로 합쳤다**(사용자 요청 2). 같은 LLM 호출에서 나온 자료를
  개요 탭은 `overview` 한 문단으로, AI 분석 탭은 네 섹션으로 **다른 카드 모양**에 담고
  있었다 — 요약을 읽고 근거를 보려면 탭을 옮겨야 했고 옮기면 요약이 사라졌다. 이제
  `StockAISummaryCard` 하나가 둘을 다 들고 오른쪽 위 `MiniToggle`(국내/해외를 고를 때
  쓰는 그 모양)이 간단/상세를 정한다. **기본값은 양쪽 다 간단**이고 상태를 저장하지
  않는다(종목을 옮길 때마다 네 섹션이 펼쳐져 있으면 개요 탭의 목적이 사라진다).
  개요 탭만 `hideWithoutLLM` — 규칙 문장이 「AI」 제목 밑에 서면 제목이 거짓말이 되는
  기존 결정(09-03)을 그대로 지킨다. 데스크톱·모바일(375px) 양쪽 실측 확인.

  **④ 번역기 컨테이너를 내렸다 — 그런데 내리기 전에 번역이 0건이었다.** 사용자가
  키를 넣어 뒀는데도 기동 로그가 이틀째 `engines=local`이었고, 원인이 **둘 다 조용한
  실패**였다. ⓐ 키 이름이 `AZURE_TRANSLATOR_API_KEY_FIRST`(Azure 포털의 KEY 1 그대로)
  인데 코드는 `AZURE_TRANSLATOR_KEY`만 읽었다. ⓑ 그리고 설령 이름이 맞았어도 안 됐다 —
  `docker-compose.yml`의 `environment:`에 `"${AZURE_TRANSLATOR_KEY:-}"`가 있어
  **`env_file`을 빈 문자열로 덮었다**(compose에서 environment가 env_file을 이긴다).
  키 없는 엔진은 조용히 사슬에서 빠지므로 **아무 오류도 안 난다.** 셋 다 고쳤다:
  코드가 포털 이름도 읽고, compose에서 그 세 줄을 빼 다른 비밀값과 같은 길로 들어오고
  (FINNHUB_NEWS_API_KEY도 그렇다), **지역을 실측으로 확정**했다(지역 없이 부르면
  401001 — `koreacentral`에서 200). 배포 후 `engines=azure`, 실제 제목이 한국어로
  들어오는 것과 `apibudget` 문자 계수까지 확인하고 나서 컨테이너를 **제거**했다 —
  `stop`만 하면 워치독이 「멈춘 컨테이너」로 보고 5분 안에 되살린다. 13 → 12 컨테이너.
  모델 볼륨(`quant_port_llama_models`)은 되돌리는 길이라 남겼다.

  **⑤ 검증하다 오태깅 하나를 더 잡았다 (요청 밖).** 고친 수집 경로가 실제로 기사를
  넣는지 보다가 「OpenAI delaying IPO **amid** AI safety concerns」에 AMID(ARGENT MID
  CAP ETF)가 붙어 있는 것을 봤다. 09-03에 **v6 재태깅이 바로 이 종목을 잡았다고 적어
  둔 자리**다(「MOVE 30 · AMID 20 · ALTO 18 …」). 그때 고친 것은 티커를 대문자 인덱스로
  옮긴 것인데, **문이 둘이었다** — 토스가 `stocks.name`에도 「AMID」를 주므로 소문자
  이름 인덱스에 `amid`가 그대로 살아 있었다(09-12~13 이틀에 다시 20건). 이름이 티커와
  같고 그 티커가 낱말이면 이름 인덱스에 넣지 않는다. 그 종목을 잃지 않는다 —
  `$AMID`·`NASDAQ: AMID` 같은 명시 표기로는 그대로 붙는다. 저장된 기사에 소급하려고
  재태깅 버전을 v9로 올렸고 **완주를 확인했다**: 국내 `scanned=93,968 changed=172`
  (9분 39초) · 해외 `scanned=46,671 changed=3,789`(4분 34초), 실패 0.
  AMID 태그 **28건 → 0건**. 해외 변경 3,789건이 AMID보다 훨씬 많은 이유는 같은
  스윕이 **새 별칭 122개**도 소급 적용했기 때문이다 — 표를 넣은 효과가 과거 기사에도
  들어갔다(실측 표본: 「Amazon.com vs. Walt Disney」→ AMZN·DIS).

  **주의: 스윕은 완주해야 마커가 남는다.** 이 세션 초반에 재배포를 다섯 번 하는
  동안 매번 잘렸다(마커 없이 부팅 30초 뒤 처음부터 시작한다). 규칙을 바꾸고 재태깅을
  기대한다면 **한 번은 15분쯤 재배포 없이 두어야 한다.**

  **⑥ 내가 낸 배선 구멍 하나를 배포 후 로그에서 잡았다.** 별칭 캐시를 DB에 붙이는
  줄(`SetStockAliasSource`)을 `EnsureSchema` **본체 끝**에 뒀는데, 그 함수는 필요한
  표가 다 있으면 **중간에서 돌아간다**(「mysql: schema ok」). 즉 운영 DB에서는 그 줄이
  한 번도 안 불렸고 — **그런데 아무 오류도 안 났다.** 캐시가 씨앗으로 조용히 돌아가기
  때문이다(이번 세션에 Azure 키에서 만난 것과 같은 모양의 침묵이다). 마이그레이션은
  일찍 돌아가는 지점 **앞**에 있어서 표와 198행은 정상이었고, 달라지는 것은 「표를
  고쳐도 반영되지 않는다」 하나뿐이라 더 안 보였다. `EnsureSchema`를 출구가 하나인
  얇은 껍데기로 만들고 「끝나고 하는 일」을 거기 뒀다. 배포 후 로그로 확인 —
  `stock aliases: 76 brand · 122 symbol`.

- **09-11 (사용자 요청 9건 — 목록 진입 429 폭주 · DART 하루치 소진 · Azure 번역 전환 · UI 5건 · 배포됨 · 미커밋)**

  **① 목록 화면 진입 한 번이 Toss 429를 90건 만들고 있었다 (가장 컸다).** 사용자가 지목한 「toss api rate limit이 매우 크리티컬」의 실체다. 실측: 마감 후 국내 뉴스 화면 **1회 진입** → `/quotes/batch` 한 요청 → `rate-limit exceeded` **90건**(orderbook 50 · trades 40), 응답 1.76초. 그때 **폴러의 터미널 집합은 0**이었다 — 폴러가 아니라 요청 경로다. Toss 클라이언트에 임시 스택 추적을 붙여 확정했다:
  `BatchQuoteSummariesResult → EnsureQuoteDetail → ensureQuoteDetailCached → loadQuoteDetailPersisted → finalizeQuoteDetail → ensurePriceMicrostructure → /trades·/orderbook`.
  **가드는 있었는데 문이 둘이었다.** `ensureQuoteDetailCached`의 `if !listMode(ctx)`가 막는 것은 `refreshTerminalSnapshot` **한 분기**뿐이고, 캐시·DB에서 찾은 **흔한 경우**는 그 가드를 지나 `finalizeQuoteDetail`로 가 종목마다 2호출을 냈다. 같은 사고를 `/stocks` 목록에서 이미 한 번 고쳤는데(그 주석의 실측: limit 5/10/20/40 → 0.18/1.9/6.4/**58초**) **그때 고친 것은 호출 지점 하나**였다. 그래서 이번엔 가드를 호출 지점이 아니라 **`ensurePriceMicrostructure` 안에** 뒀다 — 호출 지점은 셋이고 앞으로 늘어나며, 지점마다 복사하면 빠진 하나가 화면이 아니라 상류 한도에서만 드러난다. **실측 결과 90건 → 0건, 1.76초 → 106ms(16배).** 칩 시세는 그대로 나온다(퍼센트 칩 100개 확인).

  **② DART 하루치 18,000을 「원장에 없는 잡」이 다 썼다.** 09-10 19:29에 한도가 이미 비어 그날 DART 잡 셋(공시·내부자·재무)이 **한 종목도 못 부르고** 전부 실패했다. 원장에 범인이 없던 이유는 **원장에 안 남는 적재였기 때문**이다 — api의 reference data poller가 `RefreshAllStored`로 `stock_fundamentals` **전 종목**을 한 번에 돌았다(로그 「refreshed fundamentals for 12037 symbols」, 03:56~11:04 7시간). 국내 2,800종목 × 실측 6.4호출 ≈ 18,000. 나빴던 것은 호출 수가 아니라 **원장 기록·예산 예약·고정 시각 셋이 동시에 없었다**는 점이고, 특히 주기가 **api 기동 시각**을 따라가 그날은 저녁 DART 잡 바로 앞에 떨어졌다. **새 장치를 만들지 않았다** — `SyncStateRepo`의 머리글이 「SelectTargets의 ORDER BY가 등급제의 전부다」라고 적어 두었고 종목 단위 적재 셋이 모두 그걸 쓰는데 **지표 갱신만 빠져 있었다.** 이제 같은 줄에 선다: `kind='fundamentals'` 로테이션 · 회차 상한(국내 400·해외 600) · **DART 예약 9,000**(근거는 정상일 실측 7,895 = 내부자 1,500 + 재무 400×16 + 공시) · 성공/실패 모두 `Mark` · **배선 없으면 아무것도 안 함**(폴백이 곧 사고 당일 동작). 국내 400×7 = 2,800으로 한도의 16%, 7일에 한 바퀴 — 지표는 분기에 한 번 바뀌고 로테이션이 관심·보유를 앞에 세운다.

  **③ warnings 잡 Toss 호출의 19%가 「내일도 같은 답이 올 질문」이었다.** `symbol_sync_state` 실측: `kind='warnings'` 17,918행 중 **3,388행(19%)이 `404 stock-not-found`**인데 전부 `error`로 기록돼 `SelectTargets`의 30일 not_found 쿨다운이 걸리지 않았다 — 상대에게 없는 종목을 **매일** 다시 물었다. `toss.IsStockNotFound`로 갈라 `not_found`로 기록한다(429·타임아웃은 그대로 `error` — 그쪽은 다시 물어야 나아진다). 한 바퀴 뒤 자동으로 빠진다. 함께 `warningsPause` 250ms→**340ms**: 「한도 아래」는 맞았지만 한도(5/s)의 80%에 계속 붙어 `remaining=1` 로그로 가득했고, 429는 이 잡만의 문제가 아니다 — **failover가 프로세스마다 따로** 판단하므로 워커가 태운 보조 키를 api도 같이 쓴다. 404 3,388개가 빠져 대상이 14,500종목이 되므로 **82분(전 75분)**으로 느려지지 않는다.

  **④ 번역을 Azure AI Translator로 옮겼다.** 자체 모델(llama.cpp 1.7B)은 `legacy-translator` 프로파일로 내려 기본 기동에서 빠졌다(되돌리기는 `TRANSLATE_PROVIDER=local` + 그 프로파일). 한 요청에 제목 100건, 문자는 **보내기 전에** 센다(상대 과금이 보낸 원문 기준이고 응답에 사용량이 없다). `apibudget`에 `TokensPerMonth`(월 창)를 새로 넣어 F0 무료 2,000,000자를 강제한다 — 넘긴 제목은 영어로 남는다(폴백 없음, 기동 로그가 찍는다). 테스트 9건. **⚠️ `AZURE_TRANSLATOR_KEY`가 아직 비어 있어 지금은 번역이 0건이다** — 키를 넣어야 동작한다. 그리고 **처음 보고한 월 3.3M자는 과대였다**: 그 수에는 한국어 소스 기사(연합·토큰포스트 등, `containsHangulText`로 애초에 안 보낸다)가 섞여 있었다. 영어 원문만 다시 세면 평일 88K·주말 29K = **월 약 2.17M자로 한도를 8%만 넘는다.** 수집량 축소 결정의 크기가 달라지므로 레버는 사용자에게 다시 물었다.

  **⑤ UI 5건.** (1) 외부 카드·모달·검색·메모·스탬프에서 종목으로 들어갈 때 기본 탭 `ai`→`overview`(11곳) — AI 분석 본문은 일부 종목에만 있어 밖에서 들어온 사람이 빈 화면을 먼저 만났다. AI 분석 탭에서는 AI 카드가 시장 분석 위로 올라갔다. (3) 일별 시세·종합 점수 카드에 집계 시각 각주 — 점수는 종가와 같은 시각에 생기지 않는다(종가는 마감+20분, 점수는 `stock_scores` 00:25). 최신 행이 실제로 비었을 때만 브랜드색으로 올린다. (4) 모바일에서 카드 하단 종목 칩이 2줄이 되며 레이아웃이 깨지던 것 — 실측 375px 83행 중 **15행이 두 줄, 14행이 잘림**(`max-h-24` 96px에 내용 101px). `flex-nowrap` 하나로는 안 된다(flex가 칩을 눌러 「코…」가 되는데 그건 이미 한 번 고친 버그다) → 홈 메타 줄이 쓰는 **같은 측정 계산**(`measureMetaChipLayout`)을 쓰는 `NewsChipRow`로 한 줄에 들어가는 개수만 그리고 나머지는 「+N」. 실측 152행 **wrapped 0 · clipped 0 · 뭉개진 칩 0**. (8) 속보 필터가 눌려 보이지 않던 것 — 상태는 옳았고(`aria-pressed=true`, `bg-[var(--brand)]` 붙음) **커서를 치우면 파랗고 올려 두면 회색**이었다: `buttonVariants` outline의 `hover:bg-accent`가 같은 명시도에 더 나중이라 브랜드 칠을 덮는데 **누른 직후가 바로 그 상태**고 터치에서는 탭 뒤에도 남는다. 켜진 쪽 hover를 같은 색으로 못 박고 체크 표시를 더했다. (9) 국내/해외를 한 카드에서 고르는 4곳의 기본값을 **열린 시장**으로 — 판정은 서버가 한다(`/market/clocks` 신설, 달력은 이미 캐시라 상류 호출 0). 둘 다 열렸거나 둘 다 닫히면 국내, 주요 실적 일정은 해외 고정. 정해지기 전에는 `null`이라 깜박임·버린 요청이 없다. 테스트 8건.

  **⑥ 되돌아본 것.** 「일별 시세 요약글을 4개 섹션 합본으로」는 앞으로 쌓이는 것만(사용자 결정) — 과거 행은 그날의 LLM 원문이 없어 소급 불가. `isEmptySectionText`로 「내부자 거래 내역이 없습니다」류를 걸러 없는 갈래는 아예 빠진다. 테스트 4건.

  **⑦ 구멍 메우기가 매 회차 3,250건을 헛돌았다 (같은 세션 후속 — 사용자가 「남은 상위 행부터 계속 수요 줄이기」 선택).** 해외 `daily_close_fetch_stale`이 회차마다 3,250여 종목을 실패하고 **다음 회차에 같은 종목을 또 시도**했다(하루 두 번 = 6,500여 Toss 호출). 국내 같은 잡은 4,412 시도에 실패 138(97% 보유)인데 해외만 75%에 붙어 있던 이유는 **우리 카탈로그 NORMAL 종목이 Toss 취급 집합의 상위집합**이라서다(표본: 워런트·우선주·라이츠가 `stock-not-found`). 종가는 크리티컬 데이터라 **조건 셋을 모두** 만족해야 30일 빠진다 — ① 코드가 `stock-not-found`일 때만(429·타임아웃·5xx 절대 아님, Toss 장애는 종목별 404로 오지 않는다) ② **한 번도 성공한 적 없는 종목만**(`last_success_at IS NULL` — 예전에 종가를 준 종목이 지금 404면 `error`로 남겨 계속 묻는다) ③ 30일 쿨다운(영구 아님). 임시 행 3개로 대상 쿼리를 직접 확인하고 정리했다: not_found 30일 내 제외 · error 유지 · not_found 31일 전 유지. **커버리지 분모도 같이 고쳤다** — 예전엔 마스터 활성 종목 전부라 해외가 구조적으로 75%였고 「90% 미만」 경고가 고쳐질 수 없는 조건으로 매 회차 울렸다(울리는 경고는 안 보는 경고가 된다). 원장 메시지도 「실패」와 「미취급」을 갈라 적는다.

  **번역 물량 재측정 — 첫 보고가 틀렸다.** 월 3.3M자로 보고했는데 그 수에 **한국어 소스 기사**가 섞여 있었다(`headline_ko`는 채워지지만 `containsHangulText`로 애초에 번역기에 안 보낸다). 영어 원문만 세면 평일 88K·주말 29K = **월 약 2.17M자**로 한도 2M에서 **8%만 초과**다. 사용자가 그 수 위에서 「수집량 축소」를 결정했으므로 다시 물었고, **수집량은 그대로 두고 월 예산 가드만** 쓰기로 정해졌다(넘긴 제목은 영어로 남는다).

  **⑨ (같은 세션 후속 2 — 사용자 「#7 전체 진행 · 로그·DB·코드 전체 파악 · 사건 기록 테이블」)**

  **사건 원장을 만들었다**(`ops_incidents`, migration 068). 표가 둘 있었는데 둘 다 「아직 깨져 있나」에 답하지 못했다 — `batch_job_runs`는 **실행된 것만** 알고(안 돌아간 슬롯은 흔적이 없다), `batch_health`는 **단 한 행**이라 이력이 없다. 09-10 DART 소진을 조사할 때 「언제부터」를 컨테이너 로그에서 찾아야 했고 그 로그는 재시작으로 사라져 있었다. **사건은 줄이 아니라 상태로** 저장한다 — (kind, subject)당 열린 사건 하나를 **DB 유니크 인덱스가** 보장하고(워치독은 프로세스가 여럿이라 애플리케이션 판정으로는 안 된다), 다시 보이면 occurrences만 오르고 **로그는 처음 열릴 때만** 찍는다(그전에는 같은 줄을 15분마다 찍어 financials_domestic 하나가 18번 넘게 나왔다). 풀리면 닫고 재발하면 **새 행**이라 재발이 행 수로 세어진다. severity는 올라가기만 한다. 넣는 쪽은 워치독(batch_missed·failed·stuck)과 **apibudget 소진 훅**(quota_exhausted — 계층을 지키려고 훅으로 뺐고, 5분 스로틀 + 고루틴으로 요청 경로를 막지 않는다) 둘이다. 읽기는 `GET /api/v1/admin/ops/incidents`. 수명주기는 실제 DB에 돌려 확인했다(열림 → occurrences 4 → critical 고정 → 닫힘 → **재발은 새 행** → 이력 2행), 시험 행은 정리했다.

  **예약 잡이 하루 두 번 돌고 있었다.** 원장 실측 — `data_retention` 04:29:56 + 04:30:00, `candle_rollup`, **`disclosures_daily`(DART 호출이 두 배)**. 느린 잡만 한 번인 것은 고쳐져 있었던 게 아니라 **claim 락이 두 번째를 막았을 뿐**이다(락은 동시 실행을 막고 반복은 막지 않는다). 원인은 `time.After(time.Until(슬롯))` 루프가 **이르게 깨어난 것을 확인하지 않아** 같은 슬롯을 다시 계산한 것이고, 이르게 깨는 이유는 벽시계가 뒤로 밀리기 때문이다(단조 시계로 자고 벽시계로 슬롯을 센다). 매일 더 일찍 깨어난 것(−0.4초 → −2.5초 → −4.0초)이 그 증거다. `WaitForSlot`(이르게 깨면 남은 만큼 더 잔다) + `SlotGate`(같은 슬롯 두 번 금지)를 만들어 **슬롯 루프 일곱 개 전부**에 적용했다. 원장의 Begin은 그대로 무조건 INSERT다 — 원장은 실행된 것을 있는 그대로 적어야 한다.

  **Toss 「price API 위주」 설계를 검토했다**(be/docs/api-quotas.md에 표와 함께). 방향은 맞지만 **전제 하나가 어긋난다: 한도는 전역이 아니라 그룹별이다.** `/prices`·`/orderbook`·`/trades`·`/price-limits`가 **`MARKET_DATA` 한 버킷(실측 limit=15)을 공유**하므로 price API를 많이 쓰는 것은 다른 API를 아끼는 일이 아니라 같은 창에서 다투는 일이다. 09-11의 429 90건 사건이 정확히 그 모양이었고, 고친 것은 「price API를 더 쓰자」가 아니라 **그 버킷에서 쓸데없는 호출을 뺀 것**이었다. 그래서 목표를 「price API만 쓰자」에서 **「버킷마다 꼭 해야 하는 일만 남기자」**로 다시 썼다. `/candles`는 대체 불가로 확인 — `PriceResponse`의 필드가 symbol·timestamp·lastPrice·currency **넷뿐**이라 OHLC도 거래량도 없다(종가 표는 둘을 저장한다). 줄일 수 있는 것은 호출당 종목 수가 아니라 **대상 종목 수**였고 그건 ⑦에서 했다. **가장 큰 남은 기회는 `/stocks/all`** — 「토스에서 거래 가능한 종목만」을 마켓 단위로 주고(하루 7호출), 그 버킷(`STOCK_ALL`)은 우리가 **아무것도 안 쓰는** 버킷이다. 지금은 그 사실을 종목마다 404 한 번을 치르고 배운다(warnings 3,388 · 종가 3,250). 구현 전 사용자 결정 하나가 필요하다 — 그 집합에서 빠진 종목을 **화면에서도 숨길지, 수집에서만 뺄지**.

  **비-price Toss 호출을 줄였다.** 환율 폴러 15초 → **1분**(장중) · 10분(양쪽 마감): 하루 5,760 → 약 1,440호출. 15초가 사는 것을 실측했는데 분당 변동이 1342.85 → 1343.00 → 1342.92, 즉 **0.011%**로 어느 화면에서도 안 보인다. 순위 폴러는 **마감된 시장을 건너뛴다**(마감 +30분 그레이스 — 경계에서 최종 순위를 놓치지 않게): 2,304 → 약 900호출. 화면은 이미 마감을 알고 있었고(`sessionClosedAt`) 서버만 계속 받고 있었다. 배포 후 실측 확인 — 환율 4/분 → **1/분**, 순위 0.8/분 → **0.5/분**(KRX 스킵).

  **로그·DB·코드 전수 점검에서 추가로 나온 것 넷.** ⓐ **해외 일반 뉴스 카테고리 셋이 죽어 있었다** — general·merger·forex·crypto를 한 회차에 차례로 부르는데 티커 주기(2분)와 `finnhub_news` 최소 간격(2분)이 같아 **첫 갈래만 성공**했다. merger·forex·crypto는 한 번도 수집되지 않았고 남은 건 회차마다 같은 로그 셋뿐이었다(이미지의 「Finnhub log spam」의 실체). 회차마다 한 갈래씩 도는 로테이션으로 바꿨다 — **호출 수는 그대로**(예전에도 성공은 1회였다), 8분에 한 바퀴. ⓑ **FRED 계기가 12분의 1을 말하고 있었다** — `QuoteBatch`가 Check·Record를 한 번씩 하면서 시리즈마다 HTTP 2회를 냈다(지수 6종 = 실제 12회). `RecordN`을 만들어 실제 호출 수를 기록하고, 부분 실패를 로그로 남긴다(예전에는 전부 실패했을 때만 오류라 지수 패널의 구멍을 설명할 수 없었다). ⓒ **관리자 콘솔에서 제공자 둘이 안 보였다** — 목록이 `AllUsage` 안에만 있어 `finnhub_backfill`·`azure_translate`가 빠져 있었다. 한도를 지키는 장치가 화면에 안 보이면 관리되지 않는 것과 같으므로 `AllProviders()`로 뽑았다. ⓓ **소스 파일 넷이 망가져 있었다**(HEAD에 커밋된 상태) — 모든 줄 뒤에 빈 줄이 하나씩(fred.go 543줄 중 250줄이 빈 줄), 게다가 저장소 관례와 달리 LF. 비어 있지 않은 줄이 완전히 동일함을 증명하고 정리했다(fred.go · backfill-daily-closes · import/resolver · import/textparse).

  **React #418**은 원인을 특정했다 — `formatRelativeTime`이 `Date.now()`를 쓰므로 SSR 때 「3분 전」, hydration 때 「4분 전」이 되어 텍스트가 어긋난다. HTML은 캐시되지 않아(`no-store`) 창이 밀리초라 빈도는 낮지만(약 0.3%) 실재한다. React가 이 경우를 위해 둔 `suppressHydrationWarning`을 상대시각 텍스트를 **직접 담은** 요소 여섯 곳에 붙였다(모달은 SSR을 타지 않아 제외).

  **⑩ (같은 세션 후속 3 — 사용자 요청: 토스 유니버스로 화면에서 숨기기 · 종목 별칭 · 사용량 재점검)**

  **`/stocks/all` 유니버스를 도입했다**(`toss_universe` 잡, 매일 01:00, **하루 7호출**, 버킷 `STOCK_ALL`은 우리가 안 쓰던 것). **첫 실행 실측(45초)**: 유니버스 15,314종목 · **거래가능 14,520 · 숨김 3,398 · 한국어 이름 4,488건 갱신**. 숨은 3,398은 앞서 따로 잰 경고 스윕 404 건수(3,388)와 **거의 같다** — 방법이 다른 두 측정이 같은 수를 가리켰다. 표본도 예상대로였다(워런트 `ILLRW`·`NTRBW` · SPAC 유닛 `ARTCU` · SPAC 껍데기 `KOYN` · 우선주 예탁증서 `TFC^R`·`DX^C` · 라이츠 `RFAMR` · OTC 해외). 검색 실측: `NTRBW`·`ILLRW` → **0건**, `TFC` → 6건에 `TFC^R` 없음, `GOOGL`·`AAPL` 정상.
  **그리고 도입 당일 그 사고를 내가 냈다.** 01:00 예약 실행 중에 내 배포로 워커가 재시작됐고 **17,918종목이 전부 숨어 검색이 통째로 비었다**(AAPL도 0건). 원인 둘이 겹쳤다 — ⓐ 두 문장(「받은 것을 1로」·「나머지를 0으로」)이 **트랜잭션 밖**이라 사이에서 죽으면 두 번째만 남는다 ⓑ **시각 비교가 정밀도에 기대고 있었다**: `toss_checked_at`은 `TIMESTAMP` 정밀도 0(초)인데 Go는 나노초를 넘긴다. 실험으로 확정했다 — `CAST(@t AS DATETIME(0)) <> @t` 가 **참**이라 방금 1로 만든 행이 「안 온 행」 조건에 그대로 걸린다. 고친 것 셋: 한 트랜잭션 · `Truncate(time.Second)` · **커밋 직전 결과 검사**(반영 후 보이는 종목이 문턱 아래면 롤백). 세 번째가 핵심이고 실제로 작동했다 — 고치기 전 재실행이 「반영 후 보이는 종목이 0건뿐이다 — 롤백한다」로 거부해 데이터를 지켰다. 복구 후 14,520/3,398로 정상, 실 DB 대상 회귀 시험도 추가했다(유령 심볼 1,500개로 전 종목 숨김을 시도 → 거부 + 원상 유지 확인).

  **안전선 넷을 시험으로 잠갔다**: ① 일곱 마켓이 다 와야 반영 ② 합집합 1,000건 미만이면 거부(입력) ③ **반영 후** 보이는 종목 1,000건 미만이면 롤백(결과 — 위 사고가 통과한 자리) ④ **NULL은 숨기지 않는다**(배치 전/신규 종목이 숨으면 화면이 통째로 빈다). **우리 `market` 컬럼으로 마켓 단위 반영을 하지 않는다** — 우리는 ARCX 2,655·BATS 1,544·OOTC 797·KRX 2,323을 따로 쓰는데 토스는 그것을 `US_ETC`·`KR_ETC`로 답한다. 매핑이 틀리는 날 5,000종목이 숨으므로 합집합만 쓰고 우리 분류는 보지 않는다. **숨기는 것은 발견(검색)이고 가진 것은 아니다** — `/stocks?symbols=`(보유·관심)는 필터를 안 건다.

  **별칭 데이터의 출처는 토스였다.** 손으로 적을 것이 아니라 **이미 가진 것**이었다 — `/stocks/all`의 `name`이 한국어 종목명이고 주식 종류까지 들어 있다(GOOGL=알파벳 A · GOOG=알파벳 C · BRK.A=버크셔 해서웨이 A). 예전에는 `usNewsNameAliases` 13개를 손으로 유지했고 그 목록에 `구글`은 있는데 **`알파벳 A`·`알파벳 C`는 없었다.** 두 자리에서 쓴다: **구글 뉴스 검색어**(종목 이름으로 검색하고 결과에 심볼을 그 자리에서 붙인다 — 국내 경로와 같은 구조라 본문 매칭 오탐이 없다)와 **태깅 인덱스**(한국어 이름도 색인 — 위험은 국내 13,000종목이 이미 쓰는 `nameTokenMatch`의 조사 규칙이 막는다: 「애플리케이션」·「메타버스」 차단. 이름 자체가 일상어인 것만 뺀다 — `ambiguousUSLocalNames`의 「비자」). 실측: 앱 검색 `알파벳` → GOOGL(알파벳 A)·GOOG(알파벳 C), `버크셔` → BRK.A/B.

  **내가 만든 회귀를 잡았다.** 앞 회차에 넣은 세션 판정 두 곳(환율 주기·순위 마감 스킵)이 매 틱 `EnsureCalendars`를 불렀고, `marketCalendarsDateStale`이 **휴장일에는 영원히 참**이라(토요일 KR 달력은 오늘 날짜를 안 담는다) 분당 7회의 달력 호출이 나갔다 — **하루 약 10,000회로, 같은 회차에 없앤 환율 5,760회보다 컸다.** 두 겹으로 고쳤다: 게이트에 재시도 간격(`calAttemptedAt` + 10분)을 두고, 주기적으로 도는 자리는 상류를 아예 안 건드리는 `MarketClockCachedFor`를 쓴다. 배포 후 실측 **5분에 1회씩**(KR·US 각 1).

  **`tickPrice`에 세션 필터가 없었다** — `tickFast`·`tickSlow`는 각각 실측 후 붙였는데 이 틱만 빠져 있었다. 붙였고, 재발 방지로 **세 틱 모두** 필터를 부르는지 시험이 센다. **다만 이 건의 실측 해석은 내가 틀렸다**: `/prices` 분당 29회를 「양 시장 마감 중 낭비」로 적었는데, KST 토요일 00:40은 **ET 금요일 11:40**이라 미국 정규장이 열려 있었다(서버 판정 `us: regular`). 그 호출은 정상이었다. 누락은 사실이므로 고친 것은 맞고, 효과는 양 시장이 정말 닫히는 창(평일 KST 06:00~08:00·주말 05:00 이후)에서 나타난다 — 아직 그 창에서 재보지 않았다.

  **미완(#7의 나머지).** 사용자 첨부 표의 13행 중 이번에 닫은 것은 「client 429」·「404 quota 낭비」·「filings rate-limit」·「US 종가 <90%」 넷이다. 사용자 첨부 표 13행 중 **열하나가 닫혔다.** 남은 둘은 한 덩어리다: **프로세스 간 공용 리미터**(Toss를 `apibudget`에 편입 + Redis 크로스 프로세스로 `X-RateLimit` 헤더를 실제로 지키기)와 그것에 딸린 **프로세스별 failover**. 사용자가 「수요 줄이기 먼저」로 순서를 정했고 수요 쪽이 끝났으므로 다음 차례다. **검토만으로 닫힌 것 둘**: `terminal-snapshot` 1초 설계는 이미 막혀 있다(FE가 마감 중 폴링·SSE를 걷고 `nextSessionAt`에 깨며, 호가·체결은 웹소켓 400슬롯이 주 경로다), `earnings skip 13`은 의도대로다(Finnhub 실적 캘린더가 우리 카탈로그의 상위집합 — 실측 5,541행 중 10행 = 0.18%, 건수로 로그에 남는다). **구현 전 결정이 필요한 것 하나**: `/stocks/all` 유니버스 도입(위 ⑨).

### TODO (이관 시점 원문 — 열린 항목은 AGENT_STATE BACKLOG로)

> **출시 전 점검서: `docs/PRE_LAUNCH_OPTIMIZATION.md` (08-19)** — 실측 기반 P0/P1/P2. P0 6건(레이트 리밋 없음 · 요청 타임아웃 없음 · Redis `noeviction` · 8프로세스×50 커넥션 · 마이그레이션 8중 동시 실행 · FE 배포 형태 없음)이 배포 차단 항목이다.

- [x] `candle_bars.bar_time` 시간대 정규화 — 08-08 완료 (쓰기 경로 + 149,803행 보정)
- [x] 토스 계좌 연동 제거 — 08-20. 시세 Toss는 유지. 연동 LLM·동기화·FE 메뉴 삭제 (`054_drop_broker_linking.sql`)
- [ ] 점수: 섹터 컬럼 추가(DART `induty_code` / SEC `sic` — 내부자 조회 응답에 이미 포함) → 피어그룹 교체
- [ ] 점수: 예측력 검증(score(D) vs return(D+1) IC) — 40거래일 축적 후
- [x] 일별 종가 전 종목 백필 — 08-08 완료. 커버리지 1,469 → **16,225/17,901** (436,199행). 잔여 1,676종목은 재실행으로 해결 불가(미국 우선주 `^`/`.` 티커가 Toss 캔들 API `400 invalid-request`, 소수는 상장폐지 `404`)
- [ ] btc·gold 지수 이력 백필 — 날짜 있는 소스 없음(Upbit/goldprice 이력 API 확인 필요). 오늘부터는 일별 배치로 누적
- [ ] usd-krw 지수 이력 — Toss 지표캔들이 `USD/KRW` 심볼에 `400` 반환, 다른 심볼/소스 확인 필요
- [ ] Redis Pub/Sub + 다중 BE SSE fan-out
- [x] Toss 429 `Retry-After` 파싱 — 08-06
- [ ] FE: `source=db` / 장 마감 배지
- [ ] prod migration (goose)
- [~] 추적 심볼 캐싱 — 08-17에 **호출자 쪽**(`NewsStore.trackedSymbols`, 1분 TTL). 08-20부터 관심+수동보유만 (브로커 포트폴리오 경로 제거)
- [ ] 폴러 리더 선출/샤딩 — 다중 BE 인스턴스 대비
- [x] 계좌/잔고 조회 사용자수 비례 한계 — 08-20 토스 계좌 연동 제거로 해당 경로 없음

### Last (07-25 ~ 08-20)
- 08-20: **토스 계좌 연동 제거** — 시세 Toss 유지. 브로커 포트폴리오·인사이트 LLM·주문 API 삭제. 추적 심볼=관심+수동보유. `054` DROP. `extendedMigrations`에서 `007` 재적용 제거(없으면 매 기동 CREATE→DROP). FE tsc PASS, `go test ./...` PASS, Docker healthy, Tesseract OK
- 08-19: **국내 실적 C안 구현** — 시즌 10/12/14/16/18/20시 당월, 20시만 전월. 비시즌 18:30. 기동 catch-up(국내 동기·해외 비동기). 배포 후 8/14 6→88행, 달력 87 confirmed. 17–19는 소스 0건
- 08-19: **LLM 지금 차단 여부** — 현재(22:38 KST) Google 차단 아님(`blocked_until` 없음, 마지막 성공 22:30 브리핑). 아침 07:46만 PDT 전날 500으로 429. 16:00 KST 리셋 후 US 92 성공. 우리 KST 오늘 174/500. 대시보드 분당 바는 호출 없는 분이면 0
- 08-19: **LLM 500 내역** — 지수 LLM 0. 랭킹 KR+US ≈200. 뉴스 LLM은 기사 단위가 아니라 시장 브리핑 2시장. 실제 초과분은 해외 제목 번역(Papago 20/min 초과 → Gemini). 8/18 KST openai 500인데 US 랭킹 배치 0·KR 기록 없음 → 번역+브리핑만으로 500. PDT/KST 일자 불일치로 아침 KR이 전날 쿼터에 부딪힘. 미수정
- 08-19: **LLM 요약 공백 조사** — 화면이 비는 게 아니라 Gemini 무료 500 RPD 소진. KR 배치 1/106 llm(005930), SK하이닉스 포함 나머지는 `source:rule`. 우리 카운터(KST)와 Google 카운터(PDT)가 어긋나 국내 배치가 한도 밖에서 출발. US 92/96은 리셋 이후. 번역은 2분마다 24건을 간격 없이 밀어 4s 버킷을 태움. 미수정
- 08-19: **국내 실적 달력 14일 이후 공백 조사** — 마이그레이션(8/17 수동 2,791행) 이후 스케줄은 8/18 18:30 한 번(358행). 오늘 워커가 19:24에 재시작돼 18:30을 건너뜀 — `daily_close`는 catch-up이 있고 `worker-earnings`는 없다. FnGuide 라이브 8월은 14일이 마지막(89건, 거의 개별)인데 DB는 6건. 17·18일은 소스에도 없음. 제안: 기동 catch-up + 시즌 중 당월 재폴링(별도 결과 API는 없음, `_E`/`_P`가 같은 월 응답). 미구현
- 08-19: **펀더멘털 배치 vs 실적/재무 탭** — `stock_fundamentals` 9,993행(EDGAR 7,121). 미국 fetched_at은 **07-20 04:32가 마지막**(시드 ~4h46m). 24h 폴러 로그 없음. 실적 분기표·재무제표 차트는 이 표가 아니라 탭 열 때 companyfacts(Redis 24h)
- 08-19: **EDGAR 사용처 전수** — URL 4종(`company_tickers`·`companyfacts`·`submissions`·Form4 XML). 재무/실적/이력은 같은 companyfacts를 각자 호출. 내부자만 submissions+XML. 8/min은 SEC(10/s)가 아니라 자체 예산
- 08-19: **EDGAR 분당 8회는 우리 한도** — SEC 공식은 **초당 10회**(fair access). `apibudget` 8/min이 Form 4 XML 상한 3의 원인. 내부자 공백(CRWV)은 SEC 한도가 아니라 자체 예산+Table I만 파싱
- 08-19: **CRWV 내부자 탭 공백 원인 조사** — SEC는 Form 4 **431건**(최근 1000건 중) 제공. 우리는 조회 시 EDGAR를 **온디맨드**로 부르며 MySQL 적재/폴링 없음. 최근 3건만 파싱+보통주 Table I만 읽어, 최신 3건이 Class B 증여·워런트 매도(Table II)라 0건 → Redis `insider:tab:us:v1:CRWV=[]` 24h 캐시. 바로 다음 신고(8/14 Magnetar)에 Class A 매도 24건이 있음
- 08-19: **AI 추천 랭킹 실시간 시세 모달** — 「실시간 시세」가 `/market`으로 화면을 떠나 랭킹 맥락이 끊기던 것을 일봉 캔들 모달로 바꿈. `/quotes` 개요 차트를 `QuoteDailyChart`로 추출해 모달과 공유. 헤더(실시간가)·일봉·시세 그리드, 열 때만 `useQuoteDetail`/`useQuoteCandles`. 분봉·호가는 푸터 「실시간 차트에서 더 보기」. tsc PASS, eslint 신규 0건. FE only
- 08-18: **일별 종가 2026-01-01~ + 실적 반응 백필** — 국내 `-fetch-history` **14,213/14,217** 성공. 실적 반응 **2월·5월 100%**, 8월 **96%+**. **해외**: `backfill-daily-closes -market=us` + fetch 함수 asset_type 필터 추가, 배치 `daily_close_fetch_history`(market=us) 추가 — **실행 대기**(MySQL80 중지로 CLI 중단, 재시작 후 아래 명령)
- 08-18: **실적 화면 — 정렬 테이블 · 성과형 달력 · 레일 전면 교체** (목업 `docs/mockups/earnings-table-v2.html` → 사용자 선택: 표 확정 · C2 · R1·R2·R3 + R5 변형 · 현재가 열 포함)
  - **카드형 행 → 표.** 예전에는 한 행이 2줄짜리 카드라 세로로 훑는 것밖에 못 했고, 「영업이익 큰 순」·「시가총액 순」으로 줄을 세울 수 없었다. 실적 화면에서 사람이 실제로 하는 일이 그 줄 세우기다. 9열 전부 정렬 가능하고, **칩을 없애고 글자로** 적는다(연결/개별·장전/장후는 배지 대신 작은 글자)
  - **열 구성은 시장이 정한다** — 국내 영업이익 / 해외 EPS + **시간(장전·장후) 열**. investing.com에 있지만 우리가 못 채우는 **시간외거래·EPS 개정**은 빈 열로 두지 않고 **뺐다**
  - **신설 데이터 3종**: ① **시가총액** = `stocks.shares_outstanding` × 최근 종가 — 커버리지 **100%**, 저장하지 않고 읽을 때 곱한다(매일 바뀌는 파생값을 하나 더 만들지 않으려고). 심볼별 MAX(trade_date) 조인 한 방 ② **매출 서프라이즈**(국내 93%·해외 95%) — 이익이 기대를 넘었는데 매출이 밑돌면 비용 절감이지 성장이 아니라는 뜻이라 별개 열로 둔다 ③ **현재가** — 보이는 행 심볼을 모아 `quotes/batch` **1회**(상한 120), 폴링 없음
  - **C2 성과형 달력** — 칸 색이 **그날 상회 비율**을 말한다(50% 위 빨강 / 아래 파랑, 50%에서 멀수록 진하게). 예전 중립 회색 농도는 「많은 날」과 「좋았던 날」을 구별하지 못했다. **발표 전인 날은 색을 칠하지 않는다**(없는 성과를 말하는 셈) — 중립 농도로 건수만. 실측: 64%→빨강 α0.18 · 46%→파랑 α0.11 · 17%→파랑 α0.31
  - **레일 전면 교체** — 「상회/하회 TOP」은 **서프라이즈 탭이 이미 더 길게** 보여주는 목록이라 6줄을 반복하는 셈이었다. 새 넷은 전부 **표에 없는 것**: ① **다가오는 대형 발표**(시가총액 순) ② **기대와 주가가 엇갈린 발표** — 「증권사 예측 → 실제」를 글자로 밝히고 그 아래 주가를 둔다(퍼센트 둘만 나란히 두면 어느 쪽이 실적이고 어느 쪽이 주가인지 알 수 없다) ③ **꾸준히 기대를 넘는 기업**(최근 8분기 적중률, 표본 4분기 이상) ④ **내 종목 최근 발표**(지난 2주~앞으로 한 달 고정 창)
  - `/earnings/highlights` **한 요청**으로 셋을 준다 — 카드마다 부르면 레일 하나에 세 왕복인데 셋이 서버에서 같은 표를 읽는다. Redis 30분
  - **심볼 필터가 있으면 구간 상한을 31→120일로 푼다.** 31일 상한은 스캔을 묶기 위한 것인데 심볼 목록(≤100)이 이미 그 일을 하고 있어 같은 제약을 두 번 걸 이유가 없다 — 레일의 44일 창이 필요해져 드러났다
  - **국내 R1이 빈 카드였다** (실측): 국내 실적이 2·5·8·11월에 몰려 08-18 기준 향후 2주가 0건이다. 2주가 비면 **100일로 넓혀** 「다음에 올 큰 발표」를 보여준다 — 75일로 잡았다가 다음 추정 발표(11-02)가 76일 뒤라 하루 차이로 놓쳤다. 지금은 삼성전자 1,604조·SK하이닉스 1,217조가 뜬다
  - **캐시 버전을 올려야 한다** — 계산이 바뀌었는데 `earnings:highlights:v1` 키가 그대로라 5ms 캐시 히트로 옛 빈 결과가 계속 나왔다. v2로 올려 해결
  - 검증: go build/vet/test·tsc·eslint(신규·수정 파일 0건)·`next build` green, Docker 재기동. **브라우저 DOM 실측** — 9열 헤더·현대해상 「5050억원/3690억원 +36.9% · 시총 3.3조원 · 44,650 −1.43%」, 시가총액 정렬 클릭 시 3.3→1.5조 내림차순, 달력 색 6칸, 레일 4카드(옛 TOP 사라짐). ⚠️ 달력 색을 처음 읽었을 때 전부 회색으로 나왔는데 **`transition-colors` 진행 중 샘플링**이었다 — `transition:none` 후 재측정해 확인. 스크린샷은 여전히 패널 미표시로 불가
- 08-17: **실적 메뉴 국내·해외 분리 + 위젯 4종** (목업 `docs/mockups/earnings-widgets-v1.html` → 사용자 선택: W1~W4 전부 · 월/주/일 토글 · 연결 우선 + 개별 접기 · 사이드바 2항목)
  - **사이드바 「실적」 = 국내 실적 일정 / 해외 실적 일정**(뉴스 그룹과 같은 결), 화면 안 시장 토글 제거. 두 시장은 한 목록에 못 들어간다 — 국내는 예정일 소스가 없어 앞일이 전부 추정이고 지표가 **영업이익**(연결/개별), 해외는 예정일이 정확하고 **발표 시각**이 있으며 지표가 **EPS**다. `/earnings`·`/calendar` → 국내 리다이렉트
  - **W1 발표 후 주가 반응** — 서프라이즈 옆 「↗ +2.4%」. 값어치는 **둘이 엇갈린 줄**에 있다(실측 한국금융지주 +27.8% → −7.3%, 엘앤에프 −58.2% → **+7.3%**) → 「상회·하락」 배지. 세션별로 다른 종가 쌍: **장전(bmo)은 발표 당일**, 장후·국내는 다음 거래일 — 안 나누면 반응을 하루 잘못 읽는다
  - **⚠️ 목업 초안의 「클리오 +28.2%」가 틀린 값이었다** (구현 중 발견). `stock_daily_closes`의 **최근 세션 커버리지가 얇다** — 08-07까지 16,190종목/일인데 **08-10부터 200~700**(candle_bars도 1,017→692). 가드 없이 「다음 거래일」을 찾으면 08-13 발표에 **다음 주** 종가가 잡혀 7일치 등락이 그 이름으로 나간다. `maxReactionGapDays = 5`로 인접성을 강제하고 멀면 값을 내지 않는다 → 화면은 **「집계 중」**(0%로 그리면 "반응이 없었다"는 다른 말). 최근 열흘이 대부분 「집계 중」인 것은 이 기능의 버그가 아니라 **일별 종가 파이프라인의 공백**이다
  - **W2 실적 전용 상세 모달** — 예전엔 범용 시세 모달이 떴다. 헤더(서프라이즈 + 반응 나란히) · 매출/영업이익/순이익 3타일 + **QoQ** · **분기 추이 막대(기대 vs 발표 겹침)** · 적중 이력 · 다음 발표 추정 · **개별 기준 접기**. 목록이 준 값으로 헤더는 즉시, 이력만 열 때 한 번 부른다(실측 보령: 서프 +13.8% · 반응 +2.4% · QoQ 3종 · 막대 4개 · 1/2분기 상회)
  - **W3 상단 스코어보드** — **평균과 중위를 함께** 적는다: 실측 08-06 국내가 중위 **+4.3%**인데 평균 **−3.3%**였다(LG헬로비전 −69.7% 한 건이 끌어내림). 하나만 보여주면 그날 시장을 반대로 읽는다
  - **W4 해외 장전·장중·장후·미공개 타임라인** — 해외에서만 가능하다(국내 소스엔 시각이 없다). 미공개를 따로 두는 이유: 장전·장후 중 하나로 찍으면 거짓말이다
  - **`CollapseDomesticBasis` 중복 구현 정리** — 이번 작업에서 같은 일을 하는 `ConsolidateEarnings`를 새로 만들었고, `Calendar`·`Surprises`에서 **둘이 연달아 돌았다**. 앞선 것이 개별 행을 **버려서** 뒤의 `alternates`가 늘 비었고, 사용자가 고른 「개별은 접어둔다」가 **죽은 코드**였다(API에서 `alternates` 0건으로 드러남). 하나로 합치고 **진 행을 버리지 않고 `Alternates`로** 넣는다. 키에 `Date`가 들어가는 것은 유지 — 같은 분기를 다른 날 두 번 발표한 것은 별개 사건이다. 실측: 미래에셋생명 2026 2Q → 1행(대표 연결 +54.6% · 접힘 개별 −60.8%)
  - **월/주/일 토글** — 실적은 주 단위로 몰린다(8월 국내 190건이 3~14일에 전부). 주간 보드는 월~금 5칸(주말은 영원히 비어 격자 2/7 낭비) + 칸마다 **서프라이즈가 가장 컸던 종목**. 보기 단위는 `lib/earnings-view.ts` + **`useSyncExternalStore`** — 이펙트에서 localStorage를 읽어 setState하면 캐스케이드 렌더고 렌더 중 직접 읽으면 하이드레이션이 어긋난다(`targets-view.ts`와 같은 형태)
  - **기본 날짜를 「데이터가 있는 날」로 자동 이동** (브라우저 실측에서 발견): 국내는 2·5·8·11월에 몰려 **「오늘」이 1년 중 대부분 빈다** — 처음 들어온 사람이 빈 목록과 「—」 다섯 개를 봤다. `days`가 오면 오늘 이전 마지막 유효일로 옮긴다(**파생 값**으로 계산 — 이펙트+setState는 캐스케이드 렌더). **한 번 고른 뒤에는 덮지 않는다**(`dateTouched`) — 고른 빈 날짜를 옮기면 클릭이 안 먹는 것처럼 보인다
  - **보류**: 모달의 「관련 뉴스」 — `news_articles.symbols_json LIKE`가 **0.83초 무인덱스 스캔**(75,522행)이라 모달 열 때마다 못 쓴다. 심볼→기사 인덱스(조인 테이블/생성 컬럼)가 별도 작업으로 필요
  - 검증: go build/vet/test(신규 시험 — 연결 병합 5케이스·적중 이력·스코어보드 하한·세션별 반응·인접성 가드)·tsc·eslint(신규·수정 파일 0건)·`next build`(40페이지) green, Docker 재기동(Tesseract OK). 엔드포인트 하루 2.7KB/0.023s · 달 14.7KB/0.070s · stats 299B/0.005s. **브라우저 DOM 실측** — 국내 자동 이동 8/14(상회 80%·중위 +19.8%), 08-06 전환(중위 +4.3% vs 평균 −3.3%), 보령 반응 ↗+2.4%, 해외 장전·장후·미공개 3레인, 상세 모달 전 섹션. ⚠️ **스크린샷 불가**(Browser 패널 미표시 → compositing 없음)
- 08-17: **실적 캘린더 BE — 연결·개별 중복 행 collapse** — FnGuide가 같은 분기를 `(연결)`·`(개별)` 두 줄로 주는데, unique key `(symbol, fiscal_year, fiscal_period, basis)`라 DB 중복은 아니었다. 시장 캘린더·히트맵·서프라이즈 랭킹이 같은 종목을 두 번 세·그려 React duplicate key(FE)와 목록 2배(8/7 NAVER·삼성증권 실측)를 유발. `model.CollapseDomesticBasis()` — 같은 (symbol, date, fiscalPeriod)면 **연결 우선** — 를 Calendar·Summary·Surprises·EstimatedDomestic(작년 근거)에 적용. 종목 탭 announcements는 개별·연결 둘 다 유지. 검증: go test·Docker 재기동(Tesseract OK), 035420 8/7 API 2행→1행(연결)
- 08-17: **실적 캘린더 FE — React duplicate key 경고 수정** — dev 콘솔에 `Encountered two children with the same key`가 반복(예: `035420-2026-08-07`, `016360-2026-2Q-confirmed`). API가 같은 종목·분기·날짜를 **서로 다른 수치로 두 줄** 내려주는데(NAVER 8/7 실측: 영업이익 4390억 vs 5203억), 목록 key가 `symbol-date`·`symbol-fiscalPeriod-dateStatus`만 써서 충돌. `earningsEventKey()`(수치 앵커 포함)를 `earnings-shared.tsx`에 두고 캘린더 본문·레일·서프라이즈·종목 탭 announcements에 적용. FE만 — Docker 재기동 없음. 검증: tsc·eslint(만진 파일 0건), `/earnings` 8/7 선택 후 콘솔 duplicate key 0건
- 08-17: **실적 메뉴 신설 — 시장 전체 실적 캘린더 + 컨센서스·서프라이즈** (`/earnings`, 사이드바 「실적」 그룹)
  - **앱이 세 곳에서 "없다"고 말하고 있었고, 그 중 하나는 사실이 아니었다.** 종목 실적 탭은 「무료 소스(DART/Finnhub/EDGAR)에서는 컨센서스·서프라이즈를 제공하지 않습니다」를 화면에 적고 있었는데, **Finnhub 응답에 `epsEstimate`/`epsActual`이 처음부터 실려 오는데 파서가 `date`·`symbol`·`quarter`만 읽고 버리고 있었다**(과거 2주 1,446행 중 1,135행에 `epsActual`). 국내는 FnGuide가 추정치·발표치를 함께 준다. `/calendar`는 실체 없이 리다이렉트만 하는 빈 라우트였다
  - **저장** `earnings_events`(`051`, 자동): 한 행 = 한 종목의 한 회계기간. 확정·예정만 저장하고 **국내 추정 발표일은 읽을 때 계산**(저장하면 실제 발표가 들어온 순간 지우러 다녀야 하고, 놓치면 같은 분기가 두 줄로 남는다 — 목표가 도달일과 같은 이유)
  - **국내 예정일은 어떤 소스로도 얻을 수 없다** (사용자 결정: 작년 동기 추정). DART에 예정 API 없고 FnGuide 캘린더도 미래 달은 `{"dataset":[]}`(202609·202610 실측), `IR|실적발표` 타입도 0건. 그래서 「작년 같은 분기 발표일 +1년 → KRX 거래일 스냅」이고, 근거를 `estimateBasis`로 같이 내려 화면이 「추정 · 작년 2Q는 8/3 발표」로 밝힌다. 규칙은 `EstimateNextAnnouncement` **한 곳** — 캘린더와 종목 탭이 다른 날짜를 말하면 안 된다
  - **실측이 다섯 번 설계를 뒤집었다** (전부 추측이 아니라 호출해 보고 알았다):
    ① **단위 = 억원** — 삼성 2025 결산 매출 `3,336,059` ↔ DART 확정치 333,605,938,000,000원(=3,336,059.38억) **6자리 일치**. 저장은 원 단위(×1e8)로 눕혀 `formatFinancialAmount` 하나로 끝낸다
    ② **`_G`는 YoY가 아니라 QoQ** — 삼성 3Q25 매출 86.1조·G=15.4로 역산한 Q2가 74.6조로 실제와 맞았다. 처음엔 "확정 불가라 쓰지 않는다"로 적었던 값
    ③ **`FREQ`에 `"D"`(연간 결산)가 있다** — 2월 발표분 480건이 `D`. 분기 코드만 받으면 결산 시즌을 통째로 놓친다 → `FY`로 정규화
    ④ **`"적전"`(적자전환) 같은 한국어가 숫자 자리에 온다** — `json.Number`가 디코드에서 거부해 **한 행이 그 달 400행을 통째로 날렸다**(202507·202508 유실). `flexNumber`로 숫자·숫자문자열·라벨을 모두 받고 라벨은 없는 값으로 둔다
    ⑤ **Finnhub `/calendar/earnings`는 1,500행 상한이고 잘릴 때 구간의 뒤쪽만 남긴다** — 4개월을 한 번에 부르니 10월 3주치(1,295행)만 들어왔다. **7일 조각**으로 나눠 받아 5,620행·전 구간 확보
  - **정정은 별 행이 아니라 병합** — FnGuide는 정정을 원 발표와 함께 준다(삼성 2Q: 07/07 + `[정정]`07/30). **날짜는 이른 쪽(`LEAST`), 수치는 `source_date` 큰 쪽만** 덮고 `revised_at`을 남긴다. 정정일로 덮으면 "삼성전자는 7/7에 발표했다"가 7/30이 된다. 실측 확인: `event_date 2026-01-08 · revised_at 2026-01-29 · 매출 333.61조`
  - **서프라이즈는 BE에서 한 번만 계산**(`model.ComputeSurprise`) — 화면에서 다시 구하면 홈·목록·탭이 다른 숫자를 말한다(`toTargetPct` 선례). 국내=영업이익, 해외=EPS(시장 관행). **기준치 0 이하면 퍼센트를 내리지 않고** 「흑자전환·적자전환·적자지속」 라벨로 답한다 — 적자 예상에서 흑자가 나면 −800% 같은 수가 된다
  - **랭킹에 유의성 하한**(영업이익 50억원 · EPS 0.1달러): 없이 두니 1위가 **+5,498%**였고 컨센서스가 1억원인 종목이었다. 하한 뒤에는 대한항공 +319.9%·SK이노베이션 +123.8%로 바뀐다. 하한 아래 행은 캘린더 본문에는 그대로 나온다(감추는 게 아니라 줄 세우기에서만 뺀다)
  - **지난 추정일은 내지 않는다** (브라우저 실측에서 발견): 8월 14일 목록이 **173건**인데 실제 발표는 20건이고 129건이 「추정 8/14」였다 — 빗나간 추측이 진짜 발표를 묻었다. 과거 칸은 일어난 일만 말한다
  - **응답 크기**: 월 상세를 그대로 내리면 3,089행·gzip 136KB·DOM 3천 줄이라 기본을 **오늘 하루**(2.1KB)로 두고 달 전체는 300행 상한 + 「N건 중 300건」 표기. 히트맵은 날짜별 건수만(292B). 실측 summary 0.026s · 하루 0.011s · 서프라이즈 0.041s
  - **한도 버킷 두 개 신설** — `fnguide`(자체 설정 60/일·30초, 공개 API가 아니라 게시된 한도가 없다)와 `finnhub_earnings`. 후자는 `ProviderFinnhub`의 5분 간격이 **라이브 ETF 폴러용**이라 배치가 폴러 때문에 실패한 실측에서 나왔다(`finnhub_migrate`와 같은 이유). 여러 달·여러 조각을 받는 경로는 `Manager.MinInterval`로 **스스로 쉰다** — 15개월 백필이 첫 달만 넣고 14개월을 잃은 뒤 고쳤다
  - **적재는 워커 전용**(`worker-earnings`, 국내 시즌 10–20시/2h · 비시즌 18:30 · 해외 06:00 KST, 기동 catch-up). 남의 사이트 두 곳을 부르는 유일한 워커라 API 프로세스와 분리한다. 종목 탭의 `usCalendar`도 조회 중 Finnhub 호출을 없애고 표에서 읽는다
  - **FnGuide는 공개 API가 아니다** (사용자가 약관 리스크 감수 결정). 되돌릴 수 있게: `FNGUIDE_ENABLED` 킬 스위치 · 수집을 `marketdata/fnguide` 한 패키지에 가둠 · 하루 2호출 · 화면 상시 출처 표기
  - **화면**: 월 히트맵(중립 농도 — `--brand`가 `--loss`와 같은 값이라 파란 농도는 "하락한 날"로 읽힌다) → 일자 목록 → 서프라이즈 보기 토글, 우측 레일(상회/하회 TOP·내 종목 다음 발표). **확정·예정·추정 세 배지를 절대 같은 모양으로 그리지 않는다**. 종목 클릭 = 기존 `SymbolQuickLookModal`
  - 검증: go build/vet/test·tsc·eslint(신규·수정 파일 0건)·`next build`(38페이지) green, Docker 재기동(Tesseract OK), `051` 적용 로그 확인. **브라우저 DOM·네트워크 실측** — 08-06 셀 「발표됨 511·예정 49·추정 3」, 08-14 현대해상 「기대 3690억원 → 5050억원 +36.9%」, 추정 행 「작년 2Q는 8/14 발표」, 서프라이즈 50행·국내 전환 시 영업이익 기준, 종목 탭 삼성전자 `consensus.available:true`·다음 발표 추정 2026-10-14. ⚠️ **스크린샷은 불가** — Browser 패널이 표시되지 않아 compositing이 안 된다(DOM·네트워크로는 전부 확인). ⚠️ 읽을 때 `main`이 **둘**이다(모바일 셸이 width 0으로 함께 존재) — `main[1]`이 실물이다
- 08-17: **기사 모달 — 종목·지수 없는 기사에서 메모·스탬프 버튼 제거 + 우측 빈 칸 숨김** — `NewsArticleContext`가 `!hasEntities`일 때 기사 단위 `NewsGatedActionBar`(메모·스탬프)를 띄우던 폴백을 제거했다. 08-15에는 "종목 없으면 기사 단위로 남긴다"였으나 실제로는 `/journal`·`/stamps`만 열리고 종목·기사 링크가 저장되지 않아 거시 기사(금리 등)에서 오동작으로 보고됨. 관심·메모·스탬프는 **`StockMiniCard`에만**. iframe 모달 우측 칸은 `articleModalContextHasContent`(관련 종목·지수 또는 같은 이슈 묶음)가 없으면 **아예 그리지 않음** — FE만, Docker 재기동 없음
- 08-17: **뉴스 시장 분류 — 국내 경로 해외 종목 기사 → 해외 탭** — `inferNewsMarket`의 `inTitle`이 티커(NVDA)만 보고 한글명(엔비디아)은 제목 증거로 안 셌다. `symbolsMentionedInTitle`로 제목에서 이름·별칭 매칭. 기존 domestic 행은 기동 1회 `RemigrateDomesticToUS`(`news:remigrate-market:v1:`). 예: 「중국 헤지펀드, 엔비디아·AI 인프라…」→ us — `lib/dashboard-news-meta-fit.ts`로 측정 분리, `queueMicrotask`+`ResizeObserver`. 뉴스 FE eslint 7건( set-state-in-effect·exhaustive-deps ) — 모달 `key={article.id}` 프레임 분리·mock 목록 effect 제거
- 08-17: **홈 「오늘의 뉴스」 행 높이 축소** — `LatestNewsCard`에서 언론사 배지를 빼고 지수·종목 칩(`NewsArticleChips`)을 제목 아래 메타 줄(속보·시각 사이)로 옮겼다. 칩이 별도 줄이면 옆 「오늘의 시장 브리핑」과 높이를 맞추느라 빈칸이 커졌다. **시간은 칩 바로 옆** — `flex-1`로 시간이 행 끝까지 밀리던 것을 칩+시간을 `inline-flex`로 묶어 수정(이후 08-17 wrap 수정으로 1줄 고정)
- 08-17: **출시 전 응답 속도 개선 — FE·BE 실측 기반** (사용자 요청: 기능 축소 트레이드오프까지 포함)
  - **먼저 쟀다.** 느린 엔드포인트 다섯을 특정하고 원인을 하나씩 분리했다 — 추측으로 고쳤으면 전부 엉뚱한 데를 짚었을 것이다. 실제로 「Redis 라운드트립」으로 지목한 뉴스 병목은 고쳐 놓고 보니 **시간이 그대로**였고, 진짜 원인은 CPU(전수 스캔)였다
  - **인덱스 2종** (`050_perf_indexes.sql`, 부팅 시 자동): `idx_stock_scores_rank(trade_date, score DESC, peer_pct DESC)` — 랭킹 쿼리가 `stocks` 17,070행을 풀스캔+filesort 하던 것을 없앤다(**2.10초 → 0.036초**). `idx_news_market_updated(market, updated_at)` — 목록 응답의 `meta.updatedAt` 한 칸을 채우는 `MAX(updated_at)`이 3만여 행을 읽었다(**727ms → 0.45ms**). 인덱스 이름이 마커라 테이블/컬럼 방식과 달리 `indexExists`를 새로 뒀고, 6개 컨테이너 동시 부팅은 1061을 `isSchemaRaceError`로 삼킨다. **드롭 후 재기동으로 재적용까지 실측 확인**
  - **`STRAIGHT_JOIN`으로 구동 테이블 고정** — 옵티마이저는 자유롭게 두면 `asset_type` 조건이 있는 `stocks`를 먼저 잡는다. `stock_scores`부터 읽으면 정렬이 인덱스에서 공짜로 나오고 LIMIT에서 스캔이 끝난다(해외 탭은 전체의 75%라 스무 줄에 서른 행 남짓)
  - **gzip 미들웨어**(`chimw.Compress(5)`) — 그동안 **압축이 아예 없었다**. 98KB가 그대로 나갔다. SSE(`text/event-stream`)는 chi 기본 목록 밖이라 압축되지 않고 Flush도 그대로 위임된다
  - **뉴스 목록 = 이번 작업의 절반.** 세 층이었다: ① 기사마다 `ResolveEmbedPolicy`가 호스트별 Redis GET(요청당 342회) → 호스트 단위 in-process 캐시로 **0회**, 미스는 배경 조사 ② `extractUSSymbols`/`extractSymbols`가 기사마다 이름 인덱스 전수 스캔 → **세대+본문 메모** ③ `trackedSymbols`·`usIndexSymbols`가 기사마다 watchlist+포트폴리오 조회 → 1분 TTL. 마지막에 목록 자체를 15초 캐시(ingest 시 즉시 무효화). **us 1.35초 → 0.09초(cold) / 0.027초(warm), domestic 0.42초 → 0.05초**
  - **홈 전용 뉴스 엔드포인트**(`/news/{market}/dashboard`, 사용자 선택) — 홈은 100건을 받아 여덟 줄만 그리고 나머지 92건은 키워드 집계·속보 수에만 쓴다. **앞 8건은 온전히, 나머지는 제목·태그·키워드만**. 집계 규칙을 서버로 옮겨 적지 않은 이유는 그러면 같은 규칙이 두 언어에 두 벌 생기기 때문이다. 98KB(gzip 29KB) → **35KB(gzip 10KB)**. `fullCount` 밖 기사로는 모달을 열지 않는다(본문·링크가 없다)
  - **리포트 목록 차트**(사용자 선택) — `chart.bars` 200봉 OHLCV(건당 19KB) → `chart.closes` 종가 60점. 둘 중 하나만 채워 보내고 **상세는 `bars` 그대로**. 응답 72KB(무압축) → **21KB(gzip 6.5KB)**
  - **실적 배치 엔드포인트**(사용자 선택) — `/stocks/earnings/batch?symbols=` 신설(상한 20). 홈이 종목당 1요청씩 8개를 쏘던 것을 1개로. 한 종목 실패는 그 종목만 빠진다(나눠 부르던 때와 같은 결과)
  - **FE 3단 waterfall 제거** — 「랭킹 → 배치 시세 → 실적」이었다. 일정에 필요한 건 종목 코드뿐이라 랭킹 직후 `onSymbols`로 올려 **시세와 일정이 나란히** 나간다. 이름은 늦게 와도 되므로 행에 담지 않고 그릴 때 찾는다(담으면 그때의 빈 이름이 굳는다). 실측 네트워크 로그에서 두 요청이 같은 밀리초에 출발
  - **FE 요청 캐시**(`lib/fetch-cache.ts`) — 화면마다 카드가 자기 fetch를 하는 구조는 그대로 두되(한 카드가 죽어도 나머지가 뜬다), **진행 중인 같은 요청을 합치고** 짧은 시간 재사용한다. 라이브러리는 안 들였다. 랭킹·점수·실적 5분, 뉴스 30초, 배치 시세 5초(시세를 캐시하려는 게 아니라 같은 순간 두 카드가 같은 목록을 부르는 것만 거른다). 로그인·로그아웃 시 `clearFetchCache()` — 키에 사용자 정보가 없다
  - 홈 API 요청 **17개 → 11개**, 최장 체인 3단 → 2단
  - 검증: go build/vet/test green, tsc·eslint(신규·수정 파일 0건)·`next build`(37페이지), Docker 재기동(Tesseract OK), 마이그레이션 드롭→재적용 실측. **브라우저 실측** — 홈에서 `news/domestic/dashboard`·`earnings/batch` 200 + `quotes/batch`와 동시 출발 확인, 응답 모양 확인(뉴스 100건 중 fullCount 8·50번째는 url 없음 / research `closes` 60·`bars` 없음 / earnings 심볼별 calendar), 리포트 화면 스파크라인 3개가 `viewBox="0 0 320 48"`에 60점 좌표로 실제 렌더, 뉴스 화면 정상. **화면 육안(스크린샷) 검증은 미완** — Browser 패널이 표시되지 않아 compositing이 안 된다(DOM·네트워크로는 전부 확인)
- 08-16: **홈 대시보드 개편 — 클릭 = 모달 4종 + AI 시장 브리핑 + 브리핑 우선 배치** (목업 `docs/mockups/dashboard-widgets-v2.html` → 사용자 선택: I2 · W1 · L3 · 모달 4종 전부)
  - **홈만 앱의 규칙 밖에 있었다.** 관심·보유·뉴스·리포트는 이미 「모달로 훑고 마음에 들 때 넘어간다」인데 첫 화면은 지수→`/market`, 종목→`/analysis`, 뉴스→`/news/domestic`, 리포트→`/reports/{id}`로 **전부 이탈**했다. 홈에서 하는 일은 하나를 정독하는 게 아니라 여럿을 견주는 것이라 셋을 비교하려면 뒤로가기를 세 번 눌러야 했다
  - **모달 4종 중 셋은 이미 있던 것을 연결만** 했다(`StockQuickLookModal`·`NewsArticleModal`·`ResearchQuickLookModal`). 신규는 **지수 모달 하나** — 앱에서 유일하게 모달이 없던 대상이었다(`news-dashboard-indices.ts`에 `index modal (future)` 주석이 남아 있었다)
  - **`SymbolQuickLookModal` 신설** — 공용 종목 퀵룩은 이름·통화·현재가가 채워진 객체를 요구하는데 홈은 그게 없다(점수 랭킹은 종가만, 브리핑 언급 종목은 심볼·이름뿐). 목록이 시세를 들고 있으면 `seed`로 **요청 0**, 없으면 열 때 배치 시세 1회. 화면마다 "시세를 어디서 구해 오나"를 다시 짜면 같은 종목이 자리마다 다른 정보로 열린다
  - **지수 모달(I2)**: 일별 종가 + **그날의 점수 열** + 추세점수/AI 한 줄 + 같은 시각 다른 지수 + 이 지수를 다룬 뉴스. 점수 열이 핵심 — 「63점」이 좋은 건지는 어제 61, 그제 57을 봐야 알 수 있고 그 값은 `daily-closes` 응답에 **이미 실려 온다**
  - **기간 버튼을 거래일 수로 바꿨다** (실측 중 발견): `limit=66`을 줘도 코스피는 **31행**만 온다(07-02부터). 캘린더 기간으로 이름 붙이면 「3개월」과 「1년」이 **똑같은 차트**를 그린다 → 최대치를 **한 번만** 받아 클라이언트에서 자르고, 앞 버튼보다 실제로 더 많은 봉을 그리는 단계만 남긴다(31행이면 「20일」·「전체」 둘, 하나뿐이면 토글 자체를 안 그림)
  - **`aiSummary`가 툴팁에 갇혀 있었다** — `/indices/scores`는 지금도 문장을 주는데 화면은 `title` 속성에만 넣어 **모바일에서는 존재 자체가 안 보였다**. 금·비트코인은 `score: null`인데 문장은 오므로 **점수 유무로 블록을 감추지 않는다**(감추면 있는 문장까지 사라진다)
  - **AI 시장 브리핑(W1)** — 서버에 있었지만 **뉴스 화면에만** 있었다. `/news/{market}/briefing`은 **읽기 전용**(생성은 배치·관리자만)이라 방문자가 늘어도 LLM 비용은 그대로. 뉴스 카드는 가로 띠라 문단을 3줄로 자르고 나머지를 모달로 뺐지만, 홈은 세로 한 칸을 통째로 써서 **핵심 사건·지켜볼 것까지 펼친 채로** 둔다. 언급 종목 칩 = 종목 모달
  - **지수 스트립 = 한 줄 자동 흐름 + 직접 밀기**(`IndexTickerStrip`, 사용자 결정). 카드 5장 그리드와 아래쪽 「전체 지수」 접힘을 **둘 다 없애고** 열두 종을 한 줄에 흘린다 — 접힌 것은 아무도 펼치지 않아 홈이 실제로 보여주던 지수는 다섯이었다. **hover·focus에서 멈추는 것은 장식이 아니라 기능**: 셀이 모달을 여는 버튼이라 움직이는 동안에는 조준할 수 없다. 목록을 **세 벌** 이어 붙이고 가운데 벌에서 시작한다(두 벌이면 뒤로 밀 때 곧장 시작점에 부딪힌다), 사본은 `aria-hidden`+`tabIndex=-1`. `prefers-reduced-motion`이면 흐르지 않는다
  - **흐름을 CSS transform이 아니라 `scrollLeft`로 돌린다** (사용자 요청: 멈춘 채로 직접 밀 수 있게). transform으로 밀려면 컨테이너를 `overflow:hidden`으로 잠가야 해서, 멈춰 놓고 볼 수 있는 것이 화면에 걸린 일곱 칸뿐이 된다. 스크롤바는 `.hide-scrollbar`로 감추고 기능만 남긴다
  - **터치 팬은 포인터 이벤트로 잡을 수 없다** — 팬이 시작되면 브라우저가 `pointercancel`·`pointerleave`를 쏘아 손가락이 아직 화면에 있는데 멈춤이 풀리고, 띠가 손가락과 싸운다. 그래서 포인터는 **마우스 hover와 키보드 포커스만** 받고, 사용자 조작은 **스크롤이 우리가 지정한 위치와 어긋났다는 사실**로 판정해 1.2초 유예한다(휠·트랙패드·손가락·관성이 한 규칙으로 잡힌다). 멈춘 동안 모은 이동분(`carry`)은 버린다 — 안 그러면 놓는 순간 훌쩍 건너뛴다
  - **이음매 간격**: 벌마다 `pr-2`를 들고 있으므로 바깥 flex에 `gap`을 주면 안 된다. 주면 **벌과 벌 사이만 16px**이 되어 무한히 이어져야 할 띠에서 그 자리가 눈에 띈다
  - **점수 배지에 「AI」를 붙였다**(사용자 요청) — 처음 온 사람에게 코스피 옆의 `60`은 등락률도 순위도 아닌 정체불명의 수다. 다만 **툴팁은 정확해야 한다**: 이 점수는 LLM이 아니라 20일선·RSI·최근 등락으로 계산한 규칙 기반 값이다(`index_score.go:116` — *"Written from computed values only — no LLM"*). 모달 섹션도 「AI 추세 점수」로 맞춰 배지 뜻이 화면마다 갈리지 않게 했다
  - **스파크라인이 12종 중 9종에만 있다**(원/달러·나스닥100 선물·필라델피아 반도체는 빈 배열, 실측). 없는 칸에 빈 상자를 두면 구멍으로 보이므로 **그림 자리를 빼고 값을 한 단계 크게** 적는다. 점수도 9종만 있다(금·비트코인·원/달러 없음) — 배지를 안 그릴 뿐
  - **배치**: 지수 스트립(전폭) → [브리핑 | 오늘의 뉴스] 2단 → 4열 [점수 TOP · 거래대금 · 실적·공시 · 오늘의 키워드] → 리포트. 브리핑 미생성 시간대에는 2단을 만들지 않고 **뉴스가 전폭**으로 늘어난다
  - **「다가오는 일정」을 「실적·공시」로 바꿨다** (구현 중 실측으로 뒤집힘): BE 주석이 그대로 말한다 — *"DART has no forward schedule API, so domestic symbols only ever produce `filed` events"*. 거래대금 상위 국내 8종목의 `upcoming`은 **0건**이라 원안대로면 **영원히 빈 카드**다. 대신 예정(미국 Finnhub)과 최근 제출(국내 DART)을 한 목록에 두되 **날짜 표기로 구분**한다(`D-9` ↔ `2일 전`). **종목당 1건** — SK스퀘어 하나가 기재정정까지 세 줄을 먹어 일곱 줄이 전부 같은 공시였다
  - **일정 날짜 형식이 출처마다 다르다** → `lib/earnings-date.ts` 신설. DART는 `2026-08-14`(ISO), Finnhub는 **`8/26`(연도 없는 M/D)**. 지금까지 화면들이 `date.slice(5)`로 잘라 써서 미국 일정이 **「6」**으로 떴다. 연도 없는 값은 올해로 읽되 반년 이상 과거면 내년으로 넘긴다. 종목 퀵룩의 같은 버그도 함께 교정
  - **「오늘의 키워드」는 추가 요청 0** — 기사는 `DashboardOverview`가 한 번만 받아 목록 카드와 키워드 카드에 내려준다(각자 부르면 「기사 100건 중」과 목록이 다른 스냅샷이 된다). 칩 클릭 → `/news/domestic?keyword=`. 이 파라미터가 **없어서 새로 뚫었다** — 초기값으로만 읽고, `[market]` 초기화 이펙트가 마운트에서 돌지 않게 `prevMarket` ref를 뒀다(안 그러면 뜨자마자 스스로 지운다)
  - **리포트 카드 = 리포트 화면과 같은 가로형**(R2). 목록 응답에 `chart.bars`(200포인트)·`evidence.breakdown`·`progress`·`toTargetPct`·조회·좋아요가 **이미 전부 실려 온다**(실측) — 화면이 부족했던 게 아니라 받아 놓고 안 쓴 것이었다. **추가 요청 0**. 좌(근거: 표지·발행시점 스파크·5축) / 우(주장: 제목·발췌·진행축)이고 양쪽 다 `flex-1`·`mt-auto`로 **바닥까지 채워** 빈자리를 남기지 않는다. 목표까지 거리는 서버의 `toTargetPct`를 쓴다(화면에서 다시 구하면 목록과 상세가 다른 숫자를 말한다)
  - **모달 닫기 버튼이 헤더 글자와 겹쳤다** — 닫기는 `top-4 right-4`(16px)인데 헤더 여백이 `px-4`(16px)라 정확히 포개졌다. 같은 구조의 **모달 7종** 헤더에 `pr-10`. `DialogHeader` 기본값으로 못 넣는 이유는 소비자가 주는 `px-4`가 tailwind-merge에서 나중에 와 `pr`을 덮기 때문
  - `MiniToggle`을 `components/ui`로 승격(대시보드·브리핑 공용), `MoodBadge`를 뉴스 카드에서 export. `MarketOverview`(그리드)·`MarketHeroStrip`·`AllIndicesSection`·`DashboardTopSection` 삭제, `MarketOverviewCompact`만 남김
  - **제안했으나 채택 안 됨**: 장 상태 바(`/market-calendar` — 동작은 확인, 단 **다음 개장일이 응답에 없어** "다음 개장까지 N시간"은 불가) · 이슈 종목 스트립 · 비로그인 온보딩(`/sample/*`) · 리포트 의견 분포. **만들 수 없는 것**: 급등·급락 TOP — 랭킹 API가 거래대금·거래량만 지원하고 등락률 정렬이 없다
  - 검증: tsc·eslint(신규·수정 파일 **0건**)·`next build`(37페이지) green. **API 실측** — 브리핑 국내·해외 둘 다 `source:"llm"`, `indices/scores` 12종, `kospi/daily-closes` 31행 중 6행에 점수, 국내 기사 100건에서 키워드 14개(실적 23·SK하이닉스 16·반도체 8), 실적·공시 24건 중 종목당 1건으로 7행. **화면 육안 검증은 미완** — Claude Browser 패널이 표시되지 않으면 `requestAnimationFrame`이 안 불려 React가 커밋을 못 한다(fetch는 전부 200인데 DOM은 스켈레톤에서 정지, 연결된 Chrome도 없음). 패널을 띄운 뒤 **자동 흐름·hover 정지·4열·리포트 카드**를 재확인해야 한다
- 08-16: **메모장·목표가·리포트 3화면 개편** (목업 `docs/mockups/journal-widgets-v1.html`·`targets-widgets-v1.html`·`reports-widgets-v1.html` → 사용자 선택)
  - **메모장 = 기록이지 예측이 아니다.** 스탬프처럼 적중/빗나감을 매기지 않고 **작성 시점 가격만 박제**한다(`047`: `price_at_write`·`price_currency`·`price_date`·`tags_json`). 「삼성전자 실적 좋아 보임」이라고 적어두고 두 달 뒤에 열면 그때 주가가 없어서 내가 옳았는지를 이 화면 안에서는 알 수 없었다. **`Update`는 가격을 건드리지 않는다** — 문장을 고쳤다고 그때 가격이 오늘 값으로 바뀌면 박제가 아니라 재구성이다
  - 메모장 **L2 2단 인박스**(목록 \| 본문). 예전엔 작성 폼이 항상 목록 맨 위에 펼쳐져 있었고 본문은 클램프가 없어 긴 메모 하나가 화면을 다 먹었다. 수정을 누르면 폼이 목록 위에 또 떠서 **고치는 중인 행은 저 아래** 남았다. `<lg`에서는 목록만 남고 행 클릭 = 모달
  - **태그**(`매수근거`·`리스크`·`관찰`·`복기` 프리셋 + 자유). 지금까지 분류 축이 종목뿐이라 **종목 미지정 메모**(환율·시장 관찰)는 시간순으로만 쌓였다. 레일에 「**다시 볼 메모**」(30일 지남 + 그동안의 등락) 신설 — 메모장을 다시 열게 만드는 건 새로 쓰고 싶은 마음이 아니라 옛 메모가 맞았는지 궁금한 마음이다
  - **목표가 v2** (`048`: `base_price`·`base_date`·`stop_price`·`target_price_2`). 예전 행은 「목표까지 +11.4%」 숫자 하나라 **가까운 건지 먼 건지가 안 읽혔다**. 등록가→현재가→목표가(+손절)를 한 축(`TargetAxis`)에 좌표로 올린다. 재등록 시 `base_price`는 `COALESCE`로 보존 — 목표를 올릴 때마다 등록가가 리셋되면 「등록 후 +14.9%」가 영원히 0 근처를 맴돈다
  - **도달일·최고/최저는 저장하지 않는다** — 스탬프 조기 적중과 같은 방식으로 `candle_bars` 일봉 고/저를 `base_date` 이후로 훑어 **읽을 때 계산**한다. 저장하면 목표가를 수정한 순간 옛 목표 기준의 도달일이 그대로 남아 어긋난다. 방향(상방/하방)은 **등록가 기준**으로 판정 — 하방 목표를 고가로 기다리면 영원히 판정되지 않는다
  - 목표가 목록은 **두 보기**(사용자 결정): 진행축 행 `row` ↔ 카드 그리드 `card`, `targets-view.ts`(`holdings-view.ts`와 같은 형태)에 저장. 상단 4타일(클릭 = 상태 필터) + 상태 칩 + 정렬 3종. 레일에 「**목표가 없는 보유 종목**」 — 이 화면을 실제로 채우게 만드는 유일한 진입점
  - **리포트: 이미 응답에 있는 데이터를 목록이 안 쓰고 있었다.** `chart.bars`·`evidence.breakdown`·`toTargetPct`/`toStopPct`가 전부 실려 오는데 카드는 텍스트뿐이었다 → **가로형 카드**(좌: 표지·발행시점 스파크·5축 / 우: 제목·발췌·진행축), **추가 요청 0**. 스파크는 발행 전 회색·발행 후 검정 + 발행 지점 점 하나, 목표·손절 파선
  - **리포트 퀵룩 모달** — 예전엔 카드를 누르면 곧장 페이지를 떠나, 셋을 비교하려면 뒤로가기를 세 번 눌러야 했다. 근거만 훑고 마음에 들 때 「전문 읽기」. 레일에 「곧 판정됩니다」 D-N·「목표·손절에 근접」·「의견 분포」 추가, 「많이 다룬 종목」은 이제 **목록을 거른다**(종목 분석 링크는 건수 쪽에 유지)
  - **썸네일**(`049`: `thumbnail_url`) — 기존 `/api/v1/media/images` 재사용(키가 내용 해시). 작성 폼에서 ① 업로드 ② **본문 첫 이미지 가져오기** ③ 안 함, 기본은 ③이고 화면이 **종목 로고**로 대신한다. 모든 작성자에게 그림을 요구하지 않으면서 목록이 들쭉날쭉해지지 않는 유일한 방법. `normalizeThumbnailURL`로 `javascript:`·`data:`·프로토콜 상대(`//`) 차단 — 이 값은 공개 카드의 `<img src>`로 직행한다. 표지 변경은 `body_rev`를 올리지 않는다(논거가 아니라 표현)
  - **예시 모드에 실시간 캔들을 섞지 않는다** (실측 중 발견): 목표가 카드가 「현재 214,500원」 옆에 실제 캔들 범위 **1,359,000~2,959,000원**짜리 차트를 그렸다. 한 카드가 서로 다른 두 종목을 말하는 셈. `useSparklineCandles(symbol, !sample)`로 차단(메모 상세 모달도 동일 — 픽스처 기준선이 실제 범위 밖이라 선이 사라진다)
  - 레일 카드 이름 충돌 교정: 상단 타일 「도달 임박」(목표 쪽만)과 레일 「도달 임박」(손절 근접 포함)이 같은 화면에서 1과 2를 동시에 말했다 → 레일을 「**곧 결판납니다**」로
  - 검증: go build/vet/test green, tsc·eslint(**신규 파일 0건**, 잔여 5건은 전부 기존 fetch-on-mount 패턴)·`next build`(37페이지), Docker 재기동(Tesseract OK). **DB 실측**으로 `047`·`048`·`049` 컬럼 3종 확인 — `047`은 로그가 안 찍혔지만(다른 컨테이너가 레이스에서 이겨 `isSchemaRaceError`로 삼킨 것) 컬럼은 정상. **브라우저 실측** — 메모장 예시 5건(삼성 +8.5%·NVDA +8.3%·카카오 −7.5%, 태그 4종 집계, 작성 후 평균 +3.1%), 목표가 예시 5건(도달/임박/손절근접 배지, 카드↔행 토글 localStorage 유지, 레일 5카드), 리포트 실데이터 2건(5축 막대·목표까지 +10.0%/손절 −29.9%·퀵룩 모달 전 섹션·곧 판정됩니다 D-84/D-85)
- 08-16: **종목 헤더 액션 통일 — 메모장·목표가 관리**
  - **FE**: `StockSymbolActionBar` 공통 컴포넌트 — 관심·스탬프·리포트 작성·메모장·목표가 관리 5종
  - 실시간 차트(`StockWorkspaceHeader`)와 AI 추천 랭킹(`AnalysisClient`) 우측 상단에 동일 배치
  - `useWatchlistToggle` 훅으로 분석 화면 관심 토글
- 08-16: **뉴스 종목·지수 매칭 개선**
  - **US 종목**: `ListUSNameCatalog`로 전 미국 종목 이름 인덱스 구축(기존 ~30종 한정 → 전체 마스터). 한국어 별칭(`마이크론`→MU, `애플`→AAPL 등) + possessive 헤드라인(`Micron's …`) ingest
  - **지수**: 금(`gold`/`금`)·BTC 규칙 확장, Goldman 오탐 방지. US 번역 후 한국어 headlineKo로 **지수 태그 재판정**
  - **재태깅 v3**: symbols + `index:*` tags 동시 소급 (`news:retag:v3:`)
  - **FE**: 기사 카드 하단에 **지수 칩**(금·비트코인·코스피 등) + 종목 칩 함께 표시 — `NewsArticleChips`
  - 검증: go test service green, Docker 재기동. **재기동 후 v3 retag 스윕이 자동 실행**되어 기존 기사 태그 갱신
- 08-16: **뉴스 품질 개선 — 국내 암호화폐 오탐 차단 + 해외 Google News 게이트**
  - **국내**: `strongMarketTerms`에서 BTC·가상자산 등 crypto 단독 키워드 제거 → `isDomesticCryptoOnlyArticle`으로 순수 코인 기사 차단(ingest·list 공통). 코인 전문 RSS(tokenpost·blockmedia·coinreaders)는 **증시 앵커**(코스피·증시·상장 등) 또는 **국내 종목 심볼** 없으면 통과 불가 — 코인 매매 맥락의 매수·매도·배당만으로는 더 이상 통과하지 않음. **코인리더스 RSS 폴링 제거**(DB에 남은 기존 건은 list 필터로 제외). `domesticNewsQualityScore`로 한경·연합·네이버 등 프리미엄 매체 우선 정렬
  - **해외**: Google News(`google_news`) ingest·list 모두 **제목에 시장 앵커**(`hasUSTitleMarketAnchor`) 필수 — hostname `news` 등 저품질 출처의 비증권 기사 차단
  - 검증: go test `TestIsStockRelated*`, `TestPassesStockRelevance*`, `TestPassesUSStockRelevance_googleNewsRequiresTitleAnchor`, Docker 재기동(Tesseract OK). **DB에 쌓인 기존 코인리더스 기사는 list 필터로 즉시 제외** — 완전 삭제는 `/admin` cleanup 또는 `CleanupIrrelevantArticles` 수동 실행
- 08-15: **스탬프 v2 — 예상 주가 필수 + 자유 만기 + 화면 전면 개편** (목업 `docs/mockups/stamps-widgets-v1.html`·`stamps-widgets-v2-target-price.html` → 사용자 선택)
  - **스탬프가 "북마크"에서 "예측"이 됐다**. 예상 주가가 필수가 되면서 **저장 전에 폼**을 받는다 (v1은 원클릭 저장 후 가설 부착). 저장된 스탬프는 전부 채점 가능한 완성본
  - **`stance`는 더 이상 고르지 않는다** — `target_price` vs 당시가에서 파생(`StanceForTarget`, ±3% = 관망). "약세인데 목표가는 위" 같은 모순이 구조적으로 불가능해졌다. 밴드 상수는 `model.StampNeutralBandPct` 하나만 두고 판정(`neutralBandPct`)이 그걸 재사용 — 두 개면 관망으로 접수하고 다른 밴드로 채점하는 일이 생긴다
  - **`horizon_days` → `horizon_kind` + `due_at`** (`046`). 「이번 장 종료」·「다음 장 종료」는 일수로 표현이 안 된다. 만기는 **생성 시점에 캘린더로 확정 저장** — 나중에 계산하면 휴장일 목록이 갱신될 때 과거 판정이 흔들린다. 6종: `session_close`(마감 후면 다음 장) `next_session_close` `1w` `1m` `3m` `custom`. 전부 **장 마감 시각**에 착지하므로 판정에 항상 실제 종가가 있다. 시장별 캘린더(KRX 15:30 / US 16:00 ET)는 `scheduleForStamp`가 고르고, `benchmarkForStock`·`marketTradeDate`와 **US/ETF 분기가 같아야** 만기·벤치마크·거래일이 서로 다른 시장으로 흩어지지 않는다
  - **판정 = 방향 + 정확도 2단** (사용자 결정). 적중/빗나감은 종전대로 **방향**으로 가려 기존 적중률 통계를 잇고, `targetAccuracy`(|만기가−목표|÷목표)를 **별도 축**으로 더했다. NVDA가 목표 +18%에 실제 +0.4%로 「적중」이 되는 구멍을 지표가 드러낸다
  - **장중 도달 = 조기 적중 종료** (사용자 결정). `candle_bars` 일봉 고/저로 `[stamped_at, due_at]` 구간을 훑어 강세는 `high ≥ target`, 약세는 `low ≤ target`이면 그 날짜로 확정하고 **즉시 개봉 대기**로 올린다. 만기 종가만 보면 "닿았다가 되돌아온" 예측이 빗나감으로 기록된다. **스탬프 당일은 제외** — 그날 고가는 스탬프보다 앞설 수 있어 이미 지나간 가격을 적중으로 쳐주게 된다. **관망은 조기 적중 없음**(닿을 방향이 없다)
  - **예측은 5분(=쿨다운)만 수정 가능** — `PATCH /stamps/{id}/prediction`, 이후 `ErrStampPredictionLocked`. 오타는 고치되 결과를 보고 목표를 옮기는 건 막는다. `note`·`tags`·`conviction`은 계속 열려 있다(`PATCH /stamps/{id}`). 리포지토리도 `UpdateAnnotation` / `UpdatePrediction` 둘로 갈랐다
  - **기존 스탬프 9건 전량 삭제** (사용자 승인) — 예상 주가를 사후에 만들어 낼 방법이 없다. 백업 `docs/archive/stamps-pre-046.sql`
  - **마이그레이션 레이스**: API+워커 6개가 동시에 부팅해 `046`을 함께 실행 → 진 쪽이 `Can't DROP 'horizon_days'`(1091)를 뱉었다. 스키마 최종 상태는 정상이지만 6개 컨테이너가 매 재기동마다 에러를 찍는다 → `isSchemaRaceError`(1060·1061·1091)로 무시. **이 배포 형태에서는 정상적인 결과**지 실패가 아니다
  - **FE**: A-1 성과 타일 4개(적중률 타일 → **성적표 모달**) / B-1 행 = **도장 실링**(적중·빗나감·개봉·검증 D-N, 테두리로 상태 구분) + 목표가 게이지 + 이후 경로 스파크(목표가 파선 동반) + 4구간 막대 / C-1 상세를 **모달로 전환**(관심·보유·뉴스와 통일) / D **개봉 모달** — 대기분을 한 장씩 넘기며 결과 공개 + 회고 한 줄(기존 `note` **뒤에 덧붙임**, 지우지 않음) / 레일 E-2 개봉 예정 타임라인 · E-3 종목별 성적 · E-6 스탬프 후 최대 등락
  - **E-6은 BE가 계산한다** — 조기 적중 판정이 어차피 일봉 고/저를 읽으므로 `peakReturn`·`troughReturn`을 같은 스캔에서 실어 보낸다(FE 추가 요청 0). "맞았지만 도중에 -6%를 버텼다"가 종가만 보는 표에서는 통째로 사라진다
  - 집계는 전부 `lib/stamp-stats.ts`에서 **이미 받아둔 배열로** 계산 — 관점별·기간별·확신도별·종목별·누적 알파·구간 비교 전부 서버 재요청 없음
  - 검증: go build/vet/test(신규 `stamp_horizon_test.go` 8건 포함), tsc·`next build`(37페이지)·eslint(만진 파일 잔여 1건은 기존 fetch-on-mount 패턴), Docker 재기동(Tesseract OK, 마이그레이션 에러 0). **브라우저 실측** — 목록 6행·실링 5종·게이지·레일 3카드, 성적표 모달 4탭, 개봉 모달(적중 카드 1/2), 상세 모달 전 섹션, 생성 폼(삼성전자 274,500원 → 목표 310,000 입력 시 **관망 0.0% → 강세 +12.9%** 파생, 「이번 장 종료」가 토요일이라 08-17(월), 「직접」은 날짜 전까지 저장 비활성). 로그인 게이트라 목록은 fetch 스텁으로 실측
- 08-15: **기사 모달 관련 종목/지수 카드 통일 + 미니 차트 정렬 버그**
  - **미니 차트가 현재가와 안 맞던 버그**: 애플 카드가 $305를 적고 차트는 작년 10월($266~286)을 그렸다. 원인은 `useSparklineCandles.takeCloses`의 `slice(-40)` — **일봉 응답 순서가 출처마다 다르다**(DB 경로는 오름차순으로 뒤집지만 **Toss 일봉 tail은 최신이 앞**, BE `newestCandleIndex` 주석이 같은 사실을 적고 있다). 그래서 최신 40개가 아니라 **가장 오래된 40개**를 거꾸로 그렸다. `candleTimeSortKey`로 **정렬 후 자르기**(캔들스틱 차트가 이미 쓰던 방식). 실측 06-18~08-14 / $275~$340로 교정
  - **`StockMiniCard`·`IndexMiniCard` 신설** (`components/stocks`·`components/market`) — 같은 껍데기. 액션(관심·메모·스탬프)이 **카드 안으로** 들어갔다. 예전 모달 하단 한 줄은 종목이 여럿이어도 **첫 번째 종목에만** 작용했다(아마존·애플 기사에서 애플만 담김). ~~종목이 없는 기사에서만 기사 단위 메모·스탬프를 남긴다~~ → **08-17 제거**: 종목·지수 없는 기사에는 액션 없음
  - **`StockScoreChip` 신설** — 숫자만 있던 배지에 `종합점수 30/100 C` + ⓘ 팝오버(5축 이름·"매매 신호 아님"·축별 보기 링크). 퀵룩 모달의 사설 배지도 이걸로 교체
  - **탭 → 카드 전부 펼침 + 지수 카드**: `NewsRelatedEntityPanel`이 종목 최대 6장 + 기사에 걸린 지수(`articleDashboardIndexIds`)를 지수 먼저 세운다. 점수는 `useStockScores` 배치 1요청, 일정은 종목당 1요청(상한=카드 수)
  - **시세 상한 구멍**: 화면 공유 배치는 60종목 상한이라 그 밖의 종목 카드가 "시세 못 불러옴"+통화 원화 폴백이 됐다(아마존 실측). 패널이 **빠진 것만** 한 번 더 부른다
  - **종목 클릭 목적지 통일** → `analysisSymbolHref(symbol,"ai")`. 뉴스 칩·대시보드 랭킹·보유 미리보기·목표가 카드·관심 레일. 「터미널」·「일봉」이라고 **적힌** 링크만 `/stocks/{symbol}` 유지
  - **BE 태깅 두 건** — ① 미국 이름 인덱스가 도메인·클래스 꼬리를 못 뗐다: AMZN 마스터명이 `Amazon.com`이라 `amazon`이 키에 없어 **"Amazon rising"이라 쓴 기사는 전부 놓쳤다**(Apple·MSFT·META는 멀쩡해 더 안 보였다). `trimHeadlineNameTail`(`.com`·`alphabet a`) 추가 → 재태깅 스윕 **us 13,503건 중 335건 수정**, 문제의 Fortune 기사 `["AAPL"]` → `["AAPL","AMZN"]`. 표식 `news:retag:v1:` → **v2** ② 코스피 규칙에 「서울 증시」·「seoul stocks」 추가(BE·FE 양쪽)
  - **지수 판정이 번역문을 안 봤다**: 해외 기사는 화면에 한국어가 뜨는데 규칙은 영어 원문만 봤다 → FE `articleText`에 `headlineKo`·`summaryKo` 포함
  - **지수 차트 금액 표기**: `PriceHistoryChart`에 `valueFormat="plain"` — KOSPI 7,096.89가 「7,097원」으로 찍히고 있었다
  - 검증: go build/test, tsc·eslint(만진 파일 0건)·`next build`, Docker 재기동(Tesseract OK). **브라우저 실측** — 문제의 기사에서 KOSPI·애플·아마존 3카드, 링크 전부 `/analysis?symbol=…&tab=ai`, 점수 팝오버 동작. 주의: 브라우저 창이 숨겨져 있으면 `useMarketIndices`가 `isPageVisible()`에 막혀 지수가 안 뜬다(검증 시 `visibilityState` 강제 필요)
- 08-15: **뉴스 코인리더스 필터 + 지수 카드 필터 통일**
  - **코인리더스 RSS**: 가상자산 키워드만 있으면 통과하던 crypto RSS 예외 제거 → **주식·증시 키워드**(`hasStockMarketKeyword`, crypto 단독 제외) 없으면 ingest 차단. 토큰포스트·블록미디어는 기존 crypto RSS 규칙 유지
  - **지수 pulse 카드**: `/market` 이동 제거 → 이슈 종목과 동일하게 **기사 N건** 표시 + 클릭 시 해당 지수 키워드로 목록 필터(코스피·나스닥 등 pill 매핑, 원/달러 등은 `index:` 태그·텍스트 매칭). 국내·해외 공통
  - 검증: go test `TestPassesStockRelevance*`, tsc 통과
- 08-15: **뉴스 화면 레이아웃 정리 (국내·해외 공통)**
  - **「내 종목」 pill**: 브랜드 전용 색 제거 → 다른 pill과 동일 스타일. 순서를 **「전체」 바로 다음(2번째)** 으로 이동
  - **지수 흐름**: 우측 레일 `IndexPulseCard` 제거 → 중앙 상단 `NewsIssueStrip`에 이슈 종목과 **같은 카드 형태**로 5개 지수 선행 배치(클릭 시 `/market`). `pickMarketIndices`를 `index-pulse-card.tsx`에서 공유
  - **레일 순서**: 오늘의 키워드 → 다가오는 일정 → 많이 본 뉴스 → 내 종목 뉴스 → 출처 분포 → 최신 투자 리포트. 해외 전용 `NewsMacroCard`는 레일에서 제거(국내·해외 동일 구성)
  - 검증: tsc 통과
- 08-15: **종목 모달 통일 + 헤더 검색 전역화 + 뉴스 레일 정리**
  - **퀵룩 모달 공용화**: `components/stocks/stock-quick-look-modal.tsx` 신설 — 헤더·40일 종가 차트·52주 위치·5축 점수·다가오는 일정·뉴스가 공통이고, 화면별 차이는 `contextSection`(맥락 지표) / `extraSections`(폼) / `footerActions`(삭제·제거) 세 슬롯으로만 들어온다. 관심종목·보유종목 모달은 얇은 래퍼가 됐고, 보유 점수 카드가 들고 있던 사설 `ScoreDetailModal`은 지우고 같은 모달을 연다 — **같은 종목을 어디서 누르든 같은 화면**이어야 한다
  - **하단 CTA 목적지**: `/stocks/{symbol}` 워크스페이스 → `analysisSymbolHref(symbol, "ai")`. 목록에서 종목을 눌러 처음 궁금한 건 호가창이 아니라 "이 종목이 지금 어떤가"다
  - **헤더 검색 전역화**: 감추는 경로는 `/login`·`/signup`·`/onboarding`뿐이고 나머지 기본 모드가 `stock`. 카탈로그 자동완성에서 고르면 AI 추천 랭킹으로 간다. `/analysis`·`/quotes`·`/market`은 종전대로 자기 화면 안에서 종목을 바꾼다(`quotes` 모드, 페이지가 등록한 핸들러). **엔터는 종목코드·티커로 읽히는 입력만** 이동시킨다 — `?symbol=삼성전자`는 존재하지 않는 심볼이라 한글 이름은 화면 안 필터로 남긴다. 모드 판정은 `supportsStockSuggestions()` 한 곳(provider와 입력 필드가 갈라지지 않게)
  - **기사 모달을 닫으면 뉴스 목록까지 닫히던 버그**: 기사(포털 `z-[70]`) 위에서 목록 Dialog를 `modal={false}`로 두니 기사 백드롭 클릭이 목록까지 흘러갔다. 목록을 **기사 보는 동안만 닫고**(`open={open && !selectedArticle}`) 스크롤 위치를 ref로 복원하는 쪽으로 전환 — 기사·필터·페이지 상태는 Dialog 바깥에 있어 언마운트돼도 재조회가 없다
  - **뉴스 레일 모달**: 종목당 25 → **15건**, 비중 상위 8 → **10종목**
  - **총 자산 카드**: 원화·달러 현금을 **같은 글자 크기**로 나란히 표기. 내부 Metric 칸(테두리·회색 배경)은 제거하고 **흰 카드 안에 텍스트만** 나열
  - **`set-state-in-effect` 정리** — 이번에 만진 파일은 전부 무경고. 세 가지 패턴으로 바꿨다: ① 열림/닫힘에 딸린 초기화는 이펙트가 아니라 **렌더 중 조정**(`wasOpen !== open`) — 이펙트로 하면 닫힌 프레임이 한 번 더 그려진다 ② 자동완성 결과는 **자기 key를 들고 다니고** 로딩은 `key` 불일치로 판정 ③ 헤더 `activeQuery`는 상태가 아니라 `debouncedQuery`에서 **파생** — 화면이 바뀌면 첫 프레임부터 비어 있다
  - 검증: tsc·`next build`(37페이지)·eslint(만진 파일 0건, 저장소 잔존 90건은 전부 기존 파일). **브라우저 실측** — 홈에서 `삼성` 입력 → 자동완성 8건 → 엔터 → `/analysis?symbol=102780&tab=ai`. 관심종목·보유종목은 로그인 게이트라 **모달 육안 미확인**
- 08-15: **내 포트폴리오·목표가 UI 수정**
  - **뉴스 기사 모달 겹침**: `SymbolNewsModal` 위에서 기사를 열면 `NewsArticleModal`(z-50, 비포털)이 Radix Dialog 포털 뒤에 깔림. body 포털 + `z-[70]` + 기사 열면 목록 Dialog `modal={false}` (포커스 트랩이 기사를 먹지 않게)
  - **평가금액 추이 일괄등록**: `BulkRegisterDialog`가 `onOpenChange(true)`에서만 drafts를 채워, 부모가 `open`을 true로 켜면 목록이 빈 채로 뜸 → candidates에서 바로 파생
  - **총 자산 현금**: 하단 10px 줄 → 총 매입금액·평가손익·종목 수와 같은 중앙 Metric. **일괄 추가**는 총 자산에서 빼고 `종목 직접 추가` 우상단으로
  - **목표가**: 「이 브라우저에만 저장됩니다」 문구 삭제(이미 서버 저장)
  - 검증: tsc 통과. FE만 — Docker 재기동 없음. `add-holding-card` JSX 닫힘 태그 어긋남으로 런타임 오버레이가 뜬 건 파일 전체 재작성으로 해소
- 08-15: **목표가 `useSyncExternalStore` 무한 루프 수정**
  - `/targets` 런타임 오류: `getServerSnapshot`이 `() => []`로 매 렌더 새 배열 반환 → `lib/price-targets.ts`에 모듈 스코프 빈 배열 캐시(`getServerSnapshot`)
- 08-15: **매매 이력 가격 불일치 정리**
  - `trade_date` **당일 또는 직전** 종가(`stock_daily_closes`) ≠ 기록 단가(평단)인 종목의 **매매 이력 전체 삭제** — 평가금액 추이 오점 원인. `MigrateHoldingTradesPriceMismatchCleanupIfNeeded` (API 기동 시 불일치 있을 때마다 실행). 1차(당일 종가만) 8종목·16건, 2차(직전 종가 fallback) **추가 8종목·16건** 삭제, `user_holdings.source` → `manual`. `cmd/cleanup-holding-trades` (`-dry-run`)
- 08-15: **메뉴 IA 개편 + 내 포트폴리오·목표가·일괄등록 개선**
  - **사이드바 IA**: `핵심/리포트/기록` → **`순위`(실시간 차트·AI 추천 랭킹) · 뉴스 · 내 투자(내 포트폴리오·관심 종목·스탬프·메모장·목표가) · 커뮤니티(투자 리포트) · 관리`**. 활성 메뉴 재클릭 시 `window.location.reload()`로 새로고침
  - **뉴스 모달 버그**: `SymbolNewsModal` fetch effect가 `targets` 배열 참조에 걸려 필터 칩 클릭 시 **"전체"로 리셋** → `key`만 의존. 기사 링크 `/news/{id}`(404) → **`NewsArticleModal` 팝업**
  - **평가금액 추이 일괄등록**: 오늘 날짜 고정 → **`BulkRegisterDialog`**(종목별 매수일 지정). BE `POST /user/holdings/trades/bulk-register`(기존 일괄등록 trade **업데이트**, 동일 키 **중복 insert 거부`). `044_holding_trades_dedup.sql`로 기존 중복 행 정리
  - **총 자산 카드**: `현금 ($)`+`달러 현금` 분리 제거 → 하단 **`$5,000(7,000,000원)`** 통합 표기. **현금·일괄 추가** 버튼을 `HoldingsSummaryCard`로 이동(`BulkAddHoldingsDialog` 신설)
  - **목표가 서버화**: `user_price_targets` + `GET/POST/PUT/DELETE /user/price-targets`. FE `lib/price-targets.ts` → API + localStorage **1회 마이그레이션**. `/targets`·`HoldingsTargetsCard` 전부 API CRUD
  - 검증: go build, tsc, Docker API 재기동(Tesseract OK)
- 08-15: **국내주식·해외주식(뉴스) 화면 개편 + 종목 태깅 파이프라인 수정**
  - **뉴스 기사의 종목 태그가 망가져 있었다** (A1·C1의 전제). 원인 셋이 겹쳤다 —
    1. 국내 이름 인덱스를 `trackedSymbols()`(관심·보유에 담긴 종목)로만 만들어, **아무도 담지 않은 종목은 영원히 태깅 불가** → `ListDomesticNameCatalog()` 전 종목(4,406개)으로 교체 + 30분 TTL
    2. 해외는 **티커 자체를 이름 인덱스 키**로 넣고 경계 없는 `strings.Contains` → 티커 `V`가 알파벳 v 든 모든 기사에 붙음(실측 73건 중 51건 Visa). 티커 매칭을 `usTickerTokenMatch`로 분리, **3자 미만 티커는 `$V`·`NYSE: V` 형태만**
    3. **가장 결정적** — `marketdata.MergeTickerSymbols`가 **1~5자 영문 대문자만** 통과시켜, 이름 매칭이 찾아낸 국내 6자리 코드를 바로 다음 줄에서 전부 버림. 수집 경로도 원래 이랬다 → `mergeTickerCandidates`(기존 심볼 재검사 안 함)로 교체
  - 한국어 특유의 보정 둘: **조사 화이트리스트**(`링크드`가 "링크드인"에 붙던 것 차단), **최장 일치**(`두산`이 "두산에너빌리티"에 붙던 것 억제 — "에"가 조사이기도 해서 경계 검사로는 못 잡는다)
  - 결과(최근 100건): 국내 **0 → 18건**, 해외 Visa 오탐 **51 → 1건**
  - **재태깅**: 기동 시 1회(`news:retag:v1:*` Redis 표식). **MySQL만 고치면 화면이 안 바뀐다** — `loadFromStorage`가 Redis 스냅샷을 먼저 보므로 스윕 후 스냅샷 무효화 필요. 실측 59,663건 스캔 / 29,215건 수정
  - **AI 뉴스 브리핑**(A3): `news_briefings` + `GET /news/{market}/briefing`(읽기 전용) + 스케줄러 + admin 배치 2종. 입력은 헤드라인 40(매체당 최대 5) + 지수 + 언급 상위 10(시세 포함) + 속보수 + 세션 + **직전 브리핑 10건 압축본(150자)**. 출력은 길게 받아 카드에서 3줄 클램프 + 모달 전문
  - **FE**: A1 이슈 종목 스트립 · A2 24시간 뉴스 흐름 · A3 브리핑 카드+모달 / B1 「내 종목」 pill(비로그인은 인기 종목으로 **실제 동작** + 예시 배지·안내 띠) · B2 2단 필터 / C1 종목 칩+등락률 · C2 시간 구분선 / D1 모달 관련 종목 패널 · D2 같은 이슈 묶음 · G4 액션 팝오버(`?next=`에 기사 id) / E1~E4 레일 카드
  - **시세 요청은 화면당 1회** — 스트립·행 칩·레일·모달이 `useSymbolQuotes` 하나를 공유한다
- 08-14: **관심종목 행 — 카드 전폭 사용 + 52주 위젯 원복** (사용자 지적)
  - **카드의 40%가 비어 있었다**: 칸을 전부 고정 폭으로 잡아 남는 폭이 현재가 오른쪽에 몰렸다. `WL_COL`을 **`flex-[n]` 비율 + `min-w` 바닥**으로 바꿔 어느 폭에서든 카드를 다 쓴다(1200px에서 종목 187·52주 209·기간 각 110·현재가 143)
  - **52주 위치는 3줄 블록으로 원복**(사용자 결정) — 저·고가 값 / 그라데이션 트랙+마커 / 「상단 63%」. 한 줄로 눌렀더니 정보는 맞아도 보기 싫었다. 행 높이 48→**66px**(사용자가 허용)
  - **정렬 함정**: 칸 상수의 `@2xl:block`이 `WatchlistCell`의 `flex`를 이겨 `justify-center`가 무효가 되고 있었다 — 전일·전주·전월·담은 뒤만 값이 6px 위로 붙음. 가운데 정렬을 **패딩(`py-[6px]`)** 으로 바꿔 display와 무관하게 만들었다. 점수 배지도 22→19px `WatchlistScoreCell`로 값 줄에 앉힘
  - 검증: 컨테이너 300~1200px 전 구간 **정렬 어긋난 칸 0 · 오버플로 0 · 행 66px**, 값 줄 전부 top 6px, 375px 카드 351px에 빈 공간 0
- 08-14: **관심종목 행 정돈(A안) + 종목명 깜빡임 수정 + 모달 차트 공용화**
  - **종목명이 5초마다 종목코드로 튀던 버그**: BE `zeroQuoteSummary`(`quotes_overlay.go`)가 `toss.GetStock` 실패 시 **`Name`에 심볼을 넣어** 응답한다. 빈 문자열이 아니라 FE `mergeQuoteSummary`의 `!next.name` 가드를 통과 → 폴링 한 번만 실패해도 이름이 코드로 덮어써짐. `isPlaceholderName`(빈 값 또는 `name === symbol`)로 판정해 **마지막 정상 이름 유지**. `market`·`logoUrl`도 같은 이유로 가드 추가. **BE는 손대지 않음** — 다른 소비자가 `Name` 비어 있음을 가정하지 않는다
  - **행 A안**: 라벨을 행에서 걷어내고 목록 맨 위 **헤더 한 줄**로. 행 높이 62→**48px**. 폭·접힘 규칙은 `watchlist-columns.tsx`의 `WL_COL` 한 곳 — 헤더와 행이 같은 상수를 봐야 표가 어긋나지 않는다
    - 헤더 좌우 패딩은 `px-[11px]` = 행 `px-2.5`(10) + **카드 테두리 1px**. 10px면 라벨이 값보다 1px 왼쪽에 선다
    - 현재가는 `min-w`가 아니라 **고정 폭**(`w-24`/`@2xl:w-28`) — 내용에 맞춰 늘리면 행마다 앞 칸들이 밀려 헤더와 안 맞는다
    - `WatchlistCell`이 값 18px·보조 12px로 **높이 고정** — 보조 줄 없는 칸도 자리를 비워야 값이 한 선에 온다
    - 접힘: 전월(@5xl) → 전주(@4xl) → 담은 뒤·핸들(@3xl) → 전일·52주 막대(@2xl)
  - **레인지 바**: 파랑→빨강 그라데이션 제거(이 앱에서 그 두 색은 하락·상승이라는 뜻이 있다). 중립 트랙 + 브랜드색 채움. `variant="inline"`(행) / `"block"`(모달)
  - **`PriceHistoryChart` 신설** (`components/charts/`) — 관심종목 퀵룩·보유종목 상세 **공용**. 최고·최저 **금액 축** + 위아래 여백 + 면적 + 시작가 기준선 + 끝점 + 기간 변화율. 보유는 여기에 `markerPrice=평단` 점선(140×44 미니 스파크라인 대체)
    - 금액 라벨은 SVG `<text>`가 아니라 **옆 HTML 칸** — `preserveAspectRatio="none"`로 가로를 늘리면 `<text>`도 찌그러진다. 세로 배율은 1이라 뷰박스 y를 px로 그대로 쓴다
    - 하단 라벨은 **"종가 N개"**(❌ "N거래일") — 커버리지 성근 종목은 40봉이 8개월에 걸친다(실측 12-16~08-14)
  - `useSparklineCandles`가 `times`도 반환(차트 축 날짜). 이참에 `set-state-in-effect` 제거 — `{key, points, times}` 패턴으로 loading 파생
  - 검증: tsc·eslint 통과. 임시 하네스 실측 — 컨테이너 278~1198px 전 구간 행 48px·오버플로 0, **헤더/행 전 칸 좌우 좌표 완전 일치**, 값 8칸 top 92px 동일, 375px 헤더 `종목·점수·52주·현재가`, 모달 486×931에 실데이터 차트(274,500원/94,400원 축)
- 08-14: **관심종목 탭 UI 개편** (목업 `docs/mockups/watchlist-widgets-v1.html` → 사용자 선택 C안(스파크라인 제외)+D+E+F)
  - **행 재설계**: 5칸 동일 무게 지표 → `점수 배지 · 52주 레인지 바 · 전일/전주/전월 3칸 · 담은 뒤 · 현재가 · 임박 일정 점`. 행 높이 100→62px
  - **컨테이너 쿼리 필수**: 칸 접힘 기준은 뷰포트가 아니라 **본문 컬럼 폭**(`@container` + `@2xl/@3xl/@5xl`). 1280px 화면도 사이드바+레일 300px을 빼면 목록이 600px대라 `sm:`류로 짜면 데스크탑에서 가로 오버플로. 모바일 포함 **전 구간 1줄 유지**(사용자 결정)
  - **`addedPrice`·`createdAt` 사용 시작**: `GET /watchlist`가 내려주던 값을 FE가 버리고 있었다 → 「담은 뒤 수익률」 열 + 레일 `WatchlistSinceAddedCard`. 옛 항목은 `addedPrice` 없어 `—`
  - **퀵룩 모달** `WatchlistQuickLookModal`: 행 클릭 시 페이지 이탈 대신 모달(40일 종가·52주 위치·담은 뒤·5축 점수·일정·뉴스·보유에 추가 폼). 데이터는 **열 때만** 조회
  - **툴바** `WatchlistToolbar`: 내 순서/등락률/52주 위치/담은 뒤/점수 + 국내·미국·상승·하락. 정렬은 **저장하지 않고**, 「내 순서」에서만 드래그 허용(정렬 상태 드롭이 내 순서를 덮어쓰는 것 방지)
  - **점수**: `GET /stocks/scores/batch` 목록 전체 1요청, 5초 폴링에 태우지 않음
  - **일정**: `useUpcomingEvents`(종목당 1요청·상한 12) 결과를 행 표식과 레일 카드가 **공유**. 타깃은 `visibleRows`가 아니라 **내 순서 기준**(등락률 정렬 시 5초마다 12요청 재발 방지)
  - **뉴스 모달 공용화**: `HoldingsNewsModal` 본체 → `components/stocks/symbol-news-modal.tsx`(보유는 얇은 래퍼)
  - 검증: tsc·eslint 통과. 로그인 게이트라 임시 하네스로 실측 — 컨테이너 278~1198px 전 구간 행 62px·가로 오버플로 0, 375px 1줄, 모달 486×602에 실데이터(캔들·실적·뉴스 200)
- 08-14: **보유종목 탭 UI·기능 개선**
  - **현금**: 레일 `CashBalanceCard` 제거 → `AddHoldingCard` 우상단 「현금」→ `CashBalanceDialog`. 원·달러·달러 환산은 `HoldingsSummaryCard` 지표에 항상 표시(주식+현금 동시)
  - **평가금액 추이**: 본문 상단 → **보유 점수 하단**으로 이동. 매매 이력 없는 보유 종목에 **일괄 등록**(수량·평단·오늘 매수 기록) CTA
  - **보유 종목 뉴스**: 「더보기」→ `/news` 이동 대신 **`HoldingsNewsModal`**(비중 상위 8종목 × 25건 선로드, 클라이언트 페이지·무한 스크롤). 카드는 최신 6건 유지
  - **목표가**: `HoldingsTargetsCard` 보유 종목 선택 **빠른 추가** 다이얼로그. `/targets` 입력은 **`StockCatalogSearchField`**(종목명·코드). **저장소 확인**: `quant-port-targets` localStorage만 — BE `user_*`·API 없음(투자 리포트 `research_calls.target_price`와 별개)
  - 검증: tsc 통과
- 08-13: **보유종목 현금(원·달러) 관리** — `user_cash_balances` + `GET/PUT /user/cash-balances`; FE 레일 `CashBalanceCard`(종목 추가 바로 아래), `holdings-calc` 총자산·비중, 요약 「총 자산」, 구성 도넛 **자산** 탭
  - **설계**: 매매 이력·Toss 예수금과 분리 — 사용자가 직접 입력한 예수금·달러 잔고만 합산. 달러 현금·달러 종목 모두 환율 없으면 합산 보류(기존 `canTotal` 규칙과 동일)
  - **검증**: go build/test, tsc, Docker API 재기동(Tesseract OK)
- 08-13: **매매 기록 입력 UX** — `TradeDateField`(한 줄 전체·한글 날짜·오늘/어제), 첫 기록 추가 시 현재 수량·평단 자동 채움
  - **증상**: 평가손익·현재가가 장중인데도 전 정규장 종가(또는 `symbol_last_prices` 배치 캐시)에 고정
  - **원인**: 보유/관심 목록 API(`GET /quotes?filter=held|watchlist`)의 `applyLivePrices`가 `marketStore.EnsurePrices` 배치값만 `currentPrice`에 넣고 **`ResolveLiveQuotePrice`(체결·호가·forming bar)를 거치지 않음** — 종목 상세·`/quotes/batch`와 경로 불일치. FE 보유 화면도 **최초 1회만** `filter=held` 조회(5초 폴링 없음)
  - **수정**: `enrichSummaryLivePrice` 신설 — poller Touch + microstructure warm + `ResolveLiveQuotePrice` + `applyQuoteChange`(등락률). FE 보유종목은 **`/quotes/batch`(EnsureQuoteDetail 경로)** + 5초 폴링(관심종목과 동일)
  - **검증**: go test `./internal/service`(LivePrice), tsc, Docker API 재기동(Tesseract OK)
  - **증상**: NAVER 19주는 정상인데 **안트로젠(065660)** 이 같은 수량·평단으로 추가 생성
  - **원인**: NAVER 손익 컬럼 OCR에서 `안트` 같은 **2글자 hangul debris**가 파싱·해석되며 `안트로젠`과 prefix score 1.0으로 auto-match
  - **수정**: resolver `shortHangulExactOnlyScore`(3글자 미만 hangul은 exact만), `isPureShortHangulDebris` fragment 필터, preview `dedupeResolvedPhantomRows`(065660+035420 동일 qty)
  - **검증**: live MySQL `안트`→no match; 3-fixture Docker OCR+resolve PASS; Docker API 재기동
- 08-13: **3장 스크린샷 업로드 — LG전자 누락·이상 행(phantom) 수정**
  - **증상**: 국내+해외 3장 업로드 시 LG전자 미인식, ARM 552주·`-SPOL…` 등 garbage 행 추가
  - **원인**: `$`만 보고 `overseas` locale → `16전자`(2글자 hangul) 탈락; OCR 수량 502·5020류 합산; finance 컬럼 debris 통과
  - **수정**: `원`+`$` 혼합 locale; `16전자`/`L6전자`→`LG전자` 보정·mergeKey `lg`+hangul; qty 5배↑ OCR 오인 시 작은 값 유지; `-`/`#`/SPOL·금액 포함 이름 필터
  - **검증**: `toss_domestic_7`+`toss_overseas_4`+`toss_mixed` fixture Docker OCR — LG전자 7주·ARM 50주; unit tests PASS
- 08-13: **스크린샷 OCR 평단가 — XOVR·SK하이닉스 계산 실패 수정**
  - **증상**: 토스 혼합 화면에서 XOVR 평단 `—`, SK하이닉스 ~2,867,955원(비정상)
  - **원인**: XOVR ~0.04% 수익률이 materiality 0.1% 미만으로 스킵; SK에 OCR garbage 수익률(45%)이 `reconcileParsedFinance`에서 PnL을 ~29M으로 부풀림 + KRW 평가액에 USD 태그
  - **수정**: `costestimate` 소액 USD·미세 수익률 허용, KRW/USD eval magnitude 추론, garbage return% vs tiny PnL 분리; `satellite_finance` 통화별 per-share 매칭·포트폴리오 요약행(`가나다순`) 차단; `SK하이닉스` OCR 이름 보정
  - **검증**: `overseas_toss_tsmc_xovr.png` Docker OCR — XOVR avg ~20.42, SK avg ~1,576,295(KRW); unit tests PASS
- 08-13: **API 패닉 방어 + graceful shutdown 강화**
  - **스크린샷 임포트 테스트 중 크래시 상세 원인**: import job(OCR) 자체가 아니라 **동시에 돌던 SSE stream hub race** — 보유종목 화면·다른 탭의 종목 차트가 quote-stream/candle-stream을 구독한 상태에서 market poller(1s)가 Publish, 업로드 직후 페이지 이탈 시 Subscribe cancel이 inner map을 수정 → `fatal error: concurrent map iteration and map write`(recover 불가, 프로세스 전체 종료). import는 HTTP 202 후 **백그라운드 goroutine**이라 미들웨어 Recoverer가 못 잡음
  - **근본 수정(이전 턴)**: streamHub Publish RLock을 순회·send까지 유지
  - **추가 방어**: `internal/safego`(백그라운드 goroutine panic recover+stack), import job processJob/이미지별 recover→job failed, market poller 심볼 refresh·batch worker tick에 safego.Call
  - **Graceful shutdown**: `Shutdown` 순서 HTTP drain → bgCancel → **import job WaitIdle** → pool close; shutdown timeout 30s; Recoverer에 method/path+stack 로그
  - **원인**: import job이 OCR 없이 `domestic` locale 재필터 → `TSMC(ADR)`(괄호)·`XOVR`(latin debris) 탈락
  - **수정**: `InferHoldingsLocale`(국내+해외 혼합 시 필터 생략), ADR 이름 domestic 허용, resolver `TSMC→TSM` 심볼 lookup
  - **검증**: `overseas_toss_tsmc_xovr.png` fixture + 기존 kiwoom/overseas 회귀 PASS
- 08-13: **스크린샷 임포트 — 비증권 이미지 방어 (강화)**
  - **증상**: 커뮤니티/채팅·엉뚱한 사진도 종목 미리보기까지 진행 (`7시간 전`, `검색/코드` 등 garbage row)
  - **수정**: `RejectNonBrokerageScreenshot` 강화(채팅 UI·UI 아티팩트·qty-only 무finance 거부), `RejectPreviewAfterResolve`(해석 후 garbage 비율·broker UI 없음), FE failed→upload 복귀+경고
  - **메시지**: `증권사 보유 화면을 스크린샷해 주세요.`
  - **검증**: `non_brokerage_chat.png` + 기존 kiwoom/overseas/watchlist fixture Docker OCR PASS; `screenshot_guard_test.go` 10건
- 08-13: **스크린샷 임포트 — 비증권 이미지 방어**
  - **증상**: 풍경·문서 등 엉뚱한 사진도 OCR·미리보기까지 진행
  - **수정**: `RejectNonBrokerageScreenshot`(보유/시세 화면 UI·금액·수량 신호 검증), `ParseHoldingsImage`→`ImageParseResult`(OCR 텍스트 반환), import job per-image + preview 신뢰도 게이트
  - **검증**: `screenshot_guard_test.go` 6건 + 기존 import 테스트 PASS; Docker API 재기동
- 08-13: **시세/관심종목 OCR — 종목명·코드만 (수량·평단 비움)**
  - **증상**: `$` 시세 화면(보유 수량 없음) 업로드 시 phantom qty·평단 또는 0건
  - **수정**: `InferQuoteListOnly` + quote list parser; name-only rows; import preview qty/avg skip
  - **검증**: `overseas_watchlist_quotes.png`; 기존 overseas holdings 회귀 PASS
- 08-13: **스크린샷 임포트 붙여넣기 UX** — 업로드 영역 클릭 시 폴더만 열리던 문제
  - **수정**: `Ctrl+V`는 Dialog/document 레벨 paste; **사진 선택**은 별도 버튼
- 08-13: **키움 삼성전자 OCR — 평단 5원 보정 + phantom 종목 제거**
  - **증상**: 삼성전자 109주 평단 359,262원(5원 단위 아님) + `키움 KPOP ETN`/`KO 키움자산` phantom 1~72주
  - **수정**: KRW 평단 5원 반올림(≥1,000원), broker UI noise 필터(키움자산/다른자), latin OCR debris(HIGHS 등) domestic 필터
  - **검증**: `domestic_kiwoom_samsung.png` → 삼성전자 109주 avg 359,260 단독; `go test ./internal/import/...` PASS
  - **증상**: 삼성전자 109주 평단 359,262원(5원 단위 아님) + `키움 KPOP ETN`/`KO 키움자산` phantom 1~72주
  - **수정**: KRW 평단 5원 반올림(≥1,000원), broker UI noise 필터(키움자산/다른자), latin OCR debris(HIGHS 등) domestic 필터
  - **검증**: `domestic_kiwoom_samsung.png` → 삼성전자 109주 avg 359,260 단독; `go test ./internal/import/...` PASS
- 08-13: **국내 ETF OCR — Kiwoom 평단가 계산 수정** (KODEX 21,476원 / TIGER 52,094원)
  - **증상**: KODEX 평단 15,198원(손실률 부호 누락·eval+return만으로 계산), TIGER 평단 `-`(finance 미연결)
  - **원인**: TIGER `..` 이름이 OCR fragment로 탈락; PnL `-1,225,.870` 등 comma·dot 혼합 OCR; 좌측 컬럼만 OCR 시 finance 컬럼 누락; KRW 손실 %에서 마이너스 누락
  - **수정**: inline name+eval / qty+pnl 파싱, finance 컬럼 OCR pass, domestic에도 `AttachSatelliteFinance`, mixed PnL amount parser, KRW unsigned return→loss 보정
  - **검증**: `domestic_etf_kiwoom.png` → KODEX avg 21,476 + TIGER avg 52,094; `go test ./internal/import/...` PASS; Docker API 재기동
- 08-13: **국내 ETF OCR — KODEX/TIGER 미국나스닥100 분리**
  - **증상**: 키움자산 스크린샷에서 KODEX 785주만 인식, TIGER 100주 누락 또는 동부건설 등 오매칭
  - **원인**: `mergeKey`가 hangul만 써서 KODEX/TIGER 둘 다 `미국나스`로 합쳐짐; fuzzy merge도 같은 지수명을 한 종목으로 처리
  - **수정**: latin issuer prefix(KODEX/TIGER) 유지, ETF OCR 보정(006×→KODEX), UI 노이즈(더보기) 필터
  - **검증**: `domestic_etf_kiwoom.png` → KODEX 785 + TIGER 100; resolver TIGER→133690; `go test ./internal/import/...` PASS
- 08-13: **해외주식 OCR — 맵러빈→앱러빈 해석 + 5종목 평단가**
  - **증상**: `맵러빈` "확인 필요"(APP 미해석), 평단가 4/5만 표시(앱러빈·오라클 등 `—`)
  - **원인**: OCR PnL `-6234.74(4.65%` 오인; `*러빈`+`app*` 규칙이 Applied/Apple 등 17k 카탈로그와 **동점(0.86)** → fuzzy margin 실패
  - **수정**: PnL return 역산; `*러빈`은 **applovin/applov** 전용; live MySQL `맵러빈`→APP(앱러빈) 검증
  - **검증**: `overseas_holdings.png` fixture 5종목 avg 418/37.2/217.7/335.77/143.5; `go test ./internal/import/...` PASS; Docker API 재기동
- 08-13: **해외주식 스크린샷 OCR — 5종목 파싱** (Applied/IREN/ARM/AppLovin/Oracle)
  - **증상**: `마미렌`/`맵러빈`/`머플라이드`, phantom `티비씨`/`나무가`, ARM 누락, 아이렌 50주(실제 55)
  - **원인**: `.404홀딩스(600)`·`50%`가 금액/다른 블록 qty로 오인; UTF-8 byte suffix 매칭 오류; 해외 `$` 목록인데 hangul 다수 → domestic으로 locale downgrade
  - **수정**(`textparse/` + `pipeline/ocr_passes.go`): hangul 이름은 money 스킵, `주`/`%` qty lock, ADR listing 허용, `FuzzyUnifyParsedLines`(해외 전용), overseas locale 필터(2글자 phantom 제거)
  - **검증**: `overseas_holdings.png` fixture — 5종목 exact; `go test ./internal/import/...` PASS; Docker API 재기동
- 08-13: **보유종목 OCR 평단가 계산 수정** — `1.852.010원` 등 dot thousands·`~` 손실 부호·괄호 손익 파싱
  - **증상**: 평단가 8/16/1/108원 등 비정상 값 (eval=383 등 OCR 금액 꼬리만 파싱)
  - **원인**: `krwAmountPattern`이 `[\d,]+`만 허용 → `1.852.010원`에서 `010`→10원; `~` 손실을 양수 PnL로 처리
  - **수정**(`textparse/finance_line.go` + `costestimate/estimator.go`): `parseMoneyAmount`, PnL/return **부호 재조정**(SDI `89,9908!`→손실, NAVER `(18.06%)`→마이너스), 천원 단위 스냅
  - **검증**: Docker OCR fixture — 삼성SDI 485,500·NAVER 260,000·삼성전자 364,500원; `go test ./internal/import/...` PASS
- 08-13: **보유종목 OCR 종목명 해석(resolver) — `삼성5미`/`16전자` 자동 매칭**
  - **증상**: OCR 7종목·수량은 맞지만 `삼성5미`·`16전자`가 "확인 필요"(종목코드 미해석). 단위 테스트는 통과했으나 **전체 catalog(17k+)** 에서 동점·오매칭으로 fuzzy 실패
  - **원인**: `ocrHangulCore`/`chosung`이 `삼성5미`→`삼성`, `16전자`→`전자`로 축소 → 수백 종목 1.0점 동점; `si` vs `sdi`는 prefix가 아니라 subsequence(`s…i`) 매칭 필요
  - **수정**(`import/resolver/fuzzy.go`): OCR 패턴별 전용 스코어링(선행 digit→`ocrLatinPrefixScore`, embedded digit→`ocrHangulLatinTailScore`+latin subsequence), 우선주 hangul 꼬리 페널티, embedded-digit fallback cap
  - **검증**: live MySQL `삼성5미`→006400, `16전자`→066570; `go test ./internal/import/...` PASS; Docker API 재기동
- 08-13: **보유종목 스크린샷 OCR — 이상 종목(TEADS/ALDEL) 제거 + 7종목 파싱** (하드코딩 없이 구조적 휴리스틱만)
  - **증상**: TEADS/ALDEL 등 해외 노이즈 종목 유입, 한화시스템 19주 오인, 삼성SDI·현대차 누락
  - **수정**: row-band OCR 제거, locale/fragment 필터 강화, `CollapseSimilarRows`, 다수결 qty 병합, orphan qty, mergeKey hangul 우선, 범용 OCR letter→digit confusion, 좌측 컬럼 upscale-only(전처리 패스 제거 — HAVER 노이즈 원인)
  - **검증**: Docker Tesseract 통합 테스트 PASS(7종목), `go test ./internal/import/...` 전체 통과
- 08-13: **보유종목 스크린샷 OCR 정확도 개선** — 사용자 제공 국내주식 7종목 테스트 이미지 기준으로 종목명·수량 파싱 정확도 대폭 개선.
  - **증상**: Tesseract PSM 11 단일 패스만 사용 → `삼성SDI`→`삼성5미`, `LG전자`→`16전자`, `4주`→`ae`/`a`/누락, `한화시스템` 23→19 오인식
  - **OCR 다중 패스**(`import/pipeline`): 전체 프레임(raw+2×전처리) + 좌측 55% 컬럼(2×업스케일) + 7행 밴드 OCR(PSM 6/7/11) → **다수결 병합**(`MergeMultiOCRPass`)
  - **전처리**: 2× 업스케일 + 다크테마 반전 + 이진화(140) — dim gray `4주` 텍스트 분리
  - **파서**: `ae`/`a` 스킵 제거 → OCR qty 힌트(4주 오인식), `주` 단독→LG 7주, `NormalizeBrokerOCRName`(삼성5미/16전자→정규명), garbage 필터(`. 삼성전자`, `e NAVER`)
  - **퍼지 매칭**: `삼성5미`·`16전자` resolver 테스트 추가
  - **픽스처**: `be/testdata/import/domestic_holdings_samsung.png` + Docker Tesseract 통합 테스트(`pipeline/ocr_passes_test.go`) + `cmd/ocr-debug`
  - 검증: go test 전체 통과, Docker Tesseract 통합 테스트 PASS(7종목 exact match)
- 08-12: **보유종목 직접 추가 — 종목 선택 UI** — `변경` 텍스트 버튼 제거, X로 선택 취소, 검색창(선택된 종목 영역) 클릭 시 다시 검색.
- 08-12: **보유종목 스크린샷 가져오기 → 우측 레일로 이동** — 본문 목록 아래에 있어 발견성이 떨어졌다. `종목 직접 추가` 바로 아래에 같은 `SectionCard` 크기의 진입 카드를 두고, 본문은 Dialog로 연다(`placement="rail"`).
- 08-12: **보유종목 화면 전면 개편 + 매입 이력(`holding_trades`) 신설** (목업 `docs/mockups/holdings-widgets-v1.html`로 옵션 11개 제시 → 선택받고 진행)
  - **🔴 데이터는 있는데 화면에 없던 것 2건**: ① 스크린샷 임포트가 `avgPrice`를 저장해 왔는데 목록이 안 썼다 → **수익률·평가손익이 화면에 아예 없었다** ② `deleteUserHoldingClient()`가 정의만 되고 **호출부 0곳** → 잘못 인식된 종목을 지울 방법이 스크린샷 재업로드뿐이었다. 둘 다 이번에 연결
  - **🔴 또 하나의 "정의 안 된 토큰"**: `@theme inline`이 `--color-chart-1: var(--chart-1)`을 매핑하는데 **`--chart-1`~`--chart-5`가 정의된 적이 없었다**(08-11 `--success`/`--danger`와 같은 사고). 그대로 쓰면 도넛 조각이 전부 상속색 한 덩어리가 된다 → 라이트/다크 양쪽 정의
  - **매입 이력 신설**(사용자 선택 3-C — "합성 곡선 말고 진짜 이력으로"): `039_holding_trades.sql` + repo/service/handler + `GET/POST/DELETE /user/holdings/trades`, `GET /user/holdings/valuation?days=`. 날짜별 실제 보유수량을 복원해 그날 종가 × 수량으로 평가금액 시계열을 낸다(**BE에서 계산** — FE로 하면 종목 수만큼 fetch에 기기 시계마다 답이 달라진다)
    - 평단은 **이동평균법**: 매도는 수량만 줄이고 평단을 재계산하지 않는다. 전량 매도 후 재매수는 평단이 **새로 시작**한다(옛 원가를 끌고 오면 안 낸 돈으로 값을 매기는 셈). 과매도는 공매도가 아니라 오타로 보고 0에서 막는다. 유닛 6건(`holding_trades_test.go`)
    - `user_holdings`는 이력이 있는 종목에 대해 **파생 캐시**(`source='trades'`)가 된다 — 기존 리더(quotes `held` 필터·대시보드·스탬프)를 하나도 안 건드리기 위해서다. 전량 매도 시 보유행만 지우고 **이력은 남긴다**(실현손익이 거기 있다). 보유 삭제는 이력까지 지운다(재추가 시 옛 원가 상속 방지)
  - **점수 배치 API 신설** `GET /stocks/scores/batch?symbols=` — 보유 종목 점수를 종목 수만큼 요청하던 걸 1회로. `MAX(trade_date)` 조인이라 점수가 끊긴 종목도 마지막 값이 나온다(목록에서 사라지지 않는다)
  - **손익 계산은 `lib/holdings-calc.ts` 한 곳**. 위젯마다 따로 더하면 같은 화면에서 합계가 어긋난다. 규칙 3가지는 전부 "모르는 값을 0으로 그리지 않는다"의 변형 — ① 평단 없는 종목은 손익 `null`이고 **합계 수익률의 분자·분모에서 함께 빠진다** ② 환율 없는데 달러 종목이 있으면 원화 합계 **보류** ③ 시세 없으면 평가금액 `null`
  - **본문**: 요약 큰 카드 1장(2-B) · 평가금액 추이(실선 평가 / 점선 매입) · 구성 도넛 종목+통화(4-B) · 오늘 손익 기여도(5-A, **등락률이 아니라 금액** 순 — 비중 큰 종목이 조금 움직인 게 더 큰 일이다) · 보유 점수 카드(클릭 시 5축 레이더 모달) · 표↔카드 전환 목록(1-C) · 스크린샷 마법사는 **접힌 채로**(10-A, 진행 중이면 자동으로 펼침)
  - **레일에서 뺀 것**: 보유 요약·비중 상위·많이 움직인 종목 — 전부 본문이 같은 말을 하게 됐다. 특히 **총 평가금액을 레일에 두면 안 된다**(레일은 `xl` 미만에서 본문 **밑**으로 쌓여 제일 중요한 값이 맨 아래로 간다). 남은 자리는 직접 추가·다가오는 일정·목표가·보유 종목 뉴스·지수
  - **목표가 저장소를 `lib/price-targets.ts`로 승격**(`targets-client.tsx` 사설 → 공용) — 보유 화면도 같은 목표가를 읽는다. 저장 키가 두 파일에 있으면 한쪽만 고쳤을 때 조용히 갈라진다. `useSyncExternalStore` 구독이라 목표가 화면에서 저장하면 보유 화면 카드도 함께 갱신된다(그 김에 기존 `set-state-in-effect` 1건도 사라짐)
  - **`set-state-in-effect` 회피 패턴**(이번에 반복해서 필요했음): 로딩 boolean을 이펙트에서 동기로 켜지 말고 **"어떤 요청에 대한 결과인지"를 데이터와 함께 들고** `loaded?.key === key`로 판정한다. 목록 새로고침도 `load()` 호출이 아니라 **버전 번호를 올려 이펙트를 다시 돌린다** — 이펙트에서 부른 함수 안의 setState는 린트가 이펙트 본문 것으로 본다
  - **실측 중 고친 것**: 도넛 상위 6개 + 기타를 쓰면 6번째 조각과 기타가 **둘 다 회색**이라 구분이 안 됐다 → 이름 있는 색은 5개까지, 회색은 **기타 전용**
  - 검증: go build/vet/test 전체 통과(신규 유닛 6건 포함), tsc·`next build`(37페이지) 통과, eslint 신규 파일 무경고(잔존 2건은 전부 기존 `screenshot-crop-canvas.tsx`). Docker 8컨테이너 재기동(Tesseract OK), `039_holding_trades.sql` 적용 확인(worker-scores 로그 + 13컬럼 실조회). `scores/batch` 실호출 200(5개 요청 → 4개 응답, 미존재 심볼은 빠짐). **브라우저 실측** — 로그인 게이트라 임시 하네스로 7종목 픽스처(평단 없는 종목 1개 포함) 렌더 후 삭제: 총 평가 24,108,903원 / 매입 19,923,545원(카카오 제외) / 손익 +2,093,358원 +10.51%, 카카오 평단·손익 모두 `—`, 도넛 5색+기타, 점수 가중평균 66.0 A3·B·C2·D1, 모달 열림(486×552), 표↔카드 전환 localStorage 저장, 1600 `994px 340px` · 1280 · 375 전부 가로 오버플로 0(표는 자기 안에서 스크롤), 다크 토큰 정상
  - 남은 것: 로그인 세션이 없어 **실데이터 화면은 육안 미확인**(하네스로 계산·레이아웃만 검증). 매입 시점 환율(`fxRate`)은 컬럼만 두고 입력 UI가 없어 **환차손익 분리는 여전히 불가**
- 08-12: **랭킹 점수 중립색 + 신호등 구간 조정 + `넓게` 보기 레일 폭 420px**
  - **🔴 범위를 잘못 잡았다가 되돌린 건**: "점수 숫자가 다 초록"이라는 지적을 **화면 전체**로 읽고 5축·레이더·펀더멘털·일별 점수까지 중립으로 바꿨는데, 사용자가 말한 건 **종목 랭킹 목록의 점수**뿐이었다(스크린샷으로 확인). 나머지는 전부 롤백. 다음에 "점수 색"류 지적을 받으면 **어느 위젯인지 먼저 특정**할 것
  - **남은 변경 2가지**: ① `scoreTone()` 구간 **45 → 35**(0~35 빨강 / 35~65 주황 / 65~100 초록, 첫 요청에 명시) ② 종목 랭킹 행의 점수·등급만 `SCORE_NEUTRAL_TEXT`(`text-foreground` — 라이트 검정 / 다크 흰색)
  - 중립을 상수로 둔 이유: `text-foreground`를 그냥 적으면 다음 사람이 "색 지정 누락"으로 보고 신호등으로 되돌린다. 유일한 사용처가 랭킹 목록임을 `stock-score-api.ts` 주석에 남김
  - **`넓게` 보기 잘림 수정**: 좌측 레일 300px에 순위·아바타·이름·스파크라인·등락·점수를 다 넣으면 이름 몫이 20px대라 종목명이 **한 글자로 잘렸다**(사용자 스크린샷). 사용자 결정 — `넓게`일 때만 레일을 **420px**로 넓히고 본문에서 폭을 빌린다
  - 이를 위해 보기 밀도 상태를 패널 밖으로 꺼냈다 → **`lib/ranking-view.ts`**(localStorage + 모듈 스코프 구독자 집합 + `useSyncExternalStore`). `storage` 이벤트는 **다른 탭에서만** 발생해 같은 화면의 두 구독자(패널 ↔ 페이지)를 잇지 못한다. `PageColumns`에 `wideLead` prop 추가
  - 검증: tsc 통과, eslint 신규 파일 무경고(잔존 4건은 전부 기존 `set-state-in-effect`). **브라우저 실측** — 랭킹 90·88·A+ 본문색(라이트 `lab(2.75)` / 다크 `lab(98.26)`), 종합 73은 초록 유지(다크 `rgb(74,222,128)`), 5축 83·79·66 초록 / 52 주황, 레이더 초록으로 **롤백 확인**. 폭은 1280 `420px 766px` ↔ `300px 886px` 실시간 전환, 1600 3단 `420 734 340`, 375 단일 컬럼 — 전 구간 가로 오버플로 0이고 `넓게`에서 잘린 텍스트 **0개**(최장 "현대코퍼레이션홀딩스" 112px 완전 표시)
  - ⚠️ 마켓 레이더 지수 점수칩은 손대지 않음(신호등 유지)
- 08-12: **종목 분석 심화 — 점수 신호등 + 레일 위젯 3종 + 랭킹 보기 전환** (목업으로 선택받고 진행)
  - **진행 방식 변경(사용자 요청)**: 이제 페이지 UI 작업은 **먼저 목업/제안으로 의견을 받고** 착수한다. 이번엔 `docs/mockups/analysis-widgets-v1.html`(옵션 0~6, 라이트/다크 토글)을 만들어 선택받았다
  - **🔴 기존 버그: 점수 색이 전 화면에서 죽어 있었다** — `--success`·`--danger`가 **정의된 적이 없는데 13곳**에서 쓰였다(관리자 배치·점수 패널, 시장 브리핑, 종합 점수 전부). `var()`가 상속색으로 떨어져 성공/실패·등급이 모두 검정이었다(실측). 두 토큰을 처음 정의하고, 점수용으로는 **신호등 3색 신설**(`--score-high/mid/low` = 초록·주황·빨강, 사용자 결정)
  - **tone 맵을 `lib/stock-score-api.ts`로 단일화**(`SCORE_TONE_TEXT/BG/BORDER/BADGE`) — 같은 맵이 5개 파일에 복사돼 있었고 그중 `mid`만 파랑(`--brand`)이라 화면마다 중간 점수 색이 달랐다. ⚠️ 점수 색은 **점수를 말하는 자리에만** 쓴다(등락률 칸은 profit/loss가 주인 — 빨강이 두 뜻이 되면 안 된다)
  - **우측 레일 3종 신설**: `stock-daily-close-card`(최근 30거래일 면적차트 **+** 표 — 차트는 흐름, 표는 정확한 값. 면적 색은 마지막 하루가 아니라 **구간 전체 방향**으로 정한다), `stock-schedule-card`(실적·공시 타임라인, DART/Finnhub 배지, **예정 건을 위로**), `stock-insider-card`(최근 90일 매수/매도/순증 + 최근 5건)
  - **점수 5축을 막대 → 레이더로 교체**(사용자 결정 — 막대는 AI 분석 탭에 이미 있다). `score-radar-chart.tsx`는 recharts 없이 SVG 직접. **값 없는 축을 0으로 그리지 않는다** — 커버리지 낮은 종목에서 0은 "가치 점수 바닥"이라는 없는 사실이 된다(삼성전자 심리·수급이 실제로 `—`)
  - **랭킹 패널 개편**: `간단`(기본·종목+점수) ↔ `넓게`(심볼·가격·스파크라인·등락) **보기 전환**(localStorage 저장, `useSyncExternalStore`로 하이드레이션 불일치·이펙트 setState 둘 다 회피) + **정렬 헤더**(종목·등락·점수) + **등락 강도 배경**(±2% 미만은 거의 안 칠함)
  - **점수 랭킹 API는 국내 종목에 `changeRate`·`closePrice`를 주지 않는다**(실측) → 넓게 보기일 때만 `fetchQuotesBatchClient`로 보강. 안 그러면 등락 칸이 전부 `—`이고 등락 정렬이 무의미
  - **메달은 기본 정렬(점수 내림차순)에서만** 표시 — 등락순 목록에 금메달이 3번째 줄이 아닌 곳에 박히면 순위표처럼 잘못 읽힌다
  - 검증: tsc·`next build` 통과, eslint **신규 6파일 무경고**(잔존 9건은 전부 기존 파일). 대비 실측 — 라이트 초록 5.02·주황 5.18·빨강 6.47 / 다크 10.29·7.92·6.48 (전부 AA 통과). 브라우저 실측 — 레일 7카드 실데이터(삼성전자 73점 A·동종 상위 3%·커버리지 90%, 30거래일 -20.31%, DART 공시 2건, 내부자 매수 4·매도 26·순증 -18,093주), 넓게 보기에서 시세 채워짐(씨젠 32,350원 +5.20%)·히트셀 12·스파크라인 17, 1600/375 오버플로 0
- 08-11: **🔴 dev 서버 CSS 캐시로 UI가 깨져 보이던 문제 + 홈 대시보드 재구성**
  - **사용자 신고**: "색감·선이 너무 많이 바뀌어 가시성이 나빠졌다". **원인은 디자인이 아니라 dev 서버였다** — 사용자가 띄워둔 3000번 서버(PID 36808)가 shadcn 토큰 마이그레이션 **이전부터** 떠 있어 옛 스타일시트를 계속 내보내고 있었다(served CSS에 `--card: oklch` 없음, `--brand` 없음, 옛 4테마 팔레트 `#0b0f17`/`#000`/`#f1f5f9` 잔존)
  - 그 결과 새 컴포넌트의 shadcn 유틸이 전부 무효로 해석됨 — `bg-card` → **투명**(카드 구분 소멸), `border` → **rgb(15,23,42) 거의 검정**(선이 전부 진함), `text-muted-foreground` → **본문과 동일**(위계 소멸), `bg-accent`·`bg-muted` → 투명. 신고 내용과 정확히 일치. **디스크의 `globals.css`는 무결**이었다
  - 조치: 서버 kill → `.next/dev`·`.next/cache` 삭제 → 재기동. **이 사고는 08-05에 이어 두 번째** — 메모리(`css_ui_regression_caution`)에 "served CSS를 fetch해 새 토큰 존재 여부로 판별" 절차를 추가
  - **대비 보정(실측 기반)**: 텍스트 대비는 원래 정상이었으나(본문 19.8) 구조 분리가 약했다 — `--border` 0.922는 흰 카드 위 **1.26**이라 사실상 안 보임 → **0.87(1.48)**, 다크는 `1 0 0 / 10%` → **16%(1.63)**. `--muted-foreground` 0.556은 4.74인데 이 앱 보조 텍스트가 대부분 **10~11px**이라 부족 → **0.52(5.49)**. 다크 보조텍스트 6.94
  - **홈 대시보드 재구성(사용자 요청 "알차고 예쁘게")**:
    - **`MarketHeroStrip` 신설** — 지수 12종이 8.5rem 카드로 3그룹에 깔려 **첫 화면 600px이 전부 지수**였다. 매일 보는 5종(KOSPI·KOSDAQ·USD/KRW·나스닥·S&P500)만 스파크라인+점수칩과 함께 한 줄로. 나머지는 **`AllIndicesSection`(기본 접힘)**에 그대로 — 줄인 것이지 버린 게 아니다
    - **`DashboardOverview` 확장** — `오늘의 점수 TOP`(신규, 국내/해외 토글 · 이 앱만의 자산) + `거래대금 상위`(국내/해외 토글 신규) + `오늘의 뉴스 7` 3열, 그 아래 `최신 투자 리포트 4`
    - 시장 토글은 **블록마다 따로** 둔다 — 하나로 묶으면 "국내 점수 보며 해외 거래대금 보기"가 막히는데 그게 대시보드에서 하고 싶은 일이다
    - 토글 시 로딩 되돌리기는 `setBlock(LOADING)` 대신 **본문만 `key`로 재마운트** — 이펙트 setState는 렌더를 한 번 더 돌리고 그 프레임 동안 이전 시장 데이터가 새 탭 밑에 남는다
  - 검증: tsc·`next build` 통과, eslint **신규 파일 무경고**(대시보드 잔존 3건은 기존 `asset-*`). 브라우저 실측 — 홈 전 블록 실데이터(KOSPI 6,345.53 +0.73%·오스코텍 90 A+·삼성전자 241,000원·뉴스 7건·리포트 2건), 토글 4종 실제 전환 확인(점수 국내 오스코텍 ↔ 해외 트리플플래그 / 거래대금 삼성전자 ↔ 마이크론), 전체 지수 42px↔840px, 1600/1280/375 오버플로 0, 라이트·다크 양쪽 확인
  - **실측 함정(재확인)**: `document.querySelector('main')`이 **SSR 스트리밍용 숨은 복사본**(height 0)을 먼저 반환한다. 이걸 읽고 "데이터가 안 온다"고 한 번 오진했다 — 반드시 높이/`offsetParent`로 거를 것. 또 `key`로 재마운트되는 카드는 DOM 노드가 교체되므로 클릭 전에 잡아둔 참조는 **detached**가 된다(토글이 안 먹는 것처럼 보임)
- 08-11: **전체 폭 후속 — 화면별 다단 재구성(본문 + 우측 레일)**
  - **문제**: 앞선 세션에서 본문이 뷰포트를 다 쓰게 바뀌었는데(`page-shell-wide`) 페이지 구성은 그대로라, 예전 중앙 컬럼용 화면들이 **가로로 늘어나기만** 했다. 종목 분석은 랭킹 패널이 화면 폭을 통째로 가로지르는 배너였다(사용자 스크린샷)
  - **해결 방침(사용자 결정)**: 본문 폭을 다시 좁히지 않는다. 남는 폭을 **같은 맥락의 관련 위젯**으로 채운다
  - **공용 프리미티브 2종 신설** — `components/layout/page-columns.tsx`(`PageColumns`: 본문 + 우측 레일, 종목 분석만 좌측 레일까지 3단 / `StatRow`), `components/ui/section-card.tsx`(`SectionCard`·`SectionEmpty`·`SectionSkeleton`·`SectionKeyValue`). 홈 대시보드가 들고 있던 사설 `SectionCard`를 여기로 승격
  - **브레이크포인트 규칙**: 레일은 `xl`(1280) 이상에서만 컬럼이 되고 그 아래에서는 본문 밑으로 쌓인다. 컬럼 정의는 `minmax(0,1fr)` — `1fr`만 쓰면 안쪽 긴 표가 컬럼을 밀어 레일이 잘린다. **3단 화면의 우측 레일은 `2xl` 미만에서 두 컬럼을 가로지르며 `grid-cols-2`로 흐르고 sticky를 끈다**(본문 아래 블록을 붙여두면 스크롤이 멈춘 것처럼 보인다)
  - **재구성한 화면 8개**:
    - **종목 분석 3단** — 좌: 종목 랭킹(배너 → 레일, `max-h-420px` 고정 해제하고 레일 높이를 채움 / 좁은 화면에서만 420px), 중: 기존 탭, 우: 신규 `stock-side-rail.tsx`(시세 스냅샷 + 52주 위치 막대 · 종합 점수 5축 · 관련 뉴스 5 · 이 종목 리포트). **탭을 옮겨도 시세·점수·뉴스는 같은 자리에 남는다**
    - **뉴스 2단** — `news-side-rail.tsx`(지수 흐름 · 오늘의 키워드 · 많이 본 뉴스 · 최신 리포트). 키워드는 **화면에 이미 있는 기사에서 센다**(서버 재요청 없음 → 집계와 목록이 어긋날 수 없음), 누르면 피드가 그 키워드로 필터되고 제목 옆에 해제 칩이 뜬다
    - **리포트 2단** — 성적표 4타일을 레일로(`research-side-rail.tsx`: 내 성적표 · 많이 다룬 종목 · 많이 본 리포트 · 채점 방식 안내). 목록은 `2xl`에서만 2단. 순위표는 여전히 없음(§15-1)
    - **관심종목** — "종목 추가" 검색 카드를 레일로 옮기고(가끔 하는 일이 매번 목록을 밀어냈다) 오늘의 요약·강세/약세·지수 추가
    - **스탬프** — 요약 4타일 + 종목 칩 벽(종목 20개면 3줄로 접혀 목록을 밀어냄)을 레일 세로 목록으로. 개봉 대기·관점 분포 추가
    - **메모장** — 검색·종목 필터를 레일로, 고정한 메모·현황 추가. 정렬은 목록 헤더로
    - **목표가** — 입력 폼을 레일로. 목록에 **현재가·목표까지 남은 %**를 추가(`fetchQuotesBatchClient`). 현재가가 없으면 `—`로 두고 0%로 그리지 않는다
    - **보유종목** — `holdings-side-rail.tsx`(총 평가금액·오늘 손익·비중 상위·많이 움직인 종목). 원/달러가 섞이므로 **환율로 원화 환산**하고, 환율을 못 받았으면 일부만 더한 값을 "총 평가금액"이라 부르지 않고 합계를 보류
  - **손대지 않은 것**: 마켓 레이더(요청 제외), **설정**(데스크탑에서 896×640 모달이라 애초에 늘어나지 않음 — 실측 확인), 종목 워크스페이스·시세(이미 터미널형 다단)
  - **기존 버그 1건 수정**: 홈 "최신 투자 리포트"의 방향 배지가 자체 맵(`long`/`short`/`neutral`)을 쓰는데 API 값은 `buy`/`sell`/`hold`라 **전부 폴백인 "관망"으로 찍히고 있었다** → 리포트 화면과 같은 `DirectionBadge`로 교체
  - **실측 중 발견해 고친 것**: 관심종목 강세/약세를 정렬 후 위 3 / 아래 3으로 자르면 **종목이 5개 이하일 때 같은 종목이 양쪽에 나오고 보합이 "약세"로 불린다** → 부호로 먼저 거른 뒤 자른다(하네스 실측으로 확인)
  - **실측 함정(신규)**: 이 브라우저 창은 `document.visibilityState === "hidden"`이라 `useMarketIndices`의 `isPageVisible()` 게이트에 걸려 지수가 영영 안 온다. `visibilityState`를 덮어쓰고 `visibilitychange`를 쏘면 그때 로드된다. 이걸 모르면 "지수 위젯이 안 뜬다"고 오진한다(앱 정상 동작)
  - 검증: tsc·`next build`(37페이지) 통과, eslint **신규 파일 10개 무경고**(수정 파일에 새 문제 0 — 남은 `set-state-in-effect`는 전부 이전부터 있던 것). **브라우저 실측** — 1600px 3단(300/854/340, 오버플로 0) · 1280px 2단 + 레일 2×2 격자 · 375px 전 화면 세로 스택 오버플로 0; 종목 분석 랭킹 클릭 → 본문·우측 레일 동시 전환(RF머트리얼즈 38,400원/89점 A+), 뉴스 키워드 "유가 — 21건" 클릭 → 피드 100 → **정확히 21건**, 목표가 실측(삼성전자 +24.5% / SK하이닉스 "도달"), 보유 레일은 로그인 게이트라 **임시 하네스로 교차통화 합계 검증**(원 2,400,000 + $1,000×환율 = 3,817,710원, 일간손익 -1,266원, 비중 62.9/37.1) 후 하네스 삭제
  - 남은 것: 로그인 세션이 없어 **관심종목·보유종목·스탬프·메모장의 실데이터 화면은 육안 미확인**(레일 자체는 하네스/빈 상태로 확인). 관리자 7화면은 이번 범위 밖
- 08-11: **UI 후속 — 사이드 이펙트 수정 + 전체 폭 전환 + 홈 종합 대시보드**
  - **사이드 이펙트 3건 수정**: ① **라이트 모드에서 카드가 배경과 구분되지 않음** — shadcn 기본값은 `--background`도 `--card`도 순백이라 화면이 흰 판 하나가 됐다(실측). 페이지 배경만 `oklch(0.97)`(#F5F5F5)로 한 단 내림 ② `StatCard`가 자체 `p-3 sm:p-4`를 유지해 `Card`(p-2.5)와 안쪽 여백이 달라 보임 → 같은 값으로 통일 ③ 라벨·수치 글자 크기도 촘촘 밀도에 맞춤
  - **🔴 전체 폭 전환(사용자 결정)**: 모바일 반응형이 따로 있으므로 데스크탑에서 중앙 컬럼만 쓰던 것을 그만둔다. `PageScrollShell`/`PageHeader` 기본 아케타입 `reading` → **`wide`**. 거터는 `.page-shell-wide`가 `padding-inline: .75rem`로 가지고 **`PageHeader`의 `px-3` 특례는 제거**(거터가 두 겹이면 헤더/본문 경계가 다시 어긋난다). 직접 `page-shell`을 쓰던 admin 3개·news·settings도 교체. **실측 1280px에서 본문 714 → 1050px**
  - `reading`은 **긴 산문 화면에만 명시적으로** 남김 — 리포트 상세·리포트 작성. 이 둘은 `PageHeader`에도 같은 값을 넘겨야 컬럼이 맞는다
  - **홈 종합 대시보드 신설**(`components/dashboard/dashboard-overview.tsx`): 지수 바 아래에 거래대금 상위 8 + 오늘의 뉴스 6(2단) + 최신 투자 리포트 4(카드). **블록마다 독립적으로 실패**하도록 각자 fetch·로딩·빈 상태를 가진다 — 하나가 죽어도 화면 전체가 비지 않는다. 랭킹 응답에 종목명이 없어 `fetchQuotesBatchClient`로 이름을 채운다(코드만 보이면 훑을 수 없다)
  - 검증: tsc·`next build`(37페이지)·eslint 통과. **브라우저 실측** — 홈에서 3블록 모두 실데이터 렌더(삼성전자·SK하이닉스 등 8종목, 실제 뉴스 6건, 리포트 2건), 라이트/다크 배경-카드 단차 확인, 마켓 레이더 제목 x=232 ↔ 본문 x=232 정렬, 1280/375 오버플로 0
  - 남은 것: 컴포넌트별 자체 패딩(`p-4` 등)이 아직 여럿 남아 밀도가 완전히 수렴하지 않았다. raw `<button>` 182곳·`<input>` 48곳 이관도 그대로
- 08-11: **버그 수정 — 종합 점수 배치가 8/7 고정으로 산출되던 문제(종가 적재 스케줄러의 "놓친 세션" 영구 스킵)**
  - **증상**: 08-11 00:25 KST 정기 실행된 `worker-scores`가 `2026-08-07 — 17902 rows`로 산출(로그 확인) — 8/10(월)이 이미 지난 거래일인데도 그 이전 날짜로 고정
  - **원인**: `ScoreService.LatestTradeDate()`(`stock_score.go`)는 `stock_daily_closes`의 `MAX(trade_date)`를 그대로 씀. 그 테이블을 채우는 `DailyCloseService.RunMarketScheduler`/`nextTrigger`(`daily_close.go`)가 **항상 `now` 이후의 다음 트리거만 계산**해서, KRX 마감+20분(15:50 KST) 창이 이미 지난 뒤에 컨테이너가 재기동되면 그 세션은 **영원히 스킵**됨 — 8/10 세션이 정확히 이 경우였음(전 세션이 `docker compose up --build`로 8/10 17:50 KST에 재기동, 즉 그날 마감창을 이미 지난 뒤). `candle_bars`엔 8/10 1d 봉이 정상 존재(별도 폴러가 채움) — 프로젝션 단계만 누락. 이 프로젝트는 **BE 변경마다 Docker 재기동이 관행**(`AGENTS.md`)이라 재기동 타이밍이 마감 직후에 걸릴 확률이 낮지 않음
  - **즉시 복구**: `go run ./cmd/backfill-daily-closes`로 8/10 세션을 `candle_bars`→`stock_daily_closes` 프로젝션(외부 API 호출 없음, 355행), `go run ./cmd/compute-scores`로 8/10 기준 재산출(16,215 scored, healthy) + `batch_job_runs`에 수동 enqueue해 이미 떠 있는 `worker-scores` 컨테이너가 지수 재계산까지 수행. KR 지수(kospi/kosdaq)는 별도 폴러 경로라 이번 버그 영향 없이 이미 8/10 기준이었음(확인함); US 지수는 8/10 미국장이 아직 안 끝나 8/7 유지가 정상
  - **근본 수정**(`daily_close.go`): `RunMarketScheduler`가 루프 진입 전 `catchUpMissedSessions`를 한 번 실행 — 각 스케줄의 **가장 최근에 이미 지난 마감+지연 시각**을 역방향 탐색(`lastCompletedTrigger`, `nextTrigger`의 대칭)해 그 세션을 즉시 재적재. 멱등 UPSERT + 외부 호출 0이라 매 기동마다 무조건 실행해도 안전(이미 최신이면 그냥 조용히 끝남). 신규 유닛 3건(`daily_close_schedule_test.go` — 재기동 갭 재현, 주말 스킵, 정상 케이스)
  - 검증: go build/vet/test 전체 통과, Docker 8컨테이너 재기동(Tesseract OK) — 이번 재기동 자체가 버그를 유발했던 것과 같은 타이밍이었는데 로그에 `daily close: krx 2026-08-10 — ...`(도메스틱)·`daily close: us_equity 2026-08-07 — ...`(오버시즈, 정상 — 미국장 미마감)로 캐치업이 실제로 작동함을 실측 확인
  - `worker-scores` 자신의 00:25 KST 일일 스케줄도 같은 종류의 "놓친 창은 다음날까지 대기" 구조이나, 그쪽은 다음날 정기 실행 시 `LatestTradeDate()`를 그때그때 새로 읽어 자연 치유되므로(수동 큐도 있음) 이번엔 손대지 않음 — 영구 스킵 위험이 있는 종가 적재 스케줄러만 수정
- 08-11: **UI/UX 설계 인터뷰(D1~D18) + shadcn 2단계 구현 — 화면이 실제로 바뀜**
  - **설계 확정**: `docs/UI_SPEC.md`(최종 명세 17절, 사용자 승인) / 근거·버린 대안은 `docs/archive/ui-spec-interview.md` / 목업 `docs/mockups/shadcn-shell-v2.html`
  - **인터뷰 중 사실관계 정정 2건**: ① "사이드바에 없는 화면 6종"은 숨긴 게 아니라 **`page-stubs` 준비 중 화면 + `ACCOUNT_FEATURES_VISIBLE=false`로 꺼둔 기능**이었음 → ⌘K 팔레트 도입 근거가 무너져 **팔레트를 넣지 않기로 뒤집음** ② 사이드바 항목은 22개가 아니라 **일반 사용자 기준 10개**
  - **주요 결정**: 좌측 사이드바 + 전체 폭 헤더(56px) / 검색은 입력창만 / 헤더 우측은 계정 드롭다운 하나 / 사이드바 48px 레일 + 접힘 저장 / **활성 표시는 전 화면 중립**(파랑은 하락을 뜻하므로 색이 튀는 곳을 숫자로 한정) / **밀도 촘촘**(행 30px·본문 12px·컨트롤 32px) / 뉴스=행·리포트=카드 / **테마 2종**(midnight 제거, 다크 #000→#252525) / 짧은 건 모달·긴 건 시트 / 모바일 표는 3열+펼치기 / 관리자 데스크탑 전용 / 등락은 **색+부호**(화살표 제거)
  - **🔴 토큰 전략 — 레거시 토큰을 shadcn 토큰의 별칭으로**: `--surface: var(--card)`, `--text-primary: var(--foreground)` 등. `var(--surface)`를 쓰는 137곳을 건드리지 않고 색만 넘어간다. 별칭 제거는 호출부 교체 후
  - **`Card`/`Badge`는 레지스트리 버전으로 교체하지 않음** — shadcn `CardHeader`가 `grid`+`px-6` 기반이라 이 앱의 `flex justify-between`와 규칙이 달라 137곳 헤더 정렬이 한 번에 어긋난다. 동작 없는 표현 컴포넌트라 토큰만 갈아입히면 결과가 같다
  - **테마 마이그레이션**: `preferences.ts`가 저장된 `midnight`/`general` → `dark`로 변환(안 하면 기존 사용자가 light로 튕김). `layout.tsx` 초기화 스크립트·설정 카드·`@custom-variant dark`·news-badge 선택자도 정리
  - 사이드바 접힘 상태를 `quant-port-sidebar-open`에 저장(예전엔 `matchMedia`로 매번 초기화돼 새로고침하면 풀렸음). **좁은 화면에서는 저장값을 적용하지 않는다** — 오버레이라 앱을 가린 채 뜬다
  - 검증: tsc·`next build`(37페이지)·eslint 통과(기존 `set-state-in-effect` 3건은 이전부터 있던 패턴), **브라우저 실측** — 헤더 56px, 사이드바 220px↔48px, `--sidebar` 배경(라이트 #FAFAFA/다크 #343434), 활성 메뉴 중립(oklch 0.93/0.269 + weight 500), 로그인/회원가입 버튼 32px·12px·radius 8px, 라이트/다크 양쪽 오버플로 0
  - **실측 함정(신규)**: `document.querySelectorAll("header")`가 2개를 반환한다 — 하나는 **React SSR 스트리밍용 `<div id="S:0" hidden>` 안의 복사본**이다. `offsetParent`로 걸러야 진짜 셸을 잰다. 이걸 모르고 숨은 복사본을 재느라 "사이드바가 안 펼쳐진다"는 오진을 한 번 했다
  - **미확인**: 로그인 세션이 없어 **계정 드롭다운(설정·로그아웃)은 실제로 열어보지 못함**. 관리자 화면·종목 워크스페이스 탭 바도 동일 이유로 육안 미확인
  - 남은 것: 3단계(사이드바 shadcn 블록화·토스트 배선), 4단계(페이지별). **앱 전역의 raw `<button>` 182곳·`<input>` 48곳은 아직 자체 클래스** — 밀도가 컴포넌트로 수렴하려면 이들을 `Button`/`Input`으로 옮겨야 함
- 08-11: **shadcn/ui 전환 1단계 — 기반만 (FE만, 화면 변화 0)**
  - **목표(사용자 결정)**: FE 전체를 **shadcn 기본 스타일로 리디자인**. 4단계로 끊어 진행하며 이번은 1단계(기반)만. 계획·충돌표·단계 범위 → `docs/SHADCN_MIGRATION.md`
  - **스택은 그대로 적합**: Next 16.2.10 + React 19 + Tailwind v4 + `@/*` + `cn()`(clsx+tailwind-merge) + lucide. shadcn CLI 4.16.2가 Next 16을 문제없이 감지했고 **`globals.css`를 건드리지 않았다**(`add` 전후 diff 0). 토큰은 전부 수작업 — CLI init에 맡기면 자기 `:root`를 뒤에 붙여 기존 `[data-theme=...]` 팔레트를 특정도 동률·순서 우위로 덮는다
  - **🔴 `--accent` 이름 충돌이 이번 작업의 핵심**: 이 앱에선 **브랜드 블루**(링크·활성), shadcn에선 **은은한 hover 표면**으로 의미가 정반대다. 그대로 두면 shadcn 컴포넌트의 hover 배경이 전부 파래진다 → 앱 쪽을 **`--brand`/`--brand-muted`로 이름 변경(65파일 273곳)** 하고 `--accent`를 shadcn에 넘김. **새 코드에서 브랜드 색은 `var(--brand)`**
  - `--background`·`--border`는 이름·의미가 같고 **값만** 달라서 1단계에서 손대지 않음(덮으면 미교체 화면 전부가 그 자리에서 바뀜) — 2단계로 이월
  - **`dark:` 변형을 `data-theme`에 연결**: `.dark` 클래스가 아니라 `<html data-theme>`이고 어두운 테마가 셋(dark·midnight·general)이라 `@custom-variant dark (&:is([data-theme="dark"], [data-theme="midnight"], [data-theme="general"], ... *))`. 컴파일된 CSS와 4테마 계산값으로 실측 확인
  - **일부러 넣지 않은 것 2개(2단계)**: ① `--radius-*` 스케일 — shadcn 전용이 아니라 **Tailwind 기본 반경을 덮어써서** 실측상 로그인 버튼 `rounded-lg`가 8px→10px로 바뀌었고 앱 전체 **341곳**(md 105·lg 194·xl 39·sm 3)이 동시에 움직인다 ② shadcn base 레이어(`* { @apply border-border }`, `body { bg-background text-foreground }`) — 색 미지정 테두리 전체와 body 글자색을 즉시 바꾼다
  - 신규 의존성 `radix-ui`·`class-variance-authority`·`tw-animate-css`. **주의**: 돌고 있던 dev 서버는 새로 설치된 패키지를 못 찾아 `Can't resolve 'tw-animate-css'`로 죽는다 — 설치 후 dev 서버 재시작 필요
  - 검증: tsc·`next build`(37페이지)·eslint 통과. **브라우저 실측** — 4테마 전 토큰 값이 작업 전 기준선과 **완전 동일**, `/login` 제출 버튼(bg `rgb(37,99,235)`·h 44px·**radius 8px**)·입력창·카드 `rounded-xl` 12px 그대로, 375/1280 오버플로 0. 임시 하네스로 shadcn Button 6변형×4테마 실측(light `--primary` lab 7.78% / dark·midnight·general lab 90.95%, `dark:bg-destructive/60`·`dark:bg-input/30` 적용 확인) 후 하네스 삭제
  - **실측 함정(문서에 기록)**: 테마를 바꾸고 `getComputedStyle`을 그냥 읽으면 안 된다 — shadcn 컴포넌트의 `transition-all` 때문에 직후 읽으면 전이 시작값(이전 테마 색)이 나오고, 기다렸다 읽으면 `PreferencesProvider`가 `data-theme`를 되돌려 놓는다. 이 둘 때문에 "dark에서 토큰이 안 먹는다"는 잘못된 결론을 한 번 냈다. 전이를 죽이고 같은 태스크에서 동기적으로 읽어야 함
  - 남은 것: 2단계(프리미티브 Card 137·Button 182·Input 48·Badge 36 + 반경/base/`--background` 넘기기) → 3단계(사이드바·⌘K·토스트, `--sidebar-width` 충돌 정리) → 4단계(페이지별). **교체하지 않는 것**: lightweight-charts 터미널·recharts·자체 마크다운 파서/에디터·손익 색
  - 기존 버그 1건 발견(미수정): `--brand-foreground`(옛 `--accent-foreground`)는 **정의된 적이 없는데 3곳에서 쓰인다**(`admin-llm-panel` 2·`stock-news-panel` 1) → 글자색이 상속으로 떨어짐. 2단계에서 shadcn Button 교체로 자연 해소
- 08-10: **리포트 사용성 2차 — 목록 필터 재구성 + 2줄 초안 + 이미지 업로드 기반 구축**
  - **목록(`/reports`)**: 좌측 탭 `전체 리포트`(기본값·왼쪽) / `내 리포트`. **상태 필터(검증 중·판정 완료·철회) 제거**, 좌측에 의견(전체/매수/매도/관망)+역발상, **우측 끝에 정렬 `최신`/`조회 순`/`추천 순`**(`ml-auto`, 좁은 화면에선 아래 줄로 내려가되 계속 우측 정렬). 정렬에서 알파순·판정 임박이 빠졌고 조회/추천은 동점 시 최신순 폴백. 상태 배지는 카드에 그대로
  - **초안 2줄 + 클릭하면 사라짐**: `buildResearchDraft`가 방향별 안내 2줄만 반환(HTML 주석 → 평문). `MarkdownEditor`에 `clearOnFirstEdit` prop 신설 — 값이 그 안내 그대로일 때만 muted로 보이고 **첫 클릭/포커스에 지워진다**(글자수도 0). 손대지 않은 안내는 발행 시 빈 본문으로 치환돼 **절대 게시되지 않는다**. 기존 긴 자동 데이터는 `buildResearchDataBlock` → `preview.dataMd`로 분리해 **`발행 시점 데이터 넣기` 버튼**을 눌렀을 때만 삽입(LLM 0콜 유지)
  - **이미지 업로드 — 리포트 전용이 아니라 앱 공통 경로로 설계**(`docs/RESEARCH_CALL_DESIGN.md` §18): `be/internal/media`의 **`Store` 인터페이스 + 첫 구현 `LocalStore`**, `MediaService`(정책), `MediaHandler`(`POST /api/v1/media/images` 인증 / `GET /api/v1/media/*` 공개). **키 = 내용 sha256**(`ab/cd/<hash>.png`) → 같은 파일 중복 저장 없음 + 불변이라 `immutable` 캐시 가능. **타입은 서버 스니핑으로 결정**(클라이언트가 주장한 Content-Type·파일명은 근거 아님), **SVG 거부**(스크립트 담는 문서), 서빙에 `nosniff` + `CSP default-src 'none'; sandbox`, **키 정규식 검증이 경로 탈출 방지**. 객체 스토리지로 옮길 때 바꾸는 건 `Store` 구현 하나 + `MEDIA_PUBLIC_BASE_URL`
  - **Docker: `quant_port_media` named volume 신설** — import temp와 달리 영구 보관이라 writable layer에 두면 재기동마다 사라진다. Dockerfile에서 미디어 디렉터리를 미리 만들어 소유권(appuser)이 볼륨에 복사되게 함
  - **FE**: `lib/media-api.ts`(`uploadImageClient` — research 아닌 공통), 에디터에 사진 버튼 + **붙여넣기·끌어놓기** 업로드, 업로드 중 스피너·실패 메시지, 업로드는 prop으로 주입 가능(`uploadImage`). 마크다운 파서/렌더러에 이미지 노드 추가 — `src`는 링크와 같은 화이트리스트(http(s)/mailto/내부경로), **`javascript:`·`data:`는 이미지가 아니라 alt 텍스트로 렌더**(실측 확인). 업로드 중 타이핑해도 삽입이 기존 본문을 덮어쓰지 않게 `latest` ref 사용
  - 검증: go build/vet/test 전체(**신규 유닛 15건** — media store 5·media handler 6·draft 3 + 기존), tsc/`next build`, eslint 신규 파일 무경고, Docker 8컨테이너 재기동(Tesseract OK, `media store: /var/lib/quant-port/media (max 5 MB)` 로그 확인). **브라우저 실측**: 목록 버튼 배치·기본값(전체 리포트/전체 의견/최신)·정렬 우측 정렬(1280 flush right, 375 두 번째 줄) 및 375px 오버플로 0; 임시 하네스 페이지로 에디터 실측(안내 2줄 muted·클릭 시 즉시 삭제·타이핑 후 **이미지 붙여넣기 → `![차트캡처](/api/v1/media/...)` 삽입** → 미리보기 `<img loading=lazy>` 렌더, 위험 src 2종 평문화) 후 하네스 삭제; API 실측은 비로그인 401·경로 탈출 404·잘못된 키 404
  - **미확인**: 세션이 만료됐고 **계정 생성·비밀번호 입력은 하지 않는 정책**이라, 로그인 상태의 실제 업로드(멀티파트 → 저장 → 서빙)와 작성 화면의 `발행 시점 데이터 넣기` 버튼은 브라우저로 못 봄. 대신 **핸들러 레벨 테스트**(201 + 저장 왕복 + 헤더 + 415/413/400)로 고정
  - 남은 것: 참조 해제된 이미지 GC 없음(발행 안 한 초안 업로드는 남음), 사용자별 업로드 쿼터 없음, 리사이즈·썸네일 없음
- 08-10: **리서치 → 리포트 개편 — 리더보드 삭제(보상제도 미도입) + 마크다운 블로그형 UI**
  - **명칭(사용자 결정)**: 메뉴 `리서치` → **`리포트`**, 객체 `리서치 콜` → **`투자 리포트`**. FE 경로도 `/research/*` → **`/reports/*`**(`/reports`·`/reports/new`·`/reports/[id]`·`/reports/authors/[userId]`). **API·테이블·코드 식별자는 `research` 그대로** — `/api/v1/research/*`, `research_calls`, `components/research/*`, `lib/research-*.ts`. 화면에서 "리포트"를 찾을 땐 코드에서 `research`로 검색할 것
  - **리더보드 완전 삭제 + 보상제도 도입 안 함**: FE `/research/leaderboard`·`research-leaderboard-client.tsx`, BE `GET /research/leaderboard`·`ResearchService.Leaderboard`·`research_leaderboard.go`(연율화·페널티·기간/시장 필터)·`ResearchLeaderboardEntry`/`ResearchLeaderboard` 모델·`ResearchRepo.ResolvedForLeaderboard`·리더보드 유닛 11건 제거. **남긴 것**: 작성자 성적표(`AuthorProfile` → 신규 `service/research_authors.go`로 이동)와 개인 요약(적중률·평균 알파·철회율). 이유는 코드 주석·설계문서에 남김 — **줄 세우기는 리포트를 대회 참가작으로 만들고, 대회는 보상 없이 유지되지 않는다**(유사투자자문 규제선). 유지된 테스트 2건(`researchRatable`·`ViewerKey`)은 `research_social_test.go`로 이동
  - **마크다운 파서/렌더러 자체 구현(의존성 0)**: `lib/markdown.ts`(블록: 제목·문단·목록(중첩)·인용·코드펜스·구분선·GFM 표 / 인라인: 굵게·기울임·취소선·코드·링크) + `components/ui/markdown.tsx`. **`dangerouslySetInnerHTML`를 쓰지 않는다** — 리포트 본문은 공개된 사용자 작성물이라 HTML을 주입할 수 있는 렌더러는 그 자체로 저장형 XSS 경로. 링크는 `http(s)`/`mailto`/내부경로만 허용하고 나머지(`javascript:` 등)는 **앵커 대신 평문**으로 렌더(브라우저 실측 확인). **HTML 주석은 파싱 전에 제거** — 서버 초안의 `<!-- 왜 그렇게 보는지 -->` 안내가 발행 후 독자에게 보이지 않게 하려는 것
  - **에디터**(`components/ui/markdown-editor.tsx`): 툴바 10종(제목·굵게·기울임·목록·번호목록·인용·링크·코드·구분선·표)이 선택 영역을 감싸거나 줄 앞에 붙이고, `작성`/`미리보기` 탭의 미리보기는 **상세 화면과 같은 렌더러**를 쓴다(렌더러 2개가 갈라지지 않게). 좁은 화면에서 툴바는 줄바꿈이 아니라 가로 스크롤
  - **레이아웃을 한 컬럼 중앙 정렬로 통일**: 08-09에 compose/detail을 `wide`(2단)로 뒀던 것을 **`reading` 1단으로 전환**. 상세는 제목 → 요약 카드(기준가/목표가/손절가/기간/확신도 + 알파·수익률) → 본문 → 발행 시점 근거 → 반응 순으로 모든 폭에서 동일 순서. 목록은 블로그 인덱스형 카드(제목·발췌·읽는 시간·조회/좋아요/댓글). 상세의 글 제목은 `h2` — `PageHeader`가 이미 페이지의 `h1`
  - **서버 초안도 블로그 골격으로**(`research_draft.go`): `## 한 줄 요약` / `## 투자 논거` / `## 무엇이 확인되면 틀린 것인가` / `## 발행 시점 데이터(자동 생성)`. LLM 0콜 유지
  - 관리자 화면 라벨도 동기화: 배치 카드 `투자 리포트 판정`, DB 관리 카탈로그 `투자 리포트`/`리포트 댓글`… (테이블명은 `research_*` 유지)
  - 검증: go build/vet/test 전체 통과, tsc/`next build`(라우트가 `/reports*`만 남고 `/research*` 소멸), eslint 신규 3파일 무경고(기존 `set-state-in-effect` 4건은 이전부터 있던 패턴), Docker 8컨테이너 재기동(Tesseract OK). **브라우저 실측**: 사이드바 `리포트 > 투자 리포트` 단일 항목·리더보드 없음, `/research`·`/research/leaderboard` 404, `/reports/leaderboard`도 404(동적 세그먼트가 숫자가 아니면 notFound), 상세 렌더, 작성 화면 초안 1,157자 자동 주입, 미리보기에서 표 정렬(left/right/center)·중첩 목록·코드블록·인용·취소선 렌더 + `javascript:` 링크 평문화 확인, 375px에서 목록·상세·작성 모두 가로 오버플로 0
  - 남은 것: 종목 화면 `이 종목 리포트 N건` 섹션(API는 있음), 팔로우·`@handle`, 자동 썸네일
- 08-10: **리서치 콜 — 스탬프 결합 해제 + Phase 2~3(리더보드·소셜) 구현** ※ 리더보드는 위 08-10 개편으로 제거됨
  - **스탬프와 분리 확정(사용자 결정)**: 두 기능은 연동하지 않는다. ① 스탬프 상세의 `콜로 발행` 버튼 제거 ② **리서치가 스냅샷 타입을 직접 소유**(`ResearchContext`/`ResearchNews`/`ResearchIndex`/`ResearchChart`/`ResearchCurrent`/`ResearchReturn`, FE도 동일) — 다른 기능이 바뀌었다는 이유로 바뀔 수 있는 스냅샷은 박제된 게 아니다 ③ 공용이던 시계열 원시 연산을 중립 파일 `service/close_series.go`로 추출(`closeSeries`/`closePctChange`/`resolveHorizonClose`/`benchmarkForStock`/`marketTradeDate`) — 스탬프 파일 안에 있으면 리서치가 스탬프 위에 얹힌 것처럼 보인다. **주의**: `pctChange`는 `xbrl_select.go`에 이미 있어(반환값 1개) `closePctChange`로 명명
  - **038 소셜 테이블**: `research_reactions`(좋아요·북마크, kind enum), `research_comments`(1단계 대댓글, soft delete), `research_ratings`(3축), `research_views`(조회 중복제거). **카운트를 `research_calls`에 비정규화하지 않음** — 행과 어긋날 수 있는 카운터가 쿼리 하나보다 나쁘다. 피드는 페이지 크기와 무관하게 고정 쿼리 수(`CountsFor`/`MyReactions`)
  - **리더보드**(`research_leaderboard.go`): 순위 키 = **연율화 알파 × 페널티**. 연율화는 ±200%p 클램프(30일 +40%를 12배 하면 지수를 재는 것), 30일 미만은 연율화 안 함. 페널티 2종 — 월 8건 초과 발행, 동일 종목 반복(1건당 3%p), 하한 0.5. **판정 10건 미만은 순위에서 빼고 '관찰 중'으로 분리**. 철회 콜은 성과 평균에서 빠지되 발행 수·철회율엔 남는다
  - **평가는 판정 전에만**, 자기 콜 평가 금지(409). 결과를 보고 매기면 논거 평가가 아니라 성과 재측정이고 그건 알파가 이미 더 잘한다
  - **조회수는 상세 조회의 부수효과가 아니라 별도 엔드포인트**(`POST /research/calls/{id}/view`). 상세 fetch에 얹으면 리렌더·폴링마다 올라 렌더 카운터가 된다. 뷰어 키 = 로그인 user id 해시 / 비로그인 IP+UA SHA-256, **원문 미저장**. 30분 중복 제거
  - **FE**: `/research/leaderboard`(기간 3개월·12개월·전체 × 시장 탭, 내 순위 고정 표시), `/research/authors/[userId]`, 상세 화면 반응 바·댓글 스레드·3축 평가 패널, 피드 카드 조회/좋아요/댓글/평가 카운트. 사이드바 `리서치 > 리더보드` 추가
  - 검증: go build/vet/test 전체(신규 유닛 14건 — 리더보드 표본 게이트·연율화·클램프·페널티 2종·철회 처리·알파 정렬·캘리브레이션·뷰어 키 해시), tsc/`next build`, Docker 8컨테이너 재기동(Tesseract OK, 038 적용). **브라우저 실측**: 좋아요 토글 on/off, 북마크, 댓글+대댓글, **대댓글의 대댓글이 조부모로 평탄화**, 자기 콜 평가 409, 2번째 계정으로 3축 평가(3.7/5) 렌더, 조회 3회 호출에도 카운트 1 유지, 375px 오버플로 0. **리더보드 실측**: 픽스처 24건으로 "수익률 +60%/알파 +1%p" 작성자가 "수익률 +6%/알파 +18%p" 작성자에게 밀리는 것(0.73 vs 18.00) 확인 후 픽스처 삭제
  - 남은 것: 팔로우·`@handle`, 자동 썸네일, 종목 화면 `이 종목 콜 N건` 섹션(API는 있음), 시즌제 대회
- 08-09: **리서치 콜 Phase 1 구현** (`docs/RESEARCH_CALL_DESIGN.md` §16)
  - **범위 확정**: 설계안 §5.4 "계좌 연동 기반 실행 일치도"는 **사용자 결정으로 제외**. 콜 어디에도 보유·계좌 정보가 들어가지 않는다(차별점 8개 → 7개)
  - **BE**: `037_research_calls.sql`(`research_calls`/`research_revisions`/`research_reports`, `MigrateResearchIfNeeded`) + `ResearchRepo`/`ResearchService`/`ResearchHandler`. 공개 읽기는 `OptionalAuth`(`GET /research/calls*`), 쓰기는 `RequireUserID`. **DELETE 엔드포인트를 두지 않음** — 지울 수 있는 콜은 적중률을 반증 불가능하게 만든다. 철회는 `status='retracted'`로 남고 철회율에 반영
  - **불변/가변 분리**: 방향·기준가·목표가·손절가·기간·확신도·근거 스냅샷은 발행 후 UPDATE 금지. `UpdateBody`는 title/body/tags/visibility만 쓰고, 본문이 실제로 바뀔 때만 `body_rev`를 올리며 `research_revisions`에 append(가시성만 바꿔도 "수정 3회"가 되지 않게). **실측 검증**: PATCH에 `direction`/`targetPrice`/`basePrice`/`conviction`/`horizonDays`를 실어 보내도 전부 무시되고 rev만 2로 증가
  - **근거 수집은 전적으로 서버 책임**(스탬프 capture 패턴 재사용): 시세·5축 점수·백분위·커버리지·PER/PBR·52주·뉴스 5건·공시(캐시 전용)·지수·`stock_briefing.go` 룰 기반 브리핑. **LLM 0콜** — 자동 초안도 브리핑에서 생성. 온디맨드 LLM을 기본 경로에 넣지 않는다(08-09 앞선 세션과 동일 원칙)
  - **판정 엔진**(`research_performance.go`): `target_hit`/`stop_hit`(선도달 우선)/`expired_hit`/`expired_miss`/**`expired_unknown`**(만기는 지났으나 판정할 종가 없음 — 적중률 분모 제외). **종가만 사용** — 장중 고가 터치를 인정하면 유동성 낮은 종목의 순간 스파이크가 목표 달성이 된다. 데드라인 이후 종가는 목표 달성으로 치지 않음(늦게 들어온 백필이 만기를 늘리지 못하게)
  - **목표 도달 확률**(§5.5)은 `candle_bars` 일봉의 겹치는 전 구간으로 실측. `stock_daily_closes`는 커버리지가 얕아(005930 27행) 못 씀. 겹치는 구간 30개 미만이면 확률 대신 `insufficient` — "모른다"를 0%로 표시하는 게 가장 위험
  - **가드**: 일 5건 / 동일 종목 30일(409 + 남은 일수), 모순된 목표·손절 쌍 400 거부(기대값·손익비 부호가 뒤집힘), 공개 발행 시총 하한(KRW 300억 / USD 3천만, **시총 미상이면 차단하지 않음** — 못 잰 종목에 "너무 작다"고 하는 건 거짓), 신고 + 관리자 숨김을 소셜 기능보다 먼저
  - **배치**: `BatchResearchVerdict` + `cmd/worker-research`(신규 6번째 워커, compose·Dockerfile·`restart.ps1` 등록). **07:00 KST** — KRX 마감+20분·US 마감+20분(당일 05:20~06:20 KST) 둘 다 지난 시각. DB 전용이라 외부 호출 0
  - **FE**: 사이드바 신규 그룹 `리서치`(공개물이라 사적 기록 그룹과 분리), `/research`(내 콜·공개 피드 토글 + 성적표 4타일), `/research/new`(콜 값 ↔ 에디터 2단, 실측 지표 가격대 칩으로 목표가 스냅), `/research/[id]`. **원수익률 정렬 옵션을 두지 않음** — 알파순이 기본
  - **레이아웃 함정**: compose/detail은 2단이라 `reading`(뷰포트 68%)에서 1280px일 때 본문 입력칸이 **236px**까지 눌렸다 → 헤더·본문 모두 `wide`로 전환(640px 확보). 375px 가로 오버플로 0
  - 검증: go build/vet/test 전체(신규 유닛 24건 — 판정 9·요약 4·도달확률 4·EV 2·경계/게이트 5), tsc/`next build`, Docker 7컨테이너 재기동(Tesseract OK), **브라우저 실측**(테스트 계정으로 000660 실제 발행 → 근거 62/53점·1,938개 구간 실측 확률 28.3%·EV +6.93%·손익비 1.23:1 렌더 확인, 409 중복 가드, PATCH 불변성), **판정 배치 실측**(백데이트 픽스처로 `expired_miss` -17.65% / 알파 -3.88%p 산출 후 픽스처 삭제)
  - 남은 것: 리더보드·좋아요/댓글·평가 점수·팔로우는 Phase 2~3. **관리자 자격증명이 없어 `/admin/batch`의 "리서치 콜 판정" 카드와 관리자 숨김 API는 육안 미확인**(워커 큐 경로는 실제 enqueue→claim→done으로 확인)
- 08-09: **리서치 콜 설계안 작성** (`docs/RESEARCH_CALL_DESIGN.md`, 코드 변경 0)
  - Valley AI `투자 인사이트/예측대회`를 벤치마크로 받아 **우리 데이터로만 가능한 형태**로 재설계. 핵심은 "작성자가 쓴 숫자"가 아니라 **서버가 박제한 근거**(5축 점수·백분위·커버리지·시세·뉴스·공시)를 콜 옆에 나란히 두는 것
  - 차별점 8개: ① 근거 카드 서버 박제 ② **원수익률이 아니라 알파로 순위**(벤치마크 = `index_daily_closes`) ③ 콜 값 불변 + 본문 수정 이력 공개 + 삭제해도 통계 잔존(체리피킹 차단) ④ 계좌 연동 기반 **실행 일치도**(비중 구간만, 금액 비노출) ⑤ `stock_briefing.go` 룰 기반 **자동 초안**(LLM 0콜)으로 작성 마찰 제거 ⑥ 목표가 입력 시 `candle_bars` 실측 **도달 확률** 표시 ⑦ 확신도 캘리브레이션(Brier) 채점 ⑧ **역발상 콜 적중률 → `stock_scores` 예측력 검증 루프**로 환류
  - 재사용 전제: 스탬프의 불변/가변 분리·거래일 경계·알파 계산(`stamp_performance.go`)을 확장, `batch_job_runs` 큐 + 전용 워커(`worker-research`)를 종가 워커 뒤에 연결. **외부 API 호출 0**
  - LLM 예산: 기본 경로 전부 LLM 0콜(썸네일도 SVG 자동 생성). 온디맨드 LLM을 기본 경로에 넣지 않는다 — 08-09 앞선 세션에서 같은 이유로 온디맨드 요약을 제거했음
  - **명칭 충돌 주의**: `service/insights.go`(대시보드 인사이트)가 `insights`를 점유 → 신규는 `research_*` 접두어
  - 규제 제약을 설계에 선반영: 보상 구조 없이 시작(유사투자자문업), 서비스가 특정 콜을 추천하지 않음, 소형주 펌핑 방지용 종목 게이트·신고·관리자 숨김을 **Phase 1에 포함**
  - 로드맵 Phase 1(소셜 없이 검증 가능한 콜) → 2(피드·리더보드) → 3(커뮤니티·실행 일치도). **미결정 7건은 문서 §17** — 사용자 확인 후 구현 착수
- 08-09: **AI 요약 배치 수동 실행 — 휴장일에도 종가 기준으로 돌 수 있게 수정**
  - **문제**: `runMarketBatch`(`stock_summary_batch.go`)의 "오늘이 거래일인지" 체크가 스케줄 잡·수동 트리거(`RunBatchNow`) 양쪽에 그대로 걸려 있어, 관리자가 휴장일(주말·공휴일)에 배치 관리에서 수동 실행을 눌러도 `"OO 시장은 YYYY-MM-DD에 휴장입니다"` 에러로 항상 거부됐음. 대상 선정 자체는 마켓 레이더가 저장해둔 최신 랭킹(=휴장일엔 직전 거래일 데이터)을 읽을 뿐이라 실행을 막을 이유가 없었음
  - **수정**: `batchRunWindow`에 `asOf time.Time` 필드 추가(zero면 기존처럼 실행 시점 `time.Now()`로 거래일 판단 — 스케줄 경로는 무변경). `RunBatchNow`가 트리거 시점에 오늘이 휴장이면 `calendar.LastSessionCloseAt(schedule, now)`(기존 함수 재사용, 종가 기준 "가장 최근 완료된 세션")로 직전 거래일을 구해 `window.asOf`에 넣고, `runMarketBatch`는 `asOf`가 채워져 있으면 휴장일 가드를 건너뛰고 그 날짜를 `tradeDate`로 사용
  - 신규 유닛 1건(`TestRunMarketBatchManualUsesLastCloseOnHoliday` — 2026-08-15 광복절/토요일에 `asOf`가 2026-08-14로 정확히 해석되고 휴장 가드 대신 다음 체크(LLM 비활성화)까지 통과하는지 고정), go build/vet/test 전체 통과, Docker 재기동(Tesseract OK, 기동 로그에 llm/batch 에러 없음). **관리자 자격증명이 없어 배치 관리 화면에서 실제 휴장일 수동 클릭은 미확인** — 코드 경로는 유닛 테스트로 고정
- 08-09: **AI 요약 배치(스케줄 잡)를 배치 관리(/admin/batch)에서 수동 실행 가능하게 연동**
  - **동기**: AI 요약 배치는 `stockSummarySvc.Start(bgCtx)`로 API 프로세스 안에서만 스케줄 트리거(KR 08:00 KST / US 08:30 ET)로 돌고 있었고, `/admin/llm`의 "수동 생성"은 임의 종목 최대 10개만 즉석 재생성하는 별개 경로였음 — "오늘 스케줄이 실패했으니 지금 전체를 다시 돌리고 싶다" 같은 요청을 처리할 manual trigger가 배치 관리 쪽엔 없었음
  - **BE**: `model.BatchLLMSummaryDomestic`/`BatchLLMSummaryOverseas` 신규 잡 ID 2개(`batch_job.go`) 등록, `batchJobCatalog`(`batch_admin.go`)에 카드 추가(Cost=`rate_limited`, since/limit 미노출 — 항상 오늘 레이더 top 대상). `runMarketBatch`(`stock_summary_batch.go`)를 `(*model.LLMBatchRun, error)` 반환으로 바꾸고 신규 `RunBatchNow(ctx, market)`이 스케줄 창 대신 관대한 수동 창(2시간)으로 즉시 호출 — "오늘 이미 실행됨" 가드를 우회하는 게 의도(관리자가 명시적으로 다시 돌리는 것). `beginMarketRun`/`endMarketRun` mutex 가드 신설 — 스케줄 잡과 수동 트리거가 같은 시장에서 동시에 돌면 같은 Redis run/history 키·예산 체크가 레이스 나던 걸 방지
  - **LLM 배치엔 전용 워커 프로세스가 없음**(다른 5개 배치와 다른 점): `StockSummaryService`가 fundamentals/news/insider/dart/edgar/LLM client 등 API 프로세스의 거의 전체 의존성 그래프를 필요로 해서 분리 비용이 큼 — 대신 `RunBatchQueueWorker`(기존 `batch_job_runs` 큐 폴러 추상화, 원래 `cmd/worker-*`용)를 API 프로세스 안에서 두 잡 ID에 대해 goroutine으로 그대로 돌림(`server.go`). Trigger→enqueue→Catalog 조회 경로는 다른 5개 배치와 완전히 동일한 코드
  - 신규 유닛 5건(스케줄 조회, 동시 실행 가드, 미지 시장 거부, 카탈로그 등록, nil 서비스 커버리지) + go build/vet/test 전체 통과, Docker 재기동(Tesseract OK). `/admin/batch/jobs` 401(비로그인) 확인. **관리자 자격증명이 없어 실제 클릭 실행은 미확인** — 이 세션 로컬 컨테이너엔 `LLM_*` env가 비어 있어 `llm.Enabled()`가 false이므로, 관리자가 실제 로그인해 실행하면 즉시 quota 소모 없이 "LLM이 비활성화되어 있습니다" 오류로 안전하게 끝나는 경로부터 확인될 것
- 08-09: **AI 요약을 마켓 레이더 1일 top100(파생상품 제외) 자동 배치로 전환 + LLM 토큰 한도 이중 가드**
  - **동기**: 파일럿 2종목(005930·000660)만 LLM을 돌리고 있었음. 실측 결과 Gemini 무료 티어 일일 500콜 중 하루 11~49건만 사용 중이라 400건 가까이 놀고 있었고, 대상만 정하면 수백 종목이 들어감
  - **대상 선정**(`llm_batch_targets.go`): 레이더가 이미 15분마다 저장하는 **1일 기준 거래대금+거래량 top 100**(KR/US 4슬롯)의 합집합에서 ETF·ETN·ETP 제외. **외부 API 호출 0** — Redis/MySQL에 저장된 payload만 읽음. 실측(08-09): KR 155→**105종목**(파생 50 제외), US 171→**136종목**(파생 35 제외) = **하루 241콜**. `model.IsFundamentalsApplicable`은 ETF/ETN만 알아서 **미국 ETP 11건이 통과**하므로 재사용하지 않고 별도 판정 함수를 둠
  - **🔴 토큰 한도 가드 신설**: 무료 티어는 요청 수뿐 아니라 **분/일 토큰**도 제한하는데 기존 `apibudget`엔 TPD가 아예 없었고 TPM은 **사후 차단**(이미 쓴 양이 한도에 닿아야 거부)이라 큰 호출 하나가 그대로 넘길 수 있었음
    - `Limits.TokensPerDay` + `tokens_day` 카운터 신설, `RecordTokens`가 분·일 양쪽 적립
    - **`CheckTokens(estimate)` 사전 차단**: 프롬프트에서 토큰을 추정(`llm.EstimateTokens` — 한글 1자=1토큰, 라틴 3자=1토큰으로 **과대** 추정)해 **넘길 호출 자체를 거부**
    - 응답에 `usage.total_tokens`가 없으면 **추정치를 대신 기록** — 미계측 호출이 조용히 한도를 넘지 않도록
    - 전 한도를 env로 재정의 가능(`LLM_DAILY_QUOTA`/`LLM_PER_MINUTE`/`LLM_TOKENS_PER_MINUTE`/`LLM_TOKENS_PER_DAY`). **TPD 기본 1M은 공표값이 아니라 자체 상한** — provider 콘솔 값으로 맞출 것
  - **스케줄을 시장별 개장 전 마감으로 전환**(`stock_summary_schedule.go`): 기존 00:00 KST 일괄 → **KR은 08:00 KST**(09:00 개장 1시간 전), **US는 08:30 ET**(09:30 개장 1시간 전 = 서머타임 21:30 KST / 표준시 22:30 KST). 데드라인에서 `대상수 × 6초 + 10분`을 **역산해 시작**, 최대 2시간 전으로 클램프. **휴장일엔 실행하지 않음**(`calendar.IsTradingDay`, 내장 휴장일 JSON). 데드라인을 시장 현지시각으로 표현해 미국 서머타임이 자동으로 따라감
  - **예산 가드 + 이월**(`stock_summary_batch.go`): 시작 전 `Headroom()`에서 예비분(요청 80·토큰 12만 — 번역/브리핑/수동 생성 몫)을 뺀 만큼으로 대상을 **미리 자름**(잘린 종목은 호출 자체가 없음). 실행 중에도 매 종목 전에 데드라인·잔여 한도를 확인해 **첫 위반에서 중단하고 남은 종목을 다음 실행으로 이월**. 간격은 관측 토큰 EMA로 TPM의 60% 이내가 되도록 자동 연장(현재는 6초 하한이 지배)
  - **🔴 온디맨드 LLM 호출 제거**: 기존엔 캐시가 6시간 지난 파일럿 종목을 **사용자가 조회하기만 해도** LLM을 다시 호출했음 — 241종목으로 확대하면 조회 트래픽만으로 하루 최대 4배. LLM 소비 경로를 **배치 + 관리자 수동** 2개로 한정. 대신 규칙 기반 재빌드가 **직전 LLM 서술을 그대로 승계**(수치는 최신, 문장은 유지)하도록 해 요약이 몇 시간 뒤 규칙 문장으로 되돌아가던 문제도 같이 제거. 뉴스 감성 집계도 함께 승계(LLM만 산출하는 값이라 빠뜨리면 감성 배지·일별 점수의 뉴스 축이 사라짐). 새 `analyzedAt` 필드가 문장이 실제로 쓰인 시각을 보존
  - **설정**(`036_llm_batch_settings.sql`): `target_mode`(radar|manual)·`daily_cap`(250)·`quota_reserve`·`token_reserve` 추가, `pilot_symbols`는 **"항상 포함되는 고정 종목"**으로 의미 전환(TEXT, 상한 10→50). `/admin/llm`에 시장별 다음 실행·개장 전 마감·현재 대상 수·제외 사유·최근 실행 결과·잔여 한도 표시
  - **마이그레이션 함정**: `splitSQLStatements`가 `;`로 단순 분할해서 **SQL 주석 안의 세미콜론이 문을 쪼갬**(기동 실패로 발견). 036 주석에 경고 명시
  - 검증: go build/vet/test 전체(신규 유닛 20건 — 스케줄 6·선정 5·토큰 추정 5·예산 6), Docker 재기동(Tesseract OK, 036 적용), **실제 DB/Redis로 선정 결과 실측(KR 105 / US 136)**, 신규 admin 라우트 401, tsc/next build 통과. `/admin/llm` 육안 확인은 관리자 자격증명이 없어 미완료
- 08-09: **전 메뉴 페이지 헤더 통일** (FE만, BE 무변경)
  - **점수 관리·배치 관리 실제 버그**: `PageHeader`가 `PageScrollShell` **안쪽**에 중첩돼 있었음 — `PageScrollShell`의 `page-shell`(거터) + `py-4 sm:py-6`이 `PageHeader` 자신의 `page-shell` + `py-3 sm:py-4` 위에 또 씌워져 다른 관리자 메뉴보다 헤더가 눈에 띄게 크고 더 들여써짐. `PageHeader`를 `PageScrollShell`보다 **앞의 형제**로 이동(다른 모든 페이지와 동일 패턴)해서 해결
  - **마켓 레이더는 사용자가 재검토 후 `wide`→`reading` 통일 선택**: 앞선 세션에서 `wide` 아케타입으로 헤더·본문 폭 불일치는 고쳤지만, 화면(1706px)에서 여전히 `reading` 메뉴들과 좌측 여백·헤더 높이가 달라 보여 "안 똑같다"는 피드백. `PageHeader` `width` prop 제거(기본값 `reading`) + 본문 `page-shell-wide` → `page-shell`로 교체해 다른 모든 메뉴와 완전히 같은 컬럼 규칙 사용
  - **폭이 좁아진 만큼 랭킹 리스트 접기 기능 추가**: `reading`은 좁은 화면에서 `wide`보다 먼저 줄어들어 리스트+차트 동시 표시 공간이 부족해질 수 있음 — 리스트 컬럼과 워크스페이스 사이에 접기/펼치기 토글 버튼(`listOpen` state) 신설, 접으면 차트가 전체 폭 사용. `/quotes`·`/stocks/[symbol]`은 이번엔 미변경(마켓 레이더만 사용자가 지목) — 동일 리스트+워크스페이스 패턴이라 후속으로 동일 처리 필요할 수 있음
  - 검증: tsc 통과, 브라우저 실측(리스트 접기/펼치기 토글 정상 동작, 헤더가 `page-shell` 컬럼 사용 확인). 관리자 메뉴(점수·배치)는 로그인 자격증명 없어 육안 미확인 — 구조는 다른 정상 관리자 페이지와 동일 패턴으로 일치시킴
- 08-09: **레이아웃 아케타입 도입 — 마켓 레이더 중앙 정렬 통일** (FE만, BE 무변경)
  - **문제의 실제 원인**: "마켓 레이더만 전체 화면"이 아니라 **헤더와 본문의 컬럼 폭 불일치**였음. `PageHeader`는 `news-page-shell`(중앙 68%)을 쓰는데 `/market`의 `<main>`만 full-bleed → 1440px에서 제목 컬럼 x=415 w=830, 리스트 x=220 w=1220으로 어긋나 그 페이지만 겉돌아 보였음
  - **설계**: 폭을 하나로 맞추지 않고 **중앙 정렬 규칙·거터를 공유하고 max-width만 페이지 성격별로 선택**하는 아케타입 2종. `reading`(`.page-shell`, 기존 값 그대로) / `wide`(`.page-shell-wide`, **`--content-max-wide: 100%`**). `PageHeader`에 `width` prop 추가 — **본문과 같은 값을 넘기지 않으면 위 불일치가 그대로 재발**하므로 타입(`PageShellWidth`)과 헬퍼(`pageShellClass`)를 `page-content-shell.tsx`에 두고 양쪽이 공유
  - **최종 결론: 폭은 통일하지 않는다.** 상한을 112rem → 72rem으로 두 번 시도했으나 전부 되돌림
    - 112rem: 뷰포트 약 2012px 이상에서만 잘려 사용자 화면(1908px)에서 **변화가 0**("똑같은데?")
    - 72rem: 1908px에서 컬럼 1152px·좌우 268px로 `reading`(1148px)과 4px 차까지 일치시켰으나, 워크스페이스가 **1318 → 830**으로 줄어 **사용자가 보기 불편하다고 판단해 되돌림**. 터미널형은 밀도가 곧 기능
    - 남은 것은 "헤더 컬럼과 본문 컬럼이 같은 규칙을 쓴다"는 것까지. 다시 상한을 넣으려면 반드시 `min(<값>, 100%)` 형태 — `100%` 항이 없으면 `AppShell`의 `overflow-x-auto`가 좁은 화면에서 가로 스크롤을 만듦
  - **부수로 만들었던 버그 1건 수정**: `PageHeader`의 `wide` 분기에 `--content-gutter`(4vw)를 줬더니 1908px에서 제목이 76px 들여써져(x=296) 리스트(x=221)와 **새로 어긋남**. 터미널 패널과 같은 `px-3`으로 교체(제목 x=233 ↔ 첫 컨트롤 x=236)
  - `.page-shell-wide`에 좌우 경계선 없음 — 상한이 100%라 항상 꽉 차서 좌측선이 사이드바 테두리와 겹쳐 이중선이 되고 우측은 뷰포트 가장자리에 붙음
  - 적용: `/market`, `/quotes`(지수 바 포함), `/stocks/[symbol]`. `.news-page-shell` → `.page-shell` 리네임(뉴스 전용이 아니었음, 호출부 5개 + 헤더/스크롤셸)
  - 실측(최종, 1024/1440/1908): 헤더·본문 컬럼 x/w 완전 일치하며 **콘텐츠 폭 전체 사용**(1908에서 x=220 w=1688), 리스트/워크스페이스 320/472 · 320/888 · 368/1308, 전 구간 가로 오버플로 0
  - **되돌리기 중 발견해 함께 정리**: `market/page.tsx`가 `page-shell`(= reading 컬럼 830px)로 바뀌어 있어 `page-shell-wide`로 교정. 같이 들어와 있던 랭킹 리스트 접기 토글(`listOpen` + `ChevronLeft/Right` + 12px 세로 스트립)은 "reading으로 좁아진 폭 보완"이 목적이라 주석에 적혀 있었고 그 전제가 사라져 **사용자 요청으로 삭제**(state·import·JSX 전부, 잔여 참조 0). 워크스페이스 1308 → 1320 회복
  - **미확인**: 로그인 필요 데이터라 차트·호가 2단 분할이 실제로 그려진 상태는 못 봄(`시세 로딩 중…`에서 멈춤). `StockChartTerminal` 미변경 + 최종 폭이 원래대로라 회귀 가능성은 낮음
  - 검증: tsc, `next build` 통과. eslint는 `/quotes`의 기존 `react-hooks/set-state-in-effect` 4건만(이번 변경과 무관, className만 수정)
- 08-09: **스탬프 기능 신설 — 순간 스냅샷 + 가설 + 자동 채점** (`docs/STAMP_DESIGN.md`)
  - 사용자 요청(타임캡슐형 스탬프)을 그대로 만들지 않고 **가설(관점·확신도·검증기간)** 을 붙여 재방문 동기를 만드는 형태로 설계 후 승인받고 구현. 보존만 하면 두 번째 사용이 안 일어나고, 개봉·판정이 있어야 적중률·평균 알파가 쌓임
  - **BE**: `035_stamps.sql`(`MigrateStampsIfNeeded`, journal과 동일 idempotent 패턴) + `StampRepo`/`StampService`/`StampHandler`. `GET/POST /stamps`, `GET /stamps/{id}`, `PATCH`(가설만), `POST /stamps/{id}/open`, `DELETE`, `GET /stamps/summary`, `GET /stamps/state?symbol=` — 전부 `authctx.RequireUserID` + `WHERE id=? AND user_id=?`
  - **불변/가변 분리**: 시세·점수 5축·뉴스·공시·지수·차트 120봉은 생성 시 1회 복사 후 UPDATE 금지(수정 가능한 건 메모·태그·관점·검증기간뿐 — 스냅샷이 수정되면 적중률이 사후 조작 가능). 수익률/알파/판정은 조회 시 계산
  - **공시는 캐시 전용 복사**: 공시엔 이력 테이블이 없어(`GetTodayDisclosures` = Redis 당일 캐시) 나중에 복원 불가 → 생성 시 복사하고, 없으면 `dataGaps:["disclosures"]`로 남김(지어내지 않음). 스탬프 때문에 늘어나는 외부 API 호출 0
  - **수익률 경계 규칙**: "목표일 이하 마지막 종가"만 쓰면 **금요일 스탬프의 1D가 영원히 빈칸**(목표일=토요일→스탬프 당일 종가). 스탬프 이후 첫 종가를 쓰되 **목표일+5일 이내**로 제한(거래 끊긴 종목의 몇 달 뒤 종가가 1D를 채우는 것 방지). 미도래 구간은 `null`→`—`, 0% 대체 금지
  - **버그 2건(실측에서 발견)**: ① 장 마감 후 라이브 시세 거래량이 0이라 실제 거래된 세션이 "거래량 —"으로 박제됨 → `stock_daily_closes.volume` 폴백 ② 기존 `repository.nullFloat`이 `*v <= 0`을 NULL로 바꿔(보유수량용 헬퍼) **하락 종목 등락률이 통째로 유실**(000660 -1.94% → NULL) → `nullFloat64`로 교체, 재검증(035420 -6.64% 정상 저장)
  - **FE**: 사이드바 `기록 > 스탬프`(`/stamps`), 종목 헤더 관심 버튼 옆 도장 버튼(`StampButton`, 워크스페이스·레거시 상세 두 곳). **모달 먼저 띄우지 않음** — 클릭=즉시 저장 후 확인 패널에서 관점/기간/메모를 인라인으로 덧붙임(마찰이 사용률을 좌우). 5분 쿨다운은 서버 409(`retryAfterSeconds`)가 진실, 버튼 카운트다운은 보조. `/stamps`는 요약 4타일(총계·개봉대기·적중률·평균알파) + 종목/관점/판정 필터 + 타임라인/종목별 뷰, 행 확장 시 당시 vs 현재 2열·점수 5축 당시→현재·박제 차트(실선)+이후 종가(점선)·기간별 수익률(벤치마크/알파)·당시 뉴스/시장/AI 요약·dataGaps 명시
  - 검증: go build/vet/test(신규 유닛 15건 — 구간 경계·주말 브리지·gap 상한·알파·판정·정규화·US 거래일), tsc/next build, Docker 재기동(035 적용, Tesseract OK), 브라우저 실측(005930·000660·AAPL·035420 실제 스탬프 생성 — 점수 62/53/41, 뉴스 5건, 지수 5종, KST 시각, US는 `trade_date=NY 날짜`+`sp500` 벤치마크 확인, 409 쿨다운, PATCH 가설 반영, 375px 오버플로 0)
  - 남은 것: 개봉 알림은 사이드바/목록 배지까지만(푸시 인프라 없음), 자동 스탬프 제안·스탬프 2개 비교 뷰·메모장/목표가 연계는 미구현
- 08-09: **랭킹 크라운 → 금/은/동 메달 전환 + "AI 투자일지" → "메모장" 개편(백엔드 신규)**
  - **메달**: `score-ranking-panel.tsx` — `Crown` → `Medal`(lucide) 아이콘 교체, 1/2/3위 색상을 실제 금(#FFB300)/은(#9CA3AF)/동(#B36A3C)으로 지정. `getComputedStyle`로 렌더 색상 실측 확인
  - **메모장(기존 localStorage-only "AI 투자일지" 대체)**: 사용자에게 저장 방식(백엔드 DB vs localStorage 유지) + 핵심 기능(종목 연동/수정/검색·정렬/고정)을 먼저 확인 후 진행 — 전부 채택. 신규 `journal_notes` 테이블(`034_journal_notes.sql`, `MigrateJournalNotesIfNeeded` 패턴은 `batch_job_runs`와 동일하게 idempotent) + `JournalNoteRepo`/`JournalService`/`JournalHandler`, `GET/POST/PUT/DELETE /api/v1/journal/notes`(전부 `authctx.RequireUserID` + `WHERE id=? AND user_id=?` 소유권 검증). DB 관리 카탈로그(`db_admin_catalog.go`)에 `journal_notes` 행 추가
  - **FE**: 종목 연동은 기존 `StockCatalogSearchField` 재사용(신규 컴포넌트 안 만듦), 연동 종목 클릭 시 `analysisSymbolHref`로 AI 분석 탭 이동(랭킹 패널과 동일 패턴). 고정(pin)·수정은 인라인, 검색/필터는 로드된 전체 목록에 대해 클라이언트 사이드(개인 메모 규모상 서버 페이지네이션 불필요 판단). 사이드바 라벨 "AI 투자일지"→"메모장"(`BookOpen`→`NotebookPen`), `page-stubs.tsx`/`DESIGN.md`/`FE_ACCOUNT_FEATURES.md`의 잔여 구 명칭도 동기화
  - 검증: go build/vet, tsc/next build 통과. Docker 재기동(`034` 마이그레이션 무오류 적용 확인) 후 회원가입 테스트 계정(`journaltest0809@example.com`)으로 브라우저 실측 — 메모 생성(종목 연동 포함)·고정 토글·수정·검색 필터링·새로고침 후 데이터 유지 전부 확인. 삭제는 `window.confirm`이 자동화 도구에서 자동 취소되어 버튼 클릭 자체는 미확인이나, `DELETE` 엔드포인트를 직접 fetch로 호출해 정상 동작(200, 목록에서 제거) 확인
- 08-09: **종목 분석 탭에 국내/해외 점수 랭킹 패널 신설**
  - 사용자 요청: 종목 분석 탭이 항상 (기존 로직상 보유 비중 1위인) 임의 종목으로 시작해 둘러보기가 안 됨. 스크린샷(랭킹 리스트: 크라운·보유뱃지·추적일·시뮬수익률·점수)을 참고해 우리 서비스 스타일로 국내/해외 점수 랭킹을 붙여달라는 요청
  - **BE**: `GET /stocks/scores/top?market=domestic|overseas&limit=N` 신규(`handler/score.go`, `service/stock_score.go` `TopScores`, `repository/stock_scores.go` `TopScores`). `stock_scores`에서 `LatestScoredDate()`(= MAX(trade_date))로 오늘 배치 유무와 무관하게 "가장 최근 산출된" 날짜를 자동 선택 — 오늘 배치가 아직 안 돌았으면 자연히 어제 점수로 폴백됨(요청사항 그대로, 별도 분기 불필요). `stocks.asset_type`으로 국내(`domestic`)/해외(`us`+`etf`) 분리, `stock_daily_closes` LEFT JOIN으로 등락률·종가 동봉
  - **화면에 없는 필드는 지어내지 않음**: 스크린샷의 "거래 추적일"·"시뮬레이션 수익률"은 이 서비스에 대응 개념이 없어 제외 — 대신 실제 보유하는 데이터(등락률·종가·점수·등급)로 구성
  - **FE**: `ScoreRankingPanel`(신규, `components/analysis/score-ranking-panel.tsx`) — 국내/해외 토글 + 순위 1~3위 크라운, 보유/관심 뱃지(기존 market-radar 패턴 재사용, `use-score-rankings.ts`가 watchlist/holdings로 enrich), 행 클릭 시 `analysisSymbolHref`로 해당 종목 AI 분석 탭 이동. `AnalysisClient` 상단에 접기/펼치기 가능한 섹션으로 삽입(기본 펼침)
  - 검증: go build/vet, tsc, `next build`(prod 빌드 통과 확인 — 이 프로젝트는 `next build`에 eslint를 태우지 않음, 별도 `npm run lint`는 기존 코드에도 이미 있던 `react-hooks/set-state-in-effect` 다수 위반이 있어 이번 변경과 무관함을 `git stash`로 대조 확인). Docker 재기동 후 `/stocks/scores/top` 실측(국내/해외 각각 실제 순위·점수 확인, tradeDate=2026-08-07로 정상 폴백). 로그인 자격증명이 없어 회원가입으로 테스트 계정 생성 후 브라우저 실측: 랭킹 20건 렌더·국내→해외 토글 전환·행 클릭 시 해당 종목 AI 분석 이동·375px 모바일 가로 스크롤 없음 전부 확인
- 08-08: **전 서비스 시간 표기 KST 감사 — 백엔드 6곳·프론트 20여곳 실제 버그 수정**
  - **전제 확인**: 컨테이너 실측 `date` → UTC(`docker exec quant-port-api date` = UTC, KST와 9시간차). 이 세션 앞서 candle_bars 시간대 버그(§08-08 위 항목)의 원인이었던 바로 그 조건이 다른 곳에도 잠복해 있었음
  - **백엔드 실제 버그 6건**(전부 `time.Now()`/`now.Location()`을 KST 미변환 상태로 표시·큐키에 사용):
    1. `portfolio_history.go` `GetHistory` — `downsampleSnapshots(..., now.Location())`가 **컨테이너 로컬(UTC)** 을 라벨 타임존으로 사용 → 포트폴리오 자산 차트의 "15:04" 라벨이 실제로는 UTC 시각(KST−9h)이었음. `model.KST`로 수정 + 회귀 테스트 2건(`TestDownsampleSnapshotsLabelsAreKST`)
    2. `dart.go` `TodayDisclosures`/`FetchEarningsDisclosures` — "오늘" 공시 조회 기준일이 UTC 기준이라 KST 자정 전후 9시간 구간에서 DART에 잘못된 날짜를 요청할 수 있었음. `time.Now().In(model.KST)`로 수정
    3. `earnings.go` `FetchCalendar` — 동일 패턴(미국 실적 캘린더 조회 시작일)
    4. `market_data_store.go` — `GetTodayDisclosures`/`EnsureTodayDisclosures`/`refreshDisclosure`/`refreshInsightsData`의 Redis "오늘" 캐시키 4곳, DART 쪽 수정과 짝을 이뤄야 캐시 롤오버 시점이 실제 KST 자정과 맞음
    5. `market_index_store_batch.go` `eodKey` — 지수 EOD 1회 적재 완료 마커가 UTC 날짜로 키잉되어 있어 같은 KST 거래일에 중복 실행되거나 하루를 건너뛸 수 있었음
  - **프론트엔드**: `toLocaleString`/`toLocaleDateString`/`toLocaleTimeString` 호출 20여 곳에 전부 `timeZone: "Asia/Seoul"` 명시 추가(브라우저·SSR 로컬 zone 의존 제거). `format.ts`에 `formatDateTimeKST`/`formatTimeKST` 공용 헬퍼 신설, 관리자 패널 4곳의 중복 `formatDateTime` 로컬 구현을 이걸로 통일
  - **차트 크로스헤어 버그**: `chart-time.ts`의 `formatChartCrosshairTime`이 축 라벨(`formatAdaptiveChartAxisTime`, 이미 KST)과 달리 `date.getHours()` 등 **로컬 getter**를 써서 브라우저가 KST가 아니면 크로스헤어 시각이 축과 어긋날 수 있었음. `kstDateParts` 공용 헬퍼로 통일, `parseChartCandleDate`/`parseChartCandleEpoch`의 "오늘" 계산도 동일하게 수정
  - 검증: go build/vet/test 전체(신규 회귀 유닛 2건), tsc/next build, Docker 재기동 6개 컨테이너 정상. 의도적으로 남겨둔 것: `news_store_us.go`의 `time.Now().UTC()`는 Finnhub 조회 범위 파라미터로 저자가 명시적으로 UTC를 선택한 코드라 표시용이 아니라고 판단해 미변경
- 08-08: **배치 잡을 독립 워커 프로세스로 분리 + 국내/해외 일별 종가 완전 독립화**
  - **동기**: 관리자 수동 배치가 API 프로세스 안 고루틴으로 돌아 하나가 자원을 과점/hang 하면 API 자체가 영향받을 수 있었음. 국내/해외는 트리거 시각만 분리돼 있었고(§08-08 앞선 세션 확인) 실제 실행은 같은 프로세스·같은 루프 안에 있었음
  - **아키텍처 전환**: `batch_job_runs`를 큐로 전환(033 마이그레이션, `params JSON` + `queued` 상태). API 프로세스는 이제 **enqueue만** 하고(`BatchAdminService.Trigger`), 실제 실행은 전부 제거 — 각 잡 전용 워커 프로세스가 `ClaimNext`(`UPDATE...ORDER BY...LIMIT 1`, MySQL 원자적)로 폴링해 가져감(4초 주기)
  - **잡 5개 → 컨테이너 5개**: `worker-close-domestic`(KRX 마감+20분+평일 환율), `worker-close-overseas`(US 마감+20분+상시 BTC), `worker-scores`(00:25 KST, DB전용), `worker-index`(수동, Toss+FRED), `worker-fetch`(수동, fetch-stale/missing 2개 잡을 뮤텍스로 직렬화해 Toss 동시 호출 방지). 이미지는 `be/Dockerfile` 하나에 바이너리 6개(api+worker 5개)를 함께 빌드, `docker-compose.yml`에서 `command:`로만 분기
  - **국내/해외 완전 독립**: `IngestStockCloses`/`BackfillStocksFromCandleBars`에 `assetTypes` 필터 추가(`stocks.asset_type` 조인) — 기존엔 스케줄 트리거만 분리돼 있고 실제 SQL은 시장 구분 없이 전 종목을 대상으로 했음. 이제 domestic 워커는 `asset_type='domestic'`만, overseas 워커는 `IN ('us','etf')`만 건드려 서로 절대 겹치지 않음(테스트로 고정: `TestEveryAssetTypeAssignedToOneMarket`)
  - **배포 직후 크래시 발견+수정**: `worker-close-overseas`가 기동 즉시 패닉(`missing Location in call to Time.In`). 원인은 `calendar.ScheduleAlways`(BTC 등 24시간 자산)가 `loc: nil`로 정의돼 있었는데, 기존 코드는 이 스케줄로 `SessionCloseAt`을 호출한 적이 없어(진짜 "마감 트리거"로 쓰인 적이 없어서) 드러나지 않았던 잠재 버그. 신규 워커가 처음으로 이 경로를 태워 노출됨. `DAILY_CLOSE_DESIGN.md` §5.1("24/7 자산은 KST 자정 기준")에 맞춰 `loc=서울, closeHour=23:59`로 수정 + 회귀 테스트 추가
  - 실측: 큐에 직접 INSERT → 4초 내 워커가 claim → 실행 → done 기록까지 전 과정 확인(국내/해외/점수 3개 잡 개별 검증). 6개 컨테이너 전부 안정 기동
  - 검증: go build/vet/test 전체(신규 유닛 6건 — assetType 필터 분리·파라미터 검증·캘린더 회귀), Docker Compose 6개 서비스 기동, DB 직접 enqueue로 워커 클레임 실측
- 08-08: **배치 관리 메뉴 + 지수 카드 점수 + 전 종목 백필 완주**
  - **배치 관리**(`032_batch_job_runs.sql`, `batch_admin.go`, `/admin/batch`): 5개 잡(일별 종가 투영 / 종목 점수 / 지수 종가·점수 / 누락 세션 채우기 / 미보유 종목 채우기)을 목록·수동 실행·커버리지·최근 실행 기록으로 관리. **DB 전용 vs 외부 API 호출**을 배지로 구분해 한도 소모 여부를 실행 전에 알 수 있게 함
    - **동시 1건 제한(409)**: 조회 잡들은 앱과 Toss 한도를 공유하므로 병렬 실행은 빨라지는 게 아니라 앱 폴링을 굶김
    - 실행 기록을 DB에 영속화(`batch_job_runs`) — 기존엔 로그 한 줄뿐이라 "돌았는지"를 사후에 알 수 없었음. 재시작으로 `running`에 남은 행은 기동 시 정리(`MarkStaleRunsFailed`)하지 않으면 이후 실행이 영구 차단됨
    - FE는 실행 중일 때만 4초 폴링(끝나면 중단)
  - **지수 카드 점수**: 신규 `GET /indices/scores`로 12개 지수 점수·요약을 **1회 조회**(카드당 1회면 개요 화면이 이유 없이 느려짐). 대시보드 카드에 점수·등급 칩 + 요약 툴팁. 실측: 다우 61(B+), 코스닥 58(B), 코스피 50(B)
  - **전 종목 백필 완주**: 커버리지 **1,469 → 16,225/17,901**(436,199행). 점수도 재계산해 **16,215종목 산출**(평균 50.0, 표준편차 12.6, 분포 정상). 잔여 1,676종목은 Toss가 `400`을 주는 미국 우선주 티커라 재실행으로 해결 불가
  - `batch_job_runs`도 DB 관리 카탈로그에 추가(read-only — 감사 기록이라 수정은 이력 위조)
- 08-08: **룰 기반 시장 분석 + 탭별 점수 + 지수 점수 + 통화 표기 + DB 카탈로그**
  - **룰 기반 시장 분석**(`stock_briefing.go`, LLM 미사용): AI 요약 앞에 붙는 prefix. ① 투자 포인트 최대 4개(종합점수 → 수익성 → 밸류에이션 → 실적 → 주가 → 내부자 순 우선순위, 톤 색상) ② **상승/하락 시나리오 + 주요 가격대**. 가격대는 전부 **실제 지표에서 유래**(SMA20/50/200·최근 스윙 고저·52주 범위) — 목표가를 지어내지 않음. 현재가 ±0.2% 이내는 실행 불가라 제외, 가까운 순 최대 3개. 실측(005930): 상승 50일선 258,260(+9.9%)·200일선 261,002·스윙고점 267,000 / 하락 20일선 234,200(-0.34%)·스윙저점 189,200·52주최저 165,700. **LLM 미설정·파일럿 외 종목에서도 항상 제공**
  - **탭별 점수**: AI 분석 탭에 종합 점수 상세(축별 막대·동종 백분위·커버리지, 커버리지 60% 미만이면 수축 사실 명시), 나머지 탭은 해당 축만 노출(차트=추세, 밸류에이션=가치, 재무제표=수익성, 실적=성장, 뉴스·내부자=심리). 점수는 워크스페이스 상위에서 1회 fetch해 공유
  - **지수 점수·요약**(`031_index_scores.sql`, `index_backfill.go`, `index_score.go`): 7/1부터 **9개 지수 241행** 백필 + 전 지수 채점. 소스별 **날짜 있는** 이력만 사용 — Toss 지표캔들(kospi/kosdaq), FRED `SeriesHistory`(신규 추가, nasdaq/nasdaq100/sp500/dow/vix), `stock_daily_closes`(nq100=QQQ, sox=SOXX는 실제 ETF라 API 호출 0). **스파크라인은 날짜가 없어 사용 불가** — 날짜를 추정하면 전 행이 어긋남. 지수는 재무제표가 없으므로 기술적 단독 점수이고, 성격이 다른 12개(주가·FX·암호화폐·금)는 서로 피어가 아니라 **횡단면 순위를 쓰지 않음**. btc·gold는 날짜 이력 소스가 없어 이후 일별 배치로만 누적. 실측: KOSPI 50점 + "최근 5거래일 -5.1%, 20일선 아래, RSI 42.2" 자동 요약. `ScoreService.RunScheduler`에 편승해 1일 1회
  - **통화 표기**(`formatFinancialAmount`): 재무 차트 Tooltip·축에 포매터가 없어 `240000000`이 그대로 노출되던 것 수정(국내 조/억/만원, 해외 B/M/K). 기존 `formatTradingValue`는 `<= 0`을 "—"로 렌더해 **영업적자가 숨겨지던 버그**가 있어 부호를 다루는 신규 함수로 분리, `earnings-tab`의 로컬 우회 래퍼 제거
  - **DB 관리 카탈로그**: 누락 8개 추가 — `stock_daily_closes`/`index_daily_closes`/`stock_scores`(단방향 투영·배치 산출물이라 read-only), `llm_settings`/`score_settings`/`score_symbol_weights`(편집 가능), 레거시 백업 2개(read-only, 수동 DROP 대기)
  - 검증: go build/vet/test(신규 유닛 9건 — 가격대가 실제 지표에서만 유래하는지 고정), tsc/next build, 지수 백필·점수 실행, API 실측
- 08-08: **candle_bars 시간대 버그 수정 + LLM 입력 76% 압축(점수 연동)**
  - **🔴 시간대 버그 수정 완료**: `UpsertBatch`가 KST `time.Time`을 넘기는데 DSN이 `loc=Local` — UTC로 도는 컨테이너에서 `D 00:00 KST`가 `D-1 15:00`으로 저장됐고, 호스트 도구(KST)는 `D 00:00`으로 저장해 **두 표기가 공존**. `dailyCloseSessionBars`의 "날짜당 마지막 봉" 규칙이 UTC 행을 골라 **`trade_date`가 하루 밀림**. 일요일(2026-08-02) 15:00 봉 존재가 결정적 증거. **`DAILY_CLOSE_DESIGN.md` §12.1의 "00:00=전일 종가 복제" 진단은 오진**이었고 정정 주석 추가
    - 수정: 일봉(`IsDailyOrCoarser`)은 KST 벽시계 **문자열**로 저장 → 프로세스 시간대와 무관하게 결정적. 1분봉은 Go 읽기 경로(`bt.In(KST)`)가 대칭이라 미변경
    - 보정: `cmd/repair-candle-tz`(드라이런 기본). 149,803행 처리, **무손실 병합** — 세션별로 거래량이 큰 쪽(완성된 봉)을 남김. 결과 일봉 673,842행 전부 00:00
    - **드라이런이 사고 2건을 막음**: ① `bar_interval IN ('1d','1w','1M')`이 MySQL 대소문자 무시 콜레이션 때문에 **`'1m'` 분봉 280만 행까지 매칭** ② 제자리 `UPDATE bar_time`은 살아있는 유니크 인덱스에 부딪혀 실패(`Duplicate entry 'GEV-1d-2026-08-05'`) → `INSERT…ON DUPLICATE KEY`로 충돌마다 명시 해소
    - 검증: 005930 `08-07 235,000(+2.17%) / 08-06 230,000(-4.96%) / 08-04 242,000(+3.64%)` 정상 복구
  - **LLM 입력 재설계**(`stock_summary_facts.go` 신설): 호출당 **10,728자 → 2,621자(76%↓)**. 원지표 구조체·분기 재무제표 4개·내부자 거래 10건·기사 요약문을 전부 제거하고 **서버가 분류한 결과만** 전달(`trend:"bearish"`, `macd:"recovering"`, `rsiState`, `earnings.trend`, `insider.direction`). 뉴스는 기존 `dedupeNewsArticles` 재사용 후 **헤드라인만**
    - **차별화 핵심 — 종합 점수를 프롬프트에 투입**: `score.axes`/`strengths`/`weaknesses`/`peerTopPercent` + 지표별 피어 백분위(`pbrPeerPct` 등). 프롬프트를 "수치 나열 금지, 축 간 **긴장 관계의 인과**를 설명하라"로 재작성. 이전엔 점수가 프롬프트에 아예 없어 **요약과 옆에 표시된 점수가 서로 다른 근거로 말하고 있었음**
    - 실측 개선: "펀더멘털의 우수함과 주가 추세 간의 괴리가 현재 변동성을 키우는 핵심 요인", "PBR 상위 2%… 시장 기대치가 이미 주가에 반영" — 인과·우선순위·동종 대비가 실제로 등장
    - 부수: 소수점 반올림(`per: 24.934903853743542` → `24.9`) — 토큰 낭비이자 허위 정밀도, `dataGaps`로 결측 영역 명시해 전반적 헤지 방지
  - 검증: go build/vet/test(신규 유닛 8건 — 분류 경계·중복 제거·페이로드 크기 상한), Docker 재기동, 000660 실제 LLM 호출 재생성
- 08-08: **종합 점수 구현(전 종목) + 관리자 점수 관리 + AI 요약 카드 재설계**
  - **종합 점수**(`docs/STOCK_SCORE_DESIGN.md` 설계 → 구현): 마이그레이션 `030_stock_scores.sql`(`stock_scores` + `score_settings` + `score_symbol_weights`). 5축(추세30/수익성25/성장20/가치15/심리10). 배치 `cmd/compute-scores` — **외부 API 호출 0회**, 17,901종목 **5초**(일봉 스트리밍 1패스, 심볼 경계에서 지표 계산해 메모리 상수)
  - **핵심 설계 3가지**: ① 모든 지표를 **피어그룹 순위 백분위**로 정규화(측정된 ROE 227,162% 같은 꼬리값 때문에 절대 임계값 불가) ② **도메인 가드** — 윈저라이즈만으로는 순위에서 이상치를 못 막음(값을 잘라도 여전히 1위). 범위 밖은 결측 처리. 음수 부채비율(자본잠식)은 반전 시 최우량으로 뒤집히므로 하한 0 ③ **커버리지 수축** `50+(raw-50)×min(1,cov/0.6)` — 재정규화만으로는 "RSI 하나로 90점"을 못 막음
  - **지표별 최소 표본 폴백**(`minMetricSamples=20`): 그룹 크기만 보면 안 됨 — 피어에 수백 종목이 있어도 ROE 보유가 2~3개면 백분위가 0/50/100으로 튐. 실제로 삼성전자 성장 축이 0점으로 나왔고, `peer → assetType:market → assetType → *` 폴백으로 해결(005930 성장 0→66)
  - **추세 축도 순위 정규화**: 절대 곡선은 구조적으로 중심이 위(RSI 50 → 68점)라 전체 평균이 53.1로 밀려 자체 분포 게이트에 걸림. 2단계(절대 곡선으로 "좋은 셋업" 판정 → 피어 내 순위)로 전환 후 **평균 49.9 / 표준편차 12.6 / 0·100점 뭉침 0 / 12,564종목 산출**
  - 관리자 `/admin/score`: 전역 가중치 + **종목별 오버라이드**(추가/수정/삭제) + 커버리지 하한·수축 지점 + 즉시 재계산(분포 검증 결과 표시). 배치가 분포 이상 시 경고
  - `stock_daily_closes.score`에 미러링(8,550행) → 기존 일별 시세 화면에 즉시 노출
  - **8월 7일 종가 누락 원인**: `-fetch-missing`이 "종가가 **하나도 없는**" 종목만 대상이라 08-06까지 있는 종목은 영구 제외됐음(삼성전자 포함 819종목). `-fetch-stale=<날짜>` 모드 신설. 커버리지 1,469 → **9,147종목**
  - AI 요약: 개요 탭 카드 재설계(제목/본문 4줄/키워드/전체 분석 보기), `keywords` 필드 BE 신설(LLM 프롬프트 + 규칙 기반 폴백), 개요 외 탭에서 좌측 레일 숨김
  - 검증: go build/vet/test(신규 유닛 15건 — 실측 꼬리값·수축 시나리오·오버라이드), tsc/eslint/next build, Docker 재기동(Tesseract OK), 관리자 엔드포인트 401 확인. **관리자 화면 육안 확인은 자격증명 부재로 미완**
- 08-08: **탭 잘림 버그 수정 + 전망 탭 제거 + AI 분석 탭 신설 + 좌측 레일 축소**
  - **탭 콘텐츠 잘림(치명)**: `scrollMain = compact || effectiveTab === "overview"` 이라 데스크톱에서 개요 외 모든 탭이 `overflow-hidden` → 넘치는 내용에 **스크롤 수단 자체가 없었음**. 실측 잘림량: 내부자 거래 **910px/1521px**, 뉴스 484px, 재무제표 165px, 밸류에이션 80px. `effectiveTab !== "chart"`(차트 터미널만 자체 스크롤 소유)로 변경해 전부 도달 가능
  - **전망 탭 삭제**: 워크스페이스/레거시 `/analysis` 양쪽 탭 목록·렌더 제거, 참조 0이 된 `outlook-tab.tsx` 파일 삭제
  - **AI 분석 탭 신설(개요 바로 우측) + 헤더 요약 배치**: 헤더 중앙 빈 공간에 한 줄 요약(`StockAISummaryLine`, truncate 1줄 고정이라 요약 길이가 헤더 높이를 흔들지 않음) + "전체 분석 →". 전체 분석은 신규 `AIAnalysisTab`(4개 섹션 + 일별 종가/점수 리스트)에 배치 — **기존 `일별 시세` 탭을 흡수·대체**. 헤더는 넓은 화면 중앙 슬롯 / 좁은 화면 자체 행 두 곳에 같은 노드를 렌더하므로 fetch는 상위(`useStockSummary`)에서 1회만 수행하고 프레젠테이션 컴포넌트로 분리(중복 요청 방지, StrictMode 이중 마운트와 동일 수준까지 실측 확인)
  - **탭별 `StockSummaryBanner` 전면 제거**: 요약이 헤더+AI 탭으로 일원화돼 중복이고, 탭마다 세로 공간을 먹어 위 잘림 문제를 키우고 있었음. 참조 0이 된 `stock-summary-banner.tsx` 삭제
  - **좌측 레일 축소**: 시세 + 52주 범위만 유지, 거래(거래량/거래대금/시가총액)·재무(PER/PBR/ROE)·PER/PBR 설명 블록 제거(기업 정보·밸류에이션 탭과 중복). 모바일 `StockMetricsGrid`도 동일 범위로 축소하고 1열 카드 + 내부 2열로 재배치. 미사용이 된 `exchangeRate` prop 및 import 정리
  - 검증: 1440/1024/375px 실측 — 탭 9개(전망 없음, AI 분석이 개요 우측), 레일 `시세`/`52주 범위`만, 헤더 AI 라인 데스크톱 중앙(x=966 w=366 h=32)·모바일 자체 행(w=315 h=31), 전 탭 `overflow-y:auto` + 가로 오버플로 0, tsc/eslint/next build 통과
- 08-08: **마켓 레이더 반응형 정비 + 일별 종가/AI점수 적재 구현** (v0.8.0 이후, 릴리스 미생성)
  - **마켓 레이더 치명적 레이아웃 버그**: `StockChartTerminal`이 1024–1279px(비compact·비xl)에서 `flex-col + overflow-hidden` 인데 호가 aside가 `shrink-0`(525px) → **차트+체결 컬럼이 0px로 압살**되어 노트북 폭에서 차트 탭이 통째로 안 보였음(실측 확인). `xl` 미만은 스크롤 가능한 단일 컬럼, `xl` 이상만 좌우 분할로 CSS만으로 분기하도록 재작성
  - **메트릭 레일 브레이크포인트 도입**(`railFrom: "lg"|"xl"`): 마켓 레이더는 랭킹 리스트가 이미 ~320px를 쓰는데 레일이 `lg`부터 떠서 1024px에서 탭 본문이 **208px**만 남았음 → 마켓 레이더만 `xl`로 미룸(실측 208→448px). 레일이 숨는 구간에는 `StockMetricsGrid`(신규)로 시세/거래/재무/52주범위를 카드 그리드로 제공 — 기존엔 모바일에서 이 수치들에 **접근 경로 자체가 없었음**
  - 그 외: 로딩 중 레일이 `hidden` 누락으로 모바일에 220px 죽은 컬럼 렌더되던 것 수정, `?symbol=` 딥링크가 모바일에서 리스트로 튕기던 것 수정(paramsKey 리셋 effect가 마운트 시에도 실행), 모바일 랭킹 행에 **랭킹 기준값(거래대금/거래량) 노출**(기존 desktop 전용) + 탭 타겟/폰트 확대, 탭바 활성 탭 자동 스크롤(reduced-motion 존중), TOP5 차트 모바일 개방, 리스트 하단 `pb-9` 죽은 여백 제거
  - **일별 종가 적재**(`docs/DAILY_CLOSE_DESIGN.md` §12): 마이그레이션 `029_daily_closes.sql`(`stock_daily_closes` + `index_daily_closes`), `score TINYINT NULL`(없으면 FE `-`) + `ai_summary TEXT NOT NULL`(없으면 `''`). 시장별 정규장 마감 +20분에 적재하는 `DailyCloseService.RunScheduler`(고정 tick 아님 — 다음 마감까지 대기)
  - **설계안 대비 변경**: 원안은 "주식 종가는 candle_bars 직독"이었으나 실측 결과 `candle_bars` 1d가 **날짜당 2행(00:00 행이 전일 종가 복제) + 주말 날짜 행**을 갖고 있어 그대로 읽으면 등락률이 `0%↔2배`로 튐(005930에서 -9.85%/0%/+3.64% 재현). `ROW_NUMBER`로 날짜당 마지막 봉만 + 주말 제외 + 중복 제거 후 `LAG`로 정규화한 **투영 테이블**로 전환(단방향·멱등, score/ai_summary는 UPDATE 제외). 수정 후 005930 `+2.05%/+0.69%/-0.34%` 정상. 기존 주말 행 378건 `PruneWeekendRows`로 삭제. **candle_bars에 중복 행을 쓰는 상위 경로는 미특정 — 차트 등 다른 소비자는 여전히 영향받을 수 있음(후속 과제)**
  - **점수는 서버 계산**(`daily_close_score.go`): RSI30%/추세25%/MACD20%/뉴스감성15%/52주위치10%, 결측 구성요소는 비중 제외 후 재정규화(0점 처리 금지), RSI는 70 부근 정점 후 하락. 유닛테스트 5건. LLM은 문장만. 실측: 000070 `score=75`
  - **적재 커버리지 미완**: 활성 종목 17,901개 중 일봉 보유 종목만 적재 가능 → 세션 종료 시점 **1,469종목/6,131행**(2026-08-03~08-07). 일봉 없는 종목은 Toss 종목당 1회 조회 필요한데 한도가 초당 5회라 `-fetch-missing` 반복 실행 필요(재개 가능, 429는 Retry-After 준수 재시도). 백그라운드 실행 중이었음
  - 검증: go build/vet/test(신규 유닛테스트 포함), Docker 재기동, `/daily-closes` API 실측(005930·AAPL·000070), 브라우저 375px/1024px/1440px 실측(오버플로 0, 차트 컬럼 높이 복구, 레일 전환, 일별 시세 탭 렌더 + 요약 펼침), tsc/eslint/next build
- 08-07: **종목 뉴스 탭 페이지네이션** (v0.8.0 이후, 릴리스 미생성)
  - `StockNewsPanel`(default variant, 워크스페이스/`/analysis` "뉴스" 탭)이 `limit={5}`로 호출되며 그 5건이 그대로 최종 표시 개수였음 — 더 볼 방법이 없어 스샷처럼 "잘린" 인상. 백엔드 offset 페이지네이션은 만들지 않음: `NewsStore.List`가 SQL LIMIT 이후에도 관련성/중복 필터를 추가로 거는 구조라 서버 페이지네이션을 넣어도 페이지당 정확한 건수를 보장 못함 — 대신 **fetch 상한을 60건으로 올리고 클라이언트에서 8건씩 페이지네이션**(prev/next + `1 … 4 5 6 … 12`식 번호, `pageWindow` 유닛 로직은 node로 별도 검증). 심볼 전환 시 페이지 1로 리셋. 워크스페이스/analysis 두 호출부의 `limit={5}` 제거
  - 관리자 로그인 페이지들과 달리 `/stocks/[symbol]`도 일반 로그인 세션이 필요해 이 환경에서 브라우저 실사용 확인은 못 함(테스트 계정 자격증명 없음) — tsc/eslint/next build 통과 + pageWindow 로직 node 스크립트 검증까지만 완료, 사용자 로그인 후 확인 필요
- 08-07: **LLM 재생성 00:00 KST 고정 + 관리자 수동 즉시 생성 + 사이드바 버그 진짜 원인 수정 + 종가 적재 설계안** (v0.8.0 이후, 릴리스 미생성)
  - **사이드바 버그 실제 원인 발견**: 이전 세션이 `item.href === "/admin"` 특수 케이스를 제거했지만, 남은 일반 규칙 `pathname.startsWith(`${item.href}/`)` 이 `/admin`에 대해 `/admin/`으로 평가돼 **모든 admin 하위 페이지에서 여전히 뉴스 관리가 활성**이었음(이전 세션의 "브라우저 캐시 문제" 결론은 오진). 각 항목 독립 판정을 버리고 **전체 nav href 중 최장 일치 1개만 활성**(`activeNavHref`)으로 변경. 실행 중인 dev 서버가 서빙하는 번들에서 함수와 `allNavHrefs`를 직접 추출·평가해 `/admin/llm → /admin/llm`, `/admin → /admin` 검증
  - **백그라운드 재생성 00:00:00 KST 고정**: `refresh_interval_hours` 노브 제거(모델/repo/handler/FE에서 제거, DB 컬럼은 NOT NULL DEFAULT라 마이그레이션 없이 미사용 방치). `nextScheduledRefresh`(KST 자정, 유닛테스트 5케이스) 기반 루프로 교체. **기동 시 즉시 스윕도 제거** — 재기동이 곧 추가 과금이 되지 않도록. 파일럿 외 종목은 종전대로 Get() 온디맨드
  - **수동 즉시 생성**(신규): `POST/GET /api/v1/admin/llm/refresh`. 파일럿 게이트를 **의도적으로 우회**(임의 종목 지정이 목적, apibudget 한도는 그대로 적용), 종목당 6초 간격 순차 실행, 종목별 pending/running/done/error + source(llm|rule) 추적. 동시 1건 제한(409). `StockSummaryService.Start(ctx)` 신설로 bg context 보관(수동 잡도 종료 시 취소됨). 요청 로그에 `manual` 플래그 추가
  - **종가 적재 설계안**(`docs/DAILY_CLOSE_DESIGN.md`, 미구현): 종목 종가 = `candle_bars` 1d **그대로**(종가 스냅샷 테이블 신설은 candle_bars와 완전 중복이라 비권장), 지수 종가 = 신규 `index_daily_closes`(`market_indices`는 PK=index_id 최신 1행이라 이력 불가), LLM 일자별 = 신규 `stock_daily_analyses`. 점수는 **서버 계산**(뉴스 감성 집계와 동일 원칙) + LLM은 문장만. 00:00 KST 기준 `trade_date`가 시장별로 다른 함정(미국은 그 시각이 장중) 명시. 부가 제안: 점수 적중률(score(D) vs return(D+1) 정렬 필수), 지수 대비 알파, 이벤트 오버레이
  - 검증: go build/vet/test(신규 유닛테스트 포함), Docker 재기동(Tesseract OK), 신규 admin 라우트 401 확인, tsc/eslint/next build, dev 서버 번들에서 사이드바 로직 실측. **관리자 로그인 필요 화면(`/admin/llm`) 육안 확인은 이전 세션들과 동일 사유로 미완료** — 수동 즉시 생성 UI와 실제 LLM 호출은 사용자 확인 필요
- 08-07: **Gemini 실측 한도 반영(RPM 15/RPD 500/TPM 250K) + 요청 기록 종목명 표시** (v0.8.0 이후, 릴리스 미생성)
  - 사용자가 Gemini 3.1 Flash-Lite 공식 무료 티어 한도를 직접 제공(RPM 15, TPM 250K, RPD 500) — 기존 추정치(DailyQuota 800/PerMinute 10, 서드파티 집계 사이트 기반)를 정확한 값으로 교체(`DailyQuota:500, PerMinute:15, TokensPerMinute:250000`)
  - **TPM(분당 토큰) 실측 추적 신규 구현**: `apibudget.Manager`에 분당 토큰 카운터 추가(`RecordTokens`, Redis IncrBy + 2분 TTL, 메모리 폴백), `Check()`가 이번 분에 이미 누적된 토큰이 한도 이상이면 추가 호출을 차단(사전 추정이 아니라 사후 누적 기준 — 단일 요청이 한도를 처음 넘기는 건 못 막지만 같은 분 내 반복 호출로 인한 초과는 막음). `llm/client.go`가 OpenAI 호환 응답의 `usage.total_tokens`를 실제로 파싱해 기록(실측: 삼성전자+SK하이닉스 2회 호출로 14,771 토큰 기록 확인)
  - `/admin/llm`·`/admin/api` 양쪽에 분당 토큰(TPM) 사용량 바 추가
  - **요청 INPUT 기록을 종목명 기준으로 확인 가능하도록 수정**: `LLMRequestLogEntry`에 `Name` 필드 추가(로깅 시점에 `stock.Name` 그대로 사용, 별도 조회 불필요), `/admin/llm`에 종목명 드롭다운 필터 추가 — 로그 항목도 종목코드 대신 종목명을 우선 표시(코드는 보조로 작게 병기)
  - **사이드바 버그 재확인**: 사용자가 "여전히 뉴스 관리 탭이 항상 활성으로 보인다"고 재보고 — 실행 중인 dev 서버가 실제로 서빙하는 컴파일된 JS 번들을 직접 열어 검사해 수정된 코드(주석 포함)가 정상 반영되어 있고 기존 버그 패턴은 존재하지 않음을 확인. 코드는 정상이며 브라우저 캐시 문제로 판단, 사용자에게 강력 새로고침 요청
  - 검증: go build/vet/test, Docker 재기동 후 005930·000660 재검증(TPM 14,771 기록, 종목명 정상 표시, 한도값 500/15/250000 정상 노출), tsc/eslint/next build
  - 버전/릴리스 문서화는 v0.9.0에서 누적분과 함께 완료(`docs/releases/v0/v0.9/v0.9.0.md`)
- 08-07: **v0.8.0 — 사이드바 활성 탭 버그 수정 + LLM 모델 관리 상세화(호출 주기·API 한도·요청 INPUT 기록)** (`docs/releases/v0/v0.8/v0.8.0.md`)
  - **사이드바 버그**: `item.href === "/admin"` 특수 케이스가 `pathname.startsWith("/admin/")`이면 무조건 활성 처리 — `/admin`이 다른 모든 관리자 페이지의 접두사라 항상 "뉴스 관리"도 같이 활성으로 보이던 원인. 특수 케이스 제거, 다른 메뉴와 동일한 정확일치/자식경로 규칙으로 통일(`/analysis`,`/market`용 중복 특수케이스도 함께 정리)
  - **호출 주기 관리자 편집**: `llm_settings`에 `refresh_interval_hours` 컬럼 추가(1~24h 클램프, `columnExists` 기반 idempotent 마이그레이션). `time.Ticker`(런타임 변경 불가)를 버리고 매 주기 최신 설정을 재조회하는 sleep 루프로 전환
  - **API 한도 인라인 표시**: `/admin/llm`에서 openai provider의 분당/일간/월간 호출량 그래프를 바로 확인 가능(기존 `/admin/api` 이동 불필요)
  - **요청 INPUT 기록**(신규): `GenerateStockSummary` 호출마다 factsJSON+성공/실패를 Redis 캡드 리스트(`llm:requests:v1:log`, 최근 30건·7일 TTL)에 기록, `GET /admin/llm-requests`(읽기전용) + `/admin/llm`에서 펼쳐보기. 실측: 000660 호출의 실제 factsJSON(재무·내부자·뉴스 데이터 전체) Redis LRANGE로 직접 확인
  - **버그 발견+수정**: v0.7.0 스케줄러가 파일럿 종목을 간격 없이 연속 갱신해 2번째 종목부터 apibudget MinInterval(4s)에 걸려 조용히 규칙 기반으로만 캐시되던 문제를 재검증 중 다시 확인(005930 llm / 000660 rule) — 6초 간격 추가로 재수정, 재검증 후 둘 다 llm 확인
  - 검증: go build/vet/test, Docker 재기동, Redis 요청 로그 실측, 관리자 엔드포인트 401 정상 차단, tsc/eslint/next build, `/admin/llm` 미인증 리다이렉트 확인. 관리자 로그인 화면 육안 확인은 이전 세션들과 동일 사유로 미완료
- 08-07: **관리자 메뉴 개편 — LLM 모델 관리 신설 + 뉴스/데이터베이스 관리 명칭 정리** (v0.8.0에 포함되어 출시됨)
  - 사용자 피드백: API 관리 탭엔 (기존처럼) 호출량·한도만 있으면 되고, 파일럿 종목 관리는 별도 "LLM 모델 관리" 메뉴로 분리 요청 + 종목을 관리자가 직접 편집 가능해야 함(기존엔 env 전용)
  - **파일럿 종목을 DB 설정으로 전환**: 신규 `llm_settings` 테이블(단일 행, `market_data_settings`와 동일 패턴) + `LLMSettingsRepo`/`LLMSettingsStore`(요청마다 즉시 재조회 — 재기동 없이 관리자 편집 즉시 반영). 기존 `STOCK_SUMMARY_LLM_SYMBOLS` env 플러밍은 완전히 제거(단일 소스 유지)
  - 신규 `/admin/llm`(LLM 모델 관리): provider 상태 + 파일럿 종목 편집 폼(쉼표 구분 입력, 최대 10개·자동 정규화) + 종목별 캐시 상태. `/admin/api`에서는 파일럿 섹션 제거(원래 목적대로 provider별 호출량/한도 그리드만 유지 — LLM도 다른 provider와 동일하게 그 그리드에 카드로 표시됨)
  - 사이드바 라벨 정리: "관리자"→"뉴스 관리"(실제로 뉴스 키워드/통계/폴링 설정 페이지였음), "데이터베이스"→"데이터베이스 관리"
  - **버그 발견+수정**: 스케줄러가 파일럿 종목을 간격 없이 연속 갱신해 2번째 종목부터 apibudget MinInterval(4s)에 매번 걸려 조용히 규칙 기반으로만 캐시되던 것을 실측(005930은 llm, 000660은 rule로 확인) 후 발견 — 심볼 간 6초 간격 추가로 수정, 재검증 후 둘 다 llm 확인
  - 검증: go build/vet/test, Docker 재기동 후 005930·000660 모두 `source:"llm"` 확인(간격 버그 수정 전/후 대조), `/admin/llm-settings`가 401로 정상 차단, `/admin/llm` 미인증 접근 시 정상 리다이렉트 확인, tsc/eslint/next build. 관리자 로그인 화면 육안 확인은 이전 세션들과 동일 사유로 미완료
  - 버전/릴리스 문서화는 v0.8.0에서 완료(`docs/releases/v0/v0.8/v0.8.0.md`)
- 08-07: **v0.7.0 — Gemini 연동 + 2종목(삼성전자·SK하이닉스) 파일럿 제한 + 관리자 모니터링** (`docs/releases/v0/v0.7/v0.7.0.md`)
  - `config.buildLLMConfig()`: `OPENAI_API_KEY` 우선, 없으면 `GEMINI_API_KEY`로 폴백해 Google OpenAI 호환 엔드포인트+`gemini-3.1-flash-lite` 기본값 사용(코드 변경 없이 provider 전환 가능, `OPENAI_BASE_URL`/`OPENAI_MODEL`로 오버라이드 가능). 실측: 005930·000660 모두 실제 Gemini 응답으로 `source:"llm"` 확인
  - **비용 통제**: `STOCK_SUMMARY_LLM_SYMBOLS` env(기본 `005930,000660`)로 LLM 호출 자체를 파일럿 심볼로 제한(규칙 기반 요약은 모든 종목에서 여전히 제공). 스케줄러도 파일럿만 예열(기존 `AllTrackedSymbols` 전체 예열에서 축소). apibudget 한도를 Gemini 무료 티어 공개치보다 보수적으로 조정(DailyQuota 800/PerMinute 10/MinInterval 4s). 실측: AAPL 등 비파일럿 종목은 `source:"rule"` 고정, LLM 호출 자체가 안 일어나는 것 확인
  - **관리자 모니터링**: 신규 `GET /admin/stock-summary`(읽기 전용, 생성 트리거 안 함) — provider + 파일럿 종목별 캐시 상태. `/admin/api`에 "AI 요약 파일럿" 섹션 추가
  - 검증: go build/vet/test, 005930·000660·AAPL 실측(LLM/규칙 분기 정상), apibudget 사용량 증가 확인, tsc/eslint/next build, 브라우저 데스크톱+모바일 확인(관리자 화면 로그인 확인은 이전 세션과 동일 사유로 미완료)
- 08-07: **뉴스 탭 + AI 3줄 요약(밸류에이션/기술적분석/내부자/뉴스) 신규 구현** (v0.7.0에 포함되어 출시됨)
  - **뉴스 탭**: 백엔드 변경 없이 기존 파이프라인 재사용 — `fetchStockNews`(이미 심볼 필터 지원) + 그동안 미사용이던 `StockNewsPanel` 컴포넌트를 워크스페이스/`/analysis` 양쪽 "뉴스" 탭에 연결. `stock-workspace-tabs.ts`/`analysis-tabs.ts`에 `news` 탭 id 추가
  - **AI 3줄 요약**: 예시 문구(밸류에이션/기술적분석/뉴스, 감성 점수 포함)와 동일한 기능이 이미 있는 줄 알았으나 조사 결과 전무 — 종목별 LLM 파이프라인, 뉴스 감성 분석, RSI/MACD/ADX 계산 모두 신규 구현. 사용자 확인: 지표는 실제 계산(허구 금지), LLM은 비용 통제 설계 필요, 노출 위치는 탭별 상단 → Plan Mode로 설계 승인 후 구현
    - **기술적 지표**(`technical_indicators.go`, 신규): SMA20/50/200·RSI(14, Wilder)·MACD(12,26,9)·ADX(14, Wilder)·52주 고저·최근 스윙 고저 — `candle_bars` 최근 200일봉에서 순수 함수로 계산, 데이터 부족 시 해당 필드만 nil(수치 조작 없음). 유닛 테스트 3건(부족한 히스토리/단조 상승/빈 입력) 통과
    - **비용 통제**: 탭 4개(밸류에이션·기술적분석·내부자·뉴스)를 **LLM 호출 1회**로 한 번에 생성(`llm.GenerateStockSummary`, 신규). `insights.go`와 동일한 stale-while-revalidate 패턴을 심볼 기준으로 이식(`StockSummaryService`, Redis `stock:summary:v1:<symbol>`, TTL 24h/stale 6h). 백그라운드 예열은 `MarketDataStore.AllTrackedSymbols`(관심+보유 종목, 기존 헬퍼 재사용)만 6시간 주기로 순회 — 무제한 전체 예열이 아님. 뉴스 감성은 LLM이 기사별 라벨만 반환하고 긍정/부정/중립 **집계는 서버에서 직접 계산**(실제 기사 수와 항상 일치, LLM이 숫자를 지어낼 수 없는 구조)
    - `llm/client.go` 리팩터: `GenerateBriefing`/`TranslateENToKO`가 중복 구현하던 HTTP 스캐폴딩을 `chatCompletion` 공용 헬퍼로 추출(동작 변경 없음) 후 `GenerateStockSummary` 추가. `apibudget`에 `ProviderOpenAI`/`CategoryLLM` 신설, `llm.Client`에 budget Check/Record/429 BlockFor 연결(기존엔 LLM 호출이 budget 추적 대상이 아니었음 — 부수 개선)
    - **버그 발견+수정**: `apibudget.Manager.AllUsage()`의 provider 하드코딩 목록에 신규 `ProviderOpenAI`를 빠뜨려 `/admin/api`·`/health/api-quotas`에 노출되지 않던 것을 실측(`curl .../health/api-quotas`에서 openai 항목 없음) 후 발견·수정
    - **LLM 미설정(OPENAI_API_KEY 없음) 환경에서 항상 폴백**: 규칙 기반 문장(계산된 실수치만 나열, 예: "PER 35.2배, PBR 3.1배, 영업이익률 13.1%...")으로 정상 동작 확인 — 이 개발 환경엔 키가 없어 LLM 경로 자체는 라이브 검증 못 함(코드 리뷰로만 확인), 실제 LLM 응답 파싱은 사용자 환경에서 키 설정 후 재확인 권장
  - FE `StockSummaryBanner`(`section` prop으로 4종 분기) 신규, 재무제표/실적/전망/내부자거래(→valuation·insider)·차트(→technical)·뉴스(→news) 6개 탭 상단에 배선(워크스페이스+`/analysis` 양쪽)
  - 검증: go build/vet/test(신규 유닛테스트 포함) 전체 통과, Docker 재기동 후 005930·AAPL `/summary` 실측(실제 계산된 PER/PBR/RSI/ADX/내부자 집계/뉴스 건수 확인, 가짜 수치 없음), Redis 캐시 키·TTL(24h) 확인, 스케줄러가 관심종목 자동 예열하는 것 실측 확인(재기동 직후 여러 심볼 캐시 생성됨), FE tsc/eslint/next build, 브라우저로 6개 탭 전부 데스크톱+모바일 렌더링 확인
  - 버전/릴리스 문서화는 v0.7.0에서 완료(`docs/releases/v0/v0.7/v0.7.0.md`)
- 08-07: **재무제표 상세 모달 "1년" 트레일링 윈도우 버그 수정** (v0.7.0에 포함되어 출시됨)
  - `DartClient.FetchQuarterlyHistory`(`dart_financials.go`)가 `연도` 단위로 DART를 조회해(`currentYear-offset`), "1년" 선택 시 올해 들어 이미 제출된 분기만 반환 — 연초에는 1~2분기만 표시되던 버그. `years`만큼의 여유 연도를 추가로 더 가져온 뒤(`years+1`개 연도), 최근 `years*4`개 분기만 남기는 방식으로 변경해 진짜 트레일링 12개월이 되도록 수정. 미국(EDGAR) 쪽은 애초에 `FetchQuarterlySnapshots`가 "가장 최근 N개 신고기간"을 반환하는 구조라 이 버그가 없었음(대조군)
  - 검증: go build/vet, Redis 캐시(`financials:history:kr:v1:*`) 삭제 후 005930 `years=1` 재조회로 25년 6월~26년 3월(트레일링 4분기) 확인, `years=3`도 12분기 정상 확인, 브라우저로 재무제표 탭→수익성 카드→모달→"1년" 클릭해 동일 결과 렌더링 확인
- 08-07: **v0.6.0 — 내부자 거래 탭 구현 + 전망 탭·재무제표 상세 모달·DART/EDGAR 최신성 비교·마켓 레이더 탭 유지 버그 묶어 출시** (`docs/releases/v0/v0.6/v0.6.0.md`)
  - 신규 `GET /stocks/{symbol}/insider`: 국내는 DART `elestock.json`(임원ㆍ주요주주 소유보고, 날짜 필터 없음 — 전체 이력 중 최근 30건) 직접 매핑. 미국은 `submissions.json`으로 최근 Form 4/4-A 신고 목록을 얻은 뒤 각 신고의 원본 소유보고 XML을 개별 fetch해 보고자ㆍ관계(이사/임원/10%주주)ㆍ거래코드(P/S/G/M/F 등 SEC 고정 19종)ㆍ수량ㆍ단가ㆍ거래후보유를 파싱(최근 3건 신고로 제한). Redis 24h 캐시
  - **버그 발견+수정**: EDGAR `apibudget`은 분당 8회를 앱 전체가 공유하는데, US 심볼 1건 조회가 최대 7회(submissions 1 + Form4 XML 최대 5 + 티커캐시미스시 +1)를 소비 — 여러 US 심볼을 연속 조회하면 예산 소진으로 개별 fetch가 전부 실패하는데, 기존 방식은 이를 "신고 없음"으로 오인해 빈 배열을 24시간 캐시해버림(MSFT로 실측 재현). "시도한 신고가 있는데 전부 실패"면 에러를 반환해 캐시하지 않도록 수정 + 신고 상한 5→3으로 하향. AAPL/MSFT/TSLA/NVDA/AMZN 실측으로 수정 전후 재현·검증
  - FE `InsiderTab`(국내/미국 통합 테이블, 구분별 색상, 지분율(KR)/단가(US) 동적 컬럼) 신규, 워크스페이스+레거시 `/analysis` 양쪽의 "내부자 거래" 플레이스홀더 교체(`AnalysisPlaceholderTab` import 완전 제거 — 8개 탭 전부 실구현 완료)
  - v0.6.0 릴리스 문서에 이번 세션의 내부자 거래 탭과 함께, 이전 세션에서 미출시 상태로 남아있던 전망 탭ㆍ재무제표 상세 모달ㆍDART/EDGAR 최신성 비교ㆍ마켓 레이더 탭 유지 버그 수정을 모두 묶어 문서화(`docs/releases/README.md` 인덱스 갱신, `fe/package.json` 0.5.0→0.6.0)
  - `.claude/launch.json`에 FE dev 서버 `autoPort: true` 추가 — 동시 세션 포트 충돌 시 자동 대체 포트(브라우저 검증 중 다른 세션의 3000번 서버와 충돌 확인 후 조치, 없어도 기존 단일 세션 워크플로엔 영향 없음)
  - 검증: go build/vet, 005930·AAPL·MSFT·TSLA·NVDA·AMZN 실측(EDGAR 예산 소진 재현+수정 재검증 포함), tsc/eslint/next build, 데스크톱(1280×720)·모바일(375×812) 브라우저 렌더링 확인(다른 세션의 실행 중인 dev 서버에 직접 접속해 검증 — 이 세션 자체 dev 서버는 포트 충돌로 기동 불가했음)
- 08-07: **전망 탭 구현 + DART/EDGAR 최신성 비교 + 마켓 레이더 탭 유지 버그 수정** (v0.6.0에 포함되어 출시됨)
  - `전망` 탭: 실적추세(financials/history 재사용)·밸류에이션 맥락(PER/PBR + PSR은 FE에서 `price*shares/revenue`로 신규 계산, "FY{연도} 실적 기준·DART/EDGAR" 라벨)·공시일정(`earnings` 엔드포인트의 `calendar` 재사용, `CalendarSection`을 `earnings-tab.tsx`에서 export해 공유)·컨센서스 안내블록. BE 변경 없이 전부 기존 엔드포인트 재사용. 스펙 4번(DART 잠정실적 공시 "본문" 파싱)은 자유 형식 문서 파싱이라 범위 밖으로 명시, 후속 과제
  - DART/EDGAR 최신성 비교(`FundamentalsService.RefreshSymbol`): SEC `company_tickers.json` 실측으로 실제 20-F 파일러 7종목만 매핑(`krxToEdgarTicker`: SK텔레콤·KB금융·신한지주·우리금융지주·POSCO홀딩스·한국전력·LG디스플레이) — 삼성전자·SK하이닉스는 무보증 ADR이라 매핑 없음(정상, 사용자 예시가 애초에 해당 없는 케이스였음). `PeriodEnd` 비교로 EDGAR가 "진짜 더 최신"일 때만 교체, 동일/구버전이면 DART 유지. 부수 수정: `FetchAnnualFinancialSnapshots`가 지금까지 `PeriodEnd`를 아예 채우지 않던 걸 발견해 `calendarQuarterEnd(year,4)`로 채움(이전 세션에 이미 만든 헬퍼 재사용) — 이 필드가 없으면 비교 자체가 불가능했음. `cmd/seed-fundamentals -symbols=017670` 로컬 실행으로 실측(DART가 정상적으로 계속 승리 — 20-F는 연간뿐이라 DART 분기 주기가 구조적으로 항상 더 최신, 예상대로)
  - 마켓 레이더 탭 리셋 버그: `market/page.tsx`의 `selectSymbol`/URL-sync 두 군데에 있던 강제 `setWorkspaceTab("overview")` 제거 — 각 탭이 이미 `symbol` prop 변경 시 자체 재조회하므로 탭 유지만으로 안전. 헤더 검색으로 SK하이닉스 선택 브라우저 테스트로 재무제표 탭 유지 확인(마켓 레이더 랭킹 리스트는 이 세션 브라우저에서 로딩이 안 걸려 리스트-클릭 대신 헤더 검색으로 동일 콜백 경로 검증)
  - 검증: go build/vet, tsc/eslint/next build, 017670·005930 fundamentals 실측 비교, 전망 탭 브라우저 렌더 확인(005930), 탭 유지 브라우저 확인(005930→000660)
- 08-07: **재무제표 탭 — 클릭 시 상세 모달(토스 스타일) + 실데이터 다분기 히스토리** (v0.6.0에 포함되어 출시됨)
  - 신규 `GET /api/v1/stocks/{symbol}/financials/history?years=N`(기본 3, 최대 5): 국내(DART) 최근 N년 × 4개 reprt_code 순회, 손익 항목(매출/영업이익/순이익/EPS)은 1~3분기 `thstrm_amount`(이미 단독분기, 라이브 확인) 직접 사용 + 4분기는 `연간 thstrm_amount − 3분기 thstrm_add_amount(9개월 누적)`으로 역산; 재무상태표 항목은 각 reprt_code의 시점값 그대로. 미국(EDGAR)은 기존 `FetchQuarterlySnapshots`에 재무상태표 concept 추출 추가 + limit만 늘려 재사용(추가 API 호출 없음, companyfacts 응답 1건에 이미 포함)
  - **버그 발견+수정 3건** (모두 라이브 API로 직접 검증):
    1. DART 반기(11012) 보고서의 당기순이익 계정명이 "반기순이익"으로 달라 순이익만 누락 — 계정명 매칭을 정확매치에서 "(손실)" 접미사 허용 매치로 일반화(`dartAccountNameMatchesAny`), `net_income` 폴백에 "반기순이익" 추가
    2. SEC XBRL이 동일 물리적 분기를 필터마다 다른 `fy` 태그로 중복 보고(원 신고 1회 + 다음 해 비교컬럼 1회) — AAPL 실측에서 2024-09-28이 "FY2024 Q4"·"FY2025 Q4" 두 번 중복 확인. `recentReportPeriods`/`pickFactForPeriod`의 매칭 키를 (FY,FP,End)에서 (FP,End)로 변경, FY는 표시전용으로 분리
    3. (이전 세션 실적 탭에서 발견) EDGAR 표준분기 길이 검증은 이미 있었음 — 이번엔 재무상태표(BS) 항목에 동일 검증을 잘못 적용하지 않도록 `pickInstantFactForPeriod`(End만 매칭, duration 무관) 별도 추가해 회피
  - FE `FinancialDetailModal`(3-preset: 수익성/성장성/안정성, ComposedChart bar+line + 표 + 1· 3 · 5년 토글, 헤드라인 문장 "직전 분기 대비 ±%/%p") 신규, `FinancialsChartsTab`의 가짜 배수 데이터(`0.72+i*0.07`)를 실제 히스토리로 교체하고 3개 카드에 클릭 핸들러 연결
  - 검증: BE go build/vet, 005930·AAPL 실측(005930 9분기 전 필드 정상, AAPL 13분기 중복 없음 확인), FE tsc/eslint/next build, 브라우저로 두 프리셋(수익성/안정성) 클릭→모달→연도토글→닫기 전체 플로우 데스크톱 확인. **모바일 뷰포트(375px)에서 카드 자체는 정상 렌더 확인, 단 모달 오픈 클릭이 이 환경의 `computer` 도구에서 타임아웃되어(브라우저 pane 미표시) 모바일 모달 자체는 직접 클릭 검증 못함** — CSS는 기존에 이미 동작 중인 `chart-fullscreen-modal.tsx`와 동일한 반응형 패턴 재사용, 사용자 확인 권장
  - 버전/릴리스 문서화는 v0.6.0에서 완료 (`docs/releases/v0/v0.6/v0.6.0.md`)
- 08-07: **v0.5.0 — 실적 탭 구현** (`docs/releases/v0/v0.5/v0.5.0.md`)
  - 신규 `GET /api/v1/stocks/{symbol}/earnings`: 국내(DART 잠정실적/사업·분기·반기보고서 키워드 필터 공시 + reprt_code 4종 분기 상세) / 미국(Finnhub 발표일정 + EDGAR 최근 5개 신고기간) 분기 처리, Redis 24h 캐시, DB 스키마 변경 없음(라이브 조회)
  - **버그 발견+수정 1**: DART curl TLS 폴백이 `curl.exe`+`GOOS=="windows"` 게이트라 Docker(Linux) 컨테이너에서 항상 스킵 — 국내 실적 신규 코드가 전부 빈 응답이던 원인. `dart_http.go` OS 무관하게 동작하도록 수정(기존 DART 실시간 경로 전반에 영향, 잠재 버그였음)
  - **버그 발견+수정 2**: EDGAR 분기 추출이 3개월 단일분기와 6/9개월 누적(YTD) XBRL 사실을 구분 못해 애플 Q2/Q3 매출이 \$254B/\$364B로 부풀려짐 — `xbrl_select.go`에 60~100일 표준분기 길이 검증(`isStandaloneQuarterDuration`) 추가, 수정 후 \$109~144B 정상 범위로 확인
  - FE `EarningsTab`(`components/analysis/earnings-tab.tsx`) 신규, `stock-workspace-client.tsx`/`analysis-client.tsx` 양쪽 실적 탭 플레이스홀더 교체. 실적 요약은 기존 `/fundamentals` 데이터 재사용(중복 호출 없음)
  - 검증: BE go build/vet 통과, 005930·AAPL·MSFT 실측 확인, FE tsc/eslint/next build 통과, 데스크톱·모바일 브라우저 렌더링 확인(워크스페이스 + 레거시 `/analysis` 양쪽)
- 08-06: **마켓 레이더 "1분 주기" 표시 수정 + 관리자 `/admin/api` 신설**
  - 원인: 실제 폴링은 `market_data_settings.rankingsPollRealtimeMinutes`(=5)를 정상 사용 중이었으나, 툴바 라벨(`rankingRefreshHint`, `market-radar-config.ts`)이 `"1분 주기"`를 하드코딩 — `useMarketDataSettings()` 실측값을 인자로 전달하도록 수정, 훅의 DEFAULTS 폴백(1→5)도 정정
  - `apibudget` 패키지에 provider별 용도/카테고리 카탈로그(`catalog.go`) 추가; `Limits`에 커스텀 `MarshalJSON`(MinInterval을 ms로) 추가 — 기존엔 JSON 태그가 없어 FE가 못 읽는 상태였음
  - `AdminHandler.APIQuotas`가 카탈로그(용도·카테고리) + 실시간 사용량(분/일/월 카운트, 마지막 호출, 차단 여부)을 병합 응답. 라우트(`/api/v1/admin/api-quotas`)는 기존 것 재사용
  - FE `/admin/api` 신규 페이지: 카테고리(시장데이터/공시·재무/뉴스/번역)별 provider 카드 대시보드 + 사용량 progress bar. 사이드바 "관리" 그룹에 "API 관리" 추가. 관리자 홈의 인라인 "API 할당량" 표는 제거하고 링크로 대체
  - 검증: `go build`/`tsc`/`eslint`/`next build` 통과, Docker 재기동 후 `/health/api-quotas`(동일 타입 사용)로 JSON 직렬화 실측 확인. **관리자 세션 화면 육안 확인은 미완료** — 테스트 계정에 admin 권한 부여(DB UPDATE)가 auto-classifier에 차단되어 실제 로그인 후 `/admin/api` 렌더링은 사용자 확인 필요
- 08-06: **랭킹 5분 정렬 폴링 + UI 회귀 조치 + 설계안 2건** (v0.4.0 이후, 릴리스 미생성)
  - 마켓 레이더 랭킹 `realtime` duration: 1분(상대시간) → **5분, 벽시계 정렬**(00:00/00:05/…). 최초 1회는 서비스 시작 시각 그대로, 이후는 `alignedRankingBucket`(`t.UTC().Truncate(interval)`)로 버킷 경계에서만 갱신 — `market_rankings_poller.go`. DB 기존 행 기본값도 마이그레이션으로 1→5 일괄 조정(`MigrateRankingsRealtimeDefaultIfNeeded`, 관리자가 이미 바꿔둔 값은 보존)
  - 기업정보 탭 매출·산업 구성 파이차트 애니메이션 제거(`isAnimationActive={false}`) — `company-info-tab.tsx`
  - UI 회귀(모바일 뷰가 풀 화면 기준으로 보임) 조사: `data-shell` 메커니즘 자체는 정상(코드 리뷰+모바일/데스크톱 뷰포트 실측 모두 정상). 오래 켜져있던 dev 서버가 08-05와 동일 패턴(dev 캐시)일 가능성 높아 `.next` 캐시 삭제 후 재기동 — **육안 재현은 못 함(스크린샷 도구 제약), 사용자 확인 필요**
  - 설계안 2건 제시(미구현, 결정 대기): 마켓 레이더 랭킹 실시간 가격 오버레이(기존 배치가격 파이프라인에 편승), 실적/전망/내부자거래 탭 무료 API 매핑(DART `elestock.json`으로 내부자거래는 국내·미국 완전 커버 가능, 전망 탭은 목표가/컨센서스가 전부 유료라 플레이스홀더 문구 재정의 필요)
- 08-06: **v0.4.0 — 헤더/차트 시세 정합 + Toss 429 구조 개선** (`docs/releases/v0/v0.4/v0.4.0.md`)
  - 헤더·차트 가격 불일치: `refreshTerminalSnapshot`이 계산한 가격을 `refreshFormingBars`에 공유(같은 틱 내 동일값 보장); `splitFormingCandle`/`lastCompletedTime` 배열 순서 의존 제거(Toss 일봉은 내림차순 응답); FE `use-quote-candles.ts`에 2초 REST 폴백 추가(dev 프록시가 SSE 스트림을 버퍼링해 응답이 전혀 안 오던 것 확인)
  - Toss 429 폭주 원인: `/api/v1/candles`가 94% 비중 — `refreshRawCandles`가 30초마다 심볼당 1분봉 7,200개(36페이지) 전량 재조회하던 것을 DB 마지막 봉 이후 델타만 요청하도록 변경(`candles_intraday_delta.go`); `fetchIntradayStats`(시세 통계)도 동일 패턴으로 DB-first 전환
  - `/api/v1/trades`,`/api/v1/orderbook`: 관심·보유종목(pinned)이 미조회 상태여도 5초마다 갱신되던 것 제거 — 실제 조회 중(active)인 심볼만 전체 갱신, 가격은 배치 호출로 계속 신선(사용자 수 확장 시 관심종목 수가 아니라 동시 조회 수에만 비례)
  - `X-RateLimit-*` 헤더 로깅(`toss/rate_limit.go`) + `Retry-After` 실값 사용(고정 2초/60초 제거)
  - 실측: trades 75→0, orderbook 59→0 (60초 표본); candles 94%→소수 비중
  - **BE 전용 — CSS/스타일 파일 미변경** (2026-08-05 dev 서버 CSS 캐시 레이아웃 회귀와 무관)
- 08-05: **DB 최적화 + 관리자 DB 콘솔**
  - `candle_bars` 재구축: 행 단위 UPDATE 루프(사실상 미완료) → `INSERT IGNORE ... SELECT` 단일 패스 (2.53M행 27초)
  - `bar_time` VARCHAR(32) → DATETIME, PK와 완전 중복이던 보조 인덱스 제거 → **516.9MB → 195.8MB (62%↓)**
  - 이전 테이블은 `candle_bars_legacy_v1` 로 보존 → 확인 후 `migrations/026_storage_cleanup.sql` 참고해 수동 DROP
  - `ranking_snapshot_entries` 90일 보관 (`PruneOlderThan`, 12시간 주기, 5000행 배치)
  - **`/admin/database`** — 테이블 용도 카탈로그 + 행 CRUD. 접근 가능 테이블은
    `service/db_admin_catalog.go` 화이트리스트에만 한정, 식별자는 항상 live schema와 대조 후 사용,
    비밀번호 해시·암호화 자격증명 컬럼은 마스킹, 머신 관리 테이블은 읽기 전용
- 08-05: **성능 리팩토링 (BE+FE)** — 상세: `docs/archive/sessions.md`
  - Poller: fast tick 겹침 방지(CAS) + 동시성 4 제한 + active(1s)/pinned(5s) 분리 + 심볼 상한 40
  - `refreshTerminalSnapshot` Toss 콜 5→3 (no-op KRW 정규화용 `fxForSymbol` GetStock 제거)
  - `candle_bars` UpsertBatch 행별 Exec → 500행 multi-row upsert
  - `HasTable` / `barTimeIsDatetime` information_schema 결과 캐싱 (positive latch + 30s 음성 재확인)
  - watchlist/holdings 조회를 fan-out 스코프로 메모이즈 (`withOverlayCache`) — 배치당 N→1
  - MySQL pool 10 → 50/25 (`MYSQL_MAX_OPEN_CONNS`/`MYSQL_MAX_IDLE_CONNS`)
  - Graceful shutdown: http.Shutdown → bgCancel → pool close 순서 + SIGTERM 핸들링
  - `QuotesService.StartPolling(ctx)` 분리 — 생성자에서 poller 기동 시 Set* 와이어링과 경합하던 문제
- 08-04: **quotes:detail Redis 제거** — detail read = `quotes:terminal`(1s) → MySQL; startup purge `quotes:detail:*`; ensureQuoteDetail → refreshTerminalSnapshot
- 08-13: **뉴스 RSS·지수 태그 확장** — 국내 RSS(한경 finance·MK 증권·연합 경제·토큰포스트·코인리더스·블록미디어, 더팩트 제거), 크립토 관련성·Finnhub crypto, 국내 헤드라인 중복 skip, ingest `index:{id}` 태그 + 폴링 키워드(`040_news_index_keywords.sql`)
- 08-04: **Redis cache unify** — single envelope key per entity; startup purge of `:last` duplicates; freshness guard on read
- 08-03: **Admin 캐시 관리** — `/admin/cache` UI + `/api/v1/admin/cache/*` (Redis SCAN/GET/SET/DELETE; oauth 차단, apibudget 읽기전용)
- 08-03: **Continuous Toss polling** — `shouldLivePollSymbol` = Toss enabled (세션 무관); 장 마감 종가 캐시 reuse 폐기
- 08-03: **Single price resolver** — `ResolveLiveQuotePrice` (rankings·quotes·batch·terminal 공통)
- 08-03: 마켓 레이더 — 시세 시각(updatedAt) 실시간 갱신, 랭킹→배치시세 덮어쓰기(프리장) 수정, PER/PBR/PSR 한글 설명, `market_data_settings` 관리자 폴링 설정
- 07-30: FE `StockChartTerminal` — chart/trades/orderbook extracted from quotes-detail-panel → `fe/src/components/stock-workspace/stock-chart-terminal.tsx`
- 07-26: Market indices batch refresh (FRED 3분, warm-up, sp500/dow/vix fix, API `groups`/`dataKind`) → `docs/MARKET_INDICES_BATCH.md`
- 07-26: FE 계좌 연동 메뉴 비노출 (`ACCOUNT_FEATURES_VISIBLE=false`, `docs/FE_ACCOUNT_FEATURES.md`); BE/storage 유지
- 07-25: Naver/Kakao news apibudget → 일일 cap만 (MinInterval/PerMinute 제거, tick당 키워드 일괄 검색)
- 07-25: Naver 뉴스 → NAVER API HUB (`naverapihub.apigw.ntruss.com`, NCP 인증 헤더)
- 07-25: 뉴스 `stripHTML` → `html.UnescapeString` (&#34;/&#39; 등 숫자 엔티티 디코딩)
- 07-25: MySQL terminal persist (011) + write-through/read chain; AGENT_STATE durable policy
- 2026-10-05: 에이전트 작업 방식 정리 — AGENT_STATE 2,892줄을 53줄로 줄임(옛 Now·TODO·Last는 원문 그대로 이 파일 「AGENT_STATE 이관 원문」으로), 원칙은 docs/PRINCIPLES.md로 분리, AGENTS.md 프로토콜 개정, 코드 주석 참조 갱신 · 배포 없음
- 2026-10-05: 루트 임시 파일(tmp-*.json·tmp-tgt.js·--)·e2e 산출물·cmd/probetmp 제거 · 배포 없음
- 2026-10-07: 사용자 요청 11건 — 리프레시 토큰 슬라이딩 만료+갱신 401 시 세션 비움(내 종목 빈 카드), 내 종목 브리핑·뉴스 아래, 지수 출처는 모달로, SPY/DIA 숨김, 홈 카드 높이 맞춤·자산 부동산|금융 두 칸, AI 요약 상세 기본·LLM 있으면 맨 위, 주가 왜·일별 시세 한 카드, 실시간 차트 차트·호가 탭 숨김, 시나리오(점수 카드 안)·애널리스트(상세화) 개요로, 기업·재무 한 탭, 글 없는 종목 커뮤니티 탭 숨김 · 미배포(api 재시작·FE 재빌드 승인 대기)
- 2026-10-07 (2): 후속 — 작업 끝 자동 배포 규칙(AGENTS.md), 커뮤니티 탭 글만, SPY·DIA BE 카탈로그 제거·국고채 토스 실시간·QQQ/SOXX 2분, 애널리스트 이력·리포트 표(099)+카드 추이·변화·리포트, 리프레시 회전 30초 유예(로그아웃은 즉시), 커뮤니티 글 한 번 받아 공유, 스크리너 버튼 문구 · 운영 배포 완료(api·worker 3·fe)

- 2026-10-09: 예·적금 비교 적금 탭에 청년미래적금 정책상품 카드(은행연합회 공시 14행, 수기 입력 policy-products.ts) · fe 재빌드, api 재생성, worker-stock/market/content 재생성(이미지 일치)
- 2026-10-09: 청년미래적금 카드 강조(배지·최고금리·버튼) + 정기예금 탭 안내 띠 · fe 재빌드, worker 3 재생성
- 2026-10-09: 청년미래적금 카드를 정기예금 탭에도 기본 표시(안내 띠 제거) · 배포 보류(다른 작업의 stock-workspace tsc 오류)
- 2026-10-09: 요청 9건 — 종목 탭 한 벌(차트 탭 전면 삭제·analysis 모드/탭 override 제거)·해외 레딧 탭·기업·재무 토글 좌상단 lg·실적 탭/일정 표 좁으면 카드(컨테이너 쿼리)+매출·반응·적중이력·자산 미리보기 GET /assets/peek+openAsset·홈 브리핑 주식/부동산/금리 탭·금리 TOP(예금/적금/주담대, /home/assets products)·해외 Reddit 커뮤니티 카드 개편·보조 문구 정리(HelpHint) · fe 재빌드+api(네트워크 0개 재발→무빌드 재생성), worker 4 재생성, health 200
- 2026-10-09: 요청 6건 중 ①실적 AI 요약(earnings_digests·/earnings/digest·카드) ③변화 시총 순위(market_cap 이관 6,189행) 배포됨(다른 세션 배포) · ④뉴스 topic(주제 차선·분류·자산 칩·6칩·메뉴명) ⑤주제 기사→부동산·금리 브리핑/시군구 근거 구현·시험 통과, 미배포 · ②스크리너 ⑥금융상품 미착수(시안 docs/mockups/requests-1009-v1.html, 결정 메모리 requests_1009_decisions)
- 2026-10-09 (2): 요청 6건 나머지 완료 — ②스크리너 A(프리셋 /stocks/screen/presets·조건 칩·보기 탭·모바일 카드) ⑥금융상품 분석(계산기·비교 창 삭제, AI 시장 분석 fin_savings/fin_loans, 시장 내 위치) ⑤시군구 기사 근거 · 주제 검색어→관리자 키워드(101) · 헤더 검색 아파트·금융상품·금리(/search/assets) · 단지 목록 re_complexes(4.3만, 범어역우방유쉘 원인=띄어쓰기·도로명·시군구 필수·점수 단지만 색인) · 뉴스 분류 증시 가드·양방향 재분류 · fe·api·worker 4 배포(이미지 일치), asset_scores·asset_ai 수동 1회
- 2026-10-09: 검색 색인 점검 — 미색인 원인=종목 1,000 고아(SSR 링크 0)·본문 얇음, 옛 주소 6개 redirect()→next.config 308, 사이트맵 +/screener·/changes·/finance/rates·/real-estate(+auctions) · fe·api 재빌드, 워커 4 재생성(이미지 일치), /health 200
- 2026-10-09 (3): 추가 개선 3건 — 자산 이름 색인을 단지 목록 전체로(점수 없는 단지 미리보기 404 해소) · 기사 칩에 전국 유일 법정동·단지명(우세 20배 규칙, 「…아파트」 일반어·해외 금리 제외) + 재분류가 칩도 갱신 · 도로명 백필(아파트 매매 6개월 이전 7,366구간 수집 표식 삭제 → 10-10 03:30 재수집) · fe·api·worker 4 배포(이미지 일치)
- 2026-10-09: 검색 색인 개선 A~D·0 — 색인 국내 500+미국 500, /stocks 허브, 종목 접힌 요약 바, 목록 종목명 링크+데스크톱 push 뒤로가기, 작성자 페이지 SSR·색인, 사이드바 약관, 사이트맵 connection()(빌드 때 빈 판), 옛 페이지 파일 삭제 · api·fe 재빌드, 워커 4 재생성(이미지 일치), panic 0
- 2026-10-09: 사업화 검토 문서 docs/BUSINESS_REVIEW.md(경쟁·집중 데이터·유입·결함 D1~D9·운영 S1~S8·P0~P2), 코드 변경 없음 · 배포 없음
- 2026-10-09: 2차 요청 11건 — 작성자·관리자 글 수정/삭제(리포트 완전 삭제) · 문장으로 찾기·LLM 사전 잡 제거 · 스크리너 시장 탭·아이콘·열 순서·60일 · 뉴스 칩 폭 버그 · 홈 공매 카드 제거 · 레딧 카드 재배치 · 실적 꼬리말 제거 · 글 주제(migration 102, 분석 리포트·메모·관심 목록 탭) · 프리셋 5.9s→1ms · 홈 자산 0.83s→70ms · LLM/컨테이너 점검(금감원 정전으로 fin_products 실패 확인) · api·fe·worker 3 배포, worker-market은 배치 종료 뒤 재생성
- 2026-10-10: 시·도·전국 미리보기(kind sido) + 부동산 제목 시·도 칩(부동산 칩 11→75/284) · 05:37 MySQL80 크래시(0x80000003, re_trades는 04:51 종료로 무관, daily_close_fetch_stale 실행 중) — 사용자 재시작 06:11 · api·fe·worker 배포(worker-market은 도는 잡 끝난 뒤) · daily_close_fetch_stale 반복 재수집 의심 → 별도 작업 제안
- 2026-10-10: 사업화 파이프라인 W1·W2 — 부동산·금융상품·커뮤니티·리포트·스탬프·목표가 비노출(lib/hidden-features.ts·proxy 307·사이트맵), 국내 비율 TTM(판 2026-10-10), 나스닥/다우/러셀 태깅 차단(재태깅 v10), AI 점수 순위·투자 권유 아님, 시간외 표기, 요약 standing 문구, 시나리오 범위 포함률 · api·fe 재빌드, 워커 4 재생성(이미지 일치), panic 0
- 2026-10-10 (2): 요약 대상 100+100+100 재편·상한 300 · 번역 azure,local · migration 103 DB 정리(중복 인덱스 4·문장 검색 표) · 앱 안 알림(104, 헤더 종·30분 생성기) · 자산 관련 글 카드 · 스크리너 프리셋 어제 대비 · MySQL 05:37 크래시 조사(0x80000003 자체 중단, 서비스 자동 재시작 미설정) · api·fe·워커 4 배포
- 2026-10-10 (2): 장애 여파 재실행 — asset_ai(10:26 다른 세션 재배포로 끊김) 재실행 완료 · 실적 관전 포인트 전부 실패 원인 = `SELECT close FROM stock_daily_closes`(열은 close_price) → earnings_preview_repo·research_review_repo 수정 + DB 회귀 시험, 재실행 실패 0 · fin_products는 금감원 점검(10-10 24:00까지)이라 10-11 05:10 자동 · api 재시작·worker 4 재생성(이미지 일치)
- 2026-10-10: 사업화 파이프라인 2차 — 약관 점검표, D5 결함 아님 확인, 서버 렌더 SEO(종목 실적 줄·실적 2주 목록·변화 목록+/changes/[date] 보관·사이트맵), 종목 공유 카드(5축 레이더 OG), 지난 발표 결함 수정, W4 시안 · fe 재빌드(api 동반), 워커 4 재생성(이미지 일치), panic 0
- 2026-10-10: 종목 화면 A안(왜 움직였나·레이더 점수 카드·그룹 내 순위 peerRank/peerSize·다음 실적 서프라이즈·관심 등록 안내·AI 요약 펼침 유지·정규장 가격) + /analysis?backtest=1, 홈 비로그인 v2 시안 · api·fe 재빌드, 워커 4 재생성(이미지 일치), panic 0
- 2026-10-10: 홈 비로그인 블록 v2 A안(히어로 검색·대표 6종목 레이더·점수 급변 축 귀인·주요 실적·성적 공개, 쿠키 없을 때만 SSR), 레이더 labels 옵션 · fe 재빌드(api 동반), 워커 4 재생성(이미지 일치), panic 0
- 2026-10-10: 미커밋 전부 커밋·push(public, 비밀값 점검), 사업화 2차 검토 반영(홈 메타·숨긴 기능 배치 4개 중지·robots·시나리오 접기·애널리스트 고지·금리 bp·종목 SSR에 AI 요약/다섯 축/스타일·업종 순위/스타일/비교/서프라이즈 페이지·사이트맵 1,131), 메뉴 12→7(종목 찾기·뉴스·실적 탭 줄), KRX 휴장일 12일 보정, docs/COMPETITORS.md · 배포 끝(api·fe·워커 4)
- 2026-10-11: 로그 점검 개선(Toss GetPrices 스펙 밖 심볼 제외·404 격리, DART corpCode 옛 목록 유지+재시도 간격, EDGAR 분기 없음 not_found, 낡은 휴장일 시험 3건) · Toss 스펙 감시 워크플로(매일 09:00 KST 이슈) + 기준본 1.2.24·asyncapi · api·워커 4 배포, panic 0
- 2026-10-11: 홈 비로그인 블록에서 대표 카드·변화·실적·소개 카드 제거, 지수 흐름 띠 맨 위 · fe 재빌드(+api·워커 4 이미지 일치), health 200, panic 0
- 2026-10-12: 홈 뉴스 기본 칩 주식 · 뉴스 칩 금리·예금·대출 하나(BE 필터 finance) · 뉴스 Pill 다크/테두리 잘림 수정 · 종목 찾기 → AI 점수 순위·종목 스크리너 두 칸(업종·스타일·변화 메뉴 보류, 변화 배지 훅 삭제) · 탭 줄 14px/36px · 반응형 14화면×4폭 넘침 0 · 앱 출시 검토(앱인토스 정책상 불가) · fe+api 재빌드, 워커 4 이미지 일치, health 200, panic 0
- 2026-10-12: 오늘의 변화 별도 메뉴(+내 종목 배지 복원) · 커뮤니티·투자 리포트 복원(hidden-features·robots API Allow) · 태블릿 768~1023 접힌 아이콘 레일 · fe+api 재빌드, 워커 4 이미지 일치, health 200, panic 0
- 2026-10-12: 커뮤니티·투자 리포트 관리자 전용(BE communityGate 36경로 + FE ADMIN_ONLY_ROUTE_PREFIXES·AdminOnlyGate·useCommunityVisible, 사이트맵·robots 제외) · 가상 데이터 시드는 자동 권한 검사에 막혀 미진행 · fe+api 재빌드, 워커 4 이미지 일치, health 200, panic 0, 익명 API 401
- 2026-10-12: ResearchRepo.Create user_id·topic 인자 순서 버그 수정 · cmd/seed-preview(관리자 전용 커뮤니티·리포트 화면 시험용 데이터) · api 재시작, 워커 4 이미지 일치, health 200, panic 0
- 2026-10-12: 리포트 검토 노트 LLM 제거(규칙 노트만, llm_feat_research 버킷 삭제, 화면 「검토 노트」) · API 기본 Cache-Control private,no-store(DefaultNoStore) — Cloudflare가 헤더 없는 응답을 캐시해 관리자에게 옛 글이 보이던 원인 · fe+api 재빌드, 워커 4 이미지 일치, health 200, panic 0
- 2026-10-13: 커뮤니티·투자 리포트 관리자 전용 해제 — communityGate 통과·ADMIN_ONLY_ROUTE_PREFIXES 비움·AdminOnlyGate/useCommunityVisible이 목록을 따름·robots 복원, 기존 글 유지 · api+fe 배포(워커 재생성 안 함)
- 2026-10-13: 커뮤니티·리포트 주제 주식만(부동산·금융상품 선택·필터 숨김) · 브리핑 많이 언급된 종목 국내 문턱 25→20 · 배포 api+fe+worker-content
- 2026-10-13: 종목 개요 v2 — 최근 핵심 사건·AI 요약 간단 기본 위로·점수 레이더+다섯 막대·전일 칩 머리줄·다음 실적 표·90일 범위 숫자화·애널리스트 결론 문장·해외 레딧 카드, 관심 안내·질문 칩 삭제 (시안 stock-overview-v2) · fe 배포(api 재생성), health 200
- 2026-10-13: 홈 점수·거래대금·실적 카드 높이 맞춤(기준일 제목 옆·실적 7줄·priceAsOf 정규식 수정) · api+fe 배포
- 2026-10-13: 홈 실적 일정 7줄 보장(2주 창 부족분을 100일 창으로·최근 발표 recentBig로 채움) · api+fe+worker-content·stock 배포
