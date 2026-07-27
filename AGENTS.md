## Agent state

Start: read `docs/AGENT_STATE.md`. End: update it; append one line to `docs/archive/sessions.md` only if work was done.

---

<!-- BEGIN:nextjs-agent-rules -->
# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` before writing any code. Heed deprecation notices.
<!-- END:nextjs-agent-rules -->

## Backend restart (Windows)

After BE code changes, **do not use `go run ./cmd/api`** for test restarts — it compiles to a temp executable and can trigger Windows Firewall prompts.

Use the build script instead:

```powershell
cd be
.\scripts\restart.ps1 -Background
```

This runs `go build -o bin/api.exe ./cmd/api` and starts the fixed binary. Logs: `be/tmp/api.log`.
