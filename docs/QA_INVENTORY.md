# QA 인벤토리 — 화면·API ↔ 테스트 매트릭스

> 2026-09-06 작성. 목적은 「무엇이 있나」가 아니라 **「무엇이 안 덮여 있나」** 를 한 장에
> 세우는 것이다. 덮인 항목은 어디서 덮이는지만 적고, 빈 곳에 이유를 적었다.

측정 시점 기준(2026-09-06 2차): `e2e` **176 passed / 0 failed / 12 skipped**(3회 연속),
`fe` 203 unit, `be` `go test ./...` 전체 통과.

---

## 1. 테스트 레이어와 역할

| 레이어 | 위치 | 개수 | 무엇을 지키나 |
|---|---|---|---|
| Go 단위 | `be/**/*_test.go` | 26개 패키지 | 회계·시간대·점수·파싱·스케줄·**초과매도·예측잠금** 등 **순수 규칙** |
| FE 단위 | `fe/src/**/*.test.ts` | 22파일 / 203 | 계산·상태 전이·포맷 (브라우저 없이 결정적) |
| API e2e | `e2e/tests/api/` | 11파일 / 111 | 인가 경계, 소유권, 회계, 동결 계약, 응답 형태, **시세·뉴스·실적·점수 구조** |
| UI e2e | `e2e/tests/ui/` | 8파일 / 27 | 쿠키 세션, 클라이언트 분기, 자동완성, **키보드** |

원칙은 `e2e/README.md` 그대로다 — **같은 것을 두 레이어에서 두 번 돌리지 않는다.**

---

## 2. 화면(FE 라우트) ↔ 테스트

`fe/src/app` 기준 34개 라우트.

| 라우트 | UI e2e | 비고 |
|---|---|---|
| `/(auth)/login` · `/signup` | `login-logout` | 하이드레이션 대기 포함 |
| `/(main)` (홈) | — | 스윕으로 a11y·오버플로만 확인 |
| `/market` | `market-radar-mobile-back` | **폰 제스처 + 태블릿 버튼** 두 경로 |
| `/analysis` | — | `/market`과 같은 툴바·행 컴포넌트를 공유 |
| `/holdings` | `holdings-add`, `holdings-keyboard`, `sample-mode-screens` | 표·카드 **두 보기** 모두 |
| `/watchlist` `/stamps` `/journal` `/targets` | `sample-mode-screens` | 예시 on/off × 4화면 |
| `/reports` `/reports/[id]` `/reports/new` | API로만 (`research-*`) | 발행·동결·한도는 API 계약 |
| `/news/domestic` `/news/us` | — | API 구조 불변식으로 덮음(`market-data`) |
| `/earnings/*` `/calendar` `/disclosures` `/financials` | — | API 구조 불변식으로 덮음(`market-data`) |
| `/stocks/[symbol]` | — | 스윕으로 존재/오버플로만 |
| `/admin/*` (7) | `admin-gating` | 비관리자에게 메뉴가 안 보이는 것까지 |
| `/settings` | `account.spec` (API) | |

## 3. API ↔ 테스트

`be/internal/server/router.go` 기준. 공개 78 · 보호 45 · 관리자 40.

| 묶음 | 상태 | 덮는 곳 |
|---|---|---|
| `/auth/*` (8) | ✅ | `auth-session`, `account`, `authorization` |
| 인가 경계 (비로그인 401 / 일반→admin 403) | ✅ | `authorization` — 경로 목록으로 일괄 |
| 소유권 (관심·메모·목표가·보유·매매·임포트·스탬프·**댓글**) | ✅ | `authorization`, `stamps`, `research-freeze` |
| 보유·매매 회계 (평단·실현손익·**초과 매도**) | ✅ | `holdings-trades` (15) |
| 리서치 발행·동결·한도·쿨다운 | ✅ | `research-freeze`, `research-limits` |
| 스탬프 봉인·쿨다운·수정 범위 | ✅ | `stamps` |
| 종목 검색 랭킹·안정성·종목코드 | ✅ | `search-ranking` |
| 샘플 픽스처 (세션 무관 동일) | ✅ | `contract` |
| 응답 봉투 · 라우트 충돌 · 미디어 | ✅ | `contract` |
| **없는 종목 거부** | ✅ | `contract` (신규) |
| 관리자 콘솔 (11) | ⏸️ skip | 자격증명 없으면 skip — §4 |
| 시세·캔들·지수 | ✅ | `market-data` (SSE 장중 동작만 사람 몫) |
| 뉴스·키워드 | ✅ | `market-data` |
| 실적 캘린더·서프라이즈 | ✅ | `market-data` |
| 점수·랭킹 | ✅ | `market-data` |

