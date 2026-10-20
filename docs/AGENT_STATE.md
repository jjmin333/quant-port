# Agent State

> 현재 상태만 담는다. **150줄 이하, 매 세션 덮어쓴다(앞에 쌓지 않는다).** 이력은 `docs/archive/sessions.md`, 원칙은 `docs/PRINCIPLES.md`, 결정·함정은 메모리.
> 갱신: 2026-10-20 밤 (전체 점검: 변화 감지 컬럼 폭 · 내부자 거래 발행사 검증 · research 이중 실행 · CI vitest)

## NOW
- **코드는 GitHub에 있다** — `jjmin333/quant-port` public, `master`. 10-10에 09-15 이후 미커밋 전부를 3개 커밋으로 push(비밀값 점검 끝: `be/.env`·`be/secrets/`·워치독 설정은 git 밖). 이후 작업도 커밋·push 한다. 클라우드 세션 규칙은 `AGENTS.md` 「Cloud sessions」(배포는 PC 세션만).
- 사업화 단계표는 `docs/BUSINESS_REVIEW.md` §9(+「2차 검토 반영」), 경쟁사는 `docs/COMPETITORS.md`.
- **메뉴**(10-10 결정 A + 10-12 요청 3, 메모리 `menu_sections_1010`): 실시간 차트 · **AI 점수 순위** · **종목 스크리너** · **오늘의 변화**(내 종목 배지) · 뉴스(국내·해외) · 실적(국내·해외·서프라이즈) + 보유·관심·메모장. 업종·스타일만 메뉴에서 보류(주소·사이트맵 산다). 태블릿 768~1023px에 접힌 아이콘 레일(햄버거는 드로어).
- **커뮤니티·투자 리포트 = 전체 공개(10-13, 10-12~13 관리자 전용이었다)**: 스위치는 BE `router.go` communityGate(지금 통과, 닫으려면 RequireAdmin) + FE `ADMIN_ONLY_ROUTE_PREFIXES`(지금 빈 목록 — 메뉴·사이트맵·`AdminOnlyGate`·`useCommunityVisible`이 따른다) + `robots.ts`. 기존 글(seed-preview 시험 글 포함)은 그대로 공개됐다. 리포트 검토 노트는 LLM 없이 규칙 노트만. 부동산·금융상품·목표가 예측은 계속 숨김. **커뮤니티·리포트 주제도 주식만**(10-13): `ASSET_FEATURES_HIDDEN`이면 `TopicPicker`가 안 그려지고 목록 주제 필터·`?topic=`도 무시(기존 글 전부 stock). 탭 줄 `section-tabs.tsx`는 뉴스·실적만, 14px·36px.
- **뉴스 칩(10-12)**: 국내 칩 전체·주식·부동산·「금리·예금·대출」(필터 값 `finance` → BE `topic IN (rates,deposit,loan)`). 홈 실시간 뉴스 기본 칩 = 주식. 뉴스 Pill은 `buttonVariants` 없이 직접 색(다크 덮어쓰기·그림자 테두리 잘림 수정).
- 롱테일 SSR 페이지(사이트맵 포함): `/stocks/sector/<kr|us>/<업종>`(22) · `/stocks/style/<slug>`(6, 이름표 `lib/stock-style.ts`) · `/compare/<a>-vs-<b>`(정렬된 쌍 하나, 사이트맵은 업종 1위×2~5위 79개, 종목 페이지가 가까운 순위 셋과 링크) · `/earnings/surprises`(색인 대상만). 점수 일괄은 `fetchIndexableScored`(batch 200씩 5회, 1시간 캐시).
- 종목 서버 본문(접힌 요약 바)에 AI 요약 문단·다섯 축·스타일 이름표·업종 순위·비교 링크 추가. 홈 title/설명/OG가 점수 포지셔닝 문구.
- worker-asset 예약 7→3: `re_trades`·`onbid`·`asset_scores`·`asset_ai` 중지(예약 주석 + 감시표 manual + catch-up 허용 목록 제외 — 셋 다 해야 워치독이 되살리지 않는다). 금융상품·R-ONE·ECOS는 돈다. 되살리는 법은 `be/internal/workers/asset/asset.go` 주석.
- KRX 휴장일 목록 보정(`calendar/holidays_krx.json` +12일: 2025·2026 근로자의 날·선거일·한글날, 2026-08-17, 2027 대체휴일·부처님오신날). 전에는 10-09를 거래일로 보고 전 종목 종가 재수집(4,278)을 반복했다.
- **홈 비로그인 블록 축소(10-11)**: 대표 종목 카드 6장·오늘 바뀐 종목·다가오는 실적·소개 카드 3장 제거, 히어로(질문·검색·종목 칩·대표 레이더)만 남김. 지수 흐름 띠(`IndexTickerStrip`)를 홈 맨 위로. 서버 선반입도 대표 점수 하나만(변화·실적·백테스트 요청 제거).
- 브리핑 「많이 언급된 종목」 문턱: **국내 20 · 해외 25**(10-13, BE `briefingMentionMinCount(market)` = FE `MENTION_MIN_COUNT`). 저장된 브리핑은 다음 슬롯부터 20 기준.
- **종목 개요 v2(10-13 밤, 메모리 `stock_overview_v2_1013`)**: 최근 핵심 사건 → AI 요약(개요만 기본 간단) → 점수(레이더 + 다섯 축 막대, 전일 칩은 머리줄) → 다음 실적(확정/추정·예상치·지난 4번 표) | 90일 범위(접지 않음, 문장+막대+검증 세 칸만) → 애널리스트(목표가 세 칸·5단계·결론 문장) → 차트·지표·커뮤니티 → 레딧 카드(해외·자료 있을 때). 관심 등록 안내·질문 칩·`ScoreChangeLine` 삭제. 브리핑 금리 칩 bp.

