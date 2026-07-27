import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { MorningBriefing } from "@/lib/insights-types";

export function MorningBriefing({ briefing }: { briefing: MorningBriefing }) {
  const earnings = briefing.earnings[0];
  const disclosure = briefing.disclosures[0];

  return (
    <Card className="min-w-[var(--dashboard-content-min)] shrink-0">
      <CardHeader>
        <CardTitle>오늘의 투자 브리핑</CardTitle>
        <Badge variant="info">{briefing.source === "llm" ? "AI 요약" : "규칙 요약"}</Badge>
      </CardHeader>
      <p className="mb-4 text-sm leading-relaxed text-[var(--text-secondary)]">
        {briefing.summary}
      </p>
      {briefing.highlights.length > 0 && (
        <div className="mb-4 flex flex-wrap gap-2">
          {briefing.highlights.map((item) => (
            <Badge key={item} variant="default" className="max-w-full truncate">
              {item}
            </Badge>
          ))}
        </div>
      )}
      <dl className="grid gap-2 text-sm">
        <div className="flex min-w-0 items-center justify-between gap-2 rounded-lg bg-[var(--surface-elevated)] px-3 py-2">
          <dt className="shrink-0 text-[var(--text-muted)]">다음 실적발표</dt>
          <dd className="truncate text-right text-[var(--text-primary)]">
            {earnings ? `${earnings.date} ${earnings.symbol}` : "예정 없음"}
          </dd>
        </div>
        <div className="flex min-w-0 items-center justify-between gap-2 rounded-lg bg-[var(--surface-elevated)] px-3 py-2">
          <dt className="shrink-0 text-[var(--text-muted)]">오늘 공시</dt>
          <dd className="truncate text-right text-[var(--text-primary)]">
            {disclosure ? `${disclosure.symbol} — ${disclosure.event}` : "없음"}
          </dd>
        </div>
        <div className="flex min-w-0 items-center justify-between gap-2 rounded-lg bg-[var(--surface-elevated)] px-3 py-2">
          <dt className="shrink-0 text-[var(--text-muted)]">예상 변동성</dt>
          <dd className="shrink-0 font-medium text-amber-400">{briefing.volatility}</dd>
        </div>
      </dl>
    </Card>
  );
}