---

## 4. 구멍 — 왜 안 덮였고, 덮으려면 무엇이 필요한가

| # | 구멍 | 왜 지금 없나 | 덮으려면 |
|---|---|---|---|
| ~~G-1~~ | ~~시세·SSE~~ | 값이 마감 중엔 안 움직여 단언이 공허했다 | **해소** — 구조 불변식으로: batch 정확성·통화·없는 심볼 5xx 없음·404. **장중 live만 사람 몫** |
| ~~G-2~~ | ~~뉴스~~ | 외부 소스라 내용이 매번 다르다 | **해소** — limit·필드 존재·시장 분리·키워드 건수 |
| ~~G-3~~ | ~~실적~~ | 배치 적재라 비어 있을 수 있다 | **해소** — 빈 달 200·달 이동 반영·요약/서프라이즈 5xx 없음 |
| ~~G-4~~ | ~~점수·랭킹~~ | 값 자체는 배치 산출 | **해소** — 순위 연속·내림차순·시장 분리·0~100+등급·지수 중복 없음 |
| G-5 | **관리자 콘솔 11건** | 테스트가 관리자 계정을 만들면 **실제 관리자**가 생긴다(`model.DefaultAdminEmail`) | `E2E_ADMIN_EMAIL/PASSWORD` 있는 환경에서 1회 |
| ~~G-6~~ | ~~스탬프 잠금~~ | 5분 대기가 sleep 금지 원칙과 충돌 | **해소** — `predictionLocked` 순수 함수 + 경계 6케이스 |
| G-7 | OCR 임포트 | Tesseract + 실제 이미지 필요 | 파싱은 `internal/import/**`가 두텁게 덮음 |

---

## 5. 이번 세션에 늘어난 것

| 추가 | 레이어 | 지키는 결함 |
|---|---|---|
| `repository/holding_trades_oversell_test.go` (8케이스) | Go 단위 | 초과 매도 write 거부 |
| `fe/src/lib/auth-error.test.ts` (4) | FE 단위 | 429·5xx에 로그아웃되지 않음 |
| `e2e/tests/ui/holdings-keyboard.spec.ts` (표·카드 2) | UI e2e | 카드가 키보드로 안 눌리던 것 |
| `holdings-trades.spec.ts` +3 | API e2e | 초과 매도 거부 · 재전송 오탐 없음 |
| `contract.spec.ts` +3 | API e2e | 없는 심볼 거부 |
| `authorization.spec.ts` +1 | API e2e | 댓글 소유권이 4xx로 거부됨 |
| `market-radar-mobile-back.spec.ts` 재작성 | UI e2e | 폰 제스처 + **태블릿 버튼**(새 커버리지) |
| `e2e/src/ui.ts` `holdingItem()` | 헬퍼 | 보기 기본값이 또 바뀌어도 안 깨짐 |
| **`e2e/tests/api/market-data.spec.ts` (18)** | API e2e | **G-1~G-4 구멍** — 시세·뉴스·실적·점수 구조 불변식 |
| `be/internal/service/stamp_prediction_lock_test.go` (6+1) | Go 단위 | 예측 잠금 경계(G-6) — 5분 안 기다리고 |
| `market-data.spec.ts` 의 요약 404 | API e2e | 없는 종목 요약이 500이던 것 |
