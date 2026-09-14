#!/usr/bin/env bash
#
# 배치 VM 의 워커 컨테이너 8개를 한 번에 다룬다.
#   worker-close-domestic / -overseas · scores · index · fetch · research · earnings · filings
#
#   ./deploy/scripts/batch-stack.sh deploy   # 빌드 + 기동  ← 평소 배포
#   ./deploy/scripts/batch-stack.sh ps
#   ./deploy/scripts/batch-stack.sh logs worker-fetch
#   ./deploy/scripts/batch-stack.sh doctor
#
# 전제 두 가지 (deploy/.env.batch 에 설정):
#   MYSQL_DSN            관리형 MySQL 사설 엔드포인트
#   REDIS_URL_CONTAINER  **API VM 의 Redis** — 자체 Redis 를 띄우면 안 된다.
#                        Toss 토큰(client_id당 활성 토큰 1개)·apibudget 사용량·
#                        지수 스냅샷을 API 프로세스와 같은 인스턴스에서 봐야 한다.
#
# 배포 순서: API VM 을 먼저 올린 뒤 이쪽. 워커는 DDL 을 실행하지 않고
# repository.VerifySchema 로 스키마를 최대 3분 기다린다.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

STACK_LABEL="배치 스택"
# API VM 과 프로젝트 이름을 나눈다 — 한 대에서 둘 다 시험할 때 서로를 지우지 않게.
PROJECT_NAME="${COMPOSE_PROJECT_NAME:-quant-port-batch}"
ENV_FILE="$(cd "$SCRIPT_DIR/../.." && pwd)/deploy/.env.batch"
COMPOSE_FILES=("deploy/compose/docker-compose.batch.yml")
PROFILES=()
SERVICES=(
  worker-close-domestic
  worker-close-overseas
  worker-scores
  worker-index
  worker-fetch
  worker-research
  worker-earnings
  worker-filings
)

batch_health() {
  say "워커 상태"
  local ok=1

  # 워커는 HTTP 포트가 없다. "떠 있음"만으로는 일하는지 알 수 없으므로,
  # 기동 로그에 부트스트랩 실패가 없는지와 최근 로그가 도는지를 본다.
  local svc cid
  for svc in "${SERVICES[@]}"; do
    cid="$(compose ps -q "$svc" 2>/dev/null || true)"
    if [[ -z "$cid" ]]; then
      c_red "✗ $svc 컨테이너 없음"; ok=0; continue
    fi
    if [[ "$(docker inspect --format '{{.State.Status}}' "$cid")" != "running" ]]; then
      c_red "✗ $svc 정지 상태"; ok=0; continue
    fi
    local logs
    logs="$(docker logs --tail 50 "$cid" 2>&1)"

    # 실패 신호는 `bootstrap:` 하나로 충분하다 — BootstrapWorker 가 config/mysql/
    # schema/redis 오류를 전부 그 접두사로 감싸고 log.Fatalf 로 죽는다.
    #
    # **패턴을 넓히지 말 것.** 처음에 `mysql:` 을 넣었다가 정상 기동 로그
    # ("mysql: schema verified") 를 오류로 잡았다 — 8개 전부 오탐이었다.
    if grep -qE 'bootstrap:' <<<"$logs"; then
      c_red "✗ $svc 기동 실패 (logs $svc 확인)"; ok=0
    elif grep -qE "^worker-[a-z-]+: running|${svc}: running" <<<"$logs"; then
      c_grn "✓ $svc"
    else
      # 떠 있는데 준비 배너가 없다 — 아직 기동 중이거나(스키마 대기 최대 3분)
      # 배너 없이 멈춘 상태다. 단정하지 않고 알린다.
      c_ylw "! $svc 준비 배너 없음 (기동 중이거나 멈춤 — logs $svc)"
    fi
  done

  # 배치 VM 에서 가장 흔한 오설정: Redis 를 자기 자신으로 잡은 것.
  # 그러면 Toss 토큰이 갈라져 API 쪽 시세가 401 로 죽는다.
  local url="${REDIS_URL_CONTAINER:-}"
  if [[ -z "$url" && -f "$ENV_FILE" ]]; then
    url="$(awk -F= '/^REDIS_URL_CONTAINER=/{sub(/^[^=]*=/,""); print; exit}' "$ENV_FILE")"
  fi
  if [[ "$url" == *"@redis:"* || "$url" == *"//redis:"* || "$url" == *"127.0.0.1"* || "$url" == *"localhost"* ]]; then
    c_red "✗ REDIS_URL_CONTAINER 가 로컬 Redis 를 가리킵니다: $url"
    c_red "  배치 VM 은 **API VM 의 Redis** 를 써야 합니다 — 갈라지면 Toss 토큰이 서로를 401 로 만듭니다."
    ok=0
  fi

  (( ok == 1 )) || return 1
}
HEALTH_CMD=batch_health

# shellcheck source=./_stack.sh
source "$SCRIPT_DIR/_stack.sh"
stack_main "$@"
