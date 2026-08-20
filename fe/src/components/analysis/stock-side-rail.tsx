"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { PenLine } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { DirectionBadge } from "@/components/research/research-shared";
import { ScoreRadarChart } from "@/components/analysis/score-radar-chart";
import { StockDailyCloseCard } from "@/components/analysis/stock-daily-close-card";
import { StockScheduleCard } from "@/components/analysis/stock-schedule-card";
import { StockInsiderCard } from "@/components/analysis/stock-insider-card";
import {
  SectionCard,
  SectionEmpty,
  SectionKeyValue,
  SectionSkeleton,
} from "@/components/ui/section-card";
import { fetchStockNews } from "@/lib/news-api";
import {
  fetchResearchCallsClient,
  fetchResearchSymbolSummaryClient,
} from "@/lib/research-api";
import {
  formatMarketCap,
  formatPercent,
  formatPrice,
  formatRelativeTime,
  formatVolume,
} from "@/lib/format";
import {
  SCORE_TONE_TEXT as TONE_TEXT,
  scoreTone,
  type StockScoreData,
} from "@/lib/stock-score-api";
import { trendTextClass } from "@/lib/trend-colors";
import type { NewsArticle } from "@/lib/news-types";
import type { QuoteDetail } from "@/lib/quotes-types";
import type { ResearchCall, ResearchSymbolSummary } from "@/lib/research-types";
import { cn } from "@/lib/utils";

/**
 * 종목 분석 화면의 우측 레일.
 *
 * 본문 탭은 한 번에 하나(밸류에이션이면 밸류에이션만)를 보여준다. 그동안 "지금 얼마인지",
 * "점수가 몇 점인지", "무슨 일이 있었는지"는 탭을 옮겨야 알 수 있었다. 레일은 **탭과
 * 무관하게 항상 같은 자리에** 그 셋을 둔다 — 탭을 옮기는 것이 맥락을 잃는 일이 되지
 * 않도록.
 *
 * 위젯은 각자 실패한다. 뉴스가 죽어도 스냅샷은 뜨고, 비로그인이라 리포트가 비어도
 * 나머지는 그대로다.
 */

type StockSideRailProps = {
  symbol: string;
  name: string;
  currency: "KRW" | "USD";
  market: "domestic" | "us";
  quote: QuoteDetail | null;
  score: StockScoreData | null;
  scoreLoading: boolean;
};

export function StockSideRail({
  symbol,
  name,
  currency,
  market,
  quote,
  score,
  scoreLoading,
}: StockSideRailProps) {
  return (
    <>
      <QuoteSnapshotCard symbol={symbol} currency={currency} quote={quote} />
      <ScoreDigestCard symbol={symbol} score={score} loading={scoreLoading} />
      {/* 아래 세 카드는 종목이 바뀌면 key로 다시 마운트한다 — 이전 종목 데이터가
          새 종목 이름 밑에 한 프레임 남는 것을 막는다 */}
      <StockDailyCloseCard key={`close-${symbol}`} symbol={symbol} currency={currency} />
      <StockScheduleCard key={`sched-${symbol}`} symbol={symbol} />
      <StockInsiderCard key={`insider-${symbol}`} symbol={symbol} />
      {/*
        종목이 바뀌면 두 위젯은 `key`로 다시 마운트된다. 이펙트 안에서 `setItems(null)`로
        비우면 종목마다 렌더가 한 번씩 더 돌고(react-hooks/set-state-in-effect), 그
        한 프레임 동안 **이전 종목의 뉴스가 새 종목 이름 밑에 남는다**. 초기 상태로
        다시 시작하는 편이 짧고 정확하다.
      */}
      <StockNewsDigestCard key={`news-${symbol}`} symbol={symbol} name={name} market={market} />
      <StockReportsCard key={`reports-${symbol}`} symbol={symbol} />
    </>
  );
}

/* ── 시세 스냅샷 ──────────────────────────────────────────────────────────── */

