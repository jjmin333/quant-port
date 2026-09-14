# OCI 이관 — 성능·안정성 사전 진단과 개선 설계

> 대상: Application VM `VM.Standard.A1.Flex` 2 OCPU / 12 GiB / ARM64 / OL9 / 부트 50 GiB
> DB: `MySQL.FREE` 1 ECPU / 8 GiB / IOPS 3,750 / 29 MiB/s / 백업 보존 1일 / HeatWave 16 GiB × 1노드
>
> 이 문서는 **추측이 아니라 이 리포의 실측**에서 출발한다. 측정 시각 2026-08-25,
> 로컬 Docker 스택 12컨테이너 가동 중, MySQL 8.0 uptime 268,045초(3.1일).

---

## 0. 이 문서가 딛고 선 사실 (측정값)

「임의로 가정하지 말라」는 제약에 대해: 소스·Dockerfile·compose·스키마·SQL이 **이 리포에 전부 있고
스택이 실제로 돌고 있어서**, 아래 값은 가정이 아니라 관측이다. 관측할 수 없었던 것만 **[추정]** /
**[확인 필요]** 로 표시했다.

### 0.1 컨테이너 (docker stats, 유휴 상태)

| 컨테이너 | CPU% | RSS | 메모리 상한 | CPU 상한 |
|---|---|---|---|---|
| quant-port-api | 0.71% | 37.9 MiB | 1 GiB | 2.0 |
| quant-port-redis | 0.73% | 42.8 MiB | 1.5 GiB | 1.0 |
| quant-port-fe (Next standalone) | 0.01% | 59.2 MiB | 512 MiB | 1.0 |
| quant-port-nginx | 0.00% | 51.5 MiB | 128 MiB | 0.5 |
| worker × 8 | 0.00% | 4.2–8.8 MiB (합 46.1) | 각 256 MiB | 각 0.5 |
| **합계** | — | **237.5 MiB** | 합 5.6 GiB | **합 8.5 CPU** |

- 재기동 횟수 0, OOMKilled 0 (전 컨테이너).
- api 컨테이너 열린 fd **33개**, `ulimit -n` 1,048,576 → fd는 지금 문제가 아니다.
- cgroup 확인: `cpu.max = 200000 100000` → **compose의 `deploy.resources.limits`가 실제로 적용된다**
  (Compose v2). 즉 api 한 컨테이너가 박스의 2 OCPU 전부를 가져갈 수 있다.

### 0.2 애플리케이션 (`/metrics`)

```
goroutines 85 · heap 25.5 MB
mysql: open=8 in_use=0 idle=8 max_open=24 wait_total=0 wait_seconds=0
redis: total=8 idle=8 · keys=27,935 (TTL 없는 키 3,682)
poller: priced_symbols=224 terminal_symbols=1 · sse_streams=0
toss REST 누적: MARKET_DATA 251 · CHART 39 · STOCK 31 · RANKING 28 · MARKET_INFO 25
```

**커넥션 풀은 지금 여유가 아니라 과잉이다** — 상한 24에 최대 사용 8, 대기 0건.

### 0.3 DB (information_schema / SHOW GLOBAL STATUS)

총 **1.84 GB · 47 테이블**. 상위 4개가 전체의 92%.

| 테이블 | 행 | 데이터 | 인덱스 | 합 |
|---|---:|---:|---:|---:|
| stock_daily_closes | 3,718,278 | 599.7 MB | 79.6 MB | 679.3 MB |
| candle_bars (파티션 5) | 5,568,872 | 579.2 MB | **0** | 579.2 MB |
| stock_scores | 154,631 | 258.9 MB | 4.5 MB | 263.4 MB |
| news_articles | 82,184 | 127.5 MB | 57.8 MB | 185.3 MB |

3.1일 누적 상태값에서 유도한 값:

| 지표 | 실측 | 유도 |
|---|---:|---|
| Questions | 22,810,378 | **평균 85 QPS** |
| Com_insert / Com_select | 8,871,316 / 8,355,276 | 33/s / 31/s |
| Innodb_data_written | 2.41 GB | **평균 9.0 KiB/s = 29 MiB/s 상한의 0.03%** |
| Innodb_buffer_pool_reads / read_requests | 402,165 / 3,881,033,115 | **캐시 히트율 99.9896%** |
| Innodb_row_lock_waits / lock_time | 22,387 / 2,724초 | 대기 7,200건/일, **평균 122 ms** |
| Sort_merge_passes | 83,861 | 0.31/s — 정렬이 sort_buffer(256 KB)를 넘겨 디스크 병합 |
| Created_tmp_disk_tables / tmp_tables | 86 / 22,495 | 0.4% — 문제 없음 |
| Slow_queries (long_query_time=10s) | 151 | 10초를 넘긴 쿼리가 3일에 151건 |
| Connections | 356,054 | **1.33 신규 접속/s** ← 원인 미상, 원격 DB에서 비용이 된다 |
| Max_used_connections | 38 | 배정 총합 56의 68% |

인덱스 설계는 대체로 건강하다:
- `candle_bars` PK `(symbol, bar_interval, bar_time)` + `bar_interval` 파티션, **보조 인덱스 0개**
  → 쓰기 증폭이 최소. 차트 조회 패턴과 정확히 일치.
- `stock_scores`는 중복 접두사 인덱스를 이미 제거함(10.5 MB → 4.5 MB, 세션 기록).
- `news_articles`는 인덱스 6개 57.8 MB — 데이터의 45%. 사용률 확인 대상(§8.3).

### 0.4 현재 시간대 설정 — **이관의 최대 지뢰**

```
@@global.time_zone = SYSTEM,  system_time_zone = KST
NOW() = 2026-08-25 05:15:49   UTC_TIMESTAMP() = 2026-08-24 20:15:49
TIMESTAMP 컬럼 85개 · DATETIME 컬럼 4개 (candle_bars.bar_time, news_articles.published_at/created_at/updated_at)
```

DSN은 `loc=Asia%2FSeoul`. 즉 **드라이버는 "서버가 KST로 말한다"고 믿고 있다.** §4.1에서 이어진다.

### 0.5 디스크 — 부트 볼륨 50 GiB에 대한 실측

```
docker system df
Images        4.539 GB (reclaimable 3.458 GB / 76%)
Build Cache  31.59  GB (reclaimable 21.43 GB)   ← 부트 볼륨의 63%
Volumes       213.8 MB
```

`be/scripts/restart.ps1`은 `docker compose up -d --build`로 **호스트에서 이미지를 빌드한다.**
그 캐시가 로컬에서 31.6 GB까지 자랐다. 50 GiB 부트 볼륨에 그대로 옮기면 **정리 정책 없이는 언젠가 찬다.**

### 0.6 컨테이너 로그

`docker-compose.yml`에 `logging:` 블록이 **한 줄도 없다** → json-file 드라이버 기본값 = **무제한**.
`middleware.Logger`가 요청마다, nginx `access_log`가 요청마다(공식 nginx 이미지는 stdout 심볼릭 링크)
쓴다. 디스크 상한이 없다.

---

## 1. Executive Summary — 가장 위험한 것 TOP 10

순서는 **(발생 확률 × 복구 난이도)** 다. "성능이 몇 % 나빠진다"보다 "서비스가 죽거나 데이터가 틀어진다"를 위로 올렸다.

| # | 위험 | 근거 | 결과 | 등급 |
|---|---|---|---|---|
| 1 | **DB 세션 타임존이 KST→UTC로 바뀐다** | 현재 서버 tz=KST인데 DSN은 `loc=Asia/Seoul`. 관리형 MDS는 UTC가 기본[확인 필요] | TIMESTAMP 85컬럼이 **9시간 밀린다**. 캔들·종가·뉴스·배치 시각 전부. 조용히 틀어져서 며칠 뒤에 발견된다 | 🔴 P0 |
| 2 | **부트 볼륨 50 GiB가 빌드 캐시로 찬다** | 로컬 빌드 캐시 실측 **31.6 GB** + 이미지 4.5 GB | 디스크 100% → Docker·로그·앱 전부 실패. 복구는 SSH가 살아 있어야 가능 | 🔴 P0 |
| 3 | **컨테이너 로그가 무제한** | compose에 `logging:` 없음 | 위 2번을 가속. 트래픽이 늘수록 빨라진다 | 🔴 P0 |
| 4 | **published 포트가 0.0.0.0에 열린다** (`6379:6379`, `8080:8080`) | compose 실측 | 인증 없는 Redis가 인터넷에 노출. api:8080 직접 접근은 nginx의 XFF 덮어쓰기를 우회 → **레이트 리밋 무력화**(이미 한 번 실측으로 겪은 구멍) | 🔴 P0 |
| 5 | **TLS가 없다** | nginx `listen 80`만, `AUTH_COOKIE_SECURE=false` | 로그인 토큰이 평문. 공개 배포 불가 | 🔴 P0 |
| 6 | **OCR 팬아웃이 코어 수를 모른다** | `import_job.go:265` 이미지당 고루틴 1개, 각 tesseract 최대 10패스. 장수 상한은 `maxImportImages = 10`으로 **이미 있다** — 문제는 그 10장이 2 OCPU에서 동시에 도는 것 | 사용자 1명이 2 OCPU를 전부 먹는다. api 컨테이너 상한이 `cpus:2.0`이라 **막아 줄 것이 없다** | 🔴 P0 |
| 7 | **api 컨테이너가 API가 아니다** | 배경 루프 13개(시세 폴러·WS·뉴스·환율·지수·LLM 배치 4종·요약·redis census) + SSE + OCR + 마이그레이터가 한 프로세스 | 재기동 = **적재 중단 + Toss 토큰 재발급**(프로세스 기동마다 1회, 기존 토큰 401). OOM 킬이면 graceful shutdown도 없다 | 🟠 P1 |
| 8 | **DB가 1 ECPU인데 평균 85 QPS가 이미 흐른다** | 실측. 사용자 트래픽 없이 폴러·배치만으로 | 헤드룸이 생각보다 적다. 동시 실행 가능한 쿼리는 사실상 2~4개 | 🟠 P1 |
| 9 | **락 대기 평균 122 ms · 3일에 22,387건** | `Innodb_row_lock_waits` | 원인 미규명. 1 ECPU에서는 대기가 곧 큐잉 | 🟠 P1 |
| 10 | **백업 보존 1일 + 마이그레이션이 기동 시 자동 실행** | MySQL.FREE 사양 + `EnsureSchema`가 `server.New()`에서 60초 타임아웃으로 실행 | 잘못된 마이그레이션이 배포와 함께 나가고, 되돌릴 스냅샷이 24시간뿐. 60초 초과 시 api·워커 전면 기동 실패 | 🟠 P1 |

**한 문장 요약**: 이 시스템의 병목은 CPU도 IOPS도 아니다. 실측상 DB 쓰기는 29 MiB/s의 0.03%,
버퍼풀 히트 99.99%, VM 메모리는 12 GiB 중 238 MiB만 쓴다.
**위험은 전부 "경계가 없는 것"에 몰려 있다** — 로그 상한 없음, 빌드 캐시 상한 없음,
OCR 동시성 상한 없음, 포트 노출 경계 없음, 타임존 경계 없음.

---

## 2. Current Architecture Assessment

### 2.1 지금 (로컬, 그리고 「그대로 이전」했을 때)

```
                        인터넷
                          │  :80 평문 HTTP                ← TLS 없음 (P0-5)
                          ▼
              ┌───────────────────────────┐
              │ nginx 1.27-alpine         │ XFF 덮어쓰기 = 레이트리밋의 근거
              │ cpus .5 / 128M            │ resolver 127.0.0.11 (재기동 추적)
              └────────┬─────────┬────────┘
                   /api│         │/
                       ▼         ▼
   ┌────────────────────────┐  ┌──────────────────┐
   │ api  cpus 2.0 / 1 GiB  │  │ fe (Next stand-  │
   │ ────────────────────── │  │ alone) 1.0/512M  │
   │ HTTP + SSE             │  └──────────────────┘
   │ + 시세 폴러(tickFast/  │
   │   tickSlow) + Toss WS  │        ┌──────────────────────┐
   │ + 뉴스/환율/지수 스토어│        │ redis 7  1.0 / 1.5G  │
   │ + LLM 배치 4종         │◀──────▶│ maxmemory 1gb        │
   │ + Tesseract OCR ⚠무제한│        │ volatile-lru         │
   │ + 스키마 마이그레이터  │        │ :6379 → 0.0.0.0 ⚠    │
   │   :8080 → 0.0.0.0 ⚠    │        └──────────────────────┘
   └───────────┬────────────┘                    ▲
               │ pool 24                         │ 워커별 pool 4
   ┌───────────┼─────────────────────────────────┴─────────┐
   │  worker × 8 (close-domestic/overseas, scores, index,  │
   │  fetch, research, earnings, filings)                  │
   │  각 cpus 0.5 / 256M · DB 큐 폴링 4초 · SKIP LOCKED    │
   └───────────┬───────────────────────────────────────────┘
               ▼
      ┌────────────────────────────────┐
      │ MySQL 8.0                      │  ← 지금은 호스트, OCI에선 원격 관리형
      │ 1.84 GB / 47 tables            │     tz KST → UTC 전환 ⚠ (P0-1)
      │ buffer pool hit 99.99%         │     로컬소켓 → 사설망 + TLS
      └────────────────────────────────┘
```

### 2.2 이미 잘 설계돼 있는 것 (건드리지 말 것)

이관 설계에서 **바꾸면 손해인 것**을 먼저 못박는다. 아래 8가지는 이 문서의 어떤 제안도 되돌리지 않는다.

1. **배치가 이미 프로세스 단위로 분리돼 있다.** 잡별 8개 워커 컨테이너 + 공용 이미지 + `command:`
   교체. 한 배치가 멈춰도 API가 안 멈춘다. → §7에서 "분리해야 하는가"는 **이미 끝난 질문**이다.
2. **배치 큐가 `FOR UPDATE SKIP LOCKED`다** (`batch_job_runs.ClaimNext`). 중복 실행 방지와
   교착 회피가 DB 레벨에서 보장된다. 전용 인덱스 `(job_id, status, id)`도 있다(migration 058).
