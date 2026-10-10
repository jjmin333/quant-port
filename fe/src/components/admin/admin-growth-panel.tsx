"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { GrowthMarkdown } from "@/components/admin/growth-markdown";
import { PageHeader } from "@/components/layout/header";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useAuth } from "@/components/providers/auth-provider";
import { isAdminSession } from "@/lib/admin-features";
import {
  fetchAdminGrowthReport,
  fetchAdminGrowthReports,
  type GrowthReport,
  type GrowthReportKind,
  type GrowthReportSummary,
} from "@/lib/admin";
import { toUserMessage } from "@/lib/error-message";
import { cn } from "@/lib/utils";

/**
 * 검색 유입 보고서 (`/admin/growth`) — 2026-09-25.
 *
 * worker-growth가 만든 일간·주간 보고서를 읽는 화면이다. 만들기·다시 만들기는 없다 — 그건
 * 배치 콘솔의 「검색 유입 감시」(수동 실행, limit 1이면 주간까지)가 한 곳이다. 보고서가
 * 제안하는 수정은 사람이 브랜치로 옮긴다(docs/GROWTH.md).
 *
 * 기본은 **주간**이다. 일간은 추세를 보는 곳이 아니라 「어제 뭐가 새로 깨졌나」를 보는 곳이고,
 * 한 주의 할 일은 주간의 Top 20이 정한다.
 */
const KINDS: { kind: GrowthReportKind; label: string }[] = [
  { kind: "weekly", label: "주간" },
  { kind: "daily", label: "일간" },
];

function periodLabel(r: GrowthReportSummary): string {
  return r.kind === "weekly" ? `${r.periodStart} ~ ${r.periodEnd}` : r.periodEnd;
}

type ListResult = { items: GrowthReportSummary[]; first: GrowthReport | null; error: string | null };

/** 목록과 맨 위 보고서를 받는다 — 상태는 건드리지 않는다. */
async function loadListData(kind: GrowthReportKind): Promise<ListResult> {
  try {
    const items = (await fetchAdminGrowthReports(kind)) ?? [];
    const first = items.length > 0 ? await fetchAdminGrowthReport(items[0]!.id) : null;
    return { items, first, error: null };
  } catch (e) {
    return { items: [], first: null, error: toUserMessage(e, "목록을 불러오지 못했습니다") };
  }
}

export function AdminGrowthPanel() {
  const router = useRouter();
  const { session, loading } = useAuth();

  const [kind, setKind] = useState<GrowthReportKind>("weekly");
  const [list, setList] = useState<GrowthReportSummary[]>([]);
  const [selected, setSelected] = useState<number | null>(null);
  const [report, setReport] = useState<GrowthReport | null>(null);
  // 첫 목록을 받는 중으로 시작한다 — 이펙트 안에서 동기로 켜지 않으려고.
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const open = useCallback(async (id: number) => {
    setSelected(id);
    setBusy(true);
    setError(null);
    try {
      setReport(await fetchAdminGrowthReport(id));
    } catch (e) {
      setError(toUserMessage(e, "보고서를 불러오지 못했습니다"));
    } finally {
      setBusy(false);
    }
  }, []);

  // 상태는 응답을 받은 콜백에서만 바꾼다. 「불러오는 중」으로 되돌리는 것은 부르는 쪽(클릭)이 한다.
  const applyList = useCallback((res: ListResult) => {
    setList(res.items);
    setSelected(res.first?.id ?? null);
    setReport(res.first);
    setError(res.error);
    setBusy(false);
  }, []);

  const loadList = useCallback((k: GrowthReportKind) => loadListData(k).then(applyList), [applyList]);

  useEffect(() => {
    if (loading) return;
    if (!isAdminSession(session)) {
      router.replace("/");
      return;
    }
    void loadList(kind);
    // 종류를 바꿀 때는 아래 버튼이 직접 부른다.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loading, session, router]);

  if (loading || !isAdminSession(session)) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-[var(--text-muted)]">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        확인 중…
      </div>
    );
  }

  const pickKind = (k: GrowthReportKind) => {
    if (k === kind) return;
    setKind(k);
    setBusy(true);
    setError(null);
    setReport(null);
    setSelected(null);
    void loadList(k);
  };

  return (
    <>
      <PageHeader
        title="검색 유입 보고서"
        subtitle="매일 07:30 사이트 점검·검색 성과·가입 지표를 모아 만든 보고서입니다(월요일엔 주간)"
      />
      <PageScrollShell bare>
        <div className="page-shell-wide min-w-0 space-y-4 py-4">
          {error && (
            <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-300">
              {error}
            </div>
          )}

          <div className="grid gap-4 lg:grid-cols-[14rem_minmax(0,1fr)]">
            <section className="min-w-0 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-2">
              <div className="flex gap-1 p-1" role="tablist" aria-label="보고서 종류">
                {KINDS.map((k) => (
                  <button
                    key={k.kind}
                    type="button"
                    role="tab"
                    aria-selected={kind === k.kind}
                    onClick={() => pickKind(k.kind)}
                    className={cn(
                      "flex-1 rounded-lg px-2 py-1.5 text-cell",
                      kind === k.kind
                        ? "bg-[var(--brand-muted)] text-[var(--brand)]"
                        : "text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]",
                    )}
                  >
                    {k.label}
                  </button>
                ))}
              </div>
              {list.length === 0 && !busy ? (
                <p className="px-2 py-3 text-micro text-[var(--text-muted)]">
                  아직 보고서가 없습니다. 첫 보고서는 다음 07:30에 만들어집니다 —{" "}
                  <Link href="/admin/batch" className="text-[var(--brand)] underline">
                    배치 관리
                  </Link>
                  에서 「검색 유입 감시」를 바로 실행할 수도 있습니다.
                </p>
              ) : (
                <ul className="max-h-[40rem] overflow-y-auto">
                  {list.map((r) => (
                    <li key={r.id}>
                      <button
                        type="button"
                        onClick={() => void open(r.id)}
                        className={cn(
                          "w-full rounded-lg px-2 py-1.5 text-left font-num text-cell",
                          r.id === selected
                            ? "bg-[var(--brand-muted)] text-[var(--brand)]"
                            : "text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]",
                        )}
                      >
                        {periodLabel(r)}
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </section>

            <section className="min-w-0 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
              {busy && !report ? (
                <div className="flex h-32 items-center justify-center text-sm text-[var(--text-muted)]">
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  불러오는 중…
                </div>
              ) : report ? (
                <GrowthMarkdown source={report.markdown} />
              ) : (
                <p className="text-cell text-[var(--text-muted)]">왼쪽에서 보고서를 고르세요.</p>
              )}
            </section>
          </div>
        </div>
      </PageScrollShell>
    </>
  );
}
