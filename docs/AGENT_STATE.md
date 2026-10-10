# Agent State

> 현재 상태만 담는다. **150줄 이하, 매 세션 덮어쓴다(앞에 쌓지 않는다).** 이력은 `docs/archive/sessions.md`, 원칙은 `docs/PRINCIPLES.md`, 결정·함정은 메모리.
> 갱신: 2026-10-10 (사업화 파이프라인 W1·W2 — 기능 비노출·국내 TTM·태깅 오탐·문구)

## NOW
- 10-07 요청 11건 + 후속(배포·커뮤니티 글만·지수 주기·애널리스트 이력) **운영 배포 끝**(api·worker-stock/market/content 재생성, fe 재빌드). 결정 원문은 메모리 `home_and_stock_workspace_decisions_1007`.
- 앞으로 작업 끝 배포는 **묻지 않고 한다** — 절차 `AGENTS.md` 「Deploy after changes」, 메모리 `auto_deploy_after_changes`.
- 애널리스트 이력·리포트 표(099)는 비어 있다(기존 최신값 1점만 이관). 채워지는 시점: 해외 03:00 `ext_metrics_overseas`(하루 1,500종목, 약 5일에 한 바퀴), 국내 06:30 `analyst_domestic`(리포트는 첫 실행에 바로, 이력은 값이 바뀐 날만).

- 10-09 적금 탭에 「정부 정책상품 · 청년미래적금」 강조 카드(파란 띠·큰 버튼, 정기예금 탭엔 적금 탭으로 안내하는 띠)(14개 은행, 은행연합회 공시 기준일 2026-10-09). 데이터는 `fe/src/lib/policy-products.ts` 수기 — 갱신은 은행연합회 `receiving_youth_future_2.php`를 다시 읽어 이 파일만 고친다(토스뱅크는 공시 전). 다른 정책상품(청년도약계좌 등)은 미추가. 배포 끝.

- 10-09 요청 9건(종목 화면 통일·실적 카드·레딧 탭·자산 미리보기·홈 브리핑 탭·금리 TOP·레딧 카드·보조 문구) 구현. 결정은 메모리 `home_briefing_tabs_and_earnings_cards_1009`. **배포 끝**(10-09 04:36 KST, api·fe·worker 4). 같은 작업 트리에 다른 세션(금융 UI/UX·AI, 메인 화면)이 동시 작업 중 — 배포는 서로 확인 뒤.

- 10-09 요청 6건(시안 `docs/mockups/requests-1009-v1.html`, 결정 메모리 `requests_1009_decisions`) **전부 구현·배포**:
  ①실적 AI 요약 ②스크리너 A(프리셋 카드 `/stocks/screen/presets` · 조건 칩 · 보기 탭) ③변화 시총 순위 ④뉴스 카테고리·자산 칩·「실시간 국내/해외 뉴스」
  ⑤주제 기사→부동산·금리·예적금·대출 브리핑 입력 + 시군구 근거(점수식 불변) ⑥「금융상품 분석」(예·적금 분석·대출 분석·금리 동향, 계산기·비교 창 삭제, 맨 위 AI 시장 분석 `brief` fin_savings/fin_loans).
- 후속(같은 날): 주제 검색어 → 관리자 키워드 표(`news_poll_keywords.topic`, migration 101 · 뉴스 관리 화면 「종류」), 헤더 검색에 아파트·금융상품·금리·지역(`/search/assets`),
  아파트 단지 목록 `re_complexes`(4.3만, 띄어쓰기·지번·도로명·전국 검색 — 점수 단지 1.3만만 찾던 문제). 도로명(`re_trades.road`) 백필: 10-09 아파트 매매 6개월 이전 구간 7,366개의 수집 표식을 지워 **10-10 03:30 re_trades**가 다시 받는다(평소 1만 + 7.4천 호출, 한도 6만 · 남으면 다음 밤). 그 뒤 09:50 asset_scores가 단지 목록에 도로명을 싣는다.
- 10-10 시·도·전국 미리보기(`kind=sido`, 키 = 법정동 앞 두 자리 · 전국 00) + 부동산 제목 시·도 칩(「경기」 단독 제외) — 부동산 기사 칩 11→75/284.
- 도로명 백필 1회차(10-10 03:30): 15,000호출, 남은 구간 9,424 → 10-11 밤에 이어서. 아파트 매매 행 중 도로명 10.5%.
- 자산 이름 색인(태그·미리보기·헤더 검색)은 이제 **단지 목록 전체**(점수 없는 단지 포함), 기사 칩은 전국 유일 법정동·단지명(동명 단지는 거래 20배 이상 우세할 때)까지 — 「강남아파트」류 일반어·해외 금리 기사는 거른다.