3. **커넥션 풀이 "서버 상한을 나눠 쓰는 예산"으로 설계돼 있다** (api 24 + 8×4 = 56).
   근거가 `/metrics` 실측이고 주석에 남아 있다. CLI 전용 진입점(`NewMySQLForCLI`, 기본 4)도 별도.
4. **캔들 계층화**(1m→5m→10m→30m→60일 후 삭제)로 최대 테이블의 성장이 제어된다.
5. **`candle_bars`에 보조 인덱스가 0개**다. 초당 수백 행이 들어오는 표에서 정확한 선택.
6. **전 컨테이너 `restart: unless-stopped` + healthcheck + `depends_on: service_healthy`**.
7. **nginx가 `proxy_pass`에 변수를 써서 매 요청 DNS 재해석**한다. 컨테이너 재기동 502를 막는다.
8. **Go 1.25.5** → 컨테이너 cgroup CPU 상한에서 GOMAXPROCS가 자동으로 잡힌다. 수동 설정 불필요.

---

## 3. Bottleneck Analysis

「OCI로 옮겼을 때」 기준. 로컬과 조건이 달라지는 항목에 **Δ**.

| 항목 | 등급 | 근거 |
|---|---|---|
| **Disk 공간** Δ | 🔴 Critical | 빌드 캐시 31.6 GB 실측 vs 부트 50 GiB. 로그 상한 없음. 로컬은 디스크가 커서 안 터졌을 뿐 |
| **Application↔DB 통신** Δ | 🔴 Critical | 타임존 전환(§4.1). 부수적으로 로컬소켓→사설망 RTT, TLS 핸드셰이크, 신규 접속 1.33/s |
| **OS resource limits / 노출 경계** Δ | 🔴 Critical | published 포트 0.0.0.0. OL9 firewalld는 Docker의 DNAT를 못 막는 구성이 흔하다[확인 필요] |
| **CPU (VM)** Δ | 🟠 High | 유휴 0.7%이지만 ① OCR 무제한 팬아웃 ② **VM 위에서 이미지 빌드**(Go 9바이너리 + Next 빌드)가 2 OCPU를 점유 |
| **DB CPU** | 🟠 High | 1 ECPU에 이미 평균 85 QPS. 동시 유효 쿼리 2~4개[추정] |
| **Container CPU limit** | 🟠 High | 상한 합 8.5 vs 실제 2 OCPU. 상한은 있는데 **가중치(shares)가 없다** → 경합 시 API가 배치와 동등하게 밀린다 |
| **Logging** Δ | 🟠 High | 무제한. Disk 위험의 절반 |
| **Batch job** | 🟠 High | 구조는 좋다. 남은 위험은 DB 한 곳에서 만나는 것 + `warnings_sync` 2시간짜리 |
| **DB connection 수 / pool** | 🟡 Medium | 배정 56, 실측 최대 38(CLI 포함). MDS의 `max_connections`는 [확인 필요]지만 여유가 있을 것 |
| **Memory (VM)** | 🟡 Medium | 238 MiB / 12 GiB. 여유는 크지만 **api 컨테이너 상한이 1 GiB**라 OCR 피크가 여기서 터진다 |
| **Container memory limit** | 🟡 Medium | 위와 동일. OOM 킬은 graceful shutdown을 건너뛴다 |
| **애플리케이션 런타임 메모리(Go)** | 🟡 Medium | heap 25.5 MB. `GOMEMLIMIT` 미설정이라 GC가 cgroup 상한을 모른다 |
| **Docker image/storage** | 🟡 Medium | api 381 MB / fe 446 MB. 크지 않지만 태그 9개 + 빌드 캐시 |
| **Temporary file** | 🟡 Medium | OCR 중간 이미지(crop/×2/×3 업스케일)를 컨테이너 writable layer에 쓴다 → 부트 볼륨 IOPS |
| **Docker overhead** | 🟢 Low | 컨테이너 12개, 유휴 CPU 합 1.5% |
| **Disk I/O (VM)** | 🟢 Low | DB가 VM 밖. 남는 것은 로그·OCR 임시파일뿐 |
| **DB IOPS 3,750** | 🟢 Low | 쓰기 9 KiB/s, 히트율 99.99% |
| **DB throughput 29 MiB/s** | 🟢 Low(정상) / 🟠(이벤트) | 평균 0.03%. 단 **덤프 복원·롤업·백필·대형 ALTER 순간**은 여기에 붙는다 |
| **Network latency** Δ | 🟢 Low | 같은 AD 사설 서브넷이면 0.2~0.5 ms[추정] |
| **Network bandwidth** | 🟢 Low | VM 2 Gbps, DB 1 Gbps. 응답 8~15 KB 압축 |
| **Concurrent request** | 🟢 Low | SSE 0, in-flight 1. 현재 사용자 규모 |
| **File descriptor** | 🟢 Low | 33 / 1,048,576 |
| **TCP connection** | 🟢 Low | 신규 접속 1.33/s는 DB 쪽 비용이지 소켓 고갈은 아님 |
| **DNS** | 🟢 Low | nginx가 Docker DNS 매 요청 재해석 — 의도된 트레이드오프 |
| **swap** | 🟢 Low | 필요 없음(§9). 넣으면 오히려 부트 볼륨 IOPS를 뺏는다 |
| **conntrack** | 🟢 Low | 이 규모에서 nf_conntrack 기본 상한에 안 닿는다 |

---

## 4. 「그대로 이전」했을 때 예상되는 문제 (발생 가능성 순)

각 항목은 **문제 → 원인 → 측정 방법 → 개선 방법 → 예상 효과 → 부작용 → 검증** 순서로 쓴다.

### 4.1 [확률 매우 높음] 타임존 9시간 시프트 — 조용한 데이터 오염

**문제.** 지금 MySQL은 `time_zone = SYSTEM`이고 시스템 tz가 KST다. 드라이버 DSN은 `loc=Asia/Seoul`
— "서버가 돌려주는 문자열은 KST 벽시계"라는 뜻이다. 관리형 MySQL(MDS)은 서버 타임존이 **UTC**가
기본이다[확인 필요: `SELECT @@global.time_zone, @@session.time_zone`].

**원인.** 타입별로 결과가 갈린다.

| 타입 | 개수 | 서버 tz가 UTC로 바뀌면 |
|---|---:|---|
| `TIMESTAMP` | 85 | 내부는 UTC epoch. 서버가 **세션 tz로 변환해 반환** → UTC 문자열이 나오는데 드라이버가 KST로 파싱 → **읽는 값이 9시간 밀린다** |
| `DATETIME` | 4 (`candle_bars.bar_time` 포함) | 변환 없음. 저장된 KST 벽시계가 그대로 → **안전** |
| SQL 안의 `NOW()`/`CURDATE()` | `candle_rollup.go:59`, `reference_repo.go:425` 등 | 서버가 UTC를 반환 → **KST로 저장된 `bar_time`(DATETIME)과 비교하면 9시간 어긋난다** |

즉 TIMESTAMP는 **읽기**가 틀리고, DATETIME은 **비교**가 틀린다. 두 종류가 섞여 있어서 한쪽만 고치면
더 이상해진다. 이 프로젝트는 이미 같은 계열의 사고를 겪었다 — `cmd/repair-candle-tz`가
「컨테이너 UTC가 KST 자정을 전날 15:00으로 썼다」를 되돌리려고 만들어진 도구고, 5,192행을 지웠다.

**측정 방법 (이관 직후, 트래픽 붙이기 전).**
```sql
SELECT @@global.time_zone, @@session.time_zone, NOW(), UTC_TIMESTAMP();
-- 기대: session이 +09:00, NOW()가 KST 벽시계
SELECT MAX(bar_time) FROM candle_bars WHERE bar_interval='1m';  -- KST 벽시계여야 함
SELECT MAX(updated_at) FROM news_articles;                       -- 현재 시각과 9시간 차가 없어야 함
```

**개선 방법 (둘 다 한다).**
1. **DSN에 못박는다** — 서버 기본값이 무엇이든 이긴다. 코드와 함께 버전 관리된다.
   ```
   MYSQL_DSN=...&loc=Asia%2FSeoul&time_zone=%27%2B09%3A00%27
   ```
   (`time_zone='+09:00'`의 URL 인코딩. go-sql-driver는 미지 파라미터를 세션 시스템 변수로 넘긴다.)
   명명 타임존(`Asia/Seoul`) 대신 오프셋을 쓰는 이유: 명명 tz는 서버에 tz 테이블이 적재돼 있어야
   하는데 MDS에 적재돼 있다는 보장이 없다[확인 필요]. 한국은 서머타임이 없어 오프셋 고정이 무손실이다.
2. **DB 시스템 구성에서도 `time_zone = +09:00`.** DSN을 안 거치는 경로(콘솔 쿼리, Performance Hub,
   덤프 도구)까지 같은 시간을 보게 하려는 것.

**예상 효과.** 시프트 0. **부작용.** 없음 — 현재 로컬과 **동일한 상태로 맞추는 것**이라 동작 변화가
아니다. **rollback.** DSN 파라미터 제거.
**검증.** 위 쿼리 3종 + `go test ./internal/service/...`
(`TestDetectMarketSession_usSessionSpanningKSTMidnight` 등이 KST 전제를 검사한다)
+ 이관 후 첫 장중에 1분봉 지연을 `/metrics`로 확인.

### 4.2 [확률 높음] 배포 한 번이 디스크를 갉아먹는다

**문제.** `restart.ps1` → `docker compose up -d --build`. OCI에서도 같은 절차를 쓰면 **2 OCPU ARM VM이
빌더가 된다.** Go 9바이너리 + `npm ci` + `next build`.

**원인.** ① 빌드가 CPU 200%를 수 분간 점유 → 그동안 API p95가 튀고 시세 폴러가 밀린다.
② 빌드 캐시가 **정리 정책 없이** 쌓인다(로컬 실측 31.6 GB / 부트 50 GiB).

**측정.** `docker system df`, `df -h /var/lib/docker`, 배포 중 `docker stats` +
`/metrics`의 `quantport_http_requests_in_flight`·응답시간.

**개선.**
- **즉시(P0)**: `/etc/docker/daemon.json`에 빌더 GC 상한 + 로그 상한(§9.4). 배포 스크립트 끝에
  `docker image prune -f`.
- **초기 운영(P1)**: **VM에서 빌드하지 않는다.** CI(ARM64 러너 또는 buildx/QEMU)에서 빌드해
  OCIR에 푸시하고, VM은 `docker compose pull && up -d`만 한다. 배포 중 CPU 점유가 사라지고
  롤백이 태그 되돌리기로 끝난다.

**예상 효과.** 배포 중 CPU 스파이크 제거, 디스크 상한 확보. **부작용.** CI 파이프라인이 늘어난다.
**rollback.** compose에 `build:`를 남겨 두면 언제든 VM 빌드로 되돌아간다.

### 4.3 [확률 높음] 무제한 로그

**문제/원인.** §0.6.
**측정.** `docker ps -q | xargs -I{} docker inspect --format '{{.LogPath}}' {} | xargs du -ch`
**개선.** daemon.json 전역 `log-opts`(§9.4).
**예상 효과.** 12컨테이너 × 30 MB = 360 MB로 고정.
**부작용.** 오래된 로그가 사라진다 → 그래서 **에러만 따로 파일로 남기는 잡**을 §10에 둔다.
**rollback.** daemon.json 되돌리고 `systemctl restart docker`(전 컨테이너 재기동을 동반하므로 배포 창에서).

### 4.4 [확률 높음] 포트 노출과 레이트 리밋 우회

**문제.** compose가 `6379:6379`, `8080:8080`을 **0.0.0.0에** 게시한다.

**원인.** Docker는 published 포트를 iptables **nat 테이블 DNAT**로 처리한다. OL9의 firewalld에서
공개 존을 잠가도 이 경로가 우회되는 구성이 흔하다[확인 필요 — 배포 후 외부에서 실제로 스캔해 확인].
OCI Security List/NSG가 기본 거부라 1차 방어는 있지만, **방어가 한 겹뿐이면 설정 실수 한 번에 뚫린다.**

**결과 셋.**
① 인증 없는 Redis(현재 `requirepass` 없음) 노출 → 캐시 오염·`FLUSHALL`.
② `:8080` 직접 접근 → nginx의 XFF 덮어쓰기 우회. **이 프로젝트가 실측으로 잡았던 그 구멍이
그대로 되살아난다**(XFF 회전 150요청 전부 통과).
③ `/metrics` 공개 — "nginx가 프록시하지 않으니 내부 전용"이라는 전제가 8080 직접 접근으로 깨진다.

**개선.** loopback 바인딩 + Redis 비밀번호(§6.13 오버레이).
**검증.** 외부 호스트에서 `nc -vz <VM public IP> 6379 8080` → refused/timeout이어야 한다.

### 4.5 [확률 중간] OCR이 2 OCPU를 통째로 가져간다

**문제.** `import_job.go:265` — 업로드 이미지마다 고루틴 하나. 각 이미지가 `ParseHoldingsImage`에서
tesseract를 **최대 10회**(전체 psm 11/6 → 전처리 11/6 → 좌측컬럼 ×3확대 11/6/4 → 금액컬럼 11/6/4)
부르고 사이사이 Go에서 이미지 업스케일을 한다.

장수 상한은 `maxImportImages = 10`으로 **이미 있다**(`import_job.go:33`). 무제한이 아니다.
문제는 상한의 유무가 아니라 **그 10장이 2 OCPU에서 동시에 돈다**는 것이다.

**원인.** 팬아웃이 코어 수가 아니라 업로드 장수에 묶여 있다. api 컨테이너 CPU 상한이 `2.0`이라
**박스 전체**를 쓸 수 있고, 메모리 상한은 `1 GiB`라 반대로 **여기서 먼저 OOM**이 난다 —
1080×2400 스크린샷을 3배로 키우면 RGBA로 약 93 MB고, 10장이면 그것만으로 상한을 넘는다.

**측정.** 스크린샷 10장을 실제로 업로드하면서 `docker stats quant-port-api`,
`/metrics`의 heap·goroutines, 동시에 다른 터미널에서
`curl -w '%{time_total}' /api/v1/quotes/005930`.