- **Toss 스펙 감시**: `.github/workflows/toss-spec-watch.yml`이 매일 09:00 KST 공식 openapi.json·asyncapi.json을 `be/docs/toss-*.json` 기준본과 비교해 다르면 GitHub 이슈를 연다(판마다 한 번). 처리 = 기준본 갱신·영향 검토·커밋. 기준본 1.2.24(REST)·1.2.2(WS).
- 10-11 로그 개선: 시세 `GetPrices`가 스펙 밖 심볼(BAC^B·BRK/B, 카탈로그 400개) 제외 · 폴러가 404 stock-not-found도 격리 · DART corpCode 받기 실패 시 옛 목록 유지 + 10분 재시도 간격(점검 800 중 종목마다 재다운로드하던 것) · EDGAR 「분기 없음/facts 없음」은 not_found(30일 쿨다운, 은행·무매출 바이오·펀드 1,393종목).

- **홈 세 카드 높이 맞춤(10-13)**: 점수 카드 바닥 「종가 기준」 줄 → 제목 옆 작은 글씨(카드 폭 30rem 미만이면 숨김), 실적 일정 7줄 보장: 2주 안이 모자라면 100일 창으로 채우고, 그래도 모자라면 홈만 `recentBig`(최근 100일 발표·종목당 최신 1건·시총 순)을 아래에 「N일 전」으로(캐시 키 v6). 1672·1280·800px에서 세 카드 278px·7줄 실측.

- **PWA 1단계(10-14)**: `app/manifest.ts`(→ `/manifest.webmanifest`) · 아이콘 `public/icon-{192,512,maskable-512}.png`(`scripts/gen-icons.mjs`가 굽는다) · `public/sw.js`는 **화면 이동이 네트워크 실패할 때만** `offline.html`을 준다(다른 요청은 안 건드림, navigation preload 켬, offline.html 고치면 `CACHE` 판 올리기) · 등록은 `layout.tsx` 운영 빌드에서만 lazyOnload. 웹 푸시는 안 함(알림 채널 결정 대기). 앱 출시 검토: Flutter 재작성 X → PWA → 안드로이드 TWA/Capacitor → iOS Capacitor+푸시, 그 전에 서버 이전.

