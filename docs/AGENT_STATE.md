# Agent State

> 현재 상태만 담는다. **150줄 이하, 매 세션 덮어쓴다(앞에 쌓지 않는다).** 이력은 `docs/archive/sessions.md`, 원칙은 `docs/PRINCIPLES.md`, 결정·함정은 메모리.
> 갱신: 2026-10-12 오후 (요청 7건: 뉴스 칩·메뉴 분리·탭 줄·반응형 점검·앱 검토)

## NOW
- **코드는 GitHub에 있다** — `jjmin333/quant-port` public, `master`. 10-10에 09-15 이후 미커밋 전부를 3개 커밋으로 push(비밀값 점검 끝: `be/.env`·`be/secrets/`·워치독 설정은 git 밖). 이후 작업도 커밋·push 한다. 클라우드 세션 규칙은 `AGENTS.md` 「Cloud sessions」(배포는 PC 세션만).
- 사업화 단계표는 `docs/BUSINESS_REVIEW.md` §9(+「2차 검토 반영」), 경쟁사는 `docs/COMPETITORS.md`.
- **메뉴**(10-10 결정 A + 10-12 요청 3, 메모리 `menu_sections_1010`): 실시간 차트 · **AI 점수 순위** · **종목 스크리너** · **오늘의 변화**(내 종목 배지) · 뉴스(국내·해외) · 실적(국내·해외·서프라이즈) + 보유·관심·메모장. 업종·스타일만 메뉴에서 보류(주소·사이트맵 산다). 태블릿 768~1023px에 접힌 아이콘 레일(햄버거는 드로어).
- **커뮤니티·투자 리포트 복원(10-12)**: `hidden-features.ts` 목록에서 제거 + robots `/api/v1/research|community/` Allow. 부동산·금융상품·목표가 예측은 계속 숨김. 탭 줄 `section-tabs.tsx`는 이제 뉴스·실적만, 14px·36px.
- **뉴스 칩(10-12)**: 국내 칩 전체·주식·부동산·「금리·예금·대출」(필터 값 `finance` → BE `topic IN (rates,deposit,loan)`). 홈 실시간 뉴스 기본 칩 = 주식. 뉴스 Pill은 `buttonVariants` 없이 직접 색(다크 덮어쓰기·그림자 테두리 잘림 수정).
- 롱테일 SSR 페이지(사이트맵 포함): `/stocks/sector/<kr|us>/<업종>`(22) · `/stocks/style/<slug>`(6, 이름표 `lib/stock-style.ts`) · `/compare/<a>-vs-<b>`(정렬된 쌍 하나, 사이트맵은 업종 1위×2~5위 79개, 종목 페이지가 가까운 순위 셋과 링크) · `/earnings/surprises`(색인 대상만). 점수 일괄은 `fetchIndexableScored`(batch 200씩 5회, 1시간 캐시).
- 종목 서버 본문(접힌 요약 바)에 AI 요약 문단·다섯 축·스타일 이름표·업종 순위·비교 링크 추가. 홈 title/설명/OG가 점수 포지셔닝 문구.
- worker-asset 예약 7→3: `re_trades`·`onbid`·`asset_scores`·`asset_ai` 중지(예약 주석 + 감시표 manual + catch-up 허용 목록 제외 — 셋 다 해야 워치독이 되살리지 않는다). 금융상품·R-ONE·ECOS는 돈다. 되살리는 법은 `be/internal/workers/asset/asset.go` 주석.
- KRX 휴장일 목록 보정(`calendar/holidays_krx.json` +12일: 2025·2026 근로자의 날·선거일·한글날, 2026-08-17, 2027 대체휴일·부처님오신날). 전에는 10-09를 거래일로 보고 전 종목 종가 재수집(4,278)을 반복했다.
- **홈 비로그인 블록 축소(10-11)**: 대표 종목 카드 6장·오늘 바뀐 종목·다가오는 실적·소개 카드 3장 제거, 히어로(질문·검색·종목 칩·대표 레이더)만 남김. 지수 흐름 띠(`IndexTickerStrip`)를 홈 맨 위로. 서버 선반입도 대표 점수 하나만(변화·실적·백테스트 요청 제거).
- 화면 문구: 90일 시나리오 폭 ≥60%면 접음 · 애널리스트 카드 「증권사 의견, 권유 아님」 · 브리핑 금리 칩 bp.