**개선.**
1. **세마포어 두 겹**: ① tesseract 실행을 코어 수로 묶는다(`ocr` 패키지 — 다른 호출자도 보호된다)
   ② 이미지 파이프라인(crop·업스케일)도 같은 폭으로 묶는다. CPU만 막고 메모리를 안 막으면
   OOM 킬로 옮겨갈 뿐이다. 진행률 UI가 이미 있으므로 큐잉이 사용자에게 보인다.
2. **`OMP_THREAD_LIMIT=1`**: tesseract는 OpenMP로 코어 수만큼 스레드를 띄운다. 프로세스 단위로
   이미 병렬화했으므로 그 위에 스레드까지 겹치면 스케줄러만 바쁘다.
3. api 컨테이너 `cpus: 2.0 → 1.5`, `GOMEMLIMIT=800MiB`(§6.4).

**예상 효과.** OCR 중에도 API가 최소 0.5 OCPU를 확보. **부작용.** 10장 업로드의 총 처리 시간이
늘어난다(병렬 10 → 2). 사용자 1명의 대기가 길어지는 대신 **다른 모든 사용자의 응답이 유지된다**
— 이 트레이드오프는 안정성 쪽이 옳다.
**rollback.** 세마포어 크기를 환경변수(`OCR_MAX_CONCURRENCY`)로 두고 0=무제한.

### 4.6 [확률 중간] 배포/크래시가 시세 적재를 끊고 Toss 토큰을 태운다

**문제.** api 프로세스 하나가 HTTP + SSE + 배경 루프 13개(§2.1) + OCR + 스키마 마이그레이션을 겸한다.
세션 기록: **`streamToken`은 프로세스 메모리가 비면 무조건 `IssueFresh`를 부르고, 새 발급이
이전 토큰을 즉시 401로 만든다.** 즉 재기동 1회 = 토큰 1회전.

**원인.** 관심사 결합. OCI에서는 인스턴스가 1대뿐이라 **이 프로세스가 곧 SPOF**다.

**개선.** 지금 규모에서 폴러를 별 컨테이너로 떼는 것은 과하다(리더 선출·상태 공유 코드가 새로 필요하고,
이 프로젝트는 이미 「다중 인스턴스 전제 작업」을 기각한 바 있다). 대신 **재기동 횟수를 줄이는 데 집중**한다:
- OOM 킬 방지(`GOMEMLIMIT`, OCR 상한) → 크래시성 재기동 제거
- 배포 시 이미지 pull 후 `up -d` → 다운타임 초 단위
- `docker events`의 api die/restart를 알람(§10)
- 향후(P2): 폴러 분리는 두 번째 Always Free A1을 쓸 때 함께.

### 4.7 [확률 중간] 마이그레이션은 배포와 함께, 되돌릴 백업은 24시간

**문제.** `EnsureSchema`가 `server.New()`에서 **60초 타임아웃**으로 돈다. MySQL.FREE 백업 보존 1일.

**원인.** 배포 = 마이그레이션. 1 ECPU + 29 MiB/s에서 370만 행 테이블 ALTER는 60초를 넘긴다[추정].
넘기면 컨텍스트 취소 → api 기동 실패 → 워커도 `depends_on: service_healthy`라 못 뜬다 → **전면 정지.**

**개선.**
- 대형 ALTER는 **배포와 분리해 수동으로**. `ALGORITHM=INPLACE, LOCK=NONE`을 명시하고 실패하면 중단
  (자동으로 COPY로 떨어지지 않게).
- 배포 전 **논리 덤프를 Object Storage로**(§8.5) — 보존 1일을 보완하는 유일한 수단.
- `EnsureSchema` 타임아웃을 환경변수화해 큰 마이그레이션 배포에서만 늘린다.

### 4.8 [확률 낮음–중간] 신규 접속 1.33/s가 원격 DB에서 비용이 된다

**문제.** `Connections` 356,054 / 268,045초. 로컬 루프백에서는 공짜였다. 원격 + TLS면 접속당
TCP 3-way + TLS 핸드셰이크 + 인증 = **수 ms**[추정 2~6 ms, 같은 AD].

**원인 후보(미규명).** ① CLI/백필을 많이 돌린 3일간의 편향 ② `connMaxIdleTime=5분` — 폴링 주기가
5분보다 긴 배경 루프는 매번 새 커넥션을 연다 ③ 워커 재기동.

**측정 전에 고치지 않는다.** 이관 후 **유휴 1시간** 동안
`SHOW GLOBAL STATUS LIKE 'Connections'` 델타를 재면 원인이 갈린다(유휴에도 크면 ②).

**개선(측정 후).** ②로 확인되면 `connMaxIdleTime` 5분 → 25분(`connMaxLifetime` 30분보다 짧게 유지).
**부작용.** 유휴 커넥션이 더 오래 살아 DB 스레드를 점유 — 상한 24 대비 실측 8이라 무해.

### 4.9 [확률 낮음] ARM64 이미지 호환성

베이스 이미지 전부 공식 multi-arch다: `golang:1.25-bookworm`, `debian:bookworm-slim`,
`node:20-bookworm-slim`, `redis:7-alpine`, `nginx:1.27-alpine`.
`tesseract-ocr`/`-kor`/`-eng`는 Debian arm64에 있다. 빌드는 `CGO_ENABLED=0`이라 크로스 이슈가 없다.

**검증 방법**(추측하지 말고 실제로):
```bash
docker buildx build --platform linux/arm64 -t qp-api:arm64 ./be
docker run --rm --platform linux/arm64 qp-api:arm64 sh -c 'tesseract --version'
docker image inspect qp-api:arm64 --format '{{.Architecture}}'   # arm64
```
그리고 실제 스크린샷으로 OCR 회귀를 본다 — tesseract 결과가 아키텍처별 부동소수점 차이로
미세하게 달라질 수 있다[추정]. `internal/import/pipeline`의 OCR 테스트가 그 회귀망이다.

### 4.10 [확률 낮음] Always Free 회수 정책

Always Free 컴퓨트는 장기 유휴 시 회수 대상이 될 수 있다[확인 필요 — Always Free 전용 계정인지
유료 전환 계정인지에 따라 다르다]. 이 워크로드는 폴러가 상시 돌아 완전 유휴는 아니지만,
**콘솔에서 정책 문구를 직접 확인**하고 유료 전환 계정이면 무시해도 된다.

---
## 5. Recommended Architecture

**바꾸는 것은 4가지뿐이다**: 경계(포트·로그·디스크·동시성), TLS, DB 접속 규약(tz·TLS·풀),
그리고 빌드 위치. **토폴로지는 그대로 둔다** — 지금 구조가 이 규모에 맞다.

```
                          인터넷
                            │ :443 TLS (certbot, 자동 갱신)
                            │ :80 → 301 리다이렉트만
   OCI VCN                  ▼
  ┌─────────────────────────────────────────────────────────────────┐
  │ 공개 서브넷 — NSG: ingress 22(내 IP만) / 80 / 443 만            │
  │                                                                 │
  │  VM.Standard.A1.Flex  2 OCPU / 12 GiB / ARM64 / OL9 / 50 GiB    │
  │  ┌───────────────────────────────────────────────────────────┐  │
  │  │ nginx :80,:443           shares 1024  cpus .5   128M      │  │
  │  │   ├─ /api/*  → api:8080     (XFF 덮어쓰기 = 유일한 입구)   │  │
  │  │   ├─ SSE 경로 buffering off, read_timeout 3600s           │  │
  │  │   └─ /       → fe:3000                                    │  │
  │  ├───────────────────────────────────────────────────────────┤  │
  │  │ api   127.0.0.1:8080 만 게시  shares 2048 cpus 1.5  1 GiB │  │
  │  │   GOMEMLIMIT=800MiB · OCR 세마포어 2 · 잡당 이미지 ≤10    │  │
  │  ├───────────────────────────────────────────────────────────┤  │
  │  │ fe    (게시 없음, 내부만)     shares 1024 cpus 1.0  512M  │  │
  │  │ redis 127.0.0.1:6379 만 게시  shares 1024 cpus .75  768M  │  │
  │  │        requirepass · maxmemory 512mb · volatile-lru       │  │
  │  ├───────────────────────────────────────────────────────────┤  │
  │  │ worker × 8   shares 256  cpus 0.5  256M  (변경 없음)      │  │
  │  └───────────────────────────────────────────────────────────┘  │
  │  Docker daemon: log max-size 10m×3 · builder GC 5GB              │
  └───────────────────────────┬─────────────────────────────────────┘
                              │ 사설 서브넷 · TLS · time_zone='+09:00'
                              ▼
  ┌─────────────────────────────────────────────────────────────────┐
  │ 사설 서브넷 — NSG: VM 서브넷에서 3306/33060 만                  │
  │  MySQL HeatWave  1 ECPU / 8 GiB   ★ HeatWave 클러스터 미연결    │
  │  time_zone=+09:00 · slow_query_log ON · long_query_time=1        │
  └─────────────────────────────────────────────────────────────────┘
                              │ 주 1회 논리 덤프 (보존 1일 보완)
                              ▼
                    Object Storage (Always Free 20 GiB)
```

### 「구조를 그대로」 vs 「구조를 바꿨을 때」

| | 그대로 옮겼을 때 | 위 구조로 바꿨을 때 | 얻는 것 |
|---|---|---|---|
| 디스크 | 무제한 로그 + 빌드 캐시 → 언젠가 100% | 로그 360 MB 상한 + 빌더 5 GB 상한 | **디스크 고갈 제거**. 근거: 로컬 실측 31.6 GB |
| 노출면 | :80, :6379, :8080 (+ SSH) | :443, :80(리다이렉트), SSH 제한 | Redis 노출·레이트리밋 우회·`/metrics` 유출 제거 |
| CPU 경합 | 상한만 있고 우선순위 없음 | shares로 API 8:1 우대 | 배치·OCR 중에도 API가 죽지 않음 |
| OCR | 사용자 1명이 박스 점유 | 동시 2개 + 장수 상한 | p95 보호. 대가는 업로드 처리 시간 |
| DB 시각 | 9시간 시프트 | DSN + 서버 설정 이중 고정 | 데이터 정합성 |
| 배포 | VM에서 빌드(수 분 CPU 200%) | 레지스트리 pull(수 초) | 배포 중 성능 저하 제거 |
| 백업 | 24시간 | 24시간 + 주간 논리 덤프 | 실수 복구 창 확보 |

**바꾸지 않는 것**: 컨테이너 개수, 워커 분리 방식, DB 큐, 캔들 계층, 커넥션 풀 배정, 인덱스.
전부 실측 근거를 갖고 이미 결정된 것들이다.

---

## 6. Container Strategy

### 6.1 Kubernetes가 필요한 규모인가 — **아니다**

| 판단 기준 | 이 시스템 | K8s가 필요해지는 지점 |
|---|---|---|
| 노드 수 | 1 | 3+ |
| 무중단 배포 요구 | 없음(개인 서비스, 초 단위 다운타임 허용) | SLA 있음 |
| 오토스케일 | 불가(2 OCPU 고정, Always Free) | 수평 확장 여지가 있을 때 |
| 상태 없는 복제본 | api는 배경 폴러 때문에 복제 불가 | 상태 분리가 끝났을 때 |
| 운영 인력 | 1인 | 전담 |

K3s조차 컨트롤 플레인에 0.5~1 GiB RAM과 지속적인 CPU를 먹는다. **2 OCPU 중 상당 부분을 오케스트레이션에
쓰는 셈**이고, 얻는 것(셀프힐링·롤링업데이트)은 `restart: unless-stopped` + healthcheck로 이미 90% 있다.
서비스 메시는 컨테이너 12개가 한 브리지 네트워크에 있는 구성에서 순수 손실이다.

### 6.2 Docker Compose로 충분한가 — **그렇다**

이미 갖춰져 있는 것: 리소스 상한, healthcheck, `depends_on: condition: service_healthy`,
restart policy, named volume, 프로파일(`prod`), 공용 이미지 + `command:` 분기.
빠진 것은 **로그 상한 · CPU 가중치 · 포트 바인딩 범위 · 메모리 소프트리밋** 넷뿐이다.

### 6.3 Application과 Batch를 별도 컨테이너로 분리해야 하는가 — **이미 했고, 옳다**

근거는 실측이다: 워커 8개의 유휴 RSS 합이 46 MiB로 **분리 비용이 거의 0**인 반면,
얻는 격리는 실질적이다 — `warnings_sync`(전 종목 2시간)나 외부 사이트를 긁는 `worker-earnings`가
느려지거나 429를 맞아도 api 컨테이너가 함께 멈추지 않는다.

다만 **분리가 지켜 주지 않는 경계가 하나 있다: DB.** 워커 8개와 api가 결국 1 ECPU 하나에서 만난다.
그래서 §7의 시간 분산과 §8의 청크 전략이 필요하다.

### 6.4 CPU / memory limit을 어떻게 설정해야 하는가

**절대값을 확정하지 않는다.** 아래는 관측치에서 출발한 **초기값**과 조정 규칙이다.

| 서비스 | 관측 RSS | 초기 memory | 초기 cpus | 근거 |
|---|---:|---|---|---|
| api | 37.9 MiB | **1 GiB** (유지) + `GOMEMLIMIT=800MiB` | **1.5** (2.0에서 하향) | OCR 피크 미관측. GOMEMLIMIT이 cgroup 상한 **전에** GC를 강하게 돌려 OOM 킬을 회피 |
| redis | 42.8 MiB | **768M**, `maxmemory 512mb` | 0.75 | 현재 키 27,935/42.8 MiB. 1.5 GiB는 12 GiB 박스에서 과배정 |
| fe | 59.2 MiB | 512M (유지) | 1.0 | Next standalone. SSR 피크 미관측 |
| nginx | 51.5 MiB | 128M (유지) | 0.5 | 상한의 40% 사용 중 — 여유가 크지 않다, 관찰 대상 |
| worker × 8 | 4.2–8.8 MiB | 256M (유지) | 0.5 | 이미 실측 기반(peak 14 MB → 18배) |

**조정 규칙(측정 후):**
1. 1주일간 `docker stats --no-stream`을 5분 간격으로 수집 → 서비스별 **P99 RSS**를 구한다.
2. memory limit = P99 RSS × 3, 최소 128M. 3배인 이유: Go의 GC 힙은 라이브셋의 2배까지 커지고,
   거기에 OS 페이지캐시·스택 여유를 더한다.
