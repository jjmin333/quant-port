#!/usr/bin/env bash
#
# app-stack.sh / batch-stack.sh 의 공통 뼈대.
#
# 두 스크립트가 하는 일은 같다 — "이 VM 이 맡은 컨테이너 묶음"을 한 번에 올리고,
# 내리고, 재기동하고, 상태를 보여 준다. 다른 것은 **어떤 compose 파일과 어떤 서비스
# 목록을 쓰는가** 뿐이라, 그 둘만 호출부에서 채우고 나머지는 여기 둔다.
#
# 호출부가 소싱 전에 정의해야 하는 것:
#   STACK_LABEL     사람이 읽는 이름 (로그 출력용)
#   PROJECT_NAME    docker compose 프로젝트 이름 (-p)
#   ENV_FILE        compose 변수 치환용 파일 (없으면 건너뜀)
#   COMPOSE_FILES   -f 인자 배열
#   SERVICES        이 스택이 소유한 서비스 이름 배열 (비면 파일 전체)
#   PROFILES        --profile 인자 배열 (선택)
#   HEALTH_CMD      health 서브커맨드가 실행할 함수 이름 (선택)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

c_red() { printf '\033[31m%s\033[0m\n' "$*"; }
c_grn() { printf '\033[32m%s\033[0m\n' "$*"; }
c_ylw() { printf '\033[33m%s\033[0m\n' "$*"; }
say()   { printf '\n\033[1m▸ %s\033[0m\n' "$*"; }

die() { c_red "✗ $*"; exit 1; }

# compose 호출 한 줄로 모으기. --project-directory 를 리포 루트로 고정하는 이유:
# compose 파일 안의 상대 경로(./be, ./deploy/...)가 파일 위치가 아니라 리포 루트
# 기준으로 풀려야, 루트 파일과 deploy/compose/ 아래 파일이 같은 경로를 쓸 수 있다.
compose() {
  local args=(--project-directory "$REPO_ROOT" -p "$PROJECT_NAME")
  if [[ -n "${ENV_FILE:-}" && -f "$ENV_FILE" ]]; then
    args+=(--env-file "$ENV_FILE")
  fi
  local f
  for f in "${COMPOSE_FILES[@]}"; do args+=(-f "$f"); done
  local p
  for p in "${PROFILES[@]:-}"; do
    [[ -n "$p" ]] && args+=(--profile "$p")
  done
  docker compose "${args[@]}" "$@"
}

preflight() {
  command -v docker >/dev/null || die "docker 가 없습니다."
  docker compose version >/dev/null 2>&1 || die "docker compose v2 가 필요합니다."
  [[ -f "$REPO_ROOT/be/.env" ]] || die "be/.env 가 없습니다. cp be/.env.example be/.env 후 값을 채우세요."
  if [[ -n "${ENV_FILE:-}" && ! -f "$ENV_FILE" ]]; then
    c_ylw "! $ENV_FILE 가 없습니다 — compose 변수는 셸 환경에서만 읽습니다."
  fi
  # 설정 오류를 컨테이너를 띄우기 전에 잡는다. 필수 변수가 비면 여기서 실패한다.
  compose config >/dev/null || die "compose 설정이 유효하지 않습니다 (위 오류 참조)."
}

# 디스크는 이 배포 방식의 가장 큰 위험이다 — 빌드 캐시가 정리 정책 없이 자란다.
# 올리기 전에 한 번 보고, 위험하면 멈춘다.
check_disk() {
  local pct
  pct="$(df -P "$REPO_ROOT" | awk 'NR==2 {gsub(/%/,"",$5); print $5}')" || return 0
  if (( pct >= 90 )); then
    die "디스크 사용률 ${pct}% — 빌드/기동 전에 정리가 필요합니다. 'docker system df' 로 확인하고 prune 를 먼저 도세요."
  elif (( pct >= 80 )); then
    c_ylw "! 디스크 사용률 ${pct}% — 곧 정리가 필요합니다 (prune 서브커맨드)."
  fi
}

do_up() {
  preflight
  check_disk
  say "$STACK_LABEL 기동"
  compose up -d --remove-orphans "$@" "${SERVICES[@]}"
  do_ps
  [[ -n "${HEALTH_CMD:-}" ]] && "$HEALTH_CMD" || true
}

do_build() {
  preflight
  check_disk
  say "$STACK_LABEL 이미지 빌드"
  compose build "${SERVICES[@]}"
  c_grn "✓ 빌드 완료 — 반영하려면 'up' 을 실행하세요."
}

do_deploy() {
  preflight
  check_disk
  say "$STACK_LABEL 빌드 + 기동"
  compose up -d --build --remove-orphans "${SERVICES[@]}"
  do_ps
  [[ -n "${HEALTH_CMD:-}" ]] && "$HEALTH_CMD" || true
  # 빌드 직후가 이미지가 가장 많이 쌓이는 순간이다. 여기서 한 번 털어 둔다.
  say "쓰지 않는 이미지 정리"
  docker image prune -f >/dev/null && c_grn "✓ dangling 이미지 제거"
}

