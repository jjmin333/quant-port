# 배포 — 역할별 스택

이 디렉터리는 **어느 VM 이 무엇을 띄우는가**를 담는다.
왜 이렇게 나누는지, 각 값의 근거는 [`docs/OCI_MIGRATION_PLAN.md`](../docs/OCI_MIGRATION_PLAN.md).

```
deploy/
├── scripts/
│   ├── app-stack.sh      API/FE VM  — redis · api · fe · nginx
│   ├── batch-stack.sh    배치 VM    — 워커 8개
│   └── _stack.sh         두 스크립트의 공통 뼈대
├── compose/
│   ├── docker-compose.app.oci.yml   루트 compose 위에 겹치는 클라우드 오버레이
│   └── docker-compose.batch.yml     배치 VM 전용 단독 파일
├── nginx/
│   ├── nginx.conf            평문(:80) — 로컬·최초 인증서 발급용
│   ├── nginx.tls.conf        TLS(:443) — 운영
│   ├── app_routes.conf       라우팅 본문 (위 둘이 같은 것을 include)
│   ├── proxy_headers.conf    공통 프록시 헤더
│   └── tls_site.conf.example 도메인·인증서 경로 (VM 에서 채운다)
├── .env.app.example
└── .env.batch.example
```

## 명령

두 스크립트의 명령 이름은 같다.

```bash
./deploy/scripts/app-stack.sh   deploy    # 빌드 + 기동 + 이미지 정리  ← 평소 배포
./deploy/scripts/batch-stack.sh deploy

./deploy/scripts/app-stack.sh   ps        # 상태 + 자원 사용량
./deploy/scripts/app-stack.sh   logs api  # 로그 따라가기
./deploy/scripts/app-stack.sh   doctor    # 재기동/OOM/디스크 점검
./deploy/scripts/app-stack.sh   prune     # 이미지·빌드 캐시 정리
./deploy/scripts/batch-stack.sh down      # 이 스택이 소유한 것만 정지
```

`down` 이 `compose down` 이 아니라 `stop` 인 이유: `down` 은 프로젝트 전체를 내려서,
같은 프로젝트에 다른 서비스가 있으면 함께 죽는다.

Windows 로컬에서는 Git Bash 로 그대로 돌아간다. 단, 로컬 개발의 평소 흐름은
기존 `be\scripts\restart.ps1 -Background` 그대로다 — 이 스크립트들은 VM 용이다.

## 한 대에서 다 돌릴 때 (로컬 · 단일 VM)

루트 `docker-compose.yml` 하나면 된다. 바뀐 것은 안전장치뿐이다:
로그 상한(10m×3), `cpu_shares`, `stop_grace_period`, `GOMEMLIMIT`,
그리고 **api·redis 를 loopback 에만 게시**.

```bash
APP_STACK_PLAIN=1 ./deploy/scripts/app-stack.sh up   # 오버레이 없이
```

## 두 대로 나눌 때

```
        인터넷 :443
            │
   ┌────────▼─────────────────┐        ┌──────────────────────────┐
   │ API/FE VM                │        │ 배치 VM                  │
   │  nginx · api · fe        │◀──────▶│  worker × 8              │
   │  redis  (사설 IP 6379)   │ Redis  │  (Redis 없음)            │
   └────────┬─────────────────┘ 공유   └────────┬─────────────────┘
            │                                    │
            └──────────► 관리형 MySQL ◄──────────┘
```

### 순서

1. **API VM 먼저.** 스키마 마이그레이터는 `api` 하나뿐이다.
   워커는 DDL 을 실행하지 않고 `repository.VerifySchema` 로 최대 3분 기다린다.
2. `cp deploy/.env.app.example deploy/.env.app` → 값 채우기 → `app-stack.sh deploy`
3. `cp deploy/.env.batch.example deploy/.env.batch` → 값 채우기 → `batch-stack.sh deploy`

### ⚠ Redis 는 반드시 공유한다

배치 VM 에 Redis 를 따로 띄우면 **안 된다.** `worker_bootstrap.go` 가 Redis 에서
세 가지를 API 프로세스와 공유한다.