- 10-09 검색 색인 개선(GSC 색인 46 · 미색인 1,040) **전부 구현·배포**, 결정 메모리 `seo_indexing_decisions_1009`: 색인 목록 국내 500+미국 500(이전 34/966) · 허브 `/stocks`(SSR 링크 1,000, 사이드바 「종목 모음」) · 종목 페이지 접힌 요약 바(지표·최근 변화·같은 섹터 링크 8) · 목록 종목명 `<a href=/stocks/…>`(실시간 차트·스크리너·변화·뉴스 칩, 일반 클릭은 그대로) + 데스크톱 선택 push(뒤로=직전 종목) · 작성자 페이지 SSR·색인(공개 리포트 있을 때) · 사이드바 약관 링크 · 옛 주소 6개 308 · 사이트맵 요청 시 렌더(빌드 때 빈 판이 구워지던 결함).
  효과 확인은 2~4주 뒤 GSC 「색인이 생성되지 않은 이유」(발견됨↓ = 링크 효과, 크롤링됨↓ = 본문 효과). 색인 비율이 오르면 범위 확장(2,000→3,000) 검토.

- 10-09 2차 요청 11건 **구현·배포 끝**(결정 메모리 `requests_1009b_decisions`): 작성자·관리자 수정/삭제(리포트는 완전 삭제 `DELETE /research/calls/{id}`, `middleware.MarkAdmin`) · 문장으로 찾기 전면 제거(LLM 잡 `screener_phrases`·버킷 `llm_feat_screener` 포함, 표 `screener_phrase_*`는 DROP 승인 전까지 남김) · 스크리너 시장 탭 왼쪽 위·아이콘·열 순서·60일(`technical.return60d`, **10-10 00:25 점수 배치부터 채워짐**) · 뉴스 칩 「+N」만 뜨던 폭 버그 · 홈 공매 카드 제거 · 레딧 카드 좌우 재배치(인기 글 8) · 실적 AI 카드 꼬리말 제거 · 글 주제 `topic`(migration 102 — 커뮤니티·리포트·메모장, 비주식 리포트 = status `note` 판정 없음) · 관심 목록 탭(종목/아파트·지역/금융상품).
  성능: 스크리너 프리셋 5.9초→1ms(점수일 캐시 + API 20분 예열) · `/home/assets` 0.83초→70ms(공매 300행 조회 제거 + 5분 캐시).
  워커 4개 모두 api와 같은 이미지(worker-market은 배치 종료 뒤 재생성 완료).

- **10-10 사업화 파이프라인** — 단계·상태표는 `docs/BUSINESS_REVIEW.md` §9. 결정 메모리 `hidden_features_1010`.
  W1 **비노출**(부동산·공매·금융상품 예적금/대출/금리·커뮤니티·리포트·스탬프·목표가 예측): 스위치 하나 `fe/src/lib/hidden-features.ts` — 메뉴·홈 카드·종목 버튼·내 기록 칩·보유 자산 칸·관심 탭·헤더 자산 검색·기사 자산 칩·메모 주제·사이트맵, 직접 주소는 `proxy.ts`가 홈으로 307. 데이터·API·배치는 그대로 돈다. 뉴스 분류 칩(부동산·금리 등)은 기사라 남김.
  W2 **데이터 신뢰**: 국내 비율 TTM(`stock_score_ttm.go`, 판 2026-10-10 — **10-11 00:25 점수 배치부터**) · 「나스닥/다우/러셀」 이름 태깅 차단 + 재태깅 v10(배포 직후 완료, 미국 브리핑 언급 상위에서 NDAQ 빠짐) · 「AI 점수 순위」 + 「투자 권유 아님」 · 머리 가격 「시간외(NXT) 포함 최종가」 · 요약 입력 `perStanding` 등(다음 요약 배치부터) · 시나리오 「범위 포함률」+범위 폭. **배포 끝**(10-10 10:3x KST).
  W2~W4 2차(10-10 15:2x KST 배포): 약관 점검표 `docs/DATA_SOURCES_LICENSE.md` · D5는 결함 아님(A 이상 ≈2.5%) · 서버 렌더 SEO — 종목 요약 바 실적 발표 줄, 실적 일정 2주 목록(`earnings-seo-list.tsx`), 「변화」 서버 목록 + `/changes/[date]` 보관(사이트맵, 없는 날 noindex — 상태코드는 셸 때문에 200), 종목 공유 카드 `stocks/[symbol]/opengraph-image.tsx`(폰트 `fe/public/fonts`, satori는 **숫자 자식이면 오류** → String()) · 「지난 발표」에 예정 행이 섞이던 결함(`answerEarnings`). 종목 화면 A안 **배포**(10-10 16시대): `stock-overview-tab` 순서 = 왜 움직였나(`stock-overview-parts`) → 점수 카드(레이더 `score-radar` · 「그룹 N개 중 M위」 = 배치가 inputs에 `peerRank/peerSize`, **10-11 00:25부터**) → 다음 실적 | 90일 → 관심 등록 안내 → AI 요약(**접지 않음**, 사용자 지시) → 질문 칩 → 애널리스트. 머리 가격에 「정규장 N」. `/analysis?backtest=1`로 성적 열기. 홈 비로그인 블록 v2 A안 **배포**: `home-guest-showcase.tsx`(서버) + `home-guest-client.tsx` + `lib/home-guest.server.ts`, `app/(main)/page.tsx`가 `qp_access`·`qp_refresh` 쿠키가 없을 때만 받는다(10분 캐시). 대표 종목 고정 6(`GUEST_FEATURED`), 실적은 시장별 시총 순(응답 시총이 원·달러 혼합), 성적 숫자 그대로 공개. 모바일은 히어로 오른쪽 카드 숨김.