- **운영 로그 점검(10-15)**: nginx·api 5xx는 27시간 0건(502 1건은 fe 재기동 순간), 스캐너 400/404뿐. 「503」은 Gemini high demand — ① AI 요약 배치가 503·타임아웃 실패만 끝에서 10분 뒤 한 바퀴 더(`batchPass`, `llm.IsTransient`, 같은 주 모델) ② 지수 해설은 브리핑처럼 503이면 대체 모델(`chatCompletionWithFallback`, worker 풀에 FallbackModel) ③ 번역 사슬이 뒤 엔진이 대답했는데 앞 엔진의 예산 소진을 보고하던 것 → 같은 제목 2분마다 무한 재시도·로그 반복이 원인 ④ Toss 한도 「observed」 로그를 값마다 한 번(MARKET_INDICATOR 10↔5로 하루 400줄). 뉴스 목록 「rebuild slow」(0.5~0.7s, 배경 예열)·fe 「Failed to find Server Action "x"」(봇 탐침)는 둠.

- **봇 초안 · 게시 계정(10-16, 메모리 `bot_drafts_personas_1016`)**: worker-stock `bot_drafts` 잡이 KST 3시간마다(00·03·…·21시) 브리핑 많이 언급된 종목·오늘의 변화·2일 안 실적 프리뷰에서 가중 랜덤으로 커뮤니티 5 · 리포트 2 초안(짧은/긴 글)을 `bot_drafts`에 쌓는다. 미게시 24시간 뒤 삭제, 24시간 안 같은 종목 제외. **10-19부터 LLM 다시 쓰기**(`service/bot_rewrite.go`): 템플릿(facts, 사건·등락에 날짜)을 저장하고 초안마다 **작성자 유형**(`service/bot_archetypes.go` — 커뮤니티: 한 줄·감정형·초보 질문·직장인·뉴스 공유·숫자 정리·신중파·장문 분석, 리포트: 짧은 메모·체크리스트·정석·컬럼·위험 점검, 가중 랜덤, `bot_drafts.archetype`)을 뽑고 게시 계정은 아무거나 배정 → 자산 전용 키 `llm_sub` + 기능 버킷 `llm_feat_bot`(80콜·30만)으로 그 유형대로 다시 씀(유형마다 길이 검증 범위가 다르다 — 한 줄은 5자부터). 계정별 말투(`persona_style`)는 10-19 오후 사용자 결정으로 쓰지 않는다(열은 남김). 출력 숫자가 facts에 없거나(한 자리 정수만 허용) 권유 표현·길이 이탈이면 버리고 note에 이유. 503·시간 초과는 멈추고 다음 회차가 12시간 안 템플릿을 이어 씀. 게시 계정 `badge` = 서비스 뱃지 키 목록(`model.CommunityBadgeCatalog`, 활동 뱃지 계산도 같은 목록), `persona_style` = 말투. 게시 계정 글 조회수는 `/admin/personas/posts/{kind}/{id}/views`로 지정(실제 사용자 글은 404). 관리자 `/admin/bots`(탭 둘) → 「편집해서 게시」가 `/community/new?draft=` · `/reports/new?draft=`를 열고 **게시 계정**(users `role='persona'`, `badge`, `@persona.invalid`, 빈 비밀번호)을 골라 올린다. 쓰기는 기존 핸들러를 `/admin/personas/{id}/as/...`(ActAs, persona만 통과) 아래 재사용 — 쿨다운·리포트 하루 5건·종목 30일 규칙 그대로. 게시 계정은 관리자가 고른 뱃지만(활동 계산 없음, 「운영」 표시 없음 — 사용자 결정). 같은 화면에 게시 계정 글의 7일 댓글 + 그 계정으로 답글.

## BLOCKERS
- 없음

