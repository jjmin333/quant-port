import { request, type FullConfig } from "@playwright/test";

import { RESEARCH_CAP_SYMBOLS, SYMBOLS } from "./src/data";
import { API_URL, FE_URL, apiPath } from "./src/env";

/**
 * 스택이 떠 있는지, 테스트가 기대는 시장 데이터가 있는지 **먼저** 확인한다.
 *
 * 이 확인이 없으면 스택이 내려간 상태에서 스무 개 테스트가 제각기 다른 이유로
 * 실패하고, 원인은 그중 어디에도 안 적힌다. 여기서 한 번 죽는 편이 낫다.
 *
 * 서버를 직접 띄우지 않는 것은 의도다 — 이 프로젝트는 BE를 Docker로만 올리고
 * (`AGENTS.md`), dev 서버는 켠 사람 모르게 죽이지 않는 것이 규칙이다.
 */
export default async function globalSetup(config: FullConfig): Promise<void> {
  const api = await request.newContext({ baseURL: API_URL });

  const health = await api.get("/health").catch(() => null);
  if (!health || !health.ok()) {
    throw new Error(
      `BE API에 닿지 않습니다: ${API_URL}/health\n` +
        `  be 디렉터리에서 다음을 실행하세요:  .\\scripts\\restart.ps1 -Background`,
    );
  }

  /*
   * `/health`는 liveness라 프로세스가 살아 있기만 하면 200이다. 그것만 보고
   * 시작했다가 실제로 당했다 — API 컨테이너가 재기동한 직후(08:25 KST) 돌린
   * 회차에서 8건이 무더기로 깨졌고, 같은 창에서 `/indices/scores`와
   * `/news/domestic/briefing`이 500을 냈다(둘 다 기동 워밍업에 의존하는 경로).
   * 워밍업이 끝난 뒤로는 12회 연속 통과했다.
   *
   * `/ready`는 MySQL·Redis를 실제로 찔러 본다. 여기서 걸러야 "왜 깨졌는지 모르는
   * 8건" 대신 "스택이 아직 준비되지 않았다" 한 줄이 나온다.
   */
  const ready = await api.get("/ready").catch(() => null);
  const readyBody = ready ? ((await ready.json()) as {
    status: string;
    checks: Record<string, string>;
  }) : null;
  if (!ready || !ready.ok() || readyBody?.checks?.mysql !== "ok") {
    throw new Error(
      `BE가 아직 준비되지 않았습니다: ${API_URL}/ready → ${ready?.status() ?? "응답 없음"} ` +
        `${JSON.stringify(readyBody ?? {})}\n` +
        `  방금 재기동했다면 워밍업이 끝날 때까지 기다렸다가 다시 실행하세요.\n` +
        `  (기동 직후에는 캐시가 비어 있어 시세·점수·브리핑 경로가 일시적으로 실패합니다)`,
    );
  }

  // 시세·카탈로그가 비어 있으면 스탬프/리서치 발행이 "가격 없음"으로 실패한다.
  // 그건 회귀가 아니라 데이터 미적재이므로 여기서 구분해 준다.
  const missing: string[] = [];
  const needed = new Set<string>([...Object.values(SYMBOLS), ...RESEARCH_CAP_SYMBOLS]);
  for (const symbol of needed) {
    const quote = await api.get(apiPath(`/quotes/${symbol}`)).catch(() => null);
    if (!quote || !quote.ok()) {
      missing.push(symbol);
    }
  }
  if (missing.length > 0) {
    throw new Error(
      `시세를 읽을 수 없는 종목이 있습니다: ${missing.join(", ")}\n` +
        `  스탬프·리서치 발행 테스트는 실제 시세로 스냅샷을 만듭니다.\n` +
        `  종목 카탈로그 적재(cmd/seed-stocks, cmd/sync-etp)를 먼저 확인하세요.`,
    );
  }

  await api.dispose();

  // `--project=api`로 돌릴 때는 FE가 없어도 된다. 선택된 프로젝트만 보고 정한다.
  const needsBrowser = config.projects.some((project) => project.name === "ui");
  if (!needsBrowser) return;

  const fe = await request.newContext({ baseURL: FE_URL });
  const home = await fe.get("/").catch(() => null);
  if (!home || home.status() >= 500) {
    throw new Error(
      `FE에 닿지 않습니다: ${FE_URL}\n` +
        `  fe 디렉터리에서 npm run dev 를 실행하거나 E2E_FE_URL 을 지정하세요.\n` +
        `  (API 계층 테스트만 돌리려면: npm run test:api)`,
    );
  }
  await fe.dispose();
}
