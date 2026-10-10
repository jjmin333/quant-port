## Agent protocol

**Start**: read `docs/AGENT_STATE.md` only. It is the current state and is capped at 150 lines. Do not read history at start.

**Retrieve only when the task needs it**:
- Principles for storage, cache, quotes, LLM spend and list APIs → `docs/PRINCIPLES.md`, read only when touching those areas
- Decisions and ops traps → find the title in the MEMORY.md index (auto-loaded), then read that one memory file
- Past work → `Grep "<keyword|MM-DD>" docs/archive/sessions.md`, then Read a small range around the hit. Never read the whole file (≈1 MB).
- Code is the source of truth for current behavior. Archive text can be stale.

**Work**:
- For large files: search → narrow → read a small range. Don't re-read a file you already read unless it changed.
- Cap command output: `go test ./... 2>&1 | tail -30` or failures only, `npx tsc --noEmit | head -40`, `docker logs --tail 100 <c> 2>&1 | grep -iE 'error|panic'`, `git diff --stat` before any full diff.
- Don't rerun a passing check unless the code changed since. Run the relevant checks again once before finishing.
- Write a plan only for multi-step work. Small fixes go straight to implementation.

**End** (only if work was done):
0. **Deploy what you changed — without asking** (user standing authorization, 2026-10-07). See 「Deploy after changes」 below. Report the result; if a deploy step fails, say so with the log lines.
1. **Overwrite** the sections of `docs/AGENT_STATE.md`: NOW / BLOCKERS / PENDING / DECISIONS / VERIFY / NEXT / BACKLOG. Delete finished items. Write state, not a narrative. Paths, numbers and how-it-works belong in code, memory or release notes. Stay ≤150 lines.
2. Append **one line** to `docs/archive/sessions.md`: `- YYYY-MM-DD: <what changed> · <deploy state>`.
3. A new durable rule goes in `docs/PRINCIPLES.md`. A user decision or ops trap goes in memory. Never in the state file.

---

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` before writing any code. Heed deprecation notices.
## Backend restart (Windows) — Docker only

After BE code changes, **always restart via Docker** (Tesseract OCR included). Do **not** use `go run ./cmd/api` or host `bin/api.exe`.

```powershell
cd be
.\scripts\restart.ps1 -Background
```

This runs `docker compose up -d --build redis api` from the repo root, waits for `/health`, and **reports Tesseract status** from container logs. On Tesseract failure, fix/rebuild and report to the user.

Logs: `docker logs -f quant-port-api`

## Deploy after changes — automatic (user authorization 2026-10-07)

The user authorized **restarting the API/workers and rebuilding the FE container at the end of any task that changed them, without asking first**. Do it once, after all edits and checks pass (not after every edit).

1. **Before**: make sure no long batch is running in a worker you will recreate (`docker logs --tail 20 quant-port-worker-*`, or `batch_job_runs.status='running'`). If one is, wait or leave that worker out and say so. Do not deploy with failing tests/tsc.
2. **BE changed** → `cd be; .\scripts\restart.ps1 -ApiOnly -Background` (builds the shared `quant-port-api` image). Then recreate the workers whose code paths changed: `docker compose up -d --no-build --no-deps --force-recreate <worker...>` (workers share the api image — never add `build:` back to them).
3. **FE changed** → `docker compose --profile prod up -d --build fe` from the repo root. Edits do **not** reach users until this runs (`NGINX_ROUTES=app_routes.conf`). This `--build` **also rebuilds and recreates api** (dependency) and gives it a new image id — so when both changed, do the FE step **before** recreating workers (or recreate the workers again after it).
4. **After**: `/health` 200, `docker ps` shows api/fe not restart-looping, `docker logs --tail 200 quant-port-api 2>&1 | grep -iE 'panic|fatal'` is empty, and `docker inspect <worker> --format '{{.Image}}'` equals the api container's image.

Still **ask first** for: DB DROP/data deletion, external account or secret changes, Cloudflare/nginx config, and anything the user marked as needing a decision.

## Cloud sessions (claude.ai/code · 원격 컨테이너)

GitHub `jjmin333/quant-port`(public, `master`)를 받아 작업한다. 운영 스택(MySQL·Redis·Docker·Toss 자격증명)은 **사용자 PC에만** 있다.
- 되는 것: `cd be && go build ./... && go test ./...`(DB가 필요한 시험은 `*_TEST_DSN`·`LIVE_MYSQL`이 없으면 건너뛴다), `cd fe && npm ci && npx tsc --noEmit && npx vitest run && npx eslint <파일>`.
- 안 되는 것: 위 「Backend restart」·「Deploy after changes」(Windows·Docker 전용) — 클라우드에서는 **배포하지 않는다.** 브랜치를 푸시하고 PR을 올리면, 사용자 PC 세션이 머지 뒤 배포한다.
- 운영 데이터 확인은 공개 API(`https://www.quant-port.com/api/v1/...`)로 읽기만 한다. 비밀값은 저장소에 없다(`be/.env`·`be/secrets/`는 git 밖) — 새로 만들거나 커밋하지 않는다.