| 공유 대상 | 나뉘면 |
|---|---|
| `toss.NewTokenStore` | Toss 는 client_id 당 활성 토큰이 **하나**다. 워커가 자기 토큰을 발급하는 순간 API 의 토큰이 401 이 되고 시세 적재가 끊긴다 (실제로 겪은 사고) |
| `apibudget.NewManager` | 외부 API 사용량을 두 배로 쓰고, 관리자 화면의 사용량이 거짓이 된다 |
| `MarketIndexStore` | API 폴러가 쓴 지수 스냅샷을 워커가 못 읽는다 |

그래서 API VM 의 Redis 를 **사설 IP** 에 열고(`REDIS_BIND`), `requirepass` 를 걸고,
NSG 로 배치 VM 만 6379 에 닿게 한다. `batch-stack.sh` 의 헬스 체크가
`REDIS_URL_CONTAINER` 가 로컬을 가리키면 잡아낸다.

## TLS 최초 발급

인증서가 없으면 `:443` 블록이 뜨지 못한다. 평문으로 먼저 올린다.

```bash
sudo mkdir -p /etc/quant-port /var/www/certbot
sudo cp deploy/nginx/tls_site.conf.example /etc/quant-port/nginx-tls-site.conf
sudo vi /etc/quant-port/nginx-tls-site.conf          # 도메인 3곳

# 1) 평문으로 올려 ACME 챌린지를 통과시킨다
echo 'NGINX_CONF=nginx.conf'  >> deploy/.env.app
echo 'NGINX_HTTP_PORT=80'     >> deploy/.env.app
./deploy/scripts/app-stack.sh up

sudo certbot certonly --webroot -w /var/www/certbot -d app.example.com

# 2) TLS 로 전환
sed -i 's/NGINX_CONF=nginx.conf/NGINX_CONF=nginx.tls.conf/' deploy/.env.app
./deploy/scripts/app-stack.sh up
```

갱신 후 리로드는 무중단으로:

```bash
docker kill -s HUP quant-port-nginx
```

HSTS 는 `max-age=300` 으로 시작한다. **갱신이 한 주기 도는 것을 확인한 뒤에** 늘린다
— 한 번 보낸 HSTS 는 브라우저가 기억하므로, 인증서가 끊기면 접근할 방법이 없다.

## 호스트 쪽에서 한 번 해 둘 것

compose 의 로그 상한은 이 스택에만 걸린다. 호스트 전체 기본값도 함께 둔다.

```bash
sudo tee /etc/docker/daemon.json >/dev/null <<'EOF'
{
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "3" },
  "builder": { "gc": { "enabled": true, "defaultKeepStorage": "5GB" } },
  "live-restore": true
}
EOF
sudo systemctl restart docker
```

`builder.gc` 가 없으면 빌드 캐시가 정리 정책 없이 자란다 — 로컬 실측 **31.6 GB**,
부트 볼륨은 50 GiB 다. 스크립트의 `deploy` 는 기동 전 디스크 사용률을 보고
90% 이상이면 멈춘다.

## 한 대에서 두 스택을 함께 검증하기

배치 compose 는 `container_name` 을 `${CONTAINER_PREFIX:-quant-port}-worker-*` 로 짓는다.
로컬에서 API 스택이 이미 같은 이름을 쓰고 있으므로, 검증할 때만 접두사를 바꾼다.

```bash
cat > deploy/.env.batch <<'ENV'
MYSQL_DSN=quantport:PW@tcp(host.docker.internal:3306)/quantport?parseTime=true&charset=utf8mb4&loc=Asia%2FSeoul
REDIS_URL_CONTAINER=redis://host.docker.internal:6379/0
CONTAINER_PREFIX=qp-batchtest
ENV

./deploy/scripts/batch-stack.sh up      # 8개 전부 ✓ 가 나와야 한다
./deploy/scripts/batch-stack.sh down
```

배치 VM 에서는 `CONTAINER_PREFIX` 를 건드리지 않는다 — 문서와 운영 명령이
`quant-port-worker-*` 이름을 전제한다.