## PENDING — 사용자 승인/행동 필요
- **Cloudflare**: `/api/v1/` 캐시 규칙 Edge TTL을 「헤더 없으면 우회」로(현재 헤더 없는 200을 캐시). 코드는 `DefaultNoStore`로 이미 방어(메모리 `cloudflare_caches_headerless_api`)
- **측정 연결(3주+)**: GSC 서비스 계정 JSON(`be/secrets/`)·`GROWTH_GA4_PROPERTY_ID`·`GROWTH_BING_API_KEY`·GA 내부 트래픽 필터 · 네이버 서치어드바이저 사이트맵 제출(새 페이지 포함).
- 워치독 알림 웹훅(`deploy/watchdog.config.json` `WebhookUrl`) — DB 장애는 이미 감지(10-10 06:05 로그)하지만 알릴 곳이 없다. MySQL80 자동 재시작(관리자: `sc.exe failure MySQL80 reset= 86400 actions= restart/60000/restart/60000/restart/60000`).
- 소셜 로그인 앱 등록(카카오·네이버·구글) · 알림 채널(이메일/텔레그램) 결정.
- 운영 서버 이전(OCI) · 데이터 원천 법률 검토(`docs/DATA_SOURCES_LICENSE.md`: Toss 시세 재배포, 네이버 컨센서스, FnGuide, Reddit) · 사업자 등록 후 개인정보처리방침 운영자 정보.
- 행안부 법정동코드 활용신청 · 운영 DB 변경(`088` 중복 인덱스, `ranking_snapshot_entries` DROP) · Cloudflare sitemap 캐시·Tunnel 결정.
- 외부 유통(텔레그램 채널·네이버 블로그) 게시 승인.
- **원본 서버가 IP로 직접 열린다**(10-20 점검): nginx `server_name _`가 아무 Host에나 사이트 전체를 준다(`Host: 218.233.8.175` → 200, 실제로 `http://218.233.8.175/`에서 클라이언트 오류 보고가 왔다). Cloudflare WAF·캐시를 우회하는 길 — 기본 서버 `return 444` + 공유기 포워딩을 Cloudflare IP로 제한(또는 Tunnel). nginx 변경이라 승인 필요.
- 중복 인덱스 DROP: `financial_periods.idx_financial_symbol_period`(= `uk_financial_period`, 위 `088`) · `research_revisions.idx_rr_call`(= `uniq_rr_call_rev`).

## DECISIONS (현재 영향 있는 것만 — 원문은 메모리)
- 10-10 메뉴 통합 A → `menu_sections_1010` · 기능 비노출 → `hidden_features_1010` · 종목 A안·홈 비로그인 → `stock_page_a_home_guest_1010` · 요약 대상·번역 → `llm_targets_and_translation_1010`
- 커밋: 이제 정상 커밋한다. **stash·checkout·reset 금지**는 유지(동시 세션).

## CONSTRAINTS (원본 위치만)
- 프로젝트 규칙 `AGENTS.md` · 저장·캐시·LLM·목록 원칙 `docs/PRINCIPLES.md` · 운영 함정은 MEMORY.md 색인
- 배포 스크립트 출력을 파이프로 받지 말 것(메모리 `deploy_script_pipe_breaks_it`) — 10-10에 한 번 어겼고 nginx는 무사했다

## VERIFY (10-15 실측)
- go build·vet, llm·translate·toss·service 시험 통과(IsTransient·사슬 오류 보고 시험 추가) · 배포 api+worker-content+worker-market(이미지 일치), health 200, panic 0 · 재기동 뒤 한도 로그 그룹·값당 1줄
- **worker-stock은 재생성 안 함** — financials_domestic(22:00 KST 시작, 보통 ~4시간)이 돌고 있었다. 지수 해설 대체 모델은 다음 재생성부터