- 10-10 후속 4건 **배포 끝**(api·fe·워커 4, 워커는 도는 잡 끝난 뒤): ① 종목 AI 요약 대상 = 실시간 차트(거래대금∪거래량 top100, ETF 제외 ≈109) + **AI 랭킹 종합 오늘 top100**(회전 폐지) + 보유·관심, 상한 `llm_settings.daily_cap` 350→300 — 첫 적용 10-10 17:10 국내 · 07:40 해외 ② 번역 `TRANSLATE_PROVIDER=azure,local`(Azure 하루 몫 = 남은 월 한도 ÷ 남은 날, 넘치면 로컬) ③ migration 103 DB 정리 ④ 앱 안 알림(migration 104 `user_notifications`·`notify_cursors`, api 30분 루프, 헤더 종) · 자산 화면 「이 자산 이야기」(`/assets/posts`) · 스크리너 프리셋 「어제 대비 ▲▼ · 새로 든 종목」.
  ①의 자산 카드·단지/상품 알림은 **같은 날 다른 세션이 부동산·금융·커뮤니티 경로를 숨겨(`fe/src/lib/hidden-features.ts`) 지금은 안 보인다** — 종목 알림·프리셋 변화는 보인다.

## BLOCKERS
- 없음 (10-10 05:37~06:11 KST 운영 MySQL80 크래시 — mysqld 예외 0x80000003, 사용자가 서비스 재시작. 원인 미확정: 오류 로그 `JJMIN.err`는 관리자 권한 필요)

## PENDING — 사용자 승인/행동 필요
- MySQL80 자동 재시작(관리자 PowerShell: `sc.exe failure MySQL80 reset= 86400 actions= restart/60000/restart/60000/restart/60000`) · 워치독 알림 웹훅(`deploy/watchdog.config.json`) — 10-10 크래시 때 `/health`는 정상이라 아무도 몰랐다.
- 로고 누락 수정(10-03 밤)의 API 쪽: api 컨테이너가 10-03 21:35에 재생성됐는데 로고 변경을 포함했는지는 미확인 → LITE·CLS 로고 URL을 curl로 확인.
- 행안부 법정동코드 활용신청(사용자). 신청 전에는 전국 아파트 없이 서울만 나오고, 세종 시군구도 없다.
- 운영 DB 변경(사용자 판단): `088` financial_periods 중복 인덱스 제거, `ranking_snapshot_entries` DROP(덤프 `be/backups/ranking_snapshot_entries_20260926.sql.gz`). (10-09에 제안한 중복 인덱스 4개·문장 검색 표는 10-10 migration 103으로 처리 끝.)
- Cloudflare: sitemap.xml 캐시 규칙, 근본책은 Tunnel 또는 HTTP/2 to Origin 중 결정(09-25 후속4).
- 성장 감시 비밀값: GA4 서비스 계정 JSON(`be/secrets/`), `GROWTH_GA4_PROPERTY_ID`, `GROWTH_BING_API_KEY`, GA 내부 트래픽 필터.
- 아침 요약 이메일(09-26 제안 D): 발송 서비스와 개인정보처리방침 결정 대기.
- Reddit RSS 이용이 약관 회색지대인 문제(사용자 판단).
- 제안 대기(09-28 ⑥·⑦): TSM 밸류에이션(Finnhub `/stock/metric`), 국내 목표가(Naver `consensusInfo`).

