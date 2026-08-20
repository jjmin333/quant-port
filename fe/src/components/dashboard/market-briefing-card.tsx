"use client";

import { useEffect, useState } from "react";
import { Sparkles } from "lucide-react";
import { MoodBadge } from "@/components/news/news-briefing-card";
import { SymbolQuickLookModal } from "@/components/stocks/symbol-quick-look-modal";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { SectionCard } from "@/components/ui/section-card";
import { fetchNewsBriefing, type NewsMarketBriefing } from "@/lib/news-briefing-api";
import { formatPercent } from "@/lib/format";
import { formatBriefingTimestamp } from "@/lib/news-market-briefing";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 홈 「오늘의 시장 브리핑」.
 *
 * 이 데이터는 **이미 서버에 있었지만 뉴스 화면에만 있었다.** 홈은 하루에 한 번 여는
 * 화면인데 정작 "오늘 시장이 어땠나"를 문장으로 말해 주는 자리가 없었고, 숫자 표만
 * 늘어놓아 처음 온 사람은 그 숫자들이 좋은 소식인지 나쁜 소식인지 알 수 없었다.
 *
 * **생성하지 않는다 — 저장된 것을 읽기만 한다.** 브리핑은 서버 배치(장중 30분 · 그 외
 * 1시간)와 관리자 수동 실행만 만든다. 화면이 열릴 때 만들면 하루 LLM 비용이 방문자
 * 수에 비례하는데, 그건 예산을 방문자가 정하게 두는 일이다.
 *
 * 뉴스 화면의 카드(`NewsBriefingCard`)와 형태가 다른 이유는 자리가 다르기 때문이다 —
 * 저기는 목록 위에 눕는 가로 띠라 문단을 3줄로 자르고 나머지를 모달로 뺐지만, 홈에서는
 * 세로 한 칸을 통째로 쓰므로 **핵심 사건과 지켜볼 것까지 펼친 채로** 둔다. 같은 내용을
 * 두 번 클릭해 찾게 만들 이유가 없다.
 */

export type BriefingPair = {
  domestic: NewsMarketBriefing | null;
  us: NewsMarketBriefing | null;
  /** 두 요청이 모두 끝나기 전에는 레이아웃을 확정하지 않는다. */
  loading: boolean;
};

const EMPTY_PAIR: BriefingPair = { domestic: null, us: null, loading: true };

/**
 * 국내·해외 브리핑을 **함께** 받는다.
 *
 * 토글을 누른 뒤에 부르면 첫 클릭마다 빈 카드가 한 번 보이고, 무엇보다 홈의 레이아웃이
 * 브리핑 유무로 갈리는데(있으면 2단, 없으면 지수 전폭) 한쪽만 알고 배치를 정하면
 * 나머지가 도착할 때 화면이 다시 뛴다.
 */
export function useMarketBriefings(): BriefingPair {
  const [pair, setPair] = useState<BriefingPair>(EMPTY_PAIR);

  useEffect(() => {
    let alive = true;
    void (async () => {
      const [domestic, us] = await Promise.all([
        fetchNewsBriefing("domestic").catch(() => null),
        fetchNewsBriefing("us").catch(() => null),
      ]);
      if (alive) setPair({ domestic, us, loading: false });
    })();
    return () => {
      alive = false;
    };
  }, []);

  return pair;
}

