"use client";

import { useEffect, useState } from "react";
import { ListChecks } from "lucide-react";
import { Card } from "@/components/ui/card";
import { apiRequest } from "@/lib/api-client";
import { cn } from "@/lib/utils";

/**
 * 「검토 노트」 — 리포트의 주장과 우리 데이터가 맞는지 (2026-09-27 사용자 결정 C ④). 10-12부터 LLM 없이 규칙 노트만.
 *
 * 30분 배치가 게시·수정 뒤에 만들어 둔 것을 읽는다(조회 LLM 0). 판정·채점이 아니라
 * **독자가 함께 확인할 곳**을 가리키는 메모다 — 리포트의 성적은 기한이 끝나면 따로 매겨진다.
 * 아직 없으면(게시 직후·비공개 리포트) 아무것도 그리지 않는다.
 */

type ReviewCheck = { claim: string; data: string; verdict: "일치" | "근거 약함" | "반대" | "확인 불가" };
type ReviewRow = {
  callId: number;
  bodyRev: number;
  body: { checks: ReviewCheck[]; summary: string };
  source: "llm" | "rule";
  generatedAt: string;
};

const VERDICT_TONE: Record<ReviewCheck["verdict"], string> = {
  일치: "bg-emerald-500/12 text-emerald-700 dark:text-emerald-400",
  "근거 약함": "bg-amber-500/15 text-amber-700 dark:text-amber-400",
  반대: "bg-rose-500/12 text-rose-600 dark:text-rose-400",
  "확인 불가": "bg-[var(--surface-elevated)] text-[var(--text-muted)]",
};

export function ResearchReviewCard({ callId, bodyRev }: { callId: number; bodyRev: number }) {
  const [row, setRow] = useState<ReviewRow | null>(null);

  useEffect(() => {
    let alive = true;
    apiRequest<ReviewRow | null>(`/api/v1/research/calls/${callId}/review`)
      .then((r) => {
        if (alive) setRow(r);
      })
      .catch(() => {
        if (alive) setRow(null);
      });
    return () => {
      alive = false;
    };
  }, [callId]);

  if (!row || row.body.checks.length === 0) return null;
  const stale = row.bodyRev < bodyRev;

  return (
    <Card className="space-y-3 p-4 sm:p-5">
      <div className="flex flex-wrap items-center gap-2">
        <ListChecks className="h-4 w-4 text-[var(--brand)]" />
        <h2 className="text-section font-semibold text-[var(--text-primary)]">검토 노트</h2>
        <span className="text-micro text-[var(--text-muted)]">
          {row.source === "llm" ? "AI" : "규칙"} · {row.generatedAt.slice(5, 16).replace("T", " ")}
          {stale && " · 수정 전 본문 기준(곧 다시 검토)"}
        </span>
      </div>
      <ul className="space-y-2">
        {row.body.checks.map((c, i) => (
          <li key={i} className="grid grid-cols-[auto_minmax(0,1fr)] items-start gap-2 text-cell">
            <span className={cn("mt-0.5 shrink-0 rounded px-1.5 py-0.5 text-micro font-semibold", VERDICT_TONE[c.verdict] ?? VERDICT_TONE["확인 불가"])}>
              {c.verdict}
            </span>
            <span className="min-w-0">
              <b className="font-semibold text-[var(--text-primary)]">{c.claim}</b>
              <span className="block text-[var(--text-secondary)]">{c.data}</span>
            </span>
          </li>
        ))}
      </ul>
      {row.body.summary && <p className="text-cell text-[var(--text-secondary)]">{row.body.summary}</p>}
      <p className="text-micro text-[var(--text-muted)]">
        리포트의 주장과 Quant Port 데이터(점수·가격·이후 기사·공시)를 대조한 메모입니다. 리포트의 맞고 틀림을 판정하지 않습니다.
      </p>
    </Card>
  );
}