## DECISIONS (현재 영향 있는 것만 — 원문은 메모리)
- 10-03 자산 AI 범위: 홈 B, R1~R4, F1~F4 전부, 태그 A(공매 제외) → 메모리 `asset_ai_tags_decisions_1003`
- 09-28 데이터 태그와 스크리너, 09-27 커뮤니티·아이디·AI → 메모리 `data_tags_and_screener_decisions_0928`, `community_and_ai_decisions_0927`
- 커밋: 마지막 커밋은 `7e60d32`(09-15). 그 뒤 전부 미커밋이고 작업 트리 변경이 약 1,200개다. **stash·checkout·reset 금지**, 커밋은 사용자가 요청할 때만.

## CONSTRAINTS (원본 위치만)
- 프로젝트 규칙은 `AGENTS.md`(Docker 재시작, 시작·종료 프로토콜, 출력 규칙)
- 저장·캐시·LLM·목록 API 원칙은 `docs/PRINCIPLES.md`(해당 영역을 건드릴 때만 읽는다)
- 운영 함정은 메모리 색인(MEMORY.md, 자동 주입)에서 제목으로 찾는다
- api·워커 재시작과 fe 재빌드는 작업 끝에 자동(10-07 상시 승인). DROP·데이터 삭제·외부 계정·nginx/Cloudflare는 사용자 승인 후에만

## VERIFY (10-10 2차 실측)
- 서버 HTML 종목 링크: /earnings/us 60 · /changes/2026-10-08 60 · AAPL 요약 바 「다음 발표: 10/29」 · OG 005930·AAPL 200 image/png(약 70KB) · /changes/2026-01-01 noindex · health 200 · panic 0 · 워커 4 = api 이미지 · tsc·vitest 500 통과

## VERIFY (10-10 파이프라인 W1·W2 실측)
- 공개 도메인: /community·/finance/savings → 307 홈 · 사이트맵 1,012(숨긴 주소 0) · 홈 「AI 점수 순위」·금리 TOP 없음·`/home/assets` 요청 0·콘솔 오류 0 · /stocks/005930 버튼 「보유종목 등록」 하나·이야기 카드 없음·「시간외(NXT) 포함 최종가」
- go test service·llm·handler·repository 통과(Toss 웹소켓 통합 시험은 자격 401로 한 번 흔들림, 재실행 통과) · vitest 504 · tsc 통과 · 손댄 파일 eslint 신규 0
- 배포 중 api 네트워크 0개 재발 → 무빌드 재생성으로 해결(메모리 api_container_network_detach). 워커 4 = api 이미지
- 로그인 화면(보유 자산 칸·관심 탭·메모 주제)은 실계정이 없어 육안 미확인

## VERIFY (10-09 색인 개선 실측)
- seo/stocks 국내 500·미국 500(셀트리온·카카오·삼바 포함) · 서버 HTML 종목 링크: /stocks 1,000 · /market 100 · /screener 66 · /news/domestic 14 · 종목 페이지 8 (/changes는 CSR이라 0)
- 종목 페이지 서버 본문 209자 → 약 3,900~4,100자 · 사이트맵 배포 직후에도 1,024 · 공개 도메인 /stocks 링크 1,000 · panic 0
- 브라우저: /market 클릭 → 패널(이동 없음)·뒤로 → 직전 종목·한 번 더 → 1위 · 375px /stocks·/stocks/005930 가로 넘침 0
- fe 냉시동 첫 `GET /`의 transformAlgorithm 오류는 09-22부터 있던 것(이번 변경 무관)

## VERIFY (10-09 실측)
- /assets/peek region·product·rate 200, 잘못된 kind 400 · /home/assets products 3묶음(평균 포함) · 375px 실적 탭·실적 일정 가로 넘침 0 · /stocks/NVDA?tab=chart → 개요, 레딧 탭 보임 · vitest 512 · go test handler/service 통과
- 배포 직후 api 네트워크 0개 재발(메모리 api_container_network_detach) — 무빌드 재생성으로 해결

## VERIFY (10-07 실측)
- 10-07 10:1x KST 배포 실측: api·fe healthy, nginx 홈 200, api 로그 panic 0, worker-stock/market/content 이미지 = api 이미지(worker-asset은 코드 무관이라 3일 전 그대로)
- `/quotes/indices` 16종(SPY·DIA 없음), 국고채 3·10년 토스 실시간(3.931 · live), 099 마이그레이션 적용 로그 확인
- 운영 /stocks/000660: 커뮤니티 탭 없음(글 0 · 리포트만 있음), 애널리스트 카드·90일 시나리오 보임
- 시험: go test service·handler·repository·marketdata 통과, vitest 509 통과, tsc 통과, 손댄 파일 eslint 신규 0
- 로그인 상태 화면(내 종목 위치·세션 만료 뒤 비로그인 전환)은 실계정이 없어 육안 미확인