do_pull() {
  preflight
  say "$STACK_LABEL 이미지 가져오기"
  compose pull "${SERVICES[@]}"
}

do_down() {
  say "$STACK_LABEL 정지"
  # 이 스택이 소유한 것만 멈춘다. `compose down` 은 프로젝트 전체를 내리므로
  # 같은 프로젝트에 다른 서비스가 있으면 함께 죽는다 — stop 을 쓴다.
  compose stop "${SERVICES[@]}"
  c_grn "✓ 정지"
}

do_restart() {
  say "$STACK_LABEL 재기동"
  compose restart "${SERVICES[@]}"
  do_ps
  [[ -n "${HEALTH_CMD:-}" ]] && "$HEALTH_CMD" || true
}

do_ps() {
  say "$STACK_LABEL 상태"
  # 이 스택이 소유한 서비스만 — compose ps 는 기본이 프로젝트 전체라,
  # 한 대에서 두 스택을 돌릴 때 남의 컨테이너까지 보인다.
  compose ps --format 'table {{.Name}}\t{{.Status}}\t{{.Ports}}' "${SERVICES[@]}" 2>/dev/null \
    || compose ps "${SERVICES[@]}"
  printf '\n'
  # 상한 대비 실제 사용량. 리소스 상한을 조정할 때 근거가 되는 유일한 숫자다.
  local names
  names="$(compose ps -q "${SERVICES[@]}" 2>/dev/null | tr '\n' ' ')"
  if [[ -n "${names// /}" ]]; then
    # shellcheck disable=SC2086
    docker stats --no-stream --format 'table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}' $names 2>/dev/null || true
  fi
}

do_logs() {
  local svc="${1:-}"
  if [[ -n "$svc" ]]; then
    compose logs -f --tail=200 "$svc"
  else
    compose logs -f --tail=100 "${SERVICES[@]}"
  fi
}

# 재기동이 반복되는지 본다 — restart: unless-stopped 는 무한히 재시도하므로,
# 크래시 루프는 사람이 보지 않으면 조용히 계속된다.
do_doctor() {
  say "$STACK_LABEL 점검"
  local id name restarts oom status bad=0
  while read -r id; do
    [[ -z "$id" ]] && continue
    read -r name restarts oom status <<<"$(docker inspect --format '{{.Name}} {{.RestartCount}} {{.State.OOMKilled}} {{.State.Status}}' "$id")"
    local line="  ${name#/}  restarts=${restarts}  oom=${oom}  ${status}"
    if [[ "$oom" == "true" ]]; then
      c_red "$line   ← OOM 킬. 메모리 상한 또는 GOMEMLIMIT 를 확인하세요."
      bad=1
    elif (( restarts > 3 )); then
      c_ylw "$line   ← 재기동이 잦습니다. logs 를 보세요."
      bad=1
    elif [[ "$status" != "running" ]]; then
      c_ylw "$line"
      bad=1
    else
      printf '%s\n' "$line"
    fi
  done < <(compose ps -q "${SERVICES[@]}" 2>/dev/null)

  printf '\n'
  df -h "$REPO_ROOT" | awk 'NR==1||NR==2'
  printf '\n'
  docker system df

  (( bad == 0 )) && c_grn "✓ 이상 없음"
}

do_prune() {
  say "쓰지 않는 이미지·빌드 캐시 정리"
  docker image prune -f
  # 빌드 캐시는 정리 정책이 없으면 무제한으로 자란다(실측 31.6 GB).
  # 5 GB 는 남긴다 — 다음 빌드가 처음부터 다시 하지 않게.
  docker builder prune -f --keep-storage 5GB
  printf '\n'
  docker system df
}

usage() {
  cat <<EOF
$STACK_LABEL 관리

  사용법: $(basename "$0") <명령> [인자]

  up            기동 (이미지가 있으면 그대로)
  deploy        빌드 후 기동 + dangling 이미지 정리   ← 평소 배포
  build         이미지만 빌드
  pull          레지스트리에서 이미지 가져오기
  down          정지 (이 스택이 소유한 컨테이너만)
  restart       재기동
  ps            상태 + 자원 사용량
  logs [서비스] 로그 따라가기
  doctor        재기동/OOM/디스크 점검
  prune         이미지·빌드 캐시 정리
  config        최종 compose 설정 출력 (디버그)

  대상 서비스: ${SERVICES[*]}
EOF
}

stack_main() {
  local cmd="${1:-}"
  shift || true
  case "$cmd" in
    up)      do_up "$@" ;;
    deploy)  do_deploy ;;
    build)   do_build ;;
    pull)    do_pull ;;
    down|stop) do_down ;;
    restart) do_restart ;;
    ps|status) do_ps ;;
    logs)    do_logs "$@" ;;
    doctor)  do_doctor ;;
    prune)   do_prune ;;
    config)  compose config ;;
    ""|-h|--help|help) usage ;;
    *) c_red "알 수 없는 명령: $cmd"; printf '\n'; usage; exit 2 ;;
  esac
}
