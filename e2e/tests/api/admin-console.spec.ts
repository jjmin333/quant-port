import { SYMBOLS } from "@/data";
import { expect, hasAdminCredentials, test } from "@/fixtures";
import type { BatchJobStatus } from "@/types";

/**
 * 관리자 콘솔 (INV-G, H, I군).
 *
 * 자격증명이 없으면 전부 건너뛴다 — 테스트는 관리자 계정을 만들지 않는다
 * (이유는 `src/env.ts`). 일반 사용자가 막힌다는 것은 `authorization.spec.ts`에서
 * 자격증명 없이도 검증하므로, 보안 경계 자체는 언제나 검사된다.
 */

test.beforeEach(() => {
  test.skip(
    !hasAdminCredentials,
    "E2E_ADMIN_EMAIL / E2E_ADMIN_PASSWORD 를 지정하면 관리자 콘솔 검증이 켜집니다",
  );
});

type TableSummary = {
  name: string;
  access: "full" | "read-only" | "redacted";
  redactColumns?: string[];
};

type RowPage = {
  columns?: Array<{ name: string; redacted: boolean }>;
  rows: Array<Record<string, unknown>>;
};

type QuotaUsage = { provider: string; category: string; dayCount: number; minuteCount: number };

test.describe("DB 콘솔 — 접근 범위", () => {
  test("카탈로그에 없는 테이블은 이름을 지어내도 닿지 않는다", async ({ adminApi }) => {
    // 테이블 이름이 요청에서 SQL로 흘러가면 임의 테이블 접근이자 인젝션이다.
    // 실제로는 카탈로그에서만 오므로, 없는 이름은 404로 끝나야 한다.
    for (const table of ["information_schema.tables", "mysql.user", "totally_made_up"]) {
      await adminApi!.expectFailure("GET", `/admin/db/tables/${encodeURIComponent(table)}`, 404);
      await adminApi!.expectFailure(
        "GET",
        `/admin/db/tables/${encodeURIComponent(table)}/rows`,
        404,
      );
    }
  });

  test("읽기 전용 테이블은 관리자도 쓰지 못한다", async ({ adminApi }) => {
    const tables = await adminApi!.get<TableSummary[]>("/admin/db/tables");
    const readOnly = tables.find((t) => t.name === "research_calls");
    expect(readOnly, "research_calls 가 카탈로그에 있어야 한다").toBeDefined();
    expect(
      readOnly!.access,
      "발행된 콜이 콘솔에서 편집 가능하면 동결 계약이 관리자 화면으로 뚫린다",
    ).toBe("read-only");

    // 실제로도 막히는지 — 카탈로그 표시만 믿지 않는다.
    for (const method of ["POST", "PUT", "DELETE"] as const) {
      const res = await adminApi!.send(method, "/admin/db/tables/research_calls/rows", {
        key: { id: 1 },
        values: { title: "tampered" },
      });
      expect(res.ok(), `${method} 가 읽기 전용 테이블에서 통과했다`).toBe(false);
    }
  });

  test("비밀번호 해시는 어떤 응답에도 실려 나오지 않는다", async ({ adminApi }) => {
    const detail = await adminApi!.get<{ columns: Array<{ name: string; redacted: boolean }> }>(
      "/admin/db/tables/users",
    );
    const hashColumn = detail.columns.find((c) => c.name === "password_hash");
    expect(hashColumn, "users 에 password_hash 컬럼이 있어야 한다").toBeDefined();
    expect(hashColumn!.redacted, "해시 컬럼이 가려지지 않았다").toBe(true);

    const page = await adminApi!.get<RowPage>("/admin/db/tables/users/rows?limit=5");
    for (const row of page.rows) {
      expect(
        Object.keys(row),
        "행 payload에 해시가 실리면 관리자 화면 한 번으로 전 계정 해시가 유출된다",
      ).not.toContain("password_hash");
    }
  });

  test("users 테이블은 콘솔에서 편집되지 않는다", async ({ adminApi }) => {
    // 여기가 열리면 role을 손으로 바꿔 관리자를 만들 수 있다.
    const res = await adminApi!.send("PUT", "/admin/db/tables/users/rows", {
      key: { id: 1 },
      values: { role: "admin" },
    });
    expect(res.ok(), "users 가 편집 가능하면 권한 승격이 콘솔 한 번으로 가능해진다").toBe(false);
  });
});