- **Toss 스펙 감시**: `.github/workflows/toss-spec-watch.yml`이 매일 09:00 KST 공식 openapi.json·asyncapi.json을 `be/docs/toss-*.json` 기준본과 비교해 다르면 GitHub 이슈를 연다(판마다 한 번). 처리 = 기준본 갱신·영향 검토·커밋. 기준본 1.2.24(REST)·1.2.2(WS).
- 10-11 로그 개선: 시세 `GetPrices`가 스펙 밖 심볼(BAC^B·BRK/B, 카탈로그 400개) 제외 · 폴러가 404 stock-not-found도 격리 · DART corpCode 받기 실패 시 옛 목록 유지 + 10분 재시도 간격(점검 800 중 종목마다 재다운로드하던 것) · EDGAR 「분기 없음/facts 없음」은 not_found(30일 쿨다운, 은행·무매출 바이오·펀드 1,393종목).

## BLOCKERS
- 없음

## PENDING — 사용자 승인/행동 필요
- **측정 연결(3주+)**: GSC 서비스 계정 JSON(`be/secrets/`)·`GROWTH_GA4_PROPERTY_ID`·`GROWTH_BING_API_KEY`·GA 내부 트래픽 필터 · 네이버 서치어드바이저 사이트맵 제출(새 페이지 포함).
- 워치독 알림 웹훅(`deploy/watchdog.config.json` `WebhookUrl`) — DB 장애는 이미 감지(10-10 06:05 로그)하지만 알릴 곳이 없다. MySQL80 자동 재시작(관리자: `sc.exe failure MySQL80 reset= 86400 actions= restart/60000/restart/60000/restart/60000`).
- 소셜 로그인 앱 등록(카카오·네이버·구글) · 알림 채널(이메일/텔레그램) 결정.
- 운영 서버 이전(OCI) · 데이터 원천 법률 검토(`docs/DATA_SOURCES_LICENSE.md`: Toss 시세 재배포, 네이버 컨센서스, FnGuide, Reddit) · 사업자 등록 후 개인정보처리방침 운영자 정보.
- 행안부 법정동코드 활용신청 · 운영 DB 변경(`088` 중복 인덱스, `ranking_snapshot_entries` DROP) · Cloudflare sitemap 캐시·Tunnel 결정.
- 외부 유통(텔레그램 채널·네이버 블로그) 게시 승인.

## DECISIONS (현재 영향 있는 것만 — 원문은 메모리)
- 10-10 메뉴 통합 A → `menu_sections_1010` · 기능 비노출 → `hidden_features_1010` · 종목 A안·홈 비로그인 → `stock_page_a_home_guest_1010` · 요약 대상·번역 → `llm_targets_and_translation_1010`
- 커밋: 이제 정상 커밋한다. **stash·checkout·reset 금지**는 유지(동시 세션).

## CONSTRAINTS (원본 위치만)
- 프로젝트 규칙 `AGENTS.md` · 저장·캐시·LLM·목록 원칙 `docs/PRINCIPLES.md` · 운영 함정은 MEMORY.md 색인
- 배포 스크립트 출력을 파이프로 받지 말 것(메모리 `deploy_script_pipe_breaks_it`) — 10-10에 한 번 어겼고 nginx는 무사했다

## VERIFY (10-12 오후 실측)
- go test service·repository·handler·model 통과(service 첫 실행 1회 실패, 재실행 2회 통과 — 병렬 실행 간섭 의심) · vitest 512 · tsc 통과
- 반응형: 14화면 × 360/768/1024/1440 가로 넘침 0(공개 도메인, 배포 전). 768에서 순위 목록이 전폭으로 늘어나는 것은 남김

## VERIFY (10-11 저녁 실측)
- go test ./... 통과(낡은 휴장일 기대값 3건 08-17→08-18 정정) · 배포 api+워커 4(이미지 일치), health 200, panic 0, api 네트워크 1