## Windows PowerShell 스크립트의 인코딩 (2026-09-04)

이 저장소의 `.ps1` 은 Windows PowerShell **5.1**(`powershell.exe`)이 실행한다.
그 판은 **BOM 이 없는 `.ps1` 을 ANSI(코드페이지 949)로 읽는다.** 그래서 한글이
든 스크립트는 문자열이 깨지고, 운이 나쁘면 파싱 자체가 실패한다.

- 한글이 한 글자라도 있는 `.ps1` 은 **UTF-8 with BOM** 으로 저장한다.
- 자식 프로세스(node·go·docker)의 출력을 파일에 남길 때는
  `deploy/scripts/Utf8Log.ps1` 의 `Start-Utf8Log` 를 쓴다. `Tee-Object` 는 5.1 에
  `-Encoding` 이 없어 **UTF-16LE** 로 쓰고, `[Console]::OutputEncoding` 을 UTF-8 로
  올리지 않으면 그 전에 이미 문자열이 망가진다(그 실측과 배경은 그 파일 머리글에).

```powershell
powershell -NoProfile -File deploy\scripts\Utf8Log.Tests.ps1
```

## 워치독 — 스택이 스스로 돌아오게 한다 (2026-09-09)

### 왜 있는가

2026-09-08 22:07, 호스트가 커널 버그체크(`0x0000000A`)로 죽고 **60초 만에**
재부팅됐다. 그런데 스택은 **23:44 까지 96분을 더 누워 있었다.** 총 중단 1시간 37분
중 재부팅은 1분이었고, 나머지 96분은 다른 이유였다.

컨테이너 13개는 전부 `restart: unless-stopped` 였다. 설정은 옳았다. 문제는 그
정책이 **도커 엔진이 살아 있을 때만** 동작한다는 것이다 — Docker Desktop 이
`AutoStart=False` 라 부팅 후 엔진이 뜨지 않았고, 엔진이 없으니 재시작시킬 주체가
없었다. **재시작 정책이 있다는 것은 복구된다는 뜻이 아니다.** 정책은 언제나 자기보다
아래 계층의 생존을 전제한다.

워치독이 메우는 자리는 그래서 **컨테이너 아래층(엔진)과 위층(관측)** 이다.
컨테이너 층은 이미 도커가 지키므로 건드리지 않는다.

### 쓰기

```powershell
# 등록 (로그온 +1분, 그리고 5분마다)
powershell -ExecutionPolicy Bypass -File deploy\scripts\install-watchdog.ps1

# 고치지 않고 상태만 보기
powershell -ExecutionPolicy Bypass -File deploy\scripts\watchdog.ps1 -CheckOnly

# 해제
powershell -ExecutionPolicy Bypass -File deploy\scripts\install-watchdog.ps1 -Remove
```

| 항목 | 값 |
| ---- | -- |
| 헬스 기준 | `http://127.0.0.1/health` (nginx — 사용자가 실제로 지나는 길) |
| 로그 | `deploy/logs/watchdog.log` (5MB 에서 1회 회전) |
| 상태 | `deploy/logs/watchdog-state.json` |
| 알림 | `deploy/watchdog.config.json` 의 `WebhookUrl` (Discord·Slack 겸용, 비우면 로그만) |
| 일시정지 | `deploy/watchdog.paused` 파일을 만들면 아무것도 되살리지 않는다 |
| 실행 방식 | 작업 스케줄러 → `wscript.exe watchdog-hidden.vbs` → `watchdog.ps1` (창 없음) |

**`watchdog.paused` 를 꼭 기억할 것.** 스택을 일부러 내려 두고 작업할 때 이 파일이
없으면 워치독이 5분 뒤에 도로 올려 놓는다.

### 하는 일과 하지 않는 일