3. **좁게 잡지 않는다.** compose 주석의 판단이 옳다 — "OOM kill이 배치를 조용히 죽이는 쪽이 더 위험하다".
4. cpus는 상한일 뿐 예약이 아니므로 합이 2를 넘어도 된다. **중요한 것은 상한이 아니라 §6.5의 가중치다.**

### 6.5 CPU shares / cpuset이 필요한가 — **shares는 필요, cpuset은 불필요**

**shares (필요).** 현재 CPU 상한의 합은 8.5인데 실제는 2다. 경합이 나면 CFS가 **균등 분배**한다
— 워커 4개가 동시에 돌면 api가 1/5를 받는다. `cpu_shares`는 유휴일 때는 아무 영향이 없고
**경합할 때만** 비율을 바꾼다. 즉 "성능과 안정성을 맞바꾸는" 종류의 튜닝이 아니다.

```
api 2048 : fe/nginx/redis 1024 : worker 256
→ 전면 경합 시 api가 워커 1개의 8배를 받는다
```

**cpuset (불필요).** OCPU가 2개뿐이라 고정할 수 있는 자유도가 없다. 워커를 CPU 1번에 묶으면
api가 놀 때도 워커가 0번을 못 쓴다 — **총 처리량만 잃는다.**

### 6.6 restart policy — **현재 `unless-stopped`가 정답**

`always`와 달리 운영자가 명시적으로 멈춘 컨테이너를 데몬 재시작이 되살리지 않는다.
장애 조사 중 컨테이너를 세워 둔 것이 리부팅으로 되살아나면 조사가 망가진다.
**단, 크래시 루프 방지 장치가 없다** — Docker는 지수 백오프를 하지만 무한히 재시도한다.
§10의 "5분 내 재기동 3회" 알람이 그 자리를 메운다.

### 6.7 healthcheck 설계

현재 상태:
- api: `curl /health` 30s/5s/3회/start 20s — **적절**
- fe: node로 HTTP 요청(이미지에 curl 없음) — **적절**
- nginx: `wget /health` — nginx→api 경로까지 검사하므로 **적절**
- redis: `redis-cli ping` — 적절
- **worker 8개: `healthcheck: disable: true`** ← 여기가 빈 곳

워커는 HTTP 포트가 없어 healthcheck를 끄는 선택이 자연스럽지만, 결과적으로 **워커가 살아 있는지
아무도 모른다.** 프로세스가 죽으면 Docker가 재기동하지만, **살아 있으면서 아무 일도 안 하는 상태**
(폴링 루프가 패닉으로 멈춤, DB 커넥션이 영구 실패)는 감지되지 않는다.

**개선(P1, 비용이 낮다):** 워커가 4초마다 큐를 폴링하므로, 폴링 성공 시 파일에 타임스탬프를 찍고
healthcheck가 그 파일의 나이를 본다.
```yaml
healthcheck:
  test: ["CMD-SHELL", "test $(( $(date +%s) - $(stat -c %Y /tmp/alive 2>/dev/null || echo 0) )) -lt 120"]
  interval: 60s
  timeout: 5s
  retries: 3
```
`/health` 엔드포인트를 워커에 새로 다는 것보다 가볍다.
**주의:** healthcheck 실패는 Docker가 재기동해 주지 않는다(compose 단독). 알람의 신호로 쓰는 것이
목적이고, 자동 재기동이 필요하면 `autoheal` 류 사이드카가 필요한데 — **이 규모에서는 과하다.**
알람으로 충분하다.

### 6.8 Graceful shutdown

**현재 잘 돼 있다.** `cmd/api/main.go`가 SIGTERM을 받고 `srv.Shutdown(ctx)`을 부르며,
`Server.Shutdown`은 "in-flight HTTP를 먼저 비우고 → 배경 워커 취소 → 풀 종료" 순서다.
`bgCtx`/`bgCancel`로 배경 루프 전부가 한 번에 멈춘다.

**빠진 것 두 가지:**
1. **compose에 `stop_grace_period`가 없다** → 기본 10초. 시세 폴러와 SSE가 정리되기에 빠듯할 수 있다.
   api는 30초로, 워커는 **잡 실행 중일 수 있으니** 60초 권장.
   (배치는 `context.WithTimeout(ctx, 2*time.Hour)`로 돌아 어차피 중간에 끊긴다 — 그래서 §7의
   **idempotency**가 더 중요하다.)
2. **OOM 킬에는 graceful shutdown이 없다.** SIGKILL이다. §4.5·§6.4가 그래서 필요하다.

### 6.9 컨테이너 로깅 제한

서비스마다 `logging:`을 12번 쓰는 대신 **daemon.json에 전역 기본값**을 둔다(§9.1(1)).
한 곳에서 보이고, 새 서비스를 추가할 때 빠뜨리지 않는다.
**단, daemon.json 변경은 기존 컨테이너에 소급되지 않는다** — 재생성(`up -d --force-recreate`)이 필요하다.

### 6.10 Docker volume 구성

현재: `quant_port_redis_data`(Redis AOF/RDB), `quant_port_media`(사용자 업로드), 그리고
`./be/docs/BATCH_SCHEDULE.md` 바인드 마운트.

| 볼륨 | 판단 |
|---|---|
| `quant_port_media` | **필수 유지.** 영구 데이터. writable layer에 두면 recreate마다 소실 — 주석대로다 |
| `quant_port_redis_data` | 유지. 단 **Redis 데이터는 전부 캐시**이므로 소실돼도 재구성된다. 부트 볼륨 IOPS를 아끼려면 `appendonly no` + `save ""`(순수 캐시)로 두는 선택지도 있다 → §12에서 P2 |
| `BATCH_SCHEDULE.md` 바인드 | 유지. **다만 이미지에도 복사돼 있어** 마운트가 없으면 이미지 버전이 쓰인다. OCI에서 리포 체크아웃 경로가 달라지면 조용히 옛 시간표가 뜬다 → 배포 스크립트에서 경로 존재를 검사할 것 |
| OCR 임시파일 | 현재 writable layer(compose 주석대로 tmpfs 미사용). **이 판단은 유지**한다 — 12 GiB 중 여유가 많지만 tmpfs는 컨테이너 256M/1G 상한에 계산되어 OOM 위험을 만든다. 대신 **잡 실패 시 잔여 디렉터리 정리**를 확인할 것(§12 P1) |

### 6.11 이미지 크기 줄이기

현재 api 381 MB / fe 446 MB. **급하지 않다** — 레지스트리에서 pull하는 시간(2 Gbps에서 수 초)과
50 GiB 중 몇 백 MB일 뿐이다. 그래도 값싼 것부터:

| 방법 | 절감[추정] | 대가 |
|---|---|---|
| `tesseract-ocr-kor/eng`만 유지(이미 그렇게 함) | — | — |
| 빌드 스테이지에서 `go build` 9회 → 하나의 `RUN`으로 묶기 | 레이어 수 ↓, 빌드 캐시 효율 ↑ | 부분 재빌드 불가 |
| runtime을 `debian:bookworm-slim` 유지 | — | **alpine으로 바꾸지 말 것** — tesseract 언어팩과 musl 조합에서 회귀가 잦다 |
| `curl` 제거하고 healthcheck를 api 바이너리의 서브커맨드로 | ~15 MB | 코드 추가 |

**결론: 이미지 최적화는 P2다.** 지금 지출을 정당화하지 못한다.

### 6.12 ARM64 검증

§4.9. 요약: `buildx --platform linux/arm64`로 빌드 → `docker image inspect`로 아키텍처 확인 →
`tesseract --version` → **실제 스크린샷 OCR 회귀 테스트**(`internal/import/pipeline`).
"multi-arch 태그가 있으니 되겠지"로 넘기지 않는다.

### 6.13 실제 compose 오버레이 (제안)

기존 `docker-compose.yml`을 **고치지 않고** `docker-compose.oci.yml`을 겹친다.
로컬 개발 흐름이 그대로 유지되고, 클라우드 전용 설정이 한 파일에 모인다.

```bash
docker compose -f docker-compose.yml -f docker-compose.oci.yml --profile prod up -d
```

```yaml
# docker-compose.oci.yml — OCI 전용 오버레이.
# 값의 근거는 docs/OCI_MIGRATION_PLAN.md §6. 숫자는 초기값이며 §10의 측정으로 조정한다.
services:
  nginx:
    ports:
      - "80:80"
      - "443:443"        # certbot 인증서 마운트 후
    volumes:
      - /etc/letsencrypt:/etc/letsencrypt:ro
    cpu_shares: 1024
    stop_grace_period: 20s

  api:
    ports:
      - "127.0.0.1:8080:8080"    # ★ 외부 노출 차단. nginx만 입구.
    environment:
      # cgroup 상한 1 GiB보다 낮게 잡아, OOM 킬 대신 GC가 먼저 일하게 한다.
      # Go 1.25는 GOMAXPROCS를 cgroup에서 자동으로 잡으므로 그쪽은 설정하지 않는다.
      GOMEMLIMIT: "800MiB"
      OCR_MAX_CONCURRENCY: "2"   # = OCPU 수. 0이면 무제한(롤백용)
      TRUST_PROXY_HEADERS: "true"
      AUTH_COOKIE_SECURE: "true"
      # tz는 DSN에 못박는다 — §4.1
      MYSQL_DSN: ${MYSQL_DSN}
    cpu_shares: 2048
    stop_grace_period: 30s
    deploy:
      resources:
        limits:
          cpus: "1.5"     # 2.0 → 1.5. OCR이 박스를 통째로 못 가져가게.
          memory: 1G

  fe:
    cpu_shares: 1024

  redis:
    ports:
      - "127.0.0.1:6379:6379"    # ★
    command:
      - redis-server
      - --maxmemory
      - ${REDIS_MAXMEMORY:-512mb}
      - --maxmemory-policy
      - volatile-lru
      - --requirepass
      - ${REDIS_PASSWORD:?set REDIS_PASSWORD}
    cpu_shares: 1024
    deploy:
      resources:
        limits: { cpus: "0.75", memory: 768M }

  # 워커 8개 공통 — YAML 앵커가 파일 간 상속되지 않으므로 서비스마다 적는다.
  worker-close-domestic:  { cpu_shares: 256, stop_grace_period: 60s }
  worker-close-overseas:  { cpu_shares: 256, stop_grace_period: 60s }
  worker-scores:          { cpu_shares: 256, stop_grace_period: 60s }
  worker-index:           { cpu_shares: 256, stop_grace_period: 60s }
  worker-fetch:           { cpu_shares: 256, stop_grace_period: 60s }
  worker-research:        { cpu_shares: 256, stop_grace_period: 60s }
  worker-earnings:        { cpu_shares: 256, stop_grace_period: 60s }
  worker-filings:         { cpu_shares: 256, stop_grace_period: 60s }
```

> `REDIS_URL`도 `redis://:${REDIS_PASSWORD}@redis:6379/0`으로 함께 바꿔야 한다(api + 워커 8개).
> 로그 상한은 여기 두지 않고 daemon.json에 전역으로 둔다(§9.1(1)) — 12번 반복하지 않기 위해서.

---

## 7. Batch Strategy

### 7.1 실행 구조 비교

| 방식 | 평가 |
|---|---|
| **별도 batch container** (현재) | ✅ **채택 유지.** 격리·리소스 상한·독립 재기동. 비용 46 MiB |
| container 내부 cron | ❌ PID 1이 cron이 되어 잡 실패가 컨테이너 상태에 안 보인다. 로그가 파일로 갇힌다 |
| host cron | ❌ 실행 환경(환경변수·바이너리 버전)이 이미지와 갈라진다. 이미 `.env`가 컨테이너 것 |
| host systemd timer | △ cron보다 낫다(로그가 journald, 실패 상태 추적). 하지만 **잡 정의가 리포 밖으로 나간다** |
| application container에서 실행 | ❌ 지금 이미 4개(LLM 2 + 뉴스 브리핑 2)가 api 안에 있다 → §7.2 |

### 7.2 남아 있는 결합: api 안에서 도는 배치 4개

`BATCH_SCHEDULE.md` 실측 — 워커 컬럼이 `api`인 잡이 넷이다:
`llm_summary_domestic`, `llm_summary_overseas`, `news_briefing_domestic`, `news_briefing_overseas`.

**왜 문제인가.** 이들은 LLM 호출이라 **응답이 수십 초** 걸린다. 실패하면 api 프로세스에서 재시도가
돌고, api 재기동은 이 잡의 스케줄을 리셋한다. 그리고 `api`는 이미 SPOF다(§4.6).

**왜 지금 당장 옮기지 않는가.** LLM 잡은 하루 몇 회, 시장별 1건이다. 부하가 아니라 **결합**이 문제라
긴급도가 낮다. 다만 워커를 새로 만드는 비용이 낮으므로(`cmd/worker-llm` + compose 1블록, 이미 8번 한 패턴)
**P1**에 둔다.

### 7.3 API 트래픽과 배치가 DB에서 만나는 지점

```
사용자 → api ──┐
              ├──▶ MySQL 1 ECPU ◀── 워커 8 (큐 폴링 4초 × 17루프 + 잡 실행)
Toss WS  ─────┘
```

**측정으로 확인된 것**: 배치 큐 폴링만으로 **17개 루프 × 4초 = 4.25 QPS**가 상시 흐른다.
이것 자체는 가볍지만(PK 인덱스 히트), **1 ECPU에서 공짜는 아니다.**
`batchQueuePollInterval`을 4초 → 10초로 늘리면 4.25 → 1.7 QPS가 된다.
대가는 "관리자가 「지금 실행」을 눌렀을 때 최대 10초 대기". **측정 후 결정할 항목**으로 둔다
— 지금 DB CPU를 모르는 상태에서 UX를 먼저 깎을 이유가 없다.

### 7.4 배치가 API를 느리게 만드는 것을 막는 장치

이미 있는 것(유지):
- **시간 분산** — `BATCH_SCHEDULE.md`가 00:00~22:00에 걸쳐 잡을 흩어 놓았고, 같은 provider를 쓰는 잡은
  의도적으로 시각을 벌렸다. 원칙 4("같은 provider 잡은 시각을 벌리고 그 위에 게이트")가 문서화돼 있다.
