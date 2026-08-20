## Agent state

Start: read `docs/AGENT_STATE.md`. End: update it; append one line to `docs/archive/sessions.md` only if work was done.

---

<!-- BEGIN:nextjs-agent-rules -->
# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` before writing any code. Heed deprecation notices.
<!-- END:nextjs-agent-rules -->

## Backend restart (Windows) — Docker only

After BE code changes, **always restart via Docker** (Tesseract OCR included). Do **not** use `go run ./cmd/api` or host `bin/api.exe`.

```powershell
cd be
.\scripts\restart.ps1 -Background
```

This runs `docker compose up -d --build redis api` from the repo root, waits for `/health`, and **reports Tesseract status** from container logs. On Tesseract failure, fix/rebuild and report to the user.

Logs: `docker logs -f quant-port-api`