## VERIFY (10-10 저녁 실측)
- go test ./... 전부 통과(calendar 휴장일 시험 추가) · vitest 510 · tsc 통과 · 손댄 파일 eslint 신규 0(남은 set-state-in-effect 3건은 기존)
- 배포: api·fe·워커 4(= api 이미지), health 200, panic 0, api 네트워크 1, worker-asset 「3 jobs」
- 공개 도메인: 홈 title 새 문구 · /stocks/005930 서버 본문에 AI 요약·다섯 축·비교 링크 · /stocks/sector/kr/IT 120행 · /stocks/style/quality-growth 133행 · /compare/000660-vs-005930 레이더 겹침 · /earnings/surprises 56행 · 없는 업종/비교는 noindex · 사이트맵 1,131(업종 22·스타일 6·비교 79·서프라이즈 1 추가)
- 브라우저: 사이드바 7칸, 탭 줄(종목 찾기·실적) 데스크톱·375px 가로 넘침 0, /analysis 분할 화면 세로 넘침 0

## NEXT
- 앱 출시·메뉴 이름 복원은 사용자가 진행 안 함(10-12). 앱인토스는 정책 3-5·3-6으로 핵심 기능 입점 불가
- 커뮤니티·리포트 복원 뒤: 사이트맵에 다시 들어갔는지, 신고 관리 화면 동작, 스팸 글 유입 감시
- 10-12 09:00 이후: Actions 「Toss API spec watch」 첫 cron 실행 성공 확인(이슈 없음이 정상) · 10-12 09:00 financials_overseas 실패 수가 161에서 크게 줄었는지 · 10-12 20:00 insider_domestic·22:00 financials_domestic 정상(10-09·10은 DART 점검 800으로 전 종목 실패)
- Toss 새 sectors API 5종(1.2.24) — 지금 DART/SEC로 하는 업종 분류를 대체할지 검토(사용자 결정)
- 홈 `/` React #418(하이드레이션 텍스트 불일치) 하루 ~9건 — 일부는 가짜 UA 봇, 시간 의존 문구 의심
0. 10-11 아침: 점수 로그 `score: domestic TTM on N symbols` · 005930 PER이 39.8에서 내려왔는지 · 「오늘의 변화」에 방식 변경 급변이 안 섞였는지 · 점수 카드 「N개 중 M위」 · 요약 배치 뒤 「PER 상위 N%」 문구가 사라졌는지
1. 10-12(월) 장 마감 뒤: 국내 종가 자동 수집이 정상(재수집 반복 없음)인지 worker-market 로그
2. 국내 서프라이즈 「개별 기준」 행 원인 — FnGuide 잠정실적에 연결 행이 없는 종목인지, 예상치가 개별/연결 어느 쪽인지 확인 후 BE에서 거를지 결정
3. 2~4주 뒤 GSC: 새 롱테일 페이지 색인 여부(측정 연결이 먼저)
4. 로그인 상태 육안 확인(실계정 없음): 헤더 종 알림, 보유·관심 화면, 메모장 주제

## BACKLOG (상세는 `Grep "<날짜>" docs/archive/sessions.md`)
- 점수: 컨센서스 리비전·깜짝실적 축(IC 90거래일 뒤) · 레딧 성분 IC · ADR과 본주 점수 차이 표시(SK하이닉스 52 vs SKHY 73 — 비교 집단 차이)
- 종목 「전망」 탭(목표가 추이·추정 EPS·범위) — 국내 목표가 라이선스 먼저 · 시장 히트맵 · 날짜별 상위 점수
- 지수 이력: btc·gold 백필 소스 · usd-krw
- 확장: Redis Pub/Sub SSE fan-out · 폴러 리더 선출 · 프로세스 간 공용 리미터 · prod migration(goose)
- 배치: 상류 5xx 실패 잡 재처리 구멍 · worker-filings SlotGate 누락
- 정리: e2e 테스트 계정(메모리 `e2e_accounts_in_users_table`) · 메뉴에서 내린 옛 화면(`/targets`·`/stamps`·`/dashboard`)