function QuoteSnapshotCard({
  symbol,
  currency,
  quote,
}: {
  symbol: string;
  currency: "KRW" | "USD";
  quote: QuoteDetail | null;
}) {
  if (!quote) {
    return (
      <SectionCard title="시세 스냅샷">
        <SectionSkeleton rows={5} />
      </SectionCard>
    );
  }

  const change = quote.changeRate ?? 0;
  // 52주 레인지 안에서 현재가가 어디인지. 상·하한이 같으면(데이터 없음) 막대를 그리지 않는다 —
  // 0으로 나눈 결과를 "바닥"으로 그리면 없는 사실을 만들어낸다.
  const span = quote.week52High - quote.week52Low;
  const pos =
    span > 0
      ? Math.min(100, Math.max(0, ((quote.currentPrice - quote.week52Low) / span) * 100))
      : null;

  return (
    <SectionCard title="시세 스냅샷" href={`/stocks/${symbol}`} hrefLabel="터미널">
      <div className="flex items-baseline gap-2">
        <span className="font-mono text-xl font-bold tabular-nums">
          {formatPrice(quote.currentPrice, currency)}
        </span>
        <span className={cn("font-mono text-xs tabular-nums", trendTextClass(change))}>
          {formatPercent(change)}
        </span>
      </div>

      {pos !== null && (
        <div className="mt-2">
          <div className="relative h-1.5 rounded-full bg-muted">
            <span
              className="absolute top-1/2 size-2.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-foreground"
              style={{ left: `${pos}%` }}
              aria-hidden
            />
          </div>
          <div className="mt-1 flex justify-between text-[10px] text-muted-foreground">
            <span>52주 저 {formatPrice(quote.week52Low, currency)}</span>
            <span>고 {formatPrice(quote.week52High, currency)}</span>
          </div>
        </div>
      )}

      <div className="mt-2 border-t pt-1.5">
        <SectionKeyValue label="시가" value={formatPrice(quote.open, currency)} />
        <SectionKeyValue
          label="고가 / 저가"
          value={`${formatPrice(quote.high, currency)} / ${formatPrice(quote.low, currency)}`}
        />
        <SectionKeyValue
          label="거래량"
          value={quote.volume ? formatVolume(quote.volume) : "—"}
        />
        <SectionKeyValue
          label="시가총액"
          value={quote.marketCap ? formatMarketCap(quote.marketCap, currency) : "—"}
        />
        <SectionKeyValue
          label="PER"
          value={quote.per != null ? quote.per.toFixed(2) : "—"}
        />
        <SectionKeyValue
          label="PBR"
          value={quote.pbr != null ? quote.pbr.toFixed(2) : "—"}
        />
      </div>
    </SectionCard>
  );
}

/* ── 종합 점수 요약 ────────────────────────────────────────────────────────── */

function ScoreDigestCard({
  symbol,
  score,
  loading,
}: {
  symbol: string;
  score: StockScoreData | null;
  loading: boolean;
}) {
  if (loading) {
    return (
      <SectionCard title="종합 점수">
        <SectionSkeleton rows={5} />
      </SectionCard>
    );
  }
  if (!score || score.score == null) {
    return (
      <SectionCard title="종합 점수">
        <SectionEmpty text="아직 산출된 점수가 없습니다" />
      </SectionCard>
    );
  }

  const tone = scoreTone(score.score);

  return (
    <SectionCard
      title="종합 점수"
      href={`/analysis?symbol=${encodeURIComponent(symbol)}&tab=ai`}
      hrefLabel="AI 분석"
    >
      <div className="flex items-baseline gap-1.5">
        <span className={cn("font-mono text-2xl font-bold tabular-nums", TONE_TEXT[tone])}>
          {score.score}
        </span>
        <span className="text-[11px] text-muted-foreground">/ 100</span>
        {score.grade && (
          <Badge variant="default" className="ml-auto">
            {score.grade}
          </Badge>
        )}
      </div>
      <p className="mt-0.5 text-[10px] text-muted-foreground">
        {score.tradeDate} 기준
        {score.peerPct != null && ` · 동종 상위 ${Math.round(score.peerPct)}%`}
        {` · 커버리지 ${Math.round(score.coverage * 100)}%`}
      </p>

      {/* 막대 대신 레이더 (08-11 사용자 결정) — 정확한 값은 AI 분석 탭 막대가 맡는다 */}
      <div className="mt-1.5 border-t pt-1">
        <ScoreRadarChart breakdown={score.breakdown} score={score.score} />
      </div>
    </SectionCard>
  );
}