## NEXT
0. 10-11 아침: 점수 로그 `score: domestic TTM on N symbols`(N이 국내 수천이어야) · 005930 PER이 39.8에서 내려왔는지 · 「오늘의 변화」에 방식 변경 급변이 안 섞였는지(`|ttm` 판). 다음 요약 배치 뒤 「PER 상위 N%」 문구가 사라졌는지
0. 사업화: 홈 비로그인 블록·종목 A안 실사용 반응 보기(GA4 연결 뒤 홈→종목 클릭률) · 10-11 종목 점수 카드에 「N개 중 M위」가 뜨는지(005930) · 같은 회사인데 ADR과 본주 점수가 크게 다르다(SK하이닉스 52 vs SKHY 73 — 비교 집단이 달라서) — 표시 방식 검토 · W3(사용자): GSC·GA4·Bing, 커밋·백업(그 뒤 §7 정리), 서버 이전 · 카톡/X 공유 미리보기로 종목 카드 실기기 확인
0. 10-10 17:10 국내 요약 결과(`bySource` radar/score 각 ≤~100 · 대상 ≤300 · KR 토큰)와 하루 Azure 번역량(`apibudget:v1:azure_translate:tokens:day:*`). 로그인 상태로 헤더 종 알림 육안 확인.
0. 10-09 2차 요청 로그인 상태 육안 확인(실계정 없음): 커뮤니티 글쓰기 주제 · 리포트 쓰기 「부동산/금융상품」 분석 리포트 발행·상세 · 관리자 계정으로 남의 글 수정/삭제 · 메모장 주제·자산 태그 · /watchlist 탭. 10-10 오전 스크리너 60일 열이 채워졌는지. 10-11 05:10 fin_products 성공(금감원 정전 10-08 18:00~10-10 24:00).
0. 10-09 개편 육안 확인: /screener(프리셋·칩 편집·보기 탭, 375px) · /finance/savings·loans(AI 시장 분석·시장 상위 %) · 헤더 검색 아파트 · /news/domestic 칩. 10-10 오전 re_trades 메시지(호출 수·남은 구간)와 「동대구로 345」 헤더 검색 확인
0. 홈(브리핑 3탭·금리 TOP·레딧 카드)·종목 실적 탭·실적 일정 카드 보기를 375px·768px·1440px에서 육안 확인(로그인 상태 포함)
1. 10-08 오전: 애널리스트 카드에 리포트 목록(국내)·월별 분포(해외) 채워졌는지 운영에서 확인(005930 · AAPL)
2. 로그인 화면 육안 확인(내 종목 위치, ★·내 대출·내 금리·글쓰기 태그)

## BACKLOG (미확인 포함 — 상세는 `Grep "<날짜>" docs/archive/sessions.md`)
- 점수: 섹터 컬럼(DART `induty_code`/SEC `sic`)으로 피어그룹 교체 · 예측력 IC 검증(40거래일 축적 후) · 레딧 성분 IC 검증 후 반영 여부(09-29)
- 지수 이력: btc·gold 백필 소스 없음 · usd-krw(Toss 지표캔들 400)
- 확장: Redis Pub/Sub SSE fan-out · 폴러 리더 선출/샤딩 · 프로세스 간 공용 리미터(09-11 #7 나머지) · prod migration(goose)
- FE: `source=db`/장 마감 배지 · 카드 배지 자정을 넘긴 승계 「어제 21:00」(09-25) · `market/layout.tsx` 메타데이터 정리(09-22) · 도달 불가 `calendar/page.tsx`(09-20 후속)
- 배치: 상류 5xx로 실패한 잡 재처리 구멍(09-20) · worker-filings SlotGate 누락(09-29) · 05:10 금융상품 수집이 하루 한 번만 도는지(10-01)
- 금융상품: 금감원 사이트 수치와 2~3개 대조 · 법률 검토(`docs/FINANCE_PRODUCTS.md` §8)
- 커뮤니티: 글 수정 화면에 태그 도우미 없음 · 이미지 캡션·수정 · 예측·리포트 붙이기(2차) · e2e 스위트 실행 · Safari 실기기(홈 카드)
- 번역: Azure F0 한도 대비 실측 수요 월 2.17M자(09-16)
- 정리: e2e 테스트 계정 `qae2ewealth1001` 등은 정리 대상(메모리 `e2e_accounts_in_users_table`)