- **동시 실행 방지** — `FOR UPDATE SKIP LOCKED` + `providerGate`.
- **job timeout** — 2시간.
- **레이트 리밋 자기 조절** — `apibudget.Check`가 기다리지 않고 에러를 반환, 호출측이 `MinInterval`만큼 잔다.

**추가할 것:**

| 항목 | 제안 | 근거 |
|---|---|---|
| **트랜잭션 크기** | 대량 upsert는 **500~1,000행 단위 커밋**. 한 트랜잭션 안에서 수만 행을 잡지 않는다 | 1 ECPU + 29 MiB/s에서 큰 트랜잭션은 redo·언두 이력을 키우고 **다른 세션의 읽기까지 느려진다**. 락 대기 122 ms(§0.3)의 유력 후보 |
| **keyset pagination** | 배치 스캔은 `WHERE symbol > ? ORDER BY symbol LIMIT n` 형태로. `OFFSET`은 금지 | `stock_daily_closes` 370만 행에서 `OFFSET 300000`은 30만 행을 버리려고 읽는다 |
| **bulk insert** | 이미 다중행 upsert로 전환한 사례가 있다(가격 폴러 1,080/분 → 1/분). 나머지 배치도 같은 패턴으로 | 왕복 수 = 원격 DB에서 latency × N |
| **retry/backoff** | 잡 실패 시 즉시 재큐잉하지 않는다. 지수 백오프 + **최대 재시도 3회** | "batch 실패 시 재시도 폭증"이 이 구조의 유일한 자기증폭 경로 |
| **idempotency** | 이미 upsert 기반이라 대체로 안전. **2시간 타임아웃으로 중간에 끊긴 잡이 재실행됐을 때** 중복이 안 생기는지 잡별로 확인 | `stop_grace_period` 60초로도 못 끝내는 잡이 있다 |
| **배치 중 DB 부하 자기 제한** | 큰 잡은 청크 사이에 `time.Sleep(50ms)` 같은 페이싱을 둔다 | 29 MiB/s를 순간적으로 다 쓰면 API 쿼리가 뒤에 선다. **총 실행시간이 늘어도 p95를 지키는 쪽**이 이 시스템의 목표 |

### 7.5 배치 시각 재검토 (OCI 기준)

`warnings_sync`가 01:30부터 **약 2시간**(전 종목 17,900). `data_retention` 04:30,
`candle_rollup` 05:30 — 겹치지 않는다. 다만 `earnings_overseas`(live)가 **00:00부터 5분**,
그리고 미국 정규장이 KST 22:30~05:00이라 **`warnings_sync` 2시간이 미국 장중과 통째로 겹친다.**
장중에는 시세 폴러·WS·캔들 적재가 함께 돈다.

**측정 항목**: 미국 장중(01:30~03:30 KST)의 DB CPU와 API p95를, 같은 시간대의 비장중과 비교.
차이가 유의하면 `warnings_sync`를 **미국 마감 후(05:30 이후)** 로 옮긴다 — `candle_rollup`과의
순서만 지키면 된다.

---

## 8. MySQL Optimization

### 8.1 Connection

**현재 배정** (근거는 `/metrics` 실측, 주석에 남아 있음):

```
api 24 + worker 8 × 4 = 56        실측 최대 사용: api 8, Max_used_connections 38(CLI 포함)
CLI 기본 4 (NewMySQLForCLI)
```

**판단: 배정을 더 줄이지 않는다.** 1 ECPU에서 동시 유효 쿼리가 2~4개[추정]라 커넥션을 늘려도
처리량이 안 늘지만, 줄이면 재배포 순간(9프로세스 동시 기동)에 여유가 사라진다.
**`max_connections`는 올리지 않는다** — 방향이 반대다(메모리 기록과 일치).

**이관 시 반드시 할 것:**

| 항목 | 조치 | 근거 |
|---|---|---|
| `max_connections` 확인 | `SELECT @@max_connections;` → 56이 그 값의 몇 %인지 계산 | MDS는 셰이프 메모리에서 유도한다[확인 필요]. 56 < 상한의 40%면 그대로 |
| **TLS** | DSN에 `tls=preferred`(초기) → 커스텀 CA 등록 후 `tls=custom`(P1) | MDS가 `require_secure_transport=ON`이면 **평문 접속이 전부 실패**한다[확인 필요]. 이관 첫 시도가 여기서 막히는 사고가 흔하다 |
| **connection leak** | `/metrics`의 `mysql_open_connections`가 유휴에도 상한에 붙어 있으면 누수 | 현재 open=8/24, 정상 |
| **connection timeout** | 이미 DSN 기본값 주입됨: `timeout=5s readTimeout=30s writeTimeout=30s` | 드라이버 기본이 무제한이라는 것을 이미 잡아 뒀다 — 유지 |
| **idle 재접속** | §4.8. 측정 후 `connMaxIdleTime` 조정 | |

### 8.2 Query — 점검 방법 (지금은 못 하고, 이관 후 첫 주에 한다)

**막힌 것부터 뚫어야 한다.** 현재 앱 계정은 `performance_schema` SELECT 권한이 없다
(실측: `ERROR 1142 ... events_statements_summary_by_digest`). **측정할 수 없으면 최적화할 수 없다.**

```sql
-- P1: 관측 전용 계정
CREATE USER 'qp_observer'@'%' IDENTIFIED BY '...';
GRANT SELECT ON performance_schema.* TO 'qp_observer'@'%';
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'qp_observer'@'%';
```

그 위에서 **주 1회** 아래를 돌려 상위 항목만 손댄다(무작정 인덱스를 추가하지 않기 위한 절차):

```sql
-- (1) 총 소요시간 상위 — "가장 비싼 쿼리"
SELECT LEFT(DIGEST_TEXT,120) q, COUNT_STAR cnt,
       ROUND(SUM_TIMER_WAIT/1e12,1) tot_s, ROUND(AVG_TIMER_WAIT/1e9,1) avg_ms,
       SUM_ROWS_EXAMINED ex, SUM_ROWS_SENT sent,
       ROUND(SUM_ROWS_EXAMINED/GREATEST(SUM_ROWS_SENT,1),1) ex_per_sent,
       SUM_NO_INDEX_USED noidx, SUM_SORT_MERGE_PASSES smp,
       SUM_CREATED_TMP_DISK_TABLES tmpdisk
FROM performance_schema.events_statements_summary_by_digest
WHERE SCHEMA_NAME='quantport'
ORDER BY SUM_TIMER_WAIT DESC LIMIT 20;

-- (2) 인덱스를 안 쓴 것
SELECT LEFT(DIGEST_TEXT,120), COUNT_STAR, SUM_NO_INDEX_USED
FROM performance_schema.events_statements_summary_by_digest
WHERE SCHEMA_NAME='quantport' AND SUM_NO_INDEX_USED > 0
ORDER BY SUM_NO_INDEX_USED DESC LIMIT 20;

-- (3) 안 쓰는 인덱스 (제거 후보)
SELECT object_schema, object_name, index_name
FROM sys.schema_unused_indexes WHERE object_schema='quantport';

-- (4) 중복/여분 인덱스
SELECT * FROM sys.schema_redundant_indexes WHERE table_schema='quantport';
```

**판단 기준 (인덱스를 "무조건 추가"하지 않기 위한 것):**

| 신호 | 임계 | 행동 |
|---|---|---|
| `ex_per_sent` (읽은 행 / 보낸 행) | > 100 | 인덱스 후보. 100행을 보내려고 10,000행을 읽고 있다는 뜻 |
| `SUM_NO_INDEX_USED` | > 0 이고 cnt가 크다 | EXPLAIN 필수 |
| `smp` (sort merge) | > 0 | ORDER BY가 인덱스로 안 풀린다 → **복합 인덱스의 마지막 컬럼**으로 정렬 컬럼을 붙일지 검토 |
| `tmpdisk` | > 0 | GROUP BY/DISTINCT가 디스크로 샌다. 29 MiB/s에 직접 붙는다 |
| 호출 빈도 | 하루 < 10회 | **인덱스를 만들지 않는다.** 쓰기 비용이 읽기 이득보다 크다 |
| 선택도 | 매칭 행 > 전체의 20% | 인덱스를 만들어도 옵티마이저가 안 쓴다 |

그리고 **후보마다 반드시**:
```sql
EXPLAIN ANALYZE <query>;      -- 실제 실행 계획 + 실측 시간
-- 인덱스 생성 후 다시 EXPLAIN ANALYZE 하여 rows/시간 비교. 나빠지면 되돌린다.
```
이 프로젝트는 이미 이 절차대로 판단한 전례가 있다(인기순 뉴스 인덱스: 238 ms → 0.289 ms,
읽는 행 67,957 → 100 / `stock_scores` 중복 인덱스 제거: 10.5 → 4.5 MB, 쿼리 4종 계획 비교로 회귀 확인).
**같은 절차를 유지한다.**

**지금 데이터에서 이미 보이는 것 하나**: `Sort_merge_passes` 83,861건은 `sort_buffer_size`
256 KB를 넘긴 정렬이 3일에 8.4만 번 있었다는 뜻이다. **버퍼를 키우는 것이 첫 대응이 아니다**
— sort_buffer는 세션당 할당이라 56 커넥션 × 증가분이 메모리에서 나간다.
먼저 (1)의 `smp` 상위 쿼리를 찾아 **인덱스로 정렬을 없애는** 쪽을 본다.

### 8.3 Index — 현재 상태 판정

| 테이블 | 인덱스 | 판정 |
|---|---|---|
| `candle_bars` | PK `(symbol, bar_interval, bar_time)`만, 보조 0 | ✅ **이상적.** 초당 수백 행 쓰기 + 차트 조회(심볼+인터벌+시간범위)가 PK 하나로 커버 |
| `stock_daily_closes` | PK `(symbol, trade_date)` + `idx_..._date(trade_date)` | ⚠ `trade_date` 단일 인덱스: 카디널리티 447(=날짜 수), 370만 행이므로 **날짜 하나당 8,300행**. "특정 날짜 전 종목" 배치용이면 정당하지만, 인덱스 79.6 MB. **sys.schema_unused_indexes로 사용 여부 확인** |
| `stock_scores` | PK + `(trade_date, score, peer_pct)` | ✅ 이미 정리됨 |
| `news_articles` | 6개 · 57.8 MB (데이터의 45%) | ⚠ `idx_news_market_published`와 `idx_news_market_popular(market, view_count, published_at)`, `idx_news_market_updated` — market 접두사가 셋 겹친다. **redundant는 아니지만** 쓰기마다 3개를 갱신한다. 사용률 확인 대상 |

**추가 관찰:** `stock_daily_closes`가 679 MB로 최대인데 `daily_closes.go:112`를 보면
`ai_summary`(LLM 텍스트)를 이 표에 함께 담는다. 행 폭이 넓어지면 **날짜 범위 스캔이 무거워진다**
(같은 페이지에 들어가는 행 수가 줄어서). `ai_summary`를 사이드 테이블로 빼는 것이 이론적으로 옳지만,
**측정 없이 마이그레이션할 일은 아니다** → §12 P2, "차트/스코어 쿼리의 `ex_per_sent`가 높게 나오면".

### 8.4 Transaction

| 항목 | 현재 | 권고 |
|---|---|---|
| isolation level | `REPEATABLE-READ`(기본) | **유지.** `READ-COMMITTED`가 갭락을 줄여 주지만, 이 앱은 upsert 중심이라 이득이 불확실하고 **동작 변경 위험**이 있다. 락 대기가 문제로 확인된 **후에** 후보 |
| autocommit | 기본 ON, 명시적 트랜잭션은 `ClaimNext` 등 소수 | 유지 |
| long transaction | 미측정 | `SELECT * FROM information_schema.innodb_trx WHERE TIME_TO_SEC(TIMEDIFF(NOW(), trx_started)) > 5;` 를 알람에 |
| lock contention | **3일에 22,387건, 평균 122 ms** | 원인 규명이 P1. `performance_schema.data_lock_waits` + `sys.innodb_lock_waits` |
| deadlock | `SKIP LOCKED` 도입으로 큐 교착은 해결됨(Error 1213 이력) | `SHOW ENGINE INNODB STATUS`의 LATEST DETECTED DEADLOCK을 주간 점검 |
| batch transaction size | 미측정 | §7.4 — 500~1,000행 커밋 |

### 8.5 Storage — 29 MiB/s가 실제로 문제가 되는 순간

평균 쓰기 9.0 KiB/s = 상한의 **0.03%**. 정상 운영에서 스토리지 처리량은 병목이 아니다.
**붙는 순간은 네 개뿐이고, 전부 예측 가능한 이벤트다.**

| 이벤트 | 예상 부하 | 대응 |
|---|---|---|
| **초기 데이터 이관** (1.84 GB 덤프 복원) | 29 MiB/s로 나눠도 순수 전송 ~64초지만, 인덱스 재구축이 지배적 → **[추정] 20~60분** | 트래픽 붙이기 전에. `mysqlsh` 병렬 덤프/로드 사용, `--defer-table-indexes` |
| **`candle_rollup`** (05:30, 1분봉 → 5분봉 접기) | 대량 INSERT+DELETE | 이미 미국 마감 후. **유지** |
| **대형 ALTER** | 370만~560만 행 재작성 | 배포와 분리(§4.7), `LOCK=NONE` 명시 |
| **논리 덤프 백업** | 1.84 GB 읽기 | **주 1회, 새벽(예: 일요일 04:00)**. 보존 1일을 보완 |

**백업 보강(P1)** — 보존 1일은 "어제의 실수"만 되돌린다:
```bash
# VM에서 주 1회. 압축 후 Object Storage(Always Free 20 GiB)로.
mysqldump --single-transaction --set-gtid-purged=OFF --routines --events \
  -h $DB_HOST -u $DB_USER quantport | gzip > /tmp/quantport-$(date +%F).sql.gz
oci os object put -bn quant-port-backup --file /tmp/quantport-$(date +%F).sql.gz
```
`--single-transaction`인 이유: 락 없이 일관 스냅샷(InnoDB 전용). 1.84 GB × gzip ≈ [추정] 300~500 MB.
**복원 리허설을 한 번은 해 볼 것** — 검증 안 된 백업은 백업이 아니다.

