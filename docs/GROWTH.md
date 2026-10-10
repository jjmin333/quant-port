# 검색 유입 감시 (worker-growth)

> 2026-09-25 도입. 사용자 결정: **AI 에이전트가 아니라 코드만으로 도는 별도 컨테이너**.
> 코드: `be/internal/growth` · `be/cmd/worker-growth` · 표 `be/migrations/087_growth.sql`.

## 무엇을 하나

매일 07:30 KST에 한 번 돈다(`growth_audit`, 관리자 배치 콘솔에서 수동 실행도 된다).

| 원래 요구 | 하는 일 | 재료 |
|---|---|---|
| 사이트 전체 SEO 크롤 · 오류 탐지 | 사이트맵 URL을 모두 열어 상태 코드 · 리다이렉트 · noindex · canonical · robots.txt · 제목/설명(길이·중복) · H1 · 서버가 그린 본문 글자 수 · 응답 시간 · 공유 이미지 · 구조화 데이터 · 들어오는 내부 링크를 판정. 어제와 비교해 **새 문제 / 고쳐진 문제**를 적는다 | 공개 사이트 |
| 색인 상태 분석 | Search Console URL 검사를 하루 200개씩 돌려 본다(안 본 것 → 오래된 것 순). 상태가 바뀐 URL, Google이 다른 canonical을 고른 URL | Search Console |
| 검색어 변화 · 기존 콘텐츠 성과 | 최근 7일 vs 이전 7일: 오른/떨어진/새/사라진 검색어, 페이지별 노출·클릭·순위 | Search Console · Bing |
| 신규 검색 기회 · 검색어별 개선안 | 4~20위 문턱 검색어, 상위권인데 CTR이 기대의 절반 미만인 검색어, 종목 검색인데 종목 페이지가 안 걸리는 경우, **검색 수요가 있는데 noindex인 종목** | Search Console + 종목 이름표 |
| 신규 콘텐츠 후보 | 종목 × 의도(실적·배당·전망·목표가…) 검색어, 홈·목록이 20위 밖에서 받는 검색어, 7일 안 실적 발표 예정인 색인 대상 종목 | Search Console · `earnings_events` |
| 종목 페이지 품질 | 색인 대상 1,000종목: 기본 제목으로 나간 페이지(요약을 못 받음), 요약 줄이 화면에 없음, 본문 얇음 | 크롤 |
| 내부 링크 개선안 | 수요가 있는데 들어오는 링크가 ≤1인 페이지, 종류별로 사이트맵으로만 발견되는 페이지 비율 | 크롤 + Search Console |
| 외부 콘텐츠 유통 후보 | 최근 7일 공개 리포트(조회순), 가장 최근 변화 감지 상위(색인 대상 종목만). **게시하지 않는다** | DB |
| KPI 변화 | Google 클릭·노출·CTR·순위, Bing, GA4 세션·자연 검색 세션, 가입·로그인·첫 보유 등록·종목 추가 | 전부 |

월요일(또는 지난주 보고서가 없을 때)에는 **주간 보고서**를 함께 만든다: KPI 주간 비교, 가입 코호트의
7일 활성화율, **Top 20 기회**, 다음 주 우선순위 5, 기술 수정 제안, 콘텐츠 제안, 검색어별 개선안,
노출이 반 넘게 준 페이지, GA4 자연 검색 첫 페이지.

### 하지 않는 것

- **LLM 호출 없음.** 판단은 전부 규칙이다(`analyze.go`). 「클릭/주」는 순위별 기대 CTR 곡선으로 어림한 값이라 기회끼리 줄 세우는 데만 쓴다.
- **앱을 바꾸지 않는다.** 쓰는 곳은 `growth_*` 표와 보고서 파일뿐 — `store_guard_test.go`가 이 패키지의 쓰기 문장이 `growth_` 표만 겨누는지 센다. 투자 알고리즘 · 점수 산식 · 금융 원본 · 백테스트 · 수익률 · 개인정보 · 인증 · 결제 · 법적 고지는 건드릴 경로가 없다.
- **PR을 만들지 않는다.** 보고서가 「고칠 자리」(파일 경로 · 관리자 화면)까지 적고, 이슈 → 브랜치 → 시험 → 미리보기 → SEO 점검 → 승인 → 배포는 사람이 한다. (저장소가 공개라 GitHub 이슈 자동 생성도 넣지 않았다 — 검색어 데이터가 공개된다.)
- 사용자 표에서는 **개수만** 센다.

## 보고서 보는 곳

- 파일: `be/var/growth-reports/daily-YYYY-MM-DD.md`, `weekly-YYYY-MM-DD.md`(주 마지막 일요일 날짜)
- API(관리자): `GET /api/v1/admin/growth/reports?kind=weekly` · `GET /api/v1/admin/growth/reports/latest?kind=daily` · `/reports/{id}`
- 관리자 화면: **`/admin/growth`**(사이드바 「검색 유입 보고서」) — 주간/일간 목록 + 본문. 읽기만 하고, 다시 만들기는 배치 관리의 「검색 유입 감시」.
  Markdown은 `components/admin/growth-markdown.tsx`가 그린다(라이브러리 없음 · innerHTML 없음 — 보고서에 외부 검색어가 실린다).