## VERIFY (10-20 밤 전체 점검)
- go build·vet·test 전 패키지 통과 · tsc·vitest 512 통과 · eslint 71 → 64(남은 것은 React 컴파일러 권고 `set-state-in-effect`·`preserve-manual-memoization`, CI 비차단) · 5xx 24시간 0건 · 비밀값 추적 파일 없음
- `change_detect`가 10-18·19에 `magnitude` 범위 초과로 실패(내부자 순매매 1억 주 초과) → 108로 DECIMAL(20,4), 배포 api(108 적용 로그 확인). 그 이틀은 주말이라 10-16 목록은 남아 있었다
- research 판정이 07:00 몇 초 전·07:00 두 번 돌던 것 → `WaitForSlot`+`SlotGate`. 내부자 발행사 검증 시험 `TestParseForm4KeepsOnlyIssuerFilings`
- 배포 api+fe · 10-21 00:35 `change_detect` 성공(10-20분 595건) · **10-21 01:55 worker-stock 재생성**(api 이미지 `51a8bf83a825` 일치, panic 0). 01:30~07:26은 잡이 끊김 없이 이어져(warnings→ext_metrics→analyst) 빈 틈이 없고, 07:00 insider_overseas가 옛 코드로 지운 행을 되넣기 전이어야 해서 `social_posts` 끝난 순간 재생성 → `warnings_sync` 하나가 잘림 → 유예 5시간(06:30)을 기다리지 않고 catchup으로 다시 큐에 넣음(02:06 시작)
- **10-21 내부자 오귀속 행 삭제(사용자 승인)**: SEC company_tickers 회사명 = 보고자 이름인 후보 478쌍 → 공시 XML의 issuerCik로 하나씩 확인 → 종목 CIK와 다른 406공시·109종목·**1,079행** 삭제(180일 안 445행·52종목 — 다음 00:25 점수부터 심리 축 반영). 자기 회사가 보고자인 5건(COLD·NVRI·SCLX·SCLXW·SHAK)은 발행사가 맞아 남김. 백업 `be/backups/insider_misattributed_20261021.sql`(INSERT, git 밖). 파생 변화 이벤트 0건. 이름이 다른 자회사 명의 공동 보고는 못 잡았다(새 적재는 issuerCik로 막힘)

## VERIFY (10-20 새벽 실측)
- financials_domestic(01:34 끝)·warnings_sync 끝난 뒤 worker-stock 재생성(api 이미지 일치) · 수동 실행: 다시 쓰기 20·버림 0 — 유형 섞임 확인(감정형 58자·한 줄 68자·초보 질문·숫자 정리 목록·신중파·장문 525자·메모 300자·위험 764자·정석 967자 평균), 숫자는 사실 그대로 · 「주말을 맞아」(평일) 발견 → 유형 설명의 시간 표현 제거 + 「요일·시간대는 쓰지 않는다」 규칙, 재배포 api+worker-stock · 05:46 실행은 Gemini 503(1건만), 06:00 회차 9건 뒤 `llm_feat_bot` 80콜 소진(태평양 10-19 하루에 수동 시험 포함)