---

### 8.6 HeatWave에 대한 판단 — **연결하지 않는다**

사양에 16 GiB × 1노드가 딸려 있어도, **쓰지 않는 것이 이 시스템에서 옳다.**

**1) 현재 workload에서 필요한가 — 아니다.**
- 데이터 **1.84 GB**가 InnoDB 버퍼풀에 통째로 들어간다. 실측 히트율 **99.9896%**.
  HeatWave가 해결하는 문제(디스크에서 대용량을 스캔하는 것)가 **이미 없다.**
- 읽기 경로가 대부분 **PK 포인트 조회**다(`candle_bars` PK, `stock_scores` PK).
  HeatWave는 스캔·집계·조인을 가속하지 포인트 조회를 가속하지 않는다.
- 집계는 이미 **배치가 미리 계산해서 표에 넣는다**(`stock_scores`, `market_rankings`,
  `news_briefings`). 조회 경로에 무거운 집계가 없다 — 이것이 이 아키텍처의 핵심 원칙이다
  ("적재는 워커가, 조회는 표에서").

**2) OLTP와 분석을 어떻게 구분하나.**
- OLTP = 사용자 요청 경로. **전부 InnoDB.**
- 분석 = 배치가 도는 야간 집계. 지금은 InnoDB로 충분하다.
- 구분 기준: 한 쿼리가 **10만 행 이상을 스캔하고 소수 행으로 집계**하며 **초 단위**로 걸릴 때만
  HeatWave 후보다. 그런 쿼리가 생기면 §8.2 (1)번 쿼리에 `ex_per_sent`가 1,000+로 나타난다.

**3) 쓰면 무엇을 잃나.**
- **1 ECPU를 나눠 써야 한다.** HeatWave는 `SECONDARY_LOAD` 이후 InnoDB의 변경을 클러스터로
  전파(change propagation)하는데, 그 작업이 **DB 노드의 CPU**에서 돈다. 이 시스템에서 가장
  희소한 자원을 써서, 이미 메모리에서 도는 쿼리를 가속하는 셈이다.
- 쓰기(INSERT/UPDATE)는 **전혀 가속되지 않는다.** 그런데 이 앱은 쓰기가 많다(Com_insert 33/s).
- 테이블별 `ALTER TABLE ... SECONDARY_ENGINE=RAPID` + 적재 상태 관리라는 운영 부담이 는다.

**4) 언제 재검토하나 (재검토 트리거를 명시).**
- `candle_bars`가 **수천만~억 행**으로 자라고(현재 557만), 백테스트처럼 **여러 해에 걸친
  GROUP BY**를 사용자 요청 경로에서 돌리게 될 때.
- 또는 §8.2 (1)에서 `ex_per_sent > 1000`이고 `avg_ms > 1000`인 쿼리가 상위에 고정될 때.

**결론: 지금 켜는 것은 「무료니까 쓴다」이지 「필요해서 쓴다」가 아니다.** 기각.

---

### 8.7 Database ↔ Application 네트워크

**전제 변화**: 로컬 TCP 루프백(`host.docker.internal`) → **별도 클라우드 리소스**.

| 항목 | 조치 | 근거 |
|---|---|---|
| **private endpoint** | MDS는 사설 서브넷에만 엔드포인트를 만든다. VM과 **같은 AD·같은 VCN**. NSG로 VM 서브넷에서만 3306/33060 허용 | 공개 엔드포인트는 존재하지 않는 것이 기본이자 옳다 |
| **network latency** | 같은 AD 사설망 0.2~0.5 ms[추정]. **이관 후 실측할 것**: `mysqladmin ping` 100회 평균, 또는 `SELECT 1` 왕복 | 로컬 대비 왕복 비용이 생긴다 → **N+1 쿼리의 비용이 커진다.** 로컬에서 안 보이던 문제가 여기서 드러난다 |
| **TLS** | `tls=preferred` → CA 등록 후 `tls=custom` | `require_secure_transport` 확인 필요(§8.1) |
| **connection reuse** | ✅ **이미 풀을 쓴다.** 요청마다 새로 여는 구조가 **아니다** | `sql.DB`는 풀이고, `SetMaxOpenConns/MaxIdleConns/ConnMaxLifetime`이 전부 설정돼 있다 |
| **DNS lookup** | MDS 엔드포인트는 IP로 고정하지 말 것. Go 드라이버는 접속 시마다 해석하지만 OS 리졸버 캐시가 받는다 | |
| **TCP handshake** | §4.8의 1.33 conn/s를 이관 후 재측정 | |
| **keep-alive** | go-sql-driver는 기본 TCP keepalive 사용. MDS `wait_timeout`(기본 28800s)보다 `connMaxLifetime`(30분)이 훨씬 짧아 **서버가 먼저 끊는 사고는 없다** | ✅ 이미 안전 |
| **retry / timeout** | DSN에 `timeout/readTimeout/writeTimeout` 주입됨. **재시도는 없다** | DB 순단 시 요청이 그냥 실패한다. `/health`가 잡아 주지만 **커넥션 획득 실패 1회 재시도**를 추가할 가치는 있다 → P2 |

**"매 request마다 커넥션을 새로 생성하는 구조인가?" — 아니다.** 이 코드베이스는 풀을 정확히 쓴다.
만약 그런 구조였다면 원격 DB에서 요청당 **+2~6 ms**(TCP+TLS+인증)가 무조건 붙고,
p95가 두 배가 됐을 것이다. 지금은 그 위험이 없다.

---

## 9. OS / Linux 최적화 (Oracle Linux 9 / ARM64)

**원칙: 근거 없이 sysctl을 바꾸지 않는다.** 아래는 **변경이 필요하다고 판단한 것만**이다.
나머지(TCP 버퍼, ephemeral port range, conntrack, CPU governor, swappiness)는
**현재 부하에서 기본값이 충분하므로 건드리지 않는다.**

### 9.1 변경 제안

#### (1) Docker 로그 + 빌더 GC — **P0**

| | |
|---|---|
| 현재값 | json-file 무제한, 빌더 캐시 무제한 (실측 31.6 GB) |
| 권장값 | 아래 daemon.json |
| 이유 | 부트 볼륨 50 GiB. 디스크 고갈은 **복구가 가장 어려운 장애**다 |
| 부작용 | 30 MB보다 오래된 로그 소실 → §10의 에러 아카이브 잡으로 보완 |
| rollback | 파일 되돌리고 `systemctl restart docker` (컨테이너 전체 재기동 동반 → 배포 창에서) |

```json
{
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "3" },
  "builder": { "gc": { "enabled": true, "defaultKeepStorage": "5GB" } },
  "live-restore": true
}
```
`live-restore`: 데몬 재시작 시 컨테이너를 살려 둔다. 위 rollback의 부작용을 줄인다.
**적용 후 `docker compose up -d --force-recreate`가 있어야 기존 컨테이너에 로그 설정이 붙는다.**

#### (2) journald 디스크 상한 — **P0**

| | |
|---|---|
| 현재값 | OL9 기본 `SystemMaxUse` = 파일시스템의 10% (50 GiB → 최대 5 GiB) |
| 권장값 | `/etc/systemd/journald.conf.d/limits.conf` → `SystemMaxUse=500M`, `MaxRetentionSec=14day` |
| 이유 | 5 GiB는 이 디스크에서 과하다. Docker 로그가 이미 따로 상한을 갖는다 |
| 부작용 | 14일 이전 시스템 로그 소실 |
| rollback | 파일 삭제 후 `systemctl restart systemd-journald` |

#### (3) 시각 동기화 — **P0 (검증만)**

| | |
|---|---|
| 현재값 | OL9는 chronyd가 OCI NTP(169.254.169.254)를 기본 사용 |
| 권장값 | 변경 없음. **`chronyc tracking`으로 오프셋이 ms 단위인지 확인만** |
| 이유 | 이 앱은 시각이 곧 로직이다 — 시장 세션 판정, 배치 스케줄, 캔들 버킷 경계. KST 자정 경계 버그 전례가 있다 |
| 부작용 | — |
| rollback | — |

또한 **시스템 타임존을 Asia/Seoul로 설정한다**(`timedatectl set-timezone Asia/Seoul`).
컨테이너는 UTC로 돌지만(그것이 맞다), 호스트 로그·cron·journald 타임스탬프를
운영자가 읽는 시간대와 맞추면 사고 조사 때 실수가 준다.

#### (4) 부트 볼륨 여유 감시 — **P0**

| | |
|---|---|
| 현재값 | 없음 |
| 권장값 | `df` 기반 알람 80% (§10) + 주간 `docker image prune -f` |
| 이유 | 위 (1)이 상한을 걸어도 media 볼륨·OCR 임시파일은 자란다 |
| 부작용 | — |
| rollback | — |

#### (5) swap — **넣지 않는다**

| | |
|---|---|
| 현재값 | OL9 이미지 기본(보통 스왑 없음) |
| 권장값 | **변경 없음** |
| 이유 | 12 GiB 중 실측 238 MiB 사용. 스왑은 **부트 볼륨의 IOPS를 DB 아닌 곳에 쓰게 만들고**, 메모리 누수를 감춰 장애를 느리게 만든다. 컨테이너 메모리 상한이 이미 OOM 경계 역할을 한다 |
| 부작용 | 예상 못 한 메모리 폭증 시 쿠션이 없다 → 그래서 `GOMEMLIMIT`과 컨테이너 상한이 필요하다 |
| rollback | 필요해지면 2 GiB 스왑파일 + `vm.swappiness=10`. **그때 swappiness도 함께** |

#### (6) 파일 디스크립터 / ulimit — **변경 없음**

실측 api 33 fd / 상한 1,048,576. **바꿀 이유가 없다.**
SSE 스트림이 수천 개로 늘면 다시 본다(§10에 `sse_streams` 지표가 있다).

#### (7) 방화벽 — **P0**

| | |
|---|---|
| 현재값 | 미설정 |
| 권장값 | OCI NSG: ingress 22(운영자 IP만) / 80 / 443. 그 외 전부 거부. **그리고 compose에서 loopback 바인딩**(§6.13) |
| 이유 | 두 겹으로 막는다. Docker의 DNAT가 firewalld를 우회할 수 있어 **호스트 방화벽만 믿지 않는다** |
| 부작용 | 운영자 IP가 바뀌면 SSH가 막힌다 → OCI 콘솔의 Cloud Shell/시리얼 콘솔이 대피로 |
| rollback | NSG 규칙 되돌리기(즉시 반영) |

#### (8) TLS 종단 — **P0**

nginx + certbot(`--webroot`)을 권장한다.
**대안인 OCI Flexible Load Balancer(Always Free 10 Mbps)와 비교:**

| | nginx + certbot | OCI LB(무료 10 Mbps) |
|---|---|---|
| 대역폭 | VM 2 Gbps 그대로 | **10 Mbps 상한** = 1.25 MB/s |
| 인증서 | 자동 갱신 cron 필요 | 관리형 |
| SSE | 그대로 | 유휴 타임아웃 설정 필요 |
| 비용 | 0 | 0 |

뉴스 목록 같은 응답이 압축 후 8~15 KB라 10 Mbps로도 당장은 버티지만,
**이미지(`/media/`)와 SSE 다중 접속에서 먼저 막힌다.** → nginx + certbot 채택.

#### (9) unattended updates — **부분 적용**

`dnf-automatic`을 **security only + download only**로 둔다.
자동 재부팅/자동 설치는 하지 않는다 — 재부팅이 곧 시세 적재 중단이고, 이 서비스는
정해진 시각에 도는 배치가 많다. 패치 적용은 **수동 배포 창에서**.

### 9.2 건드리지 않기로 한 것과 그 이유

| 항목 | 왜 안 바꾸나 |
|---|---|
| CPU governor | Ampere A1은 하이퍼바이저가 관리. 게스트에서 바꿀 것이 없다 |
| `net.core.somaxconn`, TCP 버퍼 | 동시 요청 실측 1건. 기본값이 수천 연결까지 감당한다 |
| ephemeral port range | 아웃바운드 커넥션 수십 개 수준 |
| `nf_conntrack_max` | 동일 |
| `vm.swappiness` | 스왑이 없으므로 무의미 |
| filesystem / mount 옵션 | XFS 기본. `noatime`은 요즘 커널 기본(relatime)으로 충분 |
| logrotate | 로그가 전부 journald/docker로 가므로 별도 로테이션 대상이 없다 |

---
## 10. Observability — 최적화보다 먼저

**원칙: Prometheus + Grafana를 세우지 않는다.** 컨테이너 2개와 시계열 스토리지가 2 OCPU / 50 GiB에서
차지하는 비용이, 이 규모에서 얻는 이득보다 크다. 대신 **이미 있는 것 세 가지를 엮는다** —
① 앱의 `/metrics`(이미 존재, Prometheus 텍스트 포맷) ② OCI Monitoring 기본 지표(무료)
③ MySQL Performance Hub(무료).

### 10.1 수집 계층

```
┌── OCI Monitoring (에이전트/서비스 기본 제공, 알람 무료) ──────────────┐
│ VM:  CpuUtilization · MemoryUtilization · DiskBytesRead/Written ·     │
│      DiskIopsRead/Written · NetworksBytesIn/Out                       │
│      ※ Memory/Disk는 Oracle Cloud Agent의 "Compute Instance Monitoring"│
│        플러그인이 켜져 있어야 나온다 — 이관 후 콘솔에서 확인          │
│ MDS: CPUUtilization · MemoryUtilization · ActiveConnections ·         │
│      StatementLatency · DbVolumeWriteBytes / ReadBytes · InnoDB 지표  │
└───────────────────────────────────────────────────────────────────────┘
┌── 앱 /metrics (127.0.0.1:8080, 외부 비공개) ─────────────────────────┐
│ 이미 있음: http_requests_total{class} · in_flight · goroutines ·      │
│   heap_bytes · mysql_{open,in_use,idle,max_open,wait_total} ·         │
│   redis_{total,idle}_conns · poller_{priced,terminal,fast}_symbols ·  │
│   sse_streams · stream_subscriptions · toss_rest_{calls,path}_total   │
│ 추가 권장: HTTP latency 히스토그램(p50/p95/p99), 경로별 5xx,          │
│   ocr_active_jobs, batch_job_duration_seconds{job}                    │
└───────────────────────────────────────────────────────────────────────┘
┌── 수집 잡 (VM cron, 5분) ────────────────────────────────────────────┐
│ curl -s localhost:8080/metrics >> /var/log/qp/metrics-$(date +%F).log │
│ docker stats --no-stream >> /var/log/qp/stats-$(date +%F).log         │
│ + logrotate 14일. "지난 화요일 밤에 무슨 일이 있었나"를 답할 수 있게. │
└───────────────────────────────────────────────────────────────────────┘
```