/* ── 관련 뉴스 ────────────────────────────────────────────────────────────── */

function StockNewsDigestCard({
  symbol,
  name,
  market,
}: {
  symbol: string;
  name: string;
  market: "domestic" | "us";
}) {
  const [items, setItems] = useState<NewsArticle[] | null>(null);

  useEffect(() => {
    let alive = true;
    fetchStockNews({ symbol, name, market, limit: 5 })
      .then((articles) => {
        if (alive) setItems(articles);
      })
      .catch(() => {
        if (alive) setItems([]);
      });
    return () => {
      alive = false;
    };
  }, [symbol, name, market]);

  const newsHref = market === "us" ? "/news/us" : "/news/domestic";

  return (
    <SectionCard title="관련 뉴스" href={newsHref} hrefLabel="뉴스">
      {items === null ? (
        <SectionSkeleton rows={5} />
      ) : items.length === 0 ? (
        <SectionEmpty text="최근 관련 기사가 없습니다" />
      ) : (
        <ul className="min-w-0 space-y-0.5">
          {items.map((n) => (
            <li key={n.id}>
              <Link
                href={`${newsHref}?article=${encodeURIComponent(n.id)}`}
                className="block rounded-md px-1 py-1.5 hover:bg-accent"
              >
                <p className="line-clamp-2 text-xs leading-snug">{n.headlineKo || n.headline}</p>
                <p className="mt-1 flex items-center gap-1.5 text-[11px] text-muted-foreground">
                  <span className={n.publisher.badgeClass}>{n.publisher.name}</span>
                  {formatRelativeTime(n.publishedAt)}
                </p>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  );
}

/* ── 이 종목 리포트 ────────────────────────────────────────────────────────── */

function StockReportsCard({ symbol }: { symbol: string }) {
  const [calls, setCalls] = useState<ResearchCall[] | null>(null);
  const [summary, setSummary] = useState<ResearchSymbolSummary | null>(null);

  useEffect(() => {
    let alive = true;

    void (async () => {
      try {
        const list = await fetchResearchCallsClient({ symbol, limit: 3 });
        if (alive) setCalls(list);
      } catch {
        if (alive) setCalls([]);
      }
    })();

    void (async () => {
      try {
        const s = await fetchResearchSymbolSummaryClient(symbol);
        if (alive) setSummary(s);
      } catch {
        // 요약이 없어도 목록은 보여준다.
      }
    })();

    return () => {
      alive = false;
    };
  }, [symbol]);

  return (
    <SectionCard title="이 종목 리포트" href="/reports" hrefLabel="리포트">
      {summary && summary.total > 0 && (
        <div className="mb-2 flex flex-wrap gap-1 border-b pb-2 text-[11px] text-muted-foreground">
          <span>전체 {summary.total}건</span>
          <span className="text-[var(--profit)]">매수 {summary.buy}</span>
          <span className="text-[var(--loss)]">매도 {summary.sell}</span>
          <span>관망 {summary.hold}</span>
        </div>
      )}

      {calls === null ? (
        <SectionSkeleton rows={3} />
      ) : calls.length === 0 ? (
        <div className="py-4 text-center">
          <p className="text-[11px] text-muted-foreground">아직 이 종목 리포트가 없습니다</p>
          <Link
            href={`/reports/new?symbol=${encodeURIComponent(symbol)}`}
            className="mt-2 inline-flex items-center gap-1 rounded-lg border px-2.5 py-1 text-[11px] hover:bg-accent"
          >
            <PenLine className="size-3" />첫 리포트 쓰기
          </Link>
        </div>
      ) : (
        <ul className="min-w-0 space-y-0.5">
          {calls.map((call) => (
            <li key={call.id}>
              <Link
                href={`/reports/${call.id}`}
                className="block rounded-md px-1 py-1.5 hover:bg-accent"
              >
                <span className="flex items-center gap-1">
                  <DirectionBadge direction={call.direction} />
                  <span className="truncate text-[11px] text-muted-foreground">
                    {call.authorName ?? "익명"}
                  </span>
                </span>
                <span className="mt-1 line-clamp-2 block text-xs leading-snug">{call.title}</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  );
}