## VERIFY (10-19 실측)
- go build·vet, service(검증: 지어낸·반올림 숫자·금지어·길이·#/@, 말투 프롬프트)·handler·repository·llm·apibudget 시험 · tsc·vitest 512 · 새 파일 eslint 0 · 106 적용 · 배포 api+fe+worker-stock(이미지 일치), health 200, panic 0, 관리자 새 API 비로그인 401
- 실제 실행: 첫 회 다시 쓰기 13·버림 12 — 버림은 전부 `ErrPacing`(간격을 호출 시작부터 쟀다) → 응답 끝부터 재도록 고치고 잘못 붙은 메모 12건 지움 → 둘째 회 **다시 쓰기 24·버림 1**(사실에 없는 숫자 30). 오늘 llm_feat_bot 38콜·3.9만 토큰(건당 ~1K). 품질: 커뮤니티는 구어체, 리포트는 소제목 다섯, 숫자는 사실 그대로 — 단 과거 사건을 「앞두고」로 쓴 의미 오류 1건 봄(숫자 검증으로는 못 잡는다 → 관리자 검수 몫)
- **관리자 화면(뱃지 칩·조회수 칸·AI·유형 표시)은 브라우저로 못 봤다**(관리자 로그인 불가)
- 10-19 밤: 「편집해서 게시」(리포트) 화면이 죽던 것 = `/mentions/preview`가 태그 없는 본문에 `tokens: null`(## 소제목의 #가 미리보기를 부른다) → FE `mentionMarkdownRenderer`가 length를 읽다 렌더 오류. BE는 빈 배열, FE `previewMentions`도 배열로 맞춤. 원인은 api 로그의 `client error: kind=render` 스택으로 찾았다. 배포 api+fe(107 적용), worker-stock은 보류(위)

## VERIFY (10-16 실측)
- go build·vet, service·handler·repository 시험(봇 초안 슬롯·가중 뽑기·본문에 #/@·매수/매도 없음·업종 이름) · tsc 통과, vitest 512 통과, 새 파일 eslint 0(기존 파일의 set-state-in-effect 4건은 원래 있던 것) · 105 마이그레이션 적용 · 배포 api+fe+worker-stock(이미지 일치), health 200, panic 0 · 수동 실행 3회 각 커뮤니티 5·리포트 2(후보 306) — 본문 실측으로 업종 키→한국어, 규칙 종가 문장 제외, 머리 두 문장으로 고침 · `/admin/bot-drafts` 비로그인 401
- **관리자 화면·게시·답글은 브라우저로 못 봤다**(관리자 계정 로그인 불가) — 사용자가 `/admin/bots`에서 한 번 올려 보기

## NEXT
- 10-21 07:00 research_verdict가 한 번만 · insider_overseas 뒤 지운 공시(MDT `0001999371-26-022968` 등)가 다시 들어오지 않았는지 · 10-22 00:25 점수에서 MDT·AXP·UBER 심리 축이 바뀌었는지
- 10-20 16:00 KST(태평양 자정) 뒤 회차가 남은 템플릿 초안(10-20 새벽 예산 소진분)을 이어 쓰는지 · `llm_feat_bot` 80콜이 평소(회차 7×8=56 + 밀린 것)에 모자라면 올린다(10-19는 수동 시험 3회가 같은 날에 잡혀 다 썼다)
- 봇 초안: 밤 회차(21·00·03시)가 503으로 템플릿에 남았다가 06·09시 회차에 이어 쓰이는지 · 관리자 API 화면 `llm_feat_bot`·`llm_sub` 하루 사용량 · 버린 비율(note)이 높으면 프롬프트·숫자 허용 규칙 조정 · 게시 계정마다 말투를 적어야 글 말투가 갈린다(비면 기본 말투) · worker-content·market·asset은 10-19 이미지가 아니다(코드 경로 무관)
- 다음 국내 요약 배치(17:20 KST) 로그에 `retrying N transient failures`가 나오면 끝 줄 failed가 줄었는지 · api 로그 「예산 소진」이 하루 몇 줄로 줄었는지
- 메뉴 이름 복원은 사용자가 진행 안 함(10-12). 앱은 10-14에 PWA부터 착수(다음: 웹 푸시는 알림 채널 결정 뒤, 스토어는 서버 이전 없이 진행 — 10-14 결정, 메모리 `app_store_launch_order_1014`: 사업자 등록(사용자 진행 중) → D-U-N-S → 개발자 계정 → 안드로이드 → iOS). 앱인토스는 정책 3-5·3-6으로 핵심 기능 입점 불가
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
- 배치: 상류 5xx 실패 잡 재처리 구멍 · EDGAR 파생거래만 있는 Form 4는 행이 없어 known에 안 잡혀 매 순환 다시 받는다(종목당 20건 상한)
- 정리: e2e 테스트 계정(메모리 `e2e_accounts_in_users_table`) · 메뉴에서 내린 옛 화면(`/targets`·`/stamps`·`/dashboard`) · gofmt 미준수 443파일(독립 커밋 한 번) · eslint 컴파일러 권고 64건 · Docker 빌드 캐시 21GB 회수 가능(디스크 여유 943GB라 급하지 않음)
- 운영: worker-content·market·asset은 10-15 이미지(코드 경로 무관) · translator 메모리 상한 10G 중 9.2G(WSL CUDA 익명 메모리, 문서화된 값)