**MySQL 쪽에서 반드시 켤 것:**
```sql
-- MDS Configuration에서 (재기동 불필요한 동적 변수)
slow_query_log = ON
long_query_time = 1        -- 10초는 너무 관대하다: 실측 Slow_queries 151건은 "10초 넘은 것"뿐
log_output = TABLE         -- mysql.slow_log. 파일 접근 권한이 없는 관리형에서 유일한 경로
```
`long_query_time`을 0.1이 아니라 **1초**로 잡는 이유: `log_output=TABLE`은 슬로우 로그를 **DB에 쓴다**.
0.1초로 두면 로그 기록 자체가 29 MiB/s와 1 ECPU를 갉아먹는다. 1초로 시작해 잡히는 것이 없으면 낮춘다.

### 10.2 반드시 볼 지표와 알람

| 계층 | 지표 | 알람 임계 | 왜 이 값인가 |
|---|---|---|---|
| **VM** | 부트 볼륨 사용률 | **> 80%** 경고 / **> 90%** 긴급 | 실측 빌드 캐시 31.6 GB. 90%면 남은 5 GiB로 복구 작업이 가능 |
| | CPU 5분 평균 | > 80% 10분 지속 | 2 OCPU에서 80%면 컨텍스트 스위치가 지연을 만든다 |
| | 메모리 사용률 | > 75% | 실측 2% → 75%면 무언가 새고 있다는 뜻 |
| | 컨테이너 재기동 | **5분 내 3회** | 크래시 루프. `restart: unless-stopped`가 무한 재시도하므로 사람이 알아야 한다 |
| | api OOMKilled | **1회라도** | graceful shutdown 없음 = Toss 토큰 소실 + 적재 중단 |
| **App** | `http_requests_total{class="5xx"}` 증가율 | > 1% of total | |
| | p95 latency | **> 500 ms 5분 지속** | 현재 실측 `/stocks/search` 60~67 ms. 500 ms면 8배 열화 |
| | `mysql_wait_total` | **0을 벗어나면** | 지금 0. 풀 포화의 **가장 이른 신호**이고 오탐이 없다 |
| | `sse_streams` | 급증(> 200) | 연결 누수 |
| | `poller_terminal_symbols` | 상한(40) 지속 | 폴러가 못 따라잡고 있다 |
| | `toss_rest_calls_total` 분당 증가 | 급증 | 한도 소진 전조. 이미 실측 기준선이 있다 |
| | goroutines | > 1,000 | 현재 85. 누수 감지 |
| **MySQL** | CPUUtilization | **> 70% 10분** | 1 ECPU. 70%면 큐잉이 시작된다 |
| | ActiveConnections | > 45 (배정 56의 80%) | 재배포 겹침·CLI 폭주 |
| | StatementLatency | 기준선의 3배 | 이관 첫 주에 기준선을 잡는다 |
| | `Innodb_row_lock_waits` 증가율 | 기준선(7,200/일)의 2배 | 이미 실측 기준선이 있다 |
| | `Slow_queries` 증가 | 시간당 > 5 | `long_query_time=1` 기준 |
| | `Created_tmp_disk_tables` 비율 | > 5% | 현재 0.4% |
| | `Sort_merge_passes` 증가율 | 기준선(0.31/s)의 2배 | |
| | DbVolumeWriteBytes | > 15 MiB/s 5분 | 29 MiB/s의 절반. 배치가 폭주하는 신호 |
| | 백업 성공 여부 | 실패 시 | 보존 1일이라 한 번 실패가 곧 백업 0일 |

**알람 채널**: OCI Notifications → 이메일(무료). 이 규모에서 PagerDuty류는 과하다.

### 10.3 이관 전에 기준선을 뜬다

측정 없는 최적화를 막는 가장 싼 방법은 **이관 직전·직후에 같은 값을 재는 것**이다.
아래를 이관 전(로컬)·이관 후(OCI) 두 번 기록한다.

```bash
# 1) 앱
curl -s localhost:8080/metrics > baseline-app-$(date +%F).txt
# 2) DB (24시간 간격 두 번 → 델타가 곧 일일 처리량)
mysql -e "SHOW GLOBAL STATUS" > baseline-db-$(date +%F-%H%M).txt
# 3) 컨테이너
docker stats --no-stream > baseline-stats-$(date +%F).txt
# 4) 응답 시간 (외부 API를 안 쓰는 경로로)
for p in /health /api/v1/stocks/search?q=삼성 /api/v1/stocks/scores/top; do
  curl -so /dev/null -w "$p %{time_total}\n" "http://localhost:8080$p"; done
```

---

## 11. Benchmark Plan

### 11.0 대전제 — **부하 테스트가 외부 API 한도를 태우게 두지 말 것**

이 앱의 여러 경로가 Toss·DART·Finnhub·LLM을 부른다. Toss `MARKET_DATA`는 15 req/s이고,
**계정 단위**다. 부하 테스트를 그냥 돌리면 **실서비스의 시세가 죽는다.**

- 테스트 대상은 **DB/Redis만 읽는 경로**로 한정한다(조회 경로에서 외부 HTTP 금지가 이 프로젝트의
  설계 원칙이므로, 대부분의 GET이 여기 해당한다 — `BATCH_SCHEDULE.md` 원칙 5).
- 폴러를 끄거나(환경변수) **별도 DB 사본을 가진 스테이징 스택**에서 돈다.
- 도구: `k6`(arm64 바이너리 있음) 또는 `vegeta`. VM **바깥**에서 부하를 넣는다
  — 같은 박스에서 넣으면 부하 생성기가 2 OCPU를 나눠 먹어 결과가 무의미해진다.

### 11.1 공통 측정 항목

매 테스트마다 아래를 **동시에** 기록한다(하나만 봐서는 원인을 못 가른다):

| 축 | 항목 | 출처 |
|---|---|---|
| 부하 | 동시 사용자(VU), 목표 RPS, duration | k6 |
| 응답 | 실제 RPS, p50 / p95 / p99, error rate, 타임아웃 수 | k6 |
| VM | CPU%, load avg, RSS, 컨테이너별 CPU/Mem | `docker stats`, `vmstat 1` |
| App | `in_flight`, `goroutines`, `heap_bytes`, `mysql_in_use`, **`mysql_wait_total`** | `/metrics` |
| DB | CPU%, `Threads_running`, `Threads_connected`, `Innodb_row_lock_waits`, `Slow_queries`, `Innodb_data_written` | OCI Monitoring + `SHOW GLOBAL STATUS` 델타 |

### 11.2 시나리오

| # | 이름 | 구성 | 무엇을 답하려는 테스트인가 | 합격선(초기 목표) |
|---|---|---|---|---|
| **T1** | 일반 API 트래픽 | 대표 GET 5종 혼합. VU 1→100 램프, 각 단계 3분 | 정상 상태의 처리량과 지연 | p95 < 300 ms, error 0% |
| **T2** | API + Batch 동시 | T1의 절반 부하 + 관리자에서 `stock_scores` 수동 실행 | **배치가 API를 얼마나 밀어내는가** — 이 문서 전체의 핵심 질문 | T1 대비 p95 상승 **< 50%** |
| **T3** | DB query-heavy | `/candles?interval=1m&limit=...`, `/stocks/{symbol}/daily-closes` 위주 | 1 ECPU의 실제 천장 | `Threads_running` < 4 유지 |
| **T4** | 대량 batch 단독 | `warnings_sync` 또는 `candle_rollup`을 트래픽 없이 | 배치 단독의 DB 소비량. **29 MiB/s에 얼마나 붙나** | DbVolumeWrite < 15 MiB/s |
| **T5** | DB connection 증가 | VU를 200까지, 응답이 느린 경로 위주 | **풀 포화 지점** | `mysql_wait_total`이 0을 벗어나는 VU 수를 기록 |
| **T6** | CPU saturation | T1 + OCR 잡(스크린샷 10장) 동시 | §4.5가 실제로 얼마나 나쁜가 / 세마포어 수정 전후 비교 | 수정 후 p95 상승 < 2배 |
| **T7** | Memory pressure | T6를 30분 지속 + heap 추적 | 누수와 OOM 경계 | `heap_bytes` 안정, OOMKilled 0 |

**T2와 T6이 이 벤치마크의 존재 이유다.** 나머지는 맥락을 주는 값이다.

### 11.3 Saturation point를 찾는 방법

한 지표만 보면 못 찾는다. **세 곡선의 교차점**을 본다.

```
  처리량                    지연                     오류
    │      ┌─── 평탄          │        ╱ 급상승      │        ╱
    │    ╱                    │      ╱                │       │
    │  ╱                      │ ────                  │ ──────
    └──────── VU              └──────── VU            └──────── VU
              ▲ knee                    ▲ 같은 지점            ▲ 조금 뒤
```

절차:
1. VU를 1 → 200으로 **단계별 램프**(각 단계 3분, 워밍업 30초 제외).
2. 단계마다 (실제 RPS, p95)를 기록.
3. **saturation point = 실제 RPS가 더 이상 늘지 않는데 p95는 계속 오르는 첫 단계.**
4. 그 지점에서 **무엇이 먼저 포화했는지**를 다음 순서로 가른다:
   - `mysql_wait_total` > 0 → **커넥션 풀**이 먼저 (풀을 늘려야 하나? 아니다 — §8.1, DB CPU를 먼저 본다)
   - DB `CPUUtilization` > 80%, `Threads_running` > 4 → **DB 1 ECPU**가 천장. 앱 튜닝으로 못 넘는다
   - VM CPU > 90% → **api 컨테이너**. gzip 레벨, JSON 직렬화, 폴러 부담을 본다
   - 셋 다 여유인데 p95만 오른다 → **락 대기**(§8.4)나 외부 API
5. **운영 목표는 saturation point의 50%로 잡는다.** 여유 2배가 있어야 배치·스파이크를 견딘다.

---

## 12. Priority Table

### P0 — 배포 전에 반드시

**상태**: ✅ = 리포에 적용됨(코드/compose/스크립트) · ☁ = 클라우드 리소스가 있어야 하는 작업 ·
📄 = 파일은 준비됐고 VM 에서 값만 채우면 됨.

| # | 항목 | 상태 | 근거 | 검증 |
|---|---|---|---|---|
| P0-1 | **DSN 에 `time_zone='+09:00'` 고정** | ✅ `repository.withDefaultDSNTimeouts` (`MYSQL_TIME_ZONE` 로 override) | §4.1. TIMESTAMP 85컬럼 9시간 시프트 | `TestDSNPinsSessionTimeZone` 외 2건 · 이관 후 `SELECT @@session.time_zone, NOW()` |
| P0-2 | **로그 상한 + 빌더 GC** | ✅ compose `x-logging`(10m×3, 전 서비스) · 📄 daemon.json 은 `deploy/README.md` | §0.5/0.6. 실측 캐시 31.6 GB / 부트 50 GiB | `docker inspect` LogPath 크기 · `app-stack.sh doctor` |
| P0-3 | **published 포트 loopback + Redis `requirepass`** | ✅ base 가 `127.0.0.1:8080`/`127.0.0.1:6379` · 오버레이가 `requirepass` | §4.4. 레이트리밋 우회 + 인증 없는 Redis 노출 | 렌더 확인 완료 · VM 에서 `nc -vz` refused |
| P0-4 | **TLS 종단 + `AUTH_COOKIE_SECURE=true`** | 📄 `nginx.tls.conf` + `tls_site.conf.example` (`nginx -t` 통과) · ☁ 인증서 발급은 VM 에서 | §9.1(8) | SSL Labs · 로그인 쿠키 `Secure` |
| P0-5 | **OCR 동시성을 코어 수로 제한** (장수 상한 10 은 이미 있었음) | ✅ `internal/import/ocr/limit.go` + `import_job.go` 팬아웃 + `OMP_THREAD_LIMIT=1` | §4.5. 사용자 1명이 2 OCPU 점유 가능 | `TestAcquireBoundsConcurrency` 외 3건 · T6 벤치마크 |
| P0-6 | **`GOMEMLIMIT=800MiB` + api `cpus: 1.5`** | ✅ base 에 GOMEMLIMIT · 오버레이가 `cpus: 1.5` | §6.4. OOM 킬 = graceful shutdown 없음 = 토큰 소실 | T7 |
| P0-7 | **`cpu_shares`** (api 2048 / 나머지 1024 / 워커 256) | ✅ base | §6.5. 상한만 있고 우선순위가 없다 | T2 |
| P0-8 | **MDS TLS 요구 여부 확인 + `tls=` 파라미터** | 📄 `.env.*.example` 에 `tls=preferred` | §8.1. 아니면 **접속 자체가 실패**한다 | `\s` 로 SSL cipher 확인 |
| P0-9 | **NSG: 22(내 IP)/80/443만 허용** | ☁ | §9.1(7) | 외부 포트 스캔 |
| P0-10 | **부트 볼륨 80% 알람** | ✅ `app-stack.sh` 가 기동 전 검사(80% 경고 / 90% 중단) · ☁ OCI 알람은 별도 | §10.2 | `doctor` |
| P0-11 | **ARM64 빌드·OCR 실동작 검증** | ☁ 절차는 §4.9 | §4.9 | `buildx` + OCR 회귀 테스트 |
| P0-12 | **이관 전 논리 덤프 + 복원 리허설** | ☁ 절차는 §8.5 | §8.5. 보존 1일 | 빈 인스턴스에 복원해 행 수 대조 |
| P0-13 | **역할별 배포 스크립트** (API VM / 배치 VM) | ✅ `deploy/scripts/{app,batch}-stack.sh` | §5. 두 VM 분리 운영 | `config` 렌더 3종 통과 |

### P1 — 운영 초기 (첫 2주)

