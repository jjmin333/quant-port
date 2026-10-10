"use client";

import { useEffect, useState } from "react";
import { Sparkles } from "lucide-react";
import { apiRequest } from "@/lib/api-client";
import { formatDateTimeKST } from "@/lib/format";

/**
 * 실적 화면 맨 위 「AI 실적 요약」 (2026-10-09 사용자 요청 1 · docs/mockups/requests-1009-v1.html ①).
 *
 * 최근 7일 발표의 서프라이즈와 다가오는 대형 발표를 한 장으로. 배치(07:10 · 국내 18:50 · 해외 21:30)가
 * 저장한 것을 읽기만 한다(조회 LLM 0). 사실표가 같으면 다시 쓰지 않으므로 시각은 「마지막으로 바뀐 때」다.
 * 아직 한 번도 안 돌았으면 서지 않는다.
 */

type DigestFacts = {
  released: number;
  beats: number;
  misses: number;
  avgPct?: number;
  recent: { name: string; date: string; metric?: string; est?: string; act?: string; pct?: number; label?: string }[];
  upcoming: { name: string; date: string; status?: string }[];
};

type DigestRow = {
  market: string;
  body: { headline: string; points: string[]; watch?: string };
  facts: DigestFacts;
  source: "llm" | "rule";
  generatedAt: string;
};

export function EarningsAIDigestCard({ market }: { market: "domestic" | "us" }) {
  const [row, setRow] = useState<DigestRow | null>(null);

  useEffect(() => {
    let alive = true;
    apiRequest<DigestRow | null>(`/api/v1/earnings/digest?market=${market}`)
      .then((r) => {
        if (alive) setRow(r ?? null);
      })
      .catch(() => {
        if (alive) setRow(null);
      });
    return () => {
      alive = false;
    };
  }, [market]);

  if (!row?.body?.headline) return null;
  const { body } = row;

  return (
    <section
      aria-label="AI 실적 요약"
      className="rounded-xl border border-[var(--border)] bg-card bg-[linear-gradient(180deg,var(--brand-muted),transparent_70%)] px-4 py-3.5"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="inline-flex items-center gap-1 rounded-md bg-[var(--brand)]/10 px-1.5 py-0.5 text-micro font-bold text-[var(--brand)]">
          <Sparkles className="h-3 w-3" aria-hidden />
          AI 실적 요약 · {market === "us" ? "해외" : "국내"}
        </span>
        <span className="text-micro text-[var(--text-muted)]">
          {formatDateTimeKST(row.generatedAt)} · {row.source === "llm" ? "AI" : "규칙 문장"} · 하루 2회 갱신
        </span>
      </div>
      <p className="mt-1.5 text-sm font-bold leading-snug text-[var(--text-primary)] sm:text-base">{body.headline}</p>
      {body.points.length > 0 && (
        <ul className="mt-1.5 list-disc space-y-0.5 pl-4 text-cell leading-relaxed text-[var(--text-secondary)]">
          {body.points.map((p, i) => (
            <li key={i}>{p}</li>
          ))}
        </ul>
      )}
      {body.watch && (
        <p className="mt-2 text-cell text-[var(--text-secondary)]">
          <b className="text-[var(--text-primary)]">지켜볼 것</b> · {body.watch}
        </p>
      )}
    </section>
  );
}
