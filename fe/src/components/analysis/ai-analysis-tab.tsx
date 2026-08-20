"use client";

import { Loader2, Sparkles } from "lucide-react";
import { Card } from "@/components/ui/card";
import { DailyCloseTab } from "@/components/analysis/daily-close-tab";
import { MarketBriefing } from "@/components/analysis/market-briefing";
import { StockScoreCard } from "@/components/analysis/stock-score-card";
import type { StockScoreData } from "@/lib/stock-score-api";
import type { StockSummaryData } from "@/lib/stock-summary-types";
import { cn } from "@/lib/utils";

type AIAnalysisTabProps = {
  symbol: string;
  currency: "KRW" | "USD";
  /** Summary is fetched by the parent so the header line and this tab share one request. */
  data: StockSummaryData | null;
  loading: boolean;
  error?: string | null;
  compact?: boolean;
  /** Score is fetched by the parent too — several tabs show one axis of it. */
  score?: StockScoreData | null;
  scoreLoading?: boolean;
};

const SECTIONS: { key: keyof Pick<StockSummaryData, "valuation" | "technical" | "insider" | "news">; label: string }[] = [
  { key: "valuation", label: "밸류에이션" },
  { key: "technical", label: "기술적 분석" },
  { key: "insider", label: "내부자 거래" },
  { key: "news", label: "뉴스" },
];

function formatGeneratedAt(value: string | undefined): string {
  if (!value) return "";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleString("ko-KR", {
    timeZone: "Asia/Seoul",
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/**
 * The full AI analysis: every generated section plus the dated score history.
 * The header carries only a one-line headline, so this tab is where the whole
 * picture lives. It replaces the former 일별 시세 tab, whose close/score list is
 * embedded below — the dates and the reasoning belong together.
 */
export function AIAnalysisTab({
  symbol,
  currency,
  data,
  loading,
  error,
  compact = false,
  score = null,
  scoreLoading = false,
}: AIAnalysisTabProps) {
  const generatedAt = formatGeneratedAt(data?.generatedAt);

  return (
    <div className="space-y-3">
      {/* Deterministic first: the score and the rule-based 시장 분석 are
          available for every symbol, so they lead — the AI prose then adds
          interpretation on top rather than being the only content. */}
      <StockScoreCard score={score} loading={scoreLoading} />
      <MarketBriefing briefing={data?.briefing} currency={currency} />

      <Card className="p-0">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-[var(--border)] px-3 py-2.5">
          <h2 className="flex items-center gap-1.5 text-sm font-semibold text-[var(--text-primary)]">
            <Sparkles className="h-4 w-4 text-[var(--brand)]" aria-hidden />
            AI 분석
          </h2>
          <div className="flex items-center gap-2 text-[10px] text-[var(--text-muted)]">
            {data?.source === "rule" && (
              <span className="rounded-full bg-[var(--surface-elevated)] px-2 py-0.5">
                규칙 기반 (AI 모델 미설정)
              </span>
            )}
            {generatedAt && <span>{generatedAt} 생성</span>}
          </div>
        </div>

        {loading ? (
          <p className="flex items-center justify-center gap-2 px-3 py-10 text-xs text-[var(--text-muted)]">
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
            AI 분석 생성 중…
          </p>
        ) : error || !data ? (
          <p className="px-3 py-10 text-center text-xs text-[var(--text-muted)]">
            {error ?? "AI 분석을 불러오지 못했습니다."}
          </p>
        ) : (
          <div className={cn("grid gap-px bg-[var(--border)]", compact ? "" : "sm:grid-cols-2")}>
            {SECTIONS.map(({ key, label }) => {
              const text = data[key]?.trim();
              return (
                <section key={key} className="bg-[var(--surface)] px-3 py-2.5">
                  <div className="flex items-center justify-between gap-2">
                    <h3 className="text-[11px] font-semibold text-[var(--brand)]">{label}</h3>
                    {key === "news" && data.newsSentiment && data.newsSentiment.total > 0 && (
                      <span className="shrink-0 text-[10px] tabular-nums text-[var(--text-muted)]">
                        긍정 {data.newsSentiment.positive} · 부정 {data.newsSentiment.negative} · 중립{" "}
                        {data.newsSentiment.neutral}
                      </span>
                    )}
                  </div>
                  <p className="mt-1 text-[12px] leading-relaxed text-[var(--text-secondary)]">
                    {text || "요약할 데이터가 부족합니다."}
                  </p>
                </section>
              );
            })}
          </div>
        )}
      </Card>

      <DailyCloseTab symbol={symbol} currency={currency} compact={compact} />
    </div>
  );
}