| # | 항목 | 근거 |
|---|---|---|
| P1-1 | **`qp_observer` 계정 생성 + performance_schema 권한** | §8.2. **지금은 측정 자체가 막혀 있다** — 다른 모든 P1의 전제 |
| P1-2 | `slow_query_log ON`, `long_query_time=1`, `log_output=TABLE` | §10.1 |
| P1-3 | **락 대기 원인 규명** (3일 22,387건, 평균 122 ms) | §8.4 |
| P1-4 | **`Sort_merge_passes` 상위 쿼리 규명** (3일 83,861) | §8.2 |
| P1-5 | **주간 논리 덤프 → Object Storage** | §8.5. 보존 1일 보완 |
| P1-6 | **VM에서 빌드하지 않기** (CI → OCIR → pull) | §4.2 |
| P1-7 | **워커 healthcheck**(파일 하트비트) | §6.7 |
| P1-8 | **`stop_grace_period`** api 30s / 워커 60s | §6.8 |
| P1-9 | **LLM·뉴스 브리핑 배치를 워커로 분리** | §7.2 |
| P1-10 | **배치 트랜잭션 청크화 + keyset pagination 점검** | §7.4 |
| P1-11 | **신규 접속 1.33/s 원인 규명** → 필요 시 `connMaxIdleTime` 조정 | §4.8 |
| P1-12 | **HTTP latency 히스토그램**(p50/p95/p99)을 `/metrics`에 추가 | §10.1. 지금은 카운터만 있어 **p95를 알 수 없다** |
| P1-13 | **`EnsureSchema` 타임아웃 환경변수화** + 대형 ALTER 배포 분리 | §4.7 |
| P1-14 | **`warnings_sync`(01:30, 2시간)가 미국 장중과 겹치는 영향 측정** | §7.5 |
| P1-15 | **T1~T7 벤치마크 실행 + saturation point 기록** | §11 |

### P2 — 트래픽 증가 후 (지금 하면 과최적화)

| # | 항목 | 지금 하지 않는 이유 |
|---|---|---|
| P2-1 | 인덱스 추가/제거 (`news_articles` 6개, `stock_daily_closes.trade_date`) | **사용 근거 없이 손대지 않는다.** P1-1이 끝나야 판단 가능 |
| P2-2 | `stock_daily_closes`의 `ai_summary`를 사이드 테이블로 분리 | 679 MB지만 히트율 99.99%. 이득이 측정되지 않았다 |
| P2-3 | `batchQueuePollInterval` 4s → 10s | 4.25 QPS는 지금 아프지 않다. UX를 먼저 깎을 이유 없음 |
| P2-4 | 이미지 크기 축소 | 381/446 MB는 50 GiB에서 문제가 아니다 |
| P2-5 | Redis를 순수 캐시로(`appendonly no`) | IOPS가 병목이 아니다 |
| P2-6 | 시세 폴러를 api에서 분리 | 리더 선출·상태 공유 코드가 새로 필요. 2번째 VM이 생길 때 함께 |
| P2-7 | DB 커넥션 획득 실패 시 1회 재시도 | 순단 사례가 아직 없다 |
| P2-8 | HeatWave | §8.6. **재검토 트리거를 만족할 때만** |
| P2-9 | Kubernetes / 서비스 메시 | §6.1 |

---

## 13. Expected Performance

**아래는 전부 [추정]이다.** 실제 워크로드 프로파일(RPS, 동시 사용자, 엔드포인트 분포)이 없기 때문이고,
§11의 벤치마크가 이 표를 **실측으로 교체하기 위해** 존재한다. 추정의 근거는 명시했다.

### 13.1 지연

| 경로 | 예상 p50 | 예상 p95 | 근거 |
|---|---|---|---|
| `/health` | < 2 ms | < 5 ms | DB 미접근 |
| Redis 캐시 히트 경로 | 3~8 ms | 15~30 ms | Redis 키 27,935개 in-memory + 원격 DB 미접근 |
| 단순 DB 조회(PK) | 5~15 ms | 30~60 ms | **실측** `/stocks/search` 60~67 ms(로컬). 원격 DB RTT +0.5 ms[추정] |
| `/candles` (차트 1페이지) | 15~40 ms | 80~150 ms | PK 범위 스캔 + JSON 직렬화 + gzip |
| `/stocks/{symbol}/summary` | 30~80 ms | 150~300 ms | 여러 표를 합친다. single-flight 적용됨(20동시 요청 DB 질의 291→69) |
| SSE 첫 프레임 | < 100 ms | < 300 ms | |

### 13.2 처리량

| 조건 | 예상 지속 RPS | 제한 요인 |
|---|---|---|
| 캐시 히트 위주 GET | **300~600** | api 컨테이너 CPU(gzip 5 + JSON). VM 2 OCPU |
| DB 조회 위주 GET | **80~200** | **DB 1 ECPU.** 폴러·배치가 이미 평균 85 QPS를 쓰고 있다(실측) |
| 배치 동시 실행 중 | 위의 **50~70%** | CPU·DB 경합. `cpu_shares` 적용 후 기준 |
| OCR 진행 중 (수정 전) | **급락, 예측 불가** | §4.5. 세마포어 적용 후 위의 70~80% 예상 |

**동시 사용자로 환산하면**[추정]: 화면이 2초 폴링 3종 + SSE를 쓰므로 사용자 1명당 대략 1.5~2 RPS.
DB 조회 위주 150 RPS를 안전 계수 50%로 잡으면 **동시 접속 40~50명**이 현재 사양의 현실적 목표다.

### 13.3 이 추정이 틀릴 수 있는 지점

- **DB 1 ECPU의 실제 성능**을 모른다. ECPU는 vCPU와 1:1이 아니다[확인 필요].
- gzip 레벨 5의 CPU 비용을 ARM Ampere에서 측정한 적이 없다. 레벨 3~4로 낮추면 CPU가 줄고 대역폭이
  는다 — **2 Gbps에 여유가 크므로 트레이드가 유리할 수 있다.** T1에서 함께 측정할 것.
- 폴러가 쓰는 85 QPS는 **관심 종목 수에 비례**한다(현재 `priced_symbols` 224). 사용자가 늘면
  이 값도 늘어 **사용자 트래픽과 폴러 부하가 함께 증가**한다 — 선형이 아니다.

---

## 14. Scaling Strategy

**확장 순서의 원칙: 돈이 안 드는 것 → 구조가 안 바뀌는 것 → 돈이 드는 것 → 구조가 바뀌는 것.**

```
1. 앱 레벨 (무료 · 구조 불변)
   캐시 TTL 조정 · 쿼리/인덱스 개선 · 폴러 주기 · 응답 크기
   ▸ 트리거: p95 열화가 특정 엔드포인트에 몰릴 때
   ▸ 근거: 이 프로젝트는 이미 이 방식으로 238ms→0.289ms, 291→69 질의를 만들었다

2. 배치·폴러를 두 번째 Always Free A1로 분리 (무료 · 구조 소변경)
   Always Free 한도 4 OCPU/24 GiB 중 현재 절반만 사용 → 2 OCPU/12 GiB가 더 있다
   ▸ 트리거: T2에서 "배치 중 p95 상승 > 50%"가 나올 때
   ▸ 옮길 것: 워커 8개(이미 DB 큐 + SKIP LOCKED라 다른 호스트에서도 안전)
   ▸ 옮기지 말 것: 시세 폴러·WS (api와 메모리 상태를 공유한다)
   ▸ 대가: Redis 접근이 VCN 내부 홉을 타고, 관리 대상 호스트가 2개가 된다

3. DB scale-up: 1 ECPU → 2 (유료 · 구조 불변) ★ 첫 유료 단계로 이것을 권한다
   ▸ 트리거: DB CPUUtilization > 70%가 상시
   ▸ 이유: 측정상 DB가 가장 먼저 천장에 닿는다. 앱은 여유가 크다(VM CPU 0.7%, 메모리 2%)
   ▸ 효과가 가장 직접적이고 되돌리기 쉽다(다운타임 있는 리사이즈)

4. VM scale-up: 2 → 4 OCPU (유료)
   ▸ 트리거: VM CPU > 80% 상시 (OCR·gzip·SSR)
   ▸ 3번보다 뒤인 이유: 지금 VM은 자원의 2%를 쓴다

5. Read replica (유료 · 구조 변경)
   ▸ 트리거: 읽기가 DB CPU의 70% 이상이고 3번으로도 부족할 때
   ▸ 대가: 앱이 읽기/쓰기 커넥션을 나눠야 한다(복제 지연 처리 포함).
     조회 대부분이 배치가 만든 표를 읽으므로 **지연에 관대**하다 — 이 앱에는 잘 맞는다

6. 캐시 계층 강화 (구조 변경)
   ▸ 트리거: 같은 응답이 반복 계산될 때. 지금은 이미 Redis + single-flight가 있다
   ▸ **새 캐시 계층을 더 얹지 않는다** — 무효화 복잡도가 늘고, 실측 히트율이 이미 높다

7. API scale-out + LB (구조 변경)
   ▸ 트리거: 1~6을 다 하고도 부족할 때
   ▸ 선결 조건: api에서 배경 폴러·WS·마이그레이터를 **먼저** 분리해야 한다(§4.6).
     이 프로젝트는 다중 인스턴스 전제 작업을 한 번 기각한 바 있다 — 그 판단은 여전히 유효하고,
     여기까지 왔을 때 다시 꺼낼 항목이다

8. 큐 도입 (지금 필요 없음)
   ▸ DB 기반 큐(`batch_job_runs` + SKIP LOCKED)가 이 규모에서 정확히 맞다.
     Kafka/RabbitMQ는 초당 수천 건이 될 때의 도구다
```

---

## 15. 정확도를 높이기 위해 더 필요한 자료

리포 안에서 확인할 수 있는 것(Dockerfile, docker-compose.yml, 런타임 버전, DB 스키마, 인덱스,
배치 목록·주기, 커넥션 풀 설정, MySQL 지표, 컨테이너 자원 사용량)은 **이미 실측했다.**
아래는 리포 안에 없어서 이 문서가 [추정]으로 남겨 둔 것들이다.

### 우선순위 높음 — 없으면 §13의 숫자를 확정할 수 없다

1. **예상 RPS / 동시 사용자 / 피크 시간대** — 현재 실사용자 수(`users` 4,295행은 가입자지 동시 접속이 아니다)
2. **주요 API의 실제 호출 분포** — nginx access log 1주일치면 충분하다
   (`awk '{print $2}' access.log | sort | uniq -c | sort -rn | head -30`)
3. **EXPLAIN 결과** — P1-1(observer 계정) 이후 §8.2의 쿼리 4종 출력
4. **부하 시 CPU/RAM** — 지금은 유휴 값만 있다. 사용자가 붙은 시간대의 `docker stats`
5. **배치별 처리 row 수와 실제 소요 시간** — `batch_job_runs` 테이블의 `result` JSON을 집계하면 나온다
   (`SELECT job_id, AVG(TIMESTAMPDIFF(SECOND, started_at, finished_at)) FROM batch_job_runs GROUP BY job_id`)
6. **로그량** — 하루 요청 수 × 로그 라인 크기. daemon.json 상한을 정할 때 필요

### 클라우드 콘솔에서 확인해야 하는 것 — [확인 필요]로 남긴 것들

7. **MDS `@@global.time_zone`, `@@max_connections`, `@@require_secure_transport`, `@@innodb_buffer_pool_size`**
8. **MDS가 `Asia/Seoul` 명명 타임존을 지원하는지** (tz 테이블 적재 여부)
9. **Always Free 컴퓨트 회수 정책** 적용 여부(계정 유형)
10. **부트 볼륨 성능 등급(VPU)** — Always Free의 IOPS/throughput 실측치
11. **Oracle Cloud Agent의 Compute Instance Monitoring 플러그인** 활성 여부(메모리·디스크 지표의 전제)
12. **OCI Load Balancer 무료 한도**를 쓸지 여부(§9.1(8)의 선택)

### 있으면 좋은 것

13. 목표 SLO — "p95 몇 ms 이하", "월 다운타임 몇 분 이하"를 정하면 §12의 우선순위가 더 날카로워진다
14. 데이터 보존 정책 — 캔들 60일은 정해져 있으나 `news_articles`·`stock_scores`·`trade_ticks`는 무기한
15. 배포 빈도 — 하루 여러 번이면 P1-6(CI 빌드)이 P0로 올라간다

---

## 부록 A. 이관 당일 체크리스트

```
[준비]
 □ 논리 덤프 + 복원 리허설 완료 (P0-12)
 □ ARM64 이미지 빌드·OCR 검증 완료 (P0-11)
 □ MDS 프로비저닝: 사설 서브넷, time_zone=+09:00, slow_query_log ON
 □ MDS 파라미터 확인: max_connections / require_secure_transport / buffer_pool

[VM 준비]
 □ timedatectl set-timezone Asia/Seoul
 □ chronyc tracking (오프셋 ms 단위 확인)
 □ /etc/docker/daemon.json 배치 후 systemctl restart docker
 □ journald SystemMaxUse=500M
 □ NSG: 22(내 IP)/80/443

[배포]
 □ .env: MYSQL_DSN(tz+tls 포함) · REDIS_PASSWORD · AUTH_COOKIE_SECURE=true
 □ docker compose -f docker-compose.yml -f docker-compose.oci.yml --profile prod up -d
 □ /health 200 · Tesseract OK (컨테이너 로그)
 □ 외부에서 nc -vz <IP> 6379 8080 → refused

[이관 직후 검증 — 이 순서대로]
 □ SELECT @@session.time_zone, NOW(), UTC_TIMESTAMP()          ← 최우선
 □ SELECT MAX(bar_time) FROM candle_bars WHERE bar_interval='1m'
 □ SELECT MAX(updated_at) FROM news_articles
 □ curl /metrics → mysql_wait_total=0, goroutines 정상 범위
 □ 첫 장중에 1분봉 지연 확인 (poller 지표)
 □ 배치 1개 수동 실행 → batch_job_runs에 done 기록
 □ 24시간 후 SHOW GLOBAL STATUS 델타를 로컬 기준선과 대조
 □ df -h → 부트 볼륨 사용률 기록 (알람 기준선)
```