export function MarketBriefingCard({
  briefings,
  className,
}: {
  briefings: BriefingPair;
  className?: string;
}) {
  // 국내가 없고 해외만 있는 시간대(새벽)에는 있는 쪽으로 시작한다.
  const [market, setMarket] = useState<"domestic" | "us" | null>(null);
  const available = briefings.domestic ? "domestic" : briefings.us ? "us" : null;
  const active = market ?? available;
  const briefing = active === "us" ? briefings.us : briefings.domestic;

  const [expanded, setExpanded] = useState(false);
  const [picked, setPicked] = useState<string | null>(null);

  if (briefings.loading) {
    return (
      <SectionCard title="오늘의 시장 브리핑" className={className}>
        <div className="space-y-2 py-1">
          <div className="h-4 w-1/3 animate-pulse rounded bg-muted" />
          <div className="h-16 animate-pulse rounded bg-muted" />
          <div className="h-20 animate-pulse rounded bg-muted" />
        </div>
      </SectionCard>
    );
  }

  // 아직 한 번도 생성되지 않았으면 카드를 그리지 않는다 — 빈 상자를 두느니 없는 편이 낫다.
  if (!briefing || !active) return null;

  const facts = briefing.facts;
  const highlights = briefing.highlights ?? [];
  const watchPoints = briefing.watchPoints ?? [];
  const mentions = facts.topMentioned ?? [];

  return (
    <>
      <SectionCard
        className={className}
        title={
          <span className="flex items-center gap-1.5">
            <Sparkles className="size-3.5 text-[var(--brand)]" />
            오늘의 시장 브리핑
          </span>
        }
        action={
          <div className="flex shrink-0 items-center gap-1.5">
            {briefing.source === "rules" && (
              <span
                className="rounded-md bg-muted px-1.5 py-px text-[9px] font-semibold text-muted-foreground"
                title="AI 생성에 실패했거나 꺼져 있어, 집계만으로 만든 문장입니다"
              >
                규칙
              </span>
            )}
            {/* 한쪽만 생성돼 있으면 토글을 아예 두지 않는다 — 눌러도 빈 카드가 되는 버튼. */}
            {briefings.domestic && briefings.us && (
              <MiniToggle
                options={[
                  { id: "domestic" as const, label: "국내" },
                  { id: "us" as const, label: "해외" },
                ]}
                value={active}
                onChange={setMarket}
              />
            )}
          </div>
        }
      >
        <div className="flex flex-wrap items-center gap-1.5">
          {briefing.mood && <MoodBadge mood={briefing.mood} />}
          <span className="text-[10px] text-muted-foreground">
            {formatBriefingTimestamp(briefing.generatedAt)} · {facts.session} · 기사{" "}
            {facts.articleCount.toLocaleString("ko-KR")}건
            {facts.breakingCount > 0 ? ` · 속보 ${facts.breakingCount}건` : ""}
          </span>
        </div>

        {/*
          시황 배지는 모델의 판단이라 실제 지수 등락률을 항상 옆에 같이 둔다 — 둘이
          어긋나는 날 배지만 보고 오해하지 않도록.
        */}
        {(facts.indices?.length ?? 0) > 0 && (
          <div className="mt-2 flex flex-wrap gap-1.5">
            {facts.indices!.slice(0, 4).map((idx) => (
              <span
                key={idx.id}
                className="flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-[10px]"
              >
                <span className="text-muted-foreground">{idx.name}</span>
                <b className={cn("font-mono tabular-nums", trendTextClass(idx.changeRate))}>
                  {formatPercent(idx.changeRate)}
                </b>
              </span>
            ))}
          </div>
        )}

        {briefing.moodReason && (
          <p className="mt-2 rounded-lg bg-muted px-2.5 py-1.5 text-[11px] leading-relaxed text-muted-foreground">
            {briefing.moodReason}
          </p>
        )}

        <p
          className={cn(
            "mt-2 whitespace-pre-line text-xs leading-relaxed",
            !expanded && "line-clamp-6",
          )}
        >
          {briefing.summary}
        </p>
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          className="mt-1 text-[11px] font-medium text-[var(--brand)] hover:underline"
        >
          {expanded ? "접기" : "전체 읽기"}
        </button>

        {(highlights.length > 0 || watchPoints.length > 0) && (
          <div className="mt-3 grid gap-2 sm:grid-cols-2">
            {highlights.length > 0 && (
              <BriefingList title="오늘의 핵심" items={highlights} />
            )}
            {watchPoints.length > 0 && <BriefingList title="지켜볼 것" items={watchPoints} />}
          </div>
        )}

        {mentions.length > 0 && (
          <div className="mt-3">
            <p className="mb-1.5 text-[11px] font-semibold text-muted-foreground">
              많이 언급된 종목
            </p>
            <div className="flex flex-wrap gap-1.5">
              {mentions.slice(0, 6).map((m) => (
                <button
                  key={m.symbol}
                  type="button"
                  onClick={() => setPicked(m.symbol)}
                  className="flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-[11px] transition-colors hover:border-foreground/20 hover:bg-accent"
                >
                  <span className="max-w-[7rem] truncate">{m.name || m.symbol}</span>
                  {/* 시세를 못 붙인 종목은 0%가 아니라 "—" — 0%로 그리면 보합으로 읽힌다. */}
                  <b
                    className={cn(
                      "font-mono tabular-nums",
                      m.hasQuote ? trendTextClass(m.changeRate) : "text-muted-foreground",
                    )}
                  >
                    {m.hasQuote ? formatPercent(m.changeRate) : "—"}
                  </b>
                  <span className="text-[10px] text-muted-foreground">{m.mentions}건</span>
                </button>
              ))}
            </div>
          </div>
        )}

        <p className="mt-3 text-[10px] leading-relaxed text-muted-foreground">
          {briefing.source === "llm"
            ? "수집된 기사 제목과 지수·시세만으로 생성한 요약입니다. 투자 조언이 아닙니다."
            : "AI 생성 없이 집계 수치만으로 만든 요약입니다. 투자 조언이 아닙니다."}
        </p>
      </SectionCard>

      <SymbolQuickLookModal symbol={picked} onClose={() => setPicked(null)} />
    </>
  );
}

function BriefingList({ title, items }: { title: string; items: string[] }) {
  return (
    <div className="rounded-lg border px-2.5 py-2">
      <p className="mb-1 text-[11px] font-semibold text-muted-foreground">{title}</p>
      <ul className="list-disc space-y-0.5 pl-3.5 text-[11px] leading-relaxed">
        {items.slice(0, 4).map((item, i) => (
          <li key={i}>{item}</li>
        ))}
      </ul>
    </div>
  );
}