되살릴 때 `docker compose up -d` 를 **쓰지 않는다.** 두 가지 이유다.
`fe`·`nginx` 는 `prod` 프로파일이라 프로파일 없이 `up` 하면 빠지고, `up` 은
compose 파일과 `.env` 를 다시 평가해 컨테이너를 재생성할 수 있다.
**워치독이 배포를 하면 안 된다 — 되살리기만 해야 한다.** 그래서 라벨
(`com.docker.compose.project=quant-port`)로 멈춘 컨테이너를 찾아 이름으로
`docker start` 한다.

### ⚠ 한계 — Docker Desktop 은 GUI 앱이다

2026-09-09 검증에서 드러났다. Docker Desktop 을 강제 종료한 뒤 워치독을 돌렸더니
**감지·기동·판정·알림은 전부 설계대로 동작했는데 복구는 실패했다.** Docker Desktop 이
「An unexpected error occurred」 모달 대화상자를 띄우고 **사람이 Quit 을 누르기를
기다리고 있었기 때문이다.** 버튼은 `Quit` 과 `Reset to factory defaults` 둘뿐이었다.
서버에는 누를 사람이 없다.

원인은 고아가 된 AF_UNIX 소켓 파일이었다(`Docker\run\dockerInference`,
`docker-secrets-engine\engine.sock`). 지울 수도 열 수도 없는 상태로 남아 다음
기동이 그것을 `remove` 하려다 죽는다. 워치독은 이제 이 경우를 **알아보고 스스로
치운다** — `backend.error.json` 에서 경로를 읽어 그 디렉터리 이름을 바꾸고 한 번만
재기동한다(Docker 가 다시 만든다). 안전 조건 셋(경로가 지목됐고 · `%LOCALAPPDATA%`
아래이고 · 이름에 `ocker` 가 든다)을 모두 만족할 때만 손댄다.

그래도 남는 진짜 한계는 이것이다:

- Docker Desktop 은 **사용자 세션에 붙는다.** 자동 로그인이 아니면 부팅 후
  로그인 전까지 스택이 없다. 워치독으로는 못 메운다.
- 모달 대화상자의 **다른 원인**은 여전히 사람이 필요하다.

무인 운영이 목표라면 답은 워치독이 아니라 **바닥을 바꾸는 것**이다 —
WSL2 배포판 안의 `dockerd` 를 systemd 서비스로 돌리거나(GUI 제거),
애초에 설계돼 있는 OCI VM(`app-stack.sh`·`batch-stack.sh`)으로 옮기는 쪽이다.
리눅스에서는 `systemctl enable docker` 한 줄이 이 문서 전체를 대신한다.

### 왜 `wscript.exe` 를 거치는가 (2026-09-09)

작업 스케줄러가 `powershell.exe` 를 직접 부르면 **5분마다 창이 번쩍이고 키보드
포커스를 빼앗는다.** `-WindowStyle Hidden` 을 줘도 소용없다 — powershell 은 콘솔을
**먼저 만들고 나서** 숨기고, Windows 11 은 그 콘솔을 **Windows Terminal** 로 띄우기
때문에 숨기라는 지시가 닿기 전에 이미 화면에 뜬다.

실측(2026-09-09, 창 열거로 확인):

| 실행 방식 | 새로 뜨는 보이는 창 |
| --------- | ------------------- |
| `powershell.exe -WindowStyle Hidden` | **`WindowsTerminal`** |
| `wscript.exe watchdog-hidden.vbs` | **없음** |

`wscript.exe` 는 자기 콘솔이 없다. 거기서 `intWindowStyle = 0` 으로 powershell 을
띄우면 아무것도 보이지 않는다. `.vbs` 는 `Run(..., 0, True)` 로 **기다렸다가**
종료 코드를 그대로 돌려주므로, 태스크의 `LastTaskResult` 와 `MultipleInstances=IgnoreNew`
가 그대로 의미를 갖는다.

`watchdog-hidden.vbs` 는 **ASCII 로만** 쓴다 — wscript 는 `.vbs` 를 ANSI 로 읽어
한글 주석이 깨진다. 설명은 `watchdog.ps1` 쪽에 둔다.

점검 주기를 바꾸려면:

```powershell
powershell -ExecutionPolicy Bypass -File deploy\scripts\install-watchdog.ps1 -IntervalMinutes 15
```
