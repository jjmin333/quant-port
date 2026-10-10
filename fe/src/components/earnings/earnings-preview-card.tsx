"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ChevronDown, ChevronUp, Sparkles } from "lucide-react";
import { SectionCard } from "@/components/ui/section-card";
import { daysUntil } from "@/hooks/use-upcoming-events";
import { apiRequest } from "@/lib/api-client";
import { SCORE_TONE_BADGE, scoreTone } from "@/lib/stock-score-api";
import { cn } from "@/lib/utils";

/**
 * 「이번 주 실적 관전 포인트」 (2026-09-27 사용자 결정 C ② — 실적 메뉴에서 LLM 활용).
 *
 * 향후 7일에 발표하는 종목마다 **무엇을 봐야 하나** 세 줄과 주의점 한 줄. 매일 07:10 배치가
 * AI 점수·지난 서프라이즈·추정치 변화·가격 위치·최근 기사 제목으로 만들어 둔 것을 읽기만 한다
 * (조회 LLM 0). AI 한도가 없던 종목은 규칙 문장이고, 그렇다고 표시한다.
 */

type PreviewBody = { headline: string; points: string[]; risk?: string; dateStatus?: string };
type PreviewRow = {
  symbol: string;
  name: string;
  market: string;
  eventDate: string;
  fiscalLabel: string;
  body: PreviewBody;
  source: "llm" | "rule";
  generatedAt: string;
  score?: number;
};

const COLLAPSED = 4;

export function EarningsPreviewCard({ market }: { market: "domestic" | "us" }) {
  const [rows, setRows] = useState<PreviewRow[] | null>(null);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    let alive = true;
    apiRequest<PreviewRow[] | null>(`/api/v1/earnings/previews?market=${market}`)
      .then((r) => {
        if (alive) setRows(r ?? []);
      })
      .catch(() => {
        if (alive) setRows([]);
      });
    return () => {
      alive = false;
    };
  }, [market]);

  if (rows === null || rows.length === 0) return null;
  const shown = open ? rows : rows.slice(0, COLLAPSED);
  const generated = rows.reduce((max, r) => (r.generatedAt > max ? r.generatedAt : max), "");
  const aiCount = rows.filter((r) => r.source === "llm").length;

  return (
    <SectionCard
      title={
        <span className="flex items-center gap-1.5">
          <Sparkles className="h-4 w-4 text-[var(--brand)]" />
          이번 주 실적 관전 포인트
          <span className="text-micro font-normal text-[var(--text-muted)]">{rows.length}종목</span>
        </span>
      }
      footer={
        <span>
          AI {aiCount} / 규칙 {rows.length - aiCount}
          {generated && ` · ${generated.slice(5, 16).replace("T", " ")} 기준`}
        </span>
      }
    >
      <ul className="grid grid-cols-[minmax(0,1fr)] divide-y divide-[var(--border)]">
        {shown.map((r) => {
          const d = daysUntil(r.eventDate);
          return (
            <li key={`${r.symbol}-${r.eventDate}`} className="grid min-w-0 grid-cols-[minmax(0,1fr)] gap-1 py-2.5">
              <div className="flex min-w-0 flex-wrap items-center gap-1.5 text-cell">
                <span className="shrink-0 rounded bg-amber-500/15 px-1.5 font-semibold text-amber-700 dark:text-amber-400">
                  {r.eventDate.slice(5).replace("-", "/")} {d === 0 ? "오늘" : d != null ? `D-${d}` : ""}
                  {r.body.dateStatus === "estimated" && <span className="ml-0.5 font-normal" title="작년 발표일로 추정한 날짜">(추정)</span>}
                </span>
                <Link href={`/stocks/${encodeURIComponent(r.symbol)}`} className="truncate font-bold text-[var(--text-primary)] hover:text-[var(--brand)]">
                  {r.name}
                </Link>
                <span className="text-micro text-[var(--text-muted)]">{r.fiscalLabel}</span>
                {r.score != null && (
                  <span className={cn("rounded px-1 text-micro font-bold", SCORE_TONE_BADGE[scoreTone(r.score)])}>{r.score}</span>
                )}
                <span className="ml-auto text-micro text-[var(--text-muted)]">{r.source === "llm" ? "AI" : "규칙"}</span>
              </div>
              <p className="text-note font-semibold leading-snug text-[var(--text-primary)]">{r.body.headline}</p>
              <ul className="list-disc space-y-0.5 pl-4 text-cell leading-relaxed text-[var(--text-secondary)]">
                {r.body.points.map((p, i) => (
                  <li key={i}>{p}</li>
                ))}
              </ul>
              {r.body.risk && <p className="text-micro text-[var(--text-muted)]">주의 · {r.body.risk}</p>}
            </li>
          );
        })}
      </ul>
      {rows.length > COLLAPSED && (
        <button
          type="button"
          onClick={() => setOpen(!open)}
          className="mt-1 flex w-full items-center justify-center gap-1 rounded-md py-1.5 text-cell text-[var(--text-muted)] hover:bg-[var(--surface-elevated)]"
        >
          {open ? (
            <>
              접기 <ChevronUp className="h-3.5 w-3.5" />
            </>
          ) : (
            <>
              {rows.length - COLLAPSED}종목 더 보기 <ChevronDown className="h-3.5 w-3.5" />
            </>
          )}
        </button>
      )}
    </SectionCard>
  );
}