## 설정 — 사람이 해야 하는 것

자격 증명이 없으면 그 소스는 건너뛰고 보고서에 「미설정」으로 적는다. 크롤과 DB 지표는 설정 없이 돈다.

### 1. Google 서비스 계정 (Search Console + GA4 공용)

1. Google Cloud 콘솔 → 프로젝트 하나 → **API 사용 설정**: 「Google Search Console API」, 「Google Analytics Data API」.
2. IAM → 서비스 계정 만들기(역할 없음) → 키 → JSON 받기.
3. 받은 파일을 **`be/secrets/google-service-account.json`**에 둔다. (`be/secrets/`는 `.gitignore`와 `be/.dockerignore`에 들어 있다 — 깃에도 이미지에도 안 들어간다. 컨테이너는 읽기 전용 마운트로 읽는다.)
4. **Search Console** → 설정 → 사용자 및 권한 → 사용자 추가 → 서비스 계정 이메일(`…@….iam.gserviceaccount.com`) · 권한 **제한**.
5. **GA4** → 관리 → 속성 액세스 관리 → 같은 이메일 · 역할 **뷰어**. 그리고 `be/.env`에 속성 ID(숫자, 측정 ID `G-…`가 아님):
   ```
   GROWTH_GA4_PROPERTY_ID=123456789
   ```
6. 속성이 도메인 속성이 아니면 `GROWTH_GSC_PROPERTY=https://www.quant-port.com/` (기본값은 `sc-domain:quant-port.com`).

범위는 읽기 전용 둘뿐이다(`webmasters.readonly`, `analytics.readonly`).

### 2. Bing Webmaster Tools

1. Bing Webmaster Tools → 사이트 추가 → **Google Search Console에서 가져오기**(소유 확인과 사이트맵이 같이 넘어온다).
2. 설정(톱니) → API 액세스 → API 키 → `be/.env`에 `GROWTH_BING_API_KEY=…`.

### 3. 네이버 서치어드바이저

공개 API가 없다. 기술 점검은 크롤이 대신한다(네이버 Yeti처럼 자바스크립트 없이 받은 HTML을 본다). 검색 성과는 콘솔에서 직접 본다.

### 4. GA4 태그 (FE)

`fe/src/app/layout.tsx`에 gtag.js(측정 ID `G-T8RNF91MRH`, `lib/seo.ts`)를 넣었다 — 운영 빌드에서만.
2026-09-25 배포. 개인정보처리방침(`/legal/privacy`)의 1·2·4·5항에 GA(수집 항목 · 통계 목적 · Google LLC 미국 전송 · 쿠키와 거부 방법)를
넣었다(사용자 승인). 7항 운영자 정보 `TODO(운영자)`는 여전히 비어 있다.
GA 콘솔에서 할 일: 관리 → 데이터 스트림 → 태그 설정 → **내부 트래픽 정의**(운영자 IP) 후 데이터 필터 활성화.

### 적용

```
cd be
.\scripts\restart.ps1 -Background
```

(worker-growth가 서비스 목록에 들어 있다.) 설정을 바꾼 뒤 바로 보고 싶으면 관리자 배치 콘솔에서 「검색 유입 감시」를 수동 실행한다 — limit 1이면 주간 보고서도 만든다.

## 조정할 값 (`be/.env`)

| 변수 | 기본 | 뜻 |
|---|---|---|
| `GROWTH_SITE_URL` | `https://www.quant-port.com` | 사이트맵·robots를 받는 주소 |
| `GROWTH_CRAWL_ORIGIN` | compose: `http://nginx` (코드 기본은 비움 = 공개 주소) | 페이지 크롤을 보낼 곳. Host 헤더는 사이트 것을 쓴다. 공개 경로가 2026-09-25에 간헐적으로 8~30초 멈춰 사이트맵조차 못 받는 날이 생겨 오리진으로 돌렸다 |

**공개 주소 도달성 탐침**: 크롤과 별도로 매번 공개 주소 5곳 × 3회를 재시도 없이 열어 3초 초과·실패를 센다(KPI `public` 절).
하나라도 있으면 기회 목록 맨 위에 「공개 주소 응답 실패」로 올라온다. robots.txt도 공개 주소에서 받는다(Cloudflare가 자기 묶음을 끼워 넣는다).
| `GROWTH_CRAWL_CONCURRENCY` · `GROWTH_CRAWL_DELAY_MS` | 2 · 500 | 동시 요청 수 · 요청 사이 간격 |
| `GROWTH_INSPECT_PER_DAY` | 200 | URL 검사 하루 건수(Google 한도 속성당 2,000/일) |

## 문턱값

`audit.go`: 제목 45자 초과(템플릿 포함) · 10자 미만, 설명 160자 초과, 본문 300자 미만(종목 80자), 응답 2.5초 초과.
`analyze.go`: 기대 CTR 곡선(1위 28% → 10위 3% → 20위 1%), 문턱 검색어 4~20위 · 주 노출 5 이상, CTR 부족 = 10위 안 · 노출 20 이상 · 기대의 절반 미만.
