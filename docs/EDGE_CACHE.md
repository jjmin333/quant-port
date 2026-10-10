# 공개 API 엣지 캐시 (Cloudflare)

> 2026-09-26. 코드 쪽은 배포됨. **Cloudflare 캐시 규칙 1개를 켜야 효과가 난다** (아래 「켜는 법」).

## 왜

오리진 처리는 대부분 2~10ms인데, 공개 도메인으로 부르면 요청마다 엣지(HKG·NRT) ↔ 오리진(KT 회선)
구간을 건넌다.

| 측정 (2026-09-26 09시, 고유 쿼리 12회) | 값 |
|---|---|
| 오리진(nginx) 처리 | 1~10ms |
| 공개 도메인 TTFB | 190~600ms, 중앙값 ~270ms, 전부 `cf-cache-status: DYNAMIC` |
| 저녁 (09-25 후속4) | 16회 중 9회가 1~16초 (엣지 ↔ KT 구간 손실) |

홈 한 번에 API 14건이 그 구간을 건넌다. 엣지가 답하면 그 구간을 아예 안 건넌다 —
저녁 손실 구간의 멈춤도 그 요청들에선 사라진다.

## 코드가 하는 것

`be/internal/middleware/edge_cache.go` — 공개 조회 51개 경로(`be/internal/server/router.go`의
`edgeLive`/`edgeNews`/`edgeDaily`/`edgeSlow`)에 다음을 붙인다.

```
Cache-Control: public, max-age=0, s-maxage=<신선>, stale-while-revalidate=<낡아도 됨>
```

| 등급 | s-maxage | SWR | 경로 예 |
|---|---|---|---|
| edgeLive | 15초 | 1분 | 지수(`/quotes/indices`, 가장 많이 불림) · 순위 · 환율 · 뉴스 대시보드·목록 |
| edgeNews | 30초 | 5분 | 브리핑 · 키워드 · 시장 설정 |
| edgeDaily | 1분 | 10분 | 점수 · 실적 · 재무 · 종목 개요 · 변화 · 예시 화면 |
| edgeSlow | 5분 | 1시간 | 장 달력 · 백테스트 기준 |

- `max-age=0` — **브라우저 캐시는 그대로**(화면 폴링·fetch-cache 규칙 불변). 오래 두는 것은 엣지뿐.
- **개인화 응답은 절대 공개로 안 나간다.** 처리 중 누구든 `authctx.UserID`를 부르면(비로그인이어도)
  `private, no-store`로 바뀐다(`authctx.IdentityProbe`). 실제로 `/sample/journal/notes`·
  `/sample/price-targets`가 이 규칙에 걸려 private이 됐다(시세 덧씌우기가 관심·보유를 본다).
- 2xx가 아닌 응답은 `no-store`. 실시간 가격(`/rankings/prices`·`/market-indicators/prices`·캔들 — 다른 화면과
  값이 어긋나면 안 된다), 번역을 기다리며 다시 부르는 `/news/reader`, `/quotes/{symbol}/*`(관심·보유 표시), 리서치(작성자 본인 여부), 검색,
  SEO(Next 서버가 내부망으로 부름)는 목록에 없다.

## 켜는 법 (Cloudflare 대시보드 — 사람만 가능)

**Caching → Cache Rules → Create rule**

1. 이름: `api-public-edge`
2. 조건 (Custom filter expression):
   ```
   (http.host eq "www.quant-port.com" and starts_with(http.request.uri.path, "/api/v1/"))
   ```
3. Then: **Eligible for cache**
4. **Edge TTL → 「Use cache-control header if present, bypass cache if not」** ← 이것이 핵심.
   헤더가 없는 경로(인증 경로·쓰기·스트림)는 캐시되지 않는다. 절대 「Ignore cache-control」을 고르지 말 것.
5. Browser TTL: **Respect origin** (기본값 그대로).
6. Cache key: 기본값(쿼리스트링 포함) 그대로.

선택: **Caching → Tiered Cache → Smart Tiered Caching** 켜기(무료) — 방문이 드물어 엣지(HKG·NRT)마다
따로 비는 것을 줄인다.

### 켠 뒤 확인

```bash
curl -s -o /dev/null -D - "https://www.quant-port.com/api/v1/indices/scores" | grep -iE "cf-cache-status|age:"
```

두 번째 요청부터 `cf-cache-status: HIT`(또는 SWR 구간이면 `UPDATING`/`STALE`)이어야 한다.
개인화 경로는 계속 `DYNAMIC`/`BYPASS`여야 한다:

```bash
curl -s -o /dev/null -D - "https://www.quant-port.com/api/v1/sample/price-targets" | grep -iE "cache-control|cf-cache-status"
# → Cache-Control: private, no-store / cf-cache-status: BYPASS 또는 DYNAMIC
```

### 여전히 `DYNAMIC`이면 (2026-09-26 실제로 그랬다)

`DYNAMIC` = Cloudflare가 「캐시 대상 아님」으로 판정했다는 뜻이다. 오리진 헤더가 틀리면 `MISS`/`BYPASS`가
나오지 `DYNAMIC`은 안 나온다 — **규칙이 요청에 매칭되지 않는 것이다.** 같은 존의 robots.txt는
`UPDATING`/`EXPIRED`로 엣지 캐시가 돌고 있음을 확인했다. 순서대로 본다:

1. **배포 상태** — 규칙 목록에서 토글이 켜져 있는가(「Save as Draft」만 하면 꺼진 채다).
2. **호스트** — `quant-port.com`이 아니라 `www.quant-port.com`이다(apex는 www로 301).
3. **경로 연산자** — 「URI Path」 + **starts with** `/api/v1/`. 「equals `/api/v1/*`」는 별표를 글자로 봐서 아무것도 안 맞는다.
   와일드카드를 쓰려면 연산자가 「wildcard」여야 한다.
4. **순서** — 규칙은 위에서 아래로 평가되고 뒤의 것이 이긴다. 아래쪽에 `/api` 또는 전체에 「Bypass cache」 규칙이 있으면 덮인다.
5. **다른 장치** — Development Mode 켜짐, Page Rules의 Cache Level: Bypass, 이 경로에 걸린 Worker/Snippet.
6. 올바르면 저장 후 1분 안에 두 번째 요청부터 `HIT`이 나온다.

### 되돌리기

규칙을 끄면 즉시 원래대로다(오리진 헤더는 규칙 없이 아무 일도 안 한다). 낡은 값이 보이면
**Caching → Configuration → Purge Everything**.

## 새 공개 경로를 추가할 때

요청자와 무관한 GET이면 `public.With(edgeDaily)` 등으로 등록한다. 서버 쪽 갱신 주기보다 긴 s-maxage를
주지 않는다. 신원을 보는 경로는 넣어도 private으로 떨어질 뿐 새지는 않지만, 얻는 것이 없으니 넣지 않는다.