test.describe("배치 — 입력 검증", () => {
  test("카탈로그에 없는 배치는 큐에 들어가지 않는다", async ({ adminApi }) => {
    const message = await adminApi!.expectFailure(
      "POST",
      "/admin/batch/run?job=not_a_real_job",
      400,
    );
    expect(message).toContain("배치");
  });

  test("job 없이 부르면 거부된다", async ({ adminApi }) => {
    await adminApi!.expectFailure("POST", "/admin/batch/run", 400);
  });

  test("잘못된 기준일은 거부된다", async ({ adminApi }) => {
    await adminApi!.expectFailure(
      "POST",
      "/admin/batch/run?job=daily_close_domestic&since=2026-13-99",
      400,
    );
  });

  test("배치 목록은 실행 상태를 함께 보고한다", async ({ adminApi }) => {
    const jobs = await adminApi!.get<BatchJobStatus[]>("/admin/batch/jobs");
    expect(jobs.length, "카탈로그가 비어 있으면 콘솔이 아무것도 못 한다").toBeGreaterThan(0);

    for (const job of jobs) {
      expect(job.spec.id, "job id가 없으면 실행 버튼이 무엇을 부르는지 알 수 없다").toBeTruthy();
      // running 플래그는 최근 실행의 상태에서 파생된다 — 둘이 어긋나면 화면이
      // "실행 중"인데 버튼은 눌리는(또는 그 반대) 상태가 된다.
      const last = job.lastRun;
      const derived = !!last && (last.status === "running" || last.status === "queued");
      expect(
        job.running,
        `${job.spec.id}: running=${job.running} 인데 최근 실행 상태는 ${last?.status ?? "없음"}`,
      ).toBe(derived);
    }
  });

  test("실행 중인 배치는 다시 큐에 들어가지 않는다", async ({ adminApi }) => {
    test.skip(
      process.env.E2E_ALLOW_BATCH_TRIGGER !== "1",
      "실제 배치를 돌린다 — E2E_ALLOW_BATCH_TRIGGER=1 일 때만 실행",
    );

    // daily_close_domestic 은 candle_bars 에서 재투영만 한다(외부 호출 없음,
    // 멱등). 중복 방지를 확인하기에 가장 부작용이 적은 잡이다.
    const job = "daily_close_domestic";
    const first = await adminApi!.send("POST", `/admin/batch/run?job=${job}`);
    expect(first.status(), "첫 트리거는 202 Accepted").toBe(202);

    const second = await adminApi!.send("POST", `/admin/batch/run?job=${job}`);
    expect(
      second.status(),
      "같은 잡이 두 번 큐에 들어가면 외부 한도를 쓰는 잡에서 중복 지출이 난다",
    ).toBe(409);
  });
});

test.describe("LLM 지출 경계", () => {
  test("종목 AI 요약 조회는 LLM 호출을 한 건도 발생시키지 않는다", async ({ adminApi, anon }) => {
    // 이 프로젝트의 비용 규칙: LLM 지출 진입점은 스케줄 배치와 관리자 수동
    // 실행 둘뿐이고, 조회 경로는 캐시가 낡았어도 저장분만 읽는다
    // (`service/stock_summary.go` Get → build(..., useLLM=false)).
    const before = await adminApi!.get<QuotaUsage[]>("/admin/api-quotas");
    const llmBefore = sumLLM(before);

    // 캐시 미스를 유도하려고 서로 다른 종목을 여러 개 읽는다.
    for (const symbol of [SYMBOLS.trade, SYMBOLS.stamp, SYMBOLS.watch, SYMBOLS.domestic]) {
      const res = await anon.send("GET", `/stocks/${symbol}/summary`);
      expect(res.status(), `${symbol} 요약 조회가 실패하면 이 검증이 무의미하다`).toBe(200);
    }

    const after = await adminApi!.get<QuotaUsage[]>("/admin/api-quotas");
    const llmAfter = sumLLM(after);

    expect(
      llmAfter,
      `조회만 했는데 LLM 사용량이 ${llmBefore} → ${llmAfter} 로 늘었다. ` +
        `조회 경로가 LLM을 부르면 트래픽이 그대로 비용이 된다.`,
    ).toBe(llmBefore);
  });
});

function sumLLM(usages: QuotaUsage[]): number {
  return usages
    .filter((u) => u.category === "llm")
    .reduce((total, u) => total + (u.dayCount ?? 0), 0);
}
