#!/usr/bin/env bash
#
# API / FE VM 의 컨테이너 묶음을 한 번에 다룬다.
#   redis · api · fe · nginx   (배치 워커 8개는 batch-stack.sh 소관)
#
#   ./deploy/scripts/app-stack.sh deploy    # 빌드 + 기동  ← 평소 배포
#   ./deploy/scripts/app-stack.sh ps
#   ./deploy/scripts/app-stack.sh logs api
#   ./deploy/scripts/app-stack.sh doctor
#
# 배치를 같은 VM 에서 돌리려면 batch-stack.sh 대신 루트 compose 의 워커를 쓰거나
# (단일 VM 구성), 이 스크립트 실행 후 batch-stack.sh 를 따로 실행한다.
#
# 배포 순서: **API VM 을 먼저.** 스키마 마이그레이터는 api 하나뿐이고
# 워커는 스키마를 최대 3분 기다린다(repository.VerifySchema).

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

STACK_LABEL="API/FE 스택"
PROJECT_NAME="${COMPOSE_PROJECT_NAME:-quant-port}"
ENV_FILE="$(cd "$SCRIPT_DIR/../.." && pwd)/deploy/.env.app"
COMPOSE_FILES=(
  "docker-compose.yml"
  "deploy/compose/docker-compose.app.oci.yml"
)
# fe·nginx 는 prod 프로파일에 있다.
PROFILES=("prod")
SERVICES=(redis api fe nginx)

# 로컬 개발이나 TLS 이전 단계에서는 OCI 오버레이를 빼고 돌릴 수 있다.
#   APP_STACK_PLAIN=1 ./deploy/scripts/app-stack.sh up
if [[ "${APP_STACK_PLAIN:-0}" == "1" ]]; then
  COMPOSE_FILES=("docker-compose.yml")
  STACK_LABEL="API/FE 스택 (오버레이 없음)"
fi

app_health() {
  say "헬스 체크"
  local ok=1

  # api 는 loopback 에만 게시돼 있다 — VM 안에서만 닿는 것이 정상이다.
  if curl -fsS --max-time 5 http://127.0.0.1:8080/health >/dev/null 2>&1; then
    c_grn "✓ api /health"
  else
    c_red "✗ api /health 응답 없음 — logs api 를 보세요."; ok=0
  fi

  # Tesseract 는 이미지에 들어 있어야 한다. 없으면 스크린샷 가져오기가 조용히 실패한다.
  if docker exec quant-port-api tesseract --version >/dev/null 2>&1; then
    c_grn "✓ tesseract"
  else
    c_red "✗ tesseract 없음 — 이미지 재빌드가 필요합니다."; ok=0
  fi

  # nginx 는 프로파일이 꺼져 있을 수 있으므로 있을 때만 본다.
  if docker ps --format '{{.Names}}' | grep -q '^quant-port-nginx$'; then
    if docker exec quant-port-nginx wget -q -O - http://127.0.0.1/health >/dev/null 2>&1; then
      c_grn "✓ nginx → api"
    else
      c_red "✗ nginx 가 api 로 못 갑니다."; ok=0
    fi
  fi

  # 커넥션 풀 대기는 포화의 가장 이른 신호다. 0 이 아니면 알아야 한다.
  local waits
  waits="$(curl -fsS --max-time 5 http://127.0.0.1:8080/metrics 2>/dev/null \
            | awk '/^quantport_mysql_wait_total /{print $2}')" || true
  if [[ -n "${waits:-}" ]]; then
    if [[ "$waits" == "0" ]]; then
      c_grn "✓ mysql 풀 대기 0"
    else
      c_ylw "! mysql 풀 대기 ${waits}건 — 커넥션 예산 또는 DB CPU 를 확인하세요."
    fi
  fi

  (( ok == 1 )) || return 1
}
HEALTH_CMD=app_health

# shellcheck source=./_stack.sh
source "$SCRIPT_DIR/_stack.sh"
stack_main "$@"
